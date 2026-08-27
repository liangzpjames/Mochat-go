import { useState } from 'react';
import type { ReactNode } from 'react';
import { Link } from 'react-router';

import type {
  ConversationTrendPoint,
  DashboardOverviewAIInsight,
  DashboardOverviewAIMetrics,
  DashboardOverviewConversation,
  DashboardOverviewEmployeeRankingItem,
  DashboardOverviewLimitation,
  DashboardOverviewQuality,
  DashboardOverviewTrajectoryItem,
  DashboardOverviewTrendPoint,
} from './dashboard-overview-api';

type AISummaryLine =
  | { kind: 'divider' }
  | { kind: 'section'; text: string }
  | { kind: 'numbered'; index: string; text: string }
  | { kind: 'bullet'; text: string }
  | { kind: 'paragraph'; text: string };

export function parseAISummary(summary: string): AISummaryLine[] {
  return summary.split(/\r?\n/).map((raw) => {
    const line = raw.trim();
    if (line === '') return null;
    if (/^-{3,}$/.test(line)) return { kind: 'divider' as const };
    if (line.startsWith('✅') || line.startsWith('⚠️')) return { kind: 'section' as const, text: line };
    const numbered = line.match(/^(\d+)[.、．]\s*(.*)$/);
    if (numbered) return { kind: 'numbered' as const, index: numbered[1], text: numbered[2] };
    if (/^[-•]\s+/.test(line)) return { kind: 'bullet' as const, text: line.replace(/^[-•]\s+/, '') };
    return { kind: 'paragraph' as const, text: line };
  }).filter((line): line is AISummaryLine => line !== null);
}

function metricValue(value: number | null): string {
  return value === null ? '--' : value.toLocaleString('zh-CN');
}

function renderInline(text: string): ReactNode {
  return text.split(/\*\*(.+?)\*\*/g).map((part, index) => (index % 2 === 1 ? <strong key={index}>{part}</strong> : part));
}

type OverviewNoticeKind = 'limited' | 'unavailable' | 'empty';

export function OverviewDataNotice({ kind, title, description, limitations = [] }: {
  kind: OverviewNoticeKind;
  title: string;
  description: string;
  limitations?: readonly DashboardOverviewLimitation[];
}) {
  return <div aria-label={title} className={`overview-data-notice overview-data-notice-${kind}`} role="status">
    <span aria-hidden="true" className="overview-data-notice-icon">{kind === 'empty' ? '○' : '!'}</span>
    <div><strong>{title}</strong><p>{description}</p>{limitations.map((item) => <small key={`${item.provider}-${item.code}`}>{item.provider}：{item.message}</small>)}</div>
  </div>;
}

export function OverviewMetricCard({ label, value, note, tone, href }: {
  label: string;
  value: number | null;
  note: string;
  tone?: 'blue' | 'green' | 'violet' | 'orange';
  href?: string;
}) {
  const card = <article aria-label={label} className={`overview-metric overview-metric-${tone ?? 'blue'}${value === null ? ' overview-metric-missing' : ''}`}>
    <span aria-hidden="true" className="overview-metric-icon" />
    <div className="overview-metric-copy"><span>{label}</span><strong>{metricValue(value)}</strong><small>{note}</small></div>
    {value === null && <em>数据暂缺</em>}
  </article>;
  return href === undefined ? card : <Link aria-label={`查看${label}详情`} className="overview-metric-link" to={href}>{card}</Link>;
}

export function OverviewModuleHeader({ title, description, extra, headingId }: {
  title: string;
  description?: string;
  extra?: ReactNode;
  headingId: string;
}) {
  return <header className="overview-module-header"><div><h2 id={headingId}>{title}</h2>{description && <p>{description}</p>}</div>{extra && <div className="overview-module-extra">{extra}</div>}</header>;
}

export function OverviewEmptyState({ title, description, action }: { title: string; description: string; action?: ReactNode }) {
  return <div className="overview-empty-visual" role="status"><span aria-hidden="true" className="overview-empty-symbol" /><strong>{title}</strong><p>{description}</p>{action}</div>;
}

export function OverviewAISummary({ insight }: { insight?: DashboardOverviewAIInsight | undefined }) {
  if (insight?.capability !== 'ready' || insight.summary === '') {
    return <OverviewEmptyState action={<Link className="overview-link-button" to="/ai-setting/ai-knowledge-base">前往 AI 设置</Link>} description="配置 AI Provider 和知识库后，这里会展示基于真实会话生成的经营建议。" title="AI 能力尚未接入" />;
  }
  return <div className="overview-ai-summary-legacy"><div>{parseAISummary(insight.summary).map((line, index) => {
    if (line.kind === 'divider') return <hr key={index} />;
    if (line.kind === 'numbered') return <p key={index}>{line.index}. {renderInline(line.text)}</p>;
    if (line.kind === 'bullet') return <p key={index}>{renderInline(line.text)}</p>;
    if (line.kind === 'section' || line.kind === 'paragraph') return <p key={index}>{renderInline(line.text)}</p>;
    return null;
  })}</div><Link className="overview-link-button" to="/ai-insight/smart-analysis">查看 AI 洞察</Link></div>;
}

type AITile = { key: keyof DashboardOverviewAIMetrics; label: string; mark: string; href: string };
const aiTiles: readonly AITile[] = [
  { key: 'analysisCount', label: '已分析会话', mark: '✦', href: '/ai-insight/session-analysis' },
  { key: 'customerNegativeEmotion', label: '负向客户会话', mark: '◉', href: '/ai-insight/emotion?emotion=negative' },
  { key: 'averageEmployeeScore', label: '平均员工评分', mark: '◎', href: '/ai-insight/employee-score' },
  { key: 'keywordCount', label: '关键词总数', mark: '◇', href: '/ai-insight/communication-keyword' },
  { key: 'analyzedEmployeeCount', label: '已覆盖员工', mark: '♙', href: '/ai-insight/employee-score' },
  { key: 'analyzedCustomerCount', label: '已覆盖客户', mark: '♙', href: '/ai-insight/emotion' },
];

export function OverviewAIInsightGrid({ insight: _insight, metrics, limitations = [] }: { insight?: DashboardOverviewAIInsight | undefined; metrics?: DashboardOverviewAIMetrics | undefined; limitations?: readonly DashboardOverviewLimitation[] | undefined }) {
  const insightLimitations = limitations.filter((item) => item.provider === 'ai_insight');
  return <div className="overview-ai-insight-grid">
    {aiTiles.map((tile) => {
      const value = metrics?.[tile.key] ?? null;
      return <Link aria-label={`查看${tile.label}详情`} className="overview-ai-insight-card-link" key={tile.key} to={tile.href}>
        <article aria-label={tile.label} className={`overview-ai-insight-card${value === null ? ' overview-ai-insight-card-missing' : ''}`}>
          <div className="overview-ai-insight-card-top"><span>{tile.label}</span><i aria-hidden="true">{tile.mark}</i></div>
          <strong>{metricValue(value)}</strong>
          <small>{value === null ? '当前时间窗无可评分结果' : '同 AI 洞察页面口径'}</small>
        </article>
      </Link>;
    })}
    {insightLimitations.length > 0 && <OverviewDataNotice kind="limited" title="AI 洞察数据暂缺" description="会话洞察持久化结果当前不可用，未以其他数据源替代。" limitations={insightLimitations} />}
  </div>;
}

function trendMaximum(points: readonly DashboardOverviewTrendPoint[]): number {
  return Math.max(1, ...points.map((point) => point.addCustomerNum));
}

export function OverviewTrendChart({ points }: { points: readonly DashboardOverviewTrendPoint[] }) {
  const maximum = trendMaximum(points);
  return <div aria-label="企业客户增长趋势" className="dashboard-overview-chart" role="img">
    {points.map((point) => <div className="dashboard-overview-chart-column" key={point.date}>
      <span className="dashboard-overview-bar-value">{point.addCustomerNum}</span>
      <div className="dashboard-overview-bars"><span aria-label={`新增客户 ${point.addCustomerNum}`} className="dashboard-overview-bar dashboard-overview-bar-primary" style={{ height: `${Math.max(4, (point.addCustomerNum / maximum) * 100)}%` }} /></div>
      <span className="dashboard-overview-chart-date">{point.date}</span>
    </div>)}
  </div>;
}

type ConversationKind = 'customer' | 'room';
function conversationKindLabel(kind: ConversationKind): string { return kind === 'room' ? '客户群' : '客户会话'; }
function conversationValue(kind: ConversationKind, point: ConversationTrendPoint): number | null { return kind === 'room' ? point.roomSessions : point.customerSessions; }

function ConversationChart({ kind, points }: { kind: ConversationKind; points: readonly ConversationTrendPoint[] }) {
  const values = points.map((point) => conversationValue(kind, point)).filter((value): value is number => value !== null);
  const maximum = Math.max(1, ...values);
  return <div aria-label={`近七日${conversationKindLabel(kind)}趋势`} className="dashboard-overview-chart" role="img">
    {points.map((point) => {
      const value = conversationValue(kind, point);
      return <div className="dashboard-overview-chart-column" key={point.date}>
        <span className="dashboard-overview-bar-value">{metricValue(value)}</span>
        {value === null ? <div aria-label="会话数暂缺" className="dashboard-overview-bars overview-dashboard-bars-missing">--</div> : <div className="dashboard-overview-bars"><span aria-label={`会话数 ${value}`} className="dashboard-overview-bar dashboard-overview-bar-primary" style={{ height: `${Math.max(4, (value / maximum) * 100)}%` }} /></div>}
        <span className="dashboard-overview-chart-date">{point.date.slice(5)}</span>
      </div>;
    })}
  </div>;
}

function ConversationMetricGroup({ href, label, stats }: { href: string; label: string; stats: DashboardOverviewConversation['customer'] }) {
  return <div className="overview-conversation-stat-group"><div className="overview-conversation-stat-heading"><strong>{label}</strong><Link aria-label={`${label}详情`} className="overview-inline-link" to={href}>详情 ↗</Link></div><div className="overview-conversation-group-stats"><span><strong>{metricValue(stats.sessions)}</strong><small>会话数</small></span><span><strong>{metricValue(stats.employeeMessages)}</strong><small>员工消息数</small></span><span><strong>{metricValue(stats.customerMessages)}</strong><small>客户消息数</small></span></div></div>;
}

export function OverviewConversationWorkspace({ conversation, unavailable, limitations = [] }: { conversation?: DashboardOverviewConversation | undefined; unavailable: boolean; limitations?: readonly DashboardOverviewLimitation[] | undefined }) {
  const [kind, setKind] = useState<ConversationKind>('customer');
  if (unavailable || conversation === undefined) return <OverviewDataNotice description="完成企业微信会话存档配置后，这里会展示客户与客户群的真实消息趋势。" kind="unavailable" limitations={limitations} title="会话归档尚未接入" />;
  const selectedStats = conversation[kind];
  const summaryMissing = Object.values(conversation.customer).some((value) => value === null) || Object.values(conversation.room).some((value) => value === null);
  return <div className="overview-conversation">
    <div className="overview-conversation-control-panel">
      <div className="overview-conversation-control-heading"><h3>会话数据</h3><span>数据来源：会话归档</span></div>
      <div aria-label="会话类型" className="overview-conversation-toggle" role="group">{(['customer', 'room'] as const).map((candidate) => <button aria-pressed={kind === candidate} className={kind === candidate ? 'is-active' : ''} key={candidate} onClick={() => setKind(candidate)} type="button">{conversationKindLabel(candidate)}</button>)}</div>
      <ConversationMetricGroup href="/chat/v2-customer" label="客户会话" stats={conversation.customer} />
      <ConversationMetricGroup href="/chat/v2-group" label="客户群" stats={conversation.room} />
      {summaryMissing && <OverviewDataNotice description="接口未返回全部会话汇总字段，缺失字段保持为 —。" kind="limited" title="会话汇总数据暂缺" />}
    </div>
    <div className="overview-conversation-chart"><div className="overview-conversation-chart-title"><h3>{conversationKindLabel(kind)}趋势</h3><span>近七日 · 会话数</span></div>{conversation.trend.length === 0 ? <OverviewDataNotice description="当前周期内没有可展示的会话记录。" kind="empty" title="暂无会话趋势" /> : selectedStats.sessions === null ? <OverviewDataNotice description="当前会话类型的汇总字段缺失，暂不绘制趋势。" kind="limited" title="会话数据暂缺" /> : <ConversationChart kind={kind} points={conversation.trend} />}</div>
  </div>;
}

function qualityValue(value: number | null | undefined): number | null { return value === undefined ? null : value; }

type QualityTrendKey = 'sensitiveWords' | 'riskBehavior' | 'timeoutWarning' | 'customerLoss';
const qualityTrendSeries: readonly { key: QualityTrendKey; label: string; tone: string }[] = [
  { key: 'sensitiveWords', label: '敏感词', tone: 'sensitive' },
  { key: 'riskBehavior', label: '风险行为', tone: 'risk' },
  { key: 'timeoutWarning', label: '超时预警', tone: 'timeout' },
  { key: 'customerLoss', label: '客户流失', tone: 'loss' },
];

function QualityTrendChart({ points }: { points: DashboardOverviewQuality['trend'] }) {
  const values = points.flatMap((point) => qualityTrendSeries.map((series) => point[series.key]))
    .filter((value): value is number => value !== null);
  const maximum = Math.max(1, ...values);
  return <>
    <div aria-label="质检趋势图例" className="overview-quality-trend-legend">
      {qualityTrendSeries.map((series) => <span key={series.key}><i className={`overview-quality-trend-dot overview-quality-trend-${series.tone}`} />{series.label}</span>)}
    </div>
    <div aria-label="近七日质检趋势" className="overview-quality-trend" role="img">
      {points.map((point) => <div className="overview-quality-trend-column" key={point.date}>
        <div className="overview-quality-trend-bars">
          {qualityTrendSeries.map((series) => {
            const value = point[series.key];
            const shortDate = point.date.slice(5);
            return <span
              aria-label={`${shortDate} ${series.label} ${metricValue(value)}`}
              className={`overview-quality-trend-bar overview-quality-trend-${series.tone}${value === null ? ' is-missing' : value === 0 ? ' is-zero' : ''}`}
              key={series.key}
              style={value === null ? undefined : { height: `${value === 0 ? 3 : Math.max(10, (value / maximum) * 100)}%` }}
              title={`${shortDate} ${series.label}：${metricValue(value)}`}
            />;
          })}
        </div>
        <span className="overview-quality-trend-date">{point.date.slice(5)}</span>
      </div>)}
    </div>
  </>;
}

export function OverviewQualityPanel({ quality, limitations = [] }: { quality?: DashboardOverviewQuality | undefined; limitations?: readonly DashboardOverviewLimitation[] | undefined }) {
  const items = [
    { key: 'sensitiveWords', label: '敏感词命中', href: '/ai-insight/v2/sensitive-word' },
    { key: 'riskBehavior', label: '风险行为', href: '/ai-insight/v2/risk' },
    { key: 'timeoutWarning', label: '超时预警', href: '/ai-insight/v2/timeout' },
    { key: 'customerLoss', label: '客户流失', href: '/ai-insight/v2/customer-loss' },
  ] as const;
  const missing = items.some((item) => qualityValue(quality?.[item.key]) === null);
  const trendAvailable = quality !== undefined && quality.trend.length > 0
    && quality.trend.some((point) => qualityTrendSeries.some((series) => point[series.key] !== null));
  return <section aria-label="质检数据" className="overview-module overview-quality-panel dashboard-data-card"><OverviewModuleHeader description="风险、敏感词、超时和客户流失" extra={<span className="overview-scope-chip">当前区间</span>} headingId="overview-quality-title" title="质检数据" /><div className="overview-quality-workspace"><div className="overview-quality-controls"><div className="overview-quality-controls-title"><h3>风险监控</h3><Link aria-label="查看风险监控详情" className="overview-inline-link" to="/ai-insight/v2/risk">详情 ↗</Link></div><div className="overview-quality-grid">{items.map((item) => <OverviewMetricCard href={item.href} key={item.key} label={item.label} note={qualityValue(quality?.[item.key]) === null ? '等待真实数据' : '系统真实数据'} tone="blue" value={qualityValue(quality?.[item.key])} />)}</div>{missing && <OverviewDataNotice description="概览已接入可用数据；仍为 — 的项目表示对应表、权限或 Provider 尚未返回。" kind="limited" limitations={limitations.filter((item) => ['risk_behavior', 'sensitive_word_monitor', 'timeout_warning', 'customer_lifecycle'].includes(item.provider))} title="部分质检数据暂缺" />}</div><div className="overview-quality-visual"><div className="overview-quality-visual-title"><h3>风险监控趋势</h3><span>近七日 · 条</span></div>{trendAvailable ? <QualityTrendChart points={quality.trend} /> : <OverviewDataNotice description="概览接口未返回可用的逐日质检统计，不能以区间合计数伪造趋势。" kind="limited" title="质检趋势数据暂缺" />}</div></div></section>;
}

export function OverviewEmployeeRanking({ items }: { items: readonly DashboardOverviewEmployeeRankingItem[] }) {
  return <section aria-label="员工会话数据排行" className="overview-module overview-ranking-panel dashboard-data-card"><OverviewModuleHeader description="按员工汇总会话表现" extra={<Link aria-label="查看员工会话详情" className="overview-link-button" to="/chat/v2-staff">查看详情 ↗</Link>} headingId="overview-ranking-title" title="员工会话数据排行" /><div className="overview-ranking-body">{items.length === 0 ? <OverviewDataNotice description="当前企业还没有员工会话排行数据。" kind="empty" title="暂无员工会话排行" /> : <div className="overview-ranking-list">{items.map((item, index) => <div className="overview-ranking-row" key={`${item.employeeId}-${index}`}><span className="overview-ranking-index">{index + 1}</span><strong>{item.employeeName || `员工 ${item.employeeId}`}</strong><span><b>{item.sessions}</b> 会话</span><span><b>{item.messages}</b> 消息</span></div>)}</div>}</div></section>;
}

export function OverviewTrajectory({ items, onRefresh, refreshing = false }: { items: readonly DashboardOverviewTrajectoryItem[]; onRefresh?: () => void; refreshing?: boolean }) {
  return <section aria-label="员工会话轨迹一览" className="overview-module overview-trajectory-panel dashboard-data-card"><OverviewModuleHeader description="最近会话与消息轨迹" extra={<div className="overview-module-actions"><Link aria-label="查看会话轨迹详情" className="overview-link-button" to="/chat/trajectory">详情 ↗</Link><button aria-label="刷新员工会话轨迹" className="overview-link-button overview-link-button-secondary" disabled={refreshing || onRefresh === undefined} onClick={onRefresh} type="button">{refreshing ? '刷新中…' : '刷新'}</button></div>} headingId="overview-trajectory-title" title="员工会话轨迹一览" /><div className="overview-trajectory-body">{items.length === 0 ? <OverviewDataNotice description="当前企业还没有可展示的会话轨迹。" kind="empty" title="暂无会话轨迹" /> : <div className="overview-trajectory-list">{items.map((item) => <div className="overview-trajectory-row" key={item.id}><span className={`overview-trajectory-dot overview-trajectory-dot-${item.targetType}`} /><div><strong>{item.targetType === 'room' ? '客户群' : '客户会话'} · {item.targetId}</strong><small>{item.employeeName || '员工信息暂缺'} · {item.messageCount} 条消息</small></div><time>{item.latestAt || '--'}</time></div>)}</div>}</div></section>;
}

export function OverviewCapabilityPanel({ title, description, source, items }: { title: string; description: string; source: string; items: readonly { label: string; note?: string }[] }) {
  return <section aria-label={title} className="overview-module overview-capability-panel dashboard-data-card"><OverviewModuleHeader description={description} headingId={`overview-capability-${title}`} title={title} /><div className="overview-capability-grid">{items.map((item) => <article aria-label={item.label} className="overview-capability-item" key={item.label}><span>{item.label}</span><strong>--</strong><small>{item.note ?? '等待真实数据'}</small></article>)}</div><OverviewDataNotice description={`当前概览接口没有返回该模块字段。数据来源：${source}。`} kind="unavailable" title="能力未接入" /></section>;
}
