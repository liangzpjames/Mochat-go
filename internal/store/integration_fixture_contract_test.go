package store

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestCurrentStoreIntegrationFixturesDoNotHandwriteBusinessSchemaOrLedger(t *testing.T) {
	targets := map[string]map[string]bool{
		"mysql_integration_harness_test.go": {
			"newCurrentStoreIntegrationDB":   true,
			"newStoreIntegrationDBThrough":   true,
			"newStoreIntegrationDatabase":    true,
			"newStoreMigrationRunnerThrough": true,
		},
		"dashboard_access_integration_test.go": {
			"openDashboardIntegrationDB": true,
		},
		"wework_callback_inbox_integration_test.go": {
			"newWeWorkCallbackInboxIntegrationStore": true,
		},
		"archive_source_read_integration_test.go": {
			"TestArchiveSourceReadUsesRegistryForItemsCountsAndPages":                true,
			"TestArchiveSourceReadLegacySimulationRegistryStaysOutOfExternalDefault": true,
			"seedArchiveReadCorp": true, "createArchiveReadBusinessFixture": true,
		},
		"archive_sync_integration_test.go": {
			"TestArchiveSourceMigrationBackfillsLegacySimulationRowsOnTemporaryMariaDB": true,
			"TestArchiveSyncStoreUsesTemporarySchemaForLifecycleAndTenantIsolation":     true,
			"TestArchiveSourceStatusUsesCurrentCorpArchiveMode":                         true,
			"TestArchiveSyncStaleRunningRunIsTakenOverWithAudit":                        true,
			"TestArchiveSyncConcurrentFirstEnqueueRereadsDuplicateRun":                  true,
			"TestArchiveSyncEnqueueRejectsNamespaceMismatchWithoutMutation":             true,
			"TestArchiveSyncMigrationApplyDownApplyAndRejectsCrossTenantRun":            true,
			"TestArchiveSyncUpsertValidatesRunScopeAndRollsBackSourceFailure":           true,
			"TestArchiveSyncLeaseFenceRejectsStaleWorkerMutations":                      true,
			"TestArchiveSyncConcurrentDifferentRunsClaimOneMessageIdentity":             true,
			"TestArchiveSyncMigrationRejectsIncompleteResidualTable":                    true,
			"TestArchiveSyncMigrationRejectsWrongCompositeSourceForeignKey":             true,
			"TestArchiveSyncMigrationRejectsNonUniqueResidualScopeIndex":                true,
			"TestArchiveSyncMigrationRejectsPrefixedResidualScopeIndex":                 true,
			"TestArchiveSyncMigrationRejectsWrongAuditScopeIndex":                       true,
			"newArchiveSyncProbeDB":             true,
			"seedCurrentArchiveSyncCorpFixture": true,
			"seedCurrentArchiveMessageFixture":  true,
		},
		"message_intercept_integration_test.go": {
			"TestKeywordEntryAtomicityAndConcurrentVersionsAgainstIsolatedMySQL": true,
		},
		"risk_behavior_integration_test.go": {
			"TestRiskAndKeywordAtomicityAgainstIsolatedMySQL": true,
		},
		"work_message_customer_integration_test.go": {
			"TestCustomerDirectoryMariaDBIntegration":    true,
			"TestCustomerConversationMariaDBIntegration": true,
			"TestCustomerDetailMariaDBIntegration":       true,
		},
	}
	allowedProbeTables := map[string]map[string]bool{
		"TestArchiveSyncMigrationRejectsIncompleteResidualTable":        {"mochat_go_archive_sync_runs": true},
		"TestArchiveSyncMigrationRejectsWrongCompositeSourceForeignKey": {"mochat_go_archive_message_sources": true},
		"TestArchiveSyncMigrationRejectsNonUniqueResidualScopeIndex":    {"mochat_go_archive_message_sources": true},
		"TestArchiveSyncMigrationRejectsPrefixedResidualScopeIndex":     {"mochat_go_archive_message_sources": true},
		"TestArchiveSyncMigrationRejectsWrongAuditScopeIndex":           {"mochat_go_archive_sync_audits": true},
	}
	allowedProbeMigrationExecution := map[string]bool{
		"TestArchiveSyncMigrationRejectsIncompleteResidualTable":        true,
		"TestArchiveSyncMigrationRejectsWrongCompositeSourceForeignKey": true,
		"TestArchiveSyncMigrationRejectsNonUniqueResidualScopeIndex":    true,
		"TestArchiveSyncMigrationRejectsPrefixedResidualScopeIndex":     true,
		"TestArchiveSyncMigrationRejectsWrongAuditScopeIndex":           true,
	}
	createTablePattern := regexp.MustCompile(`(?i)CREATE\s+TABLE(?:\s+IF\s+NOT\s+EXISTS)?\s+[` + "`" + `]?([[:alnum:]_]+)` + "`" + `?`)
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
			if !ok || !functions[function.Name.Name] {
				continue
			}
			start, end := int(function.Pos()-file.Pos()), int(function.End()-file.Pos())
			source := strings.ToUpper(string(body[start:end]))
			for _, match := range createTablePattern.FindAllStringSubmatch(source, -1) {
				table := strings.ToLower(match[1])
				if !allowedProbeTables[function.Name.Name][table] {
					t.Fatalf("%s:%s handwrites non-probe table %s instead of using the production registry", path, function.Name.Name, table)
				}
			}
			for _, bypass := range []string{"NEWDASHBOARDADMINPROVISIONINGDB", "CREATEARCHIVESYNCCORPFIXTURE", "EXECUTEARCHIVEMIGRATIONFILE", "TASK6INTEGRATIONDB", "CREATE DATABASE"} {
				if strings.Contains(source, bypass) {
					if bypass == "EXECUTEARCHIVEMIGRATIONFILE" && allowedProbeMigrationExecution[function.Name.Name] {
						continue
					}
					t.Fatalf("%s:%s bypasses the current production registry through %s", path, function.Name.Name, bypass)
				}
			}
			for _, mutation := range []string{"INSERT INTO MOCHAT_GO_SCHEMA_MIGRATIONS", "UPDATE MOCHAT_GO_SCHEMA_MIGRATIONS", "DELETE FROM MOCHAT_GO_SCHEMA_MIGRATIONS", "DROP TABLE MOCHAT_GO_SCHEMA_MIGRATIONS", "CREATE TABLE MOCHAT_GO_SCHEMA_MIGRATIONS"} {
				if strings.Contains(source, mutation) {
					t.Fatalf("%s:%s mutates the production migration ledger with %q", path, function.Name.Name, mutation)
				}
			}
			delete(functions, function.Name.Name)
		}
		for name := range functions {
			t.Fatalf("static fixture contract target %s:%s no longer exists", path, name)
		}
	}
}
