package store

import (
	"context"
	"errors"
	"strings"
	"testing"

	"jiyi/mochat-go/internal/dashboardprincipal"
)

func TestTenantCorpBindingStoreRequiresExactlyOneBindingAndMatchingCorpTenant(t *testing.T) {
	cases := []struct {
		name    string
		row     identityTestRow
		wantErr bool
	}{
		{name: "one matching binding", row: identityTestRow{values: []any{int64(1), int64(902), int64(77), int64(2), int64(3), int64(902)}}},
		{name: "missing", row: identityTestRow{values: []any{int64(0), int64(0), int64(0), int64(0), int64(0), int64(0)}}, wantErr: true},
		{name: "duplicate", row: identityTestRow{values: []any{int64(2), int64(902), int64(77), int64(2), int64(3), int64(902)}}, wantErr: true},
		{name: "corp belongs to another tenant", row: identityTestRow{values: []any{int64(1), int64(902), int64(77), int64(2), int64(3), int64(903)}}, wantErr: true},
		{name: "corp missing", row: identityTestRow{values: []any{int64(1), int64(902), int64(77), int64(2), int64(3), int64(0)}}, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var query string
			store := &TenantCorpBindingStore{
				queryRow: func(_ context.Context, statement string, _ ...any) identityRowScanner {
					query = statement
					return tc.row
				},
			}
			binding, err := store.ResolveBinding(context.Background(), 902)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr=%v", err, tc.wantErr)
			}
			if !strings.Contains(query, "COUNT(*)") || !strings.Contains(query, "mochat_go_tenant_corp_bindings") || !strings.Contains(query, "mc_corp") {
				t.Fatalf("binding query does not prove uniqueness and corp ownership: %s", query)
			}
			if !tc.wantErr && binding != (dashboardprincipal.Binding{TenantID: 902, CorpID: 77, Status: dashboardprincipal.CorpBindingStatusActive, Version: 3, Count: 1}) {
				t.Fatalf("binding=%+v", binding)
			}
		})
	}
}

func TestTenantCorpBindingStoreReturnsSuspendedFactForResolverToReject(t *testing.T) {
	store := &TenantCorpBindingStore{
		queryRow: func(context.Context, string, ...any) identityRowScanner {
			return identityTestRow{values: []any{int64(1), int64(902), int64(77), int64(3), int64(4), int64(902)}}
		},
	}
	binding, err := store.ResolveBinding(context.Background(), 902)
	if err != nil {
		t.Fatal(err)
	}
	if binding.Status != dashboardprincipal.CorpBindingStatusSuspended {
		t.Fatalf("status=%q, want suspended fact", binding.Status)
	}
}

func TestTenantCorpBindingStoreRejectsUnknownStatus(t *testing.T) {
	store := &TenantCorpBindingStore{
		queryRow: func(context.Context, string, ...any) identityRowScanner {
			return identityTestRow{values: []any{int64(1), int64(902), int64(77), int64(9), int64(4), int64(902)}}
		},
	}
	if _, err := store.ResolveBinding(context.Background(), 902); !errors.Is(err, dashboardprincipal.ErrBindingUnavailable) {
		t.Fatalf("err=%v, want ErrBindingUnavailable", err)
	}
}

func TestDashboardIdentityStoreResolveIdentityUsesIdentityAuthVersionAndBusinessTenantFact(t *testing.T) {
	var query string
	store := &DashboardIdentityStore{
		queryRow: func(_ context.Context, statement string, _ ...any) identityRowScanner {
			query = statement
			return identityTestRow{values: []any{7, 902, 1, int64(1), uint64(4)}}
		},
	}
	identity, err := store.ResolveIdentity(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if identity.UserID != 7 || identity.TenantID != 902 || identity.AuthVersion != 4 || !identity.Active || !identity.IsSuperAdmin {
		t.Fatalf("identity=%+v", identity)
	}
	if !strings.Contains(query, "mochat_go_dashboard_identities") || !strings.Contains(query, "mc_user") || !strings.Contains(query, "auth_version") || strings.Contains(strings.ToLower(query), "password") {
		t.Fatalf("identity query does not prove auth identity and tenant facts: %s", query)
	}
}
