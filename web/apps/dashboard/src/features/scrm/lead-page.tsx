import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';

import { useDashboardAccess } from '../../app/access-context';
import { PageState } from '../../components/page-state/page-state';
import type { LeadApi } from './lead-api';

export function LeadPage({ api }: { api: LeadApi }) {
  const access = useDashboardAccess();
  const queryClient = useQueryClient();
  const [businessKey, setBusinessKey] = useState('');
  const [name, setName] = useState('');
  const leads = useQuery({ queryKey: ['scrm-leads', access.corp.id], queryFn: () => api.list({ pageSize: 20 }) });
  const create = useMutation({ mutationFn: () => api.create({ businessKey: businessKey.trim(), name: name.trim(), source: 'wecom' }), onSuccess: () => { setBusinessKey(''); setName(''); void queryClient.invalidateQueries({ queryKey: ['scrm-leads', access.corp.id] }); } });
  const canAdd = access.allowedActions.has('/customer/clue/default@add');

  if (leads.isPending) return <PageState state="loading" />;
  if (leads.isError) return <PageState state="error" onRetry={() => void leads.refetch()} />;
  return (
    <section className="scrm-lead-page">
      <header className="dashboard-page-header"><h1>线索池</h1><p>管理企业微信来源的客户线索，并在确认后进入联系人生命周期。</p></header>
      {canAdd && <div className="scrm-lead-toolbar dashboard-filter-bar">
        <label>客户名称<input aria-label="客户名称" value={name} onChange={(event) => setName(event.target.value)} /></label>
        <label>业务标识<input aria-label="业务标识" value={businessKey} onChange={(event) => setBusinessKey(event.target.value)} /></label>
        <button type="button" disabled={!name.trim() || !businessKey.trim() || create.isPending} onClick={() => create.mutate()}>新增线索</button>
      </div>}
      {create.isError && <p role="alert">线索保存失败，请检查权限和业务标识。</p>}
      <div className="dashboard-data-card"><div className="dashboard-table-scroll"><table><thead><tr><th>名称</th><th>业务标识</th><th>来源</th><th>状态</th><th>版本</th></tr></thead><tbody>{leads.data.items.map((lead) => <tr key={lead.id}><td>{lead.name}</td><td>{lead.businessKey}</td><td>{lead.source}</td><td>{lead.status}</td><td>{lead.version}</td></tr>)}</tbody></table></div></div>
    </section>
  );
}
