import { useQuery } from '@tanstack/react-query';
import { Link } from 'react-router';
import { reportEndpoint, type Phase35Api } from './api';
import { ReportFilters, useReportFilters } from './report-query';
import type { ReportResult } from './report-types';
import { formatMetric } from './presentation/formatters';
import { Phase35PageShell } from './components/phase35-page-shell';
import { Phase35DataState } from './components/data-state';
import { Phase35KpiLegend } from './components/phase35-kpi-legend';

const sections = [
  {
    name: '客户概览',
    description: '客户规模、新增趋势与负责人覆盖情况',
    primary: 'customer',
    tone: 'phase35-kpi-primary',
    link: '/data/customer',
    note: '主指标为联系人总数（客户分析口径），不与联系人阶段重复相加。',
  },
  {
    name: '转化漏斗',
    description: '线索到联系人、商机、赢单及订单的阶段转化',
    primary: 'lead',
    tone: 'phase35-kpi-green',
    link: '/data/conversion',
    note: '主指标为漏斗起始阶段线索数；各阶段与转化率请查看转化分析页。',
  },
  {
    name: '订单经营',
    description: '订单数量、状态和成交金额',
    primary: 'order',
    tone: 'phase35-kpi-violet',
    link: '/customer/order',
    note: '主指标为订单数（won/completed/paid 口径），不与金额相加。',
  },
  {
    name: '行为审计',
    description: '设置变更和订单操作等可追溯行为',
    primary: 'behavior',
    tone: 'phase35-kpi-orange',
    link: '/data/behavior',
    note: '主指标为筛选范围内审计事件条数。',
  },
] as const;

export function ReportPage({ api }: { api: Phase35Api }) {
  const filters = useReportFilters();
  const query = useQuery({
    queryKey: ['p35-report', 'report', filters.params],
    queryFn: () => api.read(reportEndpoint('report'), filters.params),
  });
  const result = query.data as ReportResult | undefined;
  const summary = result?.summary ?? {};
  return (
    <Phase35PageShell
      title="综合报表"
      description="按客户、转化、订单和行为四个核心区块汇总经营情况"
      actions={<span className="phase35-chip">数据来源：SCRM 实时业务记录</span>}
    >
      <div className="phase35-page">
        <section className="phase35-card phase35-filter-card">
          <ReportFilters filters={filters} api={api} />
        </section>

        <section className="phase35-kpis" aria-label="综合报表指标">
          {sections.map((section) => (
            <article key={section.name} className={`phase35-kpi ${section.tone}`}>
              <span>{section.name}</span>
              <strong>{formatMetric(summary[section.primary] ?? 0)}</strong>
              <small>{section.note}</small>
            </article>
          ))}
        </section>
        <Phase35KpiLegend />

        <section className="phase35-section-grid">
          {sections.map((section) => (
            <section role="region" aria-label={section.name} key={section.name} className="phase35-card phase35-section">
              <h2>{section.name}</h2>
              <p>{section.description}</p>
              <strong>{formatMetric(summary[section.primary] ?? 0)}</strong>
              <p>{section.note}</p>
              <Link to={section.link}>查看详情</Link>
            </section>
          ))}
        </section>

        <Phase35DataState loading={query.isLoading} error={query.isError} onRetry={() => void query.refetch()}>
          {result?.limitations?.map((item) => <p role="status" key={item.provider}>{item.message}</p>)}
        </Phase35DataState>
      </div>
    </Phase35PageShell>
  );
}
