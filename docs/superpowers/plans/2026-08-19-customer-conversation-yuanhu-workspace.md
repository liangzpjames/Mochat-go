# 客户会话圆弧式工作台 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 `/chat/v2-customer` 从通用会话卡片页改造成真实数据驱动的“客户目录 → 关联单聊/群聊 → 消息阅读”三栏工作台。

**Architecture:** 新增客户会话专用前后端契约，后端在会话权限域内统一聚合客户资料、客户关系、群成员、归档消息与关注状态，前端只管理 URL 状态和三栏联动。消息内容、会话统计和游标算法复用现有员工会话的无状态能力，但不改变员工会话行为。

**Tech Stack:** Go 1.25、MariaDB/MySQL、React 19、TypeScript 5.9、TanStack Query 5、React Router 7、Vitest 2、Testing Library、pnpm、Docker Compose。

## Global Constraints

- 目标仅限 `/chat/v2-customer`；不得联动重构全局消息、员工会话、群聊会话和其他页面。
- 不修改菜单导航、应用顶部 banner、企业切换和全局搜索。
- 客户、会话、消息、统计、状态和能力提示必须来自真实接口；不得添加静态业务数据或把“未接入”显示为 0。
- tenant、corp、user 和员工数据范围只能来自服务端 principal；忽略客户端提交的范围字段。
- 客户目录固定每页 50 个去重客户；关联会话固定每页 20 条；消息游标每次固定读取 50 条。
- 稳定会话 ID 使用 `employeeId:toUserType:toUserId`；群会话必须验证所选客户与群的当前或历史成员关系。
- 群聊入站消息不能标成所选客户发送；界面使用“非员工消息”并展示能力缺口。
- 保留工作树中已有的全局消息、员工会话、概览及其他用户改动；每次只暂存当前任务列出的文件。
- RBAC 新增迁移使用 `0142_customer_conversation_workspace_rbac`，不得改写正在开发的 `0141_conversation_workspace_rbac`。
- Docker 更新只重建 `app`；禁止 `down -v`、`down --volumes` 或删除 MySQL、Redis、应用存储卷。
- 主验收尺寸为 2560×1440，同时回归 1440×900、1066×1272 和 390×844。

---

## 文件结构

### 新建文件

- `internal/dashboard/work_message_customer.go`：领域类型、参数解析、三个 GET handler 和存储接口。
- `internal/dashboard/work_message_customer_test.go`：handler 权限、参数、错误码与响应测试。
- `internal/store/work_message_customer.go`：客户目录、关联会话、客户详情和批量标记聚合。
- `internal/store/work_message_customer_test.go`：SQL 构造、模式规则、关联校验和映射测试。
- `internal/store/work_message_customer_integration_test.go`：真实 MariaDB 去重、权限、群关联、游标和跨 corp 测试。
- `deploy/standalone/migrations/0142_customer_conversation_workspace_rbac.up.sql` 与 `.down.sql`：客户页面资源迁移。
- `web/apps/dashboard/src/features/conversation-global/customer-conversation-directory.tsx` 与 `.test.tsx`：客户目录栏。
- `web/apps/dashboard/src/features/conversation-global/customer-conversation-list.tsx` 与 `.test.tsx`：关联会话栏。
- `web/apps/dashboard/src/features/conversation-global/customer-conversation-detail.tsx` 与 `.test.tsx`：消息详情栏。
- `web/apps/dashboard/src/features/conversation-global/customer-conversation-page.tsx` 与 `.test.tsx`：URL 与查询编排。
- `web/apps/dashboard/src/styles/customer-conversation-layout.test.ts`：响应式样式契约。

### 修改文件

- `web/apps/dashboard/src/features/conversation-global/conversation-global-api.ts` 与 `.test.ts`：客户工作台合同和严格解析。
- `web/apps/dashboard/src/benchmark/page-registry.tsx` 与 `.test.tsx`：客户路由切换到专用页面。
- `web/apps/dashboard/src/styles/index.css`：客户三栏工作台样式。
- `internal/server/server.go`、`internal/server/server_test.go`、`cmd/mochat-go/main.go`：注册并注入三个新路由。
- `internal/dashboard/dashboard_page_catalog.json`、`internal/dashboard/dashboard_access_guard_test.go`：客户页面资源目录。
- `internal/migration/migration_test.go`：0142 迁移合同。

---

### Task 1: 前端客户工作台 API 合同

**Files:**
- Modify: `web/apps/dashboard/src/features/conversation-global/conversation-global-api.ts`
- Test: `web/apps/dashboard/src/features/conversation-global/conversation-global-api.test.ts`

**Interfaces:**
- Consumes: `ApiClient`、`ConversationSummary`、`ConversationMessage`、`ConversationCapability`。
- Produces: `CustomerDirectoryPage`、`CustomerConversationPage`、`CustomerConversationDetail` 和三个 API 方法。

- [ ] **Step 1: 写请求与严格解析失败测试**

```ts
it('requests canonical customer workspace endpoints', async () => {
  client.request
    .mockResolvedValueOnce({ customers: [], counts: { all: 0, focused: 0, active: 0, lost: 0 }, page: 1, pageSize: 50, total: 0, limitations: [], capabilities: [] })
    .mockResolvedValueOnce({ customer: { id: 31, name: '陈晓明', avatar: '', profileStatus: 'available' }, mode: 'direct', list: [], total: 0, page: 1, pageSize: 20, capabilities: [] })
    .mockResolvedValueOnce({ conversationId: '9:1:31', customerId: 31, customerName: '陈晓明', employeeId: 9, employeeName: '张伟', targetType: 'customer', targetId: 31, targetName: '陈晓明', focused: false, stats: { communicationDays: 1, messageTotal: 1, inboundTotal: 1, outboundTotal: 0 }, messages: [], nextBefore: '', hasMore: false, capabilities: [] });
  await api.customerDirectory?.({ mode: 'focused', keyword: '陈', page: 2, pageSize: 50 });
  await api.customerConversations?.({ customerId: 31, mode: 'group', page: 3, pageSize: 20 });
  await api.customerDetail?.({ customerId: 31, conversationId: '9:1:31', keyword: '报价', messageTypes: ['image', 'file'], date: '2026-08-16', pageSize: 50 });
  expect(client.request).toHaveBeenNthCalledWith(1, '/workMessage/customerDirectory?mode=focused&keyword=%E9%99%88&page=2&pageSize=50');
  expect(client.request).toHaveBeenNthCalledWith(2, '/workMessage/customerConversations?customerId=31&mode=group&page=3&pageSize=20');
  expect(client.request).toHaveBeenNthCalledWith(3, '/workMessage/customerDetail?customerId=31&conversationId=9%3A1%3A31&keyword=%E6%8A%A5%E4%BB%B7&messageTypes=image&messageTypes=file&date=2026-08-16&pageSize=50');
});

it('rejects partial directory data', async () => {
  client.request.mockResolvedValue({ customers: [], counts: { all: 0 }, page: 1, pageSize: 50, total: 0, limitations: [], capabilities: [] });
  await expect(api.customerDirectory?.({ mode: 'all', keyword: '', page: 1, pageSize: 50 })).rejects.toThrow('客户目录接口返回了无效数据');
});
```

- [ ] **Step 2: 运行并确认 RED**

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/features/conversation-global/conversation-global-api.test.ts`

Expected: FAIL，客户方法或类型尚不存在。

- [ ] **Step 3: 定义精确类型并实现解析器**

```ts
export type CustomerDirectoryMode = 'all' | 'focused' | 'active' | 'lost';
export type CustomerProfileStatus = 'available' | 'missing' | 'deleted';
export type CustomerConversationMode = 'direct' | 'group';
export type CustomerDirectoryInput = { mode: CustomerDirectoryMode; keyword: string; page: number; pageSize: 50 };
export type CustomerDirectoryCustomer = { id: number; name: string; avatar: string; profileStatus: CustomerProfileStatus; activeRelationCount: number; lostRelationCount: number; directConversationCount: number; groupConversationCount: number; focusedConversationCount: number; lastConversationAt: string };
export type CustomerDirectoryPage = { customers: readonly CustomerDirectoryCustomer[]; counts: { all: number; focused: number; active: number; lost: number }; page: number; pageSize: 50; total: number; limitations: readonly { key: string; reason: string }[]; capabilities: readonly ConversationCapability[] };
export type CustomerConversationInput = { customerId: number; mode: CustomerConversationMode; page: number; pageSize: 20 };
export type CustomerConversationSummary = ConversationSummary & { relationStatus?: 'active' | 'lost' | 'unknown'; membershipStatus?: 'active' | 'left' };
export type CustomerConversationPage = { customer: { id: number; name: string; avatar: string; profileStatus: CustomerProfileStatus }; mode: CustomerConversationMode; list: readonly CustomerConversationSummary[]; total: number; page: number; pageSize: 20; capabilities: readonly ConversationCapability[] };
export type CustomerDetailInput = { customerId: number; conversationId: string; keyword: string; messageTypes: readonly string[]; date: string; pageSize: 50; before?: string };
export type CustomerConversationDetail = StaffConversationDetail & { customerId: number; customerName: string };
```

实现 `parseCustomerDirectory`、`parseCustomerConversations`、`parseCustomerDetail`，逐项校验固定页大小、枚举、计数、稳定 ID、关系状态、统计和 capability。复用现有 `parseSummary`、`parseMessage`、`parseCapability`。

- [ ] **Step 4: 实现三个 API 方法**

```ts
async customerDirectory(input) {
  const query = new URLSearchParams({ mode: input.mode, page: String(input.page), pageSize: '50' });
  appendNonBlank(query, 'keyword', input.keyword);
  return parseCustomerDirectory(await client.request(`/workMessage/customerDirectory?${query.toString()}`));
},
async customerConversations(input) {
  const query = new URLSearchParams({ customerId: String(input.customerId), mode: input.mode, page: String(input.page), pageSize: '20' });
  return parseCustomerConversations(await client.request(`/workMessage/customerConversations?${query.toString()}`));
},
async customerDetail(input) {
  const query = new URLSearchParams({ customerId: String(input.customerId), conversationId: input.conversationId });
  appendNonBlank(query, 'keyword', input.keyword); appendAllNonBlank(query, 'messageTypes', input.messageTypes);
  appendNonBlank(query, 'date', input.date); query.set('pageSize', '50'); appendNonBlank(query, 'before', input.before ?? '');
  return parseCustomerDetail(await client.request(`/workMessage/customerDetail?${query.toString()}`));
},
```

- [ ] **Step 5: 运行 GREEN 并提交**

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/features/conversation-global/conversation-global-api.test.ts`

Expected: PASS。

```powershell
git add web/apps/dashboard/src/features/conversation-global/conversation-global-api.ts web/apps/dashboard/src/features/conversation-global/conversation-global-api.test.ts
git commit -m "feat: add customer conversation api contracts"
```

---

### Task 2: 后端领域合同与 Handler

**Files:**
- Create: `internal/dashboard/work_message_customer.go`
- Create: `internal/dashboard/work_message_customer_test.go`

**Interfaces:**
- Consumes: `resolveAuthorized`、`principalCorpID`、`workMessageConversationPermissionKey`、`parseWorkMessageTypes`、员工详情消息类型。
- Produces: 三个独立存储接口与 `WorkMessageCustomerDirectory/Conversations/Detail` handler。

- [ ] **Step 1: 写 principal 范围与错误码失败测试**

```go
func TestWorkMessageCustomerDirectoryUsesPrincipalScope(t *testing.T) {
  store := &fakeAutoTagStore{customerDirectory: WorkMessageCustomerDirectoryPage{Customers: []WorkMessageCustomerDirectoryItem{}, Counts: WorkMessageCustomerCounts{}, Page: 1, PageSize: 50, Total: 0, Limitations: []WorkMessageCustomerLimitation{}, Capabilities: WorkMessageCustomerCapabilities()}}
  authorizer := &recordingAuthorizer{accessSet: true, access: AccessContext{CorpID: 7, DataPermission: DataPermissionDepartment, DeptEmployeeIDs: []int{9, 10}}}
  handler := NewAutoTagHandler(store, staticAdminCache("7-9"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, authorizer)
  req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/workMessage/customerDirectory?mode=focused&keyword=%E9%99%88&page=1&pageSize=50&corpId=999", nil)
  rec := httptest.NewRecorder(); handler.WorkMessageCustomerDirectory(rec, req)
  if rec.Code != http.StatusOK { t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String()) }
  if store.customerDirectoryFilter.CorpID != 7 || !reflect.DeepEqual(store.customerDirectoryFilter.EmployeeIDs, []int{9, 10}) { t.Fatalf("filter=%#v", store.customerDirectoryFilter) }
}
```

同文件增加表驱动测试：非法 mode/customerId/pageSize/conversationId/date/messageTypes，归档 40301、RBAC 403、权限外 404。

- [ ] **Step 2: 运行并确认 RED**

Run: `go test ./internal/dashboard -run 'TestWorkMessageCustomer' -count=1`

Expected: FAIL，客户类型和 handler 尚不存在。

- [ ] **Step 3: 定义三个独立存储接口**

```go
type WorkMessageCustomerMode string
const (
  WorkMessageCustomerModeAll WorkMessageCustomerMode = "all"
  WorkMessageCustomerModeFocused WorkMessageCustomerMode = "focused"
  WorkMessageCustomerModeActive WorkMessageCustomerMode = "active"
  WorkMessageCustomerModeLost WorkMessageCustomerMode = "lost"
)
type WorkMessageCustomerConversationMode string
const (
  WorkMessageCustomerConversationModeDirect WorkMessageCustomerConversationMode = "direct"
  WorkMessageCustomerConversationModeGroup WorkMessageCustomerConversationMode = "group"
)
type WorkMessageCustomerDirectoryFilter struct { TenantID, CorpID, UserID int; Mode WorkMessageCustomerMode; Keyword string; Page, PageSize int; RestrictEmployeeIDs bool; EmployeeIDs []int }
type WorkMessageCustomerConversationFilter struct { TenantID, CorpID, UserID, CustomerID int; Mode WorkMessageCustomerConversationMode; Page, PageSize int; RestrictEmployeeIDs bool; EmployeeIDs []int }
type WorkMessageCustomerDetailFilter struct { TenantID, CorpID, UserID, CustomerID, EmployeeID, ToUserType, ToUserID int; Keyword string; MessageTypes []int; Date string; PageSize int; Before string; RestrictEmployeeIDs bool; EmployeeIDs []int }

type WorkMessageCustomerProfile struct { ID int `json:"id"`; Name string `json:"name"`; Avatar string `json:"avatar"`; ProfileStatus string `json:"profileStatus"` }
type WorkMessageCustomerDirectoryItem struct { ID int `json:"id"`; Name string `json:"name"`; Avatar string `json:"avatar"`; ProfileStatus string `json:"profileStatus"`; ActiveRelationCount int `json:"activeRelationCount"`; LostRelationCount int `json:"lostRelationCount"`; DirectConversationCount int `json:"directConversationCount"`; GroupConversationCount int `json:"groupConversationCount"`; FocusedConversationCount int `json:"focusedConversationCount"`; LastConversationAt string `json:"lastConversationAt"` }
type WorkMessageCustomerCounts struct { All int `json:"all"`; Focused int `json:"focused"`; Active int `json:"active"`; Lost int `json:"lost"` }
type WorkMessageCustomerLimitation struct { Key string `json:"key"`; Reason string `json:"reason"` }
type WorkMessageCustomerDirectoryPage struct { Customers []WorkMessageCustomerDirectoryItem `json:"customers"`; Counts WorkMessageCustomerCounts `json:"counts"`; Page int `json:"page"`; PageSize int `json:"pageSize"`; Total int `json:"total"`; Limitations []WorkMessageCustomerLimitation `json:"limitations"`; Capabilities []WorkMessageCapability `json:"capabilities"` }
type WorkMessageCustomerConversation struct { WorkMessageGlobalConversation; RelationStatus string `json:"relationStatus,omitempty"`; MembershipStatus string `json:"membershipStatus,omitempty"` }
type WorkMessageCustomerConversationPage struct { Customer WorkMessageCustomerProfile `json:"customer"`; Mode WorkMessageCustomerConversationMode `json:"mode"`; List []WorkMessageCustomerConversation `json:"list"`; Total int `json:"total"`; Page int `json:"page"`; PageSize int `json:"pageSize"`; Capabilities []WorkMessageCapability `json:"capabilities"` }
type WorkMessageCustomerDetail struct { WorkMessageStaffDetail; CustomerID int `json:"customerId"`; CustomerName string `json:"customerName"` }

type WorkMessageCustomerDirectoryStore interface { WorkMessageCustomerDirectory(context.Context, WorkMessageCustomerDirectoryFilter) (WorkMessageCustomerDirectoryPage, error) }
type WorkMessageCustomerConversationStore interface { WorkMessageCustomerConversations(context.Context, WorkMessageCustomerConversationFilter) (WorkMessageCustomerConversationPage, error) }
type WorkMessageCustomerDetailStore interface { WorkMessageCustomerDetail(context.Context, WorkMessageCustomerDetailFilter) (WorkMessageCustomerDetail, error) }

func WorkMessageCustomerCapabilities() []WorkMessageCapability {
  return []WorkMessageCapability{{Key: "groupMemberIdentity", Available: false, Reason: "当前归档数据无法稳定识别群聊入站消息的具体外部成员"}, {Key: "conversationDownload", Available: false, Reason: "当前系统未提供完整的单会话下载能力"}}
}
```

匿名嵌入只复用现有 JSON 字段，不新增第二套字段名；所有响应切片在 handler 输出前规范化为非 nil。关联会话增加 `RelationStatus`、`MembershipStatus`；详情增加 `CustomerID`、`CustomerName`。

- [ ] **Step 4: 实现授权顺序和参数固定值**

```go
func (h *AutoTagHandler) WorkMessageCustomerDirectory(w http.ResponseWriter, r *http.Request) {
  userID, _, principal, access, ok := h.resolveAuthorized(w, r, workMessageConversationPermissionKey); if !ok { return }
  corpID, ok := principalCorpID(w, r); if !ok { return }
  if !h.workMessageArchiveAllowed(w, r, principal.Principal.TenantID, corpID) { return }
  filter, ok := workMessageCustomerDirectoryFilter(w, r, principal.Principal.TenantID, corpID, userID, access); if !ok { return }
  store, ok := h.store.(WorkMessageCustomerDirectoryStore); if !ok { writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "客户目录能力未接入", nil); return }
  page, err := store.WorkMessageCustomerDirectory(r.Context(), filter); if err != nil { writeWorkMessageCustomerError(w, err); return }
  writeEnvelope(w, http.StatusOK, 200, "success", page)
}
```

另两个 handler 使用同一顺序。目录固定 50、会话固定 20、详情固定 50；非 all 权限复制 `access.DeptEmployeeIDs` 并设置 `RestrictEmployeeIDs=true`。详情 handler 解析 `conversationId` 后，还必须确认其中的 `employeeId` 位于规范化后的 `EmployeeIDs`；权限范围为空或不包含该员工时返回 404，不能把权限外资源暴露为 403。

- [ ] **Step 5: 运行 GREEN 并提交**

Run: `go test ./internal/dashboard -run 'TestWorkMessageCustomer' -count=1`

Expected: PASS。

```powershell
git add internal/dashboard/work_message_customer.go internal/dashboard/work_message_customer_test.go
git commit -m "feat: define customer conversation handlers"
```

---

### Task 3: 客户目录真实聚合

**Files:**
- Create: `internal/store/work_message_customer.go`
- Create: `internal/store/work_message_customer_test.go`
- Create: `internal/store/work_message_customer_integration_test.go`

**Interfaces:**
- Consumes: Task 2 目录 filter、归档来源、`workMessageFilteredUnionSQLWithArchiveSourceState`。
- Produces: `MySQLStore.WorkMessageCustomerDirectory`。

- [ ] **Step 1: 写去重、状态互斥和权限失败测试**

```go
func TestCustomerDirectoryModePredicateIsMutuallyExclusive(t *testing.T) {
  cases := map[dashboard.WorkMessageCustomerMode]string{dashboard.WorkMessageCustomerModeFocused: "focused_conversation_count > 0", dashboard.WorkMessageCustomerModeActive: "active_relation_count > 0", dashboard.WorkMessageCustomerModeLost: "active_relation_count = 0 AND lost_relation_count > 0"}
  for mode, fragment := range cases { where, _ := customerDirectoryOuterWhere(dashboard.WorkMessageCustomerDirectoryFilter{Mode: mode}); if !strings.Contains(where, fragment) { t.Fatalf("mode=%s where=%s", mode, where) } }
}
```

MariaDB 用例建立两个员工、三个客户、有效关系、纯流失客户、资料缺失客户、直接归档、群成员和关注记录，断言 `all=3/focused=1/active=1/lost=1`，跨员工客户只出现一次，员工 9 权限看不到员工 10 独占客户。

- [ ] **Step 2: 运行并确认 RED**

Run: `go test ./internal/store -run 'TestCustomerDirectory' -count=1`

Expected: FAIL，目录 SQL 与方法不存在。

- [ ] **Step 3: 构造同源派生表**

```go
baseSQL := `SELECT candidate.customer_id, COALESCE(NULLIF(contact.name,''),NULLIF(candidate.archive_name,''),'') name,
       COALESCE(contact.avatar,'') avatar,
       CASE WHEN contact.id IS NULL THEN 'missing' WHEN contact.deleted_at IS NOT NULL THEN 'deleted' ELSE 'available' END profile_status,
       COALESCE(relation.active_count,0) active_relation_count, COALESCE(relation.lost_count,0) lost_relation_count,
       candidate.direct_count, candidate.group_count, COALESCE(focus.focused_count,0) focused_conversation_count,
       candidate.last_conversation_at
FROM (
  SELECT source.customer_id, MAX(source.archive_name) archive_name,
         COUNT(DISTINCT source.direct_key) direct_count, COUNT(DISTINCT source.group_key) group_count,
         MAX(source.last_at) last_conversation_at
  FROM (
    SELECT wm.to_user_id customer_id, MAX(wm.target_name) archive_name,
           CONCAT(wm.work_employee_id,':1:',wm.to_user_id) direct_key, NULL group_key, MAX(wm.msg_data_time) last_at
    FROM (`+archiveSQL+`) wm WHERE wm.to_user_type=1 GROUP BY wm.work_employee_id,wm.to_user_id
    UNION ALL
    SELECT membership.contact_id, MAX(wm.target_name), NULL,
           CONCAT(wm.work_employee_id,':2:',wm.to_user_id), MAX(wm.msg_data_time)
    FROM mc_work_contact_room membership JOIN (`+archiveSQL+`) wm ON wm.to_user_type=2 AND wm.to_user_id=membership.room_id
    WHERE membership.contact_id>0 GROUP BY membership.contact_id,wm.work_employee_id,wm.to_user_id
  ) source GROUP BY source.customer_id
) candidate
LEFT JOIN mc_work_contact contact ON contact.id=candidate.customer_id AND contact.corp_id=?
LEFT JOIN (
  SELECT contact_id,
         COUNT(DISTINCT CASE WHEN status=1 AND deleted_at IS NULL THEN employee_id END) active_count,
         COUNT(DISTINCT CASE WHEN status IN (2,3) THEN employee_id END) lost_count
  FROM mc_work_contact_employee WHERE corp_id=? GROUP BY contact_id
) relation ON relation.contact_id=candidate.customer_id
LEFT JOIN (
  SELECT to_user_id contact_id,COUNT(*) focused_count FROM mochat_go_work_message_focus
  WHERE tenant_id=? AND corp_id=? AND user_id=? AND to_user_type=1 GROUP BY to_user_id
) focus ON focus.contact_id=candidate.customer_id`
baseArgs := append(append([]any{}, archiveArgs...), archiveArgs...)
baseArgs = append(baseArgs, filter.CorpID, filter.CorpID, filter.TenantID, filter.CorpID, filter.UserID)
```

两个归档分支、关系和关注都在各自派生查询中追加同一允许员工集合谓词；对应员工 ID 参数按 SQL 占位符顺序加入 `baseArgs`。

- [ ] **Step 4: 实现 count/page 同源与稳定排序**

```go
func (s *MySQLStore) WorkMessageCustomerDirectory(ctx context.Context, filter dashboard.WorkMessageCustomerDirectoryFilter) (dashboard.WorkMessageCustomerDirectoryPage, error) {
  filter.Page, filter.PageSize = positivePage(filter.Page), 50
  if filter.RestrictEmployeeIDs && len(uniquePositiveInts(filter.EmployeeIDs)) == 0 { return emptyCustomerDirectory(filter), nil }
  baseSQL, baseArgs, available, err := s.customerDirectorySource(ctx, filter); if err != nil { return dashboard.WorkMessageCustomerDirectoryPage{}, err }
  if !available { return unavailableCustomerDirectory(filter, "当前企业没有可用的会话存档数据"), nil }
  where, whereArgs := customerDirectoryOuterWhere(filter)
  counts, err := s.customerDirectoryCounts(ctx, baseSQL, baseArgs, filter); if err != nil { return dashboard.WorkMessageCustomerDirectoryPage{}, err }
  total, err := countDerivedRows(ctx, s.db, baseSQL, baseArgs, where, whereArgs); if err != nil { return dashboard.WorkMessageCustomerDirectoryPage{}, err }
  customers, err := s.customerDirectoryPage(ctx, baseSQL, baseArgs, where, whereArgs, filter.Page, 50); if err != nil { return dashboard.WorkMessageCustomerDirectoryPage{}, err }
  return dashboard.WorkMessageCustomerDirectoryPage{Customers: customers, Counts: counts, Page: filter.Page, PageSize: 50, Total: total, Limitations: customerDirectoryLimitations(customers), Capabilities: dashboard.WorkMessageCustomerCapabilities()}, nil
}
```

同文件补齐这些辅助函数，签名不得在实现时漂移：

```go
func customerDirectoryOuterWhere(filter dashboard.WorkMessageCustomerDirectoryFilter) (string, []any)
func (s *MySQLStore) customerDirectorySource(ctx context.Context, filter dashboard.WorkMessageCustomerDirectoryFilter) (string, []any, bool, error)
func (s *MySQLStore) customerDirectoryCounts(ctx context.Context, baseSQL string, baseArgs []any, filter dashboard.WorkMessageCustomerDirectoryFilter) (dashboard.WorkMessageCustomerCounts, error)
func countDerivedRows(ctx context.Context, db *sql.DB, baseSQL string, baseArgs []any, where string, whereArgs []any) (int, error)
func (s *MySQLStore) customerDirectoryPage(ctx context.Context, baseSQL string, baseArgs []any, where string, whereArgs []any, page, pageSize int) ([]dashboard.WorkMessageCustomerDirectoryItem, error)
func emptyCustomerDirectory(filter dashboard.WorkMessageCustomerDirectoryFilter) dashboard.WorkMessageCustomerDirectoryPage
func unavailableCustomerDirectory(filter dashboard.WorkMessageCustomerDirectoryFilter, reason string) dashboard.WorkMessageCustomerDirectoryPage
func customerDirectoryLimitations(items []dashboard.WorkMessageCustomerDirectoryItem) []dashboard.WorkMessageCustomerLimitation
```

`customerDirectoryOuterWhere` 只生成模式和关键词条件；`customerDirectoryCounts` 对同一 `baseSQL` 一次性计算四个计数；`countDerivedRows` 与 `customerDirectoryPage` 分别执行 count 和固定排序分页。两个 empty helper 均返回非 nil 空切片、固定 `PageSize: 50` 和 capability；unavailable 版本额外返回归档不可用 limitation。`customerDirectoryLimitations` 只根据本页 `profile_status` 汇总“资料缺失/已删除”提示，不改变计数。

排序固定为 `last_conversation_at DESC, customer_id DESC`；关键词匹配名称、归档名和外部联系人标识，但响应不返回该标识。

- [ ] **Step 5: 运行 GREEN 并提交**

Run: `go test ./internal/store -run 'TestCustomerDirectory' -count=1`

Expected: 单元测试 PASS；未设置集成 DSN 时集成用例明确 SKIP。

```powershell
git add internal/store/work_message_customer.go internal/store/work_message_customer_test.go internal/store/work_message_customer_integration_test.go
git commit -m "feat: aggregate customer conversation directory"
```

---

### Task 4: 客户关联单聊与群聊查询

**Files:**
- Modify: `internal/store/work_message_customer.go`
- Modify: `internal/store/work_message_customer_test.go`
- Modify: `internal/store/work_message_customer_integration_test.go`

**Interfaces:**
- Consumes: Task 3 的归档基础集合与 Task 2 会话 filter。
- Produces: `MySQLStore.WorkMessageCustomerConversations`。

- [ ] **Step 1: 写稳定 ID、群关系和总数同源失败测试**

```go
func TestCustomerConversationBaseUsesStableConversationID(t *testing.T) {
  sqlText, _, err := customerConversationBaseSQL(dashboard.WorkMessageCustomerConversationFilter{CustomerID: 31, Mode: dashboard.WorkMessageCustomerConversationModeDirect}, "SELECT * FROM archive", nil); if err != nil { t.Fatal(err) }
  for _, fragment := range []string{"wm.to_user_type = 1", "wm.to_user_id = ?", "CONCAT(wm.work_employee_id, ':1:', wm.to_user_id)", "GROUP BY wm.work_employee_id, wm.to_user_id"} { if !strings.Contains(sqlText, fragment) { t.Fatalf("missing %q", fragment) } }
}
```

集成用例断言：直接会话多消息只返回一张卡；同群两个员工返回两个稳定 ID；已退群返回 `left`；无成员关系不返回；单页时 `total == len(list)`。

- [ ] **Step 2: 运行并确认 RED**

Run: `go test ./internal/store -run 'TestCustomerConversation' -count=1`

Expected: FAIL。

- [ ] **Step 3: 实现直接与群聊派生查询**

直接模式按 `work_employee_id,to_user_id` 聚合；群模式先将客户成员关系按 `room_id` 去重，再与归档 `to_user_type=2` 求交。基础查询输出稳定 ID、最后消息行、消息总数、员工/目标资料、归档来源、关系或成员状态。`COUNT(*)` 和分页 `SELECT` 包裹同一个 `baseSQL`。

```go
func (s *MySQLStore) WorkMessageCustomerConversations(ctx context.Context, filter dashboard.WorkMessageCustomerConversationFilter) (dashboard.WorkMessageCustomerConversationPage, error) {
  filter.Page, filter.PageSize = positivePage(filter.Page), 20
  profile, visible, err := s.customerWorkspaceProfile(ctx, filter.CorpID, filter.CustomerID, filter.RestrictEmployeeIDs, filter.EmployeeIDs); if err != nil { return dashboard.WorkMessageCustomerConversationPage{}, err }
  if !visible { return dashboard.WorkMessageCustomerConversationPage{}, dashboard.ErrWorkMessageConversationNotFound }
  baseSQL, args, available, err := s.customerConversationSource(ctx, filter); if err != nil { return dashboard.WorkMessageCustomerConversationPage{}, err }
  if !available { return emptyCustomerConversationPage(profile, filter), nil }
  total, err := countCustomerConversations(ctx, s.db, baseSQL, args); if err != nil { return dashboard.WorkMessageCustomerConversationPage{}, err }
  list, err := s.customerConversationPage(ctx, baseSQL, args, filter.Page, 20); if err != nil { return dashboard.WorkMessageCustomerConversationPage{}, err }
  capabilities, err := s.decorateCustomerConversationFlags(ctx, filter, list); if err != nil { return dashboard.WorkMessageCustomerConversationPage{}, err }
  return dashboard.WorkMessageCustomerConversationPage{Customer: profile, Mode: filter.Mode, List: list, Total: total, Page: filter.Page, PageSize: 20, Capabilities: capabilities}, nil
}
```

同文件补齐以下辅助函数：

```go
func (s *MySQLStore) customerWorkspaceProfile(ctx context.Context, corpID, customerID int, restrictEmployeeIDs bool, employeeIDs []int) (dashboard.WorkMessageCustomerProfile, bool, error)
func (s *MySQLStore) customerConversationSource(ctx context.Context, filter dashboard.WorkMessageCustomerConversationFilter) (string, []any, bool, error)
func countCustomerConversations(ctx context.Context, db *sql.DB, baseSQL string, args []any) (int, error)
func (s *MySQLStore) customerConversationPage(ctx context.Context, baseSQL string, args []any, page, pageSize int) ([]dashboard.WorkMessageCustomerConversation, error)
func (s *MySQLStore) decorateCustomerConversationFlags(ctx context.Context, filter dashboard.WorkMessageCustomerConversationFilter, list []dashboard.WorkMessageCustomerConversation) ([]dashboard.WorkMessageCapability, error)
func emptyCustomerConversationPage(profile dashboard.WorkMessageCustomerProfile, filter dashboard.WorkMessageCustomerConversationFilter) dashboard.WorkMessageCustomerConversationPage
```

`customerWorkspaceProfile` 同时接受资料表存在和仅归档存在两种情况，并以允许员工集合验证可见性；`customerConversationSource` 返回直接/群聊模式共用的派生查询；count 与 page 包裹同一 SQL。`emptyCustomerConversationPage` 返回非 nil 空列表、固定 `PageSize: 20` 和归档不可用 capability。

- [ ] **Step 4: 批量装饰关注、风险与超时**

当前页最多执行三条批量查询；先根据本页稳定会话 ID 数量调用 `strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")` 生成绑定标记，再组成 `conversation_id IN (` + marks + `)`。空页直接跳过装饰查询。表不存在时返回 unavailable capability；不得逐卡查询，也不得把缺表映射成 0。

- [ ] **Step 5: 运行 GREEN 并提交**

Run: `go test ./internal/store -run 'TestCustomerConversation' -count=1`

Expected: PASS。

```powershell
git add internal/store/work_message_customer.go internal/store/work_message_customer_test.go internal/store/work_message_customer_integration_test.go
git commit -m "feat: query customer related conversations"
```

---

### Task 5: 客户详情校验与员工消息算法复用

**Files:**
- Modify: `internal/store/work_message_customer.go`
- Modify: `internal/store/work_message_customer_test.go`
- Modify: `internal/store/work_message_customer_integration_test.go`

**Interfaces:**
- Consumes: `MySQLStore.StaffDetail` 与客户/群关系。
- Produces: `MySQLStore.WorkMessageCustomerDetail`。

- [ ] **Step 1: 写直接一致性、群关系和游标失败测试**

```go
func TestCustomerDetailAssociationRules(t *testing.T) {
  if err := validateCustomerConversationAssociation(31, 1, 99, false); !errors.Is(err, dashboard.ErrWorkMessageConversationNotFound) { t.Fatalf("err=%v", err) }
  if err := validateCustomerConversationAssociation(31, 1, 31, false); err != nil { t.Fatal(err) }
  if err := validateCustomerConversationAssociation(31, 2, 44, false); !errors.Is(err, dashboard.ErrWorkMessageConversationNotFound) { t.Fatalf("err=%v", err) }
  if err := validateCustomerConversationAssociation(31, 2, 44, true); err != nil { t.Fatal(err) }
}
```

集成用例读取最新 50 条和更早游标页，断言无重复、全局正序、完整统计不随关键词变化；群无成员关系时返回 404。

- [ ] **Step 2: 运行并确认 RED**

Run: `go test ./internal/store -run 'TestCustomerDetail' -count=1`

Expected: FAIL。

- [ ] **Step 3: 验证关联后复用 StaffDetail**

```go
func (s *MySQLStore) WorkMessageCustomerDetail(ctx context.Context, filter dashboard.WorkMessageCustomerDetailFilter) (dashboard.WorkMessageCustomerDetail, error) {
  if filter.RestrictEmployeeIDs && !containsPositiveInt(uniquePositiveInts(filter.EmployeeIDs), filter.EmployeeID) { return dashboard.WorkMessageCustomerDetail{}, dashboard.ErrWorkMessageConversationNotFound }
  if filter.ToUserType == 1 && filter.ToUserID != filter.CustomerID { return dashboard.WorkMessageCustomerDetail{}, dashboard.ErrWorkMessageConversationNotFound }
  if filter.ToUserType == 2 { related, err := s.customerRoomMembershipExists(ctx, filter.CorpID, filter.CustomerID, filter.ToUserID); if err != nil { return dashboard.WorkMessageCustomerDetail{}, err }; if !related { return dashboard.WorkMessageCustomerDetail{}, dashboard.ErrWorkMessageConversationNotFound } }
  profile, visible, err := s.customerWorkspaceProfile(ctx, filter.CorpID, filter.CustomerID, filter.RestrictEmployeeIDs, filter.EmployeeIDs); if err != nil { return dashboard.WorkMessageCustomerDetail{}, err }; if !visible { return dashboard.WorkMessageCustomerDetail{}, dashboard.ErrWorkMessageConversationNotFound }
  detail, err := s.StaffDetail(ctx, dashboard.WorkMessageStaffDetailFilter{TenantID: filter.TenantID, CorpID: filter.CorpID, UserID: filter.UserID, EmployeeID: filter.EmployeeID, ToUserType: filter.ToUserType, ToUserID: filter.ToUserID, Keyword: filter.Keyword, MessageTypes: filter.MessageTypes, Date: filter.Date, PageSize: 50, Before: filter.Before})
  if err != nil { return dashboard.WorkMessageCustomerDetail{}, err }
  return customerDetailFromStaff(profile, detail), nil
}
```

同文件补齐以下辅助函数：

```go
func validateCustomerConversationAssociation(customerID, toUserType, toUserID int, roomMembershipExists bool) error
func (s *MySQLStore) customerRoomMembershipExists(ctx context.Context, corpID, customerID, roomID int) (bool, error)
func customerDetailFromStaff(profile dashboard.WorkMessageCustomerProfile, detail dashboard.WorkMessageStaffDetail) dashboard.WorkMessageCustomerDetail
func containsPositiveInt(values []int, target int) bool
```

`validateCustomerConversationAssociation` 对单聊要求 `toUserID == customerID`，对群聊要求成员关系存在，其余类型一律返回 `ErrWorkMessageConversationNotFound`；`customerRoomMembershipExists` 必须限定 `corp_id/contact_id/room_id`，允许已退群记录以支持历史归档；`customerDetailFromStaff` 只增加客户身份字段，消息顺序、统计、游标和 capability 原样保留；`containsPositiveInt` 只做规范化后切片的线性包含判断。

- [ ] **Step 4: 运行客户与员工详情回归**

Run: `go test ./internal/store -run 'Test(CustomerDetail|StaffMessageWhere|StaffDetail)' -count=1`

Expected: PASS。

- [ ] **Step 5: 提交**

```powershell
git add internal/store/work_message_customer.go internal/store/work_message_customer_test.go internal/store/work_message_customer_integration_test.go
git commit -m "feat: add scoped customer conversation detail"
```

---

### Task 6: Server 路由、页面目录与 RBAC

**Files:**
- Modify: `internal/server/server.go`
- Modify: `internal/server/server_test.go`
- Modify: `cmd/mochat-go/main.go`
- Modify: `internal/dashboard/dashboard_page_catalog.json`
- Modify: `internal/dashboard/dashboard_access_guard_test.go`
- Create: `deploy/standalone/migrations/0142_customer_conversation_workspace_rbac.up.sql`
- Create: `deploy/standalone/migrations/0142_customer_conversation_workspace_rbac.down.sql`
- Modify: `internal/migration/migration_test.go`

**Interfaces:**
- Consumes: Task 2 的三个 handler。
- Produces: 三个 HTTP 路由和客户页面精确资源授权。

- [ ] **Step 1: 写路由和目录资源失败测试**

```go
tests := []struct{ method, path, body, route string }{
  {http.MethodGet, "/dashboard/workMessage/customerDirectory?mode=all&page=1&pageSize=50", "customer directory", "GET /dashboard/workMessage/customerDirectory"},
  {http.MethodGet, "/dashboard/workMessage/customerConversations?customerId=31&mode=direct&page=1&pageSize=20", "customer conversations", "GET /dashboard/workMessage/customerConversations"},
  {http.MethodGet, "/dashboard/workMessage/customerDetail?customerId=31&conversationId=9:1:31&pageSize=50", "customer detail", "GET /dashboard/workMessage/customerDetail"},
}
```

目录测试断言客户页面含三个 GET 和 focus PUT/DELETE，不要求 `/dashboard/workContact/index`。

- [ ] **Step 2: 运行并确认 RED**

Run: `go test ./internal/server ./internal/dashboard ./internal/migration -run 'Test.*(WorkMessageCustomer|DashboardPageCatalog|0142)' -count=1`

Expected: FAIL。

- [ ] **Step 3: 注册 option、路由、清单和 main 注入**

```go
func WithWorkMessageCustomerDirectoryHandler(h http.Handler) Option { return func(s *Server) { s.workMessageCustomerDirectory = h } }
func WithWorkMessageCustomerConversationsHandler(h http.Handler) Option { return func(s *Server) { s.workMessageCustomerConversations = h } }
func WithWorkMessageCustomerDetailHandler(h http.Handler) Option { return func(s *Server) { s.workMessageCustomerDetail = h } }
```

在 `ServeHTTP` 精确匹配三个 GET，在 routes 清单加入三个字符串；main 注入 `autoTag.WorkMessageCustomerDirectory`、`WorkMessageCustomerConversations`、`WorkMessageCustomerDetail`。

- [ ] **Step 4: 增加幂等 0142 up/down**

```sql
INSERT INTO mochat_go_dashboard_permission_resources (permission_id,resource_type,http_method,path_pattern,scope_required,status,version)
SELECT p.id,'api',seed.http_method,seed.path_pattern,1,1,1
FROM mochat_go_dashboard_permissions p
JOIN (
 SELECT 'dashboard.chat.v2_customer' permission_code,'GET' http_method,'/dashboard/workMessage/customerDirectory' path_pattern
 UNION ALL SELECT 'dashboard.chat.v2_customer','GET','/dashboard/workMessage/customerConversations'
 UNION ALL SELECT 'dashboard.chat.v2_customer','GET','/dashboard/workMessage/customerDetail'
 UNION ALL SELECT 'dashboard.chat.v2_customer','PUT','/dashboard/workMessage/focus'
 UNION ALL SELECT 'dashboard.chat.v2_customer','DELETE','/dashboard/workMessage/focus'
) seed ON seed.permission_code=p.code
WHERE NOT EXISTS (SELECT 1 FROM mochat_go_dashboard_permission_resources e WHERE e.permission_id=p.id AND e.resource_type='api' AND e.http_method=seed.http_method AND e.path_pattern=seed.path_pattern);
```

down 仅删除这五个精确 method/path 组合。

- [ ] **Step 5: 运行 GREEN 并提交**

Run: `go test ./internal/server ./internal/dashboard ./internal/migration -run 'Test.*(WorkMessageCustomer|DashboardPageCatalog|0142)' -count=1`

Expected: PASS。

```powershell
git add internal/server/server.go internal/server/server_test.go cmd/mochat-go/main.go internal/dashboard/dashboard_page_catalog.json internal/dashboard/dashboard_access_guard_test.go internal/migration/migration_test.go deploy/standalone/migrations/0142_customer_conversation_workspace_rbac.up.sql deploy/standalone/migrations/0142_customer_conversation_workspace_rbac.down.sql
git commit -m "feat: register customer conversation workspace routes"
```

---

### Task 7: 客户目录栏

**Files:**
- Create: `web/apps/dashboard/src/features/conversation-global/customer-conversation-directory.tsx`
- Create: `web/apps/dashboard/src/features/conversation-global/customer-conversation-directory.test.tsx`

**Interfaces:**
- Consumes: `CustomerDirectoryPage`、`CustomerDirectoryMode`。
- Produces: 纯展示组件 `CustomerConversationDirectory`。

- [ ] **Step 1: 写模式、搜索、分页和资料缺失失败测试**

```tsx
render(<CustomerConversationDirectory data={page} mode="all" keywordDraft="" page={1} selectedCustomerId={null}
  pending={false} fetching={false} error={null} onModeChange={onModeChange} onKeywordDraftChange={onKeywordDraftChange}
  onSearch={onSearch} onRefresh={onRefresh} onPageChange={onPageChange} onSelectCustomer={onSelectCustomer} />);
fireEvent.click(screen.getByRole('tab', { name: '重点关注 2' }));
expect(onModeChange).toHaveBeenCalledWith('focused');
fireEvent.change(screen.getByRole('textbox', { name: '搜索客户' }), { target: { value: '陈' } });
fireEvent.submit(screen.getByRole('search', { name: '客户搜索' }));
expect(onSearch).toHaveBeenCalled();
fireEvent.click(screen.getByRole('button', { name: /陈晓明/ }));
expect(onSelectCustomer).toHaveBeenCalledWith(31);
expect(screen.getByText('客户资料未同步')).toBeTruthy();
```

- [ ] **Step 2: 运行并确认 RED**

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/features/conversation-global/customer-conversation-directory.test.tsx`

Expected: FAIL，组件不存在。

- [ ] **Step 3: 实现无查询逻辑的目录组件**

组件固定包含：客户列表/重点关注页签、搜索和刷新、滚动列表、`DashboardPagination`、有效关系/已流失底部筛选。客户整行使用 `<button>`，选中态使用 `aria-pressed`；图片缺失时显示名称首字；资料状态异常显示紧凑告警；加载、空态和错误使用 `PageState`。

```tsx
export function CustomerConversationDirectory(props: Props) {
  const all = props.data?.counts.all ?? 0;
  const focused = props.data?.counts.focused ?? 0;
  return <section aria-label="客户目录" className="customer-conversation-pane customer-conversation-directory">
    <div aria-label="客户目录模式" role="tablist">
      <button aria-selected={props.mode === 'all'} onClick={() => props.onModeChange('all')} role="tab">客户列表 {all}</button>
      <button aria-selected={props.mode === 'focused'} onClick={() => props.onModeChange('focused')} role="tab">重点关注 {focused}</button>
    </div>
    <form aria-label="客户搜索" role="search" onSubmit={props.onSearch}>
      <input aria-label="搜索客户" value={props.keywordDraft} onChange={(event) => props.onKeywordDraftChange(event.target.value)} />
      <button type="submit">查询</button><button disabled={props.fetching} onClick={props.onRefresh} type="button">刷新</button>
    </form>
    <div className="customer-conversation-scroll">{props.data?.customers.map((customer) => <button aria-pressed={props.selectedCustomerId === customer.id} key={customer.id} onClick={() => props.onSelectCustomer(customer.id)} type="button"><strong>{customer.name || `客户 ${customer.id}`}</strong><span>{customer.directConversationCount} 单聊 · {customer.groupConversationCount} 群聊</span>{customer.profileStatus !== 'available' && <em>客户资料未同步</em>}</button>)}</div>
  </section>;
}
```

- [ ] **Step 4: 运行 GREEN 并提交**

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/features/conversation-global/customer-conversation-directory.test.tsx`

Expected: PASS。

```powershell
git add web/apps/dashboard/src/features/conversation-global/customer-conversation-directory.tsx web/apps/dashboard/src/features/conversation-global/customer-conversation-directory.test.tsx
git commit -m "feat: add customer conversation directory pane"
```

---

### Task 8: 客户关联会话栏

**Files:**
- Create: `web/apps/dashboard/src/features/conversation-global/customer-conversation-list.tsx`
- Create: `web/apps/dashboard/src/features/conversation-global/customer-conversation-list.test.tsx`

**Interfaces:**
- Consumes: `CustomerConversationPage`、`CustomerConversationMode`。
- Produces: `CustomerConversationList`。

- [ ] **Step 1: 写单聊/群聊、整卡点击和能力提示失败测试**

```tsx
render(<CustomerConversationList data={groupPage} mode="group" page={1} selectedConversationId={null}
  pending={false} fetching={false} error={null} onModeChange={onModeChange} onPageChange={onPageChange}
  onRefresh={onRefresh} onSelectConversation={onSelectConversation} />);
fireEvent.click(screen.getByRole('tab', { name: '单聊' }));
expect(onModeChange).toHaveBeenCalledWith('direct');
fireEvent.click(screen.getByRole('button', { name: /产品交流群.*张伟/ }));
expect(onSelectConversation).toHaveBeenCalledWith('9:2:44');
expect(screen.getByText('已退群')).toBeTruthy();
expect(screen.getByRole('alert')).toHaveTextContent('无法稳定识别群聊入站消息');
```

- [ ] **Step 2: 运行并确认 RED**

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/features/conversation-global/customer-conversation-list.test.tsx`

Expected: FAIL。

- [ ] **Step 3: 实现客户头、模式页签和卡片列表**

直接卡片主标题使用员工名、副标题使用客户名；群聊卡片主标题使用群名、副标题明确归档员工。关系、已退群、风险、超时和关注只在响应字段可用时展示。整卡按钮使用稳定 `conversationId`；分页使用响应 `total` 和固定 20。

```tsx
<button aria-pressed={props.selectedConversationId === item.conversationId}
  aria-label={`${item.targetType === 'room' ? item.targetName : item.employeeName} ${item.employeeName}`}
  onClick={() => props.onSelectConversation(item.conversationId!)} type="button">
  <strong>{item.targetType === 'room' ? item.targetName : item.employeeName}</strong>
  <span>{item.lastMessage || '暂无可展示的消息内容'}</span>
  {item.membershipStatus === 'left' && <em>已退群</em>}
</button>
```

- [ ] **Step 4: 运行 GREEN 并提交**

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/features/conversation-global/customer-conversation-list.test.tsx`

Expected: PASS。

```powershell
git add web/apps/dashboard/src/features/conversation-global/customer-conversation-list.tsx web/apps/dashboard/src/features/conversation-global/customer-conversation-list.test.tsx
git commit -m "feat: add customer related conversation pane"
```

---

### Task 9: 客户消息详情栏

**Files:**
- Create: `web/apps/dashboard/src/features/conversation-global/customer-conversation-detail.tsx`
- Create: `web/apps/dashboard/src/features/conversation-global/customer-conversation-detail.test.tsx`
- Reuse: `web/apps/dashboard/src/features/conversation-global/conversation-message-content.tsx`

**Interfaces:**
- Consumes: `CustomerConversationDetail`、`ConversationMessageContent`。
- Produces: `CustomerConversationDetailPane`。

- [ ] **Step 1: 写统计标签、筛选、游标和关注失败测试**

```tsx
render(<CustomerConversationDetailPane data={directDetail} messages={directDetail.messages} keyword="" date="" messageTypes={[]}
  pending={false} fetching={false} loadingOlder={false} error={null} focusPending={false} focusError={null} hasMore
  onKeywordChange={onKeywordChange} onDateChange={onDateChange} onToggleMessageType={onToggleMessageType}
  onRefresh={onRefresh} onLoadOlder={onLoadOlder} onToggleFocus={onToggleFocus} />);
expect(screen.getByText('客户发送')).toBeTruthy();
fireEvent.click(screen.getByRole('button', { name: '加载更早消息' })); expect(onLoadOlder).toHaveBeenCalled();
fireEvent.click(screen.getByRole('button', { name: '重点关注' })); expect(onToggleFocus).toHaveBeenCalled();
rerender(<CustomerConversationDetailPane {...props} data={{ ...directDetail, targetType: 'room' }} />);
expect(screen.getByText('非员工消息')).toBeTruthy();
expect(screen.queryByText('客户发送')).toBeNull();
```

- [ ] **Step 2: 运行并确认 RED**

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/features/conversation-global/customer-conversation-detail.test.tsx`

Expected: FAIL。

- [ ] **Step 3: 实现详情头、统计和消息流**

复用消息类型 `text/image/voice/video/file/link` 和 `ConversationMessageContent`。统计 `null` 显示 `--`；筛选无结果只替换消息区；群聊 capability 告警持续可见；发送方为空时显示“群成员”，不得使用客户名。

```tsx
const inboundLabel = props.data?.targetType === 'room' ? '非员工消息' : '客户发送';
<article><span>{inboundLabel}</span><strong>{stat(props.data.stats.inboundTotal)}</strong><small>条</small></article>
```

- [ ] **Step 4: 运行 GREEN 并提交**

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/features/conversation-global/customer-conversation-detail.test.tsx src/features/conversation-global/conversation-message-content.test.tsx`

Expected: PASS。

```powershell
git add web/apps/dashboard/src/features/conversation-global/customer-conversation-detail.tsx web/apps/dashboard/src/features/conversation-global/customer-conversation-detail.test.tsx
git commit -m "feat: add customer conversation detail pane"
```

---

### Task 10: URL、查询编排与路由切换

**Files:**
- Create: `web/apps/dashboard/src/features/conversation-global/customer-conversation-page.tsx`
- Create: `web/apps/dashboard/src/features/conversation-global/customer-conversation-page.test.tsx`
- Modify: `web/apps/dashboard/src/benchmark/page-registry.tsx`
- Modify: `web/apps/dashboard/src/benchmark/page-registry.test.tsx`

**Interfaces:**
- Consumes: Tasks 1、7、8、9 的 API 与组件。
- Produces: `CustomerConversationPage` 和客户 route 映射。

- [ ] **Step 1: 写 URL 规范化和三层查询失败测试**

```tsx
const view = renderCustomerPage('/chat/v2-customer?customerMode=focused&customerPage=2&customerId=31&conversationMode=group&page=3&pageSize=99&conversationId=9:2:44&messageTypes=image');
await waitFor(() => expect(api.customerDirectory).toHaveBeenCalledWith({ mode: 'focused', keyword: '', page: 2, pageSize: 50 }));
await waitFor(() => expect(api.customerConversations).toHaveBeenCalledWith({ customerId: 31, mode: 'group', page: 3, pageSize: 20 }));
await waitFor(() => expect(api.customerDetail).toHaveBeenCalledWith(expect.objectContaining({ customerId: 31, conversationId: '9:2:44', messageTypes: ['image'], pageSize: 50 })));
expect(view.router.state.location.search).toContain('pageSize=20');
```

增加断言：目录模式改变清除客户和会话；单聊/群聊改变清除会话并回第 1 页；游标页倒序合并后正序去重；关注成功使三个 query key 失效；归档 40301 阻断整页；资料缺失不阻断中右栏。

- [ ] **Step 2: 运行并确认 RED**

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/features/conversation-global/customer-conversation-page.test.tsx src/benchmark/page-registry.test.tsx`

Expected: FAIL，专用页面不存在且 route 仍使用通用组件。

- [ ] **Step 3: 实现固定页大小和三层查询**

```tsx
const customerPageSize = 50 as const, conversationPageSize = 20 as const, messagePageSize = 50 as const;
const directoryQuery = useQuery({ queryKey: ['corp', access.corp.id, 'customer-directory', customerMode, customerKeyword, customerPage], queryFn: () => api.customerDirectory!({ mode: customerMode, keyword: customerKeyword, page: customerPage, pageSize: customerPageSize }) });
const conversationsQuery = useQuery({ queryKey: ['corp', access.corp.id, 'customer-conversations', customerId, conversationMode, page], queryFn: () => api.customerConversations!({ customerId: customerId!, mode: conversationMode, page, pageSize: conversationPageSize }), enabled: customerId !== null });
const detailQuery = useInfiniteQuery({ queryKey: ['corp', access.corp.id, 'customer-detail', customerId, conversationId, messageKeyword, messageTypes, messageDate], initialPageParam: '', queryFn: ({ pageParam }) => api.customerDetail!({ customerId: customerId!, conversationId: conversationId!, keyword: messageKeyword, messageTypes, date: messageDate, pageSize: messagePageSize, ...(pageParam ? { before: pageParam } : {}) }), getNextPageParam: (last) => last.hasMore && last.nextBefore ? last.nextBefore : undefined, enabled: customerId !== null && conversationId !== null });
```

非法枚举、非正整数和错误会话格式用 `replace:true` 规范化。移动端抽屉不写 URL；业务选择和筛选写 URL。

- [ ] **Step 4: 将 route 切到专用页面**

```tsx
import { CustomerConversationPage } from '../features/conversation-global/customer-conversation-page';
'/chat/v2-customer': <CustomerConversationPage api={conversationGlobalApi} />,
```

registry 测试断言客户 route 类型为 `CustomerConversationPage`；群聊 route 保持原样。

- [ ] **Step 5: 运行 GREEN 并提交**

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/features/conversation-global/customer-conversation-page.test.tsx src/features/conversation-global/customer-conversation-directory.test.tsx src/features/conversation-global/customer-conversation-list.test.tsx src/features/conversation-global/customer-conversation-detail.test.tsx src/features/conversation-global/conversation-global-api.test.ts src/benchmark/page-registry.test.tsx`

Expected: PASS。

```powershell
git add web/apps/dashboard/src/features/conversation-global/customer-conversation-page.tsx web/apps/dashboard/src/features/conversation-global/customer-conversation-page.test.tsx web/apps/dashboard/src/benchmark/page-registry.tsx web/apps/dashboard/src/benchmark/page-registry.test.tsx
git commit -m "feat: orchestrate customer conversation workspace"
```

---

### Task 11: 三栏布局与响应式回归

**Files:**
- Modify: `web/apps/dashboard/src/styles/index.css`
- Create: `web/apps/dashboard/src/styles/customer-conversation-layout.test.ts`

**Interfaces:**
- Consumes: Tasks 7–10 的 customer class。
- Produces: 宽屏三栏、目录抽屉、双栏和单栏规则。

- [ ] **Step 1: 写样式契约失败测试**

```ts
expect(css).toMatch(/\.customer-conversation-workspace\s*\{[^}]*grid-template-columns:\s*288px 360px minmax\(560px,\s*1fr\)/s);
expect(css).toMatch(/\.customer-conversation-workspace\s*\{[^}]*height:\s*calc\(100dvh/s);
expect(css).toMatch(/\.customer-conversation-scroll\s*\{[^}]*overflow-y:\s*auto/s);
expect(css).toMatch(/@media\s*\(max-width:\s*1599px\)[\s\S]*\.customer-conversation-directory[^}]*position:\s*fixed/s);
expect(css).toMatch(/@media\s*\(max-width:\s*768px\)[\s\S]*grid-template-columns:\s*1fr/s);
```

- [ ] **Step 2: 运行并确认 RED**

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/styles/customer-conversation-layout.test.ts`

Expected: FAIL。

- [ ] **Step 3: 实现连续三栏和独立滚动**

```css
.customer-conversation-workspace {
  display: grid;
  grid-template-columns: 288px 360px minmax(560px, 1fr);
  height: calc(100dvh - 72px - 32px);
  min-height: 620px;
  overflow: hidden;
  border: 1px solid var(--dashboard-border);
  border-radius: 14px;
  background: #fff;
}
.customer-conversation-pane { min-width: 0; min-height: 0; overflow: hidden; }
.customer-conversation-pane + .customer-conversation-pane { border-left: 1px solid var(--dashboard-border); }
.customer-conversation-scroll { min-height: 0; overflow-y: auto; overscroll-behavior: contain; }
```

1599px 以下目录为 fixed 抽屉且右栏至少 520px；1199px 以下允许会话栏折叠；768px 以下为分步单栏。补齐遮罩、焦点、选中、禁用、告警和分页 hover 对比度。

- [ ] **Step 4: 运行 GREEN 并提交**

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/styles/customer-conversation-layout.test.ts src/features/conversation-global/customer-conversation-page.test.tsx`

Expected: PASS。

```powershell
git add web/apps/dashboard/src/styles/index.css web/apps/dashboard/src/styles/customer-conversation-layout.test.ts
git commit -m "style: add responsive customer conversation workspace"
```

---

### Task 12: 全量验证、Docker 更新与浏览器验收

**Files:**
- Verify: Tasks 1–11 的全部文件
- Evidence: `D:\workspace\mochat-go\output\customer-conversation-workspace-20260819\`

**Interfaces:**
- Consumes: 完整客户工作台。
- Produces: 自动化、MariaDB、容器健康和四种尺寸证据。

- [ ] **Step 1: 格式化并运行目标测试**

```powershell
gofmt -w internal/dashboard/work_message_customer.go internal/dashboard/work_message_customer_test.go internal/store/work_message_customer.go internal/store/work_message_customer_test.go internal/store/work_message_customer_integration_test.go
go test ./internal/dashboard ./internal/store ./internal/server ./internal/migration -run 'Test.*(WorkMessageCustomer|CustomerConversation|CustomerDirectory|CustomerDetail|DashboardPageCatalog|0142)' -count=1
corepack pnpm --filter @mochat/dashboard exec vitest run src/features/conversation-global/customer-conversation-page.test.tsx src/features/conversation-global/customer-conversation-directory.test.tsx src/features/conversation-global/customer-conversation-list.test.tsx src/features/conversation-global/customer-conversation-detail.test.tsx src/features/conversation-global/conversation-global-api.test.ts src/benchmark/page-registry.test.tsx src/styles/customer-conversation-layout.test.ts
```

Expected: Go 与 Vitest 目标测试 PASS。

- [ ] **Step 2: 使用隔离 DSN 运行真实 MariaDB 测试**

```powershell
if (-not $env:MOCHAT_GO_MYSQL_INTEGRATION_DSN) { throw '请从受保护的本地测试环境设置 MOCHAT_GO_MYSQL_INTEGRATION_DSN' }
go test ./internal/store -run 'TestWorkMessageCustomerWorkspaceMariaDB' -count=1 -v
```

Expected: PASS，不允许 SKIP；覆盖去重、状态互斥、群关系、游标和跨 corp。

- [ ] **Step 3: 运行全量门禁**

```powershell
corepack pnpm --filter @mochat/dashboard test
corepack pnpm --filter @mochat/dashboard typecheck
corepack pnpm --filter @mochat/dashboard build
go test ./internal/dashboard ./internal/store ./internal/server ./internal/migration -count=1
```

Expected: dashboard tests PASS、typecheck/build exit 0、Go 四包 PASS。既有无关失败必须记录完整命令与测试名，不得写成通过。

- [ ] **Step 4: 记录部署前状态并只重建 app**

```powershell
New-Item -ItemType Directory -Force -Path 'D:\workspace\mochat-go\output\customer-conversation-workspace-20260819' | Out-Null
docker compose -p mochat-go-desktop --env-file deploy/standalone/.env.local -f deploy/standalone/docker-compose.yml ps | Tee-Object -FilePath 'D:\workspace\mochat-go\output\customer-conversation-workspace-20260819\docker-before.txt'
docker volume ls --filter label=com.docker.compose.project=mochat-go-desktop | Tee-Object -FilePath 'D:\workspace\mochat-go\output\customer-conversation-workspace-20260819\volumes-before.txt'
docker compose -p mochat-go-desktop --env-file deploy/standalone/.env.local -f deploy/standalone/docker-compose.yml up -d --build --no-deps app
docker compose -p mochat-go-desktop --env-file deploy/standalone/.env.local -f deploy/standalone/docker-compose.yml ps | Tee-Object -FilePath 'D:\workspace\mochat-go\output\customer-conversation-workspace-20260819\docker-after.txt'
docker volume ls --filter label=com.docker.compose.project=mochat-go-desktop | Tee-Object -FilePath 'D:\workspace\mochat-go\output\customer-conversation-workspace-20260819\volumes-after.txt'
```

Expected: app healthy/running，MySQL、Redis 未重建，命名卷集合一致。

- [ ] **Step 5: 浏览器验收四种尺寸和关键交互**

使用 Browser 技能打开 `http://127.0.0.1:18080/chat/v2-customer`：

1. 2560×1440：三栏填满，列宽 288/360/剩余，无水平滚动。
2. 1440×900：客户目录为抽屉，消息栏不少于 520px。
3. 1066×1272：无文字竖排或不可用窄栏。
4. 390×844：客户 → 会话 → 详情分步单栏，可逐级返回。
5. 操作客户搜索、目录模式、有效/流失、客户分页、单聊/群聊、会话分页、整卡选择、消息类型、内容、日期、刷新、关注、加载更早消息和 URL 刷新恢复。
6. 群聊详情显示“非员工消息”和身份缺口，不把入站发送方写成所选客户。
7. 网络响应满足 `list.length <= total`，控制台无新增 error。

保存四种尺寸截图和交互结果 JSON；不提交含真实聊天内容的截图。

- [ ] **Step 6: 检查最终差异和数据保护**

```powershell
git diff --check
git status --short
git diff --stat
```

Expected: 无空白错误，只包含客户工作台实现和用户原有改动；`.env.local`、密钥、数据库导出不在暂存区。验收产生的代码修正运行对应测试后单独提交：

```powershell
git add internal/dashboard/work_message_customer.go internal/dashboard/work_message_customer_test.go internal/store/work_message_customer.go internal/store/work_message_customer_test.go internal/store/work_message_customer_integration_test.go web/apps/dashboard/src/features/conversation-global/customer-conversation-directory.tsx web/apps/dashboard/src/features/conversation-global/customer-conversation-directory.test.tsx web/apps/dashboard/src/features/conversation-global/customer-conversation-list.tsx web/apps/dashboard/src/features/conversation-global/customer-conversation-list.test.tsx web/apps/dashboard/src/features/conversation-global/customer-conversation-detail.tsx web/apps/dashboard/src/features/conversation-global/customer-conversation-detail.test.tsx web/apps/dashboard/src/features/conversation-global/customer-conversation-page.tsx web/apps/dashboard/src/features/conversation-global/customer-conversation-page.test.tsx web/apps/dashboard/src/styles/customer-conversation-layout.test.ts
git add -p web/apps/dashboard/src/features/conversation-global/conversation-global-api.ts web/apps/dashboard/src/features/conversation-global/conversation-global-api.test.ts web/apps/dashboard/src/benchmark/page-registry.tsx web/apps/dashboard/src/benchmark/page-registry.test.tsx web/apps/dashboard/src/styles/index.css internal/server/server.go internal/server/server_test.go cmd/mochat-go/main.go internal/dashboard/dashboard_page_catalog.json internal/dashboard/dashboard_access_guard_test.go internal/migration/migration_test.go
git commit -m "fix: complete customer conversation acceptance"
```

---

## 完成定义

- `/chat/v2-customer` 使用专用 `CustomerConversationPage`，不再渲染通用概览与详情抽屉。
- 客户目录、单聊/群聊、消息详情、统计、关注和状态来自真实后端合同。
- 客户去重分页、有效/流失互斥、列表与总数同源，并通过真实 MariaDB 测试。
- 群聊客户关系经过服务端验证，入站消息不冒充所选客户。
- 四种目标尺寸全部完成浏览器验收。
- dashboard 全量测试、类型检查、构建和相关 Go 包通过；既有失败明确记录。
- app、MySQL、Redis 健康，现有 D 盘命名卷保持不变。
