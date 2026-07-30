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
	Path     string `json:"path"`
	MaxBytes int64  `json:"maxBytes"`
}

// Policy describes repository architecture constraints.
type Policy struct {
	ProductionModules []string    `json:"productionModules"`
	ExampleModules    []string    `json:"exampleModules"`
	ProtectedFiles    []SizeLimit `json:"protectedFiles"`
	ForbiddenNewFiles []string    `json:"forbiddenNewFiles"`
	Exceptions        []Exception `json:"exceptions"`
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
		if limit.Path == "" || filepath.IsAbs(limit.Path) || hasWildcard(limit.Path) || limit.MaxBytes < 0 {
			return fmt.Errorf("protectedFiles[%d] is invalid", index)
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
