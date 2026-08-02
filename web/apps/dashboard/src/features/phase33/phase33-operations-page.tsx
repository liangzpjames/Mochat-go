import { useQuery } from '@tanstack/react-query';
import { useMemo, useState } from 'react';
import { useNavigate } from 'react-router';

import { useDashboardAccess } from '../../app/access-context';
import { pageStateForError, PageState } from '../../components/page-state/page-state';
import type { BusinessWorkbenchApi } from '../business-workbench/business-workbench-page';

type OperationRecord = Record<string, unknown>;
type ProviderState = 'connected' | 'unavailable';

export type Phase33OperationConfig = {
  path: '/chat/file-audio' | '/chat/resign-staff' | '/chat/refuse-archive' | '/customer/inheritance';
  title: string;
  description: string;
  providerState: ProviderState;
  readEndpoint?: string;
  handoffPath?: string;
};

export const phase33OperationConfigs: Record<Phase33OperationConfig['path'], Phase33OperationConfig> = {
  '/chat/file-audio': {
    path: '/chat/file-audio',
    title: '文件录音',
    description: '文件与录音记录需要独立媒体 provider 后才能读取或下载。',
    providerState: 'unavailable',
  },
  '/chat/resign-staff': {
    path: '/chat/resign-staff',
    title: '离职员工',
    description: '展示已交接客户记录；交接操作仍由已接入的离职分配流程处理。',
    providerState: 'connected',
    readEndpoint: '/contactTransfer/info',
    handoffPath: '/contactTransfer/resignIndex',
  },
  '/chat/refuse-archive': {
    path: '/chat/refuse-archive',
    title: '拒绝存档',
    description: '拒绝存档记录需要独立合规 provider 后才能查询。',
    providerState: 'unavailable',
  },
  '/customer/inheritance': {
    path: '/customer/inheritance',
    title: '客户继承',
    description: '展示待分配客户并提供到现有客户交接流程的入口。',
    providerState: 'connected',
    readEndpoint: '/contactTransfer/unassignedList',
    handoffPath: '/contactTransfer/resignIndex',
  },
};

function rowsFrom(payload: unknown): OperationRecord[] {
  if (Array.isArray(payload)) return payload.filter((row): row is OperationRecord => typeof row === 'object' && row !== null);
  if (typeof payload !== 'object' || payload === null) return [];
  const source = payload as Record<string, unknown>;
  const body = typeof source.data === 'object' && source.data !== null ? source.data as Record<string, unknown> : source;
  const rows = [body.list, body.items, body.rows].find(Array.isArray);
  return Array.isArray(rows) ? rows.filter((row): row is OperationRecord => typeof row === 'object' && row !== null) : [];
}

function display(value: unknown): string {
  if (value === null || value === undefined || value === '') return '--';
  if (Array.isArray(value)) return value.map(display).join('、');
  if (typeof value === 'object') {
    try {
      return JSON.stringify(value) ?? '--';
    } catch {
      return '--';
    }
  }
  if (typeof value === 'string') return value;
  if (typeof value === 'number' || typeof value === 'boolean' || typeof value === 'bigint') {
    return value.toString();
  }
  return '--';
}

function recordKey(row: OperationRecord, index: number): string {
  const value = row.contactId ?? row.id ?? row.roomId ?? index;
  if (typeof value === 'string') return value;
  if (typeof value === 'number') return value.toString();
  return index.toString();
}

export function Phase33OperationsPage({
  api,
  config,
}: {
  api: BusinessWorkbenchApi;
  config: Phase33OperationConfig;
}) {
  const access = useDashboardAccess();
  const navigate = useNavigate();
  const [draftName, setDraftName] = useState('');
  const [contactName, setContactName] = useState('');
  const [selected, setSelected] = useState<OperationRecord | null>(null);
  const enabled = config.providerState === 'connected' && config.readEndpoint !== undefined;
  const query = useQuery({
    queryKey: ['phase33-operations', access.corp.id, config.path, contactName],
    queryFn: () => api.read(config.readEndpoint!, { ...(contactName ? { contactName } : {}), page: 1, perPage: 20 }),
    enabled,
  });
  const rows = useMemo(() => rowsFrom(query.data), [query.data]);
  const columns = useMemo(() => Object.keys(rows[0] ?? {}).slice(0, 6), [rows]);
  const can = (action: string) =>
    access.allowedActions.size === 0 || access.allowedActions.has(`${config.path}@${action}`);
  const refresh = (): void => {
    void query.refetch();
  };
  const enterHandoff = (): void => {
    if (config.handoffPath !== undefined) {
      void navigate(config.handoffPath);
    }
  };

  return (
    <section className="phase33-operations-page">
      <header className="phase33-operations-header dashboard-page-header dashboard-data-card">
        <div>
          <p className="phase33-operations-eyebrow">会话运营</p>
          <h1>{config.title}</h1>
          <p>{config.description}</p>
        </div>
        {enabled && can('refresh') && <button type="button" disabled={query.isFetching} onClick={refresh}>刷新</button>}
      </header>
      {!enabled ? (
        <div className="dashboard-data-card phase33-operations-unavailable">
          <PageState
            state="not-found"
            title="能力未接入"
            description="当前环境尚未接入可用的媒体或拒绝存档数据提供方。"
          />
        </div>
      ) : (
        <>
          <div className="dashboard-filter-bar phase33-operations-filters">
            <label>客户名称<input aria-label="客户名称" value={draftName} onChange={(event) => setDraftName(event.target.value)} /></label>
            <div className="dashboard-table-actions">
              {can('search') && <button type="button" onClick={() => { setSelected(null); setContactName(draftName.trim()); }}>查询</button>}
              {can('reset') && <button type="button" onClick={() => { setDraftName(''); setContactName(''); setSelected(null); }}>重置</button>}
            </div>
          </div>
          <div className="dashboard-data-card phase33-operations-results">
            <div className="dashboard-card-heading"><div><h2>结果列表</h2><p>仅展示当前企业权限范围内由后端返回的记录。</p></div></div>
            {query.isPending ? <PageState state="loading" /> : query.isError ? <PageState state={pageStateForError(query.error)} {...(can('refresh') ? { onRetry: refresh } : {})} /> : rows.length === 0 ? <PageState state="empty" /> : (
              <div className="dashboard-table-scroll"><table><thead><tr>{columns.map((column) => <th key={column}>{column}</th>)}<th>操作</th></tr></thead><tbody>{rows.map((row, index) => <tr key={recordKey(row, index)}>{columns.map((column) => <td key={column}>{display(row[column])}</td>)}<td>{can('detail') && <button type="button" onClick={() => setSelected(row)}>详情</button>}</td></tr>)}</tbody></table></div>
            )}
          </div>
          {selected !== null && (
            <aside className="phase33-operations-detail dashboard-data-card" aria-label="记录详情">
              <div className="dashboard-card-heading"><div><h2>记录详情</h2><p>详情来自当前列表记录，不额外构造媒体或存档数据。</p></div><button type="button" onClick={() => setSelected(null)}>关闭</button></div>
              <dl>{Object.entries(selected).map(([key, value]) => <div key={key}><dt>{key}</dt><dd>{display(value)}</dd></div>)}</dl>
              {config.handoffPath && can('handoff') && <button type="button" onClick={enterHandoff}>进入交接操作</button>}
            </aside>
          )}
        </>
      )}
    </section>
  );
}
