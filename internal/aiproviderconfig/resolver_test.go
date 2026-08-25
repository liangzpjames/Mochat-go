package aiproviderconfig

import (
	"context"
	"database/sql/driver"
	"errors"
	"net/netip"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"jiyi/mochat-go/internal/modules/providers"
	"jiyi/mochat-go/internal/outboundhttp"
)

type resolverDNSStub struct{}

func (resolverDNSStub) LookupNetIP(context.Context, string, string) ([]netip.Addr, error) {
	return []netip.Addr{netip.MustParseAddr("8.8.8.8")}, nil
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
		LIMIT 1`)).WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"tenant_id", "provider", "base_url", "model", "credential_ciphertext", "encryption_key_id", "api_key_hint", "effective_at", "expires_at", "status", "version"}).AddRow(7, "deepseek", "https://provider.example.test/v1", "deepseek-chat", ciphertext, keyID, "1234", driver.Value(nil), driver.Value(nil), StatusActive, 1))

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
			return resolverRows().AddRow(7, "openai", "https://provider.example.test/v1", "model", validCiphertext, keyID, "9876", nil, now, StatusActive, 1)
		}, code: ErrorExpired},
		{name: "unavailable key", row: func() *sqlmock.Rows {
			return resolverRows().AddRow(7, "openai", "https://provider.example.test/v1", "model", validCiphertext, "missing-key", "9876", nil, nil, StatusActive, 1)
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

func resolverRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{"tenant_id", "provider", "base_url", "model", "credential_ciphertext", "encryption_key_id", "api_key_hint", "effective_at", "expires_at", "status", "version"})
}
