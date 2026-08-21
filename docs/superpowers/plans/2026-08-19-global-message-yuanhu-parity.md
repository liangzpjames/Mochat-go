# 全局消息圆弧 AI 对标完善实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在不伪造数据的前提下，将 `/chat/v2-all` 完善为“8 个今日指标、5 个快捷状态、组合筛选、会话卡片、个人关注和详情抽屉”的圆弧式全局消息工作台。

**Architecture:** 保留 `mc_work_message_1..10` 为唯一消息事实源和现有列表/详情路由，新增独立概览接口与个人关注表，风险和超时只关联已有 Provider 记录。前端把当前大页面拆为 API 合同、筛选、指标、卡片和消息渲染器，所有提交状态继续写入 URL。

**Tech Stack:** Go 1.25、MySQL 8、React 19、TypeScript 5.9、TanStack Query 5、React Router 7、Vitest、Testing Library、Docker Compose。

## Global Constraints

- 设计依据：`docs/superpowers/specs/2026-08-19-global-message-yuanhu-parity-design.md`。
- 只完善 `/chat/v2-all`；全局导航、顶部 Banner、企业切换和登录体系不改。
- 所有数字和会话内容必须来自系统真实数据；不可用值返回 `null` 并告警，不以 0 代替。
- `simulated` 来源必须标记“演示数据”，不得标为企业微信真实存档。
- 列表、概览、详情和关注必须使用同一 tenant、corp、RBAC、员工范围和归档来源规则。
- 时区固定为 `Asia/Shanghai`，日期查询使用 `[startAt, endAt + 1 day)` 半开区间。
- 默认页大小 20、最大 100；详情最多读取最近 200 条。
- 当前 931px 实际窗口不得出现页面级横向滚动，hover 不使用 `transform` 或 `filter`。
- 会话级 AI 摘要和内部群聊不伪造：能力未接入时集中告警并隐藏重复占位。
- 不修改或清理工作区内与本任务无关的已有改动。

---

## 文件结构

### 新建

- `deploy/standalone/migrations/0140_global_message_focus_and_indexes.up.sql`：个人关注表和全局消息组合索引。
- `deploy/standalone/migrations/0140_global_message_focus_and_indexes.down.sql`：只回滚 0140 创建的表和索引。
- `internal/dashboard/work_message_global.go`：全局消息专用类型、参数解析、概览与关注 Handler。
- `internal/dashboard/work_message_global_test.go`：Handler 合同、权限和错误状态测试。
- `internal/store/work_message_global.go`：概览聚合、状态关联、个人关注和扩展列表查询。
- `internal/store/work_message_global_integration_test.go`：真实 MySQL 分表、指标、状态和隔离测试。
- `web/apps/dashboard/src/features/conversation-global/conversation-global-metrics.tsx`：8 个今日指标和 5 个快捷状态。
- `web/apps/dashboard/src/features/conversation-global/conversation-global-filters.tsx`：关键词、员工、日期、会话与消息类型筛选。
- `web/apps/dashboard/src/features/conversation-global/conversation-global-cards.tsx`：会话卡片网格与个人关注交互。
- `web/apps/dashboard/src/features/conversation-global/conversation-message-renderer.tsx`：1–18 类消息的安全展示。
- `web/apps/dashboard/src/features/conversation-global/conversation-global-components.test.tsx`：新增组件测试。
- `web/apps/dashboard/src/styles/conversation-global-layout.test.ts`：响应式和 hover 样式合同测试。

### 修改

- `internal/dashboard/auto_tag_dashboard.go`：把全局消息专用合同移至新文件，保留旧页面兼容入口。
- `internal/dashboard/auto_tag_dashboard_test.go`：更新扩展列表合同，保留旧接口回归测试。
- `internal/store/auto_tag.go`：调用新查询实现并删除迁移后的全局专用 SQL，保留其他自动标签能力。
- `internal/store/work_message_archive_sync.go`：复用统一的消息类型枚举映射。
- `internal/store/work_message_test.go`：保留旧列表与详情回归断言。
- `internal/dashboard/dashboard_page_catalog.json`：登记概览与关注资源。
- `internal/server/server.go`、`internal/server/server_test.go`：注册新 HTTP Handler。
- `cmd/mochat-go/main.go`：注入新路由 Handler。
- `web/apps/dashboard/src/features/conversation-global/conversation-global-api.ts`：扩展筛选、列表、概览、关注和详情合同。
- `web/apps/dashboard/src/features/conversation-global/conversation-global-api.test.ts`：API 序列化和解析测试。
- `web/apps/dashboard/src/features/conversation-global/conversation-global-page.tsx`：组合新组件和独立 Query。
- `web/apps/dashboard/src/features/conversation-global/conversation-global-page.test.tsx`：页面状态与 URL 测试。
- `web/apps/dashboard/src/styles/index.css`：替换全局消息表格样式为指标、筛选、卡片和抽屉布局。
- `web/e2e/tests/yuanhu-phase1.spec.ts`：增加全局消息关键交互验收。
- `docs/phases/phase-3-dashboard/phase-3.2/reports/tasks/task-4-report.md`：记录最终实现、数据来源和验证证据。

---

### Task 1: 建立个人关注表和查询索引

**Files:**
- Create: `deploy/standalone/migrations/0140_global_message_focus_and_indexes.up.sql`
- Create: `deploy/standalone/migrations/0140_global_message_focus_and_indexes.down.sql`
- Test: `internal/migration/archive_runner_integration_test.go`

**Interfaces:**
- Produces: `mochat_go_work_message_focus`，唯一键 `(tenant_id, corp_id, user_id, work_employee_id, to_user_type, to_user_id)`。
- Produces: `idx_mc_work_message_N_global`，供十张归档表的对象类型、时间、员工、对象和消息类型筛选使用。

- [ ] **Step 1: 写失败的迁移测试**

在 `internal/migration/archive_runner_integration_test.go` 增加测试：执行到 0140 后断言关注表、唯一索引以及十个 `idx_mc_work_message_N_global` 均存在；执行 down 后只删除这些对象，不删除消息分表。

```go
func TestMigration0140CreatesGlobalMessageFocusAndIndexes(t *testing.T) {
	// 使用该文件已有的 MySQL 测试夹具执行 0140 up。
	assertTableExists(t, db, "mochat_go_work_message_focus")
	assertIndexColumns(t, db, "mochat_go_work_message_focus", "uk_work_message_focus_subject",
		"tenant_id,corp_id,user_id,work_employee_id,to_user_type,to_user_id")
	for i := 1; i <= 10; i++ {
		assertIndexColumns(t, db, fmt.Sprintf("mc_work_message_%d", i),
			fmt.Sprintf("idx_mc_work_message_%d_global", i),
			"corp_id,to_user_type,msg_data_time,work_employee_id,to_user_id,msg_type")
	}
}
```

- [ ] **Step 2: 运行测试并确认失败**

Run: `go test ./internal/migration -run TestMigration0140CreatesGlobalMessageFocusAndIndexes -count=1`

Expected: FAIL，提示 0140 文件、关注表或组合索引不存在。

- [ ] **Step 3: 编写 up migration**

```sql
CREATE TABLE IF NOT EXISTS `mochat_go_work_message_focus` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `tenant_id` bigint unsigned NOT NULL,
  `corp_id` bigint unsigned NOT NULL,
  `user_id` bigint unsigned NOT NULL,
  `work_employee_id` int NOT NULL,
  `to_user_type` tinyint NOT NULL,
  `to_user_id` int NOT NULL,
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_work_message_focus_subject` (`tenant_id`,`corp_id`,`user_id`,`work_employee_id`,`to_user_type`,`to_user_id`),
  KEY `idx_work_message_focus_list` (`tenant_id`,`corp_id`,`user_id`,`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

ALTER TABLE `mc_work_message_1` ADD KEY `idx_mc_work_message_1_global` (`corp_id`,`to_user_type`,`msg_data_time`,`work_employee_id`,`to_user_id`,`msg_type`);
ALTER TABLE `mc_work_message_2` ADD KEY `idx_mc_work_message_2_global` (`corp_id`,`to_user_type`,`msg_data_time`,`work_employee_id`,`to_user_id`,`msg_type`);
ALTER TABLE `mc_work_message_3` ADD KEY `idx_mc_work_message_3_global` (`corp_id`,`to_user_type`,`msg_data_time`,`work_employee_id`,`to_user_id`,`msg_type`);
ALTER TABLE `mc_work_message_4` ADD KEY `idx_mc_work_message_4_global` (`corp_id`,`to_user_type`,`msg_data_time`,`work_employee_id`,`to_user_id`,`msg_type`);
ALTER TABLE `mc_work_message_5` ADD KEY `idx_mc_work_message_5_global` (`corp_id`,`to_user_type`,`msg_data_time`,`work_employee_id`,`to_user_id`,`msg_type`);
ALTER TABLE `mc_work_message_6` ADD KEY `idx_mc_work_message_6_global` (`corp_id`,`to_user_type`,`msg_data_time`,`work_employee_id`,`to_user_id`,`msg_type`);
ALTER TABLE `mc_work_message_7` ADD KEY `idx_mc_work_message_7_global` (`corp_id`,`to_user_type`,`msg_data_time`,`work_employee_id`,`to_user_id`,`msg_type`);
ALTER TABLE `mc_work_message_8` ADD KEY `idx_mc_work_message_8_global` (`corp_id`,`to_user_type`,`msg_data_time`,`work_employee_id`,`to_user_id`,`msg_type`);
ALTER TABLE `mc_work_message_9` ADD KEY `idx_mc_work_message_9_global` (`corp_id`,`to_user_type`,`msg_data_time`,`work_employee_id`,`to_user_id`,`msg_type`);
ALTER TABLE `mc_work_message_10` ADD KEY `idx_mc_work_message_10_global` (`corp_id`,`to_user_type`,`msg_data_time`,`work_employee_id`,`to_user_id`,`msg_type`);
```

- [ ] **Step 4: 编写精确 down migration**

```sql
ALTER TABLE `mc_work_message_10` DROP INDEX `idx_mc_work_message_10_global`;
ALTER TABLE `mc_work_message_9` DROP INDEX `idx_mc_work_message_9_global`;
ALTER TABLE `mc_work_message_8` DROP INDEX `idx_mc_work_message_8_global`;
ALTER TABLE `mc_work_message_7` DROP INDEX `idx_mc_work_message_7_global`;
ALTER TABLE `mc_work_message_6` DROP INDEX `idx_mc_work_message_6_global`;
ALTER TABLE `mc_work_message_5` DROP INDEX `idx_mc_work_message_5_global`;
ALTER TABLE `mc_work_message_4` DROP INDEX `idx_mc_work_message_4_global`;
ALTER TABLE `mc_work_message_3` DROP INDEX `idx_mc_work_message_3_global`;
ALTER TABLE `mc_work_message_2` DROP INDEX `idx_mc_work_message_2_global`;
ALTER TABLE `mc_work_message_1` DROP INDEX `idx_mc_work_message_1_global`;
DROP TABLE IF EXISTS `mochat_go_work_message_focus`;
```

- [ ] **Step 5: 验证迁移并提交**

Run: `go test ./internal/migration -run TestMigration0140CreatesGlobalMessageFocusAndIndexes -count=1`

Expected: PASS。

```bash
git add deploy/standalone/migrations/0140_global_message_focus_and_indexes.* internal/migration/archive_runner_integration_test.go
git commit -m "feat: add global message focus storage"
```

---

### Task 2: 定义全局消息后端合同与 HTTP 入口

**Files:**
- Create: `internal/dashboard/work_message_global.go`
- Create: `internal/dashboard/work_message_global_test.go`
- Modify: `internal/dashboard/auto_tag_dashboard.go`
- Modify: `internal/dashboard/dashboard_page_catalog.json`
- Modify: `internal/server/server.go`
- Modify: `internal/server/server_test.go`
- Modify: `cmd/mochat-go/main.go`

**Interfaces:**
- Consumes: Task 1 的关注表。
- Produces: `WorkMessageGlobalPage(ctx, filter)`、`WorkMessageGlobalOverview(ctx, filter)`、`SetWorkMessageFocus(ctx, input)`、`DeleteWorkMessageFocus(ctx, input)` Store 接口。
- Produces: `GET /dashboard/workMessage/globalOverview`、`PUT|DELETE /dashboard/workMessage/focus`。

- [ ] **Step 1: 写 Handler 失败测试**

覆盖以下合同：

```go
func TestWorkMessageGlobalOverviewUsesPrincipalScope(t *testing.T) {
	req := authenticatedDashboardRequestForTestAs(http.MethodGet,
		"/dashboard/workMessage/globalOverview?employeeIds=9&messageTypes=text&messageTypes=image&startAt=2026-08-01&endAt=2026-08-19",
		nil, 1, 10, 7, 42)
	// 断言 tenant=1、corp=7、user=42 来自 Principal；员工 IDs 与权限范围求交集；
	// endAt 被转换为 2026-08-20 00:00:00；返回 null capability 不被改成 0。
}

func TestWorkMessageFocusRejectsOutOfScopeConversation(t *testing.T) {
	req := authenticatedDashboardRequestForTestAs(http.MethodPut,
		"/dashboard/workMessage/focus", strings.NewReader(`{"conversationId":"99:1:31"}`),
		1, 10, 7, 42)
	// Store 报 not found 时断言 HTTP 404，不泄漏越权会话。
}
```

另加表驱动测试覆盖非法 `bucket`、非法 `messageTypes`、日期缺一端、开始晚于结束、非法 `conversationId`、非 PUT/DELETE 方法、归档 40301 和普通 RBAC 403。

- [ ] **Step 2: 运行测试并确认失败**

Run: `go test ./internal/dashboard -run 'WorkMessageGlobalOverview|WorkMessageFocus' -count=1`

Expected: FAIL，提示新类型或 Handler 未定义。

- [ ] **Step 3: 定义稳定后端类型**

在 `work_message_global.go` 定义：

```go
type WorkMessageBucket string

const (
	WorkMessageBucketAll     WorkMessageBucket = "all"
	WorkMessageBucketTimeout WorkMessageBucket = "timeout"
	WorkMessageBucketRisk    WorkMessageBucket = "risk"
	WorkMessageBucketToday   WorkMessageBucket = "today"
	WorkMessageBucketFocused WorkMessageBucket = "focused"
)

type WorkMessageMetricValue struct {
	Value      *int64   `json:"value"`
	ChangeRate *float64 `json:"changeRate"`
	Status     string   `json:"status"`
	Reason     string   `json:"reason,omitempty"`
}

type WorkMessageCapability struct {
	Available bool   `json:"available"`
	Reason    string `json:"reason"`
}

type WorkMessageGlobalFilter struct {
	TenantID, CorpID, UserID int
	Keyword                  string
	ConversationType         int
	MessageTypes             []int
	EmployeeIDs              []int
	RestrictEmployeeIDs      bool
	DateTimeStart            string
	DateTimeEnd              string
	Bucket                    WorkMessageBucket
	ArchiveSource             string
	Page, PerPage             int
}

type WorkMessageGlobalOverview struct {
	Timezone     string                                `json:"timezone"`
	GeneratedAt  string                                `json:"generatedAt"`
	Metrics      map[string]WorkMessageMetricValue     `json:"metrics"`
	Buckets      map[WorkMessageBucket]*int64           `json:"buckets"`
	Capabilities map[string]WorkMessageCapability      `json:"capabilities"`
}

type WorkMessageFocusInput struct {
	TenantID, CorpID, UserID, WorkEmployeeID, ToUserType, ToUserID int
}

type WorkMessageGlobalConversation struct {
	ID, ConversationID, EmployeeName, EmployeeAvatar string
	EmployeeID                                        int
	TargetType, TargetName, TargetAvatar              string
	TargetID                                          int
	FirstMessageAt, LastMessage, LastMessageType      string
	LastDirection, SentAt                             string
	MessageTotal, RiskCount, TimeoutCount             int
	Focused                                           bool
	AISummary                                         *string
	ArchiveSource, ArchiveSourceID                    string
}

type WorkMessageGlobalPage struct {
	Items             []WorkMessageGlobalConversation
	Total, Page, PerPage int
}
```

扩展 `AutoTagStore`，并把当前全局消息参数解析函数从 `auto_tag_dashboard.go` 移到新文件；非全局旧接口保持不变。

- [ ] **Step 4: 实现 Handler 与路由**

- `WorkMessageGlobalOverview` 使用现有 `workMessageConversationPermissionKey` 授权，调用统一筛选解析器并输出设计文档中的响应。
- `WorkMessageToUsers` 在 `view=global` 时调用 `WorkMessageGlobalPage` 并输出扩展卡片合同，其他请求继续调用旧 `WorkMessageToUsers`。
- `WorkMessageFocus` 解析当前 Principal 和不透明 `conversationId`，PUT 调 `SetWorkMessageFocus`，DELETE 调 `DeleteWorkMessageFocus`。
- `WorkMessageIndex` 在 `conversationId` 存在时按会话三元组读取详情，在只有旧 `id` 时继续执行归档消息锚点逻辑。
- 在 `dashboard_page_catalog.json` 的 `dashboard.chat.v2_all.resources` 增加 GET `globalOverview`、PUT/DELETE `focus`。
- 在 `internal/server/server.go` 增加两个 option 和精确路由，在 `cmd/mochat-go/main.go` 注入 Handler。

```go
func workMessageConversationParts(raw string) (employeeID, toUserType, toUserID int, ok bool) {
	parts := strings.Split(raw, ":")
	if len(parts) != 3 { return 0, 0, 0, false }
	employeeID, errEmployee := strconv.Atoi(parts[0])
	toUserType, errType := strconv.Atoi(parts[1])
	toUserID, errTarget := strconv.Atoi(parts[2])
	if errEmployee != nil || errType != nil || errTarget != nil ||
		employeeID <= 0 || toUserID <= 0 || toUserType < 0 || toUserType > 2 {
		return 0, 0, 0, false
	}
	return employeeID, toUserType, toUserID, true
}
```

- [ ] **Step 5: 运行后端合同测试并提交**

Run: `go test ./internal/dashboard ./internal/server -run 'WorkMessageGlobal|WorkMessageFocus|WorkMessageListAndDetailUseSameRBAC' -count=1`

Expected: PASS。

```bash
git add internal/dashboard/work_message_global.go internal/dashboard/work_message_global_test.go internal/dashboard/auto_tag_dashboard.go internal/dashboard/dashboard_page_catalog.json internal/server/server.go internal/server/server_test.go cmd/mochat-go/main.go
git commit -m "feat: add global message overview contracts"
```

---

### Task 3: 实现真实概览、状态关联、扩展列表和关注存储

**Files:**
- Create: `internal/store/work_message_global.go`
- Create: `internal/store/work_message_global_integration_test.go`
- Modify: `internal/store/auto_tag.go`
- Modify: `internal/store/work_message_archive_sync.go`
- Modify: `internal/store/work_message_test.go`
- Modify: `internal/dashboard/work_message_global.go`

**Interfaces:**
- Consumes: `WorkMessageGlobalFilter`、关注表、风险记录、超时记录、十张归档分表。
- Produces: 独立 `WorkMessageGlobalPage`、真实 `WorkMessageGlobalOverview`、幂等关注写入与删除。

- [ ] **Step 1: 写 Store 集成测试夹具**

在真实 MySQL 测试中建立两个 tenant、三个 corp、多个员工，并把消息分散写入第 1、5、10 分表；插入 customer、room、risk、timeout、focus 和 archive source 数据。至少断言：

```go
func TestWorkMessageGlobalOverviewAndBucketsAreScoped(t *testing.T) {
	got, err := store.WorkMessageGlobalOverview(ctx, dashboard.WorkMessageGlobalFilter{
		TenantID: 1, CorpID: 7, UserID: 42,
		RestrictEmployeeIDs: true, EmployeeIDs: []int{9},
		DateTimeStart: "2026-08-01 00:00:00", DateTimeEnd: "2026-08-20 00:00:00",
	})
	if err != nil { t.Fatal(err) }
	metric := got.Metrics["customerMessages"]
	if metric.Value == nil || *metric.Value != 3 || metric.ChangeRate == nil || *metric.ChangeRate != 0.5 {
		t.Fatalf("customerMessages = %#v", metric)
	}
	for bucket, want := range map[dashboard.WorkMessageBucket]int64{
		dashboard.WorkMessageBucketRisk: 1,
		dashboard.WorkMessageBucketTimeout: 1,
		dashboard.WorkMessageBucketFocused: 1,
	} {
		if got.Buckets[bucket] == nil || *got.Buckets[bucket] != want {
			t.Fatalf("bucket %s = %#v, want %d", bucket, got.Buckets[bucket], want)
		}
	}
	// tenant 2、corp 8、员工 10 的数据均不得计入。
}
```

另写测试覆盖昨日为 0 时 `ChangeRate=nil`、能力表不可用时值为 `nil`、`simulated/external` 来源隔离、消息类型筛选、今日新增会话按 MIN 时间、风险/超时按消息 ID 回溯会话，以及两个用户关注互不影响。

- [ ] **Step 2: 运行集成测试并确认失败**

Run: `go test ./internal/store -run 'WorkMessageGlobalOverview|WorkMessageGlobalList|WorkMessageFocus' -count=1`

Expected: FAIL，提示 Store 方法未实现。

- [ ] **Step 3: 实现分表基础查询和消息类型映射**

把归档同步已有消息类型映射抽成共享双向函数，查询参数仅接受以下枚举：

```go
var workMessageTypeCodes = map[string]int{
	"text": 1, "image": 2, "voice": 3, "video": 4, "file": 5,
	"link": 6, "location": 7, "emotion": 8, "mixed": 9, "markdown": 10,
	"meeting_voice_call": 11, "voip_doc_share": 12, "docmsg": 13,
	"calendar": 14, "vote": 15, "collect": 16, "redpacket": 17,
	"card": 18, "unsupported": 100,
}
```

定义全局会话查询返回项，字段名与前端合同保持一致；`WorkMessageToUser` 继续服务旧接口，不增加圆弧页面专用字段：

```go
type WorkMessageGlobalConversation struct {
	ID              string
	ConversationID  string
	EmployeeID      int
	EmployeeName    string
	EmployeeAvatar  string
	TargetType      string
	TargetID        int
	TargetName      string
	TargetAvatar    string
	FirstMessageAt  string
	LastMessage     string
	LastMessageType string
	LastDirection   string
	SentAt          string
	MessageTotal    int
	RiskCount       int
	TimeoutCount    int
	Focused         bool
	AISummary       *string
	ArchiveSource   string
	ArchiveSourceID string
}
```

生成每张分表 SQL 时先下推 `corp_id`、归档来源、员工 IDs、日期、`to_user_type`、`msg_type`，再 `UNION ALL`。扩展会话聚合返回 `first_message_at`、`last_message_type`、`last_direction`、`message_total`。

- [ ] **Step 4: 实现风险、超时和关注关联**

使用统一会话键 `CONCAT(work_employee_id, ':', to_user_type, ':', to_user_id)`。风险与超时分别按以下优先级关联：

```sql
-- 风险：规范化 conversation_id 直接命中，或 message_id 命中当前来源消息。
rr.conversation_id = conversation_key OR rr.message_id = wm.msgid

-- 超时：规范化 conversation_id 直接命中，或 trigger_message_id 命中当前来源消息。
tr.conversation_id = conversation_key OR tr.trigger_message_id = wm.msgid
```

查询必须同时包含 `tenant_id`、`corp_id`，并基于已经通过员工范围过滤的消息集合聚合。关注使用 `INSERT ... ON DUPLICATE KEY UPDATE created_at=created_at` 幂等写入，DELETE 带全量 tenant/corp/user/会话键条件。

- [ ] **Step 5: 实现 8 个指标与变化率**

用 `time.LoadLocation("Asia/Shanghai")` 计算今日、昨日边界。服务层统一使用：

```go
func workMessageChangeRate(today, yesterday int64) *float64 {
	if yesterday == 0 { return nil }
	value := float64(today-yesterday) / float64(yesterday)
	return &value
}
```

客户新增从 `mc_work_contact` 与 `mc_work_contact_employee` 去重，群新增从 `mc_work_room` 按可见员工范围去重；其余六项从过滤后的归档消息集合计算。字段或关联表不可用时返回 typed capability error，由 Handler 转成 `value:null/status:unavailable`，不能吞错后写 0。

- [ ] **Step 6: 增加 EXPLAIN 门禁并运行全部 Store 测试**

对 1、5、10 分表的代表性数据执行 EXPLAIN，断言选择 `idx_mc_work_message_N_global` 或更窄的 `corp_id` 前缀索引，并限制测试夹具估算扫描行数不超过 512。

Run: `go test ./internal/store -run 'WorkMessage' -count=1`

Expected: PASS。

```bash
git add internal/store/work_message_global.go internal/store/work_message_global_integration_test.go internal/store/auto_tag.go internal/store/work_message_archive_sync.go internal/store/work_message_test.go internal/dashboard/work_message_global.go
git commit -m "feat: aggregate global message operations"
```

---

### Task 4: 扩展前端 API 合同和 URL 状态

**Files:**
- Modify: `web/apps/dashboard/src/features/conversation-global/conversation-global-api.ts`
- Modify: `web/apps/dashboard/src/features/conversation-global/conversation-global-api.test.ts`
- Modify: `web/apps/dashboard/src/features/conversation-global/conversation-global-page.test.tsx`

**Interfaces:**
- Consumes: Task 2–3 的列表、概览、关注和详情接口。
- Produces: `ConversationSearch`、`ConversationOverview`、`ConversationCard`、`setFocus()`、`overview()`。

- [ ] **Step 1: 写 API 合同失败测试**

```ts
it('serializes global filters and preserves unavailable metrics', async () => {
  const api = createConversationGlobalApi({ request });
  await api.search({
    keyword: '报价', conversationType: 'customer', messageTypes: ['text', 'image'],
    employeeIds: ['9', '12'], startAt: '2026-08-01', endAt: '2026-08-19',
    bucket: 'risk', page: 2, pageSize: 20,
  });
  expect(request).toHaveBeenCalledWith(expect.stringContaining(
    'messageTypes=text&messageTypes=image&bucket=risk&page=2&pageSize=20'));

  const overview = await api.overview(baseSearch);
  expect(overview.metrics.newCustomerConversations.value).toBeNull();
});

it('uses idempotent focus endpoints', async () => {
  await api.setFocus('9:1:31', true);
  expect(request).toHaveBeenCalledWith('/workMessage/focus', {
    method: 'PUT', body: JSON.stringify({ conversationId: '9:1:31' }),
  });
});
```

- [ ] **Step 2: 运行测试并确认失败**

Run: `npm --prefix web/apps/dashboard test -- --run src/features/conversation-global/conversation-global-api.test.ts`

Expected: FAIL，提示新字段或方法不存在。

- [ ] **Step 3: 实现精确 TypeScript 合同与解析器**

```ts
export type ConversationBucket = 'all' | 'timeout' | 'risk' | 'today' | 'focused';
export type ConversationMessageType = 'text' | 'image' | 'voice' | 'video' | 'file'
  | 'link' | 'location' | 'emotion' | 'mixed' | 'markdown'
  | 'meeting_voice_call' | 'voip_doc_share' | 'docmsg' | 'calendar'
  | 'vote' | 'collect' | 'redpacket' | 'card' | 'unsupported';

export type ConversationMetric = {
  value: number | null;
  changeRate: number | null;
  status: 'available' | 'unavailable';
  reason?: string;
};

export type ConversationCapability = { available: boolean; reason: string };
export type ConversationMetricKey = 'newCustomers' | 'newCustomerConversations'
  | 'activeCustomers' | 'customerMessages' | 'newRooms'
  | 'newRoomConversations' | 'activeRooms' | 'roomMessages';
export type ConversationCapabilityKey = 'archive' | 'riskLinkage' | 'timeoutLinkage'
  | 'focus' | 'internalRoom' | 'aiSummary';

export type ConversationCard = {
  id: string;
  conversationId: string;
  employeeId: number;
  employeeName: string;
  employeeAvatar: string;
  targetType: ConversationTargetType;
  targetId: number;
  targetName: string;
  targetAvatar: string;
  firstMessageAt: string;
  lastMessage: string;
  lastMessageType: ConversationMessageType;
  lastDirection: 'inbound' | 'outbound';
  sentAt: string;
  messageTotal: number;
  riskCount: number;
  timeoutCount: number;
  focused: boolean;
  aiSummary: string | null;
  archiveSource: 'external' | 'simulated';
  archiveSourceId: string;
};

export type ConversationOverview = {
  timezone: 'Asia/Shanghai';
  generatedAt: string;
  metrics: Record<ConversationMetricKey, ConversationMetric>;
  buckets: Record<ConversationBucket, number | null>;
  capabilities: Record<ConversationCapabilityKey, ConversationCapability>;
};

export type ConversationPage = {
  list: readonly ConversationCard[];
  total: number;
  page: number;
  pageSize: number;
};

export type ConversationGlobalApi = {
  search(input: ConversationSearch): Promise<ConversationPage>;
  overview(input: Omit<ConversationSearch, 'bucket' | 'page' | 'pageSize'>): Promise<ConversationOverview>;
  detail(id: string): Promise<ConversationDetail>;
  detailConversation(conversationId: string): Promise<ConversationDetail>;
  setFocus(conversationId: string, focused: boolean): Promise<void>;
  employees(input: { keyword: string }): Promise<readonly ConversationEmployee[]>;
};
```

解析器严格区分 0 与 `null`；缺字段、错误枚举或错误类型抛出“全局消息接口返回了无效数据”。详情方法改为优先请求 `conversationId`，同时保留 `detail(id)` 兼容签名供其他页面使用。

- [ ] **Step 4: 实现 URL 序列化与 Query key 约束**

- `messageTypes` 与 `employeeIds` 使用重复参数。
- `bucket=all` 可以省略，解析缺省为 `all`。
- 概览请求删除 `bucket/page/pageSize`；筛选变化使列表和概览同时失效，翻页只失效列表。

- [ ] **Step 5: 运行 API 与页面合同测试并提交**

Run: `npm --prefix web/apps/dashboard test -- --run src/features/conversation-global/conversation-global-api.test.ts src/features/conversation-global/conversation-global-page.test.tsx`

Expected: PASS。

```bash
git add web/apps/dashboard/src/features/conversation-global/conversation-global-api.ts web/apps/dashboard/src/features/conversation-global/conversation-global-api.test.ts web/apps/dashboard/src/features/conversation-global/conversation-global-page.test.tsx
git commit -m "feat: extend global message client contract"
```

---

### Task 5: 实现指标、快捷状态、组合筛选和卡片布局

**Files:**
- Create: `web/apps/dashboard/src/features/conversation-global/conversation-global-metrics.tsx`
- Create: `web/apps/dashboard/src/features/conversation-global/conversation-global-filters.tsx`
- Create: `web/apps/dashboard/src/features/conversation-global/conversation-global-cards.tsx`
- Create: `web/apps/dashboard/src/features/conversation-global/conversation-global-components.test.tsx`
- Modify: `web/apps/dashboard/src/features/conversation-global/conversation-global-page.tsx`

**Interfaces:**
- Consumes: `ConversationOverview`、`ConversationSearch`、`ConversationCard`。
- Produces: `ConversationGlobalMetrics`、`ConversationGlobalFilters`、`ConversationGlobalCards`。

- [ ] **Step 1: 写组件失败测试**

```tsx
it('renders eight real metrics and disables unavailable buckets', () => {
  render(<ConversationGlobalMetrics overview={overviewWithUnavailableRisk} bucket="all" onBucketChange={onBucketChange} />);
  expect(screen.getAllByRole('article', { name: /今日/ })).toHaveLength(8);
  expect(screen.getByRole('button', { name: /风险会话/ })).toBeDisabled();
  expect(screen.getByText('昨日无基数')).toBeTruthy();
});

it('submits employee and message type multi-select values', async () => {
  render(<ConversationGlobalFilters value={search} employees={employees} capabilities={capabilities} onSubmit={onSubmit} />);
  // 选择员工 9、文本、图片并提交，断言数组值和 page=1。
});

it('opens a card but focus click does not open the drawer', async () => {
  render(<ConversationGlobalCards items={[card]} onOpen={onOpen} onFocusChange={onFocusChange} />);
  await user.click(screen.getByRole('button', { name: '关注会话' }));
  expect(onFocusChange).toHaveBeenCalledWith(card.conversationId, true);
  expect(onOpen).not.toHaveBeenCalled();
});
```

- [ ] **Step 2: 运行测试并确认失败**

Run: `npm --prefix web/apps/dashboard test -- --run src/features/conversation-global/conversation-global-components.test.tsx`

Expected: FAIL，提示三个组件不存在。

- [ ] **Step 3: 实现指标和状态组件**

- 固定顺序渲染 8 个指标；`value=null` 显示 `—` 和原因。
- `changeRate=null` 显示 `—`，`changeRate=0` 显示 `0.00%`。
- 5 个状态按钮使用 `aria-pressed`；数量为 `null` 时禁用。
- 指标和状态只展示数字、短标签与必要原因，不加入大段说明。

- [ ] **Step 4: 实现组合筛选组件**

- 员工使用搜索多选，数据由 `api.employees` 获取。
- 日期必须成对且顺序有效。
- 会话类型展示内部单聊、客户单聊、客户群聊；内部群聊为禁用项并带 capability 原因。
- 消息类型按稳定枚举多选；关键词 Enter 与“查询”提交，“重置”恢复默认筛选。

- [ ] **Step 5: 实现会话卡片并重组页面**

卡片包含最近时间、参与者、类型、消息预览、消息总数、风险/超时标签、关注按钮和来源徽标。`external` 显示“企业微信”，`simulated` 显示“演示数据”。

页面 Query 结构：

```tsx
const listQuery = useQuery({ queryKey: conversationListKey(input), queryFn: () => api.search(input) });
const overviewQuery = useQuery({ queryKey: conversationOverviewKey(overviewInput), queryFn: () => api.overview(overviewInput) });
```

概览失败时只替换指标区；列表失败时保留统一 `PageState`。删除旧三卡概览、快捷类型表格和原始员工 ID 文本框。

- [ ] **Step 6: 运行组件与页面测试并提交**

Run: `npm --prefix web/apps/dashboard test -- --run src/features/conversation-global/conversation-global-components.test.tsx src/features/conversation-global/conversation-global-page.test.tsx`

Expected: PASS。

```bash
git add web/apps/dashboard/src/features/conversation-global/conversation-global-metrics.tsx web/apps/dashboard/src/features/conversation-global/conversation-global-filters.tsx web/apps/dashboard/src/features/conversation-global/conversation-global-cards.tsx web/apps/dashboard/src/features/conversation-global/conversation-global-components.test.tsx web/apps/dashboard/src/features/conversation-global/conversation-global-page.tsx
git commit -m "feat: build yuanhu-style global message workspace"
```

---

### Task 6: 完善详情消息渲染和关注回滚

**Files:**
- Create: `web/apps/dashboard/src/features/conversation-global/conversation-message-renderer.tsx`
- Modify: `web/apps/dashboard/src/features/conversation-global/conversation-global-components.test.tsx`
- Modify: `web/apps/dashboard/src/features/conversation-global/conversation-global-page.tsx`
- Modify: `web/apps/dashboard/src/features/conversation-global/conversation-global-api.ts`

**Interfaces:**
- Consumes: `ConversationMessage`、`api.setFocus(conversationId, focused)`。
- Produces: `ConversationMessageRenderer` 和具备焦点恢复的详情抽屉。

- [ ] **Step 1: 写消息类型和乐观更新失败测试**

```tsx
it.each([
  [1, '文本内容'], [2, '图片'], [3, '语音'], [4, '视频'], [5, '报价单.pdf'],
  [6, '查看链接'], [7, '地理位置'], [8, '表情'], [9, '混合消息'], [18, '名片'],
  [100, '暂不支持的消息类型'],
])('renders message type %s safely', (type, expected) => {
  render(<ConversationMessageRenderer message={messageOf(type)} />);
  expect(screen.getByText(expected)).toBeTruthy();
  expect(document.querySelector('script')).toBeNull();
});

it('rolls focus state back when the mutation fails', async () => {
  api.setFocus.mockRejectedValue(new Error('保存失败'));
  // 点击关注后先显示已关注，失败后恢复未关注并出现错误提示。
});
```

- [ ] **Step 2: 运行测试并确认失败**

Run: `npm --prefix web/apps/dashboard test -- --run src/features/conversation-global/conversation-global-components.test.tsx`

Expected: FAIL，提示消息渲染器或关注 mutation 行为不存在。

- [ ] **Step 3: 实现安全消息渲染器**

使用纯 React 节点，不使用 `dangerouslySetInnerHTML`。文本取 `text/content`；文件取 `filename/fileName`；链接仅允许 `http:`/`https:`；媒体 URL 不安全或缺失时展示类型占位。未知类型显示代码和“暂不支持的消息类型”，不直接打印 JSON。

```ts
function safeHttpURL(raw: unknown): string | null {
  if (typeof raw !== 'string') return null;
  try {
    const url = new URL(raw, window.location.origin);
    return url.protocol === 'http:' || url.protocol === 'https:' ? url.toString() : null;
  } catch { return null; }
}
```

- [ ] **Step 4: 实现抽屉可访问性和关注 mutation**

- 详情使用 `conversationId` 请求；旧 `id` 详情方法继续可用。
- 打开时记录触发卡片，焦点移动到关闭按钮；Escape、遮罩和关闭按钮关闭；关闭后恢复焦点。
- 使用 TanStack Query mutation 乐观更新列表与概览 `focused` 数量；失败时恢复快照并显示错误。
- `aiSummary` 只在 capability 可用且非空时渲染。

- [ ] **Step 5: 运行全局消息前端测试并提交**

Run: `npm --prefix web/apps/dashboard test -- --run src/features/conversation-global`

Expected: PASS。

```bash
git add web/apps/dashboard/src/features/conversation-global/conversation-message-renderer.tsx web/apps/dashboard/src/features/conversation-global/conversation-global-components.test.tsx web/apps/dashboard/src/features/conversation-global/conversation-global-page.tsx web/apps/dashboard/src/features/conversation-global/conversation-global-api.ts
git commit -m "feat: improve global conversation reading"
```

---

### Task 7: 对齐视觉层级和响应式布局

**Files:**
- Create: `web/apps/dashboard/src/styles/conversation-global-layout.test.ts`
- Modify: `web/apps/dashboard/src/styles/index.css`

**Interfaces:**
- Consumes: Tasks 5–6 的 class names。
- Produces: 4/2/1 指标列、3/2/1 卡片列、560px 详情抽屉和无模糊 hover。

- [ ] **Step 1: 写样式合同失败测试**

```ts
it('keeps the global message page responsive without blurry hover effects', () => {
  expect(css).toContain('grid-template-columns: repeat(4, minmax(0, 1fr))');
  expect(css).toContain('grid-template-columns: repeat(3, minmax(0, 1fr))');
  expect(css).toContain('@media (max-width: 1180px)');
  expect(css).toContain('@media (max-width: 720px)');
  expect(css).not.toMatch(/conversation-global[^}]*:hover[^}]*transform:/s);
  expect(css).not.toMatch(/conversation-global[^}]*:hover[^}]*filter:/s);
});
```

- [ ] **Step 2: 运行测试并确认失败**

Run: `npm --prefix web/apps/dashboard test -- --run src/styles/conversation-global-layout.test.ts`

Expected: FAIL，当前仍是三卡概览与表格样式。

- [ ] **Step 3: 替换全局消息样式**

- 指标：大屏 4 列，1180px 以下 2 列，720px 以下 1 列。
- 卡片：大屏 3 列，1180px 以下 2 列，720px 以下 1 列。
- 筛选区使用 `minmax(0, 1fr)`，多选控件可换行。
- 详情抽屉最大 560px；720px 以下宽 100%。
- 卡片 hover 只改变边框、背景和阴影，不缩放。
- 删除旧 `.conversation-global-results table` 主布局依赖，保留导出等旧页面仍使用的兼容样式。

- [ ] **Step 4: 运行样式、类型和构建测试**

Run: `npm --prefix web/apps/dashboard test -- --run src/styles/conversation-global-layout.test.ts src/features/conversation-global`

Run: `npm --prefix web/apps/dashboard run typecheck`

Run: `npm --prefix web/apps/dashboard run build`

Expected: 三个命令全部 PASS。

- [ ] **Step 5: 提交视觉实现**

```bash
git add web/apps/dashboard/src/styles/conversation-global-layout.test.ts web/apps/dashboard/src/styles/index.css
git commit -m "style: align global messages with yuanhu layout"
```

---

### Task 8: 全链路回归、部署和浏览器验收

**Files:**
- Modify: `web/e2e/tests/yuanhu-phase1.spec.ts`
- Modify: `docs/phases/phase-3-dashboard/phase-3.2/reports/tasks/task-4-report.md`

**Interfaces:**
- Consumes: Tasks 1–7 的完整功能。
- Produces: 自动化与浏览器验收证据、最终实现报告。

- [ ] **Step 1: 补 E2E 失败用例**

```ts
test('global messages supports metrics, filters, focus and detail', async ({ page }) => {
  await page.goto('/chat/v2-all');
  await expect(page.getByRole('heading', { name: '全局消息' })).toBeVisible();
  await expect(page.getByRole('article', { name: '今日新增客户' })).toBeVisible();
  await page.getByRole('button', { name: /风险会话/ }).click();
  await expect(page).toHaveURL(/bucket=risk/);
  await page.getByRole('button', { name: '关注会话' }).first().click();
  await page.getByRole('button', { name: '查看会话' }).first().click();
  await expect(page.getByRole('dialog', { name: '会话详情' })).toBeVisible();
});
```

对 capability 不可用的测试数据改为断言按钮禁用和告警，不强行点击。

- [ ] **Step 2: 运行后端完整验证**

Run: `go test ./internal/dashboard ./internal/store ./internal/server ./internal/migration -count=1`

Expected: PASS。

- [ ] **Step 3: 运行前端完整验证**

Run: `npm --prefix web/apps/dashboard test -- --run`

Run: `npm --prefix web/apps/dashboard run typecheck`

Run: `npm --prefix web/apps/dashboard run build`

Expected: 全部 PASS；既有 jsdom 警告可记录，但不得有失败测试。

- [ ] **Step 4: 重建并启动 Docker 服务**

使用项目既有 D 盘 Docker Desktop 部署流程，禁止 `down --volumes`：

Run: `docker compose -p mochat-go-desktop -f deploy/standalone/docker-compose.yml up -d --build`

Run: `docker ps --filter "name=mochat-go-desktop" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"`

Expected: app、mysql、redis 均为 healthy；数据卷仍存在。

- [ ] **Step 5: 在 1440px 与 931px 浏览器验收**

逐项检查：

1. 导航和顶部 Banner 未变化。
2. 8 个指标、5 个状态、筛选、卡片、分页和详情顺序正确。
3. 931px 无页面级横向滚动；卡片 2 列，720px 以下 1 列。
4. 关注后刷新仍保留；另一用户不继承该关注。
5. 风险、超时、日期、消息类型和员工筛选 URL 可恢复。
6. 详情按钮、Escape、遮罩和关闭按钮均生效。
7. 企业微信与演示来源标记准确。
8. AI 摘要和内部群聊无数据时只集中告警，不显示伪造数字或重复占位。

- [ ] **Step 6: 更新报告并最终提交**

在任务报告写入：变更范围、接口、迁移、数据来源、能力缺口、测试数量、浏览器尺寸、容器健康和数据卷保留情况。

Run: `git diff --check`

Expected: 无 whitespace error；CRLF 提示不视为错误。

```bash
git add web/e2e/tests/yuanhu-phase1.spec.ts docs/phases/phase-3-dashboard/phase-3.2/reports/tasks/task-4-report.md
git commit -m "test: verify global message parity"
```

---

## 最终完成定义

- 设计文档 12 项验收标准全部有自动化或浏览器证据。
- 所有新增接口有 RBAC、企业、员工范围、归档来源和参数校验测试。
- 不可用指标为 `null`，风险/超时/AI/内部群聊缺口均有明确 capability。
- 当前完整前后端测试、类型检查、生产构建和 Docker 健康检查通过。
- 当前 931px 页面无横向溢出、按钮与分页 hover 字体清晰。
- 仅在上述条件全部满足后，才可声明“全局消息完善完成”。
