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
  request(input: RequestInfo | URL, init?: RequestInit): Promise<DashboardOverview>;
};

export function createDashboardOverviewApi(client: ApiClient): DashboardOverviewApi {
  return {
    load(input) {
      const query = new URLSearchParams({
        corpId: input.corpId,
        from: input.from,
        to: input.to,
      });
      return client.request(`/corpData/index?${query.toString()}`);
    },
  };
}
