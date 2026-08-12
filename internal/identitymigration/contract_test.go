package identitymigration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidatePreflightRequiresExplicitCorpMappingForAmbiguousTenant(t *testing.T) {
	report := PreflightReport{
		MultiCorpTenantIDs: []int64{42},
	}
	if err := report.Validate(Options{PlatformTenantID: 1}); err == nil || !strings.Contains(err.Error(), "mapping") {
		t.Fatalf("ambiguous tenant validation error=%v, want explicit mapping error", err)
	}

	options := Options{
		PlatformTenantID: 1,
		Mapping:          MappingDocument{Version: 1, Entries: []CorpMapping{{TenantID: 42, CorpID: 9001}}, SignatureVerified: true},
	}
	if err := report.Validate(options); err != nil {
		t.Fatalf("validated mapping rejected: %v", err)
	}
}

func TestValidatePreflightRejectsUnresolvedActorsAndNegativeTenants(t *testing.T) {
	report := PreflightReport{
		UnmappedSaaSActorIDs: []int64{700},
		NegativeTenantIDs:    []int64{-3},
	}
	if err := report.Validate(Options{PlatformTenantID: 1}); err == nil {
		t.Fatal("dirty actor and negative tenant preflight unexpectedly passed")
	}
}

func TestSafePreflightReportContainsOnlyCountsIDsAndRepairClasses(t *testing.T) {
	report := PreflightReport{
		ActiveDashboardUsers:  3,
		DuplicateLoginUserIDs: []int64{10, 11},
		ZeroCorpTenantIDs:     []int64{20},
		RepairClasses:         []string{"DASHBOARD_LOGIN_DUPLICATE", "TENANT_CORP_MAPPING_REQUIRED"},
	}
	output := report.SafeText()
	for _, forbidden := range []string{"phone", "password", "Secret", "token", "ciphertext"} {
		if strings.Contains(output, forbidden) {
			t.Fatalf("safe preflight output leaked %q: %s", forbidden, output)
		}
	}
	for _, required := range []string{"active_dashboard_users=3", "duplicate_login_user_ids=[10 11]", "repair_classes=[DASHBOARD_LOGIN_DUPLICATE TENANT_CORP_MAPPING_REQUIRED]"} {
		if !strings.Contains(output, required) {
			t.Fatalf("safe preflight output missing %q: %s", required, output)
		}
	}
}

func TestSafeCutoverReportContainsNoCredentialMaterialLabels(t *testing.T) {
	report := CutoverPreflightReport{
		MissingDashboardIdentityIDs: []int64{10},
		MissingCorpCiphertextIDs:    []int64{20},
		UndecryptableAgentIDs:       []int64{30},
		MissingPrerequisiteFacts:    []string{"backfill_success"},
		LegacyPasswordColumnPresent: true,
	}
	output := report.SafeText()
	for _, forbidden := range []string{"phone", "password", "secret", "token", "ciphertext"} {
		if strings.Contains(strings.ToLower(output), forbidden) {
			t.Fatalf("safe cutover output leaked %q: %s", forbidden, output)
		}
	}
	for _, required := range []string{"missing_dashboard_identity_ids=[10]", "missing_corp_credential_copy_ids=[20]", "missing_agent_credential_copy_ids=[]"} {
		if !strings.Contains(output, required) {
			t.Fatalf("safe cutover output missing %q: %s", required, output)
		}
	}
}

func TestMappingRequiresSignatureForMultipleCorpTenant(t *testing.T) {
	if _, err := ParseMappingDocument([]byte(`{"version":1,"entries":[{"tenantId":42,"corpId":9001}]}`), []byte("mapping-signing-key")); err == nil || !strings.Contains(err.Error(), "signature") {
		t.Fatalf("unsigned mapping error=%v, want signature error", err)
	}
}

func TestValidateMappingOwnershipRequiresTheExactTenantCorpPair(t *testing.T) {
	document := MappingDocument{
		Version:           1,
		Entries:           []CorpMapping{{TenantID: 42, CorpID: 9001}},
		SignatureVerified: true,
	}
	if err := ValidateMappingOwnership(document, map[int64][]int64{42: {9001, 9002}}); err != nil {
		t.Fatalf("valid multi-corp mapping rejected: %v", err)
	}
	if err := ValidateMappingOwnership(document, map[int64][]int64{42: {9001}}); err == nil || !strings.Contains(err.Error(), "one-corp") {
		t.Fatalf("one-corp mapping error=%v, want explicit rejection", err)
	}
}

func TestMaintenanceConfirmationBindsRequestAndSchema(t *testing.T) {
	confirmation := []byte("MOCHAT_IDENTITY_MAINTENANCE_V1\nschema=business_schema\nrequest_id=task8-1\n")
	if err := VerifyMaintenanceConfirmation(confirmation, "business_schema", "task8-1"); err != nil {
		t.Fatalf("valid maintenance confirmation rejected: %v", err)
	}
	if err := VerifyMaintenanceConfirmation(confirmation, "other_schema", "task8-1"); err == nil {
		t.Fatal("maintenance confirmation was reused for another schema")
	}
}

func TestActorSchemaInventoryRejectsUnknownActorColumn(t *testing.T) {
	known := []ActorColumn{
		{Table: "mochat_go_saas_admin_user_access", Column: "user_id"},
		{Table: "mochat_go_saas_admin_approvals", Column: "requester_user_id"},
		{Table: "mochat_go_saas_admin_audit_anchor_checkpoints", Column: "actor_user_id"},
		{Table: "mochat_go_saas_admin_audit_verifications", Column: "actor_user_id"},
		{Table: "mochat_go_saas_admin_health_scans", Column: "actor_user_id"},
		{Table: "mochat_go_saas_admin_tasks", Column: "actor_user_id"},
	}
	if err := ValidateActorSchemaInventory(known); err != nil {
		t.Fatalf("known actor inventory rejected: %v", err)
	}
	if err := ValidateActorSchemaInventory(append(known, ActorColumn{Table: "mochat_go_saas_admin_approvals", Column: "new_actor_user_id"})); err == nil || !strings.Contains(err.Error(), "unknown actor column") {
		t.Fatalf("unknown actor column error=%v", err)
	}
	if err := ValidateActorSchemaInventory([]ActorColumn{{Table: "mochat_go_dashboard_identity_activations", Column: "user_id", Kind: ActorColumnKindNonActorTarget}, {Table: "mochat_go_dashboard_identity_activations", Column: "created_by_saas_user_id", Kind: ActorColumnKindActor}}); err != nil {
		t.Fatalf("activation target/creator classification rejected: %v", err)
	}
	if SystemActorZeroAllowed(ActorColumn{Table: "mochat_go_saas_admin_roles", Column: "created_by"}) == false || SystemActorZeroAllowed(ActorColumn{Table: "mochat_go_dashboard_identity_activations", Column: "created_by_saas_user_id"}) {
		t.Fatal("system actor zero exception classification is not explicit")
	}
}

func TestNormalizePhoneRequiresCanonicalStoredForm(t *testing.T) {
	for _, value := range []string{"", "+8613800000000", "138 0000 0000", "1380000000x"} {
		if _, err := NormalizePhone(value); err == nil {
			t.Fatalf("non-canonical contact %q unexpectedly accepted", value)
		}
	}
	if got, err := NormalizePhone(" 13800000000 "); err != nil || got != "13800000000" {
		t.Fatalf("canonical contact normalization got=%q err=%v", got, err)
	}
}

func TestPreflightRejectsPlatformPhoneConflictAndUnknownInventory(t *testing.T) {
	report := PreflightReport{
		DuplicatePlatformPhoneUserIDs: []int64{11, 12},
		UnknownActorColumns:           []ActorColumn{{Table: "mochat_go_saas_admin_approvals", Column: "new_actor_user_id"}},
	}
	if err := report.Validate(Options{PlatformTenantID: 1}); err == nil || !strings.Contains(err.Error(), "phone/login") {
		t.Fatalf("platform conflict error=%v", err)
	}
	report = PreflightReport{UnknownActorColumns: []ActorColumn{{Table: "unknown", Column: "actor_user_id"}}}
	if err := report.Validate(Options{PlatformTenantID: 1}); err == nil || !strings.Contains(err.Error(), "unknown actor") {
		t.Fatalf("inventory error=%v", err)
	}
}

func TestSaaSAdminUsersSchemaContractDistinguishesLegacyAbsenceFromBrokenSchema(t *testing.T) {
	if err := validateSaaSAdminUsersColumns([]string{
		"id", "login_name", "phone", "password_hash", "name", "status",
		"must_rotate_password", "auth_version", "mfa_required", "bootstrap_request_key",
		"created_at", "updated_at",
	}); err != nil {
		t.Fatalf("complete 0129 SaaS identity schema rejected: %v", err)
	}
	if err := validateSaaSAdminUsersColumns([]string{"id", "login_name", "password_hash", "name", "status"}); err == nil || !strings.Contains(err.Error(), "phone") {
		t.Fatalf("broken SaaS identity schema error=%v, want missing phone contract", err)
	}
}

func TestBuildActorReferenceQueryUsesLegacyIdentityWhenSaaSIdentityTableIsAbsent(t *testing.T) {
	query, args, err := buildActorReferenceQuery([]ActorColumn{{Table: "mochat_go_saas_admin_user_access", Column: "user_id"}}, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(query, "mochat_go_saas_admin_users") || len(args) != 1 || args[0] != int64(1) {
		t.Fatalf("legacy actor query=%q args=%v, must not reference missing 0129 table", query, args)
	}
}

func TestRestoreLegacyCredentialsIsScopedToTheCutoverJournal(t *testing.T) {
	root := filepath.Join("..", "..")
	body, err := os.ReadFile(filepath.Join(root, "internal", "identitymigration", "cutover.go"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, required := range []string{
		"INNER JOIN mochat_go_identity_cutover_journal j",
		"j.request_id=?",
		"j.entity_type='corp_credentials'",
		"j.entity_type='agent_credentials'",
		"legacy credential restore journal entity is missing",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("restore implementation missing journal scope contract %q", required)
		}
	}
	if strings.Contains(text, "DELETE FROM mochat_go_identity_cutover_journal") {
		t.Fatal("restore must retain the cutover journal as immutable scope evidence")
	}
}
