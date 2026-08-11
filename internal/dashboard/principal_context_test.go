package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"jiyi/mochat-go/internal/dashboardprincipal"
)

func TestPrincipalCorpIDIgnoresClientCorpInputs(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "/dashboard/workMessage/index?corpId=999", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Corp-ID", "999")
	req.Header.Set("X-Mochat-Go-Corp-ID", "999")
	req = req.WithContext(dashboardprincipal.WithPrincipal(req.Context(), dashboardprincipal.DashboardPrincipal{
		UserID: 7, TenantID: 902, CorpID: 77, CorpStatus: dashboardprincipal.CorpBindingStatusActive, AuthVersion: 4,
	}))

	corpID, ok := principalCorpID(httptest.NewRecorder(), req)
	if !ok || corpID != 77 {
		t.Fatalf("corpID=%d ok=%v, want server principal corp 77", corpID, ok)
	}
}

func TestPrincipalCorpIDFailsClosedWithoutPrincipal(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "/dashboard/workMessage/index?corpId=77", nil)
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	if corpID, ok := principalCorpID(recorder, req); ok || corpID != 0 {
		t.Fatalf("corpID=%d ok=%v, want fail closed", corpID, ok)
	}
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s, want stable 401", recorder.Code, recorder.Body.String())
	}
}

func TestPrincipalCorpIDWritesForbiddenForSuspendedPrincipal(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/dashboard/workMessage/index", nil)
	req = req.WithContext(dashboardprincipal.WithPrincipal(req.Context(), dashboardprincipal.DashboardPrincipal{
		UserID: 7, TenantID: 902, CorpID: 77, CorpStatus: dashboardprincipal.CorpBindingStatusSuspended, AuthVersion: 4,
	}))
	recorder := httptest.NewRecorder()
	if corpID, ok := principalCorpID(recorder, req); ok || corpID != 0 {
		t.Fatalf("corpID=%d ok=%v, want suspended principal rejected", corpID, ok)
	}
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s, want stable 403", recorder.Code, recorder.Body.String())
	}
}

func TestPrincipalCorpIDRejectsInconsistentAccessContext(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/dashboard/workMessage/index", nil)
	req = req.WithContext(dashboardprincipal.WithPrincipal(req.Context(), dashboardprincipal.DashboardPrincipal{
		UserID: 7, TenantID: 902, CorpID: 77, CorpStatus: dashboardprincipal.CorpBindingStatusActive, AuthVersion: 4,
	}))
	req = req.WithContext(WithDashboardAccessContext(req.Context(), DashboardAccessContext{
		UserID: 7, TenantID: 902, CorpID: 88, WorkEmployeeID: 31,
	}))
	recorder := httptest.NewRecorder()
	if corpID, ok := principalCorpID(recorder, req); ok || corpID != 0 {
		t.Fatalf("corpID=%d ok=%v, want inconsistent access context rejected", corpID, ok)
	}
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s, want stable 401", recorder.Code, recorder.Body.String())
	}
}

func TestDashboardRequestScopeDerivesOneCorpFromPrincipalAndAccessScope(t *testing.T) {
	ctx := dashboardprincipal.WithPrincipal(context.Background(), dashboardprincipal.DashboardPrincipal{
		UserID: 7, TenantID: 902, CorpID: 77, CorpStatus: dashboardprincipal.CorpBindingStatusPending, AuthVersion: 4,
	})
	ctx = WithDashboardAccessContext(ctx, DashboardAccessContext{
		UserID: 7, TenantID: 902, CorpID: 77, WorkEmployeeID: 31, AllowedEmployeeIDs: []int{31, 32},
	})
	scope, err := DashboardRequestScopeFromContext(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if scope.Principal.CorpID != 77 || len(scope.CorpIDs) != 1 || scope.CorpIDs[0] != 77 || scope.WorkEmployeeID != 31 {
		t.Fatalf("scope=%+v, want principal-derived corp and RBAC employee scope", scope)
	}
}
