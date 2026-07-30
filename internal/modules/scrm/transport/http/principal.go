package http

import (
	"errors"
	nethttp "net/http"
)

var (
	ErrPrincipalUnauthorized = errors.New("principal unauthorized")
	ErrPrincipalUnavailable  = errors.New("principal unavailable")
)

type Principal struct {
	UserID   int64
	TenantID int64
}

type PrincipalResolver interface {
	Resolve(*nethttp.Request) (Principal, error)
}
