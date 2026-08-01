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
