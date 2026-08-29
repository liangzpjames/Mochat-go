package dashboard

import (
	"net/http"
	"testing"
)

func TestDashboardRouteRegistryIsTypedUniqueAndComplete(t *testing.T) {
	routes := DashboardRouteRegistry()
	if len(routes) < 500 {
		t.Fatalf("route registry has %d entries, want the complete Dashboard surface", len(routes))
	}
	seen := map[string]bool{}
	for _, route := range routes {
		contract := route.Method + " " + route.Path
		if route.Method == "" || route.Path == "" || route.Handler == "" || route.AuthKind == "" {
			t.Fatalf("incomplete route metadata: %+v", route)
		}
		if seen[contract] {
			t.Fatalf("duplicate route metadata: %s", contract)
		}
		seen[contract] = true
	}
}

func TestDashboardSecurityMFARoutesAreIdentityAuthenticated(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPut} {
		route, ok := LookupDashboardRoute(method, "/dashboard/user/securityMFA")
		if !ok || route.AuthKind != DashboardRouteAuthIdentity {
			t.Fatalf("%s securityMFA metadata = %+v found=%t", method, route, ok)
		}
	}
}

func TestDashboardRouteRegistryReturnsDefensiveCopy(t *testing.T) {
	routes := DashboardRouteRegistry()
	before, ok := LookupDashboardRoute(routes[0].Method, routes[0].Path)
	if !ok {
		t.Fatal("first route is not indexed")
	}
	routes[0].AuthKind = DashboardRouteAuthPublic
	after, ok := LookupDashboardRoute(before.Method, before.Path)
	if !ok || after != before {
		t.Fatalf("registry was externally mutated: before=%+v after=%+v", before, after)
	}
}
