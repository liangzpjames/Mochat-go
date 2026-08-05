import { useQuery } from '@tanstack/react-query';
import { reportEndpoint, type Phase35Api } from './api';
import { ReportFilters, useReportFilters } from './report-query';
import { ReportPrimitives } from './report-primitives';
import { Phase35PageShell } from './components/phase35-page-shell';
import { Phase35DataState } from './components/data-state';
export function ReportPage({ api }: { api: Phase35Api }) { const filters = useReportFilters(); const query = useQuery({ queryKey: ['p35-report', 'report', filters.params], queryFn: () => api.read(reportEndpoint('report'), filters.params) }); return <Phase35PageShell title="综合报表" description="汇总客户、转化、订单与行为指标并支持追溯"><ReportFilters filters={filters} /><Phase35DataState loading={query.isLoading} error={query.isError}><ReportPrimitives result={query.data as never} /></Phase35DataState></Phase35PageShell>; }
