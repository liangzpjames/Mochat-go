# Task 3 实施报告：Operation 会话、路由与任务宝纵切面

## 1. 结果与边界

已完成 Operation 活动移动端基础壳与任务宝首个真实纵切面：

- Operation 仅使用后端活动 OAuth 与同源 cookie 会话；API 客户端固定 `basePath: '/operation'`、`credentials: 'same-origin'`，未提供 `getToken`，未读取 Sidebar cookie/token 或 Dashboard storage。
- manifest 中 10 条历史 URL 均由显式 registry 唯一映射；未知路径显示 404，不回落首页。
- 删除通用 `execute()`、`POST /operation/<feature>`、固定假进度、兜底成功和奖品文案；除 `/workFission` 外的 9 条路由只显示具名“模块待迁移”状态。
- `/workFission` 请求真实 feature path `/workFission/taskData`；共享客户端会形成 `/operation/workFission/taskData`，并强制 `union_id` 与正整数 `fission_id`。
- 401 只构造当前任务宝活动的 `/auth/workFission` OAuth 地址，保留 pathname、query、hash，不清理其他应用会话。
- 未操作 Docker、Compose、服务器、数据库、数据卷或 `output`，未修改 Sidebar。

## 2. Go 原始合同核对与计划类型修正

实施前核对 `internal/dashboard/work_fission_operation.go`：

- `TaskData` 读取 query：`union_id`、正整数 `fission_id`。
- 成功 data 字段：`invite_count`、`differ_count`、`end_time`、`task`。
- `task[]` 原始字段：`count`、`status`、`receive_status`、`gift_type`、`gift_url`。
- OAuth handler 使用活动参数 `id`，入口为 `/auth/workFission`。

原计划稳定类型中的 `rewardLabel` 在 Go 响应中没有来源，会迫使前端伪造奖励名称。经源码核对后，feature boundary 改为：

```ts
type WorkFissionTask = {
  level: number;
  target: number;
  completed: boolean;
  received: boolean;
  reward: { type: number; url: string | null };
};
```

其中 `level` 仅由任务数组顺序 `index + 1` 生成；`target/completed/received/reward` 分别严格来自 `count/status/receive_status/gift_type/gift_url`。UI 只显示奖励类型（二维码/链接）与实际存在的 URL，不生成奖励名称。

## 3. TDD RED → GREEN 证据

### 3.1 Session 与参数

RED：

```text
Failed to resolve import "./operation-session"
Failed to resolve import "./work-fission-page"
Test Files 2 failed | 3 passed
```

GREEN：

```text
src/auth/operation-session.test.ts: 2 passed
src/features/work-fission/work-fission-page.test.tsx: 4 passed
Test Files 5 passed; Tests 20 passed
```

后续自审补充 feature API boundary 参数防线，先得到 3 个预期 RED（无效参数仍进入 request）；再以未知 `gift_type` 会生成前端文案为 RED，收紧为 Go 当前 `0/1` 合同，最终 GREEN 为 32/32。

### 3.2 Exact routes

RED：

```text
Failed to resolve import "../routes/registry"
operationCatalog undefined
Test Files 2 failed | 4 passed
```

GREEN：

```text
src/app/catalog.test.ts: 1 passed
src/app/operation-router.test.tsx: 3 passed
Test Files 5 passed; Tests 21 passed
```

断言覆盖 manifest 路径集合严格相等、10 条 path 唯一、10 个 module key 唯一、中文标题、具名待迁移边界和未知路径 404。

### 3.3 Work-fission

RED：

```text
Failed to resolve import "./work-fission-api"
Test Files 1 failed | 4 passed
```

GREEN：

```text
src/features/work-fission/work-fission-page.test.tsx: 15 passed
Operation total: 5 files, 32 tests passed
```

断言覆盖真实 GET 路径、raw snake_case adapter、邀请/差额/任务标签、空任务、401 OAuth、query/hash 保留、无效响应、活动失效及重试。

## 4. 文件变更

新增：

- `web/apps/operation/eslint.config.mjs`
- `web/apps/operation/src/app/operation-router.tsx`
- `web/apps/operation/src/app/operation-router.test.tsx`
- `web/apps/operation/src/auth/operation-session.ts`
- `web/apps/operation/src/auth/operation-session.test.ts`
- `web/apps/operation/src/features/work-fission/work-fission-api.ts`
- `web/apps/operation/src/features/work-fission/work-fission-page.tsx`
- `web/apps/operation/src/features/work-fission/work-fission-page.test.tsx`
- `web/apps/operation/src/routes/registry.tsx`

修改：

- `web/apps/operation/package.json`
- `web/apps/operation/index.html`
- `web/apps/operation/src/main.tsx`
- `web/apps/operation/src/styles.css`
- `web/apps/operation/src/app/catalog.ts`
- `web/apps/operation/src/app/catalog.test.ts`
- `web/apps/operation/src/phase2-completion.test.ts`
- `pnpm-lock.yaml`

删除：

- `web/apps/operation/src/app/operation-app.tsx`
- `web/apps/operation/src/app/operation-app.test.tsx`

## 5. 静态门禁

最终新鲜验证：

```text
corepack pnpm --filter @mochat/operation lint       exit 0
corepack pnpm --filter @mochat/operation typecheck  exit 0
corepack pnpm --filter @mochat/operation test       exit 0; 5 files / 32 tests
corepack pnpm --filter @mochat/operation build      exit 0; 35 modules transformed
git diff --check                                    exit 0
```

生产源码扫描未发现直接 `fetch(`、`localStorage`、`sessionStorage`、`document.cookie`、`getToken`、`Authorization`、generic `execute()`、假 activity ID、`prizeName`、固定进度或“操作已完成”。`git diff -- web/apps/sidebar` 无输出。

## 6. 自审

- 会话隔离：通过。Operation main 未提供 Bearer token provider，活动授权不读写其他应用状态。
- URL 安全：通过。OAuth target 使用 `safeInternalTarget`；外部 target 回退 `/`，当前 query/hash 有测试。
- 路由完整性：通过。10/10 显式映射，未知 404，无 generic fallback。
- 参数防线：通过。page 与 feature API boundary 均阻断空 `union_id`、零数/负数/小数 `fission_id`，无效参数请求数为 0。
- 数据真实性：通过。adapter 只在 feature boundary 转换 Go snake_case，未生成不存在的计数、奖励名称、进度或成功结果。
- 错误状态：通过。空数据、401、无效响应、活动失效、网络/业务错误重试均有明确页面状态。
- 窄屏基础：通过。复用共享 mobile CSS，关键链接最小高度 44px，长 URL 使用 `overflow-wrap`，390px 样式无固定宽度。

## 7. 疑虑与后续边界

- 本任务按约束只完成非 Docker 的静态门禁与 jsdom 单元测试；未连接真实 Go、企业微信 OAuth、数据库，也未做浏览器 390px/桌面矩阵，因此不宣称真实 OAuth 或生产业务闭环已验收。
- 其余 9 条 Operation 历史路由仅完成显式接管和诚实待迁移状态，不代表业务功能已迁移。
- `rewardLabel` 类型修正是依据当前 Go 源码合同；若后端未来新增受信任的奖励展示名称，应另行扩展合同和测试，而不是由前端推断。

## 8. Review 安全修复：奖励 URL allowlist

Task 3 review 发现 Important 问题：原 adapter 将非空 `gift_url` 原样交给页面 `href`，未阻止 `javascript:`、`data:` 或其他不受信任 scheme。

按 TDD 新增 13 个 URL 合同用例，RED 为：

```text
TypeError: safeRewardUrl is not a function
Test Files 1 failed | 4 passed
Tests 13 failed | 32 passed
```

随后在 `work-fission-api.ts` feature boundary 实现无 React runtime 依赖的 `safeRewardUrl`：

- 允许有效 `https://`、`http://` URL。
- 允许单斜杠开头的同源绝对路径，例如 `/static/rewards/gift.png`。
- 空值、protocol-relative `//evil.example`、`javascript:`、`data:`、custom scheme、反斜线、控制字符、编码反斜线及畸形 URL 均返回 `null`。
- adapter 只把 `safeRewardUrl(task.gift_url)` 放入稳定模型，因此页面仅会为 allowlist URL 生成链接。

GREEN：

```text
src/features/work-fission/work-fission-page.test.tsx: 28 passed
Operation total: 5 files, 45 tests passed
```
