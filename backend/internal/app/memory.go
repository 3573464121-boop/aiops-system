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

const maxRecall = 5 // 每次诊断自动召回的记忆条数上限

// CreateMemory 新建一条长期记忆。scope 默认 global，kind 默认 fact。
func (s *Service) CreateMemory(req domain.MemoryRequest) (domain.Memory, error) {
	content := strings.TrimSpace(req.Content)
	if content == "" {
		return domain.Memory{}, fmt.Errorf("记忆内容不能为空")
	}
	scope := strings.TrimSpace(req.Scope)
	if scope != "product" {
		scope = "global"
	}
	pid := strings.TrimSpace(req.ProductID)
	if scope == "product" && pid == "" {
		return domain.Memory{}, fmt.Errorf("product 作用域必须指定 product_id")
	}
	kind := strings.TrimSpace(req.Kind)
	if kind == "" {
		kind = "fact"
	}
	source := strings.TrimSpace(req.Source)
	if source == "" {
		source = "manual"
	}
	m := domain.Memory{
		ID:        fmt.Sprintf("MEM-%d", time.Now().UnixNano()),
		Scope:     scope,
		ProductID: pid,
		Kind:      kind,
		Content:   content,
		Source:    source,
		CreatedAt: time.Now(),
	}
	if err := s.Repo.CreateMemory(m); err != nil {
		return domain.Memory{}, err
	}
	return m, nil
}

func (s *Service) ListMemories() ([]domain.Memory, error) { return s.Repo.ListMemories() }
func (s *Service) DeleteMemory(id string) error           { return s.Repo.DeleteMemory(id) }

// recallMemories 召回与当前诊断相关的记忆：候选=全局记忆 + 该产品记忆，按与问题的关键词重合度打分，取前 limit 条。
func (s *Service) recallMemories(productID, query string, limit int) []domain.Memory {
	all, err := s.Repo.ListMemories()
	if err != nil || len(all) == 0 {
		return nil
	}
	terms := tokenize(query + " " + productID)
	type scored struct {
		m     domain.Memory
		score int
	}
	cand := make([]scored, 0, len(all))
	for _, m := range all {
		if m.Scope == "product" && !strings.EqualFold(m.ProductID, productID) {
			continue // 限定其它产品的记忆，跳过
		}
		sc := overlap(tokenize(m.Content), terms)
		if m.Scope == "product" && strings.EqualFold(m.ProductID, productID) {
			sc++ // 同产品记忆略微加权
		}
		cand = append(cand, scored{m, sc})
	}
	// 有匹配的按分数优先；全无匹配时保留全局记忆作为通用背景（按时间倒序）。
	sort.SliceStable(cand, func(i, j int) bool {
		if cand[i].score != cand[j].score {
			return cand[i].score > cand[j].score
		}
		return cand[i].m.CreatedAt.After(cand[j].m.CreatedAt)
	})
	out := make([]domain.Memory, 0, limit)
	for _, c := range cand {
		if len(out) >= limit {
			break
		}
		if c.score == 0 && c.m.Scope == "product" {
			continue // 无关的产品记忆不强行塞入
		}
		out = append(out, c.m)
	}
	return out
}

// memoryNote 把召回的记忆拼成一段供模型参考的系统提示。
func memoryNote(ms []domain.Memory) string {
	if len(ms) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("以下是与本次诊断相关的历史经验/环境记忆（供参考，仍须结合当前工具证据判断，勿凭记忆臆断）：\n")
	for _, m := range ms {
		scope := "全局"
		if m.Scope == "product" {
			scope = m.ProductID
		}
		b.WriteString(fmt.Sprintf("- [%s] %s\n", scope, m.Content))
	}
	return b.String()
}

func memoriesToEvidence(ms []domain.Memory) []domain.Evidence {
	out := make([]domain.Evidence, 0, len(ms))
	for _, m := range ms {
		scope := "global"
		if m.Scope == "product" {
			scope = m.ProductID
		}
		out = append(out, domain.Evidence{Type: "memory", Title: "记忆 · " + scope, Content: m.Content, Score: .5, Source: "memory/" + m.ID})
	}
	return out
}

// ExtractMemory 用大模型从一段文本（通常是诊断结论）里提炼一条可长期复用的记忆草稿，交由用户确认后保存。
func (s *Service) ExtractMemory(ctx context.Context, req domain.MemoryExtractRequest) (string, error) {
	text := strings.TrimSpace(req.Text)
	if text == "" {
		return "", fmt.Errorf("待提炼文本不能为空")
	}
	if s.LLM == nil || !s.LLM.Enabled() {
		return "", fmt.Errorf("未配置大模型，无法自动提炼")
	}
	sys := "你是运维知识沉淀助手。请从给定的诊断内容中提炼一条可长期复用的经验或环境事实，" +
		"要求：一句话、具体、去掉时间戳与偶发数值、聚焦可复用的因果或结论。只输出这句话本身，不要解释、不要引号、不要前缀。"
	user := "产品：" + req.ProductID + "\n诊断内容：\n" + text
	msg, err := s.LLM.Chat(ctx, []llm.Message{{Role: "system", Content: sys}, {Role: "user", Content: user}}, nil, false)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(strings.Trim(msg.Content, "\"'“”")), nil
}

// ---------- 关键词工具 ----------

func tokenize(s string) map[string]struct{} {
	s = strings.ToLower(s)
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r >= 0x4e00 && r <= 0x9fff)
	})
	set := make(map[string]struct{}, len(fields))
	for _, f := range fields {
		if len([]rune(f)) >= 2 { // 过滤过短噪声词
			set[f] = struct{}{}
		}
		// 中文按二元切分，缓解无分词器时的召回
		rs := []rune(f)
		for i := 0; i+1 < len(rs); i++ {
			if rs[i] >= 0x4e00 && rs[i] <= 0x9fff {
				set[string(rs[i:i+2])] = struct{}{}
			}
		}
	}
	return set
}

func overlap(a, b map[string]struct{}) int {
	n := 0
	for k := range a {
		if _, ok := b[k]; ok {
			n++
		}
	}
	return n
}
