# SaaS 与 Dashboard 身份域隔离及单企业绑定实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 SaaS Admin 与 Dashboard 改造成完全独立的身份域，由 SaaS 初始化和治理 Dashboard 超级管理员，并把 Dashboard 收敛为一个租户只使用一家权威企业且支持企业微信员工同步的长期代码结构。

**Architecture:** 新增 `authrealm`、`saasauth`、`dashboardauth`、`dashboardprincipal`、`companyprofile` 五个边界清晰的服务端包；SaaS 和 Dashboard 使用独立身份表、JWT、Session 与登录页。数据库以唯一 tenant-corp binding 作为企业事实，所有 Dashboard 请求从认证上下文取得 tenant/corp，企业设置只操作唯一 binding，旧企业选择、新增、共享认证和明文凭据代码在切换任务中物理删除。

**Tech Stack:** Go 1.24、MariaDB/MySQL migration runner、React 19、TypeScript、TanStack Query、Vitest、Playwright、Node.js completion gate、PowerShell Docker smoke。

## Global Constraints

- 所有实现只在隔离工作树和功能分支进行；开始前执行 `git rev-parse --show-toplevel`、`git status --short`，不得覆盖主工作区未提交改动。
- 每个行为变更严格执行 RED → GREEN → REFACTOR；先看到测试因缺少目标行为失败，再写最少但完整的生产实现。
- 不引入双读、双写、旧接口 fallback、共享表 `realm` 补丁、隐藏开关、默认企业猜测或客户端 corp 选择。
- 所有 `tenant_id`、`corp_id`、actor 均来自认证上下文；请求出现这些越权字段时严格解码返回 400。
- Secret、密码、Token、激活令牌和私钥不得进入命令行、日志、测试快照、审计正文或 evidence。
- MariaDB/MySQL DDL 不宣称事务原子性。第一条 DDL 前完成一致性预检，分阶段迁移，ledger 只在单个 migration 全部成功后记账。
- 开发阶段 MySQL integration 仅允许从 admin DSN 创建名称以 `mochat_identity_single_corp_` 开头的临时 schema，测试结束必须删除；无 DSN 明确 SKIP。
- Task 13 之前不得操作 Docker、Compose、运行服务、业务数据库或卷。最终 Docker 验收只重建 `app`，不执行 `down -v`、`volume rm`、`system prune`，不重建 MySQL/Redis。
- 每个任务独立提交，不改写历史；完成前运行 `git diff --check`。

---

## Task 1: 建立完成合同与源码级架构门禁

**Files:**

- Create: `scripts/check_identity_realm_single_corp.mjs`
- Create: `scripts/check_identity_realm_single_corp.test.mjs`
- Create: `scripts/fixtures/identity-single-corp/valid/contract.json`
- Create: `scripts/fixtures/identity-single-corp/shared-auth/store.go`
- Create: `scripts/fixtures/identity-single-corp/shared-jwt/config.go`
- Create: `scripts/fixtures/identity-single-corp/corp-selector/main.tsx`
- Create: `scripts/fixtures/identity-single-corp/legacy-corp-route/server.go`
- Create: `scripts/fixtures/identity-single-corp/plaintext-secret/store.go`
- Modify: `package.json`

**Contract:**

```ts
export type IdentitySingleCorpEvidence = {
  saasIdentityTables: SourceLocation[];
  dashboardIdentityTables: SourceLocation[];
  jwtRealms: Record<'saas_admin' | 'dashboard', SourceLocation[]>;
  dashboardPrincipalConsumers: RouteEvidence[];
  forbiddenCorpRoutes: SourceLocation[];
  forbiddenSessionCorpFields: SourceLocation[];
  plaintextSecretReads: SourceLocation[];
};
```

- [ ] 写 RED：在临时 fixture tree 中分别构造 SaaS Store 读取 `mc_user`、共用 JWT 配置、Dashboard Session 含 `corpId`、注册 corp select/bind/store、企业设置出现“新建企业”、读取明文 Secret；逐例断言 `runIdentitySingleCorpGate(root)` 失败且包含来源文件和行号。
- [ ] 写 RED：构造 Go 注释或 JSON 中出现目标字符串，断言不能被当作生产路由或 Store 证据。
- [ ] 写 RED：构造未消费 `DashboardPrincipal` 的 Dashboard handler，断言门禁失败；不能用同目录其他 handler 的证据替代。
- [ ] Run: `node --test scripts/check_identity_realm_single_corp.test.mjs`
- [ ] Expected: FAIL，提示 `runIdentitySingleCorpGate`、结构扫描器和 package script 尚不存在。
- [ ] 实现基于 AST/受限生产源码解析的门禁：Go 只扫描 `.go` 且排除 `_test.go`，前端只从 SaaS/Dashboard 生产入口递归 import，migration、catalog、fixture、注释和测试不得作为生产事实。
- [ ] 在根 `package.json` 增加：

```json
"check:identity-single-corp": "node scripts/check_identity_realm_single_corp.mjs"
```

- [ ] Run: `node --test scripts/check_identity_realm_single_corp.test.mjs`
- [ ] Expected: PASS，至少 7 个独立坏 fixture RED 转为 GREEN；对当前生产树执行门禁应因尚未完成架构而明确失败。
- [ ] Commit: `test(identity): add realm and single-corp completion contract`

## Task 2: 新建身份域与单企业数据库结构

**Files:**

- Create: `deploy/standalone/migrations/0129_identity_realms_single_corp_schema.up.sql`
- Create: `deploy/standalone/migrations/0129_identity_realms_single_corp_schema.down.sql`
- Create: `internal/migration/identity_realms_single_corp_integration_test.go`
- Modify: `internal/migration/migration_test.go`
- Modify: `deploy/standalone/migrations/README.md`
- Modify: `.gitattributes`（只在校验发现 migration EOL 需要固定时修改）

**Schema:**

- `mochat_go_saas_admin_users`
- `mochat_go_dashboard_identities`
- `mochat_go_dashboard_identity_activations`
- `mochat_go_tenant_corp_bindings`
- SaaS access/audit actor 到平台用户的复合或单列外键
- `mc_corp(tenant_id,id)` 唯一索引和所有受影响 tenant 关联列的统一 unsigned 类型

- [ ] 写 MariaDB RED：从 `MOCHAT_GO_MYSQL_INTEGRATION_DSN` 连接 admin schema，创建 `mochat_identity_single_corp_<pid>_<time>` 临时库，导入真实基础 schema/既有 migrations，再 apply 0129。
- [ ] 测试必须覆盖：四张表结构、Dashboard login identifier 全局唯一、tenant 一条 binding、corp 只能绑定一次、`(tenant_id,corp_id)` 跨租户复合 FK 失败、SaaS actor FK 不可指向 `mc_user`。
- [ ] 写部分失败恢复 RED：仅完成 tenant 类型变更或仅创建部分新表时，down 不得因不存在的列/索引再次失败。
- [ ] 写 apply→down→apply RED；down 要恢复 0129 自己修改的列类型、索引和 FK，不能删除 0128 以前的对象。
- [ ] 所有类型预检、依赖 FK/索引枚举和跨租户预检放在第一条 DDL 前；`SIGNAL` 沿用仓库动态 `PREPARE/EXECUTE` 兼容形式，不使用 `DELIMITER` 或存储过程。
- [ ] 实现 0129 up/down，并为每个 DDL 阶段提供可定位错误；`information_schema` 检查必须覆盖 0127 RBAC 表的 `tenant_id` 依赖。
- [ ] Run: `go test ./internal/migration -run IdentityRealmsSingleCorp -count=1 -v`
- [ ] Expected: 无 DSN 时输出明确 SKIP；有 admin DSN 时所有场景 PASS 且临时 schema leftovers=0。
- [ ] Run: `go test ./internal/migration -count=1`
- [ ] Commit: `feat(identity): add isolated identity and tenant corp schema`

## Task 3: 实现独立 JWT Realm 与认证 Principal 基础包

**Files:**

- Create: `internal/authrealm/realm.go`
- Create: `internal/authrealm/token.go`
- Create: `internal/authrealm/token_test.go`
- Create: `internal/dashboardprincipal/context.go`
- Create: `internal/dashboardprincipal/context_test.go`
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `deploy/standalone/.env.example`
- Modify: `deploy/standalone/docker-compose.yml`

**Interfaces:**

```go
type Realm string

const (
    RealmSaaSAdmin Realm = "saas_admin"
    RealmDashboard Realm = "dashboard"
)

type TokenConfig struct {
    Secret   []byte
    Issuer   string
    Audience string
    TTL      time.Duration
    Realm    Realm
}

type DashboardPrincipal struct {
    UserID       int
    TenantID     int
    CorpID       int
    CorpStatus   CorpBindingStatus
    IsSuperAdmin bool
    AuthVersion  uint64
}
```

- [ ] 写 RED：SaaS Token 被 Dashboard parser 拒绝，Dashboard Token 被 SaaS parser 拒绝；错误 secret、issuer、audience、realm、过期、未来 `nbf`、旧 `auth_version` 全部返回稳定 401。
- [ ] 写 RED：缺任一 Realm Secret、两个 Secret 相同、issuer/audience 为空时 `config.Load()` 失败。
- [ ] 写 RED：`DashboardPrincipalFromContext` 在缺失、类型错误、零 tenant/corp 时 fail closed。
- [ ] Run: `go test ./internal/authrealm ./internal/dashboardprincipal ./internal/config -count=1`
- [ ] Expected: FAIL，缺新包与配置。
- [ ] 实现 realm 专用 signer/parser，不改造 `simple_jwt` 成共享可选 realm；后者只供范围外 worker 使用。
- [ ] Compose 只通过 Secret 环境引用注入两个不同值，示例文件不放真实 secret。
- [ ] Run: `go test ./internal/authrealm ./internal/dashboardprincipal ./internal/config -count=1`
- [ ] Expected: PASS。
- [ ] Commit: `feat(auth): establish isolated token realms and principals`

## Task 4: 实现身份 Store 与只初始化 SaaS 平台管理员

**Files:**

- Create: `internal/saasauth/service.go`
- Create: `internal/saasauth/service_test.go`
- Create: `internal/dashboardauth/service.go`
- Create: `internal/dashboardauth/service_test.go`
- Create: `internal/store/saas_identity.go`
- Create: `internal/store/saas_identity_test.go`
- Create: `internal/store/dashboard_identity.go`
- Create: `internal/store/dashboard_identity_test.go`
- Modify: `cmd/mochat-bootstrap/main.go`
- Modify: `cmd/mochat-bootstrap/main_test.go`
- Modify: `deploy/standalone/docker-compose.yml`
- Modify: `deploy/standalone/README.md`

**Interfaces:**

```go
type SaaSIdentityStore interface {
    Authenticate(ctx context.Context, login string) (SaaSIdentity, error)
    Bootstrap(ctx context.Context, input BootstrapSaaSAdmin) (SaaSIdentity, error)
    CheckSession(ctx context.Context, userID int, authVersion uint64) error
}

type DashboardIdentityStore interface {
    Authenticate(ctx context.Context, loginIdentifier string) (DashboardIdentity, error)
    Activate(ctx context.Context, tokenDigest [32]byte, passwordHash string) error
    CheckSession(ctx context.Context, userID int, authVersion uint64) error
}
```

- [ ] 写 SQL mock 和真实 MySQL RED：SaaS Store 的所有查询只能命中 `mochat_go_saas_admin_users`；Dashboard Store 的密码查询只能命中 `mochat_go_dashboard_identities`。
- [ ] 写 bootstrap RED：密码仅从 `MOCHAT_BOOTSTRAP_SAAS_ADMIN_PASSWORD_FILE` 读取，文件权限/空值错误失败；重复 request key 幂等且不重置密码；执行后 Dashboard identity、tenant、corp、`mc_user` 增量均为 0。
- [ ] 写 RED：初始化日志、返回值和 error 不包含密码文件内容。
- [ ] Run: `go test ./internal/saasauth ./internal/dashboardauth ./internal/store ./cmd/mochat-bootstrap -run 'Identity|BootstrapSaaS' -count=1`
- [ ] 实现两个 Store、密码哈希和 session version 校验；删除 bootstrap 的 `-secret` 明文 flag 以及用 `MOCHAT_SIMPLE_JWT_SECRET` 充当密码材料的逻辑。
- [ ] Run 同一命令，Expected: PASS。
- [ ] Commit: `feat(identity): separate stores and bootstrap SaaS admin only`

## Task 5: 提供 SaaS 专用登录、MFA 与前端 Session

**Files:**

- Create: `internal/saasauth/http.go`
- Create: `internal/saasauth/http_test.go`
- Modify: `internal/server/server.go`
- Modify: `internal/server/server_test.go`
- Modify: `cmd/mochat-go/main.go`
- Create: `web/apps/saas-admin/src/lib/auth-session.ts`
- Create: `web/apps/saas-admin/src/lib/auth-session.test.ts`
- Create: `web/apps/saas-admin/src/pages/LoginPage.tsx`
- Create: `web/apps/saas-admin/src/pages/LoginPage.test.tsx`
- Modify: `web/apps/saas-admin/src/lib/api.ts`
- Modify: `web/apps/saas-admin/src/lib/api.test.ts`
- Modify: `web/apps/saas-admin/src/App.tsx`
- Modify: `web/apps/saas-admin/src/main.tsx`
- Modify: `web/apps/saas-admin/package.json`

**Routes:**

- `POST /saas/auth/login`
- `POST /saas/auth/mfa`
- `POST /saas/auth/logout`
- `GET /saas/auth/session`
- `GET /saas/login`

- [ ] 写 HTTP RED：SaaS 登录只查 SaaSIdentityStore，成功 Token claim 为 `saas_admin`；Dashboard 用户同名也不能登录。
- [ ] 写跨域 RED：Dashboard Token 请求现有 `/dashboard/saasAdmin/*` 全部 401，SaaS Token 访问任一 Dashboard 业务 API 全部 401。
- [ ] 写前端 RED：无 session 跳转 `/saas/login`，不再跳 `/security/login`；Storage 只读写 `mochat_saas_admin_*`；登出不删除 Dashboard 键。
- [ ] 首次初始化用户登录后必须进入改密和 MFA 流程，完成前不能访问 SaaS 管理 API。
- [ ] Run: `go test ./internal/saasauth ./internal/server -run SaaSAuth -count=1`
- [ ] Run: `corepack pnpm --filter @mochat/saas-admin test && corepack pnpm --filter @mochat/saas-admin typecheck`
- [ ] 实现专用路由、guard、页面和 Session；`web/apps/saas-admin/src/lib/api.ts` 不再接受旧 `ACCESS_TOKEN` 或 `/security/login` fallback。
- [ ] 在 SaaS package 增加 `lint` script，覆盖全部生产与测试 TypeScript。
- [ ] 重跑以上命令，Expected: PASS。
- [ ] Commit: `feat(saas-auth): add dedicated login and session boundary`

## Task 6: 提供 Dashboard Identity 登录、激活、MFA 与密码治理

**Files:**

- Create: `internal/dashboardauth/http.go`
- Create: `internal/dashboardauth/http_test.go`
- Modify: `internal/dashboard/auth.go`
- Modify: `internal/dashboard/auth_test.go`
- Modify: `internal/dashboard/user_admin.go`
- Modify: `internal/dashboard/user_admin_test.go`
- Modify: `internal/server/server.go`
- Modify: `cmd/mochat-go/main.go`
- Create: `web/apps/dashboard/src/features/auth/activation-page.tsx`
- Create: `web/apps/dashboard/src/features/auth/activation-page.test.tsx`
- Modify: `web/packages/auth/src/session.ts`
- Modify: `web/packages/auth/src/session.test.ts`
- Modify: `web/packages/auth/src/auth-store.ts`
- Create: `web/packages/auth/src/auth-store.test.ts`
- Modify: `web/apps/dashboard/src/main.tsx`

**Routes:**

- 保留 Dashboard 稳定登录合同 `POST /dashboard/user/auth`、`POST /dashboard/user/authMFA`，但实现只依赖 DashboardIdentityStore。
- 新增 `POST /dashboard/auth/activate`、`POST /dashboard/auth/password/reset-request`、`POST /dashboard/auth/password/reset`。

- [ ] 写 RED：`mc_user` 有密码但没有 Dashboard identity 时不能登录；Identity 正常但 user/tenant/status 不可访问时在签 Token 前失败。
- [ ] 写 RED：激活原始 token 只可消费一次、过期失败、摘要常量时间比较、成功后设置密码并递增 `auth_version`。
- [ ] 写 RED：Dashboard Session 不再包含 `corpId` 或独立 corp storage key；登出不删除 SaaS 键。
- [ ] 写 RED：登录请求只有手机号/密码/MFA，不接受 tenant/corp；手机号全局重复由数据约束阻止而非登录后选择。
- [ ] Run: `go test ./internal/dashboardauth ./internal/dashboard ./internal/server -run 'DashboardAuth|Activate|Password' -count=1`
- [ ] Run: `corepack pnpm --filter @mochat/dashboard test -- --run src/features/auth && corepack pnpm --filter @mochat/auth test && corepack pnpm --filter @mochat/auth typecheck`
- [ ] 实现并删除 `internal/store/mysql.go::UserAuthByPhone` 的密码校验职责；业务 user 查询与 identity 查询保持两个接口。
- [ ] 重跑后 Expected: PASS。
- [ ] Commit: `feat(dashboard-auth): move login and activation to tenant identity`

## Task 7: 将 SaaS 开户与 Dashboard 超级管理员治理改为原子业务流程

**Files:**

- Create: `internal/dashboardadmin/service.go`
- Create: `internal/dashboardadmin/service_test.go`
- Create: `internal/store/dashboard_admin_provisioning.go`
- Create: `internal/store/dashboard_admin_provisioning_integration_test.go`
- Modify: `internal/dashboard/saas_admin.go`
- Modify: `internal/dashboard/saas_admin_test.go`
- Modify: `internal/dashboard/saas_admin_access.go`
- Modify: `internal/store/mysql.go`
- Modify: `web/apps/saas-admin/src/pages/TenantsPage.tsx`
- Create: `web/apps/saas-admin/src/pages/TenantsPage.test.tsx`
- Modify: `web/apps/saas-admin/src/lib/types.ts`

**Use cases:**

```go
type ProvisionDashboardTenant struct {
    TenantName           string
    PackageID            int
    Limits               SaaSAdminPackageLimits
    Subscription         SubscriptionInput
    AdminLoginIdentifier string
    AdminName            string
    IdempotencyKey       string
    ExpectedVersion      uint64
}
```

- [ ] 写真实事务 RED：tenant、package、26 quota、subscription、corp placeholder、binding、`mc_user` superadmin、identity、activation digest、两类 audit 任一步失败全部回滚。
- [ ] 写 RED：同手机号第二租户开户返回 409 零写；同 idempotency key 不重复写且不重放原始激活 token。
- [ ] 写 RED：替换超管必须先激活新主体；停用最后一个有效超管返回 409；Dashboard 用户/角色接口不能写 `isSuperAdmin`。
- [ ] 写 TOCTOU RED：写事务首条业务查询在 tx 内锁定 SaaS actor 并验证 active/permission，错误 actor 在 target 查询前零写。
- [ ] 实现 `dashboardadmin.Service`，HTTP handler 只做严格 decode、认证和 envelope；事务只在 Store adapter 内。
- [ ] SaaS 页面提供开户、重发激活、替换、停用和恢复；所有 mutation 先展示摘要并二次确认，409 保留表单。
- [ ] Run: `go test ./internal/dashboardadmin ./internal/store ./internal/dashboard -run 'Provision|DashboardAdmin' -count=1`
- [ ] Run: `corepack pnpm --filter @mochat/saas-admin test && corepack pnpm --filter @mochat/saas-admin typecheck`
- [ ] Commit: `feat(saas): provision and govern dashboard administrators`

## Task 8: 回填历史身份、唯一企业与凭据密文

**Files:**

- Create: `deploy/standalone/migrations/0130_identity_realms_single_corp_backfill.up.sql`
- Create: `deploy/standalone/migrations/0130_identity_realms_single_corp_backfill.down.sql`
- Create: `internal/migration/identity_realms_single_corp_backfill_integration_test.go`
- Create: `cmd/mochat-identity-preflight/main.go`
- Create: `cmd/mochat-identity-preflight/main_test.go`
- Create: `cmd/mochat-identity-migrate/main.go`
- Create: `cmd/mochat-identity-migrate/main_test.go`
- Modify: `internal/wecomcredentials/manager.go`
- Modify: `internal/wecomcredentials/manager_test.go`

- [ ] 写 RED：重复 Dashboard 手机号、多个有效 corp 无显式 mapping、dangling tenant/corp/user、平台 actor 无法映射、密文不可解密、负 tenant ID 均在第一条 DDL 前失败。
- [ ] 写 RED：零 corp 租户创建占位 corp；一 corp 直接 binding；多 corp 只接受签名/校验过的 `tenant→corp` 映射文件，不按最小 ID 猜测。
- [ ] 写 RED：平台管理员复制到 SaaS identity 后历史 actor ID 可解释；Dashboard 密码 hash 移入 identity 后登录结果不变。
- [ ] 写 RED：`mochat-identity-migrate encrypt-credentials` 在事务内为企业和 agent 生成可解密密文，但本阶段保留原明文供 0130 down 验证；任一加密失败整批不提交，错误不包含 Secret。
- [ ] preflight 输出只包含计数、对象 ID 和修复分类，禁止输出 phone、password、Secret、token。
- [ ] 实现 0130、只读 preflight CLI 与专用 migrate CLI；mapping 文件从只读挂载读取并校验 tenant/corp 复合归属。preflight 绝不写库，migrate 必须显式 `--execute` 和 request ID 才可写临时或维护窗口目标。
- [ ] Run: `go test ./internal/migration ./internal/wecomcredentials ./cmd/mochat-identity-preflight ./cmd/mochat-identity-migrate -run 'Identity|SingleCorp|Credential' -count=1 -v`
- [ ] Expected: 无 DSN 场景明确 SKIP；真实临时 MariaDB 全部 PASS。
- [ ] Commit: `feat(migration): backfill identities bindings and encrypted credentials`

## Task 9: 建立唯一企业 Principal 并迁移所有 Dashboard handler

**Files:**

- Create: `internal/dashboardprincipal/resolver.go`
- Create: `internal/dashboardprincipal/resolver_test.go`
- Create: `internal/store/tenant_corp_binding.go`
- Create: `internal/store/tenant_corp_binding_test.go`
- Modify: `internal/server/server.go`
- Modify: `internal/server/server_test.go`
- Modify: `cmd/mochat-go/main.go`
- Modify: `cmd/mochat-go/scrm.go`
- Modify: `cmd/mochat-go/ai_debt_clearance.go`
- Modify: all production files returned by `rg -l 'selectedCorpID\(' internal/dashboard`
- Delete: `internal/dashboard/session_bridge.go`
- Modify: `scripts/check_identity_realm_single_corp.mjs`
- Modify: `scripts/check_identity_realm_single_corp.test.mjs`

**Request order:**

```text
realm authentication
  -> identity/user/auth_version validation
  -> tenant SaaS gate
  -> unique tenant-corp binding resolver
  -> DashboardPrincipal context
  -> page/API RBAC guard
  -> handler/service/store
```

- [ ] 写 middleware RED：binding 缺失/重复/暂停、corp tenant 不一致全部 fail closed；客户端 body/query/header 的 corp 不能改变 principal。
- [ ] 写 handler RED：选取 SCRM、报表、AI insight、员工、群聊、营销五类代表路由，认证上下文 corp 固定；伪造 corpId 返回 400 或被严格合同拒绝。
- [ ] 写静态 RED：每个生产 Dashboard handler 必须通过 `DashboardPrincipalFromContext` 或由已证明的统一 wrapper 注入，`selectedCorpID`、`LoginCorpInfo.CorpId`、`CorpIdMap` 任一残留即失败并报告函数位置。
- [ ] 实现 binding resolver，并在 server composition root 中只注册一次；下游不再查询“用户可选企业”。
- [ ] 按扫描清单逐文件迁移 `selectedCorpID` 调用；不能保留同名函数包装 principal，因为那会掩盖旧调用合同。
- [ ] 将 reporting、SCRM、AI 的 principal adapter 同步改为从 `DashboardPrincipal` 取 tenant/corp，保留 Phase 4 已实现的 `AllowedEmployeeIDs` scope，不回退读取旧首角色权限。
- [ ] Run: `go test ./internal/dashboardprincipal ./internal/server ./internal/dashboard ./internal/modules/... ./cmd/mochat-go -count=1`
- [ ] Run: `corepack pnpm check:identity-single-corp`
- [ ] Expected: `selectedCorpID`、旧 LoginCorpInfo corp 读取、未证明 principal consumer 均为 0。
- [ ] Commit: `refactor(dashboard): resolve tenant and corp from one principal`

## Task 10: 实现唯一企业资料、凭据与员工同步后端

**Files:**

- Create: `internal/companyprofile/model.go`
- Create: `internal/companyprofile/service.go`
- Create: `internal/companyprofile/service_test.go`
- Create: `internal/companyprofile/http.go`
- Create: `internal/companyprofile/http_test.go`
- Create: `internal/store/company_profile.go`
- Create: `internal/store/company_profile_integration_test.go`
- Modify: `internal/dashboard/corp_admin_wecom.go`
- Modify: `internal/dashboard/work_employee_sync_runner.go`
- Modify: `internal/dashboard/employee_apply_worker.go`
- Modify: `internal/dashboard/queue_registry.go`
- Modify: `internal/server/server.go`
- Modify: `cmd/mochat-go/main.go`

**API contracts:**

- `GET /dashboard/company/profile`
- `PUT /dashboard/company/profile`
- `PUT /dashboard/company/wecom-credentials`
- `PUT /dashboard/company/agent-credentials`
- `PUT /dashboard/company/archive-credentials`
- `POST /dashboard/company/verify`
- `POST /dashboard/company/employee-sync`
- `GET /dashboard/company/sync-status`
- `GET /dashboard/company/audits`

- [ ] 写 RED：所有接口只有同 tenant active superadmin 可用，target 固定 principal corp；body 含 `tenantId/corpId/actorUserId` 返回 400。
- [ ] 写 RED：`expectedVersion` 错误 409 且不覆盖；actor 状态在 tx 内锁行复核；密文更新、version、audit 同一事务，audit 失败整笔回滚。
- [ ] 写 RED：profile/审计/日志/错误只返回 `configured/keyId/updatedAt`，搜索响应序列化后不得出现 Secret、密文或 hash。
- [ ] 写 RED：首次 verify 固化唯一 CorpID 和企业微信权威名称；已验证后 Dashboard 更换 CorpID 返回 409；SaaS rebind 走独立高风险 use case。
- [ ] 写员工同步 RED：验证成功后可触发唯一企业首次全量同步；worker job payload 只有 binding ID，不信任传入 tenant/corp；部门和员工只写同 tenant/corp；同步员工不创建 Dashboard identity、不修改 `isSuperAdmin`。
- [ ] 写状态 RED：未验证/暂停/租户门槛失败时同步 fail closed；状态返回游标、计数、时间和脱敏错误。
- [ ] 实现 package 分层：HTTP 不写 SQL，Service 管规则，Store 管事务，worker 通过同一 binding resolver。
- [ ] Run: `go test ./internal/companyprofile ./internal/store ./internal/dashboard ./internal/server -run 'Company|Credential|EmployeeSync' -count=1`
- [ ] Commit: `feat(company): manage one verified corp and employee sync`

## Task 11: 重建 Dashboard 企业设置页面并移除企业选择状态

**Files:**

- Create: `web/apps/dashboard/src/features/company-settings/company-profile-api.ts`
- Create: `web/apps/dashboard/src/features/company-settings/company-profile-api.test.ts`
- Rewrite: `web/apps/dashboard/src/features/company-settings/website-page.tsx`
- Modify: `web/apps/dashboard/src/features/company-settings/company-settings-pages.test.tsx`
- Modify: `web/apps/dashboard/src/main.tsx`
- Modify: `web/apps/dashboard/src/features/access/access-api.ts`
- Modify: `web/apps/dashboard/src/features/access/access-api.test.ts`
- Delete: `web/apps/dashboard/src/features/corp/corp-provider.tsx`
- Delete: `web/apps/dashboard/src/features/corp/corp-provider.test.tsx`
- Delete: `web/apps/dashboard/src/features/corp/corp-api.ts`
- Delete: `web/apps/dashboard/src/features/corp/corp-api.test.ts`
- Modify: `web/apps/dashboard/src/styles/index.css`

**UI states:** `loading`、`error + retry`、`待配置`、`验证中`、`已验证`、`暂停`、`同步中`、`同步失败`。

- [ ] 写 RED：企业页只有唯一详情，无新建、列表、搜索、分页、删除、切换企业；普通用户 403，superadmin 可查看。
- [ ] 写 RED：Secret 输入为空代表不修改，已配置只显示掩码状态；DOM、query cache 和错误文本不包含服务端 Secret。
- [ ] 写 RED：编辑资料、轮换凭据、验证、立即同步都先展示变更摘要并经过 `ConfirmAction`；首次点击不 mutation，确认后 payload 不含 tenant/corp/actor。
- [ ] 写 RED：409 保留表单并提示刷新；`CORP_CONFIGURATION_REQUIRED` 保留 session 并导航到企业设置；`TENANT_ACCESS_DENIED` 清 session。
- [ ] 写 RED：Session 类型和 localStorage 无 corpId，main 不挂 `CorpProvider`，企业验证/同步后刷新 company profile 和 access profile。
- [ ] 写 390px RED：表单单列、Secret 控件和确认弹层可达、无 document 横向溢出；Desktop 布局分组清晰。
- [ ] 实现唯一详情页；企业微信板块明确显示“员工可从企业微信同步”，提供首次/立即同步及状态。
- [ ] Run: `corepack pnpm --filter @mochat/dashboard test -- --run company-settings corp access`
- [ ] Run: `corepack pnpm --filter @mochat/dashboard typecheck && corepack pnpm --filter @mochat/dashboard lint && corepack pnpm --filter @mochat/dashboard build`
- [ ] Expected: PASS；`rg -n 'CorpProvider|bindCorp|persistCorpId|mochat_dashboard_corp_id|新建企业' web/apps/dashboard/src web/packages/auth/src` 在生产源码为 0。API 响应可只读展示内部 corp ID，但 Session 与请求不得保存或提交它。
- [ ] Commit: `feat(dashboard): replace corp selector with single company settings`

## Task 12: 完成一次性切换并物理删除旧认证与企业接口

**Files:**

- Create: `deploy/standalone/migrations/0131_identity_realms_single_corp_cutover.up.sql`
- Create: `deploy/standalone/migrations/0131_identity_realms_single_corp_cutover.down.sql`
- Create: `internal/migration/identity_realms_single_corp_cutover_integration_test.go`
- Modify: `internal/server/server.go`
- Modify: `internal/server/server_test.go`
- Modify: `cmd/mochat-go/main.go`
- Delete: `internal/dashboard/identity_login_page.go`
- Delete: `internal/dashboard/identity_login_page_test.go`
- Delete: obsolete corp list/create/bind handlers in `internal/dashboard/corp_admin.go` after moving retained helpers to `internal/companyprofile`
- Modify: `internal/store/mysql.go`
- Modify: `scripts/check_identity_realm_single_corp.mjs`

- [ ] 写 RED：0131 前置检查要求 0129/0130 完整、每个 active user 有 identity、每个 active tenant 有唯一 binding、所有需迁移 Secret 已是密文。
- [ ] 写 RED：cutover 删除 `mc_user.password`，把已验证有密文副本的旧 Secret 列置空，并删除废弃明文读取能力；down 在系统尚未开放新写入时从 identity hash 恢复密码列，再由 `mochat-identity-migrate restore-legacy-credentials` 使用密文恢复旧 Secret，完整通过无数据损失的 apply→down→apply。
- [ ] 写路由 RED：`/dashboard/corp/select`、`/dashboard/corp/bind`、`POST /dashboard/corp/store`、旧 `/security/login` 均不在生产 route registry；普通和 superadmin 访问都不能 fallback 到旧 handler。
- [ ] 写完成门禁 RED：SaaS access/store 引用 `mc_user`、Dashboard auth 引用 `mc_user.password`、共享 JWT、corp Session、旧 route、Secret 明文读取任一出现即失败。
- [ ] 从 server composition root 删除旧注册，从 Store interface 删除旧方法，从测试 fake 删除旧合同；不保留 deprecated wrapper。
- [ ] Run: `go test ./internal/migration ./internal/server ./internal/store ./internal/dashboard -run 'Cutover|LegacyAuth|LegacyCorp' -count=1`
- [ ] Run: `corepack pnpm check:identity-single-corp`
- [ ] Expected: PASS，门禁输出两个 identity realm、一个 binding resolver、0 legacy corp route、0 corp Session、0 plaintext secret read。
- [ ] Commit: `refactor(identity): cut over and remove shared auth and corp flows`

## Task 13: 全量自动化、真实验收合同与安全部署脚本

**Files:**

- Create: `web/e2e/tests/identity-single-corp.spec.ts`
- Create: `scripts/validate_identity_single_corp_e2e_fixture.mjs`
- Create: `scripts/validate_identity_single_corp_e2e_fixture.test.mjs`
- Create: `scripts/smoke_identity_single_corp.ps1`
- Create: `scripts/smoke_identity_single_corp.test.mjs`
- Create: `docs/runbooks/2026-08-11-identity-single-corp-cutover.zh-CN.md`
- Modify: `web/e2e/package.json`
- Modify: `package.json`

**Live fixture schema:**

```ts
type IdentitySingleCorpFixture = {
  saasAdmin: LoginCredentialRef;
  dashboardSuperAdmin: LoginCredentialRef;
  dashboardOrdinary: LoginCredentialRef;
  tenantDenied: LoginCredentialRef;
  secondTenantAdmin: LoginCredentialRef;
  expectedTenantId: number;
  expectedCorpId: number;
  expectedWxCorpId: string;
};
```

Fixture JSON 只引用环境变量中的凭据键，不包含密码、JWT 或 Secret 原文。

- [ ] 写 validator RED：缺账号、重复 tenant/corp、明文 password/token/secret、相同 SaaS/Dashboard JWT、非唯一 binding 均失败。
- [ ] 写 Playwright live 用例：不 route mock，真实登录 SaaS 和 Dashboard；验证跨域 Token 双向 401、SaaS 开户/激活、Dashboard 无企业选择、普通用户无企业设置权限、超管只见一家企业。
- [ ] 验证未配置企业只能进入企业设置；企业微信验证后员工同步入口可用，读取同步状态；不得用 E2E 创建第二家企业。
- [ ] Desktop 与 390px 都验证企业资料、凭据轮换确认、Secret 不回显、链接不跳 SaaS、console error=0、除明确预期外 4xx/5xx=0。
- [ ] smoke 提供 `-ReadOnly` 和 `-ExerciseFixtureWrites` 两个互斥模式；ReadOnly 绝不 mutation 且关键表精确 `COUNT(*)` 前后不变。
- [ ] full smoke 通过受保护 API 执行 version 冲突、跨租户 404、凭据元数据更新和审计验证；Secret 从环境/Secret 文件读取，脚本和 evidence 不打印。
- [ ] smoke 强制 Compose project `mochat-go-desktop`、默认 `BaseUrl=http://localhost:18080`，通过 `-f deploy/standalone/docker-compose.yml` 定位；记录并比较 MySQL/Redis container IDs 和精确四卷 name/mountpoint，app ID 允许 app-only rebuild 后变化。
- [ ] PowerShell parser test 禁止 `down -v`、`volume rm`、`system prune`、MySQL/Redis recreate、命令行 password/token/secret。
- [ ] 增加 scripts：

```json
"test:identity-single-corp-contract": "node --test scripts/check_identity_realm_single_corp.test.mjs scripts/validate_identity_single_corp_e2e_fixture.test.mjs scripts/smoke_identity_single_corp.test.mjs",
"test:e2e:identity-single-corp": "corepack pnpm --filter @mochat/e2e test:e2e -- tests/identity-single-corp.spec.ts --workers=1"
```

- [ ] Run: `go test ./...`
- [ ] Run: `corepack pnpm check:phase4-dashboard-page-rbac && corepack pnpm check:identity-single-corp`
- [ ] Run: `corepack pnpm test:identity-single-corp-contract`
- [ ] Run: `corepack pnpm --filter @mochat/dashboard test && corepack pnpm --filter @mochat/dashboard typecheck && corepack pnpm --filter @mochat/dashboard lint && corepack pnpm --filter @mochat/dashboard build`
- [ ] Run: `corepack pnpm --filter @mochat/saas-admin test && corepack pnpm --filter @mochat/saas-admin typecheck && corepack pnpm --filter @mochat/saas-admin lint && corepack pnpm --filter @mochat/saas-admin build`
- [ ] Run: `corepack pnpm --filter @mochat/e2e typecheck && corepack pnpm --filter @mochat/e2e lint && corepack pnpm test:e2e:identity-single-corp`
- [ ] Expected: 无 live env 时仅 live describe 明确 SKIP，mock/UI 合同测试 PASS；主任务提供隔离 DSN 和真实 fixture 后 live 全矩阵 PASS。
- [ ] Commit: `test(identity): add single-corp release and acceptance gates`

## Task 14: 维护窗口部署与最终产品验收

> 本任务只在用户明确授权的 `mochat-go-desktop` 最终阶段执行；开发会话到 Task 13 必须停止并报告重叠文件。

**Evidence directory:** `D:\workspace\mochat-go\output\identity-single-corp-20260811\`

- [ ] 记录 `git HEAD`、迁移 ledger、app/mysql/redis container IDs、四卷 inspect、关键表精确计数和备份路径；evidence 不含凭据。
- [ ] 使用隔离临时 MariaDB admin DSN 运行 0129→0130→0131 apply→down→apply、脏数据失败和 Store integration；确认临时 schema leftovers=0。
- [ ] Run: `go test ./internal/migration ./internal/store -run 'IdentityRealmsSingleCorp|DashboardAdminProvisioning|CompanyProfile' -count=1 -v`
- [ ] 对业务库执行只读 preflight；有重复手机号、多企业歧义、无法加密凭据时停止，不自动修复。
- [ ] 通过 SaaS 治理流程准备 tenant→corp 显式映射和 Dashboard 超级管理员激活，不直接 SQL 修改业务事实。
- [ ] 停止并只重建 `app`；MySQL、Redis 与卷保持运行。禁止 `down -v`、volume 删除、prune 或重建数据服务。
- [ ] 运行 migration、只启动新应用并执行 full smoke 与 live Playwright。
- [ ] Run: `powershell -NoProfile -File scripts/smoke_identity_single_corp.ps1 -ComposeProject mochat-go-desktop -ComposeFile deploy/standalone/docker-compose.yml -ExerciseFixtureWrites`
- [ ] Run: `corepack pnpm test:e2e:identity-single-corp`
- [ ] 验收：初始化仅 SaaS 管理员；SaaS/Dashboard 双向 Token 隔离；Dashboard 超管由 SaaS 授权；登录无企业选择；唯一企业资料和全部 Secret 管理正确；员工可从企业微信同步；普通用户、跨租户、版本冲突和审计符合合同。
- [ ] 验收 53 页 Dashboard、SaaS 关键页、Desktop/390px、console/network；确认无 SaaS 链接、无企业新增/切换入口。
- [ ] 比较前后容器/卷/关键数据：MySQL/Redis IDs 和四卷 name/mountpoint 不变；业务计数变化只允许与已审计开户、激活、配置和同步操作一致。
- [ ] 将命令、精确数量、截图索引、已知限制和回滚边界写入中文验收记录并提交。
- [ ] Final verification: `git status --short` 必须 clean，所有非 Docker 和 live gates 有当次新鲜输出。
- [ ] Commit: `docs: record identity realm and single corp acceptance`

## 实施完成判定

以下条件缺一不可：

1. SaaS 与 Dashboard 的身份表、Store、JWT Secret、claims、Session、登录页、MFA 和 actor 全部分离。
2. 初始化只产生 SaaS 平台管理员；Dashboard 超级管理员只能经 SaaS 原子授权和治理。
3. 每个 active tenant 有且只有一个有效 tenant-corp binding，所有 Dashboard handler 只消费 `DashboardPrincipal`。
4. Dashboard 无企业选择、新增、删除、绑定其他企业的生产路由和 UI；旧代码已物理删除。
5. 企业微信凭据和会话存档 Secret 全链路加密且不回显；唯一企业验证后可安全同步员工和部门。
6. `mc_user.password` 和所有运行时明文 Secret fallback 已删除。
7. migration、Go、前端、completion gate、E2E、smoke、Docker 数据保留与真实浏览器验收均有新鲜证据。
8. 不得以 route 可访问、静态字符串存在、mock profile、隐藏开关或某一小批页面通过代替产品完成。
