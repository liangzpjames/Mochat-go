import { useState } from 'react';
import type { ReactNode } from 'react';
import { Link } from 'react-router';

import type {
  ConversationTrendPoint,
  DashboardOverviewAIInsight,
  DashboardOverviewConversation,
  DashboardOverviewLimitation,
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
    if (/^[-•]\s+/.test(line)) {
      return { kind: 'bullet' as const, text: line.replace(/^[-•]\s+/, '') };
    }
    return { kind: 'paragraph' as const, text: line };
  }).filter((line): line is AISummaryLine => line !== null);
}

function renderInline(text: string): ReactNode {
  return text.split(/\*\*(.+?)\*\*/g)
    .map((part, index) => (index % 2 === 1 ? <strong key={index}>{part}</strong> : part));
}

function metricValue(value: number | null): string {
  return value === null ? '--' : value.toLocaleString('zh-CN');
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
    <div>
      <strong>{title}</strong>
      <p>{description}</p>
      {limitations.map((item) => <small key={`${item.provider}-${item.code}`}>{item.provider}：{item.message}</small>)}
    </div>
  </div>;
}

export function OverviewMetricCard({ label, value, note, tone }: {
  label: string;
  value: number | null;
  note: string;
  tone?: 'blue' | 'green' | 'violet' | 'orange';
}) {
  return <article aria-label={label} className={`overview-metric overview-metric-${tone ?? 'blue'}${value === null ? ' overview-metric-missing' : ''}`}>
    <span aria-hidden="true" className="overview-metric-icon" />
    <div className="overview-metric-copy">
      <span>{label}</span>
      <strong>{metricValue(value)}</strong>
      <small>{note}</small>
    </div>
    {value === null && <em>数据暂缺</em>}
  </article>;
}

export function OverviewModuleHeader({ title, description, extra, headingId }: {
  title: string;
  description?: string;
  extra?: ReactNode;
  headingId: string;
}) {
  return <header className="overview-module-header">
    <div><h2 id={headingId}>{title}</h2>{description && <p>{description}</p>}</div>
    {extra && <div className="overview-module-extra">{extra}</div>}
  </header>;
}

export function OverviewEmptyState({ title, description, action }: {
  title: string;
  description: string;
  action?: ReactNode;
}) {
  return <div className="overview-empty-visual" role="status">
    <span aria-hidden="true" className="overview-empty-symbol" />
    <strong>{title}</strong>
    <p>{description}</p>
    {action}
  </div>;
}

export function OverviewAISummary({ insight }: { insight?: DashboardOverviewAIInsight | undefined }) {
  if (insight?.capability !== 'ready' || insight.summary === '') {
    return <OverviewEmptyState
      action={<Link className="overview-link-button" to="/ai-setting/ai-knowledge-base">前往 AI 设置</Link>}
      description="配置 AI Provider 和知识库后，这里会展示基于真实会话生成的经营建议。"
      title="AI 能力尚未接入"
    />;
  }
  return <div className="overview-ai-panel">
    <div className="overview-ai-summary">
      {parseAISummary(insight.summary).map((line, index) => {
        if (line.kind === 'divider') return <hr className="overview-ai-summary-divider" key={index} />;
        if (line.kind === 'section') return <p className="overview-ai-summary-section" key={index}>{renderInline(line.text)}</p>;
        if (line.kind === 'numbered') return <p className="overview-ai-summary-item" key={index}><span className="overview-ai-summary-index">{line.index}.</span><span>{renderInline(line.text)}</span></p>;
        if (line.kind === 'bullet') return <p className="overview-ai-summary-bullet" key={index}>{renderInline(line.text)}</p>;
        return <p className="overview-ai-summary-paragraph" key={index}>{renderInline(line.text)}</p>;
      })}
    </div>
    <Link className="overview-link-button" to="/ai-insight/smart-analysis">查看 AI 洞察</Link>
  </div>;
}

function compactInsightText(value: string): string {
  return value.replace(/\*\*/g, '').replace(/^(?:✅|⚠️|📌)\s*/, '').trim();
}

function insightGeneratedAt(value: string): string {
  return value === '' ? '--' : value.replace('T', ' ').replace(/([+-]\d\d:\d\d|Z)$/, '').slice(0, 16);
}

export function OverviewAIInsightGrid({ insight }: { insight?: DashboardOverviewAIInsight | undefined }) {
  if (insight?.capability !== 'ready' || insight.summary.trim() === '') {
    return <OverviewDataNotice
      description="配置 AI Provider 并完成分析后，这里会展示真实的洞察摘要。"
      kind={insight === undefined ? 'unavailable' : 'empty'}
      title={insight === undefined ? '能力未接入' : '暂无 AI 洞察'}
    />;
  }

  const lines = parseAISummary(insight.summary);
  const numbered = lines.filter((line): line is Extract<AISummaryLine, { kind: 'numbered' }> => line.kind === 'numbered');
  const followUpSectionIndex = lines.findIndex((line) => line.kind === 'section' && line.text.includes('跟进'));
  const followUps = followUpSectionIndex >= 0
    ? lines.slice(followUpSectionIndex + 1).filter((line): line is Extract<AISummaryLine, { kind: 'numbered' }> => line.kind === 'numbered')
    : [];
  const insightText = compactInsightText(numbered[0]?.text ?? '摘要已生成，请进入详情查看。');
  const followUpText = compactInsightText(followUps[0]?.text ?? numbered[1]?.text ?? '摘要未提供单独的跟进建议。');
  return <div className="overview-ai-insight-grid">
    <article aria-label="分析状态" className="overview-ai-insight-card overview-ai-insight-card-primary">
      <span>分析状态</span><strong>已生成</strong><small>{insight.provider || '来源暂缺'}</small>
    </article>
    <article aria-label="分析时间" className="overview-ai-insight-card">
      <span>最新分析</span><strong>{insightGeneratedAt(insight.generatedAt)}</strong><small>来自真实 AI 分析记录</small>
    </article>
    <article aria-label="重点洞察" className="overview-ai-insight-card">
      <span>重点洞察</span><strong>{insightText}</strong><small>仅展示摘要中的首条重点</small>
    </article>
    <article aria-label="跟进建议" className="overview-ai-insight-card">
      <span>跟进建议</span><strong>{followUpText}</strong><small>进入详情查看完整建议</small>
    </article>
    <Link className="overview-link-button overview-ai-insight-link" to="/ai-insight/smart-analysis">查看 AI 洞察</Link>
  </div>;
}

export function OverviewCapabilityPanel({ title, description, source, items }: {
  title: string;
  description: string;
  source: string;
  items: readonly { label: string; note?: string }[];
}) {
  return <section aria-label={title} className="overview-module overview-capability-panel dashboard-data-card">
    <OverviewModuleHeader description={description} headingId={`overview-capability-${title}`} title={title} />
    <div className="overview-capability-grid">
      {items.map((item) => <article aria-label={item.label} className="overview-capability-item" key={item.label}>
        <span>{item.label}</span><strong>--</strong><small>{item.note ?? '等待真实数据'}</small>
      </article>)}
    </div>
    <OverviewDataNotice
      description={`当前概览接口没有返回该模块字段。数据来源：${source}；补齐对应数据后这里会自动展示。`}
      kind="unavailable"
      title="能力未接入"
    />
  </section>;
}

function trendMaximum(points: readonly DashboardOverviewTrendPoint[]): number {
  return Math.max(1, ...points.map((point) => point.addCustomerNum));
}

export function OverviewTrendChart({ points }: { points: readonly DashboardOverviewTrendPoint[] }) {
  const maximum = trendMaximum(points);
  return <div aria-label="企业客户增长趋势" className="dashboard-overview-chart" role="img">
    {points.map((point) => <div className="dashboard-overview-chart-column" key={point.date}>
      <span className="dashboard-overview-bar-value">{point.addCustomerNum}</span>
      <div className="dashboard-overview-bars"><span
        aria-label={`新增客户 ${point.addCustomerNum}`}
        className="dashboard-overview-bar dashboard-overview-bar-primary"
        style={{ height: `${Math.max(4, (point.addCustomerNum / maximum) * 100)}%` }}
      /></div>
      <span className="dashboard-overview-chart-date">{point.date}</span>
    </div>)}
  </div>;
}

type ConversationKind = 'customer' | 'room';

function conversationKindLabel(kind: ConversationKind): string {
  return kind === 'room' ? '客户群' : '客户会话';
}

function conversationValue(kind: ConversationKind, point: ConversationTrendPoint): number | null {
  return kind === 'customer' ? point.customerSessions : point.roomSessions;
}

function ConversationChart({ kind, points }: {
  kind: ConversationKind;
  points: readonly ConversationTrendPoint[];
}) {
  const values = points.map((point) => conversationValue(kind, point)).filter((value): value is number => value !== null);
  const maximum = Math.max(1, ...values);
  return <div aria-label={`近七日${conversationKindLabel(kind)}趋势`} className="dashboard-overview-chart" role="img">
    {points.map((point) => {
      const value = conversationValue(kind, point);
      return <div className="dashboard-overview-chart-column" key={point.date}>
        <span className="dashboard-overview-bar-value">{metricValue(value)}</span>
        {value === null
          ? <div aria-label="会话数暂缺" className="dashboard-overview-bars overview-dashboard-bars-missing">--</div>
          : <div className="dashboard-overview-bars"><span
              aria-label={`会话数 ${value}`}
              className="dashboard-overview-bar dashboard-overview-bar-primary"
              style={{ height: `${Math.max(4, (value / maximum) * 100)}%` }}
            /></div>}
        <span>{point.date.slice(5)}</span>
      </div>;
    })}
  </div>;
}

export function OverviewConversationWorkspace({ conversation, unavailable, limitations = [] }: {
  conversation?: DashboardOverviewConversation | undefined;
  unavailable: boolean;
  limitations?: readonly DashboardOverviewLimitation[];
}) {
  const [kind, setKind] = useState<ConversationKind>('customer');
  if (unavailable || conversation === undefined) {
    return <OverviewDataNotice
      description="完成企业微信会话存档配置后，这里会展示客户与客户群的真实消息趋势。"
      kind="unavailable"
      limitations={limitations}
      title="会话归档尚未接入"
    />;
  }
  const conversationSummaryMissing = [
    ...Object.values(conversation.customer),
    ...Object.values(conversation.room),
  ].some((value) => value === null);
  return <div className="overview-conversation">
    {conversationSummaryMissing && <OverviewDataNotice
      description="会话归档接口没有返回全部汇总字段，已保留缺失状态；补齐对应数据后这里会自动展示。"
      kind="limited"
      title="会话汇总数据暂缺"
    />}
    <div aria-label="会话类型" className="overview-conversation-groups" role="group">
      {(['customer', 'room'] as const).map((candidate) => {
        const stats = conversation[candidate];
        return <button
          aria-pressed={kind === candidate}
          className={`overview-conversation-group${kind === candidate ? ' overview-conversation-group-active' : ''}`}
          key={candidate}
          onClick={() => setKind(candidate)}
          type="button"
        >
          <span>{conversationKindLabel(candidate)}</span>
          <div className="overview-conversation-group-stats">
            <span><strong>{metricValue(stats.sessions)}</strong><small>会话数</small></span>
            <span><strong>{metricValue(stats.customerMessages)}</strong><small>客户消息</small></span>
            <span><strong>{metricValue(stats.employeeMessages)}</strong><small>员工消息</small></span>
          </div>
        </button>;
      })}
    </div>
    <div className="overview-conversation-chart">
      <div className="overview-conversation-chart-title"><h3>{conversationKindLabel(kind)}趋势</h3><span>近七日</span></div>
      {conversation.trend.length === 0
        ? <OverviewDataNotice description="当前周期内没有可展示的会话记录。" kind="empty" title="暂无会话趋势" />
        : conversation.trend.some((point) => conversationValue(kind, point) !== null)
          ? <ConversationChart kind={kind} points={conversation.trend} />
          : <OverviewDataNotice description="接口未返回当前会话类型的趋势数值。" kind="limited" title="会话数据暂缺" />}
    </div>
  </div>;
}
