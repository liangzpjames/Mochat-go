import { useQuery } from '@tanstack/react-query';
import { useMemo, useState } from 'react';

import { useDashboardAccess } from '../../app/access-context';
import { pageStateForError, PageState } from '../../components/page-state/page-state';
import type { BusinessWorkbenchApi } from '../business-workbench/business-workbench-page';

type RiskWarningRecord = Record<string, unknown>;
type ProviderState = 'connected' | 'unavailable';

export type RiskWarningPath =
  | '/ai-insight/v2/risk'
  | '/ai-insight/v2/timeout'
  | '/ai-insight/v2/customer-loss'
  | '/ai-insight/v2/message-intercept'
  | '/ai-insight/v2/keyword-library'
  | '/ai-insight/v2/silent-customer';

export type RiskWarningConfig = {
  path: RiskWarningPath;
  title: string;
  description: string;
  providerState: ProviderState;
  readEndpoint?: '/workContact/lossContact';
};

export const riskWarningConfigs: Record<RiskWarningPath, RiskWarningConfig> = {
  '/ai-insight/v2/risk': {
    path: '/ai-insight/v2/risk',
    title: '风险行为',
    description: '风险行为识别需要独立的消息风控数据提供方。',
    providerState: 'unavailable',
  },
  '/ai-insight/v2/timeout': {
    path: '/ai-insight/v2/timeout',
    title: '超时预警',
    description: '超时规则与命中记录需要独立的会话时效数据提供方。',
    providerState: 'unavailable',
  },
  '/ai-insight/v2/customer-loss': {
    path: '/ai-insight/v2/customer-loss',
    title: '客户流失',
    description: '仅展示当前企业权限范围内由客户流失接口返回的记录。',
    providerState: 'connected',
    readEndpoint: '/workContact/lossContact',
  },
  '/ai-insight/v2/message-intercept': {
    path: '/ai-insight/v2/message-intercept',
    title: '消息拦截',
    description: '消息拦截审计需要独立的合规数据提供方。',
    providerState: 'unavailable',
  },
  '/ai-insight/v2/keyword-library': {
    path: '/ai-insight/v2/keyword-library',
    title: '关键词库',
    description: '关键词规则库需要独立的规则管理数据提供方。',
    providerState: 'unavailable',
  },
  '/ai-insight/v2/silent-customer': {
    path: '/ai-insight/v2/silent-customer',
    title: '沉默客户',
    description: '沉默客户判定需要独立的客户活跃度数据提供方。',
    providerState: 'unavailable',
  },
};

function rowsFrom(payload: unknown): RiskWarningRecord[] {
  if (Array.isArray(payload)) return payload.filter((row): row is RiskWarningRecord => typeof row === 'object' && row !== null);
  if (typeof payload !== 'object' || payload === null) return [];
  const source = payload as Record<string, unknown>;
  const body = typeof source.data === 'object' && source.data !== null ? source.data as Record<string, unknown> : source;
  const rows = [body.list, body.items, body.rows].find(Array.isArray);
  return Array.isArray(rows) ? rows.filter((row): row is RiskWarningRecord => typeof row === 'object' && row !== null) : [];
}

function display(value: unknown): string {
  if (value === null || value === undefined || value === '') return '--';
  if (Array.isArray(value)) return value.map(display).join('、');
  if (typeof value === 'object') return JSON.stringify(value);
  if (typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean' || typeof value === 'bigint') return String(value);
  return '--';
}

function recordKey(row: RiskWarningRecord, index: number): string {
  const value = row.id ?? row.contactId ?? index;
  return typeof value === 'string' || typeof value === 'number' ? String(value) : String(index);
}

export function RiskWarningPage({
  api,
  config,
}: {
  api: BusinessWorkbenchApi;
  config: RiskWarningConfig;
}) {
  const access = useDashboardAccess();
  const [draftEmployeeID, setDraftEmployeeID] = useState('');
  const [employeeID, setEmployeeID] = useState('');
  const [selected, setSelected] = useState<RiskWarningRecord | null>(null);
  const providerConnected = config.providerState === 'connected' && config.readEndpoint !== undefined;
  const enabled = providerConnected && access.corp.authorized;
  const query = useQuery({
    queryKey: ['phase33-risk-warning', access.corp.id, config.path, employeeID],
    queryFn: () => api.read(config.readEndpoint!, {
      ...(employeeID ? { employeeId: employeeID } : {}),
      page: 1,
      perPage: 20,
    }),
    enabled,
  });
  const rows = useMemo(() => rowsFrom(query.data), [query.data]);
  const columns = useMemo(() => Object.keys(rows[0] ?? {}).filter((key) => key !== 'user').slice(0, 6), [rows]);

  return (
    <section className="phase33-risk-warning-page">
      <header className="phase33-risk-warning-header dashboard-page-header dashboard-data-card">
        <div>
          <p className="phase33-risk-warning-eyebrow">风险预警</p>
          <h1>{config.title}</h1>
          <p>{config.description}</p>
        </div>
        <button type="button" disabled={!enabled || query.isFetching} onClick={() => void query.refetch()}>刷新</button>
      </header>

      <div className="dashboard-filter-bar phase33-risk-warning-filters" aria-label="风险预警筛选">
        <label>员工 ID<input aria-label="员工 ID" disabled={!enabled} inputMode="numeric" value={draftEmployeeID} onChange={(event) => setDraftEmployeeID(event.target.value)} /></label>
        <label>关键词<input aria-label="关键词" disabled title="当前数据提供方未声明关键词筛选能力" /></label>
        <label>日期<input aria-label="日期" disabled type="date" title="当前数据提供方未声明日期筛选能力" /></label>
        <label>状态<select aria-label="状态" disabled title="当前数据提供方未声明状态筛选能力"><option>未接入</option></select></label>
        <div className="dashboard-table-actions">
          <button type="button" disabled={!enabled} onClick={() => { setSelected(null); setEmployeeID(draftEmployeeID.trim()); }}>查询</button>
          <button type="button" disabled={!enabled} onClick={() => { setDraftEmployeeID(''); setEmployeeID(''); setSelected(null); }}>重置</button>
        </div>
      </div>

      {!access.corp.authorized ? (
        <div className="dashboard-data-card phase33-risk-warning-state"><PageState state="forbidden" title="无权访问当前企业数据" description="请切换到已授权企业，或联系管理员开通权限。" /></div>
      ) : !providerConnected ? (
        <div className="dashboard-data-card phase33-risk-warning-state">
          <PageState state="not-found" title="数据提供方未接入" description="当前页面尚无可用的风险预警 Provider；不会展示虚构的命中、规则或客户数据。" />
        </div>
      ) : (
        <div className="dashboard-data-card phase33-risk-warning-results">
          <div className="dashboard-card-heading"><div><h2>查询结果</h2><p>仅保留后端实际返回的客户流失记录。</p></div></div>
          {query.isPending ? <PageState state="loading" /> : query.isError ? <PageState state={pageStateForError(query.error)} onRetry={() => void query.refetch()} /> : rows.length === 0 ? <PageState state="empty" /> : (
            <div className="dashboard-table-scroll"><table><thead><tr>{columns.map((column) => <th key={column}>{column}</th>)}<th>操作</th></tr></thead><tbody>{rows.map((row, index) => <tr key={recordKey(row, index)}>{columns.map((column) => <td key={column}>{display(row[column])}</td>)}<td><button type="button" onClick={() => setSelected(row)}>详情</button></td></tr>)}</tbody></table></div>
          )}
        </div>
      )}

      {selected !== null && (
        <aside className="phase33-risk-warning-detail dashboard-data-card" aria-label="客户流失详情">
          <div className="dashboard-card-heading"><div><h2>客户流失详情</h2><p>详情来自当前查询结果，不额外构造风险信息。</p></div><button type="button" onClick={() => setSelected(null)}>关闭</button></div>
          <dl>{Object.entries(selected).map(([key, value]) => <div key={key}><dt>{key}</dt><dd>{display(value)}</dd></div>)}</dl>
        </aside>
      )}
    </section>
  );
}
