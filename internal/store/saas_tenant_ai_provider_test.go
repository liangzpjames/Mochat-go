package store

import (
	"context"
	"database/sql/driver"
	"encoding/base64"
	"errors"
	"net/netip"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	mysqlDriver "github.com/go-sql-driver/mysql"

	"jiyi/mochat-go/internal/aiproviderconfig"
	"jiyi/mochat-go/internal/dashboard"
	"jiyi/mochat-go/internal/outboundhttp"
)

const tenantAIProviderFixtureKey = "fixture-key-1234"

type tenantAIProviderEncryptedCredentialArgument struct {
	manager    *aiproviderconfig.Manager
	tenantID   int
	provider   string
	plaintext  string
	oldCipher  string
	ciphertext string
}

func (m *tenantAIProviderEncryptedCredentialArgument) Match(value driver.Value) bool {
	ciphertext, ok := value.(string)
	if !ok || ciphertext == "" || ciphertext == m.plaintext || ciphertext == m.oldCipher {
		return false
	}
	plain, err := m.manager.Decrypt(m.tenantID, m.provider, "test", ciphertext)
	if err != nil || plain != m.plaintext {
		return false
	}
	m.ciphertext = ciphertext
	return true
}

type tenantAIProviderAuditArgument struct{ keyChanged bool }

func (m tenantAIProviderAuditArgument) Match(value driver.Value) bool {
	raw, ok := value.(string)
	if !ok || !strings.Contains(raw, `"keyChanged":`+strconv.FormatBool(m.keyChanged)) {
		return false
	}
	for _, prohibited := range []string{tenantAIProviderFixtureKey, "1234", "https://provider.example.test/private", "ciphertext", "apiKey"} {
		if strings.Contains(raw, prohibited) {
			return false
		}
	}
	return true
}

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

func newTenantAIProviderSuccessStore(t *testing.T) (*MySQLStore, sqlmock.Sqlmock, *aiproviderconfig.Manager) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	manager, err := aiproviderconfig.NewManager(aiproviderconfig.Config{EncryptionKey: base64.RawStdEncoding.EncodeToString(make([]byte, 32)), EncryptionKeyID: "test"})
	if err != nil {
		t.Fatal(err)
	}
	guard, err := outboundhttp.NewGuard(outboundhttp.Config{RequireHTTPS: true, Resolver: tenantAIProviderResolver{addresses: []netip.Addr{netip.MustParseAddr("8.8.8.8")}}})
	if err != nil {
		t.Fatal(err)
	}
	return NewMySQLStore(db).WithAIProviderCredentialCipher(manager).WithAIProviderOutboundGuard(guard), mock, manager
}

func tenantAIProviderStoredColumns() []string {
	return []string{"tenant_id", "provider", "base_url", "model", "credential_ciphertext", "encryption_key_id", "api_key_hint", "effective_at", "expires_at", "status", "version", "updated_at"}
}

func expectTenantAIProviderLegacyAudit(mock sqlmock.Sqlmock, provider string, keyChanged bool, actorUserID, actorTenantID int, before any) {
	mock.ExpectExec("INSERT IGNORE INTO mochat_go_saas_admin_audit_chains").WithArgs(9, sqlmock.AnyArg(), sqlmock.AnyArg()).WillReturnError(&mysqlDriver.MySQLError{Number: 1146, Message: "Table 'mochat_go_saas_admin_audit_chains' doesn't exist"})
	mock.ExpectExec("INSERT INTO mochat_go_saas_admin_operation_logs").WithArgs(
		9, actorUserID, actorTenantID, dashboard.SaaSAdminOperationActionTenantAIProviderSave, dashboard.SaaSAdminOperationTargetTenantAIProvider,
		"9", provider, before, tenantAIProviderAuditArgument{keyChanged: keyChanged}, "tenant AI provider saved",
	).WillReturnResult(sqlmock.NewResult(11, 1))
}

func TestTenantAIProviderStoreEmptyKeyUpdatePreservesCredentialAndAuditsInTransaction(t *testing.T) {
	store, mock, manager := newTenantAIProviderSuccessStore(t)
	oldCiphertext, keyID, hint, err := manager.Encrypt(9, "openai", tenantAIProviderFixtureKey)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 25, 1, 0, 0, 0, time.UTC)
	columns := tenantAIProviderStoredColumns()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM mc_tenant WHERE id = ? AND deleted_at IS NULL LIMIT 1")).WithArgs(9).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(9))
	mock.ExpectQuery("SELECT tenant_id, provider, base_url").WithArgs(9).WillReturnRows(sqlmock.NewRows(columns).AddRow(9, "openai", "https://provider.example.test/private", "model-v1", oldCiphertext, keyID, hint, now, now.Add(time.Hour), "active", 1, now))
	mock.ExpectExec("UPDATE mochat_go_saas_tenant_ai_providers").WithArgs("openai", "https://provider.example.test/private", "model-v2", oldCiphertext, keyID, hint, sqlmock.AnyArg(), sqlmock.AnyArg(), "active", 44, 9, 1).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT tenant_id, provider, base_url").WithArgs(9).WillReturnRows(sqlmock.NewRows(columns).AddRow(9, "openai", "https://provider.example.test/private", "model-v2", oldCiphertext, keyID, hint, now, now.Add(time.Hour), "active", 2, now))
	expectTenantAIProviderLegacyAudit(mock, "openai", false, 44, 1, tenantAIProviderAuditArgument{keyChanged: false})
	mock.ExpectCommit()
	got, err := store.SaveSaaSTenantAIProvider(context.Background(), dashboard.SaaSTenantAIProviderInput{TenantID: 9, ProviderCode: "openai", BaseURL: "https://provider.example.test/private", Model: "model-v2", EffectiveAt: "2026-08-25T00:00:00Z", ExpiresAt: "2026-08-26T00:00:00Z", Status: "active", Version: 1, ActorUserID: 44, ActorTenantID: 1})
	if err != nil || got.Version != 2 {
		t.Fatal("empty-key update did not return the committed next version")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestTenantAIProviderStoreReplacementKeyEncryptsAndRedactsAudit(t *testing.T) {
	store, mock, manager := newTenantAIProviderSuccessStore(t)
	oldCiphertext, oldKeyID, oldHint, err := manager.Encrypt(9, "openai", tenantAIProviderFixtureKey)
	if err != nil {
		t.Fatal(err)
	}
	afterCiphertext, afterKeyID, afterHint, err := manager.Encrypt(9, "deepseek", "fixture-key-5678")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 25, 1, 0, 0, 0, time.UTC)
	columns := tenantAIProviderStoredColumns()
	matcher := &tenantAIProviderEncryptedCredentialArgument{manager: manager, tenantID: 9, provider: "deepseek", plaintext: "fixture-key-5678", oldCipher: oldCiphertext}
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM mc_tenant WHERE id = ? AND deleted_at IS NULL LIMIT 1")).WithArgs(9).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(9))
	mock.ExpectQuery("SELECT tenant_id, provider, base_url").WithArgs(9).WillReturnRows(sqlmock.NewRows(columns).AddRow(9, "openai", "https://provider.example.test/private", "model-v1", oldCiphertext, oldKeyID, oldHint, now, now.Add(time.Hour), "active", 1, now))
	mock.ExpectExec("UPDATE mochat_go_saas_tenant_ai_providers").WithArgs("deepseek", "https://provider.example.test/private", "model-v2", matcher, "test", "5678", sqlmock.AnyArg(), sqlmock.AnyArg(), "active", 44, 9, 1).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT tenant_id, provider, base_url").WithArgs(9).WillReturnRows(sqlmock.NewRows(columns).AddRow(9, "deepseek", "https://provider.example.test/private", "model-v2", afterCiphertext, afterKeyID, afterHint, now, now.Add(time.Hour), "active", 2, now))
	expectTenantAIProviderLegacyAudit(mock, "deepseek", true, 44, 1, tenantAIProviderAuditArgument{keyChanged: false})
	mock.ExpectCommit()
	got, err := store.SaveSaaSTenantAIProvider(context.Background(), dashboard.SaaSTenantAIProviderInput{TenantID: 9, ProviderCode: "deepseek", BaseURL: "https://provider.example.test/private", Model: "model-v2", APIKey: "fixture-key-5678", EffectiveAt: "2026-08-25T00:00:00Z", ExpiresAt: "2026-08-26T00:00:00Z", Status: "active", Version: 1, ActorUserID: 44, ActorTenantID: 1})
	if err != nil || got.Version != 2 || matcher.ciphertext == "" {
		t.Fatal("replacement-key update was not encrypted and committed")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestTenantAIProviderStoreCreateEncryptsCredentialAndAuditsInTransaction(t *testing.T) {
	store, mock, manager := newTenantAIProviderSuccessStore(t)
	matcher := &tenantAIProviderEncryptedCredentialArgument{manager: manager, tenantID: 9, provider: "openai", plaintext: tenantAIProviderFixtureKey}
	storedCiphertext, storedKeyID, storedHint, err := manager.Encrypt(9, "openai", tenantAIProviderFixtureKey)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 25, 1, 0, 0, 0, time.UTC)
	columns := tenantAIProviderStoredColumns()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM mc_tenant WHERE id = ? AND deleted_at IS NULL LIMIT 1")).WithArgs(9).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(9))
	mock.ExpectQuery("SELECT tenant_id, provider, base_url").WithArgs(9).WillReturnRows(sqlmock.NewRows(columns))
	mock.ExpectExec("INSERT INTO mochat_go_saas_tenant_ai_providers").WithArgs(9, "openai", "https://provider.example.test/private", "model-v1", matcher, "test", "1234", sqlmock.AnyArg(), sqlmock.AnyArg(), "active", 44, 44).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT tenant_id, provider, base_url").WithArgs(9).WillReturnRows(sqlmock.NewRows(columns).AddRow(9, "openai", "https://provider.example.test/private", "model-v1", storedCiphertext, storedKeyID, storedHint, now, now.Add(time.Hour), "active", 1, now))
	expectTenantAIProviderLegacyAudit(mock, "openai", true, 44, 1, nil)
	mock.ExpectCommit()
	got, err := store.SaveSaaSTenantAIProvider(context.Background(), dashboard.SaaSTenantAIProviderInput{TenantID: 9, ProviderCode: "openai", BaseURL: "https://provider.example.test/private", Model: "model-v1", APIKey: tenantAIProviderFixtureKey, EffectiveAt: "2026-08-25T00:00:00Z", ExpiresAt: "2026-08-26T00:00:00Z", Status: "active", ActorUserID: 44, ActorTenantID: 1})
	if err != nil || got.Version != 1 || matcher.ciphertext == "" {
		t.Fatal("create did not encrypt and commit the provider credential")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
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

func TestTenantAIProviderStoreCreatesConfigurationAndCommitsAudit(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	manager, err := aiproviderconfig.NewManager(aiproviderconfig.Config{EncryptionKey: base64.RawStdEncoding.EncodeToString(make([]byte, 32)), EncryptionKeyID: "test"})
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, keyID, hint, err := manager.Encrypt(9, "openai", "fixture-key-1234")
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
	mock.ExpectQuery("SELECT tenant_id, provider, base_url").WithArgs(9).WillReturnRows(sqlmock.NewRows(columns).AddRow(9, "openai", "https://provider.example.test/v1", "model-v1", ciphertext, keyID, hint, now, now.Add(time.Hour), "active", 1, now))
	mock.ExpectExec("INSERT IGNORE INTO mochat_go_saas_admin_audit_chains").WithArgs(9, sqlmock.AnyArg(), sqlmock.AnyArg()).WillReturnError(&mysqlDriver.MySQLError{Number: 1146, Message: "Table 'mochat_go_saas_admin_audit_chains' doesn't exist"})
	mock.ExpectExec("INSERT INTO mochat_go_saas_admin_operation_logs").WillReturnResult(sqlmock.NewResult(11, 1))
	mock.ExpectCommit()
	got, err := store.SaveSaaSTenantAIProvider(context.Background(), dashboard.SaaSTenantAIProviderInput{TenantID: 9, ProviderCode: "openai", BaseURL: "https://provider.example.test/v1", Model: "model-v1", APIKey: "fixture-key-1234", EffectiveAt: "2026-08-25T00:00:00Z", ExpiresAt: "2026-08-26T00:00:00Z", Status: "active"})
	if err != nil || got.Version != 1 || got.CredentialProtection != "usable" {
		t.Fatal("create-and-audit commit failed")
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

func TestTenantAIProviderStoreRejectsEmptyKeyUpdateWhenCurrentCredentialUnavailable(t *testing.T) {
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
	mock.ExpectQuery("SELECT tenant_id, provider, base_url").WithArgs(9).WillReturnRows(sqlmock.NewRows(columns).AddRow(9, "openai", "https://provider.example.test/v1", "model-v1", "corrupt", "unknown", "1234", now, now.Add(time.Hour), "active", 1, now))
	mock.ExpectRollback()
	_, err = store.SaveSaaSTenantAIProvider(context.Background(), dashboard.SaaSTenantAIProviderInput{TenantID: 9, ProviderCode: "openai", BaseURL: "https://provider.example.test/v1", Model: "model-v2", EffectiveAt: "2026-08-25T00:00:00Z", ExpiresAt: "2026-08-26T00:00:00Z", Status: "active", Version: 1})
	if err == nil || !strings.Contains(err.Error(), "must be replaced") {
		t.Fatal("unavailable-current-credential did not require replacement")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestTenantAIProviderStoreMapsExistingVersionMismatchToConflict(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	manager, err := aiproviderconfig.NewManager(aiproviderconfig.Config{EncryptionKey: base64.RawStdEncoding.EncodeToString(make([]byte, 32)), EncryptionKeyID: "test"})
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, keyID, hint, err := manager.Encrypt(9, "openai", "fixture-key-1234")
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
	mock.ExpectQuery("SELECT tenant_id, provider, base_url").WithArgs(9).WillReturnRows(sqlmock.NewRows(columns).AddRow(9, "openai", "https://provider.example.test/v1", "model-v1", ciphertext, keyID, hint, now, now.Add(time.Hour), "active", 2, now))
	mock.ExpectRollback()
	_, err = store.SaveSaaSTenantAIProvider(context.Background(), dashboard.SaaSTenantAIProviderInput{TenantID: 9, ProviderCode: "openai", BaseURL: "https://provider.example.test/v1", Model: "model-v2", EffectiveAt: "2026-08-25T00:00:00Z", ExpiresAt: "2026-08-26T00:00:00Z", Status: "active", Version: 1})
	var op *dashboard.SaaSAdminOperationError
	if !errors.As(err, &op) || op.Status != 409 {
		t.Fatal("version-mismatch did not return conflict")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestTenantAIProviderStoreMapsZeroRowUpdateToConflict(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	manager, err := aiproviderconfig.NewManager(aiproviderconfig.Config{EncryptionKey: base64.RawStdEncoding.EncodeToString(make([]byte, 32)), EncryptionKeyID: "test"})
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, keyID, hint, err := manager.Encrypt(9, "openai", "fixture-key-1234")
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
	mock.ExpectQuery("SELECT tenant_id, provider, base_url").WithArgs(9).WillReturnRows(sqlmock.NewRows(columns).AddRow(9, "openai", "https://provider.example.test/v1", "model-v1", ciphertext, keyID, hint, now, now.Add(time.Hour), "active", 1, now))
	mock.ExpectExec("UPDATE mochat_go_saas_tenant_ai_providers").WithArgs("openai", "https://provider.example.test/v1", "model-v2", ciphertext, keyID, hint, sqlmock.AnyArg(), sqlmock.AnyArg(), "active", 0, 9, 1).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()
	_, err = store.SaveSaaSTenantAIProvider(context.Background(), dashboard.SaaSTenantAIProviderInput{TenantID: 9, ProviderCode: "openai", BaseURL: "https://provider.example.test/v1", Model: "model-v2", EffectiveAt: "2026-08-25T00:00:00Z", ExpiresAt: "2026-08-26T00:00:00Z", Status: "active", Version: 1})
	var op *dashboard.SaaSAdminOperationError
	if !errors.As(err, &op) || op.Status != 409 {
		t.Fatal("zero-row update did not return conflict")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestTenantAIProviderStoreRejectsMissingTenantAndNoKeyManagerBeforeWrite(t *testing.T) {
	for _, tc := range []struct {
		name       string
		tenantRows *sqlmock.Rows
		manager    *aiproviderconfig.Manager
	}{
		{"tenant-missing", sqlmock.NewRows([]string{"id"}), nil},
		{"manager-unavailable", sqlmock.NewRows([]string{"id"}).AddRow(9), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			manager := tc.manager
			if manager == nil {
				manager, err = aiproviderconfig.NewManager(aiproviderconfig.Config{})
				if err != nil {
					t.Fatal(err)
				}
			}
			guard, err := outboundhttp.NewGuard(outboundhttp.Config{RequireHTTPS: true, Resolver: tenantAIProviderResolver{addresses: []netip.Addr{netip.MustParseAddr("8.8.8.8")}}})
			if err != nil {
				t.Fatal(err)
			}
			store := NewMySQLStore(db).WithAIProviderCredentialCipher(manager).WithAIProviderOutboundGuard(guard)
			mock.ExpectBegin()
			mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM mc_tenant WHERE id = ? AND deleted_at IS NULL LIMIT 1")).WithArgs(9).WillReturnRows(tc.tenantRows)
			if tc.name == "manager-unavailable" {
				mock.ExpectQuery("SELECT tenant_id, provider, base_url").WithArgs(9).WillReturnRows(sqlmock.NewRows([]string{"tenant_id", "provider", "base_url", "model", "credential_ciphertext", "encryption_key_id", "api_key_hint", "effective_at", "expires_at", "status", "version", "updated_at"}))
			}
			mock.ExpectRollback()
			_, err = store.SaveSaaSTenantAIProvider(context.Background(), dashboard.SaaSTenantAIProviderInput{TenantID: 9, ProviderCode: "openai", BaseURL: "https://provider.example.test/v1", Model: "model-v1", APIKey: "fixture-key-1234", EffectiveAt: "2026-08-25T00:00:00Z", ExpiresAt: "2026-08-26T00:00:00Z", Status: "active"})
			if err == nil {
				t.Fatal("failure case accepted")
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
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
