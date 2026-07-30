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

func TestValidateWorkflowRejectsUnsafeRequiredCommandControlFlow(t *testing.T) {
	workflow := readRepositoryFile(t, ".github/workflows/mysql57-amd64.yml")
	const original = "        run: go test ./...\n"
	if !strings.Contains(workflow, original) {
		t.Fatalf("workflow fixture no longer contains %q", original)
	}

	for _, tc := range []struct {
		name        string
		replacement string
	}{
		{
			name: "dead if branch",
			replacement: "        run: |\n" +
				"          if false; then\n" +
				"            go test ./...\n" +
				"          fi\n",
		},
		{
			name: "dead case branch",
			replacement: "        run: |\n" +
				"          case never in\n" +
				"            match) go test ./... ;;\n" +
				"          esac\n",
		},
		{
			name: "uncalled function",
			replacement: "        run: |\n" +
				"          dead_gate() {\n" +
				"            go test ./...\n" +
				"          }\n",
		},
		{
			name:        "or true",
			replacement: "        run: go test ./... || true\n",
		},
		{
			name: "negated command",
			replacement: "        run: |\n" +
				"          ! go test ./...\n",
		},
		{
			name: "forced successful exit",
			replacement: "        run: |\n" +
				"          go test ./...; exit 0\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := writeWorkflow(t, strings.Replace(workflow, original, tc.replacement, 1))
			assertFailureContains(
				t,
				validateWorkflow(path),
				"Go tests step does not own command: go test ./...",
			)
		})
	}
}

func TestValidateWorkflowRejectsLifecycleCommandInDeadBranch(t *testing.T) {
	workflow := readRepositoryFile(t, ".github/workflows/mysql57-amd64.yml")
	const original = "        run: bash ./scripts/smoke_schema_migrate.sh\n"
	const replacement = "        run: |\n" +
		"          if false; then\n" +
		"            bash ./scripts/smoke_schema_migrate.sh\n" +
		"          fi\n"
	if !strings.Contains(workflow, original) {
		t.Fatalf("workflow fixture no longer contains %q", original)
	}

	path := writeWorkflow(t, strings.Replace(workflow, original, replacement, 1))
	assertFailureContains(
		t,
		validateWorkflow(path),
		"Migration 0098 lifecycle gate step does not own command: bash ./scripts/smoke_schema_migrate.sh",
	)
}

func TestValidateWorkflowAllowsEnvironmentPrefixForRequiredCommand(t *testing.T) {
	workflow := readRepositoryFile(t, ".github/workflows/mysql57-amd64.yml")
	const original = "        run: go test ./...\n"
	const replacement = "        run: env CGO_ENABLED=1 go test ./...\n"
	if !strings.Contains(workflow, original) {
		t.Fatalf("workflow fixture no longer contains %q", original)
	}

	path := writeWorkflow(t, strings.Replace(workflow, original, replacement, 1))
	if failures := validateWorkflow(path); len(failures) != 0 {
		t.Fatalf("environment-prefixed real workflow failed validation: %v", failures)
	}
}

func TestValidateWorkflowRejectsCleanupCommandInNestedFunction(t *testing.T) {
	workflow := readRepositoryFile(t, ".github/workflows/mysql57-amd64.yml")
	const original = "          cleanup() {\n" +
		"            docker compose -p \"$MOCHAT_STACK_PROJECT\" -f deploy/mysql57/docker-compose.yml down -v --remove-orphans\n" +
		"          }\n"
	const replacement = "          cleanup() {\n" +
		"            never_called() {\n" +
		"              docker compose -p \"$MOCHAT_STACK_PROJECT\" -f deploy/mysql57/docker-compose.yml down -v --remove-orphans\n" +
		"            }\n" +
		"          }\n"
	if !strings.Contains(workflow, original) {
		t.Fatalf("workflow fixture no longer contains integration cleanup function")
	}

	path := writeWorkflow(t, strings.Replace(workflow, original, replacement, 1))
	assertFailureContains(
		t,
		validateWorkflow(path),
		"integration cleanup/down/trap must be defined before database startup",
	)
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

func TestValidateLifecycleRejectsCleanupCommandInNestedFunction(t *testing.T) {
	lifecycle := readRepositoryFile(t, "scripts/smoke_schema_migrate.sh")
	const original = "compose down -v --remove-orphans >/dev/null 2>&1 || true"
	const replacement = "never_called() {\n" +
		"      compose down -v --remove-orphans >/dev/null 2>&1 || true\n" +
		"    }"
	if !strings.Contains(lifecycle, original) {
		t.Fatalf("lifecycle fixture no longer contains cleanup command")
	}

	failures := validateLifecycle(strings.Replace(lifecycle, original, replacement, 1))
	assertFailureContains(
		t,
		failures,
		"authoritative lifecycle cleanup trap must be installed before startup",
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
		"safe serial and":   {script: "true && " + expected + " && true", want: true},
		"comment":           {script: "# " + expected, want: false},
		"echo":              {script: `echo "go test ./..."`, want: false},
		"printf":            {script: `printf '%s\n' 'go test ./...'`, want: false},
		"dead if branch":    {script: "if false; then\n  " + expected + "\nfi", want: false},
		"dead case branch": {
			script: "case never in\n  match) " + expected + " ;;\nesac",
			want:   false,
		},
		"uncalled function": {script: "dead_gate() {\n  " + expected + "\n}", want: false},
		"or true":           {script: expected + " || true", want: false},
		"negated":           {script: "! " + expected, want: false},
		"forced exit zero":  {script: expected + "; exit 0", want: false},
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

func TestShellFunctionExecutesRejectsDeadNestedPaths(t *testing.T) {
	const expected = "docker compose down"
	for _, tc := range []struct {
		name   string
		script string
		want   bool
	}{
		{
			name:   "direct cleanup",
			script: "cleanup() {\n  " + expected + "\n}",
			want:   true,
		},
		{
			name: "dynamic cleanup guard",
			script: "cleanup() {\n" +
				"  if [ \"${KEEP_STACK:-0}\" != \"1\" ]; then\n" +
				"    " + expected + " || true\n" +
				"  fi\n" +
				"}",
			want: true,
		},
		{
			name: "nested function",
			script: "cleanup() {\n" +
				"  never_called() {\n" +
				"    " + expected + "\n" +
				"  }\n" +
				"}",
			want: false,
		},
		{
			name:   "dead if branch",
			script: "cleanup() {\n  if false; then\n    " + expected + "\n  fi\n}",
			want:   false,
		},
		{
			name:   "dead case branch",
			script: "cleanup() {\n  case never in\n    match) " + expected + " ;;\n  esac\n}",
			want:   false,
		},
		{
			name:   "masked right side",
			script: "cleanup() {\n  true || " + expected + "\n}",
			want:   false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := shellFunctionExecutes(tc.script, "cleanup", expected); got != tc.want {
				t.Fatalf("shellFunctionExecutes(%q) = %t, want %t", tc.script, got, tc.want)
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
