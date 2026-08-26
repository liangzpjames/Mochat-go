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
