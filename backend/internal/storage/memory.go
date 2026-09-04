package storage

import (
	"fmt"
	"sync"

	"aiops-mvp/internal/domain"
)

type Memory struct {
	mu        sync.RWMutex
	issues    []domain.Issue
	audits    []domain.AuditEvent
	tasks     []domain.InspectionTask
	reports   []domain.InspectionReport
	memories  []domain.Memory
	documents []domain.KnowledgeDocument
	users     []domain.User
	approvals []domain.Approval
	runs      []domain.DiagnosisRun
	cases     []domain.FaultCase
	replays   []domain.ReplayResult
	batches   []domain.ExperimentBatch
}

func NewMemory() *Memory {
	return &Memory{issues: []domain.Issue{}, audits: []domain.AuditEvent{}, tasks: []domain.InspectionTask{}, reports: []domain.InspectionReport{}, memories: []domain.Memory{}, documents: []domain.KnowledgeDocument{}, users: []domain.User{}, approvals: []domain.Approval{}, runs: []domain.DiagnosisRun{}, cases: []domain.FaultCase{}, replays: []domain.ReplayResult{}, batches: []domain.ExperimentBatch{}}
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

func (m *Memory) CreateKnowledgeDocument(v domain.KnowledgeDocument) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, doc := range m.documents {
		if doc.ID == v.ID || doc.ContentHash == v.ContentHash {
			return fmt.Errorf("knowledge document already exists: %s", v.Name)
		}
	}
	m.documents = append([]domain.KnowledgeDocument{v}, m.documents...)
	return nil
}
func (m *Memory) ListKnowledgeDocuments() ([]domain.KnowledgeDocument, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]domain.KnowledgeDocument(nil), m.documents...), nil
}
func (m *Memory) GetKnowledgeDocument(id string) (domain.KnowledgeDocument, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, doc := range m.documents {
		if doc.ID == id {
			return doc, nil
		}
	}
	return domain.KnowledgeDocument{}, fmt.Errorf("knowledge document not found: %s", id)
}
func (m *Memory) UpdateKnowledgeDocument(v domain.KnowledgeDocument) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.documents {
		if m.documents[i].ID == v.ID {
			m.documents[i] = v
			return nil
		}
	}
	return fmt.Errorf("knowledge document not found: %s", v.ID)
}
func (m *Memory) DeleteKnowledgeDocument(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, doc := range m.documents {
		if doc.ID == id {
			m.documents = append(m.documents[:i], m.documents[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("knowledge document not found: %s", id)
}
func (m *Memory) IncrementKnowledgeDocumentHits(ids []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.documents {
		for _, id := range ids {
			if m.documents[i].ID == id {
				m.documents[i].HitCount++
				break
			}
		}
	}
	return nil
}

func (m *Memory) CreateUser(v domain.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, u := range m.users {
		if u.Username == v.Username {
			return fmt.Errorf("用户名已存在: %s", v.Username)
		}
	}
	m.users = append(m.users, v)
	return nil
}
func (m *Memory) GetUserByUsername(username string) (domain.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, u := range m.users {
		if u.Username == username {
			return u, nil
		}
	}
	return domain.User{}, fmt.Errorf("用户不存在: %s", username)
}
func (m *Memory) ListUsers() ([]domain.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]domain.User(nil), m.users...), nil
}
func (m *Memory) CountUsers() (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.users), nil
}

func (m *Memory) CreateApproval(v domain.Approval) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.approvals = append([]domain.Approval{v}, m.approvals...)
	return nil
}
func (m *Memory) ListApprovals(status string) ([]domain.Approval, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]domain.Approval, 0, len(m.approvals))
	for _, v := range m.approvals {
		if status == "" || v.Status == status {
			out = append(out, v)
		}
	}
	return out, nil
}
func (m *Memory) GetApproval(id string) (domain.Approval, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, v := range m.approvals {
		if v.ID == id {
			return v, nil
		}
	}
	return domain.Approval{}, fmt.Errorf("审批单不存在: %s", id)
}
func (m *Memory) UpdateApproval(v domain.Approval) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.approvals {
		if m.approvals[i].ID == v.ID {
			m.approvals[i] = v
			return nil
		}
	}
	return fmt.Errorf("审批单不存在: %s", v.ID)
}

func (m *Memory) CreateDiagnosisRun(v domain.DiagnosisRun) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.runs = append([]domain.DiagnosisRun{v}, m.runs...)
	if len(m.runs) > 5000 {
		m.runs = m.runs[:5000]
	}
	return nil
}
func (m *Memory) ListDiagnosisRuns(limit int) ([]domain.DiagnosisRun, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if limit <= 0 || limit > len(m.runs) {
		limit = len(m.runs)
	}
	return append([]domain.DiagnosisRun(nil), m.runs[:limit]...), nil
}
func (m *Memory) GetDiagnosisRun(id string) (domain.DiagnosisRun, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, v := range m.runs {
		if v.ID == id {
			return v, nil
		}
	}
	return domain.DiagnosisRun{}, fmt.Errorf("诊断实验记录不存在: %s", id)
}
func (m *Memory) UpdateDiagnosisRunReview(v domain.DiagnosisRun) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.runs {
		if m.runs[i].ID == v.ID {
			m.runs[i] = v
			return nil
		}
	}
	return fmt.Errorf("诊断实验记录不存在: %s", v.ID)
}

func (m *Memory) CreateFaultCase(v domain.FaultCase) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cases = append([]domain.FaultCase{v}, m.cases...)
	return nil
}
func (m *Memory) ListFaultCases() ([]domain.FaultCase, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]domain.FaultCase(nil), m.cases...), nil
}
func (m *Memory) GetFaultCase(id string) (domain.FaultCase, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, v := range m.cases {
		if v.ID == id {
			return v, nil
		}
	}
	return domain.FaultCase{}, fmt.Errorf("故障案例不存在: %s", id)
}
func (m *Memory) DeleteFaultCase(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := m.cases[:0]
	for _, v := range m.cases {
		if v.ID != id {
			out = append(out, v)
		}
	}
	m.cases = out
	return nil
}
func (m *Memory) CreateReplayResult(v domain.ReplayResult) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.replays = append([]domain.ReplayResult{v}, m.replays...)
	return nil
}
func (m *Memory) ListReplayResults(caseID, batchID string, limit int) ([]domain.ReplayResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]domain.ReplayResult, 0)
	for _, v := range m.replays {
		if (caseID == "" || v.CaseID == caseID) && (batchID == "" || v.BatchID == batchID) {
			out = append(out, v)
		}
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (m *Memory) CreateExperimentBatch(v domain.ExperimentBatch) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.batches = append([]domain.ExperimentBatch{v}, m.batches...)
	return nil
}
func (m *Memory) ListExperimentBatches(limit int) ([]domain.ExperimentBatch, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if limit <= 0 || limit > len(m.batches) {
		limit = len(m.batches)
	}
	return append([]domain.ExperimentBatch(nil), m.batches[:limit]...), nil
}
func (m *Memory) GetExperimentBatch(id string) (domain.ExperimentBatch, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, v := range m.batches {
		if v.ID == id {
			return v, nil
		}
	}
	return domain.ExperimentBatch{}, fmt.Errorf("实验批次不存在: %s", id)
}
func (m *Memory) UpdateExperimentBatch(v domain.ExperimentBatch) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.batches {
		if m.batches[i].ID == v.ID {
			m.batches[i] = v
			return nil
		}
	}
	return fmt.Errorf("实验批次不存在: %s", v.ID)
}
func (m *Memory) GetReplayResult(id string) (domain.ReplayResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, v := range m.replays {
		if v.ID == id {
			return v, nil
		}
	}
	return domain.ReplayResult{}, fmt.Errorf("回放结果不存在: %s", id)
}
func (m *Memory) UpdateReplayResultReview(v domain.ReplayResult) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.replays {
		if m.replays[i].ID == v.ID {
			m.replays[i] = v
			return nil
		}
	}
	return fmt.Errorf("回放结果不存在: %s", v.ID)
}

func (m *Memory) Close() error { return nil }
