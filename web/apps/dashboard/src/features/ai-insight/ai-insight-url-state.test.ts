import { describe, expect, it } from 'vitest';
import { readSessionFilters, readSmartState, writeSessionFilters, writeSmartState } from './ai-insight-url-state';

describe('AI 洞察 URL 状态', () => {
  it('空筛选不写入查询参数，查询变化可重置页码', () => {
    const url = writeSessionFilters({ page: 1 }, 'http://localhost/ai-insight/session-analysis?page=4&keyword=旧');
    expect(new URL(url).search).toBe('');
    const next = writeSessionFilters({ page: 1, keyword: '采购' }, url);
    expect(readSessionFilters(next)).toMatchObject({ page: 1, keyword: '采购' });
  });
  it('智能分析保留标签和规则版本', () => {
    const url = writeSmartState({ tab: 'rules', filters: { page: 2, ruleVersionId: 7 } }, 'http://localhost/ai-insight/smart-analysis');
    expect(readSmartState(url)).toEqual({ tab: 'rules', filters: { page: 2, ruleVersionId: 7 } });
  });
});
