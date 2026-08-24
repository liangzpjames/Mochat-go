import { describe, expect, it, vi } from 'vitest';
import { createAiInsightWorkspaceApi } from './ai-insight-workspace-api';

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
