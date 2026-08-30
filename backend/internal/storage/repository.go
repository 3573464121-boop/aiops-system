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

	CreateMemory(domain.Memory) error
	ListMemories() ([]domain.Memory, error)
	DeleteMemory(id string) error

	CreateUser(domain.User) error
	GetUserByUsername(username string) (domain.User, error)
	ListUsers() ([]domain.User, error)
	CountUsers() (int, error)

	Close() error
}
