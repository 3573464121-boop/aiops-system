package tools

import (
	"aiops-mvp/internal/domain"
	"aiops-mvp/internal/knowledge"
)

type Service struct {
	Knowledge      *knowledge.Index
	AlertsProvider AlertProvider
	LogsProvider   LogProvider
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
	if s.Knowledge == nil {
		return []domain.Evidence{}
	}
	return s.Knowledge.Search(q, limit)
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
