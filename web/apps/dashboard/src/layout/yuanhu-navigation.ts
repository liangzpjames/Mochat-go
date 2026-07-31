export type YuanhuManifest = {
  groups: readonly YuanhuNavigationGroup[];
  pages: readonly YuanhuNavigationPage[];
};

export type YuanhuNavigationGroup = {
  id: string;
  title: string;
};

export type YuanhuNavigationPage = {
  path: string;
  title: string;
  groupId: string | null;
};

export type YuanhuNavigationAccess = {
  allowedRoutes: ReadonlySet<string>;
  pathname: string;
};

export type YuanhuNavigationItem = {
  title: string;
  path: string;
  activePath: string | null;
};

export type YuanhuNavigationGroupResult = YuanhuNavigationGroup & {
  items: readonly YuanhuNavigationItem[];
};

export function buildYuanhuTopLevelNavigation(
  access: YuanhuNavigationAccess | null,
  manifest: YuanhuManifest,
): readonly YuanhuNavigationItem[] {
  if (access === null) {
    return [];
  }
  return manifest.pages
    .filter((page) => page.groupId === null && access.allowedRoutes.has(page.path))
    .map((page) => ({
      title: page.title,
      path: page.path,
      activePath: page.path === access.pathname ? page.path : null,
    }));
}

export function buildYuanhuNavigation(
  access: YuanhuNavigationAccess | null,
  manifest: YuanhuManifest,
): readonly YuanhuNavigationGroupResult[] {
  if (access === null) {
    return [];
  }

  const seenPaths = new Set<string>();
  return manifest.groups.map((group) => ({
    ...group,
    items: manifest.pages.flatMap((page) => {
      if (
        page.groupId !== group.id
        || !access.allowedRoutes.has(page.path)
        || seenPaths.has(page.path)
      ) {
        return [];
      }
      seenPaths.add(page.path);
      return [{
        title: page.title,
        path: page.path,
        activePath: page.path === access.pathname ? page.path : null,
      }];
    }),
  }));
}
