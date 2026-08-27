# 企微会话存档、租户对接模式与激活引导专项实施及验收报告

日期：2026-08-27

分支：`feat/wecom-archive-saas-activation`

基线：`main@49e96e9afee01668844061e380f9ed07a2088489`

## 1. 交付结论

本专项已完成代码、迁移、测试夹具、本地 Docker 和浏览器验收。结论严格限定为“去敏后的本地 SDK 合同及生产代码路径通过”，不等同于真实企业微信线上联调通过。

- 会话存档消息通过 `GetChatData -> DecryptData -> 正式解析/入库 -> Dashboard API` 路径生成，未通过直接写最终业务消息表伪造成功。
- 媒体通过 `GetMediaData` 分片合同、断点账本和对象存储路径生成，覆盖图片、语音、视频、文件、缺失媒体和损坏媒体。
- SaaS 开户必须选择企微模式；模式开户后不可切换。
- 自建应用配置只在 Dashboard“唯一企业资料”维护；SaaS 仅展示模式和脱敏状态。
- 第三方代开发配置只在 SaaS 安全写入/轮换，永久授权码不回显、不进入审计 JSON。
- 未配置企微的租户可正常登录 Dashboard，经营数据按真实零值或中性空态展示，不把未配置误报成加载失败。
- SaaS 的 Dashboard 超级管理员治理恢复了对象下拉、当前管理员下拉、确认保护、最后一名超管保护、409 保留选择和按动作/租户/对象/版本隔离的幂等键。
- `worker` 已合并进 `app` 的 `role=all` 进程；本地 compose 不再运行一个重复的独立 worker 服务。

## 2. 主要实现

### 2.1 租户企微模式

- 新增迁移 `0167_tenant_wecom_mode`：在权威租户企业绑定上保存非空枚举 `self_built` / `third_party_delegated`，并为既有绑定按当前集成回填。
- 开户 API 强制接收模式并一次性创建 `current/unconfigured` 集成快照。
- 所有候选配置、验证、切换和回滚公开路由均已退役；兼容服务入口固定返回 `ErrWeComModeImmutable`，不会访问数据库。
- 存储层同时校验绑定模式和当前集成模式，模式不一致时失败关闭。
- durable archive source/media 查询只处理有效租户、有效绑定、有效当前集成且模式一致的对象。

### 2.2 配置职责隔离

- 自建应用：Dashboard 企业资料维护 CorpID、AgentID 和自建应用凭据；SaaS 不接受自建 Secret。
- 第三方代开发：SaaS 维护 Provider App ID、永久授权码和能力范围；写入后只返回 `credentialHint`。
- 第三方模式读取企业资料时不解密遗留自建凭据，避免跨模式使用 Secret。
- 模式、配置版本和变更均进入租户范围审计，审计内容经过公共投影脱敏。

### 2.3 Dashboard 空数据与页面兼容

- 未绑定/未配置企微不再阻止 Dashboard 会话和报表读取。
- 报表查询对本地尚未安装的可选业务表采用明确的零值/空集合兼容，不吞掉非“表不存在”错误。
- 员工排行、会话轨迹无记录时展示“暂无员工会话排行”“暂无会话轨迹”，不再提示用户检查权限。
- 修复群聊、离职员工、AI/营销等页面在空数据或旧表合同下的加载异常。

### 2.4 超级管理员治理与激活

- 治理列表由服务端返回可执行动作；前端不会展示或提交不可执行操作。
- 最后一名有效超级管理员不可停用。
- 替换、停用、恢复、重发激活均带绑定版本；409 后保留当前选择。
- 幂等键包含动作、租户、目标对象集合和版本；成功后清除，同一动作再次执行生成新键。
- 治理 mutation 冻结请求发起时的租户、对象、绑定版本和幂等键；请求期间改变下拉不会清错 key。审批申请被受理后保留原 key，重复确认不会制造第二条逻辑申请。
- 激活入口只在创建或重发成功响应中展示一次，不宣称不存在的邮件/短信发送能力。
- 覆盖 valid、expired、activated、revoked、invalid 状态，入口可复制但不会在日志或普通列表回显原始 token。

### 2.5 本地会话存档合同

- `bridge` 是去敏后的 Finance SDK 边界替身，模拟 `GetChatData`、`DecryptData`、`GetMediaData`，不是生产企微服务，也不是第二套业务后端。
- `app` 内置 durable worker，负责 source cursor/lease/idempotency、解析入库、媒体分片下载、校验和、对象存储和重试。
- source 发现与媒体 claim 共用同一资格谓词；绑定未验证、企微 CorpID 漂移、缺少 `archive.read` 或存在缺失能力时均失败关闭。
- 本地夹具固定标记 `MOCHAT-LOCAL-ACCEPTANCE-20260827` 和“本地验收/非生产”，可幂等重跑并按数据集清理。
- 第二个媒体分片使用确定性延迟，验收脚本在已保存 checkpoint 后强制终止 app，再验证租约接管和断点续传。

## 3. 自动化验证证据

### 3.1 通过项

- `go test ./... -count=1`：PASS。
- 工作区 `pnpm lint`：PASS，覆盖 Dashboard、SaaS Admin、Sidebar、Operation 及共享包。
- 工作区 `pnpm typecheck`：PASS。
- 工作区 `pnpm test`：PASS；Dashboard 143 个测试文件、940 个测试，SaaS Admin 43 个测试，其他应用和共享包全部通过。
- 工作区 `pnpm build`：PASS，四个前端均完成生产构建。
- `pnpm check:phase4-dashboard-page-rbac`：PASS，53 页、49 个普通页面、4 个超级管理员页面，无未映射 Dashboard API。
- Provider completion：PASS。
- `pnpm check:dashboard-all-pages-evidence`：PASS。
- `pnpm check:yuanhu-benchmark`：PASS，53 页。
- `pnpm check:wecom-archive-saas-activation`：PASS，11 个专项静态/行为合同。
- MariaDB 真实迁移：`ArchiveSource` 和 `WeComCapabilityLedger` apply/down/apply 均 PASS。
- 全新独立 MariaDB 初始化：迁移账本 164 条，最大版本 `0167_tenant_wecom_mode`；能力账本 5 张表、批次租户列 2 个、企微模式非空枚举均存在。
- `git diff --check`：在最终提交前执行并通过。

### 3.2 明确跳过项

- 常规 Provider completion 运行时未设置外部 `MOCHAT_GO_MYSQL_INTEGRATION_DSN`，因此其“外部 DSN 集成套件”明确报告 `SKIP`，未计为 PASS。
- 为补足本地数据库证据，使用本专项隔离 MariaDB 的 root DSN 运行了临时 schema 的真实迁移测试并通过；这仍不是外部生产数据库验收。

### 3.3 会话存档验收结果

Docker 重启前、重启后均得到相同结果：

- cursor：10；同步运行：1；同步审计：6。
- 正式 Dashboard 消息投影：10 条。
- 消息类型：文本、图片、语音、视频、文件、链接及当前支持的其他合同类型。
- 媒体对象：7 个；ready 5、missing 1、corrupt 1。
- Dashboard 媒体类型：image、voice、video、file。
- 鉴权成功读取：1；缺失和损坏媒体读取均为 404。
- 断点恢复：checkpoint 已恢复对象 1，恢复 worker run 1。
- 重放后消息和媒体计数不增加，游标保持 10。

## 4. 浏览器验收

- SaaS 概览、客户租户、套餐、平台账号、系统运维、上线检查六个一级页面逐项点击，无加载失败。
- 客户租户筛选下拉正常；租户详情的治理对象、当前超管下拉正常。
- 实际打开“重发激活”确认框，摘要包含租户、管理员和绑定版本；取消不发送变更。
- 唯一有效超级管理员显示停用保护；无候选时不显示替换动作。
- 第三方租户实际安全保存 Provider App ID、永久授权码和能力范围；回读只显示提示名和版本，不回显授权码。
- 测试租户002 使用用户设定账号密码登录成功；数据概览为零数据且没有加载错误，刷新后保持一致。
- 本地归档租户实际打开全局消息和会话详情：文本、图片、语音、视频、文件、缺失/损坏状态均显示；图片通过鉴权后以 blob 展示。
- 员工会话页面在 390×844 下可打开员工目录和会话轨迹区域，无加载失败。
- SaaS、测试租户002、归档媒体验收标签页的浏览器控制台 error 均为 0。
- 最终镜像重建后再次刷新上述三个页面：页面标题、测试租户002列表和归档会话数量均恢复，未出现“加载失败/无法加载”，控制台 error 仍为 0。

## 5. Docker 运行信息

Compose project：`mochat-wecom-acceptance-20260827`

| 服务 | 作用 | 本地端口 |
| --- | --- | --- |
| app | API、Dashboard、SaaS Admin、内置 durable worker | 19080、19081、19082 |
| bridge | 去敏 Finance SDK 合同替身 | 19091 |
| mysql | 独立 MariaDB 10.6 | 19016 |
| redis | 独立 Redis | 29089 |

最终交付时保留以上容器运行。运行时密钥位于被忽略并限制 ACL 的 `.tmp-wecom-acceptance-runtime`，不提交仓库。

## 6. 访问与测试数据

- Dashboard / 登录：`http://127.0.0.1:19080/`
- SaaS 总后台：`http://127.0.0.1:19080/saas-admin/`
- 测试租户002：登录标识 `12400000000`，使用用户已设置的密码。
- 归档夹具租户：登录标识 `19008208270`；密码只保存在本机受限运行时文件，不写入本文档。
- SaaS 本地管理员密码同样只在本机受限运行时文件保存。

## 7. 清理与回滚

- 仅清理验收数据：`pwsh -NoProfile -File scripts/run_wecom_archive_saas_activation_acceptance.ps1 -Action cleanup -NoBuild`。
- 停止并删除本专项容器与命名卷：使用 compose project `mochat-wecom-acceptance-20260827` 执行 `down -v`；该命令会删除本专项数据库与媒体卷，应仅在不再需要本地验收环境时运行。
- 数据库迁移回滚文件为 `0167_tenant_wecom_mode.down.sql`；它只移除绑定模式列，不删除集成、员工身份、会话或客户数据。
- 代码回滚按本分支提交执行，不需要也不应 reset/clean 用户主工作树。

## 8. 保留的真实外部边界

- 未调用真实企业微信，未取得真实企业的 Finance SDK 授权、RSA 私钥、会话存档 Secret 或线上媒体数据。
- 未验证真实网络抖动、企微限流、真实证书/域名配置及线上授权企业永久授权码换取企业凭证。
- 因此可以确认的是本地协议、解析、持久化、幂等、恢复、鉴权和 UI 合同通过；真实企微线上联调仍需由具备真实企业授权和受控生产环境的后续验收完成。
- SaaS 顶栏的“系统异常”是上线就绪检查，不是页面请求失败。本地环境当前显示 21/29 项正常；其中数据库账本为 `0167_tenant_wecom_mode/164`，因为受控身份迁移 `0130/0131` 不允许在无签名维护请求的本地脚本中伪装执行，`0152` 已折叠进基础 schema 但未伪造账本。另有生产备份、生产证据等本地未配置项，因此本报告不宣称系统健康检查 PASS。
