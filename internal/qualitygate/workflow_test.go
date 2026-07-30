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
