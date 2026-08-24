import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useMemo, useState } from 'react';
import { useSearchParams } from 'react-router';

import { useDashboardAccess } from '../../app/access-context';
import { pageStateForError } from '../../components/page-state/page-state';
import { RiskWarningDrawer, RiskWarningPageHeader, RiskWarningShell, RiskWarningTabs } from '../risk-warning/risk-warning-shell';
import { SensitiveWordConfig } from './sensitive-word-config';
import { SensitiveWordRecords } from './sensitive-word-records';
import type { SensitiveWordApi, SensitiveWordItem, SensitiveWordMatchFilters } from './sensitive-word-api';

type Partition = 'records' | 'config';
type Feedback = { kind: 'success' | 'error'; message: string } | null;
const emptyRecords: SensitiveWordMatchFilters = { employeeIds: [], workRoomId: 0, groupId: 0, triggerStart: '', triggerEnd: '', page: 1, perPage: 20 };
const emptyWords = { groupId: 0, keywords: '', status: 0, page: 1 };

function readRecordFilters(params: URLSearchParams): SensitiveWordMatchFilters {
  const employeeParam = params.get('employeeId') ?? params.get('employeeIds') ?? '';
  const filters: SensitiveWordMatchFilters = {
    employeeIds: employeeParam.split(',').map(Number).filter((item) => item > 0),
    workRoomId: Number(params.get('workRoomId') ?? 0) || 0,
    groupId: Number(params.get('intelligentGroupId') ?? params.get('groupId') ?? 0) || 0,
    scenario: params.get('scenario') ?? '',
    triggerStart: params.get('triggerStart') ?? '',
    triggerEnd: params.get('triggerEnd') ?? '',
    page: Number(params.get('page') ?? 1) || 1,
    perPage: 20,
  };
  const sensitiveWordId = Number(params.get('sensitiveWordId') ?? 0) || 0;
  const source = Number(params.get('source') ?? 0) || 0;
  if (sensitiveWordId > 0) filters.sensitiveWordId = sensitiveWordId;
  if (source > 0) filters.source = source;
  return filters;
}

function writeRecordFilters(params: URLSearchParams, filters: SensitiveWordMatchFilters): URLSearchParams {
  const next = new URLSearchParams(params);
  for (const key of ['employeeId', 'employeeIds', 'workRoomId', 'intelligentGroupId', 'groupId', 'sensitiveWordId', 'source', 'scenario', 'triggerStart', 'triggerEnd', 'page']) next.delete(key);
  if (filters.employeeIds.length) next.set('employeeId', filters.employeeIds.join(','));
  if (filters.workRoomId > 0) next.set('workRoomId', String(filters.workRoomId));
  if (filters.groupId > 0) next.set('intelligentGroupId', String(filters.groupId));
  if (filters.sensitiveWordId && filters.sensitiveWordId > 0) next.set('sensitiveWordId', String(filters.sensitiveWordId));
  if (filters.source && filters.source > 0) next.set('source', String(filters.source));
  if (filters.scenario) next.set('scenario', filters.scenario);
  if (filters.triggerStart) next.set('triggerStart', filters.triggerStart);
  if (filters.triggerEnd) next.set('triggerEnd', filters.triggerEnd);
  if (filters.page > 1) next.set('page', String(filters.page));
  return next;
}

function idempotencyKey(action: string, id: number | string = 0): string { return `sensitive-word-${action}-${id}-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`; }
function mutationFeedback(error: unknown): string { const state = pageStateForError(error); if (state === 'conflict') return '数据已被其他人更新，请刷新后重试。'; if (state === 'forbidden') return '没有执行此操作的权限。'; if (error instanceof Error && error.message.includes('套餐额度已达上限')) return error.message; return error instanceof Error && error.message ? error.message : '操作失败，请稍后重试。'; }
function normalizeDate(value: string): string { return value ? `${value.replace('T', ' ')}${value.length === 16 ? ':00' : ''}` : ''; }

export function SensitiveWordPage({ api }: { api: SensitiveWordApi }) {
  const access = useDashboardAccess();
  const queryClient = useQueryClient();
  const [params, setParams] = useSearchParams();
  const corpID = access.corp.id;
  const partition: Partition = params.get('tab') === 'config' ? 'config' : 'records';
  const [feedback, setFeedback] = useState<Feedback>(null);
  const initialRecords = useMemo(() => readRecordFilters(params), [params]);
  const [recordDraft, setRecordDraft] = useState<SensitiveWordMatchFilters>(initialRecords);
  const [recordApplied, setRecordApplied] = useState<SensitiveWordMatchFilters>(initialRecords);
  const [selectedMatchID, setSelectedMatchID] = useState<number | null>(null);
  const [selectedGroupID, setSelectedGroupID] = useState(0);
  const [wordDraft, setWordDraft] = useState(emptyWords);
  const [wordApplied, setWordApplied] = useState(emptyWords);
  const [moveTargets, setMoveTargets] = useState<Record<number, number>>({});
  const [actionDrawer, setActionDrawer] = useState<'group' | 'word' | null>(null);
  const [newGroupName, setNewGroupName] = useState('');
  const [newWordNames, setNewWordNames] = useState('');
  const [newWordGroupID, setNewWordGroupID] = useState(0);

  const groupsKey = useMemo(() => ['sensitive-word', corpID, 'groups'] as const, [corpID]);
  const wordsKey = useMemo(() => ['sensitive-word', corpID, 'words'] as const, [corpID]);
  const recordsKey = useMemo(() => ['sensitive-word', corpID, 'records'] as const, [corpID]);
  const groups = useQuery({ queryKey: groupsKey, queryFn: () => api.groups(), retry: false });
  const options = useQuery({ queryKey: ['sensitive-word', corpID, 'filter-options'], queryFn: () => api.filterOptions(), enabled: partition === 'records', retry: false });
  const scanner = useQuery({ queryKey: ['sensitive-word', corpID, 'scanner-status'], queryFn: () => api.monitorStatus(), enabled: partition === 'records', retry: false });
  const records = useQuery({ queryKey: [...recordsKey, recordApplied], queryFn: () => api.matches(recordApplied), enabled: partition === 'records', retry: false });
  const words = useQuery({ queryKey: [...wordsKey, wordApplied], queryFn: () => { const input = { groupId: wordApplied.groupId, keywords: wordApplied.keywords, page: wordApplied.page, perPage: 20 }; return api.list(wordApplied.status ? { ...input, status: wordApplied.status } : input); }, enabled: partition === 'config', retry: false });
  const detail = useQuery({ queryKey: ['sensitive-word', corpID, 'detail', selectedMatchID], queryFn: () => api.matchDetail(selectedMatchID ?? 0), enabled: selectedMatchID !== null, retry: false });

  const invalidate = (keys: readonly (readonly unknown[])[]) => { for (const key of keys) void queryClient.invalidateQueries({ queryKey: key }); };
  const success = (message: string, keys: readonly (readonly unknown[])[]) => { setFeedback({ kind: 'success', message }); invalidate(keys); };
  const failed = (error: unknown) => setFeedback({ kind: 'error', message: mutationFeedback(error) });
  const createGroup = useMutation({ mutationFn: () => api.createGroup({ names: [newGroupName.trim()], version: '0', idempotencyKey: idempotencyKey('create-group', newGroupName.trim()) }), onSuccess: () => { setNewGroupName(''); setActionDrawer(null); success('词组已新增。', [groupsKey]); }, onError: failed });
  const createWord = useMutation({ mutationFn: () => api.create({ groupId: newWordGroupID, names: newWordNames.split(/[，,\n]+/).map((item) => item.trim()).filter(Boolean), version: '0', idempotencyKey: idempotencyKey('create-word', newWordNames) }), onSuccess: () => { setNewWordNames(''); setActionDrawer(null); success('敏感词已新增。', [groupsKey, wordsKey]); }, onError: failed });
  const setEnabled = useMutation({ mutationFn: (item: SensitiveWordItem) => api.setEnabled({ id: item.id, enabled: item.status !== 1, version: item.version, idempotencyKey: idempotencyKey('status', item.id) }), onSuccess: () => success('敏感词状态已更新。', [groupsKey, wordsKey]), onError: failed });
  const moveWord = useMutation({ mutationFn: ({ item, groupId }: { item: SensitiveWordItem; groupId: number }) => api.move({ id: item.id, groupId, version: item.version, idempotencyKey: idempotencyKey('move', item.id) }), onSuccess: () => success('敏感词已移动。', [groupsKey, wordsKey]), onError: failed });
  const removeWord = useMutation({ mutationFn: (item: SensitiveWordItem) => api.remove({ id: item.id, version: item.version, confirmed: true, idempotencyKey: idempotencyKey('delete', item.id) }), onSuccess: () => success('敏感词已删除。', [groupsKey, wordsKey]), onError: failed });

  const can = (action: string) => access.allowedActions.size === 0 || access.allowedActions.has(`/ai-insight/v2/sensitive-word@${action}`);
  const queryRecords = () => { const next = { ...recordDraft, triggerStart: normalizeDate(recordDraft.triggerStart), triggerEnd: normalizeDate(recordDraft.triggerEnd), page: 1, perPage: 20 }; setRecordApplied(next); setParams(writeRecordFilters(params, next)); };
  const resetRecords = () => { setRecordDraft(emptyRecords); setRecordApplied(emptyRecords); setParams(writeRecordFilters(params, emptyRecords)); };
  const queryWords = (input = { keywords: wordDraft.keywords, status: wordDraft.status }) => setWordApplied({ ...wordDraft, ...input, groupId: selectedGroupID, page: 1 });
  const resetWords = () => { setWordDraft({ ...emptyWords, groupId: selectedGroupID }); setWordApplied({ ...emptyWords, groupId: selectedGroupID }); };
  const refresh = () => { void groups.refetch(); if (partition === 'records') { void records.refetch(); void options.refetch(); void scanner.refetch(); } else void words.refetch(); };
  const switchPartition = (next: string) => { const value = new URLSearchParams(params); value.set('tab', next); value.delete('matchId'); setParams(value); setFeedback(null); };
  const queryError = groups.error ?? (partition === 'records' ? records.error : words.error);
  const queryPending = groups.isPending || (partition === 'records' ? records.isPending : words.isPending);

  return <RiskWarningShell className="sensitive-word-page"><RiskWarningPageHeader title="敏感词" meta="按企业真实命中记录复核风险，并维护已接入的词库。" />
    <RiskWarningTabs active={partition} tabs={[{ id: 'records', label: '命中记录' }, { id: 'config', label: '敏感词配置' }]} onChange={switchPartition} />
    {feedback ? <p className="risk-warning-inline-feedback" role={feedback.kind === 'error' ? 'alert' : 'status'}>{feedback.message}</p> : null}
    {queryError && queryPending ? <p className="risk-warning-inline-feedback" role="alert">{mutationFeedback(queryError)}</p> : null}
    {partition === 'records' ? <SensitiveWordRecords draft={recordDraft} setDraft={setRecordDraft} groups={groups.data ?? []} options={options.data} optionsLoading={options.isPending} data={records.data} loading={records.isPending} error={records.error} detailID={selectedMatchID} detail={detail.data} detailLoading={detail.isPending} detailError={detail.error} scanner={scanner.data} fetching={records.isFetching || options.isFetching} onQuery={queryRecords} onReset={resetRecords} onRefresh={refresh} onPageChange={(page) => { const next = { ...recordApplied, page }; setRecordApplied(next); setParams(writeRecordFilters(params, next)); }} onOpen={setSelectedMatchID} onCloseDetail={() => setSelectedMatchID(null)} onRetryDetail={() => void detail.refetch()} /> : <SensitiveWordConfig groups={groups.data ?? []} selectedGroupID={selectedGroupID} onSelectGroup={(id) => { setSelectedGroupID(id); setWordDraft((current) => ({ ...current, groupId: id, page: 1 })); setWordApplied((current) => ({ ...current, groupId: id, page: 1 })); }} words={words.data} keywords={wordDraft.keywords} status={wordDraft.status} onKeywordsChange={(value) => setWordDraft((current) => ({ ...current, keywords: value }))} onStatusChange={(value) => setWordDraft((current) => ({ ...current, status: value }))} onQuery={queryWords} onReset={resetWords} onRefresh={refresh} fetching={words.isFetching} loading={words.isPending} error={words.error} page={wordApplied.page} onPageChange={(page) => setWordApplied((current) => ({ ...current, page }))} canAdd={can('add')} canEdit={can('edit')} canDelete={can('delete')} onCreateGroup={() => setActionDrawer('group')} onCreateWord={() => { setNewWordGroupID(selectedGroupID || groups.data?.[0]?.id || 0); setActionDrawer('word'); }} onToggle={(item) => setEnabled.mutate(item)} onMove={(item) => moveWord.mutate({ item, groupId: moveTargets[item.id] ?? item.groupId })} onDelete={(item) => removeWord.mutate(item)} moveTargets={moveTargets} setMoveTarget={(id, value) => setMoveTargets((current) => ({ ...current, [id]: value }))} />}
    <RiskWarningDrawer open={actionDrawer !== null} title={actionDrawer === 'group' ? '新增词组' : '新增敏感词'} description="提交后将写入当前企业词库。" onClose={() => setActionDrawer(null)}>
      {actionDrawer === 'group' ? <form className="risk-warning-form" onSubmit={(event) => { event.preventDefault(); if (newGroupName.trim()) createGroup.mutate(); }}><label>词组名称<input aria-label="词组名称" value={newGroupName} onChange={(event) => setNewGroupName(event.target.value)} autoFocus /></label><p className="risk-warning-form-note">词组名称来自你的配置，不会自动创建业务数据。</p><button className="risk-warning-primary-button" type="submit" disabled={!newGroupName.trim() || createGroup.isPending}>保存词组</button></form> : <form className="risk-warning-form" onSubmit={(event) => { event.preventDefault(); if (newWordNames.trim() && newWordGroupID > 0) createWord.mutate(); }}><label>所属词组<select aria-label="新词所属词组" value={newWordGroupID} onChange={(event) => setNewWordGroupID(Number(event.target.value))}><option value={0}>请选择词组</option>{(groups.data ?? []).map((group) => <option key={group.id} value={group.id}>{group.name}</option>)}</select></label><label>敏感词<textarea aria-label="新敏感词" value={newWordNames} onChange={(event) => setNewWordNames(event.target.value)} placeholder="多个词条可用逗号或换行分隔" autoFocus /></label><p className="risk-warning-form-note">只提交你输入的词条，命中数据由会话存档扫描任务产生。</p><button className="risk-warning-primary-button" type="submit" disabled={!newWordNames.trim() || newWordGroupID <= 0 || createWord.isPending}>保存敏感词</button></form>}
    </RiskWarningDrawer>
  </RiskWarningShell>;
}
