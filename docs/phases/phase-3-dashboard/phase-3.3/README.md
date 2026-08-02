# Phase 3.3：会话与风险预警

## 阶段目标

Phase 3.3 沿用 Phase 3 已确定的业务域拆分，完成会话与风险预警菜单的真实业务闭环。阶段范围固定为 15 个页面：会话域 9 个，风险预警域 6 个。除本文件列出的页面外，不提前纳入营销工具、SCRM 扩展、数据报表、AI 能力或企业设置。

本阶段的完成标准不是菜单可见、路由可达或页面样式完成，而是每个页面具备真实查询或命令主流程，并通过租户/企业/角色隔离、持久化、权限、异常、审计、自动化测试和仓库证据门禁。

## 当前状态

- 工作分支：`phase3.3/session-risk-warning`
- 设计依据：`../reference/PHASE3_CONSTRAINTS.zh-CN.md`、`../reference/PHASE3_SAAS_DASHBOARD_DATAFLOW_AND_STATUS.zh-CN.md`、`../benchmark/feature-matrix.csv`
- Phase 3.2 基线：8 个页面已完成真实后端闭环；Phase 3.2 的未跟踪改动和浏览器最终验收不纳入本阶段基线。
- 当前阶段：范围与设计已冻结，尚未开始实现。
- 前置门禁：必须先确认会话存档许可、数据接入方式、脱敏测试租户和防误发/防导出测试策略。

## 菜单范围

### 会话

| 页面 | 路由 | 核心闭环 |
| --- | --- | --- |
| 员工会话 | `/chat/v2-staff` | 按员工和时间筛选 → 查看会话 → 查看消息上下文 |
| 客户会话 | `/chat/v2-customer` | 按客户聚合 → 查看关联员工 → 查看消息上下文 |
| 群聊会话 | `/chat/v2-group` | 按群和时间筛选 → 查看群会话 → 查看消息上下文 |
| 会话轨迹 | `/chat/trajectory` | 选择对象 → 按时间串联消息与业务事件 → 下钻原会话 |
| 会话导出 | `/chat/export` | 配置范围 → 创建异步导出任务 → 查看状态 → 受控下载 |
| 文件录音 | `/chat/file-audio` | 筛选媒体 → 预览或下载 → 回到关联会话 |
| 离职员工 | `/chat/resign-staff` | 筛选离职员工 → 查看会话资产 → 发起交接 → 查看结果 |
| 拒绝存档 | `/chat/refuse-archive` | 查看拒绝名单 → 查看授权状态 → 记录跟进或状态变化 |
| 客户继承 | `/customer/inheritance` | 选择离职员工 → 分配接替人 → 查看交接结果和审计 |

### 风险预警

| 页面 | 路由 | 核心闭环 |
| --- | --- | --- |
| 风险行为 | `/ai-insight/v2/risk` | 配置/启用规则 → 扫描会话 → 命中预警 → 处置并审计 |
| 超时预警 | `/ai-insight/v2/timeout` | 配置 SLA 阈值 → 识别超时 → 分派提醒 → 关闭或复盘 |
| 客户流失 | `/ai-insight/v2/customer-loss` | 检测流失事件 → 查看归因证据 → 分派跟进 → 更新状态 |
| 消息拦截 | `/ai-insight/v2/message-intercept` | 发送前规则检测 → 拦截或放行 → 展示解释 → 留存审计 |
| 关键词库 | `/ai-insight/v2/keyword-library` | 分组维护词条 → 关联规则 → 发布版本 → 追溯命中来源 |
| 沉默客户 | `/ai-insight/v2/silent-customer` | 配置沉默窗口 → 筛选客户 → 分派唤醒 → 跟踪结果 |

## 文档目录

- `design/2026-08-02-phase3.3-session-risk-warning-design.md`：范围、架构、数据流、状态与门禁设计。
- `plans/2026-08-02-phase3.3-session-risk-warning-plan.md`：按业务批次拆分的实施计划。
- `acceptance/PHASE3.3_ACCEPTANCE.zh-CN.md`：阶段验收入口、命令和阻塞条件。
- `reports/phase3.3-function-matrix.md`：15 个页面的功能、依赖、权限、持久化和证据矩阵。
- `reports/reuse-audit.md`：旧 Dashboard 能力、API、数据表和复用边界的审计规则与记录入口。
- `evidence/`：真实测试、浏览器验收和截图证据；未产生证据前不得标记阶段完成。

## 阶段边界

- 不在本阶段实现营销工具、SCRM 好友/客户群/订单/设置、数据报表、AI 知识库/智能体或企业设置。
- 不把普通消息回调冒充为会话存档数据；会话查询必须明确数据来源、授权范围和保留策略。
- 不在没有真实发送链路和防误发门禁时实现生产消息拦截或群发副作用；测试只允许使用隔离 provider。
- 不用静态 fixture、随机生成内容或仅有单元测试的页面提升为真实完成状态。
