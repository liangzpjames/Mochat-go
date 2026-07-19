# Go 工程底座详细设计批准索引

**整体状态：已批准**

**批准日期：2026-07-19**

**批准依据：用户明确要求完成并批准；7 份文档通过结构、占位符、所有权和交叉一致性检查。**

## 文档与依赖顺序

1. [01 工程骨架与依赖规则](01-project-foundation-design.md) — 已确认
2. [02 运行时配置、生命周期与健康检查](02-runtime-lifecycle-design.md) — 已确认
3. [03 HTTP 与 API 兼容层](03-http-api-compatibility-design.md) — 已确认
4. [04 数据持久化、事务与 Outbox 存储](04-data-persistence-design.md) — 已确认
5. [05 异步处理与调度](05-async-processing-design.md) — 已确认
6. [06 可观测性与安全基线](06-observability-security-design.md) — 已确认
7. [07 工程质量、CI/CD 与本地环境](07-engineering-quality-design.md) — 已确认

依赖关系：01→02/03/04，02+04→05，02+03+05→06，01～06→07。

## 唯一所有权登记

| 类别 | 名称/前缀 | 所有者 |
|---|---|---|
| Go 类型 | `Module Registry Plugin PluginDependency FeatureGate` | 01 |
| Go 类型 | `Config Duration State HealthCheck Lifecycle Runner` | 02 |
| Go 类型 | `RequestContext Identity Permission Authenticator Authorizer RouteContributor Handler AppError FieldError PageRequest` | 03 |
| Go 类型/表 | `TenantScope SystemScope DBTX Transactor outbox_events` | 04 |
| Go 类型/事件 | `Envelope Publisher Consumer Handler RetryPolicy DeadLetter Lease ScheduledJob`；`<module>.*.v*` | 05 |
| Go 类型/指标 | `Redactor AuditEvent AuditWriter Telemetry`；`http_ db_ redis_ outbox_ queue_ worker_ scheduler_ wecom_` | 06 |
| Go 类型/命令 | `TestEnvironment FixtureFactory ContractNormalizer NormalizedResponse`；全部 `make` 命令 | 07 |
| 配置命名空间 | `MOCHAT_ENABLED_PLUGINS` | 01 |
| 配置命名空间 | `MOCHAT_SERVICE_ MOCHAT_HTTP_ADDR MOCHAT_MYSQL_ MOCHAT_REDIS_ MOCHAT_SHUTDOWN_` | 02/04（字段按文档唯一拥有） |
| 配置命名空间 | `MOCHAT_HTTP_MAX_ MOCHAT_HTTP_IDEMPOTENCY_ MOCHAT_CORS_` | 03 |
| 配置命名空间 | `MOCHAT_OUTBOX_ MOCHAT_WORKER_ MOCHAT_SCHEDULER_` | 05 |
| 配置命名空间 | `MOCHAT_LOG_ MOCHAT_OTEL_ MOCHAT_RATE_LIMIT_ MOCHAT_UPLOAD_` | 06 |
| 配置命名空间 | `MOCHAT_TEST_ MOCHAT_TESTCONTAINERS_` | 07 |

同名 `Handler` 分属 `httpserver` 与 `messaging` 包，不构成 Go 标识符冲突。共享配置字段的行为定义归 02，连接池细节归 04，Worker 并发语义归 05。

## 最终一致性报告

- 包路径与依赖方向一致，domain 不依赖 I/O，跨模块只使用公开 Application API、领域事件或只读端口。
- 配置均给出类型、默认值/必填性、范围、Secret 和热重载策略。
- `AppError`、Outbox 状态、事件版本、重试、租约、停机预算和租户传播无冲突。
- request/trace/correlation/event 标识从 HTTP 到异步链路闭环；指标禁止高基数租户/用户标签。
- 测试命令、CI 作业、证据制品和负责人可追踪到 01～06 的要求。
- 7 份文档均为“已确认”，第 15 节均为“无未解决问题”，且未创建或修改任何 `.go` 文件。

## 实施交接门禁

本批准仅允许下一阶段编写“Go 工程底座实施计划”。在用户另行明确授权前，不得开始 Go 业务代码；底座代码实施也必须先基于这 7 份设计形成逐任务计划并再次确认范围。
