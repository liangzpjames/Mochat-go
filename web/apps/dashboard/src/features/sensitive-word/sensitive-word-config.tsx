import { ConfirmAction } from '../../components/confirm-action';
import { DashboardPagination } from '../../components/dashboard-pagination';
import { PageState } from '../../components/page-state/page-state';
import { RiskWarningQueryBar } from '../risk-warning/risk-warning-shell';
import type { SensitiveWordGroup, SensitiveWordItem, SensitiveWordPage } from './sensitive-word-api';

type Props = {
  groups: readonly SensitiveWordGroup[];
  selectedGroupID: number;
  onSelectGroup: (id: number) => void;
  words: SensitiveWordPage | undefined;
  keywords: string;
  status: number;
  onKeywordsChange: (value: string) => void;
  onStatusChange: (value: number) => void;
  onQuery: (input: { keywords: string; status: number }) => void;
  onReset: () => void;
  onRefresh: () => void;
  fetching: boolean;
  loading: boolean;
  error: unknown;
  page: number;
  onPageChange: (page: number) => void;
  canAdd: boolean;
  canEdit: boolean;
  canDelete: boolean;
  onCreateGroup: () => void;
  onCreateWord: () => void;
  onToggle: (item: SensitiveWordItem) => void;
  onMove: (item: SensitiveWordItem) => void;
  onDelete: (item: SensitiveWordItem) => void;
  moveTargets: Record<number, number>;
  setMoveTarget: (id: number, value: number) => void;
};

export function SensitiveWordConfig({ groups, selectedGroupID, onSelectGroup, words, keywords, status, onKeywordsChange, onStatusChange, onQuery, onReset, onRefresh, fetching, loading, error, page, onPageChange, canAdd, canEdit, canDelete, onCreateGroup, onCreateWord, onToggle, onMove, onDelete, moveTargets, setMoveTarget }: Props) {
  return <section aria-label="敏感词配置" className="sensitive-word-config-workspace">
    <aside className="sensitive-word-group-column">
      <div className="sensitive-word-column-header"><h2>词组</h2><p>按词组切换右侧真实词条</p></div>
      <div className="sensitive-word-group-list">
        <button type="button" aria-pressed={selectedGroupID === 0} onClick={() => onSelectGroup(0)}><span>全部词条</span><small>{groups.reduce((total, group) => total + group.wordCount, 0)}</small></button>
        {groups.map((group) => <button key={group.id} type="button" aria-pressed={selectedGroupID === group.id} onClick={() => onSelectGroup(group.id)}><span>{group.name}</span><small>{group.enabledCount}/{group.wordCount}</small></button>)}
      </div>
    </aside>
    <div className="sensitive-word-word-column">
      <RiskWarningQueryBar fetching={fetching} onQuery={() => onQuery({ keywords, status })} onReset={onReset} onRefresh={onRefresh}>
        <label>搜索词条<input aria-label="搜索敏感词" value={keywords} onChange={(event) => onKeywordsChange(event.target.value)} placeholder="输入词条后点击查询" /></label>
        <label>词条状态<select aria-label="词条状态" value={status} onChange={(event) => onStatusChange(Number(event.target.value))}><option value={0}>全部状态</option><option value={1}>启用</option><option value={2}>停用</option></select></label>
      </RiskWarningQueryBar>
      <div className="sensitive-word-column-toolbar"><div><h2>{selectedGroupID > 0 ? groups.find((group) => group.id === selectedGroupID)?.name ?? '指定词组' : '全部敏感词'}</h2><p>{words ? `当前条件 ${words.total} 条 · 固定每页 20 条` : '只维护系统已接入的真实词条'}</p></div><div className="risk-warning-page-actions">{canAdd ? <><button type="button" className="risk-warning-secondary-button" onClick={onCreateGroup}>新增词组</button><button type="button" className="risk-warning-primary-button" onClick={onCreateWord}>新增敏感词</button></> : null}</div></div>
      {loading && !words ? <PageState state="loading" /> : error && !words ? <PageState state="error" onRetry={onRefresh} /> : !words || words.items.length === 0 ? <PageState state="empty" title="暂无敏感词条" description="当前词组或筛选条件下没有真实词条。" /> : <>
        <div className="risk-warning-table-wrap"><table className="risk-warning-table sensitive-word-config-table"><thead><tr><th>词条</th><th>词组</th><th>状态</th><th>员工命中</th><th>客户命中</th><th>创建时间</th><th>操作</th></tr></thead><tbody>{words.items.map((item) => <tr key={item.id}><td><strong>{item.name}</strong></td><td>{item.groupName}</td><td><span className={`risk-warning-pill ${item.status === 1 ? 'risk-warning-pill-low' : 'risk-warning-pill-neutral'}`}>{item.status === 1 ? '启用' : '停用'}</span></td><td>{item.employeeHitCount}</td><td>{item.customerHitCount}</td><td>{item.createdAt || '--'}</td><td className="risk-warning-row-control"><div className="sensitive-word-row-actions">{canEdit ? <ConfirmAction title={`确认${item.status === 1 ? '停用' : '启用'}敏感词“${item.name}”？`} onConfirm={() => onToggle(item)}><button type="button" aria-label={`${item.status === 1 ? '停用' : '启用'} ${item.name}`}>{item.status === 1 ? '停用' : '启用'}</button></ConfirmAction> : null}<select aria-label={`移动 ${item.name}`} value={moveTargets[item.id] ?? item.groupId} onChange={(event) => setMoveTarget(item.id, Number(event.target.value))}>{groups.map((group) => <option key={group.id} value={group.id}>{group.name}</option>)}</select>{canEdit ? <button type="button" onClick={() => onMove(item)}>移动</button> : null}{canDelete ? <ConfirmAction title={`确认删除敏感词“${item.name}”？`} description="删除后无法恢复。" onConfirm={() => onDelete(item)}><button type="button" aria-label={`删除 ${item.name}`}>删除</button></ConfirmAction> : null}</div></td></tr>)}</tbody></table></div>
        <div className="risk-warning-pagination"><DashboardPagination page={page} pageSize={20} total={words.total} onPageChange={onPageChange} ariaLabel="敏感词配置分页" /></div>
      </>}
    </div>
  </section>;
}
