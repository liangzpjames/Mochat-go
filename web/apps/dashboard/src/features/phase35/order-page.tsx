import { useMutation, useQuery } from '@tanstack/react-query';
import { useState } from 'react';
import { useOptionalDashboardAccess } from '../../app/access-context';
import type { Phase35Api } from './api';
import { records, text } from './api';

export function OrderPage({ api }: { api: Phase35Api }) {
  const access = useOptionalDashboardAccess();
  const corpId = access?.corp.id;
  const query = useQuery({ queryKey: ['p35-orders', corpId], queryFn: () => api.read('/scrm/orders', { corpId: Number(corpId) }), enabled: Boolean(corpId) });
  const [contactId, setContactId] = useState('');
  const [amountCents, setAmountCents] = useState('');
  const create = useMutation({ mutationFn: () => api.write('/scrm/orders', { corpId: Number(corpId), contactId, amountCents: Number(amountCents), currency: 'CNY', status: 'pending' }, 'POST'), onSuccess: () => { setContactId(''); setAmountCents(''); void query.refetch(); } });
  const rows = records(query.data);
  return <section className="dashboard-content"><h1>订单</h1><form className="dashboard-filter-bar" onSubmit={(event) => { event.preventDefault(); create.mutate(); }}><label>联系人 ID<input aria-label="联系人 ID" value={contactId} onChange={(event) => setContactId(event.target.value)} /></label><label>金额（分）<input aria-label="金额（分）" inputMode="numeric" value={amountCents} onChange={(event) => setAmountCents(event.target.value.replace(/\D/g, ''))} /></label><button type="submit" disabled={!corpId || !contactId || !amountCents || create.isPending}>创建订单</button></form><button type="button" onClick={() => void query.refetch()} disabled={!corpId}>刷新</button>{query.isLoading ? <p>加载中…</p> : query.isError ? <p role="alert">订单加载失败</p> : !corpId ? <p role="status">未选择企业</p> : rows.length === 0 ? <p>暂无数据。</p> : <table><thead><tr><th>ID</th><th>金额</th><th>状态</th></tr></thead><tbody>{rows.map((row, index) => <tr key={index}><td>{text(row.id)}</td><td>{text(row.amountCents)}</td><td>{text(row.status)}</td></tr>)}</tbody></table>}</section>;
}
