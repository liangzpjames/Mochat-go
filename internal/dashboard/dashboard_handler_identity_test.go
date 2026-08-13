package dashboard

import (
	"context"
	"errors"
	"testing"

	"jiyi/mochat-go/internal/dashboardprincipal"
)

func TestResolveDashboardHandlerIdentity(t *testing.T) {
	principal := dashboardprincipal.DashboardPrincipal{
		UserID:      7,
		TenantID:    11,
		CorpID:      13,
		CorpStatus:  dashboardprincipal.CorpBindingStatusActive,
		AuthVersion: 1,
	}
	matchingAccess := DashboardAccessContext{
		UserID:         7,
		TenantID:       11,
		CorpID:         13,
		WorkEmployeeID: 17,
	}

	t.Run("returns the server-owned request identity", func(t *testing.T) {
		ctx := dashboardprincipal.WithPrincipal(context.Background(), principal)
		ctx = WithDashboardAccessContext(ctx, matchingAccess)

		got, err := ResolveDashboardHandlerIdentity(ctx)
		if err != nil {
			t.Fatalf("ResolveDashboardHandlerIdentity() error = %v", err)
		}
		want := (DashboardHandlerIdentity{UserID: 7, TenantID: 11, CorpID: 13, WorkEmployeeID: 17})
		if got != want {
			t.Fatalf("ResolveDashboardHandlerIdentity() = %+v, want %+v", got, want)
		}
	})

	tests := []struct {
		name   string
		ctx    context.Context
		access DashboardAccessContext
	}{
		{name: "missing principal", ctx: context.Background(), access: matchingAccess},
		{name: "missing access", ctx: dashboardprincipal.WithPrincipal(context.Background(), principal)},
		{name: "mismatched user", ctx: dashboardprincipal.WithPrincipal(context.Background(), principal), access: DashboardAccessContext{UserID: 8, TenantID: 11, CorpID: 13}},
		{name: "mismatched tenant", ctx: dashboardprincipal.WithPrincipal(context.Background(), principal), access: DashboardAccessContext{UserID: 7, TenantID: 12, CorpID: 13}},
		{name: "mismatched corp", ctx: dashboardprincipal.WithPrincipal(context.Background(), principal), access: DashboardAccessContext{UserID: 7, TenantID: 11, CorpID: 14}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := tc.ctx
			if tc.name != "missing access" {
				ctx = WithDashboardAccessContext(ctx, tc.access)
			}

			got, err := ResolveDashboardHandlerIdentity(ctx)
			if !errors.Is(err, dashboardprincipal.ErrPrincipalUnavailable) {
				t.Fatalf("ResolveDashboardHandlerIdentity() error = %v, want ErrPrincipalUnavailable", err)
			}
			if got != (DashboardHandlerIdentity{}) {
				t.Fatalf("ResolveDashboardHandlerIdentity() = %+v, want zero value", got)
			}
		})
	}
}

func TestResolveDashboardHandlerIdentityRejectsNonPositiveCoreIDs(t *testing.T) {
	validPrincipal := dashboardprincipal.DashboardPrincipal{UserID: 7, TenantID: 11, CorpID: 13}
	validAccess := DashboardAccessContext{UserID: 7, TenantID: 11, CorpID: 13, WorkEmployeeID: 17}
	tests := []struct {
		name      string
		principal dashboardprincipal.DashboardPrincipal
		access    DashboardAccessContext
	}{
		{name: "zero user", principal: dashboardprincipal.DashboardPrincipal{UserID: 0, TenantID: 11, CorpID: 13}, access: DashboardAccessContext{UserID: 0, TenantID: 11, CorpID: 13}},
		{name: "zero tenant", principal: dashboardprincipal.DashboardPrincipal{UserID: 7, TenantID: 0, CorpID: 13}, access: DashboardAccessContext{UserID: 7, TenantID: 0, CorpID: 13}},
		{name: "zero corp", principal: dashboardprincipal.DashboardPrincipal{UserID: 7, TenantID: 11, CorpID: 0}, access: DashboardAccessContext{UserID: 7, TenantID: 11, CorpID: 0}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveDashboardHandlerIdentity(tc.principal, tc.access, true)
			if !errors.Is(err, dashboardprincipal.ErrPrincipalUnavailable) {
				t.Fatalf("resolveDashboardHandlerIdentity() error = %v, want ErrPrincipalUnavailable", err)
			}
			if got != (DashboardHandlerIdentity{}) {
				t.Fatalf("resolveDashboardHandlerIdentity() = %+v, want zero value", got)
			}
		})
	}

	t.Run("zero work employee remains valid", func(t *testing.T) {
		validAccess.WorkEmployeeID = 0
		got, err := resolveDashboardHandlerIdentity(validPrincipal, validAccess, true)
		if err != nil {
			t.Fatalf("resolveDashboardHandlerIdentity() error = %v", err)
		}
		if got.WorkEmployeeID != 0 {
			t.Fatalf("WorkEmployeeID = %d, want 0", got.WorkEmployeeID)
		}
	})
}
