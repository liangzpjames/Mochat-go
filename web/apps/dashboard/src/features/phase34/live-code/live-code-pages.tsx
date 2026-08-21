import { useQuery } from '@tanstack/react-query';
import { useMemo, useState } from 'react';

import { useDashboardAccess } from '../../../app/access-context';
import { DashboardDialog } from '../../../components/dashboard-dialog';
import { PageState, pageStateForError } from '../../../components/page-state/page-state';
import type { BusinessWorkbenchApi } from '../../business-workbench/business-workbench-page';
import { numberOf, paginationFrom, recordsFrom, stateClass, stateText, statisticsFrom, textOf } from './live-code-api';
import type { LiveCodeKind, LiveCodeQuery, LiveCodeRecord } from './live-code-types';

function value(row: LiveCodeRecord, ...keys: string[]): unknown {
  for (const key of keys) {
    if (row[key] !== undefined && row[key] !== null && row[key] !== '') return row[key];
  }
  return undefined;
}

function isVerified(row: LiveCodeRecord): boolean {
  if (numberOf(value(row, 'isVerified', 'verified')) === 1) return true;
  const state = String(value(row, 'state', 'lifecycleState') ?? '').toLowerCase();
  return state === 'active' || state === 'verified' || state === 'running';
}

function imageURL(row: LiveCodeRecord): string {
  return textOf(value(row, 'qrcodeUrl', 'qrCodeUrl', 'image'), '');
}

function rowID(row: LiveCodeRecord, index: number): string {
  return textOf(value(row, 'channelCodeId', 'workRoomAutoPullId', 'id'), String(index));
}

function employeesText(row: LiveCodeRecord): string {
  return textOf(value(row, 'employees', 'employeeNames', 'drainageEmployee'));
}

function validityText(row: LiveCodeRecord): string {
  const validity = value(row, 'validity');
  if (typeof validity === 'object' && validity !== null) {
    const item = validity as Record<string, unknown>;
    if (item.kind === 'permanent') return '长期有效';
    return `${textOf(item.from, '未开始')} – ${textOf(item.until, '未设置')}`;
  }
  return textOf(value(row, 'expireAt', 'effectiveTime'), '长期有效');
}

function StatusPill({ state }: { state: unknown }) {
  return <span className={`live-code-status ${stateClass(state)}`}>{stateText(state)}</span>;
}

function QRPreview({ row }: { row: LiveCodeRecord }) {
  const url = imageURL(row);
  return url ? <img className="live-code-qr" src={url} alt="二维码" loading="lazy" /> : <span className="live-code-qr-placeholder">码</span>;
}

function LiveCodeCreateDialog({
  kind,
  open,
  saving,
  error,
  onCancel,
  onSave,
}: {
  kind: LiveCodeKind;
  open: boolean;
  saving: boolean;
  error: string;
  onCancel: () => void;
  onSave: (values: Record<string, unknown>) => void;
}) {
  const access = useDashboardAccess();
  const [name, setName] = useState('');
  const [employees, setEmployees] = useState('');
  const [leadingWords, setLeadingWords] = useState('');
  const [tags, setTags] = useState('');
  const [rooms, setRooms] = useState('[{"roomId": 0, "maxNum": 50}]');
  const [localError, setLocalError] = useState('');
  if (!open) return null;
  const employeeIDs = employees.split(',').map((item) => Number(item.trim())).filter((item) => Number.isInteger(item) && item > 0);
  const tagIDs = tags.split(',').map((item) => Number(item.trim())).filter((item) => Number.isInteger(item) && item > 0);
  const valid = name.trim() !== '' && employeeIDs.length > 0 && (kind === 'channel' || (leadingWords.trim() !== '' && tagIDs.length > 0 && rooms.trim() !== ''));
  const submit = () => {
    if (!valid) return;
    if (kind === 'group') {
      try { JSON.parse(rooms); } catch { setLocalError('群聊配置需要是有效的 JSON。'); return; }
    }
    setLocalError('');
    if (kind === 'channel') {
      onSave({
        baseInfo: { groupId: 0, name: name.trim(), autoAddFriend: 1, tags: [] },
        drainageEmployee: {
          type: 1, employees: [],
          specialPeriod: { status: 1, detail: [{ startDate: '2000-01-01', endDate: '2099-12-31', timeSlot: [{ startTime: '00:00', endTime: '00:00', employeeId: employeeIDs }] }] },
          addMax: { status: 2, employees: [], spareEmployeeIds: [] },
        },
        welcomeMessage: { scanCodePush: 2, messageDetail: [] },
      });
    } else {
      onSave({ corpId: Number(access.corp.id), qrcodeName: name.trim(), isVerified: 2, leadingWords: leadingWords.trim(), employees: employeeIDs, tags: tagIDs, rooms: rooms.trim() });
    }
  };
  return (
    <DashboardDialog
      confirmLoading={saving}
      confirmDisabled={!valid}
      confirmText={saving ? '保存中…' : `保存${kind === 'channel' ? '渠道活码' : '群活码'}`}
      mode="drawer"
      onCancel={onCancel}
      onConfirm={submit}
      open
      title={`新建${kind === 'channel' ? '渠道活码' : '群活码'}`}
    >
      <form className="live-code-form" onSubmit={(event) => { event.preventDefault(); submit(); }}>
        <div className="live-code-form-intro"><strong>先完成基础配置</strong><span>保存后会调用企业微信 Provider，未配置企业授信时不会生成虚假二维码。</span></div>
        <label>名称 <b>*</b><input aria-label={`${kind === 'channel' ? '渠道活码' : '群活码'}名称`} value={name} maxLength={30} onChange={(event) => setName(event.target.value)} placeholder={`例如：${kind === 'channel' ? '官网咨询' : '售后服务群'}`} /></label>
        <label>使用成员 ID <b>*</b><input aria-label="使用成员 ID" inputMode="numeric" value={employees} onChange={(event) => setEmployees(event.target.value)} placeholder="多个 ID 用逗号分隔" /></label>
        {kind === 'group' && <>
          <label>入群引导语 <b>*</b><textarea aria-label="入群引导语" value={leadingWords} maxLength={1000} onChange={(event) => setLeadingWords(event.target.value)} placeholder="欢迎加入我们的服务群" /></label>
          <label>客户标签 ID <b>*</b><input aria-label="客户标签 ID" inputMode="numeric" value={tags} onChange={(event) => setTags(event.target.value)} placeholder="多个 ID 用逗号分隔" /></label>
          <label>群聊配置 JSON <b>*</b><textarea aria-label="群聊配置 JSON" value={rooms} onChange={(event) => setRooms(event.target.value)} /></label>
        </>}
        {(error || localError) && <p className="live-code-form-error" role="alert">{error || localError}</p>}
      </form>
    </DashboardDialog>
  );
}

function DetailDrawer({ kind, row, onClose }: { kind: LiveCodeKind; row: LiveCodeRecord; onClose: () => void }) {
  return (
    <aside className="live-code-detail" aria-label={`${kind === 'channel' ? '渠道活码' : '群活码'}详情`}>
      <button type="button" className="live-code-detail-backdrop" aria-label="关闭详情" onClick={onClose} />
      <div className="live-code-detail-panel">
        <header><div><span className="live-code-kicker">{kind === 'channel' ? '渠道获客' : '社群获客'}</span><h2>{textOf(value(row, 'name', 'qrcodeName'), '未命名活码')}</h2><p>{kind === 'channel' ? '查看归因、人员与有效期信息' : '查看引导语、关联群聊与配置状态'}</p></div><button type="button" className="live-code-close" onClick={onClose} aria-label="关闭详情">×</button></header>
        <div className="live-code-detail-qr"><QRPreview row={row} /><div><StatusPill state={value(row, 'state', 'status', 'isVerified') === 1 ? 'active' : value(row, 'state', 'status')} /><p>{kind === 'channel' ? `新增好友 ${textOf(value(row, 'addedFriendCount', 'contactNum'), '0')} 人` : `关联群聊 ${textOf(value(row, 'roomNum', 'rooms'), '0')}`}</p></div></div>
        <dl>
          <div><dt>创建时间</dt><dd>{textOf(value(row, 'createdAt', 'created_at'), '—')}</dd></div>
          <div><dt>{kind === 'channel' ? '创建人' : '入群引导语'}</dt><dd>{kind === 'channel' ? textOf(value(row, 'creator', 'creatorName')) : textOf(value(row, 'leadingWords'))}</dd></div>
          <div><dt>使用成员</dt><dd>{employeesText(row)}</dd></div>
          {kind === 'channel' ? <>
            <div><dt>客户标签</dt><dd>{textOf(value(row, 'tags'))}</dd></div>
            <div><dt>有效期</dt><dd>{validityText(row)}</dd></div>
            <div><dt>新增好友数</dt><dd>{textOf(value(row, 'addedFriendCount', 'contactNum'), '0')}</dd></div>
            <div><dt>统计口径</dt><dd>{value(row, 'statisticsAvailable') === false ? '暂无可验证数据' : '已接入真实关联记录'}</dd></div>
          </> : <div><dt>关联群聊</dt><dd>{textOf(value(row, 'rooms'))}</dd></div>}
        </dl>
      </div>
    </aside>
  );
}

function StatisticsPanel({ api, access, path }: { api: BusinessWorkbenchApi; access: ReturnType<typeof useDashboardAccess>; path: string }) {
  const today = new Date();
  const defaultEnd = today.toISOString().slice(0, 10);
  const defaultStart = new Date(today.getTime() - 29 * 86400000).toISOString().slice(0, 10);
  const [startDate, setStartDate] = useState(defaultStart);
  const [endDate, setEndDate] = useState(defaultEnd);
  const query = useQuery({
    queryKey: ['live-code-statistics', access.corp.id, startDate, endDate],
    queryFn: () => api.read('/channelCode/workspaceStatistics', { startDate, endDate, page: 1, perPage: 20 }),
  });
  const page = useMemo(() => statisticsFrom(query.data), [query.data]);
  const exportReport = async () => {
    await api.read('/channelCode/export', { startDate, endDate, page: 1, perPage: 100 });
  };
  const summary = page.summary;
  return (
    <div className="live-code-statistics" data-page-path={path}>
      <div className="live-code-statistics-toolbar"><div><h2>渠道效果概览</h2><p>仅展示已接入真实关联记录的结果，统计时间按北京时间计算。</p></div><div className="live-code-date-filter"><label>开始日期<input aria-label="统计开始日期" type="date" value={startDate} onChange={(event) => setStartDate(event.target.value)} /></label><span>至</span><label>结束日期<input aria-label="统计结束日期" type="date" value={endDate} onChange={(event) => setEndDate(event.target.value)} /></label><button type="button" className="live-code-secondary-button" onClick={() => { void exportReport(); }}>导出报表</button></div></div>
      {query.isPending ? <PageState state="loading" /> : query.isError ? <PageState state={pageStateForError(query.error)} onRetry={() => { void query.refetch(); }} /> : <>
        <div className="live-code-stat-cards">{[['新增好友数', summary.addedCustomers], ['新增好友次数', summary.addedAttempts], ['流失好友次数', summary.lostAttempts], ['留存好友数', summary.retainedCustomers]].map(([label, amount]) => <div className="live-code-stat-card" key={String(label)}><span>{label}</span><strong>{amount}</strong></div>)}</div>
        <div className="live-code-stat-note"><span className="live-code-note-dot" />{summary.available ? `统计截至 ${summary.asOf ? new Date(summary.asOf).toLocaleString('zh-CN') : '当前'}，${summary.definition}` : '当前时间范围暂无可验证统计数据。'}</div>
        {page.rows.length === 0 ? <PageState state="empty" title="暂无统计记录" description="调整时间范围后再试，或先创建渠道活码。" /> : <div className="live-code-stat-table"><table><thead><tr><th>活码名称</th><th>新增好友次数</th><th>流失好友次数</th><th>新增好友数</th><th>留存好友数</th></tr></thead><tbody>{page.rows.map((row, index) => <tr key={rowID(row, index)}><td>{textOf(value(row, 'name'))}</td><td>{numberOf(value(row, 'addedAttempts'))}</td><td>{numberOf(value(row, 'lostAttempts'))}</td><td>{numberOf(value(row, 'addedCustomers'))}</td><td>{numberOf(value(row, 'retainedCustomers'))}</td></tr>)}</tbody></table></div>}
      </>}
    </div>
  );
}

export function LiveCodeWorkspacePage({ api, kind }: { api: BusinessWorkbenchApi; kind: LiveCodeKind }) {
  const access = useDashboardAccess();
  const isChannel = kind === 'channel';
  const path = isChannel ? '/acquisition/v2-channel-code' : '/acquisition/group-code';
  const title = isChannel ? '渠道活码' : '群活码';
  const endpoint = isChannel ? '/channelCode/index' : '/workRoomAutoPull/index';
  const nameKey = isChannel ? 'name' : 'qrcodeName';
  const [tab, setTab] = useState<'list' | 'statistics'>('list');
  const [draft, setDraft] = useState({ name: '', creator: '', employee: '', state: '' });
  const [filters, setFilters] = useState({ name: '', creator: '', employee: '', state: '' });
  const [groupId, setGroupId] = useState(0);
  const [pageNumber, setPageNumber] = useState(1);
  const [selected, setSelected] = useState<LiveCodeRecord | null>(null);
  const [createOpen, setCreateOpen] = useState(false);
  const [saving, setSaving] = useState(false);
  const [writeError, setWriteError] = useState('');
  const queryValues: LiveCodeQuery = {
    page: pageNumber, perPage: 20,
    ...(filters.name ? { name: filters.name } : {}),
    ...(isChannel && filters.creator ? { creator: filters.creator } : {}),
    ...(isChannel && filters.employee ? { employee: filters.employee } : {}),
    ...(isChannel && filters.state ? { state: filters.state } : {}),
    ...(isChannel && groupId > 0 ? { groupId } : {}),
  };
  const query = useQuery({ queryKey: ['live-code-workspace', access.corp.id, kind, queryValues], queryFn: () => api.read(endpoint, { ...queryValues, ...(isChannel ? {} : { qrcodeName: filters.name }) }) });
  const rows = useMemo(() => {
    const records = recordsFrom(query.data);
    if (isChannel || !filters.state) return records;
    return records.filter((row) => filters.state === 'verified' ? isVerified(row) : !isVerified(row));
  }, [isChannel, query.data, filters.state]);
  const pagination = useMemo(() => filters.state && !isChannel ? paginationFrom(undefined, rows.length) : paginationFrom(query.data, rows.length), [isChannel, query.data, rows.length, filters.state]);
  const groups = useMemo(() => {
    const names = new Map<number, string>();
    rows.forEach((row) => { const id = numberOf(value(row, 'groupId')); if (id > 0) names.set(id, textOf(value(row, 'groupName'), `分组 ${id}`)); });
    return [...names.entries()].map(([id, name]) => ({ id, name }));
  }, [rows]);
  const hasActionContract = useMemo(() => [...access.allowedActions].some((action) => action.startsWith(`${path}@`)), [access.allowedActions, path]);
  const can = (action: string) => !hasActionContract || access.allowedActions.has(`${path}@${action}`);
  const search = () => { setPageNumber(1); setFilters({ ...draft }); };
  const reset = () => { setDraft({ name: '', creator: '', employee: '', state: '' }); setFilters({ name: '', creator: '', employee: '', state: '' }); setGroupId(0); setPageNumber(1); };
  const save = async (values: Record<string, unknown>) => {
    setSaving(true); setWriteError('');
    try {
      await api.write(isChannel ? '/channelCode/store' : '/workRoomAutoPull/store', values, 'POST');
      setCreateOpen(false); await query.refetch();
    } catch (error) { setWriteError(error instanceof Error ? error.message : `创建${title}失败`); } finally { setSaving(false); }
  };
  return (
    <section className="live-code-page">
      <header className="live-code-header"><div><span className="live-code-kicker">营销工具 · 渠道获客</span><h1>{title}</h1><p>{isChannel ? '用清晰的归因与人员信息管理每一个获客入口。' : '以群为单位管理入群引导、群聊容量与二维码状态。'}</p></div><div className="live-code-header-actions"><span className="live-code-connection"><i />真实数据已连接</span>{can('create') ? <button type="button" onClick={() => { setWriteError(''); setCreateOpen(true); }}>新建{title}</button> : <span className="live-code-readonly">当前账号仅可查看{title}</span>}</div></header>
      <div className="live-code-layout">
        <aside className="live-code-sidebar"><div className="live-code-sidebar-title"><strong>目录</strong><span>{groups.length + 1}</span></div><button type="button" className={groupId === 0 ? 'active' : ''} onClick={() => { setGroupId(0); setPageNumber(1); }}>全部{title}<em>{pagination.total}</em></button>{groups.map((group) => <button type="button" className={groupId === group.id ? 'active' : ''} key={group.id} onClick={() => { setGroupId(group.id); setPageNumber(1); }}>{group.name}</button>)}<div className="live-code-sidebar-tip">按分组查看，列表与统计会保持当前企业权限范围。</div></aside>
        <main className="live-code-main">
          <div className="live-code-tabs"><button type="button" className={tab === 'list' ? 'active' : ''} onClick={() => setTab('list')}>活码列表</button>{isChannel && <button type="button" className={tab === 'statistics' ? 'active' : ''} onClick={() => setTab('statistics')}>统计概览</button>}</div>
          {tab === 'statistics' && isChannel ? <StatisticsPanel api={api} access={access} path={path} /> : <>
            {!isChannel && <div className="live-code-mode-tabs"><button type="button" className={!filters.state ? 'active' : ''} onClick={() => setFilters((old) => ({ ...old, state: '' }))}>全部</button><button type="button" className={filters.state === 'verified' ? 'active' : ''} onClick={() => setFilters((old) => ({ ...old, state: 'verified' }))}>已验证</button><button type="button" className={filters.state === 'pending' ? 'active' : ''} onClick={() => setFilters((old) => ({ ...old, state: 'pending' }))}>待配置</button></div>}
            <div className="live-code-filter"><label>{isChannel ? '活码名称' : '群活码名称'}<input aria-label={isChannel ? '活码名称' : '群活码名称'} value={draft.name} onChange={(event) => setDraft((old) => ({ ...old, name: event.target.value }))} placeholder={isChannel ? '搜索名称' : '搜索群活码'} /></label>{isChannel && <><label>创建人<input aria-label="创建人" value={draft.creator} onChange={(event) => setDraft((old) => ({ ...old, creator: event.target.value }))} placeholder="输入姓名" /></label><label>使用成员<input aria-label="使用成员" value={draft.employee} onChange={(event) => setDraft((old) => ({ ...old, employee: event.target.value }))} placeholder="输入姓名" /></label><label>状态<select aria-label="活码状态" value={draft.state} onChange={(event) => setDraft((old) => ({ ...old, state: event.target.value }))}><option value="">全部状态</option><option value="active">运行中</option><option value="paused">已暂停</option><option value="expired">已过期</option></select></label></>}<div className="live-code-filter-actions"><button type="button" onClick={search}>查询</button><button type="button" className="live-code-secondary-button" onClick={reset}>重置</button></div></div>
            <div className="live-code-table-card"><div className="live-code-table-heading"><div><h2>{isChannel ? '渠道活码列表' : '群活码列表'}</h2><p>{isChannel ? '创建人、使用成员、有效期和效果指标均来自已接入的业务数据。' : '扫码二维码、查看关联群聊并确认群活码配置状态。'}</p></div><span>{pagination.total} 条记录</span></div>{query.isPending ? <PageState state="loading" /> : query.isError ? <PageState state={pageStateForError(query.error)} onRetry={() => { void query.refetch(); }} /> : rows.length === 0 ? <PageState state="empty" title="暂无记录" description="调整筛选条件或创建一条新的配置。" /> : <div className="live-code-table-scroll"><table className="live-code-table"><thead><tr><th>二维码</th><th>{isChannel ? '活码名称' : '群活码名称'}</th><th>{isChannel ? '创建人 / 创建时间' : '入群引导语'}</th><th>{isChannel ? '使用成员' : '关联群聊'}</th><th>{isChannel ? '有效期 / 新增好友' : '状态 / 创建时间'}</th><th>操作</th></tr></thead><tbody>{rows.map((row, index) => <tr key={rowID(row, index)}><td><QRPreview row={row} /></td><td><strong>{textOf(value(row, nameKey, 'name'), '未命名活码')}</strong><small>{textOf(value(row, 'groupName'), '未分组')}</small></td><td>{isChannel ? <><span>{textOf(value(row, 'creator', 'creatorName'))}</span><small>{textOf(value(row, 'createdAt', 'created_at'))}</small></> : <span className="live-code-clamp">{textOf(value(row, 'leadingWords'))}</span>}</td><td>{isChannel ? <span className="live-code-clamp">{employeesText(row)}</span> : <span>{textOf(value(row, 'rooms'), '—')}</span>}</td><td>{isChannel ? <><span>{validityText(row)}</span><small>{value(row, 'statisticsAvailable') === false ? '暂无统计' : `新增好友 ${textOf(value(row, 'addedFriendCount', 'contactNum'), '0')}`}</small></> : <><StatusPill state={isVerified(row) ? 'active' : 'draft'} /><small>{textOf(value(row, 'createdAt', 'created_at'))}</small></>}</td><td><button type="button" className="live-code-link" onClick={() => setSelected(row)}>详情</button></td></tr>)}</tbody></table></div>}<div className="live-code-pagination">{pagination.totalPage > 1 && <><button type="button" disabled={pageNumber <= 1} onClick={() => setPageNumber((old) => Math.max(1, old - 1))}>上一页</button><span>第 {pageNumber} / {pagination.totalPage} 页</span><button type="button" disabled={pageNumber >= pagination.totalPage} onClick={() => setPageNumber((old) => old + 1)}>下一页</button></>}</div></div>
          </>}
        </main>
      </div>
      {selected && <DetailDrawer kind={kind} row={selected} onClose={() => setSelected(null)} />}
      <LiveCodeCreateDialog kind={kind} open={createOpen} saving={saving} error={writeError} onCancel={() => { if (!saving) setCreateOpen(false); }} onSave={(values) => { void save(values); }} />
    </section>
  );
}

export function ChannelCodePage({ api }: { api: BusinessWorkbenchApi }) { return <LiveCodeWorkspacePage api={api} kind="channel" />; }

export function GroupCodePage({ api }: { api: BusinessWorkbenchApi }) { return <LiveCodeWorkspacePage api={api} kind="group" />; }
