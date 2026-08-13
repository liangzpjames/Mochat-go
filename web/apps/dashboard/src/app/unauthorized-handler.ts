export type DashboardUnauthorizedHandlerDeps = {
  clearSession: () => void;
  clearQueries: () => void;
  getCurrentPath: () => string;
  navigateToLogin: (target: string) => void;
};

function loginTarget(currentPath: string): string {
  if (/^\/login(?:[?#]|$)/.test(currentPath)) return '/login';
  return `/login?returnTo=${encodeURIComponent(currentPath)}`;
}

export function createDashboardUnauthorizedHandler(
  deps: DashboardUnauthorizedHandlerDeps,
): () => void {
  let handled = false;

  return () => {
    if (handled) return;
    handled = true;

    const target = loginTarget(deps.getCurrentPath());
    deps.clearSession();
    deps.clearQueries();
    deps.navigateToLogin(target);
  };
}
