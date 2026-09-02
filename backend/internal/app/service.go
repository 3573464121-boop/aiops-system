package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"aiops-mvp/internal/domain"
	"aiops-mvp/internal/llm"
	"aiops-mvp/internal/storage"
	"aiops-mvp/internal/tools"
)

type Service struct {
	Tools      *tools.Service
	LLM        *llm.Client
	Judge      *llm.Client
	Repo       storage.Repository
	inspectMu  sync.Mutex // 保证同一时刻只跑一个巡检任务，避免并发压垮大模型
	approvalMu sync.Mutex // 串行化审批状态迁移，避免重复审批或执行
}

func New(t *tools.Service, l *llm.Client, repos ...storage.Repository) *Service {
	var repo storage.Repository = storage.NewMemory()
	if len(repos) > 0 && repos[0] != nil {
		repo = repos[0]
	}
	return &Service{Tools: t, LLM: l, Repo: repo}
}

const maxToolRounds = 5

// Diagnose 一次性返回诊断结果（不推送过程事件）。
func (s *Service) Diagnose(ctx context.Context, req domain.DiagnosisRequest) domain.DiagnosisResult {
	return s.DiagnoseStream(ctx, req, func(domain.StreamEvent) {})
}

// DiagnoseStream 与 Diagnose 相同，但在诊断过程中通过 emit 实时推送进度事件。
// LLM 可用时走真正的工具调用循环（Agent），否则走本地启发式兜底。
func (s *Service) DiagnoseStream(ctx context.Context, req domain.DiagnosisRequest, emit func(domain.StreamEvent)) domain.DiagnosisResult {
	started := time.Now()
	if req.WindowMinute <= 0 || req.WindowMinute > 1440 {
		req.WindowMinute = 30
	}
	var result domain.DiagnosisResult
	if s.LLM != nil && s.LLM.Enabled() {
		result = s.diagnoseAgent(ctx, req, emit)
	} else {
		emit(domain.StreamEvent{Type: "status", Message: "未配置大模型，使用本地启发式诊断"})
		result = s.diagnoseHeuristic(ctx, req)
	}
	s.recordDiagnosisRun(ctx, req, result, time.Since(started))
	return result
}

// diagnoseAgent 让模型自主决定调用哪些只读工具（最多 maxToolRounds 轮）收集证据，再产出结构化诊断。
func (s *Service) diagnoseAgent(ctx context.Context, req domain.DiagnosisRequest, emit func(domain.StreamEvent)) domain.DiagnosisResult {
	started := time.Now()
	emit(domain.StreamEvent{Type: "status", Message: "开始智能诊断，模型正在分析并决定调用哪些工具…"})
	trace := make([]domain.ToolTrace, 0, 8)
	evidence := make([]domain.Evidence, 0, 8)
	alerts := make([]domain.Alert, 0, 4)

	messages := []llm.Message{
		{Role: "system", Content: agentSystemPrompt()},
	}
	// 召回相关的长期记忆，作为背景注入，并计入证据。
	if recalled := s.recallMemories(req.ProductID, req.Question, maxRecall); len(recalled) > 0 {
		messages = append(messages, llm.Message{Role: "system", Content: memoryNote(recalled)})
		evidence = append(evidence, memoriesToEvidence(recalled)...)
		emit(domain.StreamEvent{Type: "status", Message: fmt.Sprintf("已召回 %d 条相关记忆", len(recalled))})
	}
	messages = append(messages, llm.Message{Role: "user", Content: fmt.Sprintf("产品标识：%s\n诊断问题：%s\n时间窗口：最近 %d 分钟", req.ProductID, req.Question, req.WindowMinute)})
	toolDefs := agentTools()

	var finalContent string
	for round := 0; round < maxToolRounds; round++ {
		msg, err := s.LLM.Chat(ctx, messages, toolDefs, false)
		if err != nil {
			emit(domain.StreamEvent{Type: "status", Message: "模型调用失败，降级为本地启发式诊断"})
			trace = append(trace, domain.ToolTrace{Tool: "llm_chat", Status: "error", DurationMS: time.Since(started).Milliseconds(), Summary: err.Error()})
			return s.mergeHeuristic(ctx, req, trace)
		}
		if len(msg.ToolCalls) == 0 {
			finalContent = strings.TrimSpace(msg.Content)
			emit(domain.StreamEvent{Type: "status", Message: "证据收集完成，正在生成诊断结论…"})
			break
		}
		messages = append(messages, *msg)
		for _, tc := range msg.ToolCalls {
			emit(domain.StreamEvent{Type: "tool_call", Tool: tc.Function.Name, Message: "调用工具 " + tc.Function.Name})
			t0 := time.Now()
			out, summary, status, a, evs := s.runToolCall(tc)
			trace = append(trace, domain.ToolTrace{Tool: tc.Function.Name, Status: status, DurationMS: time.Since(t0).Milliseconds(), Summary: summary})
			alerts = append(alerts, a...)
			evidence = append(evidence, evs...)
			emit(domain.StreamEvent{Type: "tool_result", Tool: tc.Function.Name, Status: status, Message: summary})
			messages = append(messages, llm.Message{Role: "tool", ToolCallID: tc.ID, Name: tc.Function.Name, Content: out})
		}
	}

	result := domain.DiagnosisResult{
		Question:   req.Question,
		ProductID:  req.ProductID,
		Evidence:   dedupEvidence(evidence),
		Alerts:     alerts,
		Trace:      trace,
		Mode:       "agent",
		Hypotheses: []domain.Hypothesis{},
		Actions:    []domain.Action{},
	}

	if parsed := parseDiagnosisJSON(finalContent); parsed != nil {
		result.Summary = parsed.Summary
		result.Confidence = parsed.Confidence
		result.Hypotheses = nonnullHypotheses(parsed.Hypotheses)
		result.Actions = enforceApproval(nonnullActions(parsed.Actions))
		result.Trace = append(result.Trace, domain.ToolTrace{Tool: "llm_answer", Status: "success", Summary: "模型基于证据给出结构化诊断"})
	} else if lr, err := s.LLM.Synthesize(ctx, req, evidence, alerts); err == nil {
		// 兜底：模型最终没有给出合法 JSON，则用已收集证据强制生成一次结构化诊断。
		result.Summary = lr.Summary
		result.Confidence = lr.Confidence
		result.Hypotheses = nonnullHypotheses(lr.Hypotheses)
		result.Actions = enforceApproval(nonnullActions(lr.Actions))
		result.Mode = "agent+synth"
		result.Trace = append(result.Trace, domain.ToolTrace{Tool: "llm_synthesis", Status: "success", Summary: "由已收集证据强制生成结构化诊断"})
	} else {
		result.Summary = "已收集证据，但模型未能给出结构化结论，请人工复核以下证据。"
		result.Confidence = .3
		result.Trace = append(result.Trace, domain.ToolTrace{Tool: "llm_synthesis", Status: "fallback", Summary: err.Error()})
	}

	s.addAudit(ctx, "diagnose", req.ProductID, "success", time.Since(started).Milliseconds())
	return result
}

// runToolCall 校验并执行一次工具调用，返回：喂回模型的结果串、审计摘要、状态、收集到的告警与证据。
func (s *Service) runToolCall(tc llm.ToolCall) (out, summary, status string, alerts []domain.Alert, evs []domain.Evidence) {
	args := map[string]any{}
	_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)
	str := func(k string) string {
		if v, ok := args[k].(string); ok {
			return strings.TrimSpace(v)
		}
		return ""
	}
	switch tc.Function.Name {
	case "get_alerts":
		a, err := s.Tools.Alerts(str("product_id"))
		if err != nil {
			return toolErr(err.Error()), err.Error(), "error", nil, nil
		}
		return toJSON(a), fmt.Sprintf("命中 %d 条活跃告警", len(a)), "success", a, nil
	case "search_logs":
		pid := str("product_id")
		if pid == "" {
			return toolErr("缺少必填参数 product_id"), "缺少必填参数 product_id", "error", nil, nil
		}
		l, err := s.Tools.Logs(pid, str("query"))
		if err != nil {
			return toolErr(err.Error()), err.Error(), "error", nil, nil
		}
		return toJSON(l), fmt.Sprintf("命中 %d 条异常日志", len(l)), "success", nil, l
	case "search_knowledge":
		q := str("query")
		if q == "" {
			return toolErr("缺少必填参数 query"), "缺少必填参数 query", "error", nil, nil
		}
		limit := 3
		if f, ok := args["limit"].(float64); ok && int(f) > 0 && int(f) <= 20 {
			limit = int(f)
		}
		k := s.Tools.SearchKnowledge(q, limit)
		return toJSON(k), fmt.Sprintf("命中 %d 条知识片段", len(k)), "success", nil, k
	case "get_assets":
		as, err := s.Tools.Assets(str("product_id"))
		if err != nil {
			return toolErr(err.Error()), err.Error(), "error", nil, nil
		}
		return toJSON(as), fmt.Sprintf("命中 %d 个资产", len(as)), "success", nil, assetsToEvidence(as)
	case "lookup_ip":
		ip := str("ip")
		if ip == "" {
			return toolErr("缺少必填参数 ip"), "缺少必填参数 ip", "error", nil, nil
		}
		as, err := s.Tools.LookupIP(ip)
		if err != nil {
			return toolErr(err.Error()), err.Error(), "error", nil, nil
		}
		return toJSON(as), fmt.Sprintf("IP %s 命中 %d 个资产", ip, len(as)), "success", nil, assetsToEvidence(as)
	case "recall_memory":
		q := str("query")
		if q == "" {
			return toolErr("缺少必填参数 query"), "缺少必填参数 query", "error", nil, nil
		}
		ms := s.recallMemories(str("product_id"), q, maxRecall)
		return toJSON(ms), fmt.Sprintf("召回 %d 条记忆", len(ms)), "success", nil, memoriesToEvidence(ms)
	default:
		msg := "未知工具：" + tc.Function.Name
		return toolErr(msg), msg, "error", nil, nil
	}
}

// diagnoseHeuristic 无 LLM 时的本地兜底。
func (s *Service) diagnoseHeuristic(ctx context.Context, req domain.DiagnosisRequest) domain.DiagnosisResult {
	return s.mergeHeuristic(ctx, req, make([]domain.ToolTrace, 0, 4))
}

// mergeHeuristic 在已有 trace 基础上追加固定顺序的证据收集与保守判断（Agent 失败时复用）。
func (s *Service) mergeHeuristic(ctx context.Context, req domain.DiagnosisRequest, trace []domain.ToolTrace) domain.DiagnosisResult {
	started := time.Now()
	t := time.Now()
	alerts, alertErr := s.Tools.Alerts(req.ProductID)
	alertStatus, alertSummary := "success", fmt.Sprintf("命中 %d 条活跃告警", len(alerts))
	if alertErr != nil {
		alerts = []domain.Alert{}
		alertStatus, alertSummary = "error", alertErr.Error()
	}
	trace = append(trace, domain.ToolTrace{Tool: "get_alerts", Status: alertStatus, DurationMS: time.Since(t).Milliseconds(), Summary: alertSummary})

	t = time.Now()
	logs, logErr := s.Tools.Logs(req.ProductID, req.Question)
	logStatus, logSummary := "success", fmt.Sprintf("命中 %d 条异常日志", len(logs))
	if logErr != nil {
		logs = []domain.Evidence{}
		logStatus, logSummary = "error", logErr.Error()
	}
	trace = append(trace, domain.ToolTrace{Tool: "search_logs", Status: logStatus, DurationMS: time.Since(t).Milliseconds(), Summary: logSummary})

	t = time.Now()
	knowledge := s.Tools.SearchKnowledge(req.Question, 3)
	trace = append(trace, domain.ToolTrace{Tool: "search_knowledge", Status: "success", DurationMS: time.Since(t).Milliseconds(), Summary: fmt.Sprintf("命中 %d 条知识片段", len(knowledge))})

	evidence := make([]domain.Evidence, 0, 1+len(logs)+len(knowledge))
	evidence = append(evidence, domain.Evidence{Type: "alert", Title: "活跃告警", Content: fmt.Sprintf("命中 %d 条产品告警，时间窗口 %d 分钟", len(alerts), req.WindowMinute), Score: 1, Source: "get_alerts"})
	evidence = append(evidence, logs...)
	evidence = append(evidence, knowledge...)

	result := domain.DiagnosisResult{
		Question: req.Question, ProductID: req.ProductID,
		Summary: "证据不足，建议先核对告警目标、时间窗口和关联日志。", Confidence: .35,
		Hypotheses: []domain.Hypothesis{}, Actions: []domain.Action{{Name: "核对告警目标及同时间窗口指标", Risk: "low"}},
		Evidence: evidence, Alerts: alerts, Trace: trace, Mode: "heuristic",
	}
	if len(logs) > 0 {
		result.Summary = "当前证据指向上游依赖响应变慢，并伴随连接池等待。"
		result.Confidence = .72
		result.Hypotheses = []domain.Hypothesis{{Rank: 1, Cause: "上游依赖超时", Confidence: .72}, {Rank: 2, Cause: "连接池容量接近上限", Confidence: .61}}
		result.Actions = []domain.Action{{Name: "核对上游服务延迟与错误率", Risk: "low"}, {Name: "检查连接池使用率和等待时间", Risk: "low"}, {Name: "回滚最近一次发布", Risk: "high", RequiresApproval: true}}
	}
	s.addAudit(ctx, "diagnose", req.ProductID, "success", time.Since(started).Milliseconds())
	return result
}

var approvalStatuses = map[string]bool{
	"pending": true, "approved": true, "rejected": true, "executed": true, "cancelled": true,
}

func (s *Service) CreateApproval(ctx context.Context, req domain.ApprovalRequest) (domain.Approval, error) {
	req.ProductID = strings.TrimSpace(req.ProductID)
	req.Action = strings.TrimSpace(req.Action)
	req.Reason = strings.TrimSpace(req.Reason)
	req.Risk = strings.ToLower(strings.TrimSpace(req.Risk))
	req.Source = strings.ToLower(strings.TrimSpace(req.Source))
	if req.ProductID == "" || req.Action == "" || req.Reason == "" {
		return domain.Approval{}, fmt.Errorf("产品、处置动作和申请理由不能为空")
	}
	if req.Risk == "" {
		req.Risk = "high"
	}
	if req.Risk != "low" && req.Risk != "medium" && req.Risk != "high" {
		return domain.Approval{}, fmt.Errorf("风险级别必须是 low、medium 或 high")
	}
	if req.Source == "" {
		req.Source = "manual"
	}
	userID, username, _ := actorFromContext(ctx)
	v := domain.Approval{
		ID: fmt.Sprintf("APR-%d", time.Now().UnixNano()), ProductID: req.ProductID,
		Action: req.Action, Risk: req.Risk, Reason: req.Reason, Source: req.Source,
		Status: "pending", RequesterID: userID, RequesterName: username, CreatedAt: time.Now(),
	}
	if err := s.Repo.CreateApproval(v); err != nil {
		s.addAudit(ctx, "create_approval", req.ProductID, "error", 0)
		return domain.Approval{}, err
	}
	s.addAudit(ctx, "create_approval", req.ProductID, "success", 0)
	return v, nil
}

func (s *Service) ListApprovals(status string) ([]domain.Approval, error) {
	status = strings.ToLower(strings.TrimSpace(status))
	if status != "" && !approvalStatuses[status] {
		return nil, fmt.Errorf("无效的审批状态")
	}
	return s.Repo.ListApprovals(status)
}

func (s *Service) GetApproval(id string) (domain.Approval, error) {
	return s.Repo.GetApproval(strings.TrimSpace(id))
}

func (s *Service) ReviewApproval(ctx context.Context, id string, req domain.ApprovalDecisionRequest) (domain.Approval, error) {
	s.approvalMu.Lock()
	defer s.approvalMu.Unlock()
	v, err := s.Repo.GetApproval(strings.TrimSpace(id))
	if err != nil {
		return domain.Approval{}, err
	}
	if v.Status != "pending" {
		return domain.Approval{}, fmt.Errorf("只有待审批的申请可以审核，当前状态为 %s", v.Status)
	}
	decision := strings.ToLower(strings.TrimSpace(req.Decision))
	comment := strings.TrimSpace(req.Comment)
	if decision != "approved" && decision != "rejected" {
		return domain.Approval{}, fmt.Errorf("审批决定必须是 approved 或 rejected")
	}
	if decision == "rejected" && comment == "" {
		return domain.Approval{}, fmt.Errorf("驳回时必须填写原因")
	}
	userID, username, _ := actorFromContext(ctx)
	v.Status, v.ReviewerID, v.ReviewerName = decision, userID, username
	v.ReviewComment, v.ReviewedAt = comment, time.Now()
	if err := s.Repo.UpdateApproval(v); err != nil {
		s.addAudit(ctx, "review_approval", v.ProductID, "error", 0)
		return domain.Approval{}, err
	}
	s.addAudit(ctx, "review_approval", v.ProductID, "success", 0)
	return v, nil
}

func (s *Service) ExecuteApproval(ctx context.Context, id string, req domain.ApprovalExecutionRequest) (domain.Approval, error) {
	s.approvalMu.Lock()
	defer s.approvalMu.Unlock()
	v, err := s.Repo.GetApproval(strings.TrimSpace(id))
	if err != nil {
		return domain.Approval{}, err
	}
	if v.Status != "approved" {
		return domain.Approval{}, fmt.Errorf("只有已批准的申请可以确认执行，当前状态为 %s", v.Status)
	}
	userID, username, _ := actorFromContext(ctx)
	v.Status, v.ExecutorID, v.ExecutorName = "executed", userID, username
	v.ExecutionNote, v.ExecutedAt = strings.TrimSpace(req.Note), time.Now()
	if err := s.Repo.UpdateApproval(v); err != nil {
		s.addAudit(ctx, "execute_approval", v.ProductID, "error", 0)
		return domain.Approval{}, err
	}
	s.addAudit(ctx, "execute_approval", v.ProductID, "success", 0)
	return v, nil
}

func (s *Service) CancelApproval(ctx context.Context, id string) (domain.Approval, error) {
	s.approvalMu.Lock()
	defer s.approvalMu.Unlock()
	v, err := s.Repo.GetApproval(strings.TrimSpace(id))
	if err != nil {
		return domain.Approval{}, err
	}
	if v.Status != "pending" {
		return domain.Approval{}, fmt.Errorf("只有待审批的申请可以撤销")
	}
	userID, _, role := actorFromContext(ctx)
	if role != "admin" && (userID == "" || userID != v.RequesterID) {
		return domain.Approval{}, fmt.Errorf("只能撤销自己发起的申请")
	}
	v.Status = "cancelled"
	if err := s.Repo.UpdateApproval(v); err != nil {
		s.addAudit(ctx, "cancel_approval", v.ProductID, "error", 0)
		return domain.Approval{}, err
	}
	s.addAudit(ctx, "cancel_approval", v.ProductID, "success", 0)
	return v, nil
}

// ---------- Agent 相关辅助 ----------

func agentSystemPrompt() string {
	return `你是一个只读的 AIOps 故障诊断助手。请针对用户描述的告警/故障，先调用工具收集真实证据，再给出有依据的诊断。

可用工具：
- get_alerts(product_id?)：查询活跃告警
- search_logs(product_id, query?)：搜索异常日志
- search_knowledge(query, limit?)：检索运维知识库/历史故障
- get_assets(product_id?)：查询资产清单（服务器 / 数据库实例）
- lookup_ip(ip)：按 IP 反查所属资产与产品归属
- recall_memory(query, product_id?)：检索历史经验/环境记忆

要求：
1. 必须先调用工具收集证据，不要凭空臆测。一般先看告警，再按线索搜日志，必要时查知识库。
2. 最多 5 轮工具调用，证据足够后立即停止调用。
3. 得出结论后，只输出一个 JSON 对象（不要任何多余文字、不要用 markdown 代码块）：
{"summary":"一句话根因结论","confidence":0.0到1.0的小数,"hypotheses":[{"rank":1,"cause":"根因假设","confidence":0.7}],"actions":[{"name":"处置建议","risk":"low|medium|high","requires_approval":false}]}
4. 只能依据工具返回的证据判断，不得捏造。回滚、重启、删库、扩容等高风险动作必须 risk=high 且 requires_approval=true。`
}

func agentTools() []llm.Tool {
	return []llm.Tool{
		{Type: "function", Function: llm.FunctionDef{
			Name:        "get_alerts",
			Description: "查询当前活跃告警，可按产品标识过滤。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"product_id": map[string]any{"type": "string", "description": "产品标识，如 payment。留空表示查询全部产品"},
				},
			},
		}},
		{Type: "function", Function: llm.FunctionDef{
			Name:        "search_logs",
			Description: "按产品和关键词搜索最近的异常日志。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"product_id": map[string]any{"type": "string", "description": "产品标识，必填，如 payment"},
					"query":      map[string]any{"type": "string", "description": "检索关键词，如 timeout、error"},
				},
				"required": []string{"product_id"},
			},
		}},
		{Type: "function", Function: llm.FunctionDef{
			Name:        "search_knowledge",
			Description: "检索运维知识库/历史故障文档，返回带引用来源的片段。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{"type": "string", "description": "检索关键词，必填"},
					"limit": map[string]any{"type": "integer", "description": "返回条数，默认 3，最大 20"},
				},
				"required": []string{"query"},
			},
		}},
		{Type: "function", Function: llm.FunctionDef{
			Name:        "get_assets",
			Description: "查询产品的资产清单（服务器与数据库实例）。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"product_id": map[string]any{"type": "string", "description": "产品标识，留空表示全部"},
				},
			},
		}},
		{Type: "function", Function: llm.FunctionDef{
			Name:        "lookup_ip",
			Description: "按 IP 反查所属资产与产品归属。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"ip": map[string]any{"type": "string", "description": "IPv4 地址，必填"},
				},
				"required": []string{"ip"},
			},
		}},
		{Type: "function", Function: llm.FunctionDef{
			Name:        "recall_memory",
			Description: "检索沉淀下来的历史经验/环境记忆（如某产品的容量上限、既往根因），按相关度返回。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query":      map[string]any{"type": "string", "description": "检索关键词，必填"},
					"product_id": map[string]any{"type": "string", "description": "产品标识，可选，用于优先召回该产品的记忆"},
				},
				"required": []string{"query"},
			},
		}},
	}
}

type diagnosisJSON struct {
	Summary    string              `json:"summary"`
	Confidence float64             `json:"confidence"`
	Hypotheses []domain.Hypothesis `json:"hypotheses"`
	Actions    []domain.Action     `json:"actions"`
}

// parseDiagnosisJSON 从模型最终回复中稳健地提取诊断 JSON（容忍代码围栏与前后杂字）。
func parseDiagnosisJSON(s string) *diagnosisJSON {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	if strings.HasPrefix(s, "```") {
		if i := strings.Index(s, "\n"); i >= 0 {
			s = s[i+1:]
		}
		s = strings.TrimSuffix(strings.TrimSpace(s), "```")
	}
	l, r := strings.Index(s, "{"), strings.LastIndex(s, "}")
	if l < 0 || r <= l {
		return nil
	}
	var d diagnosisJSON
	if err := json.Unmarshal([]byte(s[l:r+1]), &d); err != nil {
		return nil
	}
	if strings.TrimSpace(d.Summary) == "" {
		return nil
	}
	return &d
}

func enforceApproval(actions []domain.Action) []domain.Action {
	for i := range actions {
		if strings.EqualFold(actions[i].Risk, "high") {
			actions[i].RequiresApproval = true
		}
	}
	return actions
}

func toJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return `{"error":"marshal failed"}`
	}
	return string(b)
}

func toolErr(msg string) string {
	b, _ := json.Marshal(map[string]string{"error": msg})
	return string(b)
}

// dedupEvidence 按 Source 去重（同一条记忆可能既被自动注入又被 recall_memory 工具召回）。空 Source 一律保留。
func dedupEvidence(evs []domain.Evidence) []domain.Evidence {
	seen := make(map[string]struct{}, len(evs))
	out := make([]domain.Evidence, 0, len(evs))
	for _, e := range evs {
		if e.Source != "" {
			if _, ok := seen[e.Source]; ok {
				continue
			}
			seen[e.Source] = struct{}{}
		}
		out = append(out, e)
	}
	return out
}

func assetsToEvidence(as []domain.Asset) []domain.Evidence {
	out := make([]domain.Evidence, 0, len(as))
	for _, a := range as {
		out = append(out, domain.Evidence{Type: "asset", Title: a.Name, Content: fmt.Sprintf("%s · %s · IP %s · %s · %s", a.ProductID, a.Detail, a.IP, a.Env, a.Status), Score: .6, Source: "get_assets/" + a.ID})
	}
	return out
}

func nonnullHypotheses(v []domain.Hypothesis) []domain.Hypothesis {
	if v == nil {
		return []domain.Hypothesis{}
	}
	return v
}

func nonnullActions(v []domain.Action) []domain.Action {
	if v == nil {
		return []domain.Action{}
	}
	return v
}

// ---------- 工单与审计 ----------

func (s *Service) CreateIssue(ctx context.Context, req domain.IssueRequest) (domain.Issue, error) {
	i := domain.Issue{ID: fmt.Sprintf("ISS-%d", time.Now().UnixNano()), ProductID: req.ProductID, Title: req.Title, Diagnosis: req.Diagnosis, Status: "open", CreatedAt: time.Now()}
	if err := s.Repo.CreateIssue(i); err != nil {
		s.addAudit(ctx, "create_issue", req.ProductID, "error", 0)
		return domain.Issue{}, err
	}
	s.addAudit(ctx, "create_issue", req.ProductID, "success", 0)
	return i, nil
}

func (s *Service) Issues() ([]domain.Issue, error)      { return s.Repo.ListIssues() }
func (s *Service) Audits() ([]domain.AuditEvent, error) { return s.Repo.ListAudits(100) }

func (s *Service) addAudit(ctx context.Context, action, pid, status string, d int64) {
	userID, username, role := actorFromContext(ctx)
	_ = s.Repo.AddAudit(domain.AuditEvent{
		ID:         fmt.Sprintf("AUD-%d", time.Now().UnixNano()),
		Action:     action,
		ProductID:  pid,
		UserID:     userID,
		Username:   username,
		Role:       role,
		Status:     status,
		DurationMS: d,
		CreatedAt:  time.Now(),
	})
}
