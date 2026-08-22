# 渠道活码与群活码页面圆弧 AI 对齐实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在保留 MoChat 三条真实业务链路的前提下，将渠道活码、无限群活码和自建群活码改造成与圆弧 AI 信息架构一致、数据可验证、交互可闭环的专用页面。

> 2026-08-22 集成说明：本计划中的 `/groupCode/*` 与 `/groupCodeGroup/*` 是未落地的后续设想；本期权威实现按 `2026-08-22-p0-live-code-integration-design.md` 收敛为已注册的 `channelCode` 与 `workRoomAutoPull`，不得据此文档暴露未注册接口。

**Architecture:** 渠道活码继续使用 channelCode，无限群活码继续使用 workRoomAutoPull，自建群活码继续使用 roomInfinitePull；后端新增展示聚合、生命周期、分组、事件归因和导出能力，前端拆成两个专用页面并共享轻量工作区组件。写入仍进入既有领域方法，不通过内部 HTTP 转发。

**Tech Stack:** Go 1.26、MySQL 8、React 19、TypeScript 5.9、Ant Design 6、TanStack Query、React Router、Vitest、Testing Library、Playwright、Docker Compose。

## Global Constraints

- 设计依据：[渠道活码与群活码页面圆弧 AI 对齐设计](../specs/2026-08-21-channel-and-group-live-code-yuanhu-redesign-design.md)。
- 开始实施前使用 using-git-worktrees 创建隔离工作树；当前工作区存在大量无关修改，不得清理、覆盖或顺带提交。
- 每个任务遵循 TDD：先写失败测试并确认按预期失败，再实现最小代码，再运行相关回归。
- 所有查询、写入、导出和回调必须按企业/租户隔离并经过服务端权限校验。
- 统计值 0 表示已验证为零，null 表示无可验证统计；不得互相转换。
- 不在前端过滤 example.invalid 或构造二维码、扫码数、入群数；模拟数据由迁移标记并由服务端默认排除。
- 不展示独立刷新按钮、大型二级渐变头图、“已连接”徽标或没有业务价值的摘要数字。
- 圆弧 AI 空列表未展示的行内操作，以设计文档中已确定的 MoChat 闭环为准。
- 写接口失败时不得先行更新为成功状态；企微部分失败必须逐条返回并可重试。
- 每个任务只提交本任务文件；提交前运行 git diff --check，不得混入其他改动。

---

## Task 1：建立数据库结构、迁移契约与权限资源

**Files:**

- Create: deploy/standalone/migrations/0153_live_code_workspace.up.sql
- Create: deploy/standalone/migrations/0153_live_code_workspace.down.sql
- Create: internal/migration/live_code_workspace_migration_test.go
- Modify: internal/migration/migration_test.go
- Modify: internal/dashboard/dashboard_page_catalog.json

- [ ] **Step 1: 写迁移失败测试**

在 live_code_workspace_migration_test.go 中验证以下关键片段，并验证 down migration 逆序删除新增对象：

~~~go
func TestLiveCodeWorkspaceMigrationContract(t *testing.T) {
    up := migrationSQL(t, "0153_live_code_workspace.up.sql")
    required := []string{
        "ALTER TABLE mc_channel_code",
        "validity_kind", "valid_from", "valid_until",
        "lifecycle_state", "provider_state", "data_source",
        "CREATE TABLE mc_group_code_group",
        "CREATE TABLE mc_live_code_event",
        "UNIQUE KEY uk_live_code_event_source",
    }
    for _, fragment := range required {
        if !strings.Contains(up, fragment) {
            t.Fatalf("migration missing %q", fragment)
        }
    }
}
~~~

- [ ] **Step 2: 运行测试并确认失败**

Run:

~~~bash
go test ./internal/migration -run 'TestLiveCodeWorkspaceMigrationContract|TestMigrationFiles' -count=1
~~~

Expected: FAIL，提示缺少 0150 migration 或必需字段。

- [ ] **Step 3: 编写可回滚迁移**

迁移必须包含：

~~~sql
ALTER TABLE mc_channel_code
  ADD COLUMN validity_kind VARCHAR(16) NOT NULL DEFAULT 'permanent',
  ADD COLUMN valid_from DATETIME NULL,
  ADD COLUMN valid_until DATETIME NULL,
  ADD COLUMN lifecycle_state VARCHAR(16) NOT NULL DEFAULT 'active',
  ADD COLUMN provider_state VARCHAR(16) NOT NULL DEFAULT 'synced',
  ADD COLUMN provider_error VARCHAR(512) NOT NULL DEFAULT '',
  ADD COLUMN data_source VARCHAR(16) NOT NULL DEFAULT 'business';

CREATE TABLE mc_group_code_group (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  corp_id BIGINT NOT NULL,
  name VARCHAR(30) NOT NULL,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  deleted_at DATETIME NULL,
  active_name VARCHAR(30) GENERATED ALWAYS AS
    (CASE WHEN deleted_at IS NULL THEN name ELSE NULL END) STORED,
  UNIQUE KEY uk_group_code_group_corp_name (corp_id, active_name)
);

CREATE TABLE mc_live_code_event (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  corp_id BIGINT NOT NULL,
  mode VARCHAR(20) NOT NULL,
  code_id BIGINT NOT NULL,
  contact_id BIGINT NULL,
  room_id BIGINT NULL,
  event_type VARCHAR(24) NOT NULL,
  source_event_id VARCHAR(128) NOT NULL,
  occurred_at DATETIME NOT NULL,
  created_at DATETIME NOT NULL,
  UNIQUE KEY uk_live_code_event_source (corp_id, mode, source_event_id),
  KEY idx_live_code_event_daily (corp_id, mode, code_id, event_type, occurred_at)
);
~~~

同时为 mc_work_room_auto_pull、mc_room_infinite 增加 group_id、lifecycle_state、data_source；为 mc_room_infinite 增加 code_type。仅按 example.invalid 域名与 sim- 名称的组合条件标记开发模拟记录，避免单字段误伤历史业务数据。

- [ ] **Step 4: 增加页面动作权限**

在页面目录和 0150 migration 中为两个页面建立 view、create、edit、statistics、export、move、invalidate、upload 动作。SQL 必须幂等，写入现有 mochat_go_dashboard_permission_resources 体系。

- [ ] **Step 5: 验证并提交**

~~~bash
go test ./internal/migration -run 'TestLiveCodeWorkspaceMigrationContract|TestMigrationFiles|TestDashboardPage' -count=1
git add deploy/standalone/migrations/0153_live_code_workspace.* internal/migration/live_code_workspace_migration_test.go internal/migration/migration_test.go internal/dashboard/dashboard_page_catalog.json
git diff --cached --check
git commit -m "feat: add live code workspace schema"
~~~

Expected: 测试 PASS，提交只包含上述文件。

---

## Task 2：扩展渠道活码展示模型与真实筛选

**Files:**

- Modify: internal/dashboard/channel_code.go
- Modify: internal/dashboard/channel_code_routes.go
- Modify: internal/dashboard/channel_code_test.go
- Create: internal/store/channel_code_workspace.go
- Create: internal/store/channel_code_workspace_test.go
- Modify: internal/store/mysql.go

- [ ] **Step 1: 写 handler 和 store 失败测试**

覆盖名称、创建人、员工/部门、分组、状态筛选；验证创建人、创建日期、员工、部门、标签、有效期、状态和新增好友数。测试证明跨企业记录和 data_source=simulation 默认不可见。

~~~go
if got.Creator.Name != "李娜" || got.CreatedAt == "" {
    t.Fatalf("creator metadata missing: %#v", got)
}
if got.AddedFriendCount.Valid {
    t.Fatalf("unverified count must remain invalid")
}
~~~

- [ ] **Step 2: 运行测试并确认失败**

~~~bash
go test ./internal/dashboard ./internal/store -run 'TestChannelCodeWorkspace' -count=1
~~~

Expected: FAIL，缺少扩展字段或工作区查询。

- [ ] **Step 3: 增加展示类型和批量查询**

在 channel_code.go 增加 ChannelCodeWorkspaceFilter、ChannelCodeWorkspaceItem、NullableCount。专用 store 查询须：

- 从 mc_channel_code.created_at 取创建日期；
- 从创建业务日志 event=100 的 operation_id 解析创建员工；
- 从 drainage_employee 批量解析员工和部门；
- 从标签关系解析标签名；
- 从 work_contact_employee.state=channelCode-{id} 统计可验证新增好友；
- 所有 join 带企业边界、软删除条件并避免 N+1。

- [ ] **Step 4: 兼容扩展 GET /dashboard/channelCode/index**

保留旧字段并新增 creator、createdAt、employees、validity、state、addedFriendCount、statisticsAvailable。无可靠统计输出 addedFriendCount:null；非法状态、页码和越权员工筛选返回 400/403。

- [ ] **Step 5: 回归并提交**

~~~bash
go test ./internal/dashboard ./internal/store -run 'TestChannelCodeWorkspace|TestChannelCode' -count=1
git add internal/dashboard/channel_code.go internal/dashboard/channel_code_routes.go internal/dashboard/channel_code_test.go internal/store/channel_code_workspace.go internal/store/channel_code_workspace_test.go internal/store/mysql.go
git diff --cached --check
git commit -m "feat: enrich channel code workspace data"
~~~

Expected: PASS。

---

## Task 3：实现渠道活码有效期、批量作废和企微同步

**Files:**

- Modify: internal/dashboard/channel_code.go
- Modify: internal/dashboard/channel_code_write.go
- Modify: internal/dashboard/channel_code_cron.go
- Modify: internal/dashboard/channel_code_wecom.go
- Modify: internal/dashboard/channel_code_test.go
- Modify: internal/dashboard/channel_code_cron_test.go
- Modify: internal/store/channel_code_workspace.go
- Modify: internal/server/server.go
- Modify: internal/server/server_test.go
- Modify: cmd/mochat-go/main.go

- [ ] **Step 1: 写有限有效期和部分失败测试**

覆盖永久、固定期限、到期成功失效、供应商失败转 sync_failed、批量作废部分成功、重复作废幂等。

- [ ] **Step 2: 确认测试失败**

~~~bash
go test ./internal/dashboard -run 'TestChannelCode.*Validity|TestChannelCodeBatchInvalidate|TestChannelCodeCronExpires' -count=1
~~~

Expected: FAIL，缺少 provider 删除方法和作废 handler。

- [ ] **Step 3: 扩展企微客户端**

为 ChannelCodeWeComClient 增加：

~~~go
DeleteContactWay(ctx context.Context, accessToken, configID string) error
~~~

channel_code_wecom.go 调用企微“结束/删除联系我”能力并严格检查 HTTP 状态与 errcode。只有供应商确认成功后才写 provider_state=synced 和 lifecycle_state=expired|invalidated。

- [ ] **Step 4: 保存有效期并处理到期**

创建/更新校验 valid_until 大于 valid_from；永久模式清空起止时间。Cron 使用条件更新避免多实例重复执行；失败写 sync_failed/provider_error 并按退避重试，不使用 deleted_at 隐藏历史。

- [ ] **Step 5: 新增批量作废路由**

新增 POST /dashboard/channelCode/batchInvalidate，请求最多 100 个 ID：

~~~json
{"ids":[1,2]}
~~~

逐项响应：

~~~json
{"items":[{"id":1,"success":true},{"id":2,"success":false,"error":"企微同步失败"}]}
~~~

- [ ] **Step 6: 接线、回归和提交**

~~~bash
go test ./internal/dashboard ./internal/server -run 'TestChannelCode|TestServer.*ChannelCode' -count=1
git add internal/dashboard/channel_code* internal/store/channel_code_workspace.go internal/server/server.go internal/server/server_test.go cmd/mochat-go/main.go
git diff --cached --check
git commit -m "feat: add channel code lifecycle controls"
~~~

Expected: PASS，EnabledRoutes 包含新路由。

---

## Task 4：实现渠道活码聚合统计和 CSV 导出

**Files:**

- Create: internal/dashboard/channel_code_workspace.go
- Create: internal/dashboard/channel_code_workspace_test.go
- Modify: internal/store/channel_code_workspace.go
- Modify: internal/server/server.go
- Modify: internal/server/server_test.go
- Modify: cmd/mochat-go/main.go

- [ ] **Step 1: 写统计与导出失败测试**

固定 Asia/Shanghai 自然日口径，覆盖重复添加、流失、留存、分组/名称筛选、无数据 null、CSV BOM、列顺序和公式注入。以 =、+、-、@ 开头的 CSV 单元格必须加单引号。

- [ ] **Step 2: 确认失败**

~~~bash
go test ./internal/dashboard ./internal/store -run 'TestChannelCodeWorkspaceStatistics|TestChannelCodeWorkspaceExport' -count=1
~~~

Expected: FAIL，缺少聚合 handler。

- [ ] **Step 3: 实现接口**

新增：

~~~text
GET /dashboard/channelCode/workspaceStatistics
GET /dashboard/channelCode/workspaceStatisticsIndex
GET /dashboard/channelCode/export
~~~

筛选固定为 groupId、name、startDate、endDate、page、perPage。联系人状态变化为唯一事实源；查询未接通返回 available=false，不返回全零对象。响应同时返回 asOf、timezone、definition。

- [ ] **Step 4: 实现安全 CSV**

服务端重新应用筛选和数据权限，写 UTF-8 BOM，字段与页面顺序一致，并追加状态、截至时间和口径。同步导出上限 50,000 行，超限返回 422 而不是截断。

- [ ] **Step 5: 回归并提交**

~~~bash
go test ./internal/dashboard ./internal/store ./internal/server -run 'TestChannelCodeWorkspace|TestServer.*ChannelCode' -count=1
git add internal/dashboard/channel_code_workspace* internal/store/channel_code_workspace.go internal/server/server.go internal/server/server_test.go cmd/mochat-go/main.go
git diff --cached --check
git commit -m "feat: add channel code statistics and export"
~~~

Expected: PASS。

---

## Task 5：建立群活码分组、事件账本和可靠统计

**Files:**

- Create: internal/dashboard/group_code_workspace.go
- Create: internal/dashboard/group_code_workspace_test.go
- Create: internal/store/group_code_workspace.go
- Create: internal/store/group_code_workspace_test.go
- Modify: internal/dashboard/work_room_auto_pull.go
- Modify: internal/dashboard/room_infinite_pull_dashboard.go
- Modify: internal/store/mysql.go
- Modify: internal/store/room_infinite_pull.go

- [ ] **Step 1: 写分组、幂等和统计失败测试**

覆盖企业隔离、分组名唯一、两种模式移动分组、原空实现被替换、来源事件幂等、自然日、无法归因不计数、无账本数据返回 null。

~~~go
func TestGroupCodeJoinCountsOnlyConfirmedAttributedEvents(t *testing.T) {
    // contact_added 不等于入群；只有带 code_id 的 join_room 可计数。
}
~~~

- [ ] **Step 2: 确认失败**

~~~bash
go test ./internal/dashboard ./internal/store -run 'TestGroupCodeWorkspace|TestLiveCodeEvent' -count=1
~~~

Expected: FAIL，缺少分组和事件 store。

- [ ] **Step 3: 实现群活码分组接口**

新增：

~~~text
GET    /dashboard/groupCodeGroup/index
POST   /dashboard/groupCodeGroup/store
PUT    /dashboard/groupCodeGroup/update
POST   /dashboard/groupCodeGroup/move
DELETE /dashboard/groupCodeGroup/destroy
~~~

move 接收 mode 和最多 100 个 ID，在事务中验证归属并更新对应表；同时修复 WorkRoomAutoPullHandler.Move 只鉴权不写库的问题。

- [ ] **Step 4: 实现事件账本**

~~~go
type LiveCodeEvent struct {
    CorpID int
    Mode string
    CodeID, ContactID, RoomID int
    EventType string
    SourceEventID string
    OccurredAt time.Time
}
~~~

事件类型固定为 scan、contact_added、join_room、leave_room。唯一键保证回调幂等；无限模式只以 join_room 统计入群，自建模式只以 scan 统计扫码。无法唯一归因时不计数并写审计原因。

- [ ] **Step 5: 实现统一展示查询**

新增 GET /dashboard/groupCode/index?mode=infinite|selfBuilt：

- infinite 查询 workRoomAutoPull，补齐创建人、日期、分组、状态、确认入群总数和今日数；
- selfBuilt 查询 roomInfinitePull，补齐 codeType、创建人、扫码次数和关联二维码数；
- 默认排除 simulation；
- 未采集统计返回 null。

- [ ] **Step 6: 回归并提交**

~~~bash
go test ./internal/dashboard ./internal/store -run 'TestGroupCodeWorkspace|TestLiveCodeEvent|TestWorkRoomAutoPull|TestRoomInfinitePull' -count=1
git add internal/dashboard/group_code_workspace* internal/dashboard/work_room_auto_pull.go internal/dashboard/room_infinite_pull_dashboard.go internal/store/group_code_workspace* internal/store/mysql.go internal/store/room_infinite_pull.go
git diff --cached --check
git commit -m "feat: add group code workspace data model"
~~~

Expected: PASS。

---

## Task 6：接入群活码创建、上传、作废、下载和事件来源

**Files:**

- Modify: internal/dashboard/group_code_workspace.go
- Modify: internal/dashboard/group_code_workspace_test.go
- Modify: internal/dashboard/common_upload.go
- Modify: internal/dashboard/common_upload_test.go
- Modify: internal/dashboard/work_room_auto_pull.go
- Modify: internal/dashboard/work_room_auto_pull_test.go
- Modify: internal/dashboard/room_infinite_pull_dashboard.go
- Modify: internal/dashboard/room_infinite_pull_dashboard_test.go
- Modify: internal/dashboard/wework_callback_worker.go
- Modify: internal/dashboard/wework_callback_worker_test.go
- Modify: internal/dashboard/operation_h5.go
- Create: internal/dashboard/operation_h5_test.go
- Modify: internal/store/operation_h5.go
- Modify: internal/store/group_code_workspace.go
- Modify: internal/server/server.go
- Modify: internal/server/server_test.go
- Modify: cmd/mochat-go/main.go

- [ ] **Step 1: 写失败测试**

覆盖无限模式最多 5 群、群顺序、无真实自动建群能力时拒绝开启、自建图片 MIME/后缀/2MB 三重校验、最多 5 码、两类 codeType、批量作废部分失败、下载真实文件、H5 扫描事件幂等。

- [ ] **Step 2: 确认失败**

~~~bash
go test ./internal/dashboard ./internal/store -run 'TestGroupCodeMutation|TestGroupCodeUpload|TestRoomInfinitePullScanEvent' -count=1
~~~

Expected: FAIL。

- [ ] **Step 3: 实现写入适配**

新增：

~~~text
GET  /dashboard/groupCode/show
POST /dashboard/groupCode/store
PUT  /dashboard/groupCode/update
POST /dashboard/groupCode/batchInvalidate
GET  /dashboard/groupCode/export
GET  /dashboard/groupCode/download
~~~

mode=infinite 调用抽取后的 workRoomAutoPull 领域方法；mode=selfBuilt 调用 roomInfinitePull 领域方法。Handler 不互相调用。自建类型锁定为 enterpriseWeCom（一个企业微信官方群活码，永久）与 uploadedGroup（最多五个普通群二维码，MoChat 落地页轮换）。

- [ ] **Step 4: 收紧上传和下载**

复用 /dashboard/common/upload 的存储、配额和账本；purpose=groupCode 强制 image/jpeg 或 image/png、匹配后缀、单文件不超过 2MB，并读取文件头验证格式。下载只允许当前企业拥有的存储键；ZIP 最多 100 个二维码。

- [ ] **Step 5: 接入事件来源**

workRoomAutoPull 客户联系回调写 contact_added；群成员加入回调写已确认 join_room；operation/roomInfinitePull/qrCode 在返回二维码前写去重 scan。写入失败必须进入现有重试/审计机制，不能伪造统计成功。

- [ ] **Step 6: 回归并提交**

~~~bash
go test ./internal/dashboard ./internal/store ./internal/server -run 'TestGroupCode|TestWorkRoomAutoPull|TestRoomInfinitePull|TestCommonUpload|TestServer.*GroupCode' -count=1
git add internal/dashboard/group_code_workspace* internal/dashboard/common_upload* internal/dashboard/work_room_auto_pull* internal/dashboard/room_infinite_pull_dashboard* internal/dashboard/wework_callback_worker* internal/dashboard/operation_h5* internal/store/operation_h5.go internal/store/group_code_workspace.go internal/server/server.go internal/server/server_test.go cmd/mochat-go/main.go
git diff --cached --check
git commit -m "feat: complete group code operation flows"
~~~

Expected: PASS，路由清单包含所有新 endpoint。

---

## Task 7：建立前端 API 适配器与共享工作区

**Files:**

- Create: web/apps/dashboard/src/features/phase34/live-code/live-code-types.ts
- Create: web/apps/dashboard/src/features/phase34/live-code/live-code-api.ts
- Create: web/apps/dashboard/src/features/phase34/live-code/live-code-api.test.ts
- Create: web/apps/dashboard/src/features/phase34/live-code/live-code-workspace.tsx
- Create: web/apps/dashboard/src/features/phase34/live-code/live-code-workspace.test.tsx
- Create: web/apps/dashboard/src/styles/live-code-workspace.css
- Create: web/apps/dashboard/src/styles/live-code-workspace-layout.test.ts
- Modify: web/apps/dashboard/src/main.tsx

- [ ] **Step 1: 写失败测试**

测试 envelope、null 统计、分页、部分失败、CSV blob、multipart upload、URL 枚举校验；样式测试验证无大头图、内容卡间距 16px、224px 分组栏、1280px 横向滚动和宽屏自适应。

~~~ts
expect(normalizeVerifiedCount({ value: 0, valid: true })).toBe(0)
expect(normalizeVerifiedCount({ value: 0, valid: false })).toBeNull()
~~~

- [ ] **Step 2: 确认失败**

~~~bash
corepack pnpm --filter @mochat/dashboard test -- live-code-api.test.ts live-code-workspace.test.tsx live-code-workspace-layout.test.ts
~~~

Expected: FAIL，模块不存在。

- [ ] **Step 3: 实现专用 API**

live-code-api.ts 组合现有 JSON 客户端，为 blob/multipart 提供同源认证请求。所有响应先做 Zod 或显式守卫校验；字段缺失进入错误态，不生成默认业务记录。

~~~ts
export interface LiveCodeApi {
  listChannelCodes(query: ChannelCodeQuery): Promise<Page<ChannelCodeRow>>
  listGroupCodes(query: GroupCodeQuery): Promise<Page<GroupCodeRow>>
  uploadGroupCode(file: File): Promise<UploadedAsset>
  exportChannelCodes(query: ChannelCodeQuery): Promise<Blob>
  batchInvalidateChannelCodes(ids: number[]): Promise<BatchResult>
}
~~~

- [ ] **Step 4: 实现共享组件和隔离样式**

共享组件仅包含 CodeGroupDirectory、LiveCodeToolbar、LiveCodeTableFrame、LiveCodePagination、VerifiedMetric 和稳定尺寸的加载/空/错误态。CSS 全部使用 .live-code-workspace 命名空间，不修改全局 .page-*。

- [ ] **Step 5: 验证并提交**

~~~bash
corepack pnpm --filter @mochat/dashboard test -- live-code
corepack pnpm --filter @mochat/dashboard typecheck
git add web/apps/dashboard/src/features/phase34/live-code web/apps/dashboard/src/styles/live-code-workspace.css web/apps/dashboard/src/styles/live-code-workspace-layout.test.ts web/apps/dashboard/src/main.tsx
git diff --cached --check
git commit -m "feat: add live code frontend foundation"
~~~

Expected: PASS。

---

## Task 8：实现渠道活码专用页面、向导和统计

**Files:**

- Create: web/apps/dashboard/src/features/phase34/live-code/channel-code-page.tsx
- Create: web/apps/dashboard/src/features/phase34/live-code/channel-code-page.test.tsx
- Create: web/apps/dashboard/src/features/phase34/live-code/channel-code-form.tsx
- Create: web/apps/dashboard/src/features/phase34/live-code/channel-code-form.test.tsx
- Create: web/apps/dashboard/src/features/phase34/live-code/channel-code-statistics.tsx
- Create: web/apps/dashboard/src/features/phase34/live-code/channel-code-statistics.test.tsx
- Modify: web/apps/dashboard/src/features/phase34/acquisition-pages.tsx
- Modify: web/apps/dashboard/src/benchmark/page-registry.tsx

- [ ] **Step 1: 写页面、向导和统计失败测试**

覆盖 URL 恢复、筛选、无刷新按钮、null 指标、多值折叠、权限、批量部分失败、导出筛选、布局不跳变；覆盖三步校验、员工/部门、标签、备注变量、欢迎语附件、有效期、二维码预览和统计筛选。

- [ ] **Step 2: 确认失败**

~~~bash
corepack pnpm --filter @mochat/dashboard test -- channel-code-page.test.tsx channel-code-form.test.tsx channel-code-statistics.test.tsx
~~~

Expected: FAIL。

- [ ] **Step 3: 实现 URL 和页面**

查询参数固定为 groupId、name、creator、employeeId、state、page。筛选变化重置 page=1 并 replace；分页使用 push；默认值不写冗余参数。列表字段和批量动作严格按设计文档。

- [ ] **Step 4: 实现三步向导和统计**

使用 React Hook Form + Zod；员工/部门/标签只用真实选择器；有限期限必须在未来；创建成功失效查询并聚焦新记录。海报只使用服务端真实二维码生成预览。统计弹窗返回口径与 asOf，并通过 blob 导出。

- [ ] **Step 5: 替换路由组件**

acquisition-pages.tsx 删除渠道/群活码通用实现但保留 LiveCodeShortChainPage；page-registry.tsx 从新文件导入 ChannelCodePage，路由路径不变。

- [ ] **Step 6: 验证并提交**

~~~bash
corepack pnpm --filter @mochat/dashboard test -- channel-code acquisition-pages.test.tsx
corepack pnpm --filter @mochat/dashboard typecheck
git add web/apps/dashboard/src/features/phase34/live-code/channel-code-* web/apps/dashboard/src/features/phase34/acquisition-pages.tsx web/apps/dashboard/src/benchmark/page-registry.tsx
git diff --cached --check
git commit -m "feat: redesign channel code page"
~~~

Expected: PASS。

---

## Task 9：实现群活码双模式页面与业务表单

**Files:**

- Create: web/apps/dashboard/src/features/phase34/live-code/group-code-page.tsx
- Create: web/apps/dashboard/src/features/phase34/live-code/group-code-page.test.tsx
- Create: web/apps/dashboard/src/features/phase34/live-code/group-code-infinite-panel.tsx
- Create: web/apps/dashboard/src/features/phase34/live-code/group-code-infinite-panel.test.tsx
- Create: web/apps/dashboard/src/features/phase34/live-code/group-code-self-built-panel.tsx
- Create: web/apps/dashboard/src/features/phase34/live-code/group-code-self-built-panel.test.tsx
- Create: web/apps/dashboard/src/features/phase34/live-code/group-code-form.tsx
- Create: web/apps/dashboard/src/features/phase34/live-code/group-code-form.test.tsx
- Modify: web/apps/dashboard/src/benchmark/page-registry.tsx
- Modify: web/apps/dashboard/src/features/phase34/acquisition-pages.test.tsx

- [ ] **Step 1: 写双模式失败测试**

覆盖 mode=infinite|selfBuilt、非法模式回退、两套筛选/页码隔离、前进后退、页签不串数据、自建模式无空分组栏、无刷新按钮。

- [ ] **Step 2: 写两个业务面板失败测试**

无限模式覆盖分组、名称、创建人、日期、统计未知态、最多五群、顺序、自动建群能力门控、企微错误、导出、移动和作废。自建模式覆盖类型、日期、2MB/格式、最多五码、顺序/容量、上传重试、扫码未知态、详情、下载和软作废。

- [ ] **Step 3: 确认失败**

~~~bash
corepack pnpm --filter @mochat/dashboard test -- group-code
~~~

Expected: FAIL。

- [ ] **Step 4: 实现容器、面板和表单**

URL 使用 mode、groupId、name、creator、startDate、endDate、page。两模式分别维护缓存键和合法筛选。无限模式只显示确认归因入群数；自建模式显示扫码数与关联码数；null 交给 VerifiedMetric。上传提交 storage key，排序提交显式 position。

- [ ] **Step 5: 接入路由并验证**

~~~bash
corepack pnpm --filter @mochat/dashboard test -- group-code acquisition-pages.test.tsx
corepack pnpm --filter @mochat/dashboard typecheck
corepack pnpm --filter @mochat/dashboard lint
git add web/apps/dashboard/src/features/phase34/live-code/group-code-* web/apps/dashboard/src/benchmark/page-registry.tsx web/apps/dashboard/src/features/phase34/acquisition-pages.test.tsx
git diff --cached --check
git commit -m "feat: redesign group code page"
~~~

Expected: PASS，/acquisition/group-code 渲染新双模式页面。

---

## Task 10：完成端到端、Docker 和交付证据

**Files:**

- Create: web/e2e/tests/live-code-workspaces.spec.ts
- Create: web/e2e/fixtures/live-code-workspaces.ts
- Create: scripts/check_live_code_workspace.mjs
- Create: docs/phases/phase-3-dashboard/phase-3.4/reports/channel-and-group-live-code-implementation-report.md
- Modify: package.json

- [ ] **Step 1: 写完成度脚本并确认失败**

脚本检查新页面、两种模式、0150 migration、权限动作、新路由、样式命名空间、无刷新按钮、无前端 example.invalid 过滤。

~~~bash
node scripts/check_live_code_workspace.mjs
~~~

Expected: 首次 FAIL，直到全部契约存在。

- [ ] **Step 2: 写 Playwright 场景**

使用真实后端种子或受控异常 fixture，不用 fixture 伪造成功数据。覆盖三种视口、渠道筛选/URL/创建/统计/导出/作废、群模式隔离、无限创建企微失败、自建上传/落地页/下载、空态/null/无权限/部分失败、加载前后间距不跳变。

- [ ] **Step 3: 运行完整验证**

~~~bash
go test ./internal/dashboard ./internal/store ./internal/server ./internal/migration -count=1
corepack pnpm --filter @mochat/dashboard test
corepack pnpm --filter @mochat/dashboard typecheck
corepack pnpm --filter @mochat/dashboard lint
corepack pnpm --filter @mochat/dashboard build
node scripts/check_dashboard_page_rbac_catalog.mjs
node scripts/check_live_code_workspace.mjs
corepack pnpm --filter @mochat/e2e exec playwright test tests/live-code-workspaces.spec.ts --workers=1
~~~

Expected: 全部 PASS，浏览器控制台无 error，截图无溢出和间距跳变。

- [ ] **Step 4: Docker 重建和运行态验收**

~~~bash
docker compose -f deploy/standalone/docker-compose.yml build mochat-go
docker compose -f deploy/standalone/docker-compose.yml up -d mochat-go
docker compose -f deploy/standalone/docker-compose.yml ps
~~~

Expected: mochat-go 为 running/healthy，新静态资源和新路由由容器提供。随后在部署地址重复两页的核心创建、筛选、模式切换和错误态场景。

- [ ] **Step 5: 写中文实施报告**

记录实施提交、migration、接口、真实数据来源、已知限制、测试输出、三种视口截图路径、容器状态和回滚步骤；未执行项不得写成完成。

- [ ] **Step 6: 完整差异复核**

~~~bash
git status --short
git diff --check
git log --oneline --decorate -12
~~~

Expected: 只剩预期文件，无临时 fixture、调试日志和无关改动。

- [ ] **Step 7: 请求代码审查并修复**

使用 requesting-code-review 审查真实性、企业隔离、统计口径、破坏性操作、前端状态和回滚；逐项复现、修复并重跑受影响测试。

- [ ] **Step 8: 提交验收资产**

~~~bash
git add web/e2e/tests/live-code-workspaces.spec.ts web/e2e/fixtures/live-code-workspaces.ts scripts/check_live_code_workspace.mjs docs/phases/phase-3-dashboard/phase-3.4/reports/channel-and-group-live-code-implementation-report.md package.json
git diff --cached --check
git commit -m "test: verify live code workspaces end to end"
~~~

- [ ] **Step 9: 合并前最终验证**

使用 verification-before-completion 重跑 Step 3 与 Docker 运行态验收；只有最新输出全部通过，才使用 finishing-a-development-branch 提供合入 main 的选择。文档完成不代表代码功能已经完成。
