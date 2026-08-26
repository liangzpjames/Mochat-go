package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"jiyi/mochat-go/internal/dashboard"
	"jiyi/mochat-go/internal/dashboardadmin"
	"jiyi/mochat-go/internal/wecomcredentials"

	"github.com/google/uuid"
)

const weComIntegrationColumns = `id,mode,slot,status,COALESCE(verified_wx_corpid,''),COALESCE(agent_id,''),COALESCE(provider_app_id,''),COALESCE(credential_ciphertext,''),COALESCE(credential_key_id,''),COALESCE(credential_hint,''),scope_json,COALESCE(scope_digest,''),missing_capabilities_json,generation,version,COALESCE(verification_level,''),verified_at,COALESCE(last_error_code,''),updated_at`

type weComBinding struct {
	CorpID   int
	WXCorpID string
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
	if err = tx.Commit(); err != nil {
		return dashboardadmin.WeComIntegrationView{}, err
	}
	return view, nil
}

func (s *MySQLStore) SaveWeComIntegrationCandidate(ctx context.Context, actor dashboardadmin.Actor, tenantID int, input dashboardadmin.WeComIntegrationCandidateInput) (dashboardadmin.WeComIntegration, error) {
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
	view, err := loadWeComIntegrationViewTx(ctx, tx, tenantID, binding.CorpID, true)
	if err != nil {
		return dashboardadmin.WeComIntegration{}, err
	}
	var id string
	var expected uint64
	var existingCredential bool
	var credential wecomcredentials.AuthorizationCredential
	if view.Candidate != nil {
		rawCandidate, loadErr := loadWeComIntegrationBySlotTx(ctx, tx, tenantID, binding.CorpID, "candidate", true)
		if loadErr != nil {
			return dashboardadmin.WeComIntegration{}, loadErr
		}
		id = rawCandidate.ID
		expected = rawCandidate.Version
		existingCredential = rawCandidate.CredentialConfigured
		if input.Version != expected {
			return dashboardadmin.WeComIntegration{}, dashboardadmin.ErrVersionConflict
		}
		if existingCredential && input.Mode == rawCandidate.Mode {
			credential, err = s.weComCredentialCipher.DecryptAuthorization(tenantID, id, rawCandidate.CredentialKeyID, rawCandidate.CredentialCiphertext)
			if err != nil {
				return dashboardadmin.WeComIntegration{}, dashboardadmin.ErrWeComCredentialDecrypt
			}
		}
	} else {
		id = uuid.NewString()
		if view.Current == nil || input.Version != view.Current.Version {
			return dashboardadmin.WeComIntegration{}, dashboardadmin.ErrVersionConflict
		}
	}
	if err = dashboardadmin.ValidateWeComIntegrationCandidate(input, existingCredential && credential.Mode == input.Mode); err != nil {
		return dashboardadmin.WeComIntegration{}, err
	}
	credential.Mode = input.Mode
	credential.ProviderAppID = input.ProviderAppID
	if input.Mode == "self_built" {
		if input.EmployeeSecret != "" {
			credential.EmployeeSecret = input.EmployeeSecret
		}
		if input.ContactSecret != "" {
			credential.ContactSecret = input.ContactSecret
		}
		if input.AgentSecret != "" {
			credential.AgentSecret = input.AgentSecret
		}
		if input.ChatSecret != "" {
			credential.ChatSecret = input.ChatSecret
		}
		credential.PermanentCode = ""
	} else {
		credential.EmployeeSecret = ""
		credential.ContactSecret = ""
		credential.AgentSecret = ""
		credential.ChatSecret = ""
		if input.PermanentCode != "" {
			credential.PermanentCode = input.PermanentCode
		}
	}
	ciphertext, keyID, err := s.weComCredentialCipher.EncryptAuthorization(tenantID, id, credential)
	if err != nil {
		return dashboardadmin.WeComIntegration{}, dashboardadmin.ErrStoreUnavailable
	}
	hint := weComCredentialHint(credential)
	scopeJSON, _ := json.Marshal(input.Scope)
	digest := dashboardadmin.WeComScopeDigest(input.Scope)
	if view.Candidate == nil {
		_, err = tx.ExecContext(ctx, `INSERT INTO mochat_go_wecom_integrations (id,tenant_id,corp_id,mode,slot,status,agent_id,provider_app_id,credential_ciphertext,credential_key_id,credential_hint,scope_json,scope_digest,missing_capabilities_json,generation,version,verification_level,last_error_code,created_at,updated_at) VALUES (?,?,?,?,'candidate','pending_verification',?,?,?,?,?,?,?,JSON_ARRAY(),1,1,'','',NOW(6),NOW(6))`, id, tenantID, binding.CorpID, input.Mode, input.AgentID, input.ProviderAppID, ciphertext, keyID, hint, string(scopeJSON), digest)
	} else {
		result, e := tx.ExecContext(ctx, `UPDATE mochat_go_wecom_integrations SET mode=?,status='pending_verification',verified_wx_corpid='',agent_id=?,provider_app_id=?,credential_ciphertext=?,credential_key_id=?,credential_hint=?,scope_json=?,scope_digest=?,missing_capabilities_json=JSON_ARRAY(),verification_level='',verified_at=NULL,last_error_code='',last_error_at=NULL,version=version+1,updated_at=NOW(6) WHERE id=? AND tenant_id=? AND corp_id=? AND slot='candidate' AND version=?`, input.Mode, input.AgentID, input.ProviderAppID, ciphertext, keyID, hint, string(scopeJSON), digest, id, tenantID, binding.CorpID, input.Version)
		err = e
		if err == nil {
			n, _ := result.RowsAffected()
			if n != 1 {
				err = dashboardadmin.ErrVersionConflict
			}
		}
	}
	if err != nil {
		return dashboardadmin.WeComIntegration{}, err
	}
	item, err := loadWeComIntegrationBySlotTx(ctx, tx, tenantID, binding.CorpID, "candidate", false)
	if err != nil {
		return dashboardadmin.WeComIntegration{}, err
	}
	if err = insertWeComAuditTx(ctx, tx, actor.UserID, tenantID, "wecom.integration.candidate.save", item.ID, nil, &item); err != nil {
		return dashboardadmin.WeComIntegration{}, err
	}
	if err = tx.Commit(); err != nil {
		return dashboardadmin.WeComIntegration{}, err
	}
	return publicWeComIntegration(item), nil
}

func (s *MySQLStore) WeComIntegrationVerificationCandidate(ctx context.Context, actor dashboardadmin.Actor, tenantID int, version uint64) (dashboardadmin.WeComVerificationCandidate, error) {
	if s == nil || s.db == nil || s.weComCredentialCipher == nil {
		return dashboardadmin.WeComVerificationCandidate{}, dashboardadmin.ErrStoreUnavailable
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboardadmin.WeComVerificationCandidate{}, err
	}
	defer tx.Rollback()
	if err = lockSaaSActorPermissionTx(ctx, tx, actor.UserID, dashboard.SaaSAdminPermissionIntegrationsManage); err != nil {
		return dashboardadmin.WeComVerificationCandidate{}, err
	}
	binding, err := lockWeComBindingTx(ctx, tx, tenantID)
	if err != nil {
		return dashboardadmin.WeComVerificationCandidate{}, err
	}
	item, err := loadWeComIntegrationBySlotTx(ctx, tx, tenantID, binding.CorpID, "candidate", true)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboardadmin.WeComVerificationCandidate{}, dashboardadmin.ErrTargetNotFound
	}
	if err != nil {
		return dashboardadmin.WeComVerificationCandidate{}, err
	}
	if item.Version != version {
		return dashboardadmin.WeComVerificationCandidate{}, dashboardadmin.ErrVersionConflict
	}
	credential, err := s.weComCredentialCipher.DecryptAuthorization(tenantID, item.ID, item.CredentialKeyID, item.CredentialCiphertext)
	if err != nil {
		return dashboardadmin.WeComVerificationCandidate{}, dashboardadmin.ErrWeComCredentialDecrypt
	}
	if err = tx.Commit(); err != nil {
		return dashboardadmin.WeComVerificationCandidate{}, err
	}
	return dashboardadmin.WeComVerificationCandidate{Integration: publicWeComIntegration(item), TenantID: tenantID, CorpID: binding.CorpID, AuthoritativeWXCorpID: binding.WXCorpID, Credentials: credential}, nil
}

func (s *MySQLStore) CompleteWeComIntegrationVerification(ctx context.Context, actor dashboardadmin.Actor, tenantID int, version uint64, verification dashboardadmin.WeComVerificationResult, errorCode string) (dashboardadmin.WeComIntegration, error) {
	if s == nil || s.db == nil {
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
	before, err := loadWeComIntegrationBySlotTx(ctx, tx, tenantID, binding.CorpID, "candidate", true)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboardadmin.WeComIntegration{}, dashboardadmin.ErrTargetNotFound
	}
	if err != nil {
		return dashboardadmin.WeComIntegration{}, err
	}
	if before.Version != version {
		return dashboardadmin.WeComIntegration{}, dashboardadmin.ErrVersionConflict
	}
	status, err := weComIntegrationVerificationStatus(verification, errorCode)
	if err != nil {
		return dashboardadmin.WeComIntegration{}, err
	}
	verifiedAt := "NOW(6)"
	if errorCode != "" {
		verifiedAt = "NULL"
		verification = dashboardadmin.WeComVerificationResult{}
	}
	scopeJSON, _ := json.Marshal(verification.Scope)
	missingJSON, _ := json.Marshal(verification.MissingCapabilities)
	query := fmt.Sprintf(`UPDATE mochat_go_wecom_integrations SET status=?,verified_wx_corpid=?,scope_json=?,scope_digest=?,missing_capabilities_json=?,verification_level=?,verified_at=%s,last_error_code=?,last_error_at=IF(?='',NULL,NOW(6)),version=version+1,updated_at=NOW(6) WHERE id=? AND version=?`, verifiedAt)
	result, err := tx.ExecContext(ctx, query, status, verification.VerifiedWXCorpID, string(scopeJSON), dashboardadmin.WeComScopeDigest(verification.Scope), string(missingJSON), verification.VerificationLevel, errorCode, errorCode, before.ID, version)
	if err != nil {
		return dashboardadmin.WeComIntegration{}, err
	}
	n, _ := result.RowsAffected()
	if n != 1 {
		return dashboardadmin.WeComIntegration{}, dashboardadmin.ErrVersionConflict
	}
	after, err := loadWeComIntegrationBySlotTx(ctx, tx, tenantID, binding.CorpID, "candidate", false)
	if err != nil {
		return dashboardadmin.WeComIntegration{}, err
	}
	if err = insertWeComAuditTx(ctx, tx, actor.UserID, tenantID, "wecom.integration.candidate.verify", after.ID, &before, &after); err != nil {
		return dashboardadmin.WeComIntegration{}, err
	}
	if err = tx.Commit(); err != nil {
		return dashboardadmin.WeComIntegration{}, err
	}
	return publicWeComIntegration(after), nil
}

func (s *MySQLStore) SwitchWeComIntegration(ctx context.Context, actor dashboardadmin.Actor, tenantID int, version uint64) (dashboardadmin.WeComIntegrationView, error) {
	return s.swapWeComIntegration(ctx, actor, tenantID, version, "wecom.integration.switch")
}
func (s *MySQLStore) RollbackWeComIntegration(ctx context.Context, actor dashboardadmin.Actor, tenantID int, version uint64) (dashboardadmin.WeComIntegrationView, error) {
	return s.swapWeComIntegration(ctx, actor, tenantID, version, "wecom.integration.rollback")
}

func (s *MySQLStore) swapWeComIntegration(ctx context.Context, actor dashboardadmin.Actor, tenantID int, version uint64, action string) (dashboardadmin.WeComIntegrationView, error) {
	if s == nil || s.db == nil || s.weComCredentialCipher == nil {
		return dashboardadmin.WeComIntegrationView{}, dashboardadmin.ErrStoreUnavailable
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboardadmin.WeComIntegrationView{}, err
	}
	defer tx.Rollback()
	if err = lockSaaSActorPermissionTx(ctx, tx, actor.UserID, dashboard.SaaSAdminPermissionIntegrationsManage); err != nil {
		return dashboardadmin.WeComIntegrationView{}, err
	}
	binding, err := lockWeComBindingTx(ctx, tx, tenantID)
	if err != nil {
		return dashboardadmin.WeComIntegrationView{}, err
	}
	current, err := loadWeComIntegrationBySlotTx(ctx, tx, tenantID, binding.CorpID, "current", true)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboardadmin.WeComIntegrationView{}, dashboardadmin.ErrTargetNotFound
	}
	if err != nil {
		return dashboardadmin.WeComIntegrationView{}, err
	}
	candidate, err := loadWeComIntegrationBySlotTx(ctx, tx, tenantID, binding.CorpID, "candidate", true)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboardadmin.WeComIntegrationView{}, dashboardadmin.ErrTargetNotFound
	}
	if err != nil {
		return dashboardadmin.WeComIntegrationView{}, err
	}
	_, decryptErr := s.weComCredentialCipher.DecryptAuthorization(tenantID, candidate.ID, candidate.CredentialKeyID, candidate.CredentialCiphertext)
	if decryptErr != nil {
		if _, validationErr := validateWeComIntegrationSwap(binding, current, candidate, version, 0, decryptErr); validationErr != nil {
			return dashboardadmin.WeComIntegrationView{}, validationErr
		}
	}
	var leases int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM mochat_go_archive_media_objects WHERE tenant_id=? AND corp_id=? AND status='fetching' AND lease_expires_at>NOW(6) FOR UPDATE`, tenantID, binding.CorpID).Scan(&leases); err != nil {
		return dashboardadmin.WeComIntegrationView{}, err
	}
	generation, err := validateWeComIntegrationSwap(binding, current, candidate, version, leases, nil)
	if err != nil {
		return dashboardadmin.WeComIntegrationView{}, err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM mochat_go_wecom_integrations WHERE tenant_id=? AND corp_id=? AND slot IN ('current','candidate')`, tenantID, binding.CorpID); err != nil {
		return dashboardadmin.WeComIntegrationView{}, err
	}
	oldCurrent := current
	oldCurrent.Slot = "candidate"
	oldCurrent.Version++
	next := candidate
	next.Slot = "current"
	next.Generation = generation
	next.Version++
	if err = insertWeComIntegrationSnapshotTx(ctx, tx, tenantID, binding.CorpID, oldCurrent); err != nil {
		return dashboardadmin.WeComIntegrationView{}, err
	}
	if err = insertWeComIntegrationSnapshotTx(ctx, tx, tenantID, binding.CorpID, next); err != nil {
		return dashboardadmin.WeComIntegrationView{}, err
	}
	if err = insertWeComAuditTx(ctx, tx, actor.UserID, tenantID, action, next.ID, &current, &next); err != nil {
		return dashboardadmin.WeComIntegrationView{}, err
	}
	if err = tx.Commit(); err != nil {
		return dashboardadmin.WeComIntegrationView{}, err
	}
	return dashboardadmin.WeComIntegrationView{TenantID: tenantID, CorpID: binding.CorpID, Current: ptrWeCom(publicWeComIntegration(next)), Candidate: ptrWeCom(publicWeComIntegration(oldCurrent))}, nil
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
	err := tx.QueryRowContext(ctx, `SELECT b.corp_id,COALESCE(b.verified_wx_corpid,'') FROM mc_tenant t INNER JOIN mochat_go_tenant_corp_bindings b ON b.tenant_id=t.id AND b.status=2 INNER JOIN mc_corp c ON c.tenant_id=b.tenant_id AND c.id=b.corp_id AND c.deleted_at IS NULL WHERE t.id=? AND t.status=1 AND t.deleted_at IS NULL AND COALESCE(b.verified_wx_corpid,'')<>'' LIMIT 1 FOR UPDATE`, tenantID).Scan(&b.CorpID, &b.WXCorpID)
	if errors.Is(err, sql.ErrNoRows) {
		return b, dashboardadmin.ErrTargetNotFound
	}
	return b, err
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
func nextWeComIntegrationGeneration(left, right uint64) uint64 {
	if right > left {
		return right + 1
	}
	return left + 1
}

func validateWeComIntegrationSwap(binding weComBinding, current, candidate dashboardadmin.WeComIntegration, version uint64, activeLeases int, decryptErr error) (uint64, error) {
	if candidate.Version != version {
		return 0, dashboardadmin.ErrVersionConflict
	}
	if candidate.Status != "active" || candidate.VerifiedAt == "" || candidate.VerificationLevel != dashboardadmin.WeComVerificationLocalContract {
		return 0, dashboardadmin.ErrWeComCandidateNotVerified
	}
	if candidate.VerifiedWXCorpID != binding.WXCorpID {
		return 0, dashboardadmin.ErrWeComCorpMismatch
	}
	if len(candidate.MissingCapabilities) > 0 {
		return 0, dashboardadmin.ErrWeComMissingCapabilities
	}
	if decryptErr != nil {
		return 0, dashboardadmin.ErrWeComCredentialDecrypt
	}
	if activeLeases > 0 {
		return 0, dashboardadmin.ErrWeComActiveMediaLease
	}
	return nextWeComIntegrationGeneration(current.Generation, candidate.Generation), nil
}

func weComIntegrationVerificationStatus(verification dashboardadmin.WeComVerificationResult, errorCode string) (string, error) {
	if strings.TrimSpace(errorCode) != "" {
		return "failed", nil
	}
	if verification.VerificationLevel != dashboardadmin.WeComVerificationLocalContract {
		return "", dashboardadmin.ErrInvalidRequest
	}
	return "active", nil
}
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
func insertWeComIntegrationSnapshotTx(ctx context.Context, tx *sql.Tx, tenantID, corpID int, item dashboardadmin.WeComIntegration) error {
	scope, _ := json.Marshal(item.Scope)
	missing, _ := json.Marshal(item.MissingCapabilities)
	var verified any
	if item.VerifiedAt != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, item.VerifiedAt); err == nil {
			verified = parsed
		}
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO mochat_go_wecom_integrations (id,tenant_id,corp_id,mode,slot,status,verified_wx_corpid,agent_id,provider_app_id,credential_ciphertext,credential_key_id,credential_hint,scope_json,scope_digest,missing_capabilities_json,generation,version,verification_level,verified_at,last_error_code,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,NOW(6),NOW(6))`, item.ID, tenantID, corpID, item.Mode, item.Slot, item.Status, item.VerifiedWXCorpID, item.AgentID, item.ProviderAppID, item.CredentialCiphertext, item.CredentialKeyID, item.CredentialHint, string(scope), item.ScopeDigest, string(missing), item.Generation, item.Version, item.VerificationLevel, verified, item.LastErrorCode)
	return err
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
