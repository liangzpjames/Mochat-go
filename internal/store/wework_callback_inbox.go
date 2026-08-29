package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"jiyi/mochat-go/internal/dashboard"
)

func (s *MySQLStore) AcceptWeWorkCallback(ctx context.Context, event dashboard.WeWorkCallbackEvent, eventKey string, fingerprint string) (bool, error) {
	if s == nil || s.db == nil {
		return false, errors.New("wework callback inbox store is not configured")
	}
	eventKey = strings.TrimSpace(eventKey)
	fingerprint = strings.TrimSpace(fingerprint)
	if event.TenantID <= 0 || event.CorpID <= 0 || len(eventKey) != 64 || len(fingerprint) != 64 {
		return false, errors.New("invalid wework callback acceptance")
	}
	event.RawXML = ""
	event.Message = normalizedCallbackMessageForStorage(event.Message)
	raw, err := json.Marshal(event)
	if err != nil {
		return false, fmt.Errorf("encode wework callback event: %w", err)
	}
	receivedAt := time.Now().UTC()
	if parsed, parseErr := time.ParseInLocation("2006-01-02 15:04:05", strings.TrimSpace(event.ReceivedAt), time.Local); parseErr == nil {
		receivedAt = parsed
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO mochat_go_wework_callback_inbox
			(tenant_id,corp_id,event_key,payload_fingerprint,event_path,event_json,status,received_at)
		VALUES (?,?,?,?,?,?,'pending',?)
	`, event.TenantID, event.CorpID, eventKey, fingerprint, strings.TrimSpace(event.EventPath), string(raw), receivedAt)
	if err == nil {
		return false, nil
	}
	if !isMySQLDuplicateKeyError(err) {
		return false, err
	}
	var storedFingerprint string
	lookupErr := s.db.QueryRowContext(ctx, `
		SELECT payload_fingerprint
		FROM mochat_go_wework_callback_inbox
		WHERE tenant_id=? AND corp_id=? AND event_key=?
	`, event.TenantID, event.CorpID, eventKey).Scan(&storedFingerprint)
	if lookupErr != nil {
		return false, lookupErr
	}
	if storedFingerprint != fingerprint {
		return false, dashboard.ErrWeWorkCallbackConflict
	}
	return true, nil
}

func (s *MySQLStore) ClaimWeWorkCallback(ctx context.Context, leaseDuration time.Duration) (dashboard.WeWorkCallbackClaim, bool, error) {
	if s == nil || s.db == nil {
		return dashboard.WeWorkCallbackClaim{}, false, errors.New("wework callback inbox store is not configured")
	}
	if leaseDuration <= 0 {
		return dashboard.WeWorkCallbackClaim{}, false, errors.New("wework callback lease duration must be positive")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.WeWorkCallbackClaim{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	var claim dashboard.WeWorkCallbackClaim
	var raw string
	err = tx.QueryRowContext(ctx, `
		SELECT id,event_key,payload_fingerprint,event_json,attempt,lease_fence
		FROM mochat_go_wework_callback_inbox
		WHERE (status='pending' AND (next_attempt_at IS NULL OR next_attempt_at<=UTC_TIMESTAMP(6)))
		   OR (status='processing' AND lease_expires_at<=UTC_TIMESTAMP(6))
		ORDER BY id ASC
		LIMIT 1
		FOR UPDATE
	`).Scan(&claim.ID, &claim.EventKey, &claim.PayloadFingerprint, &raw, &claim.Attempt, &claim.LeaseFence)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.WeWorkCallbackClaim{}, false, nil
	}
	if err != nil {
		return dashboard.WeWorkCallbackClaim{}, false, err
	}
	claim.LeaseToken, err = newWeWorkCallbackLeaseToken()
	if err != nil {
		return dashboard.WeWorkCallbackClaim{}, false, err
	}
	claim.Attempt++
	claim.LeaseFence++
	result, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_wework_callback_inbox
		SET status='processing',attempt=?,lease_token=?,lease_fence=?,
		    lease_expires_at=DATE_ADD(UTC_TIMESTAMP(6), INTERVAL ? MICROSECOND),next_attempt_at=NULL,last_error=''
		WHERE id=?
	`, claim.Attempt, claim.LeaseToken, claim.LeaseFence, leaseDuration.Microseconds(), claim.ID)
	if err != nil {
		return dashboard.WeWorkCallbackClaim{}, false, err
	}
	if affected, affectedErr := result.RowsAffected(); affectedErr != nil || affected != 1 {
		if affectedErr != nil {
			return dashboard.WeWorkCallbackClaim{}, false, affectedErr
		}
		return dashboard.WeWorkCallbackClaim{}, false, dashboard.ErrWeWorkCallbackLeaseLost
	}
	if err := json.Unmarshal([]byte(raw), &claim.Event); err != nil {
		return dashboard.WeWorkCallbackClaim{}, false, fmt.Errorf("decode wework callback event: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return dashboard.WeWorkCallbackClaim{}, false, err
	}
	return claim, true, nil
}

func (s *MySQLStore) CompleteWeWorkCallback(ctx context.Context, claim dashboard.WeWorkCallbackClaim) error {
	if s == nil || s.db == nil {
		return errors.New("wework callback inbox store is not configured")
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE mochat_go_wework_callback_inbox
		SET status='completed',lease_token='',lease_expires_at=NULL,next_attempt_at=NULL,last_error='',completed_at=UTC_TIMESTAMP(6)
		WHERE id=? AND event_key=? AND status='processing' AND lease_token=? AND lease_fence=? AND lease_expires_at>UTC_TIMESTAMP(6)
	`, claim.ID, strings.TrimSpace(claim.EventKey), strings.TrimSpace(claim.LeaseToken), claim.LeaseFence)
	return callbackLeaseUpdateError(result, err)
}

func (s *MySQLStore) ValidateWeWorkCallbackClaim(ctx context.Context, claim dashboard.WeWorkCallbackClaim) error {
	if s == nil || s.db == nil {
		return errors.New("wework callback inbox store is not configured")
	}
	var marker int
	err := s.db.QueryRowContext(ctx, `
		SELECT 1
		FROM mochat_go_wework_callback_inbox
		WHERE id=? AND event_key=? AND status='processing' AND lease_token=? AND lease_fence=? AND lease_expires_at>UTC_TIMESTAMP(6)
	`, claim.ID, strings.TrimSpace(claim.EventKey), strings.TrimSpace(claim.LeaseToken), claim.LeaseFence).Scan(&marker)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.ErrWeWorkCallbackLeaseLost
	}
	return err
}

func (s *MySQLStore) FailWeWorkCallback(ctx context.Context, claim dashboard.WeWorkCallbackClaim, reason string, maxAttempts int, retryDelay time.Duration) (bool, error) {
	if s == nil || s.db == nil {
		return false, errors.New("wework callback inbox store is not configured")
	}
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	if retryDelay < 0 {
		retryDelay = 0
	}
	dead := claim.Attempt >= maxAttempts
	status := "pending"
	nextAttemptSQL := "DATE_ADD(UTC_TIMESTAMP(6), INTERVAL ? MICROSECOND)"
	completedAtSQL := "NULL"
	if dead {
		status = "dead"
		nextAttemptSQL = "NULL"
		completedAtSQL = "UTC_TIMESTAMP(6)"
	}
	query := `UPDATE mochat_go_wework_callback_inbox
		SET status=?,lease_token='',lease_expires_at=NULL,next_attempt_at=` + nextAttemptSQL + `,
		    last_error=?,completed_at=` + completedAtSQL + `
		WHERE id=? AND event_key=? AND status='processing' AND lease_token=? AND lease_fence=? AND lease_expires_at>UTC_TIMESTAMP(6)`
	args := []any{status}
	if !dead {
		args = append(args, retryDelay.Microseconds())
	}
	args = append(args, truncateWeWorkCallbackError(reason), claim.ID, strings.TrimSpace(claim.EventKey), strings.TrimSpace(claim.LeaseToken), claim.LeaseFence)
	result, err := s.db.ExecContext(ctx, query, args...)
	if updateErr := callbackLeaseUpdateError(result, err); updateErr != nil {
		return false, updateErr
	}
	return dead, nil
}

func callbackLeaseUpdateError(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return dashboard.ErrWeWorkCallbackLeaseLost
	}
	return nil
}

func newWeWorkCallbackLeaseToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

func normalizedCallbackMessageForStorage(message map[string]string) map[string]string {
	normalized := make(map[string]string, len(message))
	for key, value := range message {
		key = strings.TrimSpace(key)
		if key != "" {
			normalized[key] = strings.TrimSpace(value)
		}
	}
	return normalized
}

func truncateWeWorkCallbackError(value string) string {
	value = dashboard.SanitizeWeWorkCallbackFailure(value)
	if utf8.RuneCountInString(value) <= 512 {
		return value
	}
	runes := []rune(value)
	return string(runes[:512])
}
