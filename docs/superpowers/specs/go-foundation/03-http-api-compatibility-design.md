# 03 HTTP 与 API 兼容层详细设计

**状态：已确认**

**批准日期：2026-07-19**

## 1. 目标、范围与非目标

定义 chi 路由、中间件、上下文、输入、错误和 PHP/Go 契约比对。保持现有 API 行为；不重写 IAM 规则或业务 Handler。

## 2. 上游依赖与下游使用者

依赖 01 的 RouteContributor、02 的生命周期；供 06、07 和所有模块 HTTP 适配器使用。

## 3. 关键决策及被否决方案

OpenAPI 3.1 文件为权威来源，兼容迁移以 PHP 实际样本校验。否决从 Handler 反向生成契约和统一改造旧响应包络。

## 4. 包、文件和组件职责

`internal/platform/httpserver` 管路由/中间件，`httpctx` 管上下文，`errors` 管 AppError，`validation` 管输入，`api/openapi` 管契约；模块在 `httpinfra` 注册路由。

## 5. 对外接口与完整 Go 类型签名

```go
type RequestContext struct { RequestID string; TraceID string; TenantID int64; UserID int64; CorpID int64; PermissionVersion uint64 }
type Identity struct { TenantID int64; UserID int64; CorpID int64; PermissionVersion uint64 }
type Permission string
type Authenticator interface { Authenticate(*http.Request) (Identity, error) }
type Authorizer interface { Authorize(context.Context, Permission) error }
type RouteContributor interface { Routes() http.Handler }
type Handler func(http.ResponseWriter, *http.Request) error
type AppError struct { Code string; HTTPStatus int; PublicMessage string; Retryable bool; Fields []FieldError; Cause error }
type FieldError struct { Field string; Rule string; Message string }
type PageRequest struct { Page int; PageSize int; Sort string; Order string }
```

## 6. 配置项、默认值和启动校验

`MOCHAT_HTTP_MAX_BODY_BYTES` int64 默认 1MiB、1KiB..10MiB；`MOCHAT_HTTP_IDEMPOTENCY_TTL` duration 默认 24h、1h..168h；`MOCHAT_CORS_ORIGINS` 列表生产必填；均非敏感、不热重载。排序字段必须由端点白名单声明。

## 7. 正常数据流与关键时序

顺序固定：Recoverer → RequestID → W3C Trace → AccessLog → BodyLimit → CORS → Authenticate → TenantContext → Authorize → Handler → ResponseMetrics。上下文只通过 `request.Context()` 传递，不可修改或存入全局变量。

## 8. 事务、并发、幂等和一致性规则

Handler 不共享可变请求状态。写端点按现有契约读取 `Idempotency-Key`；同租户同键同请求返回首次结果，不同请求返回 `IDEMPOTENCY_CONFLICT`。

## 9. 错误分类、超时、重试与降级

| 内部类型 | HTTP | PHP 业务码 | 日志 | 可重试 |
|---|---:|---|---|---|
| validation | 422 | `PARAM_ERROR` | info | 否 |
| unauthenticated | 401 | `UNAUTHENTICATED` | info | 否 |
| forbidden | 403 | `FORBIDDEN` | warn | 否 |
| not_found | 404 | `NOT_FOUND` | info | 否 |
| conflict | 409 | `CONFLICT` | info | 否 |
| rate_limited | 429 | `RATE_LIMITED` | warn | 是 |
| dependency | 503 | `SERVICE_UNAVAILABLE` | error | 是 |
| internal | 500 | `SYSTEM_ERROR` | error | 否 |

公开消息使用稳定文案，Cause 仅进日志。严格 JSON 拒绝未知字段、重复键和尾随内容；请求超时 15s。

## 10. 安全、租户隔离和敏感数据处理

TenantID 只能来自认证身份，不能信任请求字段。Webhook 通用端口要求验签、5 分钟时间窗、原始报文摘要和来源事件 ID；业务解密归 integration。禁止返回堆栈或 Secret。

## 11. 日志、指标与 Trace 要求

访问日志包含 `request_id trace_id tenant_id user_id route method status error_code duration_ms`；禁止记录 token、cookie、原始 body。

## 12. 测试矩阵及具体验收场景

PHP/Go 对同一脱敏样本比对 HTTP 状态、JSON 字段/类型、空值、排序、分页、错误码和可观测副作用；时间、ID 先规范化。覆盖超大 Body、未知字段、排序注入、重复幂等键、权限拒绝和 Webhook 重放。任一未批准差异阻断合并。

## 13. 性能边界与容量假设

中间件自身 P95 开销低于 2ms（不含鉴权存储访问）；默认请求体 1MiB；每连接读头超时 5s、空闲超时 60s。

## 14. 发布、迁移、回退和兼容要求

旧端点路径/方法/鉴权/字段保持；新 API 使用 `/api/v1`。OpenAPI breaking diff 和契约样本必须通过，灰度可按端点/租户回切 PHP。

## 15. 未解决问题

无未解决问题。
