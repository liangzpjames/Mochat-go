import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useMemo, useState } from 'react';

import { useDashboardAccess } from '../../app/access-context';
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

export function SensitiveWordPage({ api }: { api: SensitiveWordApi }) {
  const access = useDashboardAccess();
  const queryClient = useQueryClient();
  const corpID = access.corp.id;
  const [partition, setPartition] = useState<Partition>('records');
  const [feedback, setFeedback] = useState<{ kind: 'success' | 'error'; message: string } | null>(null);

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

  const groups = useQuery({ queryKey: groupsKey, queryFn: api.groups });
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
  const canEdit = access.allowedActions.has('/ai-insight/v2/sensitive-word@edit') || canAdd;
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

  return (
	<section className="sensitive-word-page">
	  <header className="dashboard-page-header">
		<div>
		  <h1>敏感词管理</h1>
		  <p>集中查看命中记录，并维护当前企业的敏感词组与词条。</p>
		</div>
	  </header>

	  <nav aria-label="敏感词页面分区" className="dashboard-data-card dashboard-table-actions">
		<button aria-pressed={partition === 'records'} type="button" onClick={() => { setPartition('records'); setFeedback(null); }}>敏感词记录</button>
		<button aria-pressed={partition === 'config'} type="button" onClick={() => { setPartition('config'); setFeedback(null); }}>敏感词配置</button>
	  </nav>

	  {feedback && <p aria-live="polite" role={feedback.kind === 'error' ? 'alert' : 'status'}>{feedback.message}</p>}

	  {partition === 'records' && (
		<section aria-label="敏感词记录">
		  <div className="dashboard-filter-bar">
			<label>员工 ID<input aria-label="员工 ID" placeholder="例如 3,5" value={recordEmployees} onChange={(event) => setRecordEmployees(event.target.value)} /></label>
			<label>客户群 ID<input aria-label="客户群 ID" min={0} type="number" value={recordRoomID || ''} onChange={(event) => setRecordRoomID(Number(event.target.value) || 0)} /></label>
			<label>记录词组<select aria-label="记录词组" value={recordGroupID} onChange={(event) => setRecordGroupID(Number(event.target.value))}><option value={0}>全部词组</option>{(groups.data ?? []).map((group) => <option key={group.id} value={group.id}>{group.name}</option>)}</select></label>
			<label>开始时间<input aria-label="开始时间" type="datetime-local" value={recordStart} onChange={(event) => setRecordStart(event.target.value)} /></label>
			<label>结束时间<input aria-label="结束时间" type="datetime-local" value={recordEnd} onChange={(event) => setRecordEnd(event.target.value)} /></label>
			<button type="button" onClick={applyRecordFilters}>查询记录</button>
		  </div>
		  {queryPending && <PageState state="loading" />}
		  {queryError && <PageState state={pageStateForError(queryError)} onRetry={() => { void groups.refetch(); void records.refetch(); }} />}
		  {!queryPending && !queryError && records.data?.items.length === 0 && <PageState state="empty" />}
		  {!queryPending && !queryError && (records.data?.items.length ?? 0) > 0 && (
			<div className="dashboard-data-card"><div className="dashboard-table-scroll"><table>
			  <thead><tr><th>敏感词</th><th>来源</th><th>触发人</th><th>场景</th><th>触发时间</th><th>操作</th></tr></thead>
			  <tbody>{records.data?.items.map((item) => <tr key={item.id}><td>{item.sensitiveWordName}</td><td>{item.sourceText}</td><td>{item.triggerName}</td><td>{item.triggerScenario}</td><td>{item.triggerTime}</td><td><button type="button" onClick={() => detail.mutate(item.id)}>查看详情</button></td></tr>)}</tbody>
			</table></div></div>
		  )}
		  {detail.isError && <PageState state={pageStateForError(detail.error)} title="扫描结果联通失败" description="消息归档暂时不可用，请稍后重试。" onRetry={() => detail.reset()} />}
		  {detail.data && <div className="dashboard-data-card"><h2>命中上下文</h2><pre>{JSON.stringify(detail.data, null, 2)}</pre></div>}
		</section>
	  )}

	  {partition === 'config' && (
		<section aria-label="敏感词配置">
		  <div className="dashboard-filter-bar">
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
			  <div className="dashboard-data-card"><h2>词组</h2><div className="dashboard-table-scroll"><table>
				<thead><tr><th>词组名称</th><th>操作</th></tr></thead>
				<tbody>{(groups.data ?? []).map((group) => <tr key={group.id}><td><input aria-label={`词组名称 ${group.name}`} value={renameValues[group.id] ?? group.name} onChange={(event) => setRenameValues((current) => ({ ...current, [group.id]: event.target.value }))} /></td><td>{canEdit && <button type="button" onClick={() => renameGroup.mutate({ id: group.id, name: renameValues[group.id] ?? group.name, version: group.version })}>保存改名</button>}</td></tr>)}</tbody>
			  </table></div></div>
			  {(words.data?.items.length ?? 0) === 0 ? <PageState state="empty" /> : <div className="dashboard-data-card"><h2>词条</h2><div className="dashboard-table-scroll"><table>
				<thead><tr><th>敏感词</th><th>分组</th><th>状态</th><th>移动到</th><th>操作</th></tr></thead>
				<tbody>{words.data?.items.map((item) => <tr key={item.id}><td>{item.name}</td><td>{item.groupName}</td><td>{item.status === 1 ? '启用' : '停用'}</td><td><select aria-label={`移动 ${item.name}`} value={moveTargets[item.id] ?? item.groupId} onChange={(event) => setMoveTargets((current) => ({ ...current, [item.id]: Number(event.target.value) }))}>{(groups.data ?? []).map((group) => <option key={group.id} value={group.id}>{group.name}</option>)}</select></td><td><div className="dashboard-table-actions">{canEdit && <button aria-label={`${item.status === 1 ? '停用' : '启用'} ${item.name}`} type="button" onClick={() => setEnabled.mutate({ id: item.id, enabled: item.status !== 1, version: item.version })}>{item.status === 1 ? '停用' : '启用'}</button>}{canEdit && <button aria-label={`确认移动 ${item.name}`} type="button" onClick={() => moveWord.mutate({ id: item.id, targetGroupID: moveTargets[item.id] ?? item.groupId, version: item.version })}>移动</button>}{canDelete && <button aria-label={`删除 ${item.name}`} type="button" onClick={() => { if (window.confirm(`确认删除敏感词“${item.name}”？`)) removeWord.mutate({ id: item.id, version: item.version }); }}>删除</button>}</div></td></tr>)}</tbody>
			  </table></div></div>}
			</>
		  )}
		</section>
	  )}
	</section>
  );
}
