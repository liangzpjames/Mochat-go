import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';
import { reportEndpoint, type Phase35Api } from './api';
import { ReportFilters, useReportFilters } from './report-query';
import type { ReportResult } from './report-types';
import { paginationOf } from './report-types';
import { formatMetric } from './presentation/formatters';
import { Phase35PageShell } from './components/phase35-page-shell';
import { Phase35DataState } from './components/data-state';
import { Phase35DetailDrawer } from './components/detail-drawer';
import { ConversionFunnel, conversionStageLabel } from './components/conversion-funnel';
import { ReportDetailTable, type DetailColumn } from './components/report-detail-table';
import { Phase35KpiLegend } from './components/phase35-kpi-legend';

const stageColumns: Record<string, DetailColumn[]> = {
  lead: [
    { key: 'id', label: '线索 ID' },
    { key: 'name', label: '线索名称' },
    { key: 'source', label: '来源' },
    { key: 'status', label: '状态' },
    { key: 'ownerName', label: '负责人' },
    { key: 'day', label: '创建日期' },
  ],
  contact: [
    { key: 'id', label: '联系人 ID' },
    { key: 'name', label: '联系人姓名' },
    { key: 'phone', label: '手机号' },
    { key: 'ownerName', label: '负责人' },
    { key: 'day', label: '创建日期' },
  ],
  opportunity: [
    { key: 'id', label: '商机 ID' },
    { key: 'contactName', label: '关联联系人' },
    { key: 'status', label: '状态' },
    { key: 'ownerName', label: '负责人' },
    { key: 'day', label: '创建日期' },
  ],
  won: [
    { key: 'id', label: '商机 ID' },
    { key: 'contactName', label: '关联联系人' },
    { key: 'status', label: '状态' },
    { key: 'ownerName', label: '负责人' },
    { key: 'day', label: '创建日期' },
  ],
  order: [
    { key: 'id', label: '订单 ID' },
    { key: 'contactName', label: '关联联系人' },
    { key: 'amount', label: '金额' },
    { key: 'status', label: '状态' },
    { key: 'ownerName', label: '负责人' },
    { key: 'day', label: '创建日期' },
  ],
};

export function ConversionReportPage({ api }: { api: Phase35Api }) {
  const filters = useReportFilters();
  const [stage, setStage] = useState('');
  const [page, setPage] = useState(1);
  const query = useQuery({
    queryKey: ['p35-report', 'conversion', filters.params],
    queryFn: () => api.read(reportEndpoint('conversion'), filters.params),
  });
  const result = query.data as ReportResult | undefined;
  const summary = result?.summary ?? {};
  const detailQuery = useQuery({
    queryKey: ['p35-report', 'conversion-stage', stage, filters.params, page],
    queryFn: () => api.read(reportEndpoint('conversion'), { ...filters.params, stage, page, pageSize: 20 }),
    enabled: Boolean(stage),
  });
  const detail = detailQuery.data as ReportResult | undefined;
  const detailPagination = paginationOf(detail);
  const detailItems = detail?.items ?? [];
  const closeDetail = () => { setStage(''); setPage(1); };
  return (
    <Phase35PageShell
      title="转化分析"
      description="线索、联系人、商机、赢单与订单的统一口径漏斗"
      actions={<span className="phase35-chip">口径：lead → contact → opportunity → won → order</span>}
    >
      <div className="phase35-page">
        <section className="phase35-card phase35-filter-card">
          <ReportFilters filters={filters} api={api} />
        </section>

        <section className="phase35-kpis" aria-label="转化分析指标">
          <article className="phase35-kpi phase35-kpi-primary">
            <span>联系人转化率</span><strong>{formatMetric(summary.contactRate, '%')}</strong><small>联系人 / 线索</small>
          </article>
          <article className="phase35-kpi phase35-kpi-green">
            <span>商机转化率</span><strong>{formatMetric(summary.opportunityRate, '%')}</strong><small>商机 / 联系人</small>
          </article>
          <article className="phase35-kpi phase35-kpi-violet">
            <span>赢单转化率</span><strong>{formatMetric(summary.wonRate, '%')}</strong><small>赢单 / 商机</small>
          </article>
          <article className="phase35-kpi phase35-kpi-orange">
            <span>订单转化率</span><strong>{formatMetric(summary.orderRate, '%')}</strong><small>订单 / 赢单</small>
          </article>
        </section>
        <Phase35KpiLegend />

        <div className="phase35-columns">
          <section className="phase35-card phase35-funnel-card">
            <header className="phase35-card-header">
              <div><h2>转化漏斗</h2><p>点击任一阶段查看该阶段真实业务对象明细</p></div>
              <span className="phase35-chip">五阶段口径</span>
            </header>
            <Phase35DataState loading={query.isLoading} error={query.isError} onRetry={() => void query.refetch()}>
              <ConversionFunnel summary={summary} onStageClick={(nextStage) => { setStage(nextStage); setPage(1); }} />
              {result?.limitations?.map((item) => <p role="status" key={item.provider}>{item.message}</p>)}
            </Phase35DataState>
          </section>

          <section className="phase35-card">
            <header className="phase35-card-header">
              <div><h2>指标口径</h2><p>各阶段统计与转化率规则</p></div>
            </header>
            <dl className="phase35-notes">
              <div><dt>线索</dt><dd>筛选范围内创建的 SCRM 线索数，漏斗起始阶段。</dd></div>
              <div><dt>联系人 / 商机 / 赢单</dt><dd>对应阶段对象数，均按 tenant/corp 与时间窗口统计。</dd></div>
              <div><dt>订单</dt><dd>仅统计 won、completed、paid 状态的订单，与综合报表口径一致。</dd></div>
              <div><dt>转化率</dt><dd>相邻阶段数量之比；分母为 0 时显示 --，不展示误导性百分比。</dd></div>
            </dl>
          </section>
        </div>
      </div>

      <Phase35DetailDrawer title={`${conversionStageLabel(stage)}阶段明细`} open={Boolean(stage)} onClose={closeDetail}>
        <Phase35DataState
          loading={detailQuery.isLoading}
          error={detailQuery.isError}
          empty={!detailQuery.isLoading && !detailQuery.isError && !detailItems.length}
          emptyContent={<p role="status">当前筛选范围暂无{conversionStageLabel(stage)}阶段明细，请调整日期、员工或部门后重试。</p>}
          onRetry={() => void detailQuery.refetch()}
        >
          <div className="phase35-table">
            <ReportDetailTable
              items={detailItems}
              page={detailPagination.page}
              pageSize={detailPagination.pageSize}
              total={detailPagination.total}
              onPageChange={setPage}
              columns={stageColumns[stage] ?? []}
            />
          </div>
        </Phase35DataState>
      </Phase35DetailDrawer>
    </Phase35PageShell>
  );
}
