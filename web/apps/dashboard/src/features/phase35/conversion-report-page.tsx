import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';
import { reportEndpoint,type Phase35Api } from './api';
import { ReportFilters,useReportFilters } from './report-query';
import type { ReportResult } from './report-types';
import { Phase35PageShell } from './components/phase35-page-shell';
import { Phase35DataState } from './components/data-state';
import { MetricCardGrid } from './components/metric-card-grid';
import { Phase35DetailDrawer } from './components/detail-drawer';
import { ConversionFunnel,conversionStageLabel } from './components/conversion-funnel';
export function ConversionReportPage({api}:{api:Phase35Api}){const filters=useReportFilters();const [stage,setStage]=useState('');const query=useQuery({queryKey:['p35-report','conversion',filters.params],queryFn:()=>api.read(reportEndpoint('conversion'),filters.params)});const result=query.data as ReportResult|undefined;const summary=result?.summary??{};return <Phase35PageShell title="转化分析" description="线索、联系人、商机、赢单与订单的统一口径漏斗"><ReportFilters filters={filters}/><Phase35DataState loading={query.isLoading} error={query.isError} onRetry={()=>void query.refetch()}><MetricCardGrid items={[{label:'联系人转化率',value:summary.contactRate,unit:'%'},{label:'商机转化率',value:summary.opportunityRate,unit:'%'},{label:'赢单转化率',value:summary.wonRate,unit:'%'},{label:'订单转化率',value:summary.orderRate,unit:'%'}]}/><ConversionFunnel summary={summary} onStageClick={setStage}/>{result?.limitations?.map((item)=><p role="status" key={item.provider}>{item.message}</p>)}</Phase35DataState><Phase35DetailDrawer title={`${conversionStageLabel(stage)}阶段明细`} open={Boolean(stage)} onClose={()=>setStage('')}><p>{conversionStageLabel(stage)}阶段共 {String(summary[stage]??0)} 条记录。</p>{(result?.items??[]).length?<p>可按当前筛选条件查看对应业务对象。</p>:<p role="status">当前筛选范围暂无该阶段明细，请调整日期、员工或部门后查询。</p>}</Phase35DetailDrawer></Phase35PageShell>}
