import { PageState } from '../../components/page-state/page-state';
import type { RiskRecordDetail as Detail, RiskRecord } from './risk-behavior-api';
import { auditStatusLabel, conversationTypeLabel, formatRiskDateTime, riskBehaviorLabel, riskLevelLabel } from '../risk-warning/risk-warning-format';

function relatedName(record: RiskRecord): string {
  const related = record.relatedUser;
  const value = related.name ?? related.employeeName ?? related.customerName ?? related.roomName;
  return typeof value === 'string' && value.trim() ? value : '未标注对象';
}

export function RiskBehaviorDetail({ detail, loading, error, onRetry, supplementalError }: { detail: Detail | undefined; loading: boolean; error: unknown; onRetry: () => void; supplementalError?: unknown }) {
  if (loading) return <PageState state="loading" />;
  if (error) return <PageState state="error" onRetry={onRetry} />;
  if (!detail) return <PageState state="empty" />;
  const { record, audits } = detail;
  return <div>{supplementalError ? <div className="risk-warning-status-strip">详情补充接口暂不可用，当前内容已从风险记录列表读取，未虚构缺失数据。</div> : null}<dl className="risk-warning-detail-grid"><div><dt>风险行为</dt><dd>{riskBehaviorLabel(record.behavior)}</dd></div><div><dt>风险等级</dt><dd>{riskLevelLabel(record.riskLevel)}</dd></div><div><dt>关联对象</dt><dd>{relatedName(record)}</dd></div><div><dt>会话类型</dt><dd>{conversationTypeLabel(record.conversationType)}</dd></div><div><dt>触发内容</dt><dd>{record.triggerMessage || '暂无触发内容'}</dd></div><div><dt>审核状态</dt><dd>{auditStatusLabel(record.auditStatus)}</dd></div><div><dt>发生时间</dt><dd>{formatRiskDateTime(record.occurredAt)}</dd></div></dl><section className="risk-warning-detail-section"><h3>处置记录</h3>{audits.length === 0 ? <p className="risk-warning-muted">暂无处置记录</p> : <div className="risk-warning-audit-list">{audits.map((audit) => <article key={audit.id}><strong>{audit.action === 'confirmed' ? '已确认' : audit.action === 'ignored' ? '已忽略' : audit.action === 'reviewed' ? '已复核' : audit.action}</strong><span>{audit.remark || '未填写备注'}</span><time>{formatRiskDateTime(audit.createdAt)}</time></article>)}</div>}</section><section className="risk-warning-detail-section"><h3>会话定位</h3>{detail.conversationAvailable ? <p className="risk-warning-detail-available">已找到对应会话存档，可从会话菜单继续查看。</p> : <p className="risk-warning-muted">当前记录没有可定位的会话存档或会话索引，页面不虚构跳转入口。</p>}</section></div>;
}
