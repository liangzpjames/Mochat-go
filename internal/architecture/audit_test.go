package architecture

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAuditRejectsInvalidDependencies(t *testing.T) {
	cases := []struct {
		name   string
		root   string
		ruleID string
	}{
		{"domain imports SQL", "testdata/domain-imports-sql", "ARCH-DOMAIN-DEPENDENCY"},
		{"domain imports SQL driver", "testdata/domain-imports-sql-driver", "ARCH-DOMAIN-DEPENDENCY"},
		{"application imports adapter", "testdata/application-imports-adapter", "ARCH-APPLICATION-DEPENDENCY"},
		{"application imports HTTP test package", "testdata/application-imports-http-subpackage", "ARCH-APPLICATION-DEPENDENCY"},
		{"module imports dashboard", "testdata/module-imports-dashboard", "ARCH-LEGACY-DEPENDENCY"},
		{"module imports another adapter", "testdata/cross-module-adapter", "ARCH-CROSS-MODULE-PRIVATE"},
		{"legacy SCRM file", "testdata/legacy-scrm", "ARCH-FORBIDDEN-LEGACY-FILE"},
		{"domain imports own ports", "testdata/domain-imports-own-ports", "ARCH-DOMAIN-DEPENDENCY"},
		{"domain imports internal infrastructure", "testdata/domain-imports-authjwt", "ARCH-DOMAIN-DEPENDENCY"},
		{"application imports internal infrastructure", "testdata/application-imports-mysqlconn", "ARCH-APPLICATION-DEPENDENCY"},
		{"application imports another module domain and ports", "testdata/application-imports-other-module", "ARCH-APPLICATION-DEPENDENCY"},
		{"domain imports third party", "testdata/domain-imports-third-party", "ARCH-DOMAIN-DEPENDENCY"},
		{"domain imports dotless third party", "testdata/domain-imports-dotless-third-party", "ARCH-DOMAIN-DEPENDENCY"},
		{"adapter imports transport", "testdata/adapter-imports-transport", "ARCH-ADAPTERS-DEPENDENCY"},
		{"transport imports adapter", "testdata/transport-imports-adapter", "ARCH-TRANSPORT-DEPENDENCY"},
		{"module composition imports third party", "testdata/module-imports-third-party", "ARCH-MODULE-DEPENDENCY"},
		{"unknown module layer imports adapter", "testdata/unknown-module-layer", "ARCH-MODULE-DEPENDENCY"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			policy := testPolicy()
			violations, err := Audit(tc.root, policy, time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC))
			if err != nil {
				t.Fatal(err)
			}
			if !containsRule(violations, tc.ruleID) {
				t.Fatalf("violations = %#v, want %s", violations, tc.ruleID)
			}
		})
	}
}

func TestAuditAllowsOnlyExactPublicContractException(t *testing.T) {
	policy := testPolicy()
	policy.Exceptions = []Exception{{
		RuleID:    RuleApplicationDependency,
		Path:      "internal/modules/alpha/application/bad.go",
		Import:    "jiyi/mochat-go/internal/modules/beta/domain",
		Reason:    "temporary versioned public contract",
		Owner:     "backend",
		CreatedOn: "2026-07-30",
		ExpiresOn: "2026-08-15",
		Cleanup:   "replace with an alpha-owned port",
	}}
	violations, err := Audit("testdata/application-imports-other-module", policy, time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if containsImportViolation(violations, RuleApplicationDependency, "jiyi/mochat-go/internal/modules/beta/domain") {
		t.Fatalf("exact public-contract exception was not applied: %#v", violations)
	}
	if !containsImportViolation(violations, RuleApplicationDependency, "jiyi/mochat-go/internal/modules/beta/ports") {
		t.Fatalf("exception leaked beyond its exact import: %#v", violations)
	}
}

func TestPackageFamilyRequiresPathSegmentBoundary(t *testing.T) {
	for name, tc := range map[string]struct {
		imported string
		root     string
		want     bool
	}{
		"exact":              {imported: "database/sql", root: "database/sql", want: true},
		"subpackage":         {imported: "database/sql/driver", root: "database/sql", want: true},
		"similar package":    {imported: "database/sqlx", root: "database/sql", want: false},
		"similar repository": {imported: "jiyi/mochat-governance/internal/store", root: "jiyi/mochat-go", want: false},
	} {
		t.Run(name, func(t *testing.T) {
			if got := isPackageFamily(tc.imported, tc.root); got != tc.want {
				t.Fatalf("isPackageFamily(%q, %q) = %t, want %t", tc.imported, tc.root, got, tc.want)
			}
		})
	}
}

func TestAuditRejectsExpiredException(t *testing.T) {
	policy := testPolicy()
	policy.Exceptions = []Exception{{
		RuleID:    "ARCH-LEGACY-DEPENDENCY",
		Path:      "internal/modules/bad/module.go",
		Import:    "jiyi/mochat-go/internal/dashboard",
		Reason:    "temporary migration bridge",
		Owner:     "backend",
		CreatedOn: "2026-07-01",
		ExpiresOn: "2026-07-29",
		Cleanup:   "replace with a port",
	}}
	violations, err := Audit("testdata/module-imports-dashboard", policy, time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if !containsRule(violations, "ARCH-EXCEPTION-EXPIRED") {
		t.Fatalf("violations = %#v", violations)
	}
}

func TestAuditRejectsMissingProtectedFile(t *testing.T) {
	const protectedPath = "internal/dashboard/page.go"
	violations, err := Audit(t.TempDir(), Policy{
		ProtectedFiles: []SizeLimit{{Path: protectedPath, MaxBytes: 1}},
	}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	assertViolation(t, violations, RuleProtectedFileMissing, protectedPath, "protected file is missing")
}

func TestAuditReconcilesModuleDirectoriesWithPolicy(t *testing.T) {
	const root = "testdata/module-registration"
	t.Run("unregistered directory", func(t *testing.T) {
		violations, err := Audit(root, Policy{
			ProductionModules: []string{"registered"},
		}, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		assertViolation(
			t,
			violations,
			RuleModuleRegistration,
			"internal/modules/unregistered",
			"module directory is not registered as production or example",
		)
	})

	t.Run("declared production directory missing", func(t *testing.T) {
		violations, err := Audit(root, Policy{
			ProductionModules: []string{"registered", "missing"},
			ExampleModules:    []string{"unregistered"},
		}, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		assertViolation(
			t,
			violations,
			RuleModuleRegistration,
			"internal/modules/missing",
			"production module declared in policy is missing",
		)
	})

	t.Run("explicit example directory", func(t *testing.T) {
		violations, err := Audit(root, Policy{
			ProductionModules: []string{"registered"},
			ExampleModules:    []string{"unregistered"},
		}, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		if containsRule(violations, RuleModuleRegistration) {
			t.Fatalf("violations = %#v", violations)
		}
	})
}

func TestLoadPolicyRejectsModuleRegisteredTwice(t *testing.T) {
	path := filepath.Join(t.TempDir(), "policy.json")
	contents := []byte(`{
  "productionModules": ["scrm"],
  "exampleModules": ["scrm"],
  "protectedFiles": [],
  "forbiddenNewFiles": [],
  "exceptions": []
}`)
	if err := os.WriteFile(path, contents, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadPolicy(path); err == nil {
		t.Fatal("LoadPolicy accepted a module classified as both production and example")
	}
}

func TestLoadPolicyNormalizesPaths(t *testing.T) {
	policy, err := LoadPolicy(filepath.Join("testdata", "policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := policy.ProtectedFiles[0].Path, "internal/dashboard/page.go"; got != want {
		t.Fatalf("protected path = %q, want %q", got, want)
	}
}

func TestNormalizePathTreatsWindowsSeparatorsAsSlashes(t *testing.T) {
	if got, want := normalizePath(`internal\dashboard\page.go`), "internal/dashboard/page.go"; got != want {
		t.Fatalf("normalized path = %q, want %q", got, want)
	}
}

func TestNormalizePortablePathTreatsWindowsSeparatorsAsSlashes(t *testing.T) {
	if got, want := normalizePortablePath(`internal\dashboard\page.go`), "internal/dashboard/page.go"; got != want {
		t.Fatalf("normalized path = %q, want %q", got, want)
	}
}

func TestLoadPolicyRejectsBlankPaths(t *testing.T) {
	path := filepath.Join(t.TempDir(), "policy.json")
	contents := []byte(`{
  "productionModules": ["scrm"],
  "protectedFiles": [{"path": " ", "maxBytes": 1}],
  "forbiddenNewFiles": [],
  "exceptions": []
}`)
	if err := os.WriteFile(path, contents, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadPolicy(path); err == nil {
		t.Fatal("LoadPolicy accepted a blank protected path")
	}
}

func TestLoadPolicyRejectsMalformedForbiddenFilePattern(t *testing.T) {
	path := filepath.Join(t.TempDir(), "policy.json")
	contents := []byte(`{
  "productionModules": ["scrm"],
  "protectedFiles": [],
  "forbiddenNewFiles": ["internal/["],
  "exceptions": []
}`)
	if err := os.WriteFile(path, contents, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadPolicy(path); err == nil {
		t.Fatal("LoadPolicy accepted a malformed forbidden-file pattern")
	}
}

func TestExceptionValidRejectsInvalidMetadata(t *testing.T) {
	valid := Exception{
		RuleID:    "ARCH-LEGACY-DEPENDENCY",
		Path:      "internal/modules/bad/module.go",
		Import:    "jiyi/mochat-go/internal/dashboard",
		Reason:    "temporary migration bridge",
		Owner:     "backend",
		CreatedOn: "2026-07-01",
		ExpiresOn: "2026-07-30",
		Cleanup:   "replace with a port",
	}
	if !valid.Valid() {
		t.Fatal("valid exception rejected")
	}

	for _, invalid := range []Exception{
		{RuleID: valid.RuleID, Path: valid.Path, Import: valid.Import, Reason: valid.Reason, Owner: valid.Owner, CreatedOn: valid.CreatedOn, ExpiresOn: valid.ExpiresOn},
		{RuleID: valid.RuleID, Path: "internal/modules/*/module.go", Import: valid.Import, Reason: valid.Reason, Owner: valid.Owner, CreatedOn: valid.CreatedOn, ExpiresOn: valid.ExpiresOn, Cleanup: valid.Cleanup},
		{RuleID: valid.RuleID, Path: valid.Path, Import: valid.Import, Reason: valid.Reason, Owner: valid.Owner, CreatedOn: "2026/07/01", ExpiresOn: valid.ExpiresOn, Cleanup: valid.Cleanup},
		{RuleID: valid.RuleID, Path: valid.Path, Import: valid.Import, Reason: valid.Reason, Owner: valid.Owner, CreatedOn: "2026-07-31", ExpiresOn: valid.ExpiresOn, Cleanup: valid.Cleanup},
		{RuleID: valid.RuleID, Path: valid.Path, Import: valid.Import, Reason: valid.Reason, Owner: valid.Owner, CreatedOn: valid.CreatedOn, ExpiresOn: "2026-10-01", Cleanup: valid.Cleanup},
	} {
		if invalid.Valid() {
			t.Fatalf("invalid exception accepted: %#v", invalid)
		}
	}
}

func TestAuditUsesPlatformNeutralSizeLimit(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "internal", "dashboard")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "page.go"), []byte("package page\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	violations, err := Audit(root, Policy{ProtectedFiles: []SizeLimit{{Path: "internal/dashboard/page.go", MaxBytes: 13}}}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if containsRule(violations, RuleProtectedFileSize) {
		t.Fatalf("violations = %#v, CRLF must count as one newline", violations)
	}
}

func TestAuditSkipsNestedTestdataOutsideAuditRoot(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "internal", "architecture", "testdata", "internal", "modules", "bad", "domain")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "bad.go"), []byte("package domain\nimport _ \"database/sql\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	violations, err := Audit(root, Policy{}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) != 0 {
		t.Fatalf("violations = %#v, want testdata to be ignored", violations)
	}
}

func TestAuditAllowsTransportInwardAndSharedContracts(t *testing.T) {
	root := t.TempDir()
	writeArchitectureTestFile(t, root, "internal/modules/alpha/ports/repository.go", "package ports\n")
	writeArchitectureTestFile(t, root, "internal/modules/alpha/transport/http/handler.go", `package http
import (
	_ "jiyi/mochat-go/internal/modules/alpha/ports"
	_ "jiyi/mochat-go/internal/app/modules"
	_ "jiyi/mochat-go/internal/httpresponse"
)
`)
	violations, err := Audit(root, Policy{ProductionModules: []string{"alpha"}}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if containsRule(violations, RuleTransportDependency) {
		t.Fatalf("approved inward/shared transport contracts rejected: %#v", violations)
	}
}

func TestAuditAllowsOnlyExactCrossModulePublicContract(t *testing.T) {
	root := t.TempDir()
	writeArchitectureTestFile(t, root, "internal/modules/alpha/application/service.go", `package application
import (
	_ "jiyi/mochat-go/internal/modules/beta/ports"
	_ "jiyi/mochat-go/internal/modules/beta/adapters/mysql"
)
`)
	writeArchitectureTestFile(t, root, "internal/modules/beta/ports/provider.go", "package ports\n")
	writeArchitectureTestFile(t, root, "internal/modules/beta/adapters/mysql/repository.go", "package mysql\n")
	policy := Policy{
		ProductionModules:   []string{"alpha", "beta"},
		PublicModuleImports: []PublicModuleImport{{Module: "alpha", Package: "application", Layer: "application", Import: "jiyi/mochat-go/internal/modules/beta/ports"}},
	}
	violations, err := Audit(root, policy, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if containsImportViolation(violations, RuleApplicationDependency, "jiyi/mochat-go/internal/modules/beta/ports") {
		t.Fatalf("exact public contract rejected: %#v", violations)
	}
	if !containsImportViolation(violations, RuleCrossModulePrivate, "jiyi/mochat-go/internal/modules/beta/adapters/mysql") {
		t.Fatalf("private adapter was allowed by public contract: %#v", violations)
	}
}

func TestAuditPublicContractDoesNotLeakAcrossConsumerLayerOrTargetPrivatePackage(t *testing.T) {
	root := t.TempDir()
	writeArchitectureTestFile(t, root, "internal/modules/alpha/application/service.go", "package application\nimport _ \"jiyi/mochat-go/internal/modules/beta/ports\"\n")
	writeArchitectureTestFile(t, root, "internal/modules/alpha/domain/bad.go", "package domain\nimport _ \"jiyi/mochat-go/internal/modules/beta/ports\"\n")
	writeArchitectureTestFile(t, root, "internal/modules/alpha/application/private.go", "package application\nimport _ \"jiyi/mochat-go/internal/modules/beta/application\"\n")
	writeArchitectureTestFile(t, root, "internal/modules/beta/ports/provider.go", "package ports\n")
	writeArchitectureTestFile(t, root, "internal/modules/beta/application/private.go", "package application\n")
	policy := Policy{ProductionModules: []string{"alpha", "beta"}, PublicModuleImports: []PublicModuleImport{{Module: "alpha", Package: "application", Layer: "application", Import: "jiyi/mochat-go/internal/modules/beta/ports"}}}
	violations, err := Audit(root, policy, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if containsPathViolation(violations, RuleApplicationDependency, "internal/modules/alpha/application/service.go") {
		t.Fatalf("exact public contract rejected: %#v", violations)
	}
	if !containsPathViolation(violations, RuleDomainDependency, "internal/modules/alpha/domain/bad.go") {
		t.Fatalf("public contract leaked into domain: %#v", violations)
	}
	if !containsPathViolation(violations, RuleApplicationDependency, "internal/modules/alpha/application/private.go") {
		t.Fatalf("private target accepted: %#v", violations)
	}
}

func TestPolicyRejectsPrivatePublicContractTarget(t *testing.T) {
	for _, target := range []string{"domain", "application", "adapters/mysql", "transport/http"} {
		policy := Policy{ProductionModules: []string{"alpha", "beta"}, PublicModuleImports: []PublicModuleImport{{Module: "alpha", Package: "application", Layer: "application", Import: "jiyi/mochat-go/internal/modules/beta/" + target}}}
		if err := policy.normalizeAndValidate(); err == nil {
			t.Fatalf("private %s target accepted as public contract", target)
		}
	}
}

func TestAuditClassifiesOnlyDeclaredCapabilityHubPackages(t *testing.T) {
	root := t.TempDir()
	writeArchitectureTestFile(t, root, "internal/modules/providers/providers.go", "package providers\n")
	writeArchitectureTestFile(t, root, "internal/modules/providers/audio/local/local.go", `package local
import _ "jiyi/mochat-go/internal/modules/providers"
`)
	writeArchitectureTestFile(t, root, "internal/modules/providers/unknown/bad.go", "package unknown\n")
	policy := Policy{
		ProductionModules: []string{"providers"},
		ModulePackages:    []ModulePackage{{Module: "providers", Path: "audio/local", Layer: "adapters"}},
		PackageImports:    []PackageImport{{Module: "providers", Package: "audio/local", Import: "jiyi/mochat-go/internal/modules/providers"}},
	}
	violations, err := Audit(root, policy, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if containsPathViolation(violations, RuleModuleDependency, "internal/modules/providers/audio/local/local.go") {
		t.Fatalf("declared provider adapter rejected: %#v", violations)
	}
	if !containsPathViolation(violations, RuleModuleDependency, "internal/modules/providers/unknown/bad.go") {
		t.Fatalf("unknown provider package was not rejected: %#v", violations)
	}
}

func TestAuditDeclaredCapabilityPackageDoesNotClassifyUnknownDescendants(t *testing.T) {
	root := t.TempDir()
	writeArchitectureTestFile(t, root, "internal/modules/providers/providers.go", "package providers\n")
	writeArchitectureTestFile(t, root, "internal/modules/providers/archive/archive.go", "package archive\n")
	writeArchitectureTestFile(t, root, "internal/modules/providers/archive/unknown/bad.go", "package unknown\n")
	policy := Policy{ProductionModules: []string{"providers"}, ModulePackages: []ModulePackage{{Module: "providers", Path: "archive", Layer: "adapters"}}}
	violations, err := Audit(root, policy, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !containsPathViolation(violations, RuleModuleDependency, "internal/modules/providers/archive/unknown/bad.go") {
		t.Fatalf("unknown descendant inherited declared capability role: %#v", violations)
	}
}

func TestAuditAllowsControlledTestImportsWithoutWeakeningProduction(t *testing.T) {
	root := t.TempDir()
	writeArchitectureTestFile(t, root, "internal/modules/alpha/application/service.go", "package application\n")
	contents := `package mysql
import (
	_ "github.com/DATA-DOG/go-sqlmock"
	_ "jiyi/mochat-go/internal/modules/alpha/application"
)
`
	writeArchitectureTestFile(t, root, "internal/modules/alpha/adapters/mysql/repository_test.go", contents)
	writeArchitectureTestFile(t, root, "internal/modules/alpha/adapters/mysql/repository.go", contents)
	policy := Policy{
		ProductionModules: []string{"alpha"},
		TestImports:       []TestImport{{Module: "alpha", Package: "adapters/mysql", Import: "github.com/DATA-DOG/go-sqlmock"}, {Module: "alpha", Package: "adapters/mysql", Import: "jiyi/mochat-go/internal/modules/alpha/application"}},
	}
	violations, err := Audit(root, policy, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if containsPathViolation(violations, RuleAdaptersDependency, "internal/modules/alpha/adapters/mysql/repository_test.go") {
		t.Fatalf("controlled test composition rejected: %#v", violations)
	}
	if !containsPathViolation(violations, RuleAdaptersDependency, "internal/modules/alpha/adapters/mysql/repository.go") {
		t.Fatalf("production import inherited test allowance: %#v", violations)
	}
}

func TestAuditExactTestContractRejectsWrongLayerPackageAndProduction(t *testing.T) {
	root := t.TempDir()
	writeArchitectureTestFile(t, root, "internal/modules/alpha/adapters/mysql/repository_test.go", "package mysql\nimport _ \"jiyi/mochat-go/internal/modules/alpha/application\"\n")
	writeArchitectureTestFile(t, root, "internal/modules/alpha/adapters/other/repository_test.go", "package other\nimport _ \"jiyi/mochat-go/internal/modules/alpha/application\"\n")
	writeArchitectureTestFile(t, root, "internal/modules/alpha/domain/adapter_test.go", "package domain\nimport _ \"jiyi/mochat-go/internal/modules/alpha/adapters/mysql\"\n")
	writeArchitectureTestFile(t, root, "internal/modules/alpha/domain/transport_test.go", "package domain\nimport _ \"jiyi/mochat-go/internal/modules/alpha/transport/http\"\n")
	writeArchitectureTestFile(t, root, "internal/modules/alpha/adapters/mysql/repository.go", "package mysql\nimport _ \"jiyi/mochat-go/internal/modules/alpha/application\"\n")
	writeArchitectureTestFile(t, root, "internal/modules/alpha/adapters/mysql/mysql.go", "package mysql\n")
	writeArchitectureTestFile(t, root, "internal/modules/alpha/application/service.go", "package application\n")
	writeArchitectureTestFile(t, root, "internal/modules/alpha/transport/http/handler.go", "package http\n")
	policy := Policy{ProductionModules: []string{"alpha"}, TestImports: []TestImport{{Module: "alpha", Package: "adapters/mysql", Import: "jiyi/mochat-go/internal/modules/alpha/application"}}}
	violations, err := Audit(root, policy, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if containsPathViolation(violations, RuleAdaptersDependency, "internal/modules/alpha/adapters/mysql/repository_test.go") {
		t.Fatalf("exact test contract rejected: %#v", violations)
	}
	if !containsPathViolation(violations, RuleAdaptersDependency, "internal/modules/alpha/adapters/other/repository_test.go") {
		t.Fatalf("test contract leaked into wrong package: %#v", violations)
	}
	if !containsPathViolation(violations, RuleDomainDependency, "internal/modules/alpha/domain/adapter_test.go") || !containsPathViolation(violations, RuleDomainDependency, "internal/modules/alpha/domain/transport_test.go") {
		t.Fatalf("domain test imported adapter: %#v", violations)
	}
	if !containsPathViolation(violations, RuleAdaptersDependency, "internal/modules/alpha/adapters/mysql/repository.go") {
		t.Fatalf("production inherited test contract: %#v", violations)
	}
}

func TestAuditProtectedDebtRatchetsWithoutReplacingTarget(t *testing.T) {
	root := t.TempDir()
	writeArchitectureTestFile(t, root, "cmd/app/main.go", "package main\n// debt\n")
	limit := SizeLimit{
		Path: "cmd/app/main.go", TargetBytes: 10, MaxBytes: 21,
		Debt: &ProtectedDebt{
			Owner: "backend-platform", Reason: "legacy composition debt", Action: "extract runtime wiring",
			Baseline: "f2b57f31f2baa93dc871b9157a04b0a8f7e2ae36", CreatedOn: "2026-08-30", ExpiresOn: "2026-11-28",
		},
	}
	violations, err := Audit(root, Policy{ProtectedFiles: []SizeLimit{limit}}, time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if containsRule(violations, RuleProtectedFileSize) || containsRule(violations, RuleProtectedDebtExpired) {
		t.Fatalf("bounded active debt rejected: %#v", violations)
	}
	violations, err = Audit(root, Policy{ProtectedFiles: []SizeLimit{limit}}, time.Date(2026, 11, 28, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if !containsRule(violations, RuleProtectedDebtExpired) {
		t.Fatalf("protected debt remained active on its expiry date: %#v", violations)
	}
}

func TestRepositoryConformsToArchitecturePolicy(t *testing.T) {
	root := filepath.Join("..", "..")
	policy, err := LoadPolicy(filepath.Join(root, "architecture-policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	violations, err := Audit(root, policy, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) != 0 {
		t.Fatalf("repository architecture violations: %#v", violations)
	}
}

func writeArchitectureTestFile(t *testing.T, root, relativePath, contents string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relativePath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func testPolicy() Policy {
	return Policy{
		ProductionModules: []string{"scrm"},
		ForbiddenNewFiles: []string{"internal/dashboard/scrm*.go", "internal/store/scrm*.go"},
	}
}

func containsRule(violations []Violation, ruleID string) bool {
	for _, violation := range violations {
		if violation.RuleID == ruleID {
			return true
		}
	}
	return false
}

func containsImportViolation(violations []Violation, ruleID, imported string) bool {
	want := `imports "` + imported + `"`
	for _, violation := range violations {
		if violation.RuleID == ruleID && violation.Detail == want {
			return true
		}
	}
	return false
}

func containsPathViolation(violations []Violation, ruleID, path string) bool {
	for _, violation := range violations {
		if violation.RuleID == ruleID && violation.Path == path {
			return true
		}
	}
	return false
}

func assertViolation(t *testing.T, violations []Violation, ruleID, path, detail string) {
	t.Helper()
	for _, violation := range violations {
		if violation.RuleID == ruleID && violation.Path == path && violation.Detail == detail {
			return
		}
	}
	t.Fatalf("violations = %#v, want %s %s %q", violations, ruleID, path, detail)
}
