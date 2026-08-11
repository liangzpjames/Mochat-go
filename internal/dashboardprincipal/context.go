package dashboardprincipal

import (
	"context"
	"errors"
)

type CorpBindingStatus string

const (
	CorpBindingStatusPending   CorpBindingStatus = "pending"
	CorpBindingStatusActive    CorpBindingStatus = "active"
	CorpBindingStatusSuspended CorpBindingStatus = "suspended"
)

type DashboardPrincipal struct {
	UserID       int
	TenantID     int
	CorpID       int
	CorpStatus   CorpBindingStatus
	IsSuperAdmin bool
	AuthVersion  uint64
}

var ErrPrincipalUnavailable = errors.New("dashboard principal unavailable")

type principalContextKey struct{}

func WithPrincipal(ctx context.Context, principal DashboardPrincipal) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if !validPrincipal(principal) {
		return context.WithValue(ctx, principalContextKey{}, DashboardPrincipal{})
	}
	return context.WithValue(ctx, principalContextKey{}, principal)
}

func DashboardPrincipalFromContext(ctx context.Context) (DashboardPrincipal, error) {
	if ctx == nil {
		return DashboardPrincipal{}, ErrPrincipalUnavailable
	}
	principal, ok := ctx.Value(principalContextKey{}).(DashboardPrincipal)
	if !ok || !validPrincipal(principal) {
		return DashboardPrincipal{}, ErrPrincipalUnavailable
	}
	return principal, nil
}

func validPrincipal(principal DashboardPrincipal) bool {
	return principal.UserID > 0 && principal.TenantID > 0 && principal.CorpID > 0 && principal.AuthVersion > 0
}
