package store

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
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
