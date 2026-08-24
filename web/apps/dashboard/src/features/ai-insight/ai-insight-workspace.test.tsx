import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
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

afterEach(cleanup);
beforeEach(() => {
  window.sessionStorage.clear();
});

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
    customer: {
      qualityScore: 86,
      qualityLevel: '高',
      purchaseIntent: { score: 92, level: '高', reason: '客户明确询价并确认交付周期。' },
      churnRisk: { score: null, level: '', reason: '证据不足' },
      quantifiedDimensions: [
        { name: '预算匹配', score: 88, weight: 0.4, reason: '预算范围清晰。' },
        { name: '决策时机', score: null, weight: 0.3, reason: '尚未确认决策窗口。' },
      ],
      needs: ['希望 9 月前上线'],
      sentiment: '积极',
      suggestedResponse: '优先发送正式报价并确认实施窗口。',
      actionItems: ['24 小时内回访', '补充实施排期'],
      cautions: ['避免一次性给出过多折扣'],
      evidenceMessageIds: ['m2'],
    },
    employeeQa: {
      score: 78,
      dimensions: [
        { name: '需求澄清', score: 82, reason: '关键诉求已追问。' },
      ],
      unresolvedCustomerIssues: ['交付周期未完全确认'],
      unresolvedObjections: ['价格异议未闭环'],
      strengths: ['响应及时'],
      issues: ['报价说明不完整'],
      suggestions: ['补充实施里程碑'],
      evidenceMessageIds: ['m3'],
    },
  },
};

const smartV2Row: SmartInsightRow = {
  ...sessionSuccessRow,
  id: 3,
  summary: '客户近期有较高成交机会，优先级较高。',
  rule: { id: 12, name: '商机识别', version: 3 },
  result: {
    version: 'v2',
    matchScore: 88,
    confidenceScore: 73,
    priorityScore: 91,
    priorityLevel: 'P1',
    coverageScore: 65,
    summary: '命中高价值商机信号',
    conclusion: '建议 48 小时内重点回访。',
    dimensions: [
      { name: '预算信号', score: 82, weight: 0.5, reason: '客户已确认预算区间。' },
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
    version: 'v1',
    matched: true,
    confidence: 0.87,
    conclusion: '历史规则判定为潜在商机。',
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
    expect(screen.getByText('91分 · P1')).toBeTruthy();
    expect(screen.getByText(/覆盖度 65分/)).toBeTruthy();
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
    expect(screen.getByText('预算匹配')).toBeTruthy();
    expect(screen.getByText('决策时机')).toBeTruthy();
    expect(screen.getAllByText('证据不足').length).toBeGreaterThan(0);
    expect(screen.getByText('交付周期未完全确认')).toBeTruthy();
    expect(screen.getByText('价格异议未闭环')).toBeTruthy();
    expect(screen.getAllByText('关键证据').length).toBe(2);
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
    expect(screen.getByText('预算信号')).toBeTruthy();
    expect(screen.getByText('由销售主管跟进')).toBeTruthy();
    expect(screen.getByText('优先安排演示')).toBeTruthy();
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
    fireEvent.focus(combo);
    await waitFor(() => expect(loadOptions).toHaveBeenCalled());
    fireEvent.mouseDown((await screen.findAllByRole('option'))[0]!);
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

  it('失败状态和 Provider 不可用状态保持明确', () => {
    render(<AiInsightStatusStrip status={{ provider: { state: 'unavailable', message: '未配置模型凭证' } }} />);
    expect(screen.getByRole('status').textContent).toContain('AI 服务暂不可用');
    expect(screen.getByRole('status').textContent).toContain('未配置模型凭证');
  });
});
