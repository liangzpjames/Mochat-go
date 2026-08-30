package aiinsight

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	appmodules "jiyi/mochat-go/internal/app/modules"
	transporthttp "jiyi/mochat-go/internal/modules/ai-insight/transport/http"
)

type modulePrincipalResolver struct{}

func (modulePrincipalResolver) Resolve(*http.Request) (transporthttp.Principal, error) {
	return transporthttp.Principal{UserID: 7, TenantID: 11, CorpID: 22, IsSuperAdmin: true}, nil
}

func TestModuleRegistersWorkspaceRoutesWithoutAIProviderResolver(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	module, err := New(Dependencies{
		PrincipalResolver: modulePrincipalResolver{},
		DB:                db,
		AssistantContext:  struct{}{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if module.workspace == nil {
		t.Fatal("workspace handler is nil")
	}

	router := appmodules.NewRouter()
	if err := module.RegisterRoutes(router); err != nil {
		t.Fatal(err)
	}

	for _, route := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/dashboard/ai-insight/session-analysis/records"},
		{http.MethodGet, "/dashboard/ai-insight/session-analysis/status"},
		{http.MethodPost, "/dashboard/ai-insight/run"},
	} {
		if _, ok := router.Match(httptest.NewRequest(route.method, route.path, nil)); !ok {
			t.Errorf("route %s %s did not match", route.method, route.path)
		}
	}
}
