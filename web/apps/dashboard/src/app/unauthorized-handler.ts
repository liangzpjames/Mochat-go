export type DashboardUnauthorizedHandlerDeps = {
  clearSession: () => void;
  clearQueries: () => void;
  getCurrentPath: () => string;
  getSessionKey: () => string | null;
  navigateToLogin: (target: string) => void;
};

function loginTarget(currentPath: string): string {
  if (/^\/login(?:[?#]|$)/.test(currentPath)) return '/login';
  return `/login?returnTo=${encodeURIComponent(currentPath)}`;
}

export function createDashboardUnauthorizedHandler(
  deps: DashboardUnauthorizedHandlerDeps,
): () => void {
  let handledSessionKey: string | null | undefined;

  return () => {
    const sessionKey = deps.getSessionKey();
    if (
      handledSessionKey !== undefined
      && (sessionKey === null || sessionKey === handledSessionKey)
    ) return;
    handledSessionKey = sessionKey;

    const target = loginTarget(deps.getCurrentPath());
    deps.clearSession();
    deps.clearQueries();
    deps.navigateToLogin(target);
  };
}
