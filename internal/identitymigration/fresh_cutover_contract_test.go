package identitymigration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFreshSplitIdentityPlatformTenantGuardContract(t *testing.T) {
	if !platformTenantGuardAllowed(1, 0, 0, 0, 0, 1) {
		t.Fatal("fresh split-identity SaaS bootstrap must not require a legacy platform tenant")
	}
	for name, counts := range map[string][5]int64{
		"legacy user":       {0, 1, 0, 0, 1},
		"legacy tenant":     {0, 0, 1, 0, 1},
		"legacy corp":       {0, 0, 1, 0, 1},
		"dangling actor":    {0, 0, 0, 1, 1},
		"missing bootstrap": {0, 0, 0, 0, 0},
	} {
		t.Run(name, func(t *testing.T) {
			if platformTenantGuardAllowed(1, counts[0], counts[1], counts[2], counts[3], counts[4]) {
				t.Fatal("legacy or incomplete fresh state must fail closed")
			}
		})
	}
	if !platformTenantGuardAllowed(1, 1, 0, 0, 0, 0) {
		t.Fatal("legacy mode with an explicit active platform tenant must remain valid")
	}
}

func TestValidatedBatchContractAllowsOnlyExactRetryFacts(t *testing.T) {
	want := validatedBatchContract{
		PlatformTenantID: 1,
		Status:           "validated",
		MappingDigest:    "mapping-digest",
		ScriptChecksum:   "script-checksum",
		PreflightStatus:  "passed",
		CredentialStatus: "verified",
		ActorStatus:      "verified",
		MigrationSource:  migrationSource,
	}
	if !validatedBatchCompatible(want, want) {
		t.Fatal("identical validated batch facts must be reusable")
	}
	for name, mutate := range map[string]func(*validatedBatchContract){
		"platform tenant": func(value *validatedBatchContract) { value.PlatformTenantID = 2 },
		"mapping digest":  func(value *validatedBatchContract) { value.MappingDigest = "other" },
		"script checksum": func(value *validatedBatchContract) { value.ScriptChecksum = "other" },
		"status":          func(value *validatedBatchContract) { value.Status = "completed" },
		"preflight":       func(value *validatedBatchContract) { value.PreflightStatus = "failed" },
		"credentials":     func(value *validatedBatchContract) { value.CredentialStatus = "pending" },
		"actor inventory": func(value *validatedBatchContract) { value.ActorStatus = "unknown" },
		"source":          func(value *validatedBatchContract) { value.MigrationSource = "other" },
	} {
		t.Run(name, func(t *testing.T) {
			got := want
			mutate(&got)
			if validatedBatchCompatible(got, want) {
				t.Fatal("mismatched staging facts must conflict")
			}
		})
	}
}

func TestBackfillStatementClassificationProtectsPlatformTenantPreflight(t *testing.T) {
	statement := "SET @identity_0130_platform_tenant_guard_sql := IF(...)"
	if got := statementPhase(statement); got != "preflight" {
		t.Fatalf("phase = %q, want preflight", got)
	}
	if got := statementLabel(statement); got != "platform_tenant" {
		t.Fatalf("label = %q, want platform_tenant", got)
	}
}

func TestFreshSplitIdentitySQLGuardContract(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "deploy", "standalone", "migrations", "0130_identity_realms_single_corp_backfill.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	sql := string(body)
	for _, required := range []string{
		"@identity_0130_legacy_user_count",
		"@identity_0130_legacy_tenant_fact_count",
		"@identity_0130_dangling_saas_actor_count",
		"@identity_0130_active_bootstrap_root_count",
		"@identity_0130_fresh_split_identity_mode",
		"@identity_0130_fresh_split_identity_mode = 1",
		"0130 explicit platform tenant is invalid",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("0130 fresh split-identity guard is missing %q", required)
		}
	}
	guardIndex := strings.Index(sql, "SET @identity_0130_platform_tenant_guard_sql := IF(")
	freshModeIndex := strings.Index(sql, "SET @identity_0130_fresh_split_identity_mode := IF(")
	if freshModeIndex < 0 || guardIndex < 0 || freshModeIndex > guardIndex {
		t.Fatalf("fresh split-identity mode must be computed before the platform tenant guard: mode=%d guard=%d", freshModeIndex, guardIndex)
	}
}

func TestBackfillSQLBindsAndValidatesTheRequestedBatchContract(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "deploy", "standalone", "migrations", "0130_identity_realms_single_corp_backfill.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	sql := string(body)
	for _, required := range []string{
		"@identity_0130_requested_request_id",
		"@identity_0130_requested_batch_count",
		"@identity_0130_requested_validated_batch_count",
		"@identity_0130_requested_batch_count = 1",
		"@identity_0130_requested_validated_batch_count = 1",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("0130 request-scoped batch guard is missing %q", required)
		}
	}
	if strings.Contains(sql, "ORDER BY request_id") {
		t.Fatal("0130 must not guess the migration request with ORDER BY request_id")
	}
}
