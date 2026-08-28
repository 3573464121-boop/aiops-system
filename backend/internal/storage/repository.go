package storage

import "aiops-mvp/internal/domain"

// Repository 负责工单与审计持久化；实现可替换为内存或 MySQL。
type Repository interface {
	CreateIssue(domain.Issue) error
	ListIssues() ([]domain.Issue, error)
	AddAudit(domain.AuditEvent) error
	ListAudits(limit int) ([]domain.AuditEvent, error)

	CreateInspectionTask(domain.InspectionTask) error
	ListInspectionTasks() ([]domain.InspectionTask, error)
	GetInspectionTask(id string) (domain.InspectionTask, error)
	UpdateInspectionTask(domain.InspectionTask) error
	DeleteInspectionTask(id string) error
	AddInspectionReport(domain.InspectionReport) error
	ListInspectionReports(taskID string, limit int) ([]domain.InspectionReport, error)

	Close() error
}
