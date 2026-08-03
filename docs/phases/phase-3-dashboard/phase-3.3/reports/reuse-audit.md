# Phase 3.3 旧能力复用审计

## 审计原则

实施每个批次前，逐页核对旧路由、API、handler、store、数据表、任务能力和测试证据。只有确认响应语义、租户/企业边界、权限动作、错误码、分页和持久化行为一致时，才允许通过 adapter 复用；否则建立新的领域合同，不把旧响应直接渗透到 React 页面。

## 已确认的复用证据

| 能力 | 现有证据 | 可复用结论 | 必须补齐 |
| --- | --- | --- | --- |
| 会话存档同步 | `internal/dashboard/work_message_archive_sync_cron.go`：bridge client、`seq` 游标、消息解析、幂等 upsert；`internal/store/work_message_archive_sync.go`：`mc_work_message_id` 与 `mc_work_message_1..10` 写入 | 可作为归档 ingestion 基础 | 生产官方 SDK bridge、媒体解密/索引、保留删除策略、真实企业联调 |
| 会话列表与详情 | `internal/dashboard/auto_tag_dashboard.go`：`WorkMessageToUsers`、`WorkMessageIndex`、`workMessageGlobalDetail`；`internal/store/auto_tag.go`：`WorkMessageToUsers`、`WorkMessagePage`、`WorkMessageByArchiveID` | 可复用查询存储和授权检查，但必须通过新 feature contract 暴露 | 员工/客户/群三种页面筛选合同、统一分页/错误码、轨迹事件模型 |
| 企业存档授权 | `AutoTagStore.WorkMessageArchiveAuthorized`；前端 `conversation-global-page.tsx` 将 `40301` 识别为未开通存档 | 可复用授权失败语义和引导 | 为 15 页统一授权/数据范围门禁，补 API 与页面测试 |
| Dashboard React adapter | `web/apps/dashboard/src/features/conversation-global/conversation-global-api.ts` 已固定 `/workMessage/toUsers` 与 `/workMessage/detail` 响应解析 | 可抽取查询类型和错误处理模式 | 员工/客户/群专页不复制解析逻辑，改为共享会话查询合同 |
| 离职客户转移 | `internal/dashboard/contact_transfer_cron.go`、`internal/config/config.go` 存在状态同步 cron 和迁移开关；旧页面位于 `web/legacy/dashboard/src/views/contactTransfer/` | 只能作为异步状态同步参考 | 客户继承命令、幂等键、并发控制、动作权限、结果审计 |
| 客户流失 | `web/legacy/dashboard/src/api/lossContact.js` 仅发现 `/workContact/lossContact` 列表入口 | 只能复用历史查询语义线索 | 正式风险事件、归因证据、分派/处置和持久化模型 |

## 明确缺失的正式能力

以下页面在当前分支未发现对应新的 Go handler、React API adapter 或真实页面实现，当前仍由 `page-registry.tsx` 的 `DemoPage`/`PlaceholderPage` 覆盖：

- 会话：`/chat/v2-staff`、`/chat/v2-customer`、`/chat/v2-group`、`/chat/trajectory`、`/chat/export`、`/chat/file-audio`、`/chat/resign-staff`、`/chat/refuse-archive`、`/customer/inheritance`；
- 风险预警：`/ai-insight/v2/risk`、`/ai-insight/v2/timeout`、`/ai-insight/v2/customer-loss`、`/ai-insight/v2/message-intercept`、`/ai-insight/v2/keyword-library`、`/ai-insight/v2/silent-customer`。

其中 `/ai-insight/v2/sensitive-word` 已在 Phase 3.2 完成，不重复纳入本次实现；其会话扫描 cron 可作为风险事件 ingestion 的参考，但不能替代统一风险事件模型。

## 审计记录入口

| 批次 | 页面 | 旧路由/API | 可复用能力 | 缺口 | 决策 | 证据 |
| --- | --- | --- | --- | --- | --- | --- |
| 0 | 15 页 | 见上方旧路由/API 证据 | 会话归档同步、全局会话查询、存档授权判断、离职同步 | 15 页正式页面/handler 缺失；媒体、导出、风险事件和规则模型缺失 | 批次 1 先复用会话查询底座；其余按新领域合同建设 | 本文件 |

## 必查项目

- 会话存档是否与普通消息回调区分，是否具备企业微信授权、保留、脱敏和撤回语义；
- 旧接口是否从认证上下文解析 `tenant_id`、`corp_id` 和数据范围；
- 媒体/录音是否有二次权限校验、短期下载和下载审计；
- 导出、同步、规则计算和通知是否已经具备任务状态、重试、取消和幂等；
- 风险命中能否保存规则版本、原始证据引用、处置动作和审计结果；
- 旧页面是否依赖 `demo-fixtures`、无边界全量查询或客户端拼接权限。

## 批次 1 决策

批次 1 允许复用 `WorkMessageToUsers`、`WorkMessageIndex`、`WorkMessageByArchiveID` 及其归档授权判断，但不直接让三张新页面调用旧路径。下一步应先建立共享 `conversation` feature/API contract，再补员工、客户和群聊页面的专属筛选；旧 handler 只作为后端 adapter 的暂时实现，必须补齐统一 `40301`、`403`、`404` 和刷新后读取测试。

任何一项无法证明时，复用决策为“扩展或替换”，不得以“页面能显示”为通过依据。
