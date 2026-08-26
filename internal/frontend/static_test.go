package frontend

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWrapDashboardServesAssetsAndSPARoutes(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "index.html"), "index")
	writeFile(t, filepath.Join(dir, "js", "app.js"), "console.log('app')")
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("next"))
	})
	handler := WrapDashboard(next, DashboardConfig{DistDir: dir})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/login", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "index" {
		t.Fatalf("spa route = %d %q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/js/app.js", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "app") {
		t.Fatalf("asset = %d %q", rec.Code, rec.Body.String())
	}
}

func TestWrapDashboardRewritesLegacyExternalTenantClientToSameOrigin(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "index.html"), "index")
	writeFile(t, filepath.Join(dir, "js", "app.js"), `const external={baseURL:"//api.mo.chat"};`)
	handler := WrapDashboard(http.NotFoundHandler(), DashboardConfig{DistDir: dir})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/js/app.js", nil))
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `baseURL:"/dashboard"`) || strings.Contains(body, "api.mo.chat") {
		t.Fatalf("rewritten dashboard js = %d %q", rec.Code, body)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("Cache-Control = %q", got)
	}
}

func TestWrapDashboardUsesLocalFrameworkVendorsAndRemovesLegacyAnalytics(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "index.html"), `<div id="app"></div><script src="http://startupscrmmochat.duoduoker.com/cdn/vue@2.6.10distvue.min.js"></script><script src="http://startupscrmmochat.duoduoker.com/cdn/vue-router@3.1.3distvue-router.min.js"></script><script src="http://startupscrmmochat.duoduoker.com/cdn/vuex@3.1.1distvuex.min.js"></script><script src="http://startupscrmmochat.duoduoker.com/cdn/axios@0.19.0distaxios.min.js"></script><script>var _hmt = _hmt || [];
(function() { var hm = document.createElement("script"); hm.src = "//hm.baidu.com/hm.js?id"; })();</script>`)

	handler := WrapDashboard(http.NotFoundHandler(), DashboardConfig{DistDir: dir})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	body := rec.Body.String()
	for _, want := range []string{
		`src="/vendor/vue-2.6.10.min.js"`,
		`src="/vendor/vue-router-3.1.3.min.js"`,
		`src="/vendor/vuex-3.1.1.min.js"`,
		`src="/vendor/axios-0.19.0.min.js"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q: %s", want, body)
		}
	}
	if strings.Contains(body, "startupscrmmochat.duoduoker.com") || strings.Contains(body, "hm.baidu.com") || strings.Contains(body, "_hmt") {
		t.Fatalf("legacy external dependency leaked: %s", body)
	}
}

func TestWrapDashboardPassesAPIPathsToNext(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "index.html"), "index")
	handler := WrapDashboard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(r.URL.Path))
	}), DashboardConfig{DistDir: dir})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/dashboard/auth/session", nil))
	if rec.Code != http.StatusAccepted || rec.Body.String() != "/dashboard/auth/session" {
		t.Fatalf("api path = %d %q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/saas/v1/whoami", nil))
	if rec.Code != http.StatusAccepted || rec.Body.String() != "/api/saas/v1/whoami" {
		t.Fatalf("open api path = %d %q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/security/login", nil))
	if rec.Code != http.StatusAccepted || rec.Body.String() != "/security/login" {
		t.Fatalf("security page path = %d %q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusAccepted || rec.Body.String() != "/readyz" {
		t.Fatalf("readyz path = %d %q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/WW_verify_ABCDEF1234567890.txt", nil))
	if rec.Code != http.StatusAccepted || rec.Body.String() != "/WW_verify_ABCDEF1234567890.txt" {
		t.Fatalf("txt verify path = %d %q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/wecom/archive/callback?cid=4", nil))
	if rec.Code != http.StatusAccepted || rec.Body.String() != "/wecom/archive/callback" {
		t.Fatalf("archive callback path = %d %q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/Task/AutoTag/KeyWordTag", nil))
	if rec.Code != http.StatusAccepted || rec.Body.String() != "/Task/AutoTag/KeyWordTag" {
		t.Fatalf("task api path = %d %q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/roomTagPull/contactDetail?id=917001", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "index" {
		t.Fatalf("room tag pull contact detail path = %d %q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/roomTagPull/clientDetails?id=917001", nil))
	if rec.Code != http.StatusAccepted || rec.Body.String() != "/roomTagPull/clientDetails" {
		t.Fatalf("room tag pull client details path = %d %q", rec.Code, rec.Body.String())
	}
}

func TestWrapDashboardLeavesSaaSLoginAndAuthWithServerAndKeepsMountedSaaSDist(t *testing.T) {
	dashboardDir := t.TempDir()
	writeFile(t, filepath.Join(dashboardDir, "index.html"), "dashboard")
	saasDir := t.TempDir()
	writeFile(t, filepath.Join(saasDir, "index.html"), "saas")
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/saas/login" {
			http.Redirect(w, r, "/saas-admin/?login=1", http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(r.URL.Path))
	})
	handler := WrapDashboard(next, DashboardConfig{DistDir: dashboardDir})
	handler = WrapApp(handler, AppConfig{DistDir: saasDir, MountPath: "/saas-admin/"})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/saas/login", nil))
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/saas-admin/?login=1" {
		t.Fatalf("SaaS login route = %d location=%q body=%q", rec.Code, rec.Header().Get("Location"), rec.Body.String())
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/saas/auth/session", nil))
	if rec.Code != http.StatusAccepted || rec.Body.String() != "/saas/auth/session" {
		t.Fatalf("SaaS auth route = %d body=%q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/saas-admin/", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "saas" {
		t.Fatalf("mounted SaaS dist = %d body=%q", rec.Code, rec.Body.String())
	}
}

func TestWrapDashboardPassesShortLinkRedirectsToNext(t *testing.T) {

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "index.html"), "index")
	handler := WrapDashboard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusFound)
		_, _ = w.Write([]byte(r.URL.Path))
	}), DashboardConfig{DistDir: dir})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/r/abc123", nil))
	if rec.Code != http.StatusFound || rec.Body.String() != "/r/abc123" {
		t.Fatalf("short link redirect path = %d %q", rec.Code, rec.Body.String())
	}
}

func TestWrapDashboardIgnoresMissingDist(t *testing.T) {
	handler := WrapDashboard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}), DashboardConfig{DistDir: filepath.Join(t.TempDir(), "missing")})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/login", nil))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestWrapDashboardRejectsMissingAsset(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "index.html"), "index")
	handler := WrapDashboard(http.NotFoundHandler(), DashboardConfig{DistDir: dir})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/js/missing.js", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestWrapAppCanServeIndependentFrontendAssets(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "index.html"), "sidebar")
	writeFile(t, filepath.Join(dir, "css", "app.css"), "body{}")
	if !DistAvailable(dir) {
		t.Fatalf("DistAvailable = false")
	}
	handler := WrapApp(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(r.URL.Path))
	}), AppConfig{DistDir: dir})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/contact", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "sidebar" {
		t.Fatalf("spa route = %d %q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/css/app.css", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "body") {
		t.Fatalf("asset = %d %q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/sidebar/workContact/detail", nil))
	if rec.Code != http.StatusAccepted || rec.Body.String() != "/sidebar/workContact/detail" {
		t.Fatalf("api path = %d %q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/undefined/sidebar/workContact/detail", nil))
	if rec.Code != http.StatusAccepted || rec.Body.String() != "/undefined/sidebar/workContact/detail" {
		t.Fatalf("undefined sidebar api path = %d %q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/auth/workFission", nil))
	if rec.Code != http.StatusAccepted || rec.Body.String() != "/auth/workFission" {
		t.Fatalf("operation auth api path = %d %q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/openUserInfo/workFission", nil))
	if rec.Code != http.StatusAccepted || rec.Body.String() != "/openUserInfo/workFission" {
		t.Fatalf("operation user api path = %d %q", rec.Code, rec.Body.String())
	}
}

func TestWrapAppCanMountFrontendUnderPrefix(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "index.html"), `<!doctype html><link rel="icon" href="/favicon.ico"><link href="/css/app.css" rel="stylesheet"><script src="//res.wx.qq.com/open/js/jweixin.js"></script><script src="/js/app.js"></script><div id="app"></div>`)
	writeFile(t, filepath.Join(dir, "css", "app.css"), "body{}")
	writeFile(t, filepath.Join(dir, "js", "app.js"), `u.p="/";const sidebar=Object({NODE_ENV:"production",BASE_URL:"/"}).VUE_APP_API_BASE_URL+"/sidebar";const history=Object(f["b"])("/")`)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(r.URL.Path))
	})
	handler := WrapApp(next, AppConfig{DistDir: dir, MountPath: "/sidebar-app"})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/dashboard/auth/session", nil))
	if rec.Code != http.StatusAccepted || rec.Body.String() != "/dashboard/auth/session" {
		t.Fatalf("non-mounted path = %d %q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/sidebar-app/contact", nil))
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `href="/sidebar-app/css/app.css"`) || !strings.Contains(body, `src="/sidebar-app/js/app.js"`) || !strings.Contains(body, `src="//res.wx.qq.com/open/js/jweixin.js"`) {
		t.Fatalf("mounted html = %d %q", rec.Code, body)
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/sidebar-app/js/app.js", nil))
	body = rec.Body.String()
	for _, want := range []string{`.p="/sidebar-app/"`, `const sidebar="/sidebar"`, `Object(f["b"])("/sidebar-app/")`} {
		if !strings.Contains(body, want) {
			t.Fatalf("mounted js missing %q in %q", want, body)
		}
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/sidebar-app/css/app.css", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "body{}" {
		t.Fatalf("mounted asset = %d %q", rec.Code, rec.Body.String())
	}
}

func TestWrapAppRewritesViteAssetsUnderPrefix(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "index.html"), `<link rel="stylesheet" href="/assets/app.css"><script type="module" src="/assets/app.js"></script>`)
	handler := WrapApp(http.NotFoundHandler(), AppConfig{DistDir: dir, MountPath: "/sidebar-app"})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/sidebar-app/contact/remark", nil))
	body := rec.Body.String()
	for _, want := range []string{`href="/sidebar-app/assets/app.css"`, `src="/sidebar-app/assets/app.js"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("mounted Vite html missing %q in %q", want, body)
		}
	}
}

func TestWrapAppRewritesOperationRuntimeUnderPrefix(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "index.html"), `<script src="/js/app.js"></script><div id="app"></div>`)
	writeFile(t, filepath.Join(dir, "js", "app.js"), `o.p="/";const operation=Object({NODE_ENV:"production",BASE_URL:"/"}).VUE_APP_API_BASE_URL+"/operation";const router={mode:"history",base:"/",routes:[]}`)
	handler := WrapApp(http.NotFoundHandler(), AppConfig{DistDir: dir, MountPath: "operation-app"})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/operation-app/js/app.js", nil))
	body := rec.Body.String()
	for _, want := range []string{`.p="/operation-app/"`, `const operation="/operation"`, `base:"/operation-app/"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("mounted operation js missing %q in %q", want, body)
		}
	}
}

func writeFile(t *testing.T, path string, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
