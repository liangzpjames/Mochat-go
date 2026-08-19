# 会话轨迹元呼 AI 对标改造实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 `/chat/trajectory` 改造成基于真实归档数据的“员工—日期—分类统计—24 小时轨迹—详情抽屉”工作台。

**Architecture:** 复用员工目录和员工会话详情能力，新增一个服务端按日轨迹聚合接口；后端统一群消息有效目标 ID、自然日、企业和员工权限口径，前端只负责 URL 编排和语义化时间网格展示。页面拆成目录、统计时间轴和详情抽屉三个边界清晰的单元，并复用 `ConversationMessageContent`。

**Tech Stack:** Go 1.23、`net/http`、MariaDB/MySQL、React 19、TypeScript、TanStack Query、React Router、Vitest、Testing Library、CSS、Docker Compose。

## 全局约束

- 目标路由固定为 `/chat/trajectory`；不修改菜单导航、应用顶部 Banner、企业切换和全局搜索。
- 业务时区固定为 `Asia/Shanghai`；自然日范围为 `[00:00:00, 次日 00:00:00)`。
- 2560×1440 为主验收尺寸，同时回归 1440×900、1066×1272、390×844。
- 内部群聊显示 `unavailable` 和 `null`，不得显示成真实 0。
- 所有统计、活动、姓名和消息必须来自真实接口；禁止前端常量补数据。
- 外部群有效目标 ID 为 `COALESCE(NULLIF(to_user_id,0), NULLIF(room_id,0), 0)`。
- 列表、统计、轨迹和详情使用同一企业、员工权限、归档来源和日期口径。
- 行为修改测试先行；每个任务先确认 RED，再实现最小改动并确认 GREEN。
- 当前工作树有其他页面改动；提交时只暂存本任务列出的文件，不使用 `git add .`。
- Docker 只重建 `app`；禁止 `down -v`、`down --volumes` 和删除命名卷。

## 文件结构

### 后端

- `internal/dashboard/work_message_trajectory.go`：轨迹请求/响应类型、过滤器、权限检查和 handler。
- `internal/dashboard/work_message_trajectory_test.go`：handler 输入、权限、错误和响应契约测试。
- `internal/store/work_message_trajectory.go`：按日指标、小时活动和目标资料批量解析。
- `internal/store/work_message_trajectory_test.go`：纯函数、SQL 口径和空值降级测试。
- `internal/store/work_message_trajectory_integration_test.go`：MariaDB 真实分表、日期、聚合和权限集成测试。
- `internal/store/auto_tag.go`：共享有效群 ID 投影和过滤。
- `internal/store/work_message_test.go`、`internal/store/work_message_staff_test.go`：共享群 ID 回归测试。
- `internal/server/server.go`、`internal/server/server_test.go`、`cmd/mochat-go/main.go`：注册新 GET 路由。
- `deploy/standalone/migrations/0143_conversation_trajectory_rbac.up.sql`、`.down.sql`：轨迹页面 API 权限资源。
- `internal/migration/migration_test.go`：0143 迁移发现和资源契约。

### 前端

- `web/apps/dashboard/src/features/conversation-global/conversation-global-api.ts`：轨迹 API 类型、严格解析和请求序列化。
- `web/apps/dashboard/src/features/conversation-global/conversation-global-api.test.ts`：轨迹 API 契约测试。
- `web/apps/dashboard/src/features/conversation-global/conversation-trajectory-page.tsx`：URL、query key、级联状态和页面组合。
- `web/apps/dashboard/src/features/conversation-global/conversation-trajectory-directory.tsx`：员工目录和组织架构。
- `web/apps/dashboard/src/features/conversation-global/conversation-trajectory-timeline.tsx`：分类指标、24 小时行和活动卡。
- `web/apps/dashboard/src/features/conversation-global/conversation-trajectory-drawer.tsx`：当天消息游标分页和关闭行为。
- `web/apps/dashboard/src/features/conversation-global/conversation-trajectory-page.test.tsx`：页面交互和状态测试。
- `web/apps/dashboard/src/styles/conversation-trajectory-layout.test.ts`：宽屏、抽屉和响应式 CSS 契约。
- `web/apps/dashboard/src/styles/index.css`：轨迹页面样式。
- `docs/PROJECT_PROGRESS.zh-CN.md`：最终数据来源、测试和浏览器验收记录。

---

### Task 1：统一外部群有效目标 ID

**Files:**

- Modify: `internal/store/auto_tag.go`
- Modify: `internal/store/work_message_test.go`
- Modify: `internal/store/work_message_staff_test.go`

**Interfaces:**

- Produces: `workMessageEffectiveTargetIDSQL(alias string) string`
- Produces: 联合归档查询中的 `to_user_id` 已是规范化后的有效目标 ID。
- Consumes: `mc_work_message_*.to_user_type`、`to_user_id`、`room_id`。

- [ ] **Step 1：写外部群目标 ID 的失败测试**

在 `internal/store/work_message_test.go` 增加完整 SQL 契约测试：

```go
func TestWorkMessageEffectiveTargetIDSQL(t *testing.T) {
	want := "CASE WHEN COALESCE(wm.to_user_type, 0) = 2 THEN COALESCE(NULLIF(wm.to_user_id, 0), NULLIF(wm.room_id, 0), 0) ELSE COALESCE(wm.to_user_id, 0) END"
	if got := strings.Join(strings.Fields(workMessageEffectiveTargetIDSQL("wm")), " "); got != want {
		t.Fatalf("effective target SQL = %q, want %q", got, want)
	}
	if got := strings.Join(strings.Fields(workMessageEffectiveTargetIDSQL("")), " "); strings.Contains(got, "wm.") {
		t.Fatalf("unqualified target SQL = %q", got)
	}
}

func TestWorkMessageUnionNormalizesArchivedRoomID(t *testing.T) {
	query, _ := workMessageUnionSQL(7)
	normalized := strings.Join(strings.Fields(query), " ")
	want := strings.Join(strings.Fields(workMessageEffectiveTargetIDSQL("wm")+" AS to_user_id"), " ")
	if got := strings.Count(normalized, want); got != dashboard.WorkMessageArchiveMessageTableCount {
		t.Fatalf("effective target expression count=%d, want %d", got, dashboard.WorkMessageArchiveMessageTableCount)
	}
}

func TestWorkMessageUserWhereUsesEffectiveRoomTarget(t *testing.T) {
	where, args := workMessageUserBaseWhere(dashboard.WorkMessageUserFilter{
		ToUserType: 2, ToUserID: 3001,
	}, "wm.")
	if !strings.Contains(where, "NULLIF(wm.room_id, 0)") || !reflect.DeepEqual(args, []any{2, 3001}) {
		t.Fatalf("where=%q args=%#v", where, args)
	}
}
```

在 `internal/store/work_message_staff_test.go` 增加 `staffMessageWhere` 对 `conversationId=1001:2:3001` 继续按规范化 union 的 `wm.to_user_id = ?` 过滤的断言；Task 3 的 MariaDB 集成测试负责证明原始 `room_id=3001` 消息能被详情和轨迹同时读取。

- [ ] **Step 2：运行测试确认 RED**

Run: `go test ./internal/store -run 'TestWorkMessageGroupTarget|TestStaffDetail.*Room' -count=1`

Expected: FAIL；当前联合查询只投影和过滤原始 `to_user_id`，回退群消息仍是 0 或详情查不到。

- [ ] **Step 3：实现共享有效目标表达式**

在 `internal/store/auto_tag.go` 增加：

```go
func workMessageEffectiveTargetIDSQL(alias string) string {
	alias = strings.TrimSuffix(strings.TrimSpace(alias), ".")
	if alias != "" { alias += "." }
	return `CASE WHEN COALESCE(` + alias + `to_user_type, 0) = 2
		THEN COALESCE(NULLIF(` + alias + `to_user_id, 0), NULLIF(` + alias + `room_id, 0), 0)
		ELSE COALESCE(` + alias + `to_user_id, 0) END`
}
```

在 `workMessageTableSQLWithWhere` 中：

```go
effectiveTargetID := workMessageEffectiveTargetIDSQL("wm")
```

把投影的 `COALESCE(wm.to_user_id, 0) AS to_user_id` 改为 `effectiveTargetID + " AS to_user_id"`；`target_room.id` 的 join 也使用同一表达式。`workMessageUserBaseWhere` 在 `ToUserType == 2 && ToUserID > 0` 时使用有效目标表达式过滤，其他类型继续按原始 `to_user_id`。

- [ ] **Step 4：运行测试确认 GREEN**

Run: `go test ./internal/store -run 'TestWorkMessageGroupTarget|TestStaffDetail.*Room|TestWorkMessageConversation' -count=1`

Expected: PASS；外部群摘要和员工详情使用同一有效群 ID。

- [ ] **Step 5：提交**

```powershell
git add internal/store/auto_tag.go internal/store/work_message_test.go internal/store/work_message_staff_test.go
git commit -m "fix: normalize archived group conversation targets"
```

---

### Task 2：定义会话轨迹后端契约和 handler

**Files:**

- Create: `internal/dashboard/work_message_trajectory.go`
- Create: `internal/dashboard/work_message_trajectory_test.go`

**Interfaces:**

- Produces: `WorkMessageTrajectoryStore.TrajectoryDay(context.Context, WorkMessageTrajectoryFilter)`。
- Produces: `(*AutoTagHandler).WorkMessageTrajectoryDay(http.ResponseWriter, *http.Request)`。
- Consumes: 会话存档授权、`AccessContext.DeptEmployeeIDs`、当前企业和租户。

- [ ] **Step 1：写过滤器和 handler 失败测试**

测试表至少包含：

```go
func TestWorkMessageTrajectoryFilter(t *testing.T) {
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	req := authenticatedDashboardRequestForTest(http.MethodGet,
		"/dashboard/workMessage/trajectoryDay?employeeId=9&date=2026-08-19&conversationType=room", nil)
	rec := httptest.NewRecorder()

	filter, ok := workMessageTrajectoryFilter(rec, req, 10, 7, 1,
		AccessContext{DataPermission: DataPermissionDepartment, DeptEmployeeIDs: []int{9}}, now)
	if !ok { t.Fatalf("body=%s", rec.Body.String()) }
	if filter.EmployeeID != 9 || filter.Date != "2026-08-19" || filter.ConversationType != WorkMessageTrajectoryRoom {
		t.Fatalf("filter=%+v", filter)
	}
	if filter.StartAt != "2026-08-19 00:00:00" || filter.EndAt != "2026-08-20 00:00:00" {
		t.Fatalf("range=%s..%s", filter.StartAt, filter.EndAt)
	}
}
```

使用表驱动测试覆盖非法输入：

```go
for _, tc := range []struct {
	name, path string
	wantStatus int
}{
	{name: "missing employee", path: "/dashboard/workMessage/trajectoryDay?date=2026-08-19&conversationType=all", wantStatus: 400},
	{name: "invalid employee", path: "/dashboard/workMessage/trajectoryDay?employeeId=x&date=2026-08-19&conversationType=all", wantStatus: 400},
	{name: "invalid date", path: "/dashboard/workMessage/trajectoryDay?employeeId=9&date=2026-02-30&conversationType=all", wantStatus: 400},
	{name: "future date", path: "/dashboard/workMessage/trajectoryDay?employeeId=9&date=2026-08-20&conversationType=all", wantStatus: 400},
	{name: "invalid type", path: "/dashboard/workMessage/trajectoryDay?employeeId=9&date=2026-08-19&conversationType=internal-room", wantStatus: 400},
} {
	t.Run(tc.name, func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := authenticatedDashboardRequestForTest(http.MethodGet, tc.path, nil)
		_, ok := workMessageTrajectoryFilter(rec, req, 10, 7, 1, AccessContext{DataPermission: DataPermissionAll}, now)
		if ok || rec.Code != tc.wantStatus { t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String()) }
	})
}
```

独立 handler 用例覆盖越权员工 404、存档未开通 40301、store 未实现 501、store 错误 500；成功 fake store 返回 nil 数组时，解码响应并断言 `events/limitations/capabilities` 均为非 nil 空数组。

- [ ] **Step 2：运行测试确认 RED**

Run: `go test ./internal/dashboard -run 'TestWorkMessageTrajectory' -count=1`

Expected: FAIL，轨迹类型、过滤器和 handler 尚不存在。

- [ ] **Step 3：实现类型和请求过滤器**

创建 `internal/dashboard/work_message_trajectory.go`，定义：

```go
type WorkMessageTrajectoryType string

const (
	WorkMessageTrajectoryAll      WorkMessageTrajectoryType = "all"
	WorkMessageTrajectoryEmployee WorkMessageTrajectoryType = "employee"
	WorkMessageTrajectoryCustomer WorkMessageTrajectoryType = "customer"
	WorkMessageTrajectoryRoom     WorkMessageTrajectoryType = "room"
)

type WorkMessageTrajectoryFilter struct {
	TenantID, CorpID, UserID, EmployeeID int
	Date, StartAt, EndAt string
	ConversationType WorkMessageTrajectoryType
	RestrictEmployeeIDs bool
	EmployeeIDs []int
}

type WorkMessageTrajectoryMetric struct {
	Status string `json:"status"`
	SubjectTotal *int64 `json:"subjectTotal"`
	MessageTotal *int64 `json:"messageTotal"`
	Reason string `json:"reason,omitempty"`
}

type WorkMessageTrajectoryEvent struct {
	ID string `json:"id"`
	ConversationID string `json:"conversationId"`
	Hour string `json:"hour"`
	TargetType string `json:"targetType"`
	TargetID int `json:"targetId"`
	TargetName string `json:"targetName"`
	TargetAvatar string `json:"targetAvatar"`
	TargetStatus string `json:"targetStatus"`
	MessageTotal int64 `json:"messageTotal"`
	FirstMessageAt string `json:"firstMessageAt"`
	LastMessageAt string `json:"lastMessageAt"`
}

type WorkMessageTrajectoryDay struct {
	Employee struct { ID int `json:"id"`; Name string `json:"name"`; Avatar string `json:"avatar"` } `json:"employee"`
	Date string `json:"date"`
	Timezone string `json:"timezone"`
	Metrics map[string]WorkMessageTrajectoryMetric `json:"metrics"`
	Events []WorkMessageTrajectoryEvent `json:"events"`
	UnmatchedTargetMessages int64 `json:"unmatchedTargetMessages"`
	Limitations []WorkMessageStaffLimitation `json:"limitations"`
	Capabilities []WorkMessageCapability `json:"capabilities"`
}

type WorkMessageTrajectoryStore interface {
	TrajectoryDay(context.Context, WorkMessageTrajectoryFilter) (WorkMessageTrajectoryDay, error)
}
```

过滤器加载 `Asia/Shanghai`，生成开始/结束字符串；合法类型只有四个枚举；日期晚于上海当天返回 400。

- [ ] **Step 4：实现 handler 最小闭环**

`WorkMessageTrajectoryDay` 必须按顺序执行：方法校验 → 页面资源授权 → 企业解析 → 存档授权 → 参数解析 → 员工范围 404 → store 类型断言 → 查询 → 数组规范化 → 200 envelope。

内部群指标由存储层返回：

```go
WorkMessageTrajectoryMetric{
	Status: "unavailable", SubjectTotal: nil, MessageTotal: nil,
	Reason: "当前归档数据未提供内部群聊能力",
}
```

- [ ] **Step 5：运行 handler 测试确认 GREEN**

Run: `go test ./internal/dashboard -run 'TestWorkMessageTrajectory' -count=1`

Expected: PASS。

- [ ] **Step 6：提交**

```powershell
git add internal/dashboard/work_message_trajectory.go internal/dashboard/work_message_trajectory_test.go
git commit -m "feat: define conversation trajectory endpoint"
```

---

### Task 3：实现 MariaDB 按日指标和小时活动聚合

**Files:**

- Create: `internal/store/work_message_trajectory.go`
- Create: `internal/store/work_message_trajectory_test.go`
- Create: `internal/store/work_message_trajectory_integration_test.go`

**Interfaces:**

- Consumes: `dashboard.WorkMessageTrajectoryFilter`。
- Produces: `(*MySQLStore).TrajectoryDay(...)`。
- Reuses: `workMessageFilteredUnionSQLWithArchiveSourceState`、有效归档来源和 Task 1 的规范化 `to_user_id`。

- [ ] **Step 1：写聚合失败测试**

集成 fixture 插入同一员工在 2026-08-19 的数据：

```text
09:05 内部单聊 target=12
09:40 内部单聊 target=12
10:10 外部单聊 target=31
10:50 外部单聊 target=32
23:08 外部群 to_user_id=0 room_id=3001
2026-08-20 00:00 外部单聊 target=31（必须排除）
```

断言：内部单聊 `subjectTotal=1,messageTotal=2`；外部单聊 `2,2`；外部群 `1,1`；内部群两个值为 nil；活动为 09、10、10、23 四张；类型过滤 `room` 只过滤 events，不改变 metrics；群事件 ID 为 `1001:2:3001@2026-08-19T23`。

另测目标资料删除后的 `targetStatus=missing`、真实空群名为“未命名客户群”、无效目标消息进入 `unmatchedTargetMessages`、受限员工无结果。

- [ ] **Step 2：运行测试确认 RED**

Run: `go test ./internal/store -run 'TestTrajectoryDay' -count=1`

Expected: FAIL，`TrajectoryDay` 尚未实现。

- [ ] **Step 3：实现指标查询**

在 `TrajectoryDay` 中先选择有效归档源并构建只含所选员工和日期的 union。指标 SQL 使用一次条件聚合：

```sql
SELECT
  COUNT(DISTINCT CASE WHEN wm.to_user_type=0 AND wm.to_user_id>0 THEN wm.to_user_id END),
  COALESCE(SUM(CASE WHEN wm.to_user_type=0 AND wm.to_user_id>0 THEN 1 ELSE 0 END),0),
  COUNT(DISTINCT CASE WHEN wm.to_user_type=1 AND wm.to_user_id>0 THEN wm.to_user_id END),
  COALESCE(SUM(CASE WHEN wm.to_user_type=1 AND wm.to_user_id>0 THEN 1 ELSE 0 END),0),
  COUNT(DISTINCT CASE WHEN wm.to_user_type=2 AND wm.to_user_id>0 THEN wm.to_user_id END),
  COALESCE(SUM(CASE WHEN wm.to_user_type=2 AND wm.to_user_id>0 THEN 1 ELSE 0 END),0),
  COALESCE(SUM(CASE WHEN wm.to_user_id=0 THEN 1 ELSE 0 END),0)
FROM (<union SQL>) wm
```

用辅助函数生成 available 指标指针，不复用同一个指针：

```go
func trajectoryAvailableMetric(subjects, messages int64) dashboard.WorkMessageTrajectoryMetric {
	s, m := subjects, messages
	return dashboard.WorkMessageTrajectoryMetric{Status: "available", SubjectTotal: &s, MessageTotal: &m}
}
```

- [ ] **Step 4：实现小时活动查询和目标资料解析**

活动 SQL：

```sql
SELECT wm.to_user_type, wm.to_user_id, DATE_FORMAT(wm.msg_data_time, '%H'),
       COUNT(*), MIN(wm.msg_data_time), MAX(wm.msg_data_time)
FROM (<union SQL>) wm
WHERE wm.to_user_id > 0 AND (<conversation type predicate>)
GROUP BY wm.to_user_type, wm.to_user_id, DATE_FORMAT(wm.msg_data_time, '%H')
ORDER BY DATE_FORMAT(wm.msg_data_time, '%H'), MIN(wm.msg_data_time), wm.to_user_type, wm.to_user_id
```

聚合后按类型收集目标 ID，分别用三条 `IN (...)` 查询批量加载 `mc_work_employee`、`mc_work_contact`、`mc_work_room`。目标表没有行时 `targetStatus="missing"`；群行存在但名称为空时 `targetStatus="available"` 且名称为“未命名客户群”。

- [ ] **Step 5：实现员工信息、限制和能力**

员工必须通过 `corp_id + id + deleted_at IS NULL` 查询。响应固定：

```go
day.Date = filter.Date
day.Timezone = "Asia/Shanghai"
day.Capabilities = []dashboard.WorkMessageCapability{
	{Key: "internalGroup", Available: false, Reason: "当前归档数据未提供内部群聊能力"},
}
if day.UnmatchedTargetMessages > 0 {
	day.Limitations = append(day.Limitations, dashboard.WorkMessageStaffLimitation{
		Key: "unmatchedTargetMessages",
		Reason: fmt.Sprintf("有 %d 条消息无法关联会话对象", day.UnmatchedTargetMessages),
	})
}
```

- [ ] **Step 6：运行存储测试确认 GREEN**

Run: `go test ./internal/store -run 'TestTrajectoryDay' -count=1`

Expected: PASS，且 MariaDB 集成测试没有跳过 SQL 方言断言。

- [ ] **Step 7：检查查询计划再决定索引**

Run: 对 fixture 数据执行 `EXPLAIN` 版活动查询。

Expected: 使用现有 `idx_mc_work_message_*_global` 或等价范围索引。如果执行计划证明 `employeeId + date` 无法使用索引，再新建独立索引迁移；不能在无证据时把索引混入 0143 RBAC 迁移。

- [ ] **Step 8：提交**

```powershell
git add internal/store/work_message_trajectory.go internal/store/work_message_trajectory_test.go internal/store/work_message_trajectory_integration_test.go
git commit -m "feat: aggregate daily conversation trajectory"
```

---

### Task 4：注册路由并增加轨迹页面权限资源

**Files:**

- Modify: `internal/server/server.go`
- Modify: `internal/server/server_test.go`
- Modify: `cmd/mochat-go/main.go`
- Create: `deploy/standalone/migrations/0143_conversation_trajectory_rbac.up.sql`
- Create: `deploy/standalone/migrations/0143_conversation_trajectory_rbac.down.sql`
- Modify: `internal/migration/migration_test.go`

**Interfaces:**

- Produces: `GET /dashboard/workMessage/trajectoryDay`。
- Grants to: `dashboard.chat.trajectory`。
- Reuses: `staffDirectory`、`staffDetail`。

- [ ] **Step 1：写失败的路由和迁移测试**

在 server 路由表测试增加：

```go
{method: http.MethodGet, path: "/dashboard/workMessage/trajectoryDay?employeeId=9&date=2026-08-19", body: "trajectory day", route: "GET /dashboard/workMessage/trajectoryDay"}
```

在迁移测试断言最新版本为 `0143_conversation_trajectory_rbac`，up/down 都包含三个精确路径和 `dashboard.chat.trajectory`，且 up 含 `WHERE NOT EXISTS`。

- [ ] **Step 2：运行测试确认 RED**

Run: `go test ./internal/server ./internal/migration -run 'Trajectory|LatestMigration' -count=1`

Expected: FAIL，新路由 404，0143 未发现。

- [ ] **Step 3：注册 server option 和组合根**

在 `Server` 增加 `workMessageTrajectoryDay http.Handler`，并增加：

```go
func WithWorkMessageTrajectoryDayHandler(handler http.Handler) Option {
	return func(server *Server) { server.workMessageTrajectoryDay = handler }
}
```

路由 switch 和 mounted routes 分别加入精确 GET 路由；`cmd/mochat-go/main.go` 装配：

```go
compatserver.WithWorkMessageTrajectoryDayHandler(http.HandlerFunc(autoTag.WorkMessageTrajectoryDay))
```

- [ ] **Step 4：创建幂等 RBAC 迁移**

up 文件：

```sql
INSERT INTO `mochat_go_dashboard_permission_resources`
  (`permission_id`, `resource_type`, `http_method`, `path_pattern`, `scope_required`, `status`, `version`)
SELECT p.`id`, 'api', 'GET', resource_seed.`path_pattern`, 1, 1, 1
FROM `mochat_go_dashboard_permissions` p
INNER JOIN (
  SELECT '/dashboard/workMessage/staffDirectory' AS `path_pattern`
  UNION ALL SELECT '/dashboard/workMessage/trajectoryDay'
  UNION ALL SELECT '/dashboard/workMessage/staffDetail'
) resource_seed
WHERE p.`code` = 'dashboard.chat.trajectory'
  AND NOT EXISTS (
    SELECT 1 FROM `mochat_go_dashboard_permission_resources` existing
    WHERE existing.`permission_id` = p.`id`
      AND existing.`resource_type` = 'api'
      AND existing.`http_method` = 'GET'
      AND existing.`path_pattern` = resource_seed.`path_pattern`
  );
```

down 文件只删除该权限码下的三个 GET 资源，不删除页面权限和其他页面共享资源。

- [ ] **Step 5：运行路由和迁移测试确认 GREEN**

Run: `go test ./internal/server ./internal/migration -run 'Trajectory|LatestMigration' -count=1`

Expected: PASS。

- [ ] **Step 6：提交**

```powershell
git add internal/server/server.go internal/server/server_test.go cmd/mochat-go/main.go deploy/standalone/migrations/0143_conversation_trajectory_rbac.up.sql deploy/standalone/migrations/0143_conversation_trajectory_rbac.down.sql internal/migration/migration_test.go
git commit -m "feat: mount conversation trajectory resources"
```

---

### Task 5：增加前端轨迹 API 严格契约

**Files:**

- Modify: `web/apps/dashboard/src/features/conversation-global/conversation-global-api.ts`
- Modify: `web/apps/dashboard/src/features/conversation-global/conversation-global-api.test.ts`

**Interfaces:**

- Produces: `ConversationGlobalApi.trajectoryDay?(input)`。
- Produces: `ConversationTrajectoryDayInput`、`ConversationTrajectoryDay`。
- Consumes: Task 2 的 JSON 字段名。

- [ ] **Step 1：写请求序列化和解析失败测试**

```ts
it('loads a strict daily trajectory contract', async () => {
  const request = vi.fn(() => Promise.resolve({
    employee: { id: 9, name: '张三', avatar: '' },
    date: '2026-08-19', timezone: 'Asia/Shanghai',
    metrics: {
      internalSingle: { status: 'available', subjectTotal: 1, messageTotal: 2 },
      externalSingle: { status: 'available', subjectTotal: 2, messageTotal: 2 },
      internalGroup: { status: 'unavailable', subjectTotal: null, messageTotal: null, reason: '未接入' },
      externalGroup: { status: 'available', subjectTotal: 1, messageTotal: 1 },
    },
    events: [{ id: '9:2:3001@2026-08-19T23', conversationId: '9:2:3001', hour: '23', targetType: 'room', targetId: 3001, targetName: '客户群', targetAvatar: '', targetStatus: 'available', messageTotal: 1, firstMessageAt: '2026-08-19 23:08:00', lastMessageAt: '2026-08-19 23:08:00' }],
    unmatchedTargetMessages: 0, limitations: [], capabilities: [],
  }));
  const api = createConversationGlobalApi({ request });
  const result = await api.trajectoryDay?.({ employeeId: 9, date: '2026-08-19', conversationType: 'room' });
  expect(request).toHaveBeenCalledWith('/workMessage/trajectoryDay?employeeId=9&date=2026-08-19&conversationType=room');
  expect(result?.events[0]?.conversationId).toBe('9:2:3001');
});
```

增加表驱动的非法响应测试：

```ts
for (const [name, mutate] of [
  ['missing metrics', (value: any) => { delete value.metrics.externalGroup; }],
  ['available null', (value: any) => { value.metrics.internalSingle.subjectTotal = null; }],
  ['unavailable number', (value: any) => { value.metrics.internalGroup.messageTotal = 0; }],
  ['invalid hour', (value: any) => { value.events[0].hour = '24'; }],
  ['invalid target type', (value: any) => { value.events[0].targetType = 'internal-room'; }],
  ['negative total', (value: any) => { value.events[0].messageTotal = -1; }],
  ['empty id', (value: any) => { value.events[0].id = ''; }],
  ['wrong timezone', (value: any) => { value.timezone = 'UTC'; }],
] as const) {
  it(`rejects ${name}`, async () => {
    const value = validTrajectoryResponse();
    mutate(value);
    const api = createConversationGlobalApi({ request: () => Promise.resolve(value) });
    await expect(api.trajectoryDay?.({ employeeId: 9, date: '2026-08-19', conversationType: 'all' })).rejects.toThrow('会话轨迹接口返回了无效数据');
  });
}
```

`validTrajectoryResponse()` 在测试文件中返回 Step 1 的完整合法对象，避免每个用例复制大对象。

- [ ] **Step 2：运行测试确认 RED**

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/features/conversation-global/conversation-global-api.test.ts`

Expected: FAIL，轨迹类型和方法不存在。

- [ ] **Step 3：定义 TypeScript 类型**

```ts
export type ConversationTrajectoryType = 'all' | ConversationTargetType;
export type ConversationTrajectoryDayInput = { employeeId: number; date: string; conversationType: ConversationTrajectoryType };
export type ConversationTrajectoryMetric = { status: 'available' | 'unavailable'; subjectTotal: number | null; messageTotal: number | null; reason?: string };
export type ConversationTrajectoryEvent = {
  id: string; conversationId: string; hour: string;
  targetType: ConversationTargetType; targetId: number; targetName: string; targetAvatar: string;
  targetStatus: 'available' | 'missing'; messageTotal: number;
  firstMessageAt: string; lastMessageAt: string;
};
export type ConversationTrajectoryDay = {
  employee: { id: number; name: string; avatar: string };
  date: string; timezone: 'Asia/Shanghai';
  metrics: Record<'internalSingle' | 'externalSingle' | 'internalGroup' | 'externalGroup', ConversationTrajectoryMetric>;
  events: readonly ConversationTrajectoryEvent[];
  unmatchedTargetMessages: number;
  limitations: readonly { key: string; reason: string }[];
  capabilities: readonly ConversationCapability[];
};
```

给 `ConversationGlobalApi` 增加可选方法，避免破坏其他页面的窄 mock：

```ts
trajectoryDay?(input: ConversationTrajectoryDayInput): Promise<ConversationTrajectoryDay>;
```

- [ ] **Step 4：实现严格解析和请求**

解析器逐项验证。metric 规则必须显式：available 要求两个非负有限数字；unavailable 要求两个值为 null 且 reason 非空。请求方法：

```ts
async trajectoryDay(input) {
  const query = new URLSearchParams({
    employeeId: String(input.employeeId),
    date: input.date,
    conversationType: input.conversationType,
  });
  return parseTrajectoryDay(await client.request(`/workMessage/trajectoryDay?${query.toString()}`));
}
```

- [ ] **Step 5：运行 API 测试确认 GREEN**

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/features/conversation-global/conversation-global-api.test.ts`

Expected: PASS。

- [ ] **Step 6：提交**

```powershell
git add web/apps/dashboard/src/features/conversation-global/conversation-global-api.ts web/apps/dashboard/src/features/conversation-global/conversation-global-api.test.ts
git commit -m "feat: add conversation trajectory api contract"
```

---

### Task 6：构建员工目录和 URL 编排

**Files:**

- Create: `web/apps/dashboard/src/features/conversation-global/conversation-trajectory-directory.tsx`
- Modify: `web/apps/dashboard/src/features/conversation-global/conversation-trajectory-page.tsx`
- Modify: `web/apps/dashboard/src/features/conversation-global/conversation-trajectory-page.test.tsx`

**Interfaces:**

- Directory emits: `onModeChange`、`onKeywordSubmit`、`onDepartmentChange`、`onPageChange`、`onSelectEmployee`、`onRefresh`。
- Page owns: URL 规范化和员工目录 query key。
- Consumes: `api.staffDirectory`。

- [ ] **Step 1：写 URL 和目录行为失败测试**

测试必须覆盖：

```tsx
renderPage(api, '/chat/trajectory?date=2026-08-19');
expect((await screen.findByRole('tab', { name: '存档员工' })).getAttribute('aria-selected')).toBe('true');
expect(screen.getByRole('tab', { name: '组织架构' })).toBeTruthy();
fireEvent.change(screen.getByRole('textbox', { name: '员工名称' }), { target: { value: '张' } });
expect(api.staffDirectory).toHaveBeenCalledTimes(1);
fireEvent.click(screen.getByRole('button', { name: '查询员工' }));
await waitFor(() => expect(api.staffDirectory).toHaveBeenLastCalledWith(expect.objectContaining({ keyword: '张', mode: 'archived', pageSize: 50 })));
```

把测试 helper 从 `MemoryRouter` 改为可观察 URL 的 memory router：

```tsx
function renderPage(api: ConversationGlobalApi, entry = '/chat/trajectory') {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter([
    { path: '/chat/trajectory', element: <ConversationTrajectoryPage api={api} /> },
  ], { initialEntries: [entry] });
  const result = render(
    <QueryClientProvider client={queryClient}>
      <DashboardAccessProvider value={access}>
        <RouterProvider router={router} />
      </DashboardAccessProvider>
    </QueryClientProvider>,
  );
  return { ...result, router, queryClient };
}
```

增加 URL 表驱动断言：

```tsx
for (const entry of [
  '/chat/trajectory?date=bad&employeePage=0',
  '/chat/trajectory?date=2099-01-01&directoryMode=bad',
  '/chat/trajectory?date=2026-08-19&employeeId=-1',
]) {
  const { router } = renderPage(api, entry);
  await waitFor(() => expect(router.state.location.search).not.toMatch(/date=bad|2099-01-01|employeePage=0|directoryMode=bad|employeeId=-1/));
}
```

再用单独交互用例断言：选择员工保留 date/type 并清除 conversationId/eventHour；切换目录条件回第 1 页；刷新前后 `router.state.location.search` 完全相同。

- [ ] **Step 2：运行测试确认 RED**

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/features/conversation-global/conversation-trajectory-page.test.tsx`

Expected: FAIL，旧页面没有员工目录和新 URL 状态。

- [ ] **Step 3：实现 URL 纯函数**

在页面文件中定义并导出供测试使用：

```ts
export type TrajectoryDirectoryMode = 'archived' | 'organization';

export function clearTrajectoryDetail(params: URLSearchParams): URLSearchParams {
  return updateSearch(params, { conversationId: undefined, eventHour: undefined });
}

export function trajectoryToday(now = new Date()): string {
  return new Intl.DateTimeFormat('en-CA', { timeZone: 'Asia/Shanghai', year: 'numeric', month: '2-digit', day: '2-digit' }).format(now);
}
```

规范参数后才启用 query；`employeePage` 固定正整数，目录请求 `pageSize: 50`。`directoryMode=organization` 映射 API `mode='all'`，否则映射 `mode='archived'`。

- [ ] **Step 4：实现目录组件**

组件 props：

```ts
type ConversationTrajectoryDirectoryProps = {
  mode: TrajectoryDirectoryMode;
  draftKeyword: string;
  data?: StaffDirectoryPage;
  selectedEmployeeId: number | null;
  isLoading: boolean;
  isRefreshing: boolean;
  onDraftKeywordChange(value: string): void;
  onKeywordSubmit(): void;
  onModeChange(mode: TrajectoryDirectoryMode): void;
  onDepartmentChange(id: number | null): void;
  onPageChange(page: number): void;
  onSelectEmployee(id: number): void;
  onRefresh(): void;
};
```

员工卡为 `button`，选中项使用 `aria-current="true"`；部门树使用真实 `departments`；limitations 使用紧凑 `role="status"`。

- [ ] **Step 5：删除旧页面结构并组合目录壳**

删除旧 `conversation-global-header`、关键词会话列表和旧 `/detail?id=` 请求。主区暂时显示“请选择员工查看当日会话轨迹”，为 Task 7 留出明确插槽。

- [ ] **Step 6：运行目录测试确认 GREEN**

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/features/conversation-global/conversation-trajectory-page.test.tsx`

Expected: PASS 当前目录和 URL 测试；后续轨迹断言仍未加入。

- [ ] **Step 7：提交**

```powershell
git add web/apps/dashboard/src/features/conversation-global/conversation-trajectory-directory.tsx web/apps/dashboard/src/features/conversation-global/conversation-trajectory-page.tsx web/apps/dashboard/src/features/conversation-global/conversation-trajectory-page.test.tsx
git commit -m "feat: add trajectory employee directory"
```

---

### Task 7：实现分类指标和 24 小时时间轴

**Files:**

- Create: `web/apps/dashboard/src/features/conversation-global/conversation-trajectory-timeline.tsx`
- Modify: `web/apps/dashboard/src/features/conversation-global/conversation-trajectory-page.tsx`
- Modify: `web/apps/dashboard/src/features/conversation-global/conversation-trajectory-page.test.tsx`

**Interfaces:**

- Timeline consumes: `ConversationTrajectoryDay`、selected date/type、callbacks。
- Timeline emits: `onDateChange`、`onTypeChange`、`onRefresh`、`onOpenEvent`。
- Page owns: `trajectory-day` query key。

- [ ] **Step 1：写指标和时间轴失败测试**

覆盖：

```tsx
expect(await screen.findByRole('region', { name: '会话轨迹时间轴' })).toBeTruthy();
expect(screen.getAllByTestId('trajectory-hour')).toHaveLength(24);
expect(screen.getByText('内部群聊未接入')).toBeTruthy();
expect(screen.getByRole('button', { name: /客户群.*1 条.*23:08/ })).toBeTruthy();
expect(screen.getByText('沟通客户数')).toBeTruthy();
```

交互断言：前一天更新 date；今天后一天禁用；类型变化只改变 `conversationType`；刷新 URL 不变；切换类型后四类统计仍保留；早于 08:00 的活动触发滚动到最早小时；同小时多卡都存在且顺序稳定。

- [ ] **Step 2：运行测试确认 RED**

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/features/conversation-global/conversation-trajectory-page.test.tsx`

Expected: FAIL，轨迹主区仍是占位。

- [ ] **Step 3：实现工具栏和指标**

固定分类元数据只描述文案和 key，不提供数字：

```ts
const metricCards = [
  { key: 'internalSingle', title: '内部单聊', subjectLabel: '沟通同事数' },
  { key: 'externalSingle', title: '外部单聊', subjectLabel: '沟通客户数' },
  { key: 'internalGroup', title: '内部群聊', subjectLabel: '内部群数' },
  { key: 'externalGroup', title: '外部群聊', subjectLabel: '外部群数' },
] as const;
```

available 显示服务端数值，unavailable 显示 `--` 和 reason。工具栏类型选择使用 `all/employee/customer/room`，内部群作为禁用说明项而不是可请求值。

- [ ] **Step 4：实现语义化 24 小时行**

```ts
const hours = Array.from({ length: 24 }, (_, hour) => String(hour).padStart(2, '0'));
const eventsByHour = data.events.reduce<Map<string, ConversationTrajectoryEvent[]>>((groups, event) => {
  const group = groups.get(event.hour) ?? [];
  group.push(event);
  groups.set(event.hour, group);
  return groups;
}, new Map());
```

每个小时使用 `<section data-testid="trajectory-hour" aria-label="23:00">`；活动卡使用 button，accessible name 包含目标、消息数和首末时间。空数据仍渲染 24 行并显示状态文案。

初始滚动逻辑：最早活动小时小于 08 时滚到最早小时，否则滚到 08；只在 employee/date 改变时执行，不在 refresh 后抢用户滚动位置。

- [ ] **Step 5：组合 React Query**

```ts
const dayQuery = useQuery({
  queryKey: ['corp', access.corp.id, 'trajectory-day', employeeId, date, conversationType],
  queryFn: () => api.trajectoryDay!({ employeeId: employeeId!, date, conversationType }),
  enabled: employeeId !== null && normalized && api.trajectoryDay !== undefined,
});
```

API 缺失时显示“会话轨迹数据能力未接入”，不能请求旧全局列表兜底。

- [ ] **Step 6：运行时间轴测试确认 GREEN**

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/features/conversation-global/conversation-trajectory-page.test.tsx`

Expected: PASS。

- [ ] **Step 7：提交**

```powershell
git add web/apps/dashboard/src/features/conversation-global/conversation-trajectory-timeline.tsx web/apps/dashboard/src/features/conversation-global/conversation-trajectory-page.tsx web/apps/dashboard/src/features/conversation-global/conversation-trajectory-page.test.tsx
git commit -m "feat: render daily conversation timeline"
```

---

### Task 8：实现当天消息详情抽屉

**Files:**

- Create: `web/apps/dashboard/src/features/conversation-global/conversation-trajectory-drawer.tsx`
- Modify: `web/apps/dashboard/src/features/conversation-global/conversation-trajectory-page.tsx`
- Modify: `web/apps/dashboard/src/features/conversation-global/conversation-trajectory-page.test.tsx`

**Interfaces:**

- Drawer consumes: `api`、`conversationId`、`date`、`eventHour`、`onClose`。
- Reuses: `api.staffDetail`、`ConversationMessageContent`。
- Produces: 关闭按钮、遮罩、Escape、加载更早和焦点返回行为。

- [ ] **Step 1：写抽屉失败测试**

覆盖：整卡点击写入 `conversationId/eventHour`；详情请求精确带 date 和 `pageSize=50`；消息左右方向；加载更早按 ID 去重并插在顶部；关闭按钮、遮罩和 Escape 清除 URL；关闭后焦点返回原活动卡；详情错误不卸载时间轴；群成员 limitation 可见。

```tsx
fireEvent.click(await screen.findByRole('button', { name: /客户群.*1 条/ }));
expect(api.staffDetail).toHaveBeenCalledWith({
  conversationId: '9:2:3001', keyword: '', messageTypes: [],
  date: '2026-08-19', pageSize: 50,
});
expect(await screen.findByRole('dialog', { name: '会话详情' })).toBeTruthy();
fireEvent.keyDown(document, { key: 'Escape' });
await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
```

- [ ] **Step 2：运行测试确认 RED**

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/features/conversation-global/conversation-trajectory-page.test.tsx`

Expected: FAIL，活动卡尚未打开详情。

- [ ] **Step 3：实现详情查询和消息合并**

抽屉内部使用 `useInfiniteQuery`；第一页不传 before，后续传 `lastPage.nextBefore`。合并函数：

```ts
function mergeTrajectoryMessages(pages: readonly StaffConversationDetail[]): readonly ConversationMessage[] {
  const byId = new Map<string, ConversationMessage>();
  for (const page of [...pages].reverse()) {
    for (const message of page.messages) byId.set(message.id, message);
  }
  return [...byId.values()].sort((a, b) => a.sentAt.localeCompare(b.sentAt) || a.id.localeCompare(b.id));
}
```

消息内容使用 `<ConversationMessageContent type={message.type} content={message.content} />`。

- [ ] **Step 4：实现可访问抽屉**

抽屉容器使用 `role="dialog" aria-modal="true" aria-label="会话详情"`。挂载时聚焦关闭按钮；监听 Escape；遮罩只在 `event.target === event.currentTarget` 时关闭；卸载后通过页面保存的 event button ref 恢复焦点。

详情为空且活动存在时显示：

```text
活动摘要与消息详情不一致，请重新加载。
```

不能显示普通“暂无数据”。

- [ ] **Step 5：组合 URL 恢复**

打开活动写入 `conversationId` 和 `eventHour`；刷新时只有当两个参数与当前 `trajectoryDay.events` 匹配才打开抽屉，否则使用 replace 清除。关闭只清除这两个参数。

- [ ] **Step 6：运行抽屉测试确认 GREEN**

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/features/conversation-global/conversation-trajectory-page.test.tsx src/features/conversation-global/conversation-message-content.test.tsx`

Expected: PASS。

- [ ] **Step 7：提交**

```powershell
git add web/apps/dashboard/src/features/conversation-global/conversation-trajectory-drawer.tsx web/apps/dashboard/src/features/conversation-global/conversation-trajectory-page.tsx web/apps/dashboard/src/features/conversation-global/conversation-trajectory-page.test.tsx
git commit -m "feat: add trajectory conversation drawer"
```

---

### Task 9：实现宽屏布局和四档响应式

**Files:**

- Create: `web/apps/dashboard/src/styles/conversation-trajectory-layout.test.ts`
- Modify: `web/apps/dashboard/src/styles/index.css`
- Modify: `web/apps/dashboard/src/features/conversation-global/conversation-trajectory-page.test.tsx`

**Interfaces:**

- Desktop columns: `320px minmax(760px, 1fr)`。
- Drawer width: `min(880px, 72vw)`。
- Breakpoints: 1600、1200、768。

- [ ] **Step 1：写 CSS 契约失败测试**

```ts
expect(css).toContain('grid-template-columns: 320px minmax(760px, 1fr)');
expect(css).toContain('height: calc(100dvh - 104px)');
expect(css).toContain('.conversation-trajectory-scroll { min-height: 0; overflow-y: auto; }');
expect(css).toContain('width: min(880px, 72vw)');
expect(css).toContain('@media (max-width: 1599px)');
expect(css).toContain('@media (max-width: 1199px)');
expect(css).toContain('@media (max-width: 768px)');
expect(css).not.toContain('.conversation-trajectory-page { display: grid; gap: 16px; margin: 0 auto; max-width: 1320px; }');
```

页面测试增加目录抽屉与详情抽屉同一时间只打开一个、遮罩具名和移动端返回流程。

- [ ] **Step 2：运行样式测试确认 RED**

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/styles/conversation-trajectory-layout.test.ts src/features/conversation-global/conversation-trajectory-page.test.tsx`

Expected: FAIL，旧 1320px 两卡布局仍存在。

- [ ] **Step 3：实现 1600px 以上布局**

删除旧 `.conversation-trajectory-search/list/detail/timeline` 样式。新工作台使用单一白色外框和 1px 内部分隔；目录 320px，主列 `min-width:0`；轨迹主区独立滚动；指标四列；小时行使用 `grid-template-columns: 64px minmax(0,1fr)`；活动区 flex wrap。

hover 只修改 `background-color`、`border-color`、`color`，不使用 `transform`、`scale` 或字体抗锯齿切换。

- [ ] **Step 4：实现三档降级**

- 1200–1599：目录 288px，指标两列，抽屉 `min(760px,78vw)`。
- 769–1199：目录固定定位为左抽屉，主区占满；遮罩层次低于详情抽屉。
- ≤768：员工列表、轨迹、详情分步单列；指标两列；小时标签 52px；详情全屏。

所有覆盖层有 `overflow:auto`、关闭按钮 sticky，页面根节点无横向滚动。

- [ ] **Step 5：运行样式与页面测试确认 GREEN**

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/styles/conversation-trajectory-layout.test.ts src/features/conversation-global/conversation-trajectory-page.test.tsx`

Expected: PASS。

- [ ] **Step 6：提交**

```powershell
git add web/apps/dashboard/src/styles/conversation-trajectory-layout.test.ts web/apps/dashboard/src/styles/index.css web/apps/dashboard/src/features/conversation-global/conversation-trajectory-page.test.tsx
git commit -m "style: align conversation trajectory with yuanhu"
```

---

### Task 10：全量验证、Docker 更新和浏览器验收

**Files:**

- Modify: `docs/PROJECT_PROGRESS.zh-CN.md`

**Interfaces:**

- Runtime: `mochat-go-desktop`。
- Local URL: `http://127.0.0.1:18080/chat/trajectory`。
- Reference URL: `https://web-ai-analysis-work-wechat.yuanhu.com/#/chat/trajectory`。

- [ ] **Step 1：运行后端目标测试**

Run: `go test ./internal/dashboard ./internal/store ./internal/server ./internal/migration -run 'Trajectory|WorkMessageGroupTarget|StaffDetail.*Room' -count=1`

Expected: PASS；记录包数、耗时和任何 skip 原因。

- [ ] **Step 2：运行后端相关包完整测试**

Run: `go test ./internal/dashboard ./internal/store ./internal/server ./internal/migration -count=1`

Expected: PASS。

- [ ] **Step 3：运行 dashboard 目标测试和全量检查**

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/features/conversation-global/conversation-global-api.test.ts src/features/conversation-global/conversation-trajectory-page.test.tsx src/features/conversation-global/conversation-message-content.test.tsx src/styles/conversation-trajectory-layout.test.ts`

Run: `corepack pnpm --filter @mochat/dashboard test`

Run: `corepack pnpm --filter @mochat/dashboard typecheck`

Run: `corepack pnpm --filter @mochat/dashboard build`

Expected: 四条命令退出码 0；无未解释 warning/error。

- [ ] **Step 4：部署前记录容器和数据卷**

```powershell
docker compose -p mochat-go-desktop -f deploy/standalone/docker-compose.yml ps
docker volume ls --filter label=com.docker.compose.project=mochat-go-desktop
```

Expected: app、mysql、redis 已存在；保存命名卷集合用于更新后比较。

- [ ] **Step 5：只重建 app**

```powershell
docker compose -p mochat-go-desktop --env-file deploy/standalone/.env.local -f deploy/standalone/docker-compose.yml up -d --build --no-deps app
```

Expected: 不重建/删除 MySQL、Redis 和命名卷。

- [ ] **Step 6：等待健康并核对卷未变化**

Run: `docker compose -p mochat-go-desktop -f deploy/standalone/docker-compose.yml ps`

Run: `docker volume ls --filter label=com.docker.compose.project=mochat-go-desktop`

Expected: app、mysql、redis healthy；命名卷集合与 Step 4 一致。

- [ ] **Step 7：浏览器逐项验收**

在 2560×1440、1440×900、1066×1272、390×844 验证：

1. 菜单和顶部 Banner 未变，页面没有介绍卡。
2. 2560×1440 两列填满，四张统计单行，右侧轨迹为主列，无横向滚动。
3. 存档员工、组织架构、部门、员工搜索/刷新和 50 条分页真实生效。
4. 日期前后切换、未来日期禁用、会话类型和刷新真实改变请求。
5. 真 0 与内部群未接入显示不同；无法匹配目标显示真实条数告警。
6. 24 个小时行都可访问；有早于 08:00 的消息时默认定位最早活动。
7. 同小时多活动不重叠；整卡点击打开正确详情。
8. 详情只显示所选日期；加载更早、关闭按钮、遮罩、Escape 和焦点返回正常。
9. 浏览器刷新恢复员工、日期、类型和合法详情锚点。
10. 控制台无新增错误，关键请求无未解释 4xx/5xx；40301 使用配置状态。

- [ ] **Step 8：更新进度文档**

在 `docs/PROJECT_PROGRESS.zh-CN.md` 记录：变更范围、三个页面 API 资源、数据来源矩阵、内部群和群发送人缺口、测试命令与结果、四种分辨率、容器健康、卷集合未变化和浏览器控制台结果。

- [ ] **Step 9：最终提交**

```powershell
git add docs/PROJECT_PROGRESS.zh-CN.md
git commit -m "docs: record conversation trajectory acceptance"
```

完成后停留在 `/chat/trajectory`，等待用户检查，不自动进入会话导出或其他菜单。
