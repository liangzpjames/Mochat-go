import { useMutation, useQuery } from '@tanstack/react-query';
import { useState } from 'react';
import { useOptionalDashboardAccess } from '../../app/access-context';
import type { Phase35Api, Row } from './api';
import { records, text } from './api';
import { Phase35PageShell } from './components/phase35-page-shell';
import { Phase35DataState } from './components/data-state';
import { Phase35DetailDrawer } from './components/detail-drawer';

const statusLabel: Record<string, string> = { pending: '待支付', paid: '已支付', fulfilled: '已完成', cancelled: '已取消' };
const nextStatuses: Record<string, string[]> = { pending: ['paid', 'cancelled'], paid: ['fulfilled', 'cancelled'] };
const unwrap = (value: unknown): Row => value && typeof value === 'object' && 'data' in value ? ((value as { data?: Row }).data ?? {}) : (value as Row ?? {});

export function OrderPage({ api }: { api: Phase35Api }) {
  const corpId = useOptionalDashboardAccess()?.corp.id;
  const [contactId, setContactId] = useState(''); const [amount, setAmount] = useState(''); const [title, setTitle] = useState(''); const [note, setNote] = useState(''); const [selectedId, setSelectedId] = useState('');
  const orders = useQuery({ queryKey: ['p35-orders', corpId], queryFn: () => api.read('/scrm/orders', { corpId: Number(corpId) }), enabled: Boolean(corpId) });
  const contacts = useQuery({ queryKey: ['p35-order-contacts', corpId], queryFn: () => api.read('/workContact/index', { corpId: Number(corpId), name: '', page: 1, perPage: 50 }), enabled: Boolean(corpId) });
  const detail = useQuery({ queryKey: ['p35-order-detail', selectedId, corpId], queryFn: () => api.read(`/scrm/orders/${selectedId}`, { corpId: Number(corpId) }), enabled: Boolean(selectedId && corpId) });
  const create = useMutation({ mutationFn: () => api.write('/scrm/orders', { id: `P35-ORDER-${Date.now()}`, corpId: Number(corpId), contactId, amountCents: Math.round(Number(amount) * 100), currency: 'CNY', status: 'pending', title: title.trim(), note: note.trim() }, 'POST'), onSuccess: () => { setAmount(''); setTitle(''); setNote(''); void orders.refetch(); } });
  const current = unwrap(detail.data); const status = String(current.status ?? '');
  const transition = useMutation({ mutationFn: (next: string) => api.write(`/scrm/orders/${selectedId}/transition?corpId=${Number(corpId)}`, { status: next, version: Number(current.version) }, 'PUT'), onSuccess: () => { void orders.refetch(); void detail.refetch(); } });
  const rows = records(orders.data); const options = records(contacts.data); const valid = Boolean(corpId && contactId && title.trim() && /^\d+(\.\d{1,2})?$/.test(amount) && Number(amount) > 0);
  return <Phase35PageShell title="订单" description="创建、查看、流转订单并追溯审计时间线">
    <form className="dashboard-filter-bar" onSubmit={(event) => { event.preventDefault(); if (valid) create.mutate(); }}>
      <label>联系人<select aria-label="联系人" value={contactId} onChange={(event) => setContactId(event.target.value)}><option value="">请选择联系人</option>{options.map((row) => <option key={String(row.id ?? row.contactId)} value={String(row.id ?? row.contactId)}>{text(row.name ?? row.contactName)}</option>)}</select></label>
      {!contacts.isLoading && options.length === 0 && <p role="status">暂无联系人，请先前往客户联系人页面创建联系人。</p>}
      <label>订单标题<input aria-label="订单标题" value={title} onChange={(event) => setTitle(event.target.value)} /></label>
      <label>金额（元）<input aria-label="金额（元）" inputMode="decimal" value={amount} onChange={(event) => setAmount(event.target.value)} /></label>
      <label>备注<input aria-label="备注" value={note} onChange={(event) => setNote(event.target.value)} /></label>
      <button type="submit" disabled={!valid || create.isPending}>{create.isPending ? '创建中…' : '创建订单'}</button>
    </form>
    {create.isError && <p role="alert">创建失败，请检查字段、权限或版本状态。</p>}{create.isSuccess && <p role="status">订单已创建并回读列表。</p>}
    <Phase35DataState loading={orders.isLoading} error={orders.isError} empty={!orders.isLoading && !rows.length} onRetry={() => void orders.refetch()}><table><thead><tr><th>订单号</th><th>联系人</th><th>金额</th><th>状态</th><th>操作</th></tr></thead><tbody>{rows.map((row) => <tr key={String(row.id)}><td>{text(row.id)}</td><td>{text(row.contactName ?? row.contactId)}</td><td>¥{(Number(row.amountCents ?? 0) / 100).toFixed(2)}</td><td>{statusLabel[String(row.status)] ?? text(row.status)}</td><td><button type="button" onClick={() => setSelectedId(String(row.id))}>查看详情</button></td></tr>)}</tbody></table></Phase35DataState>
    <Phase35DetailDrawer title="订单详情" open={Boolean(selectedId)} onClose={() => setSelectedId('')}>
      {detail.isLoading ? <p>正在加载…</p> : detail.isError ? <p role="alert">详情加载失败</p> : <><dl><dt>订单号</dt><dd>{text(current.id)}</dd><dt>联系人</dt><dd>{text(current.contactName ?? current.contactId)}</dd><dt>金额</dt><dd>¥{(Number(current.amountCents ?? 0) / 100).toFixed(2)}</dd><dt>状态</dt><dd>{statusLabel[status] ?? text(status)}</dd><dt>版本</dt><dd>{text(current.version)}</dd></dl><h3>合法状态流转</h3>{(nextStatuses[status] ?? []).map((next) => <button key={next} type="button" disabled={transition.isPending} onClick={() => transition.mutate(next)}>变更为{statusLabel[next]}</button>)}<h3>审计时间线</h3>{records((detail.data as { audit?: unknown })?.audit).map((event, index) => <p key={index}>{text(event.action)} · {text(event.createdAt)} · {text(event.actorId)}</p>)}{transition.isError && <p role="alert">状态更新失败：版本冲突或非法流转，请刷新后重试。</p>}</>}
    </Phase35DetailDrawer>
  </Phase35PageShell>;
}
