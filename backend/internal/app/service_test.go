package app

import (
	"context"
	"testing"

	"aiops-mvp/internal/domain"
	"aiops-mvp/internal/tools"
)

func TestDiagnoseHandlesEmptyEvidence(t *testing.T) {
	s := New(tools.NewService(nil, emptyAlerts{}, emptyLogs{}), nil)
	got := s.Diagnose(context.Background(), domain.DiagnosisRequest{ProductID: "unknown", Question: "完全无匹配内容", WindowMinute: 30})
	if got.Evidence == nil {
		t.Fatal("evidence must be a JSON array")
	}
	if got.Hypotheses == nil {
		t.Fatal("hypotheses must be a JSON array")
	}
	if got.Summary == "" {
		t.Fatal("summary required")
	}
	runs, err := s.ListDiagnosisRuns(10)
	if err != nil || len(runs) != 1 {
		t.Fatalf("diagnosis run must be recorded: len=%d err=%v", len(runs), err)
	}
	if runs[0].ProductID != "unknown" || runs[0].Mode == "" || runs[0].EvidenceSources == nil || runs[0].Tools == nil {
		t.Fatalf("invalid diagnosis run: %+v", runs[0])
	}
}
func TestCreateIssueAndAudit(t *testing.T) {
	s := New(tools.NewService(nil, nil, nil), nil)
	i, err := s.CreateIssue(context.Background(), domain.IssueRequest{ProductID: "payment", Title: "test", Diagnosis: "d"})
	if err != nil {
		t.Fatal(err)
	}
	issues, err := s.Issues()
	if err != nil {
		t.Fatal(err)
	}
	audits, err := s.Audits()
	if err != nil {
		t.Fatal(err)
	}
	if i.ID == "" || len(issues) != 1 || len(audits) != 1 {
		t.Fatal("issue or audit not persisted")
	}
}

func TestApprovalLifecycle(t *testing.T) {
	s := New(tools.NewService(nil, nil, nil), nil)
	requester := WithActor(context.Background(), "USR-viewer", "viewer1", "viewer")
	admin := WithActor(context.Background(), "USR-admin", "admin", "admin")

	v, err := s.CreateApproval(requester, domain.ApprovalRequest{
		ProductID: "payment", Action: "回滚最近一次发布", Risk: "high", Reason: "错误率在发布后持续升高", Source: "diagnosis",
	})
	if err != nil {
		t.Fatal(err)
	}
	if v.Status != "pending" || v.RequesterName != "viewer1" {
		t.Fatalf("unexpected created approval: %+v", v)
	}
	if _, err := s.ExecuteApproval(admin, v.ID, domain.ApprovalExecutionRequest{}); err == nil {
		t.Fatal("pending approval must not be executable")
	}
	v, err = s.ReviewApproval(admin, v.ID, domain.ApprovalDecisionRequest{Decision: "approved", Comment: "已核对变更窗口"})
	if err != nil {
		t.Fatal(err)
	}
	if v.Status != "approved" || v.ReviewerName != "admin" {
		t.Fatalf("unexpected reviewed approval: %+v", v)
	}
	plan, err := s.PreviewApprovalExecution(v.ID)
	if err != nil || !plan.Allowed || plan.Mode != "simulate" || plan.Kind != "rollback_release" {
		t.Fatalf("unexpected execution plan: %+v err=%v", plan, err)
	}
	v, err = s.ExecuteApproval(admin, v.ID, domain.ApprovalExecutionRequest{Note: "人工回滚完成", ConfirmAction: v.Action})
	if err != nil {
		t.Fatal(err)
	}
	if v.Status != "executed" || v.ExecutorName != "admin" || v.ExecutionNote == "" || v.ExecutionMode != "simulate" || v.ExecutionOutput == "" {
		t.Fatalf("unexpected executed approval: %+v", v)
	}
	if _, err := s.ReviewApproval(admin, v.ID, domain.ApprovalDecisionRequest{Decision: "approved"}); err == nil {
		t.Fatal("executed approval must not be reviewed again")
	}
}

func TestApprovalRejectRequiresComment(t *testing.T) {
	s := New(tools.NewService(nil, nil, nil), nil)
	requester := WithActor(context.Background(), "USR-viewer", "viewer1", "viewer")
	admin := WithActor(context.Background(), "USR-admin", "admin", "admin")
	v, err := s.CreateApproval(requester, domain.ApprovalRequest{ProductID: "payment", Action: "扩容", Risk: "high", Reason: "容量不足"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReviewApproval(admin, v.ID, domain.ApprovalDecisionRequest{Decision: "rejected"}); err == nil {
		t.Fatal("rejection without comment must fail")
	}
}

func TestDiagnosisRunReviewRequiresGoldCause(t *testing.T) {
	s := New(tools.NewService(nil, emptyAlerts{}, emptyLogs{}), nil)
	ctx := WithActor(context.Background(), "USR-admin", "admin", "admin")
	s.Diagnose(ctx, domain.DiagnosisRequest{ProductID: "payment", Question: "test"})
	runs, err := s.ListDiagnosisRuns(10)
	if err != nil || len(runs) != 1 {
		t.Fatalf("missing diagnosis run: %v", err)
	}
	if _, err := s.ReviewDiagnosisRun(ctx, runs[0].ID, domain.DiagnosisRunReviewRequest{Included: true}); err == nil {
		t.Fatal("included run without gold cause must fail")
	}
	v, err := s.ReviewDiagnosisRun(ctx, runs[0].ID, domain.DiagnosisRunReviewRequest{Included: true, GoldCause: "上游依赖超时", Note: "人工复核"})
	if err != nil || !v.Included || v.ReviewedBy != "admin" {
		t.Fatalf("review failed: %+v err=%v", v, err)
	}
}

func TestFaultCaseLifecycleAndValidation(t *testing.T) {
	s := New(tools.NewService(nil, nil, nil), nil)
	ctx := WithActor(context.Background(), "USR-admin", "admin", "admin")
	if _, err := s.CreateFaultCase(ctx, domain.FaultCaseRequest{Name: "empty"}); err == nil {
		t.Fatal("incomplete fault case must be rejected")
	}
	v, err := s.CreateFaultCase(ctx, domain.FaultCaseRequest{
		Name: "database timeout", ProductID: "payment", Question: "why timeout", GoldCause: "connection pool exhausted",
		Logs: []domain.Evidence{{Content: "waited for connection"}}, Tags: []string{"db", "db", " "},
	})
	if err != nil {
		t.Fatal(err)
	}
	if v.Version != "v1" || v.Source != "imported" || v.CreatedBy != "admin" || len(v.Tags) != 1 {
		t.Fatalf("fault case was not normalized: %+v", v)
	}
	if v.Logs[0].Type != "log" || v.Logs[0].Source == "" {
		t.Fatalf("log evidence was not normalized: %+v", v.Logs[0])
	}
	items, err := s.ListFaultCases()
	if err != nil || len(items) != 1 {
		t.Fatalf("list fault cases: len=%d err=%v", len(items), err)
	}
	if err := s.DeleteFaultCase(ctx, v.ID); err != nil {
		t.Fatal(err)
	}
	items, _ = s.ListFaultCases()
	if len(items) != 0 {
		t.Fatal("fault case was not deleted")
	}
}

func TestCreateFaultCasesValidatesWholeBatch(t *testing.T) {
	s := New(tools.NewService(nil, nil, nil), nil)
	ctx := WithActor(context.Background(), "USR-admin", "admin", "admin")
	valid := domain.FaultCaseRequest{Name: "valid", ProductID: "payment", Question: "why", GoldCause: "cause", Logs: []domain.Evidence{{Content: "evidence"}}}
	invalid := domain.FaultCaseRequest{Name: "invalid", ProductID: "payment", Question: "why", GoldCause: "cause"}
	if _, err := s.CreateFaultCases(ctx, []domain.FaultCaseRequest{valid, invalid}); err == nil {
		t.Fatal("invalid batch must fail")
	}
	items, _ := s.ListFaultCases()
	if len(items) != 0 {
		t.Fatal("validation failure must not partially import cases")
	}
	created, err := s.CreateFaultCases(ctx, []domain.FaultCaseRequest{valid, valid})
	if err != nil || len(created) != 2 {
		t.Fatalf("valid batch import failed: len=%d err=%v", len(created), err)
	}
}

func TestReplayConfigValidation(t *testing.T) {
	got, err := normalizeReplayConfigs([]string{"full", "full", "bm25"})
	if err != nil || len(got) != 2 {
		t.Fatalf("unexpected configs: %v %v", got, err)
	}
	if _, err := normalizeReplayConfigs([]string{"invalid"}); err == nil {
		t.Fatal("invalid replay config must be rejected")
	}
}

func TestAssessReplayQuality(t *testing.T) {
	v := domain.ReplayResult{Judged: true, Faithfulness: 0, Hallucination: false, CauseCorrect: true, Diagnosis: domain.DiagnosisResult{Evidence: []domain.Evidence{}}}
	assessReplayQuality(&v)
	if v.QualityStatus != "warning" || len(v.QualityIssues) != 2 {
		t.Fatalf("expected two consistency warnings: %+v", v)
	}
	v = domain.ReplayResult{Judged: true, Faithfulness: 1, Hallucination: false, CauseCorrect: true, Diagnosis: domain.DiagnosisResult{Evidence: []domain.Evidence{{Source: "log/1"}}}}
	assessReplayQuality(&v)
	if v.QualityStatus != "pass" || len(v.QualityIssues) != 0 {
		t.Fatalf("well-formed result should pass: %+v", v)
	}
}

type emptyAlerts struct{}

func (emptyAlerts) Name() string                          { return "empty" }
func (emptyAlerts) Alerts(string) ([]domain.Alert, error) { return []domain.Alert{}, nil }

type emptyLogs struct{}

func (emptyLogs) Name() string                                     { return "empty" }
func (emptyLogs) Search(string, string) ([]domain.Evidence, error) { return []domain.Evidence{}, nil }
