# Task 11 实施报告：callback side-effect unknown 受控恢复

## 1. 基线与范围

- 实施基线：`64660887785c681df22d843150d23936e47c4005`。
- 唯一工作区：`D:/workspace/mochat-go/mochat-go/.worktrees/p0-local-closure-20260829`。
- Task 10 已占用 `0175_contact_batch_title`，本任务使用 `0176_wework_callback_side_effect_reconciliation`。
- 未修改 `docs/PROJECT_PROGRESS.zh-CN.md`，未调用真实企业微信、真实 AI Provider 或生产服务器，未删除 Docker 命名卷。

## 2. RED 与根因

首轮 RED 命令：

```text
go test ./internal/companyprofile ./internal/migration -run "CallbackSideEffectRecovery|CallbackSideEffectReconcile|WeWorkCallbackSideEffectReconciliation0176|DefaultMigrationsLatestIsCallback" -count=1
```

失败证据：companyprofile 缺少 unknown 列表/详情/reconcile 类型和 Service 方法；`0176` up/down 文件不存在；真实 registry 最新仍为 `0175_contact_batch_title`。路由 RED 为 `GET /dashboard/company/callback-side-effects status=502, want 204`。DB 故障语义 RED 为 HTTP 500 `INTERNAL_ERROR`，而合同要求 503 `CALLBACK_RECOVERY_UNAVAILABLE`。

根因不是 Provider 能力，而是 `0174` 只建立了 worker 侧 `pending → unknown → sent` fail-closed intent：

1. 没有按 Dashboard principal/RBAC 暴露的恢复读模型和命令入口；
2. 没有 action version、隔离期、operator decision、command receipt 或请求 fingerprint；
3. worker 的 side-effect Begin/Complete 只按 tenant/corp/event/action 更新，没有同时锁定并校验 inbox lease token/fence；
4. `unknown` 的两个 action 没有“最后一个 unknown 才复活 inbox”的事务合同；
5. DB 故障会落入通用 500，运维无法区分可重试的恢复依赖不可用。

首次 MariaDB migration targeted 还暴露一项夹具根因：package 内旧 helper 直接跑过受控 `0130`，没有生成 staging evidence，报 `controlled migration 0130 ... is pending`。修复为复用 Task 10 的外部真实 registry + controlled evidence harness；没有插 ledger 或缩小迁移前缀。

最终 RBAC catalog 门禁还暴露了候选基线中的陈旧合同：`126b7a01` 已把 public/identity/SaaS 认证分类迁入 typed route registry，但 catalog 脚本仍读取已删除的 `exactExemptDashboardRouteContracts`，迁移覆盖链也停在 0168。新增 RED 证明两点后，门禁改为从 typed registry 派生 exact exemptions、把 0176 作为普通 permission overlay，并从运行时 switch 中提取动态路径模板；没有恢复旧豁免表，也没有把静态 registry 冒充运行时接线。

## 3. 实施与决策理由

- `0176` 为 side-effect intent 增加 version、reconciliation fence、unknown/reconcile 时间和最后人工决议信息；新增提交后不可变的 command receipt 表，唯一键为 `(tenant_id, corp_id, request_id)`。事务内先写 reservation、提交前再固化首次结果，这样跨事件同请求也先串行，同 fingerprint 可重放首次结果，同 key 异义请求稳定 409。
- 列表、详情和 reconcile 都只接收 `DashboardPrincipal`，Store SQL 固定使用 principal tenant/corp。body 使用严格 JSON 解码，tenant/corp/actor 注入在 Store 前被拒绝；跨 scope 与 scope 内不存在统一 404。
- `confirm_sent` 只把目标 action 标为 `sent`；`confirm_not_sent_and_retry` 只重置为 `pending`。管理 API 没有 Provider 依赖或调用点，外发只能由 durable worker 后续执行。
- reconcile 固定锁序为 actor → active tenant/corp binding 与 corp → command receipt reservation → inbox → 同事件 actions（`action_key ASC`），再在单事务内提交 action、inbox fence/复活、Dashboard audit 和 receipt 首次结果。receipt 不使用 action 外键，避免 reservation 隐式提前锁 action；任何 SQL/commit 失败整体回滚。
- 只有最后一个 `unknown` 被解决才把 inbox 复活为 `pending`、attempt 清零并递增 lease fence。另一个 action 仍 unknown 时不修改 inbox；未知未来 action fail closed。
- worker Begin/Complete 改为携带完整 `WeWorkCallbackExecution`，先锁 inbox 并验证 `processing + lease_token + lease_fence + lease_expires_at`，再锁 action；旧 worker 的 Begin/Complete 与既有 CompleteInbox 都会 lease lost。
- `reconcile_after` 为 worker Begin 后 15 分钟。活动 lease、隔离期、version/fence/state/未知 action 均返回稳定 409；DB/事务故障规范化为 503。
- 列表使用 `unknown_at,event_key,action_key` 的降序 keyset cursor，只返回摘要与泛化错误码，不返回 `event_json`、正文、token 或 secret。

## 4. 测试证据

### RED

- 缺类型/方法、缺 0176、latest=0175：FAIL（符合预期）。
- Server 新路由未接线：FAIL 502（符合预期）。
- DB 故障返回 500：FAIL（符合预期）。
- 决议与证据类型不匹配仍进入 Store、cursor 接受尾随 JSON、审计未标出 action：三项提交前自审 RED 均先复现后修复。
- MySQL 5.7 首轮审计断言因其 JSON 文本会插入空格而 FAIL；实际字段存在。根因修复为解析 JSON 后按字段断言，同一完整 targeted 随后 PASS，未放宽字段值要求。
- 对 `4a418811` 的复审 RED：Service 缺 wakeup 注入与新响应字段导致编译失败；0176 contract 明确报缺 `reservation_token`、`remaining_unknown_actions` 且仍有 action FK；cursor 接受大写 event key。
- reservation 实现后的首次 MariaDB RED 为四个 recovery fixture 全部 `TENANT_ACCESS_DENIED`。根因是旧专用 seed 把 binding 建成 pending，而生产合同要求事务内 active；修复仅把 recovery 场景 seed 激活，并新增 suspended binding/stale auth version 的 fail-closed 断言，没有放宽 Store。
- 独立 diff 复审继续发现 wakeup 只有 Redis `Publish` 而全仓没有 subscriber，`wakeupAccepted=true` 会成为伪成功；另有 HTTP 唤醒无时间上界。修复为最多保留 64 个 list token、worker `BRPOP` 消费并在 Redis 故障时退回 MySQL poll，Service 使用 50ms 独立超时。

### PASS：无外部依赖

- `go test ./internal/companyprofile ./internal/dashboard ./internal/store ./internal/migration ./cmd/mochat-go -count=1`：五包 PASS。
- `go vet ./...`：PASS。
- `node --test scripts/check_dashboard_page_rbac_catalog.test.mjs`：43/43 PASS。
- `node scripts/check_dashboard_page_rbac_catalog.mjs`：53 pages、49 ordinary、4 superadmin_only、0 unmapped dashboard API usages。
- `git diff --check`
- 本地 Redis 7（隔离 DB 15 + 每次随机测试 key）真实 `LPUSH/LTRIM → BRPOP`：token 从 1 变 0，测试后只精确删除该随机 key；不会触碰生产 wakeup key，未删除命名卷。另有 waiter 故障注入证明 Redis 失败后 worker 回退有界 MySQL poll 并继续完成 claim。
- 覆盖 principal/RBAC 前置拒绝、body scope 注入、Idempotency-Key/decision 校验、路由、503 错误语义、post-commit wakeup 首次/重放语义、worker fake 双 action、unknown 两次 claim 都不自动重放、confirm_sent 外发 0 次、retry 只外发对应 action 1 次。

### PASS：MariaDB 10.6 本地真实数据库

- migration 0176 up/down/reapply（增加 reservation 列断言后的最终复跑）：`ok ... 7.214s`。
- recovery targeted（增加跨事件持久状态断言后的最终复跑）：`ok ... 30.627s`。
- harness 为每个测试使用 `mochat_it_<24 hex>` 隔离 schema，并在 Cleanup 校验精确删除。
- 最终显式查询 `information_schema.schemata`：`mochat_it_%` 残留 0。
- 覆盖稳定分页、同 key 首次结果重放、异 payload 冲突、32 路同 key 并发（1 首次 + 31 idempotent）、跨事件同 request key（1 成功 + 1 fingerprint conflict、无 503，且 loser action/version 未变化、audit/command 各一份）、双 action 独立、最后 unknown 才复活、active binding/auth version、严格 inbox 状态、隔离期、未知 action、跨 scope 404。
- 分别在 action update、inbox update、audit insert、receipt update、commit 注入失败，均验证 action/version、inbox/fence、audit、receipt 全回滚；另保留 reservation insert 失败证据。

### PASS：MySQL 5.7 本地真实数据库

- targeted migration/store：migration 最终复跑 `11.129s`，store 最终复跑 `47.513s`，exit 0。
- harness 同样创建并清理随机隔离 schema；容器和命名卷保持不变。
- 最终显式查询 `information_schema.schemata`：`mochat_it_%` 残留 0。
- 使用相同真实 registry、受控 0130/0165 evidence、断言和测试范围；没有 MySQL 8 专属 SQL。

### SKIP/边界

- 未调用真实企业微信，因而没有真实“已发送/未发送”证据，也不宣称 Provider exactly-once。
- fake worker 只证明：确认 sent 后该 action 不再外发；确认未发送并重置 pending 后由 worker 外发一次；unknown 继续阻止自动二次外发。
- 前端全量、Docker/browser、govulncheck 和生产验证不属于本原子提交的独立证明；这些由最终候选 SHA 的统一门禁阶段给出。

## 5. 自审

复审 `4a4188119415221fc1c44856b07f18c939eba576` 后发现该提交尚未闭合：缺生产 wakeup 接线和响应字段、事务未锁 active binding/corp、异常 inbox 状态过宽、跨事件相同 request key 仍可能在 receipt INSERT 时报 duplicate 并映射 503，且故障注入/fake Provider 证据不足。本次修复按上述 RED 逐项闭合；以下勾选均有本节命令或测试断言支撑。

- [x] 设计中的所有 `0175` 占用歧义已改为 `0176`。
- [x] 无 TODO/TBD；API、错误码、事务锁序和 down 边界明确。
- [x] 管理 API 没有 Provider 调用；人工证据错误造成重复发送的残余风险已在中文 runbook 明示。
- [x] action/receipt/audit/inbox 在同一事务，跨事件 receipt 先序列化且重放不重复审计或 fence；五个事务阶段分别故障注入后均全回滚。
- [x] principal scope 和 Store scope 双重约束，并在事务内锁定 active binding/corp 与复核 actor auth version；未从客户端接受 tenant/corp/actor。
- [x] 0176 up/down 保留 0174 表和约束，MariaDB/MySQL5.7 lifecycle 已验证。
- [x] RBAC catalog 从 typed auth registry、真实 runtime switch 和 0176 migration overlay 三个独立来源闭合，没有恢复已废弃的旧豁免合同。
