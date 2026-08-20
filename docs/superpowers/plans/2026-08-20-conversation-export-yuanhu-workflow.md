# 会话导出圆弧式工作流 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 `/chat/export` 从浏览器端导出最多 100 条会话摘要，升级为按员工、客户或群聊选择对象、由服务端异步生成真实消息 CSV/ZIP、可追踪并安全下载的两步导出工作流。

**Architecture:** 新增专用候选对象、任务创建/列表和下载接口；MySQL 保存任务真相、对象与员工范围快照，后台周期 worker 领取任务并从现有 10 张归档分表流式生成 ZIP。前端以两步工作台编排类型卡、真实候选表、导出确认弹窗和任务抽屉；所有筛选、分页、权限和空态由后端真实能力驱动。

**Tech Stack:** Go 1.26、MariaDB/MySQL、标准库 `archive/zip`/`encoding/csv`、React 19、TypeScript 5.9、TanStack Query 5、React Router 7、Vitest 2、Testing Library、pnpm、Docker Compose。

## Global Constraints

- 目标仅限 `/chat/export`；不得修改菜单导航、应用顶部 banner、企业切换、全局搜索和其他页面布局。
- 删除当前会话导出顶部介绍卡和独立右侧刷新；查询、刷新、重置只放在对应操作区。
- 候选对象、会话数量、预计/实际消息数和任务状态必须来自真实接口与存储，不得添加静态业务数据。
- tenant、corp、user 和员工范围只取服务端 principal；客户端不得指定或扩大数据范围。
- 候选列表和任务列表固定每页 20 条；后端拒绝其他 `pageSize`。
- 单任务最多选择 100 个对象、最多导出 100,000 条消息、日期跨度最多 2 年；不允许静默截断。
- 当前仅导出客户单聊和外部客户群消息；内部员工单聊、内部群聊与媒体二进制必须显示未接入。
- 工件固定为 UTF-8 BOM CSV + ZIP，保留 7 天，只能由同 tenant、同 corp、同创建用户下载。
- 任务工件写入 `/app/storage/conversation-exports`，复用现有 `app-storage` 命名卷；不得写入镜像层或 C 盘临时副本。
- 新迁移使用 `0144_work_message_export_tasks`；不得改写已存在的 `0140`–`0143`。
- 保留工作树中所有已有改动；每次只暂存当前任务明确列出的文件。
- 实施测试先行；每个行为先运行 RED，再做最小实现并运行 GREEN。
- Docker 只重建 `app`；禁止 `down -v`、`down --volumes` 或删除 MySQL、Redis、`app-storage` 数据。
- 主验收尺寸为 2560×1440，同时回归 1440×900、1066×1272 和 390×844。

---

## 文件结构

### 新建文件

- `internal/dashboard/work_message_export.go`：导出领域类型、参数校验、候选/任务/下载 handler 与 store 接口。
- `internal/dashboard/work_message_export_test.go`：handler 权限、固定分页、校验、幂等和下载错误码测试。
- `internal/dashboard/work_message_export_worker.go`：任务领取、CSV/ZIP 生成、租约恢复和过期清理。
- `internal/dashboard/work_message_export_worker_test.go`：CSV 安全、拆分/合并、临时文件和任务状态测试。
- `internal/store/work_message_export.go`：候选聚合、预检计数、任务生命周期和导出消息流查询。
- `internal/store/work_message_export_test.go`：SQL、员工范围、任务状态机和下载归属测试。
- `internal/store/work_message_export_integration_test.go`：MariaDB 分表、跨企业、范围与 100,000 条上限测试。
- `deploy/standalone/migrations/0144_work_message_export_tasks.up.sql`：任务表、索引和页面 API 权限资源。
- `deploy/standalone/migrations/0144_work_message_export_tasks.down.sql`：权限资源和任务表回滚。
- `web/apps/dashboard/src/features/conversation-global/conversation-export-stepper.tsx`：步骤条和三类导出卡。
- `web/apps/dashboard/src/features/conversation-global/conversation-export-stepper.test.tsx`：整卡选择、键盘和下一步测试。
- `web/apps/dashboard/src/features/conversation-global/conversation-export-candidates.tsx`：三类筛选、候选表、选择和固定分页。
- `web/apps/dashboard/src/features/conversation-global/conversation-export-candidates.test.tsx`：查询触发、跨页选择和禁用项测试。
- `web/apps/dashboard/src/features/conversation-global/conversation-export-dialog.tsx`：日期、范围、拆分/合并和最终提交。
- `web/apps/dashboard/src/features/conversation-global/conversation-export-dialog.test.tsx`：日期/数量校验、遮罩和 Escape 测试。
- `web/apps/dashboard/src/features/conversation-global/conversation-export-tasks.tsx`：任务抽屉、轮询、下载和重试。
- `web/apps/dashboard/src/features/conversation-global/conversation-export-tasks.test.tsx`：状态、轮询、下载、关闭测试。
- `web/apps/dashboard/src/styles/conversation-export-layout.test.ts`：宽屏填充、稳定交互和响应式契约。

### 修改文件

- `web/packages/api-client/src/client.ts` 与 `.test.ts`：增加带鉴权的二进制下载能力。
- `web/apps/dashboard/src/features/conversation-global/conversation-global-api.ts` 与 `.test.ts`：导出 DTO、严格解析和四个 API 方法。
- `web/apps/dashboard/src/features/conversation-global/conversation-export-page.tsx` 与 `.test.tsx`：替换临时 CSV 页面为工作流编排。
- `web/apps/dashboard/src/benchmark/page-registry.test.tsx`：补齐导出 API stub 和路由断言。
- `web/apps/dashboard/src/styles/index.css`：会话导出工作区、弹窗、抽屉和四档响应式样式。
- `internal/server/server.go` 与 `internal/server/server_test.go`：注册候选、任务和下载路由。
- `cmd/mochat-go/main.go`：注入 handler 根目录并注册导出 worker。
- `internal/config/config.go` 与 `internal/config/config_test.go`：增加导出根目录、worker 开关和间隔配置。
- `internal/dashboard/dashboard_page_catalog.json` 与 `internal/dashboard/dashboard_access_guard_test.go`：更新 `dashboard.chat.export` 资源。
- `internal/migration/migration_test.go`：增加 0144 迁移合同。
- `deploy/standalone/docker-compose.yml` 与 `deploy/standalone/.env.example`：声明导出目录和 worker 配置。
- `docs/PROJECT_PROGRESS.zh-CN.md`：实施完成后记录功能、数据来源、缺口和验证结果。

---

### Task 1: 为 API 客户端增加鉴权下载能力

**Files:**
- Modify: `web/packages/api-client/src/client.ts`
- Modify: `web/packages/api-client/src/client.test.ts`

**Interfaces:**

```ts
export type ApiDownload = { blob: Blob; filename: string };

export type ApiClient = {
  request<T>(input: RequestInfo | URL, init?: RequestInit): Promise<T>;
  download(input: RequestInfo | URL, init?: RequestInit): Promise<ApiDownload>;
};
```

- [ ] **Step 1: 写二进制、鉴权和错误失败测试**

在 `client.test.ts` 增加三条测试：成功响应保留 Bearer 头并解析 `Content-Disposition` 的 UTF-8 文件名；401 调用 `onUnauthorized`；JSON 错误响应仍转换为 `ApiError`。

```ts
it('downloads an authenticated attachment without parsing it as an envelope', async () => {
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('zip-bytes', {
    status: 200,
    headers: { 'Content-Disposition': "attachment; filename*=UTF-8''conversation-export.zip" },
  })));
  const client = createApiClient({ baseUrl: '/dashboard/', getToken: () => 'token', onUnauthorized: vi.fn() });
  const result = await client.download('/workMessage/exportDownload?taskId=7');
  expect(result.filename).toBe('conversation-export.zip');
  expect(await result.blob.text()).toBe('zip-bytes');
});
```

- [ ] **Step 2: 运行并确认 RED**

Run: `corepack pnpm --filter @mochat/api-client test -- client.test.ts`

Expected: FAIL，`download` 尚不存在。

- [ ] **Step 3: 抽取共享 fetch 与错误处理并实现 download**

`download` 必须复用 `resolveInput`、Bearer 注入、401/403/5xx 处理；只对成功响应调用 `blob()`。文件名优先解析 `filename*`，再解析 `filename`，最后回退 `conversation-export.zip`。

- [ ] **Step 4: 运行 GREEN**

Run: `corepack pnpm --filter @mochat/api-client test -- client.test.ts`

Expected: PASS。

- [ ] **Step 5: 提交**

```powershell
git add web/packages/api-client/src/client.ts web/packages/api-client/src/client.test.ts
git commit -m "feat: support authenticated artifact downloads"
```

### Task 2: 建立导出任务表和 RBAC 资源

**Files:**
- Create: `deploy/standalone/migrations/0144_work_message_export_tasks.up.sql`
- Create: `deploy/standalone/migrations/0144_work_message_export_tasks.down.sql`
- Modify: `internal/migration/migration_test.go`
- Modify: `internal/dashboard/dashboard_page_catalog.json`
- Modify: `internal/dashboard/dashboard_access_guard_test.go`

- [ ] **Step 1: 写迁移与资源目录失败测试**

测试必须断言迁移包含任务唯一键、领取索引、过期索引，并要求 `dashboard.chat.export` 绑定以下资源：

```text
GET  /dashboard/workMessage/exportCandidates
GET  /dashboard/workMessage/exportTasks
POST /dashboard/workMessage/exportTasks
GET  /dashboard/workMessage/exportDownload
```

- [ ] **Step 2: 运行并确认 RED**

Run: `go test ./internal/migration ./internal/dashboard -run 'Test.*0144|TestDashboardPageCatalog' -count=1`

Expected: FAIL，迁移和资源尚不存在。

- [ ] **Step 3: 新增任务表**

`0144` 使用以下核心结构，不另建公开下载 token 表：

```sql
CREATE TABLE `mochat_go_work_message_export_tasks` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `tenant_id` int unsigned NOT NULL,
  `corp_id` int unsigned NOT NULL,
  `user_id` int unsigned NOT NULL,
  `idempotency_key` varchar(96) NOT NULL,
  `export_type` varchar(16) NOT NULL,
  `selected_objects_json` json NOT NULL,
  `conversation_scopes_json` json NOT NULL,
  `employee_scope_json` json NOT NULL,
  `start_at` datetime NOT NULL,
  `end_at` datetime NOT NULL,
  `file_mode` varchar(16) NOT NULL,
  `format` varchar(16) NOT NULL,
  `status` varchar(16) NOT NULL DEFAULT 'pending',
  `estimated_message_count` int unsigned NOT NULL DEFAULT 0,
  `message_count` int unsigned NOT NULL DEFAULT 0,
  `file_count` int unsigned NOT NULL DEFAULT 0,
  `artifact_name` varchar(255) NOT NULL DEFAULT '',
  `artifact_path` varchar(512) NOT NULL DEFAULT '',
  `artifact_size` bigint unsigned NOT NULL DEFAULT 0,
  `artifact_sha256` char(64) NOT NULL DEFAULT '',
  `download_count` int unsigned NOT NULL DEFAULT 0,
  `last_downloaded_at` datetime NULL,
  `lease_owner` varchar(96) NOT NULL DEFAULT '',
  `lease_expires_at` datetime NULL,
  `error_code` varchar(64) NOT NULL DEFAULT '',
  `error_message` varchar(500) NOT NULL DEFAULT '',
  `expires_at` datetime NULL,
  `started_at` datetime NULL,
  `finished_at` datetime NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_mg_wmet_idempotency` (`tenant_id`,`corp_id`,`user_id`,`idempotency_key`),
  KEY `idx_mg_wmet_claim` (`status`,`lease_expires_at`,`id`),
  KEY `idx_mg_wmet_owner` (`tenant_id`,`corp_id`,`user_id`,`created_at`,`id`),
  KEY `idx_mg_wmet_expiry` (`status`,`expires_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

up 同时写入四条权限资源；down 先删资源再删表。

- [ ] **Step 4: 运行 GREEN**

Run: `go test ./internal/migration ./internal/dashboard -run 'Test.*0144|TestDashboardPageCatalog' -count=1`

Expected: PASS。

- [ ] **Step 5: 提交**

```powershell
git add deploy/standalone/migrations/0144_work_message_export_tasks.up.sql deploy/standalone/migrations/0144_work_message_export_tasks.down.sql internal/migration/migration_test.go internal/dashboard/dashboard_page_catalog.json internal/dashboard/dashboard_access_guard_test.go
git commit -m "feat: add conversation export task ledger"
```

### Task 3: 定义导出领域合同和候选接口

**Files:**
- Create: `internal/dashboard/work_message_export.go`
- Create: `internal/dashboard/work_message_export_test.go`

**Interfaces:**

```go
type WorkMessageExportType string
const (
  WorkMessageExportEmployee WorkMessageExportType = "employee"
  WorkMessageExportCustomer WorkMessageExportType = "customer"
  WorkMessageExportRoom WorkMessageExportType = "room"
)

type WorkMessageExportCandidateFilter struct {
  TenantID, CorpID, UserID int
  ExportType WorkMessageExportType
  Keyword string
  DepartmentID int
  CustomerType int
  CustomerStatus string
  TagIDs, EmployeeIDs []int
  Page, PageSize int
  RestrictEmployeeIDs bool
  AllowedEmployeeIDs []int
}

type WorkMessageExportCandidatePage struct {
  Items []WorkMessageExportCandidate `json:"items"`
  Total, Page, PageSize int
  Capabilities []WorkMessageCapability `json:"capabilities"`
  Limitations []WorkMessageStaffLimitation `json:"limitations"`
}
```

- [ ] **Step 1: 写 handler 参数和权限失败测试**

覆盖：合法三类型；`pageSize!=20` 返回 400；非法标签/员工 ID 返回 400；受限用户只能得到 `AllowedEmployeeIDs` 交集；未开通存档返回现有 provider-unavailable 语义；nil slice 规范化为 `[]`。

- [ ] **Step 2: 运行并确认 RED**

Run: `go test ./internal/dashboard -run 'TestWorkMessageExportCandidates' -count=1`

Expected: FAIL，类型和 handler 尚不存在。

- [ ] **Step 3: 实现只读候选 handler**

增加 `WorkMessageExportStore` 的候选方法：

```go
type WorkMessageExportStore interface {
  ExportCandidates(context.Context, WorkMessageExportCandidateFilter) (WorkMessageExportCandidatePage, error)
}
```

handler 复用 `resolveAuthorized(..., workMessageConversationPermissionKey)` 和 `workMessageArchiveAllowed`，只从 principal 构造 tenant/corp/user 和员工范围。

- [ ] **Step 4: 运行 GREEN**

Run: `go test ./internal/dashboard -run 'TestWorkMessageExportCandidates' -count=1`

Expected: PASS。

- [ ] **Step 5: 提交**

```powershell
git add internal/dashboard/work_message_export.go internal/dashboard/work_message_export_test.go
git commit -m "feat: define conversation export candidates"
```

### Task 4: 实现三类真实候选聚合

**Files:**
- Create: `internal/store/work_message_export.go`
- Create: `internal/store/work_message_export_test.go`
- Create: `internal/store/work_message_export_integration_test.go`

- [ ] **Step 1: 写候选 SQL 和 MariaDB 失败测试**

测试固定以下口径：

- 员工来自 `mc_work_employee`，部门包含后代部门，客户/外部群会话数来自共享 archive union。
- 客户来自实际有归档的客户集合，类型取 `mc_work_contact.type`，关系取 `mc_work_contact_employee`，标签取 tag pivot，单聊/群聊数量与客户会话页一致。
- 群来自 `mc_work_room`，成员取 `mc_work_contact_room`，消息数使用规范化后的有效群 ID。
- `RestrictEmployeeIDs=true` 且范围为空时结果必须为空。
- 所有类型固定 `LIMIT 20 OFFSET ?`，总数与列表使用相同过滤口径。

- [ ] **Step 2: 运行并确认 RED**

Run: `go test ./internal/store -run 'TestWorkMessageExportCandidate' -count=1`

Expected: FAIL，store 方法尚不存在。

- [ ] **Step 3: 复用共享归档来源实现聚合**

调用 `customerDirectoryArchiveMode`、`effectiveArchiveSource` 和 `workMessageFilteredUnionSQLWithArchiveSourceState`，不要直接把 10 张表写成另一套忽略真实/模拟来源的 union。返回 capability：

```go
[]dashboard.WorkMessageCapability{
  {Key: "customerDirect", Available: true},
  {Key: "externalRoom", Available: true},
  {Key: "internalDirect", Available: false, Reason: "当前归档数据未提供内部员工单聊"},
  {Key: "internalRoom", Available: false, Reason: "当前归档数据未提供内部群聊"},
  {Key: "mediaBinary", Available: false, Reason: "当前导出只包含消息摘要和元数据"},
}
```

- [ ] **Step 4: 运行单元与集成 GREEN**

Run: `go test ./internal/store -run 'TestWorkMessageExportCandidate' -count=1`

Run with configured MariaDB: `go test ./internal/store -run 'TestWorkMessageExportIntegration/Candidates' -count=1`

Expected: PASS；未配置集成 DSN 时测试按项目现有约定显式 SKIP。

- [ ] **Step 5: 提交**

```powershell
git add internal/store/work_message_export.go internal/store/work_message_export_test.go internal/store/work_message_export_integration_test.go
git commit -m "feat: aggregate conversation export candidates"
```

### Task 5: 实现任务创建、预检、幂等和列表

**Files:**
- Modify: `internal/dashboard/work_message_export.go`
- Modify: `internal/dashboard/work_message_export_test.go`
- Modify: `internal/store/work_message_export.go`
- Modify: `internal/store/work_message_export_test.go`

**Interfaces:**

```go
type WorkMessageExportCreate struct {
  ExportType WorkMessageExportType `json:"exportType"`
  SelectedIDs []int `json:"selectedIds"`
  StartDate, EndDate string
  ConversationScopes []string `json:"conversationScopes"`
  FileMode string `json:"fileMode"`
  Format string `json:"format"`
  IdempotencyKey string `json:"idempotencyKey"`
}

type WorkMessageExportTask struct {
  ID int64 `json:"id"`
  ExportType, Status string
  SelectedCount, EstimatedMessageCount, MessageCount, FileCount int
  StartAt, EndAt, CreatedAt, StartedAt, FinishedAt, ExpiresAt string
  ArtifactName, ErrorCode, ErrorMessage string
}
```

- [ ] **Step 1: 写业务校验和幂等失败测试**

覆盖：日期格式和两年跨度；最多 100 个去重正整数；合法 scope 映射；固定 `csv_zip`；0 条返回 422；超过 100,000 返回 422；越权对象返回 404；重复幂等键返回同一任务；列表只能看当前用户且固定 20 条。

- [ ] **Step 2: 运行并确认 RED**

Run: `go test ./internal/dashboard ./internal/store -run 'TestWorkMessageExport(Create|TaskList|Preflight|Idempot)' -count=1`

Expected: FAIL。

- [ ] **Step 3: 实现创建事务和任务列表**

store 在同一事务内重新加载对象名称快照、验证对象和员工范围、执行与 worker 相同条件的 `COUNT(*)`，再插入 `pending` 任务。唯一键冲突后按 tenant/corp/user/idempotency key 读取原任务，不创建重复记录。

日期转换使用上海时区半开区间：

```go
startAt := time.Date(y, m, d, 0, 0, 0, 0, shanghai)
endAt := endDate.AddDate(0, 0, 1)
```

- [ ] **Step 4: 运行 GREEN**

Run: `go test ./internal/dashboard ./internal/store -run 'TestWorkMessageExport(Create|TaskList|Preflight|Idempot)' -count=1`

Expected: PASS。

- [ ] **Step 5: 提交**

```powershell
git add internal/dashboard/work_message_export.go internal/dashboard/work_message_export_test.go internal/store/work_message_export.go internal/store/work_message_export_test.go
git commit -m "feat: create and list conversation export tasks"
```

### Task 6: 实现安全的 CSV/ZIP 导出 worker

**Files:**
- Create: `internal/dashboard/work_message_export_worker.go`
- Create: `internal/dashboard/work_message_export_worker_test.go`
- Modify: `internal/store/work_message_export.go`
- Modify: `internal/store/work_message_export_test.go`
- Modify: `internal/store/work_message_export_integration_test.go`

- [ ] **Step 1: 写任务状态机、文件和 CSV 安全失败测试**

覆盖：事务领取 pending；租约过期的 running 可恢复；同一任务不能双领；拆分/合并文件数；UTF-8 BOM；逗号/引号/换行；以 `= + - @` 开头的单元格加单引号；超过 100,000 立即失败且不保留工件；写失败删除 `.tmp`；成功原子改名并保存 SHA-256。

```go
func safeCSVText(value string) string {
  trimmed := strings.TrimLeft(value, "\t\r\n ")
  if trimmed != "" && strings.ContainsRune("=+-@", rune(trimmed[0])) {
    return "'" + value
  }
  return value
}
```

- [ ] **Step 2: 运行并确认 RED**

Run: `go test ./internal/dashboard ./internal/store -run 'TestWorkMessageExportWorker|TestWorkMessageExportClaim|TestSafeCSV' -count=1`

Expected: FAIL。

- [ ] **Step 3: 实现领取、消息流和工件生成**

worker 每次领取一条任务；查询按 `msg_data_time, seq, table_index, id` 稳定排序，输出列固定为：

```text
对象类型,对象ID,对象名称,会话ID,员工ID,员工名称,发送者,方向,消息类型,消息内容,发送时间,归档来源,归档来源ID
```

内容优先使用 `content_text`；空文本输出解析后的类型摘要，不读取或复制媒体磁盘文件。ZIP 临时文件与最终文件位于同一目录，确保原子改名。

- [ ] **Step 4: 实现过期清理和运行台账**

每轮先把租约超时任务恢复为 pending，再删除 `expires_at<=NOW()` 的成功工件并将状态改为 `expired`。worker 通过现有 `startQueueItemExecution`/`taskrunner` 记录 running、succeeded、failed，但任务业务状态仍以新表为准。

- [ ] **Step 5: 运行 GREEN 和 MariaDB 回归**

Run: `go test ./internal/dashboard ./internal/store -run 'TestWorkMessageExportWorker|TestWorkMessageExportClaim|TestSafeCSV' -count=1`

Run with configured MariaDB: `go test ./internal/store -run 'TestWorkMessageExportIntegration/(Preflight|MessageStream|Scope)' -count=1`

Expected: PASS 或按项目约定显式 SKIP 集成测试。

- [ ] **Step 6: 提交**

```powershell
git add internal/dashboard/work_message_export_worker.go internal/dashboard/work_message_export_worker_test.go internal/store/work_message_export.go internal/store/work_message_export_test.go internal/store/work_message_export_integration_test.go
git commit -m "feat: generate conversation export artifacts"
```

### Task 7: 接入安全下载、路由、配置和 worker

**Files:**
- Modify: `internal/dashboard/work_message_export.go`
- Modify: `internal/dashboard/work_message_export_test.go`
- Modify: `internal/server/server.go`
- Modify: `internal/server/server_test.go`
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `cmd/mochat-go/main.go`
- Modify: `deploy/standalone/docker-compose.yml`
- Modify: `deploy/standalone/.env.example`

- [ ] **Step 1: 写下载、路由和配置失败测试**

覆盖：相同创建者成功下载；跨用户/跨 corp 返回 404；pending/failed 返回 409；expired 返回 410；文件缺失返回 500；`../` 和绝对路径被拒绝；server 路由清单包含四个资源；默认根目录为 `./storage/conversation-exports`。

- [ ] **Step 2: 运行并确认 RED**

Run: `go test ./internal/dashboard ./internal/server ./internal/config -run 'TestWorkMessageExportDownload|Test.*ExportRoute|Test.*ConversationExport' -count=1`

Expected: FAIL。

- [ ] **Step 3: 实现下载和安全路径**

下载 handler 从任务表读取相对路径，使用 `filepath.Abs` + `filepath.Rel` 确保目标在根目录内；设置：

```go
w.Header().Set("Content-Type", "application/zip")
w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": task.ArtifactName}))
w.Header().Set("X-Content-Type-Options", "nosniff")
```

成功打开文件后流式复制，并在 store 中增加下载次数。

- [ ] **Step 4: 注册配置和 worker**

新增配置：

```text
MOCHAT_GO_CONVERSATION_EXPORT_ROOT=/app/storage/conversation-exports
MOCHAT_GO_ENABLE_CONVERSATION_EXPORT_WORKER=1
MOCHAT_GO_CONVERSATION_EXPORT_WORKER_INTERVAL_SECONDS=2
```

`cmd/mochat-go/main.go` 将根目录注入 handler，并在 worker group 注册 `cron-work-message-export`。Compose 继续使用现有 `app-storage:/app/storage`，不新增卷。

- [ ] **Step 5: 运行 GREEN**

Run: `go test ./internal/dashboard ./internal/server ./internal/config -run 'TestWorkMessageExportDownload|Test.*ExportRoute|Test.*ConversationExport' -count=1`

Expected: PASS。

- [ ] **Step 6: 提交**

```powershell
git add internal/dashboard/work_message_export.go internal/dashboard/work_message_export_test.go internal/server/server.go internal/server/server_test.go internal/config/config.go internal/config/config_test.go cmd/mochat-go/main.go deploy/standalone/docker-compose.yml deploy/standalone/.env.example
git commit -m "feat: wire conversation export worker and download"
```

### Task 8: 扩展前端导出 API 合同

**Files:**
- Modify: `web/apps/dashboard/src/features/conversation-global/conversation-global-api.ts`
- Modify: `web/apps/dashboard/src/features/conversation-global/conversation-global-api.test.ts`

**Interfaces:**

```ts
export type ConversationExportType = 'employee' | 'customer' | 'room';
export type ConversationExportStatus = 'pending' | 'running' | 'succeeded' | 'failed' | 'expired';

export type ConversationExportCandidatePage = {
  items: readonly ConversationExportCandidate[];
  total: number;
  page: number;
  pageSize: 20;
  capabilities: readonly ConversationCapability[];
  limitations: readonly { key: string; reason: string }[];
};

export type ConversationExportApi = {
  exportCandidates(input: ConversationExportCandidateInput): Promise<ConversationExportCandidatePage>;
  createExportTask(input: ConversationExportCreateInput): Promise<ConversationExportTask>;
  exportTasks(input: { page: number; status: '' | ConversationExportStatus }): Promise<ConversationExportTaskPage>;
  downloadExportTask(taskId: number): Promise<{ blob: Blob; filename: string }>;
};
```

- [ ] **Step 1: 写序列化和严格解析失败测试**

断言多值 `tagIds`/`employeeIds`、固定 `pageSize=20`、POST JSON、任务状态联合、无效 item/status 抛错，以及下载调用 `client.download`。

- [ ] **Step 2: 运行并确认 RED**

Run: `corepack pnpm --filter @mochat/dashboard test -- conversation-global-api.test.ts`

Expected: FAIL。

- [ ] **Step 3: 实现类型、解析器和 API 方法**

扩展 `ConversationGlobalApi`，不同候选类型使用 `kind` 判别联合；数字、字符串、数组和 `pageSize===20` 均严格校验。下载不走 JSON envelope 解析。

- [ ] **Step 4: 运行 GREEN**

Run: `corepack pnpm --filter @mochat/dashboard test -- conversation-global-api.test.ts`

Expected: PASS。

- [ ] **Step 5: 提交**

```powershell
git add web/apps/dashboard/src/features/conversation-global/conversation-global-api.ts web/apps/dashboard/src/features/conversation-global/conversation-global-api.test.ts
git commit -m "feat: add conversation export api contract"
```

### Task 9: 实现步骤条、类型卡和候选对象工作区

**Files:**
- Create: `web/apps/dashboard/src/features/conversation-global/conversation-export-stepper.tsx`
- Create: `web/apps/dashboard/src/features/conversation-global/conversation-export-stepper.test.tsx`
- Create: `web/apps/dashboard/src/features/conversation-global/conversation-export-candidates.tsx`
- Create: `web/apps/dashboard/src/features/conversation-global/conversation-export-candidates.test.tsx`
- Modify: `web/apps/dashboard/src/features/conversation-global/conversation-export-page.tsx`
- Modify: `web/apps/dashboard/src/features/conversation-global/conversation-export-page.test.tsx`

- [ ] **Step 1: 写页面结构和交互失败测试**

覆盖：不存在旧介绍文案和顶部刷新；三卡整卡可点击；未选择时下一步禁用；URL 恢复步骤/类型/页码；输入时不请求，查询/回车只请求一次；刷新不提交草稿；重置清空条件和选择；当前页全选、跨页保留和禁用 0 消息项。

- [ ] **Step 2: 运行并确认 RED**

Run: `corepack pnpm --filter @mochat/dashboard test -- conversation-export-stepper.test.tsx conversation-export-candidates.test.tsx conversation-export-page.test.tsx`

Expected: FAIL。

- [ ] **Step 3: 实现最小两步编排**

页面只把已提交条件写入 URL。选择项保存在 `Map<number, CandidateSnapshot>`，以对象 ID 为键；类型改变时清空选择。候选查询 key 必须包含 corp、类型和全部已提交条件。

```tsx
<section className="conversation-export-workspace">
  <ConversationExportStepper step={step} exportType={exportType} onSelect={selectType} onNext={goSelect} />
  {step === 'select' && <ConversationExportCandidates {...candidateProps} />}
  <button type="button" onClick={() => setTasksOpen(true)}>导出记录</button>
</section>
```

- [ ] **Step 4: 运行 GREEN**

Run: `corepack pnpm --filter @mochat/dashboard test -- conversation-export-stepper.test.tsx conversation-export-candidates.test.tsx conversation-export-page.test.tsx`

Expected: PASS。

- [ ] **Step 5: 提交**

```powershell
git add web/apps/dashboard/src/features/conversation-global/conversation-export-stepper.tsx web/apps/dashboard/src/features/conversation-global/conversation-export-stepper.test.tsx web/apps/dashboard/src/features/conversation-global/conversation-export-candidates.tsx web/apps/dashboard/src/features/conversation-global/conversation-export-candidates.test.tsx web/apps/dashboard/src/features/conversation-global/conversation-export-page.tsx web/apps/dashboard/src/features/conversation-global/conversation-export-page.test.tsx
git commit -m "feat: build conversation export selection workflow"
```

### Task 10: 实现导出确认弹窗和任务抽屉

**Files:**
- Create: `web/apps/dashboard/src/features/conversation-global/conversation-export-dialog.tsx`
- Create: `web/apps/dashboard/src/features/conversation-global/conversation-export-dialog.test.tsx`
- Create: `web/apps/dashboard/src/features/conversation-global/conversation-export-tasks.tsx`
- Create: `web/apps/dashboard/src/features/conversation-global/conversation-export-tasks.test.tsx`
- Modify: `web/apps/dashboard/src/features/conversation-global/conversation-export-page.tsx`
- Modify: `web/apps/dashboard/src/features/conversation-global/conversation-export-page.test.tsx`
- Modify: `web/apps/dashboard/src/benchmark/page-registry.test.tsx`

- [ ] **Step 1: 写弹窗、任务和下载失败测试**

覆盖：日期必填、顺序和两年限制；可用 scope；拆分/合并；创建中禁用重复提交；成功后打开任务抽屉；活动任务每 3 秒轮询、终态停止；下载使用服务端文件名；失败重试生成新幂等键；关闭按钮、遮罩、Escape 生效，内容区点击不关闭。

- [ ] **Step 2: 运行并确认 RED**

Run: `corepack pnpm --filter @mochat/dashboard test -- conversation-export-dialog.test.tsx conversation-export-tasks.test.tsx conversation-export-page.test.tsx page-registry.test.tsx`

Expected: FAIL。

- [ ] **Step 3: 实现弹窗和任务抽屉**

创建 mutation 成功后使 `['corp', corpId, 'conversation-export-tasks']` 失效并打开抽屉。下载成功后使用对象 URL，触发后立即撤销；下载失败不得生成空文件。

任务查询使用：

```ts
refetchInterval: (query) => query.state.data?.items.some(
  (item) => item.status === 'pending' || item.status === 'running',
) ? 3000 : false
```

- [ ] **Step 4: 运行 GREEN**

Run: `corepack pnpm --filter @mochat/dashboard test -- conversation-export-dialog.test.tsx conversation-export-tasks.test.tsx conversation-export-page.test.tsx page-registry.test.tsx`

Expected: PASS。

- [ ] **Step 5: 提交**

```powershell
git add web/apps/dashboard/src/features/conversation-global/conversation-export-dialog.tsx web/apps/dashboard/src/features/conversation-global/conversation-export-dialog.test.tsx web/apps/dashboard/src/features/conversation-global/conversation-export-tasks.tsx web/apps/dashboard/src/features/conversation-global/conversation-export-tasks.test.tsx web/apps/dashboard/src/features/conversation-global/conversation-export-page.tsx web/apps/dashboard/src/features/conversation-global/conversation-export-page.test.tsx web/apps/dashboard/src/benchmark/page-registry.test.tsx
git commit -m "feat: add conversation export jobs and downloads"
```

### Task 11: 完成圆弧式布局和稳定交互样式

**Files:**
- Modify: `web/apps/dashboard/src/styles/index.css`
- Create: `web/apps/dashboard/src/styles/conversation-export-layout.test.ts`

- [ ] **Step 1: 写样式合同失败测试**

断言：工作区无 `max-width`；宽屏三卡；高度填充；第二步 grid 为步骤条/筛选/表格/底栏；表格独立滚动；任务抽屉 420px；遮罩层；1600/1200/768 三个断点；hover/selected/pagination 不包含 `transform`。

- [ ] **Step 2: 运行并确认 RED**

Run: `corepack pnpm --filter @mochat/dashboard test -- conversation-export-layout.test.ts`

Expected: FAIL，旧样式仍是 1320px 卡片页。

- [ ] **Step 3: 替换会话导出样式块**

核心布局：

```css
.conversation-export-workspace {
  background: #fff;
  border: 1px solid #e5e9f1;
  border-radius: 14px;
  display: grid;
  grid-template-rows: auto minmax(0, 1fr);
  height: calc(100dvh - 72px - 32px);
  min-height: 620px;
  min-width: 0;
  overflow: hidden;
}

.conversation-export-type-grid {
  display: grid;
  gap: 64px;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  margin: auto;
  max-width: 1180px;
  width: calc(100% - 64px);
}

.conversation-export-select-layout {
  display: grid;
  grid-template-rows: auto minmax(0, 1fr) auto;
  min-height: 0;
}
```

所有 hover 仅改变背景、边框、颜色和阴影，不位移元素。

- [ ] **Step 4: 运行 GREEN 和前端回归**

Run: `corepack pnpm --filter @mochat/dashboard test -- conversation-export-layout.test.ts`

Run: `corepack pnpm --filter @mochat/dashboard test`

Run: `corepack pnpm --filter @mochat/dashboard typecheck`

Run: `corepack pnpm --filter @mochat/dashboard build`

Expected: 全部 PASS。

- [ ] **Step 5: 提交**

```powershell
git add web/apps/dashboard/src/styles/index.css web/apps/dashboard/src/styles/conversation-export-layout.test.ts
git commit -m "style: align conversation export with yuanhu workflow"
```

### Task 12: 全量验证、Docker 部署和浏览器验收

**Files:**
- Modify: `docs/PROJECT_PROGRESS.zh-CN.md`

- [ ] **Step 1: 运行后端和前端完整验证**

```powershell
go test ./internal/dashboard ./internal/store ./internal/server ./internal/migration ./internal/config -count=1
corepack pnpm --filter @mochat/api-client test
corepack pnpm --filter @mochat/dashboard test
corepack pnpm --filter @mochat/dashboard typecheck
corepack pnpm --filter @mochat/dashboard build
git diff --check
```

Expected: 全部 PASS；集成测试若因未配置 DSN 跳过，报告中明确列出，不能写成通过。

- [ ] **Step 2: 检查卷并只重建 app**

```powershell
docker compose -p mochat-go-desktop -f deploy/standalone/docker-compose.yml ps
docker volume ls --filter label=com.docker.compose.project=mochat-go-desktop
docker compose -p mochat-go-desktop --env-file deploy/standalone/.env.local -f deploy/standalone/docker-compose.yml up -d --build --no-deps app
docker compose -p mochat-go-desktop -f deploy/standalone/docker-compose.yml ps
```

Expected: app、MySQL、Redis 健康；原 `app-storage`、MySQL 和 Redis 卷仍存在。

- [ ] **Step 3: 浏览器验收四种尺寸**

在 `http://127.0.0.1:18080/chat/export` 验证：

1. 没有顶部介绍卡，菜单和 banner 未变。
2. 三卡整卡选择、下一步、上一步和 URL 恢复。
3. 员工/客户/群三类查询、刷新、重置、固定分页和跨页选择。
4. 输入过程中不发请求，查询只发一次请求。
5. 0 消息对象不可选；内部会话和媒体能力明确显示未接入。
6. 导出参数校验、幂等创建、任务轮询、失败说明和下载。
7. 弹窗与抽屉的按钮、遮罩和 Escape 关闭。
8. 2560×1440、1440×900、1066×1272、390×844 无抖动、模糊和异常横向滚动。
9. 控制台无未解释错误，关键请求的 tenant/corp/员工范围一致。

- [ ] **Step 4: 做真实小任务验收**

选择一条真实有消息的对象、一天时间范围创建任务；等待完成并下载 ZIP。检查：CSV 行数等于任务 `messageCount`，中文/换行正常，公式前缀安全，数据库保存相对路径，工件位于容器 `/app/storage/conversation-exports`。

- [ ] **Step 5: 更新进度文档并提交**

在 `docs/PROJECT_PROGRESS.zh-CN.md` 记录实际改动、真实数据来源、未接入能力、测试结果、浏览器尺寸、容器健康和卷状态。

```powershell
git add docs/PROJECT_PROGRESS.zh-CN.md
git commit -m "docs: record conversation export delivery"
```

---

## 完成定义

只有同时满足以下条件才可宣称会话导出优化完成：

- 旧的浏览器摘要 CSV 能力已移除，三类两步导出工作流可用。
- 候选、数量、消息、任务和下载均来自专用真实后端链路。
- 权限、范围、幂等、上限、CSV 注入、路径穿越、租约恢复和过期清理有自动化测试。
- 前端全量测试、类型检查、生产构建和相关 Go 测试通过。
- 本地 Docker app/MySQL/Redis 健康且原数据卷保留。
- 真实小任务能生成并下载正确 ZIP，四种尺寸浏览器验收通过。
- 所有能力缺口均明确显示，没有静态假数据、无效按钮或静默截断。
