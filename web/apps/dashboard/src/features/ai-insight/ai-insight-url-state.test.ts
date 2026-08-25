import { describe, expect, it } from 'vitest';
import { readSessionFilters, readSmartState, writeSessionFilters, writeSmartState } from './ai-insight-url-state';
import * as urlState from './ai-insight-url-state';
import type { DerivedInsightFilters, DerivedInsightView } from './ai-insight-workspace-api';

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

type Equal<Left, Right> = (<Type>() => Type extends Left ? 1 : 2) extends (<Type>() => Type extends Right ? 1 : 2)
  ? (<Type>() => Type extends Right ? 1 : 2) extends (<Type>() => Type extends Left ? 1 : 2) ? true : false
  : false;
type Assert<Type extends true> = Type;
type DerivedUrlStateContract = {
  readDerivedFilters(view: DerivedInsightView, input?: string | URL): DerivedInsightFilters;
  writeDerivedFilters(view: DerivedInsightView, filters: DerivedInsightFilters, input?: string | URL): string;
};
type _ReadDerivedFiltersSignature = Assert<Equal<typeof urlState.readDerivedFilters, DerivedUrlStateContract['readDerivedFilters']>>;
type _WriteDerivedFiltersSignature = Assert<Equal<typeof urlState.writeDerivedFilters, DerivedUrlStateContract['writeDerivedFilters']>>;
const derivedUrlSignaturesCompile: [_ReadDerivedFiltersSignature, _WriteDerivedFiltersSignature] = [true, true];
void derivedUrlSignaturesCompile;

function derivedUrlState(): DerivedUrlStateContract {
  const module = urlState as unknown as Partial<DerivedUrlStateContract>;
  if (typeof module.readDerivedFilters !== 'function') throw new Error('ai-insight-url-state 缺少 readDerivedFilters');
  if (typeof module.writeDerivedFilters !== 'function') throw new Error('ai-insight-url-state 缺少 writeDerivedFilters');
  return module as DerivedUrlStateContract;
}

describe('AI 洞察专用投影 URL 状态', () => {
  it('情绪五态逐一读写且不丢失', () => {
    const state = derivedUrlState();
    for (const emotion of ['positive', 'neutral', 'negative', 'mixed', 'unknown'] as const) {
      expect(state.readDerivedFilters('emotion', `http://localhost/ai-insight/emotion?emotion=${emotion}`)).toEqual({ page: 1, emotion });
      const url = state.writeDerivedFilters('emotion', { page: 1, emotion }, 'http://localhost/ai-insight/emotion?emotion=stale&minScore=0&keyword=old');
      expect(new URL(url).search).toBe(`?emotion=${emotion}`);
      expect(state.readDerivedFilters('emotion', url)).toEqual({ page: 1, emotion });
    }
  });

  it('三页只恢复 common 条件和各自专用条件', () => {
    const state = derivedUrlState();
    const common = 'page=2&employeeId=1001&customerId=2001&status=succeeded&startDate=2026-08-20&endDate=2026-08-24';
    expect(state.readDerivedFilters('emotion', `http://localhost/ai-insight/emotion?${common}&emotion=negative&minScore=0&maxScore=100&keyword=old`)).toEqual({
      page: 2, employeeId: 1001, customerId: 2001, status: 'succeeded', startDate: '2026-08-20', endDate: '2026-08-24', emotion: 'negative',
    });
    expect(state.readDerivedFilters('employee-score', `http://localhost/ai-insight/employee-score?${common}&emotion=positive&minScore=0&maxScore=100&keyword=old`)).toEqual({
      page: 2, employeeId: 1001, customerId: 2001, status: 'succeeded', startDate: '2026-08-20', endDate: '2026-08-24', minScore: 0, maxScore: 100,
    });
    expect(state.readDerivedFilters('communication-keyword', `http://localhost/ai-insight/communication-keyword?${common}&emotion=mixed&minScore=0&maxScore=100&keyword=50%25_%5C%E9%87%87%E8%B4%AD`)).toEqual({
      page: 2, employeeId: 1001, customerId: 2001, status: 'succeeded', startDate: '2026-08-20', endDate: '2026-08-24', keyword: '50%_\\采购',
    });
  });

  it('丢弃非法、越界和不属于当前 view 的参数', () => {
    const state = derivedUrlState();
    const invalid = 'http://localhost/ai-insight/emotion?page=0&employeeId=-1&status=complete&startDate=2026-13-40&endDate=today&emotion=happy&minScore=-1&maxScore=101&keyword=伪字段';
    expect(state.readDerivedFilters('emotion', invalid)).toEqual({ page: 1 });
    expect(state.readDerivedFilters('employee-score', 'http://localhost/ai-insight/employee-score?minScore=90&maxScore=20')).toEqual({ page: 1 });
    expect(state.readDerivedFilters('employee-score', 'http://localhost/ai-insight/employee-score?minScore=0&maxScore=100.5')).toEqual({ page: 1, minScore: 0 });
  });

  it('写入时清除其他 view 的陈旧专用参数并保留 0 分', () => {
    const state = derivedUrlState();
    const stale = 'http://localhost/ai-insight/employee-score?page=9&emotion=positive&minScore=80&maxScore=90&keyword=旧';
    const scoreURL = state.writeDerivedFilters('employee-score', { page: 1, employeeId: 1001, customerId: 2002, minScore: 0, maxScore: 100 }, stale);
    expect(new URL(scoreURL).search).toBe('?employeeId=1001&customerId=2002&minScore=0&maxScore=100');
    expect(state.readDerivedFilters('employee-score', scoreURL)).toEqual({ page: 1, employeeId: 1001, customerId: 2002, minScore: 0, maxScore: 100 });

    const keywordURL = state.writeDerivedFilters('communication-keyword', { page: 3, keyword: '采购' }, scoreURL);
    const params = new URL(keywordURL).searchParams;
    expect(params.get('page')).toBe('3');
    expect(params.get('keyword')).toBe('采购');
    expect(params.has('minScore')).toBe(false);
    expect(params.has('maxScore')).toBe(false);
  });

  it('查询条件变化可回到第一页，浏览器返回的旧 URL 可完整恢复', () => {
    const state = derivedUrlState();
    const oldPage = 'http://localhost/ai-insight/emotion?page=4&employeeId=1001&emotion=negative';
    const queried = state.writeDerivedFilters('emotion', { page: 1, employeeId: 1001, emotion: 'positive' }, oldPage);
    expect(new URL(queried).searchParams.has('page')).toBe(false);
    expect(state.readDerivedFilters('emotion', queried)).toEqual({ page: 1, employeeId: 1001, emotion: 'positive' });
    expect(state.readDerivedFilters('emotion', oldPage)).toEqual({ page: 4, employeeId: 1001, emotion: 'negative' });
  });
});
