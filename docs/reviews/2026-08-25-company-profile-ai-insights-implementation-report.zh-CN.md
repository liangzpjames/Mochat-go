# 企业资料权限、Dashboard Lint 与三项 AI 洞察实施及自测报告

## 1. 交付结论

本任务已在隔离分支 `feat/company-profile-ai-insights-20260824` 完成实现和本地验收。当前权威 `main` 已使用普通 push 发布并在线核实；新开发没有合入 `main`，也没有推送新分支。

本轮交付结果如下：

- “唯一企业资料”菜单、直接 URL、应用服务和数据库仓储的五层权限阻断均已闭合。具备精确页面资源、有效登录身份和激活企业绑定的普通用户可以查看并进入；未授权、待配置或停用绑定仍失败关闭。
- Dashboard 全量 ESLint 为 0 errors / 0 warnings；没有关闭规则、缩小扫描范围、增加大范围忽略或修改 TypeScript 配置掩盖问题。
- `/ai-insight/emotion`、`/ai-insight/employee-score`、`/ai-insight/communication-keyword` 已改为读取会话分析持久化结果的真实投影工作区，具备筛选、URL 恢复、分页、导出、详情和消息证据回读。
- AI 洞察筛选目录已改接企业现有权威员工与客户数据，不再用“已有分析结果”反推可选对象。当前验收企业可用员工 12、可用客户 16；成功洞察覆盖员工 9、客户 12，未覆盖对象仍能被选择并明确显示为待分析。
- 数据概览的 AI 洞察栏已与三个优化页面改为同一持久化来源，当前显示已分析会话 18、负向客户会话 1、平均员工评分 70.722、关键词总数 67、已覆盖员工 9、已覆盖客户 12。
- 已使用用户临时提供的 DeepSeek 凭证执行一次性全量回填：18 个会话分析与 18 个智能分析均成功持久化。凭证只通过临时文件注入，运行后已删除且凭证扫描无命中；自动日分析和启动即分析仍关闭。
- 新增迁移 `0162`、`0163` 已在保留卷 MariaDB 上应用并重复执行通过；应用、MariaDB、Redis 健康。
- Phase 7 继续暂停；本轮执行的是对现有归档候选的一次性显式分析，没有把验收模拟归档声明为真实企微会话存档。

## 2. 权威主线发布证据

发布前在线核对：

| 项目 | 结果 |
| --- | --- |
| 发布前本地 `main` | `756ac3765bdb358ee125689c8c78cef03423c13f` |
| 发布前 `origin/main` | `de902ef1797ae1dea2933ee9be2838253b34248e` |
| 祖先关系 | `origin/main` 是本地 `main` 的祖先，无分叉 |
| 领先/落后 | 领先 141、落后 0 |
| 发布方式 | 普通 `git push origin main`，未 force、未 amend |
| 发布后在线核实 | `git ls-remote origin refs/heads/main` 精确返回 `756ac3765bdb358ee125689c8c78cef03423c13f` |

发布时只推送了当时已经提交的 `main`。根工作树中的总进度文档、调试目录、临时文件和其他用户现场没有进入发布提交。

## 3. 企业资料问题：根因与修复

### 3.1 精确对象与复现条件

用户报告对应菜单为“企业设置 → 唯一企业资料”，路由是 `/company-setting/website`。菜单数据链为：

`RBAC catalog` → `/dashboard/access/profile.effectivePermissions` → `allowedRoutes` → Dashboard 菜单树过滤。

真实登录态复现证明问题不是折叠状态、CSS 或企业数量判断：已授权普通用户起初看不到菜单；修复目录与前端后，菜单可见但页面 API 仍返回 403，由此继续定位出后端服务层和仓储层阻断。

### 3.2 五层根因

1. 历史迁移 `0131` 把 `dashboard.company_setting.website` 改成 `superadmin_only`，角色无法正常获得该页面资源。
2. `dashboard_route_policy.go` 又将企业资料 API 和 Provider 状态放入 deny-only，目录授权在路由守卫前被拒绝。
3. `website-page.tsx` 以 `isSuperAdmin` 决定是否查询，形成前端二次阻断。
4. `companyprofile.Service.authorize` 只接受超级管理员，菜单放行后应用服务仍返回 403。
5. `store.checkCompanyActor` / `companyActorFactsAllowed` 同样把超级管理员作为唯一成立条件，服务层放行后真实仓储调用仍失败。

### 3.3 修复与安全边界

- 通过新增迁移 `0162_company_profile_grantable` 纠正目录，不改写已应用迁移字节。
- 只移除该页面所需 API 的 deny-only 策略，仍由 catalog resource、有效权限和路由守卫控制。
- 前端按 `allowedRoutes.has('/company-setting/website')` 准入，不硬编码菜单，也不放宽其他企业设置页面。
- 服务层和仓储层只识别精确资源码 `dashboard.company_setting.website`；普通用户还必须通过数据库身份、租户、企业绑定、认证版本和激活状态校验。
- 缺少权限上下文、未授权普通用户、待配置绑定、停用绑定均拒绝；首次企业配置能力仍只对同租户超级管理员开放。
- Provider 诊断详情继续只对超级管理员展示；普通授权用户仅得到脱敏状态，不返回配置缺失细节或凭证材料。

### 3.4 回归结果

- 单元与路由测试覆盖：菜单资源、直接 URL、页面高亮、API 路由、未授权普通用户、管理员、待配置与停用边界。
- 真实 MariaDB 集成测试 `TestCompanyProfileStoreAllowsGrantedOrdinaryActorOnRealMariaDB` 通过，并验证去掉精确授权上下文后仓储拒绝。
- 临时普通用户真实登录验证：激活企业绑定且只授予精确资源后，菜单可见，企业资料 API 返回 200。验收身份、会话和临时授权数据已清理，原企业绑定状态已恢复。

## 4. 三项 AI 洞察实现

### 4.1 参考调研与统一设计

已在登录状态下逐页调研圆弧 AI 的实际页面：

- 情绪识别：`/ai-insight/emotion`
- 员工评分：`/ai-insight/employee-score`
- 沟通关键词：`/ai-insight/communication-keyword`

借鉴了人员、日期、业务维度筛选，双时间展示，摘要、列表、详情和会话证据的组织方式。最终视觉与交互直接复用本仓库 `/ai-insight/session-analysis` 和 `/ai-insight/smart-analysis` 的工作区、按钮、分页、抽屉及响应式规则，没有复制圆弧品牌或静态业务样本。

### 4.2 数据来源与诚实状态

三个页面均从 `mochat_go_ai_conversation_insights` 的会话分析成功结果投影，不新增页面打开即调用 Provider 的行为：

| 页面 | 持久化字段 | 页面输出 |
| --- | --- | --- |
| 情绪识别 | `customer.emotion`、`emotionReason`、证据消息 ID | 五态情绪、原因、来源窗口、消息证据 |
| 员工评分 | `employeeQa.score`、维度、优缺点、建议 | 0–100 整数评分、说明、来源会话 |
| 沟通关键词 | `customer.keywords`、摘要、证据消息 ID | 客户关键词、摘要、来源窗口、消息证据 |

投影结果保留租户、企业、员工、客户/群聊、Provider、模型、提示词或规则版本、生成时间、来源起止时间、来源消息数和来源指纹。详情通过已有会话证据接口回读消息；知识库只作为既有分析背景，不替代消息证据。

证据消息 ID 不再使用各分表可碰撞的自增值：优先输出 `msgid:{企微消息 ID}`，缺失时输出 `shard:{分表号}:{自增 ID}`。入站直聊使用客户名称，群聊缺少参与人身份时明确标注“群成员（身份未归档）”。为兼容已经持久化的旧分析，旧自增 ID 只有在当前详情窗口唯一时可高亮；一旦跨分表碰撞就不高亮，避免把错误消息标成关键证据。

Provider 不可用或尚未运行时，页面明确显示不可用、等待分析或失败状态；已有持久化结果仍可查询。接口失败不会被转换成空列表或伪成功。非整数、越界员工评分不进入投影，CSV 导出限制条数并处理公式注入。

### 4.3 API、迁移与前端

- 新增三类列表、筛选选项和认证导出 GET 资源，共 15 个 RBAC 资源，由迁移 `0163_ai_insight_projection_resources` 登记。
- 列表支持租户/企业范围、员工、客户、情绪、分数区间、关键词、日期和分页过滤。
- 前端严格校验分页、状态、详情消息、来源时间和成功结果的 Provider、模型、提示词版本、分析时间；缺失或类型错误会进入错误态，不折叠成“暂无结果”。
- 查询条件写入 URL，刷新可恢复；重置清空 URL；最后一次请求胜出，避免快速筛选或详情切换的旧响应覆盖新状态。
- 详情抽屉支持关闭按钮、遮罩和 Escape；窄屏筛选、卡片、表格和抽屉降级为可用布局。

### 4.4 权威目录、覆盖口径与概览同源

原实现的员工选项来自洞察结果表，因此只有“已被分析的员工”可选；客户筛选只有自由文本，也没有企业权威客户目录。这把数据覆盖情况错误地当成业务全集，是本轮“没有接入所有可用客户和员工”的根因。

修复后采用两套明确分离的集合：

- 可用员工来自 `mc_work_employee`，限定当前租户/企业、`status=1` 且未删除；普通角色继续叠加员工可见范围，管理员也不能跨租户或跨企业读取。
- 可用客户来自 `mc_work_contact`，并通过 `mc_work_contact_employee` 校验客户与员工的有效关系；普通角色只能取得其可见员工关系内的客户。
- 已分析员工/客户只从成功持久化的洞察结果计算，用于覆盖数，不再裁剪筛选目录。
- 客户筛选改用稳定客户 ID 精确查询，员工与客户选择、查询、重置和 URL 恢复使用同一合同。

真实保留卷验收中，目录为 12 名可用员工、16 名可用客户；洞察覆盖为 9 名员工、12 名客户。页面同时展示“可用”与“已分析”计数，避免把未覆盖误写成无业务对象。

数据概览的 AI 洞察栏不再读取旧的混合质量指标，而是直接聚合 `mochat_go_ai_conversation_insights`，与三个页面同一租户、企业、成功状态和字段口径。六张卡片分别链接到相应优化页面，桌面端为 3×2、平板为 2×3、窄屏为单列。

### 4.5 DeepSeek 一次性全量分析

新增 `cmd/mochat-ai-insight-run` 一次性命令，要求使用仅包含凭证的临时文件，拒绝把密钥作为命令行参数或提交到配置文件。命令会记录候选数、成功数、失败数和纠错重试结果，但不输出凭证。

本轮在保留卷现有归档范围内执行：

| 分析类型 | 候选 | 最终成功 | 最终失败 |
| --- | ---: | ---: | ---: |
| 会话分析 | 18 | 18 | 0 |
| 智能分析 | 18 | 18 | 0 |

首次返回中不满足严格结构合同的结果通过一次纠错重试重新校验后成功；最终 36/36 均持久化。结果记录 Provider 为 `openai-compatible`、模型为 `deepseek-v4-flash`、提示词版本为 `conversation-v2`，并保留来源时间窗口、消息证据和生成时间。临时凭证文件在运行结束后删除，仓库凭证模式扫描为 0 命中。应用环境中的自动日分析、启动即分析与页面打开即分析均保持关闭。

## 5. Dashboard ESLint 清零

基线扫描在本分支实际得到 112 errors；用户提示的历史基线为 86 errors / 0 warnings。差异来自当前发布基点包含了更多待检查文件。修复按目录和错误类型拆成原子提交：AI 合同测试、全局会话、会话运营、风险与敏感词、投影调用次数。

最终执行 `corepack pnpm --filter @mochat/dashboard lint`：0 errors / 0 warnings。修复以收紧类型、删除无效未使用项、绑定方法、补充测试类型和准确断言为主，没有新增大范围 `eslint-disable`、ignore、`any` 或 tsconfig 绕过。

## 6. 自动化与集成验证

### 6.1 新鲜全量验证

2026-08-25 在干净隔离 worktree 重跑：

| 命令/门禁 | 结果 |
| --- | --- |
| `go test ./... -count=1` | 通过 |
| Dashboard lint | 通过，0 errors / 0 warnings |
| Dashboard typecheck | 通过 |
| Dashboard test | 142 个测试文件、918 项测试全部通过 |
| Dashboard production build | 通过；保留现有大 chunk 提示，不影响构建退出码 |
| Phase 4 Dashboard RBAC catalog/completion | 通过；53 页，49 个普通页面，4 个超级管理员页面，无未映射页面 |
| Yuanhu benchmark manifest | 53 页通过 |
| Dashboard all-pages evidence | 12/12 通过 |
| 标准 Provider completion | 通过 |
| `git diff --check` | 通过 |
| 仓库凭证模式扫描 | 0 命中；临时 DeepSeek 凭证文件已删除 |

### 6.2 真实 MariaDB 集成

使用专用临时 schema 执行并在测试结束后清理，未改写用户业务数据：

- `TestSaveInsightPersistsAndUpdatesRealMariaDB`
- `TestEmployeeOptionsSearchesNamesOnRealMariaDB`
- `TestProjectionFiltersUseRealMariaDBJSONAndEmployeeScope`
- `TestProjectionMariaDBJSONTypeScoreContract`
- `TestCompanyProfileStoreAllowsGrantedOrdinaryActorOnRealMariaDB`
- `TestDirectoryOptionsUsesAuthoritativeDirectoriesAndRestrictedRelationsOnRealMariaDB`

上述测试全部通过。新增目录集成测试在隔离 schema `mochat_go_ai_insight_integration_20260825` 中验证权威员工表、权威客户表、客户员工关系、角色员工范围和租户隔离；测试后只删除该精确临时 schema，生产 schema 未修改。AI 投影测试覆盖 MariaDB JSON 类型判定、整数评分、员工/租户范围和持久化更新；企业资料测试覆盖精确授权通过与缺少授权失败关闭。

独立复审追加的回归还覆盖：跨分表相同自增 ID 不碰撞、入站/出站发送者署名、旧证据 ID 唯一兼容与碰撞失败关闭、畸形分页/运行状态/追溯字段诚实失败。

### 6.3 迁移与保留卷 Docker

- `0162`、`0163` 已应用；迁移账本分别记录校验和 `1ea2fd27ace5f64971fa336a8087f227fb5825021c933c590483a09a6b3291a2`、`2137d532e9f17e05953f68c90a47103f8ae81bf225cc0c0eb2c79c5a411030a1`。
- 两个 up 都会保留预先存在的精确资源，无法事后证明行所有权。因此 down 改为保守 no-op，不删除或覆盖未知来源资源；旧代码仍由路由 deny-only 或缺失处理器失败关闭。已在专用 MariaDB schema 预置两个同键资源，执行 0162/0163 up 后再执行 down，最终 16 个资源全部保留，两个预置版本标记各保留 1 条，schema 清理计数为 0。
- 在同一保留卷重复执行 `mochat-migrate -action up -project-root /app` 退出码为 0，两项迁移均保持 `applied`，未改写已应用迁移。
- 最新应用镜像已从本分支重建；app、MariaDB 10.6、Redis 7 均为 healthy，`/healthz` 与 `/readyz` 均返回 200。重建只替换 app，未重建 MySQL、Redis 或数据卷。
- 首次从隔离 worktree 重建时发现该 worktree 没有根工作树受保护的 `.env.local`，导致既有加密企业资料无法解密。最终部署只读加载现有部署环境文件后重建 app，企业资料恢复正常；没有复制、输出或提交其中凭证。
- 应用最近 400 行日志中 migration failure、checksum、panic、fatal 命中数为 0。
- MariaDB 与 Redis 卷均保留，创建时间均为 `2026-08-12T10:54:32Z`；没有删除或重建数据卷。

## 7. 真实浏览器验收

本地保留卷环境 `http://127.0.0.1:18080` 已完成：

- 企业资料：真实普通用户登录复现菜单不可见和后续 API 403；修复后精确授权、激活绑定场景菜单可见且 API 200。未授权和绑定边界由自动化回归补齐。
- 情绪识别：查询、URL 恢复、重置、刷新和详情抽屉通过；详情显示真实持久化 Provider、模型、提示词版本、来源窗口、消息数和关键证据，Escape 可关闭。
- 最终镜像复核中，历史结果通过唯一旧 ID 兼容高亮两条关键证据；客户入站消息显示客户名称，未再出现“员工姓名 · 客户发送”的错误署名。
- 员工评分：分数区间筛选、URL 恢复、重置、刷新和详情通过。
- 沟通关键词：关键词筛选、URL 恢复、重置、刷新和详情通过。
- 三个页面的员工选择器实际列出 12 名可用员工，客户选择器实际列出 16 名可用客户；覆盖提示分别为已分析员工 9、已分析客户 12。
- 数据概览六项同源指标实际显示 18、1、70.722、67、9、12；每张卡片的跳转目标与优化页面一致。
- 1920×1080、375×844、1366×600 三种视口均无文档级横向溢出；窄屏和小高度保持可操作，结果表的内部横向滚动不扩张页面。
- 浏览器控制台 error/warning 为 0。
- 详情实际显示 `openai-compatible / deepseek-v4-flash`、`conversation-v2`、来源时间和消息证据；查询失败仍显示错误恢复入口，不转换为伪空态。

验收截图保存在工作区外的 `D:\workspace\mochat-go\output\company-profile-ai-insights-20260825`，包括企业资料菜单、三个洞察页面的桌面/宽屏/窄屏/小高度状态以及最终数据概览。截图不进入 Git 提交。

保留卷中的验收归档包含明确标注的模拟消息，因此这里只验证“持久化结果、投影、筛选和证据回读”链路，不把它声明为真实企业微信会话存档完成。

## 8. 已知外部限制

1. Phase 7 继续暂停；真实企微会话存档接入没有在本轮完成或宣称完成。
2. 自动日分析和启动即分析保持关闭；用户凭证只用于本轮显式一次性运行，没有配置成常驻应用密钥。未来新增归档不会自动发起外部调用。
3. 标准 Provider completion 通过；required-real 归档集成因当前验收命令没有全局真实归档 DSN 而跳过。本报告不把该项声明为真实企微归档通过。
4. 企业资料页在缺少 Provider 状态来源的环境中会显示加载失败，这是失败关闭行为，不伪装为可用状态。

## 9. 提交与分支状态

主要原子提交：

| 范围 | 提交 |
| --- | --- |
| 中文设计与实施计划 | `5f8b79c`、`c667491` |
| 企业资料失败测试与 RBAC 修复 | `55c226d`、`7485281`、`3064579`、`7e24b27` |
| 企业资料服务层与仓储层闭环 | `2430af9`、`15e7795` |
| 三项 AI 洞察失败测试与合同 | `e689af0`、`1d4b693`、`2fe8aa5` |
| AI 后端投影与评分保护 | `a1004ed`、`73479aa` |
| AI 前端工作区与竞态/导出保护 | `847e3e3`、`b8ca0eb` |
| Dashboard lint 原子批次 | `0152fe8`、`38118e8`、`0828982`、`a893149`、`63b057e` |
| 独立复审 RED 与追溯/回滚闭环 | `b79c115`、`f074f7d` |
| 追溯与 Provider 字段错误类型回归 | `b3ba279`、`672ad2b` |
| 权威目录与概览设计 | `055817f` |
| 权威员工/客户目录与前端覆盖 | `b787287`、`bf137d9` |
| 数据概览同源指标 | `08dfcbf`、`f9ec837` |
| 安全一次性分析与全量纠错 | `b5ced07`、`31e28c3` |
| 权威目录角色/真实 MariaDB 回归 | `3a9c15f` |

最终交付保留在 `feat/company-profile-ai-insights-20260824` 和隔离 worktree 中。未经用户明确授权，不合并到 `main`，不推送该分支。建议审阅后使用普通 merge 保留原子提交与完整测试轨迹。
