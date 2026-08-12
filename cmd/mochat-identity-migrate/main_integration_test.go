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

var migrateIntegrationSchemaSequence atomic.Int64

func TestRunMigrationCLIRealMariaDBLifecycle(t *testing.T) {
	db, dsn, schema := newMigrateIntegrationDB(t)
	createMigrateIntegrationFixture(t, db)
	execMigrateIntegrationFile(t, db, "0129_identity_realms_single_corp_schema.up.sql")
	if _, err := db.Exec(`INSERT INTO mochat_go_saas_admin_user_access (user_id) VALUES (11)`); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	dsnFile := filepath.Join(dir, "dsn")
	keyFile := filepath.Join(dir, "credential-key")
	confirmationFile := filepath.Join(dir, "maintenance-confirmation")
	for path, body := range map[string]string{
		dsnFile:          dsn,
		keyFile:          strings.Repeat("01", 32),
		confirmationFile: "MOCHAT_IDENTITY_MAINTENANCE_V1\nschema=" + schema + "\nrequest_id=task8-cli-first",
	} {
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	root := filepath.Join("..", "..")
	common := []string{"--execute", "--dsn-file", dsnFile, "--schema", schema, "--platform-tenant-id", "1", "--maintenance-confirmation-file", confirmationFile, "--credential-key-file", keyFile, "--credential-key-id", "task8-wecom-v1", "--project-root", root, "--timeout", "5m"}
	var output strings.Builder
	if err := runMigration(append([]string{"up", "--request-id", "task8-cli-first"}, common...), &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "up\tcompleted") {
		t.Fatalf("up output=%q", output.String())
	}
	output.Reset()
	if err := runMigration(append([]string{"encrypt-credentials", "--request-id", "task8-cli-first"}, common...), &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "encrypt-credentials\t1\t1") {
		t.Fatalf("encrypt output=%q", output.String())
	}
	output.Reset()
	if err := runMigration([]string{"down", "--execute", "--request-id", "task8-cli-first", "--dsn-file", dsnFile, "--schema", schema, "--platform-tenant-id", "1", "--maintenance-confirmation-file", confirmationFile, "--project-root", root, "--timeout", "5m"}, &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "down\tcompleted") {
		t.Fatalf("down output=%q", output.String())
	}

	secondConfirmation := filepath.Join(dir, "maintenance-confirmation-second")
	if err := os.WriteFile(secondConfirmation, []byte("MOCHAT_IDENTITY_MAINTENANCE_V1\nschema="+schema+"\nrequest_id=task8-cli-second"), 0o600); err != nil {
		t.Fatal(err)
	}
	common[8] = secondConfirmation
	output.Reset()
	if err := runMigration(append([]string{"up", "--request-id", "task8-cli-second"}, common...), &output); err != nil {
		t.Fatal(err)
	}
	output.Reset()
	if err := runMigration(append([]string{"encrypt-credentials", "--request-id", "task8-cli-second"}, common...), &output); err != nil {
		t.Fatal(err)
	}
	if err := runMigration([]string{"down", "--execute", "--request-id", "task8-cli-second", "--dsn-file", dsnFile, "--schema", schema, "--platform-tenant-id", "1", "--maintenance-confirmation-file", secondConfirmation, "--project-root", root, "--timeout", "5m"}, &output); err != nil {
		t.Fatal(err)
	}
	var standardRows int
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_schema_migrations WHERE version=?`, migrationSourceForMigrateIntegration()).Scan(&standardRows); err != nil {
		t.Fatal(err)
	}
	if standardRows != 0 {
		t.Fatalf("standard controlled migration rows=%d after CLI down, want 0", standardRows)
	}
}

func TestRunMigrationCLICutoverDownRestoreAndReapplyRealMariaDB(t *testing.T) {
	db, dsn, schema := newMigrateIntegrationDB(t)
	createMigrateIntegrationFixture(t, db)
	execMigrateIntegrationFile(t, db, "0129_identity_realms_single_corp_schema.up.sql")
	ensureMigrateIntegrationStandardLedger(t, db)

	dir := t.TempDir()
	dsnFile := filepath.Join(dir, "dsn")
	keyFile := filepath.Join(dir, "credential-key")
	if err := os.WriteFile(dsnFile, []byte(dsn), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyFile, []byte(strings.Repeat("01", 32)), 0o600); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join("..", "..")
	writeConfirmation := func(requestID string) string {
		path := filepath.Join(dir, "maintenance-"+requestID)
		body := "MOCHAT_IDENTITY_MAINTENANCE_V1\nschema=" + schema + "\nrequest_id=" + requestID
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	args := func(action, requestID, confirmation string, withKey bool) []string {
		result := []string{action, "--execute", "--request-id", requestID, "--dsn-file", dsnFile, "--schema", schema, "--platform-tenant-id", "1", "--maintenance-confirmation-file", confirmation, "--project-root", root, "--timeout", "5m"}
		if withKey {
			result = append(result, "--credential-key-file", keyFile, "--credential-key-id", "task12-wecom-v1")
		}
		return result
	}

	backfillRequest := "task12-cutover-backfill"
	var output strings.Builder
	if err := runMigration(args("up", backfillRequest, writeConfirmation(backfillRequest), true), &output); err != nil {
		t.Fatal(err)
	}
	output.Reset()
	if err := runMigration(args("encrypt-credentials", backfillRequest, writeConfirmation(backfillRequest), true), &output); err != nil {
		t.Fatal(err)
	}
	var originalCiphertext, originalKeyID string
	if err := db.QueryRow(`SELECT COALESCE(CAST(wecom_credentials_ciphertext AS CHAR),''), COALESCE(wecom_credentials_key_id,'') FROM mc_corp WHERE id=?`, 100).Scan(&originalCiphertext, &originalKeyID); err != nil {
		t.Fatal(err)
	}
	if originalCiphertext == "" || originalKeyID == "" {
		t.Fatal("credential copy was not created")
	}
	if _, err := db.Exec(`UPDATE mc_corp SET wecom_credentials_ciphertext=?, wecom_credentials_key_id=? WHERE id=?`, "tampered-copy", originalKeyID, 100); err != nil {
		t.Fatal(err)
	}
	badCutoverRequest := "task12-cutover-preflight-failure"
	output.Reset()
	if err := runMigration(args("cutover", badCutoverRequest, writeConfirmation(badCutoverRequest), true), &output); err == nil {
		t.Fatal("tampered credential cutover unexpectedly succeeded")
	}
	assertMigrateIntegrationColumn(t, db, "mc_user", "password", true)
	assertMigrateIntegrationValue(t, db, "mc_corp", "employee_secret", 100, "employee-value")
	assertMigrateIntegrationTable(t, db, "mochat_go_identity_cutover_batches", false)
	if _, err := db.Exec(`UPDATE mc_corp SET wecom_credentials_ciphertext=?, wecom_credentials_key_id=? WHERE id=?`, originalCiphertext, originalKeyID, 100); err != nil {
		t.Fatal(err)
	}

	cutoverRequest := "task12-cutover-first"
	output.Reset()
	if err := runMigration(args("cutover", cutoverRequest, writeConfirmation(cutoverRequest), true), &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "cutover\tcompleted") {
		t.Fatalf("cutover output=%q", output.String())
	}
	assertMigrateIntegrationColumn(t, db, "mc_user", "password", false)
	assertMigrateIntegrationBlank(t, db, "mc_corp", "employee_secret", 100)
	assertMigrateIntegrationBlank(t, db, "mc_work_agent", "wx_secret", 300)
	assertMigrateIntegrationCompanyPermission(t, db)
	assertMigrateIntegrationStandardCutoverLedger(t, db, true)

	output.Reset()
	downArgs := args("down", cutoverRequest, writeConfirmation(cutoverRequest), false)
	downArgs = append(downArgs, "--migration-version", "0131_identity_realms_single_corp_cutover")
	if err := runMigration(downArgs, &output); err != nil {
		t.Fatal(err)
	}
	assertMigrateIntegrationColumn(t, db, "mc_user", "password", true)
	assertMigrateIntegrationBlank(t, db, "mc_corp", "employee_secret", 100)
	assertMigrateIntegrationLegacyCompanyPermission(t, db)
	assertMigrateIntegrationStandardCutoverLedger(t, db, false)
	assertMigrateIntegrationTable(t, db, "mochat_go_identity_cutover_journal", true)
	assertMigrateIntegrationCutoverBatchStatus(t, db, cutoverRequest, "rolled_back")
	output.Reset()
	if err := runMigration(args("restore-legacy-credentials", cutoverRequest, writeConfirmation(cutoverRequest), true), &output); err != nil {
		t.Fatal(err)
	}
	assertMigrateIntegrationValue(t, db, "mc_corp", "employee_secret", 100, "employee-value")
	assertMigrateIntegrationValue(t, db, "mc_work_agent", "wx_secret", 300, "agent-value")
	assertMigrateIntegrationTable(t, db, "mochat_go_identity_cutover_journal", true)
	assertMigrateIntegrationCutoverBatchStatus(t, db, cutoverRequest, "restored")

	reapplyRequest := "task12-cutover-reapply"
	output.Reset()
	if err := runMigration(args("cutover", reapplyRequest, writeConfirmation(reapplyRequest), true), &output); err != nil {
		t.Fatal(err)
	}
	assertMigrateIntegrationColumn(t, db, "mc_user", "password", false)
}

func ensureMigrateIntegrationStandardLedger(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`CREATE TABLE mochat_go_schema_migrations (version varchar(64) NOT NULL, description varchar(255) NOT NULL DEFAULT '', checksum char(64) NOT NULL DEFAULT '', applied_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP, execution_ms int unsigned NOT NULL DEFAULT 0, PRIMARY KEY (version)) ENGINE=InnoDB`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO mochat_go_schema_migrations (version, description, checksum) VALUES ('0129_identity_realms_single_corp_schema', 'identity schema', ?)`, strings.Repeat("1", 64)); err != nil {
		t.Fatal(err)
	}
}

func assertMigrateIntegrationColumn(t *testing.T, db *sql.DB, table, column string, wantPresent bool) {
	t.Helper()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name=? AND column_name=?`, table, column).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if (count == 1) != wantPresent {
		t.Fatalf("%s.%s present=%t want=%t", table, column, count == 1, wantPresent)
	}
}

func assertMigrateIntegrationBlank(t *testing.T, db *sql.DB, table, column string, id int64) {
	t.Helper()
	var value string
	if err := db.QueryRow(`SELECT COALESCE(`+column+`, '') FROM `+table+` WHERE id=?`, id).Scan(&value); err != nil {
		t.Fatal(err)
	}
	if value != "" {
		t.Fatalf("%s.%s was not cleared", table, column)
	}
}

func assertMigrateIntegrationValue(t *testing.T, db *sql.DB, table, column string, id int64, want string) {
	t.Helper()
	var value string
	if err := db.QueryRow(`SELECT COALESCE(`+column+`, '') FROM `+table+` WHERE id=?`, id).Scan(&value); err != nil {
		t.Fatal(err)
	}
	if value != want {
		t.Fatalf("%s.%s restore mismatch", table, column)
	}
}

func assertMigrateIntegrationCompanyPermission(t *testing.T, db *sql.DB) {
	t.Helper()
	var superadminOnly int
	if err := db.QueryRow(`SELECT superadmin_only FROM mochat_go_dashboard_permissions WHERE code=?`, "dashboard.company_setting.website").Scan(&superadminOnly); err != nil {
		t.Fatal(err)
	}
	if superadminOnly != 1 {
		t.Fatalf("company permission superadmin_only=%d", superadminOnly)
	}
	var oldResources int
	if err := db.QueryRow(`
		SELECT COUNT(*)
		FROM mochat_go_dashboard_permission_resources resource
		INNER JOIN mochat_go_dashboard_permissions permission ON permission.id=resource.permission_id
		WHERE permission.code=? AND resource.path_pattern LIKE '/dashboard/corp/%'`, "dashboard.company_setting.website").Scan(&oldResources); err != nil {
		t.Fatal(err)
	}
	if oldResources != 0 {
		t.Fatalf("legacy company permission resources=%d", oldResources)
	}
	var newResources int
	if err := db.QueryRow(`
		SELECT COUNT(*)
		FROM mochat_go_dashboard_permission_resources resource
		INNER JOIN mochat_go_dashboard_permissions permission ON permission.id=resource.permission_id
		WHERE permission.code=? AND resource.path_pattern LIKE '/dashboard/company/%'`, "dashboard.company_setting.website").Scan(&newResources); err != nil {
		t.Fatal(err)
	}
	if newResources != 9 {
		t.Fatalf("company permission resources=%d want=9", newResources)
	}
}

func assertMigrateIntegrationLegacyCompanyPermission(t *testing.T, db *sql.DB) {
	t.Helper()
	var restriction string
	var superadminOnly int
	if err := db.QueryRow(`SELECT restriction, superadmin_only FROM mochat_go_dashboard_permissions WHERE code=?`, "dashboard.company_setting.website").Scan(&restriction, &superadminOnly); err != nil {
		t.Fatal(err)
	}
	if restriction != "grantable" || superadminOnly != 0 {
		t.Fatalf("restored company permission restriction=%q superadmin_only=%d", restriction, superadminOnly)
	}
	var oldResources int
	if err := db.QueryRow(`
		SELECT COUNT(*)
		FROM mochat_go_dashboard_permission_resources resource
		INNER JOIN mochat_go_dashboard_permissions permission ON permission.id=resource.permission_id
		WHERE permission.code=? AND resource.path_pattern IN (
			'/dashboard/corp/index', '/dashboard/corp/show', '/dashboard/corp/store', '/dashboard/corp/update'
		)`, "dashboard.company_setting.website").Scan(&oldResources); err != nil {
		t.Fatal(err)
	}
	if oldResources != 4 {
		t.Fatalf("restored legacy company resources=%d want=4", oldResources)
	}
	var newResources int
	if err := db.QueryRow(`
		SELECT COUNT(*)
		FROM mochat_go_dashboard_permission_resources resource
		INNER JOIN mochat_go_dashboard_permissions permission ON permission.id=resource.permission_id
		WHERE permission.code=? AND resource.path_pattern LIKE '/dashboard/company/%'`, "dashboard.company_setting.website").Scan(&newResources); err != nil {
		t.Fatal(err)
	}
	if newResources != 0 {
		t.Fatalf("restored company resources still include cutover paths=%d", newResources)
	}
}

func assertMigrateIntegrationStandardCutoverLedger(t *testing.T, db *sql.DB, wantPresent bool) {
	t.Helper()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_schema_migrations WHERE version=?`, "0131_identity_realms_single_corp_cutover").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if (count == 1) != wantPresent {
		t.Fatalf("0131 standard ledger present=%t want=%t", count == 1, wantPresent)
	}
}

func assertMigrateIntegrationCutoverBatchStatus(t *testing.T, db *sql.DB, requestID, want string) {
	t.Helper()
	var status string
	if err := db.QueryRow(`SELECT status FROM mochat_go_identity_cutover_batches WHERE request_id=?`, requestID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != want {
		t.Fatalf("cutover batch status=%q want=%q", status, want)
	}
}

func assertMigrateIntegrationTable(t *testing.T, db *sql.DB, table string, wantPresent bool) {
	t.Helper()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name=?`, table).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if (count == 1) != wantPresent {
		t.Fatalf("%s present=%t want=%t", table, count == 1, wantPresent)
	}
}

func migrationSourceForMigrateIntegration() string {
	return "0130_identity_realms_single_corp_backfill"
}

func newMigrateIntegrationDB(t *testing.T) (*sql.DB, string, string) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("MOCHAT_GO_MYSQL_INTEGRATION_DSN"))
	if dsn == "" {
		t.Skip("MOCHAT_GO_MYSQL_INTEGRATION_DSN is required for migrate CLI MariaDB integration")
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
	schema := fmt.Sprintf("mochat_identity_single_corp_%d_%d", os.Getpid(), migrateIntegrationSchemaSequence.Add(1))
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

func createMigrateIntegrationFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, statement := range []string{
		`CREATE TABLE mc_tenant (id int(10) unsigned NOT NULL AUTO_INCREMENT, name varchar(255) NOT NULL DEFAULT '', status tinyint NOT NULL DEFAULT 1, deleted_at timestamp NULL, PRIMARY KEY (id)) ENGINE=InnoDB`,
		`CREATE TABLE mc_corp (id int(10) unsigned NOT NULL AUTO_INCREMENT, tenant_id int(11) DEFAULT 0, name varchar(255) NOT NULL DEFAULT '', wx_corpid varchar(255) NOT NULL DEFAULT '', employee_secret varchar(255) NOT NULL DEFAULT '', contact_secret varchar(255) NOT NULL DEFAULT '', token varchar(255) NOT NULL DEFAULT '', encoding_aes_key varchar(255) NOT NULL DEFAULT '', chat_secret varchar(255) NOT NULL DEFAULT '', wecom_credentials_ciphertext text NULL, wecom_credentials_key_id varchar(64) NOT NULL DEFAULT '', deleted_at timestamp NULL, PRIMARY KEY (id)) ENGINE=InnoDB`,
		`CREATE TABLE mc_user (id int(10) unsigned NOT NULL AUTO_INCREMENT, tenant_id int(11) NOT NULL DEFAULT 1, phone char(11) NOT NULL DEFAULT '', password varchar(255) NOT NULL DEFAULT '', name varchar(255) NOT NULL DEFAULT '', status tinyint unsigned NOT NULL DEFAULT 1, deleted_at timestamp NULL, isSuperAdmin tinyint NOT NULL DEFAULT 0, PRIMARY KEY (id)) ENGINE=InnoDB`,
		`CREATE TABLE mc_rbac_role (id int(11) NOT NULL AUTO_INCREMENT, tenant_id int(11) NOT NULL, data_permission json DEFAULT NULL, deleted_at timestamp NULL, PRIMARY KEY (id)) ENGINE=InnoDB`,
		`CREATE TABLE mc_rbac_user_role (id int(11) NOT NULL AUTO_INCREMENT, user_id int(11) NOT NULL, role_id int(11) NOT NULL, deleted_at timestamp NULL, PRIMARY KEY (id)) ENGINE=InnoDB`,
		`CREATE TABLE mc_work_agent (id int(10) unsigned NOT NULL AUTO_INCREMENT, corp_id int(11) NOT NULL, wx_agent_id varchar(255) NOT NULL DEFAULT '', wx_secret varchar(255) NOT NULL DEFAULT '', wecom_credentials_ciphertext text NULL, wecom_credentials_key_id varchar(64) NOT NULL DEFAULT '', PRIMARY KEY (id)) ENGINE=InnoDB`,
		`INSERT INTO mc_tenant (id, name, status) VALUES (1, 'Tenant 1', 1), (2, 'Tenant 2', 1)`,
		`INSERT INTO mc_corp (id, tenant_id, name, wx_corpid, employee_secret, contact_secret, token, encoding_aes_key, chat_secret) VALUES (100, 1, 'Corp 1', 'corp-100', 'employee-value', 'contact-value', 'callback-value', 'aes-value', 'chat-value'), (200, 2, 'Corp 2', 'corp-200', '', '', '', '', '')`,
		`INSERT INTO mc_work_agent (id, corp_id, wx_agent_id, wx_secret) VALUES (300, 100, 'agent-300', 'agent-value')`,
		`INSERT INTO mc_user (id, tenant_id, phone, password, name, status, deleted_at, isSuperAdmin) VALUES (11, 1, '13800000011', 'legacy-platform-hash', 'Platform admin', 1, NULL, 1), (13, 2, '13800000013', 'legacy-dashboard-hash', 'Dashboard user', 1, NULL, 0)`,
		`INSERT INTO mc_rbac_role (id, tenant_id) VALUES (20, 1)`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	loadMigrateIntegrationDDL(t, db)
	for _, statement := range []string{
		`INSERT INTO mochat_go_dashboard_permissions (id, code, permission_type, path, name) VALUES (900, 'dashboard.test', 'page', '/test', 'Test')`,
		`INSERT INTO mochat_go_dashboard_permissions (id, code, permission_type, path, name, restriction, superadmin_only) VALUES (902, 'dashboard.company_setting.website', 'page', '/company-setting/website', '企业信息', 'grantable', 0)`,
		`INSERT INTO mochat_go_dashboard_permission_resources (id, permission_id, resource_type, http_method, path_pattern) VALUES (901, 900, 'api', 'GET', '/dashboard/test')`,
		`INSERT INTO mochat_go_dashboard_permission_resources (id, permission_id, resource_type, http_method, path_pattern) VALUES (902, 902, 'api', 'GET', '/dashboard/corp/index'), (903, 902, 'api', 'GET', '/dashboard/corp/show'), (904, 902, 'api', 'POST', '/dashboard/corp/store'), (905, 902, 'api', 'PUT', '/dashboard/corp/update')`,
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

func loadMigrateIntegrationDDL(t *testing.T, db *sql.DB) {
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
	execMigrateIntegrationSQL(t, db, script[start:end])
	for _, name := range []string{"0045_saas_admin_rbac.up.sql", "0033_saas_admin_operation_logs.up.sql", "0046_saas_admin_approvals.up.sql", "0047_saas_admin_approval_governance.up.sql"} {
		body, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		execMigrateIntegrationSQL(t, db, string(body))
	}
}

func execMigrateIntegrationFile(t *testing.T, db *sql.DB, name string) {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("..", "..", "deploy", "standalone", "migrations", name))
	if err != nil {
		t.Fatal(err)
	}
	execMigrateIntegrationSQL(t, db, string(body))
}

func execMigrateIntegrationSQL(t *testing.T, db *sql.DB, body string) {
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
