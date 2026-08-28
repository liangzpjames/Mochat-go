# 系统关键流程日志完善实施计划

> **供代理执行者使用：** 必须使用 `executing-plans` 按任务逐项实施，并使用 `test-driven-development` 先观察测试正确失败。所有步骤使用复选框跟踪。

**目标：** 在不批量制造日志的前提下，为 MoChat Go 建立结构化、分级、可脱敏、可控增长的关键流程日志体系，并修复空轮询刷屏、任务历史无界增长和会话正文进入错误文本三个根因。

**架构：** 使用标准库 `log/slog` 建立 `internal/observability`，由公共 Handler 统一 JSON/text、等级和脱敏，并用 HTTP 结果中间件覆盖认证、权限、关键写操作和健康失败。关键任务、AI、企微回调与会话存档显式使用结构化事件；legacy `log` 经兼容 writer 接入统一输出。应用只写 stdout，Docker 负责唯一一层文件轮转；后台任务数据库历史采用 periodic 成功压缩和时间保留清理。

**技术栈：** Go 1.26、`log/slog`、`net/http`、`github.com/google/uuid`、`go-sqlmock`、Docker Compose、pnpm 11。

## 全局约束

- 所有用户审阅文档使用中文。
- 不记录密码、Token、Secret、API Key、私钥、Cookie、Authorization、完整消息正文、原始回调正文或个人敏感信息。
- 不记录逐条循环成功、逐行数据、健康成功轮询、正常空队列或 periodic 空结果。
- 新增生产行为必须先有正确失败的测试；配置文件和纯文档变更用静态验证替代 TDD。
- 应用不写日志文件；Compose 的 Docker logging driver 是唯一轮转层，避免重复轮转。
- 只修改本专项文件，不清理、重置或覆盖其他分支、worktree 和用户文件。

---

### 任务 1：统一结构化日志与脱敏底座

**文件：**

- 新建：`internal/observability/logging.go`
- 新建：`internal/observability/logging_test.go`

**接口：**

- 产出：`Config`、`New(Config) (*slog.Logger, error)`、`ConfigureFromEnv(service string) (*slog.Logger, error)`、`SanitizeText(string) string`、`BridgeStandardLog(*slog.Logger)`。
- 约定环境：`MOCHAT_LOG_LEVEL=debug|info|warn|error`、`MOCHAT_LOG_FORMAT=json|text`、`MOCHAT_LOG_SOURCE=0|1`。

- [x] **步骤 1：写等级、格式和脱敏 RED 测试**

```go
func TestNewEmitsConfiguredJSONAndRedactsSensitiveValues(t *testing.T) {
	var output bytes.Buffer
	logger, err := New(Config{Level: "debug", Format: "json", Output: &output, Service: "test"})
	if err != nil { t.Fatal(err) }
	logger.Info("provider token=plain-token", "event", "redaction_check", "api_key", "plain-key", "configured", true)
	line := output.String()
	for _, forbidden := range []string{"plain-token", "plain-key"} {
		if strings.Contains(line, forbidden) { t.Fatalf("log leaked %q: %s", forbidden, line) }
	}
	for _, required := range []string{`"level":"INFO"`, `"event":"redaction_check"`, `"service":"test"`, `[REDACTED]`} {
		if !strings.Contains(line, required) { t.Fatalf("log missing %q: %s", required, line) }
	}
}
```

- [x] **步骤 2：运行测试并确认因 `New` 尚不存在而失败**

运行：`go test ./internal/observability -run TestNewEmitsConfiguredJSONAndRedactsSensitiveValues -count=1`

预期：FAIL，错误包含 `undefined: New`。

- [x] **步骤 3：实现最小 Logger、redacting Handler 和文本脱敏**

```go
type Config struct {
	Level string
	Format string
	Output io.Writer
	Service string
	AddSource bool
}

func New(cfg Config) (*slog.Logger, error) {
	level, err := parseLevel(cfg.Level)
	if err != nil { return nil, err }
	if cfg.Output == nil { cfg.Output = os.Stdout }
	options := &slog.HandlerOptions{Level: level, AddSource: cfg.AddSource}
	var handler slog.Handler
	switch strings.ToLower(strings.TrimSpace(cfg.Format)) {
	case "", "json": handler = slog.NewJSONHandler(cfg.Output, options)
	case "text": handler = slog.NewTextHandler(cfg.Output, options)
	default: return nil, fmt.Errorf("unsupported log format %q", cfg.Format)
	}
	handler = &redactingHandler{next: handler}
	return slog.New(handler).With("service", strings.TrimSpace(cfg.Service)), nil
}
```

`redactingHandler.Handle` 必须重建 `slog.Record`，对 message 调用 `SanitizeText`，对 Attr 递归处理；敏感字符串/对象替换为 `[REDACTED]`，布尔型 `*_configured` 保留。`SanitizeText` 覆盖 Bearer、敏感 `key=value`、URL userinfo、JWT 和 PEM 私钥块。

- [x] **步骤 4：补 legacy writer RED 测试并实现等级桥接**

```go
func TestBridgeStandardLogRecognizesExplicitLevelPrefix(t *testing.T) {
	var output bytes.Buffer
	logger, _ := New(Config{Level: "debug", Format: "json", Output: &output, Service: "test"})
	BridgeStandardLog(logger)
	log.Print("level=WARN event=queue_retry queue item will retry token=hidden")
	line := output.String()
	if !strings.Contains(line, `"level":"WARN"`) || !strings.Contains(line, `"event":"queue_retry"`) {
		t.Fatalf("bridged log = %s", line)
	}
	if strings.Contains(line, "hidden") { t.Fatalf("bridged log leaked secret: %s", line) }
}
```

兼容 writer 解析开头的 `level=` 和 `event=`；无显式前缀的 legacy 调用按 INFO 进入同一 Handler。测试结束必须恢复原 standard logger output/flags，避免污染其他测试。

- [x] **步骤 5：运行包测试**

运行：`go test ./internal/observability -count=1`

预期：PASS，且测试输出不含敏感夹具值。

---

### 任务 2：HTTP 关键结果日志、请求 ID 与启动集成

**文件：**

- 新建：`internal/observability/http.go`
- 新建：`internal/observability/http_test.go`
- 新建：`cmd/mochat-go/logging.go`
- 修改：`cmd/mochat-go/main.go`

**接口：**

- 消费：任务 1 的 `*slog.Logger`。
- 产出：`HTTPMiddleware(logger *slog.Logger) func(http.Handler) http.Handler`、`RequestID(context.Context) string`。
- `cmd/mochat-go/logging.go` 产出 `configureLogging`、`debugf`、`fatal`、`fatalf`。

- [x] **步骤 1：写 HTTP 成功/拒绝/失败/健康 RED 测试**

```go
func TestHTTPMiddlewareLogsOnlyCriticalOutcomes(t *testing.T) {
	var output bytes.Buffer
	logger, _ := New(Config{Level: "debug", Format: "json", Output: &output, Service: "test"})
	handler := HTTPMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/healthz": w.WriteHeader(http.StatusOK)
		case "/dashboard/company/employee-sync": w.WriteHeader(http.StatusAccepted)
		case "/dashboard/denied": w.WriteHeader(http.StatusForbidden)
		default: w.WriteHeader(http.StatusInternalServerError)
		}
	}))

	call(t, handler, http.MethodGet, "/healthz", "Bearer never-log")
	if output.Len() != 0 { t.Fatalf("successful health poll logged: %s", output.String()) }
	call(t, handler, http.MethodPost, "/dashboard/company/employee-sync", "Bearer never-log")
	call(t, handler, http.MethodPut, "/dashboard/denied", "Bearer never-log")
	call(t, handler, http.MethodPost, "/dashboard/fail", "Bearer never-log")
	logs := output.String()
	for _, required := range []string{"critical_request_completed", "request_rejected", "critical_request_failed", "request_id", "duration_ms"} {
		if !strings.Contains(logs, required) { t.Fatalf("missing %q: %s", required, logs) }
	}
	if strings.Contains(logs, "never-log") { t.Fatalf("authorization leaked: %s", logs) }
}
```

- [x] **步骤 2：运行测试并确认 HTTP 中间件缺失导致失败**

运行：`go test ./internal/observability -run TestHTTPMiddleware -count=1`

预期：FAIL，错误包含 `undefined: HTTPMiddleware`。

- [x] **步骤 3：实现透明 status writer、请求 ID 和日志分类**

`statusWriter` 只记录首个状态码并实现 `Unwrap() http.ResponseWriter`；中间件不得读取 body、query、Cookie 或 Authorization。合法 `X-Request-ID` 必须匹配 `[A-Za-z0-9._:-]{1,128}`，否则生成 UUID。成功 GET/HEAD/OPTIONS 与成功 `/healthz`、`/readyz` 静默；401/403 为 WARN；5xx 为 ERROR；其他关键写请求为 INFO。

- [x] **步骤 4：集成主进程日志并把逐条路由日志降为 DEBUG**

```go
func configureLogging() *slog.Logger {
	logger, err := observability.ConfigureFromEnv("mochat-go")
	if err != nil {
		fmt.Fprintln(os.Stderr, "invalid logging configuration")
		os.Exit(1)
	}
	slog.SetDefault(logger)
	observability.BridgeStandardLog(logger)
	return logger
}

func debugf(format string, args ...any) { slog.Debug(fmt.Sprintf(format, args...)) }
```

`main()` 第一行调用 `configureLogging()`。把 `log.Printf("go ... route enabled..."` 和 `log.Printf("go ... routes enabled..."` 机械替换为 `debugf(...)`；把主进程 `log.Fatal/Fatalf` 替换为记录 `runtime_failed` ERROR 后退出的 `fatal/fatalf`。在最终 `serveHandler` 外包 `observability.HTTPMiddleware`，主、sidebar、operation 三个入口均在成功 bind 后记录 `runtime_listening`。

- [x] **步骤 5：增加监听字段单元回归与静态策略断言并运行主程序相关测试**

运行：`go test ./cmd/mochat-go ./internal/observability ./internal/server -count=1`

预期：PASS；单元测试断言 main/sidebar/operation 三条 INFO 均包含 `listener/listen_addr`，`rg -n 'log\.Printf\("go .*route(s)? enabled' cmd/mochat-go/main.go` 无输出。

---

### 任务 3：迁移命令生命周期日志

**文件：**

- 修改：`cmd/mochat-migrate/main.go`
- 新建：`cmd/mochat-migrate/main_test.go`

**接口：**

- 产出：`runMigrationCommand(ctx context.Context, options migrationCommandOptions, runner migrationRunner, logger *slog.Logger, output io.Writer) error`。
- runner 接口只包含 `Apply`、`BaselineComposeInit`、`Status`、`Baseline`、`RollbackLast`。

- [x] **步骤 1：写成功与失败日志 RED 测试**

```go
func TestRunMigrationCommandLogsLifecycleWithoutDSN(t *testing.T) {
	var logs bytes.Buffer
	logger, _ := observability.New(observability.Config{Format: "json", Level: "info", Output: &logs, Service: "test"})
	runner := &fakeMigrationRunner{status: []migration.StatusItem{{Migration: migration.Migration{Version: "0001"}, State: "applied_now"}}}
	if err := runMigrationCommand(context.Background(), migrationCommandOptions{Action: "apply"}, runner, logger, io.Discard); err != nil { t.Fatal(err) }
	text := logs.String()
	for _, required := range []string{"migration_started", "migration_completed", `"applied":1`} {
		if !strings.Contains(text, required) { t.Fatalf("missing %q: %s", required, text) }
	}
	if strings.Contains(text, "user:password@tcp") { t.Fatalf("DSN leaked: %s", text) }
}

type fakeMigrationRunner struct {
	status []migration.StatusItem
	err error
}

func (f *fakeMigrationRunner) Apply(context.Context) ([]migration.StatusItem, error) { return f.status, f.err }
func (f *fakeMigrationRunner) BaselineComposeInit(context.Context) ([]migration.StatusItem, error) { return f.status, f.err }
func (f *fakeMigrationRunner) Status(context.Context) ([]migration.StatusItem, error) { return f.status, f.err }
func (f *fakeMigrationRunner) Baseline(context.Context) ([]migration.StatusItem, error) { return f.status, f.err }
func (f *fakeMigrationRunner) RollbackLast(context.Context) (string, error) { return "", f.err }
```

- [x] **步骤 2：运行 RED 并实现可测试的命令边界**

运行：`go test ./cmd/mochat-migrate -count=1`

预期：先 FAIL（缺少 `runMigrationCommand`），实现后 PASS。开始事件 INFO；失败事件 ERROR，字段含 action/result/error_code/duration_ms；成功事件 INFO，字段含总数、applied_now 数和 rolled_back 版本。DSN 只用于连接，不作为字段或 error 文本输出。

---

### 任务 4：后台任务等级、空 tick 静默与历史保留

**文件：**

- 修改：`internal/taskrunner/taskrunner.go`
- 修改：`internal/taskrunner/taskrunner_test.go`
- 修改：`internal/taskrunner/sql_recorder.go`
- 新建：`internal/taskrunner/sql_recorder_test.go`
- 修改：`cmd/mochat-go/main.go`

**接口：**

- `taskrunner.New(*slog.Logger)`、`PeriodicConfig.Logger *slog.Logger`、`NewSQLRecorder(*sql.DB, *slog.Logger, ...SQLRecorderOption)`。
- 新增：`WithHistoryRetention(retention time.Duration, cleanupInterval time.Duration, batchSize int)`。
- 新增：`latestPeriodicSuccessExecutionID(taskName string) string`。

- [x] **步骤 1：改写 periodic RED 测试锁定“成功压缩、失败唯一、空成功不记 INFO”**

```go
func TestPeriodicCompactsSuccessAndKeepsUniqueFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	recorder := &memoryRecorder{executions: make(chan ExecutionSnapshot, 8)}
	var calls atomic.Int32
	run := Periodic(PeriodicConfig{Name: "cron-a", Interval: time.Millisecond, RunOnStart: true, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}, func(context.Context) error {
		call := calls.Add(1)
		if call == 2 { return errors.New("tick failed") }
		if call == 3 { cancel() }
		return nil
	})
	done := make(chan error, 1)
	go func() { done <- run(withTaskRuntime(ctx, "cron-a", "run-1", recorder)) }()
	first := <-recorder.executions
	second := <-recorder.executions
	third := <-recorder.executions
	if first.Status != StatusSucceeded || third.Status != StatusSucceeded || first.ExecutionID != third.ExecutionID {
		t.Fatalf("success executions = %+v %+v", first, third)
	}
	if first.ExecutionID != latestPeriodicSuccessExecutionID("cron-a") {
		t.Fatalf("success execution id = %q", first.ExecutionID)
	}
	if second.Status != StatusFailed || second.ExecutionID == first.ExecutionID {
		t.Fatalf("failed execution = %+v", second)
	}
	if err := <-done; !errors.Is(err, context.Canceled) { t.Fatalf("run error = %v", err) }
}

func TestPeriodicDoesNotLogSuccessfulEmptyTick(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&output, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx, cancel := context.WithCancel(context.Background())
	run := Periodic(PeriodicConfig{Name: "cron-empty", Interval: time.Hour, RunOnStart: true, Logger: logger}, func(context.Context) error {
		cancel()
		return nil
	})
	if err := run(ctx); !errors.Is(err, context.Canceled) { t.Fatalf("run error = %v", err) }
	if output.Len() != 0 { t.Fatalf("successful empty tick logged at INFO: %s", output.String()) }
}
```

运行：`go test ./internal/taskrunner -run 'TestPeriodicCompacts|TestPeriodicDoesNotLogSuccessfulEmptyTick' -count=1`

预期：FAIL，现实现会记录 running + succeeded 且每次 ID 唯一。

- [x] **步骤 2：实现 taskrunner 显式等级与成功压缩**

`Periodic` 不再持久化 running tick；完成后成功与失败分别使用稳定 latest ID。成功 tick 只在 DEBUG 记录，失败用 ERROR 且连续失败每分钟最多一次，context canceled 不记录 ERROR。长驻任务启动/停止 INFO，panic/非取消错误 ERROR，字段含 task_name/run_id/execution_id/result/duration_ms。

- [x] **步骤 3：写 SQL 清理 RED 测试**

```go
func TestSQLRecorderCleansExpiredHistoryWithoutBlockingRecord(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	mock.ExpectExec("DELETE FROM mochat_go_background_task_executions").WithArgs(sqlmock.AnyArg(), 10000).WillReturnResult(sqlmock.NewResult(0, 3))
	mock.ExpectExec("DELETE FROM mochat_go_background_task_runs").WithArgs(sqlmock.AnyArg(), 10000).WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("INSERT INTO mochat_go_background_task_executions").WillReturnResult(sqlmock.NewResult(1, 1))
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	recorder := NewSQLRecorder(db, logger, WithHistoryRetention(14*24*time.Hour, time.Hour, 10000))
	snapshot := ExecutionSnapshot{ExecutionID: "cron-a-periodic-latest-success", TaskName: "cron-a", Kind: ExecutionKindPeriodicTick, Status: StatusSucceeded, StartedAt: "2026-08-28T10:00:00Z", StoppedAt: "2026-08-28T10:00:01Z"}
	if err := recorder.RecordTaskExecution(context.Background(), snapshot); err != nil { t.Fatal(err) }
	if err := mock.ExpectationsWereMet(); err != nil { t.Fatal(err) }
}
```

- [x] **步骤 4：实现按小时、分批、非阻塞历史清理**

`RecordTaskExecution` 先完成当前 upsert，再在本进程首次调用和距上次尝试达到 cleanupInterval 时启动单实例异步清理；清理使用独立 30 秒 context，每张表每次最多 batchSize。清理 error 记录 `task_history_cleanup_failed` WARN，不占用当前执行记录的 context 或返回延迟；无效 retention/batch 在构造时回退设计默认值。当前状态表不删除。

- [x] **步骤 5：在主进程启用 14 天/每小时/10,000 条默认值并运行测试**

运行：`go test ./internal/taskrunner ./cmd/mochat-go -count=1`

预期：PASS；taskrunner 日志测试确认成功空 tick 没有 INFO。

---

### 任务 5：会话存档日志根因与正文泄露修复

**文件：**

- 修改：`internal/dashboard/work_message_archive_sync_cron.go`
- 修改：`internal/dashboard/work_message_archive_sync_cron_test.go`
- 修改：`cmd/mochat-go/main.go`

**接口：**

- `NewWorkMessageArchiveSyncCron(..., logger *slog.Logger)`。
- 新增内部 `logArchiveSyncResult`，只记录非空或失败结果。

- [x] **步骤 1：写空轮询、非空、失败和脱敏 RED 测试**

```go
func TestWorkMessageArchiveSyncCronKeepsEmptyPollSilent(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	cron := NewWorkMessageArchiveSyncCron(&fakeWorkMessageArchiveSyncStore{}, &fakeWorkMessageArchiveSyncClient{}, logger)
	if err := cron.RunOnce(context.Background()); err != nil { t.Fatal(err) }
	if logs.Len() != 0 { t.Fatalf("empty poll logged: %s", logs.String()) }
}

func TestWorkMessageArchiveBridgeErrorsDoNotContainResponseOrMessageBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("secret response body"))
	}))
	defer server.Close()
	client := NewWorkMessageArchiveBridgeClient(server.URL, "bridge-secret")
	_, err := client.FetchWorkMessageArchive(context.Background(), WorkMessageArchiveCorp{CorpID: 7, WXCorpID: "ww-test"}, 0, 10)
	if err == nil || !strings.Contains(err.Error(), "HTTP 502") || strings.Contains(err.Error(), "secret response body") {
		t.Fatalf("bridge error = %v", err)
	}
	_, err = parseWorkMessageArchiveBridgeMessage(json.RawMessage(`{"text":{"content":"private conversation"}}`))
	if err == nil || strings.Contains(err.Error(), "private conversation") || strings.Contains(err.Error(), `"text"`) {
		t.Fatalf("parse error leaked message: %v", err)
	}
}
```

- [x] **步骤 2：运行 RED**

运行：`go test ./internal/dashboard -run 'TestWorkMessageArchiveSyncCronKeepsEmptyPollSilent|TestWorkMessageArchiveBridgeErrorsDoNotContainResponseOrMessageBody' -count=1`

预期：FAIL；当前空结果会打印汇总，错误包含外部 body/原始 JSON。

- [x] **步骤 3：实现结构化汇总和安全错误**

空 periodic 结果完全静默；有 fetched/inserted/failed 时记录 `archive_sync_completed` INFO 或 `archive_sync_failed` ERROR。periodic 连续失败共享限流/恢复状态；主动 `RunCorp` 的失败每次独立记录且不读取或改变 periodic 状态，未启用只记 DEBUG。bridge 非 2xx 只返回形如 `work message archive bridge returned HTTP 502` 的受控错误；缺少 msgid 返回固定 `work message archive message is missing msgid`，不得拼接 body/JSON。

- [x] **步骤 4：运行回归**

运行：`go test ./internal/dashboard -run 'WorkMessageArchive' -count=1`

预期：PASS，日志无 Secret、私钥或消息正文。

---

### 任务 6：AI Provider 与企微第三方回调关键事件

**文件：**

- 修改：`internal/modules/providers/ai/openai/openai.go`
- 修改：`internal/modules/providers/ai/openai/openai_test.go`
- 修改：`internal/wecomsuitecallback/handler.go`
- 修改：`internal/wecomsuitecallback/handler_test.go`
- 修改：`cmd/mochat-go/main.go`

**接口：**

- `openai.Config.Logger *slog.Logger`。
- `wecomsuitecallback.Config.Logger *slog.Logger`。

- [x] **步骤 1：写 AI 日志 RED 测试**

```go
func TestChatLogsOutcomeWithoutPromptCredentialOrResponse(t *testing.T) {
	var logs bytes.Buffer
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"private response"}}]}`))
	}))
	defer server.Close()
	logger := slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	client, _ := New(Config{BaseURL: server.URL, APIKey: "top-secret", Model: "model-a", Client: server.Client(), Logger: logger})
	_, _ = client.Chat(context.Background(), providers.ChatRequest{Prompt: "private prompt"})
	text := logs.String()
	for _, required := range []string{"ai_provider_call_completed", "provider", "model", "duration_ms"} {
		if !strings.Contains(text, required) { t.Fatalf("missing %q: %s", required, text) }
	}
	for _, forbidden := range []string{"top-secret", "private prompt", "private response"} {
		if strings.Contains(text, forbidden) { t.Fatalf("leaked %q: %s", forbidden, text) }
	}
}
```

- [x] **步骤 2：实现 AI 成功/失败单一边界事件**

成功 INFO；未配置、网络/4xx/超时 WARN；5xx 或响应解析/无内容 ERROR。字段只含 provider、model、request_id（HTTP 链路存在时）、result、error_code、status_code、duration_ms，不含请求/响应/Base URL/API Key。

- [x] **步骤 3：写回调验签/重复/授权 RED 测试**

扩展现有 Handler fake：为非法时间窗、解密失败、重复 claim、交换失败、保存失败和 create_auth 成功注入 JSON logger。断言事件分别为 `wecom_callback_rejected`、DEBUG 重复、`wecom_authorization_exchange_failed`、`wecom_authorization_saved`，并全局搜索日志不得出现 ticket、auth code、permanent code、signature 或加密 XML。

- [x] **步骤 4：实现回调结构化事件并运行测试**

运行：`go test ./internal/modules/providers/ai/openai ./internal/wecomsuitecallback -count=1`

预期：PASS；每个失败只在最终边界记录一次。

---

### 任务 7：Docker 轮转、部署说明和日志策略门禁

**文件：**

- 修改：`deploy/standalone/docker-compose.yml`
- 修改：`deploy/standalone/README.md`
- 新建：`scripts/check_logging_policy.mjs`
- 新建：`scripts/check_logging_policy.test.mjs`
- 修改：`package.json`

**接口：**

- Compose 扩展：`x-mochat-logging: &mochat-logging`。
- npm script：`check:logging`。

- [x] **步骤 1：写静态策略 RED 测试**

```js
test('logging policy rejects noisy and sensitive regressions', () => {
  const result = checkLoggingPolicy(repoRoot)
  assert.deepEqual(result.violations, [])
})
```

门禁扫描：主程序不得残留 INFO 路由逐条日志；archive error 不得拼外部 body/原始 JSON；新增 observability 调用不得以 password/token/secret/cookie/authorization/private_key/content/prompt/body 作为字符串字段；Compose 必须有 driver/max-size/max-file。

- [x] **步骤 2：运行 RED**

运行：`node --test scripts/check_logging_policy.test.mjs`

预期：FAIL，至少指出 Compose 缺少 rotation options。

- [x] **步骤 3：实现 Compose 唯一轮转层**

```yaml
x-mochat-logging: &mochat-logging
  driver: json-file
  options:
    max-size: "${MOCHAT_DOCKER_LOG_MAX_SIZE:-20m}"
    max-file: "${MOCHAT_DOCKER_LOG_MAX_FILES:-5}"
```

app、archive-bridge、archive-simulator、mysql、redis 添加 `logging: *mochat-logging`；app/bridge/simulator 环境添加 `MOCHAT_LOG_LEVEL`、`MOCHAT_LOG_FORMAT`、`MOCHAT_LOG_SOURCE`。不得新增应用日志 volume。

- [x] **步骤 4：补中文部署说明**

README 明确等级边界、字段、默认 JSON/INFO、Docker 100 MiB/容器上限、stdout 采集、平台与应用不得重复轮转、DEBUG 临时启用方式、后台任务 14 天保留，以及按 event/request_id/task_name/run_id 排障方法。

- [x] **步骤 5：运行策略与 Compose 验证**

运行：`node --test scripts/check_logging_policy.test.mjs && node scripts/check_logging_policy.mjs && docker compose -f deploy/standalone/docker-compose.yml config --quiet`

预期：全部退出码 0。

---

### 任务 8：端到端验证、证据、复审与提交

**文件：**

- 新建：`docs/verification/2026-08-28-critical-flow-logging.zh-CN.md`
- 更新：`docs/superpowers/plans/2026-08-28-critical-flow-logging.zh-CN.md`（勾选完成项并记录与计划差异）

**接口：** 无新生产接口；产出可复核验证证据和最终提交。

- [x] **步骤 1：格式、静态分析和全量自动化验证**

运行：

```text
gofmt -w internal/observability/logging.go internal/observability/logging_test.go internal/observability/http.go internal/observability/http_test.go cmd/mochat-go/logging.go cmd/mochat-go/main.go cmd/mochat-migrate/main.go cmd/mochat-migrate/main_test.go internal/taskrunner/taskrunner.go internal/taskrunner/taskrunner_test.go internal/taskrunner/sql_recorder.go internal/taskrunner/sql_recorder_test.go internal/dashboard/work_message_archive_sync_cron.go internal/dashboard/work_message_archive_sync_cron_test.go internal/modules/providers/ai/openai/openai.go internal/modules/providers/ai/openai/openai_test.go internal/wecomsuitecallback/handler.go internal/wecomsuitecallback/handler_test.go
go test ./...
go vet ./...
pnpm lint
pnpm typecheck
pnpm test
pnpm build
pnpm check:logging
git diff --check
```

预期：本专项相关命令退出码 0；若仓库既有命令失败，必须记录与基线一致的精确错误，不得宣称通过。

- [x] **步骤 2：代表性运行日志验证**

用 Go 测试辅助进程或本地二进制实际读取 stdout，覆盖成功写、401/403、5xx、配置缺失、AI 成功/失败、回调失败、periodic 失败、archive 空轮询。把每个场景的 event/level/必要字段/禁止字段结果写入验证文档。

- [x] **步骤 3：Docker 构建和运行验证**

运行：

```text
docker compose -f deploy/standalone/docker-compose.yml config --quiet
docker build -t mochat-go:critical-flow-logging .
docker compose --env-file <专项临时 env> --profile app -p mochat-critical-flow-logging-final -f deploy/standalone/docker-compose.yml up -d --build app
docker inspect mochat-critical-flow-logging-final-app-1 --format '{{json .HostConfig.LogConfig}}'
docker logs mochat-critical-flow-logging-final-app-1
curl.exe --fail http://127.0.0.1:28080/healthz
curl.exe --fail http://127.0.0.1:28080/readyz
```

使用独立 Compose project name 和新建卷，禁止复用或删除用户现有卷。验证 app `/healthz`、`/readyz`、stdout JSON、轮转 options、健康成功不刷日志、配置缺失错误脱敏。验证完成只移除本专项明确创建的容器/网络/卷，并在删除前核对 project label 和绝对目标。

- [x] **步骤 4：需求覆盖复审**

逐条对照设计 11 节、用户 7 项必需条件和最终交付清单；在验证文档写主要流程覆盖矩阵、等级/字段/归档说明、脱敏与噪声结果、Docker 证据、已知边界和既有基线问题。

- [x] **步骤 5：请求代码复审并修正**

使用 `requesting-code-review` 检查需求符合性与代码质量；主代理重新读取 diff、运行针对性测试，不直接相信复审结论。发现缺陷时按 TDD 修复。

- [x] **步骤 6：最终提交**

运行：

```text
git status --short
git diff --stat main...HEAD
git diff --check main...HEAD
git add -- cmd/mochat-go cmd/mochat-migrate internal/observability internal/taskrunner internal/dashboard/work_message_archive_sync_cron.go internal/dashboard/work_message_archive_sync_cron_test.go internal/modules/providers/ai/openai internal/wecomsuitecallback deploy/standalone/docker-compose.yml deploy/standalone/README.md scripts/check_logging_policy.mjs scripts/check_logging_policy.test.mjs package.json docs/superpowers/specs/2026-08-28-critical-flow-logging-design.zh-CN.md docs/superpowers/plans/2026-08-28-critical-flow-logging.zh-CN.md docs/verification/2026-08-28-critical-flow-logging.zh-CN.md
git commit -m "feat: 完善系统关键流程日志"
git rev-parse HEAD
```

预期：提交位于 `chore/critical-flow-logging-20260828`，输出精确 SHA；主检出和其他 worktree 未被修改。

## 实施结果与计划差异

1. **periodic 连续失败也改为稳定记录。** 原计划只压缩成功、失败使用唯一 execution。真实 Docker 运行发现迁移缺失可让 2 秒任务同时造成无界 ERROR 和执行历史，因此最终采用每任务一条 `periodic-latest-failure`，失败日志首轮立即输出、连续失败每分钟最多一次、恢复 INFO 一次；测试覆盖压缩、限流和恢复。
2. **conversation export worker 默认关闭。** 全新 standalone 数据库没有 `0144` 表时，原默认值会每 2 秒执行并失败。Compose 与 `.env.example` 改为默认 `0`，必须在迁移成功并确认表存在后显式启用；策略测试防止回归。
3. **启动明细进一步降噪。** 除约 565 条路由注册外，认证 resolver、能力开关、worker/cron 配置明细也统一降为 DEBUG。INFO 启动只保留 `runtime_starting`、`runtime_listening` 和实际启动的后台任务生命周期。
4. **Docker 验证使用 standalone 文件和独立端口。** 最终复跑 project 为 `mochat-critical-flow-logging-final`，端口为 28080/28081/28082、23316、36389；只使用专项容器、网络、卷，未复用现有 `mochat-go-desktop` 环境。
5. **就绪检查边界如实保留。** 当前 `/readyz` 在运行中不会主动探测数据库，暂停 MySQL 不能制造 503；本专项通过 HTTP 中间件测试验证 503 的 `readiness_failed`，Docker 只证明现有 ready/health 成功且静默，不把 200 误称为数据库可用证明。
6. **复审问题以根因方式收敛。** 三个监听端口复用同一 HTTP 中间件，并在各自成功 bind 后统一记录带 `listener=main|sidebar|operation` 的 listening；HTTP panic 受控返回 500；archive 自己限流并抑制 Periodic 重复结果，同时让手工同步完全独立于 periodic 失败/恢复状态；SQL 清理在当前记录提交后使用独立 context 异步执行；AI 与 periodic 补齐 request/execution 关联字段；回调业务对象统一使用 `object_type/object_id`。

## 实施文档自审

- [x] 每个任务都对应设计中的等级、字段、敏感信息、噪声或部署要求，并按 RED→GREEN 明确测试与实现顺序。
- [x] 计划只修改已由代码确认的流程，不要求为不存在的链路凭空新增日志。
- [x] archive、Provider、callback、HTTP、taskrunner 和迁移测试均包含“为什么记录、记录意味着什么、下一步检查什么”的事件文本或断言。
- [x] Compose 与 README 工作项覆盖 stdout/stderr、20m×5、生产采集、单层轮转和后台历史保留，最终 Docker project 与清理边界已回填。
- [x] 空轮询、失败重试、稳定 execution、conversation export 迁移顺序和 archive 双重 ERROR 均有根因修复及回归证据。
- [x] 全量 Go/前端/策略/构建、代表性日志读取、Docker 实跑、脱敏扫描、噪声计数和既有 docs 基线失败均在验证文档如实记录。
