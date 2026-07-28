import { createRoot } from 'react-dom/client';

import { createAuthStore } from '@mochat/auth';
import { createApiClient } from '@mochat/api-client';
import { parseRouteManifest } from '@mochat/routing';

import migrationRoutesJson from './migration-routes.json';
import { createAccessLoader } from './app/access-loader';
import { createDashboardQueryClient, DashboardProviders } from './app/providers';
import { createDashboardRouter } from './app/router';
import { authenticate } from './features/auth/auth-api';
import {
  bindCorp,
  loadCorps,
} from './features/corp/corp-api';
import { CorpProvider } from './features/corp/corp-provider';
import { loadMenu } from './features/navigation/menu-api';
import { buildMenuAccess } from './features/navigation/menu-tree';
import './styles/index.css';

const rootElement = document.getElementById('root');
if (rootElement === null) {
  throw new Error('Dashboard root element was not found');
}

const authStore = createAuthStore();
const apiClient = createApiClient({
  baseUrl: new URL('/dashboard/', window.location.origin).toString(),
  getToken: () => authStore.getSession()?.token ?? null,
  onUnauthorized: () => {
    authStore.clearSession();
  },
});
const loginClient = createApiClient({
  baseUrl: new URL('/dashboard/', window.location.origin).toString(),
  getToken: () => null,
  onUnauthorized: () => undefined,
});
const queryClient = createDashboardQueryClient();
const knownRoutes = new Set([
  '/',
  ...parseRouteManifest(migrationRoutesJson).map((route) => route.path),
]);
const accessLoader = createAccessLoader({
  clearSession: () => authStore.clearSession(),
  getSession: () => authStore.getSession(),
  knownRoutes,
  loadCorps: () => loadCorps(apiClient),
  loadMenu: () => loadMenu(apiClient),
});
const routerRef: { current?: ReturnType<typeof createDashboardRouter> } = {};
const router = createDashboardRouter({
  accessLoader,
  authenticate: (input) => authenticate(loginClient, input),
  getSession: () => authStore.getSession(),
  loadInitialData: () => Promise.resolve(),
  renderAccess: (access, children) => (
    <CorpProvider
      bindCorp={(corpId) => bindCorp(apiClient, corpId)}
      initialCorpId={authStore.getSession()?.corpId ?? null}
      {...('state' in access ? { initialCorps: access.corps } : {})}
      loadCorps={() => loadCorps(apiClient)}
      loadMenuAccess={async () => {
        const access = buildMenuAccess(await loadMenu(apiClient), knownRoutes);
        return { firstRoute: access.routes.values().next().value ?? '/' };
      }}
      navigate={(path) => void routerRef.current?.navigate(path)}
      persistCorpId={(corpId) => {
        const session = authStore.getSession();
        if (session !== null) {
          authStore.setSession({ ...session, corpId });
        }
      }}
      queryClient={queryClient}
    >
      {children}
    </CorpProvider>
  ),
  setSession: (session) => authStore.setSession(session),
});
routerRef.current = router;

createRoot(rootElement).render(
  <DashboardProviders queryClient={queryClient} router={router} />,
);
