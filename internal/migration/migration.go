package migration

import (
	"bytes"
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

	"jiyi/mochat-go/internal/sqlscript"
)

const VersionTable = "mochat_go_schema_migrations"

const composeInitBaselineVersion = "0104_scrm_opportunity_owner"

const composeInitPhase32Index = "idx_mc_corp_day_data_corp_date"

var composeInitCrossStageTables = []string{
	"mc_corp_day_data",
	"mochat_go_scrm_opportunities",
}

const knownLegacyInitialSchemaChecksum = "b7dbd66b24b93a4be64e33fa51d2e1a1fcbc0d305532145644c37ed1a26075e9"

// The standalone schema checksum immediately before the direct group join
// columns were added. Existing deployments legitimately retain this value in
// their immutable 0001 ledger while applying 0152 incrementally.
const knownPreviousInitialSchemaChecksum = "03425c87c5584e82b7991d7f5fe4c75f8918b31799e191dda7160a0cad150ace"

// The standalone seed shipped by the first identity-cutover deployment. Its
// statements are a compatible historical predecessor of the current
// idempotent seed and remain present in deployed migration ledgers.
const knownLegacyCoreSeedChecksum = "de6513fb142d38fbd0205ecaa6e6ddbdd9d765e5c455ab20bd2afdd158d276ed"

// The first server deployment of 0106 was built from a worktree containing
// mixed LF/CRLF line endings. The normalized SQL is identical to the current
// migration, but its immutable ledger retains this checksum.
const knownLegacySCRMLeadParityChecksum = "cf299bfb4ef21b95da0f76ee9e9c8cb24475843f575cfb749a2b23f09261496b"

// Only accept the historical mixed-line-ending checksum while the current
// migration still normalizes to the verified SQL that produced it.
const knownCanonicalSCRMLeadParityChecksum = "47cfa7915b970455f6a626806a56fea71f97dbd9056b22f1f375c42b55196266"

// The first Windows Docker Desktop deployment of 0153 was built from a
// worktree containing mixed LF/CRLF line endings. The SQL is byte-normalized
// to the current migration, but its immutable ledger retains this checksum.
const knownLegacyLiveCodeWorkspaceChecksum = "f17df230c78b79ed0e23d77b87057a939fa8ef5d1ac97fa1db43b5aa34f7344c"

// The original live-code workspace migration was published as 0150 on an
// integration branch, then renumbered to 0153 when the mainline sequence
// converged. Preserve the exact historical ledger fact instead of deleting it,
// but recognize it only when the audited old SQL checksum and valid 0153
// replacement are both present.
const knownSupersededLiveCodeVersion = "0150_live_code_workspace"
const knownSupersedingLiveCodeVersion = "0153_live_code_workspace"
const knownSupersededLiveCodeLFChecksum = "f89678394dea6164312152f9bbb5298111150d9dea8489252789ef7ed67117ed"
const knownSupersededLiveCodeCRLFChecksum = "5cce5bea0f7b89b7e607772c5aa02ffaf125af27027c79e18772642a4c7ab15d"

type Migration struct {
	Version         string
	Description     string
	Path            string
	DownPath        string
	ChecksumAliases []string
	Kind            MigrationKind
	Controlled      *ControlledMigration
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

type composeInitBaselineFacts struct {
	TenantCount             int64
	CorpCount               int64
	UserCount               int64
	MigrationLedgerCount    int64
	IdentityTableCount      int64
	HasOpportunityOwner     bool
	HasPhase32CorpDateIndex bool
	HasCrossStageTables     bool
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
		ChecksumAliases: legacyInitialSchemaChecksums(schemaPath, seedPath),
	}}
	migrations = append(migrations, standaloneIncrementalMigrations(projectRoot)...)
	return migrations
}

func legacyInitialSchemaChecksums(schemaPath, seedPath string) []string {
	aliases := legacyCombinedInitialChecksums(schemaPath, seedPath)
	for _, known := range []string{knownLegacyInitialSchemaChecksum, knownPreviousInitialSchemaChecksum} {
		found := false
		for _, alias := range aliases {
			if alias == known {
				found = true
				break
			}
		}
		if !found {
			aliases = append(aliases, known)
		}
	}
	return aliases
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
			if migration.Kind == MigrationControlled {
				if err := controlledMigrationBaselineEvidence(ctx, r.db, migration, checksum); err != nil {
					item.State = "controlled_incomplete"
					return append(result, item), err
				}
			}
			result = append(result, item)
			continue
		}
		if migration.Kind == MigrationControlled {
			return result, ControlledMigrationBlocked(migration.Version)
		}
		executionMS, err := r.execMigrationScript(ctx, migration, string(body), checksum)
		if err != nil {
			return result, fmt.Errorf("apply migration %s: %w", migration.Version, err)
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
	return r.statusItems(ctx, applied)
}

// StatusReadOnly inspects migration state without creating or changing the
// ledger. A missing ledger is an unambiguous pending state for readiness.
func (r *Runner) StatusReadOnly(ctx context.Context) ([]StatusItem, error) {
	count, err := r.informationSchemaTableCount(ctx, []string{VersionTable})
	if err != nil {
		return nil, err
	}
	if count > 1 {
		return nil, fmt.Errorf("inspect %s returned invalid table count %d", VersionTable, count)
	}
	applied := map[string]AppliedMigration{}
	if count == 1 {
		applied, err = r.applied(ctx)
		if err != nil {
			return nil, err
		}
	}
	return r.statusItems(ctx, applied)
}

func (r *Runner) statusItems(ctx context.Context, applied map[string]AppliedMigration) ([]StatusItem, error) {
	result := make([]StatusItem, 0, len(r.migrations))
	known := make(map[string]struct{}, len(r.migrations))
	validApplied := make(map[string]bool, len(r.migrations))
	for _, migration := range r.migrations {
		known[migration.Version] = struct{}{}
		_, checksum, err := migrationBodyAndChecksum(migration)
		if err != nil {
			return nil, err
		}
		state := "pending"
		if migration.Kind == MigrationControlled {
			state = "controlled_pending"
		}
		item := StatusItem{Migration: migration, Checksum: checksum, State: state}
		if existing, ok := applied[migration.Version]; ok {
			item.Applied = &existing
			item.State = "applied"
			if !checksumMatches(existing.Checksum, checksum, migration.ChecksumAliases) {
				item.State = "checksum_mismatch"
			} else if migration.Kind == MigrationControlled {
				if err := controlledMigrationBaselineEvidence(ctx, r.db, migration, checksum); err != nil {
					item.State = "controlled_incomplete"
					return append(result, item), err
				}
			}
			validApplied[migration.Version] = item.State == "applied"
		}
		result = append(result, item)
	}
	unknown := make([]string, 0)
	for version := range applied {
		if _, ok := known[version]; !ok {
			unknown = append(unknown, version)
		}
	}
	sort.Strings(unknown)
	for _, version := range unknown {
		existing := applied[version]
		state := "database_ahead"
		if version == knownSupersededLiveCodeVersion && validApplied[knownSupersedingLiveCodeVersion] &&
			(existing.Checksum == knownSupersededLiveCodeLFChecksum || existing.Checksum == knownSupersededLiveCodeCRLFChecksum) {
			state = "superseded"
		}
		result = append(result, StatusItem{
			Migration: Migration{Version: existing.Version, Description: existing.Description},
			Checksum:  existing.Checksum,
			Applied:   &existing,
			State:     state,
		})
	}
	return result, nil
}

func (r *Runner) Baseline(ctx context.Context) ([]StatusItem, error) {
	return r.baseline(ctx, "")
}

// BaselineComposeInit records only the migrations already executed by the
// standalone MariaDB init scripts. Later migrations must be applied by the
// runner so their DDL is actually present before they are ledgered.
func (r *Runner) BaselineComposeInit(ctx context.Context) ([]StatusItem, error) {
	return r.baseline(ctx, composeInitBaselineVersion)
}

func (r *Runner) baseline(ctx context.Context, throughVersion string) ([]StatusItem, error) {
	var schemaReady bool
	var err error
	if throughVersion == composeInitBaselineVersion {
		schemaReady, err = r.schemaHasComposeInitTables(ctx)
	} else {
		if err := r.ensureVersionTable(ctx); err != nil {
			return nil, err
		}
		schemaReady, err = r.schemaLooksInitialized(ctx)
	}
	if err != nil {
		return nil, err
	}
	if !schemaReady {
		return nil, errors.New("baseline requires an existing MoChat schema; run apply on empty databases")
	}
	if throughVersion == composeInitBaselineVersion {
		if err := r.ensureVersionTable(ctx); err != nil {
			return nil, err
		}
	}
	applied, err := r.applied(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]StatusItem, 0, len(r.migrations))
	foundThroughVersion := throughVersion == ""
	type pendingRecord struct {
		migration Migration
		checksum  string
	}
	pending := make([]pendingRecord, 0, len(r.migrations))
	var blockedErr error
	for _, migration := range r.migrations {
		if throughVersion != "" && migration.Version > throughVersion {
			break
		}
		if migration.Version == throughVersion {
			foundThroughVersion = true
		}
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
			if migration.Kind == MigrationControlled {
				if err := controlledMigrationBaselineEvidence(ctx, r.db, migration, checksum); err != nil {
					item.State = "controlled_incomplete"
					return append(result, item), err
				}
			}
			result = append(result, item)
			continue
		}
		if migration.Kind == MigrationControlled {
			if throughVersion != "" {
				blockedErr = ControlledMigrationBlocked(migration.Version)
				break
			}
			if err := controlledMigrationBaselineEvidence(ctx, r.db, migration, checksum); err != nil {
				blockedErr = err
				break
			}
		}
		pending = append(pending, pendingRecord{migration: migration, checksum: checksum})
		appliedItem := AppliedMigration{
			Version:     migration.Version,
			Description: migration.Description,
			Checksum:    checksum,
			AppliedAt:   r.currentTime(),
		}
		result = append(result, StatusItem{Migration: migration, Checksum: checksum, Applied: &appliedItem, State: "baselined"})
	}
	if !foundThroughVersion {
		return nil, fmt.Errorf("baseline cutoff migration %s is not configured", throughVersion)
	}
	if len(pending) > 0 {
		tx, err := r.db.BeginTx(ctx, nil)
		if err != nil {
			return nil, fmt.Errorf("begin baseline transaction: %w", err)
		}
		for _, record := range pending {
			if err := recordAppliedWith(ctx, tx, record.migration, record.checksum, 0); err != nil {
				_ = tx.Rollback()
				return nil, fmt.Errorf("record baseline migration %s: %w", record.migration.Version, err)
			}
		}
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("commit baseline transaction: %w", err)
		}
	}
	return result, blockedErr
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
		if migration.Kind == MigrationControlled {
			return "", ControlledMigrationRollbackRequired(migration.Version)
		}
		if strings.TrimSpace(migration.DownPath) == "" {
			return "", fmt.Errorf("rollback is not available for %s; create an explicit down migration before rolling back", migration.Version)
		}
		body, err := os.ReadFile(migration.DownPath)
		if err != nil {
			return "", err
		}
		if err := r.execRollbackScript(ctx, migration, string(body)); err != nil {
			return "", fmt.Errorf("rollback migration %s: %w", migration.Version, err)
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
	return recordAppliedWith(ctx, r.db, migration, checksum, executionMS)
}

func (r *Runner) execMigrationScript(ctx context.Context, migration Migration, body, checksum string) (int, error) {
	conn, err := r.db.Conn(ctx)
	if err != nil {
		return 0, fmt.Errorf("pin migration connection %s: %w", migration.Version, err)
	}
	defer conn.Close()
	body = runtimeCompatibleMigrationBody(migration.Version, body)
	if automaticMigrationNeedsServerDetection(body) {
		var serverVersion string
		if err := conn.QueryRowContext(ctx, "SELECT VERSION()").Scan(&serverVersion); err != nil {
			return 0, fmt.Errorf("read database version for %s: %w", migration.Version, err)
		}
		if strings.HasPrefix(strings.TrimSpace(serverVersion), "5.7.") {
			start := r.currentTime()
			if err := execSQLScriptMySQL57(ctx, conn, body); err != nil {
				return 0, err
			}
			executionMS := int(r.currentTime().Sub(start).Milliseconds())
			if executionMS < 0 {
				executionMS = 0
			}
			if err := recordAppliedWith(ctx, conn, migration, checksum, executionMS); err != nil {
				return 0, err
			}
			return executionMS, nil
		}
	}
	start := r.currentTime()
	if err := execSQLScriptWithExecutor(ctx, conn, body); err != nil {
		return 0, err
	}
	executionMS := int(r.currentTime().Sub(start).Milliseconds())
	if executionMS < 0 {
		executionMS = 0
	}
	if err := recordAppliedWith(ctx, conn, migration, checksum, executionMS); err != nil {
		return 0, err
	}
	return executionMS, nil
}

func runtimeCompatibleMigrationBody(version, body string) string {
	if version == "0152_group_code_direct_join" {
		return strings.ReplaceAll(body, "ADD COLUMN `", "ADD COLUMN IF NOT EXISTS `")
	}
	return body
}

func (r *Runner) execRollbackScript(ctx context.Context, migration Migration, body string) error {
	conn, err := r.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("pin rollback connection %s: %w", migration.Version, err)
	}
	defer conn.Close()
	if automaticMigrationNeedsServerDetection(body) {
		var serverVersion string
		if err := conn.QueryRowContext(ctx, "SELECT VERSION()").Scan(&serverVersion); err != nil {
			return fmt.Errorf("read database version for rollback %s: %w", migration.Version, err)
		}
		if strings.HasPrefix(strings.TrimSpace(serverVersion), "5.7.") {
			if err := execSQLScriptMySQL57(ctx, conn, body); err != nil {
				return err
			}
			_, err = conn.ExecContext(ctx, `DELETE FROM `+VersionTable+` WHERE version = ?`, migration.Version)
			return err
		}
	}
	if err := execSQLScriptWithExecutor(ctx, conn, body); err != nil {
		return err
	}
	_, err = conn.ExecContext(ctx, `DELETE FROM `+VersionTable+` WHERE version = ?`, migration.Version)
	return err
}

func automaticMigrationNeedsServerDetection(body string) bool {
	return strings.Contains(body, "ADD COLUMN IF NOT EXISTS") || strings.Contains(body, "ADD UNIQUE KEY IF NOT EXISTS") ||
		strings.Contains(body, "ADD INDEX IF NOT EXISTS") || strings.Contains(body, "ADD KEY IF NOT EXISTS") ||
		strings.Contains(body, "DROP COLUMN IF EXISTS") || strings.Contains(body, "DROP INDEX IF EXISTS")
}

type migrationQueryExecer interface {
	migrationExecer
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func execSQLScriptMySQL57(ctx context.Context, execer migrationQueryExecer, script string) error {
	statements, err := SplitSQLStatements(script)
	if err != nil {
		return err
	}
	for _, statement := range statements {
		compatible, err := mysql57ConditionalAlterStatement(ctx, execer, statement)
		if err != nil {
			return err
		}
		if strings.TrimSpace(compatible) == "" {
			continue
		}
		if err := sqlscript.ExecuteStatement(ctx, execer, compatible); err != nil {
			return fmt.Errorf("%s: %w", compactStatement(compatible), err)
		}
	}
	return nil
}

func mysql57ConditionalAlterStatement(ctx context.Context, queryer migrationQueryExecer, statement string) (string, error) {
	trimmed := strings.TrimSpace(statement)
	if !strings.HasPrefix(strings.ToUpper(trimmed), "ALTER TABLE ") || !automaticMigrationNeedsServerDetection(trimmed) {
		return statement, nil
	}
	rest := strings.TrimSpace(trimmed[len("ALTER TABLE "):])
	tableToken, clausesBody := firstSQLToken(rest)
	tableName := strings.Trim(tableToken, "`")
	if tableName == "" || strings.TrimSpace(clausesBody) == "" {
		return "", errors.New("invalid conditional ALTER TABLE statement")
	}
	clauses := splitSQLTopLevelCommas(clausesBody)
	kept := make([]string, 0, len(clauses))
	for _, clause := range clauses {
		compatible, keep, err := mysql57ConditionalAlterClause(ctx, queryer, tableName, clause)
		if err != nil {
			return "", err
		}
		if keep {
			kept = append(kept, compatible)
		}
	}
	if len(kept) == 0 {
		return "", nil
	}
	return "ALTER TABLE " + tableToken + "\n  " + strings.Join(kept, ",\n  "), nil
}

func mysql57ConditionalAlterClause(ctx context.Context, queryer migrationQueryExecer, tableName, clause string) (string, bool, error) {
	trimmed := strings.TrimSpace(clause)
	upper := strings.ToUpper(trimmed)
	type conditional struct {
		prefix, replacement, catalog string
		add                          bool
	}
	conditions := []conditional{
		{"ADD COLUMN IF NOT EXISTS ", "ADD COLUMN ", "COLUMNS", true},
		{"ADD UNIQUE INDEX IF NOT EXISTS ", "ADD UNIQUE INDEX ", "STATISTICS", true},
		{"ADD UNIQUE KEY IF NOT EXISTS ", "ADD UNIQUE KEY ", "STATISTICS", true},
		{"ADD INDEX IF NOT EXISTS ", "ADD INDEX ", "STATISTICS", true},
		{"ADD KEY IF NOT EXISTS ", "ADD KEY ", "STATISTICS", true},
		{"DROP COLUMN IF EXISTS ", "DROP COLUMN ", "COLUMNS", false},
		{"DROP INDEX IF EXISTS ", "DROP INDEX ", "STATISTICS", false},
	}
	for _, condition := range conditions {
		if !strings.HasPrefix(upper, condition.prefix) {
			continue
		}
		identifier, _ := firstSQLToken(strings.TrimSpace(trimmed[len(condition.prefix):]))
		identifier = strings.Trim(identifier, "`")
		column := "COLUMN_NAME"
		if condition.catalog == "STATISTICS" {
			column = "INDEX_NAME"
		}
		var count int
		query := "SELECT COUNT(*) FROM information_schema." + condition.catalog + " WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=? AND " + column + "=?"
		if err := queryer.QueryRowContext(ctx, query, tableName, identifier).Scan(&count); err != nil {
			return "", false, err
		}
		if (condition.add && count > 0) || (!condition.add && count == 0) {
			return "", false, nil
		}
		return condition.replacement + strings.TrimSpace(trimmed[len(condition.prefix):]), true, nil
	}
	return trimmed, true, nil
}

func firstSQLToken(value string) (string, string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", ""
	}
	if value[0] == '`' {
		if end := strings.Index(value[1:], "`"); end >= 0 {
			end++
			return value[:end+1], strings.TrimSpace(value[end+1:])
		}
	}
	if end := strings.IndexAny(value, " \t\r\n"); end >= 0 {
		return value[:end], strings.TrimSpace(value[end:])
	}
	return value, ""
}

func splitSQLTopLevelCommas(value string) []string {
	var result []string
	start, depth := 0, 0
	var quote byte
	for i := 0; i < len(value); i++ {
		current := value[i]
		if quote != 0 {
			if current == quote && (i == 0 || value[i-1] != '\\') {
				quote = 0
			}
			continue
		}
		if current == '\'' || current == '"' || current == '`' {
			quote = current
			continue
		}
		if current == '(' {
			depth++
		} else if current == ')' && depth > 0 {
			depth--
		} else if current == ',' && depth == 0 {
			result = append(result, strings.TrimSpace(value[start:i]))
			start = i + 1
		}
	}
	result = append(result, strings.TrimSpace(value[start:]))
	return result
}

type migrationExecer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func recordAppliedWith(ctx context.Context, execer migrationExecer, migration Migration, checksum string, executionMS int) error {
	_, err := execer.ExecContext(ctx, `
		INSERT INTO `+VersionTable+` (version, description, checksum, applied_at, execution_ms)
		VALUES (?, ?, ?, NOW(), ?)
	`, migration.Version, migration.Description, checksum, executionMS)
	return err
}

func (r *Runner) schemaLooksInitialized(ctx context.Context) (bool, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = DATABASE()
		  AND table_name IN (`+placeholders(len(baselineSchemaTables))+`)
	`, baselineSchemaTableArgs()...)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	present := make([]string, 0, len(baselineSchemaTables))
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			return false, err
		}
		present = append(present, table)
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	if !containsString(present, "mc_user") || !containsString(present, "mc_rbac_menu") {
		return false, nil
	}
	if err := validateBaselineSchemaTables(present); err != nil {
		return false, err
	}
	return true, nil
}

func (r *Runner) schemaHasComposeInitTables(ctx context.Context) (bool, error) {
	facts := composeInitBaselineFacts{}
	var err error
	if facts.TenantCount, err = r.tableRowCount(ctx, "mc_tenant"); err != nil {
		return false, err
	}
	if facts.CorpCount, err = r.tableRowCount(ctx, "mc_corp"); err != nil {
		return false, err
	}
	if facts.UserCount, err = r.tableRowCount(ctx, "mc_user"); err != nil {
		return false, err
	}
	var ledgerTableCount int64
	if ledgerTableCount, err = r.informationSchemaTableCount(ctx, []string{VersionTable}); err != nil {
		return false, err
	}
	if ledgerTableCount > 1 {
		return false, fmt.Errorf("inspect %s returned invalid table count %d", VersionTable, ledgerTableCount)
	}
	if ledgerTableCount == 1 {
		if facts.MigrationLedgerCount, err = r.tableRowCount(ctx, VersionTable); err != nil {
			return false, err
		}
	}
	if facts.IdentityTableCount, err = r.informationSchemaTableCount(ctx, []string{"mochat_go_saas_admin_users"}); err != nil {
		return false, err
	}
	if facts.HasOpportunityOwner, err = r.informationSchemaColumnExists(ctx, "mochat_go_scrm_opportunities", "owner_id", "bigint(20) unsigned"); err != nil {
		return false, err
	}
	if facts.HasPhase32CorpDateIndex, err = r.informationSchemaPhase32IndexExists(ctx); err != nil {
		return false, err
	}
	var crossStageTableCount int64
	if crossStageTableCount, err = r.informationSchemaTableCount(ctx, composeInitCrossStageTables); err != nil {
		return false, err
	}
	facts.HasCrossStageTables = crossStageTableCount == int64(len(composeInitCrossStageTables))
	if err := validateComposeInitBaselineFacts(facts); err != nil {
		return false, err
	}
	return true, nil
}

func validateComposeInitBaselineFacts(facts composeInitBaselineFacts) error {
	if facts.TenantCount != 0 || facts.CorpCount != 0 || facts.UserCount != 0 {
		return fmt.Errorf("baseline compose init requires an empty business schema: mc_tenant=%d mc_corp=%d mc_user=%d", facts.TenantCount, facts.CorpCount, facts.UserCount)
	}
	if !facts.HasOpportunityOwner || !facts.HasPhase32CorpDateIndex || !facts.HasCrossStageTables {
		return errors.New("baseline compose init requires the 0104 schema sentinels: opportunity owner, 0103 index, and cross-stage tables")
	}
	if facts.IdentityTableCount != 0 {
		return fmt.Errorf("baseline compose init cannot run when 0129 identity tables exist: tables=%d", facts.IdentityTableCount)
	}
	if facts.MigrationLedgerCount != 0 {
		return fmt.Errorf("baseline compose init requires an empty migration ledger: rows=%d", facts.MigrationLedgerCount)
	}
	return nil
}

func (r *Runner) tableRowCount(ctx context.Context, table string) (int64, error) {
	var count int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table).Scan(&count); err != nil {
		return 0, fmt.Errorf("count %s: %w", table, err)
	}
	return count, nil
}

func (r *Runner) informationSchemaTableCount(ctx context.Context, tables []string) (int64, error) {
	var count int64
	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM information_schema.tables
		WHERE table_schema = DATABASE()
		  AND table_name IN (`+placeholders(len(tables))+`)
	`, stringArgs(tables)...).Scan(&count); err != nil {
		return 0, fmt.Errorf("inspect information_schema.tables: %w", err)
	}
	return count, nil
}

func (r *Runner) informationSchemaColumnExists(ctx context.Context, table, column, columnType string) (bool, error) {
	var count int64
	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM information_schema.columns
		WHERE table_schema = DATABASE()
		  AND table_name = ?
		  AND column_name = ?
		  AND column_type = ?
	`, table, column, columnType).Scan(&count); err != nil {
		return false, fmt.Errorf("inspect %s.%s: %w", table, column, err)
	}
	return count == 1, nil
}

func (r *Runner) informationSchemaPhase32IndexExists(ctx context.Context) (bool, error) {
	var count, matchingFirst, matchingSecond int64
	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*),
		       COALESCE(SUM(CASE WHEN seq_in_index = 1 AND column_name = 'corp_id' THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN seq_in_index = 2 AND column_name = 'date' THEN 1 ELSE 0 END), 0)
		FROM information_schema.statistics
		WHERE table_schema = DATABASE()
		  AND table_name = 'mc_corp_day_data'
		  AND index_name = ?
	`, composeInitPhase32Index).Scan(&count, &matchingFirst, &matchingSecond); err != nil {
		return false, fmt.Errorf("inspect %s: %w", composeInitPhase32Index, err)
	}
	return count == 2 && matchingFirst == 1 && matchingSecond == 1, nil
}

// baselineSchemaTables is the minimum schema contract for baseline. A
// baseline is only valid for a database that already has the latest
// automatic schema through 0129; it must never turn a compose init schema
// (which stops at 0104) into a false ledger claim for 0127/0129.
var baselineSchemaTables = []string{
	"mc_user",
	"mc_rbac_menu",
	"mochat_go_dashboard_permissions",
	"mochat_go_dashboard_permission_resources",
	"mochat_go_dashboard_user_roles",
	"mochat_go_dashboard_role_permissions",
	"mochat_go_dashboard_user_permissions",
	"mochat_go_dashboard_permission_audits",
	"mochat_go_saas_admin_users",
	"mochat_go_saas_idempotency_receipts",
	"mochat_go_dashboard_identities",
	"mochat_go_dashboard_identity_activations",
	"mochat_go_tenant_corp_bindings",
	"mochat_go_dashboard_mfa_credentials",
	"mochat_go_dashboard_mfa_challenges",
	"mochat_go_dashboard_sessions",
	"mochat_go_dashboard_password_resets",
	"mochat_go_saas_admin_mfa_credentials",
	"mochat_go_saas_admin_mfa_challenges",
	"mochat_go_saas_admin_sessions",
}

func validateBaselineSchemaTables(present []string) error {
	available := make(map[string]struct{}, len(present))
	for _, table := range present {
		available[table] = struct{}{}
	}
	missing := make([]string, 0)
	for _, required := range baselineSchemaTables {
		if _, ok := available[required]; !ok {
			missing = append(missing, required)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("baseline requires complete 0129 schema; missing tables: %s", strings.Join(missing, ","))
	}
	return nil
}

func baselineSchemaTableArgs() []any {
	return stringArgs(baselineSchemaTables)
}

func stringArgs(values []string) []any {
	args := make([]any, 0, len(values))
	for _, value := range values {
		args = append(args, value)
	}
	return args
}

func placeholders(count int) string {
	if count <= 0 {
		return ""
	}
	result := strings.Repeat("?,", count)
	return strings.TrimSuffix(result, ",")
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
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
	seen := map[string]bool{}
	checksums := make([]string, 0, len(variants))
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
		kind, controlled := MigrationMetadata(version)
		path := filepath.Join(migrationDir, name)
		checksumAliases := migrationLineEndingChecksumAliases(path)
		if version == "0002_seed_core_data" {
			checksumAliases = append(checksumAliases, knownLegacyCoreSeedChecksum)
		}
		if version == "0106_scrm_lead_parity" && migrationNormalizedChecksum(path) == knownCanonicalSCRMLeadParityChecksum {
			checksumAliases = append(checksumAliases, knownLegacySCRMLeadParityChecksum)
		}
		if version == "0153_live_code_workspace" {
			checksumAliases = append(checksumAliases, knownLegacyLiveCodeWorkspaceChecksum)
		}
		migrations = append(migrations, Migration{
			Version:         version,
			Description:     migrationDescription(version),
			Path:            path,
			DownPath:        filepath.Join(migrationDir, version+".down.sql"),
			ChecksumAliases: checksumAliases,
			Kind:            kind,
			Controlled:      controlled,
		})
	}
	return migrations
}

func migrationNormalizedChecksum(path string) string {
	body, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return checksumBytes(bytes.ReplaceAll(body, []byte("\r\n"), []byte("\n")))
}

func migrationLineEndingChecksumAliases(path string) []string {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	normalized := bytes.ReplaceAll(body, []byte("\r\n"), []byte("\n"))
	crlf := bytes.ReplaceAll(normalized, []byte("\n"), []byte("\r\n"))
	aliases := make([]string, 0, 2)
	for _, variant := range [][]byte{normalized, crlf} {
		if bytes.Equal(body, variant) {
			continue
		}
		checksum := checksumBytes(variant)
		if checksum == checksumBytes(body) {
			continue
		}
		alreadyAdded := false
		for _, alias := range aliases {
			if alias == checksum {
				alreadyAdded = true
				break
			}
		}
		if !alreadyAdded {
			aliases = append(aliases, checksum)
		}
	}
	return aliases
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
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	return execSQLScriptWithExecutor(ctx, conn, script)
}

func execSQLScriptWithExecutor(ctx context.Context, execer migrationQueryExecer, script string) error {
	statements, err := SplitSQLStatements(script)
	if err != nil {
		return err
	}
	for _, statement := range statements {
		if err := sqlscript.ExecuteStatement(ctx, execer, statement); err != nil {
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
	for i := range migrations {
		migration := &migrations[i]
		kind, controlled := MigrationMetadata(migration.Version)
		if controlled != nil {
			migration.Kind = kind
			migration.Controlled = controlled
		} else if migration.Kind == "" {
			migration.Kind = MigrationAutomatic
		}
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
