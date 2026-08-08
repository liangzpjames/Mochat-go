import { ApiError } from '@mochat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useState } from 'react';
import { useSearchParams } from 'react-router';

import { useDashboardAccess } from '../../app/access-context';
import { pageStateForError, PageState } from '../../components/page-state/page-state';
import type { Lead, LeadApi, LeadListInput, LeadSource, LeadStatus } from './lead-api';

const statusLabel: Record<LeadStatus, string> = { new: '待确认', qualified: '已确认', converted: '已转化', discarded: '已废弃' };
const sourceLabel: Record<LeadSource, string> = { manual: '手工录入', import: '批量导入', wecom: '企业微信' };
type FilterForm = { keyword: string; status: '' | LeadStatus; source: '' | LeadSource; ownerId: string };
const emptyFilter: FilterForm = { keyword: '', status: '', source: '', ownerId: '' };

function filterFromSearch(search: URLSearchParams): FilterForm {
  const status = search.get('status');
  const source = search.get('source');
  const ownerId = search.get('ownerId') ?? '';
  return {
    keyword: search.get('keyword') ?? '',
    status: status === 'new' || status === 'qualified' || status === 'converted' || status === 'discarded' ? status : '',
    source: source === 'manual' || source === 'import' || source === 'wecom' ? source : '',
    ownerId: /^\d+$/.test(ownerId) && Number(ownerId) > 0 ? ownerId : '',
  };
}

function leadSearch(current: URLSearchParams, filter: FilterForm, cursor = ''): URLSearchParams {
  const next = new URLSearchParams(current);
  for (const key of ['keyword', 'status', 'source', 'ownerId', 'cursor']) next.delete(key);
  const keyword = filter.keyword.trim();
  if (keyword) next.set('keyword', keyword);
  if (filter.status) next.set('status', filter.status);
  if (filter.source) next.set('source', filter.source);
  if (Number(filter.ownerId) > 0) next.set('ownerId', filter.ownerId);
  if (cursor) next.set('cursor', cursor);
  return next;
}

function mutationMessage(error: unknown) {
  if (error instanceof ApiError && error.status === 409) return '数据已更新，请刷新后重试。';
  if (error instanceof ApiError && error.status === 422) return '提交内容未通过校验，请检查后重试。';
  if (error instanceof ApiError && error.status === 403) return '当前账号没有执行此操作的权限。';
  return '操作失败，请稍后重试。';
}

export function LeadPage({ api }: { api: LeadApi }) {
  const access = useDashboardAccess(); const corpId = Number(access.corp.id); const queryClient = useQueryClient();
  const [searchParams, setSearchParams] = useSearchParams();
  const searchText = searchParams.toString();
  const applied = filterFromSearch(searchParams); const cursor = searchParams.get('cursor') ?? '';
  const [filter, setFilter] = useState<FilterForm>(applied);
  useEffect(() => { setFilter(filterFromSearch(new URLSearchParams(searchText))); }, [searchText]);
  const [businessKey, setBusinessKey] = useState(''); const [name, setName] = useState(''); const [phone, setPhone] = useState(''); const [source, setSource] = useState<LeadSource>('wecom');
  const [selected, setSelected] = useState<Record<string, number>>({}); const [ownerId, setOwnerId] = useState(''); const [discardReason, setDiscardReason] = useState(''); const [feedback, setFeedback] = useState('');
  const listInput: LeadListInput = { corpId, ...(applied.keyword ? { keyword: applied.keyword } : {}), ...(applied.status ? { statuses: [applied.status] } : {}), ...(applied.source ? { sources: [applied.source] } : {}), ...(Number(applied.ownerId) > 0 ? { ownerIds: [Number(applied.ownerId)] } : {}), ...(cursor ? { cursor } : {}), pageSize: 20 };
  const leads = useQuery({ queryKey: ['scrm-leads', corpId, listInput], queryFn: () => api.list(listInput) });
  const owners = useQuery({ queryKey: ['scrm-lead-owner-options', corpId], queryFn: () => api.listOwnerOptions() });
  const ownerNames = new Map((owners.data ?? []).map((item) => [item.id, item.name]));
  const refresh = () => queryClient.invalidateQueries({ queryKey: ['scrm-leads', corpId] });
  const create = useMutation({ mutationFn: async () => { const duplicate = await api.findDuplicates({ corpId, businessKey: businessKey.trim(), phone: phone.trim() }); if (duplicate.items.length > 0) throw new Error('DUPLICATE'); return api.create({ corpId, businessKey: businessKey.trim(), name: name.trim(), phone: phone.trim(), source }); }, onSuccess: () => { setBusinessKey(''); setName(''); setPhone(''); setFeedback('线索创建成功。'); void refresh(); }, onError: (error) => setFeedback(error instanceof Error && error.message === 'DUPLICATE' ? '检测到重复线索，请核对联系电话或业务标识。' : mutationMessage(error)) });
  const assign = useMutation({ mutationFn: () => api.assign({ corpId, ownerId: Number(ownerId), targets: Object.entries(selected).map(([id, version]) => ({ id, version })) }), onSuccess: ({ results }) => { const failed = results.filter((item) => item.status === 'failed'); setFeedback(failed.length === 0 ? `分配完成：${results.length} 成功。` : `分配完成：${results.length - failed.length} 成功，${failed.length} 失败（${[...new Set(failed.map((item) => item.errorCode))].join('、')}）。`); setSelected({}); void refresh(); }, onError: (error) => setFeedback(mutationMessage(error)) });
  const transition = useMutation({ mutationFn: ({ lead, toStatus }: { lead: Lead; toStatus: LeadStatus }) => api.transition({ corpId, id: lead.id, toStatus, version: lead.version, discardReason: toStatus === 'discarded' ? discardReason.trim() : '' }), onSuccess: () => { setFeedback('线索状态已更新。'); setDiscardReason(''); void refresh(); }, onError: (error) => setFeedback(mutationMessage(error)) });
  const canAdd = access.allowedActions.has('/customer/clue/default@add'); const canAssign = access.allowedActions.has('/customer/clue/default@assign'); const canEdit = access.allowedActions.has('/customer/clue/default@edit');

  return <section className="scrm-lead-page">
    <header className="scrm-page-header dashboard-page-header dashboard-data-card"><div><p className="scrm-page-eyebrow">SCRM · 客户经营</p><h1>线索池</h1><p>统一筛选、分配和转化客户线索，所有操作均保留企业范围与版本校验。</p></div><span className="scrm-page-badge">真实线索流转</span></header>
    <div aria-label="线索数据概览" className="dashboard-stat-grid scrm-overview" role="region">
      <article><span>当前结果</span><strong>{leads.data?.items.length ?? 0}</strong><small>本页真实数据</small></article><article><span>待确认</span><strong>{leads.data?.items.filter((item) => item.status === 'new').length ?? 0}</strong><small>等待业务确认</small></article><article><span>已确认</span><strong>{leads.data?.items.filter((item) => item.status === 'qualified').length ?? 0}</strong><small>可转化联系人</small></article><article><span>已选择</span><strong>{Object.keys(selected).length}</strong><small>批量分配范围</small></article>
    </div>
    <div className="dashboard-filter-bar">
      <label>关键词<input aria-label="搜索线索" value={filter.keyword} placeholder="姓名、电话或业务标识" onChange={(event) => setFilter({ ...filter, keyword: event.target.value })} /></label>
      <label>状态<select aria-label="线索状态" value={filter.status} onChange={(event) => setFilter({ ...filter, status: event.target.value as FilterForm['status'] })}><option value="">全部状态</option>{Object.entries(statusLabel).map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select></label>
      <label>来源<select aria-label="线索来源" value={filter.source} onChange={(event) => setFilter({ ...filter, source: event.target.value as FilterForm['source'] })}><option value="">全部来源</option>{Object.entries(sourceLabel).map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select></label>
      <label>负责人<select aria-label="负责人筛选" value={filter.ownerId} onChange={(event) => setFilter({ ...filter, ownerId: event.target.value })}><option value="">全部负责人</option>{(owners.data ?? []).map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}</select></label>
      <div className="dashboard-table-actions"><button type="button" onClick={() => setSearchParams(leadSearch(searchParams, filter))}>查询</button><button type="button" onClick={() => { setFilter(emptyFilter); setSearchParams(leadSearch(searchParams, emptyFilter)); }}>重置</button></div>
    </div>
    {canAdd && <div className="dashboard-data-card scrm-lead-create"><div className="dashboard-card-heading"><div><h2>新增线索</h2><p>提交前自动检查同企业内的联系电话与业务标识。</p></div></div><div className="dashboard-filter-bar"><label>客户名称<input aria-label="客户名称" value={name} onChange={(event) => setName(event.target.value)} /></label><label>联系电话<input aria-label="联系电话" value={phone} onChange={(event) => setPhone(event.target.value)} /></label><label>业务标识<input aria-label="业务标识" value={businessKey} onChange={(event) => setBusinessKey(event.target.value)} /></label><label>来源<select aria-label="新增来源" value={source} onChange={(event) => setSource(event.target.value as LeadSource)}>{Object.entries(sourceLabel).map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select></label><button type="button" disabled={!name.trim() || !businessKey.trim() || create.isPending} onClick={() => { setFeedback(''); create.mutate(); }}>新增线索</button></div></div>}
    {feedback && <p role="alert" className="dashboard-inline-feedback">{feedback}</p>}
    <div className="dashboard-data-card scrm-list-card">
      <div className="dashboard-card-heading"><div><h2>线索列表</h2><p>版本冲突不会覆盖其他成员刚完成的操作。</p></div><div className="dashboard-table-actions"><button aria-label="刷新线索" disabled={leads.isFetching} onClick={() => void leads.refetch()} type="button">刷新</button>{canAssign && <><select aria-label="分配负责人" value={ownerId} onChange={(event) => setOwnerId(event.target.value)}><option value="">选择负责人</option>{(owners.data ?? []).map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}</select><button type="button" disabled={Object.keys(selected).length === 0 || Number(ownerId) <= 0 || assign.isPending} onClick={() => { setFeedback(''); assign.mutate(); }}>批量分配</button></>}</div></div>
      {canEdit && <div className="scrm-lead-discard-reason"><label>废弃原因<input aria-label="废弃原因" value={discardReason} placeholder="废弃前填写原因" onChange={(event) => setDiscardReason(event.target.value)} /></label></div>}
      {leads.isPending ? <PageState state="loading" /> : leads.isError ? <PageState state={pageStateForError(leads.error)} onRetry={() => void leads.refetch()} /> : leads.data.items.length === 0 ? <PageState state="empty" /> : <div className="dashboard-table-scroll"><table><thead><tr><th>选择</th><th>客户</th><th>联系方式</th><th>来源</th><th>负责人</th><th>状态</th><th>操作</th></tr></thead><tbody>{leads.data.items.map((lead) => <tr key={lead.id}><td><input type="checkbox" aria-label={`选择${lead.name}`} checked={selected[lead.id] !== undefined} disabled={!canAssign || lead.status === 'converted' || lead.status === 'discarded'} onChange={(event) => setSelected((current) => { const next = { ...current }; if (event.target.checked) next[lead.id] = lead.version; else delete next[lead.id]; return next; })} /></td><td><strong>{lead.name}</strong><small>{lead.businessKey}</small></td><td>{lead.phone || '—'}</td><td>{sourceLabel[lead.source]}</td><td>{lead.ownerId ? ownerNames.get(lead.ownerId) ?? '原负责人不可用' : '未分配'}</td><td><span className={`scrm-status scrm-status-${lead.status}`}>{statusLabel[lead.status]}</span></td><td><div className="dashboard-table-actions">{canEdit && lead.status === 'new' && <button type="button" onClick={() => transition.mutate({ lead, toStatus: 'qualified' })}>标记为已确认</button>}{canEdit && lead.status === 'qualified' && <button type="button" onClick={() => transition.mutate({ lead, toStatus: 'converted' })}>转化为联系人</button>}{canEdit && (lead.status === 'new' || lead.status === 'qualified') && <button type="button" disabled={!discardReason.trim()} onClick={() => transition.mutate({ lead, toStatus: 'discarded' })}>废弃</button>}</div></td></tr>)}</tbody></table></div>}
      {leads.data?.nextCursor && <div className="dashboard-table-actions scrm-lead-pagination"><button type="button" onClick={() => setSearchParams(leadSearch(searchParams, applied, leads.data.nextCursor))}>下一页</button></div>}
    </div>
  </section>;
}
