package domain

import "time"

type Alert struct {
	ID        string    `json:"id"`
	ProductID string    `json:"product_id"`
	Rule      string    `json:"rule"`
	Severity  int       `json:"severity"`
	Target    string    `json:"target"`
	Value     string    `json:"value"`
	Triggered time.Time `json:"triggered_at"`
}

type Asset struct {
	ID        string `json:"id"`
	ProductID string `json:"product_id"`
	Kind      string `json:"kind"` // server | db
	Name      string `json:"name"`
	IP        string `json:"ip"`
	Detail    string `json:"detail"`
	Env       string `json:"env"`
	Status    string `json:"status"` // online | offline
}

type Evidence struct {
	Type    string  `json:"type"`
	Title   string  `json:"title"`
	Content string  `json:"content"`
	Score   float64 `json:"score"`
	Source  string  `json:"source"`
}

type ToolTrace struct {
	Tool       string `json:"tool"`
	Status     string `json:"status"`
	DurationMS int64  `json:"duration_ms"`
	Summary    string `json:"summary"`
}

type DiagnosisRequest struct {
	ProductID    string `json:"product_id" binding:"required"`
	Question     string `json:"question" binding:"required"`
	WindowMinute int    `json:"window_minutes"`
}

type Hypothesis struct {
	Rank       int     `json:"rank"`
	Cause      string  `json:"cause"`
	Confidence float64 `json:"confidence"`
}
type Action struct {
	Name             string `json:"name"`
	Risk             string `json:"risk"`
	RequiresApproval bool   `json:"requires_approval"`
}

type ApprovalRequest struct {
	ProductID string `json:"product_id" binding:"required"`
	Action    string `json:"action" binding:"required"`
	Risk      string `json:"risk"`
	Reason    string `json:"reason" binding:"required"`
	Source    string `json:"source"`
}

type ApprovalDecisionRequest struct {
	Decision string `json:"decision" binding:"required"` // approved | rejected
	Comment  string `json:"comment"`
}

type ApprovalExecutionRequest struct {
	Note string `json:"note"`
}

type Approval struct {
	ID            string    `json:"id"`
	ProductID     string    `json:"product_id"`
	Action        string    `json:"action"`
	Risk          string    `json:"risk"`
	Reason        string    `json:"reason"`
	Source        string    `json:"source"`
	Status        string    `json:"status"` // pending | approved | rejected | executed | cancelled
	RequesterID   string    `json:"requester_id"`
	RequesterName string    `json:"requester_name"`
	ReviewerID    string    `json:"reviewer_id"`
	ReviewerName  string    `json:"reviewer_name"`
	ReviewComment string    `json:"review_comment"`
	ExecutorID    string    `json:"executor_id"`
	ExecutorName  string    `json:"executor_name"`
	ExecutionNote string    `json:"execution_note"`
	CreatedAt     time.Time `json:"created_at"`
	ReviewedAt    time.Time `json:"reviewed_at"`
	ExecutedAt    time.Time `json:"executed_at"`
}

type DiagnosisResult struct {
	Question   string       `json:"question"`
	ProductID  string       `json:"product_id"`
	Summary    string       `json:"summary"`
	Confidence float64      `json:"confidence"`
	Hypotheses []Hypothesis `json:"hypotheses"`
	Actions    []Action     `json:"actions"`
	Evidence   []Evidence   `json:"evidence"`
	Alerts     []Alert      `json:"alerts"`
	Trace      []ToolTrace  `json:"trace"`
	Mode       string       `json:"mode"`
}

type DataSourceStatus struct {
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	Mode       string `json:"mode"`
	Configured bool   `json:"configured"`
	Endpoint   string `json:"endpoint"`
	Status     string `json:"status"` // ready | demo | unknown | error
	Message    string `json:"message"`
	LatencyMS  int64  `json:"latency_ms"`
}

type DiagnosisRun struct {
	ID              string    `json:"id"`
	ProductID       string    `json:"product_id"`
	Question        string    `json:"question"`
	Mode            string    `json:"mode"`
	Model           string    `json:"model"`
	Summary         string    `json:"summary"`
	Confidence      float64   `json:"confidence"`
	EvidenceCount   int       `json:"evidence_count"`
	AlertCount      int       `json:"alert_count"`
	ToolCallCount   int       `json:"tool_call_count"`
	FailedToolCount int       `json:"failed_tool_count"`
	KnowledgeHit    bool      `json:"knowledge_hit"`
	MemoryHit       bool      `json:"memory_hit"`
	AssetHit        bool      `json:"asset_hit"`
	DurationMS      int64     `json:"duration_ms"`
	AlertProvider   string    `json:"alert_provider"`
	LogProvider     string    `json:"log_provider"`
	KnowledgeMode   string    `json:"knowledge_mode"`
	EvidenceSources []string  `json:"evidence_sources"`
	Tools           []string  `json:"tools"`
	Username        string    `json:"username"`
	Included        bool      `json:"included"`
	GoldCause       string    `json:"gold_cause"`
	ReviewerNote    string    `json:"reviewer_note"`
	ReviewedBy      string    `json:"reviewed_by"`
	CreatedAt       time.Time `json:"created_at"`
	ReviewedAt      time.Time `json:"reviewed_at"`
}

type DiagnosisRunReviewRequest struct {
	Included  bool   `json:"included"`
	GoldCause string `json:"gold_cause"`
	Note      string `json:"note"`
}

type FaultCaseRequest struct {
	Name      string     `json:"name" binding:"required"`
	ProductID string     `json:"product_id" binding:"required"`
	Question  string     `json:"question" binding:"required"`
	GoldCause string     `json:"gold_cause" binding:"required"`
	Source    string     `json:"source"`
	Version   string     `json:"version"`
	Tags      []string   `json:"tags"`
	Alerts    []Alert    `json:"alerts"`
	Logs      []Evidence `json:"logs"`
	Assets    []Asset    `json:"assets"`
}

type FaultCase struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	ProductID string     `json:"product_id"`
	Question  string     `json:"question"`
	GoldCause string     `json:"gold_cause"`
	Source    string     `json:"source"` // real | synthetic | imported
	Version   string     `json:"version"`
	Tags      []string   `json:"tags"`
	Alerts    []Alert    `json:"alerts"`
	Logs      []Evidence `json:"logs"`
	Assets    []Asset    `json:"assets"`
	CreatedBy string     `json:"created_by"`
	CreatedAt time.Time  `json:"created_at"`
}

type ReplayRequest struct {
	Configs []string `json:"configs"` // full | bm25 | no-agent
}

type ReplayResult struct {
	ID            string          `json:"id"`
	CaseID        string          `json:"case_id"`
	Config        string          `json:"config"`
	Model         string          `json:"model"`
	JudgeModel    string          `json:"judge_model"`
	JudgeSource   string          `json:"judge_source"` // independent | self
	Diagnosis     DiagnosisResult `json:"diagnosis"`
	CauseCorrect  bool            `json:"cause_correct"`
	Faithfulness  float64         `json:"faithfulness"`
	Hallucination bool            `json:"hallucination"`
	Judged        bool            `json:"judged"`
	DurationMS    int64           `json:"duration_ms"`
	ToolFailures  int             `json:"tool_failures"`
	CreatedBy     string          `json:"created_by"`
	CreatedAt     time.Time       `json:"created_at"`
	ReviewStatus  string          `json:"review_status"` // pending | accepted | rejected
	ReviewCause   *bool           `json:"review_cause,omitempty"`
	ReviewNote    string          `json:"review_note"`
	ReviewedBy    string          `json:"reviewed_by"`
	ReviewedAt    time.Time       `json:"reviewed_at"`
}

type ReplayResultReviewRequest struct {
	Accepted bool   `json:"accepted"`
	CauseOK  bool   `json:"cause_ok"`
	Note     string `json:"note"`
}

type IssueRequest struct {
	ProductID string `json:"product_id" binding:"required"`
	Title     string `json:"title" binding:"required"`
	Diagnosis string `json:"diagnosis" binding:"required"`
}
type Issue struct {
	ID        string    `json:"id"`
	ProductID string    `json:"product_id"`
	Title     string    `json:"title"`
	Diagnosis string    `json:"diagnosis"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}
type AuditEvent struct {
	ID         string    `json:"id"`
	Action     string    `json:"action"`
	ProductID  string    `json:"product_id"`
	UserID     string    `json:"user_id"`
	Username   string    `json:"username"`
	Role       string    `json:"role"`
	Status     string    `json:"status"`
	DurationMS int64     `json:"duration_ms"`
	CreatedAt  time.Time `json:"created_at"`
}

// InspectionTask 是一条主动巡检任务：定时对某产品跑一次诊断。
type InspectionTask struct {
	ID          string    `json:"id"`
	ProductID   string    `json:"product_id"`
	Question    string    `json:"question"`
	IntervalSec int       `json:"interval_sec"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	LastRunAt   time.Time `json:"last_run_at"` // 零值表示尚未运行
	LastStatus  string    `json:"last_status"` // ok | warn | high | error
}
type InspectionTaskRequest struct {
	ProductID   string `json:"product_id" binding:"required"`
	Question    string `json:"question"`
	IntervalSec int    `json:"interval_sec"`
}

// InspectionReport 是一次巡检运行沉淀下来的结果。
type InspectionReport struct {
	ID         string    `json:"id"`
	TaskID     string    `json:"task_id"`
	ProductID  string    `json:"product_id"`
	Question   string    `json:"question"`
	Summary    string    `json:"summary"`
	Confidence float64   `json:"confidence"`
	Risk       string    `json:"risk"` // ok | warn | high
	DurationMS int64     `json:"duration_ms"`
	CreatedAt  time.Time `json:"created_at"`
}

// Memory 是 Agent 的一条长期记忆：从诊断中沉淀、可跨对话复用的经验或环境事实。
// Scope 目前支持 global（全局）与 product（限定某产品）；个人/团队作用域待认证模块接入后再细分。
type Memory struct {
	ID        string    `json:"id"`
	Scope     string    `json:"scope"`      // global | product
	ProductID string    `json:"product_id"` // scope=product 时生效
	Kind      string    `json:"kind"`       // fact | fix | preference
	Content   string    `json:"content"`
	Source    string    `json:"source"` // manual | extracted | diagnosis
	CreatedAt time.Time `json:"created_at"`
}
type MemoryRequest struct {
	Scope     string `json:"scope"`
	ProductID string `json:"product_id"`
	Kind      string `json:"kind"`
	Content   string `json:"content" binding:"required"`
	Source    string `json:"source"`
}
type MemoryExtractRequest struct {
	ProductID string `json:"product_id"`
	Text      string `json:"text" binding:"required"`
}

// User 是系统登录用户。Role 目前分 admin（管理员，可管理配置）与 viewer（只读，可查看与诊断）。
type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"` // 只存 bcrypt 哈希，绝不出现在任何响应里
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
}
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}
type LoginResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

// StreamEvent 是流式诊断过程中实时推送给前端的单条事件。
type StreamEvent struct {
	Type    string           `json:"type"`              // status | tool_call | tool_result | result | error | done
	Message string           `json:"message,omitempty"` // 人类可读的进度文字
	Tool    string           `json:"tool,omitempty"`    // 涉及的工具名
	Status  string           `json:"status,omitempty"`  // 工具执行状态：success | error
	Result  *DiagnosisResult `json:"result,omitempty"`  // type=result 时携带完整诊断结果
}
