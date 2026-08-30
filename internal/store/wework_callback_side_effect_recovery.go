package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"

	"jiyi/mochat-go/internal/companyprofile"
	"jiyi/mochat-go/internal/dashboardprincipal"
)

type callbackSideEffectCursor struct {
	UnknownAt string `json:"unknownAt"`
	EventKey  string `json:"eventKey"`
	ActionKey string `json:"actionKey"`
}

type callbackSideEffectLocked struct {
	ActionKey      string
	PayloadHash    string
	Status         string
	Version        uint64
	ReconcileAfter sql.NullTime
}

func (s *MySQLStore) ListCallbackSideEffects(ctx context.Context, principal dashboardprincipal.DashboardPrincipal, input companyprofile.CallbackSideEffectListInput) (companyprofile.CallbackSideEffectPage, error) {
	if s == nil || s.db == nil {
		return companyprofile.CallbackSideEffectPage{}, companyprofile.ErrStoreUnavailable
	}
	if err := s.checkCompanyActor(ctx, s.db, principal, false); err != nil {
		return companyprofile.CallbackSideEffectPage{}, err
	}
	limit := input.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	query := `SELECT se.event_key,se.action_key,se.payload_hash,se.status,se.version,se.unknown_at,se.reconcile_after,
		inbox.status,inbox.attempt,inbox.last_error
		FROM mochat_go_wework_callback_side_effects se
		INNER JOIN mochat_go_wework_callback_inbox inbox
		  ON inbox.tenant_id=se.tenant_id AND inbox.corp_id=se.corp_id AND inbox.event_key=se.event_key
		WHERE se.tenant_id=? AND se.corp_id=? AND se.status='unknown'`
	args := []any{principal.TenantID, principal.CorpID}
	if strings.TrimSpace(input.Cursor) != "" {
		cursor, err := decodeCallbackSideEffectCursor(input.Cursor)
		if err != nil {
			return companyprofile.CallbackSideEffectPage{}, companyprofile.ErrInvalidRequest
		}
		cursorTime, err := time.Parse(time.RFC3339Nano, cursor.UnknownAt)
		if err != nil {
			return companyprofile.CallbackSideEffectPage{}, companyprofile.ErrInvalidRequest
		}
		query += ` AND (se.unknown_at<? OR (se.unknown_at=? AND se.event_key<?) OR (se.unknown_at=? AND se.event_key=? AND se.action_key<?))`
		args = append(args, cursorTime, cursorTime, cursor.EventKey, cursorTime, cursor.EventKey, cursor.ActionKey)
	}
	query += ` ORDER BY se.unknown_at DESC,se.event_key DESC,se.action_key DESC LIMIT ?`
	args = append(args, limit+1)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return companyprofile.CallbackSideEffectPage{}, err
	}
	defer rows.Close()
	items := make([]companyprofile.CallbackSideEffectSummary, 0, limit+1)
	for rows.Next() {
		var item companyprofile.CallbackSideEffectSummary
		var unknownAt, reconcileAfter sql.NullTime
		var lastError string
		if err := rows.Scan(&item.EventKey, &item.ActionKey, &item.PayloadHash, &item.Status, &item.Version, &unknownAt, &reconcileAfter, &item.InboxStatus, &item.Attempt, &lastError); err != nil {
			return companyprofile.CallbackSideEffectPage{}, err
		}
		item.UnknownAt = nullableTimePointer(unknownAt)
		item.ReconcileAfter = nullableTimePointer(reconcileAfter)
		item.Actionable = knownCallbackRecoveryAction(item.ActionKey) && reconcileAfter.Valid && !reconcileAfter.Time.After(time.Now().UTC())
		item.LastErrorCode = callbackRecoveryErrorCode(lastError)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return companyprofile.CallbackSideEffectPage{}, err
	}
	page := companyprofile.CallbackSideEffectPage{Items: items}
	if len(page.Items) > limit {
		last := page.Items[limit-1]
		page.Items = page.Items[:limit]
		if last.UnknownAt != nil {
			page.NextCursor = encodeCallbackSideEffectCursor(callbackSideEffectCursor{UnknownAt: last.UnknownAt.UTC().Format(time.RFC3339Nano), EventKey: last.EventKey, ActionKey: last.ActionKey})
		}
	}
	return page, nil
}

func (s *MySQLStore) GetCallbackSideEffect(ctx context.Context, principal dashboardprincipal.DashboardPrincipal, eventKey, actionKey string) (companyprofile.CallbackSideEffectDetail, error) {
	if s == nil || s.db == nil {
		return companyprofile.CallbackSideEffectDetail{}, companyprofile.ErrStoreUnavailable
	}
	if err := s.checkCompanyActor(ctx, s.db, principal, false); err != nil {
		return companyprofile.CallbackSideEffectDetail{}, err
	}
	var detail companyprofile.CallbackSideEffectDetail
	var lastError string
	if err := s.db.QueryRowContext(ctx, `SELECT event_key,status,lease_fence,attempt,last_error
		FROM mochat_go_wework_callback_inbox WHERE tenant_id=? AND corp_id=? AND event_key=?`, principal.TenantID, principal.CorpID, eventKey).
		Scan(&detail.EventKey, &detail.InboxStatus, &detail.InboxLeaseFence, &detail.Attempt, &lastError); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return companyprofile.CallbackSideEffectDetail{}, companyprofile.ErrNotFound
		}
		return companyprofile.CallbackSideEffectDetail{}, err
	}
	detail.LastErrorCode = callbackRecoveryErrorCode(lastError)
	rows, err := s.db.QueryContext(ctx, `SELECT action_key,payload_hash,status,version,unknown_at,reconcile_after
		FROM mochat_go_wework_callback_side_effects WHERE tenant_id=? AND corp_id=? AND event_key=? ORDER BY action_key ASC`, principal.TenantID, principal.CorpID, eventKey)
	if err != nil {
		return companyprofile.CallbackSideEffectDetail{}, err
	}
	defer rows.Close()
	targetFound := false
	for rows.Next() {
		var item companyprofile.CallbackSideEffectSummary
		var unknownAt, reconcileAfter sql.NullTime
		if err := rows.Scan(&item.ActionKey, &item.PayloadHash, &item.Status, &item.Version, &unknownAt, &reconcileAfter); err != nil {
			return companyprofile.CallbackSideEffectDetail{}, err
		}
		item.EventKey, item.InboxStatus, item.Attempt = eventKey, detail.InboxStatus, detail.Attempt
		item.UnknownAt, item.ReconcileAfter = nullableTimePointer(unknownAt), nullableTimePointer(reconcileAfter)
		item.Actionable = item.Status == "unknown" && knownCallbackRecoveryAction(item.ActionKey) && reconcileAfter.Valid && !reconcileAfter.Time.After(time.Now().UTC())
		detail.Actions = append(detail.Actions, item)
		if item.ActionKey == actionKey {
			targetFound = true
		}
	}
	if err := rows.Err(); err != nil {
		return companyprofile.CallbackSideEffectDetail{}, err
	}
	if !targetFound {
		return companyprofile.CallbackSideEffectDetail{}, companyprofile.ErrNotFound
	}
	return detail, nil
}

func (s *MySQLStore) ReconcileCallbackSideEffect(ctx context.Context, principal dashboardprincipal.DashboardPrincipal, eventKey, actionKey, requestID string, input companyprofile.CallbackSideEffectReconcileInput) (companyprofile.CallbackSideEffectReconcileResult, error) {
	if s == nil || s.db == nil {
		return companyprofile.CallbackSideEffectReconcileResult{}, companyprofile.ErrStoreUnavailable
	}
	fingerprint, err := callbackRecoveryFingerprint(principal, eventKey, actionKey, input)
	if err != nil {
		return companyprofile.CallbackSideEffectReconcileResult{}, companyprofile.ErrInvalidRequest
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return companyprofile.CallbackSideEffectReconcileResult{}, err
	}
	defer rollbackQuietly(tx)
	if err := s.checkCompanyActor(ctx, tx, principal, true); err != nil {
		return companyprofile.CallbackSideEffectReconcileResult{}, err
	}
	var inboxStatus, leaseToken string
	var inboxFence uint64
	var leaseExpires sql.NullTime
	if err := tx.QueryRowContext(ctx, `SELECT status,lease_token,lease_fence,lease_expires_at
		FROM mochat_go_wework_callback_inbox WHERE tenant_id=? AND corp_id=? AND event_key=? FOR UPDATE`, principal.TenantID, principal.CorpID, eventKey).
		Scan(&inboxStatus, &leaseToken, &inboxFence, &leaseExpires); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return companyprofile.CallbackSideEffectReconcileResult{}, companyprofile.ErrNotFound
		}
		return companyprofile.CallbackSideEffectReconcileResult{}, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT action_key,payload_hash,status,version,reconcile_after
		FROM mochat_go_wework_callback_side_effects WHERE tenant_id=? AND corp_id=? AND event_key=? ORDER BY action_key ASC FOR UPDATE`, principal.TenantID, principal.CorpID, eventKey)
	if err != nil {
		return companyprofile.CallbackSideEffectReconcileResult{}, err
	}
	locked := make([]callbackSideEffectLocked, 0, 2)
	for rows.Next() {
		var item callbackSideEffectLocked
		if err := rows.Scan(&item.ActionKey, &item.PayloadHash, &item.Status, &item.Version, &item.ReconcileAfter); err != nil {
			_ = rows.Close()
			return companyprofile.CallbackSideEffectReconcileResult{}, err
		}
		locked = append(locked, item)
	}
	if err := rows.Close(); err != nil {
		return companyprofile.CallbackSideEffectReconcileResult{}, err
	}
	if err := rows.Err(); err != nil {
		return companyprofile.CallbackSideEffectReconcileResult{}, err
	}
	if replay, found, replayErr := callbackRecoveryReceipt(ctx, tx, principal, requestID, fingerprint); replayErr != nil {
		return companyprofile.CallbackSideEffectReconcileResult{}, replayErr
	} else if found {
		if err := tx.Commit(); err != nil {
			return companyprofile.CallbackSideEffectReconcileResult{}, err
		}
		replay.Replayed = true
		return replay, nil
	}
	if input.ExpectedInboxLeaseFence != inboxFence {
		return companyprofile.CallbackSideEffectReconcileResult{}, companyprofile.ErrLeaseFenceConflict
	}
	if inboxStatus == "processing" && leaseToken != "" && leaseExpires.Valid && leaseExpires.Time.After(time.Now().UTC()) {
		return companyprofile.CallbackSideEffectReconcileResult{}, companyprofile.ErrCallbackLeaseActive
	}
	targetIndex := -1
	unknownOther := false
	for index, item := range locked {
		if item.Status == "unknown" && !knownCallbackRecoveryAction(item.ActionKey) {
			return companyprofile.CallbackSideEffectReconcileResult{}, companyprofile.ErrUnsupportedAction
		}
		if item.ActionKey == actionKey {
			targetIndex = index
		} else if item.Status == "unknown" {
			unknownOther = true
		}
	}
	if targetIndex < 0 {
		return companyprofile.CallbackSideEffectReconcileResult{}, companyprofile.ErrNotFound
	}
	target := locked[targetIndex]
	if target.Version != input.ExpectedVersion {
		return companyprofile.CallbackSideEffectReconcileResult{}, companyprofile.ErrVersionConflict
	}
	if target.Status != "unknown" {
		return companyprofile.CallbackSideEffectReconcileResult{}, companyprofile.ErrSideEffectConflict
	}
	if !target.ReconcileAfter.Valid || target.ReconcileAfter.Time.After(time.Now().UTC()) {
		return companyprofile.CallbackSideEffectReconcileResult{}, companyprofile.ErrQuarantineActive
	}
	newStatus := "sent"
	if input.Decision == companyprofile.CallbackSideEffectDecisionConfirmNotSentAndRetry {
		newStatus = "pending"
	}
	resultVersion := target.Version + 1
	updated, err := tx.ExecContext(ctx, `UPDATE mochat_go_wework_callback_side_effects
		SET status=?,version=?,reconciliation_fence=reconciliation_fence+1,last_decision=?,last_reason=?,
		last_evidence_kind=?,last_evidence_ref=?,last_reconciled_by=?,last_reconciled_at=UTC_TIMESTAMP(6),
		sent_at=IF(?='sent',UTC_TIMESTAMP(6),NULL),unknown_at=NULL,reconcile_after=NULL,updated_at=UTC_TIMESTAMP(6)
		WHERE tenant_id=? AND corp_id=? AND event_key=? AND action_key=? AND status='unknown' AND version=?`,
		newStatus, resultVersion, input.Decision, input.Reason, input.EvidenceKind, input.EvidenceRef, principal.UserID, newStatus,
		principal.TenantID, principal.CorpID, eventKey, actionKey, target.Version)
	if err != nil {
		return companyprofile.CallbackSideEffectReconcileResult{}, err
	}
	if err := requireCompanyRows(updated, 1); err != nil {
		return companyprofile.CallbackSideEffectReconcileResult{}, err
	}
	replayScheduled := false
	resultFence := inboxFence
	if !unknownOther {
		resultFence++
		updated, err = tx.ExecContext(ctx, `UPDATE mochat_go_wework_callback_inbox
			SET status='pending',attempt=0,lease_token='',lease_fence=?,lease_expires_at=NULL,next_attempt_at=NULL,last_error='',completed_at=NULL,updated_at=UTC_TIMESTAMP(6)
			WHERE tenant_id=? AND corp_id=? AND event_key=? AND lease_fence=?`, resultFence, principal.TenantID, principal.CorpID, eventKey, inboxFence)
		if err != nil {
			return companyprofile.CallbackSideEffectReconcileResult{}, err
		}
		if err := requireCompanyRows(updated, 1); err != nil {
			return companyprofile.CallbackSideEffectReconcileResult{}, err
		}
		replayScheduled = true
	}
	beforeJSON, _ := json.Marshal(map[string]any{"corpId": principal.CorpID, "eventKey": eventKey, "actionKey": actionKey, "status": target.Status, "version": target.Version, "inboxLeaseFence": inboxFence})
	afterJSON, _ := json.Marshal(map[string]any{"corpId": principal.CorpID, "eventKey": eventKey, "actionKey": actionKey, "status": newStatus, "version": resultVersion, "inboxLeaseFence": resultFence, "replayScheduled": replayScheduled, "decision": input.Decision, "evidenceKind": input.EvidenceKind, "evidenceRef": input.EvidenceRef})
	audit, err := tx.ExecContext(ctx, `INSERT INTO mochat_go_dashboard_permission_audits
		(tenant_id,actor_user_id,action,target_type,target_id,before_json,after_json,expected_version,result_version,request_id,created_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,UTC_TIMESTAMP(6))`, principal.TenantID, principal.UserID, "dashboard.company.callback_side_effect.reconcile", "callback_side_effect", eventKey, string(beforeJSON), string(afterJSON), target.Version, resultVersion, requestID)
	if err != nil {
		return companyprofile.CallbackSideEffectReconcileResult{}, err
	}
	auditID, err := audit.LastInsertId()
	if err != nil {
		return companyprofile.CallbackSideEffectReconcileResult{}, err
	}
	inserted, err := tx.ExecContext(ctx, `INSERT INTO mochat_go_wework_callback_side_effect_commands
		(tenant_id,corp_id,event_key,action_key,request_id,decision,request_fingerprint,expected_version,expected_inbox_lease_fence,
		 result_status,result_version,result_inbox_lease_fence,replay_scheduled,actor_user_id,reason,evidence_kind,evidence_ref,operation_audit_id)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, principal.TenantID, principal.CorpID, eventKey, actionKey, requestID, input.Decision, fingerprint[:], target.Version, inboxFence,
		newStatus, resultVersion, resultFence, replayScheduled, principal.UserID, input.Reason, input.EvidenceKind, input.EvidenceRef, auditID)
	if err != nil {
		return companyprofile.CallbackSideEffectReconcileResult{}, err
	}
	if err := requireCompanyRows(inserted, 1); err != nil {
		return companyprofile.CallbackSideEffectReconcileResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return companyprofile.CallbackSideEffectReconcileResult{}, err
	}
	return companyprofile.CallbackSideEffectReconcileResult{EventKey: eventKey, ActionKey: actionKey, Status: newStatus, Version: resultVersion, InboxLeaseFence: resultFence, ReplayScheduled: replayScheduled}, nil
}

func callbackRecoveryReceipt(ctx context.Context, tx *sql.Tx, principal dashboardprincipal.DashboardPrincipal, requestID string, fingerprint [32]byte) (companyprofile.CallbackSideEffectReconcileResult, bool, error) {
	var result companyprofile.CallbackSideEffectReconcileResult
	var storedFingerprint []byte
	err := tx.QueryRowContext(ctx, `SELECT event_key,action_key,request_fingerprint,result_status,result_version,result_inbox_lease_fence,replay_scheduled
		FROM mochat_go_wework_callback_side_effect_commands WHERE tenant_id=? AND corp_id=? AND request_id=? FOR UPDATE`, principal.TenantID, principal.CorpID, requestID).
		Scan(&result.EventKey, &result.ActionKey, &storedFingerprint, &result.Status, &result.Version, &result.InboxLeaseFence, &result.ReplayScheduled)
	if errors.Is(err, sql.ErrNoRows) {
		return companyprofile.CallbackSideEffectReconcileResult{}, false, nil
	}
	if err != nil {
		return companyprofile.CallbackSideEffectReconcileResult{}, false, err
	}
	if !bytes.Equal(storedFingerprint, fingerprint[:]) {
		return companyprofile.CallbackSideEffectReconcileResult{}, false, companyprofile.ErrIdempotencyConflict
	}
	return result, true, nil
}

func callbackRecoveryFingerprint(principal dashboardprincipal.DashboardPrincipal, eventKey, actionKey string, input companyprofile.CallbackSideEffectReconcileInput) ([32]byte, error) {
	canonical := struct {
		TenantID                int    `json:"tenantId"`
		CorpID                  int    `json:"corpId"`
		EventKey                string `json:"eventKey"`
		ActionKey               string `json:"actionKey"`
		Decision                string `json:"decision"`
		ExpectedVersion         uint64 `json:"expectedVersion"`
		ExpectedInboxLeaseFence uint64 `json:"expectedInboxLeaseFence"`
		Reason                  string `json:"reason"`
		EvidenceKind            string `json:"evidenceKind"`
		EvidenceRef             string `json:"evidenceRef"`
	}{principal.TenantID, principal.CorpID, eventKey, actionKey, input.Decision, input.ExpectedVersion, input.ExpectedInboxLeaseFence, input.Reason, input.EvidenceKind, input.EvidenceRef}
	raw, err := json.Marshal(canonical)
	if err != nil {
		return [32]byte{}, err
	}
	return sha256.Sum256(raw), nil
}

func encodeCallbackSideEffectCursor(cursor callbackSideEffectCursor) string {
	raw, _ := json.Marshal(cursor)
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodeCallbackSideEffectCursor(value string) (callbackSideEffectCursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(value))
	if err != nil || len(raw) > 1024 {
		return callbackSideEffectCursor{}, errors.New("invalid callback side effect cursor")
	}
	var cursor callbackSideEffectCursor
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cursor); err != nil || cursor.UnknownAt == "" || len(cursor.EventKey) != 64 || cursor.ActionKey == "" {
		return callbackSideEffectCursor{}, errors.New("invalid callback side effect cursor")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return callbackSideEffectCursor{}, errors.New("invalid callback side effect cursor")
	}
	return cursor, nil
}

func nullableTimePointer(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time.UTC()
	return &result
}

func callbackRecoveryErrorCode(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return "CALLBACK_PROCESSING_FAILED"
}

func knownCallbackRecoveryAction(actionKey string) bool {
	return actionKey == "fission.employee_reminder" || actionKey == "fission.customer_push"
}

var _ companyprofile.CallbackSideEffectRecoveryStore = (*MySQLStore)(nil)
