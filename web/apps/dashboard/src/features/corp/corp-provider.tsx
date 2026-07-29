import type { QueryClient } from '@tanstack/react-query';
import {
  type ReactNode,
  useCallback,
  useEffect,
  useRef,
  useState,
} from 'react';

import type { CorpOption } from './corp-api';

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
}: CorpProviderProps) {
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
      navigate(menu.firstRoute);
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
  ]);

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
    return <div>暂无可用企业</div>;
  }

  return (
    <div className="dashboard-corp-frame">
      <div aria-label="企业选择" className="dashboard-corp-switcher">
        {corps.map((corp) => (
          <button
            disabled={
              isSwitching || !corp.authorized || corp.id === activeCorpId
            }
            key={corp.id}
            onClick={() => void switchCorp(corp.id)}
            type="button"
          >
            {corp.name}
          </button>
        ))}
      </div>
      {error !== null && <div role="alert">{error}</div>}
      {activeCorpId === null
        ? <div>请选择企业</div>
        : !isSwitching && error === null
          ? children
          : null}
    </div>
  );
}
