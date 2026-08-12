package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"aiops-mvp/internal/domain"
	"aiops-mvp/internal/llm"
	"aiops-mvp/internal/tools"
)

// TestDiagnoseAgentToolLoop 用本地假服务器模拟模型的“先调用工具、再给出结构化结论”，
// 确定性地验证工具调用循环、证据收集、JSON 解析与高风险动作强制审批，无需真实 LLM。
func TestDiagnoseAgentToolLoop(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "application/json")
		if n == 1 {
			// 第 1 轮：模型请求调用 get_alerts(product_id=payment)
			_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"","tool_calls":[{"id":"call_1","type":"function","function":{"name":"get_alerts","arguments":"{\"product_id\":\"payment\"}"}}]}}]}`))
			return
		}
		// 第 2 轮：模型基于工具结果给出最终结构化 JSON（含一个高风险动作，未标 requires_approval）
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"{\"summary\":\"支付上游依赖超时导致错误率升高\",\"confidence\":0.8,\"hypotheses\":[{\"rank\":1,\"cause\":\"上游超时\",\"confidence\":0.8}],\"actions\":[{\"name\":\"回滚最近发布\",\"risk\":\"high\"}]}"}}]}`))
	}))
	defer srv.Close()

	client := &llm.Client{BaseURL: srv.URL, Model: "test-model"}
	s := New(tools.NewService(nil, tools.DemoAlertProvider{}, tools.DemoLogProvider{}), client)

	got := s.Diagnose(context.Background(), domain.DiagnosisRequest{ProductID: "payment", Question: "分析支付服务最近30分钟的高错误率告警", WindowMinute: 30})

	if got.Mode != "agent" {
		t.Fatalf("mode want agent got %q", got.Mode)
	}
	if atomic.LoadInt32(&calls) != 2 {
		t.Fatalf("want 2 LLM calls got %d", calls)
	}
	if got.Summary == "" || got.Confidence != 0.8 {
		t.Fatalf("unexpected summary/confidence: %q %v", got.Summary, got.Confidence)
	}
	if len(got.Hypotheses) != 1 {
		t.Fatalf("want 1 hypothesis got %d", len(got.Hypotheses))
	}
	if len(got.Alerts) != 2 {
		t.Fatalf("want 2 alerts collected via tool call got %d", len(got.Alerts))
	}
	if len(got.Actions) != 1 || !got.Actions[0].RequiresApproval {
		t.Fatalf("high-risk action must be forced to require approval: %+v", got.Actions)
	}
	seen := false
	for _, tr := range got.Trace {
		if tr.Tool == "get_alerts" && tr.Status == "success" {
			seen = true
		}
	}
	if !seen {
		t.Fatal("trace should contain a successful get_alerts tool call")
	}
}
