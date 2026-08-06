# MoChat Go 开发总进度

> 更新时间：2026-08-07
> 当前分支：`main`（已合入 Phase 3.5，推送远端 `e3b83aa`）
> 当前阶段：Phase 3 坏账清理 26 页验收闭合（52/53 达标），Phase 3 Final 可进入总验收

## 当前结论

主线已完成 Phase 0–3.4：Go 单体底座、React 统一前端底座、Phase 3.2 八页门禁、Phase 3.3 菜单合入（`/chat/file-audio` 明确未完成）、Phase 3.4 营销工具 9 页均已交付并合入 `main`。

当前主线是 Phase 3.5（SCRM 扩展 4 页 + 真实数据报表 5 页）：九页代码、自动化门禁、重建部署与浏览器/视觉模型验收已于 2026-08-06 晚闭合（`go test ./...`、Dashboard 476 测试、typecheck、生产构建、门禁 9/9、九页截图与订单跨页工作流全部通过）。仍待补证：`P35-ACCEPT-*` 生命周期执行记录、Playwright E2E spec、真实 Provider 数据。

Benchmark 全局口径：53 页中 52 页达标（`native`/`legacy-adapter`），`/chat/file-audio` 因无音频存储/读取 Provider 显式未完成。2026-08-07 坏账清理把剩余 26 页全部推到 `native` + `backend ready` + `integration-passed`（会话 8、风险预警 6、AI 洞察 5、AI 设置 2、企业设置 5），浏览器 + 识图模型 + 数据流工作流证据闭环。

## Phase 3 坏账清理（2026-08-07）

- 新增后端模块：`internal/modules/ai-settings`（知识库/智能体 CRUD，迁移 `0124_ai_settings_tables`）、`internal/modules/ai-insight`（5 页受限态合同）。
- 复用审计与加固：消息拦截/拒绝存档/沉默客户处理器裸断言改显式 501；风险与会话页面 5 态测试补齐。
- 企业设置 5 页 native 化：复用既有 `/dashboard/user|role|menu|corp/*` 真实后端。
- 浏览器验收：26/26 页截图无错误/占位符；`qwen3-vl-plus` 严格识图审查 3 轮；知识库/智能体数据流工作流（UI→API→MySQL→回读）闭环。
- 入口：`docs/phases/phase-3-dashboard/debt-clearance/`（设计、矩阵、验收报告）。

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
| Phase 3.5：SCRM 扩展与数据报表 | **进行中** | 九页代码完成；部署/浏览器/全量门禁未闭合 | 好友、客户群、订单、客户设置、客户/会话/转化/行为/综合报表 | [阶段详情](phases/phase-3-dashboard/phase-3.5/README.md) |

## 当前可测试范围

| 范围 | 状态 | 说明 |
| --- | --- | --- |
| `/login`、`/`、`/index` | 可测试 | React Dashboard Shell，按权限渲染导航 |
| Phase 3.2 八页 | 可测试 | `/index`、`/chat/v2-all`、`/ai-insight/v2/sensitive-word`、SCRM 五页 |
| Phase 3.4 九页 | 可测试 | `/acquisition/*` 全部 native，截图版本 2026-08-04 |
| Phase 3.5 九页 | 代码可测，浏览器待复验 | `/customer/friends|group|order|settings`、`/data/*` |
| `/saas-admin/` | 可测试 | Docker 独立产物与资源前缀已验证 |
| Sidebar / Operation | 历史分支承载 | phase2 工作树分支未合入，需按既定迁移路线收口 |
| 剩余 27 页（demo/placeholder） | 未完成 | 会话 9、风险预警 6、AI 洞察 5、AI 设置 2、企业设置 5 |

## 当前阻塞与风险

1. **Phase 3.5 已闭合项**：迁移常量与 0120 编号冲突已解决（`0120_phase35_acceptance_lifecycle` 重排为 `0122`，新增 `0123_phase35_order_collation_align` 对齐 collation，常量更新为 123）；部署已重建到最新迁移；九页浏览器验收与跨页工作流通过。
2. **已补齐两项待补证（2026-08-06 晚）**：`P35-ACCEPT-*` 生命周期 create/verify/cleanup 已执行留证；Playwright E2E spec（`web/e2e/tests/phase35-live.spec.ts`）已入库并在真实部署通过。剩余：真实企微/会话存档 Provider 数据。
3. **基准 27 页未达标。** `demo` 6 页、`placeholder` 21 页（会话 9、风险预警 6、AI 洞察 5、AI 设置 2、企业设置 5），是宣告 53 页基准完成的最大缺口。
4. **Provider 能力受限。** 会话存档等真实 Provider 未接入，`/chat/file-audio` 保持未完成；会话/风险类页面只能做受限状态呈现。
5. **未跟踪产物。** `web/saas-admin/`（dist+node_modules，约 141MB）、`.gocache-phase35-review/`、`.workbuddy/` 与个别计划文件未入库，需用户确认清理策略。
6. **验收测试数据待清理。** 运行库中 `P35-ACCEPT-*` 联系人/订单为测试数据（含经 PowerShell 创建的名称带 `?` 的记录），清理步骤见主线分析文档。

## 精确下一任务

1. 补齐剩余待补证：接入真实企微/会话存档 Provider 数据；`P35-ACCEPT-*` 生命周期与 Playwright E2E spec 已于 2026-08-06 晚完成。
2. 复核本轮改动（迁移 0121–0123、路由 PUT/PATCH、报表/明细修复）后，将 `feature/phase3.5-scrm-reporting` 合入 `main` 并同步本文件。
3. 启动基准补齐批次：先产品化 6 个 `demo` 页（会话 3、风险预警 2、AI 洞察 1），再按依赖处理 21 个 `placeholder` 页。
4. 清理验收测试数据（`P35-ACCEPT-*`，含名称带 `?` 的记录），步骤见主线分析文档。

## 最近交付

- `0bedd12`：订单审计写入原子化（Phase 3.5 最后提交，2026-08-06）。
- 2026-08-06 晚：迁移 0121–0123 应用、collation 与 transition 路由修复、九页浏览器验收与跨页工作流证据闭合。
- `0bba77a`：Phase 3.5 订单工作流产品化。
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
