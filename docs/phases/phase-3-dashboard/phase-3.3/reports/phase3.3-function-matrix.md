# Phase 3.3 功能矩阵

本矩阵固定记录会话与风险预警 15 个页面。`implementation`、`backend`、`acceptance` 的最终值必须与 `web/apps/dashboard/src/benchmark/manifest.json` 同步；设计阶段不得把 `documented` 解释为功能完成。

| page | domain | coreFlow | dependency | permission | persistence | risk | evidence |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `/chat/v2-staff` | 会话 | 员工/时间筛选→消息上下文 | 会话存档、员工目录 | 会话读取+员工数据范围 | 归档消息查询索引 | 高 | 待批次1 |
| `/chat/v2-customer` | 会话 | 客户聚合→关联员工→消息 | 会话存档、客户主数据 | 客户与会话交集范围 | 客户-会话关联索引 | 高 | 待批次1 |
| `/chat/v2-group` | 会话 | 群/时间筛选→群消息 | 会话存档、客户群 | 群与成员数据范围 | 群-消息归档索引 | 高 | 待批次1 |
| `/chat/trajectory` | 会话 | 消息/事件时间线→来源下钻 | 三类会话、客户事件 | 时间线对象读取 | 不可变事件来源指针 | 高 | 待批次2 |
| `/chat/export` | 会话 | 范围配置→异步任务→下载 | 会话查询、任务中心、对象存储 | 导出动作+范围授权 | 任务、审计、过期文件 | 高 | 待批次2 |
| `/chat/file-audio` | 会话 | 媒体筛选→预览/下载→会话 | 会话媒体存储 | 媒体读取+二次校验 | 媒体索引、下载审计 | 高 | 未完成：缺少音频文件存储与读取 Provider |
| `/chat/resign-staff` | 会话 | 离职员工→资产→交接 | 员工目录、会话存档 | 离职资产与交接动作 | 资产交接记录 | 高 | 待批次3 |
| `/chat/refuse-archive` | 会话 | 拒绝名单→授权状态→跟进 | 存档授权 provider | 授权状态读取/维护 | 状态快照、审计 | 高 | Provider、跟进与审计页面已完成 |
| `/customer/inheritance` | 会话 | 离职员工→接替人→结果 | 客户主数据、员工目录 | 客户继承动作 | 幂等交接命令、审计 | 高 | 待批次3 |
| `/ai-insight/v2/risk` | 风险预警 | 规则→命中→处置 | 会话存档、规则引擎 | 规则/事件/处置分权 | 风险事件、证据、审计 | 高 | 待批次4 |
| `/ai-insight/v2/timeout` | 风险预警 | SLA→超时→提醒→关闭 | 会话时间、通知任务 | SLA 配置和处置权限 | SLA 事件、通知、状态 | 高 | 待批次4 |
| `/ai-insight/v2/customer-loss` | 风险预警 | 流失事件→归因→跟进 | 客户事件、会话时间线 | 客户风险与分派范围 | 事件、归因、跟进记录 | 高 | 待批次4 |
| `/ai-insight/v2/message-intercept` | 风险预警 | 发送前检测→解释→审计 | 发送链路、规则引擎 | 策略维护/决定查看 | 拦截决定、规则版本、审计 | 极高 | Provider 与页面已完成；真实外发链路仍保持隔离 |
| `/ai-insight/v2/keyword-library` | 风险预警 | 词条→规则→版本发布 | 规则引擎 | 词库维护/发布权限 | 版本化词库、发布审计 | 高 | Provider、版本发布与页面已完成 |
| `/ai-insight/v2/silent-customer` | 风险预警 | 沉默窗口→筛选→唤醒 | 客户时间线、任务中心 | 客户读取与分派权限 | 计算快照、任务、跟进 | 高 | Provider、规则、分派与处置页面已完成 |

## Phase 3.3 当前验收标记

- 已接入 native 页面：会话运营四页、风险预警六页；
- 已有真实读取接口：`/chat/resign-staff`、`/customer/inheritance`、`/ai-insight/v2/customer-loss`；
- provider 未接入但已具备明确状态页：`/chat/file-audio`；
- 详细命令、截图和数据卷检查见 `acceptance/remaining-pages-acceptance.zh-CN.md`。
