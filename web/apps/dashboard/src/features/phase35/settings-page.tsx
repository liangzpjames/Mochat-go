import { useQuery } from '@tanstack/react-query';
import { useOptionalDashboardAccess } from '../../app/access-context';
import type { Phase35Api } from './api';
import { records, text } from './api';

export function SettingsPage({ api }: { api: Phase35Api }) {
  const access = useOptionalDashboardAccess();
  const corpId = access?.corp.id;
  const query = useQuery({ queryKey: ['p35-settings', corpId], queryFn: () => api.read('/scrm/settings', { corpId: Number(corpId) }), enabled: Boolean(corpId) });
  return <section className="dashboard-content"><h1>客户设置</h1>{query.isLoading ? <p>加载中…</p> : query.isError ? <p role="alert">设置加载失败</p> : !corpId ? <p role="status">未选择企业</p> : <ul>{records(query.data).map((row, index) => <li key={index}>{text(row.label ?? row.key)}：{text(row.value)}</li>)}</ul>}</section>;
}
