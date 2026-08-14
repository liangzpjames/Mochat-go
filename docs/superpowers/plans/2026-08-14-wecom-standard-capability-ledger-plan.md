# WeCom 标准能力账本与精准群发实施计划

## 目标

在现有隔离分支完成真实 runtime 证据驱动的 `wecom_standard` capability 状态、0139 operation/audit ledger 和 contact/room 精准群发可恢复闭环；不改变旧迁移，不操作外部环境。

## 实施步骤

### 批次一：能力合同与状态投影

1. 在 `internal/modules/providers` 增加结构化 capability status 字段和稳定 capability 常量，先补 registry snapshot、非法状态和缺失分类 RED。
2. 扩展 `internal/modules/providers/catalog/catalog.go` 与 `internal/dashboard/room_welcome_wecom.go`，登记 12 个标准能力；runtime 不可用时保持 unavailable，runtime 可用但未绑定租户时保持 limited。
3. 扩展 `internal/companyprofile/provider_status.go`、`internal/providerstatus` 及前端 provider status API/UI；按 profile、operation reader 和普通/superadmin 投影证据，测试不得出现 secret/config value。
4. 在 `cmd/mochat-go/main.go` 复用同一个 production client/runtime，并保留 existing provider composition gate。

### 批次二：0139 capability ledger

1. 先在 `internal/store`/migration contract test 写临时 schema RED，验证复合 FK、唯一幂等、状态字段、敏感字段禁止落库。
2. 新增 `deploy/standalone/migrations/0139_wecom_capability_operations.{up,down}.sql`，先处理现有 contact/room batch 父表 tenant/corp 复合 scope，再创建 operation、dispatch、result，完整检查 INT UNSIGNED、唯一键、复合 FK、重复/悬空/跨 corp 数据，支持 MariaDB 分阶段 apply/down/恢复。
3. 在 `internal/wecomcapability` 定义 capability/operation 类型与 store/service boundary；在 `internal/store` 实现 tenant/corp scoped create/claim/update/latest 查询，事务失败回滚。
4. 用 sqlmock/真实临时 schema 验证 duplicate idempotency、跨 tenant/corp 零写、lease fencing、结果唯一性；无 DSN 输出 SKIP。

### 批次三：durable dispatch 基础

1. operation 下增加每外呼/chunk 一行的 dispatch，父状态采用 pending/claimed/submitting/submitted/polling/succeeded/partial_failed/failed/cancelled；每条 dispatch 有稳定 idempotency、lease token、attempt、provider request id。
2. 先写 RED 锁定原子 claim、credential generation、tenant gate/package/quota/scope 重查、旧 lease fencing 和重复 chunk 不外发；已提交任务只轮询。

### 批次四：精准群发闭环

1. 在现有 contact/room handler contract test 先证明 readiness、principal/scope、body tenant/actor 无效、当前员工/客户/群归属、quota 和 capability 分离 RED。
2. 将 operation/dispatch ledger 接入两类创建路径；保留现有业务表兼容读取，确保 batch+operation+audit 同事务。
3. 增加可注入 sender runner/lease contract，复用 `RoomWelcomeWeComClient.SubmitContactMessageBatchSend`/`SubmitRoomMessageBatchSend` 及现有 poll methods；实现每 chunk msgid、逐结果、重试/死信，不用 background_task。
4. 补 fake WeCom HTTP server 合同和两类 worker 的成功、partial、429/5xx/timeout/401、duplicate、callback/poll tests；两种 capability 不互相背书。

### 批次五：页面、门禁、接线与验收

1. provider completion gate 从 production registry/runtime 结构化证据检查 12 类 capability、按凭据分组、四个 cron 开关、callback route、operation evidence，并加入未分类/假 ready/ignored fixture RED。
2. 对 `show/results/remind/delete` 检查 denyOnly 与精确 page mapping/真实 dispatch guard，补普通用户和超管边界。
3. 运行相关 Go tests、临时 schema integration（有 DSN 才实跑）、普通 provider gate、phase4 dashboard gate、Dashboard typecheck/lint/tests/build。
3. 请求独立代码审阅；对 Important 逐项补独立 commit；最终核对 clean worktree、提交 SHA、门禁结果、DSN/live 外部阻塞和与主工作区重叠文件。

## 提交边界

- 设计与计划单独提交。
- 能力合同/状态、0139 ledger、群发闭环、门禁/接线分别逻辑提交；不改写历史。
- 任一真实外部凭据缺失只报告 SKIP/limited，不把 fake 或本地状态称为 live ready。
## 主审补充的强制拆分

本批不以“注册 capability 名称”替代真实闭环，按五个可独立验收提交推进：

1. capability ledger：强类型 capabilityStatuses、四组凭据 generation/verifiedAt、runtime/operation 证据和 access profile mapping。
2. durable dispatch schema：0139 operation/dispatch/result，父子状态分层、每 chunk 一行、复合 FK/唯一键/lease fencing、父表回填与 MariaDB apply-down-apply。
3. contact batch：重新租户/套餐/quota/归属/scope 门禁、独立 contact dispatch、msgid、poll/callback 与 retry。
4. room batch：独立 room dispatch、群主/群归属与结果链；不得以 contact batch 成功替代 room batch。
5. 页面/路由/门禁：普通用户 capability mapping、superadmin 全量、show/results/remind/delete 真实 catalog/guard/page mapping；再跑全门禁并独立审阅。

每批先 RED 再 GREEN，未完成后续批次不得宣称精准群发产品闭环完成。
## 迁移与审计一致性补充

0139 明确为分阶段演进：先对旧 contact/room 父表做重复、悬空、跨 corp 预检，回填 tenant_id 并建 `(tenant_id,corp_id,id)` unique，保留旧字段兼容；再创建 operation、dispatch、result 与 append-only operation_audits/events，最后建立复合 FK/唯一键。down 按依赖逆序删除新表并仅在新增列无外部依赖时恢复旧结构；每次状态迁移与 HTTP 创建必须同事务写 event，audit 失败回滚，外呼后失败只能进入 reconcile。
页面过滤实现只能精确匹配真实 `dashboard_page_catalog.json` page code；先以坏测试证明 `/dashboard/contactMessageBatchSend/index#get` 不会授权，再以 `dashboard.acquisition.precise_group_send` 等真实 code GREEN，并由 catalog 交叉 gate 锁定 mapping 漂移。
