# Task 7：发布门禁与迁移治理实施报告

日期：2026-08-29

## 结论

九条门禁的陈旧合同已对齐当前运行时。独立审查指出的可骗绿问题已返工：Dashboard 鉴权改为 Go typed route registry；Phase 2 直接读取三端当前 manifest；归档门禁执行角色、production registrar 和关闭行为测试；Phase 3.4 以 Node 直接启动 ESLint 且命令预算低于 7000 字符；identity projection 仅将 not-found/inactive 映射为 revoked，数据库错误继续传播。

迁移 smoke 已统一消费 `mochat-migrate -action inventory` 的 TSV，动态取得 count/first/latest/checksum/kind，并覆盖完整 ledger、status、最新可回滚迁移 rollback-reapply、baseline 和 checksum drift。当前 inventory 为 173 条，`0001_initial_schema` 到 `0173_scrm_order_idempotency`。MySQL 5.7 首轮实跑暴露 0109 的 conditional ALTER 语法不兼容，证明旧 smoke 的“最新”断言并未真实闭环；现以 `information_schema` 元数据守卫兼容执行，保持原始 SQL 与 checksum 不变。

## 根因与方案

1. Dashboard auth：旧 audit 从字符串白名单推断，且 `securityMFA` 被默认 unbound 例外放行。现在 597 条结构化 method/path/handler/auth kind 作为运行时 guard 与 audit 的共同策略源；audit 独立发现生产注册并校验 handler/auth，缺失 metadata、注释伪装、handler/auth mutation 均失败。
2. Phase 2 progress：旧报告读 Phase 1 CSV，不能感知当前路由增删。现在严格读取 dashboard/sidebar/operation 三个 `migration-routes.json`，拒绝未知 app/status、重复和错误归属，稳定排序后生成 CSV；当前为 82/82 React。
3. Archive gate：旧脚本以 `body.includes` 检查死代码片段。现在执行 Go 行为测试，覆盖完整角色矩阵、fixture/production 互斥、`RegisterAll` 失败时 server factory 零调用，以及 graceful shutdown 必须调用 registrar `Close`；Node mutation 测试确保任一行为测试失败都会使门禁失败。
4. Phase 3.4：Windows 上通过 shell 启动 pnpm 且默认 12000 字符预算。现在直接以 `process.execPath` 启动 ESLint JS、`shell:false`、文件参数预算 6000；超长单文件显式失败，不静默遗漏。
5. Identity projection：`ResolveIdentity` 把任意 DB/scan 错误折叠成 principal unavailable，激活状态进一步误报 revoked。现在只把 `sql.ErrNoRows` 和 inactive 映射为 unavailable/revoked，其余错误原样传播。
6. Migration smoke：两个历史脚本硬编码 0098、删除命名卷且没有跨 controlled migration。现在共享 inventory 生命周期，正式调用 identity 0130/0131 与 AI insight 0165 受控 CLI；清理只 `compose down --remove-orphans`，不删除卷。
7. MySQL 5.7：0109/0153/0167 使用 MySQL 5.7 不支持的 conditional ALTER。runner 仅在语句需要兼容时查询 server version；对 5.7 按列/索引元数据过滤已满足 clause，再执行去掉条件修饰的剩余 clause。迁移文件与账本 checksum 不变。

## TDD 与验证证据

### RED

- typed registry 测试初始因 API 不存在编译失败；audit mutation/securityMFA 测试失败。
- Phase 2 当前 manifest 生成后，旧报告稳定复现 stale。
- production registrar 新测试初始因 `runWithServerFactory` 不存在编译失败。
- identity 数据库错误测试初始得到 revoked/nil，未传播原错误。
- Phase 3.4 新测试初始因 direct ESLint invocation 不存在失败。
- smoke 合同初始因共享 inventory lifecycle 不存在失败。
- 真实 MySQL 5.7 fresh apply 初始仅落账 108 条，最新 `0108_scrm_public_pool_parity`，0109 失败。
- metadata-guarded 兼容修复后，同一 fresh schema 重试成功越过 0109，到达 0130 controlled boundary；正式执行 0130、credential encryption 与 0131 后，自动迁移继续到 0138。0139 在 MySQL 5.7 因动态 guard 生成的 `SIGNAL` 不能经 prepared statement 执行而失败。

### GREEN / PASS

- `node --test scripts/check_dashboard_auth_context.test.mjs`：6/6 PASS；生产 audit 597 routes、missing 0、violations 0。
- `go test ./internal/dashboard -run 'DashboardRouteRegistry|DashboardSecurityMFA|DashboardRoutePolicy|DashboardAccessGuard' -count=1`：PASS。
- `node --test scripts/check_phase2_frontend_progress.test.mjs`：4/4 PASS；`node scripts/check_phase2_frontend_progress.mjs`：82/82 PASS。
- `node --test scripts/check_wecom_archive_saas_activation.test.mjs`：2/2 PASS；行为门禁 PASS。
- `node --test scripts/check_phase34_lint.test.mjs scripts/check_migration_smoke_inventory.test.mjs`：8/8 PASS。
- `go test ./internal/store -run 'TestDashboardActivationStatus|TestResolveDashboardIdentity' -count=1`：PASS。
- `go test ./internal/migration -count=1`：PASS；包含 MySQL 5.7 metadata-guarded ALTER 单测。
- `go run ./cmd/mochat-migrate -project-root . -action inventory`：173 条，0001 到 0173，latest kind=automatic。

### FAIL / SKIP

- PASS（真实 MySQL 5.7）：metadata-guarded conditional ALTER 修复已把账本从 0108 推进到 0129；0130、credential encryption、0131 controlled 流程成功，随后自动迁移推进到 0138。
- FAIL（新发现、未完成）：0139 的动态 schema guard 在 MySQL 5.7 命中 `SIGNAL` prepared-statement 限制，完整 0001→0173 尚未闭环；因此 rollback-reapply/baseline/drift 的脚本路径仍未取得真实数据库 PASS，不能把脚本/单测称作数据库完成。
- SKIP：当前 Windows 主机无可用 `/bin/bash`，两个 shell entrypoint 的 `bash -n` 无法本机执行；Node 结构合同已 PASS，Linux CI/父任务需执行脚本本体。
- SKIP：本任务没有调用真实企微、AI Provider 或生产环境。

## 可复用 smoke 接口

- MySQL 5.7：`KEEP_STACK=1 MOCHAT_STACK_PROJECT=<project> MOCHAT_MYSQL57_PORT=<port> scripts/smoke_mysql57_schema_migrate.sh`。
- 复用已启动同项目容器：再加 `MOCHAT_REUSE_MIGRATION_STACK=1`。
- 固定测试 schema：`MOCHAT_MIGRATION_SMOKE_SCHEMA`，默认 `mochat_migration_smoke`。
- 成功输出一行：`migration inventory smoke passed: count=... first=... latest=... checksum=... kind=... schema=...`，可供后续 integration gate 使用同一保留数据库；workflow 不得另走只到 0098/0108 的伪完整 apply。

## 安全边界

- 未修改 `docs/PROJECT_PROGRESS.zh-CN.md`，未 reset/clean，未删除 Docker 命名卷。
- MySQL 5.7 实跑只在另一任务明确授权的无命名卷容器中新建 `mochat_task7_smoke`，未触碰其 `mochat_task8`。
- 未调用真实企业微信、真实 AI Provider 或生产服务器。
