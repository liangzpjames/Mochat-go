# Phase 2 当前可达路由证据实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在不恢复已退役 Dashboard 深链、不降低权限要求的前提下，为当前可达的 Phase 2 路由生成真实 Playwright 桌面/移动证据，并让门禁与现行路由合同保持一致。

**Architecture:** 新增一个无状态 ESM 模块作为“当前可达 Phase 2 路由”的唯一派生入口：Dashboard 取历史迁移清单与当前 benchmark manifest 的交集，Sidebar 和 Operation 使用各自现行 manifest。Playwright、证据生成器和审计脚本复用该入口；旧 Dashboard 截图记录保留但标记为历史，不能引用本次运行。

**Tech Stack:** Node.js 22、ESM、Playwright 1.62、TypeScript 5.9、React Router、pnpm 11。

## Global Constraints

- 不恢复或放宽非 benchmark legacy Dashboard 深链；它们继续返回 403。
- 不手填证据 JSON，不伪造截图、测试次数或 Playwright 成功记录。
- 证据必须来自完整现行套件，而不是 `--grep` 单用例。
- 只使用仓库 `seedSession`、`mockDashboardBackend` 与 `cmd/mochat-frontend-e2e` 正式本地 fixture/webServer。
- 不触碰 Docker、数据库、主工作树或候选脏工作树。

---

### Task 1: 统一当前可达路由合同

**Files:**
- Create: `scripts/phase2_current_routes.mjs`
- Create: `scripts/phase2_current_routes.d.mts`
- Modify: `scripts/audit_phase2_frontend_completion.test.mjs`
- Modify: `web/apps/dashboard/src/app/access-loader.test.ts`

**Interfaces:**
- Produces: `currentPhase2Routes(root): { dashboard, sidebar, operation }` 和 `currentPhase2RouteCount(routes)`。
- Consumes: 三端 `migration-routes.json` 与 Dashboard `benchmark/manifest.json`。

- [ ] **Step 1: 写失败测试**

  在审计测试中断言 Dashboard 当前集合包含 `/company-setting/website`，排除 `/contactField/index`；在 access loader 测试中构造服务端显式授予 `/contactField/index` 的 profile，仍应因不在 manifest 返回 403。

- [ ] **Step 2: 运行 RED**

  Run: `node --test --test-name-pattern "current reachable Phase 2 routes" scripts/audit_phase2_frontend_completion.test.mjs`

  Expected: FAIL，因为共享派生模块尚未接入或当前证据未区分 current/historical。

- [ ] **Step 3: 最小实现共享模块**

  `currentPhase2Routes` 读取三端 manifest，并以 Dashboard benchmark path set 过滤 Dashboard 历史迁移清单；Sidebar、Operation 原样返回。

- [ ] **Step 4: 运行共享合同与安全测试**

  Run: `node --test scripts/audit_phase2_frontend_completion.test.mjs`

  Run: `corepack pnpm --filter @mochat/dashboard test -- --run src/app/access-loader.test.ts`

  Expected: 路由派生断言通过；显式授予的非 manifest legacy 深链仍返回 403。

### Task 2: 让 Visual Suite、生成器和审计复用合同

**Files:**
- Modify: `web/e2e/tests/dashboard-migration.spec.ts`
- Modify: `scripts/generate_phase2_evidence.mjs`
- Modify: `scripts/audit_phase2_frontend_completion.mjs`
- Modify: `scripts/audit_phase2_frontend_completion.test.mjs`

**Interfaces:**
- Consumes: `currentPhase2Routes`、`currentPhase2RouteCount`。
- Produces: 动态 `expectedTests = current route count + 2`；历史 evidence 行 `currentReachable=false` 且不引用当前 `playwright-run.json`。

- [ ] **Step 1: 使用现有失败 E2E 作为 RED**

  已观察：旧套件访问 `/autoTag/dayPartCreate`，当前安全合同返回 403，`.dashboard-content` 不存在。

- [ ] **Step 2: 修改三处消费者**

  Playwright 只遍历当前集合；生成器先将旧记录标为历史，再覆盖本次现行记录；审计只要求现行集合并拒绝陈旧测试数或历史行冒充当前行。

- [ ] **Step 3: 类型和静态测试**

  Run: `corepack pnpm --filter @mochat/e2e typecheck`

  Run: `node --test scripts/audit_phase2_frontend_completion.test.mjs`

  Expected: PASS。

### Task 3: 真实生成、验证与原子提交

**Files:**
- Modify: `docs/phases/phase-2-frontend-migration/evidence/playwright-run.json`
- Modify: `docs/phases/phase-2-frontend-migration/evidence/route-evidence.json`
- Modify: `docs/phases/phase-2-frontend-migration/evidence/rollback-records.json`
- Modify: `docs/phases/phase-2-frontend-migration/evidence/screenshots/*.png`
- Create: `docs/phases/phase-2-frontend-migration/evidence/screenshots/dashboard--company-setting__website--desktop.png`
- Create: `docs/phases/phase-2-frontend-migration/evidence/screenshots/dashboard--company-setting__website--mobile.png`
- Create: `docs/reviews/2026-08-30-phase2-current-route-evidence.zh-CN.md`

**Interfaces:**
- Consumes: 仓库 Playwright webServer 与 fixture。
- Produces: 可审计 SHA-256、通过记录、中文合同迁移说明和提交 SHA。

- [ ] **Step 1: 完整现行套件**

  Run: `corepack pnpm --filter @mochat/e2e exec playwright test tests/dashboard-migration.spec.ts --workers=1`

  Expected: 全部现行测试通过，`.last-run.json` 为 passed，4174 服务在结束后退出。

- [ ] **Step 2: 正式生成证据**

  Run: `corepack pnpm evidence:phase2`

  Expected: 当前路由拥有桌面/移动非空 PNG 与 SHA-256；历史路由不引用当前运行。

- [ ] **Step 3: 全部门禁**

  Run: `corepack pnpm check:audit`

  Run: `corepack pnpm check:phase2-progress`

  Run: `corepack pnpm check:phase2.1`

  Run: `node --test scripts/audit_phase2_frontend_completion.test.mjs scripts/check_phase2_frontend_progress.test.mjs scripts/check_phase2_1_frontend_completion.test.mjs`

  Run: `git diff --check`

  Expected: 全部退出码 0。

- [ ] **Step 4: 原子提交**

  仅暂存本计划涉及的源代码、测试、报告与证据文件，核对 staged diff 后提交 `fix(gates): align Phase 2 evidence with current routes`。
