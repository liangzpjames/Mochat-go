# AI 设置双助手与量化洞察最终实施、自测报告

日期：2026-08-24
分支：`feat/ai-settings-menu-optimization`
隔离工作树：`D:\workspace\mochat-go\mochat-go\.worktrees\ai-settings-menu-optimization`

## 1. 交付结论

本任务已完成 AI 设置菜单、知识库文档能力、两个固定分析助手，以及 AI 洞察结果页的设计、实现、迁移、真实 Provider 数据生成和浏览器验收。

- AI 设置保留两个有真实业务落点的入口：“AI 知识库”和“分析助手”。
- 分析助手固定为“会话分析助手”和“智能分析助手”，不开放无消费位置的新增、删除或改名。
- 会话分析助手独立维护客户分析提示词、员工质检提示词和会话范围；智能分析助手独立维护智能分析目标、会话范围、回看天数和最少消息数。两者的启停、说明、知识库和版本互不串改。
- AI 洞察只负责查询、量化结果、详情证据和对会话的既有操作；规则不再散落在结果页。
- 员工筛选使用真实姓名选项，两个结果页均支持客户名称搜索和 URL 查询状态，不要求用户记忆员工 ID。
- 知识库支持真实文件上传、解析、分段、关联、启停和删除；被助手引用时会拒绝删除，不以静态数组或伪成功代替后端链路。
- 会话分析和智能分析结果均使用受校验的量化 Schema，并保存 Provider、模型、提示词版本和证据消息。
- Phase 7 保持暂停，未扩展真实企微会话存档。

本轮增量设计和计划：

- [双分析助手与量化洞察设计](../superpowers/specs/2026-08-24-dual-analysis-agents-quantified-results-design.zh-CN.md)
- [双分析助手与量化洞察实施计划](../superpowers/plans/2026-08-24-dual-analysis-agents-quantified-results.zh-CN.md)
- [知识库运行链路设计](../superpowers/specs/2026-08-23-ai-settings-knowledge-runtime-design.zh-CN.md)

## 2. 页面、交互和数据职责

| 页面 | 路由 | 当前职责 | 数据来源 |
| --- | --- | --- | --- |
| AI 知识库 | `/ai-setting/ai-knowledge-base` | 知识库和文档增删改查、启停、解析状态 | 知识库/文档/分段表及私有文件存储 |
| 分析助手 | `/ai-setting/agent` | 两个固定助手的独立配置、提示词、范围和知识关联 | 助手、规则、不可变版本、审计表 |
| 会话分析 | `/ai-insight/session-analysis` | 客户名称/员工姓名筛选、量化客户分析与员工质检、证据详情 | 归档消息、会话规则版本、洞察结果表 |
| 智能分析 | `/ai-insight/smart-analysis` | 客户名称/员工姓名筛选、命中/置信度/优先级/证据覆盖率及行动建议 | 归档消息、智能规则版本、洞察结果表 |

AI 设置回答“如何分析”，AI 洞察回答“分析出了什么”。配置页提供跳转到对应结果页的入口，结果页提供返回对应助手设置的入口，但不重复放置规则编辑器。

## 3. 圆弧 AI 调研吸收

在登录态真实点击圆弧 AI 的智能体、会话分析和智能分析页面，检查了页面层级、筛选、详情、状态、空态和窄屏表现。吸收了以下适合 MoChat 的结构：

- 会话分析用客户名称、跟进人姓名、日期和量化区间查询，不暴露内部员工 ID。
- 客户分析与员工质检分区展示，分数、维度、问题、建议均能回溯到证据消息。
- 智能分析以规则命中、可信度、优先级和建议动作表达结果，不把配置与结果混为一页。
- 少量固定用途助手使用紧凑卡片和集中编辑器，避免通用智能体列表的筛选、分页和批量操作负担。

视觉没有机械复制圆弧：按钮、筛选区、卡片、表格、抽屉、反馈、间距和宽屏容器沿用本仓库已验收 Dashboard 规范。

## 4. 后端、迁移和安全边界

### 4.1 主要接口

- 助手：`GET /dashboard/ai-settings/agents`、`PUT /dashboard/ai-settings/agents/{id}`。
- 知识库：`GET/POST /dashboard/ai-settings/knowledge-bases`、`PUT/DELETE /dashboard/ai-settings/knowledge-bases/{id}`。
- 文档：`GET/POST /dashboard/ai-settings/knowledge-bases/{id}/documents`、`DELETE /dashboard/ai-settings/knowledge-bases/{id}/documents/{documentId}`。
- 会话分析：`status`、`records`、`detail`、`filter-options`、`export`。
- 智能分析：`status`、`records`、`detail`、`filter-options`。

员工选项、结果列表和详情均由服务端主体的 `tenant_id/corp_id` 与员工数据范围约束。新增的两条 `filter-options` 权限资源要求 scope，未把姓名检索变成越权目录接口。

### 4.2 增量迁移

- `0155_ai_settings_integrity_audit`：AI 设置一致性审计和助手知识库读取权限。
- `0156_ai_settings_knowledge_runtime`：知识文档、分段、私有对象和文档 API 权限。
- `0157_ai_settings_document_count_reconcile`：真实文档统计修复。
- `0158_ai_assistant_default_smart_rule`：固定智能规则及独立规则写入口停用。
- `0159_dual_analysis_assistants`：两个系统助手、两类规则和双提示词版本。
- `0160_ai_settings_knowledge_collation`：知识关联字符集/排序规则一致性。
- `0161_ai_insight_filter_options_rbac`：两个姓名筛选接口的 scoped RBAC 资源。

保留卷最终校验和：`0159=6b6ea829…`、`0160=0467a993…`、`0161=41fcf8fc…`。0161 的 up 文件保持首次发布字节不变；RBAC 门禁通过专用覆盖解析器识别其旧式 SQL，避免修改已应用迁移导致 checksum drift。

0155 down 为非破坏性保留；0156、0158 down 会明确拒绝可能破坏文档或历史规则追溯的回滚。不得通过删除 Docker 数据卷回滚。迁移前备份位于 `D:\workspace\mochat-go\output\docker-backups\ai-settings-20260824`。

### 4.3 Provider 与错误边界

- Provider 凭证只通过宿主机 ACL 保护文件挂载到 `/run/secrets`，容器环境变量中的 key 为空；仓库和镜像构建上下文不保存明文。
- Provider 非 2xx 错误只返回 HTTP 状态，不读取或持久化响应正文，避免第三方错误体泄露秘密。
- 助手停用、Provider 不可用、模型响应不符合 Schema、数据库写入失败均诚实失败，不生成伪成功结果。
- 每日分析和容器启动自动分析均关闭，避免验收后产生非预期外部调用。
- 最终 Docker 镜像 manifest digest 为 `sha256:d9ec6ae31f92dd5654f3f6da7315dc45f1a240758569aadacd19957dcfca2332`；app、MySQL、Redis 均使用原命名卷，`/healthz` 和 `/readyz` 为 200。

## 5. 真实数据与量化结果

使用既有归档模拟链路生成明确标识的验收会话，再由受保护的 DeepSeek `deepseek-v4-flash` 真实分析并持久化。未在前端写入假数组。

- 会话运行 `id=8`：2 个候选、2 个成功、0 个失败；结果 `id=39–40`。
- 智能运行 `id=9`：2 个候选、2 个成功、0 个失败；结果 `id=41–42`。
- 直接会话结果 `id=40`：客户质量 85、购买意向 80、流失风险 20、员工质检 90，并包含维度、问题、建议和证据消息。
- 智能结果 `id=42`：规则命中 85、置信度 80、优先级 85、证据覆盖率 90，并包含分维度原因、行动建议和证据消息。
- 群聊样本信息不足时，详情明确提示证据不足，没有强行制造确定结论。

## 6. 真实浏览器验收

浏览器连接保留卷 Docker 的 `http://127.0.0.1:18080`，使用真实已登录 Dashboard 会话。

### 6.1 AI 知识库

- 刷新、查询、重置按钮视觉高度一致。
- 新建临时知识库、编辑、刷新后持久化、停用、启用和删除均成功。
- 在“话术知识库”上传临时文档后真实解析，再删除文档。
- “产品知识库”已有可用文档：635 B、252 个字符、1 个分段。
- 删除被助手引用的知识库会收到“仍被 3 个智能体引用”的明确拒绝反馈。

### 6.2 分析助手

- 页面只展示“会话分析助手”和“智能分析助手”两张固定用途卡，没有新增、删除、改名、筛选或分页。
- 两个编辑器的字段、知识关联和版本互不串改；会话助手显示客户分析/员工质检双提示词，智能助手显示独立分析目标。
- 会话范围使用整卡可点击的单聊/群聊选择，选中、焦点和窄屏换行状态清晰。
- 未保存修改时按 Escape 会二次确认；继续编辑、放弃修改、遮罩和关闭均按预期工作。
- 回看天数输入 `1.5` 时保存禁用，避免后端才暴露整数合同错误。
- 保存后刷新仍保留配置；最终会话助手为 v3、智能助手为 v5。

### 6.3 AI 洞察

- 两页均可按员工姓名搜索真实选项，并可按客户名称筛选；查询状态同步到 URL。
- 会话详情展示质量、购买意向、流失风险、员工质检四项量化分数，以及维度、问题、证据、Provider、模型和提示词版本；Escape 可关闭。
- 智能详情展示命中度、置信度、优先级、证据覆盖率、维度、行动建议和证据；Escape 可关闭。
- 停止 app 容器后点击刷新，界面显示中文“服务暂时不可用，请稍后刷新重试”，而不是英文 `Failed to fetch` 或伪空态；恢复容器后数据重新可见。
- 最终镜像重建使旧登录令牌失效后，仅在本地验收管理员上临时替换密码哈希；登录成功即恢复原哈希，数据库逐值核验恢复成功且没有保留临时凭据。

### 6.4 响应式

在常规桌面宽度和真实 766×678 窄视口检查四个页面。页面在 766 px 时触发窄屏布局，`scrollWidth=751 < innerWidth=766`，无横向溢出；小高度下编辑器内容可滚动、底部操作可达。随后恢复主窗口最大化（1431×1272）。

既有截图证据目录：

- [AI 设置与洞察证据](evidence/2026-08-24-ai-assistant-insight/)

## 7. 自动化验证

| 命令/门禁 | 最终结果 |
| --- | --- |
| `go test ./... -count=1` | 通过，67 个含测试包全部通过 |
| AI 洞察真实 MariaDB integration | 3/3 通过，包含真实保存/更新和员工姓名检索 |
| Dashboard `vitest run` | 141 个文件、836 个测试全部通过 |
| `corepack pnpm typecheck` | 全工作区通过 |
| Dashboard production build | 通过，1789 个模块；仅既有 chunk size 提示 |
| 最终 Docker build / health | 通过；新镜像已替换，`healthz=200`、`readyz=200`，保留卷完整 |
| 分支改动 lint（基线 `23c50a8`） | 22 个 Dashboard TS/TSX 文件，0 错误 |
| Dashboard 全量 ESLint | 已执行；分支与本地 main 均为 86 个既有错误，本任务未新增，不跨范围修复 |
| `check:yuanhu-benchmark` | 53 页通过 |
| `check:dashboard-all-pages-evidence` | 12/12 通过 |
| Dashboard RBAC catalog | 53 页、48 个普通页、5 个超管页、0 个未映射 API；37 项脚本测试通过 |
| `git diff --check` | 通过 |
| 独立代码评审 | `APPROVED`，无新的 Critical/Important |

Dashboard 测试中 Ant Design 在 jsdom 调用伪元素 `getComputedStyle` 会输出既有 stderr，但全部断言和退出码通过。导航搜索用例此前在授权菜单异步加载前操作输入框，已增加明确等待，并在全量 836 项中稳定通过。

## 8. 提交与已知限制

本分支从设计、计划、后端、迁移、前端、测试到报告均拆为原子提交。最终安全收尾提交包括：

- `2e75402`：保留既有筛选权限资源。
- `c514837`：本地化 AI 洞察网络错误。
- `0033d6a`：Provider 错误脱敏与安全回滚边界。
- `64a96aa`：记录导出清洗器的 lint 例外。
- `264d198`：保持已应用 0161 迁移校验和不变。
- `9975f32`：稳定等待授权导航后再测试搜索。

已知限制：

- 验收消息是明确标识的模拟归档数据，分析调用和结果持久化是真实链路；不代表 Phase 7 真实企微存档已恢复。
- 用户自建智能体仍关闭；只有未来出现清晰业务消费位置后才应扩展。
- 历史隐藏智能体仍可能引用知识库，因此“产品知识库”当前显示 3 个引用；这是持久化事实，不应由前端伪装清零。
- 用户曾在对话中提供 Provider key；尽管代码、提交、日志和容器环境变量均未保存明文，仍建议在验收后轮换该 key。

## 9. 后续增量：横向编辑器与新建知识库可选上传

### 9.1 用户反馈与最终设计

针对“分析助手编辑弹窗横向展示更多信息”和“新建知识库可同时选择非必填文档”的后续反馈，本次新增：

- 两个固定分析助手继续使用弹窗交互，桌面端弹窗宽度上限为 1180px，内部为十二列网格；基础信息、关联知识库、会话范围/智能分析规则三块同排。
- 会话分析助手的客户分析提示词、员工质检提示词在桌面端并排；980px 以下主要区块改两列，768px 以下全部改为单列。
- 新建知识库增加“初始文档（可选）”，支持现有 `.txt`、`.md`、`.pdf`、`.docx` 合同，文件不参与必填校验。
- 保存复用既有真实接口，顺序执行“创建知识库 → 有文件时上传文档”。知识库创建成功但上传/解析失败时保留知识库，并明确提示到“管理文档”重试或处理；不自动删除、不伪装完整成功。
- 文件选择纳入未保存状态，Escape、遮罩、取消和保存中禁止关闭继续沿用共享弹窗语义。

增量设计与计划：

- [增量设计](../superpowers/specs/2026-08-24-ai-settings-editor-upload-followup-design.zh-CN.md)
- [增量实施计划](../superpowers/plans/2026-08-24-ai-settings-editor-upload-followup.zh-CN.md)

### 9.2 自动化与门禁

| 命令/门禁 | 新鲜结果 |
| --- | --- |
| 新增测试红灯 | 6 个预期失败，分别命中横向布局、可选文件、顺序上传、部分失败和脏状态 |
| AI 设置目标 Vitest | 2 个文件、23 个测试全部通过 |
| Dashboard 全量 Vitest | 141 个文件、842 个测试全部通过 |
| `pnpm typecheck` | 12 个工作区项目全部通过 |
| 变更范围 lint（基线 `23c50a8`） | 22 个 Dashboard TS/TSX 文件，0 错误 |
| Dashboard production build | 通过，1789 个模块；只有既有 chunk size 提示 |
| `go test ./... -count=1` | 全部通过 |
| AI 设置/AI 洞察相关 Go 包 | 全部通过 |
| Yuanhu benchmark / 页面 evidence | 53 页通过；evidence 12/12 通过 |
| RBAC catalog + completion | 47/47 脚本测试通过；53/48/5、`scopeRequired=130` |
| Docker build / health | 新镜像 `sha256:00730633…`；app/MySQL/Redis healthy，`healthz=200`、`readyz=200` |
| 数据卷 | `mysql-data`、`redis-data`、`app-storage`、`audit-anchor-storage` 全部保留 |

验收中发现 RBAC completion gate 的有效种子链只读取到 0158，漏掉已存在的 0161 AI 洞察筛选权限覆盖；独立 catalog 已应用 0161，因此两者口径不一致。已补齐 completion gate 对同一覆盖解析器的调用，并由 `runCompletionGate` 集成测试验证，不修改已应用迁移字节。

### 9.3 真实浏览器验收

- 常规 1280×720：会话分析助手弹窗实际宽度 1160px，十二列网格，三个主要区块均为 356px 且同一纵坐标；两类提示词各 527px 并排；无页面横向溢出。
- 智能分析助手同样为 1160px，三个主要区块同排，无横向溢出。
- 766×678：编辑器自动回落单列，内容区 `clientHeight=428`、`scrollHeight=1133`、`overflow-y=auto`，底部保存/取消按钮保持可见，页面无横向溢出。
- 新建“验收知识库-0824-2136”时选择 `acceptance-knowledge.md`，真实创建和文档解析成功；刷新后知识库仍为 1 份文档，“管理文档”显示“可用于分析”。该验收数据按用户要求保留，便于继续测试。
- 新建弹窗的文件输入在真实 DOM 中 `required=false`；不选文件的创建和不调用上传接口由自动化用例覆盖。
- 浏览器控制台最终没有 error 或 warning；临时视口已恢复到 1280×720。

新增证据：

- [桌面横向助手编辑器](evidence/2026-08-24-ai-assistant-insight/16-assistant-horizontal-editor.png)
- [窄屏助手编辑器](evidence/2026-08-24-ai-assistant-insight/17-assistant-editor-followup-narrow.png)
- [知识库文档解析并持久化](evidence/2026-08-24-ai-assistant-insight/18-knowledge-create-with-optional-file.png)
- [新建知识库可选初始文档](evidence/2026-08-24-ai-assistant-insight/19-knowledge-create-optional-file.png)

### 9.4 增量提交与运行状态

- `6e3018a`：增量设计。
- `3070b9d`：增量实施计划。
- `94d6da6`：横向助手编辑器与新建知识库可选上传实现/测试。
- `70fea07`：RBAC completion gate 纳入 0161 筛选权限覆盖。

用户要求取消关机后已执行系统取消命令；系统确认当时没有待执行的关机计划，此后未再安排关机。AI 日分析和启动即分析仍保持关闭。
