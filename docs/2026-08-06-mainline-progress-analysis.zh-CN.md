# 2026-08-06 MoChat Go 主线工作进度分析

> 生成时间：2026-08-06 18:11（Asia/Shanghai）
> 数据来源：`git status`/`git log`、`web/apps/dashboard/src/benchmark/manifest.json`、阶段文档、门禁实测（`go test` / `pnpm` / `node --test`）、Docker 运行时与数据库查询
> 说明：本文档只做分析记录，不修改任何代码；文内数字均来自本次实测，证据路径见文末索引。

> **2026-08-06 晚更新**：文中“阻塞与风险”第 1–3 项已解决——迁移常量与 0120 编号冲突已修复（`0120_phase35_acceptance_lifecycle` 重排为 `0122`，新增 `0123_phase35_order_collation_align`，常量更新为 123），部署已重建至最新迁移，九页浏览器验收与跨页工作流证据已闭合。最新状态见 [功能矩阵](phases/phase-3-dashboard/phase-3.5/reports/phase3.5-function-matrix.md) 与 [总验收报告](phases/phase-3-dashboard/phase-3.5/reports/phase3.5-total-acceptance-report.md)。

## 一、总体结论

1. **当前主线**是 `feature/phase3.5-scrm-reporting`（HEAD `0bedd12`，2026-08-06 15:33），领先 `main` 53 个提交、0 落后，尚未推送远端。
2. **代码实现完成度高**：Phase 3.5 的 9 个路由全部 `native` 实现；定向 Go 测试、Dashboard 全量 Vitest（475 个）、typecheck、生产构建、Phase 3.5 门禁（9/9）本次全部通过。
3. **阶段验收未闭合**：`go test ./...` 全量失败（仅 1 个迁移期望常量滞后）；运行中的 `mochat-go-desktop` 部署落后 HEAD 两个迁移；现有浏览器截图基于旧构建，最后提交之后没有复验；功能矩阵、计划复选框和总进度文档均滞后。

## 二、仓库与分支拓扑

| 分支/工作树 | HEAD | 说明 |
| --- | --- | --- |
| `main` | `5496f1a`（08-04 23:59） | 已合入至 Phase 3.4 返工验收，最近提交为 Phase 3.4 验收记录 |
| `feature/phase3.5-scrm-reporting`（当前检出） | `0bedd12`（08-06 15:33） | 领先 `main` 53 提交、0 落后；无远端跟踪分支 |
| `.worktrees/frontend-unification-foundation` | `78f4218` | `phase1/frontend-unification-foundation`，未合入 |
| `.worktrees/phase2-1-functional-frontend` | `eb25f33` | `phase2/functional-frontend`，未合入 |
| `.worktrees/phase2-page-metadata-overrides` | `e44a695` | `phase2/page-metadata-overrides`，未合入 |
| `.worktrees/yuanhu-benchmark-phase1` | `149bbe6` | `phase3/yuanhu-benchmark-phase1`，未合入 |

远端存在 `origin/main`、`origin/backup/pre-phase0-main-20260723` 及历史阶段分支；`feature/phase3.5-scrm-reporting` 只在本地。

## 三、阶段总览

| 阶段 | 状态 | 证据要点 |
| --- | --- | --- |
| Phase Pre-0 / 0 / 1 / 2 / 2.1 / 2.2 | 已合入 `main`（历史阶段） | 独立部署、Go 底座、React 统一底座、迁移治理、后端质量门禁 |
| Phase 3.1 / 3.2 / 3.3 | 已合入 `main` | 3.2 八页门禁通过；3.3 菜单合入，`/chat/file-audio` 明确保持未完成 |
| Phase 3.4 | 已合入 `main` | 9 页全部 `native`，`screenshotVersion: 2026-08-04`，总验收已记录 |
| Phase 3.5 | **进行中（本分支）** | 9 页代码完成度高；浏览器/部署/全量门禁未闭合 |

### Benchmark manifest 口径（53 页）

| Phase | 页面数 | 实现分布 | 说明 |
| --- | --- | --- | --- |
| 3.1 | 27 | 6 `demo`、21 `placeholder` | 未达到产品交付口径 |
| 3.2 | 8 | 6 `native`、2 `legacy-adapter` | 八页门禁通过 |
| 3.4 | 9 | 9 `native` | 已合入 |
| 3.5 | 9 | 9 `native` | 本分支 |
| 合计 | 53 | 26 达标 / 27 未达标 | `native`+`legacy-adapter`=26；`demo`+`placeholder`=27 |

验收分布：`integration-passed` 26、`unit-passed` 4、`e2e-passed` 2、`not-started` 21。

## 四、主线 Phase 3.5 详细分析

### 4.1 目标与范围

固定范围为 9 个路由：`/customer/friends`、`/customer/group`、`/customer/order`、`/customer/settings`、`/data/customer`、`/data/employee`、`/data/conversion`、`/data/behavior`、`/data/report`。

返工设计（`docs/phases/phase-3-dashboard/phase-3.5/design/2026-08-05-phase3.5-productization-design.md`）明确：页面不得直出内部字段，需要中文指标口径、趋势/漏斗、详情、分页、状态反馈、跨页下钻和 Provider 缺失时的结构化 `limitations`；路由可达、HTTP 200、空表格、静态指标和测试 fixture 都不算完成证据。

### 4.2 提交结构与改动规模

53 个提交分两波：

- **初始实施（08-05 20:23–23:13）**：reporting 合同、好友/群、订单/设置、五类报表、Docker 部署脚本修复（`deploy_docker_desktop.ps1` 等）。
- **产品化返工（08-05 23:39–08-06 15:33）**：统一展示层与中文字典、订单完整工作流（联系人搜索选择/快速建联系人/状态流转/审计时间线/乐观锁）、版本化客户设置、报表产品化、验收生命周期（`P35-ACCEPT-` 隔离）、最后提交 `0bedd12` 将订单审计写入改为原子操作。

相对 `main`：106 个文件，+4137/−75。分布：`web/apps` 39、`internal/modules` 33、`deploy/standalone` 8、`docs/phases` 8、`scripts` 约 10、`cmd` 3、`internal/dashboard|migration|config|app` 各 1、`web/packages` 2、`package.json` 1。

### 4.3 实现与测试证据（本次实测）

- `web/apps/dashboard/src/features/phase35/`：36 个源文件（含 11 个测试文件），覆盖 9 页、共享组件、展示层与查询。
- Go：`internal/modules/reporting` 5 个测试文件、`internal/modules/scrm` 31 个测试文件（含订单仓库集成测试）。
- 页面实现抽查：订单页具备联系人搜索选择（不要求输入内部 ID）、无联系人时快速创建并自动选中、金额格式化、合法状态流转、版本冲突提示、审计时间线；展示层为中文指标字典（`presentation/labels.ts`）。
- 验收数据脚本：`scripts/phase35_acceptance_data.ps1`（create/verify/cleanup，只命中 `P35-ACCEPT-` 前缀）及其 node 测试已存在。

### 4.4 文档与记录状态（滞后点）

| 文档 | 状态 |
| --- | --- |
| 产品化设计 / 实施计划 | 已建立 |
| `phase3.5-function-matrix.md` | **滞后**：order/settings/5 个报表仍标“未完成”，与已提交代码不符 |
| `phase3.5-total-acceptance-report.md` / `PHASE3.5_ACCEPTANCE.zh-CN.md` | 记录“非浏览器门禁阶段性通过，整体未完成”，阻塞项为 Docker 快照、九页截图、跨页下钻、生产数据库集成 |
| 两份计划（scrm-reporting / productization） | 37 个复选框全部未勾选，与实际进度不同步 |
| `docs/PROJECT_PROGRESS.zh-CN.md` | 停留在 2026-07-28，描述 Phase 2 尚未开始，与现状差距很大 |

### 4.5 验收证据现状

- `output/phase35-browser-20260805/`：返工前 9 张截图（08-05 22:36 前）。
- `output/phase3.5-review-20260806/`：`final-*` 9 张（46–58KB）加 `workflow-contact-created.png`、`10-settings-created.png`（08-06 14:56–15:05）；另有 6.8KB 的 `recheck-*` 系列，疑似加载失败/空白页，需要人工确认。
- **截图基于旧构建**：`mochat-go-desktop-app-1` 启动于 13:47，早于最后提交 `0bedd12`（15:33）；运行数据库只应用到 `0120_saas_tenant_default_corp_reconcile`，尚未应用 `0120_phase35_acceptance_lifecycle` 与 `0121_phase35_order_productization` 两个迁移。
- 验收账户文档已生成（`PHASE3.5-ACCEPTANCE-ACCOUNT.zh-CN.md`），通过 `cmd/mochat-bootstrap` 幂等写入现有租户，未重建/删除容器与数据卷。

## 五、质量门禁实测结果（2026-08-06 18:0x）

| 命令 | 结果 | 备注 |
| --- | --- | --- |
| `go test ./internal/modules/reporting/... ./internal/modules/scrm/... -count=1` | 通过 | 8 个包 ok |
| `go test ./... -count=1` | **失败** | 仅 `internal/dashboard` 1 个测试失败（见第六节） |
| `pnpm --filter @mochat/dashboard typecheck` | 通过 | |
| `pnpm --filter @mochat/dashboard test` | 通过 | 81 个文件、475 个测试 |
| `pnpm --filter @mochat/dashboard build` | 通过 | 生产构建 |
| `pnpm check:phase3-5-dashboard` | 通过 | 9/9 路由过实现门禁（仅校验 manifest） |
| `node --test scripts/check_phase3_5_dashboard_completion.test.mjs` | 通过 | 6/6 |
| `git diff --check`（main...HEAD 与工作区） | 干净 | |
| `docker ps` | app/mysql/redis 均 healthy | app 启动 13:47，端口 18080/18081/18082 |

## 六、已知问题与风险（按优先级）

1. **`go test ./...` 全量失败（阻断合入）**：`internal/dashboard/saas_admin_system_health.go` 中 `SaaSAdminExpectedMigrationCount = 120`，实际发现 122；`SaaSAdminExpectedMigrationVersion = "0120_saas_tenant_default_corp_reconcile"`，实际最后一个为 `0121_phase35_order_productization`。原因：常量只同步到中途（加了 0119 与 0120_reconcile 时更新为 120），后续又新增 `0120_phase35_acceptance_lifecycle` 与 `0121_phase35_order_productization` 未再更新。
2. **迁移编号冲突**：`0120_saas_tenant_default_corp_reconcile` 与 `0120_phase35_acceptance_lifecycle` 同为 `0120` 前缀。加载器按完整文件名字符串区分，不会报重复，但字典序会把 `phase35_acceptance_lifecycle` 排在 `saas_tenant_default_corp_reconcile` 之前；且运行库已应用 `0120_saas_tenant_default_corp_reconcile`，改编号前必须先确认 checksum/幂等策略。
3. **运行部署滞后 HEAD**：运行库缺 `0120_phase35_acceptance_lifecycle`、`0121_phase35_order_productization`；最后提交（订单审计原子化）之后无浏览器复验。
4. **浏览器验收口径**：`final-*` 截图基于旧构建；`recheck-*` 疑似失败页；需重建容器（保留 volume、禁止 `down -v`）后逐页复验并更新截图目录。
5. **文档滞后**：功能矩阵、计划复选框、`PROJECT_PROGRESS.zh-CN.md` 未同步，会误导对“完成”的判断。
6. **未跟踪产物**：`web/saas-admin/`（`dist`+`node_modules`，11580 文件 / 约 141MB，旧计划注明为既有内容勿删）、`.gocache-phase35-review/`、`.workbuddy/`、`docs/superpowers/plans/2026-07-29-phase3-frontend-acceptance.md`（未入库）。
7. **历史工作树分支**：4 个 feature 分支未合入 `main`，属阶段性隔离工作区。
8. **Vite dev 日志警告**：`vite-recheck.err.log` 出现 `createRoot()` 重复调用警告（开发态 HMR 场景，生产构建正常，建议后续排查）。

## 七、建议下一步（按优先级）

1. **修复迁移期望常量**：把 `SaaSAdminExpectedMigrationCount` 更新为 122、`SaaSAdminExpectedMigrationVersion` 更新为 `0121_phase35_order_productization`；同时复核 0120 编号冲突是否需要在合入前重排（涉及已应用迁移，需先评估）。
2. **重建并复验部署**：用 `scripts/deploy_docker_desktop.ps1` 重建 `mochat-go-desktop`（部署前记录 volume 快照，不删除 volume），确认两个新迁移应用、`/readyz` 与容器健康。
3. **逐页浏览器验收**：对 9 页执行筛选、详情、写操作、刷新、空/错误/受限状态验收，并跑通“联系人/商机/订单 → 客户或转化报表 → 明细下钻”真实链路；截图输出到 `D:\workspace\mochat-go\output\phase35-browser-20260806`。
4. **同步文档**：更新功能矩阵（9 页逐行）、勾选计划、写总验收报告，再更新 `docs/PROJECT_PROGRESS.zh-CN.md`。
5. **全量门禁复跑**：`go test ./...`、Dashboard Vitest、typecheck、build、`pnpm check:phase3-5-dashboard`、`git diff --check` 全部通过后再合入 `main`。
6. **清理确认**：`web/saas-admin` 与 `.gocache-phase35-review` 建议由用户决定入库/忽略/删除；工具产物 `.workbuddy` 与未入库计划文件建议归档或忽略。

## 八、证据索引

- 门禁脚本：`scripts/check_phase3_5_dashboard_completion.mjs`（含 `.test.mjs`）
- 部署脚本：`scripts/deploy_docker_desktop.ps1`、`scripts/test_deploy_docker_desktop.ps1`
- 验收数据：`scripts/phase35_acceptance_data.ps1`、`scripts/phase35_acceptance_data.test.mjs`
- 设计/计划：`docs/phases/phase-3-dashboard/phase-3.5/design/`、`.../plans/`、`.../reports/reuse-audit.md`
- 验收记录：`docs/phases/phase-3-dashboard/phase-3.5/acceptance/PHASE3.5_ACCEPTANCE.zh-CN.md`、`.../reports/phase3.5-total-acceptance-report.md`
- 截图：`D:\workspace\mochat-go\output\phase35-browser-20260805\`、`D:\workspace\mochat-go\output\phase3.5-review-20260806\`
- 验收账户：`D:\workspace\mochat-go\output\phase3.5-review-20260805\PHASE3.5-ACCEPTANCE-ACCOUNT.zh-CN.md`
- 迁移：`deploy/standalone/migrations/0119_phase35_orders_settings.*`、`0120_saas_tenant_default_corp_reconcile.*`、`0121_phase35_order_productization.*`、`0122_phase35_acceptance_lifecycle.*`、`0123_phase35_order_collation_align.*`
