import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { InsightDetail, InsightPage, SessionInsightRow, SmartInsightRow } from './ai-insight-workspace-api';
import { AiInsightStatusStrip, InsightDrawer, SessionTable, SmartTable } from './ai-insight-workspace';

afterEach(cleanup);

const failedRow: SessionInsightRow = {
  id: 1,
  conversationKey: '1001:1:2001',
  employee: { id: 1001, name: '员工甲', avatar: '' },
  target: { id: 2001, name: '客户甲', avatar: '', type: 'direct' },
  sourceWindow: { startedAt: '2026-08-21T09:00:00Z', endedAt: '2026-08-21T09:10:00Z', messageCount: 4, fingerprint: 'a'.repeat(64) },
  status: 'failed',
  summary: '',
  errorSummary: '模型响应超时',
  analysisAt: '',
  result: {},
  provider: '',
  model: '',
  promptVersion: 'conversation-v1',
};

describe('AI 洞察失败状态', () => {
  it('会话失败行展示失败标记和原因', () => {
    render(<SessionTable page={{ page: 1, pageSize: 20, total: 1, items: [failedRow] }} onOpen={vi.fn()} />);
    expect(screen.getByText('分析失败')).toBeTruthy();
    expect(screen.getByText('模型响应超时')).toBeTruthy();
    expect(screen.queryByText(/购买意向/)).toBeNull();
  });

  it('智能失败行不显示普通未命中', () => {
    const smart = { ...failedRow, rule: { id: 12, name: '默认智能分析', version: 1 } };
    render(<SmartTable page={{ page: 1, pageSize: 20, total: 1, items: [smart] } as InsightPage<SmartInsightRow>} onOpen={vi.fn()} />);
    expect(screen.getByText('分析失败')).toBeTruthy();
    expect(screen.getByText('模型响应超时')).toBeTruthy();
    expect(screen.queryByText('未命中')).toBeNull();
  });

  it('失败详情展示失败原因', () => {
    const detail = { ...failedRow, messages: [], conversationUrl: '/chat/v2-customer?conversationId=1' } as InsightDetail<SessionInsightRow>;
    render(<InsightDrawer detail={detail} onClose={vi.fn()} />);
    expect(screen.getByText('分析失败')).toBeTruthy();
    expect(screen.getByText('模型响应超时')).toBeTruthy();
    expect(screen.queryByText(/购买意向/)).toBeNull();
  });

  it('没有运行记录时仍展示 Provider 不可用状态', () => {
    render(<AiInsightStatusStrip status={{ provider: { state: 'unavailable', message: '未配置模型凭证' } }} />);
    expect(screen.getByRole('status').textContent).toContain('AI 服务暂不可用');
    expect(screen.getByRole('status').textContent).toContain('未配置模型凭证');
  });
});
