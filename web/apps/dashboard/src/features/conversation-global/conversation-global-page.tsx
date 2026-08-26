import { ApiError } from '@mochat/api-client';
import { useQuery } from '@tanstack/react-query';
import { useEffect, useMemo, useState } from 'react';
import type { FormEvent, KeyboardEvent, MouseEvent, ReactElement } from 'react';
import { useSearchParams } from 'react-router';

import { useDashboardAccess } from '../../app/access-context';
import { DashboardPagination } from '../../components/dashboard-pagination';
import { PageState } from '../../components/page-state/page-state';
import { updateSearch } from '../../shared/query-state';
import { ConversationArchiveUnavailableState, isConversationArchiveUnavailable } from './conversation-archive-state';
import type { ConversationGlobalApi, ConversationMessage, ConversationOverview, ConversationSearch, ConversationSummary, ConversationTargetType } from './conversation-global-api';
import { ConversationMessageContent } from './conversation-message-content';

type FilterDraft = {
  keyword: string;
  conversationType: ConversationSearch['conversationType'];
  employeeIds: string;
  startAt: string;
  endAt: string;
  bucket: NonNullable<ConversationSearch['bucket']>;
  messageTypes: string[];
};

const defaultPageSize = 20;
const statusTabs = [['all', '全部会话'], ['timeout', '超时回复'], ['risk', '风险会话'], ['today', '今日新增'], ['focused', '重点关注']] as const;
const messageTypeOptions = [['text', '文字'], ['image', '图片'], ['emotion', '表情'], ['link', '链接'], ['voice', '语音'], ['video', '视频'], ['file', '文件'], ['card', '名片']] as const;
const metricDefinitions = [['customerConversations', '客户会话', '笔'], ['customerMessages', '客户消息', '条'], ['customerEmployees', '客户覆盖员工', '人'], ['roomConversations', '群聊会话', '笔'], ['roomMessages', '群聊消息', '条'], ['roomEmployees', '群聊覆盖员工', '人'], ['riskConversations', '风险会话', '笔'], ['timeoutConversations', '超时会话', '笔']] as const;

function positiveInteger(value: string | null, fallback: number): number { const parsed = Number(value); return Number.isInteger(parsed) && parsed > 0 ? parsed : fallback; }
function filtersFromSearch(search: URLSearchParams, fixed: ConversationTargetType | undefined): FilterDraft {
  const raw = search.get('conversationType');
  const conversationType = raw === 'employee' || raw === 'customer' || raw === 'room' ? raw : '';
  const bucket = search.get('bucket');
  return { keyword: search.get('keyword') ?? '', conversationType: fixed ?? conversationType, employeeIds: search.getAll('employeeIds').join(','), startAt: search.get('startAt') ?? '', endAt: search.get('endAt') ?? '', bucket: bucket === 'timeout' || bucket === 'risk' || bucket === 'today' || bucket === 'focused' ? bucket : 'all', messageTypes: search.getAll('messageTypes') };
}
function employeeIDsFromDraft(value: string): string[] { return [...new Set(value.split(',').map((item) => item.trim()).filter(Boolean))]; }
function targetTypeLabel(type: ConversationTargetType): string { return type === 'employee' ? '员工' : type === 'customer' ? '客户' : '群聊'; }
function shortTime(value: string): string { return value.length >= 16 ? value.slice(5, 16) : value || '--'; }
function avatar(name: string, source: string): ReactElement { return source.trim() !== '' ? <img alt="" src={source} /> : <span aria-hidden="true">{name.trim().slice(0, 1) || '?'}</span>; }
function fallbackOverview(page: { total: number; list: readonly ConversationSummary[] }): ConversationOverview {
  const available = (value: number) => ({ value, changeRate: null, status: 'available' as const });
  const customers = page.list.filter((item) => item.targetType === 'customer');
  const rooms = page.list.filter((item) => item.targetType === 'room');
  return { metrics: { customerConversations: available(customers.length), customerMessages: available(customers.reduce((sum, item) => sum + (item.messageTotal ?? 0), 0)), customerEmployees: available(new Set(customers.map((item) => item.employeeId)).size), roomConversations: available(rooms.length), roomMessages: available(rooms.reduce((sum, item) => sum + (item.messageTotal ?? 0), 0)), roomEmployees: available(new Set(rooms.map((item) => item.employeeId)).size), riskConversations: available(page.list.filter((item) => (item.riskCount ?? 0) > 0).length), timeoutConversations: available(page.list.filter((item) => (item.timeoutCount ?? 0) > 0).length) }, capabilities: [] };
}
function MetricValue({ value, unit, status, reason }: { value: number | null; unit: string; status: string; reason?: string }) { return status === 'unavailable' || value === null ? <strong className="conversation-global-metric-unavailable" title={reason}>暂无</strong> : <strong>{value.toLocaleString()}<small>{unit}</small></strong>; }

function ConversationCard({ item, onDetail, onFocus, focusing }: { item: ConversationSummary; onDetail: () => void; onFocus: (event: MouseEvent<HTMLButtonElement>) => void; focusing: boolean }) {
  const focusLabel = item.focused ? '取消重点关注' : '标记重点关注';
  function handleKeyDown(event: KeyboardEvent<HTMLElement>) {
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault();
      onDetail();
    }
  }
  return <article className="conversation-global-card" onClick={onDetail} onKeyDown={handleKeyDown} tabIndex={0}>
    <header className="conversation-global-card-header"><div className="conversation-global-card-date">{shortTime(item.sentAt)}</div><button aria-label={focusLabel} className={`conversation-global-focus ${item.focused ? 'is-focused' : ''}`} disabled={focusing} onClick={(event) => { event.stopPropagation(); onFocus(event); }} type="button">{item.focused ? '★' : '☆'}</button></header>
    <div className="conversation-global-card-people"><div className="conversation-global-person"><div className="conversation-global-avatar">{avatar(item.employeeName, item.employeeAvatar)}</div><span>{item.employeeName || `员工 ${item.employeeId}`}</span></div><span className="conversation-global-arrow">→</span><div className="conversation-global-person conversation-global-person-target"><div className="conversation-global-avatar">{avatar(item.targetName, item.targetAvatar)}</div><span>{item.targetName || `${targetTypeLabel(item.targetType)} ${item.targetId}`}</span></div></div>
    <div className="conversation-global-card-meta"><span>{targetTypeLabel(item.targetType)} · {item.messageTotal ?? 0} 条消息</span>{item.riskCount ? <em className="is-risk">风险 {item.riskCount}</em> : null}{item.timeoutCount ? <em className="is-timeout">超时 {item.timeoutCount}</em> : null}</div>
    <p className="conversation-global-card-preview">{item.lastMessage || '暂无可展示的消息内容'}</p>
    <footer className="conversation-global-card-footer"><span>{item.archiveSource === 'simulated' ? '演示数据' : '企业微信存档'}</span><button onClick={onDetail} type="button">查看会话</button></footer>
  </article>;
}

export function ConversationGlobalPage({ api, fixedConversationType }: { api: ConversationGlobalApi; fixedConversationType?: ConversationTargetType }) {
  const access = useDashboardAccess();
  const [searchParams, setSearchParams] = useSearchParams();
  const currentFilters = filtersFromSearch(searchParams, fixedConversationType);
  const [draft, setDraft] = useState<FilterDraft>(currentFilters);
  const [filterError, setFilterError] = useState<string | null>(null);
  const [selectedID, setSelectedID] = useState<string | null>(null);
  const [focusingID, setFocusingID] = useState<string | null>(null);
  const [focusError, setFocusError] = useState<string | null>(null);
  const [messageTypeOpen, setMessageTypeOpen] = useState(false);
  const page = positiveInteger(searchParams.get('page'), 1);
  const pageSize = Math.min(positiveInteger(searchParams.get('pageSize'), defaultPageSize), 100);
  const searchText = searchParams.toString();
  const isGlobal = fixedConversationType === undefined;
  useEffect(() => setDraft(filtersFromSearch(new URLSearchParams(searchText), fixedConversationType)), [fixedConversationType, searchText]);
  const input = useMemo<ConversationSearch>(() => {
    const value: ConversationSearch = { keyword: currentFilters.keyword, conversationType: currentFilters.conversationType, employeeIds: employeeIDsFromDraft(currentFilters.employeeIds), startAt: currentFilters.startAt, endAt: currentFilters.endAt, page, pageSize };
    if (currentFilters.bucket !== 'all') value.bucket = currentFilters.bucket;
    if (currentFilters.messageTypes.length > 0) value.messageTypes = currentFilters.messageTypes;
    return value;
  }, [currentFilters, page, pageSize]);
  const overviewInput = useMemo(() => {
    const value: Omit<ConversationSearch, 'page' | 'pageSize'> = {
      keyword: input.keyword,
      conversationType: input.conversationType,
      employeeIds: input.employeeIds,
      startAt: input.startAt,
      endAt: input.endAt,
    };
    if (input.bucket !== undefined) value.bucket = input.bucket;
    if (input.messageTypes !== undefined) value.messageTypes = input.messageTypes;
    return value;
  }, [input]);
  const listQuery = useQuery({ queryKey: ['corp', access.corp.id, 'conversation-global', input], queryFn: () => api.search(input) });
  const overviewQuery = useQuery({ queryKey: ['corp', access.corp.id, 'conversation-global-overview', overviewInput], queryFn: () => api.overview!(overviewInput), enabled: isGlobal && api.overview !== undefined });
  const detailQuery = useQuery({ queryKey: ['corp', access.corp.id, 'conversation-global-detail', selectedID], queryFn: () => api.detail(selectedID ?? ''), enabled: selectedID !== null, retry: false });
  const data = listQuery.data;
  const overview = overviewQuery.data ?? (data === undefined ? null : fallbackOverview(data));
  const archiveUnauthorized = isConversationArchiveUnavailable(listQuery.error);
  const forbidden = listQuery.error instanceof ApiError && listQuery.error.kind === 'forbidden' && !archiveUnauthorized;
  const detailNotFound = detailQuery.error instanceof ApiError && detailQuery.error.status === 404;
  const detailForbidden = detailQuery.error instanceof ApiError && detailQuery.error.status === 403 && !isConversationArchiveUnavailable(detailQuery.error);
  const detailArchiveUnauthorized = isConversationArchiveUnavailable(detailQuery.error);
  function applyFilters(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if ((draft.startAt === '') !== (draft.endAt === '')) return setFilterError('开始日期和结束日期需要同时填写');
    if (draft.startAt !== '' && draft.startAt > draft.endAt) return setFilterError('开始日期不能晚于结束日期');
    const employeeIds = employeeIDsFromDraft(draft.employeeIds);
    if (employeeIds.some((id) => !/^\d+$/.test(id) || Number(id) <= 0)) return setFilterError('员工 ID 必须是逗号分隔的正整数');
    setFilterError(null);
    setMessageTypeOpen(false);
    const next = updateSearch(searchParams, { keyword: draft.keyword, conversationType: fixedConversationType ?? draft.conversationType, startAt: draft.startAt, endAt: draft.endAt, page: 1, pageSize });
    if (draft.bucket === 'all') next.delete('bucket'); else next.set('bucket', draft.bucket);
    next.delete('employeeIds'); employeeIds.forEach((id) => next.append('employeeIds', id));
    next.delete('messageTypes'); draft.messageTypes.forEach((type) => next.append('messageTypes', type));
    setSearchParams(next);
  }
  function resetFilters() { setFilterError(null); setFocusError(null); setMessageTypeOpen(false); setSearchParams(new URLSearchParams({ page: '1', pageSize: String(defaultPageSize) })); }
  function changePage(nextPage: number) { setSearchParams(updateSearch(searchParams, { page: nextPage, pageSize })); }
  function changeBucket(bucket: FilterDraft['bucket']) { const next = new URLSearchParams(searchParams); if (bucket === 'all') next.delete('bucket'); else next.set('bucket', bucket); next.set('page', '1'); next.set('pageSize', String(pageSize)); setSearchParams(next); }
  function toggleMessageType(value: string) { setDraft((current) => ({ ...current, messageTypes: current.messageTypes.includes(value) ? current.messageTypes.filter((item) => item !== value) : [...current.messageTypes, value] })); }
  async function toggleFocus(item: ConversationSummary) {
    if (!api.setFocus || !api.removeFocus || !item.conversationId) return setFocusError('个人关注能力未接入，当前无法保存重点会话');
    setFocusingID(item.id); setFocusError(null);
    try { await (item.focused ? api.removeFocus(item.conversationId) : api.setFocus(item.conversationId)); await listQuery.refetch(); if (overviewQuery.isEnabled) await overviewQuery.refetch(); }
    catch (error) { setFocusError(error instanceof Error ? error.message : '重点会话保存失败'); }
    finally { setFocusingID(null); }
  }

  return <section className={`conversation-global-page ${isGlobal ? 'is-global-workbench' : ''}`}>
    {overview !== null && <section aria-label="查询概览" className="conversation-global-overview"><span className="sr-only">会话总量</span><span className="sr-only">{data?.total ?? 0}</span><span className="sr-only">当前页会话</span><span className="sr-only">{data?.list.length ?? 0}</span>{metricDefinitions.map(([key, label, unit]) => { const metric = overview.metrics[key] ?? { value: null, changeRate: null, status: 'unavailable', reason: '系统未返回该项数据' }; return <article className={`conversation-global-metric ${metric.status}`} key={key}><span>{label}</span><MetricValue {...metric} unit={unit} /><small>{metric.status === 'unavailable' ? (metric.reason ?? '暂无数据') : '当前筛选范围'}</small></article>; })}</section>}
    {isGlobal && <nav aria-label="会话状态快捷筛选" className="conversation-global-status-tabs">{statusTabs.map(([value, label]) => <button aria-pressed={currentFilters.bucket === value} key={value} onClick={() => changeBucket(value)} type="button">{label}</button>)}</nav>}
    {isGlobal && <nav aria-label="会话类型快捷筛选" className="conversation-global-type-tabs">{([['', '全部会话'], ['employee', '员工会话'], ['customer', '客户会话'], ['room', '群聊会话']] as const).map(([value, label]) => <button aria-pressed={currentFilters.conversationType === value} key={value || 'all'} onClick={() => setSearchParams(updateSearch(searchParams, { conversationType: value, page: 1, pageSize }))} type="button">{label}</button>)}</nav>}
    <form className="conversation-global-filters dashboard-filter-bar" onSubmit={applyFilters}>
      <label><span>关键词</span><input aria-label="关键词" onChange={(event) => setDraft((value) => ({ ...value, keyword: event.target.value }))} placeholder="搜索对象或消息内容" value={draft.keyword} /></label>
      <label><span>会话对象类型</span><select aria-label="会话对象类型" disabled={fixedConversationType !== undefined} onChange={(event) => setDraft((value) => ({ ...value, conversationType: event.target.value as ConversationSearch['conversationType'] }))} value={fixedConversationType ?? draft.conversationType}><option value="">全部</option><option value="employee">员工</option><option value="customer">客户</option><option value="room">群聊</option></select></label>
      <label><span>员工 ID</span><input aria-label="员工 ID" inputMode="numeric" onChange={(event) => setDraft((value) => ({ ...value, employeeIds: event.target.value }))} placeholder="多个 ID 用逗号分隔" value={draft.employeeIds} /></label>
      <label><span>开始日期</span><input aria-label="开始日期" onChange={(event) => setDraft((value) => ({ ...value, startAt: event.target.value }))} type="date" value={draft.startAt} /></label>
      <label><span>结束日期</span><input aria-label="结束日期" onChange={(event) => setDraft((value) => ({ ...value, endAt: event.target.value }))} type="date" value={draft.endAt} /></label>
      <label className="conversation-global-message-types"><span>消息类型</span><div className="conversation-global-message-types-picker"><button aria-expanded={messageTypeOpen} aria-haspopup="listbox" aria-label={draft.messageTypes.length > 0 ? `已选 ${draft.messageTypes.length} 项消息类型` : '选择消息类型'} onClick={() => setMessageTypeOpen((open) => !open)} type="button">{draft.messageTypes.length > 0 ? `已选 ${draft.messageTypes.length} 项消息类型` : '全部消息类型'}<span aria-hidden="true">⌄</span></button>{messageTypeOpen && <div aria-label="消息类型选项" className="conversation-global-message-types-menu" role="group">{messageTypeOptions.map(([value, label]) => <label key={value}><input aria-label={label} checked={draft.messageTypes.includes(value)} onChange={() => toggleMessageType(value)} type="checkbox" /><span>{label}</span></label>)}</div>}</div></label>
      <button type="submit">查询</button><button aria-label="刷新消息" disabled={listQuery.isFetching} onClick={() => void listQuery.refetch()} type="button">{listQuery.isFetching && !listQuery.isPending ? '刷新中…' : '刷新消息'}</button><button onClick={resetFilters} type="button">重置</button>
    </form>
    {filterError !== null && <p className="conversation-global-inline-error" role="alert">{filterError}</p>}{focusError !== null && <p className="conversation-global-inline-error" role="alert">{focusError}</p>}
    {overview?.capabilities.some((item) => !item.available) && <p className="conversation-global-capability-note" role="alert">部分能力暂无系统数据：{overview.capabilities.filter((item) => !item.available).map((item) => item.reason ?? item.key).join('；')}</p>}
    {(currentFilters.conversationType === 'room' || data?.list.some((item) => item.targetType === 'room') === true) && <p className="conversation-global-capability-note" role="note">群聊入站消息暂无法识别具体群成员，详情中统一显示“群成员”。</p>}
    {listQuery.isPending && <PageState state="loading" title="正在加载全局消息" />}{archiveUnauthorized && <ConversationArchiveUnavailableState />}{forbidden && <PageState description="请联系管理员开通会话存档和数据范围权限。" state="forbidden" title="无权查看当前企业会话" />}{listQuery.isError && !forbidden && !archiveUnauthorized && <PageState description={listQuery.error instanceof Error ? listQuery.error.message : '请稍后重试'} onRetry={() => void listQuery.refetch()} retryLabel="重试" state="error" title="全局消息加载失败" />}
    {data?.list.length === 0 && data.total > 0 && <PageState description="该页码已超出当前结果范围。" onRetry={() => changePage(1)} retryLabel="返回第一页" state="empty" title="当前页暂无会话" />}{data?.list.length === 0 && data.total === 0 && <PageState description="清除部分筛选条件后重新查询。" state="empty" title="当前筛选条件下暂无会话" />}
    {data !== undefined && data.list.length > 0 && <div className="conversation-global-results dashboard-data-card"><div className="conversation-global-table-wrap dashboard-table-scroll"><div aria-label="会话卡片列表" className="conversation-global-cards">{data.list.map((item) => <ConversationCard focusing={focusingID === item.id} item={item} key={item.id} onDetail={() => setSelectedID(item.id)} onFocus={() => void toggleFocus(item)} />)}</div></div><DashboardPagination page={page} pageSize={pageSize} total={data.total} onPageChange={changePage} /></div>}
    {selectedID !== null && <div className="conversation-global-drawer-backdrop" data-testid="conversation-global-drawer-backdrop" onClick={() => setSelectedID(null)}><aside aria-label="会话详情" className="conversation-global-drawer" onClick={(event) => event.stopPropagation()} role="dialog"><header><div><p>会话详情</p><h2>{detailQuery.data === undefined ? '正在读取会话' : `${detailQuery.data.employeeName || `员工 ${detailQuery.data.employeeId}`} · ${detailQuery.data.targetName || `${targetTypeLabel(detailQuery.data.targetType)} ${detailQuery.data.targetId}`}`}</h2></div><button onClick={() => setSelectedID(null)} type="button">关闭</button></header>{detailQuery.isPending && <PageState state="loading" title="正在加载会话详情" />}{detailNotFound && <PageState description="会话不存在或已无权访问。" state="not-found" title="会话不存在或已无权访问" />}{detailForbidden && <PageState description="当前账号的会话读取权限已失效。" state="forbidden" title="无权读取会话详情" />}{detailArchiveUnauthorized && <PageState description="请先完成会话内容存档授权。" state="forbidden" title="当前企业未开通会话内容存档" />}{detailQuery.isError && !detailNotFound && !detailForbidden && !detailArchiveUnauthorized && <PageState description={detailQuery.error instanceof Error ? detailQuery.error.message : '详情加载失败'} onRetry={() => void detailQuery.refetch()} retryLabel="重试详情" state="error" title="会话详情加载失败" />}{detailQuery.data !== undefined && <><p className="conversation-global-window-note">{detailQuery.data.truncated ? `当前显示最近 200 条，共 ${detailQuery.data.messageTotal} 条消息` : `共 ${detailQuery.data.messageTotal} 条消息`}</p><div className="conversation-global-messages">{detailQuery.data.messages.map((message: ConversationMessage) => <article className={`conversation-global-message ${message.direction}`} key={message.id}><header><strong>{message.senderName || '未知成员'}</strong><time>{message.sentAt}</time></header><ConversationMessageContent content={message.content} type={message.type} /></article>)}</div></>}</aside></div>}
  </section>;
}
