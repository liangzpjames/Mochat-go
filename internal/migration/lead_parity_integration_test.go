//go:build integration

package migration

import (
	"context"
	"database/sql"
	"strings"
	"testing"
)

func TestLeadParityMigrationHistoricalUpgradeAndRollbackSafety(t *testing.T) {
	t.Run("unique active corp backfills historical leads and rolls back", func(t *testing.T) {
		db := newMigrationIntegrationDBThrough(t, "0105_corp_data_realtime_indexes")
		createLeadParityLegacyFixture(t, db, 41, 701, "history-one")

		runner := newMigrationRunnerThrough(t, db, "0106_scrm_lead_parity")
		applyLeadParityMigration(t, runner, false)
		var corpID int64
		if err := db.QueryRow("SELECT corp_id FROM mochat_go_scrm_leads WHERE id='history-one'").Scan(&corpID); err != nil {
			t.Fatal(err)
		}
		if corpID != 701 {
			t.Fatalf("historical lead corp_id=%d, want 701", corpID)
		}

		rollbackLeadParityMigration(t, runner, false)
		if columnExists(t, db, "mochat_go_scrm_leads", "corp_id") {
			t.Fatal("corp_id still exists after successful rollback")
		}
		if !indexExists(t, db, "mochat_go_scrm_leads", "uk_scrm_leads_tenant_business_key") {
			t.Fatal("legacy business-key index was not restored")
		}
	})

	t.Run("ambiguous historical tenant fails before schema mutation", func(t *testing.T) {
		db := newMigrationIntegrationDBThrough(t, "0105_corp_data_realtime_indexes")
		createLeadParityLegacyFixture(t, db, 42, 702, "history-ambiguous")
		if _, err := db.Exec("INSERT INTO mc_corp (id,tenant_id,name,deleted_at) VALUES (703,42,'Ambiguous corp',NULL)"); err != nil {
			t.Fatal(err)
		}

		runner := newMigrationRunnerThrough(t, db, "0106_scrm_lead_parity")
		err := applyLeadParityMigration(t, runner, true)
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
		db := newMigrationIntegrationDBThrough(t, "0105_corp_data_realtime_indexes")
		createLeadParityLegacyFixture(t, db, 43, 704, "history-conflict")
		runner := newMigrationRunnerThrough(t, db, "0106_scrm_lead_parity")
		applyLeadParityMigration(t, runner, false)
		if _, err := db.Exec("INSERT INTO mc_corp (id,tenant_id,name,deleted_at) VALUES (705,43,'Conflict corp',NULL)"); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO mochat_go_scrm_leads (id,tenant_id,corp_id,business_key,name,source,status,version,created_at,updated_at) VALUES ('cross-corp',43,705,'legacy-key','Cross corp','manual','new',1,NOW(6),NOW(6))`); err != nil {
			t.Fatal(err)
		}

		err := rollbackLeadParityMigration(t, runner, true)
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

func createLeadParityLegacyFixture(t *testing.T, db *sql.DB, tenantID, corpID int64, leadID string) {
	t.Helper()
	if _, err := db.Exec("INSERT INTO mc_corp (id,tenant_id,name,deleted_at) VALUES (?,?,'Historical corp',NULL)", corpID, tenantID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO mochat_go_scrm_leads (id,tenant_id,business_key,name,source,status,version,created_at,updated_at) VALUES (?,?,'legacy-key','Historical','manual','new',1,NOW(6),NOW(6))`, leadID, tenantID); err != nil {
		t.Fatal(err)
	}
}

func applyLeadParityMigration(t *testing.T, runner *Runner, wantError bool) error {
	t.Helper()
	items, err := runner.Apply(context.Background())
	if wantError && err == nil {
		t.Fatal("0106_scrm_lead_parity unexpectedly succeeded")
	}
	if !wantError && err != nil {
		t.Fatalf("apply production registry through 0106_scrm_lead_parity: %v", err)
	}
	if !wantError && (len(items) == 0 || items[len(items)-1].Migration.Version != "0106_scrm_lead_parity" || items[len(items)-1].State != "applied_now") {
		t.Fatalf("0106 apply status=%#v", items)
	}
	return err
}

func rollbackLeadParityMigration(t *testing.T, runner *Runner, wantError bool) error {
	t.Helper()
	version, err := runner.RollbackLast(context.Background())
	if wantError && err == nil {
		t.Fatal("0106_scrm_lead_parity rollback unexpectedly succeeded")
	}
	if !wantError && err != nil {
		t.Fatalf("rollback 0106_scrm_lead_parity: %v", err)
	}
	if !wantError && version != "0106_scrm_lead_parity" {
		t.Fatalf("rollback version=%q, want 0106_scrm_lead_parity", version)
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
