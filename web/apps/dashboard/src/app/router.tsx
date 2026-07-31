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

import type { AccessContext, CorpSelection } from './access-loader';
import { DashboardAccessProvider } from './access-context';
import { DashboardLayout } from '../layout/dashboard-layout';
import { NotFoundPage } from '../pages/not-found-page';
import { RoutedLoginPage } from '../features/auth/login-page';
import { AppErrorPage } from '../pages/app-error-page';
import { ForbiddenPage } from '../pages/forbidden-page';

export type DashboardRouterDeps = {
  getSession: () => Session | null;
  loadInitialData: () => Promise<void>;
  initialEntries?: string[];
  reactPages?: Readonly<Record<string, ReactNode>>;
  authenticate?: (input: { phone: string; password: string }) => Promise<Session>;
  setSession?: (session: Session) => void;
  accessLoader?: (args: { request: Request }) => Promise<
    AccessContext | CorpSelection
  >;
  renderAccess?: (
    data: AccessContext | CorpSelection,
    children: ReactNode,
  ) => ReactNode;
};

function DashboardHomePage() {
  return (
    <section>
      <h1>管理后台</h1>
      <p>请选择已迁移的功能页面。</p>
    </section>
  );
}

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
  const data = useLoaderData<AccessContext | CorpSelection | null>();
  if (data === null) {
    return <DashboardLayout />;
  }
  if ('state' in data) {
    return renderAccess?.(data, <DashboardHomePage />) ?? <DashboardHomePage />;
  }
  const shell = (
    <DashboardAccessProvider value={data}>
      <DashboardLayout />
    </DashboardAccessProvider>
  );
  return renderAccess?.(data, shell) ?? shell;
}

export function createDashboardRouter(deps: DashboardRouterDeps) {
  const reactRoutes = Object.entries(deps.reactPages ?? {}).map(([path, element]) => ({
    path,
    element,
  }));
  const routes = [
    {
      path: '/login',
      element: (
        <RoutedLoginPage
          authenticate={deps.authenticate ?? (() => Promise.reject(
            new Error('登录服务尚未配置'),
          ))}
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
        { index: true, element: <Navigate replace to="/index" /> },
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
