package app

import (
	"context"
	"math"
	"testing"
	"time"

	"aiops-mvp/internal/domain"
	"aiops-mvp/internal/tools"
)

func TestAlertEventAggregationAndRecovery(t *testing.T) {
	s := New(tools.NewService(nil, nil, nil), nil)
	now := time.Now()
	base := domain.AlertEventInput{ExternalID: "n9e-1", ProductID: "payment", Rule: "High error rate", Severity: 1, Target: "payment-api-01", Value: "8%", Status: "open", OccurredAt: now}
	result, err := s.IngestAlertEvents(context.Background(), []domain.AlertEventInput{base, base}, "nightingale")
	if err != nil {
		t.Fatal(err)
	}
	if result.Created != 1 || result.Merged != 1 || result.Items[1].Occurrences != 2 {
		t.Fatalf("unexpected aggregation result: %+v", result)
	}

	recovered := base
	recovered.Status = "recovered"
	recovered.Value = "1%"
	recovered.OccurredAt = now.Add(time.Minute)
	result, err = s.IngestAlertEvents(context.Background(), []domain.AlertEventInput{recovered}, "nightingale")
	if err != nil || result.Merged != 1 || result.Items[0].Status != "resolved" || result.Items[0].Occurrences != 3 {
		t.Fatalf("recovery must close the existing event: %+v err=%v", result, err)
	}

	items, metrics, err := s.ListAlertEvents("", "payment", 20)
	if err != nil || len(items) != 1 {
		t.Fatalf("list events: len=%d err=%v", len(items), err)
	}
	if metrics.RawSignals != 3 || metrics.EventCount != 1 || metrics.ResolvedCount != 1 || math.Abs(metrics.ReductionRate-2.0/3.0) > 0.0001 {
		t.Fatalf("unexpected metrics: %+v", metrics)
	}
	updated, diagnosis, err := s.DiagnoseAlertEvent(context.Background(), items[0].ID)
	if err != nil || updated.DiagnosedAt.IsZero() || updated.DiagnosisSummary == "" {
		t.Fatalf("event diagnosis was not stored: %+v err=%v", updated, err)
	}
	foundEventEvidence := false
	for _, evidence := range diagnosis.Evidence {
		if evidence.Source == "alert-event/"+items[0].ID {
			foundEventEvidence = true
			break
		}
	}
	if !foundEventEvidence {
		t.Fatalf("event snapshot must be part of diagnosis evidence: %+v", diagnosis.Evidence)
	}
	reopened, err := s.ReopenAlertEvent(context.Background(), items[0].ID)
	if err != nil || reopened.Status != "open" || reopened.Occurrences != 3 {
		t.Fatalf("reopen must only correct event status: %+v err=%v", reopened, err)
	}
}

func TestAlertEventFingerprintSeparatesTargetsAndProducts(t *testing.T) {
	s := New(tools.NewService(nil, nil, nil), nil)
	inputs := []domain.AlertEventInput{
		{ProductID: "payment", Rule: "CPU high", Target: "node-1"},
		{ProductID: "payment", Rule: "CPU high", Target: "node-2"},
		{ProductID: "order", Rule: "CPU high", Target: "node-1"},
	}
	result, err := s.IngestAlertEvents(context.Background(), inputs, "nightingale")
	if err != nil || result.Created != 3 || result.Merged != 0 {
		t.Fatalf("distinct alerts were incorrectly merged: %+v err=%v", result, err)
	}
}

func TestParseNightingaleAlertPayload(t *testing.T) {
	payload := []byte(`{"events":[{"id":101,"rule_name":"High latency","severity":1,"target_ident":"api-01","trigger_time":1725000000,"trigger_value":"p95=2s","tags":["env=prod","product_id=payment"]},{"event_id":"102","rule":"Instance down","priority":"warning","target":"api-02","status":"recovered","group_name":"payment"}]}`)
	items, err := ParseNightingaleAlertPayload(payload)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].ExternalID != "101" || items[0].ProductID != "payment" || items[0].OccurredAt.IsZero() {
		t.Fatalf("first event was not parsed: %+v", items)
	}
	if items[1].Severity != 2 || items[1].Status != "recovered" {
		t.Fatalf("second event was not parsed: %+v", items[1])
	}

	single, err := ParseNightingaleAlertPayload([]byte(`{"event":{"rule_name":"Disk full","target_ident":"db-01","is_recovered":true,"tag_map":{"product_id":"database"}}}`))
	if err != nil || len(single) != 1 || single[0].Status != "resolved" || single[0].ProductID != "database" {
		t.Fatalf("wrapped recovery event was not parsed: %+v err=%v", single, err)
	}
}
