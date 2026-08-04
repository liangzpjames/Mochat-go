package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
	lfChecksum := checksumBytes([]byte(other))
	lineEndingMigration := DefaultMigrations(root)[2]
	if !checksumMatches(lfChecksum, checksumBytes([]byte(strings.ReplaceAll(other, "\n", "\r\n"))), lineEndingMigration.ChecksumAliases) {
		t.Fatalf("LF checksum %s not accepted by aliases %#v", lfChecksum, lineEndingMigration.ChecksumAliases)
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

func TestStandaloneComposeFreshInitUsesSchemaForCorpDataIndexes(t *testing.T) {
	projectRoot := filepath.Join("..", "..")
	migrations := DefaultMigrations(projectRoot)
	latest := migrations[len(migrations)-1]
	composePath := filepath.Join(projectRoot, "deploy", "standalone", "docker-compose.yml")
	composeBody, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatal(err)
	}

	if latest.Version != "0118_material_selector_references" {
		t.Fatalf("latest migration = %q, want 0118_material_selector_references", latest.Version)
	}
	if mount := "./migrations/0105_corp_data_realtime_indexes.up.sql:"; strings.Contains(string(composeBody), mount) {
		t.Fatalf("standalone fresh init must use the synchronized base schema instead of replaying %q", mount)
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
