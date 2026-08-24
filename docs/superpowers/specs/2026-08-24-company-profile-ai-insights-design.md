# 企业资料权限与三项 AI 洞察优化设计

## 1. 目标与边界

本轮交付包含三项彼此可独立验收、但共享 Dashboard RBAC 与会话分析数据链路的工作：

1. 修复 `/company-setting/website`“唯一企业资料”在单企业模式下对已授权普通用户不可见、不可直达的问题。
2. 在不关闭 ESLint 规则、不缩小范围、不新增大范围忽略的前提下，将 `@mochat/dashboard` 全量 lint 从基线 86 errors / 0 warnings 清零。
3. 将 `/ai-insight/emotion`、`/ai-insight/employee-score`、`/ai-insight/communication-keyword` 从通用受限态壳页改造成读取真实会话分析持久化结果的专用工作台。

不在本轮范围内：恢复 Phase 7、接通真实企业微信会话存档、启用自动日分析或启动即分析、在页面打开时调用 AI Provider、伪造归档消息或分析结果、改动总进度台账、合入或推送新开发分支。

## 2. 发布基线与现场保护

- 2026-08-24 已在线核实原 `origin/main` 为 `de902ef1797ae1dea2933ee9be2838253b34248e`，本地 `main` 为 `756ac3765bdb358ee125689c8c78cef03423c13f`，远端是本地祖先，领先 141、落后 0。
- 已使用普通 push 发布 `main`，随后 `git ls-remote origin refs/heads/main` 返回 `756ac3765bdb358ee125689c8c78cef03423c13f`，本地与远端领先/落后为 0/0。
- 新开发分支为 `feat/company-profile-ai-insights-20260824`，隔离 worktree 为 `.worktrees/company-profile-ai-insights-20260824`，基点为上述远端 SHA。
- 根工作树的 `docs/PROJECT_PROGRESS.zh-CN.md`、`.workbuddy/`、调试脚本、`tmp/`、`web/saas-admin/` 等内容不进入本分支，也不执行 reset、clean、覆盖或提交。

## 3. 现状与范围矩阵

| 范围 | 当前事实 | 根本缺口 | 本轮结果 |
| --- | --- | --- | --- |
| 唯一企业资料菜单 | manifest 有路由，菜单严格按 `allowedRoutes` 过滤 | 页面权限在 0131 被改成超管专属；后端 deny-only；前端又检查 `isSuperAdmin` | 页面权限改为可授予；API 走页面资源授权；前端按实际路由授权展示 |
| 唯一企业资料直接 URL | loader 对无页面权限返回 403 | 即使普通用户具备业务授权意图，也无法获得该权限 | 已授权普通用户通过；未授权普通用户仍 403；待配置企业仍仅超管可初始化 |
| 情绪识别 | 通用两列表，读取每日聚合摘要 | 无人员/情绪/日期筛选，无证据回读，Provider 写死 | 读取会话分析结果中的客户情绪、原因与证据 ID |
| 员工评分 | 通用两列表，读取每日聚合摘要 | 无分数区间与员工维度，无评分证据 | 读取 `employeeQa.score`、维度、优缺点、建议与证据会话 |
| 沟通关键词 | 通用两列表，读取每日聚合摘要 | 无关键词筛选与原会话证据 | 读取 `customer.keywords`、摘要、来源窗口与证据消息 |
| Dashboard lint | 86 errors / 0 warnings | 严格类型规则揭示无效联合、未使用项、不安全测试断言、未绑定方法等 | 按目录和规则类型原子修复，最终全量 0 errors / 0 warnings |

## 4. 企业资料问题的系统化根因

### 4.1 数据流

菜单不是由旧菜单接口决定，而是：

`mochat_go_dashboard_permissions` → `/dashboard/access/profile` 的 `effectivePermissions` → `access-loader.ts` 的 `allowedRoutes` → `buildYuanhuNavigation()` 按 manifest 路由过滤。

直接 URL 与页面 API 分别由 `access-loader.ts` 和 `DashboardAccessGuard` 再次失败关闭。

### 4.2 三层阻断

1. 迁移 `0131_identity_realms_single_corp_cutover.up.sql` 将 `dashboard.company_setting.website` 改为 `restriction='superadmin_only'`、`superadmin_only=1`。
2. `dashboard_route_policy.go` 将企业资料全部 API 和 `/dashboard/providers/status` 列入 deny-only，普通用户即使有目录资源映射也会先被拒绝。
3. `website-page.tsx` 用 `isSuperAdmin` 决定是否发起查询，造成前端第二次拒绝。

因此该问题是确定性权限策略不一致，不是树展开、单企业数量、路由注册或 CSS 偶发问题。

### 4.3 修复策略

- 新增纠正迁移，不改写已应用的 0131 字节：把唯一企业资料页面恢复为 `grantable`。
- 从 deny-only 中移除只属于该页面资源的企业资料 API；继续由 catalog resource + effective permission 进行租户内授权。
- 待配置企业仍沿用现有 bootstrap 特例：只有同租户超级管理员可完成首次配置。
- 页面组件以 `allowedRoutes.has('/company-setting/website')` 作为普通运行时准入；测试可显式注入访问权。
- Provider 状态的底层原因与缺失项仍只对超级管理员展示；普通被授权用户只看到脱敏状态、代码和运行时间。
- 未授权普通用户不获得有效权限、菜单不出现、直接 URL 403、API 403，不放宽其他企业设置页面。

## 5. 圆弧 AI 三页调研记录

调研使用已登录会话，从 `https://web-ai-analysis-work-wechat.yuanhu.com/#/ai-insight/communication-keyword` 进入并核实实际路由。

### 5.1 情绪识别 `/ai-insight/emotion`

- 筛选：客户姓名、员工姓名、情绪对象（客户/员工）、情绪（正面/负面/中性）、会话截止日期范围。
- 列表：沟通人、沟通方式、客户情绪、员工情绪、会话时间、分析时间、分析详情、会话详情。
- 当前样本有一条真实列表记录；详情复用完整会话分析抽屉。
- 可借鉴：人员与情绪筛选分离、状态标签、会话时间与分析时间同时呈现、分析详情与原会话并列。
- 不照搬：MoChat 当前持久化合同只有客户情绪，没有独立员工情绪结论；不得把员工质检分伪装为员工情绪。

### 5.2 员工评分 `/ai-insight/employee-score`

- 筛选：客户名称、跟进人姓名、手机号、最低评分、最高评分、分析日期范围。
- 列表：沟通人、会话评分、评分说明、时间、操作。
- 调研时页面真实返回 0 条并显示“暂无数据”；该空态证明不能从参考页复制评分样本。
- 可借鉴：分数区间、评分说明、会话/分析双时间、详情与原会话跳转。

### 5.3 沟通关键词 `/ai-insight/communication-keyword`

- 筛选：客户名称、跟进人姓名、手机号、沟通关键词、分析日期范围。
- 列表：沟通人、关键词标签、会话/分析时间、分析详情、会话详情。
- 详情是右侧大抽屉，包含客户质量、成交意愿、流失风险、沟通摘要、关键词、需求、客户情绪、员工质检和行动建议；会话详情回读原消息。
- 参考页 Escape 不关闭抽屉；MoChat 已有详情组件支持关闭按钮、遮罩和 Escape，本轮继续使用更完整的本仓库交互规范。

### 5.4 统一取舍

三个页面复用 `/ai-insight/session-analysis` 与 `/ai-insight/smart-analysis` 的工作区层级、查询栏、状态条、列表、固定分页和详情抽屉，不复制圆弧品牌、颜色、静态业务数据或不足的关闭行为。

## 6. 方案比较与选择

| 方案 | 优点 | 风险 | 结论 |
| --- | --- | --- | --- |
| A. 继续读取每日聚合 `mochat_go_ai_insight_results` | 改动最少 | 只有一段摘要，无法按会话、人员、证据追溯；Provider 字段失真 | 拒绝 |
| B. 为三页重新调用模型并新增三套结果表 | 可完全定制 | 会扩大 Provider 调用面，违背自动分析关闭和 Phase 7 暂停边界，迁移与运行风险高 | 拒绝 |
| C. 将三页建成会话分析持久化结果的专用投影 | 真实来源、现有租户/RBAC/证据链可复用，不触发外部调用 | 只能展示现有 schema 已产生的客户情绪、员工评分和关键词 | 采用 |

## 7. 数据来源矩阵

| 展示字段/能力 | 前端字段 | API | 后端/存储 | 空值与失败语义 | 权限口径 |
| --- | --- | --- | --- | --- | --- |
| 员工/客户/群 | `employee`、`target` | `/{view}/records` | `mochat_go_ai_conversation_insights` 快照 + 权威人员表回读 | 名称缺失视为合同错误，不显示匿名假值 | tenant + corp + employee scope |
| 来源窗口 | `sourceWindow` | records/detail | `source_started_at`、`source_ended_at`、`source_message_count`、`source_fingerprint` | 指纹或消息数无效时前端拒绝数据 | 同上 |
| 客户情绪 | `result.customer.emotion` | emotion records/detail | `result_json` 的 schema v1/v2 会话分析结果 | 缺失显示“证据不足”，不推断 | 同上 |
| 员工评分 | `result.employeeQa.score` | employee-score records/detail | 同一 `result_json` | 失败记录无分数；0 分保留为真实数值 | 同上 |
| 沟通关键词 | `result.customer.keywords` | keyword records/detail | 同一 `result_json` | 空数组显示“未提取关键词” | 同上 |
| Provider/模型/提示词版本 | `provider`、`model`、`promptVersion` | records/detail/status | 洞察结果快照与 Provider 状态 | 未记录时明确显示“未记录”，不写死 DashScope | 同上 |
| 会话证据 | `messages`、`evidenceMessageIds` | `/{view}/detail?id=` | 按 `conversation_key`、来源窗口回读 `mc_work_message_1..10` | 消息不存在显示“暂无可展示证据”，不补模拟消息 | 同一员工 scope 重新约束 |
| 原会话跳转 | `conversationUrl` | detail | 后端按目标类型、员工和会话 key 生成 Dashboard 内部 URL | 详情缺 URL 视为合同错误 | 目标会话仍受自身 RBAC |
| 运行状态 | `provider`、`run` | `/{view}/status` | Provider `Status()` + session analysis 最近运行记录 | Provider unavailable、未运行、运行失败分别展示 | 同上 |

知识库只通过会话分析助手上下文影响分析提示，不作为证据消息。三页不发起新分析，只读取已经落库的结果。

## 8. 页面与交互方案

### 8.1 统一结构

每页依次为：轻量标题行 → 查询栏 → Provider/运行状态条 → 三张真实摘要卡 → 结果列表/卡片 → 固定每页 20 条分页 → 详情抽屉。

摘要卡必须明确口径：总记录数使用后端分页总数；“本页完成/失败”“本页分布/平均分/关键词数”仅统计当前页，并在卡片文案标注“当前页”，不伪装成全量聚合。

### 8.2 URL 状态

- 查询、重置、分页使用 `history.pushState`；刷新使用 `replaceState`。
- 浏览器前进/后退恢复 draft 与 applied 状态。
- 情绪页参数：`employeeId`、`customerName`、`emotion`、`startDate`、`endDate`、`page`。
- 员工评分参数：上述通用项加 `minScore`、`maxScore`。
- 关键词参数：上述通用项加 `keyword`。
- 日期按 `YYYY-MM-DD`；分数为 0–100 整数且最小值不得大于最大值。

### 8.3 详情

列表整行/整卡可打开详情；内部“查看原会话”按钮不误触整行。详情复用现有 `InsightDrawer`，展示来源窗口、Provider、模型、提示词版本、完整客户分析、员工质检、证据消息，并支持关闭按钮、遮罩、Escape 和内部原会话跳转。

### 8.4 诚实状态

- Provider 不可用：状态条显示真实 `state/code/message`；不把已存量结果清空。
- 没有已持久化分析：空态说明“当前筛选暂无已生成分析结果”，不写成业务正常或 0 分。
- 分析失败：行显示错误摘要；详情不渲染伪分数、伪情绪或伪关键词。
- 归档/证据不可回读：保留结果快照，同时明确证据消息为空。
- 自动日分析与启动即分析保持关闭；页面查询、刷新、打开详情均不得调用模型。

## 9. API、存储与迁移

### 9.1 API

为每个 `{view}`（`emotion`、`employee-score`、`communication-keyword`）注册：

- `GET /dashboard/ai-insight/{view}/records`
- `GET /dashboard/ai-insight/{view}/detail?id={positiveInt}`
- `GET /dashboard/ai-insight/{view}/status`
- `GET /dashboard/ai-insight/{view}/filter-options`
- `GET /dashboard/ai-insight/{view}/export`

三个视图都映射到 `analysis_type='session'`，不创建写接口。查询会追加各自 JSON 字段过滤，所有条件参数化并转义 LIKE 通配符。

### 9.2 存储

不新增结果表，不改写会话分析历史数据。只扩展现有 repository filter：

- `emotion`：`JSON_UNQUOTE(JSON_EXTRACT(result_json,'$.customer.emotion.label')) = ?`
- `minScore/maxScore`：对 `$.employeeQa.score` 做 0–100 数值过滤
- `keyword`：对 `$.customer.keywords` 的 JSON 文本做转义后的包含过滤

失败或旧 schema 缺字段的记录在无专用过滤时可见；启用专用过滤时自然不匹配。

### 9.3 迁移

- `0162_company_profile_grantable`：只纠正页面 restriction；down 恢复 0131 策略。
- `0163_ai_insight_projection_resources`：为三个页面新增只读 API resource；down 仅删除本迁移新增的资源。
- 不改写 0127、0131 或其他已应用迁移字节。
- 必须验证 fresh apply、重复执行、checksum、保留卷升级和迁移健康。

## 10. 权限、租户与安全

- 所有 API 继续从 Dashboard principal 取得 tenant/corp，不接受前端 `corpId` 决定作用域。
- 普通用户必须具有对应页面 effective permission；员工范围由 `AllowedEmployeeIDs` 与 `EmployeeScopeRestricted` 同时约束列表、筛选选项、详情、导出和证据回读。
- 唯一企业资料权限可授予，但其他四个企业设置页面继续超管专属。
- 前端权限只控制体验，后端 guard 是权威边界。
- Provider Secret、会话存档 Secret、RSA 私钥不出现在日志、文档、测试夹具或前端缓存；Provider 状态对普通授权用户继续脱敏。
- CSV 继续防公式注入；导出与列表使用相同筛选和员工 scope。

## 11. 响应式与宽屏

- 2560×1440：工作区填满可用宽度，查询栏多列排列，表格/卡片不在中间留下大块空白。
- 常规桌面：查询栏自动换行，结果列保持关键字段优先，详情抽屉保持可滚动。
- 窄屏：筛选字段和摘要卡单列，结果改为可读卡片或安全横向容器；页面本身不得出现无法到达的横向溢出。
- 小高度：详情抽屉头尾固定语义不遮挡正文，正文独立滚动，关闭按钮始终可达。

## 12. 验收标准

1. 已授权普通用户的菜单包含“唯一企业资料”，刷新后仍存在，直接 URL 进入成功且高亮正确；无权限普通用户菜单无该项，直接 URL 与 API 均 403；超管和 pending bootstrap 行为无回归。
2. Dashboard 全量 lint 为 0 errors / 0 warnings，未关闭规则、未增加大范围 ignore、未滥用 `any` 或改变 tsconfig。
3. 三页查询、重置、刷新、分页、URL 恢复、详情、证据回读、原会话跳转、错误重试、空态和 Provider 状态符合本设计。
4. 每个数值、标签和列表字段可追溯到 API 与存储；前端不存在业务静态数组、随机数、固定趋势或伪成功。
5. 相关前后端测试、Dashboard 全量测试/typecheck/lint/build、全量 Go、隔离 MariaDB integration、Phase 4 RBAC、53 页 benchmark、all-pages evidence、provider completion、`git diff --check` 和凭证扫描通过。
6. 保留卷 Docker 仅重建必要服务；app/MySQL/Redis healthy，`/healthz`、`/readyz` 200，日志无 migration/checksum/panic/fatal。
7. 浏览器覆盖普通桌面、2560×1440、窄屏和小高度，控制台无新增 error/warning，无未解释的请求失败或横向溢出。

## 13. 设计自审

- 未保留占位标记或未定义的成功口径。
- 企业资料“可授予”不等于“全员可见”，失败关闭边界明确。
- 三页只读投影与“自动分析关闭”“Phase 7 暂停”一致。
- 数据矩阵没有把知识库、模拟归档或参考页静态数据当作消息证据。
- 迁移采用新增纠正版本，不破坏已应用迁移 checksum。
