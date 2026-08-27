package store

import (
	"context"
	"database/sql"
	"encoding/base64"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"jiyi/mochat-go/internal/wecomcredentials"
)

func TestCompanyBindingUsesSelfBuiltWeComFailsClosedForDelegatedMode(t *testing.T) {
	if !companyBindingUsesSelfBuiltWeCom(companyBindingRecord{WeComIntegrationMode: "self_built"}) {
		t.Fatal("self-built binding must be writable from Dashboard")
	}
	if !companyBindingUsesSelfBuiltWeCom(companyBindingRecord{}) {
		t.Fatal("legacy binding without a mode must remain self-built compatible")
	}
	if companyBindingUsesSelfBuiltWeCom(companyBindingRecord{WeComIntegrationMode: "third_party_delegated"}) {
		t.Fatal("delegated binding must reject Dashboard credential writes")
	}
	if companyBindingUsesSelfBuiltWeCom(companyBindingRecord{WeComIntegrationMode: "unexpected"}) {
		t.Fatal("unknown binding mode must fail closed")
	}
}

func TestCompanyProfileReadDegradesSafelyWhenStoredCredentialKeyWasRetired(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	manager, err := wecomcredentials.NewManager(wecomcredentials.Config{
		EncryptionKey:   base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef")),
		EncryptionKeyID: "current-key",
	})
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT a.id, a.corp_id, c.tenant_id")).
		WithArgs(9, 17).
		WillReturnRows(sqlmock.NewRows([]string{"id", "corp_id", "tenant_id", "wx_agent_id", "ciphertext", "key_id", "updated_at"}))

	profile, err := (&MySQLStore{weComCredentialCipher: manager}).companyProfileFromBinding(context.Background(), db, companyBindingRecord{
		TenantID: 9, CorpID: 17, WeComIntegrationMode: "self_built", Status: 1, Version: 3,
		DisplayName: "旧密钥租户", LegacyWXCorpID: "ww-retired", Ciphertext: "retired-ciphertext", KeyID: "retired-key",
		UpdatedAt: sql.NullTime{Time: time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC), Valid: true},
	})
	if err != nil {
		t.Fatalf("read-only company profile must remain available for credential repair: %v", err)
	}
	if !profile.Credentials.WeCom.Configured {
		t.Fatalf("stored encrypted credential presence was lost: %+v", profile.Credentials)
	}
	if profile.Credentials.Archive.Configured || profile.Credentials.Callback.Configured {
		t.Fatalf("unreadable credential must not claim verified capability readiness: %+v", profile.Credentials)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCompanyProfileDelegatedModeDoesNotReadLegacySelfBuiltCredentials(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	profile, err := (&MySQLStore{}).companyProfileFromBinding(context.Background(), db, companyBindingRecord{
		TenantID: 9, CorpID: 17, WeComIntegrationMode: "third_party_delegated", Status: 1, Version: 3,
		DisplayName: "委托应用租户", Ciphertext: "legacy-self-built-ciphertext", KeyID: "legacy-key",
		UpdatedAt: sql.NullTime{Time: time.Date(2026, 8, 27, 0, 0, 0, 0, time.UTC), Valid: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if profile.WeComIntegrationMode != "third_party_delegated" || profile.Credentials.WeCom.Configured || profile.Credentials.Agent.Configured || profile.Credentials.Archive.Configured || profile.Credentials.Callback.Configured {
		t.Fatalf("delegated profile exposed legacy credential readiness: %+v", profile)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
