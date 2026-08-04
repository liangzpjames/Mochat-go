# Friends Circle UI Optimization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 优化朋友圈任务/素材页面与新增草稿右侧抽屉，并保持现有 Provider 功能和安全阻断可用。

**Architecture:** 在 `FriendsCirclePage` 内拆出展示辅助和抽屉组件，沿用现有查询与写入接口；朋友圈专属 CSS 使用 `phase34-friends-*` 命名，避免影响其他 Phase 3.4 页面。

**Tech Stack:** React 19、TanStack Query、TypeScript、Vitest、Testing Library、CSS。

## Global Constraints

- 不修改朋友圈后端 API 和数据库结构。
- 发布与导出保持阻断，Manifest 保持 `backend=partial`。
- 自动化不得触发企业微信真实发布。

---

### Task 1: 交互合同

**Files:**
- Modify: `web/apps/dashboard/src/features/phase34/content-reach-pages.test.tsx`
- Modify: `web/apps/dashboard/src/features/phase34/content-reach-pages.tsx`

- [ ] 新增失败测试，覆盖 Enter 查询、重置禁用、专属抽屉、字数统计、Escape 与未保存确认。
- [ ] 运行 `pnpm --filter @mochat/dashboard exec vitest run src/features/phase34/content-reach-pages.test.tsx`，确认因新交互缺失而失败。
- [ ] 实现专属抽屉和交互状态，保持任务/素材原有写接口。
- [ ] 重跑定向测试并确认通过。

### Task 2: 视觉与响应式

**Files:**
- Modify: `web/apps/dashboard/src/styles/index.css`

- [ ] 新增朋友圈专属页面、状态徽标、筛选区、抽屉、表单和固定底部样式。
- [ ] 增加 768px 以下全宽抽屉和操作区换行规则。
- [ ] 运行 Dashboard typecheck、Lint、全量测试和 build。

### Task 3: 浏览器验收与部署

**Files:**
- Create: `docs/phases/phase-3-dashboard/phase-3.4/screenshots/friends-circle-ui-optimized.png`

- [ ] 原地构建并重启 `mochat-go-desktop` 的 `app` 服务，不删除数据卷。
- [ ] 浏览器验证任务、素材、筛选、右侧抽屉、取消和保存流程。
- [ ] 截取桌面端最终页面并确认 `readyz=200`、容器健康、四个数据卷仍存在。
