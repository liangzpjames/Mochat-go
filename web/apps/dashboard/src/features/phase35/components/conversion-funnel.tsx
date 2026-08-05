import { formatMetric } from '../presentation/formatters';
const stages = ['lead', 'contact', 'opportunity', 'won', 'order'] as const;
export function ConversionFunnel({ summary, onStageClick }: { summary: Record<string, number | null>; onStageClick?: (stage: string) => void }) {
  return <div aria-label="转化漏斗" className="dashboard-funnel">{stages.map((stage, index) => { const value = summary[stage] ?? null; const rate = index ? summary[`${stage}Rate`] : null; return <button type="button" key={stage} onClick={() => onStageClick?.(stage)}><span>{stage === 'lead' ? '线索' : stage === 'contact' ? '联系人' : stage === 'opportunity' ? '商机' : stage === 'won' ? '赢单' : '订单'}</span><strong>{formatMetric(value)}</strong><small>{index ? `转化率 ${formatMetric(rate, '%')}` : '起始阶段'}</small></button>; })}</div>;
}
