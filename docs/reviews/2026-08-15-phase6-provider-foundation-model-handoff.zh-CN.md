# Phase 6 Provider 基础与企业微信能力收口——模型交接文档

> 交接日期：2026-08-15（Asia/Shanghai）
> 交接对象：后续负责继续实现、代码审阅、真实测试、合并与部署的模型
> 当前状态：已批准基础停在 `a406110`；“客户精准群发 durable 闭环”存在未提交草稿，尚未达到提交或产品完成条件
> 重要原则：本文区分“已独立批准的提交”“开发线程局部测试通过”“尚未验证草稿”。后两者不得冒充最终完成。

## 1. 一句话接手说明

进入既有隔离工作树 `D:\workspace\mochat-go\mochat-go\.worktrees\phase6-provider-foundation-wecom-closeout`，不要新建分支、不要 reset/clean、不要覆盖未提交文件；先从当前 dirty 草稿继续完成 `contact_batch_send`，补齐本文列出的 P0/P1 与真实 MariaDB/fake HTTP/UI 门禁，独立审阅通过并提交后，再单独实现 `room_batch_send`，最后完成 Provider 门禁、全量验收、合入 `main` 和经用户授权的 Docker/服务器部署。

## 2. 用户目标与协作方式

### 2.1 用户的最终目标

1. 建立真实、可审计、可扩展的 Provider 注册与状态治理，禁止“代码存在/worker running/fixture 通过”被描述为外部 Provider 已 ready。
2. 企业微信会话存档在未购买或未接入真实 `getchatdata` 时必须 truthful limited/unavailable；模拟数据源和真实数据源严格隔离，未来启用真实付费存档时不改 Dashboard、权限和报表调用方。
3. 不依赖付费会话存档的标准企微能力应分别闭环：员工、部门、客户、标签、客户群、联系我、欢迎语、转接、应用消息、客户精准群发、群聊群发、回调等。
4. 精准群发使用 0139 durable operation/dispatch/result 权威账本，具备事务、幂等、lease fencing、reconcile、结果聚合、租户/企业/员工数据范围和稳定 machine code。
5. Dashboard 就近展示 loading/error/empty/结果，不把报错堆到页面顶部；mutation 必须先 `ConfirmAction`；390px 可操作。
6. 所有不依赖外部账号的代码、合同、UI、集成测试与门禁应先完成；缺少真实企微付费凭据只允许标记 `SKIP`/`limited`，不能因此停止其余工作。

### 2.2 用户的工作偏好

- 用户只看最终结果，不希望协调中间步骤。
- 主模型负责设计审批、代码审阅和最终验收；开发可在独立线程执行。
- 任何用户需要审阅/确认的文档必须用中文。
- 采用 TDD RED→GREEN；逻辑批次独立提交；不改写已审历史。
- 不把某一页、某一条路由、某个字符串门禁、mock 或单元测试称作整项产品完成。
- 在最终部署前，先完成代码级和真实临时数据库验收，再由主模型审阅。

### 2.3 当前开发线程

- 线程标题：`Provider 基础与企微能力收口开发`
- 线程 ID：`019fff2c-5b3b-7d90-b9e0-981154a31288`
- Host：`local`
- 模型：用户明确要求该开发线程使用 `gpt-5.6-luna`
- 当前状态：已按交接要求在安全点暂停，idle；没有提交当前 contact batch 草稿。
- 继续方法：可向该线程发送后续任务；若换新模型直接接管，也必须先阅读本文和三份实施文档。

## 3. 仓库、工作树、分支与基线

### 3.1 路径

- 真实 Git 根：`D:\workspace\mochat-go\mochat-go`
- 当前功能工作树：`D:\workspace\mochat-go\mochat-go\.worktrees\phase6-provider-foundation-wecom-closeout`
- 当前分支：`phase6/provider-foundation-wecom-closeout`
- Git common dir：`D:\workspace\mochat-go\mochat-go\.git`
- 该工作树 Git dir：`D:\workspace\mochat-go\mochat-go\.git\worktrees\phase6-provider-foundation-wecom-closeout`

### 3.2 精确 Git 状态（交接快照）

- `main` / `origin/main`：`214d95b77102bd9e245d57a4ebf521b3f5475cd6`
- 当前已提交 HEAD：`a406110998affdd43da961a800db84bcacbed49f`
- merge-base：`214d95b77102bd9e245d57a4ebf521b3f5475cd6`
- 相对 `main`：`0 behind / 66 ahead`
- index：未 stage
- working tree：dirty，包含 contact batch 草稿和本交接文档
- 当前草稿没有 commit SHA；禁止把 `a406110` 说成 contact batch 完成提交。

### 3.3 首次接手必须执行的只读命令

```powershell
cd D:\workspace\mochat-go\mochat-go\.worktrees\phase6-provider-foundation-wecom-closeout
git rev-parse --show-toplevel
git branch --show-current
git rev-parse HEAD
git status --short --branch
git diff --name-status
git diff --stat
git diff --check
```

预期分支必须是 `phase6/provider-foundation-wecom-closeout`。如果不是，立即停止，不要在主工作区写入。

## 4. 强制安全边界

### 4.1 Git 与文件

- 主工作区有用户和其他任务的 dirty 文件；不得 `git reset --hard`、`git clean`、`git checkout --` 或覆盖。
- 所有本阶段写入只允许发生在本工作树。
- 不要重新创建分支或 worktree；当前现场就是要继续的工作现场。
- 保留所有当前未提交文件；不要删除“看似临时”的新文件，它们是正在实现的 contact batch 草稿。
- 使用 `apply_patch` 做人工代码修改；批量格式化仅限定于明确目标文件。
- Git 已提示若再次触碰若干文件可能发生 LF→CRLF；不要机械重写整文件，提交前用 `git diff --word-diff`、`git diff --check` 和 UTF-8 检查避免中文/换行污染。

### 4.2 Docker、服务器和数据库

当前开发批次禁止：

- 操作 Docker/Compose、重建容器或服务；
- 操作服务器 `139.196.34.133`；
- 写业务数据库；
- 删除/重建 named volumes；
- `down -v`、`volume rm`、`system prune`；
- 调用真实外部企微接口；
- 向日志、文档、命令输出写入 secret、token、RSA 私钥、数据库密码或原始外部响应 body。

允许的数据库验证仅限：主模型从本机 MariaDB 管理 DSN 创建具名临时 schema，测试自己建 fixture、跑真实迁移，结束后 DROP 临时 schema并确认 leftovers=0。不得要求生产 fixture ID，不得清理非 fixture 数据。

### 4.3 产品真实性

- `worker running`、`failure_count=0` 不证明同步成功。
- fake HTTP 只证明协议合同，不证明真实企微账号可用。
- 无真实付费会话存档时，archive 必须保持 limited/unavailable。
- 旧 `background_task`、旧 `send_status`、旧 cron 回写不能证明新 durable 群发外部完成。
- route/test/gate PASS 不等于页面和真实产品验收完成。

## 5. 必读文档与源码入口

### 5.1 实施文档

1. 总体 Provider 实施计划：`docs/superpowers/plans/2026-08-14-provider-foundation-wecom-closeout.md`
2. 0139 与精准群发计划：`docs/superpowers/plans/2026-08-14-wecom-standard-capability-ledger-plan.md`
3. 当前 contact batch 子计划：`docs/superpowers/plans/2026-08-15-contact-batch-dispatch.md`（未提交）

### 5.2 过程技能

接手后完整阅读并遵循：

- `using-superpowers`
- `brainstorming`（新增功能/行为前）
- `executing-plans`
- `test-driven-development`
- `systematic-debugging`（出现失败/异常时）
- `receiving-code-review`
- `requesting-code-review`
- `verification-before-completion`

### 5.3 关键源码

已批准基础：

- `internal/modules/providers/`
- `internal/modules/providers/catalog/`
- `internal/modules/providers/archive/`
- `internal/companyprofile/provider_status.go`
- `internal/providerstatus/`
- `internal/wecomcapability/`
- `internal/store/wecom_capability_ledger*.go`
- `internal/store/wecom_capability_dispatch*.go`
- `deploy/standalone/migrations/0138_archive_source_sync.*.sql`
- `deploy/standalone/migrations/0139_wecom_capability_operations.*.sql`
- `scripts/check_provider_completion.mjs`
- `scripts/provider_completion_ast.go`
- `scripts/productionbuild/`

当前 contact batch 草稿：

- `internal/dashboard/contact_batch_dispatch.go`
- `internal/dashboard/contact_batch_dispatch_runner.go`
- `internal/dashboard/contact_batch_dispatch_worker.go`
- `internal/dashboard/contact_batch_dispatch_test.go`
- `internal/dashboard/contact_message_batch_send.go`
- `internal/dashboard/room_welcome_wecom.go`
- `internal/store/contact_batch_dispatch.go`
- `internal/store/contact_batch_dispatch_read.go`
- `internal/store/contact_batch_dispatch_runtime.go`
- `internal/store/contact_batch_dispatch_worker.go`
- `internal/store/contact_batch_dispatch_contract_test.go`
- `cmd/mochat-go/main.go`
- `web/apps/dashboard/src/features/phase34/content-reach-pages.tsx`
- `web/apps/dashboard/src/features/phase34/content-reach-pages.test.tsx`

## 6. 已批准的提交基线

### 6.1 Provider 注册、状态与 UI

从 `a277a8a` 开始建立 Provider 分类注册、状态、Dashboard 只读投影、源码级 completion gate、真实 production composition 和错误边界。后续多轮审阅修复了：

- 注释/字符串自证；
- 不可达 registration；
- build-tag 与 Linux production build set 漂移；
- `ReleaseTags`/`ToolTags` 丢失；
- 只有 registration、没有真实 active implementation 的反向证据缺口；
- archive 假 ready；
- optional runtime 对 external 状态不 fail closed；
- secret/config 泄漏风险。

### 6.2 Archive 可替换来源与 durable sync

最终关键提交：`018116a fix provider completion AST build contract`。

独立只读审阅已批准，无 Critical/Important。已确认：

- Provider gate 固定 `linux/amd64` production build context，同时继承当前 Go toolchain 的 release/tool tags；
- active implementation evidence 使用真实 Go AST，将 package+receiver 的 `Kind`/`Status` 绑定；
- real/simulated archive source 明确区分；
- cursor progress、消息+source 同事务、run tenant/corp/source/namespace 复合约束、lease fencing、legacy simulated 隔离、cleanup/reapply 均有实现和测试；
- CLI 模拟开关必须显式启用；
- external archive 无真实 getchatdata 时不会假 ready。

### 6.3 企业微信标准能力与员工同步队列

最终关键提交：`987f34e fix: fence stale employee queue claims`，其后 0139 工作继续叠加。

独立审阅已批准，无 Critical/Important。已确认：

- Redis ack/dead-letter 只有 claim value 等于旧 QueueTicket 时才删除；
- enqueue 对 WRONGTYPE 在 claim 前 fail-fast；追加失败只条件删除本次 claim；
- DB marker 写失败不再返回假 queued；
- ticket 在事务前校验；
- 延迟旧 HTTP/旧 worker 不能覆盖新 ticket；
- worker 可从缺失 marker 恢复并完成；
- capability success evidence 要求真实 provider object/response identity；
- agent_message evidence 与当前应用 AgentID 绑定；
- 多 active application 采用规范 selector。

保留的非阻断 Minor：

- Lua `pcall` 运行时追加失败分支主要由静态合同覆盖；真实 Redis WRONGTYPE 测试命中 TYPE 预检。
- ticket parser 当前用 `uint64`，以后可收窄到 Redis `INCR` 的 `int64` 上限。
- 全局 ticket fence 依赖 Redis counter 持久性；部署层建议 AOF/保留 counter。

### 6.4 0139 capability ledger 与 durable dispatch 基础

0139 迁移关键提交范围：`88938a7` 至 `c85a19e`。durable dispatch 基础关键提交范围：`fd42a09` 至 `a406110`。

最终已批准 HEAD：`a406110998affdd43da961a800db84bcacbed49f`。

独立审阅确认无 Critical/Important。真实 MariaDB 临时 schema 证据由主模型运行：5 个 dispatch 场景 PASS，临时 schema leftovers=0。已确认：

- operation/dispatch/result/audit/event 复合 tenant/corp identity；
- stable idempotency；
- DB NOW lease、attempt/fencing；
- `pending/claimed/submitting/submitted/polling/succeeded/partial_failed/failed/cancelled` 状态边界；
- 不含 Provider identity 的 `submitted` 在事务前拒绝，零写；
- reconcile 审计记录真实 dispatch `from → to`；
- submitted 只 poll，不重新 submit；
- 旧 lease/generation 零写。

## 7. 当前未提交现场

### 7.1 已修改文件

```text
M  cmd/mochat-go/main.go
M  internal/dashboard/contact_message_batch_send.go
M  internal/dashboard/room_welcome_wecom.go
M  internal/store/company_profile.go
M  internal/store/dashboard_tenant_gate.go
M  internal/store/mysql.go
M  web/apps/dashboard/src/features/phase34/content-reach-pages.test.tsx
M  web/apps/dashboard/src/features/phase34/content-reach-pages.tsx
```

### 7.2 新增未跟踪文件

```text
?? docs/superpowers/plans/2026-08-15-contact-batch-dispatch.md
?? docs/reviews/2026-08-15-phase6-provider-foundation-model-handoff.zh-CN.md
?? internal/dashboard/contact_batch_dispatch.go
?? internal/dashboard/contact_batch_dispatch_runner.go
?? internal/dashboard/contact_batch_dispatch_test.go
?? internal/dashboard/contact_batch_dispatch_worker.go
?? internal/store/contact_batch_dispatch.go
?? internal/store/contact_batch_dispatch_contract_test.go
?? internal/store/contact_batch_dispatch_read.go
?? internal/store/contact_batch_dispatch_runtime.go
?? internal/store/contact_batch_dispatch_worker.go
```

除本文外，其余列表是开发线程暂停前的现场。未修改 0139 migration。全部未 stage、未 commit。

### 7.3 当前草稿已经实现但尚未最终验证的内容

后端创建与 worker：

- `/contactMessageBatchSend/store` durable 分支；
- strict JSON，拒绝 tenant/corp/actor/user identity 字段和未知字段；
- 客户目标使用 `{employeeId, contactId}`，由事务内查询解析 `wx_external_userid`，不允许手填 external ID；
- 当前 actor、租户套餐、quota、页面权限、scope、员工/客户归属、credential generation/secret 的事务内重查；
- 业务 batch + operation + dispatch chunks + create audit/event 单事务草稿；
- 旧 cron 与 durable runner 并存，旧 cron 查询排除有 0139 contact operation 的 batch；
- scheduled due 依据业务 batch `send_way/definite_time`；
- sender/poller adapter、复合结果 identity `{employeeID}:{externalID}`；
- poll 遍历全部 task page，有 pending 时不提前终态；
- production composition 新增 contact dispatch runner/cron。

读写路由：

- list/show 对 durable 行投影 operation；无 durable operation 才回退 legacy；
- employee/results 页查询 durable 投影；
- cancel 仅允许安全状态，并使用事务/状态 fencing 草稿；
- reminder 在外呼前创建 `agent_message/send` operation attempt，携带 lease token/attempt/idempotency；
- unknown attempt 返回 reconcile，不重复外呼；已完成 attempt 幂等返回；
- 生产应用消息增加 `enable_duplicate_check=1` 和重复检查间隔；
- 部分失败写 operation 计数并返回 `CONTACT_BATCH_REMINDER_RECONCILE_REQUIRED`。

前端：

- 创建表单改为员工/客户真实 selector，不再要求手填 external ID；
- contact target 使用员工+客户复合键；
- selector 尝试分页拉齐并去重；
- 创建、删除、提醒接入 `ConfirmAction`；
- show/employee/results 加 loading/error/empty/retry；
- 展示 durable operation 状态和结果计数。

以上均是“草稿已存在”，不是“已验收完成”。

## 8. 当前测试证据的真实边界

开发线程在暂停前报告以下命令曾 PASS：

```powershell
go test ./internal/dashboard -run 'TestContactBatchDurableReminder(UnknownAttempt|CompletedAttempt)' -count=1
go test ./internal/store -run 'TestContactBatch|TestDispatch' -count=1
go test ./internal/dashboard ./internal/store ./internal/wecomcapability -run 'TestContactBatch|TestContactMessageBatch|TestWorkAgentMessageRequest|TestDispatch|TestCapability' -count=1
cd web/apps/dashboard
pnpm typecheck
git diff --check
```

必须注意：

- 这些是开发线程局部证据，不是主模型独立验收。
- 最后一次 Dashboard typecheck 后又改了前端测试文件。
- 当前前端定向 Vitest 尚未重新运行。
- 当前没有 contact batch 真实 MariaDB integration PASS 证据。
- 当前没有 fake HTTP 完整提交+轮询链路 PASS 证据。
- 当前没有 Dashboard lint/build/full test 或 390px PASS 证据。
- 当前没有 provider/catalog/Phase4 完整门禁 PASS 证据。

## 9. 接手后先修的 P0

### P0-1：durable 业务行丢失任务名称和素材 ID

已由交接主模型只读确认：

- HTTP body 有 `batchTitle`、`mediumId`；
- `contactBatchDispatchBody.BatchTitle` 存在；
- `ContactMessageBatchSendWrite` 没有 `BatchTitle` 字段；
- `contactBatchDispatchInputFromBody` 没有把 title 传入业务 write；
- `insertContactBatchBusinessRowTx` 的 INSERT 未包含 `batch_title` 和 `medium_id`。

后果：新 durable 任务名称和素材选择会静默丢失，列表/详情显示错误或空值。

要求：先补 RED，证明 body 的 `batchTitle`/`mediumId` 持久化并能从 list/show 回读，再最小修复。不要用前端保留状态掩盖后端丢字段。

### P0-2：缺少真实临时 MariaDB contact integration

必须新增测试：

- 只接受 `MOCHAT_GO_MYSQL_INTEGRATION_DSN` 管理 DSN；
- 自建 `mochat_contact_batch_<pid/time>` 临时 schema；
- 建完整 legacy fixture，运行真实迁移到 0139；
- 通过生产 `MySQLStore`/handler/service/runner，而不是平行假表或裸 SQL 模拟；
- 测试结束 DROP schema，主模型查询 leftovers=0。

至少覆盖：

1. 创建业务 row + operation + dispatch + audit/event 原子提交；
2. audit/event 故障整笔回滚；
3. duplicate idempotency 返回相同 operation/batch，不重复 dispatch/audit；
4. 跨 tenant/corp、停用 actor、权限撤销、scope 收窄、目标归属变化时零外呼/零越权写；
5. scheduled 尚未到期不 due，到期后 due；
6. legacy batch 仍由 legacy cron 处理，durable batch 不进入 legacy cron；
7. 多 chunk、partial、success、failure 聚合；
8. cancel 与 worker claim 竞争的 fencing；
9. reminder attempt/lease/generation/部分失败/reconcile；
10. 所有测试 fixture 清理完整。

### P0-3：fake HTTP 尚未覆盖生产全链路

使用本地 `httptest.Server`，禁止真实企微调用。覆盖：

- `gettoken`
- `externalcontact/add_msg_template`
- `externalcontact/get_groupmsg_task`
- `externalcontact/get_groupmsg_send_result`

锁定真实官方合同：

- `add_msg_template` 请求包含 `chat_type=single`、`external_userid`、`sender`、`text/attachments`；
- `msgid` 是响应字段，不得作为自定义请求幂等字段发送；
- 本地 idempotency/chunk identity 与企微 response `msgid` 分离；
- submit timeout/模糊 429/5xx 在无法证明未受理时进入 `submitting/reconcile`，禁止重发；
- 401/403/contract error 终态；
- callback/poll 重复结果幂等；
- task/result 多页必须完整轮询。

### P0-4：reminder 部分失败的恢复语义没有真实证据

当前草稿按整次 reminder operation 记录 success/failure totals，部分失败后返回 reconcile；同幂等键不再整批重发。这是安全方向，但必须证明：

- 已成功收件人不会被自动再次外呼；
- 进程在外部接受后、ledger record 前崩溃时返回 reconcile，不能直接重发；
- 若未来支持“只重试失败收件人”，必须增加逐收件人持久 attempt/result fence；在此之前不得假装可安全 retry；
- provider duplicate check 只能作为辅助，不能替代本地 ledger/fence。

### P0-5：UI 行为和 390px 尚未验收

补真实 access API 分支测试：

- 第一次点击创建/删除/提醒只弹确认，不 mutation；确认后才发送 payload；
- create payload 包含 title、mediumId、employee/contact pairs、scope 和稳定 idempotency key；
- selector loading/error/empty/retry；
- 多页员工/客户选项完整，不能静默截断；
- show/employee/results loading/error/empty/retry；
- durable/legacy 状态分别显示；
- 409/reconcile 保留上下文，不清表单；
- viewport=390，关键按钮、selector、详情表格可达，无 document 级横向溢出。

## 10. 继续审查的 P1

1. `Index` 当前可能逐条调用 `ContactBatchDurableView`，形成 N+1；在正确性通过后评估批量 aggregate query，不能牺牲 tenant/corp isolation。
2. 前端 `/workContact/index` 将多个 employee IDs 作为逗号字符串传 `employeeId`；核对真实 handler 合同是否支持，否则改为逐员工分页/后端明确多值合同，不得依赖测试 mock。
3. selector 的 unknown pagination metadata 路径必须保证最终停止，不能无限循环或因重复 key 静默丢选项。
4. `sendWay=2` 应验证时间格式和产品要求的未来时间；过去时间是否立即发送必须有明确合同与测试。
5. durable read 回退 legacy 只能发生在“确实没有 durable operation”时；权限、tenant、scope、SQL 错误不得 fallback。
6. 所有动态 `{id}` 查询必须先 tenant+corp+id；跨租户统一 404，零写。
7. cancel 必须锁 operation/dispatch 并按 rows affected fence；不能删除 submitted/terminal 或覆盖 worker 状态。
8. reminder 不能在返回给前端的 payload/audit 中暴露 secret、token、原始 response。
9. `ContactBatchDispatchInput.Content` 和 WeCom builder 的 msgType/字段兼容必须一一测试；unsupported content fail closed。
10. production composition 同时运行 legacy 与 durable cron 时，必须证明不会双发；runner 配置开关、启动日志和 shutdown 行为需测试。
11. 普通用户/超管都必须经过 tenant gate；普通用户需要精确页面权限和数据范围，superadmin 只在同 tenant 隐式全权限。
12. quota 超限应使用业务 409 machine code，不能误清 session；租户门槛才使用 `TENANT_ACCESS_DENIED`。

## 11. 建议的继续执行顺序

### 阶段 A：收口 contact batch 草稿

1. 先运行当前定向 tests，记录真实失败，不要先改实现。
2. 为 P0-1 title/medium 丢失补 RED→GREEN。
3. 完成 fake HTTP submit+poll 全链路 RED→GREEN。
4. 完成临时 MariaDB integration harness 和 P0 场景。
5. 完成 reminder crash/partial/reconcile 测试。
6. 完成 UI 定向测试、390px、typecheck/lint/test/build。
7. 跑 Provider/Phase4 门禁和 `git diff --check`。
8. 自审所有 dirty 文件，删除 dead code、调试输出和类型逃逸。
9. 形成一个独立 contact batch commit；不要混入 room batch。
10. 请求独立只读审阅；对 Critical/Important 用后续独立 fix commit，不改写历史。

### 阶段 B：单独实现 room batch

只有 contact commit 独立审阅通过后开始。必须：

- 独立 `DispatchKindRoomBatch`；
- 群主/群聊归属和 scope 校验；
- room credential/generation；
- fake HTTP submit/poll；
- 多 chunk/partial/reconcile/lease/generation；
- 不能以 contact 结果为 room ready 证据；
- 独立 commit、真实 MariaDB、独立审阅。

### 阶段 C：页面、门禁与剩余 Provider

按原始委托继续盘点并 truthful 收口：

- 群活码：独立活码与扫码统计合同，不能拿自动拉群列表冒充；
- redirect-link：本地短链与外部授权状态分离；
- 企业微信客服：可注入真实同步 Provider，无权限/凭据保持 limited；
- group-template：真实员工/群主/群聊选择和归属；
- friends-circle：草稿/任务与外部发布 Provider 分离；无法真实发布时保持 limited；
- 对象存储：保留本地文件/音频，必要时实现 S3-compatible adapter，不能破坏现有卷；
- AI OpenAI-compatible：代码支持、部署可关闭，不发送测试请求、不消耗 API；
- 支付、域名、短信、邮件：仅 truthful limited/unavailable，不造假 Provider。

### 阶段 D：全量验收、合并与部署

1. 全部提交审阅通过、工作树 clean 后，才讨论合入 `main`。
2. 合并前核对主工作区 dirty 文件，禁止覆盖用户改动。
3. 本地 Docker 只在用户授权后进行；默认只重建 app，保留 MySQL/Redis 和四个 named volumes。
4. 服务器 `/opt/mochat-go` 没有 `.git`；按本地构建产物部署，不在服务器编译；通过文件 hash、容器时间、health/log/API/browser 验证。
5. 不在文档/日志输出服务器密码或数据库 secret。
6. 最终验收必须区分：源码门禁、临时数据库、local Docker、服务器 smoke、真实外部企微凭据/live；缺一项就如实标注。

## 12. 推荐验证命令

### 12.1 Contact batch 定向

```powershell
go test ./internal/dashboard ./internal/store ./internal/wecomcapability -run 'TestContactBatch|TestContactMessageBatch|TestWorkAgentMessageRequest|TestDispatch|TestCapability' -count=1
```

新增真实 integration 后，使用管理 DSN运行；命令本身不得打印 secret：

```powershell
$env:MOCHAT_GO_MYSQL_INTEGRATION_DSN = '<从本机 MariaDB 容器环境安全构造，不打印>'
try {
  go test ./internal/store ./internal/migration -run 'TestContactBatch.*Integration|TestWeComCapability.*Integration' -count=1 -v
} finally {
  Remove-Item Env:MOCHAT_GO_MYSQL_INTEGRATION_DSN -ErrorAction SilentlyContinue
}
```

测试必须自己创建/删除临时 schema，运行后再用只读 SQL 验证 `mochat_contact_batch_%` leftovers=0。

### 12.2 Dashboard

```powershell
corepack pnpm --filter @mochat/dashboard typecheck
corepack pnpm --filter @mochat/dashboard lint
corepack pnpm --filter @mochat/dashboard test
corepack pnpm --filter @mochat/dashboard build
```

本机历史上 Vitest 在某些 Codex PTY 会挂住；如果出现低 CPU 长时间无输出，先 Ctrl+C 当前会话，不要杀其他 Node 进程。需要主模型用标准命令独立复核，不能把挂起当 PASS。

### 12.3 Provider 与 Dashboard 门禁

```powershell
corepack pnpm check:provider-completion
corepack pnpm check:phase4-dashboard-page-rbac
git diff --check
git status --short --branch
```

有 DSN 的最终 real gate：

```powershell
corepack pnpm check:provider-completion-real
```

`check:provider-completion-real` 必须拒绝 integration SKIP；普通 gate 中出现明确 SKIP 不能被报告成 real PASS。

### 12.4 全量 Go

```powershell
go test ./... -count=1
```

若失败，先确认是本分支引入还是既有 baseline；不得用“无关失败”绕过 changed-package failure。

## 13. Machine code 与状态合同

当前 contact 草稿涉及的稳定错误类别应保持：

- `TENANT_ACCESS_DENIED`：403，租户/SaaS 门槛失败；前端可清 session。
- `DASHBOARD_PERMISSION_DENIED`：403，页面权限失败；保留 session。
- `CONTACT_BATCH_SCOPE_DENIED` 或既定同义 code：403，员工/客户数据范围失败；保留 session。
- `CONTACT_BATCH_QUOTA_EXCEEDED`：409，额度不足；不能映射为租户门槛。
- `CONTACT_BATCH_CAPABILITY_LIMITED`：409/既定合同，缺 credential/runtime；不能假 ready。
- `CONTACT_BATCH_CONFLICT`：409，状态/version/idempotency 冲突。
- `CONTACT_BATCH_NOT_FOUND`：404，当前 tenant/corp 下不存在，包括跨租户目标。
- `CONTACT_BATCH_REMINDER_RECONCILE_REQUIRED`：503，外部可能已接受但本地无法安全判定，禁止重发。

接手模型必须以现有实现/测试为准核对最终拼写和 HTTP status，不要靠中文 message 判断。

## 14. 数据与事务不变量

1. tenant/corp/actor 只来自认证 `DashboardPrincipal`/context，body 值一律拒绝或忽略；当前设计选择严格拒绝。
2. 所有目标查询使用 tenant/corp+id；跨租户返回 404，零 mutation。
3. create 的 business row、operation、dispatch、audit/event 同一 `sql.Tx`。
4. 每个 chunk 一条 dispatch，stable idempotency；重复 HTTP 只回读既有 operation。
5. dispatch claim/reconcile/complete/fail 都必须检查 tenant/corp、lease token、attempt、credential generation 和 DB NOW lease。
6. submitted/reconcile 只能 poll，不能再次 submit。
7. 外部调用前重查 tenant package/quota/page permission/scope/ownership/credential generation。
8. result identity 必须区分同一客户由不同员工触达的情况，当前草稿使用 `{employeeID}:{externalID}`。
9. COUNT 与 items 使用相同 SQL scope/filter；禁止先全企业分页再 handler 后过滤。
10. audit/event append 失败时，尚未外呼的事务必须整体回滚；外呼后无法持久化只能进入 reconcile。

## 15. 前端不变量

- API 使用 Dashboard `ApiClient` 的相对 `/contactMessageBatchSend/...` 路径，不写 `/dashboard` 双前缀。
- 不提供 external_userid 手填入口。
- 员工、客户选项来自真实 API，且必须受当前 tenant/corp/scope 限制。
- 选择多个员工时，客户必须保留所属员工 identity，不能做笛卡尔发送。
- 创建、删除、提醒先展示变更摘要并经过 `ConfirmAction`。
- tenant gate 403 与 page/scope 403 行为不同，不能统一清 session。
- loading/error/empty/retry 就近呈现；错误醒目但不堆页面顶部。
- 390px 下关键操作可达、表格有局部横向滚动、document 无横溢。

## 16. 本地 Docker 与服务器的后续边界

这些仅供最终阶段使用，当前禁止执行。

### 16.1 本地

- Compose project：`mochat-go-desktop`
- Dashboard 常用地址：`http://localhost:18080`
- SaaS 常用端口：`18081`
- 只重建 `app`；MySQL/Redis 不重建。
- 先后记录 app/mysql/redis container ID 与四个卷；卷默认必须不变。
- 先前容器 ID 只是历史快照，可能已过期，必须重新 inspect。

### 16.2 服务器

- 主机：`139.196.34.133`
- 项目目录：`/opt/mochat-go`
- 服务器没有 `.git`，不能用 remote git status 证明部署版本。
- 用户曾明确要求服务器不编译；本地构建后传产物。
- 验证使用 hash、容器创建时间、health、日志、认证 API、Provider 状态和浏览器。
- 历史卷名：`standalone_app-storage`、`standalone_audit-anchor-storage`、`standalone_mysql-data`、`standalone_redis-data`；属于历史服务器快照，部署前必须重新读取并保留。
- MariaDB 容器变量使用 `MARIADB_ROOT_PASSWORD`/`MARIADB_DATABASE`，不是 `MYSQL_ROOT_PASSWORD`。

## 17. 禁止的错误做法

- 不要删除当前 dirty 草稿重新实现。
- 不要把 current HEAD `a406110` 当 contact batch commit。
- 不要因为定向 Go test PASS 就提交；当前 P0 仍存在。
- 不要要求生产 fixture IDs 或写业务数据库做 integration。
- 不要用假的 `rbac_it_*` 平行表模拟生产 schema。
- 不要把 external `msgid` 当请求幂等字段。
- 不要 submit timeout 后直接重试。
- 不要让 contact batch 成功背书 room batch。
- 不要用注释、字符串、fixture、manifest 自证 Provider ready。
- 不要把 `StateReady` 设为配置存在即 ready；必须有 runtime/operation evidence。
- 不要记录 secret/token/密码。
- 不要操作 Docker/服务器，直到所有代码批次通过独立审阅且主任务明确进入部署阶段。

## 18. 交付与汇报模板

每一批提交后，向主模型报告：

```text
批次：contact batch / room batch / provider gate / deployment
分支：phase6/provider-foundation-wecom-closeout
提交 SHA：<full sha>
与基线范围：<base>..<head>
RED 证据：<命令 + 失败原因>
GREEN 证据：<命令 + tests 数量 + exit code>
MariaDB：<真实临时 schema tests；SKIP 或 PASS；leftovers=0>
Dashboard：typecheck/lint/test/build
门禁：provider completion / phase4 / diff-check
外部 live：未调用 / SKIP / 已授权真实证据
Docker/服务器：未操作，或列出明确授权和前后容器/卷证据
重叠文件：尤其 cmd/mochat-go/main.go、compose、Dockerfile
未完成/受限：逐项列出，不使用“全部完成”泛化
工作树：clean / dirty（列文件）
```

独立审阅必须给出 Critical/Important/Minor；Critical/Important 全部关闭后才能进入下一批。

## 19. 提交历史索引

完整精确历史请运行：

```powershell
git log --oneline --decorate --no-merges main..HEAD
```

里程碑索引：

- `a277a8a`：Provider foundation 设计
- `294f3ba`：classified status registry
- `53142ee`：WeCom archive/standard sync contract
- `0bc9e93`：scoped runtime status
- `018116a`：Provider completion AST production build contract，archive 阶段终审批准
- `987f34e`：员工同步队列 stale claim fencing，队列阶段终审批准
- `7012c63`、`012da0a`：0139/精准群发设计与边界文档
- `88938a7`：0139 capability ledger migration/generation
- `fd42a09`：durable dispatch foundation
- `a406110`：submitted identity/audit transition 修复，durable dispatch 阶段终审批准
- 当前 contact batch：未提交，无 SHA

## 20. 最终接手检查清单

- [ ] 位于正确 worktree 和 branch。
- [ ] 已读三份实施文档和本文。
- [ ] 已确认 main dirty，未触碰主工作区。
- [ ] 已保留全部 contact 草稿。
- [ ] 已先复现并修复 title/medium 丢失 P0。
- [ ] 已完成 fake HTTP submit+poll 全链路。
- [ ] 已完成真实临时 MariaDB contact integration，leftovers=0。
- [ ] 已证明部分 reminder 不会重发成功收件人。
- [ ] 已完成 UI ConfirmAction、状态、390px 和真实 selector tests。
- [ ] 已跑 Go/Dashboard/provider/phase4/diff gates。
- [ ] 已形成独立 contact commit 并完成只读审阅。
- [ ] contact 批次批准后才开始 room 批次。
- [ ] 所有代码批次批准后才合入 main。
- [ ] 用户明确进入部署阶段后才操作 Docker/服务器，并保留卷、保护数据、隐藏 secret。

---

本文是交接快照，不是完成报告。任何后续模型都必须用新鲜命令和真实输出重新验证当前状态。
