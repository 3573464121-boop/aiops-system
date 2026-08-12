package storage

import "aiops-mvp/internal/domain"

// Repository 负责工单与审计持久化；实现可替换为内存或 MySQL。
type Repository interface {
	CreateIssue(domain.Issue) error
	ListIssues() ([]domain.Issue, error)
	AddAudit(domain.AuditEvent) error
	ListAudits(limit int) ([]domain.AuditEvent, error)
	Close() error
}
