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
import { createPasswordApi } from './features/password/password-api';
import { PasswordPage } from './features/password/password-page';
import { createEmployeeApi } from './features/employee/employee-api';
import { EmployeePage } from './features/employee/employee-page';
import { createDepartmentApi } from './features/department/department-api';
import { DepartmentPage } from './features/department/department-page';
import { createContactFieldApi } from './features/contact-field/contact-field-api';
import { ContactFieldPage } from './features/contact-field/contact-field-page';
import { createRoleApi } from './features/role/role-api';
import { RolePage } from './features/role/role-page';
import { createMenuAdminApi } from './features/menu-admin/menu-admin-api';
import { MenuAdminPage } from './features/menu-admin/menu-admin-page';
import { createUserAdminApi } from './features/user-admin/user-admin-api';
import { UserAdminPage } from './features/user-admin/user-admin-page';
import { createContactTagApi } from './features/contact-tag/contact-tag-api';
import { ContactTagPage } from './features/contact-tag/contact-tag-page';
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
const passwordApi = createPasswordApi(apiClient);
const employeeApi = createEmployeeApi(apiClient);
const departmentApi = createDepartmentApi(apiClient);
const contactFieldApi = createContactFieldApi(apiClient);
const roleApi = createRoleApi(apiClient);
const menuAdminApi = createMenuAdminApi(apiClient);
const userAdminApi = createUserAdminApi(apiClient);
const contactTagApi = createContactTagApi(apiClient);
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
    '/passwordUpdate/index': (
      <PasswordPage
        api={passwordApi}
        onUpdated={() => {
          authStore.clearSession();
          void routerRef.current?.navigate('/login');
        }}
      />
    ),
    '/workEmployee/index': <EmployeePage api={employeeApi} />,
    '/department/index': (
      <DepartmentPage
        api={{
          ...departmentApi,
          conditions: () => employeeApi.conditions(),
          sync: () => employeeApi.sync(),
        }}
      />
    ),
    '/contactField/index': <ContactFieldPage api={contactFieldApi} />,
    '/role/index': <RolePage api={roleApi}
      navigate={(path) => void routerRef.current?.navigate(path)} />,
    '/menu/index': <MenuAdminPage api={menuAdminApi} />,
    '/user/index': <UserAdminPage api={userAdminApi} />,
    '/workContactTag/index': <ContactTagPage api={contactTagApi} />,
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
