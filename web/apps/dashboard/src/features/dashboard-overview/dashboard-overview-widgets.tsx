import { useState } from 'react';
import type { ReactNode } from 'react';
import { Link } from 'react-router';

import type {
  ConversationTrendPoint,
  DashboardOverviewAIInsight,
  DashboardOverviewConversation,
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

export function OverviewMetricCard({ label, value, note, tone }: {
  label: string;
  value: number;
  note: string;
  tone?: 'blue' | 'green' | 'violet' | 'orange';
}) {
  return <article aria-label={label} className={`overview-metric overview-metric-${tone ?? 'blue'}`}>
    <span aria-hidden="true" className="overview-metric-icon" />
    <div className="overview-metric-copy">
      <span>{label}</span>
      <strong>{value.toLocaleString('zh-CN')}</strong>
      <small>{note}</small>
    </div>
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
      <span>{point.date.slice(5)}</span>
    </div>)}
  </div>;
}

type ConversationKind = 'customer' | 'room';

function conversationKindLabel(kind: ConversationKind): string {
  return kind === 'room' ? '客户群' : '客户会话';
}

function conversationValue(kind: ConversationKind, point: ConversationTrendPoint): number {
  return kind === 'customer' ? point.customerSessions : point.roomSessions;
}

function ConversationChart({ kind, points }: {
  kind: ConversationKind;
  points: readonly ConversationTrendPoint[];
}) {
  const maximum = Math.max(1, ...points.map((point) => conversationValue(kind, point)));
  return <div aria-label={`近七日${conversationKindLabel(kind)}趋势`} className="dashboard-overview-chart" role="img">
    {points.map((point) => {
      const value = conversationValue(kind, point);
      return <div className="dashboard-overview-chart-column" key={point.date}>
        <span className="dashboard-overview-bar-value">{value}</span>
        <div className="dashboard-overview-bars"><span
          aria-label={`会话数 ${value}`}
          className="dashboard-overview-bar dashboard-overview-bar-primary"
          style={{ height: `${Math.max(4, (value / maximum) * 100)}%` }}
        /></div>
        <span>{point.date.slice(5)}</span>
      </div>;
    })}
  </div>;
}

export function OverviewConversationWorkspace({ conversation, unavailable }: {
  conversation?: DashboardOverviewConversation | undefined;
  unavailable: boolean;
}) {
  const [kind, setKind] = useState<ConversationKind>('customer');
  if (unavailable || conversation === undefined) {
    return <OverviewEmptyState
      description="完成企业微信会话存档配置后，这里会展示客户与客户群的真实消息趋势。"
      title="会话归档尚未接入"
    />;
  }
  return <div className="overview-conversation">
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
            <span><strong>{stats.sessions.toLocaleString('zh-CN')}</strong><small>会话数</small></span>
            <span><strong>{stats.customerMessages.toLocaleString('zh-CN')}</strong><small>客户消息</small></span>
            <span><strong>{stats.employeeMessages.toLocaleString('zh-CN')}</strong><small>员工消息</small></span>
          </div>
        </button>;
      })}
    </div>
    <div className="overview-conversation-chart">
      <div className="overview-conversation-chart-title"><h3>{conversationKindLabel(kind)}趋势</h3><span>近七日</span></div>
      {conversation.trend.length === 0
        ? <OverviewEmptyState description="当前周期内没有可展示的会话记录。" title="暂无会话趋势" />
        : <ConversationChart kind={kind} points={conversation.trend} />}
    </div>
    <div aria-label="近七日会话趋势明细" className="overview-conversation-detail">
      <h3>{conversationKindLabel(kind)} · 近七日趋势明细</h3>
      {conversation.trend.length === 0
        ? <OverviewEmptyState description="当前周期内没有可展示的会话明细。" title="暂无趋势明细" />
        : <div className="dashboard-table-scroll"><table><thead><tr><th>日期</th><th>会话数</th><th>员工消息数</th><th>客户消息数</th></tr></thead><tbody>
          {conversation.trend.map((point) => <tr key={point.date}>
            <td>{point.date}</td>
            <td>{kind === 'customer' ? point.customerSessions : point.roomSessions}</td>
            <td>{kind === 'customer' ? point.customerEmployeeMessages : point.roomEmployeeMessages}</td>
            <td>{kind === 'customer' ? point.customerCustomerMessages : point.roomCustomerMessages}</td>
          </tr>)}
        </tbody></table></div>}
    </div>
  </div>;
}
