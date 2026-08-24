import { describe, expect, it } from 'vitest';
import { readSessionFilters, readSmartState, writeSessionFilters, writeSmartState } from './ai-insight-url-state';

describe('AI 洞察 URL 状态', () => {
  it('空筛选不写入查询参数，查询变化可重置页码', () => {
    const url = writeSessionFilters({ page: 1 }, 'http://localhost/ai-insight/session-analysis?page=4&keyword=旧');
    expect(new URL(url).search).toBe('');
    const next = writeSessionFilters({ page: 1, keyword: '采购', customerName: '客户甲' }, url);
    expect(readSessionFilters(next)).toMatchObject({ page: 1, keyword: '采购', customerName: '客户甲' });
  });

  it('智能分析忽略旧规则页和规则版本参数，只保留结果筛选', () => {
    const input = 'http://localhost/ai-insight/smart-analysis?tab=rules&ruleVersionId=7&page=2&keyword=复购&customerName=客户乙';
    expect(readSmartState(input)).toEqual({ filters: { page: 2, keyword: '复购', customerName: '客户乙' } });
    const url = writeSmartState({ filters: { page: 2, keyword: '复购', customerName: '客户乙' } }, input);
    expect(new URL(url).searchParams.has('tab')).toBe(false);
    expect(new URL(url).searchParams.has('ruleVersionId')).toBe(false);
  });

  it('保留 employeeId 兼容解析，同时读写 customerName', () => {
    const input = 'http://localhost/ai-insight/session-analysis?employeeId=1001&targetId=2001&customerName=%E5%AE%A2%E6%88%B7%E7%94%B2';
    expect(readSessionFilters(input)).toEqual({ page: 1, employeeId: 1001, targetId: '2001', customerName: '客户甲' });
    const url = writeSessionFilters({ page: 3, employeeId: 1001, customerName: '客户甲', keyword: '总结' }, input);
    const params = new URL(url).searchParams;
    expect(params.get('page')).toBe('3');
    expect(params.get('employeeId')).toBe('1001');
    expect(params.get('customerName')).toBe('客户甲');
    expect(params.get('keyword')).toBe('总结');
  });
});
