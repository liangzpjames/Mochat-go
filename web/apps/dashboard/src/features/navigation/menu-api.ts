import type { MenuNode } from './menu-tree';

type ApiClient = {
  request(input: RequestInfo | URL, init?: RequestInit): Promise<unknown>;
};

type EffectivePermissionResponse = {
  code: string;
  name: string;
  path: string;
};

type DashboardAccessProfileResponse = {
  effectivePermissions: EffectivePermissionResponse[];
};

function mapPermission(permission: EffectivePermissionResponse): MenuNode {
  return {
    name: permission.name,
    icon: null,
    linkUrl: permission.path,
    linkType: 1,
    children: [],
  };
}

export async function loadMenu(client: ApiClient): Promise<MenuNode[]> {
  const profile = await client.request('/access/profile') as DashboardAccessProfileResponse;
  return (profile.effectivePermissions ?? []).map(mapPermission);
}
