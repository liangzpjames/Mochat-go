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

const identitySingleCorpSchemaPrefix = "mochat_identity_single_corp_migration"

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
		"mochat_go_saas_idempotency_receipts",
		"mochat_go_tenant_corp_bindings",
		"information_schema.key_column_usage",
		"information_schema.statistics",
		"mochat_go_dashboard_user_roles",
		"identity_unknown_tenant_fk_count",
		"identity_unknown_tenant_index_count",
		"unknown tenant dependency",
		"0129 invalid mc_rbac_role tenant",
		"dashboard user-role relationship",
		"deferred to 0130",
		"SIGNAL SQLSTATE ''45000''",
		"PREPARE",
		"EXECUTE",
		"UNIQUE KEY `uni_dashboard_identity_login_identifier`",
		"UNIQUE KEY `uni_tenant_corp_binding_corp`",
		"bootstrap_request_key",
		"UNIQUE KEY `uni_saas_admin_user_bootstrap_request_key`",
	} {
		if !strings.Contains(normalizedUp, strings.ToLower(strings.ReplaceAll(required, "`", ""))) {
			t.Fatalf("0129 up migration missing %q", required)
		}
	}
	activationStart := strings.Index(normalizedUp, "create table if not exists mochat_go_dashboard_identity_activations")
	activationEnd := strings.Index(normalizedUp[activationStart+1:], "create table if not exists ")
	if activationStart < 0 || activationEnd < 0 || !strings.Contains(normalizedUp[activationStart:activationStart+activationEnd+1], "updated_at") {
		t.Fatal("0129 dashboard activation table must define updated_at for resend invalidation")
	}
	for _, foreignKey := range []string{
		"fk_dashboard_permission_resource_permission",
		"fk_dashboard_user_roles_user",
		"fk_dashboard_user_roles_role",
		"fk_dashboard_role_permissions_role",
		"fk_dashboard_role_permissions_permission",
		"fk_dashboard_user_permissions_user",
		"fk_dashboard_user_permissions_permission",
		"fk_dashboard_audit_actor",
	} {
		if !strings.Contains(normalizedUp, "drop foreign key "+foreignKey) || !strings.Contains(normalizedUp, "add constraint "+foreignKey) {
			t.Fatalf("0129 up migration must pair drop/add for %s", foreignKey)
		}
		if !strings.Contains(normalizedDown, "drop foreign key "+foreignKey) || !strings.Contains(normalizedDown, "add constraint "+foreignKey) {
			t.Fatalf("0129 down migration must pair drop/add for %s", foreignKey)
		}
	}
	for _, required := range []string{
		"bootstrap_request_key",
		"uni_saas_admin_user_bootstrap_request_key",
	} {
		if !strings.Contains(normalizedDown, strings.ToLower(strings.ReplaceAll(required, "`", ""))) {
			t.Fatalf("0129 down migration must explicitly account for %q", required)
		}
	}
	if strings.Contains(normalizedUp, "add constraint fk_saas_admin_user_access_identity") || strings.Contains(normalizedDown, "drop foreign key fk_saas_admin_user_access_identity") {
		t.Fatal("0129 must defer SaaS actor FK ownership to 0130")
	}
	if strings.Contains(normalizedUp, "select @identity_tenant_dependency_fk_count") || strings.Contains(normalizedUp, "select @identity_tenant_dependency_index_count") {
		t.Fatal("dependency facts must be validated, not merely selected")
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
		for _, preflight := range []string{"duplicate dashboard login identifier", "information_schema.key_column_usage", "information_schema.statistics", "unknown tenant dependency", "dashboard user-role relationship", "0129 invalid mc_rbac_role tenant"} {
			if offset := strings.Index(normalizedUp, strings.ToLower(strings.ReplaceAll(preflight, "`", ""))); offset < 0 || offset > firstDDL {
				t.Fatalf("preflight %q must precede first DDL", preflight)
			}
		}
	} else {
		t.Fatal("0129 migration has no DDL")
	}
}

func TestIdentityRealmsSingleCorpTenantDependencyIndexAllowlistContract(t *testing.T) {
	root := filepath.Join("..", "..")
	body, err := os.ReadFile(filepath.Join(root, "deploy", "standalone", "migrations", "0129_identity_realms_single_corp_schema.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	normalized := strings.ReplaceAll(strings.ToLower(string(body)), "`", "")
	known := "or (table_name = 'mc_corp' and index_name = 'idx_mc_corp_wecom_credential_key' and signature = 'wecom_credentials_key_id,tenant_id,id')"
	if !strings.Contains(normalized, known) {
		t.Fatalf("0129 allowlist missing legacy mc_corp credential index signature: %s", known)
	}
	if !strings.Contains(normalized, "0129 unknown tenant dependency index") {
		t.Fatal("0129 must retain the fail-closed unknown tenant dependency index guard")
	}
}

func TestIdentityRealmsSingleCorpIntegration(t *testing.T) {
	db := newIdentitySingleCorpMigrationDB(t)

	t.Run("apply down apply and enforce identity and binding constraints", func(t *testing.T) {
		createIdentitySingleCorpBaseFixture(t, db)
		for _, relation := range [][2]string{
			{"mochat_go_dashboard_permission_resources", "fk_dashboard_permission_resource_permission"},
			{"mochat_go_dashboard_user_roles", "fk_dashboard_user_roles_user"},
			{"mochat_go_dashboard_user_roles", "fk_dashboard_user_roles_role"},
			{"mochat_go_dashboard_role_permissions", "fk_dashboard_role_permissions_role"},
			{"mochat_go_dashboard_role_permissions", "fk_dashboard_role_permissions_permission"},
			{"mochat_go_dashboard_user_permissions", "fk_dashboard_user_permissions_user"},
			{"mochat_go_dashboard_user_permissions", "fk_dashboard_user_permissions_permission"},
			{"mochat_go_dashboard_permission_audits", "fk_dashboard_audit_actor"},
		} {
			dropIdentityForeignKey(t, db, relation[0], relation[1])
		}
		execIdentitySingleCorpMigration(t, db, "0129_identity_realms_single_corp_schema.up.sql", false)

		for _, table := range []string{
			"mochat_go_saas_admin_users",
			"mochat_go_dashboard_identities",
			"mochat_go_dashboard_identity_activations",
			"mochat_go_saas_idempotency_receipts",
			"mochat_go_tenant_corp_bindings",
			"mochat_go_dashboard_mfa_credentials",
			"mochat_go_dashboard_mfa_challenges",
			"mochat_go_dashboard_sessions",
			"mochat_go_dashboard_password_resets",
		} {
			assertIdentityTableExists(t, db, table)
		}
		assertIdentityColumnType(t, db, "mochat_go_dashboard_identity_activations", "updated_at", "timestamp")
		assertIdentityIndexExists(t, db, "mochat_go_dashboard_identities", "uni_dashboard_identity_login_identifier")
		assertIdentityIndexExists(t, db, "mochat_go_saas_admin_users", "uni_saas_admin_user_bootstrap_request_key")
		assertIdentityColumnType(t, db, "mochat_go_saas_admin_users", "bootstrap_request_key", "varchar(96)")
		assertIdentityIndexExists(t, db, "mochat_go_tenant_corp_bindings", "uni_tenant_corp_binding_corp")
		assertIdentity0127ForeignKeys(t, db)

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
		if _, err := db.Exec(`INSERT INTO mochat_go_saas_admin_user_access (user_id) VALUES (10)`); err != nil {
			t.Fatalf("0129 must defer dangling SaaS actor validation to 0130: %v", err)
		}
		assertIdentityForeignKeyMissing(t, db, "mochat_go_saas_admin_user_access", "fk_saas_admin_user_access_identity")

		execIdentitySingleCorpMigration(t, db, "0129_identity_realms_single_corp_schema.down.sql", false)
		for _, table := range []string{
			"mochat_go_saas_admin_users",
			"mochat_go_dashboard_identities",
			"mochat_go_dashboard_identity_activations",
			"mochat_go_saas_idempotency_receipts",
			"mochat_go_tenant_corp_bindings",
			"mochat_go_dashboard_mfa_credentials",
			"mochat_go_dashboard_mfa_challenges",
			"mochat_go_dashboard_sessions",
			"mochat_go_dashboard_password_resets",
		} {
			assertIdentityTableMissing(t, db, table)
		}
		assertIdentityColumnType(t, db, "mc_user", "tenant_id", "int(11)")
		assertIdentityColumnType(t, db, "mc_corp", "tenant_id", "int(11)")
		assertIdentityIndexMissing(t, db, "mc_corp", "uni_mc_corp_tenant_id_id")
		assertIdentity0127ForeignKeys(t, db)

		execIdentitySingleCorpMigration(t, db, "0129_identity_realms_single_corp_schema.up.sql", false)
		assertIdentityIndexExists(t, db, "mochat_go_saas_admin_users", "uni_saas_admin_user_bootstrap_request_key")
		assertIdentity0127ForeignKeys(t, db)
	})

	t.Run("preflight rejects duplicate login identifiers before DDL", func(t *testing.T) {
		db := newIdentitySingleCorpMigrationDB(t)
		createIdentitySingleCorpBaseFixture(t, db)
		if _, err := db.Exec(`INSERT INTO mc_user (id, tenant_id, phone, status, deleted_at) VALUES (12, 1, '13800000002', 1, NULL), (13, 1, '13800000002', 1, NULL)`); err != nil {
			t.Fatal(err)
		}
		err := execIdentitySingleCorpMigration(t, db, "0129_identity_realms_single_corp_schema.up.sql", true)
		if !strings.Contains(strings.ToLower(err.Error()), "duplicate dashboard login identifier") {
			t.Fatalf("error=%v", err)
		}
		assertIdentityTableMissing(t, db, "mochat_go_saas_admin_users")
		assertIdentityColumnType(t, db, "mc_user", "tenant_id", "int(11)")
	})

	t.Run("preflight rejects an unknown tenant dependency before DDL", func(t *testing.T) {
		db := newIdentitySingleCorpMigrationDB(t)
		createIdentitySingleCorpBaseFixture(t, db)
		if _, err := db.Exec(`CREATE TABLE identity_dependency_probe (tenant_id int(10) unsigned NOT NULL, CONSTRAINT fk_unknown_tenant_dependency FOREIGN KEY (tenant_id) REFERENCES mc_tenant (id)) ENGINE=InnoDB`); err != nil {
			t.Fatal(err)
		}
		err := execIdentitySingleCorpMigration(t, db, "0129_identity_realms_single_corp_schema.up.sql", true)
		if !strings.Contains(strings.ToLower(err.Error()), "unknown tenant dependency") {
			t.Fatalf("error=%v", err)
		}
		assertIdentityTableMissing(t, db, "mochat_go_saas_admin_users")
		assertIdentityColumnType(t, db, "mc_user", "tenant_id", "int(11)")
	})

	t.Run("preflight rejects an unknown tenant index before DDL", func(t *testing.T) {
		db := newIdentitySingleCorpMigrationDB(t)
		createIdentitySingleCorpBaseFixture(t, db)
		if _, err := db.Exec("ALTER TABLE mc_user ADD INDEX idx_unknown_tenant_dependency (tenant_id)"); err != nil {
			t.Fatal(err)
		}
		err := execIdentitySingleCorpMigration(t, db, "0129_identity_realms_single_corp_schema.up.sql", true)
		if !strings.Contains(strings.ToLower(err.Error()), "unknown tenant dependency index") {
			t.Fatalf("error=%v", err)
		}
		assertIdentityTableMissing(t, db, "mochat_go_saas_admin_users")
		assertIdentityColumnType(t, db, "mc_user", "tenant_id", "int(11)")
	})

	t.Run("fresh compose-like schema allows the legacy mc_corp credential index", func(t *testing.T) {
		db := newIdentitySingleCorpMigrationDB(t)
		createIdentitySingleCorpBaseFixture(t, db)
		if _, err := db.Exec("ALTER TABLE mc_corp ADD INDEX idx_mc_corp_wecom_credential_key (wecom_credentials_key_id, tenant_id, id)"); err != nil {
			t.Fatal(err)
		}
		execIdentitySingleCorpMigration(t, db, "0129_identity_realms_single_corp_schema.up.sql", false)
		assertIdentityTableExists(t, db, "mochat_go_saas_admin_users")
		assertIdentityIndexExists(t, db, "mc_corp", "idx_mc_corp_wecom_credential_key")
	})

	t.Run("preflight rejects an unknown mc_corp tenant index", func(t *testing.T) {
		db := newIdentitySingleCorpMigrationDB(t)
		createIdentitySingleCorpBaseFixture(t, db)
		if _, err := db.Exec("ALTER TABLE mc_corp ADD INDEX idx_unknown_tenant_dependency (tenant_id)"); err != nil {
			t.Fatal(err)
		}
		err := execIdentitySingleCorpMigration(t, db, "0129_identity_realms_single_corp_schema.up.sql", true)
		if !strings.Contains(strings.ToLower(err.Error()), "unknown tenant dependency index") {
			t.Fatalf("error=%v", err)
		}
		assertIdentityTableMissing(t, db, "mochat_go_saas_admin_users")
	})

	t.Run("preflight rejects dangling known relation when its FK was removed", func(t *testing.T) {
		db := newIdentitySingleCorpMigrationDB(t)
		createIdentitySingleCorpBaseFixture(t, db)
		dropIdentityForeignKey(t, db, "mochat_go_dashboard_user_roles", "fk_dashboard_user_roles_user")
		if _, err := db.Exec(`INSERT INTO mochat_go_dashboard_user_roles (tenant_id, user_id, role_id) VALUES (1, 999, 20)`); err != nil {
			t.Fatal(err)
		}
		err := execIdentitySingleCorpMigration(t, db, "0129_identity_realms_single_corp_schema.up.sql", true)
		if !strings.Contains(strings.ToLower(err.Error()), "dashboard user-role relationship") {
			t.Fatalf("error=%v", err)
		}
		assertIdentityTableMissing(t, db, "mochat_go_saas_admin_users")
		assertIdentityColumnType(t, db, "mochat_go_dashboard_user_roles", "tenant_id", "int(11)")
	})

	t.Run("preflight rejects negative role tenant before DDL", func(t *testing.T) {
		db := newIdentitySingleCorpMigrationDB(t)
		createIdentitySingleCorpBaseFixture(t, db)
		dropIdentityForeignKey(t, db, "mochat_go_dashboard_user_roles", "fk_dashboard_user_roles_role")
		dropIdentityForeignKey(t, db, "mochat_go_dashboard_role_permissions", "fk_dashboard_role_permissions_role")
		if _, err := db.Exec(`UPDATE mc_rbac_role SET tenant_id = -1 WHERE id = 20`); err != nil {
			t.Fatal(err)
		}
		err := execIdentitySingleCorpMigration(t, db, "0129_identity_realms_single_corp_schema.up.sql", true)
		if !strings.Contains(strings.ToLower(err.Error()), "invalid mc_rbac_role tenant") {
			t.Fatalf("error=%v", err)
		}
		assertIdentityTableMissing(t, db, "mochat_go_saas_admin_users")
		assertIdentityColumnType(t, db, "mc_rbac_role", "tenant_id", "int(11)")
	})

	t.Run("down tolerates partial DDL", func(t *testing.T) {
		db := newIdentitySingleCorpMigrationDB(t)
		createIdentitySingleCorpBaseFixture(t, db)
		dropIdentityForeignKey(t, db, "mochat_go_dashboard_user_roles", "fk_dashboard_user_roles_user")
		dropIdentityForeignKey(t, db, "mochat_go_dashboard_user_permissions", "fk_dashboard_user_permissions_user")
		dropIdentityForeignKey(t, db, "mochat_go_dashboard_permission_audits", "fk_dashboard_audit_actor")
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
	schema := fmt.Sprintf("%s_%d_%d", identitySingleCorpSchemaPrefix, os.Getpid(), identitySingleCorpSchemaSequence.Add(1))
	var schemaExists int
	if err := admin.QueryRow("SELECT COUNT(*) FROM information_schema.schemata WHERE schema_name = ?", schema).Scan(&schemaExists); err != nil {
		_ = admin.Close()
		t.Fatalf("check isolated schema collision: %v", err)
	}
	if schemaExists != 0 {
		_ = admin.Close()
		t.Fatalf("isolated schema already exists: %s", schema)
	}
	if _, err := admin.Exec("CREATE DATABASE `" + schema + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
		_ = admin.Close()
		t.Fatalf("create isolated migration schema: %v", err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec("DROP DATABASE IF EXISTS `" + schema + "`")
		var leftovers int
		if err := admin.QueryRow("SELECT COUNT(*) FROM information_schema.schemata WHERE schema_name = ?", schema).Scan(&leftovers); err != nil {
			t.Errorf("check isolated schema cleanup: %v", err)
		} else if leftovers != 0 {
			t.Errorf("isolated schema %s still exists after cleanup", schema)
		}
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
		`CREATE TABLE mc_tenant (id int(10) unsigned NOT NULL AUTO_INCREMENT, name varchar(255) NOT NULL DEFAULT '', status tinyint NOT NULL DEFAULT 1, deleted_at timestamp NULL, PRIMARY KEY (id)) ENGINE=InnoDB`,
		`CREATE TABLE mc_corp (id int(10) unsigned NOT NULL AUTO_INCREMENT, tenant_id int(11) DEFAULT 0, name varchar(255) NOT NULL DEFAULT '', wx_corpid varchar(255) NOT NULL DEFAULT '', employee_secret varchar(255) NOT NULL DEFAULT '', contact_secret varchar(255) NOT NULL DEFAULT '', token varchar(255) NOT NULL DEFAULT '', encoding_aes_key varchar(255) NOT NULL DEFAULT '', chat_secret varchar(255) NOT NULL DEFAULT '', wecom_credentials_ciphertext text COLLATE utf8mb4_bin NULL, wecom_credentials_key_id varchar(64) NOT NULL DEFAULT '', created_at timestamp NULL, updated_at timestamp NULL, deleted_at timestamp NULL, PRIMARY KEY (id)) ENGINE=InnoDB`,
		`CREATE TABLE mc_user (id int(10) unsigned NOT NULL AUTO_INCREMENT, tenant_id int(11) NOT NULL DEFAULT 1, phone char(11) NOT NULL DEFAULT '', password varchar(255) NOT NULL DEFAULT '', name varchar(255) NOT NULL DEFAULT '', status tinyint unsigned NOT NULL DEFAULT 1, deleted_at timestamp NULL, isSuperAdmin tinyint NOT NULL DEFAULT 0, PRIMARY KEY (id)) ENGINE=InnoDB`,
		`CREATE TABLE mc_rbac_role (id int(11) NOT NULL AUTO_INCREMENT, tenant_id int(11) NOT NULL, data_permission json DEFAULT NULL, deleted_at timestamp NULL, PRIMARY KEY (id)) ENGINE=InnoDB`,
		`CREATE TABLE mc_rbac_user_role (id int(11) NOT NULL AUTO_INCREMENT, user_id int(11) NOT NULL, role_id int(11) NOT NULL, deleted_at timestamp NULL, PRIMARY KEY (id)) ENGINE=InnoDB`,
		`CREATE TABLE mc_work_agent (id int(10) unsigned NOT NULL AUTO_INCREMENT, corp_id int(11) NOT NULL, wx_agent_id varchar(255) NOT NULL DEFAULT '', wx_secret varchar(255) NOT NULL DEFAULT '', wecom_credentials_ciphertext text COLLATE utf8mb4_bin NULL, wecom_credentials_key_id varchar(64) NOT NULL DEFAULT '', PRIMARY KEY (id)) ENGINE=InnoDB`,
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
	loadRealDashboardPageRBACDDL(t, db)
	loadSaaSAdminRBACDDL(t, db)
	loadSaaSAdminHistoryDDL(t, db)
	for _, statement := range []string{
		`INSERT INTO mochat_go_dashboard_permissions (id, code, permission_type, path, name) VALUES (900, 'dashboard.test', 'page', '/test', 'Test')`,
		`INSERT INTO mochat_go_dashboard_permission_resources (id, permission_id, resource_type, http_method, path_pattern) VALUES (901, 900, 'api', 'GET', '/dashboard/test')`,
		`INSERT INTO mochat_go_dashboard_user_roles (tenant_id, user_id, role_id) VALUES (1, 10, 20)`,
		`INSERT INTO mochat_go_dashboard_role_permissions (tenant_id, role_id, permission_id) VALUES (1, 20, 900)`,
		`INSERT INTO mochat_go_dashboard_user_permissions (tenant_id, user_id, permission_id) VALUES (1, 10, 900)`,
		`INSERT INTO mochat_go_dashboard_permission_audits (tenant_id, actor_user_id, action, target_type, target_id) VALUES (1, 10, 'test', 'user', '10')`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("0127 complete fixture statement failed: %v", err)
		}
	}
}

func loadSaaSAdminRBACDDL(t *testing.T, db *sql.DB) {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("..", "..", "deploy", "standalone", "migrations", "0045_saas_admin_rbac.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if err := execSQLScript(context.Background(), db, string(body)); err != nil {
		t.Fatalf("load real SaaS RBAC DDL: %v", err)
	}
}

func loadSaaSAdminHistoryDDL(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, name := range []string{
		"0033_saas_admin_operation_logs.up.sql",
		"0035_saas_admin_tasks.up.sql",
		"0046_saas_admin_approvals.up.sql",
		"0047_saas_admin_approval_governance.up.sql",
		"0048_saas_admin_system_health.up.sql",
		"0062_saas_audit_integrity.up.sql",
		"0063_saas_audit_anchor_signatures.up.sql",
	} {
		body, err := os.ReadFile(filepath.Join("..", "..", "deploy", "standalone", "migrations", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := execSQLScript(context.Background(), db, string(body)); err != nil {
			t.Fatalf("load SaaS admin history DDL %s: %v", name, err)
		}
	}
}

func loadRealDashboardPageRBACDDL(t *testing.T, db *sql.DB) {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("..", "..", "deploy", "standalone", "migrations", "0127_dashboard_page_rbac.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	script := string(body)
	start := strings.Index(script, "ALTER TABLE `mc_user`")
	end := strings.Index(script, "INSERT INTO `mochat_go_dashboard_permissions`")
	if start < 0 || end <= start {
		t.Fatal("0127 DDL boundaries not found")
	}
	if err := execSQLScript(context.Background(), db, script[start:end]); err != nil {
		t.Fatalf("load real 0127 DDL: %v", err)
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

func dropIdentityForeignKey(t *testing.T, db *sql.DB, table, foreignKey string) {
	t.Helper()
	if _, err := db.Exec("ALTER TABLE `" + table + "` DROP FOREIGN KEY `" + foreignKey + "`"); err != nil {
		t.Fatalf("drop %s.%s: %v", table, foreignKey, err)
	}
}

func assertIdentityForeignKeyExists(t *testing.T, db *sql.DB, table, foreignKey string) {
	t.Helper()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.table_constraints WHERE constraint_schema=DATABASE() AND table_name=? AND constraint_name=? AND constraint_type='FOREIGN KEY'`, table, foreignKey).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("foreign key %s.%s exists=%d", table, foreignKey, count)
	}
}

func assertIdentityForeignKeyMissing(t *testing.T, db *sql.DB, table, foreignKey string) {
	t.Helper()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.table_constraints WHERE constraint_schema=DATABASE() AND table_name=? AND constraint_name=? AND constraint_type='FOREIGN KEY'`, table, foreignKey).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("foreign key %s.%s still exists", table, foreignKey)
	}
}

func assertIdentity0127ForeignKeys(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, relation := range [][2]string{
		{"mochat_go_dashboard_permission_resources", "fk_dashboard_permission_resource_permission"},
		{"mochat_go_dashboard_user_roles", "fk_dashboard_user_roles_user"},
		{"mochat_go_dashboard_user_roles", "fk_dashboard_user_roles_role"},
		{"mochat_go_dashboard_role_permissions", "fk_dashboard_role_permissions_role"},
		{"mochat_go_dashboard_role_permissions", "fk_dashboard_role_permissions_permission"},
		{"mochat_go_dashboard_user_permissions", "fk_dashboard_user_permissions_user"},
		{"mochat_go_dashboard_user_permissions", "fk_dashboard_user_permissions_permission"},
		{"mochat_go_dashboard_permission_audits", "fk_dashboard_audit_actor"},
	} {
		assertIdentityForeignKeyExists(t, db, relation[0], relation[1])
	}
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
