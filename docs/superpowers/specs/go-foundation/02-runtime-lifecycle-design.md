# 02 运行时配置、生命周期与健康检查详细设计

**状态：已确认**

**批准日期：2026-07-19**

## 1. 目标、范围与非目标

统一 API、Worker、Scheduler 的类型化配置、启动、探针与优雅停机。不定义业务配置中心或业务任务。

## 2. 上游依赖与下游使用者

使用 01 的 Registry 和组合根；被 03、05、06、07 使用。

## 3. 关键决策及被否决方案

采用环境变量和挂载 Secret、启动时一次加载、失败即退出。否决隐式全局配置、运行时热重载和以 liveness 表示依赖瞬时故障。

## 4. 包、文件和组件职责

`internal/platform/config` 拥有加载与校验；`internal/platform/runtime` 拥有状态机和 Runner；`internal/platform/health` 拥有探针聚合。

## 5. 对外接口与完整 Go 类型签名

```go
type Config struct { Service ServiceConfig; HTTP HTTPConfig; MySQL MySQLConfig; Redis RedisConfig; Worker WorkerConfig; Telemetry TelemetryConfig }
type Duration time.Duration
type Loader interface { Load(context.Context) (Config, error) }
type Validator interface { Validate() error }
type State string
const (Starting State="starting"; Ready State="ready"; Draining State="draining"; Stopped State="stopped")
type HealthCheck interface { Name() string; Check(context.Context) error }
type Lifecycle interface { State() State; Drain(context.Context) error; Stop(context.Context) error }
type Runner interface { Run(context.Context) error }
```

## 6. 配置项、默认值和启动校验

| 环境变量 | 类型 | 默认/范围 | Secret | 热重载 |
|---|---|---|---|---|
| `MOCHAT_ENV` | string | `local`; local/test/staging/prod | 否 | 否 |
| `MOCHAT_SERVICE_NAME` | string | `mochat-api`; 1..63 | 否 | 否 |
| `MOCHAT_HTTP_ADDR` | string | `:8080` | 否 | 否 |
| `MOCHAT_HTTP_REQUEST_TIMEOUT` | duration | `15s`; 1s..120s | 否 | 否 |
| `MOCHAT_MYSQL_DSN` | string | 必填 | 是 | 否 |
| `MOCHAT_MYSQL_MAX_OPEN` | int | `50`; 1..500 | 否 | 否 |
| `MOCHAT_REDIS_ADDR` | string | 必填 | 否 | 否 |
| `MOCHAT_REDIS_PASSWORD` | string | 空 | 是 | 否 |
| `MOCHAT_WORKER_CONCURRENCY` | int | `16`; 1..256 | 否 | 否 |
| `MOCHAT_SHUTDOWN_TIMEOUT` | duration | `25s`; 5s..55s | 否 | 否 |
| `MOCHAT_OTEL_ENDPOINT` | string | 空（禁用导出） | 否 | 否 |

解析错误、越界、生产缺少 DSN/Redis 均拒绝启动。

## 7. 正常数据流与关键时序

`加载 -> 校验 -> 构造依赖 -> 验证强依赖 -> 注册模块 -> 启动监听器/Worker -> ready`。SIGTERM 后 `draining -> 摘流/停止领取 -> 排空 -> flush telemetry -> 关闭依赖 -> stopped`。

## 8. 事务、并发、幂等和一致性规则

状态转换使用原子 CAS；Drain/Stop 可重复调用。排空不开始新任务，已领取任务遵守 05 的租约语义。

## 9. 错误分类、超时、重试与降级

配置非法不可重试；启动 MySQL 不可用，3 次（1s/2s/4s）后退出；Redis 在运行中中断使 readiness=503、liveness 仍为 200，并由调用方退避。停机超时记录 `shutdown_deadline_exceeded` 后非零退出。

## 10. 安全、租户隔离和敏感数据处理

Secret 只来自环境注入/挂载文件；错误仅报告键名，不报告值。配置结构不得被 HTTP 输出。

## 11. 日志、指标与 Trace 要求

每次状态变更记录 `state_from/state_to/reason`；暴露启动耗时、readiness 失败和停机耗时，名称见 06。

## 12. 测试矩阵及具体验收场景

验证非法 Secret/DSN 启动失败；MySQL 启动不可用退出；Redis 运行中断只摘流；请求中 SIGTERM 等待完成；租约任务 SIGTERM 停止续租并安全归还；超过截止时间强制退出。`/startupz` 仅 ready 后 200，`/readyz` 仅 ready 时 200，`/livez` 除进程死锁/不可恢复内部错误外 200。

## 13. 性能边界与容量假设

探针总预算 500ms、单检查 200ms；停机预算 25s，Kubernetes `terminationGracePeriodSeconds=30`，保留 5s 给强制终止。

## 14. 发布、迁移、回退和兼容要求

新增必填配置需先部署带默认兼容版本，再启用；配置删除跨两个版本。旧版本能忽略新增可选键。

## 15. 未解决问题

无未解决问题。
