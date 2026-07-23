package taskrunner

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"
)

const (
	backgroundTaskTable          = "mochat_go_background_tasks"
	backgroundTaskRunTable       = "mochat_go_background_task_runs"
	backgroundTaskExecutionTable = "mochat_go_background_task_executions"
)

type SQLRecorder struct {
	db     *sql.DB
	logger *log.Logger
}

func NewSQLRecorder(db *sql.DB, logger *log.Logger) *SQLRecorder {
	if logger == nil {
		logger = log.Default()
	}
	return &SQLRecorder{db: db, logger: logger}
}

func (r *SQLRecorder) Ensure(ctx context.Context) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("sql recorder database is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	_, err := r.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS mochat_go_background_tasks (
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
			name VARCHAR(128) NOT NULL,
			status VARCHAR(32) NOT NULL,
			started_at DATETIME NULL,
			stopped_at DATETIME NULL,
			last_error TEXT NULL,
			start_count BIGINT UNSIGNED NOT NULL DEFAULT 0,
			failure_count BIGINT UNSIGNED NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			PRIMARY KEY (id),
			UNIQUE KEY uniq_mochat_go_background_tasks_name (name),
			KEY idx_mochat_go_background_tasks_status_updated (status, updated_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
	`)
	if err != nil {
		return fmt.Errorf("ensure %s: %w", backgroundTaskTable, err)
	}
	_, err = r.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS mochat_go_background_task_runs (
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
			run_id VARCHAR(128) NOT NULL,
			name VARCHAR(128) NOT NULL,
			status VARCHAR(32) NOT NULL,
			started_at DATETIME NULL,
			stopped_at DATETIME NULL,
			last_error TEXT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			PRIMARY KEY (id),
			UNIQUE KEY uniq_mochat_go_background_task_runs_run_id (run_id),
			KEY idx_mochat_go_background_task_runs_name_started (name, started_at),
			KEY idx_mochat_go_background_task_runs_status_updated (status, updated_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
	`)
	if err != nil {
		return fmt.Errorf("ensure %s: %w", backgroundTaskRunTable, err)
	}
	_, err = r.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS mochat_go_background_task_executions (
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
			tenant_id INT NOT NULL DEFAULT 0,
			execution_id VARCHAR(160) NOT NULL,
			task_run_id VARCHAR(128) NULL,
			task_name VARCHAR(128) NOT NULL,
			kind VARCHAR(64) NOT NULL,
			status VARCHAR(32) NOT NULL,
			started_at DATETIME NULL,
			stopped_at DATETIME NULL,
			duration_ms BIGINT NULL,
			last_error TEXT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			PRIMARY KEY (id),
			UNIQUE KEY uniq_mochat_go_background_task_executions_id (execution_id),
			KEY idx_mochat_go_background_task_executions_task_run (task_run_id),
			KEY idx_mochat_go_background_task_executions_task_kind_started (task_name, kind, started_at),
			KEY idx_mochat_go_background_task_executions_tenant_kind_status (tenant_id, kind, status, started_at),
			KEY idx_mochat_go_background_task_executions_status_updated (status, updated_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
	`)
	if err != nil {
		return fmt.Errorf("ensure %s: %w", backgroundTaskExecutionTable, err)
	}
	if err := r.ensureExecutionTenantMetadata(ctx); err != nil {
		return err
	}
	return nil
}

func (r *SQLRecorder) ensureExecutionTenantMetadata(ctx context.Context) error {
	if err := r.ensureColumn(ctx, backgroundTaskExecutionTable, "tenant_id", `
		ALTER TABLE mochat_go_background_task_executions
		ADD COLUMN tenant_id INT NOT NULL DEFAULT 0 AFTER id
	`); err != nil {
		return err
	}
	return r.ensureIndex(ctx, backgroundTaskExecutionTable, "idx_mochat_go_background_task_executions_tenant_kind_status", `
		ALTER TABLE mochat_go_background_task_executions
		ADD KEY idx_mochat_go_background_task_executions_tenant_kind_status (tenant_id, kind, status, started_at)
	`)
}

func (r *SQLRecorder) ensureColumn(ctx context.Context, table string, column string, alterSQL string) error {
	var count int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND COLUMN_NAME = ?
	`, table, column).Scan(&count)
	if err != nil {
		return fmt.Errorf("inspect %s column %s: %w", table, column, err)
	}
	if count > 0 {
		return nil
	}
	if _, err := r.db.ExecContext(ctx, alterSQL); err != nil {
		return fmt.Errorf("alter %s add column %s: %w", table, column, err)
	}
	return nil
}

func (r *SQLRecorder) ensureIndex(ctx context.Context, table string, indexName string, alterSQL string) error {
	var count int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM information_schema.STATISTICS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND INDEX_NAME = ?
	`, table, indexName).Scan(&count)
	if err != nil {
		return fmt.Errorf("inspect %s index %s: %w", table, indexName, err)
	}
	if count > 0 {
		return nil
	}
	if _, err := r.db.ExecContext(ctx, alterSQL); err != nil {
		return fmt.Errorf("alter %s add index %s: %w", table, indexName, err)
	}
	return nil
}

func (r *SQLRecorder) RecordTaskSnapshot(ctx context.Context, snapshot Snapshot) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("sql recorder database is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	name := strings.TrimSpace(snapshot.Name)
	if name == "" {
		return fmt.Errorf("task name is required")
	}
	startedAt, err := nullableSnapshotTime(snapshot.StartedAt)
	if err != nil {
		return err
	}
	stoppedAt, err := nullableSnapshotTime(snapshot.StoppedAt)
	if err != nil {
		return err
	}
	var lastError any
	if strings.TrimSpace(snapshot.Error) != "" {
		lastError = snapshot.Error
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("record %s %s: %w", backgroundTaskTable, name, err)
	}
	defer func() {
		_ = tx.Rollback()
	}()
	_, err = tx.ExecContext(ctx, `
		INSERT INTO mochat_go_background_tasks (
			name, status, started_at, stopped_at, last_error,
			start_count, failure_count, created_at, updated_at
		)
		VALUES (
			?, ?, ?, ?, ?,
			IF(? = 'running', 1, 0),
			IF(? = 'failed', 1, 0),
			NOW(), NOW()
		)
		ON DUPLICATE KEY UPDATE
			status = VALUES(status),
			started_at = COALESCE(VALUES(started_at), started_at),
			stopped_at = VALUES(stopped_at),
			last_error = VALUES(last_error),
			start_count = start_count + IF(VALUES(status) = 'running', 1, 0),
			failure_count = failure_count + IF(VALUES(status) = 'failed', 1, 0),
			updated_at = NOW()
	`, name, snapshot.Status, startedAt, stoppedAt, lastError, snapshot.Status, snapshot.Status)
	if err != nil {
		return fmt.Errorf("record %s %s: %w", backgroundTaskTable, name, err)
	}
	runID := strings.TrimSpace(snapshot.RunID)
	if runID != "" {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO mochat_go_background_task_runs (
				run_id, name, status, started_at, stopped_at, last_error, created_at, updated_at
			)
			VALUES (?, ?, ?, ?, ?, ?, NOW(), NOW())
			ON DUPLICATE KEY UPDATE
				name = VALUES(name),
				status = VALUES(status),
				started_at = COALESCE(VALUES(started_at), started_at),
				stopped_at = VALUES(stopped_at),
				last_error = VALUES(last_error),
				updated_at = NOW()
		`, runID, name, snapshot.Status, startedAt, stoppedAt, lastError)
		if err != nil {
			return fmt.Errorf("record %s %s: %w", backgroundTaskRunTable, name, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit task snapshot %s: %w", name, err)
	}
	return nil
}

func (r *SQLRecorder) RecordTaskExecution(ctx context.Context, snapshot ExecutionSnapshot) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("sql recorder database is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	executionID := strings.TrimSpace(snapshot.ExecutionID)
	if executionID == "" {
		return fmt.Errorf("task execution id is required")
	}
	taskName := strings.TrimSpace(snapshot.TaskName)
	if taskName == "" {
		return fmt.Errorf("task name is required")
	}
	kind := strings.TrimSpace(snapshot.Kind)
	if kind == "" {
		return fmt.Errorf("task execution kind is required")
	}
	startedAt, err := parseSnapshotTime(snapshot.StartedAt)
	if err != nil {
		return err
	}
	stoppedAt, err := parseSnapshotTime(snapshot.StoppedAt)
	if err != nil {
		return err
	}
	var durationMS any
	if startedAt != nil && stoppedAt != nil {
		duration := stoppedAt.Sub(*startedAt)
		if duration < 0 {
			duration = 0
		}
		durationMS = duration.Milliseconds()
	}
	var runID any
	if strings.TrimSpace(snapshot.RunID) != "" {
		runID = strings.TrimSpace(snapshot.RunID)
	}
	var lastError any
	if strings.TrimSpace(snapshot.Error) != "" {
		lastError = snapshot.Error
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO mochat_go_background_task_executions (
			tenant_id, execution_id, task_run_id, task_name, kind, status,
			started_at, stopped_at, duration_ms, last_error, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())
		ON DUPLICATE KEY UPDATE
			tenant_id = VALUES(tenant_id),
			task_run_id = VALUES(task_run_id),
			task_name = VALUES(task_name),
			kind = VALUES(kind),
			status = VALUES(status),
			started_at = COALESCE(VALUES(started_at), started_at),
			stopped_at = VALUES(stopped_at),
			duration_ms = VALUES(duration_ms),
			last_error = VALUES(last_error),
			updated_at = NOW()
	`, snapshot.TenantID, executionID, runID, taskName, kind, snapshot.Status, nullableTime(startedAt), nullableTime(stoppedAt), durationMS, lastError)
	if err != nil {
		return fmt.Errorf("record %s %s: %w", backgroundTaskExecutionTable, taskName, err)
	}
	return nil
}

func nullableSnapshotTime(value string) (any, error) {
	parsed, err := parseSnapshotTime(value)
	if err != nil || parsed == nil {
		return nil, err
	}
	return *parsed, nil
}

func parseSnapshotTime(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, fmt.Errorf("parse task snapshot time %q: %w", value, err)
	}
	return &parsed, nil
}

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return *value
}
