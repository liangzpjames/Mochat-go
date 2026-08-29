package main

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"jiyi/mochat-go/internal/archivebridge"
	"jiyi/mochat-go/internal/wecomarchivedemo"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestLoadConfigRequiresLongIndependentTokensForFixtureMode(t *testing.T) {
	env := map[string]string{
		"MOCHAT_ARCHIVE_BRIDGE_BEARER":        "bridge-0123456789012345678901234567890123456789",
		"MOCHAT_ARCHIVE_FIXTURE_ENABLED":      "true",
		"MOCHAT_ARCHIVE_FIXTURE_ADMIN_BEARER": "admin-0123456789012345678901234567890123456789",
		"MOCHAT_ARCHIVE_FIXTURE_STATE_PATH":   "/tmp/fixture/state.json",
	}
	config, err := loadConfig(func(key string) string { return env[key] })
	if err != nil {
		t.Fatal(err)
	}
	if !config.fixtureEnabled || config.bridgeBearer == config.fixtureAdminBearer || config.address != ":8083" {
		t.Fatalf("config=%+v", config)
	}
	delete(env, "MOCHAT_ARCHIVE_FIXTURE_ADMIN_BEARER")
	if _, err := loadConfig(func(key string) string { return env[key] }); err == nil {
		t.Fatal("expected missing admin token to fail closed")
	}
}

func TestLoadConfigRejectsFixtureAndProductionSDKTogether(t *testing.T) {
	env := map[string]string{
		"MOCHAT_ARCHIVE_BRIDGE_BEARER":        "bridge-0123456789012345678901234567890123456789",
		"MOCHAT_ARCHIVE_FIXTURE_ENABLED":      "true",
		"MOCHAT_ARCHIVE_SDK_ENABLED":          "true",
		"MOCHAT_ARCHIVE_FIXTURE_ADMIN_BEARER": "admin-0123456789012345678901234567890123456789",
		"MOCHAT_ARCHIVE_FIXTURE_STATE_PATH":   "/tmp/fixture/state.json",
	}
	if _, err := loadConfig(func(key string) string { return env[key] }); err == nil {
		t.Fatal("fixture and production SDK were enabled together")
	}
}

type fakeRegistrar struct {
	count       int
	registerErr error
	closeErr    error
	registered  bool
	closed      bool
	events      *eventLog
}

func (r *fakeRegistrar) RegisterAll(context.Context) (int, error) {
	r.registered = true
	return r.count, r.registerErr
}

func (r *fakeRegistrar) Close() error {
	r.closed = true
	if r.events != nil {
		r.events.add("close")
	}
	return r.closeErr
}

func productionTestEnv() func(string) string {
	env := map[string]string{
		"MOCHAT_ARCHIVE_BRIDGE_BEARER": "bridge-0123456789012345678901234567890123456789",
		"MOCHAT_ARCHIVE_SDK_ENABLED":   "true",
	}
	return func(key string) string { return env[key] }
}

func TestRunProductionSDKModeFailsClosedBeforeListenWhenRegistrationIsEmptyOrFails(t *testing.T) {
	for _, tc := range []struct {
		name        string
		registerErr error
		wantCode    string
	}{
		{name: "zero drivers", wantCode: "ARCHIVE_DRIVER_UNAVAILABLE"},
		{name: "initialization failure", registerErr: &archivebridge.BridgeError{Code: "ARCHIVE_DRIVER_INITIALIZATION_FAILED"}, wantCode: "ARCHIVE_DRIVER_INITIALIZATION_FAILED"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			registrar := &fakeRegistrar{registerErr: tc.registerErr}
			factoryCalled := false
			err := run(context.Background(), productionTestEnv(), func(store *archivebridge.Store) (driverRegistrar, error) {
				factoryCalled = true
				if store == nil {
					t.Fatal("registrar factory received nil store")
				}
				return registrar, nil
			})
			if !factoryCalled || !registrar.registered || !registrar.closed {
				t.Fatalf("factory=%t registered=%t closed=%t", factoryCalled, registrar.registered, registrar.closed)
			}
			if code := archivebridge.ErrorCode(err); code != tc.wantCode {
				t.Fatalf("code=%q want=%q err=%v", code, tc.wantCode, err)
			}
		})
	}
}

func TestRunProductionSDKModeRejectsNilRegistrar(t *testing.T) {
	err := run(context.Background(), productionTestEnv(), func(*archivebridge.Store) (driverRegistrar, error) {
		return nil, nil
	})
	if code := archivebridge.ErrorCode(err); code != "ARCHIVE_DRIVER_UNAVAILABLE" {
		t.Fatalf("code=%q err=%v", code, err)
	}
}

func TestProductionRegistrarBootstrapSelectsProtectedMySQLSource(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectPing()
	mock.ExpectClose()
	env := map[string]string{
		"MOCHAT_MYSQL_DSN":                             "protected-dsn-reference",
		"MOCHAT_GO_WECOM_CREDENTIAL_ENCRYPTION_KEY":    base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef")),
		"MOCHAT_GO_WECOM_CREDENTIAL_ENCRYPTION_KEY_ID": "archive-production-v1",
		"MOCHAT_ARCHIVE_BRIDGE_STATE_ROOT":             t.TempDir(),
	}
	openerCalled := false
	registrar, err := newProductionRegistrar(func(key string) string { return env[key] }, func(dsn string) (*sql.DB, error) {
		openerCalled = true
		if dsn != "protected-dsn-reference" {
			t.Fatalf("dsn=%q", dsn)
		}
		return db, nil
	}, archivebridge.NewStore())
	if err != nil || registrar == nil || !openerCalled {
		t.Fatalf("registrar=%v opener=%t err=%v", registrar, openerCalled, err)
	}
	if err := registrar.Close(); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestProductionRegistrarBootstrapFailsClosedWithoutProtectedSource(t *testing.T) {
	registrar, err := newProductionRegistrar(func(string) string { return "" }, func(string) (*sql.DB, error) {
		t.Fatal("database opener must not run without protected configuration")
		return nil, nil
	}, archivebridge.NewStore())
	if registrar != nil || archivebridge.ErrorCode(err) != "ARCHIVE_DRIVER_UNAVAILABLE" {
		t.Fatalf("registrar=%v error=%v code=%s", registrar, err, archivebridge.ErrorCode(err))
	}
}

type mainFinanceDriver struct{}

func (mainFinanceDriver) FetchPage(context.Context, uint64, uint32) (wecomarchivedemo.ArchivePage, error) {
	return wecomarchivedemo.ArchivePage{}, nil
}

func (mainFinanceDriver) FetchMediaWithTimeout(context.Context, string, string, int) (wecomarchivedemo.MediaChunk, error) {
	return wecomarchivedemo.MediaChunk{}, nil
}

func TestHealthAndProductionReadinessAreDistinct(t *testing.T) {
	store := archivebridge.NewStore()
	base := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	handler := newReadinessHandler(base, store, true)

	health := httptest.NewRecorder()
	handler.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if health.Code != http.StatusOK {
		t.Fatalf("health=%d %s", health.Code, health.Body.String())
	}
	ready := httptest.NewRecorder()
	handler.ServeHTTP(ready, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if ready.Code != http.StatusServiceUnavailable || !strings.Contains(ready.Body.String(), "ARCHIVE_DRIVER_UNAVAILABLE") {
		t.Fatalf("empty production readiness=%d %s", ready.Code, ready.Body.String())
	}

	binding := archivebridge.Binding{TenantID: 1, CorpID: 2, WXCorpID: "ww-ready", IntegrationMode: archivebridge.ModeSelfBuilt}
	if err := store.RegisterFinance(binding, mainFinanceDriver{}); err != nil {
		t.Fatal(err)
	}
	ready = httptest.NewRecorder()
	handler.ServeHTTP(ready, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if ready.Code != http.StatusOK {
		t.Fatalf("registered production readiness=%d %s", ready.Code, ready.Body.String())
	}
	var status map[string]any
	if err := json.Unmarshal(ready.Body.Bytes(), &status); err != nil || status["status"] != "ready" {
		t.Fatalf("readiness body=%s err=%v", ready.Body.String(), err)
	}
}

type eventLog struct {
	mu     sync.Mutex
	values []string
}

func (l *eventLog) add(value string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.values = append(l.values, value)
}

func (l *eventLog) joined() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return strings.Join(l.values, ",")
}

type fakeBridgeServer struct {
	events      *eventLog
	started     chan struct{}
	stopped     chan struct{}
	stopOnce    sync.Once
	shutdownErr error
}

func (s *fakeBridgeServer) ListenAndServe() error {
	s.events.add("listen")
	close(s.started)
	<-s.stopped
	return http.ErrServerClosed
}

func (s *fakeBridgeServer) Shutdown(context.Context) error {
	s.events.add("drain")
	s.stopOnce.Do(func() { close(s.stopped) })
	return s.shutdownErr

}

func (s *fakeBridgeServer) Close() error {
	s.events.add("force-close")
	s.stopOnce.Do(func() { close(s.stopped) })
	return nil
}

func TestServeDrainsHTTPBeforeRegistrarCloseAndReturnsCloseFailure(t *testing.T) {
	events := &eventLog{}
	server := &fakeBridgeServer{events: events, started: make(chan struct{}), stopped: make(chan struct{})}
	registrar := &fakeRegistrar{count: 1, closeErr: errors.New("controlled close failure"), events: events}
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- serve(ctx, server, registrar) }()
	<-server.started
	cancel()
	err := <-result
	if events.joined() != "listen,drain,close" {
		t.Fatalf("events=%s", events.joined())
	}
	if !strings.Contains(err.Error(), "controlled close failure") {
		t.Fatalf("serve error=%v", err)
	}
}

type listenerErrorServer struct {
	events *eventLog
	err    error
}

func (s *listenerErrorServer) ListenAndServe() error {
	s.events.add("listen")
	return s.err
}

func (s *listenerErrorServer) Shutdown(context.Context) error {
	s.events.add("unexpected-drain")
	return nil
}

func (s *listenerErrorServer) Close() error {
	s.events.add("force-close")
	return nil
}

func TestServeForceClosesHTTPBeforeRegistrarOnListenerError(t *testing.T) {
	events := &eventLog{}
	listenErr := errors.New("controlled listener failure")
	server := &listenerErrorServer{events: events, err: listenErr}
	registrar := &fakeRegistrar{events: events}
	err := serve(context.Background(), server, registrar)
	if !errors.Is(err, listenErr) || events.joined() != "listen,force-close,close" {
		t.Fatalf("error=%v events=%s", err, events.joined())
	}
}

func TestServeForceClosesHTTPBeforeRegistrarWhenDrainFails(t *testing.T) {
	events := &eventLog{}
	drainErr := errors.New("controlled drain timeout")
	server := &fakeBridgeServer{events: events, started: make(chan struct{}), stopped: make(chan struct{}), shutdownErr: drainErr}
	registrar := &fakeRegistrar{events: events}
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- serve(ctx, server, registrar) }()
	<-server.started
	cancel()
	err := <-result
	if !errors.Is(err, drainErr) || events.joined() != "listen,drain,force-close,close" {
		t.Fatalf("error=%v events=%s", err, events.joined())
	}
}
