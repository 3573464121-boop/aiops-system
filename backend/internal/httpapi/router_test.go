package httpapi

import (
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
}
