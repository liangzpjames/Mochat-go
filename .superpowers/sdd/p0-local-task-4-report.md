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

## Reviewer 第二轮复审修复（2026-08-29）

### RED 与根因

- 部署命令 RED：fake 部署首先报“维护检查点备份命令不能在 app 容器环境直接执行”。根因是检查点打印宿主机 `go run`，而默认镜像既没有 identity maintenance binaries，也没有复用 app 的 Compose 环境。
- taskrunner RED：`AddPeriodic`、`MaxRunDuration`、`CurrentExecutionStartedAt` 均不存在；panic 快照仍可包含 raw panic value。根因是 previous snapshot 只覆盖已完成 tick，且 `Group.Add(any)` 用运行时 type switch 补元数据。
- readiness RED：callback-only 测试缺少 `redisReadinessProbe`，migration reader 计数证明每次 `/readyz` 调了两次 `StatusReadOnly`。根因分别是 callback 使用独立可选 Redis 实例，而 probe 只观察 primary 实例；两个 migration code 被实现成两个独立 probe。
- cleanup RED：post-worker build error 测试缺少统一 cleanup helper，源码审计命中 worker 启动后的 `fatal/fatalf`。`os.Exit` 会跳过 root cancel 和有界 `Wait`。

### GREEN 与故障注入

- Dockerfile 现在包含 `/usr/local/bin/mochat-identity-preflight` 与 `/usr/local/bin/mochat-identity-migrate`。controlled checkpoint 先以 `docker compose run --rm --no-deps` 执行不输出值的 `MOCHAT_MYSQL_DSN` 非空探测，再打印可直接复用 app env/volume 的 backup-create→backup-verify→preflight→0130 up→credential encryption→0131 cutover→automatic up→临时秘密文件清理命令；输出不含 DSN/密码。fake Docker 非 DryRun 路径实际接收环境探测命令并返回 `maintenance_env=ready`，同时断言 controlled pending 非零退出且不进入 ready wait。真实 Docker 仍为 SKIP。
- callback/welcome 与其他 Redis 消费者统一到同一 lazy Redis topology：callback-only 不做启动期 fatal ping，保留 Task 3 dependency defer/恢复合同；只要该实际实例已注册，就加入 `redis_connection`。注入 Redis down 时即使 worker snapshot 为 Running 仍 `/readyz=503`，恢复后为 200。
- periodic 在 tick 开始时记录 `CurrentExecutionStartedAt`，并记录由显式 `MaxRunDuration` 或 `max(3*interval,startupGrace)` 得到的阈值；已有成功的当前 tick 超时后 readiness 变为 503，完成后清空 current 状态并恢复 200。会话导出 worker 显式给出 30 分钟上限，避免 2 秒轮询间隔误杀合法长导出。
- periodic panic 和 Group 顶层 panic 分别只持久化固定 `periodic_task_panicked`、`background_task_panicked`。`panic("bare-secret")`/带 token 的故障注入断言 execution、snapshot 与结构化日志均无 raw value。
- worker 启动后的 module/server/listener build 错误统一设置 `runtimeErr` 后 return；统一 defer 先 cancel root，再以 30 秒上限等待 worker。测试注入 build error，证明 cancel 先于 wait 且 wait 未被跳过；源码门禁保证该范围不再出现 fatal/os.Exit。
- migration readiness 合并为一次 `StatusReadOnly`；typed readiness failure 在同一次 snapshot 中把 DB-ahead 映射为稳定 `migration_database_ahead`，其他 pending/checksum mismatch 使用 `migration_current`。reader 计数断言每请求只读一次 ledger/checksum。
- `Group.Add(name, func)` 恢复编译期类型安全，新增 `AddPeriodic(name, config, func)`；37 个生产 periodic 注册点全部迁移，不再使用 `any`/静默 nil。

最终验证：

```text
powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts/test_deploy_docker_desktop.ps1
go test ./internal/taskrunner ./internal/server ./cmd/mochat-go ./internal/migration ./cmd/mochat-migrate ./scripts/productionbuild -count=1
go test ./... -count=1
go vet ./...
git diff --check
```

结果全部退出码 `0`。真实 controlled migration/备份恢复、真实 Docker image build、真实 Redis/MySQL 故障、Provider 与生产继续明确 SKIP；未修改项目进度文档、未 reset/clean、未触碰命名卷。

## Reviewer 第三轮复审修复（2026-08-29）

### RED 与根因

- periodic 首次执行 RED：`go test ./internal/taskrunner -run TestRequiredTasksReadyUsesCurrentFirstRunBudget -count=1` 最初失败。根因是 readiness 先应用 startup grace，且只有 `LastSuccessAt` 非空时才检查 `CurrentExecutionStartedAt`；首次合法长任务会在自己的 30 分钟预算内被 30 秒启动宽限错误摘流。
- MySQL post-start RED：新增 DSN 为空/连接打开失败清理测试时，`initializeRuntimeMySQLStore` 尚不存在而编译失败。根因是 lazy `getMySQLStore` 直接 fatal，worker 启动后的 API module 构建仍可间接触发 `os.Exit`，跳过 root cancel 与有界 wait。
- 部署链审计确认旧检查点只打印维护命令，没有真正 quiesce app；备份加密 key 也依赖外部偶然注入，没有默认持久 secret。这会让维护写入与在线请求并发，或在维护窗口才因空 key 失败。

### GREEN、故障注入与部署顺序

- `CurrentExecutionStartedAt` 现在优先于 startup grace：只要当前 tick 已开始，就按显式 `MaxRunDuration` 或计算预算判断。测试证明首次 30 分钟任务在 30 秒后仍 ready，超过 30 分钟变 stale；普通 3 秒预算任务超过阈值同样摘流。
- lazy MySQL opener 改为返回 error，并在 worker 启动后的唯一初始化点设置 `runtimeErr` 后 return。DSN 空与 open failure 注入均证明 defer 会先取消 root、再等待 worker；源码审计同时禁止 post-start 范围调用 fatal compatibility getter、`fatal/fatalf/os.Exit`。
- ordinary `controlled_pending` 路径首先执行 `docker compose stop app` 并通过 inspect 确认 stopped；MySQL、Redis 和命名卷不停止。随后仅验证一次性 app 容器获得非空 DSN/备份 key，打印可复制的 `-ApproveControlledMigrations` 恢复命令并非零退出，默认不执行或展开受控写入。
- 显式恢复必须同时提供 `-ApproveControlledMigrations`、已审批 request ID 和维护确认工件。实际链为 `stop/confirm -> backup-create(encrypted=true) -> backup-verify(passed) -> preflight -> 0130 up -> encrypt-credentials -> 0131 cutover -> automatic up -> 清理临时秘密 -> up -d app -> health -> ready`。任一步失败时 app 保持停止，不会把未完成迁移重新接流。
- 本地 secret 初始化新增独立 32-byte CSPRNG `MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY`，持久文件为 `saas-backup-encryption.key`，并用 Windows ACL 限制到当前用户、SYSTEM 与 Administrators。密钥值不进入 argv、日志或 Git；Compose 继续只通过环境注入。fake Docker 证明一次性容器实际收到非空 key，空持久文件在 Docker 操作前 fail-fast。

DryRun 与 fake Docker 都断言完整顺序；fake Docker 还维护 app stopped 状态、解析真实 backup-create/verify 结构化输出，并证明 ordinary checkpoint 不进入 ready wait。真实 controlled migration 没有执行。

### 最终验证与自审

```text
powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts/test_deploy_docker_desktop.ps1
go test ./internal/taskrunner -count=1
go test ./cmd/mochat-go -run 'TestPostWorkerMySQLInitialization|TestNoFatalExit|TestPostWorkerBuildFailure' -count=1
go test ./... -count=1
go vet ./...
git diff --check
```

结果全部退出码 `0`。自审确认：默认路径仅停止 `app`，不执行 controlled write、不停止 MySQL/Redis、不删除卷；恢复路径只有显式授权 flag 才可达；密钥值未出现在命令或测试输出；Task 1 角色职责与 Task 3 callback defer/Redis 恢复合同未改写；未修改 `docs/PROJECT_PROGRESS.zh-CN.md`。

### 本轮 SKIP

- 真实 0130/0131、真实备份创建/恢复验证与真实维护窗口：SKIP，缺少生产授权和审批工件。
- 真实 Docker image/Compose、真实 MySQL/Redis 故障与恢复：SKIP；本轮使用无副作用 DryRun 和隔离 fake Docker 状态机验证控制流。
- 真实 Provider、浏览器、生产部署与命名卷操作：SKIP，均超出 Task 4 授权范围。

## Reviewer 第四轮复审修复（2026-08-29）

### RED 与根因

- quiesce RED：部署测试要求 `stop --timeout 70 app` 时，旧输出仍是 `stop app`。根因是 Compose 默认 stop 期限只有 10 秒，小于 HTTP drain 30 秒与 worker wait 30 秒之和，维护窗口可能在优雅关闭完成前强杀进程。
- finally cleanup RED：fake Docker 在 preflight 失败后保留 maintenance 状态文件，证明旧线性流程跳过了 DSN、WeCom key 与 confirmation 清理。
- HTTP 失败注入 RED：测试以 `-HttpCheckTimeoutSeconds 1` 运行时参数不存在；旧路径也没有可在短时测试中证明 `/healthz`、`/readyz` 失败后再次停服的入口。
- 启动前清理顺序 RED：fake 链最初观察到 `up -d app` 早于 `maintenance_cleanup=done`，说明临时秘密会一直保留到 health/ready 结束。

### GREEN、故障注入与失败状态

- Compose app 新增 `stop_grace_period: 70s`；普通 controlled checkpoint、显式 resume 首次 quiesce 与失败回收统一调用 `docker compose stop --timeout 70 app`，随后用 inspect 确认 stopped。70 秒覆盖 HTTP 30 秒 drain、worker 30 秒等待及调度余量。
- resume 改为单一 `try/catch/finally` 状态机。维护步骤错误记录为 operation failure；finally 无条件以一次性 app 容器执行 `rm -f` 清理三个临时文件。成功路径在启动 app 前先清理一次，finally 再幂等清理，避免 ready 检查期间保留秘密。
- `up -d app` 后直到 container health、`/healthz`、`/readyz` 全部通过前都不标记 completed。任一失败或 cleanup 失败，finally 都再次执行 70 秒 stop 并确认 stopped，再向外返回非零错误；cleanup/stop 自身错误会进入最终错误报告，不被原始错误吞掉。
- fake Docker 维护独立的 app stopped 与 maintenance-files 状态，逐项注入 preflight、0130、0131、automatic up、app start、container health 失败；每项都断言非零退出、临时文件无残留、至少两次 70 秒 stop 且最终 stopped。cleanup 自身失败单独断言会报告并保持 app 停止。
- 测试使用隔离本地 TCP HTTP server 分别返回 `/healthz=503`、`/readyz=503`，证明两个 HTTP 失败点都会清理并再次停服；`HttpCheckTimeoutSeconds` 仅用于缩短确定性的部署检查预算，生产默认仍为 90 秒。
- 现有备份 key/DSN 防泄露断言继续覆盖全部新增输出；清理命令只包含固定卷内路径，不输出临时文件内容。

### 最终验证与边界

```text
powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts/test_deploy_docker_desktop.ps1
go test ./... -count=1
go vet ./...
git diff --check
```

结果全部退出码 `0`。真实 controlled migration、真实备份恢复、真实 Docker Compose 与生产维护窗口继续 SKIP；测试仅使用 DryRun、fake Docker 和本地临时 HTTP server，未触碰命名卷、真实 Provider 或生产数据。Task 1/Task 3 合同未改写，`docs/PROJECT_PROGRESS.zh-CN.md` 未修改。

## Reviewer 第五轮复审修复（2026-08-29）

### RED 与根因

- `TestReadyzPublishesOnlyStableReadinessFields` 首次失败并打印了旧 `/readyz` 响应，其中包含 `background_tasks`、`latest_execution.error`、内部 Redis/MySQL/worker 主机、DSN/路径、PHP upstream 地址和迁移版本。根因不是 readiness probe 自身（其错误文本已不序列化），而是 `/readyz` 先调用完整诊断用的 `s.status()`，再在同一对象上追加 readiness 结果。
- `TestReadinessCheckerReplacesUnstableProbeCode` 首次失败，证明 probe 注册若误把内部地址放入 `Code`，旧 checker 会原样公开。稳定码合同此前只约束 typed failure code，没有约束 probe base code。

### GREEN 与公开合同

- `/readyz` 改用独立 `readinessPayload`，顶层固定只有 `ready` 与 `readiness_checks`；每项固定只有稳定 `code` 与 `ready`。不再调用或复用 `s.status()`，因此后台任务快照、错误文本、内部地址、路径、上游地址、版本和路由诊断均无法进入公开响应。
- standalone 资产状态转为 `compat_assets` bool；兼容拓扑只公开 `compat_source`、`compat_manifest`、`compat_proxy`、`compat_upstream` 四个稳定能力码，不公开实际目录、manifest 路径、probe 文本或 upstream 地址。
- `NewReadinessChecker` 对不符合 `[a-z0-9_]` 的 base code 统一降级为 `dependency_check`，仍执行 probe 并保持失败摘流语义，但不允许误配置内容进入响应。
- 故障注入同时放入 `dial tcp redis.internal:6379`、MySQL DSN、worker `LatestExecution.Error`、私有路径、PHP upstream 与版本字符串；断言故障为 `503/ready=false`、恢复为 `200/ready=true`，响应只有稳定字段且所有敏感片段零命中。`/healthz` 继续只表示进程 live；完整兼容诊断仍保留在既有 `/compat/status`，本任务未扩大其鉴权范围或复用范围。

### 最终验证与边界

```text
go test ./internal/server ./cmd/mochat-go -count=1
go test ./... -count=1
go vet ./...
git diff --check
```

前三项均已确认退出码 `0`；提交前再次执行 `git diff --check`。真实 Provider、Docker、受控迁移和生产部署仍为 SKIP；未修改 Task 1/Task 3 合同、命名卷或 `docs/PROJECT_PROGRESS.zh-CN.md`。
