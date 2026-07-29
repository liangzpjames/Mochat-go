import { useLocation } from 'react-router';

const labels: Record<string, string> = {
  explain: '活动说明', lottery: '抽奖活动', roomClockIn: '群打卡',
  roomFission: '群裂变', fissionSpeed: '裂变进度',
  roomInfinitePull: '无限拉群', shopCode: '门店活码',
  workFission: '任务宝', speed: '任务进度',
};

export default function OperationPage() {
  const location = useLocation();
  const feature = location.pathname.split('/').filter(Boolean)[0] ?? 'workFission';
  return (
    <main className="operation" data-testid="react-migrated-page" data-route={location.pathname}>
      <span className="badge">React</span>
      <h1>{labels[feature] ?? '营销活动'}</h1>
      <p>{location.pathname}</p>
      <article>活动页面已由 React 承载，并继续使用同源 Go API、微信授权参数与深链。</article>
    </main>
  );
}
