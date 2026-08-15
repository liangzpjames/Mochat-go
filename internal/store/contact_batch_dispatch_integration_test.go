package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"

	"jiyi/mochat-go/internal/dashboard"
	"jiyi/mochat-go/internal/dashboardprincipal"
	"jiyi/mochat-go/internal/migration"
	"jiyi/mochat-go/internal/wecomcapability"
	"jiyi/mochat-go/internal/wecomcredentials"
)

// P0-2: real temporary MariaDB contact batch integration. The harness creates
// an isolated mochat_contact_batch_<pid>_<seq> schema, builds the complete
// legacy fixture with real column shapes, applies the real 0139 ledger
// migration, and drives the production MySQLStore. It never touches the
// business database and drops the schema at the end (leftovers=0).

var contactBatchSchemaSequence atomic.Int64

type contactBatchIntegrationHarness struct {
	db        *sql.DB
	admin     *sql.DB
	schema    string
	store     *MySQLStore
	manager   *wecomcredentials.Manager
	principal dashboardprincipal.DashboardPrincipal
	corpID    int
	userID    int
}

func newContactBatchIntegrationHarness(t *testing.T) *contactBatchIntegrationHarness {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("MOCHAT_GO_MYSQL_INTEGRATION_DSN"))
	if dsn == "" {
		t.Skip("SKIP: MOCHAT_GO_MYSQL_INTEGRATION_DSN is not set; isolated MariaDB DSN is required")
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
	if err := admin.PingContext(context.Background()); err != nil {
		_ = admin.Close()
		t.Fatal(err)
	}
	schema := fmt.Sprintf("mochat_contact_batch_%d_%d", os.Getpid(), contactBatchSchemaSequence.Add(1))
	if _, err := admin.Exec("CREATE DATABASE `" + schema + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
		_ = admin.Close()
		t.Fatal(err)
	}
	testCfg := *cfg
	testCfg.DBName = schema
	db, err := sql.Open("mysql", testCfg.FormatDSN())
	if err != nil {
		_, _ = admin.Exec("DROP DATABASE IF EXISTS `" + schema + "`")
		_ = admin.Close()
		t.Fatal(err)
	}
	if err := db.PingContext(context.Background()); err != nil {
		_ = db.Close()
		_, _ = admin.Exec("DROP DATABASE IF EXISTS `" + schema + "`")
		_ = admin.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = db.Close()
		if _, err := admin.Exec("DROP DATABASE IF EXISTS `" + schema + "`"); err != nil {
			t.Errorf("drop temporary schema: %v", err)
		}
		var leftovers int
		if err := admin.QueryRow(`SELECT COUNT(*) FROM information_schema.schemata WHERE schema_name=?`, schema).Scan(&leftovers); err != nil {
			t.Errorf("check temporary schema cleanup: %v", err)
		} else if leftovers != 0 {
			t.Errorf("temporary schema %s still exists after cleanup", schema)
		}
		_ = admin.Close()
	})
	manager, err := wecomcredentials.NewManager(wecomcredentials.Config{
		EncryptionKey:       "3131313131313131313131313131313131313131313131313131313131313131",
		EncryptionKeyID:     "wecom-q3",
		RequireEncryption:   true,
		DedicatedConfigured: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	createContactBatchFixture(t, db, manager)
	root := filepath.Join("..", "..")
	runner, err := migration.NewRunner(db, []migration.Migration{{
		Version: "0139_wecom_capability_ledger", Description: "wecom capability ledger",
		Path:     filepath.Join(root, "deploy", "standalone", "migrations", "0139_wecom_capability_ledger.up.sql"),
		DownPath: filepath.Join(root, "deploy", "standalone", "migrations", "0139_wecom_capability_ledger.down.sql"),
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runner.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	store := NewMySQLStore(db).WithWeComCredentialCipher(manager)
	return &contactBatchIntegrationHarness{
		db: db, admin: admin, schema: schema, store: store, manager: manager,
		principal: dashboardprincipal.DashboardPrincipal{TenantID: 11, CorpID: 1101, UserID: 11001, AuthVersion: 1, IsSuperAdmin: true},
		corpID:    1101,
		userID:    11001,
	}
}

// contactBatchLimitsJSON returns a limits_json payload that satisfies the
// dashboard tenant gate (every SaaSAdminPackageLimits field present).
func contactBatchLimitsJSON() string {
	return `{"maxCorps":10,"maxUsers":100,"maxContacts":1000,"maxRooms":100,"maxAgents":10,"channelCodes":10,"shopCodes":10,"radars":10,"lotteries":10,"roomInfinitePulls":10,"roomFissions":10,"roomClockIns":10,"roomQualities":10,"roomCalendars":10,"roomReminds":10,"contactSops":10,"roomSops":10,"sensitiveWords":10,"storageMb":1024,"contactMessageBatches":100,"roomMessageBatches":100,"roomTagPulls":10,"workRoomAutoPulls":10,"workFissions":10,"officialAccounts":10,"asyncExecutions":10}`
}

func createContactBatchFixture(t *testing.T, db *sql.DB, manager *wecomcredentials.Manager) {
	t.Helper()
	corpCiphertext, corpKeyID, err := manager.EncryptCorp(11, "wx-corp-1101", wecomcredentials.CorpCredential{
		EmployeeSecret: "employee-fixture", ContactSecret: "contact-fixture",
		CallbackToken: "callback-fixture", EncodingAESKey: "aes-fixture", ChatSecret: "archive-fixture",
	})
	if err != nil {
		t.Fatal(err)
	}
	agentCiphertext, agentKeyID, err := manager.EncryptAgent(1101, "100001", wecomcredentials.AgentCredential{WXSecret: "agent-fixture"})
	if err != nil {
		t.Fatal(err)
	}
	statements := []string{
		`CREATE TABLE mc_tenant (id INT UNSIGNED NOT NULL AUTO_INCREMENT, name VARCHAR(255) NOT NULL DEFAULT '', status TINYINT UNSIGNED NOT NULL DEFAULT 1, created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL, deleted_at TIMESTAMP NULL, PRIMARY KEY (id)) ENGINE=InnoDB`,
		`CREATE TABLE mc_corp (id INT UNSIGNED NOT NULL AUTO_INCREMENT, tenant_id INT UNSIGNED NOT NULL DEFAULT 0, name VARCHAR(255) NOT NULL DEFAULT '', wx_corpid VARCHAR(255) NOT NULL DEFAULT '', employee_secret VARCHAR(255) NOT NULL DEFAULT '', contact_secret VARCHAR(255) NOT NULL DEFAULT '', token VARCHAR(255) NOT NULL DEFAULT '', encoding_aes_key VARCHAR(255) NOT NULL DEFAULT '', chat_secret VARCHAR(255) NOT NULL DEFAULT '', wecom_credentials_ciphertext TEXT NULL, wecom_credentials_key_id VARCHAR(64) NOT NULL DEFAULT '', created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL, deleted_at TIMESTAMP NULL, PRIMARY KEY (id), UNIQUE KEY uni_mc_corp_tenant_id_id (tenant_id,id)) ENGINE=InnoDB`,
		`CREATE TABLE mc_user (id INT UNSIGNED NOT NULL AUTO_INCREMENT, tenant_id INT UNSIGNED NOT NULL DEFAULT 1, phone CHAR(11) NOT NULL DEFAULT '', password VARCHAR(255) NOT NULL DEFAULT '', name VARCHAR(255) NOT NULL DEFAULT '', status TINYINT UNSIGNED NOT NULL DEFAULT 1, isSuperAdmin TINYINT NOT NULL DEFAULT 0, created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL, deleted_at TIMESTAMP NULL, PRIMARY KEY (id), UNIQUE KEY uni_dashboard_user_tenant_id_id (tenant_id,id)) ENGINE=InnoDB`,
		`CREATE TABLE mochat_go_tenant_corp_bindings (tenant_id INT UNSIGNED NOT NULL, corp_id INT UNSIGNED NOT NULL, status TINYINT UNSIGNED NOT NULL DEFAULT 1, version BIGINT UNSIGNED NOT NULL DEFAULT 1, verified_wx_corpid VARCHAR(255) NULL, verified_corp_name VARCHAR(255) NOT NULL DEFAULT '', verified_at TIMESTAMP NULL, employee_credential_generation BIGINT UNSIGNED NOT NULL DEFAULT 1, contact_credential_generation BIGINT UNSIGNED NOT NULL DEFAULT 1, agent_credential_generation BIGINT UNSIGNED NOT NULL DEFAULT 1, callback_credential_generation BIGINT UNSIGNED NOT NULL DEFAULT 1, created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL, PRIMARY KEY (tenant_id), UNIQUE KEY uni_tenant_corp_binding_corp (corp_id), CONSTRAINT fk_tenant_corp_binding_corp FOREIGN KEY (tenant_id,corp_id) REFERENCES mc_corp (tenant_id,id)) ENGINE=InnoDB`,
		`CREATE TABLE mochat_go_dashboard_identities (user_id INT UNSIGNED NOT NULL, tenant_id INT UNSIGNED NOT NULL DEFAULT 0, login_identifier VARCHAR(255) NOT NULL DEFAULT '', password_hash VARCHAR(255) NOT NULL DEFAULT '', status TINYINT UNSIGNED NOT NULL DEFAULT 1, must_rotate_password TINYINT UNSIGNED NOT NULL DEFAULT 0, auth_version BIGINT UNSIGNED NOT NULL DEFAULT 1, mfa_required TINYINT UNSIGNED NOT NULL DEFAULT 0, activated_at TIMESTAMP NULL, created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL, deleted_at TIMESTAMP NULL, PRIMARY KEY (user_id)) ENGINE=InnoDB`,
		`CREATE TABLE mc_work_employee (id INT UNSIGNED NOT NULL AUTO_INCREMENT, wx_user_id VARCHAR(255) NOT NULL DEFAULT '', corp_id INT UNSIGNED NOT NULL DEFAULT 0, name VARCHAR(255) NOT NULL DEFAULT '', status TINYINT UNSIGNED NOT NULL DEFAULT 0, log_user_id INT UNSIGNED NOT NULL DEFAULT 0, created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL, deleted_at TIMESTAMP NULL, PRIMARY KEY (id), KEY idx_company_employee_corp (corp_id, deleted_at)) ENGINE=InnoDB`,
		`CREATE TABLE mc_work_contact (id INT UNSIGNED NOT NULL AUTO_INCREMENT, corp_id INT UNSIGNED NOT NULL DEFAULT 0, wx_external_userid VARCHAR(255) NOT NULL DEFAULT '', name VARCHAR(255) NOT NULL DEFAULT '', created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL, deleted_at TIMESTAMP NULL, PRIMARY KEY (id)) ENGINE=InnoDB`,
		`CREATE TABLE mc_work_contact_employee (id INT UNSIGNED NOT NULL AUTO_INCREMENT, contact_id INT UNSIGNED NOT NULL DEFAULT 0, employee_id INT UNSIGNED NOT NULL DEFAULT 0, corp_id INT UNSIGNED NOT NULL DEFAULT 0, created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL, deleted_at TIMESTAMP NULL, PRIMARY KEY (id), KEY idx_ce_contact (contact_id), KEY idx_ce_employee (employee_id)) ENGINE=InnoDB`,
		`CREATE TABLE mc_contact_message_batch_send (id INT UNSIGNED NOT NULL AUTO_INCREMENT, tenant_id INT UNSIGNED NULL, corp_id INT UNSIGNED NOT NULL DEFAULT 0, user_id INT UNSIGNED NOT NULL DEFAULT 0, user_name VARCHAR(255) NOT NULL DEFAULT '', medium_id INT UNSIGNED NOT NULL DEFAULT 0, batch_title VARCHAR(255) NOT NULL DEFAULT '', employee_ids JSON NOT NULL, filter_params JSON NULL, filter_params_detail JSON NULL, content JSON NOT NULL, send_way TINYINT UNSIGNED NOT NULL DEFAULT 1, send_status TINYINT UNSIGNED NOT NULL DEFAULT 0, definite_time DATETIME NULL, send_time DATETIME NULL, send_employee_total INT UNSIGNED NOT NULL DEFAULT 0, send_contact_total INT UNSIGNED NOT NULL DEFAULT 0, send_total INT UNSIGNED NOT NULL DEFAULT 0, not_send_total INT UNSIGNED NOT NULL DEFAULT 0, received_total INT UNSIGNED NOT NULL DEFAULT 0, not_received_total INT UNSIGNED NOT NULL DEFAULT 0, receive_limit_total INT UNSIGNED NOT NULL DEFAULT 0, not_friend_total INT UNSIGNED NOT NULL DEFAULT 0, created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL, deleted_at TIMESTAMP NULL, PRIMARY KEY (id)) ENGINE=InnoDB`,
		`CREATE TABLE mc_contact_message_batch_send_employee (id INT UNSIGNED NOT NULL AUTO_INCREMENT, batch_id INT UNSIGNED NOT NULL DEFAULT 0, employee_id INT UNSIGNED NOT NULL DEFAULT 0, wx_user_id VARCHAR(255) NOT NULL DEFAULT '', send_contact_total INT UNSIGNED NOT NULL DEFAULT 0, created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL, last_sync_time TIMESTAMP NULL, PRIMARY KEY (id)) ENGINE=InnoDB`,
		`CREATE TABLE mc_contact_message_batch_send_result (id INT UNSIGNED NOT NULL AUTO_INCREMENT, batch_id INT UNSIGNED NOT NULL DEFAULT 0, employee_id INT UNSIGNED NOT NULL DEFAULT 0, contact_id INT UNSIGNED NOT NULL DEFAULT 0, external_user_id VARCHAR(255) NOT NULL DEFAULT '', status TINYINT UNSIGNED NOT NULL DEFAULT 0, created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL, PRIMARY KEY (id), KEY idx_result_batch_employee (batch_id, employee_id)) ENGINE=InnoDB`,
		`CREATE TABLE mc_room_message_batch_send (id INT UNSIGNED NOT NULL AUTO_INCREMENT, tenant_id INT UNSIGNED NULL, corp_id INT UNSIGNED NOT NULL DEFAULT 0, user_id INT UNSIGNED NOT NULL DEFAULT 0, employee_ids JSON NOT NULL, content JSON NOT NULL, created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL, deleted_at TIMESTAMP NULL, PRIMARY KEY (id)) ENGINE=InnoDB`,
		`CREATE TABLE mc_work_agent (id INT UNSIGNED NOT NULL AUTO_INCREMENT, corp_id INT UNSIGNED NOT NULL DEFAULT 0, wx_agent_id VARCHAR(255) NOT NULL DEFAULT '', wx_secret VARCHAR(255) NOT NULL DEFAULT '', name VARCHAR(255) NOT NULL DEFAULT '', is_reportenter TINYINT NOT NULL DEFAULT 0, close TINYINT NOT NULL DEFAULT 0, wecom_credentials_ciphertext TEXT NULL, wecom_credentials_key_id VARCHAR(64) NOT NULL DEFAULT '', created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL, deleted_at TIMESTAMP NULL, PRIMARY KEY (id)) ENGINE=InnoDB`,
		`CREATE TABLE mochat_go_saas_tenant_packages (id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT, tenant_id INT UNSIGNED NOT NULL, package_code VARCHAR(64) NOT NULL DEFAULT '', status TINYINT UNSIGNED NOT NULL DEFAULT 1, starts_at TIMESTAMP NULL, expires_at TIMESTAMP NULL, limits_json JSON NOT NULL, created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL, deleted_at TIMESTAMP NULL, PRIMARY KEY (id)) ENGINE=InnoDB`,
		`CREATE TABLE mochat_go_saas_subscriptions (id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT, tenant_id INT UNSIGNED NOT NULL, status VARCHAR(32) NOT NULL DEFAULT 'active', trial_ends_at TIMESTAMP NULL, current_period_ends_at TIMESTAMP NULL, grace_ends_at TIMESTAMP NULL, cancel_at_period_end TINYINT NOT NULL DEFAULT 0, created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL, deleted_at TIMESTAMP NULL, PRIMARY KEY (id)) ENGINE=InnoDB`,
		`CREATE TABLE mochat_go_saas_usage_counters (id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT, tenant_id INT UNSIGNED NOT NULL, metric VARCHAR(64) NOT NULL DEFAULT '', period_key VARCHAR(32) NOT NULL DEFAULT '', limit_value BIGINT UNSIGNED NOT NULL DEFAULT 0, deleted_at TIMESTAMP NULL, PRIMARY KEY (id)) ENGINE=InnoDB`,
		`CREATE TABLE mc_rbac_role (id INT UNSIGNED NOT NULL AUTO_INCREMENT, tenant_id INT UNSIGNED NOT NULL, name VARCHAR(255) NOT NULL DEFAULT '', status TINYINT UNSIGNED NOT NULL DEFAULT 1, data_permission JSON NULL, deleted_at TIMESTAMP NULL, PRIMARY KEY (id)) ENGINE=InnoDB`,
		`CREATE TABLE mochat_go_dashboard_permissions (id INT UNSIGNED NOT NULL AUTO_INCREMENT, code VARCHAR(128) NOT NULL DEFAULT '', name VARCHAR(255) NOT NULL DEFAULT '', status TINYINT UNSIGNED NOT NULL DEFAULT 1, superadmin_only TINYINT NOT NULL DEFAULT 0, deleted_at TIMESTAMP NULL, PRIMARY KEY (id)) ENGINE=InnoDB`,
		`CREATE TABLE mochat_go_dashboard_user_permissions (id INT UNSIGNED NOT NULL AUTO_INCREMENT, tenant_id INT UNSIGNED NOT NULL, user_id INT UNSIGNED NOT NULL, permission_id INT UNSIGNED NOT NULL, effect VARCHAR(16) NOT NULL DEFAULT 'allow', data_scope VARCHAR(32) NOT NULL DEFAULT 'tenant', deleted_at TIMESTAMP NULL, PRIMARY KEY (id)) ENGINE=InnoDB`,
		`CREATE TABLE mochat_go_dashboard_user_roles (id INT UNSIGNED NOT NULL AUTO_INCREMENT, tenant_id INT UNSIGNED NOT NULL, user_id INT UNSIGNED NOT NULL, role_id INT UNSIGNED NOT NULL, deleted_at TIMESTAMP NULL, PRIMARY KEY (id)) ENGINE=InnoDB`,
		`CREATE TABLE mochat_go_dashboard_role_permissions (id INT UNSIGNED NOT NULL AUTO_INCREMENT, tenant_id INT UNSIGNED NOT NULL, role_id INT UNSIGNED NOT NULL, permission_id INT UNSIGNED NOT NULL, data_scope VARCHAR(32) NOT NULL DEFAULT 'tenant', deleted_at TIMESTAMP NULL, PRIMARY KEY (id)) ENGINE=InnoDB`,
		`INSERT INTO mc_tenant (id, name, status) VALUES (11, 'Fixture tenant', 1)`,
		`INSERT INTO mc_corp (id, tenant_id, name, wx_corpid) VALUES (1101, 11, 'Fixture corp', 'wx-corp-1101')`,
		`INSERT INTO mc_user (id, tenant_id, phone, name, status, isSuperAdmin) VALUES (11001, 11, '13800000001', 'admin', 1, 1), (11002, 11, '13800000002', 'operator', 1, 0)`,
		`INSERT INTO mochat_go_dashboard_identities (user_id, tenant_id, login_identifier, password_hash, status, must_rotate_password, auth_version, mfa_required, activated_at) VALUES (11001, 11, '13800000001', '!fixture-hash', 1, 0, 1, 0, NOW()), (11002, 11, '13800000002', '!fixture-hash', 1, 0, 1, 0, NOW())`,
		`INSERT INTO mochat_go_tenant_corp_bindings (tenant_id, corp_id, status, version, verified_wx_corpid, verified_corp_name, verified_at, employee_credential_generation, contact_credential_generation, agent_credential_generation, callback_credential_generation) VALUES (11, 1101, 2, 1, 'wx-corp-1101', 'Fixture corp', NOW(), 1, 1, 1, 1)`,
		`INSERT INTO mc_work_employee (id, wx_user_id, corp_id, name, status) VALUES (101, 'emp-101', 1101, '员工甲', 1), (102, 'emp-102', 1101, '员工乙', 1)`,
		`INSERT INTO mc_work_contact (id, corp_id, wx_external_userid, name) VALUES (201, 1101, 'external-201', '客户甲'), (202, 1101, 'external-202', '客户乙'), (203, 1101, 'external-203', '客户丙')`,
		`INSERT INTO mc_work_contact_employee (contact_id, employee_id, corp_id) VALUES (201, 101, 1101), (202, 101, 1101), (203, 102, 1101)`,
		`INSERT INTO mc_work_agent (id, corp_id, wx_agent_id, name, is_reportenter) VALUES (301, 1101, '100001', 'Fixture agent', 1)`,
		`INSERT INTO mochat_go_dashboard_permissions (id, code, name, status, superadmin_only) VALUES (401, 'dashboard.acquisition.precise_group_send', '客户精准群发', 1, 0)`,
		`INSERT INTO mochat_go_dashboard_user_permissions (tenant_id, user_id, permission_id, effect, data_scope) VALUES (11, 11002, 401, 'allow', 'tenant')`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("contact batch fixture statement: %v\nSQL: %s", err, statement)
		}
	}
	if _, err := db.Exec(`UPDATE mc_corp SET wecom_credentials_ciphertext=?, wecom_credentials_key_id=? WHERE id=1101`, corpCiphertext, corpKeyID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE mc_work_agent SET wecom_credentials_ciphertext=?, wecom_credentials_key_id=? WHERE id=301`, agentCiphertext, agentKeyID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO mochat_go_saas_tenant_packages (tenant_id, package_code, status, starts_at, expires_at, limits_json) VALUES (11, 'fixture', 1, NOW() - INTERVAL 30 DAY, NOW() + INTERVAL 30 DAY, ?)`, contactBatchLimitsJSON()); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO mochat_go_saas_subscriptions (tenant_id, status) VALUES (11, 'active')`); err != nil {
		t.Fatal(err)
	}
}
func contactBatchHarnessAccess(userID int, superadmin bool) dashboard.DashboardAccessContext {
	return dashboard.DashboardAccessContext{
		UserID: userID, TenantID: 11, CorpID: 1101,
		PermissionCodes: []string{"dashboard.acquisition.precise_group_send"}, Scope: dashboard.DataScopeTenant, IsSuperAdmin: superadmin,
	}
}

func contactBatchHarnessInput(h *contactBatchIntegrationHarness, idempotencyKey string, sendWay int, definiteTime string) dashboard.ContactBatchDispatchInput {
	return dashboard.ContactBatchDispatchInput{
		Batch: dashboard.ContactMessageBatchSendWrite{
			CorpID: h.principal.CorpID, UserID: h.principal.UserID, UserName: "admin",
			EmployeeIDs: []int{101}, MediumID: 45,
			FilterParamsJSON: "{}", FilterDetailJSON: "{}",
			Content:     []dashboard.ContactMessageBatchSendContent{{MsgType: "text", Content: "hello"}},
			ContentJSON: `[{"msgType":"text","content":"hello"}]`,
			SendWay:     sendWay, DefiniteTime: definiteTime,
		},
		ContactTargets:   []dashboard.ContactBatchTarget{{EmployeeID: 101, ContactID: 201}, {EmployeeID: 101, ContactID: 202}},
		SenderEmployeeID: 101,
		IdempotencyKey:   idempotencyKey,
		RequestID:        idempotencyKey,
	}
}

func contactBatchRowCounts(t *testing.T, h *contactBatchIntegrationHarness) (batch, operation, dispatch, audit, event int) {
	t.Helper()
	for name, target := range map[string]*int{
		"mc_contact_message_batch_send":               &batch,
		"mochat_go_wecom_capability_operations":       &operation,
		"mochat_go_wecom_capability_dispatches":       &dispatch,
		"mochat_go_wecom_capability_operation_audits": &audit,
		"mochat_go_wecom_capability_operation_events": &event,
	} {
		if err := h.db.QueryRow("SELECT COUNT(*) FROM " + name).Scan(target); err != nil {
			t.Fatalf("count %s: %v", name, err)
		}
	}
	return
}

// TestContactBatchCreateCommitsBusinessOperationDispatchAuditEventAtomically
// (P0-2 scenario 1) verifies the single-transaction create: business row +
// operation + dispatch chunk + create audit/event all commit together.
func TestContactBatchCreateCommitsBusinessOperationDispatchAuditEventAtomically(t *testing.T) {
	h := newContactBatchIntegrationHarness(t)
	result, err := h.store.CreateContactBatchDispatch(context.Background(), h.principal, contactBatchHarnessAccess(h.principal.UserID, h.principal.IsSuperAdmin), contactBatchHarnessInput(h, "atomic-1", 1, ""))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if result.BatchID <= 0 || result.OperationID <= 0 || result.Status != wecomcapability.OperationPending || result.Duplicate {
		t.Fatalf("result=%#v", result)
	}
	batch, operation, dispatch, audit, event := contactBatchRowCounts(t, h)
	if batch != 1 || operation != 1 || dispatch != 1 || audit < 1 || event < 1 {
		t.Fatalf("counts batch=%d operation=%d dispatch=%d audit=%d event=%d", batch, operation, dispatch, audit, event)
	}
	var requestID, capability string
	if err := h.db.QueryRow(`SELECT request_id, capability FROM mochat_go_wecom_capability_operations WHERE id=?`, result.OperationID).Scan(&requestID, &capability); err != nil {
		t.Fatal(err)
	}
	if capability != string(wecomcapability.ContactBatchSend) || requestID != fmt.Sprintf("contact-batch:%d", result.BatchID) {
		t.Fatalf("operation request_id=%q capability=%q", requestID, capability)
	}
	var dispatchKind, dispatchStatus string
	var dispatchTenant, dispatchCorp int
	var dispatchOperation int64
	if err := h.db.QueryRow(`SELECT dispatch_kind, status, tenant_id, corp_id, operation_id FROM mochat_go_wecom_capability_dispatches LIMIT 1`).Scan(&dispatchKind, &dispatchStatus, &dispatchTenant, &dispatchCorp, &dispatchOperation); err != nil {
		t.Fatal(err)
	}
	if dispatchKind != string(wecomcapability.DispatchKindContactBatch) || dispatchStatus != wecomcapability.DispatchQueued || dispatchTenant != h.principal.TenantID || dispatchCorp != h.principal.CorpID || dispatchOperation != result.OperationID {
		t.Fatalf("dispatch kind=%q status=%q tenant=%d corp=%d operation=%d", dispatchKind, dispatchStatus, dispatchTenant, dispatchCorp, dispatchOperation)
	}
}

// TestContactBatchDurableCreatePersistsMediumIDForListShowReadback is the
// P0-1 RED evidence on real MariaDB: the durable create carries mediumId in
// the input and the persisted business row must round-trip it for list/show.
func TestContactBatchDurableCreatePersistsMediumIDForListShowReadback(t *testing.T) {
	h := newContactBatchIntegrationHarness(t)
	result, err := h.store.CreateContactBatchDispatch(context.Background(), h.principal, contactBatchHarnessAccess(h.principal.UserID, h.principal.IsSuperAdmin), contactBatchHarnessInput(h, "p01-medium-1", 1, ""))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	var mediumID int
	if err := h.db.QueryRow(`SELECT medium_id FROM mc_contact_message_batch_send WHERE id=?`, result.BatchID).Scan(&mediumID); err != nil {
		t.Fatal(err)
	}
	if mediumID != 45 {
		t.Fatalf("medium_id=%d, want 45 (P0-1: durable create silently drops medium_id)", mediumID)
	}
}

// TestContactBatchDurableAuditFailureRollsBackWholeCreate (P0-2 scenario 2)
// proves that an event-append failure before any external call rolls the
// entire create back: no business row, operation, dispatch, audit or event.
func TestContactBatchDurableAuditFailureRollsBackWholeCreate(t *testing.T) {
	h := newContactBatchIntegrationHarness(t)
	if _, err := h.db.Exec(`CREATE TRIGGER trg_contact_batch_events_fail BEFORE INSERT ON mochat_go_wecom_capability_operation_events FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'fixture audit failure'`); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = h.db.Exec(`DROP TRIGGER IF EXISTS trg_contact_batch_events_fail`) }()
	_, err := h.store.CreateContactBatchDispatch(context.Background(), h.principal, contactBatchHarnessAccess(h.principal.UserID, h.principal.IsSuperAdmin), contactBatchHarnessInput(h, "rollback-1", 1, ""))
	if err == nil {
		t.Fatal("create unexpectedly succeeded despite event append failure")
	}
	batch, operation, dispatch, audit, event := contactBatchRowCounts(t, h)
	if batch != 0 || operation != 0 || dispatch != 0 || audit != 0 || event != 0 {
		t.Fatalf("rollback left rows batch=%d operation=%d dispatch=%d audit=%d event=%d", batch, operation, dispatch, audit, event)
	}
}

// TestContactBatchDurableIdempotencyReplaysWithoutDuplicateWrites (P0-2
// scenario 3) proves a duplicate idempotency key returns the same
// operation/batch and does not duplicate dispatches or audit/event rows.
func TestContactBatchDurableIdempotencyReplaysWithoutDuplicateWrites(t *testing.T) {
	h := newContactBatchIntegrationHarness(t)
	first, err := h.store.CreateContactBatchDispatch(context.Background(), h.principal, contactBatchHarnessAccess(h.principal.UserID, h.principal.IsSuperAdmin), contactBatchHarnessInput(h, "idem-1", 1, ""))
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	batch1, operation1, dispatch1, audit1, event1 := contactBatchRowCounts(t, h)
	second, err := h.store.CreateContactBatchDispatch(context.Background(), h.principal, contactBatchHarnessAccess(h.principal.UserID, h.principal.IsSuperAdmin), contactBatchHarnessInput(h, "idem-1", 1, ""))
	if err != nil {
		t.Fatalf("duplicate create: %v", err)
	}
	if !second.Duplicate || second.BatchID != first.BatchID || second.OperationID != first.OperationID {
		t.Fatalf("duplicate replay first=%#v second=%#v", first, second)
	}
	batch2, operation2, dispatch2, audit2, event2 := contactBatchRowCounts(t, h)
	if batch1 != batch2 || operation1 != operation2 || dispatch1 != dispatch2 || audit1 != audit2 || event1 != event2 {
		t.Fatalf("duplicate wrote rows before=(%d,%d,%d,%d,%d) after=(%d,%d,%d,%d,%d)", batch1, operation1, dispatch1, audit1, event1, batch2, operation2, dispatch2, audit2, event2)
	}
}

// TestContactBatchDurableZeroWriteOnIsolationViolations (P0-2 scenario 4)
// proves cross-tenant, disabled actor, revoked permission, narrowed scope and
// changed target ownership all fail closed with zero mutations.
func TestContactBatchDurableZeroWriteOnIsolationViolations(t *testing.T) {
	t.Run("cross tenant", func(t *testing.T) {
		h := newContactBatchIntegrationHarness(t)
		other := dashboardprincipal.DashboardPrincipal{TenantID: 22, CorpID: 2201, UserID: 22001, AuthVersion: 1, IsSuperAdmin: true}
		access := dashboard.DashboardAccessContext{UserID: 22001, TenantID: 22, CorpID: 2201, IsSuperAdmin: true, Scope: dashboard.DataScopeTenant}
		input := contactBatchHarnessInput(h, "iso-cross-tenant", 1, "")
		input.Batch.CorpID, input.Batch.UserID = 2201, 22001
		_, err := h.store.CreateContactBatchDispatch(context.Background(), other, access, input)
		if err == nil || !errors.Is(err, dashboard.ErrContactBatchTenantDenied) {
			t.Fatalf("cross tenant err=%v", err)
		}
		if batch, operation, dispatch, _, _ := contactBatchRowCounts(t, h); batch != 0 || operation != 0 || dispatch != 0 {
			t.Fatalf("cross tenant wrote rows batch=%d operation=%d dispatch=%d", batch, operation, dispatch)
		}
	})
	t.Run("disabled actor", func(t *testing.T) {
		h := newContactBatchIntegrationHarness(t)
		principal := dashboardprincipal.DashboardPrincipal{TenantID: 11, CorpID: 1101, UserID: 11002, AuthVersion: 1}
		if _, err := h.db.Exec(`UPDATE mc_user SET status=2 WHERE id=11002`); err != nil {
			t.Fatal(err)
		}
		input := contactBatchHarnessInput(h, "iso-disabled", 1, "")
		input.Batch.UserID = 11002
		_, err := h.store.CreateContactBatchDispatch(context.Background(), principal, contactBatchHarnessAccess(11002, false), input)
		if err == nil || !errors.Is(err, dashboard.ErrContactBatchPermissionDenied) {
			t.Fatalf("disabled actor err=%v", err)
		}
		if batch, operation, dispatch, _, _ := contactBatchRowCounts(t, h); batch != 0 || operation != 0 || dispatch != 0 {
			t.Fatalf("disabled actor wrote rows batch=%d operation=%d dispatch=%d", batch, operation, dispatch)
		}
	})
	t.Run("revoked permission", func(t *testing.T) {
		h := newContactBatchIntegrationHarness(t)
		principal := dashboardprincipal.DashboardPrincipal{TenantID: 11, CorpID: 1101, UserID: 11002, AuthVersion: 1}
		if _, err := h.db.Exec(`DELETE FROM mochat_go_dashboard_user_permissions WHERE tenant_id=11 AND user_id=11002`); err != nil {
			t.Fatal(err)
		}
		input := contactBatchHarnessInput(h, "iso-revoked", 1, "")
		input.Batch.UserID = 11002
		_, err := h.store.CreateContactBatchDispatch(context.Background(), principal, contactBatchHarnessAccess(11002, false), input)
		if err == nil || !errors.Is(err, dashboard.ErrContactBatchPermissionDenied) {
			t.Fatalf("revoked permission err=%v", err)
		}
		if batch, operation, dispatch, _, _ := contactBatchRowCounts(t, h); batch != 0 || operation != 0 || dispatch != 0 {
			t.Fatalf("revoked permission wrote rows batch=%d operation=%d dispatch=%d", batch, operation, dispatch)
		}
	})
	t.Run("narrowed scope", func(t *testing.T) {
		h := newContactBatchIntegrationHarness(t)
		principal := dashboardprincipal.DashboardPrincipal{TenantID: 11, CorpID: 1101, UserID: 11002, AuthVersion: 1}
		if _, err := h.db.Exec(`UPDATE mochat_go_dashboard_user_permissions SET data_scope='self' WHERE tenant_id=11 AND user_id=11002`); err != nil {
			t.Fatal(err)
		}
		input := contactBatchHarnessInput(h, "iso-scope", 1, "")
		input.Batch.UserID = 11002
		_, err := h.store.CreateContactBatchDispatch(context.Background(), principal, contactBatchHarnessAccess(11002, false), input)
		if err == nil || !errors.Is(err, dashboard.ErrContactBatchTargetNotOwned) {
			t.Fatalf("narrowed scope err=%v", err)
		}
		if batch, operation, dispatch, _, _ := contactBatchRowCounts(t, h); batch != 0 || operation != 0 || dispatch != 0 {
			t.Fatalf("narrowed scope wrote rows batch=%d operation=%d dispatch=%d", batch, operation, dispatch)
		}
	})
	t.Run("target ownership changed", func(t *testing.T) {
		h := newContactBatchIntegrationHarness(t)
		if _, err := h.db.Exec(`DELETE FROM mc_work_contact_employee WHERE contact_id=201 AND employee_id=101`); err != nil {
			t.Fatal(err)
		}
		_, err := h.store.CreateContactBatchDispatch(context.Background(), h.principal, contactBatchHarnessAccess(h.principal.UserID, h.principal.IsSuperAdmin), contactBatchHarnessInput(h, "iso-ownership", 1, ""))
		if err == nil || !errors.Is(err, dashboard.ErrContactBatchTargetNotOwned) {
			t.Fatalf("ownership change err=%v", err)
		}
		if batch, operation, dispatch, _, _ := contactBatchRowCounts(t, h); batch != 0 || operation != 0 || dispatch != 0 {
			t.Fatalf("ownership change wrote rows batch=%d operation=%d dispatch=%d", batch, operation, dispatch)
		}
	})
}

// TestContactBatchDurableScheduledBatchIsDueAfterTime (P0-2 scenario 5) proves
// a scheduled batch is not due before its definite_time and becomes due once
// the time passes.
func TestContactBatchDurableScheduledBatchIsDueAfterTime(t *testing.T) {
	h := newContactBatchIntegrationHarness(t)
	if _, err := h.store.CreateContactBatchDispatch(context.Background(), h.principal, contactBatchHarnessAccess(h.principal.UserID, h.principal.IsSuperAdmin), contactBatchHarnessInput(h, "due-future", 2, "2030-01-01 00:00:00")); err != nil {
		t.Fatalf("create: %v", err)
	}
	due, err := h.store.ContactBatchDispatchDue(context.Background(), 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(due) != 0 {
		t.Fatalf("future scheduled batch is due early: %#v", due)
	}
	if _, err := h.db.Exec(`UPDATE mc_contact_message_batch_send SET definite_time=NOW() - INTERVAL 1 MINUTE`); err != nil {
		t.Fatal(err)
	}
	due, err = h.store.ContactBatchDispatchDue(context.Background(), 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(due) != 1 || due[0].Dispatch.DispatchKind != string(wecomcapability.DispatchKindContactBatch) {
		t.Fatalf("due after time=%#v", due)
	}
}

// TestContactBatchLegacyCronExcludesDurableBatches (P0-2 scenario 6) proves
// the legacy cron still claims legacy batches but never durable batches.
func TestContactBatchLegacyCronExcludesDurableBatches(t *testing.T) {
	h := newContactBatchIntegrationHarness(t)
	if _, err := h.store.CreateContactBatchDispatch(context.Background(), h.principal, contactBatchHarnessAccess(h.principal.UserID, h.principal.IsSuperAdmin), contactBatchHarnessInput(h, "legacy-excl", 2, "2000-01-01 00:00:00")); err != nil {
		t.Fatalf("create durable: %v", err)
	}
	legacySeed := `INSERT INTO mc_contact_message_batch_send (tenant_id,corp_id,user_id,user_name,employee_ids,content,send_way,send_status,definite_time,batch_title,created_at,updated_at) VALUES (11,1101,11001,'legacy','[101]','[{"msgType":"text","content":"legacy"}]',2,0,NOW() - INTERVAL 1 HOUR,'legacy batch',NOW(),NOW())`
	if _, err := h.db.Exec(legacySeed); err != nil {
		t.Fatal(err)
	}
	ids, err := h.store.DueContactMessageBatchSendIDs(context.Background(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 {
		t.Fatalf("legacy due ids=%v, want only the legacy batch", ids)
	}
	for _, id := range ids {
		var title string
		if err := h.db.QueryRow(`SELECT batch_title FROM mc_contact_message_batch_send WHERE id=?`, id).Scan(&title); err != nil {
			t.Fatal(err)
		}
		if title != "legacy batch" {
			t.Fatalf("durable batch %d leaked into legacy cron (title=%q)", id, title)
		}
	}
}
func contactBatchDispatchPrincipal(h *contactBatchIntegrationHarness) wecomcapability.DispatchPrincipal {
	return wecomcapability.DispatchPrincipal{
		UserID: h.principal.UserID, TenantID: h.principal.TenantID, CorpID: h.principal.CorpID,
		IsSuperAdmin: h.principal.IsSuperAdmin, AuthVersion: h.principal.AuthVersion,
	}
}

func contactBatchDispatchIDs(t *testing.T, h *contactBatchIntegrationHarness, operationID int64) []int64 {
	t.Helper()
	rows, err := h.db.Query(`SELECT id FROM mochat_go_wecom_capability_dispatches WHERE tenant_id=? AND corp_id=? AND operation_id=? ORDER BY id ASC`, h.principal.TenantID, h.principal.CorpID, operationID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return ids
}

func driveContactBatchDispatchToSubmitted(t *testing.T, h *contactBatchIntegrationHarness, dispatchID int64) {
	t.Helper()
	principal := contactBatchDispatchPrincipal(h)
	claimed, err := h.store.ClaimDispatch(context.Background(), wecomcapability.DispatchClaimRequest{Principal: principal, DispatchID: dispatchID, ExpectedCredentialVersion: 1, LeaseDuration: 5 * time.Minute})
	if err != nil {
		t.Fatalf("claim dispatch %d: %v", dispatchID, err)
	}
	for _, status := range []string{wecomcapability.DispatchSubmitting, wecomcapability.DispatchSubmitted} {
		if _, err := h.store.TransitionDispatch(context.Background(), wecomcapability.DispatchTransitionRequest{
			Principal: principal, DispatchID: dispatchID, Status: status, LeaseToken: claimed.LeaseToken, Attempt: claimed.Attempt,
			ProviderMessageID: fmt.Sprintf("msg-%d", dispatchID),
		}); err != nil {
			t.Fatalf("transition %s dispatch %d: %v", status, dispatchID, err)
		}
	}
}

func driveContactBatchDispatchToTerminal(t *testing.T, h *contactBatchIntegrationHarness, dispatchID int64, terminal, targetID, errorCode string) {
	t.Helper()
	principal := contactBatchDispatchPrincipal(h)
	claimed, err := h.store.ClaimDispatch(context.Background(), wecomcapability.DispatchClaimRequest{Principal: principal, DispatchID: dispatchID, ExpectedCredentialVersion: 1, LeaseDuration: 5 * time.Minute})
	if err != nil {
		t.Fatalf("claim dispatch %d: %v", dispatchID, err)
	}
	transition := func(status string) {
		_, err := h.store.TransitionDispatch(context.Background(), wecomcapability.DispatchTransitionRequest{
			Principal: principal, DispatchID: dispatchID, Status: status, LeaseToken: claimed.LeaseToken, Attempt: claimed.Attempt,
			ProviderMessageID: fmt.Sprintf("msg-%d", dispatchID), LastErrorCode: errorCode,
		})
		if err != nil {
			t.Fatalf("transition %s dispatch %d: %v", status, dispatchID, err)
		}
	}
	transition(wecomcapability.DispatchSubmitting)
	transition(wecomcapability.DispatchSubmitted)
	transition(wecomcapability.DispatchPolling)
	if _, err := h.store.RecordDispatchResult(context.Background(), wecomcapability.DispatchResultRequest{
		Principal: principal, DispatchID: dispatchID, LeaseToken: claimed.LeaseToken, Attempt: claimed.Attempt,
		TargetKind: "employee_external_userid", TargetID: targetID, Status: terminal, ProviderTargetID: "sender-1", ErrorCode: errorCode,
	}); err != nil {
		t.Fatalf("record result dispatch %d: %v", dispatchID, err)
	}
	transition(terminal)
}

// TestContactBatchDurableAggregationPartialSuccessFailure (P0-2 scenario 7)
// drives one dispatch to success and one to failure through the production
// ledger methods and verifies the durable view aggregates partial_failed with
// per-target results.
func TestContactBatchDurableAggregationPartialSuccessFailure(t *testing.T) {
	h := newContactBatchIntegrationHarness(t)
	input := contactBatchHarnessInput(h, "agg-1", 1, "")
	input.Batch.EmployeeIDs = []int{101, 102}
	input.ContactTargets = []dashboard.ContactBatchTarget{{EmployeeID: 101, ContactID: 201}, {EmployeeID: 102, ContactID: 203}}
	result, err := h.store.CreateContactBatchDispatch(context.Background(), h.principal, contactBatchHarnessAccess(h.principal.UserID, h.principal.IsSuperAdmin), input)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	ids := contactBatchDispatchIDs(t, h, result.OperationID)
	if len(ids) != 2 {
		t.Fatalf("dispatch count=%d, want 2 (one per employee)", len(ids))
	}
	driveContactBatchDispatchToTerminal(t, h, ids[0], wecomcapability.DispatchSucceeded, "101:external-201", "")
	driveContactBatchDispatchToTerminal(t, h, ids[1], wecomcapability.DispatchFailed, "102:external-203", "wecom.http_401")
	view, found, err := h.store.ContactBatchDurableView(context.Background(), h.principal, int(result.BatchID))
	if err != nil || !found {
		t.Fatalf("durable view found=%v err=%v", found, err)
	}
	if view.Operation.Status != wecomcapability.OperationPartialFailed {
		t.Fatalf("aggregate status=%s, want partial_failed", view.Operation.Status)
	}
	if len(view.Results) != 2 {
		t.Fatalf("results=%#v, want 2", view.Results)
	}
	got := map[string]string{}
	for _, item := range view.Results {
		got[item.TargetID] = item.Status
	}
	if got["101:external-201"] != wecomcapability.DispatchSucceeded || got["102:external-203"] != wecomcapability.DispatchFailed {
		t.Fatalf("result map=%#v", got)
	}
}

// TestContactBatchDurableCancelFencedByWorkerClaim (P0-2 scenario 8) proves a
// worker claim fences cancel: a claimed dispatch cannot be cancelled and the
// business row is not soft-deleted.
func TestContactBatchDurableCancelFencedByWorkerClaim(t *testing.T) {
	h := newContactBatchIntegrationHarness(t)
	result, err := h.store.CreateContactBatchDispatch(context.Background(), h.principal, contactBatchHarnessAccess(h.principal.UserID, h.principal.IsSuperAdmin), contactBatchHarnessInput(h, "cancel-fence", 1, ""))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	ids := contactBatchDispatchIDs(t, h, result.OperationID)
	if len(ids) != 1 {
		t.Fatalf("dispatch count=%d", len(ids))
	}
	if _, err := h.store.ClaimDispatch(context.Background(), wecomcapability.DispatchClaimRequest{Principal: contactBatchDispatchPrincipal(h), DispatchID: ids[0], ExpectedCredentialVersion: 1, LeaseDuration: 5 * time.Minute}); err != nil {
		t.Fatalf("claim: %v", err)
	}
	err = h.store.CancelContactBatchDurable(context.Background(), h.principal, int(result.BatchID))
	if err == nil || !errors.Is(err, dashboard.ErrContactBatchConflict) {
		t.Fatalf("cancel after claim err=%v", err)
	}
	var dispatchStatus, operationStatus string
	var deletedAt sql.NullTime
	if err := h.db.QueryRow(`SELECT status FROM mochat_go_wecom_capability_dispatches WHERE id=?`, ids[0]).Scan(&dispatchStatus); err != nil {
		t.Fatal(err)
	}
	if dispatchStatus != wecomcapability.DispatchClaimed {
		t.Fatalf("dispatch status=%s, want claimed", dispatchStatus)
	}
	if err := h.db.QueryRow(`SELECT status FROM mochat_go_wecom_capability_operations WHERE id=?`, result.OperationID).Scan(&operationStatus); err != nil {
		t.Fatal(err)
	}
	if operationStatus != wecomcapability.OperationPending {
		t.Fatalf("operation status=%s, want pending", operationStatus)
	}
	if err := h.db.QueryRow(`SELECT deleted_at FROM mc_contact_message_batch_send WHERE id=?`, result.BatchID).Scan(&deletedAt); err != nil {
		t.Fatal(err)
	}
	if deletedAt.Valid {
		t.Fatal("business row was soft-deleted despite cancel being fenced")
	}
}

// TestContactBatchDurableCancelSucceedsWhenQueued proves cancel works only
// from a safe queued state and soft-deletes the business row.
func TestContactBatchDurableCancelSucceedsWhenQueued(t *testing.T) {
	h := newContactBatchIntegrationHarness(t)
	result, err := h.store.CreateContactBatchDispatch(context.Background(), h.principal, contactBatchHarnessAccess(h.principal.UserID, h.principal.IsSuperAdmin), contactBatchHarnessInput(h, "cancel-ok", 1, ""))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := h.store.CancelContactBatchDurable(context.Background(), h.principal, int(result.BatchID)); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	var operationStatus, dispatchStatus string
	var deletedAt sql.NullTime
	if err := h.db.QueryRow(`SELECT status FROM mochat_go_wecom_capability_operations WHERE id=?`, result.OperationID).Scan(&operationStatus); err != nil {
		t.Fatal(err)
	}
	if operationStatus != wecomcapability.OperationCancelled {
		t.Fatalf("operation status=%s, want cancelled", operationStatus)
	}
	if err := h.db.QueryRow(`SELECT status FROM mochat_go_wecom_capability_dispatches WHERE operation_id=?`, result.OperationID).Scan(&dispatchStatus); err != nil {
		t.Fatal(err)
	}
	if dispatchStatus != wecomcapability.DispatchCancelled {
		t.Fatalf("dispatch status=%s, want cancelled", dispatchStatus)
	}
	if err := h.db.QueryRow(`SELECT deleted_at FROM mc_contact_message_batch_send WHERE id=?`, result.BatchID).Scan(&deletedAt); err != nil {
		t.Fatal(err)
	}
	if !deletedAt.Valid {
		t.Fatal("business row was not soft-deleted after cancel")
	}
}

// TestContactBatchDurableReminderCrashWindowAndPartialFailure (P0-2 scenario 9
// + P0-4) proves: an accepted-but-unrecorded attempt returns reconcile on
// replay (no resend), a completed attempt replays idempotent, a partial
// failure lands partial_failed and a re-prepare returns reconcile.
func TestContactBatchDurableReminderCrashWindowAndPartialFailure(t *testing.T) {
	h := newContactBatchIntegrationHarness(t)
	input := contactBatchHarnessInput(h, "remind-1", 1, "")
	input.Batch.EmployeeIDs = []int{101, 102}
	input.ContactTargets = []dashboard.ContactBatchTarget{{EmployeeID: 101, ContactID: 201}, {EmployeeID: 102, ContactID: 203}}
	result, err := h.store.CreateContactBatchDispatch(context.Background(), h.principal, contactBatchHarnessAccess(h.principal.UserID, h.principal.IsSuperAdmin), input)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	ids := contactBatchDispatchIDs(t, h, result.OperationID)
	if len(ids) != 2 {
		t.Fatalf("dispatch count=%d, want 2", len(ids))
	}
	driveContactBatchDispatchToSubmitted(t, h, ids[0])

	// 1) crash window: attempt prepared, never recorded; replay must reconcile.
	first, err := h.store.PrepareContactBatchDurableReminder(context.Background(), h.principal, int(result.BatchID), 0, "remind-req-1")
	if err != nil {
		t.Fatalf("first prepare: %v", err)
	}
	if first.AlreadyCompleted || first.Attempt != 1 || len(first.Recipients) != 2 || first.LeaseToken == "" {
		t.Fatalf("first reminder=%#v", first)
	}
	if _, err := h.store.PrepareContactBatchDurableReminder(context.Background(), h.principal, int(result.BatchID), 0, "remind-req-1"); !errors.Is(err, dashboard.ErrContactBatchReminderReconcileRequired) {
		t.Fatalf("crash-window replay err=%v, want reconcile", err)
	}

	// 2) record the first attempt as a success; replay becomes idempotent.
	if err := h.store.RecordContactBatchDurableReminder(context.Background(), h.principal, first, "", true, 2, 0); err != nil {
		t.Fatalf("record success: %v", err)
	}
	replay, err := h.store.PrepareContactBatchDurableReminder(context.Background(), h.principal, int(result.BatchID), 0, "remind-req-1")
	if err != nil || !replay.AlreadyCompleted {
		t.Fatalf("completed replay reminder=%#v err=%v", replay, err)
	}

	// 3) partial failure lands partial_failed; re-prepare returns reconcile.
	partial, err := h.store.PrepareContactBatchDurableReminder(context.Background(), h.principal, int(result.BatchID), 0, "remind-req-2")
	if err != nil {
		t.Fatalf("partial prepare: %v", err)
	}
	if err := h.store.RecordContactBatchDurableReminder(context.Background(), h.principal, partial, "wecom.contact_batch_remind_failed", false, 1, 1); err != nil {
		t.Fatalf("record partial: %v", err)
	}
	var partialStatus string
	if err := h.db.QueryRow(`SELECT status FROM mochat_go_wecom_capability_operations WHERE id=?`, partial.OperationID).Scan(&partialStatus); err != nil {
		t.Fatal(err)
	}
	if partialStatus != wecomcapability.OperationPartialFailed {
		t.Fatalf("partial status=%s, want partial_failed", partialStatus)
	}
	if _, err := h.store.PrepareContactBatchDurableReminder(context.Background(), h.principal, int(result.BatchID), 0, "remind-req-2"); !errors.Is(err, dashboard.ErrContactBatchReminderReconcileRequired) {
		t.Fatalf("partial re-prepare err=%v, want reconcile", err)
	}
}
