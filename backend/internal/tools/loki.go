package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"aiops-mvp/internal/domain"
)

// LokiLogProvider 对接 Grafana Loki 的 query_range 接口，只读。
// LogQL 约定：日志按 product 标签区分，如 {product="payment"} |~ "timeout"。
// 换成 Elasticsearch / ClickHouse 时，只需另写一个实现 LogProvider 的适配器即可，无需改诊断编排。
type LokiLogProvider struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

func (LokiLogProvider) Name() string { return "loki" }

func (p LokiLogProvider) client() *http.Client {
	if p.HTTP != nil {
		return p.HTTP
	}
	return &http.Client{Timeout: 10 * time.Second}
}

func (p LokiLogProvider) Search(productID, query string) ([]domain.Evidence, error) {
	logql := fmt.Sprintf("{product=%q}", productID)
	if q := strings.TrimSpace(query); q != "" {
		logql += fmt.Sprintf(" |~ %q", q)
	}
	end := time.Now()
	start := end.Add(-30 * time.Minute)
	params := url.Values{}
	params.Set("query", logql)
	params.Set("start", fmt.Sprintf("%d", start.UnixNano()))
	params.Set("end", fmt.Sprintf("%d", end.UnixNano()))
	params.Set("limit", "50")
	params.Set("direction", "backward")

	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(p.BaseURL, "/")+"/loki/api/v1/query_range?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if p.Token != "" {
		req.Header.Set("Authorization", "Bearer "+p.Token)
	}
	resp, err := p.client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("Loki 请求失败: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Loki HTTP %d: %s", resp.StatusCode, truncateStr(string(data), 200))
	}
	var out struct {
		Data struct {
			Result []struct {
				Stream map[string]string `json:"stream"`
				Values [][]string        `json:"values"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("Loki 响应解析失败: %w", err)
	}
	evidence := make([]domain.Evidence, 0, 16)
	for _, r := range out.Data.Result {
		title := r.Stream["service"]
		if title == "" {
			title = r.Stream["app"]
		}
		if title == "" {
			title = productID
		}
		for _, v := range r.Values {
			if len(v) < 2 {
				continue
			}
			evidence = append(evidence, domain.Evidence{
				Type:    "log",
				Title:   title,
				Content: truncateStr(v[1], 500),
				Score:   0.7,
				Source:  "loki://" + logql,
			})
			if len(evidence) >= 50 {
				return evidence, nil
			}
		}
	}
	return evidence, nil
}
