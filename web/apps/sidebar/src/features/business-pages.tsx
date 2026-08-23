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
type LoadState<T> = { kind: 'loading' } | { kind: 'error'; message: string; retryable: boolean; title: string } | { kind: 'ready'; value: T };

function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error ? error.message : fallback;
}

function classifiedError(error: unknown): Extract<LoadState<never>, { kind: 'error' }> {
  if (error instanceof MobileApiError) {
    if (error.kind === 'forbidden') return { kind: 'error', message: error.message, retryable: false, title: '无权访问' };
    if (error.kind === 'not-found') return { kind: 'error', message: error.message, retryable: false, title: '内容不存在' };
    if (error.kind === 'validation') return { kind: 'error', message: error.message, retryable: false, title: '参数有误' };
  }
  return { kind: 'error', message: errorMessage(error, '加载失败，请重试。'), retryable: true, title: '加载失败' };
}

function useAuthenticatedLoad<T>(loader: () => Promise<T>, onReauthenticate: () => void): [LoadState<T>, () => void, (value: T) => void] {
  const [attempt, setAttempt] = useState(0);
  const [state, setState] = useState<LoadState<T>>({ kind: 'loading' });
  useEffect(() => {
    let active = true; setState({ kind: 'loading' });
    void loader().then((value) => { if (active) setState({ kind: 'ready', value }); }).catch((error: unknown) => {
      if (!active) return;
      if (error instanceof MobileApiError && error.kind === 'unauthorized') onReauthenticate();
      else setState(classifiedError(error));
    });
    return () => { active = false; };
  }, [attempt, loader, onReauthenticate]);
  return [state, () => setAttempt((value) => value + 1), (value) => setState({ kind: 'ready', value })];
}

function StateCard({ state, retry, emptyTitle }: { state: LoadState<unknown>; retry: () => void; emptyTitle?: string }) {
  if (state.kind === 'loading') return <MobileCard padding="comfortable" tone="surface"><MobileState kind="loading" description="正在读取持久化数据。" /></MobileCard>;
  if (state.kind === 'error') return <MobileCard padding="comfortable" tone="surface"><MobileState {...(state.retryable ? { actionLabel: '重试', onAction: retry } : {})} description={state.message} kind="error" title={state.title} /></MobileCard>;
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
  const [groupId, setGroupId] = useState<number | null>(null); const [type, setType] = useState<number | null>(null); const [keyword, setKeyword] = useState(''); const [submittedKeyword, setSubmittedKeyword] = useState(''); const [page, setPage] = useState(1);
  const groupsLoader = useCallback(() => loadMediumGroups(request), [request]);
  const itemsLoader = useCallback(() => loadMediums(request, { groupId, keyword: submittedKeyword, page, type }), [groupId, page, request, submittedKeyword, type]);
  const [groups] = useAuthenticatedLoad<MediumGroup[]>(groupsLoader, onReauthenticate);
  const [items, retry] = useAuthenticatedLoad(itemsLoader, onReauthenticate);
  const [selected, setSelected] = useState<Map<number, MediumItem>>(new Map()); const [sending, setSending] = useState(false); const [notice, setNotice] = useState('');
  const selectedItems = useMemo(() => [...selected.values()], [selected]);

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
      } catch (error) {
        if (error instanceof MobileApiError && error.kind === 'unauthorized') { onReauthenticate(); break; }
        failures.push(`${mediumTitle(item)}：${errorMessage(error, '发送失败')}`);
      }
    }
    setNotice(failures.length === 0 ? `已发送 ${success} 项` : `已发送 ${success} 项；${failures.join('；')}`); setSending(false);
  };

  return <div className="sidebar-business-stack">
    <MobileCard padding="comfortable" tone="surface"><form className="sidebar-filter" onSubmit={(event) => { event.preventDefault(); setPage(1); setSubmittedKeyword(keyword); }}>
      <label htmlFor="medium-group">分组</label><select id="medium-group" onChange={(event) => { setPage(1); setGroupId(event.target.value === '' ? null : Number(event.target.value)); }} value={groupId ?? ''}><option value="">全部分组</option>{groups.kind === 'ready' ? groups.value.map((group) => <option key={group.id} value={group.id}>{group.name}</option>) : null}</select>
      <label htmlFor="medium-type">类型</label><select id="medium-type" onChange={(event) => { setPage(1); setType(event.target.value === '' ? null : Number(event.target.value)); }} value={type ?? ''}><option value="">全部类型</option><option value="1">文本</option><option value="2">图片</option><option value="3">图文</option><option value="5">视频</option><option value="7">文件</option></select>
      <label htmlFor="medium-search">搜索素材</label><div className="sidebar-filter__search"><input id="medium-search" onChange={(event) => setKeyword(event.target.value)} value={keyword} /><button type="submit">搜索</button></div>
    </form></MobileCard>
    {!bridge.available() ? <p className="sidebar-inline-notice">需在企业微信客户端中打开才能发送；当前仍可浏览真实素材。</p> : null}
    {items.kind !== 'ready' ? <StateCard retry={retry} state={items} /> : items.value.items.length === 0 ? <StateCard emptyTitle="暂无可用素材" retry={retry} state={items} /> : <section aria-label="素材列表" className="sidebar-medium-list">{items.value.items.map((item) => { const supported = mediumMessage(item, item.mediaId || 'pending') !== null; return <label className={`sidebar-medium-card${supported ? '' : ' sidebar-medium-card--disabled'}`} key={item.id}><input aria-label={mediumTitle(item)} checked={selected.has(item.id)} disabled={!supported} onChange={() => setSelected((current) => { const next = new Map(current); if (next.has(item.id)) next.delete(item.id); else next.set(item.id, item); return next; })} type="checkbox" /><span className="sidebar-medium-card__type">{supported ? item.typeLabel : `${item.typeLabel || '未知'}（暂不支持发送）`}</span><strong>{mediumTitle(item)}</strong>{item.content.imageFullPath ? <img alt={`${mediumTitle(item)}预览`} src={item.content.imageFullPath} /> : null}</label>; })}</section>}
    {items.kind === 'ready' && items.value.totalPage > 1 ? <div className="sidebar-pagination"><button disabled={page <= 1} onClick={() => setPage((value) => value - 1)} type="button">上一页</button><span>{page} / {items.value.totalPage}</span><button disabled={page >= items.value.totalPage} onClick={() => setPage((value) => value + 1)} type="button">下一页</button></div> : null}
    {notice ? <p aria-live="polite" className="sidebar-inline-notice">{notice}</p> : null}<button className="sidebar-form__primary sidebar-sticky-action" disabled={sending || selectedItems.length === 0} onClick={() => { void send(); }} type="button">{sending ? '发送中' : '发送已选素材'}</button>
  </div>;
}

async function copyText(text: string): Promise<void> {
  if (navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(text);
    return;
  }
  const textarea = document.createElement('textarea');
  textarea.value = text;
  textarea.readOnly = true;
  textarea.dataset.sidebarCopyFallback = 'true';
  textarea.style.position = 'fixed';
  textarea.style.opacity = '0';
  textarea.style.pointerEvents = 'none';
  document.body.append(textarea);
  try {
    textarea.focus();
    textarea.select();
    if (typeof document.execCommand !== 'function' || !document.execCommand('copy')) throw new Error('当前环境无法自动复制，请长按号码手动复制。');
  } finally {
    textarea.remove();
  }
}

function sopContent(item: SopContent, index: number) {
  const key = `${item.type}-${index}`;
  if (item.type === '0' || item.type === 'text') return <p key={key}>{item.value}</p>;
  if (item.type === '1' || item.type === 'image') return <img alt="SOP 图片素材" key={key} src={item.value} />;
  if (item.type === '2' || item.type === 'video' || item.type === '3' || item.type === 'file') return <a href={item.value} key={key} rel="noreferrer" target="_blank">查看 SOP 附件</a>;
  return <p className="contact-workspace__muted" key={key}>暂不支持的 SOP 素材类型</p>;
}

function SopContentList({ content, onCopied, onCopyError }: { content: SopContent[]; onCopied: () => void; onCopyError: (message: string) => void }) {
  const texts = content.filter((item) => item.type === '0' || item.type === 'text').map((item) => item.value).filter(Boolean);
  const copy = async () => { try { await copyText(texts.join('\n')); onCopied(); } catch (error) { onCopyError(errorMessage(error, '复制失败，请重试。')); } };
  return <section className="sidebar-sop-content"><h3>任务内容</h3>{content.length === 0 ? <p className="contact-workspace__muted">暂无任务内容</p> : content.map(sopContent)}{texts.length ? <button onClick={() => { void copy(); }} type="button">复制文本</button> : null}</section>;
}

function ContactSopCard({ sop }: { sop: ContactSop }) { const [notice, setNotice] = useState(''); const [failed, setFailed] = useState(false); return <MobileCard padding="comfortable" tone="surface"><article className="sidebar-sop-card"><header><div>{sop.avatar ? <img alt={`${sop.customerName}头像`} src={sop.avatar} /> : <span aria-hidden="true">客</span>}<div><h2>{sop.customerName}</h2><p>{sop.tipTime || sop.time}</p></div></div></header><SopContentList content={sop.content} onCopied={() => { setFailed(false); setNotice('文本已复制，尚未发送。'); }} onCopyError={(message) => { setFailed(true); setNotice(message); }} />{notice ? <p className={failed ? 'sidebar-form__error' : 'sidebar-inline-notice'} {...(failed ? { role: 'alert' } : {})}>{notice}</p> : null}</article></MobileCard>; }

export function ContactSopPage({ request, onReauthenticate }: CommonProps) {
  const [params] = useSearchParams(); const id = Number(params.get('id')); const contactId = Number(params.get('contactId')); const externalId = params.get('wxExternalUserid')?.trim() ?? '';
  const loader = useCallback(async () => { if (Number.isSafeInteger(id) && id > 0) return [await loadContactSop(request, id)]; if (Number.isSafeInteger(contactId) && contactId > 0) return loadContactSopTips(request, contactId); if (!externalId) throw new MobileApiError('validation', '缺少任务 ID 或客户 ID。'); const summary = await loadContactSummary(request, externalId); return loadContactSopTips(request, summary.id); }, [contactId, externalId, id, request]);
  const [state, retry] = useAuthenticatedLoad(loader, onReauthenticate);
  if (state.kind !== 'ready') return <StateCard retry={retry} state={state} />;
  return state.value.length === 0 ? <StateCard emptyTitle="当前客户暂无 SOP 提醒" retry={retry} state={state} /> : <div className="sidebar-business-stack">{state.value.map((sop) => <ContactSopCard key={sop.taskId} sop={sop} />)}</div>;
}

export function RoomSopPage({ request, onReauthenticate }: CommonProps) {
  const [params] = useSearchParams(); const id = Number(params.get('id')); const loader = useCallback(() => loadRoomSop(request, id), [id, request]); const [state, retry, setReady] = useAuthenticatedLoad(loader, onReauthenticate); const [saving, setSaving] = useState(false); const [notice, setNotice] = useState('');
  useEffect(() => { setNotice(''); setSaving(false); }, [id]);
  if (state.kind !== 'ready') return <StateCard retry={retry} state={state} />;
  const sop: RoomSop = state.value; const completed = sop.state !== 0;
  const markDone = async () => { if (saving || completed) return; setSaving(true); setNotice(''); try { await updateRoomSopState(request, sop.taskId); setReady(await loadRoomSop(request, id)); } catch (error) { if (error instanceof MobileApiError && error.kind === 'unauthorized') onReauthenticate(); else setNotice(errorMessage(error, '保存失败，请重试。')); } finally { setSaving(false); } };
  return <MobileCard padding="comfortable" tone="surface"><article className="sidebar-sop-card"><h2>{sop.roomName}</h2><p>执行时间：{sop.time || '未设置'}</p><SopContentList content={sop.content} onCopied={() => setNotice('文本已复制，尚未发送。')} onCopyError={setNotice} />{notice ? <p className="sidebar-form__error" role="alert">{notice}</p> : null}<button className="sidebar-form__primary" disabled={saving || completed} onClick={() => { void markDone(); }} type="button">{completed ? '已完成' : saving ? '保存并确认中' : '标记为已完成'}</button></article></MobileCard>;
}

const statusOptions = [{ value: 4, label: '全部' }, { value: 0, label: '待分配' }, { value: 1, label: '待添加' }, { value: 2, label: '待通过' }, { value: 3, label: '已添加' }];
export function BatchAddPage({ request, onReauthenticate, bridge }: CommonProps & { bridge: WeComBridge }) {
  const [params] = useSearchParams(); const batchId = Number(params.get('batchId')); const [status, setStatus] = useState(4); const loader = useCallback(() => loadBatchAdd(request, batchId, status), [batchId, request, status]); const [state, retry] = useAuthenticatedLoad(loader, onReauthenticate); const [notice, setNotice] = useState(''); const [addingId, setAddingId] = useState<number | null>(null);
  const add = async (contact: { id: number; phone: string }) => { if (addingId !== null) return; if (!bridge.available()) { setNotice('需在企业微信客户端中打开加客户能力。'); return; } setAddingId(contact.id); setNotice(''); try { await copyText(contact.phone); await bridge.navigateToAddCustomer(); setNotice(`已复制 ${contact.phone} 并打开企业微信；添加结果以企业微信为准。`); } catch (error) { if (error instanceof MobileApiError && error.kind === 'unauthorized') onReauthenticate(); else setNotice(errorMessage(error, '无法复制号码或打开加客户页面。')); } finally { setAddingId(null); } };
  return <div className="sidebar-business-stack"><MobileCard padding="comfortable" tone="surface"><label className="sidebar-form__label" htmlFor="batch-status">添加状态</label><select id="batch-status" onChange={(event) => setStatus(Number(event.target.value))} value={status}>{statusOptions.map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}</select></MobileCard>{state.kind !== 'ready' ? <StateCard retry={retry} state={state} /> : <MobileCard padding="comfortable" tone="surface"><h2 className="sidebar-section-title">{state.value.employeeName || '当前员工'}</h2>{state.value.contacts.length === 0 ? <MobileState kind="empty" title="当前筛选下暂无客户" /> : <ul className="sidebar-batch-list">{state.value.contacts.map((contact) => <li key={contact.id}><span>{contact.phone}</span><strong>{contact.status}</strong><button disabled={addingId !== null} onClick={() => { void add(contact); }} type="button">{addingId === contact.id ? '处理中' : '复制并添加'}</button></li>)}</ul>}</MobileCard>}{notice ? <p className="sidebar-inline-notice">{notice}</p> : null}</div>;
}
