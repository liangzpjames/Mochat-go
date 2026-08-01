package migration

import (
	"fmt"
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

func TestStandaloneComposeFreshInitUsesSchemaForLatestIndexes(t *testing.T) {
	projectRoot := filepath.Join("..", "..")
	migrations := DefaultMigrations(projectRoot)
	latest := migrations[len(migrations)-1]
	composePath := filepath.Join(projectRoot, "deploy", "standalone", "docker-compose.yml")
	composeBody, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatal(err)
	}

	if latest.Version != "0106_work_message_global_search_indexes" {
		t.Fatalf("latest migration = %q, want 0106_work_message_global_search_indexes", latest.Version)
	}
	for _, mount := range []string{
		"./migrations/0105_corp_data_realtime_indexes.up.sql:",
		"./migrations/0106_work_message_global_search_indexes.up.sql:",
	} {
		if strings.Contains(string(composeBody), mount) {
			t.Fatalf("standalone fresh init must use the synchronized base schema instead of replaying %q", mount)
		}
	}
}

func TestWorkMessageGlobalIndexMigrationMatchesStandaloneSchema(t *testing.T) {
	projectRoot := filepath.Join("..", "..")
	read := func(path ...string) string {
		t.Helper()
		body, err := os.ReadFile(filepath.Join(append([]string{projectRoot}, path...)...))
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}

	up := read("deploy", "standalone", "migrations", "0106_work_message_global_search_indexes.up.sql")
	down := read("deploy", "standalone", "migrations", "0106_work_message_global_search_indexes.down.sql")
	schema := read("deploy", "standalone", "schema", "mochat.sql")
	published := read("deploy", "standalone", "migrations", "0014_auto_tag.up.sql")
	if strings.Contains(published, "global_search") {
		t.Fatal("published migration 0014 must not be modified")
	}
	for tableIndex := 1; tableIndex <= 10; tableIndex++ {
		index := fmt.Sprintf("idx_mc_work_message_%d_global_search", tableIndex)
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
	if got := strings.Count(up, "information_schema.statistics"); got != 10 {
		t.Fatalf("up migration idempotence guards = %d, want 10", got)
	}
	for _, fragment := range []string{
		"table_schema = DATABASE()",
		"(corp_id, work_employee_id, deleted_at, to_user_type, msg_data_time, to_user_id, seq)",
		"PREPARE work_message_index_stmt",
		"DEALLOCATE PREPARE work_message_index_stmt",
	} {
		if !strings.Contains(up, fragment) {
			t.Fatalf("up migration missing fragment %q", fragment)
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
	legacy := strings.TrimSpace(read("deploy", "standalone", "migrations", "0103_phase3_2_query_indexes.up.sql"))
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

func TestDefaultMigrationsAcceptsPreGlobalMessageSchemaChecksum(t *testing.T) {
	migrations := DefaultMigrations(filepath.Join("..", ".."))
	const previousChecksum = "b7dbd66b24b93a4be64e33fa51d2e1a1fcbc0d305532145644c37ed1a26075e9"
	if !checksumMatches(previousChecksum, "different-current-checksum", migrations[0].ChecksumAliases) {
		t.Fatalf("pre-0106 standalone schema checksum is not accepted: %#v", migrations[0].ChecksumAliases)
	}
}
