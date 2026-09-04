package tools

import (
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"aiops-mvp/internal/domain"
	"aiops-mvp/internal/knowledge"
)

type Service struct {
	Knowledge      *knowledge.Index
	AlertsProvider AlertProvider
	LogsProvider   LogProvider
	AssetsProvider AssetProvider
	knowledgeMu    sync.RWMutex
	// Embed 可选：把查询向量化，用于知识库混合检索。未设置时退回 BM25。
	Embed           func(string) ([]float32, error)
	EmbedDocuments  func([]string) ([][]float32, error)
	OnKnowledgeHits func([]domain.Evidence)
}

func NewService(k *knowledge.Index, alerts AlertProvider, logs LogProvider) *Service {
	if alerts == nil {
		alerts = DemoAlertProvider{}
	}
	if logs == nil {
		logs = DemoLogProvider{}
	}
	return &Service{Knowledge: k, AlertsProvider: alerts, LogsProvider: logs}
}
func (s *Service) Alerts(productID string) ([]domain.Alert, error) {
	if s.AlertsProvider == nil {
		s.AlertsProvider = DemoAlertProvider{}
	}
	return s.AlertsProvider.Alerts(productID)
}
func (s *Service) Logs(productID, query string) ([]domain.Evidence, error) {
	if s.LogsProvider == nil {
		s.LogsProvider = DemoLogProvider{}
	}
	return s.LogsProvider.Search(productID, query)
}
func (s *Service) SearchKnowledge(q string, limit int) []domain.Evidence {
	index := s.KnowledgeSnapshot()
	if index == nil {
		return []domain.Evidence{}
	}
	var qvec []float32
	if s.Embed != nil && index.HasVectors() {
		if v, err := s.Embed(q); err == nil {
			qvec = v
		}
	}
	results := index.SearchHybrid(q, qvec, limit)
	if s.OnKnowledgeHits != nil && len(results) > 0 {
		s.OnKnowledgeHits(results)
	}
	return results
}
func (s *Service) KnowledgeMode() string {
	index := s.KnowledgeSnapshot()
	if index != nil && index.HasVectors() {
		return "bm25+vector(RRF)"
	}
	if index == nil {
		return "empty"
	}
	return "markdown-bm25"
}
func (s *Service) KnowledgeSnapshot() *knowledge.Index {
	s.knowledgeMu.RLock()
	defer s.knowledgeMu.RUnlock()
	return s.Knowledge
}
func (s *Service) SetKnowledge(index *knowledge.Index) {
	s.knowledgeMu.Lock()
	s.Knowledge = index
	s.knowledgeMu.Unlock()
}
func (s *Service) KnowledgeSize() int {
	if index := s.KnowledgeSnapshot(); index != nil {
		return index.Size()
	}
	return 0
}
func (s *Service) AlertProviderName() string {
	if s.AlertsProvider == nil {
		return "demo"
	}
	return s.AlertsProvider.Name()
}
func (s *Service) LogProviderName() string {
	if s.LogsProvider == nil {
		return "demo"
	}
	return s.LogsProvider.Name()
}
func (s *Service) Assets(productID string) ([]domain.Asset, error) {
	if s.AssetsProvider == nil {
		s.AssetsProvider = DemoAssetProvider{}
	}
	return s.AssetsProvider.Assets(productID)
}
func (s *Service) LookupIP(ip string) ([]domain.Asset, error) {
	if s.AssetsProvider == nil {
		s.AssetsProvider = DemoAssetProvider{}
	}
	return s.AssetsProvider.LookupIP(ip)
}
func (s *Service) AssetProviderName() string {
	if s.AssetsProvider == nil {
		return "demo"
	}
	return s.AssetsProvider.Name()
}

func safeEndpoint(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

func (s *Service) DataSources() []domain.DataSourceStatus {
	alert := domain.DataSourceStatus{Name: "alerts", Kind: "alert", Mode: s.AlertProviderName(), Status: "demo", Message: "使用内置演示告警"}
	switch p := s.AlertsProvider.(type) {
	case N9EAlertProvider:
		alert.Configured, alert.Endpoint, alert.Status, alert.Message = true, safeEndpoint(p.BaseURL), "unknown", "已配置，等待连接测试"
	case *N9EAlertProvider:
		alert.Configured, alert.Endpoint, alert.Status, alert.Message = true, safeEndpoint(p.BaseURL), "unknown", "已配置，等待连接测试"
	}
	logs := domain.DataSourceStatus{Name: "logs", Kind: "log", Mode: s.LogProviderName(), Status: "demo", Message: "使用内置演示日志"}
	switch p := s.LogsProvider.(type) {
	case LokiLogProvider:
		logs.Configured, logs.Endpoint, logs.Status, logs.Message = true, safeEndpoint(p.BaseURL), "unknown", "已配置，等待连接测试"
	case *LokiLogProvider:
		logs.Configured, logs.Endpoint, logs.Status, logs.Message = true, safeEndpoint(p.BaseURL), "unknown", "已配置，等待连接测试"
	}
	knowledgeIndex := s.KnowledgeSnapshot()
	knowledge := domain.DataSourceStatus{Name: "knowledge", Kind: "knowledge", Mode: s.KnowledgeMode(), Configured: knowledgeIndex != nil, Status: "ready", Message: "知识索引可用"}
	if knowledgeIndex == nil {
		knowledge.Status, knowledge.Message = "empty", "知识库没有启用的文档"
	}
	assets := domain.DataSourceStatus{Name: "assets", Kind: "asset", Mode: s.AssetProviderName(), Configured: s.AssetsProvider != nil, Status: "demo", Message: "使用内置演示资产"}
	if s.AssetsProvider != nil && s.AssetsProvider.Name() != "demo" {
		assets.Status, assets.Message = "unknown", "已配置，等待连接测试"
	}
	return []domain.DataSourceStatus{alert, logs, knowledge, assets}
}

func (s *Service) TestDataSource(name string) domain.DataSourceStatus {
	name = strings.ToLower(strings.TrimSpace(name))
	started := time.Now()
	var status domain.DataSourceStatus
	var count int
	var err error
	switch name {
	case "alerts":
		status = s.DataSources()[0]
		if status.Status == "demo" {
			return status
		}
		var items []domain.Alert
		items, err = s.Alerts("")
		count = len(items)
	case "logs":
		status = s.DataSources()[1]
		if status.Status == "demo" {
			return status
		}
		var items []domain.Evidence
		items, err = s.Logs("__aiops_healthcheck__", "")
		count = len(items)
	case "knowledge":
		status = s.DataSources()[2]
		count = len(s.SearchKnowledge("故障", 1))
	case "assets":
		status = s.DataSources()[3]
		if status.Status == "demo" {
			return status
		}
		var items []domain.Asset
		items, err = s.Assets("")
		count = len(items)
	default:
		return domain.DataSourceStatus{Name: name, Status: "error", Message: "未知数据源"}
	}
	status.LatencyMS = time.Since(started).Milliseconds()
	if err != nil {
		status.Status, status.Message = "error", err.Error()
		return status
	}
	status.Status = "ready"
	status.Message = fmt.Sprintf("连接正常，查询返回 %d 条记录", count)
	return status
}
