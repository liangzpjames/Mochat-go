import type { QueryClient } from '@tanstack/react-query';
import {
  type ReactNode,
  useCallback,
  useEffect,
  useRef,
  useState,
} from 'react';

import type { CorpOption } from './corp-api';
import { useDashboardSessionActions } from '../auth/session-actions';

export type CorpProviderProps = {
  bindCorp: (corpId: string) => Promise<void>;
  children: ReactNode;
  initialCorpId: string | null;
  initialCorps?: readonly CorpOption[];
  loadCorps: () => Promise<readonly CorpOption[]>;
  loadMenuAccess: (corpId: string) => Promise<{ firstRoute: string }>;
  navigate: (route: string) => void;
  persistCorpId: (corpId: string) => void;
  queryClient: QueryClient;
  refreshAccess: () => void;
};

export function CorpProvider({
  bindCorp,
  children,
  initialCorpId,
  initialCorps,
  loadCorps,
  loadMenuAccess,
  navigate,
  persistCorpId,
  queryClient,
  refreshAccess,
}: CorpProviderProps) {
  const sessionActions = useDashboardSessionActions();
  const [corps, setCorps] = useState<readonly CorpOption[] | null>(
    initialCorps ?? null,
  );
  const [activeCorpId, setActiveCorpId] = useState(initialCorpId);
  const [error, setError] = useState<string | null>(null);
  const [isSwitching, setIsSwitching] = useState(false);
  const automaticSelectionStarted = useRef(false);
  const switching = useRef(false);

  const switchCorp = useCallback(async (nextCorpId: string) => {
    if (switching.current) return;
    switching.current = true;
    setIsSwitching(true);
    setError(null);
    const previousCorpId = activeCorpId;
    try {
      if (previousCorpId !== null) {
        await queryClient.cancelQueries({ queryKey: ['corp', previousCorpId] });
      }
      await bindCorp(nextCorpId);
      persistCorpId(nextCorpId);
      setActiveCorpId(nextCorpId);
      if (previousCorpId !== null) {
        queryClient.removeQueries({ queryKey: ['corp', previousCorpId] });
      }
      const menu = await loadMenuAccess(nextCorpId);
      if (!preserveCurrentDeepLink()) {
        navigate(menu.firstRoute);
      }
      refreshAccess();
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : '企业切换失败');
    } finally {
      switching.current = false;
      setIsSwitching(false);
    }
  }, [
    activeCorpId,
    bindCorp,
    loadMenuAccess,
    navigate,
    persistCorpId,
    queryClient,
    refreshAccess,
  ]);

  function preserveCurrentDeepLink(): boolean {
    const pathname = window.location.pathname;
    return pathname !== '/' && pathname !== '/login' && pathname !== '/corpData/index';
  }

  useEffect(() => {
    let active = true;
    const acceptOptions = (options: readonly CorpOption[]) => {
      if (!active) return;
      if (initialCorps === undefined) {
        setCorps(options);
      }
      const selectable = options.filter((option) => option.authorized);
      if (
        initialCorpId === null
        && options.length === 1
        && selectable.length === 1
        && !automaticSelectionStarted.current
      ) {
        automaticSelectionStarted.current = true;
        const onlyCorp = selectable[0];
        if (onlyCorp !== undefined) {
          void switchCorp(onlyCorp.id);
        }
      }
    };
    if (initialCorps !== undefined) {
      acceptOptions(initialCorps);
    } else {
      void loadCorps()
        .then(acceptOptions)
      .catch((reason: unknown) => {
        if (active) {
          setError(reason instanceof Error ? reason.message : '企业加载失败');
        }
      });
    }
    return () => {
      active = false;
    };
  }, [initialCorpId, initialCorps, loadCorps, switchCorp]);

  if (error !== null && corps === null) {
    return <div role="alert">{error}</div>;
  }
  if (corps === null) {
    return <div>正在加载企业…</div>;
  }
  if (corps.length === 0) {
    return (
      <main className="dashboard-empty-corp">
        <section className="dashboard-empty-corp-card">
          <span className="dashboard-empty-corp-mark" aria-hidden="true">企</span>
          <h1>暂无可用企业</h1>
          <p>当前账号还未分配企业权限，请联系管理员完成授权后重新登录。</p>
          {sessionActions.userId !== null && (
            <span className="dashboard-empty-corp-account">
              当前账号：{sessionActions.userId}
            </span>
          )}
          <button
            className="dashboard-logout-button dashboard-logout-button-primary"
            disabled={sessionActions.isLoggingOut}
            onClick={() => void sessionActions.logout()}
            type="button"
          >
            {sessionActions.isLoggingOut ? '正在退出…' : '退出登录'}
          </button>
        </section>
      </main>
    );
  }

  return (
    <div className="dashboard-corp-frame">
      {corps.length > 1 && (
        <label className="dashboard-corp-switcher">
          <span>当前企业</span>
          <select
            aria-label="企业选择"
            disabled={isSwitching}
            value={activeCorpId ?? ''}
            onChange={(event) => void switchCorp(event.target.value)}
          >
            <option disabled value="">请选择企业</option>
            {corps.map((corp) => (
              <option disabled={!corp.authorized} key={corp.id} value={corp.id}>
                {corp.name}
              </option>
            ))}
          </select>
        </label>
      )}
      {error !== null && <div role="alert">{error}</div>}
      {activeCorpId === null
        ? <div>请选择企业</div>
        : !isSwitching && error === null
          ? children
          : null}
    </div>
  );
}
