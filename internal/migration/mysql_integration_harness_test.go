package migration

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"jiyi/mochat-go/internal/integrationtestdb"
)

func newMigrationIntegrationDBThrough(t *testing.T, targetVersion string) *sql.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("MOCHAT_GO_MYSQL_INTEGRATION_DSN"))
	if dsn == "" {
		t.Skip("SKIP: MOCHAT_GO_MYSQL_INTEGRATION_DSN is not set; isolated MariaDB/MySQL DSN is required")
	}
	database := integrationtestdb.NewIsolated(t, dsn)
	runner := newMigrationRunnerThrough(t, database.DB, targetVersion)
	if _, err := runner.Apply(context.Background()); err != nil {
		t.Fatalf("apply production migration registry through %s: %v", targetVersion, err)
	}
	return database.DB
}

func newMigrationRunnerThrough(t *testing.T, db *sql.DB, targetVersion string) *Runner {
	t.Helper()
	migrations := DefaultMigrations(filepath.Join("..", ".."))
	for index, candidate := range migrations {
		if candidate.Version != targetVersion {
			continue
		}
		runner, err := NewRunner(db, migrations[:index+1])
		if err != nil {
			t.Fatal(err)
		}
		return runner
	}
	t.Fatalf("production migration registry does not contain %s", targetVersion)
	return nil
}
