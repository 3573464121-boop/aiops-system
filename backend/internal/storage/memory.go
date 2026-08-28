package storage

import (
	"fmt"
	"sync"

	"aiops-mvp/internal/domain"
)

type Memory struct {
	mu      sync.RWMutex
	issues  []domain.Issue
	audits  []domain.AuditEvent
	tasks    []domain.InspectionTask
	reports  []domain.InspectionReport
	memories []domain.Memory
}

func NewMemory() *Memory {
	return &Memory{issues: []domain.Issue{}, audits: []domain.AuditEvent{}, tasks: []domain.InspectionTask{}, reports: []domain.InspectionReport{}, memories: []domain.Memory{}}
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
func (m *Memory) CreateInspectionTask(v domain.InspectionTask) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tasks = append(m.tasks, v)
	return nil
}
func (m *Memory) ListInspectionTasks() ([]domain.InspectionTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]domain.InspectionTask(nil), m.tasks...), nil
}
func (m *Memory) GetInspectionTask(id string) (domain.InspectionTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, t := range m.tasks {
		if t.ID == id {
			return t, nil
		}
	}
	return domain.InspectionTask{}, fmt.Errorf("巡检任务不存在: %s", id)
}
func (m *Memory) UpdateInspectionTask(v domain.InspectionTask) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.tasks {
		if m.tasks[i].ID == v.ID {
			m.tasks[i] = v
			return nil
		}
	}
	return fmt.Errorf("巡检任务不存在: %s", v.ID)
}
func (m *Memory) DeleteInspectionTask(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := m.tasks[:0]
	for _, t := range m.tasks {
		if t.ID != id {
			out = append(out, t)
		}
	}
	m.tasks = out
	return nil
}
func (m *Memory) AddInspectionReport(v domain.InspectionReport) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reports = append([]domain.InspectionReport{v}, m.reports...)
	if len(m.reports) > 1000 {
		m.reports = m.reports[:1000]
	}
	return nil
}
func (m *Memory) ListInspectionReports(taskID string, limit int) ([]domain.InspectionReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]domain.InspectionReport, 0, len(m.reports))
	for _, r := range m.reports {
		if taskID == "" || r.TaskID == taskID {
			out = append(out, r)
		}
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (m *Memory) CreateMemory(v domain.Memory) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.memories = append([]domain.Memory{v}, m.memories...)
	return nil
}
func (m *Memory) ListMemories() ([]domain.Memory, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]domain.Memory(nil), m.memories...), nil
}
func (m *Memory) DeleteMemory(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := m.memories[:0]
	for _, v := range m.memories {
		if v.ID != id {
			out = append(out, v)
		}
	}
	m.memories = out
	return nil
}

func (m *Memory) Close() error { return nil }
