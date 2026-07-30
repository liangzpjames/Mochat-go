# Phase 2.2 Final Gate Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:test-driven-development and execute each task as a RED-GREEN-REFACTOR cycle.

**Goal:** Close every final-review finding without weakening the Phase 2.2 architecture, CI, migration, integration, or compatibility gates.

**Architecture:** Replace layer deny rules with fail-closed import allowlists and bidirectional repository-policy reconciliation. Treat CI as a parsed workflow contract with ordered, step-local assertions. Centralize safe typed-nil detection for the router/server dispatch boundary, then refresh release evidence only from the final source tree.

**Tech Stack:** Go 1.26 AST tests, POSIX shell, Python 3 with PyYAML in the contract test, GitHub Actions YAML, Docker Compose, MySQL 5.7/MariaDB lifecycle smoke.

## Global Constraints

- Start from clean `45cc401` in the supplied linked worktree.
- No production-code change may precede its failing regression test.
- Preserve SCRM dependency direction and every existing quality gate.
- `ready: yes` is allowed only after every documented command succeeds on the final source commit/tree.

---

### Task 1: Fail-closed architecture policy

**Files:**
- Modify: `internal/architecture/rules.go`
- Modify: `internal/architecture/audit.go`
- Modify: `internal/architecture/audit_test.go`
- Create: negative fixtures beneath `internal/architecture/testdata/`
- Modify: `architecture-policy.json`
- Modify: `internal/architecture/testdata/policy.json`

**Interfaces:**
- `Audit(root string, policy Policy, now time.Time) ([]Violation, error)` remains stable.
- `Policy` gains an explicit non-production/example module registry so every directory under `internal/modules` is classified exactly once.
- Exact `Exception` entries remain the only way to waive an import finding for a public cross-module contract.

- [ ] Write tests proving rejection of domain-to-own-ports, domain/application-to-internal infrastructure, application-to-other-module-domain/ports, and domain-to-third-party imports.
- [ ] Write tests proving missing protected files, unregistered module directories, and declared-but-missing modules are stable violations; add a positive exact-exception test.
- [ ] Run `go test ./internal/architecture/... -count=1` and record expected failures against `45cc401`.
- [ ] Implement explicit per-layer allowlists: domain, ports, and application accept only permitted standard-library packages and own inward layers; adapters, transport, and module roots accept only their designed inward layers/shared contracts.
- [ ] Reconcile filesystem modules and policy registrations bidirectionally, while explicitly registering `example` as non-production.
- [ ] Run `go test ./internal/architecture/... -count=1` and `go run ./cmd/mochat-architecture -root .`.
- [ ] Commit the green architecture slice.

### Task 2: Structured backend workflow contract and Phase 3 handoff

**Files:**
- Modify: `scripts/test_backend_quality_gate_contract.sh`
- Modify: `.github/workflows/mysql57-amd64.yml`
- Modify: `docs/superpowers/plans/2026-07-29-phase3-yuanhu-business-foundation.md`
- Modify: `scripts/dev_check.sh`

**Interfaces:**
- The shell entry point remains `sh ./scripts/test_backend_quality_gate_contract.sh`.
- Python parses YAML with a loader that preserves the YAML 1.1 `on` key as text, then validates triggers and named step-local `run` blocks.

- [ ] Rewrite the contract test first to require push and PR path coverage for `cmd/mochat-go/**`, `scripts/dev_check.sh`, `scripts/smoke_schema_migrate.sh`, and all gate files.
- [ ] Require ordered workflow steps: architecture, race, full test, vet, authoritative lifecycle, then strict uncached integration.
- [ ] Require the integration step to define cleanup/trap before startup and to run `down -v --remove-orphans`; require lifecycle cleanup through the authoritative smoke script.
- [ ] Require Phase 3 Task 2 to create `customer_lifecycle_repository_integration_test.go` with an integration build tag and real tenant-isolation, optimistic-lock, and concurrent-claim scenarios.
- [ ] Require Phase 3 Task 7 to run `go test ./...`, `go vet ./...`, architecture/race/lifecycle/strict integration, and all six Go command builds.
- [ ] Run `sh ./scripts/test_backend_quality_gate_contract.sh` and record expected failure on the old workflow/plan.
- [ ] Update the workflow, developer gate, and Phase 3 plan minimally to satisfy the contract.
- [ ] Re-run the contract and parse the workflow independently.
- [ ] Commit the green CI/plan slice.

### Task 3: Typed-nil-safe HTTP boundaries

**Files:**
- Create: `internal/nilcheck/nilcheck.go`
- Create: `internal/nilcheck/nilcheck_test.go`
- Modify: `internal/app/modules/router.go`
- Modify: `internal/app/modules/router_test.go`
- Modify: `internal/server/server.go`
- Modify: `internal/server/server_test.go`

**Interfaces:**
- `nilcheck.IsNil(value any) bool` returns true for nil interfaces and nil-capable typed-nil dynamic values, and false for non-nil or non-nil-capable values without calling reflection operations on unsupported kinds.
- `Router.Handle` continues returning an error for invalid handlers.
- `WithModuleRouter` remains an `Option`; typed-nil routers and typed-nil matched handlers are ignored safely so legacy dispatch continues.

- [ ] Add typed-nil handler/router regression tests before implementation.
- [ ] Run focused router/server tests and record panic/acceptance failures.
- [ ] Add `nilcheck.IsNil`, reject typed-nil registrations, and guard router dispatch and matched handlers.
- [ ] Run `go test ./internal/nilcheck ./internal/app/modules ./internal/server -count=1`.
- [ ] Commit the green runtime-safety slice.

### Task 4: Final verification and evidence

**Files:**
- Modify: `docs/phases/phase-2.2-backend-quality-gates/acceptance.md`
- Modify: `docs/phases/phase-2.2-backend-quality-gates/README.md`
- Create: `.superpowers/sdd/final-fix-report.md`

- [ ] Run architecture CLI/tests, workflow contract, `go test ./...`, `go vet ./...`, and all six Go builds.
- [ ] Run Linux `go test -race ./internal/modules/...`.
- [ ] Run strict uncached MySQL 5.7 integration with cleanup.
- [ ] Run the authoritative `bash ./scripts/smoke_schema_migrate.sh` apply/checksum/rollback/replay lifecycle.
- [ ] Commit source/doc changes, calculate the final source commit/tree/archive checksum, then refresh acceptance evidence with those exact values.
- [ ] Request read-only code review for the full `45cc401..HEAD` range and fix all Critical/Important findings through new RED-GREEN cycles.
- [ ] Run `git diff --check`, repeat all affected verification commands, write the final report, commit it, and confirm a clean worktree.
