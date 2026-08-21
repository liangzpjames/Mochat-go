# 数据概览圆弧 AI 风格重构实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将数据概览重组为圆弧 AI 风格的紧凑控件页面，同时严格区分真实零值、缺失数据和 Provider 限制。

**Architecture:** 保留现有 `/reports/overview` 请求和查询参数，在前端解析层保留 `null` 缺失语义；页面由数据状态提示、经营快照、AI 洞察控件组、趋势图、会话面板和能力告警面板组成。没有真实字段的模块只渲染告警壳，不拼接其他来源或模拟数字。

**Tech Stack:** React 18、TypeScript、Vitest、Testing Library、Vite、Playwright、Docker Compose。

## Global Constraints

- 所有用户可审阅文档使用中文；代码标识符、接口字段和命令保留原文。
- 概览数值只能来自 MoChat 自身接口；缺失/null 显示 `--` 和告警，接口明确返回数字 0 才显示 0。
- 不删除或重置既有 MySQL、Redis、app storage、audit-anchor storage 数据卷。
- 不覆盖工作区已有未提交的迁移、进度文档、脚本和 `web/saas-admin/` 变更。
- 每个生产代码改动先写失败测试，再写最小实现。

---

### Task 1: Preserve real-data and limitation semantics

**Files:**
- Modify: `web/apps/dashboard/src/features/dashboard-overview/dashboard-overview-api.ts`
- Test: `web/apps/dashboard/src/features/dashboard-overview/dashboard-overview-api.test.ts`
- Modify: `web/apps/dashboard/src/features/dashboard-overview/dashboard-overview-page.test.tsx`

**Interfaces:**
- `DashboardOverviewSummary` and `ConversationGroupStats` expose `number | null` for values that may be absent.
- `OverviewDataNotice` consumes `DashboardOverviewLimitation[]` and module availability state from later tasks.

- [x] **Step 1: Write failing parser tests**

Add cases to `dashboard-overview-api.test.ts` that feed `summary: { customer: null }` and a conversation group with a missing metric, then assert the parsed value is `null`, while a source value of `0` remains `0`.

- [x] **Step 2: Run the parser tests and verify the failure**

Run:

```powershell
corepack pnpm --filter @mochat/dashboard exec vitest run src/features/dashboard-overview/dashboard-overview-api.test.ts
```

Expected: the new null-preservation assertions fail because the current parser converts missing values to `0`.

- [x] **Step 3: Implement null-preserving parsing**

Introduce a parser helper with the exact behavior:

```ts
function nullableNumber(value: unknown): number | null {
  return isFiniteNumber(value) ? value : null;
}
```

Use it for summary fields and conversation group/trend metrics. Do not alter `limitations` or request serialization.

- [x] **Step 4: Run the parser and page tests**

Run:

```powershell
corepack pnpm --filter @mochat/dashboard exec vitest run src/features/dashboard-overview/dashboard-overview-api.test.ts src/features/dashboard-overview/dashboard-overview-page.test.tsx
```

Expected: all tests pass and the page tests prove that numeric zero is still rendered as `0`.

### Task 2: Recompose overview widgets around compact cards

**Files:**
- Modify: `web/apps/dashboard/src/features/dashboard-overview/dashboard-overview-widgets.tsx`
- Modify: `web/apps/dashboard/src/features/dashboard-overview/dashboard-overview-page.tsx`
- Test: `web/apps/dashboard/src/features/dashboard-overview/dashboard-overview-widgets.test.tsx`
- Test: `web/apps/dashboard/src/features/dashboard-overview/dashboard-overview-page.test.tsx`

**Interfaces:**
- Create `OverviewDataNotice({ kind, title, description, limitations })` for `limited`, `unavailable`, and `empty` states.
- Create `OverviewAIInsightGrid({ insight })` that only renders parsed text from the real `aiInsight` response.
- Update `OverviewMetricCard` to accept `number | null` and show `--` for null.
- Keep `OverviewConversationWorkspace({ conversation, unavailable })` as the public conversation entry point, but replace its layout with left compact controls and right chart.

- [x] **Step 1: Write failing rendering tests**

Add tests asserting:

```tsx
expect(screen.getByRole('status', { name: /数据暂缺|能力未接入/ })).toBeTruthy();
expect(screen.getByText('--')).toBeTruthy();
expect(screen.getByRole('heading', { name: 'AI 洞察' })).toBeTruthy();
expect(screen.queryByText(/根据提供的20条企业微信/)).toBeNull();
```

Also assert that a source value of `0` remains visible and that the conversation cards switch the chart without rendering the removed detail table.

- [x] **Step 2: Run widget/page tests and verify the failure**

Run:

```powershell
corepack pnpm --filter @mochat/dashboard exec vitest run src/features/dashboard-overview/dashboard-overview-widgets.test.tsx src/features/dashboard-overview/dashboard-overview-page.test.tsx
```

Expected: the new compact-control and missing-data assertions fail against the current large-text layout.

- [x] **Step 3: Implement the widget composition**

Use the existing real fields only:

- Snapshot cards read `data.summary`.
- Growth chart reads `data.trend`.
- Conversation cards/chart read `data.conversation`.
- AI cards read `data.aiInsight.capability`, `generatedAt`, and parsed summary lines.
- Quality, employee ranking, and trajectory panels render `OverviewDataNotice` with the exact missing data source requirement; no numeric fallback is allowed.

Remove long explanatory paragraphs and both overview detail tables from the primary viewport. Keep the existing CSV export and “查看 AI 洞察” route as detailed drill-downs; do not remove the API query fields needed by export compatibility until the page tests confirm the new URL contract.

- [x] **Step 4: Run widget/page tests and verify the pass**

Run the same Vitest command from Step 2. Expected: all new and existing widget/page tests pass.

### Task 3: Apply the Yuanhu visual system and responsive layout

**Files:**
- Modify: `web/apps/dashboard/src/styles/index.css`
- Test: `web/apps/dashboard/src/styles/dashboard-overview-layout.test.ts`
- Test: `web/e2e/tests/dashboard-overview-visual.spec.ts`

**Interfaces:**
- CSS selectors remain scoped to `.dashboard-overview-page` and the new overview component class names.
- The existing pagination hover/readability overrides remain active for all page-number variants.

- [x] **Step 1: Write failing layout assertions**

Add source assertions for compact snapshot cards, AI insight grid, conversation two-column layout, warning panel styling, full-date chart labels, mobile stacking, and reduced-motion behavior.

- [x] **Step 2: Run the layout and visual tests and verify the failure**

Run:

```powershell
corepack pnpm --filter @mochat/dashboard exec vitest run src/styles/dashboard-overview-layout.test.ts
corepack pnpm --filter @mochat/e2e exec playwright test tests/dashboard-overview-visual.spec.ts --workers=1
```

Expected: the new selectors/visual expectations fail before the CSS and DOM composition are implemented.

- [x] **Step 3: Implement the styles**

Use white cards, light borders, compact spacing, blue active states, restrained copy, chart grid lines, visible full dates, `overflow-x: auto` for long ranges, and a single-column layout below the existing overview breakpoint. Add `prefers-reduced-motion: reduce` rules for all overview transitions.

- [x] **Step 4: Run layout and visual tests**

Run the same Vitest and Playwright commands. Expected: desktop and 390px visual tests pass with no horizontal page overflow.

### Task 4: Full verification and local service refresh

**Files:**
- Verify: changed files under `web/apps/dashboard/src/`
- Verify: `deploy/standalone/docker-compose.yml`

- [x] **Step 1: Run the dashboard test slice**

```powershell
corepack pnpm --filter @mochat/dashboard exec vitest run src/features/dashboard-overview src/styles/dashboard-overview-layout.test.ts src/components/dashboard-pagination-contract.test.ts
```

Expected: zero failed tests.

- [x] **Step 2: Run typecheck and production build**

```powershell
corepack pnpm --filter @mochat/dashboard typecheck
corepack pnpm --filter @mochat/dashboard build
```

Expected: both commands exit with code 0.

- [x] **Step 3: Rebuild only the app container**

Use the existing Docker Desktop secrets and run:

```powershell
docker compose --project-name mochat-go-desktop -f deploy/standalone/docker-compose.yml up -d --build --force-recreate app
```

Do not run `down --volumes`. MySQL, Redis and all four existing volumes must remain intact.

- [x] **Step 4: Verify services and data volumes**

```powershell
Invoke-WebRequest -UseBasicParsing http://127.0.0.1:18080/readyz
docker ps --filter name=mochat-go-desktop
docker volume ls --filter name=mochat-go-desktop
git diff --check
```

Expected: `/readyz` returns HTTP 200; app, MySQL and Redis are healthy; the four existing volumes remain listed; `git diff --check` passes.
