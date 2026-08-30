package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"jiyi/mochat-go/internal/dashboard"
	"jiyi/mochat-go/internal/dashboardprincipal"
	"jiyi/mochat-go/internal/modules/providers"
	archiveprovider "jiyi/mochat-go/internal/modules/providers/archive"
)

var errArchiveSyncRunNotFound = errors.New("archive sync run not found")

const archiveSyncLeaseDuration = 5 * time.Minute

type archiveSyncRunScanner interface {
	Scan(dest ...any) error
}

// EnqueueArchiveSync is the durable idempotency boundary for archive runs. The
// unique key is scoped by tenant, corp, source and caller-provided key; no
// request-body or query tenant can widen that scope.
func (s *MySQLStore) EnqueueArchiveSync(ctx context.Context, template archiveprovider.SyncRun, retryFailed bool) (archiveprovider.SyncRun, error) {
	if s == nil || s.db == nil {
		return archiveprovider.SyncRun{}, errors.New("archive sync store unavailable")
	}
	if !archiveSyncTemplateValid(template) {
		return archiveprovider.SyncRun{}, errors.New("archive sync template invalid")
	}
	for attempt := 0; attempt < 3; attempt++ {
		run, err := s.enqueueArchiveSyncOnce(ctx, template, retryFailed)
		if err == nil || !isMySQLRetryableTransactionError(err) || ctx.Err() != nil || attempt == 2 {
			return run, err
		}
		if err := waitForMySQLTransactionRetry(ctx, attempt); err != nil {
			return archiveprovider.SyncRun{}, err
		}
	}
	return archiveprovider.SyncRun{}, errors.New("archive sync enqueue retry exhausted")
}

func (s *MySQLStore) enqueueArchiveSyncOnce(ctx context.Context, template archiveprovider.SyncRun, retryFailed bool) (archiveprovider.SyncRun, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return archiveprovider.SyncRun{}, err
	}
	defer rollbackQuietly(tx)
	var corpExists int
	if err = tx.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM mc_corp
		WHERE tenant_id = ? AND id = ? AND deleted_at IS NULL
	`, template.Scope.TenantID, template.Scope.CorpID).Scan(&corpExists); err != nil {
		return archiveprovider.SyncRun{}, err
	}
	if corpExists != 1 {
		return archiveprovider.SyncRun{}, errors.New("archive sync corp is outside tenant scope")
	}

	existing, err := scanArchiveSyncRun(tx.QueryRowContext(ctx, archiveSyncRunSelect+`
		WHERE tenant_id = ? AND corp_id = ? AND source_kind = ? AND source_id = ? AND idempotency_key = ?
		LIMIT 1 FOR UPDATE`,
		template.Scope.TenantID, template.Scope.CorpID, string(template.Source), template.SourceID, template.IdempotencyKey))
	if err == nil {
		if existing.Namespace != template.Namespace {
			return archiveprovider.SyncRun{}, errors.New("archive sync namespace conflicts with existing identity")
		}
		if existing.Status == archiveprovider.SyncStatusRunning && archiveSyncRunLeaseExpired(existing, time.Now()) {
			existing.Status = archiveprovider.SyncStatusQueued
			existing.ErrorCode = "archive.stale_takeover"
			existing.Attempt++
			existing.StartedAt = nil
			existing.LeaseExpiresAt = nil
			existing.HeartbeatAt = nil
			existing.LeaseToken = ""
			if _, err = tx.ExecContext(ctx, `
				UPDATE mochat_go_archive_sync_runs
				SET status = 'queued', started_at = NULL, lease_expires_at = NULL, heartbeat_at = NULL, lease_token = '',
				    error_code = 'archive.stale_takeover', updated_at = NOW(), attempt = ?
				WHERE id = ? AND status = 'running'
			`, existing.Attempt, existing.ID); err != nil {
				return archiveprovider.SyncRun{}, err
			}
			if err = insertArchiveSyncAuditTx(ctx, tx, existing, "stale_takeover", archiveprovider.SyncStatusQueued, "archive.stale_takeover"); err != nil {
				return archiveprovider.SyncRun{}, err
			}
		} else if existing.Status == archiveprovider.SyncStatusFailed && retryFailed {
			existing.Status = archiveprovider.SyncStatusQueued
			existing.ErrorCode = ""
			existing.Counts = archiveprovider.SyncCounts{}
			existing.FinishedAt = nil
			existing.LeaseToken = ""
			existing.Attempt++
			if _, err = tx.ExecContext(ctx, `
				UPDATE mochat_go_archive_sync_runs
				SET status = 'queued', fetched_count = 0, processed_count = 0, skipped_count = 0,
				    failed_count = 0, error_code = '', finished_at = NULL, lease_expires_at = NULL,
				    heartbeat_at = NULL, lease_token = '', attempt = ?, updated_at = NOW()
				WHERE id = ? AND tenant_id = ? AND corp_id = ?
			`, existing.Attempt, existing.ID, template.Scope.TenantID, template.Scope.CorpID); err != nil {
				return archiveprovider.SyncRun{}, err
			}
			if err = insertArchiveSyncAuditTx(ctx, tx, existing, "retry", archiveprovider.SyncStatusQueued, ""); err != nil {
				return archiveprovider.SyncRun{}, err
			}
		} else if existing.Status == archiveprovider.SyncStatusSucceeded {
			existing.Idempotent = true
		}
		if err = tx.Commit(); err != nil {
			return archiveprovider.SyncRun{}, err
		}
		return existing, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return archiveprovider.SyncRun{}, err
	}

	result, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_archive_sync_runs
		(tenant_id, corp_id, source_kind, source_id, namespace, idempotency_key, status, cursor_sequence, cursor_token, attempt)
		VALUES (?, ?, ?, ?, ?, ?, 'queued', ?, ?, 1)
	`, template.Scope.TenantID, template.Scope.CorpID, string(template.Source), template.SourceID, template.Namespace, template.IdempotencyKey,
		template.Cursor.Sequence, strings.TrimSpace(template.Cursor.Token))
	if err != nil {
		if isArchiveDuplicateError(err) {
			_ = tx.Rollback()
			return s.EnqueueArchiveSync(ctx, template, retryFailed)
		}
		return archiveprovider.SyncRun{}, err
	}
	runID, err := result.LastInsertId()
	if err != nil || runID <= 0 {
		return archiveprovider.SyncRun{}, fmt.Errorf("archive sync run id: %w", err)
	}
	template.ID = strconv.FormatInt(runID, 10)
	template.Attempt = 1
	if err = insertArchiveSyncAuditTx(ctx, tx, template, "enqueue", archiveprovider.SyncStatusQueued, ""); err != nil {
		return archiveprovider.SyncRun{}, err
	}
	queued, err := scanArchiveSyncRun(tx.QueryRowContext(ctx, archiveSyncRunSelect+` WHERE id = ? LIMIT 1 FOR UPDATE`, runID))
	if err != nil {
		return archiveprovider.SyncRun{}, err
	}
	if err = tx.Commit(); err != nil {
		return archiveprovider.SyncRun{}, err
	}
	return queued, nil
}

func (s *MySQLStore) DurableArchiveBindings(ctx context.Context) ([]archiveprovider.DurableArchiveBinding, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("archive sync store unavailable")
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT integration.tenant_id,integration.corp_id,integration.verified_wx_corpid,binding.wecom_integration_mode
		FROM mochat_go_wecom_integrations integration
		INNER JOIN mc_tenant tenant ON tenant.id=integration.tenant_id AND tenant.status=1 AND tenant.deleted_at IS NULL
		INNER JOIN mc_corp corp ON corp.tenant_id=integration.tenant_id AND corp.id=integration.corp_id AND corp.deleted_at IS NULL
		INNER JOIN mochat_go_tenant_corp_bindings binding ON binding.tenant_id=integration.tenant_id AND binding.corp_id=integration.corp_id
		WHERE `+durableArchiveEligibilityPredicate+`
		ORDER BY integration.tenant_id,integration.corp_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]archiveprovider.DurableArchiveBinding, 0)
	for rows.Next() {
		var item archiveprovider.DurableArchiveBinding
		if err := rows.Scan(&item.Scope.TenantID, &item.Scope.CorpID, &item.WXCorpID, &item.IntegrationMode); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *MySQLStore) DurableArchiveBindingForScope(ctx context.Context, scope archiveprovider.Scope) (archiveprovider.DurableArchiveBinding, bool, error) {
	if s == nil || s.db == nil || !scopeValid(scope) {
		return archiveprovider.DurableArchiveBinding{}, false, errors.New("archive sync scope invalid")
	}
	var binding archiveprovider.DurableArchiveBinding
	err := s.db.QueryRowContext(ctx, `
		SELECT integration.tenant_id,integration.corp_id,integration.verified_wx_corpid,binding.wecom_integration_mode
		FROM mochat_go_wecom_integrations integration
		INNER JOIN mc_tenant tenant ON tenant.id=integration.tenant_id AND tenant.status=1 AND tenant.deleted_at IS NULL
		INNER JOIN mc_corp corp ON corp.tenant_id=integration.tenant_id AND corp.id=integration.corp_id AND corp.deleted_at IS NULL
		INNER JOIN mochat_go_tenant_corp_bindings binding ON binding.tenant_id=integration.tenant_id AND binding.corp_id=integration.corp_id
		WHERE `+durableArchiveEligibilityPredicate+`
		  AND integration.tenant_id=? AND integration.corp_id=?
		LIMIT 1
	`, scope.TenantID, scope.CorpID).Scan(&binding.Scope.TenantID, &binding.Scope.CorpID, &binding.WXCorpID, &binding.IntegrationMode)
	if errors.Is(err, sql.ErrNoRows) {
		return archiveprovider.DurableArchiveBinding{}, false, nil
	}
	if err != nil {
		return archiveprovider.DurableArchiveBinding{}, false, err
	}
	return binding, true, nil
}

func (s *MySQLStore) PendingDurableArchiveRuns(ctx context.Context, limit int) ([]archiveprovider.DurableArchivePendingRun, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("archive sync store unavailable")
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT run.id,run.tenant_id,run.corp_id,integration.verified_wx_corpid,binding.wecom_integration_mode,
		       run.cursor_sequence,run.cursor_token,run.idempotency_key
		FROM mochat_go_archive_sync_runs run
		INNER JOIN mochat_go_wecom_integrations integration
		  ON integration.tenant_id=run.tenant_id AND integration.corp_id=run.corp_id
		INNER JOIN mc_tenant tenant ON tenant.id=integration.tenant_id AND tenant.status=1 AND tenant.deleted_at IS NULL
		INNER JOIN mc_corp corp ON corp.tenant_id=integration.tenant_id AND corp.id=integration.corp_id AND corp.deleted_at IS NULL
		INNER JOIN mochat_go_tenant_corp_bindings binding ON binding.tenant_id=integration.tenant_id AND binding.corp_id=integration.corp_id
		WHERE `+durableArchiveEligibilityPredicate+`
		  AND run.source_kind='external'
		  AND run.source_id=CONCAT('wecom:',binding.wecom_integration_mode,':',integration.verified_wx_corpid)
		  AND run.namespace=run.source_id
		  AND (run.status='queued' OR (run.status='running' AND run.lease_expires_at IS NOT NULL AND run.lease_expires_at<=NOW()))
		ORDER BY run.updated_at,run.id
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]archiveprovider.DurableArchivePendingRun, 0)
	for rows.Next() {
		var item archiveprovider.DurableArchivePendingRun
		if err := rows.Scan(
			&item.RunID, &item.Binding.Scope.TenantID, &item.Binding.Scope.CorpID, &item.Binding.WXCorpID, &item.Binding.IntegrationMode,
			&item.Cursor.Sequence, &item.Cursor.Token, &item.IdempotencyKey,
		); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

// BusyDurableArchiveScopes returns eligible scopes that already have queued or
// running work. Unlike PendingDurableArchiveRuns, it intentionally includes
// running work with an active lease so the scheduled enqueuer cannot stack a
// second cursor window behind an in-flight run.
func (s *MySQLStore) BusyDurableArchiveScopes(ctx context.Context) ([]archiveprovider.Scope, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("archive sync store unavailable")
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT run.tenant_id,run.corp_id
		FROM mochat_go_archive_sync_runs run
		INNER JOIN mochat_go_wecom_integrations integration
		  ON integration.tenant_id=run.tenant_id AND integration.corp_id=run.corp_id
		INNER JOIN mc_tenant tenant ON tenant.id=integration.tenant_id AND tenant.status=1 AND tenant.deleted_at IS NULL
		INNER JOIN mc_corp corp ON corp.tenant_id=integration.tenant_id AND corp.id=integration.corp_id AND corp.deleted_at IS NULL
		INNER JOIN mochat_go_tenant_corp_bindings binding ON binding.tenant_id=integration.tenant_id AND binding.corp_id=integration.corp_id
		WHERE `+durableArchiveEligibilityPredicate+`
		  AND run.source_kind='external'
		  AND run.source_id=CONCAT('wecom:',binding.wecom_integration_mode,':',integration.verified_wx_corpid)
		  AND run.namespace=run.source_id
		  AND run.status IN ('queued','running')
		ORDER BY run.tenant_id,run.corp_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]archiveprovider.Scope, 0)
	for rows.Next() {
		var scope archiveprovider.Scope
		if err := rows.Scan(&scope.TenantID, &scope.CorpID); err != nil {
			return nil, err
		}
		result = append(result, scope)
	}
	return result, rows.Err()
}

// durableArchiveEligibilityPredicate is shared by source discovery and media
// claiming so revoked capability or verified-corp drift fails closed at both
// production boundaries.
const durableArchiveEligibilityPredicate = `
		integration.slot='current' AND integration.status='active' AND integration.verified_wx_corpid<>''
		  AND integration.mode=binding.wecom_integration_mode
		  AND integration.verified_at IS NOT NULL
		  AND binding.status=2 AND binding.verified_at IS NOT NULL
		  AND binding.verified_wx_corpid=integration.verified_wx_corpid
		  AND JSON_VALID(integration.scope_json)=1
		  AND JSON_CONTAINS(integration.scope_json, JSON_QUOTE('archive.read'))=1
		  AND JSON_VALID(integration.missing_capabilities_json)=1
		  AND JSON_LENGTH(integration.missing_capabilities_json) = 0`

func (s *MySQLStore) LatestArchiveSyncCursor(ctx context.Context, scope archiveprovider.Scope, sourceID string) (archiveprovider.Cursor, error) {
	if s == nil || s.db == nil || !scopeValid(scope) || strings.TrimSpace(sourceID) == "" {
		return archiveprovider.Cursor{}, errors.New("archive cursor scope invalid")
	}
	var cursor archiveprovider.Cursor
	err := s.db.QueryRowContext(ctx, `
		SELECT cursor_sequence,cursor_token
		FROM mochat_go_archive_sync_runs
		WHERE tenant_id=? AND corp_id=? AND source_kind='external' AND source_id=?
		ORDER BY cursor_sequence DESC,updated_at DESC,id DESC
		LIMIT 1
	`, scope.TenantID, scope.CorpID, strings.TrimSpace(sourceID)).Scan(&cursor.Sequence, &cursor.Token)
	if errors.Is(err, sql.ErrNoRows) {
		return archiveprovider.Cursor{}, nil
	}
	return cursor, err
}

func (s *MySQLStore) MarkArchiveSyncRunning(ctx context.Context, runID string, at time.Time) (archiveprovider.SyncRun, error) {
	return s.transitionArchiveSync(ctx, runID, archiveprovider.SyncStatusRunning, "start", "", archiveprovider.SyncCounts{}, archiveprovider.Cursor{}, at)
}

func (s *MySQLStore) HeartbeatArchiveSync(ctx context.Context, runID string, attempt int, leaseToken string, at time.Time) error {
	if s == nil || s.db == nil {
		return errors.New("archive sync store unavailable")
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE mochat_go_archive_sync_runs
		SET heartbeat_at = ?, lease_expires_at = ?, updated_at = ?
		WHERE id = ? AND status = 'running' AND attempt = ? AND lease_token = ?
	`, at, at.Add(archiveSyncLeaseDuration), at, runID, attempt, strings.TrimSpace(leaseToken))
	if err != nil {
		return err
	}
	return requireArchiveSyncRows(result)
}

func (s *MySQLStore) UpsertArchiveMessage(ctx context.Context, runID string, attempt int, leaseToken string, scope archiveprovider.Scope, message archiveprovider.Message) (archiveprovider.UpsertResult, error) {
	if s == nil || s.db == nil {
		return archiveprovider.UpsertResult{}, errors.New("archive sync store unavailable")
	}
	if !scopeValid(scope) || strings.TrimSpace(runID) == "" || !archiveMessageIdentityValid(message) {
		return archiveprovider.UpsertResult{}, errors.New("archive message scope or identity invalid")
	}
	runIDValue, err := strconv.ParseInt(strings.TrimSpace(runID), 10, 64)
	if err != nil || runIDValue <= 0 {
		return archiveprovider.UpsertResult{}, errors.New("archive sync run id invalid")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return archiveprovider.UpsertResult{}, err
	}
	defer rollbackQuietly(tx)
	var runTenantID, runCorpID int64
	var runSourceKind, runSourceID, runNamespace, runStatus, runLeaseToken string
	var runAttempt int
	err = tx.QueryRowContext(ctx, `
		SELECT tenant_id, corp_id, source_kind, source_id, namespace, status, attempt, lease_token
		FROM mochat_go_archive_sync_runs
		WHERE id = ?
		LIMIT 1 FOR UPDATE
	`, runIDValue).Scan(&runTenantID, &runCorpID, &runSourceKind, &runSourceID, &runNamespace, &runStatus, &runAttempt, &runLeaseToken)
	if errors.Is(err, sql.ErrNoRows) {
		return archiveprovider.UpsertResult{}, errors.New("archive sync run not found")
	}
	if err != nil {
		return archiveprovider.UpsertResult{}, err
	}
	if runTenantID != scope.TenantID || runCorpID != scope.CorpID ||
		runSourceKind != string(message.Source) || runSourceID != strings.TrimSpace(message.SourceID) ||
		runNamespace != strings.TrimSpace(message.Namespace) {
		return archiveprovider.UpsertResult{}, errors.New("archive sync run scope or source mismatch")
	}
	if runStatus != string(archiveprovider.SyncStatusRunning) || runAttempt != attempt || runLeaseToken == "" || runLeaseToken != strings.TrimSpace(leaseToken) {
		return archiveprovider.UpsertResult{}, errors.New("archive sync lease fence rejected")
	}
	if err := claimArchiveMessageSourceTx(ctx, tx, scope, message, runIDValue); err != nil {
		return archiveprovider.UpsertResult{}, err
	}
	var corpID int
	err = tx.QueryRowContext(ctx, `
		SELECT id FROM mc_corp
		WHERE id = ? AND tenant_id = ? AND deleted_at IS NULL
		LIMIT 1
	`, scope.CorpID, scope.TenantID).Scan(&corpID)
	if err == sql.ErrNoRows {
		return archiveprovider.UpsertResult{}, errors.New("archive message corp is outside tenant scope")
	}
	if err != nil {
		return archiveprovider.UpsertResult{}, err
	}
	messageResult, err := s.upsertWorkMessageArchiveWithExecutor(ctx, tx, corpID, dashboard.WorkMessageArchiveMessage{
		Seq: message.Seq, MsgID: message.MsgID, Action: message.Action, From: message.From,
		ToList: message.ToList, RoomID: message.RoomID, MsgType: message.MsgType, MsgTime: message.MsgTime,
		ContentRaw: message.ContentRaw, ContentText: message.ContentText, RawJSON: message.RawJSON,
	})
	if err != nil {
		return archiveprovider.UpsertResult{}, err
	}
	if !messageResult.Resolved {
		return archiveprovider.UpsertResult{}, errors.New("archive message participants unresolved")
	}
	if err = upsertArchiveMediaTx(ctx, tx, s.weComCredentialCipher, scope, message); err != nil {
		return archiveprovider.UpsertResult{}, err
	}
	if err = upsertArchiveComponentTx(ctx, tx, s.weComCredentialCipher, scope, message); err != nil {
		return archiveprovider.UpsertResult{}, err
	}
	if err = tx.Commit(); err != nil {
		return archiveprovider.UpsertResult{}, err
	}
	return archiveprovider.UpsertResult{Inserted: messageResult.Inserted, Skipped: !messageResult.Inserted}, nil
}

func claimArchiveMessageSourceTx(ctx context.Context, tx *sql.Tx, scope archiveprovider.Scope, message archiveprovider.Message, runID int64) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_archive_message_sources
		(tenant_id, corp_id, msgid, source_kind, source_id, namespace, run_id)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE id = LAST_INSERT_ID(id)
	`, scope.TenantID, scope.CorpID, message.MsgID, string(message.Source), strings.TrimSpace(message.SourceID), strings.TrimSpace(message.Namespace), runID)
	if err != nil {
		return err
	}

	var existingKind, existingID, existingNamespace string
	err = tx.QueryRowContext(ctx, `
		SELECT source_kind, source_id, namespace
		FROM mochat_go_archive_message_sources
		WHERE tenant_id = ? AND corp_id = ? AND msgid = ?
		LIMIT 1 FOR UPDATE
	`, scope.TenantID, scope.CorpID, message.MsgID).Scan(&existingKind, &existingID, &existingNamespace)
	if err == nil {
		if existingKind != string(message.Source) || existingID != strings.TrimSpace(message.SourceID) || existingNamespace != strings.TrimSpace(message.Namespace) {
			return errors.New("archive message source identity conflict")
		}
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	return errors.New("archive message source claim disappeared")
}

func (s *MySQLStore) SaveArchiveSyncCursor(ctx context.Context, runID string, attempt int, leaseToken string, cursor archiveprovider.Cursor, at time.Time) error {
	if s == nil || s.db == nil {
		return errors.New("archive sync store unavailable")
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE mochat_go_archive_sync_runs
		SET cursor_sequence = ?, cursor_token = ?, updated_at = ?
		WHERE id = ? AND status = 'running' AND attempt = ? AND lease_token = ?
	`, cursor.Sequence, strings.TrimSpace(cursor.Token), at, runID, attempt, strings.TrimSpace(leaseToken))
	if err != nil {
		return err
	}
	return requireArchiveSyncRows(result)
}

func (s *MySQLStore) CompleteArchiveSync(ctx context.Context, runID string, attempt int, leaseToken string, counts archiveprovider.SyncCounts, cursor archiveprovider.Cursor, at time.Time) (archiveprovider.SyncRun, error) {
	return s.finishArchiveSync(ctx, runID, attempt, leaseToken, archiveprovider.SyncStatusSucceeded, counts, cursor, "", "complete", at)
}

func (s *MySQLStore) FailArchiveSync(ctx context.Context, runID string, attempt int, leaseToken string, counts archiveprovider.SyncCounts, cursor archiveprovider.Cursor, code string, at time.Time) (archiveprovider.SyncRun, error) {
	return s.finishArchiveSync(ctx, runID, attempt, leaseToken, archiveprovider.SyncStatusFailed, counts, cursor, code, "fail", at)
}

func (s *MySQLStore) finishArchiveSync(ctx context.Context, runID string, attempt int, leaseToken string, status archiveprovider.SyncStatus, counts archiveprovider.SyncCounts, cursor archiveprovider.Cursor, code, action string, at time.Time) (archiveprovider.SyncRun, error) {
	if s == nil || s.db == nil {
		return archiveprovider.SyncRun{}, errors.New("archive sync store unavailable")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return archiveprovider.SyncRun{}, err
	}
	defer rollbackQuietly(tx)
	result, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_archive_sync_runs
		SET status = ?, cursor_sequence = ?, cursor_token = ?, fetched_count = ?, processed_count = ?,
		    skipped_count = ?, failed_count = ?, error_code = ?, finished_at = ?, heartbeat_at = ?,
		    lease_expires_at = NULL, lease_token = '', updated_at = ?
		WHERE id = ? AND status = 'running' AND attempt = ? AND lease_token = ?
	`, string(status), cursor.Sequence, strings.TrimSpace(cursor.Token), counts.Fetched, counts.Processed, counts.Skipped, counts.Failed, strings.TrimSpace(code), at, at, at, runID, attempt, strings.TrimSpace(leaseToken))
	if err != nil {
		return archiveprovider.SyncRun{}, err
	}
	if err = requireArchiveSyncRows(result); err != nil {
		return archiveprovider.SyncRun{}, err
	}
	run, err := scanArchiveSyncRun(tx.QueryRowContext(ctx, archiveSyncRunSelect+` WHERE id = ? LIMIT 1 FOR UPDATE`, runID))
	if err != nil {
		return archiveprovider.SyncRun{}, err
	}
	if err = insertArchiveSyncAuditTx(ctx, tx, run, action, status, code); err != nil {
		return archiveprovider.SyncRun{}, err
	}
	if err = tx.Commit(); err != nil {
		return archiveprovider.SyncRun{}, err
	}
	return run, nil
}

func (s *MySQLStore) transitionArchiveSync(ctx context.Context, runID string, status archiveprovider.SyncStatus, action, code string, counts archiveprovider.SyncCounts, cursor archiveprovider.Cursor, at time.Time) (archiveprovider.SyncRun, error) {
	if s == nil || s.db == nil {
		return archiveprovider.SyncRun{}, errors.New("archive sync store unavailable")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return archiveprovider.SyncRun{}, err
	}
	defer rollbackQuietly(tx)
	leaseToken, err := newArchiveLeaseToken()
	if err != nil {
		return archiveprovider.SyncRun{}, err
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_archive_sync_runs
		SET status = ?, started_at = ?, heartbeat_at = ?, lease_expires_at = ?, lease_token = ?, updated_at = ?
		WHERE id = ? AND status = 'queued'
	`, string(status), at, at, at.Add(archiveSyncLeaseDuration), leaseToken, at, runID)
	if err != nil {
		return archiveprovider.SyncRun{}, err
	}
	if err = requireArchiveSyncRows(result); err != nil {
		return archiveprovider.SyncRun{}, err
	}
	run, err := scanArchiveSyncRun(tx.QueryRowContext(ctx, archiveSyncRunSelect+` WHERE id = ? LIMIT 1 FOR UPDATE`, runID))
	if err != nil {
		return archiveprovider.SyncRun{}, err
	}
	if err = insertArchiveSyncAuditTx(ctx, tx, run, action, status, code); err != nil {
		return archiveprovider.SyncRun{}, err
	}
	if err := tx.Commit(); err != nil {
		return archiveprovider.SyncRun{}, err
	}
	return run, nil
}

const archiveSyncRunSelect = `
	SELECT id, tenant_id, corp_id, source_kind, source_id, namespace, idempotency_key,
	       status, cursor_sequence, cursor_token, fetched_count, processed_count, skipped_count,
	       failed_count, error_code, attempt, lease_token, started_at, finished_at, lease_expires_at, heartbeat_at
	FROM mochat_go_archive_sync_runs`

func scanArchiveSyncRun(scanner archiveSyncRunScanner) (archiveprovider.SyncRun, error) {
	var id, tenantID, corpID int64
	var sourceKind, sourceID, namespace, key, status, token, errorCode, leaseToken string
	var sequence int64
	var fetched, processed, skipped, failed, attempt int
	var startedAt, finishedAt, leaseExpiresAt, heartbeatAt sql.NullTime
	if err := scanner.Scan(&id, &tenantID, &corpID, &sourceKind, &sourceID, &namespace, &key, &status, &sequence, &token,
		&fetched, &processed, &skipped, &failed, &errorCode, &attempt, &leaseToken, &startedAt, &finishedAt, &leaseExpiresAt, &heartbeatAt); err != nil {
		return archiveprovider.SyncRun{}, err
	}
	source := providers.Source(sourceKind)
	if id <= 0 || tenantID <= 0 || corpID <= 0 || !scopeValid(archiveprovider.Scope{TenantID: tenantID, CorpID: corpID}) {
		return archiveprovider.SyncRun{}, errors.New("archive sync run scope invalid")
	}
	return archiveprovider.SyncRun{
		ID: strconv.FormatInt(id, 10), Scope: archiveprovider.Scope{TenantID: tenantID, CorpID: corpID},
		Source: source, SourceID: sourceID, Namespace: namespace, IdempotencyKey: key,
		Status: archiveprovider.SyncStatus(status), Cursor: archiveprovider.Cursor{Sequence: sequence, Token: token},
		Counts:    archiveprovider.SyncCounts{Fetched: fetched, Processed: processed, Skipped: skipped, Failed: failed},
		ErrorCode: errorCode, Attempt: attempt, LeaseToken: leaseToken, StartedAt: nullableArchiveTime(startedAt), FinishedAt: nullableArchiveTime(finishedAt),
		LeaseExpiresAt: nullableArchiveTime(leaseExpiresAt), HeartbeatAt: nullableArchiveTime(heartbeatAt),
	}, nil
}

func insertArchiveSyncAuditTx(ctx context.Context, tx *sql.Tx, run archiveprovider.SyncRun, action string, status archiveprovider.SyncStatus, code string) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_archive_sync_audits
		(run_id, tenant_id, corp_id, source_kind, source_id, namespace, action, status, error_code,
		 cursor_sequence, fetched_count, processed_count, skipped_count, failed_count)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, runIDInt(run.ID), run.Scope.TenantID, run.Scope.CorpID, string(run.Source), run.SourceID, run.Namespace,
		action, string(status), strings.TrimSpace(code), run.Cursor.Sequence, run.Counts.Fetched, run.Counts.Processed, run.Counts.Skipped, run.Counts.Failed)
	return err
}

func runIDInt(value string) int64 {
	runID, _ := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	return runID
}

func nullableArchiveTime(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time
	return &result
}

func archiveSyncRunLeaseExpired(run archiveprovider.SyncRun, now time.Time) bool {
	return run.Status == archiveprovider.SyncStatusRunning && run.LeaseExpiresAt != nil && !run.LeaseExpiresAt.After(now)
}

func newArchiveLeaseToken() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

func isArchiveDuplicateError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "duplicate entry") || strings.Contains(message, "duplicate key") || strings.Contains(message, "1062")
}

func requireArchiveSyncRows(result sql.Result) error {
	if result == nil {
		return errors.New("archive sync mutation returned no result")
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		return errArchiveSyncRunNotFound
	}
	return nil
}

func archiveSyncTemplateValid(run archiveprovider.SyncRun) bool {
	return scopeValid(run.Scope) && (run.Source == providers.SourceExternal || run.Source == providers.SourceSimulated) &&
		strings.TrimSpace(run.SourceID) != "" && strings.TrimSpace(run.Namespace) != "" && strings.TrimSpace(run.IdempotencyKey) != "" && run.Cursor.Sequence >= 0
}

func scopeValid(scope archiveprovider.Scope) bool {
	return scope.TenantID > 0 && scope.CorpID > 0
}

func archiveMessageIdentityValid(message archiveprovider.Message) bool {
	return (message.Source == providers.SourceExternal || message.Source == providers.SourceSimulated) &&
		strings.TrimSpace(message.SourceID) != "" && strings.TrimSpace(message.Namespace) != "" &&
		strings.TrimSpace(message.MsgID) != "" && message.Seq > 0
}

func archiveStatusSourceKind(mode workMessageArchiveMode) providers.Source {
	if mode == workMessageArchiveSimulation {
		return providers.SourceSimulated
	}
	return providers.SourceExternal
}

func (s *MySQLStore) archiveStatusMode(ctx context.Context, tenantID, corpID int) (workMessageArchiveMode, error) {
	var chatStatus int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(chat_status, 0)
		FROM mc_corp
		WHERE id = ? AND tenant_id = ? AND deleted_at IS NULL
		LIMIT 1
	`, corpID, tenantID).Scan(&chatStatus); err != nil {
		if err == sql.ErrNoRows {
			return workMessageArchiveUnavailable, nil
		}
		return workMessageArchiveUnavailable, err
	}
	if chatStatus == 1 {
		return workMessageArchiveReal, nil
	}
	var simulationAvailable bool
	err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM mochat_go_archive_simulation_batches
			WHERE corp_id = ? AND status = 'complete' AND message_count > 0
		)
	`, corpID).Scan(&simulationAvailable)
	if err != nil {
		if isMissingArchiveSourceTable(err) {
			return workMessageArchiveUnavailable, nil
		}
		return workMessageArchiveUnavailable, err
	}
	if simulationAvailable {
		return workMessageArchiveSimulation, nil
	}
	return workMessageArchiveUnavailable, nil
}

// GetArchiveSourceStatus reads only tenant/corp-scoped source metadata. It
// deliberately falls back to the pre-0138 simulation registry when the new
// migration is not installed, so an application rollout can remain truthful.
func (s *MySQLStore) GetArchiveSourceStatus(ctx context.Context, principal dashboardprincipal.DashboardPrincipal) (providers.Status, error) {
	if s == nil || s.db == nil {
		return providers.Status{}, errors.New("archive source status store unavailable")
	}
	mode, err := s.archiveStatusMode(ctx, principal.TenantID, principal.CorpID)
	if err != nil {
		return providers.Status{}, err
	}
	if mode == workMessageArchiveUnavailable {
		return providers.Status{}, nil
	}
	sourceFilterKind := archiveStatusSourceKind(mode)
	var sourceKind, sourceID, namespace, status, errorCode string
	var lastSync, lastSuccess, lastFailure sql.NullTime
	err = s.db.QueryRowContext(ctx, `
		SELECT latest.source_kind, latest.source_id, latest.namespace, latest.status, latest.error_code, latest.updated_at,
		       (SELECT finished_at FROM mochat_go_archive_sync_runs success
		        WHERE success.tenant_id = latest.tenant_id AND success.corp_id = latest.corp_id
		          AND success.source_kind = latest.source_kind AND success.source_id = latest.source_id
		          AND success.status = 'succeeded' AND success.finished_at IS NOT NULL
		        ORDER BY success.finished_at DESC, success.id DESC LIMIT 1),
		       (SELECT finished_at FROM mochat_go_archive_sync_runs failed
		        WHERE failed.tenant_id = latest.tenant_id AND failed.corp_id = latest.corp_id
		          AND failed.source_kind = latest.source_kind AND failed.source_id = latest.source_id
		          AND failed.status = 'failed' AND failed.finished_at IS NOT NULL
		        ORDER BY failed.finished_at DESC, failed.id DESC LIMIT 1)
		FROM mochat_go_archive_sync_runs latest
		WHERE latest.tenant_id = ? AND latest.corp_id = ? AND latest.source_kind = ?
		ORDER BY latest.updated_at DESC, latest.id DESC
		LIMIT 1
	`, principal.TenantID, principal.CorpID, sourceFilterKind).Scan(&sourceKind, &sourceID, &namespace, &status, &errorCode, &lastSync, &lastSuccess, &lastFailure)
	if err == nil {
		return archiveSourceStatusFromRun(sourceKind, status, errorCode, lastSync, lastSuccess, lastFailure), nil
	}
	if err != sql.ErrNoRows && !isMissingArchiveSourceTable(err) {
		return providers.Status{}, err
	}
	if mode != workMessageArchiveSimulation {
		return providers.Status{}, nil
	}
	var simulationStatus string
	err = s.db.QueryRowContext(ctx, `
		SELECT status FROM mochat_go_archive_simulation_batches batch
		INNER JOIN mc_corp corp ON corp.id = batch.corp_id AND corp.tenant_id = ? AND corp.deleted_at IS NULL
		WHERE batch.corp_id = ? AND batch.status = 'complete'
		ORDER BY batch.id DESC LIMIT 1
	`, principal.TenantID, principal.CorpID).Scan(&simulationStatus)
	if err == nil && strings.EqualFold(simulationStatus, "complete") {
		return providers.Status{Kind: "wecom_archive", State: providers.StateLimited, Source: providers.SourceSimulated, Code: "archive.simulation_ready", Reason: "simulation archive data", Action: "inspect simulation source"}, nil
	}
	if err != nil && err != sql.ErrNoRows && !isMissingArchiveSourceTable(err) {
		return providers.Status{}, err
	}
	return providers.Status{}, nil
}

func archiveSourceStatusFromRun(sourceKind, runStatus, errorCode string, lastSync, lastSuccess, lastFailure sql.NullTime) providers.Status {
	source := providers.Source(sourceKind)
	if source != providers.SourceSimulated && source != providers.SourceExternal {
		source = providers.SourceExternal
	}
	status := providers.Status{
		Kind: "wecom_archive", Source: source, State: providers.StateLimited,
		Capabilities: []string{"archive_sync"}, LastSyncAt: nullableArchiveTime(lastSync),
		LastSuccessAt: nullableArchiveTime(lastSuccess), LastFailureAt: nullableArchiveTime(lastFailure),
	}
	if source == providers.SourceExternal {
		switch strings.ToLower(strings.TrimSpace(runStatus)) {
		case string(archiveprovider.SyncStatusSucceeded):
			status.State = providers.StateReady
			status.Code = "archive.bridge_ready"
			status.Reason = "archive bridge synchronization succeeded"
			status.Action = "continue scheduled or manual synchronization"
		case string(archiveprovider.SyncStatusQueued):
			status.Code = "archive.bridge_pending"
			status.Reason = "archive bridge synchronization is queued"
			status.Action = "wait for synchronization to start"
		case string(archiveprovider.SyncStatusRunning):
			status.Code = "archive.bridge_syncing"
			status.Reason = "archive bridge synchronization is running"
			status.Action = "wait for synchronization to finish"
		case string(archiveprovider.SyncStatusFailed):
			status.State = providers.StateUnavailable
			status.Code = "archive.bridge_failed"
			status.Reason = "archive bridge synchronization failed"
			status.Action = "check the enterprise binding and retry"
			status.LastErrorCode = stableArchiveErrorCode(errorCode)
		default:
			status.Code = "archive.bridge_pending"
			status.Reason = "archive bridge synchronization has not completed"
			status.Action = "start a manual synchronization"
		}
	} else {
		switch strings.ToLower(strings.TrimSpace(runStatus)) {
		case string(archiveprovider.SyncStatusSucceeded):
			status.Code = "archive.simulation_ready"
			status.Reason = "the isolated simulation archive run succeeded"
			status.Action = "use this data for acceptance only"
		case string(archiveprovider.SyncStatusQueued):
			status.Code = "archive.simulation_pending"
			status.Reason = "the simulation archive run is queued"
			status.Action = "wait for the simulation run to finish"
		case string(archiveprovider.SyncStatusRunning):
			status.Code = "archive.simulation_syncing"
			status.Reason = "the simulation archive run is syncing"
			status.Action = "wait for the simulation run to finish"
		case string(archiveprovider.SyncStatusFailed):
			status.Code = "archive.simulation_failed"
			status.Reason = "the simulation archive run failed"
			status.Action = "repair the simulation source and retry"
			status.LastErrorCode = stableArchiveErrorCode(errorCode)
		default:
			status.Code = "archive.simulation_pending"
			status.Reason = "the simulation archive run state is unconfirmed"
			status.Action = "confirm the simulation run state before continuing"
		}
	}
	if strings.EqualFold(strings.TrimSpace(runStatus), string(archiveprovider.SyncStatusFailed)) && status.LastErrorCode == "" {
		status.LastErrorCode = stableArchiveErrorCode(errorCode)
	}
	return status
}

func stableArchiveErrorCode(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "archive.sync_failed"
	}
	for _, character := range value {
		if !((character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') || character == '.' || character == '_' || character == '-') {
			return "archive.sync_failed"
		}
	}
	return value
}

func isMissingArchiveSourceTable(err error) bool {
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(message, "mochat_go_archive_") && (strings.Contains(message, "doesn't exist") || strings.Contains(message, "does not exist"))
}
