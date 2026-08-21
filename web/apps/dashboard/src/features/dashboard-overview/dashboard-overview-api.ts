export type DashboardOverviewCard = {
  key: string;
  label: string;
  value: number | null;
};

export type DashboardOverviewTrendPoint = {
  date: string;
  addCustomerNum: number;
};

export type DashboardOverviewAIInsight = {
  capability: string;
  provider: string;
  summary: string;
  generatedAt: string;
};

export type DashboardOverviewAIMetrics = {
  analysisCount: number | null;
  employeeNegativeEmotion: number | null;
  customerNegativeEmotion: number | null;
  riskBehavior: number | null;
  sensitiveWords: number | null;
};

export type DashboardOverviewQuality = {
  sensitiveWords: number | null;
  riskBehavior: number | null;
  customerLoss: number | null;
  timeoutWarning: number | null;
  trend: readonly DashboardOverviewQualityTrendPoint[];
};

export type DashboardOverviewQualityTrendPoint = {
  date: string;
  sensitiveWords: number | null;
  riskBehavior: number | null;
  customerLoss: number | null;
  timeoutWarning: number | null;
};

export type DashboardOverviewEmployeeRankingItem = {
  employeeId: number;
  employeeName: string;
  sessions: number;
  messages: number;
};

export type DashboardOverviewTrajectoryItem = {
  id: string;
  targetType: string;
  targetId: string;
  employeeName: string;
  messageCount: number;
  latestAt: string;
};

export type ConversationGroupStats = {
  sessions: number | null;
  employeeMessages: number | null;
  customerMessages: number | null;
};

export type ConversationTrendPoint = {
  date: string;
  customerSessions: number | null;
  customerEmployeeMessages: number | null;
  customerCustomerMessages: number | null;
  roomSessions: number | null;
  roomEmployeeMessages: number | null;
  roomCustomerMessages: number | null;
};

export type DashboardOverviewConversation = {
  customer: ConversationGroupStats;
  room: ConversationGroupStats;
  trend: readonly ConversationTrendPoint[];
};

export type DashboardOverviewLimitation = {
  provider: string;
  code: string;
  message: string;
};

export type DashboardOverviewSummary = {
  customer: number | null;
  lead: number | null;
  contact: number | null;
  opportunity: number | null;
  won: number | null;
  order: number | null;
  behavior: number | null;
  employee: number | null;
};

export type DashboardOverview = {
  cards: readonly DashboardOverviewCard[];
  trend: readonly DashboardOverviewTrendPoint[];
  summary: DashboardOverviewSummary;
  limitations: readonly DashboardOverviewLimitation[];
  aiInsight?: DashboardOverviewAIInsight;
  aiMetrics?: DashboardOverviewAIMetrics;
  conversation?: DashboardOverviewConversation;
  quality?: DashboardOverviewQuality;
  employeeRanking?: readonly DashboardOverviewEmployeeRankingItem[];
  trajectory?: readonly DashboardOverviewTrajectoryItem[];
  updatedAt: string;
  page: number;
  pageSize: number;
  total: number;
};

export type DashboardOverviewQuery = {
  corpId: string;
  startDate: string;
  endDate: string;
  trendStartDate: string;
  trendEndDate: string;
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

function nullableNumber(value: unknown): number | null {
  return isFiniteNumber(value) ? value : null;
}

const timezone = 'Asia/Shanghai';

const zonedStart = (date: string) => `${date}T00:00:00+08:00`;

const summaryKeys = [
  'customer', 'lead', 'contact', 'opportunity', 'won', 'order', 'behavior', 'employee',
] as const satisfies readonly (keyof DashboardOverviewSummary)[];

function formatInZone(raw: string): string {
  const parsed = new Date(raw);
  if (Number.isNaN(parsed.getTime())) {
    return raw.replace('T', ' ').replace('Z', '').slice(0, 19);
  }
  const parts = new Intl.DateTimeFormat('en-US', {
    timeZone: timezone,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hourCycle: 'h23',
  }).formatToParts(parsed);
  const value = (type: string) => parts.find((part) => part.type === type)?.value ?? '';
  return `${value('year')}-${value('month')}-${value('day')} ${value('hour')}:${value('minute')}:${value('second')}`;
}

function parseSummary(value: unknown): DashboardOverviewSummary {
  const source = isRecord(value) ? value : {};
  return Object.fromEntries(summaryKeys.map((key) => [key, nullableNumber(source[key])])) as DashboardOverviewSummary;
}

function parseLimitations(value: unknown): DashboardOverviewLimitation[] {
  if (!Array.isArray(value)) return [];
  return value.filter((item): item is DashboardOverviewLimitation =>
    isRecord(item) && typeof item.provider === 'string' && typeof item.code === 'string' && typeof item.message === 'string');
}

function parseAIInsight(value: unknown): DashboardOverviewAIInsight | undefined {
  if (!isRecord(value) || typeof value.capability !== 'string' || value.capability === '') {
    return undefined;
  }
  return {
    capability: value.capability,
    provider: typeof value.provider === 'string' ? value.provider : '',
    summary: typeof value.summary === 'string' ? value.summary : '',
    generatedAt: typeof value.generatedAt === 'string' ? value.generatedAt : '',
  };
}

function parseAIMetrics(value: unknown): DashboardOverviewAIMetrics | undefined {
  if (!isRecord(value)) return undefined;
  return {
    analysisCount: nullableNumber(value.analysisCount),
    employeeNegativeEmotion: nullableNumber(value.employeeNegativeEmotion),
    customerNegativeEmotion: nullableNumber(value.customerNegativeEmotion),
    riskBehavior: nullableNumber(value.riskBehavior),
    sensitiveWords: nullableNumber(value.sensitiveWords),
  };
}

function parseQuality(value: unknown): DashboardOverviewQuality | undefined {
  if (!isRecord(value)) return undefined;
  const trend = Array.isArray(value.trend) ? value.trend.filter((point): point is Row =>
    isRecord(point) && typeof point.date === 'string' && point.date !== '').map((point) => ({
      date: point.date as string,
      sensitiveWords: nullableNumber(point.sensitiveWords),
      riskBehavior: nullableNumber(point.riskBehavior),
      customerLoss: nullableNumber(point.customerLoss),
      timeoutWarning: nullableNumber(point.timeoutWarning),
    })) : [];
  return {
    sensitiveWords: nullableNumber(value.sensitiveWords),
    riskBehavior: nullableNumber(value.riskBehavior),
    customerLoss: nullableNumber(value.customerLoss),
    timeoutWarning: nullableNumber(value.timeoutWarning),
    trend,
  };
}

function parseEmployeeRanking(value: unknown): DashboardOverviewEmployeeRankingItem[] {
  if (!Array.isArray(value)) return [];
  return value.filter(isRecord).map((row) => ({
    employeeId: isFiniteNumber(row.employeeId) ? row.employeeId : 0,
    employeeName: typeof row.employeeName === 'string' ? row.employeeName : '',
    sessions: isFiniteNumber(row.sessions) ? row.sessions : 0,
    messages: isFiniteNumber(row.messages) ? row.messages : 0,
  }));
}

function parseTrajectory(value: unknown): DashboardOverviewTrajectoryItem[] {
  if (!Array.isArray(value)) return [];
  return value.filter(isRecord).map((row) => ({
    id: typeof row.id === 'string' ? row.id : '',
    targetType: typeof row.targetType === 'string' ? row.targetType : '',
    targetId: typeof row.targetId === 'string' || typeof row.targetId === 'number' ? String(row.targetId) : '',
    employeeName: typeof row.employeeName === 'string' ? row.employeeName : '',
    messageCount: isFiniteNumber(row.messageCount) ? row.messageCount : 0,
    latestAt: typeof row.latestAt === 'string' ? row.latestAt : '',
  }));
}

function parseConversation(value: unknown): DashboardOverviewConversation | undefined {
  if (!isRecord(value)) return undefined;
  const group = (raw: unknown): ConversationGroupStats => {
    const source = isRecord(raw) ? raw : {};
    return {
      sessions: nullableNumber(source.sessions),
      employeeMessages: nullableNumber(source.employeeMessages),
      customerMessages: nullableNumber(source.customerMessages),
    };
  };
  const trend = Array.isArray(value.trend)
      ? value.trend.filter(isRecord).map((point) => ({
        date: typeof point.date === 'string' ? point.date : '',
        customerSessions: nullableNumber(point.customerSessions),
        customerEmployeeMessages: nullableNumber(point.customerEmployeeMessages),
        customerCustomerMessages: nullableNumber(point.customerCustomerMessages),
        roomSessions: nullableNumber(point.roomSessions),
        roomEmployeeMessages: nullableNumber(point.roomEmployeeMessages),
        roomCustomerMessages: nullableNumber(point.roomCustomerMessages),
      }))
    : [];
  return { customer: group(value.customer), room: group(value.room), trend };
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
  const updatedAt = dataThrough === '' ? '' : formatInZone(dataThrough);
  const parsed: DashboardOverview = {
    cards,
    trend,
    summary,
    limitations: parseLimitations(value.limitations),
    updatedAt,
    page: isFiniteNumber(pagination.page) ? pagination.page : 1,
    pageSize: isFiniteNumber(pagination.pageSize) ? pagination.pageSize : 20,
    total: isFiniteNumber(pagination.total) ? pagination.total : trend.length,
  };
  const aiInsight = parseAIInsight(value.aiInsight);
  if (aiInsight !== undefined) parsed.aiInsight = aiInsight;
  const conversation = parseConversation(value.conversation);
  if (conversation !== undefined) parsed.conversation = conversation;
  const aiMetrics = parseAIMetrics(value.aiMetrics);
  if (aiMetrics !== undefined) parsed.aiMetrics = aiMetrics;
  const quality = parseQuality(value.quality);
  if (quality !== undefined) parsed.quality = quality;
  parsed.employeeRanking = parseEmployeeRanking(value.employeeRanking);
  parsed.trajectory = parseTrajectory(value.trajectory);
  return parsed;
}

function serializeQuery(input: DashboardOverviewQuery): string {
  const query = new URLSearchParams({
    corpId: input.corpId,
    timezone,
    startAt: zonedStart(input.startDate),
    endAt: zonedStart(input.endDate),
  });
  if (
    typeof input.trendStartDate === 'string' && input.trendStartDate !== ''
    && typeof input.trendEndDate === 'string' && input.trendEndDate !== ''
  ) {
    query.set('trendStartAt', zonedStart(input.trendStartDate));
    query.set('trendEndAt', zonedStart(input.trendEndDate));
  }
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
