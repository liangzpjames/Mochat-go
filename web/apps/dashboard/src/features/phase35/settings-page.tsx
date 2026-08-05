import { useMutation, useQuery } from '@tanstack/react-query';
import { useState } from 'react';
import { useOptionalDashboardAccess } from '../../app/access-context';
import type { Phase35Api, Row } from './api';
import { records, text } from './api';
import { Phase35PageShell } from './components/phase35-page-shell';
import { Phase35DataState } from './components/data-state';

const definitions = [
  { type: 'customer_source', label: '客户来源', description: '用于统一标记客户的获客渠道', placeholder: '例如：官网咨询' },
  { type: 'funnel_stage', label: '转化阶段', description: '用于定义线索到订单的业务阶段', placeholder: '例如：方案确认' },
  { type: 'assignment', label: '默认分配', description: '用于设置新客户的默认分配策略', placeholder: '例如：按部门轮询' },
] as const;

export function SettingsPage({ api }: { api: Phase35Api }) {
  const corpId = useOptionalDashboardAccess()?.corp.id;
  const query = useQuery({ queryKey: ['p35-settings', corpId], queryFn: () => api.read('/scrm/settings', { corpId: Number(corpId) }), enabled: Boolean(corpId) });
  const [type, setType] = useState<string>('customer_source'); const [key, setKey] = useState(''); const [value, setValue] = useState(''); const [enabled, setEnabled] = useState(true); const [version, setVersion] = useState(0); const [id, setId] = useState('');
  const definition = definitions.find((item) => item.type === type) ?? definitions[0];
  const save = useMutation({ mutationFn: () => api.write('/scrm/settings', { id: id || undefined, corpId: Number(corpId), type, key: key.trim(), label: definition.label, value: value.trim(), enabled, version: version || undefined }, 'PUT'), onSuccess: () => { setId(''); setKey(''); setValue(''); setVersion(0); setEnabled(true); void query.refetch(); } });
  const edit = (row: Row) => { setId(String(row.id ?? '')); setType(String(row.type ?? 'customer_source')); setKey(String(row.key ?? '')); setValue(String(row.value ?? '')); setEnabled(Boolean(row.enabled)); setVersion(Number(row.version ?? 0)); };
  const valid = Boolean(corpId && /^[a-z0-9_-]{2,40}$/i.test(key.trim()) && value.trim().length >= 2 && value.trim().length <= 100);
  return <Phase35PageShell title="客户设置" description="维护类型化客户配置；保存采用版本控制并可在行为报告追溯">
    <form className="dashboard-filter-bar" onSubmit={(event) => { event.preventDefault(); if (valid) save.mutate(); }}>
      <label>设置分类<select aria-label="设置分类" value={type} onChange={(event) => setType(event.target.value)}>{definitions.map((item) => <option key={item.type} value={item.type}>{item.label}</option>)}</select></label><span>{definition.description}</span>
      <label>配置编码<input aria-label="配置编码" value={key} onChange={(event) => setKey(event.target.value)} /></label>
      <label>配置值<input aria-label="配置值" placeholder={definition.placeholder} value={value} onChange={(event) => setValue(event.target.value)} /></label>
      <label><input aria-label="启用配置" type="checkbox" checked={enabled} onChange={(event) => setEnabled(event.target.checked)} />启用</label>
      <button type="submit" disabled={!valid || save.isPending}>{save.isPending ? '保存中…' : id ? '保存修改' : '新增配置'}</button>
    </form>
    {!valid && (key || value) && <p role="status">配置编码需为 2–40 位字母、数字、下划线或短横线；配置值需为 2–100 字。</p>}
    <Phase35DataState loading={query.isLoading} error={query.isError} empty={!query.isLoading && records(query.data).length === 0} onRetry={() => void query.refetch()}><table><thead><tr><th>分类</th><th>名称</th><th>当前值</th><th>状态</th><th>版本</th><th>修改人/时间</th><th>操作</th></tr></thead><tbody>{records(query.data).map((row) => <tr key={String(row.id ?? row.key)}><td>{definitions.find((item) => item.type === row.type)?.label ?? text(row.type)}</td><td>{text(row.label ?? row.key)}</td><td>{text(row.value)}</td><td>{row.enabled ? '已启用' : '已停用'}</td><td>v{text(row.version)}</td><td>{text(row.updatedByName ?? row.updatedBy)} / {text(row.updatedAt)}</td><td><button type="button" onClick={() => edit(row)}>编辑</button></td></tr>)}</tbody></table></Phase35DataState>
    {save.isError && <p role="alert">保存失败：版本冲突、字段校验失败或权限不足，请刷新后重试。</p>}{save.isSuccess && <p role="status">设置已保存并回读，可在行为报告查看审计事件。</p>}
  </Phase35PageShell>;
}
