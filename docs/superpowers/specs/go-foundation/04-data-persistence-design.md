# 04 数据持久化、事务与 Outbox 存储详细设计

**状态：已确认**

**批准日期：2026-07-19**

## 1. 目标、范围与非目标

定义 MySQL 8、SQLC、租户作用域、事务、Migration 和 Outbox 持久化。不定义业务表或消息投递算法。

## 2. 上游依赖与下游使用者

依赖 01 的包边界；为 05 的 Relay 和所有 Repository 提供契约。

## 3. 关键决策及被否决方案

MySQL 是事实源，SQLC 生成类型安全查询，事务由用例开启。否决 ORM 隐式查询、Repository 自启嵌套事务及 Redis 作为唯一事实源。

## 4. 包、文件和组件职责

`internal/platform/database` 拥有连接与事务，模块 `infrastructure/repository` 适配 SQLC，`db/queries/<module>` 保存 SQL，`db/migrations` 保存只追加迁移。

## 5. 对外接口与完整 Go 类型签名

```go
type TenantScope struct { TenantID int64 }
type SystemScope struct { Reason string; ActorID int64 }
type DBTX interface {
    ExecContext(context.Context, string, ...any) (sql.Result, error)
    QueryContext(context.Context, string, ...any) (*sql.Rows, error)
    QueryRowContext(context.Context, string, ...any) *sql.Row
}
type Transactor interface { WithinTransaction(context.Context, func(context.Context) error) error }
```

事务句柄经 Context 私有键传播；Repository 构造器接收 DBTX，不暴露生成 Queries。

## 6. 配置项、默认值和启动校验

`MOCHAT_MYSQL_MAX_IDLE=10`（0..100）、`MAX_OPEN=50`（1..500）、`CONN_MAX_LIFETIME=30m`（1m..2h）、`QUERY_TIMEOUT=3s`（100ms..30s）、`TX_TIMEOUT=10s`（1s..60s）。DSN 为 Secret；启动 Ping 预算 2s。

## 7. 正常数据流与关键时序

用例校验 Scope → `WithinTransaction` → Repository 写业务数据 → 同事务插入 Outbox → Commit。读取必须选择 TenantScope 或经审计的 SystemScope。

## 8. 事务、并发、幂等和一致性规则

默认 `READ COMMITTED`；嵌套调用复用现有事务，内层不能独立提交。仅在整个用例无外部副作用且遇到死锁/序列化冲突时最多重试 2 次（20ms、50ms 加抖动）。唯一索引是最终幂等防线。

## 9. 错误分类、超时、重试与降级

无 Scope 为 `DATA_SCOPE_REQUIRED`；唯一冲突为领域 conflict；超时为 retryable dependency；语法/约束设计错误不可重试。数据库不可用时拒绝写入，不降级至 Redis。

## 10. 安全、租户隔离和敏感数据处理

租户表 SQL 必须显式命名参数 `tenant_id`；静态检查阻止无租户谓词。SystemScope 必须含原因和操作者并写审计。SQL 参数绑定，日志不输出 DSN/参数值。

## 11. 日志、指标与 Trace 要求

记录查询名、模块、耗时、行数、错误类和 trace；不记录 SQL 参数。慢查询阈值 500ms；指标名称由 06 定义。

## 12. 测试矩阵及具体验收场景

Testcontainers 验证跨租户不可读、缺 Scope 失败、业务和 Outbox 同回滚、并发唯一冲突、Relay 并发领取唯一、过期租约恢复、空库/基线库 migration 以及 SQLC 查询编译。

## 13. 性能边界与容量假设

单实例最大 50 连接；常规查询预算 3s、事务 10s；Outbox 领取索引支持百万级未归档记录，发布记录 30 天后归档。

## 14. 发布、迁移、回退和兼容要求

文件名 `YYYYMMDDHHMMSS_description.up.sql/.down.sql`；生产只执行 Up，采用 expand/contract。CI 从空库和脱敏基线升级；Down 仅本地验证且不得假装可逆数据恢复。

## 15. 未解决问题

无未解决问题。

### Outbox 表（本设计唯一所有者）

```sql
CREATE TABLE outbox_events (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  event_id CHAR(36) NOT NULL,
  tenant_id BIGINT UNSIGNED NOT NULL,
  aggregate_type VARCHAR(100) NOT NULL,
  aggregate_id VARCHAR(128) NOT NULL,
  event_type VARCHAR(160) NOT NULL,
  schema_version SMALLINT UNSIGNED NOT NULL,
  payload JSON NOT NULL,
  headers JSON NOT NULL,
  status ENUM('pending','processing','published','dead') NOT NULL DEFAULT 'pending',
  attempts SMALLINT UNSIGNED NOT NULL DEFAULT 0,
  available_at DATETIME(6) NOT NULL,
  lease_owner VARCHAR(128) NULL,
  lease_until DATETIME(6) NULL,
  last_error_code VARCHAR(64) NULL,
  created_at DATETIME(6) NOT NULL,
  published_at DATETIME(6) NULL,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (id), UNIQUE KEY uk_outbox_event_id (event_id),
  KEY ix_outbox_claim (status, available_at, lease_until, id),
  KEY ix_outbox_tenant_created (tenant_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

CAS：`pending -> processing` 或租约过期的 `processing -> processing` 时必须匹配旧状态/旧租约；发布成功 `processing -> published` 必须匹配 owner；失败按策略 `processing -> pending`，耗尽后 `processing -> dead`。
