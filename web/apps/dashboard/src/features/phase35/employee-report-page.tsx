import { useQuery } from '@tanstack/react-query';
import { reportEndpoint, type Phase35Api } from './api';
import { ReportFilters, useReportFilters } from './report-query';
import { ReportPrimitives } from './report-primitives';
import { Phase35PageShell } from './components/phase35-page-shell';
import { Phase35DataState } from './components/data-state';
export function EmployeeReportPage({ api }: { api: Phase35Api }) { const filters = useReportFilters(); const query = useQuery({ queryKey: ['p35-report', 'employee', filters.params], queryFn: () => api.read(reportEndpoint('employee'), filters.params) }); return <Phase35PageShell title="会话分析" description="展示会话 Provider 可用性、员工指标与受限能力"><ReportFilters filters={filters} /><Phase35DataState loading={query.isLoading} error={query.isError}><ReportPrimitives result={query.data as never} /></Phase35DataState></Phase35PageShell>; }
