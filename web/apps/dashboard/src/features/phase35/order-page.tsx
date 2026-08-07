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
const auditActionLabel: Record<string, string> = { created: '创建订单', transition: '变更订单状态' };
const unwrap = (value: unknown): Row => value && typeof value === 'object' && 'data' in value ? ((value as { data?: Row }).data ?? {}) : (value as Row ?? {});

export function OrderPage({ api }: { api: Phase35Api }) {
  const corpId = useOptionalDashboardAccess()?.corp.id;
  const [contactId, setContactId] = useState('');
  const [opportunityId, setOpportunityId] = useState('');
  const [quickContact, setQuickContact] = useState<Row>();
  const [contactKeyword, setContactKeyword] = useState('');
  const [quickName, setQuickName] = useState('');
  const [quickPhone, setQuickPhone] = useState('');
  const [amount, setAmount] = useState('');
  const [title, setTitle] = useState('');
  const [note, setNote] = useState('');
  const [selectedId, setSelectedId] = useState('');
  const [page, setPage] = useState(1);
  const orders = useQuery({
    queryKey: ['p35-orders', corpId, page],
    queryFn: () => api.read('/scrm/orders', { corpId: Number(corpId), page, pageSize: 20 }),
    enabled: Boolean(corpId),
  });
  const contacts = useQuery({
    queryKey: ['p35-order-contacts', corpId, contactKeyword],
    queryFn: () => api.read('/scrm/contacts', { corpId: Number(corpId), keyword: contactKeyword, pageSize: 50 }),
    enabled: Boolean(corpId),
  });
  const opportunities = useQuery({
    queryKey: ['p35-order-opportunities', corpId],
    queryFn: () => api.read('/scrm/opportunities', { corpId: Number(corpId), pageSize: 50 }),
    enabled: Boolean(corpId),
  });
  const detail = useQuery({
    queryKey: ['p35-order-detail', selectedId, corpId],
    queryFn: () => api.read(`/scrm/orders/${selectedId}`, { corpId: Number(corpId) }),
    enabled: Boolean(selectedId && corpId),
  });
  const create = useMutation({
    mutationFn: () => api.write('/scrm/orders', {
      corpId: Number(corpId), contactId, opportunityId,
      amountCents: Math.round(Number(amount) * 100), currency: 'CNY', status: 'pending',
      title: title.trim(), note: note.trim(),
    }, 'POST'),
    onSuccess: () => { setAmount(''); setTitle(''); setNote(''); setOpportunityId(''); void orders.refetch(); },
  });
  const quickCreate = useMutation({
    mutationFn: () => api.write('/scrm/contacts', { corpId: Number(corpId), name: quickName.trim(), phone: quickPhone.trim() }, 'POST'),
    onSuccess: (payload) => {
      const contact = unwrap(payload);
      setQuickContact(contact);
      setContactId(String(contact.id ?? ''));
      setQuickName('');
      setQuickPhone('');
      void contacts.refetch();
    },
  });
  const detailPayload = (detail.data ?? {}) as Row;
  const current = unwrap(detailPayload.order);
  const status = String(current.status ?? '');
  const transition = useMutation({
    mutationFn: (next: string) => api.write(`/scrm/orders/${selectedId}/transition?corpId=${Number(corpId)}`, { status: next, version: Number(current.version) }, 'PUT'),
    onSuccess: () => { void orders.refetch(); void detail.refetch(); },
  });
  const rows = records(orders.data);
  const listPayload = (orders.data ?? {}) as { items?: Row[]; total?: number };
  const total = Number(listPayload.total ?? rows.length);
  const options = [...(quickContact ? [quickContact] : []), ...records(contacts.data).filter((item) => item.id !== quickContact?.id)];
  const valid = Boolean(corpId && contactId && title.trim() && /^\d+(\.\d{1,2})?$/.test(amount) && Number(amount) > 0);
  return (
    <Phase35PageShell title="订单" description="创建、查看、流转订单并追踪审计时间线">
      <div className="phase35-page">
        <section className="phase35-card phase35-filter-card">
          <form className="dashboard-filter-bar" onSubmit={(event) => { event.preventDefault(); if (valid) create.mutate(); }}>
            <label>搜索联系人<input aria-label="搜索联系人" value={contactKeyword} onChange={(event) => setContactKeyword(event.target.value)} /></label>
            <label>联系人
              <select aria-label="联系人" value={contactId} onChange={(event) => setContactId(event.target.value)}>
                <option value="">请选择联系人</option>
                {options.map((row) => <option key={String(row.id ?? row.contactId)} value={String(row.id ?? row.contactId)}>{text(row.name ?? row.contactName)}</option>)}
              </select>
            </label>
            <label>关联商机
              <select aria-label="关联商机" value={opportunityId} onChange={(event) => setOpportunityId(event.target.value)}>
                <option value="">不关联商机</option>
                {records(opportunities.data).map((row) => <option key={String(row.id)} value={String(row.id)}>{text(row.title ?? row.name ?? row.id)}</option>)}
              </select>
            </label>
            {!contacts.isLoading && options.length === 0 && (
              <fieldset>
                <legend>快速创建联系人</legend>
                <p>当前没有可用联系人，可在此创建并自动选中。</p>
                <label>联系人姓名<input aria-label="快速联系人姓名" value={quickName} onChange={(event) => setQuickName(event.target.value)} /></label>
                <label>手机号码<input aria-label="快速联系人手机" value={quickPhone} onChange={(event) => setQuickPhone(event.target.value.replace(/\D/g, ''))} /></label>
                <button type="button" disabled={!quickName.trim() || quickCreate.isPending} onClick={() => quickCreate.mutate()}>{quickCreate.isPending ? '创建中…' : '创建并选中'}</button>
                {quickCreate.isError && <p role="alert">联系人创建失败，请检查姓名、手机号或权限。</p>}
              </fieldset>
            )}
            <label>订单标题<input aria-label="订单标题" value={title} onChange={(event) => setTitle(event.target.value)} /></label>
            <label>金额（元）<input aria-label="金额（元）" inputMode="decimal" value={amount} onChange={(event) => setAmount(event.target.value)} /></label>
            <label>备注<input aria-label="备注" value={note} onChange={(event) => setNote(event.target.value)} /></label>
            <button type="submit" disabled={!valid || create.isPending}>{create.isPending ? '创建中…' : '创建订单'}</button>
          </form>
        </section>
        {quickCreate.isSuccess && <p role="status">联系人已创建并自动选中，可以继续填写订单。</p>}
        {create.isError && <p role="alert">创建失败，请检查字段、权限或版本状态。</p>}
        {create.isSuccess && <p role="status">订单已创建并回填列表。</p>}
        <section className="phase35-card">
          <header className="phase35-card-header">
            <div><h2>订单列表</h2><p>创建、查看、流转订单并追踪审计时间线</p></div>
            <span className="phase35-chip">共 {total} 条</span>
          </header>
          <div className="phase35-table-card">
            <Phase35DataState loading={orders.isLoading} error={orders.isError} empty={!orders.isLoading && !rows.length} onRetry={() => void orders.refetch()}>
              <table>
                <thead><tr><th>订单号</th><th>标题</th><th>联系人</th><th>金额</th><th>状态</th><th>操作</th></tr></thead>
                <tbody>
                  {rows.map((row) => (
                    <tr key={String(row.id)}>
                      <td>{text(row.id)}</td>
                      <td>{text(row.title)}</td>
                      <td>{text(row.contactName ?? row.contactId)}</td>
                      <td>¥{(Number(row.amountCents ?? 0) / 100).toFixed(2)}</td>
                      <td><span className={`order-status order-status-${String(row.status)}`}>{statusLabel[String(row.status)] ?? text(row.status)}</span></td>
                      <td><button type="button" onClick={() => setSelectedId(String(row.id))}>查看详情</button></td>
                    </tr>
                  ))}
                </tbody>
              </table>
              <div className="dashboard-pagination">
                <span>共 {total} 条，第 {page} 页</span>
                <button type="button" disabled={page <= 1} onClick={() => setPage((value) => Math.max(1, value - 1))}>上一页</button>
                <button type="button" disabled={page * 20 >= total} onClick={() => setPage((value) => value + 1)}>下一页</button>
              </div>
            </Phase35DataState>
          </div>
        </section>
        <Phase35DetailDrawer title="订单详情" open={Boolean(selectedId)} onClose={() => setSelectedId('')}>
          {detail.isLoading ? <p>正在加载…</p> : detail.isError ? <p role="alert">详情加载失败</p> : (
            <>
              <dl>
                <dt>订单号</dt><dd>{text(current.id)}</dd>
                <dt>标题</dt><dd>{text(current.title)}</dd>
                <dt>联系人</dt><dd>{text(current.contactName ?? current.contactId)}</dd>
                <dt>关联商机</dt><dd>{text(current.opportunityId)}</dd>
                <dt>金额</dt><dd>¥{(Number(current.amountCents ?? 0) / 100).toFixed(2)}</dd>
                <dt>备注</dt><dd>{text(current.note)}</dd>
                <dt>状态</dt><dd><span className={`order-status order-status-${status}`}>{statusLabel[status] ?? text(status)}</span></dd>
                <dt>版本</dt><dd>{text(current.version)}</dd>
              </dl>
              <h3>合法状态流转</h3>
              {(nextStatuses[status] ?? []).map((next) => (
                <button key={next} type="button" disabled={transition.isPending} onClick={() => transition.mutate(next)}>变更为{statusLabel[next]}</button>
              ))}
              <h3>审计时间线</h3>
              {records(detailPayload.audit).map((event, index) => (
                <p key={index}>{auditActionLabel[String(event.action)] ?? text(event.action)} · 操作人{text(event.actorId)} · {text(event.createdAt)} · 版本 {text(event.fromVersion)} → {text(event.toVersion)}</p>
              ))}
              {transition.isError && <p role="alert">状态更新失败：版本冲突或非法流转，请刷新后重试。</p>}
            </>
          )}
        </Phase35DetailDrawer>
      </div>
    </Phase35PageShell>
  );
}
