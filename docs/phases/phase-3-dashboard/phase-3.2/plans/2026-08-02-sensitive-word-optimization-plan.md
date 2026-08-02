# 敏感词页面优化实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 优化 `/ai-insight/v2/sensitive-word` 的菜单、真实命中记录和敏感词配置体验。

**Architecture:** 继续使用 `SensitiveWordApi` 作为唯一数据来源。页面组件管理记录筛选、分页、刷新、分区和详情状态，CSS 使用 `sensitive-word-*` 作用域实现视觉统一。

**Tech Stack:** React、TanStack Query、Vitest、Testing Library、CSS。

## Global Constraints

- 仅处理敏感词页面及其直接测试、样式和归档文档。
- 不实现后端不存在的风险等级、AI 摘要、批量审计或导出。
- 所有新增行为先写失败测试，再实现并部署到保留数据卷的现有 Compose 项目。

---

### Task 1：命中记录交互

**Files:**
- Test: `web/apps/dashboard/src/features/sensitive-word/sensitive-word-page.test.tsx`
- Modify: `web/apps/dashboard/src/features/sensitive-word/sensitive-word-page.tsx`

**Interfaces:**
- Consumes: `SensitiveWordApi.matches(filters)` 与 `matchDetail(id)`。
- Produces: 真实摘要、刷新、分页和结构化详情抽屉。

- [ ] 写入摘要、刷新、分页和详情的失败测试。
- [ ] 运行测试并确认因控件缺失失败。
- [ ] 实现最小行为，确保分页保留全部筛选条件。
- [ ] 重跑单页测试并确认通过。

### Task 2：配置区与视觉统一

**Files:**
- Modify: `web/apps/dashboard/src/features/sensitive-word/sensitive-word-page.tsx`
- Modify: `web/apps/dashboard/src/styles/index.css`
- Test: `web/apps/dashboard/src/features/sensitive-word/sensitive-word-page.test.tsx`

**Interfaces:**
- Consumes: 现有词组、词条及权限控制。
- Produces: 页面标题卡、分区标签、记录工具栏、配置卡片、表格和响应式布局。

- [ ] 添加页面语义类名断言并确认失败。
- [ ] 增加限定在 `sensitive-word-*` 的结构和样式。
- [ ] 验证全部原有 mutation 测试继续通过。
- [ ] 运行 Dashboard 全量测试、类型检查和生产构建。

### Task 3：部署与验收

**Files:**
- Modify: `docs/phases/phase-3-dashboard/phase-3.2/reports/tasks/task-5-report.md`

**Interfaces:**
- Consumes: 通过生产构建的 Dashboard。
- Produces: 更新后的 `mochat-go-desktop` 容器和浏览器验收证据。

- [ ] 构建并替换应用容器，不删除命名数据卷。
- [ ] 验证容器健康与 `/readyz` 返回 200。
- [ ] 对照圆弧检查菜单、记录、配置、空状态和响应式视觉。
- [ ] 更新报告并提交当前 phase3.2 分支。
