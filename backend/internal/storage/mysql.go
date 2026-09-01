package storage

import (
	"context"
	"database/sql"
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
			kind VARCHAR(16) NOT NULL DEFAULT '',
			content TEXT NOT NULL,
			source VARCHAR(32) NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL,
			INDEX idx_memories_scope_product (scope, product_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS users (
			id VARCHAR(64) PRIMARY KEY,
			username VARCHAR(64) NOT NULL UNIQUE,
			password_hash VARCHAR(255) NOT NULL,
			role VARCHAR(16) NOT NULL DEFAULT 'viewer',
			created_at DATETIME NOT NULL
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
	_, err := m.db.Exec(`INSERT INTO memories (id, scope, product_id, kind, content, source, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		v.ID, v.Scope, v.ProductID, v.Kind, v.Content, v.Source, v.CreatedAt)
	return err
}
func (m *MySQL) ListMemories() ([]domain.Memory, error) {
	rows, err := m.db.Query(`SELECT id, scope, product_id, kind, content, source, created_at FROM memories ORDER BY created_at DESC LIMIT 500`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.Memory, 0)
	for rows.Next() {
		var v domain.Memory
		if err = rows.Scan(&v.ID, &v.Scope, &v.ProductID, &v.Kind, &v.Content, &v.Source, &v.CreatedAt); err != nil {
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

func (m *MySQL) CreateUser(v domain.User) error {
	_, err := m.db.Exec(`INSERT INTO users (id, username, password_hash, role, created_at) VALUES (?, ?, ?, ?, ?)`,
		v.ID, v.Username, v.PasswordHash, v.Role, v.CreatedAt)
	return err
}
func (m *MySQL) GetUserByUsername(username string) (domain.User, error) {
	var v domain.User
	err := m.db.QueryRow(`SELECT id, username, password_hash, role, created_at FROM users WHERE username = ?`, username).
		Scan(&v.ID, &v.Username, &v.PasswordHash, &v.Role, &v.CreatedAt)
	if err != nil {
		return domain.User{}, err
	}
	return v, nil
}
func (m *MySQL) ListUsers() ([]domain.User, error) {
	rows, err := m.db.Query(`SELECT id, username, password_hash, role, created_at FROM users ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.User, 0)
	for rows.Next() {
		var v domain.User
		if err = rows.Scan(&v.ID, &v.Username, &v.PasswordHash, &v.Role, &v.CreatedAt); err != nil {
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

func (m *MySQL) Close() error { return m.db.Close() }
