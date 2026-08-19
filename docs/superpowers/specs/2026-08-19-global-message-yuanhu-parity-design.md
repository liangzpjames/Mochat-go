# 全局消息圆弧 AI 对标完善设计

## 1. 文档目标

本文定义 `/chat/v2-all`“全局消息”的下一轮完善方案，覆盖界面布局、交互逻辑、前后端接口、真实数据来源、权限边界、异常状态和验收标准。本文是本轮实施的唯一设计依据。

本轮只完善“全局消息”。员工会话、客户会话、群聊会话继续复用现有组件，但不在本轮主动重构；全局导航、企业切换区、顶部搜索和现有 Banner 不改动。

## 2. 现状审计

### 2.1 已有能力

- 前端路由为 `/chat/v2-all`，已有关键词、会话对象类型、员工 ID、开始日期、结束日期、分页、刷新和详情抽屉。
- 列表接口为 `GET /dashboard/workMessage/toUsers?view=global`，详情接口为 `GET /dashboard/workMessage/detail`，员工选择数据来自 `GET /dashboard/workMessage/fromUsers`。
- 服务端从 `mc_work_message_1` 至 `mc_work_message_10` 十张归档分表读取消息，并按“员工 + 对象类型 + 对象 ID”聚合为会话。
- 会话存档支持 `external` 与 `simulated` 来源标识，查询受当前租户、企业、页面 RBAC 和员工数据范围共同约束。
- 风险行为记录表 `mochat_go_risk_records` 与超时记录表 `mochat_go_timeout_records` 已存在，可作为风险会话和超时回复的真实来源。
- 归档入库已把企业微信消息类型映射为 1 至 18 的稳定代码，具备消息类型筛选基础。

### 2.2 当前缺口

- 页面采用统计卡片 + 表格，信息架构与圆弧 AI 的“今日指标 + 状态快捷筛选 + 会话卡片”不同。
- “当前页会话”显示的是固定页大小而不是实际列表长度时容易误导；三个现有概览卡也不能反映当天运营状态。
- 没有今日客户/群聊指标和同比变化。
- 没有“超时回复、风险会话、今日新增、重点关注”快捷分类及数量。
- 没有消息类型筛选；员工筛选使用内部 ID 文本输入，对运营人员不友好。
- 没有会话关注持久化。
- 风险、超时记录虽存在，但全局消息没有把记录关联回会话。
- 当前只按文本展示消息内容，图片、语音、视频、文件、链接等类型缺少一致的可读呈现。
- 当前没有可按会话定位的真实 AI 摘要；`smart-analysis` 仅有页面级结果且 Provider 为 `none`，不能冒充会话摘要。
- 当前 `to_user_type` 只能区分员工、客户、群聊，不能可靠区分“内部群聊”和“外部客户群聊”。

## 3. 圆弧 AI 调研结论

2026-08-19 对圆弧 AI `/chat/v2-all` 的实际页面调研得到以下结构：

1. 顶部连续展示 8 个今日指标：今日新增客户、今日新增聊天会话、今日会话客户、今日聊天会话总数、今日新增群、今日新增群会话、今日总会话群、今日群会话总数；每项同时展示变化率。
2. 第二层为 5 个快捷状态：全部、超时回复、风险会话、今日新增、重点关注，并展示对应数量。
3. 状态栏右侧提供时间筛选、会话类型、消息类型下拉筛选。
4. 主体使用会话卡片，不使用传统表格；卡片展示日期、最近消息、关注入口、AI 摘要和会话参与者。
5. 时间筛选使用日期范围面板；会话类型包含外部单聊、外部群聊、内部单聊、内部群聊；消息类型覆盖文本、图片、语音、视频、文件、链接、位置、名片、日程等。
6. 页面以“快速浏览状态并进入会话”为主，筛选与阅读路径短，适合日常运营巡检。

### 3.1 借鉴矩阵

| 圆弧能力 | 本系统决策 | 真实数据来源 | 说明 |
| --- | --- | --- | --- |
| 8 个今日指标 | 借鉴 | 消息十分表、`mc_work_contact`、`mc_work_contact_employee`、`mc_work_room` | 昨日为 0 时变化率返回 `null`，不伪造 `0%` |
| 5 个快捷状态 | 借鉴 | 归档消息、风险记录、超时记录、个人关注表 | 状态数量和列表使用同一权限范围 |
| 卡片式会话 | 借鉴 | 当前全局会话列表接口 | 保留服务端分页，不采用无限滚动 |
| 时间范围筛选 | 借鉴 | `msg_data_time` | URL 持久化，使用 `[startAt, endAt + 1 day)` |
| 会话类型筛选 | 调整后借鉴 | `to_user_type` | 展示内部单聊、客户单聊、客户群聊；内部群聊显示“未接入” |
| 消息类型筛选 | 借鉴 | `msg_type` | 使用归档同步已有 1–18 类型映射 |
| 重点关注 | 借鉴 | 新增个人关注表 | 按当前登录用户保存，不影响其他管理员 |
| 风险/超时标记 | 借鉴 | 现有风险、超时记录表 | 优先按消息 ID 回溯会话，兼容规范化 `conversation_id` |
| AI 会话摘要 | 暂不接入 | 当前无会话级真实结果 | 卡片隐藏摘要区，页面集中告警，不重复展示占位文字 |
| 内部群聊 | 暂不接入 | 当前归档模型无可靠子类型 | 筛选项禁用并显示缺口原因 |
| 横向超宽画布 | 不借鉴 | 不适用 | 本系统必须适配当前 931px 实际窗口，不产生页面级横向滚动 |

## 4. 方案比较与决策

### 4.1 方案 A：只改前端外观

继续使用当前列表响应，把表格换成卡片，不增加后端能力。优点是改动小；缺点是指标、风险、超时、重点关注和消息类型均无法真实工作，只能隐藏或伪造，不满足“功能完善”和真实数据要求。

### 4.2 方案 B：在现有归档查询上增量补齐（采用）

保留十张消息分表作为事实源，扩展现有筛选，新增独立概览接口和个人关注存储；风险、超时只消费已有记录。优点是兼容当前 API、数据权限和归档来源，历史数据立即可用，实施边界清晰；代价是概览查询仍需跨十张分表聚合，需要补充索引和性能门禁。

### 4.3 方案 C：新增完整会话投影表

所有归档消息写入后同步维护会话投影，概览与列表只读投影。长期性能最好，但需要历史回填、断点恢复、双写一致性和重建工具，显著扩大本轮范围。现阶段消息查询已有稳定的分表聚合与 EXPLAIN 测试，不应提前引入第二套事实状态。

### 4.4 决策

采用方案 B。若压测中企业单日消息量 100 万时概览接口 p95 超过 800ms，后续再单独立项引入可重建会话投影；本轮不预建空投影系统。

## 5. 页面信息架构

```text
现有顶部导航与左侧菜单（不改）
┌─────────────────────────────────────────────────────┐
│ 全局消息                                    刷新消息 │
│ 当前企业内、当前权限范围的会话存档                    │
├────────── 8 个今日指标，桌面 4×2 / 窄屏 2×4 ─────────┤
│ 新增客户 │ 新增客户会话 │ 会话客户 │ 客户消息总数     │
│ 新增群   │ 新增群会话   │ 会话群   │ 群消息总数       │
├─────────────────────────────────────────────────────┤
│ 全部 N │ 超时 N │ 风险 N │ 今日新增 N │ 重点关注 N      │
├─────────────────────────────────────────────────────┤
│ 关键词 │ 员工多选 │ 时间范围 │ 会话类型 │ 消息类型 │ 重置 │
├─────────────────────────────────────────────────────┤
│ 能力缺口告警：只在确有缺口时出现                       │
├────────────── 会话卡片网格 3 / 2 / 1 列 ─────────────┤
│ 最近消息、人员、类型、时间、风险/超时、关注、来源       │
├─────────────────────────────────────────────────────┤
│ 共 N 条                         分页                   │
└─────────────────────────────────────────────────────┘
点击卡片 → 右侧会话详情抽屉
```

### 5.1 今日指标

- 指标是当前企业、当前员工数据权限范围内的当天数据，不受页面关键词、消息类型和快捷状态影响。
- 时区固定使用 `Asia/Shanghai`，当天区间为 `[00:00:00, 次日 00:00:00)`。
- “新增客户/新增群”读取联系人或群创建时间；“新增会话”读取某会话的首条归档消息时间；“会话客户/会话群”读取当天有消息的去重对象；“消息总数”读取当天归档消息条数。
- 变化率为 `(今日 - 昨日) / 昨日 × 100%`。昨日为 0 时返回 `null`，界面显示 `—`，提示“昨日无基数”。
- 指标查询失败或字段不可用时返回 `value: null` 和原因，界面显示告警，不以 0 代替。

### 5.2 快捷状态

- `all`：符合普通筛选条件的全部会话。
- `timeout`：存在当前企业超时记录，且能通过 `trigger_message_id` 或规范化 `conversation_id` 关联到会话。
- `risk`：存在当前企业风险记录，且能通过 `message_id` 或规范化 `conversation_id` 关联到会话。
- `today`：会话首条归档消息发生在今天。
- `focused`：当前登录用户已关注的会话。
- 状态数量受关键词、员工、日期、会话类型、消息类型和数据权限约束，但计算时不应用当前 `bucket`，避免选中后数量互相污染。
- 风险或超时能力不可用时，对应数量返回 `null`，快捷项禁用并提示原因。

### 5.3 筛选区

- 关键词搜索员工名、会话对象名、发送者名和归档消息文本。
- 员工筛选改为可搜索多选控件，数据来自 `fromUsers`，URL 仍保存重复的 `employeeIds` 参数。
- 日期选择为成对范围；只填一端、开始晚于结束时在本地阻止查询。
- 会话类型映射：`employee`=内部单聊、`customer`=客户单聊、`room`=客户群聊。内部群聊只显示禁用选项和能力说明。
- 消息类型为多选，前后端使用稳定字符串枚举，服务端映射到现有数字代码；未知类型归入 `unsupported`。
- 点击“查询”或在关键词框按 Enter 后提交；普通输入不自动请求，避免复杂筛选频繁刷新。
- 所有已提交筛选、快捷状态、页码和页大小写入 URL；刷新、前进、后退可恢复。

### 5.4 会话卡片

每张卡片必须展示：

- 最近消息时间；
- 员工头像与名称、会话对象头像与名称；
- 会话类型；
- 最近消息类型图标和可读预览；
- 消息总数；
- 风险、超时标签及真实数量（无记录则不展示）；
- 关注按钮；
- 归档来源标记：真实企业微信存档显示“企业微信”，模拟数据显示“演示数据”。

整张卡片可点击打开详情；关注按钮阻止卡片点击冒泡。关注请求采用乐观更新，失败时回滚并提示。卡片不显示重复的“AI 摘要未生成”文本；只有 `capabilities.aiSummary.available=true` 且 `aiSummary` 非空时才展示摘要。

### 5.5 详情抽屉

- 抽屉宽度桌面端 560px，窄屏占满视口。
- 抽屉标题展示员工、对象、类型、消息总数、风险/超时数量和关注状态。
- 消息按时间升序展示，出站在右、入站在左；当前后端仍读取最近 200 条，截断时显示真实总数和“仅展示最近 200 条”。
- 文本直接展示；图片、语音、视频、文件、链接、位置、名片等使用独立渲染器。媒体能力不可用时显示类型、文件名或说明，不暴露原始 JSON。
- Escape、遮罩点击和“关闭”按钮均可关闭；打开后焦点进入抽屉，关闭后回到原卡片。
- 详情请求继续由服务端重新校验企业、RBAC、员工范围和归档来源，客户端传入的会话 ID 不作为权限依据。

### 5.6 响应式规则

- 内容区宽度大于 1180px：指标 4 列、会话卡片 3 列。
- 内容区宽度 720–1180px：指标 2 列、会话卡片 2 列。
- 内容区宽度小于 720px：指标 1 列或横向卡片滚动二选一，本设计固定为 1 列；会话卡片 1 列；筛选控件纵向排列。
- 页面本体不得产生横向滚动；消息类型多选、状态标签和分页允许自身换行。
- hover 不使用缩放和模糊滤镜，避免按钮与分页文字再次出现发虚。

## 6. 前端状态与接口契约

### 6.1 URL 查询模型

```ts
type ConversationBucket = 'all' | 'timeout' | 'risk' | 'today' | 'focused';
type ConversationTargetType = 'employee' | 'customer' | 'room';
type ConversationMessageType =
  | 'text' | 'image' | 'voice' | 'video' | 'file' | 'link'
  | 'location' | 'emotion' | 'mixed' | 'markdown'
  | 'meeting_voice_call' | 'voip_doc_share' | 'docmsg' | 'calendar'
  | 'vote' | 'collect' | 'redpacket' | 'card' | 'unsupported';

type ConversationSearch = {
  keyword: string;
  conversationType: '' | ConversationTargetType;
  messageTypes: readonly ConversationMessageType[];
  employeeIds: readonly string[];
  startAt: string;
  endAt: string;
  bucket: ConversationBucket;
  page: number;
  pageSize: number;
};
```

### 6.2 列表接口

保留 `GET /dashboard/workMessage/toUsers?view=global`，新增 `messageTypes`、`bucket` 参数并扩展返回项，避免破坏员工、客户、群聊和导出页面。

```json
{
  "list": [
    {
      "id": "msg:archive-31",
      "conversationId": "9:1:31",
      "employeeId": 9,
      "employeeName": "张伟",
      "employeeAvatar": "",
      "targetType": "customer",
      "targetId": 31,
      "targetName": "陈晓明",
      "targetAvatar": "",
      "firstMessageAt": "2026-08-08 19:07:17",
      "lastMessage": "演示时间定在周四上午十点可以吗？",
      "lastMessageType": "text",
      "lastDirection": "inbound",
      "sentAt": "2026-08-15 19:24:17",
      "messageTotal": 8,
      "riskCount": 1,
      "timeoutCount": 0,
      "focused": true,
      "aiSummary": null,
      "archiveSource": "external",
      "archiveSourceId": "wecom"
    }
  ],
  "total": 16,
  "page": 1,
  "pageSize": 20
}
```

`conversationId` 是服务端生成的企业内不透明 ID，当前格式为 `employeeId:toUserType:targetId`。客户端只能回传，不能拆解或自行构造权限。

### 6.3 概览接口

新增 `GET /dashboard/workMessage/globalOverview`。它接收与列表相同的普通筛选参数，但不接收 `bucket`、`page`、`pageSize`。这些筛选只作用于 `buckets`；`metrics` 始终按“当天 + 当前权限范围”计算，避免把关键词筛选误解为企业今日经营指标。

```json
{
  "timezone": "Asia/Shanghai",
  "generatedAt": "2026-08-19T12:00:00+08:00",
  "metrics": {
    "newCustomers": {"value": 7, "changeRate": 0.4, "status": "available"},
    "newCustomerConversations": {"value": 5, "changeRate": null, "status": "available"},
    "activeCustomers": {"value": 12, "changeRate": -0.08, "status": "available"},
    "customerMessages": {"value": 86, "changeRate": 0.12, "status": "available"},
    "newRooms": {"value": 2, "changeRate": 0, "status": "available"},
    "newRoomConversations": {"value": 1, "changeRate": 0, "status": "available"},
    "activeRooms": {"value": 4, "changeRate": 0.33, "status": "available"},
    "roomMessages": {"value": 23, "changeRate": 0.15, "status": "available"}
  },
  "buckets": {"all": 16, "timeout": 2, "risk": 3, "today": 1, "focused": 4},
  "capabilities": {
    "archive": {"available": true, "reason": ""},
    "riskLinkage": {"available": true, "reason": ""},
    "timeoutLinkage": {"available": true, "reason": ""},
    "focus": {"available": true, "reason": ""},
    "internalRoom": {"available": false, "reason": "当前归档数据没有内部群聊子类型"},
    "aiSummary": {"available": false, "reason": "尚未接入会话级 AI 摘要 Provider"}
  }
}
```

不可用指标使用 `{"value": null, "changeRate": null, "status": "unavailable", "reason": "..."}`，不得返回伪造的 0。

### 6.4 关注接口

- `PUT /dashboard/workMessage/focus`，请求体 `{"conversationId":"9:1:31"}`，幂等地关注。
- `DELETE /dashboard/workMessage/focus?conversationId=9%3A1%3A31`，幂等地取消关注。
- 两个接口使用当前登录用户 ID，忽略客户端传入的用户、租户或企业字段。
- 关注前先解析会话 ID，再执行与列表相同的企业、归档权限和员工范围校验。

### 6.5 详情接口

`GET /dashboard/workMessage/detail` 保留现有 `id` 参数以兼容旧页面，新增优先参数 `conversationId`。全局消息卡片使用 `conversationId`，员工会话、轨迹和导出暂不强制迁移。

## 7. 后端数据流向

### 7.1 归档主链路

```text
企业微信会话存档 / 演示归档源
  → WorkMessageArchiveSyncCron 拉取 seq
  → UpsertWorkMessageArchive 写入 mc_work_message_1..10
  → mochat_go_archive_message_sources 记录 external/simulated 来源
  → 全局消息列表、概览、详情按当前有效来源读取
  → Handler 应用 tenant + corp + RBAC + employee scope
  → API 返回真实数据及 capability 状态
```

### 7.2 列表与筛选

```text
URL 筛选
  → 前端序列化重复 employeeIds/messageTypes
  → Handler 校验日期、页码、枚举并和数据权限求交集
  → Store 在每张消息分表先下推 corp、员工、时间、对象类型、消息类型
  → UNION ALL 后按 employee_id + to_user_type + to_user_id 分组
  → 选择最新消息并聚合 first/last/messageTotal
  → 关联风险、超时、当前用户关注
  → 服务端分页返回
```

### 7.3 风险与超时

- 风险来源仅为 `mochat_go_risk_records`；超时来源仅为 `mochat_go_timeout_records`。
- 关联顺序为：先使用记录中的规范化 `conversation_id`；不匹配时用 `message_id` 或 `trigger_message_id` 在当前归档来源内找到消息，再推导会话 ID。
- 关联必须包含 tenant、corp 和员工数据范围，禁止只按字符串 ID 跨企业连接。
- 全局消息不负责生成风险和超时记录，也不把没有记录的会话判断为风险或超时；生产职责继续属于现有风险/超时 Provider。

### 7.4 今日指标

- 客户新增数：`mc_work_contact.created_at` 在当天，关联 `mc_work_contact_employee` 后按权限内联系人去重。
- 群新增数：`mc_work_room.create_time/created_at` 在当天，并按群主或可见员工关系限制范围。
- 会话与消息指标：在十张消息分表内按 `to_user_type`、`msg_data_time` 和员工权限聚合。
- 新增会话以该会话的最早归档消息时间为准，不使用联系人创建时间替代。
- 所有今日指标都分别计算今日和昨日，服务层统一生成变化率。

### 7.5 个人关注

新增 `mochat_go_work_message_focus`：

```sql
CREATE TABLE `mochat_go_work_message_focus` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `tenant_id` bigint unsigned NOT NULL,
  `corp_id` bigint unsigned NOT NULL,
  `user_id` bigint unsigned NOT NULL,
  `work_employee_id` int NOT NULL,
  `to_user_type` tinyint NOT NULL,
  `to_user_id` int NOT NULL,
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_work_message_focus_subject` (`tenant_id`,`corp_id`,`user_id`,`work_employee_id`,`to_user_type`,`to_user_id`),
  KEY `idx_work_message_focus_list` (`tenant_id`,`corp_id`,`user_id`,`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

不存储员工名、客户名、最后消息等可由事实表获取的数据，避免关注记录变成第二份会话事实源。

## 8. 权限、安全与数据真实性

- 所有接口从登录 Principal 解析 tenant、corp、user，不接受客户端覆盖。
- 列表、概览、详情、关注使用同一个页面权限键和同一员工范围计算函数。
- 归档未授权返回现有 `40301 archive not authorized`，前端继续展示开通引导。
- 详情不存在、越权或来源不匹配统一返回 404，避免探测其他员工或企业的会话。
- `simulated` 数据必须在卡片和详情显式标为“演示数据”；导出和指标不得把演示数据标成企业微信真实数据。
- 前端不渲染归档内容中的 HTML；文本转义，媒体 URL 必须通过受控媒体接口或安全 URL 校验。
- AI 摘要、内部群聊、媒体预览等缺口通过 capability 返回并集中告警；没有对应数据时不显示 0、不生成占位结论。

## 9. 错误与空状态

- 首次加载：保留页面骨架和指标骨架，避免整页跳动。
- 列表刷新：保留旧列表并在刷新按钮显示“刷新中…”。
- 概览失败但列表成功：指标区显示局部错误和重试，列表仍可使用。
- 列表失败：使用统一 `PageState`；概览不替代列表错误。
- 当前页越界：提供返回第一页按钮。
- 筛选无结果：展示“清除部分筛选条件”，不展示空卡片。
- 关注失败：回滚乐观状态并用页面消息提示。
- 风险/超时不可用：快捷项禁用，数量显示 `—`，页面告警列出缺口。
- AI 摘要不可用：卡片不保留摘要占位高度，只在能力告警中说明一次。

## 10. 性能设计

- 为十张消息分表补充以 `corp_id` 开头、覆盖对象类型、时间、员工和对象 ID 的组合索引；迁移逐表执行，避免单次大事务。
- 所有分表查询先下推企业、归档来源、员工范围、时间、对象类型和消息类型，再执行 `UNION ALL`。
- 列表页大小默认 20，最大 100；详情最多读取最近 200 条。
- 概览与列表使用独立 React Query 缓存；翻页只刷新列表，不重复请求今日指标，筛选变化才同时失效。
- 概览接口目标：10 万归档消息/企业时 p95 小于 500ms；100 万时 p95 小于 800ms。超过阈值时记录慢查询并另行评估会话投影，不在本轮静默降级为缓存旧值。

## 11. 测试策略

### 11.1 后端

- Handler 单测覆盖枚举、重复参数、日期半开区间、页大小上限、非法会话 ID、权限交集、归档未授权和 capability 降级。
- Store 集成测试覆盖十张分表、消息类型、今日/昨日指标、风险/超时关联、个人关注隔离、企业隔离、员工范围隔离和模拟/真实来源隔离。
- EXPLAIN 测试要求每张分表使用以 `corp_id` 开头的索引，禁止大规模无索引扫描。

### 11.2 前端

- API 合同测试覆盖 `null` 指标、能力缺口、重复 `messageTypes`、关注 PUT/DELETE 和详情 `conversationId`。
- 页面测试覆盖 8 个指标、5 个快捷状态、员工多选、消息类型、URL 恢复、卡片点击、关注回滚、详情关闭和局部错误。
- 消息渲染器测试覆盖 1–18 类型和 unknown，不渲染原始 HTML。
- 样式合同测试覆盖 4/2/1 列响应式、无页面级横向滚动、hover 无 transform/filter。

### 11.3 浏览器验收

- 在 1440px 和当前 931px 实际窗口分别验收。
- 对照圆弧页面检查指标层、快捷状态、筛选层、卡片层和详情层的信息顺序。
- 验证导航与顶部 Banner 无变化。
- 验证刷新、前进、后退、分页、关注、详情、空状态、403 和归档未授权。
- 使用真实企业微信来源和演示来源各验收一次，来源标记必须准确。

## 12. 验收标准

1. `/chat/v2-all` 使用“8 指标 + 5 状态 + 筛选 + 卡片 + 详情抽屉”结构，整体信息层级与圆弧 AI 对应。
2. 当前 931px 窗口无页面级横向滚动，文字和分页 hover 不模糊。
3. 8 个指标、5 个状态数量和卡片字段全部来自系统真实数据；不可用字段显示 `—` 和原因，不以 0 冒充。
4. 关键词、员工、日期、会话类型、消息类型、状态、分页全部可用并写入 URL。
5. 风险与超时只由现有记录驱动；重点关注按当前用户持久化且跨用户隔离。
6. 会话详情可阅读最近 200 条消息，支持所有已归档消息类型的安全可读降级。
7. 企业、员工范围、归档来源和页面权限在列表、概览、详情、关注接口中一致。
8. 会话级 AI 摘要和内部群聊在数据未接入前明确告警，页面不展示重复空文案。
9. 前端类型检查、相关单测、后端单测与集成测试、生产构建、Docker 部署和浏览器验收全部通过。

## 13. 明确不在本轮实施的内容

- 不修改全局导航、顶部 Banner、企业切换与登录体系。
- 不新建会话投影表，不重写现有十张归档分表。
- 不在全局消息页面生成风险或超时记录，只消费现有 Provider 结果。
- 不接入或模拟会话级 AI 摘要；待 Provider 和会话级结果表具备后单独实施。
- 不伪造内部群聊类型；归档元数据补齐前只展示能力缺口。
- 不把员工会话、客户会话、群聊会话同步改成同一套卡片页面，避免扩大验收范围。
