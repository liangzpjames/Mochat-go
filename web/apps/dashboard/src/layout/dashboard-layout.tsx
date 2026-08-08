import { useEffect, useRef, useState } from 'react';
import { NavLink, Outlet, useLocation, useNavigate } from 'react-router';

import { useOptionalDashboardAccess } from '../app/access-context';
import yuanhuManifestJson from '../benchmark/manifest.json';
import { useDashboardSessionActions } from '../features/auth/session-actions';
import {
  buildYuanhuNavigation,
  buildYuanhuTopLevelNavigation,
  type YuanhuManifest,
} from './yuanhu-navigation';

const yuanhuManifest = yuanhuManifestJson as YuanhuManifest;

export function DashboardLayout() {
  const access = useOptionalDashboardAccess();
  const location = useLocation();
  const navigate = useNavigate();
  const sessionActions = useDashboardSessionActions();
  const navigation = buildYuanhuNavigation(
    access === null ? null : {
      allowedRoutes: access.allowedRoutes,
      pathname: location.pathname,
    },
    yuanhuManifest,
  );
  const topLevelNavigation = buildYuanhuTopLevelNavigation(
    access === null ? null : {
      allowedRoutes: access.allowedRoutes,
      pathname: location.pathname,
    },
    yuanhuManifest,
  );
  const activeGroupIds = navigation
    .filter((group) => group.items.some((item) => item.activePath !== null))
    .map((group) => group.id);
  const [expandedGroups, setExpandedGroups] = useState<ReadonlySet<string>>(
    () => new Set(activeGroupIds),
  );
  const [searchQuery, setSearchQuery] = useState('');
  const searchResultsRef = useRef<HTMLElement>(null);
  const previousPathname = useRef(location.pathname);
  const hasPages = topLevelNavigation.length > 0
    || navigation.some((group) => group.items.length > 0);
  const searchableNavigation = [
    ...topLevelNavigation,
    ...navigation.flatMap((group) => group.items),
  ];
  const normalizedSearch = searchQuery.trim().toLocaleLowerCase('zh-CN');
  const searchResults = normalizedSearch === ''
    ? []
    : searchableNavigation.filter((item) => (
      item.title.toLocaleLowerCase('zh-CN').includes(normalizedSearch)
    ));

  useEffect(() => {
    if (previousPathname.current === location.pathname) {
      return;
    }
    previousPathname.current = location.pathname;
    setExpandedGroups((current) => {
      const next = new Set(current);
      activeGroupIds.forEach((groupId) => next.add(groupId));
      return next;
    });
  }, [location.pathname]);

  useEffect(() => {
    setSearchQuery('');
  }, [location.pathname]);

  function toggleGroup(groupId: string) {
    setExpandedGroups((current) => {
      const next = new Set(current);
      if (next.has(groupId)) {
        next.delete(groupId);
      } else {
        next.add(groupId);
      }
      return next;
    });
  }

  function handleSearchKeyDown(event: React.KeyboardEvent<HTMLInputElement>) {
    if (event.key === 'Escape') {
      setSearchQuery('');
      return;
    }
    const firstResult = searchResults[0];
    if (event.key === 'Enter' && firstResult !== undefined) {
      event.preventDefault();
      void navigate(firstResult.path);
      return;
    }
    if (event.key === 'ArrowDown') {
      event.preventDefault();
      searchResultsRef.current?.querySelector<HTMLAnchorElement>('a')?.focus();
    }
  }

  return (
    <div className="dashboard-shell">
      <header className="dashboard-header">
        <NavLink className="dashboard-brand" to="/">
          <span aria-hidden="true" className="dashboard-brand-mark">M</span>
          <span>MoChat AI</span>
        </NavLink>
        <span className="dashboard-product-name">企业智能运营平台</span>
        <div className="dashboard-search" role="search">
          <span className="sr-only">搜索功能</span>
          <input
            aria-controls="dashboard-search-results"
            aria-expanded={searchResults.length > 0}
            aria-label="搜索功能"
            placeholder="搜索功能"
            type="search"
            value={searchQuery}
            onChange={(event) => setSearchQuery(event.target.value)}
            onKeyDown={handleSearchKeyDown}
          />
          {normalizedSearch !== '' && (
            <nav
              aria-label="搜索结果"
              className="dashboard-search-results"
              id="dashboard-search-results"
              ref={searchResultsRef}
            >
              {searchResults.length === 0
                ? <p>没有可访问的匹配功能</p>
                : searchResults.map((item) => (
                    <NavLink aria-label={item.title} key={item.path} to={item.path} onClick={() => setSearchQuery('')}>
                      <span>{item.title}</span>
                      <small>{item.path}</small>
                    </NavLink>
                  ))}
            </nav>
          )}
        </div>
        <a className="dashboard-admin-link" href="/saas-admin/">SaaS 管理后台</a>
        <div className="dashboard-account-actions">
          {access !== null && <span className="dashboard-corp-badge">{access.corp.name}</span>}
          {sessionActions.userId !== null && <span>{sessionActions.userName ?? `账号 ${sessionActions.userId}`}</span>}
          <button
            className="dashboard-logout-button"
            disabled={sessionActions.isLoggingOut}
            onClick={() => void sessionActions.logout()}
            type="button"
          >
            {sessionActions.isLoggingOut ? '正在退出…' : '退出登录'}
          </button>
        </div>
      </header>
      <div className="dashboard-body">
        <nav aria-label="主菜单" className="dashboard-sidebar">
          <div className="dashboard-sidebar-heading">
            <span>工作台</span>
            <span className="dashboard-sidebar-caption">全部功能</span>
          </div>
          {access === null || !hasPages ? (
            <p className="dashboard-menu-empty">暂无可访问功能</p>
          ) : (
            <ul className="dashboard-menu-list">
              {topLevelNavigation.map((item) => (
                <li key={item.path}>
                  <NavLink
                    className={item.activePath === null
                      ? 'dashboard-menu-link dashboard-menu-top-level'
                      : 'dashboard-menu-link dashboard-menu-link-active dashboard-menu-top-level'}
                    to={item.path}
                  >
                    {item.title}
                  </NavLink>
                </li>
              ))}
              {navigation.map((group) => {
                const expanded = expandedGroups.has(group.id);
                return (
                  <li className="dashboard-menu-group" key={group.id}>
                    <button
                      aria-expanded={expanded}
                      className="dashboard-menu-group-toggle"
                      onClick={() => toggleGroup(group.id)}
                      type="button"
                    >
                      <span>{group.title}</span>
                      <span aria-hidden="true">{expanded ? '⌃' : '⌄'}</span>
                    </button>
                    {expanded && group.items.length > 0 && (
                      <ul className="dashboard-menu-group-items">
                        {group.items.map((item) => (
                          <li key={item.path}>
                            <NavLink
                              className={item.activePath === null
                                ? 'dashboard-menu-link'
                                : 'dashboard-menu-link dashboard-menu-link-active'}
                              to={item.path}
                            >
                              {item.title}
                            </NavLink>
                          </li>
                        ))}
                      </ul>
                    )}
                  </li>
                );
              })}
            </ul>
          )}
        </nav>
        <main className="dashboard-content">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
