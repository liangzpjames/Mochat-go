import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';
import type { FormEvent } from 'react';
import { useSearchParams } from 'react-router';

import { useDashboardAccess } from '../../app/access-context';
import { PageState } from '../../components/page-state/page-state';
import { DashboardPagination } from '../../components/dashboard-pagination';
import { updateSearch } from '../../shared/query-state';
import { ConversationArchiveUnavailableState, isConversationArchiveUnavailable } from './conversation-archive-state';
import type { ConversationGlobalApi, ConversationMessage, ConversationTargetType } from './conversation-global-api';

const pageSize = 20;

function positiveInteger(value: string | null, fallback: number): number {
  const parsed = Number(value);
  return Number.isInteger(parsed) && parsed > 0 ? parsed : fallback;
}

function messageText(message: ConversationMessage): string {
  const content = message.content.content;
  return typeof content === 'string' && content.trim() !== '' ? content : JSON.stringify(message.content);
}

function targetLabel(type: ConversationTargetType): string {
  return type === 'customer' ? '客户' : type === 'room' ? '客户群' : '同事';
}

export function EmployeeConversationPage({ api }: { api: ConversationGlobalApi }) {
  const access = useDashboardAccess();
  const [searchParams, setSearchParams] = useSearchParams();
  const [employeeDraft, setEmployeeDraft] = useState(searchParams.get('employeeKeyword') ?? '');
  const employeeKeyword = searchParams.get('employeeKeyword') ?? '';
  const selectedEmployeeId = searchParams.get('employeeId');
  const selectedConversationId = searchParams.get('conversationId');
  const conversationType = (searchParams.get('conversationType') ?? '') as '' | ConversationTargetType;
  const page = positiveInteger(searchParams.get('page'), 1);

  const employeeQuery = useQuery({
    queryKey: ['corp', access.corp.id, 'conversation-employees', employeeKeyword],
    queryFn: () => api.employees?.({ keyword: employeeKeyword }) ?? Promise.resolve([]),
  });
  const listQuery = useQuery({
    queryKey: ['corp', access.corp.id, 'employee-conversations', selectedEmployeeId, conversationType, page],
    queryFn: () => api.search({
      keyword: '', conversationType, employeeIds: selectedEmployeeId === null ? [] : [selectedEmployeeId],
      startAt: '', endAt: '', page, pageSize,
    }),
    enabled: selectedEmployeeId !== null,
  });
  const detailQuery = useQuery({
    queryKey: ['corp', access.corp.id, 'employee-conversation-detail', selectedConversationId],
    queryFn: () => api.detail(selectedConversationId ?? ''),
    enabled: selectedConversationId !== null,
    retry: false,
  });

  function submitEmployeeSearch(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSearchParams(updateSearch(searchParams, { employeeKeyword: employeeDraft, employeeId: undefined, conversationId: undefined, page: 1 }));
  }

  function selectEmployee(id: number) {
    setSearchParams(updateSearch(searchParams, { employeeId: id, conversationId: undefined, page: 1 }));
  }

  function selectType(type: '' | ConversationTargetType) {
    setSearchParams(updateSearch(searchParams, { conversationType: type, conversationId: undefined, page: 1 }));
  }

  function selectConversation(id: string) {
    setSearchParams(updateSearch(searchParams, { conversationId: id }));
  }

  const selectedEmployee = employeeQuery.data?.find((item) => String(item.id) === selectedEmployeeId);
  const employeeArchiveUnavailable = isConversationArchiveUnavailable(employeeQuery.error);
  const listArchiveUnavailable = isConversationArchiveUnavailable(listQuery.error);
  const detailArchiveUnavailable = isConversationArchiveUnavailable(detailQuery.error);

  return (
    <section className="employee-conversation-page">
      <header className="employee-conversation-header dashboard-data-card">
        <div>
          <p className="conversation-global-eyebrow">会话存档</p>
          <h1>员工会话</h1>
          <p>选择员工，查看该员工负责的客户、客户群和同事会话。</p>
        </div>
        <button type="button" onClick={() => void employeeQuery.refetch()} disabled={employeeQuery.isFetching}>刷新</button>
      </header>

      <div className="employee-conversation-workspace">
        <aside className="employee-conversation-employees dashboard-data-card" aria-label="员工列表">
          <header>
            <div><strong>企业架构</strong><span>{employeeQuery.data?.length ?? 0} 位员工</span></div>
          </header>
          <form className="employee-conversation-employee-search" onSubmit={submitEmployeeSearch}>
            <label htmlFor="employee-name">员工名称</label>
            <div><input id="employee-name" value={employeeDraft} onChange={(event) => setEmployeeDraft(event.target.value)} placeholder="请输入员工名称" /><button type="submit" aria-label="搜索员工">搜索</button></div>
          </form>
          {employeeQuery.isPending && <PageState state="loading" />}
          {employeeArchiveUnavailable && <ConversationArchiveUnavailableState />}
          {employeeQuery.error && !employeeArchiveUnavailable && <PageState state="error" onRetry={() => void employeeQuery.refetch()} />}
          {!employeeQuery.isPending && !employeeQuery.error && employeeQuery.data?.length === 0 && <PageState state="empty" title="暂无员工" description="当前搜索条件下没有员工。" />}
          <div className="employee-conversation-employee-list">
            {employeeQuery.data?.map((employee) => (
              <button className={String(employee.id) === selectedEmployeeId ? 'is-selected' : ''} key={employee.id} onClick={() => selectEmployee(employee.id)} type="button">
                <span className="employee-conversation-avatar">{employee.name.slice(0, 1)}</span><span>{employee.name}</span>
              </button>
            ))}
          </div>
          <footer>存档成员（{employeeQuery.data?.length ?? 0}）</footer>
        </aside>

        <section className="employee-conversation-list dashboard-data-card" aria-label="会话列表">
          <header className="employee-conversation-list-header"><div><strong>{selectedEmployee?.name ?? '请选择员工'}</strong><span>会话列表</span></div><button type="button" onClick={() => void listQuery.refetch()} disabled={!selectedEmployeeId || listQuery.isFetching}>刷新</button></header>
          <nav className="employee-conversation-tabs" aria-label="会话类型">
            {([['', '全部'], ['customer', '客户'], ['room', '客户群'], ['employee', '同事']] as const).map(([value, label]) => <button key={value || 'all'} aria-pressed={conversationType === value} onClick={() => selectType(value)} type="button">{label}</button>)}
          </nav>
          {!selectedEmployeeId && <PageState state="empty" title="请选择员工" description="从左侧员工列表选择员工后查看会话。" />}
          {selectedEmployeeId && listQuery.isPending && <PageState state="loading" />}
          {selectedEmployeeId && listArchiveUnavailable && <ConversationArchiveUnavailableState />}
          {selectedEmployeeId && listQuery.error && !listArchiveUnavailable && <PageState state="error" onRetry={() => void listQuery.refetch()} />}
          {selectedEmployeeId && !listQuery.isPending && !listQuery.error && listQuery.data?.list.length === 0 && <PageState state="empty" />}
          <div className="employee-conversation-items">
            {listQuery.data?.list.map((conversation) => <button className={conversation.id === selectedConversationId ? 'is-selected' : ''} key={conversation.id} onClick={() => selectConversation(conversation.id)} type="button"><strong>{conversation.targetName}</strong><span>{targetLabel(conversation.targetType)} · {conversation.sentAt}</span><p>{conversation.lastMessage || '暂无消息'}</p></button>)}
          </div>
          {selectedEmployeeId && listQuery.data && <DashboardPagination page={page} pageSize={pageSize} total={listQuery.data.total} onPageChange={(nextPage) => setSearchParams(updateSearch(searchParams, { page: nextPage, conversationId: undefined }))} />}
        </section>

        <section className="employee-conversation-detail dashboard-data-card" aria-label="会话详情">
          {!selectedConversationId && <PageState state="empty" title="请选择会话" description="选择中间列表中的会话查看消息详情。" />}
          {selectedConversationId && detailQuery.isPending && <PageState state="loading" />}
          {selectedConversationId && detailArchiveUnavailable && <ConversationArchiveUnavailableState />}
          {selectedConversationId && detailQuery.error && !detailArchiveUnavailable && <PageState state="error" onRetry={() => void detailQuery.refetch()} />}
          {detailQuery.data && <><header><div><strong>{detailQuery.data.targetName}</strong><span>{targetLabel(detailQuery.data.targetType)} · {detailQuery.data.employeeName}</span></div></header><div className="employee-conversation-messages">{detailQuery.data.messages.map((message) => <article className={message.direction} key={message.id}><header><strong>{message.senderName}</strong><time>{message.sentAt}</time></header><p>{messageText(message)}</p></article>)}</div>{detailQuery.data.truncated && <p className="conversation-global-window-note">仅展示最近消息</p>}</>}
        </section>
      </div>
    </section>
  );
}
