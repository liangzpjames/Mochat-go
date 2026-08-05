import { describe, expect, it } from 'vitest';
import { reportEndpoint, type Phase35ReportKind } from './api';

describe('phase35 report endpoints', () => {
  it('uses dashboard-relative paths because ApiClient supplies the prefix', () => {
    const kinds: Phase35ReportKind[] = ['customer', 'employee', 'conversion', 'behavior', 'report'];
    expect(kinds.map(reportEndpoint)).toEqual([
      '/reports/customer', '/reports/employee', '/reports/conversion', '/reports/behavior', '/reports/report',
    ]);
    expect(reportEndpoint('customer')).not.toContain('/dashboard/dashboard');
  });
});
