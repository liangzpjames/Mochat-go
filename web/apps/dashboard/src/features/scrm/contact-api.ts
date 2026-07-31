export type Contact = { id: string; name: string; phone?: string };
type Client = { request<T = unknown>(input: RequestInfo | URL, init?: RequestInit): Promise<T> };

export type ContactApi = { listContacts(input: { corpId: number; keyword?: string }): Promise<{ items: Contact[] }> };

export function createContactApi(client: Client): ContactApi {
  return { async listContacts(input) {
    const query = new URLSearchParams({ corpId: String(input.corpId), page: '1', perPage: '20' });
    if (input.keyword) query.set('keyword', input.keyword);
    const payload = await client.request<{ list?: Contact[] }>(`/workContact/index?${query.toString()}`);
    return { items: payload.list ?? [] };
  } };
}
