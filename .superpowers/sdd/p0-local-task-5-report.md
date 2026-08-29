# P0 Task 5：订单端到端幂等实施报告

## 结论

Task 5 已按 TDD 完成。订单创建现在以 `(tenant_id, corp_id, Idempotency-Key)` 声明一次用户意图；首次请求在一个数据库事务内写入 receipt、订单、创建审计和首次 HTTP 状态码/响应体快照。相同 key、相同规范化 payload 会精确重放首次状态码和响应体；相同 key、不同 payload 返回 409；并发 loser 不返回占位或假成功。

前端在首次提交时为一次用户意图生成 key，网络或响应丢失后的原样重试复用该 key；成功或用户取消填写时清除；失败后编辑业务字段会开始新意图并生成新 key。

## 根因与方案

原链路有四个相互叠加的根因：

1. `order-page.tsx` 重试没有稳定的意图 key，业务 API 也没有请求头透传能力。
2. `OrderHandler` 不读取 `Idempotency-Key`，`NewOrder` 在每次请求中生成新 UUID。
3. SQL repository 将 `order.ID` 错写到 0119 的 `idempotency_key`，因此唯一键无法约束同一用户意图。
4. 事务提交后才由 handler 计算响应，数据库没有首次 HTTP status/body receipt，响应丢失后无法精确重放。

本次增加 0173 receipt 表。winner 先在事务内插入完整 receipt claim，再写订单和审计，全部成功后一起提交；任一步失败均整体回滚。duplicate insert 会等待 winner 的事务结论。为避免多个 loser 在重复键事务中继续 `SELECT ... FOR UPDATE` 产生锁升级死锁，loser 收到重复键后先回滚自己的 claim 事务，再从新的只读事务读取已提交且不可变的完整 receipt。

payload hash 使用固定字段结构和 SHA-256，忽略 JSON 字段顺序，但保留有业务语义的客户端显式订单 ID；tenant/corp 由 receipt 主键隔离。`Idempotency-Key` 与 `order.ID` 分开持久化，订单表原 0119 唯一键继续使用实际请求 key。

独立审查进一步发现两处同一合同内的根因：0123 曾把订单表 key 改为大小写不敏感的 `utf8mb4_unicode_ci`，但 receipt 使用二进制比较，导致 `Key`/`key` 在两个唯一约束中含义不同；duplicate 重放也错误依赖当前订单行仍未软删。0173 最终同步把订单表和 receipt key 改为 `VARBINARY(128)`，让 opaque key 的两个唯一约束按原始字节完全一致；重放只读取不可变 receipt 的 `order_id`、status/body，不再依赖可变订单状态。down 仅移除 receipt 并保留 VARBINARY，因为把已合法存在的字节不同 key 强制合并回字符 collation 会触发 1062，无法无损回滚；旧代码写入 UUID/order.ID，与 VARBINARY 向后兼容。

## TDD 证据

### RED

新增测试后，修复前得到预期失败：

- 缺少 `Idempotency-Key` 仍返回 200，期望 422。
- 相同 key 重试生成不同订单 ID 和不同响应体。
- 相同 key、不同金额仍返回 200，期望 409。
- 32 并发返回不同订单响应，不能收敛到一个订单。
- 0173 up/down 文件不存在。
- 前端没有第四参数请求头、失败重试 key 生命周期和取消清理行为。
- 补充自审 RED：相同 key 仅客户端显式订单 ID 不同时错误地重放 200，期望 409。
- 独立审查 RED：大小写不同 key 的第二次创建因旧唯一键 collation 冲突返回 500；订单软删后相同 key 重试因查询当前订单失败而不能重放。
- 复审 RED：up 后写入同 scope 大小写异 key，再执行恢复 unicode collation 的 down 会因唯一键合并冲突返回 1062。
- 全量 Go RED：新增 0173 后 SaaS system health 的迁移版本/数量仍停留在 0172/172。

### GREEN

- `go test ./... -count=1`：全量通过。
- `pnpm --filter @mochat/dashboard typecheck`：通过。
- `pnpm --dir web/apps/dashboard exec vitest run src/features/phase35/order-page.test.tsx src/features/business-workbench/business-workbench-api.test.ts`：2 个文件、8 个测试通过。
- `git diff --check`：通过。

Vitest 输出中的 `window.getComputedStyle(elt, pseudoElt)` 是 jsdom/Ant Design 已知非致命 stderr；测试退出码为 0，8/8 通过。

## 真实数据库、迁移与故障验证

验证均使用本任务创建的临时容器和 `--tmpfs /var/lib/mysql`，未挂载或访问现有 `mochat-go-desktop-*` 命名卷。测试完成后临时容器和临时下载的 `mysql:5.7` 镜像标签均已删除。

### MariaDB 10.6

最终代码运行：

```text
go test -tags=integration ./internal/modules/scrm/adapters/mysql -run TestOrder -count=3 -v
PASS（3 轮）
```

### MySQL 5.7

最终代码运行同一命令：

```text
go test -tags=integration ./internal/modules/scrm/adapters/mysql -run TestOrder -count=3 -v
PASS（3 轮）
```

每轮覆盖：

- 32 并发相同 key：恰好 1 个 creator、1 个 order、1 个 audit、1 个 receipt，32 个调用返回同一 order ID、同一 status/body。
- 相同 key、不同 payload：`ErrOrderIdempotencyConflict`，HTTP 映射 409。
- 相同 key 跨 tenant/corp：三个作用域分别创建自己的订单、审计和 receipt。
- 大小写、尾随 ASCII 空格、Unicode NBSP 与全角空格不同的 opaque key：在同一 tenant/corp 下分别创建并按原 key 精确重放，receipt 与订单旧唯一键均使用 `VARBINARY(128)` 原始字节等价规则。
- 首次成功后订单状态变化并软删：相同 key/payload 仍仅从不可变 receipt 精确重放首次 status/body。
- 注入创建审计失败：receipt、order、audit 同一事务回滚，均不留下部分状态。
- 0173 up/down 在真实数据库执行，建表后可见、订单 key 为无 collation 的 `VARBINARY(128)`；写入同 scope 的大小写、尾随 ASCII 空格、NBSP、全角空格异 key 后执行 down，receipt 消失、五条订单均保留且 VARBINARY 不变。静态合同同时排除 `SKIP LOCKED`、`RETURNING`、`CREATE INDEX IF NOT EXISTS`、`CHECK (` 等 MySQL 5.7 不兼容语法。

首次真实 MariaDB 32 并发曾稳定复现 `Error 1213 deadlock`，根因是 duplicate loser 在原 claim 事务中继续 `FOR UPDATE`。按上述事务边界修复后，MariaDB 连续 5 轮和最终 3 轮、MySQL 5.7 初次与最终各 3 轮均通过。

## 前端合同与必要的公共 API 改动

`Phase35Api` 复用 `BusinessWorkbenchApi`。原 `write(endpoint, values, method)` 只能固定发送 `Content-Type`，订单页即使生成 key 也无法真正发出 `Idempotency-Key`。因此仅做向后兼容的必要扩展：第四参数增加可选 `headers` 并与默认 JSON header 合并；所有旧调用不传第四参数，运行行为不变。新增 API 单测验证实际请求头透传，没有扩展其他业务能力。

## 自审

- receipt claim、order、created audit、首次 response snapshot 在同一事务中；失败不会留下不完整 receipt。
- duplicate 必须等 winner 的唯一键事务得出结果，随后只读取已提交完整 receipt；不存在 pending 假成功状态。
- response status/body 直接从 receipt 写回，未重新序列化，因此响应丢失后可字节级重放。
- receipt 是重放唯一真源；订单随后变更或软删不会影响首次 HTTP 结果。
- payload hash 不含服务器随机生成的 order ID，但包含客户端显式 ID；相同业务输入即使 JSON 字段顺序不同仍稳定。
- repository 将真实 `Idempotency-Key` 写入订单表，未再用 `order.ID` 冒充。
- receipt 与订单表的 key 都使用 `VARBINARY(128)`；大小写、PAD SPACE 和 Unicode 空白不同的 key 不会在第二道唯一约束中意外合并。
- down 保留 VARBINARY，避免合法 post-up 字节不同 key 数据在回滚时被唯一键无损不可逆地合并。
- 32 并发测试同时断言 creator 数、订单数、审计数、receipt 数和所有响应一致。
- 未修改 `docs/PROJECT_PROGRESS.zh-CN.md`，未访问真实 Provider、生产环境或现有命名卷。

## 未通过门禁与 SKIP 边界

- `pnpm check:phase3-5-dashboard` 未通过，错误为既有清单漂移：`unexpected Phase 3.5 route: /chat/file-audio`。Task 5 未修改路由、page registry 或 Phase 3.5 manifest，因此未越界整改。
- 未做浏览器验收、真实 Provider 调用、生产部署或生产数据验证；这些不属于 Task 5 的本地订单幂等闭环。
- 临时数据库初始化时主迁移 runner 在受控迁移 `0130_identity_realms_single_corp_backfill` 按设计停止；Task 5 的 0173 up/down 由集成测试在隔离影子表上直接执行并通过，不将该受控停止记为 0173 失败。

## 主控复审追加整改（Changes Required）

### 根因复核

主控复审指出 `utf8mb4_bin VARCHAR` 仍受 MySQL 5.7/MariaDB 的 PAD SPACE 规则影响，`Key` 与 `Key ` 可能在唯一键中合并；同时 handler 和 SQL repository 使用 `TrimSpace`，会把 ASCII/Unicode 空白主动规范化。复核确认该 finding 成立。另有两个 API 治理缺口：SQL repository 仍公开 `CreateContext/Create`，允许以 `order.ID`、`actor=0`、无 receipt 的方式创建；payload hash 的 variadic client ID 参数允许调用方在编译期遗漏业务字段。

### RED → GREEN

- RED：handler 依次提交 `opaque-key`、尾随 ASCII 空格、NBSP、全角空格和大小写变体时仅创建 2 个订单，期望 5 个原始字节意图。
- RED：0173 静态迁移合同仍是 `VARCHAR ... utf8mb4_bin`，未满足 VARBINARY 原始字节唯一性。
- RED：反射合同发现 `OrderCreateRequestHash` 仍为 `func(Order, ...string)`。
- RED：repository 源码合同发现 `SQLOrderRepository.CreateContext/Create` 仍存在。
- GREEN：handler/repository 不再 trim 或 Unicode 归一化 key，仅验证原始值非空且不超过 128 bytes；HTTP 协议层继续负责控制字符约束。
- GREEN：0173 将订单表和 receipt key 均改为 `VARBINARY(128)`；down 仅删除 receipt，保留向后兼容的 VARBINARY。真实 down 测试在写入五种字节不同 key 后成功且不丢订单。
- GREEN：`OrderRepository` 主接口统一为 `CreateIdempotentContext/ListContext/TransitionContext`；删除 SQL/Memory 旧创建入口和 SQL actor=0 wrapper，所有测试 seed 改走正式幂等命令。
- GREEN：`OrderCreateRequestHash(order, requestedID string)` 改为非 variadic 显式参数，反射合同和全量编译门禁阻止遗漏。

### 追加验证

- MariaDB 10.6：`go test -tags=integration ./internal/modules/scrm/adapters/mysql -run TestOrder -count=3 -v`，PASS。
- MySQL 5.7：同一命令，PASS。
- `go test ./... -count=1`：PASS。
- Dashboard typecheck：PASS；订单页与公共 API 定向 Vitest：2 文件、8 测试 PASS。
- `pnpm check:phase3-5-dashboard`：仍仅因既有 `/chat/file-audio` 清单漂移失败，本次未修改路由或 manifest。
