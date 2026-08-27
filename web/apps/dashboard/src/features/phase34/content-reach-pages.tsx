import { useQuery } from '@tanstack/react-query';
import { useEffect, useMemo, useRef, useState } from 'react';

import { useDashboardAccess } from '../../app/access-context';
import { ConfirmAction } from '../../components/confirm-action';
import { DashboardDialog } from '../../components/dashboard-dialog';
import { pageStateForError, PageState } from '../../components/page-state/page-state';
import type { BusinessWorkbenchApi } from '../business-workbench/business-workbench-page';
import { MaterialSelector } from './material-management/material-selector';

type ReachRecord = Record<string, unknown>;
type SendMode = 'contact' | 'room';

function isRecord(value: unknown): value is ReachRecord {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function rowsFrom(payload: unknown): ReachRecord[] {
  if (Array.isArray(payload)) return payload.filter(isRecord);
  if (!isRecord(payload)) return [];
  const data = isRecord(payload.data) ? payload.data : payload;
  const candidate = [data.list, data.items, data.rows, isRecord(data.data) ? data.data.list : undefined].find(Array.isArray);
  return Array.isArray(candidate) ? candidate.filter(isRecord) : [];
}

function businessErrorMessage(error: unknown, fallback: string): string {
  const message = error instanceof Error ? error.message.trim() : '';
  return message !== '' && !/provider/i.test(message) ? message : fallback;
}

function selectorTotalPages(payload: unknown): number | null {
  if (!isRecord(payload)) return null;
  const data = isRecord(payload.data) ? payload.data : payload;
  const page = isRecord(data.page) ? data.page : undefined;
  const pagination = isRecord(data.pagination) ? data.pagination : undefined;
  const candidates = [
    data.totalPage, data.total_page,
    page?.totalPage, page?.total_page,
    pagination?.totalPage, pagination?.total_page,
  ];
  for (const candidate of candidates) {
    const value = Number(candidate);
    if (Number.isInteger(value) && value > 0) return value;
  }
  return null;
}

async function fetchAllSelectorPages(api: BusinessWorkbenchApi, endpoint: string, query: Record<string, string | number>): Promise<unknown> {
  const perPage = 100;
  const first = await api.read(endpoint, { ...query, page: 1, perPage });
  const merged = rowsFrom(first);
  const totalPages = selectorTotalPages(first);
  let page = 2;
  while (totalPages !== null ? page <= totalPages : merged.length > 0 && merged.length % perPage === 0) {
    const next = await api.read(endpoint, { ...query, page, perPage });
    const nextRows = rowsFrom(next);
    if (nextRows.length === 0) break;
    const existingKeys = new Set(merged.map((row, index) => recordKey(row, index)));
    const newRows = nextRows.filter((row, index) => !existingKeys.has(recordKey(row, index)));
    if (newRows.length === 0) throw new Error('selector pagination did not advance');
    merged.push(...newRows);
    if (totalPages === null && nextRows.length < perPage) break;
    page += 1;
  }
  return { list: merged, page: { totalPage: totalPages ?? page - 1 } };
}

function primitive(value: unknown): string {
  if (value === undefined || value === null || value === '') return '--';
  if (typeof value === 'string') return value;
  if (typeof value === 'number' || typeof value === 'boolean' || typeof value === 'bigint') return value.toString();
  return '--';
}

function contentText(value: unknown): string {
  if (typeof value === 'string') {
    try {
      return contentText(JSON.parse(value));
    } catch {
      return primitive(value);
    }
  }
  if (Array.isArray(value)) {
    const text = value.map((item) => isRecord(item) ? primitive(item.content ?? item.title ?? item.name) : primitive(item)).filter((item) => item !== '--').join('；');
    return text || '--';
  }
  return isRecord(value) ? primitive(value.text ?? value.content ?? value.title ?? value.name) : primitive(value);
}

function statusText(value: unknown): string {
	if (typeof value === 'string') {
		const labels: Record<string, string> = {
			pending: '待执行', queued: '排队中', claimed: '已领取', submitting: '提交中',
			submitted: '已提交', polling: '轮询中', succeeded: '已成功',
			partial_failed: '部分失败', failed: '失败', cancelled: '已取消',
		};
		if (labels[value]) return labels[value];
		if (value.trim() !== '') return value;
	}
  if (value === 0) return '待执行';
  if (value === 1) return '执行中';
  if (value === 2) return '已完成';
  if (value === 3) return '部分失败';
  return '--';
}

function executionData(row: ReachRecord): string {
	const operation = isRecord(row.operation) ? row.operation : row;
	if (operation.targetTotal !== undefined || operation.successTotal !== undefined || operation.failureTotal !== undefined) {
		return `${primitive(operation.successTotal)} / ${primitive(operation.failureTotal)} / ${primitive(operation.targetTotal)}`;
	}
	return `${primitive(row.sendTotal)} / ${primitive(row.receivedTotal)}`;
}

function operationStatus(row: ReachRecord): unknown {
	const operation = isRecord(row.operation) ? row.operation : row;
	return operation.status ?? operation.operationStatus ?? row.sendStatus;
}

function operationRecord(row: ReachRecord): ReachRecord | null {
	return isRecord(row.operation) ? row.operation : null;
}

function batchIDFromRow(row: ReachRecord): number {
	return Number(row.id ?? row.batchId);
}

function recordKey(row: ReachRecord, index: number): string {
  const value = row.id ?? row.batchId;
  return typeof value === 'string' || typeof value === 'number' ? value.toString() : index.toString();
}

function ReachTabs({ active, onChange }: { active: SendMode; onChange: (mode: SendMode) => void }) {
  return (
    <div className="phase34-tabs" role="tablist" aria-label="群发类型">
      <button type="button" role="tab" aria-controls="precise-send-panel" aria-selected={active === 'contact'} className={active === 'contact' ? 'phase34-tab-active' : ''} onClick={() => onChange('contact')}>客户群发</button>
      <button type="button" role="tab" aria-controls="precise-send-panel" aria-selected={active === 'room'} className={active === 'room' ? 'phase34-tab-active' : ''} onClick={() => onChange('room')}>群聊群发</button>
    </div>
  );
}

type SendDetailPayload = { show: ReachRecord; employees: ReachRecord[]; receives: ReachRecord[] };

async function fetchSendDetail(api: BusinessWorkbenchApi, row: ReachRecord, mode: SendMode): Promise<SendDetailPayload> {
  const batchId = batchIDFromRow(row);
  if (!Number.isInteger(batchId) || batchId <= 0) throw new Error('任务标识无效');
  const showPromise = api.read('/contactMessageBatchSend/show', { batchId });
  if (mode !== 'contact') {
    const showRaw = await showPromise;
    return { show: isRecord(showRaw) && isRecord(showRaw.data) ? showRaw.data : isRecord(showRaw) ? showRaw : row, employees: [], receives: [] };
  }
  const [showRaw, employeeRaw, receiveRaw] = await Promise.all([
    showPromise,
    fetchAllSelectorPages(api, '/contactMessageBatchSend/employeeSendIndex', { batchId }),
    fetchAllSelectorPages(api, '/contactMessageBatchSend/contactReceiveIndex', { batchId }),
  ]);
  const show = isRecord(showRaw) && isRecord(showRaw.data) ? showRaw.data : isRecord(showRaw) ? showRaw : row;
  return { show, employees: rowsFrom(employeeRaw), receives: rowsFrom(receiveRaw) };
}

function SendDetail({ api, mode, row, onClose, onChanged }: { api: BusinessWorkbenchApi; mode: SendMode; row: ReachRecord; onClose: () => void; onChanged: () => void }) {
  const detailQuery = useQuery({
    queryKey: ['phase34-precise-send-detail', mode, batchIDFromRow(row)],
    queryFn: () => fetchSendDetail(api, row, mode),
    retry: 1,
  });
  const [actionError, setActionError] = useState('');
  const [actionBusy, setActionBusy] = useState(false);
  const detail = detailQuery.data;
  const shown = detail?.show ?? row;
  const detailData = detail ?? { show: shown, employees: [], receives: [] };
  const operation = operationRecord(shown);
  const status = primitive(operationStatus(shown));
  const canCancel = operation !== null && status === 'pending';
  const canRemind = operation !== null && (status === 'submitted' || status === 'polling');
  const runAction = async (action: 'cancel' | 'remind') => {
    setActionBusy(true);
    setActionError('');
    try {
      const batchId = batchIDFromRow(shown);
      if (action === 'cancel') await api.write('/contactMessageBatchSend/destroy', { batchId }, 'DELETE');
      else await api.write('/contactMessageBatchSend/remind', { batchId }, 'POST');
      await detailQuery.refetch();
      onChanged();
    } catch (error) {
      setActionError(businessErrorMessage(error, '操作失败，请重试。'));
    } finally {
      setActionBusy(false);
    }
  };
  const fields = [
    ['任务名称', primitive(shown.batchTitle ?? shown.title)],
    ['发送内容', contentText(shown.content)],
    ['执行结果', statusText(operationStatus(shown))],
    ['执行数据', executionData(shown)],
    ['创建时间', primitive(shown.createdAt)],
    ['执行时间', primitive(shown.sendTime ?? shown.definiteTime)],
  ];
  return (
    <aside className="phase34-detail" aria-label="群发任务详情">
      <div className="phase34-detail-backdrop" onClick={onClose} />
      <div className="phase34-detail-panel">
        <div className="dashboard-card-heading"><div><p className="phase34-eyebrow">营销工具</p><h2>群发任务详情</h2></div><button type="button" onClick={onClose}>关闭</button></div>
        {detailQuery.isPending ? <PageState state="loading" /> : detailQuery.isError ? <PageState state={pageStateForError(detailQuery.error)} onRetry={() => { void detailQuery.refetch(); }} /> : <>
          <dl>{fields.map(([label, value]) => <div key={label}><dt>{label}</dt><dd>{value}</dd></div>)}</dl>
          {operation && <dl><div><dt>目标数</dt><dd>{primitive(operation.targetTotal)}</dd></div><div><dt>成功数</dt><dd>{primitive(operation.successTotal)}</dd></div><div><dt>失败数</dt><dd>{primitive(operation.failureTotal)}</dd></div><div><dt>错误码</dt><dd>{primitive(operation.errorCode)}</dd></div></dl>}
          {actionError && <p role="alert" className="phase34-inline-error">{actionError}</p>}
          {mode === 'contact' && <>
            <div className="dashboard-table-actions">
              {canCancel && <ConfirmAction title="确认取消该群发任务？" description="取消后不会再向企微提交新的外发请求。" onConfirm={() => { void runAction('cancel'); }}><button type="button" disabled={actionBusy}>取消任务</button></ConfirmAction>}
              {canRemind && <ConfirmAction title="确认提醒发送员工？" description="仅提醒已被企微受理的任务目标。" onConfirm={() => { void runAction('remind'); }}><button type="button" disabled={actionBusy}>提醒</button></ConfirmAction>}
            </div>
            <section aria-label="员工发送结果"><h3>员工发送结果</h3>{detailData.employees.length === 0 ? <PageState state="empty" title="暂无员工结果" /> : <div className="dashboard-table-scroll"><table className="phase34-table"><thead><tr><th>员工</th><th>状态</th><th>客户数</th></tr></thead><tbody>{detailData.employees.map((item, index) => <tr key={recordKey(item, index)}><td>{primitive(item.employeeName)}</td><td>{statusText(item.status)}</td><td>{primitive(item.sendContactTotal)}</td></tr>)}</tbody></table></div>}</section>
            <section aria-label="客户发送结果"><h3>客户发送结果</h3>{detailData.receives.length === 0 ? <PageState state="empty" title="暂无客户结果" /> : <div className="dashboard-table-scroll"><table className="phase34-table"><thead><tr><th>客户</th><th>员工</th><th>状态</th></tr></thead><tbody>{detailData.receives.map((item, index) => <tr key={recordKey(item, index)}><td>{primitive(item.contactName ?? item.contactNickName)}</td><td>{primitive(item.employeeName)}</td><td>{statusText(item.status)}</td></tr>)}</tbody></table></div>}</section>
          </>}
        </>}
      </div>
    </aside>
  );
}

function positiveReachIDs(value: string): number[] {
  return value.split(',').map((item) => Number(item.trim())).filter((item) => Number.isInteger(item) && item > 0);
}

function positiveContactTargetKeys(value: string): string[] {
  return value.split(',').map((item) => item.trim()).filter((item, index, values) => {
    const parts = item.split(':');
    const employeeID = Number(parts[0] ?? '');
    const contactID = Number(parts[1] ?? '');
    return Number.isInteger(employeeID) && employeeID > 0 && Number.isInteger(contactID) && contactID > 0 && values.indexOf(item) === index;
  });
}

function contactTargetFromKey(value: string): { employeeId: number; contactId: number } | null {
  const parts = value.split(':');
  const employeeId = Number(parts[0] ?? '');
  const contactId = Number(parts[1] ?? '');
  if (!Number.isInteger(employeeId) || employeeId <= 0 || !Number.isInteger(contactId) || contactId <= 0) return null;
  return { employeeId, contactId };
}

function positiveRoomTargetKeys(value: string): string[] {
  return value.split(',').map((item) => item.trim()).filter((item, index, values) => {
    const parts = item.split(':');
    const ownerEmployeeID = Number(parts[0] ?? '');
    const roomID = Number(parts[1] ?? '');
    return Number.isInteger(ownerEmployeeID) && ownerEmployeeID > 0 && Number.isInteger(roomID) && roomID > 0 && values.indexOf(item) === index;
  });
}

function roomTargetFromKey(value: string): { ownerEmployeeId: number; roomId: number } | null {
  const parts = value.split(':');
  const ownerEmployeeId = Number(parts[0] ?? '');
  const roomId = Number(parts[1] ?? '');
  if (!Number.isInteger(ownerEmployeeId) || ownerEmployeeId <= 0 || !Number.isInteger(roomId) || roomId <= 0) return null;
  return { ownerEmployeeId, roomId };
}

function roomTargetKeyFromRow(row: ReachRecord, ownerEmployeeIDs: number[]): string {
  const roomID = Number(row.roomId ?? row.workRoomId ?? row.id);
  const rowOwnerID = Number(row.ownerId ?? row.ownerEmployeeId);
  const singleOwnerID = ownerEmployeeIDs.length === 1 ? Number(ownerEmployeeIDs[0]) : 0;
  const ownerEmployeeID = Number.isInteger(rowOwnerID) && rowOwnerID > 0 ? rowOwnerID : singleOwnerID;
  return Number.isInteger(roomID) && roomID > 0 && Number.isInteger(ownerEmployeeID) && ownerEmployeeID > 0 ? `${ownerEmployeeID}:${roomID}` : '--';
}

function newContactBatchIdempotencyKey(): string {
  const randomUUID = globalThis.crypto?.randomUUID?.();
  return `contact-${randomUUID ?? `${Date.now()}-${Math.random().toString(16).slice(2)}`}`;
}

function SendCreateDrawer({
  api,
  mode,
  title,
  employeeIDs,
  contactTargets,
  roomTargets,
  content,
  materialID,
  sendWay,
  definiteTime,
  saving,
  error,
  canSubmit,
  confirmationSummary,
  onTitleChange,
  onEmployeeIDsChange,
  onContactTargetsChange,
  onRoomTargetsChange,
  onContentChange,
  onMaterialChange,
  onSendWayChange,
  onDefiniteTimeChange,
  onClose,
  onSubmit,
}: {
  api: BusinessWorkbenchApi;
  mode: SendMode;
  title: string;
  employeeIDs: string;
  contactTargets: string;
  roomTargets: string;
  content: string;
  materialID: number;
  sendWay: string;
  definiteTime: string;
  saving: boolean;
  error: string;
  canSubmit: boolean;
  confirmationSummary: string;
  onTitleChange: (value: string) => void;
  onEmployeeIDsChange: (value: string) => void;
  onContactTargetsChange: (value: string) => void;
  onRoomTargetsChange: (value: string) => void;
  onContentChange: (value: string) => void;
  onMaterialChange: (value: number) => void;
  onSendWayChange: (value: string) => void;
  onDefiniteTimeChange: (value: string) => void;
  onClose: () => void;
  onSubmit: () => void;
}) {
  const [employeeSearch, setEmployeeSearch] = useState('');
  const [contactSearch, setContactSearch] = useState('');
  const [roomSearch, setRoomSearch] = useState('');
  const selectedEmployeeIDs = new Set(positiveReachIDs(employeeIDs));
  const selectedContactTargetKeys = new Set(positiveContactTargetKeys(contactTargets));
  const selectedRoomTargetKeys = new Set(positiveRoomTargetKeys(roomTargets));
  const employeesQuery = useQuery({
    queryKey: ['precise-send-employees', mode, employeeSearch],
    queryFn: () => fetchAllSelectorPages(api, '/workEmployee/index', employeeSearch ? { name: employeeSearch } : {}),
    enabled: true,
  });
  const contactsQuery = useQuery({
    queryKey: ['precise-send-contacts', mode, contactSearch, employeeIDs],
    queryFn: () => fetchAllSelectorPages(api, '/workContact/index', { ...(contactSearch ? { keyWords: contactSearch } : {}), employeeId: employeeIDs }),
    enabled: mode === 'contact' && selectedEmployeeIDs.size > 0,
  });
  const roomsQuery = useQuery({
    queryKey: ['precise-send-rooms', mode, roomSearch, employeeIDs],
    queryFn: async () => {
      const ownerIDs = [...selectedEmployeeIDs];
      const merged: ReachRecord[] = [];
      for (const ownerID of ownerIDs) {
        const payload = await fetchAllSelectorPages(api, '/workRoom/index', { workRoomOwnerId: String(ownerID), ...(roomSearch ? { workRoomName: roomSearch } : {}) });
        for (const row of rowsFrom(payload)) {
          merged.push({ ...row, ownerEmployeeId: ownerID });
        }
      }
      return { list: merged };
    },
    enabled: mode === 'room' && selectedEmployeeIDs.size > 0,
  });
  const employeeOptions = rowsFrom(employeesQuery.data);
  const contactOptions = rowsFrom(contactsQuery.data);
  const roomOptions = rowsFrom(roomsQuery.data);
  const employeeOptionIDs = new Set(employeeOptions.map((row) => primitive(row.id ?? row.employeeId)).filter((value) => value !== '--'));
  const contactOptionKeys = new Set(contactOptions.map((row) => {
    const employeeID = Number(row.employeeId);
    const contactID = Number(row.contactId);
    return Number.isInteger(employeeID) && employeeID > 0 && Number.isInteger(contactID) && contactID > 0 ? `${employeeID}:${contactID}` : '--';
  }).filter((value) => value !== '--'));
  const roomOptionKeys = new Set(roomOptions.map((row) => roomTargetKeyFromRow(row, [...selectedEmployeeIDs])).filter((value) => value !== '--'));
  const employeeSelectionReady = employeesQuery.isSuccess && !employeesQuery.isFetching && selectedEmployeeIDs.size > 0 && [...selectedEmployeeIDs].every((id) => employeeOptionIDs.has(String(id)));
  const contactSelectionReady = mode !== 'contact' || (contactsQuery.isSuccess && !contactsQuery.isFetching && selectedContactTargetKeys.size > 0 && [...selectedContactTargetKeys].every((key) => contactOptionKeys.has(key)));
  const roomSelectionReady = mode !== 'room' || (roomsQuery.isSuccess && !roomsQuery.isFetching && selectedRoomTargetKeys.size > 0 && [...selectedRoomTargetKeys].every((key) => roomOptionKeys.has(key)));
  const selectorsReady = employeeSelectionReady && contactSelectionReady && roomSelectionReady;

  return (
    <aside className="phase34-detail" aria-label="新建群发任务">
      <div className="phase34-detail-backdrop" aria-hidden="true" onClick={onClose} />
      <div className="phase34-detail-panel">
        <div className="dashboard-card-heading"><div><p className="phase34-eyebrow">营销工具 · 内容触达</p><h2>新建群发任务</h2><p>{mode === 'contact' ? '客户群发' : '群聊群发'}会按所选范围创建真实任务。</p></div><button type="button" aria-label="关闭新建群发" onClick={onClose}>关闭</button></div>
        <form className="phase34-detail-form" onSubmit={(event) => { event.preventDefault(); }}>
          <label><span>任务名称 <b aria-hidden="true">*</b></span><input aria-label="任务名称" required value={title} onChange={(event) => onTitleChange(event.target.value)} placeholder="请输入任务名称" /></label>
          <label><span>{mode === 'contact' ? '发送成员' : '群主'} <b aria-hidden="true">*</b></span><input aria-label={mode === 'contact' ? '搜索发送成员' : '搜索群主'} value={employeeSearch} onChange={(event) => setEmployeeSearch(event.target.value)} placeholder="搜索姓名或部门" /></label>
          <select aria-label={mode === 'contact' ? '发送成员选择' : '群主选择'} multiple required value={[...selectedEmployeeIDs].map(String)} onChange={(event) => onEmployeeIDsChange(Array.from(event.target.selectedOptions).map((option) => option.value).join(','))}>
            {employeeOptions.map((row, index) => { const id = row.id ?? row.employeeId; const value = primitive(id); return value === '--' ? null : <option key={value || index} value={value}>{primitive(row.name ?? row.employeeName)}{row.departmentName ? ` · ${primitive(row.departmentName)}` : ''}</option>; })}
          </select>
          {employeesQuery.isPending && <p role="status" className="phase34-field-hint">正在加载可用员工…</p>}
          {employeesQuery.isError && <p role="alert" className="phase34-inline-error">员工列表加载失败，请重试后再提交。</p>}
          {employeesQuery.isSuccess && employeeOptions.length === 0 && <p role="status" className="phase34-field-hint">暂无当前权限范围内的员工。</p>}
          {mode === 'contact' && <label><span>客户 <b aria-hidden="true">*</b></span><input aria-label="搜索客户" value={contactSearch} onChange={(event) => setContactSearch(event.target.value)} placeholder="搜索客户名称" /></label>}
          {mode === 'contact' && <select aria-label="客户选择" multiple required value={[...selectedContactTargetKeys]} onChange={(event) => onContactTargetsChange(Array.from(event.target.selectedOptions).map((option) => option.value).join(','))}>
            {contactOptions.map((row, index) => { const employeeID = Number(row.employeeId); const contactID = Number(row.contactId); const value = Number.isInteger(employeeID) && employeeID > 0 && Number.isInteger(contactID) && contactID > 0 ? `${employeeID}:${contactID}` : '--'; return value === '--' ? null : <option key={value || index} value={value}>{primitive(row.name ?? row.contactName ?? row.nickname)}{row.employeeName ? ` · ${primitive(row.employeeName)}` : ''}</option>; })}
          </select>}
          {mode === 'contact' && selectedEmployeeIDs.size === 0 && <p role="status" className="phase34-field-hint">请先选择发送成员，再加载其名下客户。</p>}
          {mode === 'contact' && contactsQuery.isPending && <p role="status" className="phase34-field-hint">正在加载所选员工名下客户…</p>}
          {mode === 'contact' && contactsQuery.isError && <p role="alert" className="phase34-inline-error">客户列表加载失败，请重试后再提交。</p>}
          {mode === 'contact' && contactsQuery.isSuccess && contactOptions.length === 0 && <p role="status" className="phase34-field-hint">所选员工名下暂无可发送客户。</p>}
          {mode === 'room' && <label><span>群聊 <b aria-hidden="true">*</b></span><input aria-label="搜索群聊" value={roomSearch} onChange={(event) => setRoomSearch(event.target.value)} placeholder="搜索群名称" /></label>}
          {mode === 'room' && <select aria-label="群聊选择" multiple required value={[...selectedRoomTargetKeys]} onChange={(event) => onRoomTargetsChange(Array.from(event.target.selectedOptions).map((option) => option.value).join(','))}>
            {roomOptions.map((row, index) => { const value = roomTargetKeyFromRow(row, [...selectedEmployeeIDs]); return value === '--' ? null : <option key={value || index} value={value}>{primitive(row.name ?? row.roomName)}{row.ownerName ? ` · ${primitive(row.ownerName)}` : ''}</option>; })}
          </select>}
          {mode === 'room' && selectedEmployeeIDs.size === 0 && <p role="status" className="phase34-field-hint">请先选择群主，再加载其名下群聊。</p>}
          {mode === 'room' && roomsQuery.isPending && <p role="status" className="phase34-field-hint">正在加载所选群主名下群聊…</p>}
          {mode === 'room' && roomsQuery.isError && <p role="alert" className="phase34-inline-error">群聊列表加载失败，请重试后再提交。</p>}
          {mode === 'room' && roomsQuery.isSuccess && roomOptions.length === 0 && <p role="status" className="phase34-field-hint">所选群主名下暂无客户群。</p>}
          <p id="precise-send-target-hint" className="phase34-field-hint">来源：当前企业已同步的{mode === 'contact' ? '员工及其企业微信成员 ID' : '群主与群聊 ID'}；暂无可选数据时请先完成同步。</p>
          <MaterialSelector api={api} scene="group_send" value={materialID || null} disabled={saving} onChange={(item) => { onMaterialChange(item?.id ?? 0); if (item) onContentChange(item.preview); }} />
          <label><span>群发内容 <b aria-hidden="true">*</b></span><textarea aria-label="群发内容" required value={content} onChange={(event) => onContentChange(event.target.value)} placeholder="请输入文本内容" rows={6} /></label>
          <label>发送方式<select aria-label="发送方式" value={sendWay} onChange={(event) => onSendWayChange(event.target.value)}><option value="1">立即发送</option><option value="2">定时发送</option></select></label>
          {sendWay === '2' && <label>定时发送时间<input aria-label="定时发送时间" type="datetime-local" required value={definiteTime} onChange={(event) => onDefiniteTimeChange(event.target.value)} /></label>}
          {error && <p role="alert" className="phase34-inline-error">{error}</p>}
          <p className="phase34-field-hint">发送前系统会校验成员、群主、内容和企业微信配置；失败原因会保留在任务状态中。</p>
          <div className="dashboard-table-actions"><button type="button" className="phase34-secondary-button" onClick={onClose}>取消</button><ConfirmAction title="确认创建精准群发任务？" description={confirmationSummary} onConfirm={onSubmit}><button type="button" disabled={saving || !canSubmit || !selectorsReady}>{saving ? '提交中…' : '保存并发送'}</button></ConfirmAction></div>
        </form>
      </div>
    </aside>
  );
}

export function PreciseGroupSendPage({ api }: { api: BusinessWorkbenchApi }) {
  const access = useDashboardAccess();
  const [mode, setMode] = useState<SendMode>('contact');
  const [draftTitle, setDraftTitle] = useState('');
  const [title, setTitle] = useState('');
  const [selected, setSelected] = useState<ReachRecord | null>(null);
  const [createOpen, setCreateOpen] = useState(false);
  const [createTitle, setCreateTitle] = useState('');
  const [createEmployeeIDs, setCreateEmployeeIDs] = useState('');
  const [createContactTargets, setCreateContactTargets] = useState('');
  const [createRoomTargets, setCreateRoomTargets] = useState('');
  const [createContent, setCreateContent] = useState('');
  const [createMaterialID, setCreateMaterialID] = useState(0);
  const [createSendWay, setCreateSendWay] = useState('1');
  const [createDefiniteTime, setCreateDefiniteTime] = useState('');
  const [createIdempotencyKey, setCreateIdempotencyKey] = useState('');
  const [saving, setSaving] = useState(false);
  const [writeError, setWriteError] = useState('');
  const endpoint = mode === 'contact' ? '/contactMessageBatchSend/index' : '/roomMessageBatchSend/index';
  const query = useQuery({
    queryKey: ['phase34-precise-send', access.corp.id, mode, title],
    queryFn: () => api.read(endpoint, { ...(title ? { batchTitle: title } : {}), page: 1, perPage: 20 }),
  });
  const rows = useMemo(() => rowsFrom(query.data), [query.data]);
  const refresh = () => { void query.refetch(); };
  const createEndpoint = mode === 'contact' ? '/contactMessageBatchSend/store' : '/roomMessageBatchSend/store';
  const createReachIDs = positiveReachIDs(createEmployeeIDs);
  const createContactTargetValues = positiveContactTargetKeys(createContactTargets).map(contactTargetFromKey).filter((value): value is { employeeId: number; contactId: number } => value !== null);
  const createRoomTargetValues = positiveRoomTargetKeys(createRoomTargets).map(roomTargetFromKey).filter((value): value is { ownerEmployeeId: number; roomId: number } => value !== null);
  const canSubmitCreate = Boolean(createTitle.trim() && createContent.trim() && createReachIDs.length > 0 && (mode === 'contact' ? createContactTargetValues.length > 0 : createRoomTargetValues.length > 0) && (createSendWay !== '2' || createDefiniteTime.trim()));
  const confirmationSummary = `${createReachIDs.length} 名员工，${mode === 'contact' ? createContactTargetValues.length : createRoomTargetValues.length} 个目标，${createSendWay === '2' ? `定时 ${createDefiniteTime}` : '立即发送'}。首次点击只打开确认，不会立即外发。`;
  const saveCreate = async () => {
    const ids = positiveReachIDs(createEmployeeIDs);
    const contactTargets = positiveContactTargetKeys(createContactTargets).map(contactTargetFromKey).filter((value): value is { employeeId: number; contactId: number } => value !== null);
    const roomTargets = positiveRoomTargetKeys(createRoomTargets).map(roomTargetFromKey).filter((value): value is { ownerEmployeeId: number; roomId: number } => value !== null);
    if (!createTitle.trim() || !createContent.trim() || ids.length === 0 || (mode === 'contact' ? contactTargets.length === 0 : roomTargets.length === 0)) {
      setWriteError('请填写任务名称、目标成员或群主 ID 和群发内容。');
      return;
    }
    const body: Record<string, unknown> = { batchTitle: createTitle.trim(), employeeIds: ids, sendWay: Number(createSendWay), content: [{ msgType: 'text', content: createContent.trim() }] };
    if (createMaterialID > 0) body.mediumId = createMaterialID;
    if (mode === 'contact') {
      body.filterParams = {};
      body.contactTargets = contactTargets;
      body.idempotencyKey = createIdempotencyKey || newContactBatchIdempotencyKey();
    } else {
      body.roomTargets = roomTargets;
      body.idempotencyKey = createIdempotencyKey || newContactBatchIdempotencyKey();
    }
    if (createSendWay === '2') body.definiteTime = createDefiniteTime.replace('T', ' ') + ':00';
    setSaving(true);
    setWriteError('');
    try {
      await api.write(createEndpoint, body, 'POST');
      setCreateOpen(false);
      setCreateTitle('');
      setCreateEmployeeIDs('');
      setCreateContactTargets('');
      setCreateRoomTargets('');
      setCreateContent('');
      setCreateMaterialID(0);
      setCreateSendWay('1');
      setCreateDefiniteTime('');
      setCreateIdempotencyKey('');
      await query.refetch();
    } catch (error) {
      setWriteError(businessErrorMessage(error, '群发任务创建失败，请检查企业微信配置和发送范围。'));
    } finally {
      setSaving(false);
    }
  };

  return (
    <section className="phase34-page">
      <header className="phase34-page-header">
        <div><p className="phase34-eyebrow">营销工具 · 内容触达</p><h1>精准群发</h1><p>分别查看客户群发与群聊群发任务，追踪内容、执行结果和触达数据。</p></div>
        <div className="phase34-header-actions"><span className="phase34-provider-badge phase34-provider-badge-warning">任务功能已就绪 · 外部发送待配置</span><button type="button" onClick={() => { setWriteError(''); setCreateIdempotencyKey(mode === 'contact' ? newContactBatchIdempotencyKey() : ''); setCreateOpen(true); }}>新建群发</button><button type="button" disabled={query.isFetching} onClick={refresh}>刷新</button></div>
      </header>
      <ReachTabs active={mode} onChange={(nextMode) => { setMode(nextMode); setDraftTitle(''); setTitle(''); setSelected(null); }} />
      <div id="precise-send-panel" role="tabpanel">
      <div className="dashboard-filter-bar phase34-filter-bar">
        <label>任务名称<input aria-label="任务名称" placeholder="请输入任务名称" value={draftTitle} onChange={(event) => setDraftTitle(event.target.value)} /></label>
        <label>创建开始日期<input aria-label="创建开始日期" type="date" /></label>
        <label>创建结束日期<input aria-label="创建结束日期" type="date" /></label>
        <div className="dashboard-table-actions"><button type="button" onClick={() => { setTitle(draftTitle.trim()); setSelected(null); }}>查询</button><button type="button" className="phase34-secondary-button" onClick={() => { setDraftTitle(''); setTitle(''); setSelected(null); }}>重置</button></div>
      </div>
      <div className="dashboard-data-card phase34-results-card">
        <div className="dashboard-card-heading"><div><h2>{mode === 'contact' ? '客户群发' : '群聊群发'}任务</h2><p>当前企业：{access.corp.name}，列表展示系统中已创建的真实任务。</p></div><span>{rows.length} 条</span></div>
        {writeError && !createOpen && <p role="alert" className="phase34-inline-error">{writeError}</p>}
        {query.isPending ? <PageState state="loading" /> : query.isError ? <PageState state={pageStateForError(query.error)} onRetry={refresh} /> : rows.length === 0 ? <PageState state="empty" title="暂无群发任务" description="当前筛选条件下没有可展示的任务。" /> : (
          <div className="dashboard-table-scroll"><table className="phase34-table phase34-reach-table"><thead><tr><th>创建时间</th><th>执行时间</th><th>发送内容</th><th>执行结果</th><th>执行数据</th><th>操作</th></tr></thead><tbody>{rows.map((row, index) => <tr key={recordKey(row, index)}><td>{primitive(row.createdAt)}</td><td>{primitive(row.sendTime ?? row.definiteTime)}</td><td>{contentText(row.content)}</td><td>{statusText(operationStatus(row))}</td><td>{executionData(row)}</td><td><button type="button" className="phase34-link-button" onClick={() => setSelected(row)}>详情</button></td></tr>)}</tbody></table></div>
        )}
      </div>
      </div>
      {selected !== null && <SendDetail api={api} mode={mode} row={selected} onClose={() => setSelected(null)} onChanged={refresh} />}
      {createOpen && <SendCreateDrawer api={api} mode={mode} title={createTitle} employeeIDs={createEmployeeIDs} contactTargets={createContactTargets} roomTargets={createRoomTargets} content={createContent} materialID={createMaterialID} sendWay={createSendWay} definiteTime={createDefiniteTime} saving={saving} error={writeError} canSubmit={canSubmitCreate} confirmationSummary={confirmationSummary} onTitleChange={setCreateTitle} onEmployeeIDsChange={(value) => { setCreateEmployeeIDs(value); setCreateContactTargets(''); setCreateRoomTargets(''); }} onContactTargetsChange={setCreateContactTargets} onRoomTargetsChange={setCreateRoomTargets} onContentChange={setCreateContent} onMaterialChange={setCreateMaterialID} onSendWayChange={setCreateSendWay} onDefiniteTimeChange={setCreateDefiniteTime} onClose={() => { if (!saving) setCreateOpen(false); }} onSubmit={() => { void saveCreate(); }} />}
    </section>
  );
}

type FriendsCircleTab = 'task' | 'material';

function friendsStatus(value: unknown): { label: string; tone: string } {
  const status = primitive(value);
  if (status === 'draft') return { label: '草稿', tone: 'draft' };
  if (status === 'available') return { label: '可用', tone: 'success' };
  if (status === 'failed') return { label: '失败', tone: 'danger' };
  if (status === 'publishing' || status === 'queued') return { label: '处理中', tone: 'progress' };
  return { label: status, tone: 'neutral' };
}

function csvCell(value: unknown): string {
  const text = primitive(value);
  return /[",\r\n]/.test(text) ? `"${text.replaceAll('"', '""')}"` : text;
}

function downloadFriendsCircleResults(taskID: number, rows: ReachRecord[]): void {
  const columns = ['taskId', 'targetEmployeeId', 'status', 'failureCode', 'failureReason', 'occurredAt'];
  const csv = [columns, ...rows.map((row) => columns.map((column) => csvCell(row[column])))].map((row) => row.join(',')).join('\r\n');
  const url = URL.createObjectURL(new Blob([`\uFEFF${csv}`], { type: 'text/csv;charset=utf-8' }));
  const link = document.createElement('a');
  link.href = url;
  link.download = `friends-circle-task-${taskID}-results.csv`;
  link.click();
  URL.revokeObjectURL(url);
}

function FriendsCircleTaskProgress({ task, rows, loading, error, exporting, exportError, onExport, onClose }: {
  task: ReachRecord;
  rows: ReachRecord[];
  loading: boolean;
  error: unknown;
  exporting: boolean;
  exportError: string;
  onExport: () => void;
  onClose: () => void;
}) {
  return (
    <aside className="phase34-detail" aria-label="朋友圈任务进度">
      <div className="phase34-detail-backdrop" aria-hidden="true" onClick={onClose} />
      <div className="phase34-detail-panel">
        <div className="dashboard-card-heading"><div><p className="phase34-eyebrow">营销工具 · 朋友圈</p><h2>朋友圈任务进度</h2><p>{primitive(task.taskName)} · {primitive(task.status)} · {primitive(task.completedTotal)} / {primitive(task.targetTotal)}</p></div><button type="button" aria-label="关闭朋友圈任务进度" onClick={onClose}>关闭</button></div>
        <dl><div><dt>外部任务 ID</dt><dd>{primitive(task.externalTaskId)}</dd></div><div><dt>最近回调</dt><dd>{primitive(task.lastCallbackAt)}</dd></div><div><dt>失败原因</dt><dd>{primitive(task.failureReason)}</dd></div></dl>
        <div className="dashboard-card-heading"><div><h3>目标失败明细</h3><p>仅展示当前企业且与任务关联的任务回执结果。</p></div><button type="button" disabled={exporting || loading || rows.length === 0} onClick={onExport}>{exporting ? '导出中…' : '导出失败明细'}</button></div>
        {exportError && <p role="alert" className="phase34-inline-error">{exportError}</p>}
        {loading ? <PageState state="loading" /> : error ? <PageState state={pageStateForError(error)} /> : rows.length === 0 ? <PageState state="empty" title="暂无失败明细" description="当前任务还没有可导出的失败目标。" /> : <div className="dashboard-table-scroll"><table className="phase34-table"><thead><tr><th>目标员工</th><th>状态</th><th>失败码</th><th>失败原因</th><th>发生时间</th></tr></thead><tbody>{rows.map((row, index) => <tr key={recordKey(row, index)}><td>{primitive(row.targetEmployeeId)}</td><td>{primitive(row.status)}</td><td>{primitive(row.failureCode)}</td><td>{primitive(row.failureReason)}</td><td>{primitive(row.occurredAt)}</td></tr>)}</tbody></table></div>}
      </div>
    </aside>
  );
}

function FriendsCircleComposer({ api, tab, name, content, materialID, saving, error, onNameChange, onContentChange, onMaterialChange, onClose, onSave }: {
  api: BusinessWorkbenchApi;
  tab: FriendsCircleTab;
  name: string;
  content: string;
  materialID: number;
  saving: boolean;
  error: string;
  onNameChange: (value: string) => void;
  onContentChange: (value: string) => void;
  onMaterialChange: (value: number, preview?: string) => void;
  onClose: () => void;
  onSave: () => void;
}) {
  const isTask = tab === 'task';
  const panelRef = useRef<HTMLDivElement>(null);
  const nameRef = useRef<HTMLInputElement>(null);
  const closeRef = useRef(onClose);
  const savingRef = useRef(saving);
  closeRef.current = onClose;
  savingRef.current = saving;
  useEffect(() => {
    const opener = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    nameRef.current?.focus();
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape' && !savingRef.current) closeRef.current();
      if (event.key !== 'Tab' || !panelRef.current) return;
      const focusable = Array.from(panelRef.current.querySelectorAll<HTMLElement>('button:not([disabled]), input:not([disabled]), textarea:not([disabled]), select:not([disabled])'));
      if (focusable.length === 0) { event.preventDefault(); panelRef.current.focus(); return; }
      const first = focusable[0]!;
      const last = focusable[focusable.length - 1]!;
      if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus(); }
      if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
    };
    document.addEventListener('keydown', handleKeyDown);
    return () => { document.removeEventListener('keydown', handleKeyDown); opener?.focus(); };
  }, []);
  return (
    <aside className="phase34-detail phase34-friends-drawer" aria-label="朋友圈草稿" data-variant={tab}>
      <div className="phase34-detail-backdrop" onClick={onClose} />
      <div ref={panelRef} className="phase34-detail-panel" role="dialog" aria-modal="true" aria-labelledby="friends-composer-title" tabIndex={-1}>
        <header className="phase34-friends-drawer-header">
          <div className="phase34-friends-drawer-icon" aria-hidden="true">{isTask ? '圈' : '素'}</div>
          <div><p className="phase34-eyebrow">{isTask ? '朋友圈任务' : '朋友圈素材'}</p><h2 id="friends-composer-title">{isTask ? '添加朋友圈草稿' : '添加文字素材'}</h2><p>{isTask ? '保存后可继续完善目标员工与正式发布配置。' : '创建可复用的文字内容，后续可用于朋友圈任务。'}</p></div>
          <button type="button" className="phase34-friends-close" aria-label="关闭新增面板" disabled={saving} onClick={onClose}>×</button>
        </header>
        <div className="phase34-friends-drawer-body">
          <div className="phase34-friends-safety"><strong>仅保存草稿</strong><span>当前不会向企业微信发布任何内容。</span></div>
          <div className="phase34-compose-form">
            {isTask && <MaterialSelector api={api} scene="friends_circle" value={materialID || null} disabled={saving} onChange={(item) => onMaterialChange(item?.id ?? 0, item?.preview)} />}
            <label><span>{isTask ? '任务名称' : '素材名称'} <b>*</b></span><input ref={nameRef} aria-label="草稿名称" aria-describedby="friends-name-count" required disabled={saving} maxLength={80} placeholder={isTask ? '例如：秋季新品朋友圈' : '例如：新品上市文案'} value={name} onChange={(event) => onNameChange(event.target.value)} /><small id="friends-name-count">{name.length} / 80</small></label>
            <label><span>朋友圈内容 <b>*</b></span><textarea aria-label="草稿内容" aria-describedby="friends-content-count" required disabled={saving} maxLength={500} rows={9} placeholder="请输入准备发布的朋友圈文字内容…" value={content} onChange={(event) => onContentChange(event.target.value)} /><small id="friends-content-count">{content.length} / 500</small></label>
            {error && <p className="phase34-friends-form-error" role="alert">{error}</p>}
          </div>
        </div>
        <footer className="phase34-friends-drawer-footer"><span>带 * 为必填项</span><div><button type="button" className="phase34-secondary-button" disabled={saving} onClick={onClose}>取消</button><button type="button" aria-label="保存草稿" disabled={saving || !name.trim() || !content.trim()} onClick={onSave}>{saving ? '保存中…' : isTask ? '保存任务草稿' : '保存素材'}</button></div></footer>
      </div>
    </aside>
  );
}

export function FriendsCirclePage({ api }: { api: BusinessWorkbenchApi }) {
  const access = useDashboardAccess();
  const [tab, setTab] = useState<FriendsCircleTab>('task');
  const [draftFilter, setDraftFilter] = useState('');
  const [filter, setFilter] = useState('');
  const [composerOpen, setComposerOpen] = useState(false);
  const [draftName, setDraftName] = useState('');
  const [draftContent, setDraftContent] = useState('');
  const [draftMaterialID, setDraftMaterialID] = useState(0);
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState('');
  const [discardOpen, setDiscardOpen] = useState(false);
  const [pendingTab, setPendingTab] = useState<FriendsCircleTab | null>(null);
  const [publishError, setPublishError] = useState('');
  const [selectedTask, setSelectedTask] = useState<ReachRecord | null>(null);
  const [exporting, setExporting] = useState(false);
  const [exportError, setExportError] = useState('');
  const endpoint = tab === 'task' ? '/friendsCircle/taskIndex' : '/friendsCircle/materialIndex';
  const query = useQuery({
    queryKey: ['phase34-friends-circle', access.corp.id, tab, filter],
    queryFn: () => api.read(endpoint, { ...(filter ? (tab === 'task' ? { taskName: filter } : { keyword: filter }) : {}), page: 1, perPage: 20 }),
  });
  const resultQuery = useQuery({
    queryKey: ['phase34-friends-circle-results', access.corp.id, selectedTask?.id],
    queryFn: () => api.read('/friendsCircle/taskResultIndex', { taskId: Number(selectedTask?.id), status: 'failed', page: 1, perPage: 100 }),
    enabled: selectedTask !== null && Number(selectedTask.id) > 0,
  });
  const rows = useMemo(() => rowsFrom(query.data), [query.data]);
  const refresh = () => { void query.refetch(); };
  const applyFilter = () => {
    const next = draftFilter.trim();
    if (next === filter) void query.refetch(); else setFilter(next);
  };
  const openComposer = () => { setDraftName(''); setDraftContent(''); setDraftMaterialID(0); setSaveError(''); setComposerOpen(true); };
  const publishTask = async (taskID: number) => {
    setPublishError('');
    try {
      await api.write('/friendsCircle/publish', { taskId: taskID }, 'POST');
    } catch (error) {
      setPublishError(businessErrorMessage(error, '朋友圈发布失败，任务状态已保留。'));
    } finally {
      await query.refetch();
    }
  };
  const exportResults = async () => {
    if (selectedTask === null) return;
    const taskID = Number(selectedTask.id);
    setExporting(true);
    setExportError('');
    try {
      const payload = await api.read('/friendsCircle/exportData', { taskId: taskID, status: 'failed' });
      downloadFriendsCircleResults(taskID, rowsFrom(payload));
    } catch (error) {
      setExportError(businessErrorMessage(error, '失败明细导出失败。'));
    } finally {
      setExporting(false);
    }
  };
  const finishComposerClose = (nextTab: FriendsCircleTab | null = null) => {
    setComposerOpen(false);
    setSaveError('');
    setDraftName('');
    setDraftContent('');
    setDraftMaterialID(0);
    setDiscardOpen(false);
    setPendingTab(null);
    if (nextTab !== null) {
      setTab(nextTab);
      setDraftFilter('');
      setFilter('');
    }
  };
  const closeComposer = () => {
    if (saving) return;
    if (draftName.trim() || draftContent.trim()) {
      setPendingTab(null);
      setDiscardOpen(true);
      return;
    }
    finishComposerClose();
  };
  const changeTab = (next: FriendsCircleTab) => {
    if (composerOpen && (draftName.trim() || draftContent.trim())) {
      setPendingTab(next);
      setDiscardOpen(true);
      return;
    }
    if (composerOpen) finishComposerClose();
    setTab(next);
    setDraftFilter('');
    setFilter('');
  };
  const saveDraft = async () => {
    if (!draftName.trim() || !draftContent.trim()) return;
    setSaving(true);
    setSaveError('');
    try {
      if (tab === 'task') {
        await api.write('/friendsCircle/taskStore', { taskName: draftName.trim(), content: draftContent.trim(), sendWay: 'manual', ...(draftMaterialID > 0 ? { mediumId: draftMaterialID } : {}) });
      } else {
        await api.write('/friendsCircle/materialStore', { name: draftName.trim(), type: 'text', content: { text: draftContent.trim() } });
      }
      setComposerOpen(false);
      setDraftName('');
      setDraftContent('');
      setDraftMaterialID(0);
      await query.refetch();
    } catch {
      setSaveError('草稿保存失败，请稍后重试。');
    } finally {
      setSaving(false);
    }
  };
  return (
    <section className="phase34-page phase34-friends-page">
      {publishError && <p role="alert" className="phase34-inline-error">{publishError}</p>}
      <header className="phase34-page-header phase34-friends-header"><div><p className="phase34-eyebrow">营销工具 · 内容触达</p><h1>朋友圈</h1><p>统一管理朋友圈任务与内容素材，先沉淀草稿，再安全接入发布流程。</p></div><div className="phase34-header-actions"><span className="phase34-provider-badge"><i />草稿服务正常</span><button type="button" aria-label="添加朋友圈" onClick={openComposer}>＋ 添加朋友圈</button><button type="button" disabled>导出</button><button type="button" className="phase34-secondary-button" disabled={query.isFetching} onClick={refresh}>{query.isFetching ? '刷新中…' : '刷新'}</button></div></header>
      <div className="phase34-friends-provider-notice"><span aria-hidden="true">!</span><div><strong>发布功能未配置</strong><p>当前支持任务与素材草稿管理，不会向企业微信实际发布。</p></div></div>
      <div className="phase34-tabs" role="tablist" aria-label="朋友圈内容类型"><button type="button" role="tab" aria-controls="friends-circle-panel" aria-selected={tab === 'task'} className={tab === 'task' ? 'phase34-tab-active' : ''} onClick={() => changeTab('task')}>朋友圈</button><button type="button" role="tab" aria-controls="friends-circle-panel" aria-selected={tab === 'material'} className={tab === 'material' ? 'phase34-tab-active' : ''} onClick={() => changeTab('material')}>朋友圈素材</button></div>
      <div id="friends-circle-panel" role="tabpanel">
      <div className="dashboard-filter-bar phase34-filter-bar phase34-friends-filter">
        <label>{tab === 'task' ? '任务名称' : '素材关键字'}<input aria-label={tab === 'task' ? '任务名称' : '素材关键字'} placeholder={tab === 'task' ? '搜索任务名称' : '搜索素材名称或内容'} value={draftFilter} onChange={(event) => setDraftFilter(event.target.value)} onKeyDown={(event) => { if (event.key === 'Enter') applyFilter(); }} /></label><div className="dashboard-table-actions"><button type="button" onClick={applyFilter}>查询</button><button type="button" className="phase34-secondary-button" disabled={!draftFilter && !filter} onClick={() => { setDraftFilter(''); setFilter(''); }}>重置</button></div>
      </div>
      <div className="dashboard-data-card phase34-results-card phase34-friends-results"><div className="dashboard-card-heading"><div><h2>{tab === 'task' ? '朋友圈任务' : '朋友圈素材'}</h2><p>{tab === 'task' ? '管理待完善、待发布的朋友圈任务草稿。' : '管理可在朋友圈任务中复用的内容素材。'}</p></div><span>{rows.length} 条记录</span></div>{query.isPending ? <PageState state="loading" /> : query.isError ? <PageState state={pageStateForError(query.error)} onRetry={refresh} /> : rows.length === 0 ? <div className="phase34-friends-empty"><div aria-hidden="true">◎</div><h3>{tab === 'task' ? '还没有朋友圈任务' : '还没有朋友圈素材'}</h3><p>{filter ? '没有找到符合当前关键字的记录，请调整后重试。' : '创建第一条草稿，开始沉淀朋友圈内容。'}</p>{!filter && <button type="button" onClick={openComposer}>立即创建</button>}</div> : <div className="dashboard-table-scroll"><table className="phase34-table phase34-friends-table"><thead><tr><th>{tab === 'task' ? '任务名称' : '素材名称'}</th><th>{tab === 'task' ? '发送方式' : '内容摘要'}</th><th>状态</th><th>{tab === 'task' ? '完成情况' : '素材类型'}</th><th>创建人 / 创建时间</th><th>操作</th></tr></thead><tbody>{rows.map((row, index) => { const status = friendsStatus(row.status); const taskID = Number(row.id); const hasTaskID = Number.isInteger(taskID) && taskID > 0; return <tr key={recordKey(row, index)}><td><strong>{primitive(tab === 'task' ? row.taskName : row.name)}</strong></td><td className="phase34-friends-summary">{tab === 'task' ? (row.sendWay === 'manual' ? '员工手动发送' : primitive(row.sendWay)) : contentText(row.content)}</td><td><span className={`phase34-friends-status phase34-friends-status-${status.tone}`}>{status.label}</span></td><td>{tab === 'task' ? `${primitive(row.completedTotal)} / ${primitive(row.targetTotal)}` : primitive(row.type)}</td><td><span>{primitive(row.creatorName)}</span><small>{primitive(row.createdAt)}</small></td><td className="phase34-friends-actions-cell">{tab === 'task' && hasTaskID ? <><button type="button" className="phase34-link-button" onClick={() => { setSelectedTask(row); setExportError(''); }}>查看进度</button>{row.status === 'draft' && <button type="button" className="phase34-link-button" onClick={() => void publishTask(taskID)}>发起发布</button>}</> : <span className="phase34-muted-action">—</span>}</td></tr>; })}</tbody></table></div>}</div>
      </div>
      {composerOpen && <FriendsCircleComposer api={api} tab={tab} name={draftName} content={draftContent} materialID={draftMaterialID} saving={saving} error={saveError} onNameChange={setDraftName} onContentChange={setDraftContent} onMaterialChange={(id, preview) => { setDraftMaterialID(id); if (preview) setDraftContent(preview); }} onClose={closeComposer} onSave={() => void saveDraft()} />}
      <DashboardDialog
        danger
        open={discardOpen}
        title="放弃未保存内容？"
        confirmText="放弃更改"
        onCancel={() => { setDiscardOpen(false); setPendingTab(null); }}
        onConfirm={() => finishComposerClose(pendingTab)}
      >
        <p>当前草稿尚未保存，放弃后无法恢复。</p>
      </DashboardDialog>
      {selectedTask !== null && <FriendsCircleTaskProgress task={selectedTask} rows={rowsFrom(resultQuery.data)} loading={resultQuery.isPending} error={resultQuery.error} exporting={exporting} exportError={exportError} onExport={() => void exportResults()} onClose={() => setSelectedTask(null)} />}
    </section>
  );
}
