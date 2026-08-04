import { useQuery } from '@tanstack/react-query';
import { useEffect, useMemo, useRef, useState } from 'react';

import { useDashboardAccess } from '../../../app/access-context';
import { pageStateForError, PageState } from '../../../components/page-state/page-state';
import type { BusinessWorkbenchApi } from '../../business-workbench/business-workbench-page';
import { materialPreview, materialTitle, type MaterialGroup, type MaterialRecord, type MaterialScope } from './material-types';

function records(payload: unknown): MaterialRecord[] {
  if (!payload || typeof payload !== 'object') return [];
  const source = payload as Record<string, unknown>;
  const list = Array.isArray(payload) ? payload : source.list;
  return Array.isArray(list) ? list.filter((item): item is MaterialRecord => typeof item === 'object' && item !== null && Number.isFinite(Number((item as { id?: unknown }).id))) : [];
}

function groups(payload: unknown): MaterialGroup[] {
  const list = Array.isArray(payload) ? payload : payload && typeof payload === 'object' ? (payload as Record<string, unknown>).list : [];
  return Array.isArray(list) ? list.map((item) => item as MaterialGroup).filter((item) => Number.isFinite(Number(item.id)) && typeof item.name === 'string') : [];
}

type DepartmentOption = { id: number; name: string };

function departments(payload: unknown): DepartmentOption[] {
  const source = payload && typeof payload === 'object' ? (payload as Record<string, unknown>).list : [];
  const result: DepartmentOption[] = [];
  const visit = (items: unknown) => {
    if (!Array.isArray(items)) return;
    for (const raw of items) {
      if (!raw || typeof raw !== 'object') continue;
      const item = raw as Record<string, unknown>;
      const id = Number(item.departmentId);
      if (Number.isInteger(id) && id > 0 && typeof item.name === 'string') result.push({ id, name: item.name });
      visit(item.children);
    }
  };
  visit(source);
  return result;
}

const scopes: Array<{ id: MaterialScope; label: string; description: string }> = [
  { id: 'public', label: '公共素材', description: '企业成员可复用' },
  { id: 'department', label: '部门素材', description: '按部门沉淀内容' },
  { id: 'personal', label: '个人素材', description: '仅自己可管理' },
  { id: 'sidebar', label: '聊天侧边栏', description: '会话中快捷发送' },
];

function MaterialComposer({ groups: options, departments: departmentOptions, scope, saving, onClose, onSave }: { groups: MaterialGroup[]; departments: DepartmentOption[]; scope: MaterialScope; saving: boolean; onClose: () => void; onSave: (values: { name: string; content: string; groupId: number; scopeId: number; sidebarVisible: boolean }) => void }) {
  const [name, setName] = useState('');
  const [content, setContent] = useState('');
  const [groupId, setGroupId] = useState(0);
  const [scopeId, setScopeId] = useState(0);
  const [sidebarVisible, setSidebarVisible] = useState(scope === 'sidebar');
  const panel = useRef<HTMLDivElement>(null);
  const nameInput = useRef<HTMLInputElement>(null);
  const closeRef = useRef(onClose);
  const savingRef = useRef(saving);
  closeRef.current = onClose;
  savingRef.current = saving;
  useEffect(() => {
    const opener = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    nameInput.current?.focus();
    const keyboard = (event: KeyboardEvent) => {
      if (event.key === 'Escape' && !savingRef.current) closeRef.current();
      if (event.key !== 'Tab' || !panel.current) return;
      const focusable = Array.from(panel.current.querySelectorAll<HTMLElement>('button:not([disabled]),input:not([disabled]),textarea:not([disabled]),select:not([disabled])'));
      if (!focusable.length) { event.preventDefault(); panel.current.focus(); return; }
      const first = focusable[0]!; const last = focusable[focusable.length - 1]!;
      if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus(); }
      if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
    };
    document.addEventListener('keydown', keyboard);
    return () => { document.removeEventListener('keydown', keyboard); opener?.focus(); };
  }, []);
  const close = () => {
    if (saving) return;
    if ((name.trim() || content.trim()) && !window.confirm('当前素材尚未保存，确定关闭吗？')) return;
    onClose();
  };
  return <aside className="phase34-detail phase34-material-drawer" aria-label="素材编辑面板"><div className="phase34-detail-backdrop" onClick={close} /><div ref={panel} tabIndex={-1} className="phase34-detail-panel" role="dialog" aria-modal="true" aria-label="添加素材"><header><div><p className="phase34-eyebrow">素材底座 · 文本素材</p><h2>添加素材</h2><p>保存后可在朋友圈、群发、活码和聊天侧边栏中复用。</p></div><button type="button" aria-label="关闭素材面板" disabled={saving} onClick={close}>×</button></header><div className="phase34-material-drawer-body"><label>素材名称 *<input ref={nameInput} aria-label="素材名称" required disabled={saving} maxLength={80} value={name} onChange={(event) => setName(event.target.value)} /><small>{name.length} / 80</small></label><label>素材正文 *<textarea aria-label="素材正文" required disabled={saving} maxLength={1000} rows={10} value={content} onChange={(event) => setContent(event.target.value)} /><small>{content.length} / 1000</small></label><label>素材分组<select aria-label="素材分组" disabled={saving} value={groupId} onChange={(event) => setGroupId(Number(event.target.value))}>{options.map((group) => <option key={group.id} value={group.id}>{group.name}</option>)}</select></label>{scope === 'department' && <label>所属部门 *<select aria-label="所属部门" required disabled={saving} value={scopeId} onChange={(event) => setScopeId(Number(event.target.value))}><option value={0}>请选择部门</option>{departmentOptions.map((department) => <option key={department.id} value={department.id}>{department.name}</option>)}</select></label>}<label className="phase34-material-check"><input type="checkbox" disabled={saving} checked={sidebarVisible} onChange={(event) => setSidebarVisible(event.target.checked)} />聊天侧边栏可见</label></div><footer><span>当前仅创建文本素材，不会触发企业微信外部同步。</span><div><button type="button" disabled={saving} onClick={close}>取消</button><button type="button" aria-label="保存素材" disabled={saving || !name.trim() || !content.trim() || (scope === 'department' && scopeId === 0)} onClick={() => onSave({ name: name.trim(), content: content.trim(), groupId, scopeId, sidebarVisible })}>{saving ? '保存中…' : '保存素材'}</button></div></footer></div></aside>;
}

export function MaterialManagementPage({ api }: { api: BusinessWorkbenchApi }) {
  const access = useDashboardAccess();
  const [scope, setScope] = useState<MaterialScope>('public');
  const [groupId, setGroupId] = useState(0);
  const [type, setType] = useState(0);
  const [draftKeyword, setDraftKeyword] = useState('');
  const [keyword, setKeyword] = useState('');
  const [selected, setSelected] = useState<Set<number>>(new Set());
  const [moveGroupId, setMoveGroupId] = useState(0);
  const [composerOpen, setComposerOpen] = useState(false);
  const [saving, setSaving] = useState(false);
  const [actionError, setActionError] = useState('');
  const queryScope = scope === 'sidebar' ? 'public' : scope;
  const groupQuery = useQuery({ queryKey: ['phase34-material-groups', access.session.corpId], queryFn: () => api.read('/mediumGroup/index', {}) });
  const departmentQuery = useQuery({ queryKey: ['phase34-material-departments', access.session.corpId], queryFn: () => api.read('/workDepartment/pageIndex', { page: 1, perPage: 500 }), enabled: scope === 'department' });
  const materialQuery = useQuery({ queryKey: ['phase34-materials', access.session.corpId, scope, groupId, type, keyword], queryFn: () => api.read('/medium/index', { page: 1, perPage: 20, scopeType: queryScope, ...(scope === 'sidebar' ? { sidebarVisible: 1 } : {}), ...(groupId ? { mediumGroupId: groupId } : {}), ...(type ? { type } : {}), ...(keyword ? { searchStr: keyword } : {}) }) });
  const materialGroups = useMemo(() => groups(groupQuery.data), [groupQuery.data]);
  const departmentOptions = useMemo(() => departments(departmentQuery.data), [departmentQuery.data]);
  const rows = useMemo(() => records(materialQuery.data), [materialQuery.data]);
  const applyKeyword = () => { const next = draftKeyword.trim(); if (next === keyword) void materialQuery.refetch(); else setKeyword(next); };
  const toggle = (id: number) => setSelected((current) => { const next = new Set(current); if (next.has(id)) next.delete(id); else next.add(id); return next; });
  const createMaterial = async (values: { name: string; content: string; groupId: number; scopeId: number; sidebarVisible: boolean }) => {
    setSaving(true); setActionError('');
    try {
      await api.write('/medium/store', { type: 1, mediumGroupId: values.groupId, scopeType: queryScope, ...(queryScope === 'department' ? { scopeId: values.scopeId } : {}), sidebarVisible: values.sidebarVisible, content: { title: values.name, content: values.content } }, 'POST');
      setComposerOpen(false); await materialQuery.refetch();
    } catch (error) { setActionError(error instanceof Error ? error.message : '保存素材失败'); }
    finally { setSaving(false); }
  };
  const batchMove = async () => {
    setActionError('');
    try { await api.write('/medium/batchGroupUpdate', { ids: [...selected], mediumGroupId: moveGroupId }, 'POST'); await materialQuery.refetch(); }
    catch (error) { setActionError(error instanceof Error ? error.message : '批量移动失败'); }
  };
  const batchDelete = async () => {
    setActionError('');
    try { await api.write('/medium/batchDestroy', { ids: [...selected] }, 'POST'); setSelected(new Set()); await materialQuery.refetch(); }
    catch (error) { setActionError(error instanceof Error ? error.message : '批量删除失败'); }
  };
  const createGroup = async () => {
    const name = window.prompt('请输入素材分组名称')?.trim();
    if (!name) return;
    setActionError('');
    try { await api.write('/mediumGroup/store', { name }, 'POST'); await groupQuery.refetch(); }
    catch (error) { setActionError(error instanceof Error ? error.message : '创建分组失败'); }
  };
  return <section className="phase34-page phase34-material-page"><header className="phase34-page-header phase34-material-header"><div><p className="phase34-eyebrow">营销工具 · 素材底座</p><h1>素材管理</h1><p>统一沉淀企业内容，在朋友圈、群发、活码和聊天侧边栏中安全复用。</p></div><div className="phase34-header-actions"><span className="phase34-provider-badge">真实素材 Provider</span><button type="button" aria-label="添加素材" onClick={() => { setActionError(''); setComposerOpen(true); }}>＋ 添加素材</button><button type="button" className="phase34-secondary-button" onClick={() => void materialQuery.refetch()}>刷新</button></div></header><div className="phase34-material-scopes" role="tablist" aria-label="素材作用域">{scopes.map((item) => <button key={item.id} type="button" role="tab" aria-selected={scope === item.id} className={scope === item.id ? 'active' : ''} onClick={() => { setScope(item.id); setSelected(new Set()); }}><strong>{item.label}</strong><span>{item.description}</span></button>)}</div><div className="phase34-material-workbench"><aside className="phase34-material-groups"><div><h2>素材分组</h2><button type="button" aria-label="创建分组" onClick={() => void createGroup()}>＋</button></div>{groupQuery.isPending ? <p>加载分组…</p> : materialGroups.map((group) => <button type="button" key={group.id} className={groupId === group.id ? 'active' : ''} onClick={() => setGroupId(group.id)}><span>{group.name}</span></button>)}</aside><main><div className="phase34-material-filter"><label>搜索素材<input aria-label="搜索素材" placeholder="搜索名称或正文" value={draftKeyword} onChange={(event) => setDraftKeyword(event.target.value)} onKeyDown={(event) => { if (event.key === 'Enter') applyKeyword(); }} /></label><label>素材类型<select aria-label="素材类型" value={type} onChange={(event) => setType(Number(event.target.value))}><option value={0}>全部类型</option><option value={1}>文本</option><option value={2}>图片</option><option value={6}>文件</option></select></label><button type="button" onClick={applyKeyword}>查询</button><button type="button" disabled={!draftKeyword && !keyword && !type} onClick={() => { setDraftKeyword(''); setKeyword(''); setType(0); }}>重置</button></div>{selected.size > 0 && <div className="phase34-material-batch"><strong>已选择 {selected.size} 项</strong><select aria-label="移动到分组" value={moveGroupId} onChange={(event) => setMoveGroupId(Number(event.target.value))}>{materialGroups.map((group) => <option key={group.id} value={group.id}>{group.name}</option>)}</select><button type="button" onClick={() => void batchMove()}>批量移动</button><button type="button" className="danger" onClick={() => void batchDelete()}>批量删除</button></div>}{actionError && <p className="phase34-material-error" role="alert">{actionError}</p>}{materialQuery.isPending ? <PageState state="loading" /> : materialQuery.isError ? <PageState state={pageStateForError(materialQuery.error)} onRetry={() => void materialQuery.refetch()} /> : rows.length === 0 ? <PageState state="empty" title="暂无素材" description="添加第一条可复用素材，建立企业内容底座。" /> : <div className="phase34-material-grid">{rows.map((item) => { const title = materialTitle(item); return <article key={item.id}><div className="phase34-material-card-top"><input type="checkbox" aria-label={`选择${title}`} checked={selected.has(item.id)} onChange={() => toggle(item.id)} /><span>{item.type}</span></div><h3>{title}</h3><p>{materialPreview(item)}</p><dl><div><dt>分组</dt><dd>{item.mediumGroupName || '未分组'}</dd></div><div><dt>创建人</dt><dd>{item.userName || '--'}</dd></div></dl><footer><span className={`phase34-material-status ${item.status}`}>{item.status === 'available' ? '可用' : item.status}</span><time>{item.createdAt}</time></footer></article>; })}</div>}</main></div>{composerOpen && <MaterialComposer groups={materialGroups} departments={departmentOptions} scope={scope} saving={saving} onClose={() => setComposerOpen(false)} onSave={(values) => void createMaterial(values)} />}</section>;
}
