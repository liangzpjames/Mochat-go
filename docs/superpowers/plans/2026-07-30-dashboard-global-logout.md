# Dashboard 全局退出能力实现计划

> **供智能执行者使用：** 必须使用 `executing-plans` 或 `subagent-driven-development` 按任务执行；所有步骤使用复选框跟踪。

**目标：** 让无企业空状态和正常 Dashboard 顶部栏都提供可靠的账号退出能力。

**架构：** 新增无副作用的会话操作上下文，由应用入口注入用户标识和异步退出方法。布局与企业空状态只消费上下文；应用入口负责调用服务端注销、清理本地会话与查询缓存并跳转登录页。

**技术栈：** React 19、TypeScript、React Router、TanStack Query、Vitest、Testing Library。

## 全局约束

- 所有用户可见文案使用中文。
- 服务端注销失败不能阻止本地退出。
- 退出进行中必须禁用按钮，避免重复提交。
- 不修改企业选择和权限加载的既有行为。

---

### 任务一：会话操作上下文与退出 API

**文件：**
- 新建：`web/apps/dashboard/src/features/auth/session-actions.tsx`
- 修改：`web/apps/dashboard/src/features/auth/auth-api.ts`
- 测试：`web/apps/dashboard/src/features/auth/auth-api.test.ts`

**接口：**
- 产出：`DashboardSessionActions`，包含 `userId: string | null`、`isLoggingOut: boolean`、`logout(): Promise<void>`。
- 产出：`DashboardSessionActionsProvider` 与 `useDashboardSessionActions()`。
- 产出：`logout(client): Promise<void>`，请求 `PUT /user/logout`。

- [ ] **步骤 1：先写失败测试**

验证 `logout(client)` 使用 `PUT /user/logout`。

- [ ] **步骤 2：运行测试确认红灯**

运行：`pnpm --filter @mochat/dashboard test -- src/features/auth/auth-api.test.ts`

预期：因 `logout` 尚未导出而失败。

- [ ] **步骤 3：实现最小 API 与上下文**

上下文默认值为 `null`，消费者未位于 Provider 中时抛出明确错误；API 只负责服务端请求。

- [ ] **步骤 4：运行测试确认绿灯**

运行相同测试，预期全部通过。

### 任务二：空企业状态和全局顶部栏

**文件：**
- 修改：`web/apps/dashboard/src/features/corp/corp-provider.tsx`
- 修改：`web/apps/dashboard/src/features/corp/corp-provider.test.tsx`
- 修改：`web/apps/dashboard/src/layout/dashboard-layout.tsx`
- 修改：`web/apps/dashboard/src/layout/dashboard-layout.test.tsx`
- 修改：`web/apps/dashboard/src/styles/index.css`

**接口：**
- 消费：`useDashboardSessionActions()`。
- 空状态展示标题、说明、账号标识和“退出登录”按钮。
- 顶部栏展示账号标识和“退出登录”按钮。

- [ ] **步骤 1：先写失败测试**

给两个组件包裹会话 Provider，分别断言空状态与正常布局存在唯一的“退出登录”按钮；点击后调用 `logout`，`isLoggingOut=true` 时按钮禁用。

- [ ] **步骤 2：运行测试确认红灯**

运行：`pnpm --filter @mochat/dashboard test -- src/features/corp/corp-provider.test.tsx src/layout/dashboard-layout.test.tsx`

预期：找不到“退出登录”按钮。

- [ ] **步骤 3：实现最小界面**

复用同一会话上下文；空状态采用居中卡片，顶部栏账号操作固定在右侧，不改变现有侧栏滚动布局。

- [ ] **步骤 4：运行测试确认绿灯**

运行相同测试，预期全部通过。

### 任务三：应用入口退出流程

**文件：**
- 修改：`web/apps/dashboard/src/main.tsx`
- 测试：`web/apps/dashboard/src/features/auth/session-actions.test.tsx`

**接口：**
- 应用入口向 Provider 注入当前 `authStore` 用户标识。
- 退出顺序：调用服务端注销；`finally` 清除认证、清空查询缓存、跳转 `/login`。

- [ ] **步骤 1：先写失败测试**

抽取并测试 `createLogoutAction`：服务端注销成功和失败两种情况下都调用 `clearSession`、`queryClient.clear` 与 `navigate('/login')`。

- [ ] **步骤 2：运行测试确认红灯**

运行：`pnpm --filter @mochat/dashboard test -- src/features/auth/session-actions.test.tsx`

预期：因 `createLogoutAction` 尚不存在而失败。

- [ ] **步骤 3：实现退出协调器并接入入口**

使用 `try/finally` 保证本地清理；Provider 在退出期间维护 `isLoggingOut`。

- [ ] **步骤 4：运行 Dashboard 全量测试和构建**

运行：

```powershell
pnpm --filter @mochat/dashboard test
pnpm --filter @mochat/dashboard build
```

预期：全部测试和构建通过。

### 任务四：部署与浏览器验收

**文件：**
- 不新增文件。

- [ ] **步骤 1：使用现有镜像构建能力重新部署**

保留 `mochat-go-desktop` 数据卷，沿用端口 `28080/28081/28082`。

- [ ] **步骤 2：真实浏览器验证**

使用无企业账号登录 Dashboard，确认：

1. 空状态卡片可见。
2. 退出按钮可见且可点击。
3. 点击后返回登录页，本地会话失效。
4. 正常有企业页面顶部栏也存在退出入口。

- [ ] **步骤 3：提交并推送主线**

仅提交本计划涉及的源码、测试和中文文档，不提交工作区其他未跟踪内容。
