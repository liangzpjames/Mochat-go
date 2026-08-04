import { useQuery } from '@tanstack/react-query';
import { useMemo, useState } from 'react';

import { useDashboardAccess } from '../../app/access-context';
import { pageStateForError, PageState } from '../../components/page-state/page-state';
import type { BusinessWorkbenchApi } from '../business-workbench/business-workbench-page';

type AcquisitionRecord = Record<string, unknown>;

type Column = {
  key: string;
  title: string;
  aliases: string[];
};

const channelColumns: Column[] = [
  { key: 'qrcode', title: '二维码', aliases: ['qrcodeUrl', 'qrCodeUrl', 'qrcode'] },
  { key: 'name', title: '活码名称', aliases: ['name', 'channelCodeName'] },
  { key: 'creator', title: '创建人 / 创建日期', aliases: ['createName', 'creatorName', 'createdAt', 'created_at'] },
  { key: 'employees', title: '使用人员 / 部门', aliases: ['employeeNames', 'employees', 'departmentNames'] },
  { key: 'tags', title: '客户标签', aliases: ['tagNames', 'tags'] },
  { key: 'validity', title: '有效期', aliases: ['effectiveTime', 'expireAt', 'validity'] },
  { key: 'contacts', title: '新增好友数', aliases: ['contactNum', 'contactCount', 'customerCount'] },
];

const groupColumns: Column[] = [
  { key: 'qrcode', title: '二维码', aliases: ['qrcodeUrl', 'qrCodeUrl', 'image'] },
  { key: 'name', title: '群活码名称', aliases: ['qrcodeName', 'name', 'title'] },
  { key: 'type', title: '群活码类型', aliases: ['typeText', 'type', 'stateText'] },
  { key: 'created', title: '创建时间', aliases: ['createdAt', 'created_at', 'createTime'] },
  { key: 'scan', title: '扫码次数', aliases: ['scanCount', 'contactNum'] },
  { key: 'rooms', title: '关联群聊', aliases: ['roomNum', 'roomCount', 'rooms'] },
];

function rowsFrom(payload: unknown): AcquisitionRecord[] {
  if (Array.isArray(payload)) return payload.filter(isRecord);
  if (!isRecord(payload)) return [];
  const data = isRecord(payload.data) ? payload.data : payload;
  const rows = [data.list, data.items, data.rows, isRecord(data.data) ? data.data.list : undefined].find(Array.isArray);
  return Array.isArray(rows) ? rows.filter(isRecord) : [];
}

function isRecord(value: unknown): value is AcquisitionRecord {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function valueFor(row: AcquisitionRecord, column: Column): unknown {
  for (const alias of column.aliases) {
    const value = row[alias];
    if (value !== undefined && value !== null && value !== '') return value;
  }
  return undefined;
}

function display(value: unknown): string {
  if (value === undefined || value === null || value === '') return '--';
  if (Array.isArray(value)) return value.map(display).join('、') || '--';
  if (isRecord(value)) {
    const label = value.name ?? value.title ?? value.label;
    return label === undefined ? JSON.stringify(value) : display(label);
  }
  if (typeof value === 'string') return value;
  if (typeof value === 'number' || typeof value === 'boolean' || typeof value === 'bigint') return value.toString();
  return '--';
}

function rowKey(row: AcquisitionRecord, index: number): string {
  const value = row.channelCodeId ?? row.id ?? row.workRoomAutoPullId;
  return typeof value === 'string' || typeof value === 'number' ? value.toString() : index.toString();
}

function AcquisitionTable({
  rows,
  columns,
  onDetail,
}: {
  rows: AcquisitionRecord[];
  columns: Column[];
  onDetail: (row: AcquisitionRecord) => void;
}) {
  return (
    <div className="dashboard-table-scroll phase34-table-scroll">
      <table className="phase34-table">
        <thead><tr>{columns.map((column) => <th key={column.key}>{column.title}</th>)}<th>操作</th></tr></thead>
        <tbody>{rows.map((row, index) => (
          <tr key={rowKey(row, index)}>
            {columns.map((column) => <td key={column.key}>{display(valueFor(row, column))}</td>)}
            <td><button type="button" className="phase34-link-button" onClick={() => onDetail(row)}>详情</button></td>
          </tr>
        ))}</tbody>
      </table>
    </div>
  );
}

function RecordDetail({
  title,
  row,
  columns,
  onClose,
}: {
  title: string;
  row: AcquisitionRecord;
  columns: Column[];
  onClose: () => void;
}) {
  return (
    <aside className="phase34-detail" aria-label={title}>
      <div className="phase34-detail-backdrop" onClick={onClose} />
      <div className="phase34-detail-panel">
        <div className="dashboard-card-heading">
          <div><p className="phase34-eyebrow">营销工具</p><h2>{title}</h2></div>
          <button type="button" onClick={onClose}>关闭</button>
        </div>
        <dl>{columns.map((column) => (
          <div key={column.key}><dt>{column.title}</dt><dd>{display(valueFor(row, column))}</dd></div>
        ))}</dl>
      </div>
    </aside>
  );
}

function ConnectedAcquisitionPage({
  api,
  path,
  title,
  description,
  endpoint,
  inputLabel,
  inputPlaceholder,
  columns,
  detailLabel,
  nameParam = 'name',
}: {
  api: BusinessWorkbenchApi;
  path: string;
  title: string;
  description: string;
  endpoint: string;
  inputLabel: string;
  inputPlaceholder: string;
  columns: Column[];
  detailLabel: string;
  nameParam?: string;
}) {
  const access = useDashboardAccess();
  const [draftName, setDraftName] = useState('');
  const [name, setName] = useState('');
  const [selected, setSelected] = useState<AcquisitionRecord | null>(null);
  const query = useQuery({
    queryKey: ['phase34-acquisition', access.corp.id, path, name],
    queryFn: () => api.read(endpoint, { ...(name ? { [nameParam]: name } : {}), page: 1, perPage: 20 }),
  });
  const rows = useMemo(() => rowsFrom(query.data), [query.data]);
  const hasPhase34ActionContract = useMemo(
    () => [...access.allowedActions].some((action) => action.startsWith(`${path}@`)),
    [access.allowedActions, path],
  );
  const can = (action: string) => !hasPhase34ActionContract || access.allowedActions.has(`${path}@${action}`);
  const refresh = () => { void query.refetch(); };

  return (
    <section className="phase34-page">
      <header className="phase34-page-header">
        <div><p className="phase34-eyebrow">营销工具 · 渠道获客</p><h1>{title}</h1><p>{description}</p></div>
        <div className="phase34-header-actions">
          <span className="phase34-provider-badge">数据已连接</span>
          {can('refresh') && <button type="button" disabled={query.isFetching} onClick={refresh}>刷新</button>}
        </div>
      </header>
      <div className="dashboard-filter-bar phase34-filter-bar">
        <label>{inputLabel}<input aria-label={inputLabel} placeholder={inputPlaceholder} value={draftName} onChange={(event) => setDraftName(event.target.value)} /></label>
        <label>创建人<input aria-label="创建人" placeholder="创建人名称" /></label>
        <label>使用人员<input aria-label="使用人员" placeholder="关联员工名称" /></label>
        <div className="dashboard-table-actions">
          {can('search') && <button type="button" onClick={() => { setName(draftName.trim()); setSelected(null); }}>查询</button>}
          {can('reset') && <button type="button" className="phase34-secondary-button" onClick={() => { setDraftName(''); setName(''); setSelected(null); }}>重置</button>}
        </div>
      </div>
      <div className="dashboard-data-card phase34-results-card">
        <div className="dashboard-card-heading"><div><h2>{title}列表</h2><p>当前企业：{access.corp.name}，仅展示权限范围内的真实记录。</p></div><span>{rows.length} 条</span></div>
        {query.isPending ? <PageState state="loading" /> : query.isError ? <PageState state={pageStateForError(query.error)} {...(can('refresh') ? { onRetry: refresh } : {})} /> : rows.length === 0 ? <PageState state="empty" title="暂无记录" description="当前筛选条件下没有可展示的数据。" /> : <AcquisitionTable rows={rows} columns={columns} onDetail={setSelected} />}
      </div>
      {selected !== null && <RecordDetail title={detailLabel} row={selected} columns={columns} onClose={() => setSelected(null)} />}
    </section>
  );
}

export function ChannelCodePage({ api }: { api: BusinessWorkbenchApi }) {
  return <ConnectedAcquisitionPage api={api} path="/acquisition/v2-channel-code" title="渠道活码" description="通过员工与部门活码承接客户，并追踪新增好友与渠道效果。" endpoint="/channelCode/index" inputLabel="活码名称" inputPlaceholder="请输入名称" columns={channelColumns} detailLabel="渠道活码详情" />;
}

export function GroupCodePage({ api }: { api: BusinessWorkbenchApi }) {
  return <ConnectedAcquisitionPage api={api} path="/acquisition/group-code" title="群活码" description="基于现有自动拉群能力查看群二维码配置和关联群聊。扫码统计未有统一口径时不构造数据。" endpoint="/workRoomAutoPull/index" inputLabel="群活码名称" inputPlaceholder="请输入名称" columns={groupColumns} detailLabel="群活码详情" nameParam="qrcodeName" />;
}

function shortLinkStatus(value: unknown): string {
  if (value === 'active') return '启用';
  if (value === 'disabled') return '停用';
  if (value === 'draft') return '草稿';
  return display(value);
}

function shortLinkId(row: AcquisitionRecord): number | null {
  const value = row.id;
  return typeof value === 'number' ? value : typeof value === 'string' && /^\d+$/.test(value) ? Number(value) : null;
}

function ShortLinkCreateDrawer({
  name,
  targetUrl,
  saving,
  error,
  onNameChange,
  onTargetUrlChange,
  onClose,
  onSubmit,
}: {
  name: string;
  targetUrl: string;
  saving: boolean;
  error: string;
  onNameChange: (value: string) => void;
  onTargetUrlChange: (value: string) => void;
  onClose: () => void;
  onSubmit: () => void;
}) {
  return (
    <aside className="phase34-detail" aria-label="创建短链">
      <div className="phase34-detail-backdrop" aria-hidden="true" onClick={onClose} />
      <div className="phase34-detail-panel">
        <div className="dashboard-card-heading">
          <div><p className="phase34-eyebrow">营销工具</p><h2>创建短链</h2></div>
          <button type="button" aria-label="关闭创建短链" onClick={onClose}>关闭</button>
        </div>
        <form onSubmit={(event) => { event.preventDefault(); onSubmit(); }}>
          <label>短链名称<input aria-label="短链名称" value={name} onChange={(event) => onNameChange(event.target.value)} placeholder="请输入短链名称" /></label>
          <label>目标地址<input aria-label="目标地址" value={targetUrl} onChange={(event) => onTargetUrlChange(event.target.value)} placeholder="请输入站内路径" /></label>
          <p className="phase34-field-hint">仅支持当前站内路径，访问会记录到当前企业的短链统计。</p>
          {error && <p role="alert" className="phase34-inline-error">{error}</p>}
          <div className="dashboard-table-actions">
            <button type="button" className="phase34-secondary-button" onClick={onClose}>取消</button>
            <button type="submit" disabled={saving}>{saving ? '保存中…' : '保存短链'}</button>
          </div>
        </form>
      </div>
    </aside>
  );
}

export function LiveCodeShortChainPage({ api }: { api: BusinessWorkbenchApi }) {
  const access = useDashboardAccess();
  const path = '/acquisition/live-code-short-chain';
  const [draftName, setDraftName] = useState('');
  const [name, setName] = useState('');
  const [status, setStatus] = useState('');
  const [createOpen, setCreateOpen] = useState(false);
  const [createName, setCreateName] = useState('');
  const [createTarget, setCreateTarget] = useState('');
  const [writeError, setWriteError] = useState('');
  const [saving, setSaving] = useState(false);
  const [busyID, setBusyID] = useState<number | null>(null);
  const query = useQuery({
    queryKey: ['phase34-short-link', access.corp.id, name, status],
    queryFn: () => api.read('/liveCodeShortChain/index', { ...(name ? { name } : {}), ...(status ? { status } : {}), page: 1, perPage: 20 }),
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
      await api.write('/liveCodeShortChain/store', { name: trimmedName, targetUrl: trimmedTarget, targetType: 'url' }, 'POST');
      setCreateOpen(false);
      setCreateName('');
      setCreateTarget('');
      await query.refetch();
    } catch (error) {
      setWriteError(error instanceof Error ? error.message : '创建短链失败。');
    } finally {
      setSaving(false);
    }
  };

  const disable = async (row: AcquisitionRecord) => {
    const id = shortLinkId(row);
    if (id === null) return;
    setBusyID(id);
    setWriteError('');
    try {
      await api.write('/liveCodeShortChain/disable', { id }, 'POST');
      await query.refetch();
    } catch (error) {
      setWriteError(error instanceof Error ? error.message : '停用短链失败。');
    } finally {
      setBusyID(null);
    }
  };

  return (
    <section className="phase34-page">
      <header className="phase34-page-header">
        <div><p className="phase34-eyebrow">营销工具 · 渠道获客</p><h1>活码短链</h1><p>将营销活码转换为可停用、可追踪的短链接。</p></div>
        <div className="phase34-header-actions">
          <span className="phase34-provider-badge">数据已连接</span>
          {can('create') && <button type="button" onClick={() => { setWriteError(''); setCreateOpen(true); }}>创建短链</button>}
          {can('refresh') && <button type="button" disabled={query.isFetching} onClick={refresh}>刷新</button>}
        </div>
      </header>
      <div className="dashboard-filter-bar phase34-filter-bar">
        <label>短链名称<input aria-label="短链名称" placeholder="请输入名称" value={draftName} onChange={(event) => setDraftName(event.target.value)} /></label>
        <label>短链形式<select aria-label="短链形式" value={status} onChange={(event) => setStatus(event.target.value)}><option value="">全部状态</option><option value="active">启用</option><option value="disabled">停用</option><option value="draft">草稿</option></select></label>
        <label>创建日期<input aria-label="创建日期" placeholder="创建开始日期" disabled /></label>
        <div className="dashboard-table-actions">
          {can('search') && <button type="button" onClick={() => setName(draftName.trim())}>查询</button>}
          {can('reset') && <button type="button" className="phase34-secondary-button" onClick={() => { setDraftName(''); setName(''); setStatus(''); }}>重置</button>}
        </div>
      </div>
      <div className="dashboard-data-card phase34-results-card">
        <div className="dashboard-card-heading"><div><h2>短链列表</h2><p>当前企业：{access.corp.name}，短链访问与停用状态均来自持久化 Provider。</p></div><span>{rows.length} 条</span></div>
        {writeError && !createOpen && <p role="alert" className="phase34-inline-error">{writeError}</p>}
        {query.isPending ? <PageState state="loading" /> : query.isError ? <PageState state={pageStateForError(query.error)} {...(can('refresh') ? { onRetry: refresh } : {})} /> : rows.length === 0 ? <PageState state="empty" title="暂无短链" description="创建一个站内短链后，这里会显示访问与停用状态。" /> : (
          <div className="dashboard-table-scroll phase34-table-scroll">
            <table className="phase34-table"><thead><tr><th>短链</th><th>目标地址</th><th>状态</th><th>访问次数</th><th>创建人 / 创建日期</th><th>操作</th></tr></thead>
              <tbody>{rows.map((row, index) => {
                const id = shortLinkId(row);
                const active = row.status === 'active';
                return <tr key={rowKey(row, index)}><td><strong>{display(row.name)}</strong><br /><code>/r/{display(row.token)}</code></td><td>{display(row.targetUrl)}</td><td>{shortLinkStatus(row.status)}</td><td>{display(row.visitTotal)}</td><td>{display(row.creatorName)} / {display(row.createdAt)}</td><td>{active && can('disable') && id !== null ? <button type="button" className="phase34-link-button" disabled={busyID === id} onClick={() => { void disable(row); }}>停用</button> : <span>--</span>}</td></tr>;
              })}</tbody>
            </table>
          </div>
        )}
      </div>
      {createOpen && <ShortLinkCreateDrawer name={createName} targetUrl={createTarget} saving={saving} error={writeError} onNameChange={setCreateName} onTargetUrlChange={setCreateTarget} onClose={() => { if (!saving) setCreateOpen(false); }} onSubmit={() => { void create(); }} />}
    </section>
  );
}
