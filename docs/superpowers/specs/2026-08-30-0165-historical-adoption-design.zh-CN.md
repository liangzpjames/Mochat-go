# 0165 历史已执行环境采纳设计

## 问题

`0165_ai_daily_insight_unification` 在旧版本中已经由普通迁移器执行，后续才被注册为受控迁移。新 runner 会要求数据库存在迁移前快照和 `verified` 控制记录；历史环境无法补造迁移前数据，因此 status/apply 永久失败，后续迁移也无法执行。

不能插入伪造的 `verified` 记录。那会把“只有迁移后备份、无法恢复历史被删行”的环境错误描述为完整受控执行。

## 决策

新增独立 `adopt-existing` 动作，仅用于账本已经记录 0165 的历史环境。首次对真实历史库执行时发现：同一份已发布 SQL 在 LF 与 CRLF 工作树中的原始字节 checksum 不同；普通 runner 已通过 `migrationLineEndingChecksumAliases` 将这两种值登记为等价别名，但受控 controller 错误地只接受当前工作树值。接管不能接受任意历史 checksum，只能接受当前值或迁移注册表从同一 SQL 自动生成的 LF/CRLF 别名，并把账本中的实际值单独持久化以便审计。

动作必须同时满足：

1. 操作者显式确认业务流量已经停止；
2. 提供稳定 request ID；
3. 提供已经完成外部恢复校验的逻辑或物理备份 SHA-256；
4. 普通迁移账本中的 0165 checksum 等于当前原始字节值，或等于注册表从同一 SQL 自动生成的 LF/CRLF 换行别名；
5. 0165 后置列、唯一索引和旧表删除状态全部符合当前合同；
6. 同一 checksum 只能存在一条历史采纳记录。

控制表记录 `adopted_existing`，并持久化当前脚本 checksum、账本实际 checksum、外部备份 SHA、固定恢复边界和采纳时间。该状态只证明“当前后置 schema、已登记为同源换行变体的历史 ledger 和已恢复验证的当前备份一致”，不证明迁移前被删除数据可恢复，也不等同于 `verified`。

普通 runner 仅在上述采纳记录唯一、checksum 一致、恢复边界完整且当前后置 schema 仍有效时接受历史环境。新环境仍必须走 backup → preflight → apply → verify，不能使用采纳动作跳过未执行的 0165。

### 通用受控账本的换行一致性

0165 采纳成功后，真实历史库继续在 0130 阻断。数据库中的 0130/0131 普通账本、受控成功 ledger 和 completed batch 均完整，三者保存的 checksum 又与 Git 中 LF 原始字节完全一致；Windows 当前工作树只是 CRLF 值不同。根因是通用 `controlledMigrationBaselineEvidence` 和 `RecordControlledMigration` 仍用字符串精确相等校验受控 ledger，没有复用普通 runner 已登记的 LF/CRLF 别名。

通用修复只改变 checksum 等价判定，不采纳缺失证据：0130/0131 仍必须各自具备恰好一条 success ledger 和一个 completed batch，0131 仍必须满足 `mc_corp.tenant_id` 后置合同。受控 ledger 的 `scriptChecksum` 只有在等于当前值或该迁移注册表的同源换行别名时才接受；任意其他 SQL 内容变化继续失败。

### 已重编号迁移的历史事实

0172–0176 应用成功后的 status 还发现一条 `0150_live_code_workspace` 历史账本。Git 分支 `5157dd86` 与 `b01f2d1d` 保存的旧 0150 SQL，其 LF checksum 精确为数据库值 `f8967839...`；当前 0153 设计和 down SQL 又明确把该旧版本视为共享 schema 的历史所有者。0153 已在同一数据库以当前合同成功落账，因此旧行不是未知未来版本，也不能删除以抹去历史。

runner 仅把版本名精确为 `0150_live_code_workspace`、checksum 精确等于已审计旧 SQL 的 LF/CRLF 两个值、且替代版本 `0153_live_code_workspace` 已按当前注册表通过校验的记录标为 `superseded`。版本名不符、checksum 不符或替代版本未应用时仍为 `database_ahead`，status 继续失败。

## 验收

- 已执行 0165、缺控制表的环境：普通 status/apply 先失败；采纳后通过并可继续 0172–0176。
- 未执行 0165、checksum 不是当前值或已登记换行别名、后置 schema 不符、缺停流确认、备份 SHA 非法：采纳失败且不写记录。
- 重复采纳：幂等返回同一事实或明确拒绝冲突，不产生第二条记录。
- 输出与文档必须明确 `verified=false` 和不可恢复边界。
- 已有完整 0130/0131 成功证据但工作树换行不同：普通 runner 接受已登记别名；缺 ledger、缺 batch、非登记 checksum 或 0131 后置结构不符仍失败。
- 已审计 0150 活码账本且 0153 已有效应用：status 标记 `superseded` 并保留历史行；伪造 checksum 或没有 0153 时仍标记 `database_ahead`。
