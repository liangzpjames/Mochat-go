# Task 7 report: mandatory local and CI quality gates

## Status

Implemented and verified.

## Changes

- Replaced `scripts/audit_architecture_boundaries.sh` with a compatibility wrapper around `go run ./cmd/mochat-architecture -root .`.
- Reworked the wrapper self-test to prove:
  - a temporary invalid module tree exits non-zero with the stable `ARCH-DOMAIN-DEPENDENCY` rule ID;
  - the real repository produces `architecture boundaries passed`;
  - the compatibility script delegates to the Go CLI with the fixed arguments.
- Added the mandatory architecture, module, race, full test, and vet sequence to `scripts/dev_check.sh quick` and `scripts/test.sh` without removing existing audits, frontend checks, or builds.
- Changed the default developer Go image from Alpine to Bookworm because the race detector requires CGO and a C compiler.
- Added named CI architecture and race gates.
- Runs the SCRM integration gate in a dedicated MySQL 5.7 Compose project, explicitly verifies migration `0098_scrm_lead_foundation`, and cleans the project with a shell `EXIT` trap.
- Set the integration DSN to `loc=UTC`; `loc=Local` overflows the protected MySQL maximum-timestamp test in UTC+8 by converting year 9999 to year 10000.
- Added the backend declaration and verification PR template.

## TDD evidence

- RED: the new wrapper test exited 1 against the old shell implementation with `architecture audit did not delegate to the Go CLI`.
- GREEN: the same Linux container test printed `architecture boundary wrapper self-test passed`.
- RED: the gate contract check reported both local scripts missing the ordered core commands, all three named CI gates missing, migration 0098 unverified, and the PR template missing.
- GREEN: the same contract check printed `Task 7 gate contract passed`.
- RED: `golang:1.26-alpine` failed `go test -race ./internal/modules/...` with `-race requires cgo`.
- GREEN: `golang:1.26-bookworm` passed the module race suite.
- RED: the real migrated MySQL integration run failed at the maximum timestamp with year 10000 when the DSN used `loc=Local`.
- GREEN: the same migrated MySQL 5.7 integration suite passed with `loc=UTC`.

## Verification

- Linux Docker shell parsing for the modified shell entry points.
- Windows and Linux `go run ./cmd/mochat-architecture -root .`.
- Linux Docker:
  - `go test ./internal/app/modules/... ./internal/modules/...`
  - `go test -race ./internal/modules/...`
  - `go test ./...`
  - `go vet ./...`
  - wrapper self-test
  - command builds
- MySQL 5.7 Compose database:
  - migration apply completed;
  - migration 0098 row count was exactly 1;
  - an uncached `go test -count=1 -tags=integration ./internal/modules/scrm/adapters/mysql` passed.
- Workflow YAML parsed successfully and the enforced order is architecture, race, MySQL migration smoke, then the atomic SCRM database/migration/integration lifecycle.
- Local quick paths contain no tagged integration command.

## Notes

- The first clean Linux full-test run was interrupted by a `proxy.golang.org` 403 for `github.com/klauspost/compress@v1.18.6`. Re-running with `GOPROXY=https://goproxy.cn,direct` fetched the dependency and the full suite passed; no repository proxy setting was changed.

## Review follow-up

- Added `scripts/test_backend_quality_gate_contract.sh` and wired it into developer quick checks, `scripts/test.sh`, and CI.
- RED evidence against commit `10ef337` covered:
  - zero `migrations/**` path filters instead of one under both `push` and `pull_request`;
  - retained `KEEP_STACK`;
  - missing failure-safe integration cleanup and dedicated database startup;
  - missing explicit migration apply;
  - missing uncached integration command;
  - missing local and CI contract-test hooks.
- Removed `KEEP_STACK=1` from the existing MySQL smoke gate, restoring its own cleanup trap on success and failure.
- Added `migrations/**` and the contract-test script to both workflow path filters.
- Replaced the cross-step retained database with one atomic SCRM integration step:
  - registers cleanup before starting Compose;
  - uses a dedicated project and port;
  - waits for the MySQL health check;
  - applies all migrations and asserts 0098;
  - runs `go test -count=1 -tags=integration ./internal/modules/scrm/adapters/mysql`;
  - removes the container, volume, and network on every exit path.
- Runtime verification executed the exact YAML `run` block twice against fresh MySQL 5.7 databases. Both attempts passed, and each left zero Compose containers, proving cleanup and retry behavior.
