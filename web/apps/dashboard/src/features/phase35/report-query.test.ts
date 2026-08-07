import { describe, expect, it } from 'vitest';
import { buildReportFilterParams, zonedBoundary } from './report-query';

describe('report query date boundaries', () => {
  it('serializes local midnight with an RFC3339 offset', () => {
    expect(zonedBoundary('2026-07-31', 'Asia/Shanghai')).toBe('2026-07-31T00:00:00+08:00');
    expect(zonedBoundary('2026-07-31', 'UTC')).toBe('2026-07-31T00:00:00Z');
  });

  it('sends employee and department filters under plural contract keys', () => {
    const params = buildReportFilterParams(
      { startDate: '2026-08-01', endDate: '2026-09-01', employeeId: '4', departmentId: '1' },
      'Asia/Shanghai',
      1536612155,
    );
    expect(params.employeeIds).toBe('4');
    expect(params.departmentIds).toBe('1');
    expect(params.corpId).toBe(1536612155);
    expect(params.startAt).toBe('2026-08-01T00:00:00+08:00');
  });

  it('omits employee and department keys when filters are empty', () => {
    const params = buildReportFilterParams(
      { startDate: '2026-08-01', endDate: '2026-09-01', employeeId: '', departmentId: '' },
      'UTC',
    );
    expect('employeeIds' in params).toBe(false);
    expect('departmentIds' in params).toBe(false);
  });
});
