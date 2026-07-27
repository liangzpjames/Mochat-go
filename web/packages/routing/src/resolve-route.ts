import { parseRouteManifest, type MigrationRoute } from './manifest';

function normalizePath(path: string): string {
  const pathname = path.split(/[?#]/, 1)[0] ?? '';
  return pathname === '/' ? pathname : pathname.replace(/\/+$/, '');
}

function matchesPath(pathname: string, pattern: string): boolean {
  const pathnameSegments = normalizePath(pathname).split('/');
  const patternSegments = normalizePath(pattern).split('/');
  return patternSegments.length === pathnameSegments.length
    && patternSegments.every((segment, index) => (
      segment.startsWith(':')
        ? pathnameSegments[index] !== ''
        : segment === pathnameSegments[index]
    ));
}

export function resolveRoute(
  pathname: string,
  routes: readonly MigrationRoute[],
): MigrationRoute | null {
  try {
    parseRouteManifest(routes);
  } catch {
    return null;
  }
  return routes.find((route) => matchesPath(pathname, route.path)) ?? null;
}
