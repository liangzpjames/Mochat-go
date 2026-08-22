# 员工移动端参考图高还原实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**目标：** 把员工 Sidebar 重构为参考图风格的“客户 / 会话 / 我的”三工作区，并补充当前员工范围内的真实概览、联系人列表和任务列表 API。

**架构：** Go 端新增一个只接受 Sidebar 员工 JWT 的工作台纵切面，handler 从 token 派生员工和企业，store 在 SQL 中同时绑定 `employee_id` 与 `corp_id`。React 端新增独立工作区 API/client、页面组件和原创 SVG 图标，根路径使用 `tab` 查询参数切换；既有详情路由和写入契约保持不变。

**技术栈：** Go `net/http`、MariaDB/MySQL、React 19、TypeScript、React Router、Vitest/Testing Library、Playwright、pnpm、Docker Compose。

## 全局约束

- 只修改 `web/apps/sidebar`、必要的 `web/packages/mobile-foundation`、对应测试、部署验收夹具及 Sidebar 实际调用的 Go/API。
- 不修改 Operation/Dashboard/SaaS Admin UI，不修改 `docs/PROJECT_PROGRESS.zh-CN.md`。
- 不实现或伪造会话存档、敏感词、风险行为、订单、商机、客服在线状态或 Phase 7。
- 不相信客户端传入的 `employeeId`/`corpId`；所有新 API 均从 Sidebar 员工 JWT 派生身份。
- 不把 Review Server 固定夹具打入生产 bundle；生产数据只来自持久化 API。
- 保留 12 条路径、`/sidebar-app` 前缀、现有 OAuth/JSSDK 安全语义和全部 Docker 命名卷。
- 每个生产行为先运行对应失败测试，再写最小实现；失败时先执行系统化根因分析。

---

### 任务 1：定义员工工作台领域合同与权限测试

**文件：**
- 新建：`internal/dashboard/sidebar_workbench.go`
- 新建：`internal/dashboard/sidebar_workbench_test.go`
- 修改：`internal/dashboard/contact_field.go`

**接口：**
- 复用：`SidebarEmployeeByID(context.Context, int) (SidebarEmployee, bool, error)` 与 Sidebar employee resolver。
- 产出：`SidebarWorkbenchStore`、`SidebarWorkbenchSummary`、`SidebarContactPage`、`SidebarTaskPage` 和三个 handler 方法。

- [ ] **步骤 1：先写 handler 失败测试**

测试必须断言：没有员工身份返回 `401`；不存在员工返回 `404`；summary 调用 store 时只传已解析的 `SidebarEmployee`；列表参数限制 `perPage <= 50`；非法 `kind/state/page` 返回 `422`。

```go
func TestSidebarWorkbenchSummaryUsesResolvedEmployee(t *testing.T) {
    store := &fakeSidebarWorkbenchStore{
        employee: SidebarEmployee{ID: 7, CorpID: 9, LogUserID: 3},
        summary: SidebarWorkbenchSummary{Employee: SidebarEmployeeProfile{ID: 7, Name: "员工甲"}},
    }
    handler := NewSidebarWorkbenchHandler(store, HeaderUserIDResolver{HeaderName: "X-Mochat-Go-Employee-ID"})
    req := httptest.NewRequest(http.MethodGet, "/sidebar/workbench/summary", nil)
    req.Header.Set("X-Mochat-Go-Employee-ID", "7")
    rec := httptest.NewRecorder()
    handler.Summary(rec, req)
    if rec.Code != http.StatusOK || store.summaryEmployee.ID != 7 || store.summaryEmployee.CorpID != 9 {
        t.Fatalf("code=%d employee=%+v", rec.Code, store.summaryEmployee)
    }
}
```

- [ ] **步骤 2：运行并确认 RED**

运行：`go test ./internal/dashboard -run 'TestSidebarWorkbench' -count=1`

预期：因 `NewSidebarWorkbenchHandler` 与领域类型尚不存在而编译失败。

- [ ] **步骤 3：实现最小 handler 与类型**

```go
type SidebarWorkbenchStore interface {
    SidebarEmployeeByID(context.Context, int) (SidebarEmployee, bool, error)
    SidebarWorkbenchSummary(context.Context, SidebarEmployee) (SidebarWorkbenchSummary, error)
    SidebarContacts(context.Context, SidebarEmployee, SidebarContactFilter) (SidebarContactPage, error)
    SidebarTasks(context.Context, SidebarEmployee, SidebarTaskFilter) (SidebarTaskPage, error)
}

type SidebarContactFilter struct { Keyword string; Page, PerPage int }
type SidebarTaskFilter struct { Kind, State string; Page, PerPage int }
```

三个方法统一调用私有 `resolveEmployee`；只接受 `GET`，解析参数后调用 store，并通过现有 `writeEnvelope` 返回数据。

- [ ] **步骤 4：运行 GREEN 与 dashboard 回归**

运行：

```powershell
go test ./internal/dashboard -run 'TestSidebarWorkbench' -count=1
go test ./internal/dashboard -count=1
```

预期：新测试和 dashboard 包全绿。

- [ ] **步骤 5：提交**

```powershell
git add internal/dashboard/sidebar_workbench.go internal/dashboard/sidebar_workbench_test.go internal/dashboard/contact_field.go
git commit -m "feat(sidebar): define employee workbench contracts"
```

### 任务 2：实现员工范围 SQL 聚合、联系人和任务列表

**文件：**
- 新建：`internal/store/sidebar_workbench.go`
- 新建：`internal/store/sidebar_workbench_test.go`
- 修改：`internal/store/mysql.go`

**接口：**
- 消费：任务 1 的 `SidebarWorkbenchStore` 类型。
- 产出：`MySQLStore.SidebarWorkbenchSummary`、`SidebarContacts`、`SidebarTasks`。

- [ ] **步骤 1：先写 SQL 失败测试**

使用 `sqlmock` 锁定每条查询都含员工、企业和删除范围。联系人测试还必须断言关键词参数化、总数与分页共用同一范围；任务列表三个 kind 分别验证真实日志/分配表。

```go
func TestSidebarContactsBindsEmployeeAndCorp(t *testing.T) {
    mock.ExpectQuery(`(?s)FROM mc_work_contact_employee AS rel.*JOIN mc_work_contact AS contact.*rel.employee_id = \?.*rel.corp_id = \?.*contact.corp_id = rel.corp_id.*rel.deleted_at IS NULL`).
        WithArgs(7, 9, "%客户%", "%客户%").
        WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(0))
    page, err := store.SidebarContacts(context.Background(), dashboard.SidebarEmployee{ID: 7, CorpID: 9}, dashboard.SidebarContactFilter{Keyword: "客户", Page: 1, PerPage: 20})
    if err != nil || page.Total != 0 { t.Fatalf("page=%+v err=%v", page, err) }
}
```

- [ ] **步骤 2：运行并确认 RED**

运行：`go test ./internal/store -run 'TestSidebarWorkbench|TestSidebarContacts|TestSidebarTasks' -count=1`

预期：MySQLStore 尚未实现新增接口。

- [ ] **步骤 3：实现最小 SQL**

summary 分别读取员工/企业/部门、客户关系聚合、标签客户去重、员工负责群、三类待办数量；联系人先 count 后 page query；任务按 kind 选择白名单 SQL，禁止拼接客户端列名。

```go
func (s *MySQLStore) SidebarContacts(ctx context.Context, employee dashboard.SidebarEmployee, filter dashboard.SidebarContactFilter) (dashboard.SidebarContactPage, error) {
    keyword := "%" + strings.TrimSpace(filter.Keyword) + "%"
    // count 和 list 都绑定 rel.employee_id、rel.corp_id、contact.corp_id、deleted_at。
    // LIMIT/OFFSET 来自 handler 已验证的整数。
}
```

- [ ] **步骤 4：运行 GREEN、真实 Schema 和安全回归**

运行：

```powershell
go test ./internal/store -run 'TestSidebarWorkbench|TestSidebarContacts|TestSidebarTasks|TestSidebarEmployeeSecurity' -count=1
go test ./internal/store -count=1
```

再在保留卷的 MariaDB 容器中对新增查询执行 `EXPLAIN`，预期无未知表/列错误。

- [ ] **步骤 5：提交**

```powershell
git add internal/store/sidebar_workbench.go internal/store/sidebar_workbench_test.go internal/store/mysql.go
git commit -m "feat(sidebar): query employee workbench data"
```

### 任务 3：注册 API 路由与前端契约解析

**文件：**
- 修改：`internal/server/server.go`
- 修改：`internal/server/server_test.go`
- 修改：`internal/server/compat_manifest_embedded.json`
- 新建：`web/apps/sidebar/src/features/workbench/workbench-api.ts`
- 新建：`web/apps/sidebar/src/features/workbench/workbench-api.test.ts`

**接口：**
- 消费：任务 1、2 的三个 handler/store 方法。
- 产出：`loadWorkbenchSummary`、`loadEmployeeContacts`、`loadEmployeeTasks`。

- [ ] **步骤 1：先写路由和解析失败测试**

```ts
it('rejects a malformed summary instead of rendering fake zeroes', async () => {
  const request = vi.fn().mockResolvedValue({ employee: null, customers: {}, tasks: {} });
  await expect(loadWorkbenchSummary(request)).rejects.toMatchObject({ kind: 'validation' });
});
```

Server 测试断言三条新路径只接受 GET，并出现在 migrated routes。

- [ ] **步骤 2：运行并确认 RED**

```powershell
go test ./internal/server -run 'SidebarWorkbench' -count=1
corepack pnpm --filter @mochat/sidebar test -- workbench-api.test.ts
```

预期：路由和前端函数尚不存在。

- [ ] **步骤 3：实现路由和严格解析器**

```ts
export type WorkbenchSummary = {
  employee: { id: number; name: string; avatar: string | null; departmentNames: string[]; corpName: string };
  customers: { total: number; addedToday: number; taggedTotal: number; ownedRoomTotal: number };
  tasks: { contactSopPending: number; roomSopPending: number; batchAddPending: number };
};

export function loadWorkbenchSummary(request: BusinessRequest): Promise<WorkbenchSummary>;
export function loadEmployeeContacts(request: BusinessRequest, filter: ContactListFilter): Promise<ContactListPage>;
export function loadEmployeeTasks(request: BusinessRequest, filter: TaskListFilter): Promise<TaskListPage>;
```

解析器拒绝负数、非整数、缺字段和未知 task kind；不得用 `?? 0` 吞掉缺失字段。

- [ ] **步骤 4：运行 GREEN**

```powershell
go test ./internal/server -count=1
corepack pnpm --filter @mochat/sidebar test -- workbench-api.test.ts
```

- [ ] **步骤 5：提交**

```powershell
git add internal/server web/apps/sidebar/src/features/workbench/workbench-api.ts web/apps/sidebar/src/features/workbench/workbench-api.test.ts
git commit -m "feat(sidebar): expose employee workbench APIs"
```

### 任务 4：重建三工作区和参考图视觉系统

**文件：**
- 新建：`web/apps/sidebar/src/features/workbench/workbench-page.tsx`
- 新建：`web/apps/sidebar/src/features/workbench/workbench-page.test.tsx`
- 新建：`web/apps/sidebar/src/features/workbench/workbench-icons.tsx`
- 新建：`web/apps/sidebar/src/features/workbench/contact-list.tsx`
- 修改：`web/apps/sidebar/src/app/sidebar-router.tsx`
- 修改：`web/apps/sidebar/src/ui/sidebar-page-shell.tsx`
- 修改：`web/apps/sidebar/src/ui/sidebar-page-shell.test.tsx`
- 修改：`web/apps/sidebar/src/styles.css`

**接口：**
- 消费：任务 3 的 API client、现有 `WeComBridge` 和安全内部导航函数。
- 产出：`SidebarWorkbenchPage`，支持 `tab=customers|conversations|profile`。

- [ ] **步骤 1：先写工作区失败测试**

测试覆盖：缺省客户页；三栏始终存在；tab 写入 URL；统计错误不显示零；联系人搜索/分页；长名称；未接入 tab 的诚实说明；无上下文不暴露伪详情链接。

```tsx
it('switches among three real workspaces through the tab query', async () => {
  renderWorkbench('/?tab=profile');
  expect(await screen.findByRole('heading', { name: '我的' })).not.toBeNull();
  await userEvent.click(screen.getByRole('link', { name: '客户' }));
  expect(currentLocation()).toContain('tab=customers');
});

it('keeps API errors distinct from a real zero', async () => {
  request.mockRejectedValue(new MobileApiError('server', '服务暂不可用'));
  renderWorkbench('/?tab=customers');
  expect(await screen.findByText('服务暂不可用')).not.toBeNull();
  expect(screen.queryByText('0')).toBeNull();
});
```

- [ ] **步骤 2：运行并确认 RED**

运行：`corepack pnpm --filter @mochat/sidebar test -- workbench-page.test.tsx sidebar-page-shell.test.tsx`

预期：三工作区组件和固定三栏行为不存在。

- [ ] **步骤 3：实现最小页面与原创 SVG**

客户区包含横幅、四张真实统计卡、五个快捷入口、待办摘要和联系人列表；会话区使用三类任务；我的区使用员工资料与真实设置项。SVG 使用稳定 `viewBox`、渐变 id 前缀和 `aria-hidden`，图片信息由相邻文本承担。

```tsx
const tab = parseWorkspaceTab(searchParams.get('tab'));
return (
  <SidebarWorkspaceShell current={tab}>
    {tab === 'customers' ? <CustomerWorkspace /> : null}
    {tab === 'conversations' ? <ConversationWorkspace /> : null}
    {tab === 'profile' ? <ProfileWorkspace /> : null}
  </SidebarWorkspaceShell>
);
```

- [ ] **步骤 4：实现参考图布局 CSS 并运行 GREEN**

CSS 必须包含：浅蓝背景、插画横幅、白色大面板、两列 metric grid、彩色 shortcut grid、固定底栏、FAB、安全区、320px 单列降级和横屏视觉视口处理。

运行：

```powershell
corepack pnpm --filter @mochat/sidebar test -- workbench-page.test.tsx sidebar-page-shell.test.tsx sidebar-router.test.tsx
corepack pnpm --filter @mochat/sidebar typecheck
corepack pnpm --filter @mochat/sidebar lint
```

- [ ] **步骤 5：提交**

```powershell
git add web/apps/sidebar/src/features/workbench web/apps/sidebar/src/app/sidebar-router.tsx web/apps/sidebar/src/ui web/apps/sidebar/src/styles.css
git commit -m "feat(sidebar): recreate employee mobile workspaces"
```

### 任务 5：统一既有业务页的高还原视觉

**文件：**
- 修改：`web/apps/sidebar/src/features/contact/contact-page.tsx`
- 修改：`web/apps/sidebar/src/features/contact/contact-page.test.tsx`
- 修改：`web/apps/sidebar/src/features/contact/contact-edit-page.tsx`
- 修改：`web/apps/sidebar/src/features/contact/contact-remark-page.tsx`
- 修改：`web/apps/sidebar/src/features/contact/contact-tag-page.tsx`
- 修改：`web/apps/sidebar/src/features/contact/contact-write-pages.test.tsx`
- 修改：`web/apps/sidebar/src/features/business-pages.tsx`
- 修改：`web/apps/sidebar/src/features/business-pages.test.tsx`
- 修改：`web/apps/sidebar/src/styles.css`

**接口：**
- 保持所有既有 API 与写入语义不变。
- 产出：客户、表单、标签、素材、三类任务详情与参考图一致的面板、列表和操作区。

- [ ] **步骤 1：先写失败的结构与交互测试**

断言客户详情有资料头、彩色快捷动作和分组列表；SOP/批量页拥有对象摘要与列表行；素材使用四列/两列响应式图标卡；保存、取消、校验和重复提交次数保持原合同。

- [ ] **步骤 2：运行并确认 RED**

运行：

```powershell
corepack pnpm --filter @mochat/sidebar test -- contact-page.test.tsx contact-write-pages.test.tsx business-pages.test.tsx
```

预期：新结构类名和可访问分区不存在，原行为测试仍为绿。

- [ ] **步骤 3：最小重构 JSX/CSS**

只改变结构表达和视觉，不改变请求路径、参数、持久化顺序、错误语义或 JSSDK 调用。每个页面保留独立信息层级，不复用通用占位卡。

- [ ] **步骤 4：运行 GREEN 与 Sidebar 全量测试**

```powershell
corepack pnpm --filter @mochat/sidebar test
corepack pnpm --filter @mochat/sidebar typecheck
corepack pnpm --filter @mochat/sidebar lint
corepack pnpm --filter @mochat/sidebar build
```

- [ ] **步骤 5：提交**

```powershell
git add web/apps/sidebar/src/features web/apps/sidebar/src/styles.css
git commit -m "style(sidebar): align business pages with mobile references"
```

### 任务 6：扩充本地验收夹具和浏览器合同

**文件：**
- 修改：`web/e2e/fixtures/sidebar-employee-review.json`
- 修改：`scripts/sidebar_review_server.mjs`
- 修改：`scripts/sidebar_review_server.test.mjs`
- 修改：`web/e2e/tests/sidebar-employee-mobile.spec.ts`
- 修改：`web/e2e/tests/mobile-clients-foundation.spec.ts`
- 修改：`web/e2e/tests/sidebar-review-login.spec.ts`

**接口：**
- Review Server 新增三个固定、可重置、明确标注为验收数据的只读 GET 接口。
- E2E 继续使用真实生产 bundle 与 HTTP API 形态。

- [ ] **步骤 1：先写 Review Server 与 E2E 失败测试**

测试断言新接口要求固定 Bearer、拒绝跨 agent、未知参数失败关闭；浏览器断言三工作区可切换、统计来自网络响应、联系人可搜索、三栏固定、无横向溢出。

- [ ] **步骤 2：运行并确认 RED**

```powershell
node --test scripts/sidebar_review_server.test.mjs
$env:MOCHAT_E2E_BASE_URL='http://127.0.0.1:28083'; corepack pnpm --filter @mochat/e2e exec playwright test tests/sidebar-employee-mobile.spec.ts --workers=1
```

预期：夹具接口返回 404，工作区定位器不存在。

- [ ] **步骤 3：实现夹具路由并刷新截图合同**

固定数据必须出现在 `sidebar-employee-review.json`，Review Server 只读取/复制它；页面继续显示“本地验收数据 · 重启后重置”。截图文件名按四张参考图映射更新，不删除旧证据直到新证据通过人工查看。

- [ ] **步骤 4：运行 GREEN**

```powershell
node --test scripts/sidebar_review_server.test.mjs scripts/check_standalone_public_urls.test.mjs
corepack pnpm --filter @mochat/e2e typecheck
corepack pnpm --filter @mochat/e2e lint
```

- [ ] **步骤 5：提交**

```powershell
git add scripts web/e2e
git commit -m "test(sidebar): cover high fidelity mobile workspaces"
```

### 任务 7：完整验证、视觉复验和交付报告

**文件：**
- 修改：`docs/reviews/2026-08-22-employee-sidebar-mobile-acceptance.zh-CN.md`
- 更新：`docs/reviews/evidence/employee-sidebar-mobile/*.png`

**接口：** 无新增生产接口；仅记录最终证据。

- [ ] **步骤 1：运行全部工程门禁**

```powershell
corepack pnpm --filter @mochat/sidebar test
corepack pnpm --filter @mochat/sidebar typecheck
corepack pnpm --filter @mochat/sidebar lint
corepack pnpm --filter @mochat/sidebar build
corepack pnpm --filter @mochat/mobile-foundation test
corepack pnpm --filter @mochat/mobile-foundation typecheck
corepack pnpm --filter @mochat/mobile-foundation lint
corepack pnpm --filter @mochat/mobile-foundation build
corepack pnpm check:mobile-clients-foundation
corepack pnpm --filter @mochat/operation test
go test ./... -count=1
git diff --check
```

预期：全部退出码为 0；任何失败先定位根因并补失败测试后修复。

- [ ] **步骤 2：只重建 Docker app 并检查卷**

保留项目 `mochat-sidebar-mobile-acceptance` 的 MySQL、Redis 和四个命名卷，只执行 `docker compose ... up -d --build --no-deps app`。确认 app、MySQL、Redis healthy，`/healthz` 为 200。

- [ ] **步骤 3：运行最终浏览器验收**

```powershell
$env:MOCHAT_E2E_BASE_URL='http://127.0.0.1:28080'
corepack pnpm --filter @mochat/e2e exec playwright test tests/mobile-clients-foundation.spec.ts tests/sidebar-employee-mobile.spec.ts --workers=1
$env:MOCHAT_E2E_BASE_URL='http://127.0.0.1:28083'
corepack pnpm --filter @mochat/e2e exec playwright test tests/sidebar-review-login.spec.ts --workers=1
```

使用 360×800、390×844、430×932、320×568、844×390 逐图查看；检查控制台、未处理 Promise、横向溢出、底栏/FAB/键盘遮挡和 44px 触控区。

- [ ] **步骤 4：请求独立代码与安全复审**

提供设计文档、计划、base SHA、HEAD SHA 和 diff。Critical/Important 必须修复并重新运行相关门禁；Minor 明确记录或修复。

- [ ] **步骤 5：更新中文验收报告并提交**

报告必须包含 API 数据来源、逐路由状态、PASS/FAIL/SKIP、截图差异、Docker 卷、真实企微外部 SKIP、分支/提交/回滚点和 `origin/main` 推进风险。

```powershell
git add docs/reviews
git commit -m "docs: verify high fidelity employee sidebar"
git status --short --branch
git diff --check b9a47cab45ec872bc81e61a20b06a8a3529311b2..HEAD
git diff --exit-code b9a47cab45ec872bc81e61a20b06a8a3529311b2..HEAD -- docs/PROJECT_PROGRESS.zh-CN.md
```

预期：工作树干净、监督台账无差异、不合并或推送 main。
