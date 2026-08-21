# Conversation Header Removal Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 删除所有共用会话页面的顶部介绍横幅，并把唯一的刷新入口迁移到查询按钮旁。

**Architecture:** 调整 `ConversationGlobalPage` 和独立的 `EmployeeConversationPage` 展示结构，不改变查询参数或后端接口。使用现有 React Query `refetch()` 保持刷新行为，并移除失去用途的页头 CSS。

**Tech Stack:** React 19、TypeScript、TanStack Query、Vitest、Testing Library、CSS

## Global Constraints

- 同时覆盖全局消息、员工会话、客户会话和群聊会话。
- 刷新不得改变当前 URL 或筛选条件。
- 页面中只能保留一个“刷新消息”按钮。
- 不修改后端接口和菜单导航。

---

### Task 1: 页面结构与刷新入口

**Files:**
- Modify: `web/apps/dashboard/src/features/conversation-global/conversation-global-page.test.tsx`
- Modify: `web/apps/dashboard/src/features/conversation-global/conversation-global-page.tsx`
- Modify: `web/apps/dashboard/src/features/conversation-global/employee-conversation-page.test.tsx`
- Modify: `web/apps/dashboard/src/features/conversation-global/employee-conversation-page.tsx`
- Modify: `web/apps/dashboard/src/styles/index.css`

**Interfaces:**
- Consumes: `listQuery.refetch(): Promise<QueryObserverResult>`
- Consumes: `employeeQuery.refetch(): Promise<QueryObserverResult>`
- Produces: 全局筛选表单内唯一的 `button[aria-label="刷新消息"]`，以及员工搜索表单内的 `button[aria-label="刷新员工"]`

- [x] **Step 1: Write the failing test**

```tsx
expect(container.querySelector('.conversation-global-header')).toBeNull();
const refresh = screen.getByRole('button', { name: '刷新消息' });
expect(refresh.closest('form')).toBe(screen.getByRole('button', { name: '查询' }).closest('form'));
expect(screen.getAllByRole('button', { name: '刷新消息' })).toHaveLength(1);

expect(container.querySelector('.employee-conversation-header')).toBeNull();
const employeeRefresh = screen.getByRole('button', { name: '刷新员工' });
expect(employeeRefresh.closest('form')).toBe(screen.getByRole('button', { name: '搜索员工' }).closest('form'));
```

- [x] **Step 2: Run test to verify it fails**

Run: `npm --prefix web/apps/dashboard test -- --run src/features/conversation-global/conversation-global-page.test.tsx`

Expected: FAIL，因为当前 `.conversation-global-header` 和 `.employee-conversation-header` 仍存在，刷新按钮位于表单之外。

- [x] **Step 3: Write minimal implementation**

```tsx
<button type="submit">查询</button>
<button aria-label="刷新消息" disabled={listQuery.isFetching} onClick={() => void listQuery.refetch()} type="button">
  {listQuery.isFetching && !listQuery.isPending ? '刷新中…' : '刷新消息'}
</button>
<button onClick={resetFilters} type="button">重置</button>

<button aria-label="刷新员工" disabled={employeeQuery.isFetching} onClick={() => void employeeQuery.refetch()} type="button">
  {employeeQuery.isFetching && !employeeQuery.isPending ? '刷新中…' : '刷新'}
</button>
```

删除两个页面的页头 JSX、无用的 `pageTitle`，以及 `.conversation-global-header`、`.conversation-global-eyebrow`、`.employee-conversation-header` 样式。

- [x] **Step 4: Run test to verify it passes**

Run: `npm --prefix web/apps/dashboard test -- --run src/features/conversation-global/conversation-global-page.test.tsx src/features/conversation-global/employee-conversation-page.test.tsx src/styles/conversation-global-layout.test.ts`

Expected: PASS，且刷新测试确认请求次数增加、URL 筛选保持不变。

- [x] **Step 5: Verify build and runtime**

Run: `npm --prefix web/apps/dashboard run typecheck`

Run: `npm --prefix web/apps/dashboard run build`

Expected: 两条命令均退出码 0；更新本地应用容器后，浏览器确认横幅消失且刷新按钮位于查询旁。
