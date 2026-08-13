import { describe, expect, it } from 'vitest';

import { safeInternalTarget } from './safe-target';

describe('safeInternalTarget', () => {
  it('accepts a same-origin path with query and hash', () => {
    expect(safeInternalTarget('/contact?id=7#summary', '/')).toBe('/contact?id=7#summary');
    expect(safeInternalTarget('/workFission?activityId=42#tasks', '/')).toBe(
      '/workFission?activityId=42#tasks',
    );
  });

  it('rejects protocol-relative, external, javascript and malformed targets', () => {
    const fallback = '/safe';
    expect(safeInternalTarget('//evil.example/steal', fallback)).toBe(fallback);
    expect(safeInternalTarget('https://evil.example/steal', fallback)).toBe(fallback);
    expect(safeInternalTarget('javascript:alert(1)', fallback)).toBe(fallback);
    expect(safeInternalTarget('/%E0%A4%A', fallback)).toBe(fallback);
    expect(safeInternalTarget(null, fallback)).toBe(fallback);
  });
});
