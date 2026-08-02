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
- 审查修复聚焦 Go：`go test ./internal/modules/scrm/transport/http ./internal/modules/scrm/application -run 'TestOpportunityHandler|TestCustomerLifecycle' -count=1`
- 审查修复真实 MariaDB：`go test -tags=integration ./internal/modules/scrm/adapters/mysql -run 'TestOpportunityAndTagCommandsPersistFingerprintsAndRejectOrphans|TestContactLifecycleMariaDBIsolationCombinedFilterAndAggregateDetail' -count=1`
- 审查修复前端：`assignment-editor.test.tsx`、`follow-up-timeline.test.tsx`、`contact-page.test.tsx`，3 files / 14 tests；另通过 Dashboard typecheck、SCRM `go vet` 和 `git diff --check`。

## 风险与后续

- 未执行 Dashboard、Go 全量测试和完整 build；按用户要求由控制器统一复跑。
- 浏览器真实登录态的 URL 刷新恢复及“追加跟进 → 创建商机 → 进入公海”证据延后 Task11。
- Task8、Task9、Task10 仍会分别深化商机、公海和标签页面合同；本任务只实现联系人详情中的必要入口。

## 审查修复

- `OpportunityHandler` 的商机列表/阶段变更与标签列表/创建/改名均按目标页面动作执行 RBAC，并将认证 tenant 与获授权 corp 下推；同租户第二企业在进入服务前返回 403。
- `mochat_go_scrm_contacts` 当前没有可与 `mc_work_contact` 对应的 `contact_id`、`unionid` 或 `external_userid`。详情聚合已删除姓名匹配，返回空好友列表及 `wecomFriendsAvailable=false`；真实 MariaDB 同名联系人测试证明不会串联。
- 创建商机、推进阶段、追加跟进、创建/改名/绑定标签全部在事务中持久化请求指纹：同键同请求重放原结果，同键异请求返回 409。
- 所有相关写操作先验证当前 tenant+corp 内的联系人、标签、商机、企业和负责人；跨企业或不存在资源返回 404（负责人越界为 403），事务回滚且不产生孤儿记录。
- `assignment-editor` 与 `follow-up-timeline` 的 loading、403、404、409、通用错误和 retry 已统一使用 `PageState`，并新增失败与重试测试。
- 控制器基线记录：Dashboard 全量 `56 files / 342 tests`；按收敛要求，本次不重复执行全量，交由控制器复跑。
