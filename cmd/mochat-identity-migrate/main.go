package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"jiyi/mochat-go/internal/identitymigration"
	"jiyi/mochat-go/internal/migration"
	"jiyi/mochat-go/internal/mysqlconn"
)

type migrateOptions struct {
	Action                      string
	Execute                     bool
	RequestID                   string
	DSNFile                     string
	Schema                      string
	PlatformTenantID            int64
	MaintenanceConfirmationFile string
	MappingFile                 string
	MappingKeyFile              string
	CredentialKeyFile           string
	CredentialKeyID             string
	ProjectRoot                 string
	Timeout                     time.Duration
}

func parseMigrateOptions(args []string) (migrateOptions, error) {
	if len(args) == 0 {
		return migrateOptions{}, errors.New("migration action is required")
	}
	options := migrateOptions{Action: strings.TrimSpace(args[0])}
	flags := flag.NewFlagSet("mochat-identity-migrate", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.BoolVar(&options.Execute, "execute", false, "allow a maintenance-window write")
	flags.StringVar(&options.RequestID, "request-id", "", "durable migration request identifier")
	flags.StringVar(&options.DSNFile, "dsn-file", "", "path to a restricted DSN file")
	flags.StringVar(&options.Schema, "schema", "", "target database schema")
	flags.Int64Var(&options.PlatformTenantID, "platform-tenant-id", 0, "explicit historical platform tenant id")
	flags.StringVar(&options.MaintenanceConfirmationFile, "maintenance-confirmation-file", "", "maintenance-window confirmation file")
	flags.StringVar(&options.MappingFile, "mapping-file", "", "signed tenant to corp mapping file")
	flags.StringVar(&options.MappingKeyFile, "mapping-key-file", "", "mapping signature key file")
	flags.StringVar(&options.CredentialKeyFile, "credential-key-file", "", "dedicated WeCom credential encryption key file")
	flags.StringVar(&options.CredentialKeyID, "credential-key-id", "", "dedicated WeCom credential encryption key id")
	flags.StringVar(&options.ProjectRoot, "project-root", ".", "project root containing the controlled migration")
	flags.DurationVar(&options.Timeout, "timeout", 30*time.Minute, "maintenance operation timeout")
	if err := flags.Parse(args[1:]); err != nil {
		return migrateOptions{}, err
	}
	if flags.NArg() != 0 {
		return migrateOptions{}, errors.New("migration does not accept positional arguments after flags")
	}
	if err := validateMigrateOptions(options); err != nil {
		return migrateOptions{}, err
	}
	return options, nil
}

func validateMigrateOptions(options migrateOptions) error {
	switch options.Action {
	case "up", "down", "backfill", "encrypt-credentials":
	default:
		return errors.New("unknown migration action")
	}
	if strings.TrimSpace(options.DSNFile) == "" {
		return errors.New("--dsn-file is required")
	}
	if strings.TrimSpace(options.Schema) == "" {
		return errors.New("--schema is required")
	}
	if options.PlatformTenantID <= 0 {
		return errors.New("--platform-tenant-id must be positive")
	}
	if (strings.TrimSpace(options.MappingFile) == "") != (strings.TrimSpace(options.MappingKeyFile) == "") {
		return errors.New("--mapping-file and --mapping-key-file must be provided together")
	}
	if !options.Execute {
		return errors.New("--execute is required for maintenance writes")
	}
	if strings.TrimSpace(options.RequestID) == "" {
		return errors.New("--request-id is required for maintenance execute")
	}
	if strings.TrimSpace(options.MaintenanceConfirmationFile) == "" {
		return errors.New("--maintenance-confirmation-file is required for maintenance execute")
	}
	if strings.TrimSpace(options.ProjectRoot) == "" {
		return errors.New("--project-root is required")
	}
	if options.Timeout <= 0 {
		return errors.New("--timeout must be positive")
	}
	if options.Action != "down" && (strings.TrimSpace(options.CredentialKeyFile) == "" || strings.TrimSpace(options.CredentialKeyID) == "") {
		return errors.New("--credential-key-file and --credential-key-id are required for this action")
	}
	return nil
}

func main() {
	if err := runMigration(os.Args[1:], os.Stdout); err != nil {
		writeMigrationError(os.Stderr, err)
		os.Exit(1)
	}
}

func writeMigrationError(output io.Writer, err error) {
	_, _ = fmt.Fprintln(output, "identity migration failed")
	var phaseErr *identitymigration.PhaseError
	if errors.As(err, &phaseErr) {
		_, _ = fmt.Fprintln(output, phaseErr.Error())
	}
}

func preservePhaseError(err error, fallback string) error {
	var phaseErr *identitymigration.PhaseError
	if errors.As(err, &phaseErr) {
		return phaseErr
	}
	return errors.New(fallback)
}

func runMigration(args []string, output io.Writer) error {
	options, err := parseMigrateOptions(args)
	if err != nil {
		return err
	}
	confirmation, err := identitymigration.ReadSecretFile(options.MaintenanceConfirmationFile)
	if err != nil || identitymigration.VerifyMaintenanceConfirmation(confirmation, options.Schema, options.RequestID) != nil {
		return errors.New("maintenance confirmation is invalid")
	}
	dsn, err := identitymigration.ReadSecretFile(options.DSNFile)
	if err != nil {
		return errors.New("identity migration could not read the DSN file")
	}
	root, err := filepath.Abs(options.ProjectRoot)
	if err != nil {
		return errors.New("identity migration project root is invalid")
	}
	db, err := mysqlconn.Open(string(dsn))
	if err != nil {
		return errors.New("identity migration could not open the target database")
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), options.Timeout)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return errors.New("identity migration could not reach the target database")
	}
	if err := identitymigration.VerifyTargetSchema(ctx, db, options.Schema); err != nil {
		return errors.New("identity migration target schema is not the maintenance schema")
	}

	switch options.Action {
	case "up", "backfill":
		manager, err := identitymigration.NewCredentialManagerFromFile(options.CredentialKeyFile, options.CredentialKeyID)
		if err != nil {
			return errors.New("identity migration could not configure credential decryption")
		}
		mapping, err := identitymigration.ReadMappingDocument(options.MappingFile, options.MappingKeyFile)
		if err != nil {
			return errors.New("identity migration could not verify the mapping document")
		}
		upPath, err := identitymigration.ControlledMigrationPath(root, options.Action)
		if err != nil {
			return errors.New("identity migration path is invalid")
		}
		result, err := identitymigration.ApplyBackfill(ctx, db, identitymigration.DatabaseOptions{
			Schema:            options.Schema,
			PlatformTenantID:  options.PlatformTenantID,
			Mapping:           mapping,
			RequestID:         options.RequestID,
			CredentialManager: manager,
		}, upPath)
		if err != nil {
			return preservePhaseError(err, "identity backfill did not complete")
		}
		if err := migration.RecordControlledMigration(ctx, db, root, "0130_identity_realms_single_corp_backfill", options.RequestID); err != nil {
			return &identitymigration.PhaseError{Phase: "ledger", Label: "standard_record"}
		}
		_, err = fmt.Fprintf(output, "%s\t%s\n", options.Action, map[bool]string{true: "idempotent", false: "completed"}[result.Idempotent])
		return err
	case "encrypt-credentials":
		manager, err := identitymigration.NewCredentialManagerFromFile(options.CredentialKeyFile, options.CredentialKeyID)
		if err != nil {
			return errors.New("identity migration could not configure credential encryption")
		}
		result, err := identitymigration.EncryptCredentials(ctx, db, manager, options.RequestID)
		if err != nil {
			return &identitymigration.PhaseError{Phase: "encrypt", Label: "credentials"}
		}
		_, err = fmt.Fprintf(output, "%s\t%d\t%d\n", options.Action, result.CorpRowsWritten, result.AgentRowsWritten)
		return err
	case "down":
		downPath, err := identitymigration.ControlledMigrationPath(root, options.Action)
		if err != nil {
			return errors.New("identity migration path is invalid")
		}
		body, err := os.ReadFile(downPath)
		if err != nil {
			return errors.New("identity migration rollback file is unavailable")
		}
		conn, err := db.Conn(ctx)
		if err != nil {
			return errors.New("identity migration rollback connection is unavailable")
		}
		defer conn.Close()
		if _, err := conn.ExecContext(ctx, "SET @identity_0130_requested_down_request_id = ?", options.RequestID); err != nil {
			return &identitymigration.PhaseError{Phase: "rollback", Label: "request_bind"}
		}
		statements, err := migration.SplitSQLStatements(string(body))
		if err != nil {
			return &identitymigration.PhaseError{Phase: "rollback", Label: "script_parse"}
		}
		for _, statement := range statements {
			if _, err := conn.ExecContext(ctx, statement); err != nil {
				return &identitymigration.PhaseError{Phase: "rollback", Label: "statement"}
			}
		}
		if _, err := db.ExecContext(ctx, `DELETE FROM `+migration.VersionTable+` WHERE version = ?`, "0130_identity_realms_single_corp_backfill"); err != nil {
			return &identitymigration.PhaseError{Phase: "rollback", Label: "standard_record"}
		}
		_, err = fmt.Fprintln(output, "down\tcompleted")
		return err
	default:
		return errors.New("unknown migration action")
	}
}
