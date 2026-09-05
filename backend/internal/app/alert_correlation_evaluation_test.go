package app

import (
	"context"
	"math"
	"testing"
	"time"

	"aiops-mvp/internal/domain"
	"aiops-mvp/internal/tools"
)

func TestEvaluateAlertCorrelationAgainstHumanLabels(t *testing.T) {
	s := New(tools.NewService(nil, nil, nil), nil)
	now := time.Now()
	inputs := []domain.AlertEventInput{
		{ProductID: "payment", Rule: "API error rate", Target: "payment-api-01", Severity: 1, OccurredAt: now},
		{ProductID: "payment", Rule: "API latency", Target: "payment-api-02", Severity: 2, OccurredAt: now.Add(time.Minute)},
		{ProductID: "payment", Rule: "DB latency", Target: "payment-db-01", Severity: 2, OccurredAt: now.Add(40 * time.Minute)},
		{ProductID: "payment", Rule: "DB connections", Target: "payment-db-02", Severity: 2, OccurredAt: now.Add(41 * time.Minute)},
	}
	ingested, err := s.IngestAlertEvents(context.Background(), inputs, "nightingale")
	if err != nil {
		t.Fatal(err)
	}
	byRule := make(map[string]string)
	for _, event := range ingested.Items {
		byRule[event.Rule] = event.ID
	}
	ctx := WithActor(context.Background(), "USR-1", "reviewer", "admin")
	labels, err := s.SaveAlertCorrelationLabels(ctx, domain.AlertCorrelationLabelRequest{Items: []domain.AlertCorrelationLabelInput{
		{EventID: byRule["API error rate"], FaultKey: "FAULT-1", Note: "人工确认"},
		{EventID: byRule["API latency"], FaultKey: "FAULT-2"},
		{EventID: byRule["DB latency"], FaultKey: "FAULT-1"},
		{EventID: byRule["DB connections"], FaultKey: "FAULT-1"},
	}})
	if err != nil || len(labels) != 4 {
		t.Fatalf("save labels failed: labels=%+v err=%v", labels, err)
	}
	for _, label := range labels {
		if label.LabeledBy != "reviewer" {
			t.Fatalf("missing reviewer identity: %+v", label)
		}
	}
	metrics, active, err := s.EvaluateAlertCorrelation("payment")
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 4 || metrics.EligibleEventCount != 4 || metrics.LabeledEventCount != 4 || metrics.EvaluatedPairCount != 6 {
		t.Fatalf("unexpected evaluation coverage: %+v labels=%+v", metrics, active)
	}
	if metrics.TruePositive != 1 || metrics.FalsePositive != 1 || metrics.FalseNegative != 2 || metrics.TrueNegative != 2 {
		t.Fatalf("unexpected pair confusion matrix: %+v", metrics)
	}
	assertClose(t, metrics.PairPrecision, 0.5)
	assertClose(t, metrics.PairRecall, 1.0/3.0)
	assertClose(t, metrics.PairF1, 0.4)
	assertClose(t, metrics.PairAccuracy, 0.5)
	assertClose(t, metrics.FalseLinkRate, 0.5)
	assertClose(t, metrics.MissedLinkRate, 2.0/3.0)
}

func TestSaveAlertCorrelationLabelsCanClearLabel(t *testing.T) {
	s := New(tools.NewService(nil, nil, nil), nil)
	ingested, err := s.IngestAlertEvents(context.Background(), []domain.AlertEventInput{{
		ProductID: "payment", Rule: "High errors", Target: "payment-api-01", Severity: 1, OccurredAt: time.Now(),
	}}, "nightingale")
	if err != nil {
		t.Fatal(err)
	}
	eventID := ingested.Items[0].ID
	ctx := WithActor(context.Background(), "USR-1", "reviewer", "admin")
	if _, err = s.SaveAlertCorrelationLabels(ctx, domain.AlertCorrelationLabelRequest{Items: []domain.AlertCorrelationLabelInput{{EventID: eventID, FaultKey: "FAULT-1"}}}); err != nil {
		t.Fatal(err)
	}
	labels, err := s.SaveAlertCorrelationLabels(ctx, domain.AlertCorrelationLabelRequest{Items: []domain.AlertCorrelationLabelInput{{EventID: eventID}}})
	if err != nil || len(labels) != 0 {
		t.Fatalf("label was not cleared: labels=%+v err=%v", labels, err)
	}
}

func assertClose(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("got %f, want %f", got, want)
	}
}
