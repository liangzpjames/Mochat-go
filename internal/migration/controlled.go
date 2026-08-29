package migration

import (
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// MigrationKind is the execution policy for a migration. Controlled migrations
// are visible to the ordinary runner, but require their dedicated maintenance
// command to perform the data-changing operation.
type MigrationKind string

const (
	MigrationAutomatic  MigrationKind = "automatic"
	MigrationControlled MigrationKind = "controlled"
)

// ControlledMigration is the durable contract between the migration registry,
// the ordinary runner, and the dedicated maintenance CLI.
type ControlledMigration struct {
	Version         string
	LedgerTable     string
	LedgerName      string
	SuccessPhase    string
	CompletionTable string
	RequiredCLI     string
	SuccessStatus   string
}

type ControlledMigrationPendingError struct {
	Version string
}

func (e *ControlledMigrationPendingError) Error() string {
	return fmt.Sprintf("controlled migration %s is pending; run mochat-identity-migrate before automatic migrations can continue", e.Version)
}

var controlledMigrationRegistry = map[string]ControlledMigration{
	"0130_identity_realms_single_corp_backfill": {
		Version:         "0130_identity_realms_single_corp_backfill",
		LedgerTable:     "mochat_go_identity_migration_ledger",
		LedgerName:      "0130_identity_realms_single_corp_backfill",
		SuccessPhase:    "backfill",
		CompletionTable: "mochat_go_identity_migration_batches",
		RequiredCLI:     "mochat-identity-migrate",
		SuccessStatus:   "success",
	},
	"0131_identity_realms_single_corp_cutover": {
		Version:         "0131_identity_realms_single_corp_cutover",
		LedgerTable:     "mochat_go_identity_migration_ledger",
		LedgerName:      "0131_identity_realms_single_corp_cutover",
		SuccessPhase:    "cutover",
		CompletionTable: "mochat_go_identity_cutover_batches",
		RequiredCLI:     "mochat-identity-migrate",
		SuccessStatus:   "success",
	},
}

func MigrationMetadata(version string) (MigrationKind, *ControlledMigration) {
	metadata, ok := controlledMigrationRegistry[version]
	if !ok {
		return MigrationAutomatic, nil
	}
	metadataCopy := metadata
	return MigrationControlled, &metadataCopy
}

func IsControlledMigration(version string) bool {
	_, metadata := MigrationMetadata(version)
	return metadata != nil
}

func ControlledMigrationRegistry() []ControlledMigration {
	result := make([]ControlledMigration, 0, len(controlledMigrationRegistry))
	for _, metadata := range controlledMigrationRegistry {
		result = append(result, metadata)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Version < result[j].Version })
	return result
}

func ControlledMigrationBlocked(version string) error {
	return &ControlledMigrationPendingError{Version: version}
}

func ControlledMigrationRollbackRequired(version string) error {
	return fmt.Errorf("controlled migration %s must be rolled back with mochat-identity-migrate", version)
}

// RecordControlledMigration writes the normal migration-table fact only after
// the controlled migration's own success ledger is present. This is the
// hand-off that allows later automatic migrations to proceed.
func RecordControlledMigration(ctx context.Context, db *sql.DB, projectRoot, version, requestID string) error {
	if db == nil {
		return errors.New("migration db is nil")
	}
	kind, metadata := MigrationMetadata(version)
	if kind != MigrationControlled || metadata == nil {
		return fmt.Errorf("migration %s is not registered as controlled", version)
	}
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return errors.New("controlled migration request id is required")
	}
	migrations := DefaultMigrations(projectRoot)
	var target Migration
	for _, candidate := range migrations {
		if candidate.Version == version {
			target = candidate
			break
		}
	}
	if target.Version == "" {
		return fmt.Errorf("controlled migration %s is not present in DefaultMigrations", version)
	}
	_, checksum, err := migrationBodyAndChecksum(target)
	if err != nil {
		return err
	}
	runner := &Runner{db: db}
	if err := runner.ensureVersionTable(ctx); err != nil {
		return err
	}
	var ledgerTableCount int
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM information_schema.tables
		WHERE table_schema = DATABASE() AND table_name = ?
	`, metadata.LedgerTable).Scan(&ledgerTableCount); err != nil {
		return err
	}
	if ledgerTableCount != 1 {
		return fmt.Errorf("controlled migration %s success ledger is missing", version)
	}
	var successCount int
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM `+metadata.LedgerTable+`
		WHERE migration_name = ? AND request_id = ? AND phase = ? AND status = ?
	`, metadata.LedgerName, requestID, metadata.SuccessPhase, metadata.SuccessStatus).Scan(&successCount); err != nil {
		return err
	}
	if successCount != 1 {
		return fmt.Errorf("controlled migration %s success ledger request is not committed exactly once", version)
	}
	var resultJSON []byte
	if err := db.QueryRowContext(ctx, `
		SELECT result_json
		FROM `+metadata.LedgerTable+`
		WHERE migration_name = ? AND request_id = ? AND phase = ? AND status = ?
		ORDER BY id DESC LIMIT 1
	`, metadata.LedgerName, requestID, metadata.SuccessPhase, metadata.SuccessStatus).Scan(&resultJSON); err != nil {
		return err
	}
	var result struct {
		ScriptChecksum string `json:"scriptChecksum"`
	}
	if err := json.Unmarshal(resultJSON, &result); err != nil || !validScriptChecksum(result.ScriptChecksum) || result.ScriptChecksum != checksum {
		return fmt.Errorf("controlled migration %s success ledger checksum does not match current script", version)
	}
	var batchTableCount int
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM information_schema.tables
		WHERE table_schema = DATABASE() AND table_name = ?
	`, metadata.CompletionTable).Scan(&batchTableCount); err != nil {
		return err
	}
	if batchTableCount != 1 {
		return fmt.Errorf("controlled migration %s completed batch is missing", version)
	}
	var completedBatchCount int
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM `+metadata.CompletionTable+`
		WHERE request_id = ? AND status = 'completed'
	`, requestID).Scan(&completedBatchCount); err != nil {
		return err
	}
	if completedBatchCount != 1 {
		return fmt.Errorf("controlled migration %s completed batch is missing", version)
	}
	var existingChecksum string
	err = db.QueryRowContext(ctx, `SELECT checksum FROM `+VersionTable+` WHERE version = ?`, version).Scan(&existingChecksum)
	switch {
	case err == nil:
		if !checksumMatches(existingChecksum, checksum, target.ChecksumAliases) {
			return fmt.Errorf("migration %s checksum mismatch: applied=%s current=%s", version, existingChecksum, checksum)
		}
		return nil
	case !errors.Is(err, sql.ErrNoRows):
		return err
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO `+VersionTable+` (version, description, checksum, applied_at, execution_ms)
		VALUES (?, ?, ?, NOW(), 0)
	`, target.Version, target.Description, checksum); err != nil {
		return fmt.Errorf("record controlled migration %s: %w", version, err)
	}
	return nil
}

func controlledMigrationPath(projectRoot, version, suffix string) string {
	return filepath.Join(projectRoot, "deploy", "standalone", "migrations", version+suffix)
}

func validScriptChecksum(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
