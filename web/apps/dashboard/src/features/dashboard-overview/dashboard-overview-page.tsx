import { ApiError } from '@mochat/api-client';
import { useQuery } from '@tanstack/react-query';
import { useEffect, useMemo, useState } from 'react';
import type { ReactNode } from 'react';
import { Link, useSearchParams } from 'react-router';

import { useDashboardAccess } from '../../app/access-context';
import { PageState } from '../../components/page-state/page-state';
import { DashboardPagination } from '../../components/dashboard-pagination';
import { DateRangeFields } from '../../components/date-range-fields';
import type { Phase35Api } from '../phase35/api';
import { updateSearch } from '../../shared/query-state';
import type {
  DashboardOverviewApi,
  DashboardOverviewQuery,
  ConversationTrendPoint,
  DashboardOverviewTrendPoint,
} from './dashboard-overview-api';

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

function trendMaximum(points: readonly DashboardOverviewTrendPoint[]): number {
  return Math.max(1, ...points.map((point) => point.addCustomerNum));
}

function TrendChart({ points }: { points: readonly DashboardOverviewTrendPoint[] }) {
  const maximum = trendMaximum(points);
  return <div className="dashboard-overview-chart" role="img" aria-label="企业趋势图">
    {points.map((point) => <div className="dashboard-overview-chart-column" key={point.date}>
      <span className="dashboard-overview-bar-value">{point.addCustomerNum}</span>
      <div className="dashboard-overview-bars">
        <span aria-label={`新增客户 ${point.addCustomerNum}`} className="dashboard-overview-bar dashboard-overview-bar-primary" style={{ height: `${Math.max(4, (point.addCustomerNum / maximum) * 100)}%` }} title={`新增客户 ${point.addCustomerNum}`} />
      </div>
      <span>{point.date}</span>
    </div>)}
  </div>;
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

function MetricCard({ label, value, note, tone = 'blue' }: { label: string; value: number; note: string; tone?: string }) {
  return <article className={`overview-metric overview-metric-${tone}`}>
    <span className="overview-metric-icon" aria-hidden="true" />
    <div><strong>{value.toLocaleString('zh-CN')}</strong><span>{label}</span><small>{note}</small></div>
  </article>;
}

function ModuleHeader({ title, description, extra }: { title: string; description?: string; extra?: ReactNode }) {
  return <header className="overview-module-header"><div><h2>{title}</h2>{description && <p>{description}</p>}</div>{extra}</header>;
}

function EmptyVisual({ text = '暂无数据' }: { text?: string }) {
  return <div className="overview-empty-visual"><span aria-hidden="true">⌁</span><p>{text}</p></div>;
}

type AISummaryLine =
  | { kind: 'divider' }
  | { kind: 'section'; text: string }
  | { kind: 'numbered'; index: string; text: string }
  | { kind: 'bullet'; text: string }
  | { kind: 'paragraph'; text: string };

export function parseAISummary(summary: string): AISummaryLine[] {
  return summary.split(/\r?\n/).map((raw) => {
    const line = raw.trim();
    if (line === '') return null;
    if (/^-{3,}$/.test(line)) return { kind: 'divider' };
    if (/^[✅⚠️]/.test(line)) return { kind: 'section', text: line };
    const numbered = line.match(/^(\d+)[.、．]\s*(.*)$/);
    if (numbered) return { kind: 'numbered', index: numbered[1], text: numbered[2] };
    if (/^[-•]\s+/.test(line)) return { kind: 'bullet', text: line.replace(/^[-•]\s+/, '') };
    return { kind: 'paragraph', text: line };
  }).filter((line): line is AISummaryLine => line !== null);
}

function renderInline(text: string): ReactNode {
  const parts = text.split(/\*\*(.+?)\*\*/g);
  return parts.map((part, index) => (index % 2 === 1 ? <strong key={index}>{part}</strong> : part));
}

function AISummary({ summary }: { summary: string }) {
  const lines = parseAISummary(summary);
  return <div className="overview-ai-summary">
    {lines.map((line, index) => {
      switch (line.kind) {
        case 'divider':
          return <hr className="overview-ai-summary-divider" key={index} />;
        case 'section':
          return <p className="overview-ai-summary-section" key={index}>{renderInline(line.text)}</p>;
        case 'numbered':
          return <p className="overview-ai-summary-item" key={index}>
            <span className="overview-ai-summary-index">{line.index}.</span>
            <span>{renderInline(line.text)}</span>
          </p>;
        case 'bullet':
          return <p className="overview-ai-summary-bullet" key={index}>{renderInline(line.text)}</p>;
        default:
          return <p className="overview-ai-summary-paragraph" key={index}>{renderInline(line.text)}</p>;
      }
    })}
  </div>;
}

function conversationKindLabel(kind: 'customer' | 'room'): string {
  return kind === 'room' ? '客户群' : '客户会话';
}

function ConversationTrendChart({ kind, points }: { kind: 'customer' | 'room'; points: readonly ConversationTrendPoint[] }) {
  const maximum = Math.max(1, ...points.map((point) => (kind === 'customer' ? point.customerSessions : point.roomSessions)));
  return <div className="dashboard-overview-chart" role="img" aria-label={`近七日${conversationKindLabel(kind)}趋势`}>
    {points.map((point) => <div className="dashboard-overview-chart-column" key={point.date}>
      <span className="dashboard-overview-bar-value">{kind === 'customer' ? point.customerSessions : point.roomSessions}</span>
      <div className="dashboard-overview-bars">
        <span aria-label={`会话数 ${kind === 'customer' ? point.customerSessions : point.roomSessions}`} className="dashboard-overview-bar dashboard-overview-bar-primary" style={{ height: `${Math.max(4, ((kind === 'customer' ? point.customerSessions : point.roomSessions) / maximum) * 100)}%` }} title={`会话数 ${kind === 'customer' ? point.customerSessions : point.roomSessions}`} />
      </div>
      <span>{point.date.slice(5)}</span>
    </div>)}
  </div>;
}

function ConversationGroupBlock({ title, data, active, onClick }: {
  title: string;
  data: { sessions: number; employeeMessages: number; customerMessages: number };
  active: boolean;
  onClick: () => void;
}) {
  return <button aria-pressed={active} className={`overview-conversation-group${active ? ' overview-conversation-group-active' : ''}`} onClick={onClick} type="button">
    <h3>{title}</h3>
    <span className="overview-conversation-group-note">区间汇总</span>
    <dl>
      <div><dt>会话数</dt><dd>{data.sessions.toLocaleString('zh-CN')}</dd></div>
      <div><dt>员工消息数</dt><dd>{data.employeeMessages.toLocaleString('zh-CN')}</dd></div>
      <div><dt>客户消息数</dt><dd>{data.customerMessages.toLocaleString('zh-CN')}</dd></div>
    </dl>
  </button>;
}

function BusinessDashboard({ data, page, pageSize, searchParams, setSearchParams, current, trendRange, trendDraft, onTrendDraftChange, onTrendApply }: {
  data: Awaited<ReturnType<DashboardOverviewApi['load']>>;
  page: number;
  pageSize: number;
  searchParams: URLSearchParams;
  setSearchParams: ReturnType<typeof useSearchParams>[1];
  current: OverviewRange;
  trendRange: OverviewRange;
  trendDraft: OverviewRange;
  onTrendDraftChange: (value: OverviewRange) => void;
  onTrendApply: () => void;
}) {
  const total = data.total ?? data.trend.length;
  const archiveUnavailable = data.limitations.some((item) => item.provider === 'conversation_archive');
  const [activeConversationKind, setActiveConversationKind] = useState<'customer' | 'room'>('customer');
  const aiInsight = data.aiInsight;
  const conversation = data.conversation;

  return <div className="overview-dashboard">
    <section className="overview-module dashboard-data-card">
      <ModuleHeader title="经营概览" description="客户、线索、订单与行为事件，与数据报表同口径" extra={<span className="overview-update">更新于 {data.updatedAt || '--'}</span>} />
      <div className="overview-metric-grid dashboard-stat-grid">
        <MetricCard label={data.cards[0]?.label ?? '客户总数'} value={data.summary.customer} note="同客户分析口径（联系人+分配关系）" />
        <MetricCard label={data.cards[1]?.label ?? '线索总数'} value={data.summary.lead} note="转化漏斗起始阶段" tone="green" />
        <MetricCard label={data.cards[2]?.label ?? '订单总数'} value={data.summary.order} note="won/completed/paid 口径" tone="violet" />
        <MetricCard label={data.cards[3]?.label ?? '行为事件'} value={data.summary.behavior} note="订单与设置审计事件" tone="orange" />
      </div>
    </section>

    <section className="overview-module dashboard-data-card">
      <ModuleHeader
        title="AI 洞察"
        description={aiInsight?.capability === 'ready' ? '每日 24 点自动生成的会话智能分析，页面只读展示' : 'AI 能力未接入，配置后展示会话风险与情绪信号'}
        extra={aiInsight?.capability === 'ready'
          ? <span className="overview-update">生成于 {aiInsight.generatedAt.replace('T', ' ').replace('Z', '').slice(0, 19)}</span>
          : <Link className="overview-link-button" to="/ai-setting/ai-knowledge-base">前往配置</Link>}
      />
      {aiInsight?.capability === 'ready' && aiInsight.summary !== ''
        ? <AISummary summary={aiInsight.summary} />
        : <EmptyVisual text="AI 能力未接入，请在 AI 设置中配置后查看" />}
    </section>

    <section className="overview-module dashboard-data-card">
      <ModuleHeader title="客户增长趋势" description="按天统计新增联系人，可选择展示日期区间（默认最近七天）" extra={<span className="overview-scope-chip">数据来源：SCRM</span>} />
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
      {data.trend.length === 0 ? <EmptyVisual /> : <TrendChart points={data.trend} />}
    </section>

    <section className="overview-module dashboard-data-card">
      <ModuleHeader title="会话数据" description="左侧为当前筛选区间的会话汇总，右侧为近七日每日趋势（点击左侧切换客户会话/客户群）" extra={<span className="overview-scope-chip">数据来源：会话归档</span>} />
      {archiveUnavailable || conversation === undefined
        ? <EmptyVisual text="暂无会话归档数据，配置企业微信会话存档后展示" />
        : <div className="overview-conversation">
            <div className="overview-conversation-groups">
              <ConversationGroupBlock
                active={activeConversationKind === 'customer'}
                data={conversation.customer}
                onClick={() => setActiveConversationKind('customer')}
                title="客户会话"
              />
              <ConversationGroupBlock
                active={activeConversationKind === 'room'}
                data={conversation.room}
                onClick={() => setActiveConversationKind('room')}
                title="客户群"
              />
            </div>
            <div className="overview-conversation-chart">
              <h3>{conversationKindLabel(activeConversationKind)} · 近七日趋势</h3>
              <div className="overview-conversation-legend"><span className="legend-customer">每日会话数</span></div>
              {conversation.trend.length === 0 ? <EmptyVisual /> : <ConversationTrendChart kind={activeConversationKind} points={conversation.trend} />}
            </div>
          </div>}
      {conversation !== undefined && (
        <div aria-label="近七日会话趋势明细" className="overview-conversation-detail">
          <header><h3>{conversationKindLabel(activeConversationKind)} · 近七日趋势明细</h3></header>
          <div className="dashboard-table-scroll"><table><thead><tr><th>日期</th><th>会话数</th><th>员工消息数</th><th>客户消息数</th></tr></thead><tbody>
            {conversation.trend.map((point) => <tr key={point.date}>
              <td>{point.date}</td>
              <td>{activeConversationKind === 'customer' ? point.customerSessions : point.roomSessions}</td>
              <td>{activeConversationKind === 'customer' ? point.customerEmployeeMessages : point.roomEmployeeMessages}</td>
              <td>{activeConversationKind === 'customer' ? point.customerCustomerMessages : point.roomCustomerMessages}</td>
            </tr>)}
          </tbody></table></div>
        </div>
      )}
    </section>

    <section className="overview-module overview-detail-table dashboard-data-card">
      <ModuleHeader title="经营趋势明细" description="与当前筛选和导出范围保持一致" />
      {data.trend.length === 0 ? <EmptyVisual /> : <div className="dashboard-table-scroll"><table><thead><tr><th>日期</th><th>新增客户</th></tr></thead><tbody>{data.trend.map((point) => <tr key={point.date}><td>{point.date}</td><td>{point.addCustomerNum}</td></tr>)}</tbody></table></div>}
      <DashboardPagination
        page={page}
        pageSize={pageSize}
        total={total}
        onPageChange={(nextPage) => setSearchParams(overviewSearch(searchParams, current, nextPage, pageSize))}
      />
    </section>
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
      <div><p className="dashboard-overview-eyebrow">数据中心</p><h1>数据概览</h1><p>查看当前企业客户、线索、订单与行为数据（与数据报表同口径）。</p></div>
      <div className="dashboard-overview-filters dashboard-filter-bar">
        <DateRangeFields value={{ startDate: draft.from, endDate: draft.to }} onChange={(value) => setDraft((currentDraft) => ({ ...currentDraft, from: value.startDate, to: value.endDate }))} onValidSubmit={applyFilters} />
        <button disabled={query.isFetching} onClick={() => void query.refetch()} type="button">刷新</button><button disabled={query.isFetching} onClick={() => void exportCsv()} type="button">导出 CSV</button>
      </div>
    </header>
    {rangeError !== null && <p className="dashboard-overview-inline-error" role="alert">{rangeError}</p>}
    {exportError !== null && <p className="dashboard-overview-inline-error" role="alert">{exportError}</p>}
    {query.isPending && <PageState state="loading" title="正在加载数据概览" />}
    {forbidden && <PageState description="请切换到已授权企业，或联系管理员开通权限。" state="forbidden" title="无权访问当前企业数据" />}
    {query.isError && !forbidden && <PageState description={query.error instanceof Error ? query.error.message : '请稍后重试'} onRetry={() => void query.refetch()} retryLabel="重试" state="error" title="数据加载失败" />}
    {query.data !== undefined && !query.isError && <BusinessDashboard
      current={current}
      data={query.data}
      onTrendApply={() => setTrendRange(trendDraft)}
      onTrendDraftChange={setTrendDraft}
      page={page}
      pageSize={pageSize}
      searchParams={searchParams}
      setSearchParams={setSearchParams}
      trendDraft={trendDraft}
      trendRange={trendRange}
    />}
  </section>;
}
