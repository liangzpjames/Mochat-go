import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { AiInsightWorkspaceApi } from './ai-insight-workspace-api';
import { SmartAnalysisPage } from './smart-analysis-page';

afterEach(cleanup);
beforeEach(() => {
  window.history.replaceState({}, '', '/insight');
  window.sessionStorage.clear();
  vi.clearAllMocks();
});

function createApi() {
  return {
    smartRecords: vi.fn().mockResolvedValue({ page: 1, pageSize: 20, total: 0, items: [] }),
    smartStatus: vi.fn().mockResolvedValue({
      provider: { state: 'ready' },
      assistant: { name: '智能分析助手', enabled: true, knowledgeBaseCount: 1, readyDocumentCount: 2, updatedAt: '2026-08-24T00:00:00Z' },
    }),
    smartDetail: vi.fn(),
    smartFilterOptions: vi.fn().mockResolvedValue({ employees: [{ id: 1002, name: '李四', avatar: '' }] }),
  };
}

describe('智能分析工作台', () => {
	  it('网络中断时展示可行动的中文反馈', async () => {
	    const api = { ...createApi(), smartRecords: vi.fn().mockRejectedValue(new TypeError('Failed to fetch')) };
	    render(<SmartAnalysisPage api={api as unknown as AiInsightWorkspaceApi} />);
	    expect((await screen.findByRole('alert')).textContent).toBe('服务暂时不可用，请稍后刷新重试。');
	  });
  it('只展示默认助手生成的结果，不再提供规则管理和规则版本筛选', async () => {
    const api = createApi();
    render(<SmartAnalysisPage api={api as unknown as AiInsightWorkspaceApi} />);
    expect(await screen.findByRole('heading', { name: '智能分析' })).toBeTruthy();
    expect((await screen.findByRole('region', { name: '智能分析助手配置' })).textContent).toContain('智能分析助手');
    expect(screen.queryByText('默认智能分析规则')).toBeNull();
    expect(screen.queryByText('默认分析助手')).toBeNull();
    expect(screen.getByRole('link', { name: '前往配置' }).getAttribute('href')).toBe('/ai-setting/agent');
    expect(screen.getByText('当前暂无匹配的智能分析结果')).toBeTruthy();
    expect(screen.queryByRole('tab', { name: '分析规则' })).toBeNull();
    expect(screen.queryByRole('button', { name: '新增规则' })).toBeNull();
    expect(screen.queryByLabelText('规则版本')).toBeNull();
  });
  it('支持客户名称与员工姓名筛选，并展示智能分析助手文案', async () => {
    const api = createApi();
    render(<SmartAnalysisPage api={api as unknown as AiInsightWorkspaceApi} />);
    expect((await screen.findByRole('region', { name: '智能分析助手配置' })).textContent).toContain('智能分析助手');
    expect(screen.queryByText('默认分析助手')).toBeNull();
    expect(screen.queryByText('统一助手')).toBeNull();
    expect(screen.getByPlaceholderText('搜索结果摘要或结论')).toBeTruthy();
    expect(screen.queryByLabelText('员工 ID')).toBeNull();

    fireEvent.change(screen.getByLabelText('客户名称'), { target: { value: '上海客户' } });
    const employee = screen.getByRole('combobox', { name: '员工' });
    fireEvent.focus(employee);
    await waitFor(() => expect(api.smartFilterOptions).toHaveBeenCalledWith(undefined, 20));
    await screen.findAllByRole('option');
    fireEvent.keyDown(employee, { key: 'Enter', code: 'Enter' });
    fireEvent.click(screen.getByRole('button', { name: '查询' }));

    await waitFor(() => expect(api.smartRecords).toHaveBeenLastCalledWith(expect.objectContaining({
      page: 1,
      customerName: '上海客户',
      employeeId: 1002,
    })));
    expect(window.location.search).toContain('customerName=%E4%B8%8A%E6%B5%B7%E5%AE%A2%E6%88%B7');
    expect(window.location.search).toContain('employeeId=1002');
  });
  it('助手名称为空时回退为智能分析助手，且不出现默认措辞', async () => {
    const api = {
      ...createApi(),
      smartStatus: vi.fn().mockResolvedValue({
        provider: { state: 'ready' },
        assistant: { name: '', enabled: true, knowledgeBaseCount: 0, readyDocumentCount: 0, updatedAt: '' },
      }),
    };
    render(<SmartAnalysisPage api={api as unknown as AiInsightWorkspaceApi} />);
    expect((await screen.findByRole('region', { name: '智能分析助手配置' })).textContent).toContain('智能分析助手');
    expect(screen.queryByText('默认')).toBeNull();
  });
});
