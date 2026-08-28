# durable 会话归档调度与 worker 解耦 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 自动调度关闭时 durable queued worker 继续可用，但任何周期都不创建计划 run/audit；自动调度开启时只为确有上游消息的 scope 幂等入队。

**Architecture:** 将 `DurableBridgeRunner.RunOnce` 拆为只处理账本任务的 `RunPendingOnce` 与只发现并入队的 `EnqueueScheduledOnce`。现有 cron 开关统一控制 legacy 或 durable 自动调度，durable 能力开关只控制 queued worker、人工入口和媒体恢复。

**Tech Stack:** Go、MariaDB 10.6、Docker Compose、Node.js 22、pnpm 11。

## Global Constraints

- 先 RED 后 GREEN；不删除历史 run/audit，不修改 volume，不 reset/clean。
- `durable=1, cron=0` 不得周期创建 archive run/audit；人工 queued run 必须可完成。
- `durable=1, cron=1` 只注册 durable enqueuer，不同时注册 legacy cron；空 bridge 页零入队。
- 保留 tenant/corp/source、cursor、lease、idempotency、失败不推进和媒体恢复合同。
- 日志不记录 token、凭据、正文或空轮询，只记录启动模式和真实任务关键结果。

---

### Task 1: runner 职责拆分与空数据零入队

**Files:**
- Modify: `internal/modules/providers/archive/durable_bridge.go`
- Test: `internal/modules/providers/archive/durable_bridge_test.go`
- Modify: `internal/store/archive_sync.go`
- Test: `internal/store/archive_sync_integration_test.go`

**Interfaces:**
- Produces: `RunPendingOnce(context.Context) error` 只执行 queued/expired run。
- Produces: `EnqueueScheduledOnce(context.Context) error` 只 probe 并创建有业务意义的 queued run。
- Produces: `BusyDurableArchiveScopes(context.Context) ([]Scope, error)` 只供 enqueuer 抑制 queued/running scope；不改变 worker 的领取语义。
- Keeps: `Enqueue`、`EnqueueScope`、`SyncService.Sync`、`DurableArchivePollIdempotencyKey`。

- [ ] **Step 1: 写 RED**

```go
func TestDurableBridgeRunnerPendingWorkerDoesNotPollBindings(t *testing.T) {
    store := &durableBridgeTestStore{syncTestStore: newSyncTestStore()}
    runner := NewDurableBridgeRunner(store, testBridgeClient(t, emptyPage), 10)
    require.NoError(t, runner.RunPendingOnce(context.Background()))
    require.NoError(t, runner.RunPendingOnce(context.Background()))
    require.Zero(t, store.bindingCalls)
    require.Empty(t, store.lastTemplate.IdempotencyKey)
}

func TestDurableBridgeRunnerScheduledProbeSkipsEmptyPage(t *testing.T) {
    store := eligibleDurableBridgeStore()
    require.NoError(t, NewDurableBridgeRunner(store, testBridgeClient(t, emptyPage), 10).EnqueueScheduledOnce(context.Background()))
    require.Empty(t, store.lastTemplate.IdempotencyKey)
}
```

- [ ] **Step 2: 运行 RED**

Run: `go test ./internal/modules/providers/archive -run 'TestDurableBridgeRunner(PendingWorker|Scheduled)' -count=1`

Expected: FAIL，提示 `RunPendingOnce`、`EnqueueScheduledOnce` 或计数字段不存在，且失败来自缺失行为而非测试错误。

- [ ] **Step 3: 最小实现**

```go
func (r *DurableBridgeRunner) RunPendingOnce(ctx context.Context) error {
    pending, err := r.store.PendingDurableArchiveRuns(ctx, r.limit)
    if err != nil { return err }
    var firstErr error
    for _, item := range pending {
        if err := r.syncBinding(ctx, item.Binding, item.Cursor, item.IdempotencyKey); err != nil && firstErr == nil { firstErr = err }
    }
    return firstErr
}

func (r *DurableBridgeRunner) EnqueueScheduledOnce(ctx context.Context) error {
    busy, err := r.store.BusyDurableArchiveScopes(ctx)
    if err != nil { return err }
    busyScopes := make(map[Scope]struct{}, len(busy))
    for _, scope := range busy { busyScopes[scope] = struct{}{} }
    bindings, err := r.store.DurableArchiveBindings(ctx)
    if err != nil { return err }
    var firstErr error
    for _, binding := range bindings {
        if _, exists := busyScopes[binding.Scope]; exists { continue }
        source, sourceErr := NewBridgeSource(r.client, binding.Scope, binding.WXCorpID, binding.IntegrationMode)
        if sourceErr != nil { if firstErr == nil { firstErr = sourceErr }; continue }
        cursor, cursorErr := r.store.LatestArchiveSyncCursor(ctx, binding.Scope, source.SourceID())
        if cursorErr != nil { if firstErr == nil { firstErr = cursorErr }; continue }
        page, fetchErr := source.Fetch(ctx, binding.Scope, cursor, r.limit)
        if fetchErr != nil { if firstErr == nil { firstErr = fetchErr }; continue }
        hasNewMessage := false
        for _, message := range page.Messages {
            if !messageIdentityMatches(message, source) || message.Seq <= 0 || strings.TrimSpace(message.MsgID) == "" {
                if firstErr == nil { firstErr = errors.New("archive scheduled probe returned invalid source data") }
                hasNewMessage = false
                break
            }
            if message.Seq > cursor.Sequence { hasNewMessage = true }
        }
        if !hasNewMessage {
            if page.HasMore && firstErr == nil { firstErr = errors.New("archive scheduled probe cursor stalled") }
            continue
        }
        _, enqueueErr := NewSyncService(r.store).Enqueue(ctx, source, SyncRequest{
            Scope: binding.Scope, StartCursor: cursor, Limit: r.limit, RetryFailed: true,
            IdempotencyKey: DurableArchivePollIdempotencyKey(binding.Scope, source.SourceID(), cursor, r.now()),
        })
        if enqueueErr != nil && firstErr == nil { firstErr = enqueueErr }
    }
    return firstErr
}
```

- [ ] **Step 4: 补齐同窗口、多租户、pending 抑制、probe 失败 RED→GREEN**

Run: `go test ./internal/modules/providers/archive -count=1`

Expected: PASS；已有 cursor、manual、failure tests 全绿。

- [ ] **Step 5: 提交**

```text
git add internal/modules/providers/archive/durable_bridge.go internal/modules/providers/archive/durable_bridge_test.go internal/store/archive_sync.go internal/store/archive_sync_integration_test.go
git commit -m "fix(archive): separate durable scheduling from queued work"
```

### Task 2: 配置语义与启动注册

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `cmd/mochat-go/main.go`
- Create: `cmd/mochat-go/archive_runtime.go`
- Create: `cmd/mochat-go/archive_runtime_test.go`
- Modify: `deploy/standalone/README.md`

**Interfaces:**
- Produces: `archiveRuntimePlanFor(config.Config) archiveRuntimePlan`，四种开关组合只有一个自动 pipeline。
- Consumes: Task 1 的 `RunPendingOnce`、`EnqueueScheduledOnce`。

- [ ] **Step 1: 写 RED**

```go
func TestArchiveRuntimePlanSeparatesDurableWorkerAndScheduling(t *testing.T) {
    off := archiveRuntimePlanFor(config.Config{EnableDurableWorkMessageArchive: true})
    require.True(t, off.durableWorker)
    require.False(t, off.durableScheduler)
    require.False(t, off.legacyScheduler)

    on := archiveRuntimePlanFor(config.Config{EnableDurableWorkMessageArchive: true, EnableWorkMessageArchiveSyncCron: true})
    require.True(t, on.durableWorker)
    require.True(t, on.durableScheduler)
    require.False(t, on.legacyScheduler)
}
```

配置 RED：`durable=1, cron=1` 应成功解析，不再返回 mutually exclusive；默认两个开关仍为 false。

- [ ] **Step 2: 运行 RED**

Run: `go test ./internal/config ./cmd/mochat-go -run 'Test(DurableWorkMessageArchive|ArchiveRuntimePlan)' -count=1`

Expected: FAIL，当前 config 拒绝 durable+cron，且 plan helper 不存在。

- [ ] **Step 3: 最小实现**

```go
type archiveRuntimePlan struct {
    durableWorker, durableScheduler, legacyScheduler bool
}

func archiveRuntimePlanFor(cfg config.Config) archiveRuntimePlan {
    return archiveRuntimePlan{
        durableWorker: cfg.EnableDurableWorkMessageArchive,
        durableScheduler: cfg.EnableDurableWorkMessageArchive && cfg.EnableWorkMessageArchiveSyncCron,
        legacyScheduler: !cfg.EnableDurableWorkMessageArchive && cfg.EnableWorkMessageArchiveSyncCron,
    }
}
```

主程序按 plan：durable worker 注册 `RunPendingOnce` 且 `RunOnStart=true`；durable scheduler 仅在开关开启时注册 `EnqueueScheduledOnce` 并沿用配置的 `RunOnStart`；legacy scheduler 仅在 `legacyScheduler` 时注册。媒体 worker 保持 durable 控制。

- [ ] **Step 4: 更新中文部署说明并运行 GREEN**

Run: `go test ./internal/config ./cmd/mochat-go -count=1`

Expected: PASS，四种组合和配置缺失场景全绿。

- [ ] **Step 5: 提交**

```text
git add internal/config/config.go internal/config/config_test.go cmd/mochat-go/main.go cmd/mochat-go/archive_runtime.go cmd/mochat-go/archive_runtime_test.go deploy/standalone/README.md
git commit -m "fix(archive): gate automatic durable enqueue with cron flag"
```

### Task 3: 日志、专项合同与静态兼容检查

**Files:**
- Modify: `internal/modules/providers/archive/durable_bridge.go`
- Modify: `internal/modules/providers/archive/durable_bridge_test.go`
- Modify: `scripts/check_standalone_public_urls.test.mjs`
- Create: `scripts/check_durable_archive_scheduler.mjs`
- Create: `scripts/check_durable_archive_scheduler.test.mjs`

**Interfaces:**
- Produces: 启动模式、人工入队、真实 run 开始/完成/失败的低噪声日志；空 poll/probe 无日志。
- Produces: Node 合同校验 compose 默认 cron=0、主程序注册互斥和 runner 拆分边界。

- [ ] **Step 1: 写 RED**：捕获 logger，断言空 worker/probe 输出为空，人工入队和真实 pending run 只输出脱敏关键字段；专项 Node 合同先因脚本不存在失败。
- [ ] **Step 2: 运行 RED**

Run: `go test ./internal/modules/providers/archive -run 'TestDurableBridgeRunnerLogs' -count=1 && node --test scripts/check_durable_archive_scheduler.test.mjs`

Expected: FAIL。

- [ ] **Step 3: 最小实现**：runner 接受可选 `*log.Logger`；只在新人工 queued run、真实 pending run start/complete/fail 记录 INFO/ERROR，不打印 requestId、bridge token、消息正文或普通空轮询。Node 合同读取 Go/config/compose 源并验证配置组合与任务名。
- [ ] **Step 4: 运行 GREEN**

Run: `go test ./internal/modules/providers/archive -count=1 && node --test scripts/check_durable_archive_scheduler.test.mjs scripts/check_standalone_public_urls.test.mjs`

Expected: PASS。

- [ ] **Step 5: 提交**

```text
git add internal/modules/providers/archive/durable_bridge.go internal/modules/providers/archive/durable_bridge_test.go scripts/check_durable_archive_scheduler.mjs scripts/check_durable_archive_scheduler.test.mjs scripts/check_standalone_public_urls.test.mjs
git commit -m "test(archive): enforce quiet durable scheduling boundaries"
```

### Task 4: 全量门禁与 Docker 跨周期验收

**Files:**
- Create: `docs/superpowers/reports/2026-08-28-durable-archive-scheduler-acceptance.zh-CN.md`

**Interfaces:**
- Produces: 修复前后数据库计数、容器日志、开关组合、人工 run 生命周期、重启/并发/租户/失败边界和精确 SHA 的中文证据。

- [ ] **Step 1: 新鲜代码门禁**

```text
go test ./... -count=1
go vet ./...
corepack pnpm lint
corepack pnpm typecheck
corepack pnpm test
corepack pnpm build
node --test scripts/check_durable_archive_scheduler.test.mjs scripts/check_standalone_public_urls.test.mjs
git diff --check
```

- [ ] **Step 2: Docker cron=0 验收**：保留 MariaDB/Redis/媒体 volume，仅重建 app。记录 t0 的 archive run/audit 计数，等待至少 130 秒，再记录 t1；期望完全一致。读取日志，确认仅一次 worker enabled/automatic scheduler disabled，没有每分钟空日志。
- [ ] **Step 3: 人工触发**：通过现有已登录 Dashboard API 或本地 acceptance 入口提交唯一 requestId；查询该 tenant/corp/requestId 仅一条 run，audit 为 enqueue/start/complete，status=succeeded，cursor/消息按 fixture 推进；重复 requestId 不新增。
- [ ] **Step 4: cron=1 空数据验收**：只重建 app，跨至少两个周期；bridge 空数据时 run/audit 计数不变，同窗口/并发启动不重复入队。随后恢复 cron=0 并只重建 app。
- [ ] **Step 5: 重启与失败**：在可恢复 fixture 中保留 queued run 后重启 app，确认 worker 接管；bridge 受控失败时 status=failed、cursor 不推进，恢复后 retry 成功；查询另一 tenant/corp 无交叉记录。
- [ ] **Step 6: 写中文报告并最终提交**

```text
git add docs/superpowers/reports/2026-08-28-durable-archive-scheduler-acceptance.zh-CN.md
git commit -m "docs(archive): report scheduler decoupling acceptance"
git status --short --branch
git rev-parse HEAD
```

Expected: 隔离分支干净，未合并 main，报告引用的 SHA 与最终提交一致。

## 自检结果

- 设计的配置、runner、main 注册、日志、自动化、Docker、数据库和兼容性要求均映射到明确任务。
- 文档不存在未决占位；接口名在各任务一致。
- 执行方式由用户指定为当前隔离任务内联推进，不等待额外选择，也不调用并行开发任务。
