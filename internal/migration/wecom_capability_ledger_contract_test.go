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

	"github.com/go-sql-driver/mysql"
)

var weComCapabilityLedgerSchemaSequence atomic.Int64

func loadWeComCapabilityLedgerScripts(t *testing.T) (string, string) {
	t.Helper()
	root := filepath.Join("..", "..")
	up, err := os.ReadFile(filepath.Join(root, "deploy", "standalone", "migrations", "0139_wecom_capability_ledger.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	down, err := os.ReadFile(filepath.Join(root, "deploy", "standalone", "migrations", "0139_wecom_capability_ledger.down.sql"))
	if err != nil {
		t.Fatal(err)
	}
	return string(up), string(down)
}

func TestWeComCapabilityLedgerMigrationIsRegisteredAndSplitSafe(t *testing.T) {
	root := filepath.Join("..", "..")
	migrations := DefaultMigrations(root)
	var found *Migration
	for index := range migrations {
		if migrations[index].Version == "0139_wecom_capability_ledger" {
			found = &migrations[index]
			break
		}
	}
	if found == nil || found.DownPath == "" {
		t.Fatal("0139 capability ledger migration is not registered with a down script")
	}
	up, down := loadWeComCapabilityLedgerScripts(t)
	upStatements, err := SplitSQLStatements(up)
	if err != nil {
		t.Fatal(err)
	}
	downStatements, err := SplitSQLStatements(down)
	if err != nil {
		t.Fatal(err)
	}
	if len(upStatements) < 30 || len(downStatements) < 15 {
		t.Fatalf("0139 statements up=%d down=%d, want complete guards and staged recovery", len(upStatements), len(downStatements))
	}
	if strings.Contains(strings.ToUpper(up), "DELIMITER") || strings.Contains(strings.ToUpper(down), "DELIMITER") {
		t.Fatal("0139 must be executable by the statement splitter without DELIMITER")
	}
	if strings.Contains(strings.ToUpper(up), "ADD COLUMN IF NOT EXISTS") || strings.Contains(strings.ToUpper(up), "ADD INDEX IF NOT EXISTS") || strings.Contains(strings.ToUpper(down), "DROP COLUMN IF EXISTS") || strings.Contains(strings.ToUpper(down), "DROP INDEX IF EXISTS") {
		t.Fatal("0139 must use metadata guards and prepared DDL, not version-incompatible IF EXISTS syntax")
	}
}

func TestWeComCapabilityLedgerPreflightCompletesBeforeFirstDDL(t *testing.T) {
	up, _ := loadWeComCapabilityLedgerScripts(t)
	statements, err := SplitSQLStatements(up)
	if err != nil {
		t.Fatal(err)
	}
	firstDDL := -1
	for index, statement := range statements {
		trimmed := strings.ToUpper(strings.TrimSpace(statement))
		if strings.HasPrefix(trimmed, "ALTER TABLE") || strings.HasPrefix(trimmed, "CREATE TABLE") {
			firstDDL = index
			break
		}
	}
	if firstDDL < 0 {
		t.Fatal("0139 has no DDL")
	}
	preflight := strings.ToLower(strings.Join(statements[:firstDDL], "\n"))
	for _, required := range []string{
		"mc_contact_message_batch_send", "mc_room_message_batch_send",
		"mochat_go_tenant_corp_bindings",
		"mochat_go_wecom_capability_operations",
		"mochat_go_wecom_capability_dispatches",
		"mochat_go_wecom_capability_operation_results",
		"mochat_go_wecom_capability_operation_audits",
		"mochat_go_wecom_capability_operation_events",
		"signal sqlstate",
	} {
		if !strings.Contains(preflight, required) {
			t.Fatalf("first DDL is not preceded by a complete guard for %q", required)
		}
	}
}

func TestWeComCapabilityLedgerUsesPortableStringTargetsAndIndependentGenerations(t *testing.T) {
	up, down := loadWeComCapabilityLedgerScripts(t)
	lower := strings.ToLower(up)
	for _, required := range []string{
		"employee_credential_generation", "contact_credential_generation",
		"agent_credential_generation", "callback_credential_generation",
		"credential_generation", "provider_object_id", "actual_agent_id",
		"external_success", "callback_evidence", "request_id", "lease_token",
	} {
		if !strings.Contains(lower, required) {
			t.Fatalf("0139 missing required field %q", required)
		}
	}
	for _, forbidden := range []string{
		"target_id bigint", "provider_target_id bigint", "employee_secret",
		"contact_secret", "wx_secret", "callback_token", "encoding_aes_key",
		"payload", "raw_response",
	} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("0139 contains forbidden schema contract %q", forbidden)
		}
	}
	if !strings.Contains(strings.ToLower(down), "employee_credential_generation") ||
		!strings.Contains(strings.ToLower(down), "callback_credential_generation") {
		t.Fatal("0139 down does not restore all four generation columns")
	}
	if !strings.Contains(lower, "unique key uk_wecom_capability_operation_idempotency (tenant_id,corp_id,capability,action,credential_generation,idempotency_key)") {
		t.Fatal("operation idempotency scope must include action")
	}
}

func TestWeComCapabilityLedgerDoesNotPinBigintNumericPrecision(t *testing.T) {
	up, down := loadWeComCapabilityLedgerScripts(t)
	for name, script := range map[string]string{"up": up, "down": down} {
		if strings.Contains(strings.ToLower(script), "data_type = 'bigint' and numeric_precision = 19") {
			t.Fatalf("0139 %s guard pins BIGINT numeric precision/display width", name)
		}
	}
}

func TestWeComCapabilityLedgerRealRunnerApplyDownApply(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("MOCHAT_GO_MYSQL_INTEGRATION_DSN"))
	if dsn == "" {
		t.Skip("SKIP: MOCHAT_GO_MYSQL_INTEGRATION_DSN is not set; isolated MariaDB DSN is required")
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
	if err := admin.PingContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close() })
	schema := fmt.Sprintf("mochat_wecom_0139_%d_%d", os.Getpid(), weComCapabilityLedgerSchemaSequence.Add(1))
	if _, err := admin.Exec("CREATE DATABASE `" + schema + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec("DROP DATABASE IF EXISTS `" + schema + "`"); err != nil {
			t.Errorf("drop temporary schema: %v", err)
		}
	})
	testCfg := *cfg
	testCfg.DBName = schema
	db, err := sql.Open("mysql", testCfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.PingContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	createWeComCapabilityLedgerPreMigrationFixture(t, db)
	root := filepath.Join("..", "..")
	migration := Migration{
		Version:     "0139_wecom_capability_ledger",
		Description: "wecom capability ledger",
		Path:        filepath.Join(root, "deploy", "standalone", "migrations", "0139_wecom_capability_ledger.up.sql"),
		DownPath:    filepath.Join(root, "deploy", "standalone", "migrations", "0139_wecom_capability_ledger.down.sql"),
	}
	runner, err := NewRunner(db, []Migration{migration})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runner.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertWeComCapabilityLedgerSchema(t, db, true)
	if _, err := runner.RollbackLast(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertWeComCapabilityLedgerSchema(t, db, false)
	if _, err := runner.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertWeComCapabilityLedgerSchema(t, db, true)
	if _, err := runner.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertWeComCapabilityLedgerSchema(t, db, true)
	var leftovers int
	if err := admin.QueryRow(`SELECT COUNT(*) FROM information_schema.schemata WHERE schema_name=?`, schema).Scan(&leftovers); err != nil {
		t.Fatal(err)
	}
	if leftovers != 1 {
		t.Fatalf("temporary schema existence=%d before cleanup, want 1", leftovers)
	}
}

func TestWeComCapabilityLedgerRejectsInvalidParentDataBeforeDDL(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(t *testing.T, db *sql.DB)
	}{
		{
			name: "dangling corp",
			setup: func(t *testing.T, db *sql.DB) {
				t.Helper()
				if _, err := db.Exec(`INSERT INTO mc_contact_message_batch_send(corp_id,user_id,employee_ids,content) VALUES (999999,101,'[]','{}')`); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "existing tenant cross tenant",
			setup: func(t *testing.T, db *sql.DB) {
				t.Helper()
				if _, err := db.Exec(`ALTER TABLE mc_contact_message_batch_send ADD COLUMN tenant_id INT UNSIGNED NULL`); err != nil {
					t.Fatal(err)
				}
				if _, err := db.Exec(`UPDATE mc_contact_message_batch_send SET tenant_id=22 WHERE id=1`); err != nil {
					t.Fatal(err)
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			withTemporaryWeComCapabilityLedgerSchema(t, func(db *sql.DB, root string) {
				createWeComCapabilityLedgerPreMigrationFixture(t, db)
				tc.setup(t, db)
				runner, err := NewRunner(db, []Migration{{
					Version:     "0139_wecom_capability_ledger",
					Description: "wecom capability ledger",
					Path:        filepath.Join(root, "deploy", "standalone", "migrations", "0139_wecom_capability_ledger.up.sql"),
					DownPath:    filepath.Join(root, "deploy", "standalone", "migrations", "0139_wecom_capability_ledger.down.sql"),
				}})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := runner.Apply(context.Background()); err == nil || !strings.Contains(err.Error(), "0139 incompatible batch tenant data") {
					t.Fatalf("invalid parent data error=%v, want controlled batch tenant guard", err)
				}
				var ledgerTables, generations, contactTenant int
				if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name IN ('mochat_go_wecom_capability_operations','mochat_go_wecom_capability_dispatches','mochat_go_wecom_capability_operation_results','mochat_go_wecom_capability_operation_audits','mochat_go_wecom_capability_operation_events')`).Scan(&ledgerTables); err != nil {
					t.Fatal(err)
				}
				if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='mochat_go_tenant_corp_bindings' AND column_name IN ('employee_credential_generation','contact_credential_generation','agent_credential_generation','callback_credential_generation')`).Scan(&generations); err != nil {
					t.Fatal(err)
				}
				if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='mc_contact_message_batch_send' AND column_name='tenant_id'`).Scan(&contactTenant); err != nil {
					t.Fatal(err)
				}
				if ledgerTables != 0 || generations != 0 || (tc.name == "dangling corp" && contactTenant != 0) {
					t.Fatalf("invalid parent data changed schema: ledger=%d generations=%d contactTenant=%d", ledgerTables, generations, contactTenant)
				}
			})
		})
	}
}

func TestWeComCapabilityLedgerRejectsIncompatibleExistingTenantColumnBeforeDDL(t *testing.T) {
	withTemporaryWeComCapabilityLedgerSchema(t, func(db *sql.DB, root string) {
		createWeComCapabilityLedgerPreMigrationFixture(t, db)
		if _, err := db.Exec(`ALTER TABLE mc_contact_message_batch_send ADD COLUMN tenant_id BIGINT NOT NULL DEFAULT 0`); err != nil {
			t.Fatal(err)
		}
		runner := newWeComCapabilityLedgerTestRunner(t, db, root)
		if _, err := runner.Apply(context.Background()); err == nil || !strings.Contains(err.Error(), "0139 incompatible parent or dependency schema") {
			t.Fatalf("incompatible existing tenant column error=%v", err)
		}
		var generations, ledgerTables int
		if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='mochat_go_tenant_corp_bindings' AND column_name IN ('employee_credential_generation','contact_credential_generation','agent_credential_generation','callback_credential_generation')`).Scan(&generations); err != nil {
			t.Fatal(err)
		}
		if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name='mochat_go_wecom_capability_operations'`).Scan(&ledgerTables); err != nil {
			t.Fatal(err)
		}
		if generations != 0 || ledgerTables != 0 {
			t.Fatalf("incompatible existing tenant column changed migration schema: generations=%d operations=%d", generations, ledgerTables)
		}
	})
}

func TestWeComCapabilityLedgerRejectsIncompatibleResidualBeforeDDL(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(t *testing.T, db *sql.DB)
		want   string
	}{
		{
			name: "operation unique missing action",
			mutate: func(t *testing.T, db *sql.DB) {
				t.Helper()
				if _, err := db.Exec(`ALTER TABLE mochat_go_wecom_capability_operations DROP INDEX uk_wecom_capability_operation_idempotency, ADD UNIQUE KEY uk_wecom_capability_operation_idempotency (tenant_id,corp_id,capability,credential_generation,idempotency_key)`); err != nil {
					t.Fatal(err)
				}
			},
			want: "0139 incompatible capability operations table",
		},
		{
			name: "operation actor foreign key missing",
			mutate: func(t *testing.T, db *sql.DB) {
				t.Helper()
				if _, err := db.Exec(`ALTER TABLE mochat_go_wecom_capability_operations DROP FOREIGN KEY fk_wecom_capability_operation_actor`); err != nil {
					t.Fatal(err)
				}
			},
			want: "0139 incompatible capability operations table",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			withTemporaryWeComCapabilityLedgerSchema(t, func(db *sql.DB, root string) {
				createWeComCapabilityLedgerPreMigrationFixture(t, db)
				runner := newWeComCapabilityLedgerTestRunner(t, db, root)
				if _, err := runner.Apply(context.Background()); err != nil {
					t.Fatal(err)
				}
				tc.mutate(t, db)
				if _, err := db.Exec(`DELETE FROM mochat_go_schema_migrations WHERE version='0139_wecom_capability_ledger'`); err != nil {
					t.Fatal(err)
				}
				if _, err := runner.Apply(context.Background()); err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("residual guard error=%v, want %q", err, tc.want)
				}
			})
		})
	}
}

func TestWeComCapabilityLedgerDownRecoversPartialStateThenReapplies(t *testing.T) {
	withTemporaryWeComCapabilityLedgerSchema(t, func(db *sql.DB, root string) {
		createWeComCapabilityLedgerPreMigrationFixture(t, db)
		runner := newWeComCapabilityLedgerTestRunner(t, db, root)
		if _, err := runner.Apply(context.Background()); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`ALTER TABLE mochat_go_tenant_corp_bindings DROP COLUMN employee_credential_generation, DROP COLUMN contact_credential_generation`); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`DROP TABLE mochat_go_wecom_capability_operation_events`); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`ALTER TABLE mc_contact_message_batch_send DROP FOREIGN KEY fk_wecom_contact_batch_corp`); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`ALTER TABLE mc_contact_message_batch_send DROP INDEX uk_wecom_contact_batch_scope`); err != nil {
			t.Fatal(err)
		}
		if _, err := runner.RollbackLast(context.Background()); err != nil {
			t.Fatalf("partial down: %v", err)
		}
		assertWeComCapabilityLedgerSchema(t, db, false)
		if _, err := runner.Apply(context.Background()); err != nil {
			t.Fatalf("reapply after partial down: %v", err)
		}
		assertWeComCapabilityLedgerSchema(t, db, true)
	})
}

func TestWeComCapabilityLedgerDownRejectsExternalParentDependency(t *testing.T) {
	withTemporaryWeComCapabilityLedgerSchema(t, func(db *sql.DB, root string) {
		createWeComCapabilityLedgerPreMigrationFixture(t, db)
		runner := newWeComCapabilityLedgerTestRunner(t, db, root)
		if _, err := runner.Apply(context.Background()); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`ALTER TABLE mc_contact_message_batch_send ADD KEY external_contact_tenant (tenant_id)`); err != nil {
			t.Fatal(err)
		}
		if _, err := runner.RollbackLast(context.Background()); err == nil || !strings.Contains(err.Error(), "0139 incompatible rollback residual") {
			t.Fatalf("external parent dependency rollback error=%v", err)
		}
		var exists int
		if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name='mochat_go_wecom_capability_operations'`).Scan(&exists); err != nil {
			t.Fatal(err)
		}
		if exists != 1 {
			t.Fatalf("rollback guard changed ledger tables: operations=%d", exists)
		}
	})
}

func newWeComCapabilityLedgerTestRunner(t *testing.T, db *sql.DB, root string) *Runner {
	t.Helper()
	runner, err := NewRunner(db, []Migration{{
		Version:     "0139_wecom_capability_ledger",
		Description: "wecom capability ledger",
		Path:        filepath.Join(root, "deploy", "standalone", "migrations", "0139_wecom_capability_ledger.up.sql"),
		DownPath:    filepath.Join(root, "deploy", "standalone", "migrations", "0139_wecom_capability_ledger.down.sql"),
	}})
	if err != nil {
		t.Fatal(err)
	}
	return runner
}

func withTemporaryWeComCapabilityLedgerSchema(t *testing.T, fn func(db *sql.DB, root string)) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("MOCHAT_GO_MYSQL_INTEGRATION_DSN"))
	if dsn == "" {
		t.Skip("SKIP: MOCHAT_GO_MYSQL_INTEGRATION_DSN is not set; isolated MariaDB DSN is required")
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
	if err := admin.PingContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close() })
	schema := fmt.Sprintf("mochat_wecom_0139_invalid_%d_%d", os.Getpid(), weComCapabilityLedgerSchemaSequence.Add(1))
	if _, err := admin.Exec("CREATE DATABASE `" + schema + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec("DROP DATABASE IF EXISTS `" + schema + "`"); err != nil {
			t.Errorf("drop temporary schema: %v", err)
		}
	})
	testCfg := *cfg
	testCfg.DBName = schema
	db, err := sql.Open("mysql", testCfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.PingContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	fn(db, filepath.Join("..", ".."))
}

func createWeComCapabilityLedgerPreMigrationFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	statements := []string{
		`CREATE TABLE mc_tenant (id INT UNSIGNED NOT NULL PRIMARY KEY) ENGINE=InnoDB`,
		`CREATE TABLE mc_user (id INT UNSIGNED NOT NULL, tenant_id INT UNSIGNED NOT NULL, PRIMARY KEY (id), UNIQUE KEY uni_dashboard_user_tenant_id_id (tenant_id,id)) ENGINE=InnoDB`,
		`CREATE TABLE mc_corp (id INT UNSIGNED NOT NULL, tenant_id INT UNSIGNED NOT NULL, deleted_at DATETIME NULL, PRIMARY KEY (id), UNIQUE KEY uni_mc_corp_tenant_id_id (tenant_id,id)) ENGINE=InnoDB`,
		`CREATE TABLE mochat_go_tenant_corp_bindings (tenant_id INT UNSIGNED NOT NULL, corp_id INT UNSIGNED NOT NULL, status TINYINT UNSIGNED NOT NULL DEFAULT 1, version BIGINT UNSIGNED NOT NULL DEFAULT 1, verified_wx_corpid VARCHAR(255) NULL, verified_corp_name VARCHAR(255) NOT NULL DEFAULT '', verified_at TIMESTAMP NULL, created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL, PRIMARY KEY (tenant_id), UNIQUE KEY uk_binding_corp (corp_id), CONSTRAINT fk_tenant_corp_binding_corp FOREIGN KEY (tenant_id,corp_id) REFERENCES mc_corp (tenant_id,id)) ENGINE=InnoDB`,
		`CREATE TABLE mc_contact_message_batch_send (id INT UNSIGNED NOT NULL AUTO_INCREMENT, corp_id INT UNSIGNED NOT NULL DEFAULT 0, user_id INT UNSIGNED NOT NULL DEFAULT 0, employee_ids JSON NOT NULL, content JSON NOT NULL, created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL, deleted_at TIMESTAMP NULL, PRIMARY KEY (id)) ENGINE=InnoDB`,
		`CREATE TABLE mc_room_message_batch_send (id INT UNSIGNED NOT NULL AUTO_INCREMENT, corp_id INT UNSIGNED NOT NULL DEFAULT 0, user_id INT UNSIGNED NOT NULL DEFAULT 0, employee_ids JSON NOT NULL, content JSON NOT NULL, created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL, deleted_at TIMESTAMP NULL, PRIMARY KEY (id)) ENGINE=InnoDB`,
		`CREATE TABLE mochat_go_schema_migrations (version VARCHAR(128) NOT NULL PRIMARY KEY, description VARCHAR(255) NOT NULL, checksum CHAR(64) NOT NULL, applied_at DATETIME NOT NULL, execution_ms INT NOT NULL) ENGINE=InnoDB`,
		`INSERT INTO mc_tenant VALUES (11),(22)`,
		`INSERT INTO mc_user(id,tenant_id) VALUES (101,11),(202,22)`,
		`INSERT INTO mc_corp(id,tenant_id) VALUES (1101,11),(2201,22)`,
		`INSERT INTO mochat_go_tenant_corp_bindings(tenant_id,corp_id) VALUES (11,1101),(22,2201)`,
		`INSERT INTO mc_contact_message_batch_send(corp_id,user_id,employee_ids,content) VALUES (1101,101,'[]','{}')`,
		`INSERT INTO mc_room_message_batch_send(corp_id,user_id,employee_ids,content) VALUES (1101,101,'[]','{}')`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("fixture statement %s: %v", statement, err)
		}
	}
}

func assertWeComCapabilityLedgerSchema(t *testing.T, db *sql.DB, wantPresent bool) {
	t.Helper()
	var tableCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name IN ('mochat_go_wecom_capability_operations','mochat_go_wecom_capability_dispatches','mochat_go_wecom_capability_operation_results','mochat_go_wecom_capability_operation_audits','mochat_go_wecom_capability_operation_events')`).Scan(&tableCount); err != nil {
		t.Fatal(err)
	}
	if (tableCount == 5) != wantPresent {
		t.Fatalf("0139 table count=%d wantPresent=%v", tableCount, wantPresent)
	}
	var generationCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='mochat_go_tenant_corp_bindings' AND column_name IN ('employee_credential_generation','contact_credential_generation','agent_credential_generation','callback_credential_generation')`).Scan(&generationCount); err != nil {
		t.Fatal(err)
	}
	if (generationCount == 4) != wantPresent {
		t.Fatalf("generation column count=%d wantPresent=%v", generationCount, wantPresent)
	}
}
