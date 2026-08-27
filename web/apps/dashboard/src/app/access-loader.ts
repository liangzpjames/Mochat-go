import { ApiError } from '@mochat/api-client';
import type { Session } from '@mochat/auth';
import { redirect } from 'react-router';

import type { AccessProfile, DashboardCompanyContext } from '../features/access/access-api';
import type { MenuNode } from '../features/navigation/menu-tree';

export type AccessContext = {
  session: Session;
  corp: DashboardCompanyContext;
  /** Full server profile; optional only for legacy unit-test fixtures. */
  profile?: AccessProfile;
  /** @deprecated navigation no longer derives authorization from legacy menus. */
  menu?: readonly MenuNode[];
  allowedRoutes: ReadonlySet<string>;
  /** Empty means the page permission grants every page action; non-empty sets are legacy action contracts. */
  allowedActions: ReadonlySet<string>;
};

export type AccessLoaderDeps = {
  clearQueries: () => void;
  clearSession: () => void;
  getSession: () => Session | null;
  loadProfile: () => Promise<AccessProfile>;
  knownRoutes: ReadonlySet<string>;
  manifestRoutes: ReadonlySet<string>;
  now?: () => number;
};

function localReturnTo(request: Request): string {
  const url = new URL(request.url);
  return `${url.pathname}${url.search}${url.hash}`;
}

function throwRouterResponse(response: Response): never {
  // React Router loaders use thrown Response objects as their HTTP control flow.
  // eslint-disable-next-line @typescript-eslint/only-throw-error
  throw response;
}

export function createAccessLoader(deps: AccessLoaderDeps) {
  return async ({ request }: { request: Request }): Promise<AccessContext> => {
    const session = deps.getSession();
    if (session === null) {
      throwRouterResponse(
        redirect(`/login?returnTo=${encodeURIComponent(localReturnTo(request))}`),
      );
    }
    if (session.expiresAt !== null && session.expiresAt <= (deps.now ?? Date.now)()) {
      deps.clearSession();
      deps.clearQueries();
      throwRouterResponse(redirect('/login'));
    }
    const pathname = new URL(request.url).pathname;
    if (!deps.knownRoutes.has(pathname)) {
      throwRouterResponse(new Response(null, { status: 404 }));
    }

    try {
      const profile = await deps.loadProfile();
      const corp: DashboardCompanyContext = {
        id: String(profile.corpId),
        name: profile.corpName.trim() || '未命名企业',
        authorized: profile.corpId > 0,
      };
      if (profile.corpBindingStatus === 'suspended') {
        deps.clearSession();
        deps.clearQueries();
        throwRouterResponse(redirect(`/login?returnTo=${encodeURIComponent(localReturnTo(request))}`));
      }
      const routes = new Set<string>();
      const actions = new Set<string>();
      const catalogByPath = new Map(profile.catalog.map((item) => [item.path, item]));
      for (const permission of profile.effectivePermissions) {
        const catalog = catalogByPath.get(permission.path);
        if (catalog?.superadminOnly && !profile.isSuperAdmin) continue;
        if (deps.manifestRoutes.has(permission.path)) routes.add(permission.path);
      }
      if (pathname !== '/' && !routes.has(pathname)) {
        throwRouterResponse(new Response(null, { status: 403 }));
      }
      return {
        session,
        corp,
        profile,
        allowedRoutes: routes,
        allowedActions: actions,
      };
    } catch (error) {
      if (error instanceof ApiError && error.kind === 'unauthorized') {
        deps.clearSession();
        deps.clearQueries();
        throwRouterResponse(redirect(`/login?returnTo=${encodeURIComponent(localReturnTo(request))}`));
      }
      if (error instanceof ApiError && error.kind === 'forbidden') {
        if (error.machineCode === 'TENANT_ACCESS_DENIED') {
          deps.clearSession();
          deps.clearQueries();
          throwRouterResponse(redirect(`/login?returnTo=${encodeURIComponent(localReturnTo(request))}`));
        }
        if (error.machineCode === 'CORP_CONFIGURATION_REQUIRED') {
          throwRouterResponse(redirect('/company-setting/website'));
        }
        throwRouterResponse(new Response(null, { status: 403 }));
      }
      throw error;
    }
  };
}
