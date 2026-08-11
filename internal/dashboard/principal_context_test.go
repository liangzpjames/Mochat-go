package dashboard

import (
	"context"
	"net/http"
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

	corpID, ok := principalCorpID(req)
	if !ok || corpID != 77 {
		t.Fatalf("corpID=%d ok=%v, want server principal corp 77", corpID, ok)
	}
}

func TestPrincipalCorpIDFailsClosedWithoutPrincipal(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "/dashboard/workMessage/index?corpId=77", nil)
	if err != nil {
		t.Fatal(err)
	}
	if corpID, ok := principalCorpID(req); ok || corpID != 0 {
		t.Fatalf("corpID=%d ok=%v, want fail closed", corpID, ok)
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
