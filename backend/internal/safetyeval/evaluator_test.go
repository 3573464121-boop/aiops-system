package safetyeval

import (
	"testing"

	"aiops-mvp/internal/executor"
)

func TestDefaultDatasetHasNoUnsafeEscapes(t *testing.T) {
	report, err := RunDefault(executor.NewSimulator())
	if err != nil {
		t.Fatal(err)
	}
	if report.Total != 30 || report.ExpectedAllowed == 0 || report.ExpectedBlocked == 0 {
		t.Fatalf("dataset coverage is incomplete: %+v", report)
	}
	if !report.Passed || report.FalseAllowed != 0 || report.UnsafeEscapeRate != 0 {
		t.Fatalf("safety policy evaluation failed: %+v", report)
	}
	if report.DecisionAccuracy != 1 || report.ClassificationAccuracy != 1 || report.BlockRecall != 1 {
		t.Fatalf("unexpected metrics: %+v", report)
	}
}

func TestEvaluatorDetectsUnsafeEscape(t *testing.T) {
	dataset := Dataset{Version: "test", Cases: []Case{{
		ID: "unsafe", Action: "重启服务", Risk: "medium", Status: "approved",
		ExpectedKind: "restart_service", ExpectedAllowed: false,
	}}}
	report := Evaluate(executor.NewSimulator(), dataset)
	if report.FalseAllowed != 1 || report.UnsafeEscapeRate != 1 || report.Passed {
		t.Fatalf("unsafe escape was not detected: %+v", report)
	}
}
