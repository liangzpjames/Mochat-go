import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { SmartAnalysisPage } from './smart-analysis-page';

describe('智能分析工作台', () => {
  it('只展示默认助手生成的结果，不再提供规则管理和规则版本筛选', async () => {
    const api = {
      smartRecords: vi.fn().mockResolvedValue({ page: 1, pageSize: 20, total: 0, items: [] }),
      smartStatus: vi.fn().mockResolvedValue({
        provider: { state: 'ready' },
        assistant: { name: '会话分析助手', enabled: true, knowledgeBaseCount: 1, readyDocumentCount: 2, updatedAt: '2026-08-24T00:00:00Z' },
      }),
      smartDetail: vi.fn(),
    } as never;
    render(<SmartAnalysisPage api={api} />);
    expect(await screen.findByRole('heading', { name: '智能分析' })).toBeTruthy();
    expect((await screen.findByRole('region', { name: '智能分析助手配置' })).textContent).toContain('会话分析助手');
    expect(screen.getByRole('link', { name: '前往配置' }).getAttribute('href')).toBe('/ai-setting/agent');
    expect(screen.getByText('当前暂无匹配的智能分析结果')).toBeTruthy();
    expect(screen.queryByRole('tab', { name: '分析规则' })).toBeNull();
    expect(screen.queryByRole('button', { name: '新增规则' })).toBeNull();
    expect(screen.queryByLabelText('规则版本')).toBeNull();
  });
});
