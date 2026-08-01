# Task 3：数据概览功能对标报告

状态：DONE_WITH_CONCERNS

## 已完成

- 以统一 `DashboardOverviewQuery` 串行化 `startDate`、`endDate`、`employeeIds`、`departmentIds`、`period`、`page`、`pageSize`；加载和 CSV 导出复用同一对象。
- 页面使用 URL 查询状态恢复日期、员工、部门、周期和分页；提供日/周/月、趋势图、表格、分页、空态、403、错误重试与导出错误反馈。
- 后端严格校验完整查询、日期边界、ID、周期和分页；从认证企业上下文取企业，并接入 RBAC 解析器，将受限范围与客户端员工筛选相交；趋势支持日/周/月归并及分页元数据。
- 已更新功能矩阵 `/index` 行的真实实现、权限、持久化和自动化测试路径。

## 验证

- `go test ./internal/dashboard -run 'CorpData' -count=1`：通过。
- `pnpm --filter @mochat/dashboard exec vitest run --pool=forks --poolOptions.forks.singleFork=true src/features/dashboard-overview/dashboard-overview-api.test.ts src/features/dashboard-overview/dashboard-overview-page.test.tsx`：14/14 通过。
- `pnpm --filter @mochat/dashboard typecheck`：通过。
- `pnpm --filter @mochat/dashboard build`：通过。
- `go test ./internal/server -run 'CorpData' -count=1`：通过。

## 关注点

- 浏览器验收尚未执行：当前环境没有可调用的浏览器控制会话和本地登录凭据。没有伪造截图或闭合矩阵 evidence；`/index` 保持等待 Task 11 的浏览器证据。
- 任务执行期间一次并行聚合命令未返回完整输出；随后已改用直接串行 Vitest 命令并记录为通过。

## 安全与质量审查修复追加（2026-08-01）

状态：DONE_WITH_CONCERNS

### 已修复

- 新增 `CorpDataScope`，把认证用户的 `tenantId`、已选 `corpId`、请求员工与 RBAC 员工范围的最终交集、请求部门范围及“员工范围是否受限”同时下推到汇总与趋势 Store。汇总、趋势以及复用同一查询对象的 CSV 导出现在使用同一受限范围。
- MySQL 汇总和趋势不再仅查询无员工维度的 `mc_corp_day_data`。联系人指标按 `mc_work_contact_employee.employee_id`，群指标按 `mc_work_room.owner_id` 实时聚合；员工、部门筛选通过 `mc_work_employee`、`mc_work_employee_department`、`mc_work_department` 生效。
- 每个 CorpData 业务 SQL 分支均联结 `mc_corp` 并同时校验 `tenant_id`、`corp_id`、软删除状态；部门子查询额外校验 `department.corp_id`，防止跨租户、跨企业数据泄漏。
- 受限员工范围为空时，SQL 显式追加 `1 = 0`，不再把空切片解释为无筛选。非受限用户请求了员工筛选时仍会应用请求范围。
- 复用部署合同中的 `MOCHAT_TIMEZONE`：配置默认并显式校验 `Asia/Shanghai`，Handler 使用注入的 IANA location 解析自然日；SQL 使用 Unix 边界和显式目标时区偏移聚合日期。前端默认日期也固定使用 `Asia/Shanghai`，不依赖浏览器本地时区。
- 分页增加 `page <= 1_000_000` 上限，并在计算 `(page-1)*pageSize` 前保留整数溢出防御；超大页码返回 400，不会 panic。
- 新增真实 `RBACResolver` 链路测试，覆盖请求员工交集、空授权范围、部门下推、tenant/corp 下推、跨企业拒绝、月聚合、时区午夜边界和极大页码；Store 测试覆盖受限空范围与完整 SQL 条件。
- 移除数据概览前端测试的 `as never`，保持静态类型契约。

### TDD 证据

- RED：新增 Go 契约测试先因缺少 `CorpDataScope`、完整 Store 接口、时区注入和 SQL builder 而失败；新增前端时区测试先失败。
- GREEN：实现最小 Scope/SQL/时区/分页修复后，聚焦 Go 与 Vitest 通过；随后清理死代码并保持测试通过。

### 验证

- `go test ./internal/dashboard -run 'CorpData' -count=1`：通过。
- `go test ./internal/store -run 'CorpData' -count=1`：通过（无 DSN 时集成用例按合同跳过）。
- 隔离 MariaDB：`go test ./internal/store -run '^TestIntegrationCorpDataScopedQueriesExecute$' -count=1`：通过，真实执行了带 tenant/corp、员工、部门条件的汇总与趋势 SQL。
- `go test ./internal/config -run 'Timezone|FromEnvDefaults' -count=1`：通过。
- `go test ./internal/server -run 'CorpData' -count=1`：通过；`go test ./internal/server -count=1`：通过。
- Dashboard 聚焦 Vitest：15/15 通过。
- Dashboard 全量串行 Vitest（single fork，300 秒上限）：Task 3 相关 15/15 通过；全量 327/328，通过文件 54/55。唯一失败来自非 Task 3 测试缺少 DOM cleanup，按提交边界未修改该文件。
- `pnpm --filter @mochat/dashboard typecheck`：通过。
- `pnpm --filter @mochat/dashboard build`：通过。
- `go vet ./internal/dashboard ./internal/store ./internal/config ./internal/server ./cmd/mochat-go`：通过。
- `git diff --check`：通过，仅输出仓库行尾转换提示。

### 关注点

- 未执行浏览器验收，也没有伪造浏览器证据；仍由 Task 11 使用真实登录态完成。
- Dashboard 全量 single-fork Vitest 的唯一失败是 `src/pages/access-pages.test.tsx` 读取到前序非 Task 3 测试遗留 DOM；曾用 cleanup 验证 328/328 可通过，但按任务边界已撤销并排除相关测试文件修改。
- 额外运行 `go test ./internal/dashboard -count=1` 时发现既有 `TestSaaSAdminCustomerSuccessRenewalTasksCreatesTasksFromQueue` 对续费时间硬编码为午夜，但生产结果保留当前时分秒；该失败与 CorpData 变更无关，CorpData 聚焦测试通过，本次未扩大范围修改 SaaS Admin 合同。
- Windows 环境没有 WSL `/bin/bash`，现有 Bash 集成脚本无法直接执行；已用等价 PowerShell 启动并清理命名隔离 MariaDB 栈，真实 SQL 集成测试通过。

## 第二轮阻断修复追加（2026-08-01）

状态：DONE_WITH_CONCERNS

### 已修复

- `RBACResolver` 继续在 `AccessContext.PermissionKey` 中保留带 HTTP method 的审计键，但在查询 `mc_rbac_menu.link_url` 前统一移除 `#get/#post` 后缀；`/dashboard/corpData/index#get` 因此按真实种子菜单 `/dashboard/corpData/index` 授权。新增 resolver 测试让 fake 只接受真实种子键，避免再次因忽略入参而误绿。
- 时区合同明确收敛为仅支持 `Asia/Shanghai`：`MOCHAT_TIMEZONE` 即使是合法 IANA 值，只要不是 `Asia/Shanghai` 也会被配置层拒绝；前端既有默认日期计算继续固定使用同一时区。趋势 SQL 不再从范围起点推导 offset，而是使用产品固定 `+08:00`；`America/New_York` 用例验证 Store 不会带入 DST offset。
- `CorpDataSummary` 从约 25 条串行标量 SQL 合并为 4 条资源域条件聚合 SQL：联系人、群、群成员、员工各 1 条；`CorpDataLineChat` 保持 1 条趋势 SQL。因此 `/dashboard/corpData/index` 每次请求固定执行 5 条业务查询。`TestCorpDataSummaryQueryPlanUsesAtMostFourScopedResourceQueries` 同时断言 summary 恰好 4 条、整个 index 不超过 5 条，并逐条检查 tenant/corp、员工、部门和受限最新时间条件。
- `updatedAt` 不再读取企业级、无员工维度的 `mc_work_update_time`；4 条资源域聚合在最终员工/部门 scope 内计算各自 `MAX`，应用层取受限结果集合的最新时间，并显式转换为 `+08:00`。
- MariaDB 集成测试会创建带唯一 `task3_corp_data_*` 名称的两个真实 tenant/corp、三名员工、三个部门、重复员工部门关系、联系人关系、群、群成员、日统计噪声和企业级未来更新时间，并在 `t.Cleanup` 中按依赖逆序删除所有命名数据。测试断言跨租户隔离、RBAC 空集零数据、员工筛选、部门筛选、重复关系不重复计数、月聚合、`Asia/Shanghai` 午夜边界和受限 `updatedAt`；不再只验证 SQL 可执行。
- 原有 page 安全、tenant/corp 校验、请求员工与 RBAC 员工范围交集、部门 scope、分页和 CSV 同范围逻辑保持不变。

### TDD 证据

- RED：真实菜单 resolver 测试先返回 `permission denied`，证明 `#get` 与种子 `link_url` 不一致；合法 DST 时区配置测试先得到 `err=nil`。
- RED：查询计划测试先因缺少 `corpDataSummaryQuerySpecs` 无法编译；真实 MariaDB 首次运行的指标断言正确，但 `updatedAt` 返回数据库 session 时间 `2026-08-02 01:00:00`，而产品时区期望 `2026-08-02 09:00:00`。
- RED：DST Store 合同测试先观察到趋势参数 `-05:00`；实现固定产品时区后改为 `+08:00`。
- GREEN：实现菜单键规范化、配置拒绝、4 条 summary 条件聚合、受限最新时间与固定趋势时区后，聚焦单测和真实 MariaDB 结果断言均通过。

### 查询数代码证据

- `internal/store/mysql.go` 的 `corpDataSummaryQuerySpecs` 只返回 `corpDataSummaryContactsQuery`、`corpDataSummaryRoomsQuery`、`corpDataSummaryRoomMembersQuery`、`corpDataSummaryEmployeesQuery` 四项；`CorpDataSummary` 仅遍历该四项。
- 同文件 `CorpDataLineChat` 仅调用一次 `corpDataTrendQuery` 并执行一次 `QueryContext`。Handler 的 `Index` 顺序调用一次 summary 和一次 trend，因此总数为 `4 + 1 = 5`，不再存在旧的四个时间窗口乘五项指标的串行循环。
- `internal/store/corp_data_test.go` 的 `TestCorpDataSummaryQueryPlanUsesAtMostFourScopedResourceQueries` 固定断言 summary 查询数为 4、index 查询数上限为 5。

### Fresh 验证

- `go test ./internal/dashboard -run 'CorpData|RBACResolver|PermissionKey' -count=1`：通过。
- `go test ./internal/store -run 'CorpData' -count=1`：通过；查询数上限测试通过。
- `go test ./internal/config -run 'Timezone|FromEnvDefaults' -count=1`：通过。
- `node scripts/check_phase3_2_mysql_integration.mjs`：通过；隔离 MariaDB 中 `internal/modules/scrm/adapters/mysql` 与 `internal/store` 均通过，容器和数据卷已清理。
- `go test ./internal/server -count=1`：通过。
- Dashboard Task 3 聚焦 Vitest：2 个文件、15/15 通过。
- `pnpm --filter @mochat/dashboard typecheck`：通过。
- `pnpm --filter @mochat/dashboard build`：通过。
- `go vet ./internal/dashboard ./internal/store ./internal/config ./internal/server ./cmd/mochat-go`：通过。
- `git diff --check`：通过，仅有工作区既有 CRLF 转换提示。
- 本轮按要求未重跑已知受污染的 Dashboard 全量串行测试；保留上一轮 Task 3 相关 15/15、全量 327/328 的控制证据。

### 关注点

- 本轮未执行浏览器验收，也未新增或伪造浏览器证据；真实浏览器矩阵仍由 Task 11 完成。
- Dashboard 全量串行测试的既有非 Task 3 DOM cleanup 污染未纳入本提交。

## 第三轮性能阻断收尾（2026-08-01）

状态：DONE

### 已修复

- 审计确认原最新迁移为 `0104_scrm_opportunity_owner`，新增不冲突的 `0105_corp_data_realtime_indexes` up/down；`0103_phase3_2_query_indexes` 保持原文和原语义不变。`0105` 为 `mc_work_contact_employee`、`mc_work_room`、`mc_work_contact_room`、`mc_work_employee`、`mc_work_employee_department` 增加 7 个企业、状态、软删除、日期和关联键复合索引。
- `deploy/standalone/schema/mochat.sql` 已同步相同索引。up migration 通过 `information_schema.statistics` 缺失检查兼容已有 schema 和旧库升级，down 按逆依赖顺序删除索引；migration 合同测试同时校验最新编号、0103 不变、up/down 对称和 schema 索引一致。
- 字段审计确认 `mc_work_contact_room.out_time` 是 `varchar(50)`，写路径使用 `YYYY-MM-DD HH:MM:SS`；同一退群更新原子写入 `status=2`、`out_time=DATE_FORMAT(NOW(), ...)` 和 `updated_at=NOW()`，且后续同步只加载 `status <> 2` 的成员，不会反复改写退群行。因此退群 summary/trend 改用现有可索引 `updated_at timestamp`，删除 `STR_TO_DATE(contact_room.out_time, ...)` 对事实列的函数包裹，并保留 `status=2`、非空 `out_time` 业务条件。
- `MySQLStore` 提取仅含 `QueryRowContext`/`QueryContext` 的最小 `corpDataQueryExecutor`；生产构造函数仍把真实 `*sql.DB` 注入执行器。真实 MariaDB fixture 用 counting wrapper 包裹同一个 `*sql.DB`，直接调用生产 `CorpDataSummary` 和 `CorpDataLineChat`：summary 实测 `QueryRowContext=4, QueryContext=0`，trend 实测 `QueryRowContext=0, QueryContext=1`，不存在隐藏查询。
- 真实 fixture 增加隔离的第三企业计划噪声并执行 `ANALYZE TABLE`，避免微型表导致优化器合理选择 `ALL`；噪声不改变既有 tenant/corp、RBAC、部门、时区和结果断言。

### TDD 证据

- RED：migration 合同先报告最新版本仍为 `0104` 且 `0105` 文件不存在；退群查询合同捕获 summary/trend 仍含 `STR_TO_DATE(contact_room.out_time, ...)`。
- RED：fresh schema 与首次 `0105` 同时包含索引时，真实 MariaDB 返回 `ERROR 1061 Duplicate key name`；增加缺失检查后同一 up migration 可安全执行。
- RED：真实 EXPLAIN 首先在仅 6 行的事实表上选择 `ALL`；加入跨企业计划噪声并刷新统计信息后，生产 SQL 使用 `ref/range/const`，目标事实表不再出现 `ALL`。
- RED：production-path 预算测试先因 `MySQLStore` 没有可包装的 executor 而编译失败；加入最小 executor 后，真实调用计数断言转绿。

### MariaDB EXPLAIN 与查询预算证据

- summary contacts：`type=ref`，`key=idx_mc_wce_corp_status_deleted_employee`。
- summary rooms：`type=ref`，`key=idx_mc_wr_corp_deleted_created_owner`。
- summary room members：`type=ref`，`key=idx_mc_wcr_room_status_deleted_join`。
- summary employees：受限员工主键定位，`type=const`，`key=PRIMARY`。
- trend contacts：新增路径 `type=range` / `idx_mc_wce_corp_deleted_create_employee`，流失路径 `type=range` / `idx_mc_wce_corp_status_deleted_employee`。
- trend room members：入群和退群路径均为 `type=ref` / `idx_mc_wcr_room_status_deleted_join`，并继续使用可索引的 `join_time`/`updated_at` 半开区间；目标事实表无 `ALL`。
- production-path 实测预算：`CorpDataSummary` 4 次 `QueryRowContext`、0 次 `QueryContext`；`CorpDataLineChat` 0 次 `QueryRowContext`、1 次 `QueryContext`。

### 提交范围

- 仅纳入 `0105` up/down、standalone schema、migration 合同测试、CorpData Store 实现/测试和本报告。
- 明确排除并恢复 `deploy/standalone/docker-compose.yml`、`internal/dashboard/saas_admin_system_health.go`、integration script 的临时环境改动；不纳入 acceptance、计划、前端测试或浏览器证据。

### Fresh 验证

- `go test ./internal/migration -count=1`：通过。
- `go test ./internal/store -run 'CorpData' -count=1`：通过。
- `go test ./internal/dashboard -run 'CorpData|RBACResolver|PermissionKey' -count=1`：通过。
- `go test ./internal/server -count=1`：通过。
- `go vet ./internal/migration ./internal/store ./internal/dashboard ./internal/server`：通过。
- 隔离 MariaDB 10.6：真实执行 `0105 down → up`，索引数 `7 → 0 → 7`；随后 production-path 查询预算、结果断言和 EXPLAIN 全部通过。
- `git diff --check`：通过，仅有工作区既有行尾转换提示。
