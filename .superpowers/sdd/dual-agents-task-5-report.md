# Task 5 收尾报告：员工姓名选项与客户名称服务端查询

## 实现范围

- 新增 session/smart 两页共用的只读 `filter-options` action，继续使用页面现有 `#read` 权限和 Workspace principal 的 tenant、corp、员工 scope。
- 新增 `EmployeeOptionFilter`、`EmployeeOption` 与 Repository 查询接口。选项仅来自指定 analysis type 的洞察快照，按员工 ID 去重，使用 `mc_work_employee` 补充当前姓名和头像，并按姓名、ID 稳定排序。
- `InsightFilter` 新增 `CustomerName`；`customerName` 仅匹配 `target_type='1'` 的客户名称，与 employeeId、targetId、keyword 及权限 scope 组合为 AND。
- keyword 搜索摘要、目标名称和员工名称。keyword、customerName、employeeKeyword 统一通过参数化 LIKE 与反斜杠转义，`%`、`_`、`\` 按字面匹配。
- `InsightPage` 改为复用同一组 where/args 先执行 COUNT，再按稳定倒序执行带 `LIMIT/OFFSET` 的列表 SQL，不再全量加载后用 Go 过滤和分页。
- restricted 且 allowed employee IDs 为空时，员工选项直接返回空数组，不会放宽为全企业查询。Repository 错误继续由 Workspace 返回通用错误，不暴露 SQL 或数据库信息。

## TDD 红绿证据

接手时保留了工作树中已有的 `repository_test.go`、`workspace_handler_test.go` 和未跟踪 `transport/http/routes_test.go` 红灯测试。

- RED：`go test ./internal/modules/ai-insight/... -count=1` 因 `CustomerName`、`EmployeeOptionFilter`、`EmployeeOption` 与 Repository 方法尚不存在而编译失败；路由测试明确报告两条 `filter-options` GET 路由未注册。
- GREEN：聚焦测试覆盖 escaped LIKE、customerName direct-only、keyword/customerName/employeeId/scope 组合、COUNT/SELECT where 与 args 一致、分页 total、EmployeeOptions tenant/corp/analysis/scope/keyword/limit、restricted 空 scope、双页面 handler 映射、read 权限、非法 limit、错误脱敏及双路由注册，全部通过。
- 完整 `go test ./internal/modules/ai-insight/... -count=1`：通过。

## 兼容性与关注点

- 旧 employeeId、targetId、keyword 参数仍保留；Detail、Status、Runner、Parser、AI settings、migration、Docker 和前端未修改。
- 新 action 没有新增权限名，沿用 session/smart 页面现有 read gate，因此未修改 RBAC catalog。
- `.superpowers/sdd/progress.md` 是接手前已有的未提交修改，本提交明确排除。

## 最终验证

- `go test ./internal/modules/ai-insight/... -count=1`：通过（包含 `ai-insight/transport/http` 路由测试）。
- `go test ./internal/app/bootstrap/... -count=1`：通过。
- `git diff --check`：退出码 0；仅 Git 的 LF/CRLF 提示，无 whitespace 错误。
