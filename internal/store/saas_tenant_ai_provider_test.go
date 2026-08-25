package store

import (
	"context"
	"encoding/base64"
	"errors"
	"net/netip"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	mysqlDriver "github.com/go-sql-driver/mysql"

	"jiyi/mochat-go/internal/aiproviderconfig"
	"jiyi/mochat-go/internal/dashboard"
	"jiyi/mochat-go/internal/outboundhttp"
)

func TestTenantAIProviderStoreInputRejectsMissingInitialKey(t *testing.T) {
	input := dashboard.SaaSTenantAIProviderInput{
		TenantID: 9, ProviderCode: "openai", BaseURL: "https://api.example.test/v1", Model: "model-v1",
		EffectiveAt: "2026-08-25T00:00:00Z", ExpiresAt: "2026-08-26T00:00:00Z", Status: "active", Version: 0,
	}
	if err := dashboard.ValidateSaaSTenantAIProviderCreate(input); err == nil {
		t.Fatal("first configuration without API key was accepted")
	}
}

type tenantAIProviderResolver struct{ addresses []netip.Addr }

func (r tenantAIProviderResolver) LookupNetIP(context.Context, string, string) ([]netip.Addr, error) {
	return r.addresses, nil
}

func TestTenantAIProviderStoreRejectsPrivateResolvedBaseURLBeforeWrite(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	manager, err := aiproviderconfig.NewManager(aiproviderconfig.Config{EncryptionKey: base64.RawStdEncoding.EncodeToString(make([]byte, 32)), EncryptionKeyID: "test"})
	if err != nil {
		t.Fatal(err)
	}
	guard, err := outboundhttp.NewGuard(outboundhttp.Config{RequireHTTPS: true, Resolver: tenantAIProviderResolver{addresses: []netip.Addr{netip.MustParseAddr("10.0.0.9")}}})
	if err != nil {
		t.Fatal(err)
	}
	store := NewMySQLStore(db).WithAIProviderCredentialCipher(manager).WithAIProviderOutboundGuard(guard)
	_, err = store.SaveSaaSTenantAIProvider(context.Background(), dashboard.SaaSTenantAIProviderInput{TenantID: 9, ProviderCode: "openai", BaseURL: "https://provider.example.test/v1", Model: "model-v1", APIKey: "fixture-key-1234", EffectiveAt: "2026-08-25T00:00:00Z", ExpiresAt: "2026-08-26T00:00:00Z", Status: "active"})
	if err == nil {
		t.Fatal("private resolved base URL was accepted")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestTenantAIProviderStoreRollsBackWhenAuditWriteFails(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	manager, err := aiproviderconfig.NewManager(aiproviderconfig.Config{EncryptionKey: base64.RawStdEncoding.EncodeToString(make([]byte, 32)), EncryptionKeyID: "test"})
	if err != nil {
		t.Fatal(err)
	}
	guard, err := outboundhttp.NewGuard(outboundhttp.Config{RequireHTTPS: true, Resolver: tenantAIProviderResolver{addresses: []netip.Addr{netip.MustParseAddr("8.8.8.8")}}})
	if err != nil {
		t.Fatal(err)
	}
	store := NewMySQLStore(db).WithAIProviderCredentialCipher(manager).WithAIProviderOutboundGuard(guard)
	columns := []string{"tenant_id", "provider", "base_url", "model", "credential_ciphertext", "encryption_key_id", "api_key_hint", "effective_at", "expires_at", "status", "version", "updated_at"}
	now := time.Date(2026, 8, 25, 1, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM mc_tenant WHERE id = ? AND deleted_at IS NULL LIMIT 1")).WithArgs(9).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(9))
	mock.ExpectQuery("SELECT tenant_id, provider, base_url").WithArgs(9).WillReturnRows(sqlmock.NewRows(columns))
	mock.ExpectExec("INSERT INTO mochat_go_saas_tenant_ai_providers").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT tenant_id, provider, base_url").WithArgs(9).WillReturnRows(sqlmock.NewRows(columns).AddRow(9, "openai", "https://provider.example.test/v1", "model-v1", "not-a-secret", "test", "1234", now, now.Add(time.Hour), "active", 1, now))
	mock.ExpectExec("INSERT IGNORE INTO mochat_go_saas_admin_audit_chains").WithArgs(9, sqlmock.AnyArg(), sqlmock.AnyArg()).WillReturnError(&mysqlDriver.MySQLError{Number: 1146, Message: "Table 'mochat_go_saas_admin_audit_chains' doesn't exist"})
	mock.ExpectExec("INSERT INTO mochat_go_saas_admin_operation_logs").WillReturnError(errors.New("audit write failed"))
	mock.ExpectRollback()
	_, err = store.SaveSaaSTenantAIProvider(context.Background(), dashboard.SaaSTenantAIProviderInput{TenantID: 9, ProviderCode: "openai", BaseURL: "https://provider.example.test/v1", Model: "model-v1", APIKey: "fixture-key-1234", EffectiveAt: "2026-08-25T00:00:00Z", ExpiresAt: "2026-08-26T00:00:00Z", Status: "active"})
	if err == nil {
		t.Fatal("audit failure was accepted")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestTenantAIProviderStoreMapsConcurrentCreateDuplicateToConflict(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	manager, err := aiproviderconfig.NewManager(aiproviderconfig.Config{EncryptionKey: base64.RawStdEncoding.EncodeToString(make([]byte, 32)), EncryptionKeyID: "test"})
	if err != nil {
		t.Fatal(err)
	}
	guard, err := outboundhttp.NewGuard(outboundhttp.Config{RequireHTTPS: true, Resolver: tenantAIProviderResolver{addresses: []netip.Addr{netip.MustParseAddr("8.8.8.8")}}})
	if err != nil {
		t.Fatal(err)
	}
	store := NewMySQLStore(db).WithAIProviderCredentialCipher(manager).WithAIProviderOutboundGuard(guard)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM mc_tenant WHERE id = ? AND deleted_at IS NULL LIMIT 1")).WithArgs(9).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(9))
	mock.ExpectQuery("SELECT tenant_id, provider, base_url").WithArgs(9).WillReturnRows(sqlmock.NewRows([]string{"tenant_id", "provider", "base_url", "model", "credential_ciphertext", "encryption_key_id", "api_key_hint", "effective_at", "expires_at", "status", "version", "updated_at"}))
	mock.ExpectExec("INSERT INTO mochat_go_saas_tenant_ai_providers").WillReturnError(&mysqlDriver.MySQLError{Number: 1062, Message: "duplicate tenant"})
	mock.ExpectRollback()
	_, err = store.SaveSaaSTenantAIProvider(context.Background(), dashboard.SaaSTenantAIProviderInput{TenantID: 9, ProviderCode: "openai", BaseURL: "https://provider.example.test/v1", Model: "model-v1", APIKey: "fixture-key-1234", EffectiveAt: "2026-08-25T00:00:00Z", ExpiresAt: "2026-08-26T00:00:00Z", Status: "active"})
	var operationErr *dashboard.SaaSAdminOperationError
	if !errors.As(err, &operationErr) || operationErr.Status != 409 {
		t.Fatalf("error=%v, want conflict", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestTenantAIProviderStoreReadRedactsCiphertextAndReportsUnavailableCredential(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	manager, err := aiproviderconfig.NewManager(aiproviderconfig.Config{EncryptionKey: base64.RawStdEncoding.EncodeToString(make([]byte, 32)), EncryptionKeyID: "test"})
	if err != nil {
		t.Fatal(err)
	}
	store := NewMySQLStore(db).WithAIProviderCredentialCipher(manager)
	now := time.Date(2026, 8, 25, 1, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM mc_tenant WHERE id = ? AND deleted_at IS NULL LIMIT 1")).WithArgs(9).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(9))
	mock.ExpectQuery("SELECT tenant_id, provider, base_url").WithArgs(9).WillReturnRows(sqlmock.NewRows([]string{"tenant_id", "provider", "base_url", "model", "credential_ciphertext", "encryption_key_id", "api_key_hint", "effective_at", "expires_at", "status", "version", "updated_at"}).AddRow(9, "openai", "https://api.example.test/v1", "model-v1", "corrupt", "unknown", "1234", now, now.Add(time.Hour), "active", 1, now))
	got, found, err := store.SaaSTenantAIProvider(context.Background(), 9)
	if err != nil || !found {
		t.Fatalf("read error=%v found=%v", err, found)
	}
	if got.CredentialProtection != "unavailable" || !got.APIKeyConfigured {
		t.Fatalf("protection=%q configured=%v", got.CredentialProtection, got.APIKeyConfigured)
	}
	if got.APIKeyHint != "1234" || got.BaseURL == "" {
		t.Fatal("expected redacted public fields were lost")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
