import { useQuery } from '@tanstack/react-query';
import { useMemo, useState } from 'react';
import { useOptionalDashboardAccess } from '../../app/access-context';
import { DateRangeFields, validateDateRange } from '../../components/date-range-fields';
import { records, text, type Phase35Api, type Row } from './api';

const isoDate = (date: Date) => `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`;
const flattenRows = (rows: Row[]): Row[] => rows.flatMap((row) => [row, ...flattenRows(records(row.children))]);

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
  const departments = useQuery({
    queryKey: ['p35-department-options', access?.corp.id],
    queryFn: () => api!.read('/workDepartment/pageIndex', { name: '', parentName: '', page: 1, perPage: 200 }),
    enabled: Boolean(api && access?.corp.id),
  });
  const employeeOptions = records(employees.data);
  const departmentOptions = flattenRows(records(departments.data));
  return <form className="dashboard-filter-bar" onSubmit={(event) => { event.preventDefault(); if (validateDateRange({ startDate: filters.startDate, endDate: filters.endDate }) === null) filters.apply(); }}>
    <DateRangeFields value={{ startDate: filters.startDate, endDate: filters.endDate }} onChange={(value) => { filters.setStartDate(value.startDate); filters.setEndDate(value.endDate); }} />
    <label>员工
      <select aria-label="员工" value={filters.employeeId} onChange={(event) => filters.setEmployeeId(event.target.value)}>
        <option value="">全部员工</option>
        {employeeOptions.map((row) => <option key={String(row.id)} value={String(row.id)}>{text(row.name ?? row.id)}</option>)}
      </select>
    </label>
    <label>部门<select aria-label="部门" value={filters.departmentId} onChange={(event) => filters.setDepartmentId(event.target.value)}><option value="">全部部门</option>{departmentOptions.map((row) => <option key={String(row.departmentId ?? row.id)} value={String(row.departmentId ?? row.id)}>{text(row.name ?? row.departmentName)}</option>)}</select></label>
    <button type="submit">查询</button><button type="button" onClick={filters.reset}>重置</button>
  </form>;
}
