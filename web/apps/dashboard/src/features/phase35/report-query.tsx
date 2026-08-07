import { useQuery } from '@tanstack/react-query';
import { useMemo, useState } from 'react';
import { useOptionalDashboardAccess } from '../../app/access-context';
import { records, text, type Phase35Api } from './api';

const isoDate = (date: Date) => `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`;

export function zonedBoundary(date: string, timezone: string): string {
  const probe = new Date(`${date}T00:00:00Z`);
  const part = new Intl.DateTimeFormat('en-US', { timeZone: timezone, timeZoneName: 'longOffset' })
    .formatToParts(probe).find((item) => item.type === 'timeZoneName')?.value ?? 'GMT';
  return `${date}T00:00:00${part === 'GMT' ? 'Z' : part.replace(/^GMT/, '')}`;
}

export function buildReportFilterParams(
  applied: { startDate: string; endDate: string; employeeId: string; departmentId: string },
  timezone: string,
  corpId?: number,
) {
  return {
    ...(corpId ? { corpId } : {}),
    timezone,
    startAt: zonedBoundary(applied.startDate, timezone),
    endAt: zonedBoundary(applied.endDate, timezone),
    ...(applied.employeeId ? { employeeIds: applied.employeeId } : {}),
    ...(applied.departmentId ? { departmentIds: applied.departmentId } : {}),
  };
}

export function useReportFilters() {
  const access = useOptionalDashboardAccess();
  const today = useMemo(() => new Date(), []);
  const initialStart = useMemo(() => isoDate(new Date(today.getFullYear(), today.getMonth(), 1)), [today]);
  const initialEnd = useMemo(() => isoDate(new Date(today.getFullYear(), today.getMonth() + 1, 1)), [today]);
  const [startDate, setStartDate] = useState(initialStart);
  const [endDate, setEndDate] = useState(initialEnd);
  const [employeeId, setEmployeeId] = useState('');
  const [departmentId, setDepartmentId] = useState('');
  const [applied, setApplied] = useState({ startDate: initialStart, endDate: initialEnd, employeeId: '', departmentId: '' });
  const timezone = Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC';
  const params = useMemo(
    () => buildReportFilterParams(applied, timezone, access?.corp.id ? Number(access.corp.id) : undefined),
    [access?.corp.id, applied, timezone],
  );
  const apply = () => setApplied({ startDate, endDate, employeeId, departmentId });
  const reset = () => {
    setStartDate(initialStart); setEndDate(initialEnd); setEmployeeId(''); setDepartmentId('');
    setApplied({ startDate: initialStart, endDate: initialEnd, employeeId: '', departmentId: '' });
  };
  return { params, startDate, endDate, employeeId, departmentId, setStartDate, setEndDate, setEmployeeId, setDepartmentId, apply, reset };
}

export function ReportFilters({ filters, api }: { filters: ReturnType<typeof useReportFilters>; api?: Phase35Api }) {
  const access = useOptionalDashboardAccess();
  const employees = useQuery({
    queryKey: ['p35-employee-options', access?.corp.id],
    queryFn: () => api!.read('/workEmployee/index', { page: 1, perPage: 200 }),
    enabled: Boolean(api && access?.corp.id),
  });
  const employeeOptions = records(employees.data);
  return <form className="dashboard-filter-bar" onSubmit={(event) => { event.preventDefault(); filters.apply(); }}>
    <label>开始日期<input aria-label="开始日期" type="date" value={filters.startDate} onChange={(event) => filters.setStartDate(event.target.value)} /></label>
    <label>结束日期<input aria-label="结束日期" type="date" value={filters.endDate} onChange={(event) => filters.setEndDate(event.target.value)} /></label>
    <label>员工
      <select aria-label="员工" value={filters.employeeId} onChange={(event) => filters.setEmployeeId(event.target.value)}>
        <option value="">全部员工</option>
        {employeeOptions.map((row) => <option key={String(row.id)} value={String(row.id)}>{text(row.name ?? row.id)}</option>)}
      </select>
    </label>
    <label>部门<input aria-label="部门" inputMode="numeric" value={filters.departmentId} onChange={(event) => filters.setDepartmentId(event.target.value.replace(/\D/g, ''))} /></label>
    <button type="submit">查询</button><button type="button" onClick={filters.reset}>重置</button>
  </form>;
}
