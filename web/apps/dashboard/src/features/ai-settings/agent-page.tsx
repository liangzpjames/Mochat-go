import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { useOptionalDashboardAccess } from '../../app/access-context';
import { ConfirmAction } from '../../components/confirm-action';
import { Phase35PageShell } from '../phase35/components/phase35-page-shell';
import { Phase35DataState } from '../phase35/components/data-state';
import type { AISettingsApi, AgentItem } from './ai-settings-api';

export function AgentPage({ api }: { api: AISettingsApi }) {
  const corpId = useOptionalDashboardAccess()?.corp.id;
  const queryClient = useQueryClient();
  const [editing, setEditing] = useState<AgentItem | null>(null);
  const [creating, setCreating] = useState(false);
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [selectedBases, setSelectedBases] = useState<string[]>([]);
  const [status, setStatus] = useState(1);
  const [error, setError] = useState('');

  const query = useQuery({
    queryKey: ['ai-agents', corpId],
    queryFn: () => api.listAgents(Number(corpId)),
    enabled: Boolean(corpId),
  });
  const knowledgeBases = useQuery({
    queryKey: ['ai-kb', corpId],
    queryFn: () => api.listKnowledgeBases(Number(corpId)),
    enabled: Boolean(corpId),
  });
  const items = query.data ?? [];
  const total = items.length;
  const enabledCount = items.filter((item) => item.status === 1).length;

  const save = useMutation({
    mutationFn: () => editing
      ? api.updateAgent(Number(corpId), editing.id, { name: name.trim(), description: description.trim(), knowledgeBaseIds: selectedBases, status })
      : api.createAgent(Number(corpId), { name: name.trim(), description: description.trim(), knowledgeBaseIds: selectedBases, status }),
    onSuccess: () => {
      setEditing(null); setCreating(false); setName(''); setDescription(''); setSelectedBases([]); setStatus(1); setError('');
      void queryClient.invalidateQueries({ queryKey: ['ai-agents'] });
    },
    onError: (err) => setError(err instanceof Error ? err.message : '保存失败'),
  });
  const remove = useMutation({
    mutationFn: (id: string) => api.deleteAgent(Number(corpId), id),
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: ['ai-agents'] }),
  });

  const openCreate = () => { setCreating(true); setEditing(null); setName(''); setDescription(''); setSelectedBases([]); setStatus(1); setError(''); };
  const openEdit = (item: AgentItem) => {
    setEditing(item); setCreating(false); setName(item.name); setDescription(item.description);
    setSelectedBases(item.knowledgeBaseIds ?? []); setStatus(item.status); setError('');
  };
  const toggleBase = (id: string) => setSelectedBases((current) => current.includes(id) ? current.filter((item) => item !== id) : [...current, id]);
  const valid = Boolean(corpId && name.trim().length >= 2 && name.trim().length <= 128);

  return (
    <Phase35PageShell title="智能体管理" description="维护智能体：名称、说明、关联知识库与启停状态" actions={<button type="button" onClick={openCreate}>新建智能体</button>}>
      <div className="phase35-page">
        <section className="phase35-kpis" aria-label="智能体指标">
          <article className="phase35-kpi phase35-kpi-primary"><span>智能体总数</span><strong>{total}</strong><small>当前企业内已创建的智能体</small></article>
          <article className="phase35-kpi phase35-kpi-green"><span>已启用</span><strong>{enabledCount}</strong><small>状态为启用的智能体</small></article>
          <article className="phase35-kpi phase35-kpi-violet"><span>可关联知识库数</span><strong>{knowledgeBases.data?.length ?? 0}</strong><small>可用于关联的知识库数量</small></article>
        </section>

        <section className="phase35-card phase35-table-card">
          <header className="phase35-card-header"><div><h2>智能体列表</h2><p>创建、编辑与停用智能体</p></div><span className="phase35-chip">共 {total} 条</span></header>
          <Phase35DataState
            loading={query.isLoading}
            error={query.isError}
            empty={!total}
            emptyContent={<p className="phase35-empty">暂无智能体，点击“新建智能体”开始创建</p>}
            onRetry={() => void query.refetch()}
          >
            <div className="phase35-table">
              <table>
                <thead><tr><th>名称</th><th>说明</th><th>关联知识库</th><th>状态</th><th>更新时间</th><th>操作</th></tr></thead>
                <tbody>
                  {items.map((item) => (
                    <tr key={item.id}>
                      <td>{item.name}</td><td>{item.description || '—'}</td>
                      <td>{(item.knowledgeBaseIds ?? []).length} 个</td>
                      <td>{item.status === 1 ? '启用' : '停用'}</td><td>{item.updatedAt}</td>
                      <td>
                        <button type="button" onClick={() => openEdit(item)}>编辑</button>
                        <ConfirmAction title={`确认删除智能体“${item.name}”？`} onConfirm={() => remove.mutate(item.id)}><button type="button">删除</button></ConfirmAction>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </Phase35DataState>
        </section>

        {(creating || editing) && (
          <section className="phase35-card" role="dialog" aria-label="智能体表单">
            <header className="phase35-card-header"><div><h2>{editing ? '编辑智能体' : '新建智能体'}</h2></div></header>
            {error && <p role="alert" className="phase35-limits">{error}</p>}
            <form onSubmit={(event) => { event.preventDefault(); if (valid) save.mutate(); }}>
              <label>名称<input value={name} onChange={(event) => setName(event.target.value)} placeholder="如：智能客服" /></label>
              <label>说明<input value={description} onChange={(event) => setDescription(event.target.value)} /></label>
              <fieldset>
                <legend>关联知识库</legend>
                {(knowledgeBases.data ?? []).length === 0 && <p className="phase35-empty">暂无知识库可选</p>}
                {(knowledgeBases.data ?? []).map((kb) => (
                  <label key={kb.id}><input type="checkbox" checked={selectedBases.includes(kb.id)} onChange={() => toggleBase(kb.id)} />{kb.name}</label>
                ))}
              </fieldset>
              <label>状态<select value={status} onChange={(event) => setStatus(Number(event.target.value))}><option value={1}>启用</option><option value={0}>停用</option></select></label>
              <button type="submit" disabled={!valid || save.isPending}>保存</button>
              <button type="button" onClick={() => { setEditing(null); setCreating(false); setError(''); }}>取消</button>
            </form>
          </section>
        )}
      </div>
    </Phase35PageShell>
  );
}
