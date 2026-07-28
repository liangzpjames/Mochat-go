import { Outlet } from 'react-router';

export function DashboardLayout() {
  return (
    <div className="dashboard-shell">
      <header className="dashboard-header">
        <strong>MoChat</strong>
        <span>企业管理后台</span>
      </header>
      <div className="dashboard-body">
        <nav aria-label="主菜单" className="dashboard-sidebar">
          <section aria-label="一级菜单">
            <h2>功能导航</h2>
          </section>
          <section aria-label="二级菜单">
            <p>菜单将在权限加载后显示</p>
          </section>
        </nav>
        <main className="dashboard-content">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
