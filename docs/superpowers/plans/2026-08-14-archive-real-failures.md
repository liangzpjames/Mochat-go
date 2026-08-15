# Archive 真实 MariaDB 失败修复实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复 archive read、0138 legacy backfill、Enqueue 状态回读及三张归档表 guard，使同一临时 MariaDB required-real 能真实验证这些合同。

**Architecture:** 以显式 `mochat_go_archive_message_sources` registry 作为 read-side source identity 权威，所有 count/items/page 查询使用同一 SQL predicate。0138 backfill 的每个 INSERT/SELECT/UPDATE 都使用明确目标表别名；guard 按表独立验证列、索引和复合 FK，坏 fixture 先建立其余完整签名后只破坏一个目标。Enqueue 在 INSERT 后使用同一 scope 的明确回读，返回完整 queued 状态。

**Tech Stack:** Go、`database/sql`、MariaDB 10.6、Go migration runner、Go integration tests。

## Global Constraints

- 只在 `D:\workspace\mochat-go\mochat-go\.worktrees\phase6-provider-foundation-wecom-closeout` 写入。
- 不操作 Docker、服务器、生产数据库；MariaDB 仅允许主任务使用临时 schema 验证。
- 不能修改已应用 0127-0137 migration 或无关迁移常量。
- 每个逻辑批次先写 RED，再写最小 GREEN；保持既有提交历史不改写。
- required-real 缺失 DSN 时不得把 SKIP 计为 GREEN。

---

### Task 1: 修复 registry read-side SQL

**Files:**
- Modify: `internal/store/archive_source_read.go` 及其实际查询实现
- Test: `internal/store/archive_source_read_integration_test.go`

**Interfaces:**
- 消费现有 `mochat_go_archive_message_sources` registry 与 work-message partition 查询。
- 保持 `WorkMessageToUsers`、`WorkMessagePage`、Detail 的返回字段和 source identity 不变。

- [ ] **Step 1: 写失败测试**：在临时 fixture 中插入 registry simulated 但 msgid 不带 `MOCHAT-SIM:`、registry external 但 msgid 带该前缀、无 registry 历史真实消息；分别调用 real/simulation 查询并断言 items、source、Total、TotalPage。
- [ ] **Step 2: 运行 RED**：
  `go test ./internal/store -run TestArchiveSourceReadUsesRegistryForItemsCountsAndPages\|TestArchiveSourceReadLegacySimulationRegistryStaysOutOfExternalDefault -count=1`
  预期看到 simulated items=0 或 source/count 不匹配。
- [ ] **Step 3: 修改生产查询**：将 registry join/filter 下推到 SQL；显式限定所有 `id`/`msgid`/scope 列；确保 COUNT 与分页 SELECT 复用同一 source predicate。
- [ ] **Step 4: 运行 GREEN**：同一命令在有 DSN 时通过；无 DSN 只允许明确 SKIP。
- [ ] **Step 5: 独立提交**：`git commit -m "fix archive read source registry filtering"`。

### Task 2: 修复 0138 legacy backfill 与 ambiguous column

**Files:**
- Modify: `deploy/standalone/migrations/0138_archive_source_sync.up.sql`
- Test: `internal/store/archive_sync_integration_test.go`、`internal/store/archive_source_read_integration_test.go`

**Interfaces:**
- 继续使用生产 migration runner 的 session-pinned statement 执行。
- legacy simulation batch、corp、message、run 的 scope 和 source identity 不变。

- [ ] **Step 1: 写失败回归**：真实临时 schema 的 legacy backfill 与 legacy read 测试必须覆盖 migration 和 read 两条路径，并断言无 `Column 'id' ... ambiguous`。
- [ ] **Step 2: 运行 RED**：
  `go test ./internal/store -run TestArchiveSourceMigrationBackfillsLegacySimulationRowsOnTemporaryMariaDB\|TestArchiveSourceReadLegacySimulationRegistryStaysOutOfExternalDefault -count=1`
  预期复现 `id` ambiguous 或读侧失败。
- [ ] **Step 3: 最小修复**：给 INSERT...SELECT 的 `corp`、`batch`、`message`、`run` 明确别名；`ON DUPLICATE KEY UPDATE` 右值明确绑定目标表；所有 correlated `NOT EXISTS` 的字段显式限定。
- [ ] **Step 4: 运行 GREEN**：同一测试命令通过，并检查 cleanup=0。
- [ ] **Step 5: 独立提交**：`git commit -m "fix archive legacy backfill aliases"`。

### Task 3: 修复 Enqueue queued 状态回读

**Files:**
- Modify: `internal/modules/providers/archive/*` 中的 durable sync store Enqueue 实现
- Test: `internal/store/archive_sync_integration_test.go`

**Interfaces:**
- 保持 idempotency、namespace、lease/fence 和 tenant/corp scope 合同。
- `Enqueue` 返回的 `archive.SyncRun.Status` 必须反映数据库中的 `queued`。

- [ ] **Step 1: 写失败测试**：创建合法 scope 后 Enqueue，断言返回 `Status == "queued"`、attempt/idempotency 正确；duplicate reread 也断言状态不为空。
- [ ] **Step 2: 运行 RED**：
  `go test ./internal/store -run TestArchiveSyncStoreUsesTemporarySchemaForLifecycleAndTenantIsolation -count=1`
  预期返回 `Status:""`。
- [ ] **Step 3: 最小修复**：检查 INSERT 参数/Scan 顺序，使用明确列列表回读完整 run，禁止以零值结构覆盖数据库状态。
- [ ] **Step 4: 运行 GREEN**：定向测试及 duplicate/namespace/stale 测试通过。
- [ ] **Step 5: 独立提交**：`git commit -m "fix archive enqueue status reread"`。

### Task 4: 修复 0138 三表 guard 与单因子 fixture

**Files:**
- Modify: `deploy/standalone/migrations/0138_archive_source_sync.up.sql`
- Test: `internal/store/archive_sync_integration_test.go`

**Interfaces:**
- runs、audits、message_sources 各自只在表存在且签名完整时放行。
- source/audit 复合 FK signature 需绑定正确的 child columns、referenced columns 和 constraint name；坏 fixture 必须先通过 runs guard。

- [ ] **Step 1: 写/收紧 RED**：wrong source FK、non-unique source scope index、wrong audit scope index fixture 先 apply 正常 0138，再只 drop/recreate 目标表；断言错误分别为 sources/audits guard，且不是 runs guard。
- [ ] **Step 2: 运行 RED**：
  `go test ./internal/store -run 'TestArchiveSyncMigrationRejectsWrongCompositeSourceForeignKey|TestArchiveSyncMigrationRejectsNonUniqueResidualScopeIndex|TestArchiveSyncMigrationRejectsWrongAuditScopeIndex' -count=1`
  预期当前至少有目标 guard 未触发或误报 runs。
- [ ] **Step 3: 最小修复**：修正 information_schema 查询的约束/索引 signature 聚合和 guard 顺序；每个 dynamic guard 在第一 DDL 前执行并 fail closed，不允许坏表被 `CREATE IF NOT EXISTS` 静默接受。
- [ ] **Step 4: 运行 GREEN**：三个单因子测试、apply-down-apply、incomplete residual 和 idempotency bad table 通过。
- [ ] **Step 5: 独立提交**：`git commit -m "fix archive migration table guards"`。

### Task 5: Fresh verification and handoff

- [ ] 运行定向 Go 测试：
  `go test ./internal/migration ./internal/store ./internal/archivesim ./internal/testfixtures/archivesource -run 'TestSplitSQLStatements|TestArchiveSource|TestArchiveSync|TestPrepareDashboardPermissionDependencies|TestApplyCleanupReapply' -count=1`
- [ ] 运行普通门禁：`pnpm check:provider-completion`、`pnpm check:phase4-dashboard-page-rbac`。
- [ ] 如本线程无 `MOCHAT_GO_MYSQL_INTEGRATION_DSN`，明确报告 required-real SKIP；不把它称为 MariaDB GREEN。
- [ ] 运行 `git diff --check`、`git status --short`，确认提交 clean。
- [ ] 回报每个 commit SHA、测试结果、真实 MariaDB 外部阻塞和未改动 Docker/业务库事实。
