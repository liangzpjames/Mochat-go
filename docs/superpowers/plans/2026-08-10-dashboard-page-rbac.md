# Dashboard 页面 RBAC Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 53 个 Dashboard 页面建立独立、租户隔离、服务端失败关闭的页面 RBAC，并交付多角色、直接权限、数据范围、原子审计管理及非 Docker 全门禁；真实 Docker/browser 最终验收由主任务执行。

**Architecture:** 迁移 `0127_dashboard_page_rbac` 建立权限目录、API 资源映射和租户关联表；`DashboardAccessService` 统一计算 SaaS 门槛、页面权限来源与数据范围，并由 server 前置 guard 和 `/dashboard/access/*` 管理 handler 共用。React loader 改读 `/access/profile`，manifest 只负责 53 页呈现结构，权限事实由服务端返回。

**Tech Stack:** Go 1.25、`net/http`、`database/sql`、MariaDB/MySQL 5.7 兼容 SQL、React 19、TypeScript、React Router、TanStack Query、Ant Design、Vitest、Playwright、Docker Desktop。

## Global Constraints

- 所有用户审阅文档使用中文；代码标识符、命令、路径和 API 字段保留原文。
- 仅在 `D:\workspace\mochat-go\mochat-go\.worktrees\phase4-dashboard-page-rbac` 写入，禁止 `reset`、`clean` 或覆盖主工作区 dirty 文件。
- SaaS 只控制套餐、额度、订阅和租户整体生效，不控制 Dashboard 页面；Dashboard 不含 SaaS 链接。
- `tenant_id` 只来自认证身份；跨租户管理目标返回 `404`；复合外键与应用事务同时保证隔离。
- `/company-setting/staff`、`/setting/role`、`/setting/additional`、`/setting/authorization` 及管理 API 永久 `superadmin_only`。
- 普通用户有效权限为直接权限与所有启用角色权限的并集；数据范围 `tenant > department > self`；直接权限默认 `self`。
- 普通用户未映射 Dashboard API 默认 `403`；superadmin 仅在同 tenant 隐式全部。
- 每项生产代码必须先有可解释的 RED，再写最小 GREEN；每个逻辑批次独立提交。
- 本分支只完成非 Docker 全门禁；Docker Task 10 由主任务在 `mochat-go-desktop` 执行。本分支不得重建 `app`、MySQL/Redis 或卷，也不得执行 `down -v`、`volume rm`、`system prune`。
- 最终证据写入 `D:\workspace\mochat-go\output`；route/test passed 不代表产品完成。

---

### Task 1: 锁定 53 页目录和 0127 迁移契约

**Files:**
- Create: `internal/dashboard/dashboard_page_catalog.json`
- Create: `scripts/check_dashboard_page_rbac_catalog.mjs`
- Create: `scripts/check_dashboard_page_rbac_catalog.test.mjs`
- Create: `deploy/standalone/migrations/0127_dashboard_page_rbac.up.sql`
- Create: `deploy/standalone/migrations/0127_dashboard_page_rbac.down.sql`
- Modify: `deploy/standalone/migrations/README.md`
- Test: `internal/migration/migration_test.go`
- Test: `internal/migration/dashboard_page_rbac_integration_test.go`

**Interfaces:**
- Produces: `DashboardPageCatalog` 的 53 个 `{code,path,name,groupCode,sort,superadminOnly,resources[]}` 条目；每个资源显式包含 `scopeRequired`。
- Produces: 六张 `mochat_go_dashboard_*` 表、`mc_user`/`mc_rbac_role.dashboard_access_version` 及 `(tenant_id,id)` 复合约束。
- Consumes: `web/apps/dashboard/src/benchmark/manifest.json`。

- [ ] **Step 1: 写目录门禁 RED**

在 `scripts/check_dashboard_page_rbac_catalog.test.mjs` 构造缺页、重复 path、错误管理页标记、重复资源、前端调用未映射、server handler 未登记和 fallback 放行七种 fixture，调用导出的 `validateDashboardPageRBACCatalog()`，分别断言明确错误文本；有效 fixture 断言 `pageCount=53`、`ordinaryPageCount=49`、`superadminOnlyCount=4`。

- [ ] **Step 2: 运行 RED**

Run: `node --test scripts/check_dashboard_page_rbac_catalog.test.mjs`

Expected: FAIL，原因是 `check_dashboard_page_rbac_catalog.mjs` 或导出函数不存在。

- [ ] **Step 3: 写最小目录校验器和 JSON**

校验器导出以下接口，并在直接执行时读取 manifest 与 catalog：

```js
export function validateDashboardPageRBACCatalog({ manifest, catalog, apiUsages }) {
  const expectedAdmin = new Set([
    '/company-setting/staff', '/setting/role',
    '/setting/additional', '/setting/authorization',
  ]);
  // 返回 {pageCount, ordinaryPageCount, superadminOnlyCount, resourceCount}；
  // 任一清单、标记或资源覆盖不一致时 throw Error。
}
```

`dashboard_page_catalog.json` 完整列出 manifest 的 53 页；资源使用 `METHOD /dashboard/path`，同一资源可属于多个页面。每个资源必须显式声明 `scopeRequired: true|false`；校验器扫描 React API 调用、server switch/module 注册和所有访问 employee/corp 数据的 handler/store 调用链，后者必须标记为 `true`。系统豁免使用精确 method+path 常量且不放入页面资源。除精确豁免外，任何 manifest、前端调用、server 注册、scope 标记或 catalog 漂移都失败，不能以旧菜单或 fallback 通过。

- [ ] **Step 4: 运行 GREEN 并记录资源覆盖**

Run: `node --test scripts/check_dashboard_page_rbac_catalog.test.mjs && node scripts/check_dashboard_page_rbac_catalog.mjs`

Expected: PASS，并打印 `53 pages, 49 ordinary, 4 superadmin_only, 0 unmapped dashboard API usages`。

- [ ] **Step 5: 写迁移 RED**

在 `internal/migration/migration_test.go` 新增测试，读取 DefaultMigrations 最后一项并断言版本、up/down 文件存在；读取 SQL 断言六张表、53 个权限 seed、四个 `superadmin_only=1`、跨租户和缺失 0039 subscription 两类动态 `SIGNAL SQLSTATE '45000'`、两个 `dashboard_access_version`、复合外键、直接唯一键和旧关系回填语句存在。断言预检均位于第一条 DDL 前，使用仓库 0106 的 `SET` + `PREPARE/EXECUTE` 兼容方式且不含 `DELIMITER`/存储过程；断言 `tenant_id int(11)` signed、`user_id int(10) unsigned`、`role_id int(11)` signed、`permission_id bigint(20) unsigned` 与父表一致，关系表不存在 nullable `deleted_at` 唯一键，也不使用 `ADD COLUMN IF NOT EXISTS`。

- [ ] **Step 6: 运行迁移 RED**

Run: `go test ./internal/migration -run DashboardPageRBAC -count=1`

Expected: FAIL，缺少 `0127_dashboard_page_rbac`。

- [ ] **Step 7: 写 up/down SQL**

up SQL 顺序固定为：有效 package 缺少 0039 subscription 检测 → 跨租户脏数据检测 → 两个聚合版本字段 → 复合唯一索引 → 六张表 → 53 页和完整资源 seed → 同 tenant 旧 user-role 回填 → 可映射旧 role-menu 回填。两项数据预检必须在第一条 DDL 前完成；MariaDB/MySQL DDL 会隐式提交，不宣称整体事务原子性。runner 按分号拆分，预检沿用 0106 动态 `SIGNAL`，禁止 `DELIMITER` 和存储过程。`user_roles`、`role_permissions`、`user_permissions` 使用 `(tenant_id, target_id)` 复合外键和直接唯一键，不设软删除；关系更新由应用事务物理替换，历史只进入 append-only audit。`permission_audits.actor_user_id` 可空，非空时使用 tenant+user 复合外键，`target_id varchar(64)`。down 先按外键逆序删六张表，再删两个版本字段和 0127 新增复合索引，不修改旧关联数据。

- [ ] **Step 8: 写并运行真实 MariaDB 迁移门禁**

编写隔离数据库集成测试，验证：缺 subscription 或跨 tenant 旧关系时第一条 DDL 尚未发生；正常 apply 后 ledger 才记录 0127；down 删除六表、两个版本字段和两个复合索引；随后再次 apply 成功；人为制造中途 DDL 失败时 ledger 不记账、错误可诊断，并按 down/修复路径恢复。开发会话只编写测试，不自行操作 Docker；`MOCHAT_GO_MYSQL_INTEGRATION_DSN` 缺失时明确 SKIP。主任务在 `mochat-go-desktop` 最终阶段提供隔离临时数据库 DSN，执行 apply→down→apply 并留证。

Run: `go test ./internal/migration -run DashboardPageRBACIntegration -count=1`

- [ ] **Step 9: 运行 GREEN 和迁移静态门禁**

Run: `go test ./internal/migration -run DashboardPageRBAC -count=1 && node scripts/check_dashboard_page_rbac_catalog.mjs && git diff --check`

Expected: PASS。

- [ ] **Step 10: 提交**

```powershell
git add internal/dashboard/dashboard_page_catalog.json scripts/check_dashboard_page_rbac_catalog.mjs scripts/check_dashboard_page_rbac_catalog.test.mjs deploy/standalone/migrations/0127_dashboard_page_rbac.up.sql deploy/standalone/migrations/0127_dashboard_page_rbac.down.sql deploy/standalone/migrations/README.md internal/migration/migration_test.go internal/migration/dashboard_page_rbac_integration_test.go
git commit -m "feat(rbac): add dashboard page permission schema"
```

### Task 2: SaaS 整体访问门槛失败关闭

**Files:**
- Create: `internal/dashboard/dashboard_tenant_gate.go`
- Create: `internal/dashboard/dashboard_tenant_gate_test.go`
- Create: `internal/store/dashboard_tenant_gate.go`
- Test: `internal/store/dashboard_tenant_gate_test.go`
- Modify: `internal/dashboard/auth.go`
- Modify: `internal/dashboard/auth_test.go`
- Modify: `internal/dashboard/saas_identity_security.go`
- Modify: `internal/dashboard/saas_identity_security_test.go`

**Interfaces:**
- Produces: `DashboardTenantAccessStore.DashboardTenantAccess(ctx, tenantID, now) (DashboardTenantAccess, error)`。
- Produces: `AuthHandler.WithDashboardTenantGate(gate DashboardTenantGate) *AuthHandler`。
- Consumes: tenant、tenant package、limits snapshot 与 subscription lifecycle。

- [ ] **Step 1: 写门槛矩阵 RED**

为 `EvaluateDashboardTenantAccess` 表驱动覆盖：tenant 非 1、套餐缺失/停用/未生效/过期、`limits_json` 空/null/数组/坏 JSON、订阅缺失/暂停/取消后不可访问、存储错误；另覆盖启用、有效期内、object snapshot，以及由现有 `SaaSAdminEffectiveSubscriptionStatus`/`SaaSAdminSubscriptionAllowsAccess` 判定可访问的 active/trialing/grace。0039 已回填历史 package；0127 的前置一致性检查在 Task 1 保证合法历史租户不会到运行期才被锁死。

```go
type DashboardTenantAccess struct {
    TenantID int
    Allowed bool
    Reason string
}
```

- [ ] **Step 2: 运行 RED**

Run: `go test ./internal/dashboard -run DashboardTenantAccess -count=1`

Expected: FAIL，缺少类型与函数。

- [ ] **Step 3: 写最小领域判断和 MySQL 查询**

MySQL 查询使用认证 tenant 参数，返回 tenant status、package status/start/end、原始 `limits_json`、subscription 状态与周期字段。任何 `sql.ErrNoRows` 或 JSON 解码失败返回 `Allowed=false`，数据库错误原样返回。

- [ ] **Step 4: 运行领域和 store GREEN**

Run: `go test ./internal/dashboard ./internal/store -run DashboardTenantAccess -count=1`

Expected: PASS。

- [ ] **Step 5: 写登录/MFA RED**

在 `auth_test.go` 与 `saas_identity_security_test.go` 断言密码正确但门槛失败时不签 token、返回 `403` 和稳定 machine code `TENANT_ACCESS_DENIED`；gate error 返回 `500`。MFA 完成同样覆盖，禁止靠中文 `msg` 判断。

- [ ] **Step 6: 运行 RED 并实现注入**

Run before implementation: `go test ./internal/dashboard -run 'Auth.*DashboardTenantGate|MFA.*DashboardTenantGate' -count=1`

Expected: FAIL，登录仍沿用缺套餐/缺订阅放行。

实现后 Run: `go test ./internal/dashboard -run 'Auth|MFA' -count=1`

Expected: PASS。

- [ ] **Step 7: 提交**

```powershell
git add internal/dashboard/dashboard_tenant_gate.go internal/dashboard/dashboard_tenant_gate_test.go internal/store/dashboard_tenant_gate.go internal/store/dashboard_tenant_gate_test.go internal/dashboard/auth.go internal/dashboard/auth_test.go internal/dashboard/saas_identity_security.go internal/dashboard/saas_identity_security_test.go
git commit -m "feat(rbac): fail closed on tenant access gate"
```

### Task 3: 多角色、直接权限、来源和数据范围解析

**Files:**
- Create: `internal/dashboard/dashboard_access.go`
- Create: `internal/dashboard/dashboard_access_test.go`
- Create: `internal/store/dashboard_access.go`
- Test: `internal/store/dashboard_access_test.go`

**Interfaces:**
- Produces: `DashboardAccessService.Resolve(ctx, userID, corpID) (DashboardAccessProfile, error)`。
- Produces: `DashboardAccessProfile.AllowsPage(path string) bool` 与 `ScopeFor(code string) DataScope`。
- Consumes: Task 1 catalog、Task 2 tenant gate、启用角色和直接权限行。

- [ ] **Step 1: 写解析 RED**

测试至少覆盖：普通用户无权限；两个启用角色并集；停用角色不贡献；直接权限保留；同权限多来源返回全部 source；范围 `tenant > department > self`；四个管理页不能由角色或直接权限获得；superadmin 53 页隐式全有且无授权行；目标 user tenant 与认证 user tenant 不同返回 not found。

```go
type DataScope string
const (
    DataScopeSelf DataScope = "self"
    DataScopeDepartment DataScope = "department"
    DataScopeTenant DataScope = "tenant"
)
type PermissionSource struct { Type string; ID int; Name string; Scope DataScope }
type EffectivePermission struct { Code, Path, Name string; Scope DataScope; Sources []PermissionSource }
```

- [ ] **Step 2: 运行 RED**

Run: `go test ./internal/dashboard -run DashboardAccess -count=1`

Expected: FAIL，解析器不存在。

- [ ] **Step 3: 写最小解析器与 scoped SQL**

store 的所有读 SQL 第一条件为 `tenant_id = ?`；角色只取 `status=1` 且未删除；直接权限默认 scope self；superadmin 直接从目录生成 53 条结果。按 permission code 排序 sources，保证响应稳定。解析器先把每个已授权资源的最终 scope 纳入 profile，供 Task 4 写入 `DashboardAccessContext`。

- [ ] **Step 4: 运行 GREEN**

Run: `go test ./internal/dashboard ./internal/store -run DashboardAccess -count=1`

Expected: PASS。

- [ ] **Step 5: 回归旧 RBAC 数据范围测试**

Run: `go test ./internal/dashboard -run 'RBAC|Permission' -count=1`

Expected: PASS；新解析器不破坏尚未切换的旧 handler。

- [ ] **Step 6: 提交**

```powershell
git add internal/dashboard/dashboard_access.go internal/dashboard/dashboard_access_test.go internal/store/dashboard_access.go internal/store/dashboard_access_test.go
git commit -m "feat(rbac): resolve direct and multi-role access"
```

### Task 4: API 资源 guard 与普通用户未映射默认拒绝

**Files:**
- Create: `internal/dashboard/dashboard_access_guard.go`
- Create: `internal/dashboard/dashboard_access_guard_test.go`
- Modify: `internal/server/server.go`
- Modify: `internal/server/server_test.go`
- Modify: `cmd/mochat-go/main.go`

**Interfaces:**
- Produces: `DashboardRequestGuard.Authorize(http.ResponseWriter, *http.Request) bool`。
- Produces: `server.WithDashboardRequestGuard(guard DashboardRequestGuard) Option`。
- Consumes: Task 3 access profile与 Task 1 resource matcher。

- [ ] **Step 1: 写 guard RED**

覆盖：无 token `401`；门槛失败 `403 + TENANT_ACCESS_DENIED`；普通用户已映射且有任一权限放行；无权限和未映射 `/dashboard/newUnknown` 均返回 `403 + DASHBOARD_PERMISSION_DENIED`；superadmin 同 tenant 放行；`/dashboard/saasAdmin/*` 不进入本 guard；auth/MFA/logout/corp select/bind/profile 与明确 callback 精确豁免；相似前缀不能借豁免放行。

- [ ] **Step 2: 运行 RED**

Run: `go test ./internal/dashboard -run DashboardAccessGuard -count=1`

Expected: FAIL，guard 不存在。

- [ ] **Step 3: 实现 matcher、context 与 guard**

定义 `DashboardAccessFromContext(ctx)`；路径 pattern 只支持完整静态段和 `{id}` 单段。普通用户资源查不到即拒绝。匹配资源后，guard 把认证 tenant、corp、permission code、最终 scope 和 `scopeRequired` 写入 `DashboardAccessContext`。管理 API 由 handler 再验证 superadmin，guard 不根据请求 body 获取 tenant。

catalog `scopeRequired=true` 对应的 employee/corp handler 必须使用 `DashboardAccessFromContext`；其 store 查询接收已解析的 allowed employee IDs/department IDs 或 tenant scope，并把认证 tenant/corp 作为固定过滤条件，禁止调用旧首角色 `DataPermission`。新增表驱动测试逐条遍历 catalog 的 scope 资源，断言 handler 已注册 scope consumer；漏消费或继续走旧路径即失败。

- [ ] **Step 4: 写 server 前置顺序 RED**

`server_test.go` 用 recording guard 断言 module router 和 legacy switch handler 在 guard 返回 false 时均未被调用；SaaS admin 不调用 guard；标准化 `/undefined/dashboard/...` 后只调用一次 guard。

- [ ] **Step 5: 运行 RED、实现 server option 和装配**

Run before implementation: `go test ./internal/server -run DashboardRequestGuard -count=1`

Expected: FAIL。

在 `Server.ServeHTTP` 完成 bundled path 标准化后、module router 前调用 guard；`main.go` 构建同一个 service 并注入 server 与 auth gate。

Run after implementation: `go test ./internal/server ./internal/dashboard -run Dashboard -count=1`

Expected: PASS。

- [ ] **Step 6: 提交**

```powershell
git add internal/dashboard/dashboard_access_guard.go internal/dashboard/dashboard_access_guard_test.go internal/server/server.go internal/server/server_test.go cmd/mochat-go/main.go
git commit -m "feat(rbac): guard dashboard api resources"
```

### Task 5: 租户隔离的管理读取与原子写入审计

**Files:**
- Create: `internal/dashboard/dashboard_access_admin.go`
- Create: `internal/dashboard/dashboard_access_admin_test.go`
- Create: `internal/store/dashboard_access_admin.go`
- Test: `internal/store/dashboard_access_admin_test.go`

**Interfaces:**
- Produces: `Profile`, `Catalog`, `Users`, `User`, `Roles`, `Audits`, `ReplaceUserAccess`, `CreateRole`, `UpdateRole`, `UpdateRoleStatus`, `DeleteRole`。
- Consumes: `expectedVersion` 与认证 `tenant_id`；用户集合版本来自 `mc_user.dashboard_access_version`，角色集合版本来自 `mc_rbac_role.dashboard_access_version`。

- [ ] **Step 1: 写管理领域 RED**

handler/service fake store 测试：profile 对普通用户开放；其余 API 普通用户 `403`；同 tenant superadmin 可读；跨 tenant `{id}` `404`；未知 ID `404`；用户/角色聚合 `expectedVersion` 冲突 `409`；四个管理权限不能写给普通角色/用户；角色 create/update/status/delete 完整；有 `user_roles` 成员的角色删除返回 `409`。

- [ ] **Step 2: 运行 RED**

Run: `go test ./internal/dashboard -run DashboardAccessAdmin -count=1`

Expected: FAIL。

- [ ] **Step 3: 实现读取契约**

所有分页响应使用 `{list,page:{page,perPage,total,totalPage}}`；user detail 返回 `roles`、`directPermissions`、`inheritedPermissions`、`effectivePermissions`、`version`；audit 返回 actor/action/target/before/after/version/time。

- [ ] **Step 4: 写事务 RED**

用 `sqlmock` 或仓库已有测试 DB 覆盖：用户写先执行 `SELECT dashboard_access_version ... FROM mc_user WHERE tenant_id=? AND id=? FOR UPDATE`；角色写执行对应 `mc_rbac_role` 锁；版本不符无写入；角色/权限越界回滚；关系物理 DELETE+INSERT；关联写成功但 audit 失败回滚；全部成功仅 commit 一次并 compare-and-increment 聚合版本；删除有成员角色在任何 DELETE 前返回 `409`。

- [ ] **Step 5: 运行 RED、实现 `sql.Tx`**

Run before implementation: `go test ./internal/store -run DashboardAccessAdmin -count=1`

Expected: FAIL。

实现 `ReplaceUserDashboardAccess`、`CreateDashboardRole`、`UpdateDashboardRole`、`UpdateDashboardRoleStatus` 与 `DeleteDashboardRole`，所有关系物理 delete/insert、聚合版本 update 与 audit 使用同一 tx；新角色写入现有 `mc_rbac_role` 必填字段，`operate_id`/`operate_name` 仅由认证 actor 生成，禁止客户端提供。角色 `status` 只允许 `1=启用`、`2=禁用`，用户状态沿用 `1=正常`、`2=禁用`。禁止从 input 接受 tenant，禁止用关系行 version 冒充集合版本。

Run after implementation: `go test ./internal/dashboard ./internal/store -run DashboardAccessAdmin -count=1`

Expected: PASS。

- [ ] **Step 6: 提交**

```powershell
git add internal/dashboard/dashboard_access_admin.go internal/dashboard/dashboard_access_admin_test.go internal/store/dashboard_access_admin.go internal/store/dashboard_access_admin_test.go
git commit -m "feat(rbac): add atomic access administration"
```

### Task 6: `/dashboard/access/*` HTTP 契约与旧管理 API superadmin-only

**Files:**
- Create: `internal/dashboard/dashboard_access_http.go`
- Create: `internal/dashboard/dashboard_access_http_test.go`
- Modify: `internal/server/server.go`
- Modify: `internal/server/server_test.go`
- Modify: `cmd/mochat-go/main.go`
- Modify: `internal/dashboard/user_admin.go`
- Modify: `internal/dashboard/user_admin_test.go`
- Modify: `internal/dashboard/role_admin.go`
- Modify: `internal/dashboard/role_admin_test.go`
- Modify: `internal/dashboard/menu_admin.go`
- Modify: `internal/dashboard/menu_admin_test.go`

**Interfaces:**
- Produces: prefix handler `NewDashboardAccessHTTP(service, resolver)`。
- Produces: `server.WithDashboardAccessHandler(handler http.Handler) Option`。

- [ ] **Step 1: 写路由与 envelope RED**

表驱动覆盖 GET profile/catalog/users/users/{id}/roles/audits、PUT users/{id}、POST roles、PUT roles/{id}、PUT roles/{id}/status、DELETE roles/{id}；断言 method、JSON decode、query pagination、HTTP status 和 `{code,msg,data}`。门槛拒绝稳定返回 `TENANT_ACCESS_DENIED`，权限拒绝稳定返回 `DASHBOARD_PERMISSION_DENIED`；body 含 `tenantId`、`operateId` 或 `operateName` 时返回 `400`；删除有成员角色返回 `409`。

- [ ] **Step 2: 运行 RED**

Run: `go test ./internal/dashboard -run DashboardAccessHTTP -count=1`

Expected: FAIL。

- [ ] **Step 3: 实现 prefix handler 和 server 路由**

`/dashboard/access/profile` 精确匹配；`users/{id}` 和 `roles/{id}` 仅接受单个正整数段；未知 access path `404`。server 在 guard 通过后、module router 前交给 access handler。

- [ ] **Step 4: 写旧管理 API RED 并最小加固**

普通用户分别请求旧 user/role/menu 管理读写接口，断言 `403`；superadmin 同 tenant 保持兼容。明确保留 `/dashboard/user/store`、`user/update`、`user/statusUpdate`、`user/passwordReset` 处理账号创建、资料、启停和密码；`PUT /access/users/{id}` 仅处理多角色与直接权限。旧 role/menu 写接口不再是 Phase 4 UI 的权威入口。将公共检查收敛为 `requireTenantSuperAdmin`，不复制多份 tenant 逻辑。

- [ ] **Step 5: 运行 GREEN**

Run: `go test ./internal/dashboard ./internal/server -run 'DashboardAccessHTTP|UserAdmin|RoleAdmin|MenuAdmin' -count=1`

Expected: PASS。

- [ ] **Step 6: 提交**

```powershell
git add internal/dashboard/dashboard_access_http.go internal/dashboard/dashboard_access_http_test.go internal/server/server.go internal/server/server_test.go cmd/mochat-go/main.go internal/dashboard/user_admin.go internal/dashboard/user_admin_test.go internal/dashboard/role_admin.go internal/dashboard/role_admin_test.go internal/dashboard/menu_admin.go internal/dashboard/menu_admin_test.go
git commit -m "feat(rbac): expose tenant access management api"
```

### Task 7: 前端 profile、导航和深链使用同一权限事实

**Files:**
- Create: `web/apps/dashboard/src/features/access/access-api.ts`
- Create: `web/apps/dashboard/src/features/access/access-api.test.ts`
- Modify: `web/apps/dashboard/src/app/access-loader.ts`
- Modify: `web/apps/dashboard/src/app/access-loader.test.ts`
- Modify: `web/apps/dashboard/src/app/access-context.tsx`
- Modify: `web/apps/dashboard/src/main.tsx`
- Modify: `web/apps/dashboard/src/layout/dashboard-layout.tsx`
- Modify: `web/apps/dashboard/src/layout/dashboard-layout.test.tsx`
- Modify: `web/apps/dashboard/src/features/corp/corp-provider.tsx`
- Modify: `web/apps/dashboard/src/features/corp/corp-provider.test.tsx`

**Interfaces:**
- Produces: `loadAccessProfile(client): Promise<AccessProfile>`。
- Consumes: `GET /access/profile`，注意 ApiClient 已提供 `/dashboard` base，前端必须使用相对路径。

- [ ] **Step 1: 写 API mapping RED**

断言 request 为 `/access/profile`，并把 `effectivePermissions[].path` 映射为 `allowedRoutes`，保留 sources、scope 与 `isSuperAdmin`。

- [ ] **Step 2: 运行 RED**

Run: `corepack pnpm --filter @mochat/dashboard test -- src/features/access/access-api.test.ts`

Expected: FAIL，文件不存在。

- [ ] **Step 3: 写 loader RED**

删除测试 deps 中 `benchmarkRoutes`，新增 `loadProfile`；断言无权限 benchmark 深链返回 `403 + DASHBOARD_PERMISSION_DENIED`、有权限正常、53 页 superadmin profile 正常、tenant gate `403 + TENANT_ACCESS_DENIED` 清 session 并 redirect login、普通权限 `403 + DASHBOARD_PERMISSION_DENIED` 保留 session。增加 `ApiClient` 测试证明 machine code 原样透传，禁止解析中文 `msg`。

- [ ] **Step 4: 运行 RED、实现 profile loader**

Run before implementation: `corepack pnpm --filter @mochat/dashboard test -- src/app/access-loader.test.ts`

Expected: FAIL，旧 benchmark allowlist 仍放行。

实现后 Run: `corepack pnpm --filter @mochat/dashboard test -- src/features/access/access-api.test.ts src/app/access-loader.test.ts`

Expected: PASS。

- [ ] **Step 5: 写 Dashboard 无 SaaS 链接 RED**

修改 layout 测试，断言 `queryByRole('link',{name:'SaaS 管理后台'})` 为 null；导航与搜索只展示 allowedRoutes；空权限显示“暂无可访问功能”。

- [ ] **Step 6: 运行 RED、删除链接与 `benchmarkRoutes`**

Run before implementation: `corepack pnpm --filter @mochat/dashboard test -- src/layout/dashboard-layout.test.tsx`

Expected: FAIL，仍存在 SaaS 链接。

实现后 Run: `corepack pnpm --filter @mochat/dashboard test -- src/app/access-loader.test.ts src/layout/dashboard-layout.test.tsx`

Expected: PASS。

- [ ] **Step 7: 提交**

```powershell
git add web/apps/dashboard/src/features/access web/apps/dashboard/src/app/access-loader.ts web/apps/dashboard/src/app/access-loader.test.ts web/apps/dashboard/src/app/access-context.tsx web/apps/dashboard/src/main.tsx web/apps/dashboard/src/layout/dashboard-layout.tsx web/apps/dashboard/src/layout/dashboard-layout.test.tsx web/apps/dashboard/src/features/corp/corp-provider.tsx web/apps/dashboard/src/features/corp/corp-provider.test.tsx
git commit -m "feat(rbac): enforce page profile in dashboard shell"
```

### Task 8: 多角色、直接/继承/有效来源、权限树与审计 UI

**Files:**
- Create: `web/apps/dashboard/src/features/access/access-admin-api.ts`
- Create: `web/apps/dashboard/src/features/access/access-admin-api.test.ts`
- Create: `web/apps/dashboard/src/features/access/permission-tree.tsx`
- Create: `web/apps/dashboard/src/features/access/permission-tree.test.tsx`
- Create: `web/apps/dashboard/src/features/access/user-access-panel.tsx`
- Create: `web/apps/dashboard/src/features/access/role-access-panel.tsx`
- Create: `web/apps/dashboard/src/features/access/audit-list.tsx`
- Modify: `web/apps/dashboard/src/features/company-settings/staff-page.tsx`
- Modify: `web/apps/dashboard/src/features/company-settings/role-page.tsx`
- Modify: `web/apps/dashboard/src/features/company-settings/additional-page.tsx`
- Modify: `web/apps/dashboard/src/features/company-settings/authorization-page.tsx`
- Modify: `web/apps/dashboard/src/features/company-settings/company-settings-pages.test.tsx`
- Modify: `web/apps/dashboard/src/benchmark/page-registry.tsx`
- Modify: `web/apps/dashboard/src/main.tsx`
- Modify: `web/apps/dashboard/src/styles/index.css`

**Interfaces:**
- Produces: `createAccessAdminApi(client)` 的 users/user/roles/catalog/audits/saveUser/createRole/updateRole/updateRoleStatus/deleteRole。
- Consumes: 用户或角色聚合 `expectedVersion`、`roleIds[]`、`directPermissions[]`、`permissions[]`。

- [ ] **Step 1: 写 API RED**

断言所有 path 均相对 `/access/...`，用户集合 PUT、角色 POST/PUT/status/DELETE body 精确包含所需字段且不含 tenantId；`409` 由 ApiClient 保留为 conflict，有成员角色删除展示明确原因。

- [ ] **Step 2: 运行 RED、实现 API**

Run before implementation: `corepack pnpm --filter @mochat/dashboard test -- src/features/access/access-admin-api.test.ts`

Expected: FAIL。

实现后同命令 Expected: PASS。

- [ ] **Step 3: 写权限树 RED**

覆盖 53 节点中文标题/路径、四个管理节点只读、直接/继承/有效来源标签、多来源列表、scope 默认 self 和 tenant/department/self 显示顺序；390px DOM 不产生固定最小宽度。

- [ ] **Step 4: 运行 RED、实现树与 panels**

Run before implementation: `corepack pnpm --filter @mochat/dashboard test -- src/features/access/permission-tree.test.tsx`

Expected: FAIL。

实现 `PermissionTree` 后 Expected: PASS。

- [ ] **Step 5: 写四页真实交互 RED**

`company-settings-pages.test.tsx` 覆盖：员工多角色选择；直接权限与继承来源区分；停用角色提示影响但不删除直接权限；角色权限保存；审计过滤；确认前不 mutation；提交带 expectedVersion；409 保留表单并提示刷新。

- [ ] **Step 6: 运行 RED、接入四页**

Run before implementation: `corepack pnpm --filter @mochat/dashboard test -- src/features/company-settings/company-settings-pages.test.tsx`

Expected: FAIL，旧页仍使用单角色/菜单 API。

实现后同命令 Expected: PASS。

- [ ] **Step 7: 运行 Dashboard 全测试、typecheck、build**

Run: `corepack pnpm --filter @mochat/dashboard test && corepack pnpm --filter @mochat/dashboard typecheck && corepack pnpm --filter @mochat/dashboard build`

Expected: 0 failures，exit 0。

- [ ] **Step 8: 提交**

```powershell
git add web/apps/dashboard/src/features/access web/apps/dashboard/src/features/company-settings web/apps/dashboard/src/benchmark/page-registry.tsx web/apps/dashboard/src/main.tsx web/apps/dashboard/src/styles/index.css
git commit -m "feat(rbac): add dashboard access management ui"
```

### Task 9: 全清单门禁、集成测试与验收脚本

**Files:**
- Create: `scripts/check_dashboard_page_rbac_completion.mjs`
- Create: `scripts/check_dashboard_page_rbac_completion.test.mjs`
- Create: `scripts/smoke_dashboard_page_rbac.ps1`
- Create: `web/e2e/tests/dashboard-page-rbac.spec.ts`
- Modify: `package.json`
- Modify: `web/e2e/package.json`
- Test: `internal/store/dashboard_access_integration_test.go`
- Test: `internal/store/dashboard_access_scope_integration_test.go`

**Interfaces:**
- Produces: `pnpm check:phase4-dashboard-page-rbac`。
- Produces: Docker/API/SQL evidence under caller-provided `RBAC_EVIDENCE_DIR`。

- [ ] **Step 1: 写完成门禁 RED**

测试故意保留 `benchmarkRoutes`、SaaS link、52 页 catalog、普通管理页、前端调用未映射、server handler 未登记、fallback 放行、`scopeRequired` 资源仍读取旧首角色 `DataPermission`、缺少 390px case 时分别失败；完整 fixture 通过。完成门禁输出每个需 employee/corp 数据范围的 API 与实际 handler 的一一映射，不能把“后续逐步替代”作为通过条件。

- [ ] **Step 2: 运行 RED、实现门禁**

Run before implementation: `node --test scripts/check_dashboard_page_rbac_completion.test.mjs`

Expected: FAIL。

实现后 Run: `node --test scripts/check_dashboard_page_rbac_completion.test.mjs && node scripts/check_dashboard_page_rbac_completion.mjs`

Expected: PASS，打印 53/53 catalog、49 ordinary、4 superadmin_only、0 benchmark bypass、0 unmapped API。

- [ ] **Step 3: 写 MySQL integration RED**

以独立 namespace 建两个 tenant，覆盖 FK signed/unsigned 可真实创建、复合外键拒绝跨 tenant、关系直接唯一键、同 tenant 多角色并集、角色停用即时移除、直接权限保留、用户/角色聚合 expectedVersion 冲突、关系物理替换、角色 CRUD、有成员删除 409、audit rollback。测试枚举 catalog 全部 `scopeRequired=true` 资源，定位其 handler 和 store consumer；对每条调用链注入 tenant/department/self 三种 `DashboardAccessContext`，分别断言全 tenant、同部门、本人数据集合以及 tenant/corp 固定边界，并用冲突的旧首角色 `DataPermission` 证明旧值不被读取。开发会话缺 `MOCHAT_GO_MYSQL_INTEGRATION_DSN` 时明确 SKIP，主任务最终阶段用隔离临时数据库 DSN 跑 PASS 并留证。

- [ ] **Step 4: 运行集成 GREEN**

Run: `go test ./internal/store -run DashboardAccessIntegration -count=1`

Expected: 开发会话无 DSN 时明确 SKIP；主任务最终阶段使用隔离临时数据库 DSN，迁移与 scope 集成测试均 PASS 并留证。

- [ ] **Step 5: 写 Playwright 矩阵**

`dashboard-page-rbac.spec.ts` 必须分别验证：无权限用户 53 个深链均 `403`；普通用户直接权限页；两个角色并集；角色停用后贡献消失且直接权限保留；管理页普通用户拒绝；普通用户 49 页矩阵；superadmin 53 页矩阵；桌面与 390x844；Dashboard 无 SaaS 链接；浏览器 console 0 error、业务请求无意外 4xx/5xx。

- [ ] **Step 6: 写 PowerShell smoke**

脚本接收 `-EvidenceDir`，记录 compose project、四卷 inspect、容器 ID/镜像、租户/用户/权限/审计计数，调用 API 矩阵并把 JSON/日志写入证据目录。脚本不得出现 `down -v`、`volume rm`、`system prune`、`docker volume prune`。

- [ ] **Step 7: 运行非 Docker 全门禁**

Run: `corepack pnpm check:phase4-dashboard-page-rbac && go test ./... && corepack pnpm test && corepack pnpm typecheck && corepack pnpm build`

Expected: exit 0；记录完整测试数量。

- [ ] **Step 8: 提交**

```powershell
git add scripts/check_dashboard_page_rbac_completion.mjs scripts/check_dashboard_page_rbac_completion.test.mjs scripts/smoke_dashboard_page_rbac.ps1 web/e2e/tests/dashboard-page-rbac.spec.ts package.json web/e2e/package.json internal/store/dashboard_access_integration_test.go internal/store/dashboard_access_scope_integration_test.go
git commit -m "test(rbac): gate dashboard page access completion"
```

### Task 10: 主任务执行 Docker Desktop、浏览器与数据保留最终验收

**Files:**
- Create outside repo: `D:\workspace\mochat-go\output\phase4-dashboard-page-rbac-<timestamp>\*`
- Create: `docs/reviews/2026-08-10-dashboard-page-rbac-acceptance.zh-CN.md`

**Interfaces:**
- Consumes: 主任务对 app-only rebuild 的确认。
- Produces: 四卷前后、API、SQL、Playwright、截图、容器健康与变更边界证据。

- [ ] **Step 1: 本分支停止并向主任务交接**

非 Docker 全门禁通过后，本分支必须停止，回报分支 SHA、全部提交、测试证据和重叠文件（重点是 `cmd/mochat-go/main.go`），不得执行任何 Docker 构建、替换或卷操作。以下 Step 2-9 全部由主任务执行。

- [ ] **Step 2: 记录前置状态**

Run: `docker compose -p mochat-go-desktop -f deploy/standalone/docker-compose.yml ps --format json`

Run: `docker volume ls --format json`

把 app/mysql/redis 状态、四个目标卷 name/mountpoint/labels、关键业务表计数写入 evidence；绝不记录 JWT secret。

- [ ] **Step 3: app-only 构建与替换**

先 build 指定 app image，再执行只针对 app 的 compose up；命令不得包含 mysql、redis 或 volume 删除。替换后验证 mysql/redis container ID 未变化、四卷 name/mountpoint 未变化。

- [ ] **Step 4: 应用 0127 并跑 smoke**

Run: `powershell -File scripts/smoke_dashboard_page_rbac.ps1 -EvidenceDir <absolute-output-path>`

Expected: SaaS 门槛、跨租户、未映射 API、直接权限、多角色、停用、审计与 expectedVersion 全部 PASS。

- [ ] **Step 5: 跑桌面与 390px Playwright**

Run: `corepack pnpm --filter @mochat/e2e exec playwright test tests/dashboard-page-rbac.spec.ts --workers=1`

Expected: 49 普通页、53 超管页、53 无权限深链、4 管理页、桌面与移动矩阵全部 PASS；截图复制到 evidence。

- [ ] **Step 6: 记录后置状态并比对**

再次记录四卷、mysql/redis container ID、关键业务数据计数。允许新增 0127 schema、RBAC 审计与具名验收 fixture；原有业务表计数不得减少，原有卷不得更名或重建。

- [ ] **Step 7: 写中文验收记录**

文档分别报告：单元/静态门禁、迁移/集成、真实 API 交互、53 页深链、49/53 页面矩阵、桌面/390px、SaaS 链接移除、卷/数据保留；对未验证项明确写“未验证”，不以局部通过替代产品完成。

- [ ] **Step 8: 最终新鲜验证**

Run: `git status --short && git log --oneline --decorate -12 && git diff main...HEAD --check && corepack pnpm check:phase4-dashboard-page-rbac && go test ./... && corepack pnpm --filter @mochat/dashboard test && corepack pnpm --filter @mochat/dashboard typecheck && corepack pnpm --filter @mochat/dashboard build`

Expected: 工作树除验收文档外无未提交文件；所有命令 exit 0。

- [ ] **Step 9: 提交验收文档**

```powershell
git add docs/reviews/2026-08-10-dashboard-page-rbac-acceptance.zh-CN.md
git commit -m "docs: record dashboard page rbac acceptance"
```

## 计划自审清单

- 设计要求均已映射到 Task 1-10：0039 subscription 一致性、SaaS 门槛、53 页、4 管理页、多角色、直接权限、范围并集、API fail-closed、tenant 隔离、signed/unsigned 复合外键、关系物理替换、用户/角色聚合 expectedVersion、角色完整 CRUD、事务审计、UI、49/53、390px、无 SaaS 链接、Docker 四卷。
- 每个生产变更前均有具体 RED 命令和预期失败原因。
- `DashboardTenantAccess`、`DashboardAccessProfile`、`DataScope`、`DashboardAccessService.Resolve`、`DashboardRequestGuard.Authorize`、前端 `AccessProfile` 命名在上下游保持一致。
- 计划不包含占位实现，不把 route/test passed 写成最终完成。
