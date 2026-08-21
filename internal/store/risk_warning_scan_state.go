package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"jiyi/mochat-go/internal/dashboard"
)

func scanStateTime(value sql.NullTime) string {
	if !value.Valid {
		return ""
	}
	return value.Time.Format("2006-01-02 15:04:05")
}

func (s *MySQLStore) SensitiveWordScanStatus(ctx context.Context, corpID int) (dashboard.SensitiveWordScanStatus, error) {
	var state dashboard.SensitiveWordScanStatus
	var lastAttempt, lastSuccess, lastFailure sql.NullTime
	err := s.db.QueryRowContext(ctx, `
		SELECT state, last_attempt_at, last_success_at, last_failure_at, last_error
		FROM mochat_go_sensitive_word_scan_states
		WHERE corp_id = ?
	`, corpID).Scan(&state.State, &lastAttempt, &lastSuccess, &lastFailure, &state.LastError)
	if err != nil {
		if err == sql.ErrNoRows {
			return dashboard.SensitiveWordScanStatus{State: "never_run"}, nil
		}
		return dashboard.SensitiveWordScanStatus{}, err
	}
	state.LastAttemptAt = scanStateTime(lastAttempt)
	state.LastSuccessAt = scanStateTime(lastSuccess)
	state.LastFailureAt = scanStateTime(lastFailure)
	return state, nil
}

type sensitiveWordScanStateWriter interface {
	RecordSensitiveWordScanStarted(context.Context, int) error
	RecordSensitiveWordScanFinished(context.Context, int, error) error
}

func (s *MySQLStore) RecordSensitiveWordScanStarted(ctx context.Context, corpID int) error {
	if corpID <= 0 {
		return fmt.Errorf("corp id must be positive")
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO mochat_go_sensitive_word_scan_states (corp_id, state, last_attempt_at, last_error, updated_at)
		VALUES (?, 'running', NOW(), '', NOW())
		ON DUPLICATE KEY UPDATE state='running', last_attempt_at=NOW(), last_error='', updated_at=NOW()
	`, corpID)
	return err
}

func (s *MySQLStore) RecordSensitiveWordScanFinished(ctx context.Context, corpID int, scanErr error) error {
	if corpID <= 0 {
		return fmt.Errorf("corp id must be positive")
	}
	state := "ready"
	if scanErr != nil {
		state = "failed"
	}
	errorText := ""
	if scanErr != nil {
		errorText = strings.TrimSpace(scanErr.Error())
		if len([]rune(errorText)) > 500 {
			errorText = string([]rune(errorText)[:500])
		}
	}
	if scanErr == nil {
		_, err := s.db.ExecContext(ctx, `
			INSERT INTO mochat_go_sensitive_word_scan_states (corp_id, state, last_attempt_at, last_success_at, last_failure_at, last_error, updated_at)
			VALUES (?, 'ready', NOW(), NOW(), NULL, '', NOW())
			ON DUPLICATE KEY UPDATE state='ready', last_success_at=NOW(), last_error='', updated_at=NOW()
		`, corpID)
		return err
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO mochat_go_sensitive_word_scan_states (corp_id, state, last_attempt_at, last_failure_at, last_error, updated_at)
		VALUES (?, ?, NOW(), NOW(), ?, NOW())
		ON DUPLICATE KEY UPDATE state=?, last_failure_at=NOW(), last_error=?, updated_at=NOW()
	`, corpID, state, errorText, state, errorText)
	return err
}
