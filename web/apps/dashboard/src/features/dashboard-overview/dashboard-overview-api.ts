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
};

export type DashboardOverviewApi = {
  load(input: {
    corpId: string;
    from: string;
    to: string;
  }): Promise<DashboardOverview>;
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
  return { cards, trend, updatedAt: value.updatedAt };
}

export function createDashboardOverviewApi(client: ApiClient): DashboardOverviewApi {
  return {
    async load(input) {
      const query = new URLSearchParams({
        corpId: input.corpId,
        from: input.from,
        to: input.to,
      });
      const value = await client.request(`/corpData/index?${query.toString()}`);
      return parseDashboardOverview(value);
    },
  };
}
