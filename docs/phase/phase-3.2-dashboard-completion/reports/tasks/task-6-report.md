# Task 6 完成报告：线索池功能对标

## 结果

- 完成 `/customer/clue/default` 的真实前后端闭环：组合筛选、创建、重复检测、单条/批量分配、确认、转化、废弃。
- 状态流转固定为 `new → qualified → converted`，`new`/`qualified` 可进入 `discarded`；终态及非法流转返回校验错误。
- 批量分配逐目标返回 `id`、`status`、`errorCode`，允许部分失败；版本竞争返回 `409`，校验错误返回 `422`。
- 创建以企业范围内业务键幂等，联系电话重复由数据库唯一约束检测；转化在同一事务内写入联系人和负责人分配。
- 后端按认证用户解析 tenant，校验 corp 归属并执行 RBAC；所有 repository 查询和 mutation 同时限定 tenant/corp。
- 单条/批量分配在事务内验证负责人是当前 tenant+corp 的有效员工；越界、停用或删除员工逐目标返回 `OWNER_OUT_OF_SCOPE`，且不修改线索。
- 前端使用 Phase3.2 的 header、filter、stat、card、table/action 和 PageState 体系，补齐筛选、新增、批量操作、状态反馈和冲突提示；筛选和游标分页同步 URL，支持刷新、浏览器前进/后退与重置。

## TDD 与验证

- RED 证据：Domain 缺状态机、Application 缺组合合同、HTTP 缺 403/批量/409、MariaDB 缺 `corp_id`、前端 API/page 缺对应方法与控件时，目标测试均先失败。
- `go test ./internal/modules/scrm/... ./internal/app/bootstrap ./cmd/mochat-go`：通过。
- `go test -tags integration -count=1 ./internal/modules/scrm/adapters/mysql`（真实本地 MariaDB）：通过，覆盖 tenant/corp 隔离、幂等、重复、并发版本冲突、部分失败和转化持久化。
- 审查修复聚焦 MariaDB：负责人 tenant/corp/有效状态校验、逐项失败不写入及并发/部分失败，通过。
- 线索 API/page Vitest：原 Task6 目标 2 files，9 tests，通过；审查修复后 LeadPage 聚焦 1 file，7 tests，通过，新增 URL 刷新、前进/后退、分页与重置覆盖。
- 控制器前端全量：55 files / 338 tests，通过（按控制器结果记录，本轮按要求不重复执行）。
- Dashboard typecheck、build：通过。
- `go test ./internal/migration`：通过；0106 up/down 由迁移目录自动注册。
- 真实 MariaDB 独立 schema 迁移测试：历史 fixture 在唯一有效企业时正确回填；无唯一映射时在 DDL 前明确失败；down 成功回滚及跨企业业务键冲突在任何结构变更前失败，均通过。
- 审查修复后 `go vet ./internal/modules/scrm/...`：仅因既有、非 Task6 的 `internal/modules/scrm/transport/http/opportunity_handler.go:102`（`q.CorpID` self-assignment）返回失败；本提交未修改 Task4/5 或商机文件。

## 迁移与环境说明

- `0106_scrm_lead_parity.up.sql` 仅在历史 lead 的 tenant 可唯一映射到一个有效 corp 时回填，零个或多个有效企业会明确 `SIGNAL` 失败，不再写入 `corp_id=0` 后静默隐藏。
- `0106_scrm_lead_parity.down.sql` 在 DDL 前检查跨 corp 的 `(tenant_id,business_key)` 冲突；存在不可逆冲突时明确 `SIGNAL`，避免中途半回滚。SCRM 线索表来自 0098 增量迁移，因此未把该表重复写入 standalone base schema。
- 仓库发布证据（2026-08-01 复核）：执行 `git branch -r --contains 34d6489` 与 `git tag --contains 34d6489` 均无输出，说明包含旧版 0106 的 `34d6489` 不在任何远端分支或标签中；当前 Phase3.2 分支仅在本地、尚未集成。
- 部署前置条件：Phase3.2 首次部署必须以 `f63f55e` 中的修正版 0106 为准，禁止单独部署 `34d6489`。若存在绕过仓库发布流程、由外部环境手工执行旧版 0106 SQL 的情况（不受上述分支/标签证据覆盖），必须先进行人工迁移协调和数据状态核验，不得直接继续自动迁移。
- 按用户要求未提交本机 `docker-compose` 修改；部署环境必须在启动新代码前执行迁移 runner。
- 本地数据库的 0001 历史 checksum 与当前仓库不一致，导致 runner 无法自动 apply；本轮仅为隔离集成测试手工应用 0106。此环境问题不属于代码失败。

## 延后项

- 浏览器真实登录态证据按计划延后 Task11；本任务未伪造 evidence。
- Dashboard/Go 全量回归留控制器执行；本报告只声明上述新鲜聚焦验证结果。
