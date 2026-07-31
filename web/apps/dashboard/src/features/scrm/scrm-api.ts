export type Assignment = { id: string; contactId: string; ownerId: number | null; collaboratorIds: number[]; status: string; version: number };
export type AssignmentPage = { items: Assignment[]; nextCursor: string };
type Client = { request<T = unknown>(input: RequestInfo | URL, init?: RequestInit): Promise<T> };

export type ScrmApi = {
  listPublicPool(input: { corpId: number; cursor?: string; pageSize?: number }): Promise<AssignmentPage>;
  updateAssignment(input: { corpId: number; contactId: string; ownerId: number | null; collaboratorIds: number[]; version: number; idempotencyKey: string }): Promise<Assignment>;
  releaseToPublicPool(input: { corpId: number; contactId: string; version: number; idempotencyKey: string }): Promise<Assignment>;
  claimFromPublicPool(input: { corpId: number; contactId: string; version: number; idempotencyKey: string }): Promise<Assignment>;
};

const json = (body: unknown, idempotencyKey: string): RequestInit => ({ method: 'POST', headers: { 'Content-Type': 'application/json', 'Idempotency-Key': idempotencyKey }, body: JSON.stringify(body) });

export function createScrmApi(client: Client): ScrmApi {
  return {
    async listPublicPool(input) {
      const query = new URLSearchParams({ corpId: String(input.corpId), pageSize: String(input.pageSize ?? 20) });
      if (input.cursor) query.set('cursor', input.cursor);
      return client.request(`/dashboard/scrm/assignments?${query.toString()}`) as Promise<AssignmentPage>;
    },
    async updateAssignment(input) {
      return client.request('/dashboard/scrm/assignments', { ...json(input, input.idempotencyKey), method: 'PUT' }) as Promise<Assignment>;
    },
    async releaseToPublicPool(input) {
      return client.request('/dashboard/scrm/assignments/release', json(input, input.idempotencyKey)) as Promise<Assignment>;
    },
    async claimFromPublicPool(input) {
      return client.request('/dashboard/scrm/assignments/claim', json(input, input.idempotencyKey)) as Promise<Assignment>;
    },
  };
}
