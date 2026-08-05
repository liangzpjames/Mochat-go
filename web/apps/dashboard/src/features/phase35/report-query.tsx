import { useMemo, useState } from 'react';
import { useOptionalDashboardAccess } from '../../app/access-context';

function isoDate(date: Date): string {
  return date.toISOString().slice(0, 10);
}

export function zonedBoundary(date: string, timezone: string): string {
  const probe = new Date(`${date}T00:00:00Z`);
  const part = new Intl.DateTimeFormat('en-US', { timeZone: timezone, timeZoneName: 'longOffset' })
    .formatToParts(probe).find((item) => item.type === 'timeZoneName')?.value ?? 'GMT';
  const offset = part === 'GMT' ? 'Z' : part.replace(/^GMT/, '');
  return `${date}T00:00:00${offset}`;
}

export function useReportFilters() {
  const access = useOptionalDashboardAccess();
  const today = useMemo(() => new Date(), []);
  const [startDate, setStartDate] = useState(() => isoDate(new Date(today.getFullYear(), today.getMonth(), 1)));
  const [endDate, setEndDate] = useState(() => isoDate(new Date(today.getFullYear(), today.getMonth() + 1, 1)));
  const [employeeId, setEmployeeId] = useState('');
  const [departmentId, setDepartmentId] = useState('');
  const params = useMemo(() => ({
    ...(access?.corp.id ? { corpId: Number(access.corp.id) } : {}),
    timezone: Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC',
    startAt: zonedBoundary(startDate, Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC'),
    endAt: zonedBoundary(endDate, Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC'),
    ...(employeeId ? { employeeId: Number(employeeId) } : {}),
    ...(departmentId ? { departmentId: Number(departmentId) } : {}),
  }), [access?.corp.id, departmentId, employeeId, endDate, startDate]);
  return { params, startDate, endDate, employeeId, departmentId, setStartDate, setEndDate, setEmployeeId, setDepartmentId };
}

export function ReportFilters({ filters }: { filters: ReturnType<typeof useReportFilters> }) {
  return <form className="dashboard-filter-bar" onSubmit={(event) => event.preventDefault()}>
    <label>开始日期<input aria-label="开始日期" type="date" value={filters.startDate} onChange={(event) => filters.setStartDate(event.target.value)} /></label>
    <label>结束日期<input aria-label="结束日期" type="date" value={filters.endDate} onChange={(event) => filters.setEndDate(event.target.value)} /></label>
    <label>员工 ID<input aria-label="员工 ID" inputMode="numeric" value={filters.employeeId} onChange={(event) => filters.setEmployeeId(event.target.value.replace(/\D/g, ''))} /></label>
    <label>部门 ID<input aria-label="部门 ID" inputMode="numeric" value={filters.departmentId} onChange={(event) => filters.setDepartmentId(event.target.value.replace(/\D/g, ''))} /></label>
  </form>;
}
