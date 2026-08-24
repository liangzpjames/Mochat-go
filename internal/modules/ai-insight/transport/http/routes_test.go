package http

import (
	nethttp "net/http"
	"reflect"
	"sort"
	"strings"
	"testing"

	appmodules "jiyi/mochat-go/internal/app/modules"
)

type recordedWorkspaceRoute struct {
	method string
	path   string
}

type recordingWorkspaceRegistrar struct {
	routes []recordedWorkspaceRoute
}

func (r *recordingWorkspaceRegistrar) Handle(method, path string, _ nethttp.Handler) error {
	r.routes = append(r.routes, recordedWorkspaceRoute{method: method, path: path})
	return nil
}

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
	registrar := &recordingWorkspaceRegistrar{}
	if err := RegisterWorkspaceRoutes(registrar, nethttp.HandlerFunc(func(nethttp.ResponseWriter, *nethttp.Request) {})); err != nil {
		t.Fatal(err)
	}
	for _, view := range []string{"emotion", "employee-score", "communication-keyword"} {
		prefix := "/dashboard/ai-insight/" + view + "/"
		actual := make([]string, 0)
		for _, route := range registrar.routes {
			if strings.HasPrefix(route.path, prefix) {
				actual = append(actual, route.method+" "+strings.TrimPrefix(route.path, prefix))
			}
		}
		sort.Strings(actual)
		want := []string{"GET detail", "GET export", "GET filter-options", "GET records", "GET status"}
		if !reflect.DeepEqual(actual, want) {
			t.Errorf("%s registered contract=%#v, want exactly %#v", view, actual, want)
		}
	}
}

func TestProjectionRegisterWorkspaceRoutesNeverRegistersDerivedWrites(t *testing.T) {
	router := appmodules.NewRouter()
	if err := RegisterWorkspaceRoutes(router, nethttp.HandlerFunc(func(nethttp.ResponseWriter, *nethttp.Request) {})); err != nil {
		t.Fatal(err)
	}
	for _, view := range []string{"emotion", "employee-score", "communication-keyword"} {
		for _, action := range []string{"rules", "rule-status", "rules/status", "arbitrary-suffix"} {
			path := "/dashboard/ai-insight/" + view + "/" + action
			request, err := nethttp.NewRequest(nethttp.MethodGet, path, nil)
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := router.Match(request); ok {
				t.Errorf("extra derived GET route must not be registered: GET %s", path)
			}
		}
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
