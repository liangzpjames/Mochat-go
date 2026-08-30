package runtime

import "testing"

func TestParseRole(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want Role
	}{
		{name: "empty defaults to all", raw: "", want: RoleAll},
		{name: "all", raw: "all", want: RoleAll},
		{name: "api", raw: "api", want: RoleAPI},
		{name: "worker", raw: "worker", want: RoleWorker},
		{name: "scheduler", raw: "scheduler", want: RoleScheduler},
		{name: "trims and normalizes", raw: " API ", want: RoleAPI},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseRole(tt.raw)
			if err != nil {
				t.Fatalf("ParseRole(%q): %v", tt.raw, err)
			}
			if got != tt.want {
				t.Fatalf("ParseRole(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestParseRoleRejectsUnknownRole(t *testing.T) {
	if _, err := ParseRole("background"); err == nil {
		t.Fatal("ParseRole(background) succeeded, want error")
	}
}

func TestRoleCapabilities(t *testing.T) {
	tests := []struct {
		role                    Role
		api, workers, scheduler bool
	}{
		{role: RoleAll, api: true, workers: true, scheduler: true},
		{role: RoleAPI, api: true},
		{role: RoleWorker, workers: true},
		{role: RoleScheduler, scheduler: true},
	}

	for _, tt := range tests {
		t.Run(string(tt.role), func(t *testing.T) {
			if got := tt.role.RunsAPI(); got != tt.api {
				t.Errorf("RunsAPI() = %v, want %v", got, tt.api)
			}
			if got := tt.role.RunsWorkers(); got != tt.workers {
				t.Errorf("RunsWorkers() = %v, want %v", got, tt.workers)
			}
			if got := tt.role.RunsScheduler(); got != tt.scheduler {
				t.Errorf("RunsScheduler() = %v, want %v", got, tt.scheduler)
			}
		})
	}
}

func TestRoleResponsibilitiesMatrix(t *testing.T) {
	tests := []struct {
		role               Role
		durable, automatic bool
		want               Responsibilities
	}{
		{RoleAll, true, true, Responsibilities{API: true, Workers: true, Schedulers: true, DurableArchiveAPI: true, DurableArchiveWorkers: true, ArchiveEnqueuer: true}},
		{RoleAPI, true, true, Responsibilities{API: true, DurableArchiveAPI: true}},
		{RoleWorker, true, true, Responsibilities{Workers: true, DurableArchiveWorkers: true}},
		{RoleScheduler, true, true, Responsibilities{Schedulers: true, ArchiveEnqueuer: true}},
		{RoleAll, false, true, Responsibilities{API: true, Workers: true, Schedulers: true}},
		{RoleAll, true, false, Responsibilities{API: true, Workers: true, Schedulers: true, DurableArchiveAPI: true, DurableArchiveWorkers: true}},
	}
	for _, tt := range tests {
		if got := tt.role.Responsibilities(tt.durable, tt.automatic); got != tt.want {
			t.Errorf("role=%s durable=%t automatic=%t got=%+v want=%+v", tt.role, tt.durable, tt.automatic, got, tt.want)
		}
	}
}
