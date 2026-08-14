# WeCom 标准能力账本与精准群发闭环设计

## 范围与证据边界

本批次在 `phase6/provider-foundation-wecom-closeout` 隔离 worktree 实施，不操作 Docker、服务器、业务库。目标是把现有真实 WeCom HTTP client 的能力边界、租户凭据事实、持久化 operation 证据和 Dashboard 结果连接起来；没有真实凭据时只验证 fake HTTP 合同，不把 fake、队列入列或页面状态当成外部成功。

## 盘点结论

- 生产组合根在 `cmd/mochat-go/main.go` 已创建一个 `RoomWelcomeWeComClient`，同一类 client 被员工/部门/客户/标签/群/联系我/欢迎语/转接/应用消息和两种群发 worker 复用；不新增第二套 WeCom client。
- `providers.Registry` 目前以 `wecom_standard` 单项注册，仅分类 `employee_sync`；`providers.Status` 只有 provider 级字段，`ProviderStatusSource` 只把员工同步事实投影为 ready，其他能力没有独立证据。
- 公司 profile 的有效标准凭据事实是 active binding、`Credentials.WeCom.Configured`、非空 corp id 与 `VerifiedAt`；pending/unverified 不读取同步状态。Agent、callback、archive 是独立 credential/config 事实，archive/callback 不应阻塞标准员工同步。
- `contact_message_batch_send.go`、`room_message_batch_send.go` 已负责认证 principal、corp 解析、员工/客户/群归属及 Dashboard RBAC；`batch_send_schedule_cron.go` 与 `RoomWelcomeWeComClient.Submit*BatchSend` 已有外部调用和结果轮询 client，但创建任务与 operation/audit 尚未同事务，外部部分成功也没有 durable fencing/idempotency。
- 现有批量发送表保留业务 payload/任务展示兼容性；本批次新增独立 capability ledger，不把 secret 或完整敏感 payload 写入 ledger。0139 仅追加表并使用 tenant/corp 复合 FK、scope unique idempotency key 和预检 guard。

## 设计决策

### 1. capability 状态模型

`wecom_standard` 仍是一个 provider kind，注册表稳定登记下列 capability：

`employee_sync`、`department_sync`、`external_contact_sync`、`contact_tag_sync`、`room_sync`、`contact_way`、`welcome_message`、`contact_transfer`、`agent_message`、`contact_batch_send`、`room_batch_send`、`callback`。

在 `providers.Status` 增加结构化 `capabilityStatuses`，每项包含 capability、state、code、source、reason/action、最近同步/成功/失败时间和稳定错误码。注册表的 `Capabilities` 仍是静态分类，租户状态由 runtime provider + profile + 0139 最新 operation 事实计算。普通用户保留 capability/state/code/source/时间等非敏感字段，去除 reason、missing 和配置名；superadmin 才看诊断 reason/missing。任何 capability 只能由自身的成功 operation 证据进入 ready，员工同步成功不替其他能力背书。

状态计算顺序：runtime 未注入则 unavailable；binding/对应 credential 未配置或未验证则 limited/unavailable；无该 capability 的 operation 证据则 limited `wecom.capability_operation_pending`；最近 queued/running 为 limited `wecom.capability_syncing`；最近 failed 为 limited `wecom.capability_operation_failed`；最近 succeeded 才按该 capability 的外部权限/回调要求进入 ready。所有查询严格使用 principal tenant/corp。

### 2. operation/audit ledger

0139 增加：

- `mochat_go_wecom_capability_operations`：tenant/corp/capability/action/idempotency/status/provider_request_id/target_total/success_total/failure_total/error_code/actor_user_id/requested_at/started_at/finished_at/updated_at/lease_token/lease_expires_at/attempt。
- `mochat_go_wecom_capability_operation_results`：operation_id、tenant/corp、target_kind、target_id、status、provider_target_id、error_code、error_message_safe、updated_at。

operation 表以 `(tenant_id, corp_id, capability, idempotency_key)` 唯一，结果表以 operation scope + target 唯一；两表对 `mc_corp(tenant_id,id)` 使用复合 FK。迁移使用当前项目的完整 signature guard、同连接 statement runner 和安全 down 顺序。ledger 只写稳定 code、计数、provider task/request id；不写 token、secret、原始 callback/payload。

### 3. 精准群发闭环

创建 contact/room 任务时由 handler 将 body 的 tenant/actor 忽略，以 context principal 和 DashboardAccessContext 为唯一授权事实；先验证 capability readiness、employee/客户/群 scope，再在一个数据库事务中 claim idempotency、创建 operation、保留现有业务 batch/task 记录和 audit。若业务表或 operation 任一写失败整事务回滚。

发送 worker 从 queued operation 原子领取 lease，使用不可猜 lease token 与 attempt fencing；外部响应先保存 provider request/message id 和逐目标结果，按结果计算 succeeded/partial/failed。429/5xx/超时保留 queued/limited 并可重试，401/权限/合同错误收敛 failed；没有外部成功证据不写 completed。重复 idempotency 直接返回原 operation，绝不再次外发；callback/poll 复用现有 `GroupMessageTasks`、`GroupMessageSendResults` 合同更新结果。

### 4. 兼容边界

现有旧页面与表字段继续服务展示；ledger 是能力事实来源，不用 `background_task` 自证。未能安全接入真实外部闭环的 capability 保持 limited，并显示就近 machine code/action。普通 status API 不接受客户端 tenant/corp 覆盖。

## 测试证据

- 单测先 RED 后 GREEN：能力分类/状态矩阵、普通用户脱敏、tenant/corp 隔离、无 operation 不 ready、失败/重试/部分成功、幂等与 lease fencing。
- fake HTTP server 验证 gettoken、contact 与 room 请求体、成功/部分失败、429/5xx/超时、401、provider request id、poll/callback 收敛；不发 live 请求。
- 0139 临时 schema integration 使用 admin DSN；无 DSN 明确 `SKIP`，不宣称真实集成通过。
- provider completion gate 增加 capability 分类、operation evidence 和 production composition/runtime wiring 的坏 fixture；最终运行 Go、Dashboard typecheck/lint/tests/build 与已有 phase4/provider gates。
