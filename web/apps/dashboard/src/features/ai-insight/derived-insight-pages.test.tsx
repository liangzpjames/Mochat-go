import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import type { ComponentProps } from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { AiInsightWorkspaceApi } from './ai-insight-workspace-api';
import { CommunicationKeywordInsightPage, EmotionInsightPage, EmployeeScoreInsightPage } from './derived-insight-pages';

type ExpectedDerivedPageProps = { api: AiInsightWorkspaceApi; onNavigate?: ((path: string) => void) | undefined };
type Assert<Type extends true> = Type;
type AcceptsRequiredDerivedPageProps<Props> = Props extends { api: AiInsightWorkspaceApi }
  ? 'onNavigate' extends keyof Props
    ? ExpectedDerivedPageProps extends Props ? true : false
    : false
  : false;
type _EmotionPageProps = Assert<AcceptsRequiredDerivedPageProps<ComponentProps<typeof EmotionInsightPage>>>;
type _EmployeeScorePageProps = Assert<AcceptsRequiredDerivedPageProps<ComponentProps<typeof EmployeeScoreInsightPage>>>;
type _CommunicationKeywordPageProps = Assert<AcceptsRequiredDerivedPageProps<ComponentProps<typeof CommunicationKeywordInsightPage>>>;
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

const secondRow = {
  ...row,
  id: 10,
  conversationKey: '1001:1:2002',
  target: { type: 'direct' as const, id: '2002', name: '客户乙', avatar: '' },
  summary: '客户乙询问交付时间',
};

const secondDetail = {
  ...secondRow,
  messages: [{ id: 'm2', time: '2026-08-24T09:02:00Z', direction: 'inbound' as const, senderName: '客户乙', content: '请问什么时候可以交付' }],
  conversationUrl: '/chat/v2-customer?employeeId=1001&conversationId=1001%3A1%3A2002',
};

function deferred<Value>() {
  let resolve!: (value: Value) => void;
  let reject!: (reason: Error) => void;
  const promise = new Promise<Value>((resolvePromise, rejectPromise) => { resolve = resolvePromise; reject = rejectPromise; });
  return { promise, resolve, reject };
}

function createApi(overrides: Record<string, unknown> = {}) {
  return {
    derivedRecords: vi.fn().mockResolvedValue({ page: 1, pageSize: 20, total: 1, items: [row] }),
    derivedStatus: vi.fn().mockResolvedValue({ provider: { state: 'ready' } }),
    derivedDetail: vi.fn().mockResolvedValue(detail),
    derivedFilterOptions: vi.fn().mockResolvedValue({
      employees: [{ id: 1001, name: '员工甲', avatar: '' }],
      customers: [{ id: 2001, name: '客户甲', avatar: '' }],
      coverage: { availableEmployeeCount: 12, availableCustomerCount: 16, analyzedEmployeeCount: 1, analyzedCustomerCount: 1 },
    }),
    derivedExportUrl: vi.fn().mockReturnValue('/ai-insight/emotion/export'),
    ...overrides,
  };
}

describe('三个 AI 洞察专用投影页面', () => {
  it('展示权威目录覆盖并用客户 ID 精确查询', async () => {
    const api = createApi();
    render(<EmotionInsightPage api={api as unknown as AiInsightWorkspaceApi} />);
    expect(await screen.findByText('可用员工 12 · 已分析 1')).toBeTruthy();
    expect(screen.getByText('可用客户 16 · 已分析 1')).toBeTruthy();

    const customer = screen.getByRole('combobox', { name: '客户' });
    fireEvent.focus(customer);
    fireEvent.change(customer, { target: { value: '客户' } });
    expect(await screen.findByRole('option', { name: /客户甲/ })).toBeTruthy();
    fireEvent.mouseDown(screen.getByRole('option', { name: /客户甲/ }));
    await waitFor(() => expect(screen.getByRole('combobox', { name: '客户' })).toHaveProperty('value', '客户甲'));
    fireEvent.click(screen.getByRole('button', { name: '查询' }));
    await waitFor(() => expect(api.derivedRecords).toHaveBeenLastCalledWith('emotion', expect.objectContaining({ page: 1, customerId: 2001 })));
    expect(new URLSearchParams(window.location.search).get('customerId')).toBe('2001');
  });

  it('通过认证 API 下载当前筛选的 CSV Blob，并使用服务端文件名', async () => {
    let clickedDownload = '';
    let clickedHref = '';
    const createObjectURL = vi.fn().mockReturnValue('blob:authenticated-export');
    const revokeObjectURL = vi.fn();
    Object.defineProperty(URL, 'createObjectURL', { configurable: true, value: createObjectURL });
    Object.defineProperty(URL, 'revokeObjectURL', { configurable: true, value: revokeObjectURL });
    vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function captureDownload(this: HTMLAnchorElement) {
      clickedDownload = this.download;
      clickedHref = this.href;
    });
    window.history.replaceState({}, '', '/ai-insight/emotion?emotion=negative&employeeId=1001');
    const blob = new Blob(['emotion,csv']);
    const api = createApi();
    const downloadExport = vi.fn().mockResolvedValue({ blob, filename: 'emotion-insights.csv' });
    render(<EmotionInsightPage api={api as unknown as AiInsightWorkspaceApi} downloadExport={downloadExport} />);

    fireEvent.click(await screen.findByRole('button', { name: '导出 CSV' }));

    await waitFor(() => expect(downloadExport).toHaveBeenCalledWith('emotion', expect.objectContaining({ page: 1, employeeId: 1001, emotion: 'negative' })));
    await waitFor(() => expect(clickedDownload).toBe('emotion-insights.csv'));
    expect(clickedHref).toBe('blob:authenticated-export');
    expect(createObjectURL).toHaveBeenCalledWith(blob);
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:authenticated-export');
  });

  it('认证导出失败时保留页面并显示可恢复错误', async () => {
    const api = createApi();
    const downloadExport = vi.fn().mockRejectedValue(new Error('导出权限不足'));
    render(<EmotionInsightPage api={api as unknown as AiInsightWorkspaceApi} downloadExport={downloadExport} />);

    fireEvent.click(await screen.findByRole('button', { name: '导出 CSV' }));

    expect((await screen.findByRole('alert')).textContent).toContain('导出权限不足');
    expect(screen.getByText('员工甲')).toBeTruthy();
  });

  it('三页从 URL 初始化员工、客户、状态和日期 common filters 并请求真实 ID', async () => {
    window.sessionStorage.setItem('ai-insight.employee-name', JSON.stringify({ '1001': '员工甲' }));
    for (const test of [
      { view: 'emotion', path: '/ai-insight/emotion', Page: EmotionInsightPage },
      { view: 'employee-score', path: '/ai-insight/employee-score', Page: EmployeeScoreInsightPage },
      { view: 'communication-keyword', path: '/ai-insight/communication-keyword', Page: CommunicationKeywordInsightPage },
    ] as const) {
      window.history.replaceState({}, '', `${test.path}?employeeId=1001&customerId=2001&status=failed&startDate=2026-08-20&endDate=2026-08-24`);
      const api = createApi();
      const Page = test.Page;
      render(<Page api={api as unknown as AiInsightWorkspaceApi} />);
      expect(await screen.findByRole('combobox', { name: '员工' })).toHaveProperty('value', '员工甲');
      await waitFor(() => expect(screen.getByRole('combobox', { name: '客户' })).toHaveProperty('value', '客户甲'));
      expect(screen.getByLabelText('分析状态')).toHaveProperty('value', 'failed');
      expect(screen.getByLabelText('开始日期')).toHaveProperty('value', '2026-08-20');
      expect(screen.getByLabelText('结束日期')).toHaveProperty('value', '2026-08-24');
      await waitFor(() => expect(api.derivedRecords).toHaveBeenCalledWith(test.view, expect.objectContaining({
        page: 1, employeeId: 1001, customerId: 2001, status: 'failed', startDate: '2026-08-20', endDate: '2026-08-24',
      })));
      cleanup();
    }
  });

  it('查询 push、刷新 replace，并在 popstate 时恢复 common filters 后重新请求', async () => {
    window.history.replaceState({}, '', '/ai-insight/emotion?customerId=2001');
    const pushState = vi.spyOn(window.history, 'pushState');
    const replaceState = vi.spyOn(window.history, 'replaceState');
    const api = createApi();
    render(<EmotionInsightPage api={api as unknown as AiInsightWorkspaceApi} />);
    await waitFor(() => expect(screen.getByRole('combobox', { name: '客户' })).toHaveProperty('value', '客户甲'));
    fireEvent.change(screen.getByLabelText('分析状态'), { target: { value: 'succeeded' } });
    fireEvent.click(screen.getByRole('button', { name: '查询' }));
    await waitFor(() => expect(pushState).toHaveBeenCalled());
    expect(new URLSearchParams(window.location.search).get('customerId')).toBe('2001');
    expect(new URLSearchParams(window.location.search).get('status')).toBe('succeeded');

    const pushesBeforeRefresh = pushState.mock.calls.length;
    const replacesBeforeRefresh = replaceState.mock.calls.length;
    fireEvent.click(screen.getByRole('button', { name: '刷新' }));
    await waitFor(() => expect(replaceState.mock.calls.length).toBe(replacesBeforeRefresh + 1));
    expect(pushState.mock.calls.length).toBe(pushesBeforeRefresh);

    window.history.pushState({}, '', '/ai-insight/emotion?customerId=2002&status=failed&startDate=2026-08-01&endDate=2026-08-02&page=2');
    window.dispatchEvent(new PopStateEvent('popstate'));
    await waitFor(() => expect(api.derivedRecords).toHaveBeenLastCalledWith('emotion', expect.objectContaining({
      page: 2, customerId: 2002, status: 'failed', startDate: '2026-08-01', endDate: '2026-08-02',
    })));
    expect(screen.getByRole('combobox', { name: '客户' })).toHaveProperty('value', '');
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
    expect(await screen.findByText('当前目录实体没有符合时间窗的已持久化分析结果')).toBeTruthy();
    expect(screen.getByText('AI 服务不可用：尚未配置')).toBeTruthy();
    cleanup();

    const failedBase = { ...row };
    Reflect.deleteProperty(failedBase, 'result');
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

  it('连续点击两行且第二个详情先返回时，旧响应不能覆盖最后点击的证据', async () => {
    const first = deferred<typeof detail>();
    const second = deferred<typeof secondDetail>();
    const api = createApi({
      derivedRecords: vi.fn().mockResolvedValue({ page: 1, pageSize: 20, total: 2, items: [row, secondRow] }),
      derivedDetail: vi.fn((_view: string, id: number) => id === row.id ? first.promise : second.promise),
    });
    render(<CommunicationKeywordInsightPage api={api as unknown as AiInsightWorkspaceApi} />);
    fireEvent.click(await screen.findByRole('row', { name: /员工甲.*客户甲/ }));
    fireEvent.click(screen.getByRole('row', { name: /员工甲.*客户乙/ }));

    second.resolve(secondDetail);
    expect(await screen.findByText('请问什么时候可以交付')).toBeTruthy();
    first.resolve(detail);
    await Promise.resolve();

    expect(screen.getByText('请问什么时候可以交付')).toBeTruthy();
    expect(screen.queryByText('我认可这个方案')).toBeNull();
  });

  it('旧详情先失败并执行 finally 时，不显示旧错误也不清除最新请求的 loading', async () => {
    const first = deferred<typeof detail>();
    const second = deferred<typeof secondDetail>();
    const api = createApi({
      derivedRecords: vi.fn().mockResolvedValue({ page: 1, pageSize: 20, total: 2, items: [row, secondRow] }),
      derivedDetail: vi.fn((_view: string, id: number) => id === row.id ? first.promise : second.promise),
    });
    render(<CommunicationKeywordInsightPage api={api as unknown as AiInsightWorkspaceApi} />);
    fireEvent.click(await screen.findByRole('row', { name: /员工甲.*客户甲/ }));
    fireEvent.click(screen.getByRole('row', { name: /员工甲.*客户乙/ }));

    first.reject(new Error('旧详情失败'));
    await waitFor(() => expect(screen.queryByText('旧详情失败')).toBeNull());
    expect(screen.getByText('正在打开分析详情…')).toBeTruthy();

    second.resolve(secondDetail);
    expect(await screen.findByText('请问什么时候可以交付')).toBeTruthy();
    expect(screen.queryByText('正在打开分析详情…')).toBeNull();
  });

  it('窄屏卡片为每个隐藏表头的字段保留可读标签', async () => {
    const api = createApi();
    render(<CommunicationKeywordInsightPage api={api as unknown as AiInsightWorkspaceApi} />);
    const resultRow = await screen.findByRole('row', { name: /员工甲.*客户甲/ });
    expect(within(resultRow).getAllByRole('cell').map((cell) => cell.getAttribute('data-label'))).toEqual([
      '沟通员工', '客户 / 群聊', '沟通关键词', '结果摘要', '来源窗口', '分析时间', '操作',
    ]);
  });
});
