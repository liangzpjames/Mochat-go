package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"jiyi/mochat-go/internal/archivebridge"
)

func resolveSeedBinding(ctx context.Context, db bindingQuery, tenantID int64, requestedMode string) (archivebridge.Binding, error) {
	var binding archivebridge.Binding
	err := db.QueryRowContext(ctx, `
		SELECT integration.tenant_id,integration.corp_id,
		       COALESCE(NULLIF(integration.verified_wx_corpid,''),NULLIF(binding.verified_wx_corpid,''),NULLIF(corp.wx_corpid,''),''),
		       binding.wecom_integration_mode
		FROM mochat_go_wecom_integrations integration
		INNER JOIN mochat_go_tenant_corp_bindings binding ON binding.tenant_id=integration.tenant_id AND binding.corp_id=integration.corp_id
		INNER JOIN mc_tenant tenant ON tenant.id=integration.tenant_id AND tenant.status=1 AND tenant.deleted_at IS NULL
		INNER JOIN mc_corp corp ON corp.id=integration.corp_id AND corp.tenant_id=integration.tenant_id AND corp.deleted_at IS NULL
		WHERE integration.tenant_id=? AND integration.slot='current'
		  AND integration.mode=binding.wecom_integration_mode
		LIMIT 1
	`, tenantID).Scan(&binding.TenantID, &binding.CorpID, &binding.WXCorpID, &binding.IntegrationMode)
	if errors.Is(err, sql.ErrNoRows) {
		return archivebridge.Binding{}, errors.New("tenant has no current enterprise integration")
	}
	if err != nil {
		return archivebridge.Binding{}, fmt.Errorf("resolve tenant enterprise integration: %w", err)
	}
	if binding.IntegrationMode != requestedMode {
		return archivebridge.Binding{}, fmt.Errorf("tenant authoritative mode is %s, not %s", binding.IntegrationMode, requestedMode)
	}
	if !fixtureWXCorpIDAllowed(binding.TenantID, binding.WXCorpID) {
		return archivebridge.Binding{}, errors.New("simulation refuses a production-looking WeCom enterprise identity")
	}
	if strings.TrimSpace(binding.WXCorpID) == "" {
		binding.WXCorpID = localFixtureWXCorpID(binding.TenantID)
	}
	return binding, nil
}

func activateFixtureBinding(ctx context.Context, db *sql.DB, binding archivebridge.Binding, dataset string) error {
	dataset = strings.TrimSpace(dataset)
	if db == nil || !strings.HasPrefix(dataset, archivebridge.FixtureDatasetPrefix) || !fixtureWXCorpIDAllowed(binding.TenantID, binding.WXCorpID) {
		return errors.New("fixture integration activation scope is invalid")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var integrationID, mode, status, verifiedWXCorpID, scopeRaw, providerAppID, credentialCiphertext, verificationLevel string
	var bindingStatus, corpChatStatus int
	var bindingWXCorpID, corpWXCorpID string
	var version uint64
	err = tx.QueryRowContext(ctx, `
		SELECT integration.id,integration.mode,integration.status,COALESCE(integration.verified_wx_corpid,''),
		       integration.scope_json,COALESCE(integration.provider_app_id,''),COALESCE(integration.credential_ciphertext,''),
		       integration.version,COALESCE(integration.verification_level,''),binding.status,
		       COALESCE(binding.verified_wx_corpid,''),COALESCE(corp.wx_corpid,''),corp.chat_status
		FROM mochat_go_wecom_integrations integration
		INNER JOIN mochat_go_tenant_corp_bindings binding ON binding.tenant_id=integration.tenant_id AND binding.corp_id=integration.corp_id
		INNER JOIN mc_corp corp ON corp.tenant_id=integration.tenant_id AND corp.id=integration.corp_id AND corp.deleted_at IS NULL
		WHERE integration.tenant_id=? AND integration.corp_id=? AND integration.slot='current'
		  AND integration.mode=? AND binding.wecom_integration_mode=integration.mode
		LIMIT 1 FOR UPDATE
	`, binding.TenantID, binding.CorpID, binding.IntegrationMode).Scan(
		&integrationID, &mode, &status, &verifiedWXCorpID, &scopeRaw, &providerAppID,
		&credentialCiphertext, &version, &verificationLevel, &bindingStatus,
		&bindingWXCorpID, &corpWXCorpID, &corpChatStatus,
	)
	if err != nil {
		return err
	}
	if mode == archivebridge.ModeThirdPartyDelegated && (providerAppID == "" || credentialCiphertext == "") {
		return errors.New("third-party fixture requires SaaS delegated application configuration first")
	}
	if verifiedWXCorpID != "" && verifiedWXCorpID != binding.WXCorpID {
		return errors.New("fixture integration enterprise identity changed during activation")
	}
	var scope []string
	if json.Unmarshal([]byte(scopeRaw), &scope) != nil {
		return errors.New("fixture integration capability scope is invalid")
	}
	scope = normalizedFixtureScope(scope)
	if err := ensureFixtureParticipantsTx(ctx, tx, binding.CorpID, dataset); err != nil {
		return err
	}
	if fixtureIntegrationAlreadyActive(status, verifiedWXCorpID, verificationLevel, scope, bindingStatus, bindingWXCorpID, corpWXCorpID, corpChatStatus) {
		return tx.Commit()
	}
	scope = normalizedFixtureScope(append(scope, "archive.read"))
	scopeJSON, _ := json.Marshal(scope)
	scopeDigestRaw := sha256.Sum256([]byte(strings.Join(scope, "\n")))
	scopeDigest := hex.EncodeToString(scopeDigestRaw[:])

	if _, err := tx.ExecContext(ctx, `UPDATE mc_corp SET wx_corpid=?,chat_status=1,updated_at=NOW() WHERE id=? AND tenant_id=?`, binding.WXCorpID, binding.CorpID, binding.TenantID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE mochat_go_tenant_corp_bindings SET status=2,verified_wx_corpid=?,verified_corp_name=CASE WHEN verified_corp_name='' THEN ? ELSE verified_corp_name END,verified_at=COALESCE(verified_at,NOW()),version=version+1,updated_at=NOW() WHERE tenant_id=? AND corp_id=? AND wecom_integration_mode=?`, binding.WXCorpID, dataset+" 本地验收企业", binding.TenantID, binding.CorpID, binding.IntegrationMode); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE mochat_go_wecom_integrations SET status='active',verified_wx_corpid=?,scope_json=?,scope_digest=?,missing_capabilities_json=JSON_ARRAY(),verification_level='local_contract',verified_at=COALESCE(verified_at,NOW(6)),activated_at=COALESCE(activated_at,NOW(6)),last_error_code='',last_error_at=NULL,last_audit_at=NOW(6),version=version+1,updated_at=NOW(6) WHERE id=? AND tenant_id=? AND corp_id=? AND mode=? AND slot='current' AND version=?`, binding.WXCorpID, string(scopeJSON), scopeDigest, integrationID, binding.TenantID, binding.CorpID, binding.IntegrationMode, version)
	if err != nil {
		return err
	}
	if rows, _ := result.RowsAffected(); rows != 1 {
		return errors.New("fixture integration activation was concurrently modified")
	}
	beforeJSON, _ := json.Marshal(map[string]any{"status": status, "verifiedWxCorpId": verifiedWXCorpID, "mode": mode})
	afterJSON, _ := json.Marshal(map[string]any{"status": "active", "verifiedWxCorpId": binding.WXCorpID, "mode": mode, "scope": scope, "verificationLevel": "local_contract", "dataset": dataset})
	if _, err := tx.ExecContext(ctx, `INSERT INTO mochat_go_saas_admin_operation_logs (tenant_id,actor_user_id,actor_tenant_id,action,target_type,target_id,target_name,before_json,after_json,remark,created_at) VALUES (?,0,0,'wecom.integration.fixture.activate','wecom_integration',?,? ,?,?,?,NOW())`, binding.TenantID, integrationID, dataset, string(beforeJSON), string(afterJSON), "仅用于本地企微契约验收；未调用真实企微"); err != nil {
		return err
	}
	return tx.Commit()
}

func ensureFixtureParticipantsTx(ctx context.Context, tx *sql.Tx, corpID int64, dataset string) error {
	staffWXID, externalWXID := fixtureParticipantIdentities(dataset)
	if tx == nil || corpID <= 0 || staffWXID == "" || externalWXID == "" {
		return errors.New("fixture participant scope is invalid")
	}
	staffName := strings.TrimSpace(dataset) + " 本地验收员工"
	var staffID int64
	var currentStaffName string
	var staffStatus, staffAuditStatus int
	var staffDeletedAt sql.NullTime
	err := tx.QueryRowContext(ctx, `SELECT id,name,status,audit_status,deleted_at FROM mc_work_employee WHERE corp_id=? AND wx_user_id=? ORDER BY id LIMIT 1 FOR UPDATE`, corpID, staffWXID).Scan(&staffID, &currentStaffName, &staffStatus, &staffAuditStatus, &staffDeletedAt)
	if errors.Is(err, sql.ErrNoRows) {
		_, err = tx.ExecContext(ctx, `INSERT INTO mc_work_employee (wx_user_id,corp_id,name,status,audit_status,created_at,updated_at) VALUES (?,?,?,1,1,NOW(),NOW())`, staffWXID, corpID, staffName)
	} else if err == nil && (currentStaffName != staffName || staffStatus != 1 || staffAuditStatus != 1 || staffDeletedAt.Valid) {
		_, err = tx.ExecContext(ctx, `UPDATE mc_work_employee SET name=?,status=1,audit_status=1,deleted_at=NULL,updated_at=NOW() WHERE id=? AND corp_id=? AND wx_user_id=?`, staffName, staffID, corpID, staffWXID)
	}
	if err != nil {
		return fmt.Errorf("prepare fixture employee: %w", err)
	}

	contactName := strings.TrimSpace(dataset) + " 本地验收客户"
	var contactID int64
	var currentContactName string
	var contactDeletedAt sql.NullTime
	err = tx.QueryRowContext(ctx, `SELECT id,name,deleted_at FROM mc_work_contact WHERE corp_id=? AND wx_external_userid=? ORDER BY id LIMIT 1 FOR UPDATE`, corpID, externalWXID).Scan(&contactID, &currentContactName, &contactDeletedAt)
	if errors.Is(err, sql.ErrNoRows) {
		_, err = tx.ExecContext(ctx, `INSERT INTO mc_work_contact (corp_id,wx_external_userid,name,created_at,updated_at) VALUES (?,?,?,NOW(),NOW())`, corpID, externalWXID, contactName)
	} else if err == nil && (currentContactName != contactName || contactDeletedAt.Valid) {
		_, err = tx.ExecContext(ctx, `UPDATE mc_work_contact SET name=?,deleted_at=NULL,updated_at=NOW() WHERE id=? AND corp_id=? AND wx_external_userid=?`, contactName, contactID, corpID, externalWXID)
	}
	if err != nil {
		return fmt.Errorf("prepare fixture contact: %w", err)
	}
	return nil
}

func fixtureParticipantIdentities(dataset string) (string, string) {
	dataset = strings.TrimSpace(dataset)
	if !strings.HasPrefix(dataset, archivebridge.FixtureDatasetPrefix) {
		return "", ""
	}
	return dataset + "-STAFF-01", dataset + "-EXTERNAL-01"
}

func fixtureIntegrationAlreadyActive(status, integrationWXCorpID, verificationLevel string, scope []string, bindingStatus int, bindingWXCorpID, corpWXCorpID string, corpChatStatus int) bool {
	if status != "active" || verificationLevel != "local_contract" || bindingStatus != 2 || corpChatStatus != 1 {
		return false
	}
	if integrationWXCorpID == "" || integrationWXCorpID != bindingWXCorpID || integrationWXCorpID != corpWXCorpID {
		return false
	}
	for _, capability := range scope {
		if capability == "archive.read" {
			return true
		}
	}
	return false
}

func localFixtureWXCorpID(tenantID int64) string {
	return fmt.Sprintf("wwMOCHATLOCALSIM%08d", tenantID)
}

func fixtureWXCorpIDAllowed(tenantID int64, value string) bool {
	value = strings.ToUpper(strings.TrimSpace(value))
	if tenantID <= 0 || value == "" {
		return tenantID > 0
	}
	return strings.HasPrefix(value, "WWSIM") || value == strings.ToUpper(localFixtureWXCorpID(tenantID))
}

func normalizedFixtureScope(scope []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(scope))
	for _, value := range scope {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
