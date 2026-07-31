export type Lead = { id: string; businessKey: string; name: string; source: string; status: string; version: number };
export type LeadPage = { items: Lead[]; nextCursor: string };
type Client = { request<T = unknown>(input: RequestInfo | URL, init?: RequestInit): Promise<T> };

export type LeadApi = {
  list(input: { cursor?: string; pageSize?: number }): Promise<LeadPage>;
  create(input: { businessKey: string; name: string; source: 'manual' | 'import' | 'wecom' }): Promise<Lead>;
};

export function createLeadApi(client: Client): LeadApi {
  return {
    async list(input) {
      const query = new URLSearchParams();
      if (input.cursor) query.set('cursor', input.cursor);
      query.set('pageSize', String(input.pageSize ?? 20));
      return client.request(`/scrm/leads?${query.toString()}`) as Promise<LeadPage>;
    },
    async create(input) {
      return client.request('/scrm/leads', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(input) }) as Promise<Lead>;
    },
  };
}
