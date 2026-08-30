package main

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"jiyi/mochat-go/internal/integrationtestdb"
	"jiyi/mochat-go/internal/migration"
	"jiyi/mochat-go/internal/saasauth"
	"jiyi/mochat-go/internal/store"
)

func TestFullRegistryFromEmptySchemaViaProductionAPIs(t *testing.T) {
	adminDSN := strings.TrimSpace(os.Getenv("MOCHAT_GO_MYSQL_INTEGRATION_DSN"))
	if adminDSN == "" {
		t.Skip("SKIP: MOCHAT_GO_MYSQL_INTEGRATION_DSN is required for authoritative full-registry integration")
	}
	database := integrationtestdb.NewIsolated(t, adminDSN)
	root := filepath.Join("..", "..")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	inventory, err := migration.DefaultInventory(root)
	if err != nil {
		t.Fatal(err)
	}
	runner, err := migration.NewRunner(database.DB, migration.DefaultMigrations(root))
	if err != nil {
		t.Fatal(err)
	}

	assertRegistryPending(t, ctx, runner, database.DB, "0130_identity_realms_single_corp_backfill")
	identityStore := store.NewSaaSIdentityStore(database.DB)
	bootstrapHash, err := saasauth.HashPassword("registry-bootstrap-password")
	if err != nil {
		t.Fatal(err)
	}
	rootIdentity, err := identityStore.Bootstrap(ctx, saasauth.BootstrapSaaSAdmin{
		RequestKey: "registry-empty-bootstrap", LoginName: "registry-root", Name: "Registry root", PasswordHash: bootstrapHash,
	})
	if err != nil {
		t.Fatal(err)
	}
	rotatedHash, err := saasauth.HashPassword("registry-rotated-password")
	if err != nil {
		t.Fatal(err)
	}
	rootIdentity, err = identityStore.ChangePassword(ctx, rootIdentity.ID, rootIdentity.AuthVersion, rotatedHash)
	if err != nil || rootIdentity.MustRotatePassword != 0 {
		t.Fatalf("rotate bootstrap password identity=%+v err=%v", rootIdentity, err)
	}

	files := newRegistryMaintenanceFiles(t, database.DSN, database.Name)
	request0130 := "registry-0130"
	confirmation0130 := files.confirmation(t, request0130)
	if err := runMigration(files.args("up", request0130, confirmation0130), io.Discard); err != nil {
		t.Fatal(err)
	}
	if err := runMigration(files.args("up", request0130, confirmation0130), io.Discard); err != nil {
		t.Fatalf("same 0130 request must be idempotent: %v", err)
	}
	if err := runMigration(files.args("encrypt-credentials", request0130, confirmation0130), io.Discard); err != nil {
		t.Fatal(err)
	}
	assertRegistryPending(t, ctx, runner, database.DB, "0131_identity_realms_single_corp_cutover")

	request0131 := "registry-0131"
	if err := runMigration(files.args("cutover", request0131, confirmation0130), io.Discard); err == nil {
		t.Fatal("0131 accepted the 0130 maintenance confirmation")
	}
	confirmation0131 := files.confirmation(t, request0131)
	if err := runMigration(files.args("cutover", request0131, confirmation0131), io.Discard); err != nil {
		t.Fatal(err)
	}
	if err := runMigration(files.args("cutover", request0131, confirmation0131), io.Discard); err != nil {
		t.Fatalf("same 0131 request must be idempotent: %v", err)
	}
	assertRegistryPending(t, ctx, runner, database.DB, migration.AIInsight0165Version)

	controller, err := migration.NewAIInsight0165Controller(database.DB, root)
	if err != nil {
		t.Fatal(err)
	}
	request0165 := "registry-0165"
	before, err := controller.Inventory(ctx)
	if err != nil {
		t.Fatal(err)
	}
	backup, err := controller.Backup(ctx, request0165)
	if err != nil {
		t.Fatal(err)
	}
	if before.InsightDigest == "" || before.LegacyDigest == "" || backup.BackupInsightDigest == "" || backup.BackupLegacyDigest == "" {
		t.Fatalf("0165 empty-set evidence must retain digests: before=%+v backup=%+v", before, backup)
	}
	preflight, err := controller.Preflight(ctx, request0165)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := controller.Apply(ctx, migration.AIInsight0165ApplyRequest{RequestID: request0165, ApprovalToken: preflight.ApprovalToken, DestructiveApproval: preflight.DestructiveApproval, TrafficStopped: true}); err != nil {
		t.Fatal(err)
	}
	verified, err := controller.Verify(ctx, request0165)
	if err != nil || !verified.Applied || !verified.Verified {
		t.Fatalf("0165 verification=%+v err=%v", verified, err)
	}
	if _, err := runner.Apply(ctx); err != nil {
		t.Fatal(err)
	}
	status, err := runner.StatusReadOnly(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(status) != len(inventory) {
		t.Fatalf("status items=%d inventory=%d", len(status), len(inventory))
	}
	for index, item := range status {
		if item.State != "applied" || item.Applied == nil || item.Checksum != inventory[index].Checksum || item.Migration.Version != inventory[index].Version {
			t.Fatalf("registry item %d mismatch status=%+v inventory=%+v", index, item, inventory[index])
		}
	}
	var ledgerRows int
	if err := database.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM mochat_go_schema_migrations`).Scan(&ledgerRows); err != nil || ledgerRows != len(inventory) {
		t.Fatalf("ledger rows=%d inventory=%d err=%v", ledgerRows, len(inventory), err)
	}
}

func assertRegistryPending(t *testing.T, ctx context.Context, runner *migration.Runner, db interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, version string) {
	t.Helper()
	_, err := runner.Apply(ctx)
	var pending *migration.ControlledMigrationPendingError
	if !errors.As(err, &pending) || pending.Version != version {
		t.Fatalf("runner pending=%v, want %s", err, version)
	}
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mochat_go_schema_migrations WHERE version=?`, version).Scan(&count); err != nil || count != 0 {
		t.Fatalf("pending migration %s ledger rows=%d err=%v", version, count, err)
	}
}

type registryMaintenanceFiles struct {
	dsnFile, keyFile, schema, root string
}

func newRegistryMaintenanceFiles(t *testing.T, dsn, schema string) registryMaintenanceFiles {
	t.Helper()
	dir := t.TempDir()
	files := registryMaintenanceFiles{dsnFile: filepath.Join(dir, "dsn"), keyFile: filepath.Join(dir, "credential-key"), schema: schema, root: filepath.Join("..", "..")}
	if err := os.WriteFile(files.dsnFile, []byte(dsn), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(files.keyFile, []byte(strings.Repeat("01", 32)), 0o600); err != nil {
		t.Fatal(err)
	}
	return files
}

func (files registryMaintenanceFiles) confirmation(t *testing.T, requestID string) string {
	t.Helper()
	path := filepath.Join(filepath.Dir(files.dsnFile), "confirmation-"+requestID)
	body := "MOCHAT_IDENTITY_MAINTENANCE_V1\nschema=" + files.schema + "\nrequest_id=" + requestID
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func (files registryMaintenanceFiles) args(action, requestID, confirmation string) []string {
	return []string{action, "--execute", "--request-id", requestID, "--dsn-file", files.dsnFile, "--schema", files.schema, "--platform-tenant-id", "1", "--maintenance-confirmation-file", confirmation, "--credential-key-file", files.keyFile, "--credential-key-id", "registry-test", "--project-root", files.root, "--timeout", "15m"}
}
