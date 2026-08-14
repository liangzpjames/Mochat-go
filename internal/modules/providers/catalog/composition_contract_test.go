package catalog

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"jiyi/mochat-go/internal/modules/providers"
)

type compositionStatusProvider struct{ status providers.Status }

func (p compositionStatusProvider) Status() providers.Status { return p.status }

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

func TestNewRegistrySnapshotContainsExactlyFourBuiltins(t *testing.T) {
	registry, err := NewRegistry(Dependencies{
		AI:            compositionStatusProvider{status: providers.Status{State: providers.StateReady}},
		AIEnabled:     true,
		Archive:       compositionStatusProvider{status: providers.Status{State: providers.StateLimited}},
		AudioStorage:  compositionStatusProvider{status: providers.Status{State: providers.StateReady}},
		WeComStandard: compositionStatusProvider{status: providers.Status{State: providers.StateReady}},
	})
	if err != nil {
		t.Fatal(err)
	}

	wantSources := map[string]providers.Source{
		"ai":             providers.SourceExternal,
		"audio_storage":  providers.SourceLocal,
		"wecom_archive":  providers.SourceExternal,
		"wecom_standard": providers.SourceExternal,
	}
	snapshot := registry.Snapshot(nil)
	if len(snapshot) != len(wantSources) {
		t.Fatalf("snapshot kinds=%d, want exactly %d: %#v", len(snapshot), len(wantSources), snapshot)
	}
	seen := make(map[string]bool, len(snapshot))
	for _, status := range snapshot {
		if seen[status.Kind] {
			t.Fatalf("duplicate snapshot kind=%q", status.Kind)
		}
		seen[status.Kind] = true
		if wantSources[status.Kind] != status.Source {
			t.Fatalf("kind=%q source=%q, want %q", status.Kind, status.Source, wantSources[status.Kind])
		}
	}
	for kind := range wantSources {
		if !seen[kind] {
			t.Fatalf("snapshot missing kind=%q", kind)
		}
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
		{
			name: "register hidden behind false branch",
			source: `package catalog
func NewRegistry() *Registry {
		registry := providers.NewRegistry()
		if false {
			registration := providers.Registration{Kind: "ai", Source: providers.SourceExternal}
			_ = registry.Register(registration)
		}
		return registry
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

func TestProviderCompositionContractRequiresReachableProductionDataFlow(t *testing.T) {
	source := `package main
import (
	fake "jiyi/mochat-go/internal/modules/providers/catalog"
)
func unused() { fake.NewRegistry(fake.Dependencies{}) }
func main() {}`
	if err := validateProductionProviderWiring(source); err == nil {
		t.Fatal("unreachable factory call without provider status data flow unexpectedly passed")
	}

	wrongAlias := `package main
import (
	fake "jiyi/mochat-go/internal/modules/providers/catalog"
)
func main() { fake.NewRegistry(fake.Dependencies{}) }`
	if err := validateProductionProviderWiring(wrongAlias); err == nil {
		t.Fatal("fake alias without provider status data flow unexpectedly passed")
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
