package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aiops-mvp/internal/app"
	"aiops-mvp/internal/tools"
)

func TestKnowledgeValidation(t *testing.T) {
	r := New(app.New(&tools.Service{}, nil), 0)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/knowledge/search", nil)
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("want 400 got %d", w.Code)
	}
}
func TestDiagnosis(t *testing.T) {
	r := New(app.New(&tools.Service{}, nil), 0)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/diagnoses", strings.NewReader(`{"product_id":"unknown","question":"no evidence"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("want 200 got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"evidence":[`) {
		t.Fatalf("evidence should be array: %s", w.Body.String())
	}
}
