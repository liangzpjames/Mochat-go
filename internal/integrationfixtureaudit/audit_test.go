package integrationfixtureaudit

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadTestSourcesExcludesOnlyExactRelativePath(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "nested")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		filepath.Join(root, "integration_fixture_contract_test.go"),
		filepath.Join(nested, "integration_fixture_contract_test.go"),
	} {
		if err := os.WriteFile(path, []byte("package fixture\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	sources, err := LoadTestSources(root, "integration_fixture_contract_test.go")
	if err != nil {
		t.Fatal(err)
	}
	if _, found := sources["integration_fixture_contract_test.go"]; found {
		t.Fatal("exact root contract path was not excluded")
	}
	if _, found := sources["nested/integration_fixture_contract_test.go"]; !found {
		t.Fatal("same basename at a different relative path was incorrectly excluded")
	}
}
