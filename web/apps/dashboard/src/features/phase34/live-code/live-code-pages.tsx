import { useQuery } from '@tanstack/react-query';
import { useEffect, useMemo, useState } from 'react';

import { useDashboardAccess } from '../../../app/access-context';
import { DashboardPagination } from '../../../components/dashboard-pagination';
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
  const state = textOf(value(row, 'state', 'lifecycleState'), '').toLowerCase();
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

type LiveCodeEmployeeOption = {
  id: number;
  name: string;
};

type LiveCodeRoomOption = {
  id: number;
  name: string;
  currentNum: number;
  roomMax: number;
};

function toggleNumericChoice(values: number[], id: number): number[] {
  return values.includes(id) ? values.filter((value) => value !== id) : [...values, id];
}

function numericIDs(input: unknown, keys: string[] = []): number[] {
  const ids = new Set<number>();
  const collect = (candidate: unknown, active: boolean) => {
    if (Array.isArray(candidate)) {
      candidate.forEach((item) => collect(item, active));
      return;
    }
    if (typeof candidate === 'number' || typeof candidate === 'string') {
      const id = Number(candidate);
      if (active && Number.isInteger(id) && id > 0) ids.add(id);
      return;
    }
    if (!candidate || typeof candidate !== 'object') return;
    Object.entries(candidate as Record<string, unknown>).forEach(([key, value]) => {
      collect(value, active || keys.includes(key));
    });
  };
  collect(input, keys.length === 0);
  return [...ids];
}

function fieldIDs(input: unknown, keys: string[]): number[] {
  return Array.isArray(input) && input.every((item) => typeof item === 'number' || typeof item === 'string')
    ? numericIDs(input)
    : numericIDs(input, keys);
}

async function fetchAllLiveCodeEmployees(api: BusinessWorkbenchApi): Promise<LiveCodeEmployeeOption[]> {
  const perPage = 100;
  const options: LiveCodeEmployeeOption[] = [];
  const seen = new Set<number>();
  let page = 1;
  let totalPage = 1;
  do {
    const payload = await api.read('/workEmployee/index', { status: 1, page, perPage });
    const rows = recordsFrom(payload);
    for (const row of rows) {
      const id = numberOf(value(row, 'id', 'employeeId'));
      if (!Number.isInteger(id) || id <= 0 || seen.has(id)) continue;
      seen.add(id);
      options.push({ id, name: textOf(value(row, 'name', 'employeeName'), `成员 ${id}`) });
    }
    totalPage = paginationFrom(payload, rows.length).totalPage;
    if (rows.length === 0 || page >= totalPage || rows.length < perPage) break;
    page += 1;
  } while (page <= totalPage);
  return options;
}

async function fetchLiveCodeRooms(api: BusinessWorkbenchApi): Promise<LiveCodeRoomOption[]> {
  const rows = recordsFrom(await api.read('/workRoom/roomIndex', {}));
  const seen = new Set<number>();
  const rooms: LiveCodeRoomOption[] = [];
  for (const row of rows) {
    const id = numberOf(value(row, 'roomId', 'workRoomId', 'id'));
    if (!Number.isInteger(id) || id <= 0 || seen.has(id)) continue;
    seen.add(id);
    rooms.push({
      id,
      name: textOf(value(row, 'roomName', 'name'), `未命名群聊 ${id}`),
      currentNum: numberOf(value(row, 'currentNum', 'memberNum', 'memberCount')),
      roomMax: numberOf(value(row, 'roomMax')),
    });
  }
  return rooms;
}

function LiveCodeUpsertDialog({
  api,
  kind,
  open,
  editing,
  saving,
  error,
  onCancel,
  onSave,
}: {
  api: BusinessWorkbenchApi;
  kind: LiveCodeKind;
  open: boolean;
  editing: { row: LiveCodeRecord; detail: LiveCodeRecord } | null;
  saving: boolean;
  error: string;
  onCancel: () => void;
  onSave: (values: Record<string, unknown>) => void;
}) {
  const access = useDashboardAccess();
  const detail = editing?.detail;
  const baseInfo = detail?.baseInfo && typeof detail.baseInfo === 'object' ? detail.baseInfo as LiveCodeRecord : undefined;
  const [name, setName] = useState(() => textOf(value(baseInfo ?? detail ?? {}, kind === 'channel' ? 'name' : 'qrcodeName', 'name'), ''));
  const [employeeIDs, setEmployeeIDs] = useState<number[]>(() => fieldIDs(value(detail ?? {}, 'drainageEmployee', 'employees'), ['employeeId', 'employees']));
  const [employeeSearch, setEmployeeSearch] = useState('');
  const [employeeSelectionNotice, setEmployeeSelectionNotice] = useState('');
  const [leadingWords, setLeadingWords] = useState(() => textOf(value(detail ?? {}, 'leadingWords'), ''));
  const [tags, setTags] = useState(() => fieldIDs(value(detail ?? {}, 'tags'), ['tagId', 'id']).join(','));
  const [selectedRoomIDs, setSelectedRoomIDs] = useState<number[]>(() => numericIDs(value(detail ?? {}, 'rooms'), ['roomId', 'workRoomId', 'id']));
  const [roomDraftIDs, setRoomDraftIDs] = useState<number[]>([]);
  const [roomSearch, setRoomSearch] = useState('');
  const [roomPickerOpen, setRoomPickerOpen] = useState(false);
  const [roomSelectionNotice, setRoomSelectionNotice] = useState('');
  const [validationAttempted, setValidationAttempted] = useState(false);
  const employeesQuery = useQuery({
    queryKey: ['live-code-employees', access.corp.id],
    queryFn: () => fetchAllLiveCodeEmployees(api),
    enabled: open,
  });
  const roomsQuery = useQuery({
    queryKey: ['live-code-rooms', access.corp.id],
    queryFn: () => fetchLiveCodeRooms(api),
    enabled: open && kind === 'group',
  });
  useEffect(() => {
    if (!employeesQuery.isSuccess) return;
    const validIDs = new Set((employeesQuery.data ?? []).map((employee) => employee.id));
    const nextIDs = employeeIDs.filter((id) => validIDs.has(id));
    if (nextIDs.length === employeeIDs.length) return;
    setEmployeeIDs(nextIDs);
    setEmployeeSelectionNotice('部分已选成员已失效，请重新选择。');
  }, [employeeIDs, employeesQuery.data, employeesQuery.isSuccess]);
  useEffect(() => {
    if (kind !== 'group' || !roomsQuery.isSuccess) return;
    const validIDs = new Set((roomsQuery.data ?? []).filter((room) => Number.isInteger(room.roomMax) && room.roomMax > 0).map((room) => room.id));
    const nextSelectedIDs = selectedRoomIDs.filter((id) => validIDs.has(id));
    const nextDraftIDs = roomDraftIDs.filter((id) => validIDs.has(id));
    if (nextSelectedIDs.length !== selectedRoomIDs.length) {
      setSelectedRoomIDs(nextSelectedIDs);
      setRoomSelectionNotice('部分已选群聊已失效，请重新选择。');
    }
    if (nextDraftIDs.length !== roomDraftIDs.length) setRoomDraftIDs(nextDraftIDs);
  }, [kind, roomDraftIDs, roomsQuery.data, roomsQuery.isSuccess, selectedRoomIDs]);
  if (!open) return null;
  const employeeOptions = employeesQuery.data ?? [];
  const selectedEmployeeIDs = new Set(employeeIDs);
  const normalizedSearch = employeeSearch.trim().toLowerCase();
  const visibleEmployees = employeeOptions.filter((employee) => {
    if (selectedEmployeeIDs.has(employee.id)) return true;
    return normalizedSearch === '' || employee.name.toLowerCase().includes(normalizedSearch) || String(employee.id).includes(normalizedSearch);
  });
  const roomOptions = roomsQuery.data ?? [];
  const selectedRooms = selectedRoomIDs
    .map((id) => roomOptions.find((room) => room.id === id))
    .filter((room): room is LiveCodeRoomOption => room !== undefined);
  const normalizedRoomSearch = roomSearch.trim().toLowerCase();
  const visibleRooms = roomOptions.filter((room) => normalizedRoomSearch === '' || room.name.toLowerCase().includes(normalizedRoomSearch));
  const tagIDs = tags.split(',').map((item) => Number(item.trim())).filter((item) => Number.isInteger(item) && item > 0);
  const selectedEmployeesValid = employeesQuery.isSuccess && employeeIDs.length > 0 && employeeIDs.every((id) => employeeOptions.some((employee) => employee.id === id));
  const selectedRoomsValid = roomsQuery.isSuccess
    && selectedRoomIDs.length > 0
    && selectedRooms.length === selectedRoomIDs.length
    && selectedRooms.every((room) => Number.isInteger(room.roomMax) && room.roomMax > 0);
  const valid = name.trim() !== '' && selectedEmployeesValid && (kind === 'channel' || (leadingWords.trim() !== '' && tagIDs.length > 0 && selectedRoomsValid));
  const closeRoomPicker = () => {
    setRoomPickerOpen(false);
    setRoomSearch('');
  };
  const openRoomPicker = () => {
    setRoomDraftIDs([...selectedRoomIDs]);
    setRoomSearch('');
    setRoomPickerOpen(true);
  };
  const toggleRoomDraft = (room: LiveCodeRoomOption) => {
    if (!Number.isInteger(room.roomMax) || room.roomMax <= 0) return;
    setRoomDraftIDs((current) => {
      if (current.includes(room.id)) return current.filter((id) => id !== room.id);
      return current.length >= 5 ? current : [...current, room.id];
    });
  };
  const submit = () => {
    if (!valid) {
      setValidationAttempted(true);
      return;
    }
    setValidationAttempted(false);
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
      onSave({
        corpId: Number(access.corp.id),
        qrcodeName: name.trim(),
        isVerified: 2,
        leadingWords: leadingWords.trim(),
        employees: employeeIDs,
        tags: tagIDs,
        rooms: JSON.stringify(selectedRooms.map((room) => ({ roomId: room.id, maxNum: room.roomMax }))),
      });
    }
  };
  const title = kind === 'channel' ? '渠道活码' : '群活码';
  const mode = editing ? '编辑' : '新建';
  return (
    <aside className="phase34-detail phase34-live-code-drawer phase34-live-code-create-drawer" aria-label={`${mode}${title}`}>
      <div className="phase34-detail-backdrop" aria-hidden="true" onClick={onCancel} />
      <div className="phase34-detail-panel" role="dialog" aria-modal="true" aria-label={`${mode}${title}`}>
        <header className="phase34-live-code-drawer-header">
          <div className="phase34-live-code-drawer-title"><span className="phase34-live-code-drawer-icon" aria-hidden="true">{editing ? '编' : '+'}</span><div><p className="phase34-eyebrow">营销工具 · {mode}配置</p><h2>{mode}{title}</h2><p>填写必要信息后提交到当前企业微信 Provider。</p></div></div>
          <button type="button" className="phase34-live-code-close" aria-label={`关闭${mode}${title}`} title="关闭" onClick={onCancel}>×</button>
        </header>
        <div className="phase34-live-code-drawer-body">
          <form className="phase34-detail-form" onSubmit={(event) => { event.preventDefault(); submit(); }}>
            <div className="phase34-live-code-form-intro"><strong>先完成基础配置</strong><span>保存后会调用企业微信 Provider，未配置企业授信时不会生成虚假二维码。</span></div>
            <label>{title}名称 <b>*</b><input aria-label={`${title}名称`} value={name} maxLength={30} onChange={(event) => setName(event.target.value)} placeholder={`例如：${kind === 'channel' ? '官网咨询' : '售后服务群'}`} />{validationAttempted && name.trim() === '' && <span className="phase34-inline-error">请输入活码名称</span>}</label>
            <div className="phase34-live-code-member-field">
              <label htmlFor="live-code-employee-search">使用成员 <b>*</b></label>
              <input id="live-code-employee-search" aria-label="搜索使用成员" value={employeeSearch} onChange={(event) => setEmployeeSearch(event.target.value)} placeholder="搜索成员姓名" />
              <div className="phase34-live-code-choice-summary"><span>已选择 {employeeIDs.length} 人</span><small>单击选择，再次单击取消</small></div>
              <div className="phase34-live-code-choice-list" aria-label="使用成员选择">
                {visibleEmployees.map((employee) => {
                  const selected = selectedEmployeeIDs.has(employee.id);
                  return <button key={employee.id} type="button" aria-label={`${selected ? '取消选择' : '选择'}成员 ${employee.name}`} aria-pressed={selected} onClick={() => { setEmployeeSelectionNotice(''); setEmployeeIDs((current) => toggleNumericChoice(current, employee.id)); }}><span>{employee.name}</span><span aria-hidden="true">{selected ? '✓' : '+'}</span></button>;
                })}
              </div>
              {employeeSelectionNotice && <span role="alert" className="phase34-inline-error">{employeeSelectionNotice}</span>}
              {employeesQuery.isSuccess && employeeOptions.length > 0 && visibleEmployees.length === 0 && <span role="status" className="phase34-field-hint">没有匹配的成员。</span>}
              {employeesQuery.isPending && <span role="status" className="phase34-field-hint">正在加载可用成员…</span>}
              {employeesQuery.isError && <span role="alert" className="phase34-inline-error">成员列表加载失败，请重试后再提交。</span>}
              {employeesQuery.isSuccess && employeeOptions.length === 0 && <span role="status" className="phase34-field-hint">暂无当前权限范围内的在职成员。</span>}
            </div>
            {kind === 'group' && <>
              <label>入群引导语 <b>*</b><textarea aria-label="入群引导语" value={leadingWords} maxLength={1000} onChange={(event) => setLeadingWords(event.target.value)} placeholder="欢迎加入我们的服务群" /></label>
              <label>客户标签 ID <b>*</b><input aria-label="客户标签 ID" inputMode="numeric" value={tags} onChange={(event) => setTags(event.target.value)} placeholder="多个 ID 用逗号分隔" /></label>
              <div className="phase34-live-code-room-field" role="group" aria-label="可加入的群聊">
                <div className="phase34-live-code-room-field-heading"><div><strong>可加入的群聊 <b>*</b></strong><small>按选择顺序使用，最多选择 5 个群聊。</small></div><button type="button" className="phase34-live-code-select-trigger" onClick={openRoomPicker}>选择群聊</button></div>
                {selectedRooms.length === 0 ? <div className="phase34-live-code-room-empty">尚未选择群聊</div> : <div className="phase34-live-code-selected-rooms"><span>已选择 {selectedRooms.length} 个群聊</span>{selectedRooms.map((room, index) => <div key={room.id}><em>{index + 1}</em><span><strong>{room.name}</strong><small>当前 {room.currentNum} 人 · 容量 {room.roomMax} 人</small></span><button type="button" aria-label={`移除群聊 ${room.name}`} onClick={() => { setRoomSelectionNotice(''); setSelectedRoomIDs((current) => current.filter((id) => id !== room.id)); }}>×</button></div>)}</div>}
                {roomSelectionNotice && <span role="alert" className="phase34-inline-error">{roomSelectionNotice}</span>}
                {roomsQuery.isPending && <span role="status" className="phase34-field-hint">正在加载可用群聊…</span>}
                {roomsQuery.isError && <span role="alert" className="phase34-inline-error">群聊列表加载失败，请重试后再提交。</span>}
                {roomsQuery.isSuccess && roomOptions.length === 0 && <span role="status" className="phase34-field-hint">暂无当前权限范围内的群聊，请先完成群聊同步。</span>}
              </div>
            </>}
            {validationAttempted && !valid && <p className="phase34-inline-error" role="alert">请检查填写是否合规，并补全所有必填项。</p>}
            {error && <p className="phase34-inline-error" role="alert">{error}</p>}
          </form>
        </div>
        <footer className="phase34-live-code-drawer-footer"><span>带 * 为必填项</span><div><button type="button" className="phase34-secondary-button" disabled={saving} onClick={onCancel}>取消</button><button type="button" disabled={saving} onClick={submit}>{saving ? '保存中…' : `保存${title}`}</button></div></footer>
      </div>
      {roomPickerOpen && <div className="phase34-live-code-picker-layer">
        <div className="phase34-live-code-picker-backdrop" aria-hidden="true" onClick={closeRoomPicker} />
        <section className="phase34-live-code-picker" role="dialog" aria-label="选择群聊" aria-modal="true">
          <header><div><h3>选择群聊</h3><p>按顺序选择，最多 5 个群聊</p></div><button type="button" aria-label="关闭群聊选择" onClick={closeRoomPicker}>×</button></header>
          <div className="phase34-live-code-picker-grid">
            <section aria-label="可选群聊"><label>搜索群聊<input aria-label="搜索群聊" value={roomSearch} onChange={(event) => setRoomSearch(event.target.value)} placeholder="输入群聊名称" /></label><div className="phase34-live-code-room-options">{visibleRooms.map((room) => {
              const selected = roomDraftIDs.includes(room.id);
              const capacityUnavailable = !Number.isInteger(room.roomMax) || room.roomMax <= 0;
              const limitReached = roomDraftIDs.length >= 5 && !selected;
              return <button key={room.id} type="button" aria-label={`${selected ? '取消选择' : '选择'}群聊 ${room.name}`} aria-pressed={selected} disabled={capacityUnavailable || limitReached} onClick={() => toggleRoomDraft(room)}><span className="phase34-live-code-room-mark" aria-hidden="true">{selected ? '✓' : ''}</span><span><strong>{room.name}</strong><small>{capacityUnavailable ? '容量未同步，暂不可选' : `当前 ${room.currentNum} 人 · 容量 ${room.roomMax} 人`}</small></span></button>;
            })}{roomsQuery.isSuccess && visibleRooms.length === 0 && <div className="phase34-live-code-picker-empty">没有匹配的群聊</div>}</div></section>
            <section aria-label="已选群聊"><div className="phase34-live-code-picker-selected-heading"><strong>已选择 {roomDraftIDs.length} 个群聊</strong>{roomDraftIDs.length > 0 && <button type="button" onClick={() => setRoomDraftIDs([])}>清空选择</button>}</div><p className="phase34-live-code-picker-limit">最多选择 5 个群聊</p><div className="phase34-live-code-picker-selected-list">{roomDraftIDs.length === 0 ? <div className="phase34-live-code-picker-empty">请选择群聊</div> : roomDraftIDs.map((id, index) => {
              const room = roomOptions.find((option) => option.id === id);
              if (!room) return null;
              return <div key={room.id}><em>{index + 1}</em><span><strong>{room.name}</strong><small>当前 {room.currentNum} 人 · 容量 {room.roomMax} 人</small></span><button type="button" aria-label={`移除群聊 ${room.name}`} onClick={() => setRoomDraftIDs((current) => current.filter((value) => value !== room.id))}>×</button></div>;
            })}</div></section>
          </div>
          <footer><button type="button" className="phase34-secondary-button" onClick={closeRoomPicker}>取消</button><button type="button" disabled={roomsQuery.isPending || roomsQuery.isError} onClick={() => { setRoomSelectionNotice(''); setSelectedRoomIDs([...roomDraftIDs]); closeRoomPicker(); }}>确认选择</button></footer>
        </section>
      </div>}
    </aside>
  );
}

function DetailDrawer({ kind, row, onClose }: { kind: LiveCodeKind; row: LiveCodeRecord; onClose: () => void }) {
  const isChannel = kind === 'channel';
  const name = textOf(value(row, 'name', 'qrcodeName'), '未命名活码');
  const qrURL = imageURL(row);
  const statisticsUnavailable = row.statisticsAvailable === false;
  const detail = (fallback: string, ...keys: string[]) => textOf(value(row, ...keys), fallback);
  return (
    <aside className="live-code-detail" aria-label={`${kind === 'channel' ? '渠道活码' : '群活码'}详情`}>
      <div className="phase34-detail-backdrop" data-phase34-detail-backdrop aria-hidden="true" onClick={onClose} />
      <div className="phase34-detail-panel" role="dialog" aria-modal="true">
        <header className="phase34-live-code-drawer-header">
          <div className="phase34-live-code-drawer-title"><span className="phase34-live-code-drawer-icon" aria-hidden="true">码</span><div><p className="phase34-eyebrow">营销工具 · {isChannel ? '渠道获客' : '社群获客'}</p><h2>{name}</h2><p>{isChannel ? '查看二维码、使用成员和新增好友数据。' : '查看二维码、入群引导和关联群聊配置。'}</p></div></div>
          <button type="button" className="phase34-live-code-close" aria-label="关闭详情" title="关闭详情" onClick={onClose}>×</button>
        </header>
        <div className="phase34-live-code-drawer-body">
          <section className="phase34-live-code-summary" aria-label="二维码与状态">
            <div className="phase34-live-code-qr">{qrURL ? <img src={qrURL} alt="二维码" /> : <span><strong>二维码</strong><small>未提供</small></span>}</div>
            <div><span className={`phase34-live-code-status ${qrURL ? 'is-ready' : 'is-pending'}`}>{qrURL ? '已接入二维码' : '待配置二维码'}</span><strong>{name}</strong><small>{isChannel ? statisticsUnavailable ? '暂无可验证统计' : `新增好友 ${detail('0', 'addedFriendCount', 'contactNum')}` : `关联群聊 ${detail('0', 'roomNum', 'rooms')}`}</small></div>
          </section>
          <section className="phase34-live-code-section"><h3>基础信息</h3><dl>
            <div><dt>创建时间</dt><dd>{detail('—', 'createdAt', 'created_at')}</dd></div>
            <div><dt>{isChannel ? '创建人' : '入群引导语'}</dt><dd>{isChannel ? detail('—', 'creator', 'creatorName') : detail('—', 'leadingWords')}</dd></div>
            <div><dt>使用成员</dt><dd>{employeesText(row)}</dd></div>
            <div><dt>{isChannel ? '有效期' : '状态'}</dt><dd>{isChannel ? validityText(row) : stateText(value(row, 'state', 'status', 'isVerified'))}</dd></div>
          </dl></section>
          <section className="phase34-live-code-section"><h3>{isChannel ? '归因与效果' : '配置与关系'}</h3><dl>
            {isChannel ? <>
              <div><dt>客户标签</dt><dd>{detail('—', 'tags')}</dd></div>
              <div><dt>新增好友数</dt><dd>{statisticsUnavailable ? '暂无可验证数据' : detail('0', 'addedFriendCount', 'contactNum')}</dd></div>
              <div><dt>统计口径</dt><dd>{statisticsUnavailable ? '暂无可验证数据' : '已接入真实关联记录'}</dd></div>
            </> : <div><dt>关联群聊</dt><dd>{detail('暂无关联群聊', 'rooms')}</dd></div>}
          </dl></section>
        </div>
        <footer className="phase34-live-code-drawer-footer"><span>信息来自当前企业权限范围</span><button type="button" className="phase34-secondary-button" onClick={onClose}>返回列表</button></footer>
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
  const [editing, setEditing] = useState<{ row: LiveCodeRecord; detail: LiveCodeRecord } | null>(null);
  const [editLoadingID, setEditLoadingID] = useState('');
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
  const openEdit = async (row: LiveCodeRecord) => {
    const id = numberOf(value(row, isChannel ? 'channelCodeId' : 'workRoomAutoPullId', 'id'));
    if (id <= 0) return;
    setWriteError('');
    setEditLoadingID(String(id));
    try {
      const payload = await api.read(isChannel ? '/channelCode/show' : '/workRoomAutoPull/show', isChannel ? { channelCodeId: id } : { workRoomAutoPullId: id });
      setEditing({ row, detail: payload && typeof payload === 'object' ? payload as LiveCodeRecord : row });
    } catch (error) {
      setWriteError(error instanceof Error ? error.message : `加载${title}失败`);
    } finally {
      setEditLoadingID('');
    }
  };
  const save = async (values: Record<string, unknown>) => {
    setSaving(true); setWriteError('');
    try {
      const editID = editing ? numberOf(value(editing.row, isChannel ? 'channelCodeId' : 'workRoomAutoPullId', 'id')) : 0;
      const payload = editID > 0 ? { [isChannel ? 'channelCodeId' : 'workRoomAutoPullId']: editID, ...values } : values;
      await api.write(
        isChannel ? (editID > 0 ? '/channelCode/update' : '/channelCode/store') : (editID > 0 ? '/workRoomAutoPull/update' : '/workRoomAutoPull/store'),
        payload,
        editID > 0 ? 'PUT' : 'POST',
      );
      setCreateOpen(false);
      setEditing(null);
      await query.refetch();
    } catch (error) { setWriteError(error instanceof Error ? error.message : `${editing ? '更新' : '创建'}${title}失败`); } finally { setSaving(false); }
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
            <div className="live-code-table-card"><div className="live-code-table-heading"><div><h2>{isChannel ? '渠道活码列表' : '群活码列表'}</h2><p>{isChannel ? '创建人、使用成员、有效期和效果指标均来自已接入的业务数据。' : '扫码二维码、查看关联群聊并确认群活码配置状态。'}</p></div><span>{pagination.total} 条记录</span></div>{query.isPending ? <PageState state="loading" /> : query.isError ? <PageState state={pageStateForError(query.error)} onRetry={() => { void query.refetch(); }} /> : rows.length === 0 ? <PageState state="empty" title="暂无记录" description="调整筛选条件或创建一条新的配置。" /> : <div className="live-code-table-scroll"><table className="live-code-table"><thead><tr><th>二维码</th><th>{isChannel ? '活码名称' : '群活码名称'}</th><th>{isChannel ? '创建人 / 创建时间' : '入群引导语'}</th><th>{isChannel ? '使用成员' : '关联群聊'}</th><th>{isChannel ? '有效期 / 新增好友' : '状态 / 创建时间'}</th><th>操作</th></tr></thead><tbody>{rows.map((row, index) => { const id = rowID(row, index); return <tr key={id}><td><QRPreview row={row} /></td><td><strong>{textOf(value(row, nameKey, 'name'), '未命名活码')}</strong><small>{textOf(value(row, 'groupName'), '未分组')}</small></td><td>{isChannel ? <><span>{textOf(value(row, 'creator', 'creatorName'))}</span><small>{textOf(value(row, 'createdAt', 'created_at'))}</small></> : <span className="live-code-clamp">{textOf(value(row, 'leadingWords'))}</span>}</td><td>{isChannel ? <span className="live-code-clamp">{employeesText(row)}</span> : <span>{textOf(value(row, 'rooms'), '—')}</span>}</td><td>{isChannel ? <><span>{validityText(row)}</span><small>{value(row, 'statisticsAvailable') === false ? '暂无统计' : `新增好友 ${textOf(value(row, 'addedFriendCount', 'contactNum'), '0')}`}</small></> : <><StatusPill state={isVerified(row) ? 'active' : 'draft'} /><small>{textOf(value(row, 'createdAt', 'created_at'))}</small></>}</td><td><button type="button" className="live-code-link" onClick={() => setSelected(row)}>详情</button>{can('edit') && <button type="button" className="live-code-link" disabled={editLoadingID === id} onClick={() => { void openEdit(row); }}>{editLoadingID === id ? '加载中…' : '编辑'}</button>}</td></tr>; })}</tbody></table></div>}<div className="live-code-pagination">{pagination.total > 0 && <DashboardPagination page={pageNumber} pageSize={pagination.perPage} total={pagination.total} onPageChange={setPageNumber} ariaLabel={`${title}分页`} />}</div></div>
          </>}
        </main>
      </div>
      {selected && <DetailDrawer kind={kind} row={selected} onClose={() => setSelected(null)} />}
      {(createOpen || editing) && <LiveCodeUpsertDialog api={api} kind={kind} open editing={editing} saving={saving} error={writeError} onCancel={() => { if (!saving) { setCreateOpen(false); setEditing(null); setWriteError(''); } }} onSave={(values) => { void save(values); }} />}
    </section>
  );
}

export function ChannelCodePage({ api }: { api: BusinessWorkbenchApi }) { return <LiveCodeWorkspacePage api={api} kind="channel" />; }

export function GroupCodePage({ api }: { api: BusinessWorkbenchApi }) { return <LiveCodeWorkspacePage api={api} kind="group" />; }
