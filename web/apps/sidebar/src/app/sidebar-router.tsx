import {
  MobileShell,
  MobileState,
  safeInternalTarget,
} from '@mochat/mobile-foundation';
import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react';
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
  type SidebarSessionAdapter,
} from '../auth/sidebar-session';
import { parseLegacyCodeAuth } from '../auth/code-auth';
import { BatchAddPage, ContactSopPage, MediumPage, RoomSopPage } from '../features/business-pages';
import { ContactEditPage } from '../features/contact/contact-edit-page';
import { ContactPage } from '../features/contact/contact-page';
import { ContactRemarkPage } from '../features/contact/contact-remark-page';
import { ContactTagPage } from '../features/contact/contact-tag-page';
import { WorkbenchPage } from '../features/workbench/workbench-page';
import {
  sidebarRouteRegistry,
  type SidebarRouteRegistration,
} from '../routes/registry';
import { SidebarPageShell } from '../ui/sidebar-page-shell';
import type { WeComBridge } from '../wecom/wecom-bridge';

export type SidebarRequest = <T>(path: string, init?: RequestInit) => Promise<T>;

export type SidebarRuntime = {
  basename: string;
  session: SidebarSessionAdapter;
  origin: string;
  request: SidebarRequest;
  bridge: WeComBridge;
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

function SidebarCodeAuthPage() {
  const [params] = useSearchParams();
  const result = parseLegacyCodeAuth(params);
  if (result.ok) {
    const login = new URLSearchParams({ agentId: result.agentId, target: result.target });
    return <Navigate replace to={`/login?${login.toString()}`} />;
  }
  return <MobileShell appName="MoChat 客户侧边栏" title="企业微信扫码授权"><MobileState description={result.message} kind="error" title="兼容授权参数无效" /></MobileShell>;
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

function useSidebarActions(runtime: SidebarRuntime) {
  const location = useLocation();
  const navigate = useNavigate();
  const reauthenticating = useRef(false);
  const onReauthenticate = useCallback(() => {
    if (reauthenticating.current) return;
    reauthenticating.current = true;
    const session = readSidebarSession(runtime.session);
    const queryAgentId = new URLSearchParams(location.search).get('agentId');
    clearSidebarSession(runtime.session);
    const loginQuery = new URLSearchParams({
      agentId: session.agentId ?? queryAgentId ?? '',
      target: currentTarget(location.pathname, location.search, location.hash),
    });
    void navigate(`/login?${loginQuery.toString()}`, { replace: true });
  }, [location.hash, location.pathname, location.search, navigate, runtime.session]);
  const onContactDone = useCallback(() => {
    const params = new URLSearchParams(location.search);
    const kept = new URLSearchParams();
    for (const key of ['wxExternalUserid', 'agentId']) {
      const value = params.get(key); if (value) kept.set(key, value);
    }
    void navigate(`/contact${kept.size ? `?${kept.toString()}` : ''}`, { replace: true });
  }, [location.search, navigate]);
  return { onReauthenticate, onContactDone };
}

function SidebarBusinessPage({ runtime, route }: { runtime: SidebarRuntime; route: SidebarRouteRegistration }) {
  const actions = useSidebarActions(runtime);
  let content: ReactNode;
  switch (route.moduleKey) {
    case 'contact-edit-detail': content = <ContactEditPage request={runtime.request} onDone={actions.onContactDone} onReauthenticate={actions.onReauthenticate} />; break;
    case 'contact-remark': content = <ContactRemarkPage request={runtime.request} onDone={actions.onContactDone} onReauthenticate={actions.onReauthenticate} />; break;
    case 'contact-setting-tag': content = <ContactTagPage request={runtime.request} onDone={actions.onContactDone} onReauthenticate={actions.onReauthenticate} />; break;
    case 'contact-batch-add': content = <BatchAddPage bridge={runtime.bridge} request={runtime.request} onReauthenticate={actions.onReauthenticate} />; break;
    case 'contact-sop': content = <ContactSopPage request={runtime.request} onReauthenticate={actions.onReauthenticate} />; break;
    case 'medium-library': content = <MediumPage bridge={runtime.bridge} request={runtime.request} onReauthenticate={actions.onReauthenticate} />; break;
    case 'room-sop': content = <RoomSopPage request={runtime.request} onReauthenticate={actions.onReauthenticate} />; break;
    default: content = null;
  }
  return <SidebarPageShell subtitle={route.description} title={route.title}>{content}</SidebarPageShell>;
}

function SidebarWorkbenchPage({ runtime }: { runtime: SidebarRuntime }) {
  const actions = useSidebarActions(runtime);
  return <WorkbenchPage bridge={runtime.bridge} onReauthenticate={actions.onReauthenticate} request={runtime.request} />;
}

function routeElement(route: SidebarRouteRegistration, runtime: SidebarRuntime): ReactNode {
  if (route.moduleKey === 'sidebar-login') return <SidebarLoginPage />;
  if (route.moduleKey === 'sidebar-auth-callback') {
    return <SidebarAuthCallbackPage runtime={runtime} />;
  }
  if (route.moduleKey === 'sidebar-code-auth') return <SidebarCodeAuthPage />;
  const content = route.moduleKey === 'contact-summary'
    ? <SidebarContactPage runtime={runtime} />
    : route.moduleKey === 'sidebar-home'
      ? <SidebarWorkbenchPage runtime={runtime} />
      : <SidebarBusinessPage route={route} runtime={runtime} />;
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
