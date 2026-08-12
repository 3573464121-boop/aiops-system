package tools

import "aiops-mvp/internal/domain"

// AlertProvider 与 LogProvider 隔离外部平台，后续接入 Nightingale、Loki、ES 或 ClickHouse 时无需修改诊断编排。
type AlertProvider interface {
	Name() string
	Alerts(productID string) ([]domain.Alert, error)
}

type LogProvider interface {
	Name() string
	Search(productID, query string) ([]domain.Evidence, error)
}
