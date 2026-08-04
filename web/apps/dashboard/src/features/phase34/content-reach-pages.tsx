import { useQuery } from '@tanstack/react-query';
import { useMemo, useState } from 'react';

import { useDashboardAccess } from '../../app/access-context';
import { pageStateForError, PageState } from '../../components/page-state/page-state';
import type { BusinessWorkbenchApi } from '../business-workbench/business-workbench-page';

type ReachRecord = Record<string, unknown>;
type SendMode = 'contact' | 'room';

function isRecord(value: unknown): value is ReachRecord {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function rowsFrom(payload: unknown): ReachRecord[] {
  if (Array.isArray(payload)) return payload.filter(isRecord);
  if (!isRecord(payload)) return [];
  const data = isRecord(payload.data) ? payload.data : payload;
  const candidate = [data.list, data.items, data.rows, isRecord(data.data) ? data.data.list : undefined].find(Array.isArray);
  return Array.isArray(candidate) ? candidate.filter(isRecord) : [];
}

function primitive(value: unknown): string {
  if (value === undefined || value === null || value === '') return '--';
  if (typeof value === 'string') return value;
  if (typeof value === 'number' || typeof value === 'boolean' || typeof value === 'bigint') return value.toString();
  return '--';
}

function contentText(value: unknown): string {
  if (Array.isArray(value)) {
    const text = value.map((item) => isRecord(item) ? primitive(item.content ?? item.title ?? item.name) : primitive(item)).filter((item) => item !== '--').join('；');
    return text || '--';
  }
  return isRecord(value) ? primitive(value.content ?? value.title ?? value.name) : primitive(value);
}

function statusText(value: unknown): string {
  if (typeof value === 'string' && value.trim() !== '') return value;
  if (value === 0) return '待执行';
  if (value === 1) return '执行中';
  if (value === 2) return '已完成';
  if (value === 3) return '部分失败';
  return '--';
}

function executionData(row: ReachRecord): string {
  return `${primitive(row.sendTotal)} / ${primitive(row.receivedTotal)}`;
}

function recordKey(row: ReachRecord, index: number): string {
  const value = row.id ?? row.batchId;
  return typeof value === 'string' || typeof value === 'number' ? value.toString() : index.toString();
}

function ReachTabs({ active, onChange }: { active: SendMode; onChange: (mode: SendMode) => void }) {
  return (
    <div className="phase34-tabs" role="tablist" aria-label="群发类型">
      <button type="button" role="tab" aria-selected={active === 'contact'} className={active === 'contact' ? 'phase34-tab-active' : ''} onClick={() => onChange('contact')}>客户群发</button>
      <button type="button" role="tab" aria-selected={active === 'room'} className={active === 'room' ? 'phase34-tab-active' : ''} onClick={() => onChange('room')}>群聊群发</button>
    </div>
  );
}

function SendDetail({ row, onClose }: { row: ReachRecord; onClose: () => void }) {
  const fields = [
    ['任务名称', primitive(row.batchTitle)],
    ['发送内容', contentText(row.content)],
    ['执行结果', statusText(row.sendStatus)],
    ['执行数据', executionData(row)],
    ['创建时间', primitive(row.createdAt)],
    ['执行时间', primitive(row.sendTime ?? row.definiteTime)],
  ];
  return (
    <aside className="phase34-detail" aria-label="群发任务详情">
      <div className="phase34-detail-backdrop" onClick={onClose} />
      <div className="phase34-detail-panel">
        <div className="dashboard-card-heading"><div><p className="phase34-eyebrow">营销工具</p><h2>群发任务详情</h2></div><button type="button" onClick={onClose}>关闭</button></div>
        <dl>{fields.map(([label, value]) => <div key={label}><dt>{label}</dt><dd>{value}</dd></div>)}</dl>
      </div>
    </aside>
  );
}

export function PreciseGroupSendPage({ api }: { api: BusinessWorkbenchApi }) {
  const access = useDashboardAccess();
  const [mode, setMode] = useState<SendMode>('contact');
  const [draftTitle, setDraftTitle] = useState('');
  const [title, setTitle] = useState('');
  const [selected, setSelected] = useState<ReachRecord | null>(null);
  const endpoint = mode === 'contact' ? '/contactMessageBatchSend/index' : '/roomMessageBatchSend/index';
  const query = useQuery({
    queryKey: ['phase34-precise-send', access.corp.id, mode, title],
    queryFn: () => api.read(endpoint, { ...(title ? { batchTitle: title } : {}), page: 1, perPage: 20 }),
  });
  const rows = useMemo(() => rowsFrom(query.data), [query.data]);
  const refresh = () => { void query.refetch(); };

  return (
    <section className="phase34-page">
      <header className="phase34-page-header">
        <div><p className="phase34-eyebrow">营销工具 · 内容触达</p><h1>精准群发</h1><p>分别查看客户群发与群聊群发任务，追踪内容、执行结果和触达数据。</p></div>
        <div className="phase34-header-actions"><span className="phase34-provider-badge">双 Provider 已连接</span><button type="button" disabled>新建群发</button><button type="button" disabled={query.isFetching} onClick={refresh}>刷新</button></div>
      </header>
      <ReachTabs active={mode} onChange={(nextMode) => { setMode(nextMode); setDraftTitle(''); setTitle(''); setSelected(null); }} />
      <div className="dashboard-filter-bar phase34-filter-bar">
        <label>任务名称<input aria-label="任务名称" placeholder="请输入任务名称" value={draftTitle} onChange={(event) => setDraftTitle(event.target.value)} /></label>
        <label>创建开始日期<input aria-label="创建开始日期" type="date" /></label>
        <label>创建结束日期<input aria-label="创建结束日期" type="date" /></label>
        <div className="dashboard-table-actions"><button type="button" onClick={() => { setTitle(draftTitle.trim()); setSelected(null); }}>查询</button><button type="button" className="phase34-secondary-button" onClick={() => { setDraftTitle(''); setTitle(''); setSelected(null); }}>重置</button></div>
      </div>
      <div className="dashboard-data-card phase34-results-card">
        <div className="dashboard-card-heading"><div><h2>{mode === 'contact' ? '客户群发' : '群聊群发'}任务</h2><p>当前企业：{access.corp.name}，列表仅展示 Provider 返回的真实任务。</p></div><span>{rows.length} 条</span></div>
        {query.isPending ? <PageState state="loading" /> : query.isError ? <PageState state={pageStateForError(query.error)} onRetry={refresh} /> : rows.length === 0 ? <PageState state="empty" title="暂无群发任务" description="当前筛选条件下没有可展示的任务。" /> : (
          <div className="dashboard-table-scroll"><table className="phase34-table phase34-reach-table"><thead><tr><th>创建时间</th><th>执行时间</th><th>发送内容</th><th>执行结果</th><th>执行数据</th><th>操作</th></tr></thead><tbody>{rows.map((row, index) => <tr key={recordKey(row, index)}><td>{primitive(row.createdAt)}</td><td>{primitive(row.sendTime ?? row.definiteTime)}</td><td>{contentText(row.content)}</td><td>{statusText(row.sendStatus)}</td><td>{executionData(row)}</td><td><button type="button" className="phase34-link-button" onClick={() => setSelected(row)}>详情</button></td></tr>)}</tbody></table></div>
        )}
      </div>
      {selected !== null && <SendDetail row={selected} onClose={() => setSelected(null)} />}
    </section>
  );
}

export function FriendsCirclePage({ api }: { api: BusinessWorkbenchApi }) {
  const access = useDashboardAccess();
  const [tab, setTab] = useState<'task' | 'material'>('task');
  const [draftFilter, setDraftFilter] = useState('');
  const [filter, setFilter] = useState('');
  const [composerOpen, setComposerOpen] = useState(false);
  const [draftName, setDraftName] = useState('');
  const [draftContent, setDraftContent] = useState('');
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState('');
  const endpoint = tab === 'task' ? '/friendsCircle/taskIndex' : '/friendsCircle/materialIndex';
  const query = useQuery({
    queryKey: ['phase34-friends-circle', access.corp.id, tab, filter],
    queryFn: () => api.read(endpoint, { ...(filter ? (tab === 'task' ? { taskName: filter } : { keyword: filter }) : {}), page: 1, perPage: 20 }),
  });
  const rows = useMemo(() => rowsFrom(query.data), [query.data]);
  const refresh = () => { void query.refetch(); };
  const changeTab = (next: 'task' | 'material') => {
    setTab(next);
    setDraftFilter('');
    setFilter('');
    setComposerOpen(false);
  };
  const saveDraft = async () => {
    if (!draftName.trim() || !draftContent.trim()) return;
    setSaving(true);
    setSaveError('');
    try {
      if (tab === 'task') {
        await api.write('/friendsCircle/taskStore', { taskName: draftName.trim(), content: draftContent.trim(), sendWay: 'manual' });
      } else {
        await api.write('/friendsCircle/materialStore', { name: draftName.trim(), type: 'text', content: { text: draftContent.trim() } });
      }
      setComposerOpen(false);
      setDraftName('');
      setDraftContent('');
      await query.refetch();
    } catch {
      setSaveError('草稿保存失败，请稍后重试。');
    } finally {
      setSaving(false);
    }
  };
  return (
    <section className="phase34-page">
      <header className="phase34-page-header"><div><p className="phase34-eyebrow">营销工具 · 内容触达</p><h1>朋友圈</h1><p>统一管理朋友圈任务和素材草稿；草稿持久化与正式发布使用相互隔离的 Provider。</p></div><div className="phase34-header-actions"><span className="phase34-provider-badge">持久化 Provider 已连接</span><button type="button" onClick={() => setComposerOpen(true)}>添加朋友圈</button><button type="button" disabled>导出</button><button type="button" disabled={query.isFetching} onClick={refresh}>刷新</button></div></header>
      <p className="phase34-provider-notice"><strong>发布 Provider 未配置</strong><span>，当前仅保存可审计草稿，不会向企业微信发起发布。</span></p>
      <div className="phase34-tabs" role="tablist" aria-label="朋友圈内容类型"><button type="button" role="tab" aria-selected={tab === 'task'} className={tab === 'task' ? 'phase34-tab-active' : ''} onClick={() => changeTab('task')}>朋友圈</button><button type="button" role="tab" aria-selected={tab === 'material'} className={tab === 'material' ? 'phase34-tab-active' : ''} onClick={() => changeTab('material')}>朋友圈素材</button></div>
      <div className="dashboard-filter-bar phase34-filter-bar phase34-friends-filter">
        <label>{tab === 'task' ? '任务名称' : '素材关键字'}<input aria-label={tab === 'task' ? '任务名称' : '素材关键字'} placeholder="请输入关键字" value={draftFilter} onChange={(event) => setDraftFilter(event.target.value)} /></label><label>选择员工<select aria-label="选择员工" disabled><option>全部员工</option></select></label><label>发送方式<select aria-label="发送方式" disabled><option>全部方式</option></select></label><label>任务状态<select aria-label="任务状态" disabled><option>全部状态</option></select></label><div className="dashboard-table-actions"><button type="button" onClick={() => setFilter(draftFilter.trim())}>查询</button><button type="button" className="phase34-secondary-button" onClick={() => { setDraftFilter(''); setFilter(''); }}>重置</button></div>
      </div>
      <div className="dashboard-data-card phase34-results-card"><div className="dashboard-card-heading"><div><h2>{tab === 'task' ? '朋友圈任务' : '朋友圈素材'}</h2><p>当前展示数据库 Provider 返回的真实草稿与素材记录。</p></div><span>{rows.length} 条</span></div>{query.isPending ? <PageState state="loading" /> : query.isError ? <PageState state={pageStateForError(query.error)} onRetry={refresh} /> : rows.length === 0 ? <PageState state="empty" title="暂无朋友圈记录" description="可通过添加朋友圈创建第一条可审计草稿。" /> : <div className="dashboard-table-scroll"><table className="phase34-table"><thead><tr><th>{tab === 'task' ? '任务名称' : '素材内容'}</th><th>发送方式</th><th>状态</th><th>完成情况</th><th>创建人 / 创建时间</th><th>操作</th></tr></thead><tbody>{rows.map((row, index) => <tr key={recordKey(row, index)}><td>{primitive(tab === 'task' ? row.taskName : row.name)}</td><td>{primitive(tab === 'task' ? row.sendWay : row.type)}</td><td>{statusText(row.status)}</td><td>{tab === 'task' ? `${primitive(row.completedTotal)} / ${primitive(row.targetTotal)}` : '--'}</td><td>{primitive(row.creatorName)} / {primitive(row.createdAt)}</td><td><span className="phase34-muted-action">仅草稿</span></td></tr>)}</tbody></table></div>}</div>
      {composerOpen && <aside className="phase34-detail" aria-label="朋友圈草稿"><div className="phase34-detail-backdrop" onClick={() => setComposerOpen(false)} /><div className="phase34-detail-panel"><div className="dashboard-card-heading"><div><p className="phase34-eyebrow">持久化 Provider</p><h2>{tab === 'task' ? '添加朋友圈草稿' : '添加朋友圈素材'}</h2></div><button type="button" onClick={() => setComposerOpen(false)}>关闭</button></div><div className="phase34-compose-form"><label>草稿名称<input aria-label="草稿名称" value={draftName} onChange={(event) => setDraftName(event.target.value)} /></label><label>草稿内容<textarea aria-label="草稿内容" rows={6} value={draftContent} onChange={(event) => setDraftContent(event.target.value)} /></label>{saveError && <p role="alert">{saveError}</p>}<button type="button" disabled={saving || !draftName.trim() || !draftContent.trim()} onClick={() => void saveDraft()}>{saving ? '保存中…' : '保存草稿'}</button></div></div></aside>}
    </section>
  );
}
