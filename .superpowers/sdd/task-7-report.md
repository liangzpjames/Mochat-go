# Phase 3.2 Task 7 实施报告

## 状态

联系人生命周期功能已按 Task7 范围收敛；浏览器证据按约定延后到 Task11，全量回归留控制器执行。

## 完成内容

- `/customer/contact` 改为真实 SCRM API，支持关键词、负责人、标签、分配状态、游标分页，并将筛选、分页和当前详情同步到 URL。
- 联系人详情统一聚合资料、负责人/协作人、标签、企微好友、商机摘要和按创建时间只读展示的跟进时间线。
- 页面提供负责人/协作人编辑、标签关联、追加跟进、进入公海和创建商机入口；加载、空数据、403、404、409 和重试使用统一 `PageState`。
- 新增联系人列表与详情路由；所有查询强制使用认证 tenant 与请求 corp，RBAC 在列表、详情和联系人生命周期动作前执行。
- MariaDB 查询按 tenant/corp 隔离；负责人和协作人必须属于当前 tenant/corp 的有效员工；分配操作保留 version 与持久化幂等键合同。
- 新增可逆 `0107_scrm_contact_lifecycle_idempotency` 迁移。

## TDD 证据

- RED：应用层联系人组合筛选与聚合详情测试最初因类型和方法不存在而编译失败。
- GREEN：新增类型化 repository/service 后，两项测试通过。
- RED：HTTP 联系人路由、RBAC、403/404 合同测试最初因路由和处理方法不存在而编译失败。
- GREEN：新增列表/详情处理器与鉴权后测试通过。
- RED：前端 URL 恢复、详情和生命周期交互测试最初因页面仍是简单两列表格而失败。
- GREEN：实现详情闭环后，4 个目标测试文件共 7 项测试通过。

## 已执行验证

- `go test ./internal/modules/scrm/... -run 'TestCustomerLifecycleService|TestContactHandler|TestRegisterRoutesInstallsAllSCRMRoutes' -count=1`
- `go test -tags integration ./internal/modules/scrm/adapters/mysql -run '^TestContactLifecycleMariaDBIsolationCombinedFilterAndAggregateDetail$' -count=1 -v`
- `pnpm exec vitest run src/features/scrm/contact-api.test.ts src/features/scrm/contact-page.test.tsx src/features/scrm/assignment-editor.test.tsx src/features/scrm/follow-up-timeline.test.tsx`
- `pnpm --filter @mochat/dashboard typecheck`
- `go test ./internal/migration -run 'TestStandaloneComposeFreshInitUsesSchemaForCorpDataIndexes|TestContactLifecycleIdempotencyMigrationIsReversible' -count=1`
- `git diff --check`

## 风险与后续

- 未执行 Dashboard、Go 全量测试和完整 build；按用户要求由控制器统一复跑。
- 浏览器真实登录态的 URL 刷新恢复及“追加跟进 → 创建商机 → 进入公海”证据延后 Task11。
- Task8、Task9、Task10 仍会分别深化商机、公海和标签页面合同；本任务只实现联系人详情中的必要入口。
