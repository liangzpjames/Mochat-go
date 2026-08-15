import { useQuery } from '@tanstack/react-query';
import { ApiError } from '@mochat/api-client';
import { useState } from 'react';
import type { FormEvent } from 'react';
import { useSearchParams } from 'react-router';

import { useDashboardAccess } from '../../app/access-context';
import { PageState } from '../../components/page-state/page-state';
import { updateSearch } from '../../shared/query-state';
import type { ConversationGlobalApi, ConversationTargetType } from './conversation-global-api';
import { messageText } from './conversation-global-api';

function targetLabel(type: ConversationTargetType): string {
  return type === 'customer' ? '客户' : type === 'room' ? '群聊' : '同事';
}

function isArchiveUnauthorized(error: unknown): boolean {
  return error instanceof ApiError && error.status === 403 && error.code === 40301;
}

export function ConversationTrajectoryPage({ api }: { api: ConversationGlobalApi }) {
  const access = useDashboardAccess();
  const [searchParams, setSearchParams] = useSearchParams();
  const [keyword, setKeyword] = useState(searchParams.get('keyword') ?? '');
  const selectedId = searchParams.get('conversationId');
  const query = useQuery({
    queryKey: ['corp', access.corp.id, 'conversation-trajectory', searchParams.get('keyword') ?? ''],
    queryFn: () => api.search({
      keyword: searchParams.get('keyword') ?? '', conversationType: '', employeeIds: [],
      startAt: '', endAt: '', page: 1, pageSize: 50,
    }),
  });
  const detail = useQuery({
    queryKey: ['corp', access.corp.id, 'conversation-trajectory-detail', selectedId],
    queryFn: () => api.detail(selectedId ?? ''),
    enabled: selectedId !== null,
    retry: false,
  });
  const archiveUnauthorized = isArchiveUnauthorized(query.error);

  function applySearch(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSearchParams(updateSearch(searchParams, { keyword, conversationId: undefined }));
  }

  return (
    <section className="conversation-trajectory-page">
      <header className="conversation-global-header dashboard-data-card">
        <div><p className="conversation-global-eyebrow">会话存档</p><h1>会话轨迹</h1><p>选择会话，按时间查看可追溯的消息时间线。</p></div>
        <button type="button" onClick={() => void query.refetch()} disabled={query.isFetching}>刷新</button>
      </header>
      <form className="conversation-trajectory-search dashboard-data-card" onSubmit={applySearch}>
        <label htmlFor="trajectory-keyword">关键词</label>
        <input id="trajectory-keyword" value={keyword} onChange={(event) => setKeyword(event.target.value)} placeholder="搜索对象或消息内容" />
        <button type="submit">查询</button>
      </form>
      <div className="conversation-trajectory-workspace">
        <section className="conversation-trajectory-list dashboard-data-card" aria-label="轨迹会话列表">
          <header><strong>会话列表</strong><span>{query.data?.total ?? 0} 条结果</span></header>
          {query.isPending && <PageState state="loading" />}
          {archiveUnauthorized && <PageState state="forbidden" title="当前企业未开通会话内容存档" description="请先在企业微信完成会话内容存档授权并启用归档同步。" />}
          {query.error && !archiveUnauthorized && <PageState state="error" onRetry={() => void query.refetch()} />}
          {!query.isPending && !query.error && query.data?.list.length === 0 && <PageState state="empty" />}
          <div>{query.data?.list.map((item) => <button className={item.id === selectedId ? 'is-selected' : ''} key={item.id} onClick={() => setSearchParams(updateSearch(searchParams, { conversationId: item.id }))} type="button"><strong>{item.targetName}</strong><span>{targetLabel(item.targetType)} · {item.employeeName}</span><p>{item.lastMessage || '暂无消息'}</p><time>{item.sentAt}</time></button>)}</div>
        </section>
        <section className="conversation-trajectory-detail dashboard-data-card" aria-label="消息时间线">
          {!selectedId && <PageState state="empty" title="请选择会话" description="从左侧列表选择会话查看消息时间线。" />}
          {selectedId && detail.isPending && <PageState state="loading" />}
          {selectedId && detail.error && <PageState state="error" onRetry={() => void detail.refetch()} />}
          {detail.data && <><header><div><strong>{detail.data.targetName}</strong><span>{targetLabel(detail.data.targetType)} · {detail.data.employeeName} · 共 {detail.data.messageTotal} 条消息</span></div></header><div className="conversation-trajectory-timeline">{detail.data.messages.map((message) => <article key={message.id}><div className={`conversation-trajectory-dot ${message.direction}`} /><div className="conversation-trajectory-event"><header><strong>{message.senderName}</strong><span>{message.direction === 'outbound' ? '发送' : '收到'}</span><time>{message.sentAt}</time></header><p>{messageText(message)}</p></div></article>)}</div>{detail.data.truncated && <p className="conversation-global-window-note">仅展示最近消息</p>}</>}
        </section>
      </div>
    </section>
  );
}
