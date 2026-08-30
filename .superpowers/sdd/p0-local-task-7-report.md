# Task 7：发布门禁与迁移治理实施报告

日期：2026-08-29

## 结论

九条红门禁均已对齐当前运行合同并在 Windows 本机逐项通过。独立审查指出的可骗绿路径已消除：Dashboard 鉴权由 Go typed route registry 驱动运行时和独立 audit；Phase 2 直接读取三端当前 manifest；归档门禁执行角色、production registrar 失败关闭和 shutdown 行为测试；Phase 3.4 直接启动 Node/ESLint 且单批命令预算为 6000；identity projection 仅把 not-found/inactive 映射为 revoked，数据库错误原样传播。

迁移 smoke 统一消费 `mochat-migrate -action inventory` 的 TSV，动态取得 count/first/latest/checksum/kind。真实 MySQL 5.7 隔离库已从 0001 推进到 0173，正式经过 0130/0131/0165 三个受控迁移，并完成 173 条 ledger/status、0173 rollback-reapply、checksum drift 拒绝和删除 ledger 后的受控 baseline 重建。迁移 SQL 与其不可变 checksum 均未修改。

## 根因与方案

1. Dashboard auth：旧 audit 从字符串和默认 unbound 白名单推断，`securityMFA` 可被恒真例外放行。现在 597 条结构化 method/path/handler/auth kind 作为运行时 guard 的策略源；audit 独立发现生产注册并核对 metadata、handler 和 auth，缺失 metadata、注释伪装及 handler/auth mutation 均失败。
2. Phase 2 progress：旧报告读取历史 Phase 1 CSV，不能感知当前路由增删。现在严格读取 dashboard/sidebar/operation 三个 `migration-routes.json`，拒绝未知 app/status、重复和错误归属，稳定排序生成 CSV；当前为 82/82 React。
3. Archive gate：旧脚本用 `body.includes` 检查可被死代码满足。现在执行 Go 行为测试，覆盖角色矩阵、fixture/production 互斥、`RegisterAll` 失败时零监听，以及 graceful shutdown 必须调用 registrar `Close`；mutation 测试证明行为测试不能被注释绕过。
4. Phase 3.4：Windows 旧实现通过 shell 启动 pnpm，且长命令会越过系统预算。现在用 `process.execPath`、`shell:false` 直接运行 ESLint JS，稳定分批且每批不超过 6000 字符；超长单文件显式失败，不漏文件。
5. Identity projection：`ResolveIdentity` 曾把任意 DB/scan 错误折叠为 principal unavailable，激活状态进而误报 revoked。现在仅 `sql.ErrNoRows` 和 inactive 映射为 unavailable/revoked，其余错误继续传播。
6. Migration discovery：两个 smoke 曾硬编码 0098，且一个路径删除命名卷。现在共享 inventory 生命周期覆盖真实首尾版本、完整 ledger、status、最新 rollback-reapply、baseline 和 drift；清理只 `compose down --remove-orphans`，不删除卷。
7. MySQL 5.7 conditional ALTER：0109 等迁移使用 5.7 不支持的 `ADD/DROP ... IF [NOT] EXISTS`。runner 只在命中条件 ALTER 时识别 server version；对 5.7 逐 clause 查询 `information_schema`，跳过已满足操作，并对剩余操作执行合法 5.7 等价语法。原始 SQL/checksum 保持不变。
8. 0139 前置约束：0129 将 `mc_corp.tenant_id` 放宽为 nullable，0131 受控 cutover 完成后未重新收紧；0139 的动态 `SIGNAL` guard 因而命中不兼容分支。修复放在 0131 正式受控 CLI：先确认无 NULL，再 `MODIFY ... NOT NULL`，最后以 `information_schema` 验证。没有绕过 0139 guard，也没有修改 0139 SQL。
9. 0152 fresh replay 漂移：当前 0001 schema 已包含 0152 的四列，但 0152 增量仍无条件 `ADD COLUMN`。runner 仅对 0152 的执行副本补 conditional metadata guard，使旧增量部署与当前 fresh schema 都可执行；immutable body/checksum 未变，其他版本不受影响。
10. Checksum drift：旧 `status` 会输出 `checksum_mismatch` 却返回 0，smoke 无法真正拒绝漂移。CLI 现在先保留完整 TSV 证据，再对 `checksum_mismatch`/`database_ahead` 失败关闭并返回非零。
11. Controlled baseline：旧 full baseline 遇 0130 即无条件停止。现在只有控制表中恰好一条 checksum 匹配且 completed/verified 的 0130、0131、0165 正式证据存在时才允许补 ledger；缺失、重复或 checksum 漂移均失败，不能把仅“表看起来存在”当成受控迁移完成。

## TDD 证据

### RED

- typed registry 初始不存在，Go 编译失败；audit 的注释、securityMFA、handler/auth mutation 用例失败。
- Phase 2 当前 manifest 改动不能影响旧 CSV 报告；未知/重复条目未被阻断。
- archive gate 的死代码字符串可骗绿，production registrar 行为用例初始编译失败。
- identity 数据库错误初始被映射成 revoked/nil。
- Phase 3.4 direct Node 与 7000 字符上限测试初始失败。
- 真实 MySQL 5.7 fresh apply 初始仅到 0108，0109 conditional ALTER 语法失败。
- 越过 0131 后，0139 因 `mc_corp.tenant_id` 仍 nullable 命中 schema guard；修复受控 cutover 约束后通过。
- 0152 在 current fresh schema 因重复列失败；执行态 metadata guard 修复后通过。
- checksum mutation 初始得到 `checksum_mismatch` 但进程 exit 0；新增 CLI 失败测试稳定复现。
- 删除完整 schema 的 migration ledger 后，旧 baseline 在 0130 controlled boundary 失败；新增控制证据测试要求恰好一条 verified/completed 记录。

### GREEN / PASS

- `node --test scripts/check_dashboard_auth_context.test.mjs scripts/check_phase2_frontend_progress.test.mjs scripts/check_wecom_archive_saas_activation.test.mjs scripts/check_phase34_lint.test.mjs scripts/check_migration_smoke_inventory.test.mjs`：20/20 PASS。
- `go test ./internal/migration ./cmd/mochat-migrate ./cmd/mochat-identity-migrate ./cmd/mochat-ai-insight-0165 ./internal/qualitygate -count=1`：PASS。
- 九条门禁逐项 PASS：
  - `pnpm check:phase34-lint`：301 文件分 8 批，无遗漏。
  - `pnpm check:phase3-5-dashboard`：9/9。
  - `pnpm check:phase3-final`：27/27，且 Phase 3.5 9/9。
  - `pnpm check:dashboard-auth-context`：597 routes、missing 0、violations 0。
  - `pnpm check:identity-single-corp`：PASS；public exact 11、identity-auth exact 8。
  - `pnpm check:wecom-archive-saas-activation`：2/2 mutation tests 与行为门禁 PASS。
  - `pnpm docs:check`：PASS。
  - `pnpm check:phase2-progress`：82/82 React。
  - `pnpm check:phase2.1`：82 routes、40 features。
- 真实 MySQL 5.7（仅本地隔离测试库）：
  - inventory：173 条；first=`0001_initial_schema`，latest=`0173_scrm_order_idempotency`，latest checksum=`d564d2ac2486f07673e1382dbf3c01e35c557417ee4329236f90076de9765c59`，kind=`automatic`。
  - 0130 backfill、credential encryption、0131 cutover、0165 backup/preflight/apply/verify 均使用正式受控 CLI；0165 为 applied/verified，删除/重复计数均为 0。
  - apply：0001→0173 完成；ledger count=173；status 173/173 applied。
  - rollback-reapply：0173 `rolled_back` 后 ledger=172，再次 apply 为 `applied_now` 并回到 173。
  - drift：临时修改 0173 执行副本后 status 输出 `checksum_mismatch`，exit=1，error_code=`MIGRATION_STATUS_FAILED`。
  - baseline：仅删除 `mochat_go_schema_migrations`，依赖三项受控迁移的 checksum/status 证据重建 173 条；再次 status 173/173 applied。

### SKIP / 边界

- SKIP：Windows 主机没有可用 `/bin/bash`，两个 shell entrypoint 本体未直接执行；Node 结构/反作弊合同 PASS，且共享 helper 中的每个数据库生命周期阶段已在同一 MySQL 5.7 库手工等价实跑通过。此项不能冒充 Linux shell entrypoint PASS。
- SKIP：没有调用真实企业微信、真实 AI Provider 或生产环境；本报告的 provider 边界仅为本地代码、行为测试和数据库证据。

## 可复用 smoke 接口

- MySQL 5.7：`KEEP_STACK=1 MOCHAT_STACK_PROJECT=<project> MOCHAT_MYSQL57_PORT=<port> scripts/smoke_mysql57_schema_migrate.sh`。
- 复用已启动同项目容器：再加 `MOCHAT_REUSE_MIGRATION_STACK=1`。
- 固定测试 schema：`MOCHAT_MIGRATION_SMOKE_SCHEMA`，默认 `mochat_migration_smoke`。
- 成功输出：`migration inventory smoke passed: count=... first=... latest=... checksum=... kind=... schema=...`。后续 integration gate 必须消费同一完整数据库生命周期，不能另走只到 0098/0108 的伪完整 apply。

## 安全边界

- 未修改 `docs/PROJECT_PROGRESS.zh-CN.md`，未 reset/clean，未删除 Docker 命名卷。
- MySQL 5.7 实跑只使用明确授权、无命名卷容器中的 `mochat_task7_smoke`；未触碰 `mochat_task8`，未停止或删除复用容器。
- checksum drift 只修改并删除本任务自己的临时迁移副本；仓库 immutable SQL 未变。
- 未调用真实企业微信、真实 AI Provider 或生产服务器。
