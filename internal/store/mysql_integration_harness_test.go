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
	"jiyi/mochat-go/internal/migration/testharness"
)

var currentStoreIntegrationSequence atomic.Uint64

func newCurrentStoreIntegrationDB(t *testing.T) *sql.DB {
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
	if err := testharness.ApplyLatest(context.Background(), database.DB, filepath.Join("..", ".."), evidence); err != nil {
		t.Fatalf("apply production migration registry: %v", err)
	}
	return database.DB
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
