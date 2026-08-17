# 企业微信会话存档双链路 Demo Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 构建、部署并验证一个与现有 ECS 服务完全隔离的企业微信会话存档主动拉取和事件回调 Demo。

**Architecture:** 单个 Go 二进制提供公网回调监听器和仅本机可访问的管理监听器，Linux 构建通过 CGO 链接企业微信官方 Finance SDK。配置、密钥和 JSONL 证据均使用独立 bind mount，发布物在本地构建后通过 `docker save/load` 部署。

**Tech Stack:** Go 1.26、标准库 HTTP/crypto/XML/JSON、CGO、企业微信 Finance SDK v3、Debian 12、Docker。

## Global Constraints

- 不修改或重启现有 MoChat、Nginx、MySQL、Redis、容器网络和数据卷。
- 公网端口固定 `19090`，管理端口只绑定 `127.0.0.1:19091`。
- SDK 归档 SHA-256 必须是 `afa8c017da2994ad2215933f2fcc6042d40d935663ad42d6e1e9d7716652f0d8`。
- 私钥、Secret、Admin Token 不写入镜像、Git、服务日志或公网响应。
- 服务器不得编译；只允许校验、加载本地镜像和启动容器。

---

### Task 1: 回调密码学与证据存储

**Files:**
- Create: `internal/wecomarchivedemo/callback.go`
- Create: `internal/wecomarchivedemo/callback_test.go`
- Create: `internal/wecomarchivedemo/store.go`
- Create: `internal/wecomarchivedemo/store_test.go`

**Interfaces:**
- `VerifyAndDecryptCallback(token, aesKey, expectedReceiveID string, values url.Values, encrypted string) (CallbackPlaintext, error)`
- `EvidenceStore.AppendCallback(CallbackEvidence) error`
- `EvidenceStore.LoadState() (State, error)` / `SaveState(State) error`

- [ ] **Step 1: 写回调验签、AES 解密、ReceiveID 绑定、篡改拒绝和原子状态写入测试。**
- [ ] **Step 2: 运行 `go test ./internal/wecomarchivedemo -run 'Callback|Store' -count=1`，确认因实现缺失而失败。**
- [ ] **Step 3: 最小实现 SHA-1 排序签名、32 字节 PKCS#7、XML 上限和 JSONL/原子状态写入。**
- [ ] **Step 4: 重跑相同测试并确认通过。**

### Task 2: 主动拉取领域服务与 Linux SDK 适配器

**Files:**
- Create: `internal/wecomarchivedemo/archive.go`
- Create: `internal/wecomarchivedemo/archive_test.go`
- Create: `internal/wecomarchivedemo/sdk.go`
- Create: `internal/wecomarchivedemo/sdk_linux.go`
- Create: `internal/wecomarchivedemo/sdk_stub.go`

**Interfaces:**
- `SDK.GetChatData(seq uint64, limit uint32, timeoutSeconds int) ([]byte, error)`
- `SDK.DecryptData(randomKey, encryptedMessage string) ([]byte, error)`
- `ArchiveService.Pull(context.Context) (PullResult, error)`

- [ ] **Step 1: 用 fake SDK 写 seq 推进、空页、RSA PKCS#1、错误公钥、SDK 错误和重复消息测试。**
- [ ] **Step 2: 运行 `go test ./internal/wecomarchivedemo -run Archive -count=1`，确认失败原因是功能缺失。**
- [ ] **Step 3: 实现归档响应解析、RSA PKCS#1 解密、官方 `DecryptData` 调用、证据记录和成功后 seq 推进。**
- [ ] **Step 4: 用 build tag 实现 Linux CGO SDK，非 Linux stub 明确报告不可用。**
- [ ] **Step 5: 重跑测试并确认通过。**

### Task 3: 双监听器服务和配置生成

**Files:**
- Create: `internal/wecomarchivedemo/server.go`
- Create: `internal/wecomarchivedemo/server_test.go`
- Create: `internal/wecomarchivedemo/config.go`
- Create: `internal/wecomarchivedemo/config_test.go`
- Create: `cmd/wecom-archive-demo/main.go`

**Interfaces:**
- 公网：`/healthz`、`/wecom/callback`；本机管理：`/admin/status`、`/admin/pull`。
- `keygen --output <dir>` 生成 config、RSA 密钥和可交付填写清单；`serve` 启动双监听器。

- [ ] **Step 1: 写公网/管理路由隔离、Bearer 鉴权、敏感字段不回显和 keygen 格式测试。**
- [ ] **Step 2: 运行 `go test ./internal/wecomarchivedemo ./cmd/wecom-archive-demo -count=1`，确认失败。**
- [ ] **Step 3: 实现两个 `http.Server`、优雅退出、配置校验和密钥生成。**
- [ ] **Step 4: 重跑测试并确认通过。**

### Task 4: 可复现本地镜像与安全部署

**Files:**
- Create: `deploy/wecom-archive-demo/Dockerfile`
- Create: `deploy/wecom-archive-demo/entrypoint.sh`
- Create: `scripts/build_wecom_archive_demo.ps1`
- Create: `scripts/deploy_wecom_archive_demo.sh`
- Create: `scripts/configure_wecom_archive_demo.sh`
- Create: `scripts/verify_wecom_archive_demo.ps1`
- Create: `docs/runbooks/2026-08-17-wecom-archive-demo.zh-CN.md`

**Interfaces:**
- 本地产物：`output/wecom-archive-demo-release/image.tar`、`checksums.sha256`、`secrets/`、部署脚本和填写清单。
- ECS 目标：`/opt/wecom-archive-demo`、容器 `wecom-archive-demo`、公网 `19090`、本机 `19091`。

- [ ] **Step 1: 写静态合同测试，拒绝服务器 build、宽泛删除、现有端口/目录/容器冲突和敏感值入镜像。**
- [ ] **Step 2: 实现 Dockerfile、本地构建、交互配置、部署和验证脚本。**
- [ ] **Step 3: 运行全量 Go 测试、Linux/amd64 镜像构建、容器 callback/SDK smoke、`git diff --check`。**
- [ ] **Step 4: 上传发布包，校验 SHA-256，执行 `docker load` 和隔离启动。**
- [ ] **Step 5: 验证现有容器 ID/状态未变化、19090/19091 绑定正确、公网健康可达，并输出最终填写信息。**

### Task 5: 评审与阶段进度更新

**Files:**
- Modify: `docs/phases/phase-7-wecom-archive/README.md`
- Modify: `docs/PROJECT_PROGRESS.zh-CN.md`
- Create: `docs/phases/phase-7-wecom-archive/acceptance/2026-08-17-wecom-archive-demo.md`

- [ ] **Step 1: 对实现差异、安全边界、错误处理和运维脚本进行代码评审并修复重要问题。**
- [ ] **Step 2: 记录本地与 ECS 验证证据；真实企微尚未配置的项目必须标为 `WAITING_EXTERNAL_CONFIG`，不得伪报 PASS。**
- [ ] **Step 3: 更新 Phase 7 和总进度，运行最终验证后提交。**
