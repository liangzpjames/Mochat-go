import { cleanup, fireEvent, render, screen, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { afterEach, describe, expect, it } from 'vitest';

import {
  OverviewAISummary,
  OverviewAIInsightGrid,
  OverviewCapabilityPanel,
  OverviewConversationWorkspace,
  OverviewMetricCard,
  OverviewTrendChart,
  parseAISummary,
} from './dashboard-overview-widgets';

const conversation = {
  customer: { sessions: 11, employeeMessages: 22, customerMessages: 16 },
  room: { sessions: 4, employeeMessages: 6, customerMessages: 6 },
  trend: [{
    date: '2026-08-16',
    customerSessions: 7,
    customerEmployeeMessages: 9,
    customerCustomerMessages: 5,
    roomSessions: 2,
    roomEmployeeMessages: 3,
    roomCustomerMessages: 2,
  }],
};

afterEach(cleanup);

describe('dashboard overview widgets', () => {
  it('renders a labelled metric without inventing a change rate', () => {
    render(<OverviewMetricCard label="客户总数" note="同客户分析口径" tone="blue" value={137} />);
    const card = screen.getByRole('article', { name: '客户总数' });
    expect(within(card).getByText('137')).toBeTruthy();
    expect(within(card).queryByText(/%/)).toBeNull();
  });

  it('shows a missing metric as a placeholder instead of a fabricated zero', () => {
    render(<OverviewMetricCard label="客户总数" note="等待真实数据" tone="blue" value={null} />);
    expect(screen.getByRole('article', { name: '客户总数' }).textContent).toContain('--');
    expect(screen.getByText('数据暂缺')).toBeTruthy();
  });

  it('alerts when a conversation summary field is missing', () => {
    render(<OverviewConversationWorkspace
      conversation={{ ...conversation, customer: { ...conversation.customer, employeeMessages: null } }}
      unavailable={false}
    />);
    expect(screen.getByRole('status', { name: '会话汇总数据暂缺' })).toBeTruthy();
    expect(screen.getByRole('button', { name: /客户会话/ }).textContent).toContain('--');
  });

  it('renders structured AI content and a real detail route', () => {
    render(<MemoryRouter><OverviewAISummary insight={{
      capability: 'ready',
      provider: 'dashscope',
      generatedAt: '2026-08-15T23:59:59+08:00',
      summary: '✅ **核心意图**\n1. 立即跟进',
    }} /></MemoryRouter>);
    expect(screen.getByText('核心意图')).toBeTruthy();
    expect(screen.getByRole('link', { name: '查看 AI 洞察' }).getAttribute('href'))
      .toBe('/ai-insight/smart-analysis');
  });

  it('assembles AI insight into compact controls without rendering the long summary', () => {
    render(<MemoryRouter><OverviewAIInsightGrid insight={{
      capability: 'ready',
      provider: 'dashscope',
      generatedAt: '2026-08-15T23:59:59+08:00',
      summary: '✅ **核心客户意图识别**\n1. 商务洽谈与价格协商\n📌 **跟进建议**\n1. 今日内完成响应',
    }} /></MemoryRouter>);
    expect(screen.getByRole('article', { name: '分析状态' })).toBeTruthy();
    expect(screen.getByRole('article', { name: '重点洞察' }).textContent).toContain('商务洽谈与价格协商');
    expect(screen.getByRole('article', { name: '跟进建议' }).textContent).toContain('今日内完成响应');
    expect(screen.queryByText(/核心客户意图识别/)).toBeNull();
  });

  it('shows a clear capability warning for unavailable modules', () => {
    render(<OverviewCapabilityPanel
      description="质检指标需要风险监控数据源。"
      items={[{ label: '敏感词命中' }, { label: '风险行为' }]}
      source="风险监控接口"
      title="质检数据"
    />);
    expect(screen.getByRole('status', { name: '能力未接入' })).toBeTruthy();
    expect(screen.getAllByText('--')).toHaveLength(2);
    expect(screen.getByText(/风险监控接口/)).toBeTruthy();
  });

  it('switches the conversation summary and chart together', () => {
    render(<OverviewConversationWorkspace conversation={conversation} unavailable={false} />);
    const customerCard = screen.getByRole('button', { name: /客户会话/ });
    expect(within(customerCard).getByText('11')).toBeTruthy();
    expect(within(customerCard).getByText('16')).toBeTruthy();
    expect(within(customerCard).getByText('22')).toBeTruthy();
    expect(screen.getByRole('img', { name: '近七日客户会话趋势' })).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: /客户群/ }));
    expect(screen.getByRole('img', { name: '近七日客户群趋势' })).toBeTruthy();
    expect(screen.getByLabelText('会话数 2')).toBeTruthy();
  });

  it('omits the seven-day conversation detail after the trend chart', () => {
    render(<OverviewConversationWorkspace conversation={{ ...conversation, trend: [] }} unavailable={false} />);
    expect(screen.queryByLabelText('近七日会话趋势明细')).toBeNull();
    expect(screen.queryByText('近七日趋势明细')).toBeNull();
    expect(screen.queryByRole('columnheader', { name: '日期' })).toBeNull();
  });

  it('keeps the complete date visible beneath growth bars', () => {
    render(<OverviewTrendChart points={[{ date: '2026-08-16', addCustomerNum: 12 }]} />);
    expect(screen.getByText('2026-08-16')).toBeTruthy();
  });

  it('shows a provider limitation instead of zero conversation metrics', () => {
    render(<OverviewConversationWorkspace conversation={undefined} unavailable />);
    expect(screen.getByText('会话归档尚未接入')).toBeTruthy();
    expect(screen.queryByText('员工消息数')).toBeNull();
  });

  it('parses supported AI summary lines', () => {
    expect(parseAISummary('---\n✅ **标题**\n1. 第一项\n- 子项\n正文')).toEqual([
      { kind: 'divider' },
      { kind: 'section', text: '✅ **标题**' },
      { kind: 'numbered', index: '1', text: '第一项' },
      { kind: 'bullet', text: '子项' },
      { kind: 'paragraph', text: '正文' },
    ]);
  });
});
