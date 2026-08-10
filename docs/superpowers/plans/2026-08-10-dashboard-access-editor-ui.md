# Dashboard 权限编辑器界面优化 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将员工直授权限与角色权限改造成分组、可搜索、可设置数据范围且适配 390px 的共享编辑器，并从员工弹窗移除继承权限展示。

**Architecture:** 新建纯受控 `AccessPermissionSelector`，以 catalog 和 `{code, scope}[]` 为输入，通过 `onChange` 返回完整选择结果；员工页和角色页各自继续管理查询、表单与 mutation。样式集中加入 Dashboard 全局样式文件，沿用现有 `DashboardDialog` 与 `ConfirmAction`。

**Tech Stack:** React 19、TypeScript、TanStack Query、Vitest、Testing Library、CSS。

## Global Constraints

- 所有写入仅位于 `.worktrees/phase4-dashboard-page-rbac`。
- 不操作 Docker、Compose、服务、数据库、卷或 `output` 验收目录。
- 不改变 RBAC API 和存储合同。
- 四个 `superadminOnly` 权限不可授予。
- mutation 必须继续经过 `ConfirmAction`，并保留 `expectedVersion` 与 409 表单保留语义。

---

### Task 1: 用失败测试锁定员工与角色编辑体验

**Files:**
- Modify: `web/apps/dashboard/src/features/company-settings/access-pages.test.tsx`

**Interfaces:**
- Consumes: `AccessStaffPage`、`AccessRolePage` 现有公开 props。
- Produces: 权限分组、搜索、scope、无继承展示和 390px 行为的回归合同。

- [ ] **Step 1: 编写员工弹窗 RED**

在员工详情 fixture 中保留 `inheritedPermissions`，打开编辑后断言页面不显示“继承权限”及来源角色；同时断言摘要显示角色数/直接权限数、catalog 分组标题出现、停用角色不可选。

- [ ] **Step 2: 编写共享编辑器 RED**

为员工直授和角色编辑分别选择权限，将 scope 改为 `tenant`，点击保存后先断言 mutation 未调用，再确认并断言 payload；输入搜索词后只保留匹配页面。

- [ ] **Step 3: 编写 390px RED**

设置 `window.innerWidth = 390`，断言权限编辑区仍可访问、范围选择框存在，并验证 `document.documentElement.scrollWidth <= document.documentElement.clientWidth`。

- [ ] **Step 4: 运行定向测试确认失败**

Run: `pnpm --filter @mochat/dashboard exec vitest run src/features/company-settings/access-pages.test.tsx`

Expected: FAIL，原因是现有页面仍展示继承权限且没有共享分组编辑器。

### Task 2: 实现共享权限选择器

**Files:**
- Create: `web/apps/dashboard/src/features/company-settings/access-permission-selector.tsx`
- Modify: `web/apps/dashboard/src/styles/index.css`
- Test: `web/apps/dashboard/src/features/company-settings/access-pages.test.tsx`

**Interfaces:**
- Consumes: `AccessCatalogItem[]`、`Array<{code: string; scope: string}>`。
- Produces: `AccessPermissionSelector({ catalog, value, onChange, ariaLabel })`。

- [ ] **Step 1: 建立严格类型的受控组件**

实现 `PermissionSelection` 类型与 `AccessPermissionSelector` props；先过滤 `superadminOnly`，再按名称/code 搜索并按 `groupCode` 分组。

- [ ] **Step 2: 实现权限行更新**

未选权限勾选时加入 `{code, scope: "self"}`；取消时移除；scope 下拉只更新对应 code，选项固定为 `self / department / tenant`。

- [ ] **Step 3: 实现布局样式**

增加工具栏、分组卡片、权限行、计数徽标和内部滚动样式；在 `max-width: 768px` 下切换为单列并让表单控件占满宽度。

- [ ] **Step 4: 运行定向测试**

Run: `pnpm --filter @mochat/dashboard exec vitest run src/features/company-settings/access-pages.test.tsx`

Expected: 共享组件相关测试 PASS。

### Task 3: 接入员工与角色页面

**Files:**
- Modify: `web/apps/dashboard/src/features/company-settings/access-staff-page.tsx`
- Modify: `web/apps/dashboard/src/features/company-settings/access-role-page.tsx`
- Test: `web/apps/dashboard/src/features/company-settings/access-pages.test.tsx`

**Interfaces:**
- Consumes: `AccessPermissionSelector`。
- Produces: 优化后的员工与角色权限编辑弹窗。

- [ ] **Step 1: 改造员工弹窗**

加入编辑摘要与角色卡片网格，使用共享选择器编辑 `direct`；物理删除继承权限标题与来源渲染，不改详情加载和保存防线。

- [ ] **Step 2: 改造角色弹窗**

角色名称和备注放入结构化基础信息区，使用共享选择器编辑 `codes`；保持系统角色、状态、删除和 409 规则不变。

- [ ] **Step 3: 运行 RED/GREEN 测试**

Run: `pnpm --filter @mochat/dashboard exec vitest run src/features/company-settings/access-pages.test.tsx`

Expected: 全部 PASS，保存 payload 和确认时序保持原合同。

### Task 4: 完整非 Docker 验证与提交

**Files:**
- Verify: `web/apps/dashboard/src/features/company-settings/*`
- Verify: `web/apps/dashboard/src/styles/index.css`

**Interfaces:**
- Consumes: Tasks 1-3 的完整改动。
- Produces: 可提交且通过 Dashboard 门禁的 UI 优化。

- [ ] **Step 1: 运行格式与静态检查**

Run: `pnpm --filter @mochat/dashboard typecheck`

Expected: PASS。

Run: `pnpm --filter @mochat/dashboard lint`

Expected: PASS。

- [ ] **Step 2: 运行测试和构建**

Run: `pnpm --filter @mochat/dashboard test`

Expected: 所有 Dashboard 测试 PASS。

Run: `pnpm --filter @mochat/dashboard build`

Expected: PASS。

- [ ] **Step 3: 运行 RBAC catalog gate**

Run: `pnpm check:phase4-dashboard-page-rbac`

Expected: 53 pages、49 ordinary、4 superadmin、0 unmapped。

- [ ] **Step 4: 自审并提交**

Run: `git diff --check && git status --short`

Expected: 无空白错误，改动仅限本计划文件。

Commit: `feat(dashboard): refine access permission editors`

