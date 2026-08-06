# 主目标盘点与 Dashboard 初步建设后的执行路线

> 生成时间：2026-08-06（Asia/Shanghai）
> 依据：`docs/phases/phase-3-dashboard/benchmark/README.md`、`web/apps/dashboard/src/benchmark/manifest.json`、阶段文档、2026-08-06 门禁实测
> 定位：本文档回答三个问题——项目的主目标是什么；Dashboard 初步建设完成后每一步如何执行；哪些地方还欠缺、需要补充什么。

## 一、主目标盘点

### 1.1 产品目标（圆弧基准）

把“企微数据采集 → 客户经营 → AI 洞察 → 风险治理 → 报表复盘”放进同一个后台，四条主链：

1. **会话存档与检索**：员工/客户/群消息的归档、全局检索、轨迹、导出、离职继承。
2. **风险与 AI 质检**：风险行为、敏感词、超时、流失、情绪、员工评分、关键词洞察。
3. **获客与客户经营**：活码、获客链接、群发、朋友圈、素材，衔接线索/联系人/商机/公海/订单。
4. **数据与治理**：客户/会话/转化/行为报表，配套知识库、智能体、员工、角色、授权管理。

落地为 8 个领域 53 个页面（manifest 是唯一事实源）：

| 领域 | 页数 | 已达标 | 未达标 | 未达标构成 |
| --- | ---: | ---: | ---: | --- |
| 数据概览 | 1 | 1 | 0 | — |
| 会话 | 10 | 1 | 9 | 3 `demo` + 6 `placeholder` |
| 风险预警 | 7 | 1 | 6 | 2 `demo` + 4 `placeholder` |
| AI 洞察 | 5 | 0 | 5 | 1 `demo` + 4 `placeholder` |
| 营销工具 | 9 | 9 | 0 | — |
| SCRM | 9 | 9 | 0 | — |
| 数据报表 | 5 | 5 | 0 | — |
| AI 设置 | 2 | 0 | 2 | 2 `placeholder` |
| 企业设置 | 5 | 0 | 5 | 5 `placeholder` |
| 合计 | 53 | 26 | 27 | 6 `demo` + 21 `placeholder` |

### 1.2 工程目标

- **Go 后端**：模块化单体（`internal/modules/<domain>`，依赖方向 transport/adapters → application → domain），MariaDB/MySQL 5.7 兼容，质量门禁与 GitHub Actions 覆盖。
- **前端统一**：pnpm workspace，React 19 + TypeScript 严格模式，Dashboard → Sidebar → Operation 按“先等价迁移、后体验升级”推进，未迁移路由走受控兼容层。
- **交付载体**：`mochat-go-desktop` Docker Compose 独立部署（保留 volume），验收以真实浏览器交互、截图、API 回读为准。
- **完成口径**：页面必须 `native`/`legacy-adapter` 且 `backend=ready`、验收通过；`demo`/`placeholder`/fixture/仅路由可达均不计数；每个页面要同时满足 API 合同、权限、持久化、自动化测试与证据闭环。

### 1.3 业务闭环目标

不是“135 条路由都通”，而是真实链路可走通：活码投放 → 客户进入联系人 → 商机推进 → 订单成交 → 转化漏斗与综合报表按同一口径更新 → 设置/行为可审计。Phase 3.5 已建立这条链的雏形，尚未留下端到端证据。

## 二、当前基线（2026-08-06）

- `main` 已合入至 Phase 3.4（9 页营销工具 native，截图 2026-08-04）。
- 当前主线 `feature/phase3.5-scrm-reporting`：9 页代码与自动化完成（475 个 Dashboard 测试、typecheck、build、9/9 门禁通过），但 `go test ./...` 1 个失败、部署缺 2 个迁移、浏览器证据基于旧构建。
- 基准总体：26/53 达标；会话/风险/AI/设置领域是主要缺口。
- 历史工作树：phase1/phase2×2/phase3.1 四个分支未合入，其中 phase2 承载 Sidebar/Operation 迁移路线。

## 三、Dashboard 初步建设之后的分步执行路线

### Step 0：收尾当前主线 Phase 3.5（先做，1–2 天）

**目标**：把“代码完成”变成“可合入 `main` 的阶段完成”。

1. 修复 `internal/dashboard` 迁移期望常量（120→122，版本指向 `0121_phase35_order_productization`），复核 0120 编号冲突（`0120_saas_tenant_default_corp_reconcile` vs `0120_phase35_acceptance_lifecycle`）是否需要重排；复跑 `go test ./...` 全绿。
2. 记录 volume 快照后用 `scripts/deploy_docker_desktop.ps1` 重建 `mochat-go-desktop`，确认两个新迁移应用、`/readyz` 与三容器健康；禁止 `down -v`。
3. 对九页逐页浏览器验收（菜单、直接 URL、筛选、详情、写操作、刷新、空/错误/受限状态），跑通“联系人/商机/订单 → 转化漏斗 → 明细下钻”真实链路，记录数据 ID 与 API 响应；截图输出到 `D:\workspace\mochat-go\output\phase35-browser-20260806`。
4. 用 `phase35_acceptance_data.ps1` 执行一次 `P35-ACCEPT-*` create/verify/cleanup，留下可追溯证据。
5. 同步矩阵/计划/验收报告后合入 `main`。

> **2026-08-06 晚状态**：第 1–3 项已闭环（常量更新为 123，`0120_phase35_acceptance_lifecycle` 重排为 `0122`，新增 `0123` collation 对齐迁移；重建部署后九页截图 + 视觉模型验收与订单跨页工作流通过）。第 4 项（`P35-ACCEPT` 生命周期留证）与合入 `main` 待执行。

**完成口径**：全量门禁绿、九页截图、跨页工作流证据、`P35-ACCEPT` 生命周期证据、阶段标完成。

### Step 1：补齐 6 个 `demo` 页（次做）

**目标**：把“能看不能用的演示页”变成可验收页面。

候选顺序（依赖最少优先）：

1. 会话 3 页：`/chat/v2-staff`、`/chat/v2-customer`、`/chat/v2-group`——基于已有全局消息与通讯录数据做列表/筛选/详情，需先审计是否依赖会话存档 Provider。
2. 风险预警 2 页：`/ai-insight/v2/risk`、`/ai-insight/v2/timeout`——先做“数据受限时结构化说明”，再接入真实规则事件。
3. AI 洞察 1 页：`/ai-insight/session-analysis`——依赖会话存档与 AI 任务，Provider 未就绪时按 limitations 呈现。

每页执行同一模板：复用审计（表/API/Provider/权限）→ 契约与失败测试（RED）→ 实现 → 集成测试/typecheck/build → manifest 更新 → 浏览器截图 → 矩阵更新。

**完成口径**：6 页全部 `native`、`integration-passed`，与 Phase 3.5 相同的证据标准；其中依赖存档的页面如 Provider 缺失，只能停留在受限状态不得标完成。

### Step 2：补齐 21 个 `placeholder` 页（分 3 批）

**目标**：53 页全部达到产品交付口径。

- **批 A（不依赖外部 Provider，约 7 页）**：企业设置 5 页（`/company-setting/website|staff`、`/setting/role|additional|authorization`）+ AI 设置 2 页（`/ai-setting/ai-knowledge-base|agent`）。多数是组织/RBAC/配置类 CRUD，依赖内部表，适合先做。
- **批 B（依赖会话存档/企微 Provider，约 10 页）**：会话 6 页（`/chat/trajectory|export|file-audio|resign-staff|refuse-archive`、`/customer/inheritance`）+ 风险预警 4 页（`/ai-insight/v2/customer-loss|message-intercept|keyword-library|silent-customer`）。其中 `/chat/file-audio` 在可验证的音频存储/读取 Provider 出现前保持未完成（沿用 Phase 3.3 结论）。
- **批 C（AI 洞察，约 4 页）**：`/ai-insight/smart-analysis|emotion|employee-score|communication-keyword`。依赖存档数据与 AI 能力，放在批 B 的存档/风险基础设施之后。

**完成口径**：每批逐页过 Step 1 的模板；全 53 页 manifest 无 `demo`/`placeholder`，`pnpm check:yuanhu-benchmark` 与最终基准 gate 通过。

### Step 3：53 页全量基准验收

1. 完善最终基准 gate：要求每页测试文件存在、无内部 key 直出、中文文案、工作流证据声明（当前 Phase 3.5 gate 只查 manifest 状态，需强化）。
2. 补 Playwright E2E spec（当前只有 Phase 3.2/Phase 3.1 两套，Phase 3.4/3.5 是人工截图）。
3. 全量回归：`go test ./...`、全工作区 Vitest、typecheck、build、manifest gate、`git diff --check`。
4. 逐页截图（桌面 1280×720 与移动宽度）入库，更新总验收报告。

**完成口径**：53/53 达标且证据闭环；主链端到端演示可复现。

### Step 4：React 迁移收尾（技术路线，可与 Step 1–3 并行管理）

1. 把 phase2 工作树分支（`phase2/functional-frontend`、`phase2/page-metadata-overrides`）的治理结论合入主线或明确收口策略。
2. 按既定路线推进 Sidebar → Operation 迁移，保持 URL/权限/行为等价，未迁移路由走受控兼容层。
3. 迁移完成后再做体验升级（统一组件、视觉打磨）。

**注意**：技术路线迁移不等于业务完成；135 条路由可达不能替代真实业务验收（见 `real-business-validation-debt.md`）。

### Step 5：生产化与能力完善

- 接入真实 Provider：会话存档、企微客户/群同步、微信开放平台、音频存储与读取（解锁 `/chat/file-audio`）。
- SaaS 多租户验证、生产部署脚本、备份与恢复演练（沿用 `D:\workspace\mochat-go\output\docker-backups` 习惯）。
- 性能与安全：首包体积、权限审计、审计日志、敏感字段脱敏复核。

## 四、欠缺与需补充清单

### 4.1 证据类（当前最优先）

- [ ] `go test ./...` 全绿（迁移期望常量）。
- [ ] 0120 编号冲突复核结论。
- [ ] 重建部署后 `/readyz`、容器健康、迁移应用记录与 volume 快照。
- [ ] 九页浏览器截图（新构建）+ 跨页工作流证据 + `P35-ACCEPT-*` 生命周期执行记录。
- [ ] Phase 3.4/3.5 的 Playwright E2E spec。

### 4.2 代码与门禁类

- [x] `SaaSAdminExpectedMigrationCount`/`SaaSAdminExpectedMigrationVersion` 同步（123 / `0123_phase35_order_collation_align`，2026-08-06 晚）。
- [ ] Phase 3.5 gate 强化：强制九页测试文件、中文 presentation mapping、工作流证据声明（当前 `check_phase3_5_dashboard_completion.mjs` 只查 manifest）。
- [ ] `internal/app/modules/modules.go` 与复用审计说明的不一致（实际装配在 `cmd`，文档已指出，代码侧无改动需要）。
- [ ] demo 页（6 页）的契约与状态测试。

### 4.3 Provider 类（决定若干页面能否完成）

- 会话存档 Provider：解锁会话轨迹/导出/风险预警/AI 洞察大部分页面。
- 音频存储与读取 Provider：解锁 `/chat/file-audio`。
- 企微同步 Provider：好友/客户群/离职继承数据的真实性。

### 4.4 文档类

- 本轮已同步：`phase3.5-function-matrix.md`、两份计划复选框、`PROJECT_PROGRESS.zh-CN.md`、新增主线进度分析。
- 仍缺：每页统一验收证据文件（建议给 27 个未达标页先建 spec + 证据模板，避免“页面多了再补”）；阶段 README 的“实施前状态”更新。

### 4.5 清理与治理类

- [ ] `web/saas-admin/`（dist+node_modules，约 141MB）入库/忽略决策。
- [ ] `.gocache-phase35-review/`、`.workbuddy/`、未入库计划文件归档或忽略。
- [ ] 分支合入策略：phase1/phase2/phase3.1 工作树收口时间点。
- [ ] manifest 状态机与完成比例口径（当前 26/53 的准确统计入口）。

## 五、口径与风险提示

- 路由可达、HTTP 200、空表格、静态指标、fixture 都不算完成证据（沿用各阶段验收门禁）。
- 用户审阅/确认文档使用中文；代码标识符、命令、路径、接口字段保留原文。
- 先等价迁移、后体验升级；技术路线迁移与真实业务完成分开统计。
- 任何阶段标“完成”前，必须同时满足：自动化门禁、部署健康、浏览器截图、真实数据链路四类证据。
