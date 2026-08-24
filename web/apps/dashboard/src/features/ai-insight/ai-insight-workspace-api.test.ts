import { describe, expect, it, vi } from 'vitest';
import { createAiInsightExportDownloader, createAiInsightWorkspaceApi } from './ai-insight-workspace-api';
import type {
  AiInsightWorkspaceApi,
  DerivedInsightFilters,
  DerivedInsightView,
  EmotionLabel,
  EmployeeFilterOptions,
  InsightDetail,
  InsightPage,
  InsightRunStatus,
  SessionInsightRow,
} from './ai-insight-workspace-api';

const typedDerivedViews: DerivedInsightView[] = ['emotion', 'employee-score', 'communication-keyword'];
const typedEmotionLabels: EmotionLabel[] = ['positive', 'neutral', 'negative', 'mixed', 'unknown'];
const typedDerivedFilters: DerivedInsightFilters = { page: 1, minScore: 0 };
void [typedDerivedViews, typedEmotionLabels, typedDerivedFilters];

type Equal<Left, Right> = (<Type>() => Type extends Left ? 1 : 2) extends (<Type>() => Type extends Right ? 1 : 2)
  ? (<Type>() => Type extends Right ? 1 : 2) extends (<Type>() => Type extends Left ? 1 : 2) ? true : false
  : false;
type Assert<Type extends true> = Type;
type ExpectedDerivedInsightView = 'emotion' | 'employee-score' | 'communication-keyword';
type ExpectedEmotionLabel = 'positive' | 'neutral' | 'negative' | 'mixed' | 'unknown';
type ExpectedInsightStatus = 'pending' | 'running' | 'succeeded' | 'failed';
type ExpectedDerivedInsightFilters = {
  page: number;
  employeeId?: number | undefined;
  customerName?: string | undefined;
  status?: ExpectedInsightStatus | undefined;
  startDate?: string | undefined;
  endDate?: string | undefined;
  emotion?: ExpectedEmotionLabel | undefined;
  minScore?: number | undefined;
  maxScore?: number | undefined;
  keyword?: string | undefined;
};
type _DerivedInsightViewIsExact = Assert<Equal<DerivedInsightView, ExpectedDerivedInsightView>>;
type _EmotionLabelIsExact = Assert<Equal<EmotionLabel, ExpectedEmotionLabel>>;
type _DerivedInsightFilterKeysAreExact = Assert<Equal<keyof DerivedInsightFilters, keyof ExpectedDerivedInsightFilters>>;
type _DerivedInsightFiltersAreExact = Assert<Equal<DerivedInsightFilters, ExpectedDerivedInsightFilters>>;
type ExpectedDerivedApiMethods = {
  derivedRecords(view: ExpectedDerivedInsightView, filters: ExpectedDerivedInsightFilters): Promise<InsightPage<SessionInsightRow>>;
  derivedDetail(view: ExpectedDerivedInsightView, id: number): Promise<InsightDetail<SessionInsightRow>>;
  derivedStatus(view: ExpectedDerivedInsightView): Promise<InsightRunStatus>;
  derivedFilterOptions(view: ExpectedDerivedInsightView, employeeKeyword?: string, limit?: number): Promise<EmployeeFilterOptions>;
  derivedExportUrl(view: ExpectedDerivedInsightView, filters: ExpectedDerivedInsightFilters): string;
};
type _DerivedApiPublicSignature = Assert<Equal<AiInsightWorkspaceApi & ExpectedDerivedApiMethods, AiInsightWorkspaceApi>>;
const derivedApiTypeOracle: [
  _DerivedInsightViewIsExact,
  _EmotionLabelIsExact,
  _DerivedInsightFilterKeysAreExact,
  _DerivedInsightFiltersAreExact,
  _DerivedApiPublicSignature,
] = [true, true, true, true, true];
void derivedApiTypeOracle;

const valid = {
  id: 1,
  conversationKey: '1001:1:2001',
  employee: { id: 1001, name: '员工甲', avatar: '' },
  target: { type: 'direct', id: '2001', name: '客户甲', avatar: '' },
  sourceWindow: { startedAt: '2026-08-21T09:00:00Z', endedAt: '2026-08-21T09:10:00Z', messageCount: 4, fingerprint: 'a'.repeat(64) },
  status: 'succeeded',
  summary: '客户有明确采购意向',
  analysisAt: '2026-08-21T09:11:00Z',
  result: {},
};

describe('AI 洞察工作台 API', () => {
  it('拒绝没有真实会话键的结果行', async () => {
    const client = { request: vi.fn().mockResolvedValue({ page: 1, pageSize: 20, total: 1, items: [{ id: 1 }] }) };
    await expect(createAiInsightWorkspaceApi(client).sessionRecords({ page: 1 })).rejects.toThrow('AI 洞察列表接口返回了无效数据');
  });

  it('解析会话结果并固定每页 20 条', async () => {
    const client = { request: vi.fn().mockResolvedValue({ page: 1, pageSize: 99, total: 1, items: [valid] }) };
    const page = await createAiInsightWorkspaceApi(client).sessionRecords({ page: 1, customerName: '客户甲' });
    expect(page.items[0]!.conversationKey).toBe('1001:1:2001');
    expect(page.pageSize).toBe(20);
    expect(client.request).toHaveBeenCalledWith(expect.stringContaining('customerName=%E5%AE%A2%E6%88%B7%E7%94%B2'));
  });

  it('智能结果必须带规则快照', async () => {
    const client = { request: vi.fn().mockResolvedValue({ page: 1, total: 1, items: [valid] }) };
    await expect(createAiInsightWorkspaceApi(client).smartRecords({ page: 1 })).rejects.toThrow('智能分析结果缺少规则快照');
  });

  it('列表和详情保留失败原因', async () => {
    const failed = { ...valid, status: 'failed', errorSummary: '模型响应超时' };
    const client = {
      request: vi.fn()
        .mockResolvedValueOnce({ page: 1, total: 1, items: [failed] })
        .mockResolvedValueOnce({ ...failed, messages: [], conversationUrl: '/chat/v2-customer?conversationId=1' }),
    };
    const api = createAiInsightWorkspaceApi(client);
    expect((await api.sessionRecords({ page: 1 })).items[0]!.errorSummary).toBe('模型响应超时');
    expect((await api.sessionDetail(1)).errorSummary).toBe('模型响应超时');
  });

  it('严格解析员工筛选选项响应', async () => {
    const client = {
      request: vi.fn().mockResolvedValue({
        employees: [
          { id: 1001, name: '张三', avatar: 'https://example.com/a.png' },
        ],
      }),
    };
    const api = createAiInsightWorkspaceApi(client);
    await expect(api.sessionFilterOptions('张', 5)).resolves.toEqual({
      employees: [{ id: 1001, name: '张三', avatar: 'https://example.com/a.png' }],
    });
    expect(client.request).toHaveBeenCalledWith('/ai-insight/session-analysis/filter-options?employeeKeyword=%E5%BC%A0&limit=5');
  });

  it('员工筛选选项响应无效时诚实失败', async () => {
    const client = { request: vi.fn().mockResolvedValue({ employees: [{ id: 0, name: '', avatar: '' }] }) };
    const api = createAiInsightWorkspaceApi(client);
    await expect(api.smartFilterOptions('坏数据')).rejects.toThrow('员工筛选选项返回了无效数据');
  });

  it('导出路径写入 customerName 但保留 employeeId', () => {
    const api = createAiInsightWorkspaceApi({ request: vi.fn() });
    expect(api.sessionExportUrl({ page: 2, employeeId: 1001, customerName: '客户甲', keyword: '结论' }))
      .toContain('page=2&employeeId=1001&customerName=%E5%AE%A2%E6%88%B7%E7%94%B2&keyword=%E7%BB%93%E8%AE%BA');
  });
});

const validDerived = {
  ...valid,
  provider: 'provider-a',
  model: 'model-a',
  promptVersion: 'v2',
  result: {
    schemaVersion: 2,
    summary: '真实会话投影',
    customer: {
      emotion: { label: 'positive', reason: '客户明确表达认可', evidenceMessageIds: ['m1'] },
      keywords: ['50%_\\采购', '高意向'],
    },
    employeeQa: { score: 0, dimensions: [], strengths: [], issues: [], suggestions: [] },
  },
};

type DerivedApiContract = ExpectedDerivedApiMethods;

function createDerivedApi(client: { request: ReturnType<typeof vi.fn> }): DerivedApiContract {
  const api = createAiInsightWorkspaceApi(client) as unknown as Partial<DerivedApiContract>;
  for (const name of ['derivedRecords', 'derivedDetail', 'derivedStatus', 'derivedFilterOptions', 'derivedExportUrl'] as const) {
    if (typeof api[name] !== 'function') throw new Error(`AiInsightWorkspaceApi 缺少统一方法 ${name}`);
  }
  return api as DerivedApiContract;
}

describe('AI 洞察专用投影统一 API 合同', () => {
  it('通过带登录态的客户端请求导出 Blob 并保留服务端文件名', async () => {
    const blob = new Blob(['emotion,csv']);
    const download = vi.fn().mockResolvedValue({ blob, filename: 'emotion-insights.csv' });
    const downloadExport = createAiInsightExportDownloader({ request: vi.fn(), download });

    await expect(downloadExport('emotion', { page: 2, employeeId: 1001, emotion: 'negative' }))
      .resolves.toEqual({ blob, filename: 'emotion-insights.csv' });
    expect(download).toHaveBeenCalledWith('/ai-insight/emotion/export?page=2&employeeId=1001&emotion=negative');
  });

  it('五个统一方法生成精确 derived 路径并保留真实 0 分', async () => {
    const detail = {
      ...validDerived,
      messages: [{ id: 'm1', time: '2026-08-21T09:01:00Z', direction: 'inbound', senderName: '客户甲', content: '采购 50%_\\ 套餐' }],
      conversationUrl: '/chat/v2-customer?employeeId=1001&conversationId=1001%3A1%3A2001',
    };
    const client = {
      request: vi.fn()
        .mockResolvedValueOnce({ page: 1, pageSize: 99, total: 1, items: [validDerived] })
        .mockResolvedValueOnce(detail)
        .mockResolvedValueOnce({ provider: { state: 'ready' } })
        .mockResolvedValueOnce({ employees: [{ id: 1001, name: '员工甲', avatar: '' }] }),
    };
    const api = createDerivedApi(client);
    const page = await api.derivedRecords('employee-score', {
      page: 2, employeeId: 1001, customerName: '客户甲', status: 'succeeded', startDate: '2026-08-20', endDate: '2026-08-24', minScore: 0, maxScore: 100,
    });
    expect(page.pageSize).toBe(20);
    await api.derivedDetail('emotion', 1);
    await api.derivedStatus('communication-keyword');
    await api.derivedFilterOptions('emotion', '张 三', 20);
    expect(client.request).toHaveBeenNthCalledWith(1, '/ai-insight/employee-score/records?page=2&employeeId=1001&customerName=%E5%AE%A2%E6%88%B7%E7%94%B2&status=succeeded&startDate=2026-08-20&endDate=2026-08-24&minScore=0&maxScore=100');
    expect(client.request).toHaveBeenNthCalledWith(2, '/ai-insight/emotion/detail?id=1');
    expect(client.request).toHaveBeenNthCalledWith(3, '/ai-insight/communication-keyword/status');
    expect(client.request).toHaveBeenNthCalledWith(4, '/ai-insight/emotion/filter-options?employeeKeyword=%E5%BC%A0+%E4%B8%89&limit=20');
    expect(client.request).toHaveBeenCalledTimes(4);
    expect(api.derivedExportUrl('communication-keyword', { page: 1, keyword: '50%_\\采购' }))
      .toBe('/ai-insight/communication-keyword/export?keyword=50%25_%5C%E9%87%87%E8%B4%AD');
  });

  it('按 view 严格解析五态情绪、0 到 100 分和字符串关键词数组', async () => {
    for (const label of ['positive', 'neutral', 'negative', 'mixed', 'unknown']) {
      const client = { request: vi.fn().mockResolvedValue({ page: 1, total: 1, items: [{ ...validDerived, result: { ...validDerived.result, customer: { ...validDerived.result.customer, emotion: { ...validDerived.result.customer.emotion, label } } } }] }) };
      const page = await createDerivedApi(client).derivedRecords('emotion', { page: 1 });
      expect(((page.items[0]!.result as Record<string, unknown>).customer as { emotion: { label: string } }).emotion.label).toBe(label);
    }
    for (const score of [0, 100]) {
      const client = { request: vi.fn().mockResolvedValue({ page: 1, total: 1, items: [{ ...validDerived, result: { ...validDerived.result, employeeQa: { ...validDerived.result.employeeQa, score } } }] }) };
      const page = await createDerivedApi(client).derivedRecords('employee-score', { page: 1, minScore: 0, maxScore: 100 });
      expect(((page.items[0]!.result as Record<string, unknown>).employeeQa as { score: number }).score).toBe(score);
    }
    const keywordClient = { request: vi.fn().mockResolvedValue({ page: 1, total: 1, items: [validDerived] }) };
    const keywordPage = await createDerivedApi(keywordClient).derivedRecords('communication-keyword', { page: 1, keyword: '采购' });
    expect(((keywordPage.items[0]!.result as Record<string, unknown>).customer as { keywords: string[] }).keywords).toEqual(['50%_\\采购', '高意向']);
  });

  it('失败记录允许没有 result，但必须保留 errorSummary', async () => {
    const failedBase = { ...validDerived };
    Reflect.deleteProperty(failedBase, 'result');
    const client = { request: vi.fn().mockResolvedValue({ page: 1, total: 1, items: [{ ...failedBase, status: 'failed', errorSummary: '模型响应超时' }] }) };
    const row = (await createDerivedApi(client).derivedRecords('emotion', { page: 1 })).items[0]!;
    expect(row.result).toBeUndefined();
    expect(row.errorSummary).toBe('模型响应超时');
  });

  it.each([
    ['emotion', { ...validDerived, result: { ...validDerived.result, customer: { ...validDerived.result.customer, emotion: { ...validDerived.result.customer.emotion, label: 'happy' } } } }, '未知客户情绪：happy'],
    ['communication-keyword', { ...validDerived, result: { ...validDerived.result, customer: { ...validDerived.result.customer, keywords: ['采购', 7] } } }, '客户关键词必须是字符串数组'],
  ])('拒绝 %s 投影的伪造成功结果', async (view, item, message) => {
    const client = { request: vi.fn().mockResolvedValue({ page: 1, total: 1, items: [item] }) };
    await expect(createDerivedApi(client).derivedRecords(view as DerivedInsightView, { page: 1 })).rejects.toThrow(message);
  });

  it.each([
    [-1, '员工评分超出 0-100'],
    [101, '员工评分超出 0-100'],
    ['not-a-number', '员工评分缺失或不是数值'],
    [undefined, '员工评分缺失或不是数值'],
  ])('成功记录的 employeeQa.score=%s 时明确拒绝', async (score, message) => {
    const result = { ...validDerived.result, employeeQa: { ...validDerived.result.employeeQa } } as Record<string, unknown>;
    (result.employeeQa as Record<string, unknown>).score = score;
    const client = { request: vi.fn().mockResolvedValue({ page: 1, total: 1, items: [{ ...validDerived, result }] }) };
    await expect(createDerivedApi(client).derivedRecords('employee-score', { page: 1 })).rejects.toThrow(message);
  });

  it.each([
    [{ ...validDerived, id: 0 }, 'AI 洞察列表接口返回了无效数据'],
    [{ ...validDerived, conversationKey: '' }, 'AI 洞察列表接口返回了无效数据'],
    [{ ...validDerived, sourceWindow: { ...validDerived.sourceWindow, messageCount: 0 } }, 'AI 洞察列表接口返回了无效数据'],
    [{ ...validDerived, sourceWindow: { ...validDerived.sourceWindow, fingerprint: '' } }, 'AI 洞察列表接口返回了无效数据'],
    [{ ...validDerived, employee: { id: 0, name: '', avatar: '' } }, '员工资料不完整'],
    [{ ...validDerived, target: { type: 'direct', id: '0', name: '客户甲', avatar: '' } }, '对象资料不完整'],
    [{ ...validDerived, target: { type: 'unknown', id: '2001', name: '客户甲', avatar: '' } }, '未知会话类型：unknown'],
    [{ ...validDerived, status: 'complete' }, '未知分析状态：complete'],
  ])('严格拒绝缺失真实来源身份的数据', async (item, message) => {
    const client = { request: vi.fn().mockResolvedValue({ page: 1, total: 1, items: [item] }) };
    await expect(createDerivedApi(client).derivedRecords('emotion', { page: 1 })).rejects.toThrow(message);
  });

  it('详情要求正数 id、conversationUrl 和已知消息方向', async () => {
    const invalidIdClient = { request: vi.fn() };
    await expect(createDerivedApi(invalidIdClient).derivedDetail('emotion', 0)).rejects.toThrow('详情 ID 必须为正数');
    expect(invalidIdClient.request).not.toHaveBeenCalled();

    const missingUrl = { ...validDerived, messages: [], conversationUrl: '' };
    await expect(createDerivedApi({ request: vi.fn().mockResolvedValue(missingUrl) }).derivedDetail('emotion', 1)).rejects.toThrow('详情缺少会话跳转地址');
    const badDirection = { ...validDerived, messages: [{ id: 'm1', time: '', direction: 'sideways', senderName: '客户', content: '你好' }], conversationUrl: '/chat/v2-customer' };
    await expect(createDerivedApi({ request: vi.fn().mockResolvedValue(badDirection) }).derivedDetail('emotion', 1)).rejects.toThrow('详情消息方向无效');
  });

  it('provider/model/promptVersion 未记录时保持空值，不写死展示元数据', async () => {
    const withoutMetadata = { ...validDerived };
    Reflect.deleteProperty(withoutMetadata, 'provider');
    Reflect.deleteProperty(withoutMetadata, 'model');
    Reflect.deleteProperty(withoutMetadata, 'promptVersion');
    const client = { request: vi.fn().mockResolvedValue({ page: 1, total: 1, items: [withoutMetadata] }) };
    const row = (await createDerivedApi(client).derivedRecords('emotion', { page: 1 })).items[0]!;
    expect(row).toMatchObject({ provider: '', model: '', promptVersion: '' });
  });

  it('保留真实字母数字客户和群聊目标 ID', async () => {
    for (const target of [
      { type: 'direct', id: 'wm_customer_A1', name: '客户甲', avatar: '' },
      { type: 'group', id: 'wr_room_B2', name: '客户群乙', avatar: '' },
    ]) {
      const client = { request: vi.fn().mockResolvedValue({ page: 1, total: 1, items: [{ ...validDerived, target }] }) };
      const item = (await createDerivedApi(client).derivedRecords('emotion', { page: 1 })).items[0]!;
      expect(item.target.id).toBe(target.id);
    }
  });
});
