package migration_test

import (
	"context"
	"database/sql"
	"testing"
)

func TestWeWorkCallbackSideEffectReconciliation0176UpDownReapplyLifecycle(t *testing.T) {
	db, root := newExternalMigrationIntegrationDBThrough(t, "0175_contact_batch_title", "callback-recovery-0176")
	runner := newExternalMigrationRunnerThrough(t, db, root, "0176_wework_callback_side_effect_reconciliation")
	if _, err := runner.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertCallbackRecovery0176Schema(t, db, true)
	rolledBack, err := runner.RollbackLast(context.Background())
	if err != nil || rolledBack != "0176_wework_callback_side_effect_reconciliation" {
		t.Fatalf("rollback=%q err=%v", rolledBack, err)
	}
	assertCallbackRecovery0176Schema(t, db, false)
	if _, err := runner.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertCallbackRecovery0176Schema(t, db, true)
}

func assertCallbackRecovery0176Schema(t *testing.T, db *sql.DB, present bool) {
	t.Helper()
	var columns, commands, resources, auditRequestIDLength int
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='mochat_go_wework_callback_side_effects' AND column_name IN ('version','reconciliation_fence','unknown_at','reconcile_after','last_decision','last_reconciled_at')`).Scan(&columns); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name='mochat_go_wework_callback_side_effect_commands'`).Scan(&commands); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_dashboard_permission_resources resource INNER JOIN mochat_go_dashboard_permissions permission ON permission.id=resource.permission_id WHERE permission.code='dashboard.company_setting.website' AND resource.path_pattern LIKE '/dashboard/company/callback-side-effects%'`).Scan(&resources); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT character_maximum_length FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='mochat_go_dashboard_permission_audits' AND column_name='request_id'`).Scan(&auditRequestIDLength); err != nil {
		t.Fatal(err)
	}
	if present {
		if columns != 6 || commands != 1 || resources != 3 || auditRequestIDLength != 128 {
			t.Fatalf("0176 schema columns=%d commands=%d resources=%d auditRequestIDLength=%d", columns, commands, resources, auditRequestIDLength)
		}
		return
	}
	if columns != 0 || commands != 0 || resources != 0 || auditRequestIDLength != 96 {
		t.Fatalf("0176 rollback columns=%d commands=%d resources=%d auditRequestIDLength=%d", columns, commands, resources, auditRequestIDLength)
	}
	var sideEffects int
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name='mochat_go_wework_callback_side_effects'`).Scan(&sideEffects); err != nil || sideEffects != 1 {
		t.Fatalf("0174 side-effect table after 0176 down=%d err=%v", sideEffects, err)
	}
}
