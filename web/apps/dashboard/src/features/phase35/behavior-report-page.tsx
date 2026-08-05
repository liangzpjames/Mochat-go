import { useQuery } from '@tanstack/react-query';
import { reportEndpoint, type Phase35Api } from './api';
import { ReportFilters, useReportFilters } from './report-query';
import { ReportPrimitives } from './report-primitives';
import { Phase35PageShell } from './components/phase35-page-shell';
import { Phase35DataState } from './components/data-state';
export function BehaviorReportPage({ api }: { api: Phase35Api }) { const filters = useReportFilters(); const query = useQuery({ queryKey: ['p35-report', 'behavior', filters.params], queryFn: () => api.read(reportEndpoint('behavior'), filters.params) }); return <Phase35PageShell title="行为分析" description="查看可审计业务事件、趋势和明细"><ReportFilters filters={filters} /><Phase35DataState loading={query.isLoading} error={query.isError}><ReportPrimitives result={query.data as never} /></Phase35DataState></Phase35PageShell>; }
