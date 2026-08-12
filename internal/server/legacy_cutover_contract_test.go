package server

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

func task12RepositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate legacy cutover contract source")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func TestLegacyAuthAndCorpRoutesArePhysicallyRemoved(t *testing.T) {
	root := task12RepositoryRoot(t)
	forbiddenByFile := map[string][]string{
		filepath.Join(root, "internal", "server", "server.go"): {
			"/security/login",
			"/dashboard/user/loginShow",
			"/dashboard/corp/select",
			"/dashboard/corp/bind",
			"/dashboard/corp/store",
			"identityLoginPage",
			"corpSelect",
			"corpBind",
		},
		filepath.Join(root, "cmd", "mochat-go", "main.go"): {
			"MigrateLoginShow",
			"WithLoginShowHandler",
			"MigrateCorpSelect",
			"WithCorpSelectHandler",
			"WithCorpBindHandler",
			"NewLogoutHandler",
			"WithLogoutHandler",
			"WithDashboardRequestGuard(dashboardAccessGuard)",
			"/dashboard/corp/select",
			"/dashboard/corp/bind",
			"/dashboard/corp/store",
		},
		filepath.Join(root, "internal", "dashboard", "dashboard_route_policy.go"): {
			"GET /dashboard/user/loginShow",
			"GET /dashboard/corp/select",
			"POST /dashboard/corp/bind",
			"POST /dashboard/corp/store",
		},
		filepath.Join(root, "internal", "config", "config.go"): {
			"MigrateLoginShow",
			"MOCHAT_GO_MIGRATE_LOGIN_SHOW",
			"MigrateCorpSelect",
			"MigrateCorpBind",
			"MigrateCorpIndex",
			"MigrateCorpShow",
			"MigrateCorpStore",
			"MigrateCorpUpdate",
			"MOCHAT_GO_MIGRATE_CORP_SELECT",
			"MOCHAT_GO_MIGRATE_CORP_BIND",
			"MOCHAT_GO_MIGRATE_CORP_INDEX",
			"MOCHAT_GO_MIGRATE_CORP_SHOW",
			"MOCHAT_GO_MIGRATE_CORP_STORE",
			"MOCHAT_GO_MIGRATE_CORP_UPDATE",
		},
	}
	for path, forbidden := range forbiddenByFile {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := string(body)
		for _, token := range forbidden {
			if strings.Contains(text, token) {
				t.Fatalf("legacy production contract %q remains in %s", token, path)
			}
		}
	}
	for _, path := range []string{
		filepath.Join(root, "internal", "dashboard", "identity_login_page.go"),
		filepath.Join(root, "internal", "dashboard", "identity_login_page_test.go"),
		filepath.Join(root, "internal", "dashboard", "login_show.go"),
		filepath.Join(root, "internal", "dashboard", "login_show_test.go"),
		filepath.Join(root, "web", "legacy", "dashboard", "src", "api", "login.js"),
		filepath.Join(root, "web", "legacy", "dashboard", "src", "views", "corp", "index.vue"),
	} {
		if _, err := os.Stat(path); err == nil {
			t.Fatalf("obsolete identity login file still exists: %s", path)
		} else if !os.IsNotExist(err) {
			t.Fatalf("stat obsolete identity login file %s: %v", path, err)
		}
	}
}

func TestCompanyCatalogAndRBACSeedUseOnlySingleCompanyEndpoints(t *testing.T) {
	root := task12RepositoryRoot(t)
	legacy := []string{
		"/dashboard/corp/select",
		"/dashboard/corp/bind",
		"/dashboard/corp/index",
		"/dashboard/corp/show",
		"/dashboard/corp/store",
		"/dashboard/corp/update",
	}
	paths := []string{
		filepath.Join(root, "internal", "dashboard", "dashboard_page_catalog.json"),
		filepath.Join(root, "deploy", "standalone", "migrations", "0002_seed_core_data.up.sql"),
		filepath.Join(root, "internal", "server", "compat_manifest_embedded.json"),
	}
	for _, path := range paths {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := string(body)
		for _, token := range legacy {
			if strings.Contains(text, token) {
				t.Fatalf("legacy company route %q remains in source-of-truth %s", token, path)
			}
		}
	}

	catalog, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	catalogText := string(catalog)
	for _, required := range []string{
		`"code": "dashboard.company_setting.website"`,
		`"superadminOnly": true`,
		`"pathPattern": "/dashboard/company/profile"`,
		`"pathPattern": "/dashboard/company/verify"`,
		`"pathPattern": "/dashboard/company/employee-sync"`,
		`"pathPattern": "/dashboard/company/sync-status"`,
		`"pathPattern": "/dashboard/company/wecom-credentials"`,
		`"pathPattern": "/dashboard/company/agent-credentials"`,
		`"pathPattern": "/dashboard/company/archive-credentials"`,
		`"pathPattern": "/dashboard/company/audits"`,
	} {
		if !strings.Contains(catalogText, required) {
			t.Fatalf("company catalog missing %q", required)
		}
	}

	seed, err := os.ReadFile(filepath.Join(root, "deploy", "standalone", "migrations", "0131_identity_realms_single_corp_cutover.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	seedText := string(seed)
	for _, required := range []string{
		"mochat_go_dashboard_permission_resources",
		"UPDATE mochat_go_dashboard_permissions",
		"superadmin_only",
		"DELETE",
		"/dashboard/corp/index",
		"/dashboard/corp/store",
		"'/dashboard/company/profile'",
		"'/dashboard/company/verify'",
		"'/dashboard/company/employee-sync'",
		"'/dashboard/company/sync-status'",
		"'/dashboard/company/wecom-credentials'",
		"'/dashboard/company/agent-credentials'",
		"'/dashboard/company/archive-credentials'",
		"'/dashboard/company/audits'",
	} {
		if !strings.Contains(seedText, required) {
			t.Fatalf("RBAC seed missing %q", required)
		}
	}
}

func TestActiveAcceptanceEntrypointsDoNotInvokeRetiredIdentitySmokes(t *testing.T) {
	root := task12RepositoryRoot(t)
	acceptancePath := filepath.Join(root, "scripts", "standalone_acceptance.sh")
	acceptanceBytes, err := os.ReadFile(acceptancePath)
	if err != nil {
		t.Fatal(err)
	}
	references := regexp.MustCompile(`(?:\./scripts/)?(smoke_[A-Za-z0-9_]+\.sh)`).FindAllStringSubmatch(string(acceptanceBytes), -1)
	for _, reference := range references {
		if len(reference) < 2 {
			continue
		}
		path := filepath.Join(root, "scripts", reference[1])
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("active acceptance smoke %s is missing: %v", reference[1], err)
		}
		text := string(body)
		for _, forbidden := range []string{
			"mochat-bootstrap",
			"/security/login",
			"/dashboard/corp/",
			" -secret",
		} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("active acceptance smoke %s still invokes retired identity contract %q", reference[1], forbidden)
			}
		}
	}
}
