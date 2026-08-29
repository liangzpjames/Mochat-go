package testharness_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"jiyi/mochat-go/internal/integrationtestdb"
	"jiyi/mochat-go/internal/migration"
	"jiyi/mochat-go/internal/migration/testharness"
)

func TestApplyThroughFailsClosedAt0130WithoutControlledEvidence(t *testing.T) {
	dsn := registryIntegrationDSN(t)
	database := integrationtestdb.NewIsolated(t, dsn)
	root := filepath.Join("..", "..", "..")
	err := testharness.ApplyThrough(context.Background(), database.DB, root, "0130_identity_realms_single_corp_backfill", testharness.ControlledEvidence{})
	var pending *migration.ControlledMigrationPendingError
	if !errors.As(err, &pending) {
		t.Fatalf("ApplyThrough() error=%v, want controlled migration pending", err)
	}
}

func TestApplyLatestBuildsSchemaFromCompleteProductionRegistry(t *testing.T) {
	database := integrationtestdb.NewIsolated(t, registryIntegrationDSN(t))
	root := filepath.Join("..", "..", "..")
	evidence, err := testharness.NewControlledEvidence("registry-latest")
	if err != nil {
		t.Fatal(err)
	}
	if err := testharness.ApplyLatest(context.Background(), database.DB, root, evidence); err != nil {
		t.Fatal(err)
	}
	migrations := migration.DefaultMigrations(root)
	latest := migrations[len(migrations)-1].Version
	var count int
	if err := database.DB.QueryRow(`SELECT COUNT(*) FROM mochat_go_schema_migrations WHERE version=?`, latest).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("latest migration %s ledger rows=%d, want 1", latest, count)
	}
	for index := 0; index < 2; index++ {
		if _, err := database.DB.Exec(`INSERT INTO mc_tenant (name,status) VALUES (?,1)`, "post-migration-tenant"); err != nil {
			t.Fatalf("post-migration tenant insert %d: %v", index+1, err)
		}
	}
}

func registryIntegrationDSN(t *testing.T) string {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("MOCHAT_GO_MYSQL_INTEGRATION_DSN"))
	if dsn == "" {
		t.Skip("SKIP: MOCHAT_GO_MYSQL_INTEGRATION_DSN is not set; isolated MariaDB/MySQL DSN is required")
	}
	return dsn
}
