# 群聊会话圆弧式工作台 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 `/chat/v2-group` 从通用会话卡片页改造成真实数据驱动的“客户群目录 → 群消息 → 群资料”三栏工作台，并修复现有群消息被归入“群聊 0”的数据归一化问题。

**Architecture:** 后端先在共享归档查询层统一有效群 ID，再以五个只读接口分别提供群目录、群资料、群消息、群成员和筛选选项；归档写入旁表保存新消息的原始发送人身份。前端新增专用 `GroupConversationPage` 管理 URL 和五类查询，复用现有消息内容渲染器，宽屏显示三列，普通桌面和移动端将目录或资料降级为抽屉。

**Tech Stack:** Go 1.25、MariaDB/MySQL、React 19、TypeScript 5.9、TanStack Query 5、React Router 7、Vitest 2、Testing Library、pnpm、Docker Compose。

## Global Constraints

- 目标仅限 `/chat/v2-group`；不得修改菜单导航、应用顶部 banner、企业切换、全局搜索以及其他会话页面的布局。
- 群、成员、统计、消息、风险、超时和筛选选项必须来自 MoChat 的真实接口与存储；不得添加静态业务数据。
- 参考产品有而 MoChat 未接入的能力必须以 capability 显示“未接入”或“数据受限”，不得显示为 `0`，不得提供无效按钮。
- 外部群目录固定每页 50 条；群成员固定每页 50 条；群消息每次固定读取 50 条并使用游标加载更早记录。
- tenant、corp、user 和员工数据范围只取服务端 principal；忽略客户端提交的范围字段。
- 群消息有效群 ID 统一为 `to_user_id > 0 ? to_user_id : room_id`，仅适用于 `to_user_type=2`；两者都为 0 时不得生成“群聊 0”。
- 新归档消息保存 `from` 和 `roomid`；历史记录不能猜测发送人，无法解析时显示“群成员”。
- 内部群同步、单群下载、群备注编辑和群成员管理不在本轮范围。
- 查询输入只维护草稿，点击“查询”或按回车后才请求后端；日期和离散选项可在一次明确操作后查询。
- 保留工作树中已有的全局消息、员工会话、客户会话、概览及其他用户改动；每次只暂存当前任务列出的文件。
- 新增迁移使用 `0143_group_conversation_workspace`，不得改写已有或未提交的 `0140`、`0141`、`0142` 迁移。
- Docker 只重建 `app`；禁止 `down -v`、`down --volumes`、删除 MySQL/Redis 容器数据或更改 D 盘命名卷。
- 主验收尺寸为 2560×1440，同时回归 1440×900、1066×1272 和 390×844；任何尺寸不得出现工作台横向滚动。

---

## 文件结构

### 新建文件

- `internal/dashboard/work_message_room.go`：群工作台领域类型、参数解析、五个 GET handler 和存储接口。
- `internal/dashboard/work_message_room_test.go`：handler 授权、固定页大小、参数、错误码和响应测试。
- `internal/store/work_message_room.go`：群目录、资料、消息、成员、筛选和权限范围 SQL。
- `internal/store/work_message_room_test.go`：SQL 归一化、筛选、游标、成员身份和空态测试。
- `internal/store/work_message_room_integration_test.go`：MariaDB 方言、真实分表、权限和跨企业测试。
- `deploy/standalone/migrations/0143_group_conversation_workspace.up.sql` 与 `.down.sql`：发送人身份旁表及群页面 API 资源。
- `web/apps/dashboard/src/features/conversation-global/group-conversation-directory.tsx` 与 `.test.tsx`：群目录、页签、查询、分页和已解散入口。
- `web/apps/dashboard/src/features/conversation-global/group-conversation-filter-drawer.tsx` 与 `.test.tsx`：员工、客户、群分组和状态筛选。
- `web/apps/dashboard/src/features/conversation-global/group-conversation-messages.tsx` 与 `.test.tsx`：群消息头、筛选、统计和时间线。
- `web/apps/dashboard/src/features/conversation-global/group-conversation-profile.tsx` 与 `.test.tsx`：群资料、成员、风险、超时和 capability。
- `web/apps/dashboard/src/features/conversation-global/group-conversation-page.tsx` 与 `.test.tsx`：URL、五类查询、抽屉和三列编排。
- `web/apps/dashboard/src/styles/group-conversation-layout.test.ts`：宽屏和响应式样式契约。

### 修改文件

- `internal/store/auto_tag.go` 与 `internal/store/auto_tag_test.go`：共享归档 union 增加有效群 ID。
- `internal/dashboard/work_message_archive_sync_cron.go` 与 `_test.go`：保留 bridge 的发送人和房间微信 ID 合同。
- `internal/store/work_message_archive_sync.go` 与 `_test.go`：归档消息和身份旁表同流程写入。
- `internal/server/server.go` 与 `internal/server/server_test.go`：注册五个群工作台路由。
- `cmd/mochat-go/main.go`：注入群工作台 handlers。
- `internal/dashboard/dashboard_page_catalog.json` 与 `internal/dashboard/dashboard_access_guard_test.go`：群页面 API 资源目录。
- `internal/migration/migration_test.go`：0143 迁移合同。
- `web/apps/dashboard/src/features/conversation-global/conversation-global-api.ts` 与 `.test.ts`：群工作台类型、严格解析和五个 API 方法。
- `web/apps/dashboard/src/benchmark/page-registry.tsx` 与 `.test.tsx`：群路由切换到专用页面。
- `web/apps/dashboard/src/styles/index.css`：三栏、抽屉、稳定控件和四档响应式样式。

---

### Task 1: 共享归档查询统一有效群 ID

**Files:**
- Modify: `internal/store/auto_tag.go`
- Modify: `internal/store/auto_tag_test.go`
- Regression: `internal/store/work_message_customer_test.go`

**Interfaces:**
- Consumes: 10 张 `mc_work_message_1` 至 `mc_work_message_10` 的 `to_user_type`、`to_user_id`、`room_id`。
- Produces: 共享归档 union 中规范化后的 `to_user_id`，以及可用于统计异常的原始 `room_id`。

- [ ] **Step 1: 写群 ID 回退和零值丢弃失败测试**

在 `internal/store/auto_tag_test.go` 增加断言，要求 `workMessageArchiveUnionSQL` 的每个分表分支使用相同表达式：

```go
func TestWorkMessageArchiveUnionUsesEffectiveRoomID(t *testing.T) {
  query := workMessageArchiveUnionSQL()
  effective := "CASE WHEN to_user_type = 2 THEN COALESCE(NULLIF(to_user_id, 0), NULLIF(room_id, 0), 0) ELSE to_user_id END AS to_user_id"
  if got := strings.Count(query, effective); got != dashboard.WorkMessageArchiveMessageTableCount {
    t.Fatalf("effective room expression count=%d query=%s", got, query)
  }
}
```

补一条 store 级测试：`to_user_type=2,to_user_id=0,room_id=3001` 归入 3001；两者都为 0 的记录不进入正常会话结果。回归客户会话群模式使用同一规范化字段。

- [ ] **Step 2: 运行并确认 RED**

Run: `go test ./internal/store -run 'TestWorkMessageArchiveUnionUsesEffectiveRoomID|TestWorkMessageCustomer.*Room' -count=1`

Expected: FAIL，当前 union 仍直接暴露 `to_user_id`，或者客户群关联仍要求原始 `to_user_id`。

- [ ] **Step 3: 在唯一共享入口实现规范化**

每个归档分表分支输出：

```sql
CASE
  WHEN to_user_type = 2
  THEN COALESCE(NULLIF(to_user_id, 0), NULLIF(room_id, 0), 0)
  ELSE to_user_id
END AS to_user_id,
room_id AS source_room_id
```

调用者只使用规范化后的 `to_user_id` 关联客户群；群值为 0 时从正常列表排除。不要在全局消息、员工会话、客户会话和群聊工作台各自复制不同 CASE。

- [ ] **Step 4: 运行 GREEN 和相关回归**

Run: `go test ./internal/store -run 'TestWorkMessageArchiveUnion|TestWorkMessageGlobal|TestWorkMessageStaff|TestWorkMessageCustomer' -count=1`

Expected: PASS，已有非群会话 ID 不变，群消息可用 `room_id` 回退。

- [ ] **Step 5: 提交**

```powershell
git add internal/store/auto_tag.go internal/store/auto_tag_test.go internal/store/work_message_customer_test.go
git commit -m "fix: normalize archived group conversation ids"
```

---

### Task 2: 新消息发送人身份旁表与归档写入

**Files:**
- Create: `deploy/standalone/migrations/0143_group_conversation_workspace.up.sql`
- Create: `deploy/standalone/migrations/0143_group_conversation_workspace.down.sql`
- Modify: `internal/migration/migration_test.go`
- Modify: `internal/dashboard/work_message_archive_sync_cron_test.go`
- Modify: `internal/store/work_message_archive_sync.go`
- Modify: `internal/store/work_message_archive_sync_test.go`

**Interfaces:**
- Consumes: `dashboard.WorkMessageArchiveMessage.From`、`RoomID`、`MsgID`、`Seq`。
- Produces: `mochat_go_work_message_participant_identity(corp_id,msgid,seq,sender_wx_id,room_wx_id)`；身份行和消息分表行在同一事务提交，消息重复同步时幂等更新。

- [ ] **Step 1: 写迁移和写入失败测试**

迁移测试必须验证唯一键、两个查询索引和五个群页面 GET 资源。归档 store 测试要求同一次 upsert 执行身份写入：

```go
mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO mochat_go_work_message_participant_identity`)).
  WithArgs(7, "msg-room-1", int64(88), "wmKh001", "wrRoom001").
  WillReturnResult(sqlmock.NewResult(1, 1))
```

bridge 解析测试继续断言 JSON 中的 `from`、`roomid` 完整进入 `WorkMessageArchiveMessage`，防止未来被裁掉。

- [ ] **Step 2: 运行并确认 RED**

Run: `go test ./internal/migration ./internal/dashboard ./internal/store -run 'Test.*0143|TestParseWorkMessageArchiveBridgeMessage.*Room|TestUpsertWorkMessageArchive.*Identity' -count=1`

Expected: FAIL，旁表与身份 upsert 尚不存在。

- [ ] **Step 3: 创建幂等迁移**

`0143_group_conversation_workspace.up.sql` 创建：

```sql
CREATE TABLE IF NOT EXISTS `mochat_go_work_message_participant_identity` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `corp_id` int unsigned NOT NULL,
  `msgid` varchar(128) NOT NULL,
  `seq` bigint NOT NULL,
  `sender_wx_id` varchar(128) NOT NULL DEFAULT '',
  `room_wx_id` varchar(128) NOT NULL DEFAULT '',
  `created_at` datetime NOT NULL,
  `updated_at` datetime NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_mg_wmpi_corp_msg_seq` (`corp_id`,`msgid`,`seq`),
  KEY `idx_mg_wmpi_corp_sender` (`corp_id`,`sender_wx_id`),
  KEY `idx_mg_wmpi_corp_room` (`corp_id`,`room_wx_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

同一 up migration 以 `dashboard.chat.v2_group` 权限码插入以下资源，使用 `NOT EXISTS` 保证幂等：

```text
GET /dashboard/workMessage/roomDirectory
GET /dashboard/workMessage/roomProfile
GET /dashboard/workMessage/roomMessages
GET /dashboard/workMessage/roomMembers
GET /dashboard/workMessage/roomFilterOptions
```

down migration 先精确删除这五个资源，再 `DROP TABLE IF EXISTS mochat_go_work_message_participant_identity`，不得删除页面权限本身。

- [ ] **Step 4: 身份 upsert 与消息写入使用同一事务**

在 `upsertWorkMessageArchiveWithExecutor` 中，成功解析消息并得到分表后执行：

```sql
INSERT INTO mochat_go_work_message_participant_identity
  (corp_id,msgid,seq,sender_wx_id,room_wx_id,created_at,updated_at)
VALUES (?,?,?,?,?,NOW(),NOW())
ON DUPLICATE KEY UPDATE
  sender_wx_id=VALUES(sender_wx_id),
  room_wx_id=VALUES(room_wx_id),
  updated_at=NOW()
```

仅 `From` 或 `RoomID` 至少一个非空时写旁表。将公开的 `UpsertWorkMessageArchive` 改为 `BeginTx` → 调用 `upsertWorkMessageArchiveWithExecutor(tx, ...)` → `Commit`，错误或 panic 路径 `Rollback`；helper 继续接收现有 `archiveDBTX`，方便 sqlmock 单测。身份行在消息分表 insert 之前 upsert，两步中任一步失败都回滚，不能留下“消息成功但身份静默丢失”或孤立身份行。测试明确 `ExpectBegin`、两次 `ExpectExec`、`ExpectCommit`，并为身份失败和消息失败各写一个 `ExpectRollback` 用例。

- [ ] **Step 5: 运行 GREEN 并提交**

Run: `go test ./internal/migration ./internal/dashboard ./internal/store -run 'Test.*0143|TestParseWorkMessageArchiveBridgeMessage|TestUpsertWorkMessageArchive' -count=1`

Expected: PASS，重复消息身份更新幂等，非群消息行为不变。

```powershell
git add deploy/standalone/migrations/0143_group_conversation_workspace.up.sql deploy/standalone/migrations/0143_group_conversation_workspace.down.sql internal/migration/migration_test.go internal/dashboard/work_message_archive_sync_cron_test.go internal/store/work_message_archive_sync.go internal/store/work_message_archive_sync_test.go
git commit -m "feat: persist archived group sender identity"
```

---

### Task 3: 群工作台后端领域合同与 Handler

**Files:**
- Create: `internal/dashboard/work_message_room.go`
- Create: `internal/dashboard/work_message_room_test.go`

**Interfaces:**
- Consumes: `resolveAuthorized`、`principalCorpID`、`workMessageConversationPermissionKey`、`parseWorkMessageTypes`、`DataPermission`。
- Produces: 五个 store 接口、过滤器、响应结构和 `AutoTagHandler` methods。

- [ ] **Step 1: 写 principal、参数和错误码失败测试**

测试至少覆盖：

- 客户端 `corpId=999` 不得覆盖 principal 的 corp；
- `roomMode` 只允许 `active|dissolved`；
- `pageSize` 目录/成员只接受 50，消息只接受 50；
- `roomId` 必须是正整数；
- `mode` 成员只允许 `all|employee|customer|left`；
- `kind` 只允许 `employee|customer|group`；
- 非法日期、消息类型、游标返回 400；
- 归档不可用返回现有 40301；权限外群统一返回 404；store 能力未注入返回 501。

代表性授权测试：

```go
func TestWorkMessageRoomDirectoryUsesPrincipalScope(t *testing.T) {
  authorizer := &recordingAuthorizer{accessSet: true, access: AccessContext{
    CorpID: 7, DataPermission: DataPermissionDepartment, DeptEmployeeIDs: []int{9, 10},
  }}
  store := &fakeAutoTagStore{roomDirectory: WorkMessageRoomDirectoryPage{Rooms: []WorkMessageRoomDirectoryItem{}, Page: 1, PageSize: 50}}
  handler := NewAutoTagHandler(store, staticAdminCache("7-9"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, authorizer)
  req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/workMessage/roomDirectory?roomMode=active&page=1&pageSize=50&corpId=999", nil)
  rec := httptest.NewRecorder()
  handler.WorkMessageRoomDirectory(rec, req)
  if rec.Code != http.StatusOK { t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String()) }
  if store.roomDirectoryFilter.CorpID != 7 || !reflect.DeepEqual(store.roomDirectoryFilter.EmployeeIDs, []int{9, 10}) { t.Fatalf("filter=%#v", store.roomDirectoryFilter) }
}
```

- [ ] **Step 2: 运行并确认 RED**

Run: `go test ./internal/dashboard -run 'TestWorkMessageRoom' -count=1`

Expected: FAIL，群工作台类型和 handlers 尚不存在。

- [ ] **Step 3: 定义精确过滤器和响应合同**

核心类型必须明确，不以 `map[string]any` 代替：

```go
type WorkMessageRoomDirectoryFilter struct {
  TenantID, CorpID, UserID int
  Mode string
  Keyword string
  EmployeeIDs, CustomerIDs, RoomGroupIDs, RoomStatuses []int
  RestrictEmployeeIDs bool
  Page, PageSize int
}
type WorkMessageRoomProfileFilter struct { TenantID, CorpID, UserID, RoomID int; RestrictEmployeeIDs bool; EmployeeIDs []int }
type WorkMessageRoomMessageFilter struct { TenantID, CorpID, UserID, RoomID int; Keyword, Date, Before string; MessageTypes []int; RestrictEmployeeIDs bool; EmployeeIDs []int; PageSize int }
type WorkMessageRoomMemberFilter struct { TenantID, CorpID, UserID, RoomID int; Mode string; RestrictEmployeeIDs bool; EmployeeIDs []int; Page, PageSize int }
type WorkMessageRoomOptionFilter struct { TenantID, CorpID, UserID int; Kind, Keyword string; RestrictEmployeeIDs bool; EmployeeIDs []int; Page, PageSize int }
```

定义 JSON 响应：

- `WorkMessageRoomDirectoryPage{Rooms, Counts, Total, Page, PageSize, UnmatchedRoomMessages, Limitations, Capabilities}`；
- `Counts.External int`、`Counts.Dissolved int`、`Counts.Internal *int`，内部群必须为 `nil`；
- `WorkMessageRoomProfile{Room, Stats, MemberCounts, RiskRecords, TimeoutRecords, Capabilities}`；
- `WorkMessageRoomMessagePage{Messages, NextBefore, HasMore, Capabilities}`；
- `WorkMessageRoomMemberPage{Members, Counts, Total, Page, PageSize}`；
- `WorkMessageRoomOptionPage{Options, Total, Page, PageSize}`。

消息复用 `WorkMessageConversationMessage` 或现有等价消息 DTO；风险和超时 DTO 只返回页面所需的记录 ID、员工、类型、状态、发生时间和摘要，不暴露规则内部 JSON。

- [ ] **Step 4: 定义独立 store 接口和 handler 顺序**

```go
type WorkMessageRoomDirectoryStore interface { WorkMessageRoomDirectory(context.Context, WorkMessageRoomDirectoryFilter) (WorkMessageRoomDirectoryPage, error) }
type WorkMessageRoomProfileStore interface { WorkMessageRoomProfile(context.Context, WorkMessageRoomProfileFilter) (WorkMessageRoomProfile, error) }
type WorkMessageRoomMessageStore interface { WorkMessageRoomMessages(context.Context, WorkMessageRoomMessageFilter) (WorkMessageRoomMessagePage, error) }
type WorkMessageRoomMemberStore interface { WorkMessageRoomMembers(context.Context, WorkMessageRoomMemberFilter) (WorkMessageRoomMemberPage, error) }
type WorkMessageRoomOptionStore interface { WorkMessageRoomFilterOptions(context.Context, WorkMessageRoomOptionFilter) (WorkMessageRoomOptionPage, error) }
```

五个 handler 都遵循：认证与 RBAC → principal corp → 会话存档状态（目录基础资料可返回但带消息能力限制）→ 参数规范化 → 对应 store 类型断言 → store → envelope。非 all 数据权限设置 `RestrictEmployeeIDs=true` 并复制 `DeptEmployeeIDs`；空权限范围必须得到空目录或 404，不能退化成全企业。

- [ ] **Step 5: 运行 GREEN 并提交**

Run: `go test ./internal/dashboard -run 'TestWorkMessageRoom' -count=1`

Expected: PASS，所有成功响应数组为 `[]` 而不是 `null`。

```powershell
git add internal/dashboard/work_message_room.go internal/dashboard/work_message_room_test.go
git commit -m "feat: define group conversation handlers"
```

---

### Task 4: 群目录和筛选选项真实查询

**Files:**
- Create: `internal/store/work_message_room.go`
- Create: `internal/store/work_message_room_test.go`

**Interfaces:**
- Consumes: `mc_work_room`、`mc_work_employee`、`mc_work_contact_room`、`mc_work_contact`、`mc_work_room_group`、共享归档 union。
- Produces: `WorkMessageRoomDirectory`、`WorkMessageRoomFilterOptions`。

- [ ] **Step 1: 写目录口径失败测试**

使用 `sqlmock` 和纯函数测试覆盖：

- 没有归档消息的真实客户群仍出现在目录并标记 `archived=false`；
- 目录主表始终是 `mc_work_room`，不能从消息 union 反推群列表；
- active 使用 `room.deleted_at IS NULL`，dissolved 使用 `room.deleted_at IS NOT NULL`；
- 最新消息和统计使用规范化群 ID；
- 员工筛选同时匹配群主、当前群内员工和归档员工；
- 客户筛选匹配当前或历史群成员；
- 分组与状态为同类 OR、异类 AND；
- keyword 只匹配真实群名；
- 列表与 total 共用同一 where；
- `unmatchedRoomMessages` 只统计 `to_user_type=2` 且规范化 ID 为 0 的可见消息；
- employee/customer/group 选项都只来自当前用户可见群。

- [ ] **Step 2: 运行并确认 RED**

Run: `go test ./internal/store -run 'TestWorkMessageRoomDirectory|TestWorkMessageRoomFilterOptions' -count=1`

Expected: FAIL，新 store methods 尚不存在。

- [ ] **Step 3: 实现可见群公共子查询**

在 `work_message_room.go` 提供内部 builder：

```go
func (s *MySQLStore) visibleRoomScope(ctx context.Context, filter roomScopeFilter) (query string, args []any, available bool, err error)
```

公共范围必须：

1. `room.corp_id=?`；
2. all 权限不额外限制员工；
3. restricted 权限只允许群主、`mc_work_contact_room.employee_id` 或规范化归档消息 `work_employee_id` 命中允许员工的群；
4. 允许空 `EmployeeIDs` 时返回 `SELECT ... WHERE 1=0`，不得漏掉限制；
5. 所有关联表同时校验 corp 或经已校验的 room 关联，避免跨企业相同 ID。

- [ ] **Step 4: 实现目录聚合与选项**

目录项至少返回：`id,name,ownerName,memberCount,status,groupId,groupName,createdAt,latestMessage,latestMessageAt,archived`。名称为空时 store 返回空字符串，中文“未命名客户群”由前端展示；无消息时不伪造时间。

`Counts.Internal` 保持 `nil`，capability 包含：

```go
{Key: "internalRoomArchive", Available: false, Reason: "当前系统未接入内部群发现与会话归档"}
{Key: "singleRoomDownload", Available: false, Reason: "当前系统未提供单群会话下载"}
```

选项每页固定 50，并返回稳定数字 ID、名称、头像/父级信息；员工选项按组织信息构建树所需字段，客户和分组选项保持扁平。

- [ ] **Step 5: 运行 GREEN 并提交**

Run: `go test ./internal/store -run 'TestWorkMessageRoomDirectory|TestWorkMessageRoomFilterOptions' -count=1`

Expected: PASS。

```powershell
git add internal/store/work_message_room.go internal/store/work_message_room_test.go
git commit -m "feat: query group conversation directory"
```

---

### Task 5: 群资料、风险和超时记录

**Files:**
- Modify: `internal/store/work_message_room.go`
- Modify: `internal/store/work_message_room_test.go`
- Reference only: `internal/store/risk_behavior.go`
- Reference only: `internal/store/timeout_warning.go`

**Interfaces:**
- Consumes: 可见群公共范围、`mc_work_room`、归档 union、`mochat_go_risk_records`、`mochat_go_timeout_records`。
- Produces: `WorkMessageRoomProfile`。

- [ ] **Step 1: 写群资料与记录失败测试**

覆盖：

- `roomId` 不存在、跨 corp、权限外都返回领域 `ErrWorkMessageRoomNotFound`；handler 映射 404；
- 消息统计不受前端关键词、类型和日期影响；
- `communicationDays` 按群内可见归档消息日期去重；
- `messageTotal=customerSent+employeeSent`；
- 风险/超时只取与当前群和可访问员工对应的最近 10 条；
- 风险或超时表不存在时 capability 为 unavailable，不把数据库错误吞成空数组；
- 真正无记录时返回空数组和 available capability。

- [ ] **Step 2: 运行并确认 RED**

Run: `go test ./internal/store -run 'TestWorkMessageRoomProfile' -count=1`

Expected: FAIL，profile 查询尚未实现。

- [ ] **Step 3: 实现独立资料查询**

先通过可见群范围获取基础资料，再用同一规范化群 ID 聚合消息。群状态、群主、客户群分组、建群时间、公告和成员计数都从真实表读取。风险和超时使用现有 store 的表名/状态定义，不复制另一套业务判定；通过 `employeeId:2:roomId` 或现有规范化会话字段关联，并再次限制 `work_employee_id`。

能力缺口必须是结构化数据：

```go
type WorkMessageCapability struct {
  Key string `json:"key"`
  Available bool `json:"available"`
  Reason string `json:"reason,omitempty"`
}
```

不可用能力返回 `null` 统计或空记录加 capability，不能返回貌似真实的 0。

- [ ] **Step 4: 运行 GREEN 并提交**

Run: `go test ./internal/store -run 'TestWorkMessageRoomProfile' -count=1`

Expected: PASS。

```powershell
git add internal/store/work_message_room.go internal/store/work_message_room_test.go
git commit -m "feat: query group conversation profile"
```

---

### Task 6: 群消息游标、发送人身份和群成员

**Files:**
- Modify: `internal/store/work_message_room.go`
- Modify: `internal/store/work_message_room_test.go`
- Create: `internal/store/work_message_room_integration_test.go`

**Interfaces:**
- Consumes: 共享归档 union、身份旁表、`mc_work_contact_room`、`mc_work_contact`、`mc_work_employee`。
- Produces: `WorkMessageRoomMessages`、`WorkMessageRoomMembers`。

- [ ] **Step 1: 写消息与成员失败测试**

消息测试覆盖：

- 先验证 room 可见，再查消息；
- 最近 50 条按 `(msg_data_time,seq,msgid)` 倒序取数，响应单页转正序；
- `before` 是服务端签名/编码的稳定复合游标，不使用页码或仅时间；
- 关键词只查 `content_text`，类型采用已有真实类型映射，日期使用 `[dayStart, nextDay)`；
- 多页合并不会重复边界消息；
- 身份旁表优先以 `corp_id+msgid+seq` 关联员工或客户；
- 历史无旁表时员工发送仍使用归档员工姓名，入站显示“群成员”并返回 `historicalGroupSenderIdentity=false`；
- 消息群 ID 为 0 时不被任意 room 接收。

成员测试覆盖：

- `all` 只显示当前成员，`left` 只显示已退出成员；
- `employee` 与 `customer` 使用 `mc_work_contact_room.type` 的现有语义，不按名字猜测；
- `owner_id` 对应成员标记群主；
- 员工超出数据范围时不泄露姓名头像；
- total 和各模式 count 与列表同源；
- 无成员属于同步异常，响应 capability/limitation，而不是伪造成员。

- [ ] **Step 2: 运行并确认 RED**

Run: `go test ./internal/store -run 'TestWorkMessageRoomMessages|TestWorkMessageRoomMembers' -count=1`

Expected: FAIL，methods 尚未实现。

- [ ] **Step 3: 实现消息和成员查询**

消息过滤顺序固定为：corp → 可见 room → 规范化 room ID → 员工范围 → keyword/type/date → before。游标解码失败返回领域参数错误；SQL 参数化，不拼接关键词、日期、ID 或游标值。

发送人解析优先级：

1. 身份旁表 `sender_wx_id` 命中 `mc_work_employee.wx_user_id`；
2. 命中当前或历史 `mc_work_contact_room.wx_user_id` / `mc_work_contact.wx_external_userid`；
3. outbound 使用归档行 `work_employee_id`；
4. inbound 降级为“群成员”。

成员响应至少包含：`id,identityId,name,avatar,kind,owner,status,joinAt,leftAt`。`kind` 只允许 `employee|customer`，`status` 只允许 `active|left`。

- [ ] **Step 4: 建立真实 MariaDB 集成夹具**

`TestWorkMessageRoomWorkspaceMariaDB` 使用独立临时 schema，创建最小真实字段集、10 张消息分表、两家 corp、正常/解散/未存档群、在职/退出成员、风险和超时记录，并验证：

- `to_user_id=0,room_id=501` 归入 501；
- 目录包含无消息群；
- 权限用户只能看到允许员工相关群；
- 跨 corp 相同 room ID 不泄露；
- 消息游标无重无漏；
- 新身份和历史降级都符合合同。

集成测试只在 `MOCHAT_GO_MYSQL_INTEGRATION_DSN` 设置时运行，否则明确 skip。

- [ ] **Step 5: 运行 GREEN 并提交**

Run: `go test ./internal/store -run 'TestWorkMessageRoom' -count=1`

Expected: PASS。

若已有本地集成 DSN，再运行：

```powershell
if (-not $env:MOCHAT_GO_MYSQL_INTEGRATION_DSN) { throw 'MOCHAT_GO_MYSQL_INTEGRATION_DSN 未设置' }
go test ./internal/store -run 'TestWorkMessageRoomWorkspaceMariaDB' -count=1 -v
```

Expected: PASS，不修改现有业务 schema。

```powershell
git add internal/store/work_message_room.go internal/store/work_message_room_test.go internal/store/work_message_room_integration_test.go
git commit -m "feat: query group messages and members"
```

---

### Task 7: 注册路由、注入 handler 与 RBAC 目录

**Files:**
- Modify: `internal/server/server.go`
- Modify: `internal/server/server_test.go`
- Modify: `cmd/mochat-go/main.go`
- Modify: `internal/dashboard/dashboard_page_catalog.json`
- Modify: `internal/dashboard/dashboard_access_guard_test.go`
- Modify: `internal/migration/migration_test.go`

**Interfaces:**
- Consumes: Task 3 的五个 `AutoTagHandler` methods。
- Produces: 五个可访问的 `/dashboard/workMessage/room*` GET 路由和 `dashboard.chat.v2_group` API 资源。

- [ ] **Step 1: 写路由和资源失败测试**

在 `internal/server/server_test.go` 分别注入 recording handler，断言 method/path 精确匹配；POST、错误路径和 handler 未注入时不得误命中。access guard 测试断言五个 URL 都归属 `dashboard.chat.v2_group`，不能复用 `dashboard.chat.v2_all`。

- [ ] **Step 2: 运行并确认 RED**

Run: `go test ./internal/server ./internal/dashboard ./internal/migration -run 'Test.*WorkMessageRoom|Test.*Group.*Resource|Test.*0143' -count=1`

Expected: FAIL，server fields、options、routes 或目录资源尚未完整注册。

- [ ] **Step 3: 注册五个独立 option 和 route**

在 server struct、Option、ServeHTTP 分派和 route catalog 中使用一致名称：

```text
WithWorkMessageRoomDirectoryHandler
WithWorkMessageRoomProfileHandler
WithWorkMessageRoomMessagesHandler
WithWorkMessageRoomMembersHandler
WithWorkMessageRoomFilterOptionsHandler
```

`cmd/mochat-go/main.go` 用同一个 `autoTag` 实例注入五个 `http.HandlerFunc`。保持现有 customer/staff/global 注册顺序，不改变它们的 handler。

- [ ] **Step 4: 运行 GREEN 并提交**

Run: `go test ./internal/server ./internal/dashboard ./internal/migration -run 'Test.*WorkMessageRoom|Test.*Group.*Resource|Test.*0143' -count=1`

Expected: PASS。

```powershell
git add internal/server/server.go internal/server/server_test.go cmd/mochat-go/main.go internal/dashboard/dashboard_page_catalog.json internal/dashboard/dashboard_access_guard_test.go internal/migration/migration_test.go
git commit -m "feat: register group conversation endpoints"
```

---

### Task 8: 前端群工作台 API 合同

**Files:**
- Modify: `web/apps/dashboard/src/features/conversation-global/conversation-global-api.ts`
- Modify: `web/apps/dashboard/src/features/conversation-global/conversation-global-api.test.ts`

**Interfaces:**
- Consumes: `ApiClient`、`ConversationMessage`、`ConversationCapability`。
- Produces: 五组精确 TypeScript 类型、严格解析器和 `ConversationGlobalApi` methods。

- [ ] **Step 1: 写请求序列化与严格解析失败测试**

请求测试精确断言：

```ts
await api.roomDirectory?.({ roomMode: 'active', keyword: '技术', employeeIds: [9], customerIds: [31], roomGroupIds: [4], roomStatuses: [0], page: 2, pageSize: 50 });
expect(client.request).toHaveBeenCalledWith('/workMessage/roomDirectory?roomMode=active&keyword=%E6%8A%80%E6%9C%AF&employeeIds=9&customerIds=31&roomGroupIds=4&roomStatuses=0&page=2&pageSize=50');

await api.roomMessages?.({ roomId: 3001, keyword: '报价', messageTypes: ['image', 'file'], date: '2026-08-19', pageSize: 50, before: 'cursor' });
expect(client.request).toHaveBeenCalledWith('/workMessage/roomMessages?roomId=3001&keyword=%E6%8A%A5%E4%BB%B7&messageTypes=image&messageTypes=file&date=2026-08-19&pageSize=50&before=cursor');
```

再分别让五个接口返回字段缺失、非法枚举、错误 pageSize、负 count 或 `list.length > total`，解析器必须抛出中文合同错误，不能以默认 0 掩盖后端问题。

- [ ] **Step 2: 运行并确认 RED**

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/features/conversation-global/conversation-global-api.test.ts`

Expected: FAIL，群工作台类型和 methods 尚不存在。

- [ ] **Step 3: 定义固定合同**

```ts
export type RoomDirectoryMode = 'active' | 'dissolved';
export type RoomDirectoryInput = { roomMode: RoomDirectoryMode; keyword: string; employeeIds: readonly number[]; customerIds: readonly number[]; roomGroupIds: readonly number[]; roomStatuses: readonly number[]; page: number; pageSize: 50 };
export type RoomDirectoryItem = { id: number; name: string; ownerName: string; memberCount: number; status: number; groupId: number; groupName: string; createdAt: string; latestMessage: string; latestMessageAt: string; archived: boolean };
export type RoomDirectoryPage = { rooms: readonly RoomDirectoryItem[]; counts: { external: number; dissolved: number; internal: null }; total: number; page: number; pageSize: 50; unmatchedRoomMessages: number; limitations: readonly { key: string; reason: string }[]; capabilities: readonly ConversationCapability[] };
export type RoomProfileInput = { roomId: number };
export type RoomStats = { communicationDays: number; messageTotal: number; customerSent: number; employeeSent: number };
export type RoomProfile = { room: RoomProfileSummary; stats: RoomStats; memberCounts: RoomMemberCounts; riskRecords: readonly RoomRecord[]; timeoutRecords: readonly RoomRecord[]; capabilities: readonly ConversationCapability[] };
export type RoomMessagesInput = { roomId: number; keyword: string; messageTypes: readonly string[]; date: string; pageSize: 50; before?: string };
export type RoomMessagesPage = { messages: readonly ConversationMessage[]; nextBefore: string; hasMore: boolean; capabilities: readonly ConversationCapability[] };
export type RoomMemberMode = 'all' | 'employee' | 'customer' | 'left';
export type RoomMembersInput = { roomId: number; mode: RoomMemberMode; page: number; pageSize: 50 };
export type RoomMembersPage = { members: readonly RoomMember[]; counts: RoomMemberCounts; total: number; page: number; pageSize: 50 };
export type RoomFilterKind = 'employee' | 'customer' | 'group';
export type RoomFilterOptionsInput = { kind: RoomFilterKind; keyword: string; page: number; pageSize: 50 };
export type RoomFilterOptionsPage = { options: readonly RoomFilterOption[]; total: number; page: number; pageSize: 50 };
```

为 `ConversationGlobalApi` 增加 `roomDirectory?`、`roomProfile?`、`roomMessages?`、`roomMembers?`、`roomFilterOptions?`。所有 query 使用 `URLSearchParams` 和重复键，数组先去重并保持用户选择顺序，不发送空字符串。

- [ ] **Step 4: 运行 GREEN 并提交**

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/features/conversation-global/conversation-global-api.test.ts`

Expected: PASS。

```powershell
git add web/apps/dashboard/src/features/conversation-global/conversation-global-api.ts web/apps/dashboard/src/features/conversation-global/conversation-global-api.test.ts
git commit -m "feat: add group conversation api contracts"
```

---

### Task 9: 群目录与筛选抽屉

**Files:**
- Create: `web/apps/dashboard/src/features/conversation-global/group-conversation-directory.tsx`
- Create: `web/apps/dashboard/src/features/conversation-global/group-conversation-directory.test.tsx`
- Create: `web/apps/dashboard/src/features/conversation-global/group-conversation-filter-drawer.tsx`
- Create: `web/apps/dashboard/src/features/conversation-global/group-conversation-filter-drawer.test.tsx`

**Interfaces:**
- Consumes: `RoomDirectoryPage`、`RoomFilterOptionsPage` 和纯回调。
- Produces: 无请求副作用的受控目录、筛选草稿和提交事件。

- [ ] **Step 1: 写目录交互失败测试**

Testing Library 覆盖：

- 输入群名不会调用 `onSearchSubmit`，点击查询或按回车只调用一次；
- 刷新按钮位于查询按钮之后并仅调用 `onRefresh`；
- “内部群 · 未接入”不可切换且不调用 mode 回调；
- 点击卡片头像、名称、消息等整个主体都选择同一 room；
- 卡片内未来独立链接使用 `stopPropagation`；
- 目录固定显示 50 页大小口径，上一页/下一页边界不抖动；
- 已解散群入口显示真实 `counts.dissolved` 并切换 mode；
- `unmatchedRoomMessages>0` 显示紧凑告警；
- loading/error 只替换列表区，不卸载查询栏。

- [ ] **Step 2: 写筛选抽屉失败测试**

覆盖：打开后保留已应用条件；编辑只改变 draft；“查询”一次提交并关闭；“重置”提交空条件；遮罩、关闭按钮和 Escape 都关闭；相关员工选择器支持组织结构字段和已选区；客户、分组、状态均来自 props，不写死选项。

- [ ] **Step 3: 运行并确认 RED**

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/features/conversation-global/group-conversation-directory.test.tsx src/features/conversation-global/group-conversation-filter-drawer.test.tsx`

Expected: FAIL，组件尚不存在。

- [ ] **Step 4: 实现无数据请求组件**

目录组件不导入 API client、不直接读 URL。为卡片使用原生 `button type="button"` 或等价可访问语义，选中状态只切换 class/`aria-current`，不改变 transform、font-weight 导致尺寸变化。筛选抽屉使用 dialog 语义，打开时聚焦标题/首个控件，关闭后由页面恢复到触发按钮。

- [ ] **Step 5: 运行 GREEN 并提交**

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/features/conversation-global/group-conversation-directory.test.tsx src/features/conversation-global/group-conversation-filter-drawer.test.tsx`

Expected: PASS。

```powershell
git add web/apps/dashboard/src/features/conversation-global/group-conversation-directory.tsx web/apps/dashboard/src/features/conversation-global/group-conversation-directory.test.tsx web/apps/dashboard/src/features/conversation-global/group-conversation-filter-drawer.tsx web/apps/dashboard/src/features/conversation-global/group-conversation-filter-drawer.test.tsx
git commit -m "feat: build group conversation directory"
```

---

### Task 10: 群消息主列

**Files:**
- Create: `web/apps/dashboard/src/features/conversation-global/group-conversation-messages.tsx`
- Create: `web/apps/dashboard/src/features/conversation-global/group-conversation-messages.test.tsx`
- Reuse: `web/apps/dashboard/src/features/conversation-global/conversation-message-content.tsx`

**Interfaces:**
- Consumes: 当前 `RoomDirectoryItem`、`RoomStats`、`RoomMessagesPage`、消息搜索草稿和回调。
- Produces: 受控消息筛选、统计区、日期分组时间线和加载更早事件。

- [ ] **Step 1: 写消息列失败测试**

覆盖：

- 无选中群显示引导空态；
- 选中未存档群显示“该群暂无已归档消息”，不显示接口错误；
- 内容输入不搜索，点击查询或回车才调用一次；刷新在查询旁边且不清空条件；
- 类型只展示全部、文字、图片、语音、视频、文件、链接；
- 日期、类型每次操作只触发一次回调；
- 沟通天数、消息总数、客户发送、员工发送显示后端值；
- 消息按日期分组，媒体全部交给 `ConversationMessageContent`；
- “加载更早消息”只在 `hasMore` 出现，loading 时禁用并保留宽度；
- 历史未知发送人显示“群成员”，capability 只显示紧凑提示；
- 不出现会话下载或群备注编辑按钮。

- [ ] **Step 2: 运行并确认 RED**

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/features/conversation-global/group-conversation-messages.test.tsx`

Expected: FAIL，组件尚不存在。

- [ ] **Step 3: 实现稳定主列**

组件用 memoized 日期分组，不在 render 中改变消息顺序；父组件传入的 messages 视为只读。加载更早后由页面按消息 ID 去重，组件只渲染。查询栏高度、按钮最小宽度、类型控件高度在 loading/selected/hover 状态一致，不使用缩放和滤镜。

- [ ] **Step 4: 运行 GREEN 并提交**

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/features/conversation-global/group-conversation-messages.test.tsx`

Expected: PASS。

```powershell
git add web/apps/dashboard/src/features/conversation-global/group-conversation-messages.tsx web/apps/dashboard/src/features/conversation-global/group-conversation-messages.test.tsx
git commit -m "feat: build group message timeline"
```

---

### Task 11: 群资料整列与成员分页

**Files:**
- Create: `web/apps/dashboard/src/features/conversation-global/group-conversation-profile.tsx`
- Create: `web/apps/dashboard/src/features/conversation-global/group-conversation-profile.test.tsx`

**Interfaces:**
- Consumes: `RoomProfile`、`RoomMembersPage`、成员 mode/page 回调和重试回调。
- Produces: 群概况、统计、成员、风险、超时和 capability 的整列视图。

- [ ] **Step 1: 写资料列失败测试**

覆盖：

- 群名称、建群时间、群主、分组、状态、成员数均来自 profile；
- 统计与消息主列值一致；
- 成员四种 mode 显示真实 count，切换只请求成员接口；
- 群主、员工、客户、已退出使用短标签；
- 风险和超时分别渲染最近记录，真空数组显示“暂无…记录”；
- capability unavailable 显示紧凑“能力受限”列表，`internal` 为 null 时不能渲染 0；
- profile 失败不卸载左列和中列；成员失败只替换成员区；
- 抽屉模式的关闭按钮、遮罩和 Escape 生效。

- [ ] **Step 2: 运行并确认 RED**

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/features/conversation-global/group-conversation-profile.test.tsx`

Expected: FAIL，组件尚不存在。

- [ ] **Step 3: 实现独立滚动资料列**

资料组件不请求数据。风险、超时和 capability 使用紧凑 section，不放顶部介绍卡；成员列表分页保持容器最小高度，切换 mode 时不让整列宽度或头部按钮位移。普通桌面抽屉和宽屏固定列复用同一内容组件。

- [ ] **Step 4: 运行 GREEN 并提交**

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/features/conversation-global/group-conversation-profile.test.tsx`

Expected: PASS。

```powershell
git add web/apps/dashboard/src/features/conversation-global/group-conversation-profile.tsx web/apps/dashboard/src/features/conversation-global/group-conversation-profile.test.tsx
git commit -m "feat: build group conversation profile"
```

---

### Task 12: 页面 URL、五类查询和路由切换

**Files:**
- Create: `web/apps/dashboard/src/features/conversation-global/group-conversation-page.tsx`
- Create: `web/apps/dashboard/src/features/conversation-global/group-conversation-page.test.tsx`
- Modify: `web/apps/dashboard/src/benchmark/page-registry.tsx`
- Modify: `web/apps/dashboard/src/benchmark/page-registry.test.tsx`

**Interfaces:**
- Consumes: `ConversationGlobalApi` 的五个 room methods 和 Tasks 9–11 的受控组件。
- Produces: `/chat/v2-group` 专用工作台、规范 URL 和列级请求生命周期。

- [ ] **Step 1: 写 URL 与查询编排失败测试**

覆盖：

- 初始只请求目录；目录成功后，URL 有可访问 `roomId` 才请求 profile/messages/members；
- 没有 `roomId` 时选择当前页首群使用 `replace`，不污染返回历史；
- `roomKeyword` 和 `messageKeyword` 只在提交后写 URL；
- 目录筛选改变后 `roomPage=1`，若旧 room 不在结果中清除/替换 `roomId`；
- 切换 room 保留目录条件，清空消息 keyword/type/date/before；
- message type/date 改变不重复请求 profile 或 members；
- 加载更早只追加 messages，以 ID 去重并保持正序；
- 非法枚举、负页码、非法 ID/日期使用 `replace` 规范化；
- 内部群点击不请求后端；
- 任一列失败不卸载其他列；
- 1440 和更窄视图资料抽屉遮罩、Escape 和焦点恢复生效；
- registry 对 `/chat/v2-group` 渲染 `GroupConversationPage`，不再渲染 `ConversationGlobalPage fixedConversationType="room"`。

- [ ] **Step 2: 运行并确认 RED**

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/features/conversation-global/group-conversation-page.test.tsx src/benchmark/page-registry.test.tsx`

Expected: FAIL，页面尚不存在且 registry 仍走通用卡片页。

- [ ] **Step 3: 实现 URL 规范化和查询键**

业务 URL 参数只使用设计文档定义的：`roomMode,roomKeyword,roomPage,roomId,employeeIds,customerIds,roomGroupIds,roomStatuses,messageKeyword,messageTypes,messageDate`。抽屉开关、成员局部页码和消息 `before` 不写 URL。

TanStack Query key 必须分别包含自身所有后端参数：

```ts
['room-directory', normalizedDirectoryInput]
['room-profile', roomId]
['room-messages', normalizedMessageInput]
['room-members', roomId, memberMode, memberPage]
['room-filter-options', kind, keyword, page]
```

输入 draft 使用 local state；不要在 `onChange` 更新 search params。刷新使用对应 query 的 `refetch`，不全局 invalidate 所有会话。

- [ ] **Step 4: 实现响应式抽屉状态**

宽度由 CSS media query 决定，React 只管理目录/资料 drawer 开关。小屏选择群后关闭目录 drawer；关闭 drawer 后焦点返回“群列表”或“群信息”按钮。不要监听 resize 后重建整个页面树。

- [ ] **Step 5: 运行 GREEN 并提交**

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/features/conversation-global/group-conversation-page.test.tsx src/benchmark/page-registry.test.tsx`

Expected: PASS。

```powershell
git add web/apps/dashboard/src/features/conversation-global/group-conversation-page.tsx web/apps/dashboard/src/features/conversation-global/group-conversation-page.test.tsx web/apps/dashboard/src/benchmark/page-registry.tsx web/apps/dashboard/src/benchmark/page-registry.test.tsx
git commit -m "feat: route group conversation workspace"
```

---

### Task 13: 三栏布局、无抖动控件和响应式合同

**Files:**
- Modify: `web/apps/dashboard/src/styles/index.css`
- Create: `web/apps/dashboard/src/styles/group-conversation-layout.test.ts`

**Interfaces:**
- Consumes: 群工作台组件 class names。
- Produces: 2560 宽屏三列、普通桌面资料抽屉、平板双抽屉、手机分步单列。

- [ ] **Step 1: 写 CSS 合同失败测试**

读取 `index.css` 并断言：

- `min-width:1600px` 使用 `grid-template-columns: 320px minmax(720px, 1fr) 300px`；
- 1200–1599 隐藏固定 profile rail 并启用资料 drawer；
- 769–1199 目录和资料都可作为 drawer；
- `max-width:768px` 单列；
- 工作台根节点 `min-width:0`、`overflow-x:hidden`；
- 三列内容各自 `overflow-y:auto`；
- hover/selected/page button 不使用 `transform:scale`、`filter:blur`、负 margin 或会改变尺寸的 border 宽度；
- 查询和刷新按钮有固定高度与最小宽度；loading 时不改变工具栏 grid/flex basis。

- [ ] **Step 2: 运行并确认 RED**

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/styles/group-conversation-layout.test.ts`

Expected: FAIL，群工作台样式尚不存在。

- [ ] **Step 3: 实现四档布局**

宽屏根布局：

```css
@media (min-width: 1600px) {
  .group-conversation-workspace {
    display: grid;
    grid-template-columns: 320px minmax(720px, 1fr) 300px;
  }
}
```

页面高度使用现有内容区可用高度变量/布局，不硬编码整个屏幕高度。三列之间使用一致 12–16px 间隔和 1px 稳定边框；顶部不新增介绍框。消息主列必须 `min-width:0` 并吃满剩余宽度，右侧资料在宽屏占完整列高。

- [ ] **Step 4: 运行组件、样式和类型回归**

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/features/conversation-global/group-conversation-*.test.tsx src/styles/group-conversation-layout.test.ts src/benchmark/page-registry.test.tsx`

Expected: PASS。

Run: `corepack pnpm --filter @mochat/dashboard typecheck`

Expected: PASS，无新增 TypeScript 错误。

- [ ] **Step 5: 提交**

```powershell
git add web/apps/dashboard/src/styles/index.css web/apps/dashboard/src/styles/group-conversation-layout.test.ts
git commit -m "style: align group conversation workspace"
```

---

### Task 14: 全量验证、Docker 交付和浏览器验收

**Files:**
- Verify only: all files above
- Modify only if results require a scoped fix: the exact failing task files
- Update: `docs/superpowers/specs/2026-08-19-group-conversation-yuanhu-workspace-design.md` only if implementation intentionally changes an approved contract

**Interfaces:**
- Consumes: 完成后的后端、前端、迁移和现有 Docker Compose。
- Produces: 可在 `http://127.0.0.1:18080/chat/v2-group` 验收的服务，不改变数据卷。

- [ ] **Step 1: 运行后端定向测试**

```powershell
go test ./internal/dashboard ./internal/store ./internal/server ./internal/migration -run 'TestWorkMessageRoom|TestWorkMessageArchiveUnion|TestUpsertWorkMessageArchive|Test.*0143' -count=1
```

Expected: PASS。

- [ ] **Step 2: 运行前端定向测试、类型检查和生产构建**

```powershell
corepack pnpm --filter @mochat/dashboard exec vitest run src/features/conversation-global/conversation-global-api.test.ts src/features/conversation-global/group-conversation-directory.test.tsx src/features/conversation-global/group-conversation-filter-drawer.test.tsx src/features/conversation-global/group-conversation-messages.test.tsx src/features/conversation-global/group-conversation-profile.test.tsx src/features/conversation-global/group-conversation-page.test.tsx src/styles/group-conversation-layout.test.ts src/benchmark/page-registry.test.tsx
corepack pnpm --filter @mochat/dashboard typecheck
corepack pnpm --filter @mochat/dashboard build
```

Expected: 三条命令均 PASS。

- [ ] **Step 3: 运行相邻会话回归**

```powershell
go test ./internal/dashboard ./internal/store ./internal/server -run 'TestWorkMessageGlobal|TestWorkMessageStaff|TestWorkMessageCustomer' -count=1
corepack pnpm --filter @mochat/dashboard exec vitest run src/features/conversation-global/conversation-global-page.test.tsx src/features/conversation-global/employee-conversation-page.test.tsx src/features/conversation-global/customer-conversation-page.test.tsx
```

Expected: PASS，群 ID 归一化未破坏全局、员工和客户会话。

- [ ] **Step 4: 检查 Docker 目标并仅重建 app**

先只读确认容器和命名卷：

```powershell
docker compose -p mochat-go-desktop --env-file deploy/standalone/.env.local -f deploy/standalone/docker-compose.yml ps
docker volume ls --filter label=com.docker.compose.project=mochat-go-desktop
```

确认目标后执行：

```powershell
docker compose -p mochat-go-desktop --env-file deploy/standalone/.env.local -f deploy/standalone/docker-compose.yml up -d --build --no-deps app
```

不得执行 `down`。随后验证：

```powershell
docker compose -p mochat-go-desktop --env-file deploy/standalone/.env.local -f deploy/standalone/docker-compose.yml ps
Invoke-WebRequest -UseBasicParsing http://127.0.0.1:18080/healthz | Select-Object StatusCode,Content
```

Expected: `app`、MySQL、Redis 均为健康/运行状态，healthz 为 200，迁移 0143 已应用，卷名与重建前一致。

- [ ] **Step 5: 用真实本地数据核验数据口径**

通过只读 SQL/API 断言：

- 当前 12 条 `SIM-MSG-*` 的 `to_user_id=0` 群消息按 `room_id` 进入 3001、3003、3005、3006；
- 页面/API 不再出现 `roomId=0` 或“群聊 0”；
- 外部群总数来自 `mc_work_room`，无消息群仍存在；
- 内部群返回 `null + unavailable capability`；
- 目录、profile、messages 的统计相互一致；
- 风险和超时记录有则展示真实记录，无则显示真实空态；
- 历史缺发送人身份的入站消息显示“群成员”，不伪造客户姓名。

若本地数据已变化，以同一时刻的只读 SQL 结果为基线，不写入模拟数据迎合断言。

- [ ] **Step 6: 浏览器四尺寸验收**

在已登录的本地页面依次验收：

1. 2560×1440：完整三列，`320px / 自适应主列 / 300px`，主列填满，右列占完整高度，无横向滚动；
2. 1440×900：左列 + 主列，群资料由抽屉打开，遮罩、关闭按钮和 Escape 均可关闭；
3. 1066×1272：目录和资料都能通过抽屉打开，选择群后消息可继续阅读；
4. 390×844：列表 → 消息 → 资料分步单列，返回路径完整。

每种尺寸都检查：群名草稿不自动请求、查询/刷新稳定、筛选抽屉、整卡选择、正常/已解散切换、分页、消息类型、日期、加载更早、成员模式、URL 刷新恢复、错误重试、hover/页码文字清晰、控制台无新增错误。

- [ ] **Step 7: 最终差异和提交检查**

```powershell
git diff --check
git status --short
git log --oneline -14
```

确认没有暂存/提交无关用户文件。若验收产生修复，返回对应 Task，重复该 Task 的 RED/GREEN 命令并使用该 Task 已列出的精确 `git add` 文件清单提交；不得使用 `git add .` 或临时通配占位。

完成后向用户报告：真实数据核验结果、能力缺口、测试命令、浏览器尺寸、容器健康状态和保留的卷；不要宣称未实际执行的测试已通过。

---

## 实施完成定义

- [ ] `/chat/v2-group` 已使用专用三栏群工作台，未修改菜单和顶部 banner。
- [ ] 目录以真实客户群为主对象，包含未存档群和已解散群，不按员工重复群。
- [ ] 所有 `to_user_id=0,room_id>0` 的群消息按有效群 ID 归一化，页面不出现“群聊 0”。
- [ ] 新归档群消息持久化发送人身份；历史身份缺失明确降级。
- [ ] 五个接口都执行 corp、RBAC 和员工数据范围校验，跨企业/越权 room 返回 404。
- [ ] 中间消息、右侧资料、成员、风险和超时均使用真实数据，并有列级 loading/error/empty 状态。
- [ ] 内部群、单群下载、历史发送人等缺口显示 unavailable capability，不伪装成 0 或可用按钮。
- [ ] 群名和消息内容只在提交后查询；目录/消息/成员固定页大小；消息游标无重无漏。
- [ ] 2560×1440 完整三列，另外三种尺寸按设计降级且无横向滚动、抖动或文字模糊。
- [ ] 定向测试、相邻回归、类型检查、生产构建和 MariaDB 集成测试均有真实执行证据。
- [ ] 只重建 Docker `app`；MySQL、Redis 和 D 盘数据卷保持不变且服务健康。
