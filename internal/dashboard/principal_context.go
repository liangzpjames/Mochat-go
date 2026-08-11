package dashboard

import (
	"context"
	"net/http"

	"jiyi/mochat-go/internal/dashboardprincipal"
)

// DashboardRequestScope is a request-local projection of the authenticated
// DashboardPrincipal plus page-RBAC employee scope. CorpIDs is derived only
// from Principal.CorpID; it is never populated from request input or a cache.
type DashboardRequestScope struct {
	Principal      dashboardprincipal.DashboardPrincipal
	CorpIDs        []int
	WorkEmployeeID int
	RequestSource  int
}

func DashboardPrincipalFromContext(ctx context.Context) (dashboardprincipal.DashboardPrincipal, error) {
	return dashboardprincipal.DashboardPrincipalFromContext(ctx)
}

func DashboardRequestScopeFromContext(ctx context.Context) (DashboardRequestScope, error) {
	principal, err := DashboardPrincipalFromContext(ctx)
	if err != nil {
		return DashboardRequestScope{}, err
	}
	scope := DashboardRequestScope{
		Principal:     principal,
		CorpIDs:       []int{principal.CorpID},
		RequestSource: 1,
	}
	if access, ok := DashboardAccessFromContext(ctx); ok {
		if access.UserID != principal.UserID || access.TenantID != principal.TenantID || access.CorpID != principal.CorpID {
			return DashboardRequestScope{}, dashboardprincipal.ErrPrincipalUnavailable
		}
		scope.WorkEmployeeID = access.WorkEmployeeID
	}
	return scope, nil
}

func principalCorpID(request *http.Request) (int, bool) {
	if request == nil {
		return 0, false
	}
	principal, err := DashboardPrincipalFromContext(request.Context())
	if err != nil || principal.CorpID <= 0 || principal.CorpStatus == dashboardprincipal.CorpBindingStatusSuspended {
		return 0, false
	}
	return principal.CorpID, true
}
