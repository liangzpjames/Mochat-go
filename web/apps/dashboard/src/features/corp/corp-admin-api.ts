export type CorpListInput = {
  corpId: string;
  corpName: string;
  page: number;
  perPage: number;
};

export type CorpListItem = {
  corpId: number;
  corpName: string;
  wxCorpId: string;
  createdAt: string;
};

export type CorpListResult = {
  list: CorpListItem[];
  page: {
    perPage: number;
    total: number;
    totalPage: number;
  };
};

export type CorpDetail = Omit<CorpListItem, 'createdAt'> & {
  employeeSecret: string;
  contactSecret: string;
  eventCallback: string;
  token: string;
  encodingAesKey: string;
  socialCode?: string;
  tenantId?: number;
};

export type CorpWriteInput = {
  corpName: string;
  wxCorpId: string;
  employeeSecret: string;
  contactSecret: string;
};

export type CorpUpdateInput = CorpWriteInput & { corpId: number };

type ApiClient = {
  request(input: RequestInfo | URL, init?: RequestInit): Promise<unknown>;
};

export function createCorpAdminApi(client: ApiClient) {
  return {
    list(input: CorpListInput): Promise<CorpListResult> {
      const query = new URLSearchParams({
        corpName: input.corpName,
        page: String(input.page),
        perPage: String(input.perPage),
      });
      return client.request(`/corp/index?${query.toString()}`) as Promise<CorpListResult>;
    },
    show(corpId: number): Promise<CorpDetail> {
      return client.request(`/corp/show?corpId=${corpId}`) as Promise<CorpDetail>;
    },
    async create(input: CorpWriteInput): Promise<void> {
      await client.request('/corp/store', jsonRequest('POST', input));
    },
    async update(input: CorpUpdateInput): Promise<void> {
      await client.request('/corp/update', jsonRequest('PUT', input));
    },
  };
}

function jsonRequest(method: string, body: unknown): RequestInit {
  return {
    method,
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  };
}
