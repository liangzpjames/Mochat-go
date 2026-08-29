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

## Reviewer 修复轮（2026-08-29）

### RED 与根因

本轮按 reviewer 的 Critical/Important/Minor 逐项补测试后再实现：

- 部署 RED：`scripts/test_deploy_docker_desktop.ps1` 以 fresh-schema `controlled_pending` 预览运行，最初因脚本没有 `DryRunMigrationState` 且原实现吞掉受控迁移错误而失败；根因是 `Invoke-AutomaticMigrations` 捕获 0130/0131 pending 后继续等待 `/readyz`。
- 迁移 RED：`go test ./internal/migration -run TestStatusReadOnly -count=1` 最初以 `Runner.StatusReadOnly undefined` 失败；原 `Status` 会执行 `CREATE TABLE IF NOT EXISTS`，并忽略 ledger 中 registry 未知版本。
- 周期任务 RED：`go test ./internal/taskrunner -run 'TestPeriodicSnapshotTracks|TestRequiredTasksReady' -count=1` 最初因 Snapshot 不含连续失败、最近成功和最近执行而编译失败；原 readiness 只能判断 goroutine 是否 running。
- 长响应 RED：`go test ./internal/httpresponse -run TestAllowLongWrite -count=1` 最初以 `AllowLongWrite undefined` 失败；全局 `WriteTimeout=30s` 会中止合法的大文件响应。

全仓回归首次发现 `cmd/mochat-bootstrap` 门禁把新维护命令必需的 `--platform-tenant-id` 误判为 legacy bootstrap 参数。根因是该测试用通用 `-tenant-id` 子串代替了对 `mochat-bootstrap` 调用本身的限制；门禁已收窄到原安全目标，仍然禁止部署入口出现 `mochat-bootstrap`、明文密码和旧 secret 参数。

### GREEN 与故障注入

- `controlled_pending` 现在由同一错误解析分支进入“受控迁移维护检查点”，明确要求先创建并验证备份、只读 preflight、审批维护确认工件，再按 0130 backfill、凭据加密、0131 cutover 顺序执行；脚本非零退出且不会触碰 ready wait。
- 正常/已完成 controlled 路径保持 `app /healthz -> automatic migration up -> /readyz`。DryRun 测试同时断言两条路径。
- 新增只读 `StatusReadOnly`：先查询 `information_schema`；ledger 不存在返回 pending，绝不建表。ledger 中 registry 未知/更高版本追加 `database_ahead`，readiness 以稳定 code `migration_database_ahead` 返回 503；已知版本 checksum mismatch 仍由 `migration_current` 摘流。
- periodic 每个 tick 把最近执行、最近成功和连续失败写回内存 Snapshot；成功清零，错误或 panic 递增。当前角色实际注册的必要任务允许单次瞬时失败，连续 3 次失败返回 503，下一次成功恢复 200；未执行过的 periodic 使用 `30s + 首次调度间隔` 启动宽限。
- chat-media、archive media、会话导出 ZIP、合规导出在鉴权和工件校验成功后、写 body 前通过 `http.ResponseController.SetWriteDeadline(time.Time{})` 局部清除 write deadline。注入式 writer 直接断言 zero deadline，无需等待 30 秒；HEAD、普通 API、静态前端和全局 server timeout 均未放宽。

聚焦验证：

```text
powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts/test_deploy_docker_desktop.ps1
go test ./internal/httpresponse ./internal/modules/chat-media/transport/http ./internal/dashboard ./internal/taskrunner ./internal/migration ./cmd/mochat-go -run 'TestAllowLongWrite|TestReadOnlyListDownload|TestArchiveMediaContentServesFull|TestLongResponseDeadlines|TestPeriodicSnapshotTracks|TestRequiredTasksReady|TestStatusReadOnly|TestMigrationReadiness|TestBackgroundTaskReadiness' -count=1
```

结果：全部退出码 `0`。

### 下载路由复查与自审

重新搜索了 `Content-Disposition`、`http.ServeContent`、`http.ServeFile`、`io.Copy(w, ...)` 与 `io.CopyN(w, ...)`。需要越过 30 秒的四类鉴权大响应均已局部处理；其余命中为有界 CSV 生成、前端静态资产、公开上传静态文件或服务内部文件拷贝，不扩大 deadline 例外。

- Task 1 的角色职责矩阵未改写；API 角色不会机械要求进程内不存在的 worker。
- Task 3 的 callback dependency defer/Redis 恢复合同未改写。
- 未执行真实 0130/0131、真实备份恢复、真实 Docker、真实 Provider 或生产操作。
- 未修改 `docs/PROJECT_PROGRESS.zh-CN.md`，未 reset/clean，未触碰命名卷。

### 本轮 SKIP

- 真实 controlled migration 0130/0131 与恢复演练：SKIP，缺少显式生产/维护授权与工件；脚本按设计 fail-fast。
- 真实慢连接超过 30 秒：SKIP；以注入的 ResponseController deadline 合同覆盖，不用睡眠测试冒充网络验收。
- 真实 MySQL DB-ahead 与 ledger 缺失容器：SKIP；以 sqlmock 验证只读 SQL、缺表 pending、未知版本和 checksum 行为。

## Reviewer 复审追加修复（2026-08-29）

复审进一步指出 DryRun 不能替代真实命令链解析、迁移双读存在竞态窗口、ZIP/合规测试应执行真实 handler，以及 periodic 元数据发布存在极短初始化窗口。对应根因和修复如下：

- `mochat-migrate` 现在把 typed `ControlledMigrationPendingError` 同时映射到稳定失败码和机器可解析行 `MIGRATION_CONTROLLED_PENDING\t<version>`；部署脚本从真实 migrate stderr/stdout 解析该行。fake-docker 非 DryRun 故障注入证明 fresh schema 会非零停在维护检查点且不进入 ready wait，已完成 controlled 路径仍按 health→up→ready 完成。
- readiness 的第二次只读 ledger 检查对任意非 `applied` 状态 fail closed，因此两次探测之间数据库变为 ahead 时仍返回 503；`database_ahead` 稳定 code 保持不变。
- 会话导出 ZIP 与合规导出新增真实 handler 测试，执行鉴权、工件打开、deadline 清除和 body 写出，顺序断言为 `artifact,deadline,body`；未授权请求断言不会清除 deadline。
- `ConfiguredPeriodic` 在 Group 将任务标记为 running 的同一 snapshot 更新中发布 periodic 类型和启动宽限。生产 composition root 的 37 个 periodic 注册点全部使用该入口，不再依赖 goroutine 启动后的二次初始化。

最终验证：

```text
powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts/test_deploy_docker_desktop.ps1
go test ./cmd/mochat-go ./cmd/mochat-migrate ./cmd/mochat-bootstrap ./internal/migration ./internal/taskrunner ./internal/httpresponse ./internal/modules/chat-media/transport/http ./internal/dashboard -count=1
go test ./... -count=1
go vet ./...
git diff --check
```

结果均为退出码 `0`。真实 controlled migration、真实 Docker/依赖故障和生产仍保持 SKIP，未因复审扩张授权边界。
