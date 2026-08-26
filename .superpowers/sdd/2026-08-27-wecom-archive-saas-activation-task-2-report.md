# 任务 2 实施报告：SaaS 企微集成后端与失败关闭切换

## 结论

已实现租户级 current/candidate 企微集成后端、contract-only 验证、失败关闭的切换/回滚、安全审计及运行时路由组合。0166 migration 中错误的 `platform.wecom_integrations.read/manage` seed 与回滚删除语句已移除，API 权限统一复用仓库既有的 `platform.integrations.read/manage`。

## RED 证据

1. 新增业务/HTTP 合同测试后执行：
   `go test ./internal/dashboardadmin ./internal/store ./internal/dashboard -run 'WeComIntegration|SaaSAdminAccess' -count=1`
   首次失败于 `WeComIntegrationView`、`WeComVerificationCandidate`、`WeComVerificationResult` 等类型不存在，证明新后端合同尚未实现。
2. 新增 store generation 失败关闭测试后执行：
   `go test ./internal/store -run WeComIntegration -count=1`
   首次失败于 `nextWeComIntegrationGeneration` 不存在。
3. 新增切换门禁测试后执行：
   `go test ./internal/store -run WeComIntegrationSwap -count=1`
   首次失败于 `validateWeComIntegrationSwap` 不存在。

## GREEN 证据

最终新鲜验证结果：

- `go test ./internal/dashboardadmin ./internal/store ./internal/dashboard -run 'WeComIntegration|SaaSAdminAccess' -count=1`：3 个包通过。
- `go test ./cmd/mochat-go -run 'SaaS|Approval|Composition' -count=1`：通过。
- `go test ./internal/migration -run 0166 -count=1`：通过。
- 附加验证 `go test ./internal/server -run 'SaaSAdmin.*Route|RouteList' -count=1`：通过。
- `git diff --check`：通过，仅显示 Windows 工作树的 LF/CRLF 提示，无 whitespace error。
- 生产代码与 migration 扫描未发现 `platform.wecom_integrations.read/manage`；这两个文本只保留在 migration 合同测试的禁止断言中。

## 变更文件

- 新增 `internal/dashboardadmin/wecom_integration.go`：公开 DTO、候选输入合同、可注入 verifier、默认失败关闭、验证/切换/回滚服务。
- 新增 `internal/dashboardadmin/wecom_integration_test.go`：模式互斥、首次/更新凭据合同、local contract、corp mismatch、默认 verifier、path tenant 与响应脱敏。
- 新增 `internal/store/saas_wecom_integration.go`：权限锁、authoritative binding 锁、候选密文保存/保留、验证落库、切换/回滚事务、媒体 lease 门禁、安全审计。
- 新增 `internal/store/saas_wecom_integration_test.go`：generation 单调性、凭据 hint 脱敏及所有切换失败关闭门禁。
- 修改 `internal/dashboardadmin/http.go`：六个租户级 API 与稳定错误码映射。
- 修改 `internal/dashboard/saas_admin_access.go` 及测试：GET/audits 使用 `platform.integrations.read`，其余使用 `platform.integrations.manage`。
- 修改 `cmd/mochat-go/main.go`：MySQL store 与默认 nil verifier 组合；未配置在线 verifier 时稳定返回 `WECOM_ONLINE_VERIFICATION_UNAVAILABLE`。
- 修改 `internal/server/server.go`：使六个 API 在运行时路由可达并进入 migrated route 清单。
- 修改 0166 up/down migration 及合同测试：删除错误权限 seed/delete，并增加 `verification_level` 持久字段。

## 自审

- 请求 DTO 不含 `tenantId`、`corpId`、`actor`；严格 JSON 解码会拒绝这些字段。tenant 仅来自 path，corp 仅来自锁定的 authoritative binding，actor 仅来自 SaaS principal。
- self-built 与 delegated 凭据字段严格互斥；已有同模式候选可用全空敏感字段保留原信封，首次配置仍强制要求有效凭据。
- verifier 只通过注入接口调用；生产默认不调用真实企微，且只接受显式 `local_contract` 结果。
- switch/rollback 在同一事务内锁定 actor 权限、active tenant/binding、current/candidate，校验 version、verified corp、missing capabilities、密文可解和 active media lease；失败路径不提交任何 current 变化。
- switch/rollback 统一使用 `max(current,candidate)+1` 生成新 current generation；回滚不会降低 generation。

## 审计脱敏说明

公开 `WeComIntegration` 的密文与 key id 使用 `json:"-"`，敏感输入字段从不进入响应。审计 before/after 仅序列化已清除 `CredentialCiphertext`/`CredentialKeyID` 的公开 DTO；不写入 secret、permanent code、ciphertext 或 verifier 原始错误。错误响应仅返回稳定机器码，不回显底层解密或验证错误。

## Concerns

- brief 指定测试及额外 server 路由测试均已通过；本轮未连接真实 MySQL 或真实企业微信。真实企微调用被设计为显式不启用，MySQL 事务语义由单元/合同测试和现有 schema 约束覆盖。

## 独立审查修复（2026-08-27）

### RED 证据

- 增加 candidate version 流程、store verification-level 门禁和权威权限常量测试后，执行 `go test ./internal/dashboardadmin ./internal/store ./internal/dashboard -run 'WeComIntegration|SaaSAdminAccess' -count=1`：
  - store 测试因 `weComIntegrationVerificationStatus` 尚不存在而构建失败；
  - dashboard 测试发现 `dashboardadmin` 仍重复声明 `PermissionIntegrationsRead/Manage`；
  - candidate version 流程用例明确建模 current v1、save candidate v1、verify candidate v2、switch request v2，以及陈旧 request v1 必须冲突。

### 修复说明

- switch 与 rollback 的 optimistic locking 改为比较即将提升的 `candidate.Version`，不再错误比较 `current.Version`。
- 新增 sqlmock 真实 store 流程测试，依次执行 `SaveWeComIntegrationCandidate`、`VerifyCandidate`、`Switch`，证明 candidate v1 验证后成为 v2，switch request v2 可成功提升且新 current 为 v3；独立 guard 用例证明陈旧 v1 被 `ErrVersionConflict` 拒绝。
- `CompleteWeComIntegrationVerification` 在 store 层强制成功结果的 `verificationLevel` 必须精确为 `local_contract`；空值、带额外空白、`online` 和未知值不能写成 active。失败结果仍只能写成 failed。
- switch/rollback 提升门禁同时强制候选记录已持久化 `verificationLevel=local_contract`；空值、online 和未知值统一按未验证拒绝。
- 删除 `dashboardadmin` 的重复权限字符串；store 直接引用 `dashboard.SaaSAdminPermissionIntegrationsRead/Manage` 权威常量，并加入防漂移源码合同测试。

### GREEN 证据

- `go test ./internal/dashboardadmin ./internal/store ./internal/dashboard -run 'WeComIntegration|SaaSAdminAccess' -count=1`：3 个包通过。
- `go test ./cmd/mochat-go -run 'SaaS|Approval|Composition' -count=1`：通过。
- `go test ./internal/migration -run 0166 -count=1`：通过。
- `git diff --check`：通过，仅有 Windows LF/CRLF 提示，无 whitespace error。

### 审查修复后的 concerns

- 无已知 Important/Minor 遗留；真实企业微信仍按任务约束不调用。完整 store 流程使用 sqlmock 覆盖 SQL 事务交互，未连接外部 MariaDB。

## Slot version ABA 复审修复（2026-08-27）

### RED 证据

- 扩展 `TestWeComIntegrationCandidateVersionFlowSaveV1VerifyV2SwitchV2`，要求 `(current A v1, candidate B v2)` 执行 `switch(v2)` 后 promoted B 与 demoted A 的 version 都为 v3；原实现返回 demoted A v2，测试在首次 switch 结果断言处失败，直接复现已消费 request v2 可再次命中 A 的 ABA 条件。

### 修复说明

- slot 转换事务在删除/重插两条锁定记录前计算 `nextVersion=max(current.Version,candidate.Version)+1`，promoted 与 demoted 记录统一写入该 version；无符号溢出时返回 `ErrVersionConflict` 并回滚。
- 完整 store 流程继续证明 save candidate v1 → verify candidate v2 → switch(v2) 成功。
- 同一流程新增 switch(v2) 重放：锁定新 current B v3 与 rollback target A v3 后返回 `ErrVersionConflict`，只发生 rollback，不执行 DELETE/INSERT，既有 current B v3 快照不变。
- 新增 rollback target version 覆盖：`rollback(v3)` 精确锁定 demoted A v3，成功后 A/B 两条记录都推进到 v4。

### GREEN 证据

- `go test ./internal/dashboardadmin ./internal/store ./internal/dashboard -run 'WeComIntegration|SaaSAdminAccess' -count=1`：3 个包通过。
- `go test ./cmd/mochat-go -run 'SaaS|Approval|Composition' -count=1`：通过。
- `go test ./internal/migration -run 0166 -count=1`：通过。
- `git diff --check`：通过，仅有 Windows LF/CRLF 提示，无 whitespace error。

### ABA 修复后的 concerns

- 无已知遗留；version 与 generation 现在均在每次 slot 转换时全局单调递增，重复请求不能重新命中另一 slot。
