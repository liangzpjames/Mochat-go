# Task 2：Sidebar 身份、路由与客户摘要纵切面实施报告

## 1. 结论

本任务在 `D:\workspace\mochat-go\mochat-go\.worktrees\mobile-clients-foundation`、分支 `phase5/mobile-clients-foundation` 内实施，基线为 Task 1 提交 `88ee167`。

已完成：

- Sidebar 员工身份 cookie 会话与真实 `/sidebar/agent/auth` OAuth 地址。
- manifest 12 条历史 URL 的显式注册、公开/受保护边界与未知路由 404。
- `/contact` 客户摘要真实读取纵切面及参数错误、401、网络错误、重试、响应校验。
- 有效中文标题/文案、共享移动端 CSS 与 Sidebar 窄屏样式。
- 删除旧通用 fake 页面、fake 按钮、fake 成功与“内容已加载”文案。

本任务没有操作 Docker、服务器、数据库、数据卷或 `output`，也没有读取或写入任何 `mochat_dashboard_*` storage。

## 2. TDD RED → GREEN 记录

### 2.1 session

RED 命令：

```powershell
corepack pnpm --filter @mochat/sidebar test -- src/auth/sidebar-session.test.ts
```

关键输出：

```text
FAIL src/auth/sidebar-session.test.ts
Failed to resolve import "./sidebar-session"
Test Files 1 failed | 3 passed
Tests 16 passed
```

失败原因符合预期：测试先定义了 `CookieAdapter`、cookie 读写、OAuth href、回调写入及安全 target，而生产模块尚不存在。

GREEN 命令同上，首次关键输出：

```text
src/auth/sidebar-session.test.ts (5 tests)
Test Files 4 passed
Tests 21 passed
```

自审时进一步沿真实 Go 数据流发现：`internal/dashboard/sidebar_agent.go` 会把相对 target 规范化成 Sidebar 同源绝对 URL 后回传。补充 RED 复现：

```text
expected target "/contact?...#profile"
received target "/"
Test Files 1 failed | 4 passed
Tests 1 failed | 27 passed
```

最小修复仅允许 `runtime.origin` 同源绝对 URL 转成 path/query/hash，并把转换后的值继续交给 `safeInternalTarget`；外部 origin 仍回落 `/`。补充 GREEN：

```text
src/auth/sidebar-session.test.ts (6 tests)
Test Files 5 passed
Tests 28 passed
```

### 2.2 registry / router

RED 命令：

```powershell
corepack pnpm --filter @mochat/sidebar test -- src/app/catalog.test.ts src/app/sidebar-router.test.tsx
```

关键输出：

```text
FAIL src/app/sidebar-router.test.tsx
Failed to resolve import "../routes/registry"
FAIL src/app/catalog.test.ts
Cannot convert undefined or null to object
Test Files 2 failed | 3 passed
```

失败原因符合预期：新 registry/router 尚不存在，旧 catalog 也没有新合同导出。

GREEN 命令同上，关键输出：

```text
src/app/catalog.test.ts (1 test)
src/app/sidebar-router.test.tsx (3 tests)
Test Files 4 passed
Tests 22 passed
```

### 2.3 contact

RED 命令：

```powershell
corepack pnpm --filter @mochat/sidebar test -- src/features/contact/contact-page.test.tsx
```

关键输出：

```text
FAIL src/features/contact/contact-page.test.tsx
Failed to resolve import "./contact-api"
Test Files 1 failed | 4 passed
Tests 22 passed
```

失败原因符合预期：客户领域 API 与页面尚不存在。

GREEN 命令同上，首次关键输出：

```text
src/features/contact/contact-page.test.tsx (5 tests)
Test Files 5 passed
Tests 27 passed
```

测试覆盖：真实 GET 路径和参数、姓名/头像渲染、缺少外部联系人 ID 时零请求、401 触发 Sidebar 重认证、网络失败重试再次请求、拒绝 snake_case 或空姓名等畸形成功数据。

## 3. 实现与文件

会话与 OAuth：

- `web/apps/sidebar/src/auth/sidebar-session.ts`
- `web/apps/sidebar/src/auth/sidebar-session.test.ts`

路由、注册表与元数据：

- `web/apps/sidebar/src/routes/registry.tsx`
- `web/apps/sidebar/src/app/sidebar-router.tsx`
- `web/apps/sidebar/src/app/sidebar-router.test.tsx`
- `web/apps/sidebar/src/app/catalog.ts`
- `web/apps/sidebar/src/app/catalog.test.ts`
- 删除 `web/apps/sidebar/src/app/sidebar-app.tsx`
- 删除 `web/apps/sidebar/src/app/sidebar-app.test.tsx`

客户摘要：

- `web/apps/sidebar/src/features/contact/contact-api.ts`
- `web/apps/sidebar/src/features/contact/contact-page.tsx`
- `web/apps/sidebar/src/features/contact/contact-page.test.tsx`

应用组合与工程配置：

- `web/apps/sidebar/src/main.tsx`
- `web/apps/sidebar/src/styles.css`
- `web/apps/sidebar/index.html`
- `web/apps/sidebar/package.json`
- `web/apps/sidebar/eslint.config.mjs`
- `pnpm-lock.yaml`

## 4. 自审

- 身份隔离：`readSidebarSession` 的 adapter 调用序列只包含 `token`、`agentId`；Sidebar 源码无 `localStorage`、`sessionStorage` 或 `mochat_dashboard_*`。
- Authorization：`createMobileApiClient({ basePath: '/sidebar' })` 的 `getToken` 只读取 `documentCookieAdapter` 的 Sidebar `token`；Sidebar 代码没有自定义 `Authorization` header，也没有直接 `fetch`。
- Cookie：写入包含 `Path=/; SameSite=Lax`；仅 HTTPS runtime 增加 `Secure`；清理也只清理 `token`、`agentId`。
- 跳转安全：登录 href 与回调最终 target 均通过 `safeInternalTarget`；跨源、协议相对、反斜杠等目标回落 `/`。
- 路由：12 条 manifest URL 均在 `routeDefinitions` 显式列出，module key 唯一，auth 与 manifest 一致；`/login`、`/auth` 公开，`/codeAuth` 按 manifest 公开；受保护页面先重定向；`*` 只显示“页面不存在”。
- 业务真实性：仅 `/contact` 声明本阶段完成；其余模块明确显示“模块待迁移”，没有随机数据、伪写操作或伪成功。
- 客户合同：请求固定为 `GET /workContact/detail?wxExternalUserid=...`；只接收 `id/name/avatar/corpId` camelCase 稳定模型。
- 中文与样式：HTML 标题为 `MoChat 客户侧边栏`；共享 `@mochat/mobile-foundation/styles/mobile.css` 在 Sidebar CSS 前导入；触控链接最小高度 44px，内容列使用 `minmax(0, 1fr)`，页面禁止横向溢出。
- 变更边界：Git 变更仅位于 `web/apps/sidebar`、`pnpm-lock.yaml` 与本报告；`dist` 未进入 Git。

## 5. 门禁

按 brief 执行：

```powershell
corepack pnpm --filter @mochat/sidebar lint
corepack pnpm --filter @mochat/sidebar typecheck
corepack pnpm --filter @mochat/sidebar test
corepack pnpm --filter @mochat/sidebar build
git diff --check
```

最终新鲜执行结果见提交前终端记录：lint 与 typecheck 退出码 0；测试 5 个文件、28 个测试全部通过；Vite production build 通过；`git diff --check` 退出码 0。

补充静态检查：

```text
manifest count: 12
missing explicit definitions: 0
forbidden scan: 0 matches
```

扫描词包括 `mochat_dashboard_`、`localStorage`、`sessionStorage`、直接 `fetch(`、`fake`、旧伪成功文案及 Unicode replacement character。

## 6. 疑虑与范围边界

- 按任务约束未启动 Docker、真实 Go 服务、企业微信 OAuth、数据库或浏览器，因此本报告不宣称真实企业微信/数据库业务闭环已验收。
- Go 回调回传同源绝对 target 的兼容已由单元测试覆盖，但仍需后续真实 OAuth 浏览器验收确认运行环境中的实际 Sidebar origin 配置。
- 其余 11 个历史路由在本任务只建立可辨识、无假数据的模块边界；不宣称这些业务模块已经迁移完成。
