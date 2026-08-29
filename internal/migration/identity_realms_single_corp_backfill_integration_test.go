package migration

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	mysqldriver "github.com/go-sql-driver/mysql"

	"jiyi/mochat-go/internal/identitymigration"
)

func TestIdentityRealmsSingleCorpBackfillRealMariaDB(t *testing.T) {
	db := newIdentitySingleCorpMigrationDB(t)
	createIdentitySingleCorpBaseFixture(t, db)
	if _, err := db.Exec(`INSERT INTO mc_tenant (id, name, status, deleted_at) VALUES (3, 'Tenant without corp', 1, NULL)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO mc_user (id, tenant_id, phone, password, name, status, deleted_at, isSuperAdmin) VALUES (11, 1, '13800000002', 'legacy-dashboard-hash', 'Legacy platform admin', 1, NULL, 1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO mc_user (id, tenant_id, phone, password, name, status, deleted_at, isSuperAdmin) VALUES (13, 2, '13800000013', 'legacy-business-hash', 'Business user', 1, NULL, 0)`); err != nil {
		t.Fatal(err)
	}

	if err := execIdentitySingleCorpMigration(t, db, "0129_identity_realms_single_corp_schema.up.sql", false); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO mochat_go_saas_admin_user_access (user_id) VALUES (11)`); err != nil {
		t.Fatal(err)
	}
	if err := applyIdentityBackfillWithEvidence(t, db, "task8-real-1", 1, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE mochat_go_schema_migrations (version varchar(64) NOT NULL, description varchar(255) NOT NULL DEFAULT '', checksum char(64) NOT NULL DEFAULT '', applied_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP, execution_ms int(10) unsigned NOT NULL DEFAULT 0, PRIMARY KEY (version)) ENGINE=InnoDB`); err != nil {
		t.Fatal(err)
	}
	if err := RecordControlledMigration(context.Background(), db, filepath.Join("..", ".."), "0130_identity_realms_single_corp_backfill", "task8-real-1"); err != nil {
		t.Fatalf("record controlled migration: %v", err)
	}
	if _, err := db.Exec(`DELETE FROM mochat_go_schema_migrations WHERE version=?`, "0130_identity_realms_single_corp_backfill"); err != nil {
		t.Fatal(err)
	}
	if err := RecordControlledMigration(context.Background(), db, filepath.Join("..", ".."), "0130_identity_realms_single_corp_backfill", "task8-real-1"); err != nil {
		t.Fatalf("recover standard migration record after ledger commit: %v", err)
	}

	var saasUserID int64
	if err := db.QueryRow(`SELECT id FROM mochat_go_saas_admin_users WHERE id=11`).Scan(&saasUserID); err != nil {
		t.Fatal(err)
	}
	if saasUserID != 11 {
		t.Fatalf("historical SaaS identity id=%d, want preserved legacy id 11", saasUserID)
	}
	var loginName, passwordHash string
	if err := db.QueryRow(`SELECT login_name, password_hash FROM mochat_go_saas_admin_users WHERE id=?`, saasUserID).Scan(&loginName, &passwordHash); err != nil {
		t.Fatal(err)
	}
	if loginName != "legacy-mc-user-11" || passwordHash != "legacy-dashboard-hash" {
		t.Fatalf("historical SaaS identity=%q/%q, want explainable mapped actor and copied hash", loginName, passwordHash)
	}
	var dashboardHash string
	if err := db.QueryRow(`SELECT password_hash FROM mochat_go_dashboard_identities WHERE user_id=13`).Scan(&dashboardHash); err != nil {
		t.Fatal(err)
	}
	if dashboardHash != "legacy-business-hash" {
		t.Fatalf("dashboard identity hash=%q, want historical business hash", dashboardHash)
	}
	var platformDashboardIdentities int
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_dashboard_identities d INNER JOIN mc_user u ON u.id=d.user_id WHERE u.tenant_id=1`).Scan(&platformDashboardIdentities); err != nil {
		t.Fatal(err)
	}
	if platformDashboardIdentities != 0 {
		t.Fatalf("platform tenant dashboard identities=%d, want 0", platformDashboardIdentities)
	}
	var placeholderCorpID int64
	if err := db.QueryRow(`SELECT corp_id FROM mochat_go_tenant_corp_bindings WHERE tenant_id=3`).Scan(&placeholderCorpID); err != nil {
		t.Fatal(err)
	}
	var placeholderName, verifiedName string
	if err := db.QueryRow(`SELECT c.name, b.verified_corp_name FROM mc_corp c INNER JOIN mochat_go_tenant_corp_bindings b ON b.corp_id=c.id WHERE b.tenant_id=3`).Scan(&placeholderName, &verifiedName); err != nil {
		t.Fatal(err)
	}
	if placeholderCorpID <= 0 || !strings.Contains(placeholderName, "Migration placeholder") || verifiedName != "" {
		t.Fatalf("placeholder binding corp=%d name=%q verified=%q", placeholderCorpID, placeholderName, verifiedName)
	}
	var rewrittenActorID int64
	if err := db.QueryRow(`SELECT user_id FROM mochat_go_saas_admin_user_access WHERE user_id=?`, saasUserID).Scan(&rewrittenActorID); err != nil {
		t.Fatal(err)
	}
	if rewrittenActorID != saasUserID {
		t.Fatalf("SaaS access actor id=%d, want mapped id=%d", rewrittenActorID, saasUserID)
	}
	assertIdentityForeignKeyExists(t, db, "mochat_go_saas_admin_user_access", "fk_saas_admin_user_access_identity")
	assertIdentityTableExists(t, db, "mochat_go_identity_migration_ledger")

	if err := execIdentitySingleCorpMigration(t, db, "0130_identity_realms_single_corp_backfill.down.sql", false); err != nil {
		t.Fatal(err)
	}
	assertIdentityForeignKeyMissing(t, db, "mochat_go_saas_admin_user_access", "fk_saas_admin_user_access_identity")
	assertIdentityTableMissing(t, db, "mochat_go_identity_migration_ledger")
	if err := applyIdentityBackfillWithEvidence(t, db, "task8-real-1-reapply", 1, nil); err != nil {
		t.Fatal(err)
	}
	assertIdentityForeignKeyExists(t, db, "mochat_go_saas_admin_user_access", "fk_saas_admin_user_access_identity")
}

func TestIdentityRealmsSingleCorpBackfillRejectsSaaSIdentityConflictBeforeDDL(t *testing.T) {
	db := newIdentitySingleCorpMigrationDB(t)
	createIdentitySingleCorpBaseFixture(t, db)
	if _, err := db.Exec(`INSERT INTO mc_user (id, tenant_id, phone, password, name, status, deleted_at, isSuperAdmin) VALUES (11, 1, '13800000002', 'legacy-dashboard-hash', 'Legacy platform admin', 1, NULL, 1)`); err != nil {
		t.Fatal(err)
	}
	if err := execIdentitySingleCorpMigration(t, db, "0129_identity_realms_single_corp_schema.up.sql", false); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO mochat_go_saas_admin_users (id, login_name, phone, password_hash, name, status, must_rotate_password, auth_version, mfa_required) VALUES (11, 'wrong-login', '13800000009', 'wrong-hash', 'Wrong', 1, 0, 9, 0)`); err != nil {
		t.Fatal(err)
	}
	err := applyIdentityBackfillWithEvidence(t, db, "task8-conflict", 1, nil)
	assertIdentityBackfillError(t, err, "0130 SaaS platform identity phone/login conflict")
	assertIdentityTableMissing(t, db, "mochat_go_identity_migration_ledger")
	assertIdentityForeignKeyMissing(t, db, "mochat_go_saas_admin_user_access", "fk_saas_admin_user_access_identity")
}

func TestIdentityRealmsSingleCorpBackfillDownPreservesPreexistingDashboardIdentity(t *testing.T) {
	db := newIdentitySingleCorpMigrationDB(t)
	createIdentitySingleCorpBaseFixture(t, db)
	if _, err := db.Exec(`INSERT INTO mc_user (id, tenant_id, phone, password, name, status, deleted_at, isSuperAdmin) VALUES (13, 2, '13800000013', 'legacy-business-hash', 'Business user', 1, NULL, 0)`); err != nil {
		t.Fatal(err)
	}
	if err := execIdentitySingleCorpMigration(t, db, "0129_identity_realms_single_corp_schema.up.sql", false); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO mochat_go_dashboard_identities (user_id, login_identifier, password_hash, status, must_rotate_password, auth_version, mfa_required, activated_at) VALUES (13, '13800000013', 'legacy-business-hash', 1, 0, 1, 0, NOW())`); err != nil {
		t.Fatal(err)
	}
	if err := applyIdentityBackfillWithEvidence(t, db, "task8-preexisting-dashboard", 1, nil); err != nil {
		t.Fatal(err)
	}
	if err := execIdentitySingleCorpMigration(t, db, "0130_identity_realms_single_corp_backfill.down.sql", false); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_dashboard_identities WHERE user_id=13`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("pre-existing Dashboard identity count=%d, want 1 after down", count)
	}
}

func TestIdentityRealmsSingleCorpBackfillRejectsDuplicatePlatformActorPhoneBeforeDDL(t *testing.T) {
	db := newIdentitySingleCorpMigrationDB(t)
	createIdentitySingleCorpBaseFixture(t, db)
	if err := execIdentitySingleCorpMigration(t, db, "0129_identity_realms_single_corp_schema.up.sql", false); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO mc_user (id, tenant_id, phone, password, name, status, deleted_at, isSuperAdmin) VALUES (11, 1, '13800000011', 'hash-11', 'Platform one', 1, NULL, 1), (12, 1, '13800000011', 'hash-12', 'Platform two', 1, NULL, 1)`); err != nil {
		t.Fatal(err)
	}
	err := applyIdentityBackfillWithEvidence(t, db, "task8-platform-phone-conflict", 1, nil)
	if !strings.Contains(strings.ToLower(err.Error()), "phone/login conflict") {
		t.Fatalf("error=%v, want platform phone/login conflict", err)
	}
	assertIdentityTableMissing(t, db, "mochat_go_identity_migration_ledger")
}

func TestIdentityRealmsSingleCorpBackfillRejectsBusinessTenantSuperadminAsSaaSActor(t *testing.T) {
	db := newIdentitySingleCorpMigrationDB(t)
	createIdentitySingleCorpBaseFixture(t, db)
	if _, err := db.Exec(`INSERT INTO mc_user (id, tenant_id, phone, password, name, status, deleted_at, isSuperAdmin) VALUES (12, 2, '13800000003', 'business-hash', 'Business superadmin', 1, NULL, 1)`); err != nil {
		t.Fatal(err)
	}
	if err := execIdentitySingleCorpMigration(t, db, "0129_identity_realms_single_corp_schema.up.sql", false); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO mochat_go_saas_admin_user_access (user_id) VALUES (12)`); err != nil {
		t.Fatal(err)
	}
	err := applyIdentityBackfillWithEvidence(t, db, "task8-business-actor", 1, nil)
	if !strings.Contains(strings.ToLower(err.Error()), "business tenant superadmin") {
		t.Fatalf("error=%v, want business tenant superadmin failure", err)
	}
	assertIdentityTableMissing(t, db, "mochat_go_identity_migration_ledger")
}

func TestIdentityRealmsSingleCorpBackfillRejectsMultipleCorpsBeforeDDL(t *testing.T) {
	db := newIdentitySingleCorpMigrationDB(t)
	createIdentitySingleCorpBaseFixture(t, db)
	if _, err := db.Exec(`INSERT INTO mc_corp (id, tenant_id, name) VALUES (101, 1, 'Second corp')`); err != nil {
		t.Fatal(err)
	}
	if err := execIdentitySingleCorpMigration(t, db, "0129_identity_realms_single_corp_schema.up.sql", false); err != nil {
		t.Fatal(err)
	}
	err := applyIdentityBackfillWithEvidence(t, db, "task8-multiple-corps", 1, nil)
	assertIdentityBackfillError(t, err, "0130 multiple valid corp requires signed mapping")
	assertIdentityTableMissing(t, db, "mochat_go_identity_migration_ledger")
	assertIdentityForeignKeyMissing(t, db, "mochat_go_saas_admin_user_access", "fk_saas_admin_user_access_identity")

	if _, err := db.Exec(`DELETE FROM mochat_go_identity_migration_corp_map WHERE request_id=?`, "task8-multiple-corps"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM mochat_go_identity_migration_batches WHERE request_id=?`, "task8-multiple-corps"); err != nil {
		t.Fatal(err)
	}
	if err := applyIdentityBackfillWithEvidence(t, db, "task8-real-multi", 1, []CorpMappingFixture{{TenantID: 1, CorpID: 101}}); err != nil {
		t.Fatal(err)
	}
	var mappedCorpID int64
	if err := db.QueryRow(`SELECT corp_id FROM mochat_go_tenant_corp_bindings WHERE tenant_id=1`).Scan(&mappedCorpID); err != nil {
		t.Fatal(err)
	}
	if mappedCorpID != 101 {
		t.Fatalf("multi-corp tenant binding corp=%d, want signed mapping corp=101", mappedCorpID)
	}
}

func TestIdentityRealmsSingleCorpBackfillDownToleratesPartialDDL(t *testing.T) {
	db := newIdentitySingleCorpMigrationDB(t)
	createIdentitySingleCorpBaseFixture(t, db)
	if err := execIdentitySingleCorpMigration(t, db, "0129_identity_realms_single_corp_schema.up.sql", false); err != nil {
		t.Fatal(err)
	}
	if err := applyIdentityBackfillWithEvidence(t, db, "task8-partial", 1, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`ALTER TABLE mochat_go_saas_admin_user_access DROP FOREIGN KEY fk_saas_admin_user_access_identity`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DROP TABLE mochat_go_identity_migration_ledger`); err != nil {
		t.Fatal(err)
	}
	if err := execIdentitySingleCorpMigration(t, db, "0130_identity_realms_single_corp_backfill.down.sql", false); err != nil {
		t.Fatal(err)
	}
}

type CorpMappingFixture struct {
	TenantID int64
	CorpID   int64
}

func applyIdentityBackfillWithEvidence(t *testing.T, db *sql.DB, requestID string, platformTenantID int64, mappings []CorpMappingFixture) error {
	t.Helper()
	conn, err := db.Conn(context.Background())
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(context.Background(), `SET @identity_0130_requested_request_id = ?`, requestID); err != nil {
		return err
	}
	mappingDocument := identitymigration.MappingDocument{SignatureVerified: len(mappings) > 0}
	for _, mapping := range mappings {
		mappingDocument.Entries = append(mappingDocument.Entries, identitymigration.CorpMapping{TenantID: mapping.TenantID, CorpID: mapping.CorpID})
	}
	if err := identitymigration.StageValidatedBatchOnConn(context.Background(), conn, identitymigration.DatabaseOptions{
		PlatformTenantID: platformTenantID,
		RequestID:        requestID,
		ScriptChecksum:   identityBackfillScriptChecksum(t),
		Mapping:          mappingDocument,
	}); err != nil {
		return err
	}
	body, err := os.ReadFile(filepath.Join("..", "..", "deploy", "standalone", "migrations", "0130_identity_realms_single_corp_backfill.up.sql"))
	if err != nil {
		return err
	}
	statements, err := SplitSQLStatements(string(body))
	if err != nil {
		return err
	}
	for _, statement := range statements {
		if _, err := conn.ExecContext(context.Background(), statement); err != nil {
			return err
		}
	}
	return nil
}

func assertIdentityBackfillError(t *testing.T, err error, wantMessage string) {
	t.Helper()
	var mysqlErr *mysqldriver.MySQLError
	if !errors.As(err, &mysqlErr) {
		t.Fatalf("error=%v, want MariaDB SIGNAL SQLSTATE 45000", err)
	}
	if mysqlErr.Number != 1644 || string(mysqlErr.SQLState[:]) != "45000" || mysqlErr.Message != wantMessage {
		t.Fatalf("error number=%d sqlstate=%q message=%q, want 1644/45000/%q", mysqlErr.Number, string(mysqlErr.SQLState[:]), mysqlErr.Message, wantMessage)
	}
}

func identityBackfillScriptChecksum(t *testing.T) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("..", "..", "deploy", "standalone", "migrations", "0130_identity_realms_single_corp_backfill.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func TestIdentityIntegrationHarnessRejectsNonTemporarySchema(t *testing.T) {
	for _, schema := range []string{"mochat_identity_single_corp_task8_1", "business_schema", "mochat_identity_single_corp_"} {
		if err := requireIdentityIntegrationSchemaName(schema); err != nil && strings.HasPrefix(schema, "mochat_identity_single_corp_task8_") {
			t.Fatalf("temporary schema %q rejected: %v", schema, err)
		}
	}
	if err := requireIdentityIntegrationSchemaName("business_schema"); err == nil {
		t.Fatal("integration harness accepted a business schema")
	}
}

func requireIdentityIntegrationSchemaName(schema string) error {
	if !strings.HasPrefix(schema, "mochat_identity_single_corp_") || len(strings.TrimPrefix(schema, "mochat_identity_single_corp_")) == 0 || strings.ContainsAny(schema, "` ;\r\n") {
		return fmt.Errorf("integration schema must use mochat_identity_single_corp_ prefix")
	}
	return nil
}
