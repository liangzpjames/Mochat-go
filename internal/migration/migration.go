package migration

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const VersionTable = "mochat_go_schema_migrations"

type Migration struct {
	Version         string
	Description     string
	Path            string
	DownPath        string
	ChecksumAliases []string
}

type AppliedMigration struct {
	Version     string
	Description string
	Checksum    string
	AppliedAt   time.Time
	ExecutionMS int
}

type StatusItem struct {
	Migration Migration
	Checksum  string
	Applied   *AppliedMigration
	State     string
}

type Runner struct {
	db         *sql.DB
	migrations []Migration
	now        func() time.Time
}

func NewRunner(db *sql.DB, migrations []Migration) (*Runner, error) {
	if db == nil {
		return nil, errors.New("migration db is nil")
	}
	migrations = append([]Migration{}, migrations...)
	if err := validateMigrations(migrations); err != nil {
		return nil, err
	}
	return &Runner{db: db, migrations: migrations}, nil
}

func DefaultMigrations(projectRoot string) []Migration {
	schemaPath := filepath.Join(projectRoot, "deploy", "standalone", "schema", "mochat.sql")
	seedPath := filepath.Join(projectRoot, "deploy", "standalone", "migrations", "0002_seed_core_data.up.sql")
	migrations := []Migration{{
		Version:         "0001_initial_schema",
		Description:     "MoChat standalone initial schema",
		Path:            schemaPath,
		ChecksumAliases: legacyCombinedInitialChecksums(schemaPath, seedPath),
	}}
	migrations = append(migrations, standaloneIncrementalMigrations(projectRoot)...)
	return migrations
}

func (r *Runner) Apply(ctx context.Context) ([]StatusItem, error) {
	if err := r.ensureVersionTable(ctx); err != nil {
		return nil, err
	}
	applied, err := r.applied(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]StatusItem, 0, len(r.migrations))
	for _, migration := range r.migrations {
		body, checksum, err := migrationBodyAndChecksum(migration)
		if err != nil {
			return nil, err
		}
		if existing, ok := applied[migration.Version]; ok {
			item := StatusItem{Migration: migration, Checksum: checksum, Applied: &existing, State: "applied"}
			if !checksumMatches(existing.Checksum, checksum, migration.ChecksumAliases) {
				item.State = "checksum_mismatch"
				return append(result, item), fmt.Errorf("migration %s checksum mismatch: applied=%s current=%s", migration.Version, existing.Checksum, checksum)
			}
			result = append(result, item)
			continue
		}
		start := r.currentTime()
		if err := execSQLScript(ctx, r.db, string(body)); err != nil {
			return result, fmt.Errorf("apply migration %s: %w", migration.Version, err)
		}
		executionMS := int(r.currentTime().Sub(start).Milliseconds())
		if executionMS < 0 {
			executionMS = 0
		}
		if err := r.recordApplied(ctx, migration, checksum, executionMS); err != nil {
			return result, err
		}
		appliedItem := AppliedMigration{
			Version:     migration.Version,
			Description: migration.Description,
			Checksum:    checksum,
			AppliedAt:   r.currentTime(),
			ExecutionMS: executionMS,
		}
		result = append(result, StatusItem{Migration: migration, Checksum: checksum, Applied: &appliedItem, State: "applied_now"})
	}
	return result, nil
}

func (r *Runner) Status(ctx context.Context) ([]StatusItem, error) {
	if err := r.ensureVersionTable(ctx); err != nil {
		return nil, err
	}
	applied, err := r.applied(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]StatusItem, 0, len(r.migrations))
	for _, migration := range r.migrations {
		_, checksum, err := migrationBodyAndChecksum(migration)
		if err != nil {
			return nil, err
		}
		item := StatusItem{Migration: migration, Checksum: checksum, State: "pending"}
		if existing, ok := applied[migration.Version]; ok {
			item.Applied = &existing
			item.State = "applied"
			if !checksumMatches(existing.Checksum, checksum, migration.ChecksumAliases) {
				item.State = "checksum_mismatch"
			}
		}
		result = append(result, item)
	}
	return result, nil
}

func (r *Runner) Baseline(ctx context.Context) ([]StatusItem, error) {
	if err := r.ensureVersionTable(ctx); err != nil {
		return nil, err
	}
	if ok, err := r.schemaLooksInitialized(ctx); err != nil {
		return nil, err
	} else if !ok {
		return nil, errors.New("baseline requires an existing MoChat schema; run apply on empty databases")
	}
	applied, err := r.applied(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]StatusItem, 0, len(r.migrations))
	for _, migration := range r.migrations {
		_, checksum, err := migrationBodyAndChecksum(migration)
		if err != nil {
			return nil, err
		}
		if existing, ok := applied[migration.Version]; ok {
			item := StatusItem{Migration: migration, Checksum: checksum, Applied: &existing, State: "applied"}
			if !checksumMatches(existing.Checksum, checksum, migration.ChecksumAliases) {
				item.State = "checksum_mismatch"
				return append(result, item), fmt.Errorf("migration %s checksum mismatch: applied=%s current=%s", migration.Version, existing.Checksum, checksum)
			}
			result = append(result, item)
			continue
		}
		if err := r.recordApplied(ctx, migration, checksum, 0); err != nil {
			return result, err
		}
		appliedItem := AppliedMigration{
			Version:     migration.Version,
			Description: migration.Description,
			Checksum:    checksum,
			AppliedAt:   r.currentTime(),
		}
		result = append(result, StatusItem{Migration: migration, Checksum: checksum, Applied: &appliedItem, State: "baselined"})
	}
	return result, nil
}

func (r *Runner) RollbackLast(ctx context.Context) (string, error) {
	if err := r.ensureVersionTable(ctx); err != nil {
		return "", err
	}
	applied, err := r.applied(ctx)
	if err != nil {
		return "", err
	}
	for i := len(r.migrations) - 1; i >= 0; i-- {
		migration := r.migrations[i]
		existing, ok := applied[migration.Version]
		if !ok {
			continue
		}
		_, checksum, err := migrationBodyAndChecksum(migration)
		if err != nil {
			return "", err
		}
		if !checksumMatches(existing.Checksum, checksum, migration.ChecksumAliases) {
			return "", fmt.Errorf("migration %s checksum mismatch: applied=%s current=%s", migration.Version, existing.Checksum, checksum)
		}
		if strings.TrimSpace(migration.DownPath) == "" {
			return "", fmt.Errorf("rollback is not available for %s; create an explicit down migration before rolling back", migration.Version)
		}
		body, err := os.ReadFile(migration.DownPath)
		if err != nil {
			return "", err
		}
		if err := execSQLScript(ctx, r.db, string(body)); err != nil {
			return "", fmt.Errorf("rollback migration %s: %w", migration.Version, err)
		}
		if _, err := r.db.ExecContext(ctx, `DELETE FROM `+VersionTable+` WHERE version = ?`, migration.Version); err != nil {
			return "", err
		}
		return migration.Version, nil
	}
	return "", errors.New("no applied migrations to roll back")
}

func (r *Runner) ensureVersionTable(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS `+VersionTable+` (
			version varchar(64) NOT NULL,
			description varchar(255) NOT NULL DEFAULT '',
			checksum char(64) NOT NULL DEFAULT '',
			applied_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
			execution_ms int(10) unsigned NOT NULL DEFAULT 0,
			PRIMARY KEY (version)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
	`)
	return err
}

func (r *Runner) applied(ctx context.Context) (map[string]AppliedMigration, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT version, description, checksum, applied_at, execution_ms
		FROM `+VersionTable+`
		ORDER BY version ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[string]AppliedMigration{}
	for rows.Next() {
		var item AppliedMigration
		if err := rows.Scan(&item.Version, &item.Description, &item.Checksum, &item.AppliedAt, &item.ExecutionMS); err != nil {
			return nil, err
		}
		result[item.Version] = item
	}
	return result, rows.Err()
}

func (r *Runner) recordApplied(ctx context.Context, migration Migration, checksum string, executionMS int) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO `+VersionTable+` (version, description, checksum, applied_at, execution_ms)
		VALUES (?, ?, ?, NOW(), ?)
	`, migration.Version, migration.Description, checksum, executionMS)
	return err
}

func (r *Runner) schemaLooksInitialized(ctx context.Context) (bool, error) {
	required := []string{"mc_user", "mc_rbac_menu"}
	for _, table := range required {
		var count int
		if err := r.db.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM information_schema.tables
			WHERE table_schema = DATABASE() AND table_name = ?
		`, table).Scan(&count); err != nil {
			return false, err
		}
		if count == 0 {
			return false, nil
		}
	}
	return true, nil
}

func (r *Runner) currentTime() time.Time {
	if r.now != nil {
		return r.now()
	}
	return time.Now()
}

func migrationBodyAndChecksum(migration Migration) ([]byte, string, error) {
	body, err := os.ReadFile(migration.Path)
	if err != nil {
		return nil, "", err
	}
	return body, checksumBytes(body), nil
}

func checksumBytes(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func checksumMatches(applied, current string, aliases []string) bool {
	if applied == current {
		return true
	}
	for _, alias := range aliases {
		if applied == alias {
			return true
		}
	}
	return false
}

func legacyCombinedInitialChecksums(schemaPath, seedPath string) []string {
	schema, err := os.ReadFile(schemaPath)
	if err != nil {
		return nil
	}
	seed, err := os.ReadFile(seedPath)
	if err != nil {
		return nil
	}
	seedBody := standaloneSeedBodyForLegacyChecksum(string(seed))
	if strings.TrimSpace(seedBody) == "" {
		return nil
	}
	seedBody = strings.ReplaceAll(seedBody, "INSERT IGNORE INTO", "INSERT INTO")
	schemaBody := strings.TrimRight(string(schema), " \t\r\n")
	seedBody = strings.TrimLeft(seedBody, " \t\r\n")
	variants := []string{
		schemaBody + "\n\n\n" + seedBody,
		schemaBody + "\n\n" + seedBody,
		schemaBody + "\n" + seedBody,
	}
	const preWorkMessageGlobalIndexSchemaChecksum = "b7dbd66b24b93a4be64e33fa51d2e1a1fcbc0d305532145644c37ed1a26075e9"
	seen := map[string]bool{}
	checksums := []string{preWorkMessageGlobalIndexSchemaChecksum}
	seen[preWorkMessageGlobalIndexSchemaChecksum] = true
	for _, variant := range variants {
		checksum := checksumBytes([]byte(variant))
		if seen[checksum] {
			continue
		}
		seen[checksum] = true
		checksums = append(checksums, checksum)
	}
	return checksums
}

func standaloneSeedBodyForLegacyChecksum(seed string) string {
	marker := "-- ----------------------------\n-- 高级属性"
	if idx := strings.Index(seed, marker); idx >= 0 {
		return seed[idx:]
	}
	return seed
}

func standaloneIncrementalMigrations(projectRoot string) []Migration {
	migrationDir := filepath.Join(projectRoot, "deploy", "standalone", "migrations")
	entries, err := os.ReadDir(migrationDir)
	if err != nil {
		return nil
	}
	migrations := make([]Migration, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".up.sql") {
			continue
		}
		version := strings.TrimSuffix(name, ".up.sql")
		migrations = append(migrations, Migration{
			Version:     version,
			Description: migrationDescription(version),
			Path:        filepath.Join(migrationDir, name),
			DownPath:    filepath.Join(migrationDir, version+".down.sql"),
		})
	}
	return migrations
}

func migrationDescription(version string) string {
	parts := strings.SplitN(version, "_", 2)
	if len(parts) != 2 {
		return version
	}
	description := strings.ReplaceAll(parts[1], "_", " ")
	description = strings.TrimSpace(description)
	if description == "" {
		return version
	}
	return description
}

func execSQLScript(ctx context.Context, db *sql.DB, script string) error {
	statements, err := SplitSQLStatements(script)
	if err != nil {
		return err
	}
	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("%s: %w", compactStatement(statement), err)
		}
	}
	return nil
}

func validateMigrations(migrations []Migration) error {
	if len(migrations) == 0 {
		return errors.New("no migrations configured")
	}
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})
	seen := map[string]bool{}
	for _, migration := range migrations {
		if strings.TrimSpace(migration.Version) == "" {
			return errors.New("migration version is required")
		}
		if strings.TrimSpace(migration.Path) == "" {
			return fmt.Errorf("migration %s path is required", migration.Version)
		}
		if strings.TrimSpace(migration.DownPath) != "" {
			if _, err := os.Stat(migration.DownPath); err != nil {
				return fmt.Errorf("migration %s down path is not readable: %w", migration.Version, err)
			}
		}
		if seen[migration.Version] {
			return fmt.Errorf("duplicate migration version %s", migration.Version)
		}
		seen[migration.Version] = true
	}
	return nil
}

func compactStatement(statement string) string {
	statement = strings.Join(strings.Fields(statement), " ")
	if len(statement) > 140 {
		return statement[:140] + "..."
	}
	return statement
}
