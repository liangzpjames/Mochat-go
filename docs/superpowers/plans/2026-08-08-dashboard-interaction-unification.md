# Dashboard 交互与视觉统一 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在保留 53 个 Dashboard 页面真实业务、URL、权限和数据的前提下，统一页面风格与交互，并修复已确认的数据安全和响应式问题。

**Architecture:** 先建立共享页面壳、筛选、状态、弹层和危险操作组件，再按页面族迁移。业务 API 与 React Query 调用保持在各 feature 内，公共组件只处理展示、校验和交互协议。

**Tech Stack:** React 19.2、TypeScript 5.9、Ant Design 6、TanStack Query 5、React Router 7、Vitest、Testing Library、Playwright、Docker Compose。

## Global Constraints

- 用户审阅文档使用中文。
- manifest 53 个路由全部纳入验收，不以可达路由或空态代替业务完成。
- 不删除、不重建 MySQL、Redis、app-storage、audit-anchor-storage 数据卷。
- 不执行 `docker compose down -v`、`docker volume rm` 或 `docker volume prune`。
- 只修改本计划涉及的 Dashboard 前端、测试和验收文档；保留工作树其他用户改动。
- 每个实现任务先写失败测试，再写最小实现，再运行局部和 Dashboard 全量测试。

---

### Task 1: 建立统一页面交互组件

**Files:**
- Create: `web/apps/dashboard/src/components/dashboard-page-shell.tsx`
- Create: `web/apps/dashboard/src/components/dashboard-filter-panel.tsx`
- Create: `web/apps/dashboard/src/components/dashboard-data-state.tsx`
- Create: `web/apps/dashboard/src/components/dashboard-dialog.tsx`
- Create: `web/apps/dashboard/src/components/date-range-fields.tsx`
- Create: `web/apps/dashboard/src/components/row-action.tsx`
- Modify: `web/apps/dashboard/src/components/confirm-action.tsx`
- Test: `web/apps/dashboard/src/components/dashboard-interactions.test.tsx`

**Interfaces:**
- Produces: `DashboardPageShell`, `DashboardFilterPanel`, `DashboardDataState`, `DashboardDialog`, `DateRangeFields`, `RowAction`, `ConfirmAction`。
- `ConfirmAction` 的子节点只能负责打开确认层，写操作只能从 `onConfirm` 触发。

- [ ] **Step 1: 写共享组件失败测试**

```tsx
it('does not run a destructive action before confirmation', async () => {
  const onConfirm = vi.fn();
  render(<ConfirmAction title="确认停用？" onConfirm={onConfirm}><button>停用</button></ConfirmAction>);
  await user.click(screen.getByRole('button', { name: '停用' }));
  expect(onConfirm).not.toHaveBeenCalled();
  await user.click(screen.getByRole('button', { name: '确认' }));
  expect(onConfirm).toHaveBeenCalledTimes(1);
});

it('rejects an inverted date range without submitting', async () => {
  const onSubmit = vi.fn();
  render(<DateRangeFields value={{ startDate: '2026-09-01', endDate: '2026-08-01' }} onChange={() => undefined} onValidSubmit={onSubmit} />);
  await user.click(screen.getByRole('button', { name: '查询' }));
  expect(screen.getByRole('alert')).toHaveTextContent('开始日期不能晚于结束日期');
  expect(onSubmit).not.toHaveBeenCalled();
});
```

- [ ] **Step 2: 运行测试确认失败**

Run: `pnpm --filter @mochat/dashboard test -- dashboard-interactions.test.tsx`

Expected: FAIL，提示共享组件尚不存在。

- [ ] **Step 3: 实现公共接口和统一样式类名**

```ts
export type DateRangeValue = { startDate: string; endDate: string };
export function validateDateRange(value: DateRangeValue): string | null {
  return value.startDate && value.endDate && value.startDate > value.endDate
    ? '开始日期不能晚于结束日期'
    : null;
}
```

- [ ] **Step 4: 运行组件测试、类型检查和 lint**

Run: `pnpm --filter @mochat/dashboard test -- dashboard-interactions.test.tsx`

Expected: PASS。

Run: `pnpm --filter @mochat/dashboard typecheck && pnpm --filter @mochat/dashboard lint`

Expected: 两项均退出码 0。

### Task 2: 统一全局框架、搜索和企业切换

**Files:**
- Modify: `web/apps/dashboard/src/layout/dashboard-layout.tsx`
- Modify: `web/apps/dashboard/src/layout/dashboard-layout.test.tsx`
- Modify: `web/apps/dashboard/src/features/corp/corp-provider.tsx`
- Modify: `web/apps/dashboard/src/features/corp/corp-provider.test.tsx`
- Modify: `web/apps/dashboard/src/styles/index.css`

**Interfaces:**
- Consumes: `DashboardDialog`。
- Produces: 可键盘操作的功能搜索、真实任务入口策略、单企业隐藏/多企业下拉切换。

- [ ] **Step 1: 写搜索、死链接和企业切换失败测试**

```tsx
it('filters navigation and navigates from the search box', async () => {
  renderLayout('/index');
  await user.type(screen.getByRole('searchbox', { name: '搜索功能' }), '标签');
  expect(screen.getByRole('link', { name: '客户标签' })).toHaveAttribute('href', '/customer/tags');
});

it('does not render a disabled duplicate corp switcher for a single corp', () => {
  renderCorpProvider({ corps: [authorizedCorp] });
  expect(screen.queryByLabelText('企业选择')).not.toBeInTheDocument();
});
```

- [ ] **Step 2: 运行局部测试确认失败**

Run: `pnpm --filter @mochat/dashboard test -- dashboard-layout.test.tsx corp-provider.test.tsx`

Expected: FAIL，搜索不产生结果且单企业切换条仍存在。

- [ ] **Step 3: 实现搜索结果浮层和企业切换规则**

搜索源只使用当前用户 `allowedRoutes`；移除 `href="#tasks"`。如果没有真实任务路由，则不渲染“任务中心”。

- [ ] **Step 4: 运行局部测试与 Dashboard 测试**

Run: `pnpm --filter @mochat/dashboard test`

Expected: PASS。

### Task 3: 修复响应式框架并统一视觉 token

**Files:**
- Modify: `web/apps/dashboard/src/styles/index.css`
- Modify: `web/apps/dashboard/src/styles/dashboard-scroll-layout.test.ts`
- Test: `web/e2e/tests/dashboard-responsive.spec.ts`

**Interfaces:**
- Produces: `>1080px`、`769–1080px`、`<=768px` 三档布局；移动端抽屉侧栏。

- [ ] **Step 1: 写 CSS 顺序和窄屏失败测试**

```ts
test('390px viewport has no page-level horizontal overflow', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/customer/order');
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
});
```

- [ ] **Step 2: 运行响应式测试确认失败**

Run: `pnpm --filter @mochat/e2e exec playwright test tests/dashboard-responsive.spec.ts --project=chromium`

Expected: FAIL，订单等页面根宽度超过视口。

- [ ] **Step 3: 合并重复 `.dashboard-body` 规则并实现移动端侧栏**

桌面规则必须位于移动端规则之前；移动端显式设置单列内容和抽屉导航。宽表使用 `.dashboard-table-scroll { overflow-x: auto; }`，不得把宽度传递给页面根。

- [ ] **Step 4: 验证 29 个已知溢出页面**

Run: `pnpm --filter @mochat/e2e exec playwright test tests/dashboard-responsive.spec.ts --project=chromium`

Expected: 29 个回归用例全部 PASS。

### Task 4: 修复危险操作和企业密钥

**Files:**
- Modify: `web/apps/dashboard/src/features/phase33/risk-behavior-page.tsx`
- Modify: `web/apps/dashboard/src/features/phase33/timeout-warning-page.tsx`
- Modify: `web/apps/dashboard/src/features/company-settings/website-page.tsx`
- Modify: `web/apps/dashboard/src/features/company-settings/staff-page.tsx`
- Modify: `web/apps/dashboard/src/features/company-settings/additional-page.tsx`
- Modify: `web/apps/dashboard/src/features/company-settings/authorization-page.tsx`
- Modify: `web/apps/dashboard/src/features/ai-settings/knowledge-base-page.tsx`
- Modify: `web/apps/dashboard/src/features/ai-settings/agent-page.tsx`
- Test: corresponding `*.test.tsx` files

**Interfaces:**
- Consumes: `ConfirmAction`, `RowAction`, `DashboardDialog`。
- Produces: 取消零请求、确认单请求、密钥不回填协议。

- [ ] **Step 1: 为每类危险操作写请求次数测试**

```tsx
expect(api.updateStatus).not.toHaveBeenCalled();
await user.click(screen.getByRole('button', { name: '确认' }));
expect(api.updateStatus).toHaveBeenCalledTimes(1);
```

- [ ] **Step 2: 为企业密钥写不回显测试**

```tsx
expect(screen.getByLabelText('员工密钥')).toHaveValue('');
expect(screen.getByLabelText('员工密钥')).toHaveAttribute('type', 'password');
expect(screen.getByText('留空表示不修改')).toBeVisible();
```

- [ ] **Step 3: 移除内部按钮 mutation 和所有 `window.confirm`**

`ConfirmAction` 内部按钮不得包含 `onClick={() => mutation.mutate(...)}`；授权按钮名称改为 `停用 ${node.name}`。

- [ ] **Step 4: 运行相关页面测试**

Run: `pnpm --filter @mochat/dashboard test -- risk-behavior-page timeout-warning-page company-settings-pages ai-settings-pages`

Expected: PASS。

### Task 5: 迁移会话、风险预警和营销工具页面

**Files:**
- Modify: `web/apps/dashboard/src/features/conversation-global/*.tsx`
- Modify: `web/apps/dashboard/src/features/phase33/*.tsx`
- Modify: `web/apps/dashboard/src/features/sensitive-word/sensitive-word-page.tsx`
- Modify: `web/apps/dashboard/src/features/phase34/*.tsx`
- Modify: `web/apps/dashboard/src/features/phase34/material-management/material-management-page.tsx`

**Interfaces:**
- Consumes: Task 1 全部共享组件。
- Produces: 统一 Provider 状态、日期筛选、页签和页面主操作。

- [ ] **Step 1: 写 Provider 状态一致性测试**

```tsx
expect(screen.getByRole('status')).toHaveTextContent('会话归档未开通');
expect(screen.getByRole('link', { name: '去配置会话归档' })).toHaveAttribute('href', '/company-setting/website');
```

- [ ] **Step 2: 修复员工会话把 Provider 缺失显示为“加载失败”**

同一 Provider 错误码映射到同一 `DashboardDataState`，仅真实网络/服务错误显示“加载失败”。

- [ ] **Step 3: 补齐渠道活码和群活码页面主操作**

只有后端写能力存在时显示“新建渠道活码”“新建群活码”；没有写能力时显示明确只读说明，不伪造保存。

- [ ] **Step 4: 统一营销页签语义**

页签容器使用 `role="tablist"`，页签设置 `aria-controls`，内容设置 `role="tabpanel"` 和对应 `id`。

- [ ] **Step 5: 运行页面族测试**

Run: `pnpm --filter @mochat/dashboard test -- conversation phase33 sensitive-word phase34`

Expected: PASS。

### Task 6: 迁移 SCRM 和订单工作台

**Files:**
- Modify: `web/apps/dashboard/src/features/scrm/*.tsx`
- Modify: `web/apps/dashboard/src/features/phase35/friends-page.tsx`
- Modify: `web/apps/dashboard/src/features/phase35/group-page.tsx`
- Modify: `web/apps/dashboard/src/features/phase35/order-page.tsx`
- Modify: `web/apps/dashboard/src/features/phase35/settings-page.tsx`
- Test: corresponding SCRM and phase35 tests

**Interfaces:**
- Consumes: Task 1 共享组件。
- Produces: 业务选择器、分步订单创建、可取消设置编辑。

- [ ] **Step 1: 写员工筛选和编辑状态隔离测试**

```tsx
await user.type(screen.getByLabelText('手机号筛选'), '138');
await user.click(screen.getAllByRole('button', { name: /编辑/ })[0]);
await user.click(screen.getByRole('button', { name: '取消' }));
expect(screen.getByLabelText('手机号筛选')).toHaveValue('138');
```

- [ ] **Step 2: 重排订单创建流程**

订单卡片固定为联系人选择、订单信息、提交三段；“快速创建联系人”改为独立 Drawer，创建成功后回填联系人并关闭 Drawer。

- [ ] **Step 3: 为客户设置增加取消编辑**

点击编辑后显示“保存修改”“取消编辑”；取消恢复新增模式且不改变筛选或列表。

- [ ] **Step 4: 用业务选择器替代内部 ID**

线索负责人、公海联系人、商机阶段、标签绑定均从现有 API 列表选择；版本号由选中行携带，不渲染为用户输入。

- [ ] **Step 5: 运行 SCRM 与 phase35 测试**

Run: `pnpm --filter @mochat/dashboard test -- scrm phase35`

Expected: PASS。

### Task 7: 迁移报表、AI 设置和企业设置

**Files:**
- Modify: `web/apps/dashboard/src/features/dashboard-overview/dashboard-overview-page.tsx`
- Modify: `web/apps/dashboard/src/features/phase35/*report-page.tsx`
- Modify: `web/apps/dashboard/src/features/ai-insight/ai-insight-pages.tsx`
- Modify: `web/apps/dashboard/src/features/ai-settings/*.tsx`
- Modify: `web/apps/dashboard/src/features/company-settings/*.tsx`

**Interfaces:**
- Consumes: Task 1–4 的共享组件和安全规则。
- Produces: 统一报表筛选、真正 Modal/Drawer、中文权限名称和清晰树形授权。

- [ ] **Step 1: 报表统一使用 `DateRangeFields` 和员工/部门选择器**

`/index` 与五个报表页共享日期校验；部门不再要求输入 ID。

- [ ] **Step 2: 把 AI/企业设置伪弹层迁移为 `DashboardDialog`**

打开、Esc、取消、保存成功后的焦点行为必须通过组件测试。

- [ ] **Step 3: 修复附加权限与授权树文案**

移除 `Friends circle ...` 和 `??? Provider ??` 的直接展示；优先使用后端中文名称映射，无法识别时显示“未命名权限（ID）”。路径列显示真实 `menuPath`，不显示序号。

- [ ] **Step 4: 运行报表、AI 和企业设置测试**

Run: `pnpm --filter @mochat/dashboard test -- dashboard-overview report ai-settings company-settings`

Expected: PASS。

### Task 8: 53 页全量浏览器验收与安全部署

**Files:**
- Create: `web/e2e/tests/dashboard-interaction-unification.spec.ts`
- Modify: `web/apps/dashboard/src/benchmark/manifest-status.test.ts`
- Create: `docs/phases/phase-3-dashboard/phase-3-final/2026-08-08-dashboard-interaction-unification-acceptance.zh-CN.md`

**Interfaces:**
- Consumes: manifest 53 页和 Tasks 1–7 的全部结果。
- Produces: 可重复的桌面/窄屏验收证据。

- [ ] **Step 1: 从 manifest 参数化生成 53 页用例**

```ts
for (const route of dashboardRoutes) {
  test(`${route.path} desktop and mobile`, async ({ page }) => {
    await page.goto(route.path);
    await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(await page.evaluate(() => document.documentElement.clientWidth));
  });
}
```

- [ ] **Step 2: 运行前端质量门**

Run: `pnpm --filter @mochat/dashboard test`

Run: `pnpm --filter @mochat/dashboard lint`

Run: `pnpm --filter @mochat/dashboard typecheck`

Run: `pnpm --filter @mochat/dashboard build`

Expected: 四项全部退出码 0。

- [ ] **Step 3: 记录部署前卷名与容器健康**

Run: `docker ps --format 'table {{.Names}}\t{{.Status}}'`

Run: `docker volume ls --format '{{.Name}}' | Select-String 'mochat-go-desktop_(mysql-data|redis-data|app-storage|audit-anchor-storage)'`

Expected: 三个容器 healthy，四个卷名全部存在。

- [ ] **Step 4: 只重建并更新 app**

Run: `docker compose -p mochat-go-desktop -f deploy/standalone/docker-compose.yml build app`

Run: `docker compose -p mochat-go-desktop -f deploy/standalone/docker-compose.yml up -d --no-deps app`

Expected: 只替换 app 容器；MySQL、Redis 容器 ID 与四个卷名保持不变。

- [ ] **Step 5: 执行真实服务浏览器验收**

Run: `pnpm --filter @mochat/e2e exec playwright test tests/dashboard-interaction-unification.spec.ts --project=chromium`

Expected: 53 页桌面和窄屏用例全部 PASS。

- [ ] **Step 6: 完成中文验收报告**

报告必须记录命令、结果、容器健康、卷名、53 页通过数、未执行的真实破坏性操作和浏览器截图路径。
