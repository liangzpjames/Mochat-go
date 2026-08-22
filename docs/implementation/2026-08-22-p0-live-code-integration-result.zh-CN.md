# P0 活码集成实施结果

日期：2026-08-22

分支：`feat/p0-live-code-integration-20260822`

集成基线：`origin/main@43c781514a4549e806517ab2b2a707f1320e2df7`

## 1. 结论

本分支已同步用户在其他会话完成的渠道活码、群活码权威实现，并停止继续修改两页产品实现。最终页面、组件测试、后端直连能力与 `0152_group_code_direct_join` 均采用 `origin/main@43c7815`；本分支只保留迁移兼容、Dashboard RBAC 合同和 benchmark 阶段修复，不存在第二套重复入口。

所有本任务可控代码门禁已通过。真实 MariaDB 专用集成套件因未设置 `MOCHAT_GO_MYSQL_INTEGRATION_DSN` 明确跳过；同时，保留命名卷的 Docker MariaDB 已实际完成 0152—0154 升级、读回和页面验收。

## 2. 最终权威实现

- 渠道活码与群活码页面、抽屉、组件测试及直连逻辑：以 `origin/main@43c7815` 为唯一权威来源；同步后未再改动这些页面。
- 迁移序列：`0150_repair_keyword_published_snapshots` → `0151_ai_conversation_insight_api_resources` → `0152_group_code_direct_join` → `0153_live_code_workspace` → `0154_dashboard_permission_resource_reconciliation`。
- `0153_live_code_workspace`：使用 `IF NOT EXISTS` 兼容历史开发库；down 会识别历史 `0150_live_code_workspace` 迁移账本，避免误删由旧迁移持有的共享结构与权限。
- `0154_dashboard_permission_resource_reconciliation`：补齐页面 catalog 需要的资源；恢复曾被开发期覆盖层停用的 `PUT /dashboard/channelCode/update`；保持 customer inheritance 的 8 个 active 资源。
- `/chat/file-audio` benchmark 阶段统一为现有阶段体系中的 `3.5`，未放宽校验器。

## 3. 数据与环境结果

- Docker 项目 `mochat-go-desktop` 的 app、MariaDB、Redis 均为 healthy。
- 命名卷 `app-storage`、`audit-anchor-storage`、`mysql-data`、`redis-data` 全部保留，未执行卷删除。
- 数据库迁移账本中 0152、0153、0154 均存在 64 位 checksum。
- 数据库读回：customer inheritance active 资源为 8；渠道活码更新 active 资源为 1。
- 浏览器使用受 ACL 保护的既有 Dashboard 验收凭据完成真实登录，明文未进入日志或文档；验收结束后已退出登录并清理本任务创建的短期会话夹具，未新增活码业务记录。

## 4. 提交清单

本次归档前的业务、同步与修复提交如下；归档文档自身的提交以最终 `git log origin/main..HEAD` 为准。早期活码页面提交在最终 merge 中已被权威上游实现覆盖，最终工作树不保留重复页面实现。

1. `e02c9656` `docs: design P0 live code integration`
2. `17ab6716` `docs: plan P0 live code integration`
3. `6464c3ac` `docs: reserve live code migration 0150`
4. `5f5a4f1e` `docs: align live code RBAC contract`
5. `a24e3ccb` `feat: add live code workspace schema`
6. `cf5659fa` `feat: enrich channel code workspace data`
7. `9864442c` `feat: add channel code lifecycle controls`
8. `27a3a9b2` `feat: implement live code workspaces`
9. `67a7ecb8` `fix: refine live code drawers`
10. `95e137b9` `fix: align live code drawers with merged workspace`
11. `8d64ec09` `fix: align live code choice interactions`
12. `839529d1` `feat: complete live code edit workflows`
13. `3b78b2a6` `fix: align file audio benchmark phase`
14. `f3b2214a` `fix: reconcile dashboard RBAC resources`
15. `1def3673` `fix: make live code migration legacy-safe`
16. `2cfb276a` `merge: sync authoritative live code optimizations`
17. `680bb372` `fix: reconcile RBAC after live code sync`
18. `fbd14e77` `fix: restore channel update RBAC on upgrade`

## 5. 兼容与回滚

- 代码级：本分支未推送或覆盖其他分支；最安全的整体回滚是停止集成本分支。若只撤销本分支迁移能力，应按逆序撤销 0154、0153，不回滚属于权威上游的 0152。
- 数据库级：回滚前先备份；从 0154 回到 0152 可使用迁移器执行两步 down。`0153` 的 legacy-owner 检查会保护历史 `0150_live_code_workspace` 所持有的共享结构。
- RBAC 级：0154 down 只撤销本迁移新增/修正的精确资源合同；不会整体重建权限表。
- Git merge 回滚：如需仅撤销权威同步 merge，应使用普通 `git revert -m 1 2cfb276a`，不得 force push；随后需重新评估 0152—0154 顺序。

## 6. 已知非阻塞项

- Docker 当前企业的权威活码工作区返回 0 条记录，因此浏览器覆盖了真实空态与新建抽屉，但没有可安全打开的编辑入口。按用户“同步权威代码、不要再改两页”的最新要求，未为验收伪造业务记录。
- Provider 错误态已由组件测试覆盖；浏览器未提交完整新建表单去人为制造 Provider 失败，以免产生真实业务写入。
- Phase 7 与真实企微会话存档继续冻结，未修改。
