import { useMutation, useQuery } from '@tanstack/react-query';
import { useState } from 'react';
import { useOptionalDashboardAccess } from '../../app/access-context';
import type { Phase35Api } from './api';
import { records, text } from './api';
import { Phase35PageShell } from './components/phase35-page-shell';
import { Phase35DataState } from './components/data-state';
const definitions = [{ type: 'customer_source', label: '客户来源', description: '用于客户来源标记' }, { type: 'funnel_stage', label: '转化阶段', description: '用于转化漏斗阶段' }, { type: 'assignment', label: '默认分配', description: '用于新客户分配策略' }] as const;
export function SettingsPage({ api }: { api: Phase35Api }) {
  const corpId = useOptionalDashboardAccess()?.corp.id;
  const query = useQuery({ queryKey: ['p35-settings', corpId], queryFn: () => api.read('/scrm/settings', { corpId: Number(corpId) }), enabled: Boolean(corpId) });
  const [type, setType] = useState<(typeof definitions)[number]['type']>('customer_source'); const [key, setKey] = useState(''); const [value, setValue] = useState('');
  const save = useMutation({ mutationFn: () => api.write('/scrm/settings', { corpId: Number(corpId), type, key: key.trim(), label: definitions.find((item) => item.type === type)?.label, value: value.trim(), enabled: true }, 'PUT'), onSuccess: () => { setKey(''); setValue(''); void query.refetch(); } });
  const valid = Boolean(corpId && key.trim() && value.trim());
  return <Phase35PageShell title="客户设置" description="维护客户来源、转化阶段与分配策略，保存后可在行为报表追溯"><form className="dashboard-filter-bar" onSubmit={(event) => { event.preventDefault(); if (valid) save.mutate(); }}><label>设置分类<select value={type} onChange={(event) => setType(event.target.value as (typeof definitions)[number]['type'])}>{definitions.map((item) => <option key={item.type} value={item.type}>{item.label}</option>)}</select></label><span>{definitions.find((item) => item.type === type)?.description}</span><label>名称<input aria-label="设置名称" value={key} onChange={(event) => setKey(event.target.value)} /></label><label>值<input aria-label="设置值" value={value} onChange={(event) => setValue(event.target.value)} /></label><button type="submit" disabled={!valid || save.isPending}>保存设置</button></form><Phase35DataState loading={query.isLoading} error={query.isError} empty={!corpId}><ul>{records(query.data).map((row, index) => <li key={index}>{text(row.label ?? row.key)}：{text(row.value)}（版本 {text(row.version)}）</li>)}</ul></Phase35DataState>{save.isError && <p role="alert">保存失败，请检查版本冲突或权限</p>}{save.isSuccess && <p role="status">设置已保存并回读</p>}</Phase35PageShell>;
}
