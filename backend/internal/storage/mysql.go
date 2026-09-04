package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"aiops-mvp/internal/domain"
	_ "github.com/go-sql-driver/mysql"
)

type MySQL struct{ db *sql.DB }

func NewMySQL(ctx context.Context, dsn string) (*MySQL, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err = db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("杩炴帴 MySQL: %w", err)
	}
	m := &MySQL{db: db}
	if err = m.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return m, nil
}

func (m *MySQL) migrate(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS issues (
			id VARCHAR(64) PRIMARY KEY,
			product_id VARCHAR(128) NOT NULL,
			title VARCHAR(255) NOT NULL,
			diagnosis TEXT NOT NULL,
			status VARCHAR(32) NOT NULL,
			created_at DATETIME NOT NULL,
			INDEX idx_issues_product_created (product_id, created_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS audit_events (
			id VARCHAR(64) PRIMARY KEY,
			action VARCHAR(64) NOT NULL,
			product_id VARCHAR(128) NOT NULL,
			user_id VARCHAR(64) NOT NULL DEFAULT '',
			username VARCHAR(64) NOT NULL DEFAULT '',
			role VARCHAR(16) NOT NULL DEFAULT '',
			status VARCHAR(32) NOT NULL,
			duration_ms BIGINT NOT NULL,
			created_at DATETIME NOT NULL,
			INDEX idx_audits_product_created (product_id, created_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS inspection_tasks (
			id VARCHAR(64) PRIMARY KEY,
			product_id VARCHAR(128) NOT NULL,
			question VARCHAR(512) NOT NULL,
			interval_sec INT NOT NULL,
			enabled TINYINT(1) NOT NULL,
			created_at DATETIME NOT NULL,
			last_run_at DATETIME NULL,
			last_status VARCHAR(16) NOT NULL DEFAULT ''
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS inspection_reports (
			id VARCHAR(64) PRIMARY KEY,
			task_id VARCHAR(64) NOT NULL,
			product_id VARCHAR(128) NOT NULL,
			question VARCHAR(512) NOT NULL,
			summary TEXT NOT NULL,
			confidence DOUBLE NOT NULL,
			risk VARCHAR(16) NOT NULL,
			duration_ms BIGINT NOT NULL,
			created_at DATETIME NOT NULL,
			INDEX idx_reports_task_created (task_id, created_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS memories (
			id VARCHAR(64) PRIMARY KEY,
			scope VARCHAR(16) NOT NULL,
			product_id VARCHAR(128) NOT NULL DEFAULT '',
			team_id VARCHAR(64) NOT NULL DEFAULT '',
			owner_id VARCHAR(64) NOT NULL DEFAULT '',
			owner_name VARCHAR(64) NOT NULL DEFAULT '',
			kind VARCHAR(16) NOT NULL DEFAULT '',
			content TEXT NOT NULL,
			source VARCHAR(32) NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL,
			INDEX idx_memories_scope_product (scope, product_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS knowledge_documents (
			id VARCHAR(64) PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			file_path VARCHAR(1024) NOT NULL,
			content_hash CHAR(64) NOT NULL UNIQUE,
			version VARCHAR(32) NOT NULL,
			enabled TINYINT(1) NOT NULL DEFAULT 1,
			managed TINYINT(1) NOT NULL DEFAULT 0,
			chunk_count INT NOT NULL DEFAULT 0,
			hit_count BIGINT NOT NULL DEFAULT 0,
			created_by VARCHAR(64) NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			INDEX idx_knowledge_enabled_updated (enabled, updated_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS users (
			id VARCHAR(64) PRIMARY KEY,
			username VARCHAR(64) NOT NULL UNIQUE,
			password_hash VARCHAR(255) NOT NULL,
			role VARCHAR(16) NOT NULL DEFAULT 'viewer',
			team_id VARCHAR(64) NOT NULL DEFAULT 'operations',
			created_at DATETIME NOT NULL
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS approvals (
			id VARCHAR(64) PRIMARY KEY,
			product_id VARCHAR(128) NOT NULL,
			action_name VARCHAR(512) NOT NULL,
			risk VARCHAR(16) NOT NULL,
			reason TEXT NOT NULL,
			source VARCHAR(32) NOT NULL DEFAULT 'manual',
			status VARCHAR(16) NOT NULL,
			requester_id VARCHAR(64) NOT NULL DEFAULT '',
			requester_name VARCHAR(64) NOT NULL DEFAULT '',
			reviewer_id VARCHAR(64) NOT NULL DEFAULT '',
			reviewer_name VARCHAR(64) NOT NULL DEFAULT '',
			review_comment VARCHAR(512) NOT NULL DEFAULT '',
			executor_id VARCHAR(64) NOT NULL DEFAULT '',
			executor_name VARCHAR(64) NOT NULL DEFAULT '',
			execution_note VARCHAR(512) NOT NULL DEFAULT '',
			execution_mode VARCHAR(16) NOT NULL DEFAULT '',
			execution_kind VARCHAR(32) NOT NULL DEFAULT '',
			execution_output TEXT NOT NULL,
			execution_duration_ms BIGINT NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL,
			reviewed_at DATETIME NULL,
			executed_at DATETIME NULL,
			INDEX idx_approvals_status_created (status, created_at),
			INDEX idx_approvals_product_created (product_id, created_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS diagnosis_runs (
			id VARCHAR(64) PRIMARY KEY,
			product_id VARCHAR(128) NOT NULL,
			question TEXT NOT NULL,
			mode VARCHAR(32) NOT NULL,
			model VARCHAR(128) NOT NULL DEFAULT '',
			summary TEXT NOT NULL,
			confidence DOUBLE NOT NULL,
			evidence_count INT NOT NULL,
			alert_count INT NOT NULL,
			tool_call_count INT NOT NULL,
			failed_tool_count INT NOT NULL,
			knowledge_hit TINYINT(1) NOT NULL,
			memory_hit TINYINT(1) NOT NULL,
			asset_hit TINYINT(1) NOT NULL,
			duration_ms BIGINT NOT NULL,
			alert_provider VARCHAR(64) NOT NULL DEFAULT '',
			log_provider VARCHAR(64) NOT NULL DEFAULT '',
			knowledge_mode VARCHAR(64) NOT NULL DEFAULT '',
			knowledge_version VARCHAR(80) NOT NULL DEFAULT '',
			evidence_sources TEXT NOT NULL,
			tools TEXT NOT NULL,
			username VARCHAR(64) NOT NULL DEFAULT '',
			included TINYINT(1) NOT NULL DEFAULT 0,
			gold_cause TEXT NOT NULL,
			reviewer_note TEXT NOT NULL,
			reviewed_by VARCHAR(64) NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL,
			reviewed_at DATETIME NULL,
			INDEX idx_runs_created (created_at),
			INDEX idx_runs_included_created (included, created_at),
			INDEX idx_runs_product_created (product_id, created_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS fault_cases (
			id VARCHAR(64) PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			product_id VARCHAR(128) NOT NULL,
			version VARCHAR(32) NOT NULL,
			source VARCHAR(32) NOT NULL,
			payload LONGTEXT NOT NULL,
			created_by VARCHAR(64) NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL,
			INDEX idx_fault_cases_created (created_at),
			INDEX idx_fault_cases_product (product_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS replay_results (
			id VARCHAR(64) PRIMARY KEY,
			case_id VARCHAR(64) NOT NULL,
			batch_id VARCHAR(64) NOT NULL DEFAULT '',
			trial INT NOT NULL DEFAULT 1,
			config VARCHAR(32) NOT NULL,
			model VARCHAR(128) NOT NULL DEFAULT '',
			judge_model VARCHAR(128) NOT NULL DEFAULT '',
			judge_source VARCHAR(32) NOT NULL DEFAULT '',
			knowledge_version VARCHAR(80) NOT NULL DEFAULT '',
			review_status VARCHAR(16) NOT NULL DEFAULT 'pending',
			review_cause TINYINT(1) NULL,
			review_note VARCHAR(1000) NOT NULL DEFAULT '',
			reviewed_by VARCHAR(64) NOT NULL DEFAULT '',
			reviewed_at DATETIME NULL,
			cause_correct TINYINT(1) NOT NULL,
			faithfulness DOUBLE NOT NULL,
			hallucination TINYINT(1) NOT NULL,
			judged TINYINT(1) NOT NULL,
			duration_ms BIGINT NOT NULL,
			tool_failures INT NOT NULL,
			result_json LONGTEXT NOT NULL,
			created_by VARCHAR(64) NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL,
			INDEX idx_replay_case_created (case_id, created_at),
			INDEX idx_replay_config_created (config, created_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS experiment_batches (
			id VARCHAR(64) PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			case_ids LONGTEXT NOT NULL,
			configs LONGTEXT NOT NULL,
			repeats INT NOT NULL,
			model VARCHAR(128) NOT NULL DEFAULT '',
			judge_model VARCHAR(128) NOT NULL DEFAULT '',
			judge_source VARCHAR(32) NOT NULL DEFAULT '',
			knowledge_mode VARCHAR(64) NOT NULL DEFAULT '',
			knowledge_version VARCHAR(80) NOT NULL DEFAULT '',
			status VARCHAR(16) NOT NULL,
			total_runs INT NOT NULL,
			completed_runs INT NOT NULL DEFAULT 0,
			failed_runs INT NOT NULL DEFAULT 0,
			error_text VARCHAR(1000) NOT NULL DEFAULT '',
			created_by VARCHAR(64) NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL,
			completed_at DATETIME NULL,
			INDEX idx_batches_created (created_at),
			INDEX idx_batches_status_created (status, created_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	}
	for _, statement := range statements {
		if _, err := m.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("鎵ц鏁版嵁搴撹縼绉? %w", err)
		}
	}
	for _, statement := range []string{
		`ALTER TABLE audit_events ADD COLUMN user_id VARCHAR(64) NOT NULL DEFAULT ''`,
		`ALTER TABLE audit_events ADD COLUMN username VARCHAR(64) NOT NULL DEFAULT ''`,
		`ALTER TABLE audit_events ADD COLUMN role VARCHAR(16) NOT NULL DEFAULT ''`,
		`ALTER TABLE replay_results ADD COLUMN judge_model VARCHAR(128) NOT NULL DEFAULT ''`,
		`ALTER TABLE replay_results ADD COLUMN judge_source VARCHAR(32) NOT NULL DEFAULT ''`,
		`ALTER TABLE replay_results ADD COLUMN review_status VARCHAR(16) NOT NULL DEFAULT 'pending'`,
		`ALTER TABLE replay_results ADD COLUMN review_cause TINYINT(1) NULL`,
		`ALTER TABLE replay_results ADD COLUMN review_note VARCHAR(1000) NOT NULL DEFAULT ''`,
		`ALTER TABLE replay_results ADD COLUMN reviewed_by VARCHAR(64) NOT NULL DEFAULT ''`,
		`ALTER TABLE replay_results ADD COLUMN reviewed_at DATETIME NULL`,
		`ALTER TABLE replay_results ADD COLUMN batch_id VARCHAR(64) NOT NULL DEFAULT ''`,
		`ALTER TABLE replay_results ADD COLUMN trial INT NOT NULL DEFAULT 1`,
		`ALTER TABLE memories ADD COLUMN team_id VARCHAR(64) NOT NULL DEFAULT ''`,
		`ALTER TABLE memories ADD COLUMN owner_id VARCHAR(64) NOT NULL DEFAULT ''`,
		`ALTER TABLE memories ADD COLUMN owner_name VARCHAR(64) NOT NULL DEFAULT ''`,
		`ALTER TABLE users ADD COLUMN team_id VARCHAR(64) NOT NULL DEFAULT 'operations'`,
		`ALTER TABLE approvals ADD COLUMN execution_mode VARCHAR(16) NOT NULL DEFAULT ''`,
		`ALTER TABLE approvals ADD COLUMN execution_kind VARCHAR(32) NOT NULL DEFAULT ''`,
		`ALTER TABLE approvals ADD COLUMN execution_output TEXT NOT NULL`,
		`ALTER TABLE approvals ADD COLUMN execution_duration_ms BIGINT NOT NULL DEFAULT 0`,
		`ALTER TABLE diagnosis_runs ADD COLUMN knowledge_version VARCHAR(80) NOT NULL DEFAULT ''`,
		`ALTER TABLE replay_results ADD COLUMN knowledge_version VARCHAR(80) NOT NULL DEFAULT ''`,
		`ALTER TABLE experiment_batches ADD COLUMN knowledge_version VARCHAR(80) NOT NULL DEFAULT ''`,
	} {
		if _, err := m.db.ExecContext(ctx, statement); err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
			return fmt.Errorf("audit_events alter migration failed: %w", err)
		}
	}
	return nil
}

func (m *MySQL) CreateIssue(v domain.Issue) error {
	_, err := m.db.Exec(`INSERT INTO issues (id, product_id, title, diagnosis, status, created_at) VALUES (?, ?, ?, ?, ?, ?)`, v.ID, v.ProductID, v.Title, v.Diagnosis, v.Status, v.CreatedAt)
	return err
}
func (m *MySQL) ListIssues() ([]domain.Issue, error) {
	rows, err := m.db.Query(`SELECT id, product_id, title, diagnosis, status, created_at FROM issues ORDER BY created_at DESC LIMIT 500`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.Issue, 0)
	for rows.Next() {
		var v domain.Issue
		if err = rows.Scan(&v.ID, &v.ProductID, &v.Title, &v.Diagnosis, &v.Status, &v.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (m *MySQL) AddAudit(v domain.AuditEvent) error {
	_, err := m.db.Exec(`INSERT INTO audit_events (id, action, product_id, user_id, username, role, status, duration_ms, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		v.ID, v.Action, v.ProductID, v.UserID, v.Username, v.Role, v.Status, v.DurationMS, v.CreatedAt)
	return err
}
func (m *MySQL) ListAudits(limit int) ([]domain.AuditEvent, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	rows, err := m.db.Query(`SELECT id, action, product_id, user_id, username, role, status, duration_ms, created_at FROM audit_events ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.AuditEvent, 0)
	for rows.Next() {
		var v domain.AuditEvent
		if err = rows.Scan(&v.ID, &v.Action, &v.ProductID, &v.UserID, &v.Username, &v.Role, &v.Status, &v.DurationMS, &v.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func nullTime(t time.Time) sql.NullTime {
	if t.IsZero() {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: t, Valid: true}
}

func (m *MySQL) CreateInspectionTask(v domain.InspectionTask) error {
	_, err := m.db.Exec(`INSERT INTO inspection_tasks (id, product_id, question, interval_sec, enabled, created_at, last_run_at, last_status) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		v.ID, v.ProductID, v.Question, v.IntervalSec, v.Enabled, v.CreatedAt, nullTime(v.LastRunAt), v.LastStatus)
	return err
}
func scanTask(rows *sql.Rows) (domain.InspectionTask, error) {
	var v domain.InspectionTask
	var last sql.NullTime
	if err := rows.Scan(&v.ID, &v.ProductID, &v.Question, &v.IntervalSec, &v.Enabled, &v.CreatedAt, &last, &v.LastStatus); err != nil {
		return v, err
	}
	if last.Valid {
		v.LastRunAt = last.Time
	}
	return v, nil
}
func (m *MySQL) ListInspectionTasks() ([]domain.InspectionTask, error) {
	rows, err := m.db.Query(`SELECT id, product_id, question, interval_sec, enabled, created_at, last_run_at, last_status FROM inspection_tasks ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.InspectionTask, 0)
	for rows.Next() {
		v, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (m *MySQL) GetInspectionTask(id string) (domain.InspectionTask, error) {
	var v domain.InspectionTask
	var last sql.NullTime
	err := m.db.QueryRow(`SELECT id, product_id, question, interval_sec, enabled, created_at, last_run_at, last_status FROM inspection_tasks WHERE id = ?`, id).
		Scan(&v.ID, &v.ProductID, &v.Question, &v.IntervalSec, &v.Enabled, &v.CreatedAt, &last, &v.LastStatus)
	if err != nil {
		return domain.InspectionTask{}, err
	}
	if last.Valid {
		v.LastRunAt = last.Time
	}
	return v, nil
}
func (m *MySQL) UpdateInspectionTask(v domain.InspectionTask) error {
	_, err := m.db.Exec(`UPDATE inspection_tasks SET product_id=?, question=?, interval_sec=?, enabled=?, last_run_at=?, last_status=? WHERE id=?`,
		v.ProductID, v.Question, v.IntervalSec, v.Enabled, nullTime(v.LastRunAt), v.LastStatus, v.ID)
	return err
}
func (m *MySQL) DeleteInspectionTask(id string) error {
	_, err := m.db.Exec(`DELETE FROM inspection_tasks WHERE id = ?`, id)
	return err
}
func (m *MySQL) AddInspectionReport(v domain.InspectionReport) error {
	_, err := m.db.Exec(`INSERT INTO inspection_reports (id, task_id, product_id, question, summary, confidence, risk, duration_ms, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		v.ID, v.TaskID, v.ProductID, v.Question, v.Summary, v.Confidence, v.Risk, v.DurationMS, v.CreatedAt)
	return err
}
func (m *MySQL) ListInspectionReports(taskID string, limit int) ([]domain.InspectionReport, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var rows *sql.Rows
	var err error
	if taskID == "" {
		rows, err = m.db.Query(`SELECT id, task_id, product_id, question, summary, confidence, risk, duration_ms, created_at FROM inspection_reports ORDER BY created_at DESC LIMIT ?`, limit)
	} else {
		rows, err = m.db.Query(`SELECT id, task_id, product_id, question, summary, confidence, risk, duration_ms, created_at FROM inspection_reports WHERE task_id = ? ORDER BY created_at DESC LIMIT ?`, taskID, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.InspectionReport, 0)
	for rows.Next() {
		var v domain.InspectionReport
		if err = rows.Scan(&v.ID, &v.TaskID, &v.ProductID, &v.Question, &v.Summary, &v.Confidence, &v.Risk, &v.DurationMS, &v.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (m *MySQL) CreateMemory(v domain.Memory) error {
	_, err := m.db.Exec(`INSERT INTO memories (id, scope, product_id, team_id, owner_id, owner_name, kind, content, source, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		v.ID, v.Scope, v.ProductID, v.TeamID, v.OwnerID, v.OwnerName, v.Kind, v.Content, v.Source, v.CreatedAt)
	return err
}
func (m *MySQL) ListMemories() ([]domain.Memory, error) {
	rows, err := m.db.Query(`SELECT id, scope, product_id, team_id, owner_id, owner_name, kind, content, source, created_at FROM memories ORDER BY created_at DESC LIMIT 500`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.Memory, 0)
	for rows.Next() {
		var v domain.Memory
		if err = rows.Scan(&v.ID, &v.Scope, &v.ProductID, &v.TeamID, &v.OwnerID, &v.OwnerName, &v.Kind, &v.Content, &v.Source, &v.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (m *MySQL) DeleteMemory(id string) error {
	_, err := m.db.Exec(`DELETE FROM memories WHERE id = ?`, id)
	return err
}

func (m *MySQL) CreateKnowledgeDocument(v domain.KnowledgeDocument) error {
	_, err := m.db.Exec(`INSERT INTO knowledge_documents (id, name, file_path, content_hash, version, enabled, managed, chunk_count, hit_count, created_by, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		v.ID, v.Name, v.Path, v.ContentHash, v.Version, v.Enabled, v.Managed, v.ChunkCount, v.HitCount, v.CreatedBy, v.CreatedAt, v.UpdatedAt)
	return err
}

const knowledgeDocumentColumns = `id, name, file_path, content_hash, version, enabled, managed, chunk_count, hit_count, created_by, created_at, updated_at`

type knowledgeDocumentScanner interface {
	Scan(dest ...any) error
}

func scanKnowledgeDocument(scanner knowledgeDocumentScanner) (domain.KnowledgeDocument, error) {
	var v domain.KnowledgeDocument
	err := scanner.Scan(&v.ID, &v.Name, &v.Path, &v.ContentHash, &v.Version, &v.Enabled, &v.Managed, &v.ChunkCount, &v.HitCount, &v.CreatedBy, &v.CreatedAt, &v.UpdatedAt)
	return v, err
}

func (m *MySQL) ListKnowledgeDocuments() ([]domain.KnowledgeDocument, error) {
	rows, err := m.db.Query(`SELECT ` + knowledgeDocumentColumns + ` FROM knowledge_documents ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.KnowledgeDocument, 0)
	for rows.Next() {
		v, scanErr := scanKnowledgeDocument(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (m *MySQL) GetKnowledgeDocument(id string) (domain.KnowledgeDocument, error) {
	return scanKnowledgeDocument(m.db.QueryRow(`SELECT `+knowledgeDocumentColumns+` FROM knowledge_documents WHERE id = ?`, id))
}

func (m *MySQL) UpdateKnowledgeDocument(v domain.KnowledgeDocument) error {
	_, err := m.db.Exec(`UPDATE knowledge_documents SET name=?, file_path=?, content_hash=?, version=?, enabled=?, managed=?, chunk_count=?, hit_count=?, created_by=?, updated_at=? WHERE id=?`,
		v.Name, v.Path, v.ContentHash, v.Version, v.Enabled, v.Managed, v.ChunkCount, v.HitCount, v.CreatedBy, v.UpdatedAt, v.ID)
	return err
}

func (m *MySQL) DeleteKnowledgeDocument(id string) error {
	result, err := m.db.Exec(`DELETE FROM knowledge_documents WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return fmt.Errorf("knowledge document not found: %s", id)
	}
	return nil
}

func (m *MySQL) IncrementKnowledgeDocumentHits(ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	_, err := m.db.Exec(`UPDATE knowledge_documents SET hit_count = hit_count + 1 WHERE id IN (`+placeholders+`)`, args...)
	return err
}

func (m *MySQL) CreateUser(v domain.User) error {
	_, err := m.db.Exec(`INSERT INTO users (id, username, password_hash, role, team_id, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		v.ID, v.Username, v.PasswordHash, v.Role, v.TeamID, v.CreatedAt)
	return err
}
func (m *MySQL) GetUserByUsername(username string) (domain.User, error) {
	var v domain.User
	err := m.db.QueryRow(`SELECT id, username, password_hash, role, team_id, created_at FROM users WHERE username = ?`, username).
		Scan(&v.ID, &v.Username, &v.PasswordHash, &v.Role, &v.TeamID, &v.CreatedAt)
	if err != nil {
		return domain.User{}, err
	}
	return v, nil
}
func (m *MySQL) ListUsers() ([]domain.User, error) {
	rows, err := m.db.Query(`SELECT id, username, password_hash, role, team_id, created_at FROM users ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.User, 0)
	for rows.Next() {
		var v domain.User
		if err = rows.Scan(&v.ID, &v.Username, &v.PasswordHash, &v.Role, &v.TeamID, &v.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (m *MySQL) CountUsers() (int, error) {
	var n int
	err := m.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

func (m *MySQL) CreateApproval(v domain.Approval) error {
	_, err := m.db.Exec(`INSERT INTO approvals (id, product_id, action_name, risk, reason, source, status, requester_id, requester_name, reviewer_id, reviewer_name, review_comment, executor_id, executor_name, execution_note, execution_mode, execution_kind, execution_output, execution_duration_ms, created_at, reviewed_at, executed_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		v.ID, v.ProductID, v.Action, v.Risk, v.Reason, v.Source, v.Status, v.RequesterID, v.RequesterName, v.ReviewerID, v.ReviewerName, v.ReviewComment, v.ExecutorID, v.ExecutorName, v.ExecutionNote, v.ExecutionMode, v.ExecutionKind, v.ExecutionOutput, v.ExecutionDurationMS, v.CreatedAt, nullTime(v.ReviewedAt), nullTime(v.ExecutedAt))
	return err
}

const approvalColumns = `id, product_id, action_name, risk, reason, source, status, requester_id, requester_name, reviewer_id, reviewer_name, review_comment, executor_id, executor_name, execution_note, execution_mode, execution_kind, execution_output, execution_duration_ms, created_at, reviewed_at, executed_at`

type approvalScanner interface {
	Scan(dest ...any) error
}

func scanApproval(scanner approvalScanner) (domain.Approval, error) {
	var v domain.Approval
	var reviewed, executed sql.NullTime
	err := scanner.Scan(&v.ID, &v.ProductID, &v.Action, &v.Risk, &v.Reason, &v.Source, &v.Status, &v.RequesterID, &v.RequesterName, &v.ReviewerID, &v.ReviewerName, &v.ReviewComment, &v.ExecutorID, &v.ExecutorName, &v.ExecutionNote, &v.ExecutionMode, &v.ExecutionKind, &v.ExecutionOutput, &v.ExecutionDurationMS, &v.CreatedAt, &reviewed, &executed)
	if reviewed.Valid {
		v.ReviewedAt = reviewed.Time
	}
	if executed.Valid {
		v.ExecutedAt = executed.Time
	}
	return v, err
}

func (m *MySQL) ListApprovals(status string) ([]domain.Approval, error) {
	var rows *sql.Rows
	var err error
	if status == "" {
		rows, err = m.db.Query(`SELECT ` + approvalColumns + ` FROM approvals ORDER BY created_at DESC LIMIT 500`)
	} else {
		rows, err = m.db.Query(`SELECT `+approvalColumns+` FROM approvals WHERE status = ? ORDER BY created_at DESC LIMIT 500`, status)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.Approval, 0)
	for rows.Next() {
		v, err := scanApproval(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (m *MySQL) GetApproval(id string) (domain.Approval, error) {
	return scanApproval(m.db.QueryRow(`SELECT `+approvalColumns+` FROM approvals WHERE id = ?`, id))
}

func (m *MySQL) UpdateApproval(v domain.Approval) error {
	_, err := m.db.Exec(`UPDATE approvals SET status=?, reviewer_id=?, reviewer_name=?, review_comment=?, executor_id=?, executor_name=?, execution_note=?, execution_mode=?, execution_kind=?, execution_output=?, execution_duration_ms=?, reviewed_at=?, executed_at=? WHERE id=?`,
		v.Status, v.ReviewerID, v.ReviewerName, v.ReviewComment, v.ExecutorID, v.ExecutorName, v.ExecutionNote, v.ExecutionMode, v.ExecutionKind, v.ExecutionOutput, v.ExecutionDurationMS, nullTime(v.ReviewedAt), nullTime(v.ExecutedAt), v.ID)
	return err
}

func encodeStrings(v []string) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func (m *MySQL) CreateDiagnosisRun(v domain.DiagnosisRun) error {
	_, err := m.db.Exec(`INSERT INTO diagnosis_runs (id, product_id, question, mode, model, summary, confidence, evidence_count, alert_count, tool_call_count, failed_tool_count, knowledge_hit, memory_hit, asset_hit, duration_ms, alert_provider, log_provider, knowledge_mode, knowledge_version, evidence_sources, tools, username, included, gold_cause, reviewer_note, reviewed_by, created_at, reviewed_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		v.ID, v.ProductID, v.Question, v.Mode, v.Model, v.Summary, v.Confidence, v.EvidenceCount, v.AlertCount, v.ToolCallCount, v.FailedToolCount, v.KnowledgeHit, v.MemoryHit, v.AssetHit, v.DurationMS, v.AlertProvider, v.LogProvider, v.KnowledgeMode, v.KnowledgeVersion, encodeStrings(v.EvidenceSources), encodeStrings(v.Tools), v.Username, v.Included, v.GoldCause, v.ReviewerNote, v.ReviewedBy, v.CreatedAt, nullTime(v.ReviewedAt))
	return err
}

const diagnosisRunColumns = `id, product_id, question, mode, model, summary, confidence, evidence_count, alert_count, tool_call_count, failed_tool_count, knowledge_hit, memory_hit, asset_hit, duration_ms, alert_provider, log_provider, knowledge_mode, knowledge_version, evidence_sources, tools, username, included, gold_cause, reviewer_note, reviewed_by, created_at, reviewed_at`

func scanDiagnosisRun(scanner approvalScanner) (domain.DiagnosisRun, error) {
	var v domain.DiagnosisRun
	var sources, toolsJSON string
	var reviewed sql.NullTime
	err := scanner.Scan(&v.ID, &v.ProductID, &v.Question, &v.Mode, &v.Model, &v.Summary, &v.Confidence, &v.EvidenceCount, &v.AlertCount, &v.ToolCallCount, &v.FailedToolCount, &v.KnowledgeHit, &v.MemoryHit, &v.AssetHit, &v.DurationMS, &v.AlertProvider, &v.LogProvider, &v.KnowledgeMode, &v.KnowledgeVersion, &sources, &toolsJSON, &v.Username, &v.Included, &v.GoldCause, &v.ReviewerNote, &v.ReviewedBy, &v.CreatedAt, &reviewed)
	if err != nil {
		return v, err
	}
	_ = json.Unmarshal([]byte(sources), &v.EvidenceSources)
	_ = json.Unmarshal([]byte(toolsJSON), &v.Tools)
	if v.EvidenceSources == nil {
		v.EvidenceSources = []string{}
	}
	if v.Tools == nil {
		v.Tools = []string{}
	}
	if reviewed.Valid {
		v.ReviewedAt = reviewed.Time
	}
	return v, nil
}

func (m *MySQL) ListDiagnosisRuns(limit int) ([]domain.DiagnosisRun, error) {
	if limit <= 0 || limit > 2000 {
		limit = 500
	}
	rows, err := m.db.Query(`SELECT `+diagnosisRunColumns+` FROM diagnosis_runs ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.DiagnosisRun, 0)
	for rows.Next() {
		v, err := scanDiagnosisRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (m *MySQL) GetDiagnosisRun(id string) (domain.DiagnosisRun, error) {
	return scanDiagnosisRun(m.db.QueryRow(`SELECT `+diagnosisRunColumns+` FROM diagnosis_runs WHERE id = ?`, id))
}

func (m *MySQL) UpdateDiagnosisRunReview(v domain.DiagnosisRun) error {
	_, err := m.db.Exec(`UPDATE diagnosis_runs SET included=?, gold_cause=?, reviewer_note=?, reviewed_by=?, reviewed_at=? WHERE id=?`, v.Included, v.GoldCause, v.ReviewerNote, v.ReviewedBy, nullTime(v.ReviewedAt), v.ID)
	return err
}

func (m *MySQL) CreateFaultCase(v domain.FaultCase) error {
	payload, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = m.db.Exec(`INSERT INTO fault_cases (id, name, product_id, version, source, payload, created_by, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		v.ID, v.Name, v.ProductID, v.Version, v.Source, string(payload), v.CreatedBy, v.CreatedAt)
	return err
}

func scanFaultCase(scanner approvalScanner) (domain.FaultCase, error) {
	var v domain.FaultCase
	var payload string
	if err := scanner.Scan(&payload); err != nil {
		return v, err
	}
	if err := json.Unmarshal([]byte(payload), &v); err != nil {
		return v, err
	}
	return v, nil
}

func (m *MySQL) ListFaultCases() ([]domain.FaultCase, error) {
	rows, err := m.db.Query(`SELECT payload FROM fault_cases ORDER BY created_at DESC LIMIT 1000`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.FaultCase, 0)
	for rows.Next() {
		v, err := scanFaultCase(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (m *MySQL) GetFaultCase(id string) (domain.FaultCase, error) {
	return scanFaultCase(m.db.QueryRow(`SELECT payload FROM fault_cases WHERE id = ?`, id))
}

func (m *MySQL) DeleteFaultCase(id string) error {
	_, err := m.db.Exec(`DELETE FROM fault_cases WHERE id = ?`, id)
	return err
}

func (m *MySQL) CreateReplayResult(v domain.ReplayResult) error {
	payload, err := json.Marshal(v.Diagnosis)
	if err != nil {
		return err
	}
	_, err = m.db.Exec(`INSERT INTO replay_results (id, case_id, batch_id, trial, config, model, judge_model, judge_source, knowledge_version, review_status, review_cause, review_note, reviewed_by, reviewed_at, cause_correct, faithfulness, hallucination, judged, duration_ms, tool_failures, result_json, created_by, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		v.ID, v.CaseID, v.BatchID, v.Trial, v.Config, v.Model, v.JudgeModel, v.JudgeSource, v.KnowledgeVersion, "pending", nil, "", "", nil, v.CauseCorrect, v.Faithfulness, v.Hallucination, v.Judged, v.DurationMS, v.ToolFailures, string(payload), v.CreatedBy, v.CreatedAt)
	return err
}

func scanReplayResult(scanner approvalScanner) (domain.ReplayResult, error) {
	var v domain.ReplayResult
	var payload string
	var reviewCause sql.NullBool
	var reviewedAt sql.NullTime
	err := scanner.Scan(&v.ID, &v.CaseID, &v.BatchID, &v.Trial, &v.Config, &v.Model, &v.JudgeModel, &v.JudgeSource, &v.KnowledgeVersion, &v.ReviewStatus, &reviewCause, &v.ReviewNote, &v.ReviewedBy, &reviewedAt, &v.CauseCorrect, &v.Faithfulness, &v.Hallucination, &v.Judged, &v.DurationMS, &v.ToolFailures, &payload, &v.CreatedBy, &v.CreatedAt)
	if err != nil {
		return v, err
	}
	if reviewCause.Valid {
		value := reviewCause.Bool
		v.ReviewCause = &value
	}
	if reviewedAt.Valid {
		v.ReviewedAt = reviewedAt.Time
	}
	if v.ReviewStatus == "" {
		v.ReviewStatus = "pending"
	}
	if err := json.Unmarshal([]byte(payload), &v.Diagnosis); err != nil {
		return v, err
	}
	return v, nil
}

func (m *MySQL) ListReplayResults(caseID, batchID string, limit int) ([]domain.ReplayResult, error) {
	if limit <= 0 || limit > 5000 {
		limit = 1000
	}
	columns := `id, case_id, batch_id, trial, config, model, judge_model, judge_source, knowledge_version, review_status, review_cause, review_note, reviewed_by, reviewed_at, cause_correct, faithfulness, hallucination, judged, duration_ms, tool_failures, result_json, created_by, created_at`
	var rows *sql.Rows
	var err error
	if caseID == "" && batchID == "" {
		rows, err = m.db.Query(`SELECT `+columns+` FROM replay_results ORDER BY created_at DESC LIMIT ?`, limit)
	} else if caseID != "" && batchID != "" {
		rows, err = m.db.Query(`SELECT `+columns+` FROM replay_results WHERE case_id = ? AND batch_id = ? ORDER BY created_at DESC LIMIT ?`, caseID, batchID, limit)
	} else if batchID != "" {
		rows, err = m.db.Query(`SELECT `+columns+` FROM replay_results WHERE batch_id = ? ORDER BY created_at DESC LIMIT ?`, batchID, limit)
	} else {
		rows, err = m.db.Query(`SELECT `+columns+` FROM replay_results WHERE case_id = ? ORDER BY created_at DESC LIMIT ?`, caseID, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.ReplayResult, 0)
	for rows.Next() {
		v, err := scanReplayResult(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (m *MySQL) GetReplayResult(id string) (domain.ReplayResult, error) {
	columns := `id, case_id, batch_id, trial, config, model, judge_model, judge_source, knowledge_version, review_status, review_cause, review_note, reviewed_by, reviewed_at, cause_correct, faithfulness, hallucination, judged, duration_ms, tool_failures, result_json, created_by, created_at`
	return scanReplayResult(m.db.QueryRow(`SELECT `+columns+` FROM replay_results WHERE id = ?`, id))
}

func (m *MySQL) UpdateReplayResultReview(v domain.ReplayResult) error {
	var cause any
	if v.ReviewCause != nil {
		cause = *v.ReviewCause
	}
	_, err := m.db.Exec(`UPDATE replay_results SET review_status=?, review_cause=?, review_note=?, reviewed_by=?, reviewed_at=? WHERE id=?`, v.ReviewStatus, cause, v.ReviewNote, v.ReviewedBy, nullTime(v.ReviewedAt), v.ID)
	return err
}

func (m *MySQL) CreateExperimentBatch(v domain.ExperimentBatch) error {
	caseIDs, err := json.Marshal(v.CaseIDs)
	if err != nil {
		return err
	}
	configs, err := json.Marshal(v.Configs)
	if err != nil {
		return err
	}
	_, err = m.db.Exec(`INSERT INTO experiment_batches (id, name, case_ids, configs, repeats, model, judge_model, judge_source, knowledge_mode, knowledge_version, status, total_runs, completed_runs, failed_runs, error_text, created_by, created_at, completed_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		v.ID, v.Name, string(caseIDs), string(configs), v.Repeats, v.Model, v.JudgeModel, v.JudgeSource, v.KnowledgeMode, v.KnowledgeVersion, v.Status, v.TotalRuns, v.CompletedRuns, v.FailedRuns, v.Error, v.CreatedBy, v.CreatedAt, nullTime(v.CompletedAt))
	return err
}

func scanExperimentBatch(scanner approvalScanner) (domain.ExperimentBatch, error) {
	var v domain.ExperimentBatch
	var caseIDs, configs string
	var completedAt sql.NullTime
	err := scanner.Scan(&v.ID, &v.Name, &caseIDs, &configs, &v.Repeats, &v.Model, &v.JudgeModel, &v.JudgeSource, &v.KnowledgeMode, &v.KnowledgeVersion, &v.Status, &v.TotalRuns, &v.CompletedRuns, &v.FailedRuns, &v.Error, &v.CreatedBy, &v.CreatedAt, &completedAt)
	if err != nil {
		return v, err
	}
	if err = json.Unmarshal([]byte(caseIDs), &v.CaseIDs); err != nil {
		return v, err
	}
	if err = json.Unmarshal([]byte(configs), &v.Configs); err != nil {
		return v, err
	}
	if completedAt.Valid {
		v.CompletedAt = completedAt.Time
	}
	return v, nil
}

const experimentBatchColumns = `id, name, case_ids, configs, repeats, model, judge_model, judge_source, knowledge_mode, knowledge_version, status, total_runs, completed_runs, failed_runs, error_text, created_by, created_at, completed_at`

func (m *MySQL) ListExperimentBatches(limit int) ([]domain.ExperimentBatch, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	rows, err := m.db.Query(`SELECT `+experimentBatchColumns+` FROM experiment_batches ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.ExperimentBatch, 0)
	for rows.Next() {
		v, err := scanExperimentBatch(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (m *MySQL) GetExperimentBatch(id string) (domain.ExperimentBatch, error) {
	return scanExperimentBatch(m.db.QueryRow(`SELECT `+experimentBatchColumns+` FROM experiment_batches WHERE id = ?`, id))
}

func (m *MySQL) UpdateExperimentBatch(v domain.ExperimentBatch) error {
	_, err := m.db.Exec(`UPDATE experiment_batches SET status=?, completed_runs=?, failed_runs=?, error_text=?, completed_at=? WHERE id=?`, v.Status, v.CompletedRuns, v.FailedRuns, v.Error, nullTime(v.CompletedAt), v.ID)
	return err
}

func (m *MySQL) Close() error { return m.db.Close() }
