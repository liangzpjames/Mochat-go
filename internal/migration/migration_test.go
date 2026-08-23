package migration

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHistorical0099ChecksumRemainsStable(t *testing.T) {
	root := filepath.Join("..", "..")
	for _, tc := range []struct {
		name string
		want string
	}{
		{name: "up", want: "c409fc5562fa2336f9f559efc628b10ff42b0e2f2ebab6af30bf2607f38fd099"},
		{name: "down", want: "de7efc7f22eb8b834d1b046916138b38f294023e4cdec9b828b05b6ae56f0e5a"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(root, "deploy", "standalone", "migrations", "0099_saas_tenant_default_corp."+tc.name+".sql")
			body, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			sum := sha256.Sum256(body)
			if got := hex.EncodeToString(sum[:]); got != tc.want {
				t.Fatalf("0099 %s checksum = %s, want %s", tc.name, got, tc.want)
			}
		})
	}
}

func TestDefaultMigrations(t *testing.T) {
	migrations := DefaultMigrations("/project")
	if len(migrations) != 1 {
		t.Fatalf("migrations = %#v", migrations)
	}
	if migrations[0].Version != "0001_initial_schema" {
		t.Fatalf("version = %q", migrations[0].Version)
	}
	want := filepath.Join("/project", "deploy", "standalone", "schema", "mochat.sql")
	if migrations[0].Path != want {
		t.Fatalf("path = %q, want %q", migrations[0].Path, want)
	}
}

func TestDefaultMigrationsAcceptsKnownHistoricalInitialChecksum(t *testing.T) {
	root := t.TempDir()
	schemaPath := filepath.Join(root, "deploy", "standalone", "schema")
	seedPath := filepath.Join(root, "deploy", "standalone", "migrations")
	if err := os.MkdirAll(schemaPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(seedPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(schemaPath, "mochat.sql"), []byte("CREATE TABLE mc_user (id int);\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(seedPath, "0002_seed_core_data.up.sql"), []byte("-- ----------------------------\n-- seed\nINSERT IGNORE INTO mc_user (id) VALUES (1);\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	const historicalChecksum = "b7dbd66b24b93a4be64e33fa51d2e1a1fcbc0d305532145644c37ed1a26075e9"
	initial := DefaultMigrations(root)[0]
	if !checksumMatches(historicalChecksum, "current-checksum", initial.ChecksumAliases) {
		t.Fatalf("known historical checksum %s not accepted by aliases %#v", historicalChecksum, initial.ChecksumAliases)
	}
}

func TestDefaultMigrationsAcceptsCRLFChecksumForIncrementalSQL(t *testing.T) {
	root := t.TempDir()
	schemaPath := filepath.Join(root, "deploy", "standalone", "schema")
	seedPath := filepath.Join(root, "deploy", "standalone", "migrations")
	if err := os.MkdirAll(schemaPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(seedPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(schemaPath, "mochat.sql"), []byte("CREATE TABLE mc_user (id int);\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	seed := "-- MoChat standalone core seed data.\nINSERT IGNORE INTO mc_user (id) VALUES (1);\n"
	seedFile := filepath.Join(seedPath, "0002_seed_core_data.up.sql")
	if err := os.WriteFile(seedFile, []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(seedPath, "0002_seed_core_data.down.sql"), []byte("DELETE FROM mc_user;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	other := "CREATE TABLE phase34_line_endings (id int);\n"
	if err := os.WriteFile(filepath.Join(seedPath, "0003_line_endings.up.sql"), []byte(strings.ReplaceAll(other, "\n", "\r\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(seedPath, "0003_line_endings.down.sql"), []byte("DROP TABLE phase34_line_endings;\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	crlfChecksum := checksumBytes([]byte(strings.ReplaceAll(seed, "\n", "\r\n")))
	seedMigration := DefaultMigrations(root)[1]
	if !checksumMatches(crlfChecksum, checksumBytes([]byte(seed)), seedMigration.ChecksumAliases) {
		t.Fatalf("CRLF checksum %s not accepted by aliases %#v", crlfChecksum, seedMigration.ChecksumAliases)
	}
	if !checksumMatches(knownLegacyCoreSeedChecksum, checksumBytes([]byte(seed)), seedMigration.ChecksumAliases) {
		t.Fatalf("known historical seed checksum %s not accepted by aliases %#v", knownLegacyCoreSeedChecksum, seedMigration.ChecksumAliases)
	}
	lfChecksum := checksumBytes([]byte(other))
	lineEndingMigration := DefaultMigrations(root)[2]
	if !checksumMatches(lfChecksum, checksumBytes([]byte(strings.ReplaceAll(other, "\n", "\r\n"))), lineEndingMigration.ChecksumAliases) {
		t.Fatalf("LF checksum %s not accepted by aliases %#v", lfChecksum, lineEndingMigration.ChecksumAliases)
	}
}

func TestDefaultMigrationsAcceptsHistoricalMixedLineEndingChecksumForLiveCodeWorkspace(t *testing.T) {
	root := t.TempDir()
	schemaPath := filepath.Join(root, "deploy", "standalone", "schema")
	migrationPath := filepath.Join(root, "deploy", "standalone", "migrations")
	if err := os.MkdirAll(schemaPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(migrationPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(schemaPath, "mochat.sql"), []byte("CREATE TABLE mc_user (id int);\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	current := []byte("CREATE TABLE mochat_go_live_code_workspaces (id bigint);\n")
	if err := os.WriteFile(filepath.Join(migrationPath, "0153_live_code_workspace.up.sql"), current, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(migrationPath, "0153_live_code_workspace.down.sql"), []byte("DROP TABLE mochat_go_live_code_workspaces;\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	const historicalChecksum = "f17df230c78b79ed0e23d77b87057a939fa8ef5d1ac97fa1db43b5aa34f7344c"
	migrations := DefaultMigrations(root)
	var liveCode Migration
	for _, item := range migrations {
		if item.Version == "0153_live_code_workspace" {
			liveCode = item
			break
		}
	}
	if liveCode.Version == "" {
		t.Fatalf("0153 migration missing from %#v", migrations)
	}
	if !checksumMatches(historicalChecksum, checksumBytes(current), liveCode.ChecksumAliases) {
		t.Fatalf("historical mixed-line-ending checksum %s not accepted by aliases %#v", historicalChecksum, liveCode.ChecksumAliases)
	}
}

func TestNewRunnerValidatesMigrations(t *testing.T) {
	_, err := NewRunner(nil, []Migration{{Version: "0001", Path: "one.sql"}})
	if err == nil || !strings.Contains(err.Error(), "db is nil") {
		t.Fatalf("nil db err = %v", err)
	}

	err = validateMigrations([]Migration{{Version: "0001", Path: "one.sql"}, {Version: "0001", Path: "two.sql"}})
	if err == nil || !strings.Contains(err.Error(), "duplicate migration version") {
		t.Fatalf("duplicate err = %v", err)
	}

	err = validateMigrations([]Migration{{Version: "0001"}})
	if err == nil || !strings.Contains(err.Error(), "path is required") {
		t.Fatalf("path err = %v", err)
	}

	err = validateMigrations([]Migration{{Version: "0002", Path: "up.sql", DownPath: filepath.Join(t.TempDir(), "missing.down.sql")}})
	if err == nil || !strings.Contains(err.Error(), "down path is not readable") {
		t.Fatalf("down path err = %v", err)
	}
}

func TestDefaultMigrationsDiscoversIncrementalFiles(t *testing.T) {
	root := t.TempDir()
	migrationDir := filepath.Join(root, "deploy", "standalone", "migrations")
	if err := os.MkdirAll(migrationDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(migrationDir, "0002_add_queue_indexes.up.sql"), []byte("CREATE TABLE smoke (id int);"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(migrationDir, "0002_add_queue_indexes.down.sql"), []byte("DROP TABLE smoke;"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(migrationDir, "README.md"), []byte("ignored"), 0o644); err != nil {
		t.Fatal(err)
	}

	migrations := DefaultMigrations(root)
	if len(migrations) != 2 {
		t.Fatalf("migrations = %#v", migrations)
	}
	incremental := migrations[1]
	if incremental.Version != "0002_add_queue_indexes" || incremental.Description != "add queue indexes" {
		t.Fatalf("incremental migration = %#v", incremental)
	}
	if !strings.HasSuffix(incremental.Path, "0002_add_queue_indexes.up.sql") || !strings.HasSuffix(incremental.DownPath, "0002_add_queue_indexes.down.sql") {
		t.Fatalf("paths = up:%q down:%q", incremental.Path, incremental.DownPath)
	}
}

func TestDefaultMigrationsRegistersControlledIdentityBackfill(t *testing.T) {
	migrations := DefaultMigrations(filepath.Join("..", ".."))
	for _, migration := range migrations {
		if migration.Version != "0130_identity_realms_single_corp_backfill" {
			continue
		}
		if migration.Kind != MigrationControlled || migration.Controlled == nil {
			t.Fatalf("0130 migration metadata = %#v, want controlled registry entry", migration)
		}
		if migration.Controlled.RequiredCLI != "mochat-identity-migrate" {
			t.Fatalf("0130 required CLI = %q", migration.Controlled.RequiredCLI)
		}
		return
	}
	t.Fatal("controlled 0130 identity backfill must be registered in DefaultMigrations")
}

func TestBaselineRejectsComposeLikeSchemaBeforeRecordingMissing0129Tables(t *testing.T) {
	present := []string{"mc_user", "mc_rbac_menu"}
	err := validateBaselineSchemaTables(present)
	if err == nil {
		t.Fatal("baseline accepted a compose-like 0104 schema without the 0129 identity tables")
	}
	for _, required := range []string{
		"mochat_go_saas_admin_users",
		"mochat_go_dashboard_identities",
		"mochat_go_dashboard_permission_audits",
	} {
		if !strings.Contains(err.Error(), required) {
			t.Fatalf("baseline compatibility error=%q, missing required table %q", err, required)
		}
	}
}

func TestComposeInitBaselineFactsRequireFreshComplete0104Schema(t *testing.T) {
	complete := composeInitBaselineFacts{
		TenantCount:             0,
		CorpCount:               0,
		UserCount:               0,
		MigrationLedgerCount:    0,
		IdentityTableCount:      0,
		HasOpportunityOwner:     true,
		HasPhase32CorpDateIndex: true,
		HasCrossStageTables:     true,
	}
	if err := validateComposeInitBaselineFacts(complete); err != nil {
		t.Fatalf("complete compose-init facts rejected: %v", err)
	}

	tests := []struct {
		name    string
		facts   composeInitBaselineFacts
		wantErr string
	}{
		{
			name:    "populated legacy",
			facts:   completeWith(func(f *composeInitBaselineFacts) { f.CorpCount = 1 }),
			wantErr: "empty business schema",
		},
		{
			name:    "missing 0104 sentinel",
			facts:   completeWith(func(f *composeInitBaselineFacts) { f.HasOpportunityOwner = false }),
			wantErr: "0104 schema sentinels",
		},
		{
			name:    "missing phase 3.2 index",
			facts:   completeWith(func(f *composeInitBaselineFacts) { f.HasPhase32CorpDateIndex = false }),
			wantErr: "0104 schema sentinels",
		},
		{
			name:    "identity table already exists",
			facts:   completeWith(func(f *composeInitBaselineFacts) { f.IdentityTableCount = 1 }),
			wantErr: "0129 identity tables",
		},
		{
			name:    "ledger already exists",
			facts:   completeWith(func(f *composeInitBaselineFacts) { f.MigrationLedgerCount = 1 }),
			wantErr: "empty migration ledger",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateComposeInitBaselineFacts(test.facts)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("facts error = %v, want text containing %q", err, test.wantErr)
			}
		})
	}
}

func completeWith(change func(*composeInitBaselineFacts)) composeInitBaselineFacts {
	facts := composeInitBaselineFacts{
		TenantCount:             0,
		CorpCount:               0,
		UserCount:               0,
		MigrationLedgerCount:    0,
		IdentityTableCount:      0,
		HasOpportunityOwner:     true,
		HasPhase32CorpDateIndex: true,
		HasCrossStageTables:     true,
	}
	change(&facts)
	return facts
}

func TestComposeInitBaselineCutoffPrecedes0129(t *testing.T) {
	if composeInitBaselineVersion != "0104_scrm_opportunity_owner" {
		t.Fatalf("compose init baseline cutoff=%q", composeInitBaselineVersion)
	}
	migrations := DefaultMigrations(filepath.Join("..", ".."))
	seenCutoff := false
	seen0129 := false
	for _, migration := range migrations {
		if migration.Version == composeInitBaselineVersion {
			seenCutoff = true
		}
		if migration.Version == "0129_identity_realms_single_corp_schema" {
			seen0129 = true
			if !seenCutoff {
				t.Fatal("0129 identity schema precedes the compose init baseline cutoff")
			}
		}
	}
	if !seenCutoff || !seen0129 {
		t.Fatalf("migration registry missing compose cutoff or 0129: cutoff=%t 0129=%t", seenCutoff, seen0129)
	}
}

func TestControlledMigrationPendingErrorIsStable(t *testing.T) {
	err := ControlledMigrationBlocked("0130_identity_realms_single_corp_backfill")
	if err == nil || err.Error() != "controlled migration 0130_identity_realms_single_corp_backfill is pending; run mochat-identity-migrate before automatic migrations can continue" {
		t.Fatalf("controlled migration error = %v", err)
	}
}

func TestStandaloneComposeFreshInitUsesSchemaForCorpDataIndexes(t *testing.T) {
	projectRoot := filepath.Join("..", "..")
	migrations := DefaultMigrations(projectRoot)
	latest := migrations[len(migrations)-1]
	composePath := filepath.Join(projectRoot, "deploy", "standalone", "docker-compose.yml")
	composeBody, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatal(err)
	}

	if latest.Version != "0157_ai_settings_document_count_reconcile" {
		t.Fatalf("latest migration = %q, want 0157_ai_settings_document_count_reconcile", latest.Version)
	}
	if mount := "./migrations/0105_corp_data_realtime_indexes.up.sql:"; strings.Contains(string(composeBody), mount) {
		t.Fatalf("standalone fresh init must use the synchronized base schema instead of replaying %q", mount)
	}
}

func TestConversationWorkspaceRBACMigrationContract(t *testing.T) {
	root := filepath.Join("..", "..")
	migrations := DefaultMigrations(root)
	var target Migration
	for _, migration := range migrations {
		if migration.Version == "0141_conversation_workspace_rbac" {
			target = migration
			break
		}
	}
	if target.Version == "" {
		t.Fatal("0141 conversation workspace RBAC migration was not discovered")
	}
	upBody, err := os.ReadFile(target.Path)
	if err != nil {
		t.Fatal(err)
	}
	downBody, err := os.ReadFile(target.DownPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"dashboard.chat.v2_all",
		"/dashboard/workMessage/globalOverview",
		"dashboard.chat.v2_staff",
		"/dashboard/workMessage/staffDirectory",
		"/dashboard/workMessage/staffDetail",
		"/dashboard/workMessage/focus",
	} {
		if !strings.Contains(string(upBody), required) {
			t.Fatalf("0141 up migration missing %q", required)
		}
	}
	if !strings.Contains(string(downBody), "DELETE resource") {
		t.Fatal("0141 down migration must remove only the workspace resources")
	}
}

func TestCustomerConversationWorkspaceRBAC0142MigrationContract(t *testing.T) {
	root := filepath.Join("..", "..")
	migrations := DefaultMigrations(root)
	var target Migration
	for _, migration := range migrations {
		if migration.Version == "0142_customer_conversation_workspace_rbac" {
			target = migration
			break
		}
	}
	if target.Version == "" {
		t.Fatal("0142 customer conversation workspace RBAC migration was not discovered")
	}
	upBody, err := os.ReadFile(target.Path)
	if err != nil {
		t.Fatal(err)
	}
	downBody, err := os.ReadFile(target.DownPath)
	if err != nil {
		t.Fatal(err)
	}
	up := string(upBody)
	down := string(downBody)
	for _, required := range []string{
		"dashboard.chat.v2_customer",
		"/dashboard/workMessage/customerDirectory",
		"/dashboard/workMessage/customerConversations",
		"/dashboard/workMessage/customerDetail",
		"/dashboard/workMessage/focus",
		"WHERE NOT EXISTS",
	} {
		if !strings.Contains(up, required) {
			t.Fatalf("0142 up migration missing %q", required)
		}
	}
	for _, path := range []string{
		"/dashboard/workMessage/customerDirectory",
		"/dashboard/workMessage/customerConversations",
		"/dashboard/workMessage/customerDetail",
		"/dashboard/workMessage/focus",
	} {
		if !strings.Contains(down, path) {
			t.Fatalf("0142 down migration missing %q", path)
		}
	}
	if strings.Count(down, "/dashboard/workMessage/focus") != 1 {
		t.Fatalf("0142 down migration must target the focus resource exactly once, got %d", strings.Count(down, "/dashboard/workMessage/focus"))
	}
}

func TestAIConversationInsights0148MigrationContract(t *testing.T) {
	root := filepath.Join("..", "..")
	var target Migration
	for _, migration := range DefaultMigrations(root) {
		if migration.Version == "0148_ai_conversation_insights" {
			target = migration
			break
		}
	}
	if target.Version == "" {
		t.Fatal("0148 AI conversation insights migration was not discovered")
	}
	upBody, err := os.ReadFile(target.Path)
	if err != nil {
		t.Fatal(err)
	}
	downBody, err := os.ReadFile(target.DownPath)
	if err != nil {
		t.Fatal(err)
	}
	up := string(upBody)
	down := string(downBody)
	for _, required := range []string{
		"mochat_go_ai_analysis_rules",
		"mochat_go_ai_analysis_rule_versions",
		"mochat_go_ai_conversation_insights",
		"mochat_go_ai_insight_runs",
		"uq_ai_conversation_source",
		"idx_ai_insight_scope_page",
		"result_json",
		"source_fingerprint",
	} {
		if !strings.Contains(up, required) {
			t.Fatalf("0148 up migration missing %q", required)
		}
	}
	for _, required := range []string{
		"DROP TABLE IF EXISTS `mochat_go_ai_insight_runs`",
		"DROP TABLE IF EXISTS `mochat_go_ai_conversation_insights`",
		"DROP TABLE IF EXISTS `mochat_go_ai_analysis_rule_versions`",
		"DROP TABLE IF EXISTS `mochat_go_ai_analysis_rules`",
	} {
		if !strings.Contains(down, required) {
			t.Fatalf("0148 down migration missing %q", required)
		}
	}
}

func TestGlobalMessageFocusMigrationContract(t *testing.T) {
	root := filepath.Join("..", "..")
	migrations := DefaultMigrations(root)
	var target Migration
	for _, migration := range migrations {
		if migration.Version == "0140_global_message_focus_and_indexes" {
			target = migration
			break
		}
	}
	if target.Version == "" {
		t.Fatal("0140 global message migration was not discovered")
	}
	upBody, err := os.ReadFile(target.Path)
	if err != nil {
		t.Fatal(err)
	}
	downBody, err := os.ReadFile(target.DownPath)
	if err != nil {
		t.Fatal(err)
	}
	up := string(upBody)
	down := string(downBody)
	for _, required := range []string{
		"mochat_go_work_message_focus",
		"uk_work_message_focus_subject",
		"idx_mc_work_message_1_global",
		"idx_mc_work_message_10_global",
	} {
		if !strings.Contains(up, required) {
			t.Fatalf("0140 up migration missing %q", required)
		}
	}
	if !strings.Contains(down, "DROP TABLE IF EXISTS `mochat_go_work_message_focus`") {
		t.Fatal("0140 down migration must drop the focus table")
	}
	if strings.Contains(strings.ToUpper(down), "DROP TABLE `MC_WORK_MESSAGE_") {
		t.Fatal("0140 down migration must preserve message partitions")
	}
}

func TestDefaultCorpReconciliationIsTenantScopedAndIdempotent(t *testing.T) {
	root := filepath.Join("..", "..")
	body, err := os.ReadFile(filepath.Join(root, "deploy", "standalone", "migrations", "0120_saas_tenant_default_corp_reconcile.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	sql := string(body)
	for _, required := range []string{
		"NOT EXISTS",
		"c.`tenant_id` = t.`id`",
		"e.`corp_id` = c.`id`",
		"e.`log_user_id` = u.`id`",
		"u.`isSuperAdmin` = 1",
		"SELECT MIN(c2.`id`)",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("reconciliation migration missing %q", required)
		}
	}
	rollback, err := os.ReadFile(filepath.Join(root, "deploy", "standalone", "migrations", "0120_saas_tenant_default_corp_reconcile.down.sql"))
	if err != nil {
		t.Fatal(err)
	}
	rollbackSQL := strings.ToUpper(string(rollback))
	if strings.Contains(rollbackSQL, "DELETE FROM") || strings.Contains(rollbackSQL, "DROP TABLE") {
		t.Fatal("reconciliation rollback must be non-destructive")
	}
}

func TestRetainedLedger0099Through0119Gets0120AsNextMigration(t *testing.T) {
	migrations := DefaultMigrations(filepath.Join("..", ".."))
	index := make(map[string]int, len(migrations))
	for i, migration := range migrations {
		index[migration.Version] = i
	}
	if _, ok := index["0099_saas_tenant_default_corp"]; !ok {
		t.Fatal("0099 migration was not discovered")
	}
	for _, version := range []string{"0110_risk_behavior_provider", "0111_timeout_warning_provider", "0119_phase35_orders_settings"} {
		if _, ok := index[version]; !ok {
			t.Fatalf("retained-ledger migration %s was not discovered", version)
		}
	}
	if index["0120_saas_tenant_default_corp_reconcile"] <= index["0119_phase35_orders_settings"] {
		t.Fatal("0120 reconciliation must run after the retained 0099-0119 ledger")
	}
}

func TestPhase35OrderProductizationMigrationIsForwardOnly(t *testing.T) {
	root := filepath.Join("..", "..")
	migrations := DefaultMigrations(root)
	latest := migrations[len(migrations)-1]
	if latest.Version != "0157_ai_settings_document_count_reconcile" {
		t.Fatalf("latest migration = %q, want 0157_ai_settings_document_count_reconcile", latest.Version)
	}
	up, err := os.ReadFile(filepath.Join(root, "deploy", "standalone", "migrations", "0121_phase35_order_productization.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"ADD COLUMN title", "ADD COLUMN note"} {
		if !strings.Contains(string(up), required) {
			t.Fatalf("migration missing %q", required)
		}
	}
}

func TestDashboardPageRBACMigrationContract(t *testing.T) {
	root := filepath.Join("..", "..")
	migrations := DefaultMigrations(root)
	var target Migration
	for _, migration := range migrations {
		if migration.Version == "0127_dashboard_page_rbac" {
			target = migration
			break
		}
	}
	if target.Version == "" {
		t.Fatal("0127_dashboard_page_rbac migration not found")
	}
	upBody, err := os.ReadFile(target.Path)
	if err != nil {
		t.Fatal(err)
	}
	downBody, err := os.ReadFile(target.DownPath)
	if err != nil {
		t.Fatal(err)
	}
	up := string(upBody)
	for _, required := range []string{
		"mochat_go_dashboard_permissions",
		"mochat_go_dashboard_permission_resources",
		"mochat_go_dashboard_user_roles",
		"mochat_go_dashboard_role_permissions",
		"mochat_go_dashboard_user_permissions",
		"mochat_go_dashboard_permission_audits",
		"dashboard_access_version",
		"SIGNAL SQLSTATE ''45000''",
		"missing subscription for active tenant package",
		"cross-tenant legacy user-role relationship",
		"dangling legacy user-role relationship",
		"PREPARE dashboard_subscription_guard_stmt",
		"EXECUTE dashboard_subscription_guard_stmt",
		"PREPARE dashboard_legacy_role_guard_stmt",
		"EXECUTE dashboard_legacy_role_guard_stmt",
		"LEFT JOIN `mc_user` u ON u.`id` = CAST(ur.`user_id` AS UNSIGNED)",
		"LEFT JOIN `mc_rbac_role` r ON r.`id` = ur.`role_id`",
		"`tenant_id` int(11) NOT NULL",
		"`user_id` int(10) unsigned NOT NULL",
		"`role_id` int(11) NOT NULL",
		"`permission_id` bigint(20) unsigned NOT NULL",
		"`actor_user_id` int(10) unsigned NULL",
		"`target_id` varchar(64) NOT NULL",
		"CONSTRAINT `fk_dashboard_audit_actor` FOREIGN KEY (`tenant_id`, `actor_user_id`)",
		"UNIQUE KEY `uni_dashboard_user_roles` (`tenant_id`, `user_id`, `role_id`)",
		"UNIQUE KEY `uni_dashboard_role_permissions` (`tenant_id`, `role_id`, `permission_id`)",
		"UNIQUE KEY `uni_dashboard_user_permissions` (`tenant_id`, `user_id`, `permission_id`)",
		"'/dashboard/channelCode/index'",
		"'/dashboard/workContact/index'",
		"SUBSTRING_INDEX(SUBSTRING_INDEX(m.`link_url`, '@', 1), '#', 1)",
		"INNER JOIN `mochat_go_dashboard_permission_resources` pr",
		"p.`superadmin_only` = 0",
	} {
		if !strings.Contains(up, required) {
			t.Fatalf("0127 up migration missing %q", required)
		}
	}
	if got := strings.Count(up, "'page', '/"); got != 53 {
		t.Fatalf("page seed count = %d, want 53", got)
	}
	if got := strings.Count(up, "'superadmin_only', 1"); got != 4 {
		t.Fatalf("superadmin_only seed count = %d, want 4", got)
	}
	for _, forbidden := range []string{
		"DELIMITER",
		"CREATE PROCEDURE",
		"ADD COLUMN IF NOT EXISTS",
		"UNIQUE KEY `uni_dashboard_user_roles` (`tenant_id`, `user_id`, `role_id`, `deleted_at`)",
		"UNIQUE KEY `uni_dashboard_role_permissions` (`tenant_id`, `role_id`, `permission_id`, `deleted_at`)",
		"UNIQUE KEY `uni_dashboard_user_permissions` (`tenant_id`, `user_id`, `permission_id`, `deleted_at`)",
	} {
		if strings.Contains(up, forbidden) {
			t.Fatalf("0127 up migration contains unreliable nullable uniqueness %q", forbidden)
		}
	}
	firstDDL := strings.Index(up, "ALTER TABLE")
	if firstDDL < 0 {
		t.Fatal("0127 up migration has no DDL")
	}
	for _, preflight := range []string{"missing subscription for active tenant package", "cross-tenant legacy user-role relationship"} {
		if offset := strings.Index(up, preflight); offset < 0 || offset > firstDDL {
			t.Fatalf("0127 preflight %q must run before first DDL", preflight)
		}
	}
	if strings.Index(up, "dangling legacy user-role relationship") > firstDDL {
		t.Fatal("0127 dangling legacy relation preflight must run before first DDL")
	}
	down := string(downBody)
	for _, required := range []string{
		"DROP TABLE IF EXISTS `mochat_go_dashboard_permission_audits`",
		"DROP TABLE IF EXISTS `mochat_go_dashboard_permissions`",
		"DROP COLUMN `dashboard_access_version`",
		"DROP INDEX `uni_dashboard_user_tenant_id_id`",
		"DROP INDEX `uni_dashboard_role_tenant_id_id`",
		"PREPARE dashboard_down_user_column_stmt",
		"PREPARE dashboard_down_user_index_stmt",
		"PREPARE dashboard_down_role_column_stmt",
		"PREPARE dashboard_down_role_index_stmt",
		"information_schema.columns",
		"information_schema.statistics",
	} {
		if !strings.Contains(down, required) {
			t.Fatalf("0127 down migration missing %q", required)
		}
	}
}

func TestWorkMessageExportTaskMigrationContract(t *testing.T) {
	root := filepath.Join("..", "..")
	migrations := DefaultMigrations(root)
	var target Migration
	for _, migration := range migrations {
		if migration.Version == "0144_work_message_export_tasks" {
			target = migration
			break
		}
	}
	if target.Version == "" {
		t.Fatal("0144 work message export migration not found")
	}
	upBody, err := os.ReadFile(target.Path)
	if err != nil {
		t.Fatal(err)
	}
	downBody, err := os.ReadFile(target.DownPath)
	if err != nil {
		t.Fatal(err)
	}
	up := string(upBody)
	for _, required := range []string{
		"mochat_go_work_message_export_tasks",
		"uk_mg_wmet_idempotency",
		"idx_mg_wmet_claim",
		"idx_mg_wmet_owner",
		"idx_mg_wmet_expiry",
		"selected_objects_json",
		"conversation_scopes_json",
		"employee_scope_json",
		"artifact_sha256",
	} {
		if !strings.Contains(up, required) {
			t.Fatalf("0144 up migration missing %q", required)
		}
	}
	for _, required := range []string{
		"mochat_go_work_message_export_tasks",
		"/dashboard/workMessage/exportCandidates",
		"/dashboard/workMessage/exportTasks",
		"/dashboard/workMessage/exportDownload",
	} {
		if !strings.Contains(string(downBody), required) {
			t.Fatalf("0144 down migration missing %q", required)
		}
	}
}

func TestDashboardPageRBACLegacyScopeFixMigrationContract(t *testing.T) {
	root := filepath.Join("..", "..")
	migrations := DefaultMigrations(root)
	var target Migration
	for _, migration := range migrations {
		if migration.Version == "0128_dashboard_page_rbac_legacy_scope_fix" {
			target = migration
			break
		}
	}
	if target.Version == "" {
		t.Fatal("0128_dashboard_page_rbac_legacy_scope_fix migration not found")
	}
	upBody, err := os.ReadFile(target.Path)
	if err != nil {
		t.Fatal(err)
	}
	downBody, err := os.ReadFile(target.DownPath)
	if err != nil {
		t.Fatal(err)
	}
	up := string(upBody)
	for _, required := range []string{
		"permissionType",
		"permission_type",
		"'tenant'",
		"'department'",
		"'self'",
		"migration.legacy_scope_review",
		"requiresAdminReview",
	} {
		if !strings.Contains(up, required) {
			t.Fatalf("0128 up migration missing %q", required)
		}
	}
	if strings.Contains(strings.ToUpper(string(downBody)), "UPDATE `MOCHAT_GO_DASHBOARD_ROLE_PERMISSIONS`") {
		t.Fatal("0128 down must not guess pre-correction role scopes")
	}
}

func TestStandaloneComposeEnablesSCRMRoutesByDefault(t *testing.T) {
	composePath := filepath.Join("..", "..", "deploy", "standalone", "docker-compose.yml")
	composeBody, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatal(err)
	}

	want := `MOCHAT_GO_ENABLE_PHASE2_2_SCRM_PILOT: "${MOCHAT_GO_ENABLE_PHASE2_2_SCRM_PILOT:-1}"`
	if !strings.Contains(string(composeBody), want) {
		t.Fatalf("standalone compose must enable SCRM routes by default; missing %q", want)
	}
}

func TestStandaloneComposeEnablesSaaSAdminByDefault(t *testing.T) {
	composePath := filepath.Join("..", "..", "deploy", "standalone", "docker-compose.yml")
	composeBody, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatal(err)
	}

	want := `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD: "${MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD:-1}"`
	if !strings.Contains(string(composeBody), want) {
		t.Fatalf("standalone compose must enable SaaS Admin by default; missing %q", want)
	}
}

func TestCustomerTagParityMigrationIsReversible(t *testing.T) {
	projectRoot := filepath.Join("..", "..")
	up, err := os.ReadFile(filepath.Join(projectRoot, "deploy", "standalone", "migrations", "0109_scrm_customer_tag_parity.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	down, err := os.ReadFile(filepath.Join(projectRoot, "deploy", "standalone", "migrations", "0109_scrm_customer_tag_parity.down.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{"mochat_go_scrm_tag_groups", "group_id", "idx_scrm_tag_catalog", "idx_scrm_contact_tag_usage", "默认分组"} {
		if !strings.Contains(string(up), fragment) {
			t.Errorf("up migration missing %q", fragment)
		}
	}
	for _, fragment := range []string{"DROP TABLE IF EXISTS `mochat_go_scrm_tag_groups`", "DROP COLUMN IF EXISTS `group_id`", "DROP INDEX IF EXISTS `idx_scrm_contact_tag_usage`"} {
		if !strings.Contains(string(down), fragment) {
			t.Errorf("down migration missing %q", fragment)
		}
	}
}

func TestPublicPoolParityMigrationIsReversibleAndSynced(t *testing.T) {
	projectRoot := filepath.Join("..", "..")
	read := func(path ...string) string {
		t.Helper()
		body, err := os.ReadFile(filepath.Join(append([]string{projectRoot}, path...)...))
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}
	up := read("deploy", "standalone", "migrations", "0108_scrm_public_pool_parity.up.sql")
	down := read("deploy", "standalone", "migrations", "0108_scrm_public_pool_parity.down.sql")
	for _, fragment := range []string{"`source`", "`business_type`", "`region`", "mochat_go_scrm_assignment_history", "previous_owner_id", "actor_id", "reason", "idx_scrm_pool_history_contact"} {
		if !strings.Contains(up, fragment) {
			t.Errorf("public-pool migration missing %q", fragment)
		}
	}
	for _, fragment := range []string{"information_schema.columns", "PREPARE public_pool_source_stmt", "PREPARE public_pool_history_stmt", "PREPARE public_pool_history_filter_index_stmt"} {
		if !strings.Contains(up, fragment) {
			t.Errorf("public-pool migration missing already-applied guard fragment %s", fragment)
		}
	}
	for _, fragment := range []string{"DROP TABLE IF EXISTS `mochat_go_scrm_assignment_history`", "DROP COLUMN `region`", "DROP COLUMN `business_type`", "DROP COLUMN `source`"} {
		if !strings.Contains(down, fragment) {
			t.Errorf("down migration missing %q", fragment)
		}
	}
}

func TestContactLifecycleIdempotencyMigrationIsReversible(t *testing.T) {
	projectRoot := filepath.Join("..", "..")
	up, err := os.ReadFile(filepath.Join(projectRoot, "deploy", "standalone", "migrations", "0107_scrm_contact_lifecycle_idempotency.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	down, err := os.ReadFile(filepath.Join(projectRoot, "deploy", "standalone", "migrations", "0107_scrm_contact_lifecycle_idempotency.down.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{"mochat_go_scrm_idempotency_keys", "request_fingerprint", "PRIMARY KEY (`tenant_id`,`corp_id`,`action`,`idempotency_key`)"} {
		if !strings.Contains(string(up), fragment) {
			t.Errorf("up migration missing %s", fragment)
		}
	}
	for _, fragment := range []string{"idempotency_already_applied", "information_schema.columns", "PREPARE idempotency_stmt", "DEALLOCATE PREPARE idempotency_stmt"} {
		if !strings.Contains(string(up), fragment) {
			t.Errorf("idempotency migration missing already-applied guard fragment %s", fragment)
		}
	}
	if !strings.Contains(string(down), "DROP TABLE IF EXISTS `mochat_go_scrm_idempotency_keys`") {
		t.Fatal("down migration does not remove idempotency table")
	}
}

func TestLeadParityMigrationMatchesStandaloneSchema(t *testing.T) {
	projectRoot := filepath.Join("..", "..")
	read := func(path ...string) string {
		t.Helper()
		body, err := os.ReadFile(filepath.Join(append([]string{projectRoot}, path...)...))
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}
	up := read("deploy", "standalone", "migrations", "0106_scrm_lead_parity.up.sql")
	down := read("deploy", "standalone", "migrations", "0106_scrm_lead_parity.down.sql")
	for _, fragment := range []string{"cannot uniquely map historical leads", "HAVING COUNT(c.`id`) <> 1", "UPDATE `mochat_go_scrm_leads`", "`corp_id`", "`phone`", "`owner_id`", "`converted_contact_id`", "`discard_reason`", "uk_scrm_leads_scope_phone", "idx_scrm_leads_combined_filter"} {
		if !strings.Contains(up, fragment) {
			t.Errorf("up migration missing %s", fragment)
		}
	}
	for _, fragment := range []string{"cross-corp business_key conflict", "HAVING COUNT(DISTINCT `corp_id`) > 1", "DROP INDEX `uk_scrm_leads_scope_phone`", "DROP COLUMN `corp_id`"} {
		if !strings.Contains(down, fragment) {
			t.Errorf("down migration missing %s", fragment)
		}
	}
}

func TestLeadParityMigrationSkipsAlreadyMigratedSchema(t *testing.T) {
	projectRoot := filepath.Join("..", "..")
	body, err := os.ReadFile(filepath.Join(projectRoot, "deploy", "standalone", "migrations", "0106_scrm_lead_parity.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{"lead_parity_already_applied", "information_schema.columns", "PREPARE lead_parity_stmt", "DEALLOCATE PREPARE lead_parity_stmt"} {
		if !strings.Contains(string(body), fragment) {
			t.Errorf("lead parity migration missing already-applied guard fragment %s", fragment)
		}
	}
}

func TestCorpDataRealtimeIndexMigrationMatchesStandaloneSchema(t *testing.T) {
	projectRoot := filepath.Join("..", "..")
	read := func(path ...string) string {
		t.Helper()
		body, err := os.ReadFile(filepath.Join(append([]string{projectRoot}, path...)...))
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}

	up := read("deploy", "standalone", "migrations", "0105_corp_data_realtime_indexes.up.sql")
	down := read("deploy", "standalone", "migrations", "0105_corp_data_realtime_indexes.down.sql")
	schema := read("deploy", "standalone", "schema", "mochat.sql")
	legacy := strings.ReplaceAll(strings.TrimSpace(read("deploy", "standalone", "migrations", "0103_phase3_2_query_indexes.up.sql")), "\r\n", "\n")
	if legacy != "ALTER TABLE mc_corp_day_data\n  ADD INDEX idx_mc_corp_day_data_corp_date (corp_id, date);" {
		t.Fatalf("0103 semantics changed: %q", legacy)
	}

	indexes := []string{
		"idx_mc_wce_corp_deleted_create_employee",
		"idx_mc_wce_corp_status_deleted_employee",
		"idx_mc_wr_corp_deleted_created_owner",
		"idx_mc_wcr_room_status_deleted_join",
		"idx_mc_wcr_room_status_deleted_updated",
		"idx_mc_we_corp_status_deleted",
		"idx_mc_wed_employee_deleted_department",
	}
	for _, index := range indexes {
		if !strings.Contains(up, index) {
			t.Errorf("up migration missing %s", index)
		}
		if !strings.Contains(down, "DROP INDEX "+index) {
			t.Errorf("down migration missing %s", index)
		}
		if !strings.Contains(schema, index) {
			t.Errorf("standalone schema missing %s", index)
		}
	}
	if got := strings.Count(up, "information_schema.statistics"); got != len(indexes) {
		t.Fatalf("up migration idempotence guards = %d, want %d", got, len(indexes))
	}
	for _, fragment := range []string{"table_schema = DATABASE()", "PREPARE corp_data_index_stmt", "DEALLOCATE PREPARE corp_data_index_stmt"} {
		if !strings.Contains(up, fragment) {
			t.Fatalf("up migration missing fresh-schema idempotence fragment %q", fragment)
		}
	}
}

func TestLegacyCombinedInitialChecksums(t *testing.T) {
	root := t.TempDir()
	schemaPath := filepath.Join(root, "mochat.sql")
	seedPath := filepath.Join(root, "0002_seed_core_data.up.sql")
	schema := "CREATE TABLE `mc_user` (`id` int);\n"
	seed := "-- MoChat standalone core seed data.\n\n-- ----------------------------\n-- 高级属性\n-- ----------------------------\nINSERT IGNORE INTO `mc_contact_field` (`id`) VALUES (1);\n"
	if err := os.WriteFile(schemaPath, []byte(schema), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(seedPath, []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}

	legacyBody := strings.TrimRight(schema, " \t\r\n") + "\n\n\n" + strings.ReplaceAll(standaloneSeedBodyForLegacyChecksum(seed), "INSERT IGNORE INTO", "INSERT INTO")
	legacyChecksum := checksumBytes([]byte(legacyBody))
	aliases := legacyCombinedInitialChecksums(schemaPath, seedPath)
	if !checksumMatches(legacyChecksum, "current-checksum", aliases) {
		t.Fatalf("legacy checksum %s not accepted by aliases %#v", legacyChecksum, aliases)
	}
	if checksumMatches("other", "current-checksum", aliases) {
		t.Fatalf("unexpected checksum accepted")
	}
}
