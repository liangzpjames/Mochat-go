// Package runtime defines which process responsibilities a MoChat Go
// deployment runs. It deliberately contains no business or infrastructure
// dependencies so every entrypoint can share the same role semantics.
package runtime

import (
	"fmt"
	"strings"
)

type Role string

const (
	RoleAll       Role = "all"
	RoleAPI       Role = "api"
	RoleWorker    Role = "worker"
	RoleScheduler Role = "scheduler"
)

func ParseRole(raw string) (Role, error) {
	role := Role(strings.ToLower(strings.TrimSpace(raw)))
	if role == "" {
		return RoleAll, nil
	}
	switch role {
	case RoleAll, RoleAPI, RoleWorker, RoleScheduler:
		return role, nil
	default:
		return "", fmt.Errorf("invalid runtime role %q: expected all, api, worker, or scheduler", raw)
	}
}

func (r Role) RunsAPI() bool {
	return r == RoleAll || r == RoleAPI
}

func (r Role) RunsWorkers() bool {
	return r == RoleAll || r == RoleWorker
}

func (r Role) RunsScheduler() bool {
	return r == RoleAll || r == RoleScheduler
}
