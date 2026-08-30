package migration

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"strings"
	"testing"
)

var fixtureCreateTablePattern = regexp.MustCompile(`(?i)CREATE\s+TABLE(?:\s+IF\s+NOT\s+EXISTS)?\s+(?:[[:alnum:]_]+\.)?[` + "`" + `]?([[:alnum:]_]+)` + "`" + `?`)

func TestMySQLIntegrationFixturesUseProductionRegistryBaselines(t *testing.T) {
	targets := map[string]map[string]map[string]bool{
		"archive_runner_integration_test.go": {
			"TestArchiveSourceMigrationRunnerApplyDownApplyPinsOneConnection": {
				"mochat_go_archive_sync_runs": true,
			},
		},
		"dashboard_page_rbac_integration_test.go": {
			"TestDashboardPageRBACIntegration": {
				"mochat_go_dashboard_permissions":          true,
				"mochat_go_dashboard_permission_resources": true,
				"mochat_go_dashboard_user_roles":           true,
			},
			"newDashboardRBACMigrationDB":      {},
			"createDashboardRBACLegacyFixture": {},
		},
		"identity_realms_single_corp_backfill_integration_test.go": {
			"TestIdentityRealmsSingleCorpBackfillRealMariaDB": {},
		},
		"identity_realms_single_corp_integration_test.go": {
			"newIdentitySingleCorpMigrationDB":    {},
			"createIdentitySingleCorpBaseFixture": {},
		},
		"wecom_capability_ledger_contract_test.go": {
			"TestWeComCapabilityLedgerRealRunnerApplyDownApply": {},
			"withTemporaryWeComCapabilityLedgerSchema":          {},
			"createWeComCapabilityLedgerPreMigrationFixture":    {},
		},
	}

	for path, functions := range targets {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, body, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok {
				continue
			}
			allowedTables, targeted := functions[function.Name.Name]
			if !targeted {
				continue
			}
			start, end := int(function.Pos()-file.Pos()), int(function.End()-file.Pos())
			source := string(body[start:end])
			for _, match := range fixtureCreateTablePattern.FindAllStringSubmatch(source, -1) {
				table := strings.ToLower(match[1])
				if !allowedTables[table] {
					t.Fatalf("%s:%s handwrites non-probe table %s instead of using a production registry prefix", path, function.Name.Name, table)
				}
			}
			upper := strings.ToUpper(source)
			compact := strings.NewReplacer(" ", "", "\t", "", "\r", "", "\n", "").Replace(upper)
			for _, bypass := range []string{"CREATE DATABASE", "DBNAME=\"\"", "[]MIGRATION{MIGRATION}"} {
				if strings.Contains(compact, strings.ReplaceAll(bypass, " ", "")) {
					t.Fatalf("%s:%s bypasses the production migration registry through %s", path, function.Name.Name, bypass)
				}
			}
			for _, mutation := range []string{"CREATE TABLE MOCHAT_GO_SCHEMA_MIGRATIONS", "INSERT INTO MOCHAT_GO_SCHEMA_MIGRATIONS", "UPDATE MOCHAT_GO_SCHEMA_MIGRATIONS"} {
				if strings.Contains(upper, mutation) {
					t.Fatalf("%s:%s handwrites the standard migration ledger with %q", path, function.Name.Name, mutation)
				}
			}
			delete(functions, function.Name.Name)
		}
		for name := range functions {
			t.Fatalf("static migration fixture target %s:%s no longer exists", path, name)
		}
	}
}
