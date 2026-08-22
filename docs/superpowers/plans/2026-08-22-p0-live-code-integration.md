# Mochat 今日 P0 活码集成与门禁修复实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 基于 `origin/main=b9a47cab` 形成唯一的渠道/群活码工作区，解决迁移占号、RBAC seed/catalog 和 benchmark phase 门禁，并完成全量自动化与三视口 Docker 浏览器验收。

**Architecture:** 语义重放已存在的活码领域、存储、路由和页面能力，不 merge 分叉 main；`LiveCodeWorkspacePage` 是两个活码路由的唯一页面实现。活码 schema 使用 0153，0154 负责把历史资源增量与停用语义收敛为 catalog 精确合同。

**Tech Stack:** Go、MariaDB/MySQL、React、TypeScript、TanStack Query、Vitest、Testing Library、Node 合同检查、pnpm、Docker Compose、应用内浏览器。

## Global Constraints

- 设计依据：`docs/superpowers/specs/2026-08-22-p0-live-code-integration-design.md`。
- 不修改 Phase 7 与 `docs/PROJECT_PROGRESS.zh-CN.md`。
- 不读取后提交用户当前树的 `.workbuddy/`、debug 脚本、`tmp/`、`web/saas-admin/` 或未跟踪 0152 实现。
- 所有生产行为先有正确失败的测试或门禁复现；失败先定位根因。
- 不引入生产假数据、占位二维码或伪造统计。
- Docker 不删除命名卷，不执行 `down -v` 或 `down --volumes`。

---

### Task 1: 语义重放活码领域并改号为 0153

**Files:**
- Create: `deploy/standalone/migrations/0153_live_code_workspace.up.sql`
- Create: `deploy/standalone/migrations/0153_live_code_workspace.down.sql`
- Modify: `internal/migration/live_code_workspace_migration_test.go`
- Modify: `internal/migration/migration_test.go`
- Modify: `internal/dashboard/dashboard_page_catalog.json`
- Modify: `internal/dashboard/channel_code*.go`
- Modify: `internal/dashboard/work_room_auto_pull.go`
- Modify: `internal/store/channel_code_workspace.go`
- Modify: `internal/store/mysql.go`
- Modify: `internal/server/server.go`
- Modify: `cmd/mochat-go/main.go`
- Test: `internal/dashboard/*channel_code*_test.go`
- Test: `internal/store/channel_code_workspace_test.go`

**Interfaces:**
- Produces: 渠道生命周期、统计/导出/作废；群分组与事件账本；对应 server options/routes。
- Preserves: 0150 关键词快照修复、0151 AI 资源语义。

- [ ] **Step 1: 逐个检查并重放有效提交**

按 `470b5485`、`f11f4ced`、`d88c6a66`、`dea43188`、`a3087c2a`、`a44388a5` 顺序 cherry-pick；迁移冲突选择保留远端 0150 文件，再把活码迁移内容放入 0153。禁止 cherry-pick `0b03f393` 整合提交。

- [ ] **Step 2: 先运行迁移合同测试确认旧编号引用失败**

Run: `go test ./internal/migration -run 'TestLiveCodeWorkspaceMigrationContract|TestDefaultMigrationsDiscovers' -count=1`

Expected: 因测试仍寻找 0150 活码文件或 0153 尚未存在而 FAIL，而不是编译错误。

- [ ] **Step 3: 统一迁移与测试引用**

测试必须精确读取：

```go
upPath := filepath.Join(root, "deploy", "standalone", "migrations", "0153_live_code_workspace.up.sql")
downPath := filepath.Join(root, "deploy", "standalone", "migrations", "0153_live_code_workspace.down.sql")
```

迁移内容保留本地有效提交的字段、表、索引和资源 seed，错误文字统一写 0153。

- [ ] **Step 4: 运行迁移、Dashboard、store 与 server 目标测试**

Run: `go test ./internal/migration ./internal/dashboard ./internal/store ./internal/server -run 'LiveCode|ChannelCode|WorkRoomAutoPull|RegisteredRoutes' -count=1`

Expected: PASS。

- [ ] **Step 5: 提交领域集成**

Commit: `feat: integrate live code workspaces on migration 0153`

### Task 2: 收敛前端为单一专用页面并吸收真实选择交互

**Files:**
- Modify: `web/apps/dashboard/src/features/phase34/acquisition-pages.tsx`
- Modify: `web/apps/dashboard/src/features/phase34/acquisition-pages.test.tsx`
- Create: `web/apps/dashboard/src/features/phase34/live-code/live-code-pages.tsx`
- Create: `web/apps/dashboard/src/features/phase34/live-code/live-code-api.ts`
- Create: `web/apps/dashboard/src/features/phase34/live-code/live-code-types.ts`
- Modify: `web/apps/dashboard/src/styles/index.css`

**Interfaces:**
- Consumes: `/channelCode/*`、`/workRoomAutoPull/*`、`/workEmployee/index`、`/workRoom/roomIndex`。
- Produces: `LiveCodeWorkspacePage({kind:'channel'|'group'})`，真实员工多选和最多 5 个有序群聊选择。

- [ ] **Step 1: 重放抽屉与合并适配提交**

按 `a28dd4be`、`341b2585` 重放抽屉/页面对齐，再语义吸收 `b01f2d1d` 的真实员工与群聊选择。确认 registry 只有专用页面映射，没有第二个活码页面分支。

- [ ] **Step 2: 运行现有 acquisition 测试**

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/features/phase34/acquisition-pages.test.tsx`

Expected: 真实选项加载、选择/取消、群聊确认和提交 payload 测试 PASS。

- [ ] **Step 3: 检查生产表单无技术字段**

Run: `rg -n '使用成员 ID|群聊配置 JSON|JSON。' web/apps/dashboard/src/features/phase34/live-code web/apps/dashboard/src/features/phase34/acquisition-pages.tsx`

Expected: 无匹配。

- [ ] **Step 4: 提交权威页面整合**

Commit: `feat: unify live code page interactions`

### Task 3: 以 TDD 补齐创建/编辑抽屉状态机和移动布局

**Files:**
- Modify: `web/apps/dashboard/src/features/phase34/live-code/live-code-pages.tsx`
- Modify: `web/apps/dashboard/src/features/phase34/acquisition-pages.test.tsx`
- Modify: `web/apps/dashboard/src/styles/index.css`
- Create or Modify: `web/apps/dashboard/src/styles/phase34-live-code-layout.test.ts`

**Interfaces:**
- Produces: `LiveCodeUpsertDialog` 创建/编辑模式；详情加载、字段级错误、总错误摘要、保存失败保留、遮罩/按钮/Escape 关闭；<768px 单列抽屉。

- [ ] **Step 1: 写编辑与校验失败测试**

增加下列行为断言：

```tsx
expect(screen.getByRole('dialog', { name: '编辑渠道活码' })).toBeInTheDocument();
expect(screen.getByLabelText('渠道活码名称')).toHaveValue('官网咨询');
await user.clear(screen.getByLabelText('渠道活码名称'));
await user.click(screen.getByRole('button', { name: '保存渠道活码' }));
expect(screen.getByText('请输入活码名称')).toBeInTheDocument();
expect(screen.getByRole('alert')).toHaveTextContent('请检查填写是否合规');
expect(api.write).not.toHaveBeenCalled();
```

群活码测试同时断言详情由 `GET /workRoomAutoPull/show` 回填、保存调用 `PUT /workRoomAutoPull/update`，接口失败后对话框和输入仍存在。

- [ ] **Step 2: 运行测试确认 RED**

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/features/phase34/acquisition-pages.test.tsx`

Expected: 因当前页面没有编辑抽屉/错误摘要而 FAIL。

- [ ] **Step 3: 做最小编辑实现**

创建和编辑复用同一表单状态；编辑打开时使用行 ID 调详情接口，加载成功后回填名称、员工和群聊。提交方法：

```ts
const endpoint = isChannel ? '/channelCode/update' : '/workRoomAutoPull/update';
await api.write(endpoint, { ...payload, [idKey]: recordID }, 'PUT');
```

校验失败设置逐字段错误和 `请检查填写是否合规`；保存失败只更新错误，不清空状态。

- [ ] **Step 4: 写样式合同 RED**

样式测试读取 `styles/index.css`，断言 `<768px` media block 包含抽屉 `width: 100%`、表单单列和 sticky footer。先运行并确认缺失时 FAIL。

- [ ] **Step 5: 实现响应式与关闭行为并回归 GREEN**

移动样式至少包含：

```css
@media (max-width: 767px) {
  .phase34-live-code-drawer .phase34-detail-panel { width: 100%; max-width: none; }
  .phase34-live-code-picker-grid { grid-template-columns: 1fr; }
  .phase34-live-code-drawer-footer { bottom: 0; position: sticky; }
}
```

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/features/phase34/acquisition-pages.test.tsx src/styles/phase34-live-code-layout.test.ts`

Expected: PASS。

- [ ] **Step 6: 提交交互补全**

Commit: `feat: complete live code edit and responsive states`

### Task 4: 修复 benchmark phase 合同

**Files:**
- Modify: `web/apps/dashboard/src/benchmark/manifest.json`
- Test: `scripts/check_yuanhu_benchmark_manifest.mjs`

- [ ] **Step 1: 保留已复现 RED 证据**

基线命令已输出：`page 8 has invalid phase: 3-final`。

- [ ] **Step 2: 只修改真实阶段值**

将 `/chat/file-audio` 的 `phase` 改为 `3.5`；不得修改 `allowedPhases`。

- [ ] **Step 3: 运行 GREEN**

Run: `corepack pnpm check:yuanhu-benchmark`

Expected: `53 Yuanhu benchmark pages validated.` 或等价成功输出，exit 0。

- [ ] **Step 4: 提交阶段合同修复**

Commit: `fix: align file audio benchmark phase`

### Task 5: 用 0154 对齐 RBAC catalog、迁移和校验器

**Files:**
- Create: `deploy/standalone/migrations/0154_dashboard_page_rbac_reconciliation.up.sql`
- Create: `deploy/standalone/migrations/0154_dashboard_page_rbac_reconciliation.down.sql`
- Modify: `scripts/check_dashboard_page_rbac_catalog.mjs`
- Modify: `scripts/check_dashboard_page_rbac_catalog.test.mjs`
- Modify: `internal/dashboard/dashboard_page_catalog.json`
- Test: `internal/migration/migration_test.go`

**Interfaces:**
- Produces: `applyPermissionResourceReconciliation({mappings, overlaySource})`，支持 additions 与 deactivations，返回最终精确 seed。

- [ ] **Step 1: 写 overlay 失败测试**

```js
const result = applyPermissionResourceReconciliation({
  mappings: ['dashboard.chat.export\tGET /dashboard/workMessage/toUsers\t1'],
  overlaySource: reconciliationSQL,
});
assert.deepEqual(result, ['dashboard.customer.inheritance\tGET /dashboard/contactTransfer/info\t0']);
```

另加重复 additions、停用不存在 mapping 和 scope 不一致的拒绝测试。

- [ ] **Step 2: 运行 RED**

Run: `node --test scripts/check_dashboard_page_rbac_catalog.test.mjs`

Expected: 因函数不存在或未处理 deactivations 而 FAIL。

- [ ] **Step 3: 新增 0154 migration 与最小 overlay 实现**

0154 up 使用带 `permission_code/http_method/path_pattern/scope_required` 的 derived table 幂等补种最终 catalog 增量，并用 `deactivation_seed` 停用旧资源。校验器读取 0154 后构造：

```js
const seededMappings = applyPermissionResourceReconciliation({
  mappings: employeeAccountMappings,
  overlaySource: await readFile('deploy/standalone/migrations/0154_dashboard_page_rbac_reconciliation.up.sql', 'utf8'),
});
```

- [ ] **Step 4: 运行单测和完整 RBAC 门禁**

Run: `node --test scripts/check_dashboard_page_rbac_catalog.test.mjs`

Run: `corepack pnpm check:phase4-dashboard-page-rbac`

Expected: 两者 PASS；真实 MariaDB 无 DSN 时 provider 子门禁明确 SKIP，RBAC catalog 最终通过。

- [ ] **Step 5: 提交 RBAC 修复**

Commit: `fix: reconcile dashboard page RBAC resources`

### Task 6: 自动化全量验证

**Files:**
- Verify all changed files

- [ ] **Step 1: 目标测试与 Dashboard 全量**

Run:

```powershell
corepack pnpm --filter @mochat/dashboard exec vitest run src/features/phase34/acquisition-pages.test.tsx
corepack pnpm --filter @mochat/dashboard test
corepack pnpm --filter @mochat/dashboard typecheck
corepack pnpm --filter @mochat/dashboard build
```

Expected: 全部 exit 0，记录测试文件数和用例数。

- [ ] **Step 2: Go 与合同门禁**

Run:

```powershell
go test ./internal/dashboard ./internal/store ./internal/server ./internal/migration ./cmd/mochat-go -count=1
corepack pnpm check:phase4-dashboard-page-rbac
corepack pnpm check:yuanhu-benchmark
corepack pnpm check:dashboard-all-pages-evidence
corepack pnpm check:mobile-clients-foundation
corepack pnpm check:provider-completion
git diff --check origin/main...HEAD
```

Expected: 本任务可控门禁全部 PASS；真实 MariaDB 仅在缺少 `MOCHAT_GO_MYSQL_INTEGRATION_DSN` 时记录 SKIP。

### Task 7: Docker 三视口浏览器验收与报告

**Files:**
- Create: `docs/implementation/2026-08-22-p0-live-code-integration-result.zh-CN.md`
- Create: `docs/verification/2026-08-22-p0-live-code-integration-verification.zh-CN.md`

- [ ] **Step 1: 检查容器和卷，不删除任何卷**

Run:

```powershell
docker compose -p mochat-go-desktop -f deploy/standalone/docker-compose.yml ps
docker volume ls --filter label=com.docker.compose.project=mochat-go-desktop
```

- [ ] **Step 2: 仅重建 app 并等待健康**

Run: `docker compose -p mochat-go-desktop --env-file deploy/standalone/.env.local -f deploy/standalone/docker-compose.yml up -d --build --no-deps app`

Expected: app、MySQL、Redis 达到 compose 预期健康状态，原命名卷仍存在。

- [ ] **Step 3: 应用内浏览器验收 2560×1440、1440×900、390×844**

覆盖两个路由的列表、空态、查询/重置、分页、详情、创建、编辑、字段校验、接口错误、遮罩/关闭按钮/Escape、权限态与响应式。使用 DOM 快照、截图、控制台日志和关键请求结果记录事实；不可见行为不推测。

- [ ] **Step 4: 写中文结果与验证报告**

报告列出提交、文件范围、数据来源、所有 PASS/FAIL/SKIP、MariaDB DSN、容器/卷、三视口事实、剩余外部凭证阻塞和 0154→0153 回滚点。

- [ ] **Step 5: 最终验证后提交报告**

Run: `git diff --check && git status --short --branch && git log --oneline origin/main..HEAD`

Commit: `docs: report P0 live code integration verification`
