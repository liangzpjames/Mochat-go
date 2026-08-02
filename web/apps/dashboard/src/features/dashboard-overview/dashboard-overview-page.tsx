import { ApiError } from '@mochat/api-client';
import { useQuery } from '@tanstack/react-query';
import { useEffect, useMemo, useState } from 'react';
import type { ReactNode } from 'react';
import { Link, useSearchParams } from 'react-router';

import { useDashboardAccess } from '../../app/access-context';
import { PageState } from '../../components/page-state/page-state';
import { updateSearch } from '../../shared/query-state';
import type {
  DashboardOverviewApi,
  DashboardOverviewQuery,
  DashboardOverviewTrendPoint,
} from './dashboard-overview-api';

type OverviewRange = { from: string; to: string };
type FilterDraft = OverviewRange & { employeeIds: string; departmentIds: string; period: 'day' | 'week' | 'month' };

const defaultPageSize = 20;
const enterpriseTimeZone = 'Asia/Shanghai';

function enterpriseDateText(value: Date): string {
  const parts = new Intl.DateTimeFormat('en-US', {
    timeZone: enterpriseTimeZone,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  }).formatToParts(value);
  const values = Object.fromEntries(parts.map((part) => [part.type, part.value]));
  const year = values.year ?? '';
  const month = values.month ?? '';
  const date = values.day ?? '';
  return `${year}-${month}-${date}`;
}

function defaultRange(): OverviewRange {
  const to = enterpriseDateText(new Date());
  const from = new Date(`${to}T00:00:00+08:00`);
  from.setUTCDate(from.getUTCDate() - 30);
  return { from: enterpriseDateText(from), to };
}

function positiveInteger(value: string | null, fallback: number): number {
  const parsed = Number(value);
  return Number.isInteger(parsed) && parsed > 0 ? parsed : fallback;
}

function idsFromSearch(search: URLSearchParams, key: string): string[] {
  return [...new Set(search.getAll(key)
    .flatMap((value) => value.split(','))
    .map((value) => value.trim())
    .filter((value) => /^\d+$/.test(value) && Number(value) > 0))];
}

function idsFromText(value: string): string[] {
  return [...new Set(value.split(',').map((item) => item.trim()).filter((item) => /^\d+$/.test(item) && Number(item) > 0))];
}

function filtersFromSearch(search: URLSearchParams, fallback: OverviewRange): FilterDraft {
  const period = search.get('period');
  return {
    from: search.get('startDate') ?? fallback.from,
    to: search.get('endDate') ?? fallback.to,
    employeeIds: idsFromSearch(search, 'employeeIds').join(','),
    departmentIds: idsFromSearch(search, 'departmentIds').join(','),
    period: period === 'week' || period === 'month' ? period : 'day',
  };
}

function calendarDaySpan(range: OverviewRange): number {
  const from = Date.parse(`${range.from}T00:00:00Z`);
  const to = Date.parse(`${range.to}T00:00:00Z`);
  return (to - from) / (24 * 60 * 60 * 1000);
}

function trendMaximum(points: readonly DashboardOverviewTrendPoint[]): number {
  return Math.max(1, ...points.flatMap((point) => [point.addContactNum, point.addIntoRoomNum, point.lossContactNum, point.quitRoomNum]));
}

function TrendChart({ points }: { points: readonly DashboardOverviewTrendPoint[] }) {
  const maximum = trendMaximum(points);
  return <div className="dashboard-overview-chart" role="img" aria-label="企业趋势图">
    {points.map((point) => <div className="dashboard-overview-chart-column" key={point.date}>
      <div className="dashboard-overview-bars">
        <span aria-label={`新增客户 ${point.addContactNum}`} className="dashboard-overview-bar dashboard-overview-bar-primary" style={{ height: `${Math.max(4, (point.addContactNum / maximum) * 100)}%` }} title={`新增客户 ${point.addContactNum}`} />
        <span aria-label={`新增入群 ${point.addIntoRoomNum}`} className="dashboard-overview-bar dashboard-overview-bar-secondary" style={{ height: `${Math.max(4, (point.addIntoRoomNum / maximum) * 100)}%` }} title={`新增入群 ${point.addIntoRoomNum}`} />
        <span aria-label={`流失客户 ${point.lossContactNum}`} className="dashboard-overview-bar dashboard-overview-bar-warning" style={{ height: `${Math.max(4, (point.lossContactNum / maximum) * 100)}%` }} title={`流失客户 ${point.lossContactNum}`} />
        <span aria-label={`退出群聊 ${point.quitRoomNum}`} className="dashboard-overview-bar dashboard-overview-bar-muted" style={{ height: `${Math.max(4, (point.quitRoomNum / maximum) * 100)}%` }} title={`退出群聊 ${point.quitRoomNum}`} />
      </div>
      <span>{point.date}</span>
    </div>)}
  </div>;
}

function overviewSearch(current: URLSearchParams, draft: FilterDraft, page: number, pageSize: number): URLSearchParams {
  const next = updateSearch(current, { startDate: draft.from, endDate: draft.to, period: draft.period, page, pageSize });
  next.delete('employeeIds');
  next.delete('departmentIds');
  for (const id of idsFromText(draft.employeeIds)) next.append('employeeIds', id);
  for (const id of idsFromText(draft.departmentIds)) next.append('departmentIds', id);
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

function BusinessDashboard({ data, period, page, pageSize, searchParams, setSearchParams, current, onRefresh }: {
  data: Awaited<ReturnType<DashboardOverviewApi['load']>>;
  period: FilterDraft['period'];
  page: number;
  pageSize: number;
  searchParams: URLSearchParams;
  setSearchParams: ReturnType<typeof useSearchParams>[1];
  current: FilterDraft;
  onRefresh: () => void;
}) {
  const summary = data.summary ?? {
    weChatContactNum: 0, weChatRoomNum: 0, roomMemberNum: 0, corpMemberNum: 0,
    addContactNum: 0, lastAddContactNum: 0, addIntoRoomNum: 0, lastAddIntoRoomNum: 0,
    lossContactNum: 0, lastLossContactNum: 0, quitRoomNum: 0, lastQuitRoomNum: 0,
  };
  const total = data.total ?? data.trend.length;
  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  const comparison = (value: number, previous: number) => value === previous ? '较昨日持平' : `较昨日${value > previous ? '增加' : '减少'} ${Math.abs(value - previous)}`;

  return <div className="overview-dashboard">
    <section className="overview-module dashboard-data-card">
      <ModuleHeader title="经营概览" description="客户增长、员工存档与群聊经营情况" extra={<span className="overview-update">更新于 {data.updatedAt || '--'}</span>} />
      <div className="overview-metric-grid dashboard-stat-grid">
        <MetricCard label={data.cards[0]?.label ?? '客户总数'} value={summary.weChatContactNum} note={`今日新增 ${summary.addContactNum} · ${comparison(summary.addContactNum, summary.lastAddContactNum)}`} />
        <MetricCard label={data.cards[1]?.label ?? '客户群总数'} value={summary.weChatRoomNum} note={`今日新增入群 ${summary.addIntoRoomNum}`} tone="green" />
        <MetricCard label={data.cards[3]?.label ?? '已开启存档员工'} value={summary.corpMemberNum} note={`当前员工总数 ${summary.corpMemberNum}`} tone="violet" />
        <MetricCard label={data.cards[2]?.label ?? '群成员总数'} value={summary.roomMemberNum} note={`今日流失 ${summary.lossContactNum} · 退出群聊 ${summary.quitRoomNum}`} tone="orange" />
      </div>
    </section>

    <section className="overview-module dashboard-data-card">
      <ModuleHeader title="AI 洞察" description="开启智能体监测后，系统会持续汇总会话风险与情绪信号" extra={<Link className="overview-link-button" to="/ai-insight/v2/sensitive-word">查看详情</Link>} />
      <div className="overview-compact-grid">
        {['AI 分析次数', '员工负面情绪', '客户负面情绪', '风险行为', '敏感词'].map((label, index) => <div key={label}><span>{label}</span><strong>0</strong><small>{index === 0 ? '今日分析 0' : '今日新增 0'}</small></div>)}
      </div>
    </section>

    <div className="overview-two-column">
      <section className="overview-module dashboard-data-card">
        <ModuleHeader title="会话数据" extra={<span className="overview-scope-chip">团队 · {period === 'day' ? '每日' : period === 'week' ? '每周' : '每月'}</span>} />
        <div className="overview-split-stats"><div><h3>客户会话</h3><p><strong>0</strong><span>会话数</span></p><p><strong>0</strong><span>员工消息</span></p><p><strong>0</strong><span>客户消息</span></p></div><div><h3>客户群</h3><p><strong>{summary.addIntoRoomNum}</strong><span>新增入群</span></p><p><strong>{summary.weChatRoomNum}</strong><span>客户群</span></p><p><strong>{summary.roomMemberNum}</strong><span>群成员</span></p></div></div>
        <div className="overview-chart-panel"><h3>客户增长趋势</h3>{data.trend.length === 0 ? <EmptyVisual /> : <TrendChart points={data.trend} />}</div>
      </section>

      <section className="overview-module dashboard-data-card">
        <ModuleHeader title="质检数据" extra={<span className="overview-scope-chip">团队 · 近 7 日</span>} />
        <div className="overview-split-stats"><div><h3>风险监控</h3><p><strong>0</strong><span>敏感词命中</span></p><p><strong>0</strong><span>风险行为</span></p><p><strong>{summary.lossContactNum}</strong><span>客户流失</span></p></div><div><h3>超时预警</h3><p><strong>0</strong><span>超时触发</span></p><p><strong>0</strong><span>单聊触发</span></p><p><strong>0</strong><span>群聊触发</span></p></div></div>
        <div className="overview-chart-panel"><h3>风险监控趋势</h3><EmptyVisual /></div>
      </section>
    </div>

    <div className="overview-two-column overview-lower-grid">
      <section className="overview-module dashboard-data-card"><ModuleHeader title="员工会话数据排行" extra={<span className="overview-scope-chip">按会话总数 · 近 7 日</span>} /><EmptyVisual text="当前范围暂无员工排行" /></section>
      <section className="overview-module dashboard-data-card"><ModuleHeader title="员工会话轨迹一览" extra={<button className="overview-link-button" onClick={onRefresh} type="button">刷新轨迹</button>} /><EmptyVisual text="当前范围暂无会话轨迹" /></section>
    </div>

    <section className="overview-module overview-detail-table dashboard-data-card">
      <ModuleHeader title="经营趋势明细" description="与当前筛选和导出范围保持一致" />
      {data.trend.length === 0 ? <EmptyVisual /> : <div className="dashboard-table-scroll"><table><thead><tr><th>日期</th><th>新增客户</th><th>新增入群</th><th>流失客户</th><th>退出群聊</th></tr></thead><tbody>{data.trend.map((point) => <tr key={point.date}><td>{point.date}</td><td>{point.addContactNum}</td><td>{point.addIntoRoomNum}</td><td>{point.lossContactNum}</td><td>{point.quitRoomNum}</td></tr>)}</tbody></table></div>}
      <footer className="dashboard-table-actions"><span>共 {total} 条，第 {page}/{totalPages} 页</span><div><button disabled={page <= 1} onClick={() => setSearchParams(overviewSearch(searchParams, current, page - 1, pageSize))} type="button">上一页</button><button disabled={page >= totalPages} onClick={() => setSearchParams(overviewSearch(searchParams, current, page + 1, pageSize))} type="button">下一页</button></div></footer>
    </section>
  </div>;
}

export function DashboardOverviewPage({ api, initialRange }: { api: DashboardOverviewApi; initialRange?: OverviewRange }) {
  const access = useDashboardAccess();
  const [searchParams, setSearchParams] = useSearchParams();
  const fallback = initialRange ?? defaultRange();
  const searchText = searchParams.toString();
  const current = filtersFromSearch(searchParams, fallback);
  const page = positiveInteger(searchParams.get('page'), 1);
  const pageSize = Math.min(100, positiveInteger(searchParams.get('pageSize'), defaultPageSize));
  const [draft, setDraft] = useState<FilterDraft>(current);
  const [rangeError, setRangeError] = useState<string | null>(null);
  const [exportError, setExportError] = useState<string | null>(null);

  useEffect(() => { setDraft(filtersFromSearch(new URLSearchParams(searchText), fallback)); }, [fallback.from, fallback.to, searchText]);

  const input = useMemo<DashboardOverviewQuery>(() => ({
    corpId: access.corp.id,
    startDate: current.from,
    endDate: current.to,
    employeeIds: idsFromText(current.employeeIds),
    departmentIds: idsFromText(current.departmentIds),
    period: current.period,
    page,
    pageSize,
  }), [access.corp.id, current.departmentIds, current.employeeIds, current.from, current.period, current.to, page, pageSize]);
  const query = useQuery({ queryKey: ['corp', access.corp.id, 'dashboard-overview', input], queryFn: () => api.load(input) });

  function applyFilters() {
    if (draft.from === '' || draft.to === '' || draft.from > draft.to) { setRangeError('请选择有效的日期范围'); return; }
    if (calendarDaySpan(draft) > 30) { setRangeError('日期范围最多为 31 天'); return; }
    if (draft.employeeIds.trim() !== '' && idsFromText(draft.employeeIds).length === 0) { setRangeError('员工 ID 需使用逗号分隔的正整数'); return; }
    if (draft.departmentIds.trim() !== '' && idsFromText(draft.departmentIds).length === 0) { setRangeError('部门 ID 需使用逗号分隔的正整数'); return; }
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
      <div><p className="dashboard-overview-eyebrow">数据中心</p><h1>数据概览</h1><p>查看当前企业的客户、群聊与成员变化。</p></div>
      <div className="dashboard-overview-filters dashboard-filter-bar">
        <label><span>开始日期</span><input aria-label="开始日期" type="date" value={draft.from} onChange={(event) => setDraft((value) => ({ ...value, from: event.target.value }))} /></label>
        <label><span>结束日期</span><input aria-label="结束日期" type="date" value={draft.to} onChange={(event) => setDraft((value) => ({ ...value, to: event.target.value }))} /></label>
        <label><span>趋势周期</span><select aria-label="趋势周期" value={draft.period} onChange={(event) => setDraft((value) => ({ ...value, period: event.target.value as FilterDraft['period'] }))}><option value="day">日</option><option value="week">周</option><option value="month">月</option></select></label>
        <button onClick={applyFilters} type="button">查询</button><button disabled={query.isFetching} onClick={() => void query.refetch()} type="button">刷新</button><button disabled={query.isFetching} onClick={() => void exportCsv()} type="button">导出 CSV</button>
      </div>
    </header>
    <details className="overview-advanced-filters dashboard-data-card"><summary>高级范围筛选</summary><div><label><span>员工 ID</span><input aria-label="员工 ID" placeholder="多个 ID 用逗号分隔" value={draft.employeeIds} onChange={(event) => setDraft((value) => ({ ...value, employeeIds: event.target.value }))} /></label><label><span>部门 ID</span><input aria-label="部门 ID" placeholder="多个 ID 用逗号分隔" value={draft.departmentIds} onChange={(event) => setDraft((value) => ({ ...value, departmentIds: event.target.value }))} /></label></div></details>
    {rangeError !== null && <p className="dashboard-overview-inline-error" role="alert">{rangeError}</p>}
    {exportError !== null && <p className="dashboard-overview-inline-error" role="alert">{exportError}</p>}
    {query.isPending && <PageState state="loading" title="正在加载数据概览" />}
    {forbidden && <PageState description="请切换到已授权企业，或联系管理员开通权限。" state="forbidden" title="无权访问当前企业数据" />}
    {query.isError && !forbidden && <PageState description={query.error instanceof Error ? query.error.message : '请稍后重试'} onRetry={() => void query.refetch()} retryLabel="重试" state="error" title="数据加载失败" />}
    {query.data !== undefined && !query.isError && <BusinessDashboard data={query.data} period={current.period} page={page} pageSize={pageSize} searchParams={searchParams} setSearchParams={setSearchParams} current={current} onRefresh={() => void query.refetch()} />}
  </section>;
}
