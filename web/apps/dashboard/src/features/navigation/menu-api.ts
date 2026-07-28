import type { MenuNode } from './menu-tree';

type ApiClient = {
  request(input: RequestInfo | URL, init?: RequestInit): Promise<unknown>;
};

type MenuResponse = {
  name: string;
  icon?: string | null;
  linkUrl?: string | null;
  linkType?: number;
  children?: MenuResponse[];
};

function mapMenu(node: MenuResponse): MenuNode {
  return {
    name: node.name,
    icon: node.icon || null,
    linkUrl: node.linkUrl || null,
    linkType: node.linkType === 2 ? 2 : 1,
    children: (node.children ?? []).map(mapMenu),
  };
}

export async function loadMenu(client: ApiClient): Promise<MenuNode[]> {
  const menu = await client.request('/role/permissionByUser') as MenuResponse[];
  return menu.map(mapMenu);
}
