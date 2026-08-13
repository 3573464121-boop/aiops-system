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
	Status     string    `json:"status"`
	DurationMS int64     `json:"duration_ms"`
	CreatedAt  time.Time `json:"created_at"`
}

// StreamEvent 是流式诊断过程中实时推送给前端的单条事件。
type StreamEvent struct {
	Type    string           `json:"type"`              // status | tool_call | tool_result | result | error | done
	Message string           `json:"message,omitempty"` // 人类可读的进度文字
	Tool    string           `json:"tool,omitempty"`    // 涉及的工具名
	Status  string           `json:"status,omitempty"`  // 工具执行状态：success | error
	Result  *DiagnosisResult `json:"result,omitempty"`  // type=result 时携带完整诊断结果
}
