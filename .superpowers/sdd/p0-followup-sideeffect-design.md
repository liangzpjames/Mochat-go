# 企业微信 callback side-effect `unknown` 生产恢复闭环设计

日期：2026-08-30
调查基线：`aa1fa09a7c546c8fe251b3c6d3efe53ce69626d8`
范围：设计并实施 `0174` 产生的 callback 外部副作用 `unknown` 恢复闭环；不调用真实企业微信。

## 1. 结论

采用“人工证明 Provider 事实，再由 durable worker 重放”的恢复模型：

- `confirm_sent`：有证据证明 Provider 已发送，只把目标 action 从 `unknown` 记为 `sent`；管理 API 不再外发。
- `confirm_not_sent_and_retry`：有证据证明 Provider 未发送，只把目标 action 从 `unknown` 重置为 `pending`；外发仍只能由 callback worker 执行。
- 两种命令都与 command receipt、审计、inbox 复活在一个 MySQL 事务中提交。系统绝不因超时自动把 `unknown` 当成未发送。
- 管理面放在 Dashboard 企业设置，而不是 SaaS 总后台。tenant/corp 只能来自已认证 `DashboardPrincipal` 和服务端 single-corp binding，body/path/query 均不能指定作用域。
- 一个 callback 最多存在 `fission.employee_reminder` 和 `fission.customer_push` 两个 action。命令只改变指定 action；已经 `sent` 的 action永不回退。只要仍有另一个 `unknown`，就不复活 inbox。

这里不能宣称 exactly-once Provider delivery。没有 Provider 原生幂等或可查询 receipt 时，系统能保证的是：`unknown` 自动 fail-closed、命令数据库 exactly-once、已确认 `sent` 不重复外发，以及只有“明确确认未发送”才重新尝试。

## 2. 当前实现与根因

### 2.1 现状

- `0172_wework_callback_inbox` 以 `(tenant_id, corp_id, event_key)` 唯一接收 callback，claim 使用随机 lease token、单调 `lease_fence` 和到期时间；Complete/Fail/Defer 都要求有效 lease。
- `0174_wework_callback_side_effects` 以 `(tenant_id, corp_id, event_key, action_key)` 唯一，业务事务先写 `pending` intent；worker 外发前将其置为 `unknown`，成功后置为 `sent`。
- `BeginWeWorkCallbackSideEffect` 对 `unknown/sent` 均返回“不执行”，因此恢复前不会再次外发；这正是正确的 fail-closed 基线。
- 当前没有 `unknown` 列表、详情、版本、操作员决议、command receipt 或受控复活 inbox 的入口，`unknown` 最终只能把 inbox 重试耗尽到 `dead`。

### 2.2 必须同时修的竞态

现有 side-effect Begin/Complete 没有在同一数据库事务内复核 inbox lease。若操作员在旧 lease 到期后恢复，旧 worker 仍可能随后执行 Begin 或 Complete。仅给恢复命令加 `FOR UPDATE` 不够；必须把 worker 的 side-effect 状态转换也绑定到同一 inbox lease token/fence，并统一锁顺序。

另外，Provider 调用已经开始但进程/网络结果尚未稳定时，lease 可能先到期。恢复入口必须尊重 `reconcile_after` 隔离期，不能在 Provider 调用仍可能进行时允许人工决议。

## 3. 路由与身份边界

### 3.1 选 Dashboard，不选 SaaS

沿用既有设计合同和企业设置资源：

- Dashboard principal 已由认证 identity、SaaS tenant gate、唯一 corp binding 顺序构造，包含服务端可信 `UserID/TenantID/CorpID/AuthVersion/CorpStatus`。
- `DashboardAccessGuard` 已把 `/dashboard/company/*` 绑定到页面 RBAC；新增路由归属 `dashboard.company_setting.website`。
- SaaS principal 是平台身份且刻意不携带业务 tenant/corp。虽然 SaaS 已有 `platform.integrations.*`，新建 SaaS 写入口会额外引入跨租户目标选择和第二套审计/授权边界。本闭环是租户企业自己的 callback 事实确认，不需要平台跨租户能力。

因此不新增 `/dashboard/saasAdmin/*` side-effect 路由。SaaS 管理员若需要协助，应通过受控租户会话/支持流程取得租户侧授权，而不是 body 指定 tenant/corp。

### 3.2 精确 HTTP 接口

1. `GET /dashboard/company/callback-side-effects?status=unknown&cursor={opaque}&limit=50`
   - `status` 当前只接受 `unknown`；`limit` 1–100，默认 50。
   - 稳定 keyset 顺序：`unknown_at DESC, event_key DESC, action_key DESC`。cursor 为 base64url 编码的上述三元组，服务端严格解码和绑定参数，不含 tenant/corp。
   - 只返回摘要：`eventKey/actionKey/payloadHash/status/version/unknownAt/reconcileAfter/actionable/inboxStatus/attempt/lastErrorCode`。不返回 `event_json`、callback 正文、员工/客户正文或 Provider 凭据。

2. `GET /dashboard/company/callback-side-effects/{eventKey}/{actionKey}`
   - `eventKey` 必须是 64 位小写 hex；`actionKey` 只允许两个已知常量。
   - 返回该事件 inbox 摘要和两个 action 的状态快照，便于确认另一 action 是否仍为 `unknown`；仍不返回正文。

3. `POST /dashboard/company/callback-side-effects/{eventKey}/{actionKey}/reconcile`
   - 必须带 `Idempotency-Key`，16–128 个安全 ASCII 字符。
   - JSON 使用严格 decoder、拒绝未知字段和尾随 JSON：

```json
{
  "decision": "confirm_sent",
  "expectedVersion": 3,
  "expectedInboxLeaseFence": 8,
  "reason": "已在企业微信管理端按消息编号核验",
  "evidenceKind": "provider_delivery_query_sent",
  "evidenceRef": "ticket/WECOM-20260830-001"
}
```

   - `decision` 仅允许 `confirm_sent`、`confirm_not_sent_and_retry`。
   - `confirm_sent` 的证据类型只允许 `provider_message_id`、`provider_delivery_query_sent`、`provider_support_confirmed_sent`。
   - `confirm_not_sent_and_retry` 只允许 `provider_delivery_query_absent`、`provider_request_rejected`、`provider_support_confirmed_not_sent`。不接受“超时”“主观判断”或本地 fixture 作为生产证据。
   - `reason` 1–255 字；`evidenceRef` 1–255 字，只保存引用，不保存 token/secret/消息正文。
   - 成功返回目标 action 的事务结果、`idempotent`、`inboxReplayScheduled`、`remainingUnknownActions` 和本次 `wakeupAccepted`。receipt 重放稳定返回首次事务结果并标记 `idempotent=true`；只要首次事务安排了 replay，每次 HTTP 重放都可重新 best-effort wakeup。wakeup 使用有界 Redis list token，worker 以阻塞 pop 消费；`wakeupAccepted` 表示本次在 50ms 独立超时内成功入队，不属于持久化首次结果。即使入队失败，MySQL 定时 claim 仍是正确性来源。

所有接口均从 `dashboardprincipal.DashboardPrincipalFromContext` 取作用域。principal 非 active binding、无权限或不完整直接 fail closed；Store 查询必须同时带 `tenant_id=? AND corp_id=?`。跨租户、跨 corp、未知 event/action 统一 404，避免资源枚举。

## 4. 状态机

```text
业务 Tx 创建 intent
        |
        v
     pending --[有效 inbox lease/fence；worker 获得执行权]--> unknown
        ^                                                   |
        |                                                   | Provider 成功且同 lease/fence 下 receipt 入库
        |                                                   v
        +--[confirm_not_sent_and_retry]-- unknown --------> sent
                                            |
                                            +--[confirm_sent]
```

约束：

- `pending -> unknown` 只允许 worker；`unknown -> sent/pending` 只允许恢复命令；`sent` 终态不可回退。
- worker 的 Begin 和 Complete 都要在短事务内先锁 inbox，再锁单个 side-effect，并验证完整 `(tenant, corp, event, lease_token, lease_fence, status=processing, lease_expires_at>now)`。
- Begin 将 `unknown_at=UTC_TIMESTAMP(6)`、`reconcile_after=now+15min`。15 分钟是 Provider 请求超时与 callback lease 上限之外的保守隔离期；配置若可调，必须保证不小于 `provider timeout + max lease duration + 60s`。
- 恢复命令在 `reconcile_after` 前返回 409。它不把等待时间当成 Provider 结果。
- 恢复命令锁住同事件全部 action（按 `action_key ASC`）后只更新目标 action。若仍有其他 `unknown`，保持 inbox 原状态；最后一个 `unknown` 被解决后才复活 inbox。
- 复活 inbox 时只接受 `dead`、`pending`，或同时具备非空 lease token、非空 lease expiry 且 expiry 已过期的结构完整 `processing`。`completed`、未知状态、缺 token/expiry 的损坏 processing，以及未来 expiry（即使 token 为空）均返回 409。复活将 `status='pending'`、`attempt=0`、清空 lease/next error/completed、`lease_fence=lease_fence+1`，从而 fencing 掉旧 worker。
- 即使所有 action 已 `sent`，也让正式 worker 重放并完成 inbox，不能由管理 API 直接把 inbox 标为 completed，因为 callback 还可能有非外发业务步骤。
- 历史未知 action 显示在详情中但不可决议，并阻止 inbox 复活，返回 `UNSUPPORTED_ACTION_REQUIRES_UPGRADE`。

## 5. 事务、锁顺序与命令幂等

### 5.1 统一锁顺序

恢复事务固定为：

1. 校验 principal/RBAC（HTTP guard 已做，Store 再检查 principal scope）；
2. 当前 Dashboard actor、`mochat_go_tenant_corp_bindings` 与 `mc_corp` 的准确 active scope，全部在事务中锁定并复核 auth version/status；
3. 以随机 reservation token 对 `(tenant_id,corp_id,request_id)` command receipt 执行 `INSERT ... ON DUPLICATE KEY` 并 `FOR UPDATE`，先序列化跨事件相同 request key；
4. inbox 行 `FOR UPDATE`；
5. 同事件 side-effect 行按 `action_key ASC FOR UPDATE`；
6. side-effect、inbox、审计与 receipt 首次结果更新。

worker 的 Begin/Complete 固定为 inbox → side-effect。任何路径都不得反向先锁 side-effect 再锁 inbox。事务内不调用 Provider、不做网络 I/O。

### 5.2 command receipt

`0176` 新建提交后不可变的 `mochat_go_wework_callback_side_effect_commands`。事务内先写不可见的 reservation，随后同事务完成首次结果：

- 身份：`tenant_id, corp_id, event_key, action_key`；
- `request_id varchar(128) ascii_bin`，唯一键 `(tenant_id, corp_id, request_id)`；
- `decision varchar(40)`、`request_fingerprint binary(32)`、`expected_version`、`expected_inbox_lease_fence`；
- 事务 reservation：随机 `reservation_token`；不依赖 action 外键，避免在 inbox 之前隐式锁 action；
- 首次结果：`result_status`、`result_version`、`result_inbox_lease_fence`、`replay_scheduled`、`remaining_unknown_actions`；
- 操作者与证据：`actor_user_id`、`reason`、`evidence_kind`、`evidence_ref`；
- `operation_audit_id`、`created_at`。不保存 callback/Provider payload。

fingerprint 是规范化 `{tenant,corp,eventKey,actionKey,decision,expectedVersion,expectedInboxLeaseFence,reason,evidenceKind,evidenceRef}` 的 SHA-256。reservation 的唯一键会让跨事件并发请求先在 receipt 层串行化：同 fingerprint 返回首次结果且不重复审计、不再次递增版本；不同 fingerprint 返回 409 `IDEMPOTENCY_CONFLICT`，不得因 duplicate race 降级成 503。事务回滚会同时撤销 reservation，使下一次请求可安全接管。

receipt、目标 action、inbox 和 `mochat_go_dashboard_permission_audits` 必须同事务。任一 RowsAffected 不是恰好 1、审计失败或 commit 失败全部回滚。

## 6. `0176` migration 设计

文件名：

- `deploy/standalone/migrations/0176_wework_callback_side_effect_reconciliation.up.sql`
- `deploy/standalone/migrations/0176_wework_callback_side_effect_reconciliation.down.sql`

### 6.1 up

对 `mochat_go_wework_callback_side_effects` 新增：

- `version bigint unsigned NOT NULL DEFAULT 1`
- `reconciliation_fence bigint unsigned NOT NULL DEFAULT 0`
- `unknown_at datetime(6) NULL`
- `reconcile_after datetime(6) NULL`
- `last_decision varchar(40) NOT NULL DEFAULT ''`
- `last_reason varchar(255) NOT NULL DEFAULT ''`
- `last_evidence_kind varchar(40) NOT NULL DEFAULT ''`
- `last_evidence_ref varchar(255) NOT NULL DEFAULT ''`
- `last_reconciled_by int unsigned NULL`
- `last_reconciled_at datetime(6) NULL`
- 索引 `(tenant_id,corp_id,status,unknown_at,event_key,action_key)`

回填现有 `unknown`：`unknown_at=updated_at`，`reconcile_after=DATE_ADD(updated_at, INTERVAL 15 MINUTE)`；其他状态保持 NULL。

创建上述 command receipt 表，主键 bigint 自增，唯一键 `(tenant_id,corp_id,request_id)`，scope 索引 `(tenant_id,corp_id,event_key,action_key,created_at)`。receipt 不以外键引用 action，避免 reservation 在既定 inbox→action 锁序之前隐式取得 action 锁；事件/action 的存在性仍在同一事务中于 inbox/action 行锁后验证。审计仍写既有 `mochat_go_dashboard_permission_audits`，避免形成第二套审计查询面。

在 `mochat_go_dashboard_permission_resources` 为 `dashboard.company_setting.website` 增加三条资源合同：列表 GET、详情 GET、reconcile POST；使用 `INSERT ... SELECT ... WHERE NOT EXISTS`，不伪造 permission。

更新真实 migration registry 基线、SaaS system-health 期望版本/数量和 migration contract tests 到 0176。

### 6.2 down

按依赖逆序：删除三条 0176 RBAC resource，DROP command receipt 表，DROP 新索引，再逐列删除 0176 新增字段；不删除 `0174` intent 表，不改变 `pending/unknown/sent`。

down 会丢失恢复命令元数据，运行前必须：停止 callback worker/管理写入，确认没有 `status='unknown'` 或未完成恢复，导出 command receipt 与对应 Dashboard audit，做库备份并校验，执行 down 后核验 0174 表、外键、唯一键仍在。SQL 只使用 MySQL 5.7 支持的 `ALTER TABLE`、`DATE_ADD`、普通索引/外键；不使用 CTE、window function、`SKIP LOCKED`、`RETURNING`、表达式索引或 `DROP ... IF EXISTS` 列语法。

## 7. 错误语义

| HTTP | code | 语义 |
|---|---|---|
| 400 | `INVALID_REQUEST` | 格式、action、decision、证据或 cursor 非法 |
| 401 | `SESSION_INVALID` | principal 缺失/失效 |
| 403 | `PERMISSION_DENIED` | 页面 RBAC 或 active binding 不允许 |
| 404 | `TARGET_NOT_FOUND` | 当前 principal scope 内无该 event/action；跨 scope 同样返回此码 |
| 409 | `IDEMPOTENCY_CONFLICT` | 同 request key 不同 fingerprint |
| 409 | `VERSION_CONFLICT` | action version 已变化 |
| 409 | `LEASE_FENCE_CONFLICT` | inbox fence 与读取快照不一致 |
| 409 | `CALLBACK_LEASE_ACTIVE` | inbox 仍有有效 worker lease |
| 409 | `CALLBACK_INBOX_STATE_CONFLICT` | inbox 已完成、状态未知或 processing lease 结构损坏 |
| 409 | `RECONCILIATION_QUARANTINE_ACTIVE` | 尚未到 reconcile_after |
| 409 | `SIDE_EFFECT_STATE_CONFLICT` | action 已非 unknown；同命令 receipt 重放除外 |
| 409 | `UNSUPPORTED_ACTION_REQUIRES_UPGRADE` | 历史未知 action 阻止安全重放 |
| 503 | `CALLBACK_RECOVERY_UNAVAILABLE` | DB/审计/队列能力不可用 |

worker 遇到 `unknown` 继续返回 `ErrWeWorkCallbackSideEffectReconcileRequired`，阻止 inbox Complete。lease/fence 失败统一返回 `ErrWeWorkCallbackLeaseLost`，不得吞错或降级成非 durable best-effort。

## 8. 需改文件

新增：

- `deploy/standalone/migrations/0176_wework_callback_side_effect_reconciliation.{up,down}.sql`
- `internal/companyprofile/callback_side_effect_recovery.go`：DTO、Service、校验、状态/错误合同。
- `internal/store/wework_callback_side_effect_recovery.go`：租户范围读模型和单事务 reconcile。
- `internal/companyprofile/callback_side_effect_recovery_test.go`
- `internal/store/wework_callback_side_effect_recovery_test.go`
- `internal/store/wework_callback_side_effect_recovery_integration_test.go`
- `internal/migration/wework_callback_side_effect_reconciliation_test.go`

修改：

- `internal/dashboard/wework_callback_inbox.go`：execution 增加 lease token；side-effect Store Begin/Complete 增加 lease/fence 参数及恢复错误。
- `internal/store/wework_callback_side_effect.go`：worker 状态转换改为 inbox → side-effect 的 fenced 短事务，写 unknown/reconcile 时间与版本。
- `internal/dashboard/wework_callback_worker.go` 及测试：透传 lease token/fence；两个 action 继续独立，`unknown` 继续冒泡。
- `internal/companyprofile/http.go`、`model.go`、`service.go`：注册三条 API、严格解码、principal-only scope。
- `internal/store/company_profile.go`：如复用 Store interface，挂接 recovery store；审计写既有 permission audit。
- `internal/server/server.go`、`internal/dashboard/dashboard_route_registry.go`、`internal/dashboard/dashboard_page_catalog.json`、`internal/dashboard/dashboard_access_guard.go`：路由、RBAC 与 enabled-routes 合同。
- `cmd/mochat-go/main.go`：把 MySQL recovery store 与 callback wakeup 注入 company profile handler。
- `internal/migration/migration_test.go`、`internal/dashboard/saas_admin_system_health.go` 及对应测试：最新版本 0176/数量 176。

无需修改 SaaS Admin 路由、SaaS principal 或 `platform.integrations.*` 权限。

## 9. TDD 与验证矩阵

### 9.1 RED 合同测试

- 路由：三条路由能到专用 handler；未注册、错误 method、非规范 event/action 返回 404/405。
- principal/RBAC：缺 principal、零 tenant/corp、pending/suspended binding、无 `dashboard.company_setting.website` 均在 Store 前失败；body 中 tenant/corp/actor 字段因未知字段失败。
- 列表/详情：只看本 tenant/corp；稳定 cursor 无重复/遗漏；不泄露 event_json、正文、token、secret。
- 状态机：两种 decision 的合法/非法转换；sent 不回退；隔离期未到拒绝；未知 action fail closed。
- 双 action：确认 reminder 不改 customer；customer 仍 unknown 时不复活；最后一个 unknown 解决后才复活；一个 sent、一个 pending 的重放只外发 pending。
- 幂等：同 key/同 fingerprint 返回首次结果；同 key/异 payload 409；32 路并发只一个版本增量、一个审计、一个 inbox 复活。
- 故障注入：receipt、action update、inbox update、audit、commit 任一点失败全部回滚。
- lease/fence：活动 lease 拒绝；过期 lease 可恢复且 fence+1；旧 worker 随后 Begin/Complete/CompleteInbox 全部 lease lost。

### 9.2 fake/契约 worker 测试

- fake Provider 记录 action 调用次数，不联网。
- `confirm_sent` 后重放：目标 action 0 次外发，另一个 pending action恰 1 次，最终 inbox completed。
- `confirm_not_sent_and_retry` 后重放：目标 action恰 1 次；DB Complete side-effect 失败后重新变成/保持 unknown，绝不自动二次外发。
- 两 action 任一普通错误不阻止另一个被尝试，但 durable error 汇总后 inbox 不 Complete。
- wakeup 失败后轮询 worker 仍能 claim 并完成。

### 9.3 真数据库与兼容性

- MariaDB 10.6 与 MySQL 5.7 都从真实 migration registry 执行到 0176；验证 up、并发、死锁重试边界、down 后 0174 数据/约束仍在。
- 使用隔离随机数据库；缺 DSN 必须报 SKIP，不能算 PASS。
- 对 store/worker 跑聚焦 `go test`；Linux/CGO 可用时跑 `go test -race`。Windows `CGO_ENABLED=0` 必须准确 SKIP。
- migration 静态门禁拒绝 MySQL 8 专属语法；真实 5.7 再验证 DDL、datetime(6)、复合外键和 rollback。

这些测试只证明本地 durable 状态机、事务、租户隔离和 fake 调用次数；不证明真实企业微信已发送、未发送或支持幂等。

## 10. 实施顺序与自审

1. 先写 migration/route/store/worker 的失败测试并保存 RED。
2. 先补 worker Begin/Complete 的 lease/fence 原子校验，关闭恢复与旧 worker 的竞态。
3. 实现 0176、scope read model 和 reconcile 事务。
4. 接入 company profile 路由、RBAC、wakeup，再跑 fake/并发/故障注入。
5. 最后跑 MariaDB/MySQL 5.7、race 与全仓门禁；无真实 Provider 验证必须列为边界。

自审结论：该设计没有把 `unknown` 猜成成功或失败，没有让管理 API 调用 Provider，没有通过客户端 tenant/corp 选作用域；command、审计和 inbox 复活原子，worker 与管理命令共享一致锁序和 lease fence。残余不可消除边界是人工证据真实性：若操作员错误确认“未发送”，且 Provider 不支持查询/幂等，重试仍可能产生重复。因此生产 UI 必须对 `confirm_not_sent_and_retry` 显示不可逆重复风险并强制证据字段，审计必须长期保留。
