import { useEffect, useMemo, useRef } from 'react';
import { useInfiniteQuery } from '@tanstack/react-query';
import type { ConversationGlobalApi, ConversationMessage, StaffConversationDetail } from './conversation-global-api';
import { messageText } from './conversation-global-api';

type Props = { api: ConversationGlobalApi; conversationId: string; date: string; eventHour?: string; onClose(): void };

function mergePages(pages: readonly StaffConversationDetail[]): readonly ConversationMessage[] {
  const byId = new Map<string, ConversationMessage>();
  for (const page of [...pages].reverse()) for (const message of page.messages) byId.set(message.id, message);
  return [...byId.values()].sort((a, b) => a.sentAt.localeCompare(b.sentAt) || a.id.localeCompare(b.id));
}

export function ConversationTrajectoryDrawer({ api, conversationId, date, onClose }: Props) {
  const closeRef = useRef<HTMLButtonElement>(null);
  const query = useInfiniteQuery({
    queryKey: ['trajectory-detail', conversationId, date],
    queryFn: ({ pageParam }) => api.staffDetail!({ conversationId, keyword: '', messageTypes: [], date, pageSize: 50, ...(pageParam ? { before: pageParam } : {}) }),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (lastPage) => lastPage.hasMore ? lastPage.nextBefore : undefined,
    enabled: api.staffDetail !== undefined,
  });
  const messages = useMemo(() => mergePages(query.data?.pages ?? []), [query.data?.pages]);
  useEffect(() => { closeRef.current?.focus(); const onKey = (event: KeyboardEvent) => { if (event.key === 'Escape') onClose(); }; document.addEventListener('keydown', onKey); return () => document.removeEventListener('keydown', onKey); }, [onClose]);
  return <div className="conversation-trajectory-drawer-layer" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget) onClose(); }}><aside className="conversation-trajectory-drawer" role="dialog" aria-modal="true" aria-label="会话详情"><header><div><strong>{query.data?.pages[0]?.targetName ?? '会话详情'}</strong><small>{date}</small></div><button type="button" ref={closeRef} onClick={onClose} aria-label="关闭会话详情">关闭</button></header>{query.isPending && <p role="status">正在加载消息…</p>}{query.error && <p role="alert">消息加载失败，请重试。</p>}{!query.isPending && !query.error && messages.length === 0 && <p role="status">活动摘要与消息详情不一致，请重新加载。</p>}{query.hasNextPage && <button type="button" onClick={() => void query.fetchNextPage()} disabled={query.isFetchingNextPage}>加载更早</button>}<div className="conversation-trajectory-messages">{messages.map((message) => <article key={message.id} className={message.direction}><header><strong>{message.senderName}</strong><time>{message.sentAt}</time></header><p>{messageText(message)}</p></article>)}</div></aside></div>;
}
