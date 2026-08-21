import { useMemo } from 'react';
import { DashboardPagination } from '../../components/dashboard-pagination';
import type { ConversationExportCandidate, ConversationExportCandidatesPage, ConversationExportType } from './conversation-global-api';
import { exportTypeLabel } from './conversation-export-stepper';

export function ConversationExportCandidates({
  type,
  keyword,
  onKeywordChange,
  onQuery,
  page,
  onPageChange,
  data,
  loading,
  selectedIds,
  onSelectedIdsChange,
  onBack,
  onNext,
  onOpenTasks,
}: {
  type: ConversationExportType;
  keyword: string;
  onKeywordChange: (value: string) => void;
  onQuery: () => void;
  page: number;
  onPageChange: (page: number) => void;
  data: ConversationExportCandidatesPage | undefined;
  loading: boolean;
  selectedIds: ReadonlySet<number>;
  onSelectedIdsChange: (ids: ReadonlySet<number>) => void;
  onBack: () => void;
  onNext: () => void;
  onOpenTasks: () => void;
}) {
  const selectableItems = useMemo(() => (data?.items ?? []).filter((item) => item.selectable), [data?.items]);
  const allCurrentSelected = selectableItems.length > 0 && selectableItems.every((item) => selectedIds.has(item.id));
  function toggle(id: number) {
    const next = new Set(selectedIds);
    if (next.has(id)) next.delete(id); else if (next.size < 100) next.add(id);
    onSelectedIdsChange(next);
  }
  function togglePage() {
    const next = new Set(selectedIds);
    if (allCurrentSelected) selectableItems.forEach((item) => next.delete(item.id));
    else selectableItems.forEach((item) => { if (next.size < 100) next.add(item.id); });
    onSelectedIdsChange(next);
  }
  return <section className="conversation-export-candidates">
    <div className="conversation-export-toolbar">
      <div className="conversation-export-toolbar-title"><button type="button" className="conversation-export-back" onClick={onBack}>←</button><div><strong>按{exportTypeLabel(type)}导出</strong><span>仅展示当前企业和当前账号可访问的真实归档数据</span></div></div>
      <div className="conversation-export-toolbar-actions"><button className="conversation-export-ghost" type="button" onClick={onOpenTasks}>导出任务</button></div>
    </div>
    <form className="conversation-export-query" onSubmit={(event) => { event.preventDefault(); onQuery(); }}>
      <label htmlFor="conversation-export-keyword">搜索{exportTypeLabel(type)}</label>
      <input id="conversation-export-keyword" value={keyword} onChange={(event) => onKeywordChange(event.target.value)} placeholder={`输入${exportTypeLabel(type)}名称或标识`} />
      <button className="conversation-export-secondary" type="submit">查询</button>
      <span className="conversation-export-query-hint">查询后才会请求服务端</span>
    </form>
    {data?.limitations.map((limitation) => <div className="conversation-export-notice" key={limitation.key}>数据提示：{limitation.reason}</div>)}
    <div className="conversation-export-table-card">
      <div className="conversation-export-table-heading"><div><strong>选择{exportTypeLabel(type)}</strong><span>已选择 {selectedIds.size} / 100</span></div><span>共 {data?.total ?? 0} 个</span></div>
      <div className="dashboard-table-scroll"><table className="conversation-export-table"><thead><tr><th><input aria-label="选择当前页" type="checkbox" checked={allCurrentSelected} onChange={togglePage} /></th><th>名称</th><th>归档会话</th><th>消息数</th><th>最近消息</th><th>状态</th></tr></thead><tbody>
        {loading && <tr><td colSpan={6} className="conversation-export-table-empty">正在加载真实数据…</td></tr>}
        {!loading && data?.items.length === 0 && <tr><td colSpan={6} className="conversation-export-table-empty">当前条件没有可导出的数据</td></tr>}
        {!loading && data?.items.map((item: ConversationExportCandidate) => <tr className={!item.selectable ? 'is-disabled' : ''} key={item.id} onClick={() => item.selectable && toggle(item.id)}>
          <td onClick={(event) => event.stopPropagation()}><input aria-label={`选择${item.name}`} type="checkbox" checked={selectedIds.has(item.id)} disabled={!item.selectable} onChange={() => toggle(item.id)} /></td>
          <td><div className="conversation-export-object"><span className="conversation-export-avatar">{item.name.slice(0, 1) || '—'}</span><span><strong>{item.name || '未命名对象'}</strong><small>{item.externalId || `ID ${item.id}`}</small></span></div></td>
          <td>{item.conversationCount}</td><td>{item.messageCount}</td><td>{item.lastMessageAt || '暂无'}</td><td>{item.selectable ? <span className="conversation-export-status is-ready">可导出</span> : <span className="conversation-export-status">{item.limitation || '暂不可导出'}</span>}</td>
        </tr>)}
      </tbody></table></div>
      <DashboardPagination ariaLabel="导出对象分页" page={page} pageSize={20} total={data?.total ?? 0} onPageChange={onPageChange} />
    </div>
    <div className="conversation-export-bottom-actions"><button className="conversation-export-secondary" type="button" onClick={onBack}>上一步</button><button className="conversation-export-primary" type="button" disabled={selectedIds.size === 0} onClick={onNext}>下一步</button></div>
  </section>;
}
