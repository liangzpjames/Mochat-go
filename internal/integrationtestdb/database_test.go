package integrationtestdb_test

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/go-sql-driver/mysql"

	"jiyi/mochat-go/internal/integrationtestdb"
)

func TestNewIsolatedDoesNotReuseDatabaseFromAdminDSN(t *testing.T) {
	dsn := integrationDSN(t)
	configured, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	database := integrationtestdb.NewIsolated(t, dsn)
	if database.Name == "" {
		t.Fatal("isolated database name is empty")
	}
	if configured.DBName != "" && database.Name == configured.DBName {
		t.Fatalf("isolated database reused admin DSN database %q", configured.DBName)
	}
	var current string
	if err := database.DB.QueryRow(`SELECT DATABASE()`).Scan(&current); err != nil {
		t.Fatal(err)
	}
	if current != database.Name {
		t.Fatalf("current database=%q, want %q", current, database.Name)
	}
}

func TestNewIsolatedRunsSeedRollbackBeforeDrop(t *testing.T) {
	dsn := integrationDSN(t)
	var rollbackCalled atomic.Bool
	t.Run("lifecycle", func(t *testing.T) {
		database := integrationtestdb.NewIsolated(t, dsn)
		database.RegisterSeedRollback(func(ctx context.Context, db *sql.DB) error {
			var current string
			if err := db.QueryRowContext(ctx, `SELECT DATABASE()`).Scan(&current); err != nil {
				return err
			}
			if current != database.Name {
				t.Fatalf("rollback current database=%q, want %q", current, database.Name)
			}
			rollbackCalled.Store(true)
			return nil
		})
	})
	if !rollbackCalled.Load() {
		t.Fatal("seed rollback was not called")
	}
}

func TestNewIsolatedConcurrentNamesAreUnique(t *testing.T) {
	dsn := integrationDSN(t)
	const count = 8
	names := make(chan string, count)
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			database := integrationtestdb.NewIsolated(t, dsn)
			names <- database.Name
		}()
	}
	wg.Wait()
	close(names)
	seen := make(map[string]bool, count)
	for name := range names {
		if seen[name] {
			t.Fatalf("duplicate isolated database name %q", name)
		}
		seen[name] = true
	}
}

func integrationDSN(t *testing.T) string {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("MOCHAT_GO_MYSQL_INTEGRATION_DSN"))
	if dsn == "" {
		t.Skip("SKIP: MOCHAT_GO_MYSQL_INTEGRATION_DSN is not set; isolated MariaDB/MySQL DSN is required")
	}
	return dsn
}
