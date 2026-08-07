import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { useOptionalDashboardAccess } from '../../app/access-context';
import { ConfirmAction } from '../../components/confirm-action';
import { Phase35PageShell } from '../phase35/components/phase35-page-shell';
import { Phase35DataState } from '../phase35/components/data-state';
import type { AISettingsApi, KnowledgeBaseItem } from './ai-settings-api';

export function KnowledgeBasePage({ api }: { api: AISettingsApi }) {
  const corpId = useOptionalDashboardAccess()?.corp.id;
  const queryClient = useQueryClient();
  const [editing, setEditing] = useState<KnowledgeBaseItem | null>(null);
  const [creating, setCreating] = useState(false);
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [documentCount, setDocumentCount] = useState('0');
  const [status, setStatus] = useState(1);
  const [error, setError] = useState('');

  const query = useQuery({
    queryKey: ['ai-kb', corpId],
    queryFn: () => api.listKnowledgeBases(Number(corpId)),
    enabled: Boolean(corpId),
  });
  const items = query.data ?? [];
  const total = items.length;
  const enabledCount = items.filter((item) => item.status === 1).length;
  const documents = items.reduce((sum, item) => sum + Number(item.documentCount ?? 0), 0);

  const save = useMutation({
    mutationFn: () => editing
      ? api.updateKnowledgeBase(Number(corpId), editing.id, { name: name.trim(), description: description.trim(), documentCount: Number(documentCount) || 0, status })
      : api.createKnowledgeBase(Number(corpId), { name: name.trim(), description: description.trim(), documentCount: Number(documentCount) || 0, status }),
    onSuccess: () => {
      setEditing(null); setCreating(false); setName(''); setDescription(''); setDocumentCount('0'); setStatus(1); setError('');
      void queryClient.invalidateQueries({ queryKey: ['ai-kb'] });
    },
    onError: (err) => setError(err instanceof Error ? err.message : '保存失败'),
  });
  const remove = useMutation({
    mutationFn: (id: string) => api.deleteKnowledgeBase(Number(corpId), id),
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: ['ai-kb'] }),
  });

  const openCreate = () => { setCreating(true); setEditing(null); setName(''); setDescription(''); setDocumentCount('0'); setStatus(1); setError(''); };
  const openEdit = (item: KnowledgeBaseItem) => {
    setEditing(item); setCreating(false); setName(item.name); setDescription(item.description);
    setDocumentCount(String(item.documentCount ?? 0)); setStatus(item.status); setError('');
  };
  const valid = Boolean(corpId && name.trim().length >= 2 && name.trim().length <= 128);

  return (
    <Phase35PageShell title="AI 知识库" description="维护企业知识库：名称、说明、文档数量与启停状态" actions={<button type="button" onClick={openCreate}>新建知识库</button>}>
      <div className="phase35-page">
        <section className="phase35-kpis" aria-label="知识库指标">
          <article className="phase35-kpi phase35-kpi-primary"><span>知识库总数</span><strong>{total}</strong><small>当前企业内已创建的知识库</small></article>
          <article className="phase35-kpi phase35-kpi-green"><span>已启用</span><strong>{enabledCount}</strong><small>状态为启用的知识库</small></article>
          <article className="phase35-kpi phase35-kpi-violet"><span>文档数合计</span><strong>{documents}</strong><small>全部知识库登记文档数量之和</small></article>
        </section>

        <section className="phase35-card phase35-table-card">
          <header className="phase35-card-header"><div><h2>知识库列表</h2><p>创建、编辑与停用知识库</p></div><span className="phase35-chip">共 {total} 条</span></header>
          <Phase35DataState
            loading={query.isLoading}
            error={query.isError}
            empty={!total}
            emptyContent={<p className="phase35-empty">暂无知识库，点击“新建知识库”开始创建</p>}
            onRetry={() => void query.refetch()}
          >
            <div className="phase35-table">
              <table>
                <thead><tr><th>名称</th><th>说明</th><th>文档数</th><th>状态</th><th>更新时间</th><th>操作</th></tr></thead>
                <tbody>
                  {items.map((item) => (
                    <tr key={item.id}>
                      <td>{item.name}</td><td>{item.description || '—'}</td><td>{item.documentCount}</td>
                      <td>{item.status === 1 ? '启用' : '停用'}</td><td>{item.updatedAt}</td>
                      <td>
                        <button type="button" onClick={() => openEdit(item)}>编辑</button>
                        <ConfirmAction title={`确认删除知识库“${item.name}”？`} onConfirm={() => remove.mutate(item.id)}><button type="button">删除</button></ConfirmAction>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </Phase35DataState>
        </section>

        {(creating || editing) && (
          <section className="phase35-card" role="dialog" aria-label="知识库表单">
            <header className="phase35-card-header"><div><h2>{editing ? '编辑知识库' : '新建知识库'}</h2></div></header>
            {error && <p role="alert" className="phase35-limits">{error}</p>}
            <form onSubmit={(event) => { event.preventDefault(); if (valid) save.mutate(); }}>
              <label>名称<input value={name} onChange={(event) => setName(event.target.value)} placeholder="如：售后话术库" /></label>
              <label>说明<input value={description} onChange={(event) => setDescription(event.target.value)} /></label>
              <label>文档数<input type="number" min="0" value={documentCount} onChange={(event) => setDocumentCount(event.target.value)} /></label>
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
