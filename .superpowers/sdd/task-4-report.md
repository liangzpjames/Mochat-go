# Task 4 完成报告：Completion gate 与 22 路由浏览器验收

## 1. 状态与范围

- 状态：完成。
- 工作目录：`D:\workspace\mochat-go\mochat-go\.worktrees\mobile-clients-foundation`。
- 分支：`phase5/mobile-clients-foundation`。
- Task 4 只新增 completion gate、gate fixture 测试、Playwright 浏览器矩阵与两个 package script。
- `cmd/mochat-frontend-e2e/main.go` 已具备 `/sidebar-app/`、`/operation-app/` 静态挂载能力，未修改。
- 未操作 Docker、Compose、服务器、数据库、数据卷或 `output`。
- 主工作区已有 `Dockerfile`、Compose、迁移、部署脚本等 dirty/untracked 文件；本任务未写入这些文件。

## 2. TDD RED→GREEN

### 2.1 初始 RED

先创建 `scripts/check_mobile_clients_foundation.test.mjs`，再运行：

```powershell
node --test scripts/check_mobile_clients_foundation.test.mjs
```

结果：退出码 `1`，`ERR_MODULE_NOT_FOUND`，原因是 `scripts/check_mobile_clients_foundation.mjs` 尚不存在。

加入仅返回固定成功统计的可导入骨架后再次运行同一命令：

```text
tests 16
pass 2
fail 14
```

14 个坏 fixture 分别以 `Missing expected exception` 失败，证明以下缺陷在实现前均未被 gate 捕获：

1. Sidebar 只有 11 条路由。
2. Operation 只有 9 条路由。
3. registry 出现 manifest 不存在的路由。
4. 生产页面直接调用 `fetch`。
5. Sidebar 读取 Dashboard storage key。
6. Operation 读取 Dashboard storage key。
7. 乱码标记。
8. 伪成功文案。
9. 随机奖品。
10. 固定进度。
11. 缺少 `390×844` viewport。
12. 未知路由回落首页。
13. 根 package script 缺失。
14. E2E package script 缺失。

测试另含一个有效 fixture，以及一个“测试源码中的 `fetch` 不算生产源码”的边界用例，因此总数为 16。

固定进度检测随后又做了一次更严格的 RED：fixture 从特制名称 `fixedProgress` 改为普通 `const progress = 66`，定向测试先以 `Missing expected exception` 失败；扩展检测后定向测试转为 `1/1` 通过。

### 2.2 GREEN

实现 source-derived gate 后：

```powershell
node --test scripts/check_mobile_clients_foundation.test.mjs
```

结果：退出码 `0`，`16/16` 通过。

最终命令：

```powershell
corepack pnpm check:mobile-clients-foundation
```

关键输出：

```text
sidebar routes=12
operation routes=10
direct fetch=0
dashboard session references=0
mojibake markers=0
fake business outcomes=0
mobile viewport cases>=22
tests 16
pass 16
fail 0
```

gate 独立读取两个 `migration-routes.json`，从生产 `registry.tsx` 的 `routeDefinitions` 解析路径字面量，递归扫描两个 app 的生产 `.ts/.tsx`，排除测试、fixture、构建目录，并校验两个 package script、双 viewport、全路由浏览器 case 与未知路由 404 约束。

## 3. 浏览器验收

### 3.1 启动前构建

Playwright 使用 Go 静态 E2E server 读取 production dist，因此先执行：

```powershell
corepack pnpm --filter @mochat/mobile-foundation build
corepack pnpm --filter @mochat/sidebar build
corepack pnpm --filter @mochat/operation build
```

三条命令均退出 `0`。Sidebar 构建 `35 modules transformed`，Operation 构建 `35 modules transformed`。

### 3.2 矩阵与请求 fixture

- viewport：`390×844`、`1280×900`。
- 每个 viewport：12 条 Sidebar、10 条 Operation、Sidebar/Operation 各 1 条未知路由，共 24 个用例。
- 总数：`2 × (12 + 10 + 2) = 48`。
- protected Sidebar route 由当前浏览器 context 注入自身 `token=sidebar-browser-token` 与 `agentId=7` cookie；未读写任何 Dashboard storage。
- `/sidebar/workContact/detail` mock 返回原始 Go envelope，`data` 为后端 camelCase 联系人模型。
- `/operation/workFission/taskData` mock 返回原始 Go envelope，`data` 保留 `invite_count`、`differ_count`、`end_time`、`task[].receive_status` 等 snake_case `taskData`；最终 view model 由生产 `work-fission-api.ts` 转换，测试未预计算。
- 其他待迁移路由不发业务请求；任何未登记的 `/sidebar/**` 或 `/operation/**` 请求都会被记入错误并返回 `500`，从而使验收失败。

每条已知路由检查：

- 一级标题与明确模块标签可见；
- `main` 可见且文本非空；
- `document.documentElement.scrollWidth <= clientWidth`；
- 当前页面存在的主操作/重试控件可见且高度不少于 `44px`；
- console error、page error、`requestfailed`、意外 API 请求、意外 `4xx/5xx` 均为空。

每个未知路由检查：

- 可见“页面不存在”标题；
- 不呈现对应 app 首页一级标题；
- 同样满足非空、无横向溢出及错误采集为零。

最终运行：

```powershell
corepack pnpm --filter @mochat/e2e test:mobile-clients-foundation
```

结果：退出码 `0`，`Running 48 tests using 1 worker`，`48 passed (10.3s)`。runner 正常退出，没有挂起。输出仅有 Playwright 关于 `NO_COLOR` 被 `FORCE_COLOR` 覆盖的环境警告，不是页面 console error。

## 4. 全部非 Docker gates

以下命令均退出 `0`：

```powershell
corepack pnpm --filter @mochat/mobile-foundation lint
corepack pnpm --filter @mochat/mobile-foundation typecheck
corepack pnpm --filter @mochat/mobile-foundation test
corepack pnpm --filter @mochat/mobile-foundation build
corepack pnpm --filter @mochat/sidebar lint
corepack pnpm --filter @mochat/sidebar typecheck
corepack pnpm --filter @mochat/sidebar test
corepack pnpm --filter @mochat/sidebar build
corepack pnpm --filter @mochat/operation lint
corepack pnpm --filter @mochat/operation typecheck
corepack pnpm --filter @mochat/operation test
corepack pnpm --filter @mochat/operation build
corepack pnpm --filter @mochat/e2e typecheck
corepack pnpm --filter @mochat/e2e lint
corepack pnpm --filter @mochat/e2e test:mobile-clients-foundation
corepack pnpm check:mobile-clients-foundation
```

单测精确数量：

- `@mochat/mobile-foundation`：`21/21`。
- `@mochat/sidebar`：`31/31`。
- `@mochat/operation`：`45/45`。
- 三包合计：`97/97`。
- completion gate fixtures：`16/16`。
- Playwright：`48/48`。

`git diff --check main...HEAD` 在 Task 4 提交前发现 Task1-3 已提交的两份新增文档各多一个 EOF 空行：

```text
docs/superpowers/plans/2026-08-14-mobile-clients-foundation.md:435: new blank line at EOF.
docs/superpowers/specs/2026-08-14-mobile-clients-foundation-design.md:118: new blank line at EOF.
```

Task 4 只删除这两个尾部空行；由于 `main...HEAD` 只检查已提交内容，最终结果在 Task 4 commit 后复跑记录。Task 4 工作树的 `git diff --check` 在提交前已退出 `0`。

## 5. 自审

- completion gate 的成功计数来自 manifest、registry、生产源码和 E2E 源码，不使用手工常量伪造最终结果。
- manifest 与 registry 双向比较，能捕获 registry 多路由和 manifest 漏注册。
- 生产扫描排除 `*.test.*`、`*.spec.*`、fixture、build/dist/coverage 等非生产证据。
- 未修改 Sidebar、Operation、mobile-foundation 的 Task1-3 生产逻辑；浏览器未暴露需补 RED 的生产缺陷。
- E2E fallback route 在具体 API route 之前注册、具体 route 后注册；Playwright 使用后注册者优先，因此已知 API 命中原始 envelope fixture，其他 API 必定失败。
- `cmd/mochat-frontend-e2e/main.go` 无无谓改动。
- Task 4 初始自审未发现 Critical 或 Important 问题；父线程后续独立 review 发现的两个 Important 及修复证据见第 7 节。

## 6. 边界与疑虑

- 本次 22 路由验收证明 URL 承接、页面状态、两个真实纵切面 mock 契约与移动布局；不证明其余 20 个待迁移模块已经实现真实业务闭环。
- 未连接真实 OAuth、Go API 或数据库；这符合设计中“本阶段非 Docker、确定性 raw envelope browser fixture”的范围。
- 主工作区在任务开始/结束时均有既存 dirty/untracked 文件；本任务未清理、覆盖或提交它们。

## 7. 父线程 Review Important 修复追加记录

### 7.1 Review 结论

父线程在 `0f697c1` 后发现两个 Important：

1. 原 completion gate 只统计 E2E case/viewport 声明，不能阻止删除 raw Go envelope、错误采集、overflow、44px 或真实双 viewport 循环。
2. 原 E2E 在页面断言后立即检查 audit，并且只检查“已有控件”的高度；迟到的网络事件可能漏报，`/login` 控件被删除时也可能因为控件数为 `0` 而通过。

修复保持 `0f697c1` 不变，使用独立追加 commit。

### 7.2 Review RED

先只扩展坏 fixture，再执行：

```powershell
node --test scripts/check_mobile_clients_foundation.test.mjs
```

结果：退出码 `1`，精确汇总：

```text
tests 26
pass 16
fail 10
```

新增 10 个 fixture 均以 `Missing expected exception` 独立失败：

1. contact route 不使用 raw Go envelope。
2. taskData route 不使用 raw Go envelope。
3. 缺少 `requestfailed` audit collector。
4. 缺少 `unexpectedRequests` 最终空数组断言。
5. 缺少 `response.status() >= 400` collector。
6. 缺少 `scrollWidth <= clientWidth` 断言。
7. 缺少 `expectsAction` 的强制控件数断言。
8. `/login` 没有任何 `expectsAction: true` case。
9. viewport 声明仍有两个，但路由循环只运行 mobile。
10. clean audit 前缺少 `networkidle` 稳定点。

### 7.3 Gate GREEN

completion gate 现在交叉验证：

- `browserContract.fixtures.contact` 与 `browserContract.fixtures.taskData` endpoint；
- `rawGoEnvelope` 保留原始 `code`、`msg`、`data`，且两个 `page.route` 确实调用它；
- contact raw data 保留 `id/name/avatar/corpId`；taskData 保留 `invite_count/differ_count/end_time/task/receive_status/gift_type/gift_url`，不预计算最终 view model；
- console error、`pageerror`、`requestfailed`、`response >= 400`、unexpected request 的 collector 和最终空数组断言；
- `scrollWidth <= clientWidth`；
- 每条 route case 都声明 `expectsAction`，`/login=true`，且 true case 强制 `controls.count() >= 1`；所有可见控件高度使用 `minimumActionHeight: 44`；
- 精确的 `for (const viewport of viewports)` 外层 block 内同时存在完整 Sidebar 与 Operation case loop；两个 loop 均调用布局断言和稳定 clean audit；
- stable audit 先等待 `networkidle`，再等待一个事件循环，最后执行 clean assertion；两个 unknown route 同样走 stable audit。

最终 fixture：

```powershell
node --test scripts/check_mobile_clients_foundation.test.mjs
```

结果：退出码 `0`，`26/26` 通过。

真实源码 gate：

```powershell
node scripts/check_mobile_clients_foundation.mjs
```

输出保持：

```text
sidebar routes=12
operation routes=10
direct fetch=0
dashboard session references=0
mojibake markers=0
fake business outcomes=0
mobile viewport cases>=22
```

### 7.4 E2E GREEN

每条 route case 新增显式 `expectsAction`；`/login` 为 `true`。`assertVisiblePage` 对 true case 强制至少一个可见主/重试控件，并对所有找到的控件验证不小于 `44px`。

`assertStableCleanAudit` 在最终 audit 前执行：

```text
await page.waitForLoadState('networkidle')
await page.waitForTimeout(0)
assertCleanAudit(...)
```

最终执行：

```powershell
corepack pnpm --filter @mochat/e2e test:mobile-clients-foundation
```

结果：退出码 `0`，`48 passed (31.6s)`；runner 正常结束，没有挂起。

同时通过：

```powershell
corepack pnpm --filter @mochat/e2e typecheck
corepack pnpm --filter @mochat/e2e lint
```

两条命令均退出 `0`。
