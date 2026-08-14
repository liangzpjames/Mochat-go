package catalog

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestProviderCompositionContract(t *testing.T) {
	root := providerRepositoryRoot(t)
	catalogSource, err := os.ReadFile(filepath.Join(root, "internal", "modules", "providers", "catalog", "catalog.go"))
	if err != nil {
		t.Fatal(err)
	}
	if err := validateProviderFactorySource(string(catalogSource), "NewRegistry"); err != nil {
		t.Fatalf("catalog factory contract: %v", err)
	}
	mainSource, err := os.ReadFile(filepath.Join(root, "cmd", "mochat-go", "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	if err := validateProductionProviderWiring(string(mainSource)); err != nil {
		t.Fatalf("production composition contract: %v", err)
	}
}

func TestProviderCompositionContractRejectsBadFactories(t *testing.T) {
	cases := []struct {
		name   string
		source string
	}{
		{
			name: "register after unconditional return",
			source: `package catalog
func NewRegistry() *Registry {
		registry := providers.NewRegistry()
		return registry
		registration := providers.Registration{Kind: "ai", Source: providers.SourceExternal}
		_ = registry.Register(registration)
		return registry
}`,
		},
		{
			name: "returns another registry",
			source: `package catalog
func NewRegistry() *Registry {
		registry := providers.NewRegistry()
		registration := providers.Registration{Kind: "ai", Source: providers.SourceExternal}
		_ = registry.Register(registration)
		other := providers.NewRegistry()
		return other
}`,
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if err := validateProviderFactorySource(test.source, "NewRegistry"); err == nil {
				t.Fatal("bad factory unexpectedly passed AST contract")
			}
		})
	}
}

func TestProviderCompositionContractRejectsMissingProductionWiring(t *testing.T) {
	if err := validateProductionProviderWiring(`package main
func main() {}`); err == nil {
		t.Fatal("composition root without catalog factory unexpectedly passed")
	}
}

func providerRepositoryRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime caller unavailable")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", "..", ".."))
}
