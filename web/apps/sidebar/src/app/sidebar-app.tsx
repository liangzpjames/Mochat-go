import { useState } from 'react';
import { useLocation } from 'react-router';

import { sidebarRoutes } from './catalog';

export type SidebarRequest = (
  endpoint: string,
  init?: RequestInit,
) => Promise<unknown>;

export function SidebarApp({ request }: { request: SidebarRequest }) {
  const location = useLocation();
  const config = sidebarRoutes[location.pathname] ?? sidebarRoutes['/'];
  const [activePanel, setActivePanel] = useState<string | null>(null);
  const [status, setStatus] = useState('');

  if (config === undefined) return <main className="page">页面不存在</main>;

  function act(action: string) {
    setActivePanel(action);
    if (action === '确认授权') {
      void request('/sidebar/auth', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          code: new URLSearchParams(location.search).get('code') ?? 'fake-code',
        }),
      }).then(() => setStatus('授权成功'));
    }
  }

  return (
    <main className="page" data-route={location.pathname}>
      <header className="mobile-header">
        <span className="brand">MoChat</span>
        <span className="badge">客户工作台</span>
      </header>
      <h1>{config.title}</h1>
      <p className="muted">{config.description}</p>
      {status && <p role="status">{status}</p>}
      <div className="section-list">
        {config.sections.map((section) => (
          <section className="card" key={section}>
            <h2>{section}</h2>
            <p>{section}内容已加载，可继续查看和操作。</p>
          </section>
        ))}
      </div>
      <nav className="action-bar">
        {config.actions.map((action) => (
          <button key={action} type="button" onClick={() => act(action)}>
            {action}
          </button>
        ))}
      </nav>
      {activePanel === '修改备注' && (
        <section className="drawer">
          <label>
            客户备注
            <textarea aria-label="客户备注" defaultValue="重点客户" />
          </label>
          <button type="button" onClick={() => setActivePanel(null)}>保存</button>
        </section>
      )}
      {activePanel !== null && activePanel !== '修改备注' && (
        <section className="drawer" aria-label={`${activePanel}面板`}>
          <h2>{activePanel}</h2>
          <p>操作面板已打开。</p>
        </section>
      )}
    </main>
  );
}
