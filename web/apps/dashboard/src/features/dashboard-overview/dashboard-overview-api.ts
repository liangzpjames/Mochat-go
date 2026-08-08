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
  });
  for (const employeeId of input.employeeIds) query.append('employeeIds', employeeId);
  for (const departmentId of input.departmentIds) query.append('departmentIds', departmentId);
  query.set('page', String(input.page));
  query.set('pageSize', String(input.pageSize));
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
