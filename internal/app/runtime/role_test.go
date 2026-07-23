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
		role                       Role
		api, workers, scheduler    bool
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
