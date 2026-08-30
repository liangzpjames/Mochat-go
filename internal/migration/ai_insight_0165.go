package migration

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const (
	AIInsight0165Version              = "0165_ai_daily_insight_unification"
	aiInsight0165ControlTable         = "mochat_go_controlled_migration_0165"
	aiInsight0165SourceTable          = "mochat_go_ai_conversation_insights"
	aiInsight0165LegacyTable          = "mochat_go_ai_analysis"
	aiInsight0165BackupSourceTable    = "mochat_go_backup_0165_ai_conversation_insights"
	aiInsight0165BackupLegacyTable    = "mochat_go_backup_0165_ai_analysis"
	aiInsight0165AdoptedStatus        = "adopted_existing"
	aiInsight0165RecoveryBoundaryText = "0165 was already recorded; historical rows deleted by an earlier uncontrolled execution cannot be reconstructed without a verified pre-0165 backup"
)

var (
	ErrAIInsight0165AlreadyApplied    = errors.New("0165 controlled migration is already applied")
	ErrAIInsight0165BackupMissing     = errors.New("0165 controlled migration backup is missing")
	ErrAIInsight0165BackupDrift       = errors.New("0165 controlled migration backup digest changed")
	ErrAIInsight0165SnapshotDrift     = errors.New("0165 controlled migration source snapshot changed")
	ErrAIInsight0165WrongSchema       = errors.New("0165 controlled migration schema is not the expected pre-0165 schema")
	ErrAIInsight0165ApprovalMismatch  = errors.New("0165 controlled migration approval does not match the verified snapshot")
	ErrAIInsight0165SurvivorDrift     = errors.New("0165 controlled migration survivor rows changed")
	ErrAIInsight0165TrafficNotStopped = errors.New("0165 controlled migration requires explicit traffic-stopped confirmation")
	ErrAIInsight0165ConcurrentRun     = errors.New("0165 controlled migration is already running")
	ErrAIInsight0165AdoptionConflict  = errors.New("0165 historical adoption conflicts with existing control evidence")
)

var aiInsight0165RequestPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)
var aiInsight0165SHA256Pattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

type AIInsight0165Inventory struct {
	SchemaName          string `json:"schemaName"`
	MigrationChecksum   string `json:"migrationChecksum"`
	Applied             bool   `json:"applied"`
	RecoveryBoundary    string `json:"recoveryBoundary,omitempty"`
	InsightRows         int64  `json:"insightRows"`
	InsightDigest       string `json:"insightDigest"`
	DuplicateRows       int64  `json:"duplicateRows"`
	LegacyRows          int64  `json:"legacyRows"`
	LegacyDigest        string `json:"legacyDigest"`
	BackupInsightRows   int64  `json:"backupInsightRows"`
	BackupInsightDigest string `json:"backupInsightDigest,omitempty"`
	BackupLegacyRows    int64  `json:"backupLegacyRows"`
	BackupLegacyDigest  string `json:"backupLegacyDigest,omitempty"`
	BackupPresent       bool   `json:"backupPresent"`
}

type AIInsight0165Preflight struct {
	AIInsight0165Inventory
	RequestID           string `json:"requestId"`
	ApprovalToken       string `json:"approvalToken"`
	DestructiveApproval string `json:"destructiveApproval"`
}

type AIInsight0165ApplyRequest struct {
	RequestID           string
	ApprovalToken       string
	DestructiveApproval string
	TrafficStopped      bool
}

type AIInsight0165AdoptExistingRequest struct {
	RequestID            string
	ExternalBackupSHA256 string
	TrafficStopped       bool
}

type AIInsight0165AdoptionResult struct {
	RequestID                string `json:"requestId"`
	Adopted                  bool   `json:"adopted"`
	Verified                 bool   `json:"verified"`
	SchemaName               string `json:"schemaName"`
	MigrationChecksum        string `json:"migrationChecksum"`
	AppliedMigrationChecksum string `json:"appliedMigrationChecksum"`
	ExternalBackupSHA256     string `json:"externalBackupSha256"`
	InsightRows              int64  `json:"insightRows"`
	InsightDigest            string `json:"insightDigest"`
	RecoveryBoundary         string `json:"recoveryBoundary"`
}

type AIInsight0165ApplyResult struct {
	RequestID            string `json:"requestId"`
	Applied              bool   `json:"applied"`
	Verified             bool   `json:"verified"`
	RetainedInsightRows  int64  `json:"retainedInsightRows"`
	RemovedDuplicateRows int64  `json:"removedDuplicateRows"`
	RemovedLegacyRows    int64  `json:"removedLegacyRows"`
	BackupInsightRows    int64  `json:"backupInsightRows"`
	BackupLegacyRows     int64  `json:"backupLegacyRows"`
	MigrationChecksum    string `json:"migrationChecksum"`
	RecoveryBoundary     string `json:"recoveryBoundary,omitempty"`
}

type AIInsight0165Controller struct {
	db          *sql.DB
	projectRoot string
	migration   Migration
	checksum    string
	body        string
}

type aiInsight0165Queryer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type aiInsight0165Manifest struct {
	RequestID           string
	SchemaName          string
	MigrationChecksum   string
	InsightRows         int64
	InsightDigest       string
	DuplicateRows       int64
	LegacyRows          int64
	LegacyDigest        string
	BackupInsightRows   int64
	BackupInsightDigest string
	BackupLegacyRows    int64
	BackupLegacyDigest  string
	Status              string
}

func NewAIInsight0165Controller(db *sql.DB, projectRoot string) (*AIInsight0165Controller, error) {
	if db == nil {
		return nil, errors.New("0165 controlled migration database is required")
	}
	projectRoot = strings.TrimSpace(projectRoot)
	if projectRoot == "" {
		return nil, errors.New("0165 controlled migration project root is required")
	}
	var target Migration
	for _, migration := range DefaultMigrations(projectRoot) {
		if migration.Version == AIInsight0165Version {
			target = migration
			break
		}
	}
	if target.Version == "" {
		return nil, fmt.Errorf("%s is not present in DefaultMigrations", AIInsight0165Version)
	}
	body, checksum, err := migrationBodyAndChecksum(target)
	if err != nil {
		return nil, err
	}
	return &AIInsight0165Controller{db: db, projectRoot: projectRoot, migration: target, checksum: checksum, body: string(body)}, nil
}

func (c *AIInsight0165Controller) Inventory(ctx context.Context) (AIInsight0165Inventory, error) {
	if c == nil || c.db == nil {
		return AIInsight0165Inventory{}, errors.New("0165 controlled migration controller is not initialized")
	}
	return c.inventoryWith(ctx, c.db)
}

func (c *AIInsight0165Controller) Backup(ctx context.Context, requestID string) (AIInsight0165Inventory, error) {
	requestID, err := validateAIInsight0165RequestID(requestID)
	if err != nil {
		return AIInsight0165Inventory{}, err
	}
	conn, release, err := c.lockedConnection(ctx)
	if err != nil {
		return AIInsight0165Inventory{}, err
	}
	defer release()

	before, err := c.inventoryWith(ctx, conn)
	if err != nil {
		return before, err
	}
	for _, table := range []string{aiInsight0165ControlTable, aiInsight0165BackupSourceTable, aiInsight0165BackupLegacyTable} {
		exists, tableErr := aiInsight0165TableExists(ctx, conn, table)
		if tableErr != nil {
			return before, tableErr
		}
		if exists {
			return before, fmt.Errorf("%w: table %s already exists; preserve it for audit and use a clean schema or reviewed recovery procedure", ErrAIInsight0165BackupDrift, table)
		}
	}
	if err := createAIInsight0165ControlTable(ctx, conn); err != nil {
		return before, err
	}
	if _, err := conn.ExecContext(ctx, `CREATE TABLE `+aiInsight0165BackupSourceTable+` LIKE `+aiInsight0165SourceTable); err != nil {
		return before, fmt.Errorf("create 0165 insight backup: %w", err)
	}
	if _, err := conn.ExecContext(ctx, `INSERT INTO `+aiInsight0165BackupSourceTable+` SELECT * FROM `+aiInsight0165SourceTable); err != nil {
		return before, fmt.Errorf("copy 0165 insight backup: %w", err)
	}
	legacyExists, err := aiInsight0165TableExists(ctx, conn, aiInsight0165LegacyTable)
	if err != nil {
		return before, err
	}
	if legacyExists {
		if _, err := conn.ExecContext(ctx, `CREATE TABLE `+aiInsight0165BackupLegacyTable+` LIKE `+aiInsight0165LegacyTable); err != nil {
			return before, fmt.Errorf("create 0165 legacy backup: %w", err)
		}
		if _, err := conn.ExecContext(ctx, `INSERT INTO `+aiInsight0165BackupLegacyTable+` SELECT * FROM `+aiInsight0165LegacyTable); err != nil {
			return before, fmt.Errorf("copy 0165 legacy backup: %w", err)
		}
	}
	after, err := c.inventoryWith(ctx, conn)
	if err != nil {
		return after, err
	}
	if !sameAIInsight0165Source(before, after) {
		return after, fmt.Errorf("%w while backup was being created", ErrAIInsight0165SnapshotDrift)
	}
	backupSourceRows, backupSourceDigest, err := aiInsight0165TableDigest(ctx, conn, aiInsight0165BackupSourceTable)
	if err != nil {
		return after, err
	}
	var backupLegacyRows int64
	backupLegacyDigest := emptyAIInsight0165Digest()
	if legacyExists {
		backupLegacyRows, backupLegacyDigest, err = aiInsight0165TableDigest(ctx, conn, aiInsight0165BackupLegacyTable)
		if err != nil {
			return after, err
		}
	}
	if backupSourceRows != before.InsightRows || backupSourceDigest != before.InsightDigest || backupLegacyRows != before.LegacyRows || backupLegacyDigest != before.LegacyDigest {
		return after, fmt.Errorf("%w immediately after snapshot", ErrAIInsight0165BackupDrift)
	}
	_, err = conn.ExecContext(ctx, `
		INSERT INTO `+aiInsight0165ControlTable+` (
			request_id, schema_name, migration_checksum,
			insight_rows, insight_digest, duplicate_rows, legacy_rows, legacy_digest,
			backup_insight_rows, backup_insight_digest, backup_legacy_rows, backup_legacy_digest,
			status, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'backed_up', NOW(), NOW())
	`, requestID, before.SchemaName, before.MigrationChecksum,
		before.InsightRows, before.InsightDigest, before.DuplicateRows, before.LegacyRows, before.LegacyDigest,
		backupSourceRows, backupSourceDigest, backupLegacyRows, backupLegacyDigest)
	if err != nil {
		return after, fmt.Errorf("record 0165 backup manifest: %w", err)
	}
	after.BackupPresent = true
	after.BackupInsightRows = backupSourceRows
	after.BackupInsightDigest = backupSourceDigest
	after.BackupLegacyRows = backupLegacyRows
	after.BackupLegacyDigest = backupLegacyDigest
	return after, nil
}

func (c *AIInsight0165Controller) Preflight(ctx context.Context, requestID string) (AIInsight0165Preflight, error) {
	requestID, err := validateAIInsight0165RequestID(requestID)
	if err != nil {
		return AIInsight0165Preflight{}, err
	}
	conn, release, err := c.lockedConnection(ctx)
	if err != nil {
		return AIInsight0165Preflight{}, err
	}
	defer release()
	return c.preflightWith(ctx, conn, requestID)
}

func (c *AIInsight0165Controller) Apply(ctx context.Context, request AIInsight0165ApplyRequest) (AIInsight0165ApplyResult, error) {
	requestID, err := validateAIInsight0165RequestID(request.RequestID)
	if err != nil {
		return AIInsight0165ApplyResult{}, err
	}
	if !request.TrafficStopped {
		return AIInsight0165ApplyResult{}, ErrAIInsight0165TrafficNotStopped
	}
	conn, release, err := c.lockedConnection(ctx)
	if err != nil {
		return AIInsight0165ApplyResult{}, err
	}
	defer release()

	preflight, err := c.preflightWith(ctx, conn, requestID)
	if err != nil {
		return AIInsight0165ApplyResult{}, err
	}
	if request.ApprovalToken != preflight.ApprovalToken || request.DestructiveApproval != preflight.DestructiveApproval {
		return AIInsight0165ApplyResult{}, ErrAIInsight0165ApprovalMismatch
	}
	legacyTableExists, err := aiInsight0165TableExists(ctx, conn, aiInsight0165LegacyTable)
	if err != nil {
		return AIInsight0165ApplyResult{}, err
	}
	if err := installAIInsight0165WriteGuards(ctx, conn, requestID, legacyTableExists); err != nil {
		return AIInsight0165ApplyResult{}, err
	}
	defer dropAIInsight0165WriteGuards(context.WithoutCancel(ctx), conn)
	guarded, err := c.inventoryWith(ctx, conn)
	if err != nil {
		return AIInsight0165ApplyResult{}, err
	}
	if !sameAIInsight0165Source(preflight.AIInsight0165Inventory, guarded) {
		return AIInsight0165ApplyResult{}, ErrAIInsight0165SnapshotDrift
	}
	if _, err := conn.ExecContext(ctx, `UPDATE `+aiInsight0165ControlTable+` SET status = 'applying', updated_at = NOW() WHERE request_id = ? AND status = 'backed_up'`, requestID); err != nil {
		return AIInsight0165ApplyResult{}, fmt.Errorf("mark 0165 applying: %w", err)
	}
	started := time.Now()
	var serverVersion string
	if err := conn.QueryRowContext(ctx, `SELECT VERSION()`).Scan(&serverVersion); err != nil {
		return AIInsight0165ApplyResult{}, fmt.Errorf("inspect database version before 0165: %w", err)
	}
	executableBody := aiInsight0165BodyForServer(c.body, serverVersion)
	if err := execSQLScriptWithExecutor(ctx, conn, executableBody); err != nil {
		_, _ = conn.ExecContext(context.WithoutCancel(ctx), `UPDATE `+aiInsight0165ControlTable+` SET status = 'failed', updated_at = NOW() WHERE request_id = ?`, requestID)
		return AIInsight0165ApplyResult{}, fmt.Errorf("apply immutable 0165 script: %w", err)
	}
	executionMS := int(time.Since(started).Milliseconds())
	if executionMS < 0 {
		executionMS = 0
	}
	if _, err := conn.ExecContext(ctx, `UPDATE `+aiInsight0165ControlTable+` SET status = 'applied_unverified', updated_at = NOW() WHERE request_id = ?`, requestID); err != nil {
		return AIInsight0165ApplyResult{}, fmt.Errorf("mark 0165 applied: %w", err)
	}
	result, err := c.verifyAndRecordWith(ctx, conn, requestID, executionMS)
	if err != nil {
		return result, err
	}
	return result, nil
}

func (c *AIInsight0165Controller) Verify(ctx context.Context, requestID string) (AIInsight0165ApplyResult, error) {
	requestID, err := validateAIInsight0165RequestID(requestID)
	if err != nil {
		return AIInsight0165ApplyResult{}, err
	}
	conn, release, err := c.lockedConnection(ctx)
	if err != nil {
		return AIInsight0165ApplyResult{}, err
	}
	defer release()
	return c.verifyAndRecordWith(ctx, conn, requestID, 0)
}

// AdoptExisting records an honest compatibility boundary for environments
// that executed 0165 before it became controlled. It never claims that a
// verified pre-0165 backup exists.
func (c *AIInsight0165Controller) AdoptExisting(ctx context.Context, request AIInsight0165AdoptExistingRequest) (AIInsight0165AdoptionResult, error) {
	requestID, err := validateAIInsight0165RequestID(request.RequestID)
	if err != nil {
		return AIInsight0165AdoptionResult{}, err
	}
	backupSHA := strings.ToLower(strings.TrimSpace(request.ExternalBackupSHA256))
	if !aiInsight0165SHA256Pattern.MatchString(backupSHA) {
		return AIInsight0165AdoptionResult{}, errors.New("0165 historical adoption requires a valid external backup SHA-256")
	}
	if !request.TrafficStopped {
		return AIInsight0165AdoptionResult{}, ErrAIInsight0165TrafficNotStopped
	}
	conn, release, err := c.lockedConnection(ctx)
	if err != nil {
		return AIInsight0165AdoptionResult{}, err
	}
	defer release()

	result := AIInsight0165AdoptionResult{
		RequestID: requestID, MigrationChecksum: c.checksum,
		ExternalBackupSHA256: backupSHA, RecoveryBoundary: aiInsight0165RecoveryBoundaryText,
	}
	if err := conn.QueryRowContext(ctx, `SELECT DATABASE()`).Scan(&result.SchemaName); err != nil || strings.TrimSpace(result.SchemaName) == "" {
		return result, fmt.Errorf("%w: database name is unavailable", ErrAIInsight0165WrongSchema)
	}
	var appliedChecksum string
	if err := conn.QueryRowContext(ctx, `SELECT checksum FROM `+VersionTable+` WHERE version = ?`, AIInsight0165Version).Scan(&appliedChecksum); err != nil {
		return result, fmt.Errorf("%w: historical 0165 ledger is missing", ErrAIInsight0165WrongSchema)
	}
	if !checksumMatches(appliedChecksum, c.checksum, c.migration.ChecksumAliases) {
		return result, fmt.Errorf("%w: applied checksum %s is not the current 0165 checksum %s or a registered line-ending alias", ErrAIInsight0165WrongSchema, appliedChecksum, c.checksum)
	}
	result.AppliedMigrationChecksum = appliedChecksum
	if err := validateAIInsight0165PostMigrationSchema(ctx, conn); err != nil {
		return result, err
	}
	result.InsightRows, result.InsightDigest, err = aiInsight0165TableDigest(ctx, conn, aiInsight0165SourceTable)
	if err != nil {
		return result, err
	}

	controlExists, err := aiInsight0165TableExists(ctx, conn, aiInsight0165ControlTable)
	if err != nil {
		return result, err
	}
	if !controlExists {
		if err := createAIInsight0165ControlTable(ctx, conn); err != nil {
			return result, err
		}
	}
	if err := ensureAIInsight0165AdoptionColumns(ctx, conn); err != nil {
		return result, err
	}
	var verifiedCount int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+aiInsight0165ControlTable+` WHERE status = 'verified'`).Scan(&verifiedCount); err != nil {
		return result, err
	}
	if verifiedCount != 0 {
		return result, ErrAIInsight0165AdoptionConflict
	}
	var adoptedCount int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+aiInsight0165ControlTable+` WHERE status = ?`, aiInsight0165AdoptedStatus).Scan(&adoptedCount); err != nil {
		return result, err
	}
	if adoptedCount > 1 {
		return result, ErrAIInsight0165AdoptionConflict
	}
	if adoptedCount == 1 {
		var existing AIInsight0165AdoptionResult
		err := conn.QueryRowContext(ctx, `
			SELECT request_id, schema_name, migration_checksum, applied_migration_checksum, external_backup_sha256,
				insight_rows, insight_digest, recovery_boundary
			FROM `+aiInsight0165ControlTable+` WHERE status = ?
		`, aiInsight0165AdoptedStatus).Scan(
			&existing.RequestID, &existing.SchemaName, &existing.MigrationChecksum, &existing.AppliedMigrationChecksum, &existing.ExternalBackupSHA256,
			&existing.InsightRows, &existing.InsightDigest, &existing.RecoveryBoundary,
		)
		if err != nil {
			return result, err
		}
		if existing.RequestID != result.RequestID || existing.SchemaName != result.SchemaName ||
			!checksumMatches(existing.MigrationChecksum, result.MigrationChecksum, c.migration.ChecksumAliases) ||
			existing.AppliedMigrationChecksum != result.AppliedMigrationChecksum ||
			existing.ExternalBackupSHA256 != result.ExternalBackupSHA256 ||
			existing.InsightRows != result.InsightRows || existing.InsightDigest != result.InsightDigest ||
			existing.RecoveryBoundary != result.RecoveryBoundary {
			return result, ErrAIInsight0165AdoptionConflict
		}
		existing.Adopted = true
		return existing, nil
	}
	emptyDigest := emptyAIInsight0165Digest()
	_, err = conn.ExecContext(ctx, `
		INSERT INTO `+aiInsight0165ControlTable+` (
			request_id, schema_name, migration_checksum,
			insight_rows, insight_digest, duplicate_rows, legacy_rows, legacy_digest,
			backup_insight_rows, backup_insight_digest, backup_legacy_rows, backup_legacy_digest,
			status, recovery_boundary, applied_migration_checksum, external_backup_sha256, adopted_at,
			created_at, updated_at, verified_at
		) VALUES (?, ?, ?, ?, ?, 0, 0, ?, 0, ?, 0, ?, ?, ?, ?, ?, NOW(), NOW(), NOW(), NULL)
	`, requestID, result.SchemaName, c.checksum, result.InsightRows, result.InsightDigest,
		emptyDigest, emptyDigest, emptyDigest, aiInsight0165AdoptedStatus, result.RecoveryBoundary, appliedChecksum, backupSHA)
	if err != nil {
		return result, fmt.Errorf("record 0165 historical adoption: %w", err)
	}
	result.Adopted = true
	return result, nil
}

func (c *AIInsight0165Controller) inventoryWith(ctx context.Context, queryer aiInsight0165Queryer) (AIInsight0165Inventory, error) {
	report := AIInsight0165Inventory{MigrationChecksum: c.checksum, LegacyDigest: emptyAIInsight0165Digest()}
	if err := queryer.QueryRowContext(ctx, `SELECT DATABASE()`).Scan(&report.SchemaName); err != nil || strings.TrimSpace(report.SchemaName) == "" {
		return report, fmt.Errorf("%w: database name is unavailable", ErrAIInsight0165WrongSchema)
	}
	ledgerExists, err := aiInsight0165TableExists(ctx, queryer, VersionTable)
	if err != nil {
		return report, err
	}
	if !ledgerExists {
		return report, fmt.Errorf("%w: %s is missing", ErrAIInsight0165WrongSchema, VersionTable)
	}
	var appliedChecksum string
	err = queryer.QueryRowContext(ctx, `SELECT checksum FROM `+VersionTable+` WHERE version = ?`, AIInsight0165Version).Scan(&appliedChecksum)
	switch {
	case err == nil:
		report.Applied = true
		report.RecoveryBoundary = aiInsight0165RecoveryBoundaryText
		if !checksumMatches(appliedChecksum, c.checksum, c.migration.ChecksumAliases) {
			return report, fmt.Errorf("%w: applied checksum %s is not the current 0165 checksum %s or a registered line-ending alias", ErrAIInsight0165WrongSchema, appliedChecksum, c.checksum)
		}
		return report, ErrAIInsight0165AlreadyApplied
	case !errors.Is(err, sql.ErrNoRows):
		return report, err
	}
	if err := validateAIInsight0165PreMigrationSchema(ctx, queryer); err != nil {
		return report, err
	}
	report.InsightRows, report.InsightDigest, err = aiInsight0165TableDigest(ctx, queryer, aiInsight0165SourceTable)
	if err != nil {
		return report, err
	}
	if err := queryer.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(group_count - 1), 0)
		FROM (
			SELECT COUNT(*) AS group_count
			FROM `+aiInsight0165SourceTable+`
			GROUP BY tenant_id, corp_id, analysis_type, rule_version_id, conversation_key,
				COALESCE(DATE(CONVERT_TZ(COALESCE(generated_at, source_ended_at, created_at), '+00:00', '+08:00')),
					DATE(COALESCE(generated_at, source_ended_at, created_at)), CURDATE())
			HAVING COUNT(*) > 1
		) duplicates
	`).Scan(&report.DuplicateRows); err != nil {
		return report, fmt.Errorf("count 0165 duplicate rows: %w", err)
	}
	legacyExists, err := aiInsight0165TableExists(ctx, queryer, aiInsight0165LegacyTable)
	if err != nil {
		return report, err
	}
	if legacyExists {
		report.LegacyRows, report.LegacyDigest, err = aiInsight0165TableDigest(ctx, queryer, aiInsight0165LegacyTable)
		if err != nil {
			return report, err
		}
	}
	return report, nil
}

func (c *AIInsight0165Controller) preflightWith(ctx context.Context, queryer aiInsight0165Queryer, requestID string) (AIInsight0165Preflight, error) {
	inventory, err := c.inventoryWith(ctx, queryer)
	if err != nil {
		return AIInsight0165Preflight{AIInsight0165Inventory: inventory, RequestID: requestID}, err
	}
	manifest, err := loadAIInsight0165Manifest(ctx, queryer, requestID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, ErrAIInsight0165BackupMissing) {
			return AIInsight0165Preflight{AIInsight0165Inventory: inventory, RequestID: requestID}, ErrAIInsight0165BackupMissing
		}
		return AIInsight0165Preflight{AIInsight0165Inventory: inventory, RequestID: requestID}, err
	}
	if manifest.SchemaName != inventory.SchemaName || manifest.MigrationChecksum != inventory.MigrationChecksum ||
		manifest.InsightRows != inventory.InsightRows || manifest.InsightDigest != inventory.InsightDigest ||
		manifest.DuplicateRows != inventory.DuplicateRows || manifest.LegacyRows != inventory.LegacyRows || manifest.LegacyDigest != inventory.LegacyDigest {
		return AIInsight0165Preflight{AIInsight0165Inventory: inventory, RequestID: requestID}, ErrAIInsight0165SnapshotDrift
	}
	backupSourceExists, err := aiInsight0165TableExists(ctx, queryer, aiInsight0165BackupSourceTable)
	if err != nil {
		return AIInsight0165Preflight{}, err
	}
	backupLegacyExists, err := aiInsight0165TableExists(ctx, queryer, aiInsight0165BackupLegacyTable)
	if err != nil {
		return AIInsight0165Preflight{}, err
	}
	if !backupSourceExists || (manifest.LegacyRows > 0 && !backupLegacyExists) {
		return AIInsight0165Preflight{AIInsight0165Inventory: inventory, RequestID: requestID}, ErrAIInsight0165BackupMissing
	}
	backupSourceRows, backupSourceDigest, err := aiInsight0165TableDigest(ctx, queryer, aiInsight0165BackupSourceTable)
	if err != nil {
		return AIInsight0165Preflight{}, err
	}
	backupLegacyRows := int64(0)
	backupLegacyDigest := emptyAIInsight0165Digest()
	if backupLegacyExists {
		backupLegacyRows, backupLegacyDigest, err = aiInsight0165TableDigest(ctx, queryer, aiInsight0165BackupLegacyTable)
		if err != nil {
			return AIInsight0165Preflight{}, err
		}
	}
	if backupSourceRows != manifest.BackupInsightRows || backupSourceDigest != manifest.BackupInsightDigest ||
		backupLegacyRows != manifest.BackupLegacyRows || backupLegacyDigest != manifest.BackupLegacyDigest ||
		backupSourceRows != inventory.InsightRows || backupSourceDigest != inventory.InsightDigest ||
		backupLegacyRows != inventory.LegacyRows || backupLegacyDigest != inventory.LegacyDigest {
		return AIInsight0165Preflight{AIInsight0165Inventory: inventory, RequestID: requestID}, ErrAIInsight0165BackupDrift
	}
	inventory.BackupPresent = true
	inventory.BackupInsightRows = backupSourceRows
	inventory.BackupInsightDigest = backupSourceDigest
	inventory.BackupLegacyRows = backupLegacyRows
	inventory.BackupLegacyDigest = backupLegacyDigest
	approval := fmt.Sprintf("duplicates=%d,legacy=%d", inventory.DuplicateRows, inventory.LegacyRows)
	return AIInsight0165Preflight{
		AIInsight0165Inventory: inventory,
		RequestID:              requestID,
		ApprovalToken:          aiInsight0165ApprovalToken(requestID, manifest),
		DestructiveApproval:    approval,
	}, nil
}

func (c *AIInsight0165Controller) verifyAndRecordWith(ctx context.Context, conn *sql.Conn, requestID string, executionMS int) (AIInsight0165ApplyResult, error) {
	manifest, err := loadAIInsight0165Manifest(ctx, conn, requestID)
	if err != nil {
		return AIInsight0165ApplyResult{}, err
	}
	result := AIInsight0165ApplyResult{
		RequestID:            requestID,
		RetainedInsightRows:  manifest.InsightRows - manifest.DuplicateRows,
		RemovedDuplicateRows: manifest.DuplicateRows,
		RemovedLegacyRows:    manifest.LegacyRows,
		BackupInsightRows:    manifest.BackupInsightRows,
		BackupLegacyRows:     manifest.BackupLegacyRows,
		MigrationChecksum:    manifest.MigrationChecksum,
	}
	if err := validateAIInsight0165PostMigrationSchema(ctx, conn); err != nil {
		return result, err
	}
	var retained int64
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+aiInsight0165SourceTable).Scan(&retained); err != nil {
		return result, err
	}
	if retained != result.RetainedInsightRows {
		return result, fmt.Errorf("0165 retained row count = %d, want %d", retained, result.RetainedInsightRows)
	}
	expectedSurvivorRows, expectedSurvivorDigest, actualSurvivorRows, actualSurvivorDigest, err := aiInsight0165SurvivorDigests(ctx, conn)
	if err != nil {
		return result, err
	}
	if expectedSurvivorRows != result.RetainedInsightRows || actualSurvivorRows != expectedSurvivorRows || actualSurvivorDigest != expectedSurvivorDigest {
		return result, fmt.Errorf("%w: expected rows=%d digest=%s, actual rows=%d digest=%s", ErrAIInsight0165SurvivorDrift, expectedSurvivorRows, expectedSurvivorDigest, actualSurvivorRows, actualSurvivorDigest)
	}
	backupSourceRows, backupSourceDigest, err := aiInsight0165TableDigest(ctx, conn, aiInsight0165BackupSourceTable)
	if err != nil {
		return result, err
	}
	backupLegacyRows := int64(0)
	backupLegacyDigest := emptyAIInsight0165Digest()
	backupLegacyExists, err := aiInsight0165TableExists(ctx, conn, aiInsight0165BackupLegacyTable)
	if err != nil {
		return result, err
	}
	if backupLegacyExists {
		backupLegacyRows, backupLegacyDigest, err = aiInsight0165TableDigest(ctx, conn, aiInsight0165BackupLegacyTable)
		if err != nil {
			return result, err
		}
	}
	if backupSourceRows != manifest.BackupInsightRows || backupSourceDigest != manifest.BackupInsightDigest || backupLegacyRows != manifest.BackupLegacyRows || backupLegacyDigest != manifest.BackupLegacyDigest {
		return result, ErrAIInsight0165BackupDrift
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return result, fmt.Errorf("begin controlled 0165 completion transaction: %w", err)
	}
	defer tx.Rollback()
	var appliedChecksum string
	err = tx.QueryRowContext(ctx, `SELECT checksum FROM `+VersionTable+` WHERE version = ?`, AIInsight0165Version).Scan(&appliedChecksum)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		if err := recordAppliedWith(ctx, tx, c.migration, c.checksum, executionMS); err != nil {
			return result, fmt.Errorf("record controlled 0165 migration: %w", err)
		}
	case err != nil:
		return result, err
	case appliedChecksum != c.checksum:
		return result, fmt.Errorf("%w: ledger checksum %s differs from %s", ErrAIInsight0165WrongSchema, appliedChecksum, c.checksum)
	}
	var controlStatus string
	if err := tx.QueryRowContext(ctx, `SELECT status FROM `+aiInsight0165ControlTable+`
		WHERE request_id = ? AND migration_checksum = ? FOR UPDATE`, requestID, c.checksum).Scan(&controlStatus); err != nil {
		return result, fmt.Errorf("lock controlled 0165 completion: %w", err)
	}
	switch controlStatus {
	case "applying", "applied_unverified", "failed", "verified":
	default:
		return result, fmt.Errorf("controlled 0165 completion cannot advance from status %q", controlStatus)
	}
	updated, err := tx.ExecContext(ctx, `UPDATE `+aiInsight0165ControlTable+`
		SET status = 'verified', verified_at = NOW(), updated_at = NOW()
		WHERE request_id = ? AND migration_checksum = ? AND status = ?`, requestID, c.checksum, controlStatus)
	if err != nil {
		return result, fmt.Errorf("mark controlled 0165 verified: %w", err)
	}
	affected, err := updated.RowsAffected()
	if err != nil {
		return result, fmt.Errorf("inspect controlled 0165 verified update: %w", err)
	}
	if affected != 1 && !(controlStatus == "verified" && affected == 0) {
		return result, fmt.Errorf("mark controlled 0165 verified affected %d rows", affected)
	}
	if err := tx.Commit(); err != nil {
		return result, fmt.Errorf("commit controlled 0165 completion: %w", err)
	}
	result.Applied = true
	result.Verified = true
	return result, nil
}

func (c *AIInsight0165Controller) lockedConnection(ctx context.Context) (*sql.Conn, func(), error) {
	conn, err := c.db.Conn(ctx)
	if err != nil {
		return nil, nil, err
	}
	var schema sql.NullString
	if err := conn.QueryRowContext(ctx, `SELECT DATABASE()`).Scan(&schema); err != nil {
		_ = conn.Close()
		return nil, nil, fmt.Errorf("resolve controlled 0165 lock schema: %w", err)
	}
	lockName, err := aiInsight0165LockName(schema.String, c.checksum)
	if err != nil {
		_ = conn.Close()
		return nil, nil, err
	}
	var acquired int
	if err := conn.QueryRowContext(ctx, `SELECT GET_LOCK(?, 0)`, lockName).Scan(&acquired); err != nil {
		_ = conn.Close()
		return nil, nil, err
	}
	if acquired != 1 {
		_ = conn.Close()
		return nil, nil, ErrAIInsight0165ConcurrentRun
	}
	release := func() {
		var ignored sql.NullInt64
		_ = conn.QueryRowContext(context.Background(), `SELECT RELEASE_LOCK(?)`, lockName).Scan(&ignored)
		_ = conn.Close()
	}
	return conn, release, nil
}

func aiInsight0165LockName(schema, checksum string) (string, error) {
	schema = strings.TrimSpace(schema)
	if schema == "" {
		return "", errors.New("0165 controlled migration requires a selected database")
	}
	checksum = strings.TrimSpace(checksum)
	if checksum == "" {
		return "", errors.New("0165 controlled migration checksum is required")
	}
	digest := sha256.Sum256([]byte(schema + "\x00" + checksum))
	return "mochat-go:0165:" + hex.EncodeToString(digest[:24]), nil
}

func validateAIInsight0165RequestID(value string) (string, error) {
	if !aiInsight0165RequestPattern.MatchString(value) {
		return "", errors.New("0165 controlled migration request id must be 1-128 ASCII letters, digits, dot, underscore, colon, or hyphen")
	}
	return value, nil
}

func validateAIInsight0165PreMigrationSchema(ctx context.Context, queryer aiInsight0165Queryer) error {
	required := []string{"id", "tenant_id", "corp_id", "analysis_type", "rule_version_id", "conversation_key", "source_fingerprint", "source_ended_at", "generated_at", "created_at"}
	present, err := aiInsight0165ColumnSet(ctx, queryer, aiInsight0165SourceTable)
	if err != nil {
		return err
	}
	for _, column := range required {
		if !present[column] {
			return fmt.Errorf("%w: %s.%s is missing", ErrAIInsight0165WrongSchema, aiInsight0165SourceTable, column)
		}
	}
	for _, postColumn := range []string{"analysis_date", "previous_insight_id", "previous_score", "previous_summary", "previous_generated_at"} {
		if present[postColumn] {
			return fmt.Errorf("%w: %s.%s already exists without a verified 0165 ledger", ErrAIInsight0165WrongSchema, aiInsight0165SourceTable, postColumn)
		}
	}
	var sourceIndex, dailyIndex int
	if err := queryer.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = ? AND index_name = 'uq_ai_conversation_source'`, aiInsight0165SourceTable).Scan(&sourceIndex); err != nil {
		return err
	}
	if err := queryer.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = ? AND index_name = 'uq_ai_conversation_daily'`, aiInsight0165SourceTable).Scan(&dailyIndex); err != nil {
		return err
	}
	if sourceIndex == 0 || dailyIndex != 0 {
		return fmt.Errorf("%w: pre-migration unique index contract is invalid", ErrAIInsight0165WrongSchema)
	}
	return nil
}

func validateAIInsight0165PostMigrationSchema(ctx context.Context, queryer aiInsight0165Queryer) error {
	present, err := aiInsight0165ColumnSet(ctx, queryer, aiInsight0165SourceTable)
	if err != nil {
		return err
	}
	for _, column := range []string{"analysis_date", "previous_insight_id", "previous_score", "previous_summary", "previous_generated_at"} {
		if !present[column] {
			return fmt.Errorf("%w: post-migration column %s is missing", ErrAIInsight0165WrongSchema, column)
		}
	}
	legacyExists, err := aiInsight0165TableExists(ctx, queryer, aiInsight0165LegacyTable)
	if err != nil {
		return err
	}
	if legacyExists {
		return fmt.Errorf("%w: legacy table still exists", ErrAIInsight0165WrongSchema)
	}
	var dailyIndex, sourceIndex int
	if err := queryer.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = ? AND index_name = 'uq_ai_conversation_daily'`, aiInsight0165SourceTable).Scan(&dailyIndex); err != nil {
		return err
	}
	if err := queryer.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = ? AND index_name = 'uq_ai_conversation_source'`, aiInsight0165SourceTable).Scan(&sourceIndex); err != nil {
		return err
	}
	if dailyIndex == 0 || sourceIndex != 0 {
		return fmt.Errorf("%w: post-migration unique index contract is invalid", ErrAIInsight0165WrongSchema)
	}
	return nil
}

func aiInsight0165ColumnSet(ctx context.Context, queryer aiInsight0165Queryer, table string) (map[string]bool, error) {
	rows, err := queryer.QueryContext(ctx, `SELECT column_name FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = ?`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[string]bool{}
	for rows.Next() {
		var column string
		if err := rows.Scan(&column); err != nil {
			return nil, err
		}
		result[column] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("%w: table %s is missing", ErrAIInsight0165WrongSchema, table)
	}
	return result, nil
}

func createAIInsight0165ControlTable(ctx context.Context, queryer aiInsight0165Queryer) error {
	_, err := queryer.ExecContext(ctx, `CREATE TABLE `+aiInsight0165ControlTable+` (
		request_id varchar(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
		schema_name varchar(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
		migration_checksum char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
		insight_rows bigint unsigned NOT NULL,
		insight_digest char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
		duplicate_rows bigint unsigned NOT NULL,
		legacy_rows bigint unsigned NOT NULL,
		legacy_digest char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
		backup_insight_rows bigint unsigned NOT NULL,
		backup_insight_digest char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
		backup_legacy_rows bigint unsigned NOT NULL,
		backup_legacy_digest char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
		status varchar(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
		created_at datetime NOT NULL,
		updated_at datetime NOT NULL,
		verified_at datetime NULL,
		recovery_boundary text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NULL,
		external_backup_sha256 char(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
		applied_migration_checksum char(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
		adopted_at datetime NULL,
		PRIMARY KEY (request_id)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`)
	if err != nil {
		return fmt.Errorf("create 0165 control manifest: %w", err)
	}
	return nil
}

func ensureAIInsight0165AdoptionColumns(ctx context.Context, queryer aiInsight0165Queryer) error {
	columns := []struct {
		name string
		ddl  string
	}{
		{"recovery_boundary", "text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NULL"},
		{"external_backup_sha256", "char(64) CHARACTER SET ascii COLLATE ascii_bin NULL"},
		{"applied_migration_checksum", "char(64) CHARACTER SET ascii COLLATE ascii_bin NULL"},
		{"adopted_at", "datetime NULL"},
	}
	for _, column := range columns {
		var count int
		if err := queryer.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?`, aiInsight0165ControlTable, column.name).Scan(&count); err != nil {
			return err
		}
		if count > 1 {
			return fmt.Errorf("0165 control column %s has invalid metadata count %d", column.name, count)
		}
		if count == 0 {
			if _, err := queryer.ExecContext(ctx, `ALTER TABLE `+aiInsight0165ControlTable+` ADD COLUMN `+column.name+` `+column.ddl); err != nil {
				return fmt.Errorf("add 0165 historical adoption column %s: %w", column.name, err)
			}
		}
	}
	return nil
}

func loadAIInsight0165Manifest(ctx context.Context, queryer aiInsight0165Queryer, requestID string) (aiInsight0165Manifest, error) {
	exists, err := aiInsight0165TableExists(ctx, queryer, aiInsight0165ControlTable)
	if err != nil {
		return aiInsight0165Manifest{}, err
	}
	if !exists {
		return aiInsight0165Manifest{}, ErrAIInsight0165BackupMissing
	}
	var result aiInsight0165Manifest
	err = queryer.QueryRowContext(ctx, `
		SELECT request_id, schema_name, migration_checksum,
			insight_rows, insight_digest, duplicate_rows, legacy_rows, legacy_digest,
			backup_insight_rows, backup_insight_digest, backup_legacy_rows, backup_legacy_digest, status
		FROM `+aiInsight0165ControlTable+` WHERE request_id = ?
	`, requestID).Scan(
		&result.RequestID, &result.SchemaName, &result.MigrationChecksum,
		&result.InsightRows, &result.InsightDigest, &result.DuplicateRows, &result.LegacyRows, &result.LegacyDigest,
		&result.BackupInsightRows, &result.BackupInsightDigest, &result.BackupLegacyRows, &result.BackupLegacyDigest, &result.Status,
	)
	return result, err
}

func aiInsight0165ApprovalToken(requestID string, manifest aiInsight0165Manifest) string {
	canonical := strings.Join([]string{
		"mochat-go-0165-approval-v1", requestID, manifest.SchemaName, manifest.MigrationChecksum,
		fmt.Sprint(manifest.InsightRows), manifest.InsightDigest, fmt.Sprint(manifest.DuplicateRows),
		fmt.Sprint(manifest.LegacyRows), manifest.LegacyDigest,
		fmt.Sprint(manifest.BackupInsightRows), manifest.BackupInsightDigest,
		fmt.Sprint(manifest.BackupLegacyRows), manifest.BackupLegacyDigest,
	}, "\x00")
	digest := sha256.Sum256([]byte(canonical))
	return "approval-v1:" + hex.EncodeToString(digest[:])
}

func aiInsight0165TableExists(ctx context.Context, queryer aiInsight0165Queryer, table string) (bool, error) {
	var count int
	if err := queryer.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?`, table).Scan(&count); err != nil {
		return false, err
	}
	if count > 1 {
		return false, fmt.Errorf("%w: duplicate table metadata for %s", ErrAIInsight0165WrongSchema, table)
	}
	return count == 1, nil
}

func aiInsight0165TableDigest(ctx context.Context, queryer aiInsight0165Queryer, table string) (int64, string, error) {
	rows, err := queryer.QueryContext(ctx, `SELECT * FROM `+table+` ORDER BY id`)
	if err != nil {
		return 0, "", fmt.Errorf("digest table %s: %w", table, err)
	}
	return aiInsight0165RowsDigest(rows)
}

func aiInsight0165SurvivorDigests(ctx context.Context, queryer aiInsight0165Queryer) (int64, string, int64, string, error) {
	rows, err := queryer.QueryContext(ctx, `SELECT column_name
		FROM information_schema.columns
		WHERE table_schema = DATABASE() AND table_name = ?
		ORDER BY ordinal_position`, aiInsight0165BackupSourceTable)
	if err != nil {
		return 0, "", 0, "", err
	}
	columns := make([]string, 0)
	for rows.Next() {
		var column string
		if err := rows.Scan(&column); err != nil {
			rows.Close()
			return 0, "", 0, "", err
		}
		// The immutable migration's analysis_date backfill legitimately advances
		// ON UPDATE updated_at. Every other pre-0165 column must remain byte-stable.
		if column != "updated_at" {
			columns = append(columns, column)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, "", 0, "", err
	}
	if err := rows.Close(); err != nil {
		return 0, "", 0, "", err
	}
	if len(columns) == 0 {
		return 0, "", 0, "", fmt.Errorf("%w: backup source columns are missing", ErrAIInsight0165BackupMissing)
	}
	backupColumns := make([]string, 0, len(columns))
	sourceColumns := make([]string, 0, len(columns))
	for _, column := range columns {
		quoted := "`" + strings.ReplaceAll(column, "`", "``") + "`"
		backupColumns = append(backupColumns, "backup."+quoted)
		sourceColumns = append(sourceColumns, quoted)
	}
	expectedQuery := `SELECT ` + strings.Join(backupColumns, ",") + `
		FROM ` + aiInsight0165BackupSourceTable + ` AS backup
		INNER JOIN (
			SELECT MAX(id) AS survivor_id
			FROM ` + aiInsight0165BackupSourceTable + `
			GROUP BY tenant_id, corp_id, analysis_type, rule_version_id, conversation_key,
				COALESCE(DATE(CONVERT_TZ(COALESCE(generated_at, source_ended_at, created_at), '+00:00', '+08:00')),
					DATE(COALESCE(generated_at, source_ended_at, created_at)), CURDATE())
		) AS expected ON expected.survivor_id = backup.id
		ORDER BY backup.id`
	expectedRows, err := queryer.QueryContext(ctx, expectedQuery)
	if err != nil {
		return 0, "", 0, "", fmt.Errorf("digest expected 0165 survivors: %w", err)
	}
	expectedCount, expectedDigest, err := aiInsight0165RowsDigest(expectedRows)
	if err != nil {
		return 0, "", 0, "", err
	}
	actualRows, err := queryer.QueryContext(ctx, `SELECT `+strings.Join(sourceColumns, ",")+` FROM `+aiInsight0165SourceTable+` ORDER BY id`)
	if err != nil {
		return 0, "", 0, "", fmt.Errorf("digest actual 0165 survivors: %w", err)
	}
	actualCount, actualDigest, err := aiInsight0165RowsDigest(actualRows)
	return expectedCount, expectedDigest, actualCount, actualDigest, err
}

func aiInsight0165RowsDigest(rows *sql.Rows) (int64, string, error) {
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return 0, "", err
	}
	hash := sha256.New()
	for _, column := range columns {
		fmt.Fprintf(hash, "C%d:%s", len(column), column)
	}
	values := make([]sql.RawBytes, len(columns))
	destinations := make([]any, len(columns))
	for i := range values {
		destinations[i] = &values[i]
	}
	var count int64
	for rows.Next() {
		if err := rows.Scan(destinations...); err != nil {
			return 0, "", err
		}
		count++
		fmt.Fprintf(hash, "R%d", count)
		for _, value := range values {
			if value == nil {
				hash.Write([]byte("N"))
				continue
			}
			fmt.Fprintf(hash, "V%d:", len(value))
			hash.Write(value)
		}
	}
	if err := rows.Err(); err != nil {
		return 0, "", err
	}
	return count, hex.EncodeToString(hash.Sum(nil)), nil
}

func emptyAIInsight0165Digest() string {
	digest := sha256.Sum256(nil)
	return hex.EncodeToString(digest[:])
}

func sameAIInsight0165Source(left, right AIInsight0165Inventory) bool {
	return left.SchemaName == right.SchemaName && left.MigrationChecksum == right.MigrationChecksum &&
		left.InsightRows == right.InsightRows && left.InsightDigest == right.InsightDigest && left.DuplicateRows == right.DuplicateRows &&
		left.LegacyRows == right.LegacyRows && left.LegacyDigest == right.LegacyDigest
}

func installAIInsight0165WriteGuards(ctx context.Context, queryer aiInsight0165Queryer, requestID string, guardLegacy bool) error {
	if _, err := queryer.ExecContext(ctx, `SET @mochat_0165_request_id = ?`, requestID); err != nil {
		return err
	}
	tables := []struct {
		table  string
		prefix string
	}{{aiInsight0165SourceTable, "insight"}}
	if guardLegacy {
		tables = append(tables, struct {
			table  string
			prefix string
		}{aiInsight0165LegacyTable, "legacy"})
	}
	for _, item := range tables {
		for _, event := range []string{"INSERT", "UPDATE", "DELETE"} {
			trigger := "mochat_0165_guard_" + item.prefix + "_" + strings.ToLower(event)
			statement := fmt.Sprintf(`CREATE TRIGGER %s BEFORE %s ON %s FOR EACH ROW BEGIN IF COALESCE(@mochat_0165_request_id, '') <> '%s' THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = '0165 controlled migration write guard'; END IF; END`, trigger, event, item.table, requestID)
			if _, err := queryer.ExecContext(ctx, statement); err != nil {
				dropAIInsight0165WriteGuards(context.WithoutCancel(ctx), queryer)
				return fmt.Errorf("install 0165 write guard %s: %w", trigger, err)
			}
		}
	}
	return nil
}

func dropAIInsight0165WriteGuards(ctx context.Context, queryer aiInsight0165Queryer) {
	for _, trigger := range []string{
		"mochat_0165_guard_insight_insert", "mochat_0165_guard_insight_update", "mochat_0165_guard_insight_delete",
		"mochat_0165_guard_legacy_insert", "mochat_0165_guard_legacy_update", "mochat_0165_guard_legacy_delete",
	} {
		_, _ = queryer.ExecContext(ctx, `DROP TRIGGER IF EXISTS `+trigger)
	}
	_, _ = queryer.ExecContext(ctx, `SET @mochat_0165_request_id = NULL`)
}

func aiInsight0165BodyForServer(body, serverVersion string) string {
	version := strings.ToLower(strings.TrimSpace(serverVersion))
	if !strings.HasPrefix(version, "5.7.") || strings.Contains(version, "mariadb") {
		return body
	}
	replacements := []struct {
		from string
		to   string
	}{
		{"ADD COLUMN IF NOT EXISTS", "ADD COLUMN"},
		{"MODIFY COLUMN IF EXISTS", "MODIFY COLUMN"},
		{"DROP INDEX IF EXISTS", "DROP INDEX"},
		{"ADD UNIQUE KEY IF NOT EXISTS", "ADD UNIQUE KEY"},
		{"ADD KEY IF NOT EXISTS", "ADD KEY"},
	}
	compatible := body
	for _, replacement := range replacements {
		compatible = strings.ReplaceAll(compatible, replacement.from, replacement.to)
	}
	return compatible
}
