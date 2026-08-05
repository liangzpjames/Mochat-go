import { useQuery } from '@tanstack/react-query';
import type { Phase35Api } from './api';
import { ReportFilters, useReportFilters } from './report-query';
import { ReportPrimitives } from './report-primitives';
import type { ReportResult } from './report-types';

export function CustomerReportPage({ api }: { api: Phase35Api }) {
  const filters = useReportFilters();
  const query = useQuery({ queryKey: ['p35-report', 'customer', filters.params], queryFn: () => api.read('/dashboard/reports/customer', filters.params) });
  return <section className="dashboard-content"><h1>客户分析</h1><ReportFilters filters={filters} />{query.isLoading ? <p>加载中…</p> : query.isError ? <p role="alert">客户报表加载失败</p> : <ReportPrimitives result={query.data as ReportResult} />}</section>;
}
