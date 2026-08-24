import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { AiInsightWorkspaceApi } from './ai-insight-workspace-api';
import { SessionAnalysisPage } from './session-analysis-page';

afterEach(cleanup);
beforeEach(() => {
  window.history.replaceState({}, '', '/insight');
  window.sessionStorage.clear();
  vi.clearAllMocks();
});

function createApi() {
  return {
    sessionRecords: vi.fn().mockResolvedValue({ page: 1, pageSize: 20, total: 0, items: [] }),
    sessionStatus: vi.fn().mockResolvedValue({ provider: { state: 'ready' } }),
    sessionDetail: vi.fn(),
    sessionFilterOptions: vi.fn().mockResolvedValue({ employees: [{ id: 1001, name: '张三', avatar: '' }] }),
    sessionExportUrl: vi.fn().mockReturnValue('/export'),
  };
}

describe('会话分析工作台', () => {
  it('使用紧凑查询和空状态，不显示能力说明大卡', async () => { render(<SessionAnalysisPage api={createApi() as unknown as AiInsightWorkspaceApi} />); expect(await screen.findByRole('heading', { name: '会话分析' })).toBeTruthy(); expect(screen.getByRole('button', { name: '查询' })).toBeTruthy(); expect(screen.getByText('当前筛选暂无分析结果')).toBeTruthy(); expect(screen.queryByText('AI 能力未接入')).toBeNull(); });
  it('展示实际消费的系统助手与可用知识文档数', async () => {
    const linkedApi = { sessionRecords: vi.fn().mockResolvedValue({ page: 1, pageSize: 20, total: 0, items: [] }), sessionStatus: vi.fn().mockResolvedValue({ provider: { state: 'ready' }, assistant: { name: '会话分析助手', enabled: true, knowledgeBaseCount: 2, readyDocumentCount: 5, updatedAt: '' } }), sessionDetail: vi.fn(), sessionFilterOptions: vi.fn().mockResolvedValue({ employees: [] }), sessionExportUrl: vi.fn().mockReturnValue('/export') };
    render(<SessionAnalysisPage api={linkedApi as unknown as AiInsightWorkspaceApi} />);
    expect((await screen.findByRole('region', { name: '会话分析助手配置' })).textContent).toContain('2 个已启用知识库、5 份可用文档');
    expect(screen.getByRole('link', { name: '前往配置' }).getAttribute('href')).toBe('/ai-setting/agent');
  });
  it('支持客户名称与员工姓名筛选，查询时 URL 只保存 employeeId', async () => {
    const api = createApi();
    render(<SessionAnalysisPage api={api as unknown as AiInsightWorkspaceApi} />);
    const customerName = await screen.findByLabelText('客户名称');
    const employee = screen.getByRole('combobox', { name: '员工' });
    expect(screen.getByText('结果关键词')).toBeTruthy();
    expect(screen.getByPlaceholderText('搜索结果摘要或结论')).toBeTruthy();
    expect(screen.queryByLabelText('员工 ID')).toBeNull();

    fireEvent.change(customerName, { target: { value: '杭州测试客户' } });
    fireEvent.focus(employee);
    await waitFor(() => expect(api.sessionFilterOptions).toHaveBeenCalledWith(undefined, 20));
    await screen.findAllByRole('option');
    fireEvent.keyDown(employee, { key: 'Enter', code: 'Enter' });
    fireEvent.click(screen.getByRole('button', { name: '查询' }));

    await waitFor(() => expect(api.sessionRecords).toHaveBeenLastCalledWith(expect.objectContaining({
      page: 1,
      customerName: '杭州测试客户',
      employeeId: 1001,
    })));
    expect(window.location.search).toContain('customerName=%E6%9D%AD%E5%B7%9E%E6%B5%8B%E8%AF%95%E5%AE%A2%E6%88%B7');
    expect(window.location.search).toContain('employeeId=1001');
    expect(decodeURIComponent(window.location.search)).not.toContain('张三');
  });
  it('查询和分页写入历史，popstate 时按 URL 恢复筛选并刷新，刷新不新增历史', async () => {
    const api = createApi();
    const pushState = vi.spyOn(window.history, 'pushState');
    const replaceState = vi.spyOn(window.history, 'replaceState');
    render(<SessionAnalysisPage api={api as unknown as AiInsightWorkspaceApi} />);

    fireEvent.change(await screen.findByLabelText('客户名称'), { target: { value: '客户乙' } });
    fireEvent.click(screen.getByRole('button', { name: '查询' }));
    await waitFor(() => expect(pushState).toHaveBeenCalled());

    window.history.pushState({}, '', '/insight?customerName=%E5%AE%A2%E6%88%B7%E7%94%B2&page=2');
    window.dispatchEvent(new PopStateEvent('popstate'));

    await waitFor(() => expect(api.sessionRecords).toHaveBeenLastCalledWith(expect.objectContaining({
      customerName: '客户甲',
      page: 2,
    })));

    const beforeRefresh = pushState.mock.calls.length;
    fireEvent.click(screen.getByRole('button', { name: '刷新' }));
    await waitFor(() => expect(replaceState).toHaveBeenCalled());
    expect(pushState.mock.calls.length).toBe(beforeRefresh);
  });
  it('从 URL 的 employeeId 恢复已知姓名并继续按 ID 请求', async () => {
    const api = createApi();
    window.sessionStorage.setItem('ai-insight.employee-name', JSON.stringify({ '1001': '张三' }));
    window.history.replaceState({}, '', '/insight?employeeId=1001&customerName=%E5%AE%A2%E6%88%B7%E7%94%B2');
    render(<SessionAnalysisPage api={api as unknown as AiInsightWorkspaceApi} />);
    const employee = await screen.findByRole('combobox', { name: '员工' });
    expect((employee as HTMLInputElement).value).toBe('张三');
    await waitFor(() => expect(api.sessionRecords).toHaveBeenCalledWith(expect.objectContaining({
      page: 1,
      employeeId: 1001,
      customerName: '客户甲',
    })));
  });
});
