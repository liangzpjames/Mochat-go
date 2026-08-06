import type { ReportResult } from './report-types';
import { paginationOf } from './report-types';
import { MetricCardGrid } from './components/metric-card-grid';
import { ReportDetailTable, type DetailColumn } from './components/report-detail-table';
import { SimpleTrendChart } from './components/simple-trend-chart';
export function ReportPrimitives({result,itemColumns}:{result?:ReportResult|undefined;itemColumns?:DetailColumn[]}) {
  if(!result)return <p role="status">暂无数据。</p>;
  const items=result.items??[];const pagination=paginationOf(result);
  const dimensions=Array.isArray(result.dimensions)?result.dimensions:Object.entries(result.dimensions??{}).map(([key,value])=>({key,label:key,value:Number(value??0)}));
  return <><MetricCardGrid metrics={result.summary??{}}/>{Boolean(result.series?.length)&&<section aria-label="趋势"><h2>趋势</h2><SimpleTrendChart points={result.series!.map((point)=>({at:String(point.at??point.day??''),value:Number(point.value??0)}))}/></section>}{dimensions.length>0&&<section aria-label="分布"><h2>分布</h2><MetricCardGrid items={dimensions.map((item)=>({label:item.label,value:item.value}))}/></section>}{result.limitations?.map((item)=><p role="status" key={item.provider}>{item.message}</p>)}{items.length?<ReportDetailTable items={items} {...pagination} {...(itemColumns?{columns:itemColumns}:{})}/>:!result.limitations?.length&&<p role="status">当前筛选范围暂无明细数据。</p>}</>;
}
