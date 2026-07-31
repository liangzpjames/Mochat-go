import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { useDashboardAccess } from '../../app/access-context';
import { PageState } from '../../components/page-state/page-state';
import type { ScrmApi } from './scrm-api';

export function OpportunityPage({ api }: { api: Pick<ScrmApi, 'listOpportunities' | 'changeOpportunityStage'> }) {
  const access = useDashboardAccess();
  const corpId = Number(access.corp.id);
  const client = useQueryClient();
  const [stage, setStage] = useState('');
  const query = useQuery({ queryKey: ['scrm-opportunities', corpId, stage], queryFn: () => api.listOpportunities({ corpId, ...(stage ? { stage } : {}) }) });
  const change = useMutation({ mutationFn: (input: { opportunityId: string; version: number; toStage: string }) => api.changeOpportunityStage({ corpId, ...input, reason: '', idempotencyKey: `opportunity-${input.opportunityId}-${input.toStage}-${input.version}` }), onSuccess: () => void client.invalidateQueries({ queryKey: ['scrm-opportunities', corpId] }) });
  if (query.isPending) return <PageState state="loading" />;
  if (query.isError) return <PageState state="error" onRetry={() => void query.refetch()} />;
  return <section><h1>机会</h1><label>阶段<select aria-label="阶段" value={stage} onChange={(event) => setStage(event.target.value)}><option value="">全部</option><option value="proposal">意向</option><option value="won">赢单</option><option value="lost">输单</option></select></label><table><thead><tr><th>联系人</th><th>阶段</th><th>金额</th><th>操作</th></tr></thead><tbody>{query.data.items.map((item) => <tr key={item.id}><td>{item.contactId}</td><td>{item.stage}</td><td>{item.amount}</td><td><button type="button" disabled={item.status !== 'open'} onClick={() => change.mutate({ opportunityId: item.id, version: item.version, toStage: 'won' })}>赢单</button></td></tr>)}</tbody></table>{change.isError && <p role="alert">机会状态冲突，请刷新后重试</p>}</section>;
}
