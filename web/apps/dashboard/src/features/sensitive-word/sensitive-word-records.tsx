import { DashboardPagination } from '../../components/dashboard-pagination';
import { PageState } from '../../components/page-state/page-state';
import { RiskWarningDrawer, RiskWarningQueryBar } from '../risk-warning/risk-warning-shell';
import type { SensitiveWordFilterOptions, SensitiveWordGroup, SensitiveWordMatch, SensitiveWordMatchDetail, SensitiveWordMatchFilters, SensitiveWordMatchPage, SensitiveWordScannerStatus } from './sensitive-word-api';

function sourceLabel(source: number): string { return source === 2 ? '员工触发' : source === 1 ? '客户触发' : '未知来源'; }
export function SensitiveWordRecords({
  draft, setDraft, groups, options, optionsLoading, data, loading, error, detailID, detail, detailLoading, detailError, scanner, fetching,
  onQuery, onReset, onRefresh, onPageChange, onOpen, onCloseDetail, onRetryDetail,
}: {
  draft: SensitiveWordMatchFilters;
  setDraft: (next: SensitiveWordMatchFilters) => void;
  groups: readonly SensitiveWordGroup[];
  options: SensitiveWordFilterOptions | undefined;
  optionsLoading: boolean;
  data: SensitiveWordMatchPage | undefined;
  loading: boolean;
  error: unknown;
  detailID: number | null;
  detail: readonly SensitiveWordMatchDetail[] | undefined;
  detailLoading: boolean;
  detailError: unknown;
  scanner: SensitiveWordScannerStatus | undefined;
  fetching: boolean;
  onQuery: () => void;
  onReset: () => void;
  onRefresh: () => void;
  onPageChange: (page: number) => void;
  onOpen: (id: number) => void;
  onCloseDetail: () => void;
  onRetryDetail: () => void;
}) {
  const totalPages = Math.max(1, Math.ceil((data?.total ?? 0) / 20));
  const set = (key: keyof SensitiveWordMatchFilters, value: unknown) => setDraft({ ...draft, [key]: value, page: 1 });
  const selectedEmployee = draft.employeeIds[0] ?? 0;
  const selectedGroup = draft.groupId;
  return <section aria-label="敏感词命中记录" className="risk-warning-workspace sensitive-word-records-workspace">
    <RiskWarningQueryBar fetching={fetching} onQuery={onQuery} onReset={onReset} onRefresh={onRefresh}>
      <label>触发员工<select aria-label="触发员工" value={selectedEmployee} onChange={(event) => set('employeeIds', Number(event.target.value) > 0 ? [Number(event.target.value)] : [])}><option value={0}>全部员工</option>{options?.employees.map((item) => <option key={item.id} value={item.id}>{item.name}{item.count > 0 ? ` · ${item.count}` : ''}</option>)}</select></label>
      <label>客户群<select aria-label="触发客户群" value={draft.workRoomId} onChange={(event) => set('workRoomId', Number(event.target.value) || 0)}><option value={0}>全部客户群</option>{options?.rooms.map((item) => <option key={item.id} value={item.id}>{item.name}{item.count > 0 ? ` · ${item.count}` : ''}</option>)}</select></label>
      <label>敏感词组<select aria-label="命中词组" value={selectedGroup} onChange={(event) => set('groupId', Number(event.target.value) || 0)}><option value={0}>全部词组</option>{groups.map((group) => <option key={group.id} value={group.id}>{group.name}</option>)}</select></label>
      <label>触发来源<select aria-label="触发来源" value={draft.source ?? 0} onChange={(event) => set('source', Number(event.target.value) || undefined)}><option value={0}>全部来源</option><option value={2}>员工触发</option><option value={1}>客户触发</option></select></label>
      <label>开始时间<input aria-label="开始时间" type="datetime-local" value={draft.triggerStart.replace(' ', 'T').slice(0, 16)} onChange={(event) => set('triggerStart', event.target.value)} /></label>
      <label>结束时间<input aria-label="结束时间" type="datetime-local" value={draft.triggerEnd.replace(' ', 'T').slice(0, 16)} onChange={(event) => set('triggerEnd', event.target.value)} /></label>
    </RiskWarningQueryBar>
    {scanner && scanner.state !== 'ready' ? <div className="risk-warning-status-strip" role="status">{scanner.state === 'disabled' ? '敏感词扫描任务未启用，当前只展示已有真实命中记录。' : scanner.state === 'failed' ? `敏感词扫描最近一次失败${scanner.lastError ? `：${scanner.lastError}` : '，请检查运行日志。'}` : '敏感词扫描尚未成功运行，暂无新增记录可供判断。'}</div> : null}
    {optionsLoading ? <p className="risk-warning-muted">正在加载真实筛选选项…</p> : null}
    <div className="risk-warning-results">
      <div className="risk-warning-results-header"><div><h2>命中记录</h2><p>只展示当前企业真实敏感词命中数据；没有数据时不会补造统计。</p></div><span className="risk-warning-muted">固定每页 20 条</span></div>
      {loading && !data ? <PageState state="loading" /> : error && !data ? <PageState state="error" onRetry={onRefresh} /> : !data || data.items.length === 0 ? <PageState state="empty" title="暂无敏感词命中记录" description="当前筛选条件下没有系统记录。" /> : <>
        <div className="risk-warning-summary"><div className="risk-warning-summary-item"><span>命中总数</span><strong>{data.total}</strong></div><div className="risk-warning-summary-item"><span>当前页</span><strong>{data.items.length}</strong></div><div className="risk-warning-summary-item"><span>当前页码</span><strong>{data.page}/{totalPages}</strong></div><div className="risk-warning-summary-item"><span>筛选口径</span><strong>真实记录</strong></div></div>
        <div className="risk-warning-table-wrap"><table className="risk-warning-table sensitive-word-record-table"><thead><tr><th>敏感词</th><th>来源</th><th>触发人</th><th>会话场景</th><th>消息摘要</th><th>触发时间</th></tr></thead><tbody>{data.items.map((item: SensitiveWordMatch) => <tr key={item.id} tabIndex={0} onClick={(event) => { if ((event.target as HTMLElement).closest('button,a,select,input')) return; onOpen(item.id); }} onKeyDown={(event) => { if (event.key === 'Enter') onOpen(item.id); }}><td><strong className="risk-warning-trigger-text">{item.sensitiveWordName || '敏感词暂缺'}</strong></td><td>{sourceLabel(item.source)}</td><td>{item.triggerName || '触发人暂缺'}</td><td>{item.triggerScenario || (item.workRoomID > 0 ? `客户群 #${item.workRoomID}` : '会话场景暂缺')}</td><td><span className="risk-warning-inline-preview" title={item.contentPreview}>{item.contentPreview || '消息内容暂缺'}</span></td><td>{item.triggerTime || '--'}</td></tr>)}</tbody></table></div>
        <div className="risk-warning-pagination"><DashboardPagination page={data.page} pageSize={20} total={data.total} onPageChange={onPageChange} ariaLabel="敏感词命中记录分页" /></div>
      </>}
    </div>
    <RiskWarningDrawer open={detailID !== null} title="敏感词命中详情" {...(detailID ? { description: `记录 #${detailID}` } : {})} onClose={onCloseDetail}>
      {detailLoading ? <PageState state="loading" title="正在加载命中详情" /> : detailError ? <PageState state="error" title="命中详情加载失败" description="会话归档详情暂时不可用，请稍后重试。" onRetry={onRetryDetail} /> : detail && detail.length > 0 ? <div className="risk-warning-detail-list">{detail.map((row, index) => <article key={`${detailID}-${index}`}><dl className="risk-warning-detail-grid"><div><dt>发送人</dt><dd>{row.sender || '发送人暂缺'}</dd></div><div><dt>消息类型</dt><dd>{row.messageType}</dd></div><div><dt>{row.isTrigger ? '触发消息' : '关联消息'}</dt><dd className="risk-warning-detail-message">{row.content || '消息内容暂缺'}</dd></div><div><dt>发送时间</dt><dd>{row.sendTime || '--'}</dd></div></dl></article>)}</div> : <PageState state="empty" title="暂无详情数据" description="系统没有返回该命中的会话详情。" />}
    </RiskWarningDrawer>
  </section>;
}
