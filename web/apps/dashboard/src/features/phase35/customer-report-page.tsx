import { useQuery } from '@tanstack/react-query';
import { reportEndpoint, type Phase35Api } from './api';
import { ReportFilters, useReportFilters } from './report-query';
import { ReportPrimitives } from './report-primitives';
import type { ReportResult } from './report-types';
import { Phase35PageShell } from './components/phase35-page-shell';
import { Phase35DataState } from './components/data-state';
export function CustomerReportPage({ api }: { api: Phase35Api }) { const filters = useReportFilters(); const query = useQuery({ queryKey: ['p35-report', 'customer', filters.params], queryFn: () => api.read(reportEndpoint('customer'), filters.params) }); return <Phase35PageShell title="客户分析" description="按日期与授权范围查看客户趋势、指标和明细"><ReportFilters filters={filters} /><Phase35DataState loading={query.isLoading} error={query.isError}><ReportPrimitives result={query.data as ReportResult} /></Phase35DataState></Phase35PageShell>; }
