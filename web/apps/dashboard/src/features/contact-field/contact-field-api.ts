export type ContactFieldWrite = {
  label: string;
  type: number;
  options: string[];
  order: number;
  status: number;
};
export type ContactFieldItem = ContactFieldWrite & {
  id: number;
  name: string;
  typeText: string;
  isSys: number;
};
export type ContactFieldListResult = {
  list: ContactFieldItem[];
  page: { page: number; perPage: number; total: number; totalPage: number };
};
export type ContactFieldBatchInput = {
  update: Array<ContactFieldWrite & { id: number }>;
  destroy: number[];
};

type ApiClient = { request(input: RequestInfo | URL, init?: RequestInit): Promise<unknown> };
const jsonRequest = (method: string, body: unknown): RequestInit => ({
  method, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body),
});

export function createContactFieldApi(client: ApiClient) {
  return {
    list(input: { status: number; page: number; perPage: number }): Promise<ContactFieldListResult> {
      const query = new URLSearchParams({
        status: String(input.status), page: String(input.page), perPage: String(input.perPage),
      });
      return client.request(`/contactField/index?${query.toString()}`) as Promise<ContactFieldListResult>;
    },
    async create(input: ContactFieldWrite) {
      await client.request('/contactField/store', jsonRequest('POST', input));
    },
    async update(input: ContactFieldWrite & { id: number }) {
      await client.request('/contactField/update', jsonRequest('PUT', input));
    },
    async updateStatus(id: number, status: number) {
      await client.request('/contactField/statusUpdate', jsonRequest('PUT', { id, status }));
    },
    async remove(id: number) {
      await client.request('/contactField/destroy', jsonRequest('DELETE', { id }));
    },
    async batchUpdate(input: ContactFieldBatchInput) {
      await client.request('/contactField/batchUpdate', jsonRequest('PUT', input));
    },
  };
}
