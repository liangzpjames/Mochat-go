# 会话轨迹元呼 AI 对标改造设计

日期：2026-08-19
目标路由：`/chat/trajectory`
参考路由：`https://web-ai-analysis-work-wechat.yuanhu.com/#/chat/trajectory`
停止点：完成本设计文档，经用户审阅确认后再编写实施计划；本轮不修改业务代码。

## 1. 目标与范围

本轮只优化“会话 → 会话轨迹”页面，把当前“关键词 + 会话列表 + 消息时间线”升级为以员工和日期为主轴的会话活动工作台。目标工作流固定为：

1. 在左侧选择存档员工或通过组织架构定位员工；
2. 选择日期和会话类型；
3. 查看内部单聊、外部单聊、内部群聊、外部群聊的真实统计；
4. 在 24 小时时间轴上查看当日会话活动；
5. 点击活动卡片，在右侧抽屉查看该会话当天的完整消息。

菜单导航、应用顶部 Banner、企业切换、全局搜索和其他会话页面不在本轮修改范围内。页面不能复制参考产品的演示数字、员工、群名或能力状态；所有展示值必须来自 MoChat 的真实接口和存储。

## 2. 不可破坏的约束

1. 删除当前页面纯介绍性质的顶部卡片；刷新入口进入实际查询操作区。
2. `2560×1440` 为主验收尺寸，同时回归 `1440×900`、`1066×1272` 和 `390×844`。
3. 列表、统计、轨迹和详情使用相同企业、员工权限、归档来源和自然日口径。
4. 页面不得把“未接入”“资料缺失”“同步异常”显示成 `0` 或普通空态。
5. 内部群聊当前没有可靠 Provider 和存储口径，必须显示“未接入”，不能伪装成 0 条。
6. 外部群消息必须使用统一的有效群 ID，不能继续把 `to_user_id=0` 的消息显示成“群聊 0”。
7. 历史群消息无法识别具体发送人时显示“群成员”，不得根据正文、头像或群成员数量猜测身份。
8. 实施遵循测试先行；Docker 只重建必要的 `app` 服务，不删除 MySQL、Redis 或 D 盘命名卷。
9. 当前工作树存在其他页面的未提交改动，实施时只修改会话轨迹及必要共享数据层，不覆盖用户已有改动。

## 3. 现状基线

### 3.1 当前 MoChat 页面

`ConversationTrajectoryPage` 当前使用：

- `GET /dashboard/workMessage/toUsers?view=global` 查询最多 50 个会话摘要；
- `GET /dashboard/workMessage/detail?id=<archive-message-id>` 以某条归档消息为锚点，读取该会话最近 200 条消息；
- URL 只保存 `keyword` 和 `conversationId`；
- 页面主体是左侧会话列表和右侧纵向消息时间线；
- 页面最大宽度为 1320px，在 27 寸宽屏上留下大量空白。

当前实现存在以下结构性问题：

1. 缺少员工目录，无法回答“某位员工某一天与谁沟通过”。
2. 缺少日期主轴和小时轨迹，消息只能按会话逐条阅读。
3. 缺少四类会话统计，无法快速判断当日沟通分布。
4. 列表固定请求第 1 页，超过 50 个会话后没有分页或明确截断提示。
5. 详情使用归档消息 ID，而员工、日期和目标对象没有形成稳定的 URL 状态。
6. 详情读取“最近 200 条”而不是所选日期，列表和详情时间口径不一致。
7. 页面只有两个简单组件测试，未覆盖 URL 恢复、日期、统计、权限、抽屉和宽屏布局。

### 3.2 可复用的现有能力

当前工作树已经形成员工会话专用数据能力，可供会话轨迹复用：

- `GET /dashboard/workMessage/staffDirectory`：员工、部门树、存档状态、分页、能力限制；
- `GET /dashboard/workMessage/staffDetail`：稳定 `conversationId`、日期过滤、50 条游标分页和真实消息内容；
- `ConversationMessageContent`：文本、图片、语音、视频、文件、链接的类型化展示；
- `workMessageFilteredUnionSQLWithArchiveSourceState`：10 张归档消息分表与有效归档来源选择；
- 会话存档授权检查、企业范围和部门/员工数据权限。

这些能力应复用接口和数据模型，但会话轨迹保持独立页面组件，避免把员工会话三栏工作台的交互强行耦合进轨迹页面。

### 3.3 元呼 AI 参考页重新采样

2026-08-19 在已登录参考页采样到的结构为：

1. 左列包含员工搜索、“存档员工 / 组织架构”切换、员工列表和固定分页。
2. 主区顶部包含所选员工、前一天/后一天、日期选择和会话类型筛选。
3. 主区展示内部单聊、外部单聊、内部群聊、外部群聊四张分类统计卡，每张卡含沟通对象数和消息数。
4. 主体是按小时排列的纵向时间网格，会话活动卡片落在对应小时行。
5. 点击活动卡片后打开右侧大抽屉，以左右消息气泡展示会话详情；关闭按钮和遮罩可收起抽屉。
6. 参考页在 2560×1440 下左列约 320px，轨迹主区占据其余宽度；当前有数据样本为外部群聊 1 个群、1 条消息。

本方案借鉴该信息架构和视觉节奏，但不复制其横向滚动、演示头像、颜色数量或业务数据。

## 4. 方案比较

### 4.1 方案 A：员工 + 日期 + 小时轨迹工作台（采用）

左侧员工目录，右侧按日期展示分类统计和 24 小时轨迹，活动详情使用抽屉。该方案最符合参考页的信息架构，也能直接回答“谁在什么时候与什么对象沟通了多少次”。

代价是需要新增按日聚合接口，并补齐群 ID、权限和自然日口径测试。

### 4.2 方案 B：保留会话列表，只增加日期和统计

可最大限度复用当前页面，开发量较小，但核心仍是会话列表，无法形成真正的时间轨迹；宽屏利用率和参考页差异仍然明显。不采用。

### 4.3 方案 C：前端拉取全量消息后自行生成轨迹

不新增后端接口，但需要前端连续翻页并持有一天的全部消息，统计口径、权限、性能和截断都难以保证。该方案会把数据真相放在浏览器中，不采用。

## 5. 目标信息架构

### 5.1 27 寸宽屏结构

页面不再显示顶部介绍卡，主内容为连续两列工作台：

```text
┌──────────── 320px ────────────┬──────────────────── minmax(760px, 1fr) ────────────────────┐
│ 员工搜索 [查询][刷新]          │ 员工 / 日期前后切换 / 日期 / 会话类型 / 刷新               │
│ [存档员工] [组织架构]          │ ┌内部单聊┐ ┌外部单聊┐ ┌内部群聊·未接入┐ ┌外部群聊┐         │
│                                │ │对象/消息│ │对象/消息│ │    -- / --    │ │群/消息 │         │
│ 员工卡片                       │ └────────┘ └────────┘ └────────────────┘ └────────┘         │
│ 员工卡片                       │ 00:00 ─────────────────────────────────────────────────── │
│ ...                            │ 01:00 ── [客户 A · 3 条 · 01:12–01:25]                   │
│                                │ ...                                                        │
│ 共 N 位 / 页码                 │ 23:00 ── [客户群 B · 1 条 · 23:08]                       │
└────────────────────────────────┴────────────────────────────────────────────────────────────┘
                                                   右侧详情抽屉 ────────────────┐
                                                   │ 会话对象 / 日期 / 关闭      │
                                                   │ 对方消息气泡                │
                                                   │                 员工消息气泡 │
                                                   └─────────────────────────────┘
```

主工作台高度使用视口减去应用顶部区域和页面外边距。左列员工列表、右侧轨迹区和详情抽屉分别独立滚动，页面根节点不产生横向滚动。

### 5.2 员工目录列

1. 默认进入“存档员工”，只展示当前企业、当前数据权限内至少存在一条可用归档消息的员工。
2. “组织架构”展示当前可访问部门树；选中部门后列出该部门及子部门员工。
3. 员工名称输入只维护草稿，点击“查询”或按回车后才更新 URL 和请求。
4. “刷新”紧邻查询按钮，只刷新当前员工目录，不改变 URL、页码和已选员工。
5. 员工卡展示真实头像、姓名、存档状态、会话数和最近沟通时间；头像为空时使用姓名首字占位。
6. 选中员工后保留日期和会话类型，清除已打开的活动详情，便于同一天横向比较不同员工。
7. 员工目录固定每页 50 条，底部显示总数、页码、上一页和下一页。
8. 组织架构未同步时显示“组织架构未同步，仅展示可访问员工”，属于同步限制，不显示成空部门。

### 5.3 查询操作区

1. 显示所选员工头像和姓名；未选择员工时显示“请先选择员工”。
2. 日期必须进入 URL，格式为 `YYYY-MM-DD`，自然日口径固定为 `Asia/Shanghai` 的 `[00:00:00, 次日 00:00:00)`。
3. 初次访问缺少日期时，把上海时区的当天日期以 `replace` 写入 URL 后再发请求，保证刷新可复现。
4. 前一天、后一天和日期选择器都直接触发确定请求；当天时“后一天”禁用，不允许选择未来日期。
5. 会话类型选项为“全部、内部单聊、外部单聊、外部群聊”。“内部群聊”保留可见但禁用，并显示未接入原因。
6. 会话类型只过滤下方活动轨迹，四张统计卡始终展示所选员工当天的完整分类分布。
7. “刷新”只刷新当前员工、日期和会话类型的按日轨迹，不改变 URL。

### 5.4 四类统计卡

每类返回两个独立的指标值和状态：

| 分类 | 对象指标 | 消息指标 | 当前口径 |
|---|---|---|---|
| 内部单聊 | 沟通同事数 | 消息数 | `to_user_type=0` |
| 外部单聊 | 沟通客户数 | 消息数 | `to_user_type=1` |
| 内部群聊 | 内部群数 | 消息数 | 当前未接入，两个值为 `null` |
| 外部群聊 | 外部群数 | 消息数 | `to_user_type=2` 且有效群 ID 可解析 |

统计值为真实 0 时显示 `0`；状态为 `unavailable` 时显示 `--` 和原因。不能用 `messages.length`、当前视口活动数量或前端常量推算统计。

### 5.5 24 小时活动轨迹

1. 时间轴包含 `00:00` 至 `23:00` 共 24 个语义化小时行，不使用 canvas。
2. 首次进入时，如果 08:00 前没有活动，轨迹滚动容器定位到 08:00；存在更早活动时定位到最早活动，任何消息都不能因默认视口而被隐藏。
3. 服务端以“稳定会话 ID + 自然日小时”聚合活动，同一会话跨小时会形成多张活动卡；同一小时内不会重复卡片。
4. 活动卡展示对象类型、真实对象名称、消息数和该小时内首末消息时间。
5. 同一小时存在多张卡时自动换行并增高当前小时行，不允许绝对定位后重叠。
6. 活动卡整卡可点击，键盘可聚焦；hover 和选中只改变背景、边框和文字颜色，不使用位移或缩放。
7. 无活动但接口能力正常时显示完整空时间轴和“该员工当天没有已归档会话”。
8. 目标资料缺失时显示“客户资料缺失”“员工资料缺失”或“客户群资料缺失”，同时保留真实目标类型；不能编造姓名。
9. 无法解析有效目标 ID 的消息不生成伪活动卡，页面显示真实条数告警。

活动 ID 采用确定性格式：

```text
<employeeId>:<targetType>:<effectiveTargetId>@<YYYY-MM-DD>T<HH>
```

详情锚点继续使用稳定会话 ID：

```text
<employeeId>:<targetType>:<effectiveTargetId>
```

### 5.6 会话详情抽屉

1. 点击活动卡打开右侧抽屉，抽屉头部展示会话对象、员工、所选日期和对象类型。
2. 消息只读取所选日期，按时间正序显示；初次加载最近 50 条，使用“加载更早消息”向前追加并按稳定消息 ID 去重。
3. 员工发送在右侧，对方发送在左侧；日期分隔、时间、姓名和头像均来自真实响应。
4. 消息内容复用 `ConversationMessageContent`，不在轨迹页面复制媒体解析逻辑。
5. 群聊历史消息无法确定具体发送人时显示“群成员”，并在抽屉内显示紧凑能力限制提示。
6. 抽屉可通过关闭按钮、遮罩和 `Escape` 关闭；关闭后焦点返回原活动卡。
7. 抽屉开关属于临时视觉状态，不写入 URL；当前活动的 `conversationId` 和 `eventHour` 写入 URL，使刷新后可以恢复同一天的详情。

## 6. 响应式规则

### 6.1 1600px 及以上

显示 `320px minmax(760px, 1fr)` 两列。四张统计卡单行排列，轨迹主列吃满剩余空间。详情抽屉宽度为 `min(880px, 72vw)`。

### 6.2 1200–1599px

员工目录缩至 288px，统计卡使用两行两列，轨迹仍为主列。详情抽屉宽度为 `min(760px, 78vw)`，不产生横向滚动。

### 6.3 769–1199px

员工目录改为左侧抽屉，主区默认占满。主区提供“选择员工”按钮；员工抽屉和详情抽屉都支持关闭按钮、遮罩和 `Escape`，同一时间只打开一个覆盖层。

### 6.4 768px 及以下

采用“员工列表 → 日轨迹 → 会话详情”的分步单列。统计卡两列排列，轨迹活动卡在小时标签右侧单列堆叠。详情使用全屏抽屉，返回按钮恢复轨迹滚动位置。

## 7. URL 状态

业务状态写入 URL，输入草稿和抽屉可见性不写 URL。

| 参数 | 含义 | 默认值 |
|---|---|---|
| `directoryMode` | `archived` / `organization` | `archived` |
| `employeeKeyword` | 已提交的员工关键词 | 空 |
| `departmentId` | 组织架构部门 ID | 空 |
| `employeePage` | 员工目录页码 | `1` |
| `employeeId` | 当前员工 ID | 空 |
| `date` | 上海时区自然日 | 当天，规范化后显式写入 |
| `conversationType` | `all` / `employee` / `customer` / `room` | `all` |
| `conversationId` | 当前详情的稳定会话 ID | 空 |
| `eventHour` | 当前活动小时 `00`–`23` | 空 |

级联规则：

1. 切换目录模式、部门或员工关键词时回到员工第 1 页。
2. 选择新员工时保留日期和会话类型，清除 `conversationId` 与 `eventHour`。
3. 切换日期或会话类型时关闭详情并清除 `conversationId` 与 `eventHour`。
4. `conversationId` 必须属于当前员工、日期和权限范围；不匹配时使用 `replace` 清除，不暴露目标是否存在。
5. 非法日期、未来日期、非法枚举、非正页码和无权限员工在发请求前完成规范化。
6. 浏览器前进、后退和刷新必须恢复员工、日期、会话类型和合法详情锚点。

## 8. 真实数据来源矩阵

| 页面字段/能力 | 前端来源 | API | 服务/存储来源 | 口径与空值语义 | 权限范围 |
|---|---|---|---|---|---|
| 员工目录、姓名、头像、状态 | `staffDirectory` | `/workMessage/staffDirectory` | `mc_work_employee` | 员工为空为真实空态 | 当前企业、可访问员工 |
| 部门树 | `staffDirectory` | 同上 | `mc_work_department`、`mc_work_employee_department` | 未同步为 limitation，不是空组织 | 同上 |
| 存档员工数量与状态 | `staffDirectory` | 同上 | 有效归档来源的 10 张 `mc_work_message_*` 分表 | 至少一条可用归档消息才算存档员工 | 同上 |
| 内部单聊对象数/消息数 | `trajectoryDay.metrics` | `/workMessage/trajectoryDay` | 归档消息分表，`to_user_type=0` | 可用且无记录时为 0 | 所选可访问员工、所选日期 |
| 外部单聊对象数/消息数 | 同上 | 同上 | 归档消息分表，`to_user_type=1` | 同上 | 同上 |
| 内部群聊 | `trajectoryDay.metrics` | 同上 | 当前无可靠 Provider/存储 | `status=unavailable`、值为 `null` | 不查询 |
| 外部群对象数/消息数 | `trajectoryDay.metrics` | 同上 | 归档消息分表 + 有效群 ID | 可用且无记录时为 0；无效群另行告警 | 同上 |
| 小时活动卡 | `trajectoryDay.events` | 同上 | 归档消息按会话 ID、日期小时聚合 | 一会话一小时一张卡 | 同上 |
| 同事名称 | `trajectoryDay.events` | 同上 | `mc_work_employee` | 记录缺失为“员工资料缺失” | 同企业 |
| 客户名称 | `trajectoryDay.events` | 同上 | `mc_work_contact` | 记录缺失为“客户资料缺失” | 同企业和员工关系范围 |
| 客户群名称 | `trajectoryDay.events` | 同上 | `mc_work_room` | 空名称为“未命名客户群”；记录缺失为资料缺失 | 同企业和可见员工会话 |
| 详情消息 | `staffDetail` | `/workMessage/staffDetail` | 有效归档来源分表 | 所选自然日，50 条游标向前 | 同员工、同企业、同日期 |
| 消息方向 | `staffDetail.messages.direction` | 同上 | `is_current_user` / `sender_type` | 只按持久化字段，不按文案猜测 | 同上 |
| 消息类型和内容 | `ConversationMessageContent` | 同上 | `msg_type`、归档内容 | 缺媒体字段显示“不支持预览” | 同上 |
| 群消息具体发送人 | `staffDetail.capabilities` | 同上 | 发送人身份旁表或现有归档字段 | 历史缺失时显示“群成员”与 limitation | 同上 |
| 归档授权状态 | 查询错误/能力 | 三个接口 | 会话存档配置、归档来源注册表 | 40301 为未开通，不是普通无数据 | 当前企业 |
| 无法关联目标的消息数 | `trajectoryDay.limitations` | `/workMessage/trajectoryDay` | 分表中的无效目标 ID | 显示条数告警，不生成假对象 | 同员工、同日期 |

## 9. 有效目标 ID 与分类口径

### 9.1 稳定会话 ID

内部单聊和外部单聊使用 `to_user_id`。外部群聊的有效群 ID 使用共享规则：

```sql
CASE
  WHEN wm.to_user_type = 2
  THEN COALESCE(NULLIF(wm.to_user_id, 0), NULLIF(wm.room_id, 0), 0)
  ELSE wm.to_user_id
END
```

规则要求：

1. `to_user_id` 有值时优先；
2. `to_user_id=0` 且 `room_id` 有值时使用 `room_id`；
3. 两者都为 0 时计入 `unmatchedTargetMessages`，不生成“群聊 0”；
4. 有效群 ID 仍须校验属于当前企业；
5. `staffDetail` 与 `trajectoryDay` 使用同一函数，避免轨迹能打开但详情查不到。

### 9.2 自然日与小时

- 业务时区固定为 `Asia/Shanghai`；
- 日期范围为开始时间包含、次日开始时间不包含；
- 小时使用持久化后的业务时间，不在浏览器中二次转换；
- 夏令时不适用于上海时区，但测试仍需验证跨月、跨年和闰日；
- 统计 SQL 和活动 SQL必须接收同一组开始、结束参数。

## 10. 后端接口设计

### 10.1 复用员工目录

`GET /dashboard/workMessage/staffDirectory`

请求继续使用：

```text
mode=archived|all
keyword
departmentId
page
pageSize=50
```

“存档员工”使用 `mode=archived`；“组织架构”使用 `mode=all` 并结合 `departmentId`。响应沿用现有 `departments`、`employees`、`counts`、`limitations` 和 `capabilities`。

### 10.2 新增按日轨迹接口

`GET /dashboard/workMessage/trajectoryDay`

请求：

```text
employeeId=<positive int>
date=YYYY-MM-DD
conversationType=all|employee|customer|room
```

响应：

```text
employee { id, name, avatar }
date
timezone: "Asia/Shanghai"
metrics {
  internalSingle { status, subjectTotal, messageTotal, reason? }
  externalSingle { status, subjectTotal, messageTotal, reason? }
  internalGroup  { status: "unavailable", subjectTotal: null, messageTotal: null, reason }
  externalGroup  { status, subjectTotal, messageTotal, reason? }
}
events[] {
  id, conversationId, hour,
  targetType, targetId, targetName, targetAvatar, targetStatus,
  messageTotal, firstMessageAt, lastMessageAt
}
unmatchedTargetMessages
limitations[]
capabilities[]
```

接口规则：

1. `metrics` 始终返回完整四类统计，不受 `conversationType` 过滤影响。
2. `events` 才应用 `conversationType`。
3. `events` 按小时、首条时间、稳定会话 ID 排序；数组为空时必须返回 `[]`。
4. 员工无权限、不属于企业或不存在统一返回 404。
5. 会话存档未开通继续返回现有 40301。
6. `unmatchedTargetMessages > 0` 时返回 limitation，不能把缺失数据混入正常统计对象数。

### 10.3 复用员工会话详情

`GET /dashboard/workMessage/staffDetail`

轨迹抽屉请求：

```text
conversationId=<employeeId:targetType:effectiveTargetId>
date=YYYY-MM-DD
pageSize=50
before=<optional cursor>
```

轨迹页面不使用该接口的全历史统计字段，只使用会话身份、当天消息、游标和 capabilities。实现必须让外部群的 `conversationId` 使用有效群 ID 规则。

### 10.4 权限资源

新增数据库迁移，把以下 GET 资源授予 `dashboard.chat.trajectory`：

- `/dashboard/workMessage/staffDirectory`；
- `/dashboard/workMessage/trajectoryDay`；
- `/dashboard/workMessage/staffDetail`。

旧的 `/workMessage/toUsers`、`/workMessage/detail` 在页面切换完成后不再由会话轨迹前端调用；兼容路由是否保留由其他页面的资源依赖决定，不在本轮盲目删除。

## 11. 存储与查询设计

### 11.1 单次按日聚合

新增 `WorkMessageTrajectoryStore`，入口接收企业、租户、员工、日期、类型和数据权限。存储层复用有效归档来源联合查询，不为轨迹复制 10 张分表 SQL。

查询分为两次，无 N+1：

1. 条件聚合一次得到四类对象数、消息数和无法关联目标条数；
2. 按 `effectiveTargetId + to_user_type + DATE_FORMAT(msg_data_time, '%H')` 聚合活动，得到数量和首末时间。

员工、客户和群名称在聚合后按目标类型批量查询，不能对每张活动卡单独访问数据库。

### 11.2 数据库方言与索引

1. SQL 必须在项目实际 MariaDB 版本运行，不使用仅 MySQL 8 可用且未被项目验证的窗口函数。
2. 现有全局索引 `corp_id, to_user_type, msg_data_time, work_employee_id, to_user_id, msg_type` 已覆盖主要过滤列；实施前使用真实 `EXPLAIN` 验证。
3. 如果 `employeeId + date` 查询仍扫描过多行，新增索引顺序采用 `corp_id, work_employee_id, msg_data_time, to_user_type, to_user_id`，10 张分表一致创建，并提供 down migration。
4. 不为了预期性能直接添加索引；只有 `EXPLAIN` 或集成基准证明现有索引不足时才加入迁移。

## 12. 权限与数据隔离

1. 三个接口都先校验 `dashboard.chat.trajectory` 页面权限和会话存档可用性。
2. 企业 ID 只取认证上下文，不接受查询参数覆盖。
3. 部门/员工权限用户只能看到 `DeptEmployeeIDs` 内的员工、统计、活动和详情。
4. 轨迹接口收到越权 `employeeId` 返回 404，不返回 403，以免泄露员工存在性。
5. `conversationId` 的员工部分必须等于当前选中员工，并重新经过数据权限校验。
6. 客户和群资料只能用于已经通过消息范围证明可见的目标，不能通过目标 ID 任意枚举。
7. 统计、活动和详情必须选择同一有效归档来源，禁止统计包含模拟源而详情读取外部源。

## 13. 前端组件边界

```text
ConversationTrajectoryPage
├── ConversationTrajectoryDirectory
│   ├── TrajectoryDirectoryMode
│   ├── TrajectoryEmployeeSearch
│   ├── TrajectoryDepartmentTree
│   ├── TrajectoryEmployeeList
│   └── TrajectoryEmployeePagination
├── ConversationTrajectoryWorkspace
│   ├── TrajectoryToolbar
│   ├── TrajectoryMetrics
│   └── TrajectoryTimeline
│       ├── TrajectoryHourRow
│       └── TrajectoryEventCard
└── ConversationTrajectoryDrawer
    └── ConversationMessageContent（复用）
```

组件职责：

1. 页面组件拥有 URL 解析、规范化、React Query key、级联清理和抽屉编排。
2. 目录组件只接收员工数据、加载状态和回调，不直接读 URL。
3. 工作区组件只展示工具栏、指标和活动，不自行请求后端。
4. 抽屉只处理消息分页、关闭行为和焦点恢复，不修改员工或日期。
5. TypeScript API 解析器严格校验所有指标状态、可空数字、时间、枚举、数组和稳定 ID；字段缺失时抛出“会话轨迹接口返回了无效数据”，不能默认成 0。

## 14. 查询缓存与交互编排

React Query key 固定包含企业和所有影响响应的数据状态：

```text
['corp', corpId, 'trajectory-directory', directoryMode, employeeKeyword, departmentId, employeePage]
['corp', corpId, 'trajectory-day', employeeId, date, conversationType]
['corp', corpId, 'trajectory-detail', conversationId, date]
```

规则：

1. 输入员工关键词期间不改变 query key；提交后更新 URL。
2. 轨迹刷新只 `refetch` 当前 `trajectory-day`。
3. 目录刷新只 `refetch` 当前 `trajectory-directory`。
4. 详情“加载更早消息”使用独立游标页，不改变 URL。
5. 切换员工或日期时取消或忽略旧详情响应，不能把旧消息闪现在新抽屉中。
6. 不做统计和活动的乐观更新；所有值以服务器响应为准。

## 15. 加载、空态、错误与限制

1. 未选择员工：左列保持可用，主区显示“请选择员工查看当日会话轨迹”。
2. 目录加载：只替换左列列表区域，不卸载主区。
3. 轨迹加载：保留工具栏和小时网格尺寸，指标卡显示骨架，避免布局跳动。
4. 当日无数据：四张可用指标显示真实 0，内部群显示未接入，轨迹显示完整空网格。
5. 归档未开通：显示 `ConversationArchiveState` 配置提示，不显示空统计。
6. 内部群未接入：禁用筛选项，指标显示 `--` 和原因。
7. 组织架构未同步：左列显示 limitation，仍允许使用存档员工列表。
8. 无效目标消息：显示“有 N 条消息无法关联会话对象”，不生成假卡。
9. 轨迹接口错误：只替换主区数据区，保留员工目录和当前查询条件，并提供“重新加载”。
10. 详情接口错误：错误只出现在抽屉，关闭抽屉后主轨迹不受影响。
11. 详情为空但活动存在：视为数据链路异常，显示“活动摘要与消息详情不一致”并允许重试，不能显示普通空态。

## 16. 测试设计

### 16.1 后端 handler 测试

- `employeeId`、`date`、`conversationType` 的合法与非法输入；
- 未来日期拒绝或规范化策略与前端一致；
- 页面权限、存档未开通 40301、越权员工 404；
- store 未实现时返回明确 501；
- nil 数组规范化为 `[]`，不可用指标保持 `null`；
- `metrics` 不受类型筛选影响，`events` 正确受影响。

### 16.2 存储与 MariaDB 集成测试

- 上海时区自然日开始包含、次日开始排除；
- 跨月、跨年、闰日；
- 内部单聊、外部单聊、外部群对象数和消息数；
- 内部群返回 unavailable，不伪造 0；
- 同一会话同一小时聚合、跨小时拆分；
- 活动排序稳定；
- `to_user_id=0` 使用 `room_id`，两者为 0 计入 limitation；
- 目标资料缺失的状态和名称降级；
- 有效归档来源选择；
- 部门权限、跨企业和无权限员工；
- `trajectoryDay` 和 `staffDetail` 对外部群使用同一稳定会话 ID；
- `EXPLAIN` 或索引契约验证没有退化成无界全表扫描。

### 16.3 前端 API 契约测试

- 精确序列化 `employeeId`、`date`、`conversationType`；
- 严格解析四类指标、可空值、events、limitations 和 capabilities；
- 非法 targetType、hour、status、空 ID、缺数组和错误数字全部 reject；
- `staffDetail` 的日期与游标请求不回退到旧 `/detail?id=`。

### 16.4 页面行为测试

- 初次访问写入规范日期后才请求；
- 员工关键词草稿不请求，查询和回车各只请求一次；
- 存档员工、组织架构、部门、分页和刷新；
- 选择员工保留日期/类型并清除详情；
- 前一天、后一天、日期选择、未来日期禁用；
- 类型筛选只改变 events 请求和展示，不改变四类统计；
- 真 0 与 unavailable 的不同展示；
- 24 个小时行、初始滚动定位和早间消息可见；
- 多活动卡换行不重叠；
- 整卡点击、URL 恢复、加载更早和稳定去重；
- 抽屉关闭按钮、遮罩、Escape 和焦点返回；
- 无数据、未开通、目标缺失、接口错误和摘要/详情不一致；
- 目录、轨迹、详情任一失败不会卸载其他区域。

### 16.5 样式契约与浏览器验收

- 2560×1440：连续两列填满，四张指标单行，轨迹主区无横向滚动；
- 1440×900：两列可用，指标两行，详情抽屉不遮死关闭入口；
- 1066×1272：员工目录为抽屉，主区占满；
- 390×844：员工、轨迹、详情分步单列；
- hover、选中、分页和小时文字清晰，无 transform/scale 模糊；
- 控制台无新增错误，关键请求无未解释 4xx/5xx；
- 刷新页面后 URL 状态可恢复。

## 17. 实施顺序建议

1. 先补后端按日轨迹类型、过滤器和失败测试；
2. 实现有效目标 ID 的共享函数和 MariaDB 聚合查询；
3. 注册 `/workMessage/trajectoryDay`，新增 `dashboard.chat.trajectory` 权限资源迁移；
4. 扩展前端 API 类型、序列化和严格解析器；
5. 先写页面行为测试，再拆分员工目录、统计、小时轨迹和详情抽屉；
6. 写布局契约并完成四档响应式；
7. 运行目标测试、相关全量测试、类型检查和生产构建；
8. 只重建 Docker `app`，确认 MySQL、Redis 和命名卷保持不变；
9. 在本地真实路由完成四种尺寸浏览器验收并更新进度文档。

具体文件、测试名、RED/GREEN 命令和提交边界在本设计经用户确认后写入独立实施计划。

## 18. 不在本轮实现

- 内部群聊发现、同步、命名和统计；
- 根据历史群成员关系猜测具体发送人；
- 会话导出或单会话下载；
- 风险、超时或 AI 摘要叠加到轨迹卡；
- 修改客户会话、群聊会话或员工会话的页面布局；
- 修改菜单导航、应用顶部 Banner、全局搜索或企业切换；
- 在前端生成模拟员工、模拟统计或模拟会话活动。

## 19. 完成定义

1. `/chat/trajectory` 使用员工 + 日期 + 小时轨迹工作台，不再显示旧会话列表页面和顶部介绍卡。
2. 员工目录、四类统计、小时活动和详情消息都有可追溯的真实数据来源。
3. 内部群明确显示未接入，真实 0、无数据、资料缺失、权限/同步异常可区分。
4. 外部群使用有效群 ID，不再出现“群聊 0”，轨迹与详情稳定 ID 一致。
5. 24 小时消息都可访问，默认定位 08:00 不会隐藏更早活动。
6. 2560×1440 充分利用宽屏，常规桌面和移动端无横向溢出。
7. 日期、类型、员工和详情锚点可通过 URL 恢复，刷新不丢业务状态。
8. 新行为具有先失败后通过的测试证据，相关 Go 测试、前端测试、类型检查和生产构建通过。
9. app、MySQL、Redis 健康，D 盘命名卷保持不变。
10. 完成四种尺寸浏览器验收，并在会话轨迹页面暂停等待用户检查。
