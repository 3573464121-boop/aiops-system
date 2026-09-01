package tools

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"aiops-mvp/internal/domain"
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

func TestDataSourceStatusAndConnectionTest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"dat":{"list":[]}}`)
	}))
	defer srv.Close()
	s := NewService(nil, N9EAlertProvider{BaseURL: srv.URL}, DemoLogProvider{})
	items := s.DataSources()
	if len(items) != 4 || !items[0].Configured || items[0].Endpoint != srv.URL {
		t.Fatalf("unexpected data source status: %+v", items)
	}
	result := s.TestDataSource("alerts")
	if result.Status != "ready" || result.LatencyMS < 0 {
		t.Fatalf("connection test failed: %+v", result)
	}
}

func TestReplayProvidersUseCapturedData(t *testing.T) {
	alerts := ReplayAlertProvider{Items: []domain.Alert{{ID: "A-1", ProductID: "payment"}, {ID: "A-2", ProductID: "order"}}}
	gotAlerts, err := alerts.Alerts("payment")
	if err != nil || len(gotAlerts) != 1 || gotAlerts[0].ID != "A-1" {
		t.Fatalf("unexpected replay alerts: %+v err=%v", gotAlerts, err)
	}
	logs := ReplayLogProvider{Items: []domain.Evidence{{Title: "payment-api", Content: "connection pool timeout", Source: "case/log/1"}}}
	gotLogs, err := logs.Search("payment", "unmatched query")
	if err != nil || len(gotLogs) != 1 || gotLogs[0].Source != "case/log/1" {
		t.Fatalf("replay logs must deterministically fall back to captured data: %+v err=%v", gotLogs, err)
	}
	assets := ReplayAssetProvider{Items: []domain.Asset{{ID: "S-1", ProductID: "payment", IP: "10.0.0.1"}}}
	gotAssets, err := assets.LookupIP("10.0.0.1")
	if err != nil || len(gotAssets) != 1 || gotAssets[0].ID != "S-1" {
		t.Fatalf("unexpected replay assets: %+v err=%v", gotAssets, err)
	}
}
