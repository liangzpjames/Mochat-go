# MoChat Go Phase 1 前端统一底座实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use `subagent-driven-development`（推荐）或 `executing-plans` 逐任务执行本计划。所有步骤使用 checkbox（`- [ ]`）跟踪；每个任务必须独立评审、验证并提交。

**Goal:** 在不改变现有业务契约的前提下，把 MoChat 的四个前端纳入单一 pnpm workspace，建立 React 主入口、共享认证/租户/API/路由能力、受控 legacy 兼容层和自动化质量门禁，并用首批 Dashboard 路由证明迁移闭环可工作。

**Architecture:** Go 继续作为唯一后端和静态资源入口。React Dashboard Shell 负责登录态、企业上下文、菜单权限、错误边界和路由决策；未迁移路由仍由 Go 挂载的 legacy 构建产物承接。共享包只提供稳定契约，业务页面留在各 app；每条路由只有在 API、权限、租户、浏览器和视觉回归通过后，才由清单从 `legacy` 切换到 `react`。

**Tech Stack:** React 19.2、TypeScript strict、Vite、React Router Data Mode、TanStack Query、Zustand（仅客户端状态）、React Hook Form、Zod、Ant Design、Vitest、Testing Library、MSW、Playwright、pnpm workspace、Go 1.26、GitHub Actions。

## Global Constraints

- 正式工作分支是 `phase1/frontend-unification-foundation`，工作目录是 `.worktrees/frontend-unification-foundation`。
- 旧三端源码只从 `origin/backup/pre-phase0-main-20260723` 提取；旧 `api-server/` 只用于行为和接口审计，不进入正式运行路径。
- `web/legacy/dashboard`、`web/legacy/sidebar`、`web/legacy/operation` 必须保留原始提交来源、文件哈希和许可证证据。
- React 立即成为 Dashboard 单一主入口；未迁移路由通过显式 allowlist 进入 legacy，不允许静默 fallback。
- Phase 1 不重做产品流程，不修改现有 API 路径、字段、权限语义、企业/租户边界和基本视觉层级。
- Go 仍是唯一后端；不恢复 PHP runtime，不引入微前端框架、SSR、RSC 或实验性路由能力。
- `web/packages/*` 不得反向依赖 `web/apps/*`；共享包不得包含具体业务页面。
- Zustand 仅保存不适合服务端缓存的客户端状态；服务端数据统一由 TanStack Query 管理。
- 所有前端依赖必须由根 `pnpm-lock.yaml` 固定。执行 Task 2 时重新查询 patch 版本；React/React DOM 必须保持同版本且在 `19.2.x` 内。
- 不把真实 token、cookie、密码、企业 ID、密钥或认证快照提交到 Git。
- 本地 `.workbuddy/` 是用户文件，不得修改、删除或提交。
- 新的设计、计划、验收和交接文档继续放在 `docs/handle/`。

## 计划边界与完成定义

本计划只覆盖可独立交付的“统一底座 + 首批迁移闭环”。Dashboard 其余页面、Sidebar 和 Operation 的逐页改写，依据 Task 1 生成的审计矩阵另建批次计划。

Phase 1 完成时必须同时满足：

1. 根目录可执行 `pnpm install --frozen-lockfile`、`pnpm lint`、`pnpm typecheck`、`pnpm test`、`pnpm build`。
2. SaaS Admin 在 workspace 内构建并维持 `/saas-admin/` 部署路径。
3. `/login`、Dashboard Shell、企业上下文、动态菜单和 401/403/404/5xx 处理有自动化测试。
4. React 与 legacy 路由选择完全由版本化 manifest 决定；未知路由返回 React 404，不得落入 legacy。
5. Playwright 至少覆盖登录、企业选择、权限拒绝、legacy 跳转和首批 React 路由。
6. Go 全包测试、现有架构门禁、现有 smoke 和新增前端门禁全部通过。
7. 每项验收命令、退出码、时间和证据路径记录到 `docs/handle/FRONTEND_PHASE1_VERIFICATION.zh-CN.md`。

## 任务与工期总览

| Task | 交付物 | 预计净工时 | 难度 | 独立提交 |
| --- | --- | ---: | --- | --- |
| 1 | 恢复旧源码与六类审计矩阵 | 1.5–2 天 | 高 | `docs: restore and audit legacy frontends` |
| 2 | pnpm workspace、版本策略和工具链 | 1 天 | 中 | `build: establish frontend workspace` |
| 3 | SaaS Admin 纳入 workspace | 0.5–1 天 | 中 | `refactor: move saas admin into workspace` |
| 4 | api-client、auth、routing、config 共享契约 | 2–3 天 | 高 | `feat: add shared frontend contracts` |
| 5 | Dashboard React Shell | 1.5–2 天 | 高 | `feat: add dashboard react shell` |
| 6 | 登录、企业上下文、菜单权限和错误处理 | 3–4 天 | 高 | `feat: add dashboard identity and access flow` |
| 7 | 受控兼容路由与 Go 静态入口 | 2 天 | 高 | `feat: route dashboard through migration manifest` |
| 8 | 首批 Dashboard 路由迁移闭环 | 2–3 天 | 高 | `feat: migrate dashboard foundation routes` |
| 9 | 单元、契约、Playwright、构建和 CI 门禁 | 2 天 | 高 | `ci: enforce frontend migration gates` |
| 10 | 验收、台账与下一批次拆分 | 1 天 | 中 | `docs: verify frontend unification foundation` |

预计总净工时：16–22 人日，不含等待产品、真实账号、生产证据或外部接口可用性的时间。

---

### Task 1: 恢复旧三端源码并建立审计基线

**Files:**

- Create: `web/legacy/README.md`
- Create: `web/legacy/SOURCE_MANIFEST.sha256`
- Create: `web/legacy/dashboard/**`（来自备份分支的 `dashboard/**`）
- Create: `web/legacy/sidebar/**`（来自备份分支的 `sidebar/**`）
- Create: `web/legacy/operation/**`（来自备份分支的 `operation/**`）
- Create: `docs/phases/phase-1-frontend-foundation/audit/pages.csv`
- Create: `docs/phases/phase-1-frontend-foundation/audit/routes.csv`
- Create: `docs/phases/phase-1-frontend-foundation/audit/apis.csv`
- Create: `docs/phases/phase-1-frontend-foundation/audit/permissions.csv`
- Create: `docs/phases/phase-1-frontend-foundation/audit/assets.csv`
- Create: `docs/phases/phase-1-frontend-foundation/audit/dependencies.csv`
- Create: `docs/phases/phase-1-frontend-foundation/audit/licenses.md`
- Create: `scripts/audit_legacy_frontend_inventory.mjs`
- Create: `scripts/audit_legacy_frontend_inventory.test.mjs`

**Interfaces:**

- Consumes: Git tree `origin/backup/pre-phase0-main-20260723:{dashboard,sidebar,operation,api-server}`。
- Produces: `node scripts/audit_legacy_frontend_inventory.mjs --check`。
- Produces CSV columns:
  - `pages.csv`: `app,source_file,route,status,owner,risk,batch`
  - `routes.csv`: `app,path,name,source_file,auth,corp_context,permission,render_target`
  - `apis.csv`: `app,method,path,source_file,request_fields,response_fields,auth,corp_scope,go_evidence`
  - `permissions.csv`: `app,route,menu_link_url,actions,source_file`
  - `assets.csv`: `app,source_file,kind,license_status,used_by`
  - `dependencies.csv`: `app,package,legacy_range,replacement,decision,risk`

- [ ] **Step 1: 验证来源分支、工作区和目标目录**

Run:

```powershell
git rev-parse --verify origin/backup/pre-phase0-main-20260723
git status --short
Test-Path web/legacy
```

Expected: 第一条输出提交 SHA；工作区只包含本计划的文档改动；最后一条为 `False`。

- [ ] **Step 2: 从备份分支提取三个前端**

Run:

```powershell
$archive = Join-Path $env:TEMP "mochat-legacy-frontends.tar"
git archive --format=tar --output=$archive origin/backup/pre-phase0-main-20260723 dashboard sidebar operation
tar -xf $archive -C web/legacy
Remove-Item -LiteralPath $archive
```

提取后目录必须是 `web/legacy/dashboard`、`web/legacy/sidebar`、`web/legacy/operation`。不得提取 `api-server` 到运行目录。

- [ ] **Step 3: 写来源说明和哈希清单**

`web/legacy/README.md` 必须明确：

```markdown
# Legacy frontend reference sources

- Source ref: `origin/backup/pre-phase0-main-20260723`
- Purpose: behavior, contract, asset and visual reference during React migration
- Runtime policy: these sources are never a PHP runtime dependency
- Update policy: changes require a reviewed migration task and refreshed `SOURCE_MANIFEST.sha256`
```

Run:

```powershell
Get-ChildItem web/legacy -Recurse -File |
  Sort-Object FullName |
  Get-FileHash -Algorithm SHA256 |
  ForEach-Object { "$($_.Hash.ToLower())  $($_.Path.Replace((Get-Location).Path + '\','').Replace('\','/'))" } |
  Set-Content web/legacy/SOURCE_MANIFEST.sha256
```

- [ ] **Step 4: 先写审计脚本测试**

测试 fixture 必须证明以下情况非零退出：路由缺少 page、API 缺少 method/path、依赖没有 decision、资源没有 license_status、manifest 中的源文件不存在。

Run:

```powershell
node --test scripts/audit_legacy_frontend_inventory.test.mjs
```

Expected: FAIL，因为审计脚本尚不存在。

- [ ] **Step 5: 实现审计脚本**

脚本读取六个 CSV，验证必填列、唯一键、源文件存在性和交叉引用；错误格式固定为：

```text
frontend-audit: <file>:<row>: <message>
```

成功格式固定为：

```text
frontend-audit: ok (<pages> pages, <routes> routes, <apis> apis)
```

- [ ] **Step 6: 填充真实审计数据**

数据来源至少包括：

- Dashboard：`src/router/asyncRouter.js`、`src/router/router.config.js`、`src/api/*.js`、`src/store/modules/permission.js`、`src/utils/request.js`。
- Sidebar：`src/router/routes.js`、`src/api/*.js`、`src/utils/request.js`、`src/views/**`。
- Operation：`src/router/index.js`、`src/api/*.js`、`src/plugins/axios.js`、`src/views/**`。
- PHP 仅作对照：用 `git show origin/backup/pre-phase0-main-20260723:api-server/<path>` 核验字段和行为。
- Go 契约证据：在 `internal/server/`、`internal/dashboard/`、`internal/store/` 中记录匹配 handler/test 路径。

所有 `status` 初始只能是 `legacy`、`candidate` 或 `blocked`；不能预填 `react`。

- [ ] **Step 7: 完成许可证审计**

`licenses.md` 必须记录仓库根 `LICENSE`、`web/saas-admin/NUWA-APACHE-2.0.txt`、各旧依赖许可证的核验方法，以及无法确认来源的图片/字体清单。许可证不明确的资产标记 `blocked`，不得复制到 React app。

- [ ] **Step 8: 运行审计和来源一致性验证**

Run:

```powershell
node --test scripts/audit_legacy_frontend_inventory.test.mjs
node scripts/audit_legacy_frontend_inventory.mjs --check
git diff --check
```

Expected: 全部退出码 0，且无 CSV 缺项、悬空引用或空许可证状态。

- [ ] **Step 9: 提交**

```powershell
git add web/legacy docs/phases/phase-1-frontend-foundation/audit scripts/audit_legacy_frontend_inventory*
git commit -m "docs: restore and audit legacy frontends"
```

**Acceptance:** 三端源码可追溯且哈希固定；六类审计矩阵能被机器校验；`api-server` 未进入正式运行目录。

---

### Task 2: 建立 pnpm workspace、版本策略与基础门禁

**Files:**

- Create: `package.json`
- Create: `pnpm-workspace.yaml`
- Create: `pnpm-lock.yaml`
- Create: `.npmrc`
- Create: `web/packages/config/package.json`
- Create: `web/packages/config/tsconfig/base.json`
- Create: `web/packages/config/vitest/base.ts`
- Create: `web/packages/config/eslint/index.mjs`
- Create: `scripts/check_frontend_dependency_policy.mjs`
- Create: `scripts/check_frontend_dependency_policy.test.mjs`
- Modify: `.gitignore`

**Interfaces:**

- Produces root scripts: `lint`, `typecheck`, `test`, `build`, `check:deps`, `check:audit`。
- Produces workspace packages: `@mochat/dashboard`, `@mochat/sidebar`, `@mochat/operation`, `@mochat/saas-admin`, `@mochat/api-client`, `@mochat/auth`, `@mochat/routing`, `@mochat/ui`, `@mochat/testing`, `@mochat/config`。
- Runtime floor: Node `>=22.12 <25`; pnpm 固定到执行时最新稳定 major 的精确版本。

- [ ] **Step 1: 查询并记录精确版本**

Run:

```powershell
npm view react@19.2 version
npm view react-dom@19.2 version
npm view react-router version
npm view @tanstack/react-query version
npm view zustand version
npm view react-hook-form version
npm view zod version
npm view antd version
npm view vite version
npm view vitest version
npm view msw version
npm view @playwright/test version
npm view pnpm version
```

Expected: 每条命令输出非空版本。任何查询失败都停止本任务；不得用缓存猜测 patch 版本。

- [ ] **Step 2: 先写依赖策略失败测试**

覆盖：React 与 React DOM 版本不一致、app 使用 `workspace:*` 以外的内部包、共享包依赖 app、根 manifest 缺少 `packageManager`、lockfile 漂移。

Run:

```powershell
node --test scripts/check_frontend_dependency_policy.test.mjs
```

Expected: FAIL，因为策略脚本尚不存在。

- [ ] **Step 3: 创建根 manifest**

根 `package.json` 的稳定接口：

```json
{
  "name": "mochat-go-workspace",
  "private": true,
  "engines": { "node": ">=22.12 <25" },
  "scripts": {
    "check:audit": "node scripts/audit_legacy_frontend_inventory.mjs --check",
    "check:deps": "node scripts/check_frontend_dependency_policy.mjs",
    "lint": "pnpm -r --if-present lint",
    "typecheck": "pnpm -r --if-present typecheck",
    "test": "pnpm -r --if-present test",
    "build": "pnpm -r --if-present build"
  }
}
```

写入查询得到的精确 `packageManager: "pnpm@x.y.z"`，并用 Corepack 激活。

- [ ] **Step 4: 创建 workspace 和共享配置**

`pnpm-workspace.yaml`：

```yaml
packages:
  - web/apps/*
  - web/packages/*
```

`.npmrc`：

```ini
auto-install-peers=false
dedupe-peer-dependents=true
engine-strict=true
prefer-workspace-packages=true
save-exact=true
strict-peer-dependencies=true
```

TypeScript 基线必须包含 `strict: true`、`noUncheckedIndexedAccess: true`、`exactOptionalPropertyTypes: true`、`useUnknownInCatchVariables: true`。

- [ ] **Step 5: 实现依赖策略检查并生成 lockfile**

Run:

```powershell
corepack enable
corepack prepare ((Get-Content package.json | ConvertFrom-Json).packageManager) --activate
pnpm install
node --test scripts/check_frontend_dependency_policy.test.mjs
pnpm check:deps
pnpm install --frozen-lockfile
```

Expected: 全部通过；第二次安装不修改 `pnpm-lock.yaml`。

- [ ] **Step 6: 提交**

```powershell
git add package.json pnpm-workspace.yaml pnpm-lock.yaml .npmrc .gitignore web/packages/config scripts/check_frontend_dependency_policy*
git commit -m "build: establish frontend workspace"
```

**Acceptance:** 新克隆环境可用 Corepack 和 lockfile 重现安装；依赖方向、React 配对版本和内部包引用由机器门禁保护。

---

### Task 3: 将现有 SaaS Admin 纳入 workspace

**Files:**

- Move: `web/saas-admin/**` → `web/apps/saas-admin/**`
- Modify: `web/apps/saas-admin/package.json`
- Modify: `web/apps/saas-admin/vite.config.ts`
- Create: `web/apps/saas-admin/vite.config.test.ts`
- Modify: `web/apps/saas-admin/tsconfig.app.json`
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `deploy/standalone/docker-compose.yml`
- Modify: `scripts/sync_frontend_dist.sh`

**Interfaces:**

- Package name remains `@mochat/saas-admin`。
- Build output: `web/apps/saas-admin/dist`。
- Public base: `/saas-admin/`。
- Config default: `MOCHAT_SAAS_ADMIN_DIST=./web/apps/saas-admin/dist`。

- [ ] **Step 1: 写失败测试**

在 `internal/config/config_test.go` 把 SaaS Admin 默认路径断言改为新目录；新增 Vite 配置测试，断言 `base === "/saas-admin/"`。

Run:

```powershell
go test ./internal/config -run SaaSAdminDist
pnpm --filter @mochat/saas-admin test
```

Expected: FAIL，旧路径和测试脚本尚未更新。

- [ ] **Step 2: 移动源码并清理子 lockfile**

保留现有源码和 `NUWA-APACHE-2.0.txt`，删除 `web/apps/saas-admin/pnpm-lock.yaml`，所有依赖由根 lockfile 管理。

- [ ] **Step 3: 更新构建和 Go 默认路径**

Vite 必须显式设置：

```ts
export default defineConfig({
  base: "/saas-admin/",
  build: { outDir: "dist", emptyOutDir: true },
  plugins: [react(), tailwindcss()],
});
```

同步更新 Compose、构建脚本和测试中的旧路径。

- [ ] **Step 4: 验证功能等价**

Run:

```powershell
pnpm --filter @mochat/saas-admin typecheck
pnpm --filter @mochat/saas-admin build
go test ./internal/config
go test ./internal/frontend
rg -n "web/saas-admin" --glob "!docs/handle/**" .
```

Expected: 前三项通过；最后一项没有运行时代码命中。

- [ ] **Step 5: 提交**

```powershell
git add -A web/saas-admin web/apps/saas-admin internal/config deploy/standalone scripts/sync_frontend_dist.sh pnpm-lock.yaml
git commit -m "refactor: move saas admin into workspace"
```

**Acceptance:** SaaS Admin 从根 workspace 构建，Go 仍在 `/saas-admin/` 正确服务，旧目录没有运行时引用。

---

### Task 4: 实现共享 API、认证、路由和配置契约

**Files:**

- Create: `web/packages/api-client/src/client.ts`
- Create: `web/packages/api-client/src/errors.ts`
- Create: `web/packages/api-client/src/schema.ts`
- Create: `web/packages/api-client/src/client.test.ts`
- Create: `web/packages/auth/src/session.ts`
- Create: `web/packages/auth/src/auth-store.ts`
- Create: `web/packages/auth/src/session.test.ts`
- Create: `web/packages/routing/src/manifest.ts`
- Create: `web/packages/routing/src/resolve-route.ts`
- Create: `web/packages/routing/src/resolve-route.test.ts`
- Create: `web/packages/testing/src/server.ts`
- Create: `web/packages/testing/src/handlers.ts`
- Create: `web/packages/ui/src/error-state.tsx`
- Create: `web/packages/ui/src/loading-state.tsx`
- Create: `web/packages/*/package.json`

**Interfaces:**

```ts
export type ApiEnvelope<T> = { code: number; msg: string; data: T };
export type ApiErrorKind = "unauthorized" | "forbidden" | "validation" | "server" | "network";
export class ApiError extends Error {
  readonly kind: ApiErrorKind;
  readonly status?: number;
  readonly code?: number;
}
export type Session = {
  token: string;
  userId: string;
  corpId: string | null;
  expiresAt: number | null;
};
export type RouteTarget = "react" | "legacy";
export type MigrationRoute = {
  path: string;
  target: RouteTarget;
  auth: boolean;
  corpContext: boolean;
  permission: string | null;
};
export function createApiClient(options: {
  baseUrl: string;
  getToken: () => string | null;
  onUnauthorized: () => void;
}): { request<T>(input: RequestInfo | URL, init?: RequestInit): Promise<T> };
export function resolveRoute(pathname: string, routes: readonly MigrationRoute[]): MigrationRoute | null;
```

- [ ] **Step 1: 写 API client 失败测试**

MSW 覆盖：Bearer token、成功 envelope 解包、401 触发一次 `onUnauthorized`、403 映射、业务 `code !== 0` 映射、非 JSON 5xx、网络异常、AbortError 原样传播。

Run:

```powershell
pnpm --filter @mochat/api-client test
```

Expected: FAIL，因为接口未实现。

- [ ] **Step 2: 最小实现 API client**

实现只接受标准 `fetch`；禁止在共享包直接弹 toast 或跳转。`onUnauthorized` 是唯一认证副作用注入口。

- [ ] **Step 3: 写认证存储失败测试**

断言 session 只从明确的 storage adapter 读写；`clearSession()` 删除 token、userId、corpId 和 expiresAt；损坏 JSON 返回 `null` 并清理。

- [ ] **Step 4: 实现认证 store**

浏览器默认 adapter 使用与旧 Dashboard 审计结果一致的 token 来源；测试使用内存 adapter。不得把企业上下文放入 TanStack Query cache 之外的第二份服务端数据副本。

- [ ] **Step 5: 写路由解析失败测试**

覆盖精确匹配、参数段、尾斜杠规范化、query/hash 忽略、未知路由返回 `null`、重复或歧义模式拒绝加载。

- [ ] **Step 6: 实现路由 manifest schema 和解析**

使用 Zod 在应用启动时解析 manifest。失败时渲染配置错误页，不允许默认进入 legacy。

- [ ] **Step 7: 运行共享包测试和类型检查**

Run:

```powershell
pnpm --filter @mochat/api-client test
pnpm --filter @mochat/auth test
pnpm --filter @mochat/routing test
pnpm --filter "./web/packages/**" typecheck
```

Expected: 全部通过。

- [ ] **Step 8: 提交**

```powershell
git add web/packages pnpm-lock.yaml
git commit -m "feat: add shared frontend contracts"
```

**Acceptance:** 共享包不依赖 app；错误、session 和路由目标均为显式类型；核心边界由单元测试锁定。

---

### Task 5: 建立 Dashboard React Shell

**Files:**

- Create: `web/apps/dashboard/package.json`
- Create: `web/apps/dashboard/index.html`
- Create: `web/apps/dashboard/vite.config.ts`
- Create: `web/apps/dashboard/src/main.tsx`
- Create: `web/apps/dashboard/src/app/router.tsx`
- Create: `web/apps/dashboard/src/app/providers.tsx`
- Create: `web/apps/dashboard/src/layout/dashboard-layout.tsx`
- Create: `web/apps/dashboard/src/layout/dashboard-layout.test.tsx`
- Create: `web/apps/dashboard/src/pages/not-found-page.tsx`
- Create: `web/apps/dashboard/src/styles/index.css`
- Create: `web/apps/dashboard/src/migration-routes.json`

**Interfaces:**

- Package: `@mochat/dashboard`。
- Public base: `/`。
- Build output: `web/apps/dashboard/dist`。
- `createDashboardRouter(deps: DashboardRouterDeps): Router` 使用 React Router Data Mode。
- Query defaults: queries `retry: 1`, mutations `retry: 0`, `refetchOnWindowFocus: false`。

- [ ] **Step 1: 写 Shell 失败测试**

测试无 session 显示登录路由；有效 session 渲染 header/sidebar/content；未知路由显示 React 404；初始 query 失败进入 error boundary。

Run:

```powershell
pnpm --filter @mochat/dashboard test -- dashboard-layout.test.tsx
```

Expected: FAIL，因为 app 尚不存在。

- [ ] **Step 2: 创建 app 和 providers**

Provider 顺序固定为：

```tsx
<StrictMode>
  <QueryClientProvider client={queryClient}>
    <ConfigProvider>
      <App>
        <RouterProvider router={router} />
      </App>
    </ConfigProvider>
  </QueryClientProvider>
</StrictMode>
```

禁止在 `providers.tsx` 获取业务数据。

- [ ] **Step 3: 创建 Data Router**

路由只注册 `/login`、根 Shell、错误页和 manifest 中标记为 `react` 的页面。legacy 路由由 Task 7 的专用 loader 处理。

- [ ] **Step 4: 建立基础布局**

保留旧 Dashboard 的一级/二级菜单信息层级和主要宽度断点；第一阶段不追求像素级重绘，不复制许可证不明的图片。

- [ ] **Step 5: 验证 Shell**

Run:

```powershell
pnpm --filter @mochat/dashboard lint
pnpm --filter @mochat/dashboard typecheck
pnpm --filter @mochat/dashboard test
pnpm --filter @mochat/dashboard build
```

Expected: 全部通过，`web/apps/dashboard/dist/index.html` 存在。

- [ ] **Step 6: 提交**

```powershell
git add web/apps/dashboard pnpm-lock.yaml
git commit -m "feat: add dashboard react shell"
```

**Acceptance:** React Dashboard 可独立启动和构建，根布局、404 和错误边界均有测试，尚未迁移的业务页面未被伪实现。

---

### Task 6: 实现登录、企业上下文、菜单权限与统一错误处理

**Files:**

- Create: `web/apps/dashboard/src/features/auth/auth-api.ts`
- Create: `web/apps/dashboard/src/features/auth/login-page.tsx`
- Create: `web/apps/dashboard/src/features/auth/login-page.test.tsx`
- Create: `web/apps/dashboard/src/features/corp/corp-api.ts`
- Create: `web/apps/dashboard/src/features/corp/corp-provider.tsx`
- Create: `web/apps/dashboard/src/features/corp/corp-provider.test.tsx`
- Create: `web/apps/dashboard/src/features/navigation/menu-api.ts`
- Create: `web/apps/dashboard/src/features/navigation/menu-tree.ts`
- Create: `web/apps/dashboard/src/features/navigation/menu-tree.test.ts`
- Create: `web/apps/dashboard/src/app/access-loader.ts`
- Create: `web/apps/dashboard/src/app/access-loader.test.ts`
- Create: `web/apps/dashboard/src/pages/forbidden-page.tsx`
- Create: `web/apps/dashboard/src/pages/app-error-page.tsx`

**Interfaces:**

```ts
export type CorpOption = { id: string; name: string; authorized: boolean };
export type MenuNode = {
  name: string;
  icon: string | null;
  linkUrl: string | null;
  linkType: 1 | 2;
  children: MenuNode[];
};
export type AccessContext = {
  session: Session;
  corp: CorpOption;
  allowedRoutes: ReadonlySet<string>;
  allowedActions: ReadonlySet<string>;
};
export function buildMenuAccess(nodes: readonly MenuNode[]): {
  routes: ReadonlySet<string>;
  actions: ReadonlySet<string>;
};
```

实际 endpoint、method、字段和成功 code 必须从 Task 1 的 `apis.csv` 读取，不能自行发明；若 Go 尚未实现对应契约，先在 `apis.csv` 标记 `blocked` 并停止本任务。

- [ ] **Step 1: 写登录行为测试**

覆盖空字段、服务端校验错误、成功保存 session、登录后回到安全的同源 `returnTo`、外部 URL 被拒绝、重复提交被禁用。

- [ ] **Step 2: 实现登录页面与 mutation**

React Hook Form 负责表单状态，Zod 负责客户端最小校验，服务端错误映射到字段或全局 alert。不得把密码写入 store、query key、日志或 URL。

- [ ] **Step 3: 写企业上下文测试**

覆盖无企业、单企业自动选择、多企业显式选择、无授权企业禁用、切换企业后取消旧请求并使企业作用域 query 失效。

- [ ] **Step 4: 实现企业 provider**

所有企业作用域 query key 以 `["corp", corpId, ...]` 开头。切换企业的唯一顺序：

1. 取消当前企业 queries；
2. 持久化新 corpId；
3. 清理旧企业 cache；
4. 重新验证菜单；
5. 导航到新企业首个允许路由。

- [ ] **Step 5: 写菜单权限测试**

复刻旧 `dealPermissionData` 的三级菜单和四级 action 语义，覆盖外链 `linkType === 2`、隐藏路由、空菜单、无效 `linkUrl` 和重复 action。

- [ ] **Step 6: 实现 access loader 和错误映射**

- 无 session：redirect `/login?returnTo=<encoded-local-path>`。
- session 过期或 API 401：清 session，redirect `/login`。
- 路由无权限：返回 403 页面，不得进入 legacy。
- 无企业上下文：进入企业选择态。
- 404：React 404。
- 5xx/network：错误页提供 retry，不能自动无限重试。

- [ ] **Step 7: 运行测试**

Run:

```powershell
pnpm --filter @mochat/dashboard test -- login-page corp-provider menu-tree access-loader
pnpm --filter @mochat/dashboard typecheck
pnpm --filter @mochat/dashboard build
```

Expected: 全部通过。

- [ ] **Step 8: 提交**

```powershell
git add web/apps/dashboard/src/features web/apps/dashboard/src/app web/apps/dashboard/src/pages pnpm-lock.yaml
git commit -m "feat: add dashboard identity and access flow"
```

**Acceptance:** 登录、企业与菜单权限形成单一数据流；401/403/404/5xx 行为确定；没有跨企业 cache 泄漏。

---

### Task 7: 建立显式 migration manifest 和受控 legacy 路由

**Files:**

- Create: `web/apps/dashboard/src/app/legacy-route-loader.ts`
- Create: `web/apps/dashboard/src/app/legacy-route-loader.test.ts`
- Create: `internal/frontend/migration_manifest.go`
- Create: `internal/frontend/migration_manifest_test.go`
- Modify: `internal/frontend/frontend.go`
- Modify: `cmd/mochat-go/main.go`
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `scripts/sync_frontend_dist.sh`

**Interfaces:**

- Manifest source: `web/apps/dashboard/src/migration-routes.json`。
- Go config:
  - `MOCHAT_DASHBOARD_DIST=./web/apps/dashboard/dist`
  - `MOCHAT_LEGACY_DASHBOARD_DIST=./web/dashboard/dist`
- Legacy mount: `/_legacy/dashboard/`，只允许 manifest 中 `target: "legacy"` 的路径。
- React browser URL 保持原业务路径；loader 使用同源全页导航进入 legacy mount，并携带原 query/hash。

- [ ] **Step 1: 写 Go manifest 失败测试**

覆盖：合法 manifest、重复路由、未知 target、非绝对路径、React/legacy mount 冲突、未在 manifest 的路径拒绝服务、SPA asset 正常服务。

Run:

```powershell
go test ./internal/frontend -run MigrationManifest
```

Expected: FAIL，因为 manifest reader 尚不存在。

- [ ] **Step 2: 实现 Go manifest reader**

Go 与 TypeScript 读取同一 JSON schema。服务启动时校验失败必须返回构造错误，不能只写日志后继续。

- [ ] **Step 3: 写浏览器 loader 失败测试**

断言只有 `target === "legacy"` 且通过认证、企业和权限检查的路由会生成：

```text
/_legacy/dashboard/<original-path>?<original-query>#<original-hash>
```

未知路由和无权限路由不得生成 legacy URL。

- [ ] **Step 4: 实现 legacy loader**

使用 `window.location.replace()` 做全页切换；不使用 iframe，不在 React/legacy 间同步运行时状态。认证继续使用现有同源 cookie/Bearer 语义。

- [ ] **Step 5: 修改 Go 静态入口**

服务顺序必须是：

1. API/server handler；
2. `/_legacy/dashboard/` allowlist handler；
3. `/sidebar-app/`、`/operation-app/`、`/saas-admin/`；
4. React Dashboard SPA fallback。

API 路径和静态 asset 路径不得被 SPA fallback 吞掉。

- [ ] **Step 6: 更新配置和同步脚本**

保留旧 dist 供兼容层使用，但默认 Dashboard dist 改为 React 构建目录。为两个 dist 分别写配置测试。

- [ ] **Step 7: 验证**

Run:

```powershell
go test ./internal/frontend ./internal/config
pnpm --filter @mochat/dashboard test -- legacy-route-loader
pnpm --filter @mochat/dashboard build
go test ./...
```

Expected: 全部通过。

- [ ] **Step 8: 提交**

```powershell
git add web/apps/dashboard internal/frontend internal/config cmd/mochat-go scripts/sync_frontend_dist.sh
git commit -m "feat: route dashboard through migration manifest"
```

**Acceptance:** React 是根入口；legacy 仅能服务清单中的已知路由；未知、未授权和 API 请求不会错误 fallback。

---

### Task 8: 迁移首批 Dashboard 基础路由并证明切换闭环

**Files:**

- Create: `web/apps/dashboard/src/features/account/password-update-page.tsx`
- Create: `web/apps/dashboard/src/features/account/password-update-page.test.tsx`
- Create: `web/apps/dashboard/src/features/corp/corp-page.tsx`
- Create: `web/apps/dashboard/src/features/corp/corp-page.test.tsx`
- Create: `web/apps/dashboard/src/features/home/home-page.tsx`
- Create: `web/apps/dashboard/src/features/home/home-page.test.tsx`
- Modify: `web/apps/dashboard/src/migration-routes.json`
- Modify: `docs/phases/phase-1-frontend-foundation/audit/pages.csv`
- Modify: `docs/phases/phase-1-frontend-foundation/audit/routes.csv`
- Modify: `docs/phases/phase-1-frontend-foundation/audit/apis.csv`

**Interfaces:**

- Candidate routes:
  - `/passwordUpdate/index`
  - `/corp/index`
  - `/corpData/index`
- 只有 Task 1 审计确认 API 已由 Go 覆盖且许可证无阻塞的候选路由才能切为 `react`。

- [ ] **Step 1: 为每条候选路由建立迁移卡**

在 `pages.csv` 中补齐 owner、risk、batch；在 `routes.csv` 和 `apis.csv` 明确 query 参数、表单字段、权限、企业作用域、成功/错误响应和 Go 测试证据。

若任一路由缺少 Go 契约证据，将其保持 `legacy`，并从本任务交付中剔除；不能用 mock 伪装完成。

- [ ] **Step 2: 先写页面契约测试**

每页至少覆盖：

- loader 使用正确 corpId；
- API method/path/request body 与 `apis.csv` 一致；
- action 权限隐藏或禁用写操作；
- 401、403、业务错误和空数据；
- 成功后 cache invalidation 和用户反馈；
- 原 query 参数仍可工作。

- [ ] **Step 3: 实现最小等价页面**

复用 `@mochat/api-client`、`@mochat/auth`、`@mochat/ui`；不抽象尚未出现第二个消费者的业务组件。视觉以旧页截图和 DOM 结构为基准，保留标题、字段顺序、主要操作和响应式断点。

- [ ] **Step 4: 逐条切换 manifest**

每次只把一条已通过测试的路由从 `legacy` 改为 `react`，随后运行该页测试和 legacy allowlist 测试。禁止一次性批量改状态。

- [ ] **Step 5: 更新审计状态**

`pages.csv`、`routes.csv` 中对应项改为 `react`，`apis.csv` 记录最终 Go 证据。没有迁移的路由保持 `legacy`。

- [ ] **Step 6: 验证**

Run:

```powershell
pnpm --filter @mochat/dashboard test
node scripts/audit_legacy_frontend_inventory.mjs --check
go test ./internal/frontend
pnpm --filter @mochat/dashboard build
```

Expected: 全部通过；manifest 和 CSV 状态一致。

- [ ] **Step 7: 提交**

```powershell
git add web/apps/dashboard/src/features web/apps/dashboard/src/migration-routes.json docs/phases/phase-1-frontend-foundation/audit
git commit -m "feat: migrate dashboard foundation routes"
```

**Acceptance:** 至少一条真实 Dashboard 路由完成 React 切换；若三条候选均被真实契约阻塞，本任务不得提交“完成”，而应在风险台账登记并先补 Go 契约计划。

---

### Task 9: 建立前端测试、浏览器回归、构建与 CI 门禁

**Files:**

- Create: `web/packages/testing/src/render-dashboard.tsx`
- Create: `web/e2e/package.json`
- Create: `web/e2e/playwright.config.ts`
- Create: `web/e2e/tests/dashboard-auth.spec.ts`
- Create: `web/e2e/tests/dashboard-access.spec.ts`
- Create: `web/e2e/tests/dashboard-migration.spec.ts`
- Create: `scripts/frontend_check.ps1`
- Create: `scripts/frontend_check.sh`
- Create: `scripts/test_frontend_check.ps1`
- Modify: `scripts/test.sh`
- Modify: `.github/workflows/mysql57-amd64.yml`

**Interfaces:**

- Produces:
  - Windows: `pwsh scripts/frontend_check.ps1 quick|build|e2e`
  - Linux: `bash scripts/frontend_check.sh quick|build|e2e`
- `quick`: audit + dependency policy + lint + typecheck + unit/contract tests。
- `build`: frozen install + all app builds + dist existence check。
- `e2e`: 启动测试后端/Go server，执行 Playwright Chromium。

- [ ] **Step 1: 写门禁脚本失败测试**

覆盖非法参数、Node 版本不满足、pnpm 版本不匹配、lockfile 漂移、任一 app 缺 dist、子命令失败码透传。

Run:

```powershell
pwsh scripts/test_frontend_check.ps1
```

Expected: FAIL，因为门禁脚本尚不存在。

- [ ] **Step 2: 实现跨平台门禁**

PowerShell 和 shell 必须执行相同命令顺序并 fail-fast；不得在 CI 自动修复 lint 或更新 lockfile。

- [ ] **Step 3: 写 Playwright 登录与企业场景**

覆盖：

- 未登录访问受保护 URL → `/login?returnTo=...`；
- 登录成功 → 安全 returnTo；
- 多企业选择与切换；
- 401 清理 session；
- 403 停留在 React 错误页。

- [ ] **Step 4: 写 Playwright 迁移场景**

覆盖：

- React 路由由 React DOM 渲染；
- legacy allowlist 路由进入 `/_legacy/dashboard/`；
- query/hash 保留；
- 未知路由显示 React 404；
- 未在 manifest 的 legacy URL 返回 404；
- `/api` 请求不返回 `index.html`。

- [ ] **Step 5: 接入现有 test.sh 和 CI**

CI 顺序固定为：

1. `pnpm install --frozen-lockfile`
2. `pnpm check:audit`
3. `pnpm check:deps`
4. `pnpm lint`
5. `pnpm typecheck`
6. `pnpm test`
7. `pnpm build`
8. Go tests and architecture gates
9. Playwright Chromium

缓存 key 必须包含 `pnpm-lock.yaml` hash；缓存只存 pnpm store 和 Playwright browser，不缓存 dist。

- [ ] **Step 6: 本地完整验证**

Run:

```powershell
pwsh scripts/test_frontend_check.ps1 quick
pwsh scripts/frontend_check.ps1 build
pwsh scripts/frontend_check.ps1 e2e
bash scripts/frontend_check.sh quick
bash scripts/frontend_check.sh build
go test ./...
```

Expected: 全部退出码 0。若 Windows 没有 Bash，用项目现有 Docker/Linux 验证入口执行两条 shell 命令。

- [ ] **Step 7: 提交**

```powershell
git add web/packages/testing web/e2e scripts/frontend_check.* scripts/test_frontend_check.ps1 scripts/test.sh .github/workflows/mysql57-amd64.yml pnpm-lock.yaml
git commit -m "ci: enforce frontend migration gates"
```

**Acceptance:** Windows/Linux 入口等价，关键权限与路由行为有真实浏览器覆盖，CI 不接受 lockfile 漂移或无测试的 manifest 切换。

---

### Task 10: 完成 Phase 1 验收、风险台账和下一批迁移拆分

**Files:**

- Create: `docs/handle/FRONTEND_PHASE1_VERIFICATION.zh-CN.md`
- Create: `docs/handle/FRONTEND_MIGRATION_RUNBOOK.zh-CN.md`
- Modify: `docs/handle/RISK_REGISTER.zh-CN.md`
- Modify: `docs/handle/RESOURCE_AND_SECRET_INVENTORY.zh-CN.md`
- Modify: `docs/handle/REQUIREMENT_REGISTER.zh-CN.md`
- Modify: `docs/handle/README.md`
- Modify: `docs/handle/CURRENT_SESSION_HANDOFF.zh-CN.md`

**Interfaces:**

- Verification row format: `time,commit,command,exit_code,evidence,result`。
- 下一批次排序依据：`blocked` 优先解阻；其余按 `risk asc`、共享 API 覆盖率、页面依赖关系排序。

- [ ] **Step 1: 从干净工作区执行最终门禁**

Run:

```powershell
git status --short
pnpm install --frozen-lockfile
pwsh scripts/frontend_check.ps1 quick
pwsh scripts/frontend_check.ps1 build
pwsh scripts/frontend_check.ps1 e2e
go test ./...
bash scripts/dev_check.sh quick
git diff --check
```

Expected: 起始 `git status` 为空；所有命令退出码 0。

- [ ] **Step 2: 记录验收证据**

`FRONTEND_PHASE1_VERIFICATION.zh-CN.md` 对每条命令记录 UTC+8 时间、当前 SHA、退出码和证据路径。不得粘贴 token、cookie、密码或完整环境变量。

- [ ] **Step 3: 编写逐页迁移 runbook**

每个页面必须遵循：

1. 从审计矩阵领取一个 `candidate`；
2. 补齐 API/权限/租户/浏览器/视觉基线；
3. 失败测试先行；
4. 实现页面；
5. 运行单页、契约、Playwright、build；
6. 单条 manifest 切换；
7. 更新 CSV 和风险台账；
8. 独立提交。

- [ ] **Step 4: 拆分后续计划**

按审计结果生成而不是凭文件数量估算：

- Dashboard batch 2：高复用基础 CRUD/列表页面；
- Dashboard batch 3：素材、富文本、图表、二维码等高依赖页面；
- Dashboard batch 4：复杂营销和异步任务页面；
- Sidebar migration；
- Operation migration；
- legacy 源码和 dist 最终移除。

每批计划必须能独立上线和回滚，且继续使用本计划的 manifest 门禁。

- [ ] **Step 5: 更新台账和交接**

记录：

- 许可证或生产账号阻塞；
- Node/pnpm/Playwright 外部资源；
- CI 运行时间和 flaky 风险；
- legacy 清理条件；
- 当前 manifest 中 React/legacy 路由计数；
- 下一任务的精确入口。

- [ ] **Step 6: 提交**

```powershell
git add docs/handle
git commit -m "docs: verify frontend unification foundation"
```

**Acceptance:** 验收证据可追溯，风险和资源不遗漏，下一批迁移可由零上下文工程师直接领取。

---

## 任务依赖图

```text
Task 1 审计 ───────┐
                   ├─> Task 4 共享契约 ─> Task 5 Shell ─> Task 6 身份权限
Task 2 Workspace ──┤                                      │
                   └─> Task 3 SaaS Admin                  ├─> Task 7 兼容路由
                                                          │
                                                          └─> Task 8 首批迁移
                                                                  │
                                                                  v
                                                         Task 9 CI/E2E
                                                                  │
                                                                  v
                                                         Task 10 验收交接
```

Task 3 可在 Task 1 完成后与 Task 4 的前半段并行；Task 5–10 必须按依赖顺序执行。任何 manifest 状态变更都必须在相同提交中更新审计 CSV 和相应测试。

## 回滚策略

- Workspace/工具链失败：回滚对应独立提交，不影响现有 Go 和旧 dist。
- SaaS Admin 路径失败：恢复 `MOCHAT_SAAS_ADMIN_DIST` 旧值和原目录提交。
- React Shell 发布失败：把 Go Dashboard dist 配置指回 `./web/dashboard/dist`；API 和数据库无需回滚。
- 单页迁移失败：只把该路由 manifest 从 `react` 改回 `legacy`，保留代码和失败证据供修复；不得批量回退其他已验证页面。
- 兼容层本身失败：回滚 Task 7 提交并恢复旧 Dashboard 静态入口。

所有回滚都必须新增一条 `RISK_REGISTER.zh-CN.md` 记录，说明触发条件、影响路由、证据、负责人和重新启用条件。

## 计划自检结果

- 交接要求的旧源码/许可证、六类审计、workspace、SaaS Admin、Dashboard Shell、认证/企业/菜单/错误/兼容路由、单元/契约/Playwright/构建/CI 均有明确任务。
- 每个任务均列出文件、接口、命令、验收、工时、难度和独立提交点。
- 没有把 Sidebar、Operation 或 Dashboard 全量页面改写混入底座交付；后续批次由审计矩阵驱动。
- 占位符扫描无命中，所有步骤都有确定动作或验收定义。
- TypeScript/Go 之间共享的 manifest 字段保持 `path,target,auth,corpContext,permission` 一致。
