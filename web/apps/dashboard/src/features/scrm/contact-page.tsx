import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useState } from 'react';
import { useSearchParams } from 'react-router';
import { useDashboardAccess } from '../../app/access-context';
import { pageStateForError, PageState } from '../../components/page-state/page-state';
import { AssignmentEditor } from './assignment-editor';
import type { ContactApi, ContactDetail, ContactListInput } from './contact-api';
import { FollowUpTimeline } from './follow-up-timeline';

type FilterForm = { keyword: string; ownerId: string; tagId: string; status: string };
const emptyFilter: FilterForm = { keyword: '', ownerId: '', tagId: '', status: '' };
const statusLabel: Record<string, string> = { owned: '已分配', collaborating: '协作中', public_pool: '公海' };

function filterFromSearch(search: URLSearchParams): FilterForm {
  const ownerId = search.get('ownerId') ?? '';
  const status = search.get('status') ?? '';
  return { keyword: search.get('keyword') ?? '', ownerId: /^\d+$/.test(ownerId) && Number(ownerId) > 0 ? ownerId : '', tagId: search.get('tagId') ?? '', status: status in statusLabel ? status : '' };
}
function nextSearch(current: URLSearchParams, filter: FilterForm, cursor = '', contactId = '') {
  const next = new URLSearchParams(current);
  ['keyword', 'ownerId', 'tagId', 'status', 'cursor', 'contactId'].forEach((key) => next.delete(key));
  if (filter.keyword.trim()) next.set('keyword', filter.keyword.trim());
  if (Number(filter.ownerId) > 0) next.set('ownerId', filter.ownerId);
  if (filter.tagId.trim()) next.set('tagId', filter.tagId.trim());
  if (filter.status) next.set('status', filter.status);
  if (cursor) next.set('cursor', cursor);
  if (contactId) next.set('contactId', contactId);
  return next;
}

export function ContactPage({ api }: { api: ContactApi }) {
  const access = useDashboardAccess(); const corpId = Number(access.corp.id); const queryClient = useQueryClient();
  const [searchParams, setSearchParams] = useSearchParams(); const searchText = searchParams.toString();
  const applied = filterFromSearch(searchParams); const cursor = searchParams.get('cursor') ?? ''; const contactId = searchParams.get('contactId') ?? '';
  const [filter, setFilter] = useState<FilterForm>(applied); const [tagId, setTagId] = useState(''); const [feedback, setFeedback] = useState('');
  useEffect(() => setFilter(filterFromSearch(new URLSearchParams(searchText))), [searchText]);
  const input: ContactListInput = { corpId, ...(applied.keyword ? { keyword: applied.keyword } : {}), ...(Number(applied.ownerId) > 0 ? { ownerIds: [Number(applied.ownerId)] } : {}), ...(applied.tagId ? { tagIds: [applied.tagId] } : {}), ...(applied.status ? { statuses: [applied.status] } : {}), ...(cursor ? { cursor } : {}), pageSize: 20 };
  const list = useQuery({ queryKey: ['scrm-contacts', corpId, input], queryFn: () => api.listContacts(input) });
  const detail = useQuery({ queryKey: ['scrm-contact-detail', corpId, contactId], queryFn: () => api.getContact({ corpId, contactId }), enabled: Boolean(contactId) });
  const refresh = () => { void queryClient.invalidateQueries({ queryKey: ['scrm-contacts', corpId] }); void queryClient.invalidateQueries({ queryKey: ['scrm-contact-detail', corpId, contactId] }); };
  const bindTag = useMutation({ mutationFn: () => api.bindTags({ corpId, tagId: tagId.trim(), contactIds: [contactId], idempotencyKey: `contact-tag-${contactId}-${tagId.trim()}` }), onSuccess: () => { setTagId(''); setFeedback('标签已添加'); refresh(); } });
  const release = useMutation({ mutationFn: (value: ContactDetail) => api.releaseToPublicPool({ corpId, contactId: value.id, version: value.assignment.version, idempotencyKey: `release-${value.id}-${value.assignment.version}` }), onSuccess: () => { setFeedback('联系人已进入公海'); refresh(); } });
  const [amount, setAmount] = useState(''); const [startDate, setStartDate] = useState(''); const [endDate, setEndDate] = useState('');
  const createOpportunity = useMutation({ mutationFn: () => api.createOpportunity({ corpId, contactId, stage: 'proposal', amount: Number(amount), startDate, endDate, ownerId: detail.data?.assignment.ownerId ?? null, idempotencyKey: `contact-opportunity-${contactId}-${Date.now()}` }), onSuccess: () => { setFeedback('商机已创建'); setAmount(''); refresh(); } });
  const canEdit = access.allowedActions?.has('/customer/contact@edit') ?? true;

  return <section className="scrm-contact-page">
    <header className="dashboard-page-header"><div><h1>联系人</h1><p>集中查看客户资料、企微关系、分配、标签、跟进和商机。</p></div></header>
    <div className="dashboard-stat-grid"><article><span>当前结果</span><strong>{list.data?.items.length ?? 0}</strong></article><article><span>已分配</span><strong>{list.data?.items.filter((item) => item.assignmentStatus === 'owned').length ?? 0}</strong></article><article><span>协作中</span><strong>{list.data?.items.filter((item) => item.assignmentStatus === 'collaborating').length ?? 0}</strong></article><article><span>公海</span><strong>{list.data?.items.filter((item) => item.assignmentStatus === 'public_pool').length ?? 0}</strong></article></div>
    <div className="dashboard-filter-bar">
      <label>关键词<input aria-label="搜索联系人" value={filter.keyword} placeholder="姓名或电话" onChange={(event) => setFilter({ ...filter, keyword: event.target.value })} /></label>
      <label>负责人<input aria-label="负责人筛选" type="number" min="1" value={filter.ownerId} onChange={(event) => setFilter({ ...filter, ownerId: event.target.value })} /></label>
      <label>标签 ID<input aria-label="标签筛选" value={filter.tagId} onChange={(event) => setFilter({ ...filter, tagId: event.target.value })} /></label>
      <label>分配状态<select aria-label="分配状态" value={filter.status} onChange={(event) => setFilter({ ...filter, status: event.target.value })}><option value="">全部</option>{Object.entries(statusLabel).map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select></label>
      <div className="dashboard-table-actions"><button type="button" onClick={() => setSearchParams(nextSearch(searchParams, filter))}>查询</button><button type="button" onClick={() => { setFilter(emptyFilter); setSearchParams(nextSearch(searchParams, emptyFilter)); }}>重置</button></div>
    </div>
    <div className="dashboard-data-card"><div className="dashboard-card-heading"><div><h2>联系人列表</h2><p>筛选和当前详情均保存在 URL，可刷新或复制链接继续处理。</p></div></div>
      {list.isPending ? <PageState state="loading" /> : list.isError ? <PageState state={pageStateForError(list.error)} onRetry={() => void list.refetch()} /> : list.data.items.length === 0 ? <PageState state="empty" /> : <div className="dashboard-table-scroll"><table><thead><tr><th>联系人</th><th>电话</th><th>负责人</th><th>标签</th><th>状态</th><th>操作</th></tr></thead><tbody>{list.data.items.map((item) => <tr key={item.id}><td><strong>{item.name}</strong></td><td>{item.phone || '—'}</td><td>{item.ownerId ?? '未分配'}</td><td>{item.tagNames.join('、') || '—'}</td><td>{statusLabel[item.assignmentStatus] ?? item.assignmentStatus}</td><td><button type="button" aria-label={`查看${item.name}`} onClick={() => setSearchParams(nextSearch(searchParams, applied, cursor, item.id))}>查看详情</button></td></tr>)}</tbody></table></div>}
      {list.data?.nextCursor && <div className="dashboard-table-actions"><button type="button" onClick={() => setSearchParams(nextSearch(searchParams, applied, list.data.nextCursor))}>下一页</button></div>}
    </div>
    {contactId && <div className="dashboard-data-card scrm-contact-detail"><div className="dashboard-card-heading"><div><h2>联系人详情</h2><p>资料与生命周期信息来自同一 tenant/corp 聚合接口。</p></div><button type="button" onClick={() => setSearchParams(nextSearch(searchParams, applied, cursor))}>关闭</button></div>
      {detail.isPending ? <PageState state="loading" /> : detail.isError ? <PageState state={pageStateForError(detail.error)} onRetry={() => void detail.refetch()} /> : <ContactDetailPanel value={detail.data} api={api} corpId={corpId} canEdit={canEdit} tagId={tagId} setTagId={setTagId} bindTag={() => bindTag.mutate()} release={() => release.mutate(detail.data)} amount={amount} setAmount={setAmount} startDate={startDate} setStartDate={setStartDate} endDate={endDate} setEndDate={setEndDate} createOpportunity={() => createOpportunity.mutate()} onSaved={refresh} />}
      {(bindTag.isError || release.isError || createOpportunity.isError) && <PageState state={pageStateForError(bindTag.error ?? release.error ?? createOpportunity.error)} description="操作未完成，请按提示刷新后重试。" />}{feedback && <p role="status" className="dashboard-inline-feedback">{feedback}</p>}
    </div>}
  </section>;
}

function ContactDetailPanel({ value, api, corpId, canEdit, tagId, setTagId, bindTag, release, amount, setAmount, startDate, setStartDate, endDate, setEndDate, createOpportunity, onSaved }: { value: ContactDetail; api: ContactApi; corpId: number; canEdit: boolean; tagId: string; setTagId(value: string): void; bindTag(): void; release(): void; amount: string; setAmount(value: string): void; startDate: string; setStartDate(value: string): void; endDate: string; setEndDate(value: string): void; createOpportunity(): void; onSaved(): void }) {
  return <div className="scrm-contact-detail-grid">
    <article><h3>客户资料</h3><dl><dt>姓名</dt><dd>{value.name}</dd><dt>电话</dt><dd>{value.phone || '—'}</dd><dt>标签</dt><dd>{value.tags.map((tag) => tag.name).join('、') || '—'}</dd></dl></article>
    <article><h3>企微好友</h3>{!value.wecomFriendsAvailable ? <p>企微好友关系暂不可用（缺少可靠关联键）</p> : value.wecomFriends.length === 0 ? <p>暂无企微好友关系</p> : <ul>{value.wecomFriends.map((friend) => <li key={`${friend.externalUserId}-${friend.employeeId}`}>{friend.name} · 员工 {friend.employeeId}</li>)}</ul>}</article>
    <article><h3>分配关系</h3><AssignmentEditor assignment={value.assignment} corpId={corpId} api={api} canEdit={canEdit} onSaved={onSaved} /><div className="dashboard-table-actions"><button type="button" disabled={!canEdit || value.assignment.status === 'public_pool'} onClick={release}>进入公海</button></div></article>
    <article><h3>标签维护</h3><label>标签 ID<input aria-label="联系人标签 ID" value={tagId} onChange={(event) => setTagId(event.target.value)} /></label><button type="button" disabled={!canEdit || !tagId.trim()} onClick={bindTag}>添加标签</button></article>
    <article><h3>商机摘要</h3><ul>{value.opportunities.map((item) => <li key={item.id}>{item.stage} · ¥{item.amount} · {item.status}</li>)}</ul><div className="dashboard-filter-bar"><label>金额<input aria-label="商机金额" type="number" min="0" value={amount} onChange={(event) => setAmount(event.target.value)} /></label><label>开始日期<input aria-label="商机开始日期" type="date" value={startDate} onChange={(event) => setStartDate(event.target.value)} /></label><label>结束日期<input aria-label="商机结束日期" type="date" value={endDate} onChange={(event) => setEndDate(event.target.value)} /></label><button type="button" disabled={!canEdit || Number(amount) < 0 || !startDate || !endDate} onClick={createOpportunity}>创建商机</button></div></article>
    <article><FollowUpTimeline api={api} corpId={corpId} contactId={value.id} /></article>
  </div>;
}
