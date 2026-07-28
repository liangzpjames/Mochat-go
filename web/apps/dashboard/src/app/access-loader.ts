import { ApiError } from '@mochat/api-client';
import type { Session } from '@mochat/auth';
import { redirect } from 'react-router';

import type { CorpOption } from '../features/corp/corp-api';
import {
  buildMenuAccess,
  type MenuNode,
} from '../features/navigation/menu-tree';

export type AccessContext = {
  session: Session;
  corp: CorpOption;
  menu: readonly MenuNode[];
  allowedRoutes: ReadonlySet<string>;
  allowedActions: ReadonlySet<string>;
};

export type CorpSelection = {
  state: 'select-corp';
  corps: readonly CorpOption[];
};

export type AccessLoaderDeps = {
  clearSession: () => void;
  getSession: () => Session | null;
  loadCorps: () => Promise<readonly CorpOption[]>;
  loadMenu: (corpId: string) => Promise<readonly MenuNode[]>;
  knownRoutes: ReadonlySet<string>;
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
  return async ({ request }: { request: Request }): Promise<AccessContext | CorpSelection> => {
    const session = deps.getSession();
    if (session === null) {
      throwRouterResponse(
        redirect(`/login?returnTo=${encodeURIComponent(localReturnTo(request))}`),
      );
    }
    if (session.expiresAt !== null && session.expiresAt <= (deps.now ?? Date.now)()) {
      deps.clearSession();
      throwRouterResponse(redirect('/login'));
    }
    const pathname = new URL(request.url).pathname;
    if (!deps.knownRoutes.has(pathname)) {
      throwRouterResponse(new Response(null, { status: 404 }));
    }

    try {
      const corps = await deps.loadCorps();
      if (session.corpId === null) {
        return { state: 'select-corp', corps };
      }
      const corp = corps.find(
        (candidate) => candidate.id === session.corpId && candidate.authorized,
      );
      if (corp === undefined) {
        return { state: 'select-corp', corps };
      }

      const menu = await deps.loadMenu(corp.id);
      const { routes, actions } = buildMenuAccess(menu, deps.knownRoutes);
      if (pathname !== '/' && !routes.has(pathname)) {
        throwRouterResponse(new Response(null, { status: 403 }));
      }
      return {
        session,
        corp,
        menu,
        allowedRoutes: routes,
        allowedActions: actions,
      };
    } catch (error) {
      if (error instanceof ApiError && error.kind === 'unauthorized') {
        deps.clearSession();
        throwRouterResponse(redirect('/login'));
      }
      if (error instanceof ApiError && error.kind === 'forbidden') {
        throwRouterResponse(new Response(null, { status: 403 }));
      }
      throw error;
    }
  };
}
