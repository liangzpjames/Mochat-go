# SaaS 与 Dashboard 身份域隔离及单企业切换运行手册

## 1. 适用范围与停止线

本手册对应 Task13 的自动化合同、真实浏览器验收和安全 smoke。Task13 只新增检查、测试和文档，不启动 Docker/Compose，不连接业务数据库，不重建容器，不修改数据卷，也不写入 `output`。

Task14 必须由用户明确授权后，才能在 Compose project `mochat-go-desktop` 上执行维护窗口操作。任何预检失败都必须停止并保留只读证据，不得猜测租户、企业或凭据，也不得用 SQL 直接修复业务事实。

## 2. Task13 预检

在隔离 worktree 中确认：

```powershell
Set-Location D:\workspace\mochat-go\mochat-go\.worktrees\phase4-dashboard-page-rbac
git rev-parse --show-toplevel
git branch --show-current
git status --short
```

分支必须是 `phase4/dashboard-page-rbac`，开始前工作树应为 clean。主工作区的未提交修改不得被覆盖、清理或 reset。

先执行 fixture validator 和静态合同：

```powershell
node scripts/validate_identity_single_corp_e2e_fixture.mjs .\path\to\identity-single-corp.fixture.json
corepack pnpm test:identity-single-corp-contract
corepack pnpm check:phase4-dashboard-page-rbac
corepack pnpm check:identity-single-corp
```

`acceptance inventory` 必须继续诚实保持 `106 = 36 active + 70 deferred`，Dashboard 页面覆盖必须保持 Task12 的 `53 = 48 ordinary + 5 superadmin_only`。这些数字不能通过缩减已有 tests/smoke 或修改旧路由来转绿。

## 3. Live fixture 与凭据边界

fixture JSON 只存环境变量键名，不存凭据值。每个账号必须具有如下形状：

```json
{
  "saasAdmin": { "loginEnvKey": "MOCHAT_IDENTITY_SAAS_LOGIN", "passwordEnvKey": "MOCHAT_IDENTITY_SAAS_PASSWORD" },
  "dashboardSuperAdmin": { "loginEnvKey": "MOCHAT_IDENTITY_DASHBOARD_SUPER_LOGIN", "passwordEnvKey": "MOCHAT_IDENTITY_DASHBOARD_SUPER_PASSWORD" },
  "dashboardOrdinary": { "loginEnvKey": "MOCHAT_IDENTITY_DASHBOARD_ORDINARY_LOGIN", "passwordEnvKey": "MOCHAT_IDENTITY_DASHBOARD_ORDINARY_PASSWORD" },
  "tenantDenied": { "loginEnvKey": "MOCHAT_IDENTITY_TENANT_DENIED_LOGIN", "passwordEnvKey": "MOCHAT_IDENTITY_TENANT_DENIED_PASSWORD" },
  "secondTenantAdmin": { "loginEnvKey": "MOCHAT_IDENTITY_SECOND_TENANT_LOGIN", "passwordEnvKey": "MOCHAT_IDENTITY_SECOND_TENANT_PASSWORD" },
  "expectedTenantId": 41,
  "expectedCorpId": 701,
  "expectedWxCorpId": "ww_example",
  "tenantCorpBindings": [{ "tenantId": 41, "corpId": 701, "status": "pending" }],
  "expectedSaasGovernance": {
    "dashboardProvisionOperationId": 9001,
    "dashboardProvisionRequestId": "identity-single-corp-dashboard-provision"
  }
}
```

五个账号必须齐全，凭据环境键必须互不重复；`tenantCorpBindings` 必须恰好一条且与 expected ID 一致；`expectedSaasGovernance` 必须只引用已存在的 SaaS 治理 operation ID/request ID 元数据。任何未知字段、重复 env key、重复 binding 或敏感字段都必须在 Playwright 启动前失败。`password`、`phone`、JWT、Secret、Token、Authorization 和 Cookie 原文均禁止出现在 JSON。SaaS/Dashboard Token 不写入 fixture；由真实登录产生，并在浏览器合同中验证两个 Token 不同且 `realm` 分别为 `saas_admin`、`dashboard`。

运行 live 验收前，设置 `MOCHAT_E2E_LIVE_BASE`、`MOCHAT_E2E_IDENTITY_SINGLE_CORP_FIXTURE_JSON` 以及 fixture 引用的环境变量。没有 live 环境时，`identity-single-corp.spec.ts` 只报告带 `SKIP` 原因的 live describe；它不以 route mock 伪造产品验收。

```powershell
corepack pnpm --filter @mochat/e2e typecheck
corepack pnpm --filter @mochat/e2e lint
corepack pnpm test:e2e:identity-single-corp
```

真实矩阵必须覆盖：SaaS/Dashboard 登录、双向跨 realm 401、SaaS 已授权且 Dashboard 超管已激活、无企业选择、普通用户企业设置 403、超管唯一企业、待配置企业仅能进入设置、企业微信验证、员工同步状态、Desktop/390px、凭据确认、Secret 不回显、Dashboard 无 SaaS 链接以及 console/network 预期外错误为 0。第一条用例必须以 SaaS 只读 `/dashboard/saasAdmin/operations` 的固定 operation ID/request ID 核对 `saas.admin.dashboard_tenant.provision`、tenant/corp 和激活发放记录，再以 Dashboard 治理只读 API 核对同一 `dashboardUserId` 的 `activatedAt`，不能用当前 `isSuperAdmin` 行自证。E2E 不得调用创建第二企业的 route。

## 4. PowerShell smoke

smoke 强制 project `mochat-go-desktop`，默认地址为 `http://localhost:18080`，默认 Compose 文件在 `deploy/standalone/docker-compose.yml`。必须显式二选一：`-ReadOnly` 或 `-ExerciseFixtureWrites`。

### 4.1 ReadOnly

ReadOnly 只能使用预先放入受保护环境变量 `MOCHAT_IDENTITY_SINGLE_CORP_DASHBOARD_TOKEN` 的 Dashboard Token；这样不会为了诊断触发登录写入。脚本只读取健康、权限、企业资料、同步状态和审计接口，读取容器内 `MARIADB_USER`、`MARIADB_PASSWORD`、`MARIADB_DATABASE`，并对 `mc_tenant`、`mc_user`、`mc_corp`、`tenant_corp_bindings`、`saas_admin_users`、`dashboard_identities`、`identity_activations`、`tenant_provision_runs`、`saas_admin_operation_logs`、`dashboard_permission_audits` 执行精确 `COUNT(*)` 前后比较；所有 delta 必须严格为 0。

```powershell
.\scripts\smoke_identity_single_corp.ps1 `
  -EvidenceDir .\identity-single-corp-evidence-readonly `
  -ComposeFile .\deploy\standalone\docker-compose.yml `
  -BaseUrl http://localhost:18080 `
  -ReadOnly
```

ReadOnly 任何业务 count 变化、非 GET 请求、MySQL/Redis container ID 变化、四卷 name/mountpoint 变化或预期外 4xx/5xx 都是阻断条件；`/dashboard/access/profile` 的 403 只有明确的 `CORP_CONFIGURATION_REQUIRED` 才可作为配置门槛处理。

### 4.2 ExerciseFixtureWrites

full smoke 只能通过受保护 API 验证：跨租户资源 404、`expectedVersion` 冲突 409、凭据元数据更新和企业配置审计。它不直接 SQL 写业务事实。Dashboard 登录凭据从受保护环境变量读取；企业微信测试值从受保护 JSON 文件读取，路径通过 `-CredentialSecretFile` 或 `MOCHAT_IDENTITY_SINGLE_CORP_CREDENTIAL_FILE` 提供，绝不把 Secret 放在命令行。

```powershell
.\scripts\smoke_identity_single_corp.ps1 `
  -EvidenceDir .\identity-single-corp-evidence-full `
  -ComposeFile .\deploy\standalone\docker-compose.yml `
  -BaseUrl http://localhost:18080 `
  -CrossTenantUserId 902 `
  -CredentialSecretFile D:\secure\mochat\identity-single-corp-wecom.json `
  -ExerciseFixtureWrites
```

Secret 文件只在内存中组装受保护 API 请求；响应、审计和 evidence 只保存 `configured`、`keyId`、`updatedAt`、版本、状态码和长度等元数据。full smoke 必须断言身份事实（`mc_user`、`mc_corp`、binding、identity、activation、provision run 等）delta 为 0，两个成功受保护 mutation 使 `dashboard_permission_audits` 精确 `+2`；按 `requestId`、`action`、`resultVersion` 读取并核对两条成功审计，stale `409 VERSION_CONFLICT` 不得新增审计或覆盖 version。profile mutation 使用当前 `displayName` 做幂等受保护写，不得永久写入 `identity-single-corp-smoke-*`。脚本不输出 JWT、密码、Secret、密文、哈希或请求正文。

每次 smoke 都记录四个精确卷的 `Name`/`Mountpoint` 以及 `app`、`mysql`、`redis` container ID。允许 app-only 部署后 app ID 改变；MySQL、Redis ID 和四卷必须保持不变。禁止 `down -v`、`docker volume rm`、`docker system prune`、`docker volume prune` 以及重建 MySQL/Redis。

## 5. 阻断条件

出现下列任一情况立即停止，不自动修复：

- fixture 缺少五类账号、存在明文凭据、重复 env key、重复 tenant/corp binding 或 expected ID 不一致；
- SaaS 与 Dashboard Token 相同，或跨 realm 请求不是 401；
- 活跃租户存在零条/多条 binding、CorpID 归属不明确、第二企业事实出现；
- 企业微信无法验证、密文无法解密、响应或日志出现 Secret；
- ReadOnly count 改变，full smoke 的跨租户请求不是 404、stale version 不是 409 或审计/元数据缺失；
- MySQL/Redis container ID 或任一数据卷 name/mountpoint 改变；
- 浏览器出现未声明的 console error、page error 或 4xx/5xx；
- 0129/0130/0131 迁移预检、apply→down→apply 或脏数据失败测试不通过。

阻断证据只能记录对象 ID、计数、状态码、machine code、版本和脱敏错误分类，不复制手机号、Token 或凭据值。

## 6. Task14 app-only 部署与回滚边界

仅在用户明确授权 `mochat-go-desktop` 后按以下顺序执行：

1. 记录 Git HEAD、迁移 ledger、app/mysql/redis ID、四卷 name/mountpoint、关键表 count，并完成备份；
2. 保持 MySQL、Redis 和四卷不变，只停止/重建 `app`；
3. 在隔离临时 MariaDB 运行 0129→0130→0131 的 apply→down→apply，以及重复手机号、多企业歧义、跨租户和不可解密凭据失败测试；
4. 在业务库执行只读 preflight；遇到脏数据、缺失 mapping 或无法加密时停止，不自动猜测或修复；
5. 完成维护窗口迁移和 ledger 记录后，仅通过 app-only Compose 操作部署新应用：

   ```powershell
   docker compose -p mochat-go-desktop -f deploy/standalone/docker-compose.yml up -d --no-deps --force-recreate app
   ```

6. 先执行 ReadOnly smoke，再执行受保护 API 的 full smoke，最后运行真实 live Playwright；不得创建第二企业；
7. 将命令、精确数量、ID/卷保留结果、截图索引、已知限制和回滚边界写入中文验收记录，确认 worktree clean 后再交付。

新身份或新审计已经产生后，不使用旧镜像直接回滚数据库；采用前向修复。只有新系统尚未开放写入且经过批准，才允许按迁移文档执行 down 并恢复旧镜像。任何回滚动作仍不得删除卷或重建 MySQL/Redis。

## 7. Evidence 脱敏清单

允许：Git SHA、HTTP 状态码和 machine code、租户/企业内部 ID、binding/version、表 count、container ID、四卷 name/mountpoint、配置状态、同步计数和脱敏错误分类。

禁止：密码、JWT、激活令牌、`Authorization` header、Cookie、企业微信 Secret、会话存档私钥、密文、哈希、完整手机号、请求/响应正文中的凭据字段。截图应在保存前检查 DOM、下载文件、浏览器 network 和 console，不把受保护输入值复制到文件名或说明中。
