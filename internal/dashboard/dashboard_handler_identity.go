package dashboard

import (
	"context"

	"jiyi/mochat-go/internal/dashboardprincipal"
)

// DashboardHandlerIdentity is the server-owned identity projection available
// to Dashboard business handlers after request and access guards have run.
type DashboardHandlerIdentity struct {
	UserID         int
	TenantID       int
	CorpID         int
	WorkEmployeeID int
}

// ResolveDashboardHandlerIdentity fails closed unless the authenticated
// principal and authorized access context are both present and consistent.
func ResolveDashboardHandlerIdentity(ctx context.Context) (DashboardHandlerIdentity, error) {
	principal, err := DashboardPrincipalFromContext(ctx)
	if err != nil {
		return DashboardHandlerIdentity{}, dashboardprincipal.ErrPrincipalUnavailable
	}
	access, ok := DashboardAccessFromContext(ctx)
	if !ok || access.UserID != principal.UserID || access.TenantID != principal.TenantID || access.CorpID != principal.CorpID {
		return DashboardHandlerIdentity{}, dashboardprincipal.ErrPrincipalUnavailable
	}
	return DashboardHandlerIdentity{
		UserID:         principal.UserID,
		TenantID:       principal.TenantID,
		CorpID:         principal.CorpID,
		WorkEmployeeID: access.WorkEmployeeID,
	}, nil
}
