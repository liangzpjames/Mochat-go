import { useQuery } from '@tanstack/react-query';
import { reportEndpoint, type Phase35Api } from './api';
import { ReportFilters, useReportFilters } from './report-query';
import { ReportPrimitives } from './report-primitives';
import { Phase35PageShell } from './components/phase35-page-shell';
import { Phase35DataState } from './components/data-state';
import { ProviderStatusCard } from './components/provider-status-card';
export function EmployeeReportPage({ api }: { api: Phase35Api }) { const filters = useReportFilters(); const query = useQuery({ queryKey: ['p35-report', 'employee', filters.params], queryFn: () => api.read(reportEndpoint('employee'), filters.params) }); const result = query.data as { limitations?: { message: string }[]; summary?: Record<string, number | null> } | undefined; return <Phase35PageShell title="会话分析" description="展示会话 Provider 可用性、员工指标与受限能力"><ReportFilters filters={filters} /><Phase35DataState loading={query.isLoading} error={query.isError}><ProviderStatusCard available={Boolean(result?.summary && Object.keys(result.summary).length)} provider="会话归档 Provider" {...(result?.limitations ? { limitations: result.limitations } : {})} />{result ? <ReportPrimitives result={result as never} /> : <p role="status">暂无数据。</p>}</Phase35DataState></Phase35PageShell>; }
