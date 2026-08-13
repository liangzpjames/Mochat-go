# Dashboard 全局异常 401 与 53 页逐页验收实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 消除 Dashboard 页面接口对旧 Redis 登录缓存的依赖，确保已授权页面不再错误返回 401，并以 53 页逐页点击矩阵验证后安全部署现有本地与服务器应用容器。

**Architecture:** `DashboardRequestGuard` 和 `DashboardAccessGuard` 是唯一认证/授权入口，业务 handler 只消费 `DashboardPrincipal` 与 `DashboardAccessContext`。后端门禁阻止页面 handler 回归旧缓存身份，前端将真实 401 收敛到幂等退出流程，Playwright 从 manifest 驱动 53 页逐页检查。

**Tech Stack:** Go 1.24、MariaDB/MySQL、Redis、React 19、TypeScript、TanStack Query、React Router、Vitest、Node test、Playwright、Docker Compose、PowerShell/SSH。

## Global Constraints

- 所有用户审阅文档使用中文；代码标识符、命令、路径和协议字段保留原文。
- 开发只在独立 worktree 和独立分支写入；不得 reset/clean/覆盖主工作区现有 dirty 文件。
- 不新增 Docker 项目或平行服务；本地和服务器只允许重建 app，MySQL/Redis 不重建。
- 禁止 `down -v`、`volume rm`、`system prune`、删除或重建四个命名卷。
- 服务器不编译；只上传本地已编译后端和前端静态制品。
- 逐页点击默认只执行安全、无副作用操作；不得删除或覆盖真实业务数据。
- 日志和证据不得写入密码、JWT、Secret 或完整 Authorization header。
- 页面数只取 `web/apps/dashboard/src/benchmark/manifest.json`；当前基线 53/48/5，以执行时门禁输出为准。
- 所有修复遵循 TDD RED→GREEN；单元、结构、浏览器、Docker 和生产验收是不同完成层级，必须分别报告。

---

## File Structure

- `internal/dashboard/dashboard_handler_identity.go`：统一从请求上下文解析业务 handler 身份。
- `internal/dashboard/dashboard_handler_identity_test.go`：身份缺失和身份不一致的失败关闭测试。
- `internal/dashboard/risk_behavior_handler.go`：风险行为 handler 移除旧登录缓存依赖。
- `internal/dashboard/timeout_warning_handler.go`：超时预警 handler 移除旧登录缓存依赖。
- `internal/dashboard/message_intercept_handler.go`：消息拦截、关键词库 handler 移除旧登录缓存依赖。
- `internal/dashboard/*_handler_test.go`：真实 principal/access 上下文、panic legacy cache、401/403 语义测试。
- `scripts/check_dashboard_auth_context.mjs`：从生产 Go route/handler 源码提取旧认证调用，阻止回归。
- `scripts/check_dashboard_auth_context.test.mjs`：临时源码树 RED，排除注释和测试字符串自证。
- `web/apps/dashboard/src/app/unauthorized-handler.ts`：幂等的全局真实 401 处理。
- `web/apps/dashboard/src/app/unauthorized-handler.test.ts`：单次与并发 401 的清理、导航测试。
- `web/apps/dashboard/src/main.tsx`：把 ApiClient 401 接入全局处理器。
- `web/e2e/tests/dashboard-all-pages-auth.spec.ts`：从 manifest 驱动的 53 页本地/生产点击矩阵。
- `web/e2e/fixtures/dashboard-page-actions.ts`：每页一个安全交互动作，缺映射时测试失败。
- `scripts/validate_dashboard_all_pages_evidence.mjs`：验证 53 页证据完整且无意外 401/403/404/5xx。
- `scripts/validate_dashboard_all_pages_evidence.test.mjs`：52 页、漏点击、漏网络、意外 401 的 RED。
- `package.json`、`web/e2e/package.json`：新增独立门禁命令。
- `docs/superpowers/specs/2026-08-13-dashboard-global-401-page-audit-design.md`：已审批设计。

---

### Task 1: 统一 Dashboard handler 身份解析器

**Files:**
- Create: `internal/dashboard/dashboard_handler_identity.go`
- Create: `internal/dashboard/dashboard_handler_identity_test.go`
- Modify: `internal/dashboard/principal_context.go`

**Interfaces:**
- Consumes: `DashboardPrincipalFromContext(ctx)`、`DashboardAccessFromContext(ctx)`。
- Produces: `ResolveDashboardHandlerIdentity(ctx context.Context) (DashboardHandlerIdentity, error)`。

- [ ] **Step 1: 写身份解析 RED**

覆盖：principal/access 完整且一致返回认证身份；缺 principal、缺 access、`UserID/TenantID/CorpID` 任一不一致均返回 `dashboardprincipal.ErrPrincipalUnavailable`；不能从请求参数或缓存补全。

- [ ] **Step 2: 运行 RED**

Run: `go test ./internal/dashboard -run TestResolveDashboardHandlerIdentity -count=1`

Expected: FAIL，函数或类型尚不存在。

- [ ] **Step 3: 实现最小解析器**

```go
type DashboardHandlerIdentity struct {
    UserID         int
    TenantID       int
    CorpID         int
    WorkEmployeeID int
}

func ResolveDashboardHandlerIdentity(ctx context.Context) (DashboardHandlerIdentity, error) {
    principal, err := DashboardPrincipalFromContext(ctx)
    if err != nil { return DashboardHandlerIdentity{}, dashboardprincipal.ErrPrincipalUnavailable }
    access, ok := DashboardAccessFromContext(ctx)
    if !ok || access.UserID != principal.UserID || access.TenantID != principal.TenantID || access.CorpID != principal.CorpID {
        return DashboardHandlerIdentity{}, dashboardprincipal.ErrPrincipalUnavailable
    }
    return DashboardHandlerIdentity{UserID: principal.UserID, TenantID: principal.TenantID, CorpID: principal.CorpID, WorkEmployeeID: access.WorkEmployeeID}, nil
}
```

- [ ] **Step 4: 运行 GREEN 与相邻 context 测试**

Run: `go test ./internal/dashboard -run 'TestResolveDashboardHandlerIdentity|TestDashboardRequestScope' -count=1`

Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/dashboard/dashboard_handler_identity.go internal/dashboard/dashboard_handler_identity_test.go internal/dashboard/principal_context.go
git commit -m "refactor(auth): centralize dashboard handler identity"
```

---

### Task 2: 风险预警三族 handler 退出旧 Redis 登录缓存

**Files:**
- Modify: `internal/dashboard/risk_behavior_handler.go`
- Modify: `internal/dashboard/risk_behavior_handler_test.go`
- Modify: `internal/dashboard/timeout_warning_handler.go`
- Modify: `internal/dashboard/timeout_warning_handler_test.go`
- Modify: `internal/dashboard/message_intercept_handler.go`
- Modify: `internal/dashboard/message_intercept_handler_test.go`
- Modify: `cmd/mochat-go/main.go` only if constructor cleanup is required

**Interfaces:**
- Consumes: `ResolveDashboardHandlerIdentity` 与 request-local `DashboardAccessContext`。
- Produces: 风险行为、超时预警、消息拦截/关键词库全部读写 handler 的统一 401/403 行为。

- [ ] **Step 1: 给每个 handler 族写 panic legacy cache RED**

为三族各构造：有效 principal、匹配 access、实现 `LoginCache` 但调用即 panic。分别调用一条读取路由和一条写路由，证明当前实现会访问旧缓存。

- [ ] **Step 2: 运行 RED**

Run: `go test ./internal/dashboard -run 'Test(RiskBehavior|TimeoutWarning|MessageIntercept).*IgnoresLegacyLoginCache' -count=1`

Expected: FAIL 或 panic，堆栈指向 `UserCorpCache`。

- [ ] **Step 3: 替换三族 resolve 实现**

每个 `resolve` 只调用 `ResolveDashboardHandlerIdentity(r.Context())`；tenant/corp/user/employee 全部来自该结果与 `DashboardAccessContext`。删除 `strings.Split(cached, "-")`、请求 `corpId` 身份覆盖和 `UserCorpCache` 调用。业务权限沿用已通过的 `DashboardAccessGuard`，如 handler 仍需动作级 `#manage` 校验，authorizer 输入必须来自上下文身份。

- [ ] **Step 4: 补齐错误语义 GREEN**

测试矩阵：

- 有效上下文且已授权：业务响应，不返回 401；
- 缺/错上下文：`401` + `UNAUTHORIZED`；
- 数据范围受限：继续传 `AllowedEmployeeIDs`，不能退回旧 `DataPermission`；
- 管理动作无权：`403` + `DASHBOARD_PERMISSION_DENIED`，不清身份；
- Provider 错误：5xx 或既有 provider-unavailable 合同，不能改成 401。

Run: `go test ./internal/dashboard -run 'Test(RiskBehavior|TimeoutWarning|MessageIntercept)' -count=1`

Expected: PASS。

- [ ] **Step 5: 搜索同模式并扩展同一修复批次**

Run: `rg -n "UserCorpCache\(r\.Context\(\)" internal/dashboard cmd/mochat-go`

逐项分类。认证、激活、登出可保留；任何 manifest 页面 handler 必须迁移并增加同类 RED。不能仅修三份已知文件后结束。

- [ ] **Step 6: 提交**

```bash
git add internal/dashboard cmd/mochat-go/main.go
git commit -m "fix(auth): remove legacy cache from dashboard pages"
```

---

### Task 3: 增加旧认证依赖源码门禁

**Files:**
- Create: `scripts/check_dashboard_auth_context.mjs`
- Create: `scripts/check_dashboard_auth_context.test.mjs`
- Modify: `package.json`

**Interfaces:**
- Consumes: 真实 Go server dispatch、option registration、handler method body。
- Produces: `pnpm check:dashboard-auth-context`，输出扫描到的页面 handler 数和零违规结果。

- [ ] **Step 1: 写结构化 fixture RED**

临时源码树分别包含：生产 handler 调 `UserCorpCache`、直接读 Authorization header、注释中同名字符串、`_test.go` 字符串、精确认证端点豁免、未注册 handler。前两项必须失败，注释/测试不能成为违规或通过证据，豁免必须 method+route+symbol 精确匹配。

- [ ] **Step 2: 运行 RED**

Run: `node --test scripts/check_dashboard_auth_context.test.mjs`

Expected: FAIL，门禁模块不存在。

- [ ] **Step 3: 实现生产源码提取与校验**

只扫描 `.go` 生产文件并排除 `_test.go`、migration、JSON；从 `internal/server/server.go` 和 `cmd/mochat-go` 组合根提取可达 route→handler symbol，再读取具体函数体。发现页面 handler 调用旧缓存或解析 Authorization 立即失败。

- [ ] **Step 4: 运行 GREEN 与真实门禁**

Run: `node --test scripts/check_dashboard_auth_context.test.mjs && pnpm check:dashboard-auth-context`

Expected: tests PASS；真实输出 `legacy auth violations=0`。

- [ ] **Step 5: 提交**

```bash
git add scripts/check_dashboard_auth_context.mjs scripts/check_dashboard_auth_context.test.mjs package.json
git commit -m "test(auth): guard dashboard context ownership"
```

---

### Task 4: 前端真实 401 幂等退出，403 保留登录态

**Files:**
- Create: `web/apps/dashboard/src/app/unauthorized-handler.ts`
- Create: `web/apps/dashboard/src/app/unauthorized-handler.test.ts`
- Modify: `web/apps/dashboard/src/main.tsx`
- Modify: `web/packages/api-client/src/client.test.ts` only if transport contract needs additional proof

**Interfaces:**
- Consumes: `authStore.clearSession()`、`queryClient.clear()`、`router.navigate()`。
- Produces: `createDashboardUnauthorizedHandler(deps): () => void`。

- [ ] **Step 1: 写 RED**

证明一次 401 清 session、清 query 并导航到 `/login?returnTo=<当前路径>`；连续或并发调用只执行一次；403 的 ApiClient 路径不调用 unauthorized handler。

- [ ] **Step 2: 运行 RED**

Run: `pnpm --filter @mochat/dashboard test -- unauthorized-handler.test.ts`

Expected: FAIL，处理器不存在。

- [ ] **Step 3: 实现幂等处理器并接入 main**

处理器内部设置同步 latch，先清 session/query 再导航。登录 client 保持 `onUnauthorized: () => undefined`，避免登录失败形成重定向循环。

- [ ] **Step 4: 运行 GREEN 与相邻测试**

Run: `pnpm --filter @mochat/dashboard test -- unauthorized-handler.test.ts access-loader.test.ts && pnpm --filter @mochat/api-client test`

Expected: PASS；TENANT/DASHBOARD permission machine code 行为不漂移。

- [ ] **Step 5: 提交**

```bash
git add web/apps/dashboard/src/app/unauthorized-handler.ts web/apps/dashboard/src/app/unauthorized-handler.test.ts web/apps/dashboard/src/main.tsx web/packages/api-client/src/client.test.ts
git commit -m "fix(auth): handle real dashboard session expiry once"
```

---

### Task 5: 建立 manifest 驱动的 53 页点击与证据门禁

**Files:**
- Create: `web/e2e/fixtures/dashboard-page-actions.ts`
- Create: `web/e2e/tests/dashboard-all-pages-auth.spec.ts`
- Create: `scripts/validate_dashboard_all_pages_evidence.mjs`
- Create: `scripts/validate_dashboard_all_pages_evidence.test.mjs`
- Modify: `web/e2e/package.json`
- Modify: `package.json`

**Interfaces:**
- Consumes: manifest 53 页、真实登录 fixture、页面可访问性和 browser network/console 事件。
- Produces: 每页 `{route, action, titleVisible, responseStatuses, unexpectedResponses, consoleErrors, pageErrors, screenshot}` 证据。

- [ ] **Step 1: 写证据 validator RED**

坏 fixture 独立覆盖：52 页、重复 route、缺安全点击、缺截图、意外 401、已授权页面意外 403、404/5xx、console/pageerror、点击后无法回 `/index` 或用户名消失。

- [ ] **Step 2: 运行 RED**

Run: `node --test scripts/validate_dashboard_all_pages_evidence.test.mjs`

Expected: FAIL，validator 不存在。

- [ ] **Step 3: 实现每页安全动作 registry**

为 manifest 每个 path 精确登记一个无副作用动作；registry 与 manifest 集合必须完全一致，不能 fallback 成只等待页面。按钮不存在时测试失败，不允许悄悄跳过。

- [ ] **Step 4: 实现真实 live Playwright 矩阵**

要求 `MOCHAT_E2E_LIVE_BASE` 和凭据 fixture；live 模式禁止 route interception。每页进入、点击、收集响应、检查 page shell、回数据概览并确认身份。预期 403/Provider 状态只允许按具体 URL+machine code 精确列举，不能全局忽略 401/403/404。

- [ ] **Step 5: 实现 validator 并 GREEN**

Run: `node --test scripts/validate_dashboard_all_pages_evidence.test.mjs && pnpm --filter @mochat/e2e typecheck && pnpm --filter @mochat/e2e lint`

Expected: PASS。

- [ ] **Step 6: 运行非 live UI 合同测试**

Run: `pnpm --filter @mochat/e2e exec playwright test tests/dashboard-all-pages-auth.spec.ts --grep-invert @live --workers=1`

Expected: PASS；明确标注不是生产验收。

- [ ] **Step 7: 提交**

```bash
git add web/e2e scripts/validate_dashboard_all_pages_evidence* package.json
git commit -m "test(dashboard): audit all manifest pages for auth regressions"
```

---

### Task 6: 非 Docker 完整验证和开发任务交付

**Files:**
- Modify only files required by failures attributable to this change

- [ ] **Step 1: Go 全量验证**

Run: `go test ./... -count=1`

Expected: PASS，0 failures。

- [ ] **Step 2: 前端完整验证**

Run:

```bash
pnpm --filter @mochat/dashboard typecheck
pnpm --filter @mochat/dashboard lint
pnpm --filter @mochat/dashboard test
pnpm --filter @mochat/dashboard build
pnpm --filter @mochat/e2e typecheck
pnpm --filter @mochat/e2e lint
```

Expected: 每条 exit 0；完整 dashboard 测试报告 0 failures。

- [ ] **Step 3: 结构门禁**

Run:

```bash
pnpm check:dashboard-auth-context
pnpm check:phase4-dashboard-page-rbac
node --test scripts/check_dashboard_auth_context.test.mjs scripts/validate_dashboard_all_pages_evidence.test.mjs
git diff --check
```

Expected: 53 页、零未映射 API、零 legacy auth violation、全部测试 PASS。

- [ ] **Step 4: 自审并提交修复尾项**

只修复与本次差异有关的问题，保留主工作区所有用户改动。提交后报告完整 commit 列表、测试数量和工作树状态，停止等待主任务审阅。

---

### Task 7: 主任务独立代码审阅

**Files:**
- Read-only review of all branch changes

- [ ] **Step 1: 对照设计逐项审查**

重点检查：是否仍有页面 handler 读取旧缓存；是否错误扩大权限；是否把 403/Provider 错误转 401；是否存在 gate 自证；是否每页都有实际点击动作。

- [ ] **Step 2: 审查 RED→GREEN 证据**

每个行为修复必须有先失败后通过的测试；仅字符串搜索或 mock profile 不算 live 证据。

- [ ] **Step 3: 独立运行关键门禁**

重新执行 Task 6 命令，不采信开发任务的成功摘要。发现 P0/P1 时发回开发任务追加独立 fix commit。

- [ ] **Step 4: 合并到 main**

确认不覆盖主工作区 dirty 文件后，以非破坏方式整合开发提交；整合前后分别记录 `git status --short`。

---

### Task 8: 本地现有 Docker app-only 验收

**Files:**
- Evidence only under `D:/workspace/mochat-go/output/dashboard-auth-audit-20260813/local/`

- [ ] **Step 1: 记录现状**

记录 `mochat-go-desktop` app/mysql/redis 容器 ID、四卷 name/mountpoint、关键表精确 `COUNT(*)`；证据脱敏。

- [ ] **Step 2: 本地主机编译并只重建 app**

禁止重建 MySQL/Redis，禁止任何 volume 删除命令。

- [ ] **Step 3: 健康和原始问题复现验证**

真实登录，进入风险行为，点击查询/刷新，再回数据概览；网络无错误 401，身份仍显示。

- [ ] **Step 4: 运行 53 页 live 点击矩阵**

Run: `pnpm --filter @mochat/e2e exec playwright test tests/dashboard-all-pages-auth.spec.ts --grep @live --workers=1`

Expected: 53/53 均生成完整证据；validator PASS。

- [ ] **Step 5: 对比 Docker 和数据**

MySQL/Redis 容器 ID、四卷 name/mountpoint 不变；只读点击不改变关键表；有明确授权的测试写入按预期审计。

---

### Task 9: 服务器制品部署与生产 53 页验收

**Files:**
- Build artifacts created locally
- Evidence under `D:/workspace/mochat-go/output/dashboard-auth-audit-20260813/server/`

- [ ] **Step 1: 本地生成 release 制品和校验值**

编译 Linux Go 二进制与 Dashboard 静态资源，记录 SHA256。服务器不得执行 Go/Node 编译命令。

- [ ] **Step 2: 记录并备份服务器现状**

记录 app/mysql/redis 容器 ID、四卷、关键表计数和当前制品 SHA256；备份当前二进制/静态资源到时间戳目录。

- [ ] **Step 3: 上传、校验并只重建 app**

上传临时路径，核对 SHA256 后原子替换，只 recreate `standalone-app-1`。MySQL/Redis 不重建。

- [ ] **Step 4: 运行健康、登录和原始路径验收**

验证 `/healthz`、`/readyz`、真实登录、风险行为点击和返回数据概览；不得出现错误 401。

- [ ] **Step 5: 运行生产 53 页 live 矩阵**

遍历全部 53 页并执行 registry 安全动作；要求 console/pageerror 为零，已授权请求无意外 401/403/404/5xx，证据 validator PASS。

- [ ] **Step 6: 数据保留与回滚门禁**

确认 MySQL/Redis 容器 ID和四卷不变，关键数据保留。任一硬门禁失败则恢复 app 制品并 recreate app，不操作数据库和卷。

- [ ] **Step 7: 最终报告**

报告设计/计划路径、提交 SHA、修复的旧认证 handler 清单、53/53 路由与点击数量、请求状态统计、测试数量、Docker/数据保留证据和服务器制品 SHA256。不得泄露任何凭据。
