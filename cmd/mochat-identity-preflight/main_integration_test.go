package main

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
	"jiyi/mochat-go/internal/migration"
)

var preflightIntegrationSchemaSequence atomic.Int64

func TestRunPreflightRealMariaDBIsReadOnlyAndReportsHealthyCounts(t *testing.T) {
	db, dsn, schema := newPreflightIntegrationDB(t)
	createPreflightIntegrationFixture(t, db)
	execPreflightIntegrationFile(t, db, "0129_identity_realms_single_corp_schema.up.sql")

	dir := t.TempDir()
	dsnFile := filepath.Join(dir, "dsn")
	keyFile := filepath.Join(dir, "credential-key")
	if err := os.WriteFile(dsnFile, []byte(dsn), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyFile, []byte(strings.Repeat("01", 32)), 0o600); err != nil {
		t.Fatal(err)
	}
	var before int
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name='mochat_go_identity_migration_batches'`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	var output strings.Builder
	args := []string{"--dsn-file", dsnFile, "--schema", schema, "--platform-tenant-id", "1", "--credential-key-file", keyFile, "--credential-key-id", "task8-wecom-v1"}
	if err := runPreflight(args, &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "active_dashboard_users=") || strings.Contains(output.String(), strings.Repeat("01", 32)) {
		t.Fatalf("preflight output=%q, want safe counts without key material", output.String())
	}
	var after int
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name='mochat_go_identity_migration_batches'`).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if before != after || after != 0 {
		t.Fatalf("preflight changed staging table count from %d to %d", before, after)
	}
}

func newPreflightIntegrationDB(t *testing.T) (*sql.DB, string, string) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("MOCHAT_GO_MYSQL_INTEGRATION_DSN"))
	if dsn == "" {
		t.Skip("MOCHAT_GO_MYSQL_INTEGRATION_DSN is required for preflight MariaDB integration")
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
	var baseline int
	if err := admin.QueryRow(`SELECT COUNT(*) FROM information_schema.schemata WHERE schema_name LIKE 'mochat_identity_single_corp_%'`).Scan(&baseline); err != nil {
		_ = admin.Close()
		t.Fatal(err)
	}
	schema := fmt.Sprintf("mochat_identity_single_corp_%d_%d", os.Getpid(), preflightIntegrationSchemaSequence.Add(1))
	if _, err := admin.Exec("CREATE DATABASE `" + schema + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
		_ = admin.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec("DROP DATABASE IF EXISTS `" + schema + "`")
		var leftovers int
		if err := admin.QueryRow(`SELECT COUNT(*) FROM information_schema.schemata WHERE schema_name LIKE 'mochat_identity_single_corp_%'`).Scan(&leftovers); err != nil {
			t.Errorf("check isolated schema leftovers: %v", err)
		} else if leftovers != baseline {
			t.Errorf("isolated schema leftovers changed from baseline=%d to %d", baseline, leftovers)
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
	return db, dsn, schema
}

func createPreflightIntegrationFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, statement := range []string{
		`CREATE TABLE mc_tenant (id int(10) unsigned NOT NULL AUTO_INCREMENT, name varchar(255) NOT NULL DEFAULT '', status tinyint NOT NULL DEFAULT 1, deleted_at timestamp NULL, PRIMARY KEY (id)) ENGINE=InnoDB`,
		`CREATE TABLE mc_corp (id int(10) unsigned NOT NULL AUTO_INCREMENT, tenant_id int(11) DEFAULT 0, name varchar(255) NOT NULL DEFAULT '', wx_corpid varchar(255) NOT NULL DEFAULT '', employee_secret varchar(255) NOT NULL DEFAULT '', contact_secret varchar(255) NOT NULL DEFAULT '', token varchar(255) NOT NULL DEFAULT '', encoding_aes_key varchar(255) NOT NULL DEFAULT '', chat_secret varchar(255) NOT NULL DEFAULT '', wecom_credentials_ciphertext text NULL, wecom_credentials_key_id varchar(64) NOT NULL DEFAULT '', deleted_at timestamp NULL, PRIMARY KEY (id)) ENGINE=InnoDB`,
		`CREATE TABLE mc_user (id int(10) unsigned NOT NULL AUTO_INCREMENT, tenant_id int(11) NOT NULL DEFAULT 1, phone char(11) NOT NULL DEFAULT '', password varchar(255) NOT NULL DEFAULT '', name varchar(255) NOT NULL DEFAULT '', status tinyint unsigned NOT NULL DEFAULT 1, deleted_at timestamp NULL, isSuperAdmin tinyint NOT NULL DEFAULT 0, PRIMARY KEY (id)) ENGINE=InnoDB`,
		`CREATE TABLE mc_rbac_role (id int(11) NOT NULL AUTO_INCREMENT, tenant_id int(11) NOT NULL, data_permission json DEFAULT NULL, deleted_at timestamp NULL, PRIMARY KEY (id)) ENGINE=InnoDB`,
		`CREATE TABLE mc_rbac_user_role (id int(11) NOT NULL AUTO_INCREMENT, user_id int(11) NOT NULL, role_id int(11) NOT NULL, deleted_at timestamp NULL, PRIMARY KEY (id)) ENGINE=InnoDB`,
		`CREATE TABLE mc_work_agent (id int(10) unsigned NOT NULL AUTO_INCREMENT, corp_id int(11) NOT NULL, wx_agent_id varchar(255) NOT NULL DEFAULT '', wx_secret varchar(255) NOT NULL DEFAULT '', wecom_credentials_ciphertext text NULL, wecom_credentials_key_id varchar(64) NOT NULL DEFAULT '', PRIMARY KEY (id)) ENGINE=InnoDB`,
		`INSERT INTO mc_tenant (id, name, status) VALUES (1, 'Tenant 1', 1), (2, 'Tenant 2', 1)`,
		`INSERT INTO mc_corp (id, tenant_id, name) VALUES (100, 1, 'Corp 1'), (200, 2, 'Corp 2')`,
		`INSERT INTO mc_work_agent (id, corp_id, wx_agent_id, wx_secret) VALUES (300, 100, 'agent-300', '')`,
		`INSERT INTO mc_user (id, tenant_id, phone, password, name, status, deleted_at, isSuperAdmin) VALUES (11, 1, '13800000011', 'legacy-platform-hash', 'Platform admin', 1, NULL, 1), (13, 2, '13800000013', 'legacy-dashboard-hash', 'Dashboard user', 1, NULL, 0)`,
		`INSERT INTO mc_rbac_role (id, tenant_id) VALUES (20, 1)`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	loadPreflightIntegrationDDL(t, db)
	for _, statement := range []string{
		`INSERT INTO mochat_go_dashboard_permissions (id, code, permission_type, path, name) VALUES (900, 'dashboard.test', 'page', '/test', 'Test')`,
		`INSERT INTO mochat_go_dashboard_permission_resources (id, permission_id, resource_type, http_method, path_pattern) VALUES (901, 900, 'api', 'GET', '/dashboard/test')`,
		`INSERT INTO mochat_go_dashboard_user_roles (tenant_id, user_id, role_id) VALUES (1, 11, 20)`,
		`INSERT INTO mochat_go_dashboard_role_permissions (tenant_id, role_id, permission_id) VALUES (1, 20, 900)`,
		`INSERT INTO mochat_go_dashboard_user_permissions (tenant_id, user_id, permission_id) VALUES (1, 11, 900)`,
		`INSERT INTO mochat_go_dashboard_permission_audits (tenant_id, actor_user_id, action, target_type, target_id) VALUES (1, 11, 'test', 'user', '11')`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
}

func loadPreflightIntegrationDDL(t *testing.T, db *sql.DB) {
	t.Helper()
	root := filepath.Join("..", "..", "deploy", "standalone", "migrations")
	body, err := os.ReadFile(filepath.Join(root, "0127_dashboard_page_rbac.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	script := string(body)
	start := strings.Index(script, "ALTER TABLE `mc_user`")
	end := strings.Index(script, "INSERT INTO `mochat_go_dashboard_permissions`")
	if start < 0 || end <= start {
		t.Fatal("0127 DDL boundaries not found")
	}
	execPreflightIntegrationSQL(t, db, script[start:end])
	for _, name := range []string{"0045_saas_admin_rbac.up.sql", "0033_saas_admin_operation_logs.up.sql", "0046_saas_admin_approvals.up.sql", "0047_saas_admin_approval_governance.up.sql"} {
		body, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		execPreflightIntegrationSQL(t, db, string(body))
	}
}

func execPreflightIntegrationFile(t *testing.T, db *sql.DB, name string) {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("..", "..", "deploy", "standalone", "migrations", name))
	if err != nil {
		t.Fatal(err)
	}
	execPreflightIntegrationSQL(t, db, string(body))
}

func execPreflightIntegrationSQL(t *testing.T, db *sql.DB, body string) {
	t.Helper()
	statements, err := migration.SplitSQLStatements(body)
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
}
