# 企微会话存档媒体接入、SaaS 租户对接模式与新用户激活引导设计

日期：2026-08-27

状态：本次专项已授权自主实施；设计经自审后直接进入实施，不等待阶段性批准

基准：`main@49e96e9afee01668844061e380f9ed07a2088489`

## 1. 目标与验收边界

本专项交付三个相互关联但边界清晰的能力：

1. 使用去敏后的企业微信 Finance SDK 响应夹具和确定性媒体字节，在本地沿生产解析、0138 durable 同步、消息分表、媒体任务、对象存储和 Dashboard 鉴权回读链路，验证文本、图片、语音、视频和文件。
2. 在 SaaS 管理端按租户配置“自建应用”或“第三方代开发应用”，凭据与授权材料分别隔离，模式切换失败关闭、可审计、可回滚。
3. 为新租户管理员提供可复制但不把令牌暴露给服务端日志和 Referrer 的激活入口，并明确展示有效、过期、已激活、已撤销、失败、重发、首次登录、MFA 与企业配置下一步。

本地夹具通过只表示“本地 SDK 合同、生产适配器和持久化链路通过”，不表示真实企业微信线上已开通、真实可信 IP/授权范围有效或真实媒体下载成功。真实企业微信本次不调用。

## 2. 当前实现证据与缺口

### 2.1 会话存档

- `internal/wecomarchivedemo/sdk_linux.go` 已通过官方 `libWeWorkFinanceSdk_C.so` 实现 `GetChatData` 和 `DecryptData`，但没有 `GetMediaData`。
- `internal/dashboard/work_message_archive_sync_cron.go` 可从 bridge 拉取并写入 `mc_work_message_1..10`，但使用 legacy `mc_work_message_id.type=40` 游标，真实链路绕过了迁移 0138 的 run、lease、fence、source identity 和 idempotency。
- `internal/modules/providers/archive/sync.go` 与 `internal/store/archive_sync.go` 已具备 durable 同步语义，但生产 external source 明确 fail-closed，当前只有 simulation 使用。
- Dashboard 已能渲染文本、图片、音频、视频、文件和链接；媒体类型必须获得受鉴权 URL，而现有 SDK 明文通常只有 `sdkfileid`，因此媒体只能显示元数据或不支持预览。
- `mochat_go_audio_objects` 是录音页旁路，不是统一会话媒体对象；现有开发 seed 不能作为正式接入证据。

### 2.2 SaaS 企微对接

- `mochat_go_tenant_corp_bindings` 已建立唯一租户企业绑定，Dashboard principal 按租户和企业失败关闭。
- `internal/wecomcredentials.Manager` 已提供 AES-GCM、key ring、AAD 绑定和不回显能力，可扩展第三方授权凭据用途。
- 当前 `mc_corp`/`mc_work_agent` 只表达自建应用凭据；代码没有 suite/provider app、永久授权码、授权范围、撤权或租户集成模式。
- 能力账本已有 generation fence，可作为模式切换后阻止旧任务继续调用的基础。

### 2.3 新用户激活

- 开户事务已原子创建租户、占位企业、pending binding、Dashboard 超管、激活摘要和审计；明文 token 只在首次成功响应返回，24 小时过期。
- 激活写入使用事务和 `FOR UPDATE`，支持一次性消费；MFA 已支持 TOTP、加密保存和重放防护。
- SaaS UI 只展示原始 token；Dashboard `/activate?token=...` 在用户填写密码期间保留 token 于地址栏、历史和潜在 Referrer。
- 激活页没有预检，无法区分过期、已激活和撤销；开户、激活、MFA、企业绑定和权限分配没有统一下一步读模型。

## 3. 方案选择

### 3.1 采用方案

采用“兼容适配、单一权威账本、显式状态机”的渐进方案：

- 保留现有 bridge HTTP 协议和 legacy cron 开关用于回滚，但新增 bridge-backed `ArchiveSource`，新验收和新生产组合只走 0138 `SyncService`。
- 扩充 bridge SDK 边界以支持 `GetMediaData`；消息正文 durable 落库时原子创建媒体任务，媒体下载失败不回滚正文和消息游标。
- 增加统一会话媒体对象表与带 lease/fence 的下载 worker；本地对象存储用 `.part` 临时文件和原子 rename，支持 `indexbuf` 续传、校验和重启恢复。
- 新增租户企微集成表。现有企业回填为 `self_built`，第三方代开发使用独立加密授权信封；任何模式都不自动回退到另一模式。
- 模式切换使用 current/candidate 两阶段模型：候选保存和验证不影响当前实例，激活时锁定版本、租户企业、能力和在途任务，递增 generation 并写审计。
- 激活入口使用 `/activate#token=<opaque>`；页面启动后立即读取 fragment 并清空地址栏，再以 POST body 调用预检/激活接口。
- 激活状态由数据库事实派生，不建立会漂移的单一总状态字段。

### 3.2 未采用方案

1. **继续扩展 legacy cron**：实现较少，但无法复用 0138 的 lease、fence、source identity 与重放保证，验收会制造两套游标，因此不采用。
2. **媒体与正文同事务同步下载**：实现直观，但网络失败会阻塞游标，无法满足“媒体失败不破坏正文”和重启续跑，因此不采用。
3. **两种企微模式共用 `mc_corp.secret`**：会造成授权边界混淆、切换时静默回退及密钥误用，因此禁止。
4. **把激活 token 放 query 参数**：兼容现状但泄露面更大；保留旧参数读取仅作为迁移兼容，读取后同样立即清理。

## 4. 会话存档与媒体架构

### 4.1 SDK 与 bridge 合同

`FinanceSDK` 增加：

```go
GetMediaData(ctx context.Context, sdkFileID, indexBuf string, timeout time.Duration) (MediaChunk, error)
```

`MediaChunk` 包含 `Data []byte`、`NextIndexBuf string`、`Finished bool`。Linux 实现必须解析并释放官方 SDK 的 media data、index buffer、out index buffer 和 slice；返回码和错误不得包含 Secret、私钥、RSA 随机密钥或完整 `sdkfileid`。

bridge 增加 Bearer 保护的媒体端点，接收 corp ID、sdkfileid、indexbuf 和受限超时，返回 base64 分片、下一 indexbuf、完成标记及脱敏错误码。bridge 实例继续持有 Secret/RSA 私钥，主应用请求不得携带这些敏感值。

去敏夹具必须保留真实 SDK JSON 结构：`seq`、`msgid`、`encrypt_random_key`、`encrypt_chat_msg` 以及解密后的 `msgtype` 子对象；文本、图片、语音、视频、文件至少各一条，另覆盖 link、location、mixed 和未知类型。fixture 的所有 ID 必须以 `MOCHAT-LOCAL-ACCEPTANCE` 命名空间标记。

### 4.2 durable source

新增 `BridgeSource` 实现 `ArchiveSource`：

- `Kind=external`；
- `SourceID=wecom:<verified_wx_corpid>`；
- `Namespace=wecom:<verified_wx_corpid>`；
- 只接受调用方从认证上下文解析的 `tenant_id/corp_id`；
- 将 bridge JSON 规范化为 `archive.Message`，保留完整原始明文 JSON 和媒体描述；
- page 的 `NextCursor.Sequence` 只能来自已解析且单调的最大 seq；空页和失败不前推。

新组合入口调用现有 `SyncService`。调度幂等键使用稳定的 `archive:<tenant>:<corp>:<source>:<cursor>`，相同起始游标重放只返回同一 run。legacy cursor 只作为首次迁移输入，后续 0138 run cursor 为权威。

### 4.3 媒体任务与对象

新增 `mochat_go_archive_media_objects`：

- 主键 UUID；唯一键 `(tenant_id, corp_id, msgid, sdk_file_id_hash)`；
- `media_type`、`file_name`、`mime_type`、`expected_size`；
- `status=pending|downloading|ready|failed|missing|corrupt`；
- `index_buf`、`bytes_downloaded`、`sha256`、`storage_path`；
- `attempt`、`lease_token`、`lease_expires_at`、`heartbeat_at`；
- `error_code`、`created_at/updated_at/completed_at`；
- 外键同时约束 tenant/corp/msgid/source identity。

`UpsertArchiveMessage` 在保存消息及 source claim 的同一事务里幂等创建媒体对象。worker 执行：

1. 原子 claim pending、可重试 failed 或过期 downloading 记录；
2. 读取 `.part` 长度和已持久化 `index_buf`；
3. 逐分片调用 bridge、追加写入、刷新 heartbeat 和断点；
4. 完成后校验非空、声明大小（如有）和 SHA-256；
5. 以原子 rename 提交对象并将状态置为 ready；
6. 缺失返回 missing，校验失败返回 corrupt，暂态失败返回 failed；
7. 所有更新带 `id+attempt+lease_token` fence，旧 worker 不能覆盖新 attempt。

日志只记录 media object ID、tenant/corp、消息哈希前缀、类型、字节数和错误码。

### 4.4 Dashboard 回读

消息查询根据 `(tenant_id, corp_id, msgid)` 批量关联媒体对象，并在原消息内容旁输出：

```json
{
  "media": {
    "id": "uuid",
    "status": "ready",
    "contentUrl": "/dashboard/archive/media/uuid/content",
    "downloadUrl": "/dashboard/archive/media/uuid/content?download=1",
    "name": "fixture.pdf",
    "mimeType": "application/pdf",
    "sizeBytes": 128
  }
}
```

鉴权读取必须从 Dashboard principal 获得 tenant/corp，数据库查询同时限定两者；跨租户/跨企业统一返回 404。响应设置 `X-Content-Type-Options: nosniff`、安全的 `Content-Disposition`，支持 GET/HEAD 和单段 Range。pending/failed/missing/corrupt 不读取文件，返回稳定错误码。

前端 renderer 根据状态显示预览、播放、下载、处理中、缺失或损坏，不将 `sdkfileid` 暴露到 DOM 或 URL。

## 5. SaaS 租户企微对接模式

### 5.1 数据模型

新增 `mochat_go_wecom_integrations`，每个 tenant/corp 允许一个 current 和一个 candidate：

- `mode=self_built|third_party_delegated`；
- `slot=current|candidate`；
- `status=unconfigured|pending_verification|active|suspended|revoked|failed`；
- `verified_wx_corpid`、`agent_id`、`provider_app_id`；
- `credential_ciphertext`、`credential_key_id`、`credential_hint`；
- `scope_json`、`scope_digest`、`missing_capabilities_json`；
- `generation`、`version`、`verified_at`、`last_error_code`；
- 创建/更新人和时间。

新增 `wecomcredentials.AuthorizationCredential`：

- 自建模式保存独立的 employee/contact/agent/chat secret 以及可信域名配置摘要；
- 第三方代开发模式保存 suite/provider 标识和 tenant-scoped permanent authorization code；
- AAD 使用 `resource=integration`、tenant ID 和 integration UUID；
- GET 响应只返回 `credentialConfigured`、hint 和 key ID，不返回明文或密文。

现有已验证企业回填为 current/self_built；fake/pending 企业回填为 unconfigured，不得误报 active。

### 5.2 API 与权限

SaaS API：

- `GET /dashboard/saasAdmin/tenants/{tenantId}/wecom-integration`；
- `PUT /dashboard/saasAdmin/tenants/{tenantId}/wecom-integration/candidate`；
- `POST .../candidate/verify`；
- `POST .../switch`；
- `POST .../rollback`；
- `GET .../audits`。

读取需要 `platform.integrations.read`；保存候选、验证、切换和回滚需要 `platform.integrations.manage`。切换仍使用现有关键操作审批框架；本地验收关闭审批时允许同一请求执行，但审计必须保留。

服务端从 path 和当前 SaaS principal 取得 tenant/actor，不信任请求体里的 tenant、corp 或 actor。所有写入使用 version 乐观锁。

### 5.3 验证与切换

候选验证不调用真实企微时只能得到 `contract_verified`，不能伪装为 `online_verified`。本地 Provider stub 必须显式返回 `local_fixture=true` 和固定 capability scope。

切换事务锁定 tenant binding、current 和 candidate，并重新检查：

- 租户、订阅和企业绑定有效；
- candidate 已验证且 `verified_wx_corpid` 与权威绑定一致；
- 凭据可解密；
- 必需 scope/capability 齐全；
- 没有不允许中断的企微发送任务或 archive downloading lease；
- 请求 version 与当前一致。

成功后 current 进入可回滚历史，candidate 成为 current，generation 递增；旧 generation 的任务在真正调用 provider 前失败关闭。失败时 current 不变。

## 6. 新租户激活引导

### 6.1 激活预检与链接

新增 `POST /dashboard/auth/activation/status`，请求 body 只含 token，响应：

- `status=valid|expired|activated|revoked|invalid`；
- 安全的租户名、账号掩码、过期时间；
- `primaryAction=activate|login|contact_admin`；
- 不返回 token、摘要、密码、MFA secret。

SaaS 页面生成 `/activate#token=<opaque>` 完整入口。复制时显示用途和过期时间；关闭弹窗后从 React 状态移除明文。若是审批执行结果，同样只展示一次。

Dashboard 激活页启动后读取 fragment/旧 query，立即 `history.replaceState` 到 `/activate`，再执行预检。提交成功清空组件内 token，并进入登录 CTA。

### 6.2 状态与下一步

统一读模型由现有事实派生：

`activation_issued → credentials_set → mfa_pending → corp_config_pending → corp_verified → permissions_pending → active`

异常状态：`activation_expired`、`activation_revoked`、`tenant_blocked`、`corp_suspended`、`setup_failed`。

SaaS 租户详情展示管理员当前状态、阻塞原因、过期时间和主 CTA。Dashboard 登录/MFA/企业配置页复用同一中文提示语义：

- 激活有效：设置密码；
- 过期/撤销：联系平台管理员重发；
- 已激活：前往登录；
- MFA 待完成：绑定并验证；
- 企业未配置：仅超管进入企业配置，普通用户看到等待管理员状态；
- 企业已验证：同步员工并分配权限；
- 全部完成：进入工作台。

不得展示不存在的邮件、短信或企微自动发送按钮；“重发”明确表示生成新的激活入口，由管理员通过受控渠道交付。

### 6.3 可访问性与响应式

- 桌面使用现有 SaaS/Dashboard 卡片、状态标签、按钮和表单样式；不改变菜单和顶部 banner。
- 390×844 下单列、44px 可点击区域、长链接和 MFA 文本可断行；主要 CTA 保持可见。
- 错误使用 `role=alert`，加载状态使用 `aria-live`；焦点在弹窗打开/关闭和状态切换后可预测恢复。
- 不用静态假状态补齐页面；所有状态来自 API 或明确的 local fixture 标记。

## 7. 本地验收夹具

新增幂等、可清理的 `MOCHAT-LOCAL-ACCEPTANCE-20260827` 数据集：

- 一个明确标记的租户、Dashboard 管理员和唯一企业；
- self-built current 与 third-party candidate 两种集成；
- 一组真实形状的 GetChatData/DecryptData 响应；
- 确定性的 PNG、WAV、MP4 最小容器和 PDF/文本文件字节；
- Provider stub 按固定 indexbuf 分片返回；
- 所有业务消息必须通过 `BridgeSource → SyncService → MySQLStore → media worker` 产生；禁止直接插最终消息表伪造成功。

fixture 命令支持 `seed`、`verify`、`cleanup`，重复 seed 不产生重复数据。cleanup 只删除具名 namespace/source/run/tenant，不使用宽泛条件。

## 8. 测试矩阵

### 8.1 Go 与数据库

- SDK：真实形状 JSON、RSA PKCS#1 v1.5、错误 key version、全部 symbol 释放、GetMediaData 分片/空分片/错误码。
- durable：游标推进、失败不推进、重放幂等、message 后 cursor 前崩溃、旧 lease fence、重启接管、跨租户/source 冲突。
- media：分片续传、完成校验、缺失、损坏、路径穿越、Range、HEAD、404/403 等价、消息正文不因媒体失败回滚。
- integration：两种模式、加密不回显、candidate 验证、corp 冲突、缺 scope、在途任务阻塞、version 冲突、切换/回滚审计、旧 generation 拒绝。
- activation：有效/过期/已用/撤销/无效、并发单消费、重发使旧 token 失效、响应不泄密。
- migration：发现、apply→down→apply、MySQL 5.7/MariaDB 方言、FK/索引、回填 fake corp 不激活。

### 8.2 前端

- SaaS：两种模式、候选保存、验证状态、切换确认与阻塞、审计、激活链接复制/清理/过期/已激活/重发。
- Dashboard：fragment token 清理、预检全状态、提交失败恢复、登录 CTA、媒体 ready/pending/failed/missing/corrupt renderer。
- 四前端分别执行 lint、typecheck、test、build；Sidebar/Operation 验证没有误接 Dashboard 激活凭据。

### 8.3 门禁和浏览器

- `go test ./... -count=1`；
- 四前端 lint/typecheck/test/build；
- Phase 4 RBAC、Provider completion、Dashboard 全页证据、圆弧 benchmark、迁移合同、`git diff --check`；
- MariaDB integration 无 DSN 时只能记录 `SKIP`；若本地独立 Docker MariaDB 可用，则提供 DSN 实跑；
- 独立 compose project 和端口完成迁移、健康、重启续跑；
- 浏览器点击 SaaS 模式切换保护、激活全状态、Dashboard 文本/图片/音频/视频/文件回读，检查 console、network、刷新、错误态、桌面和 390×844。

## 9. 安全、回滚与清理

- Secret、permanent authorization code、RSA 私钥、token、MFA secret 不进入日志、审计、URL query、localStorage、截图或 Git。
- 媒体对象路径由服务端生成，不接受客户端路径；文件名只作展示并安全编码。
- 关闭新 archive worker 和 integration API feature flag 可停止新行为；legacy 数据结构保留一个版本周期，不自动 fallback。
- 模式回滚是显式、审计化的 generation 切换，不是异常时静默恢复旧 secret。
- down migration 只删除本专项新增表、索引和权限；不删除已有消息、企业绑定或激活历史。
- fixture cleanup 精确按 `MOCHAT-LOCAL-ACCEPTANCE-20260827` namespace、租户 external ID 和对象根目录执行。

## 10. 自审结论

- 无 `TBD`、`TODO` 或依赖用户选择的设计空位。
- 三个子系统共享 tenant/corp principal、generation fence 与审计语义，但文件和事务边界独立，可分任务实施。
- 本地 fixture 与真实企微线上验收边界已明确；设计不把本地 Provider stub 描述为线上可用。
- 媒体失败不影响正文游标、模式切换不破坏 current、激活 token 不留在 query/地址栏，均有对应失败测试和回滚路径。
- 设计未要求真实企业微信、远端推送或公网部署，符合本次授权边界。
