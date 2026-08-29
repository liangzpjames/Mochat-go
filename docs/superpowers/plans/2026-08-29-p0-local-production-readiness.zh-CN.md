# MoChat Go 本地可闭环 P0 发布底座 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在不使用真实企微/AI/生产凭据的前提下，修复归档角色、callback 不丢、运行时韧性、数据原子性、九条门禁和供应链基线，并以真实本地 MariaDB/Redis、Linux/CGO、Docker 和浏览器分层验收。

**Architecture:** 用职责矩阵替代开关级 role 裁剪；用 DB durable inbox 承接 callback ACK；用统一 service group 管理 HTTP/worker 生命周期；用租户/企业范围 receipt 和事务关闭数据一致性缺口；门禁统一消费代码注册表和迁移发现器。

**Tech Stack:** Go 1.26.7、MySQL 5.7/MariaDB、Redis、React 19、Node 24、pnpm 11、Docker Compose、GitHub Actions。

## Global Constraints

- 基线固定为 `f2b57f31f2baa93dc871b9157a04b0a8f7e2ae36`；只在 `fix/p0-local-closure-20260829` 隔离 worktree 修改。
- 禁止 reset/clean、覆盖用户文件、删除或复用用户命名卷；禁止修改 `docs/PROJECT_PROGRESS.zh-CN.md`。
- 所有行为变更先 RED 后 GREEN；fixture/fake 只证明本地合同，不证明真实 Provider。
- 新迁移必须 up/down、MySQL 5.7 兼容、租户/企业范围唯一、并发测试。
- 不访问真实企微、AI Provider、生产服务器或真实租户凭据。

---

### Task 1: 归档职责矩阵

**Files:**
- Modify: `internal/app/runtime/role.go`
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`
- Modify: `cmd/mochat-go/archive_runtime.go`
- Test: `cmd/mochat-go/archive_runtime_test.go`
- Modify: `cmd/mochat-go/main.go`

**Interfaces:**
- Produces: `runtime.Responsibilities`，显式字段 `API`、`Workers`、`Schedulers`、`DurableArchiveAPI`、`DurableArchiveWorkers`、`ArchiveEnqueuer`。
- Keeps: durable/cron 四组合语义；role 只裁剪职责，不改用户原始开关含义。

- [ ] 写覆盖 `4 role × durable on/off × cron on/off` 的 RED，证明 worker 保留 durable/media、scheduler 只保留 enqueuer。
- [ ] 运行 `go test ./internal/app/runtime ./internal/config ./cmd/mochat-go -run 'Test.*Runtime|TestArchiveRuntime' -count=1`，确认因当前错误裁剪失败。
- [ ] 最小实现职责矩阵并让 main 只按矩阵注册 task。
- [ ] 运行相关包和 `pnpm check:durable-archive-scheduler` GREEN。
- [ ] 提交 `fix(runtime): separate archive workers from scheduling roles`。

### Task 2: bridge production registrar 与关闭生命周期

**Files:**
- Create: `internal/archivebridge/registrar.go`
- Test: `internal/archivebridge/registrar_test.go`
- Modify: `internal/archivebridge/store.go`
- Test: `internal/archivebridge/store_test.go`
- Modify: `cmd/mochat-archive-bridge/main.go`
- Test: `cmd/mochat-archive-bridge/main_test.go`
- Modify: `deploy/standalone/docker-compose.yml`

**Interfaces:**
- `ProductionBindingSource` 只返回不含明文凭据的租户/企业 binding；`DriverFactory` 在启动边界解密受控 credential 并构造 Finance/DataZone driver；`DriverRegistrar` 负责全量注册和逆序关闭。
- `run` 接受 injectable registrar factory；fixture 与 production SDK 模式互斥。
- 所有 binding 注册完成才监听；关闭顺序为 HTTP drain → registrar close。

- [ ] RED：fake registrar 覆盖 self-built/DataZone、租户/企业错配、游标、媒体分片、component、初始化失败和 Close 失败。
- [ ] 运行 `go test ./internal/archivebridge ./cmd/mochat-archive-bridge -count=1`，确认缺少生产注册调用。
- [ ] 实现 registrar 生命周期；SDK 模式零 driver、Windows/未提供 SDK factory 均明确 `ARCHIVE_DRIVER_UNAVAILABLE`，不回退 fixture，也不把 `/healthz` 误报为 production ready。
- [ ] 在 Linux 合同容器注入 fake factory，证明注册/解析/游标/媒体/关闭；不调用真实企微。
- [ ] 提交 `feat(archive): wire fail-closed bridge driver registration`。

### Task 3: durable callback inbox 与 replay

**Files:**
- Create: `deploy/standalone/migrations/0172_wework_callback_inbox.up.sql`
- Create: `deploy/standalone/migrations/0172_wework_callback_inbox.down.sql`
- Create: `internal/dashboard/wework_callback_inbox.go`
- Test: `internal/dashboard/wework_callback_test.go`
- Create: `internal/store/wework_callback_inbox.go`
- Test: `internal/store/wework_callback_inbox_integration_test.go`
- Modify: `internal/dashboard/wework_callback.go`
- Modify: `cmd/mochat-go/main.go`

**Interfaces:**
- Handler 依赖 `AcceptWeWorkCallback(ctx,event,key,fingerprint) (replayed bool,error)`，不直接依赖 Redis ACK。
- Worker 依赖 `Claim/Complete/Fail` lease 接口；Redis 只作可选 wakeup。

- [ ] RED：DB error 返回 503；Redis unavailable 但 DB accepted 返回 success；同事件重试只一条；同 key 不同 payload 冲突；过期 timestamp 拒绝。
- [ ] 运行 dashboard/store 目标测试，确认当前吞错和 10 分钟 replay 缺口。
- [ ] 写 0172 up/down 和 store 事务/lease；handler ACK 绑定 receipt。
- [ ] 在独立 MariaDB/Redis 做并发 32 次同事件、DB kill/recover、Redis kill/recover。
- [ ] 提交 `fix(callback): acknowledge only durable event acceptance`。

### Task 4: HTTP service group、readiness 与 periodic panic

**Files:**
- Create: `internal/runtimegroup/http.go`
- Test: `internal/runtimegroup/http_test.go`
- Modify: `cmd/mochat-go/main.go`
- Modify: `internal/server/server.go`
- Test: `internal/server/server_test.go`
- Modify: `internal/taskrunner/taskrunner.go`
- Test: `internal/taskrunner/taskrunner_test.go`
- Modify: `scripts/deploy_docker_desktop.ps1`
- Modify: `deploy/standalone/docker-compose.yml`

**Interfaces:**
- 三个 listener 都由 `http.Server` 持有 5s ReadHeader、15s Read、30s Write、60s Idle（streaming 路由单独显式覆盖）。
- `ReadinessChecker` 返回稳定 code，不泄露 DSN/Redis 地址。
- `Periodic` 在每个 tick 边界 recover，记录失败后继续下一 tick。

- [ ] RED：慢 header 超时、SIGTERM 在途请求完成、DB/Redis/migration behind 摘流、panic 后第二 tick 执行。
- [ ] 运行 `go test ./internal/runtimegroup ./internal/server ./internal/taskrunner ./cmd/mochat-go -count=1`。
- [ ] 实现统一 root context/service group/readiness，部署先 migrate 后 app。
- [ ] 故障注入断开 DB/Redis，验证 `/healthz=200`、`/readyz!=200`，恢复后 ready。
- [ ] 提交 `fix(runtime): add bounded servers readiness and panic recovery`。

### Task 5: 订单端到端幂等

**Files:**
- Create: `deploy/standalone/migrations/0173_scrm_order_idempotency.up.sql`
- Create: `deploy/standalone/migrations/0173_scrm_order_idempotency.down.sql`
- Modify: `internal/modules/scrm/domain/order.go`
- Modify: `internal/modules/scrm/transport/http/order_handler.go`
- Test: `internal/modules/scrm/transport/http/order_handler_test.go`
- Modify: `internal/modules/scrm/adapters/mysql/order_repository.go`
- Test: `internal/modules/scrm/adapters/mysql/order_repository_integration_test.go`
- Modify: `web/apps/dashboard/src/features/phase35/order-page.tsx`
- Test: `web/apps/dashboard/src/features/phase35/order-page.test.tsx`

- [ ] RED：丢失首次响应后重试、32 并发同 key、同 key 不同 payload、跨 tenant/corp 同 key。
- [ ] 前端一次用户意图只生成一次 key，失败重试复用，成功/取消后清除。
- [ ] repository 在事务中创建 receipt+order并保存首次响应；duplicate 读取首次结果。
- [ ] 在 MySQL 5.7/MariaDB 运行并发测试与 up/down。
- [ ] 提交 `fix(scrm): make order creation idempotent end to end`。

### Task 6: 风险规则与关键词事务

**Files:**
- Modify: `internal/store/risk_behavior.go`
- Test: `internal/store/risk_behavior_integration_test.go`
- Modify: `internal/store/message_intercept.go`
- Test: `internal/store/message_intercept_integration_test.go`

- [ ] RED：第 101 条启用规则命中；插入第 N 条失败整批回滚；trigger_count 失败回滚；关键词 entry/version 任一步失败整体回滚。
- [ ] 新增执行专用 cursor query；UI pagination 保持原合同。
- [ ] 用事务锁定 rule/library，检查 `RowsAffected`，错误不忽略。
- [ ] 在 MariaDB 运行真实 SQL 与并发更新测试。
- [ ] 提交 `fix(store): make risk and keyword writes complete and atomic`。

### Task 7: 九条门禁与 CI/迁移治理

**Files:**
- Modify: `scripts/check_phase34_lint.mjs`
- Modify: `scripts/check_phase3_5_dashboard_completion.mjs`
- Modify: `scripts/check_debt_clearance.mjs`
- Modify: `scripts/check_dashboard_auth_context.mjs`
- Modify: `scripts/check_identity_realm_single_corp.mjs`
- Modify: `scripts/check_wecom_archive_saas_activation.mjs`
- Modify matching `*.test.mjs`
- Modify: `docs/phases/phase-3-dashboard/benchmark/README.md`
- Modify: `docs/phases/phase-2-frontend-migration/audit/phase2-progress.csv`
- Modify: Phase 2.1 functional matrix/current manifest
- Modify: `.github/workflows/mysql57-amd64.yml`
- Modify: `scripts/smoke_mysql57_schema_migrate.sh`
- Modify: `scripts/smoke_schema_migrate.sh`

- [ ] 为每条当前失败先补/更新脚本单测，证明正确合同会通过、错误合同会失败。
- [ ] phase34 lint 分批执行且覆盖集合不变；phase3 路由来自单一 manifest。
- [ ] auth/identity/archive 门禁消费注册表/明确 projection，不扩大安全例外。
- [ ] phase2 报告由当前 manifest 稳定生成，修复真实文档链接与 `/corp/index` 陈旧引用。
- [ ] CI build-before-audit；迁移 smoke 用发现器的 count/latest/checksum，完整执行 0001–最新。
- [ ] 九条命令逐项 GREEN 后提交 `fix(gates): align release checks with runtime contracts`。

### Task 8: 0165 受控迁移与供应链

**Files:**
- Create: `scripts/preflight_0165_ai_daily_insight_unification.go`
- Create: `docs/deployment/2026-08-29-0165-controlled-migration.zh-CN.md`
- Modify: `go.mod`
- Modify: `Dockerfile`
- Modify: `.github/workflows/mysql57-amd64.yml`
- Create/Modify: dependency audit/SBOM/provenance workflow and policy docs

- [ ] 测试 preflight 对重复、将删除、备份缺失和计数漂移 fail closed；不改 0165 历史 checksum。
- [ ] 固定 Go 1.26.7、builder/runtime/Node digest；固定 Actions commit SHA。
- [ ] 加入 govulncheck、pnpm audit policy、SBOM 和 artifact provenance；签名保持 NOT CONFIGURED。
- [ ] 运行 govulncheck 要求 reachable=0；逐条记录 pnpm critical/high 的依赖路径与可达性。
- [ ] 提交 `build: pin patched toolchains and add supply-chain evidence`。

### Task 9: 全量本地验收与最终审查

**Files:**
- Create: `docs/reviews/2026-08-29-p0-local-production-readiness-acceptance.zh-CN.md`

- [ ] 运行 `go test ./... -count=1`、`go vet ./...`、适用 Linux `go test -race`。
- [ ] 运行 `pnpm lint`、`pnpm typecheck`、`pnpm test`、`pnpm build`、所有 `check:*` 和 `docs:check`。
- [ ] 独立 MariaDB/Redis 执行全迁移、store integration、并发、故障恢复；准确列 SKIP。
- [ ] 构建 `mochat-go:p0-<fullSHA>`，记录 image ID/digest、SBOM、source fingerprint，不触碰现有命名卷。
- [ ] 本地启动四端，验证 health/ready、迁移、日志轮转、断依赖恢复及浏览器核心流程。
- [ ] 生成 whole-branch review package，独立 reviewer 检查 Critical/Important，修复后复验。
- [ ] 提交最终验收文档并输出精确 SHA、PASS/FAIL/SKIP 和真实 Provider 边界。

### Task 10: 真实 migration registry 集成 harness

**Files:**
- Create: `internal/integrationtestdb/database.go`
- Test: `internal/integrationtestdb/database_test.go`
- Create: `internal/migration/testharness/registry.go`
- Test: `internal/migration/testharness/registry_test.go`
- Modify: `internal/store/*_integration_test.go` 中读取 `MOCHAT_GO_MYSQL_INTEGRATION_DSN` 的测试
- Modify: `internal/migration/*_integration_test.go` 中读取 `MOCHAT_GO_MYSQL_INTEGRATION_DSN` 的测试

**Interfaces:**
- `integrationtestdb.NewIsolated(t, adminDSN)` 创建随机数据库并注册 seed rollback + drop cleanup，不创建业务表。
- `testharness.ApplyThrough(ctx, db, root, targetVersion, ControlledEvidence)` 只通过生产 registry/runner 建 schema；`ApplyLatest` 覆盖完整注册表。
- `ControlledEvidence` 用正式 staging/backup API 满足 `0130` 与 `0165`，禁止直接写 migration ledger。

- [ ] RED：在隔离 MariaDB 10.6 设置 DSN 运行完整 `go test ./internal/store ./internal/migration -count=1`，保存每个局部 schema 与 registry/0130 冲突。
- [ ] 为 harness 写失败测试：拒绝复用 DSN 中已有 database、失败时执行 seed rollback、跨 0130 缺证据 fail closed、同一进程并发数据库名唯一。
- [ ] 实现通用数据库生命周期和 registry prefix/latest 执行；为 0130/0165 复用正式受控证据 API。
- [ ] 将每个历史测试改成“registry 前缀/latest + 场景 seed”，删除测试内业务 `CREATE TABLE` 与伪 ledger；不减少原断言、测试名或覆盖场景。
- [ ] 在 MariaDB 10.6 和 MySQL 5.7 分别运行完整两包测试；测试结束核对无残留临时数据库/账户。
- [ ] 提交 `test(db): build integration fixtures from migration registry`。

### Task 11: unknown side effect 权限化恢复

**Files:**
- Create: `deploy/standalone/migrations/0175_wework_callback_side_effect_reconciliation.up.sql`
- Create: `deploy/standalone/migrations/0175_wework_callback_side_effect_reconciliation.down.sql`
- Create: `internal/dashboard/wework_callback_side_effect_admin.go`
- Test: `internal/dashboard/wework_callback_side_effect_admin_test.go`
- Modify: `internal/dashboard/wework_callback_inbox.go`
- Modify: `internal/dashboard/dashboard_route_registry.go`
- Modify: `internal/dashboard/dashboard_page_catalog.json`
- Modify: `internal/store/wework_callback_side_effect.go`
- Create: `internal/store/wework_callback_side_effect_reconciliation_integration_test.go`
- Modify: `cmd/mochat-go/main.go`
- Create: `docs/deployment/2026-08-30-wework-callback-side-effect-reconciliation.zh-CN.md`

**Interfaces:**
- `ListUnknownSideEffects`/`GetSideEffect` 只接收服务端 principal 解析出的 tenant/corp。
- `ReconcileSideEffect` 接收 request id、expected version、`confirm_sent|confirm_not_sent_and_retry` 和 reason；在同一事务锁定 intent/inbox、写 command receipt/audit、推进 action 并复活 inbox。
- command receipt 按 tenant/corp/request id 幂等；payload hash 冲突、版本/fence/状态冲突返回稳定领域错误。

- [ ] RED：无权限 403、跨租户/企业 404、列表/详情不泄露；同请求重放首次响应、异义 409；并发双操作仅一成功。
- [ ] RED：审计失败/inbox 更新失败整体回滚；活动 lease 或 fence 变化 409；两个 action 可独立 confirm/retry，已 sent action 不回退。
- [ ] 写 0175 up/down 并运行 MySQL 5.7 合同测试；实现 store 事务、租户范围 query、乐观版本和审计。
- [ ] 注册三个 Dashboard 路由及 `dashboard.company_setting.website` RBAC 映射；实现 400/401/403/404/409/503 稳定错误语义和 body limit。
- [ ] 用 fake Provider 证明 `confirm_sent` 后跳过已发 action、`confirm_not_sent_and_retry` 后仅对应 action 重试；不发真实网络请求。
- [ ] 编写中文运维说明，明确取证、双人复核建议、命令幂等键、审计查询和绝不“猜测已发送”。
- [ ] 提交 `feat(callback): add audited unknown side-effect reconciliation`。

### Task 12: 全量复验、干净合入与普通推送

**Files:**
- Modify: `docs/superpowers/reports/2026-08-29-p0-local-production-readiness-final-report.zh-CN.md`

- [ ] 复跑全量 Go test/vet、前端 lint/typecheck/test/build、全部仓库门禁、govulncheck、pnpm audit、migration lifecycle、`git diff --check`。
- [ ] MariaDB 10.6 与 MySQL 5.7 完整运行 `go test ./internal/store ./internal/migration -count=1`；Linux/CGO race 覆盖新增 harness/reconcile。
- [ ] 构建候选精确 SHA Docker 产物，验证 175/175 migration、health/ready、故障恢复、四端入口和相关本地浏览器/API 流程。
- [ ] 独立 reviewer 对续作提交和 whole branch 给出规格/质量结论，修复全部 Critical/Important。
- [ ] `git fetch origin --prune` 并核对 `ls-remote`；从最新 `origin/main` 创建新的干净临时 integration worktree，普通 merge 候选，复跑合入后关键门禁。
- [ ] 非 force 推送临时集成 HEAD 到 `origin/main`；再次以 `git ls-remote origin refs/heads/main` 与本地集成提交双核验精确 SHA。
- [ ] 只清理本任务临时容器、网络、数据库/账户和临时 integration worktree；保留命名卷、候选 worktree、用户文件。

## 计划自审

- [x] 每个行为任务都有 RED 命令、GREEN 命令、精确文件和提交边界。
- [x] 迁移任务包含 up/down、MySQL 5.7、并发和失败恢复。
- [x] 角色、callback、readiness、订单、风险/关键词、门禁、供应链和验收覆盖设计全文。
- [x] 不含 TBD/TODO、空文件兼容、真实 Provider 调用或生产部署步骤。
- [x] 明确先迁移后 readiness，避免当前部署脚本死锁。
- [x] 续作覆盖历史全量 DSN、真实 registry 前缀/最新 schema、0130/0165 受控证据和可回滚 seed cleanup。
- [x] unknown 恢复覆盖权限、租户隔离、审计、幂等、lease/fence、并发、双 action、错误语义和 fake Provider 边界。
- [x] 合入使用干净临时 worktree、普通 merge/push 和远端精确 SHA 双核验，不修改脏 main 工作树。
