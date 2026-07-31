package scrm

import (
	"database/sql"
	"errors"
	nethttp "net/http"
	"testing"
	"time"

	appmodules "jiyi/mochat-go/internal/app/modules"
	"jiyi/mochat-go/internal/modules/scrm/ports"
	transporthttp "jiyi/mochat-go/internal/modules/scrm/transport/http"
)

func TestNewRejectsEachNilDependency(t *testing.T) {
	db := testDB(t)
	clock := moduleClock{}
	idGenerator := moduleIDGenerator{}
	principal := modulePrincipalResolver{}

	for _, tc := range []struct {
		name         string
		dependencies Dependencies
	}{
		{name: "database", dependencies: Dependencies{Clock: clock, IDGenerator: idGenerator, PrincipalResolver: principal}},
		{name: "clock", dependencies: Dependencies{DB: db, IDGenerator: idGenerator, PrincipalResolver: principal}},
		{name: "ID generator", dependencies: Dependencies{DB: db, Clock: clock, PrincipalResolver: principal}},
		{name: "principal resolver", dependencies: Dependencies{DB: db, Clock: clock, IDGenerator: idGenerator}},
		{name: "typed nil clock", dependencies: Dependencies{DB: db, Clock: (*moduleClock)(nil), IDGenerator: idGenerator, PrincipalResolver: principal}},
		{name: "typed nil ID generator", dependencies: Dependencies{DB: db, Clock: clock, IDGenerator: (*moduleIDGenerator)(nil), PrincipalResolver: principal}},
		{name: "typed nil principal resolver", dependencies: Dependencies{DB: db, Clock: clock, IDGenerator: idGenerator, PrincipalResolver: (*modulePrincipalResolver)(nil)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := New(tc.dependencies); err == nil {
				t.Fatal("New() error = nil")
			}
		})
	}
}

func TestNewAssemblesRepositoryServiceAndHTTPHandler(t *testing.T) {
	module, err := New(validDependencies(t))
	if err != nil {
		t.Fatal(err)
	}
	if module == nil || module.leads == nil || module.customerLifecycle == nil {
		t.Fatalf("module = %#v", module)
	}
}

func TestRegisterRoutesInstallsExactlyPostAndGetLeads(t *testing.T) {
	module, err := New(validDependencies(t))
	if err != nil {
		t.Fatal(err)
	}
	registrar := &recordingRegistrar{}

	if err := module.RegisterRoutes(registrar); err != nil {
		t.Fatal(err)
	}

	want := []registeredRoute{
		{method: nethttp.MethodPost, pattern: transporthttp.LeadsPath},
		{method: nethttp.MethodGet, pattern: transporthttp.LeadsPath},
		{method: nethttp.MethodPost, pattern: transporthttp.FormalLeadsPath},
		{method: nethttp.MethodGet, pattern: transporthttp.FormalLeadsPath},
		{method: nethttp.MethodGet, pattern: transporthttp.AssignmentsPath},
		{method: nethttp.MethodPut, pattern: transporthttp.AssignmentsPath},
		{method: nethttp.MethodPost, pattern: transporthttp.AssignmentReleasePath},
		{method: nethttp.MethodPost, pattern: transporthttp.AssignmentClaimPath},
	}
	if len(registrar.routes) != len(want) {
		t.Fatalf("routes = %#v, want %#v", registrar.routes, want)
	}
	for index := range want {
		got := registrar.routes[index]
		if got.method != want[index].method || got.pattern != want[index].pattern || got.handler == nil {
			t.Fatalf("route[%d] = %#v, want %#v with handler", index, got, want[index])
		}
	}
}

func TestRegisterRoutesReturnsCompositionRootErrors(t *testing.T) {
	module, err := New(validDependencies(t))
	if err != nil {
		t.Fatal(err)
	}

	router := appmodules.NewRouter()
	if err := module.RegisterRoutes(router); err != nil {
		t.Fatal(err)
	}
	if err := module.RegisterRoutes(router); !errors.Is(err, appmodules.ErrDuplicateRoute) {
		t.Fatalf("duplicate error = %v", err)
	}

	wantErr := errors.New("invalid internal route")
	registrar := &recordingRegistrar{failAt: 2, err: wantErr}
	if err := module.RegisterRoutes(registrar); !errors.Is(err, wantErr) {
		t.Fatalf("invalid route error = %v", err)
	}
}

func validDependencies(t *testing.T) Dependencies {
	t.Helper()
	return Dependencies{
		DB:                testDB(t),
		Clock:             moduleClock{},
		IDGenerator:       moduleIDGenerator{},
		PrincipalResolver: modulePrincipalResolver{},
	}
}

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("mysql", "unused:unused@tcp(localhost:3306)/unused")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

type moduleClock struct{}

func (moduleClock) Now() time.Time {
	return time.Date(2026, time.July, 30, 0, 0, 0, 0, time.UTC)
}

type moduleIDGenerator struct{}

func (moduleIDGenerator) NewID() (string, error) {
	return "generated-id", nil
}

type modulePrincipalResolver struct{}

func (modulePrincipalResolver) Resolve(*nethttp.Request) (transporthttp.Principal, error) {
	return transporthttp.Principal{UserID: 1, TenantID: 1}, nil
}

type registeredRoute struct {
	method  string
	pattern string
	handler nethttp.Handler
}

type recordingRegistrar struct {
	routes []registeredRoute
	failAt int
	err    error
}

func (r *recordingRegistrar) Handle(method, pattern string, handler nethttp.Handler) error {
	r.routes = append(r.routes, registeredRoute{method: method, pattern: pattern, handler: handler})
	if r.failAt > 0 && len(r.routes) == r.failAt {
		return r.err
	}
	return nil
}

var (
	_ ports.Clock                     = moduleClock{}
	_ ports.IDGenerator               = moduleIDGenerator{}
	_ transporthttp.PrincipalResolver = modulePrincipalResolver{}
)
