package executor

import (
	"context"
	"testing"

	"aiops-mvp/internal/domain"
)

func TestPreviewPolicy(t *testing.T) {
	simulator := NewSimulator()
	tests := []struct {
		name     string
		approval domain.Approval
		allowed  bool
		kind     string
	}{
		{"approved rollback", domain.Approval{ID: "1", ProductID: "payment", Action: "回滚最近一次发布", Risk: "high", Status: "approved"}, true, "rollback_release"},
		{"risk understated", domain.Approval{ID: "2", ProductID: "payment", Action: "清理缓存", Risk: "medium", Status: "approved"}, false, "clear_cache"},
		{"unknown action", domain.Approval{ID: "3", ProductID: "payment", Action: "修改所有参数", Risk: "high", Status: "approved"}, false, "unknown"},
		{"not approved", domain.Approval{ID: "4", ProductID: "payment", Action: "重启服务", Risk: "medium", Status: "pending"}, false, "restart_service"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := simulator.Preview(tt.approval)
			if plan.Allowed != tt.allowed || plan.Kind != tt.kind || plan.Mode != "simulate" {
				t.Fatalf("unexpected plan: %+v", plan)
			}
			if !plan.Allowed && plan.BlockReason == "" {
				t.Fatal("blocked plan must explain the reason")
			}
		})
	}
}

func TestExecuteNeverUsesProductionMode(t *testing.T) {
	simulator := NewSimulator()
	result, err := simulator.Execute(context.Background(), domain.Approval{
		ID: "APR-1", ProductID: "payment", Action: "滚动重启服务", Risk: "medium", Status: "approved",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Mode != "simulate" || result.Kind != "restart_service" || result.Output == "" {
		t.Fatalf("unexpected result: %+v", result)
	}
}
