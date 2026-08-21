import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useMemo, useState } from 'react';
import { useSearchParams } from 'react-router';

import { useDashboardAccess } from '../../app/access-context';
import { DashboardPagination } from '../../components/dashboard-pagination';
import { pageStateForError, PageState } from '../../components/page-state/page-state';
import { RiskWarningDrawer, RiskWarningQueryBar, RiskWarningShell, RiskWarningTabs } from '../risk-warning/risk-warning-shell';
import { createRiskWarningApi, type CustomerLossFilter, type CustomerLossRecord } from '../risk-warning/risk-warning-api';
import type { BusinessWorkbenchApi } from '../business-workbench/business-workbench-page';

const pageSize = 20;
const lossTypeLabels: Record<string, string> = { employee_removed_customer: '员工删除客户', customer_removed_employee: '客户删除员工' };
const riskLabels: Record<string, string> = { unclassified: '未分级', low: '低风险', medium: '中风险', high: '高风险' };
const statusLabels: Record<string, string> = { pending: '待处置', confirmed: '已确认', ignored: '已忽略', closed: '已关闭' };

function formatDate(value: string): string {
  if (!value) return '--';
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value.replace('T', ' ').replace(/Z$/, '') : date.toLocaleString('zh-CN', { hour12: false }).replace(/\//g, '-');
}

function formatAvatar(name: string): string { return (name.trim().slice(0, 1) || '客').toUpperCase(); }

function Avatar({ name, src }: { name: string; src: string }) {
  return src ? <img className="risk-warning-avatar" src={src} alt="" /> : <span className="risk-warning-avatar risk-warning-avatar-fallback" aria-hidden="true">{formatAvatar(name)}</span>;
}

function riskPill(value: string, labels: Record<string, string>, kind = value) {
  const tone = kind === 'high' ? 'risk-warning-pill-high' : kind === 'medium' ? 'risk-warning-pill-medium' : kind === 'low' ? 'risk-warning-pill-low' : 'risk-warning-pill-neutral';
  return <span className={`risk-warning-pill ${tone}`}>{(labels[value] ?? value) || '--'}</span>;
}

export function CustomerLossPage({ api: workbenchApi }: { api: BusinessWorkbenchApi }) {
  const access = useDashboardAccess();
  const api = useMemo(() => createRiskWarningApi(workbenchApi), [workbenchApi]);
  const queryClient = useQueryClient();
  const [params, setParams] = useSearchParams();
  const tab = params.get('tab') === 'rules' ? 'rules' : 'records';
  const [draft, setDraft] = useState({ customer: params.get('customer') ?? '', employeeId: params.get('employeeId') ?? '', lossType: params.get('lossType') ?? '', auditStatus: params.get('auditStatus') ?? '', occurredFrom: params.get('occurredFrom') ?? '', occurredTo: params.get('occurredTo') ?? '' });
  const [filters, setFilters] = useState<CustomerLossFilter>({ employeeId: params.get('employeeId') ?? undefined, page: Number(params.get('page') ?? 1) || 1, perPage: pageSize });
  const [selected, setSelected] = useState<CustomerLossRecord | null>(null);
  const records = useQuery({ queryKey: ['customer-loss-records', access.corp.id, filters, draft.customer, draft.lossType, draft.auditStatus, draft.occurredFrom, draft.occurredTo], queryFn: () => api.customerLossRecords(filters), enabled: access.corp.authorized && tab === 'records', retry: false });

  const updateURL = (next: Partial<typeof draft> & { page?: number; tab?: string }) => {
    const value = new URLSearchParams(params);
    for (const key of ['customer', 'employeeId', 'lossType', 'auditStatus', 'occurredFrom', 'occurredTo', 'page', 'tab']) value.delete(key);
    for (const [key, raw] of Object.entries(next)) if (raw !== undefined && raw !== '') value.set(key, String(raw));
    setParams(value);
  };
  const submit = () => { const next = { employeeId: draft.employeeId.trim() || undefined, page: 1, perPage: pageSize }; setFilters(next); updateURL({ ...draft, employeeId: draft.employeeId.trim(), page: 1 }); setSelected(null); };
  const reset = () => { const next = { customer: '', employeeId: '', lossType: '', auditStatus: '', occurredFrom: '', occurredTo: '' }; setDraft(next); setFilters({ page: 1, perPage: pageSize }); updateURL({ page: 1 }); setSelected(null); };
  const refresh = () => void queryClient.invalidateQueries({ queryKey: ['customer-loss-records', access.corp.id] });
  const onPage = (page: number) => { setFilters((current) => ({ ...current, page })); updateURL({ ...draft, page }); };
  const rows = records.data?.items ?? [];

  return <RiskWarningShell className="customer-loss-page">
    <RiskWarningTabs active={tab} tabs={[{ id: 'records', label: '客户流失' }, { id: 'rules', label: '流失规则' }]} onChange={(next) => { const value = new URLSearchParams(params); value.set('tab', next); value.delete('recordId'); setParams(value); setSelected(null); }} />
    {tab === 'records' ? <>
      <RiskWarningQueryBar fetching={records.isFetching} onQuery={submit} onReset={reset} onRefresh={refresh}>
        <label>客户<input aria-label="客户" value={draft.customer} placeholder="客户名称或 ID" onChange={(event) => setDraft((current) => ({ ...current, customer: event.target.value }))} /></label>
        <label>关联员工<input aria-label="关联员工" inputMode="numeric" value={draft.employeeId} placeholder="员工 ID" onChange={(event) => setDraft((current) => ({ ...current, employeeId: event.target.value }))} /></label>
        <label>流失类型<select aria-label="流失类型" value={draft.lossType} onChange={(event) => setDraft((current) => ({ ...current, lossType: event.target.value }))}><option value="">全部</option><option value="employee_removed_customer">员工删除客户</option><option value="customer_removed_employee">客户删除员工</option></select></label>
        <label>处置状态<select aria-label="处置状态" value={draft.auditStatus} onChange={(event) => setDraft((current) => ({ ...current, auditStatus: event.target.value }))}><option value="">全部</option><option value="pending">待处置</option><option value="confirmed">已确认</option><option value="ignored">已忽略</option><option value="closed">已关闭</option></select></label>
        <label>流失日期<input aria-label="流失日期" type="date" value={draft.occurredFrom} onChange={(event) => setDraft((current) => ({ ...current, occurredFrom: event.target.value }))} /></label>
      </RiskWarningQueryBar>
      <section className="risk-warning-results"><div className="risk-warning-results-header"><div><h2>流失记录</h2><p>列表按当前企业权限返回的真实客户关系记录展示。</p></div><span className="risk-warning-muted">共 {records.data?.total ?? 0} 条</span></div>
        {!access.corp.authorized ? <PageState state="forbidden" title="无权访问当前企业数据" description="请切换到已授权企业，或联系管理员开通权限。" /> : records.isPending ? <PageState state="loading" /> : records.isError ? <PageState state={pageStateForError(records.error)} onRetry={() => void records.refetch()} /> : rows.length === 0 ? <PageState state="empty" title="暂无客户流失记录" description="当前筛选条件下没有可复核的真实记录。" /> : <>
          <div className="risk-warning-table-wrap"><table className="risk-warning-table customer-loss-record-table"><thead><tr><th>流失类型</th><th>最近消息</th><th>客户</th><th>关联员工</th><th>风险等级</th><th>处置状态</th><th>流失时间</th><th>操作</th></tr></thead><tbody>{rows.map((row) => <tr key={row.id}><td>{lossTypeLabels[row.lossType] ?? '客户关系变化'}</td><td><span className="risk-warning-inline-preview" title={row.lastMessage}>{row.lastMessage || '--'}</span></td><td><span className="risk-warning-person"><Avatar name={row.customerName} src={row.customerAvatar} /><span>{row.customerName}</span></span></td><td>{row.employeeName}</td><td>{riskPill(row.riskLevel, riskLabels)}</td><td>{statusLabels[row.auditStatus] ?? row.auditStatus}</td><td>{formatDate(row.occurredAt)}</td><td className="risk-warning-row-control"><button type="button" onClick={() => { setSelected(row); const value = new URLSearchParams(params); value.set('recordId', String(row.id)); setParams(value); }}>查看详情</button></td></tr>)}</tbody></table></div>
          <div className="risk-warning-pagination"><DashboardPagination page={filters.page ?? 1} pageSize={pageSize} total={records.data?.total ?? 0} onPageChange={onPage} ariaLabel="客户流失记录分页" /></div>
        </>}
      </section>
      <RiskWarningDrawer open={selected !== null} title="客户流失详情" {...(selected ? { description: `${selected.customerName} · ${lossTypeLabels[selected.lossType] ?? '客户关系变化'}` } : {})} onClose={() => { setSelected(null); const value = new URLSearchParams(params); value.delete('recordId'); setParams(value); }}>
        {selected ? <dl className="risk-warning-detail-grid"><div><dt>客户</dt><dd><span className="risk-warning-person"><Avatar name={selected.customerName} src={selected.customerAvatar} /><span>{selected.customerName}</span></span></dd></div><div><dt>责任员工</dt><dd>{selected.employeeName}</dd></div><div><dt>流失类型</dt><dd>{lossTypeLabels[selected.lossType] ?? '客户关系变化'}</dd></div><div><dt>最近消息</dt><dd>{selected.lastMessage || '--'}</dd></div><div><dt>客户标签</dt><dd>{selected.tags.length ? selected.tags.join('、') : '--'}</dd></div><div><dt>风险等级</dt><dd>{riskPill(selected.riskLevel, riskLabels)}</dd></div><div><dt>处置状态</dt><dd>{statusLabels[selected.auditStatus] ?? selected.auditStatus}</dd></div><div><dt>流失时间</dt><dd>{formatDate(selected.occurredAt)}</dd></div></dl> : null}
      </RiskWarningDrawer>
    </> : <section className="risk-warning-results"><div className="risk-warning-results-header"><div><h2>流失规则</h2><p>按企业微信删除事件维护流失识别规则。</p></div></div><PageState state="empty" title="暂无可配置规则" description="当前企业还没有返回可编辑的流失规则。" /></section>}
  </RiskWarningShell>;
}
