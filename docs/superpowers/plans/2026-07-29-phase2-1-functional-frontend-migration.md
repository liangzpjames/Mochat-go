# Phase 2.1 前端功能迁移实施计划

> **执行要求：** 按任务逐项实施，每项均使用复选框跟踪。实施时必须使用 `subagent-driven-development`（适合允许子代理的环境）或 `executing-plans`；当前环境采用后者。

**目标：** 将所有仅完成 React 路由接管的占位页面替换为从固定 Vue 源码迁移而来的可用功能模块，并在进入 Phase 3 前，通过可重复自动化和人工浏览器点击证明每个旧 URL 基础可用。

**架构：** React 代码按业务功能域组织，不照搬 Vue 文件结构。每个功能域拥有类型化 API 适配、查询 hooks、表单、路由视图和测试；应用之间只共享通用 UI 基础能力和稳定的领域选择器。浏览器验收使用真实 Go 服务、MariaDB、Redis、确定性验收数据和 fake 外部平台。

**技术栈：** React 19、TypeScript 5.9、React Router 7、TanStack Query 5、Dashboard 使用 Ant Design 6、Sidebar/Operation 使用移动端 CSS 组件、Vitest、Playwright、Go、MariaDB、Redis。

## 全局约束

- 保留 Dashboard、Sidebar、Operation 的全部旧 URL、query、hash 和深链。
- 业务字段、权限、API 行为和用户操作对齐旧 Vue；视觉与交互采用当前 React 常用做法。
- 浏览器到 Go、Go 到数据库和缓存必须是真实链路；只有企微、微信等外部平台允许 fake。
- 业务路由不得再通过 `MigratedDashboardPage`、`web/apps/sidebar/src/page.tsx` 或 `web/apps/operation/src/page.tsx` 通过验收。
- 每个适用功能必须用确定性数据完成新建、读取、编辑、状态变更和删除。
- Phase 2.1 完成审计和人工浏览器台账全部通过前，Phase 3 保持阻塞。
- 不修改用户已有的 `.workbuddy/` 和 `web/saas-admin/` 内容。

---

### 任务 1：建立 Phase 2.1 功能矩阵和硬门禁

**文件：**

- 新建：`docs/phases/phase-2.1-functional-frontend-migration/README.md`
- 新建：`docs/phases/phase-2.1-functional-frontend-migration/functional-matrix.csv`
- 新建：`scripts/check_phase2_1_frontend_completion.mjs`
- 新建：`scripts/check_phase2_1_frontend_completion.test.mjs`
- 修改：`package.json`

**接口：**

- 输入：三个应用的 `migration-routes.json`、`web/legacy/*`、Phase 1 页面/API 清单。
- 输出：`pnpm check:phase2.1`、唯一的路由到功能映射，以及针对占位实现和缺失浏览器证据的失败门禁。

- [ ] **步骤 1：先写失败的审计测试**

测试必须拒绝以下记录：

```js
{
  app: 'dashboard',
  route: '/example',
  feature: '',
  implementation: 'placeholder',
  read: 'missing',
  write: 'missing',
  browser: 'missing',
}
```

只有能力状态为 `passed`，或者明确写为 `not-applicable:<原因>` 时才能通过。

运行：

```powershell
node --test scripts/check_phase2_1_frontend_completion.test.mjs
```

预期：检查器尚不存在，测试失败。

- [ ] **步骤 2：实现矩阵检查器**

核心状态规则：

```js
export const allowedStates = new Set(['passed']);
export const isAcceptedState = (value) =>
  allowedStates.has(value) || /^not-applicable:.+/.test(value);
```

检查器必须保证：

- manifest 每条路由恰好对应一条矩阵记录；
- 拒绝 `placeholder` 和 `route-only`；
- 引用的功能、测试和证据文件真实存在；
- 业务路由不能解析到任何通用迁移页面；
- 路由、读、写、权限、fake 外部平台、组件测试和浏览器验收状态均明确。

- [ ] **步骤 3：生成初始功能矩阵**

CSV 列固定为：

```csv
app,feature,legacy_sources,routes,react_module,go_apis,read,write,permissions,fake_external,component_tests,browser,evidence
```

覆盖当前全部 83 条 manifest 路由。已有 Dashboard 专用页面必须重新验证后才能标为 `passed`；通用页面承接的路由初始状态为 `missing`。

- [ ] **步骤 4：注册并验证门禁**

在根 `package.json` 增加：

```json
"check:phase2.1": "node scripts/check_phase2_1_frontend_completion.mjs"
```

运行：

```powershell
node --test scripts/check_phase2_1_frontend_completion.test.mjs
pnpm check:phase2.1
```

预期：检查器单测通过；仓库级门禁按实际情况列出尚未完成的功能并失败。

- [ ] **步骤 5：提交**

```powershell
git add package.json scripts/check_phase2_1_frontend_completion.mjs scripts/check_phase2_1_frontend_completion.test.mjs docs/phases/phase-2.1-functional-frontend-migration
git commit -m "test: add phase2.1 functional frontend gate"
```

### 任务 2：共享功能页基础组件

**文件：**

- 新建：`web/packages/ui/src/data-page.tsx`
- 新建：`web/packages/ui/src/filter-bar.tsx`
- 新建：`web/packages/ui/src/async-boundary.tsx`
- 新建：`web/packages/ui/src/status-action.tsx`
- 新建：`web/packages/ui/src/data-page.test.tsx`
- 修改：`web/packages/ui/src/index.ts`
- 新建：`web/apps/dashboard/src/shared/query-state.ts`
- 新建：`web/apps/dashboard/src/shared/query-state.test.ts`

**输出接口：**

```ts
export type PageState<T> =
  | { status: 'loading' }
  | { status: 'error'; message: string; retry: () => void }
  | { status: 'ready'; rows: T[] };

export function updateSearch(
  current: URLSearchParams,
  changes: Record<string, string | number | undefined>,
): URLSearchParams;
```

- [ ] **步骤 1：先写组件和 URL 状态测试**

覆盖加载、空状态、重试、分页写入 `page/perPage`、筛选保留其他 query、确认前不执行 mutation。

- [ ] **步骤 2：实现最小共享组件**

使用受控 props 和 React Router `URLSearchParams`，不引入全局业务状态仓库。

- [ ] **步骤 3：验证**

```powershell
pnpm --filter @mochat/ui lint
pnpm --filter @mochat/ui test
pnpm --filter @mochat/dashboard test -- src/shared/query-state.test.ts
```

- [ ] **步骤 4：提交**

```powershell
git add web/packages/ui web/apps/dashboard/src/shared
git commit -m "feat: add reusable functional page primitives"
```

### 任务 3：Dashboard 基础业务域

**范围：**

- 企业、成员、部门、客户字段、客户标签、角色、菜单、账号、密码。

**主要文件：**

- 修改：`web/apps/dashboard/src/features/{corp,employee,department,contact-field,contact-tag,role,menu-admin,user-admin,password}/**`
- 新建：`web/e2e/tests/phase2-1-foundation.spec.ts`
- 修改：功能矩阵。

- [ ] **步骤 1：从 Vue 提取行为契约**

逐域记录旧源码、字段与校验、权限按钮、API、query 和跳转，并在现有测试中补齐缺失断言。

- [ ] **步骤 2：完成领域行为**

使用任务 2 的页面状态和筛选组件；写操作只失效对应 query keys。

- [ ] **步骤 3：浏览器 CRUD**

至少覆盖企业编辑、成员筛选/同步、部门成员弹窗、客户字段创建/启停/删除、客户标签创建/编辑/启停、角色创建/复制、菜单创建/编辑/启停、账号创建/启停/重置密码。

- [ ] **步骤 4：人工逐路由浏览**

桌面端打开每条基础路由，点击主要标签和操作，检查控制台与失败请求，保存证据。

- [ ] **步骤 5：验证并提交**

```powershell
pnpm --filter @mochat/dashboard lint
pnpm --filter @mochat/dashboard typecheck
pnpm --filter @mochat/dashboard test
pnpm --filter @mochat/e2e test:e2e -- tests/phase2-1-foundation.spec.ts
pnpm check:phase2.1
```

### 任务 4：Dashboard 客户、内容和统计业务域

**范围：**

- 客户管理、客户群、离职继承、素材库、企业统计、欢迎语、入群欢迎语、公众号。

**主要文件：**

- 新建：`web/apps/dashboard/src/features/{work-contact,work-room,contact-transfer,material-library,statistics,greeting,room-welcome,official-account}/**`
- 修改：`web/apps/dashboard/src/pages/dashboard-page-loaders.ts`
- 新建：`web/e2e/tests/phase2-1-customer-content.spec.ts`
- 修改：功能矩阵。

- [ ] **步骤 1：根据 Vue 调用点编写 API 适配测试**
- [ ] **步骤 2：实现列表、详情和写操作模块**
- [ ] **步骤 3：将相关旧 URL 映射到明确功能组件**
- [ ] **步骤 4：完成自动化和人工浏览器验收**
- [ ] **步骤 5：运行功能测试、E2E、生产构建和 Phase 2.1 门禁后提交**

公众号授权使用 fake 微信响应，但必须经过真实 Go 接口。

### 任务 5：Dashboard 自动化、消息和群运营业务域

**范围：**

- 自动标签、客户群发、客户群群发、标签建群、自动拉群、个人 SOP、群 SOP、敏感词。

**主要文件：**

- 新建：`web/apps/dashboard/src/features/{auto-tag,contact-message-batch,room-message-batch,room-tag-pull,work-room-auto-pull,contact-sop,room-sop,sensitive-words}/**`
- 修改：Dashboard page loaders。
- 新建：`web/e2e/tests/phase2-1-automation-messaging.spec.ts`
- 修改：功能矩阵。

- [ ] **步骤 1：为多步骤表单先写契约和序列化测试**
- [ ] **步骤 2：实现功能模块和可恢复表单状态**
- [ ] **步骤 3：替换全部相关占位路由**
- [ ] **步骤 4：每个功能完成一次创建、查看、编辑/启停和删除闭环**
- [ ] **步骤 5：逐路由人工点击，验证后提交**

fake 平台必须断言媒体上传、消息模板、联系我方式和消息发送请求内容。

### 任务 6：Dashboard 增长和活动业务域

**范围：**

- 渠道活码、门店活码、雷达、抽奖、任务宝、群裂变、群打卡、群质检、群日历、群提醒、无限拉群。

**主要文件：**

- 新建：`web/apps/dashboard/src/features/{channel-code,shop-code,radar,lottery,work-fission,room-fission,room-clock-in,room-quality,room-calendar,room-remind,room-infinite-pull}/**`
- 修改：Dashboard page loaders。
- 新建：`web/e2e/tests/phase2-1-growth-campaigns.spec.ts`
- 修改：功能矩阵。

- [ ] **步骤 1：为每个领域写 API 和向导测试**
- [ ] **步骤 2：实现管理页面、数据统计和多步骤配置**
- [ ] **步骤 3：替换最后一批 Dashboard 占位路由并删除通用迁移页面**
- [ ] **步骤 4：使用 fake 外部平台完成浏览器业务闭环**
- [ ] **步骤 5：运行完整 Dashboard 测试、E2E、构建和门禁后提交**

### 任务 7：Sidebar 移动端功能应用

**文件：**

- 新建：`web/apps/sidebar/src/app/**`
- 新建：`web/apps/sidebar/src/features/{auth,contact,contact-batch-add,contact-sop,room-sop,materials}/**`
- 修改：`web/apps/sidebar/src/page-loaders.ts`
- 删除：`web/apps/sidebar/src/page.tsx`
- 新建：`web/e2e/tests/phase2-1-sidebar.spec.ts`
- 修改：功能矩阵。

- [ ] **步骤 1：编写登录、OAuth 回调、Cookie 和路由测试**
- [ ] **步骤 2：实现客户资料、备注、标签、自定义字段、互动轨迹和素材**
- [ ] **步骤 3：实现个人 SOP、群 SOP 和批量加好友详情**
- [ ] **步骤 4：在 `390x844` 下完成 fake OAuth 和全部旧 URL 点击验收**
- [ ] **步骤 5：删除通用页面，运行 lint/typecheck/test/build/E2E/门禁并提交**

### 任务 8：Operation 活动功能应用

**文件：**

- 新建：`web/apps/operation/src/app/**`
- 新建：`web/apps/operation/src/features/{lottery,room-clock-in,room-fission,room-infinite-pull,shop-code,work-fission}/**`
- 修改：`web/apps/operation/src/page-loaders.ts`
- 删除：`web/apps/operation/src/page.tsx`
- 新建：`web/e2e/tests/phase2-1-operation.spec.ts`
- 修改：功能矩阵。

- [ ] **步骤 1：编写 OAuth、Session 和显式路由组件测试**
- [ ] **步骤 2：实现各活动落地页、说明页、进度页和结果页**
- [ ] **步骤 3：实现抽奖、打卡、助力、领奖、二维码和门店交互**
- [ ] **步骤 4：在移动端和桌面端完成全部旧 URL 点击验收**
- [ ] **步骤 5：删除通用页面，运行 lint/typecheck/test/build/E2E/门禁并提交**

### 任务 9：Phase 2.1 全量验收和 Phase 3 放行

**文件：**

- 新建：`scripts/phase2_1_frontend_acceptance.ps1`
- 新建：`docs/phases/phase-2.1-functional-frontend-migration/evidence/README.md`
- 新建：`docs/phases/phase-2.1-functional-frontend-migration/evidence/results.json`
- 修改：功能矩阵。
- 修改：`docs/phases/phase-3-yuanhu-benchmark/README.md`

- [ ] **步骤 1：先写验收运行器失败测试**

缺少路由证据、存在占位实现、CRUD 未完成、浏览器控制台错误、非预期 `4xx/5xx`、Sidebar/Operation 缺少移动端证据时必须失败。

- [ ] **步骤 2：实现 Windows 原生 PowerShell 验收运行器**

运行器顺序：

1. 校验工具链；
2. 执行 lint、typecheck、unit test、build 和审计；
3. 启动隔离 MariaDB、Redis、Go 和 fake 外部平台；
4. 写入确定性租户、企业、用户和业务数据；
5. 运行全部 Phase 2.1 Playwright 项目；
6. 聚合路由、操作、请求、控制台和截图证据；
7. 只清理自身创建的项目与进程；
8. 写出 `results.json`。

- [ ] **步骤 3：运行自动化全量验收**

```powershell
pwsh -File scripts/phase2_1_frontend_acceptance.ps1
pnpm check:phase2.1
```

- [ ] **步骤 4：执行最终人工浏览器巡检**

- 登录 Dashboard，桌面端逐条打开和点击全部路由。
- 完成 fake OAuth，移动端逐条打开和点击全部 Sidebar 路由。
- 完成 fake OAuth，在移动端和桌面端逐条打开和点击全部 Operation 路由。
- 打开 SaaS Admin，确认平台导航和页面未受影响。

- [ ] **步骤 5：满足全部门禁后解除 Phase 3 阻塞并提交**

只有自动化和人工证据完整时，才能把 Phase 3 状态改为可开始。

