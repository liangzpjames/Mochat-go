package identitymigration

import (
	"context"
	"database/sql"
	"strings"
	"testing"
)

func TestPreflightRealMariaDBLegacy0128IsReadOnlyAndUsesLegacySaaSActors(t *testing.T) {
	db := newCredentialIntegrationDB(t)
	createIdentityBackfillEngineFixture(t, db)
	if _, err := db.Exec(`INSERT INTO mc_user (id, tenant_id, phone, password, name, status, isSuperAdmin) VALUES (12, 1, '13800000012', 'legacy-platform-hash-12', 'Platform operator', 1, 0)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO mochat_go_saas_admin_user_access (user_id) VALUES (12)`); err != nil {
		t.Fatal(err)
	}

	var saasIdentityTables int
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name='mochat_go_saas_admin_users'`).Scan(&saasIdentityTables); err != nil {
		t.Fatal(err)
	}
	if saasIdentityTables != 0 {
		t.Fatalf("legacy 0128 fixture unexpectedly has SaaS identity table count=%d", saasIdentityTables)
	}
	before := snapshotLegacyPreflightCounts(t, db)
	platformDuplicates, platformPhones, invalidPlatformContacts, err := platformContactFindings(context.Background(), db, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(platformDuplicates) != 0 || len(invalidPlatformContacts) != 0 {
		t.Fatalf("legacy platform findings duplicates=%v invalid=%v", platformDuplicates, invalidPlatformContacts)
	}
	if got := platformPhones["13800000012"]; len(got) != 1 || got[0] != 12 {
		t.Fatalf("legacy SaaS actor relation was not included in platform phones: %v", got)
	}

	manager := newCredentialIntegrationManager(t)
	report, err := Preflight(context.Background(), db, DatabaseOptions{PlatformTenantID: 1, CredentialManager: manager})
	if err != nil {
		t.Fatalf("pre-0129 legacy preflight failed: %v", err)
	}
	if report.ActiveDashboardUsers != 1 || report.HasFindings() {
		t.Fatalf("legacy preflight report=%+v, want one active dashboard user and no findings", report)
	}
	after := snapshotLegacyPreflightCounts(t, db)
	if before != after {
		t.Fatalf("legacy preflight changed business counts before=%+v after=%+v", before, after)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name='mochat_go_saas_admin_users'`).Scan(&saasIdentityTables); err != nil {
		t.Fatal(err)
	}
	if saasIdentityTables != 0 {
		t.Fatalf("legacy preflight created SaaS identity table count=%d", saasIdentityTables)
	}
}

func TestPreflightRealMariaDBReportsSaaSIdentityPhoneConflictAfter0129(t *testing.T) {
	db := newCredentialIntegrationDB(t)
	createIdentityBackfillEngineFixture(t, db)
	execIdentityBackfillTestFile(t, db, "0129_identity_realms_single_corp_schema.up.sql")
	if _, err := db.Exec(`INSERT INTO mochat_go_saas_admin_users (id, login_name, phone, password_hash, name, status, must_rotate_password, auth_version, mfa_required) VALUES (99, 'conflicting-saas-user', '13800000011', 'hash', 'Conflict', 1, 1, 1, 1)`); err != nil {
		t.Fatal(err)
	}

	report, err := Preflight(context.Background(), db, DatabaseOptions{PlatformTenantID: 1, CredentialManager: newCredentialIntegrationManager(t)})
	if err == nil || !strings.Contains(err.Error(), "SaaS platform identity phone/login conflict") {
		t.Fatalf("SaaS identity conflict error=%v report=%+v", err, report)
	}
	if len(report.SaaSPhoneConflictUserIDs) != 1 || report.SaaSPhoneConflictUserIDs[0] != 11 {
		t.Fatalf("SaaS identity conflict report=%v, want legacy platform user 11", report.SaaSPhoneConflictUserIDs)
	}
}

func TestPreflightRealMariaDBFailsClosedForBrokenSaaSIdentitySchema(t *testing.T) {
	db := newCredentialIntegrationDB(t)
	createIdentityBackfillEngineFixture(t, db)
	execIdentityBackfillTestFile(t, db, "0129_identity_realms_single_corp_schema.up.sql")
	if _, err := db.Exec(`ALTER TABLE mochat_go_saas_admin_users DROP INDEX uni_saas_admin_user_phone, DROP COLUMN phone`); err != nil {
		t.Fatal(err)
	}

	_, err := Preflight(context.Background(), db, DatabaseOptions{PlatformTenantID: 1, CredentialManager: newCredentialIntegrationManager(t)})
	if err == nil || !strings.Contains(err.Error(), "SaaS identity table preflight schema failed") || !strings.Contains(err.Error(), "phone") {
		t.Fatalf("broken SaaS identity schema error=%v, want fail-closed phone contract", err)
	}
}

type legacyPreflightCounts struct {
	Users       int
	Corps       int
	Roles       int
	UserRoles   int
	DashAudits  int
	IdentityLed int
}

func snapshotLegacyPreflightCounts(t *testing.T, db *sql.DB) legacyPreflightCounts {
	t.Helper()
	var counts legacyPreflightCounts
	queries := []*int{
		&counts.Users,
		&counts.Corps,
		&counts.Roles,
		&counts.UserRoles,
		&counts.DashAudits,
		&counts.IdentityLed,
	}
	for index, query := range []string{
		`SELECT COUNT(*) FROM mc_user`,
		`SELECT COUNT(*) FROM mc_corp`,
		`SELECT COUNT(*) FROM mc_rbac_role`,
		`SELECT COUNT(*) FROM mc_rbac_user_role`,
		`SELECT COUNT(*) FROM mochat_go_dashboard_permission_audits`,
		`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name IN ('mochat_go_identity_migration_ledger', 'mochat_go_identity_migration_batches', 'mochat_go_identity_migration_journal')`,
	} {
		if err := db.QueryRow(query).Scan(queries[index]); err != nil {
			t.Fatal(err)
		}
	}
	return counts
}
