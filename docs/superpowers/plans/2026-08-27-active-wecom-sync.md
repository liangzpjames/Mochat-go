# 企业微信主动同步 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为人员信息和会话存档提供真实、可审计、可恢复的主动同步入口，并在唯一企业资料及对应业务页面统一展示状态。

**Architecture:** 人员同步复用现有 Redis 可靠队列；会话存档在 0138 durable run ledger 中创建手动 queued run，由现有 durable worker 领取并执行。前端抽取共享同步状态合同，在唯一企业资料集中展示，在员工与全局会话页提供快捷入口。

**Tech Stack:** Go 1.26、MariaDB 10.6、Redis、React 19、TanStack Query、Vitest、Docker Compose。

## Global Constraints

- 自建应用和第三方代开发都能看到同步区域；第三方模式仍不得显示企微配置表单。
- 未配置时 Dashboard 正常展示空数据，同步按钮禁用且使用业务中文解释。
- 请求体不接受 tenantId、corpId、actorId；敏感值不回显、不入日志、不入前端缓存。
- 会话手动同步必须写入 0138 durable ledger，并可在重启后继续；不得使用仅进程内 goroutine 作为任务权威。
- 本地验收不得调用真实企业微信。

---

### Task 1: 会话存档手动任务合同

**Files:**
- Modify: `internal/modules/providers/archive/sync.go`
- Modify: `internal/modules/providers/archive/durable_bridge.go`
- Modify: `internal/store/archive_sync.go`
- Test: `internal/modules/providers/archive/sync_test.go`
- Test: `internal/modules/providers/archive/durable_bridge_test.go`
- Test: `internal/store/archive_sync_integration_test.go`

**Interfaces:**
- Produces: `SyncService.Enqueue(ctx, source, request) (SyncRun, error)`、`DurableBridgeRunner.Enqueue(ctx, binding, requestID)`、queued run 领取查询与窗口化轮询键。

- [ ] **Step 1: 写 RED**：证明 enqueue 只创建 queued run、不拉取 source；手动 requestId 幂等；pending run 可由 runner 领取；定时空拉取不会永久阻止下一窗口检查。
- [ ] **Step 2: 运行** `go test ./internal/modules/providers/archive ./internal/store -run 'Test.*(Enqueue|Manual|Pending|PollWindow)' -count=1`，预期因接口缺失或行为不符失败。
- [ ] **Step 3: 最小实现**：拆分 enqueue/execute，增加 pending run 读取和 bridge source 重建；所有查询以 tenant/corp/source 联合约束。
- [ ] **Step 4: 运行相同测试并扩到** `go test ./internal/modules/providers/archive/... ./internal/store -count=1`，预期 PASS。
- [ ] **Step 5: 提交** `feat(archive): enqueue durable manual sync runs`。

### Task 2: Dashboard 会话同步 API、权限与状态

**Files:**
- Modify: `internal/companyprofile/model.go`
- Modify: `internal/companyprofile/service.go`
- Modify: `internal/companyprofile/http.go`
- Modify: `internal/dashboard/dashboard_access_guard.go`
- Modify: `internal/server/server.go`
- Modify: `cmd/mochat-go/main.go`
- Test: `internal/companyprofile/service_test.go`
- Test: `internal/server/company_profile_route_test.go`
- Test: `internal/dashboard/dashboard_access_guard_test.go`

**Interfaces:**
- Produces: `POST /dashboard/company/archive-sync`、`GET /dashboard/company/archive-sync-status`；响应只包含业务状态、计数和时间。

- [ ] **Step 1: 写 RED**：覆盖超级管理员/写权限、普通只读、跨租户、配置不足、第三方能力允许、重复 requestId、空状态和技术错误码不出响应。
- [ ] **Step 2: 运行** `go test ./internal/companyprofile ./internal/dashboard ./internal/server -run 'Test.*ArchiveSync' -count=1`，预期 FAIL。
- [ ] **Step 3: 最小实现**：服务端从 principal 解析 scope，连接 Task 1 scheduler/store，新增路由和 fail-closed 权限。
- [ ] **Step 4: 运行目标测试与** `go test ./internal/companyprofile ./internal/dashboard ./internal/server -count=1`，预期 PASS。
- [ ] **Step 5: 提交** `feat(company): expose active archive synchronization`。

### Task 3: 前端共享同步合同与统一入口

**Files:**
- Modify: `web/apps/dashboard/src/features/company-settings/company-profile-api.ts`
- Modify: `web/apps/dashboard/src/features/company-settings/website-page.tsx`
- Modify: `web/apps/dashboard/src/features/company-settings/company-settings-pages.test.tsx`
- Modify: `web/apps/dashboard/src/styles/company-profile.css`

**Interfaces:**
- Consumes: Task 2 两个 archive API 与既有 employee API。
- Produces: 两种模式通用的“数据同步”区域和受控中文状态。

- [ ] **Step 1: 写 RED**：第三方模式也显示两项同步；未配置只禁用不报页面错误；确认前不 mutation；pending 防重复；状态、计数、刷新和错误文案正确；不出现 raw code/source/cursor。
- [ ] **Step 2: 运行** `pnpm --filter @mochat/dashboard test -- company-settings-pages.test.tsx`，预期 FAIL。
- [ ] **Step 3: 最小实现**：把员工同步区移出自建配置条件，增加会话同步 query/mutation 和紧凑双卡布局。
- [ ] **Step 4: 运行目标测试、typecheck 和 lint，预期 PASS。**
- [ ] **Step 5: 提交** `feat(dashboard): add unified data sync controls`。

### Task 4: 员工与会话业务页快捷入口

**Files:**
- Modify: `web/apps/dashboard/src/features/employee/employee-api.ts`
- Modify: `web/apps/dashboard/src/features/employee/employee-page.tsx`
- Modify: `web/apps/dashboard/src/features/employee/employee-page.test.tsx`
- Modify: `web/apps/dashboard/src/features/conversation-global/conversation-global-api.ts`
- Modify: `web/apps/dashboard/src/features/conversation-global/conversation-global-page.tsx`
- Modify: `web/apps/dashboard/src/features/conversation-global/conversation-global-page.test.tsx`

**Interfaces:**
- Consumes: Task 2 API；共享 Query key 与状态映射。

- [ ] **Step 1: 写 RED**：员工和全局会话页出现“立即同步”按钮；确认前无请求；同步后刷新各自列表/概览；无权限或不可用时禁用并解释。
- [ ] **Step 2: 分别运行两个目标测试，预期 FAIL。**
- [ ] **Step 3: 最小实现**：业务页只提供快捷入口，不复制配置表单；固定类型子页面不重复显示会话同步按钮。
- [ ] **Step 4: 运行目标测试及 Dashboard 全量测试，预期 PASS。**
- [ ] **Step 5: 提交** `feat(dashboard): add sync shortcuts to data pages`。

### Task 5: Docker、浏览器和交付报告

**Files:**
- Modify: `scripts/run_wecom_archive_saas_activation_acceptance.ps1`
- Modify: `docs/superpowers/reports/2026-08-27-wecom-archive-saas-activation-final-report.zh-CN.md`
- Test: `scripts/check_wecom_archive_saas_activation.test.mjs`

**Interfaces:**
- Produces: 可幂等触发的本地人员/会话主动同步夹具与最终中文证据。

- [ ] **Step 1: 写 RED**：专项合同要求主动 sync 路由、真实 durable run、状态回读及重启恢复证据。
- [ ] **Step 2: 运行专项合同，预期 FAIL。**
- [ ] **Step 3: 更新夹具/验收脚本；只重建专项 app，不删除卷。**
- [ ] **Step 4: 运行 Go 全仓、四前端 lint/typecheck/test/build、RBAC、Provider completion、页面证据、53 页 benchmark、迁移合同、专项合同和 `git diff --check`。外部 DSN 缺失必须记录 SKIP。**
- [ ] **Step 5: 浏览器实际点击唯一企业资料、员工列表、全局会话页的同步与刷新；检查桌面、390×844、网络、控制台、空态、失败态和刷新恢复。**
- [ ] **Step 6: 更新中文报告并提交** `docs: report active synchronization acceptance`。
