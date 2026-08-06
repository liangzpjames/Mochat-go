import { useQuery } from '@tanstack/react-query';
import { reportEndpoint,type Phase35Api } from './api';
import { ReportFilters,useReportFilters } from './report-query';
import { ReportPrimitives } from './report-primitives';
import type { ReportResult } from './report-types';
import { Phase35PageShell } from './components/phase35-page-shell';
import { Phase35DataState } from './components/data-state';
import { MetricCardGrid } from './components/metric-card-grid';
export function CustomerReportPage({api}:{api:Phase35Api}){const filters=useReportFilters();const query=useQuery({queryKey:['p35-report','customer',filters.params],queryFn:()=>api.read(reportEndpoint('customer'),filters.params)});const result=query.data as ReportResult|undefined;const empty=!query.isLoading&&!Number(result?.summary?.customer??0)&&!(result?.items?.length);return <Phase35PageShell title="客户分析" description="按日期与授权范围查看客户规模、趋势、负责人分布和明细"><ReportFilters filters={filters}/><MetricCardGrid items={[{label:'客户总数',value:result?.summary?.customer??0},{label:'当前范围新增',value:result?.pagination?.total??0}]}/><Phase35DataState loading={query.isLoading} error={query.isError} empty={empty} emptyContent={<section aria-label="客户分析数据说明"><h2>当前范围还没有客户数据</h2><p>本页统计真实 SCRM 联系人及负责人分配记录。可先创建联系人，或扩大日期范围后重新查询。</p><a href="/customer/order">前往订单页快速创建联系人</a></section>} onRetry={()=>void query.refetch()}><ReportPrimitives result={result}/></Phase35DataState></Phase35PageShell>}
