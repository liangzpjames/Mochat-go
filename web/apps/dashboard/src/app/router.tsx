import type { Session } from '@mochat/auth';
import { ErrorState, LoadingState } from '@mochat/ui';
import type { ReactNode } from 'react';
import {
  createBrowserRouter,
  createMemoryRouter,
  Navigate,
  redirect,
  useRouteError,
} from 'react-router';

import migrationRoutesJson from '../migration-routes.json';
import { DashboardLayout } from '../layout/dashboard-layout';
import { NotFoundPage } from '../pages/not-found-page';
import { parseRouteManifest } from '@mochat/routing';

export type DashboardRouterDeps = {
  getSession: () => Session | null;
  loadInitialData: () => Promise<void>;
  initialEntries?: string[];
  reactPages?: Readonly<Record<string, ReactNode>>;
};

function LoginPlaceholderPage() {
  return (
    <section>
      <h1>登录</h1>
      <p>登录表单将在身份接入任务中启用。</p>
    </section>
  );
}

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
  const message = error instanceof Error ? error.message : '初始化数据加载失败';
  return <ErrorState message={message} title="加载失败" />;
}

function DashboardHydrateFallback() {
  return <LoadingState label="正在初始化管理后台…" />;
}

export function createDashboardRouter(deps: DashboardRouterDeps) {
  const manifest = parseRouteManifest(migrationRoutesJson);
  const reactRoutes = manifest
    .filter((route) => route.target === 'react')
    .flatMap((route) => {
      const element = deps.reactPages?.[route.path];
      return element === undefined ? [] : [{ path: route.path, element }];
    });
  const routes = [
    {
      path: '/login',
      element: <LoginPlaceholderPage />,
    },
    {
      path: '/',
      loader: async () => {
        if (deps.getSession() === null) {
          return redirect('/login');
        }
        await deps.loadInitialData();
        return null;
      },
      element: <DashboardLayout />,
      errorElement: <DashboardRouteError />,
      HydrateFallback: DashboardHydrateFallback,
      children: [
        { index: true, element: <DashboardHomePage /> },
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
