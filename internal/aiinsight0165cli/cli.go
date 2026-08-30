package aiinsight0165cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"jiyi/mochat-go/internal/identitymigration"
	"jiyi/mochat-go/internal/migration"
	"jiyi/mochat-go/internal/mysqlconn"
)

type Options struct {
	Action                string
	DSNFile               string
	ProjectRoot           string
	RequestID             string
	ApprovalToken         string
	DestructiveApproval   string
	ExternalBackupSHA256  string
	ConfirmTrafficStopped bool
}

func ParseOptions(args []string) (Options, error) {
	if len(args) == 0 {
		return Options{}, errors.New("action must be one of inventory, backup, preflight, apply, verify, adopt-existing")
	}
	options := Options{Action: strings.TrimSpace(args[0]), ProjectRoot: "."}
	switch options.Action {
	case "inventory", "backup", "preflight", "apply", "verify", "adopt-existing":
	default:
		return Options{}, errors.New("action must be one of inventory, backup, preflight, apply, verify, adopt-existing")
	}
	flags := flag.NewFlagSet("preflight_0165_ai_daily_insight_unification "+options.Action, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&options.DSNFile, "dsn-file", "", "path to a restricted MySQL DSN file")
	flags.StringVar(&options.ProjectRoot, "project-root", ".", "MoChat Go project root containing the immutable 0165 SQL")
	flags.StringVar(&options.RequestID, "request-id", "", "stable controlled migration request id")
	flags.StringVar(&options.ApprovalToken, "approval-token", "", "approval token emitted by preflight")
	flags.StringVar(&options.DestructiveApproval, "approve-destructive", "", "exact duplicate and legacy counts emitted by preflight")
	flags.StringVar(&options.ExternalBackupSHA256, "external-backup-sha256", "", "SHA-256 of an externally restored and verified current backup")
	flags.BoolVar(&options.ConfirmTrafficStopped, "confirm-traffic-stopped", false, "confirm application traffic has been stopped before apply")
	if err := flags.Parse(args[1:]); err != nil {
		return Options{}, err
	}
	if flags.NArg() != 0 {
		return Options{}, errors.New("positional arguments are not accepted after the action")
	}
	if strings.TrimSpace(options.DSNFile) == "" {
		return Options{}, errors.New("--dsn-file is required")
	}
	if strings.TrimSpace(options.ProjectRoot) == "" {
		return Options{}, errors.New("--project-root is required")
	}
	if options.Action != "inventory" && strings.TrimSpace(options.RequestID) == "" {
		return Options{}, errors.New("--request-id is required")
	}
	if options.Action == "apply" {
		if strings.TrimSpace(options.ApprovalToken) == "" {
			return Options{}, errors.New("--approval-token is required")
		}
		if strings.TrimSpace(options.DestructiveApproval) == "" {
			return Options{}, errors.New("--approve-destructive is required")
		}
		if !options.ConfirmTrafficStopped {
			return Options{}, errors.New("--confirm-traffic-stopped is required")
		}
	}
	if options.Action == "adopt-existing" {
		if strings.TrimSpace(options.ExternalBackupSHA256) == "" {
			return Options{}, errors.New("--external-backup-sha256 is required")
		}
		if !options.ConfirmTrafficStopped {
			return Options{}, errors.New("--confirm-traffic-stopped is required")
		}
	}
	return options, nil
}

func Run(ctx context.Context, args []string, output io.Writer) error {
	options, err := ParseOptions(args)
	if err != nil {
		return err
	}
	dsn, err := identitymigration.ReadSecretFile(options.DSNFile)
	if err != nil {
		return errors.New("0165 controlled migration could not read the restricted DSN file")
	}
	db, err := mysqlconn.Open(strings.TrimSpace(string(dsn)))
	if err != nil {
		return errors.New("0165 controlled migration could not open the target database")
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		return errors.New("0165 controlled migration could not reach the target database")
	}
	controller, err := migration.NewAIInsight0165Controller(db, options.ProjectRoot)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	switch options.Action {
	case "inventory":
		report, inventoryErr := controller.Inventory(ctx)
		if encodeErr := encoder.Encode(report); encodeErr != nil {
			return encodeErr
		}
		return inventoryErr
	case "backup":
		report, backupErr := controller.Backup(ctx, options.RequestID)
		if backupErr != nil {
			return backupErr
		}
		return encoder.Encode(report)
	case "preflight":
		report, preflightErr := controller.Preflight(ctx, options.RequestID)
		if preflightErr != nil {
			return preflightErr
		}
		return encoder.Encode(report)
	case "apply":
		report, applyErr := controller.Apply(ctx, migration.AIInsight0165ApplyRequest{
			RequestID:           options.RequestID,
			ApprovalToken:       options.ApprovalToken,
			DestructiveApproval: options.DestructiveApproval,
			TrafficStopped:      options.ConfirmTrafficStopped,
		})
		if applyErr != nil {
			return applyErr
		}
		return encoder.Encode(report)
	case "verify":
		report, verifyErr := controller.Verify(ctx, options.RequestID)
		if verifyErr != nil {
			return verifyErr
		}
		return encoder.Encode(report)
	case "adopt-existing":
		report, adoptErr := controller.AdoptExisting(ctx, migration.AIInsight0165AdoptExistingRequest{
			RequestID: options.RequestID, ExternalBackupSHA256: options.ExternalBackupSHA256,
			TrafficStopped: options.ConfirmTrafficStopped,
		})
		if adoptErr != nil {
			return adoptErr
		}
		return encoder.Encode(report)
	default:
		return fmt.Errorf("unsupported action %q", options.Action)
	}
}
