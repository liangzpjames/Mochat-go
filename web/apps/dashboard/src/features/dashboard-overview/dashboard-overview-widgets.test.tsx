import { cleanup, fireEvent, render, screen, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { afterEach, describe, expect, it } from 'vitest';

import {
  OverviewAISummary,
  OverviewConversationWorkspace,
  OverviewMetricCard,
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

  it('switches the conversation summary, chart and table together', () => {
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

  it('shows an explicit empty state for conversation detail rows', () => {
    render(<OverviewConversationWorkspace conversation={{ ...conversation, trend: [] }} unavailable={false} />);
    expect(screen.getByText('暂无趋势明细')).toBeTruthy();
    expect(screen.queryByRole('columnheader', { name: '日期' })).toBeNull();
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
