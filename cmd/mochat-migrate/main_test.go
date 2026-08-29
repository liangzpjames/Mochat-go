package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"jiyi/mochat-go/internal/migration"
	"jiyi/mochat-go/internal/observability"
)

func TestRunMigrationCommandLogsLifecycleWithoutDSN(t *testing.T) {
	var logs bytes.Buffer
	logger, err := observability.New(observability.Config{Format: "json", Level: "info", Output: &logs, Service: "test"})
	if err != nil {
		t.Fatal(err)
	}
	runner := &fakeMigrationRunner{status: []migration.StatusItem{{Migration: migration.Migration{Version: "0001"}, State: "applied_now"}}}
	if err := runMigrationCommand(context.Background(), migrationCommandOptions{Action: "apply"}, runner, logger, io.Discard); err != nil {
		t.Fatal(err)
	}
	text := logs.String()
	for _, required := range []string{"migration_started", "migration_completed", `"applied":1`, `"result":"success"`} {
		if !strings.Contains(text, required) {
			t.Fatalf("missing %q: %s", required, text)
		}
	}
	if strings.Contains(text, "user:password@tcp") {
		t.Fatalf("DSN leaked: %s", text)
	}
}

func TestRunMigrationCommandLogsControlledFailure(t *testing.T) {
	var logs bytes.Buffer
	logger, err := observability.New(observability.Config{Format: "json", Level: "info", Output: &logs, Service: "test"})
	if err != nil {
		t.Fatal(err)
	}
	runner := &fakeMigrationRunner{err: errors.New("connect user:password@tcp(db:3306) failed")}
	err = runMigrationCommand(context.Background(), migrationCommandOptions{Action: "apply"}, runner, logger, io.Discard)
	if err == nil {
		t.Fatal("expected migration failure")
	}
	text := logs.String()
	for _, required := range []string{"migration_started", "migration_failed", `"level":"ERROR"`, `"error_code":"MIGRATION_APPLY_FAILED"`} {
		if !strings.Contains(text, required) {
			t.Fatalf("missing %q: %s", required, text)
		}
	}
	if strings.Contains(text, "password") || strings.Contains(text, "db:3306") {
		t.Fatalf("migration failure leaked DSN: %s", text)
	}
}

func TestRunMigrationCommandEmitsStableControlledPendingCode(t *testing.T) {
	var output bytes.Buffer
	runner := &fakeMigrationRunner{err: migration.ControlledMigrationBlocked("0130_identity_realms_single_corp_backfill")}
	err := runMigrationCommand(context.Background(), migrationCommandOptions{Action: "up"}, runner, slog.New(slog.NewTextHandler(io.Discard, nil)), &output)
	if err == nil {
		t.Fatal("expected controlled migration failure")
	}
	if got := output.String(); got != "MIGRATION_CONTROLLED_PENDING\t0130_identity_realms_single_corp_backfill\n" {
		t.Fatalf("controlled output = %q", got)
	}
}

func TestWriteMigrationInventoryEmitsStableTSV(t *testing.T) {
	items := []migration.InventoryItem{
		{Version: "0001_initial_schema", Checksum: "aaa", Kind: migration.MigrationAutomatic, Description: "initial schema"},
		{Version: "0130_identity", Checksum: "bbb", Kind: migration.MigrationControlled, Description: "identity"},
	}
	var output bytes.Buffer
	writeMigrationInventory(&output, items)
	if got, want := output.String(), "0001_initial_schema\taaa\tautomatic\tinitial schema\n0130_identity\tbbb\tcontrolled\tidentity\n"; got != want {
		t.Fatalf("inventory output = %q, want %q", got, want)
	}
}

type fakeMigrationRunner struct {
	status []migration.StatusItem
	err    error
}

func (f *fakeMigrationRunner) Apply(context.Context) ([]migration.StatusItem, error) {
	return f.status, f.err
}
func (f *fakeMigrationRunner) BaselineComposeInit(context.Context) ([]migration.StatusItem, error) {
	return f.status, f.err
}
func (f *fakeMigrationRunner) Status(context.Context) ([]migration.StatusItem, error) {
	return f.status, f.err
}
func (f *fakeMigrationRunner) Baseline(context.Context) ([]migration.StatusItem, error) {
	return f.status, f.err
}
func (f *fakeMigrationRunner) RollbackLast(context.Context) (string, error) {
	return "", f.err
}
