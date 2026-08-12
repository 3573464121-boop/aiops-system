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
}
func TestCreateIssueAndAudit(t *testing.T) {
	s := New(tools.NewService(nil, nil, nil), nil)
	i, err := s.CreateIssue(domain.IssueRequest{ProductID: "payment", Title: "test", Diagnosis: "d"})
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

type emptyAlerts struct{}

func (emptyAlerts) Name() string                          { return "empty" }
func (emptyAlerts) Alerts(string) ([]domain.Alert, error) { return []domain.Alert{}, nil }

type emptyLogs struct{}

func (emptyLogs) Name() string                                     { return "empty" }
func (emptyLogs) Search(string, string) ([]domain.Evidence, error) { return []domain.Evidence{}, nil }
