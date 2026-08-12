package embed

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEmbedParsesVectors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"embedding":[0.1,0.2,0.3]},{"embedding":[0.4,0.5,0.6]}]}`))
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, Model: "bge-m3"}
	if !c.Enabled() {
		t.Fatal("should be enabled")
	}
	vecs, err := c.Embed(context.Background(), []string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	if len(vecs) != 2 || len(vecs[0]) != 3 || vecs[1][2] != 0.6 {
		t.Fatalf("unexpected vectors: %#v", vecs)
	}
}

func TestEmbedDisabled(t *testing.T) {
	c := &Client{}
	if c.Enabled() {
		t.Fatal("empty client must be disabled")
	}
	if _, err := c.Embed(context.Background(), []string{"x"}); err == nil {
		t.Fatal("disabled client should error")
	}
}
