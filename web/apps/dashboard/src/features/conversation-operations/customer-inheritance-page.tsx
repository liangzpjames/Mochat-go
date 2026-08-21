import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useMemo, useState } from 'react';
import { useSearchParams } from 'react-router';

import { useDashboardAccess } from '../../app/access-context';
import { DashboardDialog } from '../../components/dashboard-dialog';
import { DashboardPagination } from '../../components/dashboard-pagination';
import { ConversationOperationsShell, ConversationQueryBar, ConversationTabs } from './conversation-operations-shell';
import { formatDateTime } from './conversation-operations-format';
import type {
  ContactTransferApi,
  ContactTransferFilter,
  EmployeeOption,
  TransferCustomer,
  TransferLog,
  TransferRoom,
} from './contact-transfer-api';

const pageSize = 20;
type InheritanceTab = 'resigned' | 'active';
type AssetTab = 'customer' | 'room';
const recordModes = [
  { value: 1 as const, label: '离职客户' },
  { value: 2 as const, label: '离职群聊' },
  { value: 3 as const, label: '在职客户' },
];

function positivePage(value: string | null) {
  const page = Number(value);
  return Number.isInteger(page) && page > 0 ? page : 1;
}

function stringValue(value: unknown) {
  return typeof value === 'string' ? value : value == null ? '' : String(value);
}

function employeeId(value: string | null) {
  return value && /^\d+$/.test(value) ? value : '';
}

function selectedCustomerRows(rows: readonly TransferCustomer[], selected: ReadonlySet<string>) {
  return rows.filter((row) => selected.has(String(row.contactId)));
}

function selectedRoomRows(rows: readonly TransferRoom[], selected: ReadonlySet<string>) {
  return rows.filter((row) => selected.has(row.chatId));
}

function employeeLabel(employee: EmployeeOption | undefined) {
  return employee?.name || employee?.wxUserId || '--';
}

export function CustomerInheritancePage({ api }: { api: ContactTransferApi }) {
  const access = useDashboardAccess();
  const queryClient = useQueryClient();
  const [searchParams, setSearchParams] = useSearchParams();
  const rawInheritanceTab = searchParams.get('inheritanceTab');
  const inheritanceTab: InheritanceTab = rawInheritanceTab === 'active' ? 'active' : 'resigned';
  const rawAssetTab = searchParams.get('assetTab');
  const assetTab: AssetTab = inheritanceTab === 'resigned' && rawAssetTab === 'room' ? 'room' : 'customer';
  const page = positivePage(searchParams.get('page'));
  const sourceEmployeeId = employeeId(searchParams.get('sourceEmployeeId'));
  const contactName = searchParams.get('contactName') ?? '';
  const addTimeStart = searchParams.get('addTimeStart') ?? '';
  const addTimeEnd = searchParams.get('addTimeEnd') ?? '';
  const [draft, setDraft] = useState({ contactName, addTimeStart, addTimeEnd });
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [transferredKeys, setTransferredKeys] = useState<Set<string>>(new Set());
  const [successorId, setSuccessorId] = useState('');
  const [syncOpen, setSyncOpen] = useState(false);
  const [transferOpen, setTransferOpen] = useState(false);
  const [recordsOpen, setRecordsOpen] = useState(false);
  const [recordMode, setRecordMode] = useState<1 | 2 | 3>(1);
  const [recordPage, setRecordPage] = useState(1);
  const [recordFilter, setRecordFilter] = useState({ name: '', employee: '', from: '', to: '' });
  const [statusMessage, setStatusMessage] = useState('');

  useEffect(() => {
    setDraft({ contactName, addTimeStart, addTimeEnd });
  }, [contactName, addTimeStart, addTimeEnd]);

  useEffect(() => {
    const invalidAssetTab = rawAssetTab !== null && rawAssetTab !== 'customer' && rawAssetTab !== 'room';
    if ((rawInheritanceTab !== null && rawInheritanceTab !== 'resigned' && rawInheritanceTab !== 'active') || invalidAssetTab || (inheritanceTab === 'active' && rawAssetTab === 'room')) {
      const next = new URLSearchParams(searchParams);
      next.set('inheritanceTab', inheritanceTab);
      next.set('assetTab', 'customer');
      next.set('page', '1');
      setSearchParams(next);
    }
  }, [inheritanceTab, rawAssetTab, rawInheritanceTab, searchParams, setSearchParams]);

  const employeesQuery = useQuery({
    queryKey: ['customer-inheritance-employees', access.corp.id],
    queryFn: () => api.employees(''),
    enabled: access.corp.authorized !== false,
  });
  const employees = employeesQuery.data ?? [];
  const sourceEmployee = employees.find((item) => String(item.id) === sourceEmployeeId);

  const filter: ContactTransferFilter = {
    contactName,
    employeeIds: sourceEmployeeId ? [Number(sourceEmployeeId)] : [],
    addTimeStart,
    addTimeEnd,
    page: 1,
    perPage: 200,
  };
  const resignedCustomerQuery = useQuery({
    queryKey: ['customer-inheritance', access.corp.id, 'resigned-customer', filter],
    queryFn: () => api.unassigned(filter),
    enabled: access.corp.authorized !== false && inheritanceTab === 'resigned' && assetTab === 'customer',
  });
  const roomQuery = useQuery({
    queryKey: ['customer-inheritance', access.corp.id, 'room', contactName],
    queryFn: () => api.rooms({ roomName: contactName }),
    enabled: access.corp.authorized !== false && inheritanceTab === 'resigned' && assetTab === 'room',
  });
  const activeCustomerQuery = useQuery({
    queryKey: ['customer-inheritance', access.corp.id, 'active-customer', filter],
    queryFn: () => api.assigned(filter),
    enabled: access.corp.authorized !== false && inheritanceTab === 'active' && sourceEmployeeId !== '',
  });

  const customerRows = useMemo(() => {
    const rows = inheritanceTab === 'active' ? (activeCustomerQuery.data ?? []) : (resignedCustomerQuery.data?.list ?? []);
    const keyword = contactName.trim().toLowerCase();
    return rows.filter((row) => !transferredKeys.has(`customer:${row.contactId}`) && (!keyword || `${row.contactName} ${row.nickName} ${row.contactWxId}`.toLowerCase().includes(keyword)));
  }, [activeCustomerQuery.data, contactName, inheritanceTab, resignedCustomerQuery.data, transferredKeys]);
  const roomRows = useMemo(() => {
    const keyword = contactName.trim().toLowerCase();
    return (roomQuery.data ?? []).filter((row) => !transferredKeys.has(`room:${row.chatId}`) && (!keyword || `${row.roomName} ${row.chatId}`.toLowerCase().includes(keyword)));
  }, [contactName, roomQuery.data, transferredKeys]);
  const visibleCustomers = customerRows.slice((page - 1) * pageSize, page * pageSize);
  const visibleRooms = roomRows.slice((page - 1) * pageSize, page * pageSize);
  const selectedCustomers = selectedCustomerRows(customerRows, selected);
  const selectedRooms = selectedRoomRows(roomRows, selected);
  const selectedCount = assetTab === 'customer' ? selectedCustomers.length : selectedRooms.length;
  const canTransfer = selectedCount > 0 && successorId !== '' && (inheritanceTab !== 'active' || sourceEmployeeId !== successorId);

  const currentQuery = assetTab === 'room' ? roomQuery : inheritanceTab === 'active' ? activeCustomerQuery : resignedCustomerQuery;
  const updateUrl = (changes: Record<string, string | undefined>) => {
    const next = new URLSearchParams(searchParams);
    for (const [key, value] of Object.entries(changes)) {
      if (value === undefined || value === '') next.delete(key);
      else next.set(key, value);
    }
    setSearchParams(next);
  };
  const applyFilters = () => updateUrl({ contactName: draft.contactName.trim(), addTimeStart: draft.addTimeStart, addTimeEnd: draft.addTimeEnd, page: '1' });
  const resetFilters = () => {
    setDraft({ contactName: '', addTimeStart: '', addTimeEnd: '' });
    updateUrl({ contactName: undefined, addTimeStart: undefined, addTimeEnd: undefined, page: '1' });
  };
  const changeInheritanceTab = (next: InheritanceTab) => {
    setSelected(new Set());
    setSuccessorId('');
    updateUrl({ inheritanceTab: next, assetTab: 'customer', sourceEmployeeId: undefined, page: '1' });
  };
  const changeAssetTab = (next: AssetTab) => {
    setSelected(new Set());
    updateUrl({ assetTab: next, page: '1' });
  };
  const changeSourceEmployee = (value: string) => {
    setSelected(new Set());
    setSuccessorId('');
    updateUrl({ sourceEmployeeId: value, page: '1' });
  };
  const toggleSelected = (key: string) => setSelected((current) => {
    const next = new Set(current);
    if (next.has(key)) next.delete(key); else next.add(key);
    return next;
  });
  const toggleAll = () => {
    const keys = assetTab === 'room' ? visibleRooms.map((row) => row.chatId) : visibleCustomers.map((row) => String(row.contactId));
    const allSelected = keys.length > 0 && keys.every((key) => selected.has(key));
    setSelected((current) => {
      const next = new Set(current);
      keys.forEach((key) => (allSelected ? next.delete(key) : next.add(key)));
      return next;
    });
  };

  const sync = useMutation({
    mutationFn: () => api.syncUnassigned(),
    onSuccess: async () => {
      setSyncOpen(false);
      setTransferredKeys(new Set());
      setStatusMessage('离职数据已同步');
      await queryClient.invalidateQueries({ queryKey: ['customer-inheritance', access.corp.id] });
    },
  });
  const transfer = useMutation({
    mutationFn: async () => {
      if (assetTab === 'room') return api.transferRooms({ list: selectedRooms.map((row) => row.chatId), takeoverUserId: employees.find((item) => String(item.id) === successorId)?.wxUserId ?? '' });
      return api.transferCustomers({
        type: inheritanceTab === 'active' ? 2 : 1,
        list: selectedCustomers.map((row) => ({ employeeWxId: row.employeeWxId, contactWxId: row.contactWxId })),
        takeoverUserId: employees.find((item) => String(item.id) === successorId)?.wxUserId ?? '',
      });
    },
    onSuccess: async (results) => {
      const keys = assetTab === 'room' ? selectedRooms.map((row) => row.chatId) : selectedCustomers.map((row) => String(row.contactId));
      const failed = new Set(keys.filter((_, index) => results[index]?.errcode !== 0));
      const successful = keys.filter((key) => !failed.has(key));
      const successCount = keys.length - failed.size;
      setTransferredKeys((current) => {
        const next = new Set(current);
        successful.forEach((key) => next.add(`${assetTab}:${key}`));
        return next;
      });
      setSelected(failed);
      setTransferOpen(false);
      setStatusMessage(`成功 ${successCount} 条，失败 ${failed.size} 条${failed.size > 0 ? `：${results.filter((item) => item.errcode !== 0).map((item) => stringValue(item.errmsg) || '处理失败').join('、')}` : ''}`);
      await queryClient.invalidateQueries({ queryKey: ['customer-inheritance', access.corp.id] });
    },
  });

  const logsQuery = useQuery({
    queryKey: ['customer-inheritance-logs', access.corp.id, recordMode, recordFilter],
    queryFn: () => api.logs({ mode: recordMode, name: recordFilter.name, employeeWxId: recordFilter.employee, createTimeStart: recordFilter.from, createTimeEnd: recordFilter.to }),
    enabled: recordsOpen && access.corp.authorized !== false,
  });
  const recordRows = logsQuery.data ?? [];
  const visibleRecords = recordRows.slice((recordPage - 1) * pageSize, recordPage * pageSize);

  const loading = currentQuery.isFetching;
  const queryFields = (
    <>
      <label>客户名称<input aria-label="客户名称" value={draft.contactName} onChange={(event) => setDraft({ ...draft, contactName: event.target.value })} /></label>
      <label>添加开始日期<input aria-label="添加开始日期" type="date" value={draft.addTimeStart} onChange={(event) => setDraft({ ...draft, addTimeStart: event.target.value })} /></label>
      <label>添加结束日期<input aria-label="添加结束日期" type="date" value={draft.addTimeEnd} onChange={(event) => setDraft({ ...draft, addTimeEnd: event.target.value })} /></label>
      {inheritanceTab === 'active' && <label>原跟进员工<select aria-label="原跟进员工" value={sourceEmployeeId} onChange={(event) => changeSourceEmployee(event.target.value)}><option value="">请选择原跟进员工</option>{employees.map((employee) => <option key={employee.id} value={employee.id}>{employee.name}</option>)}</select></label>}
    </>
  );

  return (
    <ConversationOperationsShell title="客户继承" description="管理离职和在职客户继承，保留可追溯的转接记录。" actions={<><button type="button" onClick={() => { setRecordsOpen(true); setRecordPage(1); }}>继承记录</button>{inheritanceTab === 'resigned' && <button type="button" className="is-primary" onClick={() => setSyncOpen(true)}>同步离职数据</button>}</>}>
      <ConversationTabs value={inheritanceTab} tabs={[{ value: 'resigned', label: '离职继承' }, { value: 'active', label: '在职继承' }]} onChange={changeInheritanceTab} />
      {inheritanceTab === 'resigned' && <ConversationTabs value={assetTab} tabs={[{ value: 'customer', label: '待分配客户' }, { value: 'room', label: '待分配群聊' }]} onChange={changeAssetTab} />}
      <ConversationQueryBar fetching={loading} onQuery={applyFilters} onReset={resetFilters} onRefresh={() => void currentQuery.refetch()}>{queryFields}</ConversationQueryBar>
      <section className="conversation-operations-card customer-inheritance-workspace">
        <div className="customer-inheritance-toolbar">
          <label>接替员工<select aria-label="接替员工" value={successorId} onChange={(event) => setSuccessorId(event.target.value)}><option value="">请选择接替员工</option>{employees.map((employee) => <option key={employee.id} value={employee.id}>{employee.name}</option>)}</select></label>
          {inheritanceTab === 'active' && sourceEmployeeId === successorId && successorId !== '' && <span className="customer-inheritance-warning">接替员工不能与原跟进员工相同</span>}
          <button type="button" className="is-primary" disabled={!canTransfer} onClick={() => setTransferOpen(true)}>分配给其他员工</button>
          {statusMessage && <span role="status" className="customer-inheritance-status">{statusMessage}</span>}
        </div>
        {inheritanceTab === 'active' && sourceEmployeeId === '' ? <div className="customer-inheritance-placeholder">请选择需要被继承的员工</div> : currentQuery.isError ? <p role="alert">继承数据加载失败，请重试。</p> : (assetTab === 'room' ? visibleRooms.length === 0 : visibleCustomers.length === 0) ? <div className="customer-inheritance-placeholder">暂无可继承记录</div> : <div className="conversation-operations-table-wrap"><table><thead><tr><th><input aria-label="全选当前页" type="checkbox" checked={(assetTab === 'room' ? visibleRooms : visibleCustomers).length > 0 && (assetTab === 'room' ? visibleRooms.map((row) => row.chatId) : visibleCustomers.map((row) => String(row.contactId))).every((key) => selected.has(key))} onChange={toggleAll} /></th><th>{assetTab === 'room' ? '群聊名称' : '客户名称'}</th><th>{assetTab === 'room' ? '群主' : '原跟进员工'}</th><th>{assetTab === 'room' ? '群人数' : '添加时间'}</th><th>状态</th></tr></thead><tbody>{assetTab === 'room' ? visibleRooms.map((row) => <tr key={row.chatId}><td><input aria-label={`选择群聊 ${row.roomId}`} type="checkbox" checked={selected.has(row.chatId)} onChange={() => toggleSelected(row.chatId)} /></td><td>{row.roomName}</td><td>{row.owner}</td><td>{row.userNum}</td><td>{row.createTime || '--'}</td></tr>) : visibleCustomers.map((row) => <tr key={row.contactId}><td><input aria-label={`选择客户 ${row.contactId}`} type="checkbox" checked={selected.has(String(row.contactId))} onChange={() => toggleSelected(String(row.contactId))} /></td><td>{row.contactName || row.nickName || row.contactWxId}</td><td>{row.employeeName || employeeLabel(sourceEmployee)}</td><td>{formatDateTime(row.addTime)}</td><td>{row.transferState || '--'}</td></tr>)}</tbody></table></div>}
        {(assetTab === 'room' ? roomRows.length : customerRows.length) > pageSize && <DashboardPagination page={page} pageSize={pageSize} total={assetTab === 'room' ? roomRows.length : customerRows.length} onPageChange={(next) => { setSelected(new Set()); updateUrl({ page: String(next) }); }} />}
      </section>
      <DashboardDialog open={syncOpen} title="同步离职待分配数据？" confirmText="开始同步" confirmLoading={sync.isPending} onCancel={() => setSyncOpen(false)} onConfirm={() => sync.mutate()}>
        <p>将从当前企业的企业微信客户联系接口重新拉取待分配客户。现有待分配快照会被真实同步结果替换。</p>
        {sync.isError && <p role="alert">同步失败，请稍后重试。</p>}
      </DashboardDialog>
      <DashboardDialog open={transferOpen} title="确认分配" confirmText="确认分配" confirmLoading={transfer.isPending} onCancel={() => setTransferOpen(false)} onConfirm={() => transfer.mutate()}>
        <p>将 {selectedCount} 条{assetTab === 'room' ? '群聊' : '客户'}分配给 {employeeLabel(employees.find((item) => String(item.id) === successorId))}。</p>
        {inheritanceTab === 'active' && sourceEmployee && <p>原跟进员工：{sourceEmployee.name}</p>}
        {transfer.isError && <p role="alert">分配失败，请检查网络后重试。</p>}
      </DashboardDialog>
      <DashboardDialog mode="drawer" open={recordsOpen} title="继承记录" cancelText="关闭继承记录" onCancel={() => setRecordsOpen(false)}>
        <ConversationTabs value={String(recordMode)} tabs={recordModes.map((mode) => ({ value: String(mode.value), label: mode.label }))} onChange={(value) => { setRecordMode(Number(value) as 1 | 2 | 3); setRecordPage(1); }} />
        <div className="customer-inheritance-record-filters"><input aria-label="记录名称" placeholder="名称" value={recordFilter.name} onChange={(event) => setRecordFilter({ ...recordFilter, name: event.target.value })} /><input aria-label="记录员工" placeholder="接替员工 WX ID" value={recordFilter.employee} onChange={(event) => setRecordFilter({ ...recordFilter, employee: event.target.value })} /><input aria-label="记录开始日期" type="date" value={recordFilter.from} onChange={(event) => setRecordFilter({ ...recordFilter, from: event.target.value })} /><input aria-label="记录结束日期" type="date" value={recordFilter.to} onChange={(event) => setRecordFilter({ ...recordFilter, to: event.target.value })} /><button type="button" onClick={() => void logsQuery.refetch()}>查询</button></div>
        {visibleRecords.length === 0 ? <div className="customer-inheritance-placeholder">暂无继承记录</div> : <div className="conversation-operations-table-wrap"><table><thead><tr><th>名称</th><th>接替员工</th><th>状态</th><th>时间</th></tr></thead><tbody>{visibleRecords.map((row, index) => <tr key={`${row.mode}-${row.name}-${index}`}><td>{row.name || '--'}</td><td>{row.employee || '--'}</td><td>{row.state || '--'}</td><td>{formatDateTime(row.createTime)}</td></tr>)}</tbody></table></div>}
        {recordRows.length > pageSize && <DashboardPagination page={recordPage} pageSize={pageSize} total={recordRows.length} onPageChange={setRecordPage} />}
      </DashboardDialog>
    </ConversationOperationsShell>
  );
}
