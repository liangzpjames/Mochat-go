// Package integrationtestdb owns only the lifecycle of isolated MySQL-family
// databases used by integration tests. Business schema must be installed by
// the production migration registry.
package integrationtestdb

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"regexp"
	"sync"
	"time"

	"github.com/go-sql-driver/mysql"
)

const isolatedPrefix = "mochat_it_"

var isolatedNamePattern = regexp.MustCompile(`^mochat_it_[0-9a-f]{24}$`)

type testCleanup interface {
	Helper()
	Cleanup(func())
	Fatalf(string, ...any)
	Errorf(string, ...any)
}

// Database is one random database created from an administrator DSN.
type Database struct {
	DB   *sql.DB
	DSN  string
	Name string

	t         testCleanup
	admin     *sql.DB
	mu        sync.Mutex
	rollbacks []func(context.Context, *sql.DB) error
	closed    bool
}

// NewIsolated creates a new random database. The database selected by
// adminDSN is intentionally ignored and is never reused or dropped.
func NewIsolated(t testCleanup, adminDSN string) *Database {
	t.Helper()
	config, err := mysql.ParseDSN(adminDSN)
	if err != nil {
		t.Fatalf("parse integration administrator DSN: %v", err)
	}
	adminConfig := config.Clone()
	adminConfig.DBName = ""
	admin, err := sql.Open("mysql", adminConfig.FormatDSN())
	if err != nil {
		t.Fatalf("open integration administrator database: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := admin.PingContext(ctx); err != nil {
		_ = admin.Close()
		t.Fatalf("ping integration administrator database: %v", err)
	}

	name, err := createRandomDatabase(ctx, admin)
	if err != nil {
		_ = admin.Close()
		t.Fatalf("create isolated integration database: %v", err)
	}
	isolatedConfig := config.Clone()
	isolatedConfig.DBName = name
	isolatedConfig.ParseTime = true
	db, err := sql.Open("mysql", isolatedConfig.FormatDSN())
	if err != nil {
		dropExactDatabase(context.Background(), admin, name)
		_ = admin.Close()
		t.Fatalf("open isolated integration database: %v", err)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		dropExactDatabase(context.Background(), admin, name)
		_ = admin.Close()
		t.Fatalf("ping isolated integration database: %v", err)
	}

	database := &Database{DB: db, DSN: isolatedConfig.FormatDSN(), Name: name, t: t, admin: admin}
	t.Cleanup(database.cleanup)
	return database
}

// RegisterSeedRollback registers scenario-data cleanup. Rollbacks run before
// the isolated database is dropped and are attempted even when a test fails.
func (d *Database) RegisterSeedRollback(rollback func(context.Context, *sql.DB) error) {
	if d == nil || rollback == nil {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		d.t.Errorf("register seed rollback after isolated database cleanup")
		return
	}
	d.rollbacks = append(d.rollbacks, rollback)
}

func createRandomDatabase(ctx context.Context, admin *sql.DB) (string, error) {
	for attempt := 0; attempt < 8; attempt++ {
		var suffix [12]byte
		if _, err := rand.Read(suffix[:]); err != nil {
			return "", err
		}
		name := isolatedPrefix + hex.EncodeToString(suffix[:])
		if _, err := admin.ExecContext(ctx, `CREATE DATABASE `+name+` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci`); err == nil {
			return name, nil
		} else if mysqlError, ok := err.(*mysql.MySQLError); !ok || mysqlError.Number != 1007 {
			return "", err
		}
	}
	return "", fmt.Errorf("could not allocate a unique isolated database name")
}

func (d *Database) cleanup() {
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return
	}
	d.closed = true
	rollbacks := append([]func(context.Context, *sql.DB) error(nil), d.rollbacks...)
	d.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	for index := len(rollbacks) - 1; index >= 0; index-- {
		if err := rollbacks[index](ctx, d.DB); err != nil {
			d.t.Errorf("rollback seed data in %s: %v", d.Name, err)
		}
	}
	if err := d.DB.Close(); err != nil {
		d.t.Errorf("close isolated database %s: %v", d.Name, err)
	}
	if err := dropExactDatabase(ctx, d.admin, d.Name); err != nil {
		d.t.Errorf("drop isolated database %s: %v", d.Name, err)
	}
	var leftovers int
	if err := d.admin.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.schemata WHERE schema_name=?`, d.Name).Scan(&leftovers); err != nil {
		d.t.Errorf("verify isolated database cleanup %s: %v", d.Name, err)
	} else if leftovers != 0 {
		d.t.Errorf("isolated database cleanup left %d schema named %s", leftovers, d.Name)
	}
	if err := d.admin.Close(); err != nil {
		d.t.Errorf("close integration administrator database: %v", err)
	}
}

func dropExactDatabase(ctx context.Context, admin *sql.DB, name string) error {
	if admin == nil || !isolatedNamePattern.MatchString(name) {
		return fmt.Errorf("refuse to drop unregistered isolated database %q", name)
	}
	_, err := admin.ExecContext(ctx, `DROP DATABASE IF EXISTS `+name)
	return err
}
