import { cleanup, fireEvent, render, screen, within } from '@testing-library/react';
import { MemoryRouter, useLocation } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';

import {
  OverviewAISummary,
  OverviewAIInsightGrid,
  OverviewCapabilityPanel,
  OverviewConversationWorkspace,
  OverviewEmployeeRanking,
  OverviewQualityPanel,
  OverviewTrajectory,
  OverviewMetricCard,
  OverviewDataNotice,
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

function CurrentPath() {
  return <output aria-label="当前测试路径">{useLocation().pathname}</output>;
}

it('does not expose raw limitation details in overview notices', () => {
  render(<OverviewDataNotice kind="limited" title="部分数据暂缺" description="当前仅展示已获取的数据。" limitations={[{ provider: 'risk_provider', code: 'provider_unavailable', message: 'Provider 未提供' }]} />);
  expect(screen.getByText('部分数据暂未同步完整，请稍后重试。')).toBeTruthy();
  expect(screen.queryByText(/risk_provider|Provider/i)).toBeNull();
});

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
    render(<MemoryRouter><OverviewConversationWorkspace
      conversation={{ ...conversation, customer: { ...conversation.customer, employeeMessages: null } }}
      unavailable={false}
    /></MemoryRouter>);
    expect(screen.getByRole('status', { name: '会话汇总数据暂缺' })).toBeTruthy();
    expect(screen.getAllByText('--').length).toBeGreaterThan(0);
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

  it('assembles AI insight into numeric controls without rendering the long summary', () => {
    render(<MemoryRouter><OverviewAIInsightGrid
      limitations={[]}
      metrics={{ analysisCount: 5, customerNegativeEmotion: 2, averageEmployeeScore: 86.5, keywordCount: 12, analyzedEmployeeCount: 3, analyzedCustomerCount: 4 }}
    /></MemoryRouter>);
    expect(screen.getByRole('article', { name: '已分析会话' }).textContent).toContain('5');
    expect(screen.getByRole('article', { name: '负向客户会话' }).textContent).toContain('2');
    expect(screen.getByRole('article', { name: '平均员工评分' }).textContent).toContain('86.5');
    expect(screen.getByRole('article', { name: '关键词总数' }).textContent).toContain('12');
    expect(screen.getByRole('article', { name: '已覆盖员工' }).textContent).toContain('3');
    expect(screen.getByRole('article', { name: '已覆盖客户' }).textContent).toContain('4');
    expect(screen.getByRole('link', { name: '查看负向客户会话详情' }).getAttribute('href')).toBe('/ai-insight/emotion?emotion=negative');
    expect(screen.queryByRole('status', { name: 'AI 洞察数据暂缺' })).toBeNull();
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

  it('renders real quality numbers and keeps unavailable AI metrics as gaps', () => {
    render(<MemoryRouter><OverviewQualityPanel
      quality={{
        sensitiveWords: 2, riskBehavior: 4, customerLoss: 0, timeoutWarning: 3,
        trend: [{ date: '2026-08-15', sensitiveWords: 1, riskBehavior: 2, customerLoss: 0, timeoutWarning: 2 }],
      }}
      limitations={[]}
    /></MemoryRouter>);
    expect(screen.getByRole('article', { name: '敏感词命中' }).textContent).toContain('2');
    expect(screen.getByRole('article', { name: '客户流失' }).textContent).toContain('0');
    expect(screen.getByRole('article', { name: '超时预警' }).textContent).toContain('3');
    expect(screen.getByRole('img', { name: '近七日质检趋势' })).toBeTruthy();
    expect(screen.getByLabelText('08-15 风险行为 2')).toBeTruthy();
    expect(screen.getByLabelText('08-15 客户流失 0')).toBeTruthy();
  });

  it('makes every quality metric card open its matching detail page', () => {
    render(<MemoryRouter><>
      <OverviewQualityPanel
        quality={{ sensitiveWords: 2, riskBehavior: 4, customerLoss: 0, timeoutWarning: 3, trend: [] }}
        limitations={[]}
      />
      <CurrentPath />
    </></MemoryRouter>);

    const cases = [
      ['查看敏感词命中详情', '/ai-insight/v2/sensitive-word'],
      ['查看风险行为详情', '/ai-insight/v2/risk'],
      ['查看超时预警详情', '/ai-insight/v2/timeout'],
      ['查看客户流失详情', '/ai-insight/v2/customer-loss'],
    ] as const;
    for (const [name, path] of cases) {
      const link = screen.getByRole('link', { name });
      expect(link.getAttribute('href')).toBe(path);
      fireEvent.click(link);
      expect(screen.getByLabelText('当前测试路径').textContent).toBe(path);
    }
  });

  it('renders employee ranking and trajectory only from returned records', () => {
    const onRefresh = vi.fn();
    render(<MemoryRouter><>
      <OverviewEmployeeRanking items={[{ employeeId: 1001, employeeName: '张伟', sessions: 1, messages: 13 }]} />
      <OverviewTrajectory items={[{ id: 'customer:2001', targetType: 'customer', targetId: '2001', employeeName: '张伟', messageCount: 13, latestAt: '2026-08-16 13:50:08' }]} onRefresh={onRefresh} />
    </></MemoryRouter>);
    expect(screen.getByText('张伟')).toBeTruthy();
    expect(screen.getByText('客户会话 · 2001')).toBeTruthy();
    expect(screen.getByText('13')).toBeTruthy();
    expect(screen.getByRole('link', { name: '查看员工会话详情' }).getAttribute('href')).toBe('/chat/v2-staff');
    expect(screen.getByRole('link', { name: '查看会话轨迹详情' }).getAttribute('href')).toBe('/chat/trajectory');
    expect(screen.getByRole('button', { name: '刷新员工会话轨迹' })).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: '刷新员工会话轨迹' }));
    expect(onRefresh).toHaveBeenCalledTimes(1);
  });

  it('switches the conversation summary and chart together', () => {
    render(<MemoryRouter><OverviewConversationWorkspace conversation={conversation} unavailable={false} /></MemoryRouter>);
    expect(screen.getByText('11')).toBeTruthy();
    expect(screen.getByText('16')).toBeTruthy();
    expect(screen.getByText('22')).toBeTruthy();
    expect(screen.getByRole('img', { name: '近七日客户会话趋势' })).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: /客户群/ }));
    expect(screen.getByRole('img', { name: '近七日客户群趋势' })).toBeTruthy();
    expect(screen.getByLabelText('会话数 2')).toBeTruthy();
    expect(screen.getByRole('link', { name: '客户会话详情' }).getAttribute('href')).toBe('/chat/v2-customer');
    expect(screen.getByRole('link', { name: '客户群详情' }).getAttribute('href')).toBe('/chat/v2-group');
  });

  it('omits the seven-day conversation detail after the trend chart', () => {
    render(<MemoryRouter><OverviewConversationWorkspace conversation={{ ...conversation, trend: [] }} unavailable={false} /></MemoryRouter>);
    expect(screen.queryByLabelText('近七日会话趋势明细')).toBeNull();
    expect(screen.queryByText('近七日趋势明细')).toBeNull();
    expect(screen.queryByRole('columnheader', { name: '日期' })).toBeNull();
  });

  it('keeps the complete date visible beneath growth bars', () => {
    render(<OverviewTrendChart points={[{ date: '2026-08-16', addCustomerNum: 12 }]} />);
    expect(screen.getByText('2026-08-16')).toBeTruthy();
  });

  it('shows a provider limitation instead of zero conversation metrics', () => {
    render(<MemoryRouter><OverviewConversationWorkspace conversation={undefined} unavailable /></MemoryRouter>);
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
