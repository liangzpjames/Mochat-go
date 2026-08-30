package store

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

func TestCurrentStoreIntegrationFixturesDoNotHandwriteBusinessSchemaOrLedger(t *testing.T) {
	targets := map[string]map[string]bool{
		"mysql_integration_harness_test.go": {
			"newCurrentStoreIntegrationDB": true,
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
			"TestArchiveSyncStoreUsesTemporarySchemaForLifecycleAndTenantIsolation": true,
			"TestArchiveSourceStatusUsesCurrentCorpArchiveMode":                     true,
			"TestArchiveSyncStaleRunningRunIsTakenOverWithAudit":                    true,
			"TestArchiveSyncConcurrentFirstEnqueueRereadsDuplicateRun":              true,
			"TestArchiveSyncEnqueueRejectsNamespaceMismatchWithoutMutation":         true,
			"TestArchiveSyncUpsertValidatesRunScopeAndRollsBackSourceFailure":       true,
			"TestArchiveSyncLeaseFenceRejectsStaleWorkerMutations":                  true,
			"TestArchiveSyncConcurrentDifferentRunsClaimOneMessageIdentity":         true,
			"seedCurrentArchiveSyncCorpFixture":                                     true,
			"seedCurrentArchiveMessageFixture":                                      true,
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
			if !ok || !functions[function.Name.Name] {
				continue
			}
			start, end := int(function.Pos()-file.Pos()), int(function.End()-file.Pos())
			source := strings.ToUpper(string(body[start:end]))
			if strings.Contains(source, "CREATE TABLE") {
				t.Fatalf("%s:%s handwrites business CREATE TABLE instead of using the production registry", path, function.Name.Name)
			}
			for _, bypass := range []string{"NEWDASHBOARDADMINPROVISIONINGDB", "CREATEARCHIVESYNCCORPFIXTURE", "EXECUTEARCHIVEMIGRATIONFILE"} {
				if strings.Contains(source, bypass) {
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
