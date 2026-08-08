# MoChat Go 开发总进度

> 更新时间：2026-08-07
> 当前分支：`main`（Phase 3.5、坏账清理与 Phase 3 Final 均已合入并推送远端）
> 当前阶段：Phase 3 Final 已合入 `main`（53/53 达标）；测试期 AI API 默认关闭

## 当前结论

主线已完成 Phase 0–3.4：Go 单体底座、React 统一前端底座、Phase 3.2 八页门禁、Phase 3.3 菜单合入、Phase 3.4 营销工具 9 页均已交付并合入 `main`。

当前主线是 Phase 3.5（SCRM 扩展 4 页 + 真实数据报表 5 页）：九页代码、自动化门禁、重建部署与浏览器/视觉模型验收已于 2026-08-06 晚闭合；`P35-ACCEPT-*` 生命周期与 Playwright E2E spec 已补齐。

Benchmark 全局口径：**53/53 达标**。2026-08-07 坏账清理把剩余 26 页全部推到 `native` + `backend ready` + `integration-passed`；Phase 3 Final 再把 `/chat/file-audio` 解锁为 `native/ready/integration-passed`，并让 AI 洞察 5 页接入真实 AI Provider 完成“归档文本→AI→落库→回读”闭环。

## Phase 3 Final（2026-08-07）

- 后端：`internal/modules/providers`（本地音频存储 / OpenAI 兼容 AI / 企微会话存档适配层）、`internal/modules/chat-media`（`/dashboard/chat/media*` 上传/列表/鉴权下载/软删）、AI 洞察真实化（DB + AIProvider + 5 分钟缓存 + `refresh=1`）、迁移 `0126_phase3_final_providers`。
- 前端：`/chat/file-audio` native 页（选择文件→上传→播放→删除→分页），manifest 更新为 `native/ready/integration-passed`；`check_debt_clearance` 门禁 27/27。
- 验收：真实 WAV 上传→落盘→鉴权下载字节一致→软删闭环；AI 五页真实结果落库回读；`qwen3-vl-plus` 严格识图复核；全部门禁绿。
- 2026-08-07 晚：Phase 3 Final 已合入并推送 `main`（`361b918`）；**测试期 AI API 默认关闭**（`MOCHAT_GO_AI_INSIGHT_ENABLED=0`，即使配置 key 也不发起调用，实测访问页面不产生新 AI 分析）；AI 洞察与文件录音页面按 phase35 参考样式收口（KPI 卡、友好类型显示、成功/错误提示条）。
- 入口：`docs/phases/phase-3-dashboard/phase-3-final/`（设计、总验收报告、README）。

## Phase 3 坏账清理（2026-08-07）

- 新增后端模块：`internal/modules/ai-settings`（知识库/智能体 CRUD，迁移 `0124_ai_settings_tables`）、`internal/modules/ai-insight`（5 页受限态合同）。
- 复用审计与加固：消息拦截/拒绝存档/沉默客户处理器裸断言改显式 501；风险与会话页面 5 态测试补齐。
- 企业设置 5 页 native 化：复用既有 `/dashboard/user|role|menu|corp/*` 真实后端。
- 浏览器验收：26/26 页截图无错误/占位符；`qwen3-vl-plus` 严格识图审查 3 轮；知识库/智能体数据流工作流（UI→API→MySQL→回读）闭环。
- 入口：`docs/phases/phase-3-dashboard/debt-clearance/`（设计、矩阵、验收报告）。

## 数据口径统一与短期清理（2026-08-08）

- 目标：消除数据概览 `/index`（legacy `/corpData/index`）与 Phase 3.5 报表（`/dashboard/reports/*`）的两套数据口径。
- 后端：`internal/modules/reporting` 新增 `overview` 报表类型，复用 customer/conversion/behavior/employee 四类查询合并结果；授权权限键 `/dashboard/corpData/index#get`（与系统首页菜单一致）；legacy `/corpData/index` 保留不动。
- 前端：`dashboard-overview-api.ts` 改为调用 `/reports/overview`（`timezone=Asia/Shanghai` + 东八区半开日期区间，与报表一致）；概览页移除硬编码 0 模块（会话数据/质检数据/员工排行/轨迹）与趋势周期下拉；KPI 改为客户总数/线索总数/订单总数/行为事件 4 卡（与综合报表同口径）；AI 与会话归档未接入时显示空态引导。
- 验收：`go test ./...` 全绿；前端 88 文件 518 测试全绿；typecheck/build 通过；Docker 重建后 `/readyz` 200；Playwright 实测 overview 与 customer/conversion/behavior 报表各主指标一致，页面无硬编码 0。
- 清理：验收数据清理已执行（AI 分析 16 行、音频 1 行、归档消息 5 行、P35 联系人/分配/订单/审计等），证据与备份在 `D:\workspace\mochat-go\output\acceptance-cleanup-20260808\`。
- 分支：`feat/2026-08-08-data-calibre-unification`（含 `cf853c0`、`ea4f27f`、`59475cf` 等，待合入 `main`）。
- 已知限制：`reportingPrincipalResolver` 未填充 `AllowedEmployeeIDs`，员工级数据权限（self/department）在报表与概览中暂未生效（菜单权限仍生效），已另行记录。

## 阶段总览

| 阶段 | 状态 | 进度口径 | 主要结果 | 入口 |
| --- | --- | --- | --- | --- |
| Phase Pre-0：独立版与 SaaS 能力收口 | 已完成 | 以独立部署、SaaS MVP 和候选证据为准 | 建立独立运行、SaaS 总后台和生产证据链 | [阶段详情](phases/phase-pre0-standalone/README.md) |
| Phase 0：Go 工程底座 | 已完成 | 计划、检查表和验收项已闭合 | 建立 Go 单体运行边界、登记表和验证基线 | [阶段详情](phases/phase-0-go-foundation/README.md) |
| Phase 1：前端统一底座 | 已完成 | Task 1–10 与发布纠偏均验收 | pnpm workspace、React Shell、共享契约、manifest、双应用发布与授权导航 | [阶段详情](phases/phase-1-frontend-foundation/README.md) |
| Phase 2：前端逐页迁移 | 部分完成（治理基线与路线已闭环，业务迁移在历史分支） | 以审计、契约、组件测试和真实业务为准 | 135 页清单、审计修复、迁移治理与真实业务验证债务清单 | [阶段详情](phases/phase-2-frontend-migration/README.md) |
| Phase 2.1：功能前端迁移 | 已完成 | 功能矩阵、自动化测试、生产构建与四前端验收 | Dashboard/Sidebar/Operation/SaaS Admin 可运行 | [阶段详情](phases/phase-2.1-functional-frontend-migration/README.md) |
| Phase 2.2：后端质量门禁 | 已完成 | Go 模块化架构与质量门禁 | 后端质量合同与 GitHub Actions 门禁 | [阶段详情](phases/phase-2.2-backend-quality-gates/README.md) |
| Phase 3.1–3.2：Dashboard 基准与八页门禁 | 已完成并合入 `main` | 八路由精确门禁，native/legacy-adapter 才计 | 数据概览、全局消息、敏感词、SCRM 五页等 8 页 | [阶段详情](phases/phase-3-dashboard/README.md) |
| Phase 3.3：会话与风险预警菜单 | 已合入 `main` | 菜单与路由按基准呈现；业务证据为准 | 菜单合入；`/chat/file-audio` 保持未完成（无真实音频存储/读取 Provider） | [阶段详情](phases/phase-3-dashboard/README.md) |
| Phase 3.4：营销工具 | 已完成并合入 `main` | 9 页 native + 截图与验收记录 | 渠道活码、群活码、获客链接、微信客服、活码短链、一键加群、精准群发、朋友圈、素材管理 | [阶段详情](phases/phase-3-dashboard/phase-3.4/README.md) |
| Phase 3.5：SCRM 扩展与数据报表 | 已完成并合入 `main` | 九页 native + 门禁 9/9 + 浏览器验收 | 好友、客户群、订单、客户设置、客户/会话/转化/行为/综合报表 | [阶段详情](phases/phase-3-dashboard/phase-3.5/README.md) |
| Phase 3 Final：Provider 接入与总验收 | **已完成并合入 `main`** | 53/53 达标；门禁/部署/浏览器/识图/数据流证据闭合 | 音频存储 Provider + `/chat/file-audio`、AI 洞察真实化、企微存档适配层 | [阶段详情](phases/phase-3-dashboard/phase-3-final/README.md) |

## 当前可测试范围

| 范围 | 状态 | 说明 |
| --- | --- | --- |
| `/login`、`/`、`/index` | 可测试 | React Dashboard Shell，按权限渲染导航 |
| Phase 3.2 八页 | 可测试 | `/index`、`/chat/v2-all`、`/ai-insight/v2/sensitive-word`、SCRM 五页 |
| Phase 3.4 九页 | 可测试 | `/acquisition/*` 全部 native，截图版本 2026-08-04 |
| Phase 3.5 九页 | 可测试 | `/customer/friends|group|order|settings`、`/data/*` |
| `/chat/file-audio` | 可测试 | 上传/播放/删除闭环，真实本地存储 Provider |
| AI 洞察五页 | 受限态（测试期） | AI API 默认关闭（`MOCHAT_GO_AI_INSIGHT_ENABLED=0`）；开启开关并配置 key 后走归档文本→分析→落库→回读 |
| `/saas-admin/` | 可测试 | Docker 独立产物与资源前缀已验证 |
| Sidebar / Operation | 历史分支承载 | phase2 工作树分支未合入，需按既定迁移路线收口 |
| 53 页基准 | 53/53 达标 | 唯一遗留阻塞 `/chat/file-audio` 已于 Phase 3 Final 解锁 |

## 当前阻塞与风险

1. **Phase 3.5 已闭合项**：迁移常量与 0120 编号冲突已解决（`0120_phase35_acceptance_lifecycle` 重排为 `0122`，新增 `0123_phase35_order_collation_align` 对齐 collation，常量更新为 123）；部署已重建到最新迁移；九页浏览器验收与跨页工作流通过。
2. **已补齐两项待补证（2026-08-06 晚）**：`P35-ACCEPT-*` 生命周期 create/verify/cleanup 已执行留证；Playwright E2E spec（`web/e2e/tests/phase35-live.spec.ts`）已入库并在真实部署通过。剩余：真实企微/会话存档 Provider 数据。
3. **基准 53/53 已达标。** 真实企微会话存档凭证仍未提供：适配层已注册为 `limited`，提供凭证后激活并做会话/风险浏览器级回读验收。
4. **AI API 测试期关闭。** `MOCHAT_GO_AI_INSIGHT_ENABLED` 默认 `0`，AI 洞察页保持受限态、不发起外部调用；需要真实 AI 分析时置 `1` 并配置 `MOCHAT_GO_AI_PROVIDER_KEY`。
5. **未跟踪产物。** `web/saas-admin/`（dist+node_modules，约 141MB）、`.gocache-phase35-review/`、`.workbuddy/` 与个别计划文件未入库，需用户确认清理策略。
6. **验收测试数据已清理（2026-08-08）。** 运行库中 `P35-ACCEPT-*`/`P36-*` 测试数据与 AI 分析结果已删除（三轮清理，含漏网验收订单），备份与证据在 `D:\workspace\mochat-go\output\acceptance-cleanup-20260808\`；legacy 旧表（`mc_work_*`）未做全面扫描，如需彻底清理可另行立项。

## 精确下一任务

1. ~~合入 Phase 3 Final 全部改动到 `main`~~ 已合入并推送（`361b918`）。
2. 接入真实企微会话存档凭证：激活 `WeComArchiveProvider`，做会话五页 + 风险事件的浏览器级回读验收。
3. 测试期保持 `MOCHAT_GO_AI_INSIGHT_ENABLED=0`；需要真实 AI 分析时置 `1` 并配置 `MOCHAT_GO_AI_PROVIDER_KEY`（可用阿里云百炼）。
4. ~~清理验收测试数据~~ 已完成（2026-08-08，证据与备份见 `D:\workspace\mochat-go\output\acceptance-cleanup-20260808\`）。
5. 推进工程收尾：React 迁移（Sidebar/Operation）与生产化加固（真实 Provider、备份演练、性能安全）。
6. 数据口径统一分支 `feat/2026-08-08-data-calibre-unification` 待合入 `main` 并推送。

## 最近交付

- 2026-08-08：数据口径统一（overview 报表 + 概览页接入统一口径）+ 验收数据清理；分支 `feat/2026-08-08-data-calibre-unification`。
- `0bedd12`：订单审计写入原子化（Phase 3.5 最后提交，2026-08-06）。
- 2026-08-06 晚：迁移 0121–0123 应用、collation 与 transition 路由修复、九页浏览器验收与跨页工作流证据闭合。
- `0bba77a`：Phase 3.5 订单工作流产品化。
- 2026-08-07：坏账清理 26 页达标；Phase 3 Final 解锁 `/chat/file-audio`、AI 洞察真实化，53/53 达标。
- `361b918`：Phase 3 Final 合入 `main` 并推送（providers/chat-media/AI/前端/门禁/文档）。
- `a438db2`：坏账清理独立验收修复（AI 受限计数、角色中文备注、企业信息分页、知识库空态引导）。
- 2026-08-07 晚：测试期 AI API 默认关闭，AI 洞察/文件录音样式按 phase35 收口。
- `c995af5`：好友与客户群详情产品化。
- `d496a1f`：客户设置产品化。
- `c7ea81f`：五类报表页产品化。
- `baf8c98`：Phase 3.5 产品化设计（9 页统一体验模型与审核门禁）。
- `5496f1a`（main）：Phase 3.4 返工验收记录。
- `d23d17a`（main）：Phase 3.4 获客页建设。
- `8e18fe7`（main）：Phase 3.3 菜单合入。

## 更新规则

- 开始任何开发实施前，更新当前阶段 README 的“实施前状态”。
- 每个任务提交后更新“已完成、未完成与阻塞、验收与证据”。
- 只有任务清单和验收定义已基线化时才给出完成比例。
- 证据生成文件不得直接覆盖人工维护的总进度。
- 用户审阅/确认文档使用中文；代码标识符、命令、路径与接口字段保留原文。
