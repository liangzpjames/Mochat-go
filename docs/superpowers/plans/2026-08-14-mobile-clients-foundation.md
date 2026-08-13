# 两个移动端基础框架实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为企微聊天侧边栏和营销活动 H5 建立共享但身份隔离的生产级 React 移动端基础框架，并交付客户摘要与任务宝进度两个真实 API 纵切面。

**Architecture:** 新增无业务状态的 `@mochat/mobile-foundation`，统一移动壳、请求错误和安全导航；Sidebar 与 Operation 各自维护认证、路由注册表与 feature API。两个应用保留既有 12/10 条 URL 和挂载前缀，不共享会话，不使用 Dashboard token。

**Tech Stack:** React 19.2、TypeScript 5.9 strict、React Router 7、TanStack Query 5、Vite 8、Vitest、Testing Library、Playwright、原生移动端 CSS。

## Global Constraints

- 所有生产行为按 TDD RED→GREEN→REFACTOR；每个任务独立提交并在进入下一任务前审阅。
- 只写 `D:\workspace\mochat-go\mochat-go\.worktrees\mobile-clients-foundation`，不修改主工作区 dirty 文件。
- 不操作 Docker、Compose、服务器、数据库和数据卷，不写 `output`。
- 保留 Sidebar 12 条和 Operation 10 条旧 URL、query、hash、`/sidebar-app`、`/operation-app`。
- Sidebar 使用自身员工 JWT/cookie/OAuth；Operation 使用活动 OAuth/同源 cookie；两者都不得读取 Dashboard storage key。
- 页面模块不得直接调用 `fetch`，不得出现随机奖品、固定假进度、假成功或乱码文案。
- 390px 下文档级无横向溢出，关键触控区域最小 44px，支持 `env(safe-area-inset-*)`。
- 未完成业务路由必须展示真实的模块迁移状态，不得伪造成业务可用；本阶段不宣称 22 条业务已全部迁移。

---

## 文件结构

```text
web/packages/mobile-foundation/
  package.json
  eslint.config.mjs
  tsconfig.json
  tsconfig.build.json
  src/index.ts
  src/api/client.ts
  src/api/client.test.ts
  src/navigation/safe-target.ts
  src/navigation/safe-target.test.ts
  src/shell/mobile-shell.tsx
  src/shell/mobile-shell.test.tsx
  src/shell/mobile-state.tsx
  src/styles/mobile.css

web/apps/sidebar/src/
  app/sidebar-router.tsx
  app/sidebar-router.test.tsx
  auth/sidebar-session.ts
  auth/sidebar-session.test.ts
  features/contact/contact-api.ts
  features/contact/contact-page.tsx
  features/contact/contact-page.test.tsx
  routes/registry.tsx

web/apps/operation/src/
  app/operation-router.tsx
  app/operation-router.test.tsx
  auth/operation-session.ts
  auth/operation-session.test.ts
  features/work-fission/work-fission-api.ts
  features/work-fission/work-fission-page.tsx
  features/work-fission/work-fission-page.test.tsx
  routes/registry.tsx

scripts/
  check_mobile_clients_foundation.mjs
  check_mobile_clients_foundation.test.mjs

web/e2e/tests/
  mobile-clients-foundation.spec.ts
```

## Task 1：共享移动端基础包

**Files:**

- Create: `web/packages/mobile-foundation/package.json`
- Create: `web/packages/mobile-foundation/eslint.config.mjs`
- Create: `web/packages/mobile-foundation/tsconfig.json`
- Create: `web/packages/mobile-foundation/tsconfig.build.json`
- Create: `web/packages/mobile-foundation/src/api/client.ts`
- Create: `web/packages/mobile-foundation/src/api/client.test.ts`
- Create: `web/packages/mobile-foundation/src/navigation/safe-target.ts`
- Create: `web/packages/mobile-foundation/src/navigation/safe-target.test.ts`
- Create: `web/packages/mobile-foundation/src/shell/mobile-shell.tsx`
- Create: `web/packages/mobile-foundation/src/shell/mobile-state.tsx`
- Create: `web/packages/mobile-foundation/src/shell/mobile-shell.test.tsx`
- Create: `web/packages/mobile-foundation/src/styles/mobile.css`
- Create: `web/packages/mobile-foundation/src/index.ts`

**Interfaces:**

- Produces: `createMobileApiClient(options).request<T>(path, init?)`
- Produces: `MobileApiError` with `kind`, `status`, `code`, `requestId`, `retryable`
- Produces: `safeInternalTarget(raw, fallback): string`
- Produces: `MobileShell`, `MobileState`, `MobileActionDock`, `MobileErrorBoundary`
- Constraint: client accepts only relative paths and never persists a token.

- [ ] **Step 1: Write failing API and navigation tests**

Cover the following exact cases:

```ts
it('adds scoped base path, same-origin credentials and optional bearer token')
it('unwraps { code, msg, data, errorCode, requestId }')
it('maps 401, 403, validation, server, network and abort separately')
it('rejects absolute request URLs')
it('accepts a same-origin path with query and hash')
it('rejects protocol-relative, external, javascript and malformed targets')
```

- [ ] **Step 2: Verify RED**

Run:

```powershell
corepack pnpm --filter @mochat/mobile-foundation test
```

Expected: FAIL because the package or exported APIs do not exist.

- [ ] **Step 3: Implement the minimal request and navigation APIs**

Use these public types verbatim:

```ts
export type MobileApiErrorKind =
  | 'unauthorized' | 'forbidden' | 'not-found' | 'conflict'
  | 'validation' | 'server' | 'network' | 'aborted';

export type MobileApiClientOptions = {
  basePath: '/sidebar' | '/operation';
  getToken?: () => string | null;
  onUnauthorized?: () => void;
  timeoutMs?: number;
};

export function safeInternalTarget(raw: string | null | undefined, fallback: string): string;
```

Every request sets `credentials: 'same-origin'`. Add Authorization only when `getToken` returns a token. Use `AbortSignal.any` when combining timeout and caller cancellation, and never log request headers or raw OAuth parameters.

- [ ] **Step 4: Write failing shell tests**

Assert semantic main/header, retry action, empty/error/forbidden/not-found variants, error-boundary fallback, safe-area class and 44px action contract.

- [ ] **Step 5: Implement shell, state and mobile tokens**

CSS must include `box-sizing: border-box`, `overflow-x: clip`, `min-height: 44px`, `env(safe-area-inset-top)` and `env(safe-area-inset-bottom)`. Components expose semantic roles and do not know either app name.

- [ ] **Step 6: Verify GREEN and package gates**

```powershell
corepack pnpm --filter @mochat/mobile-foundation lint
corepack pnpm --filter @mochat/mobile-foundation typecheck
corepack pnpm --filter @mochat/mobile-foundation test
corepack pnpm --filter @mochat/mobile-foundation build
```

Expected: all exit `0`.

- [ ] **Step 7: Commit**

```powershell
git add web/packages/mobile-foundation pnpm-lock.yaml
git commit -m "feat(mobile): add shared runtime foundation"
```

## Task 2：Sidebar 身份、路由与客户摘要纵切面

**Files:**

- Modify: `web/apps/sidebar/package.json`
- Modify: `web/apps/sidebar/index.html`
- Modify: `web/apps/sidebar/src/main.tsx`
- Modify: `web/apps/sidebar/src/styles.css`
- Delete: `web/apps/sidebar/src/app/sidebar-app.tsx`
- Delete: `web/apps/sidebar/src/app/sidebar-app.test.tsx`
- Replace: `web/apps/sidebar/src/app/catalog.ts`
- Replace: `web/apps/sidebar/src/app/catalog.test.ts`
- Create: `web/apps/sidebar/src/app/sidebar-router.tsx`
- Create: `web/apps/sidebar/src/app/sidebar-router.test.tsx`
- Create: `web/apps/sidebar/src/auth/sidebar-session.ts`
- Create: `web/apps/sidebar/src/auth/sidebar-session.test.ts`
- Create: `web/apps/sidebar/src/features/contact/contact-api.ts`
- Create: `web/apps/sidebar/src/features/contact/contact-page.tsx`
- Create: `web/apps/sidebar/src/features/contact/contact-page.test.tsx`
- Create: `web/apps/sidebar/src/routes/registry.tsx`

**Interfaces:**

- Consumes: `@mochat/mobile-foundation`
- Produces: `createSidebarRouter(runtime)` and explicit `sidebarRouteRegistry`
- Produces: `readSidebarSession`, `writeSidebarSession`, `clearSidebarSession`, `sidebarLoginHref`
- Produces: `loadContactSummary(request, externalUserId)`

- [ ] **Step 1: Write failing session tests**

Assert only `token` and `agentId` cookies are read; Dashboard localStorage is never accessed. Assert login href equals `/sidebar/agent/auth?agentId=<id>&target=<encoded-safe-target>`. Assert callback state success stores token/expiry then returns to safe target, while external target falls back to `/`.

- [ ] **Step 2: Verify session RED, implement, then verify GREEN**

```powershell
corepack pnpm --filter @mochat/sidebar test -- src/auth/sidebar-session.test.ts
```

Use a `CookieAdapter` dependency in tests. Cookie writes require `Path=/; SameSite=Lax`; add `Secure` only on HTTPS.

- [ ] **Step 3: Write failing route-registry tests**

Assert exact set equality between `migration-routes.json` and registry paths, every route has a unique module key and proper `auth`, unknown path displays “页面不存在”, and protected routes redirect without rendering feature content.

- [ ] **Step 4: Implement router and honest module boundary**

Register all 12 paths explicitly. `/login` and `/auth` are public. Protected modules render an explicit named module state when their business feature has not yet migrated; remove all fake buttons and fake success messages.

- [ ] **Step 5: Write failing contact feature tests**

For `/contact?wxExternalUserid=external-user-1&agentId=7`, assert request `GET /workContact/detail?wxExternalUserid=external-user-1`; render name/avatar; missing external ID shows parameter error and sends zero requests; 401 invokes Sidebar re-auth; network error shows retry and retry calls the API again.

- [ ] **Step 6: Implement contact API and page**

Use this stable model:

```ts
export type ContactSummary = {
  id: number;
  name: string;
  avatar: string | null;
  corpId: number;
};
```

Adapter accepts backend camelCase only; malformed success data becomes a validation error rather than rendering an empty fake contact.

- [ ] **Step 7: Fix Chinese source and app metadata**

Set the title to `MoChat 客户侧边栏`. Replace current mojibake catalog with valid Chinese route titles. Import shared CSS before app CSS. Do not add a second UI framework.

- [ ] **Step 8: Verify Sidebar gates and commit**

```powershell
corepack pnpm --filter @mochat/sidebar lint
corepack pnpm --filter @mochat/sidebar typecheck
corepack pnpm --filter @mochat/sidebar test
corepack pnpm --filter @mochat/sidebar build
git add web/apps/sidebar pnpm-lock.yaml
git commit -m "feat(sidebar): establish authenticated mobile shell"
```

## Task 3：Operation 会话、路由与任务宝纵切面

**Files:**

- Modify: `web/apps/operation/package.json`
- Modify: `web/apps/operation/index.html`
- Modify: `web/apps/operation/src/main.tsx`
- Modify: `web/apps/operation/src/styles.css`
- Delete: `web/apps/operation/src/app/operation-app.tsx`
- Delete: `web/apps/operation/src/app/operation-app.test.tsx`
- Replace: `web/apps/operation/src/app/catalog.ts`
- Replace: `web/apps/operation/src/app/catalog.test.ts`
- Create: `web/apps/operation/src/app/operation-router.tsx`
- Create: `web/apps/operation/src/app/operation-router.test.tsx`
- Create: `web/apps/operation/src/auth/operation-session.ts`
- Create: `web/apps/operation/src/auth/operation-session.test.ts`
- Create: `web/apps/operation/src/features/work-fission/work-fission-api.ts`
- Create: `web/apps/operation/src/features/work-fission/work-fission-page.tsx`
- Create: `web/apps/operation/src/features/work-fission/work-fission-page.test.tsx`
- Create: `web/apps/operation/src/routes/registry.tsx`

**Interfaces:**

- Consumes: `@mochat/mobile-foundation`
- Produces: `createOperationRouter(runtime)` and exact `operationRouteRegistry`
- Produces: `operationAuthHref(activityKind, target, params)`
- Produces: `loadWorkFissionProgress(request, { unionId, fissionId })`

- [ ] **Step 1: Write failing session/parameter tests**

Assert Operation never reads Sidebar cookies or Dashboard storage. `operationAuthHref('workFission', ...)` points to `/auth/workFission` with safe target and required activity ID. Missing `union_id` or positive `fission_id` blocks the feature request and shows a parameter error.

- [ ] **Step 2: Implement session boundary and verify GREEN**

Operation API client has no Bearer provider and always uses same-origin credentials. A 401 creates the matching activity OAuth URL without clearing any other app session.

- [ ] **Step 3: Write failing exact-route tests**

Assert exact set equality with 10 manifest paths, unique module keys, correct Chinese titles, unknown path 404, and preservation of query/hash across auth navigation.

- [ ] **Step 4: Implement router and remove fake activity behavior**

Delete the generic `execute()` that posts `/operation/<feature>` and remove random/fixed result rendering. Every route gets a named activity module boundary; `/workFission` uses the real feature below.

- [ ] **Step 5: Write failing task-progress tests**

Assert `GET /workFission/taskData?union_id=<id>&fission_id=<id>`, render invite/differ counts and task labels, empty task state, 401 OAuth action, invalid envelope, expired activity error and retry.

- [ ] **Step 6: Implement task-progress adapter and page**

Use stable types:

```ts
export type WorkFissionTask = {
  level: number;
  target: number;
  rewardLabel: string;
  received: boolean;
};
export type WorkFissionProgress = {
  inviteCount: number;
  differCount: number;
  endTime: number | null;
  tasks: WorkFissionTask[];
};
```

Normalize snake_case only at the feature API boundary. Do not invent missing counts or rewards.

- [ ] **Step 7: Fix Chinese source, verify gates and commit**

```powershell
corepack pnpm --filter @mochat/operation lint
corepack pnpm --filter @mochat/operation typecheck
corepack pnpm --filter @mochat/operation test
corepack pnpm --filter @mochat/operation build
git add web/apps/operation pnpm-lock.yaml
git commit -m "feat(operation): establish activity mobile shell"
```

## Task 4：Completion gate 与 22 路由浏览器验收

**Files:**

- Create: `scripts/check_mobile_clients_foundation.mjs`
- Create: `scripts/check_mobile_clients_foundation.test.mjs`
- Modify: `package.json`
- Create: `web/e2e/tests/mobile-clients-foundation.spec.ts`
- Modify: `web/e2e/package.json`
- Modify if needed: `cmd/mochat-frontend-e2e/main.go`

**Interfaces:**

- Produces: `pnpm check:mobile-clients-foundation`
- Produces: `pnpm --filter @mochat/e2e test:mobile-clients-foundation`

- [ ] **Step 1: Write structural gate RED fixtures**

The test creates temporary fixture trees and proves each defect fails independently:

```text
11 Sidebar routes
9 Operation routes
registry route absent from manifest
production page calls fetch directly
Dashboard storage key in either app
mojibake markers
fake success / random prize / fixed progress
missing 390 viewport browser case
unknown route falling back to home
```

- [ ] **Step 2: Implement source-derived gate**

Read manifests and production TS/TSX independently, parse registry path literals, scan only production sources, and verify package scripts. The success summary prints exact counts:

```text
sidebar routes=12
operation routes=10
direct fetch=0
dashboard session references=0
mojibake markers=0
fake business outcomes=0
mobile viewport cases>=22
```

- [ ] **Step 3: Write Playwright browser matrix**

For both `390×844` and `1280×900`:

- open all 12 `/sidebar-app/*` paths with deterministic session/request fixtures;
- open all 10 `/operation-app/*` paths with deterministic session/request fixtures;
- verify title/module label, no blank page, `scrollWidth <= clientWidth`, visible 44px primary/retry controls when applicable;
- verify unknown path has a 404 heading and does not render home content;
- collect console errors and unexpected request failures; both remain zero.

Network fixtures must return raw Go-style envelopes and must not precompute the final view model.

- [ ] **Step 4: Add package scripts and verify RED→GREEN**

Root:

```json
"check:mobile-clients-foundation": "node scripts/check_mobile_clients_foundation.mjs && node --test scripts/check_mobile_clients_foundation.test.mjs"
```

E2E:

```json
"test:mobile-clients-foundation": "playwright test tests/mobile-clients-foundation.spec.ts --workers=1"
```

- [ ] **Step 5: Run final non-Docker gates**

```powershell
corepack pnpm --filter @mochat/mobile-foundation lint
corepack pnpm --filter @mochat/mobile-foundation typecheck
corepack pnpm --filter @mochat/mobile-foundation test
corepack pnpm --filter @mochat/mobile-foundation build
corepack pnpm --filter @mochat/sidebar lint
corepack pnpm --filter @mochat/sidebar typecheck
corepack pnpm --filter @mochat/sidebar test
corepack pnpm --filter @mochat/sidebar build
corepack pnpm --filter @mochat/operation lint
corepack pnpm --filter @mochat/operation typecheck
corepack pnpm --filter @mochat/operation test
corepack pnpm --filter @mochat/operation build
corepack pnpm --filter @mochat/e2e typecheck
corepack pnpm --filter @mochat/e2e lint
corepack pnpm --filter @mochat/e2e test:mobile-clients-foundation
corepack pnpm check:mobile-clients-foundation
git diff --check main...HEAD
```

- [ ] **Step 6: Commit**

```powershell
git add package.json scripts/check_mobile_clients_foundation.mjs scripts/check_mobile_clients_foundation.test.mjs web/e2e cmd/mochat-frontend-e2e/main.go
git commit -m "test(mobile): gate both client foundations"
```

## 最终审阅清单

- 对照设计逐项检查共享边界、身份隔离、22 条 URL、真实纵切面、错误处理和 390px 约束。
- `rg` 确认两个应用无 `mochat_dashboard_`、直接 `fetch(`、乱码特征和伪业务结果。
- 审阅 `main...HEAD` 全量 diff；Critical/Important 问题修复后重新执行覆盖命令。
- 确认主工作区 dirty 文件未改变，工作树 clean，未执行 Docker/数据库/服务器操作。
