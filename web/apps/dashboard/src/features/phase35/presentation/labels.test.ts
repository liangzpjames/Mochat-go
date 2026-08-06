import { describe, expect, it } from 'vitest';
import { metricLabel } from './labels';
import { formatMetric } from './formatters';
describe('phase35 presentation mapping', () => {
  it('maps business keys without leaking unknown internal keys', () => { expect(metricLabel('customer')).toBe('客户数'); expect(metricLabel('contactRate')).toBe('联系人转化率'); expect(metricLabel('unknownKey')).toBe('业务指标'); expect(formatMetric(null)).toBe('--'); expect(formatMetric(1234)).toContain('1,234'); });
});
