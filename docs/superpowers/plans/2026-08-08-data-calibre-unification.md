# 数据口径统一（Overview 接入 reporting 模块）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 Dashboard 数据概览 `/index` 与 Phase 3.5 报表 `/data/*` 使用同一后端（`internal/modules/reporting`）、同一数据表、同一筛选与权限契约，消除两套口径；同时移除概览页硬编码 0 与空态模块。

**Architecture:** 后端在 `reporting` 模块新增 `overview` 报表类型，复用现有 `queryEntity(CustomerReport)`、`queryConversion`、`queryBehavior`、`queryEmployee` 四个查询并合并结果；前端 `dashboard-overview-api.ts` 改为调用 `/reports/overview`，页面模块按真实数据或受限态渲染。legacy `/corpData/index` 保留不动，供其他旧消费者使用。

**Tech Stack:** Go 1.26（`net/http`）、React 19 + TanStack Query、Vitest、MariaDB 10.6、Docker Compose（项目名 `mochat-go-desktop`，端口 18080/13316）。

## Global Constraints

- 工作分支：`feat/2026-08-08-data-calibre-unification`（从 `main @ 9b6515e` 创建），**不推送到远端**。
- 不修改 legacy `/corpData/index`（`internal/dashboard/corp_data.go`）及其路由；它是历史兼容入口。
- 不改变 `customer / conversion / behavior / employee / report` 五个既有报表的响应契约与查询行为。
- 新接口授权权限键：`/dashboard/corpData/index#get`（复用数据概览菜单权限，不引入 `/data/overview#get`）。**不要使用 `/index#get`**：`RBACResolver` 按去掉 `#get` 后的路径精确匹配 `mc_rbac_menu.link_url`，菜单表中“系统首页”的 `link_url` 是 `/dashboard/corpData/index`（id=219）；`/index#get` 只有超管（`IsSuperAdmin=1`）能通过，普通角色会 403。
- 日期语义与 Phase 3.5 报表一致：`startAt = startDate T00:00:00+08:00`，`endAt = endDate T00:00:00+08:00`（半开区间，结束日期为次日 00:00 边界）。
- 概览默认区间与报表一致：开始 = 本月 1 日，结束 = 下月 1 日（Asia/Shanghai）。
- 用户可见文案使用中文；代码标识符、路径、接口字段保留英文。
- 每个任务必须 TDD：先写失败测试 → 运行确认失败 → 实现 → 运行确认通过 → 提交。
- 提交信息使用 Conventional Commits（`feat:` / `test:` / `docs:`）。
- 不推送；验收数据清理由主代理单独执行并记录。

---

## 设计决策（实现前必读）

1. **为什么新增 `overview` kind 而不是改 `/corpData/index`**：legacy handler 与 legacy 数据表强耦合，改它等于把两套逻辑塞进一个旧文件；新增 kind 让概览直接复用 Phase 3.5 已经验证的 SQL 与权限路径，后续其他页面也可复用。
2. **为什么概览 KPI 只取 4 个**：客户总数（customer）、线索总数（lead）、订单总数（order）、行为事件（behavior）分别对应 `/data/customer`、`/data/conversion`（起始阶段）、`/data/report`（订单经营）、`/data/behavior`；不再展示 legacy 的“客户群/群成员/入群/退群”等无法与 SCRM 对齐的指标。
3. **会话归档数据**：合并 `queryEmployee` 的结果；归档表不可用时返回 `provider_unavailable` limitation，前端显示“会话归档未接入”空态，绝不显示硬编码 0。
4. **AI/质检模块**：AI 洞察 5 页当前全部受限（`AI_INSIGHT_ENABLED=0`），概览页不再渲染 0 值网格，改为空态 + 配置入口链接。
5. **趋势**：复用 customer 报表的 `Series`（按天新增联系人），前端按日展示；不再提供“周/月”聚合下拉（报表页本身也没有该能力，避免伪功能）。
6. **已知限制（本轮不修，文档记录）**：`reportingPrincipalResolver` 未填充 `AllowedEmployeeIDs`，报表的员工级数据权限目前只做到菜单权限层；概览与既有报表保持一致，权限收敛改造另行立项。

---

### Task 1: 后端新增 `overview` 报表

**Files:**
- Modify: `internal/modules/reporting/contracts.go`
- Modify: `internal/modules/reporting/service.go`
- Modify: `internal/modules/reporting/sql_repository.go`
- Modify: `internal/modules/reporting/transport/http/handler.go`
- Test: `internal/modules/reporting/service_test.go`
- Test: `internal/modules/reporting/transport/http/handler_test.go`
- Test: `internal/modules/reporting/sql_repository_integration_test.go`

**Interfaces:**
- Consumes: 现有 `ReportQuery`、`ReportResult`、`Source`、`NewService`、`NewSQLRepository`。
- Produces: `reporting.OverviewReport`（值 `"overview"`）；`GET /dashboard/reports/overview` 可用；授权权限键为 `/dashboard/corpData/index#get`；响应 `ReportResult` 的 `Summary` 合并 `customer/lead/contact/opportunity/won/order/behavior/employee`（含转化率 `contactRate/opportunityRate/wonRate/orderRate`），`Series`/`Items`/`Pagination` 来自 customer 查询，`Limitations` 四源合并，`Freshness.Provider="scrm"`。

- [ ] **Step 1: 写失败测试（contracts + service + handler + integration）**

在 `internal/modules/reporting/service_test.go` 末尾追加：

```go
func TestParseKindAcceptsOverview(t *testing.T) {
	kind, ok := ParseKind("overview")
	if !ok || kind != OverviewReport {
		t.Fatalf("ParseKind(overview) = %q, %v", kind, ok)
	}
}

func TestServiceRunsOverviewThroughItsSource(t *testing.T) {
	source := &sourceStub{result: ReportResult{Summary: map[string]*float64{"customer": floatPtr(3)}}}
	service := NewService(map[ReportKind]Source{OverviewReport: source})
	if _, err := service.Query(context.Background(), OverviewReport, validQuery()); err != nil {
		t.Fatalf("overview query failed: %v", err)
	}
	if source.query.CorpID != 2 || source.query.Page != 1 {
		t.Fatalf("query passed to source = %#v", source.query)
	}
}

func floatPtr(value float64) *float64 { return &value }
```

在 `internal/modules/reporting/transport/http/handler_test.go` 末尾追加：

```go
type recordingAuthorizer struct {
	permission string
}

func (a *recordingAuthorizer) Authorize(_ context.Context, _ Principal, _ int64, permission string) error {
	a.permission = permission
	return nil
}

func TestOverviewReportUsesOverviewMenuPermission(t *testing.T) {
	authorizer := &recordingAuthorizer{}
	handler := NewHandler(serviceStub{}, resolverStub{}, authorizer)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/reports/overview?corpId=9&timezone=Asia%2FShanghai&startAt=2026-08-01T00:00:00Z&endAt=2026-08-02T00:00:00Z", nil)
	req.SetPathValue("kind", "overview")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if authorizer.permission != "/dashboard/corpData/index#get" {
		t.Fatalf("permission=%q want /dashboard/corpData/index#get", authorizer.permission)
	}
}

func TestReportKindUsesDataReportMenuPermission(t *testing.T) {
	authorizer := &recordingAuthorizer{}
	handler := NewHandler(serviceStub{}, resolverStub{}, authorizer)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/reports/report?corpId=9&timezone=Asia%2FShanghai&startAt=2026-08-01T00:00:00Z&endAt=2026-08-02T00:00:00Z", nil)
	req.SetPathValue("kind", "report")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if authorizer.permission != "/data/report#get" {
		t.Fatalf("permission=%q want /data/report#get", authorizer.permission)
	}
}
```

在 `internal/modules/reporting/sql_repository_integration_test.go` 的 kind 循环中把 `DetailReport` 改为：

```go
for _, kind := range []ReportKind{CustomerReport, ConversionReport, EmployeeReport, BehaviorReport, DetailReport, OverviewReport} {
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/modules/reporting/... -count=1`
Expected: FAIL —— `undefined: OverviewReport`。

- [ ] **Step 3: 实现 contracts.go**

在 `internal/modules/reporting/contracts.go` 的 const 块追加一行：

```go
	OverviewReport   ReportKind = "overview"
```

并把 `ParseKind` 的 switch 改为：

```go
	switch kind {
	case CustomerReport, EmployeeReport, ConversionReport, BehaviorReport, DetailReport, OverviewReport:
		return kind, true
	default:
		return "", false
	}
```

- [ ] **Step 4: 实现 service.go**

把 `NewSQLService` 的 map 改为：

```go
	return NewService(map[ReportKind]Source{
		CustomerReport:   NewSQLRepository(db, CustomerReport),
		EmployeeReport:   NewSQLRepository(db, EmployeeReport),
		ConversionReport: NewSQLRepository(db, ConversionReport),
		BehaviorReport:   NewSQLRepository(db, BehaviorReport),
		DetailReport:     NewSQLRepository(db, DetailReport),
		OverviewReport:   NewSQLRepository(db, OverviewReport),
	})
```

- [ ] **Step 5: 实现 sql_repository.go**

在 `Query` 的 switch 中 `case DetailReport:` 之后追加：

```go
	case OverviewReport:
		return r.queryOverview(ctx, q)
```

在 `querySummary` 函数之后新增：

```go
func (r *SQLRepository) queryOverview(ctx context.Context, q ReportQuery) (ReportResult, error) {
	customer, err := r.queryEntity(ctx, q, CustomerReport)
	if err != nil {
		return ReportResult{}, err
	}
	conversion, err := r.queryConversion(ctx, q)
	if err != nil {
		return ReportResult{}, err
	}
	behavior, err := r.queryBehavior(ctx, q)
	if err != nil {
		return ReportResult{}, err
	}
	employee, err := r.queryEmployee(ctx, q)
	if err != nil {
		return ReportResult{}, err
	}
	summary := map[string]*float64{}
	for _, result := range []ReportResult{customer, conversion, behavior, employee} {
		for key, value := range result.Summary {
			summary[key] = value
		}
	}
	limitations := append([]Limitation{}, customer.Limitations...)
	limitations = append(limitations, conversion.Limitations...)
	limitations = append(limitations, behavior.Limitations...)
	limitations = append(limitations, employee.Limitations...)
	return ReportResult{
		Summary:     summary,
		Series:      customer.Series,
		Items:       customer.Items,
		Pagination:  customer.Pagination,
		Freshness:   Freshness{Provider: "scrm", Status: "available"},
		Limitations: limitations,
	}, nil
}
```

- [ ] **Step 6: 实现 handler.go 权限映射**

把 `ServeHTTP` 中调用 authorizer 之前的代码改为：

```go
	permission := "/data/" + string(kind) + "#get"
	if kind == reporting.OverviewReport {
		permission = "/dashboard/corpData/index#get"
	}
	if h.authorizer != nil {
		if err := h.authorizer.Authorize(r.Context(), principal, query.CorpID, permission); err != nil {
			write(w, http.StatusForbidden, "forbidden", nil)
			return
		}
	}
```

- [ ] **Step 7: 运行测试确认通过**

Run: `go test ./internal/modules/reporting/... -count=1`
Expected: PASS（integration 测试因 build tag 默认不跑，不受影响）。

另跑：`go test ./cmd/mochat-go/... -count=1`，确认编译通过。

- [ ] **Step 8: 提交**

```bash
git add internal/modules/reporting
git commit -m "feat(reporting): add overview report kind for unified dashboard overview"
```

---

### Task 2: 前端概览页切换为 `/reports/overview`

> 状态：实现已提交（`ea4f27f`，`feat(dashboard): switch overview to unified reporting data source`），依赖 Task 1 后端接口；提交内容已包含本任务全部 Step，待验证矩阵统一复跑。

**Files:**
- Modify: `web/apps/dashboard/src/features/dashboard-overview/dashboard-overview-api.ts`
- Modify: `web/apps/dashboard/src/features/dashboard-overview/dashboard-overview-api.test.ts`
- Modify: `web/apps/dashboard/src/features/dashboard-overview/dashboard-overview-page.tsx`
- Modify: `web/apps/dashboard/src/features/dashboard-overview/dashboard-overview-page.test.tsx`

**Interfaces:**
- Consumes: `ApiClient.request`（已解包 `{code,msg,data}`，直接拿到报表 `data`）；`DashboardOverviewPage` 现有调用契约。
- Produces: `DashboardOverviewApi.load(input)` 请求 `/reports/overview`；`DashboardOverview` 新类型（`summary` 8 键、`trend` 单序列 `{date,addCustomerNum}`、`limitations`、`updatedAt`、`page/pageSize/total`）；页面移除 period 下拉与硬编码 0 模块。

- [ ] **Step 1: 写失败测试（api + page）**

将 `web/apps/dashboard/src/features/dashboard-overview/dashboard-overview-api.test.ts` 整体替换为：

```ts
import { describe, expect, it, vi } from 'vitest';

import { createDashboardOverviewApi } from './dashboard-overview-api';

const reportResponse = {
  summary: {
    customer: 137, lead: 58, contact: 46, opportunity: 30, won: 15, order: 12, behavior: 41, employee: 0,
    contactRate: 0.3358, opportunityRate: 0.6522, wonRate: 0.5, orderRate: 0.8,
  },
  series: [{ at: '2026-07-30T00:00:00Z', value: 12 }],
  items: [{ id: 'c1', day: '2026-07-30', ownerId: 0, ownerName: '' }],
  pagination: { page: 1, pageSize: 20, total: 1 },
  freshness: { provider: 'scrm', status: 'available', dataThrough: '2026-07-31T09:30:00Z' },
  limitations: [{ provider: 'conversation_archive', code: 'provider_unavailable', message: '会话归档表不可用' }],
};

describe('createDashboardOverviewApi', () => {
  it('loads the unified reporting overview and parses cards, trend and limitations', async () => {
    const request = vi.fn(() => Promise.resolve(reportResponse));
    const api = createDashboardOverviewApi({ request });

    const result = await api.load({
      corpId: '7',
      startDate: '2026-07-01',
      endDate: '2026-08-01',
      employeeIds: [],
      departmentIds: [],
      page: 1,
      pageSize: 20,
    });

    expect(result).toEqual(expect.objectContaining({
      cards: [
        { key: 'customer', label: '客户总数', value: 137 },
        { key: 'lead', label: '线索总数', value: 58 },
        { key: 'order', label: '订单总数', value: 12 },
        { key: 'behavior', label: '行为事件', value: 41 },
      ],
      trend: [{ date: '2026-07-30', addCustomerNum: 12 }],
      summary: expect.objectContaining({ customer: 137, employee: 0 }),
      limitations: reportResponse.limitations,
      updatedAt: '2026-07-31 09:30:00',
      page: 1,
      pageSize: 20,
      total: 1,
    }));
    expect(request).toHaveBeenCalledWith(
      '/reports/overview?corpId=7&timezone=Asia%2FShanghai&startAt=2026-07-01T00%3A00%3A00%2B08%3A00&endAt=2026-08-01T00%3A00%3A00%2B08%3A00&page=1&pageSize=20',
    );
  });

  it('serializes employee and department filters and pagination', async () => {
    const request = vi.fn(() => Promise.resolve({ summary: {}, series: [], pagination: { page: 2, pageSize: 20, total: 0 }, freshness: { provider: 'scrm', status: 'available' }, limitations: [] }));
    const api = createDashboardOverviewApi({ request });

    await api.load({
      corpId: '7',
      startDate: '2026-07-01',
      endDate: '2026-08-01',
      employeeIds: ['9', '12'],
      departmentIds: ['3', '8'],
      page: 2,
      pageSize: 20,
    });

    expect(request).toHaveBeenCalledWith(
      '/reports/overview?corpId=7&timezone=Asia%2FShanghai&startAt=2026-07-01T00%3A00%3A00%2B08%3A00&endAt=2026-08-01T00%3A00%3A00%2B08%3A00&employeeIds=9&employeeIds=12&departmentIds=3&departmentIds=8&page=2&pageSize=20',
    );
  });

  it('exports csv with the same serialized query and a single trend column', async () => {
    const request = vi.fn(() => Promise.resolve(reportResponse));
    const api = createDashboardOverviewApi({ request });
    const blob = await api.exportCsv({
      corpId: '7',
      startDate: '2026-07-01',
      endDate: '2026-08-01',
      employeeIds: ['9'],
      departmentIds: ['3'],
      page: 1,
      pageSize: 20,
    });
    expect(blob.type).toBe('text/csv;charset=utf-8');
    expect(request).toHaveBeenCalledWith(
      '/reports/overview?corpId=7&timezone=Asia%2FShanghai&startAt=2026-07-01T00%3A00%3A00%2B08%3A00&endAt=2026-08-01T00%3A00%3A00%2B08%3A00&employeeIds=9&departmentIds=3&page=1&pageSize=20',
    );
  });

  it('rejects an incompatible legacy response instead of crashing', async () => {
    const request = vi.fn(() => Promise.resolve({ weChatContactNum: 137, updateTime: '2026-07-31 09:30:00' }));
    const api = createDashboardOverviewApi({ request });
    await expect(api.load({
      corpId: '7',
      startDate: '2026-07-01',
      endDate: '2026-08-01',
      employeeIds: [],
      departmentIds: [],
      page: 1,
      pageSize: 20,
    })).rejects.toThrow('数据概览接口返回了无效数据');
  });
});
```

将 `web/apps/dashboard/src/features/dashboard-overview/dashboard-overview-page.test.tsx` 的 `overview` fixture 替换为：

```ts
const overview: DashboardOverview = {
  cards: [
    { key: 'customer', label: '客户总数', value: 137 },
    { key: 'lead', label: '线索总数', value: 58 },
    { key: 'order', label: '订单总数', value: 12 },
    { key: 'behavior', label: '行为事件', value: 41 },
  ],
  trend: [{ date: '2026-07-30', addCustomerNum: 12 }],
  summary: {
    customer: 137, lead: 58, contact: 46, opportunity: 30, won: 15, order: 12, behavior: 41, employee: 0,
  },
  limitations: [{ provider: 'conversation_archive', code: 'provider_unavailable', message: '会话归档表不可用' }],
  updatedAt: '2026-07-31 09:30:00',
  page: 1,
  pageSize: 20,
  total: 1,
};
```

并把 `dashboard-overview-page.test.tsx` 中以下断言按新页面结构更新：

- `builds the default range...`：固定时间 `2026-08-01T16:30:00Z` 时，`load` 收到的 `startDate: '2026-08-01'`、`endDate: '2026-09-01'`，且 `load` 调用不再包含 `period` 字段。
- `shows loading, then renders cards and trend values...`：改为断言 `客户总数`、`137`、`2026-07-30`、`新增客户 12`（`aria-label`）、更新时间和 `load` 调用参数（无 `period`）。
- `keeps the complete business dashboard visible for a successful empty response...`：空响应 fixture 改为 `{ cards: [], trend: [], summary: { customer: 0, lead: 0, contact: 0, opportunity: 0, won: 0, order: 0, behavior: 0, employee: 0 }, limitations: [], updatedAt: '', page: 1, pageSize: 20, total: 0 }`；断言标题改为 `数据概览`、`经营概览`、`AI 洞察`、`经营趋势明细` 存在，且 **不再断言** `会话数据`、`质检数据`、`员工会话数据排行`、`员工会话轨迹一览` 存在。
- `restores complete filters from the URL, changes the trend period, and paginates`：URL 改为 `/index?startDate=2026-07-01&endDate=2026-08-01&employeeIds=9&employeeIds=12&departmentIds=3&page=2&pageSize=20`；删除“趋势周期”下拉断言；保留分页断言。
- `shows an export error...`：URL 去掉 `period` 参数；`exportCsv` 调用参数去掉 `period`。

- [ ] **Step 2: 运行测试确认失败**

Run: `pnpm --filter @mochat/dashboard test -- dashboard-overview`
Expected: FAIL（类型不匹配 / 请求 URL 不匹配）。

- [ ] **Step 3: 实现 dashboard-overview-api.ts**

将 `web/apps/dashboard/src/features/dashboard-overview/dashboard-overview-api.ts` 整体替换为：

```ts
export type DashboardOverviewCard = {
  key: string;
  label: string;
  value: number;
};

export type DashboardOverviewTrendPoint = {
  date: string;
  addCustomerNum: number;
};

export type DashboardOverviewLimitation = {
  provider: string;
  code: string;
  message: string;
};

export type DashboardOverviewSummary = {
  customer: number;
  lead: number;
  contact: number;
  opportunity: number;
  won: number;
  order: number;
  behavior: number;
  employee: number;
};

export type DashboardOverview = {
  cards: readonly DashboardOverviewCard[];
  trend: readonly DashboardOverviewTrendPoint[];
  summary: DashboardOverviewSummary;
  limitations: readonly DashboardOverviewLimitation[];
  updatedAt: string;
  page: number;
  pageSize: number;
  total: number;
};

export type DashboardOverviewQuery = {
  corpId: string;
  startDate: string;
  endDate: string;
  employeeIds: readonly string[];
  departmentIds: readonly string[];
  page: number;
  pageSize: number;
};

export type DashboardOverviewApi = {
  load(input: DashboardOverviewQuery): Promise<DashboardOverview>;
  exportCsv(input: DashboardOverviewQuery): Promise<Blob>;
};

type ApiClient = {
  request(input: RequestInfo | URL, init?: RequestInit): Promise<unknown>;
};

type Row = Record<string, unknown>;

function isRecord(value: unknown): value is Row {
  return typeof value === 'object' && value !== null;
}

function isFiniteNumber(value: unknown): value is number {
  return typeof value === 'number' && Number.isFinite(value);
}

const timezone = 'Asia/Shanghai';

const zonedStart = (date: string) => `${date}T00:00:00+08:00`;

const summaryKeys = [
  'customer', 'lead', 'contact', 'opportunity', 'won', 'order', 'behavior', 'employee',
] as const satisfies readonly (keyof DashboardOverviewSummary)[];

function parseSummary(value: unknown): DashboardOverviewSummary {
  const source = isRecord(value) ? value : {};
  return Object.fromEntries(summaryKeys.map((key) => [key, isFiniteNumber(source[key]) ? source[key] : 0])) as DashboardOverviewSummary;
}

function parseLimitations(value: unknown): DashboardOverviewLimitation[] {
  if (!Array.isArray(value)) return [];
  return value.filter((item): item is DashboardOverviewLimitation =>
    isRecord(item) && typeof item.provider === 'string' && typeof item.code === 'string' && typeof item.message === 'string');
}

function parseOverview(value: unknown): DashboardOverview {
  if (!isRecord(value) || !isRecord(value.summary)) {
    throw new Error('数据概览接口返回了无效数据');
  }
  const summary = parseSummary(value.summary);
  const cards: DashboardOverviewCard[] = [
    { key: 'customer', label: '客户总数', value: summary.customer },
    { key: 'lead', label: '线索总数', value: summary.lead },
    { key: 'order', label: '订单总数', value: summary.order },
    { key: 'behavior', label: '行为事件', value: summary.behavior },
  ];
  const series = Array.isArray(value.series) ? value.series : [];
  const trend = series
    .filter((point): point is Row & { at: string; value: number } =>
      isRecord(point) && typeof point.at === 'string' && isFiniteNumber(point.value))
    .map((point) => ({ date: point.at.slice(0, 10), addCustomerNum: point.value }));
  const pagination = isRecord(value.pagination) ? value.pagination : {};
  const freshness = isRecord(value.freshness) ? value.freshness : {};
  // Zero time.Time marshals as 0001-01-01T00:00:00Z; treat sentinel/missing
  // values as "no freshness timestamp" instead of rendering 0001-01-01.
  const dataThrough = typeof freshness.dataThrough === 'string' && /^20\d\d-/.test(freshness.dataThrough)
    ? freshness.dataThrough
    : '';
  const updatedAt = dataThrough === '' ? '' : dataThrough.replace('T', ' ').replace('Z', '').slice(0, 19);
  return {
    cards,
    trend,
    summary,
    limitations: parseLimitations(value.limitations),
    updatedAt,
    page: isFiniteNumber(pagination.page) ? pagination.page : 1,
    pageSize: isFiniteNumber(pagination.pageSize) ? pagination.pageSize : 20,
    total: isFiniteNumber(pagination.total) ? pagination.total : trend.length,
  };
}

function serializeQuery(input: DashboardOverviewQuery): string {
  const query = new URLSearchParams({
    corpId: input.corpId,
    timezone,
    startAt: zonedStart(input.startDate),
    endAt: zonedStart(input.endDate),
    page: String(input.page),
    pageSize: String(input.pageSize),
  });
  for (const employeeId of input.employeeIds) query.append('employeeIds', employeeId);
  for (const departmentId of input.departmentIds) query.append('departmentIds', departmentId);
  return query.toString();
}

function csvCell(value: string | number): string {
  return `"${String(value).replaceAll('"', '""')}"`;
}

function overviewCsv(overview: DashboardOverview): Blob {
  const lines = [
    ['日期', '新增客户'],
    ...overview.trend.map((point) => [point.date, point.addCustomerNum]),
  ].map((row) => row.map(csvCell).join(','));
  return new Blob([`\ufeff${lines.join('\n')}`], { type: 'text/csv;charset=utf-8' });
}

export function createDashboardOverviewApi(client: ApiClient): DashboardOverviewApi {
  return {
    async load(input) {
      const value = await client.request(`/reports/overview?${serializeQuery(input)}`);
      return parseOverview(value);
    },
    async exportCsv(input) {
      return overviewCsv(await this.load(input));
    },
  };
}
```

- [ ] **Step 4: 实现 dashboard-overview-page.tsx**

替换 `FilterDraft` 与默认区间：

```ts
type OverviewRange = { from: string; to: string };
type FilterDraft = OverviewRange & { employeeIds: string; departmentIds: string };

const defaultPageSize = 20;
const enterpriseTimeZone = 'Asia/Shanghai';
```

把 `defaultRange()` 替换为：

```ts
function defaultRange(): OverviewRange {
  const now = new Date();
  const parts = new Intl.DateTimeFormat('en-US', {
    timeZone: enterpriseTimeZone, year: 'numeric', month: '2-digit', day: '2-digit',
  }).formatToParts(now);
  const values = Object.fromEntries(parts.map((part) => [part.type, part.value]));
  const year = Number(values.year ?? 0);
  const month = Number(values.month ?? 0);
  return {
    from: `${year}-${String(month).padStart(2, '0')}-01`,
    to: `${month === 12 ? year + 1 : year}-${String(month === 12 ? 1 : month + 1).padStart(2, '0')}-01`,
  };
}
```

把 `filtersFromSearch` 的返回值去掉 `period` 字段；把 `overviewSearch` 去掉 `period` 参数与 `next.set('period', ...)` 逻辑，并在构造 `next` 后追加 `next.delete('period')`（`updateSearch` 会保留未提及的旧参数，必须显式清除 URL 中残留的 `period`）；把 `input` 的 `useMemo` 去掉 `period` 字段。

日期范围校验同步调整：默认区间“本月 1 日 → 下月 1 日”在 31 天月份跨度为 31 天，现有 `calendarDaySpan(draft) > 30` 会误伤默认区间；把 `applyFilters` 中的校验改为 `calendarDaySpan(draft) > 31`（提示文案保持“日期范围最多为 31 天”，即允许 31 天内的整月区间）。

把 `TrendChart` 替换为单序列版本：

```ts
function trendMaximum(points: readonly DashboardOverviewTrendPoint[]): number {
  return Math.max(1, ...points.map((point) => point.addCustomerNum));
}

function TrendChart({ points }: { points: readonly DashboardOverviewTrendPoint[] }) {
  const maximum = trendMaximum(points);
  return <div className="dashboard-overview-chart" role="img" aria-label="企业趋势图">
    {points.map((point) => <div className="dashboard-overview-chart-column" key={point.date}>
      <div className="dashboard-overview-bars">
        <span aria-label={`新增客户 ${point.addCustomerNum}`} className="dashboard-overview-bar dashboard-overview-bar-primary" style={{ height: `${Math.max(4, (point.addCustomerNum / maximum) * 100)}%` }} title={`新增客户 ${point.addCustomerNum}`} />
      </div>
      <span>{point.date}</span>
    </div>)}
  </div>;
}
```

把 `BusinessDashboard` 整体替换为：

```tsx
function BusinessDashboard({ data, page, pageSize, searchParams, setSearchParams, current }: {
  data: Awaited<ReturnType<DashboardOverviewApi['load']>>;
  page: number;
  pageSize: number;
  searchParams: URLSearchParams;
  setSearchParams: ReturnType<typeof useSearchParams>[1];
  current: FilterDraft;
}) {
  const total = data.total ?? data.trend.length;
  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  const archiveUnavailable = data.limitations.some((item) => item.provider === 'conversation_archive');

  return <div className="overview-dashboard">
    <section className="overview-module dashboard-data-card">
      <ModuleHeader title="经营概览" description="客户、线索、订单与行为事件，与数据报表同口径" extra={<span className="overview-update">更新于 {data.updatedAt || '--'}</span>} />
      <div className="overview-metric-grid dashboard-stat-grid">
        <MetricCard label={data.cards[0]?.label ?? '客户总数'} value={data.summary.customer} note="同客户分析口径（联系人+分配关系）" />
        <MetricCard label={data.cards[1]?.label ?? '线索总数'} value={data.summary.lead} note="转化漏斗起始阶段" tone="green" />
        <MetricCard label={data.cards[2]?.label ?? '订单总数'} value={data.summary.order} note="won/completed/paid 口径" tone="violet" />
        <MetricCard label={data.cards[3]?.label ?? '行为事件'} value={data.summary.behavior} note="订单与设置审计事件" tone="orange" />
      </div>
    </section>

    <section className="overview-module dashboard-data-card">
      <ModuleHeader title="AI 洞察" description="AI 能力未接入，配置后展示会话风险与情绪信号" extra={<Link className="overview-link-button" to="/ai-setting/ai-knowledge-base">前往配置</Link>} />
      <EmptyVisual text="AI 能力未接入，请在 AI 设置中配置后查看" />
    </section>

    <div className="overview-two-column">
      <section className="overview-module dashboard-data-card">
        <ModuleHeader title="客户增长趋势" description="按天统计新增联系人，同客户分析趋势" extra={<span className="overview-scope-chip">数据来源：SCRM</span>} />
        {data.trend.length === 0 ? <EmptyVisual /> : <TrendChart points={data.trend} />}
      </section>

      <section className="overview-module dashboard-data-card">
        <ModuleHeader title="会话归档" description="已记录会话的员工数量（会话存档底座）" extra={<span className="overview-scope-chip">conversation_archive</span>} />
        {archiveUnavailable
          ? <EmptyVisual text="会话归档未接入，配置企业微信会话存档后展示" />
          : <div className="overview-split-stats"><div><h3>存档员工</h3><p><strong>{data.summary.employee}</strong><span>已记录会话</span></p></div></div>}
      </section>
    </div>

    <section className="overview-module overview-detail-table dashboard-data-card">
      <ModuleHeader title="经营趋势明细" description="与当前筛选和导出范围保持一致" />
      {data.trend.length === 0 ? <EmptyVisual /> : <div className="dashboard-table-scroll"><table><thead><tr><th>日期</th><th>新增客户</th></tr></thead><tbody>{data.trend.map((point) => <tr key={point.date}><td>{point.date}</td><td>{point.addCustomerNum}</td></tr>)}</tbody></table></div>}
      <footer className="dashboard-table-actions"><span>共 {total} 条，第 {page}/{totalPages} 页</span><div><button disabled={page <= 1} onClick={() => setSearchParams(overviewSearch(searchParams, current, page - 1, pageSize))} type="button">上一页</button><button disabled={page >= totalPages} onClick={() => setSearchParams(overviewSearch(searchParams, current, page + 1, pageSize))} type="button">下一页</button></div></footer>
    </section>
  </div>;
}
```

把 `DashboardOverviewPage` 中：

1. `useState<FilterDraft>(current)`、`useEffect` 同步逻辑保持不变（`FilterDraft` 已去掉 period）。
2. 筛选栏删除“趋势周期” `<label>`。
3. 页头描述改为 `查看当前企业客户、线索、订单与行为数据（与数据报表同口径）。`
4. `BusinessDashboard` 调用处删除 `period` 与 `onRefresh` 参数。

顶部筛选栏的 `查询/刷新/导出 CSV` 按钮与高级筛选（员工 ID/部门 ID）保持不变。

- [ ] **Step 5: 运行测试确认通过**

Run: `pnpm --filter @mochat/dashboard test -- dashboard-overview`
Expected: PASS。

另跑：`pnpm --filter @mochat/dashboard typecheck`，Expected: PASS。

- [ ] **Step 6: 提交**

```bash
git add web/apps/dashboard/src/features/dashboard-overview
git commit -m "feat(dashboard): switch overview to unified reporting data source"
```

---

### Task 3（主代理执行，不派发子代理）：验收数据清理

**目标：** 执行 `deploy/standalone/acceptance/cleanup_phase3_final.sql`（仅删除 `P36-ACCEPT-*` / `P36-ARCH-%` 验收数据与 `mochat_go_ai_analysis` 中的 AI 验收行），并保留可恢复备份。

- [ ] Step 1：确认容器 `mochat-go-desktop-mysql-1` healthy；使用 `MOCHAT_MYSQL_DSN='mochat:mochat_pass@tcp(127.0.0.1:13316)/mochat?parseTime=true&loc=Local'`。
- [ ] Step 2：备份目标行到 `D:\workspace\mochat-go\output\acceptance-cleanup-20260808\`（`mochat_go_ai_analysis`、`mochat_go_audio_objects`、`mc_work_message_1` 按 SQL 条件导出 CSV/INSERT）。
- [ ] Step 3：执行 SQL；执行前后分别记录目标计数，验证删除后计数为 0。
- [ ] Step 4：写清理证据记录（含备份路径、前后计数、执行时间），附到最终报告。

---

### Task 4（主代理执行）：文档同步

- [ ] Step 1：更新 `PROJECT_PROGRESS.zh-CN.md`，新增 2026-08-08 数据口径统一小节。
- [ ] Step 2：更新 `docs/reviews/2026-08-08-dashboard-dataflow-scenario-analysis.zh-CN.md`：把 D1/D2/W8 标记为“已实施（2026-08-08）”，并在附注中说明 `AllowedEmployeeIDs` 传播为已知限制。

---

### Task 5（子代理执行）：AI 入口收敛为单一“AI 能力中心”（D3）

> 背景：AI 洞察 5 页（`/ai-insight/session-analysis|smart-analysis|emotion|employee-score|communication-keyword`）当前全部受限（`AI_INSIGHT_ENABLED=0`），但菜单“AI 洞察”组展示 5 个入口，用户逐页点开才发现不可用。本轮将侧边栏该组收敛为 1 个“AI 能力中心”入口，路由与直接访问保留。

**Files:**
- Add: `web/apps/dashboard/src/features/ai-insight/ai-insight-hub-page.tsx`
- Add: `web/apps/dashboard/src/features/ai-insight/ai-insight-hub-page.test.tsx`
- Modify: `web/apps/dashboard/src/benchmark/page-registry.tsx`（注册 `/ai-insight/overview`）
- Modify: `web/apps/dashboard/src/main.tsx`（`knownRoutes` 增加 `/ai-insight/overview`）
- Modify: `web/apps/dashboard/src/layout/yuanhu-navigation.ts`（组收敛 override）
- Modify: `web/apps/dashboard/src/layout/dashboard-layout.tsx`（传入 override）
- Modify: `web/apps/dashboard/src/layout/yuanhu-navigation.test.ts`（新增收敛用例）

**设计决策：**
1. **不新增后端状态接口**：Hub 页复用现有 `createAiInsightApi().read('session-analysis', corpId)` 判断 `capability`；`ready` 时展示 5 个子能力卡片（可点击跳转），`limited/unavailable` 时展示“AI 能力未接入”引导 + 5 个禁用能力卡片 + “前往接入”链接（`/ai-setting/ai-knowledge-base`）。
2. **侧边栏收敛是前端导航层 override，不修改 benchmark manifest**：`buildYuanhuNavigation(access, manifest, groupOverrides?)`，`groupOverrides` 形如 `{ groupId: 'ai-insight', title: 'AI 能力中心', path: '/ai-insight/overview' }`；当该组存在任意 allowed 页面时，仅渲染 1 个条目，路径为 Hub 页。其余组行为不变。
3. **5 个原路由保持注册与可直达**：直接 URL 仍可用（受限态原样展示），为后续 AI 能力开放保留兼容。
4. **风险预警组（`/ai-insight/v2/*`）不收拢**：它们是 legacy 业务页（敏感词/拦截/超时等），不依赖 AI Provider。
5. Hub 页展示文案与现有受限态术语一致（“AI 能力未接入 / 当前未接入可用的 AI 分析 Provider，暂无分析结果”），5 个能力卡片标题与 `aiInsightPageConfigs` 一致。

- [ ] Step 1: 写失败测试（yuanhu-navigation override + Hub 页受限/就绪两种渲染）
- [ ] Step 2: 运行测试确认失败
- [ ] Step 3: 实现 `yuanhu-navigation.ts` override 与 `dashboard-layout.tsx` 传参
- [ ] Step 4: 实现 `ai-insight-hub-page.tsx` 并注册路由（page-registry + main.tsx knownRoutes）
- [ ] Step 5: 运行 `pnpm --filter @mochat/dashboard test` 与 `pnpm --filter @mochat/dashboard typecheck` 确认通过
- [ ] Step 6: 提交 `feat(dashboard): converge AI insight entries into a single hub`

---

### Task 6（主代理执行）：统一验证、部署与文档收尾

- [ ] Step 1: 复跑 Go 单测/全量、前端单测/typecheck/build（验证矩阵）
- [ ] Step 2: 重建 Docker 栈并做浏览器验收：`/index` 客户总数 = `/data/customer` 主指标；侧边栏“AI 洞察”仅 1 个“AI 能力中心”入口；受限态 Hub 页正常
- [ ] Step 3: 更新 `PROJECT_PROGRESS.zh-CN.md` 与 `docs/reviews/2026-08-08-dashboard-dataflow-scenario-analysis.zh-CN.md`（D1/D2/D3/W8 标记已实施，含验收证据路径）
- [ ] Step 4: 提交文档与收尾改动

## 验证矩阵（全部通过才可交付）

| 检查项 | 命令 | 预期 |
| --- | --- | --- |
| Go 单测 | `go test ./internal/modules/reporting/... ./cmd/mochat-go/... -count=1` | PASS |
| Go 全量 | `go test ./... -count=1` | PASS |
| 前端单测 | `pnpm --filter @mochat/dashboard test -- dashboard-overview` | PASS |
| 前端全量 | `pnpm --filter @mochat/dashboard test` | PASS |
| 类型检查 | `pnpm --filter @mochat/dashboard typecheck` | PASS |
| 生产构建 | `pnpm --filter @mochat/dashboard build` | PASS |
| Docker 部署 | `docker compose -p mochat-go-desktop -f deploy/standalone/docker-compose.yml --profile app build app` 后 `up -d`，`/readyz` 200 | 容器 healthy |
| 浏览器验收 | 登录 `13800000000`，访问 `/index` | 概览客户数 = `/data/customer` 主指标；无硬编码 0；会话归档显示未接入空态 |
| 浏览器验收（AI 收敛） | 登录后查看侧边栏 | “AI 洞察”组仅 1 个“AI 能力中心”入口；Hub 页受限态引导 + 配置入口正常 |

## 已知限制（本轮明确不修）

- `reportingPrincipalResolver` 未填充 `AllowedEmployeeIDs`，员工级数据权限（self/department）在报表与概览中暂未生效；菜单权限仍生效。此项单独立项。
- legacy `/corpData/index` 保留但不再被 Dashboard 概览页使用；SaaS/旧客户端兼容性不受影响。
