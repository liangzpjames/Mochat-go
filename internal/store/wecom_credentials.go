package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"

	"jiyi/mochat-go/internal/dashboard"
	"jiyi/mochat-go/internal/wecomcredentials"
)

type corpCredentialRecord struct {
	ID             int
	TenantID       int
	WXCorpID       string
	EmployeeSecret string
	ContactSecret  string
	CallbackToken  string
	EncodingAESKey string
	ChatSecret     string
	Ciphertext     string
	KeyID          string
}

type agentCredentialRecord struct {
	ID         int
	CorpID     int
	TenantID   int
	WXCorpID   string
	WXAgentID  string
	WXSecret   string
	Ciphertext string
	KeyID      string
}

type corpCredentialStorage struct {
	EmployeeSecret string
	ContactSecret  string
	CallbackToken  string
	EncodingAESKey string
	ChatSecret     string
	Ciphertext     string
	KeyID          string
}

type agentCredentialStorage struct {
	WXSecret   string
	Ciphertext string
	KeyID      string
}

type queryRower interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

const authoritativeWeWorkCallbackCorpSelect = `SELECT c.id,c.tenant_id,COALESCE(c.wx_corpid,''),
       COALESCE(c.wecom_credentials_ciphertext,''),COALESCE(c.wecom_credentials_key_id,'')
FROM mc_corp c
INNER JOIN mc_tenant t
        ON t.id=c.tenant_id
       AND t.status=1
       AND t.deleted_at IS NULL
INNER JOIN mochat_go_tenant_corp_bindings b
        ON b.tenant_id=c.tenant_id
       AND b.corp_id=c.id
       AND b.status=2
       AND b.verified_at IS NOT NULL
	   AND b.verified_wx_corpid<>''
       AND b.verified_wx_corpid=c.wx_corpid`

func (s *MySQLStore) loadAuthoritativeWeWorkCallbackCorpByID(ctx context.Context, corpID int) (corpCredentialRecord, bool, error) {
	row := s.db.QueryRowContext(ctx, authoritativeWeWorkCallbackCorpSelect+`
WHERE c.id=? AND c.deleted_at IS NULL`, corpID)
	return scanCorpCredentialRecord(row)
}

func (s *MySQLStore) loadAuthoritativeWeWorkCallbackCorpByWXID(ctx context.Context, wxCorpID string) (corpCredentialRecord, bool, error) {
	rows, err := s.db.QueryContext(ctx, authoritativeWeWorkCallbackCorpSelect+`
WHERE c.wx_corpid=? AND c.deleted_at IS NULL
ORDER BY c.id ASC
LIMIT 2`, strings.TrimSpace(wxCorpID))
	if err != nil {
		return corpCredentialRecord{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return corpCredentialRecord{}, false, err
		}
		return corpCredentialRecord{}, false, nil
	}
	var item corpCredentialRecord
	if err := rows.Scan(&item.ID, &item.TenantID, &item.WXCorpID, &item.Ciphertext, &item.KeyID); err != nil {
		return corpCredentialRecord{}, false, err
	}
	if rows.Next() {
		return corpCredentialRecord{}, false, errors.New("multiple active verified corp bindings match callback receiver")
	}
	if err := rows.Err(); err != nil {
		return corpCredentialRecord{}, false, err
	}
	return item, true, nil
}

func (s *MySQLStore) loadCorpCredentialByID(ctx context.Context, queryer queryRower, corpID int, forUpdate bool) (corpCredentialRecord, bool, error) {
	suffix := ""
	if forUpdate {
		suffix = " FOR UPDATE"
	}
	row := queryer.QueryRowContext(ctx, `
		SELECT id, COALESCE(tenant_id, 0), COALESCE(wx_corpid, ''),
		       COALESCE(wecom_credentials_ciphertext, ''), COALESCE(wecom_credentials_key_id, '')
		FROM mc_corp
		WHERE id = ? AND deleted_at IS NULL`+suffix, corpID)
	return scanCorpCredentialRecord(row)
}

func (s *MySQLStore) loadCorpCredentialByWXCorpID(ctx context.Context, wxCorpID string) (corpCredentialRecord, bool, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, COALESCE(tenant_id, 0), COALESCE(wx_corpid, ''),
		       COALESCE(wecom_credentials_ciphertext, ''), COALESCE(wecom_credentials_key_id, '')
		FROM mc_corp
		WHERE wx_corpid = ? AND deleted_at IS NULL
		LIMIT 1
	`, strings.TrimSpace(wxCorpID))
	return scanCorpCredentialRecord(row)
}

func scanCorpCredentialRecord(scanner rowScanner) (corpCredentialRecord, bool, error) {
	var item corpCredentialRecord
	if err := scanner.Scan(&item.ID, &item.TenantID, &item.WXCorpID, &item.Ciphertext, &item.KeyID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return corpCredentialRecord{}, false, nil
		}
		return corpCredentialRecord{}, false, err
	}
	return item, true, nil
}

// decodeCorpCredential only resolves the post-cutover encrypted credential copy.
func (s *MySQLStore) decodeCorpCredential(item corpCredentialRecord) (wecomcredentials.CorpCredential, error) {
	if strings.TrimSpace(item.Ciphertext) == "" {
		return wecomcredentials.CorpCredential{}, errors.New("encrypted WeCom credential is unavailable")
	}
	if s.weComCredentialCipher == nil {
		return wecomcredentials.CorpCredential{}, errors.New("WeCom credential encryption manager is not configured")
	}
	return s.weComCredentialCipher.DecryptCorp(item.TenantID, item.WXCorpID, item.KeyID, item.Ciphertext)
}

func (s *MySQLStore) encodeCorpCredential(tenantID int, wxCorpID string, credential wecomcredentials.CorpCredential) (corpCredentialStorage, error) {
	if s.weComCredentialCipher != nil && s.weComCredentialCipher.ConfigStatus().EncryptionConfigured {
		ciphertext, keyID, err := s.weComCredentialCipher.EncryptCorp(tenantID, wxCorpID, credential)
		if err != nil {
			return corpCredentialStorage{}, err
		}
		return corpCredentialStorage{Ciphertext: ciphertext, KeyID: keyID}, nil
	}
	return corpCredentialStorage{}, errors.New("WeCom credential encryption manager is not configured")
}

func (s *MySQLStore) loadAgentCredentialByID(ctx context.Context, queryer queryRower, agentID int, forUpdate bool) (agentCredentialRecord, bool, error) {
	suffix := ""
	if forUpdate {
		suffix = " FOR UPDATE"
	}
	row := queryer.QueryRowContext(ctx, `
		SELECT a.id, a.corp_id, COALESCE(c.tenant_id, 0), COALESCE(c.wx_corpid, ''),
		       COALESCE(a.wx_agent_id, ''),
		       COALESCE(a.wecom_credentials_ciphertext, ''), COALESCE(a.wecom_credentials_key_id, '')
		FROM mc_work_agent a
		JOIN mc_corp c ON c.id = a.corp_id AND c.deleted_at IS NULL
		WHERE a.id = ? AND a.close = 0 AND a.deleted_at IS NULL`+suffix, agentID)
	var item agentCredentialRecord
	if err := row.Scan(&item.ID, &item.CorpID, &item.TenantID, &item.WXCorpID, &item.WXAgentID, &item.Ciphertext, &item.KeyID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return agentCredentialRecord{}, false, nil
		}
		return agentCredentialRecord{}, false, err
	}
	return item, true, nil
}

func (s *MySQLStore) decodeAgentCredential(item agentCredentialRecord) (wecomcredentials.AgentCredential, error) {
	if strings.TrimSpace(item.Ciphertext) == "" {
		return wecomcredentials.AgentCredential{}, errors.New("encrypted WeCom agent credential is unavailable")
	}
	if s.weComCredentialCipher == nil {
		return wecomcredentials.AgentCredential{}, errors.New("WeCom credential encryption manager is not configured")
	}
	return s.weComCredentialCipher.DecryptAgent(item.CorpID, item.WXAgentID, item.KeyID, item.Ciphertext)
}

func (s *MySQLStore) encodeAgentCredential(corpID int, wxAgentID string, credential wecomcredentials.AgentCredential) (agentCredentialStorage, error) {
	if s.weComCredentialCipher != nil && s.weComCredentialCipher.ConfigStatus().EncryptionConfigured {
		ciphertext, keyID, err := s.weComCredentialCipher.EncryptAgent(corpID, wxAgentID, credential)
		if err != nil {
			return agentCredentialStorage{}, err
		}
		return agentCredentialStorage{Ciphertext: ciphertext, KeyID: keyID}, nil
	}
	return agentCredentialStorage{}, errors.New("WeCom credential encryption manager is not configured")
}

func (s *MySQLStore) SaaSWeComCredentialProtection(ctx context.Context) (dashboard.SaaSWeComCredentialProtectionStatus, error) {
	status := dashboard.SaaSWeComCredentialProtectionStatus{ActiveKeyID: "primary", UnavailableKeyIDs: []string{}}
	if s.weComCredentialCipher != nil {
		config := s.weComCredentialCipher.ConfigStatus()
		status.EncryptionConfigured = config.EncryptionConfigured
		status.RequireEncryption = config.RequireEncryption
		status.DedicatedConfigured = config.DedicatedConfigured
		status.ActiveKeyID = config.ActiveKeyID
		status.KeyCount = config.KeyCount
	}
	unavailable := map[string]struct{}{}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, COALESCE(tenant_id, 0), COALESCE(wx_corpid, ''),
		       COALESCE(wecom_credentials_ciphertext, ''), COALESCE(wecom_credentials_key_id, '')
		FROM mc_corp
		WHERE deleted_at IS NULL
		ORDER BY id ASC
	`)
	if err != nil {
		return dashboard.SaaSWeComCredentialProtectionStatus{}, err
	}
	for rows.Next() {
		item, _, scanErr := scanCorpCredentialRecord(rows)
		if scanErr != nil {
			rows.Close()
			return dashboard.SaaSWeComCredentialProtectionStatus{}, scanErr
		}
		if !corpCredentialConfigured(item) {
			continue
		}
		status.ConfiguredCredentialCount++
		status.CorpCredentialCount++
		updateWeComProtectionStatus(&status, unavailable, item.Ciphertext, item.KeyID, corpLegacyPlaintextPresent(item), func() error {
			_, decryptErr := s.decodeCorpCredential(item)
			return decryptErr
		})
	}
	if err := rows.Close(); err != nil {
		return dashboard.SaaSWeComCredentialProtectionStatus{}, err
	}
	if err := rows.Err(); err != nil {
		return dashboard.SaaSWeComCredentialProtectionStatus{}, err
	}

	agentRows, err := s.db.QueryContext(ctx, `
		SELECT a.id, a.corp_id, COALESCE(c.tenant_id, 0), COALESCE(c.wx_corpid, ''),
		       COALESCE(a.wx_agent_id, ''),
		       COALESCE(a.wecom_credentials_ciphertext, ''), COALESCE(a.wecom_credentials_key_id, '')
		FROM mc_work_agent a
		JOIN mc_corp c ON c.id = a.corp_id AND c.deleted_at IS NULL
		WHERE a.deleted_at IS NULL
		ORDER BY a.id ASC
	`)
	if err != nil {
		return dashboard.SaaSWeComCredentialProtectionStatus{}, err
	}
	for agentRows.Next() {
		var item agentCredentialRecord
		if err := agentRows.Scan(&item.ID, &item.CorpID, &item.TenantID, &item.WXCorpID, &item.WXAgentID, &item.Ciphertext, &item.KeyID); err != nil {
			agentRows.Close()
			return dashboard.SaaSWeComCredentialProtectionStatus{}, err
		}
		if !agentCredentialConfigured(item) {
			continue
		}
		status.ConfiguredCredentialCount++
		status.AgentCredentialCount++
		updateWeComProtectionStatus(&status, unavailable, item.Ciphertext, item.KeyID, strings.TrimSpace(item.WXSecret) != "", func() error {
			_, decryptErr := s.decodeAgentCredential(item)
			return decryptErr
		})
	}
	if err := agentRows.Close(); err != nil {
		return dashboard.SaaSWeComCredentialProtectionStatus{}, err
	}
	if err := agentRows.Err(); err != nil {
		return dashboard.SaaSWeComCredentialProtectionStatus{}, err
	}

	for keyID := range unavailable {
		status.UnavailableKeyIDs = append(status.UnavailableKeyIDs, keyID)
	}
	sort.Strings(status.UnavailableKeyIDs)
	status.UnavailableKeyCount = len(status.UnavailableKeyIDs)
	status.Healthy = status.UnavailableKeyCount == 0 && status.DecryptFailureCount == 0 && status.RotationRequiredCount == 0
	if status.ConfiguredCredentialCount > 0 && !status.EncryptionConfigured {
		status.Healthy = false
	}
	return status, nil
}

func updateWeComProtectionStatus(status *dashboard.SaaSWeComCredentialProtectionStatus, unavailable map[string]struct{}, ciphertext string, keyID string, plaintextPresent bool, decrypt func() error) {
	ciphertext = strings.TrimSpace(ciphertext)
	keyID = strings.TrimSpace(keyID)
	if ciphertext == "" {
		status.LegacyPlaintextCount++
		status.RotationRequiredCount++
		return
	}
	status.EncryptedCredentialCount++
	if keyID == status.ActiveKeyID && !plaintextPresent {
		status.ActiveKeyCredentialCount++
	} else {
		status.RotationRequiredCount++
	}
	if keyID == "" || status.KeyCount == 0 {
		unavailableKeyID := keyID
		if unavailableKeyID == "" {
			unavailableKeyID = "(missing)"
		}
		unavailable[unavailableKeyID] = struct{}{}
		return
	}
	if decrypt != nil {
		if err := decrypt(); err != nil {
			if strings.Contains(err.Error(), "is unavailable") {
				unavailable[keyID] = struct{}{}
			} else {
				status.DecryptFailureCount++
			}
		}
	}
}

func (s *MySQLStore) CheckSaaSWeComCredentialProtection(ctx context.Context) error {
	status, err := s.SaaSWeComCredentialProtection(ctx)
	if err != nil {
		return err
	}
	if status.Healthy {
		return nil
	}
	return fmt.Errorf("WeCom credential protection unhealthy: configured=%d encrypted=%d legacy=%d rotation_required=%d unavailable_keys=%d decrypt_failures=%d",
		status.ConfiguredCredentialCount, status.EncryptedCredentialCount, status.LegacyPlaintextCount,
		status.RotationRequiredCount, status.UnavailableKeyCount, status.DecryptFailureCount)
}

func (s *MySQLStore) RotateSaaSWeComCredentials(ctx context.Context, tenantID int, limit int) (dashboard.SaaSWeComCredentialRotationResult, error) {
	result := dashboard.SaaSWeComCredentialRotationResult{TenantID: tenantID, Limit: limit}
	if tenantID < 0 {
		return result, errors.New("tenant id must be non-negative")
	}
	if limit <= 0 || limit > 1000 {
		return result, errors.New("limit must be between 1 and 1000")
	}
	if s.weComCredentialCipher == nil || !s.weComCredentialCipher.ConfigStatus().EncryptionConfigured {
		return result, errors.New("WeCom credential encryption key is not configured")
	}
	result.ActiveKeyID = s.weComCredentialCipher.ConfigStatus().ActiveKeyID

	query := `
		SELECT resource_type, resource_id
		FROM (
			SELECT 'corp' AS resource_type, c.id AS resource_id, c.id AS sort_id
			FROM mc_corp c
			WHERE c.deleted_at IS NULL
			  AND (? = 0 OR c.tenant_id = ?)
			  AND COALESCE(c.wecom_credentials_ciphertext, '') <> ''
			  AND c.wecom_credentials_key_id <> ?
			UNION ALL
			SELECT 'agent' AS resource_type, a.id AS resource_id, a.id AS sort_id
			FROM mc_work_agent a
			JOIN mc_corp c ON c.id = a.corp_id AND c.deleted_at IS NULL
			WHERE a.deleted_at IS NULL
			  AND (? = 0 OR c.tenant_id = ?)
			  AND COALESCE(a.wecom_credentials_ciphertext, '') <> ''
			  AND a.wecom_credentials_key_id <> ?
		) candidates
		ORDER BY resource_type ASC, sort_id ASC
		LIMIT ?
	`
	rows, err := s.db.QueryContext(ctx, query, tenantID, tenantID, result.ActiveKeyID, tenantID, tenantID, result.ActiveKeyID, limit)
	if err != nil {
		return result, err
	}
	type candidate struct {
		resource string
		id       int
	}
	candidates := make([]candidate, 0, limit)
	for rows.Next() {
		var item candidate
		if err := rows.Scan(&item.resource, &item.id); err != nil {
			rows.Close()
			return result, err
		}
		candidates = append(candidates, item)
	}
	if err := rows.Close(); err != nil {
		return result, err
	}
	if err := rows.Err(); err != nil {
		return result, err
	}
	result.ScannedCount = len(candidates)
	for _, item := range candidates {
		var rotated, legacy bool
		if item.resource == "corp" {
			rotated, legacy, err = s.rotateCorpCredential(ctx, item.id, tenantID)
		} else {
			rotated, legacy, err = s.rotateAgentCredential(ctx, item.id, tenantID)
		}
		if err != nil {
			return result, err
		}
		if !rotated {
			continue
		}
		result.RotatedCount++
		if item.resource == "corp" {
			result.CorpRotatedCount++
		} else {
			result.AgentRotatedCount++
		}
		if legacy {
			result.LegacyCount++
		} else {
			result.ReencryptedCount++
		}
	}
	return result, nil
}

func (s *MySQLStore) rotateCorpCredential(ctx context.Context, corpID int, tenantID int) (bool, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, false, err
	}
	defer rollbackQuietly(tx)
	item, found, err := s.loadCorpCredentialByID(ctx, tx, corpID, true)
	if err != nil || !found {
		return false, false, err
	}
	if tenantID > 0 && item.TenantID != tenantID {
		return false, false, nil
	}
	legacy := strings.TrimSpace(item.Ciphertext) == ""
	if !corpCredentialNeedsRotation(item, s.weComCredentialCipher.ConfigStatus().ActiveKeyID) {
		return false, legacy, nil
	}
	credential, err := s.decodeCorpCredential(item)
	if err != nil {
		return false, legacy, err
	}
	storage, err := s.encodeCorpCredential(item.TenantID, item.WXCorpID, credential)
	if err != nil {
		return false, legacy, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mc_corp
		SET wecom_credentials_ciphertext = ?, wecom_credentials_key_id = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`, storage.Ciphertext, storage.KeyID, item.ID); err != nil {
		return false, legacy, err
	}
	if err := tx.Commit(); err != nil {
		return false, legacy, err
	}
	return true, legacy, nil
}

func (s *MySQLStore) rotateAgentCredential(ctx context.Context, agentID int, tenantID int) (bool, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, false, err
	}
	defer rollbackQuietly(tx)
	item, found, err := s.loadAgentCredentialByID(ctx, tx, agentID, true)
	if err != nil || !found {
		return false, false, err
	}
	if tenantID > 0 && item.TenantID != tenantID {
		return false, false, nil
	}
	legacy := strings.TrimSpace(item.Ciphertext) == ""
	if !agentCredentialNeedsRotation(item, s.weComCredentialCipher.ConfigStatus().ActiveKeyID) {
		return false, legacy, nil
	}
	credential, err := s.decodeAgentCredential(item)
	if err != nil {
		return false, legacy, err
	}
	storage, err := s.encodeAgentCredential(item.CorpID, item.WXAgentID, credential)
	if err != nil {
		return false, legacy, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mc_work_agent
		SET wecom_credentials_ciphertext = ?, wecom_credentials_key_id = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`, storage.Ciphertext, storage.KeyID, item.ID); err != nil {
		return false, legacy, err
	}
	if err := tx.Commit(); err != nil {
		return false, legacy, err
	}
	return true, legacy, nil
}

func corpCredentialConfigured(item corpCredentialRecord) bool {
	return strings.TrimSpace(item.Ciphertext) != ""
}

func corpLegacyPlaintextPresent(item corpCredentialRecord) bool {
	return strings.TrimSpace(item.EmployeeSecret) != "" || strings.TrimSpace(item.ContactSecret) != "" ||
		strings.TrimSpace(item.CallbackToken) != "" || strings.TrimSpace(item.EncodingAESKey) != "" || strings.TrimSpace(item.ChatSecret) != ""
}

func corpCredentialNeedsRotation(item corpCredentialRecord, activeKeyID string) bool {
	return strings.TrimSpace(item.Ciphertext) != "" && strings.TrimSpace(item.KeyID) != strings.TrimSpace(activeKeyID)
}

func agentCredentialConfigured(item agentCredentialRecord) bool {
	return strings.TrimSpace(item.Ciphertext) != ""
}

func agentCredentialNeedsRotation(item agentCredentialRecord, activeKeyID string) bool {
	return strings.TrimSpace(item.Ciphertext) != "" && strings.TrimSpace(item.KeyID) != strings.TrimSpace(activeKeyID)
}
