import { describe, expect, it } from 'vitest';
import { zonedBoundary } from './report-query';

describe('report query date boundaries', () => {
  it('serializes local midnight with an RFC3339 offset', () => {
    expect(zonedBoundary('2026-07-31', 'Asia/Shanghai')).toBe('2026-07-31T00:00:00+08:00');
    expect(zonedBoundary('2026-07-31', 'UTC')).toBe('2026-07-31T00:00:00Z');
  });
});
