import { MobileApiError, MobileCard, MobileState } from '@mochat/mobile-foundation';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { useSearchParams } from 'react-router';

import type { WeComBridge, WeComMessage } from '../wecom/wecom-bridge';
import {
  loadBatchAdd,
  loadContactSop,
  loadContactSopTips,
  loadMediumGroups,
  loadMediums,
  loadRoomSop,
  refreshMediumMediaId,
  updateRoomSopState,
  type BusinessRequest,
  type ContactSop,
  type MediumGroup,
  type MediumItem,
  type RoomSop,
  type SopContent,
} from './business-api';
import { loadContactSummary } from './contact/contact-api';

type CommonProps = { request: BusinessRequest; onReauthenticate: () => void };
type LoadState<T> = { kind: 'loading' } | { kind: 'error'; message: string } | { kind: 'ready'; value: T };

function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error ? error.message : fallback;
}

function useAuthenticatedLoad<T>(loader: () => Promise<T>, onReauthenticate: () => void): [LoadState<T>, () => void] {
  const [attempt, setAttempt] = useState(0);
  const [state, setState] = useState<LoadState<T>>({ kind: 'loading' });
  useEffect(() => {
    let active = true; setState({ kind: 'loading' });
    void loader().then((value) => { if (active) setState({ kind: 'ready', value }); }).catch((error: unknown) => {
      if (!active) return;
      if (error instanceof MobileApiError && error.kind === 'unauthorized') onReauthenticate();
      else setState({ kind: 'error', message: errorMessage(error, '加载失败，请重试。') });
    });
    return () => { active = false; };
  }, [attempt, loader, onReauthenticate]);
  return [state, () => setAttempt((value) => value + 1)];
}

function StateCard({ state, retry, emptyTitle }: { state: LoadState<unknown>; retry: () => void; emptyTitle?: string }) {
  if (state.kind === 'loading') return <MobileCard padding="comfortable" tone="surface"><MobileState kind="loading" description="正在读取持久化数据。" /></MobileCard>;
  if (state.kind === 'error') return <MobileCard padding="comfortable" tone="surface"><MobileState actionLabel="重试" description={state.message} kind="error" onAction={retry} title="加载失败" /></MobileCard>;
  return emptyTitle ? <MobileCard padding="comfortable" tone="surface"><MobileState kind="empty" title={emptyTitle} /></MobileCard> : null;
}

function mediumTitle(item: MediumItem): string {
  return item.content.title || item.content.content || item.content.fileName || item.content.videoName || `素材 ${item.id}`;
}

function mediumMessage(item: MediumItem, mediaId: string): WeComMessage | null {
  if (item.kind === 'text' && item.content.content) return { type: 'text', content: item.content.content };
  if (item.kind === 'link' && item.content.imageLink) return { type: 'news', link: item.content.imageLink, title: item.content.title || '图文素材', description: item.content.description || '', imageUrl: item.content.imageFullPath || '' };
  if ((item.kind === 'image' || item.kind === 'video' || item.kind === 'file') && mediaId) return { type: item.kind, mediaId };
  return null;
}

export function MediumPage({ request, onReauthenticate, bridge }: CommonProps & { bridge: WeComBridge }) {
  const [groupId, setGroupId] = useState<number | null>(null); const [keyword, setKeyword] = useState(''); const [submittedKeyword, setSubmittedKeyword] = useState(''); const [page, setPage] = useState(1);
  const groupsLoader = useCallback(() => loadMediumGroups(request), [request]);
  const itemsLoader = useCallback(() => loadMediums(request, { groupId, keyword: submittedKeyword, page }), [groupId, page, request, submittedKeyword]);
  const [groups] = useAuthenticatedLoad<MediumGroup[]>(groupsLoader, onReauthenticate);
  const [items, retry] = useAuthenticatedLoad(itemsLoader, onReauthenticate);
  const [selected, setSelected] = useState<Set<number>>(new Set()); const [sending, setSending] = useState(false); const [notice, setNotice] = useState('');
  const selectedItems = useMemo(() => items.kind === 'ready' ? items.value.items.filter((item) => selected.has(item.id)) : [], [items, selected]);

  const send = async () => {
    if (sending || selectedItems.length === 0) return;
    if (!bridge.available()) { setNotice('需在企业微信客户端中打开才能发送素材。'); return; }
    setSending(true); setNotice(''); let success = 0; const failures: string[] = [];
    for (const item of selectedItems) {
      try {
        const mediaId = item.kind === 'image' || item.kind === 'video' || item.kind === 'file' ? await refreshMediumMediaId(request, item.id) : item.mediaId;
        const message = mediumMessage(item, mediaId);
        if (message === null) throw new Error('该素材缺少可发送内容。');
        await bridge.sendChatMessage(message); success += 1;
      } catch (error) { failures.push(`${mediumTitle(item)}：${errorMessage(error, '发送失败')}`); }
    }
    setNotice(failures.length === 0 ? `已发送 ${success} 项` : `已发送 ${success} 项；${failures.join('；')}`); setSending(false);
  };

  return <div className="sidebar-business-stack">
    <MobileCard padding="comfortable" tone="surface"><form className="sidebar-filter" onSubmit={(event) => { event.preventDefault(); setPage(1); setSubmittedKeyword(keyword); }}>
      <label htmlFor="medium-group">分组</label><select id="medium-group" onChange={(event) => { setPage(1); setGroupId(event.target.value === '' ? null : Number(event.target.value)); }} value={groupId ?? ''}><option value="">全部分组</option>{groups.kind === 'ready' ? groups.value.map((group) => <option key={group.id} value={group.id}>{group.name}</option>) : null}</select>
      <label htmlFor="medium-search">搜索素材</label><div className="sidebar-filter__search"><input id="medium-search" onChange={(event) => setKeyword(event.target.value)} value={keyword} /><button type="submit">搜索</button></div>
    </form></MobileCard>
    {!bridge.available() ? <p className="sidebar-inline-notice">需在企业微信客户端中打开才能发送；当前仍可浏览真实素材。</p> : null}
    {items.kind !== 'ready' ? <StateCard retry={retry} state={items} /> : items.value.items.length === 0 ? <StateCard emptyTitle="暂无可用素材" retry={retry} state={items} /> : <section aria-label="素材列表" className="sidebar-medium-list">{items.value.items.map((item) => <label className="sidebar-medium-card" key={item.id}><input aria-label={mediumTitle(item)} checked={selected.has(item.id)} onChange={() => setSelected((current) => { const next = new Set(current); if (next.has(item.id)) next.delete(item.id); else next.add(item.id); return next; })} type="checkbox" /><span className="sidebar-medium-card__type">{item.typeLabel}</span><strong>{mediumTitle(item)}</strong>{item.content.imageFullPath ? <img alt="" src={item.content.imageFullPath} /> : null}</label>)}</section>}
    {items.kind === 'ready' && items.value.totalPage > 1 ? <div className="sidebar-pagination"><button disabled={page <= 1} onClick={() => setPage((value) => value - 1)} type="button">上一页</button><span>{page} / {items.value.totalPage}</span><button disabled={page >= items.value.totalPage} onClick={() => setPage((value) => value + 1)} type="button">下一页</button></div> : null}
    {notice ? <p aria-live="polite" className="sidebar-inline-notice">{notice}</p> : null}<button className="sidebar-form__primary sidebar-sticky-action" disabled={sending || selectedItems.length === 0} onClick={() => { void send(); }} type="button">{sending ? '发送中' : '发送已选素材'}</button>
  </div>;
}

function SopContentList({ content, onCopied }: { content: SopContent[]; onCopied: () => void }) {
  const texts = content.filter((item) => item.type === '0' || item.type === 'text').map((item) => item.value).filter(Boolean);
  const copy = async () => { await navigator.clipboard.writeText(texts.join('\n')); onCopied(); };
  return <section className="sidebar-sop-content"><h3>任务内容</h3>{content.length === 0 ? <p className="contact-workspace__muted">暂无任务内容</p> : content.map((item, index) => item.type === '0' || item.type === 'text' ? <p key={`${item.type}-${index}`}>{item.value}</p> : <img alt="SOP 素材" key={`${item.type}-${index}`} src={item.value} />)}{texts.length ? <button onClick={() => { void copy(); }} type="button">复制文本</button> : null}</section>;
}

function ContactSopCard({ sop }: { sop: ContactSop }) { const [notice, setNotice] = useState(''); return <MobileCard padding="comfortable" tone="surface"><article className="sidebar-sop-card"><header><div>{sop.avatar ? <img alt="" src={sop.avatar} /> : <span aria-hidden="true">客</span>}<div><h2>{sop.customerName}</h2><p>{sop.tipTime || sop.time}</p></div></div></header><SopContentList content={sop.content} onCopied={() => setNotice('文本已复制，尚未发送。')} />{notice ? <p className="sidebar-inline-notice">{notice}</p> : null}</article></MobileCard>; }

export function ContactSopPage({ request, onReauthenticate }: CommonProps) {
  const [params] = useSearchParams(); const id = Number(params.get('id')); const externalId = params.get('wxExternalUserid')?.trim() ?? '';
  const loader = useCallback(async () => { if (Number.isSafeInteger(id) && id > 0) return [await loadContactSop(request, id)]; if (!externalId) throw new MobileApiError('validation', '缺少任务 ID 或 wxExternalUserid。'); const summary = await loadContactSummary(request, externalId); return loadContactSopTips(request, summary.id); }, [externalId, id, request]);
  const [state, retry] = useAuthenticatedLoad(loader, onReauthenticate);
  if (state.kind !== 'ready') return <StateCard retry={retry} state={state} />;
  return state.value.length === 0 ? <StateCard emptyTitle="当前客户暂无 SOP 提醒" retry={retry} state={state} /> : <div className="sidebar-business-stack">{state.value.map((sop) => <ContactSopCard key={sop.taskId} sop={sop} />)}</div>;
}

export function RoomSopPage({ request, onReauthenticate }: CommonProps) {
  const [params] = useSearchParams(); const id = Number(params.get('id')); const loader = useCallback(() => loadRoomSop(request, id), [id, request]); const [state, retry] = useAuthenticatedLoad(loader, onReauthenticate); const [saving, setSaving] = useState(false); const [done, setDone] = useState(false); const [notice, setNotice] = useState('');
  if (state.kind !== 'ready') return <StateCard retry={retry} state={state} />;
  const sop: RoomSop = state.value; const completed = done || sop.state === 1;
  const markDone = async () => { if (saving || completed) return; setSaving(true); setNotice(''); try { await updateRoomSopState(request, sop.taskId); setDone(true); } catch (error) { if (error instanceof MobileApiError && error.kind === 'unauthorized') onReauthenticate(); else setNotice(errorMessage(error, '保存失败，请重试。')); } finally { setSaving(false); } };
  return <MobileCard padding="comfortable" tone="surface"><article className="sidebar-sop-card"><h2>{sop.roomName}</h2><p>执行时间：{sop.time || '未设置'}</p><SopContentList content={sop.content} onCopied={() => setNotice('文本已复制，尚未发送。')} />{notice ? <p className="sidebar-form__error" role="alert">{notice}</p> : null}<button className="sidebar-form__primary" disabled={saving || completed} onClick={() => { void markDone(); }} type="button">{completed ? '已完成' : saving ? '保存中' : '标记为已完成'}</button></article></MobileCard>;
}

const statusOptions = [{ value: 4, label: '全部' }, { value: 0, label: '待分配' }, { value: 1, label: '待添加' }, { value: 2, label: '待通过' }, { value: 3, label: '已添加' }];
export function BatchAddPage({ request, onReauthenticate, bridge }: CommonProps & { bridge: WeComBridge }) {
  const [params] = useSearchParams(); const batchId = Number(params.get('batchId')); const [status, setStatus] = useState(4); const loader = useCallback(() => loadBatchAdd(request, batchId, status), [batchId, request, status]); const [state, retry] = useAuthenticatedLoad(loader, onReauthenticate); const [notice, setNotice] = useState('');
  const add = async () => { if (!bridge.available()) { setNotice('需在企业微信客户端中打开加客户能力。'); return; } try { await bridge.navigateToAddCustomer(); setNotice('已打开企业微信加客户页面；添加结果以企业微信为准。'); } catch (error) { setNotice(errorMessage(error, '无法打开加客户页面。')); } };
  return <div className="sidebar-business-stack"><MobileCard padding="comfortable" tone="surface"><label className="sidebar-form__label" htmlFor="batch-status">添加状态</label><select id="batch-status" onChange={(event) => setStatus(Number(event.target.value))} value={status}>{statusOptions.map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}</select></MobileCard>{state.kind !== 'ready' ? <StateCard retry={retry} state={state} /> : <MobileCard padding="comfortable" tone="surface"><h2 className="sidebar-section-title">{state.value.employeeName || '当前员工'}</h2>{state.value.contacts.length === 0 ? <MobileState kind="empty" title="当前筛选下暂无客户" /> : <ul className="sidebar-batch-list">{state.value.contacts.map((contact) => <li key={contact.id}><span>{contact.phone}</span><strong>{contact.status}</strong></li>)}</ul>}</MobileCard>}<button className="sidebar-form__primary" onClick={() => { void add(); }} type="button">打开企微加客户</button>{notice ? <p className="sidebar-inline-notice">{notice}</p> : null}</div>;
}
