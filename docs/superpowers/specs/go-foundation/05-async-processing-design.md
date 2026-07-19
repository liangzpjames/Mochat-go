# 05 异步处理与调度详细设计

**状态：已确认**

**批准日期：2026-07-19**

## 1. 目标、范围与非目标

定义 Outbox Relay、队列端口、Worker、重试、死信和 Scheduler 的工程语义；不定义具体业务任务内容。

## 2. 上游依赖与下游使用者

依赖 02 生命周期和 04 Outbox/事务；供 06、07 及业务模块使用。

## 3. 关键决策及被否决方案

采用至少一次投递、消费者幂等、带租约领取和 Redis 队列适配器。否决“恰好一次”承诺、进程内队列和无审计人工重放。

## 4. 包、文件和组件职责

`internal/platform/messaging` 拥有信封/端口，`outbox` 拥有 Relay，`worker` 拥有执行器，`scheduler` 拥有调度；Redis 细节位于 `internal/integrations/messaging`。

## 5. 对外接口与完整 Go 类型签名

```go
type Envelope struct { ID string; Type string; Version uint16; TenantID int64; OccurredAt time.Time; CorrelationID string; CausationID string; Payload json.RawMessage }
type Handler interface { Handle(context.Context, Envelope) error }
type Publisher interface { Publish(context.Context, ...Envelope) error }
type Consumer interface { Run(context.Context, Handler) error }
type RetryPolicy interface { Next(attempt uint16, err error) (time.Duration, bool) }
type DeadLetter struct { Event Envelope; Attempts uint16; ErrorCode string; FailedAt time.Time }
type Lease struct { Owner string; Until time.Time }
type ScheduledJob interface { Name() string; Run(context.Context, string) error }
```

## 6. 配置项、默认值和启动校验

| 键 | 默认/范围 |
|---|---|
| `MOCHAT_OUTBOX_BATCH_SIZE` | 100；1..1000 |
| `MOCHAT_OUTBOX_LEASE` | 30s；10s..5m |
| `MOCHAT_WORKER_CONCURRENCY` | 16；1..256 |
| `MOCHAT_WORKER_MAX_ATTEMPTS` | 8；1..20 |
| `MOCHAT_WORKER_MAX_PAYLOAD_BYTES` | 256KiB；1KiB..1MiB |
| `MOCHAT_SCHEDULER_TIMEZONE` | `Asia/Shanghai`，必须为 IANA 名称 |
| `MOCHAT_SCHEDULER_CATCHUP_LIMIT` | 3；0..100 |

均非敏感、不热重载；租约必须大于单次心跳间隔三倍。

## 7. 正常数据流与关键时序

Relay 按 `(status,available_at,id)` 领取批次并 CAS 写 owner/lease → 发布 Envelope → Ack 后 CAS 为 published。Worker 解析版本、恢复 trace、执行 Handler、写幂等结果后 Ack。Scheduler 取得 `{job}:{scheduled_at}` 租约后调用任务。

## 8. 事务、并发、幂等和一致性规则

同租户按轮转桶公平领取，每轮最多占批次 25%；队列高水位暂停 Relay。消费者以 `(handler,event_id)` 持久化幂等结果；外部副作用使用稳定业务幂等键。Scheduler 默认 `forbid-overlap`，人工触发使用独立 trigger ID 但相同业务幂等键。

## 9. 错误分类、超时、重试与降级

临时网络、429、5xx 可重试；校验、权限、未知版本和业务冲突不可重试。退避 `min(500ms*2^(attempt-1),5m)` 加 ±20% 抖动，最多 8 次；耗尽转 dead。重放必须权限校验、原因、操作者、审计事件并生成新投递尝试，不篡改原记录。

## 10. 安全、租户隔离和敏感数据处理

Envelope 必含 TenantID；Payload 只含必要数据，不含 token/Secret，敏感值改为引用。Worker 在调用 Handler 前建立 TenantScope。

## 11. 日志、指标与 Trace 要求

传播 `traceparent`、`correlation_id`、`causation_id`；日志含 `event_id job handler attempt lease_owner error_code`。准确指标见 06。

## 12. 测试矩阵及具体验收场景

验证副作用完成但 Ack 前崩溃不会重复副作用；租约过期由另一实例接管；重复消息只执行一次；Redis 中断停止领取且保留 Outbox；毒消息转 dead；大租户不饿死小租户；Scheduler 失主接管、重叠触发被拒；SIGTERM 停止新领取并在截止前完成或释放租约。

## 13. 性能边界与容量假设

单 Worker 默认并发 16；Relay 批次 100；Payload 上限 256KiB。背压以队列深度 10,000 或最老消息 5 分钟为高水位，恢复阈值为 50%。

## 14. 发布、迁移、回退和兼容要求

事件名 `<module>.<aggregate>.<action>.v<major>`；消费者先兼容新版本再发布生产者。Scheduler 的 misfire 策略显式为 `skip`、`run-once` 或 `catch-up`，默认 `run-once`；回退不得删除未消费事件。

## 15. 未解决问题

无未解决问题。
