# 超时预警与客户流失页面优化实施计划

> 当前状态：已确认并实施中。执行范围仅包含 `/ai-insight/v2/timeout` 与 `/ai-insight/v2/customer-loss`；会话菜单相关改动不属于本计划。

> **实施要求：** 使用 `executing-plans` 或 `subagent-driven-development` 按任务执行。每个行为变更必须先取得正确的失败测试证据，再写最小实现；每个任务完成后只暂存该任务文件。

**Goal:** 将 `/ai-insight/v2/timeout` 和 `/ai-insight/v2/customer-loss` 建成布局紧凑、数据真实、规则可配置、记录可处置、可导出并通过浏览器验收的风险预警工作台。

**Architecture:** 前端新增 `risk-warning` 领域目录，提供共享壳层、类型化 API、固定字段表格、URL 状态和抽屉。后端补齐现有 timeout provider 的筛选、详情、导出和归档消息自动评估；客户流失新增不可变事件快照、规则、审计和导出 provider，并在企业微信删除回调事务中生产记录。旧 `/workContact/lossContact` 保持兼容。

**Tech Stack:** React 19、TypeScript、TanStack Query、React Router、Vitest、Testing Library、Go `net/http`、MySQL 5.7/MariaDB、Docker Compose、Codex in-app Browser。

## Global Constraints

- 设计基准：`docs/superpowers/specs/2026-08-21-timeout-warning-customer-loss-yuanhu-design.md`。
- 用户已确认实施本计划；实施时所有新增用户文案使用中文。
- 不修改左侧菜单名称、顺序、图标、顶部 banner 和两个现有页面路由。
- 不复制圆弧 AI 的企业数据、统计数字、AI 摘要、H5 地址或示例记录。
- 页面不得显示“能力未接入”“当前无法识别”一类说明卡，也不得显示无真实写链路的开关或按钮。
- AI 摘要、超时通知分发和流失预测只登记到 `docs/PROJECT_PROGRESS.zh-CN.md`；页面不渲染对应入口。
- 页面不设置窄屏 `max-width`；主验收分辨率为 `2560×1440`，回归 `1440×900` 和 `1024×768`。
- 所有租户、企业和数据权限只来自认证上下文；客户端同名参数不得改变数据范围。
- 新迁移预留 `0146_risk_warning_providers`。实施开始先确认没有编号冲突；如有冲突，只整体顺延 up/down 文件和健康版本常量，不改变本计划表结构。
- SQL 兼容 MySQL 5.7；JSON 字段使用现有项目兼容写法。
- Docker 只重建 `app`，禁止 `down -v`、`down --volumes` 或删除 MySQL/Redis 命名卷。
- 当前工作树包含大量其他改动；禁止 reset、checkout 或清理，禁止把无关文件带入本任务提交。

---

## File Structure

### 新建

- `web/apps/dashboard/src/features/risk-warning/risk-warning-shell.tsx`：共享标签、查询区、工具栏、数据卡和抽屉壳层。
- `web/apps/dashboard/src/features/risk-warning/risk-warning-format.ts`：时间、时长、消息、状态和风险格式化。
- `web/apps/dashboard/src/features/risk-warning/risk-warning-target-picker.tsx`：按真实员工和部门名称选择监听对象/处理人。
- `web/apps/dashboard/src/features/risk-warning/risk-warning-api.ts`：两个页面的类型化 API、严格解析和下载。
- `web/apps/dashboard/src/features/risk-warning/risk-warning-api.test.ts`：查询参数、合同解析、下载测试。
- `web/apps/dashboard/src/features/risk-warning/timeout-warning-page.tsx`：超时记录、规则和高级设置编排。
- `web/apps/dashboard/src/features/risk-warning/timeout-warning-page.test.tsx`：超时页面行为测试。
- `web/apps/dashboard/src/features/risk-warning/customer-loss-page.tsx`：客户流失记录和规则编排。
- `web/apps/dashboard/src/features/risk-warning/customer-loss-page.test.tsx`：流失页面行为测试。
- `web/apps/dashboard/src/styles/risk-warning.css`：两页独立布局和控件样式。
- `web/apps/dashboard/src/styles/risk-warning-layout.test.ts`：宽屏、紧凑高度、响应式和 hover 合同。
- `internal/dashboard/timeout_warning_cron.go`：未回复消息周期评估。
- `internal/dashboard/timeout_warning_cron_test.go`：时长、回复、重复扫描和多策略测试。
- `internal/store/timeout_warning_pending.go`：归档消息待评估状态机和 cron store。
- `internal/store/timeout_warning_pending_test.go`：SQL/状态机单元测试。
- `internal/store/timeout_warning_pending_integration_test.go`：MariaDB 归档消息与超时记录闭环。
- `internal/dashboard/customer_loss_warning.go`：流失规则、记录、筛选、校验和 provider 接口。
- `internal/dashboard/customer_loss_warning_handler.go`：记录、详情、规则、处置和导出 handler。
- `internal/dashboard/customer_loss_warning_handler_test.go`：身份、筛选、权限、错误和 CSV 测试。
- `internal/store/customer_loss_warning.go`：流失 provider、规则匹配、快照和审计。
- `internal/store/customer_loss_warning_test.go`：where、规则匹配和幂等测试。
- `internal/store/customer_loss_warning_integration_test.go`：MariaDB 删除回调、回填、重新添加和跨企业测试。
- `deploy/standalone/migrations/0146_risk_warning_providers.up.sql`：超时待评估表和客户流失 provider 表。
- `deploy/standalone/migrations/0146_risk_warning_providers.down.sql`：只回滚本迁移新增的表和 timeout 快照列。

### 修改

- `web/apps/dashboard/src/benchmark/page-registry.tsx`：两个路由切换到新页面并注入 API。
- `web/apps/dashboard/src/benchmark/page-registry.test.tsx`：断言不再使用旧 phase33 页面。
- `web/apps/dashboard/src/main.tsx`：创建 `riskWarningApi` 并导入样式。
- `web/apps/dashboard/src/styles/index.css`：只删除旧两页专用且已被替代的 `.phase33-risk-warning-*` 规则。
- `internal/dashboard/timeout_warning.go`：扩展筛选、详情、审计和导出模型。
- `internal/dashboard/timeout_warning_handler.go`：解析筛选、详情和导出。
- `internal/dashboard/timeout_warning_handler_test.go`：扩展 handler 合同。
- `internal/store/timeout_warning.go`：日期/状态筛选、详情审计和导出查询。
- `internal/store/work_message_archive_sync.go`：归档插入成功后在同一事务更新待评估状态。
- `internal/store/work_message_archive_sync_test.go`：验证入站入队、出站关闭和重复消息幂等。
- `internal/store/mysql.go`：客户删除事务写入不可变流失记录；同步 reconcile 写入明确来源，员工离职批量删除不写流失。
- `internal/dashboard/wework_callback_worker_test.go`：验证两种删除事件状态和记录触发保持一致。
- `internal/server/server.go`、`internal/server/server_test.go`：注册新增路由。
- `cmd/mochat-go/main.go`：装配 handler 和 timeout cron。
- `internal/config/config.go`、`internal/config/config_test.go`：增加 timeout cron 开关、间隔和 run-on-start。
- `deploy/standalone/.env.example`、`deploy/standalone/docker-compose.yml`：登记 cron 配置。
- `internal/dashboard/dashboard_page_catalog.json`：登记两个页面的 read/manage/export 资源。
- `internal/dashboard/dashboard_access_guard_test.go`：验证页面资源和数据范围。
- `internal/migration/migration_test.go`、`internal/migration/provider_schema_reconcile_test.go`：迁移发现和表结构合同。
- `internal/dashboard/saas_admin_system_health.go` 及对应测试：将期望迁移版本同步到 146。
- `docs/PROJECT_PROGRESS.zh-CN.md`：只登记 AI 摘要、通知分发和流失预测三个专项边界，并记录本轮完成项。

旧文件 `web/apps/dashboard/src/features/phase33/timeout-warning-page.tsx`、`customer-loss-page.tsx` 在新路由和测试稳定后删除；若仍有测试直接引用，先迁移测试再删除。

## Shared Contracts

```ts
export type RiskLevel = 'unclassified' | 'low' | 'medium' | 'high';
export type AuditStatus = 'pending' | 'confirmed' | 'ignored' | 'closed';
export type RuleStatus = 'enabled' | 'disabled';
export type MonitorTarget = 'all' | 'employee' | 'department';

export type Page<T> = {
  items: readonly T[];
  total: number;
  page: number;
  perPage: number;
};

export type RiskTargetOption = {
  id: number;
  name: string;
  avatar: string;
};
```

前端 API 层必须逐字段解析；页面组件不得再接收 `Record<string, unknown>` 或根据 `Object.keys` 生成列。

---

### Task 1: 建立共享风险工作台和样式合同

**Files:**

- Create: `web/apps/dashboard/src/features/risk-warning/risk-warning-shell.tsx`
- Create: `web/apps/dashboard/src/features/risk-warning/risk-warning-format.ts`
- Create: `web/apps/dashboard/src/features/risk-warning/risk-warning-target-picker.tsx`
- Create: `web/apps/dashboard/src/styles/risk-warning.css`
- Create: `web/apps/dashboard/src/styles/risk-warning-layout.test.ts`
- Modify: `web/apps/dashboard/src/main.tsx`

**Produces:** `RiskWarningShell`、`RiskWarningTabs`、`RiskWarningQueryBar`、`RiskWarningDrawer`、`RiskWarningTargetPicker`、格式化函数。

- [ ] **Step 1: 写失败的 CSS 合同测试**

```ts
const css = readFileSync(new URL('./risk-warning.css', import.meta.url), 'utf8');

it('uses a compact full-width workspace', () => {
  expect(css).toMatch(/\.risk-warning-page[^}]*max-width:\s*none/s);
  expect(css).toMatch(/\.risk-warning-tabs[^}]*min-height:\s*4[0-8]px/s);
  expect(css).toMatch(/\.risk-warning-results[^}]*min-height:\s*0/s);
  expect(css).not.toMatch(/:hover[^}]*transform\s*:/s);
});

it('collapses filters and drawer on narrow screens', () => {
  expect(css).toMatch(/@media\s*\(max-width:\s*1024px\)/);
  expect(css).toMatch(/\.risk-warning-drawer[^}]*width:\s*min\(100%/s);
});
```

- [ ] **Step 2: 运行并确认因文件不存在失败**

Run: `corepack pnpm --filter @mochat/dashboard test -- src/styles/risk-warning-layout.test.ts`

Expected: FAIL，错误包含 `ENOENT risk-warning.css`。

- [ ] **Step 3: 实现壳层、抽屉和格式化**

```tsx
export function RiskWarningShell(props: { children: ReactNode }) {
  return <section className="risk-warning-page">{props.children}</section>;
}

export function RiskWarningDrawer(props: {
  label: string;
  onClose(): void;
  children: ReactNode;
}) {
  useEscape(props.onClose);
  return <div className="risk-warning-drawer-layer" onMouseDown={props.onClose}>
    <aside aria-label={props.label} className="risk-warning-drawer" onMouseDown={(event) => event.stopPropagation()}>
      {props.children}
    </aside>
  </div>;
}
```

`RiskWarningTargetPicker` 读取类型化员工/部门 options，显示名称、头像和已选标签；禁止出现“员工 ID”输入框。

- [ ] **Step 4: 实现样式并导入**

关键布局：

```css
.risk-warning-page {
  display: grid;
  grid-template-rows: auto auto minmax(0, 1fr);
  width: 100%;
  max-width: none;
  min-height: calc(100dvh - var(--dashboard-topbar-height));
}

.risk-warning-results {
  display: grid;
  grid-template-rows: auto minmax(0, 1fr) auto;
  min-height: 0;
}
```

- [ ] **Step 5: 运行测试**

Run: `corepack pnpm --filter @mochat/dashboard test -- src/styles/risk-warning-layout.test.ts`

Expected: PASS。

- [ ] **Step 6: 暂存并提交本任务**

```powershell
git add web/apps/dashboard/src/features/risk-warning/risk-warning-shell.tsx web/apps/dashboard/src/features/risk-warning/risk-warning-format.ts web/apps/dashboard/src/features/risk-warning/risk-warning-target-picker.tsx web/apps/dashboard/src/styles/risk-warning.css web/apps/dashboard/src/styles/risk-warning-layout.test.ts web/apps/dashboard/src/main.tsx
git diff --cached --check
git commit -m "feat: add risk warning workspace primitives"
```

---

### Task 2: 建立类型化前端 API

**Files:**

- Create: `web/apps/dashboard/src/features/risk-warning/risk-warning-api.ts`
- Create: `web/apps/dashboard/src/features/risk-warning/risk-warning-api.test.ts`
- Modify: `web/apps/dashboard/src/main.tsx`

**Consumes:** `apiClient.request`、`apiClient.download`。

**Produces:** `RiskWarningApi`，包含 timeout/customer-loss 的 records、detail、rules、settings、write、audit、assign、export、employeeOptions、departmentOptions。

- [ ] **Step 1: 写失败测试**

```ts
it('serializes timeout filters without client corp fields', async () => {
  await api.timeoutRecords({ customer: '张三', riskLevel: 'high', startDate: '2026-08-01', endDate: '2026-08-21', page: 2, perPage: 20 });
  expect(request).toHaveBeenCalledWith('/timeout-warning/records?customer=%E5%BC%A0%E4%B8%89&riskLevel=high&startDate=2026-08-01&endDate=2026-08-21&page=2&perPage=20');
});

it('rejects dynamic customer loss rows with missing typed fields', async () => {
  request.mockResolvedValue({ items: [{ avatar: 'x' }], total: 1, page: 1, perPage: 20 });
  await expect(api.customerLossRecords(defaultLossFilter)).rejects.toThrow('客户流失记录接口返回了无效数据');
});

it('uses authenticated download for exports', async () => {
  await api.exportTimeoutRecords(defaultTimeoutFilter);
  expect(download).toHaveBeenCalledWith(expect.stringContaining('/timeout-warning/records/export?'));
});
```

- [ ] **Step 2: 运行并确认模块不存在**

Run: `corepack pnpm --filter @mochat/dashboard test -- src/features/risk-warning/risk-warning-api.test.ts`

Expected: FAIL，无法解析 `risk-warning-api`。

- [ ] **Step 3: 实现严格解析和下载**

页面类型至少包含：

```ts
export type TimeoutRecord = {
  id: number; timeoutSeconds: number; triggerMessage: string; messageType: string;
  customerId: string; customerName: string; customerAvatar: string;
  employeeId: number; employeeName: string; conversationType: 'single' | 'group';
  riskLevel: Exclude<RiskLevel, 'unclassified'>; ruleId: number; ruleName: string;
  auditStatus: AuditStatus; assignedEmployeeId: number; occurredAt: string;
};

export type CustomerLossRecord = {
  id: number; lossType: 'employee_removed_customer' | 'customer_removed_employee';
  source: 'wecom_callback' | 'sync_reconcile' | 'legacy_backfill';
  customerId: number; customerName: string; customerAvatar: string;
  employeeId: number; employeeName: string; tagNames: readonly string[];
  lastMessage: string; lastMessageType: string; lastMessageAt: string;
  riskLevel: RiskLevel; ruleId: number; ruleName: string;
  auditStatus: AuditStatus; occurredAt: string;
};
```

- [ ] **Step 4: 运行测试和类型检查**

Run: `corepack pnpm --filter @mochat/dashboard test -- src/features/risk-warning/risk-warning-api.test.ts && corepack pnpm --filter @mochat/dashboard typecheck`

Expected: PASS。

- [ ] **Step 5: 提交**

```powershell
git add web/apps/dashboard/src/features/risk-warning/risk-warning-api.ts web/apps/dashboard/src/features/risk-warning/risk-warning-api.test.ts web/apps/dashboard/src/main.tsx
git diff --cached --check
git commit -m "feat: add typed risk warning api"
```

---

### Task 3: 扩展超时记录查询、详情和导出

**Files:**

- Modify: `internal/dashboard/timeout_warning.go`
- Modify: `internal/dashboard/timeout_warning_handler.go`
- Modify: `internal/dashboard/timeout_warning_handler_test.go`
- Modify: `internal/store/timeout_warning.go`
- Modify: `internal/server/server.go`
- Modify: `internal/server/server_test.go`

- [ ] **Step 1: 写 handler 和 store 失败测试**

断言：

- `ruleId/auditStatus/startDate/endDate/page/perPage` 透传；
- 非法日期和逆序日期返回 `400`；
- 详情返回记录和 audits，跨企业返回 `404`；
- 导出使用认证上下文和数据范围，返回 UTF-8 BOM、中文表头和安全单元格；
- 导出上限为 10,000 条。

```go
func TestTimeoutRecordsPassesDateAndScopeFilters(t *testing.T) { /* fake provider assertions */ }
func TestTimeoutRecordDetailRejectsCrossCorp(t *testing.T) { /* 404 */ }
func TestTimeoutRecordExportUsesBOMAndGuardsFormula(t *testing.T) { /* \xEF\xBB\xBF */ }
```

- [ ] **Step 2: 运行 RED**

Run: `go test ./internal/dashboard ./internal/store ./internal/server -run 'TestTimeout(Records|RecordDetail|RecordExport)' -count=1`

Expected: FAIL，缺少筛选字段、详情和导出 handler/provider。

- [ ] **Step 3: 扩展领域合同**

```go
type TimeoutRecordFilter struct {
    TenantID, CorpID int
    Customer, RiskLevel, ConversationType, AuditStatus string
    RuleID int64
    StartAt, EndAt time.Time
    Page, PerPage int
    AllowedEmployeeIDs []int
    RestrictEmployeeIDs bool
}

type TimeoutRecordDetail struct {
    Record TimeoutRecord `json:"record"`
    Audits []TimeoutRecordAudit `json:"audits"`
}
```

`TimeoutRecord` 补 `CustomerAvatar` 和规则名称快照读取；删除规则时保留记录可读。

- [ ] **Step 4: 实现 SQL 和 CSV**

所有 where 条件从同一个 `timeoutRecordWhere` 生成，分页、详情、批量写入和导出共享企业/员工数据范围。CSV 单元格复用或提取现有 work-message 导出的公式保护函数，不复制不一致实现。

- [ ] **Step 5: 注册路由**

新增：

```text
GET /dashboard/timeout-warning/records/detail?id={id}
GET /dashboard/timeout-warning/records/export
```

路由使用显式 option 字段和 server test；不使用未登记通配路由。

- [ ] **Step 6: 运行 GREEN 和回归**

Run: `go test ./internal/dashboard ./internal/store ./internal/server -run 'TestTimeout' -count=1`

Expected: PASS。

- [ ] **Step 7: 提交**

```powershell
git add internal/dashboard/timeout_warning.go internal/dashboard/timeout_warning_handler.go internal/dashboard/timeout_warning_handler_test.go internal/store/timeout_warning.go internal/server/server.go internal/server/server_test.go
git diff --cached --check
git commit -m "feat: complete timeout warning record queries"
```

---

### Task 4: 接通归档消息到超时预警的自动状态机

**Files:**

- Create: `internal/dashboard/timeout_warning_cron.go`
- Create: `internal/dashboard/timeout_warning_cron_test.go`
- Create: `internal/store/timeout_warning_pending.go`
- Create: `internal/store/timeout_warning_pending_test.go`
- Create: `internal/store/timeout_warning_pending_integration_test.go`
- Modify: `internal/store/work_message_archive_sync.go`
- Modify: `internal/store/work_message_archive_sync_test.go`
- Modify: `cmd/mochat-go/main.go`
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `deploy/standalone/.env.example`
- Modify: `deploy/standalone/docker-compose.yml`

- [ ] **Step 1: 写状态机 RED 测试**

覆盖：

1. 客户单聊入站写入 pending；
2. 客户群聊入站写入以群为会话键的 pending；
3. 员工在同会话回复关闭更早 pending；
4. 其他会话回复不关闭；
5. 重复归档消息不重复 pending；
6. 3 分钟和 10 分钟策略在各自时间只生成一次；
7. 先命中 3 分钟、5 分钟回复后不再命中 10 分钟；
8. 结束语、白名单、静音时段和员工/部门目标仍生效；
9. 180 分钟后完成并停止扫描。

```go
func TestTimeoutWarningCronEvaluatesReachedStrategiesOnce(t *testing.T) {}
func TestTimeoutPendingReplyClosesOnlySameConversation(t *testing.T) {}
```

- [ ] **Step 2: 运行 RED**

Run: `go test ./internal/dashboard ./internal/store -run 'TestTimeout(WarningCron|Pending)' -count=1`

Expected: FAIL，缺少 cron、pending store 和归档 hook。

- [ ] **Step 3: 定义 pending store 合同**

```go
type TimeoutPendingMessage struct {
    ID int64
    TenantID, CorpID int
    ConversationKey, ConversationType string
    TriggerMessageID, TriggerMessage, MessageType string
    CustomerID string
    CustomerName string
    CustomerAvatar string
    EmployeeID int64
    EmployeeName string
    DepartmentIDs []int64
    MessageAt time.Time
}

type TimeoutWarningCronStore interface {
    PendingTimeoutMessages(context.Context, int, time.Time) ([]TimeoutPendingMessage, error)
    EvaluateTimeoutMessage(context.Context, TimeoutEvaluation) (int, error)
    CompleteTimeoutPending(context.Context, int64, time.Time) error
}
```

- [ ] **Step 4: 在归档事务内更新 pending**

只有 `work_message` 实际插入成功才更新状态：客户入站 `INSERT ... ON DUPLICATE KEY`；员工出站按同一 conversation key 和时间窗口更新 `status='replied'`。归档同步失败时两者一起回滚。

- [ ] **Step 5: 实现周期任务**

默认每分钟扫描，单次有上限和稳定顺序；重复扫描依赖记录唯一键幂等。配置：

```text
MOCHAT_GO_ENABLE_TIMEOUT_WARNING_CRON=1
MOCHAT_GO_TIMEOUT_WARNING_CRON_INTERVAL=1m
MOCHAT_GO_TIMEOUT_WARNING_CRON_RUN_ON_START=1
```

- [ ] **Step 6: 运行单元和 MariaDB 集成测试**

Run: `go test ./internal/dashboard ./internal/store -run 'TestTimeout(WarningCron|Pending|Archive)' -count=1`

若设置 `MOCHAT_GO_MYSQL_INTEGRATION_DSN`，Expected: PASS；未设置时集成测试必须明确 `SKIP`，单元测试仍 PASS。

- [ ] **Step 7: 提交**

```powershell
git add internal/dashboard/timeout_warning_cron.go internal/dashboard/timeout_warning_cron_test.go internal/store/timeout_warning_pending.go internal/store/timeout_warning_pending_test.go internal/store/timeout_warning_pending_integration_test.go internal/store/work_message_archive_sync.go internal/store/work_message_archive_sync_test.go cmd/mochat-go/main.go internal/config/config.go internal/config/config_test.go deploy/standalone/.env.example deploy/standalone/docker-compose.yml
git diff --cached --check
git commit -m "feat: evaluate timeout warnings from archive messages"
```

---

### Task 5: 重做超时记录页面

**Files:**

- Create: `web/apps/dashboard/src/features/risk-warning/timeout-warning-page.tsx`
- Create: `web/apps/dashboard/src/features/risk-warning/timeout-warning-page.test.tsx`

- [ ] **Step 1: 写页面 RED 测试**

覆盖：

- 无大标题介绍卡、默认标签“超时记录”；
- 草稿筛选点击查询后才请求并写 URL；
- 重置、刷新、规则/状态/日期筛选、分页和每页条数；
- 固定中文列，不出现 `aiSummary`、`AI 摘要` 或原始英文键；
- 客户头像失败/缺失使用名称首字；
- 行详情、URL 恢复、遮罩和 Escape；
- 当前页全选、跨页清空、批量确认/忽略/关闭；
- 分派使用员工名称选择器；
- 导出调用当前已应用筛选；
- 无 manage/export 权限时按钮不渲染；
- 错误重试和写失败保留输入。

```tsx
expect(screen.queryByText('风险预警')).not.toBeInTheDocument();
expect(screen.queryByText('AI 摘要')).not.toBeInTheDocument();
await user.click(screen.getByRole('button', { name: '查询' }));
expect(api.timeoutRecords).toHaveBeenLastCalledWith(expect.objectContaining({ page: 1, customer: '张三' }));
```

- [ ] **Step 2: 运行 RED**

Run: `corepack pnpm --filter @mochat/dashboard test -- src/features/risk-warning/timeout-warning-page.test.tsx`

Expected: FAIL，页面文件不存在。

- [ ] **Step 3: 实现记录页**

使用 `DashboardPagination`，`perPage` 允许 10/20/50，默认 20。批量抽屉统一提交：

```ts
type TimeoutRecordAction =
  | { kind: 'audit'; action: 'confirmed' | 'ignored' | 'closed'; remark: string }
  | { kind: 'assign'; employeeId: number; remark: string };
```

触发消息只显示纯文本/类型标签，不使用 `dangerouslySetInnerHTML`。

- [ ] **Step 4: 运行页面测试、ESLint 和类型检查**

Run: `corepack pnpm --filter @mochat/dashboard test -- src/features/risk-warning/timeout-warning-page.test.tsx && corepack pnpm --filter @mochat/dashboard lint -- src/features/risk-warning && corepack pnpm --filter @mochat/dashboard typecheck`

Expected: PASS。

- [ ] **Step 5: 提交**

```powershell
git add web/apps/dashboard/src/features/risk-warning/timeout-warning-page.tsx web/apps/dashboard/src/features/risk-warning/timeout-warning-page.test.tsx
git diff --cached --check
git commit -m "feat: rebuild timeout warning records workspace"
```

---

### Task 6: 完成超时规则与高级设置

**Files:**

- Modify: `web/apps/dashboard/src/features/risk-warning/timeout-warning-page.tsx`
- Modify: `web/apps/dashboard/src/features/risk-warning/timeout-warning-page.test.tsx`
- Modify: `internal/dashboard/timeout_warning.go`
- Modify: `internal/dashboard/timeout_warning_handler.go`
- Modify: `internal/store/timeout_warning.go`

- [ ] **Step 1: 写 RED 测试**

覆盖：

- 规则名称/状态筛选和分页；
- 新增/编辑抽屉载入全部真实字段；
- all/employee/department 切换与名称选择；
- 单聊/群聊至少选一项；
- 1～5 条策略、3～180 分钟、同规则时长不可重复；
- 静音时段最多 5 条且起止合法；
- 启停与删除二次确认；
- 页面不存在 AI 开关、通知方式和员工 ID 输入；
- 设置以词组标签和消息类型按钮编辑，最多 5 组/每组 20 词；
- 保存失败保留草稿，成功后刷新。

- [ ] **Step 2: 运行 RED**

Run: `corepack pnpm --filter @mochat/dashboard test -- src/features/risk-warning/timeout-warning-page.test.tsx -t '规则|高级设置'`

Expected: FAIL，缺少相应交互或筛选合同。

- [ ] **Step 3: 补后端状态筛选和前端完整编辑器**

后端继续接受兼容字段 `notifyType`/`aiInsightEnabled`，但新页面保存时固定保留已有值；新增规则默认 `aiInsightEnabled=false`、`notifyType=none`，不创建误导 intent。删除规则时允许清理规则主体和子配置，但不得删除已产生的记录；历史记录继续使用 `rule_name` 快照，增加回归测试证明规则删除前后记录名称不变。

- [ ] **Step 4: 运行测试**

Run: `go test ./internal/dashboard ./internal/store -run 'TestTimeoutRule' -count=1 && corepack pnpm --filter @mochat/dashboard test -- src/features/risk-warning/timeout-warning-page.test.tsx && corepack pnpm --filter @mochat/dashboard typecheck`

Expected: PASS。

- [ ] **Step 5: 提交**

```powershell
git add web/apps/dashboard/src/features/risk-warning/timeout-warning-page.tsx web/apps/dashboard/src/features/risk-warning/timeout-warning-page.test.tsx internal/dashboard/timeout_warning.go internal/dashboard/timeout_warning_handler.go internal/store/timeout_warning.go
git diff --cached --check
git commit -m "feat: complete timeout rules and settings"
```

---

### Task 7: 建立风险 provider 迁移

**Files:**

- Create: `deploy/standalone/migrations/0146_risk_warning_providers.up.sql`
- Create: `deploy/standalone/migrations/0146_risk_warning_providers.down.sql`
- Modify: `internal/migration/migration_test.go`
- Modify: `internal/migration/provider_schema_reconcile_test.go`
- Modify: `internal/dashboard/saas_admin_system_health.go`
- Modify: 对应 health 测试文件

- [ ] **Step 1: 写迁移 RED 测试**

断言发现 0146、健康版本为 146，且 up.sql 包含：

```text
mochat_go_timeout_pending_messages
mochat_go_customer_loss_rules
mochat_go_customer_loss_rule_targets
mochat_go_customer_loss_records
mochat_go_customer_loss_record_audits
```

同时断言关键唯一键、租户企业查询索引、记录员工范围索引、timeout 记录快照列，以及 down.sql 只删除本迁移新增的表和列。

- [ ] **Step 2: 运行 RED**

Run: `go test ./internal/migration ./internal/dashboard -run 'Test.*(0146|ExpectedMigration|RiskWarning)' -count=1`

Expected: FAIL，迁移不存在或版本仍为 145。

- [ ] **Step 3: 编写迁移**

`mochat_go_timeout_pending_messages` 至少包含：范围、会话键、消息快照、客户/员工快照、部门 JSON、状态、message_at、replied_at、expires_at、created_at、updated_at。

现有 `mochat_go_timeout_records` 幂等增加：

```sql
`rule_name` varchar(80) NOT NULL DEFAULT '',
`customer_avatar` varchar(512) NOT NULL DEFAULT ''
```

迁移从当前规则和客户资料回填可获得的快照；此后写入新记录时直接保存快照，不依赖列表查询时临时关联当前资料。

客户流失记录表至少包含：

```sql
`source` varchar(24) NOT NULL,
`source_event_key` varchar(191) NOT NULL,
`relation_id` bigint unsigned NOT NULL,
`loss_type` varchar(32) NOT NULL,
`customer_id` bigint unsigned NOT NULL,
`customer_name` varchar(128) NOT NULL,
`customer_avatar` varchar(512) NOT NULL DEFAULT '',
`employee_id` bigint unsigned NOT NULL,
`employee_name` varchar(128) NOT NULL,
`tag_names_json` json NOT NULL,
`last_message_id` varchar(128) NOT NULL DEFAULT '',
`last_message` text NULL,
`last_message_type` varchar(32) NOT NULL DEFAULT '',
`last_message_at` datetime(6) NULL,
`rule_id` bigint unsigned NOT NULL DEFAULT 0,
`rule_name` varchar(80) NOT NULL DEFAULT '',
`risk_level` varchar(16) NOT NULL DEFAULT 'unclassified',
`audit_status` varchar(16) NOT NULL DEFAULT 'pending',
`occurred_at` datetime(6) NOT NULL
```

`source_event_key` 保证重复回调/同步幂等。历史回填使用 `legacy:{relation_id}:{status}:{deleted_at}`，排除员工离职关系。

- [ ] **Step 4: 运行迁移测试**

Run: `go test ./internal/migration ./internal/dashboard -run 'Test.*(0146|ExpectedMigration|RiskWarning)' -count=1`

Expected: PASS。

- [ ] **Step 5: 提交**

```powershell
git add deploy/standalone/migrations/0146_risk_warning_providers.up.sql deploy/standalone/migrations/0146_risk_warning_providers.down.sql internal/migration/migration_test.go internal/migration/provider_schema_reconcile_test.go internal/dashboard/saas_admin_system_health.go
git add internal/dashboard/saas_admin_system_health_test.go
git diff --cached --check
git commit -m "feat: add risk warning provider schema"
```

---

### Task 8: 实现客户流失领域、查询和规则

**Files:**

- Create: `internal/dashboard/customer_loss_warning.go`
- Create: `internal/store/customer_loss_warning.go`
- Create: `internal/store/customer_loss_warning_test.go`
- Create: `internal/store/customer_loss_warning_integration_test.go`

- [ ] **Step 1: 写领域和 store RED 测试**

覆盖：

- 规则名称、监控对象、流失类型和风险等级校验；
- employee/department/all 匹配；
- 多规则选最高风险，同风险选 ID 最小；
- 无规则生成 `unclassified`；
- 客户、员工、类型、风险、规则、状态、日期和数据范围筛选；
- 详情返回审计；
- 删除规则不删除历史记录；
- 批量处置只更新当前企业和允许员工记录；
- 触发次数只为实际选中规则增加；
- 重复 `sourceEventKey` 不插入、不增加计数。

```go
func TestSelectCustomerLossRulePrefersHighestRiskThenLowestID(t *testing.T) {}
func TestCustomerLossRecordPageAppliesEmployeeScope(t *testing.T) {}
func TestInsertCustomerLossRecordIsIdempotent(t *testing.T) {}
```

- [ ] **Step 2: 运行 RED**

Run: `go test ./internal/dashboard ./internal/store -run 'TestCustomerLoss' -count=1`

Expected: FAIL，领域和 provider 尚不存在。

- [ ] **Step 3: 实现领域合同**

```go
const (
    CustomerLossEmployeeRemoved CustomerLossType = "employee_removed_customer"
    CustomerLossCustomerRemoved CustomerLossType = "customer_removed_employee"
)

type CustomerLossRule struct { /* typed fields and snapshots */ }
type CustomerLossRecord struct { /* design fields */ }
type CustomerLossRecordFilter struct { /* scope, filters, dates, page */ }
type CustomerLossWarningProvider interface { /* list/detail/rules */ }
type CustomerLossWarningWriter interface { /* rules/status/delete/audit */ }
```

- [ ] **Step 4: 实现 store 和 MariaDB 用例**

所有 SQL 第一层包含 `tenant_id=? AND corp_id=?`；记录数据范围使用 `employee_id IN (...)`。JSON 解析失败必须返回错误，不静默降级成空数组。

- [ ] **Step 5: 运行测试**

Run: `go test ./internal/dashboard ./internal/store -run 'TestCustomerLoss' -count=1`

Expected: PASS；无 DSN 时集成测试明确 SKIP。

- [ ] **Step 6: 提交**

```powershell
git add internal/dashboard/customer_loss_warning.go internal/store/customer_loss_warning.go internal/store/customer_loss_warning_test.go internal/store/customer_loss_warning_integration_test.go
git diff --cached --check
git commit -m "feat: add customer loss warning provider"
```

---

### Task 9: 在真实删除事件中写入客户流失快照

**Files:**

- Modify: `internal/store/mysql.go`
- Modify: `internal/dashboard/wework_callback_worker_test.go`
- Modify: `internal/store/customer_loss_warning_integration_test.go`

- [ ] **Step 1: 写事件链 RED 测试**

MariaDB 场景：

1. `del_external_contact` 生成 `employee_removed_customer` + `wecom_callback`；
2. `del_follow_user` 生成 `customer_removed_employee` + `wecom_callback`；
3. 快照包含当时客户、员工、标签和流失前最后消息；
4. 回调重复不重复记录；
5. 客户重新添加后旧记录仍存在，再次删除生成新事件；
6. `removeMissingWorkContactEmployeeRelationsTx` 写 `sync_reconcile`；
7. `markWorkContactEmployeeRelationsRemovedTx` 的员工离职批量删除不写客户流失；
8. 规则匹配和关系状态更新同事务，任一失败全部回滚。

- [ ] **Step 2: 运行 RED**

Run: `go test ./internal/store ./internal/dashboard -run 'Test(CustomerLossEvent|WeWorkCallbackWorker.*RemovesContact)' -count=1`

Expected: FAIL，关系删除尚未调用快照 writer。

- [ ] **Step 3: 在事务中插入记录**

`RemoveWorkContactEmployeeRelation` 在确认 `RowsAffected=1` 后、提交前调用 `insertCustomerLossRecordTx`。event key 使用已存在的 callback 幂等业务键或“关系 ID + 状态 + 本次 deleted_at”稳定构造；禁止使用随机值规避唯一键。

同步 reconcile 只为明确由客户同步缺失产生的关系写记录；员工删除级联函数保持不写。

- [ ] **Step 4: 运行测试**

Run: `go test ./internal/store ./internal/dashboard -run 'Test(CustomerLossEvent|WeWorkCallbackWorker.*RemovesContact)' -count=1`

Expected: PASS。

- [ ] **Step 5: 提交**

```powershell
git add internal/store/mysql.go internal/dashboard/wework_callback_worker_test.go internal/store/customer_loss_warning_integration_test.go
git diff --cached --check
git commit -m "feat: capture customer loss events"
```

---

### Task 10: 暴露客户流失 HTTP、CSV 和 RBAC

**Files:**

- Create: `internal/dashboard/customer_loss_warning_handler.go`
- Create: `internal/dashboard/customer_loss_warning_handler_test.go`
- Modify: `internal/server/server.go`
- Modify: `internal/server/server_test.go`
- Modify: `cmd/mochat-go/main.go`
- Modify: `internal/dashboard/dashboard_page_catalog.json`
- Modify: `internal/dashboard/dashboard_access_guard_test.go`

- [ ] **Step 1: 写 handler/route/RBAC RED 测试**

覆盖所有设计接口、非法日期、跨企业、员工数据范围、manage/export 权限、CSV BOM/中文表头/公式保护、未知记录 404、非法动作 400。

- [ ] **Step 2: 运行 RED**

Run: `go test ./internal/dashboard ./internal/server -run 'TestCustomerLoss' -count=1`

Expected: FAIL，handler 和路由不存在。

- [ ] **Step 3: 实现并注册接口**

资源：

```text
/ai-insight/v2/customer-loss#read
/ai-insight/v2/customer-loss#manage
/ai-insight/v2/customer-loss#export
```

`GET records/detail/export` 和 `POST records/audit` 均为 `scopeRequired=true`；规则写入为企业级 manage。catalog 保留旧 `lossContact` 兼容资源，新增新 provider 资源。

- [ ] **Step 4: 运行 GREEN 和 catalog 回归**

Run: `go test ./internal/dashboard ./internal/server -run 'Test(CustomerLoss|DashboardAccess)' -count=1`

Expected: PASS。

- [ ] **Step 5: 提交**

```powershell
git add internal/dashboard/customer_loss_warning_handler.go internal/dashboard/customer_loss_warning_handler_test.go internal/server/server.go internal/server/server_test.go cmd/mochat-go/main.go internal/dashboard/dashboard_page_catalog.json internal/dashboard/dashboard_access_guard_test.go
git diff --cached --check
git commit -m "feat: expose customer loss warning routes"
```

---

### Task 11: 重做客户流失页面

**Files:**

- Create: `web/apps/dashboard/src/features/risk-warning/customer-loss-page.tsx`
- Create: `web/apps/dashboard/src/features/risk-warning/customer-loss-page.test.tsx`

- [ ] **Step 1: 写页面 RED 测试**

覆盖：

- “客户流失 / 流失规则”标签；
- 无大块介绍卡和动态英文表头；
- 客户、员工、流失类型、风险、规则、状态、日期筛选；
- 查询、重置、刷新、分页、perPage、URL 恢复；
- 固定中文列、中文状态和正确头像回退；
- 未匹配规则显示“未分级/未匹配规则”，不伪造风险；
- 详情结构和审计轨迹；
- 当前页全选和批量确认/忽略/关闭；
- 导出当前筛选；
- 规则筛选、分页、新增、编辑、启停、删除；
- 规则目标使用员工/部门名称；
- 页面没有 AI 摘要、预测评分、通知开关和原始员工 ID 输入；
- 权限、空态、错误态和写失败保留输入。

- [ ] **Step 2: 运行 RED**

Run: `corepack pnpm --filter @mochat/dashboard test -- src/features/risk-warning/customer-loss-page.test.tsx`

Expected: FAIL，页面不存在。

- [ ] **Step 3: 实现记录与规则标签**

表格行 key 使用记录 ID；头像 fallback 为 `customerName.trim().slice(0, 1) || '客'`，禁止固定“请”。长消息一行截断，详情抽屉展示完整纯文本。

规则保存合同：

```ts
type CustomerLossRuleInput = {
  id?: number;
  name: string;
  status: RuleStatus;
  monitorTarget: MonitorTarget;
  monitorTargetIds: readonly number[];
  lossTypes: readonly CustomerLossRecord['lossType'][];
  riskLevel: Exclude<RiskLevel, 'unclassified'>;
};
```

- [ ] **Step 4: 运行测试、lint、typecheck**

Run: `corepack pnpm --filter @mochat/dashboard test -- src/features/risk-warning/customer-loss-page.test.tsx && corepack pnpm --filter @mochat/dashboard lint -- src/features/risk-warning && corepack pnpm --filter @mochat/dashboard typecheck`

Expected: PASS。

- [ ] **Step 5: 提交**

```powershell
git add web/apps/dashboard/src/features/risk-warning/customer-loss-page.tsx web/apps/dashboard/src/features/risk-warning/customer-loss-page.test.tsx
git diff --cached --check
git commit -m "feat: rebuild customer loss workspace"
```

---

### Task 12: 切换路由、删除旧页面并更新总进度

**Files:**

- Modify: `web/apps/dashboard/src/benchmark/page-registry.tsx`
- Modify: `web/apps/dashboard/src/benchmark/page-registry.test.tsx`
- Modify: `web/apps/dashboard/src/main.tsx`
- Delete: `web/apps/dashboard/src/features/phase33/timeout-warning-page.tsx`
- Delete: `web/apps/dashboard/src/features/phase33/customer-loss-page.tsx`
- Modify/Delete: 对应旧测试（迁移有效合同后删除）
- Modify: `web/apps/dashboard/src/styles/index.css`
- Modify: `docs/PROJECT_PROGRESS.zh-CN.md`

- [ ] **Step 1: 写 registry RED 测试**

断言两个路由分别渲染 `TimeoutWarningPage` 和 `CustomerLossPage` 新模块，且 `businessWorkbenchApi` 不再注入旧通用页面。

- [ ] **Step 2: 运行 RED**

Run: `corepack pnpm --filter @mochat/dashboard test -- src/benchmark/page-registry.test.tsx`

Expected: FAIL，registry 仍引用 phase33 页面。

- [ ] **Step 3: 切换路由和清理旧样式**

只删除确定仅服务两个旧页面的 `.phase33-risk-warning-*` 规则；共享 `dashboard-*` 不删除。运行 `rg` 确认无旧模块引用后再删除文件。

- [ ] **Step 4: 更新总进度文档**

使用中文记录：

- 本轮完成：真实超时自动评估、客户流失事件快照、规则、筛选、处置、导出、浏览器验收；
- 后续专项：AI 摘要生成、超时通知分发、客户流失预测；
- 页面不展示上述专项缺口文案。

- [ ] **Step 5: 运行前端全量回归**

Run: `corepack pnpm --filter @mochat/dashboard test && corepack pnpm --filter @mochat/dashboard typecheck && corepack pnpm --filter @mochat/dashboard build`

Expected: PASS。

- [ ] **Step 6: 提交**

```powershell
git add web/apps/dashboard/src/benchmark/page-registry.tsx web/apps/dashboard/src/benchmark/page-registry.test.tsx web/apps/dashboard/src/main.tsx web/apps/dashboard/src/styles/index.css docs/PROJECT_PROGRESS.zh-CN.md
git add -u web/apps/dashboard/src/features/phase33
git diff --cached --check
git commit -m "feat: deliver risk warning pages"
```

---

### Task 13: 全栈验证、Docker 交付和浏览器验收

**Files:**

- Modify only if acceptance finds a scoped defect: files already listed above
- Create: `docs/reviews/2026-08-21-risk-warning-pages-acceptance.zh-CN.md`

- [ ] **Step 1: 检查工作树和差异范围**

Run: `git status --short && git diff --check && git diff --stat main...HEAD`

Expected: 本任务提交只包含计划文件；无空白错误；无用户其他改动被覆盖。

- [ ] **Step 2: 运行 Go 测试**

Run: `go test ./internal/dashboard ./internal/store ./internal/server ./internal/config ./internal/migration ./cmd/mochat-go -count=1`

Expected: PASS；若 MariaDB DSN 未配置，只有明确标记的集成测试 SKIP。

- [ ] **Step 3: 运行前端测试**

Run: `corepack pnpm --filter @mochat/dashboard test && corepack pnpm --filter @mochat/dashboard typecheck && corepack pnpm --filter @mochat/dashboard build`

Expected: PASS。

- [ ] **Step 4: 使用现有 Docker Compose 重建 app**

先解析 compose 文件和服务名，确认目标是当前仓库的 `app`，再运行：

Run: `docker compose -f deploy/standalone/docker-compose.yml build app`

Run: `docker compose -f deploy/standalone/docker-compose.yml up -d --no-deps app`

Expected: app healthy；MySQL/Redis 容器和卷未重建、未删除。

- [ ] **Step 5: 浏览器真实点击验收**

使用 in-app Browser 打开：

```text
http://127.0.0.1:18080/ai-insight/v2/timeout
http://127.0.0.1:18080/ai-insight/v2/customer-loss
```

按设计第 10.2 节逐项操作。必须至少验证：

- 两个分辨率截图；
- 每个标签和筛选；
- 分页与 URL 刷新恢复；
- 详情和 Escape；
- 开发测试数据上的规则写入、处置和导出；
- 超时测试消息入站、未回复后生成记录、及时回复不生成记录；
- 两种客户删除事件各生成一条正确类型记录；
- 页面没有 AI 摘要、通知开关、预测文案、英文动态列或“能力未接入”说明。

- [ ] **Step 6: 写中文验收报告**

报告包含：环境、提交、自动化命令结果、浏览器矩阵、截图路径、发现与修复、未决专项（只引用总进度文档）。

- [ ] **Step 7: 最终复核后提交验收报告**

Run: `git diff --check && git status --short`

```powershell
git add docs/reviews/2026-08-21-risk-warning-pages-acceptance.zh-CN.md
git diff --cached --check
git commit -m "docs: record risk warning page acceptance"
```

---

## Final Verification Checklist

- [ ] 两页不再有大块介绍卡，布局与按钮视觉一致；
- [ ] 所有列固定、中文、类型化，不使用动态 `Object.keys`；
- [ ] 筛选、分页、URL、详情、批量处置和导出均有真实测试；
- [ ] 超时记录由归档消息自动状态机生产，回复和多策略行为正确且幂等；
- [ ] 客户流失区分 status 2/3，事件快照在重新添加后仍保留；
- [ ] 员工离职关系清理不会误写客户流失；
- [ ] 规则、设置、审计和导出均受租户/企业/RBAC/员工数据范围约束；
- [ ] AI 摘要、通知分发和流失预测不在页面出现，只登记总进度；
- [ ] 迁移 up/down、版本健康、MariaDB 方言和索引测试通过；
- [ ] 前端 test/typecheck/build 与 Go 目标回归通过；
- [ ] Docker app 重建后真实浏览器验收通过；
- [ ] 没有覆盖、提交或删除工作树中其他人的改动。
