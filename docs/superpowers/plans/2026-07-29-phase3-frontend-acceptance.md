# Phase 3 Frontend Acceptance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Before Phase 3 starts, prove that Dashboard, Sidebar, Operation, and SaaS Admin build successfully and render usable authenticated interfaces through the standalone Go stack.

**Architecture:** Use the repository's pnpm workspace checks for deterministic source validation, then use the standalone Go service with MariaDB and Redis for real login and browser acceptance. Preserve existing user changes; only make a code change when a reproduced failure has a confirmed root cause.

**Tech Stack:** pnpm 11, TypeScript, React, Vite, Vitest, Playwright/browser UI validation, Go, MariaDB, Redis.

## Global Constraints

- Validate all four applications: `dashboard`, `sidebar`, `operation`, and `saas-admin`.
- Acceptance requires successful builds, accessible HTTP entry points, and visible non-error UI after authentication or the application's supported signed context.
- Do not modify or remove pre-existing untracked `.workbuddy/` or `web/saas-admin/` content.
- Record exact failures and fresh verification evidence; do not infer success from prior evidence files.

---

### Task 1: Deterministic frontend gates

**Files:**
- Inspect: `package.json`
- Inspect: `web/apps/*/package.json`
- Test: `web/apps/**`

**Interfaces:**
- Consumes: the checked-out pnpm workspace and installed dependencies.
- Produces: fresh lint, typecheck, unit-test, build, dependency-policy, and Phase 2 audit results.

- [ ] **Step 1: Confirm toolchain and workspace state**

Run: `node --version && pnpm --version && git status --short`

Expected: Node and pnpm versions print successfully; existing user changes are identified and preserved.

- [ ] **Step 2: Run source checks**

Run: `pnpm lint && pnpm typecheck && pnpm test`

Expected: exit code 0 with no failed workspace package.

- [ ] **Step 3: Build every frontend**

Run: `pnpm build`

Expected: exit code 0 and fresh `dist` output for all four applications.

- [ ] **Step 4: Run migration and dependency gates**

Run: `pnpm check:deps && pnpm check:audit`

Expected: exit code 0 with all migrated routes and current artifacts accepted.

### Task 2: Standalone runtime and login preparation

**Files:**
- Inspect: `deploy/standalone/docker-compose.yml`
- Inspect: `scripts/smoke_bootstrap_standalone.sh`
- Inspect: `cmd/mochat-go/**`

**Interfaces:**
- Consumes: fresh frontend `dist` output and Docker.
- Produces: healthy MariaDB/Redis, migrated schema, an acceptance administrator, and listening frontend endpoints.

- [ ] **Step 1: Start required services**

Run: `docker compose -f deploy/standalone/docker-compose.yml up -d mysql redis`

Expected: both services report healthy.

- [ ] **Step 2: Apply schema and create a local acceptance account**

Run the repository's standalone migration/bootstrap commands with localhost-only acceptance credentials.

Expected: migrations finish without error and the account can authenticate.

- [ ] **Step 3: Start the Go application with all frontend servers enabled**

Run the standalone Go application with `MOCHAT_GO_ENABLE_FRONTEND_SERVERS=1` and explicit local ports.

Expected: Dashboard/API, Sidebar, Operation, and SaaS Admin entry points return successful HTTP responses.

### Task 3: Browser acceptance for all four frontends

**Files:**
- Inspect: `web/apps/dashboard/src/**`
- Inspect: `web/apps/sidebar/src/**`
- Inspect: `web/apps/operation/src/**`
- Inspect: `web/apps/saas-admin/src/**`

**Interfaces:**
- Consumes: the running standalone stack and acceptance credentials/session context.
- Produces: observed URLs, visible UI assertions, console/page error results, and screenshots when useful.

- [ ] **Step 1: Validate Dashboard login**

Open the Dashboard login page, authenticate, and verify a protected React page is visible.

Expected: authentication succeeds, the browser leaves `/login`, and the Dashboard shell/content renders without a fatal error.

- [ ] **Step 2: Validate SaaS Admin**

Open `/saas-admin/` under the authenticated platform account and verify its main workspace is visible.

Expected: the SaaS Admin shell and at least one primary workspace render without a fatal error.

- [ ] **Step 3: Validate Sidebar**

Open the Sidebar standalone entry with the repository-supported signed test context and verify its customer UI.

Expected: a Sidebar React page is visible and usable without a fatal error.

- [ ] **Step 4: Validate Operation**

Open the Operation standalone entry with the repository-supported signed test context and verify its campaign UI.

Expected: an Operation React page is visible and usable without a fatal error.

- [ ] **Step 5: Record acceptance**

Re-run any affected checks after fixes, then report each application's URL, authentication method, visible assertion, console/page errors, and final pass/fail status.

Expected: all four applications have fresh, reproducible evidence or an explicit blocker requiring user action.
