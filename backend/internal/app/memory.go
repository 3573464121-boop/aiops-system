package app

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"aiops-mvp/internal/domain"
	"aiops-mvp/internal/llm"
)

const maxRecall = 5

func (s *Service) CreateMemory(ctx context.Context, req domain.MemoryRequest) (domain.Memory, error) {
	content := strings.TrimSpace(req.Content)
	if content == "" {
		return domain.Memory{}, fmt.Errorf("记忆内容不能为空")
	}
	userID, username, role, actorTeam := actorFromContext(ctx)
	scope := strings.ToLower(strings.TrimSpace(req.Scope))
	if scope == "" {
		scope = "personal"
	}
	if scope != "global" && scope != "product" && scope != "team" && scope != "personal" {
		return domain.Memory{}, fmt.Errorf("不支持的记忆范围: %s", scope)
	}
	if (scope == "global" || scope == "product") && role != "admin" {
		return domain.Memory{}, fmt.Errorf("全局或产品记忆仅管理员可创建")
	}
	if scope == "personal" && userID == "" {
		return domain.Memory{}, fmt.Errorf("个人记忆需要登录用户")
	}

	productID := strings.TrimSpace(req.ProductID)
	if scope == "product" && productID == "" {
		return domain.Memory{}, fmt.Errorf("产品记忆必须指定 product_id")
	}
	teamID := strings.TrimSpace(req.TeamID)
	if scope == "team" {
		if teamID == "" {
			teamID = actorTeam
		}
		if actorTeam == "" || !strings.EqualFold(teamID, actorTeam) {
			return domain.Memory{}, fmt.Errorf("只能创建当前团队的记忆")
		}
	}

	kind := strings.ToLower(strings.TrimSpace(req.Kind))
	if kind != "fix" && kind != "preference" {
		kind = "fact"
	}
	source := strings.TrimSpace(req.Source)
	if source == "" {
		source = "manual"
	}
	m := domain.Memory{
		ID:        fmt.Sprintf("MEM-%d", time.Now().UnixNano()),
		Scope:     scope,
		ProductID: productID,
		TeamID:    teamID,
		OwnerID:   userID,
		OwnerName: username,
		Kind:      kind,
		Content:   content,
		Source:    source,
		CreatedAt: time.Now(),
	}
	if err := s.Repo.CreateMemory(m); err != nil {
		s.addAudit(ctx, "create_memory", productID, "error", 0)
		return domain.Memory{}, err
	}
	s.addAudit(ctx, "create_memory", productID, "success", 0)
	return m, nil
}

func (s *Service) ListMemories(ctx context.Context) ([]domain.Memory, error) {
	all, err := s.Repo.ListMemories()
	if err != nil {
		return nil, err
	}
	userID, _, _, teamID := actorFromContext(ctx)
	out := make([]domain.Memory, 0, len(all))
	for _, m := range all {
		if canReadMemory(m, userID, teamID) {
			out = append(out, m)
		}
	}
	return out, nil
}

func (s *Service) DeleteMemory(ctx context.Context, id string) error {
	all, err := s.Repo.ListMemories()
	if err != nil {
		return err
	}
	userID, _, role, teamID := actorFromContext(ctx)
	var found *domain.Memory
	for i := range all {
		if all[i].ID == id {
			found = &all[i]
			break
		}
	}
	if found == nil || !canReadMemory(*found, userID, teamID) {
		return fmt.Errorf("记忆不存在或无权访问")
	}
	if !canDeleteMemory(*found, userID, role, teamID) {
		return fmt.Errorf("无权删除该记忆")
	}
	if err := s.Repo.DeleteMemory(id); err != nil {
		s.addAudit(ctx, "delete_memory", found.ProductID, "error", 0)
		return err
	}
	s.addAudit(ctx, "delete_memory", found.ProductID, "success", 0)
	return nil
}

func canReadMemory(m domain.Memory, userID, teamID string) bool {
	switch m.Scope {
	case "personal":
		return userID != "" && m.OwnerID == userID
	case "team":
		return teamID != "" && strings.EqualFold(m.TeamID, teamID)
	default:
		return m.Scope == "global" || m.Scope == "product" || m.Scope == ""
	}
}

func canDeleteMemory(m domain.Memory, userID, role, teamID string) bool {
	switch m.Scope {
	case "personal":
		return userID != "" && m.OwnerID == userID
	case "team":
		return canReadMemory(m, userID, teamID) && (role == "admin" || m.OwnerID == userID)
	default:
		return role == "admin"
	}
}

func (s *Service) recallMemories(ctx context.Context, productID, query string, limit int) []domain.Memory {
	all, err := s.ListMemories(ctx)
	if err != nil || len(all) == 0 {
		return nil
	}
	terms := tokenize(query + " " + productID)
	type scored struct {
		m     domain.Memory
		score int
	}
	candidates := make([]scored, 0, len(all))
	for _, m := range all {
		if m.Scope == "product" && !strings.EqualFold(m.ProductID, productID) {
			continue
		}
		score := overlap(tokenize(m.Content), terms)
		if m.Scope == "product" || m.Scope == "team" || m.Scope == "personal" {
			score++
		}
		candidates = append(candidates, scored{m: m, score: score})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		return candidates[i].m.CreatedAt.After(candidates[j].m.CreatedAt)
	})
	out := make([]domain.Memory, 0, limit)
	for _, candidate := range candidates {
		if len(out) >= limit {
			break
		}
		out = append(out, candidate.m)
	}
	return out
}

func memoryScopeLabel(m domain.Memory) string {
	switch m.Scope {
	case "product":
		return "产品/" + m.ProductID
	case "team":
		return "团队/" + m.TeamID
	case "personal":
		return "个人/" + m.OwnerName
	default:
		return "全局"
	}
}

func memoryNote(memories []domain.Memory) string {
	if len(memories) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("以下是与本次问题相关的历史经验或环境记忆。仅作参考，必须结合当前证据判断：\n")
	for _, m := range memories {
		b.WriteString(fmt.Sprintf("- [%s] %s\n", memoryScopeLabel(m), m.Content))
	}
	return b.String()
}

func memoriesToEvidence(memories []domain.Memory) []domain.Evidence {
	out := make([]domain.Evidence, 0, len(memories))
	for _, m := range memories {
		out = append(out, domain.Evidence{
			Type: "memory", Title: "记忆 / " + memoryScopeLabel(m),
			Content: m.Content, Score: .5, Source: "memory/" + m.ID,
		})
	}
	return out
}

func (s *Service) ExtractMemory(ctx context.Context, req domain.MemoryExtractRequest) (string, error) {
	text := strings.TrimSpace(req.Text)
	if text == "" {
		return "", fmt.Errorf("待提炼文本不能为空")
	}
	if s.LLM == nil || !s.LLM.Enabled() {
		return "", fmt.Errorf("未配置大模型，无法自动提炼")
	}
	system := "你是运维知识整理助手。从给定诊断内容中提炼一条可长期复用的经验或环境事实。" +
		"要求一句话、具体、去掉时间戳和任务编号；不要把猜测写成确定结论。只输出这句话，不要解释、引号或前缀。"
	user := "产品：" + req.ProductID + "\n诊断内容：\n" + text
	msg, err := s.LLM.Chat(ctx, []llm.Message{{Role: "system", Content: system}, {Role: "user", Content: user}}, nil, false)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(strings.Trim(msg.Content, "\"'“”")), nil
}

func tokenize(s string) map[string]struct{} {
	s = strings.ToLower(s)
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r >= 0x4e00 && r <= 0x9fff)
	})
	set := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		if len([]rune(field)) >= 2 {
			set[field] = struct{}{}
		}
		runes := []rune(field)
		for i := 0; i+1 < len(runes); i++ {
			if runes[i] >= 0x4e00 && runes[i] <= 0x9fff {
				set[string(runes[i:i+2])] = struct{}{}
			}
		}
	}
	return set
}

func overlap(a, b map[string]struct{}) int {
	count := 0
	for term := range a {
		if _, ok := b[term]; ok {
			count++
		}
	}
	return count
}
