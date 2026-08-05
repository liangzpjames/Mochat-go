import { useMutation, useQuery } from '@tanstack/react-query';
import { useState } from 'react';
import { useOptionalDashboardAccess } from '../../app/access-context';
import type { Phase35Api } from './api';
import { records, text } from './api';

export function SettingsPage({ api }: { api: Phase35Api }) {
  const access = useOptionalDashboardAccess();
  const corpId = access?.corp.id;
  const query = useQuery({ queryKey: ['p35-settings', corpId], queryFn: () => api.read('/scrm/settings', { corpId: Number(corpId) }), enabled: Boolean(corpId) });
  const [key, setKey] = useState('');
  const [value, setValue] = useState('');
  const save = useMutation({ mutationFn: () => api.write('/scrm/settings', { corpId: Number(corpId), type: 'scrm', key, label: key, value, enabled: true }, 'PUT'), onSuccess: () => { setKey(''); setValue(''); void query.refetch(); } });
  return <section className="dashboard-content"><h1>客户设置</h1><form className="dashboard-filter-bar" onSubmit={(event) => { event.preventDefault(); save.mutate(); }}><label>设置键<input aria-label="设置键" value={key} onChange={(event) => setKey(event.target.value)} /></label><label>设置值<input aria-label="设置值" value={value} onChange={(event) => setValue(event.target.value)} /></label><button type="submit" disabled={!corpId || !key || save.isPending}>保存设置</button></form>{query.isLoading ? <p>加载中…</p> : query.isError ? <p role="alert">设置加载失败</p> : !corpId ? <p role="status">未选择企业</p> : <ul>{records(query.data).map((row, index) => <li key={index}>{text(row.label ?? row.key)}：{text(row.value)}</li>)}</ul>}</section>;
}
