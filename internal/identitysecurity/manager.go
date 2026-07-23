package identitysecurity

import (
	"context"
	"crypto/hmac"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"

	"jiyi/mochat-go/internal/clientip"
)

type Config struct {
	EncryptionKey     string
	EncryptionKeys    string
	EncryptionKeyID   string
	Issuer            string
	EnforceSessions   bool
	TrustProxyHeaders bool
	TrustedProxyCIDRs []string
	Now               func() time.Time
}

type Manager struct {
	store            Store
	keys             map[string][]byte
	activeKeyID      string
	issuer           string
	enforceSessions  bool
	clientIPResolver *clientip.Resolver
	now              func() time.Time
}

func NewManager(store Store, config Config) (*Manager, error) {
	if store == nil {
		return nil, errors.New("identity security store is required")
	}
	keys, activeID, err := parseIdentityKeys(config.EncryptionKey, config.EncryptionKeys, config.EncryptionKeyID)
	if err != nil {
		return nil, err
	}
	clientIPResolver, err := clientip.NewResolver(clientip.Config{
		TrustProxyHeaders: config.TrustProxyHeaders,
		TrustedProxyCIDRs: config.TrustedProxyCIDRs,
	})
	if err != nil {
		return nil, fmt.Errorf("identity client IP resolver: %w", err)
	}
	now := config.Now
	if now == nil {
		now = time.Now
	}
	issuer := strings.TrimSpace(config.Issuer)
	if issuer == "" {
		issuer = "MoChat Go"
	}
	return &Manager{
		store: store, keys: keys, activeKeyID: activeID, issuer: issuer,
		enforceSessions: config.EnforceSessions, clientIPResolver: clientIPResolver, now: now,
	}, nil
}

func (m *Manager) ConfigStatus() ConfigStatus {
	clientIPStatus := m.clientIPResolver.Status()
	return ConfigStatus{
		Enabled: true, SessionEnforced: m.enforceSessions, MFAEncryptionReady: len(m.keys) > 0,
		MFAEncryptionKeyID: m.activeKeyID, MFAEncryptionKeys: len(m.keys),
		TrustedProxyHeaders: clientIPStatus.TrustProxyHeaders,
		TrustedProxyCIDRs:   clientIPStatus.TrustedProxyCIDRs, TrustedProxyCIDRCount: clientIPStatus.TrustedProxyCIDRCount,
	}
}

func (m *Manager) CheckConfiguration(context.Context) error {
	if len(m.keys) == 0 {
		return errors.New("MFA encryption key is not configured")
	}
	if _, ok := m.keys[m.activeKeyID]; !ok {
		return errors.New("active MFA encryption key is unavailable")
	}
	return nil
}

func DefaultPolicy(tenantID int) Policy {
	return Policy{
		TenantID: tenantID, Status: PolicyStatusActive, MaxFailedAttempts: 5, LockoutMinutes: 30,
		SessionTTLMinutes: 7 * 24 * 60, IdleTimeoutMinutes: 24 * 60, MaxConcurrentSessions: 5,
		AllowedIPCIDRs: []string{}, LoginEventRetentionDays: 180, SessionRetentionDays: 90, Version: 0,
	}
}

func (m *Manager) Policy(ctx context.Context, tenantID int) (Policy, error) {
	if tenantID <= 0 {
		return Policy{}, Invalid("tenantId 必须大于 0")
	}
	policy, found, err := m.store.IdentityPolicy(ctx, tenantID)
	if err != nil {
		return Policy{}, err
	}
	if !found {
		policy = DefaultPolicy(tenantID)
	}
	active, enrolled, err := m.store.IdentityMFAEnrollmentCoverage(ctx, tenantID)
	if err != nil {
		return Policy{}, err
	}
	policy.ActiveUserCount = active
	policy.MFAEnrolledUserCount = enrolled
	policy.MFAMissingUserCount = active - enrolled
	if policy.MFAMissingUserCount < 0 {
		policy.MFAMissingUserCount = 0
	}
	return policy, nil
}

func (m *Manager) ValidatePolicyUpdate(ctx context.Context, input PolicyUpdate) (PolicyUpdate, error) {
	input.Status = strings.ToLower(strings.TrimSpace(input.Status))
	input.AllowedIPCIDRs = normalizeCIDRs(input.AllowedIPCIDRs)
	if input.TenantID <= 0 || input.ExpectedVersion < 0 || (input.Status != PolicyStatusActive && input.Status != PolicyStatusDisabled) {
		return PolicyUpdate{}, Invalid("tenantId、expectedVersion 或策略状态无效")
	}
	if input.MaxFailedAttempts < 3 || input.MaxFailedAttempts > 20 || input.LockoutMinutes < 1 || input.LockoutMinutes > 1440 {
		return PolicyUpdate{}, Invalid("失败上限必须为 3 至 20 次，锁定时间必须为 1 至 1440 分钟")
	}
	if input.SessionTTLMinutes < 15 || input.SessionTTLMinutes > 43200 || input.IdleTimeoutMinutes < 5 || input.IdleTimeoutMinutes > input.SessionTTLMinutes {
		return PolicyUpdate{}, Invalid("会话有效期必须为 15 至 43200 分钟，空闲超时必须为 5 分钟至会话有效期")
	}
	if input.MaxConcurrentSessions < 1 || input.MaxConcurrentSessions > 50 {
		return PolicyUpdate{}, Invalid("并发会话上限必须为 1 至 50")
	}
	if input.LoginEventRetentionDays < 30 || input.LoginEventRetentionDays > 3650 || input.SessionRetentionDays < 7 || input.SessionRetentionDays > 3650 {
		return PolicyUpdate{}, Invalid("登录事件保留期必须为 30 至 3650 天，会话保留期必须为 7 至 3650 天")
	}
	for _, value := range input.AllowedIPCIDRs {
		if _, err := netip.ParsePrefix(value); err != nil {
			return PolicyUpdate{}, Invalid("IP 白名单必须使用有效 CIDR: " + value)
		}
	}
	if input.RequireMFA {
		active, enrolled, err := m.store.IdentityMFAEnrollmentCoverage(ctx, input.TenantID)
		if err != nil {
			return PolicyUpdate{}, err
		}
		if active > enrolled {
			return PolicyUpdate{}, Conflict(fmt.Sprintf("仍有 %d 个启用账号未完成 MFA，不能强制启用", active-enrolled))
		}
		if len(m.keys) == 0 {
			return PolicyUpdate{}, Unavailable("MFA 加密密钥未配置")
		}
	}
	return input, nil
}

func (m *Manager) PlanPolicyUpdate(ctx context.Context, input PolicyUpdate) (PolicyUpdate, Policy, error) {
	normalized, err := m.ValidatePolicyUpdate(ctx, input)
	if err != nil {
		return PolicyUpdate{}, Policy{}, err
	}
	current, err := m.Policy(ctx, normalized.TenantID)
	if err != nil {
		return PolicyUpdate{}, Policy{}, err
	}
	if current.Version != normalized.ExpectedVersion {
		return PolicyUpdate{}, Policy{}, Conflict("身份安全策略版本已变化，请刷新后重试")
	}
	return normalized, current, nil
}

func (m *Manager) UpdatePolicy(ctx context.Context, input PolicyUpdate) (Policy, error) {
	normalized, err := m.ValidatePolicyUpdate(ctx, input)
	if err != nil {
		return Policy{}, err
	}
	return m.store.UpdateIdentityPolicy(ctx, normalized)
}

func (m *Manager) CheckPasswordLogin(ctx context.Context, principal Principal, meta RequestMeta) (Policy, error) {
	policy, err := m.Policy(ctx, principal.TenantID)
	if err != nil {
		return Policy{}, err
	}
	if policy.Status == PolicyStatusDisabled {
		return policy, nil
	}
	state, found, err := m.store.IdentityUserState(ctx, principal.UserID)
	if err != nil {
		return Policy{}, err
	}
	now := m.now()
	if found && !state.LockedUntilValue.IsZero() && now.Before(state.LockedUntilValue) {
		_ = m.recordEvent(ctx, principal, meta, EventLoginBlocked, "blocked", RiskCritical, "account_locked", nil)
		return Policy{}, Forbidden("账户已临时锁定，请稍后重试")
	}
	if len(policy.AllowedIPCIDRs) > 0 && !ipAllowed(meta.IP, policy.AllowedIPCIDRs) {
		_ = m.recordEvent(ctx, principal, meta, EventLoginBlocked, "blocked", RiskCritical, "ip_not_allowed", nil)
		return Policy{}, Forbidden("当前网络不在登录白名单")
	}
	return policy, nil
}

func (m *Manager) RecordUnknownPasswordFailure(ctx context.Context, phone string, meta RequestMeta) error {
	return m.store.RecordIdentityLoginEvent(ctx, LoginEvent{
		EventType: EventPasswordFailed, Result: "failed", RiskLevel: RiskWarning, ReasonCode: "invalid_credentials", IP: meta.IP, UserAgent: meta.UserAgent,
	}, m.phoneHash(phone), m.now())
}

func (m *Manager) RecordPasswordFailure(ctx context.Context, principal Principal, phone string, meta RequestMeta, policy Policy) (UserState, error) {
	if policy.Status == PolicyStatusDisabled {
		_ = m.RecordUnknownPasswordFailure(ctx, phone, meta)
		return UserState{}, nil
	}
	return m.store.RecordIdentityLoginFailure(ctx, LoginFailure{
		Principal: principal, PhoneHash: m.phoneHash(phone), Meta: meta, Policy: policy, Reason: "invalid_credentials", Now: m.now(),
	})
}

func (m *Manager) PasswordAccepted(ctx context.Context, principal Principal, phone string, meta RequestMeta, policy Policy) (AuthDecision, error) {
	credential, found, err := m.store.IdentityMFACredential(ctx, principal.UserID)
	if err != nil {
		return AuthDecision{}, err
	}
	mfaRequired := found && credential.Status == MFAStatusActive
	if policy.RequireMFA && !mfaRequired {
		return AuthDecision{}, Forbidden("该账号尚未完成 MFA，管理员必须先解除强制策略或重置账号")
	}
	eventType := EventPasswordVerified
	if mfaRequired {
		eventType = EventMFARequired
	}
	newIP, err := m.store.RecordIdentityLoginSuccess(ctx, LoginSuccess{
		Principal: principal, PhoneHash: m.phoneHash(phone), Meta: meta, EventType: eventType,
		Risk: RiskNormal, Reason: "password_verified", Now: m.now(),
	})
	if err != nil {
		return AuthDecision{}, err
	}
	decision := AuthDecision{Policy: policy, MFARequired: mfaRequired, AuthMethod: AuthMethodPassword, NewIP: newIP}
	if !mfaRequired {
		return decision, nil
	}
	token, err := randomToken(32)
	if err != nil {
		return AuthDecision{}, err
	}
	expiresAt := m.now().Add(5 * time.Minute)
	if _, err := m.store.CreateIdentityChallenge(ctx, sha256Hex(token), principal, meta, expiresAt); err != nil {
		return AuthDecision{}, err
	}
	decision.ChallengeToken = token
	decision.ChallengeExpiry = expiresAt
	return decision, nil
}

func (m *Manager) CompleteMFA(ctx context.Context, challengeToken, code string, meta RequestMeta) (MFACompletion, error) {
	return m.completeMFA(ctx, challengeToken, code, meta, 0)
}

func (m *Manager) CompleteMFAForTenant(ctx context.Context, challengeToken, code string, meta RequestMeta, tenantID int) (MFACompletion, error) {
	if tenantID <= 0 {
		return MFACompletion{}, Unauthorized("二次认证挑战租户无效")
	}
	return m.completeMFA(ctx, challengeToken, code, meta, tenantID)
}

func (m *Manager) completeMFA(ctx context.Context, challengeToken, code string, meta RequestMeta, expectedTenantID int) (MFACompletion, error) {
	challengeToken = strings.TrimSpace(challengeToken)
	code = strings.TrimSpace(code)
	if challengeToken == "" || code == "" {
		return MFACompletion{}, Invalid("challengeToken 和 code 必填")
	}
	challenge, err := m.store.IdentityChallenge(ctx, sha256Hex(challengeToken))
	if err != nil {
		return MFACompletion{}, Unauthorized("二次认证挑战无效或已过期")
	}
	now := m.now()
	if challenge.Status != ChallengeStatusPending || now.After(challenge.ExpiresAt) || challenge.Attempts >= challenge.MaxAttempts {
		return MFACompletion{}, Unauthorized("二次认证挑战无效或已过期")
	}
	if expectedTenantID > 0 && challenge.TenantID != expectedTenantID {
		return MFACompletion{}, Unauthorized("二次认证挑战无效或已过期")
	}
	if challenge.IP != "" && meta.IP != challenge.IP {
		_, _ = m.store.FailIdentityChallenge(ctx, challenge.ID, "ip_changed", now)
		return MFACompletion{}, Unauthorized("二次认证挑战环境已变化")
	}
	credential := challenge.Credential
	if credential.Status != MFAStatusActive {
		return MFACompletion{}, Unauthorized("MFA 凭据不可用")
	}
	master, ok := m.keys[credential.EncryptionKeyID]
	if !ok {
		return MFACompletion{}, Unavailable("MFA 历史加密密钥不可用")
	}
	secret, err := decryptIdentitySecret(master, credential.EncryptionKeyID, credential.UserID, credential.SecretCiphertext)
	if err != nil {
		return MFACompletion{}, err
	}
	authMethod := ""
	totpStep := int64(0)
	recoveryJSON := credential.RecoveryCodeHashesJSON
	recoveryCount := credential.RecoveryCodesRemaining
	if step, valid := validateTOTPStep(code, secret, now); valid && step > credential.LastTOTPStep {
		authMethod, totpStep = AuthMethodTOTP, step
	} else {
		var hashes []string
		if err := json.Unmarshal([]byte(recoveryJSON), &hashes); err == nil {
			normalized := normalizeRecoveryCode(code)
			candidate := identityHMAC(master, "recovery-code", credential.UserID, normalized)
			for index, digest := range hashes {
				if hmac.Equal([]byte(candidate), []byte(digest)) {
					hashes = append(hashes[:index], hashes[index+1:]...)
					authMethod = AuthMethodRecovery
					break
				}
			}
			if authMethod == AuthMethodRecovery {
				encoded, marshalErr := json.Marshal(hashes)
				if marshalErr != nil {
					return MFACompletion{}, marshalErr
				}
				recoveryJSON, recoveryCount = string(encoded), len(hashes)
			}
		}
	}
	if authMethod == "" {
		failed, _ := m.store.FailIdentityChallenge(ctx, challenge.ID, "invalid_mfa_code", now)
		_ = m.recordEvent(ctx, Principal{UserID: challenge.UserID, TenantID: challenge.TenantID}, meta, EventMFAFailed, "failed", RiskWarning, "invalid_mfa_code", map[string]any{"attempts": failed.Attempts})
		return MFACompletion{}, Unauthorized("验证码错误")
	}
	if _, err := m.store.UseIdentityMFA(ctx, credential.UserID, credential.Version, recoveryJSON, recoveryCount, totpStep, now); err != nil {
		return MFACompletion{}, err
	}
	if _, err := m.store.ConsumeIdentityChallenge(ctx, challenge.ID, now); err != nil {
		return MFACompletion{}, err
	}
	policy, err := m.Policy(ctx, challenge.TenantID)
	if err != nil {
		return MFACompletion{}, err
	}
	return MFACompletion{Principal: Principal{UserID: challenge.UserID, TenantID: challenge.TenantID}, Policy: policy, AuthMethod: authMethod}, nil
}

func (m *Manager) BeginMFA(ctx context.Context, input MFABegin) (MFAEnrollment, error) {
	master, ok := m.keys[m.activeKeyID]
	if !ok {
		return MFAEnrollment{}, Unavailable("MFA 加密密钥未配置")
	}
	existing, found, err := m.store.IdentityMFACredential(ctx, input.Principal.UserID)
	if err != nil {
		return MFAEnrollment{}, err
	}
	if found && existing.Status == MFAStatusActive {
		return MFAEnrollment{}, Conflict("MFA 已启用，如需重设请先执行安全重置")
	}
	accountName := strings.TrimSpace(input.Principal.Phone)
	if accountName == "" {
		accountName = "user-" + strconv.Itoa(input.Principal.UserID)
	}
	key, err := totp.Generate(totp.GenerateOpts{Issuer: m.issuer, AccountName: accountName, Period: 30, SecretSize: 20, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1})
	if err != nil {
		return MFAEnrollment{}, err
	}
	ciphertext, err := encryptIdentitySecret(master, m.activeKeyID, input.Principal.UserID, key.Secret())
	if err != nil {
		return MFAEnrollment{}, err
	}
	codes, hashes, err := generateRecoveryCodes(master, input.Principal.UserID, 8)
	if err != nil {
		return MFAEnrollment{}, err
	}
	hashesJSON, err := json.Marshal(hashes)
	if err != nil {
		return MFAEnrollment{}, err
	}
	saved, err := m.store.SavePendingIdentityMFA(ctx, MFACredential{
		UserID: input.Principal.UserID, TenantID: input.Principal.TenantID, Status: MFAStatusPending,
		SecretCiphertext: ciphertext, EncryptionKeyID: m.activeKeyID,
		RecoveryCodeHashesJSON: string(hashesJSON), RecoveryCodesRemaining: len(hashes), Version: existing.Version,
	}, input.Actor)
	if err != nil {
		return MFAEnrollment{}, err
	}
	return MFAEnrollment{
		UserID: input.Principal.UserID, TenantID: input.Principal.TenantID, Secret: key.Secret(),
		ProvisioningURI: key.URL(), RecoveryCodes: codes, KeyID: m.activeKeyID, Version: saved.Version,
	}, nil
}

func (m *Manager) VerifyMFA(ctx context.Context, input MFAVerify) (MFACredential, error) {
	credential, found, err := m.store.IdentityMFACredential(ctx, input.Principal.UserID)
	if err != nil {
		return MFACredential{}, err
	}
	if !found || credential.Status != MFAStatusPending || credential.Version != input.ExpectedVersion {
		return MFACredential{}, Conflict("MFA 待验证凭据不存在或版本已变化")
	}
	master, ok := m.keys[credential.EncryptionKeyID]
	if !ok {
		return MFACredential{}, Unavailable("MFA 历史加密密钥不可用")
	}
	secret, err := decryptIdentitySecret(master, credential.EncryptionKeyID, credential.UserID, credential.SecretCiphertext)
	if err != nil {
		return MFACredential{}, err
	}
	step, valid := validateTOTPStep(input.Code, secret, m.now())
	if !valid {
		return MFACredential{}, Invalid("动态验证码错误")
	}
	return m.store.ActivateIdentityMFA(ctx, credential.UserID, credential.Version, credential.RecoveryCodeHashesJSON, credential.RecoveryCodesRemaining, step, input.Actor, m.now())
}

func (m *Manager) PlanMFAReset(ctx context.Context, input MFAReset) (MFAReset, UserState, MFACredential, error) {
	input.Reason = strings.TrimSpace(input.Reason)
	if input.UserID <= 0 || input.ExpectedVersion <= 0 || input.Reason == "" || len([]rune(input.Reason)) > 255 {
		return MFAReset{}, UserState{}, MFACredential{}, Invalid("userId、expectedVersion 和不超过 255 字的 reason 必填")
	}
	state, found, err := m.store.IdentityUserState(ctx, input.UserID)
	if err != nil {
		return MFAReset{}, UserState{}, MFACredential{}, err
	}
	if !found {
		return MFAReset{}, UserState{}, MFACredential{}, NotFound("身份安全账号不存在")
	}
	credential, found, err := m.store.IdentityMFACredential(ctx, input.UserID)
	if err != nil {
		return MFAReset{}, UserState{}, MFACredential{}, err
	}
	if !found {
		return MFAReset{}, UserState{}, MFACredential{}, NotFound("MFA 凭据不存在")
	}
	if credential.Status != MFAStatusActive && credential.Status != MFAStatusPending {
		return MFAReset{}, UserState{}, MFACredential{}, Conflict("MFA 凭据已经停用")
	}
	if credential.Version != input.ExpectedVersion {
		return MFAReset{}, UserState{}, MFACredential{}, Conflict("MFA 凭据版本已变化")
	}
	if state.TenantID <= 0 || credential.TenantID != state.TenantID || (input.TenantID > 0 && input.TenantID != state.TenantID) {
		return MFAReset{}, UserState{}, MFACredential{}, Conflict("MFA 凭据租户归属已变化")
	}
	input.TenantID = state.TenantID
	return input, state, credential, nil
}

func (m *Manager) ResetMFA(ctx context.Context, input MFAReset) (MFAResetResult, error) {
	input.Reason = strings.TrimSpace(input.Reason)
	if input.UserID <= 0 || input.TenantID <= 0 || input.ExpectedVersion <= 0 || input.Reason == "" || len([]rune(input.Reason)) > 255 {
		return MFAResetResult{}, Invalid("userId、tenantId、expectedVersion 和不超过 255 字的 reason 必填")
	}
	return m.store.ResetIdentityMFA(ctx, input, m.now())
}

func (m *Manager) RegisterSession(ctx context.Context, principal Principal, token string, payload map[string]any, method string, meta RequestMeta, policy Policy) (Session, error) {
	jti, _ := payload["jti"].(string)
	iat, _ := numericClaim(payload["iat"])
	exp, _ := numericClaim(payload["exp"])
	if jti == "" || iat <= 0 || exp <= iat {
		return Session{}, Invalid("JWT 会话声明无效")
	}
	issuedAt, expiresAt := time.Unix(iat, 0), time.Unix(exp, 0)
	idle := issuedAt.Add(time.Duration(policy.IdleTimeoutMinutes) * time.Minute)
	if idle.After(expiresAt) {
		idle = expiresAt
	}
	returnSession, _, err := m.store.CreateIdentitySession(ctx, SessionCreate{
		Principal: principal, SessionJTI: jti, TokenSHA256: sha256Hex(token), AuthMethod: method, Meta: meta,
		IssuedAt: issuedAt, ExpiresAt: expiresAt, IdleExpiresAt: idle, MaxConcurrentSessions: policy.MaxConcurrentSessions,
	})
	return returnSession, err
}

func (m *Manager) SessionTTL(policy Policy, fallback time.Duration) time.Duration {
	ttl := time.Duration(policy.SessionTTLMinutes) * time.Minute
	if ttl <= 0 {
		ttl = fallback
	}
	if fallback > 0 && ttl > fallback {
		ttl = fallback
	}
	return ttl
}

func (m *Manager) ValidateSession(ctx context.Context, claims SessionClaims) error {
	if m == nil || !m.enforceSessions {
		return nil
	}
	return m.store.ValidateIdentitySession(ctx, claims, m.now())
}

func (m *Manager) ValidateJWTSession(ctx context.Context, jti string, userID int, issuedAt time.Time, expiresAt time.Time) error {
	return m.ValidateSession(ctx, SessionClaims{JTI: jti, UserID: userID, IssuedAt: issuedAt, ExpiresAt: expiresAt})
}

func (m *Manager) Overview(ctx context.Context, tenantID, limit int) (Overview, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	policy, err := m.Policy(ctx, tenantID)
	if err != nil {
		return Overview{}, err
	}
	summary, users, sessions, events, incidents, err := m.store.IdentityOverview(ctx, tenantID, limit)
	if err != nil {
		return Overview{}, err
	}
	return Overview{Policy: policy, Summary: summary, Users: users, Sessions: sessions, Events: events, Incidents: incidents, Config: m.ConfigStatus()}, nil
}

func (m *Manager) Sessions(ctx context.Context, options SessionOptions) ([]Session, error) {
	return m.store.IdentitySessions(ctx, options)
}

func (m *Manager) UserState(ctx context.Context, userID int) (UserState, error) {
	if userID <= 0 {
		return UserState{}, Invalid("userId 必须大于 0")
	}
	state, found, err := m.store.IdentityUserState(ctx, userID)
	if err != nil {
		return UserState{}, err
	}
	if !found {
		return UserState{UserID: userID, MFAStatus: MFAStatusDisabled}, nil
	}
	return state, nil
}

func (m *Manager) RevokeSession(ctx context.Context, input SessionRevoke) (Session, error) {
	input.Reason = strings.TrimSpace(input.Reason)
	if input.SessionID <= 0 && input.UserID <= 0 {
		return Session{}, Invalid("sessionId 或 userId 至少填写一项")
	}
	if input.Reason == "" {
		return Session{}, Invalid("reason 必填")
	}
	return m.store.RevokeIdentitySession(ctx, input, m.now())
}

func (m *Manager) RevokeCurrentSession(ctx context.Context, jti string, userID int, reason string) error {
	if strings.TrimSpace(jti) == "" || userID <= 0 {
		return nil
	}
	return m.store.RevokeCurrentIdentitySession(ctx, strings.TrimSpace(jti), userID, strings.TrimSpace(reason), m.now())
}

func (m *Manager) UnlockUser(ctx context.Context, userID, expectedVersion int, reason string, actor Actor) (UserState, error) {
	if userID <= 0 || expectedVersion <= 0 || strings.TrimSpace(reason) == "" {
		return UserState{}, Invalid("userId、expectedVersion 和 reason 必填")
	}
	return m.store.UnlockIdentityUser(ctx, userID, expectedVersion, strings.TrimSpace(reason), actor, m.now())
}

func (m *Manager) LoginEvents(ctx context.Context, options EventOptions) ([]LoginEvent, error) {
	return m.store.IdentityLoginEvents(ctx, options)
}

func (m *Manager) Incidents(ctx context.Context, options IncidentOptions) ([]Incident, error) {
	return m.store.IdentityIncidents(ctx, options)
}

func (m *Manager) UpdateIncident(ctx context.Context, input IncidentUpdate) (Incident, error) {
	input.Action = strings.ToLower(strings.TrimSpace(input.Action))
	input.AssignedTo = strings.TrimSpace(input.AssignedTo)
	input.Resolution = strings.TrimSpace(input.Resolution)
	if input.ID <= 0 || input.ExpectedVersion <= 0 || (input.Action != "acknowledge" && input.Action != "assign" && input.Action != "resolve" && input.Action != "reopen") {
		return Incident{}, Invalid("事故操作参数无效")
	}
	if input.Action == "assign" && input.AssignedTo == "" {
		return Incident{}, Invalid("assignedTo 必填")
	}
	if input.Action == "resolve" && input.Resolution == "" {
		return Incident{}, Invalid("resolution 必填")
	}
	return m.store.UpdateIdentityIncident(ctx, input, m.now())
}

func (m *Manager) Cleanup(ctx context.Context, limit int) (CleanupResult, error) {
	if limit <= 0 || limit > 5000 {
		limit = 500
	}
	return m.store.CleanupIdentitySecurity(ctx, m.now(), limit)
}

func (m *Manager) ClientMeta(r *http.Request) RequestMeta {
	if r == nil {
		return RequestMeta{}
	}
	return RequestMeta{IP: m.clientIPResolver.Resolve(r), UserAgent: truncate(strings.TrimSpace(r.UserAgent()), 500)}
}

func (m *Manager) phoneHash(phone string) string {
	phone = strings.TrimSpace(phone)
	if master, ok := m.keys[m.activeKeyID]; ok {
		return identityHMAC(master, "phone", 0, phone)
	}
	return sha256Hex(phone)
}

func (m *Manager) recordEvent(ctx context.Context, principal Principal, meta RequestMeta, eventType, result, risk, reason string, metadata map[string]any) error {
	return m.store.RecordIdentityLoginEvent(ctx, LoginEvent{
		UserID: principal.UserID, TenantID: principal.TenantID, EventType: eventType, Result: result,
		RiskLevel: risk, ReasonCode: reason, IP: meta.IP, UserAgent: meta.UserAgent, Metadata: metadata,
	}, "", m.now())
}

func normalizeCIDRs(items []string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if prefix, err := netip.ParsePrefix(item); err == nil {
			item = prefix.Masked().String()
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result
}

func ipAllowed(value string, cidrs []string) bool {
	ip, err := netip.ParseAddr(strings.TrimSpace(value))
	if err != nil {
		return false
	}
	for _, cidr := range cidrs {
		prefix, err := netip.ParsePrefix(cidr)
		if err == nil && prefix.Contains(ip) {
			return true
		}
	}
	return false
}

func validateTOTPStep(code, secret string, now time.Time) (int64, bool) {
	code = strings.TrimSpace(code)
	for _, offset := range []int64{-1, 0, 1} {
		candidateTime := now.Add(time.Duration(offset) * 30 * time.Second)
		valid, err := totp.ValidateCustom(code, secret, candidateTime, totp.ValidateOpts{Period: 30, Skew: 0, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1})
		if err == nil && valid {
			return candidateTime.Unix() / 30, true
		}
	}
	return 0, false
}

func numericClaim(value any) (int64, bool) {
	switch typed := value.(type) {
	case int64:
		return typed, true
	case int:
		return int64(typed), true
	case float64:
		return int64(typed), true
	case json.Number:
		parsed, err := typed.Int64()
		return parsed, err == nil
	case string:
		parsed, err := strconv.ParseInt(typed, 10, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func truncate(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}
