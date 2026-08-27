import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';
import { reportEndpoint, text, type Phase35Api } from './api';
import { ReportFilters, useReportFilters } from './report-query';
import { SimpleTrendChart } from './components/simple-trend-chart';
import { ReportDetailTable } from './components/report-detail-table';
import { Phase35KpiLegend } from './components/phase35-kpi-legend';
import type { ReportResult } from './report-types';
import { paginationOf } from './report-types';
import { Phase35PageShell } from './components/phase35-page-shell';
import { Phase35DataState } from './components/data-state';
import { BusinessDataNotice } from './components/business-data-notice';

export function CustomerReportPage({ api }: { api: Phase35Api }) {
  const filters = useReportFilters();
  const [page, setPage] = useState(1);
  const params = { ...filters.params, page, pageSize: 20 };
  const query = useQuery({
    queryKey: ['p35-report', 'customer', params],
    queryFn: () => api.read(reportEndpoint('customer'), params),
  });
  const result = query.data as ReportResult | undefined;
  const items = result?.items ?? [];
  const total = Number(result?.summary?.customer ?? 0);
  const pagination = paginationOf(result);
  const owned = items.filter((row) => text(row.ownerName) !== '--');
  const coverage = items.length ? `${Math.round((owned.length / items.length) * 100)}%` : '--';
  const empty = !query.isLoading && !total && !items.length;
  const series = (result?.series ?? []).map((point) => ({ at: text(point.at ?? point.day), value: Number(point.value ?? 0) }));
  const limitations = result?.limitations ?? [];

  return (
    <Phase35PageShell
      title="客户分析"
      description="按日期与授权范围查看客户规模、趋势、负责人分布和明细"
      actions={<span className="phase35-chip">数据来源：SCRM 联系人 · 负责人分配</span>}
    >
      <div className="phase35-page">
<section className="phase35-card phase35-filter-card">
          <ReportFilters filters={filters} api={api} />
        </section>

        <section className="phase35-kpis" aria-label="客户分析指标">
          <article className="phase35-kpi phase35-kpi-primary">
            <span>客户总数</span><strong>{total}</strong><small>筛选范围内未删除的客户（SCRM 联系人）总数</small>
          </article>
          <article className="phase35-kpi phase35-kpi-green">
            <span>当前页明细</span><strong>{items.length}</strong><small>当前筛选条件下第 {pagination.page} 页展示条数</small>
          </article>
          <article className="phase35-kpi phase35-kpi-violet">
            <span>负责人覆盖</span><strong>{coverage}</strong><small>当前页明细中已分配负责人的客户（联系人）占比</small>
          </article>
        </section>
        <Phase35KpiLegend />

        <div className="phase35-columns">
          <section className="phase35-card">
            <header className="phase35-card-header">
              <div><h2>客户增长趋势</h2><p>按日聚合的新增客户曲线</p></div>
              <span className="phase35-chip">按日</span>
            </header>
            {series.length
              ? <div className="phase35-chart"><SimpleTrendChart points={series} /></div>
              : <p className="phase35-empty">当前范围暂无趋势数据</p>}
          </section>

          <section className="phase35-card">
            <header className="phase35-card-header">
              <div><h2>指标口径</h2><p>本页统计口径与数据说明</p></div>
            </header>
            <dl className="phase35-notes">
              <div><dt>客户总数</dt><dd>筛选范围内未删除的客户（SCRM 联系人）总数，含已分配与未分配客户。</dd></div>
              <div><dt>当前页明细</dt><dd>同一筛选条件下返回的客户明细条数，随分页变化。</dd></div>
              <div><dt>负责人覆盖</dt><dd>明细中最近分配记录存在负责人的客户（联系人）占比；未分配客户不计入。</dd></div>
            </dl>
            <BusinessDataNotice visible={limitations.length > 0} />
          </section>
        </div>

        <section className="phase35-card">
          <header className="phase35-card-header">
            <div><h2>客户明细</h2><p>按日期与负责人查看客户记录</p></div>
            <span className="phase35-chip">共 {pagination.total} 条</span>
          </header>
          <Phase35DataState
            loading={query.isLoading}
            error={query.isError}
            empty={empty}
            emptyContent={(
              <section aria-label="客户分析数据说明" className="phase35-empty-state">
                <h2>当前范围还没有客户数据</h2>
                <p>本页统计真实 SCRM 联系人及负责人分配记录。可先创建联系人，或扩大日期范围后重新查询。</p>
                <a href="/customer/order">前往订单页快速创建联系人</a>
              </section>
            )}
            onRetry={() => void query.refetch()}
          >
            <div className="phase35-table">
              <ReportDetailTable
                items={items}
                page={pagination.page}
                pageSize={pagination.pageSize}
                total={pagination.total}
                onPageChange={setPage}
                columns={[{ key: 'day', label: '日期' }, { key: 'ownerName', label: '负责人' }]}
              />
            </div>
          </Phase35DataState>
        </section>
      </div>
    </Phase35PageShell>
  );
}
