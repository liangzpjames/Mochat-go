# 数据概览真实数据补齐 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在不改现有导航和顶部 banner 的前提下，补齐数据概览真实数据并落地圆弧式内容区。

**Architecture:** 后端 reporting overview 继续作为单一数据入口，新增结构化的 AI 指标、质检、员工排行和会话轨迹字段；前端只消费这些字段并按 `null/limitations` 展示缺口状态。页面壳层不动，只替换 banner 以下的 `BusinessDashboard` 内容和专属样式。

**Tech Stack:** Go reporting module、MariaDB、React 19、TanStack Query、Vitest、Vite、Docker Compose。

## Global Constraints

- 保留现有菜单导航和顶部 banner，不修改 `dashboard-layout.tsx`、`yuanhu-navigation.ts` 及概览顶部 header。
- 所有数字来自当前企业和查询区间的真实数据库/Provider；不能把缺失值渲染成 0。
- AI 自然语言摘要不解析猜测情绪或风险数字；无结构化字段时显示“—”并告警。
- 不执行 `docker compose down --volumes`，保留现有 MySQL、Redis、app 和 audit-anchor 数据卷。

---

### Task 1: 固化数据契约与失败测试

**Files:**
- Modify: `internal/modules/reporting/contracts.go`
- Modify: `web/apps/dashboard/src/features/dashboard-overview/dashboard-overview-api.ts`
- Modify: `web/apps/dashboard/src/features/dashboard-overview/dashboard-overview-api.test.ts`
- Modify: `web/apps/dashboard/src/features/dashboard-overview/dashboard-overview-widgets.test.tsx`

- [ ] **Step 1: 写后端契约测试**：构造 overview JSON，断言 `aiMetrics`、`quality`、`employeeRanking`、`trajectory` 可被序列化为公开字段。
- [ ] **Step 2: 运行后端测试确认失败**：运行 `go test ./internal/modules/reporting/...`，预期因字段和结构不存在失败。
- [ ] **Step 3: 写前端解析失败测试**：给 `createDashboardOverviewApi` 返回新增字段，断言解析结果保留 `null` 和缺口字段；给缺口模块组件断言显示“—”和数据来源。
- [ ] **Step 4: 运行前端定向测试确认失败**：运行 `npm --prefix web/apps/dashboard test -- --run src/features/dashboard-overview/dashboard-overview-api.test.ts src/features/dashboard-overview/dashboard-overview-widgets.test.tsx`，预期因类型和组件未实现失败。

### Task 2: 接入后端真实质检、AI 数量、员工排行和轨迹

**Files:**
- Modify: `internal/modules/reporting/contracts.go`
- Modify: `internal/modules/reporting/sql_repository.go`
- Modify: `internal/modules/reporting/sql_repository_integration_test.go`
- Modify: `internal/modules/reporting/transport/http/handler_test.go`

- [ ] **Step 1: 增加结构化 contract**：定义 `AIMetrics`、`QualityStats`、`EmployeeRankingItem`、`ConversationTrajectoryItem`，全部可选或可为空。
- [ ] **Step 2: 在 overview 查询中加入真实统计**：从现有 AI、风险、敏感词、超时、客户关系和 archive 表读取，表不存在或不可用时返回 limitation 而不是返回假数据。
- [ ] **Step 3: 加入员工排行**：按员工聚合归档消息的 distinct 会话数和消息数，尊重企业与员工权限范围。
- [ ] **Step 4: 加入最近轨迹**：按最近消息时间聚合返回有限条会话摘要，字段缺失时不补造客户名称。
- [ ] **Step 5: 运行 integration/unit tests**：运行 `go test ./internal/modules/reporting/...`，预期 PASS。

### Task 3: 重构概览内容区，保持壳层不变

**Files:**
- Modify: `web/apps/dashboard/src/features/dashboard-overview/dashboard-overview-page.tsx`
- Modify: `web/apps/dashboard/src/features/dashboard-overview/dashboard-overview-widgets.tsx`
- Modify: `web/apps/dashboard/src/styles/index.css`
- Modify: `web/apps/dashboard/src/features/dashboard-overview/dashboard-overview-page.test.tsx`
- Modify: `web/apps/dashboard/src/features/dashboard-overview/dashboard-overview-widgets.test.tsx`
- Modify: `web/apps/dashboard/src/styles/dashboard-overview-layout.test.ts`

- [ ] **Step 1: 写页面结构失败断言**：断言现有 banner 仍存在，页面内容依次包含经营概览、AI 洞察、会话数据、质检、排行、轨迹；断言 AI 长文本不出现。
- [ ] **Step 2: 运行前端测试确认失败**。
- [ ] **Step 3: 替换 `BusinessDashboard` 内容**：经营概览四卡、AI 五数字卡、会话左右分栏、质检左右分栏、排行/轨迹紧凑模块；所有缺口使用同一告警组件。
- [ ] **Step 4: 保持 `DashboardOverviewPage` 顶部 header 不变**：只调整内容区调用和数据传递，不修改导航或 banner 文件。
- [ ] **Step 5: 写圆弧式布局样式**：固定左侧控件比例，右侧趋势图 `minmax(0, 1fr)` 填满；日期标签不裁切；按钮和分页 hover 保持高对比度。
- [ ] **Step 6: 运行前端测试确认 PASS**。

### Task 4: 构建、启动和页面自检

**Files:**
- No unrelated files.

- [ ] **Step 1: 运行前端完整测试和构建**：`npm --prefix web/apps/dashboard test -- --run`、`npm --prefix web/apps/dashboard run build`。
- [ ] **Step 2: 运行 Go reporting/server tests**：`go test ./internal/modules/reporting/... ./internal/server/...`。
- [ ] **Step 3: 只重建 app 服务**：`docker compose -f deploy/standalone/docker-compose.yml -p mochat-go-desktop up -d --build --no-deps app`；不停止或删除 MySQL、Redis，不删除卷。
- [ ] **Step 4: 检查服务**：确认 app、mysql、redis healthy，`/readyz` 返回 200。
- [ ] **Step 5: 用当前登录态检查页面**：确认导航和顶部 banner 未变，确认真实数字、趋势日期、缺口告警、会话左右填充和无水平溢出。
- [ ] **Step 6: 运行 `git diff --check` 并审阅变更范围**：确认没有修改用户未授权的壳层文件或脏文件。
