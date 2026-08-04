# 朋友圈 Provider Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现可持久化、企业隔离、可审计且发布适配器可插拔的朋友圈 Provider，并接入现有 Phase 3.4 页面。

**Architecture:** 新增 0114 数据迁移、Dashboard Handler/Store 合同、MySQL repository、Server 路由和前端 adapter。发布能力经独立接口隔离，未配置时返回 503，草稿入库不冒充外部发布。

**Tech Stack:** Go 1.26、MariaDB 10.6、React 19、TanStack Query、Vitest。

## Global Constraints

- TDD：每个 Handler、Store 和前端行为先失败再实现。
- 自动化环境禁止真实朋友圈发布。
- 所有数据按登录上下文 `corp_id` 隔离。
- Docker 部署保留 `mochat-go-desktop` 数据卷。

---

### Task 1: 数据库与领域 Provider

**Files:**
- Create: `deploy/standalone/migrations/0114_friends_circle_provider.up.sql`
- Create: `deploy/standalone/migrations/0114_friends_circle_provider.down.sql`
- Create: `internal/dashboard/friends_circle.go`
- Create: `internal/dashboard/friends_circle_test.go`
- Modify: `internal/store/mysql.go`

- [ ] 先写 Handler 失败测试：任务/素材分页、草稿创建、跨企业隐藏、未配置发布返回 503。
- [ ] 添加领域类型、Store/Publisher 接口和最小 Handler，使测试转绿。
- [ ] 添加迁移和 MySQL repository，运行相关 Go 测试。

### Task 2: Server 与配置接线

**Files:**
- Modify: `internal/server/server.go`
- Modify: `internal/server/server_test.go`
- Modify: `internal/config/config.go`
- Modify: `cmd/mochat-go/main.go`

- [ ] 先写路由失败测试，再添加五个 Server option 与路由。
- [ ] 添加默认随 migrated routes 启用的配置开关和主程序接线。
- [ ] 运行 server/config/cmd 测试与 Go 全量测试。

### Task 3: 前端真实 Provider 页面

**Files:**
- Modify: `web/apps/dashboard/src/features/phase34/content-reach-pages.tsx`
- Modify: `web/apps/dashboard/src/features/phase34/content-reach-pages.test.tsx`
- Modify: `web/apps/dashboard/src/benchmark/manifest.json`

- [ ] 将朋友圈阻断测试改为真实任务/素材查询、筛选、重试、草稿创建测试，并先观察失败。
- [ ] 接入 taskIndex/materialIndex/taskStore/materialStore；发布和导出按能力保持禁用。
- [ ] 运行定向测试、前端全量门禁和 benchmark 检查。

### Task 4: 数据库、Docker 与浏览器验收

**Files:**
- Create: `docs/phases/phase-3-dashboard/phase-3.4/reports/friends-circle-provider-report.md`
- Update: `docs/phases/phase-3-dashboard/phase-3.4/screenshots/friends-circle.png`

- [ ] 在 MariaDB 应用 0114 并验证表、企业隔离和草稿持久化。
- [ ] 仅重建 `mochat-go-desktop app`，验证健康和四个数据卷。
- [ ] 浏览器验证任务/素材切换、创建草稿、刷新持久化和阻断发布。
- [ ] 提交 `feat: add friends circle provider`。
