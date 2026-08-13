import {
  MobileShell,
  MobileState,
  safeInternalTarget,
} from '@mochat/mobile-foundation';
import { useEffect, useRef, useState, type ReactNode } from 'react';
import {
  createBrowserRouter,
  Navigate,
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
  type CookieAdapter,
} from '../auth/sidebar-session';
import { ContactPage } from '../features/contact/contact-page';
import {
  sidebarRouteRegistry,
  type SidebarRouteRegistration,
} from '../routes/registry';

export type SidebarRequest = <T>(path: string, init?: RequestInit) => Promise<T>;

export type SidebarRuntime = {
  basename: string;
  cookies: CookieAdapter;
  origin: string;
  request: SidebarRequest;
  secure: boolean;
};

function currentTarget(pathname: string, search: string, hash: string): string {
  return safeInternalTarget(`${pathname}${search}${hash}`, '/');
}

function SidebarAuthBoundary({ runtime, children }: {
  runtime: SidebarRuntime;
  children: ReactNode;
}) {
  const location = useLocation();
  const session = readSidebarSession(runtime.cookies);
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
        runtime.cookies,
        runtime.secure,
        runtime.origin,
      ),
    });
  }, [callbackKey, params, runtime.cookies, runtime.origin, runtime.secure]);

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
  return (
    <MobileShell
      appName="MoChat 客户侧边栏"
      title={route.title}
      subtitle={route.description}
    >
      <MobileState
        kind="empty"
        title={`${route.title}模块待迁移`}
        description="该历史入口已由新路由承接，业务功能将在后续迁移。"
      />
    </MobileShell>
  );
}

function SidebarContactPage({ runtime }: { runtime: SidebarRuntime }) {
  const location = useLocation();
  const navigate = useNavigate();
  const onReauthenticate = () => {
    const session = readSidebarSession(runtime.cookies);
    const queryAgentId = new URLSearchParams(location.search).get('agentId');
    clearSidebarSession(runtime.cookies, runtime.secure);
    const loginQuery = new URLSearchParams({
      agentId: session.agentId ?? queryAgentId ?? '',
      target: currentTarget(location.pathname, location.search, location.hash),
    });
    void navigate(`/login?${loginQuery.toString()}`, { replace: true });
  };
  return <ContactPage request={runtime.request} onReauthenticate={onReauthenticate} />;
}

function routeElement(route: SidebarRouteRegistration, runtime: SidebarRuntime): ReactNode {
  if (route.moduleKey === 'sidebar-login') return <SidebarLoginPage />;
  if (route.moduleKey === 'sidebar-auth-callback') {
    return <SidebarAuthCallbackPage runtime={runtime} />;
  }
  const content = route.moduleKey === 'contact-summary'
    ? <SidebarContactPage runtime={runtime} />
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
