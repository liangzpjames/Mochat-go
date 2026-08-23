import { describe, expect, it } from 'vitest';
import {
  filterAndPageAISettings,
  formatAISettingsTime,
  parseAISettingsListState,
  resolveKnowledgeBaseNames,
} from './list-state';

const items = [
  { id: 'enabled-agent', name: '客服助手', description: '处理售后', status: 1 },
  { id: 'disabled-agent', name: '客服助手', description: '已停用', status: 0 },
  { id: 'sales-agent', name: '销售助手', description: '跟进线索', status: 1 },
  { id: 'customer-success-agent', name: 'Customer Success', description: 'Escalation Workflow', status: 1 },
];

describe('AI 设置列表 URL 状态', () => {
  it('uses safe defaults for malformed URL values', () => {
    expect(parseAISettingsListState(new URLSearchParams('q=%20客服%20&status=unknown&page=-3&pageSize=99'))).toEqual({
      q: '客服', status: 'all', page: 1, pageSize: 10,
    });
  });

  it('keeps only supported URL state values', () => {
    expect(parseAISettingsListState('q=%E5%94%AE%E5%90%8E&status=disabled&page=2&pageSize=50')).toEqual({
      q: '售后', status: 'disabled', page: 2, pageSize: 50,
    });
  });
});

describe('AI 设置列表过滤与分页', () => {
  it('clamps page and filters real records', () => {
    const result = filterAndPageAISettings(items, { q: '客服', status: 'disabled', page: 9, pageSize: 10 });
    expect(result.page).toBe(1);
    expect(result.total).toBe(1);
    expect(result.items.map((item) => item.id)).toEqual(['disabled-agent']);
  });

  it('matches name and description without mutating records', () => {
    const result = filterAndPageAISettings(items, { q: '线索', status: 'all', page: 1, pageSize: 10 });
    expect(result.items.map((item) => item.id)).toEqual(['sales-agent']);
    expect(items).toHaveLength(4);
  });

  it('matches English keywords regardless of mixed case', () => {
    const result = filterAndPageAISettings(items, { q: 'cUsToMeR sUcCeSs', status: 'all', page: 1, pageSize: 10 });
    expect(result.items.map((item) => item.id)).toEqual(['customer-success-agent']);
  });

  it('returns an empty first page when no real record matches', () => {
    expect(filterAndPageAISettings(items, { q: '不存在', status: 'all', page: 3, pageSize: 20 })).toMatchObject({
      items: [], total: 0, page: 1,
    });
  });
});

describe('AI 设置显示格式', () => {
  it('uses a stable placeholder for absent or invalid timestamps', () => {
    expect(formatAISettingsTime('')).toBe('—');
    expect(formatAISettingsTime('not-a-time')).toBe('—');
  });

  it('formats a valid ISO timestamp with the configured local formatter', () => {
    const value = '2026-08-23T12:34:56.000Z';
    const expected = new Intl.DateTimeFormat('zh-CN', {
      year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hourCycle: 'h23',
    }).format(new Date(value));
    expect(formatAISettingsTime(value)).toBe(expected);
  });

  it('resolves only names returned by the real knowledge-base list', () => {
    expect(resolveKnowledgeBaseNames(['kb-1', 'removed-kb'], [
      { id: 'kb-1', name: '售后话术库' },
    ])).toBe('售后话术库、已失效（ID: removed-kb）');
  });

  it('deduplicates IDs in first-seen order while preserving duplicate names and orphan clarity', () => {
    expect(resolveKnowledgeBaseNames(['kb-1', 'kb-1', 'same-name-kb', 'removed-kb', 'removed-kb'], [
      { id: 'kb-1', name: '售后话术库' },
      { id: 'same-name-kb', name: '售后话术库' },
    ])).toBe('售后话术库、售后话术库、已失效（ID: removed-kb）');
  });

  it('uses a placeholder when an agent has no associated knowledge base', () => {
    expect(resolveKnowledgeBaseNames([], [])).toBe('—');
  });
});
