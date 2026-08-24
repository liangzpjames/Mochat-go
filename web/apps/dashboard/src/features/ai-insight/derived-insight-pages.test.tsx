import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import type { ComponentProps } from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { AiInsightWorkspaceApi } from './ai-insight-workspace-api';
import { CommunicationKeywordInsightPage, EmotionInsightPage, EmployeeScoreInsightPage } from './derived-insight-pages';

type ExpectedDerivedPageProps = { api: AiInsightWorkspaceApi; onNavigate?: ((path: string) => void) | undefined };
type Equal<Left, Right> = (<Type>() => Type extends Left ? 1 : 2) extends (<Type>() => Type extends Right ? 1 : 2)
  ? (<Type>() => Type extends Right ? 1 : 2) extends (<Type>() => Type extends Left ? 1 : 2) ? true : false
  : false;
type Assert<Type extends true> = Type;
type _EmotionPageProps = Assert<Equal<ComponentProps<typeof EmotionInsightPage>, ExpectedDerivedPageProps>>;
type _EmployeeScorePageProps = Assert<Equal<ComponentProps<typeof EmployeeScoreInsightPage>, ExpectedDerivedPageProps>>;
type _CommunicationKeywordPageProps = Assert<Equal<ComponentProps<typeof CommunicationKeywordInsightPage>, ExpectedDerivedPageProps>>;
const derivedPagePropsCompile: [_EmotionPageProps, _EmployeeScorePageProps, _CommunicationKeywordPageProps] = [true, true, true];
void derivedPagePropsCompile;

afterEach(cleanup);
beforeEach(() => {
  window.history.replaceState({}, '', '/ai-insight/emotion');
  window.sessionStorage.clear();
  vi.clearAllMocks();
});

const row = {
  id: 9,
  conversationKey: '1001:1:2001',
  employee: { id: 1001, name: '员工甲', avatar: '' },
  target: { type: 'direct', id: '2001', name: '客户甲', avatar: '' },
  sourceWindow: { startedAt: '2026-08-24T09:00:00Z', endedAt: '2026-08-24T09:10:00Z', messageCount: 4, fingerprint: 'a'.repeat(64) },
  status: 'succeeded',
  summary: '客户明确表达采购意向',
  errorSummary: '',
  analysisAt: '2026-08-24T09:11:00Z',
  provider: '',
  model: '',
  promptVersion: '',
  result: {
    schemaVersion: 2,
    summary: '客户明确表达采购意向',
    customer: {
      emotion: { label: 'positive', reason: '客户明确认可方案', evidenceMessageIds: ['m1'] },
      keywords: ['50%_\\采购', '高意向'],
    },
    employeeQa: { score: 0, dimensions: [], strengths: ['响应及时'], issues: [], suggestions: [] },
  },
};

const detail = {
  ...row,
  messages: [{ id: 'm1', time: '2026-08-24T09:01:00Z', direction: 'inbound', senderName: '客户甲', content: '我认可这个方案' }],
  conversationUrl: '/chat/v2-customer?employeeId=1001&conversationId=1001%3A1%3A2001',
};

function createApi(overrides: Record<string, unknown> = {}) {
  return {
    derivedRecords: vi.fn().mockResolvedValue({ page: 1, pageSize: 20, total: 1, items: [row] }),
    derivedStatus: vi.fn().mockResolvedValue({ provider: { state: 'ready' } }),
    derivedDetail: vi.fn().mockResolvedValue(detail),
    derivedFilterOptions: vi.fn().mockResolvedValue({ employees: [{ id: 1001, name: '员工甲', avatar: '' }] }),
    derivedExportUrl: vi.fn().mockReturnValue('/ai-insight/emotion/export'),
    ...overrides,
  };
}

describe('三个 AI 洞察专用投影页面', () => {
  it('三页从 URL 初始化员工、客户、状态和日期 common filters 并请求真实 ID', async () => {
    window.sessionStorage.setItem('ai-insight.employee-name', JSON.stringify({ '1001': '员工甲' }));
    for (const test of [
      { view: 'emotion', path: '/ai-insight/emotion', Page: EmotionInsightPage },
      { view: 'employee-score', path: '/ai-insight/employee-score', Page: EmployeeScoreInsightPage },
      { view: 'communication-keyword', path: '/ai-insight/communication-keyword', Page: CommunicationKeywordInsightPage },
    ] as const) {
      window.history.replaceState({}, '', `${test.path}?employeeId=1001&customerName=%E5%AE%A2%E6%88%B7%E7%94%B2&status=failed&startDate=2026-08-20&endDate=2026-08-24`);
      const api = createApi();
      const Page = test.Page;
      render(<Page api={api as unknown as AiInsightWorkspaceApi} />);
      expect((await screen.findByRole('combobox', { name: '员工' }) as HTMLInputElement).value).toBe('员工甲');
      expect((screen.getByLabelText('客户名称') as HTMLInputElement).value).toBe('客户甲');
      expect((screen.getByLabelText('分析状态') as HTMLSelectElement).value).toBe('failed');
      expect((screen.getByLabelText('开始日期') as HTMLInputElement).value).toBe('2026-08-20');
      expect((screen.getByLabelText('结束日期') as HTMLInputElement).value).toBe('2026-08-24');
      await waitFor(() => expect(api.derivedRecords).toHaveBeenCalledWith(test.view, expect.objectContaining({
        page: 1, employeeId: 1001, customerName: '客户甲', status: 'failed', startDate: '2026-08-20', endDate: '2026-08-24',
      })));
      cleanup();
    }
  });

  it('查询 push、刷新 replace，并在 popstate 时恢复 common filters 后重新请求', async () => {
    window.history.replaceState({}, '', '/ai-insight/emotion?customerName=%E5%AE%A2%E6%88%B7%E7%94%B2');
    const pushState = vi.spyOn(window.history, 'pushState');
    const replaceState = vi.spyOn(window.history, 'replaceState');
    const api = createApi();
    render(<EmotionInsightPage api={api as unknown as AiInsightWorkspaceApi} />);
    expect((await screen.findByLabelText('客户名称') as HTMLInputElement).value).toBe('客户甲');

    fireEvent.change(screen.getByLabelText('客户名称'), { target: { value: '客户乙' } });
    fireEvent.change(screen.getByLabelText('分析状态'), { target: { value: 'succeeded' } });
    fireEvent.click(screen.getByRole('button', { name: '查询' }));
    await waitFor(() => expect(pushState).toHaveBeenCalled());
    expect(new URLSearchParams(window.location.search).get('customerName')).toBe('客户乙');
    expect(new URLSearchParams(window.location.search).get('status')).toBe('succeeded');

    const pushesBeforeRefresh = pushState.mock.calls.length;
    fireEvent.click(screen.getByRole('button', { name: '刷新' }));
    await waitFor(() => expect(replaceState).toHaveBeenCalled());
    expect(pushState.mock.calls.length).toBe(pushesBeforeRefresh);

    window.history.pushState({}, '', '/ai-insight/emotion?customerName=%E5%AE%A2%E6%88%B7%E4%B8%99&status=failed&startDate=2026-08-01&endDate=2026-08-02&page=2');
    window.dispatchEvent(new PopStateEvent('popstate'));
    await waitFor(() => expect(api.derivedRecords).toHaveBeenLastCalledWith('emotion', expect.objectContaining({
      page: 2, customerName: '客户丙', status: 'failed', startDate: '2026-08-01', endDate: '2026-08-02',
    })));
    expect((screen.getByLabelText('客户名称') as HTMLInputElement).value).toBe('客户丙');
  });

  it('情绪页只展示五态客户情绪和真实原因，不制造员工情绪结论', async () => {
    const api = createApi();
    render(<EmotionInsightPage api={api as unknown as AiInsightWorkspaceApi} />);
    expect(await screen.findByRole('heading', { name: '客户情绪洞察' })).toBeTruthy();
    const emotion = screen.getByRole('combobox', { name: '客户情绪' });
    expect(within(emotion).getAllByRole('option').map((option) => option.getAttribute('value'))).toEqual(['', 'positive', 'neutral', 'negative', 'mixed', 'unknown']);
    expect(await screen.findByText('正向')).toBeTruthy();
    expect(screen.getByText('客户明确认可方案')).toBeTruthy();
    expect(screen.queryByText(/员工情绪/)).toBeNull();

    fireEvent.change(emotion, { target: { value: 'negative' } });
    fireEvent.click(screen.getByRole('button', { name: '查询' }));
    await waitFor(() => expect(api.derivedRecords).toHaveBeenLastCalledWith('emotion', expect.objectContaining({ page: 1, emotion: 'negative' })));
  });

  it('员工评分页提供最低最高分并把真实 0 分显示出来', async () => {
    const api = createApi();
    render(<EmployeeScoreInsightPage api={api as unknown as AiInsightWorkspaceApi} />);
    expect(await screen.findByRole('heading', { name: '员工评分洞察' })).toBeTruthy();
    expect(screen.getByText('0 分')).toBeTruthy();
    fireEvent.change(screen.getByLabelText('最低分'), { target: { value: '0' } });
    fireEvent.change(screen.getByLabelText('最高分'), { target: { value: '100' } });
    fireEvent.click(screen.getByRole('button', { name: '查询' }));
    await waitFor(() => expect(api.derivedRecords).toHaveBeenLastCalledWith('employee-score', expect.objectContaining({ page: 1, minScore: 0, maxScore: 100 })));
  });

  it('沟通关键词页按真实 customer keywords 筛选并渲染 tags', async () => {
    const api = createApi();
    render(<CommunicationKeywordInsightPage api={api as unknown as AiInsightWorkspaceApi} />);
    expect(await screen.findByRole('heading', { name: '沟通关键词洞察' })).toBeTruthy();
    expect(screen.getByText('50%_\\采购')).toBeTruthy();
    expect(screen.getByText('高意向')).toBeTruthy();
    fireEvent.change(screen.getByLabelText('沟通关键词'), { target: { value: '采购' } });
    fireEvent.click(screen.getByRole('button', { name: '查询' }));
    await waitFor(() => expect(api.derivedRecords).toHaveBeenLastCalledWith('communication-keyword', expect.objectContaining({ page: 1, keyword: '采购' })));
  });

  it('共享查询支持重置、刷新、固定 20 条分页及总数和当前页摘要', async () => {
    const api = createApi({ derivedRecords: vi.fn().mockResolvedValue({ page: 1, pageSize: 20, total: 21, items: [row] }) });
    render(<EmotionInsightPage api={api as unknown as AiInsightWorkspaceApi} />);
    expect(await screen.findByText('共 21 条，当前页 1 / 2')).toBeTruthy();
    expect(screen.getByText('每页 20 条')).toBeTruthy();

    fireEvent.change(screen.getByRole('combobox', { name: '客户情绪' }), { target: { value: 'mixed' } });
    fireEvent.click(screen.getByRole('button', { name: '查询' }));
    await waitFor(() => expect(api.derivedRecords).toHaveBeenLastCalledWith('emotion', expect.objectContaining({ page: 1, emotion: 'mixed' })));
    fireEvent.click(screen.getByRole('button', { name: '下一页' }));
    await waitFor(() => expect(api.derivedRecords).toHaveBeenLastCalledWith('emotion', expect.objectContaining({ page: 2, emotion: 'mixed' })));
    fireEvent.click(screen.getByRole('button', { name: '重置' }));
    await waitFor(() => expect(api.derivedRecords).toHaveBeenLastCalledWith('emotion', expect.objectContaining({ page: 1 })));
    expect(api.derivedRecords.mock.calls.at(-1)?.[1]).not.toHaveProperty('emotion');
    const beforeRefresh = api.derivedRecords.mock.calls.length;
    fireEvent.click(screen.getByRole('button', { name: '刷新' }));
    await waitFor(() => expect(api.derivedRecords.mock.calls.length).toBe(beforeRefresh + 1));
  });

  it('共享状态诚实覆盖 loading、error/retry、empty、provider unavailable 和 failed row', async () => {
    let reject!: (reason: Error) => void;
    const pending = new Promise<never>((_resolve, rejectPromise) => { reject = rejectPromise; });
    const loadingApi = createApi({ derivedRecords: vi.fn().mockReturnValue(pending) });
    render(<EmotionInsightPage api={loadingApi as unknown as AiInsightWorkspaceApi} />);
    expect(screen.getByText('正在加载洞察数据…')).toBeTruthy();
    reject(new TypeError('Failed to fetch'));
    expect((await screen.findByRole('alert')).textContent).toContain('服务暂时不可用，请稍后刷新重试。');
    fireEvent.click(screen.getByRole('button', { name: '重试' }));
    expect(loadingApi.derivedRecords).toHaveBeenCalledTimes(2);
    cleanup();

    const emptyApi = createApi({
      derivedRecords: vi.fn().mockResolvedValue({ page: 1, pageSize: 20, total: 0, items: [] }),
      derivedStatus: vi.fn().mockResolvedValue({ provider: { state: 'unavailable', message: '尚未配置' } }),
    });
    render(<EmotionInsightPage api={emptyApi as unknown as AiInsightWorkspaceApi} />);
    expect(await screen.findByText('当前筛选暂无洞察结果')).toBeTruthy();
    expect(screen.getByText('AI 服务不可用：尚未配置')).toBeTruthy();
    cleanup();

    const { result: _result, ...failedBase } = row;
    const failedApi = createApi({ derivedRecords: vi.fn().mockResolvedValue({ page: 1, pageSize: 20, total: 1, items: [{ ...failedBase, status: 'failed', errorSummary: '模型响应超时' }] }) });
    render(<EmotionInsightPage api={failedApi as unknown as AiInsightWorkspaceApi} />);
    expect(await screen.findByText('分析失败：模型响应超时')).toBeTruthy();
  });

  it('整行复用 InsightDrawer，关闭按钮、Escape、遮罩均可关闭，原会话按钮不重复打开详情', async () => {
    const onNavigate = vi.fn();
    const api = createApi();
    render(<CommunicationKeywordInsightPage api={api as unknown as AiInsightWorkspaceApi} onNavigate={onNavigate} />);
    const resultRow = await screen.findByRole('row', { name: /员工甲.*客户甲/ });
    fireEvent.click(resultRow);
    expect(await screen.findByRole('dialog', { name: '分析详情' })).toBeTruthy();
    expect(api.derivedDetail).toHaveBeenCalledWith('communication-keyword', 9);
    const callsBeforeNavigate = api.derivedDetail.mock.calls.length;
    fireEvent.click(screen.getByRole('button', { name: '查看原会话' }));
    expect(onNavigate).toHaveBeenCalledWith(detail.conversationUrl);
    expect(api.derivedDetail).toHaveBeenCalledTimes(callsBeforeNavigate);
    fireEvent.click(screen.getByRole('button', { name: '关闭' }));
    expect(screen.queryByRole('dialog')).toBeNull();

    fireEvent.click(resultRow);
    await screen.findByRole('dialog');
    fireEvent.keyDown(window, { key: 'Escape', code: 'Escape' });
    expect(screen.queryByRole('dialog')).toBeNull();

    fireEvent.click(resultRow);
    await screen.findByRole('dialog');
    fireEvent.click(screen.getByLabelText('关闭详情'));
    expect(screen.queryByRole('dialog')).toBeNull();
  });
});
