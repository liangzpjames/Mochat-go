package bootstrap

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	appmodules "jiyi/mochat-go/internal/app/modules"
	"jiyi/mochat-go/internal/authjwt"
	"jiyi/mochat-go/internal/dashboard"
	transporthttp "jiyi/mochat-go/internal/modules/scrm/transport/http"
)

func TestRegisterSCRMDisabledAcceptsNilDependenciesAndInstallsNoRoute(t *testing.T) {
	router := appmodules.NewRouter()

	if err := RegisterSCRM(router, false, SCRMDependencies{}); err != nil {
		t.Fatal(err)
	}

	assertSCRMRouteMissing(t, router, http.MethodPost)
	assertSCRMRouteMissing(t, router, http.MethodGet)
}

func TestRegisterSCRMEnabledRejectsNilDatabaseOrPrincipalResolver(t *testing.T) {
	db := bootstrapTestDB(t)
	resolver := fixedPrincipalResolver{principal: transporthttp.Principal{UserID: 7, TenantID: 42}}

	for _, tc := range []struct {
		name string
		deps SCRMDependencies
	}{
		{name: "database", deps: SCRMDependencies{PrincipalResolver: resolver}},
		{name: "principal resolver", deps: SCRMDependencies{DB: db}},
		{name: "typed nil principal resolver", deps: SCRMDependencies{DB: db, PrincipalResolver: (*fixedPrincipalResolver)(nil)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := RegisterSCRM(appmodules.NewRouter(), true, tc.deps); err == nil {
				t.Fatal("RegisterSCRM() error = nil")
			}
		})
	}
}

func TestRegisterSCRMEnabledInstallsBothRoutes(t *testing.T) {
	router := appmodules.NewRouter()
	deps := SCRMDependencies{
		DB:                bootstrapTestDB(t),
		PrincipalResolver: fixedPrincipalResolver{principal: transporthttp.Principal{UserID: 7, TenantID: 42}},
	}

	if err := RegisterSCRM(router, true, deps); err != nil {
		t.Fatal(err)
	}

	assertSCRMRouteInstalled(t, router, http.MethodPost)
	assertSCRMRouteInstalled(t, router, http.MethodGet)
}

func TestRegisterSCRMDuplicateRegistrationFailsStartup(t *testing.T) {
	router := appmodules.NewRouter()
	deps := SCRMDependencies{
		DB:                bootstrapTestDB(t),
		PrincipalResolver: fixedPrincipalResolver{principal: transporthttp.Principal{UserID: 7, TenantID: 42}},
	}
	if err := RegisterSCRM(router, true, deps); err != nil {
		t.Fatal(err)
	}

	if err := RegisterSCRM(router, true, deps); !errors.Is(err, appmodules.ErrDuplicateRoute) {
		t.Fatalf("duplicate error = %v", err)
	}
}

func TestSCRMPrincipalResolverIgnoresClaimedTenantHeader(t *testing.T) {
	resolver, err := NewSCRMPrincipalResolver(
		fixedUserIDResolver{userID: 7},
		fixedSCRMUserStore{user: dashboard.User{ID: 7, TenantID: 42}},
	)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, transporthttp.LeadsPath, nil)
	request.Header.Set("X-Mochat-Go-Tenant-ID", "999")

	principal, err := resolver.Resolve(request)
	if err != nil {
		t.Fatal(err)
	}
	if principal.UserID != 7 || principal.TenantID != 42 {
		t.Fatalf("principal = %+v", principal)
	}
}

func TestSCRMPrincipalResolverClassifiesCredentialFailuresAsUnauthorized(t *testing.T) {
	for _, tc := range []struct {
		name    string
		userIDs dashboard.UserIDResolver
		users   SCRMUserStore
	}{
		{name: "revoked token", userIDs: fixedUserIDResolver{err: authjwt.ErrTokenBlacklisted}, users: fixedSCRMUserStore{}},
		{name: "user not found", userIDs: fixedUserIDResolver{userID: 7}, users: fixedSCRMUserStore{}},
		{name: "tenant not found", userIDs: fixedUserIDResolver{userID: 7}, users: fixedSCRMUserStore{user: dashboard.User{ID: 7}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resolver, err := NewSCRMPrincipalResolver(tc.userIDs, tc.users)
			if err != nil {
				t.Fatal(err)
			}
			_, err = resolver.Resolve(httptest.NewRequest(http.MethodGet, transporthttp.LeadsPath, nil))
			if !errors.Is(err, transporthttp.ErrPrincipalUnauthorized) {
				t.Fatalf("Resolve() error = %v", err)
			}
		})
	}
}

func TestSCRMPrincipalResolverClassifiesBackendFailuresAsUnavailable(t *testing.T) {
	for _, tc := range []struct {
		name    string
		userIDs dashboard.UserIDResolver
		users   SCRMUserStore
	}{
		{name: "Redis blacklist", userIDs: fixedUserIDResolver{err: authjwt.ErrBackendUnavailable}, users: fixedSCRMUserStore{}},
		{name: "session backend", userIDs: fixedUserIDResolver{err: errors.New("session database unavailable")}, users: fixedSCRMUserStore{}},
		{name: "MySQL user lookup", userIDs: fixedUserIDResolver{userID: 7}, users: fixedSCRMUserStore{err: errors.New("mysql unavailable")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resolver, err := NewSCRMPrincipalResolver(tc.userIDs, tc.users)
			if err != nil {
				t.Fatal(err)
			}
			_, err = resolver.Resolve(httptest.NewRequest(http.MethodGet, transporthttp.LeadsPath, nil))
			if !errors.Is(err, transporthttp.ErrPrincipalUnavailable) {
				t.Fatalf("Resolve() error = %v", err)
			}
			detail := strings.ToLower(err.Error())
			if strings.Contains(detail, "redis") || strings.Contains(detail, "session") || strings.Contains(detail, "mysql") {
				t.Fatalf("Resolve() leaked backend detail: %v", err)
			}
		})
	}
}

func TestNewSCRMPrincipalResolverRejectsNilDependencies(t *testing.T) {
	for _, tc := range []struct {
		name    string
		userIDs dashboard.UserIDResolver
		users   SCRMUserStore
	}{
		{name: "user ID resolver", users: fixedSCRMUserStore{}},
		{name: "user store", userIDs: fixedUserIDResolver{userID: 7}},
		{name: "typed nil user ID resolver", userIDs: (*fixedUserIDResolver)(nil), users: fixedSCRMUserStore{}},
		{name: "typed nil user store", userIDs: fixedUserIDResolver{userID: 7}, users: (*fixedSCRMUserStore)(nil)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewSCRMPrincipalResolver(tc.userIDs, tc.users); err == nil {
				t.Fatal("NewSCRMPrincipalResolver() error = nil")
			}
		})
	}
}

func TestSCRMClockUsesUTC(t *testing.T) {
	if location := (scrmClock{}).Now().Location(); location != time.UTC {
		t.Fatalf("clock location = %v", location)
	}
}

func TestSCRMIDGeneratorProducesUUIDV4CompatibleIDs(t *testing.T) {
	id, err := (scrmIDGenerator{}).NewID()
	if err != nil {
		t.Fatal(err)
	}
	const uuidV4Pattern = `^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`
	if !regexp.MustCompile(uuidV4Pattern).MatchString(id) {
		t.Fatalf("id = %q", id)
	}
}

func assertSCRMRouteInstalled(t *testing.T, router *appmodules.Router, method string) {
	t.Helper()
	request := httptest.NewRequest(method, transporthttp.LeadsPath, nil)
	if handler, ok := router.Match(request); !ok || handler == nil {
		t.Fatalf("%s %s was not installed", method, transporthttp.LeadsPath)
	}
}

func assertSCRMRouteMissing(t *testing.T, router *appmodules.Router, method string) {
	t.Helper()
	request := httptest.NewRequest(method, transporthttp.LeadsPath, nil)
	if handler, ok := router.Match(request); ok || handler != nil {
		t.Fatalf("%s %s was installed", method, transporthttp.LeadsPath)
	}
}

func bootstrapTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("mysql", "unused:unused@tcp(localhost:3306)/unused")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

type fixedPrincipalResolver struct {
	principal transporthttp.Principal
}

func (r fixedPrincipalResolver) Resolve(*http.Request) (transporthttp.Principal, error) {
	return r.principal, nil
}

type fixedUserIDResolver struct {
	userID int
	err    error
}

func (r fixedUserIDResolver) UserID(*http.Request) (int, error) {
	return r.userID, r.err
}

type fixedSCRMUserStore struct {
	user dashboard.User
	err  error
}

func (s fixedSCRMUserStore) UserByID(_ context.Context, userID int) (dashboard.User, bool, error) {
	if s.err != nil {
		return dashboard.User{}, false, s.err
	}
	if userID != s.user.ID {
		return dashboard.User{}, false, nil
	}
	return s.user, true, nil
}
