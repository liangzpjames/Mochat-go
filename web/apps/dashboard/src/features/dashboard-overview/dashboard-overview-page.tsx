import { ApiError } from '@mochat/api-client';
import { useQuery } from '@tanstack/react-query';
import { Button } from 'antd';
import { useEffect, useMemo, useState } from 'react';
import { useSearchParams } from 'react-router';

import { useDashboardAccess } from '../../app/access-context';
import { PageState } from '../../components/page-state/page-state';
import { DateRangeFields } from '../../components/date-range-fields';
import type { Phase35Api } from '../phase35/api';
import { updateSearch } from '../../shared/query-state';
import type {
  DashboardOverviewApi,
  DashboardOverviewQuery,
} from './dashboard-overview-api';
import {
  OverviewAIInsightGrid,
  OverviewCapabilityPanel,
  OverviewConversationWorkspace,
  OverviewDataNotice,
  OverviewEmptyState,
  OverviewMetricCard,
  OverviewModuleHeader,
  OverviewTrendChart,
} from './dashboard-overview-widgets';

export { parseAISummary } from './dashboard-overview-widgets';

type OverviewRange = { from: string; to: string };

const defaultPageSize = 20;
const enterpriseTimeZone = 'Asia/Shanghai';

function defaultRange(): OverviewRange {
  const now = new Date();
  const parts = new Intl.DateTimeFormat('en-US', {
    timeZone: enterpriseTimeZone, year: 'numeric', month: '2-digit', day: '2-digit',
  }).formatToParts(now);
  const values = Object.fromEntries(parts.map((part) => [part.type, part.value]));
  const year = Number(values.year ?? 0);
  const month = Number(values.month ?? 0);
  return {
    from: `${year}-${String(month).padStart(2, '0')}-01`,
    to: `${month === 12 ? year + 1 : year}-${String(month === 12 ? 1 : month + 1).padStart(2, '0')}-01`,
  };
}

function formatDateInZone(date: Date): string {
  const parts = new Intl.DateTimeFormat('en-US', {
    timeZone: enterpriseTimeZone, year: 'numeric', month: '2-digit', day: '2-digit',
  }).formatToParts(date);
  const values = Object.fromEntries(parts.map((part) => [part.type, part.value]));
  return `${values.year ?? 0}-${String(values.month ?? 0).padStart(2, '0')}-${String(values.day ?? 0).padStart(2, '0')}`;
}

function lastSevenDays(): OverviewRange {
  const today = new Date();
  const from = new Date(today);
  from.setDate(from.getDate() - 6);
  const to = new Date(today);
  to.setDate(to.getDate() + 1);
  return { from: formatDateInZone(from), to: formatDateInZone(to) };
}

function positiveInteger(value: string | null, fallback: number): number {
  const parsed = Number(value);
  return Number.isInteger(parsed) && parsed > 0 ? parsed : fallback;
}

function filtersFromSearch(search: URLSearchParams, fallback: OverviewRange): OverviewRange {
  return {
    from: search.get('startDate') ?? fallback.from,
    to: search.get('endDate') ?? fallback.to,
  };
}

function calendarDaySpan(range: OverviewRange): number {
  const from = Date.parse(`${range.from}T00:00:00Z`);
  const to = Date.parse(`${range.to}T00:00:00Z`);
  return (to - from) / (24 * 60 * 60 * 1000);
}

function overviewSearch(current: URLSearchParams, draft: OverviewRange, page: number, pageSize: number): URLSearchParams {
  const next = updateSearch(current, { startDate: draft.from, endDate: draft.to, page, pageSize });
  next.delete('employeeIds');
  next.delete('departmentIds');
  next.delete('period');
  return next;
}

function saveCsv(blob: Blob): void {
  const href = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = href;
  link.download = 'dashboard-overview.csv';
  link.click();
  URL.revokeObjectURL(href);
}

function BusinessDashboard({ data, trendDraft, onTrendDraftChange, onTrendApply }: {
  data: Awaited<ReturnType<DashboardOverviewApi['load']>>;
  trendDraft: OverviewRange;
  onTrendDraftChange: (value: OverviewRange) => void;
  onTrendApply: () => void;
}) {
  const archiveUnavailable = data.limitations.some((item) => item.provider === 'conversation_archive');
  const aiInsight = data.aiInsight;
  const conversation = data.conversation;
  const snapshotMissing = Object.values(data.summary).some((value) => value === null);

  return <div className="overview-dashboard">
    {data.limitations.length > 0 && <div aria-label="数据限制" className="overview-data-alerts">
      {data.limitations.map((item) => <OverviewDataNotice
        description="当前模块已按真实 Provider 返回结果展示，未使用缺失数据填充。"
        kind="limited"
        key={`${item.provider}-${item.code}`}
        limitations={[item]}
        title={`数据受限：${item.provider}`}
      />)}
    </div>}
    <section aria-labelledby="overview-snapshot-title" className="overview-module overview-snapshot dashboard-data-card">
      <OverviewModuleHeader
        description="客户、线索、订单与行为事件"
        extra={<span className="overview-update">更新于 {data.updatedAt || '--'}</span>}
        headingId="overview-snapshot-title"
        title="经营快照"
      />
      {snapshotMissing && <OverviewDataNotice
        description="概览接口没有返回全部经营快照字段，已保留缺失状态；补齐对应数据后这里会自动展示。"
        kind="limited"
        title="经营快照数据暂缺"
      />}
      <div className="overview-metric-grid dashboard-stat-grid">
        <OverviewMetricCard label={data.cards[0]?.label ?? '客户总数'} note="SCRM 真实数据" value={data.summary.customer} />
        <OverviewMetricCard label={data.cards[1]?.label ?? '线索总数'} note="当前企业范围" tone="green" value={data.summary.lead} />
        <OverviewMetricCard label={data.cards[2]?.label ?? '订单总数'} note="订单真实数据" tone="violet" value={data.summary.order} />
        <OverviewMetricCard label={data.cards[3]?.label ?? '行为事件'} note="可审计业务行为" tone="orange" value={data.summary.behavior} />
      </div>
    </section>

    <div className="overview-intelligence-grid">
      <section aria-labelledby="overview-ai-title" className="overview-module dashboard-data-card">
        <OverviewModuleHeader
          description="真实 AI 分析摘要的快速入口"
          extra={aiInsight?.capability === 'ready'
            ? <span className="overview-update">生成于 {aiInsight.generatedAt.replace('T', ' ').replace('Z', '').slice(0, 19)}</span>
            : <span className="overview-capability-chip">待配置</span>}
          headingId="overview-ai-title"
          title="AI 洞察"
        />
        <OverviewAIInsightGrid insight={aiInsight} />
      </section>

      <section aria-labelledby="overview-growth-title" className="overview-module dashboard-data-card">
        <OverviewModuleHeader
          description="按天统计新增联系人"
          extra={<span className="overview-scope-chip">SCRM · Asia/Shanghai</span>}
          headingId="overview-growth-title"
          title="客户增长趋势"
        />
        <div className="overview-trend-range">
          <DateRangeFields
            value={{ startDate: trendDraft.from, endDate: trendDraft.to }}
            endLabel="趋势结束"
            startLabel="趋势开始"
            submitLabel="应用区间"
            onChange={(value) => onTrendDraftChange({ from: value.startDate, to: value.endDate })}
            onValidSubmit={() => onTrendApply()}
          />
        </div>
        {data.trend.length === 0
          ? <OverviewEmptyState description="当前日期范围没有新增客户记录。" title="暂无增长趋势" />
          : <OverviewTrendChart points={data.trend} />}
      </section>
    </div>

    <section aria-labelledby="overview-conversation-title" className="overview-module dashboard-data-card">
      <OverviewModuleHeader
        description="左侧指标控件，右侧近七日趋势"
        extra={<span className="overview-scope-chip">数据来源：会话归档</span>}
        headingId="overview-conversation-title"
        title="会话工作台"
      />
      <OverviewConversationWorkspace conversation={conversation} limitations={data.limitations} unavailable={archiveUnavailable} />
    </section>

    <div className="overview-capability-grid-layout">
      <OverviewCapabilityPanel
        description="风险监控与敏感词检查状态"
        items={[{ label: '敏感词命中' }, { label: '风险行为' }, { label: '客户流失' }]}
        source="风险监控/敏感词接口"
        title="质检数据"
      />
      <OverviewCapabilityPanel
        description="按员工汇总会话表现"
        items={[{ label: '会话员工' }, { label: '会话总数' }, { label: '平均响应' }]}
        source="员工会话分析接口"
        title="员工会话数据排行"
      />
      <OverviewCapabilityPanel
        description="最近会话与消息轨迹"
        items={[{ label: '最新会话' }, { label: '会话时间' }, { label: '消息数量' }]}
        source="员工会话轨迹接口"
        title="员工会话轨迹一览"
      />
    </div>
  </div>;
}

export function DashboardOverviewPage({ api, initialRange }: { api: DashboardOverviewApi; initialRange?: OverviewRange; optionsApi?: Phase35Api | undefined }) {
  const access = useDashboardAccess();
  const [searchParams, setSearchParams] = useSearchParams();
  const fallback = initialRange ?? defaultRange();
  const searchText = searchParams.toString();
  const current = filtersFromSearch(searchParams, fallback);
  const page = positiveInteger(searchParams.get('page'), 1);
  const pageSize = Math.min(100, positiveInteger(searchParams.get('pageSize'), defaultPageSize));
  const [draft, setDraft] = useState<OverviewRange>(current);
  const [rangeError, setRangeError] = useState<string | null>(null);
  const [exportError, setExportError] = useState<string | null>(null);
  const [trendRange, setTrendRange] = useState<OverviewRange>(() => lastSevenDays());
  const [trendDraft, setTrendDraft] = useState<OverviewRange>(() => lastSevenDays());

  useEffect(() => { setDraft(filtersFromSearch(new URLSearchParams(searchText), fallback)); }, [fallback.from, fallback.to, searchText]);

  const input = useMemo<DashboardOverviewQuery>(() => ({
    corpId: access.corp.id,
    startDate: current.from,
    endDate: current.to,
    trendStartDate: trendRange.from,
    trendEndDate: trendRange.to,
    employeeIds: [],
    departmentIds: [],
    page,
    pageSize,
  }), [access.corp.id, current.from, current.to, page, pageSize, trendRange.from, trendRange.to]);
  const query = useQuery({ queryKey: ['corp', access.corp.id, 'dashboard-overview', input], queryFn: () => api.load(input) });

  function applyFilters() {
    if (draft.from === '' || draft.to === '' || draft.from > draft.to) { setRangeError('请选择有效的日期范围'); return; }
    if (calendarDaySpan(draft) > 31) { setRangeError('日期范围最多为 31 天'); return; }
    setRangeError(null);
    setSearchParams(overviewSearch(searchParams, draft, 1, pageSize));
  }

  async function exportCsv() {
    setExportError(null);
    try { saveCsv(await api.exportCsv(input)); } catch (error) { setExportError(error instanceof Error ? error.message : '导出失败，请稍后重试'); }
  }

  const forbidden = query.error instanceof ApiError && query.error.kind === 'forbidden';
  return <section className="dashboard-overview-page">
    <header className="dashboard-overview-header dashboard-page-header dashboard-data-card">
      <div className="dashboard-overview-heading">
        <p className="dashboard-overview-eyebrow">数据中心</p>
        <div className="dashboard-overview-title-row"><h1>数据概览</h1><span>统一报表口径</span></div>
        <p>当前企业：<strong>{access.corp.name}</strong> · 查看客户、转化、行为与会话经营状态。</p>
      </div>
      <div className="dashboard-overview-filters dashboard-filter-bar">
        <DateRangeFields value={{ startDate: draft.from, endDate: draft.to }} onChange={(value) => setDraft((currentDraft) => ({ ...currentDraft, from: value.startDate, to: value.endDate }))} onValidSubmit={applyFilters} />
        <div className="dashboard-overview-actions">
          <Button aria-label="刷新" disabled={query.isFetching} loading={query.isFetching} onClick={() => void query.refetch()}>刷新</Button>
          <Button aria-label="导出 CSV" disabled={query.isFetching} onClick={() => void exportCsv()}>导出 CSV</Button>
        </div>
      </div>
    </header>
    {rangeError !== null && <p className="dashboard-overview-inline-error" role="alert">{rangeError}</p>}
    {exportError !== null && <p className="dashboard-overview-inline-error" role="alert">{exportError}</p>}
    {query.isPending && <PageState state="loading" title="正在加载数据概览" />}
    {forbidden && <PageState description="请切换到已授权企业，或联系管理员开通权限。" state="forbidden" title="无权访问当前企业数据" />}
    {query.isError && !forbidden && <PageState description={query.error instanceof Error ? query.error.message : '请稍后重试'} onRetry={() => void query.refetch()} retryLabel="重试" state="error" title="数据加载失败" />}
    {query.data !== undefined && !query.isError && <BusinessDashboard
      data={query.data}
      onTrendApply={() => setTrendRange(trendDraft)}
      onTrendDraftChange={setTrendDraft}
      trendDraft={trendDraft}
    />}
  </section>;
}
