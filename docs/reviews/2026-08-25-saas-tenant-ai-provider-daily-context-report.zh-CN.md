# SaaS 租户 AI Provider、当日洞察与页面优化实施及自测报告

## 1. 结论

本分支已完成企业资料菜单权限闭环、Dashboard ESLint 清零、情绪识别/员工评分/沟通关键词三个洞察页面优化、完整员工与客户目录接入、数据概览同源指标、SaaS 租户级 AI Provider 配置、凭证加密、Provider 有效期、当日分析、上一日结果连续性上下文和旧表删除。代码、迁移、真实 MariaDB、保留卷 Docker、自动化门禁与已登录 Dashboard 浏览器点击均已验收。

当前运行库中的归档来源全部标记为模拟来源，因此本报告不把现有洞察称为真实企微会话存档。Phase 7 继续暂停。DeepSeek 配置已加密保存并具备网络解析条件，但将会话正文发送给外部 Provider 的一次性真实调用尚未执行，原因见第 12 节。

## 2. 发布与分支证据

- Git 根：`D:\workspace\mochat-go\mochat-go`。
- 当前已提交主线已使用普通 push 发布；发布前确认 `origin/main` 是 `main` 祖先且无分叉。
- 推送后本地 `main`、`origin/main` 与在线 `git ls-remote origin refs/heads/main` 均为 `756ac3765bdb358ee125689c8c78cef03423c13f`。
- 总进度文档、根工作树未跟踪目录及其他受保护现场均未加入发布。
- 新开发位于隔离分支 `feat/saas-tenant-ai-provider-daily-analysis-20260825`，本文编写前实现 HEAD 为 `bf942ff825f9fb54de97e26ca7ec484cdcd1f839`。
- 新分支未合入 `main`、未 push、未改写历史。

## 3. 企业资料菜单根因与修复

根因不是页面缺失，而是“唯一企业资料”资源在菜单、路由、RBAC catalog、角色可授予权限与服务层仓储校验之间不一致：普通有权角色无法取得完整授权链，服务层又把精确授权用户误判为不可访问。

修复内容：

- 恢复企业资料页面资源的可授予属性并补齐 RBAC catalog、completion 与菜单树映射。
- 后端以精确页面权限校验仓储访问，不放宽全局权限，不绕过 RBAC。
- 保持单企业绑定语义，页面不提供新建、切换或删除企业。
- 增加普通授权用户、无权用户、管理员、直达 URL、刷新与菜单可见性回归。

浏览器实际结果：已登录 Dashboard 超级管理员展开“企业设置”后可见“唯一企业资料”，点击进入 `/company-setting/website`；刷新仍停留原路由，页面标题、Provider 状态、企业应用、回调、存档、验证、同步和审计区域正常显示，AI 洞察菜单切换后对应链接具有 active 状态。

## 4. Dashboard ESLint

按 AI 洞察、全局会话、会话运营、风控与敏感词目录拆分原子提交，未关闭规则、未缩小 lint 范围、未增加大范围 ignore、未更改 tsconfig 掩盖错误。

最终命令：

```text
corepack pnpm --filter @mochat/dashboard lint
```

结果：0 errors，0 warnings，退出码 0。

## 5. 三个 AI 洞察页面

### 5.1 同一页面体系

情绪识别、员工评分、沟通关键词复用会话分析/智能分析的工作区层级、查询栏、状态条、目录覆盖、摘要、结果表、固定 20 条分页、详情抽屉、证据消息、原会话跳转、反馈和响应式降级。页面不保存静态业务数组、不生成随机分数、不伪造趋势或成功状态。

### 5.2 真实持久化投影

三页只读取 `mochat_go_ai_conversation_insights` 的结构化持久化结果：

- 情绪识别投影 `customer.emotion.label/reason/evidenceMessageIds`。
- 员工评分投影 `employeeQa.score/dimensions/strengths/issues/suggestions`，合法 0 分不会被当作空值。
- 沟通关键词投影 `customer.keywords`，保留真实字符串内容并由后端完成筛选。
- 详情回读同一条洞察的来源窗口、Provider、模型、Prompt 版本、消息数量和证据消息。

浏览器验收发现运行库中存在模拟归档后，页面措辞统一收紧为“已归档”“已持久化”，不再笼统宣称“真实归档”或“真实 AI 数据”。

### 5.3 完整目录与角色范围

此前员工/客户筛选由已产生洞察的结果行反推，因此未分析实体不会出现。现已改为读取权威员工和客户目录，再独立统计已分析覆盖数；后端仍应用当前登录人的 RBAC 与关系范围。

当前租户页面实测显示：可用员工 12、已分析 9；可用客户 16、已分析 12。数据库中的租户 1 原始目录为 15 名员工、16 名客户，页面 12 名员工是当前登录角色关系范围过滤后的可用集合，不是结果表反推集合。

## 6. 数据概览同步

数据概览的“AI 洞察”模块与三个优化页面读取同一结构化结果口径。浏览器实测值：

| 指标 | 值 | 跳转 |
| --- | ---: | --- |
| 已分析会话 | 18 | 会话分析 |
| 负向客户会话 | 1 | 情绪识别负向筛选 |
| 平均员工评分 | 70.722 | 员工评分 |
| 关键词总数 | 67 | 沟通关键词 |
| 已覆盖员工 | 9 | 员工评分 |
| 已覆盖客户 | 12 | 情绪识别 |

缺失字段保持 `—`，不以 0 伪装正常；模块标记为“持久化 AI 数据”。

## 7. SaaS 租户级 AI Provider

### 7.1 API 与界面

新增：

- `GET /dashboard/saasAdmin/tenantAIProvider?tenantId={id}`，权限 `platform.integrations.read`。
- `PUT /dashboard/saasAdmin/tenantAIProvider`，权限 `platform.integrations.manage`。
- SaaS“客户租户 → 租户详情 → AI 分析配置”界面，可分别配置厂商、OpenAI-compatible Base URL、模型、API Key、生效时间、失效时间、状态和乐观锁版本。

前端输入的明文 Key 仅存在于编辑与提交期间；成功、失败、Escape 和关闭均清空；GET、Query cache、日志、审计、错误与 URL 均不返回 Key 或密文。运行时按需解密 Key，仅用于 Authorization header。版本冲突会重新拉取服务端事实，重新拉取失败时保持诚实错误，不显示伪成功。

### 7.2 安全

- 每租户唯一当前配置，运行时按明确 tenant/corp 解析。
- AES-GCM 随机 nonce，加密 AAD 含租户和用途，支持 key ring 轮换。
- 缺失、未生效、过期、停用、解密失败、非法模型或地址均失败关闭，不回退平台公共 Key。
- Base URL 必须 HTTPS，拒绝 userinfo、环回、私网、链路本地和解析后不安全地址。
- 自动日分析和 run-on-start 保持关闭；保存配置不会触发外部测试请求或模型费用。

### 7.3 本地验收配置

租户 1 已保存 DeepSeek/OpenAI-compatible 配置，模型为 `deepseek-chat`，有效期为 2026-08-25 至 2026-09-25，版本 1，凭证保护状态可用。API Key 仅以认证加密密文入库，本报告和 Git 均不包含其明文、密文或末四位。

由于 SaaS 管理浏览器会话停在 MFA 登录页，无法在没有用户 MFA 的前提下完成已登录 SaaS 页面点击。配置通过同一 `MySQLStore.SaveSaaSTenantAIProvider` 正式业务入口保存，完整走服务端校验、加密、出站地址保护、事务和审计；SaaS 页面行为由 26 项全量组件/API 测试覆盖。未把缺少登录态的 SaaS 浏览器验收写成通过。

## 8. AI 请求参数如何组装

每个候选会话执行一次结构化会话分析流程；如果首次结果未通过结构校验，最多追加一次结构纠错请求。三个投影页不会分别追加三次模型调用。请求组装顺序：

1. 按租户解析当前有效 Provider、Base URL、模型和解密后的 Key；Key 仅进入 Authorization header。
2. 固定系统约束：模型只能使用本次提供的消息证据，必须输出约定 JSON schema，证据 ID 必须来自允许集合。
3. 加入当前租户、企业、分析类型、规则版本、会话键、Asia/Shanghai 当日时间窗和提示词版本。
4. 加入当天 00:00 至执行时刻的来源消息，形成唯一允许的 `evidenceMessageIds` 集合。
5. 加入已启用助手/规则说明；知识库只作背景，不能替代消息证据。
6. 加入上一自然日同租户、同企业、同分析类型、同规则版本、同会话的最近成功结果：`generatedAt`、评分和摘要。
7. 明确历史结果只用于连续性，不能作为事实或证据；证据不足时避免无依据断崖变化，确有当日消息支持时允许显著变化。
8. 使用 JSON 输出模式调用 OpenAI-compatible Chat Completions；第一次结构纠错仍保留同一证据与连续性约束。

会话分析返回客户情绪、关键词、客户判断和员工质检；智能分析返回规则匹配结论。上一日摘要中的任意伪消息 ID 不会加入证据白名单。

## 9. 表统一与迁移

- 0164 新建 `mochat_go_saas_tenant_ai_providers`。
- 0165 为 `mochat_go_ai_conversation_insights` 增加 `analysis_date`、`previous_insight_id`、`previous_score`、`previous_summary`、`previous_generated_at`，唯一范围调整为租户、企业、分析类型、规则版本、会话和分析日期。
- 同日重跑更新当日记录；跨日保留历史，上一日成功快照可追溯。
- 删除旧运行链路与旧表 `mochat_go_ai_analysis`；该表无法映射会话证据，因此不伪造数据迁移。down 只能重建空结构，恢复旧数据必须依赖迁移前备份。

真实 MariaDB 隔离 schema 已验证 0164/0165 apply、完整重复执行、每个中断阶段恢复、down/up、checksum、日去重、数据断言和旧表删除；隔离 schema 验收后已删除。保留卷运行库两次 apply 均为幂等，0165 checksum 为 `eb6c469a0b46bccb44da6f51d10d517b600f6aa4e6bbfa10f9dbcfd7e1163d5b`，旧表数量为 0。

## 10. 自动化与质量门禁

| 验证 | 结果 |
| --- | --- |
| `go test ./... -count=1` | PASS |
| Dashboard lint | PASS，0 errors / 0 warnings |
| Dashboard typecheck | PASS |
| Dashboard test | PASS，142 files / 918 tests |
| Dashboard production build | PASS |
| SaaS Admin test | PASS，6 files / 26 tests |
| SaaS Admin typecheck/build | PASS |
| Yuanhu benchmark manifest | PASS，53 pages |
| Dashboard all-pages evidence | PASS，12/12 |
| Phase 4 RBAC completion | PASS，53/49/4，scopeRequired=145 |
| Provider completion | PASS |
| `git diff --check` | PASS |
| 跟踪文件通用 `sk-` 凭证扫描 | PASS，0 files |

Dashboard 全量测试中的 `jsdom window.getComputedStyle(..., pseudoElt)` 为既有测试环境提示；所有测试文件和测试用例最终退出码为 0。

## 11. Docker 与浏览器验收

### 11.1 Docker

- 使用 `mochat-go-desktop` 保留卷原地重建最新 app，未删除数据卷。
- app、MySQL、Redis 均 healthy。
- `/healthz` 与 `/readyz` 均返回 200。
- app 日志未发现 migration/checksum/panic/fatal。
- 0164/0165 在保留卷重复 apply 成功。
- 自动日分析和启动即分析仍关闭。
- 本机 DNS 会把外部域名映射到透明代理保留段，验收容器临时写入 DeepSeek 公网解析；这是本地验收临时措施，容器再次创建后需由正常 DNS 代替，不属于产品配置。

### 11.2 浏览器点击

已登录 Dashboard 会话逐页完成：

- 唯一企业资料：菜单展开、点击、直达、刷新与页面加载。
- 情绪识别：状态查询、URL 恢复、刷新、重置、详情与证据。
- 员工评分：50–90 分查询、URL 恢复、刷新、重置、详情与证据。
- 沟通关键词：“续费”查询、URL 恢复、刷新、重置、详情与证据。
- 详情抽屉：关闭按钮、Escape、可见遮罩区域、完成按钮。
- 数据概览：六项 AI 指标和对应跳转口径。
- 1920×1080、390×844、1280×600：三页均可操作且文档级无横向溢出；390px 抽屉无横向溢出。
- 最终镜像三页重新打开后均显示已归档/已持久化诚实措辞，控制台 error/warning 为 0。

## 12. 外部边界与未宣称事项

1. 当前租户 1 有 13 个归档来源，全部为 `simulated`，真实外部企微会话存档来源为 0；因此现有 18 条洞察只能作为模拟归档链路验收，不能宣称 Phase 7 或真实企微存档完成。
2. DeepSeek 一次性真实模型调用尚未执行。该动作会把租户 1 当日模拟会话正文、规则/提示词上下文和已配置 API Key 发送给 DeepSeek，属于外部敏感数据传输；必须在执行时取得用户对这次具体外发的即时确认。配置、网络和认证运行入口均已准备好，确认后只执行一次，不开启自动日分析或 run-on-start。
3. SaaS 浏览器未取得 MFA 登录态，因此未完成 SaaS 管理端真实登录点击；没有使用绕过认证的方式伪造通过。
4. 未执行导出下载，避免在验收机生成额外业务文件；导出认证、文件名和失败恢复由自动化测试覆盖。

## 13. 提交与交付方式

本分支包含中文设计、中文实施计划、企业资料/RBAC、三页投影、目录与概览、Dashboard lint、租户 Provider 加密/API/UI、当日快照、连续性上下文、迁移中断恢复和诚实状态文案等原子提交。独立复审对实现与迁移给出 PASS，未发现 Critical、Important 或 Minor 问题。

建议由用户审阅本报告和分支 diff 后，以普通 merge 或 cherry-pick 合入；本任务没有自行合并或推送新分支。
