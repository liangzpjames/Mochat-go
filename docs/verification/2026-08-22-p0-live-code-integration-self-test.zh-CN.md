# P0 活码集成自测报告

日期：2026-08-22

分支：`feat/p0-live-code-integration-20260822`

## 1. 自动化门禁

| 项目 | 命令/证据 | 结果 |
| --- | --- | --- |
| 活码组件回归 | `vitest run src/features/phase34/acquisition-pages.test.tsx` | PASS，21/21 |
| Dashboard 全量测试 | `corepack pnpm --filter @mochat/dashboard test` | PASS，137/137 文件、779/779 用例 |
| Dashboard typecheck | `corepack pnpm --filter @mochat/dashboard typecheck` | PASS |
| Dashboard production build | `corepack pnpm --filter @mochat/dashboard build` | PASS，1787 modules transformed |
| Sidebar/Operation production build | 两个 workspace 的 `build` | PASS；为移动端截图门禁生成隔离 worktree 的忽略产物 |
| 相关 Go 五包 | `go test ./internal/migration ./internal/dashboard ./internal/store ./internal/server ./cmd/mochat-go -count=1` | PASS |
| Dashboard 页面 RBAC | `corepack pnpm check:phase4-dashboard-page-rbac` | PASS，53 页、48 ordinary、5 superadmin-only、0 unmapped，`scopeRequired=132` |
| 圆弧 benchmark | `corepack pnpm check:yuanhu-benchmark` | PASS，53 pages |
| Dashboard evidence 合同 | `corepack pnpm check:dashboard-all-pages-evidence` | PASS，12/12 |
| 移动端基础 | `corepack pnpm check:mobile-clients-foundation` | PASS，57/57；Sidebar 12 路由、Operation 10 路由 |
| Provider completion | `corepack pnpm check:provider-completion` | PASS，21/21 合同测试及相关 Go 合同通过 |
| 迁移编号审计 | 0140—0154 up/down 文件分组检查 | PASS，每个编号恰好一对，无重复占用 |
| Git 空白检查 | `git diff --check` | PASS |

移动端门禁首次及首次隔离重跑均在等待截图夹具联系人“林小青”时超时。根因是独立 worktree 尚无被 Git 忽略的 `web/apps/sidebar/dist` 与 `web/apps/operation/dist`，本地证据服务器返回 404 空页。构建两个正式产物后，在不修改校验器与 30 秒超时的情况下原样重跑通过 57/57。

## 2. MariaDB 与 Docker

| 项目 | 结果 |
| --- | --- |
| `MOCHAT_GO_MYSQL_INTEGRATION_DSN` 专用隔离套件 | SKIP：环境变量未设置，门禁明确输出 SKIP |
| 保留卷 Docker MariaDB 实际迁移 | PASS：0152、0153、0154 均应用成功，checksum 长度均为 64 |
| 历史 0150 兼容 | PASS：保留历史迁移账本的数据卷可升级到 0153/0154 |
| RBAC 数据读回 | PASS：customer inheritance=8，channel update=1 |
| 容器健康 | PASS：app、mysql、redis 均 healthy |
| 命名卷保护 | PASS：四个命名卷均存在，未删除 |

## 3. Docker 浏览器验收

通过应用内浏览器在 `http://localhost:18080` 使用真实登录表单验收。宽屏与桌面视口分别为 2560×1440、1440×900；移动端通过校准浏览器外框得到实际 `clientWidth=390`、`clientHeight=844`。

| 页面/状态 | 2560×1440 | 1440×900 | 390×844 |
| --- | --- | --- | --- |
| 渠道活码列表与真实空态 | PASS | PASS | PASS |
| 渠道活码新建抽屉 | PASS | PASS | PASS；`scrollWidth=clientWidth=390` |
| 渠道活码必填校验 | PASS：名称/成员为必填，未完成时保存按钮 disabled | PASS | PASS |
| 群活码列表与真实空态 | PASS | PASS | PASS |
| 群活码新建抽屉 | PASS | PASS | PASS；等待抽屉动画结束后无裁切，`scrollWidth=clientWidth=390` |
| 群活码必填校验 | PASS：名称/群聊为必填，未完成时保存按钮 disabled | PASS | PASS |
| 编辑入口 | SKIP：真实列表为 0 条，且用户要求不再修改权威页面；未注入伪造记录 |
| Provider 失败提交 | SKIP：避免通过真实表单制造业务写入；组件测试覆盖错误态 |

验收期间观察到抽屉进入动画尚未结束时截图会呈现瞬时左移；等待约 1 秒后布局稳定，实际 390px 内容宽度无横向溢出或标题裁切。两页均保持 Mochat 现有导航、卡片、抽屉与按钮语言，未复制圆弧视觉资产。

## 4. 最终状态

- PASS：所有可控代码、构建、合同、Docker 迁移和最新用户范围内的浏览器门禁。
- FAIL：无最终失败项。
- SKIP：仅缺少 `MOCHAT_GO_MYSQL_INTEGRATION_DSN` 的专用 MariaDB 隔离套件，以及因真实列表为空且禁止伪造数据而无法执行的编辑/Provider 失败写入浏览器步骤。
- 外部阻塞：仅上述 DSN；真实保留卷 MariaDB 已提供本次迁移的补充实证。
