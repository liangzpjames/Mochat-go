import { ApiError } from '@mochat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useRef, useState } from 'react';
import type { MouseEvent } from 'react';
import { useSearchParams } from 'react-router';
import { useOptionalDashboardAccess } from '../../app/access-context';
import { ConfirmAction } from '../../components/confirm-action';
import { DashboardDialog } from '../../components/dashboard-dialog';
import { DashboardFilterPanel } from '../../components/dashboard-filter-panel';
import { DashboardPagination } from '../../components/dashboard-pagination';
import { updateSearch } from '../../shared/query-state';
import { Phase35PageShell } from '../phase35/components/phase35-page-shell';
import { Phase35DataState } from '../phase35/components/data-state';
import type { AgentItem, AISettingsApi } from './ai-settings-api';
import { filterAndPageAISettings, formatAISettingsTime, parseAISettingsListState, resolveKnowledgeBaseNames } from './list-state';

function operationError(error: unknown): string {
  if (error instanceof ApiError) {
    const messages: Record<string, string> = {
      AI_SETTINGS_KNOWLEDGE_BASE_INVALID: '关联知识库已失效或不属于当前企业，请刷新后重新选择。',
      AI_SETTINGS_NOT_FOUND: '记录不存在或已被删除，请刷新列表后重试。',
      AI_SETTINGS_STATUS_INVALID: '状态值无效，请重新选择。',
      AI_SETTINGS_NAME_REQUIRED: '请输入名称。',
      AI_SETTINGS_NAME_INVALID: '名称需为 2–128 个字符。',
      AI_SETTINGS_DESCRIPTION_INVALID: '说明内容过长，请精简后重试。',
      AI_SETTINGS_INVALID_JSON: '请求格式无效，请刷新后重试。',
      AI_SETTINGS_STORAGE_FAILURE: '服务暂时不可用，请稍后重试。',
    };
    return messages[error.machineCode ?? ''] ?? '操作失败，请稍后重试。';
  }
  return '操作失败，请稍后重试。';
}

function textLength(value: string): number { return Array.from(value).length; }

export function AgentPage({ api }: { api: AISettingsApi }) {
  const corpId = useOptionalDashboardAccess()?.corp.id;
  const queryClient = useQueryClient();
  const [searchParams, setSearchParams] = useSearchParams();
  const listState = parseAISettingsListState(searchParams);
  const [draftKeyword, setDraftKeyword] = useState(listState.q);
  const [draftStatus, setDraftStatus] = useState(listState.status);
  const [editing, setEditing] = useState<AgentItem | null>(null);
  const [creating, setCreating] = useState(false);
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [selectedBases, setSelectedBases] = useState<string[]>([]);
  const [status, setStatus] = useState(1);
  const [editorError, setEditorError] = useState('');
  const [feedback, setFeedback] = useState<{ kind: 'error' | 'success'; text: string } | null>(null);
  const [discardOpen, setDiscardOpen] = useState(false);
  const editorTriggerRef = useRef<HTMLButtonElement>(null);

  useEffect(() => { setDraftKeyword(listState.q); setDraftStatus(listState.status); }, [listState.q, listState.status]);

  const query = useQuery({ queryKey: ['ai-agents', corpId], queryFn: () => api.listAgents(Number(corpId)), enabled: Boolean(corpId) });
  const knowledgeBases = useQuery({ queryKey: ['ai-kb', corpId], queryFn: () => api.listKnowledgeBases(Number(corpId)), enabled: Boolean(corpId) });
  const items = query.data ?? [];
  const page = filterAndPageAISettings(items, listState);
  useEffect(() => {
    const changes: Record<string, string | number> = {};
    const resolvedPage = query.isSuccess ? page.page : listState.page;
    const rawPage = searchParams.get('page');
    const rawPageSize = searchParams.get('pageSize');
    const rawStatus = searchParams.get('status');
    if (rawPage !== null && rawPage !== String(resolvedPage)) changes.page = resolvedPage;
    if (rawPageSize !== null && rawPageSize !== String(listState.pageSize)) changes.pageSize = listState.pageSize;
    if (rawStatus !== null && rawStatus !== listState.status) changes.status = listState.status;
    if (Object.keys(changes).length === 0) return;
    setSearchParams(updateSearch(searchParams, changes), { replace: true });
  }, [listState.page, listState.pageSize, listState.status, page.page, query.isSuccess, searchParams, setSearchParams]);
  const total = items.length;
  const enabledCount = items.filter((item) => item.status === 1).length;
  const activeBases = (knowledgeBases.data ?? []).filter((item) => item.status === 1);
  const metricsHaveData = query.data !== undefined;
  const metricsNote = query.isError
    ? (metricsHaveData ? '上次成功数据，刷新失败' : '数据加载失败')
    : (!metricsHaveData ? '正在加载' : null);
  const basesHaveData = knowledgeBases.data !== undefined;
  const basesMetricsNote = knowledgeBases.isError
    ? (basesHaveData ? '上次成功数据，刷新失败' : '知识库加载失败')
    : (!basesHaveData ? '正在加载' : null);
  const initialBases = editing?.knowledgeBaseIds ?? [];
  const editorOpen = creating || editing !== null;
  const dirty = editorOpen && (name !== (editing?.name ?? '') || description !== (editing?.description ?? '') || status !== (editing?.status ?? 1) || JSON.stringify(selectedBases) !== JSON.stringify(initialBases));

  function submitFilters() {
    setSearchParams(updateSearch(searchParams, { q: draftKeyword.trim() || undefined, status: draftStatus === 'all' ? undefined : draftStatus, page: 1, pageSize: listState.pageSize }));
  }
  function resetFilters() {
    setDraftKeyword(''); setDraftStatus('all'); setSearchParams(updateSearch(searchParams, { q: undefined, status: undefined, page: 1, pageSize: listState.pageSize }));
  }
  function resetEditor() {
    setEditing(null); setCreating(false); setName(''); setDescription(''); setSelectedBases([]); setStatus(1); setEditorError(''); setDiscardOpen(false);
  }
  function requestEditorClose() {
    if (save.isPending) return;
    if (dirty) { setDiscardOpen(true); return; }
    resetEditor();
  }
  function rememberTrigger(event: MouseEvent<HTMLButtonElement>) { editorTriggerRef.current = event.currentTarget; }
  function openCreate(event: MouseEvent<HTMLButtonElement>) {
    rememberTrigger(event); setCreating(true); setEditing(null); setName(''); setDescription(''); setSelectedBases([]); setStatus(1); setEditorError(''); setFeedback(null);
  }
  function openEdit(item: AgentItem, event: MouseEvent<HTMLButtonElement>) {
    rememberTrigger(event); setEditing(item); setCreating(false); setName(item.name); setDescription(item.description); setSelectedBases(item.knowledgeBaseIds ?? []); setStatus(item.status); setEditorError(''); setFeedback(null);
  }
  function toggleBase(id: string) { setSelectedBases((current) => current.includes(id) ? current.filter((item) => item !== id) : [...current, id]); }

  const save = useMutation({
    mutationFn: () => editing
      ? api.updateAgent(Number(corpId), editing.id, { name: name.trim(), description: description.trim(), knowledgeBaseIds: selectedBases, status })
      : api.createAgent(Number(corpId), { name: name.trim(), description: description.trim(), knowledgeBaseIds: selectedBases, status }),
    onSuccess: async () => { await queryClient.invalidateQueries({ queryKey: ['ai-agents', corpId] }); setFeedback({ kind: 'success', text: editing ? '智能体已更新。' : '智能体已创建。' }); resetEditor(); },
    onError: (error) => setEditorError(operationError(error)),
  });
  const remove = useMutation({
    mutationFn: (item: AgentItem) => api.deleteAgent(Number(corpId), item.id).then(() => item),
    onMutate: () => setFeedback(null),
    onSuccess: async (item) => { await queryClient.invalidateQueries({ queryKey: ['ai-agents', corpId] }); setFeedback({ kind: 'success', text: `智能体“${item.name}”已删除。` }); },
    onError: (error) => setFeedback({ kind: 'error', text: operationError(error) }),
  });
  const toggle = useMutation({
    mutationFn: (item: AgentItem) => api.updateAgent(Number(corpId), item.id, { name: item.name, description: item.description, knowledgeBaseIds: item.knowledgeBaseIds, status: item.status === 1 ? 0 : 1 }).then(() => item),
    onMutate: () => setFeedback(null),
    onSuccess: async (item) => { await queryClient.invalidateQueries({ queryKey: ['ai-agents', corpId] }); setFeedback({ kind: 'success', text: `智能体“${item.name}”已${item.status === 1 ? '停用' : '启用'}。` }); },
    onError: (error) => setFeedback({ kind: 'error', text: operationError(error) }),
  });

  const valid = Boolean(corpId && textLength(name.trim()) >= 2 && textLength(name.trim()) <= 128 && textLength(description) <= 512 && knowledgeBases.isSuccess);
  const filteredEmpty = total > 0 && page.total === 0;
  const listedBaseIds = new Set((knowledgeBases.data ?? []).map((item) => item.id));
  const invalidOriginalBases = initialBases.filter((id) => !listedBaseIds.has(id));

  return (
    <Phase35PageShell title="智能体管理" description="维护智能体配置元数据与企业内知识库关联" actions={<button type="button" onClick={openCreate}>新建智能体</button>}>
      <div className="phase35-page ai-settings-workspace">
        <p className="ai-settings-capability-notice" role="note">当前仅保存智能体配置元数据；模型、提示词、工具、发布与真实运行能力尚未接入。启用不代表智能体已可对外服务。</p>
        <section className="phase35-kpis" aria-label="智能体指标">
          <article className="phase35-kpi phase35-kpi-primary"><span>智能体总数</span><strong>{metricsHaveData ? total : '—'}</strong><small>{metricsNote ?? '当前企业真实记录'}</small></article>
          <article className="phase35-kpi phase35-kpi-green"><span>已启用</span><strong>{metricsHaveData ? enabledCount : '—'}</strong><small>{metricsNote ?? '仅表示配置状态'}</small></article>
          <article className="phase35-kpi phase35-kpi-violet"><span>可新关联知识库</span><strong>{basesHaveData ? activeBases.length : '—'}</strong><small>{basesMetricsNote ?? '仅统计已启用知识库'}</small></article>
        </section>
        <DashboardFilterPanel className="ai-settings-filter" onSubmit={submitFilters} onReset={resetFilters} pending={query.isFetching} extraActions={<button type="button" onClick={() => { setFeedback(null); void Promise.all([query.refetch(), knowledgeBases.refetch()]); }} disabled={query.isFetching || knowledgeBases.isFetching}>刷新</button>}>
          <label>关键词<input aria-label="关键词" value={draftKeyword} onChange={(event) => setDraftKeyword(event.target.value)} placeholder="搜索名称或说明" /></label>
          <label>状态<select aria-label="状态" value={draftStatus} onChange={(event) => setDraftStatus(event.target.value as typeof draftStatus)}><option value="all">全部状态</option><option value="enabled">启用</option><option value="disabled">停用</option></select></label>
        </DashboardFilterPanel>
        {feedback && <p className={`ai-settings-feedback ai-settings-feedback--${feedback.kind}`} role={feedback.kind === 'error' ? 'alert' : 'status'}>{feedback.text}</p>}
        <section className="phase35-card phase35-table-card">
          <header className="phase35-card-header"><div><h2>智能体列表</h2><p>真实展示知识库关联，并提供显式启停与删除</p></div><span className="phase35-chip">筛选 {metricsHaveData ? page.total : '—'} / 全部 {metricsHaveData ? total : '—'} 条</span></header>
          <Phase35DataState loading={query.isLoading} error={query.isError} empty={!total} emptyContent={<p className="phase35-empty">暂无智能体，点击“新建智能体”开始创建</p>} onRetry={() => void query.refetch()}>
            {filteredEmpty ? <p className="phase35-empty">当前筛选条件下没有匹配的智能体</p> : <>
              <div className="phase35-table ai-settings-table-scroll"><table>
                <thead><tr><th>名称</th><th>说明</th><th>关联知识库</th><th>配置状态</th><th>更新时间</th><th>操作</th></tr></thead>
                <tbody>{page.items.map((item) => {
                  const rowPending = (remove.isPending && remove.variables?.id === item.id) || (toggle.isPending && toggle.variables?.id === item.id);
                  return <tr key={item.id}><td>{item.name}</td><td>{item.description || '—'}</td><td>{knowledgeBases.isLoading ? '正在加载知识库名称…' : knowledgeBases.isError ? '知识库名称加载失败' : resolveKnowledgeBaseNames(item.knowledgeBaseIds ?? [], knowledgeBases.data ?? [])}</td><td><span className={`ai-settings-status ai-settings-status--${item.status === 1 ? 'enabled' : 'disabled'}`}>{item.status === 1 ? '启用' : '停用'}</span></td><td>{formatAISettingsTime(item.updatedAt)}</td><td><div className="ai-settings-row-actions">
                    <button type="button" aria-label={`编辑 ${item.name}`} disabled={rowPending} onClick={(event) => openEdit(item, event)}>编辑</button>
                    <ConfirmAction danger={item.status === 1} title={`确认${item.status === 1 ? '停用' : '启用'}智能体“${item.name}”？`} onConfirm={() => toggle.mutateAsync(item).catch(() => undefined)}><button type="button" aria-label={`${item.status === 1 ? '停用' : '启用'} ${item.name}`} disabled={rowPending}>{item.status === 1 ? '停用' : '启用'}</button></ConfirmAction>
                    <ConfirmAction title={`确认删除智能体“${item.name}”？`} description="删除后无法恢复。" onConfirm={() => remove.mutateAsync(item).catch(() => undefined)}><button type="button" aria-label={`删除 ${item.name}`} disabled={rowPending}>删除</button></ConfirmAction>
                  </div></td></tr>;
                })}</tbody>
              </table></div>
              <DashboardPagination ariaLabel="智能体分页" page={page.page} pageSize={page.pageSize} total={page.total} pageSizeOptions={[10, 20, 50]} onPageChange={(nextPage) => setSearchParams(updateSearch(searchParams, { page: nextPage, pageSize: page.pageSize }))} onPageSizeChange={(nextPageSize) => setSearchParams(updateSearch(searchParams, { page: 1, pageSize: nextPageSize }))} />
            </>}
          </Phase35DataState>
        </section>
        <DashboardDialog open={editorOpen} title={editing ? '编辑智能体' : '新建智能体'} triggerRef={editorTriggerRef} confirmDisabled={!valid} confirmLoading={save.isPending} onCancel={requestEditorClose} onConfirm={() => save.mutate()}>
          <div className="ai-settings-dialog-body">{editorError && <p role="alert" className="ai-settings-feedback ai-settings-feedback--error">{editorError}</p>}<form onSubmit={(event) => { event.preventDefault(); if (valid && !save.isPending) save.mutate(); }}>
            <label>名称<input value={name} onChange={(event) => setName(event.target.value)} placeholder="如：智能客服" /></label>
            <label>说明<textarea value={description} onChange={(event) => setDescription(event.target.value)} rows={3} /></label>
            <fieldset className="ai-settings-base-options"><legend>关联知识库</legend>
              {knowledgeBases.isLoading && <p role="status">正在加载可关联知识库…</p>}
              {knowledgeBases.isError && <p role="alert">知识库加载失败，暂时无法安全保存关联配置。请刷新后重试。</p>}
              {knowledgeBases.isSuccess && activeBases.length === 0 && initialBases.length === 0 && <p className="phase35-empty">暂无已启用知识库可选。<a href="/ai-setting/ai-knowledge-base">前往 AI 知识库</a></p>}
              {(knowledgeBases.data ?? []).map((kb) => {
                const previouslyRelated = initialBases.includes(kb.id);
                const selectable = kb.status === 1 || previouslyRelated;
                return <label key={kb.id} className={!selectable ? 'ai-settings-base-option--disabled' : undefined}><input type="checkbox" checked={selectedBases.includes(kb.id)} disabled={!selectable} onChange={() => toggleBase(kb.id)} />{kb.name}<small>{kb.status === 1 ? '已启用' : previouslyRelated ? '已停用，仅可保留既有关联或移除' : '已停用，不可新关联'}</small></label>;
              })}
              {invalidOriginalBases.map((id) => <label key={id} className="ai-settings-base-option--invalid"><input type="checkbox" checked={selectedBases.includes(id)} disabled={!selectedBases.includes(id)} onChange={() => toggleBase(id)} />已失效（ID: {id}）<small>仅可从既有关联中移除</small></label>)}
            </fieldset>
            <label>配置状态<select value={status} onChange={(event) => setStatus(Number(event.target.value))}><option value={1}>启用</option><option value={0}>停用</option></select><small>启用不代表模型与运行能力已接入。</small></label>
          </form></div>
        </DashboardDialog>
        <DashboardDialog open={discardOpen} title="放弃未保存更改？" danger confirmText="放弃更改" cancelText="继续编辑" onCancel={() => setDiscardOpen(false)} onConfirm={resetEditor}><p>当前修改尚未保存，放弃后无法恢复。</p></DashboardDialog>
      </div>
    </Phase35PageShell>
  );
}
