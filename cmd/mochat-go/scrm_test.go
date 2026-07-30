package main

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	"jiyi/mochat-go/internal/config"
	"jiyi/mochat-go/internal/dashboard"
	"jiyi/mochat-go/internal/store"

	transporthttp "jiyi/mochat-go/internal/modules/scrm/transport/http"
)

func TestNewSCRMModuleRouterDisabledDoesNotResolveDependencies(t *testing.T) {
	router, err := newSCRMModuleRouter(config.Config{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	assertMainSCRMRouteMissing(t, router, http.MethodPost)
	assertMainSCRMRouteMissing(t, router, http.MethodGet)
}

func TestNewSCRMModuleRouterEnabledInstallsBothRoutes(t *testing.T) {
	db := mainTestDB(t)
	mysqlStore := store.NewMySQLStore(db)
	buildUserResolver := func(string) (dashboard.UserIDResolver, dashboard.LoginCache) {
		return mainFixedUserIDResolver{userID: 7}, nil
	}

	router, err := newSCRMModuleRouter(
		config.Config{EnablePhase22SCRMPilot: true},
		func() *store.MySQLStore { return mysqlStore },
		buildUserResolver,
	)
	if err != nil {
		t.Fatal(err)
	}
	assertMainSCRMRouteInstalled(t, router, http.MethodPost)
	assertMainSCRMRouteInstalled(t, router, http.MethodGet)
}

func assertMainSCRMRouteInstalled(t *testing.T, router routeMatcher, method string) {
	t.Helper()
	request := httptest.NewRequest(method, transporthttp.LeadsPath, nil)
	if handler, ok := router.Match(request); !ok || handler == nil {
		t.Fatalf("%s %s was not installed", method, transporthttp.LeadsPath)
	}
}

func assertMainSCRMRouteMissing(t *testing.T, router routeMatcher, method string) {
	t.Helper()
	request := httptest.NewRequest(method, transporthttp.LeadsPath, nil)
	if handler, ok := router.Match(request); ok || handler != nil {
		t.Fatalf("%s %s was installed", method, transporthttp.LeadsPath)
	}
}

type routeMatcher interface {
	Match(*http.Request) (http.Handler, bool)
}

func mainTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("mysql", "unused:unused@tcp(localhost:3306)/unused")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

type mainFixedUserIDResolver struct {
	userID int
}

func (r mainFixedUserIDResolver) UserID(*http.Request) (int, error) {
	return r.userID, nil
}
