import { useState } from 'react';
import { NavLink, Outlet, useLocation } from 'react-router';

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
  const [expandedGroups, setExpandedGroups] = useState<ReadonlySet<string>>(
    () => new Set(),
  );
  const hasPages = topLevelNavigation.length > 0
    || navigation.some((group) => group.items.length > 0);

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

  return (
    <div className="dashboard-shell">
      <header className="dashboard-header">
        <NavLink className="dashboard-brand" to="/">MoChat AI</NavLink>
        <span className="dashboard-product-name">企业智能运营平台</span>
        <label className="dashboard-search">
          <span className="sr-only">搜索功能</span>
          <input aria-label="搜索功能" placeholder="搜索功能" type="search" />
        </label>
        <a className="dashboard-task-link" href="#tasks">任务中心</a>
        <a className="dashboard-admin-link" href="/saas-admin/">SaaS 管理后台</a>
        <div className="dashboard-account-actions">
          {access !== null && <span className="dashboard-corp-badge">{access.corp.name}</span>}
          {sessionActions.userId !== null && <span>账号 {sessionActions.userId}</span>}
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
