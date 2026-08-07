import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';
import { reportEndpoint, type Phase35Api } from './api';
import { ReportFilters, useReportFilters } from './report-query';
import type { ReportItem, ReportResult } from './report-types';
import { paginationOf } from './report-types';
import { behaviorLabel } from './presentation/labels';
import { formatDate } from './presentation/formatters';
import { Phase35PageShell } from './components/phase35-page-shell';
import { Phase35DataState } from './components/data-state';
import { Phase35DetailDrawer } from './components/detail-drawer';
import { Phase35KpiLegend } from './components/phase35-kpi-legend';

export function BehaviorReportPage({ api }: { api: Phase35Api }) {
  const filters = useReportFilters();
  const [selected, setSelected] = useState<ReportItem>();
  const [page, setPage] = useState(1);
  const params = { ...filters.params, page, pageSize: 20 };
  const query = useQuery({
    queryKey: ['p35-report', 'behavior', params],
    queryFn: () => api.read(reportEndpoint('behavior'), params),
  });
  const result = query.data as ReportResult | undefined;
  const pagination = paginationOf(result);
  const items = result?.items ?? [];
  const types = new Set(items.map((item) => String(item.eventType ?? ''))).size;
  return (
    <Phase35PageShell
      title="行为分析"
      description="查看设置、订单等可审计业务操作及其操作者和对象"
      actions={<span className="phase35-chip">来源：订单 / 设置审计</span>}
    >
      <div className="phase35-page">
        <section className="phase35-card phase35-filter-card">
          <ReportFilters filters={filters} api={api} />
        </section>

        <section className="phase35-kpis" aria-label="行为分析指标">
          <article className="phase35-kpi phase35-kpi-primary">
            <span>行为事件总数</span><strong>{pagination.total}</strong><small>筛选范围内审计事件条数</small>
          </article>
          <article className="phase35-kpi phase35-kpi-green">
            <span>当前页展示</span><strong>{items.length}</strong><small>第 {pagination.page} 页</small>
          </article>
          <article className="phase35-kpi phase35-kpi-violet">
            <span>事件类型</span><strong>{types}</strong><small>当前页不同类型数量</small>
          </article>
        </section>
        <Phase35KpiLegend />

        <section className="phase35-card">
          <header className="phase35-card-header">
            <div><h2>行为事件</h2><p>设置变更与订单操作等可追溯行为</p></div>
            <span className="phase35-chip">共 {pagination.total} 条</span>
          </header>
          <div className="phase35-table-card">
            <Phase35DataState loading={query.isLoading} error={query.isError} empty={!query.isLoading && !items.length} onRetry={() => void query.refetch()}>
              <table>
                <thead><tr><th>行为类型</th><th>操作者</th><th>业务对象</th><th>发生时间</th><th>变更摘要</th><th>操作</th></tr></thead>
                <tbody>
                  {items.map((item, index) => (
                    <tr key={String(item.id ?? index)}>
                      <td>{behaviorLabel(item.eventType)}</td>
                      <td>账号 {String(item.actorId ?? '--')}</td>
                      <td>{String(item.objectLabel ?? item.objectId ?? '--')}</td>
                      <td>{formatDate(item.occurredAt)}</td>
                      <td>{String(item.detail ?? '可查看详情')}</td>
                      <td><button type="button" aria-label="查看行为详情" onClick={() => setSelected(item)}>查看详情</button></td>
                    </tr>
                  ))}
                </tbody>
              </table>
              <div className="dashboard-pagination">
                <span>共 {pagination.total} 条，第 {pagination.page} 页</span>
                <button type="button" disabled={pagination.page <= 1} onClick={() => setPage((value) => Math.max(1, value - 1))}>上一页</button>
                <button type="button" disabled={pagination.page * pagination.pageSize >= pagination.total} onClick={() => setPage((value) => value + 1)}>下一页</button>
              </div>
            </Phase35DataState>
          </div>
        </section>

        <section className="phase35-card">
          <header className="phase35-card-header">
            <div><h2>指标口径</h2><p>行为审计统计规则</p></div>
          </header>
          <dl className="phase35-notes">
            <div><dt>事件来源</dt><dd>订单创建/状态流转与客户设置更新等可审计业务操作。</dd></div>
            <div><dt>操作者</dt><dd>记录触发操作的系统账号；对象列展示业务对象标识。</dd></div>
            <div><dt>数据范围</dt><dd>按 tenant、corp 与时间窗口过滤，未知事件类型不展示。</dd></div>
          </dl>
        </section>
      </div>

      <Phase35DetailDrawer title="行为详情" open={Boolean(selected)} onClose={() => setSelected(undefined)}>
        <dl>
          <dt>行为类型</dt><dd>{behaviorLabel(selected?.eventType)}</dd>
          <dt>操作者</dt><dd>账号 {String(selected?.actorId ?? '--')}</dd>
          <dt>业务对象</dt><dd>{String(selected?.objectLabel ?? selected?.objectId ?? '--')}</dd>
          <dt>发生时间</dt><dd>{formatDate(selected?.occurredAt)}</dd>
          <dt>变更摘要</dt><dd>{String(selected?.detail ?? '暂无补充说明')}</dd>
        </dl>
      </Phase35DetailDrawer>
    </Phase35PageShell>
  );
}
