// Package architecture checks Go source trees against repository architecture rules.
package architecture

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	RuleDomainDependency      = "ARCH-DOMAIN-DEPENDENCY"
	RulePortsDependency       = "ARCH-PORTS-DEPENDENCY"
	RuleApplicationDependency = "ARCH-APPLICATION-DEPENDENCY"
	RuleAdaptersDependency    = "ARCH-ADAPTERS-DEPENDENCY"
	RuleTransportDependency   = "ARCH-TRANSPORT-DEPENDENCY"
	RuleModuleDependency      = "ARCH-MODULE-DEPENDENCY"
	RuleLegacyDependency      = "ARCH-LEGACY-DEPENDENCY"
	RuleCrossModulePrivate    = "ARCH-CROSS-MODULE-PRIVATE"
	RuleForbiddenLegacyFile   = "ARCH-FORBIDDEN-LEGACY-FILE"
	RuleProtectedFileSize     = "ARCH-PROTECTED-FILE-SIZE"
	RuleProtectedFileMissing  = "ARCH-PROTECTED-FILE-MISSING"
	RuleProtectedDebtExpired  = "ARCH-PROTECTED-DEBT-EXPIRED"
	RuleModuleRegistration    = "ARCH-MODULE-REGISTRATION"
	RuleExceptionExpired      = "ARCH-EXCEPTION-EXPIRED"
)

// Violation is an architecture-policy finding.
type Violation struct {
	RuleID string
	Path   string
	Detail string
}

// SizeLimit prevents protected files from exceeding their recorded size limit.
type SizeLimit struct {
	Path        string         `json:"path"`
	TargetBytes int64          `json:"targetBytes,omitempty"`
	MaxBytes    int64          `json:"maxBytes"`
	Debt        *ProtectedDebt `json:"debt,omitempty"`
}

// ProtectedDebt is a bounded ratchet for legacy growth already present in a
// protected file. TargetBytes remains the cleanup target; MaxBytes is only a
// temporary no-growth ceiling and requires complete, expiring ownership.
type ProtectedDebt struct {
	Owner     string `json:"owner"`
	Reason    string `json:"reason"`
	Action    string `json:"action"`
	Baseline  string `json:"baseline"`
	CreatedOn string `json:"createdOn"`
	ExpiresOn string `json:"expiresOn"`
}

// ModulePackage assigns a deliberate non-standard package subtree to a
// standard dependency layer. It is used for capability hubs such as provider
// adapters while unknown subtrees remain fail-closed.
type ModulePackage struct {
	Module string `json:"module"`
	Path   string `json:"path"`
	Layer  string `json:"layer"`
}

// PublicModuleImport is an exact cross-module public contract.
type PublicModuleImport struct {
	Module  string `json:"module"`
	Package string `json:"package"`
	Layer   string `json:"layer"`
	Import  string `json:"import"`
}

// PackageImport is an exact dependency required by one declared package.
type PackageImport struct {
	Module  string `json:"module"`
	Package string `json:"package"`
	Import  string `json:"import"`
}

// TestImport is an exact import granted only to _test.go files in one module
// package. It never changes production dependencies or other test packages.
type TestImport struct {
	Module  string `json:"module"`
	Package string `json:"package"`
	Import  string `json:"import"`
}

// Policy describes repository architecture constraints.
type Policy struct {
	ProductionModules   []string             `json:"productionModules"`
	ExampleModules      []string             `json:"exampleModules"`
	ModulePackages      []ModulePackage      `json:"modulePackages,omitempty"`
	PublicModuleImports []PublicModuleImport `json:"publicModuleImports,omitempty"`
	PackageImports      []PackageImport      `json:"packageImports,omitempty"`
	TestImports         []TestImport         `json:"testImports,omitempty"`
	ProtectedFiles      []SizeLimit          `json:"protectedFiles"`
	ForbiddenNewFiles   []string             `json:"forbiddenNewFiles"`
	Exceptions          []Exception          `json:"exceptions"`
}

// Exception records a temporary, exact import-policy exception.
type Exception struct {
	RuleID    string `json:"ruleID"`
	Path      string `json:"path"`
	Import    string `json:"import"`
	Reason    string `json:"reason"`
	Owner     string `json:"owner"`
	CreatedOn string `json:"createdOn"`
	ExpiresOn string `json:"expiresOn"`
	Cleanup   string `json:"cleanup"`
}

// LoadPolicy loads and validates a JSON architecture policy.
func LoadPolicy(path string) (Policy, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return Policy{}, err
	}

	var policy Policy
	if err := json.Unmarshal(contents, &policy); err != nil {
		return Policy{}, fmt.Errorf("decode architecture policy: %w", err)
	}
	if err := policy.normalizeAndValidate(); err != nil {
		return Policy{}, fmt.Errorf("validate architecture policy: %w", err)
	}
	return policy, nil
}

// Valid reports whether an exception has complete, bounded metadata.
func (e Exception) Valid() bool {
	for _, value := range []string{e.RuleID, e.Path, e.Import, e.Reason, e.Owner, e.CreatedOn, e.ExpiresOn, e.Cleanup} {
		if strings.TrimSpace(value) == "" {
			return false
		}
	}
	if hasWildcard(e.Path) || filepath.IsAbs(e.Path) {
		return false
	}

	createdOn, err := time.Parse("2006-01-02", e.CreatedOn)
	if err != nil {
		return false
	}
	expiresOn, err := time.Parse("2006-01-02", e.ExpiresOn)
	if err != nil || expiresOn.Before(createdOn) || expiresOn.Sub(createdOn) > 90*24*time.Hour {
		return false
	}
	return true
}

func (p *Policy) normalizeAndValidate() error {
	registeredModules := make(map[string]string, len(p.ProductionModules)+len(p.ExampleModules))
	if err := normalizeModuleList("productionModules", p.ProductionModules, registeredModules); err != nil {
		return err
	}
	if err := normalizeModuleList("exampleModules", p.ExampleModules, registeredModules); err != nil {
		return err
	}
	for index := range p.ProtectedFiles {
		limit := &p.ProtectedFiles[index]
		limit.Path = normalizePath(limit.Path)
		if limit.TargetBytes == 0 {
			limit.TargetBytes = limit.MaxBytes
		}
		if limit.Path == "" || filepath.IsAbs(limit.Path) || hasWildcard(limit.Path) || limit.TargetBytes < 0 || limit.MaxBytes < limit.TargetBytes {
			return fmt.Errorf("protectedFiles[%d] is invalid", index)
		}
		if limit.MaxBytes > limit.TargetBytes {
			if limit.Debt == nil || !limit.Debt.Valid() {
				return fmt.Errorf("protectedFiles[%d] debt is invalid", index)
			}
		} else if limit.Debt != nil {
			return fmt.Errorf("protectedFiles[%d] has debt metadata without debt", index)
		}
	}
	registered := make(map[string]struct{}, len(registeredModules))
	for module := range registeredModules {
		registered[module] = struct{}{}
	}
	seenPackages := make(map[string]struct{})
	for index := range p.ModulePackages {
		item := &p.ModulePackages[index]
		item.Module = strings.TrimSpace(item.Module)
		item.Path = normalizePath(item.Path)
		item.Layer = strings.TrimSpace(item.Layer)
		key := item.Module + ":" + item.Path
		if _, ok := registered[item.Module]; !ok || item.Path == "" || filepath.IsAbs(item.Path) || hasWildcard(item.Path) || !validLayerName(item.Layer) {
			return fmt.Errorf("modulePackages[%d] is invalid", index)
		}
		if _, exists := seenPackages[key]; exists {
			return fmt.Errorf("modulePackages[%d] duplicates %q", index, key)
		}
		seenPackages[key] = struct{}{}
	}
	for index := range p.PublicModuleImports {
		item := &p.PublicModuleImports[index]
		item.Module = strings.TrimSpace(item.Module)
		item.Package = normalizePath(item.Package)
		item.Layer = strings.TrimSpace(item.Layer)
		item.Import = strings.TrimSpace(item.Import)
		consumerLayer, layerOK := policyPackageLayer(item.Module, item.Package, p.ModulePackages)
		if _, ok := registered[item.Module]; !ok || item.Package == "" || !validLayerName(item.Layer) || !layerOK || item.Layer != consumerLayer || !validPublicModuleTarget(item.Module, item.Import, registered) {
			return fmt.Errorf("publicModuleImports[%d] is invalid", index)
		}
	}
	for index := range p.PackageImports {
		item := &p.PackageImports[index]
		item.Module = strings.TrimSpace(item.Module)
		item.Package = normalizePath(item.Package)
		item.Import = strings.TrimSpace(item.Import)
		if _, ok := registered[item.Module]; !ok || item.Package == "" || filepath.IsAbs(item.Package) || hasWildcard(item.Package) || !validExactImport(item.Import) {
			return fmt.Errorf("packageImports[%d] is invalid", index)
		}
	}
	for index := range p.TestImports {
		item := &p.TestImports[index]
		item.Module = strings.TrimSpace(item.Module)
		item.Package = normalizePath(item.Package)
		item.Import = strings.TrimSpace(item.Import)
		layer, layerOK := policyPackageLayer(item.Module, item.Package, p.ModulePackages)
		if _, ok := registered[item.Module]; !ok || item.Package == "" || !validExactImport(item.Import) || !layerOK || (layer != "adapters" && layer != "transport" && layer != "module") {
			return fmt.Errorf("testImports[%d] is invalid", index)
		}
	}
	for index, pattern := range p.ForbiddenNewFiles {
		pattern = normalizePath(pattern)
		if pattern == "" || filepath.IsAbs(pattern) {
			return fmt.Errorf("forbiddenNewFiles[%d] is invalid", index)
		}
		if _, err := filepath.Match(filepath.FromSlash(pattern), ""); err != nil {
			return fmt.Errorf("forbiddenNewFiles[%d] is invalid: %w", index, err)
		}
		p.ForbiddenNewFiles[index] = pattern
	}
	for index := range p.Exceptions {
		exception := &p.Exceptions[index]
		exception.Path = normalizePath(exception.Path)
		if !exception.Valid() {
			return fmt.Errorf("exceptions[%d] is invalid", index)
		}
	}
	return nil
}

func validPublicModuleTarget(consumerModule, imported string, registered map[string]struct{}) bool {
	if !validExactImport(imported) {
		return false
	}
	const prefix = "jiyi/mochat-go/internal/modules/"
	if !strings.HasPrefix(imported, prefix) {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(imported, prefix), "/")
	if len(parts) == 0 || parts[0] == consumerModule {
		return false
	}
	if _, ok := registered[parts[0]]; !ok {
		return false
	}
	return len(parts) == 1 || (len(parts) == 2 && parts[1] == "ports")
}

func policyPackageLayer(module, packagePath string, declared []ModulePackage) (string, bool) {
	packagePath = normalizePath(packagePath)
	if packagePath == "." {
		return "module", true
	}
	first := strings.Split(packagePath, "/")[0]
	switch first {
	case "domain", "ports", "application", "adapters":
		return first, true
	case "transport":
		return "transport", true
	}
	for _, item := range declared {
		if item.Module == module && item.Path == packagePath {
			return item.Layer, true
		}
	}
	return "", false
}

// Valid reports whether protected debt has complete, bounded metadata.
func (d ProtectedDebt) Valid() bool {
	for _, value := range []string{d.Owner, d.Reason, d.Action, d.Baseline, d.CreatedOn, d.ExpiresOn} {
		if strings.TrimSpace(value) == "" {
			return false
		}
	}
	createdOn, err := time.Parse("2006-01-02", d.CreatedOn)
	if err != nil {
		return false
	}
	expiresOn, err := time.Parse("2006-01-02", d.ExpiresOn)
	return err == nil && !expiresOn.Before(createdOn) && expiresOn.Sub(createdOn) <= 90*24*time.Hour
}

func validLayerName(value string) bool {
	switch value {
	case "domain", "ports", "application", "adapters", "transport", "module":
		return true
	default:
		return false
	}
}

func validExactImport(value string) bool {
	return value != "" && !hasWildcard(value) && !strings.ContainsAny(value, `\\`)
}

func normalizeModuleList(label string, modules []string, registered map[string]string) error {
	for index, module := range modules {
		module = strings.TrimSpace(module)
		if module == "" || strings.ContainsAny(module, `/\\`) {
			return fmt.Errorf("%s[%d] is invalid", label, index)
		}
		if previous, exists := registered[module]; exists {
			return fmt.Errorf("%s[%d] duplicates module %q already registered in %s", label, index, module, previous)
		}
		registered[module] = label
		modules[index] = module
	}
	return nil
}

func hasWildcard(path string) bool {
	return strings.ContainsAny(path, "*?[]")
}

func normalizePath(path string) string {
	return normalizePortablePath(path)
}

func normalizePortablePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	path = strings.ReplaceAll(path, "\\", "/")
	path = filepath.ToSlash(filepath.Clean(path))
	return strings.TrimPrefix(path, "./")
}
