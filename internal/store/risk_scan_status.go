package store

import (
	"context"
	"database/sql"

	"jiyi/mochat-go/internal/dashboard"
)

func (s *MySQLStore) RiskScanStatus(ctx context.Context, tenantID int, corpID int) (dashboard.RiskScanStatus, error) {
	var state dashboard.RiskScanStatus
	var lastAttempt, lastSuccess, lastFailure sql.NullTime
	err := s.db.QueryRowContext(ctx, `
		SELECT state, last_attempt_at, last_success_at, last_failure_at, last_error
		FROM mochat_go_risk_scan_states
		WHERE tenant_id = ? AND corp_id = ?
	`, tenantID, corpID).Scan(&state.State, &lastAttempt, &lastSuccess, &lastFailure, &state.LastError)
	if err != nil {
		if err == sql.ErrNoRows {
			return dashboard.RiskScanStatus{State: "never_run"}, nil
		}
		return dashboard.RiskScanStatus{}, err
	}
	state.LastAttemptAt = scanStateTime(lastAttempt)
	state.LastSuccessAt = scanStateTime(lastSuccess)
	state.LastFailureAt = scanStateTime(lastFailure)
	return state, nil
}
