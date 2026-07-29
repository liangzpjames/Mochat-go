import { useState } from 'react';
import { useLocation } from 'react-router';

import { operationRoutes } from './catalog';

export type OperationRequest = (
  endpoint: string,
  init?: RequestInit,
) => Promise<unknown>;

export function OperationApp({ request }: { request: OperationRequest }) {
  const location = useLocation();
  const config = operationRoutes[location.pathname] ?? operationRoutes['/'];
  const [result, setResult] = useState('');

  if (config === undefined) return <main className="operation">活动不存在</main>;

  function execute() {
    const feature = location.pathname.split('/').filter(Boolean)[0] ?? 'index';
    void request(`/operation/${feature}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        activityId: new URLSearchParams(location.search).get('activityId') ?? 'fake-activity',
      }),
    }).then((payload) => {
      const prize = typeof payload === 'object' && payload !== null && 'prizeName' in payload
        ? String(payload.prizeName)
        : '操作已完成';
      setResult(prize);
    });
  }

  return (
    <main className="operation" data-route={location.pathname}>
      <div className="hero">
        <span className="badge">MoChat 活动</span>
        <h1>{config.title}</h1>
        <p>{config.subtitle}</p>
      </div>
      {config.progress && <strong className="progress">{config.progress}</strong>}
      <ol className="stage-list">
        {config.stages.map((stage, index) => (
          <li key={stage}>
            <span>{index + 1}</span>
            <div><h2>{stage}</h2><p>{stage}信息已准备完成。</p></div>
          </li>
        ))}
      </ol>
      {result && <section className="result" role="status">{result}</section>}
      <button className="primary-action" type="button" onClick={execute}>
        {config.primaryAction}
      </button>
    </main>
  );
}
