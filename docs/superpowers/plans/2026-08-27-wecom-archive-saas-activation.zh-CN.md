# 企微会话存档媒体接入、SaaS 租户对接模式与新用户激活引导实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**目标：** 在不调用真实企业微信的前提下，交付可验证的 Finance SDK 消息/媒体生产链路、租户级企微对接模式和新租户管理员激活引导。

**架构：** bridge 负责官方 SDK 边界，主程序用 0138 durable source 同步正文并原子派生媒体任务，独立 worker 可续传媒体到本地对象存储；SaaS 通过 current/candidate 集成域选择自建或第三方代开发模式；激活入口使用 URL fragment 和 POST 预检，状态从现有数据库事实派生。

**技术栈：** Go 1.26、MySQL 5.7/MariaDB、Redis、企业微信 Finance SDK C bridge、React 19、TypeScript、Vitest、Ant Design、Tailwind/Radix、Docker Compose、Playwright/应用内浏览器。

## 全局约束

- 基准固定为 `main@49e96e9afee01668844061e380f9ed07a2088489`，只在 `feat/wecom-archive-saas-activation` 隔离 worktree 工作。
- 不调用真实企业微信，不推送远端，不部署公网；结论必须区分“本地 SDK 合同通过”和“真实企微线上通过”。
- 不修改或提交主工作树的 `docs/PROJECT_PROGRESS.zh-CN.md`、`.workbuddy/`、`tmp/`、调试脚本、旧构建产物或其他未提交内容。
- 业务消息必须通过 `BridgeSource → SyncService → MySQLStore → media worker` 生成；禁止直接插最终消息表伪造接入成功。
- Secret、永久授权码、RSA 私钥、激活 token、MFA secret 不得进入日志、审计、URL query、localStorage、截图或 Git。
- 两种企微模式凭据隔离，任何失败不得自动回退到另一模式。
- 媒体失败不得回滚已安全保存的消息正文或推进错误游标；所有 lease 更新必须带 attempt/token fence。
- 测试数据统一标记 `MOCHAT-LOCAL-ACCEPTANCE-20260827`，seed 幂等且有精确 cleanup。
- 用户审阅设计、计划和验收报告均使用中文。

---

## 文件结构与职责

- `deploy/standalone/migrations/0166_wecom_integration_and_archive_media.*.sql`：集成 current/candidate、媒体对象、权限和回填。
- `internal/wecomcredentials/manager.go`：第三方授权信封的加密/解密。
- `internal/store/saas_wecom_integration.go`：租户集成读写、验证、切换、回滚和审计。
- `internal/dashboardadmin/wecom_integration.go`：SaaS 企微集成 DTO、校验和服务方法。
- `internal/dashboardadmin/http.go`：SaaS 集成 API。
- `web/apps/saas-admin/src/pages/TenantsPage.tsx`：模式、候选、验证、切换保护与激活交付状态。
- `internal/dashboardauth/http.go`、`internal/store/dashboard_identity.go`：激活预检。
- `web/apps/dashboard/src/features/auth/activation-page.tsx`：fragment 清理和激活状态 UI。
- `internal/wecomarchivedemo/sdk*.go`、`archive.go`、`server.go`：GetMediaData 生产边界和 bridge 媒体端点。
- `internal/testfixtures/archivesource/`：去敏 SDK JSON 和确定性媒体 Provider stub。
- `internal/modules/providers/archive/bridge_source.go`：bridge-backed external source。
- `internal/store/archive_media.go`、`internal/modules/providers/archive/media_sync.go`：媒体任务账本与 worker。
- `internal/dashboard/archive_media_http.go`：租户/企业鉴权媒体读取。
- `web/apps/dashboard/src/features/conversation-global/conversation-message-content.tsx`：媒体状态与预览/播放/下载。
- `cmd/mochat-archive-acceptance/`、`scripts/run_wecom_archive_saas_activation_acceptance.ps1`：幂等 seed/verify/cleanup 与本地验收。
- `deploy/local-acceptance/`：独立 compose 项目和端口配置。
- `docs/verification/2026-08-27-wecom-archive-saas-activation-report.zh-CN.md`：最终中文证据报告。

### 任务 1：迁移与凭据加密合同

**文件：**

- 新建：`deploy/standalone/migrations/0166_wecom_integration_and_archive_media.up.sql`
- 新建：`deploy/standalone/migrations/0166_wecom_integration_and_archive_media.down.sql`
- 修改：`internal/migration/migration_test.go`
- 修改：`internal/migration/migration_integration_test.go`
- 修改：`internal/wecomcredentials/manager.go`
- 修改：`internal/wecomcredentials/manager_test.go`

**接口：**

- 产出：`AuthorizationCredential`、`EncryptAuthorization(tenantID, integrationID, value)`、`DecryptAuthorization(...)`。
- 产出：`mochat_go_wecom_integrations` 与 `mochat_go_archive_media_objects`。

- [ ] **步骤 1：写失败测试**

在 credential 测试中创建 tenant 7/integration `integration-a` 的第三方授权，断言 round-trip、错误 tenant/ID/key 均失败且 ciphertext 不含明文；在 migration 测试中断言 0166 up/down 被发现，up 含 mode/slot/status/generation/version 唯一约束和媒体 lease/fence 字段，down 不删除已有表。

```go
func TestAuthorizationCredentialIsTenantAndIntegrationBound(t *testing.T) {
	manager := mustCredentialManager(t)
	value := AuthorizationCredential{Mode: "third_party_delegated", PermanentCode: "local-secret", ProviderAppID: "provider-a"}
	ciphertext, keyID, err := manager.EncryptAuthorization(7, "integration-a", value)
	if err != nil || strings.Contains(ciphertext, value.PermanentCode) { t.Fatalf("ciphertext=%q err=%v", ciphertext, err) }
	got, err := manager.DecryptAuthorization(7, "integration-a", keyID, ciphertext)
	if err != nil || got != value { t.Fatalf("got=%+v err=%v", got, err) }
	if _, err := manager.DecryptAuthorization(8, "integration-a", keyID, ciphertext); err == nil { t.Fatal("cross-tenant decrypt succeeded") }
}
```

- [ ] **步骤 2：运行 RED**

运行：`go test ./internal/wecomcredentials ./internal/migration -run 'AuthorizationCredential|0166' -count=1`

预期：缺少类型、方法和迁移而失败。

- [ ] **步骤 3：最小实现**

新增授权 DTO 并复用 manager 私有 `encrypt/decrypt`，resource 固定为 `integration`；迁移创建 current/candidate 唯一键、状态 CHECK 的 MySQL 兼容等价约束、tenant/corp FK、媒体唯一身份、lease 索引和 Dashboard/SaaS 权限种子。既有 verified corp 回填 self-built active，`fake_tenant_%` 与 pending binding 回填 unconfigured。

- [ ] **步骤 4：运行 GREEN 与迁移回归**

运行：

```powershell
go test ./internal/wecomcredentials ./internal/migration -count=1
go test ./internal/store -run 'Migration|Schema' -count=1
```

预期：通过；无 DSN 的 integration 测试明确输出 SKIP。

- [ ] **步骤 5：提交**

`git commit -m "feat: add WeCom integration and archive media schema"`

### 任务 2：SaaS 企微集成后端与失败关闭切换

**文件：**

- 新建：`internal/dashboardadmin/wecom_integration.go`
- 新建：`internal/dashboardadmin/wecom_integration_test.go`
- 新建：`internal/store/saas_wecom_integration.go`
- 新建：`internal/store/saas_wecom_integration_test.go`
- 修改：`internal/dashboardadmin/http.go`
- 修改：`internal/dashboardadmin/http_test.go`
- 修改：`cmd/mochat-go/main.go`

**接口：**

- `GetWeComIntegration(ctx, actor, tenantID)`。
- `SaveWeComIntegrationCandidate(ctx, actor, tenantID, input)`。
- `VerifyWeComIntegrationCandidate(...)` 返回 `contract_verified` 或失败原因。
- `SwitchWeComIntegration(...)`、`RollbackWeComIntegration(...)` 使用 version/generation。

- [ ] **步骤 1：写 service/store/HTTP 失败测试**

覆盖 self-built 与 third-party DTO 校验、只读响应不含 secret/permanent code、cross-tenant、候选验证失败不影响 current、corp mismatch、缺 scope、在途 archive lease、version 冲突、成功切换 generation+1 和审计、rollback 生成新 generation。

```go
func TestSwitchCandidateFailsClosedWhenArchiveLeaseIsActive(t *testing.T) {
	store := newIntegrationStoreFixture(t)
	current, candidate := store.seedCurrentAndVerifiedCandidate(t)
	store.seedActiveArchiveLease(t, current.TenantID, current.CorpID)
	_, err := store.SwitchWeComIntegration(context.Background(), SwitchInput{TenantID: current.TenantID, CandidateVersion: candidate.Version})
	if ErrorCode(err) != "WECOM_INTEGRATION_ARCHIVE_BUSY" { t.Fatalf("err=%v", err) }
	assertCurrentIntegration(t, store, current.ID, current.Generation)
}
```

- [ ] **步骤 2：运行 RED**

运行：`go test ./internal/dashboardadmin ./internal/store -run 'WeComIntegration' -count=1`

预期：新接口不存在而失败。

- [ ] **步骤 3：实现 DTO、Store 事务、权限与路由**

请求只接受 mode、providerAppID、agentID、scope、凭据字段和 version；tenant/actor 从 path/principal 注入。自建与第三方验证使用可注入 verifier；默认 verifier 在未配置真实 provider 时返回 `WECOM_ONLINE_VERIFICATION_UNAVAILABLE`，本地 fixture verifier 明确标记 contract-only。切换事务锁定 binding/current/candidate 和活跃媒体 lease，禁止隐式 fallback。

- [ ] **步骤 4：运行 GREEN 和相关权限测试**

运行：

```powershell
go test ./internal/dashboardadmin ./internal/store -run 'WeComIntegration|SaaSAdminAccess' -count=1
go test ./cmd/mochat-go -run 'SaaS|Approval|Composition' -count=1
```

- [ ] **步骤 5：提交**

`git commit -m "feat: add tenant-scoped WeCom integration switching"`

### 任务 3：新租户激活预检与安全入口

**文件：**

- 修改：`internal/dashboardauth/http.go`
- 修改：`internal/dashboardauth/http_test.go`
- 修改：`internal/store/dashboard_identity.go`
- 修改：`internal/store/dashboard_identity_test.go`
- 修改：`internal/dashboardadmin/service.go`
- 修改：`internal/store/dashboard_admin_provisioning.go`
- 修改：`web/apps/dashboard/src/features/auth/auth-api.ts`
- 修改：`web/apps/dashboard/src/features/auth/activation-page.tsx`
- 修改：`web/apps/dashboard/src/features/auth/activation-page.test.tsx`

**接口：**

- `POST /dashboard/auth/activation/status`，body `{activationToken}`。
- `ActivationStatus = valid|expired|activated|revoked|invalid`。
- SaaS 开户/重发响应补充 `activationPath=/activate#token=...` 与 expiresAt，但不声明发送能力。

- [ ] **步骤 1：写后端和前端失败测试**

后端覆盖五种状态和统一安全响应；前端覆盖 fragment/旧 query 读取后立即清理、预检加载、过期/已用 CTA、失败重试、成功后 token 从状态清除。

```tsx
it('从 fragment 读取令牌并在预检前清理地址栏', async () => {
  window.history.replaceState({}, '', '/activate#token=local-token');
  render(<RoutedActivationPage activate={activate} inspect={inspect} />);
  await waitFor(() => expect(inspect).toHaveBeenCalledWith('local-token'));
  expect(window.location.href).not.toContain('local-token');
});
```

- [ ] **步骤 2：运行 RED**

运行：

```powershell
go test ./internal/dashboardauth ./internal/store -run 'ActivationStatus|DashboardActivation' -count=1
corepack pnpm --filter @mochat/dashboard test -- src/features/auth/activation-page.test.tsx
```

- [ ] **步骤 3：实现状态查询和页面状态机**

Store 按 token digest 查询最近事实并只返回安全上下文；public route contract 精确加入 POST status。页面错误使用 `role=alert`、加载使用 `aria-live`，主 CTA 根据状态为激活、登录或联系管理员。

- [ ] **步骤 4：运行 GREEN**

运行同上并增加：`go test ./internal/dashboardadmin -run 'Provision|ResendActivation' -count=1`。

- [ ] **步骤 5：提交**

`git commit -m "feat: guide tenant admins through secure activation"`

### 任务 4：SaaS 管理端模式与激活交付 UI

**文件：**

- 修改：`web/apps/saas-admin/src/lib/api.ts`
- 修改：`web/apps/saas-admin/src/lib/api.test.ts`
- 修改：`web/apps/saas-admin/src/pages/TenantsPage.tsx`
- 修改：`web/apps/saas-admin/src/pages/TenantsPage.test.tsx`
- 修改：`web/apps/saas-admin/src/index.css`

**接口：**

- 消费任务 2 的集成 API和任务 3 的 activationPath/status。
- 产出当前模式卡、候选编辑器、验证状态、切换/回滚确认和安全复制反馈。

- [ ] **步骤 1：写 UI 失败测试**

覆盖 read-only 权限、两种模式字段互斥、secret 输入提交后清空、candidate 未验证禁用切换、阻塞原因、确认弹窗、版本冲突刷新、审计时间线、完整 fragment 链接复制、弹窗关闭清除 token、无虚假“邮件已发送”。

- [ ] **步骤 2：运行 RED**

运行：`corepack pnpm --filter @mochat/saas-admin test -- src/lib/api.test.ts src/pages/TenantsPage.test.tsx`

- [ ] **步骤 3：实现最小 UI**

复用现有 Tailwind/Radix/Lucide/sonner；模式表单只在当前租户详情展开；第三方 permanent code 使用 password input；状态卡显示“本地合同验证”而非“线上已验证”。390px 下按钮全宽、长 URL break-all。

- [ ] **步骤 4：运行 GREEN、typecheck 和 build**

```powershell
corepack pnpm --filter @mochat/saas-admin test
corepack pnpm --filter @mochat/saas-admin typecheck
corepack pnpm --filter @mochat/saas-admin build
```

- [ ] **步骤 5：提交**

`git commit -m "feat: expose tenant WeCom modes and activation delivery"`

### 任务 5：Finance SDK 媒体边界与去敏 Provider stub

**文件：**

- 修改：`internal/wecomarchivedemo/sdk.go`
- 修改：`internal/wecomarchivedemo/sdk_linux.go`
- 修改：`internal/wecomarchivedemo/sdk_stub.go`
- 修改：`internal/wecomarchivedemo/archive.go`
- 修改：`internal/wecomarchivedemo/archive_test.go`
- 修改：`internal/wecomarchivedemo/server.go`
- 修改：`internal/wecomarchivedemo/server_test.go`
- 修改：`internal/testfixtures/archivesource/fixture.go`
- 修改：`internal/testfixtures/archivesource/fixture_test.go`

**接口：**

- `GetMediaData(ctx, sdkFileID, indexBuf, timeout) (MediaChunk, error)`。
- `POST /work-message/archive/media` 返回 dataBase64/nextIndexBuf/finished。
- fixture provider 提供确定性 7-byte 分片和真实形状消息。

- [ ] **步骤 1：写失败测试**

覆盖文本、图片、语音、视频、文件、link、location、mixed、unknown 的解析；媒体 3 分片完成、错误码、corp/Bearer 绑定、响应/日志不含 secret、private key、random key 或完整 sdkfileid。

- [ ] **步骤 2：运行 RED**

`go test ./internal/wecomarchivedemo ./internal/testfixtures/archivesource -run 'Media|Fixture|Archive' -count=1`

- [ ] **步骤 3：实现 SDK/bridge/stub**

Linux purego 绑定官方 `GetMediaData` 及 media data 生命周期；非 Linux 返回明确 capability unavailable。fixture 只实现生产接口，不向最终表写入。

- [ ] **步骤 4：运行 GREEN 和 bridge 回归**

```powershell
go test ./internal/wecomarchivedemo ./internal/testfixtures/archivesource -count=1
go test ./cmd/wecom-archive-demo -count=1
```

- [ ] **步骤 5：提交**

`git commit -m "feat: add Finance SDK media bridge contract"`

### 任务 6：0138 BridgeSource 与可续传媒体 worker

**文件：**

- 新建：`internal/modules/providers/archive/bridge_source.go`
- 新建：`internal/modules/providers/archive/bridge_source_test.go`
- 新建：`internal/modules/providers/archive/media_sync.go`
- 新建：`internal/modules/providers/archive/media_sync_test.go`
- 新建：`internal/store/archive_media.go`
- 新建：`internal/store/archive_media_test.go`
- 修改：`internal/modules/providers/archive/source.go`
- 修改：`internal/store/archive_sync.go`
- 修改：`internal/store/archive_sync_test.go`
- 修改：`cmd/mochat-go/main.go`

**接口：**

- `BridgeArchiveClient.FetchMessages/FetchMedia`。
- `BridgeSource` 实现 `ArchiveSource`。
- `MediaSyncService.RunOne(ctx)` 与 fenced store contract。

- [ ] **步骤 1：写 source/durable/media RED 测试**

覆盖正式解析、游标单调、失败不推进、重放幂等、message 后 cursor 前失败、source mismatch、媒体 outbox 原子性、分片断点、lease takeover、旧 lease 拒绝、missing/corrupt、完成原子 rename、媒体失败不回滚正文。

- [ ] **步骤 2：运行 RED**

`go test ./internal/modules/providers/archive/... ./internal/store -run 'BridgeSource|ArchiveMedia|DurableBridge' -count=1`

- [ ] **步骤 3：实现最小生产链路**

扩充 `archive.Message` 的可选媒体描述；`UpsertArchiveMessage` 同事务创建媒体对象；worker 使用配置的 storage root，路径只由 UUID 派生。main 在新 feature flag 下组装 BridgeSource/SyncService/media worker，legacy 开关保持关闭且不得与新开关同时启用。

- [ ] **步骤 4：运行 GREEN 与 Provider completion**

```powershell
go test ./internal/modules/providers/archive/... ./internal/store ./cmd/mochat-go -count=1
corepack pnpm check:provider-completion
```

无 `MOCHAT_GO_MYSQL_INTEGRATION_DSN` 时记录 SKIP，不能写 PASS。

- [ ] **步骤 5：提交**

`git commit -m "feat: sync WeCom bridge archives through durable media pipeline"`

### 任务 7：Dashboard 媒体鉴权回读与渲染

**文件：**

- 新建：`internal/dashboard/archive_media_http.go`
- 新建：`internal/dashboard/archive_media_http_test.go`
- 修改：`internal/store/work_message_staff.go`
- 修改：`internal/store/work_message_room.go`
- 修改：`internal/store/auto_tag.go`
- 修改：`internal/server/server.go`
- 修改：`internal/server/server_test.go`
- 修改：`web/apps/dashboard/src/features/conversation-global/conversation-message-content.tsx`
- 修改：`web/apps/dashboard/src/features/conversation-global/conversation-message-content.test.tsx`

**接口：**

- `GET|HEAD /dashboard/archive/media/{id}/content`，支持单段 Range。
- 消息 content 增加 normalized media，不暴露 sdkfileid。

- [ ] **步骤 1：写后端和 renderer RED 测试**

覆盖 ready GET/HEAD/Range、download disposition、nosniff、跨租户/企业 404、路径穿越、pending/failed/missing/corrupt；前端覆盖图片、音频、视频、文件及各错误状态和键盘可达。

- [ ] **步骤 2：运行 RED**

```powershell
go test ./internal/dashboard ./internal/server ./internal/store -run 'ArchiveMedia|WorkMessageMedia' -count=1
corepack pnpm --filter @mochat/dashboard test -- src/features/conversation-global/conversation-message-content.test.tsx
```

- [ ] **步骤 3：实现鉴权 API、批量投影和 renderer**

HTTP handler 只从 `dashboardprincipal` 取 scope；store 批量查询避免 N+1；文件响应使用服务端 MIME allowlist 和清理后的文件名。UI 不将 pending 当作空数据，也不把 missing 显示成 0 字节成功。

- [ ] **步骤 4：运行 GREEN 与 Dashboard 回归**

```powershell
go test ./internal/dashboard ./internal/server ./internal/store -count=1
corepack pnpm --filter @mochat/dashboard test -- src/features/conversation-global
corepack pnpm --filter @mochat/dashboard typecheck
corepack pnpm --filter @mochat/dashboard build
```

- [ ] **步骤 5：提交**

`git commit -m "feat: serve and render authenticated archive media"`

### 任务 8：本地验收夹具、独立 Docker、浏览器与最终门禁

**文件：**

- 新建：`cmd/mochat-archive-acceptance/main.go`
- 新建：`cmd/mochat-archive-acceptance/main_test.go`
- 新建：`scripts/run_wecom_archive_saas_activation_acceptance.ps1`
- 新建：`deploy/local-acceptance/docker-compose.yml`
- 新建：`deploy/local-acceptance/.env.example`
- 修改：`package.json`
- 新建：`scripts/check_wecom_archive_saas_activation.test.mjs`
- 新建：`scripts/check_wecom_archive_saas_activation.mjs`
- 新建：`docs/verification/2026-08-27-wecom-archive-saas-activation-report.zh-CN.md`

**接口：**

- CLI：`seed|verify|cleanup`，固定 dataset ID，可重复执行。
- Compose：项目 `mochat-wecom-acceptance-20260827`，独立端口和命名卷。

- [ ] **步骤 1：写 CLI、静态门禁和 compose RED 测试**

断言 seed 重放同一 run/object，verify 检查文本/图片/语音/视频/文件、cursor/run/audit/source/media 状态，cleanup 只命中具名 fixture；静态门禁扫描 secret DTO、legacy direct-sync composition 和 URL query token。

- [ ] **步骤 2：运行 RED**

```powershell
go test ./cmd/mochat-archive-acceptance -count=1
node --test scripts/check_wecom_archive_saas_activation.test.mjs
```

- [ ] **步骤 3：实现 CLI、compose 与报告模板**

compose 使用独立 project、端口、数据库和对象卷；本地 key 文件在 ignored 临时目录创建，权限最小化；fixture 通过正式 API/worker 路径生成。

- [ ] **步骤 4：执行 Docker 验收**

```powershell
powershell -File scripts/run_wecom_archive_saas_activation_acceptance.ps1 -Action seed
powershell -File scripts/run_wecom_archive_saas_activation_acceptance.ps1 -Action verify
docker compose -p mochat-wecom-acceptance-20260827 -f deploy/local-acceptance/docker-compose.yml restart app
powershell -File scripts/run_wecom_archive_saas_activation_acceptance.ps1 -Action verify
```

保持 app/mysql/redis 最终健康且本地 URL 可访问；不得执行 `down -v`。

- [ ] **步骤 5：应用内浏览器实际点击验收**

覆盖 SaaS current/candidate 两种模式、验证和切换阻塞；激活 valid/expired/activated/revoked/重发；Dashboard 文本/图片/音频/视频/文件、鉴权播放/查看、刷新恢复、空态/失败态。尺寸：2560×1440、1366×768、390×844；检查 console、network 和意外 4xx/5xx。

- [ ] **步骤 6：运行完整门禁**

```powershell
go test ./... -count=1
corepack pnpm --filter @mochat/dashboard lint && corepack pnpm --filter @mochat/dashboard typecheck && corepack pnpm --filter @mochat/dashboard test && corepack pnpm --filter @mochat/dashboard build
corepack pnpm --filter @mochat/saas-admin lint && corepack pnpm --filter @mochat/saas-admin typecheck && corepack pnpm --filter @mochat/saas-admin test && corepack pnpm --filter @mochat/saas-admin build
corepack pnpm --filter @mochat/operation lint && corepack pnpm --filter @mochat/operation typecheck && corepack pnpm --filter @mochat/operation test && corepack pnpm --filter @mochat/operation build
corepack pnpm --filter @mochat/sidebar lint && corepack pnpm --filter @mochat/sidebar typecheck && corepack pnpm --filter @mochat/sidebar test && corepack pnpm --filter @mochat/sidebar build
corepack pnpm check:phase4-dashboard-page-rbac
corepack pnpm check:provider-completion
corepack pnpm check:dashboard-all-pages-evidence
corepack pnpm check:yuanhu-benchmark
go test ./internal/migration -count=1
git diff --check 49e96e9afee01668844061e380f9ed07a2088489...HEAD
```

- [ ] **步骤 7：代码审查与修复**

生成 `49e96e9...HEAD` 全量 diff review package；审查租户隔离、secret 泄露、cursor/lease、文件读取、模式 fallback、激活 token 清理和迁移回滚。修复 Critical/Important 后重跑覆盖测试。

- [ ] **步骤 8：完成中文报告与提交**

报告真实 PASS/FAIL/SKIP、fixture 边界、镜像/容器/端口、URL、非敏感登录说明、回滚/cleanup 和真实企微外部边界。

`git commit -m "test: verify WeCom archive SaaS activation initiative"`

## 计划自审

- 设计中的 SDK、durable、media、integration、activation、fixture、Docker、浏览器和全量门禁均映射到明确任务。
- 文件职责和跨任务接口一致；source/media 与 integration/activation 可独立测试，最终在任务 8 集成。
- 无 `TBD`、`TODO`、“类似任务 N”或无法执行的占位步骤。
- 所有生产行为修改均先有失败测试和 RED 命令，再做最小实现与 GREEN 回归。
- 真实企微线上调用明确排除；MariaDB integration 无 DSN 时必须记录 SKIP。
