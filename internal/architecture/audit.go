package architecture

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type layer int

const (
	layerUnknown layer = iota
	layerDomain
	layerPorts
	layerApplication
	layerAdapters
	layerTransport
	layerModule
)

type finding struct {
	Violation
	importPath string
}

// Audit evaluates root against policy and returns stable, sorted violations.
func Audit(root string, policy Policy, now time.Time) ([]Violation, error) {
	root = filepath.Clean(root)
	findings := make([]finding, 0)

	for _, limit := range policy.ProtectedFiles {
		contents, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(limit.Path)))
		if err != nil {
			if !os.IsNotExist(err) {
				return nil, err
			}
			continue
		}
		size := int64(len(bytes.ReplaceAll(contents, []byte("\r\n"), []byte("\n"))))
		if size > limit.MaxBytes {
			findings = append(findings, finding{Violation: Violation{
				RuleID: RuleProtectedFileSize,
				Path:   limit.Path,
				Detail: fmt.Sprintf("%d bytes exceeds %d-byte limit", size, limit.MaxBytes),
			}})
		}
	}

	auditRootIncludesTestdata := pathContainsSegment(root, "testdata")
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == "testdata" && !auditRootIncludesTestdata {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}

		relativePath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relativePath = normalizePath(relativePath)
		for _, pattern := range policy.ForbiddenNewFiles {
			matched, err := filepath.Match(filepath.FromSlash(pattern), filepath.FromSlash(relativePath))
			if err != nil {
				return fmt.Errorf("invalid forbidden file pattern %q: %w", pattern, err)
			}
			if matched {
				findings = append(findings, finding{Violation: Violation{
					RuleID: RuleForbiddenLegacyFile,
					Path:   relativePath,
					Detail: fmt.Sprintf("matches forbidden file pattern %q", pattern),
				}})
			}
		}

		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return fmt.Errorf("parse %s: %w", relativePath, err)
		}
		currentModule, currentLayer := classify(relativePath)
		if currentLayer == layerUnknown {
			return nil
		}
		for _, importSpec := range file.Imports {
			importPath, err := importPath(importSpec)
			if err != nil {
				return fmt.Errorf("parse import in %s: %w", relativePath, err)
			}
			findings = append(findings, dependencyFindings(relativePath, currentModule, currentLayer, importPath)...)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	findings = applyExceptions(findings, policy.Exceptions, now)
	sort.Slice(findings, func(left, right int) bool {
		if findings[left].RuleID != findings[right].RuleID {
			return findings[left].RuleID < findings[right].RuleID
		}
		if findings[left].Path != findings[right].Path {
			return findings[left].Path < findings[right].Path
		}
		return findings[left].Detail < findings[right].Detail
	})

	violations := make([]Violation, len(findings))
	for index, item := range findings {
		violations[index] = item.Violation
	}
	return violations, nil
}

func pathContainsSegment(path, segment string) bool {
	for _, part := range strings.Split(normalizePath(path), "/") {
		if part == segment {
			return true
		}
	}
	return false
}

func dependencyFindings(path, currentModule string, currentLayer layer, imported string) []finding {
	findings := make([]finding, 0, 2)
	add := func(ruleID string) {
		findings = append(findings, finding{Violation: Violation{
			RuleID: ruleID,
			Path:   path,
			Detail: fmt.Sprintf("imports %q", imported),
		}, importPath: imported})
	}

	importedModule, importedLayer := classifyModuleImport(imported)
	if importedModule != "" && importedModule != currentModule && (importedLayer == layerAdapters || importedLayer == layerTransport) {
		add(RuleCrossModulePrivate)
	}
	if isLegacyImport(imported) {
		add(RuleLegacyDependency)
	}
	if !forbiddenByLayer(currentLayer, currentModule, imported, importedModule, importedLayer) {
		return findings
	}

	switch currentLayer {
	case layerDomain:
		add(RuleDomainDependency)
	case layerPorts:
		add(RulePortsDependency)
	case layerApplication:
		add(RuleApplicationDependency)
	case layerAdapters:
		add(RuleAdaptersDependency)
	case layerTransport:
		add(RuleTransportDependency)
	}
	return findings
}

func forbiddenByLayer(currentLayer layer, currentModule, imported, importedModule string, importedLayer layer) bool {
	if isLegacyImport(imported) {
		return false
	}
	switch currentLayer {
	case layerDomain:
		return imported == "database/sql" || imported == "net/http" || importedLayer == layerApplication || importedLayer == layerAdapters || importedLayer == layerTransport
	case layerPorts:
		return imported == "database/sql" || imported == "net/http" || importedLayer == layerApplication || importedLayer == layerAdapters || importedLayer == layerTransport
	case layerApplication:
		return imported == "database/sql" || imported == "net/http" || importedLayer == layerAdapters || importedLayer == layerTransport
	case layerAdapters:
		return importedLayer == layerTransport
	case layerTransport:
		return imported == "database/sql" || importedLayer == layerAdapters
	default:
		return false
	}
}

func classify(path string) (string, layer) {
	parts := strings.Split(normalizePath(path), "/")
	for index := 0; index+2 < len(parts); index++ {
		if parts[index] != "internal" || parts[index+1] != "modules" {
			continue
		}
		module := parts[index+2]
		if module == "" {
			return "", layerUnknown
		}
		if index+3 == len(parts) {
			return module, layerModule
		}
		switch parts[index+3] {
		case "domain":
			return module, layerDomain
		case "ports":
			return module, layerPorts
		case "application":
			return module, layerApplication
		case "adapters":
			return module, layerAdapters
		case "transport":
			return module, layerTransport
		default:
			return module, layerModule
		}
	}
	return "", layerUnknown
}

func classifyModuleImport(imported string) (string, layer) {
	const modulePrefix = "jiyi/mochat-go/internal/modules/"
	if !strings.HasPrefix(imported, modulePrefix) {
		return "", layerUnknown
	}
	parts := strings.Split(strings.TrimPrefix(imported, modulePrefix), "/")
	if len(parts) == 0 || parts[0] == "" {
		return "", layerUnknown
	}
	if len(parts) == 1 {
		return parts[0], layerModule
	}
	switch parts[1] {
	case "domain":
		return parts[0], layerDomain
	case "ports":
		return parts[0], layerPorts
	case "application":
		return parts[0], layerApplication
	case "adapters":
		return parts[0], layerAdapters
	case "transport":
		return parts[0], layerTransport
	default:
		return parts[0], layerModule
	}
}

func isLegacyImport(imported string) bool {
	for _, packagePath := range []string{
		"jiyi/mochat-go/internal/dashboard",
		"jiyi/mochat-go/internal/store",
		"jiyi/mochat-go/internal/server",
		"jiyi/mochat-go/internal/config",
	} {
		if imported == packagePath || strings.HasPrefix(imported, packagePath+"/") {
			return true
		}
	}
	return false
}

func importPath(spec *ast.ImportSpec) (string, error) {
	return strconv.Unquote(spec.Path.Value)
}

func applyExceptions(findings []finding, exceptions []Exception, now time.Time) []finding {
	kept := make([]finding, 0, len(findings))
	for _, item := range findings {
		matched := false
		for _, exception := range exceptions {
			if !exception.Valid() || exception.RuleID != item.RuleID || exception.Path != item.Path || exception.Import != item.importPath {
				continue
			}
			matched = true
			if exceptionExpired(exception, now) {
				kept = append(kept, finding{Violation: Violation{
					RuleID: RuleExceptionExpired,
					Path:   item.Path,
					Detail: fmt.Sprintf("exception for %q expired on %s", item.importPath, exception.ExpiresOn),
				}})
				break
			}
			break
		}
		if !matched || (matched && exceptionMatchExpired(exceptions, item, now)) {
			kept = append(kept, item)
		}
	}
	return kept
}

func exceptionMatchExpired(exceptions []Exception, item finding, now time.Time) bool {
	for _, exception := range exceptions {
		if exception.Valid() && exception.RuleID == item.RuleID && exception.Path == item.Path && exception.Import == item.importPath {
			return exceptionExpired(exception, now)
		}
	}
	return false
}

func exceptionExpired(exception Exception, now time.Time) bool {
	expiresOn, err := time.Parse("2006-01-02", exception.ExpiresOn)
	if err != nil {
		return false
	}
	today, err := time.Parse("2006-01-02", now.UTC().Format("2006-01-02"))
	return err == nil && today.After(expiresOn)
}
