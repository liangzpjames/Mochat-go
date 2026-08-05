# Phase 3.5 复用审计

本审计只确认可复用的数据身份、权限和持久化边界，不把路由、fixture、demo 或静态提示视为实现。

| 路由 | 可复用数据/API/Provider | 权限与身份边界 | 已确认缺口 |
| --- | --- | --- | --- |
| `/customer/friends` | `mc_work_contact`、`mc_work_contact_employee`、联系人标签与负责人 store | tenant、corp 与员工数据范围取交集；按 `wx_external_userid`/主键关联 | 需要聚焦的列表/详情合同；同步仅可调用真实企微 Provider |
| `/customer/group` | `mc_work_room`、`mc_work_contact_room`、群主员工资料 | tenant、corp、群主/成员可见范围 | 需要群列表/详情适配器及 Provider 新鲜度 |
| `/customer/order` | SCRM 联系人、商机和幂等键 | 关联对象必须同 tenant/corp | 无等价订单模型，需最小订单表、审计和乐观锁 |
| `/customer/settings` | `mochat_go_scrm_tags`、`mochat_go_scrm_stages`、assignments | 所有配置按 tenant/corp 隔离 | 无统一版本化字段/来源/阶段/分配规则模型，需最小配置表 |
| `/data/customer` | work contacts、SCRM contacts/assignments/follow-ups | 禁止姓名关联；按可验证主键去重 | 需要统一时间窗口聚合与明细 |
| `/data/employee` | `mc_work_message_*` 归档、组织员工范围 | 归档授权与员工范围同时满足 | Provider/授权不可用时必须返回 `limitations` |
| `/data/conversion` | leads、contacts、opportunities、orders | tenant/corp + 稳定对象 ID | 需要统一漏斗事件口径，零分母返回 `null` |
| `/data/behavior` | 已存在且可审计的业务事件 | 未知事件类型拒绝，不虚构事件 | 需要事件白名单及明细分页 |
| `/data/report` | 复用上述 reporting 服务 | 不复制 SQL，复用同一数据范围 | 需要报表选择与统一返回合同 |

实际模块装配入口并非计划中不存在的 `internal/app/modules/modules.go`；实现时应复用当前 `cmd`/Dashboard 路由装配结构。Dashboard 客户端基址已经是 `/dashboard/`，前端 API 使用 `/reports/...`、`/scrm/...` 等相对路径，禁止再次添加 `/dashboard`。

## Migration 结论

好友、群和报表查询复用现有业务表；订单与版本化客户设置没有等价表，允许从当前最大 migration 编号顺延增加最小持久化，并必须具备 apply、rollback、replay 与 MySQL 5.7/MariaDB 验证。
