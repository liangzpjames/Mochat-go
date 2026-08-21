import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useState } from 'react';
import type { FormEvent } from 'react';
import { useSearchParams } from 'react-router';

import { useDashboardAccess } from '../../app/access-context';
import { DashboardDialog } from '../../components/dashboard-dialog';
import { DashboardPagination } from '../../components/dashboard-pagination';
import { Phase35DataState } from '../phase35/components/data-state';
import { ConversationOperationsShell, ConversationQueryBar, ConversationTabs } from './conversation-operations-shell';
import { formatDateTime } from './conversation-operations-format';
import type { RefuseArchiveFilter, RefuseArchiveRecord, RefuseArchiveApi, RefuseArchiveSubjectType } from './refuse-archive-api';

const pageSize = 20;

const authorizationLabels: Record<string, string> = { refused: '已拒绝', pending: '待授权', authorized: '已授权', expired: '已过期' };
const followLabels: Record<string, string> = { unfollowed: '未跟进', contacted: '已联系', waiting: '等待授权', completed: '跟进完成', followed: '跟进完成' };

function statusLabel(value: string, labels: Record<string, string>) { return labels[value] ?? (value.trim() ? value : '--'); }
function editableFollowStatus(value: string): string {
  return value === 'followed' ? 'completed' : value === 'unfollowed' ? 'contacted' : value;
}
function numberParam(value: string | null) { const parsed = Number(value); return Number.isInteger(parsed) && parsed > 0 ? parsed : 1; }

function initialFilter(searchParams: URLSearchParams): RefuseArchiveFilter {
  const subjectType = searchParams.get('subjectType') === 'room' ? 'room' : 'customer';
  const rawEmployee = searchParams.get('employeeId');
  const employeeId = rawEmployee && /^\d+$/.test(rawEmployee) ? Number(rawEmployee) : '';
  return {
    subjectType,
    subject: searchParams.get('subject') ?? '',
    employeeId,
    refusedFrom: searchParams.get('refusedFrom') ?? '',
    refusedTo: searchParams.get('refusedTo') ?? '',
    authorizationStatus: searchParams.get('authorizationStatus') ?? '',
    followUpStatus: searchParams.get('followUpStatus') ?? '',
    page: numberParam(searchParams.get('page')),
    perPage: pageSize,
  };
}

export function RefuseArchivePage({ api }: { api: RefuseArchiveApi }) {
  const access = useDashboardAccess();
  const queryClient = useQueryClient();
  const [searchParams, setSearchParams] = useSearchParams();
  const applied = initialFilter(searchParams);
  const [draft, setDraft] = useState(applied);
  const [selected, setSelected] = useState<RefuseArchiveRecord | null>(null);
  const [followStatus, setFollowStatus] = useState('contacted');
  const [note, setNote] = useState('');
  const canManage = access.allowedActions.size === 0 || access.allowedActions.has('/chat/refuse-archive#manage');

  useEffect(() => { setDraft(applied); }, [searchParams.toString()]);

  const query = useQuery({
    queryKey: ['refuse-archive', access.corp.id, applied],
    queryFn: () => api.list(applied),
    enabled: access.corp.authorized !== false,
  });
  const rows = query.data?.items ?? [];

  const follow = useMutation({
    mutationFn: () => api.followUp({ id: selected?.id ?? 0, status: followStatus, note }),
    onSuccess: async () => {
      setSelected(null);
      await queryClient.invalidateQueries({ queryKey: ['refuse-archive', access.corp.id] });
    },
  });

  const updateUrl = (next: RefuseArchiveFilter) => {
    const params = new URLSearchParams({ subjectType: next.subjectType, page: String(next.page) });
    if (next.subject.trim()) params.set('subject', next.subject.trim());
    if (next.employeeId !== '') params.set('employeeId', String(next.employeeId));
    if (next.refusedFrom) params.set('refusedFrom', next.refusedFrom);
    if (next.refusedTo) params.set('refusedTo', next.refusedTo);
    if (next.authorizationStatus) params.set('authorizationStatus', next.authorizationStatus);
    if (next.followUpStatus) params.set('followUpStatus', next.followUpStatus);
    setSearchParams(params);
  };
  const submit = (event?: FormEvent) => { event?.preventDefault(); updateUrl({ ...draft, page: 1 }); };
  const reset = () => {
    const next: RefuseArchiveFilter = { ...draft, subject: '', employeeId: '', refusedFrom: '', refusedTo: '', authorizationStatus: '', followUpStatus: '', page: 1 };
    setDraft(next);
    updateUrl(next);
  };
  const switchSubject = (subjectType: RefuseArchiveSubjectType) => updateUrl({ ...applied, subjectType, page: 1 });

  return (
    <ConversationOperationsShell title="拒绝存档" description="按客户和群聊查看授权记录，记录后续跟进。">
      <ConversationTabs value={applied.subjectType} tabs={[{ value: 'customer', label: '客户' }, { value: 'room', label: '群聊' }]} onChange={switchSubject} />
      <ConversationQueryBar fetching={query.isFetching} onQuery={() => submit()} onReset={reset} onRefresh={() => void query.refetch()}>
        <label>主体名称/ID<input aria-label="主体名称/ID" value={draft.subject} onChange={(event) => setDraft({ ...draft, subject: event.target.value })} /></label>
        <label>关联员工<input aria-label="关联员工" inputMode="numeric" value={draft.employeeId} onChange={(event) => setDraft({ ...draft, employeeId: event.target.value ? Number(event.target.value) : '' })} /></label>
        <label>拒绝开始日期<input aria-label="拒绝开始日期" type="date" value={draft.refusedFrom} onChange={(event) => setDraft({ ...draft, refusedFrom: event.target.value })} /></label>
        <label>拒绝结束日期<input aria-label="拒绝结束日期" type="date" value={draft.refusedTo} onChange={(event) => setDraft({ ...draft, refusedTo: event.target.value })} /></label>
        <label>授权状态<select aria-label="授权状态" value={draft.authorizationStatus} onChange={(event) => setDraft({ ...draft, authorizationStatus: event.target.value })}><option value="">全部</option><option value="refused">已拒绝</option><option value="pending">待授权</option><option value="authorized">已授权</option><option value="expired">已过期</option></select></label>
        <label>跟进状态<select aria-label="跟进状态" value={draft.followUpStatus} onChange={(event) => setDraft({ ...draft, followUpStatus: event.target.value })}><option value="">全部</option><option value="unfollowed">未跟进</option><option value="contacted">已联系</option><option value="waiting">等待授权</option><option value="completed">跟进完成</option></select></label>
      </ConversationQueryBar>
      <section className="conversation-operations-card">
        <Phase35DataState loading={query.isLoading} error={query.isError} empty={!query.isLoading && rows.length === 0} onRetry={() => void query.refetch()}>
          <div className="conversation-operations-table-wrap"><table><thead><tr><th>主体信息</th><th>关联员工</th><th>授权状态</th><th>拒绝时间</th><th>同步来源</th><th>跟进状态</th><th>操作</th></tr></thead><tbody>{rows.map((row) => <tr key={row.id}><td>{row.subjectName || row.subjectId}</td><td>{row.employeeName || '--'}</td><td>{statusLabel(row.authorizationStatus, authorizationLabels)}</td><td>{formatDateTime(row.refusedAt)}</td><td>{row.source || '--'}</td><td>{statusLabel(row.followUpStatus, followLabels)}</td><td>{canManage ? <button type="button" onClick={() => { setSelected(row); setFollowStatus(editableFollowStatus(row.followUpStatus)); setNote(row.followUpNote); }}>跟进</button> : '--'}</td></tr>)}</tbody></table></div>
          <DashboardPagination page={applied.page} pageSize={pageSize} total={query.data?.total ?? 0} onPageChange={(page) => updateUrl({ ...applied, page })} />
        </Phase35DataState>
      </section>
      <DashboardDialog mode="drawer" open={selected !== null} title="拒绝存档跟进" onCancel={() => setSelected(null)} onConfirm={() => follow.mutate()} confirmText="保存跟进" confirmLoading={follow.isPending}>
        <p>主体：{selected?.subjectName || selected?.subjectId || '--'}</p>
        <p>当前授权状态：{statusLabel(selected?.authorizationStatus ?? '', authorizationLabels)}</p>
        <label>跟进状态<select aria-label="跟进状态" value={followStatus} onChange={(event) => setFollowStatus(event.target.value)}><option value="contacted">已联系</option><option value="waiting">等待授权</option><option value="completed">跟进完成</option></select></label>
        <label>跟进备注<input aria-label="跟进备注" value={note} onChange={(event) => setNote(event.target.value)} /></label>
      </DashboardDialog>
    </ConversationOperationsShell>
  );
}
