// Package runtime defines which process responsibilities a MoChat Go
// deployment runs. It deliberately contains no business or infrastructure
// dependencies so every entrypoint can share the same role semantics.
package runtime

import (
	"fmt"
	"strings"
)

type Role string

// Responsibilities is the runtime ownership matrix. Archive switches remain
// user configuration; the role selects which process is allowed to act on it.
type Responsibilities struct {
	API                   bool
	Workers               bool
	Schedulers            bool
	DurableArchiveAPI     bool
	DurableArchiveWorkers bool
	ArchiveEnqueuer       bool
}

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

func (r Role) Responsibilities(durableArchive, automaticArchiveScheduling bool) Responsibilities {
	if r == "" {
		r = RoleAll
	}
	return Responsibilities{
		API:                   r.RunsAPI(),
		Workers:               r.RunsWorkers(),
		Schedulers:            r.RunsScheduler(),
		DurableArchiveAPI:     r.RunsAPI() && durableArchive,
		DurableArchiveWorkers: r.RunsWorkers() && durableArchive,
		ArchiveEnqueuer:       r.RunsScheduler() && durableArchive && automaticArchiveScheduling,
	}
}
