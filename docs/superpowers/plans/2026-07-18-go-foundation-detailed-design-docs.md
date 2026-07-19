# Go 工程底座详细设计文档实施计划

> **供智能代理执行：** 必须使用 `subagent-driven-development`（推荐）或 `executing-plans` 技能逐项执行本计划。所有步骤使用复选框（`- [ ]`）跟踪。

**目标：** 产出并确认 7 份可直接指导实施的 MoChat Go 工程底座详细设计文档。

**架构：** 文档按稳定工程能力拆分，而不是按运行进程拆分。文档 01 统一定义包边界及共享类型归属；文档 02～04 分别细化独立的平台能力；文档 05～07 只引用上游定义，不得重复定义。

**技术栈：** Go、`net/http`、`chi`、MySQL 8、`database/sql`、SQLC、`golang-migrate`、Redis、`log/slog`、OpenTelemetry、Testcontainers、OpenAPI 3.1、Kubernetes。

## 全局约束

- 外部 HTTP API 必须保持现有路径、方法、请求字段、响应结构、错误码、鉴权方式、Webhook 协议和可观察副作用。
- 目标架构为模块化单体，API、Worker、Scheduler 和 Migrate 使用独立运行入口。
- 核心业务模块完成后才能整体切换；PHP 与 Go 不得长期共同写入。
- MySQL 8 是业务事实源；Redis 不得保存无法恢复的唯一业务事实。
- 模块只能通过公开 Application API、领域事件和专用只读接口协作。
- 插件使用编译期注册和租户功能开关，禁止使用 Go 原生运行时插件。
- 每份详细设计必须提供精确包路径、Go 类型签名、配置键、数据结构、默认值、状态转换、测试场景和验收结果。
- 存在未解决问题或占位内容的文档不得标记为“已确认”。

---

## 计划文件清单

| 文件 | 职责 |
| --- | --- |
| `docs/superpowers/specs/go-foundation/01-project-foundation-design.md` | Go Module、包边界、组合根、依赖注入、模块接口、插件注册和依赖检查 |
| `docs/superpowers/specs/go-foundation/02-runtime-lifecycle-design.md` | 类型化配置、进程生命周期、健康探针和优雅停机 |
| `docs/superpowers/specs/go-foundation/03-http-api-compatibility-design.md` | 路由、中间件、请求上下文、校验、错误、兼容映射和 OpenAPI 流程 |
| `docs/superpowers/specs/go-foundation/04-data-persistence-design.md` | MySQL、SQLC、租户作用域、事务、Repository、Migration 和 Outbox 存储 |
| `docs/superpowers/specs/go-foundation/05-async-processing-design.md` | 事件信封、Outbox Relay、队列端口、Worker、重试、死信和 Scheduler |
| `docs/superpowers/specs/go-foundation/06-observability-security-design.md` | 日志、追踪、指标、审计、Secret、限流和安全基线 |
| `docs/superpowers/specs/go-foundation/07-engineering-quality-design.md` | 测试结构、Testcontainers、契约比对、代码生成、CI/CD 和本地环境 |

## 通用校验命令

每个任务完成后，在 `D:\workspace\Mochat` 运行：

```powershell
$files = Get-ChildItem docs/superpowers/specs/go-foundation -Filter '*-design.md'
rg -n 'TBD|TODO|FIXME|PLACEHOLDER|待定|稍后实现|类似上述' $files.FullName
git diff --check -- $files.FullName
$files | ForEach-Object { rg -n '^## (1[0-5]|[1-9])\.' $_.FullName }
```

预期结果：

- 占位内容扫描无匹配；明确声明“禁止占位内容”的规则文本除外。
- `git diff --check` 无输出。
- 每份文档按顺序包含 15 个强制章节。

### 任务 1：工程骨架与依赖规则

**文件：**

- 创建：`docs/superpowers/specs/go-foundation/01-project-foundation-design.md`
- 阅读：`docs/superpowers/specs/2026-07-17-go-backend-architecture-design.md`
- 阅读：`docs/superpowers/specs/2026-07-18-go-foundation-detailed-design-docs-design.md`
- 阅读：`api-server/composer.json`
- 阅读：`api-server/config/container.php`
- 阅读：`api-server/config/autoload/dependencies.php`
- 阅读：所有 `api-server/app/core/*/composer.json`
- 阅读：所有 `api-server/plugin/mochat/*/composer.json`

**接口关系：**

- 输入：上位架构已确认的模块化单体边界及 10 个目标领域。
- 输出：标准 Go Module 路径、包结构、组合根契约、`Module`、`Registry`、`Plugin`、`FeatureGate` 和架构检查规则，供任务 2～7 引用。

- [ ] **步骤 1：盘点现有模块和插件边界**

运行：

```powershell
Get-ChildItem api-server/app/core -Directory | Select-Object -ExpandProperty Name
Get-ChildItem api-server/plugin/mochat -Directory | Select-Object -ExpandProperty Name
rg -n '"autoload"|"require"|MoChat\\App|MoChat\\Plugin' api-server/app/core api-server/plugin/mochat -g composer.json
```

预期：列出全部核心包、官方插件、命名空间和声明依赖；在文档背景章节记录它们与目标领域的映射差异。

- [ ] **步骤 2：编写 15 个强制章节和标准目录结构**

固定 Go Module 为 `gitee.com/mochat/mochat/api-server-go`，并在文档中定义以下结构：

```text
api-server-go/
  cmd/{api,worker,scheduler,migrate}/
  internal/platform/{bootstrap,clock,errors,feature}/
  internal/modules/contact/{domain,application,transport,httpinfra}/
  internal/integrations/{wecom,storage,messaging}/
  internal/plugins/channelcode/
  api/openapi/
  db/{migrations,queries}/
  tests/{contract,integration,e2e}/
```

逐项说明目录所有权，并明确禁止建立全局 `common`、`helper` 或 `utils` 杂物包。

- [ ] **步骤 3：定义完整共享 Go 契约**

至少包含以下可编译签名：

```go
type Module interface {
    Name() string
    Register(*Registry) error
}

type Registry interface {
    AddHTTPRoutes(RouteContributor) error
    AddWorkerHandlers(...WorkerHandler) error
    AddScheduledJobs(...ScheduledJob) error
    AddHealthChecks(...HealthCheck) error
}

type Plugin interface {
    Module
    Version() string
    Dependencies() []PluginDependency
}

type FeatureGate interface {
    Enabled(ctx context.Context, tenantID int64, feature string) (bool, error)
}
```

为每个引用类型指定所属包。属于后续文档的类型只定义最小跨文档契约，并注明由哪一份文档细化行为。

- [ ] **步骤 4：定义依赖检查和验收用例**

指定机器可读的导入规则配置和 CI 命令，并覆盖：

```text
通过：modules/contact/application 导入 modules/contact/domain
通过：modules/contact/application 使用 modules/organization/application/public
失败：modules/contact 导入 modules/organization/infrastructure
失败：modules/contact 导入 integrations/wecom 的具体客户端
失败：任意 domain 包导入 net/http、database/sql、Redis 或第三方 SDK
```

文档必须给出违规时的预期 CI 错误格式。

- [ ] **步骤 5：校验并提交**

运行通用校验命令，再运行：

```powershell
rg -n 'iam|corp|organization|contact|room|content|campaign|chatops|analytics|platform' docs/superpowers/specs/go-foundation/01-project-foundation-design.md
rg -n 'cmd/api|cmd/worker|cmd/scheduler|cmd/migrate' docs/superpowers/specs/go-foundation/01-project-foundation-design.md
```

预期：所有领域和入口都有唯一所有权，不存在相互冲突的包结构。

```powershell
git add docs/superpowers/specs/go-foundation/01-project-foundation-design.md
git commit -m "docs: 设计 Go 工程骨架"
```

### 任务 2：运行时配置、生命周期与健康检查

**文件：**

- 创建：`docs/superpowers/specs/go-foundation/02-runtime-lifecycle-design.md`
- 阅读：`docs/superpowers/specs/go-foundation/01-project-foundation-design.md`
- 阅读：`api-server/config/config.php`
- 阅读：`api-server/config/autoload/server.php`
- 阅读：`api-server/config/autoload/databases.php`
- 阅读：`api-server/config/autoload/redis.php`
- 阅读：`api-server/config/autoload/processes.php`
- 阅读：`api-server/.env.example`

**接口关系：**

- 输入：任务 1 定义的组合根、模块注册表和健康检查扩展点。
- 输出：`Config`、`Loader`、`Validator`、`Lifecycle`、`Runner`、`HealthCheck` 和停机预算契约，供任务 3、5、6、7 使用。

- [ ] **步骤 1：盘点配置和进程钩子**

```powershell
rg -n 'env\(|getenv|SIGTERM|SIGINT|onStart|onShutdown|Process' api-server/config api-server/app api-server/plugin/mochat -g '*.php'
```

预期：分类列出服务器、MySQL、Redis、队列、文件、企业微信和进程配置，包含当前默认值及 Secret 属性。

- [ ] **步骤 2：定义精确的类型化配置**

至少包含：

```go
type Config struct {
    Service   ServiceConfig
    HTTP      HTTPConfig
    MySQL     MySQLConfig
    Redis     RedisConfig
    Worker    WorkerConfig
    Telemetry TelemetryConfig
}

type Duration time.Duration

type Loader interface {
    Load(context.Context) (Config, error)
}

type Validator interface {
    Validate() error
}
```

每个字段都要写明环境变量名、类型、默认值、允许范围、是否敏感及是否支持重载。默认规则为仅启动时加载，不支持运行时热重载。

- [ ] **步骤 3：定义生命周期和探针状态机**

状态固定为 `starting`、`ready`、`draining`、`stopped`，顺序为：

```text
加载配置 -> 校验 -> 构造依赖 -> 验证强依赖 -> 注册模块 -> 启动监听器/Worker -> ready
SIGTERM -> 标记未就绪 -> 停止接流/领取任务 -> 排空 -> 刷新 telemetry -> 关闭依赖 -> 退出
```

定义 MySQL 或 Redis 异常时 startup、readiness、liveness 的具体响应，并保证应用排空超时小于 Kubernetes `terminationGracePeriodSeconds`。

- [ ] **步骤 4：定义失败与验收矩阵**

覆盖：Secret 非法、启动时 MySQL 不可用、就绪后 Redis 中断、HTTP 请求执行中收到 SIGTERM、租约任务执行中收到 SIGTERM、停机超过截止时间。

- [ ] **步骤 5：校验并提交**

```powershell
rg -n 'starting|ready|draining|stopped|startup|readiness|liveness|SIGTERM' docs/superpowers/specs/go-foundation/02-runtime-lifecycle-design.md
git add docs/superpowers/specs/go-foundation/02-runtime-lifecycle-design.md
git commit -m "docs: 设计 Go 运行时生命周期"
```

预期：所有生命周期状态和探针行为均有明确定义，通用校验通过。

### 任务 3：HTTP 与 API 兼容层

**文件：**

- 创建：`docs/superpowers/specs/go-foundation/03-http-api-compatibility-design.md`
- 阅读：文档 01、02
- 阅读：`api-server/config/routes.php`
- 阅读：`api-server/config/autoload/middlewares.php`
- 阅读：`api-server/config/autoload/exceptions.php`
- 阅读：`api-server/app/core/common/src/Middleware/`
- 阅读：`api-server/app/core/rbac/src/Middleware/`
- 阅读：具有代表性的核心模块和插件 `src/Action/` 文件

**接口关系：**

- 输入：模块路由贡献者和生命周期契约。
- 输出：`RequestContext`、中间件顺序、`AppError`、字段校验错误、`Handler`、响应映射、分页类型和 OpenAPI 权威来源规则，供任务 6、7 使用。

- [ ] **步骤 1：盘点路由、中间件和响应行为**

```powershell
rg -n 'Router::|@Controller|@RequestMapping|@Middleware|@Middlewares' api-server/config api-server/app api-server/plugin/mochat -g '*.php'
rg -n 'DashboardAuthMiddleware|PermissionMiddleware|ExceptionHandler|Response' api-server/app api-server/config -g '*.php'
```

预期：完成路由组、公开端点、认证、权限和异常响应映射分类。

- [ ] **步骤 2：定义中间件顺序和请求上下文**

固定恢复、请求 ID、Trace、访问日志、Body 限制、CORS、认证、租户上下文、授权、Handler、响应指标的执行顺序，并定义：

```go
type RequestContext struct {
    RequestID         string
    TraceID           string
    TenantID          int64
    UserID            int64
    CorpID            int64
    PermissionVersion uint64
}

type Authenticator interface {
    Authenticate(*http.Request) (Identity, error)
}

type Authorizer interface {
    Authorize(context.Context, Permission) error
}
```

说明上下文的写入和读取方式，禁止可变全局状态。

- [ ] **步骤 3：定义错误、兼容映射和请求规则**

完整定义 `AppError`、字段错误、分页、排序白名单、严格 JSON、Body 大小、超时和幂等请求头。提供内部错误类型到现有 HTTP 状态、PHP 业务码、消息策略、日志等级和可重试性的映射表。

- [ ] **步骤 4：定义 OpenAPI 和差异契约流程**

固定 `api/openapi/` 为权威来源，明确生成命令、非确定字段规范化规则，以及状态码、JSON 结构、排序、空值、分页、错误码和副作用的比对方法。

- [ ] **步骤 5：校验并提交**

```powershell
rg -n 'RequestContext|Authenticator|Authorizer|AppError|OpenAPI|breaking|分页|排序' docs/superpowers/specs/go-foundation/03-http-api-compatibility-design.md
git add docs/superpowers/specs/go-foundation/03-http-api-compatibility-design.md
git commit -m "docs: 设计 HTTP 兼容层"
```

预期：所有归属类型及兼容维度均存在，通用校验通过。

### 任务 4：数据持久化、事务与 Outbox 存储

**文件：**

- 创建：`docs/superpowers/specs/go-foundation/04-data-persistence-design.md`
- 阅读：文档 01
- 阅读：`api-server/config/autoload/databases.php`
- 阅读：`api-server/migrations/`
- 阅读：`api-server/seeders/`
- 阅读：具有代表性的核心模块 `src/Model/` 和 `Repository/` 文件

**接口关系：**

- 输入：包所有权和组合根定义。
- 输出：`TenantScope`、`SystemScope`、`DBTX`、`Transactor`、Repository 构造规则、Migration 约定和完整 `outbox_events` Schema，供任务 5 使用。

- [ ] **步骤 1：盘点数据库配置、Schema 和访问模式**

```powershell
rg -n 'CREATE TABLE|Schema::|Migration|tenant_id|corp_id|deleted_at' api-server/migrations api-server/seeders api-server/app -g '*.php' -g '*.sql'
rg -n 'Model|Repository|where\(|transaction' api-server/app/core -g '*.php'
```

预期：汇总租户键、软删除、事务使用和高风险无作用域访问。

- [ ] **步骤 2：定义精确持久化契约**

```go
type TenantScope struct { TenantID int64 }
type SystemScope struct { Reason string; ActorID int64 }

type DBTX interface {
    ExecContext(context.Context, string, ...any) (sql.Result, error)
    QueryContext(context.Context, string, ...any) (*sql.Rows, error)
    QueryRowContext(context.Context, string, ...any) *sql.Row
}

type Transactor interface {
    WithinTransaction(context.Context, func(context.Context) error) error
}
```

明确事务在 Context 中的传播、嵌套行为、默认隔离级别、允许重试的边界，以及 Repository 与 SQLC 生成查询的构造方式。

- [ ] **步骤 3：定义 SQLC 和 Migration 约定**

固定查询目录、命名、生成包、空值类型、枚举映射、动态排序白名单、Migration 文件名、生产只前进策略、本地 Down Migration 及空库/基线库升级测试。

- [ ] **步骤 4：定义完整 Outbox 表和状态转换**

提供可执行 MySQL 8 DDL，包含事件 ID、租户、聚合、类型、版本、JSON Payload、Header、状态、尝试次数、可执行时间、租约所有者/期限、时间戳、最后错误码，以及领取和保留索引。

状态固定为：`pending -> processing -> published` 和 `processing -> pending/dead`，并明确 Compare-and-Set 条件。

- [ ] **步骤 5：定义验收测试、校验并提交**

覆盖跨租户读取、缺失租户作用域、业务与 Outbox 一起回滚、并发领取、过期租约恢复、空库和基线库迁移、SQLC 查询编译。

```powershell
rg -n 'TenantScope|SystemScope|DBTX|Transactor|CREATE TABLE outbox_events|pending|processing|published|dead' docs/superpowers/specs/go-foundation/04-data-persistence-design.md
git add docs/superpowers/specs/go-foundation/04-data-persistence-design.md
git commit -m "docs: 设计 Go 数据持久化"
```

### 任务 5：异步处理与调度

**文件：**

- 创建：`docs/superpowers/specs/go-foundation/05-async-processing-design.md`
- 阅读：文档 02、04
- 阅读：`api-server/config/autoload/async_queue.php`
- 阅读：`api-server/config/autoload/processes.php`
- 阅读：`api-server/config/autoload/crontab.php`
- 阅读：核心模块及插件的 `Process/`、`Job/`、`Task/`、`Listener/` 目录

**接口关系：**

- 输入：生命周期状态机、`outbox_events` DDL、领取状态转换和事务规则。
- 输出：`Envelope`、`Publisher`、`Consumer`、`Handler`、`RetryPolicy`、`DeadLetter`、`Lease`、`ScheduledJob`，供任务 6、7 使用。

- [ ] **步骤 1：盘点队列、消费者、监听器和定时任务**

```powershell
rg -n '@Process|ConsumerProcess|AsyncQueue|@Crontab|Listener|Job' api-server/app api-server/plugin/mochat api-server/config -g '*.php'
```

预期：每个异步负载均有来源、触发方式、Payload、重试行为、并发方式和目标模块。

- [ ] **步骤 2：定义事件信封和队列端口**

```go
type Envelope struct {
    ID            string
    Type          string
    Version       uint16
    TenantID      int64
    OccurredAt    time.Time
    CorrelationID string
    CausationID   string
    Payload       json.RawMessage
}

type Handler interface { Handle(context.Context, Envelope) error }
type Publisher interface { Publish(context.Context, ...Envelope) error }
type Consumer interface { Run(context.Context, Handler) error }
```

明确序列化、版本兼容、Payload 大小和 Trace 传播。

- [ ] **步骤 3：定义 Outbox Relay、Worker、重试和死信算法**

用伪代码写出批大小配置、租约、心跳、并发、租户公平性、背压、带抖动指数退避、最大次数、不可重试错误、死信记录和审计化重放。

- [ ] **步骤 4：定义 Scheduler 语义**

明确任务 ID、Cron 时区、租约键、错过执行策略、重叠策略、补跑上限、人工触发、幂等键和停机行为。将当前定时任务映射到目标注册点，但不设计其业务逻辑。

- [ ] **步骤 5：定义失败测试、校验并提交**

覆盖外部副作用完成但 Ack 前崩溃、租约过期、重复消息、Redis 中断、毒消息、租户饥饿、Scheduler 主节点丢失、重叠触发和处理中 SIGTERM。

```powershell
rg -n 'Envelope|Publisher|Consumer|Handler|RetryPolicy|DeadLetter|lease|misfire|jitter|幂等' docs/superpowers/specs/go-foundation/05-async-processing-design.md
git add docs/superpowers/specs/go-foundation/05-async-processing-design.md
git commit -m "docs: 设计异步处理"
```

### 任务 6：可观测性与安全基线

**文件：**

- 创建：`docs/superpowers/specs/go-foundation/06-observability-security-design.md`
- 阅读：文档 02、03、05
- 阅读：`api-server/config/autoload/logger.php`
- 阅读：`api-server/config/autoload/auth.php`
- 阅读：`api-server/config/autoload/middlewares.php`
- 阅读：`api-server/config/autoload/file.php`

**接口关系：**

- 输入：请求上下文、应用错误、生命周期、事件信封、Worker 和 Scheduler 契约。
- 输出：标准日志字段、Telemetry 初始化和传播、指标目录、审计信封、脱敏接口、限流键策略和安全控制矩阵，供任务 7 使用。

- [ ] **步骤 1：盘点日志、鉴权、Secret、上传和限流**

```powershell
rg -n 'logger|Log::|auth|token|secret|password|upload|mime|cors|rate|limit' api-server/config api-server/app api-server/plugin/mochat -g '*.php'
```

预期：分类记录现有控制和缺口，禁止把 Secret 值写入文档。

- [ ] **步骤 2：精确定义日志、Trace 和指标**

提供强制 JSON 日志 Schema、等级规则、采样、脱敏、OpenTelemetry Resource、W3C Trace Context，以及包含准确名称、类型、单位、标签和高基数限制的指标表。

```go
type Redactor interface { Redact(key string, value any) any }
type AuditWriter interface { Write(context.Context, AuditEvent) error }
type Telemetry interface { Shutdown(context.Context) error }
```

- [ ] **步骤 3：定义安全控制矩阵**

覆盖 Secret 来源和轮换、Token 脱敏、Webhook 验签钩子、CORS、安全响应头、Body/上传限制、MIME 校验、对象私有性、SQL 注入控制、租户限流键、审计保留、依赖扫描、SBOM 和容器运行限制。

- [ ] **步骤 4：定义 SLO 和端到端验收**

使用月度 99.9% 可用性目标。定义 HTTP 请求、Outbox 事件、队列消息、Worker 尝试、外部调用、重试和死信之间的 request/trace/correlation 标识关联。加入验证 Secret 和手机号不出现在日志或 Span 中的测试。

- [ ] **步骤 5：校验并提交**

```powershell
rg -n 'trace_id|request_id|tenant_id|event_id|error_code|99.9|Redactor|AuditWriter|SBOM' docs/superpowers/specs/go-foundation/06-observability-security-design.md
git add docs/superpowers/specs/go-foundation/06-observability-security-design.md
git commit -m "docs: 设计可观测性与安全"
```

### 任务 7：工程质量、CI/CD 与本地环境

**文件：**

- 创建：`docs/superpowers/specs/go-foundation/07-engineering-quality-design.md`
- 阅读：前 6 份工程底座详细设计
- 阅读：`.github/workflows/`
- 阅读：`api-server/phpunit.xml`
- 阅读：`api-server/phpstan.neon`
- 阅读：`api-server/docker-compose.sample.yml`
- 阅读：`api-server/Dockerfile`
- 阅读：`.build/`

**接口关系：**

- 输入：任务 1～6 定义的所有包、生成器、Schema、契约、Telemetry、安全和验收规则。
- 输出：标准开发命令、测试结构、Fixture 契约、契约比对器、CI 作业图、制品策略、镜像门禁和工程底座完成定义。

- [ ] **步骤 1：盘点当前构建、测试和部署自动化**

```powershell
rg --files .github .build api-server | rg 'workflow|Dockerfile|compose|phpunit|phpstan|test|Makefile|package.json'
rg -n 'phpunit|phpstan|docker|build|test|lint' .github .build api-server -g '*.yml' -g '*.yaml' -g '*.json' -g '*.xml' -g 'Dockerfile*'
```

预期：汇总现有 CI、开发依赖、测试入口和镜像构建假设。

- [ ] **步骤 2：定义标准本地命令面**

```text
make bootstrap
make generate
make lint
make test-unit
make test-integration
make test-contract
make test-e2e
make test-race
make test
make build
make image
make verify
```

为每个命令列出前置条件、生成或修改的文件、退出条件和 CI 使用情况。工具版本固定在工具清单中，不依赖开发者全局安装。

- [ ] **步骤 3：定义测试架构和 Fixture**

指定 MySQL 8、Redis、S3 兼容 Testcontainers Fixture 的包路径和 API，并定义 Migration 初始化、确定性时钟/ID、租户工厂、PHP/Go 样本脱敏、JSON 规范化、副作用捕获和失败诊断。

- [ ] **步骤 4：定义 CI/CD 作业图和硬门禁**

包含依赖缓存、生成差异、格式、Vet、Staticcheck、单元、集成、契约、核心并发包 Race、空库/基线 Migration、OpenAPI Breaking Diff、覆盖率、Secret、漏洞、许可证、SBOM、镜像扫描和签名制品。明确可并行作业及下游消费的制品。

- [ ] **步骤 5：定义全底座追踪矩阵**

将文档 01～06 的每项要求映射到命令、测试套件、CI 作业、证据制品和失败负责人。仅当给出明确评审清单和批准角色时，才允许标记为“仅人工检查”。

- [ ] **步骤 6：联合校验 7 份文档**

```powershell
$files = Get-ChildItem docs/superpowers/specs/go-foundation -Filter '*-design.md'
if ($files.Count -ne 7) { throw "预期 7 份设计文档，实际为 $($files.Count) 份" }
rg -n 'TBD|TODO|FIXME|PLACEHOLDER|待定|稍后实现|类似上述' docs/superpowers/specs/go-foundation
git diff --check -- docs/superpowers/specs/go-foundation
rg -n 'type (Module|Config|RequestContext|AppError|TenantScope|Envelope)|CREATE TABLE outbox_events' docs/superpowers/specs/go-foundation
```

预期：恰好 7 份文档；无占位内容或空白错误；每个跨文档标准类型和 Outbox DDL 只有一个定义所有者，引用保持一致。

- [ ] **步骤 7：提交**

```powershell
git add docs/superpowers/specs/go-foundation/07-engineering-quality-design.md
git commit -m "docs: 设计 Go 工程质量体系"
```

### 任务 8：最终跨文档审查与批准索引

**文件：**

- 创建：`docs/superpowers/specs/go-foundation/README.md`
- 审查发现缺陷时，修改对应的 7 份详细设计文档

**接口关系：**

- 输入：7 份完整文档及其批准证据。
- 输出：导航、依赖顺序、所有权登记表、批准状态和供工程实施规划使用的最终一致性报告。

- [ ] **步骤 1：建立所有权登记表**

列出每个公开 Go 类型、配置命名空间、数据库表、事件命名空间、指标前缀、命令及其唯一所属文档，并使用相对路径链接文档。

- [ ] **步骤 2：执行最终一致性审查**

检查：

```text
包路径和导入规则
类型和方法签名
配置键、类型、默认值和单位
错误类型和可重试性
Outbox 和消息状态
超时、租约、重试和停机预算
租户传播和审计标识
指标和日志字段名
测试命令和 CI 作业名
```

发现冲突时先修改定义所有者所在的上游文档，再更新下游引用。

- [ ] **步骤 3：运行最终自动检查**

```powershell
rg -n 'TBD|TODO|FIXME|PLACEHOLDER|待定|稍后实现|类似上述' docs/superpowers/specs/go-foundation
git diff --check -- docs/superpowers/specs/go-foundation
Get-ChildItem docs/superpowers/specs/go-foundation -Filter '*-design.md' | ForEach-Object {
  $count = (Select-String -Path $_.FullName -Pattern '^## (1[0-5]|[1-9])\.' -AllMatches).Count
  if ($count -ne 15) { throw "$($_.Name) 的强制章节数为 $count，预期为 15" }
}
```

预期：占位内容和空白错误扫描无输出；每份设计恰好包含 15 个强制章节。

- [ ] **步骤 4：提交批准索引和审查修正**

```powershell
git add docs/superpowers/specs/go-foundation
git commit -m "docs: 完成 Go 工程底座详细设计"
```

- [ ] **步骤 5：记录交接条件**

README 必须明确：仅当 7 份文档状态均为“已确认”、未解决问题章节均为“无未解决问题”且最终一致性检查通过后，才可开始 Go 工程底座的代码实施规划。
