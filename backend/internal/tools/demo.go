package tools

import (
	"strings"
	"time"

	"aiops-mvp/internal/domain"
)

type DemoAlertProvider struct{}

func (DemoAlertProvider) Name() string { return "demo" }

func (DemoAlertProvider) Alerts(productID string) ([]domain.Alert, error) {
	now := time.Now()
	all := []domain.Alert{
		{ID: "ALT-24001", ProductID: "payment", Rule: "支付接口错误率过高", Severity: 1, Target: "payment-api-01", Value: "error_rate=8.7%", Triggered: now.Add(-12 * time.Minute)},
		{ID: "ALT-24002", ProductID: "payment", Rule: "P95响应时间过高", Severity: 2, Target: "payment-api-02", Value: "p95=3.8s", Triggered: now.Add(-9 * time.Minute)},
		{ID: "ALT-24003", ProductID: "inventory", Rule: "实例存活异常", Severity: 1, Target: "inventory-03", Value: "up=0", Triggered: now.Add(-4 * time.Minute)},
		{ID: "ALT-24004", ProductID: "inventory", Rule: "JVM 老年代使用率过高", Severity: 2, Target: "inventory-01", Value: "old_gen=94%", Triggered: now.Add(-7 * time.Minute)},
		{ID: "ALT-24005", ProductID: "order", Rule: "订单创建超时率上升", Severity: 2, Target: "order-api-02", Value: "timeout_rate=5.2%", Triggered: now.Add(-15 * time.Minute)},
		{ID: "ALT-24006", ProductID: "order", Rule: "消息队列消费积压", Severity: 3, Target: "order-consumer-01", Value: "lag=12000", Triggered: now.Add(-20 * time.Minute)},
		{ID: "ALT-24007", ProductID: "gateway", Rule: "网关 5xx 突增", Severity: 1, Target: "gateway-02", Value: "5xx_qps=340", Triggered: now.Add(-3 * time.Minute)},
		{ID: "ALT-24008", ProductID: "gateway", Rule: "上游连接被拒绝", Severity: 3, Target: "gateway-01", Value: "conn_refused=57", Triggered: now.Add(-6 * time.Minute)},
		{ID: "ALT-24009", ProductID: "user", Rule: "登录失败率升高", Severity: 2, Target: "user-api-03", Value: "login_fail=14%", Triggered: now.Add(-11 * time.Minute)},
	}
	out := make([]domain.Alert, 0)
	for _, a := range all {
		if productID == "" || strings.EqualFold(a.ProductID, productID) {
			out = append(out, a)
		}
	}
	return out, nil
}

type DemoLogProvider struct{}

func (DemoLogProvider) Name() string { return "demo" }

// demoLogs 按产品维护一批贴近真实的异常日志，兼作诊断演示与论文实验的种子数据。
var demoLogs = map[string][]domain.Evidence{
	"payment": {
		{Type: "log", Title: "payment-api timeout", Content: "upstream inventory-service timeout after 3000ms; pending_requests=182", Score: .94, Source: "demo://logs/payment-api"},
		{Type: "log", Title: "connection pool wait", Content: "db pool wait duration p95=812ms, active=48, max=50", Score: .81, Source: "demo://logs/payment-db"},
	},
	"inventory": {
		{Type: "log", Title: "inventory GC pause", Content: "Full GC (Ergonomics) 1.84s, old gen 94%->91%; frequent full gc detected", Score: .9, Source: "demo://logs/inventory-jvm"},
		{Type: "log", Title: "health check fail", Content: "liveness probe failed: HTTP 500 from /healthz; instance inventory-03 restarting", Score: .86, Source: "demo://logs/inventory-03"},
	},
	"order": {
		{Type: "log", Title: "order create timeout", Content: "call payment-service timeout 2000ms while creating order; retry=2", Score: .88, Source: "demo://logs/order-api"},
		{Type: "log", Title: "kafka consumer lag", Content: "consumer group order-consumer lag=12000, rebalancing frequently", Score: .8, Source: "demo://logs/order-mq"},
	},
	"gateway": {
		{Type: "log", Title: "gateway 502", Content: "502 Bad Gateway upstream=order-api connect timeout; 340 req/s failing", Score: .92, Source: "demo://logs/gateway"},
		{Type: "log", Title: "upstream connection refused", Content: "dial tcp order-api:8080: connect: connection refused (x57)", Score: .84, Source: "demo://logs/gateway"},
	},
	"user": {
		{Type: "log", Title: "session redis timeout", Content: "redis GET session timeout 200ms; auth fallback rejected login", Score: .83, Source: "demo://logs/user-api"},
	},
}

func (DemoLogProvider) Search(productID, query string) ([]domain.Evidence, error) {
	logs := demoLogs[strings.ToLower(strings.TrimSpace(productID))]
	if len(logs) == 0 {
		return []domain.Evidence{}, nil
	}
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return logs, nil
	}
	// 关键词命中则优先返回命中项；都不命中则返回该产品全部日志，保证 AI 有证据可依。
	matched := make([]domain.Evidence, 0, len(logs))
	for _, v := range logs {
		if strings.Contains(strings.ToLower(v.Title+" "+v.Content), q) {
			matched = append(matched, v)
		}
	}
	if len(matched) > 0 {
		return matched, nil
	}
	return logs, nil
}
