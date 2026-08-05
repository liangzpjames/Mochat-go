import { useQuery } from '@tanstack/react-query';
import type { Phase35Api } from './api';
import { ReportFilters, useReportFilters } from './report-query';
import { ReportPrimitives } from './report-primitives';

export function BehaviorReportPage({ api }: { api: Phase35Api }) {
  const filters = useReportFilters();
  const query = useQuery({ queryKey: ['p35-report', 'behavior', filters.params], queryFn: () => api.read('/dashboard/reports/behavior', filters.params) });
  return <section className="dashboard-content"><h1>行为分析</h1><ReportFilters filters={filters} />{query.isLoading ? <p>加载中…</p> : query.isError ? <p role="alert">行为报表加载失败</p> : <ReportPrimitives result={query.data as never} />}</section>;
}
