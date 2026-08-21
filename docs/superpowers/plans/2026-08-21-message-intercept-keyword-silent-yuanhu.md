# 消息拦截、关键词库与沉默客户菜单优化实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将消息拦截、关键词库和沉默客户三个菜单改造成使用真实会话归档数据、固定分页、URL 状态、权限隔离和可持续自动扫描的统一风险预警工作台。

**Architecture:** 前端复用现有 `risk-warning` 工作台组件，并为三个领域建立类型化 API 与独立页面组件。后端保留现有 CRUD，在存储层补充筛选、分页、详情、审计、动作状态机和自动扫描游标；消息拦截与沉默客户各自通过可配置 cron 消费真实归档消息，状态持久化后供页面读取。

**Tech Stack:** React 19、TypeScript、TanStack Query、Vitest、Go `net/http`、MariaDB、Docker Compose、Codex in-app Browser。

## Global Constraints

- 用户审阅通过前不修改业务代码。
- 保留三个菜单名称、路由、侧栏和顶部 banner。
- 页面不得展示“能力未接入”“数据提供方未接入”类占位文案。
- 所有业务数据来自真实 API/数据库或显式本地开发种子，前端不写业务假数据。
- 查询显式提交，分页固定每页 20 条，已提交状态写入 URL。
- 消息拦截表示归档后自动检测和复核，不宣称发送前阻断。
- 先写失败测试，再做最小实现；每个任务独立通过后再进入下一任务。
- Docker 只重建必要的 `app` 服务，不执行 `down -v`，不删除 MySQL、Redis 或命名卷。
- 用户现有工作树很脏；只修改本计划列出的目标文件和必要依赖，不覆盖无关改动。

---

### Task 1: 固化页面、API、样式和权限基线

**Files:**

- Modify: `web/apps/dashboard/src/features/phase33/message-intercept-pages.test.tsx`
- Modify: `web/apps/dashboard/src/features/phase33/phase33-closure-pages.test.tsx`
- Modify: `web/apps/dashboard/src/styles/risk-warning-workspace-layout.test.ts`
- Modify: `internal/dashboard/dashboard_access_guard_test.go`
- Modify: `internal/dashboard/message_intercept_handler_test.go`
- Modify: `internal/dashboard/phase33_closure_handler_test.go`

**Interfaces:**

- Captures current routes: `/ai-insight/v2/message-intercept`、`/ai-insight/v2/keyword-library`、`/ai-insight/v2/silent-customer`。
- Defines fixed `perPage=20` and URL-backed page behavior for later tasks.
- Defines action resources for configuration, audit, assignment and follow-up.

- [ ] **Step 1: 写失败的前端契约测试**

在三个页面测试中加入断言：不渲染 `.phase33-risk-warning-header.dashboard-data-card`；输入筛选不请求，点击查询后请求；分页使用 20；刷新只重取已提交条件；页面不出现英文业务状态、员工 ID 输入和原始对象字段名。

```tsx
fireEvent.change(screen.getByLabelText('消息或命中词'), { target: { value: '转账' } });
expect(api.read).toHaveBeenCalledTimes(1);
fireEvent.click(screen.getByRole('button', { name: '查询' }));
await waitFor(() => expect(api.read).toHaveBeenLastCalledWith(
  '/message-intercept/records',
  expect.objectContaining({ keyword: '转账', page: 1, perPage: 20 }),
));
expect(screen.queryByLabelText('分派员工 ID')).toBeNull();
```

- [ ] **Step 2: 写失败的样式和权限测试**

样式测试断言三个页面使用 `.risk-warning-workspace`、无固定窄 `max-width`、双栏词库左栏为稳定宽度、按钮 hover 不含 `transform`。权限测试断言新增 scanner status、详情、审计和分派资源继承对应页面权限且记录资源 `scopeRequired=true`。

- [ ] **Step 3: 运行并确认 RED**

Run:

```powershell
corepack pnpm --filter @mochat/dashboard exec vitest run src/features/phase33/message-intercept-pages.test.tsx src/features/phase33/phase33-closure-pages.test.tsx src/styles/risk-warning-workspace-layout.test.ts
go test ./internal/dashboard -run 'MessageIntercept|Silent|DashboardAccessGuard' -count=1
```

Expected: FAIL，旧页面仍有顶部卡、即时查询、固定第 1 页、员工 ID 输入和缺失资源。

- [ ] **Step 4: 提交基线测试**

```powershell
git add web/apps/dashboard/src/features/phase33/message-intercept-pages.test.tsx web/apps/dashboard/src/features/phase33/phase33-closure-pages.test.tsx web/apps/dashboard/src/styles/risk-warning-workspace-layout.test.ts internal/dashboard/dashboard_access_guard_test.go internal/dashboard/message_intercept_handler_test.go internal/dashboard/phase33_closure_handler_test.go
git commit -m "test: define intercept keyword and silent workspace contracts"
```

---

### Task 2: 建立三个页面的类型化前端 API

**Files:**

- Create: `web/apps/dashboard/src/features/phase33/message-intercept-api.ts`
- Create: `web/apps/dashboard/src/features/phase33/message-intercept-api.test.ts`
- Create: `web/apps/dashboard/src/features/phase33/silent-customer-api.ts`
- Create: `web/apps/dashboard/src/features/phase33/silent-customer-api.test.ts`
- Modify: `web/apps/dashboard/src/features/risk-warning/risk-warning-api.ts`

**Interfaces:**

- Produces: `MessageInterceptApi` with `rules`, `records`, `recordDetail`, `libraries`, `audit`, `scannerStatus`, `write`.
- Produces: `KeywordLibraryApi` with `libraries`, `entries`, `addEntries`, `publish`, `write`.
- Produces: `SilentCustomerApi` with `rules`, `records`, `recordDetail`, `staffOptions`, `act`, `scannerStatus`, `write`.
- Reuses: `Page<T>` and scanner state types from `risk-warning-api.ts`.

- [ ] **Step 1: 写解析与请求失败测试**

测试精确断言分页、中文展示所需字段、空值语义和请求参数。例如：

```ts
expect(await api.records({ keyword: '转账', page: 2, perPage: 20 })).toEqual({
  items: [expect.objectContaining({ id: 9, decision: 'blocked', auditStatus: 'pending' })],
  total: 41,
  page: 2,
  perPage: 20,
});
expect(read).toHaveBeenCalledWith('/message-intercept/records', { keyword: '转账', page: 2, perPage: 20 });
```

员工选项测试必须解析员工姓名、部门和 ID，不能生成“员工 1001”一类前端假名称。

- [ ] **Step 2: 运行并确认 RED**

Run:

```powershell
corepack pnpm --filter @mochat/dashboard exec vitest run src/features/phase33/message-intercept-api.test.ts src/features/phase33/silent-customer-api.test.ts
```

Expected: FAIL，新模块尚不存在。

- [ ] **Step 3: 实现类型、解析器和 API**

只在 API 层兼容响应 envelope 和空值，不在页面组件中使用 `Record<string, unknown>`。关键类型：

```ts
export type MessageInterceptRecord = {
  id: number; ruleId: number; ruleName: string; libraryName: string; libraryVersion: number;
  conversationType: string; conversationId: string; messageId: string;
  senderName: string; messageContent: string; matchedKeywords: string[];
  decision: string; explanation: string; auditStatus: string; occurredAt: string;
};

export type SilentCustomerRecord = {
  id: number; ruleId: number; ruleName: string; customerId: string; customerName: string;
  employeeId: number; employeeName: string; lastInteractionAt: string; silentDays: number;
  status: string; assignedEmployeeId: number; assignedEmployeeName: string;
  followUpNote: string; updatedAt: string;
};
```

- [ ] **Step 4: 运行 GREEN**

Run:

```powershell
corepack pnpm --filter @mochat/dashboard exec vitest run src/features/phase33/message-intercept-api.test.ts src/features/phase33/silent-customer-api.test.ts
corepack pnpm --filter @mochat/dashboard typecheck
```

Expected: PASS。

- [ ] **Step 5: 提交 API 模块**

```powershell
git add web/apps/dashboard/src/features/phase33/message-intercept-api.ts web/apps/dashboard/src/features/phase33/message-intercept-api.test.ts web/apps/dashboard/src/features/phase33/silent-customer-api.ts web/apps/dashboard/src/features/phase33/silent-customer-api.test.ts web/apps/dashboard/src/features/risk-warning/risk-warning-api.ts
git commit -m "feat: add typed intercept keyword and silent APIs"
```

---

### Task 3: 补齐后端筛选、详情、分页和规则引用保护

**Files:**

- Modify: `internal/dashboard/message_intercept.go`
- Modify: `internal/dashboard/message_intercept_handler.go`
- Modify: `internal/dashboard/message_intercept_handler_test.go`
- Modify: `internal/dashboard/phase33_closure.go`
- Modify: `internal/dashboard/phase33_closure_handler.go`
- Modify: `internal/dashboard/phase33_closure_handler_test.go`
- Modify: `internal/store/message_intercept.go`
- Create: `internal/store/message_intercept_filter_test.go`
- Modify: `internal/store/phase33_closure.go`
- Create: `internal/store/silent_customer_filter_test.go`
- Modify: `internal/server/server.go`
- Modify: `internal/server/server_test.go`

**Interfaces:**

- Extends `MessageInterceptRuleFilter`: `Decision`.
- Extends `MessageInterceptRecordFilter`: `ConversationType, OccurredFrom, OccurredTo`.
- Extends `SilentRecordFilter`: `EmployeeID, AssignedEmployeeID, MinSilentDays, MaxSilentDays, SnapshotFrom, SnapshotTo`.
- Produces: `GET /dashboard/message-intercept/records/detail?id=`.
- Produces: `GET /dashboard/silent-customer/records/detail?id=` with audit timeline and assignee name.
- Produces: `POST /dashboard/keyword-library/entries/batch` with transactional per-item results.
- Changes delete semantics: message intercept and silent customer rules use `deleted_at` soft delete; referenced libraries cannot be deleted.

- [ ] **Step 1: 写失败的 handler 和 SQL 测试**

Handler 测试断言所有查询参数传入 provider，并拒绝非法日期、倒序区间和越界分页。存储测试断言 SQL 包含 tenant/corp、员工权限范围、日期闭区间和真实 total 查询。

```go
req := authenticatedInterceptRequest(http.MethodGet,
    "/dashboard/message-intercept/records?conversationType=group&auditStatus=pending&startAt=2026-08-01&endAt=2026-08-21&page=2&perPage=20", nil)
handler.Records(rec, req)
if got := provider.recordFilter; got.ConversationType != "group" || got.Page != 2 || got.PerPage != 20 {
    t.Fatalf("filter=%+v", got)
}
```

- [ ] **Step 2: 运行并确认 RED**

Run:

```powershell
go test ./internal/dashboard ./internal/store ./internal/server -run 'MessageIntercept|Keyword|Silent' -count=1
```

Expected: FAIL，新过滤字段、详情和引用保护尚不存在。

- [ ] **Step 3: 实现过滤和详情**

使用参数化 SQL。结束日期转换为下一天零点的排他上界；详情按 tenant/corp 和允许员工范围读取。审计时间线按 `created_at ASC,id ASC` 返回。

- [ ] **Step 4: 实现引用保护**

删除词库前查询未删除规则和历史版本；被引用时返回冲突和引用规则名称。消息拦截规则与沉默规则统一写 `deleted_at`，所有规则读取和扫描 SQL 同步增加 `deleted_at IS NULL`。

批量词条接口在一个事务内逐项归一化、去重和插入，返回 `created`、`duplicates` 和对应关键词；任一非重复数据库错误回滚整批。

- [ ] **Step 5: 运行 GREEN**

Run:

```powershell
go test ./internal/dashboard ./internal/store ./internal/server -run 'MessageIntercept|Keyword|Silent' -count=1
```

Expected: PASS。

- [ ] **Step 6: 提交后端读取闭环**

```powershell
git add internal/dashboard/message_intercept.go internal/dashboard/message_intercept_handler.go internal/dashboard/message_intercept_handler_test.go internal/dashboard/phase33_closure.go internal/dashboard/phase33_closure_handler.go internal/dashboard/phase33_closure_handler_test.go internal/store/message_intercept.go internal/store/message_intercept_filter_test.go internal/store/phase33_closure.go internal/store/silent_customer_filter_test.go internal/server/server.go internal/server/server_test.go
git commit -m "feat: complete intercept and silent query contracts"
```

---

### Task 4: 重建关键词库双栏工作区

**Files:**

- Create: `web/apps/dashboard/src/features/phase33/keyword-library-page.tsx`
- Create: `web/apps/dashboard/src/features/phase33/keyword-library-page.test.tsx`
- Modify: `web/apps/dashboard/src/main.tsx`
- Modify: `web/apps/dashboard/src/styles/risk-warning-workspace.css`
- Modify: `web/apps/dashboard/src/styles/risk-warning-workspace-layout.test.ts`

**Interfaces:**

- Consumes: `KeywordLibraryApi` from Task 2.
- Produces URL state: `libraryKeyword,libraryId,entryKeyword,entryStatus,entryPage`.
- Reuses: `RiskWarningShell`, `RiskWarningPageHeader`, `RiskWarningQueryBar`, `RiskWarningDrawer`, `DashboardPagination`, `ConfirmAction`.

- [ ] **Step 1: 写失败页面测试**

覆盖左 260px 词库列、右侧词条列、选择不抖动、显式查询、固定分页、批量拆词去重、草稿/发布版本、启停/发布/删除确认、错误保留和 URL 恢复。

```tsx
fireEvent.click(await screen.findByRole('button', { name: '违禁词库' }));
expect(screen.getByText('草稿 v1')).toBeTruthy();
expect(screen.getByText('已发布 v1')).toBeTruthy();
expect(screen.getByRole('button', { name: '发布新版本' })).toBeTruthy();
```

- [ ] **Step 2: 运行并确认 RED**

Run:

```powershell
corepack pnpm --filter @mochat/dashboard exec vitest run src/features/phase33/keyword-library-page.test.tsx src/styles/risk-warning-workspace-layout.test.ts
```

Expected: FAIL，新页面不存在。

- [ ] **Step 3: 实现双栏、抽屉和查询状态**

新增词条拆分函数：

```ts
export function splitKeywordDraft(value: string): string[] {
  return [...new Set(value.split(/[，,、；;\n\r]+/).map((item) => item.trim()).filter(Boolean))];
}
```

左栏选择只修改 `libraryId` 和 `entryPage=1`；右栏加载时保留外层宽度。发布、启停和删除在确认前不得发送请求。

- [ ] **Step 4: 运行 GREEN**

Run:

```powershell
corepack pnpm --filter @mochat/dashboard exec vitest run src/features/phase33/keyword-library-page.test.tsx src/features/phase33/message-intercept-api.test.ts src/styles/risk-warning-workspace-layout.test.ts
corepack pnpm --filter @mochat/dashboard typecheck
```

Expected: PASS。

- [ ] **Step 5: 提交关键词库页面**

```powershell
git add web/apps/dashboard/src/features/phase33/keyword-library-page.tsx web/apps/dashboard/src/features/phase33/keyword-library-page.test.tsx web/apps/dashboard/src/main.tsx web/apps/dashboard/src/styles/risk-warning-workspace.css web/apps/dashboard/src/styles/risk-warning-workspace-layout.test.ts
git commit -m "feat: redesign keyword library workspace"
```

---

### Task 5: 重建消息拦截规则与命中复核页面

**Files:**

- Create: `web/apps/dashboard/src/features/phase33/message-intercept-page.tsx`
- Create: `web/apps/dashboard/src/features/phase33/message-intercept-rules.tsx`
- Create: `web/apps/dashboard/src/features/phase33/message-intercept-records.tsx`
- Modify: `web/apps/dashboard/src/features/phase33/message-intercept-pages.test.tsx`
- Modify: `web/apps/dashboard/src/main.tsx`
- Modify: `web/apps/dashboard/src/styles/risk-warning-workspace.css`

**Interfaces:**

- Consumes: `MessageInterceptApi` from Task 2 and detail endpoints from Task 3.
- Produces URL state for rules and records exactly as the design spec.
- Uses normalized values: `single/group`, `blocked/review_required/allowed`, `pending/confirmed/ignored`.

- [ ] **Step 1: 写失败交互测试**

测试默认规则标签、规则筛选、新建抽屉、已发布词库、会话范围、确认启停/删除；命中记录测试显式查询、批量复核、中文状态、整行详情、审计时间线、遮罩/Escape 和刷新后持久化。

- [ ] **Step 2: 运行并确认 RED**

Run:

```powershell
corepack pnpm --filter @mochat/dashboard exec vitest run src/features/phase33/message-intercept-pages.test.tsx
```

Expected: FAIL，旧页面默认记录标签并枚举原始字段。

- [ ] **Step 3: 实现规则页**

规则保存只提交已发布版本：

```ts
const input = {
  id: editor.id,
  name: draft.name.trim(),
  libraryId: draft.libraryId,
  libraryVersion: selectedLibrary.publishedVersion,
  conversationScopes: draft.conversationScopes,
  decision: draft.decision,
  status: draft.status,
};
```

- [ ] **Step 4: 实现命中记录和详情**

详情按业务标签显示，禁止 `Object.entries(record)`。批量请求成功后清空选择并 invalidate；部分失败显示成功/失败数量，失败记录仍保留可选。

- [ ] **Step 5: 运行 GREEN**

Run:

```powershell
corepack pnpm --filter @mochat/dashboard exec vitest run src/features/phase33/message-intercept-pages.test.tsx src/features/phase33/message-intercept-api.test.ts src/features/risk-warning/risk-warning-shell.test.tsx
corepack pnpm --filter @mochat/dashboard typecheck
```

Expected: PASS。

- [ ] **Step 6: 提交消息拦截页面**

```powershell
git add web/apps/dashboard/src/features/phase33/message-intercept-page.tsx web/apps/dashboard/src/features/phase33/message-intercept-rules.tsx web/apps/dashboard/src/features/phase33/message-intercept-records.tsx web/apps/dashboard/src/features/phase33/message-intercept-pages.test.tsx web/apps/dashboard/src/main.tsx web/apps/dashboard/src/styles/risk-warning-workspace.css
git commit -m "feat: redesign message intercept workspace"
```

---

### Task 6: 实现消息拦截归档扫描、游标、状态和幂等

**Files:**

- Create: `internal/dashboard/message_intercept_cron.go`
- Create: `internal/dashboard/message_intercept_cron_test.go`
- Modify: `internal/store/message_intercept.go`
- Create: `internal/store/message_intercept_cron_test.go`
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `cmd/mochat-go/main.go`
- Modify: `internal/server/server.go`
- Modify: `internal/server/server_test.go`
- Modify: `deploy/standalone/.env.example`
- Modify: `deploy/standalone/docker-compose.yml`

**Interfaces:**

- Produces: `MessageInterceptCron.RunOnce(context.Context) error`.
- Store methods: `ActiveMessageInterceptRules`, `MessageInterceptCursor`, `PendingMessageInterceptMessages`, `InsertMessageInterceptRecord`, `UpdateMessageInterceptCursor`, `RecordMessageInterceptScanStarted`, `RecordMessageInterceptScanFinished`.
- Config: `MOCHAT_GO_ENABLE_MESSAGE_INTERCEPT_CRON`, `MOCHAT_GO_MESSAGE_INTERCEPT_CRON_INTERVAL_SECONDS`, `MOCHAT_GO_MESSAGE_INTERCEPT_CRON_RUN_ON_START`.
- Endpoint: `GET /dashboard/message-intercept/scanner-status`.

- [ ] **Step 1: 写失败 cron 测试**

覆盖：只扫描启用规则；使用固定词库版本；`single/group` 范围过滤；同一消息同一规则只插入一次；失败不越过游标；空消息推进游标；状态记录成功/失败。

```go
func TestMessageInterceptCronIsIdempotentPerRuleAndMessage(t *testing.T) {
    store := newInterceptCronStoreWithOneRuleAndMessage()
    cron := NewMessageInterceptCron(store, nil)
    if err := cron.RunOnce(context.Background()); err != nil { t.Fatal(err) }
    if err := cron.RunOnce(context.Background()); err != nil { t.Fatal(err) }
    if store.inserted != 1 { t.Fatalf("inserted=%d", store.inserted) }
}
```

- [ ] **Step 2: 写失败配置和状态测试**

配置测试断言默认关闭、间隔必须为正数、standalone 仅在显式环境值时启动。状态接口返回 `disabled/never_run/ready/failed`。

- [ ] **Step 3: 运行并确认 RED**

Run:

```powershell
go test ./internal/dashboard ./internal/store ./internal/config ./internal/server ./cmd/mochat-go -run 'MessageIntercept' -count=1
```

Expected: FAIL，cron、配置、游标和状态尚不存在。

- [ ] **Step 4: 实现最小扫描链**

复用敏感词 cron 已验证的 10 分表读取方式，但使用独立 cursor type，避免互相推进。记录幂等键包含 `tenant_id,corp_id,rule_id,message_table_index,message_row_id`；数据库唯一键作为最终防线。

- [ ] **Step 5: 运行 GREEN**

Run:

```powershell
go test ./internal/dashboard ./internal/store ./internal/config ./internal/server ./cmd/mochat-go -run 'MessageIntercept' -count=1
```

Expected: PASS。

- [ ] **Step 6: 提交自动扫描链**

```powershell
git add internal/dashboard/message_intercept_cron.go internal/dashboard/message_intercept_cron_test.go internal/store/message_intercept.go internal/store/message_intercept_cron_test.go internal/config/config.go internal/config/config_test.go cmd/mochat-go/main.go internal/server/server.go internal/server/server_test.go deploy/standalone/.env.example deploy/standalone/docker-compose.yml
git commit -m "feat: scan archived messages for intercept rules"
```

---

### Task 7: 重建沉默客户明细、规则和处置交互

**Files:**

- Create: `web/apps/dashboard/src/features/phase33/silent-customer-page.tsx`
- Create: `web/apps/dashboard/src/features/phase33/silent-customer-records.tsx`
- Create: `web/apps/dashboard/src/features/phase33/silent-customer-rules.tsx`
- Modify: `web/apps/dashboard/src/features/phase33/phase33-closure-pages.test.tsx`
- Modify: `web/apps/dashboard/src/main.tsx`
- Modify: `web/apps/dashboard/src/styles/risk-warning-workspace.css`

**Interfaces:**

- Consumes: `SilentCustomerApi` from Task 2 and detail endpoints from Task 3.
- Reuses the real employee directory picker already used by employee/resigned staff pages.
- Produces URL state exactly as the design spec.

- [ ] **Step 1: 写失败页面测试**

覆盖显式查询、员工姓名/部门选择、固定分页、中文状态和时间、整行详情、审计时间线、批量工具条、分派确认、动作状态机、部分失败、刷新后持久化、规则 CRUD 和抽屉关闭。

```tsx
expect(screen.queryByLabelText('分派员工 ID')).toBeNull();
fireEvent.click(screen.getByRole('button', { name: '选择负责人' }));
fireEvent.click(await screen.findByRole('button', { name: /张伟/ }));
expect(screen.getByText('张伟')).toBeTruthy();
```

- [ ] **Step 2: 运行并确认 RED**

Run:

```powershell
corepack pnpm --filter @mochat/dashboard exec vitest run src/features/phase33/phase33-closure-pages.test.tsx src/features/phase33/silent-customer-api.test.ts
```

Expected: FAIL，旧页面仍要求员工 ID 且没有详情。

- [ ] **Step 3: 实现明细、员工选择和批量动作**

动作可用性由当前状态映射：

```ts
const transitions: Record<string, string[]> = {
  pending: ['assign', 'followed', 'closed'],
  assigned: ['followed', 'awakened', 'closed'],
  followed: ['awakened', 'closed'],
  awakened: [],
  ignored: [],
  closed: [],
};
```

前端只用于按钮状态；后端 Task 8 必须重复校验。

- [ ] **Step 4: 实现规则配置**

规则名称非空、沉默天数 1–365；启停和删除确认前不请求；被历史记录引用时保留规则快照。

- [ ] **Step 5: 运行 GREEN**

Run:

```powershell
corepack pnpm --filter @mochat/dashboard exec vitest run src/features/phase33/phase33-closure-pages.test.tsx src/features/phase33/silent-customer-api.test.ts src/features/risk-warning/risk-warning-shell.test.tsx
corepack pnpm --filter @mochat/dashboard typecheck
```

Expected: PASS。

- [ ] **Step 6: 提交沉默客户页面**

```powershell
git add web/apps/dashboard/src/features/phase33/silent-customer-page.tsx web/apps/dashboard/src/features/phase33/silent-customer-records.tsx web/apps/dashboard/src/features/phase33/silent-customer-rules.tsx web/apps/dashboard/src/features/phase33/phase33-closure-pages.test.tsx web/apps/dashboard/src/main.tsx web/apps/dashboard/src/styles/risk-warning-workspace.css
git commit -m "feat: redesign silent customer workspace"
```

---

### Task 8: 实现沉默客户自动评估、动作状态机和扫描状态

**Files:**

- Create: `internal/dashboard/silent_customer_cron.go`
- Create: `internal/dashboard/silent_customer_cron_test.go`
- Modify: `internal/dashboard/phase33_closure.go`
- Modify: `internal/dashboard/phase33_closure_handler.go`
- Modify: `internal/dashboard/phase33_closure_handler_test.go`
- Modify: `internal/store/phase33_closure.go`
- Create: `internal/store/silent_customer_cron_test.go`
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `cmd/mochat-go/main.go`
- Modify: `internal/server/server.go`
- Modify: `internal/server/server_test.go`
- Modify: `deploy/standalone/.env.example`
- Modify: `deploy/standalone/docker-compose.yml`

**Interfaces:**

- Produces: `SilentCustomerCron.RunOnce(context.Context) error`.
- Store methods: `ActiveSilentCustomerRules`, `SilentCustomerActivities`, `UpsertSilentCustomerSnapshot`, `ResolveAwakenedSilentCustomers`, scan status start/finish.
- Config: `MOCHAT_GO_ENABLE_SILENT_CUSTOMER_CRON`, `MOCHAT_GO_SILENT_CUSTOMER_CRON_INTERVAL_SECONDS`, `MOCHAT_GO_SILENT_CUSTOMER_CRON_RUN_ON_START`.
- Endpoint: `GET /dashboard/silent-customer/scanner-status`.
- Enforces backend transitions for `assign/followed/awakened/closed`.

- [ ] **Step 1: 写失败的聚合与 cron 测试**

覆盖员工和客户双向消息的最大时间、当前企业、员工权限外数据不进入页面、首次命中、重复幂等、保持人工字段、恢复互动自动 `awakened`、审计记录和单个企业失败隔离。

- [ ] **Step 2: 写失败的动作状态机测试**

```go
func TestActSilentRecordsRejectsClosedToAssigned(t *testing.T) {
    // existing status=closed, action=assign
    if _, err := store.ActSilentRecords(ctx, 1, 1, 7, []int64{9}, "assign", 1002, ""); err == nil {
        t.Fatal("expected invalid transition")
    }
}
```

同时断言 assignee 属于当前企业和允许员工范围；批量动作在事务内逐项返回，而不是静默跳过。

- [ ] **Step 3: 运行并确认 RED**

Run:

```powershell
go test ./internal/dashboard ./internal/store ./internal/config ./internal/server ./cmd/mochat-go -run 'SilentCustomer|Silent' -count=1
```

Expected: FAIL，自动聚合、状态机、配置和状态接口尚不存在。

- [ ] **Step 4: 实现评估和状态持久化**

从 10 个归档分表按 `corp_id + customer_id + employee_id` 聚合 `MAX(msg_data_time)`，不将查看页面当作触发器。更新已有记录时只修改最后互动、沉默天数和系统状态，不覆盖人工负责人和备注。

- [ ] **Step 5: 实现动作事务和审计**

在同一事务内锁定记录、校验旧状态与员工范围、更新记录并插入审计。返回 `requested/succeeded/failed/items` 供前端逐项反馈。

- [ ] **Step 6: 运行 GREEN**

Run:

```powershell
go test ./internal/dashboard ./internal/store ./internal/config ./internal/server ./cmd/mochat-go -run 'SilentCustomer|Silent' -count=1
```

Expected: PASS。

- [ ] **Step 7: 提交沉默评估链**

```powershell
git add internal/dashboard/silent_customer_cron.go internal/dashboard/silent_customer_cron_test.go internal/dashboard/phase33_closure.go internal/dashboard/phase33_closure_handler.go internal/dashboard/phase33_closure_handler_test.go internal/store/phase33_closure.go internal/store/silent_customer_cron_test.go internal/config/config.go internal/config/config_test.go cmd/mochat-go/main.go internal/server/server.go internal/server/server_test.go deploy/standalone/.env.example deploy/standalone/docker-compose.yml
git commit -m "feat: evaluate and track silent customers"
```

---

### Task 9: 添加迁移、历史数据规范化、RBAC 和开发种子

**Files:**

- Create: `deploy/standalone/migrations/0148_intercept_silent_workspace.up.sql`
- Create: `deploy/standalone/migrations/0148_intercept_silent_workspace.down.sql`
- Modify: `internal/migration/migration_test.go`
- Modify: `internal/migration/provider_schema_reconcile_test.go`
- Modify: `internal/dashboard/dashboard_page_catalog.json`
- Modify: `internal/dashboard/dashboard_access_guard_test.go`
- Create: `scripts/seed_dev_risk_warning_workspace/main.go`
- Create: `scripts/seed_dev_risk_warning_workspace/README.zh-CN.md`

**Interfaces:**

- Normalizes legacy scopes: `customer → single`, `room → group`.
- Adds soft-delete columns if Task 3 adopts soft delete.
- Adds message identity/idempotency columns and unique key for intercept records.
- Adds scanner state tables for intercept and silent customer.
- Adds indexes for new filters and assignee joins.
- Registers scanner status, details, staff directory and action resources.

- [ ] **Step 1: 写失败迁移和权限测试**

迁移测试断言 up/down 成对、版本连续、关键表和索引存在；权限测试逐资源断言 method、path 和 `scopeRequired`。

- [ ] **Step 2: 运行并确认 RED**

Run:

```powershell
go test ./internal/migration ./internal/dashboard -run 'Migration|ProviderSchema|DashboardAccessGuard' -count=1
```

Expected: FAIL，0148 和新增资源尚不存在。

- [ ] **Step 3: 实现迁移和 action RBAC**

迁移使用 MariaDB 10.6 兼容 SQL，不依赖 PostgreSQL 语法。前端写操作资源拆分为规则配置、词库发布、记录复核和沉默处置，避免只有超级管理员能操作，也避免获得页面读取权限即拥有全部写权限。

- [ ] **Step 4: 实现幂等本地开发种子**

种子使用 `MOCHAT-DEV-RISK-WORKSPACE` 前缀，补充可测试的规则、版本、记录、审计、员工和沉默状态；只允许显式执行，禁止在应用启动时自动写入。

- [ ] **Step 5: 运行 GREEN**

Run:

```powershell
go test ./internal/migration ./internal/dashboard -run 'Migration|ProviderSchema|DashboardAccessGuard' -count=1
go run ./scripts/seed_dev_risk_warning_workspace --dry-run
```

Expected: PASS；dry-run 只报告计划写入，不改变数据库。

- [ ] **Step 6: 提交迁移、权限和种子**

```powershell
git add deploy/standalone/migrations/0148_intercept_silent_workspace.up.sql deploy/standalone/migrations/0148_intercept_silent_workspace.down.sql internal/migration/migration_test.go internal/migration/provider_schema_reconcile_test.go internal/dashboard/dashboard_page_catalog.json internal/dashboard/dashboard_access_guard_test.go scripts/seed_dev_risk_warning_workspace
git commit -m "feat: migrate intercept and silent workspace data"
```

---

### Task 10: 删除旧页面实现并完成自动化回归

**Files:**

- Delete: `web/apps/dashboard/src/features/phase33/message-intercept-pages.tsx`
- Delete: `web/apps/dashboard/src/features/phase33/phase33-closure-pages.tsx`
- Create: `web/apps/dashboard/src/features/conversation-operations/refuse-archive-page.tsx`
- Create: `web/apps/dashboard/src/features/conversation-operations/refuse-archive-page.test.tsx`
- Delete: `web/apps/dashboard/src/features/phase33/phase33-closure-pages.test.tsx`
- Modify: `web/apps/dashboard/src/main.tsx`
- Modify: `web/apps/dashboard/src/styles/index.css`
- Modify: `web/apps/dashboard/src/benchmark/page-registry.test.tsx`
- Modify: `docs/PROJECT_PROGRESS.zh-CN.md`

**Interfaces:**

- `main.tsx` imports the new focused page modules only.
- Old `.phase33-risk-warning-*` rules no longer control these pages.
- Project progress records only true follow-up work: client-side pre-send blocking and production enterprise verification.

- [ ] **Step 1: 写失败的旧实现清理测试**

页面注册测试断言三个路由使用新组件；拒绝存档测试覆盖迁移前已有行为；样式测试断言旧顶部卡和内联详情类不再被三个页面引用；源码扫描不出现 `Object.entries(selected)`、`分派员工 ID` 或 `perPage:50`。

- [ ] **Step 2: 删除旧导出和冲突样式**

将 `RefuseArchivePage` 原样提取到 `conversation-operations/refuse-archive-page.tsx`，把现有拒绝存档测试迁移到同目录；`main.tsx` 更新导入后删除两个旧聚合文件及旧测试。该步骤不改变拒绝存档的接口、样式或交互。

- [ ] **Step 3: 运行前端完整验证**

Run:

```powershell
corepack pnpm --filter @mochat/dashboard exec vitest run src/features/phase33 src/features/risk-warning src/styles/risk-warning-workspace-layout.test.ts src/benchmark/page-registry.test.tsx src/components/dashboard-pagination-contract.test.ts
corepack pnpm --filter @mochat/dashboard typecheck
corepack pnpm --filter @mochat/dashboard build
```

Expected: PASS；若全量测试存在与本任务无关的基线失败，必须给出精确测试名和已有改动证据，目标测试仍需全部通过。

- [ ] **Step 4: 运行后端完整验证**

Run:

```powershell
go test ./internal/dashboard ./internal/store ./internal/server ./internal/migration ./internal/config ./cmd/mochat-go -count=1
git diff --check
```

Expected: PASS。

- [ ] **Step 5: 更新总进度文档**

用中文记录已完成数据链、真实剩余专项、测试证据和待浏览器验收状态。页面不新增“能力未接入”文案。

- [ ] **Step 6: 提交清理和回归**

```powershell
git add web/apps/dashboard/src/features/phase33 web/apps/dashboard/src/main.tsx web/apps/dashboard/src/styles/index.css web/apps/dashboard/src/benchmark/page-registry.test.tsx docs/PROJECT_PROGRESS.zh-CN.md
git commit -m "refactor: retire legacy risk warning pages"
```

---

### Task 11: 部署、真实数据点击验收和文档回填

**Files:**

- Modify: `docs/PROJECT_PROGRESS.zh-CN.md`
- Modify: `docs/superpowers/specs/2026-08-21-message-intercept-keyword-silent-yuanhu-design.md`
- Modify: `docs/superpowers/plans/2026-08-21-message-intercept-keyword-silent-yuanhu.md`
- Create: `docs/reviews/2026-08-21-message-intercept-keyword-silent-acceptance.zh-CN.md`

**Interfaces:**

- Uses existing Compose project `mochat-go-desktop` and `deploy/standalone/.env.local`.
- Produces browser acceptance evidence for all three routes at wide and normal desktop sizes.

- [ ] **Step 1: 检查并执行本地开发种子**

先确认目标 corp/tenant，再执行显式种子；查询表验证数据只落到开发企业。禁止删除或重建数据库卷。

- [ ] **Step 2: 只重建 app**

Run:

```powershell
docker compose -p mochat-go-desktop --env-file deploy/standalone/.env.local -f deploy/standalone/docker-compose.yml up -d --build --no-deps app
docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
```

Expected: app、MySQL、Redis healthy；MySQL/Redis 容器和命名卷未重建。

- [ ] **Step 3: 点击验收消息拦截**

在 `/ai-insight/v2/message-intercept` 依次验证：

- 默认规则标签、显式查询、重置、刷新、分页和 URL 恢复；
- 新建/编辑、词库版本、会话范围、启停/删除确认与取消；
- 命中记录筛选、整行详情、审计时间线、遮罩、关闭、Escape；
- 勾选不误开详情、批量复核成功后刷新仍保持状态；
- 新归档开发消息经扫描后只生成一条幂等记录。

- [ ] **Step 4: 点击验收关键词库**

在 `/ai-insight/v2/keyword-library` 验证：

- 左右栏宽度稳定、切换词库无抖动；
- 新建/编辑词库、批量添加词条、重复反馈、词条分页；
- 启停/删除确认与取消；
- 发布新版本后版本号和规则下拉更新；
- 已被引用词库无法被无提示删除。

- [ ] **Step 5: 点击验收沉默客户**

在 `/ai-insight/v2/silent-customer` 验证：

- 客户、原员工、负责人、天数、状态和日期显式查询；
- 员工选择器显示姓名/部门，不显示员工 ID 输入；
- 整行详情、审计、遮罩、关闭、Escape；
- 批量分派、跟进、唤醒、关闭的按钮状态和确认；
- 操作成功后刷新页面状态不回退；
- 新归档互动经评估后正确新增或唤醒记录。

- [ ] **Step 6: 宽屏、常规宽度和运行检查**

检查 `2560×1440` 与常规桌面尺寸，无大面积无意义留白、重叠、模糊 hover、异常横向滚动。读取控制台错误和关键请求，确认没有未解释 4xx/5xx。

- [ ] **Step 7: 回填中文验收文档**

记录变更范围、数据来源、测试命令和结果、浏览器尺寸、点击步骤、扫描状态、容器健康、卷保留和仅剩后续专项。不得将未执行写成已通过。

- [ ] **Step 8: 提交验收证据**

```powershell
git add docs/PROJECT_PROGRESS.zh-CN.md docs/superpowers/specs/2026-08-21-message-intercept-keyword-silent-yuanhu-design.md docs/superpowers/plans/2026-08-21-message-intercept-keyword-silent-yuanhu.md docs/reviews/2026-08-21-message-intercept-keyword-silent-acceptance.zh-CN.md
git commit -m "docs: record intercept keyword and silent acceptance"
```

---

## Completion Definition

只有同时满足以下条件才可宣布三个菜单完成：

- 三个页面使用统一风险预警工作台，顶部介绍大卡和重复刷新已移除；
- 所有查询显式提交、分页固定 20、URL 可恢复；
- 关键词库的草稿、不可变发布版本和规则引用关系可验证；
- 消息拦截从真实归档消息自动生成幂等记录，记录、详情和复核审计闭环；
- 沉默客户从真实归档互动自动评估，分派/跟进状态机、负责人和审计闭环；
- 页面不显示技术字段、英文业务状态、手填员工 ID 或“能力未接入”占位；
- 读取、配置、复核和处置权限边界均通过后端测试；
- 目标测试、相关全量测试、类型检查、生产构建和 Go 回归通过；
- app、MySQL、Redis 健康，D 盘持久化和命名卷保留；
- 三个页面的布局、按钮样式、交互逻辑和刷新持久化均由浏览器实际点击验证。

## Plan Self-Review

- 规格覆盖：共享布局、三个页面、类型化 API、后端过滤、详情、版本、自动扫描、状态机、权限、迁移、种子、部署和浏览器验收分别由 Task 1–11 覆盖。
- 数据真实性：所有页面值来自现有业务表、归档消息、员工目录、扫描状态或显式开发种子；前端没有业务常量。
- 类型一致性：Task 2 输出 API；Task 3、6、8 输出后端合同；Task 4、5、7 消费；Task 9 注册迁移和权限；Task 10–11 做集成与验收。
- 占位符扫描：本文无待定占位、模糊引用或缺失验证命令的步骤。
- 范围检查：三个页面共享前端骨架和同一次部署，但自动扫描和领域模型保持独立；发送前客户端阻断明确留作专项。

## 实施回填（2026-08-21）

- [x] 三页共享工作台、显式查询、固定 20 条分页、URL 恢复和中文业务标签已完成。
- [x] 消息拦截记录/规则/详情/复核、关键词库草稿与发布版本、沉默客户筛选/负责人/处置状态均已接入真实 API 和数据库。
- [x] 规则保存、词库版本引用、员工选择器、批量处置确认和刷新持久化已由浏览器实际点击验证。
- [x] 开发样例词库发布快照修复迁移 `0150_repair_keyword_published_snapshots` 已应用；规则编辑保存闭环已复测通过。
- [x] Dashboard 全量测试 `136/136` 文件、`764/764` 测试通过；typecheck、生产构建和相关 Go 回归通过。
- [x] Docker app、MySQL、Redis 健康；未删除命名卷。仅需独立链路或客户端支持的后续专项已记录在总进度文档，不在页面展示占位能力文案。
