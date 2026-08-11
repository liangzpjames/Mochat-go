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

var identitySingleCorpSchemaSequence atomic.Int64

func TestIdentityRealmsSingleCorpMigrationContract(t *testing.T) {
	root := filepath.Join("..", "..")
	upPath := filepath.Join(root, "deploy", "standalone", "migrations", "0129_identity_realms_single_corp_schema.up.sql")
	downPath := filepath.Join(root, "deploy", "standalone", "migrations", "0129_identity_realms_single_corp_schema.down.sql")
	upBody, err := os.ReadFile(upPath)
	if err != nil {
		t.Fatal(err)
	}
	downBody, err := os.ReadFile(downPath)
	if err != nil {
		t.Fatal(err)
	}
	up := string(upBody)
	down := string(downBody)
	normalizedUp := strings.ReplaceAll(strings.ToLower(up), "`", "")
	normalizedDown := strings.ReplaceAll(strings.ToLower(down), "`", "")
	for _, required := range []string{
		"mochat_go_saas_admin_users",
		"mochat_go_dashboard_identities",
		"mochat_go_dashboard_identity_activations",
		"mochat_go_tenant_corp_bindings",
		"information_schema.key_column_usage",
		"information_schema.statistics",
		"mochat_go_dashboard_user_roles",
		"SIGNAL SQLSTATE ''45000''",
		"PREPARE",
		"EXECUTE",
		"UNIQUE KEY `uni_dashboard_identity_login_identifier`",
		"UNIQUE KEY `uni_tenant_corp_binding_corp`",
	} {
		if !strings.Contains(normalizedUp, strings.ToLower(strings.ReplaceAll(required, "`", ""))) {
			t.Fatalf("0129 up migration missing %q", required)
		}
	}
	for _, required := range []string{
		"DROP TABLE IF EXISTS `mochat_go_tenant_corp_bindings`",
		"DROP TABLE IF EXISTS `mochat_go_dashboard_identity_activations`",
		"DROP TABLE IF EXISTS `mochat_go_dashboard_identities`",
		"DROP TABLE IF EXISTS `mochat_go_saas_admin_users`",
		"information_schema.columns",
		"DROP INDEX",
	} {
		if !strings.Contains(normalizedDown, strings.ToLower(strings.ReplaceAll(required, "`", ""))) {
			t.Fatalf("0129 down migration missing %q", required)
		}
	}
	if strings.Contains(strings.ToUpper(up), "DELIMITER") || strings.Contains(strings.ToUpper(up), "CREATE PROCEDURE") {
		t.Fatal("0129 migration must not use DELIMITER or stored procedures")
	}
	if firstDDL := firstIdentitySchemaDDL(up); firstDDL >= 0 {
		for _, preflight := range []string{"duplicate dashboard login identifier", "information_schema.key_column_usage", "information_schema.statistics"} {
			if offset := strings.Index(normalizedUp, strings.ToLower(strings.ReplaceAll(preflight, "`", ""))); offset < 0 || offset > firstDDL {
				t.Fatalf("preflight %q must precede first DDL", preflight)
			}
		}
	} else {
		t.Fatal("0129 migration has no DDL")
	}
}

func TestIdentityRealmsSingleCorpIntegration(t *testing.T) {
	db := newIdentitySingleCorpMigrationDB(t)

	t.Run("apply down apply and enforce identity and binding constraints", func(t *testing.T) {
		createIdentitySingleCorpBaseFixture(t, db)
		execIdentitySingleCorpMigration(t, db, "0129_identity_realms_single_corp_schema.up.sql", false)

		for _, table := range []string{
			"mochat_go_saas_admin_users",
			"mochat_go_dashboard_identities",
			"mochat_go_dashboard_identity_activations",
			"mochat_go_tenant_corp_bindings",
		} {
			assertIdentityTableExists(t, db, table)
		}
		assertIdentityIndexExists(t, db, "mochat_go_dashboard_identities", "uni_dashboard_identity_login_identifier")
		assertIdentityIndexExists(t, db, "mochat_go_tenant_corp_bindings", "uni_tenant_corp_binding_corp")

		if _, err := db.Exec(`INSERT INTO mochat_go_saas_admin_users (login_name, password_hash, name) VALUES ('platform', 'hash', 'Platform')`); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO mochat_go_dashboard_identities (user_id, login_identifier, password_hash) VALUES (10, '13800000001', 'hash')`); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO mochat_go_tenant_corp_bindings (tenant_id, corp_id, status, verified_corp_name) VALUES (1, 100, 1, '')`); err != nil {
			t.Fatal(err)
		}
		assertExecFails(t, db, `INSERT INTO mochat_go_dashboard_identities (user_id, login_identifier, password_hash) VALUES (11, '13800000001', 'hash')`)
		assertExecFails(t, db, `INSERT INTO mochat_go_tenant_corp_bindings (tenant_id, corp_id, status, verified_corp_name) VALUES (1, 101, 1, '')`)
		assertExecFails(t, db, `INSERT INTO mochat_go_tenant_corp_bindings (tenant_id, corp_id, status, verified_corp_name) VALUES (2, 100, 1, '')`)
		assertExecFails(t, db, `INSERT INTO mochat_go_tenant_corp_bindings (tenant_id, corp_id, status, verified_corp_name) VALUES (1, 200, 1, '')`)
		assertExecFails(t, db, `INSERT INTO mochat_go_saas_admin_user_access (user_id) VALUES (10)`)

		execIdentitySingleCorpMigration(t, db, "0129_identity_realms_single_corp_schema.down.sql", false)
		for _, table := range []string{
			"mochat_go_saas_admin_users",
			"mochat_go_dashboard_identities",
			"mochat_go_dashboard_identity_activations",
			"mochat_go_tenant_corp_bindings",
		} {
			assertIdentityTableMissing(t, db, table)
		}
		assertIdentityColumnType(t, db, "mc_user", "tenant_id", "int(11)")
		assertIdentityColumnType(t, db, "mc_corp", "tenant_id", "int(11)")
		assertIdentityIndexMissing(t, db, "mc_corp", "uni_mc_corp_tenant_id_id")

		execIdentitySingleCorpMigration(t, db, "0129_identity_realms_single_corp_schema.up.sql", false)
	})

	t.Run("preflight rejects duplicate login identifiers before DDL", func(t *testing.T) {
		db := newIdentitySingleCorpMigrationDB(t)
		createIdentitySingleCorpBaseFixture(t, db)
		if _, err := db.Exec(`INSERT INTO mc_user (id, tenant_id, phone, status, deleted_at) VALUES (10, 1, '13800000002', 1, NULL), (11, 1, '13800000002', 1, NULL)`); err != nil {
			t.Fatal(err)
		}
		err := execIdentitySingleCorpMigration(t, db, "0129_identity_realms_single_corp_schema.up.sql", true)
		if !strings.Contains(strings.ToLower(err.Error()), "duplicate dashboard login identifier") {
			t.Fatalf("error=%v", err)
		}
		assertIdentityTableMissing(t, db, "mochat_go_saas_admin_users")
		assertIdentityColumnType(t, db, "mc_user", "tenant_id", "int(11)")
	})

	t.Run("down tolerates partial DDL", func(t *testing.T) {
		db := newIdentitySingleCorpMigrationDB(t)
		createIdentitySingleCorpBaseFixture(t, db)
		if _, err := db.Exec(`ALTER TABLE mc_user MODIFY tenant_id int(10) unsigned NOT NULL DEFAULT 1`); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`CREATE TABLE mochat_go_dashboard_identities (user_id int(10) unsigned NOT NULL PRIMARY KEY) ENGINE=InnoDB`); err != nil {
			t.Fatal(err)
		}
		execIdentitySingleCorpMigration(t, db, "0129_identity_realms_single_corp_schema.down.sql", false)
		assertIdentityTableMissing(t, db, "mochat_go_dashboard_identities")
		assertIdentityColumnType(t, db, "mc_user", "tenant_id", "int(11)")
	})
}

func newIdentitySingleCorpMigrationDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("MOCHAT_GO_MYSQL_INTEGRATION_DSN")
	if dsn == "" {
		t.Skip("MOCHAT_GO_MYSQL_INTEGRATION_DSN is required for Identity Realms Single Corp MariaDB migration tests")
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
	schema := fmt.Sprintf("mochat_identity_single_corp_%d_%d", os.Getpid(), identitySingleCorpSchemaSequence.Add(1))
	if _, err := admin.Exec("CREATE DATABASE `" + schema + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
		_ = admin.Close()
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
		_ = db.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func createIdentitySingleCorpBaseFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	statements := []string{
		`CREATE TABLE mc_tenant (id int(10) unsigned NOT NULL AUTO_INCREMENT, name varchar(255) NOT NULL DEFAULT '', status tinyint NOT NULL DEFAULT 1, PRIMARY KEY (id)) ENGINE=InnoDB`,
		`CREATE TABLE mc_corp (id int(10) unsigned NOT NULL AUTO_INCREMENT, tenant_id int(11) DEFAULT 0, name varchar(255) NOT NULL DEFAULT '', wx_corpid varchar(255) NOT NULL DEFAULT '', employee_secret varchar(255) NOT NULL DEFAULT '', contact_secret varchar(255) NOT NULL DEFAULT '', token varchar(255) NOT NULL DEFAULT '', encoding_aes_key varchar(255) NOT NULL DEFAULT '', PRIMARY KEY (id)) ENGINE=InnoDB`,
		`CREATE TABLE mc_user (id int(10) unsigned NOT NULL AUTO_INCREMENT, tenant_id int(11) NOT NULL DEFAULT 1, phone char(11) NOT NULL DEFAULT '', password varchar(255) NOT NULL DEFAULT '', name varchar(255) NOT NULL DEFAULT '', status tinyint unsigned NOT NULL DEFAULT 1, deleted_at timestamp NULL, isSuperAdmin tinyint NOT NULL DEFAULT 0, PRIMARY KEY (id)) ENGINE=InnoDB`,
		`CREATE TABLE mc_rbac_role (id int(11) NOT NULL AUTO_INCREMENT, tenant_id int(11) NOT NULL, data_permission json DEFAULT NULL, deleted_at timestamp NULL, PRIMARY KEY (id)) ENGINE=InnoDB`,
		`CREATE TABLE mc_rbac_user_role (id int(11) NOT NULL AUTO_INCREMENT, user_id int(11) NOT NULL, role_id int(11) NOT NULL, deleted_at timestamp NULL, PRIMARY KEY (id)) ENGINE=InnoDB`,
		`CREATE TABLE mochat_go_dashboard_permissions (id bigint unsigned NOT NULL AUTO_INCREMENT, PRIMARY KEY (id)) ENGINE=InnoDB`,
		`CREATE TABLE mochat_go_dashboard_permission_resources (id bigint unsigned NOT NULL AUTO_INCREMENT, permission_id bigint unsigned NOT NULL, PRIMARY KEY (id)) ENGINE=InnoDB`,
		`CREATE TABLE mochat_go_dashboard_user_roles (id bigint unsigned NOT NULL AUTO_INCREMENT, tenant_id int(11) NOT NULL, user_id int(10) unsigned NOT NULL, role_id int(11) NOT NULL, PRIMARY KEY (id)) ENGINE=InnoDB`,
		`CREATE TABLE mochat_go_dashboard_role_permissions (id bigint unsigned NOT NULL AUTO_INCREMENT, tenant_id int(11) NOT NULL, role_id int(11) NOT NULL, permission_id bigint unsigned NOT NULL, PRIMARY KEY (id)) ENGINE=InnoDB`,
		`CREATE TABLE mochat_go_dashboard_user_permissions (id bigint unsigned NOT NULL AUTO_INCREMENT, tenant_id int(11) NOT NULL, user_id int(10) unsigned NOT NULL, permission_id bigint unsigned NOT NULL, PRIMARY KEY (id)) ENGINE=InnoDB`,
		`CREATE TABLE mochat_go_dashboard_permission_audits (id bigint unsigned NOT NULL AUTO_INCREMENT, tenant_id int(11) NOT NULL, actor_user_id int(10) unsigned NULL, PRIMARY KEY (id)) ENGINE=InnoDB`,
		`CREATE TABLE mochat_go_saas_admin_user_access (id bigint unsigned NOT NULL AUTO_INCREMENT, user_id int(10) unsigned NOT NULL, PRIMARY KEY (id), UNIQUE KEY uni_mochat_go_saas_admin_user_access_user (user_id)) ENGINE=InnoDB`,
		`INSERT INTO mc_tenant (id, name, status) VALUES (1, 'Tenant 1', 1), (2, 'Tenant 2', 1)`,
		`INSERT INTO mc_corp (id, tenant_id, name) VALUES (100, 1, 'Corp 1'), (200, 2, 'Corp 2')`,
		`INSERT INTO mc_user (id, tenant_id, phone, status, deleted_at) VALUES (10, 1, '13800000001', 1, NULL)`,
		`INSERT INTO mc_rbac_role (id, tenant_id) VALUES (20, 1)`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("fixture statement failed: %v", err)
		}
	}
}

func execIdentitySingleCorpMigration(t *testing.T, db *sql.DB, name string, wantError bool) error {
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

func firstIdentitySchemaDDL(sqlText string) int {
	positions := []int{}
	for _, token := range []string{"ALTER TABLE", "CREATE TABLE", "CREATE UNIQUE INDEX"} {
		if position := strings.Index(strings.ToUpper(sqlText), token); position >= 0 {
			positions = append(positions, position)
		}
	}
	if len(positions) == 0 {
		return -1
	}
	first := positions[0]
	for _, position := range positions[1:] {
		if position < first {
			first = position
		}
	}
	return first
}

func assertExecFails(t *testing.T, db *sql.DB, query string) {
	t.Helper()
	if _, err := db.Exec(query); err == nil {
		t.Fatalf("expected constraint failure")
	}
}

func assertIdentityTableExists(t *testing.T, db *sql.DB, table string) {
	t.Helper()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name=?`, table).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("table %s exists=%d", table, count)
	}
}

func assertIdentityTableMissing(t *testing.T, db *sql.DB, table string) {
	t.Helper()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name=?`, table).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("table %s still exists", table)
	}
}

func assertIdentityIndexExists(t *testing.T, db *sql.DB, table, index string) {
	t.Helper()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name=? AND index_name=?`, table, index).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count == 0 {
		t.Fatalf("index %s.%s missing", table, index)
	}
}

func assertIdentityIndexMissing(t *testing.T, db *sql.DB, table, index string) {
	t.Helper()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name=? AND index_name=?`, table, index).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("index %s.%s still exists", table, index)
	}
}

func assertIdentityColumnType(t *testing.T, db *sql.DB, table, column, want string) {
	t.Helper()
	var columnType string
	if err := db.QueryRow(`SELECT column_type FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name=? AND column_name=?`, table, column).Scan(&columnType); err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(columnType, want) {
		t.Fatalf("%s.%s type=%q want=%q", table, column, columnType, want)
	}
}
