import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useDashboardAccess } from '../../app/access-context';
import { pageStateForError, PageState } from '../../components/page-state/page-state';
import type { ScrmApi } from './scrm-api';
import { useState } from 'react';

export function TagPage({ api }: { api: Pick<ScrmApi, 'listTags' | 'createTag' | 'bindTags'> }) {
  const access = useDashboardAccess();
  const corpId = Number(access.corp.id);
  const client = useQueryClient();
  const [name, setName] = useState('');
  const query = useQuery({ queryKey: ['scrm-tags', corpId], queryFn: () => api.listTags({ corpId }) });
  const create = useMutation({ mutationFn: () => api.createTag({ corpId, name: name.trim(), idempotencyKey: `tag-create-${name.trim()}` }), onSuccess: () => { setName(''); void client.invalidateQueries({ queryKey: ['scrm-tags', corpId] }); } });
  const bind = useMutation({ mutationFn: (tagId: string) => api.bindTags({ corpId, tagId, contactIds: ['c1'], idempotencyKey: `tag-bind-${tagId}-c1` }) });
  if (query.isPending) return <PageState state="loading" />;
  if (query.isError) return <PageState state={pageStateForError(query.error)} onRetry={() => void query.refetch()} />;
  if (query.data.items.length === 0) return <PageState state="empty" />;
  return <section><header className="dashboard-page-header"><h1>客户标签</h1></header><div className="dashboard-table-actions"><label>新标签<input aria-label="新标签" value={name} onChange={(event) => setName(event.target.value)} /></label><button type="button" disabled={!name.trim()} onClick={() => create.mutate()}>创建标签</button></div><div className="dashboard-data-card"><ul>{query.data.items.map((tag) => <li key={tag.id}>{tag.name}<button type="button" onClick={() => bind.mutate(tag.id)}>绑定到联系人</button></li>)}</ul></div></section>;
}
