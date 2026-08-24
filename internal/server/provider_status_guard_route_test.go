package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"jiyi/mochat-go/internal/config"
	"jiyi/mochat-go/internal/dashboard"
	"jiyi/mochat-go/internal/dashboardprincipal"
)

type providerStatusGuardStore struct {
	identity dashboard.DashboardAccessIdentity
}

func (store *providerStatusGuardStore) DashboardAccessIdentity(context.Context, int) (dashboard.DashboardAccessIdentity, bool, error) {
	return store.identity, true, nil
}

func (*providerStatusGuardStore) DashboardPermissionCatalog(context.Context) ([]dashboard.DashboardPermissionDefinition, error) {
	return []dashboard.DashboardPermissionDefinition{{
		ID: 49, Code: "dashboard.company_setting.website", Path: "/company-setting/website", Name: "唯一企业资料",
	}}, nil
}

func (*providerStatusGuardStore) DashboardPermissionGrants(context.Context, int, int) ([]dashboard.DashboardPermissionGrantFact, error) {
	return nil, nil
}

func (*providerStatusGuardStore) DashboardEmployeeScope(context.Context, int, int, int) (dashboard.DashboardEmployeeScope, bool, error) {
	return dashboard.DashboardEmployeeScope{}, false, nil
}

func (*providerStatusGuardStore) DashboardTenantAccess(_ context.Context, tenantID int, _ time.Time) (dashboard.DashboardTenantAccess, error) {
	return dashboard.DashboardTenantAccess{TenantID: tenantID, Allowed: true}, nil
}

func (*providerStatusGuardStore) DashboardPermissionResources(_ context.Context, method string) ([]dashboard.DashboardPermissionResource, error) {
	if method != http.MethodGet {
		return nil, nil
	}
	return []dashboard.DashboardPermissionResource{{
		PermissionCode: "dashboard.company_setting.website", Method: http.MethodGet,
		PathPattern: "/dashboard/providers/status", ScopeRequired: false,
	}}, nil
}

func TestProviderStatusRouteHonorsRealDashboardGuard(t *testing.T) {
	tests := []struct {
		name        string
		superadmin  bool
		corpStatus  dashboardprincipal.CorpBindingStatus
		wantStatus  int
		wantAllowed bool
	}{
		{name: "active superadmin", superadmin: true, corpStatus: dashboardprincipal.CorpBindingStatusActive, wantStatus: http.StatusOK, wantAllowed: true},
		{name: "active ordinary user", corpStatus: dashboardprincipal.CorpBindingStatusActive, wantStatus: http.StatusForbidden},
		{name: "pending superadmin", superadmin: true, corpStatus: dashboardprincipal.CorpBindingStatusPending, wantStatus: http.StatusOK, wantAllowed: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &providerStatusGuardStore{identity: dashboard.DashboardAccessIdentity{
				UserID: 7, TenantID: 9, UserName: "admin", Status: 1, IsSuperAdmin: test.superadmin,
			}}
			guard := dashboard.NewDashboardAccessGuard(store, nil)
			srv, err := New(config.Config{},
				WithDashboardRequestGuard(guard),
				WithProviderStatusHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					access, ok := dashboard.DashboardAccessFromContext(r.Context())
					if !ok {
						http.Error(w, "missing access context", http.StatusInternalServerError)
						return
					}
					_ = json.NewEncoder(w).Encode(map[string]int{"tenantId": access.TenantID, "corpId": access.CorpID})
				})),
			)
			if err != nil {
				t.Fatal(err)
			}
			principal := dashboardprincipal.DashboardPrincipal{
				UserID: 7, TenantID: 9, CorpID: 12, AuthVersion: 1,
				CorpStatus: test.corpStatus, IsSuperAdmin: test.superadmin,
			}
			request := httptest.NewRequest(http.MethodGet, "/dashboard/providers/status?tenantId=999&corpId=888", nil)
			request = request.WithContext(dashboardprincipal.WithPrincipal(request.Context(), principal))
			response := httptest.NewRecorder()
			srv.ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("status=%d body=%s want=%d", response.Code, response.Body.String(), test.wantStatus)
			}
			if !test.wantAllowed {
				return
			}
			var body map[string]int
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body["tenantId"] != 9 || body["corpId"] != 12 {
				t.Fatalf("scope=%v, query parameters must not override principal", body)
			}
		})
	}
}
