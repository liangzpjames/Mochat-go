export type TagGroup = { groupId: number; groupName: string };
export type TagItem = { id: number; name: string; contactNum: number; groupId?: number };
export type TagDetail = { tagId: number; tagName: string; groupId: number };
export type TagListResult = { list: TagItem[]; syncTagTime: string;
  page: { page: number; perPage: number; total: number; totalPage: number } };
type Client = { request(input: RequestInfo | URL, init?: RequestInit): Promise<unknown> };
const json = (method: string, body?: unknown): RequestInit => ({
  method, headers: { 'Content-Type': 'application/json' }, ...(body === undefined ? {} : { body: JSON.stringify(body) }),
});
export function createContactTagApi(client: Client) {
  return {
    list(input: { groupId: number; page: number; perPage: number }): Promise<TagListResult> {
      const q = new URLSearchParams({ groupId: String(input.groupId), page: String(input.page), perPage: String(input.perPage) });
      return client.request(`/workContactTag/index?${q.toString()}`) as Promise<TagListResult>;
    },
    groups: () => client.request('/workContactTagGroup/index') as Promise<TagGroup[]>,
    detail: (tagId: number) => client.request(`/workContactTag/detail?tagId=${tagId}`) as Promise<TagDetail>,
    async createTags(groupId: number, tagName: string[]) { await client.request('/workContactTag/store', json('POST', { groupId, tagName })); },
    async updateTag(input: TagDetail & { isUpdate: number }) { await client.request('/workContactTag/update', json('PUT', input)); },
    async removeTags(ids: number[]) { await client.request('/workContactTag/destroy', json('DELETE', { tagId: ids.join(',') })); },
    async moveTags(ids: number[], groupId: number) { await client.request('/workContactTag/move', json('PUT', { tagId: ids.join(','), groupId })); },
    async sync() { await client.request('/workContactTag/synContactTag', json('PUT')); },
    async createGroup(groupName: string) { await client.request('/workContactTagGroup/store', json('POST', { groupName })); },
    async updateGroup(groupId: number, groupName: string) { await client.request('/workContactTagGroup/update', json('PUT', { groupId, groupName, isUpdate: 2 })); },
    async removeGroup(groupId: number) { await client.request('/workContactTagGroup/destroy', json('DELETE', { groupId })); },
  };
}
