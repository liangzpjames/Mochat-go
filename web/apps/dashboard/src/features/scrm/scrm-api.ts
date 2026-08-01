export type Assignment = { id: string; contactId: string; ownerId: number | null; collaboratorIds: number[]; status: string; version: number };
export type AssignmentPage = { items: Assignment[]; nextCursor: string };
export type Opportunity = { id: string; contactId: string; stage: string; amount: number; startDate: string; endDate: string; ownerId: number | null; status: string; lostReason: string; version: number };
export type OpportunityPage = { items: Opportunity[]; nextCursor: string };
export type FollowUpRecord = { id: string; contactId: string; content: string; createdAt: string; createdBy: number };
export type FollowUpPage = { items: FollowUpRecord[]; nextCursor: string };
export type Tag = { id: string; name: string; version: number };
export type TagPage = { items: Tag[]; nextCursor: string };
type Client = { request<T = unknown>(input: RequestInfo | URL, init?: RequestInit): Promise<T> };

export type ScrmApi = {
  listPublicPool(input: { corpId: number; cursor?: string; pageSize?: number }): Promise<AssignmentPage>;
  updateAssignment(input: { corpId: number; contactId: string; ownerId: number | null; collaboratorIds: number[]; version: number; idempotencyKey: string }): Promise<Assignment>;
  releaseToPublicPool(input: { corpId: number; contactId: string; version: number; idempotencyKey: string }): Promise<Assignment>;
  claimFromPublicPool(input: { corpId: number; contactId: string; version: number; idempotencyKey: string }): Promise<Assignment>;
  listOpportunities(input: { corpId: number; stage?: string; status?: string; ownerId?: number; cursor?: string; pageSize?: number }): Promise<OpportunityPage>;
  createOpportunity(input: { corpId: number; contactId: string; stage: string; amount: number; startDate: string; endDate: string; ownerId: number | null; idempotencyKey: string }): Promise<Opportunity>;
  changeOpportunityStage(input: { corpId: number; opportunityId: string; stageId: string; lostReason: string; version: number; idempotencyKey: string }): Promise<Opportunity>;
  listFollowUps(input: { corpId: number; contactId: string }): Promise<FollowUpPage>;
  appendFollowUp(input: { corpId: number; contactId: string; content: string; idempotencyKey: string }): Promise<FollowUpRecord>;
  listTags(input: { corpId: number }): Promise<TagPage>;
  createTag(input: { corpId: number; name: string; idempotencyKey: string }): Promise<Tag>;
  renameTag(input: { corpId: number; tagId: string; name: string; version: number; idempotencyKey: string }): Promise<Tag>;
  bindTags(input: { corpId: number; tagId: string; contactIds: string[]; idempotencyKey: string }): Promise<void>;
};

const json = (body: unknown, idempotencyKey: string): RequestInit => ({ method: 'POST', headers: { 'Content-Type': 'application/json', 'Idempotency-Key': idempotencyKey }, body: JSON.stringify(body) });

export function createScrmApi(client: Client): ScrmApi {
  return {
    async listPublicPool(input) {
      const query = new URLSearchParams({ corpId: String(input.corpId), pageSize: String(input.pageSize ?? 20) });
      if (input.cursor) query.set('cursor', input.cursor);
      return client.request(`/scrm/assignments?${query.toString()}`) as Promise<AssignmentPage>;
    },
    async updateAssignment(input) {
      return client.request('/scrm/assignments', { ...json(input, input.idempotencyKey), method: 'PUT' }) as Promise<Assignment>;
    },
    async releaseToPublicPool(input) {
      return client.request('/scrm/assignments/release', json(input, input.idempotencyKey)) as Promise<Assignment>;
    },
    async claimFromPublicPool(input) {
      return client.request('/scrm/assignments/claim', json(input, input.idempotencyKey)) as Promise<Assignment>;
    },
    async listOpportunities(input) {
      const query = new URLSearchParams({ corpId: String(input.corpId), pageSize: String(input.pageSize ?? 20) });
      if (input.stage) query.set('stage', input.stage);
      if (input.status) query.set('status', input.status);
      if (input.ownerId !== undefined) query.set('ownerId', String(input.ownerId));
      if (input.cursor) query.set('cursor', input.cursor);
      return client.request(`/scrm/opportunities?${query.toString()}`) as Promise<OpportunityPage>;
    },
    async createOpportunity(input) { return client.request('/scrm/opportunities', json(input, input.idempotencyKey)) as Promise<Opportunity>; },
    async changeOpportunityStage(input) {
      const body = { corpId: input.corpId, stageId: input.stageId, lostReason: input.lostReason, version: input.version };
      return client.request(`/scrm/opportunities/${encodeURIComponent(input.opportunityId)}/stage`, json(body, input.idempotencyKey)) as Promise<Opportunity>;
    },
    async listFollowUps(input) { return client.request(`/scrm/contacts/${input.contactId}/follow-ups?corpId=${input.corpId}`) as Promise<FollowUpPage>; },
    async appendFollowUp(input) {
      const body = { corpId: input.corpId, content: input.content };
      return client.request(`/scrm/contacts/${encodeURIComponent(input.contactId)}/follow-ups`, json(body, input.idempotencyKey)) as Promise<FollowUpRecord>;
    },
    async listTags(input) { return client.request(`/scrm/tags?corpId=${input.corpId}`) as Promise<TagPage>; },
    async createTag(input) { return client.request('/scrm/tags', json(input, input.idempotencyKey)) as Promise<Tag>; },
    async renameTag(input) { return client.request(`/scrm/tags/${input.tagId}`, { ...json(input, input.idempotencyKey), method: 'PUT' }) as Promise<Tag>; },
    async bindTags(input) { await client.request(`/scrm/tags/${input.tagId}/contacts`, json(input, input.idempotencyKey)); },
  };
}
