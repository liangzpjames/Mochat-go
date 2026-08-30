package store

import (
	"strings"
	"testing"

	"jiyi/mochat-go/internal/integrationfixtureaudit"
)

func TestCurrentStoreIntegrationFixturesDoNotHandwriteBusinessSchemaOrLedger(t *testing.T) {
	sources, err := integrationfixtureaudit.LoadTestSources(".", "integration_fixture_contract_test.go")
	if err != nil {
		t.Fatal(err)
	}
	issues := auditStoreFixtureSources(sources)
	if len(issues) > 0 {
		t.Fatalf("current Store integration fixture contract violations:\n%s", strings.Join(issues, "\n"))
	}
}

func TestStoreFixtureContractMutationRejectsCallbackLocalRunner(t *testing.T) {
	sources := map[string][]byte{
		"callback_integration_test.go": []byte(`package store
import "jiyi/mochat-go/internal/migration"
func newCallbackStore() { newCurrentStoreIntegrationDB(nil) }
func TestCallbackLifecycle() {
	newCallbackStore()
	migration.NewRunner(nil, []migration.Migration{{Version: "0174"}})
}`),
	}
	issues := auditStoreFixtureSources(sources)
	if !issuesContain(issues, "TestCallbackLifecycle", "local migration runner", "migration slice") {
		t.Fatalf("callback fixture local migration runner mutation was not rejected: %v", issues)
	}
}

func TestStoreFixtureContractMutationDiscoversStructuredDatabaseSink(t *testing.T) {
	for _, tc := range []struct {
		name string
		sink string
	}{
		{name: "integrationtestdb NewIsolated", sink: "integrationtestdb.NewIsolated(nil, dsn)"},
		{name: "database sql Open", sink: `sql.Open("mysql", dsn)`},
		{name: "standard registry harness", sink: `testharness.ApplyThrough(nil, nil, "../..", "0174", evidence)`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := `package store
import (
	"database/sql"
	"jiyi/mochat-go/internal/integrationtestdb"
	"jiyi/mochat-go/internal/migration"
	"jiyi/mochat-go/internal/migration/testharness"
)
const integrationDSN = "configured-indirectly"
func openIndirectIntegrationDB(dsn string) { ` + tc.sink + ` }
func TestIndirectIntegrationEntry() {
	openIndirectIntegrationDB(integrationDSN)
	migration.NewRunner(nil, []migration.Migration{{Version: "0174"}})
}`
			issues := auditStoreFixtureSources(map[string][]byte{"indirect_integration_test.go": []byte(source)})
			if !issuesContain(issues, "TestIndirectIntegrationEntry", "local migration runner", "migration slice") {
				t.Fatalf("structured database sink did not discover indirect integration entry: %v", issues)
			}
		})
	}
}

func auditStoreFixtureSources(sources map[string][]byte) []string {
	operation := func(path, function, name, object string) integrationfixtureaudit.OperationAllowance {
		return integrationfixtureaudit.OperationAllowance{
			Ref: integrationfixtureaudit.Ref(path, function), Operation: name, Object: object,
		}
	}
	allowedOperations := []integrationfixtureaudit.OperationAllowance{
		operation("archive_sync_integration_test.go", "TestArchiveSyncMigrationRejectsIncompleteResidualTable", "CREATE TABLE", "mochat_go_archive_sync_runs"),
		operation("archive_sync_integration_test.go", "TestArchiveSyncMigrationRejectsIncompleteResidualTable", "DROP TABLE", "mochat_go_archive_sync_runs"),
		operation("archive_sync_integration_test.go", "TestArchiveSyncMigrationRejectsWrongCompositeSourceForeignKey", "CREATE TABLE", "mochat_go_archive_message_sources"),
		operation("archive_sync_integration_test.go", "TestArchiveSyncMigrationRejectsWrongCompositeSourceForeignKey", "DROP TABLE", "mochat_go_archive_message_sources"),
		operation("archive_sync_integration_test.go", "TestArchiveSyncMigrationRejectsNonUniqueResidualScopeIndex", "CREATE TABLE", "mochat_go_archive_message_sources"),
		operation("archive_sync_integration_test.go", "TestArchiveSyncMigrationRejectsNonUniqueResidualScopeIndex", "DROP TABLE", "mochat_go_archive_message_sources"),
		operation("archive_sync_integration_test.go", "TestArchiveSyncMigrationRejectsPrefixedResidualScopeIndex", "CREATE TABLE", "mochat_go_archive_message_sources"),
		operation("archive_sync_integration_test.go", "TestArchiveSyncMigrationRejectsPrefixedResidualScopeIndex", "DROP TABLE", "mochat_go_archive_message_sources"),
		operation("archive_sync_integration_test.go", "TestArchiveSyncMigrationRejectsWrongAuditScopeIndex", "CREATE TABLE", "mochat_go_archive_sync_audits"),
		operation("archive_sync_integration_test.go", "TestArchiveSyncMigrationRejectsWrongAuditScopeIndex", "DROP TABLE", "mochat_go_archive_sync_audits"),
		operation("dashboard_admin_provisioning_integration_test.go", "TestDashboardAdminProvisioningRealMariaDB", "DROP TABLE", "mochat_go_dashboard_permission_audits"),
	}
	allowedCallRefs := []string{}
	for _, function := range []string{
		"executeArchiveMigrationFile",
		"TestArchiveSyncMigrationRejectsIncompleteResidualTable",
		"TestArchiveSyncMigrationRejectsSingleFactorIdempotencyKeyTypeMismatch",
		"TestArchiveSyncMigrationRejectsWrongCompositeSourceForeignKey",
		"TestArchiveSyncMigrationRejectsNonUniqueResidualScopeIndex",
		"TestArchiveSyncMigrationRejectsPrefixedResidualScopeIndex",
		"TestArchiveSyncMigrationRejectsWrongAuditScopeIndex",
	} {
		for _, call := range []string{"executeArchiveMigrationFile", "executeArchiveMigrationFileErr"} {
			allowedCallRefs = append(allowedCallRefs, integrationfixtureaudit.Ref("archive_sync_integration_test.go", function)+":"+call)
		}
	}
	return integrationfixtureaudit.Audit(sources, integrationfixtureaudit.Config{
		RootFunctions: []string{
			"newCurrentStoreIntegrationDB",
			"newStoreIntegrationDBThrough",
			"newStoreIntegrationDatabase",
		},
		AllowedRunnerRefs: []string{
			integrationfixtureaudit.Ref("mysql_integration_harness_test.go", "newStoreMigrationRunnerThrough"),
		},
		AllowedOperations: allowedOperations,
		ForbiddenCalls: []string{
			"newDashboardAdminProvisioningDB",
			"createArchiveSyncCorpFixture",
			"executeArchiveMigrationFile",
			"executeArchiveMigrationFileErr",
			"task6IntegrationDB",
		},
		AllowedCallRefs: allowedCallRefs,
	})
}

func issuesContain(issues []string, function string, fragments ...string) bool {
	for _, issue := range issues {
		if !strings.Contains(issue, function) {
			continue
		}
		for _, fragment := range fragments {
			if strings.Contains(issue, fragment) {
				return true
			}
		}
	}
	return false
}
