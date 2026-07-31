import { ApiError } from '@mochat/api-client';
import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';

import { useDashboardAccess } from '../../app/access-context';
import type {
  DashboardOverviewApi,
  DashboardOverviewTrendPoint,
} from './dashboard-overview-api';

type OverviewRange = {
  from: string;
  to: string;
};

function localDateText(value: Date): string {
  const year = value.getFullYear();
  const month = String(value.getMonth() + 1).padStart(2, '0');
  const date = String(value.getDate()).padStart(2, '0');
  return `${year}-${month}-${date}`;
}

function defaultRange(): OverviewRange {
  const to = new Date();
  const from = new Date(to.getFullYear(), to.getMonth(), to.getDate() - 30);
  return { from: localDateText(from), to: localDateText(to) };
}

function trendMaximum(points: readonly DashboardOverviewTrendPoint[]): number {
  return Math.max(
    1,
    ...points.flatMap((point) => [
      point.addContactNum,
      point.addIntoRoomNum,
      point.lossContactNum,
      point.quitRoomNum,
    ]),
  );
}

function TrendChart({ points }: { points: readonly DashboardOverviewTrendPoint[] }) {
  const maximum = trendMaximum(points);
  return (
    <div className="dashboard-overview-chart" role="img" aria-label="企业趋势图">
      {points.map((point) => (
        <div className="dashboard-overview-chart-column" key={point.date}>
          <div className="dashboard-overview-bars">
            <span
              aria-label={`新增客户 ${point.addContactNum}`}
              className="dashboard-overview-bar dashboard-overview-bar-primary"
              style={{ height: `${Math.max(4, (point.addContactNum / maximum) * 100)}%` }}
              title={`新增客户 ${point.addContactNum}`}
            />
            <span
              aria-label={`新增入群 ${point.addIntoRoomNum}`}
              className="dashboard-overview-bar dashboard-overview-bar-secondary"
              style={{ height: `${Math.max(4, (point.addIntoRoomNum / maximum) * 100)}%` }}
              title={`新增入群 ${point.addIntoRoomNum}`}
            />
            <span
              aria-label={`流失客户 ${point.lossContactNum}`}
              className="dashboard-overview-bar dashboard-overview-bar-warning"
              style={{ height: `${Math.max(4, (point.lossContactNum / maximum) * 100)}%` }}
              title={`流失客户 ${point.lossContactNum}`}
            />
            <span
              aria-label={`退出群聊 ${point.quitRoomNum}`}
              className="dashboard-overview-bar dashboard-overview-bar-muted"
              style={{ height: `${Math.max(4, (point.quitRoomNum / maximum) * 100)}%` }}
              title={`退出群聊 ${point.quitRoomNum}`}
            />
          </div>
          <span>{point.date}</span>
        </div>
      ))}
    </div>
  );
}

export function DashboardOverviewPage({
  api,
  initialRange,
}: {
  api: DashboardOverviewApi;
  initialRange?: OverviewRange;
}) {
  const access = useDashboardAccess();
  const [draftRange, setDraftRange] = useState<OverviewRange>(
    initialRange ?? defaultRange(),
  );
  const [range, setRange] = useState<OverviewRange>(initialRange ?? defaultRange());
  const [rangeError, setRangeError] = useState<string | null>(null);
  const query = useQuery({
    queryKey: ['corp', access.corp.id, 'dashboard-overview', range.from, range.to],
    queryFn: () => api.load({ corpId: access.corp.id, ...range }),
  });

  function applyRange() {
    if (draftRange.from === '' || draftRange.to === '' || draftRange.from > draftRange.to) {
      setRangeError('请选择有效的日期范围');
      return;
    }
    setRangeError(null);
    setRange(draftRange);
  }

  const forbidden = query.error instanceof ApiError
    && query.error.kind === 'forbidden';
  const empty = query.data !== undefined
    && query.data.cards.length === 0
    && query.data.trend.length === 0;

  return (
    <section className="dashboard-overview-page">
      <header className="dashboard-overview-header">
        <div>
          <p className="dashboard-overview-eyebrow">数据中心</p>
          <h1>数据概览</h1>
          <p>查看当前企业的客户、群聊与成员变化。</p>
        </div>
        <div className="dashboard-overview-filters">
          <label>
            <span>开始日期</span>
            <input
              aria-label="开始日期"
              type="date"
              value={draftRange.from}
              onChange={(event) => setDraftRange((current) => ({
                ...current,
                from: event.target.value,
              }))}
            />
          </label>
          <label>
            <span>结束日期</span>
            <input
              aria-label="结束日期"
              type="date"
              value={draftRange.to}
              onChange={(event) => setDraftRange((current) => ({
                ...current,
                to: event.target.value,
              }))}
            />
          </label>
          <button onClick={applyRange} type="button">查询</button>
          <button
            disabled={query.isFetching}
            onClick={() => void query.refetch()}
            type="button"
          >
            刷新
          </button>
        </div>
      </header>

      {rangeError !== null && <p className="dashboard-overview-inline-error" role="alert">{rangeError}</p>}
      {query.isPending && (
        <div className="dashboard-overview-state" role="status">正在加载数据概览…</div>
      )}
      {forbidden && (
        <div className="dashboard-overview-state dashboard-overview-state-error">
          <h2>无权访问当前企业数据</h2>
          <p>请切换到已授权企业，或联系管理员开通权限。</p>
        </div>
      )}
      {query.isError && !forbidden && (
        <div className="dashboard-overview-state dashboard-overview-state-error" role="alert">
          <h2>数据加载失败</h2>
          <p>{query.error instanceof Error ? query.error.message : '请稍后重试'}</p>
          <button onClick={() => void query.refetch()} type="button">重试</button>
        </div>
      )}
      {empty && (
        <div className="dashboard-overview-state">
          <h2>当前日期范围暂无数据</h2>
          <p>调整日期范围后重新查询。</p>
        </div>
      )}
      {query.data !== undefined && !empty && (
        <>
          <div className="dashboard-overview-cards">
            {query.data.cards.map((card) => (
              <article className="dashboard-overview-card" key={card.key}>
                <span>{card.label}</span>
                <strong>{card.value.toLocaleString('zh-CN')}</strong>
              </article>
            ))}
          </div>
          <article className="dashboard-overview-trend">
            <header>
              <div>
                <h2>企业数据趋势</h2>
                <p>新增客户、入群、流失与退群的每日变化</p>
              </div>
              <p>更新时间：{query.data.updatedAt || '--'}</p>
            </header>
            {query.data.trend.length === 0
              ? <p className="dashboard-overview-trend-empty">当前日期范围暂无趋势数据</p>
              : <TrendChart points={query.data.trend} />}
          </article>
        </>
      )}
    </section>
  );
}
