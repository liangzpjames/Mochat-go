package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"jiyi/mochat-go/internal/app/modules"
	"jiyi/mochat-go/internal/config"
	"jiyi/mochat-go/internal/taskrunner"
)

func TestRootKeepsMoChatCompatibility(t *testing.T) {
	srv := newTestServer(t, "")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "Hello MoChat " {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestFriendsCircleProviderRoutes(t *testing.T) {
	handler := func(body string) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) })
	}
	srv, err := New(config.Config{},
		WithFriendsCircleTaskIndexHandler(handler("task-index")),
		WithFriendsCircleMaterialIndexHandler(handler("material-index")),
		WithFriendsCircleTaskStoreHandler(handler("task-store")),
		WithFriendsCircleMaterialStoreHandler(handler("material-store")),
		WithFriendsCirclePublishHandler(handler("publish")),
		WithFriendsCircleTaskResultIndexHandler(handler("task-result-index")),
		WithFriendsCircleExportHandler(handler("export")),
		WithFriendsCircleExportDataHandler(handler("export-data")),
		WithFriendsCircleProviderCallbackHandler(handler("callback")),
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodGet, "/dashboard/friendsCircle/taskIndex", "task-index"},
		{http.MethodGet, "/dashboard/friendsCircle/materialIndex", "material-index"},
		{http.MethodPost, "/dashboard/friendsCircle/taskStore", "task-store"},
		{http.MethodPost, "/dashboard/friendsCircle/materialStore", "material-store"},
		{http.MethodPost, "/dashboard/friendsCircle/publish", "publish"},
		{http.MethodGet, "/dashboard/friendsCircle/taskResultIndex", "task-result-index"},
		{http.MethodGet, "/dashboard/friendsCircle/export", "export"},
		{http.MethodGet, "/dashboard/friendsCircle/exportData", "export-data"},
		{http.MethodPost, "/dashboard/friendsCircle/providerCallback", "callback"},
	} {
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
		if rec.Code != http.StatusOK || rec.Body.String() != tc.body {
			t.Fatalf("%s %s: status=%d body=%q", tc.method, tc.path, rec.Code, rec.Body.String())
		}
	}
}

func TestMaterialFoundationProviderRoutes(t *testing.T) {
	handler := func(body string) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) })
	}
	srv, err := New(config.Config{},
		WithMediumBatchGroupUpdateHandler(handler("batch-move")),
		WithMediumReferenceCheckHandler(handler("reference-check")),
		WithMediumBatchDestroyHandler(handler("batch-destroy")),
		WithMaterialSelectorIndexHandler(handler("selector")),
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodPost, "/dashboard/medium/batchGroupUpdate", "batch-move"},
		{http.MethodPost, "/dashboard/medium/referenceCheck", "reference-check"},
		{http.MethodPost, "/dashboard/medium/batchDestroy", "batch-destroy"},
		{http.MethodGet, "/dashboard/materialSelector/index", "selector"},
	} {
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
		if rec.Code != http.StatusOK || rec.Body.String() != tc.body {
			t.Fatalf("%s %s: status=%d body=%q", tc.method, tc.path, rec.Code, rec.Body.String())
		}
	}
}

func TestPhase34AcquisitionProviderRoutes(t *testing.T) {
	handler := func(body string) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) })
	}
	srv, err := New(config.Config{},
		WithPhase34AcquisitionLinkIndexHandler(handler("acquisition-index")),
		WithPhase34AcquisitionLinkStoreHandler(handler("acquisition-store")),
		WithPhase34AcquisitionLinkAuthorizeHandler(handler("acquisition-authorize")),
		WithPhase34CustomerServiceIndexHandler(handler("customer-index")),
		WithPhase34CustomerServiceStoreHandler(handler("customer-store")),
		WithPhase34CustomerServiceSyncHandler(handler("customer-sync")),
		WithPhase34ShortLinkIndexHandler(handler("short-index")),
		WithPhase34ShortLinkStoreHandler(handler("short-store")),
		WithPhase34ShortLinkDisableHandler(handler("short-disable")),
		WithPhase34ShortLinkRedirectHandler(handler("short-redirect")),
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodGet, "/dashboard/acquisitionLink/index", "acquisition-index"},
		{http.MethodPost, "/dashboard/acquisitionLink/store", "acquisition-store"},
		{http.MethodPost, "/dashboard/acquisitionLink/authorize", "acquisition-authorize"},
		{http.MethodGet, "/dashboard/customerService/index", "customer-index"},
		{http.MethodPost, "/dashboard/customerService/store", "customer-store"},
		{http.MethodPost, "/dashboard/customerService/sync", "customer-sync"},
		{http.MethodGet, "/dashboard/liveCodeShortChain/index", "short-index"},
		{http.MethodPost, "/dashboard/liveCodeShortChain/store", "short-store"},
		{http.MethodPost, "/dashboard/liveCodeShortChain/disable", "short-disable"},
		{http.MethodGet, "/r/abc123", "short-redirect"},
	} {
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
		if rec.Code != http.StatusOK || rec.Body.String() != tc.body {
			t.Fatalf("%s %s: status=%d body=%q", tc.method, tc.path, rec.Code, rec.Body.String())
		}
	}
}

func TestHeadRootDoesNotWriteBody(t *testing.T) {
	srv := newTestServer(t, "")

	req := httptest.NewRequest(http.MethodHead, "/", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("HEAD body length = %d, want 0", rec.Body.Len())
	}
}

func TestModuleRouterRunsBeforeLegacyFallback(t *testing.T) {
	router := modules.NewRouter()
	if err := router.Handle(http.MethodGet, "/api/phase2-2/ping", http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) },
	)); err != nil {
		t.Fatal(err)
	}
	server, err := New(config.Config{}, WithModuleRouter(router))
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/phase2-2/ping", nil))
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
}

func TestUnmatchedModuleRouteKeepsLegacyHealthRoute(t *testing.T) {
	server, err := New(config.Config{}, WithModuleRouter(modules.NewRouter()))
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestWithModuleRouterRejectsTypedNilRouter(t *testing.T) {
	var router *fixedModuleRouter
	server, err := New(config.Config{}, WithModuleRouter(router))
	if err != nil {
		t.Fatal(err)
	}
	if server.moduleRouter != nil {
		t.Fatal("typed-nil module router was retained")
	}
}

func TestModuleRouterTypedNilHandlerFallsBackSafely(t *testing.T) {
	var handler *panicHandler
	server, err := New(config.Config{}, WithModuleRouter(&fixedModuleRouter{
		handler: handler,
		matched: true,
	}))
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
}

type fixedModuleRouter struct {
	handler http.Handler
	matched bool
}

func (r *fixedModuleRouter) Match(*http.Request) (http.Handler, bool) {
	return r.handler, r.matched
}

type panicHandler struct{}

func (*panicHandler) ServeHTTP(http.ResponseWriter, *http.Request) {
	panic("typed-nil handler must never be dispatched")
}

type recordingDashboardRequestGuard struct {
	allow bool
	calls int
	paths []string
}

func (guard *recordingDashboardRequestGuard) Authorize(w http.ResponseWriter, request *http.Request) bool {
	guard.calls++
	guard.paths = append(guard.paths, request.URL.Path)
	if !guard.allow {
		http.Error(w, "guard denied", http.StatusForbidden)
	}
	return guard.allow
}

func TestDashboardRequestGuardRunsBeforeModuleRouterAndLegacySwitch(t *testing.T) {
	for _, test := range []struct {
		name    string
		path    string
		options func(*testing.T, http.Handler) []Option
	}{
		{
			name: "module router", path: "/dashboard/reports/overview",
			options: func(t *testing.T, handler http.Handler) []Option {
				router := modules.NewRouter()
				if err := router.Handle(http.MethodGet, "/dashboard/reports/overview", handler); err != nil {
					t.Fatal(err)
				}
				return []Option{WithModuleRouter(router)}
			},
		},
		{
			name: "legacy switch", path: "/dashboard/corpData/index",
			options: func(_ *testing.T, handler http.Handler) []Option {
				return []Option{WithCorpDataIndexHandler(handler)}
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			guard := &recordingDashboardRequestGuard{}
			handlerCalled := false
			handler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { handlerCalled = true })
			options := append(test.options(t, handler), WithDashboardRequestGuard(guard))
			server, err := New(config.Config{}, options...)
			if err != nil {
				t.Fatal(err)
			}
			response := httptest.NewRecorder()
			server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, test.path, nil))
			if response.Code != http.StatusForbidden || handlerCalled || guard.calls != 1 {
				t.Fatalf("status=%d handlerCalled=%v guardCalls=%d", response.Code, handlerCalled, guard.calls)
			}
		})
	}
}

func TestDashboardAccessHandlerRunsAfterGuardAndBeforeOtherModuleRoutes(t *testing.T) {
	router := modules.NewRouter()
	registered := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	if err := router.Handle(http.MethodGet, "/dashboard/access/profile", registered); err != nil {
		t.Fatal(err)
	}
	fallbackCalls := 0
	fallback := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fallbackCalls++
		w.WriteHeader(http.StatusMethodNotAllowed)
	})
	guard := &recordingDashboardRequestGuard{allow: true}
	server, err := New(config.Config{}, WithDashboardRequestGuard(guard), WithDashboardAccessHandler(fallback), WithModuleRouter(router))
	if err != nil {
		t.Fatal(err)
	}

	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/dashboard/access/profile", nil))
	if response.Code != http.StatusCreated || guard.calls != 1 || fallbackCalls != 0 {
		t.Fatalf("registered status=%d guard=%d fallback=%d", response.Code, guard.calls, fallbackCalls)
	}
	response = httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/dashboard/access/profile", nil))
	if response.Code != http.StatusMethodNotAllowed || guard.calls != 2 || fallbackCalls != 1 {
		t.Fatalf("fallback status=%d guard=%d fallback=%d", response.Code, guard.calls, fallbackCalls)
	}

	deniedGuard := &recordingDashboardRequestGuard{}
	server, err = New(config.Config{}, WithDashboardRequestGuard(deniedGuard), WithDashboardAccessHandler(fallback), WithModuleRouter(router))
	if err != nil {
		t.Fatal(err)
	}
	response = httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/dashboard/access/profile", nil))
	if response.Code != http.StatusForbidden || deniedGuard.calls != 1 || fallbackCalls != 1 {
		t.Fatalf("denied status=%d guard=%d fallback=%d", response.Code, deniedGuard.calls, fallbackCalls)
	}
}

func TestDashboardRequestGuardSkipsSaaSAndRunsOnceAfterBundledNormalization(t *testing.T) {
	t.Run("saas admin bypass", func(t *testing.T) {
		guard := &recordingDashboardRequestGuard{}
		server, err := New(config.Config{},
			WithDashboardRequestGuard(guard),
			WithSaaSAdminOverviewHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })),
		)
		if err != nil {
			t.Fatal(err)
		}
		response := httptest.NewRecorder()
		server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/overview", nil))
		if response.Code != http.StatusNoContent || guard.calls != 0 {
			t.Fatalf("status=%d guardCalls=%d", response.Code, guard.calls)
		}
	})

	t.Run("normalized dashboard request", func(t *testing.T) {
		guard := &recordingDashboardRequestGuard{allow: true}
		server, err := New(config.Config{},
			WithDashboardRequestGuard(guard),
			WithCorpDataIndexHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })),
		)
		if err != nil {
			t.Fatal(err)
		}
		response := httptest.NewRecorder()
		server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/undefined/dashboard/corpData/index", nil))
		if response.Code != http.StatusNoContent || guard.calls != 1 || len(guard.paths) != 1 || guard.paths[0] != "/dashboard/corpData/index" {
			t.Fatalf("status=%d calls=%d paths=%v", response.Code, guard.calls, guard.paths)
		}
	})
}

func TestUnknownRouteFallsBackToPHP(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/dashboard/user/loginShow" {
			t.Fatalf("unexpected upstream path: %s", r.URL.Path)
		}
		if got := r.Header.Get("X-Mochat-Go-Compat"); got != "fallback-proxy" {
			t.Fatalf("compat header = %q", got)
		}
		w.Header().Set("X-Upstream", "php")
		_, _ = w.Write([]byte("proxied"))
	}))
	defer upstream.Close()

	srv := newTestServer(t, upstream.URL)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/user/loginShow", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Header().Get("X-Upstream") != "php" {
		t.Fatalf("missing upstream header")
	}
	if rec.Header().Get("X-Mochat-Go-Compat") != "fallback-proxy" {
		t.Fatalf("missing compat fallback response header")
	}
	if rec.Body.String() != "proxied" {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestUnknownRouteWithoutPHPFallbackReturnsBadGateway(t *testing.T) {
	srv := newTestServer(t, "")

	req := httptest.NewRequest(http.MethodGet, "/dashboard/user/loginShow", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadGateway)
	}
	if rec.Header().Get("X-Mochat-Go-Compat") != "" {
		t.Fatalf("unexpected fallback header = %q", rec.Header().Get("X-Mochat-Go-Compat"))
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["message"] != "no PHP upstream configured for unmigrated route" {
		t.Fatalf("payload = %+v", payload)
	}
}

func TestIdentityLoginPageHandlerTakesPrecedenceOverFallback(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("identity login page unexpectedly reached fallback: %s", r.URL.Path)
	}))
	defer upstream.Close()

	srv, err := New(config.Config{
		ListenAddr:   ":0",
		PHPUpstream:  upstream.URL,
		SourceRoot:   t.TempDir(),
		ManifestPath: writeManifest(t),
		ProxyTimeout: time.Second,
	}, WithIdentityLoginPageHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<form id="password-form"></form>`))
	})))
	if err != nil {
		t.Fatal(err)
	}

	for _, method := range []string{http.MethodGet, http.MethodHead} {
		req := httptest.NewRequest(method, "/security/login", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want %d", method, rec.Code, http.StatusOK)
		}
		if method == http.MethodGet && !strings.Contains(rec.Body.String(), `id="password-form"`) {
			t.Fatalf("GET body = %q", rec.Body.String())
		}
	}
}

func TestReadyzReportsSourceAndProxyState(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("Hello MoChat "))
	}))
	defer upstream.Close()

	sourceRoot := t.TempDir()
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		PHPUpstream:  upstream.URL,
		SourceRoot:   sourceRoot,
		ManifestPath: writeManifest(t),
		ProxyTimeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var payload statusPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if !payload.SourceRootExists {
		t.Fatalf("SourceRootExists = false")
	}
	if !payload.ManifestExists {
		t.Fatalf("ManifestExists = false")
	}
	if !payload.ProxyFallbackEnabled {
		t.Fatalf("ProxyFallbackEnabled = false")
	}
	if !payload.PHPUpstreamReady {
		t.Fatalf("PHPUpstreamReady = false, probe=%s", payload.PHPUpstreamProbe)
	}
	if payload.NextMigrationBoundary != "auth/tenant/rbac" {
		t.Fatalf("NextMigrationBoundary = %q", payload.NextMigrationBoundary)
	}
}

func TestReadyzRejectsWrongUpstream(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not mochat"))
	}))
	defer upstream.Close()

	sourceRoot := t.TempDir()
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		PHPUpstream:  upstream.URL,
		SourceRoot:   sourceRoot,
		ManifestPath: writeManifest(t),
		ProxyTimeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusServiceUnavailable, rec.Body.String())
	}
}

func TestReadyzStandaloneDoesNotRequireSourceManifestOrPHP(t *testing.T) {
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		Standalone:   true,
		ProxyTimeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var payload statusPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if !payload.Standalone || payload.Mode != "standalone-go" {
		t.Fatalf("standalone payload = %+v", payload)
	}
	if payload.SourceRootExists {
		t.Fatalf("SourceRootExists = true")
	}
	if !payload.ManifestExists {
		t.Fatalf("ManifestExists = false")
	}
	if payload.ProxyFallbackEnabled {
		t.Fatalf("ProxyFallbackEnabled = true")
	}
	if payload.PHPUpstreamReady {
		t.Fatalf("PHPUpstreamReady = true")
	}
}

func TestCompatStatusReportsBackgroundTasks(t *testing.T) {
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		Standalone:   true,
		ProxyTimeout: time.Second,
	}, WithBackgroundTasks(func() []taskrunner.Snapshot {
		return []taskrunner.Snapshot{{Name: "employee-apply", Status: taskrunner.StatusRunning}}
	}))
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/compat/status", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var payload statusPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.BackgroundTasks) != 1 || payload.BackgroundTasks[0].Name != "employee-apply" || payload.BackgroundTasks[0].Status != taskrunner.StatusRunning {
		t.Fatalf("background tasks = %+v", payload.BackgroundTasks)
	}
}

func TestCompatRoutesReturnsManifestWithMigratedRoutes(t *testing.T) {
	sourceRoot := t.TempDir()
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		PHPUpstream:  "http://127.0.0.1:9501",
		SourceRoot:   sourceRoot,
		ManifestPath: writeManifest(t),
		ProxyTimeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/compat/routes", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["source_revision"] != "test-revision" {
		t.Fatalf("source_revision = %v", payload["source_revision"])
	}
	if got := int(payload["migrated_route_count"].(float64)); got != len(migratedRoutes) {
		t.Fatalf("migrated_route_count = %d, want %d", got, len(migratedRoutes))
	}
}

func TestCompatRoutesStandaloneUsesEmbeddedManifest(t *testing.T) {
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		Standalone:   true,
		ProxyTimeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/compat/routes", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["source_revision"] == "" {
		t.Fatalf("source_revision missing: %#v", payload)
	}
	routes, ok := payload["routes"].([]any)
	if !ok || len(routes) == 0 {
		t.Fatalf("routes missing: %#v", payload["routes"])
	}
	if got := int(payload["migrated_route_count"].(float64)); got != len(migratedRoutes) {
		t.Fatalf("migrated_route_count = %d, want %d", got, len(migratedRoutes))
	}
}

func TestStandaloneUnknownRouteReturnsNotMigrated(t *testing.T) {
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		Standalone:   true,
		ProxyTimeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/dashboard/user/loginShow", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusNotImplemented, rec.Body.String())
	}
	if rec.Header().Get("X-Mochat-Go-Compat") != "" {
		t.Fatalf("unexpected fallback header = %q", rec.Header().Get("X-Mochat-Go-Compat"))
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["message"] != "route not yet migrated in standalone Go runtime" {
		t.Fatalf("payload = %#v", payload)
	}
}

func TestBundledFrontendUndefinedAPIPrefixRoutesToMigratedHandlers(t *testing.T) {
	echoPath := func(prefix string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(prefix + " " + r.URL.RequestURI()))
		}
	}
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		Standalone:   true,
		ProxyTimeout: time.Second,
	},
		WithCorpDataIndexHandler(echoPath("dashboard")),
		WithSidebarWorkContactDetailHandler(echoPath("sidebar")),
		WithOperationWorkFissionTaskDataHandler(echoPath("operation")),
	)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		path string
		want string
	}{
		{path: "/undefined/dashboard/corpData/index?corpId=1", want: "dashboard /dashboard/corpData/index?corpId=1"},
		{path: "/undefined/sidebar/workContact/detail?wxExternalUserid=external-user-900001", want: "sidebar /sidebar/workContact/detail?wxExternalUserid=external-user-900001"},
		{path: "/undefined/operation/workFission/taskData?taskId=1", want: "operation /operation/workFission/taskData?taskId=1"},
	} {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, body=%s", tc.path, rec.Code, rec.Body.String())
		}
		if rec.Body.String() != tc.want {
			t.Fatalf("%s body = %q, want %q", tc.path, rec.Body.String(), tc.want)
		}
	}
}

func TestLoginShowFallsBackWhenNotMigrated(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/dashboard/user/loginShow" {
			t.Fatalf("unexpected upstream path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte("php loginShow"))
	}))
	defer upstream.Close()

	srv := newTestServer(t, upstream.URL)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/user/loginShow", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "php loginShow" {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestAuthFallsBackWhenNotMigrated(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/dashboard/user/auth" {
			t.Fatalf("unexpected upstream path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte("php auth"))
	}))
	defer upstream.Close()

	srv := newTestServer(t, upstream.URL)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/user/auth", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "php auth" {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestAuthUsesMigratedHandlerWhenConfigured(t *testing.T) {
	migrated := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go auth"))
	})
	sourceRoot := t.TempDir()
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		PHPUpstream:  "http://127.0.0.1:9501",
		SourceRoot:   sourceRoot,
		ManifestPath: writeManifest(t),
		ProxyTimeout: time.Second,
	}, WithAuthHandler(migrated))
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/dashboard/user/auth", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "go auth" {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestSaaSAlertRoutesDispatchWhenConfigured(t *testing.T) {
	page := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas alert page"))
	})
	index := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas alert index"))
	})
	resolve := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas alert resolve"))
	})
	setting := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas alert setting"))
	})
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		Standalone:   true,
		ProxyTimeout: time.Second,
	}, WithSaaSAlertPageHandler(page), WithSaaSAlertIndexHandler(index), WithSaaSAlertResolveHandler(resolve), WithSaaSAlertSettingHandler(setting))
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodGet, path: "/dashboard/saasAlert/page", body: "go saas alert page"},
		{method: http.MethodGet, path: "/dashboard/saasAlert/index", body: "go saas alert index"},
		{method: http.MethodPut, path: "/dashboard/saasAlert/resolve", body: "go saas alert resolve"},
		{method: http.MethodPost, path: "/dashboard/saasAlert/resolve", body: "go saas alert resolve"},
		{method: http.MethodGet, path: "/dashboard/saasAlert/setting", body: "go saas alert setting"},
		{method: http.MethodPut, path: "/dashboard/saasAlert/setting", body: "go saas alert setting"},
		{method: http.MethodPost, path: "/dashboard/saasAlert/setting", body: "go saas alert setting"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s %s status = %d, body=%s", tc.method, tc.path, rec.Code, rec.Body.String())
		}
		if rec.Body.String() != tc.body {
			t.Fatalf("%s %s body = %q", tc.method, tc.path, rec.Body.String())
		}
	}

	routes := srv.migratedRoutes()
	for _, route := range []string{
		"GET /dashboard/saasAlert/page",
		"HEAD /dashboard/saasAlert/page",
		"GET /dashboard/saasAlert/index",
		"PUT /dashboard/saasAlert/resolve",
		"POST /dashboard/saasAlert/resolve",
		"GET /dashboard/saasAlert/setting",
		"PUT /dashboard/saasAlert/setting",
		"POST /dashboard/saasAlert/setting",
	} {
		if !containsString(routes, route) {
			t.Fatalf("route %q missing from %+v", route, routes)
		}
	}
}

func TestSaaSAdminRoutesDispatchWhenConfigured(t *testing.T) {
	page := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin page"))
	})
	overview := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin overview"))
	})
	tenant := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin tenant"))
	})
	tenantLifecycle := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin tenant lifecycle"))
	})
	usage := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin usage"))
	})
	risk := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin risk"))
	})
	businessMetrics := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin business metrics"))
	})
	businessTrends := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin business trends"))
	})
	operationQueue := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin operation queue"))
	})
	operationQueueOwners := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin operation queue owners"))
	})
	operationQueueAssignments := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin operation queue assignments"))
	})
	operationQueueAssignmentClose := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin operation queue assignment close"))
	})
	operationQueueAssignmentNotifications := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin operation queue assignment notifications"))
	})
	operationQueueAssign := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin operation queue assign"))
	})
	renewalForecast := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin renewal forecast"))
	})
	renewalForecastTasks := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin renewal forecast tasks"))
	})
	renewalForecastAssign := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin renewal forecast assign"))
	})
	renewalForecastNotifications := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin renewal forecast notifications"))
	})
	customerSuccess := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin customer success"))
	})
	customerSuccessOwners := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin customer success owners"))
	})
	customerSuccessAssign := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin customer success assign"))
	})
	customerSuccessRenewalTasks := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin customer success renewal tasks"))
	})
	customerSuccessRenewalNotifications := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin customer success renewal notifications"))
	})
	riskFollowUp := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin risk follow up"))
	})
	riskFollowUps := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin risk follow ups"))
	})
	riskFollowUpOwners := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin risk follow up owners"))
	})
	riskFollowUpBulkClose := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin risk follow up bulk close"))
	})
	alerts := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin alerts"))
	})
	alertResolve := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin alert resolve"))
	})
	alertBulkResolve := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin alert bulk resolve"))
	})
	notifications := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin notifications"))
	})
	notificationPolicies := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin notification policies"))
	})
	notificationPolicy := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin notification policy"))
	})
	notificationPolicyTest := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin notification policy test"))
	})
	notificationRetry := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin notification retry"))
	})
	notificationBulkRetry := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin notification bulk retry"))
	})
	notificationClose := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin notification close"))
	})
	notificationBulkClose := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin notification bulk close"))
	})
	packages := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin packages"))
	})
	operations := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin operations"))
	})
	billingEvents := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin billing events"))
	})
	billingReconciliation := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin billing reconciliation"))
	})
	billingReconciliationFollowUp := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin billing reconciliation follow up"))
	})
	billingReconciliationFollowUps := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin billing reconciliation follow ups"))
	})
	billingReconciliationFollowUpOwners := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin billing reconciliation follow up owners"))
	})
	billingReconciliationFollowUpBulkClose := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin billing reconciliation follow up bulk close"))
	})
	tasks := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin tasks"))
	})
	taskOwners := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin task owners"))
	})
	taskSLA := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin task sla"))
	})
	taskSLANotifications := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin task sla notifications"))
	})
	taskCancel := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin task cancel"))
	})
	taskBulkCancel := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin task bulk cancel"))
	})
	taskBulkReset := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin task bulk reset"))
	})
	taskReset := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin task reset"))
	})
	dailyReport := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin daily report"))
	})
	export := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin export"))
	})
	pkg := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin package"))
	})
	packageSync := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin package sync"))
	})
	packageSyncTask := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin package sync task"))
	})
	packageSyncTaskApply := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin package sync task apply"))
	})
	packageSyncTaskBulkApply := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin package sync task bulk apply"))
	})
	tenantStatus := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin tenant status"))
	})
	tenantRenewal := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin tenant renewal"))
	})
	tenantRenewalTask := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin tenant renewal task"))
	})
	tenantRenewalTaskApply := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin tenant renewal task apply"))
	})
	tenantRenewalTaskBulkApply := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin tenant renewal task bulk apply"))
	})
	tenantProvision := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin tenant provision"))
	})
	tenantProvisionTask := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin tenant provision task"))
	})
	tenantProvisionTaskApply := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin tenant provision task apply"))
	})
	tenantProvisionTaskBulkApply := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin tenant provision task bulk apply"))
	})
	tenantPackage := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin tenant package"))
	})
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		Standalone:   true,
		ProxyTimeout: time.Second,
	}, WithSaaSAdminPageHandler(page), WithSaaSAdminOverviewHandler(overview), WithSaaSAdminTenantHandler(tenant), WithSaaSAdminTenantLifecycleHandler(tenantLifecycle), WithSaaSAdminUsageHandler(usage), WithSaaSAdminRiskHandler(risk), WithSaaSAdminBusinessMetricsHandler(businessMetrics), WithSaaSAdminBusinessTrendsHandler(businessTrends), WithSaaSAdminOperationQueueHandler(operationQueue), WithSaaSAdminOperationQueueOwnersHandler(operationQueueOwners), WithSaaSAdminOperationQueueAssignmentsHandler(operationQueueAssignments), WithSaaSAdminOperationQueueAssignmentCloseHandler(operationQueueAssignmentClose), WithSaaSAdminOperationQueueAssignmentNotificationsHandler(operationQueueAssignmentNotifications), WithSaaSAdminOperationQueueAssignHandler(operationQueueAssign), WithSaaSAdminRenewalForecastHandler(renewalForecast), WithSaaSAdminRenewalForecastTasksHandler(renewalForecastTasks), WithSaaSAdminRenewalForecastAssignHandler(renewalForecastAssign), WithSaaSAdminRenewalForecastNotificationsHandler(renewalForecastNotifications), WithSaaSAdminCustomerSuccessHandler(customerSuccess), WithSaaSAdminCustomerSuccessOwnersHandler(customerSuccessOwners), WithSaaSAdminCustomerSuccessAssignHandler(customerSuccessAssign), WithSaaSAdminCustomerSuccessRenewalTasksHandler(customerSuccessRenewalTasks), WithSaaSAdminCustomerSuccessRenewalNotificationsHandler(customerSuccessRenewalNotifications), WithSaaSAdminRiskFollowUpHandler(riskFollowUp), WithSaaSAdminRiskFollowUpsHandler(riskFollowUps), WithSaaSAdminRiskFollowUpOwnersHandler(riskFollowUpOwners), WithSaaSAdminRiskFollowUpBulkCloseHandler(riskFollowUpBulkClose), WithSaaSAdminAlertsHandler(alerts), WithSaaSAdminAlertResolveHandler(alertResolve), WithSaaSAdminAlertBulkResolveHandler(alertBulkResolve), WithSaaSAdminNotificationsHandler(notifications), WithSaaSAdminNotificationPoliciesHandler(notificationPolicies), WithSaaSAdminNotificationPolicyHandler(notificationPolicy), WithSaaSAdminNotificationPolicyTestHandler(notificationPolicyTest), WithSaaSAdminNotificationRetryHandler(notificationRetry), WithSaaSAdminNotificationBulkRetryHandler(notificationBulkRetry), WithSaaSAdminNotificationCloseHandler(notificationClose), WithSaaSAdminNotificationBulkCloseHandler(notificationBulkClose), WithSaaSAdminPackagesHandler(packages), WithSaaSAdminOperationsHandler(operations), WithSaaSAdminBillingEventsHandler(billingEvents), WithSaaSAdminBillingReconciliationHandler(billingReconciliation), WithSaaSAdminBillingReconciliationFollowUpHandler(billingReconciliationFollowUp), WithSaaSAdminBillingReconciliationFollowUpsHandler(billingReconciliationFollowUps), WithSaaSAdminBillingReconciliationFollowUpOwnersHandler(billingReconciliationFollowUpOwners), WithSaaSAdminBillingReconciliationFollowUpBulkCloseHandler(billingReconciliationFollowUpBulkClose), WithSaaSAdminTasksHandler(tasks), WithSaaSAdminTaskOwnersHandler(taskOwners), WithSaaSAdminTaskSLAHandler(taskSLA), WithSaaSAdminTaskSLANotificationsHandler(taskSLANotifications), WithSaaSAdminTaskCancelHandler(taskCancel), WithSaaSAdminTaskBulkCancelHandler(taskBulkCancel), WithSaaSAdminTaskBulkResetHandler(taskBulkReset), WithSaaSAdminTaskResetHandler(taskReset), WithSaaSAdminDailyReportHandler(dailyReport), WithSaaSAdminExportHandler(export), WithSaaSAdminPackageHandler(pkg), WithSaaSAdminPackageSyncHandler(packageSync), WithSaaSAdminPackageSyncTaskHandler(packageSyncTask), WithSaaSAdminPackageSyncTaskApplyHandler(packageSyncTaskApply), WithSaaSAdminPackageSyncTaskBulkApplyHandler(packageSyncTaskBulkApply), WithSaaSAdminTenantStatusHandler(tenantStatus), WithSaaSAdminTenantRenewalHandler(tenantRenewal), WithSaaSAdminTenantRenewalTaskHandler(tenantRenewalTask), WithSaaSAdminTenantRenewalTaskApplyHandler(tenantRenewalTaskApply), WithSaaSAdminTenantRenewalTaskBulkApplyHandler(tenantRenewalTaskBulkApply), WithSaaSAdminTenantProvisionHandler(tenantProvision), WithSaaSAdminTenantProvisionTaskHandler(tenantProvisionTask), WithSaaSAdminTenantProvisionTaskApplyHandler(tenantProvisionTaskApply), WithSaaSAdminTenantProvisionTaskBulkApplyHandler(tenantProvisionTaskBulkApply), WithSaaSAdminTenantPackageHandler(tenantPackage))
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodGet, path: "/dashboard/saasAdmin/page", body: "go saas admin page"},
		{method: http.MethodHead, path: "/dashboard/saasAdmin/page", body: "go saas admin page"},
		{method: http.MethodGet, path: "/dashboard/saasAdmin/overview", body: "go saas admin overview"},
		{method: http.MethodGet, path: "/dashboard/saasAdmin/tenant", body: "go saas admin tenant"},
		{method: http.MethodGet, path: "/dashboard/saasAdmin/tenantLifecycle", body: "go saas admin tenant lifecycle"},
		{method: http.MethodGet, path: "/dashboard/saasAdmin/usage", body: "go saas admin usage"},
		{method: http.MethodGet, path: "/dashboard/saasAdmin/risk", body: "go saas admin risk"},
		{method: http.MethodGet, path: "/dashboard/saasAdmin/businessMetrics", body: "go saas admin business metrics"},
		{method: http.MethodGet, path: "/dashboard/saasAdmin/businessTrends", body: "go saas admin business trends"},
		{method: http.MethodGet, path: "/dashboard/saasAdmin/operationQueue", body: "go saas admin operation queue"},
		{method: http.MethodGet, path: "/dashboard/saasAdmin/operationQueueOwners", body: "go saas admin operation queue owners"},
		{method: http.MethodGet, path: "/dashboard/saasAdmin/operationQueueAssignments", body: "go saas admin operation queue assignments"},
		{method: http.MethodPost, path: "/dashboard/saasAdmin/operationQueueAssignmentClose", body: "go saas admin operation queue assignment close"},
		{method: http.MethodPut, path: "/dashboard/saasAdmin/operationQueueAssignmentClose", body: "go saas admin operation queue assignment close"},
		{method: http.MethodPost, path: "/dashboard/saasAdmin/operationQueueAssignmentNotifications", body: "go saas admin operation queue assignment notifications"},
		{method: http.MethodPut, path: "/dashboard/saasAdmin/operationQueueAssignmentNotifications", body: "go saas admin operation queue assignment notifications"},
		{method: http.MethodPost, path: "/dashboard/saasAdmin/operationQueueAssign", body: "go saas admin operation queue assign"},
		{method: http.MethodPut, path: "/dashboard/saasAdmin/operationQueueAssign", body: "go saas admin operation queue assign"},
		{method: http.MethodGet, path: "/dashboard/saasAdmin/renewalForecast", body: "go saas admin renewal forecast"},
		{method: http.MethodPost, path: "/dashboard/saasAdmin/renewalForecastTasks", body: "go saas admin renewal forecast tasks"},
		{method: http.MethodPut, path: "/dashboard/saasAdmin/renewalForecastTasks", body: "go saas admin renewal forecast tasks"},
		{method: http.MethodPost, path: "/dashboard/saasAdmin/renewalForecastAssign", body: "go saas admin renewal forecast assign"},
		{method: http.MethodPut, path: "/dashboard/saasAdmin/renewalForecastAssign", body: "go saas admin renewal forecast assign"},
		{method: http.MethodPost, path: "/dashboard/saasAdmin/renewalForecastNotifications", body: "go saas admin renewal forecast notifications"},
		{method: http.MethodPut, path: "/dashboard/saasAdmin/renewalForecastNotifications", body: "go saas admin renewal forecast notifications"},
		{method: http.MethodGet, path: "/dashboard/saasAdmin/customerSuccess", body: "go saas admin customer success"},
		{method: http.MethodGet, path: "/dashboard/saasAdmin/customerSuccessOwners", body: "go saas admin customer success owners"},
		{method: http.MethodPost, path: "/dashboard/saasAdmin/customerSuccessAssign", body: "go saas admin customer success assign"},
		{method: http.MethodPut, path: "/dashboard/saasAdmin/customerSuccessAssign", body: "go saas admin customer success assign"},
		{method: http.MethodPost, path: "/dashboard/saasAdmin/customerSuccessRenewalTasks", body: "go saas admin customer success renewal tasks"},
		{method: http.MethodPut, path: "/dashboard/saasAdmin/customerSuccessRenewalTasks", body: "go saas admin customer success renewal tasks"},
		{method: http.MethodPost, path: "/dashboard/saasAdmin/customerSuccessRenewalNotifications", body: "go saas admin customer success renewal notifications"},
		{method: http.MethodPut, path: "/dashboard/saasAdmin/customerSuccessRenewalNotifications", body: "go saas admin customer success renewal notifications"},
		{method: http.MethodPost, path: "/dashboard/saasAdmin/riskFollowUp", body: "go saas admin risk follow up"},
		{method: http.MethodPut, path: "/dashboard/saasAdmin/riskFollowUp", body: "go saas admin risk follow up"},
		{method: http.MethodGet, path: "/dashboard/saasAdmin/riskFollowUps", body: "go saas admin risk follow ups"},
		{method: http.MethodGet, path: "/dashboard/saasAdmin/riskFollowUpOwners", body: "go saas admin risk follow up owners"},
		{method: http.MethodPost, path: "/dashboard/saasAdmin/riskFollowUpBulkClose", body: "go saas admin risk follow up bulk close"},
		{method: http.MethodPut, path: "/dashboard/saasAdmin/riskFollowUpBulkClose", body: "go saas admin risk follow up bulk close"},
		{method: http.MethodGet, path: "/dashboard/saasAdmin/alerts", body: "go saas admin alerts"},
		{method: http.MethodPost, path: "/dashboard/saasAdmin/alertResolve", body: "go saas admin alert resolve"},
		{method: http.MethodPut, path: "/dashboard/saasAdmin/alertResolve", body: "go saas admin alert resolve"},
		{method: http.MethodPost, path: "/dashboard/saasAdmin/alertBulkResolve", body: "go saas admin alert bulk resolve"},
		{method: http.MethodPut, path: "/dashboard/saasAdmin/alertBulkResolve", body: "go saas admin alert bulk resolve"},
		{method: http.MethodGet, path: "/dashboard/saasAdmin/notifications", body: "go saas admin notifications"},
		{method: http.MethodGet, path: "/dashboard/saasAdmin/notificationPolicies", body: "go saas admin notification policies"},
		{method: http.MethodGet, path: "/dashboard/saasAdmin/notificationPolicy", body: "go saas admin notification policy"},
		{method: http.MethodPost, path: "/dashboard/saasAdmin/notificationPolicy", body: "go saas admin notification policy"},
		{method: http.MethodPut, path: "/dashboard/saasAdmin/notificationPolicy", body: "go saas admin notification policy"},
		{method: http.MethodPost, path: "/dashboard/saasAdmin/notificationPolicyTest", body: "go saas admin notification policy test"},
		{method: http.MethodPut, path: "/dashboard/saasAdmin/notificationPolicyTest", body: "go saas admin notification policy test"},
		{method: http.MethodPost, path: "/dashboard/saasAdmin/notificationRetry", body: "go saas admin notification retry"},
		{method: http.MethodPut, path: "/dashboard/saasAdmin/notificationRetry", body: "go saas admin notification retry"},
		{method: http.MethodPost, path: "/dashboard/saasAdmin/notificationBulkRetry", body: "go saas admin notification bulk retry"},
		{method: http.MethodPut, path: "/dashboard/saasAdmin/notificationBulkRetry", body: "go saas admin notification bulk retry"},
		{method: http.MethodPost, path: "/dashboard/saasAdmin/notificationClose", body: "go saas admin notification close"},
		{method: http.MethodPut, path: "/dashboard/saasAdmin/notificationClose", body: "go saas admin notification close"},
		{method: http.MethodPost, path: "/dashboard/saasAdmin/notificationBulkClose", body: "go saas admin notification bulk close"},
		{method: http.MethodPut, path: "/dashboard/saasAdmin/notificationBulkClose", body: "go saas admin notification bulk close"},
		{method: http.MethodGet, path: "/dashboard/saasAdmin/packages", body: "go saas admin packages"},
		{method: http.MethodGet, path: "/dashboard/saasAdmin/operations", body: "go saas admin operations"},
		{method: http.MethodGet, path: "/dashboard/saasAdmin/billingEvents", body: "go saas admin billing events"},
		{method: http.MethodGet, path: "/dashboard/saasAdmin/billingReconciliation", body: "go saas admin billing reconciliation"},
		{method: http.MethodPost, path: "/dashboard/saasAdmin/billingReconciliationFollowUp", body: "go saas admin billing reconciliation follow up"},
		{method: http.MethodPut, path: "/dashboard/saasAdmin/billingReconciliationFollowUp", body: "go saas admin billing reconciliation follow up"},
		{method: http.MethodGet, path: "/dashboard/saasAdmin/billingReconciliationFollowUps", body: "go saas admin billing reconciliation follow ups"},
		{method: http.MethodGet, path: "/dashboard/saasAdmin/billingReconciliationFollowUpOwners", body: "go saas admin billing reconciliation follow up owners"},
		{method: http.MethodPost, path: "/dashboard/saasAdmin/billingReconciliationFollowUpBulkClose", body: "go saas admin billing reconciliation follow up bulk close"},
		{method: http.MethodPut, path: "/dashboard/saasAdmin/billingReconciliationFollowUpBulkClose", body: "go saas admin billing reconciliation follow up bulk close"},
		{method: http.MethodGet, path: "/dashboard/saasAdmin/tasks", body: "go saas admin tasks"},
		{method: http.MethodGet, path: "/dashboard/saasAdmin/taskOwners", body: "go saas admin task owners"},
		{method: http.MethodGet, path: "/dashboard/saasAdmin/taskSla", body: "go saas admin task sla"},
		{method: http.MethodPost, path: "/dashboard/saasAdmin/taskSlaNotifications", body: "go saas admin task sla notifications"},
		{method: http.MethodPut, path: "/dashboard/saasAdmin/taskSlaNotifications", body: "go saas admin task sla notifications"},
		{method: http.MethodPost, path: "/dashboard/saasAdmin/taskCancel", body: "go saas admin task cancel"},
		{method: http.MethodPut, path: "/dashboard/saasAdmin/taskCancel", body: "go saas admin task cancel"},
		{method: http.MethodPost, path: "/dashboard/saasAdmin/taskBulkCancel", body: "go saas admin task bulk cancel"},
		{method: http.MethodPut, path: "/dashboard/saasAdmin/taskBulkCancel", body: "go saas admin task bulk cancel"},
		{method: http.MethodPost, path: "/dashboard/saasAdmin/taskBulkReset", body: "go saas admin task bulk reset"},
		{method: http.MethodPut, path: "/dashboard/saasAdmin/taskBulkReset", body: "go saas admin task bulk reset"},
		{method: http.MethodPost, path: "/dashboard/saasAdmin/taskReset", body: "go saas admin task reset"},
		{method: http.MethodPut, path: "/dashboard/saasAdmin/taskReset", body: "go saas admin task reset"},
		{method: http.MethodGet, path: "/dashboard/saasAdmin/dailyReport", body: "go saas admin daily report"},
		{method: http.MethodGet, path: "/dashboard/saasAdmin/export", body: "go saas admin export"},
		{method: http.MethodPost, path: "/dashboard/saasAdmin/package", body: "go saas admin package"},
		{method: http.MethodPut, path: "/dashboard/saasAdmin/package", body: "go saas admin package"},
		{method: http.MethodPost, path: "/dashboard/saasAdmin/packageSync", body: "go saas admin package sync"},
		{method: http.MethodPut, path: "/dashboard/saasAdmin/packageSync", body: "go saas admin package sync"},
		{method: http.MethodPost, path: "/dashboard/saasAdmin/packageSyncTask", body: "go saas admin package sync task"},
		{method: http.MethodPut, path: "/dashboard/saasAdmin/packageSyncTask", body: "go saas admin package sync task"},
		{method: http.MethodPost, path: "/dashboard/saasAdmin/packageSyncTaskApply", body: "go saas admin package sync task apply"},
		{method: http.MethodPut, path: "/dashboard/saasAdmin/packageSyncTaskApply", body: "go saas admin package sync task apply"},
		{method: http.MethodPost, path: "/dashboard/saasAdmin/packageSyncTaskBulkApply", body: "go saas admin package sync task bulk apply"},
		{method: http.MethodPut, path: "/dashboard/saasAdmin/packageSyncTaskBulkApply", body: "go saas admin package sync task bulk apply"},
		{method: http.MethodPost, path: "/dashboard/saasAdmin/tenantStatus", body: "go saas admin tenant status"},
		{method: http.MethodPut, path: "/dashboard/saasAdmin/tenantStatus", body: "go saas admin tenant status"},
		{method: http.MethodPost, path: "/dashboard/saasAdmin/tenantRenewal", body: "go saas admin tenant renewal"},
		{method: http.MethodPut, path: "/dashboard/saasAdmin/tenantRenewal", body: "go saas admin tenant renewal"},
		{method: http.MethodPost, path: "/dashboard/saasAdmin/tenantRenewalTask", body: "go saas admin tenant renewal task"},
		{method: http.MethodPut, path: "/dashboard/saasAdmin/tenantRenewalTask", body: "go saas admin tenant renewal task"},
		{method: http.MethodPost, path: "/dashboard/saasAdmin/tenantRenewalTaskApply", body: "go saas admin tenant renewal task apply"},
		{method: http.MethodPut, path: "/dashboard/saasAdmin/tenantRenewalTaskApply", body: "go saas admin tenant renewal task apply"},
		{method: http.MethodPost, path: "/dashboard/saasAdmin/tenantRenewalTaskBulkApply", body: "go saas admin tenant renewal task bulk apply"},
		{method: http.MethodPut, path: "/dashboard/saasAdmin/tenantRenewalTaskBulkApply", body: "go saas admin tenant renewal task bulk apply"},
		{method: http.MethodPost, path: "/dashboard/saasAdmin/tenantProvision", body: "go saas admin tenant provision"},
		{method: http.MethodPut, path: "/dashboard/saasAdmin/tenantProvision", body: "go saas admin tenant provision"},
		{method: http.MethodPost, path: "/dashboard/saasAdmin/tenantProvisionTask", body: "go saas admin tenant provision task"},
		{method: http.MethodPut, path: "/dashboard/saasAdmin/tenantProvisionTask", body: "go saas admin tenant provision task"},
		{method: http.MethodPost, path: "/dashboard/saasAdmin/tenantProvisionTaskApply", body: "go saas admin tenant provision task apply"},
		{method: http.MethodPut, path: "/dashboard/saasAdmin/tenantProvisionTaskApply", body: "go saas admin tenant provision task apply"},
		{method: http.MethodPost, path: "/dashboard/saasAdmin/tenantProvisionTaskBulkApply", body: "go saas admin tenant provision task bulk apply"},
		{method: http.MethodPut, path: "/dashboard/saasAdmin/tenantProvisionTaskBulkApply", body: "go saas admin tenant provision task bulk apply"},
		{method: http.MethodPost, path: "/dashboard/saasAdmin/tenantPackage", body: "go saas admin tenant package"},
		{method: http.MethodPut, path: "/dashboard/saasAdmin/tenantPackage", body: "go saas admin tenant package"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s %s status = %d, body=%s", tc.method, tc.path, rec.Code, rec.Body.String())
		}
		if rec.Body.String() != tc.body {
			t.Fatalf("%s %s body = %q", tc.method, tc.path, rec.Body.String())
		}
	}

	routes := srv.migratedRoutes()
	for _, route := range []string{
		"GET /dashboard/saasAdmin/page",
		"HEAD /dashboard/saasAdmin/page",
		"GET /dashboard/saasAdmin/overview",
		"GET /dashboard/saasAdmin/tenant",
		"GET /dashboard/saasAdmin/tenantLifecycle",
		"GET /dashboard/saasAdmin/usage",
		"GET /dashboard/saasAdmin/risk",
		"GET /dashboard/saasAdmin/businessMetrics",
		"GET /dashboard/saasAdmin/businessTrends",
		"GET /dashboard/saasAdmin/operationQueue",
		"GET /dashboard/saasAdmin/operationQueueOwners",
		"GET /dashboard/saasAdmin/operationQueueAssignments",
		"POST /dashboard/saasAdmin/operationQueueAssignmentClose",
		"PUT /dashboard/saasAdmin/operationQueueAssignmentClose",
		"POST /dashboard/saasAdmin/operationQueueAssignmentNotifications",
		"PUT /dashboard/saasAdmin/operationQueueAssignmentNotifications",
		"POST /dashboard/saasAdmin/operationQueueAssign",
		"PUT /dashboard/saasAdmin/operationQueueAssign",
		"GET /dashboard/saasAdmin/renewalForecast",
		"POST /dashboard/saasAdmin/renewalForecastTasks",
		"PUT /dashboard/saasAdmin/renewalForecastTasks",
		"POST /dashboard/saasAdmin/renewalForecastAssign",
		"PUT /dashboard/saasAdmin/renewalForecastAssign",
		"POST /dashboard/saasAdmin/renewalForecastNotifications",
		"PUT /dashboard/saasAdmin/renewalForecastNotifications",
		"GET /dashboard/saasAdmin/customerSuccess",
		"GET /dashboard/saasAdmin/customerSuccessOwners",
		"POST /dashboard/saasAdmin/customerSuccessAssign",
		"PUT /dashboard/saasAdmin/customerSuccessAssign",
		"POST /dashboard/saasAdmin/customerSuccessRenewalTasks",
		"PUT /dashboard/saasAdmin/customerSuccessRenewalTasks",
		"POST /dashboard/saasAdmin/customerSuccessRenewalNotifications",
		"PUT /dashboard/saasAdmin/customerSuccessRenewalNotifications",
		"POST /dashboard/saasAdmin/riskFollowUp",
		"PUT /dashboard/saasAdmin/riskFollowUp",
		"GET /dashboard/saasAdmin/riskFollowUps",
		"GET /dashboard/saasAdmin/riskFollowUpOwners",
		"POST /dashboard/saasAdmin/riskFollowUpBulkClose",
		"PUT /dashboard/saasAdmin/riskFollowUpBulkClose",
		"GET /dashboard/saasAdmin/alerts",
		"POST /dashboard/saasAdmin/alertResolve",
		"PUT /dashboard/saasAdmin/alertResolve",
		"POST /dashboard/saasAdmin/alertBulkResolve",
		"PUT /dashboard/saasAdmin/alertBulkResolve",
		"GET /dashboard/saasAdmin/notifications",
		"GET /dashboard/saasAdmin/notificationPolicies",
		"GET /dashboard/saasAdmin/notificationPolicy",
		"POST /dashboard/saasAdmin/notificationPolicy",
		"PUT /dashboard/saasAdmin/notificationPolicy",
		"POST /dashboard/saasAdmin/notificationPolicyTest",
		"PUT /dashboard/saasAdmin/notificationPolicyTest",
		"POST /dashboard/saasAdmin/notificationRetry",
		"PUT /dashboard/saasAdmin/notificationRetry",
		"POST /dashboard/saasAdmin/notificationBulkRetry",
		"PUT /dashboard/saasAdmin/notificationBulkRetry",
		"POST /dashboard/saasAdmin/notificationClose",
		"PUT /dashboard/saasAdmin/notificationClose",
		"POST /dashboard/saasAdmin/notificationBulkClose",
		"PUT /dashboard/saasAdmin/notificationBulkClose",
		"GET /dashboard/saasAdmin/packages",
		"GET /dashboard/saasAdmin/operations",
		"GET /dashboard/saasAdmin/billingEvents",
		"GET /dashboard/saasAdmin/billingReconciliation",
		"POST /dashboard/saasAdmin/billingReconciliationFollowUp",
		"PUT /dashboard/saasAdmin/billingReconciliationFollowUp",
		"GET /dashboard/saasAdmin/billingReconciliationFollowUps",
		"GET /dashboard/saasAdmin/billingReconciliationFollowUpOwners",
		"POST /dashboard/saasAdmin/billingReconciliationFollowUpBulkClose",
		"PUT /dashboard/saasAdmin/billingReconciliationFollowUpBulkClose",
		"GET /dashboard/saasAdmin/tasks",
		"GET /dashboard/saasAdmin/taskOwners",
		"GET /dashboard/saasAdmin/taskSla",
		"POST /dashboard/saasAdmin/taskSlaNotifications",
		"PUT /dashboard/saasAdmin/taskSlaNotifications",
		"POST /dashboard/saasAdmin/taskCancel",
		"PUT /dashboard/saasAdmin/taskCancel",
		"POST /dashboard/saasAdmin/taskBulkCancel",
		"PUT /dashboard/saasAdmin/taskBulkCancel",
		"GET /dashboard/saasAdmin/dailyReport",
		"GET /dashboard/saasAdmin/export",
		"POST /dashboard/saasAdmin/package",
		"PUT /dashboard/saasAdmin/package",
		"POST /dashboard/saasAdmin/packageSync",
		"PUT /dashboard/saasAdmin/packageSync",
		"POST /dashboard/saasAdmin/packageSyncTask",
		"PUT /dashboard/saasAdmin/packageSyncTask",
		"POST /dashboard/saasAdmin/packageSyncTaskApply",
		"PUT /dashboard/saasAdmin/packageSyncTaskApply",
		"POST /dashboard/saasAdmin/packageSyncTaskBulkApply",
		"PUT /dashboard/saasAdmin/packageSyncTaskBulkApply",
		"POST /dashboard/saasAdmin/tenantStatus",
		"PUT /dashboard/saasAdmin/tenantStatus",
		"POST /dashboard/saasAdmin/tenantRenewal",
		"PUT /dashboard/saasAdmin/tenantRenewal",
		"POST /dashboard/saasAdmin/tenantRenewalTask",
		"PUT /dashboard/saasAdmin/tenantRenewalTask",
		"POST /dashboard/saasAdmin/tenantRenewalTaskApply",
		"PUT /dashboard/saasAdmin/tenantRenewalTaskApply",
		"POST /dashboard/saasAdmin/tenantRenewalTaskBulkApply",
		"PUT /dashboard/saasAdmin/tenantRenewalTaskBulkApply",
		"POST /dashboard/saasAdmin/tenantProvision",
		"PUT /dashboard/saasAdmin/tenantProvision",
		"POST /dashboard/saasAdmin/tenantProvisionTask",
		"PUT /dashboard/saasAdmin/tenantProvisionTask",
		"POST /dashboard/saasAdmin/tenantProvisionTaskApply",
		"PUT /dashboard/saasAdmin/tenantProvisionTaskApply",
		"POST /dashboard/saasAdmin/tenantProvisionTaskBulkApply",
		"PUT /dashboard/saasAdmin/tenantProvisionTaskBulkApply",
		"POST /dashboard/saasAdmin/tenantPackage",
		"PUT /dashboard/saasAdmin/tenantPackage",
	} {
		if !containsString(routes, route) {
			t.Fatalf("route %q missing from %+v", route, routes)
		}
	}
}

func TestSaaSAdminNotificationHealthRouteDispatchesAndLists(t *testing.T) {
	health := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin notification health"))
	})
	slo := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin notification slo"))
	})
	recovery := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go saas admin notification health recovery"))
	})
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		Standalone:   true,
		ProxyTimeout: time.Second,
	}, WithSaaSAdminNotificationHealthHandler(health), WithSaaSAdminNotificationSLOHandler(slo), WithSaaSAdminNotificationHealthRecoveryHandler(recovery))
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/notificationHealth", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "go saas admin notification health" {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
	sloReq := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/notificationSlo", nil)
	sloRec := httptest.NewRecorder()
	srv.ServeHTTP(sloRec, sloReq)
	if sloRec.Code != http.StatusOK || sloRec.Body.String() != "go saas admin notification slo" {
		t.Fatalf("SLO status=%d body=%q", sloRec.Code, sloRec.Body.String())
	}
	for _, method := range []string{http.MethodPost, http.MethodPut} {
		recoveryReq := httptest.NewRequest(method, "/dashboard/saasAdmin/notificationHealthRecovery", nil)
		recoveryRec := httptest.NewRecorder()
		srv.ServeHTTP(recoveryRec, recoveryReq)
		if recoveryRec.Code != http.StatusOK || recoveryRec.Body.String() != "go saas admin notification health recovery" {
			t.Fatalf("method=%s status=%d body=%q", method, recoveryRec.Code, recoveryRec.Body.String())
		}
	}

	routes := srv.migratedRoutes()
	if !containsString(routes, "GET /dashboard/saasAdmin/notificationHealth") {
		t.Fatalf("routes missing notification health: %v", routes)
	}
	if !containsString(routes, "GET /dashboard/saasAdmin/notificationSlo") {
		t.Fatalf("routes missing notification SLO: %v", routes)
	}
	for _, route := range []string{"POST /dashboard/saasAdmin/notificationHealthRecovery", "PUT /dashboard/saasAdmin/notificationHealthRecovery"} {
		if !containsString(routes, route) {
			t.Fatalf("routes missing %s: %v", route, routes)
		}
	}
}

func TestSaaSAdminSubscriptionRoutesDispatchAndList(t *testing.T) {
	handler := func(body string) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) })
	}
	srv, err := New(config.Config{ListenAddr: ":0", Standalone: true, ProxyTimeout: time.Second},
		WithSaaSAdminSubscriptionsHandler(handler("subscriptions")),
		WithSaaSAdminSubscriptionEventsHandler(handler("subscription events")),
		WithSaaSAdminSubscriptionTransitionHandler(handler("subscription transition")),
		WithSaaSAdminSubscriptionReconcileHandler(handler("subscription reconcile")),
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/dashboard/saasAdmin/subscriptions", "subscriptions"},
		{http.MethodGet, "/dashboard/saasAdmin/subscriptionEvents", "subscription events"},
		{http.MethodPost, "/dashboard/saasAdmin/subscriptionTransition", "subscription transition"},
		{http.MethodPut, "/dashboard/saasAdmin/subscriptionTransition", "subscription transition"},
		{http.MethodPost, "/dashboard/saasAdmin/subscriptionReconcile", "subscription reconcile"},
		{http.MethodPut, "/dashboard/saasAdmin/subscriptionReconcile", "subscription reconcile"},
	} {
		req := httptest.NewRequest(test.method, test.path, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || rec.Body.String() != test.body {
			t.Fatalf("%s %s status=%d body=%q", test.method, test.path, rec.Code, rec.Body.String())
		}
	}
	for _, route := range []string{
		"GET /dashboard/saasAdmin/subscriptions",
		"GET /dashboard/saasAdmin/subscriptionEvents",
		"POST /dashboard/saasAdmin/subscriptionTransition",
		"PUT /dashboard/saasAdmin/subscriptionTransition",
		"POST /dashboard/saasAdmin/subscriptionReconcile",
		"PUT /dashboard/saasAdmin/subscriptionReconcile",
	} {
		if !containsString(srv.migratedRoutes(), route) {
			t.Fatalf("route %q missing", route)
		}
	}
}

func TestSaaSPaymentRoutesDispatchAndList(t *testing.T) {
	handler := func(body string) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) })
	}
	srv, err := New(config.Config{ListenAddr: ":0", Standalone: true, ProxyTimeout: time.Second},
		WithSaaSAdminPaymentOrdersHandler(handler("payment orders")),
		WithSaaSAdminPaymentWebhookEventsHandler(handler("payment webhook events")),
		WithSaaSAdminPaymentOrderHandler(handler("payment order")),
		WithSaaSAdminPaymentOrderCancelHandler(handler("payment order cancel")),
		WithSaaSAdminPaymentDunningHandler(handler("payment dunning")),
		WithSaaSAdminPaymentRefundsHandler(handler("payment refunds")),
		WithSaaSAdminPaymentRefundHandler(handler("payment refund")),
		WithSaaSAdminPaymentRefundCancelHandler(handler("payment refund cancel")),
		WithSaaSAdminPaymentSettlementBatchesHandler(handler("payment settlement batches")),
		WithSaaSAdminPaymentSettlementEntriesHandler(handler("payment settlement entries")),
		WithSaaSAdminPaymentSettlementImportHandler(handler("payment settlement import")),
		WithSaaSAdminPaymentSettlementReconcileHandler(handler("payment settlement reconcile")),
		WithSaaSAdminPaymentSettlementResolveHandler(handler("payment settlement resolve")),
		WithSaaSAdminPaymentSettlementTransitionHandler(handler("payment settlement transition")),
		WithSaaSAdminPaymentSettlementSyncRunsHandler(handler("payment settlement sync runs")),
		WithSaaSAdminPaymentSettlementSyncHandler(handler("payment settlement sync")),
		WithSaaSPaymentWebhookHandler(handler("payment webhook")),
	)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/dashboard/saasAdmin/paymentOrders", "payment orders"},
		{http.MethodGet, "/dashboard/saasAdmin/paymentWebhookEvents", "payment webhook events"},
		{http.MethodPost, "/dashboard/saasAdmin/paymentOrder", "payment order"},
		{http.MethodPut, "/dashboard/saasAdmin/paymentOrder", "payment order"},
		{http.MethodPost, "/dashboard/saasAdmin/paymentOrderCancel", "payment order cancel"},
		{http.MethodPut, "/dashboard/saasAdmin/paymentOrderCancel", "payment order cancel"},
		{http.MethodPost, "/dashboard/saasAdmin/paymentDunning", "payment dunning"},
		{http.MethodPut, "/dashboard/saasAdmin/paymentDunning", "payment dunning"},
		{http.MethodGet, "/dashboard/saasAdmin/paymentRefunds", "payment refunds"},
		{http.MethodPost, "/dashboard/saasAdmin/paymentRefund", "payment refund"},
		{http.MethodPut, "/dashboard/saasAdmin/paymentRefund", "payment refund"},
		{http.MethodPost, "/dashboard/saasAdmin/paymentRefundCancel", "payment refund cancel"},
		{http.MethodPut, "/dashboard/saasAdmin/paymentRefundCancel", "payment refund cancel"},
		{http.MethodGet, "/dashboard/saasAdmin/paymentSettlementBatches", "payment settlement batches"},
		{http.MethodGet, "/dashboard/saasAdmin/paymentSettlementEntries", "payment settlement entries"},
		{http.MethodPost, "/dashboard/saasAdmin/paymentSettlementImport", "payment settlement import"},
		{http.MethodPut, "/dashboard/saasAdmin/paymentSettlementImport", "payment settlement import"},
		{http.MethodPost, "/dashboard/saasAdmin/paymentSettlementReconcile", "payment settlement reconcile"},
		{http.MethodPut, "/dashboard/saasAdmin/paymentSettlementReconcile", "payment settlement reconcile"},
		{http.MethodPost, "/dashboard/saasAdmin/paymentSettlementResolve", "payment settlement resolve"},
		{http.MethodPut, "/dashboard/saasAdmin/paymentSettlementResolve", "payment settlement resolve"},
		{http.MethodPost, "/dashboard/saasAdmin/paymentSettlementTransition", "payment settlement transition"},
		{http.MethodPut, "/dashboard/saasAdmin/paymentSettlementTransition", "payment settlement transition"},
		{http.MethodGet, "/dashboard/saasAdmin/paymentSettlementSyncRuns", "payment settlement sync runs"},
		{http.MethodPost, "/dashboard/saasAdmin/paymentSettlementSync", "payment settlement sync"},
		{http.MethodPut, "/dashboard/saasAdmin/paymentSettlementSync", "payment settlement sync"},
		{http.MethodPost, "/webhooks/saas/payment", "payment webhook"},
	}
	for _, test := range tests {
		req := httptest.NewRequest(test.method, test.path, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || rec.Body.String() != test.body {
			t.Fatalf("%s %s status=%d body=%q", test.method, test.path, rec.Code, rec.Body.String())
		}
	}
	for _, test := range tests {
		route := test.method + " " + test.path
		if !containsString(srv.migratedRoutes(), route) {
			t.Fatalf("route %q missing", route)
		}
	}
}

func TestSaaSTenantDomainDeliveryRoutesDispatchAndList(t *testing.T) {
	handler := func(body string) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) })
	}
	srv, err := New(config.Config{ListenAddr: ":0", Standalone: true, ProxyTimeout: time.Second},
		WithSaaSAdminTenantDomainDeliveryJobsHandler(handler("domain delivery jobs")),
		WithSaaSAdminTenantDomainDeliveryHandler(handler("domain delivery")),
		WithSaaSTenantDomainDeliveryWebhookHandler(handler("domain delivery webhook")),
	)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/dashboard/saasAdmin/tenantDomainDeliveryJobs", "domain delivery jobs"},
		{http.MethodPost, "/dashboard/saasAdmin/tenantDomainDelivery", "domain delivery"},
		{http.MethodPut, "/dashboard/saasAdmin/tenantDomainDelivery", "domain delivery"},
		{http.MethodPost, "/webhooks/saas/domain-delivery", "domain delivery webhook"},
	}
	for _, test := range tests {
		req := httptest.NewRequest(test.method, test.path, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || rec.Body.String() != test.body {
			t.Fatalf("%s %s status=%d body=%q", test.method, test.path, rec.Code, rec.Body.String())
		}
		if !containsString(srv.migratedRoutes(), test.method+" "+test.path) {
			t.Fatalf("route %q missing", test.method+" "+test.path)
		}
	}
}

func TestSaaSAdminAccessRoutesDispatchAndList(t *testing.T) {
	handler := func(body string) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) })
	}
	srv, err := New(config.Config{ListenAddr: ":0", Standalone: true, ProxyTimeout: time.Second},
		WithSaaSAdminAccessProfileHandler(handler("access profile")),
		WithSaaSAdminAccessRolesHandler(handler("access roles")),
		WithSaaSAdminAccessRoleHandler(handler("access role")),
		WithSaaSAdminAccessAssignmentsHandler(handler("access assignments")),
		WithSaaSAdminAccessAssignmentHandler(handler("access assignment")),
	)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/dashboard/saasAdmin/accessProfile", "access profile"},
		{http.MethodGet, "/dashboard/saasAdmin/accessRoles", "access roles"},
		{http.MethodPost, "/dashboard/saasAdmin/accessRole", "access role"},
		{http.MethodPut, "/dashboard/saasAdmin/accessRole", "access role"},
		{http.MethodGet, "/dashboard/saasAdmin/accessAssignments", "access assignments"},
		{http.MethodPost, "/dashboard/saasAdmin/accessAssignment", "access assignment"},
		{http.MethodPut, "/dashboard/saasAdmin/accessAssignment", "access assignment"},
	}
	for _, test := range tests {
		req := httptest.NewRequest(test.method, test.path, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || rec.Body.String() != test.body {
			t.Fatalf("%s %s status=%d body=%q", test.method, test.path, rec.Code, rec.Body.String())
		}
		if !containsString(srv.migratedRoutes(), test.method+" "+test.path) {
			t.Fatalf("route %q missing", test.method+" "+test.path)
		}
	}
}

func TestSaaSAdminTenantReadinessRouteDispatchAndList(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("tenant readiness"))
	})
	srv, err := New(config.Config{ListenAddr: ":0", Standalone: true, ProxyTimeout: time.Second},
		WithSaaSAdminTenantReadinessHandler(handler),
	)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/tenantReadiness", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "tenant readiness" {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
	if !containsString(srv.migratedRoutes(), "GET /dashboard/saasAdmin/tenantReadiness") {
		t.Fatal("tenant readiness route missing")
	}
}

func TestSaaSAdminApprovalRoutesDispatchAndList(t *testing.T) {
	handler := func(body string) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) })
	}
	srv, err := New(config.Config{ListenAddr: ":0", Standalone: true, ProxyTimeout: time.Second},
		WithSaaSAdminApprovalPoliciesHandler(handler("approval policies")),
		WithSaaSAdminApprovalPolicyHandler(handler("approval policy")),
		WithSaaSAdminApprovalsHandler(handler("approvals")),
		WithSaaSAdminApprovalEventsHandler(handler("approval events")),
		WithSaaSAdminApprovalDecisionsHandler(handler("approval decisions")),
		WithSaaSAdminApprovalDelegationsHandler(handler("approval delegations")),
		WithSaaSAdminApprovalDelegationHandler(handler("approval delegation")),
		WithSaaSAdminApprovalRemindersHandler(handler("approval reminders")),
		WithSaaSAdminApprovalRequestHandler(handler("approval request")),
		WithSaaSAdminApprovalDecisionHandler(handler("approval decision")),
		WithSaaSAdminApprovalCancelHandler(handler("approval cancel")),
		WithSaaSAdminApprovalExecuteHandler(handler("approval execute")),
	)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/dashboard/saasAdmin/approvalPolicies", "approval policies"},
		{http.MethodPost, "/dashboard/saasAdmin/approvalPolicy", "approval policy"},
		{http.MethodPut, "/dashboard/saasAdmin/approvalPolicy", "approval policy"},
		{http.MethodGet, "/dashboard/saasAdmin/approvals", "approvals"},
		{http.MethodGet, "/dashboard/saasAdmin/approvalEvents", "approval events"},
		{http.MethodGet, "/dashboard/saasAdmin/approvalDecisions", "approval decisions"},
		{http.MethodGet, "/dashboard/saasAdmin/approvalDelegations", "approval delegations"},
		{http.MethodPost, "/dashboard/saasAdmin/approvalDelegation", "approval delegation"},
		{http.MethodPut, "/dashboard/saasAdmin/approvalDelegation", "approval delegation"},
		{http.MethodPost, "/dashboard/saasAdmin/approvalReminders", "approval reminders"},
		{http.MethodPut, "/dashboard/saasAdmin/approvalReminders", "approval reminders"},
		{http.MethodPost, "/dashboard/saasAdmin/approvalRequest", "approval request"},
		{http.MethodPut, "/dashboard/saasAdmin/approvalRequest", "approval request"},
		{http.MethodPost, "/dashboard/saasAdmin/approvalDecision", "approval decision"},
		{http.MethodPut, "/dashboard/saasAdmin/approvalDecision", "approval decision"},
		{http.MethodPost, "/dashboard/saasAdmin/approvalCancel", "approval cancel"},
		{http.MethodPut, "/dashboard/saasAdmin/approvalCancel", "approval cancel"},
		{http.MethodPost, "/dashboard/saasAdmin/approvalExecute", "approval execute"},
		{http.MethodPut, "/dashboard/saasAdmin/approvalExecute", "approval execute"},
	}
	for _, test := range tests {
		req := httptest.NewRequest(test.method, test.path, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || rec.Body.String() != test.body {
			t.Fatalf("%s %s status=%d body=%q", test.method, test.path, rec.Code, rec.Body.String())
		}
		if !containsString(srv.migratedRoutes(), test.method+" "+test.path) {
			t.Fatalf("route %q missing", test.method+" "+test.path)
		}
	}
}

func TestSaaSAdminSystemHealthRoutesDispatchAndList(t *testing.T) {
	handler := func(body string) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) })
	}
	srv, err := New(config.Config{ListenAddr: ":0", Standalone: true, ProxyTimeout: time.Second},
		WithSaaSAdminSystemHealthHandler(handler("system health")),
		WithSaaSAdminSystemHealthScansHandler(handler("system health scans")),
		WithSaaSAdminSystemIncidentsHandler(handler("system incidents")),
		WithSaaSAdminSystemHealthScanHandler(handler("system health scan")),
		WithSaaSAdminSystemIncidentHandler(handler("system incident")),
	)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/dashboard/saasAdmin/systemHealth", "system health"},
		{http.MethodGet, "/dashboard/saasAdmin/systemHealthScans", "system health scans"},
		{http.MethodGet, "/dashboard/saasAdmin/systemIncidents", "system incidents"},
		{http.MethodPost, "/dashboard/saasAdmin/systemHealthScan", "system health scan"},
		{http.MethodPut, "/dashboard/saasAdmin/systemHealthScan", "system health scan"},
		{http.MethodPost, "/dashboard/saasAdmin/systemIncident", "system incident"},
		{http.MethodPut, "/dashboard/saasAdmin/systemIncident", "system incident"},
	}
	for _, test := range tests {
		req := httptest.NewRequest(test.method, test.path, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || rec.Body.String() != test.body {
			t.Fatalf("%s %s status=%d body=%q", test.method, test.path, rec.Code, rec.Body.String())
		}
		if !containsString(srv.migratedRoutes(), test.method+" "+test.path) {
			t.Fatalf("route %q missing", test.method+" "+test.path)
		}
	}
}

func TestSaaSServiceAccountRoutesDispatchAndList(t *testing.T) {
	handler := func(body string) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) })
	}
	srv, err := New(config.Config{ListenAddr: ":0", Standalone: true, ProxyTimeout: time.Second},
		WithSaaSAdminServiceAccountsHandler(handler("service accounts")),
		WithSaaSAdminServiceAccountUsageHandler(handler("service account usage history")),
		WithSaaSAdminServiceAccountUsageAlertEvaluateHandler(handler("service account usage alert evaluate")),
		WithSaaSAdminServiceAccountHandler(handler("service account")),
		WithSaaSAdminServiceAccountKeyRotateHandler(handler("service account rotate")),
		WithSaaSAdminServiceAccountKeyRevokeHandler(handler("service account revoke")),
		WithSaaSServiceAccountWhoAmIHandler(handler("service account identity")),
		WithSaaSServiceAccountUsageHandler(handler("service account usage")),
		WithSaaSServiceAccountAlertsHandler(handler("service account alerts")),
	)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/dashboard/saasAdmin/serviceAccounts", "service accounts"},
		{http.MethodGet, "/dashboard/saasAdmin/serviceAccountUsage", "service account usage history"},
		{http.MethodPost, "/dashboard/saasAdmin/serviceAccountUsageAlertEvaluate", "service account usage alert evaluate"},
		{http.MethodPut, "/dashboard/saasAdmin/serviceAccountUsageAlertEvaluate", "service account usage alert evaluate"},
		{http.MethodPost, "/dashboard/saasAdmin/serviceAccount", "service account"},
		{http.MethodPut, "/dashboard/saasAdmin/serviceAccount", "service account"},
		{http.MethodPost, "/dashboard/saasAdmin/serviceAccountKeyRotate", "service account rotate"},
		{http.MethodPut, "/dashboard/saasAdmin/serviceAccountKeyRotate", "service account rotate"},
		{http.MethodPost, "/dashboard/saasAdmin/serviceAccountKeyRevoke", "service account revoke"},
		{http.MethodPut, "/dashboard/saasAdmin/serviceAccountKeyRevoke", "service account revoke"},
		{http.MethodGet, "/api/saas/v1/whoami", "service account identity"},
		{http.MethodGet, "/api/saas/v1/usage", "service account usage"},
		{http.MethodGet, "/api/saas/v1/alerts", "service account alerts"},
	}
	for _, test := range tests {
		req := httptest.NewRequest(test.method, test.path, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || rec.Body.String() != test.body {
			t.Fatalf("%s %s status=%d body=%q", test.method, test.path, rec.Code, rec.Body.String())
		}
		if !containsString(srv.migratedRoutes(), test.method+" "+test.path) {
			t.Fatalf("route %q missing", test.method+" "+test.path)
		}
	}
}

func TestSaaSBackupRecoveryRoutesDispatchAndList(t *testing.T) {
	handler := func(body string) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) })
	}
	srv, err := New(config.Config{ListenAddr: ":0", Standalone: true, ProxyTimeout: time.Second},
		WithSaaSAdminBackupOverviewHandler(handler("backup overview")),
		WithSaaSAdminBackupPolicyHandler(handler("backup policy")),
		WithSaaSAdminBackupRunHandler(handler("backup run")),
		WithSaaSAdminRestoreDrillHandler(handler("restore drill")),
	)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/dashboard/saasAdmin/backupOverview", "backup overview"},
		{http.MethodPost, "/dashboard/saasAdmin/backupPolicy", "backup policy"},
		{http.MethodPut, "/dashboard/saasAdmin/backupPolicy", "backup policy"},
		{http.MethodPost, "/dashboard/saasAdmin/backupRun", "backup run"},
		{http.MethodPut, "/dashboard/saasAdmin/backupRun", "backup run"},
		{http.MethodPost, "/dashboard/saasAdmin/restoreDrill", "restore drill"},
		{http.MethodPut, "/dashboard/saasAdmin/restoreDrill", "restore drill"},
	}
	for _, test := range tests {
		req := httptest.NewRequest(test.method, test.path, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || rec.Body.String() != test.body {
			t.Fatalf("%s %s status=%d body=%q", test.method, test.path, rec.Code, rec.Body.String())
		}
		if !containsString(srv.migratedRoutes(), test.method+" "+test.path) {
			t.Fatalf("route %q missing", test.method+" "+test.path)
		}
	}
}

func TestSaaSAuditAnchorRoutesDispatchAndList(t *testing.T) {
	handler := func(body string) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) })
	}
	srv, err := New(config.Config{ListenAddr: ":0", Standalone: true, ProxyTimeout: time.Second},
		WithSaaSAdminAuditAnchorsHandler(handler("audit anchors")),
		WithSaaSAdminAuditAnchorHandler(handler("audit anchor action")),
	)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/dashboard/saasAdmin/auditAnchors", "audit anchors"},
		{http.MethodPost, "/dashboard/saasAdmin/auditAnchor", "audit anchor action"},
		{http.MethodPut, "/dashboard/saasAdmin/auditAnchor", "audit anchor action"},
	}
	for _, test := range tests {
		req := httptest.NewRequest(test.method, test.path, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || rec.Body.String() != test.body {
			t.Fatalf("%s %s status=%d body=%q", test.method, test.path, rec.Code, rec.Body.String())
		}
		if !containsString(srv.migratedRoutes(), test.method+" "+test.path) {
			t.Fatalf("route %q missing", test.method+" "+test.path)
		}
	}
}

func TestSaaSComplianceRoutesDispatchAndList(t *testing.T) {
	handler := func(body string) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) })
	}
	srv, err := New(config.Config{ListenAddr: ":0", Standalone: true, ProxyTimeout: time.Second},
		WithSaaSAdminComplianceOverviewHandler(handler("compliance overview")),
		WithSaaSAdminCompliancePolicyHandler(handler("compliance policy")),
		WithSaaSAdminComplianceLegalHoldHandler(handler("compliance legal hold")),
		WithSaaSAdminComplianceExportHandler(handler("compliance export")),
		WithSaaSAdminComplianceExportDownloadHandler(handler("compliance export download")),
		WithSaaSAdminComplianceErasureHandler(handler("compliance erasure")),
		WithSaaSAdminComplianceErasureStepsHandler(handler("compliance erasure steps")),
	)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/dashboard/saasAdmin/complianceOverview", "compliance overview"},
		{http.MethodPost, "/dashboard/saasAdmin/compliancePolicy", "compliance policy"},
		{http.MethodPut, "/dashboard/saasAdmin/compliancePolicy", "compliance policy"},
		{http.MethodPost, "/dashboard/saasAdmin/complianceLegalHold", "compliance legal hold"},
		{http.MethodPut, "/dashboard/saasAdmin/complianceLegalHold", "compliance legal hold"},
		{http.MethodPost, "/dashboard/saasAdmin/complianceExport", "compliance export"},
		{http.MethodPut, "/dashboard/saasAdmin/complianceExport", "compliance export"},
		{http.MethodGet, "/dashboard/saasAdmin/complianceExportDownload", "compliance export download"},
		{http.MethodPost, "/dashboard/saasAdmin/complianceErasure", "compliance erasure"},
		{http.MethodPut, "/dashboard/saasAdmin/complianceErasure", "compliance erasure"},
		{http.MethodGet, "/dashboard/saasAdmin/complianceErasureSteps", "compliance erasure steps"},
	}
	for _, test := range tests {
		req := httptest.NewRequest(test.method, test.path, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || rec.Body.String() != test.body {
			t.Fatalf("%s %s status=%d body=%q", test.method, test.path, rec.Code, rec.Body.String())
		}
		if !containsString(srv.migratedRoutes(), test.method+" "+test.path) {
			t.Fatalf("route %q missing", test.method+" "+test.path)
		}
	}
}

func TestSaaSBillingInvoiceRoutesDispatchAndList(t *testing.T) {
	handler := func(body string) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) })
	}
	srv, err := New(config.Config{ListenAddr: ":0", Standalone: true, ProxyTimeout: time.Second},
		WithSaaSBillingPageHandler(handler("billing page")),
		WithSaaSBillingSummaryHandler(handler("billing summary")),
		WithSaaSBillingPaymentOrdersHandler(handler("billing orders")),
		WithSaaSBillingPaymentRefundsHandler(handler("billing refunds")),
		WithSaaSBillingInvoiceProfileHandler(handler("billing profile")),
		WithSaaSBillingInvoicesHandler(handler("billing invoices")),
		WithSaaSBillingInvoiceHandler(handler("billing invoice")),
		WithSaaSBillingInvoiceCancelHandler(handler("billing invoice cancel")),
		WithSaaSAdminInvoiceProfileHandler(handler("admin invoice profile")),
		WithSaaSAdminInvoiceDocumentsHandler(handler("admin invoice documents")),
		WithSaaSAdminInvoiceHandler(handler("admin invoice")),
		WithSaaSAdminCreditNoteHandler(handler("admin credit note")),
		WithSaaSAdminInvoiceTransitionHandler(handler("admin invoice transition")),
	)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/dashboard/saasBilling/page", "billing page"},
		{http.MethodHead, "/dashboard/saasBilling/page", "billing page"},
		{http.MethodGet, "/dashboard/saasBilling/summary", "billing summary"},
		{http.MethodGet, "/dashboard/saasBilling/paymentOrders", "billing orders"},
		{http.MethodGet, "/dashboard/saasBilling/paymentRefunds", "billing refunds"},
		{http.MethodGet, "/dashboard/saasBilling/invoiceProfile", "billing profile"},
		{http.MethodPost, "/dashboard/saasBilling/invoiceProfile", "billing profile"},
		{http.MethodPut, "/dashboard/saasBilling/invoiceProfile", "billing profile"},
		{http.MethodGet, "/dashboard/saasBilling/invoices", "billing invoices"},
		{http.MethodPost, "/dashboard/saasBilling/invoice", "billing invoice"},
		{http.MethodPut, "/dashboard/saasBilling/invoice", "billing invoice"},
		{http.MethodPost, "/dashboard/saasBilling/invoiceCancel", "billing invoice cancel"},
		{http.MethodPut, "/dashboard/saasBilling/invoiceCancel", "billing invoice cancel"},
		{http.MethodGet, "/dashboard/saasAdmin/invoiceProfile", "admin invoice profile"},
		{http.MethodPost, "/dashboard/saasAdmin/invoiceProfile", "admin invoice profile"},
		{http.MethodPut, "/dashboard/saasAdmin/invoiceProfile", "admin invoice profile"},
		{http.MethodGet, "/dashboard/saasAdmin/invoiceDocuments", "admin invoice documents"},
		{http.MethodPost, "/dashboard/saasAdmin/invoice", "admin invoice"},
		{http.MethodPut, "/dashboard/saasAdmin/invoice", "admin invoice"},
		{http.MethodPost, "/dashboard/saasAdmin/creditNote", "admin credit note"},
		{http.MethodPut, "/dashboard/saasAdmin/creditNote", "admin credit note"},
		{http.MethodPost, "/dashboard/saasAdmin/invoiceTransition", "admin invoice transition"},
		{http.MethodPut, "/dashboard/saasAdmin/invoiceTransition", "admin invoice transition"},
	}
	for _, test := range tests {
		req := httptest.NewRequest(test.method, test.path, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || rec.Body.String() != test.body {
			t.Fatalf("%s %s status=%d body=%q", test.method, test.path, rec.Code, rec.Body.String())
		}
		if !containsString(srv.migratedRoutes(), test.method+" "+test.path) {
			t.Fatalf("route %q missing", test.method+" "+test.path)
		}
	}
}

func TestSensitiveWordsPageRouteDispatchWhenConfigured(t *testing.T) {
	page := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go sensitive words page"))
	})
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		Standalone:   true,
		ProxyTimeout: time.Second,
	}, WithSensitiveWordsPageHandler(page))
	if err != nil {
		t.Fatal(err)
	}

	for _, method := range []string{http.MethodGet, http.MethodHead} {
		req := httptest.NewRequest(method, "/dashboard/sensitiveWords/page", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, body=%s", method, rec.Code, rec.Body.String())
		}
		if method == http.MethodGet && rec.Body.String() != "go sensitive words page" {
			t.Fatalf("body = %q", rec.Body.String())
		}
	}

	routes := srv.migratedRoutes()
	for _, route := range []string{
		"GET /dashboard/sensitiveWords/page",
		"HEAD /dashboard/sensitiveWords/page",
	} {
		if !containsString(routes, route) {
			t.Fatalf("route %q missing from %+v", route, routes)
		}
	}
}

func TestRoomRemindPageRouteDispatchWhenConfigured(t *testing.T) {
	page := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go room remind page"))
	})
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		Standalone:   true,
		ProxyTimeout: time.Second,
	}, WithRoomRemindPageHandler(page))
	if err != nil {
		t.Fatal(err)
	}

	for _, method := range []string{http.MethodGet, http.MethodHead} {
		req := httptest.NewRequest(method, "/dashboard/roomRemind/page", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, body=%s", method, rec.Code, rec.Body.String())
		}
		if method == http.MethodGet && rec.Body.String() != "go room remind page" {
			t.Fatalf("body = %q", rec.Body.String())
		}
	}

	routes := srv.migratedRoutes()
	for _, route := range []string{
		"GET /dashboard/roomRemind/page",
		"HEAD /dashboard/roomRemind/page",
	} {
		if !containsString(routes, route) {
			t.Fatalf("route %q missing from %+v", route, routes)
		}
	}
}

func TestRoomQualityPageRouteDispatchWhenConfigured(t *testing.T) {
	page := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go room quality page"))
	})
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		Standalone:   true,
		ProxyTimeout: time.Second,
	}, WithRoomQualityPageHandler(page))
	if err != nil {
		t.Fatal(err)
	}

	for _, method := range []string{http.MethodGet, http.MethodHead} {
		req := httptest.NewRequest(method, "/dashboard/roomQuality/page", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, body=%s", method, rec.Code, rec.Body.String())
		}
		if method == http.MethodGet && rec.Body.String() != "go room quality page" {
			t.Fatalf("body = %q", rec.Body.String())
		}
	}

	routes := srv.migratedRoutes()
	for _, route := range []string{
		"GET /dashboard/roomQuality/page",
		"HEAD /dashboard/roomQuality/page",
	} {
		if !containsString(routes, route) {
			t.Fatalf("route %q missing from %+v", route, routes)
		}
	}
}

func TestRoomCalendarPageRouteDispatchWhenConfigured(t *testing.T) {
	page := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go room calendar page"))
	})
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		Standalone:   true,
		ProxyTimeout: time.Second,
	}, WithRoomCalendarPageHandler(page))
	if err != nil {
		t.Fatal(err)
	}

	for _, method := range []string{http.MethodGet, http.MethodHead} {
		req := httptest.NewRequest(method, "/dashboard/roomCalendar/page", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, body=%s", method, rec.Code, rec.Body.String())
		}
		if method == http.MethodGet && rec.Body.String() != "go room calendar page" {
			t.Fatalf("body = %q", rec.Body.String())
		}
	}

	routes := srv.migratedRoutes()
	for _, route := range []string{
		"GET /dashboard/roomCalendar/page",
		"HEAD /dashboard/roomCalendar/page",
	} {
		if !containsString(routes, route) {
			t.Fatalf("route %q missing from %+v", route, routes)
		}
	}
}

func TestRoomInfinitePullPageRouteDispatchWhenConfigured(t *testing.T) {
	page := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go room infinite pull page"))
	})
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		Standalone:   true,
		ProxyTimeout: time.Second,
	}, WithRoomInfinitePullPageHandler(page))
	if err != nil {
		t.Fatal(err)
	}

	for _, method := range []string{http.MethodGet, http.MethodHead} {
		req := httptest.NewRequest(method, "/dashboard/roomInfinitePull/page", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, body=%s", method, rec.Code, rec.Body.String())
		}
		if method == http.MethodGet && rec.Body.String() != "go room infinite pull page" {
			t.Fatalf("body = %q", rec.Body.String())
		}
	}

	routes := srv.migratedRoutes()
	for _, route := range []string{
		"GET /dashboard/roomInfinitePull/page",
		"HEAD /dashboard/roomInfinitePull/page",
	} {
		if !containsString(routes, route) {
			t.Fatalf("route %q missing from %+v", route, routes)
		}
	}
}

func TestRoomClockInPageRouteDispatchWhenConfigured(t *testing.T) {
	page := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go room clock in page"))
	})
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		Standalone:   true,
		ProxyTimeout: time.Second,
	}, WithRoomClockInPageHandler(page))
	if err != nil {
		t.Fatal(err)
	}

	for _, method := range []string{http.MethodGet, http.MethodHead} {
		req := httptest.NewRequest(method, "/dashboard/roomClockIn/page", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, body=%s", method, rec.Code, rec.Body.String())
		}
		if method == http.MethodGet && rec.Body.String() != "go room clock in page" {
			t.Fatalf("body = %q", rec.Body.String())
		}
	}

	routes := srv.migratedRoutes()
	for _, route := range []string{
		"GET /dashboard/roomClockIn/page",
		"HEAD /dashboard/roomClockIn/page",
	} {
		if !containsString(routes, route) {
			t.Fatalf("route %q missing from %+v", route, routes)
		}
	}
}

func TestLotteryPageRouteDispatchWhenConfigured(t *testing.T) {
	page := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go lottery page"))
	})
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		Standalone:   true,
		ProxyTimeout: time.Second,
	}, WithLotteryPageHandler(page))
	if err != nil {
		t.Fatal(err)
	}

	for _, method := range []string{http.MethodGet, http.MethodHead} {
		req := httptest.NewRequest(method, "/dashboard/lottery/page", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, body=%s", method, rec.Code, rec.Body.String())
		}
		if method == http.MethodGet && rec.Body.String() != "go lottery page" {
			t.Fatalf("body = %q", rec.Body.String())
		}
	}

	routes := srv.migratedRoutes()
	for _, route := range []string{
		"GET /dashboard/lottery/page",
		"HEAD /dashboard/lottery/page",
	} {
		if !containsString(routes, route) {
			t.Fatalf("route %q missing from %+v", route, routes)
		}
	}
}

func TestRadarPageRouteDispatchWhenConfigured(t *testing.T) {
	page := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go radar page"))
	})
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		Standalone:   true,
		ProxyTimeout: time.Second,
	}, WithRadarPageHandler(page))
	if err != nil {
		t.Fatal(err)
	}

	for _, method := range []string{http.MethodGet, http.MethodHead} {
		req := httptest.NewRequest(method, "/dashboard/radar/page", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, body=%s", method, rec.Code, rec.Body.String())
		}
		if method == http.MethodGet && rec.Body.String() != "go radar page" {
			t.Fatalf("body = %q", rec.Body.String())
		}
	}

	routes := srv.migratedRoutes()
	for _, route := range []string{
		"GET /dashboard/radar/page",
		"HEAD /dashboard/radar/page",
	} {
		if !containsString(routes, route) {
			t.Fatalf("route %q missing from %+v", route, routes)
		}
	}
}

func TestRoomFissionPageRouteDispatchWhenConfigured(t *testing.T) {
	page := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go room fission page"))
	})
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		Standalone:   true,
		ProxyTimeout: time.Second,
	}, WithRoomFissionPageHandler(page))
	if err != nil {
		t.Fatal(err)
	}

	for _, method := range []string{http.MethodGet, http.MethodHead} {
		req := httptest.NewRequest(method, "/dashboard/roomFission/page", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, body=%s", method, rec.Code, rec.Body.String())
		}
		if method == http.MethodGet && rec.Body.String() != "go room fission page" {
			t.Fatalf("body = %q", rec.Body.String())
		}
	}

	routes := srv.migratedRoutes()
	for _, route := range []string{
		"GET /dashboard/roomFission/page",
		"HEAD /dashboard/roomFission/page",
	} {
		if !containsString(routes, route) {
			t.Fatalf("route %q missing from %+v", route, routes)
		}
	}
}

func TestShopCodePageRouteDispatchWhenConfigured(t *testing.T) {
	page := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go shop code page"))
	})
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		Standalone:   true,
		ProxyTimeout: time.Second,
	}, WithShopCodePageHandler(page))
	if err != nil {
		t.Fatal(err)
	}

	for _, method := range []string{http.MethodGet, http.MethodHead} {
		req := httptest.NewRequest(method, "/dashboard/shopCode/page", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, body=%s", method, rec.Code, rec.Body.String())
		}
		if method == http.MethodGet && rec.Body.String() != "go shop code page" {
			t.Fatalf("body = %q", rec.Body.String())
		}
	}

	routes := srv.migratedRoutes()
	for _, route := range []string{
		"GET /dashboard/shopCode/page",
		"HEAD /dashboard/shopCode/page",
	} {
		if !containsString(routes, route) {
			t.Fatalf("route %q missing from %+v", route, routes)
		}
	}
}

func TestContactSOPPageRouteDispatchWhenConfigured(t *testing.T) {
	page := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go contact sop page"))
	})
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		Standalone:   true,
		ProxyTimeout: time.Second,
	}, WithContactSOPPageHandler(page))
	if err != nil {
		t.Fatal(err)
	}

	for _, method := range []string{http.MethodGet, http.MethodHead} {
		req := httptest.NewRequest(method, "/dashboard/contactSop/page", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, body=%s", method, rec.Code, rec.Body.String())
		}
		if method == http.MethodGet && rec.Body.String() != "go contact sop page" {
			t.Fatalf("body = %q", rec.Body.String())
		}
	}

	routes := srv.migratedRoutes()
	for _, route := range []string{
		"GET /dashboard/contactSop/page",
		"HEAD /dashboard/contactSop/page",
	} {
		if !containsString(routes, route) {
			t.Fatalf("route %q missing from %+v", route, routes)
		}
	}
}

func TestRoomSOPPageRouteDispatchWhenConfigured(t *testing.T) {
	page := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go room sop page"))
	})
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		Standalone:   true,
		ProxyTimeout: time.Second,
	}, WithRoomSOPPageHandler(page))
	if err != nil {
		t.Fatal(err)
	}

	for _, method := range []string{http.MethodGet, http.MethodHead} {
		req := httptest.NewRequest(method, "/dashboard/roomSop/page", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, body=%s", method, rec.Code, rec.Body.String())
		}
		if method == http.MethodGet && rec.Body.String() != "go room sop page" {
			t.Fatalf("body = %q", rec.Body.String())
		}
	}

	routes := srv.migratedRoutes()
	for _, route := range []string{
		"GET /dashboard/roomSop/page",
		"HEAD /dashboard/roomSop/page",
	} {
		if !containsString(routes, route) {
			t.Fatalf("route %q missing from %+v", route, routes)
		}
	}
}

func TestLoginShowUsesMigratedHandlerWhenConfigured(t *testing.T) {
	migrated := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go loginShow"))
	})
	sourceRoot := t.TempDir()
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		PHPUpstream:  "http://127.0.0.1:9501",
		SourceRoot:   sourceRoot,
		ManifestPath: writeManifest(t),
		ProxyTimeout: time.Second,
	}, WithLoginShowHandler(migrated))
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/dashboard/user/loginShow", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "go loginShow" {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestWorkFissionRoutesUseMigratedHandlersWhenConfigured(t *testing.T) {
	index := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go workFission index"))
	})
	destroy := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go workFission destroy"))
	})
	sourceRoot := t.TempDir()
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		PHPUpstream:  "http://127.0.0.1:9501",
		SourceRoot:   sourceRoot,
		ManifestPath: writeManifest(t),
		ProxyTimeout: time.Second,
	}, WithWorkFissionIndexHandler(index), WithWorkFissionDestroyHandler(destroy))
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/dashboard/workFission/index", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "go workFission index" {
		t.Fatalf("index status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "/dashboard/workFission/destroy", nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "go workFission destroy" {
		t.Fatalf("destroy status=%d body=%q", rec.Code, rec.Body.String())
	}
	if !containsString(srv.migratedRoutes(), "GET /dashboard/workFission/index") || !containsString(srv.migratedRoutes(), "DELETE /dashboard/workFission/destroy") {
		t.Fatalf("missing workFission routes: %v", srv.migratedRoutes())
	}
}

func TestOperationWorkFissionFrontendAliasesUseMigratedHandlers(t *testing.T) {
	auth := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go operation auth " + r.URL.RequestURI()))
	})
	openUserInfo := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go operation open user " + r.URL.RequestURI()))
	})
	sourceRoot := t.TempDir()
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		PHPUpstream:  "http://127.0.0.1:9501",
		SourceRoot:   sourceRoot,
		ManifestPath: writeManifest(t),
		ProxyTimeout: time.Second,
	}, WithOperationWorkFissionAuthHandler(auth), WithOperationWorkFissionOpenUserInfoHandler(openUserInfo))
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		path string
		want string
	}{
		{path: "/operation/auth/workFission?id=9&target=%2FworkFission%3Fid%3D9", want: "go operation auth /operation/auth/workFission?id=9&target=%2FworkFission%3Fid%3D9"},
		{path: "/auth/workFission?id=9&target=%2FworkFission%3Fid%3D9", want: "go operation auth /auth/workFission?id=9&target=%2FworkFission%3Fid%3D9"},
		{path: "/operation/openUserInfo/workFission?id=9", want: "go operation open user /operation/openUserInfo/workFission?id=9"},
		{path: "/openUserInfo/workFission?id=9", want: "go operation open user /openUserInfo/workFission?id=9"},
	} {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status=%d body=%q", tc.path, rec.Code, rec.Body.String())
		}
		if rec.Body.String() != tc.want {
			t.Fatalf("%s body=%q want=%q", tc.path, rec.Body.String(), tc.want)
		}
	}
}

func TestLogoutFallsBackWhenNotMigrated(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/dashboard/user/logout" {
			t.Fatalf("unexpected upstream path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte("php logout"))
	}))
	defer upstream.Close()

	srv := newTestServer(t, upstream.URL)
	req := httptest.NewRequest(http.MethodPut, "/dashboard/user/logout", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "php logout" {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestLogoutUsesMigratedHandlerWhenConfigured(t *testing.T) {
	migrated := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go logout"))
	})
	sourceRoot := t.TempDir()
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		PHPUpstream:  "http://127.0.0.1:9501",
		SourceRoot:   sourceRoot,
		ManifestPath: writeManifest(t),
		ProxyTimeout: time.Second,
	}, WithLogoutHandler(migrated))
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPut, "/dashboard/user/logout", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "go logout" {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestUserAdminUsesMigratedHandlersWhenConfigured(t *testing.T) {
	sourceRoot := t.TempDir()
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		PHPUpstream:  "http://127.0.0.1:9501",
		SourceRoot:   sourceRoot,
		ManifestPath: writeManifest(t),
		ProxyTimeout: time.Second,
	},
		WithUserIndexHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go user index"))
		})),
		WithUserShowHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go user show"))
		})),
		WithUserStoreHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go user store"))
		})),
		WithUserUpdateHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go user update"))
		})),
		WithUserStatusUpdateHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go user status"))
		})),
		WithUserPasswordResetHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go user reset"))
		})),
		WithUserPasswordUpdateHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go user password"))
		})),
	)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodGet, path: "/dashboard/user/index", body: "go user index"},
		{method: http.MethodGet, path: "/dashboard/user/show", body: "go user show"},
		{method: http.MethodPost, path: "/dashboard/user/store", body: "go user store"},
		{method: http.MethodPut, path: "/dashboard/user/update", body: "go user update"},
		{method: http.MethodPut, path: "/dashboard/user/statusUpdate", body: "go user status"},
		{method: http.MethodPut, path: "/dashboard/user/passwordReset", body: "go user reset"},
		{method: http.MethodPut, path: "/dashboard/user/passwordUpdate", body: "go user password"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s %s status = %d, want %d", tc.method, tc.path, rec.Code, http.StatusOK)
		}
		if rec.Body.String() != tc.body {
			t.Fatalf("%s %s body = %q", tc.method, tc.path, rec.Body.String())
		}
	}
}

func TestStatisticUsesMigratedHandlersWhenConfigured(t *testing.T) {
	sourceRoot := t.TempDir()
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		PHPUpstream:  "http://127.0.0.1:9501",
		SourceRoot:   sourceRoot,
		ManifestPath: writeManifest(t),
		ProxyTimeout: time.Second,
	},
		WithStatisticIndexHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go statistic index"))
		})),
		WithStatisticTopListHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go statistic top"))
		})),
		WithStatisticEmployeeCountsHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go statistic counts"))
		})),
		WithStatisticEmployeesHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go statistic employees"))
		})),
		WithStatisticEmployeesTrendHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go statistic trend"))
		})),
	)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		path string
		body string
	}{
		{path: "/dashboard/statistic/index", body: "go statistic index"},
		{path: "/dashboard/statistic/topList", body: "go statistic top"},
		{path: "/dashboard/statistic/employeeCounts", body: "go statistic counts"},
		{path: "/dashboard/statistic/employees", body: "go statistic employees"},
		{path: "/dashboard/statistic/employeesTrend", body: "go statistic trend"},
	} {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want %d", tc.path, rec.Code, http.StatusOK)
		}
		if rec.Body.String() != tc.body {
			t.Fatalf("%s body = %q", tc.path, rec.Body.String())
		}
	}
}

func TestCommonUploadUsesMigratedHandlersWhenConfigured(t *testing.T) {
	sourceRoot := t.TempDir()
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		PHPUpstream:  "http://127.0.0.1:9501",
		SourceRoot:   sourceRoot,
		ManifestPath: writeManifest(t),
		ProxyTimeout: time.Second,
	},
		WithCommonUploadHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go common upload"))
		})),
		WithCommonUploadFileHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go common uploadFile"))
		})),
		WithSidebarCommonUploadHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go sidebar upload"))
		})),
	)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		path string
		body string
	}{
		{path: "/dashboard/common/upload", body: "go common upload"},
		{path: "/dashboard/common/uploadFile", body: "go common uploadFile"},
		{path: "/sidebar/common/upload", body: "go sidebar upload"},
	} {
		req := httptest.NewRequest(http.MethodPost, tc.path, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want %d", tc.path, rec.Code, http.StatusOK)
		}
		if rec.Body.String() != tc.body {
			t.Fatalf("%s body = %q", tc.path, rec.Body.String())
		}
	}
}

func TestPermissionByUserFallsBackWhenNotMigrated(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/dashboard/role/permissionByUser" {
			t.Fatalf("unexpected upstream path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte("php permission"))
	}))
	defer upstream.Close()

	srv := newTestServer(t, upstream.URL)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/role/permissionByUser", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "php permission" {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestPermissionByUserUsesMigratedHandlerWhenConfigured(t *testing.T) {
	migrated := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go permission"))
	})
	sourceRoot := t.TempDir()
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		PHPUpstream:  "http://127.0.0.1:9501",
		SourceRoot:   sourceRoot,
		ManifestPath: writeManifest(t),
		ProxyTimeout: time.Second,
	}, WithPermissionByUserHandler(migrated))
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/dashboard/role/permissionByUser", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "go permission" {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestCorpSelectFallsBackWhenNotMigrated(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/dashboard/corp/select" {
			t.Fatalf("unexpected upstream path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte("php corp select"))
	}))
	defer upstream.Close()

	srv := newTestServer(t, upstream.URL)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/corp/select", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "php corp select" {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestCorpSelectUsesMigratedHandlerWhenConfigured(t *testing.T) {
	migrated := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go corp select"))
	})
	sourceRoot := t.TempDir()
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		PHPUpstream:  "http://127.0.0.1:9501",
		SourceRoot:   sourceRoot,
		ManifestPath: writeManifest(t),
		ProxyTimeout: time.Second,
	}, WithCorpSelectHandler(migrated))
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/dashboard/corp/select", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "go corp select" {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestCorpBindFallsBackWhenNotMigrated(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/dashboard/corp/bind" {
			t.Fatalf("unexpected upstream path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte("php corp bind"))
	}))
	defer upstream.Close()

	srv := newTestServer(t, upstream.URL)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/corp/bind", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "php corp bind" {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestCorpBindUsesMigratedHandlerWhenConfigured(t *testing.T) {
	migrated := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go corp bind"))
	})
	sourceRoot := t.TempDir()
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		PHPUpstream:  "http://127.0.0.1:9501",
		SourceRoot:   sourceRoot,
		ManifestPath: writeManifest(t),
		ProxyTimeout: time.Second,
	}, WithCorpBindHandler(migrated))
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/dashboard/corp/bind", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "go corp bind" {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestCorpAdminRoutesUseMigratedHandlersWhenConfigured(t *testing.T) {
	sourceRoot := t.TempDir()
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		PHPUpstream:  "http://127.0.0.1:9501",
		SourceRoot:   sourceRoot,
		ManifestPath: writeManifest(t),
		ProxyTimeout: time.Second,
	},
		WithCorpIndexHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go corp index"))
		})),
		WithCorpShowHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go corp show"))
		})),
		WithCorpStoreHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go corp store"))
		})),
		WithCorpUpdateHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go corp update"))
		})),
		WithWeWorkCallbackHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go wework callback"))
		})),
	)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodGet, path: "/dashboard/corp/index", body: "go corp index"},
		{method: http.MethodGet, path: "/dashboard/corp/show", body: "go corp show"},
		{method: http.MethodPost, path: "/dashboard/corp/store", body: "go corp store"},
		{method: http.MethodPut, path: "/dashboard/corp/update", body: "go corp update"},
		{method: http.MethodGet, path: "/dashboard/corp/weWorkCallback", body: "go wework callback"},
		{method: http.MethodPost, path: "/dashboard/corp/weWorkCallback", body: "go wework callback"},
		{method: http.MethodGet, path: "/weWork/callback", body: "go wework callback"},
		{method: http.MethodPost, path: "/weWork/callback", body: "go wework callback"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("%s %s status = %d", tc.method, tc.path, rec.Code)
		}
		if rec.Body.String() != tc.body {
			t.Fatalf("%s %s body = %q", tc.method, tc.path, rec.Body.String())
		}
	}
}

func TestCorpDataRoutesFallBackWhenNotMigrated(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/dashboard/corpData/index":
			_, _ = w.Write([]byte("php corp data index"))
		case "/dashboard/corpData/lineChat":
			_, _ = w.Write([]byte("php corp data line"))
		default:
			t.Fatalf("unexpected upstream path: %s", r.URL.Path)
		}
	}))
	defer upstream.Close()

	srv := newTestServer(t, upstream.URL)
	for _, tc := range []struct {
		path string
		body string
	}{
		{path: "/dashboard/corpData/index", body: "php corp data index"},
		{path: "/dashboard/corpData/lineChat", body: "php corp data line"},
	} {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d", tc.path, rec.Code)
		}
		if rec.Body.String() != tc.body {
			t.Fatalf("%s body = %q", tc.path, rec.Body.String())
		}
	}
}

func TestCorpDataRoutesUseMigratedHandlersWhenConfigured(t *testing.T) {
	sourceRoot := t.TempDir()
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		PHPUpstream:  "http://127.0.0.1:9501",
		SourceRoot:   sourceRoot,
		ManifestPath: writeManifest(t),
		ProxyTimeout: time.Second,
	},
		WithCorpDataIndexHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(r.URL.RequestURI()))
		})),
		WithCorpDataLineChatHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go corp data line"))
		})),
	)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		path string
		body string
	}{
		{
			path: "/dashboard/corpData/index?corpId=5&from=2026-07-01&to=2026-07-31",
			body: "/dashboard/corpData/index?corpId=5&from=2026-07-01&to=2026-07-31",
		},
		{path: "/dashboard/corpData/lineChat", body: "go corp data line"},
	} {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d", tc.path, rec.Code)
		}
		if rec.Body.String() != tc.body {
			t.Fatalf("%s body = %q", tc.path, rec.Body.String())
		}
	}
}

func TestWorkReadRoutesFallBackWhenNotMigrated(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/dashboard/workEmployee/index":
			_, _ = w.Write([]byte("php employee index"))
		case "/dashboard/workEmployee/searchCondition":
			_, _ = w.Write([]byte("php employee search condition"))
		case "/dashboard/workDepartment/index":
			_, _ = w.Write([]byte("php work department index"))
		case "/dashboard/workEmployeeDepartment/memberIndex":
			_, _ = w.Write([]byte("php work department members"))
		case "/dashboard/workDepartment/selectByPhone":
			_, _ = w.Write([]byte("php work department select by phone"))
		case "/dashboard/workDepartment/pageIndex":
			_, _ = w.Write([]byte("php work department page index"))
		case "/dashboard/workDepartment/showEmployee":
			_, _ = w.Write([]byte("php work department show employee"))
		case "/dashboard/workContactTagGroup/index":
			_, _ = w.Write([]byte("php work contact tag group index"))
		case "/dashboard/workContactTagGroup/detail":
			_, _ = w.Write([]byte("php work contact tag group detail"))
		case "/sidebar/workContactTagGroup/index":
			_, _ = w.Write([]byte("php sidebar work contact tag group index"))
		case "/dashboard/workContactTag/index":
			_, _ = w.Write([]byte("php work contact tag index"))
		case "/dashboard/workContactTag/detail":
			_, _ = w.Write([]byte("php work contact tag detail"))
		case "/dashboard/workContactTag/contactTagList":
			_, _ = w.Write([]byte("php work contact tag list"))
		case "/dashboard/workContactTag/allTag":
			_, _ = w.Write([]byte("php work contact tag all"))
		case "/dashboard/workContactTag/synContactTag":
			_, _ = w.Write([]byte("php work contact tag sync"))
		case "/dashboard/workContact/synContact":
			_, _ = w.Write([]byte("php work contact sync"))
		case "/dashboard/workContact/source":
			_, _ = w.Write([]byte("php work contact source"))
		case "/dashboard/workContact/show":
			_, _ = w.Write([]byte("php work contact show"))
		case "/dashboard/workContact/track":
			_, _ = w.Write([]byte("php work contact track"))
		case "/dashboard/workContact/update":
			_, _ = w.Write([]byte("php work contact update"))
		case "/dashboard/workContact/batchLabeling":
			_, _ = w.Write([]byte("php work contact batch labeling"))
		case "/dashboard/workContactRoom/index":
			_, _ = w.Write([]byte("php work contact room index"))
		case "/dashboard/workRoom/roomIndex":
			_, _ = w.Write([]byte("php work room room index"))
		case "/dashboard/workRoom/statistics":
			_, _ = w.Write([]byte("php work room statistics"))
		case "/dashboard/workRoom/statisticsIndex":
			_, _ = w.Write([]byte("php work room statistics index"))
		case "/dashboard/workRoom/syn":
			_, _ = w.Write([]byte("php work room sync"))
		case "/dashboard/workRoom/batchUpdate":
			_, _ = w.Write([]byte("php work room batch update"))
		case "/sidebar/workContactTag/allTag":
			_, _ = w.Write([]byte("php sidebar work contact tag all"))
		case "/sidebar/workContact/detail":
			_, _ = w.Write([]byte("php sidebar work contact detail"))
		case "/sidebar/workContact/show":
			_, _ = w.Write([]byte("php sidebar work contact show"))
		case "/sidebar/workContact/track":
			_, _ = w.Write([]byte("php sidebar work contact track"))
		case "/sidebar/workContact/update":
			_, _ = w.Write([]byte("php sidebar work contact update"))
		case "/sidebar/contactProcessStatus/index":
			_, _ = w.Write([]byte("php sidebar contact process status index"))
		case "/sidebar/contactProcessStatus/update":
			_, _ = w.Write([]byte("php sidebar contact process status update"))
		case "/dashboard/contactField/index":
			_, _ = w.Write([]byte("php contact field index"))
		case "/dashboard/contactField/show":
			_, _ = w.Write([]byte("php contact field show"))
		case "/dashboard/contactField/portrait":
			_, _ = w.Write([]byte("php contact field portrait"))
		case "/dashboard/contactFieldPivot/index":
			_, _ = w.Write([]byte("php contact field pivot index"))
		case "/dashboard/contactFieldPivot/update":
			_, _ = w.Write([]byte("php contact field pivot update"))
		case "/sidebar/contactFieldPivot/index":
			_, _ = w.Write([]byte("php sidebar contact field pivot index"))
		case "/sidebar/contactFieldPivot/update":
			_, _ = w.Write([]byte("php sidebar contact field pivot update"))
		default:
			t.Fatalf("unexpected upstream path: %s", r.URL.Path)
		}
	}))
	defer upstream.Close()

	srv := newTestServer(t, upstream.URL)
	for _, tc := range []struct {
		path string
		body string
	}{
		{path: "/dashboard/workEmployee/index", body: "php employee index"},
		{path: "/dashboard/workEmployee/searchCondition", body: "php employee search condition"},
		{path: "/dashboard/workDepartment/index", body: "php work department index"},
		{path: "/dashboard/workEmployeeDepartment/memberIndex", body: "php work department members"},
		{path: "/dashboard/workDepartment/selectByPhone", body: "php work department select by phone"},
		{path: "/dashboard/workDepartment/pageIndex", body: "php work department page index"},
		{path: "/dashboard/workDepartment/showEmployee", body: "php work department show employee"},
		{path: "/dashboard/workContactTagGroup/index", body: "php work contact tag group index"},
		{path: "/dashboard/workContactTagGroup/detail", body: "php work contact tag group detail"},
		{path: "/sidebar/workContactTagGroup/index", body: "php sidebar work contact tag group index"},
		{path: "/dashboard/workContactTag/index", body: "php work contact tag index"},
		{path: "/dashboard/workContactTag/detail", body: "php work contact tag detail"},
		{path: "/dashboard/workContactTag/contactTagList", body: "php work contact tag list"},
		{path: "/dashboard/workContactTag/allTag", body: "php work contact tag all"},
		{path: "/dashboard/workContact/source", body: "php work contact source"},
		{path: "/dashboard/workContact/show", body: "php work contact show"},
		{path: "/dashboard/workContact/track", body: "php work contact track"},
		{path: "/dashboard/workContactRoom/index", body: "php work contact room index"},
		{path: "/dashboard/workRoom/roomIndex", body: "php work room room index"},
		{path: "/dashboard/workRoom/statistics", body: "php work room statistics"},
		{path: "/dashboard/workRoom/statisticsIndex", body: "php work room statistics index"},
		{path: "/sidebar/workContactTag/allTag", body: "php sidebar work contact tag all"},
		{path: "/sidebar/workContact/detail", body: "php sidebar work contact detail"},
		{path: "/sidebar/workContact/show", body: "php sidebar work contact show"},
		{path: "/sidebar/workContact/track", body: "php sidebar work contact track"},
		{path: "/sidebar/contactProcessStatus/index", body: "php sidebar contact process status index"},
		{path: "/dashboard/contactField/index", body: "php contact field index"},
		{path: "/dashboard/contactField/show", body: "php contact field show"},
		{path: "/dashboard/contactField/portrait", body: "php contact field portrait"},
		{path: "/dashboard/contactFieldPivot/index", body: "php contact field pivot index"},
		{path: "/sidebar/contactFieldPivot/index", body: "php sidebar contact field pivot index"},
	} {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d", tc.path, rec.Code)
		}
		if rec.Body.String() != tc.body {
			t.Fatalf("%s body = %q", tc.path, rec.Body.String())
		}
	}
	req := httptest.NewRequest(http.MethodPut, "/sidebar/contactProcessStatus/update", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("process status update status = %d", rec.Code)
	}
	if rec.Body.String() != "php sidebar contact process status update" {
		t.Fatalf("process status update body = %q", rec.Body.String())
	}
	for _, tc := range []struct {
		path string
		body string
	}{
		{path: "/dashboard/contactFieldPivot/update", body: "php contact field pivot update"},
		{path: "/dashboard/workContactTag/synContactTag", body: "php work contact tag sync"},
		{path: "/dashboard/workContact/synContact", body: "php work contact sync"},
		{path: "/dashboard/workContact/update", body: "php work contact update"},
		{path: "/dashboard/workRoom/syn", body: "php work room sync"},
		{path: "/dashboard/workRoom/batchUpdate", body: "php work room batch update"},
		{path: "/sidebar/workContact/update", body: "php sidebar work contact update"},
		{path: "/sidebar/contactFieldPivot/update", body: "php sidebar contact field pivot update"},
	} {
		req := httptest.NewRequest(http.MethodPut, tc.path, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d", tc.path, rec.Code)
		}
		if rec.Body.String() != tc.body {
			t.Fatalf("%s body = %q", tc.path, rec.Body.String())
		}
	}
	req = httptest.NewRequest(http.MethodPost, "/dashboard/workContact/batchLabeling", nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("batch labeling status = %d", rec.Code)
	}
	if rec.Body.String() != "php work contact batch labeling" {
		t.Fatalf("batch labeling body = %q", rec.Body.String())
	}
}

func TestWorkReadRoutesUseMigratedHandlersWhenConfigured(t *testing.T) {
	sourceRoot := t.TempDir()
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		PHPUpstream:  "http://127.0.0.1:9501",
		SourceRoot:   sourceRoot,
		ManifestPath: writeManifest(t),
		ProxyTimeout: time.Second,
	},
		WithWorkEmployeeIndexHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go employee index"))
		})),
		WithWorkEmployeeSearchConditionHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go employee search condition"))
		})),
		WithWorkEmployeeSyncHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go employee sync"))
		})),
		WithWorkDepartmentIndexHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go work department index"))
		})),
		WithWorkEmployeeDepartmentMemberIndexHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go work department members"))
		})),
		WithWorkDepartmentSelectByPhoneHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go work department select by phone"))
		})),
		WithWorkDepartmentPageIndexHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go work department page index"))
		})),
		WithWorkDepartmentShowEmployeeHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go work department show employee"))
		})),
		WithWorkContactTagGroupIndexHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go work contact tag group index"))
		})),
		WithWorkContactTagGroupDetailHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go work contact tag group detail"))
		})),
		WithSidebarWorkContactTagGroupIndexHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go sidebar work contact tag group index"))
		})),
		WithWorkContactTagIndexHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go work contact tag index"))
		})),
		WithWorkContactTagDetailHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go work contact tag detail"))
		})),
		WithWorkContactTagListHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go work contact tag list"))
		})),
		WithWorkContactTagAllHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go work contact tag all"))
		})),
		WithWorkContactTagSyncHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go work contact tag sync"))
		})),
		WithWorkContactSyncHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go work contact sync"))
		})),
		WithWorkContactIndexHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go work contact index"))
		})),
		WithWorkContactLossHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go work contact loss"))
		})),
		WithWorkContactSourceHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go work contact source"))
		})),
		WithWorkContactShowHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go work contact show"))
		})),
		WithWorkContactTrackHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go work contact track"))
		})),
		WithWorkContactUpdateHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go work contact update"))
		})),
		WithWorkContactBatchLabelingHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go work contact batch labeling"))
		})),
		WithWorkContactRoomIndexHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go work contact room index"))
		})),
		WithWorkRoomIndexHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go work room index"))
		})),
		WithWorkRoomRoomIndexHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go work room room index"))
		})),
		WithWorkRoomStatisticsHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go work room statistics"))
		})),
		WithWorkRoomStatisticsIndexHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go work room statistics index"))
		})),
		WithWorkRoomSyncHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go work room sync"))
		})),
		WithWorkRoomBatchUpdateHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go work room batch update"))
		})),
		WithSidebarWorkRoomManageHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go sidebar work room manage"))
		})),
		WithWorkRoomAutoPullIndexHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go work room auto pull index"))
		})),
		WithWorkRoomAutoPullShowHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go work room auto pull show"))
		})),
		WithWorkRoomAutoPullStoreHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go work room auto pull store"))
		})),
		WithWorkRoomAutoPullUpdateHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go work room auto pull update"))
		})),
		WithWorkRoomAutoPullMoveHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go work room auto pull move"))
		})),
		WithRoomTagPullIndexHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room tag pull index"))
		})),
		WithRoomTagPullShowHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room tag pull show"))
		})),
		WithRoomTagPullShowContactHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room tag pull show contact"))
		})),
		WithRoomTagPullRoomListHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room tag pull room list"))
		})),
		WithRoomTagPullChooseContactHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room tag pull choose contact"))
		})),
		WithRoomTagPullStoreHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room tag pull store"))
		})),
		WithRoomTagPullFilterContactHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room tag pull filter contact"))
		})),
		WithRoomTagPullRemindSendHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room tag pull remind send"))
		})),
		WithRoomTagPullDestroyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room tag pull destroy"))
		})),
		WithRoomTagPullContactDetailPageHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room tag pull contact detail"))
		})),
		WithContactMessageBatchSendIndexHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go contact message batch send index"))
		})),
		WithContactMessageBatchSendShowHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go contact message batch send show"))
		})),
		WithContactMessageBatchSendMessageShowHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go contact message batch send message show"))
		})),
		WithContactMessageBatchSendShowRoomHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go contact message batch send show room"))
		})),
		WithContactMessageBatchSendEmployeeSendIndexHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go contact message batch send employee"))
		})),
		WithContactMessageBatchSendContactReceiveIndexHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go contact message batch send contact receive"))
		})),
		WithContactMessageBatchSendStoreHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go contact message batch send store"))
		})),
		WithContactMessageBatchSendRemindHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go contact message batch send remind"))
		})),
		WithContactMessageBatchSendDestroyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go contact message batch send destroy"))
		})),
		WithRoomMessageBatchSendIndexHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room message batch send index"))
		})),
		WithRoomMessageBatchSendShowHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room message batch send show"))
		})),
		WithRoomMessageBatchSendRoomOwnerSendIndexHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room message batch send owner"))
		})),
		WithRoomMessageBatchSendRoomReceiveIndexHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room message batch send room receive"))
		})),
		WithRoomMessageBatchSendStoreHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room message batch send store"))
		})),
		WithRoomMessageBatchSendRemindHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room message batch send remind"))
		})),
		WithRoomMessageBatchSendDestroyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room message batch send destroy"))
		})),
		WithOfficialAccountIndexHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go official account index"))
		})),
		WithOfficialAccountSetHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go official account set"))
		})),
		WithSidebarWorkContactTagAllHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go sidebar work contact tag all"))
		})),
		WithSidebarWorkContactDetailHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go sidebar work contact detail"))
		})),
		WithSidebarWorkContactShowHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go sidebar work contact show"))
		})),
		WithSidebarWorkContactTrackHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go sidebar work contact track"))
		})),
		WithSidebarWorkContactUpdateHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go sidebar work contact update"))
		})),
		WithSidebarContactProcessStatusIndexHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go sidebar contact process status index"))
		})),
		WithSidebarContactProcessStatusUpdateHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go sidebar contact process status update"))
		})),
		WithContactFieldIndexHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go contact field index"))
		})),
		WithContactFieldShowHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go contact field show"))
		})),
		WithContactFieldPortraitHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go contact field portrait"))
		})),
		WithContactFieldPivotIndexHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go contact field pivot index"))
		})),
		WithContactFieldPivotUpdateHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go contact field pivot update"))
		})),
		WithSidebarContactFieldPivotIndexHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go sidebar contact field pivot index"))
		})),
		WithSidebarContactFieldPivotUpdateHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go sidebar contact field pivot update"))
		})),
	)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		path string
		body string
	}{
		{path: "/dashboard/workEmployee/index", body: "go employee index"},
		{path: "/dashboard/workEmployee/searchCondition", body: "go employee search condition"},
		{path: "/dashboard/workDepartment/index", body: "go work department index"},
		{path: "/dashboard/workEmployeeDepartment/memberIndex", body: "go work department members"},
		{path: "/dashboard/workDepartment/memberIndex", body: "go work department members"},
		{path: "/dashboard/workDepartment/selectByPhone", body: "go work department select by phone"},
		{path: "/dashboard/workDepartment/pageIndex", body: "go work department page index"},
		{path: "/dashboard/workDepartment/showEmployee", body: "go work department show employee"},
		{path: "/dashboard/workContactTagGroup/index", body: "go work contact tag group index"},
		{path: "/dashboard/workContactTagGroup/detail", body: "go work contact tag group detail"},
		{path: "/sidebar/workContactTagGroup/index", body: "go sidebar work contact tag group index"},
		{path: "/dashboard/workContactTag/index", body: "go work contact tag index"},
		{path: "/dashboard/workContactTag/detail", body: "go work contact tag detail"},
		{path: "/dashboard/workContactTag/contactTagList", body: "go work contact tag list"},
		{path: "/dashboard/workContactTag/allTag", body: "go work contact tag all"},
		{path: "/dashboard/workContact/index", body: "go work contact index"},
		{path: "/dashboard/workContact/lossContact", body: "go work contact loss"},
		{path: "/dashboard/workContact/source", body: "go work contact source"},
		{path: "/dashboard/workContact/show", body: "go work contact show"},
		{path: "/dashboard/workContact/track", body: "go work contact track"},
		{path: "/dashboard/workContactRoom/index", body: "go work contact room index"},
		{path: "/dashboard/workRoom/index", body: "go work room index"},
		{path: "/dashboard/workRoom/roomIndex", body: "go work room room index"},
		{path: "/dashboard/workRoom/statistics", body: "go work room statistics"},
		{path: "/dashboard/workRoom/statisticsIndex", body: "go work room statistics index"},
		{path: "/sidebar/workRoom/roomManage", body: "go sidebar work room manage"},
		{path: "/dashboard/workRoomAutoPull/index", body: "go work room auto pull index"},
		{path: "/dashboard/workRoomAutoPull/show", body: "go work room auto pull show"},
		{path: "/dashboard/roomTagPull/index", body: "go room tag pull index"},
		{path: "/dashboard/roomTagPull/show", body: "go room tag pull show"},
		{path: "/dashboard/roomTagPull/showContact", body: "go room tag pull show contact"},
		{path: "/dashboard/roomTagPull/contactDetail", body: "go room tag pull contact detail"},
		{path: "/roomTagPull/contactDetail", body: "go room tag pull contact detail"},
		{path: "/roomTagPull/clientDetails", body: "go room tag pull contact detail"},
		{path: "/dashboard/roomTagPull/roomList", body: "go room tag pull room list"},
		{path: "/dashboard/roomTagPull/chooseContact", body: "go room tag pull choose contact"},
		{path: "/dashboard/roomTagPull/remindSend", body: "go room tag pull remind send"},
		{path: "/dashboard/contactMessageBatchSend/index", body: "go contact message batch send index"},
		{path: "/dashboard/contactMessageBatchSend/show", body: "go contact message batch send show"},
		{path: "/dashboard/contactMessageBatchSend/messageShow", body: "go contact message batch send message show"},
		{path: "/dashboard/contactMessageBatchSend/showRoom", body: "go contact message batch send show room"},
		{path: "/dashboard/contactMessageBatchSend/employeeSendIndex", body: "go contact message batch send employee"},
		{path: "/dashboard/contactMessageBatchSend/contactReceiveIndex", body: "go contact message batch send contact receive"},
		{path: "/dashboard/roomMessageBatchSend/index", body: "go room message batch send index"},
		{path: "/dashboard/roomMessageBatchSend/show", body: "go room message batch send show"},
		{path: "/dashboard/roomMessageBatchSend/roomOwnerSendIndex", body: "go room message batch send owner"},
		{path: "/dashboard/roomMessageBatchSend/roomReceiveIndex", body: "go room message batch send room receive"},
		{path: "/dashboard/officialAccount/index", body: "go official account index"},
		{path: "/dashboard/officialAccount/set", body: "go official account set"},
		{path: "/sidebar/workContactTag/allTag", body: "go sidebar work contact tag all"},
		{path: "/sidebar/workContact/detail", body: "go sidebar work contact detail"},
		{path: "/sidebar/workContact/show", body: "go sidebar work contact show"},
		{path: "/sidebar/workContact/track", body: "go sidebar work contact track"},
		{path: "/sidebar/contactProcessStatus/index", body: "go sidebar contact process status index"},
		{path: "/dashboard/contactField/index", body: "go contact field index"},
		{path: "/dashboard/contactField/show", body: "go contact field show"},
		{path: "/dashboard/contactField/portrait", body: "go contact field portrait"},
		{path: "/dashboard/contactFieldPivot/index", body: "go contact field pivot index"},
		{path: "/sidebar/contactFieldPivot/index", body: "go sidebar contact field pivot index"},
	} {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d", tc.path, rec.Code)
		}
		if rec.Body.String() != tc.body {
			t.Fatalf("%s body = %q", tc.path, rec.Body.String())
		}
	}
	req := httptest.NewRequest(http.MethodPut, "/sidebar/contactProcessStatus/update", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("process status update status = %d", rec.Code)
	}
	if rec.Body.String() != "go sidebar contact process status update" {
		t.Fatalf("process status update body = %q", rec.Body.String())
	}
	for _, tc := range []struct {
		path string
		body string
	}{
		{path: "/dashboard/contactFieldPivot/update", body: "go contact field pivot update"},
		{path: "/dashboard/workEmployee/synEmployee", body: "go employee sync"},
		{path: "/dashboard/workContactTag/synContactTag", body: "go work contact tag sync"},
		{path: "/dashboard/workContact/synContact", body: "go work contact sync"},
		{path: "/dashboard/workContact/update", body: "go work contact update"},
		{path: "/dashboard/workRoom/syn", body: "go work room sync"},
		{path: "/dashboard/workRoom/batchUpdate", body: "go work room batch update"},
		{path: "/dashboard/workRoomAutoPull/update", body: "go work room auto pull update"},
		{path: "/dashboard/workRoomAutoPull/move", body: "go work room auto pull move"},
		{path: "/sidebar/workContact/update", body: "go sidebar work contact update"},
		{path: "/sidebar/contactFieldPivot/update", body: "go sidebar contact field pivot update"},
	} {
		req := httptest.NewRequest(http.MethodPut, tc.path, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d", tc.path, rec.Code)
		}
		if rec.Body.String() != tc.body {
			t.Fatalf("%s body = %q", tc.path, rec.Body.String())
		}
	}
	req = httptest.NewRequest(http.MethodPost, "/dashboard/workContact/batchLabeling", nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("batch labeling status = %d", rec.Code)
	}
	if rec.Body.String() != "go work contact batch labeling" {
		t.Fatalf("batch labeling body = %q", rec.Body.String())
	}
	req = httptest.NewRequest(http.MethodPost, "/dashboard/workRoomAutoPull/store", nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("auto pull store status = %d", rec.Code)
	}
	if rec.Body.String() != "go work room auto pull store" {
		t.Fatalf("auto pull store body = %q", rec.Body.String())
	}
	for _, tc := range []struct {
		path string
		body string
	}{
		{path: "/dashboard/roomTagPull/store", body: "go room tag pull store"},
		{path: "/dashboard/roomTagPull/filterContact", body: "go room tag pull filter contact"},
		{path: "/dashboard/contactMessageBatchSend/store", body: "go contact message batch send store"},
		{path: "/dashboard/contactMessageBatchSend/remind", body: "go contact message batch send remind"},
		{path: "/dashboard/roomMessageBatchSend/store", body: "go room message batch send store"},
	} {
		req = httptest.NewRequest(http.MethodPost, tc.path, nil)
		rec = httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d", tc.path, rec.Code)
		}
		if rec.Body.String() != tc.body {
			t.Fatalf("%s body = %q", tc.path, rec.Body.String())
		}
	}
	req = httptest.NewRequest(http.MethodDelete, "/dashboard/roomTagPull/destroy", nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("room tag pull destroy status = %d", rec.Code)
	}
	if rec.Body.String() != "go room tag pull destroy" {
		t.Fatalf("room tag pull destroy body = %q", rec.Body.String())
	}
	req = httptest.NewRequest(http.MethodDelete, "/dashboard/contactMessageBatchSend/destroy", nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("contact message batch send destroy status = %d", rec.Code)
	}
	if rec.Body.String() != "go contact message batch send destroy" {
		t.Fatalf("contact message batch send destroy body = %q", rec.Body.String())
	}
	req = httptest.NewRequest(http.MethodGet, "/dashboard/roomMessageBatchSend/remind", nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("room message batch send remind status = %d", rec.Code)
	}
	if rec.Body.String() != "go room message batch send remind" {
		t.Fatalf("room message batch send remind body = %q", rec.Body.String())
	}
	req = httptest.NewRequest(http.MethodDelete, "/dashboard/roomMessageBatchSend/destroy", nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("room message batch send destroy status = %d", rec.Code)
	}
	if rec.Body.String() != "go room message batch send destroy" {
		t.Fatalf("room message batch send destroy body = %q", rec.Body.String())
	}
}

func TestChatToolConfigFallsBackWhenNotMigrated(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/dashboard/chatTool/config" {
			t.Fatalf("unexpected upstream path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte("php chat tool config"))
	}))
	defer upstream.Close()

	srv := newTestServer(t, upstream.URL)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/chatTool/config", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "php chat tool config" {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestChatToolConfigUsesMigratedHandlerWhenConfigured(t *testing.T) {
	migrated := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go chat tool config"))
	})
	sourceRoot := t.TempDir()
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		PHPUpstream:  "http://127.0.0.1:9501",
		SourceRoot:   sourceRoot,
		ManifestPath: writeManifest(t),
		ProxyTimeout: time.Second,
	}, WithChatToolConfigHandler(migrated))
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/dashboard/chatTool/config", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "go chat tool config" {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestSidebarMediumMediaIDUpdateUsesMigratedHandlerWhenConfigured(t *testing.T) {
	sourceRoot := t.TempDir()
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		PHPUpstream:  "http://127.0.0.1:9501",
		SourceRoot:   sourceRoot,
		ManifestPath: writeManifest(t),
		ProxyTimeout: time.Second,
	}, WithSidebarMediumMediaIDUpdateHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go sidebar medium media id update"))
	})))
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/sidebar/medium/mediaIdUpdate", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "go sidebar medium media id update" {
		t.Fatalf("body = %q", rec.Body.String())
	}
	for _, route := range srv.migratedRoutes() {
		if route == "GET /sidebar/medium/mediaIdUpdate" {
			return
		}
	}
	t.Fatalf("missing migrated route: %#v", srv.migratedRoutes())
}

func TestSidebarAgentHandlersAreRouted(t *testing.T) {
	sourceRoot := t.TempDir()
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		PHPUpstream:  "http://127.0.0.1:9501",
		SourceRoot:   sourceRoot,
		ManifestPath: writeManifest(t),
		ProxyTimeout: time.Second,
	}, WithSidebarAgentAuthHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go sidebar agent auth"))
	})), WithSidebarAgentOAuthHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go sidebar agent oauth"))
	})), WithSidebarAgentJSSDKHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go sidebar agent jssdk"))
	})), WithSidebarWxJSSDKHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go sidebar wx jssdk"))
	})))
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		method string
		path   string
		body   string
		route  string
	}{
		{method: http.MethodGet, path: "/sidebar/agent/auth", body: "go sidebar agent auth", route: "GET /sidebar/agent/auth"},
		{method: http.MethodPost, path: "/sidebar/agent/auth", body: "go sidebar agent auth", route: "POST /sidebar/agent/auth"},
		{method: http.MethodGet, path: "/sidebar/agent/oauth", body: "go sidebar agent oauth", route: "GET /sidebar/agent/oauth"},
		{method: http.MethodGet, path: "/sidebar/agent/jssdkConfig", body: "go sidebar agent jssdk", route: "GET /sidebar/agent/jssdkConfig"},
		{method: http.MethodGet, path: "/sidebar/wxJsSdk/config", body: "go sidebar wx jssdk", route: "GET /sidebar/wxJsSdk/config"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d", tc.path, rec.Code)
		}
		if rec.Body.String() != tc.body {
			t.Fatalf("%s body = %q", tc.path, rec.Body.String())
		}
		if !containsString(srv.migratedRoutes(), tc.route) {
			t.Fatalf("missing migrated route %q in %#v", tc.route, srv.migratedRoutes())
		}
	}
}

func TestSidebarContactSOPHandlersAreRouted(t *testing.T) {
	sourceRoot := t.TempDir()
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		PHPUpstream:  "http://127.0.0.1:9501",
		SourceRoot:   sourceRoot,
		ManifestPath: writeManifest(t),
		ProxyTimeout: time.Second,
	}, WithSidebarContactSOPGetInfoHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go sidebar contact sop info"))
	})), WithSidebarContactSOPGetTipInfoHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go sidebar contact sop tip info"))
	})))
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		path  string
		body  string
		route string
	}{
		{path: "/sidebar/contactSop/getSopInfo", body: "go sidebar contact sop info", route: "GET /sidebar/contactSop/getSopInfo"},
		{path: "/sidebar/contactSop/getSopTipInfo", body: "go sidebar contact sop tip info", route: "GET /sidebar/contactSop/getSopTipInfo"},
	} {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d", tc.path, rec.Code)
		}
		if rec.Body.String() != tc.body {
			t.Fatalf("%s body = %q", tc.path, rec.Body.String())
		}
		if !containsString(srv.migratedRoutes(), tc.route) {
			t.Fatalf("missing migrated route %q in %#v", tc.route, srv.migratedRoutes())
		}
	}
}

func TestSidebarContactBatchAddDetailHandlerIsRouted(t *testing.T) {
	sourceRoot := t.TempDir()
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		PHPUpstream:  "http://127.0.0.1:9501",
		SourceRoot:   sourceRoot,
		ManifestPath: writeManifest(t),
		ProxyTimeout: time.Second,
	}, WithSidebarContactBatchAddDetailHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go sidebar contact batch add detail"))
	})))
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/sidebar/contactBatchAdd/detail", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if rec.Body.String() != "go sidebar contact batch add detail" {
		t.Fatalf("body = %q", rec.Body.String())
	}
	if !containsString(srv.migratedRoutes(), "GET /sidebar/contactBatchAdd/detail") {
		t.Fatalf("missing migrated route in %#v", srv.migratedRoutes())
	}
}

func TestContactBatchAddDashboardHandlersAreRouted(t *testing.T) {
	sourceRoot := t.TempDir()
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		PHPUpstream:  "http://127.0.0.1:9501",
		SourceRoot:   sourceRoot,
		ManifestPath: writeManifest(t),
		ProxyTimeout: time.Second,
	},
		WithContactBatchAddIndexHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go contact batch add index"))
		})),
		WithContactBatchAddImportIndexHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go contact batch add import index"))
		})),
		WithContactBatchAddImportStoreHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go contact batch add import store"))
		})),
		WithContactBatchAddAllotHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go contact batch add allot"))
		})),
		WithContactBatchAddDataStatisticHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go contact batch add data statistic"))
		})),
		WithContactBatchAddDestroyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go contact batch add destroy"))
		})),
		WithContactBatchAddImportDestroyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go contact batch add import destroy"))
		})),
		WithContactBatchAddSettingEditHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go contact batch add setting edit"))
		})),
		WithContactBatchAddSettingUpdateHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go contact batch add setting update"))
		})),
		WithContactBatchAddRemindHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go contact batch add remind"))
		})),
	)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		method string
		path   string
		body   string
		route  string
	}{
		{method: http.MethodGet, path: "/dashboard/contactBatchAdd/index", body: "go contact batch add index", route: "GET /dashboard/contactBatchAdd/index"},
		{method: http.MethodGet, path: "/dashboard/contactBatchAdd/importIndex", body: "go contact batch add import index", route: "GET /dashboard/contactBatchAdd/importIndex"},
		{method: http.MethodPost, path: "/dashboard/contactBatchAdd/importStore", body: "go contact batch add import store", route: "POST /dashboard/contactBatchAdd/importStore"},
		{method: http.MethodPost, path: "/dashboard/contactBatchAdd/allot", body: "go contact batch add allot", route: "POST /dashboard/contactBatchAdd/allot"},
		{method: http.MethodGet, path: "/dashboard/contactBatchAdd/dataStatistic", body: "go contact batch add data statistic", route: "GET /dashboard/contactBatchAdd/dataStatistic"},
		{method: http.MethodDelete, path: "/dashboard/contactBatchAdd/destroy", body: "go contact batch add destroy", route: "DELETE /dashboard/contactBatchAdd/destroy"},
		{method: http.MethodDelete, path: "/dashboard/contactBatchAdd/importDestroy", body: "go contact batch add import destroy", route: "DELETE /dashboard/contactBatchAdd/importDestroy"},
		{method: http.MethodGet, path: "/dashboard/contactBatchAdd/settingEdit", body: "go contact batch add setting edit", route: "GET /dashboard/contactBatchAdd/settingEdit"},
		{method: http.MethodPost, path: "/dashboard/contactBatchAdd/settingUpdate", body: "go contact batch add setting update", route: "POST /dashboard/contactBatchAdd/settingUpdate"},
		{method: http.MethodGet, path: "/dashboard/contactBatchAdd/remind", body: "go contact batch add remind", route: "GET /dashboard/contactBatchAdd/remind"},
		{method: http.MethodPost, path: "/dashboard/contactBatchAdd/remind", body: "go contact batch add remind", route: "POST /dashboard/contactBatchAdd/remind"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s %s status = %d", tc.method, tc.path, rec.Code)
		}
		if rec.Body.String() != tc.body {
			t.Fatalf("%s %s body = %q", tc.method, tc.path, rec.Body.String())
		}
		if !containsString(srv.migratedRoutes(), tc.route) {
			t.Fatalf("missing migrated route %q in %#v", tc.route, srv.migratedRoutes())
		}
	}
}

func TestSensitiveWordHandlersAreRouted(t *testing.T) {
	sourceRoot := t.TempDir()
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		PHPUpstream:  "http://127.0.0.1:9501",
		SourceRoot:   sourceRoot,
		ManifestPath: writeManifest(t),
		ProxyTimeout: time.Second,
	},
		WithSensitiveWordIndexHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go sensitive word index"))
		})),
		WithSensitiveWordStoreHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go sensitive word store"))
		})),
		WithSensitiveWordDestroyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go sensitive word destroy"))
		})),
		WithSensitiveWordStatusUpdateHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go sensitive word status update"))
		})),
		WithSensitiveWordMoveHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go sensitive word move"))
		})),
		WithSensitiveWordGroupSelectHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go sensitive word group select"))
		})),
		WithSensitiveWordGroupStoreHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go sensitive word group store"))
		})),
		WithSensitiveWordGroupUpdateHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go sensitive word group update"))
		})),
		WithSensitiveWordsMonitorIndexHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go sensitive words monitor index"))
		})),
		WithSensitiveWordsMonitorShowHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go sensitive words monitor show"))
		})),
	)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		method string
		path   string
		body   string
		route  string
	}{
		{method: http.MethodGet, path: "/dashboard/sensitiveWord/index", body: "go sensitive word index", route: "GET /dashboard/sensitiveWord/index"},
		{method: http.MethodPost, path: "/dashboard/sensitiveWord/store", body: "go sensitive word store", route: "POST /dashboard/sensitiveWord/store"},
		{method: http.MethodDelete, path: "/dashboard/sensitiveWord/destroy", body: "go sensitive word destroy", route: "DELETE /dashboard/sensitiveWord/destroy"},
		{method: http.MethodPut, path: "/dashboard/sensitiveWord/statusUpdate", body: "go sensitive word status update", route: "PUT /dashboard/sensitiveWord/statusUpdate"},
		{method: http.MethodPut, path: "/dashboard/sensitiveWord/move", body: "go sensitive word move", route: "PUT /dashboard/sensitiveWord/move"},
		{method: http.MethodGet, path: "/dashboard/sensitiveWordGroup/select", body: "go sensitive word group select", route: "GET /dashboard/sensitiveWordGroup/select"},
		{method: http.MethodPost, path: "/dashboard/sensitiveWordGroup/store", body: "go sensitive word group store", route: "POST /dashboard/sensitiveWordGroup/store"},
		{method: http.MethodPut, path: "/dashboard/sensitiveWordGroup/update", body: "go sensitive word group update", route: "PUT /dashboard/sensitiveWordGroup/update"},
		{method: http.MethodGet, path: "/dashboard/sensitiveWordsMonitor/index", body: "go sensitive words monitor index", route: "GET /dashboard/sensitiveWordsMonitor/index"},
		{method: http.MethodGet, path: "/dashboard/sensitiveWordsMonitor/show", body: "go sensitive words monitor show", route: "GET /dashboard/sensitiveWordsMonitor/show"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s %s status = %d", tc.method, tc.path, rec.Code)
		}
		if rec.Body.String() != tc.body {
			t.Fatalf("%s %s body = %q", tc.method, tc.path, rec.Body.String())
		}
		if !containsString(srv.migratedRoutes(), tc.route) {
			t.Fatalf("missing migrated route %q in %#v", tc.route, srv.migratedRoutes())
		}
	}
}

func TestSOPDashboardHandlersAreRouted(t *testing.T) {
	sourceRoot := t.TempDir()
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		PHPUpstream:  "http://127.0.0.1:9501",
		SourceRoot:   sourceRoot,
		ManifestPath: writeManifest(t),
		ProxyTimeout: time.Second,
	},
		WithContactSOPIndexHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go contact sop index"))
		})),
		WithContactSOPStoreHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go contact sop store"))
		})),
		WithContactSOPSetEmployeeHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go contact sop set employee"))
		})),
		WithContactSOPStateHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go contact sop state"))
		})),
		WithContactSOPInfoHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go contact sop info"))
		})),
		WithContactSOPDestroyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go contact sop destroy"))
		})),
		WithContactSOPUpdateHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go contact sop update"))
		})),
		WithRoomSOPIndexHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room sop index"))
		})),
		WithRoomSOPStoreHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room sop store"))
		})),
		WithRoomSOPSetRoomHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room sop set room"))
		})),
		WithRoomSOPStateHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room sop state"))
		})),
		WithRoomSOPInfoHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room sop info"))
		})),
		WithRoomSOPDestroyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room sop destroy"))
		})),
		WithRoomSOPUpdateHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room sop update"))
		})),
	)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		method string
		path   string
		body   string
		route  string
	}{
		{method: http.MethodGet, path: "/dashboard/contactSop/index", body: "go contact sop index", route: "GET /dashboard/contactSop/index"},
		{method: http.MethodPost, path: "/dashboard/contactSop/store", body: "go contact sop store", route: "POST /dashboard/contactSop/store"},
		{method: http.MethodPut, path: "/dashboard/contactSop/setEmployee", body: "go contact sop set employee", route: "PUT /dashboard/contactSop/setEmployee"},
		{method: http.MethodPut, path: "/dashboard/contactSop/state", body: "go contact sop state", route: "PUT /dashboard/contactSop/state"},
		{method: http.MethodGet, path: "/dashboard/contactSop/info", body: "go contact sop info", route: "GET /dashboard/contactSop/info"},
		{method: http.MethodDelete, path: "/dashboard/contactSop/destroy", body: "go contact sop destroy", route: "DELETE /dashboard/contactSop/destroy"},
		{method: http.MethodPut, path: "/dashboard/contactSop/update", body: "go contact sop update", route: "PUT /dashboard/contactSop/update"},
		{method: http.MethodGet, path: "/dashboard/roomSop/index", body: "go room sop index", route: "GET /dashboard/roomSop/index"},
		{method: http.MethodPost, path: "/dashboard/roomSop/store", body: "go room sop store", route: "POST /dashboard/roomSop/store"},
		{method: http.MethodPut, path: "/dashboard/roomSop/setRoom", body: "go room sop set room", route: "PUT /dashboard/roomSop/setRoom"},
		{method: http.MethodPut, path: "/dashboard/roomSop/state", body: "go room sop state", route: "PUT /dashboard/roomSop/state"},
		{method: http.MethodGet, path: "/dashboard/roomSop/info", body: "go room sop info", route: "GET /dashboard/roomSop/info"},
		{method: http.MethodDelete, path: "/dashboard/roomSop/destroy", body: "go room sop destroy", route: "DELETE /dashboard/roomSop/destroy"},
		{method: http.MethodPut, path: "/dashboard/roomSop/update", body: "go room sop update", route: "PUT /dashboard/roomSop/update"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s %s status = %d", tc.method, tc.path, rec.Code)
		}
		if rec.Body.String() != tc.body {
			t.Fatalf("%s %s body = %q", tc.method, tc.path, rec.Body.String())
		}
		if !containsString(srv.migratedRoutes(), tc.route) {
			t.Fatalf("missing migrated route %q in %#v", tc.route, srv.migratedRoutes())
		}
	}
}

func TestShopCodeDashboardHandlersAreRouted(t *testing.T) {
	sourceRoot := t.TempDir()
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		PHPUpstream:  "http://127.0.0.1:9501",
		SourceRoot:   sourceRoot,
		ManifestPath: writeManifest(t),
		ProxyTimeout: time.Second,
	},
		WithShopCodeLocationHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go shop code location"))
		})),
		WithShopCodeAddressKeyWordListHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go shop code address keyword list"))
		})),
		WithShopCodeStoreHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go shop code store"))
		})),
		WithShopCodeUpdateHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go shop code update"))
		})),
		WithShopCodeDestroyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go shop code destroy"))
		})),
		WithShopCodeInfoHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go shop code info"))
		})),
		WithShopCodeStatusHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go shop code status"))
		})),
		WithShopCodeIndexHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go shop code index"))
		})),
		WithShopCodeSearchCityHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go shop code search city"))
		})),
		WithShopCodeShareHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go shop code share"))
		})),
		WithShopCodePageInfoHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go shop code page info"))
		})),
		WithShopCodePageSetHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go shop code page set"))
		})),
		WithShopCodeShowHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go shop code show"))
		})),
		WithShopCodeShowContactHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go shop code show contact"))
		})),
		WithShopCodeShowShopHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go shop code show shop"))
		})),
		WithShopCodeUpdateEmployeeHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go shop code update employee"))
		})),
		WithShopCodeUpdateQRCodeHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go shop code update qrcode"))
		})),
		WithShopCodeBatchContactTagsHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go shop code batch contact tags"))
		})),
	)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		method string
		path   string
		body   string
		route  string
	}{
		{method: http.MethodGet, path: "/dashboard/shopCode/location", body: "go shop code location", route: "GET /dashboard/shopCode/location"},
		{method: http.MethodGet, path: "/dashboard/shopCode/addressKeyWordList", body: "go shop code address keyword list", route: "GET /dashboard/shopCode/addressKeyWordList"},
		{method: http.MethodPost, path: "/dashboard/shopCode/store", body: "go shop code store", route: "POST /dashboard/shopCode/store"},
		{method: http.MethodPut, path: "/dashboard/shopCode/update", body: "go shop code update", route: "PUT /dashboard/shopCode/update"},
		{method: http.MethodDelete, path: "/dashboard/shopCode/destroy", body: "go shop code destroy", route: "DELETE /dashboard/shopCode/destroy"},
		{method: http.MethodGet, path: "/dashboard/shopCode/info", body: "go shop code info", route: "GET /dashboard/shopCode/info"},
		{method: http.MethodPut, path: "/dashboard/shopCode/status", body: "go shop code status", route: "PUT /dashboard/shopCode/status"},
		{method: http.MethodGet, path: "/dashboard/shopCode/index", body: "go shop code index", route: "GET /dashboard/shopCode/index"},
		{method: http.MethodGet, path: "/dashboard/shopCode/searchCity", body: "go shop code search city", route: "GET /dashboard/shopCode/searchCity"},
		{method: http.MethodGet, path: "/dashboard/shopCode/share", body: "go shop code share", route: "GET /dashboard/shopCode/share"},
		{method: http.MethodGet, path: "/dashboard/shopCode/pageInfo", body: "go shop code page info", route: "GET /dashboard/shopCode/pageInfo"},
		{method: http.MethodPost, path: "/dashboard/shopCode/pageSet", body: "go shop code page set", route: "POST /dashboard/shopCode/pageSet"},
		{method: http.MethodGet, path: "/dashboard/shopCode/show", body: "go shop code show", route: "GET /dashboard/shopCode/show"},
		{method: http.MethodGet, path: "/dashboard/shopCode/showContact", body: "go shop code show contact", route: "GET /dashboard/shopCode/showContact"},
		{method: http.MethodGet, path: "/dashboard/shopCode/showShop", body: "go shop code show shop", route: "GET /dashboard/shopCode/showShop"},
		{method: http.MethodPost, path: "/dashboard/shopCode/updateEmployee", body: "go shop code update employee", route: "POST /dashboard/shopCode/updateEmployee"},
		{method: http.MethodPost, path: "/dashboard/shopCode/updateQrcode", body: "go shop code update qrcode", route: "POST /dashboard/shopCode/updateQrcode"},
		{method: http.MethodPut, path: "/dashboard/shopCode/batchContactTags", body: "go shop code batch contact tags", route: "PUT /dashboard/shopCode/batchContactTags"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s %s status = %d", tc.method, tc.path, rec.Code)
		}
		if rec.Body.String() != tc.body {
			t.Fatalf("%s %s body = %q", tc.method, tc.path, rec.Body.String())
		}
		if !containsString(srv.migratedRoutes(), tc.route) {
			t.Fatalf("missing migrated route %q in %#v", tc.route, srv.migratedRoutes())
		}
	}
}

func TestRoomWelcomeIndexAliasesAreRouted(t *testing.T) {
	sourceRoot := t.TempDir()
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		PHPUpstream:  "http://127.0.0.1:9501",
		SourceRoot:   sourceRoot,
		ManifestPath: writeManifest(t),
		ProxyTimeout: time.Second,
	},
		WithRoomWelcomeIndexHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room welcome index"))
		})),
	)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		path  string
		route string
	}{
		{path: "/dashboard/roomWelcome/index", route: "GET /dashboard/roomWelcome/index"},
		{path: "/dashboard/clockIn/index", route: "GET /dashboard/clockIn/index"},
	} {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d", tc.path, rec.Code)
		}
		if rec.Body.String() != "go room welcome index" {
			t.Fatalf("%s body = %q", tc.path, rec.Body.String())
		}
		if !containsString(srv.migratedRoutes(), tc.route) {
			t.Fatalf("missing migrated route %q in %#v", tc.route, srv.migratedRoutes())
		}
	}
}

func TestRadarDashboardHandlersAreRouted(t *testing.T) {
	sourceRoot := t.TempDir()
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		PHPUpstream:  "http://127.0.0.1:9501",
		SourceRoot:   sourceRoot,
		ManifestPath: writeManifest(t),
		ProxyTimeout: time.Second,
	},
		WithRadarStoreHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go radar store"))
		})),
		WithRadarUpdateHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go radar update"))
		})),
		WithRadarIndexHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go radar index"))
		})),
		WithRadarDestroyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go radar destroy"))
		})),
		WithRadarInfoHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go radar info"))
		})),
		WithRadarStoreChannelHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go radar store channel"))
		})),
		WithRadarStoreChannelLinkHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go radar store channel link"))
		})),
		WithRadarIndexChannelHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go radar index channel"))
		})),
		WithRadarIndexChannelLinkHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go radar index channel link"))
		})),
		WithRadarShowHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go radar show"))
		})),
		WithRadarShowContactHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go radar show contact"))
		})),
		WithRadarShowChannelHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go radar show channel"))
		})),
		WithRadarArticleHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go radar article"))
		})),
	)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		method string
		path   string
		body   string
		route  string
	}{
		{method: http.MethodPost, path: "/dashboard/radar/store", body: "go radar store", route: "POST /dashboard/radar/store"},
		{method: http.MethodPut, path: "/dashboard/radar/update", body: "go radar update", route: "PUT /dashboard/radar/update"},
		{method: http.MethodGet, path: "/dashboard/radar/index", body: "go radar index", route: "GET /dashboard/radar/index"},
		{method: http.MethodDelete, path: "/dashboard/radar/destroy", body: "go radar destroy", route: "DELETE /dashboard/radar/destroy"},
		{method: http.MethodGet, path: "/dashboard/radar/info", body: "go radar info", route: "GET /dashboard/radar/info"},
		{method: http.MethodPost, path: "/dashboard/radar/storeChannel", body: "go radar store channel", route: "POST /dashboard/radar/storeChannel"},
		{method: http.MethodPost, path: "/dashboard/radar/storeChannelLink", body: "go radar store channel link", route: "POST /dashboard/radar/storeChannelLink"},
		{method: http.MethodGet, path: "/dashboard/radar/indexChannel", body: "go radar index channel", route: "GET /dashboard/radar/indexChannel"},
		{method: http.MethodGet, path: "/dashboard/radar/indexChannelLink", body: "go radar index channel link", route: "GET /dashboard/radar/indexChannelLink"},
		{method: http.MethodGet, path: "/dashboard/radar/show", body: "go radar show", route: "GET /dashboard/radar/show"},
		{method: http.MethodGet, path: "/dashboard/radar/showContact", body: "go radar show contact", route: "GET /dashboard/radar/showContact"},
		{method: http.MethodGet, path: "/dashboard/radar/showChannel", body: "go radar show channel", route: "GET /dashboard/radar/showChannel"},
		{method: http.MethodGet, path: "/dashboard/radar/radarArticle", body: "go radar article", route: "GET /dashboard/radar/radarArticle"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s %s status = %d", tc.method, tc.path, rec.Code)
		}
		if rec.Body.String() != tc.body {
			t.Fatalf("%s %s body = %q", tc.method, tc.path, rec.Body.String())
		}
		if !containsString(srv.migratedRoutes(), tc.route) {
			t.Fatalf("missing migrated route %q in %#v", tc.route, srv.migratedRoutes())
		}
	}
}

func TestAutoTagDashboardHandlersAreRouted(t *testing.T) {
	sourceRoot := t.TempDir()
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		PHPUpstream:  "http://127.0.0.1:9501",
		SourceRoot:   sourceRoot,
		ManifestPath: writeManifest(t),
		ProxyTimeout: time.Second,
	},
		WithAutoTagStoreHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go auto tag store"))
		})),
		WithAutoTagIndexHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go auto tag index"))
		})),
		WithAutoTagDestroyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go auto tag destroy"))
		})),
		WithAutoTagOnOffHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go auto tag on off"))
		})),
		WithAutoTagShowHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go auto tag show"))
		})),
		WithAutoTagShowContactKeyWordHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go auto tag keyword contacts"))
		})),
		WithAutoTagKeyWordTagHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go auto tag keyword task"))
		})),
		WithAutoTagShowContactRoomHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go auto tag room contacts"))
		})),
		WithAutoTagShowContactTimeHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go auto tag time contacts"))
		})),
		WithWorkMessageFromUsersHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go work message from users"))
		})),
		WithWorkMessageToUsersHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go work message to users"))
		})),
		WithWorkMessageIndexHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go work message index"))
		})),
		WithWorkMessageConfigCorpStoreHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go work message config corp store"))
		})),
		WithWorkMessageConfigCorpShowHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go work message config corp show"))
		})),
		WithWorkMessageConfigCorpIndexHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go work message config corp index"))
		})),
		WithWorkMessageConfigStepCreateHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go work message config step create"))
		})),
		WithWorkMessageConfigStepUpdateHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go work message config step update"))
		})),
	)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		method string
		path   string
		body   string
		route  string
	}{
		{method: http.MethodPost, path: "/dashboard/autoTag/store", body: "go auto tag store", route: "POST /dashboard/autoTag/store"},
		{method: http.MethodGet, path: "/dashboard/autoTag/index", body: "go auto tag index", route: "GET /dashboard/autoTag/index"},
		{method: http.MethodDelete, path: "/dashboard/autoTag/destroy", body: "go auto tag destroy", route: "DELETE /dashboard/autoTag/destroy"},
		{method: http.MethodPut, path: "/dashboard/autoTag/onOff", body: "go auto tag on off", route: "PUT /dashboard/autoTag/onOff"},
		{method: http.MethodGet, path: "/dashboard/autoTag/show", body: "go auto tag show", route: "GET /dashboard/autoTag/show"},
		{method: http.MethodGet, path: "/dashboard/autoTag/showContactKeyWord", body: "go auto tag keyword contacts", route: "GET /dashboard/autoTag/showContactKeyWord"},
		{method: http.MethodGet, path: "/Task/AutoTag/KeyWordTag", body: "go auto tag keyword task", route: "GET /Task/AutoTag/KeyWordTag"},
		{method: http.MethodGet, path: "/dashboard/Task/AutoTag/KeyWordTag", body: "go auto tag keyword task", route: "GET /dashboard/Task/AutoTag/KeyWordTag"},
		{method: http.MethodGet, path: "/dashboard/autoTag/showContactRoom", body: "go auto tag room contacts", route: "GET /dashboard/autoTag/showContactRoom"},
		{method: http.MethodGet, path: "/dashboard/autoTag/showContactTime", body: "go auto tag time contacts", route: "GET /dashboard/autoTag/showContactTime"},
		{method: http.MethodGet, path: "/dashboard/workMessage/fromUsers", body: "go work message from users", route: "GET /dashboard/workMessage/fromUsers"},
		{method: http.MethodGet, path: "/dashboard/workMessage/toUsers", body: "go work message to users", route: "GET /dashboard/workMessage/toUsers"},
		{method: http.MethodGet, path: "/dashboard/workMessage/index", body: "go work message index", route: "GET /dashboard/workMessage/index"},
		{method: http.MethodGet, path: "/dashboard/workMessage/detail", body: "go work message index", route: "GET /dashboard/workMessage/detail"},
		{method: http.MethodPost, path: "/dashboard/workMessageConfig/corpStore", body: "go work message config corp store", route: "POST /dashboard/workMessageConfig/corpStore"},
		{method: http.MethodGet, path: "/dashboard/workMessageConfig/corpShow", body: "go work message config corp show", route: "GET /dashboard/workMessageConfig/corpShow"},
		{method: http.MethodGet, path: "/dashboard/workMessageConfig/corpIndex", body: "go work message config corp index", route: "GET /dashboard/workMessageConfig/corpIndex"},
		{method: http.MethodGet, path: "/dashboard/workMessageConfig/stepCreate", body: "go work message config step create", route: "GET /dashboard/workMessageConfig/stepCreate"},
		{method: http.MethodPut, path: "/dashboard/workMessageConfig/stepUpdate", body: "go work message config step update", route: "PUT /dashboard/workMessageConfig/stepUpdate"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s %s status = %d", tc.method, tc.path, rec.Code)
		}
		if rec.Body.String() != tc.body {
			t.Fatalf("%s %s body = %q", tc.method, tc.path, rec.Body.String())
		}
		if !containsString(srv.migratedRoutes(), tc.route) {
			t.Fatalf("missing migrated route %q in %#v", tc.route, srv.migratedRoutes())
		}
	}
}

func TestSidebarRoomSOPHandlersAreRouted(t *testing.T) {
	sourceRoot := t.TempDir()
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		PHPUpstream:  "http://127.0.0.1:9501",
		SourceRoot:   sourceRoot,
		ManifestPath: writeManifest(t),
		ProxyTimeout: time.Second,
	}, WithSidebarRoomSOPGetInfoHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go sidebar room sop info"))
	})), WithSidebarRoomSOPLogStateHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go sidebar room sop log state"))
	})))
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		method string
		path   string
		body   string
		route  string
	}{
		{method: http.MethodGet, path: "/sidebar/roomSop/getSopInfo", body: "go sidebar room sop info", route: "GET /sidebar/roomSop/getSopInfo"},
		{method: http.MethodPut, path: "/sidebar/roomSop/logState", body: "go sidebar room sop log state", route: "PUT /sidebar/roomSop/logState"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d", tc.path, rec.Code)
		}
		if rec.Body.String() != tc.body {
			t.Fatalf("%s body = %q", tc.path, rec.Body.String())
		}
		if !containsString(srv.migratedRoutes(), tc.route) {
			t.Fatalf("missing migrated route %q in %#v", tc.route, srv.migratedRoutes())
		}
	}
}

func TestLotteryDashboardHandlersAreRouted(t *testing.T) {
	sourceRoot := t.TempDir()
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		PHPUpstream:  "http://127.0.0.1:9501",
		SourceRoot:   sourceRoot,
		ManifestPath: writeManifest(t),
		ProxyTimeout: time.Second,
	},
		WithLotteryIndexHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go lottery index"))
		})),
		WithLotteryStoreHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go lottery store"))
		})),
		WithLotteryShowContactHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go lottery show contact"))
		})),
		WithLotteryShowHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go lottery show"))
		})),
		WithLotteryDestroyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go lottery destroy"))
		})),
		WithLotteryShareHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go lottery share"))
		})),
		WithLotteryUpdateHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go lottery update"))
		})),
		WithLotteryInfoHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go lottery info"))
		})),
		WithLotteryWriteOffHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go lottery write off"))
		})),
		WithLotteryBatchContactTagsHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go lottery batch contact tags"))
		})),
	)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		method string
		path   string
		body   string
		route  string
	}{
		{method: http.MethodGet, path: "/dashboard/lottery/index", body: "go lottery index", route: "GET /dashboard/lottery/index"},
		{method: http.MethodPost, path: "/dashboard/lottery/store", body: "go lottery store", route: "POST /dashboard/lottery/store"},
		{method: http.MethodGet, path: "/dashboard/lottery/showContact", body: "go lottery show contact", route: "GET /dashboard/lottery/showContact"},
		{method: http.MethodGet, path: "/dashboard/lottery/show", body: "go lottery show", route: "GET /dashboard/lottery/show"},
		{method: http.MethodDelete, path: "/dashboard/lottery/destroy", body: "go lottery destroy", route: "DELETE /dashboard/lottery/destroy"},
		{method: http.MethodGet, path: "/dashboard/lottery/share", body: "go lottery share", route: "GET /dashboard/lottery/share"},
		{method: http.MethodPut, path: "/dashboard/lottery/update", body: "go lottery update", route: "PUT /dashboard/lottery/update"},
		{method: http.MethodGet, path: "/dashboard/lottery/info", body: "go lottery info", route: "GET /dashboard/lottery/info"},
		{method: http.MethodGet, path: "/dashboard/lottery/writeOff", body: "go lottery write off", route: "GET /dashboard/lottery/writeOff"},
		{method: http.MethodPut, path: "/dashboard/lottery/batchContactTags", body: "go lottery batch contact tags", route: "PUT /dashboard/lottery/batchContactTags"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s %s status = %d", tc.method, tc.path, rec.Code)
		}
		if rec.Body.String() != tc.body {
			t.Fatalf("%s %s body = %q", tc.method, tc.path, rec.Body.String())
		}
		if !containsString(srv.migratedRoutes(), tc.route) {
			t.Fatalf("missing migrated route %q in %#v", tc.route, srv.migratedRoutes())
		}
	}
}

func TestRoomFissionDashboardHandlersAreRouted(t *testing.T) {
	sourceRoot := t.TempDir()
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		PHPUpstream:  "http://127.0.0.1:9501",
		SourceRoot:   sourceRoot,
		ManifestPath: writeManifest(t),
		ProxyTimeout: time.Second,
	},
		WithRoomFissionIndexHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room fission index"))
		})),
		WithRoomFissionStoreHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room fission store"))
		})),
		WithRoomFissionInfoHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room fission info"))
		})),
		WithRoomFissionUpdateHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room fission update"))
		})),
		WithRoomFissionDestroyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room fission destroy"))
		})),
		WithRoomFissionInviteHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room fission invite"))
		})),
		WithRoomFissionShowHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room fission show"))
		})),
		WithRoomFissionShowRoomHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room fission show room"))
		})),
		WithRoomFissionShowContactHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room fission show contact"))
		})),
		WithRoomFissionWriteOffHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room fission write off"))
		})),
	)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		method string
		path   string
		body   string
		route  string
	}{
		{method: http.MethodGet, path: "/dashboard/roomFission/index", body: "go room fission index", route: "GET /dashboard/roomFission/index"},
		{method: http.MethodPost, path: "/dashboard/roomFission/store", body: "go room fission store", route: "POST /dashboard/roomFission/store"},
		{method: http.MethodGet, path: "/dashboard/roomFission/info", body: "go room fission info", route: "GET /dashboard/roomFission/info"},
		{method: http.MethodPut, path: "/dashboard/roomFission/update", body: "go room fission update", route: "PUT /dashboard/roomFission/update"},
		{method: http.MethodDelete, path: "/dashboard/roomFission/destroy", body: "go room fission destroy", route: "DELETE /dashboard/roomFission/destroy"},
		{method: http.MethodPost, path: "/dashboard/roomFission/invite", body: "go room fission invite", route: "POST /dashboard/roomFission/invite"},
		{method: http.MethodGet, path: "/dashboard/roomFission/show", body: "go room fission show", route: "GET /dashboard/roomFission/show"},
		{method: http.MethodGet, path: "/dashboard/roomFission/showRoom", body: "go room fission show room", route: "GET /dashboard/roomFission/showRoom"},
		{method: http.MethodGet, path: "/dashboard/roomFission/showContact", body: "go room fission show contact", route: "GET /dashboard/roomFission/showContact"},
		{method: http.MethodGet, path: "/dashboard/roomFission/writeOff", body: "go room fission write off", route: "GET /dashboard/roomFission/writeOff"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s %s status = %d", tc.method, tc.path, rec.Code)
		}
		if rec.Body.String() != tc.body {
			t.Fatalf("%s %s body = %q", tc.method, tc.path, rec.Body.String())
		}
		if !containsString(srv.migratedRoutes(), tc.route) {
			t.Fatalf("missing migrated route %q in %#v", tc.route, srv.migratedRoutes())
		}
	}
}

func TestRoomClockInDashboardHandlersAreRouted(t *testing.T) {
	sourceRoot := t.TempDir()
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		PHPUpstream:  "http://127.0.0.1:9501",
		SourceRoot:   sourceRoot,
		ManifestPath: writeManifest(t),
		ProxyTimeout: time.Second,
	},
		WithRoomClockInIndexHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room clock in index"))
		})),
		WithRoomClockInStoreHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room clock in store"))
		})),
		WithRoomClockInUpdateHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room clock in update"))
		})),
		WithRoomClockInDestroyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room clock in destroy"))
		})),
		WithRoomClockInShowHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room clock in show"))
		})),
		WithRoomClockInShowContactHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room clock in show contact"))
		})),
		WithRoomClockInBatchContactTagsHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room clock in batch contact tags"))
		})),
		WithRoomClockInInfoHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room clock in info"))
		})),
		WithRoomClockInDayDetailHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room clock in day detail"))
		})),
	)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		method string
		path   string
		body   string
		route  string
	}{
		{method: http.MethodGet, path: "/dashboard/roomClockIn/index", body: "go room clock in index", route: "GET /dashboard/roomClockIn/index"},
		{method: http.MethodPost, path: "/dashboard/roomClockIn/store", body: "go room clock in store", route: "POST /dashboard/roomClockIn/store"},
		{method: http.MethodPut, path: "/dashboard/roomClockIn/update", body: "go room clock in update", route: "PUT /dashboard/roomClockIn/update"},
		{method: http.MethodDelete, path: "/dashboard/roomClockIn/destroy", body: "go room clock in destroy", route: "DELETE /dashboard/roomClockIn/destroy"},
		{method: http.MethodGet, path: "/dashboard/roomClockIn/show", body: "go room clock in show", route: "GET /dashboard/roomClockIn/show"},
		{method: http.MethodGet, path: "/dashboard/roomClockIn/showContact", body: "go room clock in show contact", route: "GET /dashboard/roomClockIn/showContact"},
		{method: http.MethodPut, path: "/dashboard/roomClockIn/batchContactTags", body: "go room clock in batch contact tags", route: "PUT /dashboard/roomClockIn/batchContactTags"},
		{method: http.MethodGet, path: "/dashboard/roomClockIn/info", body: "go room clock in info", route: "GET /dashboard/roomClockIn/info"},
		{method: http.MethodGet, path: "/dashboard/roomClockIn/dayDetail", body: "go room clock in day detail", route: "GET /dashboard/roomClockIn/dayDetail"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s %s status = %d", tc.method, tc.path, rec.Code)
		}
		if rec.Body.String() != tc.body {
			t.Fatalf("%s %s body = %q", tc.method, tc.path, rec.Body.String())
		}
		if !containsString(srv.migratedRoutes(), tc.route) {
			t.Fatalf("missing migrated route %q in %#v", tc.route, srv.migratedRoutes())
		}
	}
}

func TestRoomQualityDashboardHandlersAreRouted(t *testing.T) {
	sourceRoot := t.TempDir()
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		PHPUpstream:  "http://127.0.0.1:9501",
		SourceRoot:   sourceRoot,
		ManifestPath: writeManifest(t),
		ProxyTimeout: time.Second,
	},
		WithRoomQualityIndexHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room quality index"))
		})),
		WithRoomQualityStoreHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room quality store"))
		})),
		WithRoomQualityStatusHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room quality status"))
		})),
		WithRoomQualityInfoHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room quality info"))
		})),
		WithRoomQualityUpdateHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room quality update"))
		})),
		WithRoomQualityShowContactHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room quality show contact"))
		})),
		WithRoomQualityDestroyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room quality destroy"))
		})),
		WithRoomQualityContactDetailHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room quality contact detail"))
		})),
	)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		method string
		path   string
		body   string
		route  string
	}{
		{method: http.MethodGet, path: "/dashboard/roomQuality/index", body: "go room quality index", route: "GET /dashboard/roomQuality/index"},
		{method: http.MethodPost, path: "/dashboard/roomQuality/store", body: "go room quality store", route: "POST /dashboard/roomQuality/store"},
		{method: http.MethodPut, path: "/dashboard/roomQuality/status", body: "go room quality status", route: "PUT /dashboard/roomQuality/status"},
		{method: http.MethodGet, path: "/dashboard/roomQuality/info", body: "go room quality info", route: "GET /dashboard/roomQuality/info"},
		{method: http.MethodPut, path: "/dashboard/roomQuality/update", body: "go room quality update", route: "PUT /dashboard/roomQuality/update"},
		{method: http.MethodGet, path: "/dashboard/roomQuality/showContact", body: "go room quality show contact", route: "GET /dashboard/roomQuality/showContact"},
		{method: http.MethodDelete, path: "/dashboard/roomQuality/destroy", body: "go room quality destroy", route: "DELETE /dashboard/roomQuality/destroy"},
		{method: http.MethodGet, path: "/dashboard/roomQuality/contactDetail", body: "go room quality contact detail", route: "GET /dashboard/roomQuality/contactDetail"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s %s status = %d", tc.method, tc.path, rec.Code)
		}
		if rec.Body.String() != tc.body {
			t.Fatalf("%s %s body = %q", tc.method, tc.path, rec.Body.String())
		}
		if !containsString(srv.migratedRoutes(), tc.route) {
			t.Fatalf("missing migrated route %q in %#v", tc.route, srv.migratedRoutes())
		}
	}
}

func TestRoomCalendarDashboardHandlersAreRouted(t *testing.T) {
	sourceRoot := t.TempDir()
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		PHPUpstream:  "http://127.0.0.1:9501",
		SourceRoot:   sourceRoot,
		ManifestPath: writeManifest(t),
		ProxyTimeout: time.Second,
	},
		WithRoomCalendarIndexHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room calendar index"))
		})),
		WithRoomCalendarAddRoomHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room calendar add room"))
		})),
		WithRoomCalendarDestroyRoomHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room calendar destroy room"))
		})),
		WithRoomCalendarStoreHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room calendar store"))
		})),
		WithRoomCalendarDestroyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room calendar destroy"))
		})),
		WithRoomCalendarShowHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room calendar show"))
		})),
		WithRoomCalendarUpdateHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room calendar update"))
		})),
	)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		method string
		path   string
		body   string
		route  string
	}{
		{method: http.MethodGet, path: "/dashboard/roomCalendar/index", body: "go room calendar index", route: "GET /dashboard/roomCalendar/index"},
		{method: http.MethodPost, path: "/dashboard/roomCalendar/addRoom", body: "go room calendar add room", route: "POST /dashboard/roomCalendar/addRoom"},
		{method: http.MethodDelete, path: "/dashboard/roomCalendar/destroyRoom", body: "go room calendar destroy room", route: "DELETE /dashboard/roomCalendar/destroyRoom"},
		{method: http.MethodPost, path: "/dashboard/roomCalendar/store", body: "go room calendar store", route: "POST /dashboard/roomCalendar/store"},
		{method: http.MethodDelete, path: "/dashboard/roomCalendar/destroy", body: "go room calendar destroy", route: "DELETE /dashboard/roomCalendar/destroy"},
		{method: http.MethodGet, path: "/dashboard/roomCalendar/show", body: "go room calendar show", route: "GET /dashboard/roomCalendar/show"},
		{method: http.MethodPut, path: "/dashboard/roomCalendar/update", body: "go room calendar update", route: "PUT /dashboard/roomCalendar/update"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s %s status = %d", tc.method, tc.path, rec.Code)
		}
		if rec.Body.String() != tc.body {
			t.Fatalf("%s %s body = %q", tc.method, tc.path, rec.Body.String())
		}
		if !containsString(srv.migratedRoutes(), tc.route) {
			t.Fatalf("missing migrated route %q in %#v", tc.route, srv.migratedRoutes())
		}
	}
}

func TestRoomRemindDashboardHandlersAreRouted(t *testing.T) {
	sourceRoot := t.TempDir()
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		PHPUpstream:  "http://127.0.0.1:9501",
		SourceRoot:   sourceRoot,
		ManifestPath: writeManifest(t),
		ProxyTimeout: time.Second,
	},
		WithRoomRemindIndexHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room remind index"))
		})),
		WithRoomRemindDestroyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room remind destroy"))
		})),
		WithRoomRemindInfoHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room remind info"))
		})),
		WithRoomRemindStatusHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room remind status"))
		})),
		WithRoomRemindStoreHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room remind store"))
		})),
		WithRoomRemindUpdateHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room remind update"))
		})),
		WithRoomRemindTaskHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room remind task"))
		})),
	)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		method string
		path   string
		body   string
		route  string
	}{
		{method: http.MethodGet, path: "/dashboard/roomRemind/index", body: "go room remind index", route: "GET /dashboard/roomRemind/index"},
		{method: http.MethodDelete, path: "/dashboard/roomRemind/destroy", body: "go room remind destroy", route: "DELETE /dashboard/roomRemind/destroy"},
		{method: http.MethodGet, path: "/dashboard/roomRemind/info", body: "go room remind info", route: "GET /dashboard/roomRemind/info"},
		{method: http.MethodGet, path: "/dashboard/roomRemind/status", body: "go room remind status", route: "GET /dashboard/roomRemind/status"},
		{method: http.MethodPost, path: "/dashboard/roomRemind/store", body: "go room remind store", route: "POST /dashboard/roomRemind/store"},
		{method: http.MethodPut, path: "/dashboard/roomRemind/update", body: "go room remind update", route: "PUT /dashboard/roomRemind/update"},
		{method: http.MethodGet, path: "/dashboard/task/roomRemind", body: "go room remind task", route: "GET /dashboard/task/roomRemind"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s %s status = %d", tc.method, tc.path, rec.Code)
		}
		if rec.Body.String() != tc.body {
			t.Fatalf("%s %s body = %q", tc.method, tc.path, rec.Body.String())
		}
		if !containsString(srv.migratedRoutes(), tc.route) {
			t.Fatalf("missing migrated route %q in %#v", tc.route, srv.migratedRoutes())
		}
	}
}

func TestRoomInfinitePullDashboardHandlersAreRouted(t *testing.T) {
	sourceRoot := t.TempDir()
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		PHPUpstream:  "http://127.0.0.1:9501",
		SourceRoot:   sourceRoot,
		ManifestPath: writeManifest(t),
		ProxyTimeout: time.Second,
	},
		WithRoomInfinitePullIndexHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room infinite pull index"))
		})),
		WithRoomInfinitePullInfoHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room infinite pull info"))
		})),
		WithRoomInfinitePullUpdateHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room infinite pull update"))
		})),
		WithRoomInfinitePullDestroyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room infinite pull destroy"))
		})),
		WithRoomInfinitePullStoreHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go room infinite pull store"))
		})),
	)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		method string
		path   string
		body   string
		route  string
	}{
		{method: http.MethodGet, path: "/dashboard/roomInfinitePull/index", body: "go room infinite pull index", route: "GET /dashboard/roomInfinitePull/index"},
		{method: http.MethodGet, path: "/dashboard/roomInfinitePull/info", body: "go room infinite pull info", route: "GET /dashboard/roomInfinitePull/info"},
		{method: http.MethodPut, path: "/dashboard/roomInfinitePull/update", body: "go room infinite pull update", route: "PUT /dashboard/roomInfinitePull/update"},
		{method: http.MethodDelete, path: "/dashboard/roomInfinitePull/destroy", body: "go room infinite pull destroy", route: "DELETE /dashboard/roomInfinitePull/destroy"},
		{method: http.MethodPost, path: "/dashboard/roomInfinitePull/store", body: "go room infinite pull store", route: "POST /dashboard/roomInfinitePull/store"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s %s status = %d", tc.method, tc.path, rec.Code)
		}
		if rec.Body.String() != tc.body {
			t.Fatalf("%s %s body = %q", tc.method, tc.path, rec.Body.String())
		}
		if !containsString(srv.migratedRoutes(), tc.route) {
			t.Fatalf("missing migrated route %q in %#v", tc.route, srv.migratedRoutes())
		}
	}
}

func TestDashboardAgentStoreUsesMigratedHandlerWhenConfigured(t *testing.T) {
	sourceRoot := t.TempDir()
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		PHPUpstream:  "http://127.0.0.1:9501",
		SourceRoot:   sourceRoot,
		ManifestPath: writeManifest(t),
		ProxyTimeout: time.Second,
	}, WithDashboardAgentStoreHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go dashboard agent store"))
	})))
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/dashboard/agent/store", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "go dashboard agent store" {
		t.Fatalf("body = %q", rec.Body.String())
	}
	if !containsString(srv.migratedRoutes(), "POST /dashboard/agent/store") {
		t.Fatalf("missing migrated route in %#v", srv.migratedRoutes())
	}
}

func TestContactTransferRoutesUseMigratedHandlersWhenConfigured(t *testing.T) {
	sourceRoot := t.TempDir()
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		PHPUpstream:  "http://127.0.0.1:9501",
		SourceRoot:   sourceRoot,
		ManifestPath: writeManifest(t),
		ProxyTimeout: time.Second,
	},
		WithContactTransferInfoHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go contact transfer info"))
		})),
		WithContactTransferUnassignedListHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go contact transfer unassigned"))
		})),
		WithContactTransferRoomHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go contact transfer room"))
		})),
		WithContactTransferLogHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go contact transfer log"))
		})),
		WithContactTransferSaveUnassignedListHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go contact transfer sync"))
		})),
		WithContactTransferCustomerHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go contact transfer customer"))
		})),
		WithContactTransferRoomStoreHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go contact transfer room store"))
		})),
	)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		method string
		path   string
		body   string
		route  string
	}{
		{method: http.MethodGet, path: "/dashboard/contactTransfer/info", body: "go contact transfer info", route: "GET /dashboard/contactTransfer/info"},
		{method: http.MethodGet, path: "/dashboard/contactTransfer/unassignedList", body: "go contact transfer unassigned", route: "GET /dashboard/contactTransfer/unassignedList"},
		{method: http.MethodGet, path: "/dashboard/contactTransfer/room", body: "go contact transfer room", route: "GET /dashboard/contactTransfer/room"},
		{method: http.MethodGet, path: "/dashboard/contactTransfer/log", body: "go contact transfer log", route: "GET /dashboard/contactTransfer/log"},
		{method: http.MethodGet, path: "/dashboard/contactTransfer/saveUnassignedList", body: "go contact transfer sync", route: "GET /dashboard/contactTransfer/saveUnassignedList"},
		{method: http.MethodPost, path: "/dashboard/contactTransfer/index", body: "go contact transfer customer", route: "POST /dashboard/contactTransfer/index"},
		{method: http.MethodPost, path: "/dashboard/contactTransfer/room", body: "go contact transfer room store", route: "POST /dashboard/contactTransfer/room"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s %s status = %d body=%s", tc.method, tc.path, rec.Code, rec.Body.String())
		}
		if rec.Body.String() != tc.body {
			t.Fatalf("%s %s body = %q", tc.method, tc.path, rec.Body.String())
		}
		if !containsString(srv.migratedRoutes(), tc.route) {
			t.Fatalf("missing migrated route %q in %#v", tc.route, srv.migratedRoutes())
		}
	}
}

func TestAgentTxtVerifyUploadFallsBackWhenNotMigrated(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/dashboard/agent/txtVerifyUpload" {
			t.Fatalf("unexpected upstream path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte("php txt upload"))
	}))
	defer upstream.Close()

	srv := newTestServer(t, upstream.URL)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/agent/txtVerifyUpload", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "php txt upload" {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestAgentTxtVerifyUploadUsesMigratedHandlerWhenConfigured(t *testing.T) {
	migrated := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go txt upload"))
	})
	sourceRoot := t.TempDir()
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		PHPUpstream:  "http://127.0.0.1:9501",
		SourceRoot:   sourceRoot,
		ManifestPath: writeManifest(t),
		ProxyTimeout: time.Second,
	}, WithAgentTxtUploadHandler(migrated))
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/dashboard/agent/txtVerifyUpload", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "go txt upload" {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestStaticUploadServesConfiguredStorageRoot(t *testing.T) {
	storageRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(storageRoot, "qrcode"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(storageRoot, "qrcode", "front-channel.png"), []byte("PNGDATA"), 0644); err != nil {
		t.Fatal(err)
	}
	srv, err := New(config.Config{
		ListenAddr:      ":0",
		FileStorageRoot: storageRoot,
		SourceRoot:      t.TempDir(),
		ManifestPath:    writeManifest(t),
		ProxyTimeout:    time.Second,
		Standalone:      true,
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/static/qrcode/front-channel.png", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "PNGDATA" {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestStaticUploadRejectsMissingAndTraversal(t *testing.T) {
	storageRoot := t.TempDir()
	outside := filepath.Join(filepath.Dir(storageRoot), "outside.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0644); err != nil {
		t.Fatal(err)
	}
	srv, err := New(config.Config{
		ListenAddr:      ":0",
		FileStorageRoot: storageRoot,
		SourceRoot:      t.TempDir(),
		ManifestPath:    writeManifest(t),
		ProxyTimeout:    time.Second,
		Standalone:      true,
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{"/static/missing.png", "/static/../outside.txt"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s status = %d, body=%s", path, rec.Code, rec.Body.String())
		}
	}
}

func TestAgentTxtVerifyUsesMigratedHandlerWhenConfigured(t *testing.T) {
	sourceRoot := t.TempDir()
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		PHPUpstream:  "http://127.0.0.1:9501",
		SourceRoot:   sourceRoot,
		ManifestPath: writeManifest(t),
		ProxyTimeout: time.Second,
	}, WithAgentTxtVerify())
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/WW_verify_ABCDEF1234567890.txt", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "ABCDEF1234567890" {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestRoleSelectFallsBackWhenNotMigrated(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/dashboard/role/select":
			_, _ = w.Write([]byte("php role select"))
		case "/dashboard/role/index":
			_, _ = w.Write([]byte("php role index"))
		case "/dashboard/role/show":
			_, _ = w.Write([]byte("php role show"))
		case "/dashboard/role/permissionShow":
			_, _ = w.Write([]byte("php role permission"))
		case "/dashboard/role/showEmployee":
			_, _ = w.Write([]byte("php role employee"))
		default:
			t.Fatalf("unexpected upstream path: %s", r.URL.Path)
		}
	}))
	defer upstream.Close()

	srv := newTestServer(t, upstream.URL)
	for _, tc := range []struct {
		path string
		body string
	}{
		{path: "/dashboard/role/select", body: "php role select"},
		{path: "/dashboard/role/index", body: "php role index"},
		{path: "/dashboard/role/show", body: "php role show"},
		{path: "/dashboard/role/permissionShow", body: "php role permission"},
		{path: "/dashboard/role/showEmployee", body: "php role employee"},
	} {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want %d", tc.path, rec.Code, http.StatusOK)
		}
		if rec.Body.String() != tc.body {
			t.Fatalf("%s body = %q", tc.path, rec.Body.String())
		}
	}
}

func TestRoleSelectUsesMigratedHandlerWhenConfigured(t *testing.T) {
	sourceRoot := t.TempDir()
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		PHPUpstream:  "http://127.0.0.1:9501",
		SourceRoot:   sourceRoot,
		ManifestPath: writeManifest(t),
		ProxyTimeout: time.Second,
	},
		WithRoleSelectHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go role select"))
		})),
		WithRoleIndexHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go role index"))
		})),
		WithRoleShowHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go role show"))
		})),
		WithRolePermissionShowHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go role permission"))
		})),
		WithRoleShowEmployeeHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go role employee"))
		})),
	)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		path string
		body string
	}{
		{path: "/dashboard/role/select", body: "go role select"},
		{path: "/dashboard/role/index", body: "go role index"},
		{path: "/dashboard/role/show", body: "go role show"},
		{path: "/dashboard/role/permissionShow", body: "go role permission"},
		{path: "/dashboard/role/showEmployee", body: "go role employee"},
	} {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want %d", tc.path, rec.Code, http.StatusOK)
		}
		if rec.Body.String() != tc.body {
			t.Fatalf("%s body = %q", tc.path, rec.Body.String())
		}
	}
}

func TestMenuReadRoutesFallBackWhenNotMigrated(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/dashboard/menu/iconIndex":
			_, _ = w.Write([]byte("php menu icon"))
		case "/dashboard/menu/select":
			_, _ = w.Write([]byte("php menu select"))
		case "/dashboard/menu/index":
			_, _ = w.Write([]byte("php menu index"))
		case "/dashboard/menu/show":
			_, _ = w.Write([]byte("php menu show"))
		default:
			t.Fatalf("unexpected upstream path: %s", r.URL.Path)
		}
	}))
	defer upstream.Close()

	srv := newTestServer(t, upstream.URL)
	for _, tc := range []struct {
		path string
		body string
	}{
		{path: "/dashboard/menu/iconIndex", body: "php menu icon"},
		{path: "/dashboard/menu/select", body: "php menu select"},
		{path: "/dashboard/menu/index", body: "php menu index"},
		{path: "/dashboard/menu/show", body: "php menu show"},
	} {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d", tc.path, rec.Code)
		}
		if rec.Body.String() != tc.body {
			t.Fatalf("%s body = %q", tc.path, rec.Body.String())
		}
	}
}

func TestMenuReadRoutesUseMigratedHandlersWhenConfigured(t *testing.T) {
	sourceRoot := t.TempDir()
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		PHPUpstream:  "http://127.0.0.1:9501",
		SourceRoot:   sourceRoot,
		ManifestPath: writeManifest(t),
		ProxyTimeout: time.Second,
	},
		WithMenuIconIndexHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go menu icon"))
		})),
		WithMenuSelectHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go menu select"))
		})),
		WithMenuIndexHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go menu index"))
		})),
		WithMenuShowHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go menu show"))
		})),
	)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		path string
		body string
	}{
		{path: "/dashboard/menu/iconIndex", body: "go menu icon"},
		{path: "/dashboard/menu/select", body: "go menu select"},
		{path: "/dashboard/menu/index", body: "go menu index"},
		{path: "/dashboard/menu/show", body: "go menu show"},
	} {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d", tc.path, rec.Code)
		}
		if rec.Body.String() != tc.body {
			t.Fatalf("%s body = %q", tc.path, rec.Body.String())
		}
	}
}

func TestInvalidAgentTxtVerifyFallsBackToPHP(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/WW_verify_short.txt" {
			t.Fatalf("unexpected upstream path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte("php txt verify"))
	}))
	defer upstream.Close()

	sourceRoot := t.TempDir()
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		PHPUpstream:  upstream.URL,
		SourceRoot:   sourceRoot,
		ManifestPath: writeManifest(t),
		ProxyTimeout: time.Second,
	}, WithAgentTxtVerify())
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/WW_verify_short.txt", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "php txt verify" {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestChannelCodeGroupHandlersAreRouted(t *testing.T) {
	sourceRoot := t.TempDir()
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		SourceRoot:   sourceRoot,
		ManifestPath: writeManifest(t),
		ProxyTimeout: time.Second,
	}, WithChannelCodeIndexHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go channel code index"))
	})), WithChannelCodeShowHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go channel code show"))
	})), WithChannelCodeContactHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go channel code contact"))
	})), WithChannelCodeStatisticsHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go channel code statistics"))
	})), WithChannelCodeStatisticsIndexHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go channel code statistics index"))
	})), WithChannelCodeStoreHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go channel code store"))
	})), WithChannelCodeUpdateHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go channel code update"))
	})), WithChannelCodeGroupIndexHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go channel code group index"))
	})), WithChannelCodeGroupDetailHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go channel code group detail"))
	})), WithChannelCodeGroupStoreHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go channel code group store"))
	})), WithChannelCodeGroupUpdateHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go channel code group update"))
	})), WithChannelCodeGroupMoveHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("go channel code group move"))
	})))
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodGet, path: "/dashboard/channelCode/index", body: "go channel code index"},
		{method: http.MethodGet, path: "/dashboard/channelCode/show", body: "go channel code show"},
		{method: http.MethodGet, path: "/dashboard/channelCode/contact", body: "go channel code contact"},
		{method: http.MethodGet, path: "/dashboard/channelCode/statistics", body: "go channel code statistics"},
		{method: http.MethodGet, path: "/dashboard/channelCode/statisticsIndex", body: "go channel code statistics index"},
		{method: http.MethodPost, path: "/dashboard/channelCode/store", body: "go channel code store"},
		{method: http.MethodPut, path: "/dashboard/channelCode/update", body: "go channel code update"},
		{method: http.MethodGet, path: "/dashboard/channelCodeGroup/index", body: "go channel code group index"},
		{method: http.MethodGet, path: "/dashboard/channelCodeGroup/detail", body: "go channel code group detail"},
		{method: http.MethodPost, path: "/dashboard/channelCodeGroup/store", body: "go channel code group store"},
		{method: http.MethodPut, path: "/dashboard/channelCodeGroup/update", body: "go channel code group update"},
		{method: http.MethodPut, path: "/dashboard/channelCodeGroup/move", body: "go channel code group move"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d", tc.path, rec.Code)
		}
		if rec.Body.String() != tc.body {
			t.Fatalf("%s body = %q", tc.path, rec.Body.String())
		}
	}
}

func TestSaaSAdminBrandingHandlersAreRouted(t *testing.T) {
	sourceRoot := t.TempDir()
	srv, err := New(config.Config{
		ListenAddr:   ":0",
		SourceRoot:   sourceRoot,
		ManifestPath: writeManifest(t),
		ProxyTimeout: time.Second,
	},
		WithSaaSAdminBrandingProfilesHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go branding profiles"))
		})),
		WithSaaSAdminBrandingProfileHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("go branding profile " + r.Method))
		})),
	)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/dashboard/saasAdmin/brandingProfiles", "go branding profiles"},
		{http.MethodGet, "/dashboard/saasAdmin/brandingProfile", "go branding profile GET"},
		{http.MethodPost, "/dashboard/saasAdmin/brandingProfile", "go branding profile POST"},
		{http.MethodPut, "/dashboard/saasAdmin/brandingProfile", "go branding profile PUT"},
	}
	for _, test := range tests {
		req := httptest.NewRequest(test.method, test.path, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || rec.Body.String() != test.body {
			t.Fatalf("%s %s status=%d body=%q", test.method, test.path, rec.Code, rec.Body.String())
		}
		if !containsString(srv.migratedRoutes(), test.method+" "+test.path) {
			t.Fatalf("route %s %s missing", test.method, test.path)
		}
	}
}

func newTestServer(t *testing.T, upstream string) *Server {
	t.Helper()
	sourceRoot := t.TempDir()
	cfg := config.Config{
		ListenAddr:   ":0",
		PHPUpstream:  upstream,
		SourceRoot:   sourceRoot,
		ManifestPath: writeManifest(t),
		ProxyTimeout: time.Second,
	}
	srv, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return srv
}

func TestSaaSAdminNotificationCredentialRotationRoute(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("go saas admin notification credential rotation"))
	})
	srv, err := New(config.Config{ListenAddr: ":0", Standalone: true, ProxyTimeout: time.Second}, WithSaaSAdminNotificationCredentialRotationHandler(handler))
	if err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{http.MethodPost, http.MethodPut} {
		req := httptest.NewRequest(method, "/dashboard/saasAdmin/notificationCredentialRotation", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || rec.Body.String() != "go saas admin notification credential rotation" {
			t.Fatalf("%s status=%d body=%q", method, rec.Code, rec.Body.String())
		}
		if !containsString(srv.migratedRoutes(), method+" /dashboard/saasAdmin/notificationCredentialRotation") {
			t.Fatalf("route missing for %s", method)
		}
	}
}

func TestSaaSAdminWeComCredentialRoutes(t *testing.T) {
	protectionHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("go saas admin WeCom credential protection"))
	})
	rotationHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("go saas admin WeCom credential rotation"))
	})
	srv, err := New(
		config.Config{ListenAddr: ":0", Standalone: true, ProxyTimeout: time.Second},
		WithSaaSAdminWeComCredentialProtectionHandler(protectionHandler),
		WithSaaSAdminWeComCredentialRotationHandler(rotationHandler),
	)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/wecomCredentialProtection", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "go saas admin WeCom credential protection" {
		t.Fatalf("GET status=%d body=%q", rec.Code, rec.Body.String())
	}
	if !containsString(srv.migratedRoutes(), "GET /dashboard/saasAdmin/wecomCredentialProtection") {
		t.Fatal("protection route missing")
	}

	for _, method := range []string{http.MethodPost, http.MethodPut} {
		req = httptest.NewRequest(method, "/dashboard/saasAdmin/wecomCredentialRotation", nil)
		rec = httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || rec.Body.String() != "go saas admin WeCom credential rotation" {
			t.Fatalf("%s status=%d body=%q", method, rec.Code, rec.Body.String())
		}
		if !containsString(srv.migratedRoutes(), method+" /dashboard/saasAdmin/wecomCredentialRotation") {
			t.Fatalf("rotation route missing for %s", method)
		}
	}
}

func writeManifest(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "compat_manifest.json")
	if err := os.WriteFile(path, []byte(`{"source_revision":"test-revision","routes":[]}`), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
