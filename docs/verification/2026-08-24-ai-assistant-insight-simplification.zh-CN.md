# AI 设置与 AI 洞察融合优化实施与自测报告

日期：2026-08-24
分支：`feat/ai-settings-menu-optimization`
隔离工作树：`D:\workspace\mochat-go\mochat-go\.worktrees\ai-settings-menu-optimization`

## 1. 交付结论

本轮已完成 AI 设置菜单、知识库运行链路、固定分析助手及 AI 洞察结果页的设计、实施、迁移、测试、Docker 部署和真实浏览器验收。

- AI 设置只保留“AI 知识库”和“分析助手”两个真实入口。
- “分析助手”只展示一个固定系统助手“会话分析助手”，不再提供无消费场景的新增、删除、筛选、分页或通用智能体管理。
- 会话分析与智能分析共用该助手的分析要求、启停状态和知识库；智能分析另有一条固定默认规则，但也只能在分析助手编辑器内维护。
- 智能分析页面移除规则配置，仅负责查询和展示结果；历史结果继续保留规则版本快照。
- 知识库支持真实文档上传、解析、持久化、关联、启停和删除，不使用前端假数据或伪成功。
- 规则独立写接口返回 405，相关 RBAC 写资源停用；旧自定义智能体和规则数据只为兼容历史保留，不进入当前运行链路。
- Phase 7 保持暂停，未扩展真实企微会话存档工作。

设计和实施计划分别见：

- [设计文档](../superpowers/specs/2026-08-24-ai-assistant-insight-simplification-design.zh-CN.md)
- [实施计划](../superpowers/plans/2026-08-24-ai-assistant-insight-simplification.zh-CN.md)

## 2. 路由与业务关系

| 页面 | 路由 | 职责 |
| --- | --- | --- |
| AI 知识库 | `/ai-setting/ai-knowledge-base` | 企业知识库及文档的真实增删改查、启停和解析状态 |
| 分析助手 | `/ai-setting/agent` | 固定会话分析助手、知识库关联、统一分析要求和默认智能分析规则 |
| 会话分析 | `/ai-insight/session-analysis` | 查看会话分析结果、详情、证据消息和原会话入口 |
| 智能分析 | `/ai-insight/smart-analysis` | 查询固定规则生成的结果，不提供规则配置 |

AI 设置负责“怎么分析”，AI 洞察负责“分析出了什么”。两个洞察页面都展示当前助手状态和知识文档数量，并提供返回配置的入口。

## 3. 数据、接口、迁移与安全

### 3.1 真实数据链路

- 助手读取和原子保存：`GET /dashboard/ai-settings/agents`、`PUT /dashboard/ai-settings/agents/{id}`。
- 知识库：`GET/POST /dashboard/ai-settings/knowledge-bases`、`PUT/DELETE /dashboard/ai-settings/knowledge-bases/{id}`。
- 文档：`GET/POST /dashboard/ai-settings/knowledge-bases/{id}/documents`、`DELETE /dashboard/ai-settings/knowledge-bases/{id}/documents/{documentId}`。
- 会话分析结果：`/dashboard/ai-insight/session-analysis/status|records|detail|export`。
- 智能分析结果：`/dashboard/ai-insight/smart-analysis/status|records|detail`。

助手更新、固定规则更新、不可变规则版本和审计记录位于同一数据库事务中。未变化的保存不会产生空版本；规则变化才递增版本。运行器只选择 `system_key=default-smart-analysis` 的规则，并把同一助手的要求、知识和配置指纹交给两类分析。

### 3.2 迁移

迁移 `0158_ai_assistant_default_smart_rule` 已在保留卷 MariaDB 上执行：

- 为 `mochat_go_ai_analysis_rules` 增加 `system_key` 及租户、企业内唯一索引。
- 为每个有效企业绑定补齐固定默认规则和 v1 版本。
- 将菜单权限名称更新为“分析助手”。
- 软停用独立规则创建、更新、删除和状态写资源；0158 up 校验和保持 `4408de65d9dbca04bda72320ba094f5b1e21853985c94236870323004d92eed8`。由于已部署 up 未保存逐行权限前态，down 保守地保持这些写资源关闭，不根据当前状态猜测并复活历史权限。

最终数据库核验：最新版本为 `0158`；当前企业固定规则 1 条，回看 30 天、最少 2 条消息、当前 v3；固定助手启用且无验收知识库残留；独立规则活动写资源为 0。

### 3.3 租户、权限和诚实失败

- 所有助手、规则、知识库、文档和分析查询都使用服务端主体验证后的 `tenant_id/corp_id`，前端 `corpId` 不能越权替代服务端身份。
- 修复了 RBAC 资源匹配器只识别 `{id}`、无法识别 `{documentId}` 的缺陷；现在与模块路由对任意命名动态段的语义一致，文档删除能在权限检查后进入真实处理器。
- 助手停用时，两类分析都记录真实停用/失败状态，不调用 Provider，不生成假结果。
- Provider 未就绪时仍按企业为会话分析和默认智能分析记录失败运行；助手加载、运行状态查询或失败记录持久化出错会向上返回错误，不再静默成空态或成功。
- 归档分片只忽略明确的 MySQL/MariaDB 1146 表不存在错误；连接、权限和其他 SQL 错误会终止分析并留下真实失败。候选来源窗口按最早到最晚消息保存，可完整回读证据消息。
- 新关联知识库必须处于启用状态；已关联后停用的知识库可以保留或移除，后端事务内校验防止绕过 UI、跨租户或跨企业关联。
- 最终重建后的容器把 AI Provider 如实报告为“暂不可用：尚未运行”，会话分析与智能分析即使没有历史 run 也显示该状态；本次没有强制制造外部模型调用，也不宣称未执行的外部分析通过。

## 4. 圆弧 AI 调研吸收

真实登录圆弧 AI 后逐项检查了智能体、智能分析和会话分析页面。保留了适合 MoChat 的交互：固定“会话分析助手”、集中规则参数、结果页筛选、状态摘要、版本快照、详情中的摘要/客户意向/流失风险/员工质检/证据消息/原会话入口。没有照搬其通用智能体卡片管理和视觉密度；MoChat 采用既有 Dashboard 的导航、顶栏、按钮层级、筛选区、表格、抽屉、分页、反馈与宽屏容器规范。

## 5. 真实浏览器验收

浏览器使用当前已登录 Dashboard 会话，连接重新构建后的 `http://127.0.0.1:18080`，未通过 DOM 伪造数据。

### 5.1 分析助手

- 1280×720：单卡片展示固定助手，刷新和编辑按钮高度均为 42 px。
- 编辑器同时包含助手要求、启停、知识库和唯一默认规则，没有新增/删除入口。
- 将回看天数 30 改为 29，保存后生成 v2，刷新仍为 29；最终恢复 30 并生成 v3。
- 停用助手后，智能分析页面立即显示“已停用”；最终恢复启用。
- 修改后按 Escape 会出现“放弃未保存更改”确认，再次 Escape 返回编辑器；取消、关闭和遮罩均按共享弹窗规则工作。
- 390×680、390×480 和 1600×900 均无横向溢出；窄屏按钮等宽等高，编辑区内部滚动且底部操作可达。
- 最终运行镜像再次验证卡片显示后端统计“0 个知识库 · 0 份就绪文档”，刷新与编辑按钮分别为 72×42、88×42 px，按钮高度一致。

证据：

- [桌面单卡片](evidence/2026-08-24-ai-assistant-insight/01-assistant-desktop.png)
- [集中规则编辑器](evidence/2026-08-24-ai-assistant-insight/02-assistant-rule-editor.png)
- [停用跨页联动](evidence/2026-08-24-ai-assistant-insight/03-smart-analysis-disabled.png)
- [390×680 窄屏](evidence/2026-08-24-ai-assistant-insight/07-assistant-narrow-390x680.png)
- [窄屏编辑器](evidence/2026-08-24-ai-assistant-insight/08-assistant-editor-narrow.png)
- [小高度编辑器](evidence/2026-08-24-ai-assistant-insight/09-assistant-editor-small-height.png)
- [1600×900 宽屏](evidence/2026-08-24-ai-assistant-insight/10-assistant-wide-1600x900.png)
- [最终运行镜像](evidence/2026-08-24-ai-assistant-insight/15-assistant-final-runtime.png)

### 5.2 知识库

- 新建“浏览器验收知识库-0824”，编辑说明，刷新后内容仍存在。
- 通过真实文件选择器上传 `acceptance-knowledge.md`；服务端解析为 371 B、150 字符、1 个分段并标记“可用于分析”。
- 关联助手后，会话分析和智能分析都显示 1 个已启用知识库、1 份可用文档。
- 停用知识库后两页降为 0/0，重新启用后恢复 1/1。
- 上传 `.exe` 得到明确的“仅支持 TXT、Markdown、PDF 和 DOCX 文件”错误，没有生成记录。
- 验收中发现并修复 `{documentId}` RBAC 匹配缺陷；修复后文档真实删除、计数归零，解除关联后知识库真实删除，刷新后仍不存在。
- 验收知识库、文档、私有对象和关联均已清理，未改动原有“产品知识库”“话术知识库”。

证据：

- [文档解析完成](evidence/2026-08-24-ai-assistant-insight/04-knowledge-document-ready.png)
- [不支持文件错误](evidence/2026-08-24-ai-assistant-insight/11-knowledge-upload-error.png)
- [删除后恢复原始知识库列表](evidence/2026-08-24-ai-assistant-insight/12-knowledge-clean-after-delete.png)

### 5.3 AI 洞察

- 智能分析只包含助手摘要、筛选、结果列表/空态和分页，不含规则页签、规则表单或新增按钮。
- 旧 `tab=rules&ruleVersionId=999` 参数不会恢复规则管理；执行查询后 URL 归一化为当前筛选参数。
- 智能分析的重置、刷新、查询按钮均为 68×38 px（宽×高）；会话分析同样一致。
- 两页都正确显示同一个会话分析助手及知识运行统计。
- 最终镜像在两页均明确显示“AI 服务暂不可用：尚未运行”，证明无历史 run 时也不会把 Provider 故障隐藏成普通空态。
- 当前真实结果为 0 条，因此完成了查询、重置、刷新、空态和分页验收；没有虚构结果，也没有声称未能点击的真实结果详情或外部模型写操作通过。详情、Escape、遮罩、原会话动作由相关组件自动化测试覆盖。

证据：

- [智能分析结果模式](evidence/2026-08-24-ai-assistant-insight/05-smart-analysis-result-only.png)
- [会话分析共享助手](evidence/2026-08-24-ai-assistant-insight/06-session-analysis-linked.png)
- [智能分析 Provider 不可用](evidence/2026-08-24-ai-assistant-insight/13-smart-provider-unavailable.png)
- [会话分析 Provider 不可用](evidence/2026-08-24-ai-assistant-insight/14-session-provider-unavailable.png)

## 6. 自动化与运行时验证

| 验证 | 结果 |
| --- | --- |
| `go test ./... -count=1` | 通过，包含 Dashboard、迁移、AI 设置、AI 洞察、Provider 和存储包 |
| Dashboard 全量 `vitest run` | 通过，141 个文件、838 个测试 |
| 本任务定向 Vitest | 通过；AI 洞察新增失败态相关 6 个文件 17 个测试、AI 设置页面 35 个测试，独立复审相关前端共 56 个测试通过 |
| Dashboard `typecheck` | 通过 |
| Dashboard production `build` | 通过，1789 个模块；只有既有 chunk size 提示 |
| 本任务修改文件定向 ESLint | 通过，0 错误 |
| Dashboard 全量 ESLint | 已执行；仓库既有非本任务文件仍有 86 个错误，本任务修改文件为 0；未扩大范围修改会话、风险预警等页面 |
| `check:phase4-dashboard-page-rbac` | 通过：53 页、48 个普通页、5 个超管页、0 个未映射 API，`scopeRequired=128` |
| `check:yuanhu-benchmark` | 通过：53 页 |
| `check:dashboard-all-pages-evidence` | 12/12 通过 |
| `git diff --check 11027eab...HEAD` | 通过，包含报告本身，不再有行尾空格 |
| 独立代码复审 | Critical 0、Important 0；报告提交后 Ready to merge |
| Docker | 重建最新 app 镜像并替换 Compose 容器；MariaDB、Redis 命名卷完整保留；0158 校验和通过，应用健康，`/readyz` 为 200 |

Dashboard 测试中的 `jsdom window.getComputedStyle(..., pseudoElt)` 是 Ant Design 测量滚动条时的已知 stderr，不影响测试退出码和断言结果。

## 7. 提交

本轮核心原子提交：

- `696b72d` `docs(ai): design assistant insight simplification`
- `8a7246f` `feat(ai-settings): seed one default smart rule`
- `ff4c018` `feat(ai-settings): save assistant and smart rule atomically`
- `6869ace` `feat(ai-insight): consume fixed analysis assistant`
- `297d509` `feat(dashboard): simplify analysis assistant settings`
- `f06c193` `feat(ai-insight): make smart analysis result only`
- `5b013cf` `fix(rbac): deactivate standalone smart rule writes`
- `e6bda23` `test(rbac): include assistant rule permission overlay`
- `da8782e` `fix(rbac): match named dashboard path segments`
- `2cfd886` `fix(dashboard): simplify insight filter type constraint`
- `4034ad6` `fix(ai-insight): preserve real archive windows`
- `4b9eaa8` `fix(ai-insight): surface honest analysis failures`
- `2d6ffb3` `fix(ai-settings): validate assistant knowledge links`
- `f38bffd` `fix(migration): keep disabled grants closed on rollback`
- `a77ce7c` `fix(ai-insight): propagate workspace persistence failures`
- `16a24c4` `docs(ai): clarify safe RBAC rollback boundary`

## 8. 已知限制与回滚边界

- 本次没有真实分析结果行，因此没有在浏览器中点击结果详情；自动化合同已覆盖详情和会话动作，但外部 Provider 执行结果仍需有真实归档会话后再验收。
- 当前最终容器的 AI Provider 未就绪，因此两页展示真实受限状态；没有外部模型结果可供浏览器验收。
- 全量 ESLint 的 86 个既有问题属于其他页面，已记录但未跨范围修复。
- 0158 up 已部署且未保存逐行 RBAC 前态，当前状态无法可靠区分迁移停用与历史停用。为避免安全权限复活，down 不自动恢复规则写资源；旧版本若必须恢复该能力，只能依据迁移前备份制作显式 allowlist。执行 down 前还需确认没有依赖 `system_key` 的新规则版本，且不得删除 Docker 数据卷。
- 最终浏览器登录使用了可逆的临时验收密码；登录成功后已立即恢复原密码哈希，并以数据库查询确认临时哈希残留数为 0。
