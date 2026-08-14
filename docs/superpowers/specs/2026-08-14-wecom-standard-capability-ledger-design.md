# WeCom 标准能力账本与精准群发闭环设计

## 范围与证据边界

本批次在 `phase6/provider-foundation-wecom-closeout` 隔离 worktree 实施，不操作 Docker、服务器、业务库。目标是把现有真实 WeCom HTTP client 的能力边界、租户凭据事实、持久化 operation 证据和 Dashboard 结果连接起来；没有真实凭据时只验证 fake HTTP 合同，不把 fake、队列入列或页面状态当成外部成功。

## 盘点结论

- 生产组合根在 `cmd/mochat-go/main.go` 已创建一个 `RoomWelcomeWeComClient`，同一类 client 被员工/部门/客户/标签/群/联系我/欢迎语/转接/应用消息和两种群发 worker 复用；不新增第二套 WeCom client。
- `providers.Registry` 目前以 `wecom_standard` 单项注册，仅分类 `employee_sync`；`providers.Status` 只有 provider 级字段，`ProviderStatusSource` 只把员工同步事实投影为 ready，其他能力没有独立证据。
- 公司 profile 的有效标准凭据事实是 active binding、`Credentials.WeCom.Configured`、非空 corp id 与 `VerifiedAt`；pending/unverified 不读取同步状态。Agent、callback、archive 是独立 credential/config 事实，archive/callback 不应阻塞标准员工同步。
- `contact_message_batch_send.go`、`room_message_batch_send.go` 已负责认证 principal、corp 解析、员工/客户/群归属及 Dashboard RBAC；`batch_send_schedule_cron.go` 与 `RoomWelcomeWeComClient.Submit*BatchSend` 已有外部调用和结果轮询 client，但创建任务与 operation/audit 尚未同事务，外部部分成功也没有 durable fencing/idempotency。
- 现有批量发送表保留业务 payload/任务展示兼容性；本批次新增独立 capability ledger，不把 secret 或完整敏感 payload 写入 ledger。0139 采用“旧 contact/room 父表分阶段演进 + 新 ledger/dispatch/audit 表”，使用 tenant/corp 复合 FK、scope unique idempotency key 和第一条 DDL 前的预检 guard。

## 设计决策

### 1. capability 状态模型

`wecom_standard` 仍是一个 provider kind，注册表稳定登记下列 capability：

`employee_sync`、`department_sync`、`external_contact_sync`、`contact_tag_sync`、`room_sync`、`contact_way`、`welcome_message`、`contact_transfer`、`agent_message`、`contact_batch_send`、`room_batch_send`、`callback`。

凭据边界按能力独立计算：employee/department 需要 `corp_id + employee_secret`；contact/tag/room/contact_way/welcome/contact_batch_send/room_batch_send 需要 `contact_secret`；agent_message 需要 `agent_id + wx_secret`；callback 需要 token、AES key 以及已注入的 callback route/receive evidence。会话存档仍只属于 `wecom_archive`，不参与标准能力 ready 判定。两种群发即使共享 contact secret，也必须各自有 operation/dispatch/result 证据。

在 `providers.Status` 增加结构化 `capabilityStatuses`，每项包含 capability、state、code、source、reason/action、最近同步/成功/失败时间和稳定错误码。注册表的 `Capabilities` 仍是静态分类，租户状态由 runtime provider + profile + 0139 最新 operation 事实计算。普通用户保留 capability/state/code/source/时间等非敏感字段，去除 reason、missing 和配置名；superadmin 才看诊断 reason/missing。任何 capability 只能由自身的成功 operation 证据进入 ready，员工同步成功不替其他能力背书。

状态计算顺序：runtime 未注入则 unavailable；binding/对应 credential 未配置或未验证则 limited/unavailable；无该 capability 的 operation 证据则 limited `wecom.capability_operation_pending`；最近 queued/running 为 limited `wecom.capability_syncing`；最近 failed 为 limited `wecom.capability_operation_failed`；最近 succeeded 才按该 capability 的外部权限/回调要求进入 ready。所有查询严格使用 principal tenant/corp。

### 2. operation/audit ledger

0139 增加：

- `mochat_go_wecom_capability_operations`：tenant/corp/capability/action/idempotency/status/provider_request_id/target_total/success_total/failure_total/error_code/actor_user_id/requested_at/started_at/finished_at/updated_at/lease_token/lease_expires_at/attempt。
- `mochat_go_wecom_capability_operation_results`：operation_id、tenant/corp、target_kind、target_id、status、provider_target_id、error_code、error_message_safe、updated_at。
- `mochat_go_wecom_capability_dispatches`：operation_id、tenant/corp、dispatch_kind、target_id、chunk_no、idempotency_key、status、provider_request_id、lease_token、attempt、next_poll_at、last_error_code、timestamps；每个真实外呼/chunk 一行。

operation 表以 `(tenant_id, corp_id, capability, idempotency_key)` 唯一，结果表以 operation scope + target 唯一；两表对 `mc_corp(tenant_id,id)` 使用复合 FK。迁移使用当前项目的完整 signature guard、同连接 statement runner 和安全 down 顺序。ledger 只写稳定 code、计数、provider task/request id；不写 token、secret、原始 callback/payload。

### 3. 精准群发闭环

创建 contact/room 任务时由 handler 将 body 的 tenant/actor 忽略，以 context principal 和 DashboardAccessContext 为唯一授权事实；先验证 capability readiness、employee/客户/群 scope，再在一个数据库事务中 claim idempotency、创建 operation、保留现有业务 batch/task 记录和 audit。若业务表或 operation 任一写失败整事务回滚。创建和 worker 领取前都重新验证 SaaS tenant gate、package/subscription/quota、tenant-corp binding、credential generation、员工/客户/群当前归属与 RBAC scope；租户停用后不继续外呼，已提交外部任务只允许轮询。

父 operation 采用 `pending/claimed/submitting/submitted/polling/succeeded/partial_failed/failed/cancelled`；旧 `send_status` 只作为兼容投影。发送 worker 从 queued operation 原子领取 lease，使用不可猜 lease token 与 attempt fencing；每一 chunk 先落一条 dispatch，再外呼，外部响应保存独立 provider request/message id 和逐目标结果，按结果计算父状态。429/5xx/超时保留可重试 dispatch，401/权限/合同错误收敛 failed；提交成功必须有非空 msgid，没有外部成功证据不写 completed。重复 idempotency 直接返回原 operation，绝不再次外发；callback/poll 复用现有 `GroupMessageTasks`、`GroupMessageSendResults`，遍历 next_cursor、退避并有终态/死信。

所有 ledger/dispatch/result 写查都绑定 `tenant_id + corp_id + id`；跨租户或跨企业请求 404 且零写。系统 actor 使用 nullable actor + `source=system`，禁止用 0；remind 只能作用于该 batch target 且通过当前 scope。

### 4. 兼容边界

现有旧页面与表字段继续服务展示；ledger 是能力事实来源，不用 `background_task` 自证。未能安全接入真实外部闭环的 capability 保持 limited，并显示就近 machine code/action。普通 status API 不接受客户端 tenant/corp 覆盖。

## 测试证据

- 单测先 RED 后 GREEN：能力分类/状态矩阵、普通用户脱敏、tenant/corp 隔离、无 operation 不 ready、失败/重试/部分成功、幂等与 lease fencing。
- fake HTTP server 验证 gettoken、contact 与 room 请求体、成功/部分失败、429/5xx/超时、401、provider request id、poll/callback 收敛；不发 live 请求。
- 0139 临时 schema integration 使用 admin DSN；无 DSN 明确 `SKIP`，不宣称真实集成通过。
- provider completion gate 增加 capability 分类、operation evidence 和 production composition/runtime wiring 的坏 fixture；最终运行 Go、Dashboard typecheck/lint/tests/build 与已有 phase4/provider gates。
- 迁移先回填父业务表 tenant_id，再添加 `(tenant_id,corp_id,id)` unique 和 ledger/dispatch/result 复合 FK；第一条 DDL 前检查重复、悬空和跨 corp 数据。MariaDB 分阶段 apply/down/部分恢复均需要临时 schema 实测。
- production runtime evidence 必须覆盖四个群发/同步 cron 开关及 callback route 注入；页面 `show/results/remind/delete` 现有 denyOnly 合同若属于闭环页面，则补精确 page mapping 和真实 guard dispatch 测试。
## 预审 Critical 收敛（编码前锁定）

1. `capabilityStatuses` 必须是强类型数组。凭据边界独立为：employee/department 使用 corp_id+employee_secret；contact/tag/room/contact_way/welcome/contact_batch_send/room_batch_send 使用 contact_secret；agent_message/remind 使用 agent_id+wx_secret；callback 必须同时有 token、AES、route/receive evidence；archive 只属于 `wecom_archive`。两个群发 capability 即使共享 secret，也必须有各自 Worker、operation、dispatch 和 result 证据。
2. operation 与 dispatch 使用两套明确状态：父 operation 为 pending/claimed/submitting/submitted/polling/succeeded/partial_failed/failed/cancelled；dispatch 为 queued/claimed/submitting/submitted/polling/succeeded/partial_failed/failed，未知值 fail closed。每次真实外呼/chunk 先落权威 dispatch，独立 msgid、稳定幂等键、lease/fencing，禁止多 chunk 后写覆盖单一 msgid。
3. 创建与 claim 都重新验证 SaaS tenant gate、package/subscription/quota、tenant-corp binding、credential generation、目标当前归属和 Dashboard scope；停用租户不继续外发，已提交任务只允许轮询。所有读写绑定 tenant_id+corp_id+id，跨界 404 零写；系统 actor 使用 nullable + source=system，禁止 actor=0。
4. credential generation/fingerprint 不含 secret，employee/contact/agent/callback 各自独立；operation/dispatch 记录 generation，状态只采当前 generation 的成功证据，轮换后旧 evidence 与旧 worker 失效。generation 原值不进 API、日志或错误文案。
5. 普通用户按显式 Dashboard page/permission→capability mapping 过滤，未知 capability 默认不返回；superadmin 才看完整列表。`show/results/remind/delete` 的 denyOnly/pageMapped 必须与真实 guard dispatch 一致；remind 还要证明 employee 属于 batch target 且在当前 scope。
## 迁移与审计一致性补充

0139 是“旧父表分阶段演进 + 新 ledger/dispatch/audit 表”的迁移，不是纯 append-only：第一条 DDL 前检查旧 contact/room batch 表重复、悬空和跨 corp 数据，回填并建立 tenant/corp 复合 scope；旧字段继续兼容读取，down 只能在安全条件满足时恢复新增列/索引。新增 `mochat_go_wecom_capability_operation_audits`/events append-only 表，记录 tenant/corp、operation、可空 dispatch、from_status/to_status、action、可空 actor_user_id、actor_source、request_id、安全 machine code/counts/timestamp；复合 FK 严格绑定 scope，禁止 payload/secret。HTTP create、claim、submit、retry、poll、final、cancel 每次状态迁移都在同事务 append；外呼后若本地 audit 失败，dispatch 进入可恢复 reconcile，禁止再次外发。
## 页面授权映射口径

能力可见性只消费 `DashboardAccessContext.PermissionCodes` 中的真实 page code，并与 `internal/dashboard/dashboard_page_catalog.json` 做交叉校验；绝不从 `/dashboard/...` API path、请求字符串或 provider kind 推导授权。当前精确映射包括：`dashboard.acquisition.precise_group_send`→contact_batch_send+room_batch_send；`dashboard.acquisition.v2_channel_code`、`dashboard.acquisition.group_code`、`dashboard.acquisition.group_template`→contact_way；`dashboard.customer.contact`/`dashboard.customer.friends`→external_contact_sync；`dashboard.customer.group`→room_sync；`dashboard.customer.tags`→contact_tag_sync。未知 page code 与 API path 均暴露 0 个 capability，catalog page code 增删涉及这些页面时 gate 失败。
