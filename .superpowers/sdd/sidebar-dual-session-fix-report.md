# Sidebar 双部署会话隔离修复报告

日期：2026-08-14
工作树：`D:\workspace\mochat-go\mochat-go\.worktrees\mobile-clients-foundation`
代码提交：`0f19588831bddf7d0103eb823ea081198e6f98e2`

## 修复结论

复核 `web/apps/sidebar/src/deployment.ts` 和真实 Go Sidebar OAuth redirect 后，确认 Sidebar 同时支持两种部署形态，现按部署形态选择独立会话存储：

- 主同源挂载 `/sidebar-app/*`：使用前端可读 cookie，但固定 `Path=/sidebar-app`，Operation/Dashboard 路径不能读取或自动携带该 cookie。
- 独立前端 root mount：使用当前 origin、当前 tab 的 `sessionStorage`，不写 `Path=/` cookie。独立端口属于不同 origin，可与独立 Operation 端口隔离。

router、鉴权边界、OAuth callback 和 Contact 重认证仅依赖统一的 `SidebarSessionAdapter`，不直接访问 cookie 或 Web Storage。`main.tsx` 是唯一按 `sidebarBasename(window.location.pathname)` 选择 adapter 的部署边界。

## Root sessionStorage 合同

存储键固定为 Sidebar 专属 `mochat_sidebar_session_v1`，值包含：

- `token`：非空字符串；
- `agentId`：正整数字符串；
- `expiresAt`：安全整数毫秒时间戳，且必须晚于 `Date.now()`。

JSON 解析失败、字段缺失/非法、过期、storage 访问异常或写入失败均 fail closed：清除该 Sidebar 键并返回 `{ token: null, agentId: null }`。代码不读取任何 `mochat_dashboard_*` key。

## TDD RED → GREEN

### 双适配器和 callback

RED：

```text
corepack pnpm --filter @mochat/sidebar test
12 failed / 35 passed
- createCookieSidebarSessionAdapter is not a function
- createSessionStorageSidebarSessionAdapter is not a function
```

GREEN：

```text
corepack pnpm --filter @mochat/sidebar test
5 files passed；49 tests passed
```

证明内容：

- prefixed callback 写入两个 `Path=/sidebar-app` cookie，并可进入受保护页；
- root callback 只写一个 Sidebar sessionStorage 键，同 tab 跳转后可进入受保护页；
- root callback 不触碰 `document.cookie`；
- malformed、字段非法和 expired storage 全部清除并 fail closed；
- cookie adapter 不产生 `Path=/`。

### Completion gate

RED：

```text
node --test scripts/check_mobile_clients_foundation.test.mjs
32 passed / 3 failed
- 旧 gate 未拒绝 Sidebar token Path=/
- 旧 gate 未拒绝缺少 root sessionStorage adapter
- 旧 gate 未拒绝缺少 root fail-closed 测试
```

GREEN：

```text
corepack pnpm check:mobile-clients-foundation
35 tests passed
sidebar routes=12
operation routes=10
direct fetch=0
dashboard session references=0
mojibake markers=0
fake business outcomes=0
mobile viewport cases>=22
```

### 真实绝对 callback target

Go `SidebarAgentHandler.normalizeTarget` 会把相对 target 拼成 `sidebarBaseURL + target`。对 prefixed 部署，callback 因此可能收到同源绝对 target，例如 `http://host/sidebar-app/medium`。

RED：prefixed router 导航得到 `/sidebar-app/sidebar-app/medium` 并显示 404。

GREEN：callback 校验同源后，仅剥离当前 runtime basename，再交给 router 导航；最终为 `/sidebar-app/medium`，受保护页正常渲染。外部 origin 仍回退 `/`。

## 最终验证

以下命令均 exit 0：

```text
corepack pnpm --filter @mochat/operation lint
corepack pnpm --filter @mochat/operation typecheck
corepack pnpm --filter @mochat/operation test       # 5 files / 74 tests
corepack pnpm --filter @mochat/operation build

corepack pnpm --filter @mochat/sidebar lint
corepack pnpm --filter @mochat/sidebar typecheck
corepack pnpm --filter @mochat/sidebar test         # 5 files / 49 tests
corepack pnpm --filter @mochat/sidebar build

corepack pnpm --filter @mochat/mobile-foundation lint
corepack pnpm --filter @mochat/mobile-foundation typecheck
corepack pnpm --filter @mochat/mobile-foundation test  # 3 files / 21 tests
corepack pnpm --filter @mochat/mobile-foundation build

corepack pnpm check:mobile-clients-foundation       # 35/35
corepack pnpm --filter @mochat/e2e lint
corepack pnpm --filter @mochat/e2e typecheck
corepack pnpm --filter @mochat/e2e test:mobile-clients-foundation
# Chromium 48/48；mobile-390 + desktop-1280

git diff --check
# exit 0；只有 LF→CRLF 提示，无 whitespace error
```

## 范围与安全边界

- 未修改 Go 后端；未操作 Docker、数据库、服务器或 `output`。
- `sessionStorage` 隔离依赖独立端口形成不同 origin；它不是防同 origin XSS 的秘密存储。
- `/sidebar-app` cookie 的 Path 只约束浏览器路径可见性和发送范围，不替代 HttpOnly 或服务端 session。
- 本轮未执行真实企业微信 OAuth；callback、target、会话持久化与受保护路由使用自动化合同测试验证。
