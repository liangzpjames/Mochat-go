package http

import nethttp "net/http"

type Principal struct {
	UserID   int64
	TenantID int64
}

type PrincipalResolver interface {
	Resolve(*nethttp.Request) (Principal, error)
}
