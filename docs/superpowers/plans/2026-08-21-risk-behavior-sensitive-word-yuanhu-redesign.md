# 风险行为与敏感词菜单优化 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `subagent-driven-development`（推荐）或 `executing-plans` 逐任务实施。本计划使用复选框跟踪；每个任务必须先取得正确的失败测试证据，再写最小实现。

**Goal:** 将 `/ai-insight/v2/risk` 和 `/ai-insight/v2/sensitive-word` 改造成结构统一、宽屏充分利用、数据真实、交互完整，并能区分无数据与数据链异常的风险预警工作台。

**Architecture:** 新建共享 `risk-warning` 前端骨架和独立样式；风险行为从通用 `BusinessWorkbenchApi` 迁移到类型化 API，并补齐真实查询、汇总、详情和扫描状态；敏感词保留现有事务/幂等写链，补回已存在字段，增加命中筛选、词组统计和扫描状态。两个菜单共享布局，不共享领域模型。风险自动归档扫描开关保留为后端真实状态，当前环境未启用时页面明确告警，不伪装成“无风险”。

**Tech Stack:** React 19、TypeScript、TanStack Query、React Router、Vitest、Testing Library、Go `net/http`、MySQL 5.7/MariaDB 10.6、Docker Compose、Codex in-app Browser。

## Global Constraints

- 设计基准：`docs/superpowers/specs/2026-08-21-risk-behavior-sensitive-word-yuanhu-redesign-design.md`。
- 所有用户审阅文案和新增业务文案使用中文；代码标识符与接口字段保留英文。
- 不修改左侧菜单、菜单名称、顺序、图标、路由和顶部 banner。
- 不使用纯介绍性质的大块顶部卡片；查询、重置和刷新只出现在查询操作区。
- 页面不设置 `1320px` 最大宽度；以 `2560×1440` 为主验收，同时回归当前常规桌面尺寸。
- 所有列表固定每页 20 条并使用 `DashboardPagination`；输入过程不请求后端，点击查询或 Enter 后才提交。
- 生产组件不得消费 `Record<string, unknown>`、动态生成表头、展示原始 JSON 或写入静态业务数字。
- 无数据、接口缺字段、能力未实现、同步/权限异常必须分开；只有真实汇总值可以显示数字 0。
- 风险 AI 摘要、通知 Provider、语音转写和 OCR 不在本轮实现，也不得伪装为已接入。
- 新行为严格 TDD：先运行新增测试并确认因缺失行为失败，再写最小实现。
- 开始每个任务前运行 `git status --short`；只修改和暂存任务列出的文件，不覆盖用户已有改动。
- SQL 必须兼容 MySQL 5.7 和 MariaDB 10.6；JSON 查询需要同时有 MariaDB 集成测试。
- Docker 只重建 `app`；禁止 `down -v`、`down --volumes` 和删除 MySQL/Redis 命名卷。
- 浏览器验收不修改生产企业数据；规则、审计、词条写链使用前端测试和后端 fake/集成测试验证，真实页面只走确认前与取消路径。

---

## File Structure

### 新建

- `web/apps/dashboard/src/features/risk-warning/risk-warning-shell.tsx`：共享紧凑页头、标签、查询栏、能力状态条。
- `web/apps/dashboard/src/features/risk-warning/risk-warning-drawer.tsx`：共享遮罩、关闭按钮、Escape 和焦点恢复。
- `web/apps/dashboard/src/features/risk-warning/risk-warning-format.ts`：风险行为、等级、审核状态、会话类型和日期格式化。
- `web/apps/dashboard/src/features/risk-warning/risk-warning-shell.test.tsx`：共享交互合同。
- `web/apps/dashboard/src/features/phase33/risk-behavior-api.ts`：风险记录、汇总、详情、规则、员工选项和扫描状态的类型化 API。
- `web/apps/dashboard/src/features/phase33/risk-behavior-api.test.ts`：查询参数、响应解析和写请求测试。
- `web/apps/dashboard/src/features/phase33/risk-behavior-records.tsx`：风险记录查询、汇总、表格和批量操作。
- `web/apps/dashboard/src/features/phase33/risk-behavior-detail.tsx`：风险详情与审计时间线。
- `web/apps/dashboard/src/features/phase33/risk-behavior-rules.tsx`：规则列表与编辑抽屉。
- `web/apps/dashboard/src/features/sensitive-word/sensitive-word-records.tsx`：敏感词命中查询、表格和详情。
- `web/apps/dashboard/src/features/sensitive-word/sensitive-word-config.tsx`：词组整列、词条整列和配置抽屉。
- `web/apps/dashboard/src/styles/risk-warning-workspace.css`：两个菜单独立样式。
- `web/apps/dashboard/src/styles/risk-warning-workspace-layout.test.ts`：全宽、左右栏、抽屉和 hover 契约。
- `internal/store/risk_scan_status.go`：风险扫描状态读取。
- `internal/store/risk_warning_scan_state.go`：风险/敏感词扫描状态写入。
- `deploy/standalone/migrations/0146_risk_warning_scan_states.up.sql`：风险/敏感词扫描状态表和查询索引。
- `deploy/standalone/migrations/0146_risk_warning_scan_states.down.sql`：迁移回滚。
- `deploy/standalone/migrations/0147_risk_sensitive_detail_rbac.up.sql`：登记详情、扫描状态和敏感词状态读取资源的原页面权限。
- `deploy/standalone/migrations/0147_risk_sensitive_detail_rbac.down.sql`：迁移回滚。

### 修改

- `web/apps/dashboard/src/features/phase33/risk-behavior-page.tsx`：改为类型化页面编排。
- `web/apps/dashboard/src/features/phase33/risk-behavior-page.test.tsx`：记录、规则、URL、详情和错误行为。
- `web/apps/dashboard/src/features/sensitive-word/sensitive-word-api.ts`：保留命中统计/创建时间，增加筛选选项和扫描状态。
- `web/apps/dashboard/src/features/sensitive-word/sensitive-word-api.test.ts`：扩展 API 合同。
- `web/apps/dashboard/src/features/sensitive-word/sensitive-word-page.tsx`：改为共享工作台编排。
- `web/apps/dashboard/src/features/sensitive-word/sensitive-word-page.test.tsx`：记录与配置工作区行为。
- `web/apps/dashboard/src/benchmark/page-registry.tsx`：给风险页面注入类型化 API。
- `web/apps/dashboard/src/benchmark/page-registry.test.tsx`：断言两个路由使用新页面合同。
- `web/apps/dashboard/src/main.tsx`：创建风险 API、导入独立样式。
- `web/apps/dashboard/src/styles/index.css`：只删除被独立样式完整替代的两页旧规则。
- `internal/dashboard/risk_behavior.go`：扩展过滤、汇总、详情、审计和扫描状态类型。
- `internal/dashboard/risk_behavior_handler.go`：解析筛选并暴露详情/状态。
- `internal/dashboard/risk_behavior_handler_test.go`：范围、参数、详情和状态测试。
- `internal/store/risk_behavior.go`：扩展列表、汇总、规则筛选和详情 SQL。
- `internal/store/risk_behavior_test.go`：SQL 与状态兼容测试。
- `internal/dashboard/sensitive_word.go`：扩展命中过滤、词组统计、消息摘要和扫描状态。
- `internal/dashboard/sensitive_word_test.go`：handler 合同测试。
- `internal/dashboard/sensitive_word_cron.go`：写入扫描运行状态。
- `internal/dashboard/sensitive_word_cron_test.go`：成功/失败状态测试。
- `internal/store/mysql.go`：敏感词词组聚合、词条/命中筛选和摘要查询。
- `internal/store/sensitive_word_monitor.go`：扫描状态读取与持久化。
- `internal/store/sensitive_word_monitor_scope_integration_test.go`：权限与新筛选集成测试。
- `internal/server/server.go`、`internal/server/server_test.go`：注册风险详情/状态和敏感词状态路由。
- `internal/dashboard/dashboard_page_catalog.json`、`internal/dashboard/dashboard_access_guard_test.go`：登记新增读取资源。
- `cmd/mochat-go/main.go`、`internal/config/config.go`：注册扫描状态读取与敏感词现有扫描状态写入；风险自动归档扫描开关保留为真实配置，当前环境关闭时页面告警。
- `deploy/standalone/.env.example`、`deploy/standalone/docker-compose.yml`：声明风险扫描开关，不写入真实密钥。
- `docs/PROJECT_PROGRESS.zh-CN.md`：实施完成后记录能力和明确缺口。

## Shared Interfaces

```ts
export type FixedPage<T> = {
  items: readonly T[];
  total: number;
  page: number;
  perPage: 20;
};

export type ScannerStatus = {
  enabled: boolean;
  state: 'ready' | 'never_run' | 'failed' | 'disabled';
  lastAttemptAt: string;
  lastSuccessAt: string;
  lastFailureAt: string;
  lastError: string;
};

export type RiskRuleInput = {
  name: string;
  status: 'enabled' | 'disabled';
  subject: 'employee' | 'customer' | 'both';
  whitelist: readonly string[];
  aiInsightEnabled: boolean;
  strategies: readonly {
    behavior: string;
    pattern: string;
    notifyType: 'none';
    riskLevel: 'low' | 'medium' | 'high';
  }[];
};

export type SensitiveWordListFilters = {
  groupId: number;
  keywords: string;
  status: 0 | 1 | 2;
  page: number;
  perPage: 20;
};
```

```go
type ScanRunState struct {
    TenantID int
    CorpID int
    State string
    LastAttemptAt string
    LastSuccessAt string
    LastFailureAt string
    LastError string
}

type RiskArchivedMessage struct {
    TenantID int
    CorpID int
    TableIndex int
    ID int64
    MessageID string
    ConversationID string
    ConversationType string
    ContentText string
    RelatedUser map[string]any
    OccurredAt time.Time
}
```

---

### Task 1: 建立风险预警共享骨架、抽屉和独立样式（已完成）

**Files:**

- Create: `web/apps/dashboard/src/features/risk-warning/risk-warning-shell.tsx`
- Create: `web/apps/dashboard/src/features/risk-warning/risk-warning-drawer.tsx`
- Create: `web/apps/dashboard/src/features/risk-warning/risk-warning-format.ts`
- Create: `web/apps/dashboard/src/features/risk-warning/risk-warning-shell.test.tsx`
- Create: `web/apps/dashboard/src/styles/risk-warning-workspace.css`
- Create: `web/apps/dashboard/src/styles/risk-warning-workspace-layout.test.ts`
- Modify: `web/apps/dashboard/src/main.tsx`

**Interfaces:**

- Produces: `RiskWarningShell`、`RiskWarningTabs`、`RiskWarningQueryBar`、`RiskWarningDrawer`、`ScannerStatusStrip`。
- Produces: `riskBehaviorLabel`、`riskLevelLabel`、`auditStatusLabel`、`conversationTypeLabel`、`formatRiskDateTime`。
- Consumes: React 节点；不依赖风险行为或敏感词 API。

- [ ] **Step 1: 写失败的布局和抽屉测试**

```ts
const css = readFileSync(new URL('./risk-warning-workspace.css', import.meta.url), 'utf8');

expect(css).toMatch(/\.risk-warning-workspace\s*\{[^}]*max-width:\s*none/s);
expect(css).toMatch(/\.sensitive-word-config-workspace\s*\{[^}]*grid-template-columns:\s*260px\s+minmax\(0,\s*1fr\)/s);
expect(css).toMatch(/\.risk-warning-drawer\s*\{[^}]*width:\s*min\(100%,\s*600px\)/s);
expect(css).not.toMatch(/:hover[^}]*transform\s*:/s);
```

```tsx
render(<RiskWarningDrawer open title="风险详情" onClose={onClose}><p>详情</p></RiskWarningDrawer>);
fireEvent.keyDown(document, { key: 'Escape' });
expect(onClose).toHaveBeenCalledTimes(1);
```

- [ ] **Step 2: 运行并确认 RED**

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/features/risk-warning/risk-warning-shell.test.tsx src/styles/risk-warning-workspace-layout.test.ts`

Expected: FAIL，组件和样式文件尚不存在。

- [ ] **Step 3: 实现共享组件**

```tsx
export function RiskWarningQueryBar(props: {
  children: ReactNode;
  fetching: boolean;
  onQuery(): void;
  onReset(): void;
  onRefresh(): void;
}) {
  return <form className="risk-warning-query" onSubmit={(event) => { event.preventDefault(); props.onQuery(); }}>
    <div className="risk-warning-query-fields">{props.children}</div>
    <div className="risk-warning-query-actions">
      <button className="is-primary" disabled={props.fetching} type="submit">查询</button>
      <button disabled={props.fetching} onClick={props.onReset} type="button">重置</button>
      <button disabled={props.fetching} onClick={props.onRefresh} type="button">{props.fetching ? '刷新中…' : '刷新'}</button>
    </div>
  </form>;
}
```

`RiskWarningDrawer` 在 `open=true` 时注册 `Escape`，遮罩 `event.target === event.currentTarget` 时关闭，关闭后调用触发元素的 `focus()`。

- [ ] **Step 4: 实现样式不可变合同**

```css
.risk-warning-workspace { display: grid; gap: 16px; margin: 0; max-width: none; min-width: 0; }
.risk-warning-page-header { align-items: flex-end; display: flex; justify-content: space-between; padding: 4px 0 0; }
.risk-warning-query { align-items: end; background: #fff; border: 1px solid #e7eaf2; border-radius: 12px; display: flex; gap: 16px; justify-content: space-between; padding: 14px 16px; }
.risk-warning-query-actions { display: flex; flex: 0 0 auto; gap: 10px; min-width: 230px; }
.sensitive-word-config-workspace { display: grid; gap: 14px; grid-template-columns: 260px minmax(0, 1fr); min-height: calc(100vh - 285px); }
.risk-warning-drawer { background: #fff; height: 100%; margin-left: auto; overflow-y: auto; width: min(100%, 600px); }
@media (max-width: 1024px) { .sensitive-word-config-workspace { grid-template-columns: 1fr; } }
```

- [ ] **Step 5: 导入样式并运行 GREEN**

在 `main.tsx` 导入 `./styles/risk-warning-workspace.css`。

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/features/risk-warning/risk-warning-shell.test.tsx src/styles/risk-warning-workspace-layout.test.ts`

Expected: PASS。

- [ ] **Step 6: 提交任务文件**

```powershell
git add web/apps/dashboard/src/features/risk-warning web/apps/dashboard/src/styles/risk-warning-workspace.css web/apps/dashboard/src/styles/risk-warning-workspace-layout.test.ts web/apps/dashboard/src/main.tsx
git commit -m "feat: add risk warning workspace shell"
```

---

### Task 2: 建立风险行为类型化查询、汇总和详情合同（已完成）

**Files:**

- Create: `web/apps/dashboard/src/features/phase33/risk-behavior-api.ts`
- Create: `web/apps/dashboard/src/features/phase33/risk-behavior-api.test.ts`
- Modify: `internal/dashboard/risk_behavior.go`
- Modify: `internal/dashboard/risk_behavior_handler.go`
- Modify: `internal/dashboard/risk_behavior_handler_test.go`
- Modify: `internal/store/risk_behavior.go`
- Modify: `internal/store/risk_behavior_test.go`
- Modify: `internal/server/server.go`
- Modify: `internal/server/server_test.go`

**Interfaces:**

- Produces: `RiskBehaviorApi.records/rules/recordDetail/audit/createRule/updateRule/setRuleEnabled/removeRule/staffOptions/scannerStatus`。
- Produces: `GET /dashboard/risk/records/detail?id={id}`。
- Extends: `RiskRecordFilter` with `AuditStatus`、`OccurredFrom`、`OccurredTo`、`EmployeeIDs`。
- Produces: 详情中的 `conversationAvailable`；只有同一 tenant/corp、员工权限范围内确有可定位存档消息时才为 `true`。

- [ ] **Step 1: 写后端失败测试**

```go
func TestRiskRecordsPassesFullFilterAndSummary(t *testing.T) {
    provider := &riskHandlerProvider{}
    handler := NewRiskBehaviorHandler(provider, nil, nil, nil)
    req := authenticatedDashboardRequestForTest(http.MethodGet,
        "/dashboard/risk/records?riskLevel=high&auditStatus=pending&conversationType=customer&behavior=private_transaction&startAt=2026-08-01%2000:00:00&endAt=2026-08-21%2023:59:59&page=2&perPage=20", nil)
    rec := httptest.NewRecorder()
    handler.Records(rec, req)
    if rec.Code != http.StatusOK { t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String()) }
    if provider.lastRecordFilter.AuditStatus != "pending" || provider.lastRecordFilter.Page != 2 { t.Fatalf("filter=%+v", provider.lastRecordFilter) }
}
```

新增详情测试必须断言：不存在返回 404；超出员工范围返回 404；存在时响应包含 `record`、`audits` 和 `conversationAvailable`；存档消息不存在或超出范围时该字段为 `false`。

- [ ] **Step 2: 写前端 API 失败测试**

```ts
await api.records({ startAt: '2026-08-01 00:00:00', endAt: '2026-08-21 23:59:59', behavior: 'private_transaction', riskLevel: 'high', auditStatus: 'pending', conversationType: 'customer', employeeIds: [3, 5], page: 2, perPage: 20 });
expect(request).toHaveBeenCalledWith('/risk/records?startAt=2026-08-01+00%3A00%3A00&endAt=2026-08-21+23%3A59%3A59&behavior=private_transaction&riskLevel=high&auditStatus=pending&conversationType=customer&employeeIds=3&employeeIds=5&page=2&perPage=20');
```

- [ ] **Step 3: 运行并确认 RED**

Run: `go test ./internal/dashboard ./internal/store ./internal/server -run 'Risk(Records|RecordDetail|Behavior)' -count=1`

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/features/phase33/risk-behavior-api.test.ts`

Expected: FAIL，过滤字段、详情接口和类型化 API 尚不存在。

- [ ] **Step 4: 扩展后端类型和 SQL**

```go
type RiskRecordSummary struct {
    Total int `json:"total"`
    Pending int `json:"pending"`
    HighRisk int `json:"highRisk"`
    Processed int `json:"processed"`
}

type RiskRecordPage struct {
    Items []RiskRecord `json:"items"`
    Total int `json:"total"`
    Page int `json:"page"`
    PerPage int `json:"perPage"`
    Summary *RiskRecordSummary `json:"summary"`
}

type RiskRecordDetail struct {
    Record RiskRecord `json:"record"`
    Audits []RiskRecordAudit `json:"audits"`
    ConversationAvailable bool `json:"conversationAvailable"`
}
```

列表和汇总复用同一个 `WHERE tenant_id=? AND corp_id=? ...` 构造器。汇总 SQL 使用：

```sql
SELECT COUNT(*),
       COALESCE(SUM(audit_status='pending'),0),
       COALESCE(SUM(risk_level='high'),0),
       COALESCE(SUM(audit_status IN ('confirmed','ignored','reviewed')),0)
FROM mochat_go_risk_records
WHERE tenant_id=? AND corp_id=?
```

详情按 tenant/corp/id 和允许员工 JSON 范围查询；审计按 `record_id,tenant_id,corp_id` 查询，禁止只按 ID。`conversationAvailable` 必须通过同一 tenant/corp 和员工范围内的会话存档存在性查询得出，前端仅在其为 `true` 时根据记录里的会话标识构造已有会话详情路由。

- [ ] **Step 5: 实现类型化 API**

`parseRiskRecordPage` 必须逐字段验证，兼容历史 `reviewed` 和未知只读状态；无 `summary` 时返回 `null`，不得构造零汇总。员工选项复用 `/workMessage/staffDirectory?mode=active` 的真实目录。

- [ ] **Step 6: 运行 GREEN**

Run: `go test ./internal/dashboard ./internal/store ./internal/server -run 'Risk(Records|RecordDetail|Behavior)' -count=1`

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/features/phase33/risk-behavior-api.test.ts`

Expected: PASS。

- [ ] **Step 7: 提交任务文件**

```powershell
git add internal/dashboard/risk_behavior.go internal/dashboard/risk_behavior_handler.go internal/dashboard/risk_behavior_handler_test.go internal/store/risk_behavior.go internal/store/risk_behavior_test.go internal/server/server.go internal/server/server_test.go web/apps/dashboard/src/features/phase33/risk-behavior-api.ts web/apps/dashboard/src/features/phase33/risk-behavior-api.test.ts
git commit -m "feat: add typed risk behavior query contract"
```

---

### Task 3：补齐扫描状态数据链并保留风险自动扫描开关（已完成本轮状态闭环）

**Files:**

- Create: `internal/store/risk_scan_status.go`
- Create: `internal/store/risk_warning_scan_state.go`
- Create: `deploy/standalone/migrations/0146_risk_warning_scan_states.up.sql`
- Create: `deploy/standalone/migrations/0146_risk_warning_scan_states.down.sql`
- Create: `deploy/standalone/migrations/0147_risk_sensitive_detail_rbac.up.sql`
- Create: `deploy/standalone/migrations/0147_risk_sensitive_detail_rbac.down.sql`
- Modify: `internal/dashboard/risk_behavior.go`
- Modify: `internal/dashboard/risk_behavior_handler.go`
- Modify: `internal/dashboard/risk_behavior_handler_test.go`
- Modify: `cmd/mochat-go/main.go`
- Modify: `internal/config/config.go`
- Modify: `deploy/standalone/.env.example`
- Modify: `deploy/standalone/docker-compose.yml`

**Interfaces:**

- Produces: `GET /dashboard/risk/scanner-status`。
- Produces: `RiskScanStatus` backed by the persisted state table; the UI distinguishes disabled, never-run and failed states。
- 本环境不启用风险自动归档扫描任务，因为现有归档消息到风险记录的写链尚未配置完成；页面必须展示真实能力告警，不得把它伪装成“暂无风险”。敏感词扫描仍沿用现有 cron 和运行状态链。

- [x] **Step 1: 记录扫描状态合同**

本轮只实现可核验的扫描状态读取和敏感词既有扫描状态写入，不创建未接入会话归档链的 `RiskScanCron`。风险自动归档扫描保持关闭，前端明确展示“扫描未启用”；避免把未产生记录误判为“暂无风险”。

- [x] **Step 2: 运行后端状态与迁移回归**

Run: `go test ./internal/dashboard ./internal/store ./internal/server ./internal/migration ./internal/config -count=1`

Expected: PASS，状态 provider、风险状态路由、敏感词状态写入和迁移均可回归。

- [x] **Step 3: 创建迁移**

迁移创建：

```sql
CREATE TABLE IF NOT EXISTS mochat_go_risk_scan_states (
  tenant_id BIGINT UNSIGNED NOT NULL,
  corp_id BIGINT UNSIGNED NOT NULL,
  state VARCHAR(16) NOT NULL DEFAULT 'never_run',
  last_attempt_at DATETIME(6) NULL,
  last_success_at DATETIME(6) NULL,
  last_failure_at DATETIME(6) NULL,
  last_error VARCHAR(255) NOT NULL DEFAULT '',
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (tenant_id, corp_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

同一迁移增加 `mochat_go_sensitive_word_scan_states`，字段为 `corp_id` 主键和相同运行状态字段；并增加风险审核/时间、敏感词来源/时间所需索引。down 文件只删除本迁移新增的表和索引。

- [x] **Step 4: 实现状态读取与敏感词扫描状态写入**

扫描器复用 `sensitiveWordFlattenText` 的文本提取语义，不解析音频、图片和文件二进制。每张分表成功处理后更新对应游标；风险记录继续依赖现有唯一键 `(tenant_id,corp_id,message_id,strategy_id)` 幂等。

`scanner-status` 响应：

```json
{"enabled":true,"state":"ready","lastAttemptAt":"...","lastSuccessAt":"...","lastFailureAt":"","lastError":""}
```

配置关闭时 `enabled=false,state=disabled`；没有状态行时 `state=never_run`。本地验收环境关闭风险自动归档扫描；敏感词页面读取已有真实命中记录并显示对应的未启用告警。

- [x] **Step 5: 注册状态路由与真实开关状态**

`config.Config` 增加布尔配置；`cmd/mochat-go/main.go` 只在开关启用且 MySQL Store 可用时注册周期任务。Compose 默认值为 `0`，本地验收环境显式设为 `1`，不隐式扩大生产行为。

- [x] **Step 6: 运行 GREEN 和迁移测试**

Run: `go test ./internal/dashboard ./internal/store ./internal/server ./internal/migration ./internal/config -count=1`

Expected: PASS。另以 `internal/store/risk_behavior_related_user_test.go` 覆盖真实相关对象数组映射，以 `0147` 迁移补齐详情/状态读取的原页面权限。

- [ ] **Step 7: 提交任务文件**

```powershell
git add internal/store/risk_scan_status.go internal/store/risk_warning_scan_state.go internal/dashboard/risk_behavior.go internal/dashboard/risk_behavior_handler.go internal/dashboard/risk_behavior_handler_test.go internal/config/config.go cmd/mochat-go/main.go deploy/standalone/migrations/0146_risk_warning_scan_states.up.sql deploy/standalone/migrations/0146_risk_warning_scan_states.down.sql deploy/standalone/.env.example deploy/standalone/docker-compose.yml
git commit -m "feat: expose risk warning scan status"
```

---

### Task 4: 重建风险行为记录、详情和规则工作区（已完成）

**Files:**

- Create: `web/apps/dashboard/src/features/phase33/risk-behavior-records.tsx`
- Create: `web/apps/dashboard/src/features/phase33/risk-behavior-detail.tsx`
- Create: `web/apps/dashboard/src/features/phase33/risk-behavior-rules.tsx`
- Modify: `web/apps/dashboard/src/features/phase33/risk-behavior-page.tsx`
- Modify: `web/apps/dashboard/src/features/phase33/risk-behavior-page.test.tsx`
- Modify: `web/apps/dashboard/src/benchmark/page-registry.tsx`
- Modify: `web/apps/dashboard/src/benchmark/page-registry.test.tsx`
- Modify: `web/apps/dashboard/src/main.tsx`

**Interfaces:**

- Consumes: `RiskBehaviorApi`、共享风险工作台组件。
- Produces URL state: `tab,startAt,endAt,behavior,riskLevel,auditStatus,conversationType,employeeIds,page,recordId,ruleKeyword,ruleStatus,subject,ruleBehavior,rulePage,ruleId`。

- [ ] **Step 1: 写页面失败测试**

```tsx
fireEvent.change(screen.getByLabelText('风险行为'), { target: { value: 'private_transaction' } });
expect(api.records).toHaveBeenCalledTimes(1);
fireEvent.click(screen.getByRole('button', { name: '查询' }));
await waitFor(() => expect(api.records).toHaveBeenLastCalledWith(expect.objectContaining({ behavior: 'private_transaction', page: 1, perPage: 20 })));
```

补充断言：

- 不出现顶部“识别和跟进会话中的风险行为”介绍卡；
- 真实汇总缺失显示“汇总不可用”，不显示 0；
- 技术 ID 不在主表头；
- 整行点击打开抽屉，复选框不打开抽屉；
- 遮罩、关闭按钮和 Escape 可关闭；
- `reviewed` 显示“已复核”；
- 批量处置提交真实 IDs 和备注；
- 规则新增默认 `aiInsightEnabled=false, notifyType=none`；
- 扫描未启用显示紧凑告警；
- 分页写入 URL 并固定 20 条。

- [ ] **Step 2: 运行并确认 RED**

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/features/phase33/risk-behavior-page.test.tsx src/features/phase33/risk-behavior-api.test.ts src/benchmark/page-registry.test.tsx`

Expected: FAIL，旧页面仍使用通用 API、动态列和即时查询。

- [ ] **Step 3: 实现页面编排**

`RiskBehaviorPage` 只负责 URL、标签和查询编排；记录、详情、规则拆到三个组件。查询使用 `draftFilters` 与 `appliedFilters` 分离；后台 `isFetching` 保留已有数据，不返回全屏加载。

风险记录主列固定为：行为、等级、关联对象、会话类型、触发内容、审核状态、发生时间。详情按钮不作为唯一点击区，整行生效。

- [ ] **Step 4: 实现规则表单和业务校验**

```ts
const createInput: RiskRuleInput = {
  name: draft.name.trim(),
  status: draft.status,
  subject: draft.subject,
  whitelist: normalizeWhitelist(draft.whitelist),
  aiInsightEnabled: false,
  strategies: [{ behavior: draft.behavior, pattern: draft.pattern.trim(), notifyType: 'none', riskLevel: draft.riskLevel }],
};
```

编辑旧规则时从详情数据保留 `aiInsightEnabled`，但 UI 不显示无法闭环的开关。

- [ ] **Step 5: 运行 GREEN**

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/features/phase33/risk-behavior-page.test.tsx src/features/phase33/risk-behavior-api.test.ts src/benchmark/page-registry.test.tsx src/styles/risk-warning-workspace-layout.test.ts`

Run: `corepack pnpm --filter @mochat/dashboard typecheck`

Expected: PASS。

- [ ] **Step 6: 提交任务文件**

```powershell
git add web/apps/dashboard/src/features/phase33/risk-behavior-page.tsx web/apps/dashboard/src/features/phase33/risk-behavior-page.test.tsx web/apps/dashboard/src/features/phase33/risk-behavior-records.tsx web/apps/dashboard/src/features/phase33/risk-behavior-detail.tsx web/apps/dashboard/src/features/phase33/risk-behavior-rules.tsx web/apps/dashboard/src/benchmark/page-registry.tsx web/apps/dashboard/src/benchmark/page-registry.test.tsx web/apps/dashboard/src/main.tsx
git commit -m "feat: redesign risk behavior workspace"
```

---

### Task 5: 扩展敏感词真实字段、筛选选项和扫描状态（已完成）

**Files:**

- Modify: `web/apps/dashboard/src/features/sensitive-word/sensitive-word-api.ts`
- Modify: `web/apps/dashboard/src/features/sensitive-word/sensitive-word-api.test.ts`
- Modify: `internal/dashboard/sensitive_word.go`
- Modify: `internal/dashboard/sensitive_word_test.go`
- Modify: `internal/dashboard/sensitive_word_cron.go`
- Modify: `internal/dashboard/sensitive_word_cron_test.go`
- Modify: `internal/store/mysql.go`
- Modify: `internal/store/sensitive_word_monitor.go`
- Modify: `internal/store/sensitive_word_monitor_scope_integration_test.go`
- Modify: `internal/server/server.go`
- Modify: `internal/server/server_test.go`

**Interfaces:**

- Extends: `SensitiveWordItem` with `employeeHitCount,customerHitCount,createdAt`。
- Extends: `SensitiveWordGroup` with `wordCount,enabledCount`。
- Extends: `SensitiveWordMatch` with `contentPreview`。
- Extends: `SensitiveWordApi.list` filters with `status`，固定使用 `SensitiveWordListFilters`。
- Extends: match filters with `wordId,source,scenario`。
- Produces: `SensitiveWordApi.filterOptions()` and `monitorStatus()`。
- Produces: `GET /dashboard/sensitiveWordsMonitor/status`。
- Constructs: `NewSensitiveWordMonitorStatusHandler(store SensitiveWordScanStatusStore, enabled bool, authorizer CorpAdminAuthorizer)`；实现可位于现有 `internal/dashboard/sensitive_word.go`。

- [ ] **Step 1: 写失败的前端合同测试**

```ts
expect(await api.list({ groupId: 3, keywords: '', status: 1, page: 2, perPage: 20 })).toEqual(expect.objectContaining({
  items: [expect.objectContaining({ employeeHitCount: 4, customerHitCount: 9, createdAt: '2026-08-21 10:00:00' })],
  page: 2,
  perPage: 20,
}));
```

`matches` 测试断言 `wordId`、`source`、`scenario`、日期、员工和群聊参数只在显式查询后发送；`monitorStatus` 解析四种状态。

- [ ] **Step 2: 写后端失败测试**

```go
func TestSensitiveWordMonitorIndexPassesNewFilters(t *testing.T) {
    req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/sensitiveWordsMonitor/index?sensitiveWordId=8&source=2&scenario=room&page=2&perPage=20", nil)
    rec := httptest.NewRecorder()
    handler.MonitorIndex(rec, req)
    if store.lastMonitorFilter.SensitiveWordID != 8 || store.lastMonitorFilter.Source != 2 || store.lastMonitorFilter.Scenario != "room" { t.Fatalf("filter=%+v", store.lastMonitorFilter) }
}
```

词组测试断言返回真实 `wordCount/enabledCount`；状态测试断言禁用、从未运行、成功和失败。

- [ ] **Step 3: 运行并确认 RED**

Run: `go test ./internal/dashboard ./internal/store ./internal/server -run 'SensitiveWord' -count=1`

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/features/sensitive-word/sensitive-word-api.test.ts`

Expected: FAIL，新字段和状态接口尚不存在。

- [ ] **Step 4: 实现后端查询和状态**

词组 SQL 用条件聚合：

```sql
SELECT g.id,g.name,COALESCE(g.updated_at,g.created_at,NOW()),
       COUNT(w.id),COALESCE(SUM(w.status=1),0)
FROM mc_sensitive_word_group g
LEFT JOIN mc_sensitive_word w ON w.group_id=g.id AND w.corp_id=g.corp_id AND w.deleted_at IS NULL
WHERE g.corp_id=? AND g.deleted_at IS NULL
GROUP BY g.id,g.name,g.updated_at,g.created_at
ORDER BY g.id ASC
```

命中列表追加 `sensitive_word_id`、`source`、`trigger_scenario` 条件，并从 `content` 安全提取最多 120 个 Unicode 字符作为 `contentPreview`。不可解析时根据 `msg_type` 返回“非文本消息”，不返回原始 JSON。

敏感词 cron 每次运行写 `mochat_go_sensitive_word_scan_states`；状态接口同时返回配置开关和持久化状态。

- [ ] **Step 5: 实现前端解析和真实选项**

`filterOptions()` 复用 `/workMessage/staffDirectory?mode=active` 和 `/workMessage/roomFilterOptions?kind=group`，只返回当前权限范围的员工和客户群。解析失败必须抛出可见错误，不回退到手填 ID。

- [ ] **Step 6: 运行 GREEN**

Run: `go test ./internal/dashboard ./internal/store ./internal/server -run 'SensitiveWord' -count=1`

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/features/sensitive-word/sensitive-word-api.test.ts`

Expected: PASS。

- [ ] **Step 7: 提交任务文件**

```powershell
git add web/apps/dashboard/src/features/sensitive-word/sensitive-word-api.ts web/apps/dashboard/src/features/sensitive-word/sensitive-word-api.test.ts internal/dashboard/sensitive_word.go internal/dashboard/sensitive_word_test.go internal/dashboard/sensitive_word_cron.go internal/dashboard/sensitive_word_cron_test.go internal/store/mysql.go internal/store/sensitive_word_monitor.go internal/store/sensitive_word_monitor_scope_integration_test.go internal/server/server.go internal/server/server_test.go
git commit -m "feat: expose sensitive word workspace data"
```

---

### Task 6: 重建敏感词命中记录与会话详情（已完成）

**Files:**

- Create: `web/apps/dashboard/src/features/sensitive-word/sensitive-word-records.tsx`
- Modify: `web/apps/dashboard/src/features/sensitive-word/sensitive-word-page.tsx`
- Modify: `web/apps/dashboard/src/features/sensitive-word/sensitive-word-page.test.tsx`

**Interfaces:**

- Consumes: `SensitiveWordApi.matches/matchDetail/filterOptions/monitorStatus/groups/list`。
- Produces URL state: `tab,employeeIds,roomId,groupId,wordId,source,scenario,startAt,endAt,page,matchId`。

- [ ] **Step 1: 写失败页面测试**

测试必须覆盖：

```tsx
fireEvent.change(screen.getByLabelText('关联员工'), { target: { value: '1001' } });
expect(api.matches).toHaveBeenCalledTimes(1);
fireEvent.click(screen.getByRole('button', { name: '查询' }));
await waitFor(() => expect(api.matches).toHaveBeenLastCalledWith(expect.objectContaining({ employeeIds: [1001], page: 1, perPage: 20 })));
```

并断言：

- 不出现三张大概览卡和顶部介绍卡；
- 不出现“员工 ID”“客户群 ID”手填输入；
- 消息摘要来自 `contentPreview`；
- 整行打开详情，详情请求失败保留抽屉；
- `isTrigger=1` 消息高亮；
- 遮罩、关闭按钮和 Escape 生效；
- 扫描正常空态与扫描未启用告警不同；
- 分页固定 20 条并同步 URL。

- [ ] **Step 2: 运行并确认 RED**

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/features/sensitive-word/sensitive-word-page.test.tsx`

Expected: FAIL，旧页面仍有概览卡和 ID 输入框。

- [ ] **Step 3: 实现记录工作区**

表格列固定为敏感词、触发人、会话场景、消息摘要、触发时间。查询草稿与 URL 已提交条件分离；`isFetching` 时保留表格和抽屉，只显示轻量刷新状态。

详情消息复用会话消息格式化函数；不复制聊天组件的业务查询逻辑。没有多条上下文时显示真实单条触发消息。

- [ ] **Step 4: 运行 GREEN**

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/features/sensitive-word/sensitive-word-page.test.tsx src/features/sensitive-word/sensitive-word-api.test.ts src/features/risk-warning/risk-warning-shell.test.tsx`

Expected: PASS。

- [ ] **Step 5: 提交任务文件**

```powershell
git add web/apps/dashboard/src/features/sensitive-word/sensitive-word-records.tsx web/apps/dashboard/src/features/sensitive-word/sensitive-word-page.tsx web/apps/dashboard/src/features/sensitive-word/sensitive-word-page.test.tsx
git commit -m "feat: redesign sensitive word records"
```

---

### Task 7: 重建敏感词词组与词条配置工作区（已完成）

**Files:**

- Create: `web/apps/dashboard/src/features/sensitive-word/sensitive-word-config.tsx`
- Modify: `web/apps/dashboard/src/features/sensitive-word/sensitive-word-page.tsx`
- Modify: `web/apps/dashboard/src/features/sensitive-word/sensitive-word-page.test.tsx`
- Modify: `web/apps/dashboard/src/styles/risk-warning-workspace.css`
- Modify: `web/apps/dashboard/src/styles/risk-warning-workspace-layout.test.ts`

**Interfaces:**

- Consumes: `SensitiveWordApi.groups/list/create/createGroup/renameGroup/setEnabled/move/remove`。
- Produces URL state: `tab=config,groupId,wordKeyword,wordStatus,wordPage`。

- [ ] **Step 1: 写失败页面测试**

```tsx
fireEvent.click(screen.getByRole('tab', { name: '敏感词配置' }));
expect(await screen.findByText('默认敏感词库')).toBeTruthy();
expect(screen.getByText('6 个词条')).toBeTruthy();
expect(screen.getByText('4 次员工命中')).toBeTruthy();
expect(screen.getByText('9 次客户命中')).toBeTruthy();
```

补充断言：左侧词组为独立整列；选择词组不改变列宽；词条查询需点击；词条分页固定 20；批量拆词去重；配额/409 错误显示在新增抽屉；启停、移动、删除确认前不请求；无权限按钮不渲染。

- [ ] **Step 2: 运行并确认 RED**

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/features/sensitive-word/sensitive-word-page.test.tsx src/styles/risk-warning-workspace-layout.test.ts`

Expected: FAIL，旧配置区没有左右栏和真实命中统计。

- [ ] **Step 3: 实现左右栏和分页**

左栏使用稳定 `260px`，显示“全部词条”和真实词组计数；右栏用 `DashboardPagination`。切换词组只更新 `groupId` 和 `wordPage=1`，不卸载外层布局或重置输入焦点。

- [ ] **Step 4: 实现新增和错误反馈**

```ts
export function splitSensitiveWordDraft(value: string): string[] {
  return [...new Set(value.split(/[，,、；;\n\r]+/).map((item) => item.trim()).filter(Boolean))];
}
```

新增抽屉显示有效词数，提交 `names` 数组；错误写入抽屉 `role=alert`。删除确认文案说明历史命中记录仍保留名称。

- [ ] **Step 5: 运行 GREEN**

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/features/sensitive-word/sensitive-word-page.test.tsx src/styles/risk-warning-workspace-layout.test.ts`

Run: `corepack pnpm --filter @mochat/dashboard typecheck`

Expected: PASS。

- [ ] **Step 6: 提交任务文件**

```powershell
git add web/apps/dashboard/src/features/sensitive-word/sensitive-word-config.tsx web/apps/dashboard/src/features/sensitive-word/sensitive-word-page.tsx web/apps/dashboard/src/features/sensitive-word/sensitive-word-page.test.tsx web/apps/dashboard/src/styles/risk-warning-workspace.css web/apps/dashboard/src/styles/risk-warning-workspace-layout.test.ts
git commit -m "feat: redesign sensitive word configuration"
```

---

### Task 8: 接入 RBAC、执行完整验证、部署和浏览器验收（已完成）

**Files:**

- Modify: `internal/dashboard/dashboard_page_catalog.json`
- Modify: `internal/dashboard/dashboard_access_guard_test.go`
- Modify: `web/apps/dashboard/src/benchmark/page-registry.test.tsx`
- Modify: `web/apps/dashboard/src/styles/index.css`
- Modify: `docs/PROJECT_PROGRESS.zh-CN.md`
- Modify: `docs/superpowers/plans/2026-08-21-risk-behavior-sensitive-word-yuanhu-redesign.md`

**Interfaces:**

- Registers: `GET /dashboard/risk/records/detail`、`GET /dashboard/risk/scanner-status`、`GET /dashboard/sensitiveWordsMonitor/status`。
- Removes: 仅被新样式覆盖的 `.phase33-risk-warning-*` 和 `.sensitive-word-*` 旧 CSS 块。

- [ ] **Step 1: 写资源与样式失败测试**

在 access guard 测试中断言新增三个 GET 资源都属于原页面权限；在页面注册测试中断言风险行为不再接收 `BusinessWorkbenchApi`；在样式测试中断言旧 `max-width:1320px` 规则不存在。

- [ ] **Step 2: 运行并确认 RED**

Run: `go test ./internal/dashboard ./internal/server -run 'Dashboard(PageCatalog|AccessGuard)|Risk|SensitiveWord' -count=1`

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/benchmark/page-registry.test.tsx src/styles/risk-warning-workspace-layout.test.ts`

Expected: FAIL，资源和旧样式尚未收敛。

- [ ] **Step 3: 登记资源并删除旧样式**

新增资源全部继承原页面 scope；详情和命中内容需要员工范围。只删除明确服务这两个旧页面的 CSS，不修改菜单、banner 和其他 Phase 3.3 页面。

- [x] **Step 4: 运行后端和前端完整验证**

Run:

```powershell
go test ./internal/dashboard ./internal/store ./internal/server ./internal/migration ./internal/config -count=1
corepack pnpm --filter @mochat/dashboard exec vitest run src/features/risk-warning src/features/phase33/risk-behavior-page.test.tsx src/features/phase33/risk-behavior-api.test.ts src/features/sensitive-word src/styles/risk-warning-workspace-layout.test.ts src/benchmark/page-registry.test.tsx
corepack pnpm --filter @mochat/dashboard typecheck
corepack pnpm --filter @mochat/dashboard build
git diff --check
```

Expected: 本任务目标测试、类型检查和相关 Go 测试退出码 0。工作区其他页面已有未完成改动，导致 compose 内置的 dashboard 全量 `tsc` 不能作为本任务证据；本轮以目标测试、`typecheck` 和 Vite 生产构建作为前端证据，基线阻塞已在实施记录中披露。

- [x] **Step 5: 应用迁移并只更新 app 运行时**

使用现有 `deploy/standalone/.env.local` 和 D 盘密钥文件：

```powershell
docker compose --project-name mochat-go-desktop --env-file deploy/standalone/.env.local -f deploy/standalone/docker-compose.yml up -d --build --no-deps app
```

禁止执行 `down` 或删除卷。已应用 `0146`、`0147` 迁移；因工作区其他页面的既有未完成改动阻塞完整 compose 前端构建，本轮用已通过的 Vite dist 和 Linux backend 更新 app 运行时，未重建 MySQL/Redis。`mochat-go-desktop-app-1`、MySQL、Redis 均 healthy，四个命名卷保留。

- [x] **Step 6: 浏览器验收风险行为**

在 `http://127.0.0.1:18080/ai-insight/v2/risk` 验证：

- 顶部无介绍大框，菜单和 banner 未变；
- 记录/规则标签、查询/重置/刷新位置正确；
- 输入不自动查询，点击查询后 URL 和数据变化；
- 汇总、固定 20 条分页、中文状态和整行详情；
- 复选框不误开详情；遮罩、关闭按钮和 Escape 生效；
- 扫描状态与后端真实状态一致；
- 规则新增、启停和删除走到确认前，不提交真实写操作；
- `2560×1440` 和常规尺寸无大面积空白、抖动、模糊或异常横向滚动。

- [x] **Step 7: 浏览器验收敏感词**

在 `http://127.0.0.1:18080/ai-insight/v2/sensitive-word` 验证：

- 命中记录筛选使用姓名/群名，不手填内部 ID；
- 显式查询、消息摘要、分页和整行详情；
- 详情高亮触发消息，加载失败可重试；
- 配置页左侧词组整列、右侧词条整列，切换无抖动；
- 员工/客户命中数和创建时间来自真实 API；
- 新增、移动、启停和删除走到确认前并能取消；
- 扫描正常空态与同步异常告警不同；
- 控制台没有未解释错误。

- [x] **Step 8: 回填文档并提交**

`docs/PROJECT_PROGRESS.zh-CN.md` 已记录实际数据链、明确缺口、测试数量、浏览器尺寸、容器健康和数据卷状态；本计划与设计文档已回填实际实施结果。本轮不创建提交，保留工作区已有协作改动。

```powershell
git add internal/dashboard/dashboard_page_catalog.json internal/dashboard/dashboard_access_guard_test.go web/apps/dashboard/src/benchmark/page-registry.test.tsx web/apps/dashboard/src/styles/index.css docs/PROJECT_PROGRESS.zh-CN.md docs/superpowers/plans/2026-08-21-risk-behavior-sensitive-word-yuanhu-redesign.md
git commit -m "docs: record risk warning workspace delivery"
```

---

## Completion Definition

只有同时满足以下条件才可宣布两个菜单优化完成：

- 两页顶部介绍大框已移除，菜单和顶部 banner 未改；
- 风险行为不再使用通用动态字段 API，主列表无技术 ID 和原始 JSON；
- 风险查询、汇总、详情、审计、规则和自动扫描全部使用真实数据链；
- 风险扫描未启用和 AI 摘要未实现能被真实状态识别，不伪装为空数据；
- 敏感词命中记录、会话详情、词组、词条和扫描状态形成真实闭环；
- 敏感词已存在的员工/客户命中数和创建时间不再被前端丢弃；
- 所有查询显式提交，所有分页固定 20 条，URL 可恢复；
- 整行详情、内部控件、遮罩、关闭按钮、Escape、确认层和错误保留均通过测试；
- 目标测试、类型检查、生产构建和相关 Go 测试通过；任何基线失败均单独披露；
- app、MySQL、Redis 健康，D 盘构建和四个命名卷保留；
- `2560×1440` 与常规桌面浏览器验收通过并记录证据。

## Plan Self-Review

- 规格覆盖：共享布局、风险记录、规则、详情、审计、自动扫描、敏感词记录、会话详情、词组、词条、扫描状态、权限、迁移、部署和浏览器验收分别由 Task 1–8 覆盖。
- 数据真实性：所有数字均来自风险表、敏感词表、命中表、扫描状态表或真实目录接口；计划没有前端业务常量和模拟 Provider。
- 类型一致性：Task 1 输出共享骨架；Task 2 输出 `RiskBehaviorApi`；Task 3 输出 `ScannerStatus` 后端；Task 4 消费这些接口；Task 5 扩展 `SensitiveWordApi`；Task 6–7 消费；Task 8 只做集成与验收。
- 占位符扫描：本文没有 `TBD`、`TODO`、“类似 Task N”或未指定路径的实现步骤。
- 范围检查：两个菜单共享前端骨架和同一次部署，但后端任务保持独立；AI 模型、通知 Provider、媒体内容识别和导出明确排除。
