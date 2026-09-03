package storage

import (
	"testing"
	"time"

	"aiops-mvp/internal/domain"
)

func TestExperimentBatchAndReplayFiltering(t *testing.T) {
	repo := NewMemory()
	batch := domain.ExperimentBatch{ID: "BATCH-1", Name: "test", Status: "pending", CaseIDs: []string{"CASE-1"}, Configs: []string{"full"}, Repeats: 1, TotalRuns: 1, CreatedAt: time.Now()}
	if err := repo.CreateExperimentBatch(batch); err != nil {
		t.Fatal(err)
	}
	batch.Status, batch.CompletedRuns = "completed", 1
	if err := repo.UpdateExperimentBatch(batch); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetExperimentBatch(batch.ID)
	if err != nil || got.Status != "completed" || got.CompletedRuns != 1 {
		t.Fatalf("unexpected batch: %+v err=%v", got, err)
	}
	for _, result := range []domain.ReplayResult{
		{ID: "R-1", CaseID: "CASE-1", BatchID: "BATCH-1", Trial: 1},
		{ID: "R-2", CaseID: "CASE-1", BatchID: "BATCH-2", Trial: 1},
	} {
		if err := repo.CreateReplayResult(result); err != nil {
			t.Fatal(err)
		}
	}
	items, err := repo.ListReplayResults("CASE-1", "BATCH-1", 10)
	if err != nil || len(items) != 1 || items[0].ID != "R-1" {
		t.Fatalf("batch filter failed: %+v err=%v", items, err)
	}
}
