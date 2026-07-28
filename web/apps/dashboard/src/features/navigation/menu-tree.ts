export type MenuNode = {
  name: string;
  icon: string | null;
  linkUrl: string | null;
  linkType: 1 | 2;
  children: MenuNode[];
};

function isInternalPath(value: string | null): value is string {
  return value !== null
    && value.startsWith('/')
    && !value.includes('@')
    && !value.startsWith('//');
}

export function buildMenuAccess(
  nodes: readonly MenuNode[],
  registeredRoutes?: ReadonlySet<string>,
): {
  routes: ReadonlySet<string>;
  actions: ReadonlySet<string>;
} {
  const routes = new Set<string>();
  const actions = new Set<string>();

  const visit = (items: readonly MenuNode[], depth: number) => {
    for (const item of items) {
      if (item.linkType === 1 && isInternalPath(item.linkUrl)) {
        const isRegisteredPage = depth === 3
          ? registeredRoutes === undefined || registeredRoutes.has(item.linkUrl)
          : depth >= 4 && registeredRoutes?.has(item.linkUrl) === true;
        if (isRegisteredPage) {
          routes.add(item.linkUrl);
        }
      }
      if (depth >= 4 && item.linkUrl?.startsWith('/')) {
        actions.add(item.linkUrl);
      }
      visit(item.children, depth + 1);
    }
  };

  visit(nodes, 1);
  return { routes, actions };
}
