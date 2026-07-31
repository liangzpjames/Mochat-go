import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import type { ScrmApi } from './scrm-api';

export function FollowUpTimeline({ api, corpId, contactId }: { api: Pick<ScrmApi, 'listFollowUps' | 'appendFollowUp'>; corpId: number; contactId: string }) {
  const client = useQueryClient();
  const [content, setContent] = useState('');
  const query = useQuery({ queryKey: ['scrm-follow-ups', corpId, contactId], queryFn: () => api.listFollowUps({ corpId, contactId }) });
  const append = useMutation({ mutationFn: () => api.appendFollowUp({ corpId, contactId, content: content.trim(), idempotencyKey: `follow-up-${contactId}-${Date.now()}` }), onSuccess: () => { setContent(''); void client.invalidateQueries({ queryKey: ['scrm-follow-ups', corpId, contactId] }); } });
  return <section><h2>跟进记录</h2><ul>{query.data?.items.map((item) => <li key={item.id}>{item.createdAt}：{item.content}</li>)}</ul><label>跟进内容<textarea aria-label="跟进内容" value={content} onChange={(event) => setContent(event.target.value)} /></label><button type="button" disabled={!content.trim() || append.isPending} onClick={() => append.mutate()}>添加跟进</button>{append.isError && <p role="alert">跟进内容无效或提交冲突</p>}</section>;
}
