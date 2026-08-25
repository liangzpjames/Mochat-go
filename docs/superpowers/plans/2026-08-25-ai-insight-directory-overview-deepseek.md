# AI 洞察目录覆盖、概览同源与 DeepSeek 实跑 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将三个 AI 洞察页接入权威员工/客户目录，把数据概览切到同一会话洞察聚合，并用 DeepSeek 对保留卷真实可分析会话执行一次受控分析。

**Architecture:** `ai-insight` repository 新增目录选项与覆盖查询，Workspace API 向三个专用页面返回员工、客户和覆盖数；结果列表仍只投影真实会话洞察。`reporting` 直接聚合 `mochat_go_ai_conversation_insights` 的结构化 JSON，与专用页共享租户、企业、时间和员工范围。

**Tech Stack:** Go 1.24、MariaDB 10.6、React 19、TypeScript、TanStack Query、Vitest、Testing Library、Docker Compose、OpenAI-compatible DeepSeek API。

## Global Constraints

- 不修改总进度台账，不推送或合入新开发分支。
- 不删除 Docker 数据卷，不改写已应用迁移。
- DeepSeek Key 只通过临时受限文件和 Docker secret 使用，不进入 Git、数据库、日志、文档或前端。
- 自动日分析、启动即分析和 Phase 7 保持关闭。
- 行为修改必须先有正确失败的测试，再做最小实现。

---

### Task 1: 固化目录选项与权限合同

**Files:**
- Modify: `internal/modules/ai-insight/contracts.go`
- Modify: `internal/modules/ai-insight/repository.go`
- Modify: `internal/modules/ai-insight/repository_test.go`
- Modify: `internal/modules/ai-insight/repository_integration_test.go`
- Modify: `internal/modules/ai-insight/workspace_handler.go`
- Modify: `internal/modules/ai-insight/workspace_handler_test.go`

**Interfaces:**
- Produces: `DirectoryOptions(ctx, DirectoryOptionFilter) (DirectoryOptions, error)`，返回有效员工、有效客户及目录/洞察覆盖数。

- [ ] 写 repository 和 handler 失败测试：员工从 `mc_work_employee status=1` 返回、客户从有效关系返回、受限员工范围闭合、客户 ID 精确过滤。
- [ ] 运行 `go test ./internal/modules/ai-insight -run 'Directory|FilterOptions|CustomerID' -count=1`，确认因目录接口缺失失败。
- [ ] 实现最小 SQL、合同、响应与 `customerId` 参数校验。
- [ ] 重跑目标测试并运行 `go test ./internal/modules/ai-insight/... -count=1`。
- [ ] 提交 `feat(ai): 接入权威员工客户目录`。

### Task 2: 三页接入客户选择与覆盖摘要

**Files:**
- Modify: `web/apps/dashboard/src/features/ai-insight/ai-insight-workspace-api.ts`
- Modify: `web/apps/dashboard/src/features/ai-insight/ai-insight-workspace-api.test.ts`
- Modify: `web/apps/dashboard/src/features/ai-insight/ai-insight-url-state.ts`
- Modify: `web/apps/dashboard/src/features/ai-insight/ai-insight-url-state.test.ts`
- Modify: `web/apps/dashboard/src/features/ai-insight/ai-insight-workspace.tsx`
- Modify: `web/apps/dashboard/src/features/ai-insight/derived-insight-pages.tsx`
- Modify: `web/apps/dashboard/src/features/ai-insight/derived-insight-pages.test.tsx`
- Modify: `web/apps/dashboard/src/styles/ai-insight-workspace.css`

**Interfaces:**
- Consumes: `filter-options` 的 `employees/customers/coverage`。
- Produces: URL 中的 `customerId`、可搜索客户选择器和目录覆盖摘要。

- [ ] 写失败测试：接口严格解析客户与覆盖；选择客户发出 `customerId`；URL 刷新恢复；空态解释目录有实体但无分析。
- [ ] 运行目标 Vitest，确认因合同与控件缺失失败。
- [ ] 实现通用人员搜索控件复用、客户精确筛选和覆盖摘要。
- [ ] 重跑目标 Vitest、AI insight 目录 ESLint 与 typecheck。
- [ ] 提交 `feat(dashboard): 展示 AI 洞察目录覆盖`。

### Task 3: 数据概览切换到同源结构化聚合

**Files:**
- Modify: `internal/modules/reporting/contracts.go`
- Modify: `internal/modules/reporting/sql_repository.go`
- Modify: `internal/modules/reporting/sql_repository_integration_test.go`
- Modify: `web/apps/dashboard/src/features/dashboard-overview/dashboard-overview-api.ts`
- Modify: `web/apps/dashboard/src/features/dashboard-overview/dashboard-overview-api.test.ts`
- Modify: `web/apps/dashboard/src/features/dashboard-overview/dashboard-overview-widgets.tsx`
- Modify: `web/apps/dashboard/src/features/dashboard-overview/dashboard-overview-widgets.test.tsx`
- Modify: `web/apps/dashboard/src/features/dashboard-overview/dashboard-overview-page.test.tsx`

**Interfaces:**
- Produces: `analysisCount/customerNegativeEmotion/averageEmployeeScore/keywordCount/analyzedEmployeeCount/analyzedCustomerCount`。

- [ ] 写失败测试：保留卷聚合值必须与直接查询会话洞察表一致；前端必须展示六个同源指标并链接到三个页面。
- [ ] 运行 reporting integration 与 Dashboard 目标 Vitest，确认旧表/旧字段导致失败。
- [ ] 用参数化 SQL 聚合会话洞察 JSON，套用 tenant/corp/time/employee scope；删除旧结构化缺口告警。
- [ ] 更新前端合同与卡片，重跑目标测试。
- [ ] 提交 `feat(overview): 同步会话 AI 洞察指标`。

### Task 4: DeepSeek 受控实跑与持久化核验

**Files:**
- Runtime only: 工作区外临时 secret 文件，不提交。
- Update: `docs/reviews/2026-08-25-company-profile-ai-insights-implementation-report.zh-CN.md`

**Interfaces:**
- Consumes: `MOCHAT_GO_AI_PROVIDER_BASE_URL`、`MOCHAT_GO_AI_PROVIDER_MODEL`、Docker secret。
- Produces: `mochat_go_ai_insight_runs` 与 `mochat_go_ai_conversation_insights` 的真实 Provider 结果。

- [ ] 记录运行前目录、归档会话、候选、成功结果数量。
- [ ] 创建受限 secret 文件，使用 AI Compose overlay 仅重建 app；确认明文环境 Key 为空、自动任务开关为 0。
- [ ] 通过现有手动分析入口执行 corp 1，会话候选按当前启用规则时间窗处理。
- [ ] 查询 run 和结果追溯字段，抽查来源证据；记录部分失败和外部限制。
- [ ] 删除临时 secret 文件，保留卷不删除。

### Task 5: 完整验收、复审与报告

**Files:**
- Update: `docs/reviews/2026-08-25-company-profile-ai-insights-implementation-report.zh-CN.md`

- [ ] 运行目标测试、Dashboard 全量 test/typecheck/lint/build、全量 Go、隔离 MariaDB integration、RBAC/benchmark/all-pages/provider gates、`git diff --check` 和凭证扫描。
- [ ] 只重建 app，确认 app/MySQL/Redis healthy、healthz/readyz 200、日志无 migration/checksum/panic/fatal。
- [ ] 用真实浏览器验证三页员工/客户全集搜索、查询/重置/刷新/URL、详情证据、错误恢复、桌面/宽屏/窄屏/小高度，以及数据概览六项指标。
- [ ] 进行独立代码复审，修复重要问题并重新验证。
- [ ] 更新中文实施报告，提交文档；保留隔离分支，不 push、不合入 main。

## 计划自审

设计中的目录、客户精确过滤、覆盖、概览六项指标、DeepSeek 密钥边界、红绿测试和最终验收均有对应任务；没有占位步骤或未定义接口。
