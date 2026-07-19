# 06 可观测性与安全基线详细设计

**状态：已确认**

**批准日期：2026-07-19**

## 1. 目标、范围与非目标

统一日志、Trace、指标、审计、Secret、HTTP/上传/容器安全基线和 SLO；不替代业务授权规则或生产告警平台配置。

## 2. 上游依赖与下游使用者

依赖 02 状态、03 RequestContext/AppError、05 Envelope/Worker；供 07 门禁及所有运行时组件使用。

## 3. 关键决策及被否决方案

采用 `log/slog` JSON、OpenTelemetry、字段白名单和默认脱敏。否决自由文本生产日志、把 tenant/user 作为无限制指标标签及在日志中记录请求体。

## 4. 包、文件和组件职责

`internal/platform/telemetry` 初始化 OTel，`logging` 管 slog/脱敏，`audit` 管不可抵赖事件，`security` 管通用 HTTP 控制；模块拥有业务审计动作名。

## 5. 对外接口与完整 Go 类型签名

```go
type Redactor interface { Redact(key string, value any) any }
type AuditEvent struct { ID string; OccurredAt time.Time; TenantID int64; ActorID int64; Action string; Resource string; ResourceID string; Result string; TraceID string; Metadata map[string]string }
type AuditWriter interface { Write(context.Context, AuditEvent) error }
type Telemetry interface { Shutdown(context.Context) error }
```

## 6. 配置项、默认值和启动校验

`MOCHAT_LOG_LEVEL=info`；`MOCHAT_LOG_SAMPLE_RATE=1.0`（0..1）；`MOCHAT_OTEL_ENDPOINT` 可空；`MOCHAT_OTEL_SAMPLE_RATIO=0.1`（0..1，错误强制采样）；`MOCHAT_CORS_ORIGINS` 生产必填；`MOCHAT_RATE_LIMIT_RPS=50`（1..10000）；`MOCHAT_UPLOAD_MAX_BYTES=10MiB`（1MiB..100MiB）；审计保留 `365d`。Secret 值均不得出现在配置诊断中。

## 7. 正常数据流与关键时序

HTTP 建立 request/trace → DB/Redis/外部调用创建子 Span → Outbox 写入关联 ID → Envelope 传播 correlation/causation → Worker 恢复链接 → dead/replay 继续关联。审计先校验必填字段，再持久化，失败时高风险操作失败关闭。

## 8. 事务、并发、幂等和一致性规则

审计 ID 唯一，关键业务审计与业务写同事务或写 Outbox；日志/Span 导出失败不得回滚业务，但必须计数。限流键为 `{env}:{tenant_id}:{principal}:{route}`，缺租户的公开端点使用来源 IP 的 HMAC 摘要。

## 9. 错误分类、超时、重试与降级

Telemetry 导出超时 3s、停机 flush 5s；导出失败丢弃并计数，不能阻塞请求。审计写失败对高风险写操作返回 503；普通诊断日志失败写 stderr。

## 10. 安全、租户隔离和敏感数据处理

Secret 只由 Secret Manager/Kubernetes Secret 注入并支持双版本轮换。禁止日志/Span/指标含 password、token、cookie、authorization、corp_secret、手机号、邮箱、原始 body。启用 HSTS、nosniff、frame deny、严格 CORS；上传校验大小、扩展名、探测 MIME、异步病毒扫描，对象默认私有。容器非 root、只读根文件系统、删除 Linux capabilities，依赖扫描、SBOM 和镜像签名为门禁。

## 11. 日志、指标与 Trace 要求

JSON 必含 `timestamp level service env message trace_id request_id tenant_id user_id module event_id error_code`（不适用字段为空）。指标：`http_server_requests_total` counter（route/method/status_class）；`http_server_duration_seconds` histogram；`db_client_duration_seconds` histogram（operation/module）；`redis_client_errors_total` counter；`outbox_events_total` counter（status/type）；`queue_oldest_age_seconds` gauge；`worker_attempts_total` counter（handler/result）；`scheduler_runs_total` counter（job/result）；`wecom_client_requests_total` counter（operation/result）。禁止 tenant_id、user_id、event_id 作指标标签。

## 12. 测试矩阵及具体验收场景

端到端追踪 HTTP→Outbox→队列→Worker→外部调用；验证 request/trace/correlation 可检索。注入 token、手机号、Secret 到错误、请求和 Payload，断言日志与 Span 均无明文。验证跨租户限流隔离、Webhook 重放拒绝、私有对象、MIME 欺骗和非 root 镜像。

## 13. 性能边界与容量假设

核心 API 月度可用性 99.9%；日志同步开销 P95 <1ms；生产正常 trace 采样 10%、错误 100%；指标每实例活跃时序低于 10,000。

## 14. 发布、迁移、回退和兼容要求

新字段先在收集端兼容；删除指标至少保留一个发布周期并提供替代名。安全基线只能通过有期限、具审批人和补偿控制的例外记录放宽。

## 15. 未解决问题

无未解决问题。
