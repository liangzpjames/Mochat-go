import { useMemo, useState } from 'react';
import { useOptionalDashboardAccess } from '../../app/access-context';

const isoDate = (date: Date) => date.toISOString().slice(0, 10);

export function zonedBoundary(date: string, timezone: string): string {
  const probe = new Date(`${date}T00:00:00Z`);
  const part = new Intl.DateTimeFormat('en-US', { timeZone: timezone, timeZoneName: 'longOffset' })
    .formatToParts(probe).find((item) => item.type === 'timeZoneName')?.value ?? 'GMT';
  return `${date}T00:00:00${part === 'GMT' ? 'Z' : part.replace(/^GMT/, '')}`;
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
  const params = useMemo(() => ({
    ...(access?.corp.id ? { corpId: Number(access.corp.id) } : {}), timezone,
    startAt: zonedBoundary(applied.startDate, timezone), endAt: zonedBoundary(applied.endDate, timezone),
    ...(applied.employeeId ? { employeeId: Number(applied.employeeId) } : {}),
    ...(applied.departmentId ? { departmentId: Number(applied.departmentId) } : {}),
  }), [access?.corp.id, applied, timezone]);
  const apply = () => setApplied({ startDate, endDate, employeeId, departmentId });
  const reset = () => {
    setStartDate(initialStart); setEndDate(initialEnd); setEmployeeId(''); setDepartmentId('');
    setApplied({ startDate: initialStart, endDate: initialEnd, employeeId: '', departmentId: '' });
  };
  return { params, startDate, endDate, employeeId, departmentId, setStartDate, setEndDate, setEmployeeId, setDepartmentId, apply, reset };
}

export function ReportFilters({ filters }: { filters: ReturnType<typeof useReportFilters> }) {
  return <form className="dashboard-filter-bar" onSubmit={(event) => { event.preventDefault(); filters.apply(); }}>
    <label>开始日期<input aria-label="开始日期" type="date" value={filters.startDate} onChange={(event) => filters.setStartDate(event.target.value)} /></label>
    <label>结束日期<input aria-label="结束日期" type="date" value={filters.endDate} onChange={(event) => filters.setEndDate(event.target.value)} /></label>
    <label>员工<input aria-label="员工" inputMode="numeric" value={filters.employeeId} onChange={(event) => filters.setEmployeeId(event.target.value.replace(/\D/g, ''))} /></label>
    <label>部门<input aria-label="部门" inputMode="numeric" value={filters.departmentId} onChange={(event) => filters.setDepartmentId(event.target.value.replace(/\D/g, ''))} /></label>
    <button type="submit">查询</button><button type="button" onClick={filters.reset}>重置</button>
  </form>;
}
