package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"aiops-mvp/internal/domain"
	"aiops-mvp/internal/llm"
	"aiops-mvp/internal/storage"
	"aiops-mvp/internal/tools"
)

var replayConfigs = map[string]bool{"full": true, "bm25": true, "no-agent": true}

func (s *Service) CreateFaultCase(ctx context.Context, req domain.FaultCaseRequest) (domain.FaultCase, error) {
	if err := normalizeFaultCaseRequest(&req); err != nil {
		return domain.FaultCase{}, err
	}
	_, username, _ := actorFromContext(ctx)
	now := time.Now()
	v := domain.FaultCase{
		ID: fmt.Sprintf("CASE-%d", now.UnixNano()), Name: req.Name, ProductID: req.ProductID,
		Question: req.Question, GoldCause: req.GoldCause, Source: req.Source, Version: req.Version,
		Tags: req.Tags, Alerts: req.Alerts, Logs: req.Logs, Assets: req.Assets,
		CreatedBy: username, CreatedAt: now,
	}
	if err := s.Repo.CreateFaultCase(v); err != nil {
		s.addAudit(ctx, "create_fault_case", req.ProductID, "error", 0)
		return domain.FaultCase{}, err
	}
	s.addAudit(ctx, "create_fault_case", req.ProductID, "success", 0)
	return v, nil
}

func normalizeFaultCaseRequest(req *domain.FaultCaseRequest) error {
	req.Name = strings.TrimSpace(req.Name)
	req.ProductID = strings.TrimSpace(req.ProductID)
	req.Question = strings.TrimSpace(req.Question)
	req.GoldCause = strings.TrimSpace(req.GoldCause)
	req.Source = strings.ToLower(strings.TrimSpace(req.Source))
	req.Version = strings.TrimSpace(req.Version)
	if req.Name == "" || req.ProductID == "" || req.Question == "" || req.GoldCause == "" {
		return fmt.Errorf("案例名称、产品、诊断问题和标准根因不能为空")
	}
	if req.Version == "" {
		req.Version = "v1"
	}
	if req.Source == "" {
		req.Source = "imported"
	}
	if req.Source != "real" && req.Source != "synthetic" && req.Source != "imported" {
		return fmt.Errorf("案例来源只能是 real、synthetic 或 imported")
	}
	if len(req.Alerts)+len(req.Logs)+len(req.Assets) == 0 {
		return fmt.Errorf("案例至少需要一条告警、日志或资产证据")
	}
	seenTags := map[string]bool{}
	tags := make([]string, 0, len(req.Tags))
	for _, tag := range req.Tags {
		tag = strings.TrimSpace(tag)
		if tag != "" && !seenTags[tag] {
			tags = append(tags, tag)
			seenTags[tag] = true
		}
	}
	req.Tags = tags
	for i := range req.Alerts {
		if strings.TrimSpace(req.Alerts[i].ProductID) == "" {
			req.Alerts[i].ProductID = req.ProductID
		}
	}
	for i := range req.Assets {
		if strings.TrimSpace(req.Assets[i].ProductID) == "" {
			req.Assets[i].ProductID = req.ProductID
		}
	}
	for i := range req.Logs {
		if req.Logs[i].Type == "" {
			req.Logs[i].Type = "log"
		}
		if req.Logs[i].Title == "" {
			req.Logs[i].Title = fmt.Sprintf("回放日志 %d", i+1)
		}
		if req.Logs[i].Source == "" {
			req.Logs[i].Source = fmt.Sprintf("replay/log/%d", i+1)
		}
	}
	if req.Tags == nil {
		req.Tags = []string{}
	}
	if req.Alerts == nil {
		req.Alerts = []domain.Alert{}
	}
	if req.Logs == nil {
		req.Logs = []domain.Evidence{}
	}
	if req.Assets == nil {
		req.Assets = []domain.Asset{}
	}
	return nil
}

func (s *Service) ListFaultCases() ([]domain.FaultCase, error) { return s.Repo.ListFaultCases() }

func (s *Service) CreateFaultCases(ctx context.Context, requests []domain.FaultCaseRequest) ([]domain.FaultCase, error) {
	if len(requests) == 0 || len(requests) > 500 {
		return nil, fmt.Errorf("批量导入数量必须在 1 到 500 条之间")
	}
	for i := range requests {
		if err := normalizeFaultCaseRequest(&requests[i]); err != nil {
			return nil, fmt.Errorf("第 %d 条案例无效: %w", i+1, err)
		}
	}
	out := make([]domain.FaultCase, 0, len(requests))
	for _, req := range requests {
		v, err := s.CreateFaultCase(ctx, req)
		if err != nil {
			return out, err
		}
		out = append(out, v)
	}
	return out, nil
}

func (s *Service) GetFaultCase(id string) (domain.FaultCase, error) {
	return s.Repo.GetFaultCase(strings.TrimSpace(id))
}

func (s *Service) DeleteFaultCase(ctx context.Context, id string) error {
	v, err := s.GetFaultCase(id)
	if err != nil {
		return err
	}
	if err := s.Repo.DeleteFaultCase(v.ID); err != nil {
		s.addAudit(ctx, "delete_fault_case", v.ProductID, "error", 0)
		return err
	}
	s.addAudit(ctx, "delete_fault_case", v.ProductID, "success", 0)
	return nil
}

func (s *Service) ListReplayResults(caseID, batchID string, limit int) ([]domain.ReplayResult, error) {
	items, err := s.Repo.ListReplayResults(strings.TrimSpace(caseID), strings.TrimSpace(batchID), limit)
	if err != nil {
		return nil, err
	}
	for i := range items {
		assessReplayQuality(&items[i])
	}
	return items, nil
}

func (s *Service) ReviewReplayResult(ctx context.Context, id string, req domain.ReplayResultReviewRequest) (domain.ReplayResult, error) {
	v, err := s.Repo.GetReplayResult(strings.TrimSpace(id))
	if err != nil {
		return domain.ReplayResult{}, err
	}
	if strings.TrimSpace(req.Note) == "" {
		return domain.ReplayResult{}, fmt.Errorf("人工复核必须填写说明")
	}
	_, username, _ := actorFromContext(ctx)
	value := req.CauseOK
	v.ReviewStatus = "rejected"
	if req.Accepted {
		v.ReviewStatus = "accepted"
	}
	v.ReviewCause, v.ReviewNote, v.ReviewedBy, v.ReviewedAt = &value, strings.TrimSpace(req.Note), username, time.Now()
	if err := s.Repo.UpdateReplayResultReview(v); err != nil {
		return domain.ReplayResult{}, err
	}
	assessReplayQuality(&v)
	s.addAudit(ctx, "review_replay_result", v.Diagnosis.ProductID, "success", 0)
	return v, nil
}

func (s *Service) ReplayFaultCase(ctx context.Context, id string, req domain.ReplayRequest) ([]domain.ReplayResult, error) {
	if s.LLM == nil || !s.LLM.Enabled() {
		return nil, fmt.Errorf("回放实验需要配置可用的大模型")
	}
	c, err := s.GetFaultCase(id)
	if err != nil {
		return nil, err
	}
	configs, err := normalizeReplayConfigs(req.Configs)
	if err != nil {
		return nil, err
	}
	_, username, _ := actorFromContext(ctx)
	results := make([]domain.ReplayResult, 0, len(configs))
	for _, config := range configs {
		if err := ctx.Err(); err != nil {
			return results, err
		}
		result, err := s.executeReplay(ctx, c, config, "", 1, username)
		if err != nil {
			s.addAudit(ctx, "replay_fault_case", c.ProductID, "error", result.DurationMS)
			return results, err
		}
		results = append(results, result)
	}
	s.addAudit(ctx, "replay_fault_case", c.ProductID, "success", 0)
	return results, nil
}

func (s *Service) executeReplay(ctx context.Context, c domain.FaultCase, config, batchID string, trial int, username string) (domain.ReplayResult, error) {
	result := s.runReplayConfig(ctx, c, config)
	result.ID = fmt.Sprintf("REPLAY-%d", time.Now().UnixNano())
	result.CaseID, result.BatchID, result.Trial = c.ID, batchID, trial
	result.Config, result.Model = config, s.LLM.Model
	result.CreatedBy, result.CreatedAt = username, time.Now()
	result.JudgeModel, result.JudgeSource = s.judgeInfo()
	result.Faithfulness, result.Hallucination, result.CauseCorrect, result.Judged = s.judgeReplay(ctx, c, result.Diagnosis)
	assessReplayQuality(&result)
	return result, s.Repo.CreateReplayResult(result)
}

func assessReplayQuality(v *domain.ReplayResult) {
	issues := make([]string, 0, 3)
	evidenceCount := len(v.Diagnosis.Evidence)
	if !v.Judged {
		issues = append(issues, "judge_unavailable")
	}
	if v.Judged && evidenceCount == 0 && v.Faithfulness > 0 {
		issues = append(issues, "faithfulness_without_evidence")
	}
	if v.Judged && v.Faithfulness == 0 && !v.Hallucination {
		issues = append(issues, "zero_faithfulness_without_hallucination")
	}
	if v.Judged && evidenceCount == 0 && v.CauseCorrect {
		issues = append(issues, "correct_cause_without_evidence")
	}
	v.QualityStatus = "pass"
	if len(issues) > 0 {
		v.QualityStatus = "warning"
	}
	v.QualityIssues = issues
}

func (s *Service) judgeInfo() (string, string) {
	if s.Judge != nil && s.Judge.Enabled() {
		return s.Judge.Model, "independent"
	}
	if s.LLM != nil {
		return s.LLM.Model, "self"
	}
	return "", "unavailable"
}

func normalizeReplayConfigs(configs []string) ([]string, error) {
	if len(configs) == 0 {
		return []string{"full", "bm25", "no-agent"}, nil
	}
	out := make([]string, 0, len(configs))
	seen := map[string]bool{}
	for _, config := range configs {
		config = strings.ToLower(strings.TrimSpace(config))
		if !replayConfigs[config] {
			return nil, fmt.Errorf("未知回放配置: %s", config)
		}
		if !seen[config] {
			out = append(out, config)
			seen[config] = true
		}
	}
	return out, nil
}

func (s *Service) runReplayConfig(ctx context.Context, c domain.FaultCase, config string) domain.ReplayResult {
	started := time.Now()
	var diagnosis domain.DiagnosisResult
	if config == "no-agent" {
		diagnosis = s.runNoAgentReplay(ctx, c)
	} else {
		t := tools.NewService(s.Tools.Knowledge, tools.ReplayAlertProvider{Items: c.Alerts}, tools.ReplayLogProvider{Items: c.Logs})
		t.AssetsProvider = tools.ReplayAssetProvider{Items: c.Assets}
		if config == "full" {
			t.Embed = s.Tools.Embed
		}
		// Each replay uses an isolated repository so regular diagnosis runs and memories stay untouched.
		tmp := New(t, s.LLM, storage.NewMemory())
		runCtx, cancel := context.WithTimeout(ctx, 180*time.Second)
		diagnosis = tmp.Diagnose(runCtx, domain.DiagnosisRequest{ProductID: c.ProductID, Question: c.Question, WindowMinute: 30})
		cancel()
	}
	failures := 0
	for _, trace := range diagnosis.Trace {
		if trace.Status == "error" {
			failures++
		}
	}
	return domain.ReplayResult{Diagnosis: diagnosis, DurationMS: time.Since(started).Milliseconds(), ToolFailures: failures}
}

func (s *Service) runNoAgentReplay(ctx context.Context, c domain.FaultCase) domain.DiagnosisResult {
	system := `你是运维故障诊断助手。当前没有工具和外部证据，只能根据问题描述推断。只输出 JSON 对象：{"summary":"一句话结论","confidence":0.0,"hypotheses":[{"rank":1,"cause":"根因","confidence":0.0}],"actions":[{"name":"处置建议","risk":"low|medium|high","requires_approval":false}]}。高风险动作必须 requires_approval=true。`
	msg, err := s.LLM.Chat(ctx, []llm.Message{{Role: "system", Content: system}, {Role: "user", Content: fmt.Sprintf("产品：%s\n问题：%s", c.ProductID, c.Question)}}, nil, true)
	result := domain.DiagnosisResult{Question: c.Question, ProductID: c.ProductID, Mode: "no-agent", Evidence: []domain.Evidence{}, Alerts: []domain.Alert{}, Trace: []domain.ToolTrace{}, Hypotheses: []domain.Hypothesis{}, Actions: []domain.Action{}}
	if err != nil || msg == nil {
		result.Summary = "无证据基线调用失败"
		result.Trace = append(result.Trace, domain.ToolTrace{Tool: "llm_answer", Status: "error", Summary: fmt.Sprint(err)})
		return result
	}
	if parsed := parseDiagnosisJSON(msg.Content); parsed != nil {
		result.Summary, result.Confidence = parsed.Summary, parsed.Confidence
		result.Hypotheses, result.Actions = nonnullHypotheses(parsed.Hypotheses), enforceApproval(nonnullActions(parsed.Actions))
		result.Trace = append(result.Trace, domain.ToolTrace{Tool: "llm_answer", Status: "success", Summary: "无证据单轮回答"})
		return result
	}
	result.Summary = "无证据基线未返回有效结构"
	result.Trace = append(result.Trace, domain.ToolTrace{Tool: "llm_answer", Status: "error", Summary: "模型响应不是有效诊断 JSON"})
	return result
}

func (s *Service) judgeReplay(ctx context.Context, c domain.FaultCase, diagnosis domain.DiagnosisResult) (float64, bool, bool, bool) {
	judgeClient := s.LLM
	if s.Judge != nil && s.Judge.Enabled() {
		judgeClient = s.Judge
	}
	if judgeClient == nil || !judgeClient.Enabled() {
		return 0, false, false, false
	}
	var evidence strings.Builder
	for i, e := range diagnosis.Evidence {
		fmt.Fprintf(&evidence, "%d. [%s] %s\n", i+1, e.Title, e.Content)
	}
	if evidence.Len() == 0 {
		evidence.WriteString("（无可用证据）")
	}
	payload := fmt.Sprintf("【问题】%s\n【标准根因】%s\n【可用证据】\n%s\n【诊断结论】%s\n【根因假设】%s", c.Question, c.GoldCause, evidence.String(), diagnosis.Summary, toJSON(diagnosis.Hypotheses))
	system := `你是严格的运维诊断评审。只输出 JSON 对象：{"faithfulness":0.0,"has_hallucination":false,"cause_correct":false}。faithfulness 表示诊断论断被可用证据支持的比例；没有证据却给出具体断言应判为低忠实度和存在幻觉；cause_correct 仅在诊断识别出标准根因的核心机理时为 true。`
	judgeCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	msg, err := judgeClient.Chat(judgeCtx, []llm.Message{{Role: "system", Content: system}, {Role: "user", Content: payload}}, nil, true)
	if err != nil || msg == nil {
		return 0, false, false, false
	}
	var v struct {
		Faithfulness     float64 `json:"faithfulness"`
		HasHallucination bool    `json:"has_hallucination"`
		CauseCorrect     bool    `json:"cause_correct"`
	}
	if json.Unmarshal([]byte(strings.TrimSpace(msg.Content)), &v) != nil {
		return 0, false, false, false
	}
	if v.Faithfulness < 0 {
		v.Faithfulness = 0
	}
	if v.Faithfulness > 1 {
		v.Faithfulness = 1
	}
	return v.Faithfulness, v.HasHallucination, v.CauseCorrect, true
}
