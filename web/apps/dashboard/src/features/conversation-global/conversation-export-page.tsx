import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';

import { useDashboardAccess } from '../../app/access-context';
import { PageState } from '../../components/page-state/page-state';
import type { ConversationExportTaskInput, ConversationExportTaskPage, ConversationExportType, ConversationGlobalApi } from './conversation-global-api';
import { ConversationExportCandidates } from './conversation-export-candidates';
import { ConversationExportDialog } from './conversation-export-dialog';
import { ConversationExportStepper } from './conversation-export-stepper';
import { ConversationExportTasks } from './conversation-export-tasks';

export function shouldPollExportTasks(data: ConversationExportTaskPage | undefined): boolean {
  return data?.items.some((task) => task.status === 'pending' || task.status === 'running') ?? false;
}

export function ConversationExportPage({ api }: { api: ConversationGlobalApi }) {
  const access = useDashboardAccess();
  const queryClient = useQueryClient();
  const [step, setStep] = useState<1 | 2>(1);
  const [type, setType] = useState<ConversationExportType>('employee');
  const [draftKeyword, setDraftKeyword] = useState('');
  const [keyword, setKeyword] = useState('');
  const [page, setPage] = useState(1);
  const [selectedIds, setSelectedIds] = useState<ReadonlySet<number>>(new Set());
  const [dialogOpen, setDialogOpen] = useState(false);
  const [tasksOpen, setTasksOpen] = useState(false);
  const [tasksPage, setTasksPage] = useState(1);
  const [notice, setNotice] = useState('');
  const [dialogError, setDialogError] = useState('');
  const [creatingTask, setCreatingTask] = useState(false);

  const candidates = useQuery({
    queryKey: ['corp', access.corp.id, 'conversation-export-candidates', type, keyword, page],
    queryFn: async () => {
      if (!api.exportCandidates) throw new Error('当前服务尚未接入会话导出候选数据接口');
      return api.exportCandidates({ type, keyword, departmentId: null, page, pageSize: 20 });
    },
    enabled: step === 2,
    retry: false,
  });
  const tasks = useQuery({
    queryKey: ['corp', access.corp.id, 'conversation-export-tasks', tasksPage],
    queryFn: async () => {
      if (!api.exportTasks) throw new Error('当前服务尚未接入会话导出任务接口');
      return api.exportTasks(tasksPage);
    },
    enabled: tasksOpen,
    refetchInterval: (query) => shouldPollExportTasks(query.state.data) ? 3000 : false,
    retry: false,
  });

  function selectType(nextType: ConversationExportType) {
    setType(nextType); setSelectedIds(new Set()); setPage(1); setKeyword(''); setDraftKeyword('');
  }
  function openCandidates() { setStep(2); setPage(1); setNotice(''); }
  function backToTypes() { setStep(1); setSelectedIds(new Set()); setNotice(''); }
  function submitQuery() { setKeyword(draftKeyword.trim()); setPage(1); }
  async function createTask(input: ConversationExportTaskInput) {
    if (!api.createExportTask) { setDialogError('当前服务尚未接入导出任务创建接口'); return; }
    setNotice('');
    setDialogError('');
    setCreatingTask(true);
    try {
      const result = await api.createExportTask(input);
      setDialogOpen(false); setTasksOpen(true); setTasksPage(1); setSelectedIds(new Set());
      setNotice(result.reused ? '已找到相同条件的导出任务，已在任务中心展示。' : '导出任务已创建，文件生成完成后可在任务中心下载。');
      await queryClient.invalidateQueries({ queryKey: ['corp', access.corp.id, 'conversation-export-tasks'] });
    } catch (error) {
      setDialogError(error instanceof Error ? error.message : '导出任务创建失败');
    } finally {
      setCreatingTask(false);
    }
  }
  async function downloadTask(task: { id: number; artifactName: string }) {
    if (!api.downloadExport) { setNotice('当前服务尚未接入导出文件下载接口'); return; }
    try {
      const result = await api.downloadExport(task.id);
      const url = URL.createObjectURL(result.blob); const link = document.createElement('a'); link.href = url; link.download = result.filename || task.artifactName || 'conversation-export.zip'; link.click(); URL.revokeObjectURL(url);
    } catch (error) { setNotice(error instanceof Error ? error.message : '导出文件下载失败'); }
  }

  return <section className="conversation-export-page">
    <div className="conversation-export-page-heading"><div><span className="conversation-export-eyebrow">会话存档</span><h1>会话导出</h1></div><button className="conversation-export-ghost" type="button" onClick={() => { setTasksOpen(true); setTasksPage(1); }}>任务中心</button></div>
    {notice && <div className="conversation-export-notice is-global">{notice}</div>}
    {step === 1 && <ConversationExportStepper value={type} onChange={selectType} onNext={openCandidates} />}
    {step === 2 && <>
      {candidates.isPending && <PageState state="loading" />}
      {candidates.error && <PageState state="error" onRetry={() => void candidates.refetch()} />}
      {!candidates.error && <ConversationExportCandidates type={type} keyword={draftKeyword} onKeywordChange={setDraftKeyword} onQuery={submitQuery} page={page} onPageChange={setPage} data={candidates.data} loading={candidates.isPending || candidates.isFetching} selectedIds={selectedIds} onSelectedIdsChange={setSelectedIds} onBack={backToTypes} onNext={() => setDialogOpen(true)} onOpenTasks={() => { setTasksOpen(true); setTasksPage(1); }} />}
    </>}
    {dialogOpen && <ConversationExportDialog type={type} selectedIds={selectedIds} submitting={creatingTask} submitError={dialogError} onClearError={() => setDialogError('')} onClose={() => { setDialogOpen(false); setDialogError(''); }} onSubmit={(input) => void createTask(input)} />}
    {tasksOpen && <ConversationExportTasks data={tasks.data} loading={tasks.isPending} refreshing={tasks.isFetching && !tasks.isPending} onClose={() => setTasksOpen(false)} onPageChange={setTasksPage} onDownload={(task) => void downloadTask(task)} />}
  </section>;
}
