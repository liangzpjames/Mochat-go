package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"jiyi/mochat-go/internal/aiproviderconfig"
	"jiyi/mochat-go/internal/dashboard"
)

func (s *MySQLStore) WithAIProviderCredentialCipher(cipher *aiproviderconfig.Manager) *MySQLStore {
	if s != nil {
		s.aiProviderCredentialCipher = cipher
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
	provider, found, err := scanSaaSTenantAIProvider(s.db.QueryRowContext(ctx, `
		SELECT tenant_id, provider, base_url, model, api_key_hint, effective_at, expires_at, status, version, updated_at
		FROM mochat_go_saas_tenant_ai_providers WHERE tenant_id = ? LIMIT 1`, tenantID))
	return provider, found, err
}

func (s *MySQLStore) SaveSaaSTenantAIProvider(ctx context.Context, input dashboard.SaaSTenantAIProviderInput) (dashboard.SaaSTenantAIProvider, error) {
	if err := dashboard.NormalizeAndValidateSaaSTenantAIProviderInput(&input); err != nil {
		return dashboard.SaaSTenantAIProvider{}, dashboard.NewSaaSAdminBadRequest(err.Error())
	}
	if s == nil || s.db == nil || s.aiProviderCredentialCipher == nil {
		return dashboard.SaaSTenantAIProvider{}, errors.New("AI provider credential protection is unavailable")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.SaaSTenantAIProvider{}, err
	}
	defer tx.Rollback()
	if err := s.ensureSaaSTenantExists(ctx, tx, input.TenantID); err != nil {
		return dashboard.SaaSTenantAIProvider{}, err
	}
	var ciphertext, keyID, hint string
	var currentVersion int
	err = tx.QueryRowContext(ctx, `SELECT credential_ciphertext, encryption_key_id, api_key_hint, version FROM mochat_go_saas_tenant_ai_providers WHERE tenant_id = ? FOR UPDATE`, input.TenantID).Scan(&ciphertext, &keyID, &hint, &currentVersion)
	created := errors.Is(err, sql.ErrNoRows)
	if err != nil && !created {
		return dashboard.SaaSTenantAIProvider{}, err
	}
	if created {
		if err := dashboard.ValidateSaaSTenantAIProviderCreate(input); err != nil {
			return dashboard.SaaSTenantAIProvider{}, dashboard.NewSaaSAdminBadRequest(err.Error())
		}
		if input.Version != 0 {
			return dashboard.SaaSTenantAIProvider{}, &dashboard.SaaSAdminOperationError{Status: 409, Message: "tenant AI provider version changed"}
		}
	} else if input.Version != currentVersion {
		return dashboard.SaaSTenantAIProvider{}, &dashboard.SaaSAdminOperationError{Status: 409, Message: "tenant AI provider version changed"}
	}
	if strings.TrimSpace(input.APIKey) != "" {
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
	if created {
		_, err = tx.ExecContext(ctx, `INSERT INTO mochat_go_saas_tenant_ai_providers (tenant_id, provider, base_url, model, credential_ciphertext, encryption_key_id, api_key_hint, effective_at, expires_at, status, version, created_by, updated_by) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?, ?)`, input.TenantID, input.ProviderCode, input.BaseURL, input.Model, ciphertext, keyID, hint, effective, expires, input.Status, input.ActorUserID, input.ActorUserID)
	} else {
		_, err = tx.ExecContext(ctx, `UPDATE mochat_go_saas_tenant_ai_providers SET provider = ?, base_url = ?, model = ?, credential_ciphertext = ?, encryption_key_id = ?, api_key_hint = ?, effective_at = ?, expires_at = ?, status = ?, version = version + 1, updated_by = ? WHERE tenant_id = ? AND version = ?`, input.ProviderCode, input.BaseURL, input.Model, ciphertext, keyID, hint, effective, expires, input.Status, input.ActorUserID, input.TenantID, input.Version)
	}
	if err != nil {
		return dashboard.SaaSTenantAIProvider{}, err
	}
	provider, found, err := scanSaaSTenantAIProvider(tx.QueryRowContext(ctx, `SELECT tenant_id, provider, base_url, model, api_key_hint, effective_at, expires_at, status, version, updated_at FROM mochat_go_saas_tenant_ai_providers WHERE tenant_id = ?`, input.TenantID))
	if err != nil {
		return dashboard.SaaSTenantAIProvider{}, err
	}
	if !found {
		return dashboard.SaaSTenantAIProvider{}, errors.New("tenant AI provider was not saved")
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSTenantAIProvider{}, err
	}
	return provider, nil
}

type tenantAIProviderRow interface{ Scan(...any) error }

func scanSaaSTenantAIProvider(row tenantAIProviderRow) (dashboard.SaaSTenantAIProvider, bool, error) {
	var item dashboard.SaaSTenantAIProvider
	var effective, expires sql.NullTime
	var updated time.Time
	err := row.Scan(&item.TenantID, &item.ProviderCode, &item.BaseURL, &item.Model, &item.APIKeyHint, &effective, &expires, &item.Status, &item.Version, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSTenantAIProvider{}, false, nil
	}
	if err != nil {
		return dashboard.SaaSTenantAIProvider{}, false, err
	}
	item.APIKeyConfigured = true
	item.CredentialProtection = "encrypted"
	item.UpdatedAt = updated.UTC().Format(time.RFC3339)
	if effective.Valid {
		item.EffectiveAt = effective.Time.UTC().Format(time.RFC3339)
	}
	if expires.Valid {
		item.ExpiresAt = expires.Time.UTC().Format(time.RFC3339)
	}
	return item, true, nil
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
