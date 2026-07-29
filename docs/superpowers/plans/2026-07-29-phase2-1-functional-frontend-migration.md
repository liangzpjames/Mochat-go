# Phase 2.1 Functional Frontend Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace every route-only React placeholder with a functional React module derived from the pinned Vue source, and prove every legacy URL through repeatable browser interaction before Phase 3.

**Architecture:** Organize React code by business feature instead of Vue file shape. Each feature owns typed API adapters, query hooks, forms, route views, and tests; applications share only generic UI primitives and stable domain selectors. Browser acceptance runs against the real Go service, MariaDB, Redis, deterministic seed data, and fake external platforms.

**Tech Stack:** React 19, TypeScript 5.9, React Router 7, TanStack Query 5, Ant Design 6 for Dashboard, CSS-first mobile UI for Sidebar/Operation, Vitest, Playwright, Go, MariaDB, Redis.

## Global Constraints

- Preserve all existing Dashboard, Sidebar, and Operation URLs, query parameters, hashes, and deep links.
- Match legacy Vue business fields, permissions, API behavior, and user-visible operations; use current React interaction and visual conventions.
- Browser-to-Go and Go-to-database/cache calls must be real; only external WeCom/WeChat services may be fake.
- No business route may pass completion through `MigratedDashboardPage`, `web/apps/sidebar/src/page.tsx`, or `web/apps/operation/src/page.tsx`.
- Every applicable feature must demonstrate create, read, update, status transition, and delete with deterministic acceptance data.
- Phase 3 remains blocked until the Phase 2.1 completion audit and manual-browser ledger both pass.
- Preserve unrelated user changes in `.workbuddy/` and `web/saas-admin/`.

---

### Task 1: Phase 2.1 functional inventory and hard completion gate

**Files:**
- Create: `docs/phases/phase-2.1-functional-frontend-migration/README.md`
- Create: `docs/phases/phase-2.1-functional-frontend-migration/functional-matrix.csv`
- Create: `scripts/check_phase2_1_frontend_completion.mjs`
- Create: `scripts/check_phase2_1_frontend_completion.test.mjs`
- Modify: `package.json`

**Interfaces:**
- Consumes: `web/apps/*/src/migration-routes.json`, `web/legacy/*`, Phase 1 page/API inventories.
- Produces: `pnpm check:phase2.1`, a deterministic list of route-to-feature mappings, and a failing gate for placeholders or missing browser evidence.

- [ ] **Step 1: Write failing audit tests**

Create test fixtures that assert the checker rejects:

```js
{
  app: 'dashboard',
  route: '/example',
  feature: '',
  implementation: 'placeholder',
  read: 'missing',
  write: 'missing',
  browser: 'missing',
}
```

and accepts only rows whose applicable capabilities equal `passed` or explicitly equal `not-applicable:<reason>`.

Run: `node --test scripts/check_phase2_1_frontend_completion.test.mjs`

Expected: FAIL because the checker does not exist.

- [ ] **Step 2: Implement the matrix checker**

The checker must:

```js
export const allowedStates = new Set(['passed']);
export const isAcceptedState = (value) =>
  allowedStates.has(value) || /^not-applicable:.+/.test(value);
```

It must compare every manifest route with exactly one CSV row, reject `placeholder` and `route-only`, verify referenced feature/test/evidence files exist, and fail when a business route resolves to any generic migration page.

- [ ] **Step 3: Generate the initial matrix**

Include columns:

```csv
app,feature,legacy_sources,routes,react_module,go_apis,read,write,permissions,fake_external,component_tests,browser,evidence
```

Populate every current route. Existing specialized Dashboard pages may start as `passed` only after their current tests and browser behavior are reverified; all generic routes start as `missing`.

- [ ] **Step 4: Register and verify the gate**

Add:

```json
"check:phase2.1": "node scripts/check_phase2_1_frontend_completion.mjs"
```

Run:

```powershell
node --test scripts/check_phase2_1_frontend_completion.test.mjs
pnpm check:phase2.1
```

Expected: unit tests PASS; repository gate FAILS with the exact remaining functional gaps.

- [ ] **Step 5: Commit**

```powershell
git add package.json scripts/check_phase2_1_frontend_completion.mjs scripts/check_phase2_1_frontend_completion.test.mjs docs/phases/phase-2.1-functional-frontend-migration
git commit -m "test: add phase2.1 functional frontend gate"
```

### Task 2: Shared functional page primitives

**Files:**
- Create: `web/packages/ui/src/data-page.tsx`
- Create: `web/packages/ui/src/filter-bar.tsx`
- Create: `web/packages/ui/src/async-boundary.tsx`
- Create: `web/packages/ui/src/status-action.tsx`
- Create: `web/packages/ui/src/data-page.test.tsx`
- Modify: `web/packages/ui/src/index.ts`
- Create: `web/apps/dashboard/src/shared/query-state.ts`
- Create: `web/apps/dashboard/src/shared/query-state.test.ts`

**Interfaces:**
- Produces: reusable list/detail loading, empty, error, pagination, filter, confirmation, and URL-query primitives.
- Consumers: every Dashboard feature and equivalent lightweight patterns in Sidebar/Operation.

- [ ] **Step 1: Write component and URL-state tests**

Test:

- loading renders a named skeleton;
- empty renders the provided empty action;
- retry invokes exactly once;
- pagination writes `page` and `perPage`;
- filters preserve unrelated query parameters;
- status confirmation does not call mutation before confirmation.

Run:

```powershell
pnpm --filter @mochat/ui test
pnpm --filter @mochat/dashboard test -- src/shared/query-state.test.ts
```

Expected: FAIL because the primitives do not exist.

- [ ] **Step 2: Implement minimal primitives**

Use controlled props and React Router `URLSearchParams`; do not introduce a global store. Export stable interfaces:

```ts
export type PageState<T> =
  | { status: 'loading' }
  | { status: 'error'; message: string; retry: () => void }
  | { status: 'ready'; rows: T[] };
```

```ts
export function updateSearch(
  current: URLSearchParams,
  changes: Record<string, string | number | undefined>,
): URLSearchParams;
```

- [ ] **Step 3: Verify shared primitives**

Run:

```powershell
pnpm --filter @mochat/ui lint
pnpm --filter @mochat/ui test
pnpm --filter @mochat/dashboard test -- src/shared/query-state.test.ts
```

Expected: PASS.

- [ ] **Step 4: Commit**

```powershell
git add web/packages/ui web/apps/dashboard/src/shared
git commit -m "feat: add reusable functional page primitives"
```

### Task 3: Dashboard foundational business domains

**Files:**
- Modify: `web/apps/dashboard/src/features/corp/**`
- Modify: `web/apps/dashboard/src/features/employee/**`
- Modify: `web/apps/dashboard/src/features/department/**`
- Modify: `web/apps/dashboard/src/features/contact-field/**`
- Modify: `web/apps/dashboard/src/features/contact-tag/**`
- Modify: `web/apps/dashboard/src/features/role/**`
- Modify: `web/apps/dashboard/src/features/menu-admin/**`
- Modify: `web/apps/dashboard/src/features/user-admin/**`
- Modify: `web/apps/dashboard/src/features/password/**`
- Create: `web/e2e/tests/phase2-1-foundation.spec.ts`
- Modify: `docs/phases/phase-2.1-functional-frontend-migration/functional-matrix.csv`

**Interfaces:**
- Produces: stable corp/employee/department/contact metadata/permission selectors used by later features.

- [ ] **Step 1: Extract legacy behavior contracts**

For each domain, record in its test file:

- legacy source path;
- fields and validation;
- permission-controlled actions;
- list/detail/write API paths;
- URL query behavior.

Add missing tests before changing implementations.

- [ ] **Step 2: Verify and complete domain behavior**

Implement missing legacy-visible behavior using existing API modules. Remove local one-off loading/error/table patterns in favor of Task 2 primitives. Ensure every write invalidates only its feature query keys.

- [ ] **Step 3: Add browser CRUD coverage**

The browser test must authenticate, select a seeded corp, and exercise:

- enterprise detail edit;
- employee filtering and synchronization;
- department member dialog;
- contact field create/status/delete;
- contact tag create/edit/status;
- role create/copy/member guard;
- menu create/edit/status;
- user create/enable/password reset;
- self password validation without completing a destructive password change.

- [ ] **Step 4: Manually inspect all foundation routes**

Open each route at desktop width, click all primary tabs/actions, inspect console and failed requests, and record screenshots/evidence rows.

- [ ] **Step 5: Verify and commit**

Run:

```powershell
pnpm --filter @mochat/dashboard lint
pnpm --filter @mochat/dashboard typecheck
pnpm --filter @mochat/dashboard test
pnpm --filter @mochat/e2e test:e2e -- tests/phase2-1-foundation.spec.ts
pnpm check:phase2.1
```

Commit:

```powershell
git add web/apps/dashboard/src/features web/e2e/tests/phase2-1-foundation.spec.ts docs/phases/phase-2.1-functional-frontend-migration
git commit -m "feat: complete phase2.1 dashboard foundations"
```

### Task 4: Dashboard customer, content, and analytics domains

**Files:**
- Create: `web/apps/dashboard/src/features/work-contact/**`
- Create: `web/apps/dashboard/src/features/work-room/**`
- Create: `web/apps/dashboard/src/features/contact-transfer/**`
- Create: `web/apps/dashboard/src/features/material-library/**`
- Create: `web/apps/dashboard/src/features/statistics/**`
- Create: `web/apps/dashboard/src/features/greeting/**`
- Create: `web/apps/dashboard/src/features/room-welcome/**`
- Create: `web/apps/dashboard/src/features/official-account/**`
- Modify: `web/apps/dashboard/src/pages/dashboard-page-loaders.ts`
- Create: `web/e2e/tests/phase2-1-customer-content.spec.ts`
- Modify: `docs/phases/phase-2.1-functional-frontend-migration/functional-matrix.csv`

**Interfaces:**
- Consumes: shared selectors and page primitives.
- Produces: functional customer, group, inheritance, material, statistics, welcome-message, and official-account routes.

- [ ] **Step 1: Write API adapter tests from Vue call sites**

Cover exact methods and fields for:

- `/workContact/*`
- `/workRoom/*`
- `/contactTransfer/*`
- `/mediumGroup/*` and `/medium/*`
- `/corpData/*`
- `/greeting/*`
- `/roomWelcome/*`
- `/officialAccount/*`

Run focused Vitest tests and confirm they fail before implementations exist.

- [ ] **Step 2: Implement list/detail/write modules**

Each domain must expose a route component and query-key factory. Reuse selectors for employees, contacts, rooms, tags, and materials. Official-account external authorization uses fake WeChat responses but real Go endpoints.

- [ ] **Step 3: Replace route loaders**

Every route in these domains must import its explicit feature route component. Delete its mapping to `loadMigratedPage`.

- [ ] **Step 4: Test and manually browse**

Exercise filters, pagination, detail, create/edit/delete/status operations where applicable; inspect desktop layout, console, and requests.

- [ ] **Step 5: Verify and commit**

Run feature tests, focused E2E, production build, and `pnpm check:phase2.1`.

Commit:

```powershell
git add web/apps/dashboard/src/features web/apps/dashboard/src/pages/dashboard-page-loaders.ts web/e2e/tests/phase2-1-customer-content.spec.ts docs/phases/phase-2.1-functional-frontend-migration
git commit -m "feat: migrate dashboard customer and content domains"
```

### Task 5: Dashboard automation, messaging, and group-operation domains

**Files:**
- Create: `web/apps/dashboard/src/features/auto-tag/**`
- Create: `web/apps/dashboard/src/features/contact-message-batch/**`
- Create: `web/apps/dashboard/src/features/room-message-batch/**`
- Create: `web/apps/dashboard/src/features/room-tag-pull/**`
- Create: `web/apps/dashboard/src/features/work-room-auto-pull/**`
- Create: `web/apps/dashboard/src/features/contact-sop/**`
- Create: `web/apps/dashboard/src/features/room-sop/**`
- Create: `web/apps/dashboard/src/features/sensitive-words/**`
- Modify: `web/apps/dashboard/src/pages/dashboard-page-loaders.ts`
- Create: `web/e2e/tests/phase2-1-automation-messaging.spec.ts`
- Modify: `docs/phases/phase-2.1-functional-frontend-migration/functional-matrix.csv`

**Interfaces:**
- Consumes: contacts, rooms, employees, tags, and materials selectors.
- Produces: functional automation and messaging workflows with deterministic fake WeCom side effects.

- [ ] **Step 1: Write legacy contract and form-flow tests**

For every multi-step flow, assert validation and serialized request bodies at each step. Include scheduled/immediate modes, selected employee/contact/room scope, material payloads, and update hydration.

- [ ] **Step 2: Implement feature modules**

Use shared step/form components only where fields and semantics match. Persist draft state within the route module and preserve it on recoverable API failures.

- [ ] **Step 3: Replace all related placeholder loaders**

Map each old list/create/show/detail URL to explicit route views.

- [ ] **Step 4: Browser-test representative full flows**

Create, view, edit/status, and delete one deterministic record per feature. Assert fake external request payloads and visible persisted results.

- [ ] **Step 5: Manually inspect every route and commit**

Run static, component, focused E2E, build, and Phase 2.1 gates before committing.

### Task 6: Dashboard growth and campaign domains

**Files:**
- Create: `web/apps/dashboard/src/features/channel-code/**`
- Create: `web/apps/dashboard/src/features/shop-code/**`
- Create: `web/apps/dashboard/src/features/radar/**`
- Create: `web/apps/dashboard/src/features/lottery/**`
- Create: `web/apps/dashboard/src/features/work-fission/**`
- Create: `web/apps/dashboard/src/features/room-fission/**`
- Create: `web/apps/dashboard/src/features/room-clock-in/**`
- Create: `web/apps/dashboard/src/features/room-quality/**`
- Create: `web/apps/dashboard/src/features/room-calendar/**`
- Create: `web/apps/dashboard/src/features/room-remind/**`
- Create: `web/apps/dashboard/src/features/room-infinite-pull/**`
- Modify: `web/apps/dashboard/src/pages/dashboard-page-loaders.ts`
- Create: `web/e2e/tests/phase2-1-growth-campaigns.spec.ts`
- Modify: `docs/phases/phase-2.1-functional-frontend-migration/functional-matrix.csv`

**Interfaces:**
- Produces: Dashboard authoring and management surfaces consumed by Operation campaign pages.

- [ ] **Step 1: Write per-domain API and wizard tests**

Cover list, create, info/show, update, statistics, customer/room detail, status, and delete endpoints found in the Vue API inventory.

- [ ] **Step 2: Implement management modules**

Use shared campaign shell, metric cards, scoped selectors, material picker, and wizard primitives only where behavior matches. Keep each campaign domain independently importable and testable.

- [ ] **Step 3: Replace placeholder loaders and delete dead generic mappings**

After the last Dashboard route is migrated, remove `MigratedDashboardPage` and make the audit reject its reintroduction.

- [ ] **Step 4: Browser-test and manually inspect all campaign routes**

Use fake WeCom/WeChat for QR codes, message templates, contact ways, and media uploads. Verify persisted Go/database state after each representative flow.

- [ ] **Step 5: Verify and commit**

Run the full Dashboard suite, all Dashboard Phase 2.1 E2E files, production build, and completion gate.

### Task 7: Sidebar functional mobile application

**Files:**
- Create: `web/apps/sidebar/src/app/**`
- Create: `web/apps/sidebar/src/features/auth/**`
- Create: `web/apps/sidebar/src/features/contact/**`
- Create: `web/apps/sidebar/src/features/contact-batch-add/**`
- Create: `web/apps/sidebar/src/features/contact-sop/**`
- Create: `web/apps/sidebar/src/features/room-sop/**`
- Create: `web/apps/sidebar/src/features/materials/**`
- Modify: `web/apps/sidebar/src/page-loaders.ts`
- Delete: `web/apps/sidebar/src/page.tsx`
- Create: `web/e2e/tests/phase2-1-sidebar.spec.ts`
- Modify: `docs/phases/phase-2.1-functional-frontend-migration/functional-matrix.csv`

**Interfaces:**
- Consumes: sidebar cookie auth, Go sidebar APIs, fake WeCom OAuth/JSSDK.
- Produces: functional mobile customer profile, remark/tag editing, customer batch-add detail, SOP, and material selection routes.

- [ ] **Step 1: Write auth and route tests**

Test login redirect, auth callback, token/agent cookie persistence, target safety, 401 recovery, and every manifest route mapping to a distinct functional view.

- [ ] **Step 2: Implement contact and material workflows**

Cover customer detail, profile fields, rooms/tags/tracks, remark editing, tag setting, custom-field editing, material groups/list, media refresh, and upload.

- [ ] **Step 3: Implement SOP and batch-add workflows**

Cover personal SOP reminder/detail, room SOP completion, calendar/quality auxiliary views, and batch-add progress.

- [ ] **Step 4: Browser-test fake OAuth and mobile layouts**

Run at `390x844`, complete OAuth callback, visit every old URL, click tabs/actions, perform applicable updates, and assert console/request cleanliness.

- [ ] **Step 5: Delete generic page, verify, and commit**

Run Sidebar lint/typecheck/test/build, focused E2E, and Phase 2.1 gate.

### Task 8: Operation functional campaign application

**Files:**
- Create: `web/apps/operation/src/app/**`
- Create: `web/apps/operation/src/features/lottery/**`
- Create: `web/apps/operation/src/features/room-clock-in/**`
- Create: `web/apps/operation/src/features/room-fission/**`
- Create: `web/apps/operation/src/features/room-infinite-pull/**`
- Create: `web/apps/operation/src/features/shop-code/**`
- Create: `web/apps/operation/src/features/work-fission/**`
- Modify: `web/apps/operation/src/page-loaders.ts`
- Delete: `web/apps/operation/src/page.tsx`
- Create: `web/e2e/tests/phase2-1-operation.spec.ts`
- Modify: `docs/phases/phase-2.1-functional-frontend-migration/functional-matrix.csv`

**Interfaces:**
- Consumes: Dashboard-authored campaign records, operation session auth, fake WeChat OAuth/JSSDK.
- Produces: functional campaign landing, explanation, progress, prize, check-in, QR, and redemption views.

- [ ] **Step 1: Write OAuth/session and route tests**

Test safe targets, fake OAuth callback, session cookie, open-user loading, and explicit route components.

- [ ] **Step 2: Implement campaign views**

Each domain reads the real Go API and renders its own campaign model. Share only mobile campaign primitives such as hero, progress, prize card, rule section, QR card, and result modal.

- [ ] **Step 3: Implement interactive flows**

Cover lottery draw/result, clock-in/result/ranking, fission invite/progress/reward, infinite-pull QR, shop-code location/QR, and work-fission poster/progress/reward.

- [ ] **Step 4: Browser-test all routes at mobile and desktop sizes**

Use fake OAuth/external APIs, verify every main action and persisted state, and record screenshots plus request/console summaries.

- [ ] **Step 5: Delete generic page, verify, and commit**

Run Operation lint/typecheck/test/build, focused E2E, and Phase 2.1 gate.

### Task 9: Full Phase 2.1 acceptance and Phase 3 release gate

**Files:**
- Create: `scripts/phase2_1_frontend_acceptance.ps1`
- Create: `docs/phases/phase-2.1-functional-frontend-migration/evidence/README.md`
- Create: `docs/phases/phase-2.1-functional-frontend-migration/evidence/results.json`
- Modify: `docs/phases/phase-2.1-functional-frontend-migration/functional-matrix.csv`
- Modify: `docs/phases/phase-3-yuanhu-benchmark/README.md`

**Interfaces:**
- Consumes: all feature tests, deterministic stack, fake external platforms, browser evidence.
- Produces: one reproducible Windows-compatible acceptance command and an explicit Phase 3 go/no-go result.

- [ ] **Step 1: Write acceptance-runner tests**

Test that the runner fails on:

- missing route evidence;
- placeholder implementation;
- failed CRUD capability;
- unexpected browser console error;
- unexpected `4xx/5xx`;
- missing mobile evidence for Sidebar/Operation.

- [ ] **Step 2: Implement the PowerShell acceptance runner**

The runner must:

1. verify toolchain;
2. run lint/typecheck/unit/build/audits;
3. start isolated MariaDB/Redis/Go/fake external services;
4. seed deterministic tenant/corp/users/business data;
5. run all Phase 2.1 Playwright projects;
6. aggregate route, action, request, console, and screenshot evidence;
7. clean up only its own project and processes;
8. write `results.json`.

- [ ] **Step 3: Run automated full acceptance**

Run:

```powershell
pwsh -File scripts/phase2_1_frontend_acceptance.ps1
pnpm check:phase2.1
```

Expected: all gates PASS and every matrix row is accepted.

- [ ] **Step 4: Perform final manual browser walk**

Using the acceptance stack:

- log in to Dashboard;
- open and click every Dashboard URL at desktop width;
- complete fake OAuth and open every Sidebar URL at mobile width;
- complete fake OAuth and open every Operation URL at mobile and desktop widths;
- open SaaS Admin and verify platform navigation remains unaffected.

Record any warning or visual defect before declaring completion.

- [ ] **Step 5: Mark Phase 3 unblocked and commit**

Only when automated and manual evidence are complete, update the Phase 3 README from blocked to ready.

Commit:

```powershell
git add scripts/phase2_1_frontend_acceptance.ps1 docs/phases/phase-2.1-functional-frontend-migration docs/phases/phase-3-yuanhu-benchmark/README.md
git commit -m "test: complete phase2.1 frontend acceptance"
```
