//go:build integration

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

	mysqldriver "github.com/go-sql-driver/mysql"
)

var leadParitySchemaSequence atomic.Int64

func TestLeadParityMigrationHistoricalUpgradeAndRollbackSafety(t *testing.T) {
	t.Run("unique active corp backfills historical leads and rolls back", func(t *testing.T) {
		db := newLeadParityMigrationDB(t)
		createLeadParityLegacyFixture(t, db, 41, 701, "history-one")

		execLeadParityMigration(t, db, "0106_scrm_lead_parity.up.sql", false)
		var corpID int64
		if err := db.QueryRow("SELECT corp_id FROM mochat_go_scrm_leads WHERE id='history-one'").Scan(&corpID); err != nil {
			t.Fatal(err)
		}
		if corpID != 701 {
			t.Fatalf("historical lead corp_id=%d, want 701", corpID)
		}

		execLeadParityMigration(t, db, "0106_scrm_lead_parity.down.sql", false)
		if columnExists(t, db, "mochat_go_scrm_leads", "corp_id") {
			t.Fatal("corp_id still exists after successful rollback")
		}
		if !indexExists(t, db, "mochat_go_scrm_leads", "uk_scrm_leads_tenant_business_key") {
			t.Fatal("legacy business-key index was not restored")
		}
	})

	t.Run("ambiguous historical tenant fails before schema mutation", func(t *testing.T) {
		db := newLeadParityMigrationDB(t)
		createLeadParityLegacyFixture(t, db, 42, 702, "history-ambiguous")
		if _, err := db.Exec("INSERT INTO mc_corp (id,tenant_id,deleted_at) VALUES (703,42,NULL)"); err != nil {
			t.Fatal(err)
		}

		err := execLeadParityMigration(t, db, "0106_scrm_lead_parity.up.sql", true)
		if !strings.Contains(err.Error(), "cannot uniquely map historical leads") {
			t.Fatalf("upgrade error=%v", err)
		}
		if columnExists(t, db, "mochat_go_scrm_leads", "corp_id") {
			t.Fatal("ambiguous upgrade partially added corp_id")
		}
		if !indexExists(t, db, "mochat_go_scrm_leads", "uk_scrm_leads_tenant_business_key") {
			t.Fatal("ambiguous upgrade partially removed legacy index")
		}
	})

	t.Run("cross-corp business-key conflict fails before rollback mutation", func(t *testing.T) {
		db := newLeadParityMigrationDB(t)
		createLeadParityLegacyFixture(t, db, 43, 704, "history-conflict")
		execLeadParityMigration(t, db, "0106_scrm_lead_parity.up.sql", false)
		if _, err := db.Exec("INSERT INTO mc_corp (id,tenant_id,deleted_at) VALUES (705,43,NULL)"); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO mochat_go_scrm_leads (id,tenant_id,corp_id,business_key,name,source,status,version,created_at,updated_at) VALUES ('cross-corp',43,705,'legacy-key','Cross corp','manual','new',1,NOW(6),NOW(6))`); err != nil {
			t.Fatal(err)
		}

		err := execLeadParityMigration(t, db, "0106_scrm_lead_parity.down.sql", true)
		if !strings.Contains(err.Error(), "cross-corp business_key conflict") {
			t.Fatalf("rollback error=%v", err)
		}
		for _, column := range []string{"corp_id", "phone", "owner_id", "converted_contact_id", "discard_reason"} {
			if !columnExists(t, db, "mochat_go_scrm_leads", column) {
				t.Fatalf("rollback partially removed %s", column)
			}
		}
		if !indexExists(t, db, "mochat_go_scrm_leads", "uk_scrm_leads_scope_business_key") {
			t.Fatal("rollback partially removed scoped business-key index")
		}
	})
}

func newLeadParityMigrationDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("MOCHAT_MYSQL_DSN")
	if dsn == "" {
		if os.Getenv("MOCHAT_REQUIRE_MYSQL_INTEGRATION") == "1" {
			t.Fatal("MOCHAT_MYSQL_DSN is required when MOCHAT_REQUIRE_MYSQL_INTEGRATION=1")
		}
		t.Skip("MOCHAT_MYSQL_DSN is required for MariaDB migration tests")
	}
	cfg, err := mysqldriver.ParseDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	adminCfg := *cfg
	adminCfg.DBName = ""
	admin, err := sql.Open("mysql", adminCfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("mochat_task6_migration_%d_%d", os.Getpid(), leadParitySchemaSequence.Add(1))
	if _, err := admin.Exec("CREATE DATABASE `" + schema + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
		admin.Close()
		t.Fatalf("create isolated migration schema: %v", err)
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

func createLeadParityLegacyFixture(t *testing.T, db *sql.DB, tenantID, corpID int64, leadID string) {
	t.Helper()
	if _, err := db.Exec(`CREATE TABLE mc_corp (id bigint unsigned NOT NULL, tenant_id bigint NOT NULL, deleted_at timestamp NULL, PRIMARY KEY (id)) ENGINE=InnoDB`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO mc_corp (id,tenant_id,deleted_at) VALUES (?,?,NULL)", corpID, tenantID); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join("..", "..", "deploy", "standalone", "migrations", "0098_scrm_lead_foundation.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if err := execSQLScript(context.Background(), db, string(body)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO mochat_go_scrm_leads (id,tenant_id,business_key,name,source,status,version,created_at,updated_at) VALUES (?,?,'legacy-key','Historical','manual','new',1,NOW(6),NOW(6))`, leadID, tenantID); err != nil {
		t.Fatal(err)
	}
}

func execLeadParityMigration(t *testing.T, db *sql.DB, name string, wantError bool) error {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("..", "..", "deploy", "standalone", "migrations", name))
	if err != nil {
		t.Fatal(err)
	}
	err = execSQLScript(context.Background(), db, string(body))
	if wantError && err == nil {
		t.Fatalf("%s unexpectedly succeeded", name)
	}
	if !wantError && err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return err
}

func columnExists(t *testing.T, db *sql.DB, table, column string) bool {
	t.Helper()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name=? AND column_name=?`, table, column).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count == 1
}

func indexExists(t *testing.T, db *sql.DB, table, index string) bool {
	t.Helper()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name=? AND index_name=?`, table, index).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count > 0
}
