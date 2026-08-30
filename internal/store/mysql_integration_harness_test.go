package store

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

var currentStoreIntegrationSequence atomic.Uint64

func newCurrentStoreIntegrationDB(t *testing.T) *sql.DB {
	t.Helper()
	database, evidence := newStoreIntegrationDatabase(t)
	if err := testharness.ApplyLatest(context.Background(), database.DB, filepath.Join("..", ".."), evidence); err != nil {
		t.Fatalf("apply production migration registry: %v", err)
	}
	return database.DB
}

func newStoreIntegrationDBThrough(t *testing.T, targetVersion string) *sql.DB {
	t.Helper()
	database, evidence := newStoreIntegrationDatabase(t)
	if err := testharness.ApplyThrough(context.Background(), database.DB, filepath.Join("..", ".."), targetVersion, evidence); err != nil {
		t.Fatalf("apply production migration registry through %s: %v", targetVersion, err)
	}
	return database.DB
}

func newStoreIntegrationDatabase(t *testing.T) (*integrationtestdb.Database, testharness.ControlledEvidence) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("MOCHAT_GO_MYSQL_INTEGRATION_DSN"))
	if dsn == "" {
		t.Skip("SKIP: MOCHAT_GO_MYSQL_INTEGRATION_DSN is not set; isolated MariaDB/MySQL DSN is required")
	}
	database := integrationtestdb.NewIsolated(t, dsn)
	prefix := fmt.Sprintf("store-%d-%d", os.Getpid(), currentStoreIntegrationSequence.Add(1))
	evidence, err := testharness.NewControlledEvidence(prefix)
	if err != nil {
		t.Fatal(err)
	}
	return database, evidence
}

func newStoreMigrationRunnerThrough(t *testing.T, db *sql.DB, targetVersion string) *migration.Runner {
	t.Helper()
	migrations := migration.DefaultMigrations(filepath.Join("..", ".."))
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

func TestCurrentStoreIntegrationDBUsesLatestProductionRegistry(t *testing.T) {
	db := newCurrentStoreIntegrationDB(t)
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_schema_migrations WHERE version=(SELECT MAX(version) FROM mochat_go_schema_migrations)`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("latest production migration ledger rows=%d, want 1", count)
	}
}
