# 双模式企微归档模拟器实施计划

状态：已实施并通过本地验收；逐项结果见专项验收报告。

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在不调用真实企微的前提下，实现自建应用 Finance SDK 与第三方代开发/数据专区两种协议边界的真实加解密、传递、主动同步、Dashboard 回读及 `seed/send/status/cleanup` 操作接口。

**Architecture:** `internal/archivefixture` 持有确定性上游队列、双模式密码学和受控管理 API；`archive-bridge` 对 app 暴露统一归档合同。自建模式输出明文投影和 Finance 媒体 locator，第三方模式只输出元数据和加密 component locator；两者复用 0138 durable worker、最终消息表和 Dashboard RBAC。

**Tech Stack:** Go 1.26、RSA-2048 PKCS#1 v1.5、AES-256-GCM、MariaDB migrations、标准库 HTTP/XML/JSON、React/TypeScript、Vitest。

## Global Constraints

- 租户权威模式只能是不可变的 `self_built` 或 `third_party_delegated`，模拟器不得切换模式。
- 模拟消息只能注入上游 fixture，禁止直接写最终消息或媒体表。
- fixture 必须显式启用、标记为 `MOCHAT-LOCAL-SIM`、幂等、可 dry-run 清理。
- 自建 fixture 的 AES-GCM 只声明合同等价，不声明与官方闭源 `DecryptData` 字节一致。
- 第三方正文和媒体只经 component 展示，不进入普通明文数据库字段或对象存储。
- 所有凭据、密文、locator、bearer、secret-key 不进入日志、错误文本或审计 JSON。
- 每项行为变更遵循 RED → GREEN → REFACTOR，生产代码前必须看见对应测试按预期失败。

---

### Task 1: 自建 Finance fixture 真实密码学

**Files:**
- Create: `internal/archivefixture/finance.go`
- Create: `internal/archivefixture/finance_test.go`
- Modify: `internal/testfixtures/archivesource/fixture.go`
- Modify: `internal/testfixtures/archivesource/fixture_test.go`

**Interfaces:**
- Produces: `type FinanceCipher struct`, `NewFinanceCipher(corpID string)`, `Encrypt(seq uint64, msgID string, plaintext []byte) (EncryptedChatData, error)`, `Decrypt(EncryptedChatData) ([]byte, error)`。
- Produces: `EncryptedChatData{PublicKeyVersion uint32, EncryptedRandomKey string, EncryptedMessage string}`。

- [ ] **Step 1: 写失败测试**

```go
func TestFinanceCipherRoundTripAndTamper(t *testing.T) {
    cipher, _ := NewFinanceCipher("ww-local-a")
    envelope, _ := cipher.Encrypt(7, "msg-7", []byte(`{"msgtype":"text"}`))
    plain, err := cipher.Decrypt(envelope)
    if err != nil || string(plain) != `{"msgtype":"text"}` { t.Fatalf("round trip: %q %v", plain, err) }
    envelope.EncryptedMessage = envelope.EncryptedMessage[:len(envelope.EncryptedMessage)-1] + "A"
    if _, err := cipher.Decrypt(envelope); err == nil { t.Fatal("tampered ciphertext accepted") }
}
```

- [ ] **Step 2: 运行 RED**

Run: `go test ./internal/archivefixture ./internal/testfixtures/archivesource -run 'TestFinanceCipher|TestArchiveFixture' -count=1`

Expected: FAIL，因为 `internal/archivefixture` 和 `NewFinanceCipher` 尚不存在。

- [ ] **Step 3: 最小实现**

实现每消息独立 32-byte key、12-byte nonce、RSA PKCS#1 v1.5 包装，AES-GCM AAD 固定为 `corpID\x00seq\x00msgID\x00publicKeyVersion`；错误只返回稳定文本 `finance fixture authentication failed`。

- [ ] **Step 4: 把现有 fixture 的查表解密替换为 FinanceCipher**

`GetChatData` 返回真实 base64 密文；`DecryptData` 必须从 RSA 解包后的 random key执行 AES-GCM 解密，不再按固定字符串查找明文。

- [ ] **Step 5: 运行 GREEN 与回归**

Run: `go test ./internal/archivefixture ./internal/testfixtures/archivesource ./internal/wecomarchivedemo -count=1`

Expected: PASS。

- [ ] **Step 6: 提交**

```text
git add internal/archivefixture/finance.go internal/archivefixture/finance_test.go internal/testfixtures/archivesource/fixture.go internal/testfixtures/archivesource/fixture_test.go
git commit -m "feat: encrypt archive finance fixtures"
```

### Task 2: 第三方 suite 授权 fixture

**Files:**
- Create: `internal/archivefixture/suite.go`
- Create: `internal/archivefixture/suite_test.go`
- Modify: `internal/wecomcredentials/manager.go`
- Modify: `internal/wecomcredentials/manager_test.go`

**Interfaces:**
- Produces: `SuiteProvider` 的 `PushTicket`、`ExchangeSuiteToken`、`CreatePreAuthCode`、`ExchangePermanentCode`、`CorpToken`、`RevokeAuthorization`。
- Produces: `AuthorizationCredential` 新字段 `SuiteID`、`SuiteSecret`、`SuiteTicket`、`AuthCorpID`，与 self-built secret 字段互斥。

- [ ] **Step 1: 写 suite token 与租户隔离失败测试**

测试最新 ticket 覆盖旧 ticket、token 两小时过期、auth code 只能消费一次、permanent code 按 corp 隔离、撤销后无法获取 corp token。

- [ ] **Step 2: 运行 RED**

Run: `go test ./internal/archivefixture ./internal/wecomcredentials -run 'TestSuite|TestAuthorizationCredential' -count=1`

Expected: FAIL，缺少 SuiteProvider 和新凭据字段。

- [ ] **Step 3: 实现确定性状态机**

状态只保存在 fixture store；token 返回不可预测随机值但测试通过注入时钟和随机源保持确定。错误映射为 `SUITE_TICKET_INVALID`、`SUITE_TOKEN_EXPIRED`、`AUTH_CODE_CONSUMED`、`AUTHORIZATION_REVOKED`、`CORP_SCOPE_MISMATCH`。

- [ ] **Step 4: 扩展凭据加密合同**

沿用 `EncryptAuthorization(tenantID, integrationID, value)` 的 AES-GCM AAD；测试证明跨 tenant/integration 解密失败，JSON 和错误不包含 suite secret、ticket、永久授权码。

- [ ] **Step 5: 运行 GREEN**

Run: `go test ./internal/archivefixture ./internal/wecomcredentials -count=1`

Expected: PASS。

- [ ] **Step 6: 提交**

```text
git add internal/archivefixture/suite.go internal/archivefixture/suite_test.go internal/wecomcredentials/manager.go internal/wecomcredentials/manager_test.go
git commit -m "feat: simulate delegated suite authorization"
```

### Task 3: 数据专区消息和展示 locator

**Files:**
- Create: `internal/archivefixture/datazone.go`
- Create: `internal/archivefixture/datazone_test.go`
- Modify: `internal/modules/providers/archive/source.go`
- Modify: `internal/modules/providers/archive/source_test.go`

**Interfaces:**
- Produces: `ContentPolicy` 常量 `plaintext`、`component`。
- Produces: `ComponentDescriptor{MessageID, PublicKeyVersion, EncryptedSecretKey}`，挂到 `archive.Message.Component`。
- Produces: `DataZoneProvider.Append`、`Fetch`、`Render`。

- [ ] **Step 1: 写失败测试**

测试文本、图片、语音、视频、文件的 metadata 拉取；RSA 解开 secret-key 后才能 render；错误密钥、跨 corp、过期和撤销均失败；`archive.Message` 的普通内容字段不包含正文。

- [ ] **Step 2: 运行 RED**

Run: `go test ./internal/archivefixture ./internal/modules/providers/archive -run 'TestDataZone|TestComponent' -count=1`

Expected: FAIL，缺少 DataZoneProvider、ContentPolicy 和 ComponentDescriptor。

- [ ] **Step 3: 实现数据专区 provider**

每条消息生成 32-byte secret-key，以租户应用 RSA 公钥加密；fixture store 保存确定性内容，Fetch 只返回 metadata 和 encrypted secret-key，Render 要求正确 `msgid + secret-key`。

- [ ] **Step 4: 扩展归档消息模型**

保证 `ContentRaw`/`RawJSON` 只包含可出区 metadata；`ComponentDescriptor` 不参与 Finance 媒体任务派生。

- [ ] **Step 5: 运行 GREEN**

Run: `go test ./internal/archivefixture ./internal/modules/providers/archive -count=1`

Expected: PASS。

- [ ] **Step 6: 提交**

```text
git add internal/archivefixture/datazone.go internal/archivefixture/datazone_test.go internal/modules/providers/archive/source.go internal/modules/providers/archive/source_test.go
git commit -m "feat: model data zone archive content"
```

### Task 4: 双模式 bridge HTTP 合同

**Files:**
- Create: `internal/archivebridge/handler.go`
- Create: `internal/archivebridge/handler_test.go`
- Create: `internal/archivebridge/store.go`
- Create: `internal/archivebridge/store_test.go`
- Create: `cmd/mochat-archive-bridge/main.go`
- Create: `cmd/mochat-archive-bridge/main_test.go`
- Modify: `internal/modules/providers/archive/bridge_source.go`
- Modify: `internal/modules/providers/archive/bridge_source_test.go`

**Interfaces:**
- Produces: `POST /v1/archive/messages`、`POST /v1/archive/media/chunks`、`POST /v1/archive/component/session`、`GET /healthz`。
- Produces: `NewBridgeSource(client, scope, wxCorpID, mode)`，source ID 为 `wecom:<mode>:<wxCorpID>`。

- [ ] **Step 1: 写失败合同测试**

覆盖 Bearer、tenant/corp/mode 二次绑定、自建消息和媒体、第三方 metadata、component session、未知字段拒绝、body 上限和脱敏错误。

- [ ] **Step 2: 运行 RED**

Run: `go test ./internal/archivebridge ./internal/modules/providers/archive ./cmd/mochat-archive-bridge -count=1`

Expected: FAIL，缺少 bridge 包、命令和显式 mode 参数。

- [ ] **Step 3: 实现 handler/store/命令**

fixture store 使用 `/app/storage/archive-fixture` 的原子 JSON snapshot；私钥和 secret 单独保存在权限 `0600` 的 fixture 配置，不输出到 status。

- [ ] **Step 4: 修改 app client 和 parser**

请求显式携带 `integration_mode`；Finance 继续解析 content/media，Data Zone 只解析 metadata/component。旧 `/work-message/archive/*` 在一个兼容期内代理到 v1，但不允许缺省模式。

- [ ] **Step 5: 运行 GREEN**

Run: `go test ./internal/archivebridge ./internal/modules/providers/archive ./cmd/mochat-archive-bridge -count=1`

Expected: PASS。

- [ ] **Step 6: 提交**

```text
git add internal/archivebridge cmd/mochat-archive-bridge internal/modules/providers/archive/bridge_source.go internal/modules/providers/archive/bridge_source_test.go
git commit -m "feat: add dual-mode archive bridge"
```

### Task 5: 持久化 component locator 与 durable 模式选择

**Files:**
- Create: `deploy/standalone/migrations/0169_archive_component_locator.up.sql`
- Create: `deploy/standalone/migrations/0169_archive_component_locator.down.sql`
- Create: `internal/migration/archive_component_locator_test.go`
- Create: `internal/store/archive_component.go`
- Create: `internal/store/archive_component_test.go`
- Modify: `internal/store/archive_sync.go`
- Modify: `internal/store/archive_sync_test.go`
- Modify: `internal/store/durable_archive_bridge.go`
- Modify: `internal/store/durable_archive_bridge_test.go`

**Interfaces:**
- Produces: `mochat_go_archive_component_locators`，唯一键 `(tenant_id,corp_id,msgid)`；locator 使用 wecom credential manager 加密。
- Produces: durable binding 携带 `IntegrationMode`，并拒绝数据库模式与 bridge 消息模式不一致。

- [ ] **Step 1: 写迁移与 store 失败测试**

测试 tenant/corp/msgid 唯一性、密文非明文、跨租户解密失败、删除消息级联、审计和普通查询不回显 locator。

- [ ] **Step 2: 运行 RED**

Run: `go test ./internal/migration ./internal/store -run 'TestArchiveComponent|TestDurableArchive' -count=1`

Expected: FAIL，迁移和 store 方法不存在。

- [ ] **Step 3: 实现迁移和事务写入**

`UpsertArchiveMessage` 在同一事务内保存消息 source identity 与加密 locator；任一步失败都不推进 cursor。

- [ ] **Step 4: 让 durable binding 读取不可变模式**

SQL 从 `mochat_go_tenant_corp_bindings.wecom_integration_mode` 读取模式；未配置 bridge 时 run 记录业务错误但页面查询返回空数据。

- [ ] **Step 5: 运行 GREEN 与 MariaDB 门禁**

Run: `go test ./internal/migration ./internal/store -count=1`

Run when DSN exists: `go test ./internal/store -run Integration -count=1`

Expected: 单元测试 PASS；无 DSN 明确记录 SKIP。

- [ ] **Step 6: 提交**

```text
git add deploy/standalone/migrations/0169_archive_component_locator.* internal/migration/archive_component_locator_test.go internal/store/archive_component.go internal/store/archive_component_test.go internal/store/archive_sync.go internal/store/archive_sync_test.go internal/store/durable_archive_bridge.go internal/store/durable_archive_bridge_test.go
git commit -m "feat: persist archive component locators"
```

### Task 6: Dashboard 鉴权展示组件

**Files:**
- Create: `internal/dashboard/archive_component.go`
- Create: `internal/dashboard/archive_component_test.go`
- Create: `internal/server/archive_component_route_test.go`
- Modify: `internal/server/server.go`
- Modify: `internal/dashboard/dashboard_access_guard.go`
- Modify: `internal/dashboard/dashboard_access_guard_test.go`
- Modify: `internal/store/work_message.go`
- Modify: `internal/store/work_message_media_test.go`
- Modify: `web/apps/dashboard/src/features/conversation-global/work-message-api.ts`
- Modify: `web/apps/dashboard/src/features/conversation-global/work-message-api.test.ts`
- Modify: `web/apps/dashboard/src/features/conversation-global/work-message-detail.tsx`
- Modify: `web/apps/dashboard/src/features/conversation-global/work-message-detail.test.tsx`

**Interfaces:**
- Produces: `POST /dashboard/archive/components/{id}/session`，返回一次性同源 URL；`GET /dashboard/archive/components/session/{token}` 返回 fixture 组件 HTML/媒体流。
- Produces: 消息内容投影 `contentPolicy`、`component.available`、`component.sessionUrl`，不返回 locator 或 secret-key。

- [ ] **Step 1: 写后端和前端失败测试**

覆盖未登录 401、无页面权限 403、跨租户 404、过期/重放 token 404、同租户成功；前端点击“安全查看”后打开组件，空态和错误使用业务语言。

- [ ] **Step 2: 运行 RED**

Run: `go test ./internal/dashboard ./internal/server ./internal/store -run 'TestArchiveComponent|TestWorkMessage' -count=1`

Run: `pnpm --filter @mochat/dashboard test -- --run work-message`

Expected: FAIL，路由、投影和组件交互不存在。

- [ ] **Step 3: 实现短期 session**

token 绑定 user/auth_version/tenant/corp/msgid，60 秒过期、一次性消费；服务端向 bridge 换 component session，不把 secret-key 交给浏览器普通 JSON。

- [ ] **Step 4: 实现 Dashboard 交互**

`plaintext` 沿用现有内容和媒体；`component` 显示“安全查看会话内容”，在同源 iframe/dialog 中打开，失败可重试且不污染全页。

- [ ] **Step 5: 运行 GREEN**

Run: `go test ./internal/dashboard ./internal/server ./internal/store -count=1`

Run: `pnpm --filter @mochat/dashboard test -- --run`

Expected: PASS。

- [ ] **Step 6: 提交**

```text
git add internal/dashboard/archive_component.go internal/dashboard/archive_component_test.go internal/server/archive_component_route_test.go internal/server/server.go internal/dashboard/dashboard_access_guard.go internal/dashboard/dashboard_access_guard_test.go internal/store/work_message.go internal/store/work_message_media_test.go web/apps/dashboard/src/features/conversation-global
git commit -m "feat: render delegated archive content securely"
```

### Task 7: `seed/send/status/cleanup` CLI

**Files:**
- Create: `internal/archivefixture/client.go`
- Create: `internal/archivefixture/client_test.go`
- Modify: `cmd/mochat-archive-simulator/main.go`
- Modify: `cmd/mochat-archive-simulator/main_test.go`
- Modify: `internal/archivesim/simulator.go`
- Modify: `internal/archivesim/simulator_test.go`

**Interfaces:**
- Produces: `seed --tenant-id --mode --scenario --dataset`。
- Produces: `send --tenant-id --mode --type --text|--file --dataset`。
- Produces: `status --tenant-id --dataset`。
- Produces: `cleanup --tenant-id --dataset [--confirm-dataset]`，默认 dry-run。

- [ ] **Step 1: 写 CLI 失败测试**

覆盖未启用模拟、模式不符、文本/媒体参数互斥、文件越界/过大、幂等 seed、status 脱敏、cleanup 双重标记和非 fixture 数据保留。

- [ ] **Step 2: 运行 RED**

Run: `go test ./cmd/mochat-archive-simulator ./internal/archivefixture ./internal/archivesim -count=1`

Expected: FAIL，四个目标子命令和 fixture client 不存在。

- [ ] **Step 3: 实现 CLI client**

DSN 仅用于解析租户权威绑定与 cleanup 正式账本；消息创建只调用 bridge fixture admin API。输出 JSON 只包含 dataset、mode、cursor、count、status、stableErrorCode。

- [ ] **Step 4: 实现安全 cleanup**

先列出数据集注册表拥有的 msgid/object/run/file，验证全部带 `MOCHAT-LOCAL-SIM` 和安全根路径，再在事务中删除；未传精确 confirm 时只输出 dry-run。

- [ ] **Step 5: 运行 GREEN**

Run: `go test ./cmd/mochat-archive-simulator ./internal/archivefixture ./internal/archivesim -count=1`

Expected: PASS。

- [ ] **Step 6: 提交**

```text
git add internal/archivefixture/client.go internal/archivefixture/client_test.go cmd/mochat-archive-simulator/main.go cmd/mochat-archive-simulator/main_test.go internal/archivesim/simulator.go internal/archivesim/simulator_test.go
git commit -m "feat: add archive simulator commands"
```

### Task 8: 专项集成与文档

**Files:**
- Modify: `cmd/mochat-archive-acceptance/main.go`
- Modify: `cmd/mochat-archive-acceptance/main_test.go`
- Create: `docs/guides/archive-simulator.zh-CN.md`
- Create: `docs/verification/2026-08-27-dual-mode-archive-simulator-report.zh-CN.md`

**Interfaces:**
- Produces: acceptance 子命令 `verify-dual-mode`，验证两个测试租户通过正式上游/bridge/worker/Dashboard 路径。

- [ ] **Step 1: 写失败验收测试**

测试报告必须包含两种模式、游标、重放、失败不推进、媒体/组件、重启和无泄密证据；任何模式缺失即失败。

- [ ] **Step 2: 运行 RED**

Run: `go test ./cmd/mochat-archive-acceptance -run TestDualMode -count=1`

Expected: FAIL，`verify-dual-mode` 不存在。

- [ ] **Step 3: 实现验收与中文操作指南**

指南给出启动、创建匹配模式测试租户、seed、send 文本/图片/语音/文件、手动同步、status、Dashboard 查看和 cleanup 的可复制 PowerShell 命令；所有 secret 从受保护 env/file 读取。

- [ ] **Step 4: 运行专项门禁**

Run: `go test ./cmd/mochat-archive-simulator ./cmd/mochat-archive-acceptance ./internal/archivefixture ./internal/archivebridge ./internal/modules/providers/archive ./internal/store ./internal/dashboard ./internal/server -count=1`

Expected: PASS。

- [ ] **Step 5: 提交**

```text
git add cmd/mochat-archive-acceptance docs/guides/archive-simulator.zh-CN.md docs/verification/2026-08-27-dual-mode-archive-simulator-report.zh-CN.md
git commit -m "test: verify dual-mode archive simulator"
```
