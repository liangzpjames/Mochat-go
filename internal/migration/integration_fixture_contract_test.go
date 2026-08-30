package migration

import (
	"strings"
	"testing"

	"jiyi/mochat-go/internal/integrationfixtureaudit"
)

func TestMySQLIntegrationFixturesUseProductionRegistryBaselines(t *testing.T) {
	sources, err := integrationfixtureaudit.LoadTestSources(".", "integration_fixture_contract_test.go")
	if err != nil {
		t.Fatal(err)
	}
	issues := auditMigrationFixtureSources(sources)
	if len(issues) > 0 {
		t.Fatalf("migration integration fixture contract violations:\n%s", strings.Join(issues, "\n"))
	}
}

func TestMigrationFixtureContractMutationsRejectLocalRunnerAndParentDDL(t *testing.T) {
	t.Run("WeCom single migration runner", func(t *testing.T) {
		sources := map[string][]byte{
			"wecom_integration_test.go": []byte(`package migration
import (
	"os"
	"jiyi/mochat-go/internal/migration"
)
func withWeComDB() { _ = os.Getenv("MOCHAT_GO_MYSQL_INTEGRATION_DSN") }
func TestWeComLifecycle() {
	withWeComDB()
	migration.NewRunner(nil, []migration.Migration{{Version: "0139"}})
}`),
		}
		issues := auditMigrationFixtureSources(sources)
		if !migrationIssuesContain(issues, "TestWeComLifecycle", "local migration runner", "migration slice") {
			t.Fatalf("WeCom single-migration runner mutation was not rejected: %v", issues)
		}
	})

	t.Run("parent function cannot inherit probe table allowlist", func(t *testing.T) {
		sources := map[string][]byte{
			"identity_realms_single_corp_integration_test.go": []byte(`package migration
func createIdentityUnknownTenantDependencyProbe() {
	newMigrationIntegrationDBThrough(nil, "0128")
	db.Exec("CREATE TABLE identity_dependency_probe (id bigint)")
}
func TestIdentityParent() {
	newMigrationIntegrationDBThrough(nil, "0128")
	db.Exec("CREATE TABLE identity_dependency_probe (id bigint)")
}`),
		}
		issues := auditMigrationFixtureSources(sources)
		if !migrationIssuesContain(issues, "TestIdentityParent", "CREATE TABLE") {
			t.Fatalf("parent function same-name DDL mutation was not rejected: %v", issues)
		}
		for _, issue := range issues {
			if strings.Contains(issue, "createIdentityUnknownTenantDependencyProbe") {
				t.Fatalf("exact probe helper was unexpectedly rejected: %s", issue)
			}
		}
	})

	t.Run("ledger delete must pin controlled 0130 version", func(t *testing.T) {
		sources := map[string][]byte{
			"identity_realms_single_corp_backfill_integration_test.go": []byte(`package migration
func rollbackIdentityBackfillWithEvidence() {
	newMigrationIntegrationDBThrough(nil, "0128")
	db.Exec("DELETE FROM mochat_go_schema_migrations WHERE version=?", "0139_wecom_capability_ledger")
}`),
		}
		issues := auditMigrationFixtureSources(sources)
		if !migrationIssuesContain(issues, "rollbackIdentityBackfillWithEvidence", "exact controlled version") {
			t.Fatalf("wrong-version ledger delete mutation was not rejected: %v", issues)
		}
	})

	t.Run("0139 raw probe is only reachable through exact helper", func(t *testing.T) {
		sources := map[string][]byte{
			"wecom_capability_ledger_contract_test.go": []byte(`package migration_test
import (
	"os"
	"jiyi/mochat-go/internal/migration"
)
func TestWeComDirectProbe() {
	_ = os.Getenv("MOCHAT_GO_MYSQL_INTEGRATION_DSN")
	migration.ExecuteWeComCapabilityLedger0139UpProbe(nil, nil, "../..")
}`),
		}
		issues := auditMigrationFixtureSources(sources)
		if !migrationIssuesContain(issues, "TestWeComDirectProbe", "bypasses") {
			t.Fatalf("direct 0139 raw probe mutation was not rejected: %v", issues)
		}
	})

	t.Run("exact runner helper rejects single element registry", func(t *testing.T) {
		sources := map[string][]byte{
			"mysql_integration_harness_test.go": []byte(`package migration
func newMigrationIntegrationDBThrough() { newMigrationRunnerThrough() }
func newMigrationRunnerThrough() {
	NewRunner(nil, []Migration{{Version: "0139"}})
}`),
		}
		issues := auditMigrationFixtureSources(sources)
		if !migrationIssuesContain(issues, "newMigrationRunnerThrough", "exact production prefix") {
			t.Fatalf("exact helper single-element runner mutation was not rejected: %v", issues)
		}
	})

	t.Run("ledger mutation resolves variable const and helper", func(t *testing.T) {
		for _, tc := range []struct {
			name   string
			source string
		}{
			{
				name: "file const and local variable",
				source: `package migration
const ledgerTable = "mochat_go_schema_migrations"
func newMigrationIntegrationDBThrough() {
	query := "DELETE FROM " + ledgerTable + " WHERE version=?"
	db.Exec(query, "0139_wecom_capability_ledger")
}`,
			},
			{
				name: "helper concatenation",
				source: `package migration
const ledgerTable = "mochat_go_schema_migrations"
func ledgerDeleteSQL() string { return "DELETE FROM " + ledgerTable + " WHERE version=?" }
func newMigrationIntegrationDBThrough() {
	db.Exec(ledgerDeleteSQL(), "0139_wecom_capability_ledger")
}`,
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				issues := auditMigrationFixtureSources(map[string][]byte{"ledger_integration_test.go": []byte(tc.source)})
				if !migrationIssuesContain(issues, "ledger", "standard migration ledger", "exact controlled version") {
					t.Fatalf("ledger indirection mutation was not rejected: %v", issues)
				}
			})
		}
	})

	t.Run("function value aliases keep canonical call policy", func(t *testing.T) {
		for _, tc := range []struct {
			name      string
			source    string
			function  string
			fragments []string
		}{
			{
				name: "NewRunner selector alias",
				source: `package migration
func newMigrationIntegrationDBThrough() {
	run := NewRunner
	var migrations []Migration
	run(nil, migrations)
}`,
				function:  "newMigrationIntegrationDBThrough",
				fragments: []string{"local migration runner"},
			},
			{
				name: "sql Open selector alias",
				source: `package migration
import "database/sql"
func openAliasedDatabase() {
	open := sql.Open
	open("mysql", "configured-indirectly")
}
func TestAliasedDatabaseEntry() {
	openAliasedDatabase()
	NewRunner(nil, []Migration{{Version: "0139"}})
}`,
				function:  "TestAliasedDatabaseEntry",
				fragments: []string{"local migration runner", "migration slice"},
			},
			{
				name: "0139 forbidden selector alias",
				source: `package migration_test
import "jiyi/mochat-go/internal/migration"
func TestAliasedWeComProbe() {
	_ = "MOCHAT_GO_MYSQL_INTEGRATION_DSN"
	open := migration.ExecuteWeComCapabilityLedger0139UpProbe
	open(nil, nil, "../..")
}`,
				function:  "TestAliasedWeComProbe",
				fragments: []string{"bypasses"},
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				issues := auditMigrationFixtureSources(map[string][]byte{"aliased_integration_test.go": []byte(tc.source)})
				if !migrationIssuesContain(issues, tc.function, tc.fragments...) {
					t.Fatalf("function-value alias mutation was not rejected: %v", issues)
				}
			})
		}
	})

	t.Run("ledger helper propagates constant parameters", func(t *testing.T) {
		sources := map[string][]byte{
			"ledger_parameter_integration_test.go": []byte(`package migration
func ledgerTable(table string) string {
	return table
}
func newMigrationIntegrationDBThrough() {
	query := "DELETE FROM " + ledgerTable("mochat_go_schema_migrations") + " WHERE version=?"
	db.Exec(query, "0139_wecom_capability_ledger")
}`),
		}
		issues := auditMigrationFixtureSources(sources)
		if !migrationIssuesContain(issues, "newMigrationIntegrationDBThrough", "exact controlled version") {
			t.Fatalf("ledger helper parameter mutation was not rejected: %v", issues)
		}
	})

	t.Run("method cannot inherit package helper allowlist", func(t *testing.T) {
		sources := map[string][]byte{
			"identity_realms_single_corp_integration_test.go": []byte(`package migration
type fixtureProbe struct{}
func (fixtureProbe) createIdentityUnknownTenantDependencyProbe() {
	newMigrationIntegrationDBThrough(nil, "0128")
	db.Exec("CREATE TABLE identity_dependency_probe (id bigint)")
}`),
		}
		issues := auditMigrationFixtureSources(sources)
		if !migrationIssuesContain(issues, "fixtureProbe.createIdentityUnknownTenantDependencyProbe", "CREATE TABLE") {
			t.Fatalf("same-name method inherited package helper allowlist: %v", issues)
		}
	})
}

func auditMigrationFixtureSources(sources map[string][]byte) []string {
	operation := func(path, function, name, object string) integrationfixtureaudit.OperationAllowance {
		return integrationfixtureaudit.OperationAllowance{
			Ref: integrationfixtureaudit.Ref(path, function), Operation: name, Object: object,
		}
	}
	return integrationfixtureaudit.Audit(sources, integrationfixtureaudit.Config{
		RootFunctions: []string{"newMigrationIntegrationDBThrough"},
		AllowedRunnerRefs: []string{
			integrationfixtureaudit.Ref("mysql_integration_harness_test.go", "newMigrationRunnerThrough"),
			integrationfixtureaudit.Ref("mysql_external_integration_harness_test.go", "newExternalMigrationRunnerThrough"),
		},
		AllowedOperations: []integrationfixtureaudit.OperationAllowance{
			operation("archive_runner_integration_test.go", "TestArchiveSourceMigrationRunnerApplyDownApplyPinsOneConnection", "CREATE TABLE", "mochat_go_archive_sync_runs"),
			operation("archive_runner_integration_test.go", "TestArchiveSourceMigrationRunnerApplyDownApplyPinsOneConnection", "DROP TABLE", "mochat_go_archive_sync_runs"),
			operation("dashboard_page_rbac_integration_test.go", "createDashboardRBACPartialTablesProbe", "CREATE TABLE", "mochat_go_dashboard_permissions"),
			operation("dashboard_page_rbac_integration_test.go", "createDashboardRBACPartialTablesProbe", "CREATE TABLE", "mochat_go_dashboard_permission_resources"),
			operation("dashboard_page_rbac_integration_test.go", "createDashboardRBACPartialTablesProbe", "CREATE TABLE", "mochat_go_dashboard_user_roles"),
			operation("identity_realms_single_corp_integration_test.go", "createIdentityUnknownTenantDependencyProbe", "CREATE TABLE", "identity_dependency_probe"),
			operation("identity_realms_single_corp_integration_test.go", "createIdentityPartialDashboardIdentityProbe", "CREATE TABLE", "mochat_go_dashboard_identities"),
			operation("identity_realms_single_corp_backfill_integration_test.go", "TestIdentityRealmsSingleCorpBackfillDownToleratesPartialDDL", "DROP TABLE", "mochat_go_identity_migration_ledger"),
			operation("wecom_capability_ledger_contract_test.go", "createWeComCapabilityExternalForeignKeyProbe", "CREATE TABLE", "mo_chat_wecom_0139_external_fk_probe"),
			operation("wecom_capability_ledger_contract_test.go", "createWeComCapabilityExternalForeignKeyProbeDatabase", "CREATE DATABASE", "<dynamic>"),
			operation("wecom_capability_ledger_contract_test.go", "TestWeComCapabilityLedgerDownRecoversPartialStateThenReapplies", "DROP TABLE", "mochat_go_wecom_capability_operation_events"),
		},
		AllowedLedgerDelete: map[string]string{
			integrationfixtureaudit.Ref("identity_realms_single_corp_backfill_integration_test.go", "TestIdentityRealmsSingleCorpBackfillRealMariaDB"): "0130_identity_realms_single_corp_backfill",
			integrationfixtureaudit.Ref("identity_realms_single_corp_backfill_integration_test.go", "rollbackIdentityBackfillWithEvidence"):            "0130_identity_realms_single_corp_backfill",
		},
		ForbiddenCalls: []string{"ExecuteWeComCapabilityLedger0139UpProbe"},
		AllowedCallRefs: []string{
			integrationfixtureaudit.Ref("wecom_capability_ledger_contract_test.go", "executeWeComCapabilityLedgerUpResidualProbe") + ":ExecuteWeComCapabilityLedger0139UpProbe",
		},
	})
}

func migrationIssuesContain(issues []string, function string, fragments ...string) bool {
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
