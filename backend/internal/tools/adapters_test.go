package tools

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestN9EAlertProviderMapping(t *testing.T) {
	now := time.Now().Unix()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"dat":{"list":[
			{"id":101,"rule_name":"支付错误率过高","severity":1,"target_ident":"pay-01","trigger_time":%d,"trigger_value":"err=9%%","group_name":"payment"},
			{"id":102,"rule_name":"库存存活异常","severity":2,"target_ident":"inv-03","trigger_time":%d,"trigger_value":"up=0","group_name":"inventory"}
		]}}`, now, now)
	}))
	defer srv.Close()

	p := N9EAlertProvider{BaseURL: srv.URL, Token: "t"}
	all, err := p.Alerts("")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("want 2 alerts got %d", len(all))
	}
	if all[0].ID != "N9E-101" || all[0].ProductID != "payment" || all[0].Severity != 1 || all[0].Rule != "支付错误率过高" {
		t.Fatalf("unexpected mapping: %+v", all[0])
	}
	// 产品过滤
	inv, _ := p.Alerts("inventory")
	if len(inv) != 1 || inv[0].ProductID != "inventory" {
		t.Fatalf("product filter failed: %+v", inv)
	}
}

func TestLokiLogProviderMapping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("query") == "" {
			t.Error("missing LogQL query param")
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":{"result":[
			{"stream":{"service":"payment-api"},"values":[["1700000000000000000","upstream timeout 3000ms"],["1700000000000000001","db pool wait high"]]}
		]}}`)
	}))
	defer srv.Close()

	p := LokiLogProvider{BaseURL: srv.URL}
	evs, err := p.Search("payment", "timeout")
	if err != nil {
		t.Fatal(err)
	}
	if len(evs) != 2 {
		t.Fatalf("want 2 log evidence got %d", len(evs))
	}
	if evs[0].Type != "log" || evs[0].Title != "payment-api" || evs[0].Content == "" {
		t.Fatalf("unexpected loki mapping: %+v", evs[0])
	}
}
