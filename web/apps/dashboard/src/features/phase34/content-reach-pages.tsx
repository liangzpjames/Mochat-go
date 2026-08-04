import { useQuery } from '@tanstack/react-query';
import { useEffect, useMemo, useRef, useState } from 'react';

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
  if (typeof value === 'string') {
    try {
      return contentText(JSON.parse(value));
    } catch {
      return primitive(value);
    }
  }
  if (Array.isArray(value)) {
    const text = value.map((item) => isRecord(item) ? primitive(item.content ?? item.title ?? item.name) : primitive(item)).filter((item) => item !== '--').join('；');
    return text || '--';
  }
  return isRecord(value) ? primitive(value.text ?? value.content ?? value.title ?? value.name) : primitive(value);
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

function positiveReachIDs(value: string): number[] {
  return value.split(',').map((item) => Number(item.trim())).filter((item) => Number.isInteger(item) && item > 0);
}

function SendCreateDrawer({
  mode,
  title,
  employeeIDs,
  content,
  sendWay,
  definiteTime,
  saving,
  error,
  onTitleChange,
  onEmployeeIDsChange,
  onContentChange,
  onSendWayChange,
  onDefiniteTimeChange,
  onClose,
  onSubmit,
}: {
  mode: SendMode;
  title: string;
  employeeIDs: string;
  content: string;
  sendWay: string;
  definiteTime: string;
  saving: boolean;
  error: string;
  onTitleChange: (value: string) => void;
  onEmployeeIDsChange: (value: string) => void;
  onContentChange: (value: string) => void;
  onSendWayChange: (value: string) => void;
  onDefiniteTimeChange: (value: string) => void;
  onClose: () => void;
  onSubmit: () => void;
}) {
  return (
    <aside className="phase34-detail" aria-label="新建群发任务">
      <div className="phase34-detail-backdrop" aria-hidden="true" onClick={onClose} />
      <div className="phase34-detail-panel">
        <div className="dashboard-card-heading"><div><p className="phase34-eyebrow">营销工具 · 内容触达</p><h2>新建群发任务</h2><p>{mode === 'contact' ? '客户群发' : '群聊群发'}会走对应的真实 Provider。</p></div><button type="button" aria-label="关闭新建群发" onClick={onClose}>关闭</button></div>
        <form onSubmit={(event) => { event.preventDefault(); onSubmit(); }}>
          <label>任务名称<input aria-label="任务名称" required value={title} onChange={(event) => onTitleChange(event.target.value)} placeholder="请输入任务名称" /></label>
          <label>{mode === 'contact' ? '发送成员ID' : '群主ID'}<input aria-label={mode === 'contact' ? '发送成员ID' : '群主ID'} required value={employeeIDs} onChange={(event) => onEmployeeIDsChange(event.target.value)} placeholder="多个 ID 用逗号分隔" /></label>
          <label>群发内容<textarea aria-label="群发内容" required value={content} onChange={(event) => onContentChange(event.target.value)} placeholder="请输入文本内容" rows={6} /></label>
          <label>发送方式<select aria-label="发送方式" value={sendWay} onChange={(event) => onSendWayChange(event.target.value)}><option value="1">立即发送</option><option value="2">定时发送</option></select></label>
          {sendWay === '2' && <label>定时发送时间<input aria-label="定时发送时间" type="datetime-local" required value={definiteTime} onChange={(event) => onDefiniteTimeChange(event.target.value)} /></label>}
          {error && <p role="alert" className="phase34-inline-error">{error}</p>}
          <p className="phase34-field-hint">发送前会由 Go Provider 校验成员、群主、内容和企业微信凭据；失败会保留在任务状态中。</p>
          <div className="dashboard-table-actions"><button type="button" className="phase34-secondary-button" onClick={onClose}>取消</button><button type="submit" disabled={saving}>{saving ? '提交中…' : '保存并发送'}</button></div>
        </form>
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
  const [createOpen, setCreateOpen] = useState(false);
  const [createTitle, setCreateTitle] = useState('');
  const [createEmployeeIDs, setCreateEmployeeIDs] = useState('');
  const [createContent, setCreateContent] = useState('');
  const [createSendWay, setCreateSendWay] = useState('1');
  const [createDefiniteTime, setCreateDefiniteTime] = useState('');
  const [saving, setSaving] = useState(false);
  const [writeError, setWriteError] = useState('');
  const endpoint = mode === 'contact' ? '/contactMessageBatchSend/index' : '/roomMessageBatchSend/index';
  const query = useQuery({
    queryKey: ['phase34-precise-send', access.corp.id, mode, title],
    queryFn: () => api.read(endpoint, { ...(title ? { batchTitle: title } : {}), page: 1, perPage: 20 }),
  });
  const rows = useMemo(() => rowsFrom(query.data), [query.data]);
  const refresh = () => { void query.refetch(); };
  const createEndpoint = mode === 'contact' ? '/contactMessageBatchSend/store' : '/roomMessageBatchSend/store';
  const saveCreate = async () => {
    const ids = positiveReachIDs(createEmployeeIDs);
    if (!createTitle.trim() || !createContent.trim() || ids.length === 0) {
      setWriteError('请填写任务名称、目标成员或群主 ID 和群发内容。');
      return;
    }
    const body: Record<string, unknown> = { batchTitle: createTitle.trim(), employeeIds: ids, sendWay: Number(createSendWay), content: [{ msgType: 'text', content: createContent.trim() }] };
    if (mode === 'contact') body.filterParams = {};
    if (createSendWay === '2') body.definiteTime = createDefiniteTime.replace('T', ' ') + ':00';
    setSaving(true);
    setWriteError('');
    try {
      await api.write(createEndpoint, body, 'POST');
      setCreateOpen(false);
      setCreateTitle('');
      setCreateEmployeeIDs('');
      setCreateContent('');
      setCreateSendWay('1');
      setCreateDefiniteTime('');
      await query.refetch();
    } catch (error) {
      setWriteError(error instanceof Error ? error.message : '群发任务创建失败。');
    } finally {
      setSaving(false);
    }
  };

  return (
    <section className="phase34-page">
      <header className="phase34-page-header">
        <div><p className="phase34-eyebrow">营销工具 · 内容触达</p><h1>精准群发</h1><p>分别查看客户群发与群聊群发任务，追踪内容、执行结果和触达数据。</p></div>
        <div className="phase34-header-actions"><span className="phase34-provider-badge">双 Provider 已连接</span><button type="button" onClick={() => { setWriteError(''); setCreateOpen(true); }}>新建群发</button><button type="button" disabled={query.isFetching} onClick={refresh}>刷新</button></div>
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
        {writeError && !createOpen && <p role="alert" className="phase34-inline-error">{writeError}</p>}
        {query.isPending ? <PageState state="loading" /> : query.isError ? <PageState state={pageStateForError(query.error)} onRetry={refresh} /> : rows.length === 0 ? <PageState state="empty" title="暂无群发任务" description="当前筛选条件下没有可展示的任务。" /> : (
          <div className="dashboard-table-scroll"><table className="phase34-table phase34-reach-table"><thead><tr><th>创建时间</th><th>执行时间</th><th>发送内容</th><th>执行结果</th><th>执行数据</th><th>操作</th></tr></thead><tbody>{rows.map((row, index) => <tr key={recordKey(row, index)}><td>{primitive(row.createdAt)}</td><td>{primitive(row.sendTime ?? row.definiteTime)}</td><td>{contentText(row.content)}</td><td>{statusText(row.sendStatus)}</td><td>{executionData(row)}</td><td><button type="button" className="phase34-link-button" onClick={() => setSelected(row)}>详情</button></td></tr>)}</tbody></table></div>
        )}
      </div>
      {selected !== null && <SendDetail row={selected} onClose={() => setSelected(null)} />}
      {createOpen && <SendCreateDrawer mode={mode} title={createTitle} employeeIDs={createEmployeeIDs} content={createContent} sendWay={createSendWay} definiteTime={createDefiniteTime} saving={saving} error={writeError} onTitleChange={setCreateTitle} onEmployeeIDsChange={setCreateEmployeeIDs} onContentChange={setCreateContent} onSendWayChange={setCreateSendWay} onDefiniteTimeChange={setCreateDefiniteTime} onClose={() => { if (!saving) setCreateOpen(false); }} onSubmit={() => { void saveCreate(); }} />}
    </section>
  );
}

type FriendsCircleTab = 'task' | 'material';
type SelectableMaterial = { id: number; title: string; content: string };

function selectableMaterials(payload: unknown): SelectableMaterial[] {
  return rowsFrom(payload).flatMap((row) => {
    const id = Number(row.id);
    if (!Number.isInteger(id) || id <= 0) return [];
    const title = primitive(row.name ?? (isRecord(row.content) ? row.content.title ?? row.content.name : undefined));
    const content = primitive(row.preview) !== '--' ? primitive(row.preview) : contentText(row.content);
    return title === '--' || content === '--' ? [] : [{ id, title, content }];
  });
}

function friendsStatus(value: unknown): { label: string; tone: string } {
  const status = primitive(value);
  if (status === 'draft') return { label: '草稿', tone: 'draft' };
  if (status === 'available') return { label: '可用', tone: 'success' };
  if (status === 'failed') return { label: '失败', tone: 'danger' };
  if (status === 'publishing' || status === 'queued') return { label: '处理中', tone: 'progress' };
  return { label: status, tone: 'neutral' };
}

function FriendsCircleComposer({ tab, name, content, materials, saving, error, onNameChange, onContentChange, onClose, onSave }: {
  tab: FriendsCircleTab;
  name: string;
  content: string;
  materials: SelectableMaterial[];
  saving: boolean;
  error: string;
  onNameChange: (value: string) => void;
  onContentChange: (value: string) => void;
  onClose: () => void;
  onSave: () => void;
}) {
  const isTask = tab === 'task';
  const panelRef = useRef<HTMLDivElement>(null);
  const nameRef = useRef<HTMLInputElement>(null);
  const closeRef = useRef(onClose);
  const savingRef = useRef(saving);
  closeRef.current = onClose;
  savingRef.current = saving;
  useEffect(() => {
    const opener = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    nameRef.current?.focus();
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape' && !savingRef.current) closeRef.current();
      if (event.key !== 'Tab' || !panelRef.current) return;
      const focusable = Array.from(panelRef.current.querySelectorAll<HTMLElement>('button:not([disabled]), input:not([disabled]), textarea:not([disabled]), select:not([disabled])'));
      if (focusable.length === 0) { event.preventDefault(); panelRef.current.focus(); return; }
      const first = focusable[0]!;
      const last = focusable[focusable.length - 1]!;
      if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus(); }
      if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
    };
    document.addEventListener('keydown', handleKeyDown);
    return () => { document.removeEventListener('keydown', handleKeyDown); opener?.focus(); };
  }, []);
  return (
    <aside className="phase34-detail phase34-friends-drawer" aria-label="朋友圈草稿" data-variant={tab}>
      <div className="phase34-detail-backdrop" onClick={onClose} />
      <div ref={panelRef} className="phase34-detail-panel" role="dialog" aria-modal="true" aria-labelledby="friends-composer-title" tabIndex={-1}>
        <header className="phase34-friends-drawer-header">
          <div className="phase34-friends-drawer-icon" aria-hidden="true">{isTask ? '圈' : '素'}</div>
          <div><p className="phase34-eyebrow">{isTask ? '朋友圈任务' : '朋友圈素材'}</p><h2 id="friends-composer-title">{isTask ? '添加朋友圈草稿' : '添加文字素材'}</h2><p>{isTask ? '保存后可继续完善目标员工与正式发布配置。' : '创建可复用的文字内容，后续可用于朋友圈任务。'}</p></div>
          <button type="button" className="phase34-friends-close" aria-label="关闭新增面板" disabled={saving} onClick={onClose}>×</button>
        </header>
        <div className="phase34-friends-drawer-body">
          <div className="phase34-friends-safety"><strong>仅保存草稿</strong><span>当前不会向企业微信发布任何内容。</span></div>
          <div className="phase34-compose-form">
            {isTask && <label><span>引用素材</span><select aria-label="引用素材" disabled={saving} defaultValue="" onChange={(event) => { const material = materials.find((item) => item.id === Number(event.target.value)); if (material) onContentChange(material.content); }}><option value="">不引用，手动填写</option>{materials.map((material) => <option key={material.id} value={material.id}>{material.title}</option>)}</select><small>来自统一素材库，选择后仍可继续编辑。</small></label>}
            <label><span>{isTask ? '任务名称' : '素材名称'} <b>*</b></span><input ref={nameRef} aria-label="草稿名称" aria-describedby="friends-name-count" required disabled={saving} maxLength={80} placeholder={isTask ? '例如：秋季新品朋友圈' : '例如：新品上市文案'} value={name} onChange={(event) => onNameChange(event.target.value)} /><small id="friends-name-count">{name.length} / 80</small></label>
            <label><span>朋友圈内容 <b>*</b></span><textarea aria-label="草稿内容" aria-describedby="friends-content-count" required disabled={saving} maxLength={500} rows={9} placeholder="请输入准备发布的朋友圈文字内容…" value={content} onChange={(event) => onContentChange(event.target.value)} /><small id="friends-content-count">{content.length} / 500</small></label>
            {error && <p className="phase34-friends-form-error" role="alert">{error}</p>}
          </div>
        </div>
        <footer className="phase34-friends-drawer-footer"><span>带 * 为必填项</span><div><button type="button" className="phase34-secondary-button" disabled={saving} onClick={onClose}>取消</button><button type="button" aria-label="保存草稿" disabled={saving || !name.trim() || !content.trim()} onClick={onSave}>{saving ? '保存中…' : isTask ? '保存任务草稿' : '保存素材'}</button></div></footer>
      </div>
    </aside>
  );
}

export function FriendsCirclePage({ api }: { api: BusinessWorkbenchApi }) {
  const access = useDashboardAccess();
  const [tab, setTab] = useState<FriendsCircleTab>('task');
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
  const selectorQuery = useQuery({
    queryKey: ['phase34-material-selector', access.corp.id, 'friends_circle'],
    queryFn: () => api.read('/materialSelector/index', { scene: 'friends_circle' }),
    enabled: composerOpen && tab === 'task',
  });
  const rows = useMemo(() => rowsFrom(query.data), [query.data]);
  const materials = useMemo(() => selectableMaterials(selectorQuery.data), [selectorQuery.data]);
  const refresh = () => { void query.refetch(); };
  const applyFilter = () => {
    const next = draftFilter.trim();
    if (next === filter) void query.refetch(); else setFilter(next);
  };
  const openComposer = () => { setDraftName(''); setDraftContent(''); setSaveError(''); setComposerOpen(true); };
  const closeComposer = () => {
    if (saving) return false;
    if ((draftName.trim() || draftContent.trim()) && !window.confirm('当前内容尚未保存，确定关闭吗？')) return false;
    setComposerOpen(false);
    setSaveError('');
    return true;
  };
  const changeTab = (next: FriendsCircleTab) => {
    if (composerOpen && !closeComposer()) return;
    setTab(next);
    setDraftFilter('');
    setFilter('');
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
    <section className="phase34-page phase34-friends-page">
      <header className="phase34-page-header phase34-friends-header"><div><p className="phase34-eyebrow">营销工具 · 内容触达</p><h1>朋友圈</h1><p>统一管理朋友圈任务与内容素材，先沉淀草稿，再安全接入发布流程。</p></div><div className="phase34-header-actions"><span className="phase34-provider-badge"><i />草稿服务正常</span><button type="button" aria-label="添加朋友圈" onClick={openComposer}>＋ 添加朋友圈</button><button type="button" disabled>导出</button><button type="button" className="phase34-secondary-button" disabled={query.isFetching} onClick={refresh}>{query.isFetching ? '刷新中…' : '刷新'}</button></div></header>
      <div className="phase34-friends-provider-notice"><span aria-hidden="true">!</span><div><strong>发布 Provider 未配置</strong><p>当前支持任务与素材草稿管理，不会向企业微信实际发布。</p></div></div>
      <div className="phase34-tabs" role="tablist" aria-label="朋友圈内容类型"><button type="button" role="tab" aria-selected={tab === 'task'} className={tab === 'task' ? 'phase34-tab-active' : ''} onClick={() => changeTab('task')}>朋友圈</button><button type="button" role="tab" aria-selected={tab === 'material'} className={tab === 'material' ? 'phase34-tab-active' : ''} onClick={() => changeTab('material')}>朋友圈素材</button></div>
      <div className="dashboard-filter-bar phase34-filter-bar phase34-friends-filter">
        <label>{tab === 'task' ? '任务名称' : '素材关键字'}<input aria-label={tab === 'task' ? '任务名称' : '素材关键字'} placeholder={tab === 'task' ? '搜索任务名称' : '搜索素材名称或内容'} value={draftFilter} onChange={(event) => setDraftFilter(event.target.value)} onKeyDown={(event) => { if (event.key === 'Enter') applyFilter(); }} /></label><div className="dashboard-table-actions"><button type="button" onClick={applyFilter}>查询</button><button type="button" className="phase34-secondary-button" disabled={!draftFilter && !filter} onClick={() => { setDraftFilter(''); setFilter(''); }}>重置</button></div>
      </div>
      <div className="dashboard-data-card phase34-results-card phase34-friends-results"><div className="dashboard-card-heading"><div><h2>{tab === 'task' ? '朋友圈任务' : '朋友圈素材'}</h2><p>{tab === 'task' ? '管理待完善、待发布的朋友圈任务草稿。' : '管理可在朋友圈任务中复用的内容素材。'}</p></div><span>{rows.length} 条记录</span></div>{query.isPending ? <PageState state="loading" /> : query.isError ? <PageState state={pageStateForError(query.error)} onRetry={refresh} /> : rows.length === 0 ? <div className="phase34-friends-empty"><div aria-hidden="true">◎</div><h3>{tab === 'task' ? '还没有朋友圈任务' : '还没有朋友圈素材'}</h3><p>{filter ? '没有找到符合当前关键字的记录，请调整后重试。' : '创建第一条草稿，开始沉淀朋友圈内容。'}</p>{!filter && <button type="button" onClick={openComposer}>立即创建</button>}</div> : <div className="dashboard-table-scroll"><table className="phase34-table phase34-friends-table"><thead><tr><th>{tab === 'task' ? '任务名称' : '素材名称'}</th><th>{tab === 'task' ? '发送方式' : '内容摘要'}</th><th>状态</th><th>{tab === 'task' ? '完成情况' : '素材类型'}</th><th>创建人 / 创建时间</th><th>操作</th></tr></thead><tbody>{rows.map((row, index) => { const status = friendsStatus(row.status); return <tr key={recordKey(row, index)}><td><strong>{primitive(tab === 'task' ? row.taskName : row.name)}</strong></td><td className="phase34-friends-summary">{tab === 'task' ? (row.sendWay === 'manual' ? '员工手动发送' : primitive(row.sendWay)) : contentText(row.content)}</td><td><span className={`phase34-friends-status phase34-friends-status-${status.tone}`}>{status.label}</span></td><td>{tab === 'task' ? `${primitive(row.completedTotal)} / ${primitive(row.targetTotal)}` : primitive(row.type)}</td><td><span>{primitive(row.creatorName)}</span><small>{primitive(row.createdAt)}</small></td><td><span className="phase34-muted-action">草稿管理</span></td></tr>; })}</tbody></table></div>}</div>
      {composerOpen && <FriendsCircleComposer tab={tab} name={draftName} content={draftContent} materials={materials} saving={saving} error={saveError} onNameChange={setDraftName} onContentChange={setDraftContent} onClose={closeComposer} onSave={() => void saveDraft()} />}
    </section>
  );
}
