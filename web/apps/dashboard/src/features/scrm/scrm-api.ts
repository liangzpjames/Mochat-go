export type PublicPoolAction = 'enter' | 'return' | 'reclaim';
export type Assignment = { id: string; contactId: string; ownerId: number | null; collaboratorIds: number[]; status: string; version: number };
export type PublicPoolAssignment = Assignment & {
  contactName: string;
  source: string; businessType: string; tagNames: string[]; region: string; recycleCount: number; poolAction: string;
  poolReason: string; previousOwnerId: number | null; lastFollowUpAt: string;
};
export type AssignmentPage = { items: PublicPoolAssignment[]; nextCursor: string };
export type PublicPoolListInput = { corpId: number; keyword?: string; sources?: string[]; businessTypes?: string[]; tagIds?: string[]; regions?: string[]; reasons?: string[]; previousOwnerIds?: number[]; cursor?: string; pageSize?: number };
export type PublicPoolClaimTarget = { contactId: string; version: number; idempotencyKey: string };
export type PublicPoolMutationResult = { id: string; status: 'succeeded' | 'failed'; errorCode: string; assignment?: Assignment };
export type Opportunity = { id: string; contactId: string; stage: string; amount: number; startDate: string; endDate: string; ownerId: number | null; status: string; lostReason: string; version: number };
export type OpportunityPage = { items: Opportunity[]; nextCursor: string };
export type FollowUpRecord = { id: string; contactId: string; content: string; createdAt: string; createdBy: number };
export type FollowUpPage = { items: FollowUpRecord[]; nextCursor: string };
export type TagGroup = { id: string; name: string; version: number; tagCount: number };
export type Tag = { id: string; groupId: string; name: string; version: number; usageCount: number };
export type TagPage = { items: Tag[]; nextCursor: string };
export type TagCatalog = { groups: TagGroup[]; tags: Tag[] };
export type BusinessOption = { id: string; name: string; version?: number };
type Client = { request<T = unknown>(input: RequestInfo | URL, init?: RequestInit): Promise<T> };

export type ScrmApi = {
  listContactOptions?: (input: { corpId: number }) => Promise<BusinessOption[]>;
  listEmployeeOptions?: () => Promise<BusinessOption[]>;
  listStageOptions?: (input: { corpId: number }) => Promise<BusinessOption[]>;
  listPublicPool: (input: PublicPoolListInput) => Promise<AssignmentPage>;
  updateAssignment: (input: { corpId: number; contactId: string; ownerId: number | null; collaboratorIds: number[]; version: number; idempotencyKey: string }) => Promise<Assignment>;
  releaseToPublicPool: (input: { corpId: number; contactId: string; version: number; action: PublicPoolAction; reason: string; idempotencyKey: string }) => Promise<Assignment>;
  claimFromPublicPool: (input: { corpId: number; contactId: string; userId: number; version: number; idempotencyKey: string }) => Promise<Assignment>;
  batchClaimFromPublicPool: (input: { corpId: number; userId: number; targets: PublicPoolClaimTarget[] }) => Promise<{ results: PublicPoolMutationResult[] }>;
  listOpportunities: (input: { corpId: number; stage?: string; status?: string; ownerId?: number; cursor?: string; pageSize?: number }) => Promise<OpportunityPage>;
  createOpportunity: (input: { corpId: number; contactId: string; stage: string; amount: number; startDate: string; endDate: string; ownerId: number | null; idempotencyKey: string }) => Promise<Opportunity>;
  changeOpportunityStage: (input: { corpId: number; opportunityId: string; stageId: string; lostReason: string; version: number; idempotencyKey: string }) => Promise<Opportunity>;
  listFollowUps: (input: { corpId: number; contactId: string }) => Promise<FollowUpPage>;
  appendFollowUp: (input: { corpId: number; contactId: string; content: string; idempotencyKey: string }) => Promise<FollowUpRecord>;
  listTags: (input: { corpId: number }) => Promise<TagPage>;
  listTagCatalog?: (input: { corpId: number; groupId?: string; keyword?: string }) => Promise<TagCatalog>;
  createTagGroup?: (input: { corpId: number; name: string; idempotencyKey: string }) => Promise<TagGroup>;
  renameTagGroup?: (input: { corpId: number; groupId: string; name: string; version: number; idempotencyKey: string }) => Promise<TagGroup>;
  createTag: (input: { corpId: number; groupId: string; name: string; idempotencyKey: string }) => Promise<Tag>;
  renameTag: (input: { corpId: number; tagId: string; name: string; version: number; idempotencyKey: string }) => Promise<Tag>;
  moveTag?: (input: { corpId: number; tagId: string; groupId: string; version: number; idempotencyKey: string }) => Promise<Tag>;
  deleteTag?: (input: { corpId: number; tagId: string; version: number; idempotencyKey: string }) => Promise<{ affectedResourceCount: number }>;
  previewDeleteTag?: (input: { corpId: number; tagId: string }) => Promise<{ tagId: string; version: number; affectedResourceCount: number }>;
  maintainTagContacts?: (input: { corpId: number; tagId: string; addContactIds: string[]; removeContactIds: string[]; version: number; idempotencyKey: string }) => Promise<Tag>;
};

const json = (body: unknown, idempotencyKey: string): RequestInit => ({ method: 'POST', headers: { 'Content-Type': 'application/json', 'Idempotency-Key': idempotencyKey }, body: JSON.stringify(body) });

export function createScrmApi(client: Client): ScrmApi {
  return {
    async listContactOptions(input) {
      const result = await client.request<{ items?: Array<{ id?: string; name?: string; assignmentVersion?: number; version?: number }> }>(`/scrm/contacts?corpId=${input.corpId}&pageSize=100`);
      return (result.items ?? []).filter((item) => Boolean(item.id)).map((item) => ({ id: item.id!, name: item.name?.trim() || '未命名联系人', version: Number(item.assignmentVersion ?? item.version ?? 0) }));
    },
    async listEmployeeOptions() {
      const result = await client.request<{ list?: Array<{ id?: number; name?: string }> }>('/workEmployee/index?page=1&perPage=200');
      return (result.list ?? []).filter((item) => Number(item.id) > 0).map((item) => ({ id: String(item.id), name: item.name?.trim() || '未命名员工' }));
    },
    async listStageOptions(input) {
      const result = await client.request<Array<{ type?: string; key?: string; value?: string; label?: string; enabled?: boolean }>>(`/scrm/settings?corpId=${input.corpId}`);
      return (Array.isArray(result) ? result : []).filter((item) => item.type === 'funnel_stage' && item.enabled !== false && item.key).map((item) => ({ id: item.key!, name: item.value?.trim() || item.label?.trim() || item.key! }));
    },
    async listPublicPool(input) {
      const query = new URLSearchParams({ corpId: String(input.corpId), pageSize: String(input.pageSize ?? 20) });
      if (input.keyword?.trim()) query.set('keyword', input.keyword.trim());
      input.sources?.forEach((value) => query.append('source', value));
      input.businessTypes?.forEach((value) => query.append('businessType', value));
      input.tagIds?.forEach((value) => query.append('tagId', value));
      input.regions?.forEach((value) => query.append('region', value));
      input.reasons?.forEach((value) => query.append('reason', value));
      input.previousOwnerIds?.forEach((value) => query.append('previousOwnerId', String(value)));
      if (input.cursor) query.set('cursor', input.cursor);
      return client.request(`/scrm/assignments?${query.toString()}`);
    },
    async updateAssignment(input) {
      return client.request('/scrm/assignments', { ...json(input, input.idempotencyKey), method: 'PUT' });
    },
    async releaseToPublicPool(input) {
      const body = { corpId: input.corpId, contactId: input.contactId, version: input.version, action: input.action, reason: input.reason };
      return client.request('/scrm/assignments/release', json(body, input.idempotencyKey));
    },
    async claimFromPublicPool(input) {
      const body = { corpId: input.corpId, contactId: input.contactId, userId: input.userId, version: input.version };
      return client.request('/scrm/assignments/claim', json(body, input.idempotencyKey));
    },
    async batchClaimFromPublicPool(input) { return client.request('/scrm/assignments/claim/batch', json(input, `batch-${input.userId}`)); },
    async listOpportunities(input) {
      const query = new URLSearchParams({ corpId: String(input.corpId), pageSize: String(input.pageSize ?? 20) });
      if (input.stage) query.set('stage', input.stage);
      if (input.status) query.set('status', input.status);
      if (input.ownerId !== undefined) query.set('ownerId', String(input.ownerId));
      if (input.cursor) query.set('cursor', input.cursor);
      return client.request(`/scrm/opportunities?${query.toString()}`);
    },
    async createOpportunity(input) { return client.request('/scrm/opportunities', json(input, input.idempotencyKey)); },
    async changeOpportunityStage(input) {
      const body = { corpId: input.corpId, stageId: input.stageId, lostReason: input.lostReason, version: input.version };
      return client.request(`/scrm/opportunities/${encodeURIComponent(input.opportunityId)}/stage`, json(body, input.idempotencyKey));
    },
    async listFollowUps(input) { return client.request(`/scrm/contacts/${input.contactId}/follow-ups?corpId=${input.corpId}`); },
    async appendFollowUp(input) {
      const body = { corpId: input.corpId, content: input.content };
      return client.request(`/scrm/contacts/${encodeURIComponent(input.contactId)}/follow-ups`, json(body, input.idempotencyKey));
    },
    async listTags(input) { return client.request(`/scrm/tags?corpId=${input.corpId}`); },
    async listTagCatalog(input) {
      const query = new URLSearchParams({ corpId: String(input.corpId) });
      if (input.groupId) query.set('groupId', input.groupId);
      if (input.keyword?.trim()) query.set('keyword', input.keyword.trim());
      return client.request(`/scrm/tags?${query.toString()}`);
    },
    async createTagGroup(input) { return client.request('/scrm/tag-groups', json({ corpId: input.corpId, name: input.name }, input.idempotencyKey)); },
    async renameTagGroup(input) { return client.request(`/scrm/tag-groups/${encodeURIComponent(input.groupId)}`, { ...json({ corpId: input.corpId, name: input.name, version: input.version }, input.idempotencyKey), method: 'PUT' }); },
    async createTag(input) { return client.request('/scrm/tags', json({ corpId: input.corpId, groupId: input.groupId, name: input.name }, input.idempotencyKey)); },
    async renameTag(input) { return client.request(`/scrm/tags/${encodeURIComponent(input.tagId)}`, { ...json({ corpId: input.corpId, name: input.name, version: input.version }, input.idempotencyKey), method: 'PUT' }); },
    async moveTag(input) { return client.request(`/scrm/tags/${encodeURIComponent(input.tagId)}/move`, json({ corpId: input.corpId, groupId: input.groupId, version: input.version }, input.idempotencyKey)); },
    async deleteTag(input) { return client.request(`/scrm/tags/${encodeURIComponent(input.tagId)}`, { ...json({ corpId: input.corpId, version: input.version }, input.idempotencyKey), method: 'DELETE' }); },
    async previewDeleteTag(input) { return client.request(`/scrm/tags/${encodeURIComponent(input.tagId)}/delete-preview?corpId=${input.corpId}`); },
    async maintainTagContacts(input) { return client.request(`/scrm/tags/${encodeURIComponent(input.tagId)}/contacts`, { ...json({ corpId: input.corpId, addContactIds: input.addContactIds, removeContactIds: input.removeContactIds, version: input.version }, input.idempotencyKey), method: 'PUT' }); },
  };
}
