import {
  resolveRoute,
  type MigrationRoute,
} from '@mochat/routing';

export type LegacyRouteLoaderDeps = {
  allowedRoutes: ReadonlySet<string>;
  manifest: readonly MigrationRoute[];
  replace?: (url: string) => void;
};

function throwRouteResponse(status: number): never {
  // React Router loaders use thrown Response objects as HTTP control flow.
  // eslint-disable-next-line @typescript-eslint/only-throw-error
  throw new Response(null, { status });
}

export function createLegacyRouteLoader(deps: LegacyRouteLoaderDeps) {
  return ({ request }: { request: Request }): Promise<null> => {
    return Promise.resolve().then(() => {
      const url = new URL(request.url);
      const route = resolveRoute(url.pathname, deps.manifest);
      if (route === null || route.target !== 'legacy') {
        throwRouteResponse(404);
      }
      if (
        !deps.allowedRoutes.has(route.path)
        && !deps.allowedRoutes.has(url.pathname)
      ) {
        throwRouteResponse(403);
      }
      const destination = `/_legacy/dashboard${url.pathname}${url.search}${url.hash}`;
      (deps.replace ?? ((target) => window.location.replace(target)))(destination);
      return null;
    });
  };
}
