# Mobile Clients Postfix Review 修复报告

日期：2026-08-14
工作树：`D:\workspace\mochat-go\mochat-go\.worktrees\mobile-clients-foundation`
代码提交：`abb534aed3851ba2f69e233d5db78a19730674fb`

## 结论

Postfix review 的 4 项 Important 已按 TDD 修复：

1. 真实 Go `openUserInfo` 缺失文案 `数据不存在` 映射为 `not-found / 活动已失效`，不提供重试。
2. `taskData.end_time` 采用严格正整数 Unix 秒合同；到期或当前秒结束时显示 `not-found / 活动已结束`，不渲染进行中任务和重试。`0`、负数、小数和字符串均安全失败为不可重试 validation，不会默认进行中。
3. Sidebar 前端可读 `token`、`agentId` cookie 的 Path 从 `/` 收窄为 `/sidebar-app`。Sidebar API 仍由客户端读取 token 后显式添加 Bearer；Operation 既不添加 Bearer，浏览器也不会向 Operation 路径发送这两个 cookie。
4. Completion gate 的 WorkFission 证据已移入 Operation `/workFission` 分支，并以分支执行计数 `=== 1` 证明实际执行；Gate 不再依靠文件全局字符串自证。

未修改 Go 后端，未操作 Docker、服务器、数据库或 `output`。

## 合同核对

- `internal/dashboard/work_fission_oauth.go`：活动不存在返回 HTTP 400，消息为 `数据不存在`。
- `internal/dashboard/work_fission_operation.go`：成功 `taskData` 包含 `end_time`。
- Sidebar 本轮部署合同为 `/sidebar-app` basename，callback/login/app 均在该路径下。
- Cookie Path 是浏览器发送范围与脚本可见范围的收窄手段，不是同源应用之间的强安全隔离。因为 cookie 仍是前端可读，不能把 Path 视作 HttpOnly、独立 origin 或服务端 session 的替代品。

## TDD RED → GREEN

### 1. 真实“不存在”活动与活动截止时间

RED：

```text
corepack pnpm --filter @mochat/operation test -- src/features/work-fission/work-fission-page.test.tsx
4 failed / 70 passed
- end_time=0 未被拒绝
- 数据不存在未映射 not-found
- end_time 等于或早于固定当前时间仍显示进行中
```

GREEN：

```text
corepack pnpm --filter @mochat/operation test
5 files passed；74 tests passed
```

固定时间测试使用 `Date.now() = 2_000_000_000_000`：`end_time <= 2_000_000_000` 均结束，`2_000_000_001` 可显示任务进度。页面级用例进一步证明 `end_time=0` 显示“任务进度响应格式无效。”，无“任务进行中”和“重试”。

### 2. Sidebar cookie Path

RED：

```text
corepack pnpm --filter @mochat/sidebar test -- src/auth/sidebar-session.test.ts
2 failed / 37 passed
实际值仍为 Path=/，期望 Path=/sidebar-app
```

GREEN：

```text
corepack pnpm --filter @mochat/sidebar test
5 files passed；39 tests passed
```

E2E 同时证明：

- Sidebar `/sidebar/workContact/detail` 请求仍为 `Authorization: Bearer sidebar-browser-token`。
- Operation 页面 `document.cookie` 不含 Sidebar token 或 `agentId`。
- `/operation/openUserInfo/workFission` 与 `/operation/workFission/taskData` 均无 `Authorization`，其 Cookie header 也不含 `token`/`agentId`。

### 3. Completion gate 分支结构

RED：

```text
node --test scripts/check_mobile_clients_foundation.test.mjs
30 passed / 2 failed
- WorkFission 证据被改到不可执行分支时，旧 gate 未抛错
- 删除 WorkFission 分支执行计数时，旧 gate 未抛错
```

GREEN：

```text
corepack pnpm check:mobile-clients-foundation
sidebar routes=12
operation routes=10
direct fetch=0
dashboard session references=0
mojibake markers=0
fake business outcomes=0
mobile viewport cases>=22
32 tests passed
```

## 最终验证

以下命令均在上述工作树中执行并以 exit 0 完成：

```text
corepack pnpm --filter @mochat/operation lint
corepack pnpm --filter @mochat/operation typecheck
corepack pnpm --filter @mochat/operation test       # 5 files / 74 tests
corepack pnpm --filter @mochat/operation build

corepack pnpm --filter @mochat/sidebar lint
corepack pnpm --filter @mochat/sidebar typecheck
corepack pnpm --filter @mochat/sidebar test         # 5 files / 39 tests
corepack pnpm --filter @mochat/sidebar build

corepack pnpm --filter @mochat/mobile-foundation lint
corepack pnpm --filter @mochat/mobile-foundation typecheck
corepack pnpm --filter @mochat/mobile-foundation test  # 3 files / 21 tests
corepack pnpm --filter @mochat/mobile-foundation build

corepack pnpm check:mobile-clients-foundation       # 32/32
corepack pnpm --filter @mochat/e2e lint
corepack pnpm --filter @mochat/e2e typecheck
corepack pnpm --filter @mochat/e2e test:mobile-clients-foundation
# Chromium：48 passed；mobile-390 + desktop-1280

git diff --check
# exit 0；只有工作树 LF→CRLF 提示，无 whitespace error
```

## 独立审阅与边界

独立只读 code review 未发现 Critical。审阅者提出根路径 Sidebar 部署与固定 `/sidebar-app` cookie Path 的兼容疑虑；本轮明确合同要求 app/callback/login 均部署在 `/sidebar-app`，并明确禁止继续写 `Path=/`，因此未恢复根路径 cookie。采纳其 Minor 建议，E2E 已同时对 token 与 `agentId` 的页面可见性和 Operation 请求 Cookie header 作显式否定断言。

本报告不声称验证真实企业微信 OAuth、真实生产部署或跨 origin 隔离；这些均不在本轮无服务器、无 Docker 的授权范围内。
