package app

import (
	"context"
	"testing"
	"time"

	"aiops-mvp/internal/domain"
	"aiops-mvp/internal/tools"
)

func TestCorrelateAlertEventsBuildsExplainableClusters(t *testing.T) {
	s := New(tools.NewService(nil, nil, nil), nil)
	now := time.Now()
	inputs := []domain.AlertEventInput{
		{ProductID: "payment", Rule: "High error rate", Target: "payment-api-01", Severity: 1, OccurredAt: now},
		{ProductID: "payment", Rule: "High latency", Target: "payment-api-02", Severity: 2, OccurredAt: now.Add(5 * time.Minute)},
		{ProductID: "payment", Rule: "Disk capacity", Target: "payment-db-01", Severity: 2, OccurredAt: now.Add(40 * time.Minute)},
		{ProductID: "order", Rule: "High error rate", Target: "payment-api-01", Severity: 1, OccurredAt: now.Add(3 * time.Minute)},
	}
	if _, err := s.IngestAlertEvents(context.Background(), inputs, "nightingale"); err != nil {
		t.Fatal(err)
	}
	incidents, metrics, err := s.CorrelateAlertEvents("")
	if err != nil {
		t.Fatal(err)
	}
	if len(incidents) != 3 || metrics.IncidentCount != 3 || metrics.CorrelatedEventCount != 2 || metrics.SingletonCount != 2 {
		t.Fatalf("unexpected correlation result: incidents=%+v metrics=%+v", incidents, metrics)
	}
	if metrics.AlgorithmVersion != "rule-v1" || metrics.WindowMinutes != 20 || metrics.Threshold != 0.65 {
		t.Fatalf("algorithm configuration must be exposed: %+v", metrics)
	}
	var cluster domain.AlertIncident
	for _, incident := range incidents {
		if incident.ProductID == "payment" && incident.EventCount == 2 {
			cluster = incident
		}
	}
	if cluster.ID == "" || cluster.SignalCount != 2 || len(cluster.Reasons) < 3 {
		t.Fatalf("correlated incident is incomplete: %+v", cluster)
	}
	if cluster.ID != incidentID([]domain.AlertEvent{cluster.Events[1], cluster.Events[0]}) {
		t.Fatal("incident id must not depend on event ordering")
	}
}

func TestDiagnoseAlertIncidentIncludesEveryEvent(t *testing.T) {
	s := New(tools.NewService(nil, nil, nil), nil)
	now := time.Now()
	inputs := []domain.AlertEventInput{
		{ProductID: "payment", Rule: "High error rate", Target: "payment-api-01", Severity: 1, OccurredAt: now},
		{ProductID: "payment", Rule: "High latency", Target: "payment-api-02", Severity: 2, OccurredAt: now.Add(time.Minute)},
	}
	if _, err := s.IngestAlertEvents(context.Background(), inputs, "nightingale"); err != nil {
		t.Fatal(err)
	}
	incidents, _, err := s.CorrelateAlertEvents("payment")
	if err != nil || len(incidents) != 1 {
		t.Fatalf("missing incident: %+v err=%v", incidents, err)
	}
	incident, diagnosis, err := s.DiagnoseAlertIncident(context.Background(), incidents[0].ID, "payment")
	if err != nil {
		t.Fatal(err)
	}
	if incident.EventCount != 2 || len(diagnosis.Alerts) < 2 {
		t.Fatalf("cluster diagnosis missed alert context: incident=%+v alerts=%+v", incident, diagnosis.Alerts)
	}
	seen := make(map[string]bool)
	for _, evidence := range diagnosis.Evidence {
		if len(evidence.Source) > len("alert-event/") && evidence.Source[:len("alert-event/")] == "alert-event/" {
			seen[evidence.Source] = true
		}
	}
	if len(seen) != 2 {
		t.Fatalf("cluster diagnosis must retain every event snapshot: %+v", diagnosis.Evidence)
	}
}
