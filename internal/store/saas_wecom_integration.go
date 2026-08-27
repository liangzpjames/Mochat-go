package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"jiyi/mochat-go/internal/dashboard"
	"jiyi/mochat-go/internal/dashboardadmin"
	"jiyi/mochat-go/internal/wecomcredentials"
)

const weComIntegrationColumns = `id,mode,slot,status,COALESCE(verified_wx_corpid,''),COALESCE(agent_id,''),COALESCE(provider_app_id,''),COALESCE(credential_ciphertext,''),COALESCE(credential_key_id,''),COALESCE(credential_hint,''),scope_json,COALESCE(scope_digest,''),missing_capabilities_json,generation,version,COALESCE(verification_level,''),verified_at,COALESCE(last_error_code,''),updated_at`

type weComBinding struct {
	CorpID   int
	WXCorpID string
	Mode     string
}

func (s *MySQLStore) WeComIntegration(ctx context.Context, actor dashboardadmin.Actor, tenantID int) (dashboardadmin.WeComIntegrationView, error) {
	if s == nil || s.db == nil {
		return dashboardadmin.WeComIntegrationView{}, dashboardadmin.ErrStoreUnavailable
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboardadmin.WeComIntegrationView{}, err
	}
	defer tx.Rollback()
	if err = lockSaaSActorPermissionTx(ctx, tx, actor.UserID, dashboard.SaaSAdminPermissionIntegrationsRead); err != nil {
		return dashboardadmin.WeComIntegrationView{}, err
	}
	binding, err := lockWeComBindingTx(ctx, tx, tenantID)
	if err != nil {
		return dashboardadmin.WeComIntegrationView{}, err
	}
	view, err := loadWeComIntegrationViewTx(ctx, tx, tenantID, binding.CorpID, false)
	if err != nil {
		return dashboardadmin.WeComIntegrationView{}, err
	}
	if err = validateWeComBindingMode(binding, view.Current); err != nil {
		return dashboardadmin.WeComIntegrationView{}, err
	}
	view.Candidate = nil
	if err = tx.Commit(); err != nil {
		return dashboardadmin.WeComIntegrationView{}, err
	}
	return view, nil
}

func (s *MySQLStore) SaveWeComIntegrationCurrent(ctx context.Context, actor dashboardadmin.Actor, tenantID int, input dashboardadmin.WeComIntegrationCandidateInput) (dashboardadmin.WeComIntegration, error) {
	if s == nil || s.db == nil || s.weComCredentialCipher == nil {
		return dashboardadmin.WeComIntegration{}, dashboardadmin.ErrStoreUnavailable
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboardadmin.WeComIntegration{}, err
	}
	defer tx.Rollback()
	if err = lockSaaSActorPermissionTx(ctx, tx, actor.UserID, dashboard.SaaSAdminPermissionIntegrationsManage); err != nil {
		return dashboardadmin.WeComIntegration{}, err
	}
	binding, err := lockWeComBindingTx(ctx, tx, tenantID)
	if err != nil {
		return dashboardadmin.WeComIntegration{}, err
	}
	current, err := loadWeComIntegrationBySlotTx(ctx, tx, tenantID, binding.CorpID, "current", true)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboardadmin.WeComIntegration{}, dashboardadmin.ErrTargetNotFound
	}
	if err != nil {
		return dashboardadmin.WeComIntegration{}, err
	}
	if err = validateWeComBindingMode(binding, &current); err != nil || binding.Mode != dashboardadmin.WeComIntegrationModeThirdPartyDelegated || input.Mode != binding.Mode {
		return dashboardadmin.WeComIntegration{}, dashboardadmin.ErrWeComModeImmutable
	}
	if input.Version != current.Version {
		return dashboardadmin.WeComIntegration{}, dashboardadmin.ErrVersionConflict
	}
	credential := wecomcredentials.AuthorizationCredential{Mode: current.Mode}
	if current.CredentialConfigured {
		credential, err = s.weComCredentialCipher.DecryptAuthorization(tenantID, current.ID, current.CredentialKeyID, current.CredentialCiphertext)
		if err != nil {
			return dashboardadmin.WeComIntegration{}, dashboardadmin.ErrWeComCredentialDecrypt
		}
	}
	if err = dashboardadmin.ValidateWeComIntegrationCandidate(input, current.CredentialConfigured); err != nil {
		return dashboardadmin.WeComIntegration{}, err
	}
	credential.Mode = current.Mode
	credential.ProviderAppID = input.ProviderAppID
	if input.PermanentCode != "" {
		credential.PermanentCode = input.PermanentCode
	}
	credential.EmployeeSecret, credential.ContactSecret, credential.AgentSecret, credential.ChatSecret = "", "", "", ""
	ciphertext, keyID, err := s.weComCredentialCipher.EncryptAuthorization(tenantID, current.ID, credential)
	if err != nil {
		return dashboardadmin.WeComIntegration{}, dashboardadmin.ErrStoreUnavailable
	}
	scopeJSON, _ := json.Marshal(input.Scope)
	result, err := tx.ExecContext(ctx, `UPDATE mochat_go_wecom_integrations SET status='pending_verification',provider_app_id=?,credential_ciphertext=?,credential_key_id=?,credential_hint=?,scope_json=?,scope_digest=?,missing_capabilities_json=JSON_ARRAY(),last_error_code='',last_error_at=NULL,version=version+1,updated_at=NOW(6) WHERE id=? AND tenant_id=? AND corp_id=? AND slot='current' AND mode='third_party_delegated' AND version=?`, input.ProviderAppID, ciphertext, keyID, weComCredentialHint(credential), string(scopeJSON), dashboardadmin.WeComScopeDigest(input.Scope), current.ID, tenantID, binding.CorpID, input.Version)
	if err != nil {
		return dashboardadmin.WeComIntegration{}, err
	}
	if rows, _ := result.RowsAffected(); rows != 1 {
		return dashboardadmin.WeComIntegration{}, dashboardadmin.ErrVersionConflict
	}
	after, err := loadWeComIntegrationBySlotTx(ctx, tx, tenantID, binding.CorpID, "current", false)
	if err != nil {
		return dashboardadmin.WeComIntegration{}, err
	}
	if err = insertWeComAuditTx(ctx, tx, actor.UserID, tenantID, "wecom.integration.delegated.rotate", after.ID, &current, &after); err != nil {
		return dashboardadmin.WeComIntegration{}, err
	}
	if err = tx.Commit(); err != nil {
		return dashboardadmin.WeComIntegration{}, err
	}
	return publicWeComIntegration(after), nil
}

func (s *MySQLStore) SaveWeComIntegrationCandidate(context.Context, dashboardadmin.Actor, int, dashboardadmin.WeComIntegrationCandidateInput) (dashboardadmin.WeComIntegration, error) {
	return dashboardadmin.WeComIntegration{}, dashboardadmin.ErrWeComModeImmutable
}

func (s *MySQLStore) WeComIntegrationVerificationCandidate(context.Context, dashboardadmin.Actor, int, uint64) (dashboardadmin.WeComVerificationCandidate, error) {
	return dashboardadmin.WeComVerificationCandidate{}, dashboardadmin.ErrWeComModeImmutable
}

func (s *MySQLStore) CompleteWeComIntegrationVerification(context.Context, dashboardadmin.Actor, int, uint64, dashboardadmin.WeComVerificationResult, string) (dashboardadmin.WeComIntegration, error) {
	return dashboardadmin.WeComIntegration{}, dashboardadmin.ErrWeComModeImmutable
}

func (s *MySQLStore) SwitchWeComIntegration(context.Context, dashboardadmin.Actor, int, uint64) (dashboardadmin.WeComIntegrationView, error) {
	return dashboardadmin.WeComIntegrationView{}, dashboardadmin.ErrWeComModeImmutable
}
func (s *MySQLStore) RollbackWeComIntegration(context.Context, dashboardadmin.Actor, int, uint64) (dashboardadmin.WeComIntegrationView, error) {
	return dashboardadmin.WeComIntegrationView{}, dashboardadmin.ErrWeComModeImmutable
}

func (s *MySQLStore) WeComIntegrationAudits(ctx context.Context, actor dashboardadmin.Actor, tenantID int) ([]dashboardadmin.WeComIntegrationAudit, error) {
	if s == nil || s.db == nil {
		return nil, dashboardadmin.ErrStoreUnavailable
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = lockSaaSActorPermissionTx(ctx, tx, actor.UserID, dashboard.SaaSAdminPermissionIntegrationsRead); err != nil {
		return nil, err
	}
	if _, err = lockWeComBindingTx(ctx, tx, tenantID); err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT id,action,target_id,COALESCE(before_json,JSON_OBJECT()),COALESCE(after_json,JSON_OBJECT()),actor_user_id,COALESCE(DATE_FORMAT(created_at,'%Y-%m-%dT%H:%i:%sZ'),'') FROM mochat_go_saas_admin_operation_logs WHERE tenant_id=? AND target_type='wecom_integration' AND deleted_at IS NULL ORDER BY id DESC LIMIT 100`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []dashboardadmin.WeComIntegrationAudit{}
	for rows.Next() {
		var item dashboardadmin.WeComIntegrationAudit
		if err = rows.Scan(&item.ID, &item.Action, &item.TargetID, &item.BeforeJSON, &item.AfterJSON, &item.ActorUserID, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return items, nil
}

func lockSaaSActorPermissionTx(ctx context.Context, tx *sql.Tx, userID int, permission string) error {
	if userID <= 0 {
		return dashboardadmin.ErrPermissionDenied
	}
	var id int
	err := tx.QueryRowContext(ctx, `SELECT u.id FROM mochat_go_saas_admin_users u WHERE u.id=? AND u.status=1 AND EXISTS (SELECT 1 FROM mochat_go_saas_admin_user_roles ur INNER JOIN mochat_go_saas_admin_roles r ON r.id=ur.role_id AND r.status=1 INNER JOIN mochat_go_saas_admin_role_permissions p ON p.role_id=r.id WHERE ur.user_id=u.id AND (p.permission_code=? OR p.permission_code='*')) LIMIT 1 FOR UPDATE`, userID, permission).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboardadmin.ErrPermissionDenied
	}
	return err
}
func lockWeComBindingTx(ctx context.Context, tx *sql.Tx, tenantID int) (weComBinding, error) {
	var b weComBinding
	err := tx.QueryRowContext(ctx, `SELECT b.corp_id,COALESCE(b.verified_wx_corpid,''),COALESCE(b.wecom_integration_mode,'') FROM mc_tenant t INNER JOIN mochat_go_tenant_corp_bindings b ON b.tenant_id=t.id AND b.status IN (1,2) INNER JOIN mc_corp c ON c.tenant_id=b.tenant_id AND c.id=b.corp_id AND c.deleted_at IS NULL WHERE t.id=? AND t.status=1 AND t.deleted_at IS NULL LIMIT 1 FOR UPDATE`, tenantID).Scan(&b.CorpID, &b.WXCorpID, &b.Mode)
	if errors.Is(err, sql.ErrNoRows) {
		return b, dashboardadmin.ErrTargetNotFound
	}
	return b, err
}

func validateWeComBindingMode(binding weComBinding, current *dashboardadmin.WeComIntegration) error {
	if (binding.Mode != dashboardadmin.WeComIntegrationModeSelfBuilt && binding.Mode != dashboardadmin.WeComIntegrationModeThirdPartyDelegated) || current == nil || current.Mode != binding.Mode {
		return dashboardadmin.ErrWeComModeImmutable
	}
	return nil
}

func loadWeComIntegrationViewTx(ctx context.Context, tx *sql.Tx, tenantID, corpID int, lock bool) (dashboardadmin.WeComIntegrationView, error) {
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	rows, err := tx.QueryContext(ctx, `SELECT `+weComIntegrationColumns+` FROM mochat_go_wecom_integrations WHERE tenant_id=? AND corp_id=? ORDER BY slot`+suffix, tenantID, corpID)
	if err != nil {
		return dashboardadmin.WeComIntegrationView{}, err
	}
	defer rows.Close()
	view := dashboardadmin.WeComIntegrationView{TenantID: tenantID, CorpID: corpID}
	for rows.Next() {
		item, err := scanWeComIntegration(rows)
		if err != nil {
			return view, err
		}
		public := publicWeComIntegration(item)
		if item.Slot == "current" {
			view.Current = &public
		} else if item.Slot == "candidate" {
			view.Candidate = &public
		}
	}
	return view, rows.Err()
}

type weComScanner interface{ Scan(...any) error }

func loadWeComIntegrationBySlotTx(ctx context.Context, tx *sql.Tx, tenantID, corpID int, slot string, lock bool) (dashboardadmin.WeComIntegration, error) {
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	return scanWeComIntegration(tx.QueryRowContext(ctx, `SELECT `+weComIntegrationColumns+` FROM mochat_go_wecom_integrations WHERE tenant_id=? AND corp_id=? AND slot=? LIMIT 1`+suffix, tenantID, corpID, slot))
}
func scanWeComIntegration(scanner weComScanner) (dashboardadmin.WeComIntegration, error) {
	var item dashboardadmin.WeComIntegration
	var scopeRaw, missingRaw string
	var verifiedAt sql.NullTime
	var updatedAt time.Time
	err := scanner.Scan(&item.ID, &item.Mode, &item.Slot, &item.Status, &item.VerifiedWXCorpID, &item.AgentID, &item.ProviderAppID, &item.CredentialCiphertext, &item.CredentialKeyID, &item.CredentialHint, &scopeRaw, &item.ScopeDigest, &missingRaw, &item.Generation, &item.Version, &item.VerificationLevel, &verifiedAt, &item.LastErrorCode, &updatedAt)
	if err != nil {
		return item, err
	}
	_ = json.Unmarshal([]byte(scopeRaw), &item.Scope)
	_ = json.Unmarshal([]byte(missingRaw), &item.MissingCapabilities)
	if item.Scope == nil {
		item.Scope = []string{}
	}
	if item.MissingCapabilities == nil {
		item.MissingCapabilities = []string{}
	}
	item.CredentialConfigured = item.CredentialCiphertext != "" && item.CredentialKeyID != ""
	if verifiedAt.Valid {
		item.VerifiedAt = verifiedAt.Time.UTC().Format(time.RFC3339Nano)
	}
	item.UpdatedAt = updatedAt.UTC().Format(time.RFC3339Nano)
	return item, nil
}
func publicWeComIntegration(item dashboardadmin.WeComIntegration) dashboardadmin.WeComIntegration {
	item.CredentialCiphertext = ""
	item.CredentialKeyID = ""
	return item
}
func ptrWeCom(item dashboardadmin.WeComIntegration) *dashboardadmin.WeComIntegration { return &item }

func weComCredentialHint(c wecomcredentials.AuthorizationCredential) string {
	names := []string{}
	if c.EmployeeSecret != "" {
		names = append(names, "employee")
	}
	if c.ContactSecret != "" {
		names = append(names, "contact")
	}
	if c.AgentSecret != "" {
		names = append(names, "agent")
	}
	if c.ChatSecret != "" {
		names = append(names, "chat")
	}
	if c.PermanentCode != "" {
		names = append(names, "permanent_code")
	}
	sort.Strings(names)
	return strings.Join(names, ",")
}
func insertWeComAuditTx(ctx context.Context, tx *sql.Tx, actorUserID, tenantID int, action, targetID string, before, after *dashboardadmin.WeComIntegration) error {
	safe := func(item *dashboardadmin.WeComIntegration) string {
		if item == nil {
			return ""
		}
		value := publicWeComIntegration(*item)
		raw, _ := json.Marshal(value)
		return string(raw)
	}
	_, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{TenantID: tenantID, ActorUserID: actorUserID, Action: action, TargetType: "wecom_integration", TargetID: targetID, BeforeJSON: safe(before), AfterJSON: safe(after), Remark: "tenant-scoped WeCom integration state transition"})
	return err
}
