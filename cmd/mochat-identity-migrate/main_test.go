package main

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"

	"jiyi/mochat-go/internal/identitymigration"
)

func TestMigrateCLIReportsUsageWhenActionIsMissing(t *testing.T) {
	if _, err := parseMigrateOptions(nil); err == nil || !strings.Contains(err.Error(), "usage: mochat-identity-migrate") {
		t.Fatalf("missing action error=%v, want stable usage", err)
	}
}

func TestMigrateCLIRejectsUnknownAction(t *testing.T) {
	if _, err := parseMigrateOptions([]string{"not-a-migration"}); err == nil || !strings.Contains(err.Error(), "unknown migration action") {
		t.Fatalf("unknown action error=%v", err)
	}
}

func TestMigrateCLIReadsMaintenanceConfirmationBoundToSchemaAndRequest(t *testing.T) {
	path := t.TempDir() + "\\maintenance.confirm"
	if err := os.WriteFile(path, []byte("MOCHAT_IDENTITY_MAINTENANCE_V1\nschema=maintenance_schema\nrequest_id=task12-1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := readMaintenanceConfirmation(path, "maintenance_schema", "task12-1"); err != nil {
		t.Fatalf("valid maintenance confirmation rejected: %v", err)
	}
	if err := readMaintenanceConfirmation(path, "other_schema", "task12-1"); err == nil {
		t.Fatal("maintenance confirmation reused for another schema")
	}
}

func TestMigrateCLIOnlyPrintsVerifiedPhaseDiagnostics(t *testing.T) {
	var output bytes.Buffer
	writeMigrationError(&output, &identitymigration.PhaseError{Phase: "fk_access", Label: "access_fk"})
	if !strings.Contains(output.String(), "phase=fk_access label=access_fk") {
		t.Fatalf("operator output=%q, want safe phase diagnostic", output.String())
	}
	output.Reset()
	writeMigrationError(&output, errors.New("driver leaked SQL and secret"))
	if strings.Contains(output.String(), "driver leaked") || !strings.Contains(output.String(), "identity migration failed") {
		t.Fatalf("operator output=%q leaked non-phase error or omitted generic error", output.String())
	}
}

func TestMigrateCLIRequiresExecuteAndRequestIDForWrites(t *testing.T) {
	base := []string{"encrypt-credentials", "--dsn-file", "dsn-file", "--schema", "business_schema", "--platform-tenant-id", "7", "--credential-key-file", "credential.key", "--credential-key-id", "task8-wecom-v1"}
	if _, err := parseMigrateOptions(append(base, "--request-id", "task8-1")); err == nil || !strings.Contains(err.Error(), "--execute") {
		t.Fatalf("missing execute error=%v", err)
	}
	if _, err := parseMigrateOptions(append(base, "--execute", "--request-id", "task8-1")); err == nil || !strings.Contains(err.Error(), "maintenance") {
		t.Fatalf("missing maintenance confirmation error=%v", err)
	}
	options, err := parseMigrateOptions([]string{"encrypt-credentials", "--execute", "--request-id", "task8-1", "--dsn-file", "dsn-file", "--schema", "business_schema", "--platform-tenant-id", "7", "--maintenance-confirmation-file", "maintenance.confirm", "--credential-key-file", "credential.key", "--credential-key-id", "task8-wecom-v1"})
	if err != nil {
		t.Fatal(err)
	}
	if !options.Execute || options.RequestID != "task8-1" || options.Schema != "business_schema" || options.PlatformTenantID != 7 || options.MaintenanceConfirmationFile != "maintenance.confirm" {
		t.Fatalf("migrate options=%+v, want explicit write contract", options)
	}
}

func TestMigrateCLIAcceptsExplicitLegacyCredentialRestoreAction(t *testing.T) {
	options, err := parseMigrateOptions([]string{
		"restore-legacy-credentials",
		"--execute",
		"--request-id", "task12-restore-1",
		"--dsn-file", "dsn-file",
		"--schema", "maintenance_schema",
		"--platform-tenant-id", "7",
		"--maintenance-confirmation-file", "maintenance.confirm",
		"--credential-key-file", "credential.key",
		"--credential-key-id", "task12-wecom-v1",
	})
	if err != nil {
		t.Fatalf("legacy restore action error=%v", err)
	}
	if options.Action != "restore-legacy-credentials" || !options.Execute || options.RequestID != "task12-restore-1" {
		t.Fatalf("restore options=%+v, want explicit maintenance action", options)
	}
}

func TestMigrateCLIRejectsPasswordAndSecretFlags(t *testing.T) {
	for _, args := range [][]string{{"encrypt-credentials", "--password", "secret"}, {"encrypt-credentials", "--secret", "secret"}, {"encrypt-credentials", "--token", "token"}} {
		if _, err := parseMigrateOptions(args); err == nil {
			t.Fatalf("migrate CLI accepted sensitive flag %v", args)
		}
	}
}
