# Phase 3.3 超时预警数据模型与 Provider 设计

## 1. 目标

为 `/ai-insight/v2/timeout` 建立可独立运行的数据模型、Provider、HTTP 接口和前端业务闭环，替换当前无 Provider 状态页。功能闭环为：配置监听规则 → 评估客户消息是否超时 → 生成超时记录 → 查询与审计处置 → 留存操作流水。

本轮不实现后台定时扫描器，也不向企业微信实际发送通知；Provider 负责产生可供会话归档任务或后续扫描任务调用的评估入口，并持久化通知意图。

## 2. 圆弧参考界面结论

圆弧页面包含三个页签：

1. **超时记录**：按客户、风险等级、会话类型、规则、创建时间筛选；列表展示超时时长、触发消息、相关人、风险等级、规则、AI 摘要、记录时间和操作；支持批量审计。
2. **规则配置**：按名称查询；规则包含监听员工或部门、单聊/群聊范围、多档超时策略、静音时段、额外通知人员、AI 分析开关、启停状态和触发次数。
3. **高级设置**：结束语词表和白名单消息类型。匹配结束语或白名单消息类型的客户消息不生成超时记录。

本项目不完全复制圆弧样式，但保留上述业务能力和字段语义。

## 3. 数据模型

### 3.1 `mochat_go_timeout_rules`

规则主表：

- `id`、`tenant_id`、`corp_id`
- `name`：规则名称，企业内非空
- `status`：`enabled` / `disabled`
- `monitor_target`：`employee` / `department` / `all`
- `monitor_target_ids_json`：员工或部门 ID 列表
- `conversation_scopes_json`：`single`、`group` 的组合
- `ai_insight_enabled`
- `trigger_count`
- `created_at`、`updated_at`

### 3.2 `mochat_go_timeout_rule_strategies`

一个规则最多五档策略：

- `rule_id`、`tenant_id`、`corp_id`
- `timeout_minutes`：3–180 分钟，同一规则内不可重复
- `notify_type`：`none` / `owner` / `extra`
- `risk_level`：`low` / `medium` / `high`
- `sort_order`

### 3.3 `mochat_go_timeout_rule_quiet_periods`

静音时段：

- `rule_id`、`weekday`：1–7
- `start_time`、`end_time`
- 每条规则最多五段；处于静音时段的消息不生成记录。

### 3.4 `mochat_go_timeout_rule_notify_targets`

额外通知对象：

- `rule_id`
- `target_type`：`employee` / `department`
- `target_id`

### 3.5 `mochat_go_timeout_settings`

企业级高级设置，每企业一条：

- `tenant_id`、`corp_id`
- `closing_phrases_json`：最多五个词表，每个词表最多二十个结束词
- `whitelist_message_types_json`
- `updated_at`

### 3.6 `mochat_go_timeout_records`

超时记录：

- `rule_id`、`strategy_id`
- `conversation_type`、`conversation_id`
- `customer_id`、`customer_name`
- `employee_id`、`employee_name`
- `trigger_message_id`、`trigger_message`
- `message_type`
- `timeout_seconds`
- `risk_level`
- `ai_summary`
- `audit_status`：`pending` / `confirmed` / `ignored` / `closed`
- `assigned_employee_id`
- `occurred_at`、`created_at`
- 唯一键 `(corp_id, rule_id, strategy_id, trigger_message_id)`，保证重复评估不重复落库。

### 3.7 `mochat_go_timeout_record_audits`

处置流水：

- `record_id`、`tenant_id`、`corp_id`
- `actor_id`
- `action`：`confirmed` / `ignored` / `assigned` / `closed`
- `assigned_employee_id`、`remark`、`created_at`

### 3.8 `mochat_go_timeout_notification_intents`

通知意图：

- `record_id`、`strategy_id`
- `notify_type`
- `target_type`、`target_id`
- `status`：本轮只写入 `pending`
- `created_at`

该表为后续通知任务提供稳定输入，本轮不发送外部消息。

## 4. Provider 与接口

Provider 分为三个清晰边界：

- `TimeoutRuleProvider`：规则分页、新增、编辑、启停、删除及关联策略/静音时段/通知对象装载。
- `TimeoutRecordProvider`：记录分页、详情、批量审计、分派与关闭。
- `TimeoutEvaluatorProvider`：依据客户消息、当前时间和已启用规则进行匹配，幂等生成记录与通知意图。

HTTP 路由：

- `GET/POST/PUT/DELETE /dashboard/timeout-warning/rules`
- `PUT /dashboard/timeout-warning/rules/status`
- `GET /dashboard/timeout-warning/records`
- `POST /dashboard/timeout-warning/records/audit`
- `PUT /dashboard/timeout-warning/records/assign`
- `GET/PUT /dashboard/timeout-warning/settings`
- `POST /dashboard/timeout-warning/evaluate`

所有接口必须从当前登录企业解析 `tenant_id` 与 `corp_id`，读写都按两者限定；管理接口使用 `/ai-insight/v2/timeout#manage`，查询接口使用 `/ai-insight/v2/timeout#read`。

## 5. 评估规则

评估输入包含客户消息、消息类型、会话类型、客户与责任员工、消息时间、评估时间：

1. 只处理客户发送且尚未收到员工回复的消息。
2. 规则必须启用，且会话类型、员工/部门范围匹配。
3. 消息命中结束语、白名单消息类型或静音时段时跳过。
4. 对每个已达到阈值的策略分别生成记录；唯一键保证重复调用幂等。
5. `timeout_seconds = evaluated_at - message_at`，不得为负数。
6. 创建记录后递增规则触发次数，并按策略及额外通知对象写入通知意图。
7. AI 摘要本轮允许为空，不因缺少 AI 服务阻断超时记录生成。

## 6. 前端范围

沿用现有 phase3.3 页面结构，实现三个页签：

- 超时记录：客户、风险等级、会话类型、规则筛选；记录列表、详情、批量确认/忽略、分派、关闭。
- 规则配置：规则列表及完整编辑表单，支持最多五档策略和五段静音时段。
- 高级设置：结束语词表与白名单消息类型。

界面字段和枚举均使用中文，不显示“数据提供方未提供”或权限范围说明等内部提示。搜索、查询和操作按钮保持横向文字并确保窄屏可横向滚动。

## 7. 错误处理与约束

- 名称为空、策略数量超限、阈值不在 3–180 分钟、范围为空或枚举非法时返回 400。
- 规则已有超时记录时禁止物理删除，返回可理解的业务错误；无记录时级联删除关联配置。
- 审计、分派或关闭不存在及跨企业记录时不更新数据。
- 数据库写入规则及其关联配置时使用事务。
- Provider 不可用时接口返回 501，但生产接线必须确保 MySQL Provider 已注册。

## 8. 测试与验收

- 迁移测试：表、索引、唯一约束及回滚脚本。
- 单元测试：规则校验、结束语/消息类型/范围/静音时段匹配、阈值计算、幂等键。
- Handler 测试：租户企业解析、读写权限、参数校验和状态码。
- Store 测试：规则事务写入、分页过滤、审计流水、通知意图及跨企业隔离。
- 前端测试：三个页签、Provider 请求、规则表单、记录处置和错误状态。
- 浏览器验收：真实创建、编辑、启停规则；生成测试记录；检查详情、审计、分派、关闭；截图检查中文字段和布局；最终清理验收数据。
- Docker 验收：仅重建应用容器，不执行 `down -v`，保留所有当前数据卷。

## 9. 不在本轮范围

- 定时扫描全量会话消息。
- 实际发送企业微信、短信、邮件或 Webhook 通知。
- 调用外部 AI 服务生成摘要。
- 导出文件生成；保留记录筛选模型，为后续导出复用。
