# 员工会话圆弧式工作台 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. Subagent execution remains disabled unless the user explicitly reauthorizes it.

**Goal:** 把 `/chat/v2-staff` 改造成使用真实组织与归档数据的连续三栏员工会话工作台。

**Architecture:** 后端在现有会话权限域中增加 `staffDirectory` 和 `staffDetail` 两个只读契约，复用员工、部门、归档、风险/超时和个人关注数据。前端把现有单文件页面拆成目录、会话列表、详情三个面板，由页面组件统一维护 URL 状态；桌面三栏各自滚动，窄屏使用目录抽屉或分步单栏。

**Tech Stack:** Go 1.24、MariaDB 10.6、React 19、TypeScript 5.9、TanStack Query、React Router、Vitest、Testing Library、CSS、Docker Compose

**Design:** `docs/superpowers/specs/2026-08-19-employee-conversation-yuanhu-workspace-design.md`

## Global Constraints

- 只改造 `/chat/v2-staff`，不联动重构其他会话页面。
- 不修改菜单导航、应用顶部 banner、企业切换和全局搜索。
- 所有数字、人员、组织、消息和状态必须来自 MoChat 真实接口；缺口显示不可用原因，禁止静态业务假数据。
- 会话列表每页固定 20 条；员工目录每批固定 50 人；消息每批固定 50 条。
- `conversationId` 使用 `employeeId:toUserType:toUserId`，列表、详情与关注保持一致。
- 内部群显示禁用告警；本页不新增不完整的会话下载。
- Docker 仅重建必要的 `app` 服务，不删除或重建 MySQL、Redis 和命名卷，继续使用 D 盘路径。
- 遵循 TDD：每项行为先运行失败测试，再写最小实现。

---

## 文件结构

**新增：**

- `internal/dashboard/work_message_staff.go`：员工目录/详情领域类型、参数解析和 HTTP 处理器。
- `internal/dashboard/work_message_staff_test.go`：处理器权限、参数、错误和响应契约测试。
- `internal/store/work_message_staff.go`：目录聚合、会话统计和游标消息查询。
- `internal/store/work_message_staff_test.go`：SQL 构造和 MariaDB 集成测试。
- `web/apps/dashboard/src/features/conversation-global/employee-conversation-directory.tsx`：左栏。
- `web/apps/dashboard/src/features/conversation-global/employee-conversation-list.tsx`：中栏。
- `web/apps/dashboard/src/features/conversation-global/employee-conversation-detail.tsx`：右栏。
- `web/apps/dashboard/src/features/conversation-global/conversation-message-content.tsx`：消息内容类型化渲染。
- `web/apps/dashboard/src/styles/employee-conversation-layout.test.ts`：宽屏、滚动和响应式 CSS 契约。

**修改：**

- `internal/server/server.go`：注册两个 GET 路由和 readiness 路由清单。
- `cmd/mochat-go/main.go`：装配两个新处理器。
- `internal/dashboard/auto_tag_dashboard_test.go`：扩展已有测试 store 对 staff 接口的支持。
- `web/apps/dashboard/src/features/conversation-global/conversation-global-api.ts`：新增目录、详情、游标与严格解析。
- `web/apps/dashboard/src/features/conversation-global/conversation-global-api.test.ts`：接口序列化与响应校验。
- `web/apps/dashboard/src/features/conversation-global/employee-conversation-page.tsx`：仅保留状态编排和三栏组合。
- `web/apps/dashboard/src/features/conversation-global/employee-conversation-page.test.tsx`：跨栏与 URL 行为。
- `web/apps/dashboard/src/styles/index.css`：替换员工会话旧三卡片样式。
- `docs/PROJECT_PROGRESS.zh-CN.md`：实现完成后记录能力、缺口和验收证据。

---

### Task 1: 员工目录领域契约与处理器

**Files:**

- Create: `internal/dashboard/work_message_staff.go`
- Create: `internal/dashboard/work_message_staff_test.go`
- Modify: `internal/dashboard/auto_tag_dashboard_test.go`

**Interfaces:**

- Produces: `WorkMessageStaffStore.StaffDirectory(context.Context, WorkMessageStaffDirectoryFilter) (WorkMessageStaffDirectoryPage, error)`
- Produces: `AutoTagHandler.WorkMessageStaffDirectory(http.ResponseWriter, *http.Request)`
- Consumes: `workMessageConversationPermissionKey`、`principalCorpID`、`workMessageArchiveAllowed`

- [ ] **Step 1: 写失败的处理器测试**

在 `work_message_staff_test.go` 增加：

```go
func TestWorkMessageStaffDirectoryUsesPrincipalAndEmployeeScope(t *testing.T) {
    store := &fakeAutoTagStore{user: User{ID: 42, TenantID: 1}}
    store.staffDirectory = WorkMessageStaffDirectoryPage{
        Employees: []WorkMessageStaffEmployee{{ID: 9, Name: "张伟", Status: 1, Archived: true}},
        Counts: WorkMessageStaffCounts{All: 1, Archived: 1}, Page: 1, PageSize: 50, Total: 1,
    }
    authorizer := &recordingAuthorizer{accessSet: true, access: AccessContext{
        CorpID: 7, WorkEmployeeID: 9, DataPermission: DataPermissionDepartment, DeptEmployeeIDs: []int{9, 10},
    }}
    handler := NewAutoTagHandler(store, staticAdminCache("7-9"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, authorizer)
    req := httptest.NewRequest(http.MethodGet, "/dashboard/workMessage/staffDirectory?mode=focused&keyword=张&departmentId=10&page=2&pageSize=50", nil)
    req.Header.Set("X-Mochat-Go-User-ID", "42")
    req = withAutoTagTestPrincipal(req, 42, 1, 7)
    rec := httptest.NewRecorder()

    handler.WorkMessageStaffDirectory(rec, req)

    if rec.Code != http.StatusOK { t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String()) }
    if got := store.staffDirectoryFilter; got.TenantID != 1 || got.CorpID != 7 || got.UserID != 42 ||
        got.Mode != WorkMessageStaffModeFocused || got.DepartmentID != 10 || got.Page != 2 || got.PageSize != 50 ||
        !got.RestrictEmployeeIDs || !reflect.DeepEqual(got.EmployeeIDs, []int{9, 10}) {
        t.Fatalf("filter=%#v", got)
    }
}
```

同时覆盖非法 `mode`、`departmentId`、`pageSize != 50` 返回 400，以及归档未开通返回 40301。

- [ ] **Step 2: 运行测试确认 RED**

Run: `go test ./internal/dashboard -run 'TestWorkMessageStaffDirectory' -count=1`

Expected: FAIL，错误指向缺少 `WorkMessageStaffDirectoryPage`、store 方法或处理器。

- [ ] **Step 3: 定义最小领域类型和参数解析**

在 `work_message_staff.go` 定义：

```go
type WorkMessageStaffMode string
const (
    WorkMessageStaffModeAll WorkMessageStaffMode = "all"
    WorkMessageStaffModeFocused WorkMessageStaffMode = "focused"
    WorkMessageStaffModeArchived WorkMessageStaffMode = "archived"
    WorkMessageStaffModeDeparted WorkMessageStaffMode = "departed"
)

type WorkMessageStaffDirectoryFilter struct {
    TenantID, CorpID, UserID int
    Mode WorkMessageStaffMode
    Keyword string
    DepartmentID, Page, PageSize int
    RestrictEmployeeIDs bool
    EmployeeIDs []int
}
type WorkMessageStaffDepartment struct { ID, ParentID int; Name string; EmployeeCount int; Children []WorkMessageStaffDepartment }
type WorkMessageStaffEmployee struct {
    ID int; Name, Avatar string; Status int; DepartmentIDs []int; Archived bool
    ConversationCount, FocusedConversationCount int; LastConversationAt string
}
type WorkMessageStaffCounts struct { All, Focused, Archived, Departed int }
type WorkMessageStaffLimitation struct { Key, Reason string }
type WorkMessageStaffDirectoryPage struct {
    Departments []WorkMessageStaffDepartment; Employees []WorkMessageStaffEmployee
    Counts WorkMessageStaffCounts; Page, PageSize, Total int; Limitations []WorkMessageStaffLimitation
    Capabilities []WorkMessageCapability
}
type WorkMessageStaffStore interface {
    StaffDirectory(context.Context, WorkMessageStaffDirectoryFilter) (WorkMessageStaffDirectoryPage, error)
}
```

`WorkMessageStaffDirectory` 必须调用 `resolveAuthorized(..., workMessageConversationPermissionKey)`，将非全量权限转成 `RestrictEmployeeIDs=true`，并只接受固定 `pageSize=50`。

- [ ] **Step 4: 运行处理器测试确认 GREEN**

Run: `go test ./internal/dashboard -run 'TestWorkMessageStaffDirectory' -count=1`

Expected: PASS。

- [ ] **Step 5: 提交**

```powershell
git add internal/dashboard/work_message_staff.go internal/dashboard/work_message_staff_test.go internal/dashboard/auto_tag_dashboard_test.go
git commit -m "feat: define employee conversation directory contract"
```

---

### Task 2: 真实组织、员工与重点关注目录聚合

**Files:**

- Create: `internal/store/work_message_staff.go`
- Create: `internal/store/work_message_staff_test.go`

**Interfaces:**

- Consumes: `WorkMessageStaffDirectoryFilter`
- Produces: `MySQLStore.StaffDirectory`
- Data: `mc_work_employee`、`mc_work_department`、`mc_work_employee_department`、当前有效归档来源、`mochat_go_work_message_focus`

- [ ] **Step 1: 写失败的 SQL/集成测试**

覆盖以下固定断言：

```go
func TestStaffDirectoryWhereUsesPermissionDepartmentAndMode(t *testing.T) {
    where, args := staffDirectoryWhere(dashboard.WorkMessageStaffDirectoryFilter{
        TenantID: 1, CorpID: 7, UserID: 42, Mode: dashboard.WorkMessageStaffModeFocused,
        Keyword: "张%", DepartmentID: 10, RestrictEmployeeIDs: true, EmployeeIDs: []int{9, 10},
    })
    for _, fragment := range []string{"e.corp_id=?", "e.id IN (?,?)", "e.name LIKE ? ESCAPE", "wed.department_id=?", "focus.user_id=?"} {
        if !strings.Contains(where, fragment) { t.Fatalf("missing %q in %s", fragment, where) }
    }
    if len(args) == 0 { t.Fatal("expected scoped args") }
}
```

MariaDB 集成 fixture 必须证明：

- 无权限员工不会进入列表、部门计数或四项 counts。
- `archived` 只统计当前有效归档来源中出现过消息的员工。
- `focused` 按 tenant/corp/current user 聚合，不泄露其他用户关注。
- `departed` 只匹配 `status=5`。
- 关键词 `%`、`_` 被转义为字面字符。

- [ ] **Step 2: 运行测试确认 RED**

Run: `go test ./internal/store -run 'TestStaffDirectory' -count=1`

Expected: FAIL，缺少 `staffDirectoryWhere` 和 `StaffDirectory`。

- [ ] **Step 3: 实现分页员工查询与计数**

实现顺序固定为：

1. 根据 `workMessageArchiveMode` 与来源注册表确定有效归档 union。
2. 用 CTE/派生表按员工聚合 `conversation_count`、`last_conversation_at` 和 `archived`。
3. 用 `mochat_go_work_message_focus` 按 `tenant_id/corp_id/user_id` 聚合 `focused_conversation_count`。
4. 应用 corp、权限员工、部门、状态、关键词和 mode 条件。
5. 单独计算 `all/focused/archived/departed`，四项都使用相同权限边界。
6. `LIMIT 50 OFFSET (page-1)*50`，并返回真实 total。
7. 返回 `internalGroup` capability；当前值为 unavailable，原因来自后端契约。

关键词必须复用项目的 LIKE 转义方法；不能直接拼 SQL。若部门表为空，返回员工和 limitation：

```go
dashboard.WorkMessageStaffLimitation{Key: "departments", Reason: "组织架构未同步，仅展示可访问员工"}
```

- [ ] **Step 4: 构建按权限裁剪的部门树**

读取可见员工对应的 `mc_work_employee_department`，仅保留有可见员工的部门及其祖先；每个节点 `employeeCount` 为该节点及后代可见员工去重数。树按现有部门顺序稳定排序。

- [ ] **Step 5: 运行 store 测试确认 GREEN**

Run: `go test ./internal/store -run 'TestStaffDirectory' -count=1`

Expected: PASS；若本地 MariaDB fixture 未启用，纯 SQL 测试通过且集成测试明确 SKIP，不得写成已通过。

- [ ] **Step 6: 提交**

```powershell
git add internal/store/work_message_staff.go internal/store/work_message_staff_test.go
git commit -m "feat: aggregate scoped employee conversation directory"
```

---

### Task 3: 员工会话详情筛选、统计与游标

**Files:**

- Modify: `internal/dashboard/work_message_staff.go`
- Modify: `internal/dashboard/work_message_staff_test.go`
- Modify: `internal/store/work_message_staff.go`
- Modify: `internal/store/work_message_staff_test.go`

**Interfaces:**

- Produces: `WorkMessageStaffStore.StaffDetail(context.Context, WorkMessageStaffDetailFilter) (WorkMessageStaffDetail, error)`
- Produces: `AutoTagHandler.WorkMessageStaffDetail`
- Query: `conversationId`、`keyword`、重复 `messageTypes`、`date`、`pageSize=50`、opaque `before`

- [ ] **Step 1: 写失败的详情处理器测试**

测试必须断言 `conversationId=9:1:31` 被解析为员工 9、客户类型 1、对象 31；权限集合不含 9 时返回 404；非法类型、日期、游标和页大小返回 400；归档未开通返回 40301。

成功响应断言：

```go
if data.Stats.CommunicationDays != 2 || data.Stats.MessageTotal != 15 ||
    data.Stats.InboundTotal != 5 || data.Stats.OutboundTotal != 10 || !data.HasMore {
    t.Fatalf("detail=%#v", data)
}
```

- [ ] **Step 2: 写失败的 store 测试**

fixture 包含同一会话跨两天、双向、三种消息类型的数据。断言：

- stats 基于完整会话，不受关键词/类型/日期筛选影响。
- message rows 同时应用 `keyword + messageTypes + date`。
- 第一页返回最新 50 条但响应内按时间正序；`before` 只读取更早消息。
- 游标包含时间、来源表和稳定消息 ID，并通过服务端签名/编码后作为 opaque string 返回。
- 同时间消息按稳定 ID 排序，无重复、无遗漏。

- [ ] **Step 3: 运行测试确认 RED**

Run: `go test ./internal/dashboard ./internal/store -run 'TestWorkMessageStaffDetail|TestStaffDetail' -count=1`

Expected: FAIL，缺少详情类型、处理器和查询。

- [ ] **Step 4: 定义详情契约并实现查询**

核心类型：

```go
type WorkMessageStaffDetailFilter struct {
    TenantID, CorpID, UserID, EmployeeID, ToUserType, ToUserID int
    Keyword, Date, Before string
    MessageTypes, EmployeeIDs []int
    RestrictEmployeeIDs bool
    PageSize int
}
type WorkMessageStaffStats struct { CommunicationDays, MessageTotal, InboundTotal, OutboundTotal int }
type WorkMessageStaffMessage struct {
    ID, SenderName, SenderAvatar, Direction, SentAt, ArchiveSource, ArchiveSourceID string
    Type int
    Content any
}
type WorkMessageStaffDetail struct {
    ConversationID string
    EmployeeID, TargetID int
    EmployeeName, TargetType, TargetName string
    Focused bool
    Stats WorkMessageStaffStats
    Messages []WorkMessageStaffMessage
    NextBefore string
    HasMore bool
    Capabilities []WorkMessageCapability
}
```

统计 SQL 使用 `COUNT(DISTINCT DATE(msg_data_time))`、`COUNT(*)` 和方向条件聚合；消息查询使用当前归档来源 union 和 keyset 条件。`internalGroup` capability 固定 unavailable，原因使用设计文档原文。

- [ ] **Step 5: 运行详情测试确认 GREEN**

Run: `go test ./internal/dashboard ./internal/store -run 'TestWorkMessageStaffDetail|TestStaffDetail' -count=1`

Expected: PASS。

- [ ] **Step 6: 提交**

```powershell
git add internal/dashboard/work_message_staff.go internal/dashboard/work_message_staff_test.go internal/store/work_message_staff.go internal/store/work_message_staff_test.go
git commit -m "feat: add filtered employee conversation detail"
```

---

### Task 4: 路由注册与组合根

**Files:**

- Modify: `internal/server/server.go`
- Modify: `internal/server/server_test.go`
- Modify: `cmd/mochat-go/main.go`

**Interfaces:**

- Produces: `GET /dashboard/workMessage/staffDirectory`
- Produces: `GET /dashboard/workMessage/staffDetail`

- [ ] **Step 1: 写失败的 server 路由测试**

在现有路由表测试中加入：

```go
{method: http.MethodGet, path: "/dashboard/workMessage/staffDirectory", body: "staff directory", route: "GET /dashboard/workMessage/staffDirectory"},
{method: http.MethodGet, path: "/dashboard/workMessage/staffDetail?conversationId=9:1:31", body: "staff detail", route: "GET /dashboard/workMessage/staffDetail"},
```

并断言 readiness 列表包含两个精确路由。

- [ ] **Step 2: 运行测试确认 RED**

Run: `go test ./internal/server -run 'WorkMessageStaff|MountedRoutes' -count=1`

Expected: FAIL，两个路由返回 404 或未出现在清单。

- [ ] **Step 3: 添加两个 handler option 与 switch 分支**

在 `Server` 增加 `workMessageStaffDirectory`、`workMessageStaffDetail`；增加：

```go
func WithWorkMessageStaffDirectoryHandler(h http.Handler) Option { return func(s *Server) { s.workMessageStaffDirectory = h } }
func WithWorkMessageStaffDetailHandler(h http.Handler) Option { return func(s *Server) { s.workMessageStaffDetail = h } }
```

路由仅接受 GET，readiness 清单使用精确字符串。`main.go` 装配 `autoTag.WorkMessageStaffDirectory` 和 `autoTag.WorkMessageStaffDetail`。

- [ ] **Step 4: 运行 server 与组合测试确认 GREEN**

Run: `go test ./internal/server ./cmd/mochat-go -run 'WorkMessageStaff|MountedRoutes' -count=1`

Expected: PASS。

- [ ] **Step 5: 提交**

```powershell
git add internal/server/server.go internal/server/server_test.go cmd/mochat-go/main.go
git commit -m "feat: mount employee conversation workspace endpoints"
```

---

### Task 5: 前端严格 API 契约

**Files:**

- Modify: `web/apps/dashboard/src/features/conversation-global/conversation-global-api.ts`
- Modify: `web/apps/dashboard/src/features/conversation-global/conversation-global-api.test.ts`

**Interfaces:**

- Produces: `ConversationGlobalApi.staffDirectory(input)`
- Produces: `ConversationGlobalApi.staffDetail(input)`
- Reuses: `setFocus(id)`、`removeFocus(id)`、`search(input)`

- [ ] **Step 1: 写失败的序列化与解析测试**

测试精确 URL：

```ts
expect(request).toHaveBeenCalledWith(
  '/workMessage/staffDirectory?mode=focused&keyword=%E5%BC%A0&departmentId=10&page=2&pageSize=50',
);
expect(request).toHaveBeenCalledWith(
  '/workMessage/staffDetail?conversationId=9%3A1%3A31&keyword=%E6%8A%A5%E4%BB%B7&messageTypes=image&messageTypes=file&date=2026-08-16&pageSize=50',
);
```

对缺少 `counts`、非法 status、非法 targetType、非数组 messages、错误 stats 和空 `conversationId` 分别断言 reject。

- [ ] **Step 2: 运行测试确认 RED**

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/features/conversation-global/conversation-global-api.test.ts`

Expected: FAIL，`staffDirectory`、`staffDetail` 或新类型不存在。

- [ ] **Step 3: 定义 TypeScript 类型和解析器**

加入：

```ts
export type StaffDirectoryMode = 'all' | 'focused' | 'archived' | 'departed';
export type StaffDirectoryInput = { mode: StaffDirectoryMode; keyword: string; departmentId: number | null; page: number; pageSize: 50 };
export type StaffDetailInput = { conversationId: string; keyword: string; messageTypes: readonly string[]; date: string; pageSize: 50; before?: string };
export type StaffConversationStats = { communicationDays: number | null; messageTotal: number | null; inboundTotal: number | null; outboundTotal: number | null };
```

所有数字使用 `isFiniteNumber` 校验；数组与 capabilities 逐项解析；后端返回不完整时抛出“员工目录接口返回了无效数据”或“员工会话详情接口返回了无效数据”。不要用默认 0 掩盖缺字段。中栏内部群禁用态必须读取目录响应的 `internalGroup` capability。

- [ ] **Step 4: 运行 API 测试确认 GREEN**

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/features/conversation-global/conversation-global-api.test.ts`

Expected: PASS。

- [ ] **Step 5: 提交**

```powershell
git add web/apps/dashboard/src/features/conversation-global/conversation-global-api.ts web/apps/dashboard/src/features/conversation-global/conversation-global-api.test.ts
git commit -m "feat: add employee conversation workspace api"
```

---

### Task 6: 左栏目录、中栏会话与 URL 编排

**Files:**

- Create: `web/apps/dashboard/src/features/conversation-global/employee-conversation-directory.tsx`
- Create: `web/apps/dashboard/src/features/conversation-global/employee-conversation-list.tsx`
- Modify: `web/apps/dashboard/src/features/conversation-global/employee-conversation-page.tsx`
- Modify: `web/apps/dashboard/src/features/conversation-global/employee-conversation-page.test.tsx`

**Interfaces:**

- Directory emits: `onSelectEmployee(id: number)`、`onModeChange(mode)`、`onDepartmentChange(id | null)`
- List emits: `onSelectConversation(conversationId: string)`、`onTypeChange(type)`、`onPageChange(page)`
- Page owns: URL canonicalization and React Query keys

- [ ] **Step 1: 写失败的页面行为测试**

测试必须覆盖：

```tsx
expect(screen.getByRole('tab', { name: /企业架构/ })).toHaveAttribute('aria-selected', 'true');
expect(screen.getByRole('button', { name: /存档成员/ })).toBeTruthy();
expect(screen.getByRole('button', { name: /内部群/ })).toBeDisabled();
expect(screen.getByRole('navigation', { name: '会话分页' })).toHaveTextContent('共 21 条');
```

交互断言：切换员工清空 `conversationId` 并重置 `page=1&pageSize=20`；切换部门清空员工和会话；加载更多员工请求 `page=2&pageSize=50`；刷新员工不改变 URL；整张会话卡写入稳定 `conversationId`。

- [ ] **Step 2: 运行测试确认 RED**

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/features/conversation-global/employee-conversation-page.test.tsx`

Expected: FAIL，旧页面没有目录模式、部门、稳定 conversationId 或内部群 capability 状态。

- [ ] **Step 3: 实现 URL 解析和级联清理**

在页面文件中定义并单测纯函数：

```ts
const conversationPageSize = 20 as const;
const employeePageSize = 50 as const;
const messagePageSize = 50 as const;

function clearFromEmployee(params: URLSearchParams) {
  return updateSearch(params, { employeeId: undefined, conversationId: undefined, page: 1, pageSize: conversationPageSize });
}
function clearFromConversation(params: URLSearchParams) {
  return updateSearch(params, { conversationId: undefined, page: 1, pageSize: conversationPageSize });
}
```

非法 mode、类型、日期、page 和 pageSize 在首次 effect 中替换为 canonical 值；不能发出非法请求。

- [ ] **Step 4: 实现目录和会话列表组件**

目录组件只渲染 props，不直接读 URL。员工使用真实头像，空头像使用姓名首字；limitations 用紧凑 `role="status"`。会话组件使用 `conversation.id` 作为详情锚点时优先改为 `conversation.conversationId`，缺失稳定 ID 的记录显示不可打开状态而不是回退到假 ID。

- [ ] **Step 5: 运行页面测试确认 GREEN**

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/features/conversation-global/employee-conversation-page.test.tsx`

Expected: PASS。

- [ ] **Step 6: 提交**

```powershell
git add web/apps/dashboard/src/features/conversation-global/employee-conversation-directory.tsx web/apps/dashboard/src/features/conversation-global/employee-conversation-list.tsx web/apps/dashboard/src/features/conversation-global/employee-conversation-page.tsx web/apps/dashboard/src/features/conversation-global/employee-conversation-page.test.tsx
git commit -m "feat: build employee conversation directory and trajectory panes"
```

---

### Task 7: 详情筛选、统计、消息渲染和关注

**Files:**

- Create: `web/apps/dashboard/src/features/conversation-global/employee-conversation-detail.tsx`
- Create: `web/apps/dashboard/src/features/conversation-global/conversation-message-content.tsx`
- Create: `web/apps/dashboard/src/features/conversation-global/conversation-message-content.test.tsx`
- Modify: `web/apps/dashboard/src/features/conversation-global/employee-conversation-page.tsx`
- Modify: `web/apps/dashboard/src/features/conversation-global/employee-conversation-page.test.tsx`

**Interfaces:**

- Detail receives: selected conversation ID, URL-backed filters, query data, focus mutation callbacks
- Message renderer receives: `{ type: number; content: Record<string, unknown> }`

- [ ] **Step 1: 写失败的详情和内容渲染测试**

覆盖：

- 四个统计数字来自 `staffDetail.stats`，null 显示 `--`，不由 messages.length 推算。
- 消息类型、关键词和日期变化后请求带真实参数并重置消息游标。
- “加载更早消息”把下一批插到顶部且按 ID 去重。
- 关注/取消关注分别调用 PUT/DELETE，成功后失效详情、会话列表和目录 query key。
- 图片只使用真实 `url`；文件只使用真实 `name/url`；缺字段显示“该类型内容暂不支持预览”。

示例：

```tsx
render(<ConversationMessageContent type={2} content={{ url: 'https://cdn.example/image.png' }} />);
expect(screen.getByRole('img', { name: '会话图片' }).getAttribute('src')).toBe('https://cdn.example/image.png');

render(<ConversationMessageContent type={5} content={{}} />);
expect(screen.getByText('该类型内容暂不支持预览')).toBeTruthy();
```

- [ ] **Step 2: 运行测试确认 RED**

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/features/conversation-global/employee-conversation-page.test.tsx src/features/conversation-global/conversation-message-content.test.tsx`

Expected: FAIL，详情组件、筛选、游标或类型化渲染不存在。

- [ ] **Step 3: 实现最小消息渲染器**

支持真实类型：文本 1、图片 2、语音 3、视频 4、文件 5、链接 6；其他类型显示 `消息类型 {type}` 与安全文本回退。所有外链增加 `rel="noreferrer"`，不使用 `dangerouslySetInnerHTML`。

- [ ] **Step 4: 实现详情面板与无限向前加载**

React Query 第一页 query key 包含 corp、conversationId、keyword、messageTypes、date；更早消息使用 `useInfiniteQuery` 的 `nextBefore`。筛选控件顺序固定为“全部、消息类型、搜索会话内容、检索日期”。刷新只 refetch 当前详情。

- [ ] **Step 5: 实现关注 mutation**

按钮文案取决于后端 `focused`。mutation 成功后失效：

```ts
queryClient.invalidateQueries({ queryKey: ['corp', access.corp.id, 'staff-directory'] });
queryClient.invalidateQueries({ queryKey: ['corp', access.corp.id, 'employee-conversations', selectedEmployeeId] });
queryClient.invalidateQueries({ queryKey: ['corp', access.corp.id, 'staff-detail', conversationId] });
```

失败时保留原状态并显示可重试错误，禁止乐观伪成功。

- [ ] **Step 6: 运行详情测试确认 GREEN**

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/features/conversation-global/employee-conversation-page.test.tsx src/features/conversation-global/conversation-message-content.test.tsx`

Expected: PASS。

- [ ] **Step 7: 提交**

```powershell
git add web/apps/dashboard/src/features/conversation-global/employee-conversation-detail.tsx web/apps/dashboard/src/features/conversation-global/conversation-message-content.tsx web/apps/dashboard/src/features/conversation-global/conversation-message-content.test.tsx web/apps/dashboard/src/features/conversation-global/employee-conversation-page.tsx web/apps/dashboard/src/features/conversation-global/employee-conversation-page.test.tsx
git commit -m "feat: add employee conversation reader and filters"
```

---

### Task 8: 连续三栏布局与响应式契约

**Files:**

- Create: `web/apps/dashboard/src/styles/employee-conversation-layout.test.ts`
- Modify: `web/apps/dashboard/src/styles/index.css`

**Interfaces:**

- Desktop columns: `272px 380px minmax(0, 1fr)`
- Desktop workspace height: `calc(100dvh - 128px)`，与 72px header 和 56px 内容留白对应
- Breakpoints: 1600、1200、768

- [ ] **Step 1: 写失败的 CSS 契约测试**

```ts
expect(css).toContain('grid-template-columns: 272px 380px minmax(0, 1fr)');
expect(css).toContain('height: calc(100dvh - 128px)');
expect(css).toContain('.employee-conversation-pane { min-height: 0; overflow: hidden; }');
expect(css).toContain('.employee-conversation-scroll { min-height: 0; overflow-y: auto; }');
expect(css).toContain('@media (max-width: 1599px)');
expect(css).toContain('@media (max-width: 1199px)');
```

同时断言删除旧 `grid-template-columns: 250px minmax(300px, 390px) minmax(0, 1fr)`。

- [ ] **Step 2: 运行样式测试确认 RED**

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/styles/employee-conversation-layout.test.ts`

Expected: FAIL，旧样式仍存在且缺少视口高度/响应式抽屉契约。

- [ ] **Step 3: 实现宽屏连续工作台**

`.employee-conversation-workspace` 使用单一白色外框、1px 内部分隔、无列间 gap；右栏 `min-width: 0` 并占满剩余空间。目录、列表、消息各有独立 `.employee-conversation-scroll`，分页固定在中栏底部。

- [ ] **Step 4: 实现三档降级**

- 1200–1599：目录使用覆盖式抽屉，中栏 360px，右栏 `minmax(520px, 1fr)`。
- 769–1199：默认两栏，目录按钮打开抽屉；遮罩与 Escape 均可关闭。
- ≤768：员工、会话、详情分步单栏，返回按钮切换层级；不在 DOM 中同时堆叠三块 360px 高区域。

hover 只改变背景/边框/文字颜色，不使用 transform、scale 或字体抗锯齿切换。

- [ ] **Step 5: 运行样式与页面测试确认 GREEN**

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/styles/employee-conversation-layout.test.ts src/features/conversation-global/employee-conversation-page.test.tsx`

Expected: PASS。

- [ ] **Step 6: 提交**

```powershell
git add web/apps/dashboard/src/styles/employee-conversation-layout.test.ts web/apps/dashboard/src/styles/index.css
git commit -m "style: align employee conversation workspace with yuanhu"
```

---

### Task 9: 全量验证、Docker 更新与浏览器验收

**Files:**

- Modify: `docs/PROJECT_PROGRESS.zh-CN.md`

**Interfaces:**

- Runtime: `mochat-go-desktop`
- Local URL: `http://127.0.0.1:18080/chat/v2-staff`
- Reference URL: `https://web-ai-analysis-work-wechat.yuanhu.com/#/chat/v2-staff`

- [ ] **Step 1: 运行后端相关测试**

Run: `go test ./internal/dashboard ./internal/store ./internal/server ./internal/migration -count=1`

Expected: PASS；记录包数和耗时。任何 SKIP 单独列出原因。

- [ ] **Step 2: 运行 dashboard 全量检查**

Run: `corepack pnpm --filter @mochat/dashboard test`

Run: `corepack pnpm --filter @mochat/dashboard typecheck`

Run: `corepack pnpm --filter @mochat/dashboard build`

Expected: 三条命令退出码 0，无未解释 warning/error。

- [ ] **Step 3: 仅重建 app 容器**

先运行：

```powershell
docker compose -p mochat-go-desktop -f deploy/standalone/docker-compose.yml ps
docker volume ls --filter label=com.docker.compose.project=mochat-go-desktop
```

确认 MySQL、Redis 和卷存在后运行：

```powershell
docker compose -p mochat-go-desktop --env-file deploy/standalone/.env.local -f deploy/standalone/docker-compose.yml up -d --build --no-deps app
```

禁止 `down -v`、`down --volumes` 和任何卷删除命令。

- [ ] **Step 4: 等待健康并核对卷未变化**

Run: `docker compose -p mochat-go-desktop -f deploy/standalone/docker-compose.yml ps`

Expected: app、mysql、redis 均为 healthy；更新前后的命名卷集合一致。

- [ ] **Step 5: 浏览器逐项验收**

在 2560×1440、1440×900、1066×1272、390×844 验证：

1. 菜单和顶部 banner 不变，没有介绍框。
2. 宽屏三栏填满，右栏为主列；1066px 不再出现 83px 详情窄条。
3. 长会话仅消息区滚动。
4. 企业架构、重点关注、部门、员工搜索/刷新、存档/离职筛选真实生效。
5. 会话类型、固定 20 条分页、整卡点击和 URL 恢复正常。
6. 详情类型/关键词/日期、四项统计、刷新、关注和加载更早消息正常。
7. 内部群显示不可用原因；图片/文件缺字段时不伪造预览。
8. 控制台无错误，关键请求无 4xx/5xx；归档未开通场景正确显示 40301 配置状态。

- [ ] **Step 6: 更新进度文档**

在 `docs/PROJECT_PROGRESS.zh-CN.md` 记录：变更范围、两个新接口、数据来源矩阵、内部群与下载缺口、测试命令和结果、四档分辨率、容器健康与数据卷保留情况。

- [ ] **Step 7: 最终提交**

```powershell
git add docs/PROJECT_PROGRESS.zh-CN.md
git commit -m "docs: record employee conversation workspace acceptance"
```

完成后停在员工会话页面，等待用户检查，不自动进入客户会话。
