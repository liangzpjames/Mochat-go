import { useQuery } from '@tanstack/react-query';
import { useMemo, useState } from 'react';

import { useDashboardAccess } from '../../app/access-context';
import { pageStateForError, PageState } from '../../components/page-state/page-state';
import type { BusinessWorkbenchApi } from '../business-workbench/business-workbench-page';

type RecordItem = Record<string, unknown>;

function rowsFrom(value: unknown): RecordItem[] {
  if (Array.isArray(value)) return value.filter((item): item is RecordItem => typeof item === 'object' && item !== null);
  if (!value || typeof value !== 'object') return [];
  const source = value as Record<string, unknown>;
  const body = source.data && typeof source.data === 'object' ? source.data as Record<string, unknown> : source;
  const rows = [body.list, body.items, body.rows].find(Array.isArray);
  return Array.isArray(rows) ? rows.filter((item): item is RecordItem => typeof item === 'object' && item !== null) : [];
}

function display(value: unknown): string {
  if (value === null || value === undefined || value === '') return '--';
  if (typeof value === 'object') return JSON.stringify(value);
  if (typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean' || typeof value === 'bigint') return String(value);
  return '--';
}

export function CustomerLossPage({ api }: { api: BusinessWorkbenchApi }) {
  const access = useDashboardAccess();
  const [draftEmployeeId, setDraftEmployeeId] = useState('');
  const [employeeId, setEmployeeId] = useState('');
  const [selected, setSelected] = useState<RecordItem | null>(null);
  const enabled = access.corp.authorized;
  const query = useQuery({
    queryKey: ['phase33-customer-loss', access.corp.id, employeeId],
    queryFn: () => api.read('/workContact/lossContact', { ...(employeeId ? { employeeId } : {}), page: 1, perPage: 20 }),
    enabled,
  });
  const rows = useMemo(() => rowsFrom(query.data), [query.data]);
  const columns = useMemo(() => Object.keys(rows[0] ?? {}).slice(0, 7), [rows]);

  return <section className="phase33-risk-warning-page">
    <header className="phase33-risk-warning-header dashboard-page-header dashboard-data-card"><div><p className="phase33-risk-warning-eyebrow">风险预警</p><h1>客户流失</h1><p>查看近期客户流失情况，及时跟进高价值客户。</p></div><button type="button" disabled={!enabled || query.isFetching} onClick={() => void query.refetch()}>刷新</button></header>
    <div className="dashboard-filter-bar phase33-risk-warning-filters" aria-label="客户流失筛选"><label>员工 ID<input aria-label="员工 ID" inputMode="numeric" value={draftEmployeeId} onChange={(event) => setDraftEmployeeId(event.target.value)} /></label><div className="dashboard-table-actions"><button type="button" disabled={!enabled} onClick={() => { setSelected(null); setEmployeeId(draftEmployeeId.trim()); }}>查询</button><button type="button" disabled={!enabled} onClick={() => { setDraftEmployeeId(''); setEmployeeId(''); setSelected(null); }}>重置</button></div></div>
    {!enabled ? <div className="dashboard-data-card phase33-risk-warning-state"><PageState state="forbidden" title="无权访问当前企业数据" description="请切换到已授权企业，或联系管理员开通权限。" /></div> : <div className="dashboard-data-card phase33-risk-warning-results"><div className="dashboard-card-heading"><div><h2>流失记录</h2><p>关注客户状态变化，及时安排后续跟进。</p></div></div>{query.isPending ? <PageState state="loading" /> : query.isError ? <PageState state={pageStateForError(query.error)} onRetry={() => void query.refetch()} /> : rows.length === 0 ? <PageState state="empty" /> : <div className="dashboard-table-scroll"><table><thead><tr>{columns.map((column) => <th key={column}>{column}</th>)}<th>操作</th></tr></thead><tbody>{rows.map((row, index) => <tr key={display(row.id ?? row.contactId ?? index)}>{columns.map((column) => <td key={column}>{display(row[column])}</td>)}<td><button type="button" onClick={() => setSelected(row)}>详情</button></td></tr>)}</tbody></table></div>}</div>}
    {selected && <aside className="phase33-risk-warning-detail dashboard-data-card" aria-label="客户流失详情"><div className="dashboard-card-heading"><div><h2>客户流失详情</h2><p>查看客户信息和流失记录。</p></div><button type="button" onClick={() => setSelected(null)}>关闭</button></div><dl>{Object.entries(selected).map(([key, value]) => <div key={key}><dt>{key}</dt><dd>{display(value)}</dd></div>)}</dl></aside>}
  </section>;
}
