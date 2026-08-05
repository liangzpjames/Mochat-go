import { useQuery } from '@tanstack/react-query';
import { reportEndpoint, type Phase35Api } from './api';
import { ReportFilters, useReportFilters } from './report-query';
import { ReportPrimitives } from './report-primitives';

export function ReportPage({ api }: { api: Phase35Api }) {
  const filters = useReportFilters();
  const query = useQuery({ queryKey: ['p35-report', 'report', filters.params], queryFn: () => api.read(reportEndpoint('report'), filters.params) });
  return <section className="dashboard-content"><h1>数据报表</h1><ReportFilters filters={filters} />{query.isLoading ? <p>加载中…</p> : query.isError ? <p role="alert">综合报表加载失败</p> : <ReportPrimitives result={query.data as never} />}</section>;
}
