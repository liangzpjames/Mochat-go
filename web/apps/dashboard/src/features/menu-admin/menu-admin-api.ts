export type MenuNode = {
  menuId: number; name: string; level: number; parentId: number; icon: string;
  status: number; menuPath: string; levelName: string; operateName: string;
  updatedAt: string; children: MenuNode[];
};
export type MenuOption = {
  menuId: number; name: string; level: number; parentId: number;
  dataPermission: number; children: MenuOption[];
};
export type MenuWrite = {
  level: number; name: string; icon: string; linkUrl: string; linkType: number;
  firstMenuId?: number; secondMenuId?: number; thirdMenuId?: number; fourthMenuId?: number;
  isPageMenu?: number; dataPermission?: number;
};
export type MenuDetail = MenuWrite & { menuId: number; status: number };
export type MenuListResult = {
  list: MenuNode[]; page: { page: number; perPage: number; total: number; totalPage: number };
};
type Client = { request(input: RequestInfo | URL, init?: RequestInit): Promise<unknown> };
const json = (method: string, body: unknown): RequestInit => ({
  method, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body),
});
export function createMenuAdminApi(client: Client) {
  return {
    list(input: { name: string; page: number; perPage: number }): Promise<MenuListResult> {
      const q = new URLSearchParams({ name: input.name, page: String(input.page), perPage: String(input.perPage) });
      return client.request(`/menu/index?${q.toString()}`) as Promise<MenuListResult>;
    },
    options: () => client.request('/menu/select') as Promise<MenuOption[]>,
    detail: (menuId: number) => client.request(`/menu/show?menuId=${menuId}`) as Promise<MenuDetail>,
    usedIcons: () => client.request('/menu/iconIndex') as Promise<string[]>,
    async create(input: MenuWrite) { await client.request('/menu/store', json('POST', input)); },
    async update(menuId: number, input: MenuWrite) { await client.request('/menu/update', json('PUT', { ...input, menuId })); },
    async updateStatus(menuId: number, status: number) { await client.request('/menu/statusUpdate', json('PUT', { menuId, status })); },
    async remove(menuId: number) { await client.request('/menu/destroy', json('DELETE', { menuId })); },
  };
}
