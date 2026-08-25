package aiproviderconfig

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"jiyi/mochat-go/internal/modules/providers"
	openai "jiyi/mochat-go/internal/modules/providers/ai/openai"
	"jiyi/mochat-go/internal/outboundhttp"
)

const (
	ErrorNotConfigured          = "AI_PROVIDER_NOT_CONFIGURED"
	ErrorCorpScopeInvalid       = "AI_PROVIDER_CORP_SCOPE_INVALID"
	ErrorDisabled               = "AI_PROVIDER_DISABLED"
	ErrorNotEffective           = "AI_PROVIDER_NOT_EFFECTIVE"
	ErrorExpired                = "AI_PROVIDER_EXPIRED"
	ErrorCredentialUnavailable  = "AI_PROVIDER_CREDENTIAL_UNAVAILABLE"
	ErrorOutboundAddressInvalid = "AI_PROVIDER_OUTBOUND_ADDRESS_INVALID"
	ErrorConfigurationInvalid   = "AI_PROVIDER_CONFIGURATION_INVALID"
)

// ResolveError deliberately contains only stable, safe diagnostics. Do not
// wrap database, URL, ciphertext, or credential failures with this error.
type ResolveError struct {
	Code   string
	Reason string
}

func (e *ResolveError) Error() string {
	if e == nil {
		return ""
	}
	if e.Reason == "" {
		return e.Code
	}
	return e.Code + ": " + e.Reason
}

func (e *ResolveError) SafeCode() string {
	if e == nil {
		return ""
	}
	return e.Code
}

func (e *ResolveError) SafeReason() string {
	if e == nil {
		return ""
	}
	return e.Reason
}

// TenantResolver resolves one database-backed tenant provider for a bound
// active corp. It has no cache so resolved plaintext cannot cross scopes.
type TenantResolver struct {
	DB      *sql.DB
	Cipher  *Manager
	Guard   *outboundhttp.Guard
	Now     func() time.Time
	Timeout time.Duration
}

var _ providers.AIProviderResolver = (*TenantResolver)(nil)

func NewTenantResolver(db *sql.DB, cipher *Manager, guard *outboundhttp.Guard) *TenantResolver {
	return &TenantResolver{DB: db, Cipher: cipher, Guard: guard, Now: time.Now, Timeout: 30 * time.Second}
}

func (r *TenantResolver) Resolve(ctx context.Context, tenantID, corpID int64) (providers.AIProvider, error) {
	if r == nil || r.DB == nil || r.Cipher == nil || tenantID <= 0 || corpID <= 0 {
		return nil, safeResolveError(ErrorConfigurationInvalid, "租户模型配置不可用")
	}
	if err := r.ensureBoundActiveCorp(ctx, tenantID, corpID); err != nil {
		return nil, err
	}
	stored, found, err := r.load(ctx, tenantID)
	if err != nil {
		return nil, safeResolveError(ErrorNotConfigured, "租户模型配置不可用")
	}
	if !found {
		return nil, safeResolveError(ErrorNotConfigured, "尚未配置租户模型")
	}
	now := time.Now()
	if r.Now != nil {
		now = r.Now()
	}
	if err := stored.validateForUse(now); err != nil {
		return nil, resolveStoredValidationError(stored, now)
	}
	guard := r.Guard
	if guard == nil {
		guard = outboundhttp.MustDefaultGuard()
	}
	if err := guard.ValidateURLWithResolution(ctx, stored.BaseURL); err != nil {
		return nil, safeResolveError(ErrorOutboundAddressInvalid, "模型服务地址不可用")
	}
	apiKey, err := r.Cipher.DecryptStored(stored, now)
	if err != nil {
		return nil, safeResolveError(ErrorCredentialUnavailable, "模型凭证不可用")
	}
	provider, err := openai.New(openai.Config{BaseURL: stored.BaseURL, APIKey: apiKey, Model: stored.Model, Provider: stored.Provider, Timeout: r.Timeout, Client: guard.NewClient()})
	if err != nil {
		return nil, safeResolveError(ErrorConfigurationInvalid, "租户模型配置不可用")
	}
	return provider, nil
}

func (r *TenantResolver) ensureBoundActiveCorp(ctx context.Context, tenantID, corpID int64) error {
	var one int
	err := r.DB.QueryRowContext(ctx, `SELECT 1
		FROM mc_corp c
		JOIN mochat_go_tenant_corp_bindings b ON b.tenant_id = c.tenant_id AND b.corp_id = c.id
		WHERE c.id = ? AND c.tenant_id = ? AND c.deleted_at IS NULL AND b.status = 2
		LIMIT 1`, corpID, tenantID).Scan(&one)
	if err != nil {
		return safeResolveError(ErrorCorpScopeInvalid, "企业不属于当前租户或未启用")
	}
	return nil
}

func (r *TenantResolver) load(ctx context.Context, tenantID int64) (StoredConfig, bool, error) {
	var config StoredConfig
	var effective, expires sql.NullTime
	err := r.DB.QueryRowContext(ctx, `SELECT tenant_id, provider, base_url, model, credential_ciphertext, encryption_key_id, api_key_hint, effective_at, expires_at, status, version
		FROM mochat_go_saas_tenant_ai_providers
		WHERE tenant_id = ?
		LIMIT 1`, tenantID).Scan(&config.TenantID, &config.Provider, &config.BaseURL, &config.Model, &config.CredentialCiphertext, &config.KeyID, &config.Hint, &effective, &expires, &config.Status, &config.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return StoredConfig{}, false, nil
	}
	if err != nil {
		return StoredConfig{}, false, err
	}
	if effective.Valid {
		value := effective.Time.UTC()
		config.EffectiveAt = &value
	}
	if expires.Valid {
		value := expires.Time.UTC()
		config.ExpiresAt = &value
	}
	return config, true, nil
}

func resolveStoredValidationError(config StoredConfig, now time.Time) error {
	if config.Status != StatusActive {
		return safeResolveError(ErrorDisabled, "租户模型已停用")
	}
	if config.EffectiveAt != nil && now.Before(*config.EffectiveAt) {
		return safeResolveError(ErrorNotEffective, "租户模型尚未生效")
	}
	if config.ExpiresAt != nil && !now.Before(*config.ExpiresAt) {
		return safeResolveError(ErrorExpired, "租户模型已过期")
	}
	return safeResolveError(ErrorConfigurationInvalid, "租户模型配置不可用")
}

func safeResolveError(code, reason string) error {
	return &ResolveError{Code: strings.TrimSpace(code), Reason: strings.TrimSpace(reason)}
}
