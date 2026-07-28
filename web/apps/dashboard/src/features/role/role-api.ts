export type RoleWrite = {
  name: string;
  remarks: string;
  dataPermission: number;
};
export type RoleItem = {
  roleId: number;
  name: string;
  employeeNum: number;
  remarks: string;
  updatedAt: string;
  status: number;
  dataPermission?: number;
};
export type RoleDetail = RoleWrite & { roleId: number };
export type RoleMember = {
  employeeId: number;
  employeeName: string;
  phone: string;
  email: string;
  department: string;
};
export type RolePageResult<T> = {
  list: T[];
  page: { page: number; perPage: number; total: number; totalPage: number };
};

type ApiClient = { request(input: RequestInfo | URL, init?: RequestInit): Promise<unknown> };
const jsonRequest = (method: string, body: unknown): RequestInit => ({
  method, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body),
});

export function createRoleApi(client: ApiClient) {
  return {
    list(input: { name: string; page: number; perPage: number }): Promise<RolePageResult<RoleItem>> {
      const query = new URLSearchParams({
        name: input.name, page: String(input.page), perPage: String(input.perPage),
      });
      return client.request(`/role/index?${query.toString()}`) as Promise<RolePageResult<RoleItem>>;
    },
    detail(roleId: number): Promise<RoleDetail> {
      return client.request(`/role/show?roleId=${roleId}`) as Promise<RoleDetail>;
    },
    members(input: { roleId: number; page: number; perPage: number }): Promise<RolePageResult<RoleMember>> {
      const query = new URLSearchParams({
        roleId: String(input.roleId), page: String(input.page), perPage: String(input.perPage),
      });
      return client.request(`/role/showEmployee?${query.toString()}`) as Promise<RolePageResult<RoleMember>>;
    },
    async create(input: RoleWrite) {
      await client.request('/role/store', jsonRequest('POST', input));
    },
    async copy(roleId: number, input: RoleWrite) {
      await client.request('/role/store', jsonRequest('POST', { ...input, roleId }));
    },
    async update(roleId: number, input: RoleWrite) {
      await client.request('/role/update', jsonRequest('PUT', { ...input, roleId }));
    },
    async updateStatus(roleId: number, status: number) {
      await client.request('/role/statusUpdate', jsonRequest('PUT', { roleId, status }));
    },
    async remove(roleId: number) {
      await client.request('/role/destroy', jsonRequest('DELETE', { roleId }));
    },
  };
}
