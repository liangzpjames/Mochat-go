import { NavLink, Outlet } from 'react-router';

import { useOptionalDashboardAccess } from '../app/access-context';
import { useDashboardSessionActions } from '../features/auth/session-actions';
import type { MenuNode } from '../features/navigation/menu-tree';

type NavigationItem = {
  name: string;
  path: string;
};

function authorizedNavigation(
  nodes: readonly MenuNode[],
  allowedRoutes: ReadonlySet<string>,
): NavigationItem[] {
  const items: NavigationItem[] = [];
  const seen = new Set<string>();
  const visit = (menu: readonly MenuNode[]) => {
    for (const node of menu) {
      if (
        node.linkUrl !== null
        && allowedRoutes.has(node.linkUrl)
        && !seen.has(node.linkUrl)
      ) {
        seen.add(node.linkUrl);
        items.push({ name: node.name, path: node.linkUrl });
      }
      visit(node.children);
    }
  };
  visit(nodes);
  return items;
}

export function DashboardLayout() {
  const access = useOptionalDashboardAccess();
  const sessionActions = useDashboardSessionActions();
  const navigation = access === null
    ? []
    : authorizedNavigation(access.menu, access.allowedRoutes);

  return (
    <div className="dashboard-shell">
      <header className="dashboard-header">
        <NavLink className="dashboard-brand" to="/">MoChat</NavLink>
        <span className="dashboard-product-name">企业管理后台</span>
        <a className="dashboard-admin-link" href="/saas-admin/">
          SaaS 管理后台
        </a>
        <div className="dashboard-account-actions">
          {sessionActions.userId !== null && (
            <span>账号 {sessionActions.userId}</span>
          )}
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
          <h2>功能导航</h2>
          {access !== null && (
            <p className="dashboard-corp-name">{access.corp.name}</p>
          )}
          {navigation.length === 0
            ? <p className="dashboard-menu-empty">权限菜单暂无可用页面</p>
            : (
              <ul className="dashboard-menu-list">
                {navigation.map((item) => (
                  <li key={item.path}>
                    <NavLink
                      className={({ isActive }) => isActive
                        ? 'dashboard-menu-link dashboard-menu-link-active'
                        : 'dashboard-menu-link'}
                      to={item.path}
                    >
                      {item.name}
                    </NavLink>
                  </li>
                ))}
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
