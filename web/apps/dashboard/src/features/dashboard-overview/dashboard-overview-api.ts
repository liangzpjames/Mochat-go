export type DashboardOverviewCard = {
  key: string;
  label: string;
  value: number;
};

export type DashboardOverviewTrendPoint = {
  date: string;
  addContactNum: number;
  addIntoRoomNum: number;
  lossContactNum: number;
  quitRoomNum: number;
};

export type DashboardOverview = {
  cards: readonly DashboardOverviewCard[];
  trend: readonly DashboardOverviewTrendPoint[];
  updatedAt: string;
  page?: number;
  pageSize?: number;
  total?: number;
};

export type DashboardOverviewQuery = {
  corpId: string;
  startDate: string;
  endDate: string;
  employeeIds: readonly string[];
  departmentIds: readonly string[];
  period: 'day' | 'week' | 'month';
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

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}

function isFiniteNumber(value: unknown): value is number {
  return typeof value === 'number' && Number.isFinite(value);
}

function parseDashboardOverview(value: unknown): DashboardOverview {
  if (!isRecord(value) || !Array.isArray(value.cards) || !Array.isArray(value.trend)
    || typeof value.updatedAt !== 'string') {
    throw new Error('数据概览接口尚未启用');
  }
  const cards = value.cards.filter((card): card is DashboardOverviewCard =>
    isRecord(card)
    && typeof card.key === 'string'
    && typeof card.label === 'string'
    && isFiniteNumber(card.value));
  const trend = value.trend.filter((point): point is DashboardOverviewTrendPoint =>
    isRecord(point)
    && typeof point.date === 'string'
    && isFiniteNumber(point.addContactNum)
    && isFiniteNumber(point.addIntoRoomNum)
    && isFiniteNumber(point.lossContactNum)
    && isFiniteNumber(point.quitRoomNum));
  if (cards.length !== value.cards.length || trend.length !== value.trend.length) {
    throw new Error('数据概览接口返回了无效数据');
  }
  const page = isFiniteNumber(value.page) ? value.page : 1;
  const pageSize = isFiniteNumber(value.pageSize) ? value.pageSize : trend.length;
  const total = isFiniteNumber(value.total) ? value.total : trend.length;
  return { cards, trend, updatedAt: value.updatedAt, page, pageSize, total };
}

function serializeQuery(input: DashboardOverviewQuery): string {
  const query = new URLSearchParams({
    corpId: input.corpId,
    startDate: input.startDate,
    endDate: input.endDate,
  });
	if (input.employeeIds.length === 0) query.append('employeeIds', '');
	for (const employeeId of input.employeeIds) query.append('employeeIds', employeeId);
	if (input.departmentIds.length === 0) query.append('departmentIds', '');
	for (const departmentId of input.departmentIds) query.append('departmentIds', departmentId);
  query.set('period', input.period);
  query.set('page', String(input.page));
  query.set('pageSize', String(input.pageSize));
  return query.toString();
}

function csvCell(value: string | number): string {
  return `"${String(value).replaceAll('"', '""')}"`;
}

function overviewCsv(overview: DashboardOverview): Blob {
  const lines = [
    ['日期', '新增客户', '新增入群', '流失客户', '退出群聊'],
    ...overview.trend.map((point) => [
      point.date,
      point.addContactNum,
      point.addIntoRoomNum,
      point.lossContactNum,
      point.quitRoomNum,
    ]),
  ].map((row) => row.map(csvCell).join(','));
  return new Blob([`\ufeff${lines.join('\n')}`], { type: 'text/csv;charset=utf-8' });
}

export function createDashboardOverviewApi(client: ApiClient): DashboardOverviewApi {
  return {
    async load(input) {
      const value = await client.request(`/corpData/index?${serializeQuery(input)}`);
      return parseDashboardOverview(value);
    },
    async exportCsv(input) {
      return overviewCsv(await this.load(input));
    },
  };
}
