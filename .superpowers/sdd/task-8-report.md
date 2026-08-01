# Phase 3.2 Task 8 实施报告

## 状态

Task8 商机功能已按 `f25e4b0` 基线收敛；浏览器真实登录态证据延后 Task11，全量测试留控制器执行。

## 完成内容

- `/customer/opportunity` 支持阶段、状态、负责人组合筛选，筛选与游标同步 URL，可刷新、前进后退和继续分页。
- 页面支持创建商机、推进自定义阶段、赢单、输单及必填输单原因，并可从商机行追加联系人跟进；`won`/`lost` 终态不再显示阶段操作。
- `ChangeOpportunityStageCommand` 使用完整 `opportunityId`、`stageId`、`version`、`lostReason`、`idempotencyKey` 合同。
- 服务端校验金额、日期、联系人、负责人和自定义阶段的 tenant/corp 归属；列表、创建、阶段与跟进均执行认证企业及 RBAC。
- 阶段变化使用事务乐观锁；非法终态变化返回 422，版本冲突返回 409，越权与资源不存在分别返回 403/404。
- 创建、阶段和跟进复用 Task7 持久化请求指纹幂等；同键同请求重放、同键异请求冲突。前端为每次新的创建/跟进动作生成新键，失败重试期间保持同键，避免误吞以后合法的相同内容。
- 跟进保持只追加；跟进接口允许“联系人编辑”或“商机编辑”任一权限，保持联系人页与商机页入口兼容。
- 公共 `PageState` 无最终修改；商机阶段的 422 仅在商机页本地映射为 `invalid-transition`，不会改变其他页面的 422 展示。

## TDD 证据

- RED：Go 测试最初因 `OpportunityPage`、`StageID`、`LostReason`、分页筛选和非法阶段错误合同不存在而编译失败。
- RED：目标 Vitest 8 项中 7 项因 URL 恢复、创建、输单、跟进和共享错误状态尚未实现而失败；API 测试明确指出缺少 `status` 与完整阶段字段。
- GREEN：实现最小合同后，聚焦 Go、真实 MariaDB、目标 Vitest 和 typecheck 全部通过。
- 回归 RED/GREEN：新增“成功后的相同创建/跟进必须使用新幂等键”和“商机编辑权限可追加跟进”测试，先分别复现错误重放与 403，再修复转绿。

## 最终验证

- `go test ./internal/modules/scrm/application ./internal/modules/scrm/domain ./internal/modules/scrm/transport/http -run 'Opportunity|FollowUp' -count=1`：通过。
- 设置本地隔离 MariaDB DSN 与 `MOCHAT_REQUIRE_MYSQL_INTEGRATION=1` 后，运行 `go test -tags=integration ./internal/modules/scrm/adapters/mysql -run 'TestOpportunityRepositoryListUsesPersistedStageID|TestOpportunityRepositoryCombinedFilterAndCursorPagination|TestOpportunityAndTagCommandsPersistFingerprintsAndRejectOrphans' -count=1`：通过，未跳过。
- `pnpm --filter @mochat/dashboard exec vitest run src/features/scrm/scrm-api.test.ts src/features/scrm/opportunity-page.test.tsx`：2 files / 11 tests 通过。
- `pnpm --filter @mochat/dashboard typecheck`：通过。
- `go vet ./internal/modules/scrm/...`：此前本任务实现阶段通过；最终收敛未扩大 Go 生产范围。
- `git diff --check`：提交前通过。

## 风险与后续

- 按控制器要求未运行 Dashboard/Go 全量回归；由控制器统一执行。
- 浏览器创建、推进、跟进、赢单/输单和刷新证据延后 Task11，不伪造登录态证据。
- 商机阶段目前使用既有阶段 ID 输入；后续若增加阶段配置下拉，应复用企业阶段数据源，不放宽服务端归属校验。

## P1 HTTP 合同修复

- 商机阶段 handler 改用独立 HTTP DTO，明确接收 `corpId`、`opportunityId`、`stageId`、`version`、`lostReason`、`idempotencyKey`；资源 ID 以 path 为主，body 重复提供时必须一致，否则返回 422。
- 阶段和跟进幂等键兼容 `Idempotency-Key` header 或 body `idempotencyKey`；两处同时提供时必须一致，缺失或冲突返回 422。
- 跟进 DTO 不接收 `contactId`，联系人 ID 只从 path 读取；两个前端 API 均只发送 `corpId/content`，避免重复资源字段。
- 商机 handler 的严格 JSON 解码失败统一返回明确 JSON 400，unknown field、尾随 JSON、畸形 JSON 不再产生空 200。
- 新增 handler 级真实前端 payload 测试，覆盖普通阶段推进、won、lost/lostReason、follow-up、path/body 冲突、幂等键缺失/冲突及 unknown field。

### P1 聚焦验证

- `go test ./internal/modules/scrm/application ./internal/modules/scrm/domain ./internal/modules/scrm/transport/http -run "Opportunity|FollowUp" -count=1`：通过。
- `pnpm --filter @mochat/dashboard exec vitest run src/features/scrm/scrm-api.test.ts src/features/scrm/contact-api.test.ts`：2 files / 6 tests 通过。
- `pnpm --filter @mochat/dashboard typecheck`：通过。
- 未重复运行真实 MariaDB 集成测试；P1 未修改应用服务、repository、迁移或持久化合同，沿用 Task8 已通过的 MariaDB 证据。
