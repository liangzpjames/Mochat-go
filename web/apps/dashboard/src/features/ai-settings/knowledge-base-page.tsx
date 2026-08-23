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
import type { AISettingsApi, KnowledgeBaseItem } from './ai-settings-api';
import { filterAndPageAISettings, formatAISettingsTime, parseAISettingsListState } from './list-state';

function operationError(error: unknown): string {
  if (error instanceof ApiError) {
    const messages: Record<string, string> = {
      AI_SETTINGS_KNOWLEDGE_BASE_REFERENCED: '该知识库仍被智能体引用，请先解除关联后再删除。',
      AI_SETTINGS_NOT_FOUND: '记录不存在或已被删除，请刷新列表后重试。',
      AI_SETTINGS_STATUS_INVALID: '状态值无效，请重新选择。',
      AI_SETTINGS_NAME_REQUIRED: '请输入名称。',
      AI_SETTINGS_NAME_INVALID: '名称需为 2–128 个字符。',
      AI_SETTINGS_DESCRIPTION_INVALID: '说明内容过长，请精简后重试。',
      AI_SETTINGS_DOCUMENT_COUNT_INVALID: '登记文档数必须是非负整数。',
      AI_SETTINGS_STORAGE_FAILURE: '服务暂时不可用，请稍后重试。',
    };
    return messages[error.machineCode ?? ''] ?? '操作失败，请稍后重试。';
  }
  return '操作失败，请稍后重试。';
}

function textLength(value: string): number { return Array.from(value).length; }
const maxDocumentCount = 2_147_483_647;

export function KnowledgeBasePage({ api }: { api: AISettingsApi }) {
  const corpId = useOptionalDashboardAccess()?.corp.id;
  const queryClient = useQueryClient();
  const [searchParams, setSearchParams] = useSearchParams();
  const listState = parseAISettingsListState(searchParams);
  const [draftKeyword, setDraftKeyword] = useState(listState.q);
  const [draftStatus, setDraftStatus] = useState(listState.status);
  const [editing, setEditing] = useState<KnowledgeBaseItem | null>(null);
  const [creating, setCreating] = useState(false);
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [documentCount, setDocumentCount] = useState('0');
  const [status, setStatus] = useState(1);
  const [editorError, setEditorError] = useState('');
  const [feedback, setFeedback] = useState<{ kind: 'error' | 'success'; text: string } | null>(null);
  const [discardOpen, setDiscardOpen] = useState(false);
  const editorTriggerRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    setDraftKeyword(listState.q);
    setDraftStatus(listState.status);
  }, [listState.q, listState.status]);

  const query = useQuery({ queryKey: ['ai-kb', corpId], queryFn: () => api.listKnowledgeBases(Number(corpId)), enabled: Boolean(corpId) });
  const items = query.data ?? [];
  const page = filterAndPageAISettings(items, listState);
  useEffect(() => {
    if (!query.isSuccess || page.page === listState.page) return;
    setSearchParams(updateSearch(searchParams, { page: page.page, pageSize: page.pageSize }), { replace: true });
  }, [listState.page, page.page, page.pageSize, query.isSuccess, searchParams, setSearchParams]);
  const total = items.length;
  const enabledCount = items.filter((item) => item.status === 1).length;
  const documents = items.reduce((sum, item) => sum + Number(item.documentCount ?? 0), 0);
  const editorOpen = creating || editing !== null;
  const initialEditor = editing
    ? { name: editing.name, description: editing.description, documentCount: String(editing.documentCount ?? 0), status: editing.status }
    : { name: '', description: '', documentCount: '0', status: 1 };
  const dirty = editorOpen && (name !== initialEditor.name || description !== initialEditor.description || documentCount !== initialEditor.documentCount || status !== initialEditor.status);

  function submitFilters() {
    setSearchParams(updateSearch(searchParams, { q: draftKeyword.trim() || undefined, status: draftStatus === 'all' ? undefined : draftStatus, page: 1, pageSize: listState.pageSize }));
  }

  function resetFilters() {
    setDraftKeyword('');
    setDraftStatus('all');
    setSearchParams(updateSearch(searchParams, { q: undefined, status: undefined, page: 1, pageSize: listState.pageSize }));
  }

  function resetEditor() {
    setEditing(null); setCreating(false); setName(''); setDescription(''); setDocumentCount('0'); setStatus(1); setEditorError(''); setDiscardOpen(false);
  }

  function requestEditorClose() {
    if (save.isPending) return;
    if (dirty) { setDiscardOpen(true); return; }
    resetEditor();
  }

  function rememberTrigger(event: MouseEvent<HTMLButtonElement>) { editorTriggerRef.current = event.currentTarget; }
  function openCreate(event: MouseEvent<HTMLButtonElement>) {
    rememberTrigger(event); setCreating(true); setEditing(null); setName(''); setDescription(''); setDocumentCount('0'); setStatus(1); setEditorError(''); setFeedback(null);
  }
  function openEdit(item: KnowledgeBaseItem, event: MouseEvent<HTMLButtonElement>) {
    rememberTrigger(event); setEditing(item); setCreating(false); setName(item.name); setDescription(item.description); setDocumentCount(String(item.documentCount ?? 0)); setStatus(item.status); setEditorError(''); setFeedback(null);
  }

  const save = useMutation({
    mutationFn: () => editing
      ? api.updateKnowledgeBase(Number(corpId), editing.id, { name: name.trim(), description: description.trim(), documentCount: Number(documentCount), status })
      : api.createKnowledgeBase(Number(corpId), { name: name.trim(), description: description.trim(), documentCount: Number(documentCount), status }),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['ai-kb', corpId] });
      setFeedback({ kind: 'success', text: editing ? '知识库已更新。' : '知识库已创建。' });
      resetEditor();
    },
    onError: (error) => setEditorError(operationError(error)),
  });
  const remove = useMutation({
    mutationFn: (item: KnowledgeBaseItem) => api.deleteKnowledgeBase(Number(corpId), item.id).then(() => item),
    onMutate: () => setFeedback(null),
    onSuccess: async (item) => { await queryClient.invalidateQueries({ queryKey: ['ai-kb', corpId] }); setFeedback({ kind: 'success', text: `知识库“${item.name}”已删除。` }); },
    onError: (error) => setFeedback({ kind: 'error', text: operationError(error) }),
  });
  const toggle = useMutation({
    mutationFn: (item: KnowledgeBaseItem) => api.updateKnowledgeBase(Number(corpId), item.id, { name: item.name, description: item.description, documentCount: item.documentCount, status: item.status === 1 ? 0 : 1 }).then(() => item),
    onMutate: () => setFeedback(null),
    onSuccess: async (item) => { await queryClient.invalidateQueries({ queryKey: ['ai-kb', corpId] }); setFeedback({ kind: 'success', text: `知识库“${item.name}”已${item.status === 1 ? '停用' : '启用'}。` }); },
    onError: (error) => setFeedback({ kind: 'error', text: operationError(error) }),
  });

  const documentCountValid = /^\d+$/.test(documentCount) && Number.isSafeInteger(Number(documentCount)) && Number(documentCount) <= maxDocumentCount;
  const valid = Boolean(corpId && textLength(name.trim()) >= 2 && textLength(name.trim()) <= 128 && textLength(description) <= 512 && documentCountValid);
  const filteredEmpty = total > 0 && page.total === 0;

  return (
    <Phase35PageShell title="AI 知识库" description="维护可供未来 AI 能力消费的知识库元数据" actions={<button type="button" onClick={openCreate}>新建知识库</button>}>
      <div className="phase35-page ai-settings-workspace">
        <p className="ai-settings-capability-notice" role="note">当前仅保存知识库配置元数据；文档上传、解析与检索能力尚未接入。文档数量均为人工登记值，不代表内容已可检索。</p>
        <section className="phase35-kpis" aria-label="知识库指标">
          <article className="phase35-kpi phase35-kpi-primary"><span>知识库总数</span><strong>{total}</strong><small>当前企业真实记录</small></article>
          <article className="phase35-kpi phase35-kpi-green"><span>已启用</span><strong>{enabledCount}</strong><small>仅表示配置状态</small></article>
          <article className="phase35-kpi phase35-kpi-violet"><span>登记文档数合计</span><strong>{documents}</strong><small>人工登记值，未接入内容链路</small></article>
        </section>
        <DashboardFilterPanel className="ai-settings-filter" onSubmit={submitFilters} onReset={resetFilters} pending={query.isFetching} extraActions={<button type="button" onClick={() => { setFeedback(null); void query.refetch(); }} disabled={query.isFetching}>刷新</button>}>
          <label>关键词<input aria-label="关键词" value={draftKeyword} onChange={(event) => setDraftKeyword(event.target.value)} placeholder="搜索名称或说明" /></label>
          <label>状态<select aria-label="状态" value={draftStatus} onChange={(event) => setDraftStatus(event.target.value as typeof draftStatus)}><option value="all">全部状态</option><option value="enabled">启用</option><option value="disabled">停用</option></select></label>
        </DashboardFilterPanel>
        {feedback && <p className={`ai-settings-feedback ai-settings-feedback--${feedback.kind}`} role={feedback.kind === 'error' ? 'alert' : 'status'}>{feedback.text}</p>}
        <section className="phase35-card phase35-table-card">
          <header className="phase35-card-header"><div><h2>知识库列表</h2><p>新增、编辑、显式启停与删除企业内知识库</p></div><span className="phase35-chip">筛选 {page.total} / 全部 {total} 条</span></header>
          <Phase35DataState loading={query.isLoading} error={query.isError} empty={!total} emptyContent={<p className="phase35-empty">暂无知识库，点击“新建知识库”开始创建</p>} onRetry={() => void query.refetch()}>
            {filteredEmpty ? <p className="phase35-empty">当前筛选条件下没有匹配的知识库</p> : <>
              <div className="phase35-table ai-settings-table-scroll"><table>
                <thead><tr><th>名称</th><th>说明</th><th>登记文档数</th><th>配置状态</th><th>更新时间</th><th>操作</th></tr></thead>
                <tbody>{page.items.map((item) => {
                  const rowPending = (remove.isPending && remove.variables?.id === item.id) || (toggle.isPending && toggle.variables?.id === item.id);
                  return <tr key={item.id}><td>{item.name}</td><td>{item.description || '—'}</td><td>{item.documentCount}</td><td><span className={`ai-settings-status ai-settings-status--${item.status === 1 ? 'enabled' : 'disabled'}`}>{item.status === 1 ? '启用' : '停用'}</span></td><td>{formatAISettingsTime(item.updatedAt)}</td><td><div className="ai-settings-row-actions">
                    <button type="button" aria-label={`编辑 ${item.name}`} disabled={rowPending} onClick={(event) => openEdit(item, event)}>编辑</button>
                    <ConfirmAction danger={item.status === 1} title={`确认${item.status === 1 ? '停用' : '启用'}知识库“${item.name}”？`} onConfirm={() => toggle.mutateAsync(item).catch(() => undefined)}><button type="button" aria-label={`${item.status === 1 ? '停用' : '启用'} ${item.name}`} disabled={rowPending}>{item.status === 1 ? '停用' : '启用'}</button></ConfirmAction>
                    <ConfirmAction title={`确认删除知识库“${item.name}”？`} description="删除后无法恢复；被智能体引用时服务端会拒绝删除。" onConfirm={() => remove.mutateAsync(item).catch(() => undefined)}><button type="button" aria-label={`删除 ${item.name}`} disabled={rowPending}>删除</button></ConfirmAction>
                  </div></td></tr>;
                })}</tbody>
              </table></div>
              <DashboardPagination ariaLabel="知识库分页" page={page.page} pageSize={page.pageSize} total={page.total} pageSizeOptions={[10, 20, 50]} onPageChange={(nextPage) => setSearchParams(updateSearch(searchParams, { page: nextPage, pageSize: page.pageSize }))} onPageSizeChange={(nextPageSize) => setSearchParams(updateSearch(searchParams, { page: 1, pageSize: nextPageSize }))} />
            </>}
          </Phase35DataState>
        </section>
        <DashboardDialog open={editorOpen} title={editing ? '编辑知识库' : '新建知识库'} triggerRef={editorTriggerRef} confirmDisabled={!valid} confirmLoading={save.isPending} onCancel={requestEditorClose} onConfirm={() => save.mutate()}>
          <div className="ai-settings-dialog-body">{editorError && <p role="alert" className="ai-settings-feedback ai-settings-feedback--error">{editorError}</p>}<form onSubmit={(event) => { event.preventDefault(); if (valid && !save.isPending) save.mutate(); }}>
            <label>名称<input value={name} onChange={(event) => setName(event.target.value)} placeholder="如：售后话术库" /></label>
            <label>说明<textarea value={description} onChange={(event) => setDescription(event.target.value)} rows={3} /></label>
            <label>登记文档数<input type="number" min="0" max={maxDocumentCount} step="1" value={documentCount} onChange={(event) => setDocumentCount(event.target.value)} /><small>仅为登记值，不会上传或索引文档。</small></label>
            <label>配置状态<select value={status} onChange={(event) => setStatus(Number(event.target.value))}><option value={1}>启用</option><option value={0}>停用</option></select><small>启用不代表 AI 运行能力已接入。</small></label>
          </form></div>
        </DashboardDialog>
        <DashboardDialog open={discardOpen} title="放弃未保存更改？" danger confirmText="放弃更改" cancelText="继续编辑" onCancel={() => setDiscardOpen(false)} onConfirm={resetEditor}><p>当前修改尚未保存，放弃后无法恢复。</p></DashboardDialog>
      </div>
    </Phase35PageShell>
  );
}
