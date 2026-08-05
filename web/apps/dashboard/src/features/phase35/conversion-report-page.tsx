import { useQuery } from '@tanstack/react-query';
import { reportEndpoint, type Phase35Api } from './api';
import { ReportFilters, useReportFilters } from './report-query';
import { ReportPrimitives } from './report-primitives';
import type { ReportResult } from './report-types';
import { Phase35PageShell } from './components/phase35-page-shell';
import { Phase35DataState } from './components/data-state';
import { ConversionFunnel } from './components/conversion-funnel';
export function ConversionReportPage({ api }: { api: Phase35Api }) { const filters = useReportFilters(); const query = useQuery({ queryKey: ['p35-report', 'conversion', filters.params], queryFn: () => api.read(reportEndpoint('conversion'), filters.params) }); const result = query.data as ReportResult | undefined; return <Phase35PageShell title="转化分析" description="线索、联系人、商机、赢单与订单漏斗"><ReportFilters filters={filters} /><Phase35DataState loading={query.isLoading} error={query.isError}><ConversionFunnel summary={result?.summary ?? {}} />{result ? <ReportPrimitives result={result} /> : <p role="status">暂无数据。</p>}</Phase35DataState></Phase35PageShell>; }
