import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';
import { reportEndpoint,type Phase35Api } from './api';
import { ReportFilters,useReportFilters } from './report-query';
import type { ReportItem,ReportResult } from './report-types';
import { paginationOf } from './report-types';
import { behaviorLabel } from './presentation/labels';
import { formatDate } from './presentation/formatters';
import { Phase35PageShell } from './components/phase35-page-shell';
import { Phase35DataState } from './components/data-state';
import { MetricCardGrid } from './components/metric-card-grid';
import { Phase35DetailDrawer } from './components/detail-drawer';
export function BehaviorReportPage({api}:{api:Phase35Api}){const filters=useReportFilters();const [selected,setSelected]=useState<ReportItem>();const query=useQuery({queryKey:['p35-report','behavior',filters.params],queryFn:()=>api.read(reportEndpoint('behavior'),filters.params)});const result=query.data as ReportResult|undefined;const pagination=paginationOf(result);const items=result?.items??[];return <Phase35PageShell title="行为分析" description="查看设置、订单等可审计业务操作及其操作者和对象"><ReportFilters filters={filters}/><Phase35DataState loading={query.isLoading} error={query.isError} empty={!query.isLoading&&!items.length} onRetry={()=>void query.refetch()}><MetricCardGrid items={[{label:'行为事件总数',value:result?.summary?.behavior??pagination.total,unit:'条'}]}/><table><thead><tr><th>行为类型</th><th>操作者</th><th>业务对象</th><th>发生时间</th><th>变更摘要</th><th>操作</th></tr></thead><tbody>{items.map((item,index)=><tr key={String(item.id??index)}><td>{behaviorLabel(item.eventType)}</td><td>账号 {String(item.actorId??'--')}</td><td>{String(item.objectLabel??item.objectId??'--')}</td><td>{formatDate(item.occurredAt)}</td><td>{String(item.detail??'可查看详情')}</td><td><button type="button" aria-label="查看行为详情" onClick={()=>setSelected(item)}>查看详情</button></td></tr>)}</tbody></table><p>共 {pagination.total} 条，第 {pagination.page} 页</p></Phase35DataState><Phase35DetailDrawer title="行为详情" open={Boolean(selected)} onClose={()=>setSelected(undefined)}><dl><dt>行为类型</dt><dd>{behaviorLabel(selected?.eventType)}</dd><dt>操作者</dt><dd>账号 {String(selected?.actorId??'--')}</dd><dt>业务对象</dt><dd>{String(selected?.objectLabel??selected?.objectId??'--')}</dd><dt>发生时间</dt><dd>{formatDate(selected?.occurredAt)}</dd><dt>变更摘要</dt><dd>{String(selected?.detail??'暂无补充说明')}</dd></dl></Phase35DetailDrawer></Phase35PageShell>}
