# Phase 7 真实企业微信会话存档实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 使用企业微信官方会话内容存档 C SDK，在本地构建 Linux amd64 镜像并部署到阿里云 ECS，完成真实会话主动拉取、媒体下载、企业微信回调被动接收和 Dashboard 回读闭环。

**Architecture:** 新增 Debian/glibc `mochat-wecom-archive-bridge` sidecar，以项目内薄 `cgo` 适配层调用官方 SDK，只通过同机 Unix socket 服务主应用；主应用把 bridge 包装为 `archive.ArchiveSource`，复用 `0138` durable cursor/lease/source/audit。现有企业微信回调入口继续负责被动事件，服务器只加载本地构建镜像，不执行源码编译。

**Tech Stack:** Go 1.26、cgo、企业微信官方 Linux x86_64 C SDK、Unix domain socket、MariaDB/MySQL 5.7、Redis、Docker BuildKit、Docker Compose、PowerShell、Bash、Playwright。

## Global Constraints

- 所有用户可见文档使用中文；代码标识符、SDK API、命令和 machine code 保留英文。
- SDK 二进制只存在于 `.local-sdk/wecom-finance/linux-amd64/`，不得提交 Git。
- 服务器禁止 `go build`、`pnpm build` 和 `docker build`；只允许 `docker load` 本地工件。
- 不在仓库、命令参数、日志、截图、测试报告或 shell history 中保存服务器口令、Chat Secret、RSA 私钥、回调 Token/AES Key。
- 部署前轮换已在对话中出现的服务器口令，改用 SSH key；主计划不使用密码自动化。
- 不执行 `down -v`、`volume rm`、`volume prune`、`system prune`；MySQL、Redis 和现有 named volumes 不重建。
- 会话正文只能由主动 SDK 拉取获得；callback 只作为被动事件链路，不作为会话完整性来源。
- `ready` 必须同时有真实 SDK pull、解密、durable 落库、Dashboard 回读和真实 callback 证据。
- Phase 7 live 验收期间保持 AI 自动分析关闭。

---

### Task 1: 固化 SDK 供应链与构建输入

**Files:**
- Create: `third_party/wecom-finance-sdk/README.zh-CN.md`
- Create: `third_party/wecom-finance-sdk/checksums.txt`
- Create: `scripts/verify_wecom_finance_sdk.ps1`
- Test: `scripts/verify_wecom_finance_sdk.test.ps1`
- Modify: `.gitignore`
- Modify: `.dockerignore`

**Interfaces:**
- Consumes: `.local-sdk/wecom-finance/linux-amd64/libWeWorkFinanceSdk_C.so` 与官方头文件。
- Produces: 本地 SDK 校验结果 `{version, platform, sha256, licenseReviewed}`，供 bridge Docker 构建使用。

- [ ] **Step 1: 写 RED 校验测试**

测试必须覆盖 SDK 目录缺失、`.so` 缺失、头文件缺失、checksum 不匹配、ELF 架构不是 x86_64、许可证核对未确认六种失败，并断言错误不输出文件内容。

- [ ] **Step 2: 运行 RED**

Run: `pwsh -File scripts/verify_wecom_finance_sdk.test.ps1`
Expected: FAIL，原因是校验脚本不存在。

- [ ] **Step 3: 实现校验脚本**

脚本只接受 `-SdkRoot`，默认值为仓库下 `.local-sdk/wecom-finance/linux-amd64`；使用 `Get-FileHash -Algorithm SHA256`，读取 `checksums.txt`，并通过容器内 `file`/`readelf` 确认 ELF64 x86-64。输出只包含版本、平台、文件名和摘要。

- [ ] **Step 4: 锁定忽略规则**

`.gitignore` 与 `.dockerignore` 排除 `.local-sdk/`；SDK 只通过 BuildKit bind/secret mount进入 bridge build，不进入普通 app build context。

- [ ] **Step 5: 运行 GREEN 与泄漏检查**

Run: `pwsh -File scripts/verify_wecom_finance_sdk.test.ps1`
Run: `git check-ignore .local-sdk/wecom-finance/linux-amd64/libWeWorkFinanceSdk_C.so`
Expected: 测试 PASS，SDK 文件被 Git 忽略。

- [ ] **Step 6: 提交**

```powershell
git add .gitignore .dockerignore third_party/wecom-finance-sdk scripts/verify_wecom_finance_sdk.ps1 scripts/verify_wecom_finance_sdk.test.ps1
git commit -m "build(archive): verify official WeCom finance SDK input"
```

### Task 2: 建立可替换的 SDK 接口与 fake 实现

**Files:**
- Create: `internal/wecomarchive/sdk/client.go`
- Create: `internal/wecomarchive/sdk/model.go`
- Create: `internal/wecomarchive/sdk/errors.go`
- Create: `internal/wecomarchive/sdk/fake.go`
- Test: `internal/wecomarchive/sdk/client_test.go`

**Interfaces:**
- Produces:

```go
type Config struct {
    WXCorpID  string
    Secret    string
    PrivateKeys map[uint32]string
}

type Client interface {
    GetChatData(ctx context.Context, seq uint64, limit uint64, timeout time.Duration) (ChatPage, error)
    GetMediaData(ctx context.Context, sdkFileID, indexBuf string, timeout time.Duration) (MediaChunk, error)
    Close() error
}

type Factory interface {
    Open(context.Context, Config) (Client, error)
}
```

- `ChatPage` 保存 `Records []EncryptedRecord`；每条记录必须包含 `Seq`、`MsgID`、`PublicKeyVersion`、`EncryptRandomKey` 和 `EncryptChatMsg`。
- 稳定错误码只允许 `sdk.params`、`sdk.network`、`sdk.parse`、`sdk.system`、`sdk.secret`、`sdk.file_id`、`sdk.decrypt`、`sdk.private_key_missing`、`sdk.encrypt_key`、`sdk.ip_denied`、`sdk.data_expired`、`sdk.certificate`。

- [ ] **Step 1: 写接口合同 RED 测试**

覆盖 limit `0`、limit 超上限、缺企业 ID/Secret、缺消息对应私钥版本、context cancel、Close 幂等和错误文本不包含 Secret/PEM。

- [ ] **Step 2: 运行 RED**

Run: `go test ./internal/wecomarchive/sdk -count=1`
Expected: FAIL，package 不存在。

- [ ] **Step 3: 实现公共模型、错误和 fake**

fake 使用内存页和媒体分片，不引用 C；所有测试和非 cgo 门禁均可运行。

- [ ] **Step 4: 运行 GREEN**

Run: `go test ./internal/wecomarchive/sdk -count=1`
Expected: PASS。

- [ ] **Step 5: 提交**

```powershell
git add internal/wecomarchive/sdk
git commit -m "feat(archive): define WeCom finance SDK boundary"
```

### Task 3: 实现 Linux cgo 官方 SDK 适配器

**Files:**
- Create: `internal/wecomarchive/sdk/cgo/client_linux.go`
- Create: `internal/wecomarchive/sdk/cgo/bindings_linux.go`
- Create: `internal/wecomarchive/sdk/cgo/client_stub.go`
- Test: `internal/wecomarchive/sdk/cgo/client_contract_test.go`
- Create: `Dockerfile.archive-bridge`

**Interfaces:**
- Consumes: Task 2 的 `sdk.Factory`/`sdk.Client`。
- Produces: `cgosdk.NewFactory() sdk.Factory`；仅 `linux && cgo` 构建真实实现，其他平台返回 `sdk.platform_unsupported`。

- [ ] **Step 1: 写 cgo 资源生命周期 RED 测试**

通过可注入的 C shim 计数器断言 `NewSdk/Init/GetChatData/DecryptData/GetMediaData/DestroySdk` 成对调用；错误路径同样释放 slice/media/sdk 指针。

- [ ] **Step 2: 运行 RED**

Run inside Linux SDK build container: `go test ./internal/wecomarchive/sdk/cgo -count=1`
Expected: FAIL，真实适配器不存在。

- [ ] **Step 3: 实现最小 cgo wrapper**

wrapper 只负责参数转换、SDK 调用、C 内存复制与释放、错误码归一化。RSA 私钥版本选择发生在调用 `DecryptData` 前；未知版本返回 `sdk.private_key_missing`，不得尝试 active key 猜测解密。

- [ ] **Step 4: 增加 race/并发边界**

如果官方 SDK client 不保证并发安全，则每个 client 内串行调用；不同企业 client 可并行。测试覆盖同 client 并发请求不会交叉释放 C 指针。

- [ ] **Step 5: 构建 bridge 基础镜像**

`Dockerfile.archive-bridge` 使用 Debian/glibc builder 与 runtime，非 root UID/GID 与 app 协调；只复制 bridge binary、批准 `.so` 和必要系统证书。

- [ ] **Step 6: 运行 GREEN**

Run: `docker buildx build --platform linux/amd64 -f Dockerfile.archive-bridge --target test --secret id=wecom_sdk,src=.local-sdk/wecom-finance/linux-amd64 --progress=plain .`
Expected: SDK adapter tests PASS，动态库可解析。

- [ ] **Step 7: 提交**

```powershell
git add internal/wecomarchive/sdk/cgo Dockerfile.archive-bridge
git commit -m "feat(archive): bind official WeCom finance SDK"
```

### Task 4: 实现 Unix socket archive bridge

**Files:**
- Create: `cmd/mochat-wecom-archive-bridge/main.go`
- Create: `internal/wecomarchive/bridge/server.go`
- Create: `internal/wecomarchive/bridge/model.go`
- Test: `internal/wecomarchive/bridge/server_test.go`
- Create: `scripts/smoke_wecom_archive_bridge.sh`

**Interfaces:**
- `GET /healthz` 返回 `{status:"ok", sdkLoaded:true}`，不调用外部网络。
- `POST /v1/archive/pages` 输入 SDK 配置、`seq`、`limit`、`timeoutSeconds`，输出 `{messages,nextSeq,hasMore}`。
- `POST /v1/archive/media/chunks` 输出 `{data,outIndexBuf,isFinish,bytes}`。
- server 只允许 Unix socket `/run/mochat-wecom-archive/archive.sock`，请求体最大 4 MiB，socket mode `0660`。

- [ ] **Step 1: 写 HTTP/Unix socket RED 测试**

覆盖 TCP listen 被拒绝、socket 权限、health 不调用 SDK、page 解密、媒体分片、请求上限、timeout、错误码、日志不含 Secret/PEM/明文。

- [ ] **Step 2: 运行 RED**

Run: `go test ./internal/wecomarchive/bridge -count=1`
Expected: FAIL，bridge package 不存在。

- [ ] **Step 3: 实现 bridge server**

使用 Task 2 的 `sdk.Factory`，每个请求通过 `defer client.Close()` 释放资源；响应不得回显输入凭据或 SDK 原始 body。

- [ ] **Step 4: 实现命令入口和优雅退出**

SIGTERM 时停止接收新请求、等待在途请求至配置超时、关闭 listener 并移除 socket 文件。

- [ ] **Step 5: 运行 GREEN 与容器 smoke**

Run: `go test ./internal/wecomarchive/bridge -count=1`
Run: `bash scripts/smoke_wecom_archive_bridge.sh`
Expected: fake SDK page/media 通过 Unix socket 返回，未监听 TCP。

- [ ] **Step 6: 提交**

```powershell
git add cmd/mochat-wecom-archive-bridge internal/wecomarchive/bridge scripts/smoke_wecom_archive_bridge.sh
git commit -m "feat(archive): serve finance SDK over a local Unix socket"
```

### Task 5: 扩展版本化 RSA keyring

**Files:**
- Modify: `internal/wecomcredentials/manager.go`
- Modify: `internal/wecomcredentials/manager_test.go`
- Modify: `internal/companyprofile/model.go`
- Modify: `internal/store/company_profile.go`
- Test: `internal/store/company_profile_integration_test.go`
- Create: `deploy/standalone/migrations/0140_wecom_archive_live.up.sql`
- Create: `deploy/standalone/migrations/0140_wecom_archive_live.down.sql`
- Modify: `internal/migration/migration_test.go`

**Interfaces:**
- `CorpCredential` 新增：

```go
ArchiveRSAActiveVersion uint32            `json:"archiveRsaActiveVersion,omitempty"`
ArchiveRSAPrivateKeys   map[string]string `json:"archiveRsaPrivateKeys,omitempty"`
```

- 旧 `ArchiveRSAPrivateKey` 仅用于兼容读取，保存新配置时必须转换为带明确版本的 keyring。
- `0140` 只增加媒体 outbox/object 状态与必要索引，不以明文列保存 keyring。

- [ ] **Step 1: 写 keyring RED 测试**

覆盖旧 key 迁移、新旧版本并存、active version、缺版本失败、轮换后旧消息仍可解密、密文中不出现 PEM、跨 tenant/corp AAD 解密失败。

- [ ] **Step 2: 运行 RED**

Run: `go test ./internal/wecomcredentials ./internal/store -run 'ArchiveRSA|ArchiveKeyring' -count=1`
Expected: FAIL，新字段和转换不存在。

- [ ] **Step 3: 实现 keyring 与管理 API 合同**

禁止删除仍被未完成 run 或媒体 outbox 引用的私钥版本；管理操作继续使用乐观锁、审计和加密凭据存储。

- [ ] **Step 4: 写并验证 0140 迁移**

真实 MariaDB 临时 schema 执行 apply→down→apply；验证 `mochat_go_archive_media_jobs` 和 `mochat_go_archive_media_objects` 的 tenant/corp/msgid 复合隔离、幂等键、lease token 和状态索引。

- [ ] **Step 5: 运行 GREEN**

Run: `go test ./internal/wecomcredentials ./internal/store ./internal/migration -run 'ArchiveRSA|ArchiveKeyring|WeComArchiveLive' -count=1`
Expected: PASS 或在未提供集成 DSN 时仅真实 MariaDB 用例明确 SKIP。

- [ ] **Step 6: 提交**

```powershell
git add internal/wecomcredentials internal/companyprofile internal/store/company_profile.go internal/store/company_profile_integration_test.go deploy/standalone/migrations/0140_wecom_archive_live.* internal/migration/migration_test.go
git commit -m "feat(archive): persist versioned RSA keyring and media jobs"
```

### Task 6: 把 bridge 接入真实 ArchiveSource 和 0138 durable sync

**Files:**
- Create: `internal/modules/providers/archive/wecom/bridge_client.go`
- Modify: `internal/modules/providers/archive/wecom/archive.go`
- Modify: `internal/modules/providers/archive/wecom/archive_test.go`
- Create: `internal/dashboard/work_message_archive_source_cron.go`
- Test: `internal/dashboard/work_message_archive_source_cron_test.go`
- Modify: `cmd/mochat-go/main.go`
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Deprecate: `internal/dashboard/work_message_archive_sync_cron.go` 的 production composition

**Interfaces:**
- `wecom.NewWithBridge(config Config, bridge BridgeClient) (*Archive, error)` 返回真实 `ArchiveSource`。
- `BridgeClient.FetchPage(context.Context, BridgePageRequest) (BridgePage, error)` 通过 Unix socket 请求。
- 新 cron 对每个企业调用 `archive.SyncService.Sync`，不再直接更新 `mc_work_message_id.type=40`。

- [ ] **Step 1: 写 ArchiveSource RED 测试**

覆盖 source identity、seq/nextSeq、hasMore、多消息类型、未知 `publickey_ver`、bridge timeout、SDK 10009、失败不返回 partial success、响应日志脱敏。

- [ ] **Step 2: 运行 RED**

Run: `go test ./internal/modules/providers/archive/wecom -count=1`
Expected: FAIL，bridge source 未实现。

- [ ] **Step 3: 实现 bridge client 与规范消息映射**

Unix socket transport 禁止 fallback TCP；消息类型未知时保存原始类型和结构但标为受限，不静默丢弃；无法安全识别来源/收件人时本轮失败且 cursor 不推进。

- [ ] **Step 4: 写 scheduler RED 测试**

覆盖多企业隔离、单企业并发 fence、连续翻页、空页、失败重试、重启续传、旧 cron 不再 production compose。

- [ ] **Step 5: 实现 scheduler 与配置**

新增 `MOCHAT_GO_WECOM_ARCHIVE_BRIDGE_SOCKET`，默认 `/run/mochat-wecom-archive/archive.sock`；启用真实 archive cron 时必须存在 socket 且凭据加密强制开启。

- [ ] **Step 6: 运行 GREEN**

Run: `go test ./internal/modules/providers/archive/... ./internal/dashboard ./internal/config -run 'Archive|WorkMessageArchiveSource' -count=1`
Expected: PASS。

- [ ] **Step 7: 提交**

```powershell
git add internal/modules/providers/archive/wecom internal/dashboard/work_message_archive_source_cron* cmd/mochat-go/main.go internal/config
git commit -m "feat(archive): synchronize real WeCom pages through durable source runs"
```

### Task 7: 实现媒体分片下载与鉴权回读

**Files:**
- Create: `internal/wecomarchive/media/service.go`
- Create: `internal/wecomarchive/media/worker.go`
- Test: `internal/wecomarchive/media/service_test.go`
- Create: `internal/store/archive_media.go`
- Test: `internal/store/archive_media_integration_test.go`
- Modify: `cmd/mochat-go/main.go`
- Modify: `internal/modules/chat-media` 中现有鉴权读取路径

**Interfaces:**
- 正文落库后对含 `sdkfileid` 的消息创建唯一媒体 job。
- worker 使用 `{jobID,attempt,leaseToken}` fencing，循环调用 `GetMediaData(indexBuf,sdkFileID)`。
- 临时文件固定写入 `/app/storage/upload/static/archive/.staging/<job-id>.part`，完成后校验大小与 MD5，原子移动到 tenant/corp/msgid 作用域目录。

- [ ] **Step 1: 写媒体 RED 测试**

覆盖 512 KiB 多分片、重复分片、`indexBuf` 停滞、MD5 不匹配、磁盘不足、stale lease、重启续传、重复 job 幂等和跨企业下载拒绝。

- [ ] **Step 2: 运行 RED**

Run: `go test ./internal/wecomarchive/media ./internal/store -run ArchiveMedia -count=1`
Expected: FAIL，媒体 service/store 不存在。

- [ ] **Step 3: 实现 service/store/worker**

`.part` 永不被 HTTP 暴露；失败保留安全诊断和重试状态，不在日志输出 `sdkfileid` 全值或消息正文。

- [ ] **Step 4: 接入现有媒体读取鉴权**

沿用 Dashboard principal、tenant/corp/employee 范围；URL 不携带 Secret、私钥或对象物理路径。

- [ ] **Step 5: 运行 GREEN 与真实 MariaDB 测试**

Run: `go test ./internal/wecomarchive/media ./internal/store ./internal/modules/chat-media/... -run 'ArchiveMedia|ChatMedia' -count=1`
Expected: PASS，临时 schema 和临时文件 leftovers=0。

- [ ] **Step 6: 提交**

```powershell
git add internal/wecomarchive/media internal/store/archive_media* internal/modules/chat-media cmd/mochat-go/main.go
git commit -m "feat(archive): download and serve archived media safely"
```

### Task 8: 加固并验收企业微信被动回调

**Files:**
- Modify: `internal/dashboard/wework_callback.go`
- Modify: `internal/dashboard/wework_callback_test.go`
- Modify: `internal/dashboard/wework_callback_worker.go`
- Modify: `internal/dashboard/wework_callback_worker_test.go`
- Modify: `internal/store/redis.go`
- Create: `scripts/smoke_wecom_callback_live.ps1`
- Modify: `deploy/standalone/docker-compose.yml`

**Interfaces:**
- GET 验证在 1 秒内返回解密 `echostr`。
- POST 在 5 秒窗口内完成验签、解密、持久入队并空 `200`；业务处理只在 worker 执行。
- 去重优先使用 `MsgId`；无 `MsgId` 的事件使用 corp/event/changeType/entityID/createTime 的稳定摘要。

- [ ] **Step 1: 写 callback RED 测试**

覆盖 URL decode、签名篡改、timestamp 重放窗口、错误 CorpID、XML bomb/body 上限、重复 POST、Redis 失败返回非 2xx、响应不带 BOM/换行和日志脱敏。

- [ ] **Step 2: 运行 RED**

Run: `go test ./internal/dashboard ./internal/store -run 'WeWorkCallback|CallbackReplay' -count=1`
Expected: 新增安全合同至少一项失败。

- [ ] **Step 3: 最小加固 handler/worker**

保持现有业务 listener，不在回调请求线程执行外部 API；队列 envelope 记录稳定 event ID、tenant/corp 与接收时间。

- [ ] **Step 4: 编写 live 验收脚本**

脚本只读取受保护环境文件，不生成或回显 Token/AES Key；检查公开 HTTPS URL、最近 callback audit、worker execution 和 dead-letter 增量。

- [ ] **Step 5: 运行 GREEN**

Run: `go test ./internal/dashboard ./internal/store -run 'WeWorkCallback|CallbackReplay' -count=1`
Expected: PASS。

- [ ] **Step 6: 提交**

```powershell
git add internal/dashboard/wework_callback* internal/store/redis.go scripts/smoke_wecom_callback_live.ps1 deploy/standalone/docker-compose.yml
git commit -m "fix(wecom): harden passive callback ingestion"
```

### Task 9: Dashboard 状态、真实数据与告警闭环

**Files:**
- Modify: `internal/companyprofile/provider_status.go`
- Modify: `internal/providerstatus/service.go`
- Modify: `web/apps/dashboard/src/features/provider-status/provider-status-page.tsx`
- Modify: `web/apps/dashboard/src/features/provider-status/provider-status-page.test.tsx`
- Modify: `web/apps/dashboard/src/features/phase33/*` 的会话页面（只补真实状态/媒体展示所需改动）
- Create: `web/e2e/tests/phase7-wecom-archive.spec.ts`

**Interfaces:**
- Provider 状态展示 `source=external`、last pull/callback success、cursor lag、media pending/failed、稳定错误码和下一步动作。
- `ready` 判定必须读取真实 `0138` 成功 run，不接受配置存在或 fake source。

- [ ] **Step 1: 写状态 RED 测试**

覆盖配置完整但无 live run 为 `limited`、live pull 成功但 callback 未验证为 `limited`、pull+callback+回读完成为 `ready`、stale/failure 降级、普通用户不见敏感诊断。

- [ ] **Step 2: 运行 RED**

Run: `go test ./internal/companyprofile ./internal/providerstatus -run Archive -count=1`
Run: `corepack pnpm --filter @mochat/dashboard test -- provider-status phase33`
Expected: 新状态合同失败。

- [ ] **Step 3: 实现状态投影和页面反馈**

页面就近显示 loading/error/empty/limited，不把 secret、PEM、socket 路径或原始 SDK body返回浏览器。

- [ ] **Step 4: 编写 Playwright live 用例**

用例验证 external 数据源、双向单聊、群聊、撤回、媒体、数据范围、刷新后持久化和 Provider 状态；没有真实数据时必须 SKIP 而不是造 fixture PASS。

- [ ] **Step 5: 运行 GREEN**

Run: `go test ./internal/companyprofile ./internal/providerstatus -run Archive -count=1`
Run: `corepack pnpm --filter @mochat/dashboard typecheck`
Run: `corepack pnpm --filter @mochat/dashboard test`
Expected: PASS。

- [ ] **Step 6: 提交**

```powershell
git add internal/companyprofile internal/providerstatus web/apps/dashboard/src/features web/e2e/tests/phase7-wecom-archive.spec.ts
git commit -m "feat(dashboard): expose verified WeCom archive runtime state"
```

### Task 10: 本地生成不可变 ECS 发布工件

**Files:**
- Create: `scripts/build_phase7_release.ps1`
- Test: `scripts/build_phase7_release.test.ps1`
- Create: `deploy/standalone/docker-compose.phase7.yml`
- Create: `scripts/phase7_release_manifest.py`
- Modify: `Dockerfile`

**Interfaces:**
- Produces `output/phase7-release-<UTC>/images.tar`、`release-manifest.json`、`checksums.sha256`、Compose 文件和部署脚本副本。
- app/bridge 镜像使用源码 fingerprint + Git SHA 标签；manifest 记录架构、镜像 digest、SDK checksum、迁移版本和门禁结果，不含任何 secret。

- [ ] **Step 1: 写 release builder RED 测试**

覆盖 dirty tracked 文件拒绝、SDK 未批准拒绝、错误架构拒绝、镜像缺 healthcheck 拒绝、manifest 含敏感键拒绝、服务器 build 指令拒绝。

- [ ] **Step 2: 运行 RED**

Run: `pwsh -File scripts/build_phase7_release.test.ps1`
Expected: FAIL，builder 不存在。

- [ ] **Step 3: 实现本地构建**

使用 `docker buildx build --platform linux/amd64 --load` 分别构建 app 与 bridge；执行全量 Go、前端、Provider、RBAC、identity、migration、bridge 和 compose 配置门禁后才 `docker save -o images.tar`。

- [ ] **Step 4: 运行容器级集成 smoke**

使用临时 Compose project 和 fake SDK backend，验证 app→Unix socket→bridge→MariaDB durable sync；结束后只删除该具名临时 project/volume，并核对目标 ECS/桌面项目未被触碰。

- [ ] **Step 5: 生成工件并校验**

Run: `pwsh -File scripts/build_phase7_release.ps1`
Expected: 输出 release 目录、两个不可变 image digest、`images.tar` SHA-256、迁移 `0140` 和全部门禁 PASS。

- [ ] **Step 6: 提交构建定义**

```powershell
git add scripts/build_phase7_release.ps1 scripts/build_phase7_release.test.ps1 scripts/phase7_release_manifest.py deploy/standalone/docker-compose.phase7.yml Dockerfile
git commit -m "build(release): package Phase 7 ECS images locally"
```

### Task 11: ECS 安全部署与回滚

**Files:**
- Create: `scripts/deploy_phase7_release.sh`
- Create: `scripts/verify_phase7_ecs.sh`
- Create: `docs/runbooks/2026-08-17-phase7-ecs-deployment.zh-CN.md`

**Interfaces:**
- 脚本输入只有 release 目录和受保护环境文件路径；不接受命令行密码。
- 服务器只执行 checksum、backup、`docker load`、一次性 migration、`docker compose --no-build`、health/readback/rollback。

- [ ] **Step 1: 写 shell 静态合同 RED 测试**

断言脚本包含精确目标目录检查、容器/卷前后快照、磁盘阈值、备份、checksum、`--no-build`、迁移、health、回滚；并拒绝 `docker build`、`down -v`、`volume rm/prune`、`system prune`、口令参数。

- [ ] **Step 2: 实现部署与验证脚本**

release 解压到新目录；当前 release 通过符号链接切换。先启动 bridge 并检查 Unix socket，再替换 app；数据库迁移 forward-only，应用镜像保留上一个 digest用于兼容回滚。

- [ ] **Step 3: 本地 shellcheck 与 fixture 验证**

Run: `bash -n scripts/deploy_phase7_release.sh scripts/verify_phase7_ecs.sh`
Run: `shellcheck scripts/deploy_phase7_release.sh scripts/verify_phase7_ecs.sh`
Expected: PASS。

- [ ] **Step 4: 轮换登录口令并配置 SSH key**

这是进入真实服务器前的硬门禁。确认 SSH key 登录成功后，不再使用已暴露口令；服务器目标通过本地环境变量 `MOCHAT_PHASE7_ECS_HOST` 提供。

- [ ] **Step 5: 上传并部署**

使用 `scp`/`sftp` 上传 release 工件；在 ECS 上执行 `deploy_phase7_release.sh`。保存部署前后容器、镜像、卷、迁移、health 和备份证据。

- [ ] **Step 6: 提交 runbook 与脚本**

```powershell
git add scripts/deploy_phase7_release.sh scripts/verify_phase7_ecs.sh docs/runbooks/2026-08-17-phase7-ecs-deployment.zh-CN.md
git commit -m "docs(deploy): add safe Phase 7 ECS release runbook"
```

### Task 12: 真实企业微信联合验收与阶段收口

**Files:**
- Create: `docs/phases/phase-7-wecom-archive/acceptance/2026-08-17-phase7-live-acceptance.zh-CN.md`
- Modify: `docs/phases/phase-7-wecom-archive/README.md`
- Modify: `docs/PROJECT_PROGRESS.zh-CN.md`

**Interfaces:**
- 真实验收报告逐项区分 `PASS`、`FAIL`、`SKIP`，包含证据摘要但不包含凭据、消息全文或个人敏感信息。

- [ ] **Step 1: 完成企业微信后台配置**

确认会话存档已购买/开启、测试员工在开启范围、RSA 公钥版本生效、ECS 出口 IP 在白名单、回调 HTTPS URL 验证通过，并完成告知/授权记录。

- [ ] **Step 2: 生成最小真实测试矩阵**

在专用测试员工/客户/群范围内产生双向文本、群聊、撤回、图片或文件；记录时间窗口和脱敏参与人标识，不在验收文档复制正文。

- [ ] **Step 3: 验证主动拉取**

核对 `GetChatData` run、seq 单调推进、消息幂等、source=`external`、重启续传、失败不推进和 Dashboard 五页回读。

- [ ] **Step 4: 验证媒体与被动回调**

核对媒体分片/MD5/鉴权读取；触发至少一个真实 callback 事件，确认 ingress→验签解密→Redis→worker→DB/audit 完整链路。

- [ ] **Step 5: 验证权限、安全与恢复**

普通用户只能读允许员工范围；超管可读企业范围；日志/浏览器/证据无 secret；重启 app/bridge 后无重复；执行一次镜像回滚演练并确认数据卷不变。

- [ ] **Step 6: 运行最终全量门禁**

Run: `go test ./... -count=1`
Run: `corepack pnpm lint && corepack pnpm typecheck && corepack pnpm test && corepack pnpm build`
Run: `corepack pnpm check:provider-completion-real`
Run: `corepack pnpm check:phase4-dashboard-page-rbac`
Run: `git diff --check`
Expected: 全部 PASS；真实 Provider gate 不允许 integration SKIP。

- [ ] **Step 7: 更新阶段状态并提交**

只有所有硬门禁通过才把 Phase 7 标为完成；否则 README 与总进度保留“进行中/limited”并列出精确 blocker。

```powershell
git add docs/phases/phase-7-wecom-archive docs/PROJECT_PROGRESS.zh-CN.md
git commit -m "docs(phase7): record live WeCom archive acceptance"
```
