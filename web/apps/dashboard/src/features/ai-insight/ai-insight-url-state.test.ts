import { describe, expect, it } from 'vitest';
import { readSessionFilters, readSmartState, writeSessionFilters, writeSmartState } from './ai-insight-url-state';

describe('AI 洞察 URL 状态', () => {
  it('空筛选不写入查询参数，查询变化可重置页码', () => {
    const url = writeSessionFilters({ page: 1 }, 'http://localhost/ai-insight/session-analysis?page=4&keyword=旧');
    expect(new URL(url).search).toBe('');
    const next = writeSessionFilters({ page: 1, keyword: '采购' }, url);
    expect(readSessionFilters(next)).toMatchObject({ page: 1, keyword: '采购' });
  });
  it('智能分析忽略旧规则页和规则版本参数，只保留结果筛选', () => {
    const input = 'http://localhost/ai-insight/smart-analysis?tab=rules&ruleVersionId=7&page=2&keyword=复购';
    expect(readSmartState(input)).toEqual({ filters: { page: 2, keyword: '复购' } });
    const url = writeSmartState({ filters: { page: 2, keyword: '复购' } }, input);
    expect(new URL(url).searchParams.has('tab')).toBe(false);
    expect(new URL(url).searchParams.has('ruleVersionId')).toBe(false);
  });
});
