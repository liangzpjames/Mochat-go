import { describe, expect, it } from 'vitest';
import { metricLabel } from './labels';
import { formatMetric } from './formatters';
describe('phase35 presentation mapping', () => {
  it('maps business keys and formats null safely', () => { expect(metricLabel('customer')).toBe('客户数'); expect(metricLabel('unknownKey')).toBe('Unknown Key'); expect(formatMetric(null)).toBe('--'); expect(formatMetric(1234)).toContain('1,234'); });
});
