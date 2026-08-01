# Task 6 完成报告：线索池功能对标

## 结果

- 完成 `/customer/clue/default` 的真实前后端闭环：组合筛选、创建、重复检测、单条/批量分配、确认、转化、废弃。
- 状态流转固定为 `new → qualified → converted`，`new`/`qualified` 可进入 `discarded`；终态及非法流转返回校验错误。
- 批量分配逐目标返回 `id`、`status`、`errorCode`，允许部分失败；版本竞争返回 `409`，校验错误返回 `422`。
- 创建以企业范围内业务键幂等，联系电话重复由数据库唯一约束检测；转化在同一事务内写入联系人和负责人分配。
- 后端按认证用户解析 tenant，校验 corp 归属并执行 RBAC；所有 repository 查询和 mutation 同时限定 tenant/corp。
- 前端使用 Phase3.2 的 header、filter、stat、card、table/action 和 PageState 体系，补齐筛选、新增、批量操作、状态反馈和冲突提示。

## TDD 与验证

- RED 证据：Domain 缺状态机、Application 缺组合合同、HTTP 缺 403/批量/409、MariaDB 缺 `corp_id`、前端 API/page 缺对应方法与控件时，目标测试均先失败。
- `go test ./internal/modules/scrm/... ./internal/app/bootstrap ./cmd/mochat-go`：通过。
- `go test -tags integration -count=1 ./internal/modules/scrm/adapters/mysql`（真实本地 MariaDB）：通过，覆盖 tenant/corp 隔离、幂等、重复、并发版本冲突、部分失败和转化持久化。
- 线索 API/page Vitest：2 files，9 tests，通过。
- Dashboard typecheck、build：通过。
- `go test ./internal/migration`：通过；0106 up/down 由迁移目录自动注册。

## 迁移与环境说明

- 新增 `0106_scrm_lead_parity.up.sql` / `.down.sql`，可升级和回滚；SCRM 线索表来自 0098 增量迁移，因此未把该表重复写入 standalone base schema。
- 按用户要求未提交本机 `docker-compose` 修改；部署环境必须在启动新代码前执行迁移 runner。
- 本地数据库的 0001 历史 checksum 与当前仓库不一致，导致 runner 无法自动 apply；本轮仅为隔离集成测试手工应用 0106。此环境问题不属于代码失败。

## 延后项

- 浏览器真实登录态证据按计划延后 Task11；本任务未伪造 evidence。
- Dashboard/Go 全量回归留控制器执行；本报告只声明上述新鲜聚焦验证结果。
