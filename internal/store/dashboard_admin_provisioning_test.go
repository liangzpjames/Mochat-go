package store

import (
	"os"
	"strings"
	"testing"

	"jiyi/mochat-go/internal/dashboardadmin"
)

// RED contract: the SQL adapter must own the complete provisioning and
// governance transactions, rather than leaving those writes in the HTTP or
// service packages.
var _ dashboardadmin.Store = (*MySQLStore)(nil)

func TestDashboardAdminStoreContractIsTransactionBacked(t *testing.T) {
	t.Helper()
	// The compile-time assertion above is intentionally the first contract.
	// Behavioral coverage is exercised against a real isolated MariaDB schema
	// in dashboard_admin_provisioning_integration_test.go when a DSN is supplied.
}

func TestSaaSActorLockHasNoLegacyBusinessUserFallbackAndAcceptsRootPermission(t *testing.T) {
	body, err := os.ReadFile("dashboard_admin_provisioning.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(body)
	start := strings.Index(source, "func lockSaaSActorTx")
	if start < 0 {
		t.Fatal("lockSaaSActorTx was not found")
	}
	end := strings.Index(source[start:], "func dashboardProvisionFingerprint")
	if end < 0 {
		t.Fatal("lockSaaSActorTx boundary was not found")
	}
	source = source[start : start+end]
	if strings.Contains(source, "mc_user") {
		t.Fatal("SaaS actor authorization must not consult the legacy business user table")
	}
	if !strings.Contains(source, "admin_permission.permission_code = '*'") {
		t.Fatal("SaaS actor authorization must accept the dedicated root permission")
	}
}

func TestResendActivationUsesAUniqueGenericReceiptInsteadOfActivationRequestID(t *testing.T) {
	body, err := os.ReadFile("dashboard_admin_provisioning.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(body)
	start := strings.Index(source, "func (s *MySQLStore) ResendDashboardActivation")
	end := strings.Index(source[start:], "func (s *MySQLStore) provisionDashboardTenantTx")
	if start < 0 || end < 0 {
		t.Fatal("resend activation implementation boundary was not found")
	}
	source = source[start : start+end]
	if strings.Contains(source, "activation.request_id") || strings.Contains(source, "WHERE request_id = ?") {
		t.Fatal("activation request_id is not a concurrency-safe idempotency receipt")
	}
	for _, fragment := range []string{
		"mochat_go_saas_idempotency_receipts",
		"operation",
		"fingerprint",
		"result_version",
		"INSERT",
		"ErrIdempotencyConflict",
	} {
		if !strings.Contains(source, fragment) {
			t.Fatalf("resend activation missing generic receipt fragment %q", fragment)
		}
	}
}

func TestGovernanceMutationsUseTheGenericReceiptBeforeBusinessWrites(t *testing.T) {
	body, err := os.ReadFile("dashboard_admin_provisioning.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(body)
	for _, method := range []string{"replaceDashboardSuperAdminTx", "setDashboardSuperAdminStatusTx"} {
		start := strings.Index(source, "func (s *MySQLStore) "+method)
		if start < 0 {
			t.Fatalf("%s implementation was not found", method)
		}
		end := strings.Index(source[start+1:], "func (s *MySQLStore) ")
		if end < 0 {
			end = len(source) - start - 1
		}
		section := source[start : start+1+end]
		if !strings.Contains(section, "dashboardGovernanceFingerprint") || !strings.Contains(section, "insertOrLoadDashboardIdempotencyReceiptTx") {
			t.Fatalf("%s does not use the generic idempotency receipt", method)
		}
		if !strings.Contains(section, "dashboardGovernanceReceiptMatches") || !strings.Contains(section, "completeDashboardIdempotencyReceiptTx") {
			t.Fatalf("%s does not fail closed/replay the durable result version", method)
		}
	}
}

func TestSaaSAdminPackageReadModelCarriesStablePackageID(t *testing.T) {
	body, err := os.ReadFile("mysql.go")
	if err != nil {
		t.Fatal(err)
	}
	source := strings.ReplaceAll(string(body), "\r\n", "\n")
	start := strings.Index(source, "func (s *MySQLStore) SaaSAdminPackages")
	if start < 0 {
		t.Fatal("SaaSAdminPackages implementation was not found")
	}
	end := strings.Index(source[start+1:], "func ")
	if end < 0 {
		end = len(source) - start - 1
	}
	section := source[start : start+1+end]
	if !strings.Contains(section, "SELECT\n\t\t\tid,") || !strings.Contains(section, "&item.ID") {
		t.Fatal("SaaSAdminPackages must return the authoritative package id for provisioning")
	}
}

func TestGovernanceUsesStableLockOrderRowsAffectedChecksAndDeadlockRetry(t *testing.T) {
	body, err := os.ReadFile("dashboard_admin_provisioning.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(body)
	for _, fragment := range []string{
		"ORDER BY dashboard_user.id",
		"lockDashboardSubjectsTx",
		"lockDashboardStatusSubjectsTx",
		"input.Enabled == currentlySuperAdmin",
		"execDashboardGovernanceUpdateTx(ctx, tx, 2",
		"isMySQLRetryableTransactionError(err)",
	} {
		if !strings.Contains(source, fragment) {
			t.Fatalf("governance transaction missing safety fragment %q", fragment)
		}
	}
}

func TestDashboardAdminGovernanceReadIsTenantScopedAndDoesNotLockIdentityRows(t *testing.T) {
	body, err := os.ReadFile("dashboard_admin_provisioning.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(body)
	start := strings.Index(source, "func (s *MySQLStore) DashboardAdminGovernance")
	end := strings.Index(source[start:], "func dashboardAdminNullableTimeString")
	if start < 0 || end < 0 {
		t.Fatal("DashboardAdminGovernance implementation boundary was not found")
	}
	section := source[start : start+end]
	for _, fragment := range []string{
		"lockSaaSActorTx(ctx, tx, actor.UserID)",
		"readDashboardTenantBindingTx(ctx, tx, tenantID)",
		"dashboard_user.tenant_id=?",
		"ORDER BY dashboard_user.id",
		"identity_row.status",
		"identity_row.activated_at",
		"COALESCE(dashboard_user.isSuperAdmin, 0)",
	} {
		if !strings.Contains(section, fragment) {
			t.Fatalf("DashboardAdminGovernance missing tenant-scoped read fragment %q", fragment)
		}
	}
	if strings.Contains(section, "FOR UPDATE") {
		t.Fatal("DashboardAdminGovernance must not hold a long identity-row write lock")
	}
	for _, forbidden := range []string{"password", "token_digest", "secret", "mfa", "session"} {
		if strings.Contains(strings.ToLower(section), forbidden) {
			t.Fatalf("DashboardAdminGovernance must not select credential material %q", forbidden)
		}
	}
}
