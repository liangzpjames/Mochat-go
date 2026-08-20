# MoChat Go 开发总进度

> 更新时间：2026-08-18
> 当前分支：`main`（Phase 7 Demo 已快进合入，并已与 `origin/main` 非强制同步）
> 当前阶段：Phase 7 隔离 Demo 已完成代码、本地镜像、ECS 部署与合成双链路验收；真实企业配置和 production 接入待完成

## 客户会话工作台紧凑化记录（2026-08-20）

`/chat/v2-customer` 本轮已直接完成页面可优化项：右侧会话统计改为四列紧凑卡片，消息类型筛选改为参考员工会话的轻量选择框，搜索与日期控件压缩到同一筛选区；无客户、选中客户、刷新和筛选后的详情链路继续由真实 API 驱动。客户页不再展示“当前归档数据无法稳定识别群聊入站消息的具体外部成员”等能力缺口说明，避免把后台能力状态占用工作区空间。

当前不能仅靠页面优化完成、必须后续单独建设的事项只有：真实会话存档仍未稳定提供群聊入站外部成员的权威身份字段，以及内部群聊能力尚未接入。后续专项需要补齐真实 archive source/成员资料、权限口径和回读证据；在此之前，前端继续使用现有真实消息与“群成员”安全回退，不伪造外部成员身份，也不在页面重复展示能力缺口文案。

## 会话轨迹工作台实施记录（2026-08-20）

本次在隔离分支 `feat/conversation-trajectory-yuanhu-workspace` 完成 `/chat/trajectory` 的元呼 AI 对标改造：

- 后端新增 `GET /dashboard/workMessage/trajectoryDay`，统一 `Asia/Shanghai` 自然日、企业范围、员工数据权限和会话存档授权；按日返回四类指标、24 小时活动、目标资料状态、未匹配消息告警和能力缺口。
- 外部群会话统一使用 `COALESCE(NULLIF(to_user_id,0), NULLIF(room_id,0), 0)` 有效目标 ID，列表、详情和轨迹共享该口径。
- 前端改为员工目录、指标卡、24 小时语义时间轴和当天消息详情抽屉；日期、员工、类型、详情锚点均可通过 URL 恢复，桌面端采用 `320px minmax(760px, 1fr)`，并覆盖 1600/1200/768 三档响应式降级。
- 轨迹 API 资源已在现有 `0143_group_conversation_workspace` 迁移中幂等登记为 `dashboard.chat.trajectory`；内部群指标明确返回 `unavailable`，不会伪造为 0。

验证记录：`go test ./internal/dashboard ./internal/store ./internal/server ./internal/migration -run 'Trajectory|WorkMessageGroupTarget|StaffDetail.*Room|MountedRoutes|MigrationContract' -count=1` 通过；Dashboard 轨迹/API 测试 16 项通过；`corepack pnpm --filter @mochat/dashboard typecheck` 和 `build` 通过。Dashboard 全量测试中已有的迁移计数合同仍报告 `expected 139, discovered 140`，原因是工作树已有 0143 迁移，不属于本次轨迹代码失败，未修改该既有合同。

## 当前结论

主线已完成 Phase 0–3 Final 的 Go 单体、React Dashboard 和 53 页基准交付；Phase 4 Dashboard 页面 RBAC、单企业身份隔离、Sidebar/Operation 移动端基础、Provider/企业微信标准能力基础与客户/客户群 durable 精准群发也已进入当前本地 `main`。

截至 2026-08-18，Dashboard 概览继续完成会话统计、AI 洞察、趋势范围、企业时区和展示一致性优化；SaaS/Dashboard 身份域、单企业切换、页面 RBAC、移动端基础、Provider 真实性和企业微信 durable 精准群发均已进入 `main`。当前产品缺口已从“页面是否存在”转为“真实外部 Provider、生产运行和持续证据是否闭合”。

Phase 7 已建立中文设计、实施计划和 ECS 交付路线，并完成隔离 Demo：官方 Linux SDK v3 已纳入 checksum 构建，本地 Linux/amd64 镜像已上传 ECS，公网加密 GET/POST 合成回调通过。真实会话存档仍是 `limited`：企业自己的 CorpID/会话存档 Secret、公钥后台配置、真实 `GetChatData/GetMediaData`、真实 callback 和 Dashboard external 数据回读证据尚未闭合。

## 近期开发文档整理（2026-08-08—2026-08-18）

| 日期 | 里程碑 | 已完成事实 | 当前边界与证据入口 |
| --- | --- | --- | --- |
| 08-08—08-09 | 数据口径与 Dashboard 交互统一 | 概览接入统一 reporting 口径；53 页桌面/390px 交互与视觉验收完成；分支已快进合入并推送主线 | 53/53 页面不等于所有外部 Provider 均真实可用；见 [主线合入记录](reviews/2026-08-09-dashboard-mainline-handoff.zh-CN.md) |
| 08-10 | 真实企微标准同步与 Phase 4 RBAC | 通讯录、客户标签、客户、客户群同步两轮幂等；53 页权限目录、49 个可授予页、4 个超管页、238 条 API 资源及数据范围门禁完成真实 Docker/API/浏览器验收 | 会话存档和 AI 当时仍如实显示未接入；见 [企微同步验收](reviews/2026-08-10-wecom-production-sync-acceptance.zh-CN.md) 与 [RBAC 验收](reviews/2026-08-10-phase4-dashboard-page-rbac-acceptance.zh-CN.md) |
| 08-11—08-12 | 身份域隔离与单企业切换 | SaaS/Dashboard token、principal、store 与认证流程分离；单企业绑定、身份迁移/预检/回填、租户门禁、独立 MFA 开关和失败关闭合同进入主线 | 生产切换仍须按只读预检、维护窗口、app-only 部署和精确回滚执行；见 [切换手册](runbooks/2026-08-11-identity-single-corp-cutover.zh-CN.md) |
| 08-13—08-14 | 账号、鉴权、移动端与 Provider 基础 | 员工账号与归档模拟器、全局 401 会话处理、服务器 53 页点击验收完成；Sidebar/Operation 共享移动基础、独立会话、22 路由门禁和企微风格视觉进入主线；Provider 状态改为证据驱动 | 移动端只有已登记纵切面可称可用，待迁移页面不伪造成完成；Provider 仍按 configured/runtime/operation evidence 分级 |
| 08-14—08-15 | Archive durable source 与企微标准能力 | `0138` archive source cursor/lease/idempotency/source identity、`0139` capability ledger、凭据 generation fencing、员工同步队列和 durable dispatch 基础完成 | 真实 archive bridge 尚未接入 `0138`；Provider completion 的普通门禁不能替代 real integration gate |
| 08-15 | 客户/客户群精准群发 | contact 与 room batch durable 闭环合入 `main`：事务写入、稳定幂等、lease/attempt/generation fencing、submit/poll/reconcile、部分失败与数据范围门禁完成 | 生产外部群发未在收口测试中调用；见 [Contact 收口](reviews/2026-08-15-contact-batch-closeout.zh-CN.md) 与 [Phase 6 交接](reviews/2026-08-15-phase6-provider-foundation-model-handoff.zh-CN.md) |
| 08-15—08-16 | Dashboard 概览与 AI 展示收口 | 修复归档报表 schema/群指标/群发标题；AI 改为每日持久化分析；概览补齐会话统计、AI 洞察、趋势联动、企业时区、完整日期序列与展示一致性 | 真实会话内容仍取决于 external archive source；AI 在 Phase 7 live 验收期间保持关闭 |
| 08-17—08-18 | Phase 7 双链路 Demo | 官方 Finance SDK v3、`GetChatData/DecryptData`、RSA 解密、seq/JSONL 证据、加密 callback GET/POST、本地构建/ECS 部署和公网隔离验收完成 | 合成双链路 PASS，真实企业主动拉取/真实事件为 `WAITING_EXTERNAL_CONFIG`；见 [Phase 7 验收](phases/phase-7-wecom-archive/acceptance/2026-08-18-wecom-archive-demo.md) |

## Phase 7：真实企业微信会话存档（2026-08-17 启动）

- SDK 方案：企业微信官方 Linux x86_64 C SDK + 项目内薄 `cgo` adapter，不采用第三方 Go wrapper 作为生产信任根。
- 运行架构：新增 Debian/glibc `archive-bridge` sidecar，只通过同机 Unix socket 服务主应用，不暴露 TCP。
- 数据链路：会话正文通过 SDK 主动轮询；现有 `/weWork/callback` 负责企业微信事件被动接收，两者独立验收。
- 持久化：真实 source 必须复用迁移 `0138_archive_source_sync` 的 cursor、lease、幂等、source identity 和审计；新增 `0140` 管理版本化 RSA keyring 和媒体任务/对象。
- 发布：app 与 bridge 镜像在本地构建为 `linux/amd64`，生成不可变 tar、digest 和 checksum；阿里云 ECS 只执行 `docker load`、迁移和 `docker compose --no-build`。
- 安全：服务器地址与凭据不入库；已在会话中出现的服务器口令必须立即轮换并改用 SSH key；Phase 7 live 期间关闭 AI 自动分析。
- 入口：[Phase 7 指导文档](phases/phase-7-wecom-archive/README.md)、[详细设计](superpowers/specs/2026-08-17-phase7-wecom-archive-design.md)、[实施计划](superpowers/plans/2026-08-17-phase7-wecom-archive.md)。
- Demo：运行镜像代码提交 `083198b87da1`，镜像 `mochat/wecom-archive-demo:083198b87da1`；独立目录/容器/端口，公网 `19090`，本机管理 `19091`；现有服务未变。
- Demo 证据：官方 SDK `.so` 装载/符号解析、容器 smoke、公网合成加密 GET/POST、证据落盘与清理均 PASS；真实企微配置标记 `WAITING_EXTERNAL_CONFIG`。
- Demo 入口：[使用说明](runbooks/2026-08-18-wecom-archive-demo.zh-CN.md)、[验收记录](phases/phase-7-wecom-archive/acceptance/2026-08-18-wecom-archive-demo.md)。

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
- 主线状态：`feat/2026-08-08-data-calibre-unification` 已于 2026-08-09 快进合入并推送 `main`，合入点与证据见主线合入记录。
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
| Phase 4：Dashboard 页面 RBAC | 已完成并进入 `main` | 当前门禁 53 页、48 页可授予、5 页超管专属、未映射 API 为 0、`scopeRequired=97` | 多角色、直接权限、数据范围、失败关闭与授权审计 | [验收记录](reviews/2026-08-10-phase4-dashboard-page-rbac-acceptance.zh-CN.md) |
| 身份域与单企业切换 | 已进入 `main` | SaaS/Dashboard realm 隔离、single principal、单企业绑定、迁移预检/回填与 live smoke 合同 | 身份失败关闭、独立 MFA、app-only 切换与回滚边界 | [运行手册](runbooks/2026-08-11-identity-single-corp-cutover.zh-CN.md) |
| Sidebar / Operation 移动端基础 | 基础与视觉已进入 `main` | 共享 runtime、独立会话、22 路由 completion gate、企微风格双端视觉 | 已实现纵切面可用；其余页面仍按迁移状态展示 | [基础计划](superpowers/plans/2026-08-14-mobile-clients-foundation.md) |
| Phase 6：Provider 与企微标准能力收口 | 已进入 `main` | Provider completion、0138/0139、durable 客户/群群发和定向门禁 | truthful Provider、archive source 边界、能力账本、员工队列与精准群发 | [交接记录](reviews/2026-08-15-phase6-provider-foundation-model-handoff.zh-CN.md) |
| Phase 7：真实企微会话存档 | **隔离 Demo 已部署，生产接入进行中** | Demo 合成双链路已过；最终仍以真实 SDK pull、callback、ECS、Dashboard 回读和恢复证据为准 | 官方 SDK Demo、主动拉取入口、被动回调、本地构建/ECS 部署；production sidecar/媒体待完成 | [阶段指导](phases/phase-7-wecom-archive/README.md) |

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
| Sidebar / Operation | 基础可测试 | 当前主线已有会话隔离、移动端 shell 和企微风格视觉基础；完整业务仍需持续 E2E |
| 53 页基准 | 53/53 达标 | 唯一遗留阻塞 `/chat/file-audio` 已于 Phase 3 Final 解锁 |
| 真实企微会话存档 | `limited`（Demo 可配置） | SDK/公网回调 Demo 已部署；等待真实 CorpID/Secret、公钥、存档范围和企微事件后完成 live pull/callback |

## 当前阻塞与风险

1. **主工作树仍有受保护的未提交内容。** `deploy/standalone/migrations/0127_dashboard_page_rbac.up.sql` 的既有修改及 `.workbuddy/`、调试脚本、旧 `web/saas-admin/` 构建产物等未跟踪内容均未纳入本次同步；后续生产发布必须在干净工作树/独立 worktree 上固化门禁和发布 SHA。
2. **真实会话存档仍未接通到 MoChat。** 隔离 Demo 已具备官方 SDK 和公网 callback，但当前 `Archive.Fetch` 仍 fail closed；真实 CorpID/Secret、公钥后台版本、`GetChatData/GetMediaData` 和 live message evidence 均未闭合。
3. **现有旧 bridge 合同存在敏感数据边界问题。** 旧客户端会把 Chat Secret 与 RSA 私钥放入 HTTP JSON；Phase 7 必须改为同机 Unix socket，且真实同步必须复用 `0138` durable 账本。
4. **RSA 密钥轮换尚不完整。** 当前凭据模型主要保存单个私钥，真实消息返回的 `publickey_ver` 需要版本化 keyring；缺少版本时必须停止并且不推进 cursor。
5. **服务器口令已在会话中出现。** 应立即轮换并改用 SSH key；口令不得写入仓库、脚本、命令历史或验收材料。
6. **AI 与真实会话的处理边界。** Phase 7 live 验收期间保持 AI 自动分析关闭；将真实会话用于 AI 前需要单独确认处理目的、权限、告知/同意和保存期限。
7. **未跟踪产物仍需保护。** `.workbuddy/`、调试脚本、旧 `web/saas-admin/` 构建产物及其他未跟踪内容属于现有工作区，Phase 7 不得通过 `clean/reset` 清除。

## 精确下一任务

1. 按 [Demo 使用说明](runbooks/2026-08-18-wecom-archive-demo.zh-CN.md) 在企微后台填写回调 URL/Token/AES Key，并配置 RSA 公钥、测试成员范围和允许 IP。
2. 在 ECS 运行 `/opt/wecom-archive-demo/configure_wecom_archive_demo.sh`，隐藏输入 CorpID/会话存档 Secret，执行第一次真实 `GetChatData/DecryptData`。
3. 产生最小真实文本与事件，核对 `callback_count`、`pull_count`、`pulled_message_count`、seq、公钥版本和 JSONL 证据。
4. 固化当前本地 `main` 的发布基线后，按正式实施计划完成 Unix socket bridge、RSA keyring、`0140` 媒体账本和 `GetMediaData`。
5. 把真实 bridge source 接入 `0138` durable sync，停用旧生产 cron composition；完成 Dashboard external 数据回读、权限、重启与回滚验收。
6. 立即轮换已在会话中出现的服务器口令并配置 SSH key。

## 最近交付

- 2026-08-20：客户会话 `/chat/v2-customer` 完成紧凑化收口；统计卡改为四列低高度布局，筛选区压缩，消息类型选择框对齐员工会话的轻量选项样式，移除客户页能力缺口提示卡；真实客户选择、群聊详情和 URL 筛选状态完成 Docker/浏览器复测。群聊外部成员权威身份字段与内部群聊能力仍按本文件“客户会话工作台紧凑化记录”登记为后续专项，页面不再重复展示限制文案。
- 2026-08-20：客户会话消息内容继续对齐员工会话；统一消息区背景、独立滚动、消息头间距、`12px / 1.6` 正文、气泡内边距/圆角、发送方向和媒体展示上限。完整工作区 typecheck/Docker 镜像重建受并行增量类型错误阻塞，客户目标测试与 Vite 编译已通过，浏览器 computed style 对比一致。
- 2026-08-18：隔离企业微信会话存档 Demo 完成本地构建、ECS 部署、官方 SDK 装载和公网合成 GET/POST 验收；最终镜像 `083198b87da1` 已补齐回调/拉取并发一致性、重复页幂等和管理端隔离检查，真实企业配置待用户在企微后台完成。
- 2026-08-18：Phase 7 Demo 分支已快进合入 `main`；近期开发文档按 08-08—08-18 时间线重新整理，远端采用非强制推送同步。
- 2026-08-17：Phase 7 真实企业微信会话存档指导、设计和实施计划完成；总进度同步，尚未开始代码实现与 ECS 部署。
- `883e216`（本地 `main`）：Dashboard 会话卡片标签与趋势标题继续收口（2026-08-16）。
- `8061ba8`：Phase 6 Provider 基础、0138/0139、客户/客户群 durable 精准群发合入当前本地 `main`（2026-08-15）。
- 2026-08-10：Phase 4 Dashboard 页面 RBAC 真实 Docker/API/浏览器验收完成。
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
