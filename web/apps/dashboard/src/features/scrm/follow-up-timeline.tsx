import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { pageStateForError, PageState } from '../../components/page-state/page-state';
import type { ScrmApi } from './scrm-api';

export function FollowUpTimeline({ api, corpId, contactId }: { api: Pick<ScrmApi, 'listFollowUps' | 'appendFollowUp'>; corpId: number; contactId: string }) {
  const client = useQueryClient();
  const [content, setContent] = useState('');
  const query = useQuery({ queryKey: ['scrm-follow-ups', corpId, contactId], queryFn: () => api.listFollowUps({ corpId, contactId }) });
  const append = useMutation({ mutationFn: () => api.appendFollowUp({ corpId, contactId, content: content.trim(), idempotencyKey: `follow-up-${contactId}-${Date.now()}` }), onSuccess: () => { setContent(''); void client.invalidateQueries({ queryKey: ['scrm-follow-ups', corpId, contactId] }); } });
  if (query.isPending) return <section><h2>跟进记录</h2><PageState state="loading" /></section>;
  if (query.isError) return <section><h2>跟进记录</h2><PageState state={pageStateForError(query.error)} onRetry={() => void query.refetch()} /></section>;
  return <section><h2>跟进记录</h2><ul>{query.data.items.map((item) => <li key={item.id}>{item.createdAt}：{item.content}</li>)}</ul><label>跟进内容<textarea aria-label="跟进内容" value={content} onChange={(event) => setContent(event.target.value)} /></label><button type="button" disabled={!content.trim() || append.isPending} onClick={() => append.mutate()}>添加跟进</button>{append.isPending && <PageState state="loading" description="正在追加跟进记录…" />}{append.isError && <PageState state={pageStateForError(append.error)} description="跟进记录未追加，请重试。" onRetry={() => append.mutate()} />}</section>;
}
