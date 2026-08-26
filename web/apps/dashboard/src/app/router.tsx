import type { Session } from '@mochat/auth';
import { LoadingState } from '@mochat/ui';
import type { ReactNode } from 'react';
import {
  createBrowserRouter,
  createMemoryRouter,
  Navigate,
  redirect,
  isRouteErrorResponse,
  useLoaderData,
  useRevalidator,
  useRouteError,
} from 'react-router';

import type { AccessContext } from './access-loader';
import {
  DashboardAccessProvider,
  useOptionalDashboardAccess,
} from './access-context';
import { DashboardLayout } from '../layout/dashboard-layout';
import { NotFoundPage } from '../pages/not-found-page';
import { RoutedActivationPage } from '../features/auth/activation-page';
import { RoutedLoginPage } from '../features/auth/login-page';
import type {
  ActivationInput,
  DashboardActivationStatus,
  DashboardAuthResult,
  LoginInput,
  MFAInput,
} from '../features/auth/auth-api';
import { AppErrorPage } from '../pages/app-error-page';
import { ForbiddenPage } from '../pages/forbidden-page';

export type DashboardRouterDeps = {
  getSession: () => Session | null;
  loadInitialData: () => Promise<void>;
  initialEntries?: string[];
  reactPages?: Readonly<Record<string, ReactNode>>;
  authenticate?: (input: LoginInput) => Promise<DashboardAuthResult>;
  completeMFA?: (input: MFAInput) => Promise<DashboardAuthResult>;
  activate?: (input: ActivationInput) => Promise<void>;
  inspectActivation?: (token: string) => Promise<DashboardActivationStatus>;
  setSession?: (session: Session) => void;
  accessLoader?: (args: { request: Request }) => Promise<AccessContext>;
  renderAccess?: (
    data: AccessContext,
    children: ReactNode,
  ) => ReactNode;
};

function DashboardRouteError() {
  const error = useRouteError();
  const revalidator = useRevalidator();
  const status = isRouteErrorResponse(error)
    ? error.status
    : error instanceof Response
      ? error.status
      : null;
  if (status === 403) {
    return <ForbiddenPage />;
  }
  if (status === 404) {
    return <NotFoundPage />;
  }
  const message = error instanceof Error ? error.message : '初始化数据加载失败';
  return (
    <AppErrorPage
      message={message}
      retry={() => void revalidator.revalidate()}
    />
  );
}

function DashboardHydrateFallback() {
  return <LoadingState label="正在初始化管理后台…" />;
}

function DashboardAccessShell({ renderAccess }: {
  renderAccess?: DashboardRouterDeps['renderAccess'];
}) {
  const data = useLoaderData<AccessContext | null>();
  if (data === null) {
    return <DashboardLayout />;
  }
  const shell = (
    <DashboardAccessProvider value={data}>
      <DashboardLayout />
    </DashboardAccessProvider>
  );
  return renderAccess?.(data, shell) ?? shell;
}

export function dashboardLandingRoute(allowedRoutes: ReadonlySet<string>): string | null {
  if (allowedRoutes.has('/index')) return '/index';
  return allowedRoutes.values().next().value ?? null;
}

function DashboardLandingPage() {
  const access = useOptionalDashboardAccess();
  const target = dashboardLandingRoute(access?.allowedRoutes ?? new Set());
  return target === null ? <ForbiddenPage /> : <Navigate replace to={target} />;
}

export function createDashboardRouter(deps: DashboardRouterDeps) {
  const reactRoutes = Object.entries(deps.reactPages ?? {}).map(([path, element]) => ({
    path,
    element,
  }));
  const routes = [
    {
      path: '/activate',
      element: (
        <RoutedActivationPage
          activate={deps.activate ?? (() => Promise.reject(
            new Error('Dashboard activation is not configured'),
          ))}
          inspect={deps.inspectActivation ?? (() => Promise.resolve({
            status: 'invalid',
            tenantName: '',
            accountHint: '',
            expiresAt: 0,
            primaryAction: 'contact_admin',
          }))}
        />
      ),
    },
    {
      path: '/login',
      element: (
        <RoutedLoginPage
          authenticate={deps.authenticate ?? (() => Promise.reject(
            new Error('登录服务尚未配置'),
          ))}
          {...(deps.completeMFA === undefined ? {} : { completeMFA: deps.completeMFA })}
          setSession={deps.setSession ?? (() => undefined)}
        />
      ),
    },
    {
      path: '/',
      loader: async ({ request }: { request: Request }) => {
        if (deps.accessLoader !== undefined) {
          return deps.accessLoader({ request });
        }
        if (deps.getSession() === null) {
          return redirect('/login');
        }
        await deps.loadInitialData();
        return null;
      },
      element: (
        <DashboardAccessShell renderAccess={deps.renderAccess} />
      ),
      errorElement: <DashboardRouteError />,
      HydrateFallback: DashboardHydrateFallback,
      children: [
        { index: true, element: <DashboardLandingPage /> },
        { path: 'corpData/index', element: <Navigate replace to="/index" /> },
        ...reactRoutes,
        { path: '*', element: <NotFoundPage /> },
      ],
    },
    {
      path: '*',
      element: <Navigate replace to="/" />,
    },
  ];

  return deps.initialEntries === undefined
    ? createBrowserRouter(routes)
    : createMemoryRouter(routes, { initialEntries: deps.initialEntries });
}
