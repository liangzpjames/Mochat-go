import { ApiError } from '@mochat/api-client';
import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';
import type { FormEvent } from 'react';
import { useSearchParams } from 'react-router';

import { useDashboardAccess } from '../../app/access-context';
import { PageState } from '../../components/page-state/page-state';
import { updateSearch } from '../../shared/query-state';
import type { ConversationGlobalApi } from './conversation-global-api';

function isArchiveUnauthorized(error: unknown): boolean { return error instanceof ApiError && error.status === 403 && error.code === 40301; }
function csvCell(value: unknown): string { return `"${String(value ?? '').replaceAll('"', '""')}"`; }

export function ConversationExportPage({ api }: { api: ConversationGlobalApi }) {
  const access = useDashboardAccess();
  const [params, setParams] = useSearchParams();
  const [keyword, setKeyword] = useState(params.get('keyword') ?? '');
  const query = useQuery({
    queryKey: ['corp', access.corp.id, 'conversation-export', params.get('keyword') ?? ''],
    queryFn: () => api.search({ keyword: params.get('keyword') ?? '', conversationType: '', employeeIds: [], startAt: '', endAt: '', page: 1, pageSize: 100 }),
  });
  const archiveUnauthorized = isArchiveUnauthorized(query.error);
  function apply(event: FormEvent<HTMLFormElement>) { event.preventDefault(); setParams(updateSearch(params, { keyword })); }
  function exportCurrent() {
    if (!query.data) return;
    const rows = [['会话 ID', '员工', '对象', '类型', '最后消息', '时间'], ...query.data.list.map((item) => [item.id, item.employeeName, item.targetName, item.targetType, item.lastMessage, item.sentAt])];
    const blob = new Blob([`\uFEFF${rows.map((row) => row.map(csvCell).join(',')).join('\n')}`], { type: 'text/csv;charset=utf-8' });
    const url = URL.createObjectURL(blob); const link = document.createElement('a'); link.href = url; link.download = `conversation-export-${new Date().toISOString().slice(0, 10)}.csv`; link.click(); URL.revokeObjectURL(url);
  }
  return <section className="conversation-export-page">
    <header className="conversation-global-header dashboard-data-card"><div><p className="conversation-global-eyebrow">会话存档</p><h1>会话导出</h1><p>按当前查询条件导出会话摘要，下载内容受当前企业和会话存档权限控制。</p></div><button type="button" disabled={query.isFetching} onClick={() => void query.refetch()}>刷新</button></header>
    <form className="conversation-export-filter dashboard-data-card" onSubmit={apply}><label htmlFor="export-keyword">关键词</label><input id="export-keyword" value={keyword} onChange={(event) => setKeyword(event.target.value)} placeholder="搜索对象或消息内容" /><button type="submit">查询</button><button type="button" onClick={exportCurrent} disabled={!query.data || query.data.list.length === 0}>导出当前结果</button></form>
    {query.isPending && <PageState state="loading" />}
    {archiveUnauthorized && <PageState state="forbidden" title="当前企业未开通会话内容存档" description="请先完成会话存档授权，再进行会话导出。" />}
    {query.error && !archiveUnauthorized && <PageState state="error" onRetry={() => void query.refetch()} />}
    {query.data && <section className="conversation-export-results dashboard-data-card"><header><strong>当前结果</strong><span>共 {query.data.total} 条，本次展示 {query.data.list.length} 条</span></header>{query.data.list.length === 0 ? <PageState state="empty" /> : <div className="dashboard-table-scroll"><table><thead><tr><th>对象</th><th>员工</th><th>类型</th><th>最后消息</th><th>时间</th></tr></thead><tbody>{query.data.list.map((item) => <tr key={item.id}><td>{item.targetName}</td><td>{item.employeeName}</td><td>{item.targetType}</td><td>{item.lastMessage || '暂无消息'}</td><td>{item.sentAt}</td></tr>)}</tbody></table></div>}</section>}
  </section>;
}
