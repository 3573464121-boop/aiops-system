package executor

import (
	"context"
	"fmt"
	"strings"
	"time"

	"aiops-mvp/internal/domain"
)

type Result struct {
	Kind       string
	Mode       string
	Output     string
	DurationMS int64
}

type Simulator struct{}

func NewSimulator() *Simulator {
	return &Simulator{}
}

func (s *Simulator) Preview(approval domain.Approval) domain.ExecutionPlan {
	kind, minimumRisk, steps := classify(approval.Action, approval.ProductID)
	plan := domain.ExecutionPlan{
		ApprovalID:           approval.ID,
		Kind:                 kind,
		Mode:                 "simulate",
		Steps:                steps,
		RequiresConfirmation: true,
		GeneratedAt:          time.Now(),
	}
	if approval.Status != "approved" {
		plan.BlockReason = "处置单尚未批准"
		return plan
	}
	if kind == "unknown" {
		plan.BlockReason = "动作未命中受控执行白名单，只能在线下处理"
		return plan
	}
	if riskRank(approval.Risk) < riskRank(minimumRisk) {
		plan.BlockReason = fmt.Sprintf("申报风险为 %s，低于策略要求的 %s", approval.Risk, minimumRisk)
		return plan
	}
	plan.Allowed = true
	return plan
}

func (s *Simulator) Execute(ctx context.Context, approval domain.Approval) (Result, error) {
	started := time.Now()
	plan := s.Preview(approval)
	if !plan.Allowed {
		return Result{}, fmt.Errorf("%s", plan.BlockReason)
	}
	select {
	case <-ctx.Done():
		return Result{}, ctx.Err()
	default:
	}
	return Result{
		Kind:       plan.Kind,
		Mode:       plan.Mode,
		Output:     fmt.Sprintf("模拟执行完成：已验证 %s 的前置条件和回滚路径，未调用任何生产接口", displayKind(plan.Kind)),
		DurationMS: time.Since(started).Milliseconds(),
	}, nil
}

func classify(action, productID string) (kind, minimumRisk string, steps []string) {
	normalized := strings.ToLower(strings.TrimSpace(action))
	product := strings.TrimSpace(productID)
	switch {
	case containsAny(normalized, "回滚", "rollback"):
		return "rollback_release", "high", []string{
			"校验 " + product + " 当前版本与目标版本",
			"验证变更窗口、健康检查和回滚点",
			"模拟回滚并生成结果，不触达生产环境",
		}
	case containsAny(normalized, "重启", "restart"):
		return "restart_service", "medium", []string{
			"确认 " + product + " 实例范围与最小可用副本数",
			"验证摘流和健康检查条件",
			"模拟滚动重启，不触达生产环境",
		}
	case containsAny(normalized, "扩容", "缩容", "扩缩容", "scale"):
		return "scale_service", "medium", []string{
			"读取 " + product + " 当前容量和目标容量",
			"验证配额、容量上限和回退阈值",
			"模拟容量调整，不触达生产环境",
		}
	case containsAny(normalized, "清理缓存", "刷新缓存", "clear cache", "flush cache"):
		return "clear_cache", "high", []string{
			"确认缓存命名空间和影响范围",
			"验证缓存预热与回源容量",
			"模拟缓存清理，不触达生产环境",
		}
	case containsAny(normalized, "切流", "流量切换", "traffic switch", "failover"):
		return "switch_traffic", "high", []string{
			"确认源集群、目标集群和流量比例",
			"验证目标集群健康度与回切条件",
			"模拟流量切换，不触达生产环境",
		}
	default:
		return "unknown", "high", nil
	}
}

func containsAny(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if strings.Contains(value, candidate) {
			return true
		}
	}
	return false
}

func riskRank(risk string) int {
	switch strings.ToLower(strings.TrimSpace(risk)) {
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

func displayKind(kind string) string {
	return map[string]string{
		"rollback_release": "版本回滚",
		"restart_service":  "服务重启",
		"scale_service":    "容量调整",
		"clear_cache":      "缓存清理",
		"switch_traffic":   "流量切换",
	}[kind]
}
