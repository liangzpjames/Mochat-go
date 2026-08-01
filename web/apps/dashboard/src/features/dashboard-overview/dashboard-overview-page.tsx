import { ApiError } from '@mochat/api-client';
import { useQuery } from '@tanstack/react-query';
import { useEffect, useMemo, useState } from 'react';
import { useSearchParams } from 'react-router';

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
  const empty = !query.isError && query.data !== undefined && query.data.cards.length === 0 && query.data.trend.length === 0;
  const total = query.data?.total ?? query.data?.trend.length ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / pageSize));

  return <section className="dashboard-overview-page">
    <header className="dashboard-overview-header dashboard-page-header dashboard-data-card">
      <div><p className="dashboard-overview-eyebrow">数据中心</p><h1>数据概览</h1><p>查看当前企业的客户、群聊与成员变化。</p></div>
      <div className="dashboard-overview-filters dashboard-filter-bar">
        <label><span>开始日期</span><input aria-label="开始日期" type="date" value={draft.from} onChange={(event) => setDraft((value) => ({ ...value, from: event.target.value }))} /></label>
        <label><span>结束日期</span><input aria-label="结束日期" type="date" value={draft.to} onChange={(event) => setDraft((value) => ({ ...value, to: event.target.value }))} /></label>
        <label><span>员工 ID</span><input aria-label="员工 ID" value={draft.employeeIds} onChange={(event) => setDraft((value) => ({ ...value, employeeIds: event.target.value }))} /></label>
        <label><span>部门 ID</span><input aria-label="部门 ID" value={draft.departmentIds} onChange={(event) => setDraft((value) => ({ ...value, departmentIds: event.target.value }))} /></label>
        <label><span>趋势周期</span><select aria-label="趋势周期" value={draft.period} onChange={(event) => setDraft((value) => ({ ...value, period: event.target.value as FilterDraft['period'] }))}><option value="day">日</option><option value="week">周</option><option value="month">月</option></select></label>
        <button onClick={applyFilters} type="button">查询</button><button disabled={query.isFetching} onClick={() => void query.refetch()} type="button">刷新</button><button disabled={query.isFetching} onClick={() => void exportCsv()} type="button">导出 CSV</button>
      </div>
    </header>
    {rangeError !== null && <p className="dashboard-overview-inline-error" role="alert">{rangeError}</p>}
    {exportError !== null && <p className="dashboard-overview-inline-error" role="alert">{exportError}</p>}
    {query.isPending && <PageState state="loading" title="正在加载数据概览" />}
    {forbidden && <PageState description="请切换到已授权企业，或联系管理员开通权限。" state="forbidden" title="无权访问当前企业数据" />}
    {query.isError && !forbidden && <PageState description={query.error instanceof Error ? query.error.message : '请稍后重试'} onRetry={() => void query.refetch()} retryLabel="重试" state="error" title="数据加载失败" />}
    {empty && <PageState description="调整日期范围后重新查询。" state="empty" title="当前日期范围暂无数据" />}
    {query.data !== undefined && !query.isError && !empty && <>
      <div className="dashboard-overview-cards dashboard-stat-grid">{query.data.cards.map((card) => <article className="dashboard-overview-card dashboard-data-card" key={card.key}><span>{card.label}</span><strong>{card.value.toLocaleString('zh-CN')}</strong></article>)}</div>
      <article className="dashboard-overview-trend dashboard-data-card"><header><div><h2>企业数据趋势</h2><p>新增客户、入群、流失与退群的{current.period === 'day' ? '每日' : current.period === 'week' ? '每周' : '每月'}变化</p></div><p>更新时间：{query.data.updatedAt || '--'}</p></header>{query.data.trend.length === 0 ? <p className="dashboard-overview-trend-empty">当前日期范围暂无趋势数据</p> : <TrendChart points={query.data.trend} />}</article>
      <article className="dashboard-overview-table dashboard-data-card"><div className="dashboard-table-scroll"><table><thead><tr><th>日期</th><th>新增客户</th><th>新增入群</th><th>流失客户</th><th>退出群聊</th></tr></thead><tbody>{query.data.trend.map((point) => <tr key={point.date}><td>{point.date}</td><td>{point.addContactNum}</td><td>{point.addIntoRoomNum}</td><td>{point.lossContactNum}</td><td>{point.quitRoomNum}</td></tr>)}</tbody></table></div><footer className="dashboard-table-actions"><span>共 {total} 条，第 {page}/{totalPages} 页</span><div><button disabled={page <= 1} onClick={() => setSearchParams(overviewSearch(searchParams, current, page - 1, pageSize))} type="button">上一页</button><button disabled={page >= totalPages} onClick={() => setSearchParams(overviewSearch(searchParams, current, page + 1, pageSize))} type="button">下一页</button></div></footer></article>
    </>}
  </section>;
}
