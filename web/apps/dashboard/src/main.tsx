import { createRoot } from 'react-dom/client';

import { createAuthStore } from '@mochat/auth';
import { createApiClient } from '@mochat/api-client';
import {
  parseRouteManifest,
  resolveRoute,
} from '@mochat/routing';

import migrationRoutesJson from './migration-routes.json';
import { createAccessLoader } from './app/access-loader';
import { createLegacyRouteLoader } from './app/legacy-route-loader';
import { createDashboardQueryClient, DashboardProviders } from './app/providers';
import { createDashboardRouter } from './app/router';
import { authenticate } from './features/auth/auth-api';
import {
  bindCorp,
  loadCorps,
} from './features/corp/corp-api';
import { createCorpAdminApi } from './features/corp/corp-admin-api';
import { CorpPage } from './features/corp/corp-page';
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
const corpAdminApi = createCorpAdminApi(apiClient);
const migrationManifest = parseRouteManifest(migrationRoutesJson);
const knownRoutes = new Set([
  '/',
  ...migrationManifest.map((route) => route.path),
]);
const loadAccess = createAccessLoader({
  clearSession: () => authStore.clearSession(),
  getSession: () => authStore.getSession(),
  knownRoutes,
  loadCorps: () => loadCorps(apiClient),
  loadMenu: () => loadMenu(apiClient),
});
const accessLoader = async ({ request }: { request: Request }) => {
  const access = await loadAccess({ request });
  const route = resolveRoute(new URL(request.url).pathname, migrationManifest);
  if (route?.target === 'legacy' && !('state' in access)) {
    await createLegacyRouteLoader({
      allowedRoutes: access.allowedRoutes,
      manifest: migrationManifest,
    })({ request });
  }
  return access;
};
const routerRef: { current?: ReturnType<typeof createDashboardRouter> } = {};
const router = createDashboardRouter({
  accessLoader,
  authenticate: (input) => authenticate(loginClient, input),
  getSession: () => authStore.getSession(),
  loadInitialData: () => Promise.resolve(),
  reactPages: {
    '/corp/index': <CorpPage api={corpAdminApi} />,
  },
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
