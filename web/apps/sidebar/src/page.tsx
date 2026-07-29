import { useLocation } from 'react-router';

const labels: Record<string, string> = {
  auth: '授权', codeAuth: '扫码授权', contact: '客户资料',
  contactBatchAdd: '批量加好友', contactSop: '客户 SOP',
  login: '登录', medium: '素材库', roomSop: '群 SOP',
};

export default function SidebarPage() {
  const location = useLocation();
  const feature = location.pathname.split('/').filter(Boolean)[0] ?? 'contact';
  return (
    <main className="page" data-testid="react-migrated-page" data-route={location.pathname}>
      <header><span className="brand">MoChat</span><span className="badge">React</span></header>
      <h1>{labels[feature] ?? '客户侧边栏'}</h1>
      <p>{location.pathname}</p>
      <section className="card">页面已迁移到统一前端，保留原 URL、查询参数和企微工作台入口。</section>
    </main>
  );
}
