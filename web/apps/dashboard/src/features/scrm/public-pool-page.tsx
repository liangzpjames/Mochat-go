import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useDashboardAccess } from '../../app/access-context';
import { pageStateForError, PageState } from '../../components/page-state/page-state';
import type { ScrmApi } from './scrm-api';

export function PublicPoolPage({ api }: { api: Pick<ScrmApi, 'listPublicPool' | 'claimFromPublicPool'> }) {
  const access = useDashboardAccess();
  const queryClient = useQueryClient();
  const query = useQuery({ queryKey: ['scrm-public-pool', access.corp.id], queryFn: () => api.listPublicPool({ corpId: Number(access.corp.id), pageSize: 20 }) });
  const claim = useMutation({ mutationFn: (item: { contactId: string; version: number }) => api.claimFromPublicPool({ corpId: Number(access.corp.id), ...item, idempotencyKey: `claim-${item.contactId}-${item.version}` }), onSuccess: () => void queryClient.invalidateQueries({ queryKey: ['scrm-public-pool', access.corp.id] }) });
  if (query.isPending) return <PageState state="loading" />;
  if (query.isError) return <PageState state={pageStateForError(query.error)} onRetry={() => void query.refetch()} />;
  if (query.data.items.length === 0) return <PageState state="empty" />;
  const claimState = claim.isError ? pageStateForError(claim.error) : null;
  return <section className="scrm-public-pool-page"><header className="dashboard-page-header"><h1>客户公海</h1><p>领取后将成为当前账号负责的客户。</p></header><div className="dashboard-data-card"><div className="dashboard-table-scroll"><table><thead><tr><th>客户</th><th>状态</th><th>版本</th><th>操作</th></tr></thead><tbody>{query.data.items.map((item) => <tr key={item.id}><td>{item.contactId}</td><td>{item.status}</td><td>{item.version}</td><td><button type="button" disabled={claim.isPending} onClick={() => claim.mutate({ contactId: item.contactId, version: item.version })}>领取</button></td></tr>)}</tbody></table></div></div>{claimState === 'conflict' && <PageState description="领取冲突，请刷新后重试。" state="conflict" />}{claimState === 'error' && <p role="alert">领取冲突，请刷新后重试。</p>}</section>;
}
