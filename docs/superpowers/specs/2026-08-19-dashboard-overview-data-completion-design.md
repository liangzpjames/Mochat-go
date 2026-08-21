# 数据概览真实数据补齐与圆弧式内容区设计

## 目标

在不改动现有菜单导航、顶部 banner、登录态和企业权限壳层的前提下，将“数据概览”内容区重构为圆弧 AI 会话式的信息架构，并把当前系统能够可靠取得的真实数据接入页面。

## 设计边界

- 保留 `dashboard-layout.tsx`、`yuanhu-navigation.ts` 以及 `DashboardOverviewPage` 顶部 banner 的现有结构和文案。
- 只重构 banner 以下的数据概览内容区及其数据契约。
- 所有数字必须来自当前企业、当前查询区间和现有数据库/Provider。
- 无数据、接口未返回或字段语义不足时显示“—”，同时展示原因；不使用 0 代替缺失值。
- 不根据 AI 自然语言摘要猜测情绪数量、风险数量等结构化指标。

## 页面结构

1. 经营概览：四个真实系统指标，保留当前企业范围和更新时间。
2. AI 洞察：五个紧凑数字控件。AI 分析次数使用成功分析记录数；情绪类数字只有在后端存在结构化字段时展示，否则显示“—”并标记“AI 摘要无结构化数字字段”。不展示长文本摘要。
3. 会话数据：圆弧式左右分栏。左侧为完整填充的客户会话/客户群控件，右侧为铺满剩余空间的七日趋势图；点击左侧类型同步切换右侧趋势。
4. 质检数据：风险行为、敏感词命中、超时预警、客户流失使用现有表或现有客户数据；缺表或无权限时逐项告警。
5. 员工会话数据排行：从会话归档按员工聚合会话数、消息数，使用真实返回记录；没有归档表时告警。
6. 员工会话轨迹一览：从会话归档返回最近会话摘要；字段不足时只展示实际字段和缺口说明。

## 数据契约

概览接口新增可选字段 `aiMetrics`、`quality`、`employeeRanking`、`trajectory`。字段缺失与 `limitations` 一起返回，前端根据 `null` 和 limitation 显示告警状态。

### 数据口径

- `aiMetrics.analysisCount`：当前企业查询区间内 `mochat_go_ai_analysis` 中 `status=succeeded` 的记录数。
- `quality.riskBehavior`：`mochat_go_risk_records.occurred_at` 区间内记录数。
- `quality.sensitiveWords`：`mc_sensitive_words_monitor.send_time`（为空时使用 `created_at`）区间内记录数。
- `quality.timeoutWarning`：`mochat_go_timeout_records.occurred_at` 区间内记录数。
- `quality.customerLoss`：沿用客户关系数据中 `mc_work_contact_employee.deleted_at` 区间内 `status IN (2,3)` 的数量。
- `employeeRanking`：按 `work_employee_id` 聚合归档消息，展示员工、会话数、消息数。
- `trajectory`：按最近消息时间返回归档中的最近会话、员工、会话类型、消息数和时间。

## 缺口分类

- 数据存在但此前未接入：风险、敏感词、超时、员工排行、会话轨迹，本次接入概览。
- 只有自然语言摘要：员工/客户负面情绪；本次保留“—”，需要后续由 AI Provider 输出结构化 JSON 字段后再展示。
- 运行环境缺表、权限不足或 Provider 不可用：保留模块并显示来源及原因，不隐藏模块、不造数。

## 验收标准

- 菜单导航和顶部 banner 与实施前保持一致。
- 数据概览内容区按上述层级展示，无旧的长文本 AI 洞察、无趋势明细表、无重复分页控件。
- 会话数据左侧填满控件区，右侧趋势图填满剩余区域，日期标签完整可读。
- 当前 Docker 环境可以看到真实数据：经营概览、会话、AI 分析次数、质检记录和员工排行至少有已确认的数据；无法可靠计算的情绪指标显示“—”并有告警。
- 前端单测、后端测试、生产构建和本地页面检查通过。
