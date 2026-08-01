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
