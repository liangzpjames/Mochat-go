# Task 7：九条门禁与 CI/迁移治理实施报告

日期：2026-08-29

## 结论

本任务指定的九条仓库门禁全部由 RED 转为 GREEN。修复遵循当前运行时合同，没有恢复旧路由、创建空文件或把 fixture 冒充生产实现。CI 已调整为先构建前端再执行依赖构建产物的 audit。迁移命令新增无数据库副作用的 `inventory` 输出，直接复用 `DefaultMigrations` 与运行时 checksum 实现；当前注册表实测为 173 条，首条 `0001_initial_schema`，最新 `0173_scrm_order_idempotency`。

MySQL 5.7 容器 smoke 本次没有执行，也尚未把两个历史 smoke 脚本的大段 `0098` 断言全部切换到 inventory；该项按 FAIL/未完成记录，不把 inventory 单测冒充数据库 smoke。

## 根因与方案

1. `phase34-lint`：根因是一次把约 300 个文件交给 Windows 子进程，命令行超过平台上限。现在同时按文件数和命令字符数稳定分批，逐批执行并累计失败，测试验证扁平化后文件集合完全相等，长文件名不会被静默丢弃。
2. `phase3-5-dashboard` / `phase3-final`：根因是 `/chat/file-audio` 在阶段合同里双重归属。唯一生产 manifest 现将其归为 `3-final`，Phase 3.5 严格保持九条路由；两个门禁共同消费同一 manifest。
3. `dashboard-auth-context`：根因是以 handler 源码中是否出现 `Authorization` 字符串猜鉴权，无法表达全局 guard 和公共激活状态。现输出显式 route→handler→auth metadata，并从 `dashboard_route_policy.go` 的精确 method+route 合同区分 public、identity-authenticated、SaaS principal 与默认 Dashboard principal；仍校验全局 guard 组合。
4. `identity-single-corp`：根因是激活展示直接 join `mc_user`，迫使门禁扩大例外。现激活查询只读 identity/activation projection，再通过 `ResolveIdentity` 唯一受控绑定取得 tenant，最后查询 tenant；同时移除本地模拟明文凭据 fallback，解密失败关闭。
5. `wecom-archive-saas-activation`：根因是门禁仍检查已删除的 flags 互斥函数。现对齐 runtime responsibilities、worker/scheduler 分工、production registrar 注册/关闭合同，并明确生产 bridge 不得引用本地 acceptance fixture。
6. `docs:check`：根因是 Phase 3 README 到 Phase 2.1 文档少退一级目录，修复真实相对链接。
7. `phase2-progress`：根因是报告依赖输入行顺序且旧 CSV 已漂移。现在从当前 inventory 稳定排序、严格状态归类后生成，结果为 134/134 React、0 legacy。
8. `phase2.1`：根因是合同仍要求 `/corp/index`、`/corpData/index` 和已不存在的组件。现统一到唯一企业资料路由 `/company-setting/website` 及实际 dedicated module，不回退 generic workbench。
9. CI / migration：根因是 quick audit 在 build 之前、workflow 名称和 smoke 断言把 `0098` 当最新。workflow 已 build-before-audit；`mochat-migrate -action inventory` 从真实发现注册表输出 version/checksum/kind/description，可供 smoke 动态消费。历史 smoke 脚本完全接线仍未完成。

## TDD 证据

### RED

- 九门禁初始分别复现：Windows 命令过长、Phase 3.5 多出 file-audio、final 被相同漂移阻断、auth 缺 34 路由且激活误判、identity 直接读 `mc_user`、archive 检查陈旧 flags、docs 断链、Phase 2 报告漂移、Phase 2.1 陈旧 corp 路由。
- CI 新合同测试初始 2/2 RED：build 位于 quick 之后、workflow 仍名为 `Migration 0098 lifecycle gate`。
- migration inventory 新 Go 测试初始因 `DefaultInventory` / `writeMigrationInventory` 不存在而编译 RED。

### GREEN / PASS

- 九条命令全部 PASS：
  - `pnpm check:phase34-lint`：301 文件，8 批（40×7 + 21），exit 0。
  - `pnpm check:phase3-5-dashboard`：9/9。
  - `pnpm check:phase3-final`：27/27，且 Phase 3.5 9/9。
  - `pnpm check:dashboard-auth-context`：594 routes、561 catalog contracts、missing 0、violations 0。
  - `pnpm check:identity-single-corp`：Dashboard principal 578、public 11、identity-auth 5，PASS。
  - `pnpm check:wecom-archive-saas-activation`：12/12 脚本测试及静态 acceptance contract PASS。
  - `pnpm docs:check`：PASS。
  - `pnpm check:phase2-progress`：134/134 React，PASS。
  - `pnpm check:phase2.1`：82 routes、40 features，PASS。
- 相关 Node 脚本单测：68/68 PASS。
- `go test ./internal/migration ./cmd/mochat-migrate -run 'TestDefaultInventory|TestWriteMigrationInventory' -count=1`：PASS。
- `go run ./cmd/mochat-migrate -project-root . -action inventory`：173 行；0001 到 0173，checksum 均由运行时实现生成。

### FAIL / SKIP

- FAIL（未完成）：`scripts/smoke_schema_migrate.sh` 与 `scripts/smoke_mysql57_schema_migrate.sh` 尚未完整改为消费 inventory，仍存在历史 `0098` 硬编码。
- SKIP：未启动 MySQL 5.7/MariaDB 容器，因此没有数据库 apply/baseline/rollback 的真实证据。
- SKIP：全仓 Go test/vet、前端全量 test/build、Docker/浏览器不属于本子任务的收束证据，由父任务统一执行。

## 安全边界

- 未修改 `docs/PROJECT_PROGRESS.zh-CN.md`。
- 未调用真实企业微信、真实 AI Provider 或生产服务器。
- 未执行 reset/clean，未删除或重建 Docker 命名卷。
- production archive registrar 与 local acceptance fixture 仍明确分离。
