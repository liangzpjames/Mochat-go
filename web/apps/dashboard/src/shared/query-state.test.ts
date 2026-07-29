import { describe, expect, it } from 'vitest';

import { updateSearch } from './query-state';

describe('updateSearch', () => {
  it('updates pagination while preserving unrelated query values', () => {
    const result = updateSearch(new URLSearchParams('keyword=张三&status=active&page=1'), {
      page: 2,
      perPage: 20,
    });

    expect(result.get('keyword')).toBe('张三');
    expect(result.get('status')).toBe('active');
    expect(result.get('page')).toBe('2');
    expect(result.get('perPage')).toBe('20');
  });

  it('removes undefined and blank filters without mutating the input', () => {
    const current = new URLSearchParams('keyword=张三&status=active');
    const result = updateSearch(current, { keyword: ' ', status: undefined });

    expect(result.has('keyword')).toBe(false);
    expect(result.has('status')).toBe(false);
    expect(current.get('keyword')).toBe('张三');
  });
});
