// Package testharness builds integration-test schemas exclusively from the
// production migration registry and controlled migration APIs.
package testharness

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"jiyi/mochat-go/internal/identitymigration"
	"jiyi/mochat-go/internal/migration"
	"jiyi/mochat-go/internal/wecomcredentials"
)

const freshInstallPlatformTenantID int64 = 4294967294

var requestPrefixPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,47}$`)

// ControlledEvidence carries the real maintenance-controller inputs needed
// to cross controlled registry entries. A zero value deliberately fails
// closed at the first controlled migration.
type ControlledEvidence struct {
	IdentityPlatformTenantID  int64
	IdentityRequestID         string
	IdentityCredentialManager *wecomcredentials.Manager
	AIInsightRequestID        string
}

// NewControlledEvidence creates non-secret, request-scoped evidence for a
// fresh integration database. The generated encryption key remains in memory.
func NewControlledEvidence(requestPrefix string) (ControlledEvidence, error) {
	requestPrefix = strings.TrimSpace(requestPrefix)
	if !requestPrefixPattern.MatchString(requestPrefix) {
		return ControlledEvidence{}, errors.New("controlled evidence request prefix is invalid")
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return ControlledEvidence{}, fmt.Errorf("generate controlled evidence encryption key: %w", err)
	}
	manager, err := wecomcredentials.NewManager(wecomcredentials.Config{
		EncryptionKey:       base64.StdEncoding.EncodeToString(key),
		EncryptionKeyID:     "integration",
		RequireEncryption:   true,
		DedicatedConfigured: true,
	})
	if err != nil {
		return ControlledEvidence{}, err
	}
	return ControlledEvidence{
		IdentityPlatformTenantID:  freshInstallPlatformTenantID,
		IdentityRequestID:         requestPrefix + "-identity",
		IdentityCredentialManager: manager,
		AIInsightRequestID:        requestPrefix + "-0165",
	}, nil
}

func ApplyLatest(ctx context.Context, db *sql.DB, projectRoot string, evidence ControlledEvidence) error {
	migrations := migration.DefaultMigrations(projectRoot)
	if len(migrations) == 0 {
		return errors.New("production migration registry is empty")
	}
	return applyPrefix(ctx, db, projectRoot, migrations, evidence)
}

func ApplyThrough(ctx context.Context, db *sql.DB, projectRoot, targetVersion string, evidence ControlledEvidence) error {
	all := migration.DefaultMigrations(projectRoot)
	for index, candidate := range all {
		if candidate.Version == targetVersion {
			return applyPrefix(ctx, db, projectRoot, all[:index+1], evidence)
		}
	}
	return fmt.Errorf("target migration %s is not present in production registry", targetVersion)
}

func applyPrefix(ctx context.Context, db *sql.DB, projectRoot string, migrations []migration.Migration, evidence ControlledEvidence) error {
	if db == nil {
		return errors.New("migration test harness database is nil")
	}
	for index, candidate := range migrations {
		if candidate.Kind != migration.MigrationControlled {
			continue
		}
		if err := applyAutomaticPrefix(ctx, db, migrations[:index]); err != nil {
			return err
		}
		switch candidate.Version {
		case "0130_identity_realms_single_corp_backfill":
			if err := applyIdentityBackfill(ctx, db, projectRoot, candidate, evidence); err != nil {
				return err
			}
		case "0131_identity_realms_single_corp_cutover":
			if err := applyIdentityCutover(ctx, db, projectRoot, candidate, evidence); err != nil {
				return err
			}
		case migration.AIInsight0165Version:
			if err := applyAIInsight0165(ctx, db, projectRoot, evidence); err != nil {
				return err
			}
		default:
			return migration.ControlledMigrationBlocked(candidate.Version)
		}
	}
	return applyAutomaticPrefix(ctx, db, migrations)
}

func applyAutomaticPrefix(ctx context.Context, db *sql.DB, migrations []migration.Migration) error {
	runner, err := migration.NewRunner(db, migrations)
	if err != nil {
		return err
	}
	_, err = runner.Apply(ctx)
	return err
}

func applyIdentityBackfill(ctx context.Context, db *sql.DB, projectRoot string, target migration.Migration, evidence ControlledEvidence) error {
	if evidence.IdentityPlatformTenantID <= 0 || strings.TrimSpace(evidence.IdentityRequestID) == "" || evidence.IdentityCredentialManager == nil {
		return migration.ControlledMigrationBlocked(target.Version)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO mc_tenant (id,name,status) VALUES (?,?,1)`, evidence.IdentityPlatformTenantID, "integration platform tenant"); err != nil {
		return fmt.Errorf("seed controlled identity platform tenant: %w", err)
	}
	schema, err := currentSchema(ctx, db)
	if err != nil {
		return err
	}
	_, err = identitymigration.ApplyBackfill(ctx, db, identitymigration.DatabaseOptions{
		Schema:            schema,
		PlatformTenantID:  evidence.IdentityPlatformTenantID,
		RequestID:         evidence.IdentityRequestID,
		CredentialManager: evidence.IdentityCredentialManager,
	}, target.Path)
	if err != nil {
		return err
	}
	return migration.RecordControlledMigration(ctx, db, projectRoot, target.Version, evidence.IdentityRequestID)
}

func applyIdentityCutover(ctx context.Context, db *sql.DB, projectRoot string, target migration.Migration, evidence ControlledEvidence) error {
	if evidence.IdentityPlatformTenantID <= 0 || strings.TrimSpace(evidence.IdentityRequestID) == "" || evidence.IdentityCredentialManager == nil {
		return migration.ControlledMigrationBlocked(target.Version)
	}
	schema, err := currentSchema(ctx, db)
	if err != nil {
		return err
	}
	_, err = identitymigration.ApplyCutover(ctx, db, identitymigration.DatabaseOptions{
		Schema:            schema,
		PlatformTenantID:  evidence.IdentityPlatformTenantID,
		RequestID:         evidence.IdentityRequestID,
		CredentialManager: evidence.IdentityCredentialManager,
	}, target.Path)
	if err != nil {
		return err
	}
	return migration.RecordControlledMigration(ctx, db, projectRoot, target.Version, evidence.IdentityRequestID)
}

func applyAIInsight0165(ctx context.Context, db *sql.DB, projectRoot string, evidence ControlledEvidence) error {
	requestID := strings.TrimSpace(evidence.AIInsightRequestID)
	if requestID == "" {
		return migration.ControlledMigrationBlocked(migration.AIInsight0165Version)
	}
	controller, err := migration.NewAIInsight0165Controller(db, filepath.Clean(projectRoot))
	if err != nil {
		return err
	}
	if _, err := controller.Backup(ctx, requestID); err != nil {
		return err
	}
	preflight, err := controller.Preflight(ctx, requestID)
	if err != nil {
		return err
	}
	_, err = controller.Apply(ctx, migration.AIInsight0165ApplyRequest{
		RequestID:           requestID,
		ApprovalToken:       preflight.ApprovalToken,
		DestructiveApproval: preflight.DestructiveApproval,
		TrafficStopped:      true,
	})
	return err
}

func currentSchema(ctx context.Context, db *sql.DB) (string, error) {
	var schema sql.NullString
	if err := db.QueryRowContext(ctx, `SELECT DATABASE()`).Scan(&schema); err != nil || !schema.Valid || strings.TrimSpace(schema.String) == "" {
		return "", errors.New("migration test harness requires a selected isolated database")
	}
	return schema.String, nil
}
