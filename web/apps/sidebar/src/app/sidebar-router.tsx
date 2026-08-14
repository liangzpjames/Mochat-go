import {
  MobileCard,
  MobileIconTile,
  MobileShell,
  MobileState,
  safeInternalTarget,
} from '@mochat/mobile-foundation';
import { useEffect, useRef, useState, type ReactNode } from 'react';
import {
  createBrowserRouter,
  Navigate,
  useHref,
  useLocation,
  useNavigate,
  useSearchParams,
} from 'react-router';

import {
  clearSidebarSession,
  completeSidebarAuthCallback,
  readSidebarSession,
  sidebarLoginHref,
  type SidebarAuthCallbackResult,
  type SidebarSessionAdapter,
} from '../auth/sidebar-session';
import { ContactPage } from '../features/contact/contact-page';
import {
  sidebarRouteRegistry,
  type SidebarRouteRegistration,
} from '../routes/registry';
import { SidebarPageShell, sidebarBusinessContextSuffix } from '../ui/sidebar-page-shell';

export type SidebarRequest = <T>(path: string, init?: RequestInit) => Promise<T>;

export type SidebarRuntime = {
  basename: string;
  session: SidebarSessionAdapter;
  origin: string;
  request: SidebarRequest;
};

function currentTarget(pathname: string, search: string, hash: string): string {
  return safeInternalTarget(`${pathname}${search}${hash}`, '/');
}

function SidebarAuthBoundary({ runtime, children }: {
  runtime: SidebarRuntime;
  children: ReactNode;
}) {
  const location = useLocation();
  const session = readSidebarSession(runtime.session);
  if (session.token) return children;

  const queryAgentId = new URLSearchParams(location.search).get('agentId');
  const loginQuery = new URLSearchParams({
    agentId: session.agentId ?? queryAgentId ?? '',
    target: currentTarget(location.pathname, location.search, location.hash),
  });
  return <Navigate replace to={`/login?${loginQuery.toString()}`} />;
}

function SidebarLoginPage() {
  const [params] = useSearchParams();
  const agentId = params.get('agentId')?.trim() ?? '';
  const target = safeInternalTarget(params.get('target'), '/');

  return (
    <MobileShell
      appName="MoChat 客户侧边栏"
      title="侧边栏登录"
      subtitle="使用企业微信员工身份继续访问。"
    >
      {/^\d+$/.test(agentId) && agentId !== '0' ? (
        <a className="sidebar-primary-link" href={sidebarLoginHref(agentId, target)}>
          继续授权
        </a>
      ) : (
        <MobileState
          kind="error"
          title="登录参数错误"
          description="缺少有效的企业应用 ID。"
        />
      )}
    </MobileShell>
  );
}

function SidebarAuthCallbackPage({ runtime }: { runtime: SidebarRuntime }) {
  const [params] = useSearchParams();
  const callbackKey = params.toString();
  const completedCallback = useRef<string | null>(null);
  const [outcome, setOutcome] = useState<{
    key: string;
    result: SidebarAuthCallbackResult;
  } | null>(null);

  useEffect(() => {
    if (completedCallback.current === callbackKey) return;
    completedCallback.current = callbackKey;
    setOutcome({
      key: callbackKey,
      result: completeSidebarAuthCallback(
        params,
        runtime.session,
        runtime.origin,
        runtime.basename,
      ),
    });
  }, [callbackKey, params, runtime.basename, runtime.origin, runtime.session]);

  if (outcome === null || outcome.key !== callbackKey) {
    return (
      <MobileShell appName="MoChat 客户侧边栏" title="企业微信授权回调">
        <MobileState
          kind="loading"
          title="正在处理授权"
          description="正在验证企业微信员工登录状态。"
        />
      </MobileShell>
    );
  }

  const { result } = outcome;
  if (result.ok) return <Navigate replace to={result.target} />;

  const agentId = params.get('agentId')?.trim() ?? '';
  return (
    <MobileShell appName="MoChat 客户侧边栏" title="企业微信授权回调">
      <MobileState
        kind="error"
        title="登录失败"
        description={result.message}
      />
      {/^\d+$/.test(agentId) && agentId !== '0' ? (
        <a className="sidebar-primary-link" href={sidebarLoginHref(agentId, result.target)}>
          重新授权
        </a>
      ) : null}
    </MobileShell>
  );
}

function PendingModulePage({ route }: { route: SidebarRouteRegistration }) {
  const content = (
    <MobileCard padding="comfortable" tone="surface">
      <MobileState
        kind="empty"
        title={`${route.title}模块待迁移`}
        description="该历史入口已由新路由承接，业务功能将在后续迁移。"
      />
    </MobileCard>
  );

  if (!route.auth) {
    return (
      <MobileShell appName="MoChat 客户侧边栏" title={route.title} subtitle={route.description}>
        {content}
      </MobileShell>
    );
  }

  return (
    <SidebarPageShell title={route.title} subtitle={route.description}>
      {content}
    </SidebarPageShell>
  );
}

function WorkbenchIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      <path d="M5 5.5h5v5H5zM14 5.5h5v5h-5zM5 14h5v5H5zM14 14h5v5h-5z" fill="none" stroke="currentColor" strokeLinejoin="round" strokeWidth="1.7" />
    </svg>
  );
}

function SidebarWorkbenchTile({
  route,
  suffix,
}: {
  route: SidebarRouteRegistration;
  suffix: string;
}) {
  const href = useHref(`${route.path}${suffix}`);
  return (
    <MobileIconTile
      description={route.description}
      href={href}
      icon={<WorkbenchIcon />}
      title={route.title}
    />
  );
}

function SidebarWorkbenchPage() {
  const location = useLocation();
  const routes = sidebarRouteRegistry.filter((route) => route.auth && route.path !== '/');
  const suffix = sidebarBusinessContextSuffix(location.search, location.hash);
  return (
    <SidebarPageShell
      eyebrow="员工工作台"
      hero={<MobileCard padding="comfortable" tone="accent"><p className="sidebar-workbench__hero-kicker">企业微信工作台</p><h2>从当前客户会话开始工作</h2><p>已为你保留现有业务入口。</p></MobileCard>}
      subtitle="查看当前会话客户与可用工作模块。"
      title="客户侧边栏"
    >
      <section aria-label="可用工作模块" className="sidebar-workbench__tiles">
        {routes.map((route) => (
          <SidebarWorkbenchTile
            key={route.moduleKey}
            route={route}
            suffix={suffix}
          />
        ))}
      </section>
    </SidebarPageShell>
  );
}

function SidebarContactPage({ runtime }: { runtime: SidebarRuntime }) {
  const location = useLocation();
  const navigate = useNavigate();
  const onReauthenticate = () => {
    const session = readSidebarSession(runtime.session);
    const queryAgentId = new URLSearchParams(location.search).get('agentId');
    clearSidebarSession(runtime.session);
    const loginQuery = new URLSearchParams({
      agentId: session.agentId ?? queryAgentId ?? '',
      target: currentTarget(location.pathname, location.search, location.hash),
    });
    void navigate(`/login?${loginQuery.toString()}`, { replace: true });
  };
  return (
    <SidebarPageShell title="客户资料" subtitle="当前会话客户摘要">
      <ContactPage request={runtime.request} onReauthenticate={onReauthenticate} />
    </SidebarPageShell>
  );
}

function routeElement(route: SidebarRouteRegistration, runtime: SidebarRuntime): ReactNode {
  if (route.moduleKey === 'sidebar-login') return <SidebarLoginPage />;
  if (route.moduleKey === 'sidebar-auth-callback') {
    return <SidebarAuthCallbackPage runtime={runtime} />;
  }
  const content = route.moduleKey === 'contact-summary'
    ? <SidebarContactPage runtime={runtime} />
    : route.moduleKey === 'sidebar-home'
      ? <SidebarWorkbenchPage />
      : <PendingModulePage route={route} />;
  return route.auth ? (
    <SidebarAuthBoundary runtime={runtime}>{content}</SidebarAuthBoundary>
  ) : content;
}

function SidebarNotFoundPage() {
  return (
    <MobileShell appName="MoChat 客户侧边栏" title="未找到页面">
      <MobileState
        kind="not-found"
        title="页面不存在"
        description="请检查访问地址。"
      />
    </MobileShell>
  );
}

export function createSidebarRouter(runtime: SidebarRuntime) {
  return createBrowserRouter([
    ...sidebarRouteRegistry.map((route) => ({
      path: route.path,
      element: routeElement(route, runtime),
    })),
    { path: '*', element: <SidebarNotFoundPage /> },
  ], { basename: runtime.basename });
}
