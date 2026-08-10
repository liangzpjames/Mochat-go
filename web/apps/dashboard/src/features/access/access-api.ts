import type { DataScope } from './access-types';

type ApiClient = {
  request(input: RequestInfo | URL, init?: RequestInit): Promise<unknown>;
};

export type PermissionSource = {
  type: string;
  id: number;
  name: string;
  scope: DataScope;
};

export type EffectivePermission = {
  code: string;
  path: string;
  name: string;
  scope: DataScope;
  sources: PermissionSource[];
};

export type AccessCatalogItem = {
  id: number;
  code: string;
  path: string;
  name: string;
  groupCode: string;
  sort: number;
  superadminOnly: boolean;
  scopeRequired: boolean;
};

export type AccessProfile = {
  userId: number;
  userName: string;
  tenantId: number;
  corpId: number;
  workEmployeeId: number;
  departmentIds: number[];
  departmentEmployeeIds: number[];
  isSuperAdmin: boolean;
  catalog: AccessCatalogItem[];
  effectivePermissions: EffectivePermission[];
  allowedRoutes: string[];
};

export async function loadAccessProfile(client: ApiClient): Promise<AccessProfile> {
  return await client.request('/access/profile') as AccessProfile;
}
