package store

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"regexp"
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
	ActionKey            string
	PayloadHash          string
	Status               string
	Version              uint64
	ReconcileAfterValid  bool
	ReconcileAfterFuture bool
}

var callbackSideEffectCursorEventKeyPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

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
		COALESCE(se.reconcile_after<=UTC_TIMESTAMP(6),FALSE),
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
		cursorClock := callbackRecoveryUTCValue(cursorTime)
		query += ` AND (se.unknown_at<? OR (se.unknown_at=? AND se.event_key<?) OR (se.unknown_at=? AND se.event_key=? AND se.action_key<?))`
		args = append(args, cursorClock, cursorClock, cursor.EventKey, cursorClock, cursor.EventKey, cursor.ActionKey)
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
		var reconcileReady bool
		var lastError string
		if err := rows.Scan(&item.EventKey, &item.ActionKey, &item.PayloadHash, &item.Status, &item.Version, &unknownAt, &reconcileAfter, &reconcileReady, &item.InboxStatus, &item.Attempt, &lastError); err != nil {
			return companyprofile.CallbackSideEffectPage{}, err
		}
		unknownAt, reconcileAfter = callbackRecoveryUTCTime(unknownAt), callbackRecoveryUTCTime(reconcileAfter)
		item.UnknownAt = nullableTimePointer(unknownAt)
		item.ReconcileAfter = nullableTimePointer(reconcileAfter)
		item.Actionable = knownCallbackRecoveryAction(item.ActionKey) && reconcileReady
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
			return companyprofile.CallbackSideEffectDetail{}, companyprofile.ErrRecoveryTargetNotFound
		}
		return companyprofile.CallbackSideEffectDetail{}, err
	}
	detail.LastErrorCode = callbackRecoveryErrorCode(lastError)
	rows, err := s.db.QueryContext(ctx, `SELECT action_key,payload_hash,status,version,unknown_at,reconcile_after,
		COALESCE(reconcile_after<=UTC_TIMESTAMP(6),FALSE)
		FROM mochat_go_wework_callback_side_effects WHERE tenant_id=? AND corp_id=? AND event_key=? ORDER BY action_key ASC`, principal.TenantID, principal.CorpID, eventKey)
	if err != nil {
		return companyprofile.CallbackSideEffectDetail{}, err
	}
	defer rows.Close()
	targetFound := false
	for rows.Next() {
		var item companyprofile.CallbackSideEffectSummary
		var unknownAt, reconcileAfter sql.NullTime
		var reconcileReady bool
		if err := rows.Scan(&item.ActionKey, &item.PayloadHash, &item.Status, &item.Version, &unknownAt, &reconcileAfter, &reconcileReady); err != nil {
			return companyprofile.CallbackSideEffectDetail{}, err
		}
		unknownAt, reconcileAfter = callbackRecoveryUTCTime(unknownAt), callbackRecoveryUTCTime(reconcileAfter)
		item.EventKey, item.InboxStatus, item.Attempt = eventKey, detail.InboxStatus, detail.Attempt
		item.UnknownAt, item.ReconcileAfter = nullableTimePointer(unknownAt), nullableTimePointer(reconcileAfter)
		item.Actionable = item.Status == "unknown" && knownCallbackRecoveryAction(item.ActionKey) && reconcileReady
		detail.Actions = append(detail.Actions, item)
		if item.ActionKey == actionKey {
			targetFound = true
		}
	}
	if err := rows.Err(); err != nil {
		return companyprofile.CallbackSideEffectDetail{}, err
	}
	if !targetFound {
		return companyprofile.CallbackSideEffectDetail{}, companyprofile.ErrRecoveryTargetNotFound
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
	binding, err := s.loadCompanyBinding(ctx, tx, principal, true)
	if err != nil {
		if errors.Is(err, companyprofile.ErrNotFound) {
			return companyprofile.CallbackSideEffectReconcileResult{}, companyprofile.ErrTenantAccessDenied
		}
		return companyprofile.CallbackSideEffectReconcileResult{}, err
	}
	if binding.Status != 2 {
		return companyprofile.CallbackSideEffectReconcileResult{}, companyprofile.ErrTenantAccessDenied
	}
	replay, receiptID, reservationToken, reserved, err := callbackRecoveryReserve(ctx, tx, principal, eventKey, actionKey, requestID, fingerprint, input)
	if err != nil {
		return companyprofile.CallbackSideEffectReconcileResult{}, err
	}
	if !reserved {
		if err := tx.Commit(); err != nil {
			return companyprofile.CallbackSideEffectReconcileResult{}, err
		}
		return replay, nil
	}
	var inboxStatus, leaseToken string
	var inboxFence uint64
	var leaseExpiresValid, leaseActive bool
	if err := tx.QueryRowContext(ctx, `SELECT status,lease_token,lease_fence,lease_expires_at IS NOT NULL,
		COALESCE(lease_expires_at>UTC_TIMESTAMP(6),FALSE)
		FROM mochat_go_wework_callback_inbox WHERE tenant_id=? AND corp_id=? AND event_key=? FOR UPDATE`, principal.TenantID, principal.CorpID, eventKey).
		Scan(&inboxStatus, &leaseToken, &inboxFence, &leaseExpiresValid, &leaseActive); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return companyprofile.CallbackSideEffectReconcileResult{}, companyprofile.ErrRecoveryTargetNotFound
		}
		return companyprofile.CallbackSideEffectReconcileResult{}, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT action_key,payload_hash,status,version,reconcile_after IS NOT NULL,
		COALESCE(reconcile_after>UTC_TIMESTAMP(6),FALSE)
		FROM mochat_go_wework_callback_side_effects WHERE tenant_id=? AND corp_id=? AND event_key=? ORDER BY action_key ASC FOR UPDATE`, principal.TenantID, principal.CorpID, eventKey)
	if err != nil {
		return companyprofile.CallbackSideEffectReconcileResult{}, err
	}
	locked := make([]callbackSideEffectLocked, 0, 2)
	for rows.Next() {
		var item callbackSideEffectLocked
		if err := rows.Scan(&item.ActionKey, &item.PayloadHash, &item.Status, &item.Version, &item.ReconcileAfterValid, &item.ReconcileAfterFuture); err != nil {
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
	if input.ExpectedInboxLeaseFence != inboxFence {
		return companyprofile.CallbackSideEffectReconcileResult{}, companyprofile.ErrLeaseFenceConflict
	}
	switch inboxStatus {
	case "dead", "pending":
	case "processing":
		if leaseActive {
			return companyprofile.CallbackSideEffectReconcileResult{}, companyprofile.ErrCallbackLeaseActive
		}
		if leaseToken == "" || !leaseExpiresValid {
			return companyprofile.CallbackSideEffectReconcileResult{}, companyprofile.ErrInboxStateConflict
		}
	default:
		return companyprofile.CallbackSideEffectReconcileResult{}, companyprofile.ErrInboxStateConflict
	}
	targetIndex := -1
	remainingUnknownActions := 0
	for index, item := range locked {
		if item.Status == "unknown" && !knownCallbackRecoveryAction(item.ActionKey) {
			return companyprofile.CallbackSideEffectReconcileResult{}, companyprofile.ErrUnsupportedAction
		}
		if item.ActionKey == actionKey {
			targetIndex = index
		} else if item.Status == "unknown" {
			remainingUnknownActions++
		}
	}
	if targetIndex < 0 {
		return companyprofile.CallbackSideEffectReconcileResult{}, companyprofile.ErrRecoveryTargetNotFound
	}
	target := locked[targetIndex]
	if target.Version != input.ExpectedVersion {
		return companyprofile.CallbackSideEffectReconcileResult{}, companyprofile.ErrVersionConflict
	}
	if target.Status != "unknown" {
		return companyprofile.CallbackSideEffectReconcileResult{}, companyprofile.ErrSideEffectConflict
	}
	if !target.ReconcileAfterValid || target.ReconcileAfterFuture {
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
	if remainingUnknownActions == 0 {
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
	updatedReceipt, err := tx.ExecContext(ctx, `UPDATE mochat_go_wework_callback_side_effect_commands
		SET result_status=?,result_version=?,result_inbox_lease_fence=?,replay_scheduled=?,remaining_unknown_actions=?,
			operation_audit_id=?,reservation_token=''
		WHERE id=? AND reservation_token=? AND result_status='reserved'`, newStatus, resultVersion, resultFence, replayScheduled,
		remainingUnknownActions, auditID, receiptID, reservationToken)
	if err != nil {
		return companyprofile.CallbackSideEffectReconcileResult{}, err
	}
	if err := requireCompanyRows(updatedReceipt, 1); err != nil {
		return companyprofile.CallbackSideEffectReconcileResult{}, err
	}
	if err := s.commitCallbackRecovery(tx); err != nil {
		return companyprofile.CallbackSideEffectReconcileResult{}, err
	}
	return companyprofile.CallbackSideEffectReconcileResult{EventKey: eventKey, ActionKey: actionKey, Status: newStatus, Version: resultVersion,
		InboxLeaseFence: resultFence, InboxReplayScheduled: replayScheduled, RemainingUnknownActions: remainingUnknownActions}, nil
}

func callbackRecoveryReserve(ctx context.Context, tx *sql.Tx, principal dashboardprincipal.DashboardPrincipal, eventKey, actionKey, requestID string,
	fingerprint [32]byte, input companyprofile.CallbackSideEffectReconcileInput) (companyprofile.CallbackSideEffectReconcileResult, int64, string, bool, error) {
	reservationToken, err := callbackRecoveryReservationToken()
	if err != nil {
		return companyprofile.CallbackSideEffectReconcileResult{}, 0, "", false, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO mochat_go_wework_callback_side_effect_commands
		(tenant_id,corp_id,event_key,action_key,request_id,decision,request_fingerprint,reservation_token,expected_version,expected_inbox_lease_fence,
		 result_status,result_version,result_inbox_lease_fence,replay_scheduled,remaining_unknown_actions,actor_user_id,reason,evidence_kind,evidence_ref,operation_audit_id)
		VALUES (?,?,?,?,?,?,?,?,?,?,'reserved',0,0,0,0,?,?,?,?,0)
		ON DUPLICATE KEY UPDATE id=id`, principal.TenantID, principal.CorpID, eventKey, actionKey, requestID, input.Decision, fingerprint[:], reservationToken,
		input.ExpectedVersion, input.ExpectedInboxLeaseFence, principal.UserID, input.Reason, input.EvidenceKind, input.EvidenceRef); err != nil {
		return companyprofile.CallbackSideEffectReconcileResult{}, 0, "", false, err
	}
	var result companyprofile.CallbackSideEffectReconcileResult
	var receiptID int64
	var storedFingerprint []byte
	var storedReservation string
	err = tx.QueryRowContext(ctx, `SELECT id,event_key,action_key,request_fingerprint,reservation_token,result_status,result_version,result_inbox_lease_fence,replay_scheduled,remaining_unknown_actions
		FROM mochat_go_wework_callback_side_effect_commands WHERE tenant_id=? AND corp_id=? AND request_id=? FOR UPDATE`, principal.TenantID, principal.CorpID, requestID).
		Scan(&receiptID, &result.EventKey, &result.ActionKey, &storedFingerprint, &storedReservation, &result.Status, &result.Version, &result.InboxLeaseFence,
			&result.InboxReplayScheduled, &result.RemainingUnknownActions)
	if err != nil {
		return companyprofile.CallbackSideEffectReconcileResult{}, 0, "", false, err
	}
	if !bytes.Equal(storedFingerprint, fingerprint[:]) {
		return companyprofile.CallbackSideEffectReconcileResult{}, 0, "", false, companyprofile.ErrIdempotencyConflict
	}
	if storedReservation == reservationToken && result.Status == "reserved" {
		return companyprofile.CallbackSideEffectReconcileResult{}, receiptID, reservationToken, true, nil
	}
	if storedReservation != "" || result.Status == "reserved" {
		return companyprofile.CallbackSideEffectReconcileResult{}, 0, "", false, companyprofile.ErrRecoveryUnavailable
	}
	result.Idempotent = true
	return result, receiptID, "", false, nil
}

func callbackRecoveryReservationToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

func (s *MySQLStore) commitCallbackRecovery(tx *sql.Tx) error {
	if s.callbackRecoveryCommit != nil {
		return s.callbackRecoveryCommit(tx)
	}
	return tx.Commit()
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
	if err := decoder.Decode(&cursor); err != nil || cursor.UnknownAt == "" || !callbackSideEffectCursorEventKeyPattern.MatchString(cursor.EventKey) || !knownCallbackRecoveryAction(cursor.ActionKey) {
		return callbackSideEffectCursor{}, errors.New("invalid callback side effect cursor")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return callbackSideEffectCursor{}, errors.New("invalid callback side effect cursor")
	}
	return cursor, nil
}

// Callback recovery timestamps are written with UTC_TIMESTAMP into timezone-
// free DATETIME columns. Reinterpret the scanned wall clock as UTC so a DSN
// using loc=Local cannot shift API timestamps or cursor boundaries.
func callbackRecoveryUTCTime(value sql.NullTime) sql.NullTime {
	if !value.Valid {
		return value
	}
	clock := value.Time
	value.Time = time.Date(clock.Year(), clock.Month(), clock.Day(), clock.Hour(), clock.Minute(), clock.Second(), clock.Nanosecond(), time.UTC)
	return value
}

func callbackRecoveryUTCValue(value time.Time) string {
	return value.UTC().Format("2006-01-02 15:04:05.999999")
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
