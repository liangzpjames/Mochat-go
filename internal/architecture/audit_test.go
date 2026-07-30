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
		{"application imports adapter", "testdata/application-imports-adapter", "ARCH-APPLICATION-DEPENDENCY"},
		{"module imports dashboard", "testdata/module-imports-dashboard", "ARCH-LEGACY-DEPENDENCY"},
		{"module imports another adapter", "testdata/cross-module-adapter", "ARCH-CROSS-MODULE-PRIVATE"},
		{"legacy SCRM file", "testdata/legacy-scrm", "ARCH-FORBIDDEN-LEGACY-FILE"},
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

func TestLoadPolicyNormalizesPaths(t *testing.T) {
	policy, err := LoadPolicy(filepath.Join("testdata", "policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := policy.ProtectedFiles[0].Path, "internal/dashboard/page.go"; got != want {
		t.Fatalf("protected path = %q, want %q", got, want)
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
