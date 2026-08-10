import type { AccessCatalogItem, EffectivePermission } from './access-api';

type Client = { request(input: RequestInfo | URL, init?: RequestInit): Promise<unknown> };
const json = (method: string, body: unknown): RequestInit => ({ method, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });

export type AccessRole = { id: number; name: string; remark: string; status: number; isSystem: boolean; memberCount: number; permissions: { code: string; scope: 'self' | 'department' | 'tenant' }[]; version: number };
export type AccessUser = { id: number; name: string; phone: string; status: number; isSuperAdmin: boolean; roles: { id: number; name: string; status: number; version: number }[]; directPermissions: { code: string; scope: 'self' | 'department' | 'tenant' }[]; inheritedPermissions: EffectivePermission[]; effectivePermissions: EffectivePermission[]; version: number };
export type Page<T> = { list: T[]; page: { page: number; perPage: number; total: number; totalPage: number } };

export function createDashboardAccessAdminApi(client: Client) {
  return {
    users: (input: { page: number; perPage: number }): Promise<Page<AccessUser>> => client.request(`/access/users?page=${input.page}&perPage=${input.perPage}`) as Promise<Page<AccessUser>>,
    user: (id: number): Promise<AccessUser> => client.request(`/access/users/${id}`) as Promise<AccessUser>,
    replaceUser: (id: number, input: { roleIds: number[]; directPermissions: { code: string; scope: string }[]; expectedVersion: number }) => client.request(`/access/users/${id}`, json('PUT', input)) as Promise<AccessUser>,
    roles: (input: { page: number; perPage: number }): Promise<Page<AccessRole>> => client.request(`/access/roles?page=${input.page}&perPage=${input.perPage}`) as Promise<Page<AccessRole>>,
    createRole: (input: { name: string; remark: string; status: number; permissions: { code: string; scope: string }[] }) => client.request('/access/roles', json('POST', input)) as Promise<AccessRole>,
    updateRole: (id: number, input: { name: string; remark: string; permissions: { code: string; scope: string }[]; expectedVersion: number }) => client.request(`/access/roles/${id}`, json('PUT', input)) as Promise<AccessRole>,
    updateRoleStatus: (id: number, input: { status: number; expectedVersion: number }) => client.request(`/access/roles/${id}/status`, json('PUT', input)) as Promise<AccessRole>,
    deleteRole: (id: number, expectedVersion: number) => client.request(`/access/roles/${id}`, json('DELETE', { expectedVersion })),
    catalog: (): Promise<AccessCatalogItem[]> => client.request('/access/catalog') as Promise<AccessCatalogItem[]>,
    audits: (input: { page: number; perPage: number }): Promise<Page<Record<string, unknown>>> => client.request(`/access/audits?page=${input.page}&perPage=${input.perPage}`) as Promise<Page<Record<string, unknown>>>,
  };
}
