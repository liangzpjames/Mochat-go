# 双分析助手与量化洞察实施计划

> **执行要求：** 在隔离分支 `feat/ai-settings-menu-optimization` 内按测试驱动逐步执行；每个生产改动前先运行新测试并确认红灯。不得修改总进度台账、`.workbuddy/`、`tmp/` 或旧 `web/saas-admin/`。

**目标：** 将会话分析和智能分析拆成两个固定、独立配置和运行的系统助手，补齐会话双提示词、姓名筛选与量化展示，修复真实洞察写库故障，并通过受保护 DeepSeek 运行时生成持久化验收数据。

**架构：** 复用 `mochat_go_ai_agents` 的系统用途唯一键和 `mochat_go_ai_analysis_rules/*_versions` 的不可变版本链。新增会话系统规则和智能分析助手；Runner 按分析类型加载助手与规则。结果继续落 `mochat_go_ai_conversation_insights.result_json`，Schema 版本向后兼容。前端保持 AI 设置负责配置、AI 洞察负责查询和操作结果。

**技术栈：** Go、MariaDB/MySQL migration、React/TypeScript、TanStack Query、Vitest/Testing Library、Docker Compose、真实浏览器。

---

## 任务一：迁移双助手、双规则和提示词版本字段

**文件：**

- 新增：`deploy/standalone/migrations/0159_dual_analysis_assistants.up.sql`
- 新增：`deploy/standalone/migrations/0159_dual_analysis_assistants.down.sql`
- 新增：`internal/migration/dual_analysis_assistants_test.go`
- 修改：`internal/migration/migration_test.go`（如存在最新版本断言）
- 修改：`internal/modules/ai-settings/ports/repository.go`

**红灯：**

1. 断言 up migration 为当前有效 tenant/corp 幂等创建 `smart-analysis` 助手和 `session-analysis` 会话规则；规则/版本表均含两段提示词字段；智能规则展示名为“智能分析规则”。
2. 断言唯一键、非空默认值、版本引用安全 down 和 Phase 7 不被触碰。
3. 运行 `go test ./internal/migration -run 'DualAnalysis|Migration' -count=1`，确认新断言先失败。

**绿灯：**

1. 新增 0159 up/down；保留已发布 `default-smart-analysis` 数据键，不重写历史主键。
2. 会话规则默认提示词分别覆盖客户购买/流失/需求和员工质检/异议/改进。
3. Down 先保护被洞察引用的版本，不删除用户历史数据；只回滚本迁移可安全撤回的资源。
4. 运行迁移测试、`git diff --check`，提交：`feat(ai-settings): seed dual analysis assistants`。

## 任务二：后端拆分助手读取、更新和审计

**文件：**

- 修改：`internal/modules/ai-settings/ports/repository.go`
- 修改：`internal/modules/ai-settings/adapters/mysql/runtime_repository.go`
- 修改：`internal/modules/ai-settings/adapters/mysql/runtime_repository_test.go`
- 修改：`internal/modules/ai-settings/transport/http/handler.go`
- 修改：`internal/modules/ai-settings/transport/http/handler_test.go`
- 修改：`internal/modules/ai-settings/module.go`

**红灯：**

1. GET agents 必须恰好返回会话分析助手和智能分析助手。
2. PUT 根据数据库中受信任 `system_key` 分流：会话助手只接受会话规则及双提示词，智能助手只接受智能规则；客户端不能串改用途。
3. 两助手的状态、知识库、说明、设置指纹互相独立；提示词变化只生成一个新版本，无变化不增版本；审计/版本/助手任一步失败全事务回滚。
4. 跨 tenant/corp 返回 404，POST/DELETE 保持 405，提示词按 Unicode 长度和空值校验。
5. 运行目标测试，确认因合同尚未实现失败。

**绿灯：**

1. 泛化系统助手 Repository/Context，并保留必要兼容 wrapper，避免扩大调用面。
2. `Agent` 根据用途返回互斥的 `sessionAnalysisRule` 或 `smartAnalysisRule`。
3. 更新错误码映射，不泄露数据库或 Provider 内部信息。
4. 运行 `go test ./internal/modules/ai-settings/... -count=1`，提交：`feat(ai-settings): separate analysis assistant configs`。

## 任务三：修复洞察结果真实写库并建立集成保护

**文件：**

- 修改：`internal/modules/ai-insight/repository.go`
- 修改：`internal/modules/ai-insight/repository_test.go`
- 新增或修改：现有 MariaDB integration 测试文件（沿用仓库 testcontainer/保留库口径）

**根因证据：** `SaveInsight` 的 INSERT 有 27 列、25 个业务实参，但当前 VALUES 多一个占位符，保留库运行记录稳定报 MySQL 1136，22 个候选全部保存失败。

**红灯：**

1. 用真实 MariaDB 表执行 `SaveInsight` 插入成功结果，读取并逐字段比对。
2. 对相同唯一键执行 ON DUPLICATE 更新，确认状态、结果、模型、提示词版本和时间正确更新。
3. 运行测试并记录 Column count mismatch 红灯。

**绿灯：**

1. 仅修正 VALUES 的占位符数量，不捎带重构。
2. 运行目标 unit + integration 测试和一次保留库最小插入/清理验证。
3. 提交：`fix(ai-insight): persist conversation results`。

## 任务四：Runner 按类型加载助手与提示词并升级量化 Schema

**文件：**

- 修改：`internal/modules/ai-insight/contracts.go`
- 修改：`internal/modules/ai-insight/result_parser.go`
- 修改：`internal/modules/ai-insight/result_parser_test.go`
- 修改：`internal/modules/ai-insight/conversation_runner.go`
- 修改：`internal/modules/ai-insight/conversation_runner_test.go`
- 修改：`internal/modules/ai-insight/workspace_handler.go`
- 修改：`internal/modules/ai-insight/workspace_handler_test.go`

**红灯：**

1. 停用/加载失败一个助手不影响另一个；两类请求分别包含自己的说明、知识库和指纹。
2. 会话客户提示词与员工质检提示词进入各自受限区段，不能替代末尾固定 Schema/证据约束；结果记录会话规则版本。
3. 会话量化维度的分数、权重、证据 ID 越界或越源被拒绝。
4. 智能 Schema v2 的命中度、置信度、覆盖度、优先级和维度受校验；v1 历史结果仍可读且缺失不补 0。
5. status 按页面返回对应助手。

**绿灯：**

1. 会话结果增加质量分和带权重证据维度；员工质检增加未解决问题/异议结构。
2. 智能结果新请求输出 v2，解析器兼容 v1/v2。
3. Provider 请求仅在洞察结构化分析启用 JSON object 模式；持久化真实 provider/model 标识。
4. 运行 `go test ./internal/modules/ai-insight/... ./internal/modules/providers/ai/openai/... -count=1`，提交：`feat(ai-insight): quantify separate analysis flows`。

## 任务五：服务端员工选项与客户名称查询

**文件：**

- 修改：`internal/modules/ai-insight/contracts.go`
- 修改：`internal/modules/ai-insight/repository.go`
- 修改：`internal/modules/ai-insight/repository_test.go`
- 修改：`internal/modules/ai-insight/workspace_handler.go`
- 修改：`internal/modules/ai-insight/workspace_handler_test.go`
- 修改：`internal/modules/ai-insight/transport/http/routes.go`
- 修改：Dashboard 路由策略/RBAC catalog 和对应 migration 测试（仅在现有 gate 要求时）

**红灯：**

1. `filter-options` 只返回当前分析类型、tenant/corp、员工数据范围内真实有结果的员工，并支持姓名关键词与上限。
2. `customerName` 只匹配客户单聊对象；LIKE 特殊字符被正确转义。
3. keyword、customerName、employeeId 进入 SQL WHERE 和相同 COUNT，分页 total 正确；旧 employeeId/targetId 参数兼容。

**绿灯：**

1. 注册两页 filter-options GET 路由并复用现有 read 权限。
2. 将当前内存 keyword 过滤迁入参数化 SQL。
3. 运行相关 Repository/Handler/RBAC 测试，提交：`feat(ai-insight): add scoped name filters`。

## 任务六：重做两助手配置页和按钮样式

**文件：**

- 修改：`web/apps/dashboard/src/features/ai-settings/ai-settings-api.ts`
- 修改：`web/apps/dashboard/src/features/ai-settings/ai-settings-api.test.ts`
- 修改：`web/apps/dashboard/src/features/ai-settings/agent-page.tsx`
- 修改：`web/apps/dashboard/src/features/ai-settings/ai-settings-pages.test.tsx`
- 修改：`web/apps/dashboard/src/features/ai-settings/knowledge-base-page.tsx`
- 修改：`web/apps/dashboard/src/styles/index.css`
- 修改：`web/apps/dashboard/src/styles/ai-settings-layout.test.ts`

**红灯：**

1. 两个固定卡分别打开互斥配置，页面不出现“默认智能体/默认智能分析规则”。
2. 会话助手保存两段提示词；智能助手保存目标、会话范围、回看和最少消息；刷新后从 API 恢复。
3. 会话范围整卡可点击，原生 checkbox 语义、焦点、选中态和窄屏一列均存在。
4. 知识库刷新按钮与查询/重置统一 36px 高、最小宽度和 padding。
5. 未保存关闭、Escape、遮罩、保存失败、知识库失效反馈保持可用。

**绿灯：**

1. 拆分 EditorState 和提交 DTO，防止一张卡带出另一张卡字段。
2. 两卡常规桌面并排、移动端纵向；配置正文小高度可滚动。
3. 运行 AI settings 定向测试与 CSS gate，提交：`feat(dashboard): simplify dual assistant settings`。

## 任务七：重做查询与量化结果展示

**文件：**

- 修改：`web/apps/dashboard/src/features/ai-insight/ai-insight-workspace-api.ts`
- 修改：`web/apps/dashboard/src/features/ai-insight/ai-insight-workspace-api.test.ts`
- 修改：`web/apps/dashboard/src/features/ai-insight/ai-insight-url-state.ts`
- 修改：`web/apps/dashboard/src/features/ai-insight/ai-insight-url-state.test.ts`
- 修改：`web/apps/dashboard/src/features/ai-insight/session-analysis-page.tsx`
- 修改：`web/apps/dashboard/src/features/ai-insight/session-analysis-page.test.tsx`
- 修改：`web/apps/dashboard/src/features/ai-insight/smart-analysis-page.tsx`
- 修改：`web/apps/dashboard/src/features/ai-insight/smart-analysis-page.test.tsx`
- 修改：`web/apps/dashboard/src/features/ai-insight/ai-insight-workspace.tsx`
- 修改：`web/apps/dashboard/src/features/ai-insight/ai-insight-workspace.test.tsx`
- 修改：`web/apps/dashboard/src/styles/ai-insight-workspace.css`

**红灯：**

1. 员工组合框显示姓名但请求/URL 写稳定 ID；输入搜索防抖并受错误态保护；刷新后回显所选姓名。
2. 客户名称独立写入 URL/API；重置清空；日期与分页兼容。
3. 会话列表显示购买意向、流失风险、员工质检三指标；详情按客户分析、员工质检、证据消息分区，维度与证据可读。
4. 智能列表/详情显示 v2 四项量化数据；v1/缺失显示“历史结果无综合分”或“证据不足”，不补 0。
5. 两页状态条分别显示对应助手，失败详情、遮罩、Escape、原会话跳转保持通过。

**绿灯：**

1. 添加页面内员工搜索选择器和独立详情组件，避免共用 Drawer 抹平业务差异。
2. CSV 同步增加量化列，历史行留空。
3. 运行所有 AI insight 前端测试，提交：`feat(dashboard): show quantified AI insights`。

## 任务八：受保护 DeepSeek 配置与真实验收数据

**文件：**

- 修改：`cmd/mochat-go/ai_debt_clearance.go`
- 修改：`cmd/mochat-go/ai_provider_runtime_test.go`
- 修改：`deploy/standalone/docker-compose.yml`
- 修改：`internal/archivesim/simulator.go` 及测试（仅为每批次专用、可清理验收身份所需）
- repo 外：Docker secret 文件和 ignored `.env.local` 路径配置（不得提交）

**红灯：**

1. `MOCHAT_GO_AI_PROVIDER_KEY_FILE` 文件优先、旧 key env fallback；缺失/空文件诚实 limited；测试不打印 secret。
2. archive simulator 每 batch 创建明确“AI验收”专用员工/客户/群，apply 幂等，cleanup 只删除该 batch 登记数据。

**绿灯与运行：**

1. 密钥写入仓库外、Windows ACL 限当前用户/SYSTEM 的文件；Compose 只挂载 secret，`docker inspect` 不出现密钥。
2. 配置官方 `https://api.deepseek.com` 和 `deepseek-v4-flash`，仅打开本次 one-shot/run-on-start 所需开关，完成后关闭自动启动，保留 Provider 配置供用户测试。
3. 用 `mochat-archive-simulator apply --corp-id 1 --batch ai_insight_accept_20260824 --enable-simulation` 写入真实归档链；触发一次 Runner。
4. 数据库验证 session/smart run 成功、result JSON 量化字段、证据 ID、provider/model、规则版本；API 和浏览器刷新验证持久化。
5. 不执行宽泛清理；为让用户继续查看，保留明确标记的验收数据，并在报告写出精确清理方法。
6. 提交仅包含代码/测试/compose secret 接线，不包含密钥：`feat(runtime): support protected AI provider secret`。

## 任务九：全量验证、浏览器证据和报告

**自动化：**

1. 相关 Go：`go test ./internal/modules/ai-settings/... ./internal/modules/ai-insight/... ./internal/modules/providers/ai/openai/... ./internal/migration/... -count=1`。
2. Go 全量：`go test ./... -count=1`。
3. Dashboard 目标测试与全量测试；按仓库脚本运行 `typecheck`、任务范围 lint、production build。
4. 运行权限/RBAC、benchmark、页面 evidence、架构边界等现有门禁。
5. `git diff --check`、`git status --short`；检查仓库与提交中不存在密钥前缀或 secret 内容。

**Docker/浏览器：**

1. 保留卷执行 0159 migration、健康检查、真实登录。
2. 每个 AI 设置/洞察页面验证查询、编辑、启停、上传/关联、结果详情、失败反馈、刷新持久化、遮罩/Escape/关闭。
3. 尺寸至少覆盖常规桌面、2560 宽屏、390 窄屏、1366×620 小高度；保存截图与网络/控制台证据。
4. 不对未执行的 Provider 外部写操作或真实企微归档宣称通过。

**报告与收尾：**

- 新增 `docs/verification/2026-08-24-dual-analysis-agents-quantified-results.zh-CN.md`，列出提交、路由、迁移、命令结果、浏览器证据、测试数据、已知限制和密钥轮换提醒。
- 请求独立代码审查，处理高优先级问题后重新运行新鲜验证。
- 将报告与证据原子提交；最终确认分支提交清晰、主工作树未受污染。
