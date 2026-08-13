# 员工账号生命周期与会话存档模拟器 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 交付可审计的员工账号开通/绑定/停启用/重置密码流程，以及与真实企微会话存档隔离的模拟数据工具。

**Architecture:** 账号生命周期扩展现有 Dashboard Access 管理域和 Dashboard identity 表，所有写入在 MySQL 事务内完成。模拟器作为独立 CLI，复用生产存档解析写入并使用批次注册表追踪模拟数据，不触碰真实配置和游标。

**Tech Stack:** Go、MariaDB 10.6、React 19、TanStack Query、Vitest、Docker standalone。

## Global Constraints

- 直接在已获用户授权的 `main` 工作，保留所有既有未提交文件。
- 不创建额外 Docker 服务；部署时只重建现有 app，保留 MySQL、Redis 和四个卷。
- 密码只展示一次，不写日志、审计、fixture 或验收证据。
- 模拟消息必须可识别、幂等、可单独清理，且不得修改真实存档游标和企业存档配置。
- 所有代码行为先写失败测试再实现。

---

### Task 1: 数据库与领域合同

**Files:**
- Create: `deploy/standalone/migrations/0133_employee_accounts_archive_simulation.up.sql`
- Create: `deploy/standalone/migrations/0133_employee_accounts_archive_simulation.down.sql`
- Modify: `internal/dashboard/dashboard_access_admin.go`
- Test: `internal/dashboard/dashboard_access_admin_test.go`

**Interfaces:**
- Produces: `DashboardEmployeeAccountPage`、`ProvisionEmployeeAccount`、`UpdateEmployeeAccountStatus`、`ResetEmployeeAccountPassword`。

- [ ] 写失败测试，覆盖输入校验、superadmin actor、跨租户和临时密码不进入审计。
- [ ] 增加 0133 批次注册表、必要索引和 down 迁移。
- [ ] 实现领域输入、结果和 service 方法。
- [ ] 运行 `go test ./internal/dashboard -run EmployeeAccount -count=1`，预期 PASS。
- [ ] 提交 `feat(access): add employee account lifecycle domain`。

### Task 2: MySQL 原子生命周期与 HTTP

**Files:**
- Create: `internal/store/dashboard_employee_account.go`
- Test: `internal/store/dashboard_employee_account_test.go`
- Modify: `internal/dashboard/dashboard_access_http.go`
- Modify: `cmd/mochat-go/dashboard_access_routes.go`
- Test: `internal/dashboard/dashboard_access_http_test.go`

**Interfaces:**
- Consumes: Task 1 的命令和结果类型。
- Produces: `/dashboard/access/employees/*` 四类 API。

- [ ] 写 store/HTTP 失败测试，断言 actor `FOR UPDATE` 是第一条 SQL、创建/绑定/审计同事务、冲突 409、跨租户 404。
- [ ] 实现 MySQL store，并在审计失败时回滚全部写入。
- [ ] 实现严格 JSON 路由和稳定错误 envelope。
- [ ] 运行 `go test ./internal/dashboard ./internal/store -run EmployeeAccount -count=1`，预期 PASS。
- [ ] 提交 `feat(access): add employee account administration api`。

### Task 3: 员工权限页面

**Files:**
- Modify: `web/apps/dashboard/src/features/access/access-admin-api.ts`
- Modify: `web/apps/dashboard/src/features/company-settings/access-staff-page.tsx`
- Test: `web/apps/dashboard/src/features/company-settings/access-pages.test.tsx`

**Interfaces:**
- Consumes: Task 2 HTTP API。
- Produces: 员工主表、账号状态、开通/停启用/重置交互。

- [ ] 写失败交互测试：未开通员工可见、首次保存不写、确认后 payload、临时密码一次展示、停用和重置确认、390px 可达。
- [ ] 扩展强类型 API client。
- [ ] 实现员工主表与对话框，不展示继承权限。
- [ ] 运行定向 Vitest、typecheck、lint、build，预期 PASS。
- [ ] 提交 `feat(ui): manage dashboard accounts from synced employees`。

### Task 4: 隔离式会话存档模拟器

**Files:**
- Create: `internal/archivefixture/service.go`
- Create: `internal/archivefixture/mysql.go`
- Test: `internal/archivefixture/service_test.go`
- Create: `cmd/mochat-archive-simulator/main.go`
- Modify: `Dockerfile`

**Interfaces:**
- Produces: `apply/status/cleanup` CLI；批次前缀 `MOCHAT-SIM:<batch>:`。

- [ ] 写失败测试：数据矩阵完整、重复 apply 幂等、游标/配置不写、cleanup 保留真实数据。
- [ ] 实现 fixture 生成器和事务型 MySQL 注册/清理。
- [ ] 实现 CLI 参数校验，DSN 仅从环境读取，输出不含凭据。
- [ ] 运行 `go test ./internal/archivefixture ./cmd/mochat-archive-simulator -count=1`，预期 PASS。
- [ ] 提交 `feat(archive): add isolated conversation simulator`。

### Task 5: 完成门禁与现有服务器验收

**Files:**
- Modify only when a RED exposes a production defect.

- [ ] 运行 `go test ./internal/dashboard ./internal/store ./internal/archivefixture ./cmd/mochat-archive-simulator -count=1`。
- [ ] 运行 Dashboard 定向测试、typecheck、lint、build。
- [ ] 构建 linux/amd64 镜像并上传，不在服务器编译。
- [ ] 记录 app/mysql/redis 和四卷，执行 0133 migration，只重建 app。
- [ ] 在真实页面开通一名员工账号，验证首次改密 challenge、停启用、重置和审计；不得在证据中记录临时密码。
- [ ] 执行 simulator apply 两次，验证幂等、真实游标/配置不变及员工/客户/群会话页面；保留模拟数据供用户检查。
- [ ] 提交最终修复并报告精确 SHA 与验证证据。

