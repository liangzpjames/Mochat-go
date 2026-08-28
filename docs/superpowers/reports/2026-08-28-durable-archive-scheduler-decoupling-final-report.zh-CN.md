# Durable 会话归档调度与 Worker 解耦最终验收报告

日期：2026-08-28

分支：`fix/durable-archive-scheduler-decoupling-20260828`

隔离 worktree：`D:\workspace\mochat-go\mochat-go\.worktrees\durable-archive-scheduler-decoupling-20260828`

基线：`7cce30a7a6b57646371669ebd37a89c8e8a5b1bf`

代码收口提交：`ec093fbf563a981fa37e7f4eeacf0f6d6cfbf9cb`

## 1. 交付结论

本红灯已修复，且没有通过过滤日志、删除审计、延长周期或定时清理掩盖问题。

- `MOCHAT_GO_ENABLE_DURABLE_WORK_MESSAGE_ARCHIVE=1` 只表示 durable 队列 worker、媒体 worker 和人工 durable 入队能力可用。
- `MOCHAT_GO_ENABLE_WORK_MESSAGE_ARCHIVE_SYNC_CRON=1` 才允许注册自动周期入队器。
- durable=1、cron=0 时只消费已经持久化的任务，不枚举租户、不探测 bridge、不创建周期 run/audit。
- durable=1、cron=1 时，自动入队器先探测有效绑定；只有发现至少一条新且身份有效的消息才创建 run。空页不落 run/audit。
- 人工“立即同步会话”继续使用同一 durable 账本，且在 cron=0 时可以成功完成。
- legacy cron 仅在 durable=0、cron=1 时注册，durable 与 legacy 自动调度器不会同时启动。

本报告只确认本地 fixture bridge、真实 Go 服务、真实 MariaDB 持久化和 Docker 运行合同通过，不等同于真实企业微信 Finance SDK 线上授权验收通过。

## 2. 根因与影响

### 2.1 根因

旧实现把“处理已入队任务”和“发现所有 eligible binding 并为当前分钟创建周期任务”放在同一个 `DurableBridgeRunner.RunOnce` 中。`cmd/mochat-go/main.go` 又只根据 durable 开关无条件注册该周期任务。因此 cron 开关虽然关闭了 legacy cron，却没有关闭 durable runner 内部的 scheduled poll。

每次调用会先枚举 binding，再按当前 cursor 和分钟窗口生成 `archive:poll:*` 幂等键。即使 bridge 返回空页，也会先创建 run，随后 worker 把它推进 queued → running → succeeded，于是每个租户每分钟产生一条空 run 和 enqueue/start/complete 三条审计。

### 2.2 修复前实测

2026-08-28 11:12，本地配置为 durable=1、cron=0、interval=60s、run-on-start=0：

- 11:12:12：runs=954，audits=2862。
- 11:12:36：runs=958，audits=2868。
- 两个 eligible binding 每分钟各新增一个空 run；新 run 均为 fetched=0、processed=0。

影响按 eligible 租户数 N 线性增长：每天 `1440N` 条 run、`4320N` 条审计。本地 N=2 时即每天 2880 条 run、8640 条审计。

## 3. 实现说明

### 3.1 运行职责

- `RunPendingOnce`：只查询 queued 或 lease 已过期的 running run，并执行 durable 同步。
- `EnqueueScheduledOnce`：只负责自动发现、空页抑制和周期入队，不推进权威 cursor。
- `BusyDurableArchiveScopes`：把 queued 与全部 running scope 都视为 busy，阻止在途任务后面堆叠第二个 cursor 窗口。
- `DurableArchivePendingRun.RunID`：把持久化 run 身份带给 worker 日志，不记录请求正文或 Provider 响应。

### 3.2 配置矩阵

| durable | cron | durable worker | durable scheduled enqueuer | legacy cron |
| --- | --- | --- | --- | --- |
| 0 | 0 | 关闭 | 关闭 | 关闭 |
| 0 | 1 | 关闭 | 关闭 | 开启 |
| 1 | 0 | 开启 | 关闭 | 关闭 |
| 1 | 1 | 开启 | 开启 | 关闭 |

两个开关缺失时继续沿用既有默认值 0。此前的“两个开关互斥”校验已移除，因为 durable worker 与自动调度现在是正交职责；默认值和 compose 默认仍保持关闭，兼容旧部署的安全边界。

### 3.3 空数据与幂等

- scheduled probe 返回空页且 `hasMore=false`：不创建 run/audit。
- `hasMore=true` 但无 cursor 进展：返回 `archive.cursor_stalled`，不制造无意义 run。
- 消息来源身份、seq 或 msgid 无效：失败关闭，不入队。
- 同 scope 已有 queued/running：跳过自动探测和入队。
- 同一分钟、同 cursor 的并发入队仍由现有唯一键和数据库重读保证只保留一条 run。
- worker 继续保留 lease fence、过期接管、失败不推进、重试复用、媒体 checkpoint 恢复和租户隔离逻辑。

### 3.4 日志

- 空 pending 队列和空 scheduled probe 零日志。
- 人工/自动实际入队记录 tenant、corp、run 和状态。
- worker start/complete 记录 tenant、corp、run 和聚合计数；失败只记录稳定错误码。
- 自动化明确断言请求 ID、Bearer、消息正文和 Provider 响应体不会进入日志。

## 4. 自动化验收

### 4.1 专项 RED → GREEN

- durable worker 连续两个周期只查询 pending，不调用 binding discovery/bridge，不写 run/audit，且零日志。
- scheduled enqueuer 使用权威 cursor 和分钟幂等键；同窗口重复调用只有一条 run/一条 enqueue 审计。
- 空 binding probe 与 busy scope 均不入队。
- 人工入队请求路径不调用 bridge；pending worker 后续处理并推进 cursor。
- 人工成功/失败日志的生命周期、计数、错误码与脱敏均有测试。
- 四种配置组合、legacy/durable scheduler 互斥和缺省配置均有测试。
- Node 静态启动契约会拒绝 worker 重新发现 binding、无条件注册 durable scheduler 或 compose 默认开启 cron。

### 4.2 真实 MariaDB 隔离 schema

通过 `MOCHAT_GO_MYSQL_INTEGRATION_DSN` 指向当前本地 MariaDB，测试自动创建并删除三个独立 schema，清理后残留均为 0：

- `TestArchiveSyncStoreUsesTemporarySchemaForLifecycleAndTenantIsolation`：PASS。
- `TestArchiveSyncStaleRunningRunIsTakenOverWithAudit`：PASS。
- `TestArchiveSyncConcurrentFirstEnqueueRereadsDuplicateRun`：PASS。
- `TestSyncServiceFailureIsStableAndRetryReusesScopeAndAudit`：PASS。
- `TestSyncServicePassesLeaseIdentityToEveryMutation`：PASS。
- `TestMediaSyncRestartsFromCheckpointAndRejectsEmptyStall`：PASS。

## 5. Docker 与数据库运行验收

Compose project：`mochat-go-desktop`

镜像来源：本隔离 worktree；只对 app 执行构建和计划内重建，最终 app 为 healthy。

### 5.1 cron=0、worker=1，跨周期稳定

启动日志：

```text
INFO durable archive workers enabled: automatic_schedule=false interval=1m0s schedule_run_on_start=false
background task started: worker-durable-work-message-archive-sync
background task started: worker-durable-work-message-archive-media
```

数据库直接查询：

| 时间 | runs | audits | enqueue | start | complete |
| --- | ---: | ---: | ---: | ---: | ---: |
| 11:49:57 | 1032 | 3090 | 1030 | 1030 | 1030 |
| 11:52:04 | 1032 | 3090 | 1030 | 1030 | 1030 |

观察时间 127 秒，覆盖至少两个 60 秒周期，所有计数保持不变。

### 5.2 同配置人工触发

通过真实 Dashboard `POST /dashboard/company/archive-sync` 触发 tenant=1/corp=1 的人工同步，HTTP 200。run 1252 的数据库结果：

- idempotency key：`archive:manual:durable-manual-acceptance-20260828-1155`。
- cursor：10 → 11。
- status：succeeded；attempt=1。
- fetched=1、processed=1、skipped=0、failed=0。
- 审计严格为 enqueue/queued、start/running、complete/succeeded 三条。
- 容器日志严格为 manual enqueued、run started、run completed 三条，计数与数据库一致。

Windows `Invoke-WebRequest` 在收到首个成功响应后抛出客户端空引用，后续使用 `HttpClient` 确认时又产生了 run 1253。该 run 是明确的第二次人工请求，不是自动调度；它以 cursor=11、fetched=0 成功完成并保留三条人工生命周期审计。本次没有删除或改写任何 run/audit 历史。

### 5.3 cron=1、空数据跨周期稳定

启动日志只出现一次：

```text
INFO durable archive workers enabled: automatic_schedule=true interval=1m0s schedule_run_on_start=false
background task started: cron-durable-work-message-archive-enqueue
```

数据库直接查询：

| 时间 | runs | audits | enqueue | start | complete |
| --- | ---: | ---: | ---: | ---: | ---: |
| 11:57:55 | 1034 | 3096 | 1032 | 1032 | 1032 |
| 11:59:35 | 1034 | 3096 | 1032 | 1032 | 1032 |
| 12:00:43 | 1034 | 3096 | 1032 | 1032 | 1032 |

两个完整周期内，两个 eligible tenant 的空 probe 都没有新增 run/audit，也没有 scheduled enqueue 或失败噪声日志。历史 run 按租户为 tenant1=503、tenant4=529；修复后两者的自动空增量均为 0。

### 5.4 重启与最终状态

- app 在 cron=0 → cron=1 → cron=0 三次启动配置中均恢复 healthy。
- 每次启动只注册配置矩阵允许的任务；最终状态为 durable=1、cron=0。
- 最终数据库计数为 runs=1034、audits=3096。
- MySQL、Redis、app storage 和 audit anchor 命名卷均保留。

`run_archive_simulator.ps1 send` 的 Compose one-off 调用曾意外重建 MySQL 容器，但没有执行 `down -v` 或删除命名卷。重建后直接确认仍挂载 `mochat-go-desktop_mysql-data`，runs=1032、audits=3090 和两个 fixture dataset 均完整；后续不再使用该包装脚本触碰基础设施。

## 6. 全量质量门禁

- `go test ./... -count=1`：PASS。
- `go vet ./...`：PASS。
- `corepack pnpm lint`：PASS。
- `corepack pnpm typecheck`：PASS。
- `corepack pnpm test`：PASS；Dashboard 147 个测试文件、966 项测试通过，其他 workspace 包同样通过。
- `corepack pnpm build`：PASS；Dashboard、SaaS Admin、Sidebar、Operation 和共享包生产构建通过。
- `corepack pnpm check:durable-archive-scheduler`：PASS。
- `scripts/check_standalone_public_urls.test.mjs`：PASS，包含 durable compose 转发合同。
- `git diff --check`：PASS。
- Docker image build：PASS；app health：healthy。

隔离 worktree 首次运行前端门禁时因尚未安装 `node_modules` 而提示 eslint/tsc 不存在；执行 `pnpm install --frozen-lockfile` 后，所有失败门禁均原样重跑并通过。

附加的 `pnpm docs:check` 仍报告仓库既有断链：`docs/phases/phase-3-dashboard/benchmark/README.md` 指向不存在的 `../phase-2.1-functional-frontend-migration/evidence/README.md`。该文件与链接不在本任务改动范围内，未擅自修改；此附加检查不冒充通过。

## 7. 兼容性与已知边界

- 未修改人工同步 API、callback handler、cursor schema、lease 时长、失败不推进规则、媒体对象状态或恢复协议。
- legacy 部署 durable=0、cron=1 的行为保持不变。
- compose 和配置缺省仍关闭 durable 与 cron，不会因升级自动开启外部同步。
- scheduled probe 会调用 bridge 判断是否有新消息，但不推进权威 cursor；真实处理仍由 durable worker 完成。
- Provider 短暂失败会让当前 scheduled cycle 返回稳定错误，下一周期重试；失败本身不创建 run/audit。
- 本地 fixture bridge 不是真实企微 Finance SDK；真实企业凭据、网络限流、SDK 二进制、线上媒体和生产部署仍需后续受控验收。
- 分支未合并主线，未 reset、clean 或覆盖其他 worktree/用户改动。

## 8. 提交清单

- `a53b6d6a27b00c1f8d4a9eae79ae4a16f1540ad1`：中文根因设计与实施计划。
- `88a9c5bc9ed0fc224e7d0fd79a944b7a362ac481`：scheduled enqueue 与 pending worker 解耦。
- `a3aa35ed9566cb5a37f86a0adb1bdb9102ab2a91`：配置矩阵、启动注册和 compose 合同。
- `ec093fbf563a981fa37e7f4eeacf0f6d6cfbf9cb`：日志脱敏、pending run 身份和静态回归门禁。

最终报告提交 SHA 以交付时 `git rev-parse HEAD` 的新鲜输出为准。
