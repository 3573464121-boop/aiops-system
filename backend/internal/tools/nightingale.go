package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"aiops-mvp/internal/domain"
)

// N9EAlertProvider 对接夜莺（Nightingale / N9E）的当前活跃告警接口，只读。
// 端点与字段遵循 Nightingale v6/v7 的 /api/n9e/alert-cur-events/list，
// 不同版本字段可能略有差异，接入真实实例时按需微调映射即可。
type N9EAlertProvider struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

func (N9EAlertProvider) Name() string { return "nightingale" }

func (p N9EAlertProvider) client() *http.Client {
	if p.HTTP != nil {
		return p.HTTP
	}
	return &http.Client{Timeout: 10 * time.Second}
}

func (p N9EAlertProvider) Alerts(productID string) ([]domain.Alert, error) {
	url := strings.TrimRight(p.BaseURL, "/") + "/api/n9e/alert-cur-events/list?limit=200&p=1"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if p.Token != "" {
		req.Header.Set("Authorization", p.Token)
		req.Header.Set("X-User-Token", p.Token)
	}
	resp, err := p.client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("夜莺请求失败: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("夜莺 HTTP %d: %s", resp.StatusCode, truncateStr(string(data), 200))
	}
	var out struct {
		Dat struct {
			List []n9eEvent `json:"list"`
		} `json:"dat"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("夜莺响应解析失败: %w", err)
	}
	alerts := make([]domain.Alert, 0, len(out.Dat.List))
	for _, e := range out.Dat.List {
		if productID != "" && !strings.EqualFold(e.GroupName, productID) {
			continue
		}
		alerts = append(alerts, domain.Alert{
			ID:        fmt.Sprintf("N9E-%d", e.ID),
			ProductID: e.GroupName,
			Rule:      e.RuleName,
			Severity:  mapN9ESeverity(e.Severity),
			Target:    e.TargetIdent,
			Value:     e.TriggerValue,
			Triggered: time.Unix(e.TriggerTime, 0),
		})
	}
	return alerts, nil
}

type n9eEvent struct {
	ID           int64  `json:"id"`
	RuleName     string `json:"rule_name"`
	Severity     int    `json:"severity"`
	TargetIdent  string `json:"target_ident"`
	TriggerTime  int64  `json:"trigger_time"`
	TriggerValue string `json:"trigger_value"`
	GroupName    string `json:"group_name"`
}

// mapN9ESeverity 夜莺 1=一级(最严重) 2=二级 3=三级 → 统一 0-4 分级。
func mapN9ESeverity(s int) int {
	switch s {
	case 1:
		return 1
	case 2:
		return 2
	case 3:
		return 3
	default:
		return 3
	}
}

func truncateStr(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
