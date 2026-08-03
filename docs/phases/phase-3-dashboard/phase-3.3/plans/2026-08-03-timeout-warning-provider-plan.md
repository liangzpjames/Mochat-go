# Phase 3.3 超时预警 Provider 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**目标：** 完成超时预警数据模型、MySQL Provider、评估器、HTTP 接口和三个前端页签，并在保留 Docker 数据卷的前提下部署验收。

**架构：** 领域模型与校验位于 `internal/dashboard`，MySQL 持久化位于 `internal/store`，Handler 负责登录企业与权限解析，前端通过 `BusinessWorkbenchApi` 调用。评估器由会话链路或后续扫描任务主动调用，不在本轮创建定时任务。

**技术栈：** Go 1.26、MariaDB、React、TypeScript、TanStack Query、Vitest、Docker Compose。

## 全局约束

- 所有数据按 `tenant_id` 和 `corp_id` 隔离。
- 超时策略每规则 1–5 条，阈值为 3–180 分钟。
- 不发送外部通知，只写入 `pending` 通知意图。
- 不执行 `docker compose down -v`，保留现有数据卷。
- 用户界面和验收文档使用中文。

---

### 任务 1：数据库迁移与发布基线

**文件：**
- 新建：`deploy/standalone/migrations/0111_timeout_warning_provider.up.sql`
- 新建：`deploy/standalone/migrations/0111_timeout_warning_provider.down.sql`
- 修改：`internal/dashboard/saas_admin_system_health.go`
- 修改：`internal/migration/migration_test.go`
- 测试：`internal/migration/timeout_warning_integration_test.go`

**产出：** 八张超时预警表、租户和筛选索引、记录幂等唯一键及可回滚迁移。

- [ ] 先写迁移目录与 SQL 结构断言测试并运行，确认因 0111 不存在而失败。
- [ ] 创建 up/down SQL，包含规则、策略、静音时段、通知对象、设置、记录、审计、通知意图。
- [ ] 更新最新迁移版本和迁移总数基线。
- [ ] 运行 `go test ./internal/migration ./internal/dashboard -run 'TimeoutWarning|MigrationExpectation' -count=1`，确认通过。
- [ ] 提交 `feat: add timeout warning schema`。

### 任务 2：领域模型、校验与评估匹配

**文件：**
- 新建：`internal/dashboard/timeout_warning.go`
- 新建：`internal/dashboard/timeout_warning_evaluator.go`
- 测试：`internal/dashboard/timeout_warning_test.go`
- 测试：`internal/dashboard/timeout_warning_evaluator_test.go`

**接口：**
- `ValidateTimeoutRule(TimeoutRule) error`
- `MatchTimeoutStrategies(TimeoutEvaluation, []TimeoutRule, TimeoutSettings) []TimeoutRecord`
- `TimeoutWarningProvider`、`TimeoutRuleProviderWriter`、`TimeoutRecordProviderWriter`、`TimeoutEvaluatorProvider`

- [ ] 先写规则数量、阈值、范围、静音时段、结束语、白名单和幂等记录字段测试并确认失败。
- [ ] 实现最小领域类型、校验和纯函数评估逻辑。
- [ ] 运行 `go test ./internal/dashboard -run 'TimeoutRule|TimeoutEvaluation' -count=1`。
- [ ] 提交 `feat: add timeout warning domain model`。

### 任务 3：MySQL Provider

**文件：**
- 新建：`internal/store/timeout_warning.go`
- 测试：`internal/store/timeout_warning_test.go`

**接口产出：**
- 规则分页、事务新增/更新、状态切换和受保护删除。
- 设置读取与覆盖更新。
- 记录分页、批量审计、分派和关闭。
- `EvaluateTimeoutMessage` 幂等写入记录、触发次数和通知意图。

- [ ] 先写 SQL mock/集成风格测试，覆盖筛选、事务、租户条件及审核流水，并确认失败。
- [ ] 实现 Provider，所有更新语句同时限定企业和租户。
- [ ] 运行 `go test ./internal/store -run TimeoutWarning -count=1`。
- [ ] 提交 `feat: add timeout warning mysql provider`。

### 任务 4：HTTP Handler 与服务器接线

**文件：**
- 新建：`internal/dashboard/timeout_warning_handler.go`
- 测试：`internal/dashboard/timeout_warning_handler_test.go`
- 修改：`internal/server/server.go`
- 修改：`cmd/mochat-go/main.go`

**路由：**
- `GET/POST/PUT/DELETE /dashboard/timeout-warning/rules`
- `PUT /dashboard/timeout-warning/rules/status`
- `GET /dashboard/timeout-warning/records`
- `POST /dashboard/timeout-warning/records/audit`
- `PUT /dashboard/timeout-warning/records/assign`
- `GET/PUT /dashboard/timeout-warning/settings`
- `POST /dashboard/timeout-warning/evaluate`

- [ ] 先写租户解析、查询、创建、审核和参数错误 Handler 测试并确认失败。
- [ ] 实现 Handler、权限键和 Provider 能力检查。
- [ ] 注册 server options、路由与 main wiring。
- [ ] 运行 `go test ./internal/dashboard ./internal/server ./cmd/mochat-go -run TimeoutWarning -count=1`。
- [ ] 提交 `feat: expose timeout warning provider api`。

### 任务 5：前端三个页签

**文件：**
- 新建：`web/apps/dashboard/src/features/phase33/timeout-warning-page.tsx`
- 修改：`web/apps/dashboard/src/features/phase33/risk-warning-pages.tsx`
- 修改：`web/apps/dashboard/src/app/page-registry.tsx`
- 测试：`web/apps/dashboard/src/features/phase33/risk-warning-pages.test.tsx`

**产出：** 超时记录、规则配置、高级设置三个页签；中文筛选、表格、详情、审计和规则表单；不再显示 Provider 缺失状态。

- [ ] 先更新页面测试，断言真实 Provider 请求与三个页签，并确认失败。
- [ ] 实现页面和 API mutation，复用 phase3.3 卡片、筛选栏和表格样式。
- [ ] 运行 TypeScript 与定向 Vitest。
- [ ] 提交 `feat: connect timeout warning page`。

### 任务 6：部署与端到端验收

**文件：**
- 新建：`docs/phases/phase-3-dashboard/phase-3.3/acceptance/2026-08-03-timeout-warning-acceptance.zh-CN.md`

- [ ] 运行后端相关包测试、前端类型检查、定向 Vitest 和 `git diff --check`。
- [ ] 用现有 Docker Compose 项目仅重建应用容器，确认 `/readyz` 为 200。
- [ ] 如迁移器受历史校验和阻塞，精确手工应用 0111，不重建数据库、不删除卷，并记录原因。
- [ ] 在真实页面创建、编辑、启停规则，生成临时记录，验证详情、审计、分派、关闭和高级设置。
- [ ] 截图检查中文字段、按钮排版和错误状态，清理临时验收数据。
- [ ] 写入验收证据并提交 `docs: record timeout warning acceptance`。
