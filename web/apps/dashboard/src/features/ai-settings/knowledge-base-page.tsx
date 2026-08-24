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
import type { AISettingsApi, KnowledgeBaseItem, KnowledgeDocumentItem } from './ai-settings-api';
import { filterAndPageAISettings, formatAISettingsTime, parseAISettingsListState } from './list-state';

function operationError(error: unknown): string {
  if (error instanceof ApiError) {
    if (error.machineCode === 'AI_SETTINGS_KNOWLEDGE_BASE_REFERENCED') {
      const data = error.data;
      const referenceCount = typeof data === 'object' && data !== null && 'referenceCount' in data
        ? (data as { referenceCount?: unknown }).referenceCount
        : undefined;
      if (typeof referenceCount === 'number' && Number.isSafeInteger(referenceCount) && referenceCount > 0) {
        return `该知识库仍被 ${referenceCount} 个智能体引用，请先解除关联后再删除。`;
      }
      return '该知识库仍被智能体引用，请先解除关联后再删除。';
    }
    if (error.machineCode === 'AI_SETTINGS_KNOWLEDGE_BASE_HAS_DOCUMENTS') {
      return '该知识库仍包含文档，请先在“管理文档”中删除全部文档。';
    }
    const messages: Record<string, string> = {
      AI_SETTINGS_NOT_FOUND: '记录不存在或已被删除，请刷新列表后重试。',
      AI_SETTINGS_STATUS_INVALID: '状态值无效，请重新选择。',
      AI_SETTINGS_NAME_REQUIRED: '请输入名称。',
      AI_SETTINGS_NAME_INVALID: '名称需为 2–128 个字符。',
      AI_SETTINGS_DESCRIPTION_INVALID: '说明内容过长，请精简后重试。',
      AI_SETTINGS_DOCUMENT_COUNT_INVALID: '登记文档数必须是非负整数。',
      AI_SETTINGS_DOCUMENT_DUPLICATE: '该文件内容已存在，无需重复上传。',
      AI_SETTINGS_DOCUMENT_LIMIT: '单个知识库最多保存 100 个文档。',
      AI_SETTINGS_DOCUMENT_TOO_LARGE: '文件不能超过 20 MB。',
      AI_SETTINGS_DOCUMENT_TEXT_TOO_LARGE: '文档文本过长，暂不支持解析。',
      AI_SETTINGS_DOCUMENT_TYPE_UNSUPPORTED: '仅支持 TXT、Markdown、PDF 和 DOCX 文件。',
      AI_SETTINGS_DOCUMENT_UNREADABLE: '无法读取该文档，请确认文件未加密且包含可提取文字。',
      AI_SETTINGS_INVALID_JSON: '请求格式无效，请刷新后重试。',
      AI_SETTINGS_STORAGE_FAILURE: '服务暂时不可用，请稍后重试。',
    };
    return messages[error.machineCode ?? ''] ?? '操作失败，请稍后重试。';
  }
  return '操作失败，请稍后重试。';
}

function textLength(value: string): number { return Array.from(value).length; }

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
}

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
  const [status, setStatus] = useState(1);
  const [createFile, setCreateFile] = useState<File | null>(null);
  const [managing, setManaging] = useState<KnowledgeBaseItem | null>(null);
  const [uploadFile, setUploadFile] = useState<File | null>(null);
  const documentTriggerRef = useRef<HTMLButtonElement>(null);
  const [editorError, setEditorError] = useState('');
  const [feedback, setFeedback] = useState<{ kind: 'error' | 'success'; text: string } | null>(null);
  const [discardOpen, setDiscardOpen] = useState(false);
  const editorTriggerRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    setDraftKeyword(listState.q);
    setDraftStatus(listState.status);
  }, [listState.q, listState.status]);

  const query = useQuery({ queryKey: ['ai-kb', corpId], queryFn: () => api.listKnowledgeBases(Number(corpId)), enabled: Boolean(corpId) });
  const documentQuery = useQuery({ queryKey: ['ai-kb-documents', corpId, managing?.id], queryFn: () => api.listKnowledgeDocuments(Number(corpId), String(managing?.id)), enabled: Boolean(corpId && managing) });
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
  const documents = items.reduce((sum, item) => sum + Number(item.documentCount ?? 0), 0);
  const metricsHaveData = query.data !== undefined;
  const metricsNote = query.isError
    ? (metricsHaveData ? '上次成功数据，刷新失败' : '数据加载失败')
    : (!metricsHaveData ? '正在加载' : null);
  const editorOpen = creating || editing !== null;
  const initialEditor = editing
    ? { name: editing.name, description: editing.description, status: editing.status }
    : { name: '', description: '', status: 1 };
  const dirty = editorOpen && (name !== initialEditor.name || description !== initialEditor.description || status !== initialEditor.status || (creating && createFile !== null));

  function submitFilters() {
    setSearchParams(updateSearch(searchParams, { q: draftKeyword.trim() || undefined, status: draftStatus === 'all' ? undefined : draftStatus, page: 1, pageSize: listState.pageSize }));
  }

  function resetFilters() {
    setDraftKeyword('');
    setDraftStatus('all');
    setSearchParams(updateSearch(searchParams, { q: undefined, status: undefined, page: 1, pageSize: listState.pageSize }));
  }

  function resetEditor() {
    setEditing(null); setCreating(false); setName(''); setDescription(''); setStatus(1); setCreateFile(null); setEditorError(''); setDiscardOpen(false);
  }

  function requestEditorClose() {
    if (save.isPending) return;
    if (dirty) { setDiscardOpen(true); return; }
    resetEditor();
  }

  function rememberTrigger(event: MouseEvent<HTMLButtonElement>) { editorTriggerRef.current = event.currentTarget; }
  function openCreate(event: MouseEvent<HTMLButtonElement>) {
    rememberTrigger(event); setCreating(true); setEditing(null); setName(''); setDescription(''); setStatus(1); setCreateFile(null); setEditorError(''); setFeedback(null);
  }
  function openEdit(item: KnowledgeBaseItem, event: MouseEvent<HTMLButtonElement>) {
    rememberTrigger(event); setEditing(item); setCreating(false); setName(item.name); setDescription(item.description); setStatus(item.status); setCreateFile(null); setEditorError(''); setFeedback(null);
  }

  const save = useMutation({
    mutationFn: async () => {
      if (editing) {
        const item = await api.updateKnowledgeBase(Number(corpId), editing.id, { name: name.trim(), description: description.trim(), documentCount: editing.documentCount, status });
        return { mode: 'updated' as const, item, document: null, uploadError: null };
      }
      const item = await api.createKnowledgeBase(Number(corpId), { name: name.trim(), description: description.trim(), documentCount: 0, status });
      if (!createFile) return { mode: 'created' as const, item, document: null, uploadError: null };
      try {
        const document = await api.uploadKnowledgeDocument(Number(corpId), item.id, createFile);
        return { mode: 'created' as const, item, document, uploadError: null };
      } catch (uploadError) {
        return { mode: 'created' as const, item, document: null, uploadError };
      }
    },
    onSuccess: async (result) => {
      await queryClient.invalidateQueries({ queryKey: ['ai-kb', corpId] });
      if (result.mode === 'updated') {
        setFeedback({ kind: 'success', text: '知识库已更新。' });
      } else if (result.uploadError) {
        setFeedback({ kind: 'error', text: `知识库“${result.item.name}”已创建，但初始文档上传失败：${operationError(result.uploadError)} 请在“管理文档”中重试。` });
      } else if (result.document?.status === 'failed') {
        setFeedback({ kind: 'error', text: `知识库“${result.item.name}”已创建，但文档“${result.document.filename}”解析失败，暂不能用于分析；请在“管理文档”中处理。` });
      } else if (result.document) {
        setFeedback({ kind: 'success', text: `知识库“${result.item.name}”已创建，文档“${result.document.filename}”已解析，可用于分析。` });
      } else {
        setFeedback({ kind: 'success', text: '知识库已创建。' });
      }
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
  const upload = useMutation({
    mutationFn: () => api.uploadKnowledgeDocument(Number(corpId), String(managing?.id), uploadFile as File),
    onSuccess: async (document) => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['ai-kb-documents', corpId, managing?.id] }),
        queryClient.invalidateQueries({ queryKey: ['ai-kb', corpId] }),
      ]);
      setUploadFile(null);
      setFeedback(document.status === 'ready'
        ? { kind: 'success', text: `文档“${document.filename}”已解析，可用于会话分析。` }
        : { kind: 'error', text: `文档“${document.filename}”已保存，但解析失败，暂不会用于分析。` });
    },
    onError: (error) => setFeedback({ kind: 'error', text: operationError(error) }),
  });
  const removeDocument = useMutation({
    mutationFn: (document: KnowledgeDocumentItem) => api.deleteKnowledgeDocument(Number(corpId), String(managing?.id), document.id).then(() => document),
    onSuccess: async (document) => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['ai-kb-documents', corpId, managing?.id] }),
        queryClient.invalidateQueries({ queryKey: ['ai-kb', corpId] }),
      ]);
      setFeedback({ kind: 'success', text: `文档“${document.filename}”已删除。` });
    },
    onError: (error) => setFeedback({ kind: 'error', text: operationError(error) }),
  });

  const valid = Boolean(corpId && textLength(name.trim()) >= 2 && textLength(name.trim()) <= 128 && textLength(description) <= 512);
  const filteredEmpty = total > 0 && page.total === 0;

  return (
    <Phase35PageShell title="AI 知识库" description="上传并管理会话分析助手可检索的企业文档" actions={<button type="button" onClick={openCreate}>新建知识库</button>}>
      <div className="phase35-page ai-settings-workspace">
        <p className="ai-settings-capability-notice" role="note">已启用且被“会话分析助手”关联的知识库，会在 AI 洞察生成会话分析时提供背景资料。知识不能替代真实会话消息作为分析证据。</p>
        <section className="phase35-kpis" aria-label="知识库指标">
          <article className="phase35-kpi phase35-kpi-primary"><span>知识库总数</span><strong>{metricsHaveData ? total : '—'}</strong><small>{metricsNote ?? '当前企业真实记录'}</small></article>
          <article className="phase35-kpi phase35-kpi-green"><span>已启用</span><strong>{metricsHaveData ? enabledCount : '—'}</strong><small>{metricsNote ?? '仅表示配置状态'}</small></article>
          <article className="phase35-kpi phase35-kpi-violet"><span>已上传文档</span><strong>{metricsHaveData ? documents : '—'}</strong><small>{metricsNote ?? '来自持久化文档记录'}</small></article>
        </section>
        <DashboardFilterPanel className="ai-settings-filter" onSubmit={submitFilters} onReset={resetFilters} pending={query.isFetching} extraActions={<button className="dashboard-secondary-action" type="button" onClick={() => { setFeedback(null); void query.refetch(); }} disabled={query.isFetching}>刷新</button>}>
          <label>关键词<input aria-label="关键词" value={draftKeyword} onChange={(event) => setDraftKeyword(event.target.value)} placeholder="搜索名称或说明" /></label>
          <label>状态<select aria-label="状态" value={draftStatus} onChange={(event) => setDraftStatus(event.target.value as typeof draftStatus)}><option value="all">全部状态</option><option value="enabled">启用</option><option value="disabled">停用</option></select></label>
        </DashboardFilterPanel>
        {feedback && <p className={`ai-settings-feedback ai-settings-feedback--${feedback.kind}`} role={feedback.kind === 'error' ? 'alert' : 'status'}>{feedback.text}</p>}
        <section className="phase35-card phase35-table-card">
          <header className="phase35-card-header"><div><h2>知识库列表</h2><p>新增、编辑、显式启停与删除企业内知识库</p></div><span className="phase35-chip">筛选 {metricsHaveData ? page.total : '—'} / 全部 {metricsHaveData ? total : '—'} 条</span></header>
          <Phase35DataState loading={query.isLoading} error={query.isError} empty={!total} emptyContent={<p className="phase35-empty">暂无知识库，点击“新建知识库”开始创建</p>} onRetry={() => void query.refetch()}>
            {filteredEmpty ? <p className="phase35-empty">当前筛选条件下没有匹配的知识库</p> : <>
              <div className="phase35-table ai-settings-table-scroll"><table>
                <thead><tr><th>名称</th><th>说明</th><th>文档数</th><th>配置状态</th><th>更新时间</th><th>操作</th></tr></thead>
                <tbody>{page.items.map((item) => {
                  const rowPending = (remove.isPending && remove.variables?.id === item.id) || (toggle.isPending && toggle.variables?.id === item.id);
                  return <tr key={item.id}><td>{item.name}</td><td>{item.description || '—'}</td><td>{item.documentCount}</td><td><span className={`ai-settings-status ai-settings-status--${item.status === 1 ? 'enabled' : 'disabled'}`}>{item.status === 1 ? '启用' : '停用'}</span></td><td>{formatAISettingsTime(item.updatedAt)}</td><td><div className="ai-settings-row-actions">
                    <button type="button" aria-label={`管理文档 ${item.name}`} disabled={rowPending} onClick={(event) => { documentTriggerRef.current = event.currentTarget; setManaging(item); setUploadFile(null); setFeedback(null); }}>管理文档</button>
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
            <label>配置状态<select value={status} onChange={(event) => setStatus(Number(event.target.value))}><option value={1}>启用</option><option value={0}>停用</option></select><small>停用后关联文档不会进入新的会话分析。</small></label>
            {creating && <label className="ai-settings-create-upload">初始文档（可选）<input aria-label="初始文档（可选）" type="file" accept=".txt,.md,.pdf,.docx" disabled={save.isPending} onChange={(event) => setCreateFile(event.target.files?.[0] ?? null)} /><small>支持 .txt、.md、.pdf、.docx，单个文件不超过 20 MB；也可创建后再到“管理文档”上传。</small></label>}
          </form></div>
        </DashboardDialog>
        <DashboardDialog open={managing !== null} title={`管理文档 · ${managing?.name ?? ''}`} triggerRef={documentTriggerRef} width={860} cancelText="关闭" onCancel={() => { if (!upload.isPending && !removeDocument.isPending) { setManaging(null); setUploadFile(null); } }} footer={null}>
          <div className="ai-settings-document-manager">
            <div className="ai-settings-upload-panel">
              <div><strong>上传文档</strong><p>支持 .txt、.md、.pdf、.docx，单个文件不超过 20 MB。PDF 需包含文字层。</p></div>
              <input aria-label="选择文档" type="file" accept=".txt,.md,.pdf,.docx" disabled={upload.isPending} onChange={(event) => setUploadFile(event.target.files?.[0] ?? null)} />
              <button type="button" disabled={!uploadFile || upload.isPending} onClick={() => upload.mutate()}>{upload.isPending ? '上传解析中…' : '上传并解析'}</button>
            </div>
            {documentQuery.isLoading && <p role="status">正在加载文档…</p>}
            {documentQuery.isError && <p role="alert" className="ai-settings-feedback ai-settings-feedback--error">文档列表加载失败，请关闭后重试。</p>}
            {documentQuery.isSuccess && documentQuery.data.length === 0 && <p className="phase35-empty">暂无文档。上传成功并解析后，内容才会参与会话分析。</p>}
            {documentQuery.isSuccess && documentQuery.data.length > 0 && <div className="phase35-table ai-settings-table-scroll"><table><thead><tr><th>文件</th><th>解析状态</th><th>大小</th><th>内容</th><th>上传时间</th><th>操作</th></tr></thead><tbody>
              {documentQuery.data.map((document) => <tr key={document.id}><td>{document.filename}</td><td><span className={`ai-settings-status ai-settings-status--${document.status === 'ready' ? 'enabled' : 'disabled'}`}>{document.status === 'ready' ? '可用于分析' : '解析失败'}</span>{document.errorSummary && <small className="ai-settings-document-error">{document.errorSummary}</small>}</td><td>{formatBytes(document.sizeBytes)}</td><td>{document.status === 'ready' ? `${document.characterCount} 字符 / ${document.chunkCount} 分段` : '未建立分段'}</td><td>{formatAISettingsTime(document.createdAt)}</td><td><ConfirmAction title={`确认删除文档“${document.filename}”？`} description="删除后，该文档内容将不再用于新的会话分析。" onConfirm={() => removeDocument.mutateAsync(document).catch(() => undefined)}><button type="button" aria-label={`删除文档 ${document.filename}`} disabled={removeDocument.isPending || upload.isPending}>删除</button></ConfirmAction></td></tr>)}
            </tbody></table></div>}
            <div className="ai-settings-document-footer"><button type="button" disabled={upload.isPending || removeDocument.isPending} onClick={() => { setManaging(null); setUploadFile(null); }}>关闭</button></div>
          </div>
        </DashboardDialog>
        <DashboardDialog open={discardOpen} title="放弃未保存更改？" danger confirmText="放弃更改" cancelText="继续编辑" onCancel={() => setDiscardOpen(false)} onConfirm={resetEditor}><p>当前修改尚未保存，放弃后无法恢复。</p></DashboardDialog>
      </div>
    </Phase35PageShell>
  );
}
