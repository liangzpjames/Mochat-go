# 全局消息页面优化实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在不伪造数据的前提下，使 `/chat/v2-all` 的功能层级和视觉体验与圆弧全局消息页面对应。

**Architecture:** 继续使用 `ConversationGlobalApi` 的真实列表与详情合同。页面组件负责 URL 筛选状态、类型快捷筛选、刷新、摘要和详情交互，现有全局样式表负责响应式视觉统一。

**Tech Stack:** React、React Router、TanStack Query、Vitest、Testing Library、CSS。

## Global Constraints

- 仅修改 `/chat/v2-all` 及其直接样式和测试。
- 不增加虚构统计，不改动其他会话页面。
- 新行为必须先有失败测试，再实现并通过生产构建和浏览器验收。

---

### Task 1：补齐页面交互合同

**Files:**
- Test: `web/apps/dashboard/src/features/conversation-global/conversation-global-page.test.tsx`
- Modify: `web/apps/dashboard/src/features/conversation-global/conversation-global-page.tsx`

**Interfaces:**
- Consumes: `ConversationGlobalApi.search(input)`、URL `conversationType` 与分页参数。
- Produces: 类型快捷筛选、真实结果摘要、刷新按钮和既有详情入口。

- [ ] 写入快捷类型筛选、结果摘要和刷新行为的失败测试。
- [ ] 运行单文件测试，确认因控件缺失而失败。
- [ ] 实现最小页面行为并保持 URL 为唯一筛选状态来源。
- [ ] 重跑单文件测试，确认全部通过。

### Task 2：完成视觉层级与状态统一

**Files:**
- Modify: `web/apps/dashboard/src/styles/index.css`
- Test: `web/apps/dashboard/src/features/conversation-global/conversation-global-page.test.tsx`

**Interfaces:**
- Consumes: Task 1 新增的语义类名和现有 `dashboard-*` 共享样式。
- Produces: 摘要区、类型标签、工具栏、结果卡片、详情抽屉及响应式布局。

- [ ] 添加页面结构类名存在性的失败断言。
- [ ] 运行测试并确认断言失败。
- [ ] 增加对应 CSS，避免改变其他七个 phase3.2 页面。
- [ ] 运行 Dashboard 测试、类型检查和生产构建。

### Task 3：部署并浏览器验收

**Files:**
- Modify: `docs/phases/phase-3-dashboard/phase-3.2/reports/tasks/task-4-report.md`

**Interfaces:**
- Consumes: 通过构建的 Dashboard 产物。
- Produces: 保留现有数据卷的 `mochat-go-desktop` 容器及本轮验收证据。

- [ ] 使用现有 Compose 项目构建并替换应用容器，不删除命名数据卷。
- [ ] 验证容器健康和 `/readyz` 返回 200。
- [ ] 对照圆弧与本地页面检查菜单选中、筛选、未授权状态和响应式布局。
- [ ] 更新任务报告并提交当前 phase3.2 分支。
