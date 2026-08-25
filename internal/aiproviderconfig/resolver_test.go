package aiproviderconfig

import (
	"context"
	"errors"
	"net/netip"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"jiyi/mochat-go/internal/modules/providers"
	openai "jiyi/mochat-go/internal/modules/providers/ai/openai"
	"jiyi/mochat-go/internal/outboundhttp"
)

type resolverDNSStub struct{}

func (resolverDNSStub) LookupNetIP(context.Context, string, string) ([]netip.Addr, error) {
	return []netip.Addr{netip.MustParseAddr("8.8.8.8")}, nil
}

type resolverFixedDNS struct{ addresses []netip.Addr }

func (r resolverFixedDNS) LookupNetIP(context.Context, string, string) ([]netip.Addr, error) {
	return r.addresses, nil
}

type resolverTestProvider struct{}

func (resolverTestProvider) Chat(context.Context, providers.ChatRequest) (string, error) {
	return "{}", nil
}
func (resolverTestProvider) Status() providers.Status {
	return providers.Status{State: providers.StateReady}
}

func TestTenantResolverBuildsIsolatedConfiguredProvider(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	manager, err := NewManager(Config{EncryptionKey: testKey(8), EncryptionKeyID: "ai-v1", RequireEncryption: true})
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, keyID, _, err := manager.Encrypt(7, "deepseek", "fixture-key-resolver-1234")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 25, 9, 0, 0, 0, time.UTC)
	effective, expires := now.Add(-time.Hour), now.Add(time.Hour)
	guard, err := outboundhttp.NewGuard(outboundhttp.Config{RequireHTTPS: true, Resolver: resolverDNSStub{}})
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT 1
		FROM mc_corp c
		JOIN mochat_go_tenant_corp_bindings b ON b.tenant_id = c.tenant_id AND b.corp_id = c.id
		WHERE c.id = ? AND c.tenant_id = ? AND c.deleted_at IS NULL AND b.status = 2
		LIMIT 1`)).WithArgs(int64(9), int64(7)).WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT tenant_id, provider, base_url, model, credential_ciphertext, encryption_key_id, api_key_hint, effective_at, expires_at, status, version
		FROM mochat_go_saas_tenant_ai_providers
		WHERE tenant_id = ?
		LIMIT 1`)).WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"tenant_id", "provider", "base_url", "model", "credential_ciphertext", "encryption_key_id", "api_key_hint", "effective_at", "expires_at", "status", "version"}).AddRow(7, "deepseek", "https://provider.example.test/v1", "deepseek-chat", ciphertext, keyID, "1234", effective, expires, StatusActive, 1))

	resolver := NewTenantResolver(db, manager, guard)
	resolver.Now = func() time.Time { return now }
	provider, err := resolver.Resolve(context.Background(), 7, 9)
	if err != nil {
		t.Fatal(err)
	}
	metadataReader, ok := provider.(interface {
		Metadata() providers.AIProviderMetadata
	})
	if !ok {
		t.Fatalf("provider does not expose metadata: %T", provider)
	}
	metadata := metadataReader.Metadata()
	if metadata.Provider != "deepseek" || metadata.Model != "deepseek-chat" {
		t.Fatalf("metadata = %#v", metadata)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestTenantResolverFailsClosedWithSafeCodes(t *testing.T) {
	manager, err := NewManager(Config{EncryptionKey: testKey(9), EncryptionKeyID: "ai-v1", RequireEncryption: true})
	if err != nil {
		t.Fatal(err)
	}
	validCiphertext, keyID, _, err := manager.Encrypt(7, "openai", "fixture-key-never-in-error-9876")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 25, 9, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		row  func() *sqlmock.Rows
		code string
	}{
		{name: "missing", row: func() *sqlmock.Rows {
			return sqlmock.NewRows([]string{"tenant_id", "provider", "base_url", "model", "credential_ciphertext", "encryption_key_id", "api_key_hint", "effective_at", "expires_at", "status", "version"})
		}, code: ErrorNotConfigured},
		{name: "expired boundary", row: func() *sqlmock.Rows {
			return resolverRows().AddRow(7, "openai", "https://provider.example.test/v1", "model", validCiphertext, keyID, "9876", now.Add(-time.Hour), now, StatusActive, 1)
		}, code: ErrorExpired},
		{name: "unavailable key", row: func() *sqlmock.Rows {
			return resolverRows().AddRow(7, "openai", "https://provider.example.test/v1", "model", validCiphertext, "missing-key", "9876", now.Add(-time.Hour), now.Add(time.Hour), StatusActive, 1)
		}, code: ErrorCredentialUnavailable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			mock.ExpectQuery(regexp.QuoteMeta(`SELECT 1
		FROM mc_corp c
		JOIN mochat_go_tenant_corp_bindings b ON b.tenant_id = c.tenant_id AND b.corp_id = c.id
		WHERE c.id = ? AND c.tenant_id = ? AND c.deleted_at IS NULL AND b.status = 2
		LIMIT 1`)).WithArgs(int64(9), int64(7)).WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
			mock.ExpectQuery(regexp.QuoteMeta(`SELECT tenant_id, provider, base_url, model, credential_ciphertext, encryption_key_id, api_key_hint, effective_at, expires_at, status, version
		FROM mochat_go_saas_tenant_ai_providers
		WHERE tenant_id = ?
		LIMIT 1`)).WithArgs(int64(7)).WillReturnRows(test.row())
			guard, guardErr := outboundhttp.NewGuard(outboundhttp.Config{RequireHTTPS: true, Resolver: resolverDNSStub{}})
			if guardErr != nil {
				t.Fatal(guardErr)
			}
			resolver := NewTenantResolver(db, manager, guard)
			resolver.Now = func() time.Time { return now }
			_, err = resolver.Resolve(context.Background(), 7, 9)
			var safe *ResolveError
			if !errors.As(err, &safe) || safe.Code != test.code {
				t.Fatalf("error=%v code=%#v, want %s", err, safe, test.code)
			}
			if strings.Contains(err.Error(), "fixture-key-never-in-error-9876") || strings.Contains(err.Error(), validCiphertext) || strings.Contains(err.Error(), "provider.example.test") {
				t.Fatalf("resolver error leaked sensitive fixture: %v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestStoredConfigRuntimeValidationRequiresCompleteActiveWindowAndKnownProvider(t *testing.T) {
	now := time.Date(2026, 8, 25, 9, 0, 0, 0, time.UTC)
	start, end := now.Add(-time.Hour), now.Add(time.Hour)
	valid := StoredConfig{TenantID: 7, Provider: "openai", BaseURL: "https://provider.example.test/v1", Model: "model", CredentialCiphertext: "cipher", KeyID: "ai-v1", Status: StatusActive, Version: 1, EffectiveAt: &start, ExpiresAt: &end}
	for name, mutate := range map[string]func(*StoredConfig){
		"missing effective": func(config *StoredConfig) { config.EffectiveAt = nil },
		"missing expiry":    func(config *StoredConfig) { config.ExpiresAt = nil },
		"invalid window":    func(config *StoredConfig) { value := start; config.ExpiresAt = &value },
		"unknown provider":  func(config *StoredConfig) { config.Provider = "other" },
		"missing base url":  func(config *StoredConfig) { config.BaseURL = "" },
	} {
		t.Run(name, func(t *testing.T) {
			config := valid
			mutate(&config)
			if err := config.validateForUse(now); err == nil {
				t.Fatal("validateForUse error=nil, want fail closed")
			}
		})
	}
}

func TestTenantResolverKeepsTenantProviderSecretsAndModelsIsolated(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	manager, err := NewManager(Config{EncryptionKey: testKey(18), EncryptionKeyID: "ai-v1", RequireEncryption: true})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 25, 9, 0, 0, 0, time.UTC)
	type tenantCase struct {
		tenant, corp                     int64
		provider, baseURL, model, secret string
	}
	tests := []tenantCase{
		{tenant: 7, corp: 70, provider: "deepseek", baseURL: "https://deepseek.example.test/v1", model: "deepseek-chat", secret: "fixture-tenant-a-key-1234"},
		{tenant: 8, corp: 80, provider: "openai", baseURL: "https://openai.example.test/v1", model: "gpt-tenant-b", secret: "fixture-tenant-b-key-5678"},
	}
	for _, test := range tests {
		ciphertext, keyID, hint, encryptErr := manager.Encrypt(int(test.tenant), test.provider, test.secret)
		if encryptErr != nil {
			t.Fatal(encryptErr)
		}
		mock.ExpectQuery("SELECT 1").WithArgs(test.corp, test.tenant).WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
		mock.ExpectQuery("SELECT tenant_id, provider, base_url").WithArgs(test.tenant).WillReturnRows(resolverRows().AddRow(test.tenant, test.provider, test.baseURL, test.model, ciphertext, keyID, hint, now.Add(-time.Hour), now.Add(time.Hour), StatusActive, 1))
	}
	guard, err := outboundhttp.NewGuard(outboundhttp.Config{RequireHTTPS: true, Resolver: resolverDNSStub{}})
	if err != nil {
		t.Fatal(err)
	}
	resolver := NewTenantResolver(db, manager, guard)
	resolver.Now = func() time.Time { return now }
	var captured []openai.Config
	resolver.Factory = func(config openai.Config) (providers.AIProvider, error) {
		captured = append(captured, config)
		return resolverTestProvider{}, nil
	}
	for _, test := range tests {
		if _, err := resolver.Resolve(context.Background(), test.tenant, test.corp); err != nil {
			t.Fatal(err)
		}
	}
	if len(captured) != 2 {
		t.Fatalf("factory calls=%d", len(captured))
	}
	for index, test := range tests {
		got := captured[index]
		if got.Provider != test.provider || got.BaseURL != test.baseURL || got.Model != test.model || got.APIKey != test.secret || got.Client == nil {
			t.Fatalf("tenant index=%d config mismatch", index)
		}
	}
	if captured[0].APIKey == captured[1].APIKey || captured[0].BaseURL == captured[1].BaseURL {
		t.Fatal("tenant provider configurations crossed scopes")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestTenantResolverRejectsEveryUnsafeRuntimeStateBeforeProviderCreation(t *testing.T) {
	manager, err := NewManager(Config{EncryptionKey: testKey(19), EncryptionKeyID: "ai-v1", RequireEncryption: true})
	if err != nil {
		t.Fatal(err)
	}
	secret := "fixture-resolver-secret-never-leak-1234"
	ciphertext, keyID, hint, err := manager.Encrypt(7, "openai", secret)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 25, 9, 0, 0, 0, time.UTC)
	validRow := func() *sqlmock.Rows {
		return resolverRows().AddRow(7, "openai", "https://provider.example.test/v1", "model", ciphertext, keyID, hint, now.Add(-time.Hour), now.Add(time.Hour), StatusActive, 1)
	}
	tests := []struct {
		name       string
		binding    func(sqlmock.Sqlmock)
		row        func() *sqlmock.Rows
		code       string
		manager    *Manager
		guard      *outboundhttp.Guard
		setOldEnvs bool
	}{
		{name: "binding missing", binding: func(mock sqlmock.Sqlmock) {
			mock.ExpectQuery("SELECT 1").WithArgs(int64(9), int64(7)).WillReturnRows(sqlmock.NewRows([]string{"1"}))
		}, code: ErrorCorpScopeInvalid},
		{name: "binding database error", binding: func(mock sqlmock.Sqlmock) {
			mock.ExpectQuery("SELECT 1").WithArgs(int64(9), int64(7)).WillReturnError(errors.New("database unavailable"))
		}, code: ErrorCorpScopeInvalid},
		{name: "not effective", row: func() *sqlmock.Rows {
			return resolverRows().AddRow(7, "openai", "https://provider.example.test/v1", "model", ciphertext, keyID, hint, now.Add(time.Minute), now.Add(time.Hour), StatusActive, 1)
		}, code: ErrorNotEffective},
		{name: "disabled", row: func() *sqlmock.Rows {
			return resolverRows().AddRow(7, "openai", "https://provider.example.test/v1", "model", ciphertext, keyID, hint, now.Add(-time.Hour), now.Add(time.Hour), StatusDisabled, 1)
		}, code: ErrorDisabled},
		{name: "corrupt ciphertext", row: func() *sqlmock.Rows {
			return resolverRows().AddRow(7, "openai", "https://provider.example.test/v1", "model", "corrupt-ciphertext", keyID, hint, now.Add(-time.Hour), now.Add(time.Hour), StatusActive, 1)
		}, code: ErrorCredentialUnavailable},
		{name: "empty key ring", row: validRow, code: ErrorCredentialUnavailable, manager: func() *Manager { value, _ := NewManager(Config{}); return value }()},
		{name: "unsafe literal", row: func() *sqlmock.Rows {
			return resolverRows().AddRow(7, "openai", "https://127.0.0.1/v1", "model", ciphertext, keyID, hint, now.Add(-time.Hour), now.Add(time.Hour), StatusActive, 1)
		}, code: ErrorOutboundAddressInvalid},
		{name: "unsafe dns", row: func() *sqlmock.Rows {
			return resolverRows().AddRow(7, "openai", "https://provider.example.test/v1", "model", ciphertext, keyID, hint, now.Add(-time.Hour), now.Add(time.Hour), StatusActive, 1)
		}, code: ErrorOutboundAddressInvalid, guard: func() *outboundhttp.Guard {
			value, _ := outboundhttp.NewGuard(outboundhttp.Config{RequireHTTPS: true, Resolver: resolverFixedDNS{addresses: []netip.Addr{netip.MustParseAddr("127.0.0.1")}}})
			return value
		}()},
		{name: "old environment fallback forbidden", row: func() *sqlmock.Rows { return resolverRows() }, code: ErrorNotConfigured, setOldEnvs: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.setOldEnvs {
				t.Setenv("MOCHAT_GO_AI_PROVIDER_KEY", "old-global-secret")
				t.Setenv("MOCHAT_GO_AI_PROVIDER_BASE_URL", "https://old-global.example/private")
				t.Setenv("MOCHAT_GO_AI_PROVIDER_MODEL", "old-model")
			}
			db, mock, openErr := sqlmock.New()
			if openErr != nil {
				t.Fatal(openErr)
			}
			defer db.Close()
			if test.binding != nil {
				test.binding(mock)
			} else {
				mock.ExpectQuery("SELECT 1").WithArgs(int64(9), int64(7)).WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
				mock.ExpectQuery("SELECT tenant_id, provider, base_url").WithArgs(int64(7)).WillReturnRows(test.row())
			}
			guard := test.guard
			if guard == nil {
				guard, _ = outboundhttp.NewGuard(outboundhttp.Config{RequireHTTPS: true, Resolver: resolverDNSStub{}})
			}
			cipherManager := test.manager
			if cipherManager == nil {
				cipherManager = manager
			}
			resolver := NewTenantResolver(db, cipherManager, guard)
			resolver.Now = func() time.Time { return now }
			factoryCalls := 0
			resolver.Factory = func(openai.Config) (providers.AIProvider, error) { factoryCalls++; return resolverTestProvider{}, nil }
			_, resolveErr := resolver.Resolve(context.Background(), 7, 9)
			var safe *ResolveError
			if !errors.As(resolveErr, &safe) || safe.Code != test.code || factoryCalls != 0 {
				t.Fatalf("code=%v factoryCalls=%d", safe, factoryCalls)
			}
			for _, forbidden := range []string{secret, ciphertext, hint, "Authorization", "provider.example.test", "/private/path", "token=hidden", "old-global-secret"} {
				if forbidden != "" && strings.Contains(resolveErr.Error(), forbidden) {
					t.Fatalf("safe error leaked fixture category in %s", test.name)
				}
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func resolverRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{"tenant_id", "provider", "base_url", "model", "credential_ciphertext", "encryption_key_id", "api_key_hint", "effective_at", "expires_at", "status", "version"})
}
