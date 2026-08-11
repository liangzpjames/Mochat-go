package identitymigration

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"jiyi/mochat-go/internal/migration"
)

func TestIdentityBackfillEngineRealMariaDBLifecycle(t *testing.T) {
	db := newCredentialIntegrationDB(t)
	createIdentityBackfillEngineFixture(t, db)
	execIdentityBackfillTestFile(t, db, "0129_identity_realms_single_corp_schema.up.sql")
	if _, err := db.Exec(`INSERT INTO mochat_go_saas_admin_user_access (user_id) VALUES (11)`); err != nil {
		t.Fatal(err)
	}

	manager := newCredentialIntegrationManager(t)
	schema := currentIdentityBackfillSchema(t, db)
	upPath := filepath.Join("..", "..", "deploy", "standalone", "migrations", "0130_identity_realms_single_corp_backfill.up.sql")
	root := filepath.Join("..", "..")
	firstRequest := "task8-engine-first"
	result, err := ApplyBackfill(context.Background(), db, DatabaseOptions{
		Schema:            schema,
		PlatformTenantID:  1,
		RequestID:         firstRequest,
		CredentialManager: manager,
	}, upPath)
	if err != nil {
		t.Fatal(err)
	}
	if result.Idempotent {
		t.Fatal("first engine backfill unexpectedly reported idempotent")
	}
	second, err := ApplyBackfill(context.Background(), db, DatabaseOptions{
		Schema:            schema,
		PlatformTenantID:  1,
		RequestID:         firstRequest,
		CredentialManager: manager,
	}, upPath)
	if err != nil {
		t.Fatal(err)
	}
	if !second.Idempotent {
		t.Fatal("completed engine backfill did not become idempotent")
	}
	if err := migration.RecordControlledMigration(context.Background(), db, root, migrationSource, firstRequest); err != nil {
		t.Fatal(err)
	}
	credentialResult, err := EncryptCredentials(context.Background(), db, manager, firstRequest)
	if err != nil {
		t.Fatal(err)
	}
	if credentialResult.CorpRowsWritten != 1 || credentialResult.AgentRowsWritten != 1 {
		t.Fatalf("engine credential result=%+v, want one corp and one agent write", credentialResult)
	}
	if err := execIdentityDownScript(t, db, firstRequest); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM mochat_go_schema_migrations WHERE version=?`, migrationSource); err != nil {
		t.Fatal(err)
	}
	assertCredentialCiphertextState(t, db, "mc_corp", 100, false)
	assertCredentialCiphertextState(t, db, "mc_work_agent", 300, false)

	secondRequest := "task8-engine-reapply"
	if _, err := ApplyBackfill(context.Background(), db, DatabaseOptions{
		Schema:            schema,
		PlatformTenantID:  1,
		RequestID:         secondRequest,
		CredentialManager: manager,
	}, upPath); err != nil {
		t.Fatal(err)
	}
	if err := migration.RecordControlledMigration(context.Background(), db, root, migrationSource, secondRequest); err != nil {
		t.Fatal(err)
	}
	if _, err := EncryptCredentials(context.Background(), db, manager, secondRequest); err != nil {
		t.Fatal(err)
	}
	if err := execIdentityDownScript(t, db, secondRequest); err != nil {
		t.Fatal(err)
	}
}

func currentIdentityBackfillSchema(t *testing.T, db *sql.DB) string {
	t.Helper()
	var schema string
	if err := db.QueryRow(`SELECT DATABASE()`).Scan(&schema); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(schema) == "" {
		t.Fatal("isolated integration schema is empty")
	}
	return schema
}

func createIdentityBackfillEngineFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, statement := range []string{
		`CREATE TABLE mc_tenant (id int(10) unsigned NOT NULL AUTO_INCREMENT, name varchar(255) NOT NULL DEFAULT '', status tinyint NOT NULL DEFAULT 1, logo varchar(255) NOT NULL DEFAULT '', login_background varchar(255) NOT NULL DEFAULT '', url varchar(255) DEFAULT '', created_at timestamp NULL DEFAULT CURRENT_TIMESTAMP, updated_at timestamp NULL DEFAULT NULL, deleted_at timestamp NULL, copyright varchar(255) NOT NULL DEFAULT '', server_ips json DEFAULT NULL, PRIMARY KEY (id)) ENGINE=InnoDB`,
		`CREATE TABLE mc_corp (id int(10) unsigned NOT NULL AUTO_INCREMENT, name varchar(255) NOT NULL DEFAULT '', wx_corpid char(255) NOT NULL DEFAULT '', social_code char(255) NOT NULL DEFAULT '', employee_secret char(255) NOT NULL DEFAULT '', event_callback varchar(255) NOT NULL DEFAULT '', contact_secret char(255) NOT NULL DEFAULT '', token char(255) NOT NULL DEFAULT '', encoding_aes_key char(255) NOT NULL DEFAULT '', chat_admin varchar(255) NOT NULL DEFAULT '', chat_admin_phone varchar(255) NOT NULL DEFAULT '', chat_admin_idcard varchar(255) NOT NULL DEFAULT '', chat_apply_status tinyint(1) NOT NULL DEFAULT 0, chat_status tinyint(1) NOT NULL DEFAULT 0, chat_secret varchar(255) NOT NULL DEFAULT '', service_contact_url varchar(255) NOT NULL DEFAULT '', chat_whitelist_ip json DEFAULT NULL, chat_rsa_key json DEFAULT NULL, tenant_id int(11) DEFAULT 0, created_at timestamp NULL DEFAULT NULL, updated_at timestamp NULL DEFAULT CURRENT_TIMESTAMP, deleted_at timestamp NULL, wecom_credentials_ciphertext text COLLATE utf8mb4_bin NULL, wecom_credentials_key_id varchar(64) COLLATE utf8mb4_bin NOT NULL DEFAULT '', PRIMARY KEY (id)) ENGINE=InnoDB`,
		`CREATE TABLE mc_user (id int(10) unsigned NOT NULL AUTO_INCREMENT, phone char(11) NOT NULL DEFAULT '', password varchar(255) NOT NULL DEFAULT '', name varchar(255) NOT NULL DEFAULT '', gender tinyint unsigned NOT NULL DEFAULT 0, department varchar(255) NOT NULL DEFAULT '', position varchar(255) NOT NULL DEFAULT '', login_time timestamp NULL DEFAULT NULL, status tinyint unsigned NOT NULL DEFAULT 0, tenant_id int(11) NOT NULL DEFAULT 1, created_at timestamp NULL DEFAULT CURRENT_TIMESTAMP, updated_at timestamp NULL DEFAULT NULL, deleted_at timestamp NULL, isSuperAdmin tinyint(1) DEFAULT 0, PRIMARY KEY (id)) ENGINE=InnoDB`,
		`CREATE TABLE mc_rbac_role (id int(11) NOT NULL AUTO_INCREMENT, tenant_id int(11) NOT NULL, name varchar(255) NOT NULL DEFAULT '', remarks varchar(255) NOT NULL DEFAULT '', status tinyint NOT NULL DEFAULT 1, operate_id int(11) NOT NULL, operate_name varchar(255) NOT NULL DEFAULT '', data_permission json DEFAULT NULL, created_at timestamp NULL DEFAULT NULL, updated_at timestamp NULL DEFAULT CURRENT_TIMESTAMP, deleted_at timestamp NULL DEFAULT NULL, PRIMARY KEY (id)) ENGINE=InnoDB`,
		`CREATE TABLE mc_rbac_user_role (id int(11) NOT NULL AUTO_INCREMENT, user_id int(11) NOT NULL DEFAULT 0, role_id int(11) NOT NULL DEFAULT 0, created_at timestamp NULL DEFAULT NULL, updated_at timestamp NULL DEFAULT CURRENT_TIMESTAMP, deleted_at timestamp NULL DEFAULT NULL, PRIMARY KEY (id)) ENGINE=InnoDB`,
		`CREATE TABLE mc_work_agent (id int(10) unsigned NOT NULL AUTO_INCREMENT, corp_id int(11) NOT NULL, wx_agent_id varchar(255) NOT NULL DEFAULT '', wx_secret varchar(255) NOT NULL DEFAULT '', name varchar(255) NOT NULL DEFAULT '', square_logo_url varchar(255) NOT NULL DEFAULT '', description varchar(255) NOT NULL DEFAULT '', close tinyint NOT NULL DEFAULT 0, redirect_domain varchar(255) NOT NULL DEFAULT '', report_location_flag tinyint NOT NULL DEFAULT 0, is_reportenter tinyint NOT NULL DEFAULT 0, home_url varchar(255) NOT NULL DEFAULT '', created_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at timestamp NULL DEFAULT NULL, deleted_at timestamp NULL DEFAULT NULL, wecom_credentials_ciphertext text COLLATE utf8mb4_bin NULL, wecom_credentials_key_id varchar(64) COLLATE utf8mb4_bin NOT NULL DEFAULT '', PRIMARY KEY (id)) ENGINE=InnoDB`,
		`INSERT INTO mc_tenant (id, name, status) VALUES (1, 'Tenant 1', 1), (2, 'Tenant 2', 1)`,
		`INSERT INTO mc_corp (id, tenant_id, name, wx_corpid, employee_secret, contact_secret, token, encoding_aes_key, chat_secret) VALUES (100, 1, 'Corp 1', 'corp-100', 'employee-value', 'contact-value', 'callback-value', 'aes-value', 'chat-value'), (200, 2, 'Corp 2', 'corp-200', '', '', '', '', '')`,
		`INSERT INTO mc_work_agent (id, corp_id, wx_agent_id, wx_secret) VALUES (300, 100, 'agent-300', 'agent-value')`,
		`INSERT INTO mc_user (id, tenant_id, phone, password, name, status, deleted_at, isSuperAdmin) VALUES (11, 1, '13800000011', 'legacy-platform-hash', 'Platform admin', 1, NULL, 1), (13, 2, '13800000013', 'legacy-dashboard-hash', 'Dashboard user', 1, NULL, 0)`,
		`INSERT INTO mc_rbac_role (id, tenant_id, operate_id) VALUES (20, 1, 0)`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	loadIdentityBackfillTestDDL(t, db)
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

func loadIdentityBackfillTestDDL(t *testing.T, db *sql.DB) {
	t.Helper()
	root := filepath.Join("..", "..", "deploy", "standalone", "migrations")
	pageBody, err := os.ReadFile(filepath.Join(root, "0127_dashboard_page_rbac.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	pageSQL := string(pageBody)
	start := strings.Index(pageSQL, "ALTER TABLE `mc_user`")
	end := strings.Index(pageSQL, "INSERT INTO `mochat_go_dashboard_permissions`")
	if start < 0 || end <= start {
		t.Fatal("0127 DDL boundaries not found")
	}
	execIdentityBackfillTestSQL(t, db, pageSQL[start:end])
	for _, name := range []string{"0045_saas_admin_rbac.up.sql", "0033_saas_admin_operation_logs.up.sql", "0046_saas_admin_approvals.up.sql", "0047_saas_admin_approval_governance.up.sql"} {
		body, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		execIdentityBackfillTestSQL(t, db, string(body))
	}
}

func execIdentityBackfillTestFile(t *testing.T, db *sql.DB, name string) {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("..", "..", "deploy", "standalone", "migrations", name))
	if err != nil {
		t.Fatal(err)
	}
	execIdentityBackfillTestSQL(t, db, string(body))
}

func execIdentityBackfillTestSQL(t *testing.T, db *sql.DB, body string) {
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
