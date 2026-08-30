package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"jiyi/mochat-go/internal/dashboard"
)

func (s *MySQLStore) BeginWeWorkCallbackSideEffect(ctx context.Context, execution dashboard.WeWorkCallbackExecution, actionKey, payloadHash string) (bool, string, error) {
	if s == nil || s.db == nil {
		return false, "", errors.New("wework callback side effect store is not configured")
	}
	if err := validateWeWorkCallbackSideEffectExecution(execution, actionKey, payloadHash); err != nil {
		return false, "", err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, "", err
	}
	defer rollbackQuietly(tx)
	if err := lockValidWeWorkCallbackExecution(ctx, tx, execution); err != nil {
		return false, "", err
	}
	var status, storedHash string
	err = tx.QueryRowContext(ctx, `SELECT status,payload_hash FROM mochat_go_wework_callback_side_effects
		WHERE tenant_id=? AND corp_id=? AND event_key=? AND action_key=? FOR UPDATE`, execution.TenantID, execution.CorpID, execution.EventKey, actionKey).Scan(&status, &storedHash)
	if err != nil {
		return false, "", err
	}
	if storedHash != payloadHash {
		return false, "", errors.New("wework callback side effect payload conflicts with durable intent")
	}
	if status == dashboard.WeWorkCallbackSideEffectSent || status == dashboard.WeWorkCallbackSideEffectUnknown {
		if err := tx.Commit(); err != nil {
			return false, "", err
		}
		return false, status, nil
	}
	if status != dashboard.WeWorkCallbackSideEffectPending {
		return false, "", fmt.Errorf("wework callback side effect has invalid status %q", status)
	}
	result, err := tx.ExecContext(ctx, `UPDATE mochat_go_wework_callback_side_effects
		SET status='unknown',version=version+1,unknown_at=UTC_TIMESTAMP(6),
		    reconcile_after=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 15 MINUTE),updated_at=UTC_TIMESTAMP(6)
		WHERE tenant_id=? AND corp_id=? AND event_key=? AND action_key=? AND payload_hash=? AND status='pending'`, execution.TenantID, execution.CorpID, execution.EventKey, actionKey, payloadHash)
	if err != nil {
		return false, "", err
	}
	if affected, affectedErr := result.RowsAffected(); affectedErr != nil || affected != 1 {
		if affectedErr != nil {
			return false, "", affectedErr
		}
		return false, "", errors.New("wework callback side effect begin lost durable ownership")
	}
	if err := tx.Commit(); err != nil {
		return false, "", err
	}
	return true, dashboard.WeWorkCallbackSideEffectUnknown, nil
}

func (s *MySQLStore) CompleteWeWorkCallbackSideEffect(ctx context.Context, execution dashboard.WeWorkCallbackExecution, actionKey, payloadHash string) error {
	if s == nil || s.db == nil {
		return errors.New("wework callback side effect store is not configured")
	}
	if err := validateWeWorkCallbackSideEffectExecution(execution, actionKey, payloadHash); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer rollbackQuietly(tx)
	if err := lockValidWeWorkCallbackExecution(ctx, tx, execution); err != nil {
		return err
	}
	var status, storedHash string
	if err := tx.QueryRowContext(ctx, `SELECT status,payload_hash FROM mochat_go_wework_callback_side_effects
		WHERE tenant_id=? AND corp_id=? AND event_key=? AND action_key=? FOR UPDATE`, execution.TenantID, execution.CorpID, execution.EventKey, actionKey).Scan(&status, &storedHash); err != nil {
		return err
	}
	if storedHash != payloadHash {
		return errors.New("wework callback side effect completion lost durable ownership")
	}
	if status == dashboard.WeWorkCallbackSideEffectSent {
		return tx.Commit()
	}
	if status != dashboard.WeWorkCallbackSideEffectUnknown {
		return errors.New("wework callback side effect completion lost durable ownership")
	}
	result, err := tx.ExecContext(ctx, `UPDATE mochat_go_wework_callback_side_effects
		SET status='sent',version=version+1,sent_at=UTC_TIMESTAMP(6),unknown_at=NULL,reconcile_after=NULL,updated_at=UTC_TIMESTAMP(6)
		WHERE tenant_id=? AND corp_id=? AND event_key=? AND action_key=? AND payload_hash=? AND status='unknown'`, execution.TenantID, execution.CorpID, execution.EventKey, actionKey, payloadHash)
	if err != nil {
		return err
	}
	if affected, affectedErr := result.RowsAffected(); affectedErr != nil || affected != 1 {
		if affectedErr != nil {
			return affectedErr
		}
		return errors.New("wework callback side effect completion lost durable ownership")
	}
	return tx.Commit()
}

func lockValidWeWorkCallbackExecution(ctx context.Context, tx *sql.Tx, execution dashboard.WeWorkCallbackExecution) error {
	var marker int
	err := tx.QueryRowContext(ctx, `SELECT 1 FROM mochat_go_wework_callback_inbox
		WHERE tenant_id=? AND corp_id=? AND event_key=? AND status='processing' AND lease_token=? AND lease_fence=? AND lease_expires_at>UTC_TIMESTAMP(6)
		FOR UPDATE`, execution.TenantID, execution.CorpID, execution.EventKey, execution.LeaseToken, execution.LeaseFence).Scan(&marker)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.ErrWeWorkCallbackLeaseLost
	}
	return err
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

func validateWeWorkCallbackSideEffectExecution(execution dashboard.WeWorkCallbackExecution, actionKey, payloadHash string) error {
	if err := validateWeWorkCallbackSideEffectIdentity(execution.TenantID, execution.CorpID, execution.EventKey, actionKey, payloadHash); err != nil {
		return err
	}
	if strings.TrimSpace(execution.LeaseToken) == "" || execution.LeaseFence == 0 {
		return errors.New("invalid wework callback side effect lease")
	}
	return nil
}
