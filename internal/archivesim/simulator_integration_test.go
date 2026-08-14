package archivesim

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/go-sql-driver/mysql"

	"jiyi/mochat-go/internal/migration"
	archivesourcefixture "jiyi/mochat-go/internal/testfixtures/archivesource"
)

var simulationSchemaSequence atomic.Int64

func TestApplyCleanupReapplyRemovesSimulationSourceRun(t *testing.T) {
	db := newArchiveSimulationIntegrationDB(t)
	createArchiveSimulationFixture(t, db)
	executeArchiveSimulationMigration(t, db, "0138_archive_source_sync.up.sql")
	executeArchiveSimulationMigration(t, db, "0133_archive_simulation_registry.up.sql")

	simulator := New(db)
	ctx := context.Background()
	first, err := simulator.Apply(ctx, 27, "cleanup-reapply")
	if err != nil {
		t.Fatal(err)
	}
	if first.Status != "complete" || first.MessageCount != 13 || first.Idempotent {
		t.Fatalf("first apply=%#v", first)
	}
	var runID int64
	if err := db.QueryRow(`SELECT id FROM mochat_go_archive_sync_runs WHERE tenant_id=11 AND corp_id=27 AND source_kind='simulated' AND source_id='simulation:cleanup-reapply' AND namespace='MOCHAT-SIM:cleanup-reapply'`).Scan(&runID); err != nil {
		t.Fatal(err)
	}

	cleaned, err := simulator.Cleanup(ctx, 27, "cleanup-reapply")
	if err != nil {
		t.Fatal(err)
	}
	if cleaned.Status != "cleaned" || cleaned.MessageCount != 13 {
		t.Fatalf("cleanup=%#v", cleaned)
	}
	assertSimulationRows(t, db, runID, false)

	second, err := simulator.Apply(ctx, 27, "cleanup-reapply")
	if err != nil {
		t.Fatal(err)
	}
	if second.Status != "complete" || second.MessageCount != 13 || second.Idempotent {
		t.Fatalf("reapply=%#v", second)
	}
	var newRunID int64
	if err := db.QueryRow(`SELECT id FROM mochat_go_archive_sync_runs WHERE tenant_id=11 AND corp_id=27 AND source_kind='simulated' AND source_id='simulation:cleanup-reapply' AND namespace='MOCHAT-SIM:cleanup-reapply'`).Scan(&newRunID); err != nil {
		t.Fatal(err)
	}
	if newRunID == runID {
		t.Fatalf("reapply reused deleted run id=%d", newRunID)
	}
	assertSimulationRows(t, db, newRunID, true)
}

func assertSimulationRows(t *testing.T, db *sql.DB, expectedRunID int64, expectedPresence bool) {
	t.Helper()
	var sources, audits, runs, messages int
	for query, destination := range map[string]*int{
		`SELECT COUNT(*) FROM mochat_go_archive_message_sources WHERE tenant_id=11 AND corp_id=27 AND source_kind='simulated' AND source_id='simulation:cleanup-reapply' AND namespace='MOCHAT-SIM:cleanup-reapply'`: &sources,
		`SELECT COUNT(*) FROM mochat_go_archive_sync_audits WHERE tenant_id=11 AND corp_id=27 AND source_kind='simulated' AND source_id='simulation:cleanup-reapply' AND namespace='MOCHAT-SIM:cleanup-reapply'`:     &audits,
		`SELECT COUNT(*) FROM mochat_go_archive_sync_runs WHERE tenant_id=11 AND corp_id=27 AND source_kind='simulated' AND source_id='simulation:cleanup-reapply' AND namespace='MOCHAT-SIM:cleanup-reapply'`:       &runs,
	} {
		if err := db.QueryRow(query).Scan(destination); err != nil {
			t.Fatal(err)
		}
	}
	for index := 1; index <= 10; index++ {
		var count int
		if err := db.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM mc_work_message_%d WHERE corp_id=27 AND msgid LIKE 'MOCHAT-SIM:cleanup-reapply:%%'`, index)).Scan(&count); err != nil {
			t.Fatal(err)
		}
		messages += count
	}
	if !expectedPresence {
		if expectedRunID <= 0 || sources != 0 || audits != 0 || runs != 0 || messages != 0 {
			t.Fatalf("cleanup left expectedRunID=%d sources=%d audits=%d runs=%d messages=%d", expectedRunID, sources, audits, runs, messages)
		}
		var oldRunRows int
		if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_archive_sync_runs WHERE id=?`, expectedRunID).Scan(&oldRunRows); err != nil {
			t.Fatal(err)
		}
		if oldRunRows != 0 {
			t.Fatalf("cleanup retained old run id=%d", expectedRunID)
		}
		return
	}
	if expectedRunID <= 0 || sources != 13 || audits == 0 || runs != 1 || messages != 13 {
		t.Fatalf("reapply rows sources=%d audits=%d runs=%d messages=%d", sources, audits, runs, messages)
	}
	var matchingRuns int
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_archive_sync_runs WHERE id=? AND tenant_id=11 AND corp_id=27 AND source_kind='simulated' AND source_id='simulation:cleanup-reapply' AND namespace='MOCHAT-SIM:cleanup-reapply'`, expectedRunID).Scan(&matchingRuns); err != nil {
		t.Fatal(err)
	}
	if matchingRuns != 1 {
		t.Fatalf("reapply expected run=%d rows=%d", expectedRunID, matchingRuns)
	}
}

func createArchiveSimulationFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, statement := range []string{
		`CREATE TABLE mc_corp (id INT(10) UNSIGNED NOT NULL AUTO_INCREMENT, tenant_id INT(10) UNSIGNED NOT NULL, deleted_at DATETIME NULL, PRIMARY KEY (id), UNIQUE KEY uk_mc_corp_tenant_id (tenant_id,id)) ENGINE=InnoDB`,
		`INSERT INTO mc_corp (id,tenant_id) VALUES (27,11)`,
		`CREATE TABLE mc_work_message_id (id INT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY, corp_id INT UNSIGNED NOT NULL, type INT NOT NULL, last_id BIGINT NOT NULL DEFAULT 0, deleted_at DATETIME NULL, created_at DATETIME NULL, updated_at DATETIME NULL) ENGINE=InnoDB`,
		`CREATE TABLE mc_work_employee (id INT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY, corp_id INT UNSIGNED NOT NULL, wx_user_id VARCHAR(255) NOT NULL, name VARCHAR(255) NOT NULL DEFAULT '', avatar VARCHAR(255) NOT NULL DEFAULT '', status INT NOT NULL DEFAULT 1, log_user_id INT NOT NULL DEFAULT 0, deleted_at DATETIME NULL, created_at DATETIME NULL, updated_at DATETIME NULL) ENGINE=InnoDB`,
		`CREATE TABLE mc_work_contact (id INT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY, corp_id INT UNSIGNED NOT NULL, wx_external_userid VARCHAR(255) NOT NULL, name VARCHAR(255) NOT NULL DEFAULT '', nick_name VARCHAR(255) NOT NULL DEFAULT '', avatar VARCHAR(255) NOT NULL DEFAULT '', deleted_at DATETIME NULL, created_at DATETIME NULL, updated_at DATETIME NULL) ENGINE=InnoDB`,
		`CREATE TABLE mc_work_room (id INT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY, corp_id INT UNSIGNED NOT NULL, wx_chat_id VARCHAR(255) NOT NULL, name VARCHAR(255) NOT NULL DEFAULT '', owner_id INT NOT NULL DEFAULT 0, notice TEXT NOT NULL, status INT NOT NULL DEFAULT 1, deleted_at DATETIME NULL, created_at DATETIME NULL, updated_at DATETIME NULL) ENGINE=InnoDB`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	if err := archivesourcefixture.PrepareDashboardPermissionDependencies(context.Background(), db); err != nil {
		t.Fatalf("0127 dashboard permission fixture: %v", err)
	}
	for index := 1; index <= 10; index++ {
		if _, err := db.Exec(fmt.Sprintf(`CREATE TABLE mc_work_message_%d (
			id INT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY, corp_id INT UNSIGNED NOT NULL, msgid VARCHAR(255) NOT NULL,
			seq BIGINT NOT NULL, work_employee_id INT NOT NULL, to_user_type INT NOT NULL, to_user_id INT NOT NULL,
			sender_type INT NOT NULL DEFAULT 0, action INT NOT NULL DEFAULT 0, type INT NOT NULL DEFAULT 1, msg_type INT NOT NULL DEFAULT 1,
			content TEXT NOT NULL, content_text TEXT NOT NULL, room_id INT NOT NULL DEFAULT 0, status INT NOT NULL DEFAULT 0,
			msg_data_time DATETIME NULL, created_at DATETIME NULL, updated_at DATETIME NULL, deleted_at DATETIME NULL,
			KEY idx_sim_message_scope (corp_id,msgid)
		) ENGINE=InnoDB`, index)); err != nil {
			t.Fatal(err)
		}
	}
}

func newArchiveSimulationIntegrationDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("MOCHAT_GO_MYSQL_INTEGRATION_DSN"))
	if dsn == "" {
		t.Skip("SKIP: MOCHAT_GO_MYSQL_INTEGRATION_DSN is not set; isolated MariaDB DSN is required")
	}
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	adminCfg := *cfg
	adminCfg.DBName = ""
	admin, err := sql.Open("mysql", adminCfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	if err := admin.PingContext(context.Background()); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	schema := fmt.Sprintf("mochat_archive_sim_%d_%d", os.Getpid(), simulationSchemaSequence.Add(1))
	if _, err := admin.Exec("CREATE DATABASE `" + schema + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec("DROP DATABASE IF EXISTS `" + schema + "`")
		_ = admin.Close()
	})
	testCfg := *cfg
	testCfg.DBName = schema
	db, err := sql.Open("mysql", testCfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	if err := db.PingContext(context.Background()); err != nil {
		db.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func executeArchiveSimulationMigration(t *testing.T, db *sql.DB, name string) {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join("..", "..", "deploy", "standalone", "migrations", name))
	if err != nil {
		t.Fatal(err)
	}
	statements, err := migration.SplitSQLStatements(string(contents))
	if err != nil {
		t.Fatal(err)
	}
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	for _, statement := range statements {
		if _, err := conn.ExecContext(context.Background(), statement); err != nil {
			t.Fatalf("migration %s: %v", name, err)
		}
	}
}
