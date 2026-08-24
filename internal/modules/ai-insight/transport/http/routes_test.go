package http

import (
	nethttp "net/http"
	"testing"

	appmodules "jiyi/mochat-go/internal/app/modules"
)

func TestRegisterWorkspaceRoutesIncludesScopedFilterOptions(t *testing.T) {
	router := appmodules.NewRouter()
	if err := RegisterWorkspaceRoutes(router, nethttp.HandlerFunc(func(nethttp.ResponseWriter, *nethttp.Request) {})); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"/dashboard/ai-insight/session-analysis/filter-options",
		"/dashboard/ai-insight/smart-analysis/filter-options",
	} {
		request, err := nethttp.NewRequest(nethttp.MethodGet, path, nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := router.Match(request); !ok {
			t.Fatalf("route not registered: GET %s", path)
		}
	}
}
