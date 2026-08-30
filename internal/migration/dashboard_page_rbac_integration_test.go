package migration

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDashboardPageRBACIntegration(t *testing.T) {
	t.Run("apply down apply and legacy api mapping", func(t *testing.T) {
		db := newDashboardRBACMigrationDB(t)
		createDashboardRBACLegacyFixture(t, db)
		execDashboardRBACMigration(t, db, "0127_dashboard_page_rbac.up.sql", false)

		var userRoles int
		if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_dashboard_user_roles WHERE tenant_id=1 AND user_id=10 AND role_id=20`).Scan(&userRoles); err != nil {
			t.Fatal(err)
		}
		if userRoles != 1 {
			t.Fatalf("legacy user-role backfill count=%d, want 1", userRoles)
		}
		var grantable, protected int
		if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_dashboard_role_permissions rp INNER JOIN mochat_go_dashboard_permissions p ON p.id=rp.permission_id WHERE rp.tenant_id=1 AND rp.role_id=20 AND p.path IN ('/acquisition/v2-channel-code','/customer/friends')`).Scan(&grantable); err != nil {
			t.Fatal(err)
		}
		if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_dashboard_role_permissions rp INNER JOIN mochat_go_dashboard_permissions p ON p.id=rp.permission_id WHERE rp.tenant_id=1 AND rp.role_id=20 AND p.superadmin_only=1`).Scan(&protected); err != nil {
			t.Fatal(err)
		}
		if grantable != 2 || protected != 0 {
			t.Fatalf("legacy API mapping grantable=%d protected=%d, want 2 and 0", grantable, protected)
		}

		execDashboardRBACMigration(t, db, "0127_dashboard_page_rbac.down.sql", false)
		assertDashboardRBACRemoved(t, db)
		execDashboardRBACMigration(t, db, "0127_dashboard_page_rbac.up.sql", false)
		if !dashboardColumnExists(t, db, "mc_user", "dashboard_access_version") {
			t.Fatal("dashboard_access_version missing after second apply")
		}
	})

	t.Run("forward correction preserves legacy tenant department and self scopes", func(t *testing.T) {
		db := newDashboardRBACMigrationDB(t)
		createDashboardRBACLegacyFixture(t, db)
		execDashboardRBACMigration(t, db, "0127_dashboard_page_rbac.up.sql", false)
		execDashboardRBACMigration(t, db, "0128_dashboard_page_rbac_legacy_scope_fix.up.sql", false)

		want := map[int]string{20: "department", 21: "self", 22: "tenant", 23: "self"}
		for roleID, wantScope := range want {
			var scope string
			err := db.QueryRow(`
				SELECT rp.data_scope
				FROM mochat_go_dashboard_role_permissions rp
				INNER JOIN mochat_go_dashboard_permissions p ON p.id=rp.permission_id
				WHERE rp.tenant_id=1 AND rp.role_id=? AND p.path='/acquisition/v2-channel-code'
			`, roleID).Scan(&scope)
			if err != nil || scope != wantScope {
				t.Fatalf("role %d scope=%q want=%q err=%v", roleID, scope, wantScope, err)
			}
		}

		var reviewAudit int
		if err := db.QueryRow(`
			SELECT COUNT(*) FROM mochat_go_dashboard_permission_audits
			WHERE tenant_id=1 AND target_type='role' AND target_id='23'
			  AND action='migration.legacy_scope_review'
			  AND JSON_EXTRACT(after_json, '$.requiresAdminReview') = TRUE
		`).Scan(&reviewAudit); err != nil || reviewAudit != 1 {
			t.Fatalf("ambiguous role review audit=%d err=%v", reviewAudit, err)
		}
	})

	t.Run("subscription preflight fails before DDL", func(t *testing.T) {
		db := newDashboardRBACMigrationDB(t)
		createDashboardRBACLegacyFixture(t, db)
		if _, err := db.Exec(`DELETE FROM mochat_go_saas_subscriptions`); err != nil {
			t.Fatal(err)
		}
		err := execDashboardRBACMigration(t, db, "0127_dashboard_page_rbac.up.sql", true)
		if !strings.Contains(err.Error(), "missing subscription") {
			t.Fatalf("migration error=%v", err)
		}
		assertDashboardRBACPreflightLeftNoDDL(t, db)
	})

	t.Run("dangling legacy role fails before DDL", func(t *testing.T) {
		db := newDashboardRBACMigrationDB(t)
		createDashboardRBACLegacyFixture(t, db)
		if _, err := db.Exec(`INSERT INTO mc_rbac_user_role (user_id,role_id,deleted_at) VALUES (999,20,NULL)`); err != nil {
			t.Fatal(err)
		}
		err := execDashboardRBACMigration(t, db, "0127_dashboard_page_rbac.up.sql", true)
		if !strings.Contains(err.Error(), "dangling legacy user-role") {
			t.Fatalf("migration error=%v", err)
		}
		assertDashboardRBACPreflightLeftNoDDL(t, db)
	})

	t.Run("cross tenant legacy role fails before DDL", func(t *testing.T) {
		db := newDashboardRBACMigrationDB(t)
		createDashboardRBACLegacyFixture(t, db)
		if _, err := db.Exec(`INSERT INTO mc_user (id,tenant_id,isSuperAdmin,deleted_at) VALUES (11,2,0,NULL)`); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO mc_rbac_user_role (user_id,role_id,deleted_at) VALUES (11,20,NULL)`); err != nil {
			t.Fatal(err)
		}
		err := execDashboardRBACMigration(t, db, "0127_dashboard_page_rbac.up.sql", true)
		if !strings.Contains(err.Error(), "cross-tenant legacy user-role") {
			t.Fatalf("migration error=%v", err)
		}
		assertDashboardRBACPreflightLeftNoDDL(t, db)
	})

	t.Run("down recovers when only user alter completed", func(t *testing.T) {
		db := newDashboardRBACMigrationDB(t)
		createDashboardRBACLegacyFixture(t, db)
		if _, err := db.Exec(`ALTER TABLE mc_user ADD COLUMN dashboard_access_version bigint(20) unsigned NOT NULL DEFAULT 1, ADD UNIQUE INDEX uni_dashboard_user_tenant_id_id (tenant_id,id)`); err != nil {
			t.Fatal(err)
		}
		recoverDashboardRBACPartialState(t, db)
		assertDashboardRBACRemoved(t, db)
	})

	t.Run("down recovers from partially created tables", func(t *testing.T) {
		db := newDashboardRBACMigrationDB(t)
		createDashboardRBACLegacyFixture(t, db)
		if _, err := db.Exec(`ALTER TABLE mc_user ADD COLUMN dashboard_access_version bigint(20) unsigned NOT NULL DEFAULT 1, ADD UNIQUE INDEX uni_dashboard_user_tenant_id_id (tenant_id,id)`); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`ALTER TABLE mc_rbac_role ADD COLUMN dashboard_access_version bigint(20) unsigned NOT NULL DEFAULT 1, ADD UNIQUE INDEX uni_dashboard_role_tenant_id_id (tenant_id,id)`); err != nil {
			t.Fatal(err)
		}
		createDashboardRBACPartialTablesProbe(t, db)
		recoverDashboardRBACPartialState(t, db)
		assertDashboardRBACRemoved(t, db)
	})
}

func createDashboardRBACPartialTablesProbe(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, statement := range []string{
		`CREATE TABLE mochat_go_dashboard_permissions (id bigint unsigned NOT NULL PRIMARY KEY) ENGINE=InnoDB`,
		`CREATE TABLE mochat_go_dashboard_permission_resources (id bigint unsigned NOT NULL PRIMARY KEY) ENGINE=InnoDB`,
		`CREATE TABLE mochat_go_dashboard_user_roles (id bigint unsigned NOT NULL PRIMARY KEY) ENGINE=InnoDB`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
}

func newDashboardRBACMigrationDB(t *testing.T) *sql.DB {
	t.Helper()
	return newMigrationIntegrationDBThrough(t, "0126_phase3_final_providers")
}

func createDashboardRBACLegacyFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	statements := []string{
		`INSERT INTO mc_user (id,tenant_id,isSuperAdmin,deleted_at) VALUES (10,1,0,NULL)`,
		`INSERT INTO mc_rbac_role (id,tenant_id,operate_id,operate_name,data_permission,deleted_at) VALUES
			(20,1,10,'Fixture actor',NULL,NULL),
			(21,1,10,'Fixture actor','[{"corpId":1,"permissionType":2}]',NULL),
			(22,1,10,'Fixture actor','[{"corpId":1,"permissionType":1}]',NULL),
			(23,1,10,'Fixture actor','[{"corpId":1,"permissionType":1},{"corpId":2,"permissionType":2}]',NULL)`,
		`INSERT INTO mc_rbac_user_role (user_id,role_id,created_at,updated_at,deleted_at) VALUES (10,20,NOW(),NOW(),NULL)`,
		`INSERT INTO mc_rbac_menu (id,parent_id,link_url,data_permission,deleted_at) VALUES (300030,0,'/dashboard/channelCode/index#GET',1,NULL),(300031,0,'/dashboard/workContact/index@read',1,NULL),(300032,0,'/dashboard/user/index#GET',1,NULL),(300033,0,'/dashboard/channelCode/index#GET',2,NULL)`,
		`INSERT INTO mc_rbac_role_menu (role_id,menu_id,created_at,updated_at) VALUES (20,300030,NOW(),NOW()),(20,300031,NOW(),NOW()),(20,300032,NOW(),NOW()),(21,300030,NOW(),NOW()),(22,300033,NOW(),NOW()),(23,300030,NOW(),NOW())`,
		`INSERT INTO mochat_go_saas_tenant_packages (id,tenant_id,starts_at,expires_at,status,deleted_at) VALUES (1,1,NULL,NULL,1,NULL)`,
		`INSERT INTO mochat_go_saas_subscriptions (id,tenant_id,deleted_at) VALUES (1,1,NULL)`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("fixture statement failed: %v\n%s", err, statement)
		}
	}
}

func execDashboardRBACMigration(t *testing.T, db *sql.DB, name string, wantError bool) error {
	t.Helper()
	version := strings.TrimSuffix(strings.TrimSuffix(name, ".up.sql"), ".down.sql")
	runner := newMigrationRunnerThrough(t, db, version)
	var err error
	if strings.HasSuffix(name, ".down.sql") {
		_, err = runner.RollbackLast(context.Background())
	} else {
		_, err = runner.Apply(context.Background())
	}
	if wantError && err == nil {
		t.Fatalf("%s unexpectedly succeeded", name)
	}
	if !wantError && err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return err
}

func recoverDashboardRBACPartialState(t *testing.T, db *sql.DB) {
	t.Helper()
	target := DefaultMigrations(filepath.Join("..", ".."))
	for _, candidate := range target {
		if candidate.Version != "0127_dashboard_page_rbac" {
			continue
		}
		body, err := os.ReadFile(candidate.DownPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := execSQLScript(context.Background(), db, string(body)); err != nil {
			t.Fatalf("recover partial 0127 state: %v", err)
		}
		return
	}
	t.Fatal("production migration registry does not contain 0127_dashboard_page_rbac")
}

func assertDashboardRBACPreflightLeftNoDDL(t *testing.T, db *sql.DB) {
	t.Helper()
	if dashboardColumnExists(t, db, "mc_user", "dashboard_access_version") || dashboardTableExists(t, db, "mochat_go_dashboard_permissions") {
		t.Fatal("preflight failure left Dashboard RBAC DDL")
	}
}

func assertDashboardRBACRemoved(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, table := range []string{
		"mochat_go_dashboard_permissions", "mochat_go_dashboard_permission_resources",
		"mochat_go_dashboard_user_roles", "mochat_go_dashboard_role_permissions",
		"mochat_go_dashboard_user_permissions", "mochat_go_dashboard_permission_audits",
	} {
		if dashboardTableExists(t, db, table) {
			t.Fatalf("table %s still exists after down", table)
		}
	}
	if dashboardColumnExists(t, db, "mc_user", "dashboard_access_version") || dashboardColumnExists(t, db, "mc_rbac_role", "dashboard_access_version") {
		t.Fatal("dashboard_access_version still exists after down")
	}
	if dashboardIndexExists(t, db, "mc_user", "uni_dashboard_user_tenant_id_id") || dashboardIndexExists(t, db, "mc_rbac_role", "uni_dashboard_role_tenant_id_id") {
		t.Fatal("Dashboard RBAC composite index still exists after down")
	}
}

func dashboardTableExists(t *testing.T, db *sql.DB, table string) bool {
	t.Helper()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name=?`, table).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count == 1
}

func dashboardColumnExists(t *testing.T, db *sql.DB, table, column string) bool {
	t.Helper()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name=? AND column_name=?`, table, column).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count == 1
}

func dashboardIndexExists(t *testing.T, db *sql.DB, table, index string) bool {
	t.Helper()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name=? AND index_name=?`, table, index).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count > 0
}
