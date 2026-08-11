package http

import (
	"errors"
	nethttp "net/http"
)

var (
	ErrPrincipalUnauthorized = errors.New("principal unauthorized")
	ErrPrincipalUnavailable  = errors.New("principal unavailable")
	ErrCorpPrincipalMismatch = errors.New("corp does not match dashboard principal")
)

type Principal struct {
	UserID                  int64
	TenantID                int64
	CorpID                  int64
	WorkEmployeeID          int64
	AllowedEmployeeIDs      []int64
	EmployeeScopeRestricted bool
}

// ResolveCorp keeps the Dashboard binding authoritative. A request may omit
// corpId, but it may never select a different corp than the server principal.
func (p Principal) ResolveCorp(assertion int64) (int64, error) {
	if p.CorpID <= 0 {
		return 0, ErrPrincipalUnauthorized
	}
	if assertion > 0 && assertion != p.CorpID {
		return 0, ErrCorpPrincipalMismatch
	}
	return p.CorpID, nil
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
