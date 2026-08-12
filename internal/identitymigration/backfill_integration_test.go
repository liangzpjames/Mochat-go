package identitymigration

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/go-sql-driver/mysql"
	"jiyi/mochat-go/internal/migration"
	"jiyi/mochat-go/internal/wecomcredentials"
)

var credentialIntegrationSchemaSequence atomic.Int64

const credentialIntegrationSchemaPrefix = "mochat_identity_single_corp_identitymigration"

func TestEncryptCredentialsRealMariaDBReadsLockedRowsBeforeWritesAndSkipsEmptyRows(t *testing.T) {
	db := newCredentialIntegrationDB(t)
	for _, statement := range []string{
		`CREATE TABLE mc_corp (id int unsigned NOT NULL AUTO_INCREMENT, tenant_id int NOT NULL, wx_corpid varchar(255) NOT NULL DEFAULT '', employee_secret varchar(255) NOT NULL DEFAULT '', contact_secret varchar(255) NOT NULL DEFAULT '', token varchar(255) NOT NULL DEFAULT '', encoding_aes_key varchar(255) NOT NULL DEFAULT '', chat_secret varchar(255) NOT NULL DEFAULT '', wecom_credentials_ciphertext text NULL, wecom_credentials_key_id varchar(64) NOT NULL DEFAULT '', PRIMARY KEY (id)) ENGINE=InnoDB`,
		`CREATE TABLE mc_work_agent (id int unsigned NOT NULL AUTO_INCREMENT, corp_id int NOT NULL, wx_agent_id varchar(255) NOT NULL DEFAULT '', wx_secret varchar(255) NOT NULL DEFAULT '', wecom_credentials_ciphertext text NULL, wecom_credentials_key_id varchar(64) NOT NULL DEFAULT '', PRIMARY KEY (id)) ENGINE=InnoDB`,
		`CREATE TABLE mochat_go_identity_migration_ledger (id bigint unsigned NOT NULL AUTO_INCREMENT, migration_name varchar(96) NOT NULL, request_id varchar(128) NOT NULL, phase varchar(32) NOT NULL, status varchar(16) NOT NULL, result_json json DEFAULT NULL, created_at timestamp NULL DEFAULT CURRENT_TIMESTAMP, updated_at timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP, PRIMARY KEY (id), UNIQUE KEY uni_identity_migration_ledger_request (migration_name, request_id), KEY idx_identity_migration_ledger_status (migration_name, phase, status, id)) ENGINE=InnoDB`,
		`CREATE TABLE mochat_go_identity_migration_batches (request_id varchar(128) NOT NULL, platform_tenant_id int unsigned NOT NULL, status varchar(16) NOT NULL, mapping_digest char(64) NOT NULL DEFAULT '', script_checksum char(64) NOT NULL, preflight_status varchar(16) NOT NULL DEFAULT 'passed', credential_status varchar(16) NOT NULL DEFAULT 'verified', actor_inventory_status varchar(16) NOT NULL DEFAULT 'verified', migration_source varchar(96) NOT NULL DEFAULT '0130_identity_realms_single_corp_backfill', PRIMARY KEY (request_id)) ENGINE=InnoDB`,
		`CREATE TABLE mochat_go_identity_migration_journal (id bigint unsigned NOT NULL AUTO_INCREMENT, request_id varchar(128) NOT NULL, entity_type varchar(48) NOT NULL, entity_id varchar(128) NOT NULL, before_json json DEFAULT NULL, created_at timestamp NULL DEFAULT CURRENT_TIMESTAMP, PRIMARY KEY (id), UNIQUE KEY uni_identity_migration_journal_entity (request_id, entity_type, entity_id), KEY idx_identity_migration_journal_request (request_id, id)) ENGINE=InnoDB`,
		`INSERT INTO mc_corp (id, tenant_id, wx_corpid, employee_secret, contact_secret, token, encoding_aes_key, chat_secret) VALUES (100, 1, 'corp-100', 'employee-value', 'contact-value', 'callback-value', 'aes-value', 'chat-value'), (101, 1, 'placeholder-101', '', '', '', '', '')`,
		`INSERT INTO mc_work_agent (id, corp_id, wx_agent_id, wx_secret) VALUES (300, 100, 'agent-configured', 'agent-value'), (301, 100, 'agent-empty', '')`,
		`INSERT INTO mochat_go_identity_migration_batches (request_id, platform_tenant_id, status, script_checksum) VALUES ('task8-credentials', 1, 'completed', '0000000000000000000000000000000000000000000000000000000000000000')`,
		`INSERT INTO mochat_go_identity_migration_ledger (migration_name, request_id, phase, status, result_json) VALUES ('0130_identity_realms_single_corp_backfill', 'task8-credentials', 'backfill', 'success', JSON_OBJECT('scriptChecksum', '0000000000000000000000000000000000000000000000000000000000000000'))`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	manager, err := wecomcredentials.NewManager(wecomcredentials.Config{
		EncryptionKey:       strings.Repeat("01", 32),
		EncryptionKeyID:     "task8-test",
		RequireEncryption:   true,
		DedicatedConfigured: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := EncryptCredentials(context.Background(), db, manager, "task8-credentials")
	if err != nil {
		t.Fatal(err)
	}
	if result.CorpRowsWritten != 1 || result.AgentRowsWritten != 1 {
		t.Fatalf("credential rows written corp=%d agent=%d, want 1/1", result.CorpRowsWritten, result.AgentRowsWritten)
	}
	assertCredentialCiphertextState(t, db, "mc_corp", 100, true)
	assertCredentialCiphertextState(t, db, "mc_corp", 101, false)
	assertCredentialCiphertextState(t, db, "mc_work_agent", 300, true)
	assertCredentialCiphertextState(t, db, "mc_work_agent", 301, false)

	var corpCiphertext, corpKeyID string
	if err := db.QueryRow(`SELECT wecom_credentials_ciphertext, wecom_credentials_key_id FROM mc_corp WHERE id=100`).Scan(&corpCiphertext, &corpKeyID); err != nil {
		t.Fatal(err)
	}
	corp, err := manager.DecryptCorp(1, "corp-100", corpKeyID, corpCiphertext)
	if err != nil {
		t.Fatal(err)
	}
	if corp.EmployeeSecret != "employee-value" || corp.ContactSecret != "contact-value" || corp.CallbackToken != "callback-value" || corp.EncodingAESKey != "aes-value" || corp.ChatSecret != "chat-value" {
		t.Fatal("corp credential fields did not round-trip")
	}
	var agentCiphertext, agentKeyID string
	if err := db.QueryRow(`SELECT wecom_credentials_ciphertext, wecom_credentials_key_id FROM mc_work_agent WHERE id=300`).Scan(&agentCiphertext, &agentKeyID); err != nil {
		t.Fatal(err)
	}
	agent, err := manager.DecryptAgent(100, "agent-configured", agentKeyID, agentCiphertext)
	if err != nil || agent.WXSecret != "agent-value" {
		t.Fatal("agent credential did not round-trip")
	}
	var employeeSecret, contactSecret, callbackToken, encodingAESKey, chatSecret, agentSecret string
	if err := db.QueryRow(`SELECT employee_secret, contact_secret, token, encoding_aes_key, chat_secret FROM mc_corp WHERE id=100`).Scan(&employeeSecret, &contactSecret, &callbackToken, &encodingAESKey, &chatSecret); err != nil {
		t.Fatal(err)
	}
	if employeeSecret != "employee-value" || contactSecret != "contact-value" || callbackToken != "callback-value" || encodingAESKey != "aes-value" || chatSecret != "chat-value" {
		t.Fatal("legacy corp plaintext was not retained")
	}
	if err := db.QueryRow(`SELECT wx_secret FROM mc_work_agent WHERE id=300`).Scan(&agentSecret); err != nil {
		t.Fatal(err)
	}
	if agentSecret != "agent-value" {
		t.Fatal("legacy agent plaintext was not retained")
	}
}

func TestEncryptCredentialsRealMariaDBRollsBackAllWritesWhenAgentUpdateFails(t *testing.T) {
	db := newCredentialIntegrationDB(t)
	for _, statement := range []string{
		`CREATE TABLE mc_corp (id int unsigned NOT NULL AUTO_INCREMENT, tenant_id int NOT NULL, wx_corpid varchar(255) NOT NULL DEFAULT '', employee_secret varchar(255) NOT NULL DEFAULT '', contact_secret varchar(255) NOT NULL DEFAULT '', token varchar(255) NOT NULL DEFAULT '', encoding_aes_key varchar(255) NOT NULL DEFAULT '', chat_secret varchar(255) NOT NULL DEFAULT '', wecom_credentials_ciphertext text NULL, wecom_credentials_key_id varchar(64) NOT NULL DEFAULT '', PRIMARY KEY (id)) ENGINE=InnoDB`,
		`CREATE TABLE mc_work_agent (id int unsigned NOT NULL AUTO_INCREMENT, corp_id int NOT NULL, wx_agent_id varchar(255) NOT NULL DEFAULT '', wx_secret varchar(255) NOT NULL DEFAULT '', wecom_credentials_ciphertext text NULL, wecom_credentials_key_id varchar(64) NOT NULL DEFAULT '', PRIMARY KEY (id)) ENGINE=InnoDB`,
		`CREATE TABLE mochat_go_identity_migration_ledger (id bigint unsigned NOT NULL AUTO_INCREMENT, migration_name varchar(96) NOT NULL, request_id varchar(128) NOT NULL, phase varchar(32) NOT NULL, status varchar(16) NOT NULL, result_json json DEFAULT NULL, created_at timestamp NULL DEFAULT CURRENT_TIMESTAMP, updated_at timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP, PRIMARY KEY (id), UNIQUE KEY uni_identity_migration_ledger_request (migration_name, request_id), KEY idx_identity_migration_ledger_status (migration_name, phase, status, id)) ENGINE=InnoDB`,
		`CREATE TABLE mochat_go_identity_migration_batches (request_id varchar(128) NOT NULL, platform_tenant_id int unsigned NOT NULL, status varchar(16) NOT NULL, mapping_digest char(64) NOT NULL DEFAULT '', script_checksum char(64) NOT NULL, preflight_status varchar(16) NOT NULL DEFAULT 'passed', credential_status varchar(16) NOT NULL DEFAULT 'verified', actor_inventory_status varchar(16) NOT NULL DEFAULT 'verified', migration_source varchar(96) NOT NULL DEFAULT '0130_identity_realms_single_corp_backfill', PRIMARY KEY (request_id)) ENGINE=InnoDB`,
		`CREATE TABLE mochat_go_identity_migration_journal (id bigint unsigned NOT NULL AUTO_INCREMENT, request_id varchar(128) NOT NULL, entity_type varchar(48) NOT NULL, entity_id varchar(128) NOT NULL, before_json json DEFAULT NULL, created_at timestamp NULL DEFAULT CURRENT_TIMESTAMP, PRIMARY KEY (id), UNIQUE KEY uni_identity_migration_journal_entity (request_id, entity_type, entity_id), KEY idx_identity_migration_journal_request (request_id, id)) ENGINE=InnoDB`,
		`INSERT INTO mc_corp (id, tenant_id, wx_corpid, employee_secret, contact_secret, token, encoding_aes_key, chat_secret) VALUES (100, 1, 'corp-100', 'employee-value', 'contact-value', 'callback-value', 'aes-value', 'chat-value')`,
		`INSERT INTO mc_work_agent (id, corp_id, wx_agent_id, wx_secret) VALUES (300, 100, 'agent-configured', 'agent-value')`,
		`INSERT INTO mochat_go_identity_migration_batches (request_id, platform_tenant_id, status, script_checksum) VALUES ('task8-rollback', 1, 'completed', '0000000000000000000000000000000000000000000000000000000000000000')`,
		`INSERT INTO mochat_go_identity_migration_ledger (migration_name, request_id, phase, status, result_json) VALUES ('0130_identity_realms_single_corp_backfill', 'task8-rollback', 'backfill', 'success', JSON_OBJECT('scriptChecksum', '0000000000000000000000000000000000000000000000000000000000000000'))`,
		`CREATE TRIGGER task8_fail_agent_credential_update BEFORE UPDATE ON mc_work_agent FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'task8 agent update failure'`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	manager, err := wecomcredentials.NewManager(wecomcredentials.Config{
		EncryptionKey:       strings.Repeat("01", 32),
		EncryptionKeyID:     "task8-test",
		RequireEncryption:   true,
		DedicatedConfigured: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := EncryptCredentials(context.Background(), db, manager, "task8-rollback"); err == nil {
		t.Fatal("agent update failure unexpectedly committed")
	}
	var corpCiphertext, corpKeyID, agentCiphertext, agentKeyID string
	if err := db.QueryRow(`SELECT COALESCE(CAST(wecom_credentials_ciphertext AS CHAR),''), COALESCE(wecom_credentials_key_id,'') FROM mc_corp WHERE id=100`).Scan(&corpCiphertext, &corpKeyID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COALESCE(CAST(wecom_credentials_ciphertext AS CHAR),''), COALESCE(wecom_credentials_key_id,'') FROM mc_work_agent WHERE id=300`).Scan(&agentCiphertext, &agentKeyID); err != nil {
		t.Fatal(err)
	}
	if corpCiphertext != "" || corpKeyID != "" || agentCiphertext != "" || agentKeyID != "" {
		t.Fatal("credential ciphertext writes survived a failed transaction")
	}
	var baseSuccessLedgerCount, encryptSuccessLedgerCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_identity_migration_ledger WHERE migration_name=? AND request_id='task8-rollback' AND phase='backfill' AND status='success'`, migrationSource).Scan(&baseSuccessLedgerCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_identity_migration_ledger WHERE migration_name=? AND request_id='task8-rollback' AND phase='encrypt-credentials' AND status='success'`, migrationSource+"/encrypt-credentials").Scan(&encryptSuccessLedgerCount); err != nil {
		t.Fatal(err)
	}
	if baseSuccessLedgerCount != 1 {
		t.Fatalf("base success ledger rows=%d after rollback, want 1", baseSuccessLedgerCount)
	}
	if encryptSuccessLedgerCount != 0 {
		t.Fatalf("encrypt success ledger rows=%d after rollback, want 0", encryptSuccessLedgerCount)
	}
	var employeeSecret, contactSecret, callbackToken, encodingAESKey, chatSecret, agentSecret string
	if err := db.QueryRow(`SELECT employee_secret, contact_secret, token, encoding_aes_key, chat_secret FROM mc_corp WHERE id=100`).Scan(&employeeSecret, &contactSecret, &callbackToken, &encodingAESKey, &chatSecret); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT wx_secret FROM mc_work_agent WHERE id=300`).Scan(&agentSecret); err != nil {
		t.Fatal(err)
	}
	if employeeSecret != "employee-value" || contactSecret != "contact-value" || callbackToken != "callback-value" || encodingAESKey != "aes-value" || chatSecret != "chat-value" || agentSecret != "agent-value" {
		t.Fatal("legacy plaintext changed after rollback")
	}
}

func TestEncryptCredentialsRequiresTheSameRequestBaseSuccessLedgerBeforeAnyWrite(t *testing.T) {
	db := newCredentialIntegrationDB(t)
	requestID := "task8-ledger-guard"
	for _, statement := range credentialAuthorizationFixtureStatements(requestID, false) {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	manager := newCredentialIntegrationManager(t)

	if _, err := EncryptCredentials(context.Background(), db, manager, requestID); err == nil {
		t.Fatal("completed batch without same-request backfill success ledger unexpectedly encrypted credentials")
	}
	assertCredentialCiphertextState(t, db, "mc_corp", 100, false)
	assertCredentialCiphertextState(t, db, "mc_work_agent", 300, false)

	if _, err := db.Exec(`INSERT INTO mochat_go_identity_migration_ledger (migration_name, request_id, phase, status, result_json) VALUES (?, ?, 'backfill', 'success', JSON_OBJECT('scriptChecksum', ?))`, migrationSource, requestID, strings.Repeat("1", 64)); err != nil {
		t.Fatal(err)
	}
	if _, err := EncryptCredentials(context.Background(), db, manager, requestID); err == nil {
		t.Fatal("base ledger checksum mismatch unexpectedly encrypted credentials")
	}
	assertCredentialCiphertextState(t, db, "mc_corp", 100, false)
	assertCredentialCiphertextState(t, db, "mc_work_agent", 300, false)
}

func TestEncryptCredentialsDownUsesJournalDigestAndKeyAndCanResumeAfterLaterFailure(t *testing.T) {
	db := newCredentialIntegrationDB(t)
	requestID := "task8-down-recovery"
	for _, statement := range credentialAuthorizationFixtureStatements(requestID, true) {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	manager := newCredentialIntegrationManager(t)
	if _, err := EncryptCredentials(context.Background(), db, manager, requestID); err != nil {
		t.Fatal(err)
	}

	var journalJSON string
	if err := db.QueryRow(`SELECT CAST(before_json AS CHAR) FROM mochat_go_identity_migration_journal WHERE request_id=? AND entity_type='corp_credentials' AND entity_id='100'`, requestID).Scan(&journalJSON); err != nil {
		t.Fatal(err)
	}
	var journal struct {
		AfterCiphertextSha256 string `json:"afterCiphertextSha256"`
		WrittenKeyID          string `json:"writtenKeyId"`
	}
	if err := json.Unmarshal([]byte(journalJSON), &journal); err != nil {
		t.Fatal(err)
	}
	if journal.AfterCiphertextSha256 == "" || journal.WrittenKeyID == "" || strings.Contains(journalJSON, "employeeSecret") || strings.Contains(journalJSON, "ciphertext") || strings.Contains(journalJSON, "wxSecret") {
		t.Fatalf("credential journal contains an invalid or sensitive shape: %s", journalJSON)
	}

	if err := execIdentityDownScript(t, db, requestID); err == nil {
		t.Fatal("down unexpectedly ignored the post-clear dashboard delete failure")
	}
	assertCredentialCiphertextState(t, db, "mc_corp", 100, false)
	assertCredentialCiphertextState(t, db, "mc_work_agent", 300, false)
	var credentialJournalCount, dashboardJournalCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_identity_migration_journal WHERE request_id=? AND entity_type IN ('corp_credentials','agent_credentials')`, requestID).Scan(&credentialJournalCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_identity_migration_journal WHERE request_id=? AND entity_type='dashboard_identity'`, requestID).Scan(&dashboardJournalCount); err != nil {
		t.Fatal(err)
	}
	if credentialJournalCount != 0 || dashboardJournalCount != 1 {
		t.Fatalf("after partial down credential journal=%d dashboard journal=%d, want 0/1", credentialJournalCount, dashboardJournalCount)
	}

	if _, err := db.Exec(`DROP TRIGGER task8_fail_dashboard_identity_delete`); err != nil {
		t.Fatal(err)
	}
	if err := execIdentityDownScript(t, db, requestID); err != nil {
		t.Fatal(err)
	}
	var leftover int
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name IN ('mochat_go_identity_migration_batches','mochat_go_identity_migration_journal','mochat_go_identity_migration_ledger')`).Scan(&leftover); err != nil {
		t.Fatal(err)
	}
	if leftover != 0 {
		t.Fatalf("down left migration tables=%d, want 0", leftover)
	}
}

func TestIdentityBackfillDownRejectsTamperedCredentialBeforeMutation(t *testing.T) {
	db := newCredentialIntegrationDB(t)
	requestID := "task8-down-tamper"
	for _, statement := range credentialAuthorizationFixtureStatements(requestID, true) {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	manager := newCredentialIntegrationManager(t)
	if _, err := EncryptCredentials(context.Background(), db, manager, requestID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE mc_corp SET wecom_credentials_ciphertext='tampered' WHERE id=100`); err != nil {
		t.Fatal(err)
	}
	if err := execIdentityDownScript(t, db, requestID); err == nil {
		t.Fatal("down unexpectedly accepted a tampered credential ciphertext")
	}
	var ciphertext, keyID string
	if err := db.QueryRow(`SELECT COALESCE(CAST(wecom_credentials_ciphertext AS CHAR),''), COALESCE(wecom_credentials_key_id,'') FROM mc_corp WHERE id=100`).Scan(&ciphertext, &keyID); err != nil {
		t.Fatal(err)
	}
	if ciphertext != "tampered" || keyID == "" {
		t.Fatal("tampered credential was mutated after preflight failure")
	}
	var journalCount, ledgerCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_identity_migration_journal WHERE request_id=? AND entity_type='corp_credentials'`, requestID).Scan(&journalCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_identity_migration_ledger WHERE request_id=?`, requestID).Scan(&ledgerCount); err != nil {
		t.Fatal(err)
	}
	if journalCount != 1 || ledgerCount != 2 {
		t.Fatalf("tamper failure metadata journal=%d ledger=%d, want 1/2", journalCount, ledgerCount)
	}
}

func credentialAuthorizationFixtureStatements(requestID string, addDashboardDeleteFailure bool) []string {
	statements := []string{
		`CREATE TABLE mc_corp (id int unsigned NOT NULL AUTO_INCREMENT, tenant_id int NOT NULL, wx_corpid varchar(255) NOT NULL DEFAULT '', employee_secret varchar(255) NOT NULL DEFAULT '', contact_secret varchar(255) NOT NULL DEFAULT '', token varchar(255) NOT NULL DEFAULT '', encoding_aes_key varchar(255) NOT NULL DEFAULT '', chat_secret varchar(255) NOT NULL DEFAULT '', wecom_credentials_ciphertext text NULL, wecom_credentials_key_id varchar(64) NOT NULL DEFAULT '', PRIMARY KEY (id)) ENGINE=InnoDB`,
		`CREATE TABLE mc_work_agent (id int unsigned NOT NULL AUTO_INCREMENT, corp_id int NOT NULL, wx_agent_id varchar(255) NOT NULL DEFAULT '', wx_secret varchar(255) NOT NULL DEFAULT '', wecom_credentials_ciphertext text NULL, wecom_credentials_key_id varchar(64) NOT NULL DEFAULT '', PRIMARY KEY (id)) ENGINE=InnoDB`,
		`CREATE TABLE mochat_go_identity_migration_ledger (id bigint unsigned NOT NULL AUTO_INCREMENT, migration_name varchar(96) NOT NULL, request_id varchar(128) NOT NULL, phase varchar(32) NOT NULL, status varchar(16) NOT NULL, result_json json DEFAULT NULL, created_at timestamp NULL DEFAULT CURRENT_TIMESTAMP, updated_at timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP, PRIMARY KEY (id), UNIQUE KEY uni_identity_migration_ledger_request (migration_name, request_id), KEY idx_identity_migration_ledger_status (migration_name, phase, status, id)) ENGINE=InnoDB`,
		`CREATE TABLE mochat_go_identity_migration_batches (request_id varchar(128) NOT NULL, platform_tenant_id int unsigned NOT NULL, status varchar(16) NOT NULL, mapping_digest char(64) NOT NULL DEFAULT '', script_checksum char(64) NOT NULL, preflight_status varchar(16) NOT NULL DEFAULT 'passed', credential_status varchar(16) NOT NULL DEFAULT 'verified', actor_inventory_status varchar(16) NOT NULL DEFAULT 'verified', migration_source varchar(96) NOT NULL DEFAULT '0130_identity_realms_single_corp_backfill', PRIMARY KEY (request_id)) ENGINE=InnoDB`,
		`CREATE TABLE mochat_go_identity_migration_journal (id bigint unsigned NOT NULL AUTO_INCREMENT, request_id varchar(128) NOT NULL, entity_type varchar(48) NOT NULL, entity_id varchar(128) NOT NULL, before_json json DEFAULT NULL, created_at timestamp NULL DEFAULT CURRENT_TIMESTAMP, PRIMARY KEY (id), UNIQUE KEY uni_identity_migration_journal_entity (request_id, entity_type, entity_id), KEY idx_identity_migration_journal_request (request_id, id)) ENGINE=InnoDB`,
		`INSERT INTO mc_corp (id, tenant_id, wx_corpid, employee_secret, contact_secret, token, encoding_aes_key, chat_secret) VALUES (100, 1, 'corp-100', 'employee-value', 'contact-value', 'callback-value', 'aes-value', 'chat-value')`,
		`INSERT INTO mc_work_agent (id, corp_id, wx_agent_id, wx_secret) VALUES (300, 100, 'agent-configured', 'agent-value')`,
		fmt.Sprintf(`INSERT INTO mochat_go_identity_migration_batches (request_id, platform_tenant_id, status, script_checksum) VALUES ('%s', 1, 'completed', '%s')`, requestID, strings.Repeat("0", 64)),
	}
	if addDashboardDeleteFailure {
		statements = append(statements,
			fmt.Sprintf(`INSERT INTO mochat_go_identity_migration_ledger (migration_name, request_id, phase, status, result_json) VALUES ('%s', '%s', 'backfill', 'success', JSON_OBJECT('scriptChecksum', '%s'))`, migrationSource, requestID, strings.Repeat("0", 64)),
			`CREATE TABLE mochat_go_dashboard_identities (user_id int unsigned NOT NULL PRIMARY KEY) ENGINE=InnoDB`,
			`INSERT INTO mochat_go_dashboard_identities (user_id) VALUES (900)`,
			fmt.Sprintf(`INSERT INTO mochat_go_identity_migration_journal (request_id, entity_type, entity_id, before_json) VALUES ('%s', 'dashboard_identity', '900', NULL)`, requestID),
			`CREATE TRIGGER task8_fail_dashboard_identity_delete BEFORE DELETE ON mochat_go_dashboard_identities FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'task8 dashboard delete failure'`,
		)
	}
	return statements
}

func newCredentialIntegrationManager(t *testing.T) *wecomcredentials.Manager {
	t.Helper()
	manager, err := wecomcredentials.NewManager(wecomcredentials.Config{
		EncryptionKey:       strings.Repeat("01", 32),
		EncryptionKeyID:     "task8-test",
		RequireEncryption:   true,
		DedicatedConfigured: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return manager
}

func execIdentityDownScript(t *testing.T, db *sql.DB, requestID string) error {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("..", "..", "deploy", "standalone", "migrations", "0130_identity_realms_single_corp_backfill.down.sql"))
	if err != nil {
		t.Fatal(err)
	}
	statements, err := migration.SplitSQLStatements(string(body))
	if err != nil {
		t.Fatal(err)
	}
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(context.Background(), `SET @identity_0130_requested_down_request_id = ?`, requestID); err != nil {
		t.Fatal(err)
	}
	for index, statement := range statements {
		if _, err := conn.ExecContext(context.Background(), statement); err != nil {
			return fmt.Errorf("identity down statement %d failed: %w", index, err)
		}
	}
	return nil
}

func assertCredentialCiphertextState(t *testing.T, db *sql.DB, table string, id int64, wantPresent bool) {
	t.Helper()
	var ciphertext string
	if err := db.QueryRow(`SELECT COALESCE(CAST(wecom_credentials_ciphertext AS CHAR),'') FROM `+table+` WHERE id=?`, id).Scan(&ciphertext); err != nil {
		t.Fatal(err)
	}
	if (ciphertext != "") != wantPresent {
		t.Fatalf("%s id=%d ciphertext presence=%t want=%t", table, id, ciphertext != "", wantPresent)
	}
}

func newCredentialIntegrationDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("MOCHAT_GO_MYSQL_INTEGRATION_DSN"))
	if dsn == "" {
		t.Skip("MOCHAT_GO_MYSQL_INTEGRATION_DSN is required for credential MariaDB integration")
	}
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	adminCfg := *cfg
	adminCfg.DBName = ""
	admin, err := sql.Open("mysql", adminCfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("%s_%d_%d", credentialIntegrationSchemaPrefix, os.Getpid(), credentialIntegrationSchemaSequence.Add(1))
	var schemaExists int
	if err := admin.QueryRow("SELECT COUNT(*) FROM information_schema.schemata WHERE schema_name=?", schema).Scan(&schemaExists); err != nil {
		_ = admin.Close()
		t.Fatal(err)
	}
	if schemaExists != 0 {
		_ = admin.Close()
		t.Fatalf("isolated schema already exists: %s", schema)
	}
	if _, err := admin.Exec("CREATE DATABASE `" + schema + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
		_ = admin.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec("DROP DATABASE IF EXISTS `" + schema + "`")
		var leftovers int
		if err := admin.QueryRow("SELECT COUNT(*) FROM information_schema.schemata WHERE schema_name=?", schema).Scan(&leftovers); err != nil {
			t.Errorf("check isolated schema leftovers: %v", err)
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
