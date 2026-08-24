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

func TestProjectionRegisterWorkspaceRoutesExposesExactlyFiveReadActionsPerDerivedView(t *testing.T) {
	router := appmodules.NewRouter()
	if err := RegisterWorkspaceRoutes(router, nethttp.HandlerFunc(func(nethttp.ResponseWriter, *nethttp.Request) {})); err != nil {
		t.Fatal(err)
	}
	for _, view := range []string{"emotion", "employee-score", "communication-keyword"} {
		for _, action := range []string{"records", "detail", "status", "filter-options", "export"} {
			path := "/dashboard/ai-insight/" + view + "/" + action
			request, err := nethttp.NewRequest(nethttp.MethodGet, path, nil)
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := router.Match(request); !ok {
				t.Errorf("derived route not registered: GET %s", path)
			}
		}
	}
}

func TestProjectionRegisterWorkspaceRoutesNeverRegistersDerivedWrites(t *testing.T) {
	router := appmodules.NewRouter()
	if err := RegisterWorkspaceRoutes(router, nethttp.HandlerFunc(func(nethttp.ResponseWriter, *nethttp.Request) {})); err != nil {
		t.Fatal(err)
	}
	for _, view := range []string{"emotion", "employee-score", "communication-keyword"} {
		for _, action := range []string{"records", "detail", "status", "filter-options", "export", "rules", "rules/status"} {
			for _, method := range []string{nethttp.MethodPost, nethttp.MethodPut, nethttp.MethodPatch, nethttp.MethodDelete} {
				path := "/dashboard/ai-insight/" + view + "/" + action
				request, err := nethttp.NewRequest(method, path, nil)
				if err != nil {
					t.Fatal(err)
				}
				if _, ok := router.Match(request); ok {
					t.Errorf("derived write route must not be registered: %s %s", method, path)
				}
			}
		}
	}
}
