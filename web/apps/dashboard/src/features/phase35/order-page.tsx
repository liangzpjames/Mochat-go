import { useQuery } from '@tanstack/react-query';
import { useOptionalDashboardAccess } from '../../app/access-context';
import type { Phase35Api } from './api';
import { records, text } from './api';

export function OrderPage({ api }: { api: Phase35Api }) {
  const access = useOptionalDashboardAccess();
  const corpId = access?.corp.id;
  const query = useQuery({ queryKey: ['p35-orders', corpId], queryFn: () => api.read('/scrm/orders', { corpId: Number(corpId) }), enabled: Boolean(corpId) });
  const rows = records(query.data);
  return <section className="dashboard-content"><h1>订单</h1><button type="button" onClick={() => void query.refetch()} disabled={!corpId}>刷新</button>{query.isLoading ? <p>加载中…</p> : query.isError ? <p role="alert">订单加载失败</p> : !corpId ? <p role="status">未选择企业</p> : rows.length === 0 ? <p>暂无数据。</p> : <table><tbody>{rows.map((row, index) => <tr key={index}><td>{text(row.id)}</td><td>{text(row.amountCents)}</td><td>{text(row.status)}</td></tr>)}</tbody></table>}</section>;
}
