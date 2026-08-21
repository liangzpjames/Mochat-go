import { describe, expect, it } from 'vitest';

import { toExportEndISO } from './conversation-export-dialog';

describe('toExportEndISO', () => {
  it('uses the current instant when the selected end date is today in Shanghai', () => {
    const now = new Date('2026-08-21T04:00:00.000Z');
    expect(toExportEndISO('2026-08-21', now)).toBe(now.toISOString());
  });

  it('keeps a completed day at its Shanghai end of day', () => {
    const now = new Date('2026-08-21T04:00:00.000Z');
    expect(toExportEndISO('2026-08-20', now)).toBe('2026-08-20T23:59:59+08:00');
  });
});
