# Go 工程底座实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在不迁移任何业务逻辑的前提下，建立可启动、可测试、可观测、可安全发布的 MoChat Go 工程底座。

**Architecture:** 新建并行目录 `api-server-go`，实现模块化单体的四个组合根。底座以显式接口和构造函数注入连接 HTTP、MySQL/SQLC、Outbox、Worker/Scheduler、Telemetry 与质量门禁；PHP 继续承载业务流量。

**Tech Stack:** Go（版本由首个任务固定）、`net/http`、chi、MySQL 8、`database/sql`、SQLC、golang-migrate、Redis、`log/slog`、OpenTelemetry、Testcontainers、OpenAPI 3.1、Docker、Kubernetes。

## Global Constraints

- 以 `docs/superpowers/specs/go-foundation/` 的 7 份已批准设计为唯一工程依据。
- Go Module 固定为 `gitee.com/mochat/mochat/api-server-go`。
- 本计划只建设工程底座、示例空模块和测试夹具，不迁移 IAM、客户、群、活动或插件业务逻辑。
- 四个入口固定为 `cmd/api`、`cmd/worker`、`cmd/scheduler`、`cmd/migrate`。
- MySQL 8 是事实源；租户查询必须显式使用 `TenantScope`，跨租户访问必须显式使用 `SystemScope`。
- 异步语义为至少一次投递与消费者幂等；不得宣称恰好一次。
- 所有公开 HTTP 兼容性变化必须通过 PHP/Go 契约差异门禁。
- 核心 domain/application 覆盖率门禁为 80%；不得用整体覆盖率代替。
- 每项任务使用 TDD，先观察失败，再写最小实现；每个任务独立提交。

---

### Task 1: 工程骨架、工具链与依赖规则

**Files:**
- Create: `api-server-go/go.mod`
- Create: `api-server-go/Makefile`
- Create: `api-server-go/tools/tools.go`
- Create: `api-server-go/internal/platform/bootstrap/{module.go,registry.go,registry_test.go}`
- Create: `api-server-go/internal/platform/feature/gate.go`
- Create: `api-server-go/cmd/{api,worker,scheduler,migrate}/main.go`
- Create: `api-server-go/.golangci.yml`
- Create: `api-server-go/architecture.yml`
- Test: `api-server-go/internal/platform/bootstrap/registry_test.go`

**Interfaces:**
- Consumes: 设计 01 的 `Module`、`Registry`、`Plugin`、`FeatureGate`。
- Produces: 冻结后只读的注册表和四个可编译空入口，供后续任务组合。

- [ ] **Step 1: 固定 Go 版本与依赖**

创建 `go.mod`，仅加入 chi、测试断言和后续任务明确需要的底座依赖；运行 `go mod tidy` 后必须无业务包。

- [ ] **Step 2: 写注册表失败测试**

覆盖重复模块、缺失插件依赖、循环依赖、冻结后写入；运行 `go test ./internal/platform/bootstrap -run TestRegistry -v`，预期因类型或实现缺失而失败。

- [ ] **Step 3: 实现最小注册契约**

按设计 01 的完整签名实现 `Module`、`Registry`、`PluginDependency`，错误码固定为 `BOOTSTRAP_DUPLICATE`、`BOOTSTRAP_DEPENDENCY_MISSING`、`BOOTSTRAP_DEPENDENCY_CYCLE`、`BOOTSTRAP_FROZEN`。

- [ ] **Step 4: 建立架构检查**

规则必须拒绝 domain 导入 I/O、跨模块 infrastructure 和模块直连 integrations；运行 `make lint`，预期合法样本通过、三个非法 fixture 分别输出 `ARCH001`。

- [ ] **Step 5: 验证并提交**

运行 `go test ./...`、`make lint`、`go build ./cmd/...`，预期全部退出 0。提交：`feat: bootstrap Go foundation skeleton`。

### Task 2: 类型化配置、生命周期与探针

**Files:**
- Create: `api-server-go/internal/platform/config/{config.go,loader.go,validate.go,config_test.go}`
- Create: `api-server-go/internal/platform/runtime/{lifecycle.go,runner.go,lifecycle_test.go}`
- Create: `api-server-go/internal/platform/health/{check.go,handler.go,handler_test.go}`
- Modify: `api-server-go/cmd/{api,worker,scheduler}/main.go`

**Interfaces:**
- Consumes: Task 1 Registry。
- Produces: `Config`、`Loader`、`Validator`、`Lifecycle`、`Runner`、`HealthCheck`。

- [ ] **Step 1: 写配置表驱动测试**

覆盖默认值、生产缺少 DSN、Duration 越界、Secret 错误不泄密；运行 `go test ./internal/platform/config -v`，预期失败。

- [ ] **Step 2: 实现环境加载与校验**

逐字实现设计 02 第 6 节的键、默认值与范围；禁止运行时热重载。

- [ ] **Step 3: 写生命周期与探针测试**

验证 `starting -> ready -> draining -> stopped`、重复 Drain、Redis 中断只影响 readiness、SIGTERM 排空和截止超时。

- [ ] **Step 4: 实现状态机与三个探针**

实现 `/startupz`、`/readyz`、`/livez`；停机预算 25s，并确保小于 Kubernetes 30s grace period。

- [ ] **Step 5: 验证并提交**

运行 `go test -race ./internal/platform/config ./internal/platform/runtime ./internal/platform/health`。提交：`feat: add runtime configuration and lifecycle`。

### Task 3: HTTP 服务器与兼容契约基座

**Files:**
- Create: `api-server-go/internal/platform/httpctx/context.go`
- Create: `api-server-go/internal/platform/httpserver/{server.go,middleware.go,handler.go,response.go,middleware_test.go}`
- Create: `api-server-go/internal/platform/errors/app_error.go`
- Create: `api-server-go/internal/platform/validation/json.go`
- Create: `api-server-go/api/openapi/foundation.yaml`
- Create: `api-server-go/tests/contract/{normalizer.go,normalizer_test.go}`

**Interfaces:**
- Consumes: Task 1 RouteContributor、Task 2 生命周期。
- Produces: `RequestContext`、`Authenticator`、`Authorizer`、`Handler`、`AppError` 和契约规范化器。

- [ ] **Step 1: 写中间件顺序与上下文测试**

测试恢复、RequestID、Trace、日志、Body、CORS、认证、租户、授权、Handler、指标的固定顺序；预期首次失败。

- [ ] **Step 2: 实现严格请求与错误映射**

实现设计 03 的 8 类错误映射、1MiB Body、未知字段/重复键/尾随内容拒绝、排序白名单和 15s 请求超时。

- [ ] **Step 3: 建立最小 OpenAPI 与差异器**

只声明探针和空示例端点；规范化时间/ID 后比较状态、JSON、分页、错误码和副作用。

- [ ] **Step 4: 安全与幂等测试**

覆盖 TenantID 不可由 body 覆盖、幂等冲突、Webhook 时间窗/重放钩子、日志不含 token/body。

- [ ] **Step 5: 验证并提交**

运行 `go test -race ./internal/platform/http... ./tests/contract/...` 和 OpenAPI lint/breaking diff。提交：`feat: add HTTP compatibility foundation`。

### Task 4: MySQL、SQLC、事务、Migration 与 Outbox

**Files:**
- Create: `api-server-go/sqlc.yaml`
- Create: `api-server-go/internal/platform/database/{scope.go,dbtx.go,transactor.go,transactor_test.go}`
- Create: `api-server-go/db/migrations/20260719000100_create_outbox_events.{up,down}.sql`
- Create: `api-server-go/db/queries/outbox.sql`
- Create: `api-server-go/internal/platform/outbox/store.go`
- Test: `api-server-go/tests/integration/{database_test.go,migration_test.go,outbox_store_test.go}`

**Interfaces:**
- Consumes: Task 2 MySQL 配置。
- Produces: `TenantScope`、`SystemScope`、`DBTX`、`Transactor` 和 Outbox Store。

- [ ] **Step 1: 写 Testcontainers 失败测试**

覆盖缺 Scope、跨租户读取、事务回滚、并发领取和租约过期；首次运行预期因 schema/实现缺失失败。

- [ ] **Step 2: 写并验证 Migration**

逐列实现设计 04 的 `outbox_events` DDL；验证空库 up/down/up 和脱敏基线 up。

- [ ] **Step 3: 实现事务与 SQLC 查询**

默认 READ COMMITTED，嵌套复用 Context 中事务；只有死锁/序列化冲突可在无外部副作用边界重试两次。

- [ ] **Step 4: 实现 Outbox CAS**

仅允许 `pending -> processing -> published` 与 `processing -> pending/dead`；owner/lease 不匹配返回冲突。

- [ ] **Step 5: 验证并提交**

运行 `make generate`、生成差异检查、`go test -race ./internal/platform/database ./tests/integration/...`。提交：`feat: add persistence and outbox store`。

### Task 5: Outbox Relay、Worker 与 Scheduler

**Files:**
- Create: `api-server-go/internal/platform/messaging/{envelope.go,ports.go,retry.go}`
- Create: `api-server-go/internal/platform/outbox/{relay.go,relay_test.go}`
- Create: `api-server-go/internal/platform/worker/{runner.go,idempotency.go,runner_test.go}`
- Create: `api-server-go/internal/platform/scheduler/{registry.go,runner.go,runner_test.go}`
- Create: `api-server-go/internal/integrations/messaging/redis.go`
- Modify: `api-server-go/cmd/{worker,scheduler}/main.go`

**Interfaces:**
- Consumes: Task 2 生命周期、Task 4 Outbox Store。
- Produces: `Envelope`、`Publisher`、`Consumer`、`RetryPolicy`、`DeadLetter`、`Lease`、`ScheduledJob`。

- [ ] **Step 1: 写故障注入测试**

覆盖 Ack 前崩溃、重复消息、租约接管、Redis 中断、毒消息、租户公平、重叠调度和 SIGTERM。

- [ ] **Step 2: 实现版本化信封与重试策略**

实现 256KiB 上限、W3C trace 传播、500ms 指数退避上限 5m、±20% 抖动、最多 8 次。

- [ ] **Step 3: 实现 Relay 与 Worker**

批次 100、租约 30s、单租户每轮不超过 25%、高水位 10,000；Handler 执行前建立 TenantScope 和幂等记录。

- [ ] **Step 4: 实现 Scheduler 基座**

支持 IANA 时区、`forbid-overlap`、`skip/run-once/catch-up`、补跑上限 3 和审计化人工触发。

- [ ] **Step 5: 验证并提交**

运行 `go test -race ./internal/platform/outbox ./internal/platform/worker ./internal/platform/scheduler`。提交：`feat: add asynchronous runtime foundation`。

### Task 6: Telemetry、审计与安全基线

**Files:**
- Create: `api-server-go/internal/platform/telemetry/{telemetry.go,metrics.go,telemetry_test.go}`
- Create: `api-server-go/internal/platform/logging/{logger.go,redactor.go,redactor_test.go}`
- Create: `api-server-go/internal/platform/audit/{event.go,writer.go,writer_test.go}`
- Create: `api-server-go/internal/platform/security/{headers.go,ratelimit.go,upload.go}`
- Modify: `api-server-go/cmd/{api,worker,scheduler}/main.go`

**Interfaces:**
- Consumes: Task 2/3/5 的状态、请求和事件标识。
- Produces: `Redactor`、`AuditEvent`、`AuditWriter`、`Telemetry` 与批准指标目录。

- [ ] **Step 1: 写泄密与关联测试**

向错误、请求、事件注入 token、手机号、Secret，断言日志/Span 无明文；验证 HTTP 到 Worker 的 trace/correlation 连续。

- [ ] **Step 2: 实现 slog JSON 与 OTel**

实现设计 06 的强制字段、10% 正常采样、100% 错误采样、3s 导出超时和 5s flush。

- [ ] **Step 3: 实现指标与审计端口**

指标只允许批准标签；高风险操作审计失败时返回 503，普通 telemetry 故障不回滚业务。

- [ ] **Step 4: 实现通用安全控制**

安全头、严格 CORS、租户/主体/路由限流键、上传大小/MIME 检测接口、非公开对象策略。

- [ ] **Step 5: 验证并提交**

运行 `go test -race ./internal/platform/{telemetry,logging,audit,security}` 和泄密扫描。提交：`feat: add observability and security baseline`。

### Task 7: 测试基座、质量命令与 CI/CD

**Files:**
- Create: `api-server-go/internal/testkit/{environment.go,fixture.go,clock.go,ids.go}`
- Create: `api-server-go/tests/{contract,integration,e2e}/README.md`
- Modify: `api-server-go/Makefile`
- Create: `.github/workflows/go-foundation.yml`
- Create: `api-server-go/Dockerfile`
- Create: `api-server-go/deployments/kubernetes/{api,worker,scheduler}.yaml`

**Interfaces:**
- Consumes: Task 1～6 的生成器、测试、指标和安全要求。
- Produces: `TestEnvironment`、`FixtureFactory`、固定命令面、CI 证据和签名镜像。

- [ ] **Step 1: 写 testkit 自测**

验证 MySQL 8、Redis、S3 容器、Migration、确定时钟/ID、租户重置和无共享状态并行测试。

- [ ] **Step 2: 完成 Makefile 命令面**

实现 `bootstrap generate lint test-unit test-integration test-contract test-e2e test-race test build image verify`，每个目标失败即非零退出。

- [ ] **Step 3: 实现 CI 作业图**

并行运行生成差异、格式/vet/staticcheck、单元、集成、契约、race、migration、OpenAPI diff、覆盖率、Secret、漏洞、许可证、SBOM、镜像扫描和签名。

- [ ] **Step 4: 实现最小权限镜像与部署模板**

非 root、只读根文件系统、删除 capabilities；API/Worker/Scheduler 独立部署，grace period 30s。

- [ ] **Step 5: 验证并提交**

在干净环境运行 `make verify`、`make image`，验证 SBOM、扫描报告和签名元数据。提交：`ci: establish Go foundation quality gates`。

### Task 8: 联合验收与实施边界确认

**Files:**
- Create: `api-server-go/docs/foundation-acceptance.md`
- Modify: `docs/superpowers/specs/go-foundation/README.md`

**Interfaces:**
- Consumes: Task 1～7 的全部证据。
- Produces: 可审计的底座验收结论；不包含业务迁移授权。

- [ ] **Step 1: 从空环境执行总门禁**

运行 `make bootstrap && make verify && make build && make image`；预期四入口构建成功、全部测试与安全门禁通过。

- [ ] **Step 2: 执行生命周期与故障验收**

启动四入口，验证探针、MySQL/Redis 故障、队列背压、租约接管、SIGTERM 排空和 telemetry flush。

- [ ] **Step 3: 核对设计追踪矩阵**

逐条将设计 01～07 的验收要求映射至命令、测试、CI 作业和制品；任何缺口必须回到所属任务补齐。

- [ ] **Step 4: 确认无业务迁移**

运行 `git diff --name-only <base>...HEAD` 并人工复核：不得包含 IAM、客户、群、活动、插件业务 Handler/Repository 或生产路由切换。

- [ ] **Step 5: 验证并提交**

运行 `make verify`，记录 commit、工具版本、测试数、覆盖率、镜像 digest、SBOM digest。提交：`docs: accept Go engineering foundation`。

## Execution Gate

本计划编写完成不等于授权执行。执行前必须由用户明确选择执行方式，并确认仍以“只建设底座、不迁移 Go 业务代码”为边界。
