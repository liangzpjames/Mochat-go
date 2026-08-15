package migration

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
)

var archiveRunnerSchemaSequence atomic.Int64

func TestArchiveSourceMigrationRunnerApplyDownApplyPinsOneConnection(t *testing.T) {
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
	schema := fmt.Sprintf("mochat_archive_runner_%d_%d", os.Getpid(), archiveRunnerSchemaSequence.Add(1))
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
	t.Cleanup(func() { _ = db.Close() })
	if err := db.PingContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE mc_corp (id INT(10) UNSIGNED NOT NULL AUTO_INCREMENT, tenant_id INT(10) UNSIGNED NOT NULL, deleted_at DATETIME NULL, PRIMARY KEY (id), UNIQUE KEY uk_runner_corp_scope (tenant_id,id)) ENGINE=InnoDB`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO mc_corp (id,tenant_id) VALUES (27,11)`); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join("..", "..")
	migration := Migration{
		Version:     "0138_archive_source_sync",
		Description: "archive source sync",
		Path:        filepath.Join(root, "deploy", "standalone", "migrations", "0138_archive_source_sync.up.sql"),
		DownPath:    filepath.Join(root, "deploy", "standalone", "migrations", "0138_archive_source_sync.down.sql"),
	}
	runner, err := NewRunner(db, []Migration{migration})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runner.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertArchiveRunnerTables(t, db, true)
	var applied int
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_schema_migrations WHERE version='0138_archive_source_sync'`).Scan(&applied); err != nil {
		t.Fatal(err)
	}
	if applied != 1 {
		t.Fatalf("applied ledger rows=%d", applied)
	}
	if _, err := runner.RollbackLast(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertArchiveRunnerTables(t, db, false)
	if _, err := db.Exec(`CREATE TABLE mochat_go_archive_sync_runs (id BIGINT UNSIGNED NOT NULL PRIMARY KEY) ENGINE=InnoDB`); err != nil {
		t.Fatal(err)
	}
	if _, err := runner.Apply(context.Background()); err == nil {
		t.Fatal("incompatible residual archive table unexpectedly applied")
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_schema_migrations WHERE version='0138_archive_source_sync'`).Scan(&applied); err != nil {
		t.Fatal(err)
	}
	if applied != 0 {
		t.Fatalf("failed apply recorded ledger rows=%d", applied)
	}
	if _, err := db.Exec(`DROP TABLE mochat_go_archive_sync_runs`); err != nil {
		t.Fatal(err)
	}
	if _, err := runner.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertArchiveRunnerTables(t, db, true)
	if statuses, err := runner.Apply(context.Background()); err != nil {
		t.Fatal(err)
	} else if len(statuses) != 1 || statuses[0].State != "applied" {
		t.Fatalf("repeat apply statuses=%#v", statuses)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_schema_migrations WHERE version='0138_archive_source_sync'`).Scan(&applied); err != nil {
		t.Fatal(err)
	}
	if applied != 1 {
		t.Fatalf("repeat apply ledger rows=%d", applied)
	}
}

func assertArchiveRunnerTables(t *testing.T, db *sql.DB, want bool) {
	t.Helper()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name IN ('mochat_go_archive_sync_runs','mochat_go_archive_sync_audits','mochat_go_archive_message_sources')`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if (count == 3) != want {
		t.Fatalf("archive tables=%d wantPresent=%v", count, want)
	}
}
