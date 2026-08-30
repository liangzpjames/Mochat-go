package migration_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"jiyi/mochat-go/internal/integrationtestdb"
	"jiyi/mochat-go/internal/migration"
	"jiyi/mochat-go/internal/migration/testharness"
)

func newExternalMigrationIntegrationDBThrough(t *testing.T, targetVersion, evidencePrefix string) (*sql.DB, string) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("MOCHAT_GO_MYSQL_INTEGRATION_DSN"))
	if dsn == "" {
		t.Skip("SKIP: MOCHAT_GO_MYSQL_INTEGRATION_DSN is required for migration lifecycle integration")
	}
	database := integrationtestdb.NewIsolated(t, dsn)
	root := filepath.Join("..", "..")
	evidence, err := testharness.NewControlledEvidence(evidencePrefix)
	if err != nil {
		t.Fatal(err)
	}
	if err := testharness.ApplyThrough(context.Background(), database.DB, root, targetVersion, evidence); err != nil {
		t.Fatalf("apply production migration registry through %s: %v", targetVersion, err)
	}
	return database.DB, root
}

func newExternalMigrationRunnerThrough(t *testing.T, db *sql.DB, root, targetVersion string) *migration.Runner {
	t.Helper()
	migrations := migration.DefaultMigrations(root)
	for index, candidate := range migrations {
		if candidate.Version != targetVersion {
			continue
		}
		runner, err := migration.NewRunner(db, migrations[:index+1])
		if err != nil {
			t.Fatal(err)
		}
		return runner
	}
	t.Fatalf("production migration registry does not contain %s", targetVersion)
	return nil
}
