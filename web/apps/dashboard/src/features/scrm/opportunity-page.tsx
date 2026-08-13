import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useState } from 'react';
import { useSearchParams } from 'react-router';
import { useDashboardAccess } from '../../app/access-context';
import { pageStateForError, PageState } from '../../components/page-state/page-state';
import { DashboardCursorPagination } from '../../components/dashboard-pagination';
import type { Opportunity, ScrmApi } from './scrm-api';

type OpportunityApi = Pick<ScrmApi, 'listOpportunities' | 'createOpportunity' | 'changeOpportunityStage' | 'appendFollowUp' | 'listContactOptions' | 'listEmployeeOptions' | 'listStageOptions'>;
type FilterForm = { stage: string; status: string; ownerId: string };
const emptyFilter: FilterForm = { stage: '', status: '', ownerId: '' };
const statusLabel: Record<string, string> = { open: '进行中', won: '已赢单', lost: '已输单' };

function filterFromSearch(search: URLSearchParams): FilterForm {
  const status = search.get('status') ?? '';
  const ownerId = search.get('ownerId') ?? '';
  return {
    stage: search.get('stage') ?? '',
    status: status === 'open' || status === 'won' || status === 'lost' ? status : '',
    ownerId: /^\d+$/.test(ownerId) && Number(ownerId) > 0 ? ownerId : '',
  };
}

function opportunitySearch(current: URLSearchParams, filter: FilterForm, cursor = '') {
  const next = new URLSearchParams(current);
  for (const key of ['stage', 'status', 'ownerId', 'cursor']) next.delete(key);
  if (filter.stage.trim()) next.set('stage', filter.stage.trim());
  if (filter.status) next.set('status', filter.status);
  if (Number(filter.ownerId) > 0) next.set('ownerId', filter.ownerId);
  if (cursor) next.set('cursor', cursor);
  return next;
}

let operationSequence = 0;
function newOperationKey(prefix: string) {
  operationSequence += 1;
  return `${prefix}-${Date.now().toString(36)}-${operationSequence.toString(36)}`;
}

export function OpportunityPage({ api }: { api: OpportunityApi }) {
  const access = useDashboardAccess();
  const corpId = Number(access.corp.id);
  const queryClient = useQueryClient();
  const [searchParams, setSearchParams] = useSearchParams();
  const searchText = searchParams.toString();
  const applied = filterFromSearch(searchParams);
  const cursor = searchParams.get('cursor') ?? '';
  const [filter, setFilter] = useState<FilterForm>(applied);
  const [contactId, setContactId] = useState('');
  const [amount, setAmount] = useState('');
  const [startDate, setStartDate] = useState('');
  const [endDate, setEndDate] = useState('');
  const [ownerId, setOwnerId] = useState('');
  const [createStage, setCreateStage] = useState('proposal');
  const [createKey, setCreateKey] = useState(() => newOperationKey('opportunity-create'));
  const [lostReasons, setLostReasons] = useState<Record<string, string>>({});
  const [targetStages, setTargetStages] = useState<Record<string, string>>({});
  const [followUps, setFollowUps] = useState<Record<string, string>>({});
  const [followKeys, setFollowKeys] = useState<Record<string, string>>({});
  const [feedback, setFeedback] = useState('');
  useEffect(() => setFilter(filterFromSearch(new URLSearchParams(searchText))), [searchText]);

  const input = { corpId, ...(applied.stage ? { stage: applied.stage } : {}), ...(applied.status ? { status: applied.status } : {}), ...(Number(applied.ownerId) > 0 ? { ownerId: Number(applied.ownerId) } : {}), ...(cursor ? { cursor } : {}), pageSize: 20 };
  const opportunities = useQuery({ queryKey: ['scrm-opportunities', corpId, input], queryFn: () => api.listOpportunities(input) });
  const contacts = useQuery({ queryKey: ['scrm-contact-options', corpId], queryFn: () => api.listContactOptions?.({ corpId }) ?? Promise.resolve([]) });
  const employees = useQuery({ queryKey: ['scrm-employee-options', corpId], queryFn: () => api.listEmployeeOptions?.() ?? Promise.resolve([]) });
  const stages = useQuery({ queryKey: ['scrm-stage-options', corpId], queryFn: () => api.listStageOptions?.({ corpId }) ?? Promise.resolve([]) });
  const contactNames = new Map((contacts.data ?? []).map((item) => [item.id, item.name]));
  const employeeNames = new Map((employees.data ?? []).map((item) => [item.id, item.name]));
  const stageNames = new Map((stages.data ?? []).map((item) => [item.id, item.name]));
  const refresh = () => queryClient.invalidateQueries({ queryKey: ['scrm-opportunities', corpId] });
  const create = useMutation({
    mutationFn: () => api.createOpportunity({ corpId, contactId: contactId.trim(), stage: createStage, amount: Number(amount), startDate, endDate, ownerId: Number(ownerId) > 0 ? Number(ownerId) : null, idempotencyKey: createKey }),
    onSuccess: () => { setContactId(''); setAmount(''); setStartDate(''); setEndDate(''); setOwnerId(''); setCreateKey(newOperationKey('opportunity-create')); setFeedback('商机创建成功。'); void refresh(); },
  });
  const change = useMutation({
    mutationFn: ({ item, stageId }: { item: Opportunity; stageId: string }) => api.changeOpportunityStage({ corpId, opportunityId: item.id, stageId, lostReason: stageId === 'lost' ? (lostReasons[item.id] ?? '').trim() : '', version: item.version, idempotencyKey: `opportunity-stage-${item.id}-${stageId}-${item.version}` }),
    onSuccess: () => { setFeedback('商机阶段已更新。'); void refresh(); },
  });
  const follow = useMutation({
    mutationFn: (item: Opportunity) => { const content = (followUps[item.id] ?? '').trim(); return api.appendFollowUp({ corpId, contactId: item.contactId, content, idempotencyKey: followKeys[item.id] ?? newOperationKey('opportunity-follow') }); },
    onSuccess: (_, item) => { setFollowUps((current) => ({ ...current, [item.id]: '' })); setFollowKeys((current) => ({ ...current, [item.id]: newOperationKey('opportunity-follow') })); setFeedback('跟进记录已追加。'); void refresh(); },
  });
  const canEdit = access.allowedActions?.size === 0 || access.allowedActions?.has('/customer/opportunity@edit') || access.allowedActions === undefined;
  const mutationError = create.error ?? change.error ?? follow.error;
  const mutationState = change.error && typeof change.error === 'object' && 'status' in change.error && change.error.status === 422 ? 'invalid-transition' : pageStateForError(mutationError);
  const validCreate = contactId.trim() !== '' && amount.trim() !== '' && Number.isFinite(Number(amount)) && Number(amount) >= 0 && startDate !== '' && endDate !== '' && endDate >= startDate;

  return <section className="scrm-opportunity-page">
    <header className="scrm-page-header dashboard-page-header dashboard-data-card"><div><p className="scrm-page-eyebrow">SCRM · 销售协同</p><h1>商机管理</h1><p>集中创建、筛选和推进商机，版本冲突不会覆盖其他成员的最新操作。</p></div><span className="scrm-page-badge">销售进程可追踪</span></header>
    <div aria-label="商机数据概览" className="dashboard-stat-grid scrm-overview" role="region"><article><span>当前结果</span><strong>{opportunities.data?.items.length ?? 0}</strong><small>本页真实数据</small></article><article><span>进行中</span><strong>{opportunities.data?.items.filter((item) => item.status === 'open').length ?? 0}</strong><small>持续跟进推进</small></article><article><span>赢单</span><strong>{opportunities.data?.items.filter((item) => item.status === 'won').length ?? 0}</strong><small>已完成转化</small></article><article><span>输单</span><strong>{opportunities.data?.items.filter((item) => item.status === 'lost').length ?? 0}</strong><small>已记录原因</small></article></div>
    <div className="dashboard-filter-bar">
      <label>阶段<select aria-label="商机阶段" value={filter.stage} onChange={(event) => setFilter({ ...filter, stage: event.target.value })}><option value="">全部阶段</option>{(stages.data ?? []).map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}</select></label>
      <label>状态<select aria-label="商机状态" value={filter.status} onChange={(event) => setFilter({ ...filter, status: event.target.value })}><option value="">全部状态</option><option value="open">进行中</option><option value="won">已赢单</option><option value="lost">已输单</option></select></label>
      <label>负责人<select aria-label="负责人筛选" value={filter.ownerId} onChange={(event) => setFilter({ ...filter, ownerId: event.target.value })}><option value="">全部负责人</option>{(employees.data ?? []).map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}</select></label>
      <div className="dashboard-table-actions"><button type="button" onClick={() => setSearchParams(opportunitySearch(searchParams, filter))}>查询</button><button type="button" onClick={() => { setFilter(emptyFilter); setSearchParams(opportunitySearch(searchParams, emptyFilter)); }}>重置</button></div>
    </div>
    {canEdit && <div className="dashboard-data-card"><div className="dashboard-card-heading"><div><h2>创建商机</h2><p>联系人、负责人和阶段均由服务端按当前企业校验。</p></div></div><div className="dashboard-filter-bar">
      <label>联系人<select aria-label="商机联系人" value={contactId} onChange={(event) => { setContactId(event.target.value); setCreateKey(newOperationKey('opportunity-create')); }}><option value="">选择联系人</option>{(contacts.data ?? []).map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}</select></label>
      <label>初始阶段<select aria-label="初始阶段" value={createStage} onChange={(event) => { setCreateStage(event.target.value); setCreateKey(newOperationKey('opportunity-create')); }}>{(stages.data ?? []).map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}</select></label>
      <label>金额<input aria-label="商机金额" type="number" min="0" step="0.01" value={amount} onChange={(event) => { setAmount(event.target.value); setCreateKey(newOperationKey('opportunity-create')); }} /></label>
      <label>开始日期<input aria-label="开始日期" type="date" value={startDate} onChange={(event) => { setStartDate(event.target.value); setCreateKey(newOperationKey('opportunity-create')); }} /></label>
      <label>结束日期<input aria-label="结束日期" type="date" value={endDate} onChange={(event) => { setEndDate(event.target.value); setCreateKey(newOperationKey('opportunity-create')); }} /></label>
      <label>负责人<select aria-label="商机负责人" value={ownerId} onChange={(event) => { setOwnerId(event.target.value); setCreateKey(newOperationKey('opportunity-create')); }}><option value="">暂不分配</option>{(employees.data ?? []).map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}</select></label>
      <button type="button" disabled={!validCreate || create.isPending} onClick={() => { setFeedback(''); create.mutate(); }}>创建商机</button>
    </div></div>}
    {mutationError && <PageState state={mutationState} description="操作未完成，请刷新数据并核对提交内容后重试。" />}
    {feedback && <p role="status" className="dashboard-inline-feedback">{feedback}</p>}
    <div className="dashboard-data-card scrm-list-card"><div className="dashboard-card-heading"><div><h2>商机列表</h2><p>筛选与游标保存在链接，刷新和前进后退均可恢复。</p></div><button aria-label="刷新商机" disabled={opportunities.isFetching} onClick={() => void opportunities.refetch()} type="button">刷新</button></div>
      {opportunities.isPending ? <PageState state="loading" /> : opportunities.isError ? <PageState state={pageStateForError(opportunities.error)} onRetry={() => void opportunities.refetch()} /> : opportunities.data.items.length === 0 ? <PageState state="empty" /> : <div className="dashboard-table-scroll"><table><thead><tr><th>联系人</th><th>阶段</th><th>金额</th><th>周期</th><th>负责人</th><th>状态</th><th>操作</th></tr></thead><tbody>{opportunities.data.items.map((item) => <tr key={item.id}><td><strong>{contactNames.get(item.contactId) ?? '联系人不可用'}</strong><small>{item.contactId}</small></td><td>{stageNames.get(item.stage) ?? item.stage}</td><td>¥{item.amount.toFixed(2)}</td><td>{item.startDate || '—'} 至 {item.endDate || '—'}</td><td>{item.ownerId ? employeeNames.get(String(item.ownerId)) ?? '原负责人不可用' : '未分配'}</td><td><span className={`scrm-status scrm-status-${item.status}`}>{statusLabel[item.status] ?? item.status}{item.lostReason ? ` · ${item.lostReason}` : ''}</span></td><td><div className="scrm-opportunity-actions">
        {canEdit && item.status === 'open' && <><div className="dashboard-table-actions"><select aria-label={`目标阶段 ${item.id}`} value={targetStages[item.id] ?? ''} onChange={(event) => setTargetStages((current) => ({ ...current, [item.id]: event.target.value }))}><option value="">选择目标阶段</option>{(stages.data ?? []).filter((stage) => stage.id !== item.stage).map((stage) => <option key={stage.id} value={stage.id}>{stage.name}</option>)}</select><button type="button" aria-label={`推进阶段 ${item.id}`} disabled={!(targetStages[item.id] ?? '').trim() || change.isPending} onClick={() => change.mutate({ item, stageId: (targetStages[item.id] ?? '').trim() })}>推进阶段</button><button type="button" aria-label={`赢单 ${item.id}`} disabled={change.isPending} onClick={() => change.mutate({ item, stageId: 'won' })}>赢单</button></div><div className="dashboard-table-actions"><input aria-label={`输单原因 ${item.id}`} value={lostReasons[item.id] ?? ''} placeholder="输单原因" onChange={(event) => setLostReasons((current) => ({ ...current, [item.id]: event.target.value }))} /><button type="button" aria-label={`输单 ${item.id}`} disabled={!(lostReasons[item.id] ?? '').trim() || change.isPending} onClick={() => change.mutate({ item, stageId: 'lost' })}>输单</button></div></>}
        {canEdit && <div className="dashboard-table-actions"><input aria-label={`跟进内容 ${item.id}`} value={followUps[item.id] ?? ''} placeholder="追加跟进" onChange={(event) => { setFollowUps((current) => ({ ...current, [item.id]: event.target.value })); setFollowKeys((current) => ({ ...current, [item.id]: newOperationKey('opportunity-follow') })); }} /><button type="button" aria-label={`追加跟进 ${item.id}`} disabled={!(followUps[item.id] ?? '').trim() || follow.isPending} onClick={() => follow.mutate(item)}>追加跟进</button></div>}
      </div></td></tr>)}</tbody></table></div>}
      {(cursor || opportunities.data?.nextCursor) && <DashboardCursorPagination cursor={cursor} nextCursor={opportunities.data?.nextCursor} onCursorChange={(nextCursor) => setSearchParams(opportunitySearch(searchParams, applied, nextCursor))} />}
    </div>
  </section>;
}
