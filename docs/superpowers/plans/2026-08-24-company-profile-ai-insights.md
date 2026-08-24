# 企业资料权限与三项 AI 洞察优化 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复唯一企业资料的可授予权限链，清零 Dashboard ESLint，并把情绪识别、员工评分、沟通关键词改造成读取真实会话分析结果与证据的可验收工作台。

**Architecture:** 企业资料继续使用页面级 RBAC，只把该页面从超管专属改为可授予，API 仍由 catalog resource 和租户 principal 失败关闭。三个 AI 页面作为现有 `session` 会话分析持久化结果的只读投影，共用 repository、状态、详情证据与员工范围，不新增 Provider 调用或结果表。

**Tech Stack:** Go 1.24、MariaDB/MySQL migrations、React 19、TypeScript、TanStack Query、React Router、Vitest、Testing Library、ESLint、pnpm 11、Docker Compose。

## Global Constraints

- 不修改或提交 `docs/PROJECT_PROGRESS.zh-CN.md`，不清理根工作树受保护内容。
- 不 force push、不改写历史、不再次推送新开发成果。
- 不删除 Docker 数据卷，不改写已应用迁移字节。
- Phase 7 继续暂停；自动日分析和启动即分析保持关闭。
- 不在前端或测试外生产代码中放静态业务数据、随机分数、固定趋势、伪成功、Provider/企微凭证。
- 所有行为修改先运行能因缺失行为而失败的测试，再写最小实现。
- 每个任务完成目标测试后形成独立提交；lint 按目录/错误类型拆分提交。

---

### Task 1: 固化中文设计与计划

**Files:**
- Create: `docs/superpowers/specs/2026-08-24-company-profile-ai-insights-design.md`
- Create: `docs/superpowers/plans/2026-08-24-company-profile-ai-insights.md`

**Interfaces:**
- Consumes: 圆弧三页浏览器调研、仓库菜单/RBAC/API/存储追踪结果。
- Produces: 后续任务唯一范围、接口、迁移与验收依据。

- [ ] **Step 1: 扫描文档占位符和矛盾**

Run: 人工逐节核对标题、字段、命令、预期结果与失败关闭边界。

Expected: 无输出，退出码 1。

- [ ] **Step 2: 核对强制章节**

Run: `rg -n "现状与范围矩阵|系统化根因|圆弧 AI 三页调研|数据来源矩阵|API、存储与迁移|权限、租户与安全|诚实状态|响应式与宽屏|验收标准" docs/superpowers/specs/2026-08-24-company-profile-ai-insights-design.md`

Expected: 每个章节均命中。

- [ ] **Step 3: 提交设计与计划**

```powershell
git add docs/superpowers/specs/2026-08-24-company-profile-ai-insights-design.md docs/superpowers/plans/2026-08-24-company-profile-ai-insights.md
git commit -m "docs: 设计企业资料权限与三项 AI 洞察"
```

### Task 2: 用失败测试复现企业资料三层权限根因

**Files:**
- Modify: `web/apps/dashboard/src/app/access-loader.test.ts`
- Modify: `web/apps/dashboard/src/layout/yuanhu-navigation.test.ts`
- Modify: `web/apps/dashboard/src/features/company-settings/company-settings-pages.test.tsx`
- Modify: `internal/dashboard/dashboard_access_test.go`
- Modify: `internal/dashboard/dashboard_access_guard_test.go`
- Modify: `scripts/check_dashboard_page_rbac_catalog.test.mjs`

**Interfaces:**
- Consumes: `dashboard.company_setting.website` 页面码与 `/company-setting/website` 路由。
- Produces: 已授权普通用户菜单/直接 URL/API 成功、未授权用户失败关闭的回归合同。

- [ ] **Step 1: 写前端失败测试**

新增断言：普通用户 profile 的 effective permission 包含 `dashboard.company_setting.website` 时，loader 的 `allowedRoutes` 包含该路由、导航包含“唯一企业资料”、页面会调用 `getProfile`；未包含时 loader 抛 403 且页面不发请求。

- [ ] **Step 2: 验证前端 RED**

Run: `corepack pnpm --filter @mochat/dashboard test -- src/app/access-loader.test.ts src/layout/yuanhu-navigation.test.ts src/features/company-settings/company-settings-pages.test.tsx`

Expected: 普通已授权页面仍显示“只有企业超级管理员”或未调用 `getProfile`，测试失败。

- [ ] **Step 3: 写后端与 catalog 失败测试**

新增断言：catalog 中 website 为 grantable；普通用户持有该权限 grant 时 `Resolve` 返回该路由；guard 对 website resource 允许该用户，对无 grant 用户返回 `DASHBOARD_PERMISSION_DENIED`；deny-only 合同不含 website API。

- [ ] **Step 4: 验证后端 RED**

Run: `go test ./internal/dashboard -run "TestDashboardAccess.*Company|TestDashboardAccessGuard.*Company" -count=1; node --test scripts/check_dashboard_page_rbac_catalog.test.mjs`

Expected: 当前 superadmin-only 与 deny-only 策略导致至少一项失败。

### Task 3: 修复企业资料可授予权限链

**Files:**
- Create: `deploy/standalone/migrations/0162_company_profile_grantable.up.sql`
- Create: `deploy/standalone/migrations/0162_company_profile_grantable.down.sql`
- Modify: `internal/dashboard/dashboard_page_catalog.json`
- Modify: `internal/dashboard/dashboard_route_policy.go`
- Modify: `web/apps/dashboard/src/features/company-settings/website-page.tsx`
- Modify: `web/apps/dashboard/src/features/provider-status/provider-status-page.tsx`
- Modify: Phase 4 expected-count tests/scripts that explicitly encode 48 grantable / 5 superadmin-only.

**Interfaces:**
- Consumes: Task 2 的失败合同。
- Produces: 可授予 website permission；catalog resource 权威 API 授权；普通用户脱敏 Provider 状态。

- [ ] **Step 1: 新增纠正迁移**

`up` 执行参数固定的 catalog 更新：`restriction='grantable', superadmin_only=0`，并幂等登记 website 的 `GET /dashboard/providers/status` 只读资源；`down` 恢复 `restriction='superadmin_only', superadmin_only=1`，并删除该迁移登记的资源键。不得编辑 0131。

- [ ] **Step 2: 对齐 canonical catalog 与 deny-only**

把 website `superadminOnly` 设为 false，从 `DenyOnlyDashboardRouteContracts()` 移除 website 映射的 company/provider-status API；其他企业设置与其他 deny-only 保持不变。

- [ ] **Step 3: 修复页面准入与脱敏**

页面准入改为“显式测试授权或 `allowedRoutes` 包含 website”；传给 Provider 状态的超管标志仍取真实 `profile.isSuperAdmin`，不能把普通页面授权当成超级管理员。

- [ ] **Step 4: 验证 GREEN**

Run: `corepack pnpm --filter @mochat/dashboard test -- src/app/access-loader.test.ts src/layout/yuanhu-navigation.test.ts src/features/company-settings/company-settings-pages.test.tsx src/features/provider-status/provider-status-page.test.tsx; go test ./internal/dashboard ./internal/store ./internal/migration -count=1; node --test scripts/check_dashboard_page_rbac_catalog.test.mjs`

Expected: 全部通过。

- [ ] **Step 5: 提交企业资料修复**

```powershell
git add deploy/standalone/migrations/0162_company_profile_grantable.* internal/dashboard web/apps/dashboard/src/features/company-settings web/apps/dashboard/src/features/provider-status scripts
git commit -m "fix(rbac): 恢复唯一企业资料可授予权限"
```

### Task 4: 用失败测试定义三个 AI 专用只读投影

**Files:**
- Modify: `internal/modules/ai-insight/workspace_handler_test.go`
- Modify: `internal/modules/ai-insight/repository_test.go`
- Modify: `internal/modules/ai-insight/repository_integration_test.go`
- Modify: `internal/modules/ai-insight/transport/http/routes_test.go`
- Modify: `web/apps/dashboard/src/features/ai-insight/ai-insight-workspace-api.test.ts`
- Create: `web/apps/dashboard/src/features/ai-insight/derived-insight-pages.test.tsx`
- Modify: `web/apps/dashboard/src/features/ai-insight/ai-insight-url-state.test.ts`

**Interfaces:**
- Consumes: 已有 `ConversationInsight`、`SessionAnalysisResult`、`InsightDrawer`、员工 scope。
- Produces: 三页 records/detail/status/filter-options/export 与 URL/页面行为合同。

- [ ] **Step 1: 写后端 RED**

测试三个 page slug 都映射 `AnalysisTypeSession`；情绪、分数区间、关键词参数进入参数化 repository filter；详情按同一 tenant/corp/employee scope 回读消息；无权限或跨租户返回 403/404；页面读取不调用 `AIProvider.Chat`。

- [ ] **Step 2: 运行后端 RED**

Run: `go test ./internal/modules/ai-insight/... -run "Projection|Emotion|EmployeeScore|CommunicationKeyword" -count=1`

Expected: 路由或字段尚不存在，测试失败。

- [ ] **Step 3: 写前端 RED**

测试三页各自筛选字段、查询/重置/刷新、固定 20 条分页、当前页真实摘要、失败/空态、Provider 状态、整行详情、Escape/遮罩/关闭、原会话跳转和 URL 恢复；API parser 拒绝无来源指纹/无会话 URL/未知枚举。

- [ ] **Step 4: 运行前端 RED**

Run: `corepack pnpm --filter @mochat/dashboard test -- src/features/ai-insight/derived-insight-pages.test.tsx src/features/ai-insight/ai-insight-workspace-api.test.ts src/features/ai-insight/ai-insight-url-state.test.ts`

Expected: 专用页面或 API 方法未定义，测试失败。

### Task 5: 实现后端投影、过滤、RBAC 与导出

**Files:**
- Create: `deploy/standalone/migrations/0163_ai_insight_projection_resources.up.sql`
- Create: `deploy/standalone/migrations/0163_ai_insight_projection_resources.down.sql`
- Modify: `internal/modules/ai-insight/contracts.go`
- Modify: `internal/modules/ai-insight/repository.go`
- Modify: `internal/modules/ai-insight/workspace_handler.go`
- Modify: `internal/modules/ai-insight/transport/http/routes.go`
- Modify: `internal/dashboard/dashboard_page_catalog.json`
- Modify: `scripts/check_dashboard_page_rbac_catalog.mjs`

**Interfaces:**
- Consumes: `InsightFilter`、`Repository.InsightPage`、`Repository.InsightDetail`。
- Produces: `GET /dashboard/ai-insight/{view}/{records|detail|status|filter-options|export}`。

- [ ] **Step 1: 扩展 filter 合同**

新增 `View string`、`Emotion string`、`MinScore *int`、`MaxScore *int`；校验 emotion 仅 `positive|neutral|negative|mixed|unknown`，分数 0–100 且 min<=max。

- [ ] **Step 2: 实现参数化 JSON 过滤**

`insightWhere` 仅在相应 view 下追加 MariaDB 兼容 `JSON_EXTRACT/JSON_UNQUOTE` 条件；关键词 LIKE 使用 `escapedLike`；员工 scope 条件保持最后合并且不可绕过。

- [ ] **Step 3: 扩展 workspace handler 与 CSV**

三个 view 使用 session records/status/detail/filter-options；导出根据 view 写真实列，复用 `workspaceCSV` 防公式注入；页面读取路径不调用模型。

- [ ] **Step 4: 注册路由和资源迁移**

为每页添加 5 个 GET route/resource，`scope_required=1`；up 用 `NOT EXISTS` 幂等插入，down 只删除精确方法+路径。

- [ ] **Step 5: 验证后端 GREEN**

Run: `go test ./internal/modules/ai-insight/... ./internal/dashboard ./internal/migration -count=1`

Expected: 通过。

- [ ] **Step 6: 提交后端投影**

```powershell
git add deploy/standalone/migrations/0163_ai_insight_projection_resources.* internal/modules/ai-insight internal/dashboard/dashboard_page_catalog.json scripts/check_dashboard_page_rbac_catalog.mjs
git commit -m "feat(ai): 提供三项洞察会话结果投影"
```

### Task 6: 实现三个 AI 洞察工作台

**Files:**
- Modify: `web/apps/dashboard/src/features/ai-insight/ai-insight-workspace-api.ts`
- Modify: `web/apps/dashboard/src/features/ai-insight/ai-insight-url-state.ts`
- Create: `web/apps/dashboard/src/features/ai-insight/derived-insight-pages.tsx`
- Modify: `web/apps/dashboard/src/features/ai-insight/ai-insight-workspace.tsx`
- Modify: `web/apps/dashboard/src/benchmark/page-registry.tsx`
- Modify: `web/apps/dashboard/src/styles/index.css`
- Replace/remove runtime use of: `web/apps/dashboard/src/features/ai-insight/ai-insight-pages.tsx` for the three target routes.

**Interfaces:**
- Consumes: Task 5 API；现有 `AiInsightHeader`、`AiInsightQueryBar`、`EmployeeSearchField`、`AiInsightStatusStrip`、`InsightPagination`、`InsightDrawer`。
- Produces: `EmotionInsightPage`、`EmployeeScoreInsightPage`、`CommunicationKeywordInsightPage`。

- [ ] **Step 1: 实现严格 API parser 和 URL serializer**

使用 `unknown`→record guard，不引入 `any`；保留 0 分；缺来源指纹、未知情绪枚举、无会话 URL 时抛明确合同错误。

- [ ] **Step 2: 实现统一工作台壳**

共享加载、错误、分页、详情、状态和当前页摘要逻辑；三个页面只提供字段配置与行投影，避免复制三套请求状态机。

- [ ] **Step 3: 实现专用字段与真实摘要**

情绪页显示客户情绪与原因；员工评分显示真实 score/维度/优缺点；关键词页显示真实关键词标签。当前页统计必须显式标注当前页。

- [ ] **Step 4: 接入 registry 与响应式 CSS**

三条路由替换为新页面；复用现有宽屏容器和抽屉，补齐 2560×1440、常规桌面、窄屏、小高度规则。

- [ ] **Step 5: 验证前端 GREEN**

Run: `corepack pnpm --filter @mochat/dashboard test -- src/features/ai-insight/derived-insight-pages.test.tsx src/features/ai-insight/ai-insight-workspace-api.test.ts src/features/ai-insight/ai-insight-url-state.test.ts src/features/ai-insight/ai-insight-workspace.test.tsx`

Expected: 通过且无未解释 stderr。

- [ ] **Step 6: 提交前端工作台**

```powershell
git add web/apps/dashboard/src/features/ai-insight web/apps/dashboard/src/benchmark/page-registry.tsx web/apps/dashboard/src/styles/index.css
git commit -m "feat(ai): 优化情绪评分与关键词工作台"
```

### Task 7: 分批清零 Dashboard 86 个 ESLint 错误

**Files:**
- Modify: lint 输出命中的 `conversation-global/**`、`conversation-operations/**`、`phase33/**`、`risk-warning/**`、`sensitive-word/**`、`styles/conversation-operations-layout.test.ts`。

**Interfaces:**
- Consumes: 基线 lint 精确清单。
- Produces: 不改变业务行为的严格类型与测试修复。

- [ ] **Step 1: 修复 conversation-global 批次**

移除被 `string` 覆盖的字面量联合或改为有限枚举+未知解析；删除未使用项；把提取的方法改为箭头包装；把测试 `any` 攓为 `unknown` guard/精确类型。

Run: `corepack pnpm --filter @mochat/dashboard exec eslint src/features/conversation-global`

Expected: 0 errors。

Commit: `git commit -m "chore(lint): 清理会话全局模块严格类型"`

- [ ] **Step 2: 修复 conversation-operations 批次**

为 object stringification 增加 string/number/boolean guard；修复 unbound method、无效断言和未使用类型。

Run: `corepack pnpm --filter @mochat/dashboard exec eslint src/features/conversation-operations src/styles/conversation-operations-layout.test.ts`

Expected: 0 errors。

Commit: `git commit -m "chore(lint): 清理会话运营模块严格类型"`

- [ ] **Step 3: 修复 phase33 与 risk-warning 批次**

使用显式未知值解析替代冗余联合；事件处理用 void 包装 Promise；测试 mock 使用接口签名；删除未使用类型。

Run: `corepack pnpm --filter @mochat/dashboard exec eslint src/features/phase33 src/features/risk-warning`

Expected: 0 errors。

Commit: `git commit -m "chore(lint): 清理风险预警模块严格类型"`

- [ ] **Step 4: 修复 sensitive-word 与剩余批次**

把 API mock 参数声明为精确 filter 类型，箭头包装 query 方法，消除不安全 member access。

Run: `corepack pnpm --filter @mochat/dashboard exec eslint src/features/sensitive-word`

Expected: 0 errors。

Commit: `git commit -m "chore(lint): 清理敏感词模块严格类型"`

- [ ] **Step 5: 运行全量 lint**

Run: `corepack pnpm --filter @mochat/dashboard lint`

Expected: 0 errors / 0 warnings，退出码 0。

### Task 8: 迁移与真实 MariaDB 集成验证

**Files:**
- Modify/Create only if a failing migration contract exposes a real defect: `internal/migration/*_test.go`。

**Interfaces:**
- Consumes: 0162、0163。
- Produces: fresh apply、幂等、checksum、升级可用证据。

- [ ] **Step 1: 运行迁移合同**

Run: `go test ./internal/migration -count=1`

Expected: 通过，0162/0163 被发现且 checksum 稳定。

- [ ] **Step 2: 使用隔离 schema 运行 MariaDB integration**

创建仅以 `mochat_company_ai_insight_` 开头的临时 schema，设置仓库既有 integration DSN 环境变量，运行企业资料 permission 与 AI JSON filter/迁移集成测试；测试结束只删除已验证前缀的隔离 schema，不接触用户业务库。

Expected: apply、重复执行、查询和 down/up 合同通过。

- [ ] **Step 3: 验证保留卷升级**

在现有 Compose 命名卷上构建 app，确认 migration ledger 新增 0162/0163 且旧 checksum 无变化。

### Task 9: 全量工程门禁

**Files:**
- Modify only when a gate reports a task-scoped defect。

**Interfaces:**
- Consumes: 所有实现提交。
- Produces: 自动化可验收证据。

- [ ] **Step 1: Dashboard 全量**

```powershell
corepack pnpm --filter @mochat/dashboard test
corepack pnpm --filter @mochat/dashboard typecheck
corepack pnpm --filter @mochat/dashboard lint
corepack pnpm --filter @mochat/dashboard build
```

Expected: 141 个测试文件/846+ 测试全部通过；typecheck/lint/build 退出码 0；无新增 warning。

- [ ] **Step 2: Go 全量**

Run: `go test ./... -count=1`

Expected: 退出码 0；命令只在干净隔离 worktree 执行。

- [ ] **Step 3: 专项完成门禁**

```powershell
corepack pnpm check:phase4-dashboard-page-rbac
corepack pnpm check:yuanhu-benchmark
corepack pnpm check:dashboard-all-pages-evidence
corepack pnpm check:provider-completion
```

Expected: Phase 4 数量与新策略一致，53 页 benchmark、all-pages evidence、provider completion 全部通过。

- [ ] **Step 4: 静态安全检查**

Run: `git diff --check; rg -n --hidden -g '!node_modules/**' -g '!.git/**' "(BEGIN (RSA |EC |OPENSSH )?PRIVATE KEY|sk-[A-Za-z0-9_-]{16,}|AKID[A-Za-z0-9]{12,}|(?i)(secret|token|password)\s*[:=]\s*['\"][^'\"]{8,})"`

Expected: `git diff --check` 无输出；凭证扫描只有既有测试占位符或文档字段名，经逐项确认无真实凭证。

### Task 10: Docker 与真实浏览器验收

**Files:**
- Create: `docs/reviews/2026-08-24-company-profile-ai-insights-implementation-report.zh-CN.md`

**Interfaces:**
- Consumes: production build、保留卷 Compose、可用测试账号与真实浏览器。
- Produces: 页面、容器、控制台、响应式验收证据与中文报告。

- [ ] **Step 1: 仅重建 app**

Run: `docker compose -p mochat-go-desktop --env-file deploy/standalone/.env.local -f deploy/standalone/docker-compose.yml up -d --build --no-deps app`

Expected: 不删除/重建 MySQL、Redis 卷。

- [ ] **Step 2: 健康与日志**

Run: `docker compose -p mochat-go-desktop -f deploy/standalone/docker-compose.yml ps; curl.exe -fsS http://127.0.0.1:18080/healthz; curl.exe -fsS http://127.0.0.1:18080/readyz; docker compose -p mochat-go-desktop -f deploy/standalone/docker-compose.yml logs --since 10m app mysql redis`

Expected: app/MySQL/Redis healthy，两个端点 200，日志无 migration/checksum/panic/fatal。

- [ ] **Step 3: 企业资料浏览器矩阵**

使用超管、已授予 website 的普通用户、未授予普通用户验证菜单、刷新、直接 URL、菜单高亮、API 403、pending bootstrap；不在浏览器中输入或传输真实 Provider/企微凭证。

- [ ] **Step 4: 三页浏览器矩阵**

逐页验证查询、重置、刷新、分页、URL 恢复、详情、证据、原会话跳转、错误恢复、遮罩/Escape/关闭；视口覆盖常规桌面、2560×1440、390px 窄屏和小高度；控制台无新增 error/warning，无横向溢出。

- [ ] **Step 5: 编写实施与自测报告**

报告必须列出：远端发布 SHA 证据、根因与修复、三个页面与数据来源、接口/迁移、提交、每条测试命令/结果、Docker/浏览器证据、外部限制、Phase 7 暂停、分支 HEAD 与未推送状态。

### Task 11: 独立代码复审与最终新鲜验证

**Files:**
- Modify only for复审发现的 Critical/Important 问题。

**Interfaces:**
- Consumes: `origin/main..HEAD` 全部 diff、设计与计划。
- Produces: 独立复审结论和最终交付状态。

- [ ] **Step 1: 请求独立代码复审**

向 reviewer 提供基点 `756ac3765bdb358ee125689c8c78cef03423c13f`、当前 HEAD、设计文档和验收要求；Critical/Important 必须修复并重跑受影响测试。

- [ ] **Step 2: 新鲜重跑最终证明命令**

至少重跑 Dashboard 全量 lint/test/typecheck/build、Go 全量、四个专项门禁、`git diff --check`、凭证扫描、容器健康与关键浏览器路径。

- [ ] **Step 3: 核对 Git 状态**

Run: `git status --short --branch; git log --oneline origin/main..HEAD; git rev-parse HEAD; git ls-remote origin refs/heads/main`

Expected: 隔离分支仅包含本任务原子提交；根主线远端仍是已发布 SHA；不 push 新分支、不合入 main。

## 计划自审

- 设计中的企业资料权限、三个投影、真实数据、自动分析关闭、响应式、迁移、Docker、浏览器和完整门禁均有对应任务。
- 所有新增类型名、路由名、迁移名和页面组件名在首次出现处已定义，后续引用一致。
- 计划没有占位实现或“类似上一任务”的模糊步骤。
