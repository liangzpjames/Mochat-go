# AI 洞察会话分析与智能分析优化设计

> 文档日期：2026-08-21
>
> 审阅状态：待用户审阅，尚未开始实施
>
> 目标路由：`/ai-insight/session-analysis`、`/ai-insight/smart-analysis`

## 1. 目标与停止点

本轮只优化“AI 洞察”下的两个菜单：

1. `会话分析`：`/ai-insight/session-analysis`；
2. `智能分析`：`/ai-insight/smart-analysis`。

用户原请求中的“只能分析”按现有菜单、路由和圆弧 AI 页面核对后，确定为“智能分析”。情绪识别、员工评分、沟通关键词以及风险预警下的页面均不在本轮范围。

本轮先完成调研、设计文档和实施文档，不修改业务代码、不构建、不部署。用户审阅通过后，才按测试先行方式进入开发。

设计目标：

- 保留现有侧边栏、全局顶部栏、菜单名称和路由；
- 借鉴圆弧 AI 的筛选密度、结果表、分页和详情层级，不复制其数据、评分、文案和不可验证能力；
- 两页不再共用“能力状态 + 三张统计卡 + 一段总摘要”的空壳；
- 每一条结果对应一个真实会话、真实分析批次和真实来源消息；
- 会话分析提供客户洞察与员工质检的结构化详情；
- 智能分析提供可配置规则、规则关联结果与证据详情；
- 页面读取不调用模型，后台任务负责增量分析，刷新只重新读取已落库结果；
- 固定每页 20 条，筛选、分页和选中详情写入 URL；
- 按租户、企业、员工数据范围和动作权限约束列表、详情、导出及规则管理；
- 以 `2560×1440` 为主要验收尺寸，同时回归常规桌面宽度；
- 实施完成后必须自行点击验证，不以测试通过代替真实页面验收。

## 2. 调研基线

### 2.1 圆弧 AI 会话分析

2026-08-21 已在登录态核对参考路由：

`https://web-ai-analysis-work-wechat.yuanhu.com/#/ai-insight/session-analysis`

可验证结构：

- 顶部为紧凑筛选区，不使用介绍大卡；
- 筛选包括客户名称、跟进人姓名、手机号、标签、成交意愿区间和分析日期；
- 查询、重置、导出位于同一操作区；
- 结果表固定展示沟通人、分析结果、时间和操作；
- 行内摘要同时展示成交意愿、流失风险和短摘要；
- “分析详情”打开大尺寸弹层，包含“客户分析”和“员工质检”两个分区；
- 客户分析包含质量等级、成交意愿、流失风险、沟通摘要、关键词、需求、情绪和行动建议；
- “会话详情”提供回到原始会话的入口；
- 表格使用固定页大小和完整分页。

MoChat 采用其信息优先级和交互节奏，但不直接采用圆弧 AI 的分数、权重、风险算法和示例内容。所有结论必须来自 MoChat 归档消息和结构化模型输出，并保留证据消息标识。

### 2.2 圆弧 AI 智能分析

2026-08-21 已在登录态核对参考路由：

`https://web-ai-analysis-work-wechat.yuanhu.com/#/ai-insight/smart-analysis`

可验证结构：

- 筛选包括客户名称、群名称、跟进人和分析日期；
- 查询与刷新位置清楚；
- 结果表固定展示沟通人/群聊名称、关联规则、分析结果概述、时间和操作；
- 当前参考租户为真实空列表，分页仍保持稳定。

参考页没有展示规则配置入口，不能据此推断 MoChat 已有规则来源。MoChat 当前也没有智能分析规则存储，因此本设计在同一路由增加“分析结果 / 分析规则”两个标签，建立可解释的规则来源，不用前端常量伪造“关联规则”。

### 2.3 MoChat 当前页面

两个路由目前都渲染 `web/apps/dashboard/src/features/ai-insight/ai-insight-pages.tsx` 中的通用 `AiInsightPage`，只替换标题和描述。

当前问题：

1. 页面结构相同，不能表达两个菜单不同的业务任务；
2. 顶部三张大卡重复显示“AI 能力未接入”，占据较多空间；
3. 没有客户、群聊、员工、日期和规则筛选；
4. 没有分页、URL 状态、导出、详情和会话跳转；
5. 结果只读取 `data[0].summary`，关键词虽然在合同中存在，但当前始终为空数组；
6. 页面将“没有新结果”“模型不可用”“查询失败”合并成同一受限态；
7. 两个页面没有独立测试，只由情绪识别页面的通用受限态测试间接覆盖；
8. 当前本地页面未展示数据库已经存在的历史分析结果，接口读取错误被静默降级成“每日分析尚未生成”，不利于定位问题。

### 2.4 当前后端与数据链

当前链路：

```text
每日定时任务
  → 从 `mc_work_message_1` 至 `mc_work_message_10` 依次取最多 20 条 `content_text`
  → 对每个企业、每个 AI 洞察页面调用一次模型
  → 将一段 summary 写入 mochat_go_ai_analysis
  → GET /dashboard/ai-insight/{page} 读取该企业该页面最近一行
```

已确认的结构性问题：

- 结果是“企业 + 页面”级总摘要，不是“会话”级结果；
- 取数只带消息正文，没有消息时间、发送方向、员工、客户/群聊和会话上下文；
- 十张分表按表号依次取数，不是跨分表的全局时间排序；
- 定时任务忽略员工数据范围，而页面接口虽然解析了数据范围，读取时并未使用；
- `mochat_go_ai_analysis.tenant_id` 当前写死为 `0`；
- 查询只按 `corp_id + page`，没有租户条件；
- 模型返回被整体存入 `summary`，缺少可验证的结构化字段和证据引用；
- 页面打开时虽然不调用模型，但读取错误被忽略，无法区分无结果和存储故障。

### 2.5 本地真实数据与运行状态

2026-08-21 对 `mochat-go-desktop` 做了只读核对：

| 数据 | 当前值 | 结论 |
|---|---:|---|
| 客户单聊归档消息 | 41 条 | 可形成 14 个真实会话候选 |
| 客户群归档消息 | 15 条 | 可形成 8 个真实会话候选 |
| 企业员工 | 13 名 | 可用于员工筛选和权限验收 |
| 外部联系人 | 15 名 | 可用于客户筛选和资料快照 |
| 客户群 | 6 个 | 可用于群聊筛选和资料快照 |
| 会话分析历史结果 | 5 行 | 仅企业级总摘要，最近生成于 2026-08-21 00:00:05 |
| 智能分析历史结果 | 5 行 | 仅企业级总摘要，最近生成于 2026-08-21 00:00:19 |

本地容器中的 AI 定时任务相关开关、服务地址和模型名称已配置，但当前应用容器没有可用的 Provider Key；数据库中的历史结果来自此前成功任务。实施验收必须区分“读取历史结果”“生成新结果”和“Provider 当前不可用”，不能把三者混为一谈。

## 3. 方案比较与结论

### 方案 A：保留通用页面，只调整颜色、间距和卡片

- 优点：改动最少；
- 缺点：企业级总摘要、权限泄漏风险、无筛选/分页/详情、两个页面语义相同等问题全部保留。

### 方案 B：继续使用 `mochat_go_ai_analysis`，把模型文本拆成几个前端卡片

- 优点：后端改动较少；
- 缺点：仍无法按会话筛选、无法关联客户/群/员工、无法提供证据和规则，也不能实现圆弧式结果表。

### 方案 C：建立会话级分析快照与智能规则工作台（采用）

以真实会话键为最小分析单元，保存来源窗口、对象快照、结构化结论、证据消息、模型和提示词版本；智能分析结果再关联真实规则版本。两个页面共享紧凑工作台骨架，但分别使用独立接口、表格和详情组件。

该方案改动较多，但能一次解决数据口径、权限、交互和可追溯性问题，并为后续情绪识别、员工评分和沟通关键词复用同一分析底座。本轮只接入前两个菜单，不改另外三个页面。

## 4. 共享页面设计

### 4.1 页面骨架

```text
页面标题 + 最近完成时间/任务状态 + 必要主操作
筛选区：字段 + 查询 + 重置 + 刷新（会话分析增加导出）
紧凑任务状态条（仅在失败、积压或 Provider 不可用时出现）
结果表 / 规则表
固定分页
右侧详情抽屉或大尺寸详情弹层
```

约束：

- 页面标题不放入大块卡片，不显示“AI 洞察”眉标题；
- 不保留三张通用能力统计卡；
- 查询输入维护 `draftFilters`，点击查询或回车后才更新 `appliedFilters`；
- 刷新只重新请求已提交条件，不重置筛选、不触发模型调用；
- 首次加载使用轻量骨架，后台刷新保留旧数据；
- 空态只说明当前筛选没有已完成结果，并提供“重置筛选”；
- 失败态保留筛选和旧列表，提供“重新加载”；
- 任务故障只显示一条紧凑状态条，不重复铺满页面；
- 固定每页 20 条；
- 整行可进入详情，复选框、按钮和链接阻止冒泡；
- 详情支持关闭按钮、遮罩、`Escape`，关闭后焦点返回触发行；
- hover 只改变颜色、背景和边框，不使用位移或缩放；
- 宽屏充分使用内容区，常规桌面允许表格横向滚动但不允许页面整体溢出。

### 4.2 共享业务状态

任务状态统一为：

- `pending`：等待分析；
- `running`：分析中；
- `succeeded`：分析完成；
- `failed`：本次分析失败；
- `stale`：来源会话已产生新消息，当前结果待更新。

页面能力状态统一为：

- `ready`：Provider 可用且最近任务成功；
- `degraded`：仍可读取历史结果，但最近任务失败、积压或 Provider 当前不可用；
- `empty`：任务可用但当前筛选没有已完成结果；
- `unavailable`：存储或权限链不可用。

页面不把 `degraded` 伪装成零数据，也不使用大块“能力未接入”卡片。

## 5. 会话分析页面

### 5.1 筛选区

字段：

- 客户/群聊名称；
- 跟进员工：使用真实部门/员工选择器；
- 会话类型：全部、客户单聊、客户群聊；
- 成交意愿：低、中、高、证据不足；
- 流失风险：低、中、高、证据不足；
- 分析日期：开始日期、结束日期；
- 关键词：匹配摘要、关键词和需求提炼。

操作：查询、重置、刷新、导出当前筛选。

不增加手机号和客户标签筛选。当前会话分析数据底座无法稳定保证手机号和历史标签快照完整；这些字段不是完成本页核心任务的必要条件，避免为追求参考页一致而扩大查询耦合。

### 5.2 结果表

固定列：

1. 选择；
2. 沟通对象：员工、客户/群聊头像和名称；
3. 分析结果：成交意愿、流失风险、短摘要；
4. 会话范围：来源消息时间范围、消息数；
5. 分析时间与状态；
6. 操作：分析详情、会话详情。

头像优先使用真实头像；缺失时使用对象名称首字，不使用固定“请”字或假头像。

导出使用服务端 CSV，字段与当前筛选一致，包含 UTF-8 BOM、中文表头和公式前缀防护。导出不包含完整消息正文和模型内部提示词。

### 5.3 分析详情

详情采用宽度约 `920px` 的抽屉或大尺寸弹层，并保持两类信息分区：

#### 客户分析

- 客户质量：等级、置信度、理由；
- 成交意愿：等级、可选百分制分值、维度和证据；
- 流失风险：等级、维度、证据和风险说明；
- 沟通摘要；
- 沟通关键词；
- 核心诉求与隐性需求；
- 客户情绪；
- 行动建议、推荐话术、动作清单和注意事项；
- 分析来源窗口、消息数、生成时间、模型和提示词版本。

#### 员工质检

- 总体评分；
- 响应时效、需求理解、专业表达、合规性和推进能力；
- 做得好的地方；
- 待改进问题；
- 可执行改进建议；
- 每项证据消息。

任何分数或结论证据不足时显示“证据不足”，不以 `0` 代替。证据只引用本企业、本会话且当前用户有权读取的消息 ID、时间和短摘；详情接口二次校验权限。

### 5.4 会话详情跳转

根据会话快照构造已有路由：

- 客户单聊：`/chat/v2-customer?customerId={targetId}&conversationId={conversationKey}`；
- 客户群聊：`/chat/v2-group?roomId={targetId}&conversationId={conversationKey}`。

跳转前不在前端自行拼接未经后端验证的对象类型；详情响应直接返回已校验的 `conversationUrl`。

## 6. 智能分析页面

### 6.1 信息架构

页面包含两个标签：

1. `分析结果`（默认）；
2. `分析规则`。

标签、筛选、页码、选中结果或规则写入 URL。切换标签不会丢失各自已提交筛选。

### 6.2 分析结果

筛选字段：

- 客户/群聊名称；
- 跟进员工；
- 会话类型；
- 关联规则；
- 结果状态；
- 分析日期。

固定列：

1. 沟通对象；
2. 关联规则及版本；
3. 分析结果概述；
4. 关键证据数量；
5. 会话范围；
6. 分析时间与状态；
7. 操作。

详情抽屉展示规则目标、结构化结论、证据消息、建议动作、来源窗口、规则版本和模型版本，并提供会话详情跳转。

### 6.3 分析规则

规则表筛选：规则名称、状态、会话范围。

固定列：规则名称、分析目标、会话范围、对象范围、当前版本、最近命中/运行时间、状态、更新时间、操作。

新增/编辑规则使用右侧抽屉：

- 规则名称，1～80 字；
- 分析目标，1～500 字；
- 会话范围：客户单聊、客户群聊，可多选；
- 对象范围：全部、部门、员工；
- 回看窗口：1～30 天；
- 最少消息数：2～50 条；
- 启用状态。

规则指令不是自由执行脚本。后端使用固定系统提示词包裹用户目标，禁止工具调用、外部链接执行和跨会话取数；模型只返回固定 JSON 结构。

规则每次保存生成不可变版本。历史结果继续关联生成时的规则版本，编辑规则不会改写历史结论。启停和删除需要二次确认；删除采用软删除并保留历史结果。

同一企业最多启用 10 条智能分析规则，避免任务量和模型成本无边界增长。

## 7. 结构化分析合同

### 7.1 会话分析结果

后端保存并严格校验以下 JSON 结构：

```json
{
  "schemaVersion": 1,
  "summary": "本次会话的客观摘要",
  "customer": {
    "qualityLevel": "medium",
    "qualityReason": "基于当前消息的理由",
    "purchaseIntent": { "level": "medium", "score": 70, "reason": "理由", "evidenceMessageIds": ["msg:1"] },
    "churnRisk": { "level": "medium", "score": 45, "reason": "理由", "evidenceMessageIds": ["msg:2"] },
    "keywords": ["报价", "私有化"],
    "explicitNeeds": ["获取私有化方案"],
    "implicitNeeds": ["确认交付周期"],
    "emotion": { "label": "neutral", "reason": "理由", "evidenceMessageIds": ["msg:3"] },
    "recommendedReply": "建议回复",
    "actions": ["发送方案书"],
    "notes": ["不得承诺未确认的交付日期"]
  },
  "employeeQa": {
    "score": 82,
    "dimensions": [{ "key": "needs_understanding", "score": 85, "reason": "理由", "evidenceMessageIds": ["msg:4"] }],
    "strengths": ["主动确认需求"],
    "issues": ["缺少明确下一步时间"],
    "suggestions": ["给出下一次跟进时间"]
  }
}
```

枚举值由服务端限定。分数可为 `null`；缺少证据时必须返回 `null + reason`，不能输出伪造的 `0`。

### 7.2 智能分析结果

```json
{
  "schemaVersion": 1,
  "conclusion": "规则目标下的结论",
  "matched": true,
  "confidence": 0.86,
  "evidenceMessageIds": ["msg:8", "msg:10"],
  "recommendations": ["由负责人在 24 小时内确认报价边界"]
}
```

解析失败、缺字段、未知枚举或引用不属于来源窗口的消息 ID 时，本次任务标记为 `failed`，不得把原始模型文本当作成功结果展示。

## 8. 数据模型

新增迁移 `0148_ai_conversation_insights`，保留旧 `mochat_go_ai_analysis` 供另外三个旧页面和历史兼容使用。

### 8.1 `mochat_go_ai_analysis_rules`

保存智能分析规则主体：租户、企业、名称、会话范围、对象范围、回看天数、最少消息数、状态、当前版本、创建/更新/删除信息。

### 8.2 `mochat_go_ai_analysis_rule_versions`

保存不可变规则版本：规则 ID、版本号、分析目标快照、范围快照、创建人和创建时间。唯一键为 `tenant_id + corp_id + rule_id + version`。

### 8.3 `mochat_go_ai_conversation_insights`

每行对应“分析类型 + 规则版本 + 会话 + 来源窗口”的一个结果：

- `analysis_type`：`session` 或 `smart`；
- `rule_id/rule_version_id`：会话分析为 `0`，智能分析必填；
- `conversation_key`：`employeeId:targetType:targetId`；
- 员工、客户/群聊名称和头像快照；
- 来源开始/结束时间、消息数、来源指纹；
- 状态、摘要、结构化 JSON、错误摘要；
- Provider、模型、提示词版本、生成时间；
- 创建和更新时间。

幂等键：`tenant_id + corp_id + analysis_type + rule_version_id + conversation_key + source_fingerprint`。

默认列表展示最新一次分析尝试：会话分析按 `conversation_key` 分组，智能分析按 `rule_version_id + conversation_key` 分组。失败尝试不得覆盖或删除之前的成功行，历史成功结果继续保留用于审计。

### 8.4 `mochat_go_ai_insight_runs`

保存任务批次和运行状态：分析类型、规则版本、计划时间、开始/结束时间、候选数、成功数、失败数、状态和错误摘要。页面状态条从此表读取，不从日志猜测。

## 9. 数据流与任务调度

```text
`mc_work_message_1` 至 `mc_work_message_10`
        ↓ 按 tenant/corp/员工范围外的企业级后台任务读取
跨分表按 msg_data_time + seq + table_index + id 全局排序
        ↓ 以 employee_id:to_user_type:to_user_id 分组
补齐员工、客户/群聊快照与消息方向
        ↓ 计算来源指纹，跳过已成功的相同窗口
会话分析固定提示词 / 已启用智能规则版本
        ↓ 调用 AI Provider，严格解析 JSON
ai_conversation_insights + insight_runs
        ↓
页面按当前用户员工数据范围读取列表和详情
```

任务原则：

- 页面打开和刷新永不调用模型；
- 继续使用现有每日调度开关和执行小时；
- 每个企业先生成会话候选，再按会话增量分析；
- 会话分析默认回看最近 30 天、最多 200 条消息；
- 智能分析按规则的回看天数和最少消息数执行；
- 只有来源指纹变化才生成新结果；
- 单企业并发最多 2 个模型请求；
- 单次运行设置可配置候选上限，超过部分保留到下一批并在运行状态中记录积压；
- 某一会话失败不终止整个企业批次；
- 失败不覆盖上一条成功结果；
- 定时任务拥有企业级读取权，但结果页面始终按当前账号员工范围过滤；
- 原始消息不写入分析结果表，模型请求日志不记录正文。

## 10. API 设计

### 10.1 会话分析

```text
GET /dashboard/ai-insight/session-analysis/records
GET /dashboard/ai-insight/session-analysis/detail?id={insightId}
GET /dashboard/ai-insight/session-analysis/status
GET /dashboard/ai-insight/session-analysis/export
```

列表参数：`keyword`、`employeeId`、`conversationType`、`purchaseIntent`、`churnRisk`、`startDate`、`endDate`、`page`、`pageSize=20`。

### 10.2 智能分析

```text
GET    /dashboard/ai-insight/smart-analysis/records
GET    /dashboard/ai-insight/smart-analysis/detail?id={insightId}
GET    /dashboard/ai-insight/smart-analysis/status
GET    /dashboard/ai-insight/smart-analysis/rules
POST   /dashboard/ai-insight/smart-analysis/rules
PUT    /dashboard/ai-insight/smart-analysis/rules
DELETE /dashboard/ai-insight/smart-analysis/rules?id={ruleId}
POST   /dashboard/ai-insight/smart-analysis/rules/status
```

结果列表参数：`keyword`、`employeeId`、`conversationType`、`ruleId`、`status`、`startDate`、`endDate`、`page`、`pageSize=20`。

规则列表参数：`keyword`、`status`、`conversationType`、`page`、`pageSize=20`。

### 10.3 旧接口兼容

`GET /dashboard/ai-insight/session-analysis` 和 `GET /dashboard/ai-insight/smart-analysis` 暂时保留一版，只返回兼容状态或最近企业级历史结果，前端新页面不再使用。情绪识别、员工评分和沟通关键词继续走旧接口，本轮不重写。

## 11. 数据来源矩阵

| 页面字段/动作 | 前端接口 | 后端/存储 | 口径与权限 |
|---|---|---|---|
| 会话分析列表 | `GET /dashboard/ai-insight/session-analysis/records` | `mochat_go_ai_conversation_insights` | 当前 tenant/corp、`analysis_type=session`、当前员工范围、每页 20 条 |
| 会话分析详情 | `GET /dashboard/ai-insight/session-analysis/detail` | insight 结构化 JSON + 来源消息短摘 | 二次校验 insight 所属企业和员工范围 |
| 会话分析状态 | `GET /dashboard/ai-insight/session-analysis/status` | `mochat_go_ai_insight_runs` + Provider 状态 | 当前 tenant/corp，只返回运行元数据 |
| 会话分析导出 | `GET /dashboard/ai-insight/session-analysis/export` | 与列表相同查询 | 当前员工范围和导出权限，CSV 不含消息全文 |
| 智能分析列表 | `GET /dashboard/ai-insight/smart-analysis/records` | insights + rule version | 当前 tenant/corp、`analysis_type=smart`、当前员工范围 |
| 智能分析详情 | `GET /dashboard/ai-insight/smart-analysis/detail` | insight 结构化 JSON + 规则版本 | 二次校验企业、员工范围和规则快照 |
| 智能规则列表 | `GET /dashboard/ai-insight/smart-analysis/rules` | rules + current version | 当前 tenant/corp，配置读取权限 |
| 智能规则写入 | `POST/PUT/DELETE/status` | rules + immutable versions | 当前 tenant/corp，规则管理权限，服务端校验上限和范围 |
| 员工选择器 | `GET /dashboard/workMessage/staffDirectory` | 企业员工、部门、归档统计 | 只返回当前账号可选员工 |
| 对象快照 | 后台任务内部查询 | 员工、外部联系人、客户群表 | 在生成时保存名称和头像，历史结果不随资料改名漂移 |
| 来源消息 | 后台任务内部查询 | `mc_work_message_1` 至 `mc_work_message_10` | 跨分表全局排序，只处理当前企业有效归档消息 |

## 12. URL 状态

```text
/ai-insight/session-analysis?
keyword=&employeeId=&conversationType=&purchaseIntent=&churnRisk=&startDate=&endDate=&page=1&insightId=

/ai-insight/smart-analysis?
tab=results&keyword=&employeeId=&conversationType=&ruleId=&status=&startDate=&endDate=&page=1&insightId=

/ai-insight/smart-analysis?
tab=rules&ruleKeyword=&ruleStatus=&ruleConversationType=&rulePage=1&ruleId=
```

空值不写入 URL。修改已提交筛选后页码重置为 1；关闭详情删除对应 ID；浏览器前进/后退和页面刷新恢复状态。

## 13. 权限、安全与成本

1. 资源拆分为 `read`、`detail`、`export`、`rule.manage`，不能由页面可见权限推导写权限；
2. 所有查询必须同时限制 `tenant_id` 和 `corp_id`；
3. 结果和详情额外限制当前账号允许的 `employee_id`；
4. 智能规则部门/员工范围只能选择当前企业对象，后端重新校验；
5. 后台任务处理企业级消息，但不得把不同企业或不同会话的消息拼入同一提示词；
6. 消息正文、提示词、模型原始响应不写入 URL、普通日志或错误摘要；
7. 模型输出按固定 JSON Schema 校验，并校验所有证据消息属于来源窗口；
8. CSV 做 UTF-8 BOM、换行转义和公式前缀防护；
9. 每企业启用智能规则不超过 10 条，任务并发和单批候选数可配置；
10. 规则删除和编辑不破坏历史版本及历史结果；
11. Provider 不可用时不创建伪成功结果，保留最近成功结果并将运行状态标为降级。

## 14. 测试与浏览器验收

### 14.1 自动化测试

后端：

- 跨十张消息表全局时间排序和会话分组；
- 来源指纹幂等、增量消息触发新版本、失败不覆盖成功结果；
- 客户单聊/客户群聊快照；
- 会话分析和智能分析 JSON 严格解析；
- 证据消息越界拒绝；
- 规则版本不可变、启用上限、软删除和范围校验；
- tenant/corp/员工范围、详情、导出和规则写权限；
- MariaDB JSON、唯一键、分页、筛选和迁移 up/down；
- Provider 不可用、单会话失败、批次部分成功和积压状态。

前端：

- 两个路由渲染独立页面，不再渲染通用三卡片壳；
- 输入阶段不请求，查询/回车/重置/刷新请求正确；
- URL 恢复、标签切换、固定分页和前进/后退；
- 中文固定列、状态文案、头像回退和长文本截断；
- 会话分析导出当前筛选；
- 分析详情两分区、证据跳转、会话跳转；
- 智能规则新增、编辑、启停、删除、校验和写失败保留草稿；
- 整行详情与内部按钮隔离；
- 遮罩、关闭、`Escape` 和焦点恢复；
- 加载、空、错误、降级和无权限状态。

### 14.2 浏览器必须自行点击

两个路由分别在 `2560×1440` 和常规桌面宽度验证：

- 页面标题、筛选密度、表格高度、文字层级和宽屏利用率；
- 查询、回车、重置、刷新、分页及刷新后 URL 恢复；
- 会话分析筛选、导出、分析详情、客户分析/员工质检切换和会话详情跳转；
- 智能分析结果/规则切换、规则新增/编辑/启停/删除和持久化；
- 详情关闭按钮、遮罩和 `Escape`；
- Provider 可用时新增一条真实会话分析结果；
- Provider 不可用时仍能读取最近成功结果并显示紧凑降级状态；
- 控制台无未解释的 error/warn，关键网络请求参数正确；
- app、MySQL、Redis 健康，现有命名卷和 D 盘持久化未受影响。

## 15. 明确不在本轮实施的内容

- 不优化情绪识别、员工评分和沟通关键词三个菜单；
- 不提供页面内“立即调用模型”按钮，避免重复点击产生不可控费用；
- 不改全局侧边栏和顶部栏；
- 不将模型结论自动写入客户标签、商机或员工绩效；
- 不实现实时逐消息分析，本轮以每日增量任务为准；
- 不把旧企业级历史摘要伪装成会话级结果。

以下内容必须在正式环境专项闭环，并记录到总进度文档，不在页面重复展示大段说明：

1. 正式企业微信会话存档 Provider 的生产凭据、范围和持续同步证据；
2. 正式 AI Provider Key、模型配额、数据处理协议、脱敏策略、保留期限和成本告警；
3. 真实生产消息进入外部模型前的企业授权、告知/同意和审计流程。

## 16. 审阅结论

建议按方案 C 实施。它保留两个现有菜单，但将数据单元从“企业级一段摘要”纠正为“可筛选、可追溯、受权限控制的会话级分析结果”，并为智能分析建立真实规则来源。

本文档审阅通过前，不开始业务代码实施。
