# 线索池与联系人页面体验优化实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在不虚构后端能力的前提下，完成线索池和联系人两个 Phase 3.2 页面的一致化、优雅化与可验收实现。

**Architecture:** 保留现有 React Query、URL 筛选状态和 API 合同，只重组页面组件层级、交互状态与作用域 CSS。联系人详情使用 URL 驱动的可访问抽屉，线索池沿用真实游标分页和状态机。

**Tech Stack:** React 19、TypeScript、React Query、React Router、Vitest、Testing Library、CSS。

## Global Constraints

- 仅修改线索池、联系人及其共享视觉；不修改商机、公海和标签页面行为。
- 不实现 API 不支持的渠道对接、导出、批量删除、批量转移或好友编辑。
- 所有新增行为先有失败测试，权限、URL、错误状态和游标分页必须保持真实。

---

### Task 1: 线索池页面

**Files:**
- Modify: `web/apps/dashboard/src/features/scrm/lead-page.test.tsx`
- Modify: `web/apps/dashboard/src/features/scrm/lead-page.tsx`
- Modify: `web/apps/dashboard/src/styles/index.css`

**Interfaces:**
- Consumes: `LeadApi.list/create/findDuplicates/assign/transition` 与 `LeadListInput`。
- Produces: URL 驱动筛选、刷新、真实摘要、批量分配和状态流转页面。

- [ ] **Step 1: 添加失败测试**：断言刷新按钮、当前筛选摘要、批量工具栏、保留筛选的游标分页与权限控制。
- [ ] **Step 2: 验证 RED**：运行 `pnpm vitest run src/features/scrm/lead-page.test.tsx`，确认因新结构或交互缺失失败。
- [ ] **Step 3: 实现线索池结构与作用域样式**：重组头部、筛选、创建区、表格和工具栏，不改变 API 请求合同。
- [ ] **Step 4: 验证 GREEN**：重复目标测试并运行 `pnpm typecheck`，预期全部通过。

### Task 2: 联系人列表与详情抽屉

**Files:**
- Modify: `web/apps/dashboard/src/features/scrm/contact-page.test.tsx`
- Modify: `web/apps/dashboard/src/features/scrm/contact-page.tsx`
- Modify: `web/apps/dashboard/src/styles/index.css`

**Interfaces:**
- Consumes: `ContactApi.listContacts/getContact/listTagCatalog/updateAssignment/maintainTagContacts/releaseToPublicPool/createOpportunity` 及 `FollowUpTimeline`。
- Produces: URL 驱动列表和可访问详情抽屉，保留全部既有生命周期操作。

- [ ] **Step 1: 添加失败测试**：断言刷新、摘要、详情 dialog、Escape 关闭、焦点恢复和详情错误重试。
- [ ] **Step 2: 验证 RED**：运行 `pnpm vitest run src/features/scrm/contact-page.test.tsx`，确认因抽屉语义或交互缺失失败。
- [ ] **Step 3: 实现列表及抽屉**：加入工具栏、详情遮罩、分区卡片、关闭/重试和键盘焦点逻辑，保留 URL 状态。
- [ ] **Step 4: 验证 GREEN**：重复目标测试并运行 `pnpm typecheck`，预期全部通过。

### Task 3: 回归、浏览器验收与部署

**Files:**
- Modify: `docs/phase/phase-3.2-dashboard-completion/reports/tasks/task-6-report.md`
- Modify: `docs/phase/phase-3.2-dashboard-completion/reports/tasks/task-7-report.md`

- [ ] **Step 1: 运行全量验证**：执行 Dashboard 全量 Vitest、typecheck、build 和 `git diff --check`。
- [ ] **Step 2: 浏览器对照**：在圆弧与本地分别检查线索池和联系人，验证菜单选中、空/错误状态、筛选、工具栏和详情抽屉。
- [ ] **Step 3: 独立代码审查**：检查权限、URL、错误重试、CSS 作用域和不存在的伪功能。
- [ ] **Step 4: 部署**：使用 `mochat-go-desktop` Compose 项目重建 app，保留既有数据卷并确认 `/readyz` 为 200。
- [ ] **Step 5: 提交**：提交页面、测试、样式和 Phase 3.2 文档，保持当前工作分支。

