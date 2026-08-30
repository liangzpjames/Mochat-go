# Task 6 风险规则与关键词事务实施报告

## 根因与方案

1. 风险执行复用了 UI 的 `RiskRulePage`，其 `PerPage` 上限为 100，导致第 101 条及以后规则静默失效。新增执行专用的租户/企业范围稳定查询，按 `rule.id,strategy.id` 全量读取启用规则；UI 分页合同保持不变。
2. 风险记录逐条独立提交，`trigger_count` 更新错误被丢弃。改为同一事务内读取规则快照、写入全部命中记录并更新计数；任何非重复键错误均整体回滚。重复消息仅识别唯一键冲突，不再使用会吞掉其他数据库错误的 `INSERT IGNORE`。
3. 风险规则创建先写规则再写策略，策略失败会留下无策略规则。规则和策略现由同一事务提交。更新、状态、删除和审计同时使用 `tenant_id + corp_id`；删除先锁定所属规则，避免错误范围先删除策略。
4. 关键词 entry 与 `draft_version` 原本是两个独立提交点且忽略后者错误。保存时先按租户/企业锁定 library，再在同一事务写 entry、检查 `RowsAffected`、递增版本并提交；并发写被 library 行锁串行化，不丢版本。

## TDD 证据

### RED

首次执行：

```text
go test ./internal/store -run 'Test(EvaluateRiskMessage|CreateRiskRule|SaveKeywordEntry)' -count=1
FAIL
- EvaluateRiskMessage 实际先执行 UI COUNT/分页，未开始事务；
- SaveKeywordEntry 实际直接 INSERT SELECT，未开始事务；
- CreateRiskRule 未开始事务。
```

租户范围接口 RED：`MySQLStore` 因旧 `DeleteRiskRule(context.Context,int,int64)` 签名不能满足 tenant/corp writer 合同而编译失败。去除 `INSERT IGNORE` 的 RED 也确认旧 SQL 与新合同不匹配。

### GREEN / PASS

- `go test ./internal/store ./internal/dashboard -run 'Test(EvaluateRiskMessage|CreateRiskRule|UpdateRiskRule|SaveKeywordEntry|RiskBehaviorHandler)' -count=1`：PASS。
- `go test ./internal/store ./internal/dashboard -count=1`：PASS。
- `docker run --rm ... golang:1.26.7-bookworm go test -race ./internal/store -run 'Test(EvaluateRiskMessage|CreateRiskRule|UpdateRiskRule|SaveKeywordEntry)' -count=1`：Linux/CGO race PASS。
- MariaDB 10.6 隔离容器：第 101 条命中、计数失败触发器整体回滚、16 并发重复消息仅一条记录/一次计数、关键词版本失败整体回滚、16 并发版本无丢失：PASS。
- MySQL 5.7.44 隔离容器：同一组真实 SQL、故障触发器和并发测试：PASS。
- 两个数据库容器均使用 `--rm` 且不挂载命名卷；验证后已停止并删除，没有触碰现有业务卷。

### SKIP / NOT RUN

- 未调用真实企业微信、真实 AI Provider、生产服务器或真实租户凭据；这些边界均 NOT RUN，且与本任务数据层合同无关。

## 变更范围

- `internal/store/risk_behavior.go`
- `internal/store/message_intercept.go`
- `internal/dashboard/risk_behavior.go`
- `internal/dashboard/risk_behavior_handler.go`
- `internal/store/risk_behavior_integration_test.go`
- `internal/store/message_intercept_integration_test.go`

未修改 `docs/PROJECT_PROGRESS.zh-CN.md`，未增加迁移；本次只修正既有表上的查询和事务边界，SQL 已在 MariaDB 10.6 与 MySQL 5.7.44 实际执行。
