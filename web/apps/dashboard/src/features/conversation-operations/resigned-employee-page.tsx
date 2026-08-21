import { keepPreviousData, useInfiniteQuery, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useMemo, useState } from 'react';
import type { FormEvent } from 'react';
import { useSearchParams } from 'react-router';

import { useDashboardAccess } from '../../app/access-context';
import { updateSearch } from '../../shared/query-state';
import type { ConversationGlobalApi, ConversationTargetType, StaffDirectoryMode } from '../conversation-global/conversation-global-api';
import { EmployeeConversationDetail } from '../conversation-global/employee-conversation-detail';
import { EmployeeConversationDirectory } from '../conversation-global/employee-conversation-directory';
import { EmployeeConversationList } from '../conversation-global/employee-conversation-list';
import { ConversationOperationsShell, ConversationQueryBar } from './conversation-operations-shell';

const conversationPageSize = 20 as const;
const employeePageSize = 50 as const;
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

export function ResignedEmployeePage({ api }: { api: ConversationGlobalApi }) {
  const access = useDashboardAccess();
  const queryClient = useQueryClient();
  const [searchParams, setSearchParams] = useSearchParams();
  const searchText = searchParams.toString();
  const departmentId = optionalPositiveInteger(searchParams.get('departmentId'));
  const selectedEmployeeId = optionalPositiveInteger(searchParams.get('employeeId'));
  const rawConversationId = searchParams.get('conversationId');
  const selectedConversationId = rawConversationId !== null && /^\d+:[012]:\d+$/.test(rawConversationId) ? rawConversationId : null;
  const selectedConversationRowId = searchParams.get('conversationRowId');
  const rawType = searchParams.get('conversationType');
  const conversationType: '' | ConversationTargetType = rawType === 'customer' || rawType === 'room' || rawType === 'employee' ? rawType : '';
  const page = positiveInteger(searchParams.get('page'), 1);
  const employeeKeyword = searchParams.get('employeeKeyword') ?? '';
  const startAt = searchParams.get('startAt') ?? '';
  const endAt = searchParams.get('endAt') ?? '';
  const detailKeyword = searchParams.get('messageKeyword') ?? '';
  const detailDate = searchParams.get('messageDate') ?? '';
  const detailMessageTypes = searchParams.getAll('messageTypes').filter(Boolean);
  const [employeeKeywordDraft, setEmployeeKeywordDraft] = useState(employeeKeyword);
  const [detailKeywordDraft, setDetailKeywordDraft] = useState(detailKeyword);
  const [startDraft, setStartDraft] = useState(startAt);
  const [endDraft, setEndDraft] = useState(endAt);
  const [directoryOpen, setDirectoryOpen] = useState(false);
  const [focusError, setFocusError] = useState<string | null>(null);

  useEffect(() => { setEmployeeKeywordDraft(employeeKeyword); }, [employeeKeyword]);
  useEffect(() => { setDetailKeywordDraft(detailKeyword); }, [detailKeyword, selectedConversationId]);
  useEffect(() => { setStartDraft(startAt); setEndDraft(endAt); }, [startAt, endAt]);
  useEffect(() => {
    const next = new URLSearchParams(searchText);
    let changed = false;
    if (next.get('pageSize') !== String(conversationPageSize)) { next.set('pageSize', String(conversationPageSize)); changed = true; }
    if (rawType !== null && !['customer', 'room', 'employee'].includes(rawType)) { next.delete('conversationType'); changed = true; }
    if (rawConversationId !== null && selectedConversationId === null) { next.delete('conversationId'); next.delete('conversationRowId'); changed = true; }
    if (String(page) !== (next.get('page') ?? '1')) { next.set('page', String(page)); changed = true; }
    if (changed) setSearchParams(next, { replace: true });
  }, [page, rawConversationId, rawType, searchText, selectedConversationId, setSearchParams]);
  useEffect(() => {
    function closeOnEscape(event: KeyboardEvent) { if (event.key === 'Escape') setDirectoryOpen(false); }
    window.addEventListener('keydown', closeOnEscape);
    return () => window.removeEventListener('keydown', closeOnEscape);
  }, []);

  const directoryQuery = useInfiniteQuery({
    queryKey: ['corp', access.corp.id, 'resigned-staff-directory', employeeKeyword, departmentId],
    initialPageParam: 1,
    placeholderData: keepPreviousData,
    queryFn: ({ pageParam }) => {
      if (!api.staffDirectory) return Promise.reject(new Error('员工目录加载失败'));
      return api.staffDirectory({ mode: 'departed', keyword: employeeKeyword, departmentId, page: pageParam, pageSize: employeePageSize });
    },
    getNextPageParam: (lastPage, pages) => pages.reduce((sum, item) => sum + item.employees.length, 0) < lastPage.total ? pages.length + 1 : undefined,
  });
  const directoryPages = directoryQuery.data?.pages ?? [];
  const directoryData = directoryPages[0];
  const employees = useMemo(() => directoryPages.flatMap((item) => item.employees), [directoryPages]);
  const selectedEmployee = employees.find((item) => item.id === selectedEmployeeId);

  const listQuery = useQuery({
    queryKey: ['corp', access.corp.id, 'resigned-staff-conversations', selectedEmployeeId, conversationType, startAt, endAt, page],
    queryFn: () => api.search({ keyword: '', conversationType, employeeIds: selectedEmployeeId === null ? [] : [String(selectedEmployeeId)], startAt, endAt, page, pageSize: conversationPageSize }),
    enabled: selectedEmployeeId !== null,
  });

  const detailQuery = useInfiniteQuery({
    queryKey: ['corp', access.corp.id, 'resigned-staff-detail', selectedConversationId, detailKeyword, detailMessageTypes, detailDate],
    initialPageParam: '',
    placeholderData: keepPreviousData,
    queryFn: ({ pageParam }) => {
      if (!api.staffDetail || !selectedConversationId) return Promise.reject(new Error('会话详情加载失败'));
      const input = { conversationId: selectedConversationId, keyword: detailKeyword, messageTypes: detailMessageTypes, date: detailDate, pageSize: messagePageSize } as const;
      return api.staffDetail(pageParam ? { ...input, before: pageParam } : input);
    },
    getNextPageParam: (lastPage) => lastPage.hasMore && lastPage.nextBefore ? lastPage.nextBefore : undefined,
    enabled: selectedConversationId !== null,
    retry: false,
  });
  const detailData = detailQuery.data?.pages[0];
  const detailMessages = useMemo(() => {
    const seen = new Set<string>();
    const result = [] as NonNullable<typeof detailData>['messages'][number][];
    for (const detailPage of [...(detailQuery.data?.pages ?? [])].reverse()) {
      for (const message of detailPage.messages) if (!seen.has(message.id)) { seen.add(message.id); result.push(message); }
    }
    return result;
  }, [detailData, detailQuery.data?.pages]);

  const focusMutation = useMutation({
    mutationFn: async () => {
      if (!detailData || !selectedConversationId || !api.setFocus || !api.removeFocus) throw new Error('重点关注保存失败');
      await (detailData.focused ? api.removeFocus(selectedConversationId) : api.setFocus(selectedConversationId));
    },
    onSuccess: async () => {
      setFocusError(null);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['corp', access.corp.id, 'resigned-staff-directory'] }),
        queryClient.invalidateQueries({ queryKey: ['corp', access.corp.id, 'resigned-staff-conversations', selectedEmployeeId] }),
        queryClient.invalidateQueries({ queryKey: ['corp', access.corp.id, 'resigned-staff-detail', selectedConversationId] }),
      ]);
    },
    onError: (error) => setFocusError(error instanceof Error ? error.message : '重点关注保存失败'),
  });

  function submitEmployeeSearch(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSearchParams(updateSearch(searchParams, { employeeKeyword: employeeKeywordDraft, employeeId: undefined, conversationId: undefined, conversationRowId: undefined, page: 1, pageSize: conversationPageSize }));
  }
  function changeDepartment(nextDepartmentId: number | null) {
    setSearchParams(updateSearch(searchParams, { departmentId: nextDepartmentId ?? undefined, employeeId: undefined, conversationId: undefined, conversationRowId: undefined, page: 1, pageSize: conversationPageSize }));
  }
  function selectEmployee(id: number) {
    setDirectoryOpen(false);
    setSearchParams(updateSearch(searchParams, { employeeId: id, conversationId: undefined, conversationRowId: undefined, page: 1, pageSize: conversationPageSize }));
  }
  function applyFilters() {
    setSearchParams(updateSearch(searchParams, { startAt: startDraft, endAt: endDraft, page: 1, conversationId: undefined, conversationRowId: undefined, pageSize: conversationPageSize }));
  }
  function resetFilters() {
    setStartDraft(''); setEndDraft('');
    setSearchParams(updateSearch(searchParams, { startAt: undefined, endAt: undefined, page: 1, conversationId: undefined, conversationRowId: undefined, pageSize: conversationPageSize }));
  }
  function selectType(type: '' | ConversationTargetType) {
    setSearchParams(updateSearch(searchParams, { conversationType: type, conversationId: undefined, conversationRowId: undefined, page: 1, pageSize: conversationPageSize }));
  }
  function selectConversation(id: string, rowId: string) { setSearchParams(updateSearch(searchParams, { conversationId: id, conversationRowId: rowId })); }
  function submitDetailKeyword(event: FormEvent<HTMLFormElement>) { event.preventDefault(); setSearchParams(updateSearch(searchParams, { messageKeyword: detailKeywordDraft })); }
  function changeDetailFilter(key: 'messageKeyword' | 'messageDate', value: string) { setSearchParams(updateSearch(searchParams, { [key]: value })); }
  function toggleMessageType(type: string) {
    const nextTypes = detailMessageTypes.includes(type) ? detailMessageTypes.filter((item) => item !== type) : [...detailMessageTypes, type];
    setSearchParams(setRepeated(new URLSearchParams(searchParams), 'messageTypes', nextTypes));
  }

  return (
    <ConversationOperationsShell title="离职员工" description="查看离职员工的真实归档会话和历史消息">
      <ConversationQueryBar fetching={directoryQuery.isFetching || listQuery.isFetching} onQuery={applyFilters} onReset={resetFilters} onRefresh={() => { void directoryQuery.refetch(); void listQuery.refetch(); }}>
        <label>开始日期<input aria-label="会话开始日期" type="date" value={startDraft} onChange={(event) => setStartDraft(event.target.value)} /></label>
        <label>结束日期<input aria-label="会话结束日期" type="date" value={endDraft} onChange={(event) => setEndDraft(event.target.value)} /></label>
      </ConversationQueryBar>
      <section className={`employee-conversation-page conversation-operations-card ${selectedEmployeeId !== null ? 'has-employee' : ''} ${selectedConversationId !== null ? 'has-conversation' : ''}`}>
        {selectedConversationId !== null && <button className="employee-conversation-mobile-back" onClick={() => setSearchParams(updateSearch(searchParams, { conversationId: undefined, conversationRowId: undefined }))} type="button">返回会话列表</button>}
        {selectedEmployeeId !== null && selectedConversationId === null && <button className="employee-conversation-mobile-back" onClick={() => setDirectoryOpen(true)} type="button">重新选择员工</button>}
        <button className="employee-conversation-directory-trigger" onClick={() => setDirectoryOpen(true)} type="button">选择员工</button>
        {directoryOpen && <button aria-label="关闭员工目录" className="employee-conversation-directory-backdrop" onClick={() => setDirectoryOpen(false)} type="button" />}
        <div className={`employee-conversation-workspace resigned-employee-workspace ${directoryOpen ? 'is-directory-open' : ''}`}>
          <EmployeeConversationDirectory
            data={directoryData} employees={employees} error={asError(directoryQuery.error)} fetching={directoryQuery.isFetching}
            hasMore={directoryQuery.hasNextPage} keywordDraft={employeeKeywordDraft} lockedMode="departed" mode="departed"
            onDepartmentChange={changeDepartment} onKeywordDraftChange={setEmployeeKeywordDraft}
            onLoadMore={() => void directoryQuery.fetchNextPage()} onModeChange={() => undefined} onRefresh={() => void directoryQuery.refetch()}
            onSearch={submitEmployeeSearch} onSelectEmployee={selectEmployee} pending={directoryQuery.isPending}
            selectedDepartmentId={departmentId} selectedEmployeeId={selectedEmployeeId}
          />
          <EmployeeConversationList
            data={listQuery.data} employee={selectedEmployee} error={asError(listQuery.error)} fetching={listQuery.isFetching}
            allowedTypes={['', 'customer', 'room', 'employee']} internalGroupReason="" onPageChange={(nextPage) => setSearchParams(updateSearch(searchParams, { page: nextPage, conversationId: undefined, conversationRowId: undefined, pageSize: conversationPageSize }))}
            onRefresh={() => void listQuery.refetch()} onSelectConversation={selectConversation} onTypeChange={selectType}
            page={page} pending={listQuery.isPending} selectedConversationId={selectedConversationId} selectedConversationRowId={selectedConversationRowId} type={conversationType}
          />
          <EmployeeConversationDetail
            data={detailData} date={detailDate} error={asError(detailQuery.error)} fetching={detailQuery.isFetching}
            focusError={focusError} focusPending={focusMutation.isPending} hasMore={detailQuery.hasNextPage}
            keyword={detailKeywordDraft} loadingOlder={detailQuery.isFetchingNextPage} messageTypes={detailMessageTypes}
            messages={detailMessages} onDateChange={(value) => changeDetailFilter('messageDate', value)} onKeywordSearch={submitDetailKeyword}
            onKeywordChange={setDetailKeywordDraft} onLoadOlder={() => void detailQuery.fetchNextPage()}
            onRefresh={() => void detailQuery.refetch()} onToggleFocus={() => focusMutation.mutate()}
            onToggleMessageType={toggleMessageType} pending={selectedConversationId !== null && detailQuery.isPending}
          />
        </div>
      </section>
    </ConversationOperationsShell>
  );
}
