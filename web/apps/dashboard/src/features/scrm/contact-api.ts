export type ContactSummary = {
  id: string; name: string; phone: string; ownerId: number | null; assignmentStatus: string;
  tagNames: string[]; version: number; assignmentVersion: number; updatedAt: string;
};
export type ContactListInput = { corpId: number; keyword?: string; ownerIds?: number[]; tagIds?: string[]; statuses?: string[]; cursor?: string; pageSize?: number };
export type Assignment = { id: string; contactId: string; ownerId: number | null; collaboratorIds: number[]; status: string; version: number };
export type ContactDetail = ContactSummary & {
  assignment: Assignment;
  tags: { id: string; name: string }[];
  wecomFriends: { externalUserId: string; name: string; employeeId: number; addedAt: string }[];
  wecomFriendsAvailable: boolean;
  opportunities: { id: string; stage: string; status: string; amount: number; version: number }[];
  followUps: { id: string; contactId: string; content: string; createdAt: string; createdBy: number }[];
};
type Client = { request<T = unknown>(input: RequestInfo | URL, init?: RequestInit): Promise<T> };
const json = (body: unknown, idempotencyKey: string, method = 'POST'): RequestInit => ({ method, headers: { 'Content-Type': 'application/json', 'Idempotency-Key': idempotencyKey }, body: JSON.stringify(body) });

export type ContactApi = {
  listContacts(input: ContactListInput): Promise<{ items: ContactSummary[]; nextCursor: string }>;
  getContact(input: { corpId: number; contactId: string }): Promise<ContactDetail>;
  updateAssignment(input: { corpId: number; contactId: string; ownerId: number | null; collaboratorIds: number[]; version: number; idempotencyKey: string }): Promise<Assignment>;
  bindTags(input: { corpId: number; tagId: string; contactIds: string[]; idempotencyKey: string }): Promise<void>;
  appendFollowUp(input: { corpId: number; contactId: string; content: string; idempotencyKey: string }): Promise<ContactDetail['followUps'][number]>;
  listFollowUps(input: { corpId: number; contactId: string }): Promise<{ items: ContactDetail['followUps']; nextCursor: string }>;
  releaseToPublicPool(input: { corpId: number; contactId: string; version: number; idempotencyKey: string }): Promise<Assignment>;
  createOpportunity(input: { corpId: number; contactId: string; stage: string; amount: number; startDate: string; endDate: string; ownerId: number | null; idempotencyKey: string }): Promise<ContactDetail['opportunities'][number]>;
};

export function createContactApi(client: Client): ContactApi {
  return {
    async listContacts(input) {
      const query = new URLSearchParams({ corpId: String(input.corpId), pageSize: String(input.pageSize ?? 20) });
      if (input.keyword?.trim()) query.set('keyword', input.keyword.trim());
      input.ownerIds?.forEach((id) => query.append('ownerId', String(id)));
      input.tagIds?.forEach((id) => query.append('tagId', id));
      input.statuses?.forEach((status) => query.append('status', status));
      if (input.cursor) query.set('cursor', input.cursor);
      return client.request(`/scrm/contacts?${query.toString()}`) as Promise<{ items: ContactSummary[]; nextCursor: string }>;
    },
    async getContact(input) { return client.request(`/scrm/contacts/${encodeURIComponent(input.contactId)}?corpId=${input.corpId}`) as Promise<ContactDetail>; },
    async updateAssignment(input) { return client.request('/scrm/assignments', json(input, input.idempotencyKey, 'PUT')) as Promise<Assignment>; },
    async bindTags(input) { await client.request(`/scrm/tags/${encodeURIComponent(input.tagId)}/contacts`, json(input, input.idempotencyKey)); },
    async appendFollowUp(input) { return client.request(`/scrm/contacts/${encodeURIComponent(input.contactId)}/follow-ups`, json(input, input.idempotencyKey)) as Promise<ContactDetail['followUps'][number]>; },
    async listFollowUps(input) { return client.request(`/scrm/contacts/${encodeURIComponent(input.contactId)}/follow-ups?corpId=${input.corpId}`) as Promise<{ items: ContactDetail['followUps']; nextCursor: string }>; },
    async releaseToPublicPool(input) { return client.request('/scrm/assignments/release', json(input, input.idempotencyKey)) as Promise<Assignment>; },
    async createOpportunity(input) { return client.request('/scrm/opportunities', json(input, input.idempotencyKey)) as Promise<ContactDetail['opportunities'][number]>; },
  };
}
