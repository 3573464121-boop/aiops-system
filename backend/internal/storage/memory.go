package storage

import (
	"sync"

	"aiops-mvp/internal/domain"
)

type Memory struct {
	mu     sync.RWMutex
	issues []domain.Issue
	audits []domain.AuditEvent
}

func NewMemory() *Memory {
	return &Memory{issues: []domain.Issue{}, audits: []domain.AuditEvent{}}
}
func (m *Memory) CreateIssue(v domain.Issue) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.issues = append([]domain.Issue{v}, m.issues...)
	return nil
}
func (m *Memory) ListIssues() ([]domain.Issue, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]domain.Issue(nil), m.issues...), nil
}
func (m *Memory) AddAudit(v domain.AuditEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.audits = append([]domain.AuditEvent{v}, m.audits...)
	if len(m.audits) > 1000 {
		m.audits = m.audits[:1000]
	}
	return nil
}
func (m *Memory) ListAudits(limit int) ([]domain.AuditEvent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if limit <= 0 || limit > len(m.audits) {
		limit = len(m.audits)
	}
	return append([]domain.AuditEvent(nil), m.audits[:limit]...), nil
}
func (m *Memory) Close() error { return nil }
