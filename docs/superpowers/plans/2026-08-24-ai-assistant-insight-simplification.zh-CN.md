# AI 分析助手与洞察工作台精简实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**目标：** 将一个固定“会话分析助手”、唯一默认智能分析规则、知识库和两个 AI 洞察结果页连成真实、可追溯且可验收的完整链路。

**架构：** 以 `mochat_go_ai_agents` 为设置聚合入口，以带系统键的 `mochat_go_ai_analysis_rules` 为唯一智能分析规则；AI Settings 的一次 PUT 在同一事务更新助手和规则版本。AI Insight Runner 仅消费系统默认规则，并让会话分析与智能分析共享助手上下文；前端设置页负责配置，洞察页只负责结果消费。

**技术栈：** Go、MySQL 8、React 19、TypeScript、TanStack Query、Vitest/Testing Library、Docker Compose、浏览器真实验收。

## 全局约束

- 不修改 `docs/PROJECT_PROGRESS.zh-CN.md`、`.workbuddy/`、`tmp/`、调试脚本、旧 `web/saas-admin/` 和无关差异。
- 不 force push、不 reset/clean、不删除 Docker 数据卷、不恢复 Phase 7。
- 所有用户审阅文档使用中文；所有业务数据来自真实接口和持久化链路。
- 每项行为变更必须先添加会因缺失行为而失败的测试，再写最小实现。
- 路由保持 `/ai-setting/agent`、`/ai-insight/session-analysis`、`/ai-insight/smart-analysis` 兼容。

---

### 任务 1：固化唯一系统规则与迁移合同

**文件：**

- 新建：`deploy/standalone/migrations/0158_ai_assistant_default_smart_rule.up.sql`
- 新建：`deploy/standalone/migrations/0158_ai_assistant_default_smart_rule.down.sql`
- 新建：`internal/migration/ai_assistant_default_smart_rule_test.go`
- 修改：`internal/dashboard/health.go`
- 修改：相关迁移版本/数量测试文件

**接口：**

- 产生：`system_key='default-smart-analysis'` 的每企业唯一规则及 v1 版本。
- 产生：菜单显示名“分析助手”；移除规则独立写权限资源。

- [ ] 写迁移合同失败测试：检查列、唯一索引、幂等种子、版本种子、菜单改名、权限收敛和 down 回滚边界。
- [ ] 运行精确 Go 测试并确认因缺少 0158 文件/版本而失败。
- [ ] 编写 up/down SQL，更新健康检查期望版本与数量。
- [ ] 运行迁移与健康相关测试并确认通过。
- [ ] 提交 `feat(ai-settings): seed one default smart rule`。

### 任务 2：将助手和默认规则作为一个原子设置合同

**文件：**

- 修改：`internal/modules/ai-settings/ports/repository.go`
- 修改：`internal/modules/ai-settings/adapters/mysql/runtime_repository.go`
- 修改：`internal/modules/ai-settings/adapters/mysql/runtime_repository_test.go`
- 修改：`internal/modules/ai-settings/transport/http/handler.go`
- 修改：`internal/modules/ai-settings/transport/http/handler_test.go`
- 修改：`web/apps/dashboard/src/features/ai-settings/ai-settings-api.ts`
- 修改：`web/apps/dashboard/src/features/ai-settings/ai-settings-api.test.ts`

**接口：**

- `Agent.smartAnalysisRule`：`id/name/objective/conversationTypes/lookbackDays/minimumMessages/currentVersion/updatedAt`。
- `AgentInput.smartAnalysisRule`：可编辑 `objective/conversationTypes/lookbackDays/minimumMessages`。
- `PUT /dashboard/ai-settings/agents/{id}`：原子更新助手、规则和不可变版本。

- [ ] 先写 Repository 测试：GET 聚合默认规则；PUT 字段变化递增版本；未变化不增版；任一步失败整体回滚；跨租户/企业不可更新。
- [ ] 运行精确测试，确认因缺少聚合字段和事务逻辑而失败。
- [ ] 扩展 ports 与 MySQL Repository，实现聚合读取和原子保存。
- [ ] 先写 HTTP 测试：校验会话范围、回看天数、最少消息数，错误返回稳定机器码。
- [ ] 运行 HTTP 测试，确认预期失败后实现输入校验与 JSON 合同。
- [ ] 先写前端 API 解析/请求测试，确认缺少字段失败后扩展 TypeScript 合同。
- [ ] 运行 AI Settings 相关 Go/前端测试并确认通过。
- [ ] 提交 `feat(ai-settings): save assistant and smart rule atomically`。

### 任务 3：让执行链只消费默认规则和同一助手上下文

**文件：**

- 修改：`internal/modules/ai-insight/contracts.go`
- 修改：`internal/modules/ai-insight/repository.go`
- 修改：`internal/modules/ai-insight/conversation_runner.go`
- 修改：`internal/modules/ai-insight/conversation_runner_test.go`
- 修改：`internal/modules/ai-insight/workspace_handler.go`
- 修改：`internal/modules/ai-insight/workspace_handler_test.go`

**接口：**

- `EnabledRuleVersions` 只返回 `default-smart-analysis` 当前版本。
- 两类分析都接收 `SessionAssistantContext`，设置指纹进入去重指纹。
- session/smart status 都返回同一助手摘要。

- [ ] 写 Runner 失败测试：只执行一个系统规则；助手停用时两类分析都不调用 Provider；智能分析提示词包含助手要求和知识库；设置变化改变智能分析指纹。
- [ ] 运行精确测试并确认按缺失行为失败。
- [ ] 修改规则查询和 Runner，最小实现上述行为。
- [ ] 写 Workspace status 失败测试：智能分析也返回助手摘要。
- [ ] 实现 status 聚合，并将旧规则 POST/PUT/DELETE/status 写入口收敛为 405。
- [ ] 运行 AI Insight 包测试并确认通过。
- [ ] 提交 `feat(ai-insight): consume the fixed analysis assistant`。

### 任务 4：精简分析助手设置页

**文件：**

- 修改：`web/apps/dashboard/src/features/ai-settings/agent-page.tsx`
- 修改：`web/apps/dashboard/src/features/ai-settings/ai-settings-pages.test.tsx`
- 修改：`web/apps/dashboard/src/styles/index.css`
- 修改：`web/apps/dashboard/src/styles/ai-settings-layout.test.ts`

**接口：**

- 页面只渲染一个助手卡片和一个编辑弹窗。
- 保存一次调用 `updateAgent`，包含基础设置、知识库和默认规则。

- [ ] 写页面失败测试：没有 KPI/筛选/分页/新增/删除；显示两个使用场景、规则摘要、知识库摘要；打开编辑可修改默认规则并一次保存。
- [ ] 运行页面测试并确认旧通用管理 UI 导致失败。
- [ ] 重写 `AgentPage` 为固定卡片和分区弹窗，保留加载、空态、错误、成功反馈、脏数据确认、Escape/遮罩关闭合同。
- [ ] 写布局失败测试：卡片自适应、窄屏单列、小高度弹窗滚动、刷新与编辑按钮等高。
- [ ] 实现最小 CSS 并运行目标页面/布局测试。
- [ ] 提交 `feat(dashboard): simplify analysis assistant settings`。

### 任务 5：将智能分析收敛为结果工作台并关联助手

**文件：**

- 修改：`web/apps/dashboard/src/features/ai-insight/smart-analysis-page.tsx`
- 修改：`web/apps/dashboard/src/features/ai-insight/smart-analysis-page.test.tsx`
- 修改：`web/apps/dashboard/src/features/ai-insight/ai-insight-url-state.ts`
- 修改：`web/apps/dashboard/src/features/ai-insight/ai-insight-url-state.test.ts`
- 修改：`web/apps/dashboard/src/features/ai-insight/ai-insight-workspace-api.ts`
- 修改：`web/apps/dashboard/src/styles/ai-insight-workspace.css`
- 修改：相关样式合同测试

**接口：**

- URL 只保存结果筛选与页码，不再保存 `tab` 和 `ruleVersionId`。
- 页面不调用任何规则 CRUD API。
- smart status 的 `assistant` 用于关联条和设置深链。

- [ ] 写页面失败测试：不存在规则页签/新增/编辑/启停/删除/规则版本筛选；存在助手关联条、查询、重置、刷新、结果、详情和会话跳转。
- [ ] 运行精确测试并确认旧页面导致失败。
- [ ] 删除规则管理状态/组件/调用，简化 URL 状态和 API 类型。
- [ ] 写按钮样式失败测试：查询、重置、刷新共享 36px 最小高度与一致 padding。
- [ ] 实现样式，运行 Smart/Session/Workspace/API/URL 测试并确认通过。
- [ ] 提交 `feat(ai-insight): make smart analysis result-only`。

### 任务 6：更新菜单、权限基准和中文文档

**文件：**

- 修改：`internal/dashboard/dashboard_page_catalog.json`
- 修改：`web/apps/dashboard/src/benchmark/manifest.json`
- 修改：相关 navigation/benchmark/RBAC/evidence 测试与快照
- 新建：`docs/verification/2026-08-24-ai-assistant-insight-simplification.zh-CN.md`

**接口：**

- 菜单名：“分析助手”；路由与权限 code 不变。

- [ ] 写/更新菜单合同测试，先确认旧名称导致失败。
- [ ] 更新 catalog、manifest 和必要的权限/evidence 期望。
- [ ] 运行 benchmark、RBAC、page evidence 门禁。
- [ ] 编写中文实施与自测报告骨架，记录设计、提交、路由、接口、迁移、限制和待验证项。
- [ ] 提交 `docs(ai): align assistant and insight navigation`。

### 任务 7：新鲜验证、Docker 迁移与真实浏览器验收

**文件：**

- 修改：`docs/verification/2026-08-24-ai-assistant-insight-simplification.zh-CN.md`

**接口：** 无新增；验证完整交付。

- [ ] 运行相关前后端目标测试并记录通过数量。
- [ ] 运行 Dashboard 全量测试、typecheck、目标 lint、全量 lint、production build。
- [ ] 运行相关 Go 包和 `go test ./... -count=1`。
- [ ] 运行迁移、权限/RBAC、benchmark、page evidence 门禁和 `git diff --check`。
- [ ] 复用 `mochat-go-desktop` 保留卷 Compose，仅重建 app；执行 0158 migration，确认 app/MySQL/Redis 健康与 `/readyz`。
- [ ] 真实登录态验证 `/ai-setting/agent`：读取、编辑、保存、刷新持久化、启停、失败反馈、脏数据关闭、遮罩、Escape、常规桌面、2560×1440、390×680 和小高度。
- [ ] 真实登录态验证 `/ai-insight/smart-analysis`：无规则配置入口、查询/重置/刷新、空态或真实结果、详情/会话跳转、助手关联条、按钮实测等高、错误态和刷新后 URL。
- [ ] 回归 `/ai-insight/session-analysis` 的助手关联、结果详情和会话跳转；不对未实际执行的外部 Provider 写操作宣称通过。
- [ ] 将证据、命令、结果、容器健康、数据卷状态和已知限制写入报告，运行文档占位符扫描。
- [ ] 提交 `docs(verification): report assistant insight acceptance`。
- [ ] 确认工作树干净并记录分支/HEAD，然后安排 Windows 延时关机并立即向用户交付结果与取消命令。
