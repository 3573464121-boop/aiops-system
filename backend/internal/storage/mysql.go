package storage

import (
	"context"
	"database/sql"
	"fmt"
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
			status VARCHAR(32) NOT NULL,
			duration_ms BIGINT NOT NULL,
			created_at DATETIME NOT NULL,
			INDEX idx_audits_product_created (product_id, created_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	}
	for _, statement := range statements {
		if _, err := m.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("鎵ц鏁版嵁搴撹縼绉? %w", err)
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
	_, err := m.db.Exec(`INSERT INTO audit_events (id, action, product_id, status, duration_ms, created_at) VALUES (?, ?, ?, ?, ?, ?)`, v.ID, v.Action, v.ProductID, v.Status, v.DurationMS, v.CreatedAt)
	return err
}
func (m *MySQL) ListAudits(limit int) ([]domain.AuditEvent, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	rows, err := m.db.Query(`SELECT id, action, product_id, status, duration_ms, created_at FROM audit_events ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.AuditEvent, 0)
	for rows.Next() {
		var v domain.AuditEvent
		if err = rows.Scan(&v.ID, &v.Action, &v.ProductID, &v.Status, &v.DurationMS, &v.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (m *MySQL) Close() error { return m.db.Close() }
