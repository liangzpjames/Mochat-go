package qualitygate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateWorkflowRejectsRequiredGatesSplitAcrossJobs(t *testing.T) {
	path := writeWorkflow(t, `
on:
  push:
    paths: []
  pull_request:
    paths: []
jobs:
  mysql57-amd64:
    steps:
      - name: Go architecture gate
        run: go run ./cmd/mochat-architecture -root .
      - name: Go module race gate
        run: go test -race ./internal/modules/...
      - name: Go tests
        run: go test ./...
      - name: Go vet
        run: go vet ./...
      - name: Migration 0098 lifecycle gate
        run: bash ./scripts/smoke_schema_migrate.sh
  detached-integration:
    steps:
      - name: SCRM MySQL integration gate
        run: go test -v -count=1 -tags=integration ./internal/modules/scrm/adapters/mysql
`)

	assertFailureContains(
		t,
		validateWorkflow(path),
		"workflow job mysql57-amd64 missing named step: SCRM MySQL integration gate",
	)
}

func TestValidateWorkflowRejectsDuplicateStepNames(t *testing.T) {
	path := writeWorkflow(t, `
on:
  push:
    paths: []
  pull_request:
    paths: []
jobs:
  mysql57-amd64:
    steps:
      - name: Go tests
        run: go test ./...
      - name: Go tests
        run: echo bypass
`)

	assertFailureContains(
		t,
		validateWorkflow(path),
		`workflow job mysql57-amd64 has duplicate step name "Go tests"`,
	)
}

func TestValidateWorkflowRequiresPhase3PlanInBothTriggers(t *testing.T) {
	path := writeWorkflow(t, `
on:
  push:
    paths:
      - docs/superpowers/plans/2026-07-29-phase3-yuanhu-business-foundation.md
  pull_request:
    paths: []
jobs:
  mysql57-amd64:
    steps: []
`)

	failures := validateWorkflow(path)
	assertFailureContains(
		t,
		failures,
		"on.pull_request.paths missing: docs/superpowers/plans/2026-07-29-phase3-yuanhu-business-foundation.md",
	)
	if failureContains(failures, "on.push.paths missing: docs/superpowers/plans/2026-07-29-phase3-yuanhu-business-foundation.md") {
		t.Fatalf("push trigger incorrectly rejected the Phase 3 plan path: %v", failures)
	}
}

func TestValidateWorkflowRejectsRequiredCommandAsInertText(t *testing.T) {
	workflow := readRepositoryFile(t, ".github/workflows/mysql57-amd64.yml")
	cases := []struct {
		name        string
		original    string
		replacement string
		step        string
		command     string
	}{
		{
			name:        "echoed full test",
			original:    "        run: go test ./...\n",
			replacement: "        run: echo \"go test ./...\"\n",
			step:        "Go tests",
			command:     "go test ./...",
		},
		{
			name:        "commented lifecycle",
			original:    "        run: bash ./scripts/smoke_schema_migrate.sh\n",
			replacement: "        run: |\n          # bash ./scripts/smoke_schema_migrate.sh\n",
			step:        "Migration 0098 lifecycle gate",
			command:     "bash ./scripts/smoke_schema_migrate.sh",
		},
		{
			name:     "integration command in heredoc data",
			original: "          go test -v -count=1 -tags=integration ./internal/modules/scrm/adapters/mysql\n",
			replacement: "          cat <<'INERT_COMMAND'\n" +
				"          go test -v -count=1 -tags=integration ./internal/modules/scrm/adapters/mysql\n" +
				"          INERT_COMMAND\n",
			step:    "SCRM MySQL integration gate",
			command: "go test -v -count=1 -tags=integration ./internal/modules/scrm/adapters/mysql",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !strings.Contains(workflow, tc.original) {
				t.Fatalf("workflow fixture no longer contains %q", tc.original)
			}
			path := writeWorkflow(t, strings.Replace(workflow, tc.original, tc.replacement, 1))
			assertFailureContains(
				t,
				validateWorkflow(path),
				tc.step+" step does not own command: "+tc.command,
			)
		})
	}
}

func TestValidateLifecycleRejectsCommentedCriticalCommand(t *testing.T) {
	lifecycle := readRepositoryFile(t, "scripts/smoke_schema_migrate.sh")
	command := `"$MIGRATE_BIN" -dsn "$MIGRATE_DSN" -project-root "$PWD" -action apply >"$WORK_DIR/apply.out"`
	if !strings.Contains(lifecycle, command) {
		t.Fatalf("lifecycle fixture no longer contains %q", command)
	}

	failures := validateLifecycle(strings.Replace(lifecycle, command, "# "+command, 1))
	assertFailureContains(
		t,
		failures,
		"authoritative lifecycle script must execute 0098 apply/checksum/rollback/replay in order",
	)
}

func TestExecutableCommandRecognitionAllowsOnlyRealCommands(t *testing.T) {
	const expected = "go test ./..."
	for name, tc := range map[string]struct {
		script string
		want   bool
	}{
		"exact":             {script: expected, want: true},
		"assignment prefix": {script: "CGO_ENABLED=1 " + expected, want: true},
		"env prefix":        {script: "env CGO_ENABLED=1 " + expected, want: true},
		"comment":           {script: "# " + expected, want: false},
		"echo":              {script: `echo "go test ./..."`, want: false},
		"printf":            {script: `printf '%s\n' 'go test ./...'`, want: false},
		"heredoc": {
			script: "cat <<'INERT'\n" + expected + "\nINERT\n",
			want:   false,
		},
	} {
		t.Run(name, func(t *testing.T) {
			if got := containsExecutableCommandsInOrder(tc.script, []string{expected}); got != tc.want {
				t.Fatalf("containsExecutableCommandsInOrder(%q) = %t, want %t", tc.script, got, tc.want)
			}
		})
	}
}

func readRepositoryFile(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(path)))
	if err != nil {
		t.Fatal(err)
	}
	return string(contents)
}

func writeWorkflow(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "workflow.yml")
	if err := os.WriteFile(path, []byte(strings.TrimSpace(contents)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func assertFailureContains(t *testing.T, failures []string, want string) {
	t.Helper()
	if !failureContains(failures, want) {
		t.Fatalf("failures = %v, want %q", failures, want)
	}
}

func failureContains(failures []string, want string) bool {
	for _, failure := range failures {
		if failure == want {
			return true
		}
	}
	return false
}
