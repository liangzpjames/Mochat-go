import { keepPreviousData, useInfiniteQuery, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useMemo, useRef, useState } from 'react';
import type { FormEvent } from 'react';
import { useSearchParams } from 'react-router';

import { useDashboardAccess } from '../../app/access-context';
import { updateSearch } from '../../shared/query-state';
import type { CustomerConversationMode, ConversationGlobalApi, CustomerDirectoryMode, CustomerConversationProfile } from './conversation-global-api';
import { ConversationArchiveUnavailableState, isConversationArchiveUnavailable } from './conversation-archive-state';
import { CustomerConversationDetailPane } from './customer-conversation-detail';
import { CustomerConversationDirectory } from './customer-conversation-directory';
import { CustomerConversationList } from './customer-conversation-list';

const customerPageSize = 50 as const;
const conversationPageSize = 20 as const;
const messagePageSize = 50 as const;

function positiveInteger(value: string | null, fallback: number) {
  const parsed = Number(value);
  return Number.isInteger(parsed) && parsed > 0 ? parsed : fallback;
}

function optionalPositiveInteger(value: string | null) {
  if (value === null || value.trim() === '') return null;
  const parsed = Number(value);
  return Number.isInteger(parsed) && parsed > 0 ? parsed : null;
}

function asError(error: unknown): Error | null {
  return error instanceof Error ? error : error == null ? null : new Error('数据加载失败');
}

function setRepeated(next: URLSearchParams, key: string, values: readonly string[]) {
  next.delete(key);
  values.forEach((value) => next.append(key, value));
  return next;
}

function validConversationId(value: string | null) {
  return value !== null && /^\d+:[012]:\d+$/.test(value) ? value : null;
}

export function CustomerConversationPage({ api }: { api: ConversationGlobalApi }) {
  const access = useDashboardAccess();
  const queryClient = useQueryClient();
  const [searchParams, setSearchParams] = useSearchParams();
  const searchText = searchParams.toString();
  const rawCustomerMode = searchParams.get('customerMode');
  const customerMode: CustomerDirectoryMode = rawCustomerMode === 'focused' || rawCustomerMode === 'active' || rawCustomerMode === 'lost' ? rawCustomerMode : 'all';
  const customerKeyword = searchParams.get('customerKeyword') ?? '';
  const customerPage = positiveInteger(searchParams.get('customerPage'), 1);
  const rawCustomerId = searchParams.get('customerId');
  const customerId = optionalPositiveInteger(searchParams.get('customerId'));
  const rawConversationMode = searchParams.get('conversationMode');
  const conversationMode: CustomerConversationMode = rawConversationMode === 'group' ? 'group' : 'direct';
  const page = positiveInteger(searchParams.get('page'), 1);
  const rawConversationId = searchParams.get('conversationId');
  const conversationId = validConversationId(rawConversationId);
  const messageKeyword = searchParams.get('messageKeyword') ?? '';
  const messageDate = searchParams.get('messageDate') ?? '';
  const messageTypes = searchParams.getAll('messageTypes').filter(Boolean);
  const [customerKeywordDraft, setCustomerKeywordDraft] = useState(customerKeyword);
  const [messageKeywordDraft, setMessageKeywordDraft] = useState(messageKeyword);
  const autoSelectedConversationKey = useRef<string | null>(null);

  useEffect(() => { setCustomerKeywordDraft(customerKeyword); }, [customerKeyword]);
  useEffect(() => { setMessageKeywordDraft(messageKeyword); }, [messageKeyword, conversationId]);
  useEffect(() => {
    const next = new URLSearchParams(searchText);
    let changed = false;
    if (next.get('pageSize') !== String(conversationPageSize)) { next.set('pageSize', String(conversationPageSize)); changed = true; }
    if (rawCustomerMode !== null && !['all', 'focused', 'active', 'lost'].includes(rawCustomerMode)) { next.delete('customerMode'); changed = true; }
    if (rawConversationMode !== null && !['direct', 'group'].includes(rawConversationMode)) { next.delete('conversationMode'); changed = true; }
    if (rawCustomerId !== null && customerId === null) { next.delete('customerId'); next.delete('conversationId'); changed = true; }
    if (rawConversationId !== null && conversationId === null) { next.delete('conversationId'); changed = true; }
    if (String(customerPage) !== (next.get('customerPage') ?? '1')) { next.set('customerPage', String(customerPage)); changed = true; }
    if (String(page) !== (next.get('page') ?? '1')) { next.set('page', String(page)); changed = true; }
    if (changed) setSearchParams(next, { replace: true });
  }, [conversationId, customerId, customerPage, page, rawConversationId, rawCustomerId, rawConversationMode, rawCustomerMode, searchText, setSearchParams]);

  const directoryQuery = useQuery({
    queryKey: ['corp', access.corp.id, 'customer-directory', customerMode, customerKeyword, customerPage],
    queryFn: () => api.customerDirectory ? api.customerDirectory({ mode: customerMode, keyword: customerKeyword, page: customerPage, pageSize: customerPageSize }) : Promise.reject(new Error('客户目录接口未接入')),
    placeholderData: keepPreviousData,
    retry: false,
  });
  const conversationsQuery = useQuery({
    queryKey: ['corp', access.corp.id, 'customer-conversations', customerId, conversationMode, page],
    queryFn: () => api.customerConversations ? api.customerConversations({ customerId: customerId!, mode: conversationMode, page, pageSize: conversationPageSize }) : Promise.reject(new Error('客户会话接口未接入')),
    enabled: customerId !== null,
    placeholderData: keepPreviousData,
    retry: false,
  });
  const detailQuery = useInfiniteQuery({
    queryKey: ['corp', access.corp.id, 'customer-detail', customerId, conversationId, messageKeyword, messageTypes, messageDate],
    initialPageParam: '',
    queryFn: ({ pageParam }) => api.customerDetail ? api.customerDetail({ customerId: customerId!, conversationId: conversationId!, keyword: messageKeyword, messageTypes, date: messageDate, pageSize: messagePageSize, ...(pageParam ? { before: pageParam } : {}) }) : Promise.reject(new Error('客户会话详情接口未接入')),
    getNextPageParam: (lastPage) => lastPage.hasMore && lastPage.nextBefore ? lastPage.nextBefore : undefined,
    enabled: customerId !== null && conversationId !== null,
    placeholderData: keepPreviousData,
    retry: false,
  });
  const directoryData = directoryQuery.data;
  const conversationData = conversationsQuery.data;
  const selectedCustomer = useMemo<CustomerConversationProfile | undefined>(() => {
    if (customerId === null) return undefined;
    const directoryCustomer = directoryData?.customers.find((item) => item.id === customerId);
    return directoryCustomer
      ? { id: directoryCustomer.id, name: directoryCustomer.name, avatar: directoryCustomer.avatar, profileStatus: directoryCustomer.profileStatus }
      : { id: customerId, name: `客户 ${customerId}`, avatar: '', profileStatus: 'missing' };
  }, [customerId, directoryData]);
  useEffect(() => {
    if (customerId === null || conversationData === undefined || conversationData.customer.id !== customerId || conversationData.mode !== conversationMode) return;
    const key = `${customerId}:${conversationMode}`;
    if (autoSelectedConversationKey.current === key) return;
    autoSelectedConversationKey.current = key;
    if (conversationId === null) {
      const firstConversationId = conversationData.list.find((item) => item.conversationId?.trim())?.conversationId;
      if (firstConversationId) setSearchParams(updateSearch(searchParams, { conversationId: firstConversationId }));
    }
  }, [conversationData, conversationId, conversationMode, customerId, searchParams, setSearchParams]);
  const detailData = detailQuery.data?.pages[0];
  const detailMessages = useMemo(() => {
    const seen = new Set<string>();
    const result: NonNullable<typeof detailData>['messages'][number][] = [];
    for (const pageData of [...(detailQuery.data?.pages ?? [])].reverse()) {
      for (const message of pageData.messages) if (!seen.has(message.id)) { seen.add(message.id); result.push(message); }
    }
    return result;
  }, [detailQuery.data?.pages]);
  const [directoryOpen, setDirectoryOpen] = useState(false);
  const [focusError, setFocusError] = useState<string | null>(null);
  const focusMutation = useMutation({
    mutationFn: async () => {
      if (!detailData || !conversationId || !api.setFocus || !api.removeFocus) throw new Error('重点关注能力未接入');
      await (detailData.focused ? api.removeFocus(conversationId) : api.setFocus(conversationId));
    },
    onSuccess: async () => {
      setFocusError(null);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['corp', access.corp.id, 'customer-directory'] }),
        queryClient.invalidateQueries({ queryKey: ['corp', access.corp.id, 'customer-conversations', customerId] }),
        queryClient.invalidateQueries({ queryKey: ['corp', access.corp.id, 'customer-detail', customerId, conversationId] }),
      ]);
    },
    onError: (error) => setFocusError(error instanceof Error ? error.message : '重点关注保存失败'),
  });

  function submitCustomerSearch(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSearchParams(updateSearch(searchParams, { customerKeyword: customerKeywordDraft, customerPage: 1, customerId: undefined, conversationId: undefined, page: 1, pageSize: conversationPageSize }));
  }
  function changeCustomerMode(mode: CustomerDirectoryMode) {
    setDirectoryOpen(false);
    setSearchParams(updateSearch(searchParams, { customerMode: mode === 'all' ? undefined : mode, customerPage: 1, customerId: undefined, conversationId: undefined, page: 1, pageSize: conversationPageSize }));
  }
  function selectCustomer(id: number) {
    setDirectoryOpen(false);
    setSearchParams(updateSearch(searchParams, { customerId: id, conversationId: undefined, page: 1, pageSize: conversationPageSize }));
  }
  function changeConversationMode(mode: CustomerConversationMode) {
    setSearchParams(updateSearch(searchParams, { conversationMode: mode, page: 1, conversationId: undefined, pageSize: conversationPageSize }));
  }
  function selectConversation(id: string) { setSearchParams(updateSearch(searchParams, { conversationId: id })); }
  function submitMessageSearch(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSearchParams(updateSearch(searchParams, { messageKeyword: messageKeywordDraft }));
  }
  function toggleMessageType(type: string) {
    const nextTypes = messageTypes.includes(type) ? messageTypes.filter((item) => item !== type) : [...messageTypes, type];
    setSearchParams(setRepeated(new URLSearchParams(searchParams), 'messageTypes', nextTypes));
  }

  const archiveError = [directoryQuery.error, conversationsQuery.error, detailQuery.error].find(isConversationArchiveUnavailable);
  if (archiveError !== undefined) return <ConversationArchiveUnavailableState />;

  return <section className={`customer-conversation-page ${customerId !== null ? 'has-customer' : ''} ${conversationId !== null ? 'has-conversation' : ''}`}>
    {conversationId !== null && <button className="customer-conversation-mobile-back" onClick={() => setSearchParams(updateSearch(searchParams, { conversationId: undefined }))} type="button">返回会话列表</button>}
    {customerId !== null && conversationId === null && <button className="customer-conversation-mobile-back" onClick={() => setDirectoryOpen(true)} type="button">重新选择客户</button>}
    <button className="customer-conversation-directory-trigger" onClick={() => setDirectoryOpen(true)} type="button">选择客户</button>
    {directoryOpen && <button aria-label="关闭客户目录" className="customer-conversation-directory-backdrop" onClick={() => setDirectoryOpen(false)} type="button" />}
    <div className={`customer-conversation-workspace ${directoryOpen ? 'is-directory-open' : ''}`}>
      <CustomerConversationDirectory
        data={directoryData} error={asError(directoryQuery.error)} fetching={directoryQuery.isFetching} keywordDraft={customerKeywordDraft} mode={customerMode}
        onKeywordDraftChange={setCustomerKeywordDraft} onModeChange={changeCustomerMode} onPageChange={(nextPage) => setSearchParams(updateSearch(searchParams, { customerPage: nextPage }))} page={customerPage}
        onRefresh={() => void directoryQuery.refetch()} onSearch={submitCustomerSearch} onSelectCustomer={selectCustomer} pending={directoryQuery.isPending} selectedCustomerId={customerId}
      />
      <CustomerConversationList
        customer={selectedCustomer} data={conversationData} error={asError(conversationsQuery.error)} fetching={conversationsQuery.isFetching} mode={conversationMode} onModeChange={changeConversationMode}
        onPageChange={(nextPage) => setSearchParams(updateSearch(searchParams, { page: nextPage, conversationId: undefined, pageSize: conversationPageSize }))} onRefresh={() => void conversationsQuery.refetch()}
        onSelectConversation={selectConversation} page={page} pending={conversationsQuery.isPending} selectedConversationId={conversationId}
      />
      <CustomerConversationDetailPane
        data={detailData} date={messageDate} error={asError(detailQuery.error)} fetching={detailQuery.isFetching} focusError={focusError} focusPending={focusMutation.isPending}
        hasMore={detailQuery.hasNextPage} keyword={messageKeywordDraft} loadingOlder={detailQuery.isFetchingNextPage} messageTypes={messageTypes} messages={detailMessages}
        onDateChange={(value) => setSearchParams(updateSearch(searchParams, { messageDate: value }))} onKeywordChange={setMessageKeywordDraft} onKeywordSearch={submitMessageSearch}
        onLoadOlder={() => void detailQuery.fetchNextPage()} onRefresh={() => void detailQuery.refetch()} onToggleFocus={() => focusMutation.mutate()} onToggleMessageType={toggleMessageType}
        pending={conversationId !== null && detailQuery.isPending}
      />
    </div>
  </section>;
}
