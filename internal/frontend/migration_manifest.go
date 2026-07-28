package frontend

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	urlpath "path"
	"path/filepath"
	"strings"
)

const LegacyDashboardMount = "/_legacy/dashboard/"

type MigrationRoute struct {
	Path        string  `json:"path"`
	Target      string  `json:"target"`
	Auth        bool    `json:"auth"`
	CorpContext bool    `json:"corpContext"`
	Permission  *string `json:"permission"`
}

type MigrationManifest struct {
	routes []MigrationRoute
}

func ParseMigrationManifest(raw []byte) (MigrationManifest, error) {
	var encodedRoutes []json.RawMessage
	if err := json.Unmarshal(raw, &encodedRoutes); err != nil {
		return MigrationManifest{}, fmt.Errorf("decode migration manifest: %w", err)
	}
	if encodedRoutes == nil {
		return MigrationManifest{}, fmt.Errorf("decode migration manifest: expected array")
	}
	routes := make([]MigrationRoute, 0, len(encodedRoutes))
	seen := make(map[string]struct{}, len(encodedRoutes))
	for index, encoded := range encodedRoutes {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(encoded, &fields); err != nil {
			return MigrationManifest{}, fmt.Errorf("decode migration route %d: %w", index, err)
		}
		for _, required := range []string{"path", "target", "auth", "corpContext", "permission"} {
			if _, exists := fields[required]; !exists {
				return MigrationManifest{}, fmt.Errorf("migration route %d missing %s", index, required)
			}
		}
		for _, booleanField := range []string{"auth", "corpContext"} {
			var value bool
			if string(fields[booleanField]) == "null" || json.Unmarshal(fields[booleanField], &value) != nil {
				return MigrationManifest{}, fmt.Errorf("migration route %d has invalid %s", index, booleanField)
			}
		}
		var route MigrationRoute
		if err := json.Unmarshal(encoded, &route); err != nil {
			return MigrationManifest{}, fmt.Errorf("decode migration route %d: %w", index, err)
		}
		canonical := canonicalRoutePath(route.Path)
		if canonical == "" || !strings.HasPrefix(route.Path, "/") {
			return MigrationManifest{}, fmt.Errorf("migration route %d must be absolute", index)
		}
		if canonical == strings.TrimSuffix(LegacyDashboardMount, "/") ||
			strings.HasPrefix(canonical, LegacyDashboardMount) {
			return MigrationManifest{}, fmt.Errorf("migration route %q conflicts with legacy mount", route.Path)
		}
		if route.Target != "react" && route.Target != "legacy" {
			return MigrationManifest{}, fmt.Errorf("migration route %q has unknown target %q", route.Path, route.Target)
		}
		if _, exists := seen[canonical]; exists {
			return MigrationManifest{}, fmt.Errorf("duplicate migration route %q", route.Path)
		}
		for _, previous := range routes {
			if routePatternsOverlap(previous.Path, canonical) {
				return MigrationManifest{}, fmt.Errorf("ambiguous migration route %q", route.Path)
			}
		}
		seen[canonical] = struct{}{}
		route.Path = canonical
		routes = append(routes, route)
	}
	return MigrationManifest{routes: routes}, nil
}

func routePatternsOverlap(left string, right string) bool {
	leftParts := strings.Split(strings.Trim(left, "/"), "/")
	rightParts := strings.Split(strings.Trim(right, "/"), "/")
	if len(leftParts) != len(rightParts) {
		return false
	}
	for index := range leftParts {
		if leftParts[index] == rightParts[index] ||
			strings.HasPrefix(leftParts[index], ":") ||
			strings.HasPrefix(rightParts[index], ":") {
			continue
		}
		return false
	}
	return true
}

func LoadMigrationManifest(path string) (MigrationManifest, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return MigrationManifest{}, fmt.Errorf("read migration manifest %s: %w", path, err)
	}
	manifest, err := ParseMigrationManifest(raw)
	if err != nil {
		return MigrationManifest{}, fmt.Errorf("validate migration manifest %s: %w", path, err)
	}
	return manifest, nil
}

func (m MigrationManifest) IsLegacyRoute(path string) bool {
	canonical := canonicalRoutePath(path)
	for _, route := range m.routes {
		if route.Target == "legacy" && routePatternMatches(route.Path, canonical) {
			return true
		}
	}
	return false
}

func canonicalRoutePath(path string) string {
	if path == "" || !strings.HasPrefix(path, "/") {
		return ""
	}
	clean := urlpath.Clean(path)
	if clean == "." {
		return ""
	}
	return clean
}

func routePatternMatches(pattern string, actual string) bool {
	patternParts := strings.Split(strings.Trim(pattern, "/"), "/")
	actualParts := strings.Split(strings.Trim(actual, "/"), "/")
	if len(patternParts) != len(actualParts) {
		return false
	}
	for index := range patternParts {
		if strings.HasPrefix(patternParts[index], ":") {
			if actualParts[index] == "" {
				return false
			}
			continue
		}
		if patternParts[index] != actualParts[index] {
			return false
		}
	}
	return true
}

type LegacyDashboardConfig struct {
	DistDir  string
	Manifest MigrationManifest
}

func WrapLegacyDashboard(next http.Handler, cfg LegacyDashboardConfig) http.Handler {
	distDir := cleanDistDir(cfg.DistDir)
	distAvailable := distDir != "." && distDir != "" && fileExists(filepath.Join(distDir, "index.html"))
	app := &appHandler{
		next:      http.NotFoundHandler(),
		distDir:   distDir,
		mountPath: LegacyDashboardMount,
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == strings.TrimSuffix(LegacyDashboardMount, "/") {
			http.NotFound(w, r)
			return
		}
		if !strings.HasPrefix(r.URL.Path, LegacyDashboardMount) {
			next.ServeHTTP(w, r)
			return
		}
		if !distAvailable {
			http.NotFound(w, r)
			return
		}
		originalPath := "/" + strings.TrimPrefix(r.URL.Path, LegacyDashboardMount)
		extension := strings.ToLower(filepath.Ext(originalPath))
		if extension == ".html" ||
			(extension == "" && !cfg.Manifest.IsLegacyRoute(originalPath)) ||
			(extension != "" && !fileExists(filepath.Join(distDir, filepath.FromSlash(strings.TrimPrefix(originalPath, "/"))))) {
			http.NotFound(w, r)
			return
		}
		app.ServeHTTP(w, r)
	})
}
