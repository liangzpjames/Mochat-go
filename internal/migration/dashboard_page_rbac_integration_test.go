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

var dashboardRBACSchemaSequence atomic.Int64

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
		execDashboardRBACMigration(t, db, "0127_dashboard_page_rbac.down.sql", false)
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
		for _, statement := range []string{
			`CREATE TABLE mochat_go_dashboard_permissions (id bigint unsigned NOT NULL PRIMARY KEY) ENGINE=InnoDB`,
			`CREATE TABLE mochat_go_dashboard_permission_resources (id bigint unsigned NOT NULL PRIMARY KEY) ENGINE=InnoDB`,
			`CREATE TABLE mochat_go_dashboard_user_roles (id bigint unsigned NOT NULL PRIMARY KEY) ENGINE=InnoDB`,
		} {
			if _, err := db.Exec(statement); err != nil {
				t.Fatal(err)
			}
		}
		execDashboardRBACMigration(t, db, "0127_dashboard_page_rbac.down.sql", false)
		assertDashboardRBACRemoved(t, db)
	})
}

func newDashboardRBACMigrationDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("MOCHAT_GO_MYSQL_INTEGRATION_DSN")
	if dsn == "" {
		t.Skip("MOCHAT_GO_MYSQL_INTEGRATION_DSN is required for Dashboard RBAC MariaDB migration tests")
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
	schema := fmt.Sprintf("mochat_dashboard_rbac_%d_%d", os.Getpid(), dashboardRBACSchemaSequence.Add(1))
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

func createDashboardRBACLegacyFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	statements := []string{
		`CREATE TABLE mc_user (id int(10) unsigned NOT NULL, tenant_id int(11) NOT NULL, deleted_at timestamp NULL, isSuperAdmin tinyint(1) DEFAULT 0, PRIMARY KEY(id)) ENGINE=InnoDB`,
		`CREATE TABLE mc_rbac_role (id int(11) NOT NULL, tenant_id int(11) NOT NULL, data_permission json DEFAULT NULL, deleted_at timestamp NULL, PRIMARY KEY(id)) ENGINE=InnoDB`,
		`CREATE TABLE mc_rbac_user_role (id int(11) NOT NULL AUTO_INCREMENT, user_id int(11) NOT NULL, role_id int(11) NOT NULL, created_at timestamp NULL, updated_at timestamp NULL, deleted_at timestamp NULL, PRIMARY KEY(id)) ENGINE=InnoDB`,
		`CREATE TABLE mc_rbac_menu (id int(11) NOT NULL, link_url varchar(255) NOT NULL, data_permission tinyint(1) NOT NULL DEFAULT 1, deleted_at timestamp NULL, PRIMARY KEY(id)) ENGINE=InnoDB`,
		`CREATE TABLE mc_rbac_role_menu (id int(11) NOT NULL AUTO_INCREMENT, role_id int(11) NOT NULL, menu_id int(11) NOT NULL, created_at timestamp NULL, updated_at timestamp NULL, PRIMARY KEY(id)) ENGINE=InnoDB`,
		`CREATE TABLE mochat_go_saas_tenant_packages (id int(10) unsigned NOT NULL, tenant_id int(10) unsigned NOT NULL, starts_at timestamp NULL, expires_at timestamp NULL, status tinyint(4) NOT NULL, deleted_at timestamp NULL, PRIMARY KEY(id)) ENGINE=InnoDB`,
		`CREATE TABLE mochat_go_saas_subscriptions (id bigint(20) unsigned NOT NULL, tenant_id int(10) unsigned NOT NULL, deleted_at timestamp NULL, PRIMARY KEY(id)) ENGINE=InnoDB`,
		`INSERT INTO mc_user (id,tenant_id,isSuperAdmin,deleted_at) VALUES (10,1,0,NULL)`,
		`INSERT INTO mc_rbac_role (id,tenant_id,data_permission,deleted_at) VALUES (20,1,NULL,NULL)`,
		`INSERT INTO mc_rbac_user_role (user_id,role_id,created_at,updated_at,deleted_at) VALUES (10,20,NOW(),NOW(),NULL)`,
		`INSERT INTO mc_rbac_menu (id,link_url,data_permission,deleted_at) VALUES (30,'/dashboard/channelCode/index#GET',1,NULL),(31,'/dashboard/workContact/index@read',1,NULL),(32,'/dashboard/user/index#GET',1,NULL)`,
		`INSERT INTO mc_rbac_role_menu (role_id,menu_id,created_at,updated_at) VALUES (20,30,NOW(),NOW()),(20,31,NOW(),NOW()),(20,32,NOW(),NOW())`,
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
