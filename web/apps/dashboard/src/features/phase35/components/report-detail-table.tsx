import { text } from '../api';
import { metricLabel } from '../presentation/labels';
import { formatDate,formatMetric } from '../presentation/formatters';
import { DashboardPagination } from '../../../components/dashboard-pagination';
export type DetailColumn = { key: string; label: string };
export function ReportDetailTable({items,page=1,pageSize=20,total=0,onPageChange,columns}:{items:Record<string,unknown>[];page?:number;pageSize?:number;total?:number;onPageChange?:(page:number)=>void;columns?:DetailColumn[]}){const keys=columns?columns.map((column)=>column.key):Object.keys(items[0]??{});const labelFor=(key:string)=>columns?.find((column)=>column.key===key)?.label??metricLabel(key);return <><table><thead><tr>{keys.map((key)=><th key={key}>{labelFor(key)}</th>)}</tr></thead><tbody>{items.map((row,index)=><tr key={text(row.id??index)}>{keys.map((key)=><td key={key}>{key.endsWith('At')?formatDate(row[key]):formatMetric(row[key])}</td>)}</tr>)}</tbody></table><DashboardPagination page={page} pageSize={pageSize} total={total} onPageChange={(nextPage)=>onPageChange?.(nextPage)}/></>}
