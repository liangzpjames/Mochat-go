import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useMemo, useRef, useState } from 'react';

import { useDashboardAccess } from '../../app/access-context';
import { ConfirmAction } from '../../components/confirm-action';
import { pageStateForError, PageState } from '../../components/page-state/page-state';
import type { SensitiveWordApi, SensitiveWordMatchFilters } from './sensitive-word-api';

type Partition = 'records' | 'config';

const emptyRecordFilters: SensitiveWordMatchFilters = {
  employeeIds: [], workRoomId: 0, groupId: 0, triggerStart: '', triggerEnd: '', page: 1, perPage: 10,
};

function idempotencyKey(action: string, id: number | string = 0): string {
  return `sensitive-word-${action}-${id}-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
}

function dashboardDateTime(value: string): string {
  if (!value) return '';
  return `${value.replace('T', ' ')}${value.length === 16 ? ':00' : ''}`;
}

function mutationFeedback(error: unknown): string {
  if (pageStateForError(error) === 'conflict') return '数据已被其他人更新，请刷新后重试。';
  if (pageStateForError(error) === 'forbidden') return '没有执行此操作的权限。';
  if (error instanceof Error && error.message.includes('套餐额度已达上限')) return error.message;
  return '操作失败，请稍后重试。';
}

function detailValue(value: unknown): string {
  if (value === null || value === undefined) return '--';
  if (Array.isArray(value)) return value.length === 0 ? '--' : value.map(detailValue).join(' · ');
  if (typeof value === 'object') {
	const entries = Object.entries(value as Record<string, unknown>);
	return entries.length === 0 ? '--' : entries.map(([key, item]) => `${key}: ${detailValue(item)}`).join(' · ');
  }
  return typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean' || typeof value === 'bigint' ? String(value) : '--';
}

export function SensitiveWordPage({ api }: { api: SensitiveWordApi }) {
  const access = useDashboardAccess();
  const queryClient = useQueryClient();
  const corpID = access.corp.id;
  const [partition, setPartition] = useState<Partition>('records');
  const [feedback, setFeedback] = useState<{ kind: 'success' | 'error'; message: string } | null>(null);
  const [selectedMatchID, setSelectedMatchID] = useState<number | null>(null);
  const detailCloseRef = useRef<HTMLButtonElement>(null);
  const detailTriggerRef = useRef<HTMLButtonElement | null>(null);

  const [recordEmployees, setRecordEmployees] = useState('');
  const [recordRoomID, setRecordRoomID] = useState(0);
  const [recordGroupID, setRecordGroupID] = useState(0);
  const [recordStart, setRecordStart] = useState('');
  const [recordEnd, setRecordEnd] = useState('');
  const [recordFilters, setRecordFilters] = useState<SensitiveWordMatchFilters>(emptyRecordFilters);

  const [groupID, setGroupID] = useState(0);
  const [keywords, setKeywords] = useState('');
  const [wordName, setWordName] = useState('');
  const [groupName, setGroupName] = useState('');
  const [moveTargets, setMoveTargets] = useState<Record<number, number>>({});
  const [renameValues, setRenameValues] = useState<Record<number, string>>({});

  const groupsKey = useMemo(() => ['sensitive-word', corpID, 'groups'] as const, [corpID]);
  const wordsKey = useMemo(() => ['sensitive-word', corpID, 'words'] as const, [corpID]);
  const recordsKey = useMemo(() => ['sensitive-word', corpID, 'records'] as const, [corpID]);

  const groups = useQuery({ queryKey: groupsKey, queryFn: () => api.groups() });
  const words = useQuery({
	queryKey: [...wordsKey, groupID, keywords],
	queryFn: () => api.list({ groupId: groupID, keywords, page: 1, perPage: 10 }),
	enabled: partition === 'config',
  });
  const records = useQuery({
	queryKey: [...recordsKey, recordFilters],
	queryFn: () => api.matches(recordFilters),
	enabled: partition === 'records',
  });
  const detail = useMutation({ mutationFn: (id: number) => api.matchDetail(id) });

  const success = (message: string, keys: readonly (readonly unknown[])[]) => {
	setFeedback({ kind: 'success', message });
	for (const key of keys) void queryClient.invalidateQueries({ queryKey: key });
  };
  const failed = (error: unknown) => setFeedback({ kind: 'error', message: mutationFeedback(error) });

  const createWord = useMutation({
	mutationFn: () => api.create({ groupId: groupID, names: [wordName.trim()], version: '0', idempotencyKey: idempotencyKey('create-word', wordName.trim()) }),
	onSuccess: () => { setWordName(''); success('操作成功：敏感词已新增。', [wordsKey]); },
	onError: failed,
  });
  const createGroup = useMutation({
	mutationFn: () => api.createGroup({ names: [groupName.trim()], version: '0', idempotencyKey: idempotencyKey('create-group', groupName.trim()) }),
	onSuccess: () => { setGroupName(''); success('操作成功：词组已新增。', [groupsKey]); },
	onError: failed,
  });
  const renameGroup = useMutation({
	mutationFn: ({ id, name, version }: { id: number; name: string; version: string }) => api.renameGroup({ id, name, version, idempotencyKey: idempotencyKey('rename-group', id) }),
	onSuccess: () => success('操作成功：词组已改名。', [groupsKey, wordsKey]),
	onError: failed,
  });
  const setEnabled = useMutation({
	mutationFn: ({ id, enabled, version }: { id: number; enabled: boolean; version: string }) => api.setEnabled({ id, enabled, version, idempotencyKey: idempotencyKey('status', id) }),
	onSuccess: () => success('操作成功：敏感词状态已更新。', [wordsKey]),
	onError: failed,
  });
  const moveWord = useMutation({
	mutationFn: ({ id, targetGroupID, version }: { id: number; targetGroupID: number; version: string }) => api.move({ id, groupId: targetGroupID, version, idempotencyKey: idempotencyKey('move', id) }),
	onSuccess: () => success('操作成功：敏感词已移动。', [wordsKey]),
	onError: failed,
  });
  const removeWord = useMutation({
	mutationFn: ({ id, version }: { id: number; version: string }) => api.remove({ id, version, confirmed: true, idempotencyKey: idempotencyKey('delete', id) }),
	onSuccess: () => success('操作成功：敏感词已删除。', [wordsKey]),
	onError: failed,
  });

  const canAdd = access.allowedActions.has('/ai-insight/v2/sensitive-word@add');
  const canEdit = access.allowedActions.has('/ai-insight/v2/sensitive-word@edit');
  const canDelete = access.allowedActions.has('/ai-insight/v2/sensitive-word@delete');

  const applyRecordFilters = () => {
	const employeeIds = recordEmployees.split(/[，,\s]+/).map(Number).filter((id) => Number.isInteger(id) && id > 0);
	setRecordFilters({
	  employeeIds, workRoomId: recordRoomID, groupId: recordGroupID,
	  triggerStart: dashboardDateTime(recordStart), triggerEnd: dashboardDateTime(recordEnd), page: 1, perPage: 10,
	});
  };

  const queryError = groups.error ?? (partition === 'records' ? records.error : words.error);
  const queryPending = groups.isPending || (partition === 'records' ? records.isPending : words.isPending);
  const recordTotalPages = Math.max(1, Math.ceil((records.data?.total ?? 0) / recordFilters.perPage));

  const changeRecordPage = (page: number) => {
	setRecordFilters((current) => ({ ...current, page }));
  };

  const closeDetail = () => {
	setSelectedMatchID(null);
	detail.reset();
	detailTriggerRef.current?.focus();
  };

  useEffect(() => {
	if (records.data && recordFilters.page > recordTotalPages) changeRecordPage(recordTotalPages);
  }, [recordFilters.page, recordTotalPages, records.data]);

  useEffect(() => {
	if (selectedMatchID === null) return undefined;
	detailCloseRef.current?.focus();
	const onKeyDown = (event: KeyboardEvent) => { if (event.key === 'Escape') closeDetail(); };
	document.addEventListener('keydown', onKeyDown);
	return () => document.removeEventListener('keydown', onKeyDown);
  }, [selectedMatchID]);

  return (
	<section className="sensitive-word-page">
	  <header className="sensitive-word-header dashboard-page-header dashboard-data-card">
		<div>
		  <p className="sensitive-word-eyebrow">风险预警</p>
		  <h1>敏感词管理</h1>
		  <p>集中查看命中记录，并维护当前企业的敏感词组与词条。</p>
		</div>
	  </header>

	  <nav aria-label="敏感词页面分区" className="sensitive-word-partitions dashboard-data-card dashboard-table-actions">
		<button aria-pressed={partition === 'records'} type="button" onClick={() => { setPartition('records'); setFeedback(null); }}>敏感词记录</button>
		<button aria-pressed={partition === 'config'} type="button" onClick={() => { setPartition('config'); setFeedback(null); }}>敏感词配置</button>
	  </nav>

	  {feedback && <p aria-live="polite" role={feedback.kind === 'error' ? 'alert' : 'status'}>{feedback.message}</p>}

	  {partition === 'records' && (
		<section aria-label="敏感词记录" className="sensitive-word-records">
		  <div className="sensitive-word-record-filters dashboard-filter-bar">
			<label>员工 ID<input aria-label="员工 ID" placeholder="例如 3,5" value={recordEmployees} onChange={(event) => setRecordEmployees(event.target.value)} /></label>
			<label>客户群 ID<input aria-label="客户群 ID" min={0} type="number" value={recordRoomID || ''} onChange={(event) => setRecordRoomID(Number(event.target.value) || 0)} /></label>
			<label>记录词组<select aria-label="记录词组" value={recordGroupID} onChange={(event) => setRecordGroupID(Number(event.target.value))}><option value={0}>全部词组</option>{(groups.data ?? []).map((group) => <option key={group.id} value={group.id}>{group.name}</option>)}</select></label>
			<label>开始时间<input aria-label="开始时间" type="datetime-local" value={recordStart} onChange={(event) => setRecordStart(event.target.value)} /></label>
			<label>结束时间<input aria-label="结束时间" type="datetime-local" value={recordEnd} onChange={(event) => setRecordEnd(event.target.value)} /></label>
			<button type="button" onClick={applyRecordFilters}>查询记录</button>
		  </div>
		  {records.data !== undefined && (
			<section aria-label="记录概览" className="sensitive-word-overview">
			  <article><span>命中记录总数</span><strong>{records.data.total}</strong><small>符合当前筛选条件</small></article>
			  <article><span>当前页记录</span><strong>{records.data.items.length}</strong><small>第 {recordFilters.page} / {recordTotalPages} 页</small></article>
			  <article><span>当前词组</span><strong>{recordFilters.groupId > 0 ? (groups.data ?? []).find((group) => group.id === recordFilters.groupId)?.name ?? '指定词组' : '全部词组'}</strong><small>真实监控命中范围</small></article>
			</section>
		  )}
		  <div className="sensitive-word-record-toolbar dashboard-data-card">
			<div><strong>命中记录</strong><span>查看当前企业的真实敏感词触发记录</span></div>
			<button disabled={records.isFetching} onClick={() => void records.refetch()} type="button">刷新记录</button>
		  </div>
		  {queryPending && <PageState state="loading" />}
		  {queryError && <PageState state={pageStateForError(queryError)} onRetry={() => { void groups.refetch(); void records.refetch(); }} />}
		  {!queryPending && !queryError && records.data?.items.length === 0 && <PageState state="empty" />}
		  {!queryPending && !queryError && (records.data?.items.length ?? 0) > 0 && (
			<div className="sensitive-word-record-results dashboard-data-card"><div className="dashboard-table-scroll"><table>
			  <thead><tr><th>敏感词</th><th>来源</th><th>触发人</th><th>场景</th><th>触发时间</th><th>操作</th></tr></thead>
			  <tbody>{records.data?.items.map((item) => <tr key={item.id}><td><strong className="sensitive-word-trigger">{item.sensitiveWordName}</strong></td><td>{item.sourceText}</td><td>{item.triggerName}</td><td>{item.triggerScenario}</td><td>{item.triggerTime}</td><td><button type="button" onClick={(event) => { detailTriggerRef.current = event.currentTarget; setSelectedMatchID(item.id); detail.mutate(item.id); }}>查看详情</button></td></tr>)}</tbody>
			</table></div><footer className="sensitive-word-pagination dashboard-table-actions"><span>共 {records.data?.total ?? 0} 条记录</span><div><button disabled={recordFilters.page <= 1} onClick={() => changeRecordPage(recordFilters.page - 1)} type="button">上一页</button><button disabled={recordFilters.page >= recordTotalPages} onClick={() => changeRecordPage(recordFilters.page + 1)} type="button">下一页</button></div></footer></div>
		  )}
		  {selectedMatchID !== null && (
			<div className="sensitive-word-detail-backdrop">
			  <aside aria-label="命中详情" aria-modal="true" className="sensitive-word-detail" role="dialog">
				<header><div><p>敏感词记录</p><h2>命中详情</h2></div><button aria-label="关闭详情" onClick={closeDetail} ref={detailCloseRef} type="button">关闭</button></header>
				{detail.isPending && <PageState state="loading" title="正在加载命中详情" />}
				{detail.isError && <PageState state={pageStateForError(detail.error)} title="扫描结果联通失败" description="消息归档暂时不可用，请稍后重试。" onRetry={() => detail.mutate(selectedMatchID)} />}
				{detail.data && <div className="sensitive-word-detail-list">{detail.data.map((row, index) => <article key={`${selectedMatchID}-${index}`}><dl>{Object.entries(row).map(([key, value]) => <div key={key}><dt>{key}</dt><dd>{detailValue(value)}</dd></div>)}</dl></article>)}</div>}
			  </aside>
			</div>
		  )}
		</section>
	  )}

	  {partition === 'config' && (
		<section aria-label="敏感词配置" className="sensitive-word-config">
		  <div className="sensitive-word-config-actions dashboard-filter-bar dashboard-data-card">
			<label>新词组名称<input aria-label="新词组名称" value={groupName} onChange={(event) => setGroupName(event.target.value)} /></label>
			{canAdd && <button disabled={!groupName.trim() || createGroup.isPending} type="button" onClick={() => createGroup.mutate()}>新增词组</button>}
			<label>敏感词名称<input aria-label="敏感词名称" value={wordName} onChange={(event) => setWordName(event.target.value)} /></label>
			<label>敏感词分组<select aria-label="敏感词分组" value={groupID} onChange={(event) => setGroupID(Number(event.target.value))}><option value={0}>请选择词组</option>{(groups.data ?? []).map((group) => <option key={group.id} value={group.id}>{group.name}</option>)}</select></label>
			{canAdd && <button disabled={!wordName.trim() || groupID <= 0 || createWord.isPending} type="button" onClick={() => createWord.mutate()}>新增敏感词</button>}
			<label>搜索敏感词<input aria-label="搜索敏感词" value={keywords} onChange={(event) => setKeywords(event.target.value)} /></label>
		  </div>
		  {queryPending && <PageState state="loading" />}
		  {queryError && <PageState state={pageStateForError(queryError)} onRetry={() => { void groups.refetch(); void words.refetch(); }} />}
		  {!queryPending && !queryError && (
			<>
			  <div className="sensitive-word-config-card dashboard-data-card"><header><h2>词组管理</h2><span>{(groups.data ?? []).length} 个词组</span></header><div className="dashboard-table-scroll"><table>
				<thead><tr><th>词组名称</th><th>操作</th></tr></thead>
				<tbody>{(groups.data ?? []).map((group) => <tr key={group.id}><td><input aria-label={`词组名称 ${group.name}`} value={renameValues[group.id] ?? group.name} onChange={(event) => setRenameValues((current) => ({ ...current, [group.id]: event.target.value }))} /></td><td>{canEdit && <button type="button" onClick={() => renameGroup.mutate({ id: group.id, name: renameValues[group.id] ?? group.name, version: group.version })}>保存改名</button>}</td></tr>)}</tbody>
			  </table></div></div>
			  {(words.data?.items.length ?? 0) === 0 ? <PageState state="empty" /> : <div className="sensitive-word-config-card dashboard-data-card"><header><h2>敏感词条</h2><span>{words.data?.total ?? 0} 个词条</span></header><div className="dashboard-table-scroll"><table>
				<thead><tr><th>敏感词</th><th>分组</th><th>状态</th><th>移动到</th><th>操作</th></tr></thead>
				<tbody>{words.data?.items.map((item) => <tr key={item.id}><td>{item.name}</td><td>{item.groupName}</td><td>{item.status === 1 ? '启用' : '停用'}</td><td><select aria-label={`移动 ${item.name}`} value={moveTargets[item.id] ?? item.groupId} onChange={(event) => setMoveTargets((current) => ({ ...current, [item.id]: Number(event.target.value) }))}>{(groups.data ?? []).map((group) => <option key={group.id} value={group.id}>{group.name}</option>)}</select></td><td><div className="dashboard-table-actions">{canEdit && <ConfirmAction title={`确认${item.status === 1 ? '停用' : '启用'}敏感词“${item.name}”？`} onConfirm={() => setEnabled.mutate({ id: item.id, enabled: item.status !== 1, version: item.version })}><button aria-label={`${item.status === 1 ? '停用' : '启用'} ${item.name}`} type="button">{item.status === 1 ? '停用' : '启用'}</button></ConfirmAction>}{canEdit && <button aria-label={`确认移动 ${item.name}`} type="button" onClick={() => moveWord.mutate({ id: item.id, targetGroupID: moveTargets[item.id] ?? item.groupId, version: item.version })}>移动</button>}{canDelete && <ConfirmAction title={`确认删除敏感词“${item.name}”？`} description="删除后无法恢复。" onConfirm={() => removeWord.mutate({ id: item.id, version: item.version })}><button aria-label={`删除 ${item.name}`} type="button">删除</button></ConfirmAction>}</div></td></tr>)}</tbody>
			  </table></div></div>}
			</>
		  )}
		</section>
	  )}
	</section>
  );
}
