package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"jiyi/mochat-go/internal/identitymigration"
	"jiyi/mochat-go/internal/mysqlconn"
)

type preflightOptions struct {
	DSNFile           string
	Schema            string
	PlatformTenantID  int64
	MappingFile       string
	MappingKeyFile    string
	CredentialKeyFile string
	CredentialKeyID   string
	Execute           bool
}

func parsePreflightOptions(args []string) (preflightOptions, error) {
	var options preflightOptions
	flags := flag.NewFlagSet("mochat-identity-preflight", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&options.DSNFile, "dsn-file", "", "path to a restricted DSN file")
	flags.StringVar(&options.Schema, "schema", "", "database schema to inspect")
	flags.Int64Var(&options.PlatformTenantID, "platform-tenant-id", 0, "explicit historical platform tenant id")
	flags.StringVar(&options.MappingFile, "mapping-file", "", "signed tenant to corp mapping file")
	flags.StringVar(&options.MappingKeyFile, "mapping-key-file", "", "mapping signature key file")
	flags.StringVar(&options.CredentialKeyFile, "credential-key-file", "", "dedicated WeCom credential encryption key file")
	flags.StringVar(&options.CredentialKeyID, "credential-key-id", "", "dedicated WeCom credential encryption key id")
	if err := flags.Parse(args); err != nil {
		return preflightOptions{}, err
	}
	if flags.NArg() != 0 {
		return preflightOptions{}, errors.New("preflight does not accept positional arguments")
	}
	if err := validatePreflightOptions(options); err != nil {
		return preflightOptions{}, err
	}
	return options, nil
}

func validatePreflightOptions(options preflightOptions) error {
	if strings.TrimSpace(options.DSNFile) == "" {
		return errors.New("--dsn-file is required")
	}
	if strings.TrimSpace(options.Schema) == "" {
		return errors.New("--schema is required")
	}
	if options.PlatformTenantID <= 0 {
		return errors.New("--platform-tenant-id must be positive")
	}
	if strings.TrimSpace(options.CredentialKeyFile) == "" || strings.TrimSpace(options.CredentialKeyID) == "" {
		return errors.New("--credential-key-file and --credential-key-id are required")
	}
	if options.Execute {
		return errors.New("preflight is read-only and does not accept --execute")
	}
	if (strings.TrimSpace(options.MappingFile) == "") != (strings.TrimSpace(options.MappingKeyFile) == "") {
		return errors.New("--mapping-file and --mapping-key-file must be provided together")
	}
	return nil
}

func safePreflightOutput(_ ...string) string {
	return "preflight output contains counts, object IDs, and repair classes only"
}

func main() {
	if err := runPreflight(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "identity preflight failed")
		os.Exit(1)
	}
}

func runPreflight(args []string, output io.Writer) error {
	options, err := parsePreflightOptions(args)
	if err != nil {
		return err
	}
	dsn, err := identitymigration.ReadSecretFile(options.DSNFile)
	if err != nil {
		return errors.New("identity preflight could not read the DSN file")
	}
	manager, err := identitymigration.NewCredentialManagerFromFile(options.CredentialKeyFile, options.CredentialKeyID)
	if err != nil {
		return errors.New("identity preflight could not configure credential decryption")
	}
	mapping, err := identitymigration.ReadMappingDocument(options.MappingFile, options.MappingKeyFile)
	if err != nil {
		return errors.New("identity preflight could not verify the mapping document")
	}
	db, err := mysqlconn.Open(string(dsn))
	if err != nil {
		return errors.New("identity preflight could not open the target database")
	}
	defer db.Close()
	if err := db.PingContext(context.Background()); err != nil {
		return errors.New("identity preflight could not reach the target database")
	}
	report, err := identitymigration.Preflight(context.Background(), db, identitymigration.DatabaseOptions{
		Schema:            options.Schema,
		PlatformTenantID:  options.PlatformTenantID,
		Mapping:           mapping,
		CredentialManager: manager,
	})
	if err != nil {
		var consistency *identitymigration.ConsistencyError
		if errors.As(err, &consistency) {
			_, _ = fmt.Fprintln(output, consistency.Report.SafeText())
		}
		return errors.New("identity preflight found an unsafe maintenance target")
	}
	_, err = fmt.Fprintln(output, report.SafeText())
	return err
}
