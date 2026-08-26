# MoChat Go 开发总进度

> 更新时间：2026-08-26
> 当前分支：`fix/ai-insight-readonly-20260826`（本次服务器部署源码 `cacdc740a14a`；后续验收文档另行提交，用户已有 `.superpowers/sdd/progress.md` 改动保持未触碰，本地产物目录未提交）
> 主线同步：本地 `main` 与权威远端 `origin/main` 均为 `c696aa66400b5cb7185f18a4152ea6c0ac593834`；本次部署分支尚未合入主线
> 当前阶段：Phase 7 专用 callback、SDK bridge 和定时同步已部署；真实企微配置、真实消息拉取、Sidebar 应用授权与三端完整角色验收仍受外部条件阻塞
> 证据入口：[2026-08-26 会话存档部署与三端验收报告](deployment/2026-08-26-wecom-archive-live-deployment-acceptance.zh-CN.md)

## 会话存档生产接入与三端实测（2026-08-26）

本次在服务器 `139.196.34.133` 完成主应用与 Finance SDK bridge 的可回滚部署。所有 Linux/amd64 镜像均在本地构建，服务器只校验 SHA-256、`docker load` 和精确重建 app/bridge；MySQL、Redis、命名卷和生产数据库未删除或重建。部署前完整回滚点位于 `/opt/mochat-go/backups/wecom-archive-e1546ad6ad90-predeploy-20260826T1838CST`。

已完成事实：

- 主应用最终代码 `cacdc740a14a`，专用地址 `http://139.196.34.133/wecom/archive/callback?cid=4`；合成 GET 不再落入 Dashboard SPA，非法签名返回 `400 text/plain`，非法方法和子路径失败关闭。
- bridge `d0c9409df11a` 健康运行，管理口仅服务器本机可用且需要 Bearer 鉴权；主应用通过 Docker 内网访问。
- app/bridge 重启后均 healthy，`/healthz`、`/readyz` 和关键静态入口为 200，165 个迁移全部已应用，MySQL/Redis 容器 ID 在 app-only 发布前后不变。
- 企业配置页已增加会话存档“接收事件服务器 URL”的只读展示和复制反馈，不回显会话存档 Secret 或 RSA 私钥。
- 本地 Go 全量测试通过；Dashboard lint/typecheck/build 通过，全量 `142` 个测试文件、`918` 项测试通过。

真实验收边界：

- 2026-08-26 20:27:30（CST）企业微信向专用地址发起真实 GET URL 校验并获得 200，证明公网 URL、Token 签名和 EncodingAESKey 解密/回包链路匹配；请求参数和凭据未写入证据。当前企业 CorpID 仍未验证、会话存档 Secret/RSA 仍未配置，定时任务如实报告 `corps=0`；尚未收到 `msgaudit_notify` 或真实 `GetChatData/DecryptData` 消息。
- 当前 Dashboard 账号仅有“唯一企业资料”权限，数据概览、AI 设置和 AI 洞察会被 RBAC 重定向；SaaS Admin 没有现成登录态，均不能冒充验收通过。
- Sidebar 12 条路由的登录/失败状态可达，但用当前 AgentID 发起授权返回“应用不存在”；企业资料中的应用配置尚未闭合到 Sidebar 应用记录和员工 OAuth/JSSDK。
- 应用内浏览器本轮未实际得到 390×844 viewport，因此生产移动视觉项保持跳过；代码门禁不替代真实企微客户端验收。

下一步必须由真实企微配置驱动：完成 CorpID 验证、会话存档试用/Secret/RSA 公钥/允许 IP/测试范围，生成真实消息；同时修复 Sidebar 应用记录链路并准备 SaaS Admin 与 Dashboard 完整角色登录态。只有真实事件 POST、SDK 拉取、数据库落库和三端回读证据闭合后，Phase 7 才能标记为生产通过。

## 超时预警与客户流失菜单实施范围纠偏（2026-08-21）

本轮只实施 `/ai-insight/v2/timeout` 和 `/ai-insight/v2/customer-loss` 两个 AI 洞察风险预警菜单。已完成统一紧凑工作台、固定中文字段表格、筛选/分页/详情抽屉、规则与高级设置入口、确认操作和 URL 状态恢复；页面不再展示能力缺口说明卡。前期客户会话菜单的改动与本专项无关，不计入本专项交付。本轮代码已提交为 `e7de0ca`，尚未合入本地 `main`。

以下事项不能仅靠页面优化完成，保留在总进度台账中，页面不展示提示：客户流失仍需企业微信删除事件的不可变历史快照、流失规则和审计写链路；超时预警仍需归档消息自动扫描/定时评估闭环及更完整的通知分发、导出能力。当前页面继续使用已有真实接口，缺失字段采用稳定中文回退，不伪造业务数据。

## 会话运营四页后续专项登记（2026-08-20）

`/chat/file-audio`、`/chat/resign-staff`、`/chat/refuse-archive`、`/customer/inheritance` 本轮优化只在页面中呈现已有真实接口能够完成的业务闭环，不再把产品建设进度做成用户可见的标签、禁用按钮、提示卡或占位模块。常规布局、按钮样式、筛选、分页、抽屉、确认、错误反馈、URL 恢复和响应式问题均纳入本轮优化，不列为后续事项。

以下能力经当前接口和存储口径核对，不能靠页面调整完成，必须另立专项：

| 后续专项 | 当前事实 | 必须专项建设的原因 | 本轮四页处理 |
| --- | --- | --- | --- |
| 会话存档媒体与文件中心 | `mochat_go_audio_objects` 只承载上传音频对象；当前没有把 `mc_work_message_%` 中的消息、发送方、接收方、媒体对象、摘要、统计和导出任务串成可查询链路 | 需要接入 `GetMediaData`/内容存储、消息对象关联、媒体索引、聚合查询和异步导出合同，不能用现有上传列表替代 | 文件录音页只完成上传、日期/名称筛选、鉴权播放、删除和分页；不增加聊天文件入口或参考页专属字段 |
| 继承侧边栏 H5 | 当前仓库没有可验证的继承 H5 路由、侧边栏身份鉴权和历史会话查询接口 | 需要独立的 Sidebar 路由、企业微信上下文鉴权、客户继承关系读取和历史会话链，并单独做移动端验收 | 客户继承页只呈现离职继承、在职继承和继承记录，不提供侧边栏配置入口 |
| 在职员工群聊继承 | 当前 `contactTransfer/room` 对应离职待分配群聊及企业微信群主转接，缺少在职员工群资产读取和可验证的转接合同 | 需要先确定群资产归属口径、数据权限、企业微信能力边界、接口和审计日志，再实现批量转接 | 在职继承只完成客户读取与客户转接，不增加在职群聊入口 |
| 离职员工内部群会话归档 | 当前员工会话工作区已有外部联系人、客户群和员工单聊链路，内部群归档没有完整查询与详情合同 | 需要补内部群会话标识、成员资料、消息聚合、详情权限和归档同步链，再独立回归现有员工会话页 | 离职员工页直接隐藏内部群筛选项，只呈现能够查询详情的会话类型 |

以上四项是本次设计审阅确认的专项边界，尚未实施或排期；后续启动时必须另写设计、实施计划和真实数据验收证据。除表中四项外，本轮四页发现的问题原则上都应在当前优化中完成。

## 会话运营四页实施记录（2026-08-21）

四页已按设计和实施计划完成当前接口合同范围内的开发，并在最新 Docker 构建上完成真实登录态浏览器验收：

- `/chat/file-audio`：真实落盘音频列表、文件名/日期筛选、URL 恢复、上传弹窗、鉴权播放和删除确认。
- `/chat/resign-staff`：离职员工目录、会话类型/日期筛选、会话列表与详情工作区、URL 恢复和移动窄屏返回路径；当前企业无离职员工数据时展示普通空态。
- `/chat/refuse-archive`：客户/群聊切换、授权/跟进筛选、分页、跟进抽屉和状态备注保存；补齐分页 JSON 字段，历史 `followed` 状态归一为“跟进完成”。
- `/customer/inheritance`：离职客户/群聊、在职客户、员工选择、勾选分配确认、同步确认、继承记录抽屉和 URL 恢复；转接写操作仅在确认后执行。

验收证据见 [会话运营四页实施验收记录](reviews/2026-08-21-conversation-operations-pages-acceptance.zh-CN.md)。本次浏览器检查的 2560×1440 与 1440×900 均未发现页面级横向溢出，控制台无 error/warn，页面未展示“能力未接入”“功能未接入”类建设提示。完整工作区 typecheck/go test 的既有失败项已在验收记录登记，未修改其所属用户工作。

监督复核边界：上述四页和会话导出仍位于当前未提交工作树，不能按已合入交付计。2026-08-21 新鲜复验中 Dashboard typecheck、production build 和相关 Go 五包通过，但 Dashboard 全量测试为 728/729、Phase 4 RBAC 门禁和圆弧 benchmark 门禁仍为 RED；必须先完成分支级整体验收和固化。

## 文件录音菜单修复记录（2026-08-21）

用户复核后将 `/chat/file-audio` 的产品口径改为“企微同步录音只读查询”，本次追加实施：

- 移除上传录音、删除录音、文件名/上传日期筛选和“人工上传”来源；查询改为发送人、接收人、发送日期区间，列表对齐圆弧 AI 的音视频通话信息结构。
- 修复播放链路：录音列表只返回已落盘媒体；对已有对象的缺失时长从 WAV/MP3 文件头补算并回写，播放继续使用登录鉴权内容接口。
- 开发环境允许使用 3 条明确标记为 `wecom_sync` 的模拟企微录音数据和有效 WAV 字节，用于验证播放、时长、筛选和响应式布局；该数据不代表真实企业微信生产同步。
- 真实企微 `GetMediaData` 拉取、媒体对象索引、发送人/接收人关联和生产媒体回读仍需后续专项，页面不展示建设提示，只在本台账登记。

## 客户会话工作台紧凑化记录（2026-08-20）

`/chat/v2-customer` 本轮已直接完成页面可优化项：右侧会话统计改为与员工会话一致的四列分隔统计条，消息类型筛选改为参考员工会话的轻量选择框，查询/刷新按钮统一为短文案并压缩到同一筛选区；无客户、选中客户、刷新和筛选后的详情链路继续由真实 API 驱动。另修复群聊重点关注后的刷新查询，外层只使用规范 `to_user_id` 投影，不再触发 MariaDB `Unknown column 'room_id'`。客户页不再展示“当前归档数据无法稳定识别群聊入站消息的具体外部成员”等能力缺口说明，避免把后台能力状态占用工作区空间。

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

截至 2026-08-21，本地 `main` 已新增 Dashboard 概览、员工会话、客户会话、群会话与会话轨迹等高价值工作台优化；当前功能分支继续完成会话导出及文件录音、离职员工、拒绝存档、客户继承四页的真实接口范围优化。上述进展显著提升了 Dashboard 会话模块，但当前功能分支尚未固化，移动端自 2026-08-14 后无新提交，53 个 Dashboard 路由与 22 个移动端路由的“存在/有门禁”仍不能代表整体功能对等。

Phase 7 已建立中文设计、实施计划和 ECS 交付路线，并完成隔离 Demo：官方 Linux SDK v3 已纳入 checksum 构建，本地 Linux/amd64 镜像已上传 ECS，公网加密 GET/POST 合成回调通过。真实会话存档仍是 `limited`：企业自己的 CorpID/会话存档 Secret、公钥后台配置、真实 `GetChatData/GetMediaData`、真实 callback 和 Dashboard external 数据回读证据尚未闭合。按 2026-08-18 用户决策，Phase 7 保留现有 Demo 与证据、暂停继续投入，不计为完成或放弃；恢复条件是外部配置具备。

## 监督口径与距离最终目标

- 每次由用户触发后，固定检查：`main`、全部本地/远端分支、全部 worktree 未提交状态、最近提交、关键门禁、外部阻塞和当前阶段证据。
- 每次输出并回写：最新事实、今日唯一主攻方向、可并行任务、所需资源、阻塞与总体距离；不在本会话写业务代码或扩写阶段设计。
- 最终停止条件：不仅页面相似，而是核心功能与圆弧 AI 会话基本对等，并具备真实 Provider、租户隔离、权限、安全、部署、备份恢复、监控和首个客户交付证据，可作为产品对外售卖。
- 2026-08-21 复核估算：**整体约 60%（合理区间 58%–62%），距可售卖终点约 40%**。本地 `main` 的会话工作台进展贡献约 2 个百分点；当前未提交分支只按部分成果计权，不把专项验收等同于主线交付。移动端 17 个业务路由待迁移、真实企微会话存档和首客生产交付仍是主要差距。该比例是监督指标，不是工期承诺。

| 维度 | 当前判断 | 距离终点的主要差距 |
| --- | --- | --- |
| 圆弧 Dashboard 页面与功能面 | 路由覆盖 53 页；概览及会话类核心页已进入产品化中后段 | 本地 `main` 已改造概览、员工/客户/群会话与轨迹；当前分支增加导出和四个运营页，但全量分页合同、RBAC 种子、benchmark 契约仍未绿，好友、群活码等非会话代表页也仍待逐页深挖 |
| Sidebar / Operation 移动端 | 基础门禁已覆盖 12 + 10 路由，产品化仍在早中期 | Sidebar 仅工作台、客户概要 2 个业务纵切面，Operation 仅工作裂变 1 个业务纵切面；其余 8 + 9 个业务路由仍为通用待迁移页 |
| 核心真实业务 | 中后期 | 通讯录、客户、群发、SCRM 已有真实/持久化基础；会话存档、媒体与部分外部发布仍未生产闭环 |
| SaaS 可交付能力 | 中期偏后 | 租户、套餐、RBAC、身份隔离已有基础；仍缺首客环境的完整开户、运行、支持与退出/恢复证据 |
| 生产可信度 | 中期 | ECS Demo 已有；正式 bridge、真实 Provider gate、监控告警、备份回滚和安全收口未全部闭合 |

## 近期开发文档整理（2026-08-08—2026-08-21）

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
| 08-19—08-20 | Dashboard 概览与会话工作台产品化 | 本地 `main` 新增概览、员工会话、客户会话、群会话和会话轨迹的真实查询、筛选、详情、URL 恢复与响应式工作台 | 本地 `main` 领先远端 53 个提交，尚未形成远端可追溯基线；内部群和真实 archive 成员身份仍受数据源限制 |
| 08-20—08-21 | 会话导出与运营四页 | 当前功能分支已完成会话导出，以及文件录音、离职员工、拒绝存档、客户继承的目标测试和浏览器验收 | 当前工作树仍有 117 项未提交改动；Dashboard 全量 728/729，RBAC 与 benchmark 门禁为 RED，尚不能计入主线交付 |

## Phase 7：真实企业微信会话存档（2026-08-17 启动，2026-08-18 暂停）

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
- 暂停边界：不继续开发 production bridge、媒体账本或 live 接入；现有 Demo、镜像和验收证据保留。外部 CorpID/会话存档 Secret、公钥、测试范围和白名单齐备后再恢复。

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
| Phase 3 Final：Provider 接入与总验收 | **历史验收完成并合入 `main`；当前基准门禁待复验** | 历史 53/53 达标；2026-08-18 新鲜检查发现 manifest 的 `3-final` 与校验器允许值不一致 | 音频存储 Provider + `/chat/file-audio`、AI 洞察真实化、企微存档适配层 | [阶段详情](phases/phase-3-dashboard/phase-3-final/README.md) |
| Phase 4：Dashboard 页面 RBAC | 已完成并进入 `main` | 当前门禁 53 页、48 页可授予、5 页超管专属、未映射 API 为 0、`scopeRequired=97` | 多角色、直接权限、数据范围、失败关闭与授权审计 | [验收记录](reviews/2026-08-10-phase4-dashboard-page-rbac-acceptance.zh-CN.md) |
| 身份域与单企业切换 | 已进入 `main` | SaaS/Dashboard realm 隔离、single principal、单企业绑定、迁移预检/回填与 live smoke 合同 | 身份失败关闭、独立 MFA、app-only 切换与回滚边界 | [运行手册](runbooks/2026-08-11-identity-single-corp-cutover.zh-CN.md) |
| Sidebar / Operation 移动端基础 | 基础门禁已进入 `main`，业务产品化待逐页推进 | 共享 runtime、独立会话、12 + 10 路由门禁与企微风格视觉；新鲜检查 57/57 测试通过 | 仅 Sidebar 2 个、Operation 1 个业务纵切面不是通用待迁移页；其余 17 个业务路由待完成真实功能 | [基础计划](superpowers/plans/2026-08-14-mobile-clients-foundation.md) |
| Phase 6：Provider 与企微标准能力收口 | 已进入 `main` | Provider completion、0138/0139、durable 客户/群群发和定向门禁 | truthful Provider、archive source 边界、能力账本、员工队列与精准群发 | [交接记录](reviews/2026-08-15-phase6-provider-foundation-model-handoff.zh-CN.md) |
| Phase 7：真实企微会话存档 | **已暂停（外部配置阻塞）** | Demo 合成双链路已过并保留；暂停期间不推进 production 接入，恢复后仍以真实 SDK pull、callback、ECS、Dashboard 回读和恢复证据为准 | 官方 SDK Demo、主动拉取入口、被动回调、本地构建/ECS 部署；production sidecar/媒体待完成 | [阶段指导](phases/phase-7-wecom-archive/README.md) |

## 当前可测试范围

| 范围 | 状态 | 说明 |
| --- | --- | --- |
| `/login`、`/`、`/index` | 可测试 | React Dashboard Shell，按权限渲染导航 |
| Phase 3.2 八页 | 可测试 | `/index`、`/chat/v2-all`、`/ai-insight/v2/sensitive-word`、SCRM 五页 |
| Phase 3.4 九页 | 可测试 | `/acquisition/*` 全部 native，截图版本 2026-08-04 |
| Phase 3.5 九页 | 可测试 | `/customer/friends|group|order|settings`、`/data/*` |
| `/chat/file-audio` | 可测试 | 上传/播放/删除闭环，真实本地存储 Provider |
| `/chat/v2-staff` | 可测试 | 员工目录、会话筛选、详情统计与消息检索均读取本系统会话存档；URL 可恢复选择与筛选状态 |
| `/chat/v2-customer`、群会话、`/chat/trajectory` | 本地 `main` 可测试 | 已完成真实归档查询、目录/详情工作区与响应式优化；内部群及部分权威成员身份仍取决于真实 archive source |
| `/chat/export`、会话运营四页 | 当前分支专项可测试，未合入 | 目标测试、Docker 和浏览器证据已有；全量分页、RBAC、benchmark 三个整体验收缺口闭合前不按主线可交付计 |
| AI 洞察五页 | 受限态（测试期） | AI API 默认关闭（`MOCHAT_GO_AI_INSIGHT_ENABLED=0`）；开启开关并配置 key 后走归档文本→分析→落库→回读 |
| `/saas-admin/` | 可测试 | Docker 独立产物与资源前缀已验证 |
| Sidebar / Operation | 基础门禁可测试，完整业务不可按 22/22 计 | `check:mobile-clients-foundation` 新鲜 PASS、57/57 测试通过；Sidebar 2 个、Operation 1 个业务纵切面可继续验收，另有 17 个业务路由仍为通用待迁移页 |
| 53 页基准 | 历史路由加载 53/53；当前功能对等与门禁待复验 | manifest 仍登记 53 页且均为 `documented`；当前门禁因 `/chat/file-audio` 的 `phase=3-final` 不在校验器允许集合中而失败，历史加载证据不能替代逐页真实功能验收 |
| 真实企微会话存档 | `limited`（Demo 可配置） | SDK/公网回调 Demo 已部署；等待真实 CorpID/Secret、公钥、存档范围和企微事件后完成 live pull/callback |

## 当前阻塞与风险

1. **当前功能分支规模大且尚未固化。** `feat/conversation-operations-pages` 领先本地 `main` 2 个提交，但工作树仍有 117 项改动，tracked diff 已达 57 文件、约 `+4273/-1347`，另含迁移、后端、前端和验收文档等未跟踪文件。禁止 reset/clean；继续开新页面会进一步放大集成风险。
2. **真实会话存档仍未接通到 MoChat。** 隔离 Demo 已具备官方 SDK 和公网 callback，但当前 `Archive.Fetch` 仍 fail closed；真实 CorpID/Secret、公钥后台版本、`GetChatData/GetMediaData` 和 live message evidence 均未闭合。
3. **现有旧 bridge 合同存在敏感数据边界问题。** 旧客户端会把 Chat Secret 与 RSA 私钥放入 HTTP JSON；Phase 7 必须改为同机 Unix socket，且真实同步必须复用 `0138` durable 账本。
4. **RSA 密钥轮换尚不完整。** 当前凭据模型主要保存单个私钥，真实消息返回的 `publickey_ver` 需要版本化 keyring；缺少版本时必须停止并且不推进 cursor。
5. **服务器口令已在会话中出现。** 应立即轮换并改用 SSH key；口令不得写入仓库、脚本、命令历史或验收材料。
6. **AI 与真实会话的处理边界。** Phase 7 live 验收期间保持 AI 自动分析关闭；将真实会话用于 AI 前需要单独确认处理目的、权限、告知/同意和保存期限。
7. **未跟踪产物仍需保护。** `.workbuddy/`、调试脚本、旧 `web/saas-admin/` 构建产物及其他未跟踪内容属于现有工作区，Phase 7 不得通过 `clean/reset` 清除。
8. **当前圆弧基准门禁仍为 RED。** 2026-08-21 新鲜执行 `corepack pnpm check:yuanhu-benchmark` 仍失败：第 8 页 `/chat/file-audio` 使用 `phase=3-final`，校验器仅允许 `3.1`–`3.6`；该 P0 已连续三天未闭合。
9. **历史 Phase 4 验收 worktree 仍有未提交差异。** `.worktrees/phase4-dashboard-rbac-acceptance` 处于 detached `3915a2d`，5 个文件有 39 行新增、9 行删除；多数能力已在后续主线以其他实现存在，但仍需只读归档判断，禁止直接清理或误当成未合入的新分支提交。
10. **Phase 7 暂停不消除可售卖阻塞。** 暂停仅改变当前投入顺序，真实企微会话存档、生产 Provider 和首客运行证据仍计入最终差距。
11. **页面数量会高估产品完成度。** Dashboard 53 页和移动端 22 路由目前主要证明覆盖面；逐页检查必须继续验证真实 API、业务操作、状态处理、权限、响应式布局和可恢复失败，不能以壳页面或通用“待迁移”页计为完成。
12. **Dashboard 全量测试仍为 RED。** 2026-08-21 新鲜执行 124 个测试文件：123 个文件通过，728/729 测试通过；唯一失败是会话导出的候选列表和任务抽屉共 4 个字面“上一页/下一页”按钮未复用统一 `DashboardPagination`。
13. **Phase 4 RBAC 门禁被新资源打破。** 新鲜执行 `check:phase4-dashboard-page-rbac` 失败，`dashboard_page_catalog.json` 与迁移中的 permission resource seed 不精确一致；必须先对齐资源、迁移和授权合同。
14. **主线尚未同步远端。** 已在线核实 `origin/main` 仍停留在 `e6d238e`；本地 `main=718330d` 领先 53 个提交，当前分支又领先本地 `main` 2 个提交。未形成干净、门禁全绿、可追溯的远端基线前，不进行生产发布。

## 今日/下一工作日任务（每次检查刷新）

**今日唯一主攻方向：停止新增 Dashboard 页面，先把当前会话功能分支收口为可合入基线；门禁全绿后立即恢复移动端第一批产品化。**

1. P0 整体门禁修复（当前分支原位处理，不新建并行 Dashboard 分支）：
   - 会话导出候选表和任务抽屉统一改用 `DashboardPagination`，目标为 Dashboard 全量 729/729。
   - 对齐 `dashboard_page_catalog.json` 与 0141/0144 等迁移中的 permission resource seed，恢复 Phase 4 RBAC 门禁。
   - 统一 `/chat/file-audio` 的 manifest `phase` 与 benchmark 校验器契约，恢复 53 页基准门禁。
2. P0 完整复验：Dashboard 全量测试、typecheck、production build，相关 Go 五包、Phase 4 RBAC、圆弧 benchmark、53 页证据合同、`git diff --check` 全部通过；Provider 门禁中的 MariaDB integration 若无 DSN，只能记录 SKIP。
3. P0 真实闭环：在隔离种子企业创建一个最小会话导出任务，等待 worker 完成并下载 ZIP，核对 CSV 行数、中文/换行、公式前缀防护、企业/员工权限与工件路径；补齐当前“只测 worker、未在浏览器创建任务”的证据缺口。
4. P0 固化与同步：把 117 项改动按迁移/后端、前端、验收证据拆成可审阅提交，确认没有误纳入 `.workbuddy/`、调试脚本和旧构建产物；合入本地 `main` 后再复跑最终门禁并同步 `origin/main`。
5. P1 移动端恢复：只有上述 P0 闭合后，才启动 Sidebar `/contact/editDetail`、`/contact/remark`、`/contact/settingTag`，随后启动 Operation `/speed`；必须复用真实客户/裂变 API，完成 390×844 与 1280px 浏览器证据。移动端四页完成前，不再扩散到其他 Dashboard 页面。
6. 后续 Dashboard 队列：移动端第一批完成后，再回到 `/customer/contact` 与 `/acquisition/group-code` 的功能深度和布局对标；Phase 7 继续冻结。

所需资源：保留当前脏分支上下文的单一 Dashboard 收口会话、一个在主线更新后再创建的干净移动端 worktree/会话、可登录圆弧环境、本地 Docker 稳定种子企业、可运行真实导出 worker 的 app-storage，以及可选的隔离 MariaDB integration DSN。今日无需新增外部服务，Phase 7 的企微凭据不占用本轮资源。

2026-08-21 新鲜门禁：Dashboard typecheck PASS、production build PASS、相关 Go 五包 PASS、`check:dashboard-all-pages-evidence` 12/12 PASS、`check:mobile-clients-foundation` 57/57 PASS、`check:provider-completion` PASS（真实 MariaDB integration 明确 SKIP）、`git diff --check` PASS；Dashboard 全量测试 728/729 FAIL、`check:phase4-dashboard-page-rbac` FAIL、`check:yuanhu-benchmark` FAIL。

## 最近交付

- 2026-08-20：客户会话 `/chat/v2-customer` 完成紧凑化收口；统计卡改为四列低高度布局，筛选区压缩，消息类型选择框对齐员工会话的轻量选项样式，移除客户页能力缺口提示卡；真实客户选择、群聊详情和 URL 筛选状态完成 Docker/浏览器复测。群聊外部成员权威身份字段与内部群聊能力仍按本文件“客户会话工作台紧凑化记录”登记为后续专项，页面不再重复展示限制文案。
- 2026-08-20：客户会话消息内容继续对齐员工会话；统一消息区背景、独立滚动、消息头间距、`12px / 1.6` 正文、气泡内边距/圆角、发送方向和媒体展示上限。完整工作区 typecheck/Docker 镜像重建受并行增量类型错误阻塞，客户目标测试与 Vite 编译已通过，浏览器 computed style 对比一致。
- 2026-08-20：会话导出 `/chat/export` 完成圆弧式两步工作台实施；保留菜单与顶部 banner，移除顶部介绍框和独立刷新，新增员工/客户/群聊真实候选选择、显式查询、固定 20 条分页、日期与会话范围确认、异步导出任务抽屉、遮罩/Escape 关闭和安全下载。后端新增迁移 `0144_work_message_export_tasks`、权限资源、任务租约 worker 与 UTF-8 BOM CSV/ZIP 工件；数据仅来自现有会话归档，当前不接入内部会话、媒体二进制，缺口在界面显式提示。设计文档与实施计划见 `docs/superpowers/specs/2026-08-20-conversation-export-yuanhu-workflow-design.md`、`docs/superpowers/plans/2026-08-20-conversation-export-yuanhu-workflow.md`。
- 2026-08-19：修复员工会话部门切换加载态的布局抖动；使用 `keepPreviousData` 保留上一份真实目录数据，部门控制区固定 190px 高度，查询/刷新按钮固定 56px，避免按钮短暂归零、操作栏位移和员工列表扩张重排。
- 2026-08-19：修复员工会话详情搜索输入框逐字输入时自动失焦；详情查询切换期间保留上一份真实详情数据，确保筛选区和输入焦点持续稳定。
- 2026-08-19：优化员工会话详情关键词检索；输入阶段仅保留前端草稿，新增“查询会话内容”按钮，点击或回车后才提交 `messageKeyword` 并请求后端，避免逐字触发查询。
- 2026-08-19：修复员工会话总公司筛选不含子部门成员、部门筛选后其他部门统计归零、群聊会话详情加载失败三项问题；部门筛选改为基于真实组织树后端内存匹配，目录统计保持全量可切换，群聊 `<员工ID>:2:0` 会话允许真实归档详情读取并以“客户群/群成员”展示。相关 Go/前端回归、Docker 重建和浏览器真实数据验收通过。
- 2026-08-19：修复员工会话部门筛选和重复会话选中问题；部门筛选后保留完整组织树，空部门仍保留页面组件，员工会话摘要改用确定性窗口分组并以唯一摘要 ID选中，URL 增加 `conversationRowId`，完成真实 Docker/浏览器回归。
- 2026-08-19：员工会话 `/chat/v2-staff` 按圆弧 AI 工作区结构完成产品化改造；新增真实员工目录与会话详情接口、严格分页和 URL 状态、消息类型/关键词/日期筛选、归档授权失败关闭、迁移 `0141_conversation_workspace_rbac`，并完成 Docker 与浏览器验收。当前数据能力不支持“内部群”和单会话完整下载，界面按能力状态禁用，不构造替代数据。设计与实施计划见 `docs/superpowers/specs/2026-08-19-employee-conversation-yuanhu-workspace-design.md`、`docs/superpowers/plans/2026-08-19-employee-conversation-yuanhu-workspace.md`。
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

## 企业微信会话内容存档真实试用记录（2026-08-26）

- 专用地址 `http://139.196.34.133/wecom/archive/callback?cid=4` 已通过企业微信真实 GET challenge；2026-08-26 21:23:46、21:24:05（CST）又收到两次真实存档事件 POST，均返回 200。
- 隔离 Finance SDK bridge 使用用户隐藏交付的 CorpID 与会话存档 Secret 完成真实 `GetChatData/DecryptData`：游标从 `seq=0` 推进至 `seq=4`，得到 4 条真实记录（1 条图片、3 条文本），密钥版本为 `publickey_ver=1`。
- 幂等与恢复证据：`seq=4` 空页复拉为 0；bridge 重启后仍从 `seq=4` 开始、再次返回 0；状态为 `pulled_message_count=4`、JSONL 证据 4 行，无重复写入和拉取错误。
- 服务器受限证据快照位于 `/opt/mochat-go/backups/wecom-archive-live-evidence-20260826T212700CST`，目录 `0700`、文件 `0600`，校验和已验证。消息正文、人员标识、Secret、私钥及管理 Token 未写入本文。
- 真实与模拟边界：上述结果证明企微事件入口、官方 SDK 直拉、RSA 解密、seq、幂等和重启续拉；图片只完成类型元数据解密，尚未验证 `GetMediaData` 文件下载。
- 主程序仍未闭环：两次真实事件均被主应用识别，但因 `mc_corp.id=4` 尚未在 Dashboard/主库配置完整加密归档凭据而记录 `archive is not enabled`；定时任务仍为 `corps=0`。隔离 bridge 的 4 条 JSONL 证据不得冒充 MySQL 落库、Dashboard 回读或 Sidebar 真实数据。
- 下一门禁是安全导入会话存档凭据与 RSA 密钥到 MoChat 加密凭据存储，启用企业归档状态，再完成主库幂等落库、Dashboard 回读、敏感词消费与媒体下载验证；在此之前 Provider 保持 `LIMITED`。

## 离职员工部门选择器优化记录（2026-08-21）

- 目标：移除部门名称后的员工数，统一员工会话与离职员工的部门选择控件，并补齐本地可验收的离职员工数据。
- 方案：共享紧凑部门触发器与展开式部门树，使用真实 `/workMessage/staffDirectory` 数据；员工条目继续展示真实会话数。
- 开发数据：新增 3 名 `status=5` 离职员工、销售/客服部门关系和归档消息夹具，脚本使用 `MOCHAT-DEV-RESIGNED` 前缀幂等执行。
- 后续专项：真实企业微信离职状态同步和历史会话完整性仍需生产同步链路验证，仅记录在本台账，不在页面显示能力提示。

## 客户继承实测修复记录（2026-08-21）

- 浏览器复现并修复客户名称筛选空列表问题：后端现在同时匹配员工侧备注、客户名称和外部联系人 ID。
- 开始日期、结束日期改为独立生效，结束日期包含当天；继承记录查询沿用同一日期边界口径。
- 群聊转接响应按 chat ID 归一化为逐项成功/失败结果；离职客户和离职群聊均可从同一个“同步离职数据”入口刷新真实待分配快照。
- 未在真实企业微信上点击最终转接确认；浏览器仅验证选择、筛选、确认弹窗、取消和记录抽屉，写链路由前端 mock 与 Go fake Provider 测试覆盖。

## 客户继承确认分配错误修复记录（2026-08-21）

- 复现发现开发样例企业的 `wwSIM...` 凭据被送往真实企业微信，接口返回 `40013 invalid corpid`，前端显示“分配失败，请检查网络后重试”。
- 已将开发模拟凭据限定在客户/群聊转接本地成功路径，避免外部请求；真实企业凭据仍使用企业微信 HTTP 接口。
- 已补 Provider 回归测试并完成 app 重建；浏览器已验证确认分配成功反馈、列表更新和继承记录落库。

### 客户继承确认分配错误最终闭环（2026-08-21）

- 继续追查发现本地容器未注入 `preview-wecom-v1` 密钥，开发样例企业在凭据解密阶段即失败；新增仅匹配 `wwSIM...` 与 `SIM-...` 的明文样例回退，真实企业凭据仍保持密文强制读取。
- 将 `POST /dashboard/contactTransfer/sync` 加入精确权限白名单，修复同步按钮被 Dashboard RBAC 先行拒绝的问题。
- 浏览器实测：同步后显示“离职数据已同步”；选择客户 2013、接替员工张伟并点击“确认分配”，显示“成功 1 条，失败 0 条”，客户行从待分配列表移除。
- 后续专项：真实企业微信凭据配置与生产转接回调仍需正式环境验证，仅记录于本台账，不在页面展示能力提示。

## 风险行为与敏感词工作台实施验收（2026-08-21）

- 已完成 `/ai-insight/v2/risk` 与 `/ai-insight/v2/sensitive-word` 的统一工作台改造：移除顶部介绍大框，保留菜单导航、菜单名称、路由和顶部 banner；查询、重置、刷新集中在查询区，列表固定每页 20 条，输入过程不自动请求。
- 风险行为已接入类型化真实 API，支持记录筛选、汇总、整行详情、审计状态、规则配置、遮罩/关闭/Escape 和复选框隔离；敏感词已接入真实命中记录、员工/客户群/词组筛选、消息详情、词组与词条配置左右栏、固定分页和确认操作。
- 真实数据缺口已显式告警：当前环境风险自动归档扫描未启用，风险页面提示“扫描任务未启用，当前页面不会自动产生新记录”；敏感词页面提示“敏感词扫描任务未启用，当前只展示已有真实命中记录”。页面没有用空列表伪装成“无风险”。
- 已应用迁移 `0146_risk_warning_scan_states` 和 `0147_risk_sensitive_detail_rbac`；后者补齐详情、扫描状态和敏感词状态读取资源的原页面权限，并修复真实相关对象数组的结构化展示。
- 验证证据：`go test ./internal/dashboard ./internal/store ./internal/server ./internal/migration ./internal/config -count=1` 通过；dashboard `typecheck`、目标 Vitest 和 Vite 生产构建通过。完整 compose 前端构建仍受工作区其他页面既有未完成改动影响，本轮未覆盖或回滚这些用户改动，已使用通过的 Vite dist 与 Linux backend 更新 app 运行时。
- 运行环境：`mochat-go-desktop-app-1`、MySQL、Redis 均 healthy；未执行 `down -v`、未删除数据卷，D 盘构建和四个命名卷保持不变。
- 浏览器验收：在 `2560×1440` 视口逐项点击风险筛选、整行详情、复选框、遮罩、关闭按钮、Escape，以及敏感词筛选、详情、遮罩、配置页和词组切换；均符合预期，无自动查询、详情误触或关闭失效。
- 复核补充：发现敏感词详情曾把 `isTrigger`、`msgContent` 等技术字段直接呈现；已补充类型化详情解析、失败测试和回归测试，当前浏览器详情展示为发送人、消息类型、触发消息、发送时间，遮罩关闭再次验证通过。
- 遮罩修复：新增敏感词抽屉的遮罩改为低透明度叠加，并增加背景虚化；提高选择器优先级，避免旧的全局按钮白底样式覆盖遮罩。浏览器已点击验证弹框显示与遮罩关闭。
- 顶部文案收敛：移除风险行为和敏感词页面内容头部的“AI 洞察”眉标题，保留风险预警菜单、页面标题和应用导航；浏览器已核对两个主内容区均不再展示该文案。

## 消息拦截、关键词库与沉默客户优化设计记录（2026-08-21）

- 已完成 `/ai-insight/v2/message-intercept`、`/ai-insight/v2/keyword-library`、`/ai-insight/v2/silent-customer` 的现状、圆弧 AI 参考界面、前后端接口、权限、数据库和本地样例数据核查；设计与实施计划已完成并据此实施。
- 设计采用共享风险预警工作台、三个独立领域页：消息拦截覆盖规则与归档后命中复核，关键词库覆盖草稿词条与不可变发布版本，沉默客户覆盖真实归档互动评估、分派、跟进和审计；页面不增加“能力未接入”类占位内容。
- 当前真实数据为 1 个关键词库、6 个词条、1 条拦截规则、2 条拦截记录、1 条沉默规则和 3 条沉默记录；这些数量只用于验证数据链，不作为前端常量。
- 后续独立专项：企业微信会话存档只能在消息产生后拉取，现有 MoChat 没有员工发送端插件或客户端 SDK，因此真正“发送前阻断并即时提示员工”不能由本轮 Dashboard 与后端页面优化完成。当前方案只实现归档后自动检测与合规复核，不在页面展示能力缺口，也不伪装成发送前已阻断。
- 审阅文档：`docs/superpowers/specs/2026-08-21-message-intercept-keyword-silent-yuanhu-design.md`、`docs/superpowers/plans/2026-08-21-message-intercept-keyword-silent-yuanhu.md`。

## AI 洞察会话分析与智能分析实施验收（2026-08-21）

- 已完成 `/ai-insight/session-analysis` 与 `/ai-insight/smart-analysis` 两个菜单的设计、实施和真实浏览器验收；保留情绪识别、员工评分、沟通关键词三个旧菜单，不在本轮扩展范围。
- 会话分析改为真实归档会话结果工作台：关键词、会话类型、员工 ID、日期筛选，固定每页 20 条，URL 状态、重置、刷新、导出和详情入口均已接入；智能分析拆分分析结果/分析规则，支持规则新增、不可变版本、启停、编辑和删除。
- 后端新增结构化结果契约、严格 JSON/evidence 校验、跨消息分表会话归并、幂等结果仓储和每日后台任务；页面读取只读已落库结果，不提供即时调用模型按钮，不使用前端静态业务数据。
- 已应用迁移 `0148_ai_conversation_insights`；发现存量数据库虽已执行 0148，但 API 权限资源未随修改后的种子块回填，补充幂等迁移 `0151_ai_conversation_insight_api_resources` 并在本地容器执行到 `applied_now`，浏览器复测权限恢复正常。
- 页面移除旧版“AI 能力未接入”三张指标卡和大段能力说明；空态、头像 fallback 和规则抽屉均使用稳定业务文案，不展示固定“请”字或技术故障长文。
- 自动化证据：Go `internal/modules/ai-insight`、`internal/dashboard`、`internal/migration`、`internal/server` 回归通过；Dashboard 全量 Vitest `136 files / 764 tests passed`，typecheck、定向 ESLint、`git diff --check` 通过；Docker app-only 重建、迁移、`/readyz=200` 通过。
- 浏览器证据：在 `2560×1440` 和 `1366×900` 逐页点击打开、查询、重置、刷新、标签切换、规则抽屉和规则保存；两页 `scrollWidth=innerWidth`，控制台 error/warn 为 0，无权限拒绝和旧 banner。验收期间创建的“验收规则”及其版本已精确清理。

## 关键词库页面浏览器复核与交互收口（2026-08-21）

- 真实登录态复核发现两项布局问题：左侧唯一词库按钮曾被网格默认拉伸成整列高度；选中词库后右侧操作区同时使用多个蓝色主按钮，且关键词表在常规宽度下出现横向滚动，操作按钮被截断。
- 已完成收口：左侧词库项改为内容高度并固定分页到底部；“发布”保留唯一蓝色主操作，“编辑词库/启停”统一次级样式，“删除”使用低强调危险样式；关键词表改为固定列比例并在 `1366px` 左右视口内完整展示。
- 新建流程已修复为“表单保持空白但保留当前词库上下文”，关闭新建抽屉后仍可继续添加关键词；编辑、添加关键词抽屉均已通过真实点击验证。
- 浏览器验收证据：常规视口 `1353×1272` 与宽屏 `2560×1440` 均无页面或表格横向溢出，关键词库操作按钮层级符合预期，控制台 error/warn 为 0；目标 Vitest 5/5、Dashboard production build 通过。
- 后续专项（只记录，不在页面展示）：正式企业微信会话存档凭据/授权与持续同步、正式 AI Provider Key/配额/脱敏/成本治理、外部模型调用前授权告知与审计流程；本地开发库暂无新增会话级分析结果，页面按真实数据展示空态。
- 设计与验收文档：`docs/superpowers/specs/2026-08-21-ai-insight-session-smart-yuanhu-design.md`、`docs/superpowers/plans/2026-08-21-ai-insight-session-smart-yuanhu.md`、`docs/reviews/2026-08-21-ai-insight-session-smart-acceptance.zh-CN.md`。

## 消息拦截、关键词库与沉默客户实施验收记录（2026-08-21）

- 三页已切换到共享风险预警工作台：保留原菜单和路由，移除风险预警子菜单重复的顶部 banner、重复介绍卡和重复刷新入口；查询均显式提交，分页固定每页 20 条，已提交条件可由 URL 恢复。
- 消息拦截已完成命中记录/规则双页签、会话类型/决定/复核状态筛选、类型化详情、规则编辑、词库版本引用、启停/删除确认和批量复核；保存失败会在页面给出可读原因，成功后关闭抽屉并刷新列表。
- 关键词库已完成左右分栏、词库与词条编辑、发布版本、启停/删除确认；未保留白名单或技术字段展示，词库和词条均来自真实接口数据。
- 沉默客户已完成客户/状态/负责人筛选、真实员工选择器、记录详情、规则编辑、批量分派/跟进/唤醒/关闭确认；操作成功后刷新仍保持已分派状态，刷新 URL 后规则编辑数据可恢复。
- 新增迁移 `0150_repair_keyword_published_snapshots`，修复开发样例词库“已发布 v1 但缺少不可变版本快照”导致规则保存失败的数据链问题；迁移已在本地容器应用并验证 6 条词条快照。
- 额外修复工作区类型与共享分页契约，确保全量 Dashboard 构建和分页门禁可通过；未改变其他业务行为。
- 自动扫描、发送端即时阻断等需要独立运行链路或客户端能力的事项仅记录在本文和实施计划中，未在页面伪造状态或展示“能力未接入”文案。
- 验证证据：Dashboard 全量 Vitest `136` 个文件、`764` 个测试全部通过；Dashboard typecheck、Vite production build、Go `internal/dashboard/internal/store/internal/server/internal/migration` 回归全部通过；Docker app、MySQL、Redis healthy；浏览器实际点击验证三页查询、详情、编辑保存、确认分派、刷新持久化和 URL 恢复。

### 风险预警子菜单与关键词库细节修复（2026-08-21）

- 风险预警主菜单下的消息拦截、关键词库、沉默客户、客户流失、风险行为和超时预警均移除重复的顶部 banner；通用风险预警兜底页同步移除，业务内容从查询栏或工作区工具栏开始。
- 关键词库左侧列表改为可伸展布局，分页固定贴在底部；页码、上一页/下一页和页大小等控件强制保持单行，窄宽时仅在分页区域横向滚动。
- “新建词库”与“编辑词库”明确区分：新建打开空白表单，不继承当前选中词库；编辑使用实心主按钮，新增词库和添加关键词使用统一主操作样式。
- 关键词库未选择时右侧只显示选择提示，不再展示默认词条或操作内容。
- 风险预警通用兜底页、旧会话运营兜底页和风险行为规则抽屉不再展示“能力未接入/数据提供方未接入”类提示，统一使用当前筛选无记录或规则行为说明；真正需要独立专项的事项仍只保留在本总进度文档中。
- 回归证据：风险预警及会话运营定向测试 `28/28` 通过，Dashboard 全量 Vitest `136` 个文件、`765` 个测试通过，typecheck 通过，Docker Desktop 重建后 app、MySQL、Redis healthy；真实浏览器验收因容器重建导致原登录态失效，未伪造点击通过结论，待重新登录后补做一次最终点击核对。
