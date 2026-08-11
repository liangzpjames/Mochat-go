package dashboardprincipal

import (
	"context"
	"errors"
	"testing"
)

func TestDashboardPrincipalRoundTripsThroughContext(t *testing.T) {
	want := DashboardPrincipal{
		UserID:       7,
		TenantID:     11,
		CorpID:       13,
		CorpStatus:   CorpBindingStatusActive,
		IsSuperAdmin: true,
		AuthVersion:  2,
	}

	ctx := WithPrincipal(context.Background(), want)
	got, err := DashboardPrincipalFromContext(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("principal = %+v, want %+v", got, want)
	}
}

func TestDashboardPrincipalFromContextFailsClosed(t *testing.T) {
	cases := []struct {
		name string
		ctx  context.Context
	}{
		{name: "missing", ctx: context.Background()},
		{name: "wrong type", ctx: context.WithValue(context.Background(), principalContextKey{}, "not a principal")},
		{name: "zero tenant", ctx: context.WithValue(context.Background(), principalContextKey{}, DashboardPrincipal{UserID: 7, CorpID: 13, AuthVersion: 1})},
		{name: "zero corp", ctx: context.WithValue(context.Background(), principalContextKey{}, DashboardPrincipal{UserID: 7, TenantID: 11, AuthVersion: 1})},
		{name: "zero user", ctx: context.WithValue(context.Background(), principalContextKey{}, DashboardPrincipal{TenantID: 11, CorpID: 13, AuthVersion: 1})},
		{name: "zero auth version", ctx: context.WithValue(context.Background(), principalContextKey{}, DashboardPrincipal{UserID: 7, TenantID: 11, CorpID: 13})},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := DashboardPrincipalFromContext(tc.ctx)
			if !errors.Is(err, ErrPrincipalUnavailable) {
				t.Fatalf("err = %v, want ErrPrincipalUnavailable", err)
			}
		})
	}
}

func TestWithPrincipalDoesNotStoreInvalidPrincipal(t *testing.T) {
	ctx := WithPrincipal(context.Background(), DashboardPrincipal{UserID: 7, TenantID: 0, CorpID: 13, AuthVersion: 1})
	if _, err := DashboardPrincipalFromContext(ctx); !errors.Is(err, ErrPrincipalUnavailable) {
		t.Fatalf("err = %v, want invalid principal to fail closed", err)
	}

	valid := DashboardPrincipal{UserID: 7, TenantID: 11, CorpID: 13, AuthVersion: 1}
	ctx = WithPrincipal(context.Background(), valid)
	ctx = WithPrincipal(ctx, DashboardPrincipal{UserID: 7, TenantID: 0, CorpID: 13, AuthVersion: 1})
	if _, err := DashboardPrincipalFromContext(ctx); !errors.Is(err, ErrPrincipalUnavailable) {
		t.Fatalf("err = %v, want invalid replacement to shadow the prior principal", err)
	}
}
