package main

import (
	"strings"
	"testing"
)

func TestPreflightCLIIsReadOnlyAndCanScanAnExplicitMaintenanceSchema(t *testing.T) {
	options, err := parsePreflightOptions([]string{"--dsn-file", "dsn-file", "--schema", "business_schema", "--platform-tenant-id", "7", "--credential-key-file", "credential.key", "--credential-key-id", "task8-wecom-v1", "--mapping-file", "mapping.json", "--mapping-key-file", "mapping.key"})
	if err != nil {
		t.Fatal(err)
	}
	if options.Execute || options.DSNFile != "dsn-file" || options.Schema != "business_schema" || options.PlatformTenantID != 7 || options.CredentialKeyFile != "credential.key" || options.CredentialKeyID != "task8-wecom-v1" || options.MappingFile != "mapping.json" {
		t.Fatalf("preflight options=%+v, want read-only maintenance-schema scan", options)
	}
	if err := validatePreflightOptions(options); err != nil {
		t.Fatalf("read-only preflight rejected maintenance schema: %v", err)
	}
}

func TestPreflightCLIRequiresDedicatedCredentialKey(t *testing.T) {
	_, err := parsePreflightOptions([]string{"--dsn-file", "dsn-file", "--schema", "business_schema", "--platform-tenant-id", "7"})
	if err == nil || !strings.Contains(err.Error(), "credential-key") {
		t.Fatalf("missing credential key error=%v", err)
	}
}

func TestPreflightCLIRejectsWriteFlagsAndSensitiveOutputContract(t *testing.T) {
	if _, err := parsePreflightOptions([]string{"--execute"}); err == nil {
		t.Fatal("preflight accepted --execute")
	}
	if strings.Contains(safePreflightOutput("13800000000", "password", "secret", "token"), "13800000000") {
		t.Fatal("preflight renderer accepted sensitive values")
	}
}
