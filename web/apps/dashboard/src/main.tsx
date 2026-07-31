import { createRoot } from 'react-dom/client';
import { lazy, Suspense, type ReactNode } from 'react';

import { createAuthStore } from '@mochat/auth';
import { createApiClient } from '@mochat/api-client';
import {
  parseRouteManifest,
} from '@mochat/routing';

import migrationRoutesJson from './migration-routes.json';
import { benchmarkManifest } from './benchmark/benchmark-manifest';
import {
  createBenchmarkP0Pages,
  createPageRegistry,
} from './benchmark/page-registry';
import { createAccessLoader } from './app/access-loader';
import { createDashboardQueryClient, DashboardProviders } from './app/providers';
import { createDashboardRouter } from './app/router';
import {
  authenticate,
  logout as invalidateServerSession,
} from './features/auth/auth-api';
import {
  createLogoutAction,
  DashboardSessionActionsProvider,
} from './features/auth/session-actions';
import {
  bindCorp,
  loadCorps,
} from './features/corp/corp-api';
import { createCorpAdminApi } from './features/corp/corp-admin-api';
import { CorpProvider } from './features/corp/corp-provider';
import { loadMenu } from './features/navigation/menu-api';
import { buildMenuAccess } from './features/navigation/menu-tree';
import { createPasswordApi } from './features/password/password-api';
import { createEmployeeApi } from './features/employee/employee-api';
import { createDepartmentApi } from './features/department/department-api';
import { createContactFieldApi } from './features/contact-field/contact-field-api';
import { createRoleApi } from './features/role/role-api';
import { createMenuAdminApi } from './features/menu-admin/menu-admin-api';
import { createUserAdminApi } from './features/user-admin/user-admin-api';
import { createContactTagApi } from './features/contact-tag/contact-tag-api';
import { createBusinessWorkbenchApi } from './features/business-workbench/business-workbench-api';
import { BusinessWorkbenchPage } from './features/business-workbench/business-workbench-page';
import { businessRouteCatalog } from './features/business-workbench/catalog';
import { createDashboardOverviewApi } from './features/dashboard-overview/dashboard-overview-api';
import { createConversationGlobalApi } from './features/conversation-global/conversation-global-api';
import { createSensitiveWordApi } from './features/sensitive-word/sensitive-word-api';
import { createLeadApi } from './features/scrm/lead-api';
import { createScrmApi } from './features/scrm/scrm-api';
import { createContactApi } from './features/scrm/contact-api';
import './styles/index.css';

const CorpPage = lazy(async () => ({ default: (await import('./features/corp/corp-page')).CorpPage }));
const PasswordPage = lazy(async () => ({ default: (await import('./features/password/password-page')).PasswordPage }));
const EmployeePage = lazy(async () => ({ default: (await import('./features/employee/employee-page')).EmployeePage }));
const DepartmentPage = lazy(async () => ({ default: (await import('./features/department/department-page')).DepartmentPage }));
const ContactFieldPage = lazy(async () => ({ default: (await import('./features/contact-field/contact-field-page')).ContactFieldPage }));
const RolePage = lazy(async () => ({ default: (await import('./features/role/role-page')).RolePage }));
const RolePermissionPage = lazy(async () => ({
  default: (await import('./features/role/role-permission-page')).RolePermissionPage,
}));
const MenuAdminPage = lazy(async () => ({ default: (await import('./features/menu-admin/menu-admin-page')).MenuAdminPage }));
const UserAdminPage = lazy(async () => ({ default: (await import('./features/user-admin/user-admin-page')).UserAdminPage }));
const ContactTagPage = lazy(async () => ({ default: (await import('./features/contact-tag/contact-tag-page')).ContactTagPage }));
const page = (content: ReactNode) => <Suspense fallback={null}>{content}</Suspense>;

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
const businessWorkbenchApi = createBusinessWorkbenchApi(apiClient);
const dashboardOverviewApi = createDashboardOverviewApi(apiClient);
const conversationGlobalApi = createConversationGlobalApi(
  apiClient,
  () => authStore.getSession()?.corpId ?? null,
);
const sensitiveWordApi = createSensitiveWordApi(apiClient);
const leadApi = createLeadApi(apiClient);
const scrmApi = createScrmApi(apiClient);
const contactApi = createContactApi(apiClient);
const migratedPages = Object.fromEntries(
  Object.entries(businessRouteCatalog).map(([path, config]) => [
    path,
    page(
      <BusinessWorkbenchPage
        config={config}
        api={businessWorkbenchApi}
        navigate={(target) => void routerRef.current?.navigate(target)}
      />,
    ),
  ]),
);
const migrationManifest = parseRouteManifest(migrationRoutesJson);
const knownRoutes = new Set([
  '/',
  ...migrationManifest.map((route) => route.path),
  ...benchmarkManifest.pages.map((page) => page.path),
]);
const loadAccess = createAccessLoader({
  clearSession: () => authStore.clearSession(),
  getSession: () => authStore.getSession(),
  knownRoutes,
  benchmarkRoutes: new Set(benchmarkManifest.pages.map((page) => page.path)),
  loadCorps: () => loadCorps(apiClient),
  loadMenu: () => loadMenu(apiClient),
});
const accessLoader = ({ request }: { request: Request }) => loadAccess({ request });
const routerRef: { current?: ReturnType<typeof createDashboardRouter> } = {};
const performLogout = createLogoutAction({
  clearQueries: () => queryClient.clear(),
  clearSession: () => authStore.clearSession(),
  invalidateServerSession: () => invalidateServerSession(apiClient),
  navigateToLogin: () => {
    void routerRef.current?.navigate('/login');
  },
});
const router = createDashboardRouter({
  accessLoader,
  authenticate: (input) => authenticate(loginClient, input),
  getSession: () => authStore.getSession(),
  loadInitialData: () => Promise.resolve(),
  reactPages: {
    ...migratedPages,
    ...createPageRegistry({
      manifest: benchmarkManifest,
      p0Pages: createBenchmarkP0Pages({ dashboardOverviewApi, conversationGlobalApi, sensitiveWordApi, leadApi, scrmApi, contactApi }),
      p1Pages: {},
    }),
    '/corp/index': page(<CorpPage api={corpAdminApi} />),
    '/passwordUpdate/index': (
      page(<PasswordPage
        api={passwordApi}
        onUpdated={() => {
          authStore.clearSession();
          void routerRef.current?.navigate('/login');
        }}
      />)
    ),
    '/workEmployee/index': page(<EmployeePage api={employeeApi} />),
    '/department/index': (
      page(<DepartmentPage
        api={{
          ...departmentApi,
          conditions: () => employeeApi.conditions(),
          sync: () => employeeApi.sync(),
        }}
      />)
    ),
    '/contactField/index': page(<ContactFieldPage api={contactFieldApi} />),
    '/role/index': page(<RolePage api={roleApi}
      navigate={(path) => void routerRef.current?.navigate(path)} />),
    '/role/permissionShow': page(
      <RolePermissionPage
        api={roleApi}
        navigate={(path) => void routerRef.current?.navigate(path)}
      />,
    ),
    '/menu/index': page(<MenuAdminPage api={menuAdminApi} />),
    '/user/index': page(<UserAdminPage api={userAdminApi} />),
    '/workContactTag/index': page(<ContactTagPage api={contactTagApi} />),
  },
  renderAccess: (access, children) => (
    <DashboardSessionActionsProvider
      onLogout={performLogout}
      userId={authStore.getSession()?.userId ?? null}
    >
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
        refreshAccess={() => {
          void routerRef.current?.revalidate();
        }}
      >
        {children}
      </CorpProvider>
    </DashboardSessionActionsProvider>
  ),
  setSession: (session) => authStore.setSession(session),
});
routerRef.current = router;

createRoot(rootElement).render(
  <DashboardProviders queryClient={queryClient} router={router} />,
);
