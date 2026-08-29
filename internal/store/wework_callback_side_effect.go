package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"jiyi/mochat-go/internal/dashboard"
)

func (s *MySQLStore) BeginWeWorkCallbackSideEffect(ctx context.Context, tenantID, corpID int, eventKey, actionKey, payloadHash string) (bool, string, error) {
	if s == nil || s.db == nil {
		return false, "", errors.New("wework callback side effect store is not configured")
	}
	if err := validateWeWorkCallbackSideEffectIdentity(tenantID, corpID, eventKey, actionKey, payloadHash); err != nil {
		return false, "", err
	}
	result, err := s.db.ExecContext(ctx, "UPDATE mochat_go_wework_callback_side_effects SET status='unknown',updated_at=UTC_TIMESTAMP(6) WHERE tenant_id=? AND corp_id=? AND event_key=? AND action_key=? AND payload_hash=? AND status='pending'", tenantID, corpID, eventKey, actionKey, payloadHash)
	if err != nil {
		return false, "", err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, "", err
	}
	if affected == 1 {
		return true, dashboard.WeWorkCallbackSideEffectUnknown, nil
	}
	if affected > 1 {
		return false, "", errors.New("wework callback side effect scope affected multiple rows")
	}
	var status, storedHash string
	err = s.db.QueryRowContext(ctx, `SELECT status,payload_hash FROM mochat_go_wework_callback_side_effects WHERE tenant_id=? AND corp_id=? AND event_key=? AND action_key=?`, tenantID, corpID, eventKey, actionKey).Scan(&status, &storedHash)
	if err != nil {
		return false, "", err
	}
	if storedHash != payloadHash {
		return false, "", errors.New("wework callback side effect payload conflicts with durable intent")
	}
	switch status {
	case dashboard.WeWorkCallbackSideEffectSent, dashboard.WeWorkCallbackSideEffectUnknown:
		return false, status, nil
	default:
		return false, "", fmt.Errorf("wework callback side effect has invalid status %q", status)
	}
}

func (s *MySQLStore) CompleteWeWorkCallbackSideEffect(ctx context.Context, tenantID, corpID int, eventKey, actionKey, payloadHash string) error {
	if s == nil || s.db == nil {
		return errors.New("wework callback side effect store is not configured")
	}
	if err := validateWeWorkCallbackSideEffectIdentity(tenantID, corpID, eventKey, actionKey, payloadHash); err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, "UPDATE mochat_go_wework_callback_side_effects SET status='sent',sent_at=UTC_TIMESTAMP(6),updated_at=UTC_TIMESTAMP(6) WHERE tenant_id=? AND corp_id=? AND event_key=? AND action_key=? AND payload_hash=? AND status='unknown'", tenantID, corpID, eventKey, actionKey, payloadHash)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 1 {
		return nil
	}
	if affected > 1 {
		return errors.New("wework callback side effect completion affected multiple rows")
	}
	var status, storedHash string
	if err := s.db.QueryRowContext(ctx, `SELECT status,payload_hash FROM mochat_go_wework_callback_side_effects WHERE tenant_id=? AND corp_id=? AND event_key=? AND action_key=?`, tenantID, corpID, eventKey, actionKey).Scan(&status, &storedHash); err != nil {
		return err
	}
	if storedHash == payloadHash && status == dashboard.WeWorkCallbackSideEffectSent {
		return nil
	}
	return errors.New("wework callback side effect completion lost durable ownership")
}

func insertWeWorkCallbackSideEffectIntent(ctx context.Context, tx *sql.Tx, execution dashboard.WeWorkCallbackExecution, actionKey string, payload any) error {
	if tx == nil || execution.TenantID <= 0 || execution.CorpID <= 0 || len(strings.TrimSpace(execution.EventKey)) != 64 {
		return errors.New("wework callback side effect execution scope is invalid")
	}
	payloadHash, err := dashboard.WeWorkCallbackSideEffectPayloadHash(actionKey, payload)
	if err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO mochat_go_wework_callback_side_effects (tenant_id,corp_id,event_key,action_key,payload_hash,status) VALUES (?,?,?,?,?,'pending')`, execution.TenantID, execution.CorpID, execution.EventKey, actionKey, payloadHash)
	if err == nil {
		affected, affectedErr := result.RowsAffected()
		if affectedErr != nil {
			return affectedErr
		}
		if affected != 1 {
			return errors.New("wework callback side effect intent affected an unexpected number of rows")
		}
		return nil
	}
	if !isMySQLDuplicateKeyError(err) {
		return err
	}
	var storedHash string
	if err := tx.QueryRowContext(ctx, `SELECT payload_hash FROM mochat_go_wework_callback_side_effects WHERE tenant_id=? AND corp_id=? AND event_key=? AND action_key=? FOR UPDATE`, execution.TenantID, execution.CorpID, execution.EventKey, actionKey).Scan(&storedHash); err != nil {
		return err
	}
	if storedHash != payloadHash {
		return errors.New("wework callback side effect payload conflicts with durable intent")
	}
	return nil
}

func validateWeWorkCallbackSideEffectIdentity(tenantID, corpID int, eventKey, actionKey, payloadHash string) error {
	if tenantID <= 0 || corpID <= 0 || len(strings.TrimSpace(eventKey)) != 64 || strings.TrimSpace(actionKey) == "" || len(actionKey) > 64 || len(strings.TrimSpace(payloadHash)) != 64 {
		return errors.New("invalid wework callback side effect identity")
	}
	return nil
}
