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

// TestDiagnoseStreamEmitsProgress 验证流式诊断在过程中推送了 status / tool_call / tool_result 进度事件。
func TestDiagnoseStreamEmitsProgress(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "application/json")
		if n == 1 {
			_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"","tool_calls":[{"id":"c1","type":"function","function":{"name":"get_alerts","arguments":"{\"product_id\":\"payment\"}"}}]}}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"{\"summary\":\"ok\",\"confidence\":0.7,\"hypotheses\":[],\"actions\":[]}"}}]}`))
	}))
	defer srv.Close()

	client := &llm.Client{BaseURL: srv.URL, Model: "test-model"}
	s := New(tools.NewService(nil, tools.DemoAlertProvider{}, tools.DemoLogProvider{}), client)

	var events []domain.StreamEvent
	res := s.DiagnoseStream(context.Background(), domain.DiagnosisRequest{ProductID: "payment", Question: "test"}, func(ev domain.StreamEvent) {
		events = append(events, ev)
	})
	if res.Mode != "agent" {
		t.Fatalf("mode want agent got %q", res.Mode)
	}
	var sawStatus, sawToolCall, sawToolResult bool
	for _, e := range events {
		switch e.Type {
		case "status":
			sawStatus = true
		case "tool_call":
			if e.Tool == "get_alerts" {
				sawToolCall = true
			}
		case "tool_result":
			if e.Tool == "get_alerts" && e.Status == "success" {
				sawToolResult = true
			}
		}
	}
	if !sawStatus || !sawToolCall || !sawToolResult {
		t.Fatalf("missing progress events: status=%v tool_call=%v tool_result=%v", sawStatus, sawToolCall, sawToolResult)
	}
}
