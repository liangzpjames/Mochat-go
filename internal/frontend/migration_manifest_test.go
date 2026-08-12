package frontend

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestMigrationManifestAcceptsValidRoutes(t *testing.T) {
	manifest, err := ParseMigrationManifest([]byte(`[
		{"path":"/workContact/index","target":"legacy","auth":true,"corpContext":true,"permission":null},
		{"path":"/corp/index","target":"react","auth":true,"corpContext":true,"permission":"corp.read"}
	]`))
	if err != nil {
		t.Fatal(err)
	}
	if !manifest.IsLegacyRoute("/workContact/index") {
		t.Fatal("legacy route was not registered")
	}
	if manifest.IsLegacyRoute("/corp/index") {
		t.Fatal("react route was registered as legacy")
	}
}

func TestMigrationManifestRejectsInvalidRoutes(t *testing.T) {
	tests := map[string]string{
		"duplicate":             `[{"path":"/same","target":"legacy","auth":true,"corpContext":true,"permission":null},{"path":"/same/","target":"react","auth":true,"corpContext":true,"permission":null}]`,
		"ambiguous pattern":     `[{"path":"/report/:id","target":"legacy","auth":true,"corpContext":true,"permission":null},{"path":"/report/:slug","target":"react","auth":true,"corpContext":true,"permission":null}]`,
		"unknown target":        `[{"path":"/route","target":"other","auth":true,"corpContext":true,"permission":null}]`,
		"relative path":         `[{"path":"route","target":"legacy","auth":true,"corpContext":true,"permission":null}]`,
		"legacy mount conflict": `[{"path":"/_legacy/dashboard/route","target":"legacy","auth":true,"corpContext":true,"permission":null}]`,
		"missing auth":          `[{"path":"/route","target":"legacy","corpContext":true,"permission":null}]`,
		"null manifest":         `null`,
		"null auth":             `[{"path":"/route","target":"legacy","auth":null,"corpContext":true,"permission":null}]`,
		"null corp context":     `[{"path":"/route","target":"legacy","auth":true,"corpContext":null,"permission":null}]`,
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseMigrationManifest([]byte(body)); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestMigrationManifestLegacyHandlerAllowsKnownRoutesAndAssets(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "index.html"), "legacy")
	writeFile(t, filepath.Join(dir, "js", "app.js"), "app")
	manifest, err := ParseMigrationManifest([]byte(`[
		{"path":"/workContact/index","target":"legacy","auth":true,"corpContext":true,"permission":null}
	]`))
	if err != nil {
		t.Fatal(err)
	}
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
	handler := WrapLegacyDashboard(next, LegacyDashboardConfig{
		DistDir:  dir,
		Manifest: manifest,
	})

	for _, path := range []string{
		"/_legacy/dashboard/workContact/index",
		"/_legacy/dashboard/js/app.js",
	} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d body=%q", path, rec.Code, rec.Body.String())
		}
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/_legacy/dashboard/not-known", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown status = %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/_legacy/dashboard/index.html", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("legacy shell status = %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/_legacy/dashboard", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("legacy mount without slash status = %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/dashboard/auth/session", nil))
	if rec.Code != http.StatusTeapot {
		t.Fatalf("api status = %d", rec.Code)
	}
}

func TestMigrationManifestLegacyHandlerFailsClosedWithoutDist(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
	handler := WrapLegacyDashboard(next, LegacyDashboardConfig{DistDir: t.TempDir()})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/_legacy/dashboard/workContact/index", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing dist status = %d", rec.Code)
	}
}

func TestMigrationManifestLoadsFromFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "migration-routes.json")
	if err := os.WriteFile(path, []byte(`[]`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadMigrationManifest(path); err != nil {
		t.Fatal(err)
	}
}

func TestMigrationManifestRepositorySourceHasExitedLegacy(t *testing.T) {
	manifest, err := LoadMigrationManifest(filepath.Join(
		"..", "..", "web", "apps", "dashboard", "src", "migration-routes.json",
	))
	if err != nil {
		t.Fatal(err)
	}
	if manifest.IsLegacyRoute("/workContact/index") {
		t.Fatal("repository manifest still registers a legacy route")
	}
}
