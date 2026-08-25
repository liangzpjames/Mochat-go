package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"jiyi/mochat-go/internal/aiproviderconfig"
	"jiyi/mochat-go/internal/dashboard"
	"jiyi/mochat-go/internal/outboundhttp"
)

func (s *MySQLStore) WithAIProviderCredentialCipher(cipher *aiproviderconfig.Manager) *MySQLStore {
	if s != nil {
		s.aiProviderCredentialCipher = cipher
	}
	return s
}
func (s *MySQLStore) WithAIProviderOutboundGuard(guard *outboundhttp.Guard) *MySQLStore {
	if s != nil {
		s.aiProviderOutboundGuard = guard
	}
	return s
}

func (s *MySQLStore) SaaSTenantAIProvider(ctx context.Context, tenantID int) (dashboard.SaaSTenantAIProvider, bool, error) {
	if tenantID <= 0 {
		return dashboard.SaaSTenantAIProvider{}, false, dashboard.NewSaaSAdminBadRequest("tenantId is invalid")
	}
	if err := s.ensureSaaSTenantExists(ctx, s.db, tenantID); err != nil {
		return dashboard.SaaSTenantAIProvider{}, false, err
	}
	stored, found, err := scanTenantAIProviderStored(s.db.QueryRowContext(ctx, `SELECT tenant_id, provider, base_url, model, credential_ciphertext, encryption_key_id, api_key_hint, effective_at, expires_at, status, version, updated_at FROM mochat_go_saas_tenant_ai_providers WHERE tenant_id = ? LIMIT 1`, tenantID))
	if err != nil || !found {
		return dashboard.SaaSTenantAIProvider{}, found, err
	}
	return s.tenantAIProviderPublic(stored), true, nil
}

func (s *MySQLStore) SaveSaaSTenantAIProvider(ctx context.Context, input dashboard.SaaSTenantAIProviderInput) (dashboard.SaaSTenantAIProvider, error) {
	if err := dashboard.NormalizeAndValidateSaaSTenantAIProviderInput(&input); err != nil {
		return dashboard.SaaSTenantAIProvider{}, dashboard.NewSaaSAdminBadRequest(err.Error())
	}
	if s == nil || s.db == nil || s.aiProviderCredentialCipher == nil {
		return dashboard.SaaSTenantAIProvider{}, errors.New("AI provider credential protection is unavailable")
	}
	guard := s.aiProviderOutboundGuard
	if guard == nil {
		guard = outboundhttp.MustDefaultGuard()
	}
	if err := guard.ValidateURLWithResolution(ctx, input.BaseURL); err != nil {
		return dashboard.SaaSTenantAIProvider{}, dashboard.NewSaaSAdminBadRequest("baseUrl is not an allowed outbound destination")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.SaaSTenantAIProvider{}, err
	}
	defer tx.Rollback()
	if err := s.ensureSaaSTenantExists(ctx, tx, input.TenantID); err != nil {
		return dashboard.SaaSTenantAIProvider{}, err
	}
	current, found, err := scanTenantAIProviderStored(tx.QueryRowContext(ctx, `SELECT tenant_id, provider, base_url, model, credential_ciphertext, encryption_key_id, api_key_hint, effective_at, expires_at, status, version, updated_at FROM mochat_go_saas_tenant_ai_providers WHERE tenant_id = ? FOR UPDATE`, input.TenantID))
	if err != nil {
		return dashboard.SaaSTenantAIProvider{}, err
	}
	if !found {
		if err := dashboard.ValidateSaaSTenantAIProviderCreate(input); err != nil {
			return dashboard.SaaSTenantAIProvider{}, dashboard.NewSaaSAdminBadRequest(err.Error())
		}
		if input.Version != 0 {
			return dashboard.SaaSTenantAIProvider{}, tenantAIProviderConflict()
		}
	} else {
		if strings.TrimSpace(input.APIKey) == "" {
			if _, decryptErr := s.aiProviderCredentialCipher.Decrypt(current.tenantID, current.provider, current.keyID, current.ciphertext); decryptErr != nil {
				return dashboard.SaaSTenantAIProvider{}, errors.New("AI provider credential must be replaced")
			}
		}
		if err := dashboard.ValidateSaaSTenantAIProviderUpdate(input, s.tenantAIProviderPublic(current)); err != nil {
			return dashboard.SaaSTenantAIProvider{}, dashboard.NewSaaSAdminBadRequest(err.Error())
		}
		if input.Version != current.version {
			return dashboard.SaaSTenantAIProvider{}, tenantAIProviderConflict()
		}
	}
	ciphertext, keyID, hint := current.ciphertext, current.keyID, current.hint
	keyChanged := strings.TrimSpace(input.APIKey) != ""
	if keyChanged {
		ciphertext, keyID, hint, err = s.aiProviderCredentialCipher.Encrypt(input.TenantID, input.ProviderCode, input.APIKey)
		if err != nil {
			return dashboard.SaaSTenantAIProvider{}, errors.New("AI provider credential protection failed")
		}
	}
	if strings.TrimSpace(ciphertext) == "" || strings.TrimSpace(keyID) == "" {
		return dashboard.SaaSTenantAIProvider{}, errors.New("AI provider credential protection failed")
	}
	effective, _ := time.Parse(time.RFC3339, input.EffectiveAt)
	expires, _ := time.Parse(time.RFC3339, input.ExpiresAt)
	if !found {
		_, err = tx.ExecContext(ctx, `INSERT INTO mochat_go_saas_tenant_ai_providers (tenant_id, provider, base_url, model, credential_ciphertext, encryption_key_id, api_key_hint, effective_at, expires_at, status, version, created_by, updated_by) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?, ?)`, input.TenantID, input.ProviderCode, input.BaseURL, input.Model, ciphertext, keyID, hint, effective, expires, input.Status, input.ActorUserID, input.ActorUserID)
		if isMySQLDuplicateKeyError(err) {
			return dashboard.SaaSTenantAIProvider{}, tenantAIProviderConflict()
		}
	} else {
		var result sql.Result
		result, err = tx.ExecContext(ctx, `UPDATE mochat_go_saas_tenant_ai_providers SET provider = ?, base_url = ?, model = ?, credential_ciphertext = ?, encryption_key_id = ?, api_key_hint = ?, effective_at = ?, expires_at = ?, status = ?, version = version + 1, updated_by = ? WHERE tenant_id = ? AND version = ?`, input.ProviderCode, input.BaseURL, input.Model, ciphertext, keyID, hint, effective, expires, input.Status, input.ActorUserID, input.TenantID, input.Version)
		if err == nil {
			affected, affectedErr := result.RowsAffected()
			if affectedErr != nil || affected != 1 {
				return dashboard.SaaSTenantAIProvider{}, tenantAIProviderConflict()
			}
		}
	}
	if err != nil {
		if isMySQLDuplicateKeyError(err) || isMySQLRetryableTransactionError(err) {
			return dashboard.SaaSTenantAIProvider{}, tenantAIProviderConflict()
		}
		return dashboard.SaaSTenantAIProvider{}, err
	}
	after, afterFound, err := scanTenantAIProviderStored(tx.QueryRowContext(ctx, `SELECT tenant_id, provider, base_url, model, credential_ciphertext, encryption_key_id, api_key_hint, effective_at, expires_at, status, version, updated_at FROM mochat_go_saas_tenant_ai_providers WHERE tenant_id = ?`, input.TenantID))
	if err != nil || !afterFound {
		if err != nil {
			return dashboard.SaaSTenantAIProvider{}, err
		}
		return dashboard.SaaSTenantAIProvider{}, errors.New("tenant AI provider was not saved")
	}
	beforeJSON := ""
	if found {
		beforeJSON = tenantAIProviderAuditJSON(s.tenantAIProviderPublic(current), false)
	}
	if _, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{TenantID: input.TenantID, ActorUserID: input.ActorUserID, ActorTenantID: input.ActorTenantID, Action: dashboard.SaaSAdminOperationActionTenantAIProviderSave, TargetType: dashboard.SaaSAdminOperationTargetTenantAIProvider, TargetID: strconv.Itoa(input.TenantID), TargetName: input.ProviderCode, BeforeJSON: beforeJSON, AfterJSON: tenantAIProviderAuditJSON(s.tenantAIProviderPublic(after), keyChanged), Remark: "tenant AI provider saved"}); err != nil {
		return dashboard.SaaSTenantAIProvider{}, err
	}
	if err := tx.Commit(); err != nil {
		if isMySQLDuplicateKeyError(err) || isMySQLRetryableTransactionError(err) {
			return dashboard.SaaSTenantAIProvider{}, tenantAIProviderConflict()
		}
		return dashboard.SaaSTenantAIProvider{}, err
	}
	return s.tenantAIProviderPublic(after), nil
}

func tenantAIProviderConflict() error {
	return &dashboard.SaaSAdminOperationError{Status: 409, Message: "tenant AI provider version changed"}
}
func tenantAIProviderAuditJSON(provider dashboard.SaaSTenantAIProvider, keyChanged bool) string {
	raw, _ := json.Marshal(dashboard.SaaSTenantAIProviderAuditPayload(provider, keyChanged))
	return string(raw)
}

type tenantAIProviderStored struct {
	tenantID                                                  int
	provider, baseURL, model, ciphertext, keyID, hint, status string
	effective, expires                                        sql.NullTime
	version                                                   int
	updated                                                   time.Time
}
type tenantAIProviderRow interface{ Scan(...any) error }

func scanTenantAIProviderStored(row tenantAIProviderRow) (tenantAIProviderStored, bool, error) {
	var value tenantAIProviderStored
	err := row.Scan(&value.tenantID, &value.provider, &value.baseURL, &value.model, &value.ciphertext, &value.keyID, &value.hint, &value.effective, &value.expires, &value.status, &value.version, &value.updated)
	if errors.Is(err, sql.ErrNoRows) {
		return tenantAIProviderStored{}, false, nil
	}
	return value, err == nil, err
}
func (s *MySQLStore) tenantAIProviderPublic(value tenantAIProviderStored) dashboard.SaaSTenantAIProvider {
	item := dashboard.SaaSTenantAIProvider{TenantID: value.tenantID, ProviderCode: value.provider, BaseURL: value.baseURL, Model: value.model, APIKeyConfigured: strings.TrimSpace(value.ciphertext) != "", APIKeyHint: value.hint, Status: value.status, Version: value.version, UpdatedAt: value.updated.UTC().Format(time.RFC3339), CredentialProtection: "unavailable"}
	if value.effective.Valid {
		item.EffectiveAt = value.effective.Time.UTC().Format(time.RFC3339)
	}
	if value.expires.Valid {
		item.ExpiresAt = value.expires.Time.UTC().Format(time.RFC3339)
	}
	if item.APIKeyConfigured && s.aiProviderCredentialCipher != nil {
		if _, err := s.aiProviderCredentialCipher.Decrypt(value.tenantID, value.provider, value.keyID, value.ciphertext); err == nil {
			item.CredentialProtection = "usable"
		}
	}
	return item
}

type tenantAIProviderTenantQueryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func (s *MySQLStore) ensureSaaSTenantExists(ctx context.Context, queryer tenantAIProviderTenantQueryer, tenantID int) error {
	var id int
	err := queryer.QueryRowContext(ctx, `SELECT id FROM mc_tenant WHERE id = ? AND deleted_at IS NULL LIMIT 1`, tenantID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.NewSaaSAdminNotFound("tenant not found")
	}
	return err
}

var _ dashboard.SaaSTenantAIProviderStore = (*MySQLStore)(nil)
