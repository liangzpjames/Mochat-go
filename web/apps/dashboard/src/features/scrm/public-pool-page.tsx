import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useState } from 'react';
import { useSearchParams } from 'react-router';

import { useDashboardAccess } from '../../app/access-context';
import { pageStateForError, PageState } from '../../components/page-state/page-state';
import type { PublicPoolListInput, ScrmApi } from './scrm-api';

type PublicPoolApi = Pick<ScrmApi, 'listPublicPool' | 'claimFromPublicPool' | 'batchClaimFromPublicPool' | 'releaseToPublicPool'>;
type ClaimInput = Parameters<PublicPoolApi['claimFromPublicPool']>[0];
type BatchClaimInput = Parameters<PublicPoolApi['batchClaimFromPublicPool']>[0];
type MoveInput = Parameters<PublicPoolApi['releaseToPublicPool']>[0];
type FailedMutation =
  | { kind: 'claim'; input: ClaimInput; error: unknown }
  | { kind: 'batch'; input: BatchClaimInput; error: unknown }
  | { kind: 'move'; input: MoveInput; error: unknown };
type FilterForm = { keyword: string; source: string; businessType: string; tagId: string; region: string; reason: string; previousOwnerId: string };
const emptyFilter: FilterForm = { keyword: '', source: '', businessType: '', tagId: '', region: '', reason: '', previousOwnerId: '' };

function filtersFromSearch(search: URLSearchParams): FilterForm {
  const previousOwnerId = search.get('previousOwnerId') ?? '';
  return {
    keyword: search.get('keyword') ?? '', source: search.get('source') ?? '', businessType: search.get('businessType') ?? '',
    tagId: search.get('tagId') ?? '', region: search.get('region') ?? '', reason: search.get('reason') ?? '',
    previousOwnerId: /^\d+$/.test(previousOwnerId) && Number(previousOwnerId) > 0 ? previousOwnerId : '',
  };
}

function poolSearch(current: URLSearchParams, filter: FilterForm, cursor = '') {
  const next = new URLSearchParams(current);
  ['keyword', 'source', 'businessType', 'tagId', 'region', 'reason', 'previousOwnerId', 'cursor'].forEach((key) => next.delete(key));
  for (const [key, raw] of Object.entries(filter)) {
    const value = raw.trim();
    if (value) next.set(key, value);
  }
  if (cursor) next.set('cursor', cursor);
  return next;
}

function listInput(corpId: number, filter: FilterForm, cursor: string): PublicPoolListInput {
  return {
    corpId, ...(filter.keyword ? { keyword: filter.keyword } : {}), ...(filter.source ? { sources: [filter.source] } : {}),
    ...(filter.businessType ? { businessTypes: [filter.businessType] } : {}), ...(filter.tagId ? { tagIds: [filter.tagId] } : {}),
    ...(filter.region ? { regions: [filter.region] } : {}), ...(filter.reason ? { reasons: [filter.reason] } : {}),
    ...(Number(filter.previousOwnerId) > 0 ? { previousOwnerIds: [Number(filter.previousOwnerId)] } : {}), ...(cursor ? { cursor } : {}), pageSize: 20,
  };
}

function actionLabel(action: string) {
  return ({ enter: '进入', return: '退回', reclaim: '回收' } as Record<string, string>)[action] ?? '进入';
}

export function PublicPoolPage({ api }: { api: PublicPoolApi }) {
  const access = useDashboardAccess(); const corpId = Number(access.corp.id); const userId = Number(access.session.userId); const queryClient = useQueryClient();
  const [searchParams, setSearchParams] = useSearchParams(); const searchText = searchParams.toString();
  const applied = filtersFromSearch(searchParams); const cursor = searchParams.get('cursor') ?? '';
  const [filter, setFilter] = useState<FilterForm>(applied); const [selected, setSelected] = useState<Record<string, number>>({});
  const [feedback, setFeedback] = useState(''); const [operationContactId, setOperationContactId] = useState(''); const [operationVersion, setOperationVersion] = useState(''); const [operationReason, setOperationReason] = useState('');
  const [failedMutation, setFailedMutation] = useState<FailedMutation | null>(null);
  useEffect(() => setFilter(filtersFromSearch(new URLSearchParams(searchText))), [searchText]);
  const input = listInput(corpId, applied, cursor);
  const query = useQuery({ queryKey: ['scrm-public-pool', corpId, input], queryFn: () => api.listPublicPool(input) });
  const refresh = () => queryClient.invalidateQueries({ queryKey: ['scrm-public-pool', corpId] });
  const claim = useMutation({
    mutationFn: (input: ClaimInput) => api.claimFromPublicPool(input),
    onError: (error, input) => setFailedMutation({ kind: 'claim', input, error }),
    onSuccess: () => { setFailedMutation(null); setFeedback('领取成功，联系人已转入我的负责范围。'); void refresh(); },
  });
  const batch = useMutation({
    mutationFn: (input: BatchClaimInput) => api.batchClaimFromPublicPool(input),
    onError: (error, input) => setFailedMutation({ kind: 'batch', input, error }),
    onSuccess: ({ results }) => { setFailedMutation(null); const failed = results.filter((result) => result.status === 'failed'); setFeedback(failed.length === 0 ? `批量领取完成：${results.length} 成功。` : `批量领取完成：${results.length - failed.length} 成功，${failed.length} 失败（${[...new Set(failed.map((result) => result.errorCode))].join('、')}）。`); setSelected({}); void refresh(); },
  });
  const move = useMutation({
    mutationFn: (input: MoveInput) => api.releaseToPublicPool(input),
    onError: (error, input) => setFailedMutation({ kind: 'move', input, error }),
    onSuccess: () => { setFailedMutation(null); setFeedback('公海归属已更新，历史负责人和原因已留痕。'); setOperationContactId(''); setOperationVersion(''); setOperationReason(''); void refresh(); },
  });
  const canEdit = access.allowedActions.has('/customer/contact@edit');
  const mutationState = failedMutation ? pageStateForError(failedMutation.error) : null;
  const retryFailedMutation = () => {
    if (!failedMutation) return;
    if (pageStateForError(failedMutation.error) === 'conflict') {
      setFailedMutation(null);
      void query.refetch();
      return;
    }
    setFeedback('');
    if (failedMutation.kind === 'claim') claim.mutate(failedMutation.input);
    if (failedMutation.kind === 'batch') batch.mutate(failedMutation.input);
    if (failedMutation.kind === 'move') move.mutate(failedMutation.input);
  };

  return <section className="scrm-public-pool-page">
    <header className="dashboard-page-header"><div><h1>客户公海</h1><p>统一筛选、回收与领取客户；版本、幂等和企业范围共同保护每次归属变更。</p></div></header>
    <div className="dashboard-stat-grid"><article><span>当前结果</span><strong>{query.data?.items.length ?? 0}</strong></article><article><span>已选择</span><strong>{Object.keys(selected).length}</strong></article><article><span>本页回收次数</span><strong>{query.data?.items.reduce((sum, item) => sum + item.recycleCount, 0) ?? 0}</strong></article><article><span>可领取状态</span><strong>{query.data?.items.filter((item) => item.status === 'public_pool').length ?? 0}</strong></article></div>
    <div className="dashboard-filter-bar">
      <label>关键词<input aria-label="公海关键词" value={filter.keyword} placeholder="客户姓名或电话" onChange={(event) => setFilter({ ...filter, keyword: event.target.value })} /></label>
      <label>来源<input aria-label="来源筛选" value={filter.source} placeholder="wecom / manual" onChange={(event) => setFilter({ ...filter, source: event.target.value })} /></label>
      <label>业务类型<input aria-label="业务类型筛选" value={filter.businessType} onChange={(event) => setFilter({ ...filter, businessType: event.target.value })} /></label>
      <label>标签 ID<input aria-label="公海标签筛选" value={filter.tagId} onChange={(event) => setFilter({ ...filter, tagId: event.target.value })} /></label>
      <label>地区<input aria-label="地区筛选" value={filter.region} onChange={(event) => setFilter({ ...filter, region: event.target.value })} /></label>
      <label>入海原因<input aria-label="原因筛选" value={filter.reason} onChange={(event) => setFilter({ ...filter, reason: event.target.value })} /></label>
      <label>前负责人<input aria-label="前负责人筛选" type="number" min="1" value={filter.previousOwnerId} onChange={(event) => setFilter({ ...filter, previousOwnerId: event.target.value })} /></label>
      <div className="dashboard-table-actions"><button type="button" onClick={() => setSearchParams(poolSearch(searchParams, filter))}>查询</button><button type="button" onClick={() => { setFilter(emptyFilter); setSearchParams(poolSearch(searchParams, emptyFilter)); }}>重置</button></div>
    </div>
    {canEdit && <div className="dashboard-data-card"><div className="dashboard-card-heading"><div><h2>纳入公海</h2><p>进入、退回和管理员回收共用一套原子变更与审计历史。</p></div></div><div className="dashboard-filter-bar"><label>联系人 ID<input aria-label="操作联系人 ID" value={operationContactId} onChange={(event) => setOperationContactId(event.target.value)} /></label><label>当前版本<input aria-label="操作版本" type="number" min="1" value={operationVersion} onChange={(event) => setOperationVersion(event.target.value)} /></label><label>原因<input aria-label="操作入海原因" value={operationReason} onChange={(event) => setOperationReason(event.target.value)} /></label><div className="dashboard-table-actions">{([['enter', '进入公海'], ['return', '退回公海'], ['reclaim', '管理员回收']] as const).map(([action, label]) => <button key={action} type="button" disabled={!operationContactId.trim() || Number(operationVersion) <= 0 || !operationReason.trim() || move.isPending} onClick={() => { const contactId = operationContactId.trim(); const version = Number(operationVersion); move.mutate({ corpId, contactId, version, action, reason: operationReason.trim(), idempotencyKey: `pool-${action}-${contactId}-${version}-${userId}` }); }}>{label}</button>)}</div></div></div>}
    {feedback && <p role="alert" className="dashboard-inline-feedback">{feedback}</p>}
    <div className="dashboard-data-card"><div className="dashboard-card-heading"><div><h2>公海客户</h2><p>展示来源、标签、前负责人、入海原因、回收次数和最后跟进时间。</p></div>{canEdit && <button type="button" disabled={Object.keys(selected).length === 0 || batch.isPending} onClick={() => { setFeedback(''); batch.mutate({ corpId, userId, targets: Object.entries(selected).map(([contactId, version]) => ({ contactId, version, idempotencyKey: `batch-claim-${contactId}-${version}-${userId}` })) }); }}>批量领取</button>}</div>
      {query.isPending ? <PageState state="loading" /> : query.isError ? <PageState state={pageStateForError(query.error)} onRetry={() => void query.refetch()} /> : query.data.items.length === 0 ? <PageState state="empty" /> : <div className="dashboard-table-scroll"><table><thead><tr><th>选择</th><th>客户</th><th>来源 / 业务</th><th>标签 / 地区</th><th>入海历史</th><th>最后跟进</th><th>操作</th></tr></thead><tbody>{query.data.items.map((item) => <tr key={item.id}><td><input type="checkbox" aria-label={`选择 ${item.contactName || item.contactId}`} checked={selected[item.contactId] !== undefined} disabled={!canEdit} onChange={(event) => setSelected((current) => { const next = { ...current }; if (event.target.checked) next[item.contactId] = item.version; else delete next[item.contactId]; return next; })} /></td><td><strong>{item.contactName || item.contactId}</strong><small>{item.contactId}</small></td><td>{item.source || '—'}<small>{item.businessType || '—'}</small></td><td>{item.tagNames.join('、') || '—'}<small>{item.region || '—'}</small></td><td><span>{actionLabel(item.poolAction)}</span> · <span>{item.poolReason || '—'}</span><small>前负责人 <span>{item.previousOwnerId ?? '—'}</span> · <span>{item.recycleCount} 次</span></small></td><td>{item.lastFollowUpAt ? new Date(item.lastFollowUpAt).toLocaleString('zh-CN', { hour12: false }) : '暂无跟进'}</td><td><button type="button" aria-label={`领取 ${item.contactName || item.contactId}`} disabled={!canEdit || claim.isPending} onClick={() => { setFeedback(''); claim.mutate({ corpId, contactId: item.contactId, userId, version: item.version, idempotencyKey: `claim-${item.contactId}-${item.version}-${userId}` }); }}>领取</button></td></tr>)}</tbody></table></div>}
      {query.data?.nextCursor && <div className="dashboard-table-actions"><button type="button" onClick={() => setSearchParams(poolSearch(searchParams, applied, query.data.nextCursor))}>下一页</button></div>}
    </div>
    {failedMutation && mutationState && <PageState state={mutationState} description={mutationState === 'conflict' ? '数据已发生变化，请刷新数据后重新选择并提交。' : '公海操作未完成，可重试原操作。'} retryLabel={mutationState === 'conflict' ? '刷新数据' : '重试原操作'} onRetry={retryFailedMutation} />}
  </section>;
}
