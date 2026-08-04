import { useQuery } from '@tanstack/react-query';
import { useMemo, useState } from 'react';

import { useDashboardAccess } from '../../app/access-context';
import { pageStateForError, PageState } from '../../components/page-state/page-state';
import type { BusinessWorkbenchApi } from '../business-workbench/business-workbench-page';

type ConversionRecord = Record<string, unknown>;
type Column = { key: string; title: string; aliases: string[] };

const groupTemplateColumns: Column[] = [
  { key: 'name', title: '模板名称', aliases: ['qrcodeName', 'name', 'title'] },
  { key: 'status', title: '状态', aliases: ['stateText', 'statusText', 'status'] },
  { key: 'total', title: '群总数', aliases: ['roomNum', 'roomCount', 'totalRooms'] },
  { key: 'today', title: '今日新增', aliases: ['todayContactNum', 'todayCount', 'todayAdded'] },
  { key: 'rooms', title: '关联群聊', aliases: ['roomNames', 'rooms'] },
  { key: 'creator', title: '创建人 / 创建时间', aliases: ['createName', 'creatorName', 'createdAt', 'created_at', 'createTime'] },
];

function isRecord(value: unknown): value is ConversionRecord {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function rowsFrom(payload: unknown): ConversionRecord[] {
  if (Array.isArray(payload)) return payload.filter(isRecord);
  if (!isRecord(payload)) return [];
  const data = isRecord(payload.data) ? payload.data : payload;
  const rows = [data.list, data.items, data.rows, isRecord(data.data) ? data.data.list : undefined].find(Array.isArray);
  return Array.isArray(rows) ? rows.filter(isRecord) : [];
}

function valueFor(row: ConversionRecord, column: Column): unknown {
  for (const alias of column.aliases) {
    const value = row[alias];
    if (value !== undefined && value !== null && value !== '') return value;
  }
  return undefined;
}

function display(value: unknown): string {
  if (value === undefined || value === null || value === '') return '--';
  if (Array.isArray(value)) return value.map(display).join('、') || '--';
  if (isRecord(value)) return display(value.name ?? value.title ?? value.label);
  if (typeof value === 'string') return value;
  if (typeof value === 'number' || typeof value === 'boolean' || typeof value === 'bigint') return value.toString();
  return '--';
}

function rowKey(row: ConversionRecord, index: number): string {
  const value = row.workRoomAutoPullId ?? row.id;
  return typeof value === 'string' || typeof value === 'number' ? value.toString() : index.toString();
}

function ProviderBlockedPage({
  title,
  description,
  statusTitle,
  statusDescription,
  actionLabel,
  inputLabel,
  headings,
}: {
  title: string;
  description: string;
  statusTitle: string;
  statusDescription: string;
  actionLabel: string;
  inputLabel: string;
  headings: string[];
}) {
  return (
    <section className="phase34-page">
      <header className="phase34-page-header">
        <div><p className="phase34-eyebrow">营销工具 · 转化承接</p><h1>{title}</h1><p>{description}</p></div>
        <div className="phase34-header-actions">
          <span className="phase34-provider-badge phase34-provider-badge-warning">等待 Provider</span>
          <button type="button" disabled>{actionLabel}</button>
        </div>
      </header>
      <div className="dashboard-filter-bar phase34-filter-bar">
        <label>{inputLabel}<input aria-label={inputLabel} placeholder={`请输入${inputLabel}`} disabled /></label>
        <label>创建人<input aria-label="创建人" placeholder="创建人名称" disabled /></label>
        <label>创建日期<input aria-label="创建日期" placeholder="请选择日期" disabled /></label>
        <button type="button" disabled>查询</button>
      </div>
      <div className="dashboard-data-card phase34-results-card">
        <div className="dashboard-card-heading"><div><h2>{title}列表</h2><p>以下字段将在正式服务接入后展示真实业务数据。</p></div></div>
        <div className="phase34-preview-headings phase34-preview-headings-wide" aria-hidden="true">
          {headings.map((heading) => <span key={heading}>{heading}</span>)}
        </div>
        <PageState state="not-found" title={statusTitle} description={statusDescription} />
      </div>
    </section>
  );
}

export function RedirectLinkPage({ api: _api }: { api: BusinessWorkbenchApi }) {
  return <ProviderBlockedPage title="获客链接" description="以可追踪链接承接外部访问，并查看访问与新增客户转化。" statusTitle="企业授权尚未接入" statusDescription="当前环境缺少获客链接授权、创建和访问统计 Provider，相关写入操作已停用。" actionLabel="立即授权并使用" inputLabel="链接名称" headings={['获客链接', '授权状态', '访问人数', '新增客户', '创建人 / 创建日期', '操作']} />;
}

export function WechatCustomerServicePage({ api: _api }: { api: BusinessWorkbenchApi }) {
  return <ProviderBlockedPage title="微信客服" description="管理企业微信客服账号、接待员工与接待方式。" statusTitle="微信客服 Provider 未配置" statusDescription="当前环境尚无可审计的客服账号同步与接待关系 Provider，同步和查询操作已停用。" actionLabel="同步客服账号" inputLabel="客服名称" headings={['客服名称', '客服账号', '接待员工', '接待方式', '创建时间', '操作']} />;
}

function Detail({ row, onClose }: { row: ConversionRecord; onClose: () => void }) {
  return (
    <aside className="phase34-detail" aria-label="一键加群详情">
      <div className="phase34-detail-backdrop" onClick={onClose} />
      <div className="phase34-detail-panel">
        <div className="dashboard-card-heading"><div><p className="phase34-eyebrow">营销工具</p><h2>一键加群详情</h2></div><button type="button" onClick={onClose}>关闭</button></div>
        <dl>{groupTemplateColumns.map((column) => <div key={column.key}><dt>{column.title}</dt><dd>{display(valueFor(row, column))}</dd></div>)}</dl>
      </div>
    </aside>
  );
}

export function GroupTemplatePage({ api }: { api: BusinessWorkbenchApi }) {
  const access = useDashboardAccess();
  const path = '/acquisition/group-template';
  const [draftName, setDraftName] = useState('');
  const [name, setName] = useState('');
  const [selected, setSelected] = useState<ConversionRecord | null>(null);
  const query = useQuery({
    queryKey: ['phase34-group-template', access.corp.id, name],
    queryFn: () => api.read('/workRoomAutoPull/index', { ...(name ? { qrcodeName: name } : {}), page: 1, perPage: 20 }),
  });
  const rows = useMemo(() => rowsFrom(query.data), [query.data]);
  const hasActionContract = useMemo(() => [...access.allowedActions].some((action) => action.startsWith(`${path}@`)), [access.allowedActions]);
  const can = (action: string) => !hasActionContract || access.allowedActions.has(`${path}@${action}`);
  const refresh = () => { void query.refetch(); };

  return (
    <section className="phase34-page">
      <header className="phase34-page-header">
        <div><p className="phase34-eyebrow">营销工具 · 转化承接</p><h1>一键加群</h1><p>复用自动拉群 Provider 管理加群模板，查看关联群聊和当日新增。</p></div>
        <div className="phase34-header-actions"><span className="phase34-provider-badge">数据已连接</span>{can('refresh') && <button type="button" disabled={query.isFetching} onClick={refresh}>刷新</button>}</div>
      </header>
      <div className="dashboard-filter-bar phase34-filter-bar">
        <label>模板名称<input aria-label="模板名称" placeholder="请输入模板名称" value={draftName} onChange={(event) => setDraftName(event.target.value)} /></label>
        <label>创建人<input aria-label="创建人" placeholder="创建人名称" /></label>
        <label>状态<select aria-label="状态" defaultValue=""><option value="">全部状态</option></select></label>
        <div className="dashboard-table-actions">
          {can('search') && <button type="button" onClick={() => { setName(draftName.trim()); setSelected(null); }}>查询</button>}
          {can('reset') && <button type="button" className="phase34-secondary-button" onClick={() => { setDraftName(''); setName(''); setSelected(null); }}>重置</button>}
        </div>
      </div>
      <div className="dashboard-data-card phase34-results-card">
        <div className="dashboard-card-heading"><div><h2>加群模板列表</h2><p>当前企业：{access.corp.name}，展示 Provider 返回的真实模板记录。</p></div><span>{rows.length} 条</span></div>
        {query.isPending ? <PageState state="loading" /> : query.isError ? <PageState state={pageStateForError(query.error)} {...(can('refresh') ? { onRetry: refresh } : {})} /> : rows.length === 0 ? <PageState state="empty" title="暂无模板" description="当前筛选条件下没有可展示的加群模板。" /> : (
          <div className="dashboard-table-scroll phase34-table-scroll"><table className="phase34-table"><thead><tr>{groupTemplateColumns.map((column) => <th key={column.key}>{column.title}</th>)}<th>操作</th></tr></thead><tbody>{rows.map((row, index) => <tr key={rowKey(row, index)}>{groupTemplateColumns.map((column) => <td key={column.key}>{display(valueFor(row, column))}</td>)}<td><button type="button" className="phase34-link-button" onClick={() => setSelected(row)}>详情</button></td></tr>)}</tbody></table></div>
        )}
      </div>
      {selected !== null && <Detail row={selected} onClose={() => setSelected(null)} />}
    </section>
  );
}
