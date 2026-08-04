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

function conversionStatus(value: unknown): string {
  if (value === 'active' || value === 'authorized') return value === 'authorized' ? '已授权' : '启用';
  if (value === 'pending_sync') return '待同步';
  if (value === 'unauthorized') return '未授权';
  if (value === 'disabled') return '停用';
  if (value === 'failed') return '失败';
  return display(value);
}

function conversionID(row: ConversionRecord): number | null {
  const value = row.id;
  return typeof value === 'number' ? value : typeof value === 'string' && /^\d+$/.test(value) ? Number(value) : null;
}

function ConversionCreateDrawer({
  label,
  name,
  accountLabel,
  account,
  target,
  saving,
  error,
  onNameChange,
  onAccountChange,
  onClose,
  onSubmit,
}: {
  label: string;
  name: string;
  accountLabel: string;
  account: string;
  target?: string;
  saving: boolean;
  error: string;
  onNameChange: (value: string) => void;
  onAccountChange: (value: string) => void;
  onClose: () => void;
  onSubmit: () => void;
}) {
  return (
    <aside className="phase34-detail" aria-label={label}>
      <div className="phase34-detail-backdrop" aria-hidden="true" onClick={onClose} />
      <div className="phase34-detail-panel">
        <div className="dashboard-card-heading"><div><p className="phase34-eyebrow">营销工具</p><h2>{label}</h2></div><button type="button" aria-label={`关闭${label}`} onClick={onClose}>关闭</button></div>
        <form onSubmit={(event) => { event.preventDefault(); onSubmit(); }}>
          <label>{label === '创建获客链接' ? '链接名称' : '客服名称'}<input aria-label={label === '创建获客链接' ? '链接名称' : '客服名称'} value={name} onChange={(event) => onNameChange(event.target.value)} placeholder="请输入名称" /></label>
          <label>{accountLabel}<input aria-label={accountLabel} value={account} onChange={(event) => onAccountChange(event.target.value)} placeholder={label === '创建获客链接' ? '请输入站内路径' : '请输入客服账号'} /></label>
          {target && <p className="phase34-field-hint">仅支持站内路径，短链访问和转化会由当前企业 Provider 记录。</p>}
          {!target && <p className="phase34-field-hint">新记录先保存为待同步状态，外部客服能力成功后才会启用。</p>}
          {error && <p role="alert" className="phase34-inline-error">{error}</p>}
          <div className="dashboard-table-actions"><button type="button" className="phase34-secondary-button" onClick={onClose}>取消</button><button type="submit" disabled={saving}>{saving ? '保存中…' : label === '创建获客链接' ? '保存获客链接' : '保存客服'}</button></div>
        </form>
      </div>
    </aside>
  );
}

export function RedirectLinkPage({ api }: { api: BusinessWorkbenchApi }) {
  const access = useDashboardAccess();
  const path = '/acquisition/redirect-link';
  const [draftName, setDraftName] = useState('');
  const [name, setName] = useState('');
  const [createOpen, setCreateOpen] = useState(false);
  const [createName, setCreateName] = useState('');
  const [createTarget, setCreateTarget] = useState('');
  const [saving, setSaving] = useState(false);
  const [authorizing, setAuthorizing] = useState(false);
  const [writeError, setWriteError] = useState('');
  const query = useQuery({
    queryKey: ['phase34-acquisition-link', access.corp.id, name],
    queryFn: () => api.read('/acquisitionLink/index', { ...(name ? { name } : {}), page: 1, perPage: 20 }),
  });
  const rows = useMemo(() => rowsFrom(query.data), [query.data]);
  const hasActionContract = useMemo(() => [...access.allowedActions].some((action) => action.startsWith(`${path}@`)), [access.allowedActions]);
  const can = (action: string) => !hasActionContract || access.allowedActions.has(`${path}@${action}`);
  const refresh = () => { void query.refetch(); };

  const create = async () => {
    const trimmedName = createName.trim();
    const trimmedTarget = createTarget.trim();
    if (!trimmedName || !trimmedTarget || !trimmedTarget.startsWith('/') || trimmedTarget.startsWith('//')) {
      setWriteError('请输入名称和安全的站内目标地址。');
      return;
    }
    setSaving(true);
    setWriteError('');
    try {
      await api.write('/acquisitionLink/store', { name: trimmedName, targetUrl: trimmedTarget }, 'POST');
      setCreateOpen(false);
      setCreateName('');
      setCreateTarget('');
      await query.refetch();
    } catch (error) {
      setWriteError(error instanceof Error ? error.message : '创建获客链接失败。');
    } finally {
      setSaving(false);
    }
  };

  const authorize = async () => {
    setAuthorizing(true);
    setWriteError('');
    try {
      await api.write('/acquisitionLink/authorize', {}, 'POST');
      await query.refetch();
    } catch (error) {
      setWriteError(error instanceof Error ? error.message : '企业授权失败。');
    } finally {
      setAuthorizing(false);
    }
  };

  return (
    <section className="phase34-page">
      <header className="phase34-page-header"><div><p className="phase34-eyebrow">营销工具 · 转化承接</p><h1>获客链接</h1><p>以可追踪链接承接外部访问，并查看访问与新增客户转化。</p></div><div className="phase34-header-actions"><span className="phase34-provider-badge">本地 Provider 已连接</span>{can('create') && <button type="button" onClick={() => { setWriteError(''); setCreateOpen(true); }}>创建获客链接</button>}{can('authorize') && <button type="button" disabled={authorizing} onClick={() => { void authorize(); }}>立即授权并使用</button>}{can('refresh') && <button type="button" disabled={query.isFetching} onClick={refresh}>刷新</button>}</div></header>
      <div className="dashboard-filter-bar phase34-filter-bar"><label>链接名称<input aria-label="链接名称" placeholder="请输入链接名称" value={draftName} onChange={(event) => setDraftName(event.target.value)} /></label><label>创建人<input aria-label="创建人" placeholder="创建人名称" disabled /></label><label>创建日期<input aria-label="创建日期" placeholder="请选择日期" disabled /></label><div className="dashboard-table-actions">{can('search') && <button type="button" onClick={() => setName(draftName.trim())}>查询</button>}{can('reset') && <button type="button" className="phase34-secondary-button" onClick={() => { setDraftName(''); setName(''); }}>重置</button>}</div></div>
      <div className="dashboard-data-card phase34-results-card"><div className="dashboard-card-heading"><div><h2>获客链接列表</h2><p>当前企业：{access.corp.name}，列表来自持久化 Provider；外部授权失败会保留在当前状态。</p></div><span>{rows.length} 条</span></div>{writeError && !createOpen && <p role="alert" className="phase34-inline-error">{writeError}</p>}{query.isPending ? <PageState state="loading" /> : query.isError ? <PageState state={pageStateForError(query.error)} {...(can('refresh') ? { onRetry: refresh } : {})} /> : rows.length === 0 ? <PageState state="empty" title="暂无获客链接" description="创建一条站内获客链接后，这里会显示授权和访问状态。" /> : <div className="dashboard-table-scroll phase34-table-scroll"><table className="phase34-table"><thead><tr><th>获客链接</th><th>目标地址</th><th>授权状态</th><th>访问人数</th><th>新增客户</th><th>创建人 / 创建日期</th></tr></thead><tbody>{rows.map((row, index) => <tr key={rowKey(row, index)}><td><strong>{display(row.name)}</strong></td><td>{display(row.targetUrl)}</td><td>{conversionStatus(row.authorizationStatus)} / {conversionStatus(row.status)}</td><td>{display(row.visitTotal)}</td><td>{display(row.conversionTotal)}</td><td>{display(row.creatorName)} / {display(row.createdAt)}</td></tr>)}</tbody></table></div>}</div>
      {createOpen && <ConversionCreateDrawer label="创建获客链接" name={createName} accountLabel="目标地址" account={createTarget} target={createTarget} saving={saving} error={writeError} onNameChange={setCreateName} onAccountChange={setCreateTarget} onClose={() => { if (!saving) setCreateOpen(false); }} onSubmit={() => { void create(); }} />}
    </section>
  );
}

export function WechatCustomerServicePage({ api }: { api: BusinessWorkbenchApi }) {
  const access = useDashboardAccess();
  const path = '/acquisition/wechat-customer-service';
  const [draftName, setDraftName] = useState('');
  const [name, setName] = useState('');
  const [createOpen, setCreateOpen] = useState(false);
  const [createName, setCreateName] = useState('');
  const [createAccount, setCreateAccount] = useState('');
  const [saving, setSaving] = useState(false);
  const [syncing, setSyncing] = useState(false);
  const [writeError, setWriteError] = useState('');
  const query = useQuery({
    queryKey: ['phase34-customer-service', access.corp.id, name],
    queryFn: () => api.read('/customerService/index', { ...(name ? { name } : {}), page: 1, perPage: 20 }),
  });
  const rows = useMemo(() => rowsFrom(query.data), [query.data]);
  const hasActionContract = useMemo(() => [...access.allowedActions].some((action) => action.startsWith(`${path}@`)), [access.allowedActions]);
  const can = (action: string) => !hasActionContract || access.allowedActions.has(`${path}@${action}`);
  const refresh = () => { void query.refetch(); };

  const create = async () => {
    const trimmedName = createName.trim();
    const trimmedAccount = createAccount.trim();
    if (!trimmedName || !trimmedAccount) {
      setWriteError('请输入客服名称和客服账号。');
      return;
    }
    setSaving(true);
    setWriteError('');
    try {
      await api.write('/customerService/store', { name: trimmedName, account: trimmedAccount, employeeIds: [], receiveMode: 'round_robin' }, 'POST');
      setCreateOpen(false);
      setCreateName('');
      setCreateAccount('');
      await query.refetch();
    } catch (error) {
      setWriteError(error instanceof Error ? error.message : '创建客服失败。');
    } finally {
      setSaving(false);
    }
  };

  const sync = async () => {
    setSyncing(true);
    setWriteError('');
    try {
      await api.write('/customerService/sync', {}, 'POST');
      await query.refetch();
    } catch (error) {
      setWriteError(error instanceof Error ? error.message : '同步客服账号失败。');
    } finally {
      setSyncing(false);
    }
  };

  return (
    <section className="phase34-page">
      <header className="phase34-page-header"><div><p className="phase34-eyebrow">营销工具 · 转化承接</p><h1>微信客服</h1><p>管理企业微信客服账号、接待员工与接待方式。</p></div><div className="phase34-header-actions"><span className="phase34-provider-badge">本地 Provider 已连接</span>{can('create') && <button type="button" onClick={() => { setWriteError(''); setCreateOpen(true); }}>创建客服</button>}{can('sync') && <button type="button" disabled={syncing} onClick={() => { void sync(); }}>同步客服账号</button>}{can('refresh') && <button type="button" disabled={query.isFetching} onClick={refresh}>刷新</button>}</div></header>
      <div className="dashboard-filter-bar phase34-filter-bar"><label>客服名称<input aria-label="客服名称" placeholder="请输入客服名称" value={draftName} onChange={(event) => setDraftName(event.target.value)} /></label><label>创建人<input aria-label="创建人" placeholder="创建人名称" disabled /></label><label>创建日期<input aria-label="创建日期" placeholder="请选择日期" disabled /></label><div className="dashboard-table-actions">{can('search') && <button type="button" onClick={() => setName(draftName.trim())}>查询</button>}{can('reset') && <button type="button" className="phase34-secondary-button" onClick={() => { setDraftName(''); setName(''); }}>重置</button>}</div></div>
      <div className="dashboard-data-card phase34-results-card"><div className="dashboard-card-heading"><div><h2>微信客服列表</h2><p>当前企业：{access.corp.name}，客服记录和同步状态来自持久化 Provider。</p></div><span>{rows.length} 条</span></div>{writeError && !createOpen && <p role="alert" className="phase34-inline-error">{writeError}</p>}{query.isPending ? <PageState state="loading" /> : query.isError ? <PageState state={pageStateForError(query.error)} {...(can('refresh') ? { onRetry: refresh } : {})} /> : rows.length === 0 ? <PageState state="empty" title="暂无客服账号" description="创建客服记录后，再通过同步动作接入外部客服 Provider。" /> : <div className="dashboard-table-scroll phase34-table-scroll"><table className="phase34-table"><thead><tr><th>客服名称</th><th>客服账号</th><th>接待员工</th><th>接待方式</th><th>状态</th><th>创建人 / 创建日期</th></tr></thead><tbody>{rows.map((row, index) => <tr key={rowKey(row, index)}><td><strong>{display(row.name)}</strong></td><td>{display(row.account)}</td><td>{display(row.employeeIds)}</td><td>{display(row.receiveMode)}</td><td>{conversionStatus(row.status)}</td><td>{display(row.creatorName)} / {display(row.createdAt)}</td></tr>)}</tbody></table></div>}</div>
      {createOpen && <ConversionCreateDrawer label="创建客服" name={createName} accountLabel="客服账号" account={createAccount} saving={saving} error={writeError} onNameChange={setCreateName} onAccountChange={setCreateAccount} onClose={() => { if (!saving) setCreateOpen(false); }} onSubmit={() => { void create(); }} />}
    </section>
  );
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
