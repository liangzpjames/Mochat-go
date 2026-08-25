# AI 洞察目录覆盖、概览同源与 DeepSeek 实跑设计

## 1. 背景与目标

现有三个专用洞察页已经读取 `mochat_go_ai_conversation_insights` 的真实会话结果，但员工筛选项仍从该结果表反查，客户仅提供名称文本输入；因此未产生洞察的有效员工和客户不会进入筛选体系。数据概览的 AI 区域则仍读取旧表 `mochat_go_ai_analysis`，并把结构化情绪指标声明为不可用，与三个优化页面的数据源和口径分叉。

本轮目标是：

1. 让情绪识别、员工评分、沟通关键词使用当前租户与企业的全部有效员工、有效客户作为筛选目录，同时继续遵守员工数据范围。
2. 明确区分“目录中存在”“有真实归档消息”“已成功分析”三种状态，不为无消息实体生成伪结果。
3. 将数据概览 AI 区域切换到与三个洞察页相同的会话洞察持久化表、时间口径与员工范围。
4. 使用用户提供的 DeepSeek 凭证，通过受保护的 Docker secret 运行一次手动会话分析；凭证不进入 Git、数据库、日志、文档或浏览器存储。

自动日分析、启动即分析和 Phase 7 继续保持关闭。页面查询、刷新和打开详情不触发 Provider 调用。

## 2. 根因

### 2.1 目录遗漏

`SQLRepository.EmployeeOptions` 以 `mochat_go_ai_conversation_insights i` 为主表并按 `i.analysis_type` 聚合。只有已经产生结果的员工才会返回。前端的客户筛选是自由文本，没有客户 ID、权威客户目录或员工范围校验。因此当前环境 15 个未删除员工中仅验收数据员工可见，16 个未删除客户无法形成完整、精确的选择集合。

### 2.2 概览分叉

`reporting.SQLRepository.queryOverviewExtras` 的 AI 次数读取旧表 `mochat_go_ai_analysis`，客户/员工情绪直接返回空并附加 `structured_metrics_unavailable`。三个专用页读取 `mochat_go_ai_conversation_insights.result_json` 的 `customer.emotion`、`employeeQa.score` 与 `customer.keywords`。两处不是同一数据链。

### 2.3 实跑覆盖不足

保留卷当前 corp 1 有 25 个归档会话、12 个出现过消息的员工和 12 个出现过消息的客户，但会话洞察仅 4 条验收结果；历史运行还记录过 22 个候选全部因旧保存 SQL 列数错误失败。当前代码已修正保存语句，本轮需要用真实 Provider 对当前规则时间窗中的候选重新执行并如实记录成功、失败、跳过与积压。

## 3. 方案比较

| 方案 | 优点 | 缺点 | 结论 |
| --- | --- | --- | --- |
| 只扩大前端静态下拉 | 改动小 | 绕过租户/范围、会失真、无法解释覆盖 | 拒绝 |
| 为每个员工和客户生成占位洞察 | 页面看似完整 | 把无消息伪装成 AI 结果，污染指标 | 拒绝 |
| 权威目录筛选 + 会话结果投影 + 同源聚合 | 数据真实、范围一致、可解释未分析原因 | 需要扩展 API、前后端合同与报表聚合 | 采用 |

## 4. 数据来源矩阵

| 能力 | 权威来源 | 过滤条件 | 空值语义 |
| --- | --- | --- | --- |
| 可用员工 | `mc_work_employee` | `corp_id`、`deleted_at IS NULL`、`status=1`、员工范围 | 目录无有效员工 |
| 可用客户 | `mc_work_contact` + `mc_work_contact_employee` | 同企业；受限角色只返回其允许员工仍处于有效关系的客户 | 目录无有效客户或当前范围无客户 |
| 可分析会话 | `mc_work_message_1..10` | 企业、规则时间窗、有效文本消息、员工范围 | 有实体但没有归档消息 |
| 已分析员工/客户 | `mochat_go_ai_conversation_insights` | 租户、企业、`analysis_type='session'`、结果状态与员工范围 | 尚未运行、失败或无可分析会话 |
| 客户情绪 | `result_json.$.customer.emotion.label` | 与三个洞察页查询相同 | 缺字段为合同失败，不补零 |
| 员工评分 | `result_json.$.employeeQa.score` | 仅 0–100 数值 | 无有效评分返回空 |
| 关键词 | `result_json.$.customer.keywords` | JSON 数组 | 空数组表示模型未提取，缺字段表示合同失败 |
| 数据概览 AI 指标 | 同一会话洞察表 | 概览选择的时间范围、租户、企业、员工范围 | 表不存在时明确不可用；无结果时真实为 0，平均分无样本时为空 |

## 5. API 与交互

三个页面的 `GET /dashboard/ai-insight/{view}/filter-options` 返回：

- `employees`: 权威有效员工，字段 `id/name/avatar`。
- `customers`: 权威有效客户，字段 `id/name/avatar`。
- `coverage`: `availableEmployeeCount`、`availableCustomerCount`、`analyzedEmployeeCount`、`analyzedCustomerCount`。

员工和客户支持关键字搜索，初始结果按名称、ID 稳定排序；受限角色的两类选项和覆盖数都受 `AllowedEmployeeIDs` 约束。客户筛选改用 `customerId` 精确请求参数，URL 可恢复；保留 `customerName` 后端兼容，但新页面不再依赖模糊名称作为主筛选。

三个页面在摘要区显示权威目录覆盖和当前查询结果。存在目录但无结果时，空态明确说明“当前目录实体没有符合时间窗的已持久化分析”，不显示伪成功。

数据概览 AI 区域展示：

- 已分析会话数；
- 负向客户会话数；
- 平均员工评分；
- 关键词总数；
- 已覆盖员工数；
- 已覆盖客户数。

每张卡链接到对应专用页面。指标按概览日期范围统计 `source_ended_at`，并套用与报表相同的员工范围。旧自然语言 AI 摘要可继续保留兼容字段，但不再驱动结构化数字。

## 6. DeepSeek 受控实跑

使用 OpenAI 兼容 Provider，Base URL 为 DeepSeek 官方兼容地址，模型为 `deepseek-chat`。密钥写入工作区外的临时受限文件，通过 `deploy/standalone/docker-compose.ai.yml` 的 Docker secret 挂载；Compose 环境中的明文 Key 保持为空。只临时启动一次启用 Provider 的 app，调用现有手动会话分析入口或一次性 runner；自动日分析与启动即分析仍为 0。

执行前记录候选数和时间窗；执行后记录最新 run 的 candidate/success/failure/backlog、成功结果的 provider/model/promptVersion/generatedAt，并抽查证据消息。外部调用失败按真实错误记录，不把部分成功宣称为全量成功。

## 7. 权限、安全与诚实状态

- 租户与企业始终来自 Dashboard principal，不接受请求体决定作用域。
- 普通角色只能看到允许员工及其有效客户，详情、导出和概览聚合使用同一范围。
- Provider Key 不写入跟踪文件、数据库、日志、测试快照或最终报告；临时文件在 Provider 运行结束后删除。
- 不生成没有来源消息的洞察，不把知识库作为证据，不启用模拟归档。
- 数据概览不再展示没有数据合同支撑的“员工负面情绪”；以真实员工评分替代。

## 8. 验收标准

1. corp 1 的有效员工和有效客户均可通过筛选 API 搜索到；离职员工不属于可用员工；受限角色无法越权看到其他员工或其客户。
2. 客户和员工选择会精确改变列表请求、结果与 URL，刷新、前进后退和重置可恢复。
3. 三页覆盖数字来自目录和会话结果表；无结果实体不会生成占位洞察。
4. 数据概览 AI 六项指标与同日期、同员工范围下三个洞察页的持久化结果一致。
5. DeepSeek 运行产生真实 run 记录；成功结果可追溯到 Provider、模型、提示词版本、来源时间窗和消息证据，失败结果保留真实原因。
6. 目标单测完成红绿循环；相关 Go、Dashboard 测试、typecheck、lint、build、MariaDB integration、Docker 健康和真实浏览器验收通过。

## 9. 自审

本文没有待定项。目录全集与洞察结果全集明确分离；概览字段均能映射到现有 JSON schema；凭证生命周期、员工范围、无数据语义和外部调用失败边界明确。
