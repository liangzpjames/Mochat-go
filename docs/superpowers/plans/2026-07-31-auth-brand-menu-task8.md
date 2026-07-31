# MoChat Auth, Branding, Menu, and Task8 Implementation Plan

> **For agentic workers:** Implement task-by-task with test-first changes and verify each checkpoint.

**Goal:** Isolate Dashboard and SaaS sessions, standardize MoChat branding, default the Dashboard menu collapsed, and add a repeatable Task8 browser acceptance gate.

**Architecture:** Dashboard and SaaS keep separate browser storage keys and logout paths. The shared identity login endpoint will emit only the SaaS token for SaaS pages, while Dashboard retains its own `/dashboard/user/auth` session. Benchmark navigation remains independently renderable from its manifest. Task8 adds deterministic browser checks and an aggregate package command.

**Tech Stack:** React, TypeScript, Vitest, Playwright, Go HTTP handlers, pnpm.

## Global Constraints

- Preserve existing data volumes and migration history.
- Do not use `ACCESS_TOKEN` as a cross-application fallback.
- Keep `MoChat` as the product brand in runtime UI; historical comparison documents may retain source naming.
- Task8 must report route, status, and screenshot paths on failure.

### Task 1: Session Storage Isolation

**Files:** `web/packages/auth/src/session.ts`, Dashboard auth tests, `web/apps/saas-admin/src/lib/api.ts`, identity login page tests/handler.

- Add failing tests proving Dashboard reads/writes only its namespace and SaaS ignores Dashboard storage.
- Remove SaaS fallback to `ACCESS_TOKEN`; use `mochat_saas_admin_token` only.
- Make Dashboard session keys use `mochat_dashboard_*` names and update logout cleanup.
- Update identity login page to write only the SaaS key and scoped cookie.
- Run package tests and Go identity page tests.

### Task 2: Brand and Menu Defaults

**Files:** Dashboard layout, SaaS login/admin runtime copy, affected tests and manifest labels.

- Add failing assertions for `MoChat AI` and collapsed initial groups.
- Replace runtime source-brand text with `MoChat`/`MoChat AI`.
- Initialize expanded groups with an empty set; preserve active-group auto expansion.
- Run Dashboard tests, typecheck, and build.

### Task 3: Task8 Browser Gate

**Files:** `web/e2e/benchmark/yuanhu-phase1.spec.ts`, `docs/benchmark/yuanhu/acceptance/phase1.md`, `package.json`, manifest checker, completion tests.

- Add browser tests for login, eight groups, representative P1/P2 pages, refresh/history, search/filter interactions, and console errors.
- Capture screenshots under a deterministic acceptance directory and emit route/status/path diagnostics.
- Add `check:yuanhu-phase1` chaining manifest, Dashboard typecheck/test/build, and E2E.
- Run the gate in Docker Desktop and record pass/fail evidence.

### Task 4: Deployment Verification

- Deploy through `scripts/deploy_docker_desktop.ps1` only.
- Confirm independent token behavior, SaaS routes enabled, all volumes preserved, and health endpoints passing.
- Commit each task separately and report exact verification output.
