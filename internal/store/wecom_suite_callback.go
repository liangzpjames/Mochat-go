package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"jiyi/mochat-go/internal/archivefixture"
	"jiyi/mochat-go/internal/dashboard"
	"jiyi/mochat-go/internal/wecomcredentials"
)

func (s *MySQLStore) ClaimSuiteCallback(ctx context.Context, suiteID, digest, eventType string, receivedAt time.Time) (bool, error) {
	if s == nil || s.db == nil || strings.TrimSpace(suiteID) == "" || len(strings.TrimSpace(digest)) != 64 || strings.TrimSpace(eventType) == "" {
		return false, errors.New("WeCom suite callback claim is invalid")
	}
	result, err := s.db.ExecContext(ctx, `
		INSERT IGNORE INTO mochat_go_wecom_suite_callback_events
		(suite_id,event_digest,event_type,status,attempt,received_at,created_at,updated_at)
		VALUES (?,?,?,'processing',1,?,NOW(6),NOW(6))
	`, strings.TrimSpace(suiteID), strings.TrimSpace(digest), strings.TrimSpace(eventType), receivedAt.UTC())
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	if err != nil || rows == 1 {
		return rows == 1, err
	}
	result, err = s.db.ExecContext(ctx, `
		UPDATE mochat_go_wecom_suite_callback_events
		SET event_type=?,status='processing',attempt=attempt+1,received_at=?,completed_at=NULL,updated_at=NOW(6)
		WHERE suite_id=? AND event_digest=?
		  AND (status='failed' OR (status='processing' AND updated_at<DATE_SUB(?,INTERVAL 5 MINUTE)))
	`, strings.TrimSpace(eventType), receivedAt.UTC(), strings.TrimSpace(suiteID), strings.TrimSpace(digest), receivedAt.UTC())
	if err != nil {
		return false, err
	}
	rows, err = result.RowsAffected()
	return rows == 1, err
}

func (s *MySQLStore) CompleteSuiteCallback(ctx context.Context, suiteID, digest string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE mochat_go_wecom_suite_callback_events SET status='completed',completed_at=NOW(6),updated_at=NOW(6) WHERE suite_id=? AND event_digest=? AND status='processing'`, strings.TrimSpace(suiteID), strings.TrimSpace(digest))
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows != 1 {
		return errors.New("WeCom suite callback completion lease was lost")
	}
	return nil
}

func (s *MySQLStore) FailSuiteCallback(ctx context.Context, suiteID, digest string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE mochat_go_wecom_suite_callback_events SET status='failed',updated_at=NOW(6) WHERE suite_id=? AND event_digest=? AND status='processing'`, strings.TrimSpace(suiteID), strings.TrimSpace(digest))
	return err
}

func (s *MySQLStore) SaveSuiteTicket(ctx context.Context, suiteID, ticket string, receivedAt time.Time) error {
	if s == nil || s.db == nil || s.weComCredentialCipher == nil {
		return errors.New("WeCom suite ticket store is unavailable")
	}
	ciphertext, keyID, err := s.weComCredentialCipher.EncryptSuiteTicket(suiteID, ticket)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO mochat_go_wecom_suite_tickets (suite_id,ticket_ciphertext,ticket_key_id,received_at,created_at,updated_at)
		VALUES (?,?,?,?,NOW(6),NOW(6))
		ON DUPLICATE KEY UPDATE ticket_ciphertext=VALUES(ticket_ciphertext),ticket_key_id=VALUES(ticket_key_id),received_at=VALUES(received_at),updated_at=NOW(6)
	`, strings.TrimSpace(suiteID), ciphertext, keyID, receivedAt.UTC())
	return err
}

func (s *MySQLStore) LoadSuiteTicket(ctx context.Context, suiteID string) (string, error) {
	if s == nil || s.db == nil || s.weComCredentialCipher == nil {
		return "", errors.New("WeCom suite ticket store is unavailable")
	}
	var ciphertext, keyID string
	if err := s.db.QueryRowContext(ctx, `SELECT ticket_ciphertext,ticket_key_id FROM mochat_go_wecom_suite_tickets WHERE suite_id=? LIMIT 1`, strings.TrimSpace(suiteID)).Scan(&ciphertext, &keyID); err != nil {
		return "", err
	}
	return s.weComCredentialCipher.DecryptSuiteTicket(suiteID, keyID, ciphertext)
}

func (s *MySQLStore) SaveSuiteAuthorization(ctx context.Context, suiteID string, authorization archivefixture.SuiteAuthorization) error {
	if s == nil || s.db == nil || s.weComCredentialCipher == nil || authorization.TenantID <= 0 || strings.TrimSpace(authorization.CorpID) == "" || strings.TrimSpace(authorization.PermanentCode) == "" {
		return errors.New("WeCom suite authorization is invalid")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var integrationID, status, ciphertext, keyID string
	err = tx.QueryRowContext(ctx, `
		SELECT integration.id,integration.status,COALESCE(integration.credential_ciphertext,''),COALESCE(integration.credential_key_id,'')
		FROM mochat_go_wecom_integrations integration
		INNER JOIN mc_corp corp ON corp.tenant_id=integration.tenant_id AND corp.id=integration.corp_id AND corp.deleted_at IS NULL
		INNER JOIN mochat_go_tenant_corp_bindings binding ON binding.tenant_id=integration.tenant_id AND binding.corp_id=integration.corp_id
		WHERE integration.tenant_id=? AND integration.mode='third_party_delegated' AND integration.slot='current'
		  AND integration.provider_app_id=? AND corp.wx_corpid=? AND binding.wecom_integration_mode=integration.mode
		LIMIT 1 FOR UPDATE
	`, authorization.TenantID, strings.TrimSpace(suiteID), strings.TrimSpace(authorization.CorpID)).Scan(&integrationID, &status, &ciphertext, &keyID)
	if err != nil {
		return err
	}
	newCiphertext, newKeyID, err := rotateSuiteAuthorizationCredential(s.weComCredentialCipher, authorization.TenantID, integrationID, ciphertext, keyID, suiteID, authorization.PermanentCode)
	if err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE mochat_go_wecom_integrations SET status=IF(status='active','active','pending_verification'),verified_wx_corpid=?,credential_ciphertext=?,credential_key_id=?,credential_hint='permanent_code',last_error_code='',last_error_at=NULL,version=version+1,updated_at=NOW(6) WHERE id=? AND tenant_id=? AND mode='third_party_delegated' AND slot='current'`, strings.TrimSpace(authorization.CorpID), newCiphertext, newKeyID, integrationID, authorization.TenantID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows != 1 {
		return errors.New("WeCom suite authorization target changed")
	}
	before, _ := json.Marshal(map[string]any{"status": status, "credentialConfigured": ciphertext != "" && keyID != ""})
	after, _ := json.Marshal(map[string]any{"status": "pending_verification", "credentialConfigured": true, "providerAppId": strings.TrimSpace(suiteID), "verifiedWxCorpId": strings.TrimSpace(authorization.CorpID)})
	if _, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID: authorization.TenantID, Action: "wecom.integration.suite_callback.authorize",
		TargetType: "wecom_integration", TargetID: integrationID, TargetName: "suite callback",
		BeforeJSON: string(before), AfterJSON: string(after), Remark: "encrypted callback authorization",
	}); err != nil {
		return err
	}
	return tx.Commit()
}

// rotateSuiteAuthorizationCredential treats a successful create_auth callback as
// the authoritative replacement for the delegated credential. The previous
// ciphertext is deliberately not decrypted: it contains no self-built fields
// that may be retained, and requiring a retired key would make reauthorization
// unable to recover an integration after a legitimate key-ring retirement.
func rotateSuiteAuthorizationCredential(manager *wecomcredentials.Manager, tenantID int, integrationID, _ string, _ string, suiteID, permanentCode string) (string, string, error) {
	credential := wecomcredentials.AuthorizationCredential{
		Mode:          "third_party_delegated",
		ProviderAppID: strings.TrimSpace(suiteID),
		PermanentCode: strings.TrimSpace(permanentCode),
	}
	return manager.EncryptAuthorization(tenantID, integrationID, credential)
}
