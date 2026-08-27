import { act, cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type {
  EmployeeFilterOptions,
  InsightDetail,
  InsightPage,
  Person,
  SessionInsightRow,
  SmartInsightRow,
} from './ai-insight-workspace-api';
import {
  AiInsightStatusStrip,
  EmployeeSearchField,
  InsightDrawer,
  SessionTable,
  SmartTable,
} from './ai-insight-workspace';

function createDeferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

afterEach(cleanup);
beforeEach(() => {
  window.sessionStorage.clear();
});

const failedRow: SessionInsightRow = {
  id: 1,
  conversationKey: '1001:1:2001',
  employee: { id: 1001, name: '员工甲', avatar: '' },
  target: { id: '2001', name: '客户甲', avatar: '', type: 'direct' },
  sourceWindow: { startedAt: '2026-08-21T09:00:00Z', endedAt: '2026-08-21T09:10:00Z', messageCount: 4, fingerprint: 'a'.repeat(64) },
  status: 'failed',
  summary: '',
  errorSummary: '模型响应超时',
  analysisAt: '',
  result: {},
  provider: 'openai',
  model: 'gpt-5',
  promptVersion: 'conversation-v1',
};

const sessionSuccessRow: SessionInsightRow = {
  ...failedRow,
  id: 2,
  status: 'succeeded',
  summary: '客户表达了强烈采购意向，需尽快报价。',
  errorSummary: '',
  analysisAt: '2026-08-21T09:11:00Z',
  result: {
    schemaVersion: 2,
    customer: {
      qualityScore: 86,
      qualityLevel: 'high',
      qualityReason: '预算与需求都比较清晰。',
      purchaseIntent: {
        score: 92,
        level: 'high',
        reason: '客户明确询价并确认交付周期。',
        evidenceMessageIds: ['m2'],
        dimensions: [
          { name: '预算匹配', score: 88, weight: 0.4, reason: '预算范围清晰。', evidenceMessageIds: ['m2'] },
        ],
      },
      churnRisk: {
        score: null,
        level: 'insufficient',
        reason: '证据不足',
        evidenceMessageIds: [],
        dimensions: [
          { name: '决策时机', score: null, weight: 0.3, reason: '尚未确认决策窗口。', evidenceMessageIds: ['m4'] },
        ],
      },
      keywords: ['报价', '实施周期'],
      explicitNeeds: ['希望 9 月前上线'],
      implicitNeeds: ['需要更明确的实施排期'],
      emotion: {
        label: 'positive',
        reason: '客户持续追问报价与上线时间。',
        evidenceMessageIds: ['m1'],
      },
      recommendedReply: '优先发送正式报价并确认实施窗口。',
      actions: ['24 小时内回访', '补充实施排期'],
      notes: ['避免一次性给出过多折扣'],
    },
    employeeQa: {
      score: 78,
      dimensions: [
        { name: '需求澄清', score: 82, comment: '关键诉求已追问。' },
      ],
      unresolvedCustomerIssues: [
        { title: '交付周期未完全确认', reason: '客户需要更明确的上线时间表。', evidenceMessageIds: ['m1'] },
      ],
      unresolvedObjections: [
        { title: '价格异议未闭环', reason: '客户仍在比较预算区间。', evidenceMessageIds: ['m2', 'm3'] },
      ],
      strengths: ['响应及时'],
      issues: ['报价说明不完整'],
      suggestions: ['补充实施里程碑'],
    },
  },
};

const smartV2Row: SmartInsightRow = {
  ...sessionSuccessRow,
  id: 3,
  summary: '客户近期有较高成交机会，优先级较高。',
  rule: { id: 12, name: '商机识别', version: 3 },
  result: {
    schemaVersion: 2,
    matchScore: 88,
    confidenceScore: 73,
    priorityScore: 0,
    priorityLevel: 'low',
    evidenceCoverageScore: 65,
    conclusion: '建议 48 小时内重点回访。',
    dimensions: [
      { name: '预算信号', score: 82, weight: 0.5, reason: '客户已确认预算区间。', evidenceMessageIds: ['m2'] },
    ],
    recommendations: ['由销售主管跟进', '优先安排演示'],
    evidenceMessageIds: ['m2'],
  },
};

const smartV1Row: SmartInsightRow = {
  ...sessionSuccessRow,
  id: 4,
  summary: '历史记录显示存在初步意向。',
  rule: { id: 13, name: '历史规则', version: 1 },
  result: {
    schemaVersion: 1,
    matched: true,
    confidence: 0.87,
    conclusion: '历史规则判定为潜在商机。',
    evidenceMessageIds: ['m2'],
    recommendations: [],
  },
};

const sessionSuccessResult = sessionSuccessRow.result as Record<string, unknown> & {
  customer: Record<string, unknown> & {
    qualityScore: number | null;
    qualityLevel: string;
    purchaseIntent: Record<string, unknown> & {
      score: number | null;
      level: string;
    };
  };
};

const sessionNullableScoreRow: SessionInsightRow = {
  ...sessionSuccessRow,
  id: 5,
  result: {
    ...sessionSuccessResult,
    customer: {
      ...sessionSuccessResult.customer,
      qualityScore: null,
      qualityLevel: 'high',
      purchaseIntent: {
        ...sessionSuccessResult.customer.purchaseIntent,
        score: null,
        level: 'low',
      },
    },
  },
};

function renderFilterField(overrides: {
  selectedEmployeeId?: number;
  knownEmployeeName?: string;
  loadOptions?: (keyword?: string, limit?: number) => Promise<EmployeeFilterOptions>;
  onSelect?: (employee: Person | undefined) => void;
} = {}) {
  const loadOptions = overrides.loadOptions ?? vi.fn().mockResolvedValue({
    employees: [{ id: 1001, name: '张三', avatar: '' }],
  });
  const onSelect = overrides.onSelect ?? vi.fn();
  render(
    <EmployeeSearchField
      label="员工"
      selectedEmployeeId={overrides.selectedEmployeeId}
      knownEmployeeName={overrides.knownEmployeeName}
      loadOptions={loadOptions}
      onSelect={onSelect}
    />,
  );
  return { loadOptions, onSelect };
}

describe('AI 洞察工作台渲染', () => {
  it('会话结果行突出三项量化指标，null 显示证据不足', () => {
    render(<SessionTable page={{ page: 1, pageSize: 20, total: 1, items: [sessionSuccessRow] }} onOpen={vi.fn()} />);
    expect(screen.getByText('购买意向分')).toBeTruthy();
    expect(screen.getByText('92分')).toBeTruthy();
    expect(screen.getByText('流失风险分')).toBeTruthy();
    expect(screen.getAllByText('证据不足').length).toBeGreaterThan(0);
    expect(screen.getByText('员工质检分')).toBeTruthy();
    expect(screen.getByText('78分')).toBeTruthy();
    expect(screen.getByText('客户表达了强烈采购意向，需尽快报价。')).toBeTruthy();
  });

  it('智能 v2 结果行突出命中度、置信度和优先级', () => {
    render(<SmartTable page={{ page: 1, pageSize: 20, total: 1, items: [smartV2Row] }} onOpen={vi.fn()} />);
    expect(screen.getByText('命中度')).toBeTruthy();
    expect(screen.getByText('88分')).toBeTruthy();
    expect(screen.getByText('置信度')).toBeTruthy();
    expect(screen.getByText('73分')).toBeTruthy();
    expect(screen.getByText('优先级')).toBeTruthy();
    expect(screen.getByText('0分 · 低')).toBeTruthy();
    expect(screen.getByText(/覆盖度 65分/)).toBeTruthy();
    expect(screen.queryByText('low')).toBeNull();
  });

  it('智能 v1 历史结果显示 matched、confidence 百分比和历史说明', () => {
    render(<SmartTable page={{ page: 1, pageSize: 20, total: 1, items: [smartV1Row] } as InsightPage<SmartInsightRow>} onOpen={vi.fn()} />);
    expect(screen.getByText('已命中')).toBeTruthy();
    expect(screen.getByText('87%')).toBeTruthy();
    expect(screen.getByText('历史结果无综合分')).toBeTruthy();
  });

  it('会话详情按客户分析、员工质检和证据消息分区，并高亮证据消息', () => {
    const detail = {
      ...sessionSuccessRow,
      messages: [
        { id: 'm1', time: '2026-08-21T09:01:00Z', direction: 'inbound', senderName: '客户甲', content: '我们 9 月前要上线。' },
        { id: 'm2', time: '2026-08-21T09:03:00Z', direction: 'inbound', senderName: '客户甲', content: '预算大约 20 万。' },
        { id: 'm3', time: '2026-08-21T09:05:00Z', direction: 'outbound', senderName: '员工甲', content: '我稍后补充报价。' },
      ],
      conversationUrl: '/chat/v2-customer?conversationId=2',
    } as InsightDetail<SessionInsightRow>;
    render(<InsightDrawer detail={detail} onClose={vi.fn()} />);
    expect(screen.getByRole('heading', { name: '客户分析' })).toBeTruthy();
    expect(screen.getByRole('heading', { name: '员工质检' })).toBeTruthy();
    expect(screen.getByRole('heading', { name: '证据消息' })).toBeTruthy();
    expect(screen.getByText('质量分 / 等级')).toBeTruthy();
    expect(screen.getByText('86分 · 高')).toBeTruthy();
    expect(screen.getByText('质量分 / 等级')).toBeTruthy();
    expect(screen.getByText('客户情绪')).toBeTruthy();
    expect(screen.getByText('积极')).toBeTruthy();
    expect(screen.queryByText('positive')).toBeNull();
    expect(screen.getByText('客户持续追问报价与上线时间。')).toBeTruthy();
    expect(screen.getByText('客户明确询价并确认交付周期。')).toBeTruthy();
    expect(screen.getByText('尚未确认决策窗口。')).toBeTruthy();
    expect(screen.getByText('预算匹配')).toBeTruthy();
    expect(screen.getByText('决策时机')).toBeTruthy();
    expect(screen.getAllByText('证据不足').length).toBeGreaterThan(0);
    expect(screen.getByText('交付周期未完全确认')).toBeTruthy();
    expect(screen.getByText('客户需要更明确的上线时间表。')).toBeTruthy();
    expect(screen.getByText('价格异议未闭环')).toBeTruthy();
    expect(screen.getByText('希望 9 月前上线')).toBeTruthy();
    expect(screen.getByText('需要更明确的实施排期')).toBeTruthy();
    expect(screen.getByText('避免一次性给出过多折扣')).toBeTruthy();
    expect(screen.getByText('关键诉求已追问。')).toBeTruthy();
    expect(screen.getAllByText('关键证据').length).toBe(3);
    expect(screen.queryByText('insufficient')).toBeNull();
  });

  it('智能 v2 详情展示四项 KPI、维度、建议和证据', () => {
    const detail = {
      ...smartV2Row,
      messages: [
        { id: 'm2', time: '2026-08-21T09:03:00Z', direction: 'inbound', senderName: '客户甲', content: '预算大约 20 万。' },
      ],
      conversationUrl: '/chat/v2-customer?conversationId=3',
    } as InsightDetail<SmartInsightRow>;
    render(<InsightDrawer detail={detail} onClose={vi.fn()} />);
    expect(screen.getByRole('heading', { name: '量化指标' })).toBeTruthy();
    expect(screen.getByText('覆盖度')).toBeTruthy();
    expect(screen.getByText('65分')).toBeTruthy();
    expect(screen.getByText('0分 · 低')).toBeTruthy();
    expect(screen.getByText('建议 48 小时内重点回访。')).toBeTruthy();
    expect(screen.getByText('预算信号')).toBeTruthy();
    expect(screen.getByText('由销售主管跟进')).toBeTruthy();
    expect(screen.getByText('优先安排演示')).toBeTruthy();
  });

  it('历史分表内 ID 仅在详情窗口唯一时兼容高亮，碰撞时失败关闭', () => {
    const result = { ...sessionSuccessRow.result, evidenceMessageIds: ['7'] };
    const baseDetail = { ...sessionSuccessRow, result, conversationUrl: '/chat/v2-customer?conversationId=legacy' };
    const unique = render(<InsightDrawer detail={{ ...baseDetail, messages: [
      { id: 'msgid:global-7', legacyId: '7', time: '2026-08-21T09:01:00Z', direction: 'inbound', senderName: '客户甲', content: '唯一旧证据' },
    ] } as InsightDetail<SessionInsightRow>} onClose={vi.fn()} />);
    expect(screen.getAllByText('关键证据')).toHaveLength(1);
    unique.unmount();

    render(<InsightDrawer detail={{ ...baseDetail, messages: [
      { id: 'msgid:global-7', legacyId: '7', time: '2026-08-21T09:01:00Z', direction: 'inbound', senderName: '客户甲', content: '碰撞证据一' },
      { id: 'shard:2:7', legacyId: '7', time: '2026-08-21T09:02:00Z', direction: 'inbound', senderName: '客户乙', content: '碰撞证据二' },
    ] } as InsightDetail<SessionInsightRow>} onClose={vi.fn()} />);
    expect(screen.queryByText('关键证据')).toBeNull();
  });

  it('nullable 分数即使带 high/low 枚举也必须显示证据不足，0 分仍保留数值', () => {
    const detail = {
      ...sessionNullableScoreRow,
      messages: [],
      conversationUrl: '/chat/v2-customer?conversationId=5',
    } as InsightDetail<SessionInsightRow>;
    render(<InsightDrawer detail={detail} onClose={vi.fn()} />);

    const qualityPair = screen.getByText('质量分 / 等级').closest('.ai-insight-detail-pair');
    const purchasePair = screen.getByText('购买意向').closest('.ai-insight-detail-pair');
    expect(qualityPair).toBeTruthy();
    expect(purchasePair).toBeTruthy();
    expect(within(qualityPair as HTMLElement).getByText('证据不足')).toBeTruthy();
    expect(within(qualityPair as HTMLElement).queryByText('高')).toBeNull();
    expect(within(purchasePair as HTMLElement).getByText('证据不足')).toBeTruthy();
    expect(within(purchasePair as HTMLElement).queryByText('低')).toBeNull();

    render(<SmartTable page={{ page: 1, pageSize: 20, total: 1, items: [smartV2Row] }} onOpen={vi.fn()} />);
    expect(screen.getByText('0分 · 低')).toBeTruthy();
  });

  it('失败详情不渲染伪分数，并支持 Escape、遮罩和原会话跳转', async () => {
    const onClose = vi.fn();
    const onNavigate = vi.fn();
    const detail = { ...failedRow, messages: [], conversationUrl: '/chat/v2-customer?conversationId=1' } as InsightDetail<SessionInsightRow>;
    render(<InsightDrawer detail={detail} onClose={onClose} onNavigate={onNavigate} />);
    expect(screen.getByText('分析失败')).toBeTruthy();
    expect(screen.queryByText('购买意向分')).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: '查看原会话' }));
    expect(onNavigate).toHaveBeenCalledWith('/chat/v2-customer?conversationId=1');
    fireEvent.keyDown(window, { key: 'Escape', code: 'Escape' });
    await waitFor(() => expect(onClose).toHaveBeenCalledTimes(1));
    fireEvent.click(screen.getByLabelText('关闭详情'));
    expect(onClose).toHaveBeenCalledTimes(2);
  });

  it('员工筛选组合框按姓名搜索、选择并保留已知姓名', async () => {
    const loadOptions = vi.fn().mockResolvedValue({
      employees: [{ id: 1001, name: '张三', avatar: '' }],
    });
    const onSelect = vi.fn();
    renderFilterField({ selectedEmployeeId: 1001, knownEmployeeName: '张三', loadOptions, onSelect });
    const combo = screen.getByRole('combobox', { name: '员工' });
    expect((combo as HTMLInputElement).value).toBe('张三');
    expect(combo.getAttribute('aria-activedescendant')).toBeNull();
    fireEvent.focus(combo);
    await waitFor(() => expect(loadOptions).toHaveBeenCalled());
    const option = (await screen.findAllByRole('option'))[0]!;
    expect(option.id).toContain('ai-insight-employee-option-1001');
    expect(combo.getAttribute('aria-activedescendant')).toBe(option.id);
    fireEvent.mouseDown(option);
    expect(onSelect).toHaveBeenLastCalledWith({ id: 1001, name: '张三', avatar: '' });
    fireEvent.click(screen.getByRole('button', { name: '清除员工' }));
    expect(onSelect).toHaveBeenLastCalledWith(undefined);
  });

  it('员工筛选组合框展示加载失败和无结果状态', async () => {
    const { loadOptions } = renderFilterField({
      loadOptions: vi.fn()
        .mockRejectedValueOnce(new Error('boom'))
        .mockResolvedValueOnce({ employees: [] }),
    });
    const combo = screen.getByRole('combobox', { name: '员工' });
    fireEvent.focus(combo);
    expect((await screen.findByRole('alert')).textContent).toContain('员工列表加载失败');
    fireEvent.change(combo, { target: { value: '李' } });
    await waitFor(() => expect(loadOptions).toHaveBeenCalledTimes(2));
    expect((await screen.findByRole('status')).textContent).toContain('没有匹配员工');
  });

  it('员工筛选组合框忽略过期请求结果，loading 时不保留旧 option 或失效 aria', async () => {
    vi.useFakeTimers();
    try {
      const first = createDeferred<EmployeeFilterOptions>();
      const second = createDeferred<EmployeeFilterOptions>();
      const loadOptions = vi.fn()
        .mockReturnValueOnce(first.promise)
        .mockReturnValueOnce(second.promise);
      renderFilterField({ loadOptions });
      const combo = screen.getByRole('combobox', { name: '员工' });

      fireEvent.focus(combo);
      await act(async () => {
        await vi.advanceTimersByTimeAsync(150);
      });
      expect(loadOptions).toHaveBeenNthCalledWith(1, undefined, 20);
      expect(screen.getByRole('status').textContent).toContain('正在加载员工…');
      expect(screen.queryByRole('option')).toBeNull();
      expect(combo.getAttribute('aria-activedescendant')).toBeNull();

      fireEvent.change(combo, { target: { value: '李' } });
      await act(async () => {
        await vi.advanceTimersByTimeAsync(150);
      });
      expect(loadOptions).toHaveBeenNthCalledWith(2, '李', 20);
      expect(screen.getByRole('status').textContent).toContain('正在加载员工…');
      expect(screen.queryByRole('option')).toBeNull();
      expect(combo.getAttribute('aria-activedescendant')).toBeNull();

      await act(async () => {
        first.resolve({ employees: [{ id: 1001, name: '张三', avatar: '' }] });
        await Promise.resolve();
      });
      expect(screen.queryByText('张三')).toBeNull();
      expect(combo.getAttribute('aria-activedescendant')).toBeNull();

      await act(async () => {
        second.resolve({ employees: [{ id: 1002, name: '李四', avatar: '' }] });
        await Promise.resolve();
      });
      const option = screen.getByRole('option', { name: /李四/ });
      expect(option.id).toContain('ai-insight-employee-option-1002');
      expect(combo.getAttribute('aria-activedescendant')).toBe(option.id);

      fireEvent.click(screen.getByRole('button', { name: '清除员工' }));
      expect(screen.queryByRole('option')).toBeNull();
      expect(combo.getAttribute('aria-activedescendant')).toBeNull();
    } finally {
      vi.useRealTimers();
    }
  });

  it('失败状态和 AI 服务不可用状态保持明确且不透传技术细节', () => {
    render(<AiInsightStatusStrip status={{ provider: { state: 'unavailable', message: '未配置模型凭证' } }} />);
    expect(screen.getByRole('status').textContent).toContain('AI 服务暂不可用');
    expect(screen.getByRole('status').textContent).toContain('请检查 AI 设置');
    expect(screen.getByRole('status').textContent).not.toContain('未配置模型凭证');
  });
});
