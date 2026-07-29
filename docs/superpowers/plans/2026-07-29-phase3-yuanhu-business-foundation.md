# Phase 3 Yuanhu Business Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first production-shaped Phase 3 vertical slice: customer master data, leads, opportunities, ownership/collaboration, and the contracts used by marketing and reporting.

**Architecture:** Extend the existing tenant-aware Go service with a focused SCRM domain instead of adding another application framework. React Dashboard pages consume versioned typed APIs through the existing API client and query cache. Domain events feed later reporting, risk, and AI modules without coupling the first slice to those implementations.

**Tech Stack:** Go 1.26, MySQL/MariaDB, Redis, React 19, TypeScript, Ant Design, TanStack Query, Vitest, Playwright, Docker Compose.

## Global Constraints

- Every record is tenant scoped; corp-scoped records additionally require authorized corp access.
- Data permissions support owner, collaborator, department, descendant departments, and tenant-wide scopes.
- Mutating endpoints require optimistic version checks and stable idempotency keys where retries are possible.
- No AI feature is introduced in this slice.
- No live message, group-send, purchase, or destructive third-party action is used in automated tests.
- Existing Phase 2 routes remain stable while real pages replace generic migrated-page loaders.

---

### Task 1: Freeze the SCRM vocabulary and contracts

**Files:**
- Create: `docs/phases/phase-3-yuanhu-benchmark/scrm-domain-contract.md`
- Create: `docs/phases/phase-3-yuanhu-benchmark/scrm-api-contract.yaml`
- Test: `scripts/check_phase3_scrm_contract.mjs`

**Interfaces:**
- Produces: canonical `Lead`, `ContactProfile`, `CustomerAssignment`, `Opportunity`, `OpportunityStage`, and `FollowUp` schemas.
- Produces: state transitions and error codes consumed by all later tasks.

- [ ] **Step 1: Write a failing contract checker**

Create a Node test that requires every mutable resource to define `tenantId`, `version`, `createdAt`, `updatedAt`, permission rules, and allowed transitions.

- [ ] **Step 2: Run the checker**

Run: `node scripts/check_phase3_scrm_contract.mjs`

Expected: FAIL because both contract documents are absent.

- [ ] **Step 3: Write the domain and API contracts**

Define:

- lead states: `new`, `qualified`, `converted`, `discarded`;
- assignment states: `owned`, `collaborating`, `public_pool`;
- opportunity states: configurable stage plus terminal `won` and `lost`;
- immutable follow-up events;
- `409` for version conflicts, `403` for data-scope denial, and `422` for invalid transitions.

- [ ] **Step 4: Verify the contract**

Run: `node scripts/check_phase3_scrm_contract.mjs`

Expected: PASS with all resources and transitions reported.

- [ ] **Step 5: Commit**

```bash
git add docs/phases/phase-3-yuanhu-benchmark scripts/check_phase3_scrm_contract.mjs
git commit -m "docs: define phase3 SCRM contracts"
```

### Task 2: Add tenant-scoped SCRM persistence

**Files:**
- Create: `deploy/standalone/migrations/0090_scrm_customer_lifecycle.up.sql`
- Create: `deploy/standalone/migrations/0090_scrm_customer_lifecycle.down.sql`
- Create: `internal/store/scrm.go`
- Create: `internal/store/scrm_test.go`

**Interfaces:**
- Consumes: Task 1 resource and transition definitions.
- Produces: `SCRMStore` methods for leads, assignments, opportunities, stages, and follow-ups.

- [ ] **Step 1: Write store tests**

Cover tenant isolation, duplicate external keys, optimistic versions, concurrent public-pool claims, collaborator visibility, stage transitions, and immutable follow-ups.

- [ ] **Step 2: Run focused tests**

Run: `go test ./internal/store -run 'TestSCRM'`

Expected: FAIL because the migration and store are absent.

- [ ] **Step 3: Add migration and store**

Use explicit `tenant_id`, nullable `corp_id`, integer `version`, soft-delete columns, unique tenant-scoped business keys, and indexes for owner, stage, state, next follow-up, and updated time.

- [ ] **Step 4: Re-run focused tests**

Run: `go test ./internal/store -run 'TestSCRM'`

Expected: PASS.

- [ ] **Step 5: Verify migration lifecycle**

Run the repository migration apply, checksum, rollback, and replay checks against the standalone database.

Expected: migration 0090 applies, rolls back, and reapplies without drift.

- [ ] **Step 6: Commit**

```bash
git add deploy/standalone/migrations/0090_scrm_customer_lifecycle.* internal/store/scrm*
git commit -m "feat: add SCRM customer lifecycle persistence"
```

### Task 3: Implement authorization and SCRM APIs

**Files:**
- Create: `internal/dashboard/scrm.go`
- Create: `internal/dashboard/scrm_test.go`
- Modify: `cmd/mochat-go/main.go`
- Modify: `internal/server/routes.go`

**Interfaces:**
- Consumes: `SCRMStore`.
- Produces: `/dashboard/scrm/leads`, `/contacts`, `/assignments`, `/opportunities`, `/stages`, and `/followUps`.

- [ ] **Step 1: Write failing handler tests**

Test list filters, pagination, create/update transitions, owner transfer, collaborator changes, atomic public-pool claim, permission denial, idempotent retry, and version conflict.

- [ ] **Step 2: Run focused tests**

Run: `go test ./internal/dashboard -run 'TestSCRM'`

Expected: FAIL because the handler is absent.

- [ ] **Step 3: Implement handlers and route registration**

Reuse the current user resolver, corp authorizer, tenant context, JSON envelope, and audit conventions. Never trust tenant, owner, or permission scope supplied only by the client.

- [ ] **Step 4: Re-run focused tests**

Run: `go test ./internal/dashboard -run 'TestSCRM'`

Expected: PASS.

- [ ] **Step 5: Run frontend/API inventory**

Run: `pnpm refresh:audit`

Expected: the new endpoints are present without unexplained gaps.

- [ ] **Step 6: Commit**

```bash
git add internal/dashboard/scrm* internal/server/routes.go cmd/mochat-go/main.go docs/phases/phase-1-frontend-foundation/audit
git commit -m "feat: expose tenant-scoped SCRM APIs"
```

### Task 4: Build the Contact and Lead React pages

**Files:**
- Create: `web/apps/dashboard/src/features/scrm/scrm-api.ts`
- Create: `web/apps/dashboard/src/features/scrm/scrm-api.test.ts`
- Create: `web/apps/dashboard/src/features/scrm/contact-page.tsx`
- Create: `web/apps/dashboard/src/features/scrm/contact-page.test.tsx`
- Create: `web/apps/dashboard/src/features/scrm/lead-page.tsx`
- Create: `web/apps/dashboard/src/features/scrm/lead-page.test.tsx`
- Modify: `web/apps/dashboard/src/main.tsx`
- Modify: `web/apps/dashboard/src/pages/dashboard-page-loaders.ts`

**Interfaces:**
- Consumes: Task 3 APIs.
- Produces: real React implementations for contact and lead routes.

- [ ] **Step 1: Write API and component tests**

Cover source/status/tag/keyword filters, pagination, add, owner transfer, collaborator update, abandon, public-pool move, batch tag updates, loading, empty, forbidden, conflict, and retry states.

- [ ] **Step 2: Run tests**

Run: `pnpm --filter @mochat/dashboard test -- src/features/scrm`

Expected: FAIL because the feature is absent.

- [ ] **Step 3: Implement typed API and pages**

Use existing Dashboard access context, corp provider, TanStack Query keys containing tenant/corp/filter state, Ant Design accessible controls, and conflict refresh prompts.

- [ ] **Step 4: Replace generic route loaders**

Wire the real pages in `main.tsx`; preserve current URLs and query parameters.

- [ ] **Step 5: Re-run tests**

Run: `pnpm --filter @mochat/dashboard test -- src/features/scrm`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add web/apps/dashboard/src/features/scrm web/apps/dashboard/src/main.tsx web/apps/dashboard/src/pages/dashboard-page-loaders.ts
git commit -m "feat: add SCRM lead and contact pages"
```

### Task 5: Build Opportunity and Follow-up workflows

**Files:**
- Create: `web/apps/dashboard/src/features/scrm/opportunity-page.tsx`
- Create: `web/apps/dashboard/src/features/scrm/opportunity-page.test.tsx`
- Create: `web/apps/dashboard/src/features/scrm/follow-up-timeline.tsx`
- Create: `web/apps/dashboard/src/features/scrm/follow-up-timeline.test.tsx`
- Modify: `web/apps/dashboard/src/features/scrm/scrm-api.ts`

**Interfaces:**
- Consumes: opportunities, stages, and follow-up APIs.
- Produces: opportunity list/editor and immutable customer timeline.

- [ ] **Step 1: Write failing workflow tests**

Cover stage/date/owner filters, expected amount and close date validation, stage transition, won/lost terminal rules, owner transfer, collaborator visibility, and chronological follow-up rendering.

- [ ] **Step 2: Run focused tests**

Run: `pnpm --filter @mochat/dashboard test -- src/features/scrm/opportunity-page.test.tsx src/features/scrm/follow-up-timeline.test.tsx`

Expected: FAIL.

- [ ] **Step 3: Implement opportunity and timeline UI**

Render current stage and transition controls separately; require explicit reasons for `lost`; display author, event time, next follow-up, and source for each follow-up event.

- [ ] **Step 4: Re-run focused tests**

Run the command from Step 2.

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/apps/dashboard/src/features/scrm
git commit -m "feat: add opportunity and follow-up workflows"
```

### Task 6: Add business events and first reporting slice

**Files:**
- Create: `internal/dashboard/scrm_metrics.go`
- Create: `internal/dashboard/scrm_metrics_test.go`
- Create: `web/apps/dashboard/src/features/scrm/scrm-report-page.tsx`
- Create: `web/apps/dashboard/src/features/scrm/scrm-report-page.test.tsx`
- Create: `docs/phases/phase-3-yuanhu-benchmark/metric-dictionary.md`

**Interfaces:**
- Consumes: immutable SCRM lifecycle events.
- Produces: lead-to-contact and opportunity funnel metrics with documented definitions.

- [ ] **Step 1: Write metric definition and failing tests**

Define denominator, event time, tenant timezone, deduplication, and late-event treatment for lead count, qualified count, conversion count, open opportunity amount, won amount, and stage conversion.

- [ ] **Step 2: Run tests**

Run: `go test ./internal/dashboard -run 'TestSCRMMetrics'`

Expected: FAIL.

- [ ] **Step 3: Implement aggregate API**

Support date range, corp, department, owner, source, and stage filters; return summary, trend, distribution, and paged detail references.

- [ ] **Step 4: Implement and test the report page**

Run: `pnpm --filter @mochat/dashboard test -- src/features/scrm/scrm-report-page.test.tsx`

Expected: PASS after summary, trend, distribution, and detail states are implemented.

- [ ] **Step 5: Commit**

```bash
git add internal/dashboard/scrm_metrics* web/apps/dashboard/src/features/scrm/scrm-report-page* docs/phases/phase-3-yuanhu-benchmark/metric-dictionary.md
git commit -m "feat: add SCRM funnel reporting"
```

### Task 7: End-to-end evidence and release gate

**Files:**
- Create: `web/e2e/tests/phase3-scrm.spec.ts`
- Create: `docs/phases/phase-3-yuanhu-benchmark/acceptance.md`
- Modify: `package.json`

**Interfaces:**
- Consumes: all prior Phase 3 tasks.
- Produces: deterministic SCRM acceptance evidence and a release decision.

- [ ] **Step 1: Write failing Playwright scenarios**

Cover lead creation and conversion, contact ownership/collaboration, public-pool claim race, opportunity stage progression, follow-up timeline, permission denial, conflict refresh, and funnel report updates.

- [ ] **Step 2: Run Playwright**

Run: `pnpm --filter @mochat/e2e test:e2e -- phase3-scrm.spec.ts`

Expected: FAIL before fixtures and UI are complete.

- [ ] **Step 3: Add deterministic tenant fixtures**

Use isolated tenant/corp/user IDs and mock only external WeCom boundaries; exercise the real Go API and database for SCRM behavior.

- [ ] **Step 4: Re-run all Phase 3 gates**

Run:

```bash
go test ./internal/store ./internal/dashboard ./internal/frontend
pnpm test
pnpm build
pnpm check:audit
pnpm --filter @mochat/e2e test:e2e -- phase3-scrm.spec.ts
docker compose -f deploy/standalone/docker-compose.yml --profile app up -d --build
```

Expected: all commands exit 0; application, MySQL, and Redis report healthy.

- [ ] **Step 5: Record acceptance**

Document scenario, role, fixture, expected result, actual result, screenshot/trace, and unresolved real-WeCom validation debt.

- [ ] **Step 6: Commit**

```bash
git add web/e2e/tests/phase3-scrm.spec.ts docs/phases/phase-3-yuanhu-benchmark/acceptance.md package.json
git commit -m "test: add phase3 SCRM release gate"
```
