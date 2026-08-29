# Task 4：HTTP service group、readiness 与 periodic panic 实施报告

日期：2026-08-29

工作目录：`D:\workspace\mochat-go\mochat-go\.worktrees\p0-local-closure-20260829`

基线：`54c382284755d3d55ab4a3413fe38b716a3c2f0b`

## 根因与改动

根因有三组：

1. `cmd/mochat-go` 的 main、Sidebar、Operation 三个 listener 直接调用 `http.Serve`，没有统一的 server timeout、drain 状态和错误收敛；worker 又从独立的 `context.Background()` 启动，SIGTERM 无法形成“先摘流、再停新任务、再 drain HTTP、最后有界等待 worker”的顺序。
2. `/readyz` 只检查前端资产和兼容 PHP upstream，没有检查实际进程依赖的 MySQL、Redis、迁移 ledger/checksum 和后台任务存活状态。Compose 同时把 `/readyz` 当 app 启动 healthcheck，而部署脚本在 app healthy 后才迁移，若 readiness 纳入迁移就会形成自锁。
3. `taskrunner.Periodic` 只在最外层 Group goroutine recover；任一 tick panic 都会终止整个 periodic goroutine，后续 tick 永久不再执行。

本次新增 `internal/runtimegroup`：

- 三端都由显式 `http.Server` 持有，统一 `ReadHeaderTimeout=5s`、`ReadTimeout=15s`、`WriteTimeout=30s`、`IdleTimeout=60s`。
- 仓库未发现 SSE、WebSocket 或 `http.Flusher` 流式 HTTP 路由，因此没有需要单独覆盖 `WriteTimeout` 的现有路由。
- request `BaseContext` 使用 `context.WithoutCancel(root)`：保留统一 root 的值，但 signal/root cancellation 不会提前取消在途请求；在途请求由 `http.Server.Shutdown` 负责 drain。
- service group 收敛所有 Serve/Accept 错误；开始关闭时先 `BeginDrain()`，再取消 worker root 停止接新任务，然后并行 drain HTTP。worker 由 `Group.Wait(ctx)` 最多等待 30 秒。
- composition root 仍保留在 `main()`。listener/serve 错误写入 `runtimeErr` 后 return，最早注册的 fatal defer 会在后注册的 listener/worker/root cleanup defer 之后执行，避免 `os.Exit` 跳过清理。

readiness 改为稳定 code 合同：

- `mysql_connection`
- `redis_connection`
- `migration_current`
- `background_tasks`
- `runtime_draining`

每项依赖探针使用 400ms 子超时，资产/PHP 与依赖探针共享 2 秒总预算。响应只包含 code 与 ready 布尔值，不序列化底层错误，因此不会泄露 DSN、Redis 地址、密码或迁移细节。

拓扑没有机械要求 API 进程内存在 worker：只有进程实际登记了后台任务时才加入 `background_tasks`；worker/scheduler-only 角色不启动 HTTP listener。Task 1 的 durable worker、scheduler 与 API 职责矩阵未改写，Task 3 的 callback Redis 可恢复降级合同也未改写。

部署顺序改为：

```text
启动基础设施与 app
-> app /healthz 通过
-> baseline / incremental migration
-> app /readyz 通过
-> 页面入口检查
```

Compose app healthcheck 已从 `/readyz` 改为始终表示进程存活的 `/healthz`；部署脚本在迁移前显式检查 `/healthz`，迁移完成后才检查 `/readyz`，因此 readiness 检查迁移状态不会自锁。

## RED（先测失败）

首次加入慢 header、在途 drain、listener error、依赖故障、draining 与 periodic panic 测试后运行：

```text
go test ./internal/runtimegroup ./internal/server ./internal/taskrunner \
  -run 'Test(NewHTTPServer|HTTPServer|GroupDrains|GroupReturns|HealthzStays|ReadyzFailsImmediately|PeriodicRecoversPanic)' \
  -count=1
```

结果：预期失败，退出码 `1`。

关键证据：

```text
internal\runtimegroup\http_test.go:17:12: undefined: NewHTTPServer
internal\server\server_test.go:658:13: undefined: NewReadinessChecker
panic: tick exploded token=hidden-panic-token
```

这证明 service group/readiness API 不存在，且 periodic 首 tick panic 会直接击穿 goroutine。

自审时又补了全局 readiness 预算 RED：

```text
go test ./internal/server -run TestReadyzCapsTheWholeProbeBudgetAtTwoSeconds -count=1
```

结果：预期失败，退出码 `1`，旧 PHP probe 用时 `3.0056082s`，证明仅给依赖 checker 设置预算不能满足整个 `/readyz` 的 2 秒上限。

## GREEN 与故障注入

目标门禁：

```text
go test ./internal/runtimegroup ./internal/server ./internal/taskrunner ./cmd/mochat-go -count=1
go vet ./internal/runtimegroup ./internal/server ./internal/taskrunner ./cmd/mochat-go
git diff --check
```

结果：全部退出码 `0`。

全仓回归：

```text
go test ./... -count=1
```

结果：退出码 `0`，所有 Go 包通过或按既有条件报告 `[no test files]`。

故障注入由 `TestHealthzStaysLiveWhileDependencyReadinessRecovers` 完成：

- 注入 MySQL down、Redis down、migration behind 后，`/healthz=200`、`/readyz=503`。
- 响应稳定包含 `mysql_connection`、`redis_connection`、`migration_current`，且断言不包含伪造 DSN、主机地址、密码和迁移版本错误详情。
- 清除三个故障后，同一 server 的 `/readyz` 恢复为 `200`。
- `TestReadyzFailsImmediatelyWhenDraining` 证明 drain 开始即以 `runtime_draining` 摘流。

HTTP 故障与关闭测试还证明：

- 不完整慢 header 在测试缩短后的 header deadline 内被拒绝，业务 handler 未执行。
- shutdown context（生产由 SIGTERM 的 `signal.NotifyContext` 触发）开始关闭后，readiness 先摘流，root worker context 随后取消；已进入 handler 的请求 context 不被 root cancel，释放后正常返回 `204`。
- listener Accept error 会执行 readiness/stop-work hook，并作为错误返回，不在 serve goroutine 中调用 fatal。
- periodic 首 tick panic 被记录为失败，敏感 token 被清洗，第二 tick 继续执行并记录 `periodic_task_recovered`。

## 部署顺序验证

运行：

```text
powershell.exe -NoProfile -ExecutionPolicy Bypass -Command \
  "[Console]::OutputEncoding = [System.Text.Encoding]::UTF8; \
   & '.\scripts\deploy_docker_desktop.ps1' -DryRun -ProjectName 'mochat-task4-dryrun'"
```

结果：退出码 `0`。输出顺序为 app health 等待、迁移前 `/healthz`、baseline/up migration、迁移后 `/readyz`，未执行 Docker 变更。

## 自审与额外门禁发现

第一次全仓测试曾失败于 `internal/modules/providers/catalog/TestProviderCompositionContract`。原因不是 Provider 业务回归，而是我最初把 composition root 从 `main()` 提取到 `run()`，既有 AST 门禁只允许真实 Provider data flow 在 `main()` 可达。最终没有削弱门禁，而是把 composition root 放回 `main()`，用 defer 注册顺序同时保留清理完成后非零退出的语义；该专项测试和随后全仓测试均通过。

其他自审结论：

- 没有修改 `docs/PROJECT_PROGRESS.zh-CN.md`。
- 没有 reset/clean、没有修改用户其他 worktree、没有删除或重建 Docker 命名卷。
- 没有扩写业务幂等边界。
- 没有访问真实 Provider 或生产环境。

## SKIP 与证据边界

- 真实 MariaDB/Redis 容器断开与恢复：SKIP。本次证据是自动化的进程内 readiness probe 故障注入，不冒充真实依赖集成验收。
- 真实 Docker Compose 构建/启动/迁移：SKIP；只运行无副作用 DryRun。
- `go test -race`：SKIP；本任务未在 Windows 环境启用 race 工具链。
- 真实 SIGTERM 子进程端到端：SKIP；自动化测试覆盖同一 `signal.NotifyContext` 下游 cancellation/drain 边界，但没有向生产形态子进程发送 OS signal。
- 真实 Provider、浏览器与生产部署：SKIP，均不在 Task 4 授权范围内。
