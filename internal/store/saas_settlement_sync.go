package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"jiyi/mochat-go/internal/dashboard"
)

const saasPaymentSettlementSyncQueryMaxLimit = 5000

func (s *MySQLStore) BeginSaaSPaymentSettlementSync(ctx context.Context, input dashboard.SaaSPaymentSettlementSyncBegin) (dashboard.SaaSPaymentSettlementSyncRun, dashboard.SaaSPaymentSettlementSyncState, error) {
	provider := strings.ToLower(strings.TrimSpace(input.Provider))
	runNo := strings.TrimSpace(input.RunNo)
	if provider == "" || runNo == "" || input.ActorTenantID <= 0 {
		return dashboard.SaaSPaymentSettlementSyncRun{}, dashboard.SaaSPaymentSettlementSyncState{}, dashboard.NewSaaSAdminBadRequest("payment settlement sync begin fields invalid")
	}
	if input.StaleAfterSeconds <= 0 {
		input.StaleAfterSeconds = 15 * 60
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.SaaSPaymentSettlementSyncRun{}, dashboard.SaaSPaymentSettlementSyncState{}, err
	}
	defer rollbackQuietly(tx)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_saas_payment_settlement_sync_states
			(provider, `+"`cursor`"+`, active_run_id, last_error, version, created_at, updated_at)
		VALUES (?, '', NULL, '', 1, NOW(), NOW())
		ON DUPLICATE KEY UPDATE provider = VALUES(provider)
	`, provider); err != nil {
		return dashboard.SaaSPaymentSettlementSyncRun{}, dashboard.SaaSPaymentSettlementSyncState{}, err
	}
	state, found, err := saasPaymentSettlementSyncStateByProviderTx(ctx, tx, provider, true)
	if err != nil {
		return dashboard.SaaSPaymentSettlementSyncRun{}, dashboard.SaaSPaymentSettlementSyncState{}, err
	}
	if !found {
		return dashboard.SaaSPaymentSettlementSyncRun{}, dashboard.SaaSPaymentSettlementSyncState{}, errors.New("payment settlement sync state insert did not return a row")
	}
	if state.ActiveRunID > 0 {
		active, runFound, runErr := saasPaymentSettlementSyncRunByIDTx(ctx, tx, state.ActiveRunID, true)
		if runErr != nil {
			return dashboard.SaaSPaymentSettlementSyncRun{}, dashboard.SaaSPaymentSettlementSyncState{}, runErr
		}
		stale := true
		if runFound && active.Status == dashboard.SaaSPaymentSettlementSyncStatusRunning {
			var isStale int
			if err := tx.QueryRowContext(ctx, `
				SELECT CASE WHEN started_at IS NULL OR TIMESTAMPDIFF(SECOND, started_at, NOW()) >= ? THEN 1 ELSE 0 END
				FROM mochat_go_saas_payment_settlement_sync_runs WHERE id = ?
			`, input.StaleAfterSeconds, active.ID).Scan(&isStale); err != nil {
				return dashboard.SaaSPaymentSettlementSyncRun{}, dashboard.SaaSPaymentSettlementSyncState{}, err
			}
			stale = isStale == 1
			if !stale {
				return dashboard.SaaSPaymentSettlementSyncRun{}, dashboard.SaaSPaymentSettlementSyncState{}, &dashboard.SaaSAdminOperationError{Status: 409, Message: fmt.Sprintf("provider %s already has a running settlement sync", provider)}
			}
		}
		if runFound && active.Status == dashboard.SaaSPaymentSettlementSyncStatusRunning && stale {
			if _, err := tx.ExecContext(ctx, `
				UPDATE mochat_go_saas_payment_settlement_sync_runs
				SET status = 'failed', finished_at = NOW(), error_message = 'stale sync run recovered by a new run', updated_at = NOW()
				WHERE id = ? AND status = 'running'
			`, active.ID); err != nil {
				return dashboard.SaaSPaymentSettlementSyncRun{}, dashboard.SaaSPaymentSettlementSyncState{}, err
			}
		}
	}
	result, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_saas_payment_settlement_sync_runs
			(run_no, provider, source, status, dry_run, cursor_before, cursor_after,
			 actor_user_id, actor_tenant_id, started_at, created_at, updated_at)
		VALUES (?, ?, ?, 'running', ?, ?, '', ?, ?, NOW(), NOW(), NOW())
	`, runNo, provider, strings.TrimSpace(input.Source), boolTinyInt(input.DryRun), state.Cursor, input.ActorUserID, input.ActorTenantID)
	if err != nil {
		if isMySQLDuplicateKeyError(err) {
			return dashboard.SaaSPaymentSettlementSyncRun{}, dashboard.SaaSPaymentSettlementSyncState{}, saasPaymentConflict("payment settlement sync run already exists")
		}
		return dashboard.SaaSPaymentSettlementSyncRun{}, dashboard.SaaSPaymentSettlementSyncState{}, err
	}
	runID, err := result.LastInsertId()
	if err != nil {
		return dashboard.SaaSPaymentSettlementSyncRun{}, dashboard.SaaSPaymentSettlementSyncState{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_payment_settlement_sync_states
		SET active_run_id = ?, last_attempt_at = NOW(), last_error = '', version = version + 1, updated_at = NOW()
		WHERE id = ?
	`, runID, state.ID); err != nil {
		return dashboard.SaaSPaymentSettlementSyncRun{}, dashboard.SaaSPaymentSettlementSyncState{}, err
	}
	run, found, err := saasPaymentSettlementSyncRunByIDTx(ctx, tx, runID, false)
	if err != nil || !found {
		if err == nil {
			err = errors.New("payment settlement sync run insert did not return a row")
		}
		return dashboard.SaaSPaymentSettlementSyncRun{}, dashboard.SaaSPaymentSettlementSyncState{}, err
	}
	state, _, err = saasPaymentSettlementSyncStateByProviderTx(ctx, tx, provider, false)
	if err != nil {
		return dashboard.SaaSPaymentSettlementSyncRun{}, dashboard.SaaSPaymentSettlementSyncState{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSPaymentSettlementSyncRun{}, dashboard.SaaSPaymentSettlementSyncState{}, err
	}
	return run, state, nil
}

func (s *MySQLStore) FinishSaaSPaymentSettlementSync(ctx context.Context, input dashboard.SaaSPaymentSettlementSyncFinish) (dashboard.SaaSPaymentSettlementSyncRun, dashboard.SaaSPaymentSettlementSyncState, error) {
	provider := strings.ToLower(strings.TrimSpace(input.Provider))
	if input.RunID <= 0 || provider == "" || (input.Status != dashboard.SaaSPaymentSettlementSyncStatusSucceeded && input.Status != dashboard.SaaSPaymentSettlementSyncStatusPreviewed) {
		return dashboard.SaaSPaymentSettlementSyncRun{}, dashboard.SaaSPaymentSettlementSyncState{}, dashboard.NewSaaSAdminBadRequest("payment settlement sync finish fields invalid")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.SaaSPaymentSettlementSyncRun{}, dashboard.SaaSPaymentSettlementSyncState{}, err
	}
	defer rollbackQuietly(tx)
	state, found, err := saasPaymentSettlementSyncStateByProviderTx(ctx, tx, provider, true)
	if err != nil || !found {
		if err == nil {
			err = dashboard.NewSaaSAdminNotFound("payment settlement sync state not found")
		}
		return dashboard.SaaSPaymentSettlementSyncRun{}, dashboard.SaaSPaymentSettlementSyncState{}, err
	}
	run, found, err := saasPaymentSettlementSyncRunByIDTx(ctx, tx, input.RunID, true)
	if err != nil || !found {
		if err == nil {
			err = dashboard.NewSaaSAdminNotFound("payment settlement sync run not found")
		}
		return dashboard.SaaSPaymentSettlementSyncRun{}, dashboard.SaaSPaymentSettlementSyncState{}, err
	}
	if state.ActiveRunID != run.ID || run.Provider != provider || run.Status != dashboard.SaaSPaymentSettlementSyncStatusRunning {
		return dashboard.SaaSPaymentSettlementSyncRun{}, dashboard.SaaSPaymentSettlementSyncState{}, saasPaymentConflict("payment settlement sync run is no longer active")
	}
	cursorAfter := strings.TrimSpace(input.CursorAfter)
	if input.DryRun || input.Status == dashboard.SaaSPaymentSettlementSyncStatusPreviewed {
		cursorAfter = run.CursorBefore
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_payment_settlement_sync_runs
		SET status = ?, dry_run = ?, cursor_after = ?, fetched_page_count = ?, fetched_batch_count = ?,
			imported_batch_count = ?, idempotent_batch_count = ?, entry_count = ?, issue_count = ?, open_issue_count = ?,
			finished_at = NOW(), error_message = '', updated_at = NOW()
		WHERE id = ? AND status = 'running'
	`, input.Status, boolTinyInt(input.DryRun), cursorAfter, nonNegativeInt(input.FetchedPageCount), nonNegativeInt(input.FetchedBatchCount),
		nonNegativeInt(input.ImportedBatchCount), nonNegativeInt(input.IdempotentBatchCount), nonNegativeInt(input.EntryCount),
		nonNegativeInt(input.IssueCount), nonNegativeInt(input.OpenIssueCount), run.ID); err != nil {
		return dashboard.SaaSPaymentSettlementSyncRun{}, dashboard.SaaSPaymentSettlementSyncState{}, err
	}
	if input.Status == dashboard.SaaSPaymentSettlementSyncStatusSucceeded && !input.DryRun {
		_, err = tx.ExecContext(ctx, `
			UPDATE mochat_go_saas_payment_settlement_sync_states
			SET `+"`cursor`"+` = ?, active_run_id = NULL, last_success_at = NOW(), last_error = '', version = version + 1, updated_at = NOW()
			WHERE id = ? AND active_run_id = ?
		`, cursorAfter, state.ID, run.ID)
	} else {
		_, err = tx.ExecContext(ctx, `
			UPDATE mochat_go_saas_payment_settlement_sync_states
			SET active_run_id = NULL, last_error = '', version = version + 1, updated_at = NOW()
			WHERE id = ? AND active_run_id = ?
		`, state.ID, run.ID)
	}
	if err != nil {
		return dashboard.SaaSPaymentSettlementSyncRun{}, dashboard.SaaSPaymentSettlementSyncState{}, err
	}
	afterJSON, _ := json.Marshal(map[string]any{
		"runNo": run.RunNo, "provider": provider, "status": input.Status, "dryRun": input.DryRun,
		"cursorBefore": run.CursorBefore, "cursorAfter": cursorAfter,
		"fetchedBatchCount": input.FetchedBatchCount, "importedBatchCount": input.ImportedBatchCount,
		"idempotentBatchCount": input.IdempotentBatchCount, "entryCount": input.EntryCount,
		"issueCount": input.IssueCount, "openIssueCount": input.OpenIssueCount,
	})
	operationID, opErr := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID: input.ActorTenantID, ActorUserID: input.ActorUserID, ActorTenantID: input.ActorTenantID,
		Action: dashboard.SaaSAdminOperationActionSettlementSync, TargetType: dashboard.SaaSAdminOperationTargetSettlementSync,
		TargetID: provider, TargetName: run.RunNo, AfterJSON: string(afterJSON), Remark: "payment settlement sync " + input.Status,
	})
	if opErr != nil && !isMissingSaaSTableError(opErr) {
		return dashboard.SaaSPaymentSettlementSyncRun{}, dashboard.SaaSPaymentSettlementSyncState{}, opErr
	}
	if operationID > 0 {
		if _, err := tx.ExecContext(ctx, `UPDATE mochat_go_saas_payment_settlement_sync_runs SET operation_id = ?, updated_at = NOW() WHERE id = ?`, operationID, run.ID); err != nil {
			return dashboard.SaaSPaymentSettlementSyncRun{}, dashboard.SaaSPaymentSettlementSyncState{}, err
		}
	}
	run, _, err = saasPaymentSettlementSyncRunByIDTx(ctx, tx, run.ID, false)
	if err != nil {
		return dashboard.SaaSPaymentSettlementSyncRun{}, dashboard.SaaSPaymentSettlementSyncState{}, err
	}
	state, _, err = saasPaymentSettlementSyncStateByProviderTx(ctx, tx, provider, false)
	if err != nil {
		return dashboard.SaaSPaymentSettlementSyncRun{}, dashboard.SaaSPaymentSettlementSyncState{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSPaymentSettlementSyncRun{}, dashboard.SaaSPaymentSettlementSyncState{}, err
	}
	return run, state, nil
}

func (s *MySQLStore) FailSaaSPaymentSettlementSync(ctx context.Context, input dashboard.SaaSPaymentSettlementSyncFailure) (dashboard.SaaSPaymentSettlementSyncRun, dashboard.SaaSPaymentSettlementSyncState, error) {
	provider := strings.ToLower(strings.TrimSpace(input.Provider))
	if input.RunID <= 0 || provider == "" {
		return dashboard.SaaSPaymentSettlementSyncRun{}, dashboard.SaaSPaymentSettlementSyncState{}, dashboard.NewSaaSAdminBadRequest("payment settlement sync failure fields invalid")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.SaaSPaymentSettlementSyncRun{}, dashboard.SaaSPaymentSettlementSyncState{}, err
	}
	defer rollbackQuietly(tx)
	state, found, err := saasPaymentSettlementSyncStateByProviderTx(ctx, tx, provider, true)
	if err != nil || !found {
		if err == nil {
			err = dashboard.NewSaaSAdminNotFound("payment settlement sync state not found")
		}
		return dashboard.SaaSPaymentSettlementSyncRun{}, dashboard.SaaSPaymentSettlementSyncState{}, err
	}
	run, found, err := saasPaymentSettlementSyncRunByIDTx(ctx, tx, input.RunID, true)
	if err != nil || !found {
		if err == nil {
			err = dashboard.NewSaaSAdminNotFound("payment settlement sync run not found")
		}
		return dashboard.SaaSPaymentSettlementSyncRun{}, dashboard.SaaSPaymentSettlementSyncState{}, err
	}
	message := truncateRunes(strings.TrimSpace(input.ErrorMessage), 1000)
	if run.Status == dashboard.SaaSPaymentSettlementSyncStatusRunning {
		if _, err := tx.ExecContext(ctx, `
			UPDATE mochat_go_saas_payment_settlement_sync_runs
			SET status = 'failed', fetched_page_count = ?, fetched_batch_count = ?, imported_batch_count = ?,
				idempotent_batch_count = ?, entry_count = ?, issue_count = ?, open_issue_count = ?,
				finished_at = NOW(), error_message = ?, updated_at = NOW()
			WHERE id = ? AND status = 'running'
		`, nonNegativeInt(input.FetchedPageCount), nonNegativeInt(input.FetchedBatchCount), nonNegativeInt(input.ImportedBatchCount),
			nonNegativeInt(input.IdempotentBatchCount), nonNegativeInt(input.EntryCount), nonNegativeInt(input.IssueCount),
			nonNegativeInt(input.OpenIssueCount), message, run.ID); err != nil {
			return dashboard.SaaSPaymentSettlementSyncRun{}, dashboard.SaaSPaymentSettlementSyncState{}, err
		}
	}
	if state.ActiveRunID == run.ID {
		if _, err := tx.ExecContext(ctx, `
			UPDATE mochat_go_saas_payment_settlement_sync_states
			SET active_run_id = NULL, last_error = ?, version = version + 1, updated_at = NOW()
			WHERE id = ? AND active_run_id = ?
		`, message, state.ID, run.ID); err != nil {
			return dashboard.SaaSPaymentSettlementSyncRun{}, dashboard.SaaSPaymentSettlementSyncState{}, err
		}
	}
	afterJSON, _ := json.Marshal(map[string]any{"runNo": run.RunNo, "provider": provider, "status": dashboard.SaaSPaymentSettlementSyncStatusFailed, "error": message})
	operationID, opErr := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID: input.ActorTenantID, ActorUserID: input.ActorUserID, ActorTenantID: input.ActorTenantID,
		Action: dashboard.SaaSAdminOperationActionSettlementSync, TargetType: dashboard.SaaSAdminOperationTargetSettlementSync,
		TargetID: provider, TargetName: run.RunNo, AfterJSON: string(afterJSON), Remark: "payment settlement sync failed",
	})
	if opErr != nil && !isMissingSaaSTableError(opErr) {
		return dashboard.SaaSPaymentSettlementSyncRun{}, dashboard.SaaSPaymentSettlementSyncState{}, opErr
	}
	if operationID > 0 {
		if _, err := tx.ExecContext(ctx, `UPDATE mochat_go_saas_payment_settlement_sync_runs SET operation_id = ?, updated_at = NOW() WHERE id = ?`, operationID, run.ID); err != nil {
			return dashboard.SaaSPaymentSettlementSyncRun{}, dashboard.SaaSPaymentSettlementSyncState{}, err
		}
	}
	run, _, err = saasPaymentSettlementSyncRunByIDTx(ctx, tx, run.ID, false)
	if err != nil {
		return dashboard.SaaSPaymentSettlementSyncRun{}, dashboard.SaaSPaymentSettlementSyncState{}, err
	}
	state, _, err = saasPaymentSettlementSyncStateByProviderTx(ctx, tx, provider, false)
	if err != nil {
		return dashboard.SaaSPaymentSettlementSyncRun{}, dashboard.SaaSPaymentSettlementSyncState{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSPaymentSettlementSyncRun{}, dashboard.SaaSPaymentSettlementSyncState{}, err
	}
	return run, state, nil
}

func (s *MySQLStore) SaaSAdminPaymentSettlementSyncRuns(ctx context.Context, options dashboard.SaaSPaymentSettlementSyncRunOptions) (dashboard.SaaSPaymentSettlementSyncReport, error) {
	stateWhere, stateArgs := "", []any{}
	if strings.TrimSpace(options.Provider) != "" {
		stateWhere = " WHERE provider = ?"
		stateArgs = append(stateArgs, strings.TrimSpace(options.Provider))
	}
	stateRows, err := s.db.QueryContext(ctx, saasPaymentSettlementSyncStateSelectSQL()+stateWhere+" ORDER BY provider ASC", stateArgs...)
	if err != nil {
		return dashboard.SaaSPaymentSettlementSyncReport{}, err
	}
	states := make([]dashboard.SaaSPaymentSettlementSyncState, 0)
	for stateRows.Next() {
		state, scanErr := scanSaaSPaymentSettlementSyncState(stateRows)
		if scanErr != nil {
			stateRows.Close()
			return dashboard.SaaSPaymentSettlementSyncReport{}, scanErr
		}
		states = append(states, state)
	}
	err = stateRows.Err()
	stateRows.Close()
	if err != nil {
		return dashboard.SaaSPaymentSettlementSyncReport{}, err
	}
	where := []string{"1 = 1"}
	args := []any{}
	if strings.TrimSpace(options.Provider) != "" {
		where = append(where, "provider = ?")
		args = append(args, strings.TrimSpace(options.Provider))
	}
	if options.Source != "" && options.Source != "all" {
		where = append(where, "source = ?")
		args = append(args, options.Source)
	}
	if options.Status != "" && options.Status != "all" {
		where = append(where, "status = ?")
		args = append(args, options.Status)
	}
	rows, err := s.db.QueryContext(ctx, saasPaymentSettlementSyncRunSelectSQL()+" WHERE "+strings.Join(where, " AND ")+" ORDER BY started_at DESC, id DESC LIMIT ?", append(args, saasPaymentSettlementSyncQueryMaxLimit)...)
	if err != nil {
		return dashboard.SaaSPaymentSettlementSyncReport{}, err
	}
	defer rows.Close()
	allRuns := make([]dashboard.SaaSPaymentSettlementSyncRun, 0)
	for rows.Next() {
		run, scanErr := scanSaaSPaymentSettlementSyncRun(rows)
		if scanErr != nil {
			return dashboard.SaaSPaymentSettlementSyncReport{}, scanErr
		}
		allRuns = append(allRuns, run)
	}
	if err := rows.Err(); err != nil {
		return dashboard.SaaSPaymentSettlementSyncReport{}, err
	}
	limit := options.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > saasPaymentSettlementSyncQueryMaxLimit {
		limit = saasPaymentSettlementSyncQueryMaxLimit
	}
	runs := allRuns
	if len(runs) > limit {
		runs = runs[:limit]
	}
	summary := dashboard.SaaSPaymentSettlementSyncSummary{ProviderCount: len(states), RunCount: len(allRuns)}
	for _, state := range states {
		if state.ActiveRunID > 0 {
			summary.ActiveCount++
		}
	}
	for _, run := range allRuns {
		switch run.Status {
		case dashboard.SaaSPaymentSettlementSyncStatusSucceeded:
			summary.SucceededCount++
		case dashboard.SaaSPaymentSettlementSyncStatusPreviewed:
			summary.PreviewedCount++
		case dashboard.SaaSPaymentSettlementSyncStatusFailed:
			summary.FailedCount++
		}
		summary.ImportedCount += run.ImportedBatchCount
		summary.IdempotentCount += run.IdempotentBatchCount
		summary.EntryCount += run.EntryCount
		summary.IssueCount += run.IssueCount
		summary.OpenIssueCount += run.OpenIssueCount
	}
	return dashboard.SaaSPaymentSettlementSyncReport{Options: options, Summary: summary, States: states, Runs: runs}, nil
}

func saasPaymentSettlementSyncStateSelectSQL() string {
	return "SELECT id, provider, `cursor`, COALESCE(active_run_id, 0), last_attempt_at, last_success_at, last_error, version, created_at, updated_at FROM mochat_go_saas_payment_settlement_sync_states"
}

func saasPaymentSettlementSyncRunSelectSQL() string {
	return `SELECT id, run_no, provider, source, status, dry_run, cursor_before, cursor_after,
		fetched_page_count, fetched_batch_count, imported_batch_count, idempotent_batch_count,
		entry_count, issue_count, open_issue_count, started_at, finished_at, error_message,
		actor_user_id, actor_tenant_id, operation_id, created_at, updated_at
		FROM mochat_go_saas_payment_settlement_sync_runs`
}

func saasPaymentSettlementSyncStateByProviderTx(ctx context.Context, tx *sql.Tx, provider string, forUpdate bool) (dashboard.SaaSPaymentSettlementSyncState, bool, error) {
	query := saasPaymentSettlementSyncStateSelectSQL() + " WHERE provider = ?"
	if forUpdate {
		query += " FOR UPDATE"
	}
	state, err := scanSaaSPaymentSettlementSyncState(tx.QueryRowContext(ctx, query, provider))
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSPaymentSettlementSyncState{}, false, nil
	}
	return state, err == nil, err
}

func saasPaymentSettlementSyncRunByIDTx(ctx context.Context, tx *sql.Tx, id int64, forUpdate bool) (dashboard.SaaSPaymentSettlementSyncRun, bool, error) {
	query := saasPaymentSettlementSyncRunSelectSQL() + " WHERE id = ?"
	if forUpdate {
		query += " FOR UPDATE"
	}
	run, err := scanSaaSPaymentSettlementSyncRun(tx.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSPaymentSettlementSyncRun{}, false, nil
	}
	return run, err == nil, err
}

type saasPaymentSettlementSyncScanner interface {
	Scan(...any) error
}

func scanSaaSPaymentSettlementSyncState(scanner saasPaymentSettlementSyncScanner) (dashboard.SaaSPaymentSettlementSyncState, error) {
	var item dashboard.SaaSPaymentSettlementSyncState
	var lastAttemptAt, lastSuccessAt, createdAt, updatedAt sql.NullTime
	err := scanner.Scan(&item.ID, &item.Provider, &item.Cursor, &item.ActiveRunID, &lastAttemptAt, &lastSuccessAt, &item.LastError, &item.Version, &createdAt, &updatedAt)
	if err != nil {
		return item, err
	}
	item.LastAttemptAt = formatTime(lastAttemptAt)
	item.LastSuccessAt = formatTime(lastSuccessAt)
	item.CreatedAt = formatTime(createdAt)
	item.UpdatedAt = formatTime(updatedAt)
	return item, nil
}

func scanSaaSPaymentSettlementSyncRun(scanner saasPaymentSettlementSyncScanner) (dashboard.SaaSPaymentSettlementSyncRun, error) {
	var item dashboard.SaaSPaymentSettlementSyncRun
	var dryRun int
	var startedAt, finishedAt, createdAt, updatedAt sql.NullTime
	err := scanner.Scan(
		&item.ID, &item.RunNo, &item.Provider, &item.Source, &item.Status, &dryRun, &item.CursorBefore, &item.CursorAfter,
		&item.FetchedPageCount, &item.FetchedBatchCount, &item.ImportedBatchCount, &item.IdempotentBatchCount,
		&item.EntryCount, &item.IssueCount, &item.OpenIssueCount, &startedAt, &finishedAt, &item.ErrorMessage,
		&item.ActorUserID, &item.ActorTenantID, &item.OperationID, &createdAt, &updatedAt,
	)
	if err != nil {
		return item, err
	}
	item.DryRun = dryRun == 1
	item.StartedAt = formatTime(startedAt)
	item.FinishedAt = formatTime(finishedAt)
	item.CreatedAt = formatTime(createdAt)
	item.UpdatedAt = formatTime(updatedAt)
	return item, nil
}

func nonNegativeInt(value int) int {
	if value < 0 {
		return 0
	}
	return value
}
