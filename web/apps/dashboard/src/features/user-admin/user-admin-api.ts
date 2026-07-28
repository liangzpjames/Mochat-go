export type UserWrite = {
  userName: string; phone: string; gender: number; roleId: number; status: number;
};
export type UserCreate = UserWrite & { password: string; confirmPass: string };
export type Department = { departmentId?: number; departmentName?: string;
  workDepartmentId?: number; workDepartmentName?: string };
export type UserItem = {
  userId: number; userName: string; phone: string; gender: number; roleId: number;
  roleName: string; status: number; statusText: string; createdAt: string; department: Department[];
};
export type UserListResult = {
  list: UserItem[]; normalNum: number; notEnabledNum: number; disableNum: number;
  page: { perPage: number; total: number; totalPage: number };
};
export type RoleOption = { roleId: number; name: string };
type Client = { request(input: RequestInfo | URL, init?: RequestInit): Promise<unknown> };
const json = (method: string, body: unknown): RequestInit => ({
  method, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body),
});
export function createUserAdminApi(client: Client) {
  return {
    list(input: { phone: string; status: number | ''; page: number; perPage: number }): Promise<UserListResult> {
      const q = new URLSearchParams({ phone: input.phone, status: String(input.status), page: String(input.page), perPage: String(input.perPage) });
      return client.request(`/user/index?${q.toString()}`) as Promise<UserListResult>;
    },
    detail: (userId: number) => client.request(`/user/show?userId=${userId}`) as Promise<UserItem>,
    departments: (phone: string) => client.request(`/workDepartment/selectByPhone?phone=${encodeURIComponent(phone)}`) as Promise<Department[]>,
    roles: () => client.request('/role/select') as Promise<RoleOption[]>,
    async create(input: UserCreate) { await client.request('/user/store', json('POST', input)); },
    async update(userId: number, input: UserWrite) { await client.request('/user/update', json('PUT', { ...input, userId })); },
    async updateStatus(userIds: number[], status: number) {
      await client.request('/user/statusUpdate', json('PUT', { userId: userIds.join(','), status }));
    },
    async resetPassword(id: number, newPassword: string) {
      await client.request('/user/passwordReset', json('PUT', { id, newPassword }));
    },
  };
}
