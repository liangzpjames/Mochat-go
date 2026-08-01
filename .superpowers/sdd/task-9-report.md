# Phase 3.2 Task 9 实施报告

## 状态

Task9 已从基线 `3513c63` 收敛完成。浏览器真实登录态验收延至 Task11；Dashboard/Go 全量回归、build 与 go vet 按控制器要求留给控制器执行。

## 完成内容

- `/customer/public-sea` 支持关键字、来源、业务类型、标签、地区、回收原因、历史负责人组合筛选，筛选条件与游标可由 URL 恢复。
- 公海列表展示来源、业务类型、标签、地区、回收次数、历史负责人、回收原因与最近跟进；进入、退回、回收操作均记录原因。
- 单次领取命令完整包含 `contactId`、`userId`、`version`、`idempotencyKey`；批量领取逐项返回 `id/status/errorCode`，允许部分成功。
- 服务端执行 tenant/corp、RBAC、owner 在职归属及领取用户与当前主体一致性校验。
- MariaDB repository 使用原子条件更新；同一联系人并发领取仅一个成功，其余返回 409。领取与公海变更历史持久化，并使用请求指纹保证幂等键同请求重放、异请求冲突。
- 新增 `0108_scrm_public_pool_parity` 迁移，补充公海筛选字段、索引与 assignment history 表。
- 页面沿用统一 `PageState` 和 Phase3.2 控件、表格、操作区样式。

## TDD 证据

- RED：新增前端公海场景后 5 项失败，暴露 URL 组合筛选、进入/退回/回收、完整领取字段和批量逐项结果未实现。
- RED：新增 HTTP/API 测试后因批量端点及完整命令体缺失失败；新增 Go 合同后因 filter、command、batch 类型缺失而编译失败。
- GREEN：补齐领域合同、应用服务、HTTP、MariaDB repository、迁移与前端交互后，以下聚焦验证全部通过。

## 最终验证

- `go test ./internal/modules/scrm/application ./internal/modules/scrm/transport/http ./internal/migration`：通过。
- `go test -tags=integration ./internal/modules/scrm/adapters/mysql -run '^TestPublicPoolMariaDBFiltersHistoryIsolationAndAtomicClaims$' -count=1`：通过，真实 MariaDB，未跳过。
- `pnpm --filter @mochat/dashboard exec vitest run src/features/scrm/public-pool-page.test.tsx src/features/scrm/scrm-api.test.ts src/features/scrm/contact-api.test.ts src/features/scrm/contact-page.test.tsx`：4 files / 15 tests 通过。
- `pnpm --filter @mochat/dashboard typecheck`：通过。
- `git diff --check`：通过。

## 风险与后续

- 部署前必须执行 `0108_scrm_public_pool_parity` 迁移；聚焦 MariaDB 测试会自检并补齐测试库结构，但不替代部署迁移。
- 按最新收敛指令未运行 Dashboard/Go 全量回归、build 与 go vet，由控制器统一执行。
- 浏览器端组合筛选、单领、批量部分失败和刷新后历史展示证据留 Task11，在本任务中不伪造登录态验收结果。
