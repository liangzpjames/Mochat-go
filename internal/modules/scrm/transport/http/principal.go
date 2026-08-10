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
	UserID                  int64
	TenantID                int64
	WorkEmployeeID          int64
	AllowedEmployeeIDs      []int64
	EmployeeScopeRestricted bool
}

func (p Principal) AllowsEmployee(id int64) bool {
	if !p.EmployeeScopeRestricted {
		return true
	}
	for _, allowed := range p.AllowedEmployeeIDs {
		if allowed == id {
			return true
		}
	}
	return false
}

func (p Principal) EmployeeDataOperationAllowed() bool {
	return !p.EmployeeScopeRestricted || len(p.AllowedEmployeeIDs) > 0
}

type PrincipalResolver interface {
	Resolve(*nethttp.Request) (Principal, error)
}
