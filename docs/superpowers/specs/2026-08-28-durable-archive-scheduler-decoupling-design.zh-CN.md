# durable 会话归档调度与 worker 解耦设计

## 1. 目标与非目标

本次只修复 durable 会话归档的自动入队边界：

- `MOCHAT_GO_ENABLE_DURABLE_WORK_MESSAGE_ARCHIVE=1`、`MOCHAT_GO_ENABLE_WORK_MESSAGE_ARCHIVE_SYNC_CRON=0` 时，只启动处理既有 queued run 的 durable worker 和媒体 worker；跨任意调度周期均不得创建计划 run 或 enqueue/start/complete 审计。
- 同一配置下，Dashboard“立即同步会话”仍写入一个 `archive:manual:<requestId>` queued run，并由 durable worker 完成既有 cursor、lease、幂等、失败不推进和媒体派生链路。
- `MOCHAT_GO_ENABLE_WORK_MESSAGE_ARCHIVE_SYNC_CRON=1` 时，自动调度只负责发现有数据的 scope 并入队；durable worker 只负责处理已入队任务。空上游页不创建 run/audit。
- 保留 legacy pipeline、WeWork callback、Dashboard API、媒体恢复和 0138 账本合同，不删除历史 run/audit，不用日志过滤、延长周期或清理任务掩盖增长。

非目标：不重写 Finance SDK bridge，不改变消息/媒体表结构，不修改 Dashboard 页面合同，不清理现有约 956 条 run 和 2862 条审计历史证据。

## 2. 复现与根因

### 2.1 运行复现

2026-08-28 本地 `mochat-go-desktop` 的 app 环境为：

- `MOCHAT_GO_ENABLE_DURABLE_WORK_MESSAGE_ARCHIVE=1`
- `MOCHAT_GO_ENABLE_WORK_MESSAGE_ARCHIVE_SYNC_CRON=0`
- `MOCHAT_GO_WORK_MESSAGE_ARCHIVE_SYNC_CRON_INTERVAL_SECONDS=60`
- 两个满足 durable eligibility 的 tenant/corp scope

数据库在 11:12:28 为 `runs=956`、`audits=2862`；11:12:36 为 `runs=958`、`audits=2868`。两个新增 run 均为 `archive:poll:*:<minute-window>`，状态 succeeded 且 `fetched_count=0`、`processed_count=0`。增长率为：

- run：`有效 scope 数 × 每分钟 1 条`
- audit：`有效 scope 数 × 每分钟 enqueue/start/complete 3 条`

因此 N 个有效租户每天产生 `1,440N` 条空 run 和 `4,320N` 条无业务数据审计；当前两个 scope 即每天 2,880/8,640 条。

### 2.2 根因调用链

`cmd/mochat-go/main.go` 只判断 `EnableDurableWorkMessageArchive`，便无条件注册 `cron-durable-work-message-archive-sync`。该 task 每分钟调用 `DurableBridgeRunner.RunOnce`。

`RunOnce` 同时承担两种职责：

1. 查询并执行 `PendingDurableArchiveRuns`，这是人工 queued run、失败重试和重启 lease 接管需要的 worker 职责。
2. 查询 `DurableArchiveBindings`，为所有未处理 scope 生成分钟窗口键并直接调用 `Sync`，这是自动周期发现与入队职责。

第二步不读取 `EnableWorkMessageArchiveSyncCron`。即使自动调度开关为 0，durable worker 仍自行创建计划 run；`SyncService.Sync` 依次调用 `EnqueueArchiveSync`、`MarkArchiveSyncRunning`、`CompleteArchiveSync`，所以空页也落一条 run 和三联审计。

人工同步由 `POST /dashboard/company/archive-sync` 经 `CompanyArchiveSyncScheduler` 调用 `DurableBridgeRunner.EnqueueScope`，只创建 queued run，不直接访问 bridge；它需要的是 durable worker 的第一项职责，而不是自动发现。

legacy `event.msgaudit_notify` 由 Redis `WeWorkCallbackWorker` 调用 `WorkMessageArchiveSyncCron.RunCorp`。它只在 legacy pipeline 组装，当前 durable pipeline 未复用该直接同步 trigger。本次保持该既有边界，不让自动调度开关变化破坏 callback。

媒体恢复由独立 `cron-durable-work-message-archive-media` 领取媒体任务；它不创建 archive sync run，继续只由 durable 能力开关控制。

## 3. 备选方案

### 3.1 复用现有 cron 开关（采用）

把 `MOCHAT_GO_ENABLE_WORK_MESSAGE_ARCHIVE_SYNC_CRON` 定义为“是否启动会话归档自动调度”，把 `MOCHAT_GO_ENABLE_DURABLE_WORK_MESSAGE_ARCHIVE` 定义为“是否启用 durable 账本、人工入队 worker 和媒体 worker”。durable 模式决定调度器使用 durable enqueuer，非 durable 模式继续使用 legacy cron。

优点：修正现有开关名与运维直觉，默认值不变，现有生产组合无需新增配置。缺点：`durable=1, cron=1` 从原来的非法组合变为合法的 durable 自动调度组合，需要更新互斥测试和说明。

### 3.2 新增 durable 专用 cron 开关

新增 `MOCHAT_GO_ENABLE_DURABLE_WORK_MESSAGE_ARCHIVE_SYNC_CRON`。优点是字段最显式；缺点是三个开关形成冗余状态，旧 `MOCHAT_GO_ENABLE_WORK_MESSAGE_ARCHIVE_SYNC_CRON=0` 仍不能独立表达用户期望，升级配置复杂。

### 3.3 durable 永不自动调度

把现有 `RunOnce` 改成只处理 pending。优点是改动最小；缺点是无法满足自动调度开启场景，也会永久丢失 durable 定时兜底能力。

## 4. 配置与启动语义

| durable | archive cron | 行为 |
|---:|---:|---|
| 0 | 0 | 不启动会话归档同步 |
| 0 | 1 | 保持 legacy `cron-work-message-archive-sync` |
| 1 | 0 | 启动 durable queued worker、媒体 worker；不启动自动 enqueuer |
| 1 | 1 | 启动 durable queued worker、媒体 worker和 durable 自动 enqueuer；不启动 legacy cron |

两个开关默认仍为 0。移除 legacy/durable 互斥校验，改由启动组合保证同一进程只注册一种自动调度器。`WORK_MESSAGE_ARCHIVE_SYNC_CRON_INTERVAL_SECONDS` 继续控制 worker 与调度 tick，避免新增升级参数；queued worker 在进程启动时立即执行一次，以恢复重启前任务，之后按 interval 轮询。`RUN_ON_START` 只控制自动 enqueuer 是否启动即探测，不再阻止 queued worker 恢复。

启动日志只记录一次：durable worker 已启用、自动调度已启用或已关闭。普通空 worker poll 和空自动探测不写日志。

## 5. 组件与数据流

### 5.1 durable queued worker

`DurableBridgeRunner.RunPendingOnce(ctx)`：

1. 查询最多 limit 条 queued 或 lease 已过期的 run。
2. 按账本中的 binding、cursor、idempotency key 重建 source。
3. 调用既有 `SyncService.Sync`；已完成/并发领取按现有幂等与 lease fence 处理。
4. 不调用 `DurableArchiveBindings`，没有 pending 时不写任何 archive run/audit，也不打印空轮询日志。

人工同步继续执行：HTTP principal → `EnqueueScope` → 0138 queued run/enqueue audit → `RunPendingOnce` → start/complete 或 fail audit → 媒体 worker。

### 5.2 durable 自动 enqueuer

`DurableBridgeRunner.EnqueueScheduledOnce(ctx)`：

1. 先读取 busy scope（queued 或 running run），已有在途任务的 scope 不再探测或重复入队；worker 领取列表仍只包含 queued 与 lease 已过期的 running run，二者不能混用。
2. 读取所有有效 durable bindings，按 tenant/corp 隔离。
3. 读取各 scope 权威 cursor，并用 bridge `Fetch` 做只读 availability probe。
4. 空页且 `HasMore=false`：直接结束，不写 run/audit、不写正常日志。
5. 非空页：用 `archive:poll:<tenant>:<corp>:<source>:<cursor>:<UTC-minute>` 入队 queued run；worker 再按同一 cursor 正式执行并持久化。
6. 空页却 `HasMore=true`、source 身份不一致或 bridge 失败：返回错误并由周期 task 记录一条失败日志，不推进 cursor。

probe 不消费 Finance SDK 数据；正式 worker 会重新按相同 seq 拉取。只有至少一条 source identity 正确、`seq` 大于权威 cursor 且 `msgid` 非空的消息才入队；全是旧消息的页不落 run，身份错误或游标停滞直接失败。数据库唯一键保证同 scope/source/窗口的多实例并发只形成一个 run。busy-scope 抑制避免 worker 尚未完成时跨窗口堆积同 cursor run。

### 5.3 callback 与媒体

legacy callback 仍调用 legacy `RunCorp`，不经过 durable 自动 enqueuer；durable 自动调度关闭不改变 callback worker 的其他事件处理。媒体 worker 保持独立领取由消息 upsert 派生的媒体任务，失败可重试且不回滚正文 cursor。

## 6. 并发、重启与失败

- 重启：queued 和 lease 过期 running run 由启动即执行的 `RunPendingOnce` 恢复；未完成页不凭自动调度开关推进 cursor。
- 并发进程：自动 enqueuer 的窗口键由数据库唯一约束幂等；worker 的 running lease/token fence 防止旧实例提交。
- 多租户：binding、pending、cursor、run 唯一键均包含 tenant/corp/source；一个 scope 的 probe 或 worker 失败不扩大到其他 scope，但本轮返回第一个错误供运维发现。
- 失败重试：人工 requestId 与自动窗口键不变；failed run 按现有 `RetryFailed=true` 重置为 queued，保留 retry 审计。
- 空数据：不创建 scheduled run，避免把“没有消息”伪装成业务生命周期证据。

## 7. 自动化与运行验收

1. runner 单测证明连续两次 `RunPendingOnce` 在无 queued run 时不读取 bindings、不访问 bridge、不写模板。
2. 手动 queued run 单测/集成测试证明只形成一次 enqueue/start/complete，cursor 和消息写入正确。
3. 自动 enqueuer 单测覆盖空页零入队、非空页入队、同窗口幂等、pending scope 抑制、多租户隔离和 probe 失败不推进。
4. config/main 组合测试覆盖四种开关组合、默认缺失、legacy 兼容、durable 自动模式不重复注册。
5. 保持 store 的 lease fence、跨租户 FK、失败不推进、媒体恢复测试通过。
6. Docker 只重建/重建 app，不删除 volume：cron=0 跨至少两个 60 秒周期查询 run/audit 稳定；人工触发后仅增加一个业务 run 生命周期；cron=1 空上游跨两个周期仍不增加 run/audit；重启后 queued run 完成。
7. 运行 Go 全量测试、相关 Node 合同、lint/typecheck/build、`git diff --check`，读取 app 日志确认只有启动、真实入队、任务完成/失败等关键事件。

## 8. 已知边界

- 本地 fixture bridge 证明正式协议、durable 账本和容器路径，不等同于真实企业微信生产拉取。
- queued worker 仍按数据库轮询，因此通用 `mochat_go_background_task_executions` 会按 taskrunner 的现有策略记录 tick；本次修复的权威边界是 archive run/audit 不再因空调度增长，不扩展为全局 taskrunner 留存改造。
- 不删除修复前的历史空 run/audit；它们保留为事故证据。
