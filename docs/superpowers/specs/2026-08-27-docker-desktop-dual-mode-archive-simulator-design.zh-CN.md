# Docker Desktop 合流与双模式企微归档模拟器设计

日期：2026-08-27

状态：待用户审阅

适用范围：本地开发环境 `mochat-go-desktop`、会话存档 bridge、归档 durable worker、Dashboard 回读和模拟数据清理

## 1. 背景与当前事实

本地目前同时存在两套 Compose 项目：

- `mochat-go-desktop`：原有长期开发环境，保存用户需要保留的 MySQL、Redis、应用文件和审计锚点数据；当前容器已停止或不健康。
- `mochat-wecom-acceptance-20260827`：专项验收环境，包含 app、bridge、MySQL、Redis 和多组验收卷；当前页面使用 `19080` 端口。

Docker 当前逻辑占用约为：镜像 `9.852 GB`、命名卷 `3.664 GB`、构建缓存 `107.1 GB`。其中大部分镜像、非 desktop 卷和可回收构建缓存不属于必须保留的数据。

仓库已有两套互补的会话模拟能力：

1. `mochat-archive-simulator` 可幂等生成和清理多种消息，但上游是进程内 `ArchiveSource`，不能证明 bridge、RSA 解包、SDK 解密或媒体分片合同。
2. `mochat-archive-acceptance` 通过 `GetChatData`、`DecryptData`、`GetMediaData` 形状的 fixture bridge 进入正式 durable 链路，覆盖文本、图片、语音、视频、文件、mixed、缺失和损坏媒体；但当前 `DecryptData` 主要按固定密文查表，不是真正的对称解密，也只模拟自建应用 Finance SDK。

运行时模式现状也必须明确区分：

- `self_built` 已有正式 Finance SDK bridge、主应用 BridgeSource、0138 durable cursor/lease/idempotency、媒体 worker、对象存储和 Dashboard 鉴权读取。
- `third_party_delegated` 已有不可变租户模式、Provider App ID、永久授权码加密存储、能力范围和 SaaS 界面合同，但尚无 suite_ticket、suite_access_token、授权换码、企业 token 或数据与智能专区会话适配器。

所以，仅给当前 fixture 增加一个 `mode` 字段会掩盖真实缺口。本次设计采用两个上游协议适配器汇入一个下游归档账本的方案。

## 2. 目标

1. 只保留一个长期 Compose 项目 `mochat-go-desktop`，其中 bridge 是独立容器，worker 仍运行在 app 进程中。
2. 保留且只保留以下四个 desktop 命名卷：
   - `mochat-go-desktop_mysql-data`
   - `mochat-go-desktop_redis-data`
   - `mochat-go-desktop_app-storage`
   - `mochat-go-desktop_audit-anchor-storage`
3. 清理专项验收栈、其他容器、非保留命名卷、无用镜像和可回收构建缓存；清理前后记录精确对象和逻辑磁盘占用。
4. 把会话 fixture 收敛成 `mochat-go-desktop` 的显式开发 profile，支持自建应用和第三方代开发应用两套不同的真实协议边界。
5. 模拟消息必须经“模拟企微上游 → bridge/回调 → durable worker → 正式消息和媒体表 → Dashboard”生成，不允许直接插入最终消息或媒体表伪造成功。
6. 提供可重复、可审计、可清理的预置场景和单条发送命令，用户可以在本地自行模拟文本、图片、语音、视频、文件及失败场景。
7. 明确本地合同通过与真实企业微信线上通过的边界，不把合同等价的本地密码学实现冒充官方 SDK 字节级实现。

## 3. 非目标

- 本次不调用真实企业微信，不上传真实 Secret、永久授权码、RSA 私钥或真实会话内容。
- 本次不部署公网服务器，不配置真实可信 IP、回调域名或数据专区环境。
- 不在主应用镜像内动态链接官方 Finance SDK；生产 bridge 继续独立运行。
- 不把第三方数据专区的消息正文或媒体导出成普通明文对象，因为真实数据专区要求正文通过展示组件呈现。
- 不更改租户开户时已经选择且不可切换的 `self_built` / `third_party_delegated` 权威模式。
- 不把 Docker 逻辑清理必然等同于 Windows 虚拟磁盘文件立即缩小；若 Docker Desktop 的 VHDX 未自动回收，只记录物理占用边界，不在本次自动重建或格式化虚拟磁盘。

## 4. 方案比较与选择

### 4.1 方案 A：一个通用伪接口，两种模式只改参数

优点是开发量小。缺点是会错误假设第三方代开发与自建应用都使用 `GetChatData/GetMediaData`，无法验证 suite 授权、租户 token 隔离、数据专区密钥或展示组件约束。

结论：拒绝。

### 4.2 方案 B：两套完全独立的 bridge、worker 和消息表

优点是隔离直观。缺点是重复 durable cursor、媒体任务、鉴权、Dashboard 和运维逻辑，长期容易产生两套幂等规则和数据语义。

结论：拒绝。

### 4.3 方案 C：两个上游适配器 + 一个 bridge 容器 + 一个下游归档信封

bridge 根据租户不可变模式选择 `self_built_finance` 或 `third_party_data_zone` 适配器。两个适配器保留不同协议和内容访问策略，但对 app 输出统一的消息身份、参与者、时间、类型、游标和内容访问描述。app/worker 复用现有 0138 账本和 Dashboard 权限。

结论：采用。原因是它同时保持真实协议差异和统一业务处理，不让 bridge 变成两套重复业务系统。

## 5. 总体架构

```text
                 ┌──────────────────────────────────────────────┐
                 │ mochat-go-desktop（同一 Compose project）    │
                 │                                              │
自建 fixture ───▶│ bridge:self_built_finance                    │
 GetChatData     │   RSA 解包 → 对称解密 → 规范消息             │
 GetMediaData    │                                              │
                 │                         ┌──────────────────┐ │
第三方 fixture ─▶│ bridge:third_party_data_zone ──────────────▶│ │
 suite 回调/换码  │   suite token → 企业授权 → 数据专区记录      │ │
 展示密钥/组件    │                         │统一归档信封       │ │
                 │                         └────────┬─────────┘ │
                 │                                  │ Bearer     │
                 │ app + worker                    ▼            │
                 │   durable run/cursor/lease/idempotency       │
                 │   message/source/media/display locator       │
                 │                 │                             │
                 │                 ▼                             │
                 │         MySQL / app-storage / Redis          │
                 └─────────────────┬────────────────────────────┘
                                   ▼
                          Dashboard 鉴权回读
```

生产环境和 fixture 环境共用 bridge HTTP 合同、app adapter 和 durable worker。差异仅在 bridge 的上游驱动：开发 profile 使用确定性 fixture driver；生产 profile 使用官方 SDK 或数据专区运行时。

## 6. Compose 合流设计

### 6.1 长期服务

`deploy/standalone/docker-compose.yml` 最终包含：

- `app`：HTTP、四前端静态资源和后台 worker；继续使用 `MOCHAT_GO_RUNTIME_ROLE=all`。
- `mysql`：复用 `mochat-go-desktop_mysql-data`。
- `redis`：复用 `mochat-go-desktop_redis-data`。
- `archive-bridge`：独立容器，不在公网发布端口，只加入 Compose 内部网络；健康检查只证明进程和所选驱动可用。
- `archive-simulator`：一次性工具服务，属于 `archive-fixture` profile，默认不启动。

bridge 状态使用 `tmpfs` 或可重建的容器状态，不新增命名卷。权威 cursor、任务和幂等事实只在 MySQL；确定性 fixture 队列需要重启保留时，保存到 `app-storage/archive-fixture`，并带数据集目录白名单。

### 6.2 Profile

- 默认/`app`：启动 app、mysql、redis 和生产形态的 archive-bridge；未配置真实上游时 bridge 健康但能力为“未配置”，Dashboard 正常展示空数据。
- `archive-fixture`：bridge 使用本地 fixture driver，同时开放仅容器网络可访问的模拟管理接口。
- 真实 SDK profile 以后使用 `archive-sdk`；要求官方 SDK 构建材料通过校验，不与 fixture 同时启用。

fixture 和真实 SDK 标志互斥，配置同时打开时启动失败。模拟管理接口只在 fixture profile 存在，且仅绑定内部网络和独立高强度 bearer。

### 6.3 端口

长期桌面环境继续使用 `18080` 作为应用入口。专项验收的 `19080` 在合流并验证成功后下线。bridge 不映射宿主机端口。

## 7. 统一 bridge 合同

app 面向 bridge 继续使用受认证的内部 HTTP，但合同升级到显式模式和内容策略：

- `POST /v1/archive/messages`
- `POST /v1/archive/media/chunks`
- `POST /v1/archive/component/session`
- `GET /healthz`

每次请求必须包含 app 从权威绑定解析出的 `tenant_id`、`corp_id`、`wx_corpid` 和 `integration_mode`。bridge 必须再次检查 fixture/生产驱动中登记的企业绑定，跨租户或模式不匹配返回稳定错误，不自动回退另一模式。

统一消息信封包含：

- `source_id`、`source_mode`、`cursor`、`msgid`；
- `action`、发送方、接收方、群 ID、发送时间、消息类型；
- `content_policy=plaintext|component`；
- `content`：仅 `plaintext` 模式存在的正式解析内容；
- `media`：仅允许直接下载的媒体 locator；
- `component_locator`：数据专区消息 ID、公钥版本和加密展示密钥；
- `integrity`：fixture 数据集标记、载荷摘要和协议版本。

bridge 返回中不包含永久授权码、suite secret、Finance Secret、RSA 私钥、解密随机密钥或完整 bearer。日志只记录 tenant/corp 的内部数值 ID、模式、游标、数量、稳定错误码和 request ID。

## 8. 自建应用 Finance SDK 模拟

### 8.1 生产边界

自建模式保持真实生产调用顺序：

1. `GetChatData(seq, limit, timeout)` 返回 `publickey_ver`、`encrypt_random_key` 和 `encrypt_chat_msg`。
2. bridge 根据 `publickey_ver` 选择租户 RSA 私钥，使用 RSA PKCS#1 v1.5 解开随机会话密钥。
3. bridge 调驱动的 `DecryptData` 得到 SDK JSON。
4. 对图片、语音、视频、文件和 mixed 媒体，worker 使用 `GetMediaData(sdkfileid, indexbuf)` 分片下载。
5. 全页解析和业务写入成功后推进 cursor；媒体任务独立重试，不回退正文 cursor。

### 8.2 fixture 密码学

当前查表式 `DecryptData` 改为真正的密码学往返：

- 每个模拟企业生成独立 RSA 2048 keypair，并保留公钥版本。
- 每条消息使用独立 256-bit 内容密钥和随机 nonce。
- fixture 用应用 RSA 公钥加密内容密钥；消息正文使用 AES-256-GCM 加密，并把 corp ID、seq、msgid 和公钥版本作为附加认证数据。
- bridge 使用私钥解包并通过 fixture SDK driver 解密正文；任意密文、标签、corp、seq 或密钥版本被篡改都必须失败，且 cursor 不推进。

AES-GCM 是本地 fixture 的合同等价实现，用于证明真实密钥选择、机密性、完整性和错误边界；它不是对企业微信闭源 `DecryptData` 内部算法的字节级复刻。真实上线仍由官方 SDK 执行 `DecryptData`。

### 8.3 媒体

确定性媒体覆盖 PNG、WAV、MP4、PDF 和 mixed 图片。每个对象保存预期大小、MD5 和 SHA-256。fixture 按小分片返回 `data/indexbuf/is_finish`，可注入：

- 首分片失败；
- 中途断开后续传；
- 缺失对象；
- 内容损坏；
- 错误 indexbuf；
- 重启后从持久化 checkpoint 续传。

## 9. 第三方代开发与数据专区模拟

### 9.1 授权面

第三方 fixture 单独模拟以下流程，不复用自建应用 Secret：

1. 使用企业微信回调 AES 和签名算法向 MoChat 指令回调推送 `suite_ticket`。
2. MoChat 以 `suite_id + suite_secret + suite_ticket` 换取短期 `suite_access_token`。
3. 生成 `pre_auth_code` 和带 state 的授权会话。
4. 授权完成后以一次性 `auth_code` 调 `get_permanent_code`，取得租户独立 `permanent_code` 和授权企业信息。
5. 以后以 `suite_access_token + auth_corpid + permanent_code` 获取企业 access token；缓存按租户、suite 和 corp 隔离，并验证过期、撤销与轮换。

仓库现有 `AuthorizationCredential` 扩展 suite/provider 所需字段，仍由 tenant + integration ID 作为 AES-GCM 附加数据加密；API 和审计只返回非敏感 hint。自建模式字段与第三方字段互斥。

### 9.2 会话数据面

第三方代开发会话存档按数据与智能专区合同模拟：

- 拉取接口返回发送者/接收者 ID、群 ID、发送时间、消息类型、`msgid`、`public_key_ver` 和 `encrypted_secret_key`。
- 每条消息的 secret-key 使用应用 RSA 公钥加密，bridge 用对应私钥解开。
- 普通 app 数据库只保存元数据和加密后的 component locator，不保存正文或原始媒体字节。
- Dashboard 查看时由后端创建短期、一次性、绑定用户/tenant/corp/msgid 的展示会话；本地 fixture component 验证会话后用 `msgid + secret-key` 渲染确定性文本、图片、语音、视频或文件预览。
- 生产环境替换成企微展示组件或数据专区内程序，不改变 Dashboard 的授权和 locator 合同。

这条路径不调用 `GetMediaData`，也不把数据专区内容复制到 `app-storage/archive-media`。原因是数据专区的正文和媒体有独立的数据出区限制，必须使用消息 ID 与 secret-key 的展示组件呈现。

### 9.3 回调与主动拉取

第三方 fixture 可发送两类信号：

- 授权类回调：suite_ticket、授权成功、永久授权码重置、授权变更、取消授权。
- 会话可用通知：仅作为加速同步信号，不携带正文；durable worker 仍主动拉取数据专区记录并使用同一个 cursor。

回调必须执行签名校验、AES 解密、receive ID 检查、时间窗和 nonce 重放保护。回调失败不得创建同步任务；重复通知只能合并为同一 durable run。

## 10. 模拟消息发送入口

新增 `archive-simulator` 一次性命令。它只向 fixture bridge 的内部管理端点提交“上游消息”，不访问 MySQL 最终消息表。

### 10.1 预置场景

```powershell
docker compose -p mochat-go-desktop --profile app --profile archive-fixture `
  -f deploy/standalone/docker-compose.yml run --rm archive-simulator `
  seed --tenant-id <租户ID> --scenario full --mode self_built
```

`full` 场景幂等生成文本、图片、语音、视频、文件、mixed、缺失和损坏媒体。第三方模式把同类内容放入本地展示组件，不写普通媒体对象。

### 10.2 单条文本

```powershell
docker compose -p mochat-go-desktop --profile app --profile archive-fixture `
  -f deploy/standalone/docker-compose.yml run --rm archive-simulator `
  send --tenant-id <租户ID> --mode self_built --type text --text "本地模拟消息"
```

第三方模式只把 `--mode` 改为 `third_party_delegated`。命令会先核对租户权威模式；模式不同则拒绝，不允许为了测试临时切换租户。

### 10.3 媒体

```powershell
docker compose -p mochat-go-desktop --profile app --profile archive-fixture `
  -f deploy/standalone/docker-compose.yml run --rm archive-simulator `
  send --tenant-id <租户ID> --mode self_built --type image --file /fixtures/input/sample.png
```

本地目录只读挂载到 `/fixtures/input`。允许的类型、大小和 MIME 由 simulator 校验；文件内容会复制到具名数据集目录并计算摘要。第三方模式的媒体进入 fixture component，不进入 Finance 媒体分片接口。

### 10.4 触发同步和查看状态

消息入上游队列后有两种处理方式：

- 等待周期 worker；
- 在 Dashboard“唯一企业资料 → 数据同步”点击“立即同步会话”。

CLI 另提供只读 `status`，显示模拟数据集、模式、上游最大 cursor、app 已提交 cursor、消息数、待处理媒体和最近稳定错误码，不展示密钥或正文。

### 10.5 清理

```powershell
docker compose -p mochat-go-desktop --profile app --profile archive-fixture `
  -f deploy/standalone/docker-compose.yml run --rm archive-simulator `
  cleanup --tenant-id <租户ID> --dataset <数据集ID>
```

默认先 dry-run。实际删除要求显式 `--confirm-dataset <数据集ID>`，且只能删除 simulator 注册表记录的消息、对象、展示 locator、run 和 fixture 文件。任何未带 `MOCHAT-LOCAL-SIM` 标记或不在数据集白名单内的记录都拒绝删除。

## 11. 数据模型

复用已有 `mochat_go_archive_simulation_batches/messages`，补充或新建最小协议账本：

- fixture upstream message：数据集、tenant/corp、模式、上游 cursor、加密载荷、状态和摘要；
- fixture upstream media：数据集、模式、locator、字节路径、摘要、分片/失败策略；
- third-party suite event/token：suite、corp、事件类型、加密载荷、有效期、消费状态和重放键；
- archive content locator：`content_policy`、密文 locator、公钥版本、展示状态和过期时间。

所有新表都包含 tenant/corp、唯一幂等键、创建/更新时间和数据集标记。敏感 ciphertext 与 locator 不进入普通审计 JSON；日志和 API 不返回它们。

不增加第二套消息业务表。最终列表、详情、权限、搜索和统计仍读取现有消息表及 source identity；第三方正文通过 `content_policy=component` 延迟展示。

## 12. 失败边界

必须稳定覆盖：

- 租户不存在、租户模式不符、企业绑定不符；
- bridge bearer 错误、fixture 管理 bearer 错误；
- RSA 版本缺失、随机密钥解包失败、AES-GCM 验证失败；
- suite_ticket 签名错误、receive ID 错误、回调过期、nonce 重放；
- suite_access_token 过期、auth_code 重用、permanent_code 撤销、企业 token 跨租户使用；
- page 中任意消息损坏时整页失败且 cursor 不推进；
- 重放同一 msgid/seq 时不重复插入；
- Finance 媒体缺失/损坏/中断与数据专区展示会话过期；
- app、bridge、worker、MySQL 或 Redis 重启后继续运行；
- Dashboard 未登录、跨租户、无页面权限或过期展示会话均拒绝读取。

页面无数据或未配置真实企微时正常返回空列表和业务化空态，不因 bridge 未配置返回全页加载失败。

## 13. Docker 清理与迁移顺序

1. 记录全部 Compose 项目、容器、镜像、卷、网络、构建缓存和 `docker system df`。
2. 对四个 desktop 保留卷逐一核对 Compose label 和实际挂载；创建 MySQL 逻辑备份并放入 `app-storage/backups/local-maintenance-<时间>`，验证备份非空和可读取。
3. 使用新 compose 配置构建 `mochat-go-desktop`，先启动 MySQL/Redis，执行迁移，再启动 bridge 和 app。
4. 验证 desktop 原有租户、登录、核心页面、手动同步空态和重启恢复。
5. 在 desktop 上分别执行自建和第三方 fixture 的发送、同步、回读与清理验收。
6. 停止并删除 `mochat-wecom-acceptance-20260827` 及其卷、网络；不把专项验收数据迁入 desktop。
7. 精确列出所有非 desktop 容器和卷，检查待删除列表不包含四个保留卷后再删除。
8. 清理未使用镜像和 BuildKit/buildx 缓存；保留正在运行的 desktop 镜像。
9. 再次执行 `docker system df`、卷列表、Compose 列表、健康检查和重启恢复，记录实际回收量。

禁止使用未经核对的通配删除；Windows 文件系统操作始终使用解析后的绝对路径。若 Docker Desktop 虚拟磁盘物理文件没有同步缩小，只报告逻辑空间已释放及后续可选压缩步骤，不自动删除或重建 VHDX。

## 14. 测试先行与门禁

### 14.1 Go 测试

- Finance fixture RSA/AES 往返、不同消息不同密钥、篡改失败和无密钥泄漏。
- media 分片、断点、MD5/SHA-256、缺失、损坏和重启续传。
- suite callback 签名/AES、ticket 更新、token 缓存、授权换码、撤销和租户隔离。
- 数据专区 secret-key RSA 解密、component locator 加密存储、展示会话鉴权和过期。
- 两种 adapter 进入同一 durable cursor/lease/idempotency 链路；失败不推进、重放不重复。
- CLI seed/send/status/cleanup 的显式开关、模式校验、路径校验、dry-run 和安全删除。
- 相关包测试与 `go test ./... -count=1`。

### 14.2 数据库和合同测试

- up/down migration 文本合同与 MariaDB integration；无 DSN 时明确记录 `SKIP`。
- 迁移后旧 desktop 数据可读，四个保留卷未替换。
- Provider completion、Phase 4 RBAC、Dashboard archive media/content 权限合同。

### 14.3 前端和构建

- 四前端 lint、typecheck、test、build。
- Dashboard 全局消息/会话详情在 `plaintext` 与 `component` 两种策略下都能正常展示。
- 空数据、同步中、同步失败、媒体失败、展示会话过期均使用普通用户语言。
- Dashboard 页面证据、圆弧 benchmark 和 `git diff --check`。

### 14.4 Docker 端到端

- `docker compose config`、镜像构建、迁移、健康检查。
- 自建模式：发送文本/图片/语音/视频/文件，主动同步，鉴权回读；验证媒体 chunk 和对象摘要。
- 第三方模式：完成 suite_ticket → 授权换码 → 数据专区拉取 → component 展示；验证正文未进入普通数据库/对象存储。
- 注入两种模式失败，验证 cursor 不推进和日志不泄密。
- 重启 app/bridge/worker/MySQL/Redis 后继续同步且不重复。
- 清理一个 fixture 数据集后真实/非 fixture 数据保留。

### 14.5 浏览器验收

- 实际登录 Dashboard，点击“立即同步会话”，检查状态刷新、浏览器刷新恢复和网络请求。
- 在消息列表/详情查看文本、图片、音频、视频和文件；第三方内容经本地展示组件打开。
- 未登录、跨租户和无权限请求被拒绝；控制台无未处理错误。
- 核查空态、失败态、桌面响应式；涉及员工端的视图补做 `390×844`。

## 15. 验收口径

完成后可以宣告：

- 本地 Docker 中两种模式的授权/加密/传递合同通过；
- 自建模式复用了正式 Finance bridge 和媒体下载生产路径；
- 第三方模式复用了 suite 授权、数据专区 locator 和展示组件边界；
- 两种模式共用同一 durable worker、业务消息投影和 Dashboard 权限；
- Docker 只保留 `mochat-go-desktop` 的四个数据卷，并给出实际逻辑回收量。

不能宣告：

- 真实企业微信 Finance SDK 已线上通过；
- 真实第三方服务商资质、suite 回调、授权企业或数据专区程序已线上通过；
- fixture AES-GCM 密文与官方闭源 `DecryptData` 字节完全一致；
- Windows Docker 虚拟磁盘物理文件一定已经压缩到逻辑占用大小。

## 16. 回滚

- 代码回滚：保留合流前镜像摘要和 Git 提交，app/bridge 可切回上一镜像；迁移保持向后兼容，不自动执行破坏性 down。
- 数据回滚：四个 desktop 卷不删除；启动前逻辑备份保存在 app-storage。若迁移失败，停止新 app 并用备份恢复到单独临时数据库验证后再决定恢复。
- Compose 回滚：恢复旧 desktop compose 配置和 `18080` 端口，不恢复已删除的专项验收数据。
- fixture 回滚：按数据集 cleanup，只删除带注册表和 `MOCHAT-LOCAL-SIM` 双重证明的数据。
- Docker 清理不可恢复的对象仅限已核对的非 desktop 容器、卷、镜像和构建缓存；四个保留卷不在清理命令目标中。

## 17. 设计决策摘要

1. bridge 独立容器，worker 留在 app，避免把官方 SDK 动态库和主应用耦合。
2. 两个上游适配器，不把第三方数据专区伪装成 Finance SDK。
3. 一个下游 durable 归档账本，避免两套游标和幂等规则。
4. 自建 fixture 做真实 RSA 解包和对称认证解密，但明确不是闭源 SDK 算法复刻。
5. 第三方正文通过 component 策略展示，不写普通明文媒体对象。
6. simulator 只注入上游，测试数据显式标记、幂等且可清理。
7. Compose 只保留一个项目和四个长期数据卷，清理使用精确对象列表和清理前备份。
