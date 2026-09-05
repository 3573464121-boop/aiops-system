package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aiops-mvp/internal/app"
	"aiops-mvp/internal/auth"
	"aiops-mvp/internal/tools"
)

func TestKnowledgeValidation(t *testing.T) {
	signer := auth.NewSigner("test-secret", time.Hour)
	token, err := signer.Issue("USR-1", "tester", "admin", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	r := New(app.New(&tools.Service{}, nil), signer, 0)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/knowledge/search", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("want 400 got %d", w.Code)
	}
}
func TestDiagnosis(t *testing.T) {
	signer := auth.NewSigner("test-secret", time.Hour)
	token, err := signer.Issue("USR-1", "tester", "admin", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	r := New(app.New(&tools.Service{}, nil), signer, 0)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/diagnoses", strings.NewReader(`{"product_id":"unknown","question":"no evidence"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("want 200 got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"evidence":[`) {
		t.Fatalf("evidence should be array: %s", w.Body.String())
	}
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/diagnosis-runs", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"total":1`) {
		t.Fatalf("diagnosis run should be queryable: %d %s", w.Code, w.Body.String())
	}
}

func TestCORSAllowsLoopbackFrontend(t *testing.T) {
	signer := auth.NewSigner("test-secret", time.Hour)
	r := New(app.New(&tools.Service{}, nil), signer, 0)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/auth/login", nil)
	req.Header.Set("Origin", "http://127.0.0.1:5173")
	req.Header.Set("Access-Control-Request-Method", "POST")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent || w.Header().Get("Access-Control-Allow-Origin") != "http://127.0.0.1:5173" {
		t.Fatalf("loopback CORS failed: %d %v", w.Code, w.Header())
	}
}

func TestApprovalPermissionsAndLifecycle(t *testing.T) {
	signer := auth.NewSigner("test-secret", time.Hour)
	viewerToken, err := signer.Issue("USR-viewer", "viewer1", "viewer", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	adminToken, err := signer.Issue("USR-admin", "admin", "admin", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	r := New(app.New(&tools.Service{}, nil), signer, 0)

	request := func(method, path, body, token string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		r.ServeHTTP(w, req)
		return w
	}

	w := request(http.MethodPost, "/api/v1/approvals", `{"product_id":"payment","action":"回滚发布","risk":"high","reason":"错误率持续升高"}`, viewerToken)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: want 201 got %d: %s", w.Code, w.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil || created.ID == "" {
		t.Fatalf("invalid create response: %s", w.Body.String())
	}

	w = request(http.MethodPost, "/api/v1/approvals/"+created.ID+"/review", `{"decision":"approved"}`, viewerToken)
	if w.Code != http.StatusForbidden {
		t.Fatalf("viewer review: want 403 got %d", w.Code)
	}
	w = request(http.MethodPost, "/api/v1/approvals/"+created.ID+"/review", `{"decision":"approved","comment":"同意"}`, adminToken)
	if w.Code != http.StatusOK {
		t.Fatalf("admin review: want 200 got %d: %s", w.Code, w.Body.String())
	}
	w = request(http.MethodGet, "/api/v1/approvals/"+created.ID+"/execution-plan", ``, adminToken)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"mode":"simulate"`) || !strings.Contains(w.Body.String(), `"allowed":true`) {
		t.Fatalf("execution plan: want allowed simulation got %d: %s", w.Code, w.Body.String())
	}
	w = request(http.MethodPost, "/api/v1/approvals/"+created.ID+"/execute", `{"note":"人工执行完成","confirm_action":"回滚发布"}`, adminToken)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"status":"executed"`) {
		t.Fatalf("execute: want executed got %d: %s", w.Code, w.Body.String())
	}
}

func TestFaultCaseRoutesRequireAdmin(t *testing.T) {
	signer := auth.NewSigner("test-secret", time.Hour)
	viewerToken, _ := signer.Issue("USR-viewer", "viewer", "viewer", time.Now())
	adminToken, _ := signer.Issue("USR-admin", "admin", "admin", time.Now())
	r := New(app.New(&tools.Service{}, nil), signer, 0)
	body := `{"name":"timeout","product_id":"payment","question":"why timeout","gold_cause":"pool exhausted","logs":[{"content":"pool wait timeout"}]}`
	request := func(token string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/fault-cases", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		r.ServeHTTP(w, req)
		return w
	}
	if w := request(viewerToken); w.Code != http.StatusForbidden {
		t.Fatalf("viewer create: want 403 got %d", w.Code)
	}
	if w := request(adminToken); w.Code != http.StatusCreated {
		t.Fatalf("admin create: want 201 got %d: %s", w.Code, w.Body.String())
	}
}

func TestNightingaleWebhookRequiresDedicatedToken(t *testing.T) {
	signer := auth.NewSigner("test-secret", time.Hour)
	r := New(app.New(&tools.Service{}, nil), signer, 0)
	body := `{"rule_name":"High error rate","target_ident":"payment-api-01","group_name":"payment","severity":1}`

	t.Setenv("ALERT_WEBHOOK_TOKEN", "")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/alerts/nightingale", strings.NewReader(body))
	r.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("disabled webhook: want 503 got %d", w.Code)
	}

	t.Setenv("ALERT_WEBHOOK_TOKEN", "webhook-secret")
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/alerts/nightingale", strings.NewReader(body))
	req.Header.Set("X-Webhook-Token", "wrong")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("wrong token: want 401 got %d", w.Code)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/alerts/nightingale", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer webhook-secret")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"created":1`) {
		t.Fatalf("valid webhook failed: %d %s", w.Code, w.Body.String())
	}
}
