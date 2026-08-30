package qualitygate

import (
	"go/ast"
	"go/parser"
	"go/token"
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
      - name: Migration registry lifecycle gate
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

func TestValidateWorkflowRequiresControlled0165PathsInBothTriggers(t *testing.T) {
	workflow := readRepositoryFile(t, ".github/workflows/mysql57-amd64.yml")
	for _, required := range []string{
		"cmd/mochat-ai-insight-0165/**",
		"scripts/preflight_0165_ai_daily_insight_unification.go",
		"scripts/lib/migration_inventory_smoke.sh",
	} {
		if strings.Count(workflow, `- "`+required+`"`) != 2 {
			t.Fatalf("workflow must include %q in both push and pull_request paths", required)
		}
		mutated := strings.Replace(workflow, `      - "`+required+`"`+"\n", "", 1)
		path := writeWorkflow(t, mutated)
		assertFailureContains(t, validateWorkflow(path), "on.push.paths missing: "+required)
	}
}

func TestValidateWorkflowRejectsExcessivePermissions(t *testing.T) {
	workflow := readRepositoryFile(t, ".github/workflows/mysql57-amd64.yml")
	const safe = "permissions:\n  contents: read\n"
	if !strings.Contains(workflow, safe) {
		t.Fatal("workflow fixture no longer has the exact read-only permission baseline")
	}
	for _, test := range []struct {
		name        string
		replacement string
	}{
		{name: "missing", replacement: ""},
		{name: "write all", replacement: "permissions: write-all\n"},
		{name: "contents write", replacement: "permissions:\n  contents: write\n"},
		{name: "oidc write", replacement: "permissions:\n  contents: read\n  id-token: write\n"},
		{name: "attestations write", replacement: "permissions:\n  contents: read\n  attestations: write\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := writeWorkflow(t, strings.Replace(workflow, safe, test.replacement, 1))
			assertFailureContains(t, validateWorkflow(path), "workflow permissions must be exactly contents: read")
		})
	}

	t.Run("job override", func(t *testing.T) {
		const job = "  mysql57-amd64:\n"
		if !strings.Contains(workflow, job) {
			t.Fatal("workflow fixture no longer has the mysql57-amd64 job")
		}
		mutated := strings.Replace(workflow, job, job+"    permissions: write-all\n", 1)
		path := writeWorkflow(t, mutated)
		assertFailureContains(t, validateWorkflow(path), "workflow job mysql57-amd64 must not override permissions")
	})
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
			step:        "Migration registry lifecycle gate",
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
		{
			name: "errexit disabled with successful override",
			replacement: "        run: |\n" +
				"          set +e\n" +
				"          go test ./...\n" +
				"          exit 0\n",
		},
		{
			name: "semicolon errexit disable with successful override",
			replacement: "        run: |\n" +
				"          set +e;\n" +
				"          go test ./...\n" +
				"          true\n",
		},
		{
			name:        "semicolon exit before command",
			replacement: "        run: exit 0; go test ./...\n",
		},
		{
			name: "successful exit in compound before command",
			replacement: "        run: |\n" +
				"          if true; then\n" +
				"            exit 0\n" +
				"          fi\n" +
				"          go test ./...\n",
		},
		{
			name:        "exit before safe serial command",
			replacement: "        run: exit 0 && go test ./...\n",
		},
		{
			name:        "return before safe serial command",
			replacement: "        run: return 0 && go test ./...\n",
		},
		{
			name:        "exec before safe serial command",
			replacement: "        run: exec true && go test ./...\n",
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

func TestValidateWorkflowRejectsRequiredStepDisableControls(t *testing.T) {
	workflow := readRepositoryFile(t, ".github/workflows/mysql57-amd64.yml")
	const original = "      - name: Go tests\n" +
		"        run: go test ./...\n"
	if !strings.Contains(workflow, original) {
		t.Fatalf("workflow fixture no longer contains Go tests step")
	}

	for _, tc := range []struct {
		name        string
		replacement string
		failure     string
	}{
		{
			name: "literal false condition",
			replacement: "      - name: Go tests\n" +
				"        if: false\n" +
				"        run: go test ./...\n",
			failure: "Go tests step must be unconditional",
		},
		{
			name: "expression condition",
			replacement: "      - name: Go tests\n" +
				"        if: ${{ github.event_name == 'push' }}\n" +
				"        run: go test ./...\n",
			failure: "Go tests step must be unconditional",
		},
		{
			name: "quoted true condition",
			replacement: "      - name: Go tests\n" +
				"        if: \"true\"\n" +
				"        run: go test ./...\n",
			failure: "Go tests step must be unconditional",
		},
		{
			name: "continue on error",
			replacement: "      - name: Go tests\n" +
				"        continue-on-error: true\n" +
				"        run: go test ./...\n",
			failure: "Go tests step must not continue on error",
		},
		{
			name: "continue on error expression",
			replacement: "      - name: Go tests\n" +
				"        continue-on-error: ${{ matrix.allow_failure }}\n" +
				"        run: go test ./...\n",
			failure: "Go tests step must not continue on error",
		},
		{
			name: "quoted continue on error false",
			replacement: "      - name: Go tests\n" +
				"        continue-on-error: \"false\"\n" +
				"        run: go test ./...\n",
			failure: "Go tests step must not continue on error",
		},
		{
			name: "unsafe custom shell",
			replacement: "      - name: Go tests\n" +
				"        shell: bash {0}\n" +
				"        run: go test ./...\n",
			failure: "Go tests step shell must provide supported bash errexit semantics",
		},
		{
			name: "shell expression",
			replacement: "      - name: Go tests\n" +
				"        shell: ${{ matrix.shell }}\n" +
				"        run: go test ./...\n",
			failure: "Go tests step shell must provide supported bash errexit semantics",
		},
		{
			name: "non scalar shell",
			replacement: "      - name: Go tests\n" +
				"        shell: [bash]\n" +
				"        run: go test ./...\n",
			failure: "Go tests step shell must provide supported bash errexit semantics",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := writeWorkflow(t, strings.Replace(workflow, original, tc.replacement, 1))
			assertFailureContains(t, validateWorkflow(path), tc.failure)
		})
	}
}

func TestValidateWorkflowRejectsJobAndDefaultShellBypasses(t *testing.T) {
	workflow := readRepositoryFile(t, ".github/workflows/mysql57-amd64.yml")
	for _, tc := range []struct {
		name        string
		original    string
		replacement string
		failure     string
	}{
		{
			name:        "workflow defaults shell",
			original:    "jobs:",
			replacement: "defaults:\n  run:\n    shell: bash {0}\n\njobs:",
			failure:     "workflow defaults.run.shell must provide supported bash errexit semantics",
		},
		{
			name:     "job defaults shell",
			original: "    timeout-minutes: 45",
			replacement: "    timeout-minutes: 45\n" +
				"    defaults:\n" +
				"      run:\n" +
				"        shell: bash {0}\n",
			failure: "workflow job mysql57-amd64 defaults.run.shell must provide supported bash errexit semantics",
		},
		{
			name:     "job false condition",
			original: "    timeout-minutes: 45",
			replacement: "    timeout-minutes: 45\n" +
				"    if: false\n",
			failure: "workflow job mysql57-amd64 must be unconditional",
		},
		{
			name:     "job continue on error expression",
			original: "    timeout-minutes: 45",
			replacement: "    timeout-minutes: 45\n" +
				"    continue-on-error: ${{ matrix.allow_failure }}\n",
			failure: "workflow job mysql57-amd64 must not continue on error",
		},
		{
			name:        "unsupported runner",
			original:    "    runs-on: ubuntu-22.04",
			replacement: "    runs-on: windows-latest",
			failure:     "workflow job mysql57-amd64 must use supported runner ubuntu-22.04",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if !strings.Contains(workflow, tc.original) {
				t.Fatalf("workflow fixture no longer contains %q", tc.original)
			}
			path := writeWorkflow(t, strings.Replace(workflow, tc.original, tc.replacement, 1))
			assertFailureContains(t, validateWorkflow(path), tc.failure)
		})
	}
}

func TestValidateWorkflowRejectsInheritedExecutionContextBypasses(t *testing.T) {
	workflow := readRepositoryFile(t, ".github/workflows/mysql57-amd64.yml")
	for _, tc := range []struct {
		name    string
		mutate  func(*testing.T, string) string
		failure string
	}{
		{
			name: "workflow default working directory",
			mutate: func(t *testing.T, workflow string) string {
				return replaceWorkflowFixture(
					t,
					workflow,
					"jobs:",
					"defaults:\n  run:\n    working-directory: nested\n\njobs:",
				)
			},
			failure: "workflow defaults.run.working-directory must stay at repository root",
		},
		{
			name: "job default working directory",
			mutate: func(t *testing.T, workflow string) string {
				return replaceWorkflowFixture(
					t,
					workflow,
					"    timeout-minutes: 45",
					"    timeout-minutes: 45\n"+
						"    defaults:\n"+
						"      run:\n"+
						"        working-directory: nested\n",
				)
			},
			failure: "workflow job mysql57-amd64 defaults.run.working-directory must stay at repository root",
		},
		{
			name: "step working directory expression",
			mutate: func(t *testing.T, workflow string) string {
				return replaceWorkflowFixture(
					t,
					workflow,
					"      - name: Go tests\n        run: go test ./...\n",
					"      - name: Go tests\n"+
						"        working-directory: ${{ matrix.directory }}\n"+
						"        run: go test ./...\n",
				)
			},
			failure: "Go tests step working-directory must stay at repository root",
		},
		{
			name: "job container",
			mutate: func(t *testing.T, workflow string) string {
				return replaceWorkflowFixture(
					t,
					workflow,
					"    timeout-minutes: 45",
					"    timeout-minutes: 45\n"+
						"    container: ubuntu:24.04\n",
				)
			},
			failure: "workflow job mysql57-amd64 must not use a container",
		},
		{
			name: "skipped prerequisite",
			mutate: func(t *testing.T, workflow string) string {
				workflow = replaceWorkflowFixture(
					t,
					workflow,
					"jobs:",
					"jobs:\n"+
						"  skipped-prerequisite:\n"+
						"    runs-on: ubuntu-22.04\n"+
						"    if: false\n"+
						"    steps:\n"+
						"      - run: true\n",
				)
				return replaceWorkflowFixture(
					t,
					workflow,
					"    timeout-minutes: 45",
					"    timeout-minutes: 45\n"+
						"    needs: skipped-prerequisite\n",
				)
			},
			failure: "workflow job mysql57-amd64 must not depend on prerequisite jobs",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := writeWorkflow(t, tc.mutate(t, workflow))
			assertFailureContains(t, validateWorkflow(path), tc.failure)
		})
	}
}

func TestValidateWorkflowRejectsEnvironmentOverrides(t *testing.T) {
	workflow := readRepositoryFile(t, ".github/workflows/mysql57-amd64.yml")
	for _, tc := range []struct {
		name        string
		original    string
		replacement string
		failure     string
	}{
		{
			name:        "workflow GOFLAGS",
			original:    "jobs:",
			replacement: "env:\n  GOFLAGS: -run=^$\n\njobs:",
			failure:     "workflow must not set environment overrides",
		},
		{
			name:     "job PATH",
			original: "    timeout-minutes: 45",
			replacement: "    timeout-minutes: 45\n" +
				"    env:\n" +
				"      PATH: ./fake-bin\n",
			failure: "workflow job mysql57-amd64 must not set environment overrides",
		},
		{
			name: "Go tests GOFLAGS",
			original: "      - name: Go tests\n" +
				"        run: go test ./...\n",
			replacement: "      - name: Go tests\n" +
				"        env:\n" +
				"          GOFLAGS: -run=^$\n" +
				"        run: go test ./...\n",
			failure: "Go tests step environment must exactly match the required allowlist",
		},
		{
			name: "Go tests BASH_ENV expression",
			original: "      - name: Go tests\n" +
				"        run: go test ./...\n",
			replacement: "      - name: Go tests\n" +
				"        env:\n" +
				"          BASH_ENV: ${{ matrix.bootstrap }}\n" +
				"        run: go test ./...\n",
			failure: "Go tests step environment must exactly match the required allowlist",
		},
		{
			name: "lifecycle extra environment",
			original: "        env:\n" +
				"          MOCHAT_STACK_PROJECT: mochat-go-schema-migrate-ci\n" +
				"          MOCHAT_MYSQL_PORT: \"13331\"\n",
			replacement: "        env:\n" +
				"          MOCHAT_STACK_PROJECT: mochat-go-schema-migrate-ci\n" +
				"          MOCHAT_MYSQL_PORT: \"13331\"\n" +
				"          BASH_ENV: ./disable-errexit.sh\n",
			failure: "Migration registry lifecycle gate step environment must exactly match the required allowlist",
		},
		{
			name: "integration extra environment",
			original: "          MOCHAT_REQUIRE_MYSQL_INTEGRATION: \"1\"\n" +
				"        run: |\n",
			replacement: "          MOCHAT_REQUIRE_MYSQL_INTEGRATION: \"1\"\n" +
				"          GOFLAGS: -run=^$\n" +
				"        run: |\n",
			failure: "SCRM MySQL integration gate step environment must exactly match the required allowlist",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := writeWorkflow(
				t,
				replaceWorkflowFixture(t, workflow, tc.original, tc.replacement),
			)
			assertFailureContains(t, validateWorkflow(path), tc.failure)
		})
	}
}

func TestValidateWorkflowRejectsPersistentRunnerStateWrites(t *testing.T) {
	workflow := readRepositoryFile(t, ".github/workflows/mysql57-amd64.yml")
	const goTests = "      - name: Go tests\n" +
		"        run: go test ./...\n"

	for _, tc := range []struct {
		name      string
		stepName  string
		injection string
	}{
		{
			name:     "GOFLAGS through GITHUB_ENV",
			stepName: "Poison GOFLAGS",
			injection: "      - name: Poison GOFLAGS\n" +
				"        run: echo 'GOFLAGS=-run=^$' >> \"$GITHUB_ENV\"\n\n",
		},
		{
			name:     "fake go through braced GITHUB_PATH",
			stepName: "Poison Go PATH",
			injection: "      - name: Poison Go PATH\n" +
				"        run: |\n" +
				"          mkdir -p fake-bin\n" +
				"          printf '#!/bin/sh\\nexit 0\\n' > fake-bin/go\n" +
				"          chmod +x fake-bin/go\n" +
				"          echo \"$PWD/fake-bin\" >> \"${GITHUB_PATH}\"\n\n",
		},
		{
			name:     "BASH_ENV through GitHub expression",
			stepName: "Poison Bash startup",
			injection: "      - name: Poison Bash startup\n" +
				"        run: |\n" +
				"          printf 'go() { :; }\\n' > .fake-bash-env\n" +
				"          printf 'BASH_ENV=%s/.fake-bash-env\\n' \"$PWD\" >> \"${{ github.env }}\"\n\n",
		},
		{
			name:     "indirect GITHUB_ENV assignment",
			stepName: "Poison through env alias",
			injection: "      - name: Poison through env alias\n" +
				"        run: |\n" +
				"          runner_state=\"$GITHUB_ENV\"\n" +
				"          echo 'GOFLAGS=-run=^$' >> \"$runner_state\"\n\n",
		},
		{
			name:     "indirect GITHUB_PATH through tee",
			stepName: "Poison through path alias",
			injection: "      - name: Poison through path alias\n" +
				"        run: |\n" +
				"          path_state=\"${GITHUB_PATH}\"\n" +
				"          printf '%s\\n' \"$PWD/fake-bin\" | tee -a \"$path_state\"\n\n",
		},
		{
			name:     "constructed GITHUB_ENV name",
			stepName: "Poison through constructed name",
			injection: "      - name: Poison through constructed name\n" +
				"        run: |\n" +
				"          suffix=ENV\n" +
				"          declare -n runner_state=GITHUB_$suffix\n" +
				"          printf 'GOFLAGS=-run=^$\\n' >> \"$runner_state\"\n\n",
		},
		{
			name:     "append constructed GITHUB_ENV name",
			stepName: "Poison through appended name",
			injection: "      - name: Poison through appended name\n" +
				"        run: |\n" +
				"          state_name=GITHUB_\n" +
				"          state_name+=ENV\n" +
				"          declare -n runner_state=$state_name\n" +
				"          printf 'GOFLAGS=-run=^$\\n' >> \"$runner_state\"\n\n",
		},
		{
			name:     "nested shell constructed GITHUB_ENV name",
			stepName: "Poison through nested shell",
			injection: "      - name: Poison through nested shell\n" +
				"        run: |\n" +
				"          bash -c 'state_name=GITHUB_; state_name+=ENV; declare -n state=$state_name; printf \"GOFLAGS=-run=^$\\\\n\" >> \"$state\"'\n\n",
		},
		{
			name:     "bracket GitHub env expression",
			stepName: "Poison through bracket expression",
			injection: "      - name: Poison through bracket expression\n" +
				"        run: echo 'GOFLAGS=-run=^$' >> \"${{ github['env'] }}\"\n\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := writeWorkflow(
				t,
				replaceWorkflowFixture(t, workflow, goTests, tc.injection+goTests),
			)
			assertFailureContains(
				t,
				validateWorkflow(path),
				"workflow job mysql57-amd64 step "+tc.stepName+
					" must not write GitHub runner environment state",
			)
		})
	}
}

func TestValidateWorkflowRejectsPersistentRunnerStateContextBypasses(t *testing.T) {
	workflow := readRepositoryFile(t, ".github/workflows/mysql57-amd64.yml")
	const goTests = "      - name: Go tests\n" +
		"        run: go test ./...\n"

	for _, tc := range []struct {
		name      string
		injection string
		failure   string
	}{
		{
			name: "Python constructed state name",
			injection: "      - name: Python runner poison\n" +
				"        shell: python\n" +
				"        run: |\n" +
				"          import os\n" +
				"          key = \"GITHUB_\" + \"ENV\"\n" +
				"          with open(os.environ[key], \"a\") as state:\n" +
				"              state.write(\"GOFLAGS=-run=^$\\\\n\")\n\n",
			failure: "workflow job mysql57-amd64 step Python runner poison shell must provide supported bash errexit semantics",
		},
		{
			name: "PowerShell constructed state name",
			injection: "      - name: PowerShell runner poison\n" +
				"        shell: pwsh\n" +
				"        run: |\n" +
				"          $key = 'GITHUB_' + 'PATH'\n" +
				"          Add-Content -Path (Get-Item \"Env:$key\").Value -Value \"$PWD/fake-bin\"\n\n",
			failure: "workflow job mysql57-amd64 step PowerShell runner poison shell must provide supported bash errexit semantics",
		},
		{
			name: "state path through step env",
			injection: "      - name: Step env runner poison\n" +
				"        env:\n" +
				"          STATE_FILE: ${{ github.env }}\n" +
				"        run: echo 'GOFLAGS=-run=^$' >> \"$STATE_FILE\"\n\n",
			failure: "workflow job mysql57-amd64 step Step env runner poison must not set environment overrides",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := writeWorkflow(
				t,
				replaceWorkflowFixture(t, workflow, goTests, tc.injection+goTests),
			)
			assertFailureContains(t, validateWorkflow(path), tc.failure)
		})
	}
}

func TestValidateWorkflowRejectsUnapprovedRunSteps(t *testing.T) {
	workflow := readRepositoryFile(t, ".github/workflows/mysql57-amd64.yml")
	const goTests = "      - name: Go tests\n" +
		"        run: go test ./...\n"

	for _, tc := range []struct {
		name     string
		workflow string
		stepName string
	}{
		{
			name: "inserted harmless run step",
			workflow: replaceWorkflowFixture(
				t,
				workflow,
				goTests,
				"      - name: Unapproved helper\n"+
					"        run: echo harmless\n\n"+
					goTests,
			),
			stepName: "Unapproved helper",
		},
		{
			name: "timeout wrapped nested shell",
			workflow: replaceWorkflowFixture(
				t,
				workflow,
				goTests,
				"      - name: Wrapped runner poison\n"+
					"        run: timeout 10 bash -c 'n=GITHUB_; n+=ENV; declare -n f=$n; printf \"GOFLAGS=-run=^$\\\\n\" >> \"$f\"'\n\n"+
					goTests,
			),
			stepName: "Wrapped runner poison",
		},
		{
			name: "stdin fed nested shell",
			workflow: replaceWorkflowFixture(
				t,
				workflow,
				goTests,
				"      - name: Stdin runner poison\n"+
					"        run: bash <<< 'n=GITHUB_; n+=ENV; declare -n f=$n; printf \"GOFLAGS=-run=^$\\\\n\" >> \"$f\"'\n\n"+
					goTests,
			),
			stepName: "Stdin runner poison",
		},
		{
			name: "approved name with changed command",
			workflow: replaceWorkflowFixture(
				t,
				workflow,
				"      - name: Docker info\n"+
					"        run: docker info\n",
				"      - name: Docker info\n"+
					"        run: |\n"+
					"          docker info\n"+
					"          true\n",
			),
			stepName: "Docker info",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := writeWorkflow(t, tc.workflow)
			assertFailureContains(
				t,
				validateWorkflow(path),
				"workflow job mysql57-amd64 step "+tc.stepName+
					" must match its approved run fingerprint",
			)
		})
	}
}

func TestWritesGitHubRunnerEnvironmentState(t *testing.T) {
	for _, tc := range []struct {
		name   string
		script string
		want   bool
	}{
		{
			name:   "direct unquoted append",
			script: `echo "GOFLAGS=-run=^$" >>$GITHUB_ENV`,
			want:   true,
		},
		{
			name:   "parameter modifier overwrite",
			script: `echo "$PWD/fake-bin" >"${GITHUB_PATH:?missing}"`,
			want:   true,
		},
		{
			name: "command substitution alias",
			script: "state=\"$(printenv GITHUB_ENV)\"\n" +
				"printf '%s\\n' poison | command tee -a \"$state\"",
			want: true,
		},
		{
			name: "printf variable alias",
			script: "printf -v state '%s' \"$GITHUB_ENV\"\n" +
				"echo poison >> \"$state\"",
			want: true,
		},
		{
			name: "array alias",
			script: "state=(\"$GITHUB_PATH\")\n" +
				"echo \"$PWD/fake-bin\" >> \"${state[0]}\"",
			want: true,
		},
		{
			name:   "nested shell writer",
			script: `bash -c 'echo "BASH_ENV=/tmp/poison" >> "$GITHUB_ENV"'`,
			want:   true,
		},
		{
			name:   "GitHub context expression",
			script: `echo "GOFLAGS=-run=^$" >> "${{ github.env }}"`,
			want:   true,
		},
		{
			name:   "GitHub bracket context expression",
			script: `echo "GOFLAGS=-run=^$" >> "${{ github['env'] }}"`,
			want:   true,
		},
		{
			name:   "single quoted literal filename",
			script: `printf x > '$GITHUB_ENV'`,
			want:   false,
		},
		{
			name:   "harmless direct read",
			script: `printf '%s\n' "$GITHUB_ENV"`,
			want:   false,
		},
		{
			name:   "harmless read redirected elsewhere",
			script: `printf '%s\n' "$GITHUB_PATH" > debug.txt`,
			want:   false,
		},
		{
			name:   "alias without writer",
			script: `state="$GITHUB_ENV"`,
			want:   false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := writesGitHubRunnerEnvironmentState(tc.script); got != tc.want {
				t.Fatalf(
					"writesGitHubRunnerEnvironmentState(%q) = %t, want %t",
					tc.script,
					got,
					tc.want,
				)
			}
		})
	}
}

func TestValidateWorkflowAllowsExplicitSafeExecutionControls(t *testing.T) {
	workflow := readRepositoryFile(t, ".github/workflows/mysql57-amd64.yml")
	workflow = strings.Replace(
		workflow,
		"jobs:",
		"defaults:\n"+
			"  run:\n"+
			"    shell: bash\n"+
			"    working-directory: .\n\n"+
			"jobs:",
		1,
	)
	workflow = strings.Replace(
		workflow,
		"    timeout-minutes: 45",
		"    timeout-minutes: 45\n"+
			"    if: true\n"+
			"    continue-on-error: false\n"+
			"    defaults:\n"+
			"      run:\n"+
			"        shell: bash\n"+
			"        working-directory: .\n",
		1,
	)
	workflow = strings.Replace(
		workflow,
		"      - name: Go tests\n        run: go test ./...\n",
		"      - name: Go tests\n"+
			"        if: true\n"+
			"        continue-on-error: false\n"+
			"        shell: bash\n"+
			"        working-directory: .\n"+
			"        run: go test ./...\n",
		1,
	)

	path := writeWorkflow(t, workflow)
	if failures := validateWorkflow(path); len(failures) != 0 {
		t.Fatalf("explicit safe execution controls failed validation: %v", failures)
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
		"Migration registry lifecycle gate step does not own command: bash ./scripts/smoke_schema_migrate.sh",
	)
}

func TestValidateWorkflowRequiresTwentyMinuteFullFixtureBudget(t *testing.T) {
	workflow := readRepositoryFile(t, ".github/workflows/mysql57-amd64.yml")
	const required = "          go test ./internal/store ./internal/migration -count=1 -timeout 20m\n"
	if !strings.Contains(workflow, required) {
		t.Fatalf("workflow fixture no longer contains %q", required)
	}
	for _, replacement := range []string{
		"          go test ./internal/store ./internal/migration -count=1\n",
		"          go test ./internal/store ./internal/migration -count=1 -timeout 10m\n",
		"          go test ./internal/store -count=1 -timeout 20m\n",
	} {
		path := writeWorkflow(t, strings.Replace(workflow, required, replacement, 1))
		assertFailureContains(
			t,
			validateWorkflow(path),
			"integration step must run the complete store/migration fixture gate with a 20m timeout budget",
		)
	}
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

func TestValidateLifecycleAcceptsRepositoryScripts(t *testing.T) {
	lifecycle := readRepositoryFile(t, "scripts/smoke_schema_migrate.sh")
	inventoryLifecycle := readRepositoryFile(t, "scripts/lib/migration_inventory_smoke.sh")
	if _, err := parseShell(lifecycle); err != nil {
		t.Fatalf("parse authoritative lifecycle: %v", err)
	}
	if _, err := parseShell(inventoryLifecycle); err != nil {
		t.Fatalf("parse shared lifecycle: %v", err)
	}
	if failures := validateLifecycle(lifecycle, inventoryLifecycle); len(failures) != 0 {
		t.Fatalf("repository migration lifecycle failed validation: %v", failures)
	}
}

func TestValidateLifecycleRejectsCommentedCriticalCommand(t *testing.T) {
	lifecycle := readRepositoryFile(t, "scripts/smoke_schema_migrate.sh")
	inventoryLifecycle := readRepositoryFile(t, "scripts/lib/migration_inventory_smoke.sh")
	command := `"$MIGRATE_BIN" -project-root "$PWD" -action inventory >"$INVENTORY_FILE"`
	if !strings.Contains(inventoryLifecycle, command) {
		t.Fatalf("lifecycle fixture no longer contains %q", command)
	}

	failures := validateLifecycle(lifecycle, strings.Replace(inventoryLifecycle, command, "# "+command, 1))
	assertFailureContains(
		t,
		failures,
		"shared lifecycle must derive the migration registry from runtime inventory",
	)
}

func TestValidateLifecycleRejectsCleanupCommandInNestedFunction(t *testing.T) {
	lifecycle := readRepositoryFile(t, "scripts/smoke_schema_migrate.sh")
	inventoryLifecycle := readRepositoryFile(t, "scripts/lib/migration_inventory_smoke.sh")
	const original = "compose down --remove-orphans >/dev/null 2>&1 || true"
	const replacement = "never_called() {\n" +
		"      compose down --remove-orphans >/dev/null 2>&1 || true\n" +
		"    }"
	if !strings.Contains(lifecycle, original) {
		t.Fatalf("lifecycle fixture no longer contains cleanup command")
	}

	failures := validateLifecycle(strings.Replace(lifecycle, original, replacement, 1), inventoryLifecycle)
	assertFailureContains(
		t,
		failures,
		"authoritative lifecycle cleanup trap must be installed before startup without deleting volumes",
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
		"exec required":     {script: "exec " + expected, want: true},
		"safe serial and":   {script: "true && " + expected + " && true", want: true},
		"disabled errexit final command": {
			script: "set +e\n" + expected,
			want:   true,
		},
		"restored errexit": {
			script: "set +e\nset -e\n" + expected,
			want:   true,
		},
		"restored named errexit": {
			script: "set +o errexit\nset -o errexit\n" + expected,
			want:   true,
		},
		"terminator in uncalled function": {
			script: "dead_gate() {\n  exit 0\n}\n" + expected,
			want:   true,
		},
		"terminator in subshell": {
			script: "(exit 0)\n" + expected,
			want:   true,
		},
		"comment":        {script: "# " + expected, want: false},
		"echo":           {script: `echo "go test ./..."`, want: false},
		"printf":         {script: `printf '%s\n' 'go test ./...'`, want: false},
		"dead if branch": {script: "if false; then\n  " + expected + "\nfi", want: false},
		"dead case branch": {
			script: "case never in\n  match) " + expected + " ;;\nesac",
			want:   false,
		},
		"uncalled function": {script: "dead_gate() {\n  " + expected + "\n}", want: false},
		"or true":           {script: expected + " || true", want: false},
		"negated":           {script: "! " + expected, want: false},
		"forced exit zero":  {script: expected + "; exit 0", want: false},
		"disabled errexit successful override": {
			script: "set +e\n" + expected + "\nexit 0",
			want:   false,
		},
		"disabled errexit trailing success": {
			script: "set +e\n" + expected + "\ntrue",
			want:   false,
		},
		"semicolon disables errexit": {
			script: "set +e;\n" + expected + "\ntrue",
			want:   false,
		},
		"builtin disables errexit": {
			script: "builtin set +e\n" + expected + "\ntrue",
			want:   false,
		},
		"command disables named errexit": {
			script: "command set +o errexit\n" + expected + "\nexit 0",
			want:   false,
		},
		"exit before command":   {script: "exit 0 && " + expected, want: false},
		"return before command": {script: "return 0 && " + expected, want: false},
		"exec before command":   {script: "exec true && " + expected, want: false},
		"exit before following command": {
			script: "exit 0\n" + expected,
			want:   false,
		},
		"return before following command": {
			script: "return 0\n" + expected,
			want:   false,
		},
		"exec before following command": {
			script: "exec true\n" + expected,
			want:   false,
		},
		"semicolon exit before command": {
			script: "exit 0; " + expected,
			want:   false,
		},
		"successful exit in compound before command": {
			script: "if true; then\n  exit 0\nfi\n" + expected,
			want:   false,
		},
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

func TestWeWorkCallbackQueueGateAndSmokeUseDurableInboxContract(t *testing.T) {
	audit := readRepositoryFile(t, "scripts/audit_queue_annotation_coverage.sh")
	for _, required := range []string{"wework-callback", "durable-inbox", "WeWorkCallbackInbox", "LegacyWeWorkCallbackBacklog"} {
		if !strings.Contains(audit, required) {
			t.Fatalf("queue annotation audit missing durable callback token %q", required)
		}
	}
	if strings.Contains(audit, `"wework-callback", "WeWorkCallbackQueueDescriptor"`) {
		t.Fatal("queue annotation audit still requires deleted Redis callback descriptor")
	}

	smoke := readRepositoryFile(t, "scripts/smoke_wework_callback_worker.sh")
	if strings.Contains(smoke, "RPUSH mochat-go:wework-callback") {
		t.Fatal("callback smoke still injects new events through the legacy Redis queue")
	}
	for _, required := range []string{"mochat-callback-inbox-seed", "mochat_go_wework_callback_inbox", "status = 'pending'", "status = 'completed'", "lease_fence", "status = 'dead'"} {
		if !strings.Contains(smoke, required) {
			t.Fatalf("callback smoke missing durable inbox lifecycle token %q", required)
		}
	}
	standaloneAcceptance := readRepositoryFile(t, "scripts/standalone_acceptance.sh")
	if !strings.Contains(standaloneAcceptance, "MOCHAT_CALLBACK_EXTENDED_SIDE_EFFECT_SMOKE=1 ./scripts/smoke_wework_callback_worker.sh") {
		t.Fatal("standalone acceptance no longer preserves the full callback side-effect smoke")
	}
	assertDurableCallbackWorkerIsRedisOptional(t)
	cutover := readRepositoryFile(t, "cmd/mochat-callback-legacy-cutover/main.go")
	for _, required := range []string{"confirm-legacy-traffic-stopped", "MOCHAT_GO_WEWORK_CALLBACK_LEGACY_TRAFFIC_STOPPED", "sourceFingerprint", "ownerToken", "newCutoverOwnerToken", "BeginWeWorkCallbackLegacyCutover", "CompleteWeWorkCallbackLegacyCutover", "LegacyWeWorkCallbackCutoverName"} {
		if !strings.Contains(cutover, required) {
			t.Fatalf("controlled legacy cutover command missing %q", required)
		}
	}
	for _, forbidden := range []string{`flag.String("dsn"`, `flag.String("redis-password"`, `flag.String("redis-addr"`} {
		if strings.Contains(cutover, forbidden) {
			t.Fatalf("controlled legacy cutover exposes secret-bearing argv flag %q", forbidden)
		}
	}
}

func TestValidateDeveloperScriptsAcceptsCallbackCutoverBuildTarget(t *testing.T) {
	files := map[string]string{
		"scripts/dev_check.sh": readRepositoryFile(t, "scripts/dev_check.sh"),
		"scripts/test.sh":      readRepositoryFile(t, "scripts/test.sh"),
	}
	if failures := validateDeveloperScripts(files); len(failures) != 0 {
		t.Fatalf("developer scripts rejected after adding callback cutover build target: %v", failures)
	}
}

func assertDurableCallbackWorkerIsRedisOptional(t *testing.T) {
	t.Helper()
	mainPath := filepath.Join("..", "..", "cmd", "mochat-go", "main.go")
	file, err := parser.ParseFile(token.NewFileSet(), mainPath, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var callbackBlock *ast.BlockStmt
	ast.Inspect(file, func(node ast.Node) bool {
		if statement, ok := node.(*ast.IfStmt); ok {
			selector, ok := statement.Cond.(*ast.SelectorExpr)
			if ok && selector.Sel.Name == "EnableWeWorkCallbackWorker" {
				callbackBlock = statement.Body
			}
		}
		return true
	})
	if callbackBlock == nil {
		t.Fatal("durable callback worker startup block not found")
	}
	lazyResolverInjected := false
	staticEmptyCapabilities := false
	contactWelcomeConstructed := false
	contactWelcomeRegistered := false
	ast.Inspect(callbackBlock, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		if name, ok := call.Fun.(*ast.Ident); ok {
			switch name.Name {
			case "getRedisStore", "ImportLegacyWeWorkCallbackBacklog", "executeCutover":
				t.Fatalf("ordinary durable callback startup calls forbidden dependency %s", name.Name)
			case "optionalWeWorkCallbackCapabilities":
				t.Fatal("callback worker composition must not use a one-shot Redis startup probe")
			}
		}
		if selector, ok := call.Fun.(*ast.SelectorExpr); ok {
			switch selector.Sel.Name {
			case "WithCapabilityResolver":
				lazyResolverInjected = true
			case "NewWeWorkCallbackWorker":
				if len(call.Args) > 0 {
					if literal, ok := call.Args[0].(*ast.CompositeLit); ok && len(literal.Elts) == 0 {
						staticEmptyCapabilities = true
					}
				}
			case "NewContactWelcomeWorker":
				contactWelcomeConstructed = true
			case "Add":
				if len(call.Args) > 0 {
					if taskName, ok := call.Args[0].(*ast.BasicLit); ok && taskName.Value == `"contact-welcome"` {
						contactWelcomeRegistered = true
					}
				}
			}
		}
		return true
	})
	if !lazyResolverInjected || !staticEmptyCapabilities {
		t.Fatal("durable callback producer does not use an empty static capability set plus lazy Redis resolver")
	}
	if !contactWelcomeConstructed || !contactWelcomeRegistered {
		t.Fatal("contact welcome consumer is not always registered with the durable callback worker")
	}
	for _, statement := range callbackBlock.List {
		conditional, ok := statement.(*ast.IfStmt)
		if !ok {
			continue
		}
		conditionalWelcome := false
		ast.Inspect(conditional.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			if selector, ok := call.Fun.(*ast.SelectorExpr); ok && selector.Sel.Name == "NewContactWelcomeWorker" {
				conditionalWelcome = true
			}
			return true
		})
		if conditionalWelcome {
			t.Fatal("contact welcome consumer registration is still conditional on a startup probe")
		}
	}

	configPath := filepath.Join("..", "..", "internal", "config", "config.go")
	configFile, err := parser.ParseFile(token.NewFileSet(), configPath, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	redisGateFound := false
	ast.Inspect(configFile, func(node ast.Node) bool {
		assign, ok := node.(*ast.AssignStmt)
		if !ok || len(assign.Rhs) != 1 {
			return true
		}
		for _, lhs := range assign.Lhs {
			name, named := lhs.(*ast.Ident)
			if !named || name.Name != "redisWorkerEnabled" {
				continue
			}
			redisGateFound = true
			ast.Inspect(assign.Rhs[0], func(child ast.Node) bool {
				if selector, ok := child.(*ast.SelectorExpr); ok && selector.Sel.Name == "EnableWeWorkCallbackWorker" {
					t.Fatal("durable callback worker is still part of the Redis-required config gate")
				}
				return true
			})
		}
		return true
	})
	if !redisGateFound {
		t.Fatal("Redis-required worker config gate was not found")
	}
}

func readRepositoryFile(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(path)))
	if err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(string(contents), "\r\n", "\n")
}

func writeWorkflow(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "workflow.yml")
	if err := os.WriteFile(path, []byte(strings.TrimSpace(contents)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func replaceWorkflowFixture(t *testing.T, workflow, original, replacement string) string {
	t.Helper()
	if !strings.Contains(workflow, original) {
		t.Fatalf("workflow fixture no longer contains %q", original)
	}
	return strings.Replace(workflow, original, replacement, 1)
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
