# Mobile Clients Foundation 最终修复报告

## 1. 结论

本轮在 `D:\workspace\mochat-go\mochat-go\.worktrees\mobile-clients-foundation`、分支 `phase5/mobile-clients-foundation` 内完成最终审阅的 Critical 与 Important 修复，代码提交为 `035517f`。

完成内容：

- Operation 任务宝真实入口改为 `/workFission?id=<positive>`，兼容旧 `fission_id`，但不再把 URL `union_id` 当作身份。
- 页面先通过同源 cookie 会话请求 `/operation/openUserInfo/workFission?id=<id>`；仅使用服务端 session 返回的 `unionid` 请求 taskData。
- session 返回 `[]` 或 401 时提供当前活动 OAuth；目标地址经过 `safeInternalTarget`，真实 href 为 `/auth/workFission?id=<id>&target=<safe>`。
- `requiresActivitySession` 由 Operation router 显式消费；未迁移的 `/speed` 不再声明虚假的已实现 session 元数据。
- Contact 与 WorkFission 按 `MobileApiError.kind/retryable` 映射 forbidden、not-found、不可重试 error、可重试 error 与 401 专属重认证。
- 活动不存在、失效或结束的 HTTP 400 validation 消息显示为 not-found/活动失效，且不显示重试。
- completion gate 强制真实 `id` 入口、openUserInfo raw Go envelope、session unionid 证明和 raw `[]` OAuth 证明。
- Playwright 在 390×844 与 1280×900 两个视口完成 48 项验收。

本轮未修改 Go 后端，未操作 Docker、服务器、数据库、数据卷或 `output`。

## 2. 合同依据

实施前完整核对：

- `internal/dashboard/work_fission_oauth.go`
- `internal/dashboard/work_fission_operation.go`
- `internal/dashboard/wework_callback_worker.go:838`
- `internal/dashboard/work_fission.go:1247`
- `web/legacy/operation/src/views/workFission/index.vue`
- `web/legacy/operation/src/views/workFission/speed.vue`
- `web/legacy/operation/src/api/workFission.js`

确认的真实合同：

1. Go 生成的活动目标为 `/workFission?id=<id>`。
2. OAuth/OpenUserInfo 使用 `MOCHAT_SESSION_ID` HttpOnly、SameSite=Lax 同源 cookie 会话。
3. `GET /operation/openUserInfo/workFission?id=<id>` 的 envelope `data` 是 `[]` 或 OAuth 用户对象；前端严格校验 `openid`、`unionid`、`nickname`、`headimgurl`。
4. `GET /operation/workFission/taskData` 仍要求 `union_id` 与正整数 `fission_id`；其中 `union_id` 必须来自上述 session 用户对象。
5. Operation 客户端保持 `credentials: same-origin`，未配置 `getToken`，不发送 Bearer，不读取 Sidebar/Dashboard 身份。

## 3. Fix A：Critical 身份与真实入口

### 3.1 RED

命令：

```powershell
corepack pnpm --filter @mochat/operation test -- src/features/work-fission/work-fission-page.test.tsx src/app/operation-router.test.tsx
node --test scripts/check_mobile_clients_foundation.test.mjs
node scripts/check_mobile_clients_foundation.mjs
```

关键结果：

```text
Operation: 2 failed files; 20 failed / 35 passed
loadWorkFissionParticipant is not a function
真实 id 入口仍显示“缺少 union_id”
registry 仍要求 union_id/fission_id
raw [] 后找不到“重新授权”链接

completion gate defect tests: 30/30 passed
real gate: failed - taskData browser fixture must prove a session-derived unionid
```

失败原因均为目标行为尚未实现，而非测试语法或 fixture 初始化错误。

### 3.2 GREEN

聚焦命令同上，关键结果：

```text
Operation: 5 files / 55 tests passed
completion gate defect tests: 30/30 passed
real completion gate: passed
```

实现要点：

- 新增 `loadWorkFissionParticipant(request, id)`，请求 `/openUserInfo/workFission?id=<id>`。
- `[]` 映射为 `null`；非空数组、缺字段、空 `openid/unionid` 或错误字段类型均抛出 validation。
- `WorkFissionPage` 先 participant、后 taskData；taskData 的 `union_id` 只取 `participant.unionid`。
- URL `union_id` 可出现在安全回跳 target 中，但从不参与 API 身份参数。
- router 通过 `requiresActivitySession + activityKind` 选择 session-aware WorkFission 页面。

## 4. Fix B：Important 错误状态与重试

### 4.1 RED

命令：

```powershell
corepack pnpm --filter @mochat/sidebar test -- src/features/contact/contact-page.test.tsx
corepack pnpm --filter @mochat/operation test -- src/features/work-fission/work-fission-page.test.tsx
```

关键结果：

```text
Sidebar: 5 failed / 34 passed
Operation: 8 failed / 58 passed
```

预期失败表现：

- forbidden/not-found 仍渲染普通 error。
- validation/conflict/未知错误仍错误显示“重试”。
- 活动不存在类 validation 尚未映射为 not-found。

### 4.2 GREEN

```text
Sidebar: 5 files / 39 tests passed
Operation: 5 files / 66 tests passed
```

最终映射：

| 错误 | MobileState | 重试 |
| --- | --- | --- |
| unauthorized | 各应用原有重认证/OAuth | 否 |
| forbidden | `forbidden` | 否 |
| not-found | `not-found` | 否 |
| validation/conflict | `error`，保留服务端明确消息 | 否 |
| network/server | `error` | 是 |
| 其他且 `retryable=true` | `error` | 是 |
| 未知错误 | `error` | 否 |
| WorkFission validation 且消息匹配活动不存在/失效/结束 | `not-found`/活动已失效 | 否 |

## 5. 最终新鲜验证

### 5.1 Operation

```text
corepack pnpm --filter @mochat/operation lint       exit 0
corepack pnpm --filter @mochat/operation typecheck  exit 0
corepack pnpm --filter @mochat/operation test       exit 0; 5 files / 66 tests
corepack pnpm --filter @mochat/operation build      exit 0; 35 modules transformed
```

### 5.2 Sidebar

```text
corepack pnpm --filter @mochat/sidebar lint       exit 0
corepack pnpm --filter @mochat/sidebar typecheck  exit 0
corepack pnpm --filter @mochat/sidebar test       exit 0; 5 files / 39 tests
corepack pnpm --filter @mochat/sidebar build      exit 0; 35 modules transformed
```

### 5.3 共享基础、完成门禁与 E2E

```text
corepack pnpm --filter @mochat/mobile-foundation test
exit 0; 3 files / 21 tests

corepack pnpm check:mobile-clients-foundation
exit 0; defect tests 30/30
sidebar routes=12
operation routes=10
direct fetch=0
dashboard session references=0
mojibake markers=0
fake business outcomes=0
mobile viewport cases>=22

corepack pnpm --filter @mochat/e2e typecheck
exit 0

corepack pnpm --filter @mochat/e2e lint
exit 0

corepack pnpm --filter @mochat/e2e test:mobile-clients-foundation
exit 0; 48 passed (31.9s)
```

浏览器中的 `/workFission?id=17` 用例逐个断言：

- openUserInfo 与 taskData 两个请求均无 Authorization header。
- openUserInfo 的 `id=17`。
- taskData 的 `union_id=union-browser-session-1`，来自 session fixture，而不是 URL。
- participant raw `[]` 后只新增 openUserInfo 请求，不新增 taskData 请求。
- OAuth href 为 `/auth/workFission?id=17&target=%2FworkFission%3Fid%3D17%26union_id%3Dattacker-controlled`。
- console、pageerror、requestfailed、4xx/5xx 与 unexpected request 审计均为空。

## 6. 文件与提交

代码提交：

```text
035517f fix(mobile): bind activity identity and error states
```

主要文件：

- `web/apps/operation/src/features/work-fission/work-fission-api.ts`
- `web/apps/operation/src/features/work-fission/work-fission-page.tsx`
- `web/apps/operation/src/app/operation-router.tsx`
- `web/apps/operation/src/routes/registry.tsx`
- `web/apps/sidebar/src/features/contact/contact-page.tsx`
- `web/e2e/tests/mobile-clients-foundation.spec.ts`
- `scripts/check_mobile_clients_foundation.mjs`
- 对应组件、router 与 gate 测试文件

## 7. 边界与疑虑

- 本轮遵守范围约束，没有启动 Docker、连接真实服务器/数据库、写数据或操作 `output`。
- Playwright 使用真实 React 构建与 raw Go envelope fixture，证明请求顺序、参数、身份来源、OAuth href 和浏览器状态；它不等同于真实公众号 OAuth/Redis session/企业微信网络闭环。
- Go/legacy 文件仅作为合同依据读取，未修改 Go 后端。
- `/speed` 仍是明确的“模块待迁移”页面；本轮不宣称其历史业务已经迁移。
