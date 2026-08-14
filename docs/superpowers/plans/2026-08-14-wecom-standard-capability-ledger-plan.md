# WeCom 标准能力账本与精准群发实施计划

## 目标

在现有隔离分支完成真实 runtime 证据驱动的 `wecom_standard` capability 状态、0139 operation/audit ledger 和 contact/room 精准群发可恢复闭环；不改变旧迁移，不操作外部环境。

## 实施步骤

### 批次一：能力合同与状态投影

1. 在 `internal/modules/providers` 增加结构化 capability status 字段和稳定 capability 常量，先补 registry snapshot、非法状态和缺失分类 RED。
2. 扩展 `internal/modules/providers/catalog/catalog.go` 与 `internal/dashboard/room_welcome_wecom.go`，登记 12 个标准能力；runtime 不可用时保持 unavailable，runtime 可用但未绑定租户时保持 limited。
3. 扩展 `internal/companyprofile/provider_status.go`、`internal/providerstatus` 及前端 provider status API/UI；按 profile、operation reader 和普通/superadmin 投影证据，测试不得出现 secret/config value。
4. 在 `cmd/mochat-go/main.go` 复用同一个 production client/runtime，并保留 existing provider composition gate。

### 批次二：0139 ledger

1. 先在 `internal/store`/migration contract test 写临时 schema RED，验证复合 FK、唯一幂等、状态字段、敏感字段禁止落库。
2. 新增 `deploy/standalone/migrations/0139_wecom_capability_operations.{up,down}.sql`，实现缺表创建、残表完整 signature 预检和安全 down。
3. 在 `internal/wecomcapability` 定义 capability/operation 类型与 store/service boundary；在 `internal/store` 实现 tenant/corp scoped create/claim/update/latest 查询，事务失败回滚。
4. 用 sqlmock/真实临时 schema 验证 duplicate idempotency、跨 tenant/corp 零写、lease fencing、结果唯一性；无 DSN 输出 SKIP。

### 批次三：精准群发闭环

1. 在现有 contact/room handler contract test 先证明 readiness、principal/scope、body tenant/actor 无效和 capability 分离 RED。
2. 将 operation ledger 接入两类创建路径；保留现有业务表兼容读取，确保 batch+operation+audit 同事务。
3. 增加可注入 sender runner/lease contract，复用 `RoomWelcomeWeComClient.SubmitContactMessageBatchSend`/`SubmitRoomMessageBatchSend` 及现有 poll methods；实现结果状态和 retry classification，不用 background_task。
4. 补 fake WeCom HTTP server 合同和两类 worker 的成功、partial、429/5xx/timeout/401、duplicate、callback/poll tests；两种 capability 不互相背书。

### 批次四：门禁、接线与验收

1. provider completion gate 从 production registry/runtime 结构化证据检查 12 类 capability、状态和 operation wiring，并加入未分类/假 ready/ignored fixture RED。
2. 运行相关 Go tests、临时 schema integration（有 DSN 才实跑）、普通 provider gate、phase4 dashboard gate、Dashboard typecheck/lint/tests/build。
3. 请求独立代码审阅；对 Important 逐项补独立 commit；最终核对 clean worktree、提交 SHA、门禁结果、DSN/live 外部阻塞和与主工作区重叠文件。

## 提交边界

- 设计与计划单独提交。
- 能力合同/状态、0139 ledger、群发闭环、门禁/接线分别逻辑提交；不改写历史。
- 任一真实外部凭据缺失只报告 SKIP/limited，不把 fake 或本地状态称为 live ready。
