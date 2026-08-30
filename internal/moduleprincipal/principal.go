package moduleprincipal

import (
	"context"
	"net/http"
)

type Principal struct {
	UserID   int64
	TenantID int64
	CorpID   int64
}

type Resolver interface {
	Resolve(*http.Request) (Principal, error)
}
type Authorizer interface {
	Authorize(context.Context, Principal, int64, string) error
}
