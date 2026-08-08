export type LeadStatus = 'new' | 'qualified' | 'converted' | 'discarded';
export type LeadSource = 'manual' | 'import' | 'wecom';
export type Lead = { id: string; businessKey: string; name: string; phone: string; source: LeadSource; status: LeadStatus; ownerId: number | null; convertedContactId: string; discardReason: string; version: number };
export type LeadPage = { items: Lead[]; nextCursor: string };
export type LeadTarget = { id: string; version: number };
export type LeadMutationResult = { id: string; status: 'succeeded' | 'failed'; errorCode: string; lead?: Lead };
export type LeadOwnerOption = { id: number; name: string };
type Client = { request<T = unknown>(input: RequestInfo | URL, init?: RequestInit): Promise<T> };

export type LeadListInput = { corpId: number; keyword?: string; statuses?: LeadStatus[]; sources?: LeadSource[]; ownerIds?: number[]; createdFrom?: string; createdTo?: string; cursor?: string; pageSize?: number };
export type LeadApi = {
  listOwnerOptions(): Promise<LeadOwnerOption[]>;
  list(input: LeadListInput): Promise<LeadPage>;
  create(input: { corpId: number; businessKey: string; name: string; phone: string; source: LeadSource }): Promise<Lead>;
  findDuplicates(input: { corpId: number; businessKey?: string; phone?: string }): Promise<{ items: Lead[] }>;
  assign(input: { corpId: number; ownerId: number; targets: LeadTarget[] }): Promise<{ results: LeadMutationResult[] }>;
  transition(input: { corpId: number; id: string; toStatus: LeadStatus; version: number; discardReason: string }): Promise<Lead>;
};

const jsonRequest = (body: unknown): RequestInit => ({ method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });

export function createLeadApi(client: Client): LeadApi {
  return {
    async listOwnerOptions() {
      const result = await client.request<{ list?: Array<{ id?: number; name?: string }> }>('/workEmployee/index?page=1&perPage=200');
      return (result.list ?? [])
        .filter((item): item is { id: number; name?: string } => Number(item.id) > 0)
        .map((item) => ({ id: item.id, name: item.name?.trim() || `员工 ${item.id}` }));
    },
    async list(input) {
      const query = new URLSearchParams({ corpId: String(input.corpId) });
      if (input.keyword?.trim()) query.set('keyword', input.keyword.trim());
      input.statuses?.forEach((status) => query.append('status', status));
      input.sources?.forEach((source) => query.append('source', source));
      input.ownerIds?.forEach((ownerId) => query.append('ownerId', String(ownerId)));
      if (input.createdFrom) query.set('createdFrom', input.createdFrom);
      if (input.createdTo) query.set('createdTo', input.createdTo);
      if (input.cursor) query.set('cursor', input.cursor);
      query.set('pageSize', String(input.pageSize ?? 20));
      return client.request(`/scrm/leads?${query.toString()}`);
    },
    async create(input) { return client.request('/scrm/leads', jsonRequest(input)); },
    async findDuplicates(input) {
      const query = new URLSearchParams({ corpId: String(input.corpId) });
      if (input.businessKey?.trim()) query.set('businessKey', input.businessKey.trim());
      if (input.phone?.trim()) query.set('phone', input.phone.trim());
      return client.request(`/scrm/leads/duplicates?${query.toString()}`);
    },
    async assign(input) { return client.request('/scrm/leads/assignments', jsonRequest(input)); },
    async transition(input) { return client.request('/scrm/leads/transition', jsonRequest(input)); },
  };
}
