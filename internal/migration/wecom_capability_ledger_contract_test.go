package migration_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"jiyi/mochat-go/internal/integrationtestdb"
	"jiyi/mochat-go/internal/migration"
	"jiyi/mochat-go/internal/migration/testharness"
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

func quoteMigrationIdentifier(value string) string {
	return "`" + strings.ReplaceAll(value, "`", "``") + "`"
}

func TestWeComCapabilityLedgerMigrationIsRegisteredAndSplitSafe(t *testing.T) {
	root := filepath.Join("..", "..")
	migrations := migration.DefaultMigrations(root)
	var found *migration.Migration
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
	upStatements, err := migration.SplitSQLStatements(up)
	if err != nil {
		t.Fatal(err)
	}
	downStatements, err := migration.SplitSQLStatements(down)
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
	statements, err := migration.SplitSQLStatements(up)
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

func TestWeComCapabilityLedgerRollbackAndResidualGuardsAreDiagnosticAndStrict(t *testing.T) {
	up, down := loadWeComCapabilityLedgerScripts(t)
	lowerUp, lowerDown := strings.ToLower(up), strings.ToLower(down)
	for _, required := range []string{
		"auto_increment", "on update current_timestamp(6)",
		"fk_wecom_capability_audit_operation foreign key", "fk_wecom_capability_event_operation foreign key",
	} {
		if !strings.Contains(lowerUp, required) {
			t.Fatalf("0139 up script missing strict signature evidence %q", required)
		}
	}
	for _, required := range []string{
		"delete_rule", "delete_rule <> 'restrict'", "incompatible rollback residual:",
	} {
		if !strings.Contains(lowerDown, required) {
			t.Fatalf("0139 down guard missing diagnostic/signature evidence %q", required)
		}
	}
	if !strings.Contains(lowerUp, "table_name = 'mochat_go_wecom_capability_operations' and constraint_name in ('fk_wecom_capability_operation_corp','fk_wecom_capability_operation_actor') and delete_rule <> 'restrict'") {
		t.Fatal("0139 up operations guard does not require RESTRICT corp/actor foreign keys")
	}
	for name, script := range map[string]string{"up": lowerUp, "down": lowerDown} {
		if !strings.Contains(script, "upper(trim(column_default)) = 'null'") {
			t.Fatalf("0139 %s parent tenant guard must accept MariaDB's string NULL metadata", name)
		}
	}
}

func TestWeComCapabilityLedgerRollbackPreflightsExternalInboundForeignKeys(t *testing.T) {
	_, down := loadWeComCapabilityLedgerScripts(t)
	lowerDown := strings.ToLower(down)
	for _, required := range []string{
		"@wecom_0139_down_unexpected_fk_invalid",
		"unique_constraint_schema = database()",
		"information_schema.key_column_usage",
		"referenced_table_name in",
		"'mochat_go_wecom_capability_operations'",
		"'mochat_go_wecom_capability_dispatches'",
		"0139 rollback blocked by external foreign key",
	} {
		if !strings.Contains(lowerDown, required) {
			t.Fatalf("0139 down lacks external inbound foreign-key preflight %q", required)
		}
	}
}

func TestWeComCapabilityLedgerRealRollbackRejectsExternalInboundForeignKeysBeforeDrop(t *testing.T) {
	withTemporaryWeComCapabilityLedgerSchema(t, func(db *sql.DB, root string) {
		createWeComCapabilityLedgerPreMigrationFixture(t, db)
		runner := newWeComCapabilityLedgerTestRunner(t, db, root)
		if _, err := runner.Apply(context.Background()); err != nil {
			t.Fatal(err)
		}
		var currentSchema string
		if err := db.QueryRow(`SELECT DATABASE()`).Scan(&currentSchema); err != nil {
			t.Fatal(err)
		}
		probeSchema := createWeComCapabilityExternalForeignKeyProbeDatabase(t, db)
		defer func() {
			if _, err := db.Exec("DROP DATABASE IF EXISTS `" + probeSchema + "`"); err != nil {
				t.Errorf("drop cross-schema probe: %v", err)
				return
			}
			var leftovers int
			if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.schemata WHERE schema_name=?`, probeSchema).Scan(&leftovers); err != nil {
				t.Errorf("check cross-schema probe cleanup: %v", err)
			} else if leftovers != 0 {
				t.Errorf("cross-schema probe leftovers=%d", leftovers)
			}
		}()
		createWeComCapabilityExternalForeignKeyProbe(t, db, probeSchema, currentSchema)
		if _, err := runner.RollbackLast(context.Background()); err == nil || !strings.Contains(err.Error(), "0139 rollback blocked by external foreign key") {
			t.Fatalf("external inbound foreign key rollback error=%v", err)
		}
		var remaining int
		if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name IN ('mochat_go_wecom_capability_operations','mochat_go_wecom_capability_dispatches','mochat_go_wecom_capability_operation_results','mochat_go_wecom_capability_operation_audits','mochat_go_wecom_capability_operation_events')`).Scan(&remaining); err != nil {
			t.Fatal(err)
		}
		if remaining != 5 {
			t.Fatalf("external inbound foreign key rollback dropped ledger tables: remaining=%d", remaining)
		}
	})
}

func createWeComCapabilityExternalForeignKeyProbeDatabase(t *testing.T, db *sql.DB) string {
	t.Helper()
	probeSchema := fmt.Sprintf("mochat_wecom_0139_fk_probe_%d_%d", os.Getpid(), weComCapabilityLedgerSchemaSequence.Add(1))
	if _, err := db.Exec("CREATE DATABASE `" + probeSchema + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
		t.Fatal(err)
	}
	return probeSchema
}

func createWeComCapabilityExternalForeignKeyProbe(t *testing.T, db *sql.DB, probeSchema, currentSchema string) {
	t.Helper()
	if _, err := db.Exec(fmt.Sprintf(`
CREATE TABLE %s.mo_chat_wecom_0139_external_fk_probe (
  id INT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id INT UNSIGNED NOT NULL,
  corp_id INT UNSIGNED NOT NULL,
  operation_id BIGINT UNSIGNED NOT NULL,
  dispatch_id BIGINT UNSIGNED NOT NULL,
  PRIMARY KEY (id),
  KEY idx_external_operation (tenant_id,corp_id,operation_id),
  KEY idx_external_dispatch (tenant_id,corp_id,dispatch_id),
  CONSTRAINT fk_external_operation FOREIGN KEY (tenant_id,corp_id,operation_id)
    REFERENCES %s.mochat_go_wecom_capability_operations (tenant_id,corp_id,id),
  CONSTRAINT fk_external_dispatch FOREIGN KEY (tenant_id,corp_id,dispatch_id)
    REFERENCES %s.mochat_go_wecom_capability_dispatches (tenant_id,corp_id,id)
) ENGINE=InnoDB`, quoteMigrationIdentifier(probeSchema), quoteMigrationIdentifier(currentSchema), quoteMigrationIdentifier(currentSchema))); err != nil {
		t.Fatal(err)
	}
}

func TestWeComCapabilityLedgerRealRollbackRejectsUnexpectedInternalForeignKeysBeforeDrop(t *testing.T) {
	withTemporaryWeComCapabilityLedgerSchema(t, func(db *sql.DB, root string) {
		createWeComCapabilityLedgerPreMigrationFixture(t, db)
		runner := newWeComCapabilityLedgerTestRunner(t, db, root)
		if _, err := runner.Apply(context.Background()); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`ALTER TABLE mochat_go_wecom_capability_operation_audits
ADD KEY tmp_0139_unexpected_event_fk (dispatch_id),
ADD CONSTRAINT fk_0139_unexpected_audit_event FOREIGN KEY (dispatch_id)
REFERENCES mochat_go_wecom_capability_operation_events (id)`); err != nil {
			t.Fatal(err)
		}
		if _, err := runner.RollbackLast(context.Background()); err == nil || !strings.Contains(err.Error(), "0139 rollback blocked by external foreign key") {
			t.Fatalf("unexpected internal foreign key rollback error=%v", err)
		}
		var remaining int
		if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name IN ('mochat_go_wecom_capability_operations','mochat_go_wecom_capability_dispatches','mochat_go_wecom_capability_operation_results','mochat_go_wecom_capability_operation_audits','mochat_go_wecom_capability_operation_events')`).Scan(&remaining); err != nil {
			t.Fatal(err)
		}
		if remaining != 5 {
			t.Fatalf("unexpected internal foreign key rollback dropped ledger tables: remaining=%d", remaining)
		}
	})
}

func TestWeComCapabilityLedgerRealRunnerApplyDownApply(t *testing.T) {
	withTemporaryWeComCapabilityLedgerSchema(t, func(db *sql.DB, root string) {
		createWeComCapabilityLedgerPreMigrationFixture(t, db)
		runner := newWeComCapabilityLedgerTestRunner(t, db, root)
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
	})
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
				runner := newWeComCapabilityLedgerTestRunner(t, db, root)
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
				if err := executeWeComCapabilityLedgerUpResidualProbe(t, db, root); err == nil || !strings.Contains(err.Error(), tc.want) {
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

func TestWeComCapabilityLedgerDownRejectsWrongChildIndexSignatures(t *testing.T) {
	for _, tc := range []struct {
		name  string
		alter string
	}{
		{
			name:  "dispatch claim index order",
			alter: `ALTER TABLE mochat_go_wecom_capability_dispatches DROP INDEX idx_wecom_capability_dispatch_claim, ADD KEY idx_wecom_capability_dispatch_claim (tenant_id,status,corp_id,next_poll_at)`,
		},
		{
			name:  "result target index order",
			alter: `ALTER TABLE mochat_go_wecom_capability_operation_results DROP INDEX uk_wecom_capability_result_target, ADD UNIQUE KEY uk_wecom_capability_result_target (tenant_id,corp_id,target_kind,operation_id,target_id)`,
		},
		{
			name:  "audit operation index order",
			alter: `ALTER TABLE mochat_go_wecom_capability_operation_audits ADD KEY tmp_0139_audit_operation_fk (tenant_id,corp_id,operation_id), DROP INDEX idx_wecom_capability_audit_operation, ADD KEY idx_wecom_capability_audit_operation (tenant_id,operation_id,corp_id,created_at)`,
		},
		{
			name:  "event operation index order",
			alter: `ALTER TABLE mochat_go_wecom_capability_operation_events ADD KEY tmp_0139_event_operation_fk (tenant_id,corp_id,operation_id), DROP INDEX idx_wecom_capability_event_operation, ADD KEY idx_wecom_capability_event_operation (tenant_id,operation_id,corp_id,created_at)`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			withTemporaryWeComCapabilityLedgerSchema(t, func(db *sql.DB, root string) {
				createWeComCapabilityLedgerPreMigrationFixture(t, db)
				runner := newWeComCapabilityLedgerTestRunner(t, db, root)
				if _, err := runner.Apply(context.Background()); err != nil {
					t.Fatal(err)
				}
				if _, err := db.Exec(tc.alter); err != nil {
					t.Fatal(err)
				}
				if _, err := runner.RollbackLast(context.Background()); err == nil || !strings.Contains(err.Error(), "0139 incompatible rollback residual") {
					t.Fatalf("wrong child index rollback error=%v", err)
				}
				var tables int
				if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name IN ('mochat_go_wecom_capability_operations','mochat_go_wecom_capability_dispatches','mochat_go_wecom_capability_operation_results','mochat_go_wecom_capability_operation_audits','mochat_go_wecom_capability_operation_events')`).Scan(&tables); err != nil {
					t.Fatal(err)
				}
				if tables != 5 {
					t.Fatalf("wrong child index rollback dropped ledger tables: tables=%d", tables)
				}
			})
		})
	}
}

func newWeComCapabilityLedgerTestRunner(t *testing.T, db *sql.DB, root string) *migration.Runner {
	t.Helper()
	return newExternalMigrationRunnerThrough(t, db, root, "0139_wecom_capability_ledger")
}

func executeWeComCapabilityLedgerUpResidualProbe(t *testing.T, db *sql.DB, root string) error {
	t.Helper()
	return migration.ExecuteWeComCapabilityLedger0139UpProbe(context.Background(), db, root)
}

func withTemporaryWeComCapabilityLedgerSchema(t *testing.T, fn func(db *sql.DB, root string)) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("MOCHAT_GO_MYSQL_INTEGRATION_DSN"))
	if dsn == "" {
		t.Skip("SKIP: MOCHAT_GO_MYSQL_INTEGRATION_DSN is not set; isolated MariaDB DSN is required")
	}
	database := integrationtestdb.NewIsolated(t, dsn)
	evidence, err := testharness.NewControlledEvidence("wecom-0139")
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join("..", "..")
	if err := testharness.ApplyThrough(context.Background(), database.DB, root, "0138_archive_source_sync", evidence); err != nil {
		t.Fatalf("apply production migration registry through 0138: %v", err)
	}
	fn(database.DB, root)
}

func createWeComCapabilityLedgerPreMigrationFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	statements := []string{
		`INSERT INTO mc_tenant(id,name,status) VALUES (11,'Tenant 11',1),(22,'Tenant 22',1)`,
		`INSERT INTO mc_user(id,tenant_id) VALUES (101,11),(202,22)`,
		`INSERT INTO mc_corp(id,tenant_id,name) VALUES (1101,11,'Corp 11'),(2201,22,'Corp 22')`,
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
