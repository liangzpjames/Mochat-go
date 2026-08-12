package dashboard

import "context"

// CorpAdminAuthorizer is the shared data/page authorization contract used by
// Dashboard handlers. It does not select or bind a company; the principal and
// request scope are resolved by the Dashboard identity middleware.
type CorpAdminAuthorizer interface {
	Resolve(ctx context.Context, userID int, permissionKey string, corpID int, workEmployeeID int) (AccessContext, error)
}
