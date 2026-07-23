package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"jiyi/mochat-go/internal/dashboard"
	"jiyi/mochat-go/internal/identitysecurity"
)

const identityPolicySelect = `
	SELECT p.tenant_id, COALESCE(t.name, ''), p.status, p.max_failed_attempts, p.lockout_minutes,
		p.session_ttl_minutes, p.idle_timeout_minutes, p.max_concurrent_sessions, p.require_mfa,
		COALESCE(CAST(p.allowed_ip_cidrs AS CHAR), '[]'), p.login_event_retention_days,
		p.session_retention_days, p.version, p.updated_by, p.created_at, p.updated_at
	FROM mochat_go_saas_identity_policies p
	LEFT JOIN mc_tenant t ON t.id = p.tenant_id AND t.deleted_at IS NULL
`

func (s *MySQLStore) IdentityPolicy(ctx context.Context, tenantID int) (identitysecurity.Policy, bool, error) {
	policy, err := scanIdentityPolicy(s.db.QueryRowContext(ctx, identityPolicySelect+` WHERE p.tenant_id = ?`, tenantID))
	if errors.Is(err, sql.ErrNoRows) {
		return identitysecurity.Policy{}, false, nil
	}
	return policy, err == nil, err
}

func (s *MySQLStore) UpdateIdentityPolicy(ctx context.Context, input identitysecurity.PolicyUpdate) (identitysecurity.Policy, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return identitysecurity.Policy{}, err
	}
	defer rollbackQuietly(tx)
	before, err := scanIdentityPolicy(tx.QueryRowContext(ctx, identityPolicySelect+` WHERE p.tenant_id = ? FOR UPDATE`, input.TenantID))
	created := false
	if errors.Is(err, sql.ErrNoRows) {
		if input.ExpectedVersion != 0 {
			return identitysecurity.Policy{}, identitysecurity.Conflict("身份安全策略尚未创建，请刷新后重试")
		}
		created = true
	} else if err != nil {
		return identitysecurity.Policy{}, err
	} else if before.Version != input.ExpectedVersion {
		return identitysecurity.Policy{}, identitysecurity.Conflict("身份安全策略版本已变化，请刷新后重试")
	}
	cidrsJSON, err := json.Marshal(input.AllowedIPCIDRs)
	if err != nil {
		return identitysecurity.Policy{}, err
	}
	if created {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO mochat_go_saas_identity_policies
				(tenant_id, status, max_failed_attempts, lockout_minutes, session_ttl_minutes,
				 idle_timeout_minutes, max_concurrent_sessions, require_mfa, allowed_ip_cidrs,
				 login_event_retention_days, session_retention_days, version, updated_by, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?, NOW(), NOW())
		`, input.TenantID, input.Status, input.MaxFailedAttempts, input.LockoutMinutes, input.SessionTTLMinutes,
			input.IdleTimeoutMinutes, input.MaxConcurrentSessions, boolInt(input.RequireMFA), string(cidrsJSON),
			input.LoginEventRetentionDays, input.SessionRetentionDays, input.Actor.UserID)
	} else {
		var result sql.Result
		result, err = tx.ExecContext(ctx, `
			UPDATE mochat_go_saas_identity_policies
			SET status = ?, max_failed_attempts = ?, lockout_minutes = ?, session_ttl_minutes = ?,
				idle_timeout_minutes = ?, max_concurrent_sessions = ?, require_mfa = ?, allowed_ip_cidrs = ?,
				login_event_retention_days = ?, session_retention_days = ?, version = version + 1,
				updated_by = ?, updated_at = NOW()
			WHERE tenant_id = ? AND version = ?
		`, input.Status, input.MaxFailedAttempts, input.LockoutMinutes, input.SessionTTLMinutes,
			input.IdleTimeoutMinutes, input.MaxConcurrentSessions, boolInt(input.RequireMFA), string(cidrsJSON),
			input.LoginEventRetentionDays, input.SessionRetentionDays, input.Actor.UserID, input.TenantID, input.ExpectedVersion)
		if err == nil {
			if affected, _ := result.RowsAffected(); affected != 1 {
				return identitysecurity.Policy{}, identitysecurity.Conflict("身份安全策略版本已变化，请刷新后重试")
			}
		}
	}
	if err != nil {
		return identitysecurity.Policy{}, err
	}
	after, err := scanIdentityPolicy(tx.QueryRowContext(ctx, identityPolicySelect+` WHERE p.tenant_id = ?`, input.TenantID))
	if err != nil {
		return identitysecurity.Policy{}, err
	}
	beforeJSON := ""
	if !created {
		beforeJSON = identityPolicyAuditJSON(before)
	}
	operationID, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID: input.TenantID, ActorUserID: input.Actor.UserID, ActorTenantID: input.Actor.TenantID,
		Action: identitysecurity.OperationPolicyUpdate, TargetType: "identity_policy", TargetID: strconv.Itoa(input.TenantID),
		TargetName: after.TenantName, BeforeJSON: beforeJSON, AfterJSON: identityPolicyAuditJSON(after), Remark: "update identity security policy",
	})
	if err != nil {
		return identitysecurity.Policy{}, err
	}
	if err := markSaaSAdminApprovalEffectTx(ctx, tx, input.ApprovalExecutionID, input.ApprovalExecutionVersion, input.Actor.UserID, operationID); err != nil {
		return identitysecurity.Policy{}, err
	}
	if err := tx.Commit(); err != nil {
		return identitysecurity.Policy{}, err
	}
	return after, nil
}

func (s *MySQLStore) IdentityMFAEnrollmentCoverage(ctx context.Context, tenantID int) (int, int, error) {
	var active, enrolled int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*), SUM(CASE WHEN m.status = 'active' THEN 1 ELSE 0 END)
		FROM mc_user u
		LEFT JOIN mochat_go_saas_identity_mfa_credentials m ON m.user_id = u.id
		WHERE u.tenant_id = ? AND u.status = 1 AND u.deleted_at IS NULL
	`, tenantID).Scan(&active, &enrolled)
	return active, enrolled, err
}

func (s *MySQLStore) IdentityUserState(ctx context.Context, userID int) (identitysecurity.UserState, bool, error) {
	state, err := scanIdentityUserState(s.db.QueryRowContext(ctx, identityUserStateSelect+` WHERE u.id = ?`, userID))
	if errors.Is(err, sql.ErrNoRows) {
		return identitysecurity.UserState{}, false, nil
	}
	return state, err == nil, err
}

func (s *MySQLStore) RecordIdentityLoginFailure(ctx context.Context, input identitysecurity.LoginFailure) (identitysecurity.UserState, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return identitysecurity.UserState{}, err
	}
	defer rollbackQuietly(tx)
	lockedUntil := input.Now.Add(time.Duration(input.Policy.LockoutMinutes) * time.Minute)
	_, err = tx.ExecContext(ctx, `
		INSERT INTO mochat_go_saas_identity_user_states
			(user_id, tenant_id, failed_attempts, locked_until, last_failed_at, last_failed_ip,
			 last_success_at, last_success_ip, password_changed_at, version, created_at, updated_at)
		VALUES (?, ?, 1, NULL, ?, ?, NULL, '', NULL, 1, NOW(), NOW())
		ON DUPLICATE KEY UPDATE
			tenant_id = VALUES(tenant_id),
			locked_until = IF(failed_attempts + 1 >= ?, ?, locked_until),
			failed_attempts = failed_attempts + 1,
			last_failed_at = VALUES(last_failed_at), last_failed_ip = VALUES(last_failed_ip),
			version = version + 1, updated_at = NOW()
	`, input.Principal.UserID, input.Principal.TenantID, input.Now, input.Meta.IP,
		input.Policy.MaxFailedAttempts, lockedUntil)
	if err != nil {
		return identitysecurity.UserState{}, err
	}
	state, err := scanIdentityUserState(tx.QueryRowContext(ctx, identityUserStateSelect+` WHERE u.id = ? FOR UPDATE`, input.Principal.UserID))
	if err != nil {
		return identitysecurity.UserState{}, err
	}
	risk := identitysecurity.RiskWarning
	if !state.LockedUntilValue.IsZero() && input.Now.Before(state.LockedUntilValue) {
		risk = identitysecurity.RiskCritical
	}
	if err := insertIdentityLoginEventTx(ctx, tx, identitysecurity.LoginEvent{
		UserID: input.Principal.UserID, TenantID: input.Principal.TenantID, EventType: identitysecurity.EventPasswordFailed,
		Result: "failed", RiskLevel: risk, ReasonCode: input.Reason, IP: input.Meta.IP, UserAgent: input.Meta.UserAgent,
		Metadata: map[string]any{"failedAttempts": state.FailedAttempts, "lockedUntil": state.LockedUntil},
	}, input.PhoneHash, input.Now); err != nil {
		return identitysecurity.UserState{}, err
	}
	if risk == identitysecurity.RiskCritical {
		if err := upsertIdentityIncidentTx(ctx, tx, identityIncidentInput{
			StableKey: "brute_force:user:" + strconv.Itoa(input.Principal.UserID), TenantID: input.Principal.TenantID,
			UserID: input.Principal.UserID, Type: "brute_force", Severity: identitysecurity.RiskCritical,
			Title: "账号连续登录失败已锁定", Detail: fmt.Sprintf("失败 %d 次，锁定至 %s，来源 %s", state.FailedAttempts, state.LockedUntil, input.Meta.IP), Now: input.Now,
		}); err != nil {
			return identitysecurity.UserState{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return identitysecurity.UserState{}, err
	}
	return state, nil
}

func (s *MySQLStore) RecordIdentityLoginEvent(ctx context.Context, event identitysecurity.LoginEvent, phoneHash string, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer rollbackQuietly(tx)
	if err := insertIdentityLoginEventTx(ctx, tx, event, phoneHash, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *MySQLStore) RecordIdentityLoginSuccess(ctx context.Context, input identitysecurity.LoginSuccess) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer rollbackQuietly(tx)
	var previousIP string
	err = tx.QueryRowContext(ctx, `SELECT last_success_ip FROM mochat_go_saas_identity_user_states WHERE user_id = ? FOR UPDATE`, input.Principal.UserID).Scan(&previousIP)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}
	newIP := previousIP != "" && input.Meta.IP != "" && previousIP != input.Meta.IP
	_, err = tx.ExecContext(ctx, `
		INSERT INTO mochat_go_saas_identity_user_states
			(user_id, tenant_id, failed_attempts, locked_until, last_failed_at, last_failed_ip,
			 last_success_at, last_success_ip, password_changed_at, version, created_at, updated_at)
		VALUES (?, ?, 0, NULL, NULL, '', ?, ?, NULL, 1, NOW(), NOW())
		ON DUPLICATE KEY UPDATE tenant_id = VALUES(tenant_id), failed_attempts = 0, locked_until = NULL,
			last_success_at = VALUES(last_success_at), last_success_ip = VALUES(last_success_ip),
			version = version + 1, updated_at = NOW()
	`, input.Principal.UserID, input.Principal.TenantID, input.Now, input.Meta.IP)
	if err != nil {
		return false, err
	}
	risk := input.Risk
	if newIP {
		risk = identitysecurity.RiskWarning
	}
	if err := insertIdentityLoginEventTx(ctx, tx, identitysecurity.LoginEvent{
		UserID: input.Principal.UserID, TenantID: input.Principal.TenantID, EventType: input.EventType,
		Result: "succeeded", RiskLevel: risk, ReasonCode: input.Reason, IP: input.Meta.IP, UserAgent: input.Meta.UserAgent,
		Metadata: map[string]any{"newIp": newIP, "previousIp": previousIP},
	}, input.PhoneHash, input.Now); err != nil {
		return false, err
	}
	if newIP {
		if err := upsertIdentityIncidentTx(ctx, tx, identityIncidentInput{
			StableKey: "new_ip:user:" + strconv.Itoa(input.Principal.UserID), TenantID: input.Principal.TenantID,
			UserID: input.Principal.UserID, Type: "new_ip", Severity: identitysecurity.RiskWarning,
			Title: "账号从新 IP 登录", Detail: fmt.Sprintf("来源由 %s 变化为 %s", previousIP, input.Meta.IP), Now: input.Now,
		}); err != nil {
			return false, err
		}
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return newIP, nil
}

func (s *MySQLStore) IdentityMFACredential(ctx context.Context, userID int) (identitysecurity.MFACredential, bool, error) {
	credential, err := scanIdentityMFACredential(s.db.QueryRowContext(ctx, identityMFACredentialSelect+` WHERE user_id = ?`, userID))
	if errors.Is(err, sql.ErrNoRows) {
		return identitysecurity.MFACredential{}, false, nil
	}
	return credential, err == nil, err
}

func (s *MySQLStore) SavePendingIdentityMFA(ctx context.Context, credential identitysecurity.MFACredential, actor identitysecurity.Actor) (identitysecurity.MFACredential, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return identitysecurity.MFACredential{}, err
	}
	defer rollbackQuietly(tx)
	before, err := scanIdentityMFACredential(tx.QueryRowContext(ctx, identityMFACredentialSelect+` WHERE user_id = ? FOR UPDATE`, credential.UserID))
	found := err == nil
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return identitysecurity.MFACredential{}, err
	}
	if found && before.Status == identitysecurity.MFAStatusActive {
		return identitysecurity.MFACredential{}, identitysecurity.Conflict("MFA 已启用")
	}
	if found {
		result, err := tx.ExecContext(ctx, `
			UPDATE mochat_go_saas_identity_mfa_credentials
			SET tenant_id = ?, status = 'pending', secret_ciphertext = ?, encryption_key_id = ?,
				recovery_code_hashes = ?, recovery_codes_remaining = ?, verified_at = NULL, last_used_at = NULL,
				last_totp_step = 0, disabled_at = NULL, disabled_by = 0, disabled_reason = '',
				version = version + 1, updated_at = NOW()
			WHERE user_id = ? AND version = ?
		`, credential.TenantID, credential.SecretCiphertext, credential.EncryptionKeyID,
			credential.RecoveryCodeHashesJSON, credential.RecoveryCodesRemaining, credential.UserID, before.Version)
		if err != nil {
			return identitysecurity.MFACredential{}, err
		}
		if affected, _ := result.RowsAffected(); affected != 1 {
			return identitysecurity.MFACredential{}, identitysecurity.Conflict("MFA 凭据版本已变化")
		}
	} else {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO mochat_go_saas_identity_mfa_credentials
				(user_id, tenant_id, status, secret_ciphertext, encryption_key_id, recovery_code_hashes,
				 recovery_codes_remaining, verified_at, last_used_at, last_totp_step, disabled_at,
				 disabled_by, disabled_reason, version, created_at, updated_at)
			VALUES (?, ?, 'pending', ?, ?, ?, ?, NULL, NULL, 0, NULL, 0, '', 1, NOW(), NOW())
		`, credential.UserID, credential.TenantID, credential.SecretCiphertext, credential.EncryptionKeyID,
			credential.RecoveryCodeHashesJSON, credential.RecoveryCodesRemaining)
		if err != nil {
			return identitysecurity.MFACredential{}, err
		}
	}
	after, err := scanIdentityMFACredential(tx.QueryRowContext(ctx, identityMFACredentialSelect+` WHERE user_id = ?`, credential.UserID))
	if err != nil {
		return identitysecurity.MFACredential{}, err
	}
	if _, err := insertIdentityOperationTx(ctx, tx, identitysecurity.OperationMFABegin, credential.TenantID, credential.UserID, actor, foundMFAAudit(before, found), mfaAuditJSON(after), "begin MFA enrollment"); err != nil {
		return identitysecurity.MFACredential{}, err
	}
	if err := insertIdentityLoginEventTx(ctx, tx, identitysecurity.LoginEvent{
		UserID: credential.UserID, TenantID: credential.TenantID, EventType: identitysecurity.EventMFABegan,
		Result: "succeeded", RiskLevel: identitysecurity.RiskNormal, ReasonCode: "self_enrollment",
	}, "", time.Now()); err != nil {
		return identitysecurity.MFACredential{}, err
	}
	if err := tx.Commit(); err != nil {
		return identitysecurity.MFACredential{}, err
	}
	return after, nil
}

func (s *MySQLStore) ActivateIdentityMFA(ctx context.Context, userID, expectedVersion int, recoveryHashesJSON string, recoveryCount int, totpStep int64, actor identitysecurity.Actor, now time.Time) (identitysecurity.MFACredential, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return identitysecurity.MFACredential{}, err
	}
	defer rollbackQuietly(tx)
	before, err := scanIdentityMFACredential(tx.QueryRowContext(ctx, identityMFACredentialSelect+` WHERE user_id = ? FOR UPDATE`, userID))
	if errors.Is(err, sql.ErrNoRows) {
		return identitysecurity.MFACredential{}, identitysecurity.NotFound("MFA 凭据不存在")
	}
	if err != nil {
		return identitysecurity.MFACredential{}, err
	}
	if before.Status != identitysecurity.MFAStatusPending || before.Version != expectedVersion {
		return identitysecurity.MFACredential{}, identitysecurity.Conflict("MFA 凭据状态或版本已变化")
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_identity_mfa_credentials
		SET status = 'active', recovery_code_hashes = ?, recovery_codes_remaining = ?, verified_at = ?,
			last_used_at = ?, last_totp_step = ?, version = version + 1, updated_at = NOW()
		WHERE user_id = ? AND version = ? AND status = 'pending'
	`, recoveryHashesJSON, recoveryCount, now, now, totpStep, userID, expectedVersion)
	if err != nil {
		return identitysecurity.MFACredential{}, err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return identitysecurity.MFACredential{}, identitysecurity.Conflict("MFA 凭据状态或版本已变化")
	}
	after, err := scanIdentityMFACredential(tx.QueryRowContext(ctx, identityMFACredentialSelect+` WHERE user_id = ?`, userID))
	if err != nil {
		return identitysecurity.MFACredential{}, err
	}
	if _, err := insertIdentityOperationTx(ctx, tx, identitysecurity.OperationMFAEnable, before.TenantID, userID, actor, mfaAuditJSON(before), mfaAuditJSON(after), "enable MFA"); err != nil {
		return identitysecurity.MFACredential{}, err
	}
	if err := insertIdentityLoginEventTx(ctx, tx, identitysecurity.LoginEvent{
		UserID: userID, TenantID: before.TenantID, EventType: identitysecurity.EventMFAEnabled,
		Result: "succeeded", RiskLevel: identitysecurity.RiskNormal, ReasonCode: "totp_verified",
	}, "", now); err != nil {
		return identitysecurity.MFACredential{}, err
	}
	if err := tx.Commit(); err != nil {
		return identitysecurity.MFACredential{}, err
	}
	return after, nil
}

func (s *MySQLStore) UseIdentityMFA(ctx context.Context, userID, expectedVersion int, recoveryHashesJSON string, recoveryCount int, totpStep int64, now time.Time) (identitysecurity.MFACredential, error) {
	query := `
		UPDATE mochat_go_saas_identity_mfa_credentials
		SET recovery_code_hashes = ?, recovery_codes_remaining = ?, last_used_at = ?,
			last_totp_step = IF(? > 0, ?, last_totp_step), version = version + 1, updated_at = NOW()
		WHERE user_id = ? AND version = ? AND status = 'active' AND (? = 0 OR last_totp_step < ?)
	`
	result, err := s.db.ExecContext(ctx, query, recoveryHashesJSON, recoveryCount, now, totpStep, totpStep, userID, expectedVersion, totpStep, totpStep)
	if err != nil {
		return identitysecurity.MFACredential{}, err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return identitysecurity.MFACredential{}, identitysecurity.Conflict("动态验证码已使用或 MFA 凭据版本已变化")
	}
	credential, _, err := s.IdentityMFACredential(ctx, userID)
	return credential, err
}

func (s *MySQLStore) ResetIdentityMFA(ctx context.Context, input identitysecurity.MFAReset, now time.Time) (identitysecurity.MFAResetResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return identitysecurity.MFAResetResult{}, err
	}
	defer rollbackQuietly(tx)
	var tenantID, userStatus int
	err = tx.QueryRowContext(ctx, `
		SELECT tenant_id, status
		FROM mc_user
		WHERE id = ? AND deleted_at IS NULL
		FOR UPDATE
	`, input.UserID).Scan(&tenantID, &userStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return identitysecurity.MFAResetResult{}, identitysecurity.NotFound("身份安全账号不存在")
	}
	if err != nil {
		return identitysecurity.MFAResetResult{}, err
	}
	if tenantID != input.TenantID {
		return identitysecurity.MFAResetResult{}, identitysecurity.Conflict("身份安全账号租户归属已变化")
	}
	before, err := scanIdentityMFACredential(tx.QueryRowContext(ctx, identityMFACredentialSelect+` WHERE user_id = ? FOR UPDATE`, input.UserID))
	if errors.Is(err, sql.ErrNoRows) {
		return identitysecurity.MFAResetResult{}, identitysecurity.NotFound("MFA 凭据不存在")
	}
	if err != nil {
		return identitysecurity.MFAResetResult{}, err
	}
	if before.TenantID != input.TenantID {
		return identitysecurity.MFAResetResult{}, identitysecurity.Conflict("MFA 凭据租户归属已变化")
	}
	if before.Version != input.ExpectedVersion || (before.Status != identitysecurity.MFAStatusActive && before.Status != identitysecurity.MFAStatusPending) {
		return identitysecurity.MFAResetResult{}, identitysecurity.Conflict("MFA 凭据状态或版本已变化")
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_identity_mfa_credentials
		SET status = 'disabled', secret_ciphertext = '', encryption_key_id = '', recovery_code_hashes = JSON_ARRAY(),
			recovery_codes_remaining = 0, last_totp_step = 0, disabled_at = ?, disabled_by = ?, disabled_reason = ?,
			version = version + 1, updated_at = NOW()
		WHERE user_id = ? AND version = ?
	`, now, input.Actor.UserID, truncateRunes(input.Reason, 255), input.UserID, input.ExpectedVersion)
	if err != nil {
		return identitysecurity.MFAResetResult{}, err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return identitysecurity.MFAResetResult{}, identitysecurity.Conflict("MFA 凭据状态或版本已变化")
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_identity_auth_challenges
		SET status = 'expired', active_slot = NULL, updated_at = NOW()
		WHERE user_id = ? AND status = 'pending'
	`, input.UserID); err != nil {
		return identitysecurity.MFAResetResult{}, err
	}
	sessionResult, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_identity_sessions
		SET status = 'revoked', revoked_at = ?, revoked_by = ?, revocation_reason = ?,
			version = version + 1, updated_at = NOW()
		WHERE user_id = ? AND tenant_id = ? AND status = 'active'
	`, now, input.Actor.UserID, truncateRunes("MFA 重置："+input.Reason, 255), input.UserID, input.TenantID)
	if err != nil {
		return identitysecurity.MFAResetResult{}, err
	}
	revokedSessions64, _ := sessionResult.RowsAffected()
	revokedSessions := int(revokedSessions64)
	after, err := scanIdentityMFACredential(tx.QueryRowContext(ctx, identityMFACredentialSelect+` WHERE user_id = ?`, input.UserID))
	if err != nil {
		return identitysecurity.MFAResetResult{}, err
	}
	operationID, err := insertIdentityOperationTx(ctx, tx, identitysecurity.OperationMFAReset, before.TenantID, input.UserID, input.Actor,
		mfaAuditJSON(before), mfaResetAuditJSON(after, revokedSessions, userStatus), input.Reason)
	if err != nil {
		return identitysecurity.MFAResetResult{}, err
	}
	if err := insertIdentityLoginEventTx(ctx, tx, identitysecurity.LoginEvent{
		UserID: input.UserID, TenantID: before.TenantID, EventType: identitysecurity.EventMFAReset,
		Result: "succeeded", RiskLevel: identitysecurity.RiskCritical, ReasonCode: "admin_reset",
		Metadata: map[string]any{"revokedSessions": revokedSessions, "approvalId": input.ApprovalExecutionID},
	}, "", now); err != nil {
		return identitysecurity.MFAResetResult{}, err
	}
	if err := markSaaSAdminApprovalEffectTx(ctx, tx, input.ApprovalExecutionID, input.ApprovalExecutionVersion, input.Actor.UserID, operationID); err != nil {
		return identitysecurity.MFAResetResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return identitysecurity.MFAResetResult{}, err
	}
	return identitysecurity.MFAResetResult{Credential: after, RevokedSessions: revokedSessions, OperationID: operationID}, nil
}

func (s *MySQLStore) CreateIdentityChallenge(ctx context.Context, challengeHash string, principal identitysecurity.Principal, meta identitysecurity.RequestMeta, expiresAt time.Time) (identitysecurity.Challenge, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return identitysecurity.Challenge{}, err
	}
	defer rollbackQuietly(tx)
	if _, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_identity_auth_challenges
		SET status = 'expired', active_slot = NULL, updated_at = NOW()
		WHERE user_id = ? AND status = 'pending'
	`, principal.UserID); err != nil {
		return identitysecurity.Challenge{}, err
	}
	result, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_saas_identity_auth_challenges
			(challenge_hash, user_id, tenant_id, challenge_type, status, active_slot, attempts,
			 max_attempts, ip_address, user_agent, expires_at, consumed_at, created_at, updated_at)
		VALUES (?, ?, ?, 'mfa_login', 'pending', ?, 0, 5, ?, ?, ?, NULL, NOW(), NOW())
	`, challengeHash, principal.UserID, principal.TenantID, principal.UserID, meta.IP, truncateRunes(meta.UserAgent, 500), expiresAt)
	if err != nil {
		return identitysecurity.Challenge{}, err
	}
	id, _ := result.LastInsertId()
	challenge, err := identityChallengeByID(ctx, tx, id, false)
	if err != nil {
		return identitysecurity.Challenge{}, err
	}
	if err := tx.Commit(); err != nil {
		return identitysecurity.Challenge{}, err
	}
	return challenge, nil
}

func (s *MySQLStore) IdentityChallenge(ctx context.Context, challengeHash string) (identitysecurity.Challenge, error) {
	challenge, err := identityChallengeByHash(ctx, s.db, challengeHash, false)
	if errors.Is(err, sql.ErrNoRows) {
		return identitysecurity.Challenge{}, identitysecurity.NotFound("二次认证挑战不存在")
	}
	return challenge, err
}

func (s *MySQLStore) FailIdentityChallenge(ctx context.Context, challengeID int64, reason string, now time.Time) (identitysecurity.Challenge, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return identitysecurity.Challenge{}, err
	}
	defer rollbackQuietly(tx)
	before, err := identityChallengeByID(ctx, tx, challengeID, true)
	if err != nil {
		return identitysecurity.Challenge{}, err
	}
	if before.Status != identitysecurity.ChallengeStatusPending {
		return identitysecurity.Challenge{}, identitysecurity.Conflict("二次认证挑战已结束")
	}
	status := identitysecurity.ChallengeStatusPending
	activeSlot := any(before.UserID)
	if before.Attempts+1 >= before.MaxAttempts || now.After(before.ExpiresAt) {
		status, activeSlot = identitysecurity.ChallengeStatusLocked, nil
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_identity_auth_challenges
		SET attempts = attempts + 1, status = ?, active_slot = ?, updated_at = NOW()
		WHERE id = ? AND status = 'pending'
	`, status, activeSlot, challengeID); err != nil {
		return identitysecurity.Challenge{}, err
	}
	after, err := identityChallengeByID(ctx, tx, challengeID, false)
	if err != nil {
		return identitysecurity.Challenge{}, err
	}
	if after.Status == identitysecurity.ChallengeStatusLocked {
		if err := upsertIdentityIncidentTx(ctx, tx, identityIncidentInput{
			StableKey: "mfa_failure:user:" + strconv.Itoa(after.UserID), TenantID: after.TenantID, UserID: after.UserID,
			Type: "mfa_failure", Severity: identitysecurity.RiskCritical, Title: "MFA 连续验证失败",
			Detail: fmt.Sprintf("挑战 #%d 已锁定，来源 %s，原因 %s", after.ID, after.IP, reason), Now: now,
		}); err != nil {
			return identitysecurity.Challenge{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return identitysecurity.Challenge{}, err
	}
	return after, nil
}

func (s *MySQLStore) ConsumeIdentityChallenge(ctx context.Context, challengeID int64, now time.Time) (identitysecurity.Challenge, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return identitysecurity.Challenge{}, err
	}
	defer rollbackQuietly(tx)
	before, err := identityChallengeByID(ctx, tx, challengeID, true)
	if err != nil {
		return identitysecurity.Challenge{}, err
	}
	if before.Status != identitysecurity.ChallengeStatusPending || now.After(before.ExpiresAt) {
		return identitysecurity.Challenge{}, identitysecurity.Conflict("二次认证挑战已结束")
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_identity_auth_challenges
		SET status = 'consumed', active_slot = NULL, consumed_at = ?, updated_at = NOW()
		WHERE id = ? AND status = 'pending'
	`, now, challengeID)
	if err != nil {
		return identitysecurity.Challenge{}, err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return identitysecurity.Challenge{}, identitysecurity.Conflict("二次认证挑战已结束")
	}
	after, err := identityChallengeByID(ctx, tx, challengeID, false)
	if err != nil {
		return identitysecurity.Challenge{}, err
	}
	if err := tx.Commit(); err != nil {
		return identitysecurity.Challenge{}, err
	}
	return after, nil
}

func (s *MySQLStore) CreateIdentitySession(ctx context.Context, input identitysecurity.SessionCreate) (identitysecurity.Session, int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return identitysecurity.Session{}, 0, err
	}
	defer rollbackQuietly(tx)
	if _, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_identity_sessions
		SET status = 'expired', version = version + 1, updated_at = NOW()
		WHERE user_id = ? AND status = 'active' AND (expires_at <= ? OR idle_expires_at <= ?)
	`, input.Principal.UserID, input.IssuedAt, input.IssuedAt); err != nil {
		return identitysecurity.Session{}, 0, err
	}
	result, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_saas_identity_sessions
			(session_jti, token_sha256, user_id, tenant_id, status, auth_method, ip_address, user_agent,
			 issued_at, expires_at, idle_expires_at, last_seen_at, revoked_at, revoked_by,
			 revocation_reason, version, created_at, updated_at)
		VALUES (?, ?, ?, ?, 'active', ?, ?, ?, ?, ?, ?, ?, NULL, 0, '', 1, NOW(), NOW())
	`, input.SessionJTI, input.TokenSHA256, input.Principal.UserID, input.Principal.TenantID, input.AuthMethod,
		input.Meta.IP, truncateRunes(input.Meta.UserAgent, 500), input.IssuedAt, input.ExpiresAt, input.IdleExpiresAt, input.IssuedAt)
	if isMySQLDuplicateKeyError(err) {
		return identitysecurity.Session{}, 0, identitysecurity.Conflict("登录会话已存在")
	}
	if err != nil {
		return identitysecurity.Session{}, 0, err
	}
	id, _ := result.LastInsertId()
	rows, err := tx.QueryContext(ctx, `
		SELECT id FROM mochat_go_saas_identity_sessions
		WHERE user_id = ? AND status = 'active'
		ORDER BY issued_at DESC, id DESC
	`, input.Principal.UserID)
	if err != nil {
		return identitysecurity.Session{}, 0, err
	}
	var activeIDs []int64
	for rows.Next() {
		var activeID int64
		if err := rows.Scan(&activeID); err != nil {
			rows.Close()
			return identitysecurity.Session{}, 0, err
		}
		activeIDs = append(activeIDs, activeID)
	}
	if err := rows.Close(); err != nil {
		return identitysecurity.Session{}, 0, err
	}
	limit := input.MaxConcurrentSessions
	if limit <= 0 {
		limit = 5
	}
	revoked := 0
	if len(activeIDs) > limit {
		oldIDs := activeIDs[limit:]
		query, args := identityInQuery(`
			UPDATE mochat_go_saas_identity_sessions
			SET status = 'revoked', revoked_at = ?, revoked_by = 0,
				revocation_reason = 'concurrent session limit', version = version + 1, updated_at = NOW()
			WHERE status = 'active' AND id IN (%s)
		`, []any{input.IssuedAt}, oldIDs)
		res, err := tx.ExecContext(ctx, query, args...)
		if err != nil {
			return identitysecurity.Session{}, 0, err
		}
		affected, _ := res.RowsAffected()
		revoked = int(affected)
		if revoked > 0 {
			if err := upsertIdentityIncidentTx(ctx, tx, identityIncidentInput{
				StableKey: "concurrent_session:user:" + strconv.Itoa(input.Principal.UserID), TenantID: input.Principal.TenantID,
				UserID: input.Principal.UserID, Type: "concurrent_session", Severity: identitysecurity.RiskWarning,
				Title: "并发会话超过策略上限", Detail: fmt.Sprintf("自动撤销 %d 个最旧会话，上限 %d", revoked, limit), Now: input.IssuedAt,
			}); err != nil {
				return identitysecurity.Session{}, 0, err
			}
		}
	}
	if err := insertIdentityLoginEventTx(ctx, tx, identitysecurity.LoginEvent{
		UserID: input.Principal.UserID, TenantID: input.Principal.TenantID, EventType: identitysecurity.EventLoginSucceeded,
		Result: "succeeded", RiskLevel: identitysecurity.RiskNormal, ReasonCode: input.AuthMethod,
		IP: input.Meta.IP, UserAgent: input.Meta.UserAgent, Metadata: map[string]any{"sessionId": id, "authMethod": input.AuthMethod, "autoRevoked": revoked},
	}, "", input.IssuedAt); err != nil {
		return identitysecurity.Session{}, 0, err
	}
	session, err := identitySessionByID(ctx, tx, id, false)
	if err != nil {
		return identitysecurity.Session{}, 0, err
	}
	if err := tx.Commit(); err != nil {
		return identitysecurity.Session{}, 0, err
	}
	return session, revoked, nil
}

func (s *MySQLStore) ValidateIdentitySession(ctx context.Context, claims identitysecurity.SessionClaims, now time.Time) error {
	var id int64
	var userID, userStatus, tenantStatus, idleMinutes int
	var status string
	var expiresAt, idleExpiresAt, lastSeenAt time.Time
	err := s.db.QueryRowContext(ctx, `
		SELECT sess.id, sess.user_id, sess.status, sess.expires_at, sess.idle_expires_at, sess.last_seen_at,
			COALESCE(u.status, 0), COALESCE(t.status, 2), COALESCE(p.idle_timeout_minutes, 1440)
		FROM mochat_go_saas_identity_sessions sess
		LEFT JOIN mc_user u ON u.id = sess.user_id AND u.deleted_at IS NULL
		LEFT JOIN mc_tenant t ON t.id = sess.tenant_id AND t.deleted_at IS NULL
		LEFT JOIN mochat_go_saas_identity_policies p ON p.tenant_id = sess.tenant_id AND p.status = 'active'
		WHERE sess.session_jti = ?
		LIMIT 1
	`, claims.JTI).Scan(&id, &userID, &status, &expiresAt, &idleExpiresAt, &lastSeenAt, &userStatus, &tenantStatus, &idleMinutes)
	if errors.Is(err, sql.ErrNoRows) {
		return identitysecurity.ErrSessionNotFound
	}
	if err != nil {
		return err
	}
	if userID != claims.UserID || userStatus != 1 || tenantStatus == 2 || status == identitysecurity.SessionStatusRevoked {
		return identitysecurity.ErrSessionRevoked
	}
	if status != identitysecurity.SessionStatusActive || !now.Before(expiresAt) || !now.Before(idleExpiresAt) {
		_, _ = s.db.ExecContext(ctx, `UPDATE mochat_go_saas_identity_sessions SET status = 'expired', version = version + 1, updated_at = NOW() WHERE id = ? AND status = 'active'`, id)
		return identitysecurity.ErrSessionExpired
	}
	if now.Sub(lastSeenAt) >= time.Minute {
		newIdle := now.Add(time.Duration(idleMinutes) * time.Minute)
		if newIdle.After(expiresAt) {
			newIdle = expiresAt
		}
		_, err = s.db.ExecContext(ctx, `
			UPDATE mochat_go_saas_identity_sessions
			SET last_seen_at = ?, idle_expires_at = ?, updated_at = NOW()
			WHERE id = ? AND status = 'active' AND last_seen_at = ?
		`, now, newIdle, id, lastSeenAt)
	}
	return err
}

func (s *MySQLStore) IdentityOverview(ctx context.Context, tenantID, limit int) (identitysecurity.Summary, []identitysecurity.UserState, []identitysecurity.Session, []identitysecurity.LoginEvent, []identitysecurity.Incident, error) {
	now := time.Now()
	var summary identitysecurity.Summary
	err := s.db.QueryRowContext(ctx, `
		SELECT
			(SELECT COUNT(*) FROM mochat_go_saas_identity_sessions WHERE tenant_id = ? AND status = 'active' AND expires_at > ? AND idle_expires_at > ?),
			(SELECT COUNT(*) FROM mochat_go_saas_identity_user_states WHERE tenant_id = ? AND locked_until > ?),
			(SELECT COUNT(*) FROM mc_user WHERE tenant_id = ? AND status = 1 AND deleted_at IS NULL),
			(SELECT COUNT(*) FROM mochat_go_saas_identity_mfa_credentials WHERE tenant_id = ? AND status = 'active'),
			(SELECT COUNT(*) FROM mochat_go_saas_identity_security_incidents WHERE tenant_id = ? AND status IN ('open','acknowledged')),
			(SELECT COUNT(*) FROM mochat_go_saas_identity_security_incidents WHERE tenant_id = ? AND status IN ('open','acknowledged') AND severity = 'critical'),
			(SELECT COUNT(*) FROM mochat_go_saas_identity_login_events WHERE tenant_id = ? AND result IN ('failed','blocked') AND occurred_at >= DATE_SUB(?, INTERVAL 24 HOUR)),
			(SELECT COUNT(*) FROM mochat_go_saas_identity_login_events WHERE tenant_id = ? AND event_type = 'login_succeeded' AND result = 'succeeded' AND occurred_at >= DATE_SUB(?, INTERVAL 24 HOUR))
	`, tenantID, now, now, tenantID, now, tenantID, tenantID, tenantID, tenantID, tenantID, now, tenantID, now).Scan(
		&summary.ActiveSessions, &summary.LockedUsers, &summary.ActiveUsers, &summary.MFAEnrolledUsers,
		&summary.OpenIncidents, &summary.CriticalIncidents, &summary.FailedLogins24h, &summary.SuccessfulLogins24h)
	if err != nil {
		return summary, nil, nil, nil, nil, err
	}
	users, err := s.identityUsers(ctx, tenantID, limit)
	if err != nil {
		return summary, nil, nil, nil, nil, err
	}
	sessions, err := s.IdentitySessions(ctx, identitysecurity.SessionOptions{TenantID: tenantID, Limit: limit})
	if err != nil {
		return summary, nil, nil, nil, nil, err
	}
	events, err := s.IdentityLoginEvents(ctx, identitysecurity.EventOptions{TenantID: tenantID, Limit: limit})
	if err != nil {
		return summary, nil, nil, nil, nil, err
	}
	incidents, err := s.IdentityIncidents(ctx, identitysecurity.IncidentOptions{TenantID: tenantID, Limit: limit})
	return summary, users, sessions, events, incidents, err
}

func (s *MySQLStore) IdentitySessions(ctx context.Context, options identitysecurity.SessionOptions) ([]identitysecurity.Session, error) {
	limit := options.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	where := []string{"1 = 1"}
	args := make([]any, 0, 6)
	if options.TenantID > 0 {
		where = append(where, "sess.tenant_id = ?")
		args = append(args, options.TenantID)
	}
	if options.UserID > 0 {
		where = append(where, "sess.user_id = ?")
		args = append(args, options.UserID)
	}
	if status := strings.TrimSpace(options.Status); status != "" && status != "all" {
		where = append(where, "sess.status = ?")
		args = append(args, status)
	}
	if keyword := strings.TrimSpace(options.Keyword); keyword != "" {
		where = append(where, "(u.name LIKE ? OR u.phone LIKE ? OR sess.ip_address LIKE ? OR sess.user_agent LIKE ? OR sess.session_jti LIKE ?)")
		like := "%" + keyword + "%"
		args = append(args, like, like, like, like, like)
	}
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, identitySessionSelect+` WHERE `+strings.Join(where, " AND ")+` ORDER BY sess.issued_at DESC, sess.id DESC LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]identitysecurity.Session, 0)
	for rows.Next() {
		item, err := scanIdentitySession(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQLStore) RevokeIdentitySession(ctx context.Context, input identitysecurity.SessionRevoke, now time.Time) (identitysecurity.Session, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return identitysecurity.Session{}, err
	}
	defer rollbackQuietly(tx)
	var before identitysecurity.Session
	if input.SessionID > 0 {
		before, err = identitySessionByID(ctx, tx, input.SessionID, true)
		if errors.Is(err, sql.ErrNoRows) {
			return identitysecurity.Session{}, identitysecurity.NotFound("登录会话不存在")
		}
		if err != nil {
			return identitysecurity.Session{}, err
		}
		if input.ExpectedVersion > 0 && before.Version != input.ExpectedVersion {
			return identitysecurity.Session{}, identitysecurity.Conflict("登录会话版本已变化")
		}
		if before.Status != identitysecurity.SessionStatusActive {
			return identitysecurity.Session{}, identitysecurity.Conflict("登录会话已经结束")
		}
		result, err := tx.ExecContext(ctx, `
			UPDATE mochat_go_saas_identity_sessions
			SET status = 'revoked', revoked_at = ?, revoked_by = ?, revocation_reason = ?,
				version = version + 1, updated_at = NOW()
			WHERE id = ? AND status = 'active' AND version = ?
		`, now, input.Actor.UserID, truncateRunes(input.Reason, 255), input.SessionID, before.Version)
		if err != nil {
			return identitysecurity.Session{}, err
		}
		if affected, _ := result.RowsAffected(); affected != 1 {
			return identitysecurity.Session{}, identitysecurity.Conflict("登录会话版本已变化")
		}
	} else {
		rows, err := tx.QueryContext(ctx, identitySessionSelect+` WHERE sess.user_id = ? AND sess.status = 'active' ORDER BY sess.id DESC FOR UPDATE`, input.UserID)
		if err != nil {
			return identitysecurity.Session{}, err
		}
		if rows.Next() {
			before, err = scanIdentitySession(rows)
		}
		rows.Close()
		if err != nil {
			return identitysecurity.Session{}, err
		}
		if before.ID == 0 {
			return identitysecurity.Session{}, identitysecurity.NotFound("账号没有活动会话")
		}
		_, err = tx.ExecContext(ctx, `
			UPDATE mochat_go_saas_identity_sessions
			SET status = 'revoked', revoked_at = ?, revoked_by = ?, revocation_reason = ?,
				version = version + 1, updated_at = NOW()
			WHERE user_id = ? AND status = 'active'
		`, now, input.Actor.UserID, truncateRunes(input.Reason, 255), input.UserID)
		if err != nil {
			return identitysecurity.Session{}, err
		}
	}
	after, err := identitySessionByID(ctx, tx, before.ID, false)
	if err != nil {
		return identitysecurity.Session{}, err
	}
	if err := insertIdentityLoginEventTx(ctx, tx, identitysecurity.LoginEvent{
		UserID: before.UserID, TenantID: before.TenantID, EventType: identitysecurity.EventSessionRevoked,
		Result: "succeeded", RiskLevel: identitysecurity.RiskWarning, ReasonCode: "admin_revoked", IP: before.IP,
		Metadata: map[string]any{"sessionId": before.ID, "revokedBy": input.Actor.UserID, "reason": input.Reason},
	}, "", now); err != nil {
		return identitysecurity.Session{}, err
	}
	if _, err := insertIdentityOperationTx(ctx, tx, identitysecurity.OperationSessionRevoke, before.TenantID, before.UserID, input.Actor,
		identitySessionAuditJSON(before), identitySessionAuditJSON(after), input.Reason); err != nil {
		return identitysecurity.Session{}, err
	}
	if err := tx.Commit(); err != nil {
		return identitysecurity.Session{}, err
	}
	return after, nil
}

func (s *MySQLStore) RevokeCurrentIdentitySession(ctx context.Context, jti string, userID int, reason string, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer rollbackQuietly(tx)
	var id int64
	var tenantID int
	err = tx.QueryRowContext(ctx, `
		SELECT id, tenant_id FROM mochat_go_saas_identity_sessions
		WHERE session_jti = ? AND user_id = ? AND status = 'active' FOR UPDATE
	`, jti, userID).Scan(&id, &tenantID)
	if errors.Is(err, sql.ErrNoRows) {
		return tx.Commit()
	}
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_identity_sessions
		SET status = 'revoked', revoked_at = ?, revoked_by = ?, revocation_reason = ?,
			version = version + 1, updated_at = NOW()
		WHERE id = ? AND status = 'active'
	`, now, userID, truncateRunes(reason, 255), id); err != nil {
		return err
	}
	if err := insertIdentityLoginEventTx(ctx, tx, identitysecurity.LoginEvent{
		UserID: userID, TenantID: tenantID, EventType: identitysecurity.EventSessionRevoked,
		Result: "succeeded", RiskLevel: identitysecurity.RiskNormal, ReasonCode: "user_logout",
		Metadata: map[string]any{"sessionId": id},
	}, "", now); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *MySQLStore) UnlockIdentityUser(ctx context.Context, userID, expectedVersion int, reason string, actor identitysecurity.Actor, now time.Time) (identitysecurity.UserState, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return identitysecurity.UserState{}, err
	}
	defer rollbackQuietly(tx)
	before, err := scanIdentityUserState(tx.QueryRowContext(ctx, identityUserStateSelect+` WHERE u.id = ? FOR UPDATE`, userID))
	if errors.Is(err, sql.ErrNoRows) {
		return identitysecurity.UserState{}, identitysecurity.NotFound("账号安全状态不存在")
	}
	if err != nil {
		return identitysecurity.UserState{}, err
	}
	if before.Version != expectedVersion {
		return identitysecurity.UserState{}, identitysecurity.Conflict("账号安全状态版本已变化")
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_identity_user_states
		SET failed_attempts = 0, locked_until = NULL, version = version + 1, updated_at = NOW()
		WHERE user_id = ? AND version = ?
	`, userID, expectedVersion)
	if err != nil {
		return identitysecurity.UserState{}, err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return identitysecurity.UserState{}, identitysecurity.Conflict("账号安全状态版本已变化")
	}
	after, err := scanIdentityUserState(tx.QueryRowContext(ctx, identityUserStateSelect+` WHERE u.id = ?`, userID))
	if err != nil {
		return identitysecurity.UserState{}, err
	}
	if _, err := insertIdentityOperationTx(ctx, tx, identitysecurity.OperationUserUnlock, before.TenantID, userID, actor,
		identityUserStateAuditJSON(before), identityUserStateAuditJSON(after), reason); err != nil {
		return identitysecurity.UserState{}, err
	}
	if err := tx.Commit(); err != nil {
		return identitysecurity.UserState{}, err
	}
	return after, nil
}

func (s *MySQLStore) IdentityLoginEvents(ctx context.Context, options identitysecurity.EventOptions) ([]identitysecurity.LoginEvent, error) {
	limit := options.Limit
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	where := []string{"1 = 1"}
	args := make([]any, 0, 8)
	if options.TenantID > 0 {
		where, args = append(where, "e.tenant_id = ?"), append(args, options.TenantID)
	}
	if options.UserID > 0 {
		where, args = append(where, "e.user_id = ?"), append(args, options.UserID)
	}
	if risk := strings.TrimSpace(options.Risk); risk != "" && risk != "all" {
		where, args = append(where, "e.risk_level = ?"), append(args, risk)
	}
	if result := strings.TrimSpace(options.Result); result != "" && result != "all" {
		where, args = append(where, "e.result = ?"), append(args, result)
	}
	if keyword := strings.TrimSpace(options.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		where = append(where, "(u.name LIKE ? OR t.name LIKE ? OR e.ip_address LIKE ? OR e.reason_code LIKE ? OR e.user_agent LIKE ?)")
		args = append(args, like, like, like, like, like)
	}
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, identityLoginEventSelect+` WHERE `+strings.Join(where, " AND ")+` ORDER BY e.occurred_at DESC, e.id DESC LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]identitysecurity.LoginEvent, 0)
	for rows.Next() {
		item, err := scanIdentityLoginEvent(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQLStore) IdentityIncidents(ctx context.Context, options identitysecurity.IncidentOptions) ([]identitysecurity.Incident, error) {
	limit := options.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	where := []string{"1 = 1"}
	args := make([]any, 0, 8)
	if options.TenantID > 0 {
		where, args = append(where, "i.tenant_id = ?"), append(args, options.TenantID)
	}
	if options.UserID > 0 {
		where, args = append(where, "i.user_id = ?"), append(args, options.UserID)
	}
	if status := strings.TrimSpace(options.Status); status != "" && status != "all" {
		where, args = append(where, "i.status = ?"), append(args, status)
	}
	if severity := strings.TrimSpace(options.Severity); severity != "" && severity != "all" {
		where, args = append(where, "i.severity = ?"), append(args, severity)
	}
	if keyword := strings.TrimSpace(options.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		where = append(where, "(i.incident_no LIKE ? OR i.title LIKE ? OR i.latest_detail LIKE ? OR i.assigned_to LIKE ? OR u.name LIKE ? OR t.name LIKE ?)")
		args = append(args, like, like, like, like, like, like)
	}
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, identityIncidentSelect+` WHERE `+strings.Join(where, " AND ")+` ORDER BY FIELD(i.status,'open','acknowledged','resolved'), FIELD(i.severity,'critical','warning','normal'), i.last_occurred_at DESC, i.id DESC LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]identitysecurity.Incident, 0)
	for rows.Next() {
		item, err := scanIdentityIncident(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *MySQLStore) UpdateIdentityIncident(ctx context.Context, input identitysecurity.IncidentUpdate, now time.Time) (identitysecurity.Incident, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return identitysecurity.Incident{}, err
	}
	defer rollbackQuietly(tx)
	before, err := identityIncidentByID(ctx, tx, input.ID, true)
	if errors.Is(err, sql.ErrNoRows) {
		return identitysecurity.Incident{}, identitysecurity.NotFound("身份安全事故不存在")
	}
	if err != nil {
		return identitysecurity.Incident{}, err
	}
	if before.Version != input.ExpectedVersion {
		return identitysecurity.Incident{}, identitysecurity.Conflict("身份安全事故版本已变化")
	}
	query := ""
	args := []any{}
	switch input.Action {
	case "acknowledge":
		query = `UPDATE mochat_go_saas_identity_security_incidents SET status = 'acknowledged', acknowledged_at = ?, acknowledged_by = ?, version = version + 1, updated_at = NOW() WHERE id = ? AND version = ?`
		args = []any{now, input.Actor.UserID, input.ID, input.ExpectedVersion}
	case "assign":
		query = `UPDATE mochat_go_saas_identity_security_incidents SET assigned_to = ?, version = version + 1, updated_at = NOW() WHERE id = ? AND version = ?`
		args = []any{truncateRunes(input.AssignedTo, 80), input.ID, input.ExpectedVersion}
	case "resolve":
		query = `UPDATE mochat_go_saas_identity_security_incidents SET status = 'resolved', resolved_at = ?, resolved_by = ?, resolution = ?, version = version + 1, updated_at = NOW() WHERE id = ? AND version = ?`
		args = []any{now, input.Actor.UserID, truncateRunes(input.Resolution, 500), input.ID, input.ExpectedVersion}
	case "reopen":
		query = `UPDATE mochat_go_saas_identity_security_incidents SET status = 'open', acknowledged_at = NULL, acknowledged_by = 0, resolved_at = NULL, resolved_by = 0, resolution = '', version = version + 1, updated_at = NOW() WHERE id = ? AND version = ?`
		args = []any{input.ID, input.ExpectedVersion}
	}
	result, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return identitysecurity.Incident{}, err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return identitysecurity.Incident{}, identitysecurity.Conflict("身份安全事故版本已变化")
	}
	after, err := identityIncidentByID(ctx, tx, input.ID, false)
	if err != nil {
		return identitysecurity.Incident{}, err
	}
	if _, err := insertIdentityOperationTx(ctx, tx, identitysecurity.OperationIncidentUpdate, before.TenantID, before.UserID, input.Actor,
		identityIncidentAuditJSON(before), identityIncidentAuditJSON(after), input.Action); err != nil {
		return identitysecurity.Incident{}, err
	}
	if err := tx.Commit(); err != nil {
		return identitysecurity.Incident{}, err
	}
	return after, nil
}

func (s *MySQLStore) CleanupIdentitySecurity(ctx context.Context, now time.Time, limit int) (identitysecurity.CleanupResult, error) {
	result := identitysecurity.CleanupResult{}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	defer rollbackQuietly(tx)
	res, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_identity_auth_challenges
		SET status = 'expired', active_slot = NULL, updated_at = NOW()
		WHERE status = 'pending' AND expires_at <= ? LIMIT ?
	`, now, limit)
	if err != nil {
		return result, err
	}
	affected, _ := res.RowsAffected()
	result.ExpiredChallenges = int(affected)
	res, err = tx.ExecContext(ctx, `
		UPDATE mochat_go_saas_identity_sessions
		SET status = 'expired', version = version + 1, updated_at = NOW()
		WHERE status = 'active' AND (expires_at <= ? OR idle_expires_at <= ?) LIMIT ?
	`, now, now, limit)
	if err != nil {
		return result, err
	}
	affected, _ = res.RowsAffected()
	result.ExpiredSessions = int(affected)
	res, err = tx.ExecContext(ctx, `
		DELETE FROM mochat_go_saas_identity_auth_challenges
		WHERE id IN (
			SELECT id FROM (
				SELECT c.id FROM mochat_go_saas_identity_auth_challenges c
				WHERE c.status <> 'pending' AND c.updated_at < DATE_SUB(?, INTERVAL 7 DAY)
				ORDER BY c.id ASC LIMIT ?
			) candidates
		)
	`, now, limit)
	if err != nil {
		return result, err
	}
	affected, _ = res.RowsAffected()
	result.DeletedChallenges = int(affected)
	res, err = tx.ExecContext(ctx, `
		DELETE FROM mochat_go_saas_identity_sessions
		WHERE id IN (
			SELECT id FROM (
				SELECT sess.id FROM mochat_go_saas_identity_sessions sess
				LEFT JOIN mochat_go_saas_identity_policies p ON p.tenant_id = sess.tenant_id
				WHERE sess.status <> 'active'
					AND sess.updated_at < DATE_SUB(?, INTERVAL COALESCE(p.session_retention_days, 90) DAY)
				ORDER BY sess.id ASC LIMIT ?
			) candidates
		)
	`, now, limit)
	if err != nil {
		return result, err
	}
	affected, _ = res.RowsAffected()
	result.DeletedSessions = int(affected)
	res, err = tx.ExecContext(ctx, `
		DELETE FROM mochat_go_saas_identity_login_events
		WHERE id IN (
			SELECT id FROM (
				SELECT e.id FROM mochat_go_saas_identity_login_events e
				LEFT JOIN mochat_go_saas_identity_policies p ON p.tenant_id = e.tenant_id
				WHERE e.occurred_at < DATE_SUB(?, INTERVAL COALESCE(p.login_event_retention_days, 180) DAY)
				ORDER BY e.id ASC LIMIT ?
			) candidates
		)
	`, now, limit)
	if err != nil {
		return result, err
	}
	affected, _ = res.RowsAffected()
	result.DeletedEvents = int(affected)
	if err := tx.Commit(); err != nil {
		return result, err
	}
	return result, nil
}

const identityUserStateSelect = `
	SELECT u.id, u.tenant_id, u.name, u.phone, u.status,
		COALESCE(st.failed_attempts, 0), st.locked_until, st.last_failed_at,
		COALESCE(st.last_failed_ip, ''), st.last_success_at, COALESCE(st.last_success_ip, ''),
		COALESCE(m.status, ''), COALESCE(m.recovery_codes_remaining, 0), COALESCE(m.version, 0),
		(SELECT COUNT(*) FROM mochat_go_saas_identity_sessions sess
		 WHERE sess.user_id = u.id AND sess.status = 'active' AND sess.expires_at > NOW() AND sess.idle_expires_at > NOW()),
		COALESCE(st.version, 0), st.updated_at
	FROM mc_user u
	LEFT JOIN mochat_go_saas_identity_user_states st ON st.user_id = u.id
	LEFT JOIN mochat_go_saas_identity_mfa_credentials m ON m.user_id = u.id
`

const identityMFACredentialSelect = `
	SELECT id, user_id, tenant_id, status, secret_ciphertext, encryption_key_id,
		COALESCE(CAST(recovery_code_hashes AS CHAR), '[]'), recovery_codes_remaining,
		verified_at, last_used_at, last_totp_step, disabled_at, disabled_by, disabled_reason,
		version, created_at, updated_at
	FROM mochat_go_saas_identity_mfa_credentials
`

const identityChallengeSelect = `
	SELECT c.id, c.user_id, c.tenant_id, c.status, c.attempts, c.max_attempts,
		c.ip_address, c.user_agent, c.expires_at, c.consumed_at,
		m.id, m.user_id, m.tenant_id, m.status, m.secret_ciphertext, m.encryption_key_id,
		COALESCE(CAST(m.recovery_code_hashes AS CHAR), '[]'), m.recovery_codes_remaining,
		m.verified_at, m.last_used_at, m.last_totp_step, m.disabled_at, m.disabled_by,
		m.disabled_reason, m.version, m.created_at, m.updated_at
	FROM mochat_go_saas_identity_auth_challenges c
	INNER JOIN mochat_go_saas_identity_mfa_credentials m ON m.user_id = c.user_id
`

const identitySessionSelect = `
	SELECT sess.id, sess.session_jti, sess.user_id, COALESCE(u.name, ''), COALESCE(u.phone, ''),
		sess.tenant_id, COALESCE(t.name, ''), sess.status, sess.auth_method, sess.ip_address,
		sess.user_agent, sess.issued_at, sess.expires_at, sess.idle_expires_at, sess.last_seen_at,
		sess.revoked_at, sess.revoked_by, sess.revocation_reason, sess.version, sess.created_at, sess.updated_at
	FROM mochat_go_saas_identity_sessions sess
	LEFT JOIN mc_user u ON u.id = sess.user_id
	LEFT JOIN mc_tenant t ON t.id = sess.tenant_id
`

const identityLoginEventSelect = `
	SELECT e.id, e.user_id, COALESCE(u.name, ''), e.tenant_id, COALESCE(t.name, ''),
		e.event_type, e.result, e.risk_level, e.reason_code, e.ip_address, e.user_agent,
		COALESCE(CAST(e.metadata_json AS CHAR), '{}'), e.occurred_at
	FROM mochat_go_saas_identity_login_events e
	LEFT JOIN mc_user u ON u.id = e.user_id
	LEFT JOIN mc_tenant t ON t.id = e.tenant_id
`

const identityIncidentSelect = `
	SELECT i.id, i.incident_no, i.tenant_id, COALESCE(t.name, ''), i.user_id,
		COALESCE(u.name, ''), i.incident_type, i.severity, i.status, i.title,
		i.latest_detail, i.occurrence_count, i.first_occurred_at, i.last_occurred_at,
		i.assigned_to, i.acknowledged_at, i.acknowledged_by, i.resolved_at,
		i.resolved_by, i.resolution, i.version, i.created_at, i.updated_at
	FROM mochat_go_saas_identity_security_incidents i
	LEFT JOIN mc_user u ON u.id = i.user_id
	LEFT JOIN mc_tenant t ON t.id = i.tenant_id
`

func (s *MySQLStore) identityUsers(ctx context.Context, tenantID, limit int) ([]identitysecurity.UserState, error) {
	rows, err := s.db.QueryContext(ctx, identityUserStateSelect+`
		WHERE u.tenant_id = ? AND u.deleted_at IS NULL
		ORDER BY CASE WHEN st.locked_until > NOW() THEN 0 ELSE 1 END, u.id ASC LIMIT ?
	`, tenantID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]identitysecurity.UserState, 0)
	for rows.Next() {
		item, err := scanIdentityUserState(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func scanIdentityPolicy(scanner interface{ Scan(...any) error }) (identitysecurity.Policy, error) {
	var item identitysecurity.Policy
	var requireMFA int
	var cidrsJSON string
	var createdAt, updatedAt sql.NullTime
	err := scanner.Scan(&item.TenantID, &item.TenantName, &item.Status, &item.MaxFailedAttempts, &item.LockoutMinutes,
		&item.SessionTTLMinutes, &item.IdleTimeoutMinutes, &item.MaxConcurrentSessions, &requireMFA,
		&cidrsJSON, &item.LoginEventRetentionDays, &item.SessionRetentionDays, &item.Version, &item.UpdatedBy, &createdAt, &updatedAt)
	if err != nil {
		return item, err
	}
	item.RequireMFA = requireMFA != 0
	_ = json.Unmarshal([]byte(cidrsJSON), &item.AllowedIPCIDRs)
	if item.AllowedIPCIDRs == nil {
		item.AllowedIPCIDRs = []string{}
	}
	item.CreatedAt, item.UpdatedAt = formatTime(createdAt), formatTime(updatedAt)
	return item, nil
}

func scanIdentityUserState(scanner interface{ Scan(...any) error }) (identitysecurity.UserState, error) {
	var item identitysecurity.UserState
	var lockedUntil, failedAt, successAt, updatedAt sql.NullTime
	err := scanner.Scan(&item.UserID, &item.TenantID, &item.UserName, &item.Phone, &item.UserStatus,
		&item.FailedAttempts, &lockedUntil, &failedAt, &item.LastFailedIP, &successAt, &item.LastSuccessIP,
		&item.MFAStatus, &item.MFARecoveryCodes, &item.MFAVersion, &item.ActiveSessions, &item.Version, &updatedAt)
	if err != nil {
		return item, err
	}
	if lockedUntil.Valid {
		item.LockedUntilValue = lockedUntil.Time
	}
	item.LockedUntil, item.LastFailedAt, item.LastSuccessAt, item.UpdatedAt = formatTime(lockedUntil), formatTime(failedAt), formatTime(successAt), formatTime(updatedAt)
	return item, nil
}

func scanIdentityMFACredential(scanner interface{ Scan(...any) error }) (identitysecurity.MFACredential, error) {
	var item identitysecurity.MFACredential
	var verifiedAt, lastUsedAt, disabledAt, createdAt, updatedAt sql.NullTime
	err := scanner.Scan(&item.ID, &item.UserID, &item.TenantID, &item.Status, &item.SecretCiphertext,
		&item.EncryptionKeyID, &item.RecoveryCodeHashesJSON, &item.RecoveryCodesRemaining,
		&verifiedAt, &lastUsedAt, &item.LastTOTPStep, &disabledAt, &item.DisabledBy,
		&item.DisabledReason, &item.Version, &createdAt, &updatedAt)
	if err != nil {
		return item, err
	}
	item.VerifiedAt, item.LastUsedAt, item.DisabledAt = formatTime(verifiedAt), formatTime(lastUsedAt), formatTime(disabledAt)
	item.CreatedAt, item.UpdatedAt = formatTime(createdAt), formatTime(updatedAt)
	return item, nil
}

func identityChallengeByHash(ctx context.Context, queryer saasAdminAccessQueryer, hash string, forUpdate bool) (identitysecurity.Challenge, error) {
	query := identityChallengeSelect + ` WHERE c.challenge_hash = ?`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	return scanIdentityChallenge(queryer.QueryRowContext(ctx, query, hash))
}

func identityChallengeByID(ctx context.Context, queryer saasAdminAccessQueryer, id int64, forUpdate bool) (identitysecurity.Challenge, error) {
	query := identityChallengeSelect + ` WHERE c.id = ?`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	return scanIdentityChallenge(queryer.QueryRowContext(ctx, query, id))
}

func scanIdentityChallenge(scanner interface{ Scan(...any) error }) (identitysecurity.Challenge, error) {
	var item identitysecurity.Challenge
	var consumedAt sql.NullTime
	var verifiedAt, lastUsedAt, disabledAt, createdAt, updatedAt sql.NullTime
	err := scanner.Scan(&item.ID, &item.UserID, &item.TenantID, &item.Status, &item.Attempts, &item.MaxAttempts,
		&item.IP, &item.UserAgent, &item.ExpiresAt, &consumedAt,
		&item.Credential.ID, &item.Credential.UserID, &item.Credential.TenantID, &item.Credential.Status,
		&item.Credential.SecretCiphertext, &item.Credential.EncryptionKeyID,
		&item.Credential.RecoveryCodeHashesJSON, &item.Credential.RecoveryCodesRemaining,
		&verifiedAt, &lastUsedAt, &item.Credential.LastTOTPStep, &disabledAt,
		&item.Credential.DisabledBy, &item.Credential.DisabledReason, &item.Credential.Version, &createdAt, &updatedAt)
	if err != nil {
		return item, err
	}
	item.ConsumedAt = formatTime(consumedAt)
	item.Credential.VerifiedAt, item.Credential.LastUsedAt = formatTime(verifiedAt), formatTime(lastUsedAt)
	item.Credential.DisabledAt = formatTime(disabledAt)
	item.Credential.CreatedAt, item.Credential.UpdatedAt = formatTime(createdAt), formatTime(updatedAt)
	return item, nil
}

func identitySessionByID(ctx context.Context, queryer saasAdminAccessQueryer, id int64, forUpdate bool) (identitysecurity.Session, error) {
	query := identitySessionSelect + ` WHERE sess.id = ?`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	return scanIdentitySession(queryer.QueryRowContext(ctx, query, id))
}

func scanIdentitySession(scanner interface{ Scan(...any) error }) (identitysecurity.Session, error) {
	var item identitysecurity.Session
	var issuedAt, expiresAt, idleExpiresAt, lastSeenAt time.Time
	var revokedAt, createdAt, updatedAt sql.NullTime
	err := scanner.Scan(&item.ID, &item.SessionJTI, &item.UserID, &item.UserName, &item.Phone,
		&item.TenantID, &item.TenantName, &item.Status, &item.AuthMethod, &item.IP, &item.UserAgent,
		&issuedAt, &expiresAt, &idleExpiresAt, &lastSeenAt, &revokedAt, &item.RevokedBy,
		&item.RevocationReason, &item.Version, &createdAt, &updatedAt)
	if err != nil {
		return item, err
	}
	item.IssuedAt, item.ExpiresAt = formatRequiredTime(issuedAt), formatRequiredTime(expiresAt)
	item.IdleExpiresAt, item.LastSeenAt = formatRequiredTime(idleExpiresAt), formatRequiredTime(lastSeenAt)
	item.RevokedAt, item.CreatedAt, item.UpdatedAt = formatTime(revokedAt), formatTime(createdAt), formatTime(updatedAt)
	return item, nil
}

func scanIdentityLoginEvent(scanner interface{ Scan(...any) error }) (identitysecurity.LoginEvent, error) {
	var item identitysecurity.LoginEvent
	var metadataJSON string
	var occurredAt time.Time
	err := scanner.Scan(&item.ID, &item.UserID, &item.UserName, &item.TenantID, &item.TenantName,
		&item.EventType, &item.Result, &item.RiskLevel, &item.ReasonCode, &item.IP, &item.UserAgent,
		&metadataJSON, &occurredAt)
	if err != nil {
		return item, err
	}
	_ = json.Unmarshal([]byte(metadataJSON), &item.Metadata)
	if item.Metadata == nil {
		item.Metadata = map[string]any{}
	}
	item.OccurredAt = formatRequiredTime(occurredAt)
	return item, nil
}

func identityIncidentByID(ctx context.Context, queryer saasAdminAccessQueryer, id int64, forUpdate bool) (identitysecurity.Incident, error) {
	query := identityIncidentSelect + ` WHERE i.id = ?`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	return scanIdentityIncident(queryer.QueryRowContext(ctx, query, id))
}

func scanIdentityIncident(scanner interface{ Scan(...any) error }) (identitysecurity.Incident, error) {
	var item identitysecurity.Incident
	var firstAt, lastAt time.Time
	var acknowledgedAt, resolvedAt, createdAt, updatedAt sql.NullTime
	err := scanner.Scan(&item.ID, &item.IncidentNo, &item.TenantID, &item.TenantName, &item.UserID,
		&item.UserName, &item.IncidentType, &item.Severity, &item.Status, &item.Title,
		&item.LatestDetail, &item.OccurrenceCount, &firstAt, &lastAt, &item.AssignedTo,
		&acknowledgedAt, &item.AcknowledgedBy, &resolvedAt, &item.ResolvedBy, &item.Resolution,
		&item.Version, &createdAt, &updatedAt)
	if err != nil {
		return item, err
	}
	item.FirstOccurredAt, item.LastOccurredAt = formatRequiredTime(firstAt), formatRequiredTime(lastAt)
	item.AcknowledgedAt, item.ResolvedAt = formatTime(acknowledgedAt), formatTime(resolvedAt)
	item.CreatedAt, item.UpdatedAt = formatTime(createdAt), formatTime(updatedAt)
	return item, nil
}

func insertIdentityLoginEventTx(ctx context.Context, tx *sql.Tx, event identitysecurity.LoginEvent, phoneHash string, now time.Time) error {
	metadata, err := json.Marshal(event.Metadata)
	if err != nil {
		return err
	}
	if event.Metadata == nil {
		metadata = []byte(`{}`)
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO mochat_go_saas_identity_login_events
			(user_id, tenant_id, phone_sha256, event_type, result, risk_level, reason_code,
			 ip_address, user_agent, metadata_json, occurred_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW())
	`, event.UserID, event.TenantID, phoneHash, truncateRunes(event.EventType, 32), truncateRunes(event.Result, 16),
		truncateRunes(event.RiskLevel, 16), truncateRunes(event.ReasonCode, 64), truncateRunes(event.IP, 64),
		truncateRunes(event.UserAgent, 500), string(metadata), now)
	return err
}

type identityIncidentInput struct {
	StableKey string
	TenantID  int
	UserID    int
	Type      string
	Severity  string
	Title     string
	Detail    string
	Now       time.Time
}

func upsertIdentityIncidentTx(ctx context.Context, tx *sql.Tx, input identityIncidentInput) error {
	digest := sha256Text(input.StableKey + "\x00" + input.Now.UTC().Format(time.RFC3339Nano))
	incidentNo := "SEC-" + strings.ToUpper(digest[:24])
	_, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_saas_identity_security_incidents
			(incident_no, stable_key, tenant_id, user_id, incident_type, severity, status,
			 title, latest_detail, occurrence_count, first_occurred_at, last_occurred_at,
			 assigned_to, acknowledged_at, acknowledged_by, resolved_at, resolved_by,
			 resolution, version, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, 'open', ?, ?, 1, ?, ?, '', NULL, 0, NULL, 0, '', 1, NOW(), NOW())
		ON DUPLICATE KEY UPDATE tenant_id = VALUES(tenant_id), user_id = VALUES(user_id),
			incident_type = VALUES(incident_type), severity = VALUES(severity), status = 'open',
			title = VALUES(title), latest_detail = VALUES(latest_detail),
			occurrence_count = occurrence_count + 1, last_occurred_at = VALUES(last_occurred_at),
			resolved_at = NULL, resolved_by = 0, resolution = '', version = version + 1, updated_at = NOW()
	`, incidentNo, input.StableKey, input.TenantID, input.UserID, input.Type, input.Severity,
		truncateRunes(input.Title, 255), truncateRunes(input.Detail, 1000), input.Now, input.Now)
	return err
}

func insertIdentityOperationTx(ctx context.Context, tx *sql.Tx, action string, tenantID, userID int, actor identitysecurity.Actor, before, after, remark string) (int64, error) {
	return insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID: tenantID, ActorUserID: actor.UserID, ActorTenantID: actor.TenantID,
		Action: action, TargetType: "identity_user", TargetID: strconv.Itoa(userID),
		TargetName: "身份安全账号 #" + strconv.Itoa(userID), BeforeJSON: before, AfterJSON: after, Remark: remark,
	})
}

func identityPolicyAuditJSON(item identitysecurity.Policy) string {
	payload := map[string]any{
		"tenantId": item.TenantID, "status": item.Status, "maxFailedAttempts": item.MaxFailedAttempts,
		"lockoutMinutes": item.LockoutMinutes, "sessionTtlMinutes": item.SessionTTLMinutes,
		"idleTimeoutMinutes": item.IdleTimeoutMinutes, "maxConcurrentSessions": item.MaxConcurrentSessions,
		"requireMfa": item.RequireMFA, "allowedIpCidrs": item.AllowedIPCIDRs,
		"loginEventRetentionDays": item.LoginEventRetentionDays, "sessionRetentionDays": item.SessionRetentionDays,
		"version": item.Version,
	}
	encoded, _ := json.Marshal(payload)
	return string(encoded)
}

func mfaAuditJSON(item identitysecurity.MFACredential) string {
	payload := map[string]any{
		"userId": item.UserID, "tenantId": item.TenantID, "status": item.Status,
		"encryptionKeyId": item.EncryptionKeyID, "recoveryCodesRemaining": item.RecoveryCodesRemaining,
		"verifiedAt": item.VerifiedAt, "lastUsedAt": item.LastUsedAt, "version": item.Version,
	}
	encoded, _ := json.Marshal(payload)
	return string(encoded)
}

func mfaResetAuditJSON(item identitysecurity.MFACredential, revokedSessions, userStatus int) string {
	payload := map[string]any{
		"userId": item.UserID, "tenantId": item.TenantID, "status": item.Status,
		"encryptionKeyId": item.EncryptionKeyID, "recoveryCodesRemaining": item.RecoveryCodesRemaining,
		"verifiedAt": item.VerifiedAt, "lastUsedAt": item.LastUsedAt, "version": item.Version,
		"revokedSessions": revokedSessions, "userStatus": userStatus,
	}
	encoded, _ := json.Marshal(payload)
	return string(encoded)
}

func foundMFAAudit(item identitysecurity.MFACredential, found bool) string {
	if !found {
		return ""
	}
	return mfaAuditJSON(item)
}

func identitySessionAuditJSON(item identitysecurity.Session) string {
	payload := map[string]any{
		"id": item.ID, "userId": item.UserID, "tenantId": item.TenantID, "status": item.Status,
		"authMethod": item.AuthMethod, "ipAddress": item.IP, "issuedAt": item.IssuedAt,
		"expiresAt": item.ExpiresAt, "lastSeenAt": item.LastSeenAt, "revocationReason": item.RevocationReason,
		"version": item.Version,
	}
	encoded, _ := json.Marshal(payload)
	return string(encoded)
}

func identityUserStateAuditJSON(item identitysecurity.UserState) string {
	payload := map[string]any{
		"userId": item.UserID, "tenantId": item.TenantID, "failedAttempts": item.FailedAttempts,
		"lockedUntil": item.LockedUntil, "lastFailedAt": item.LastFailedAt, "version": item.Version,
	}
	encoded, _ := json.Marshal(payload)
	return string(encoded)
}

func identityIncidentAuditJSON(item identitysecurity.Incident) string {
	payload := map[string]any{
		"id": item.ID, "tenantId": item.TenantID, "userId": item.UserID, "type": item.IncidentType,
		"severity": item.Severity, "status": item.Status, "assignedTo": item.AssignedTo,
		"resolution": item.Resolution, "version": item.Version,
	}
	encoded, _ := json.Marshal(payload)
	return string(encoded)
}

func identityInQuery(format string, prefix []any, ids []int64) (string, []any) {
	ids = append([]int64(nil), ids...)
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	placeholders := make([]string, len(ids))
	args := append([]any(nil), prefix...)
	for index, id := range ids {
		placeholders[index] = "?"
		args = append(args, id)
	}
	return fmt.Sprintf(format, strings.Join(placeholders, ",")), args
}

func formatRequiredTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format("2006-01-02 15:04:05")
}

func sha256Text(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}
