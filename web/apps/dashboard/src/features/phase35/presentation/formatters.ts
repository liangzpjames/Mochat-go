function primitiveText(value: unknown): string {
  if (typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean' || typeof value === 'bigint') return String(value);
  return '--';
}

export function formatMetric(value: unknown, unit = ''): string {
  if (value === null || value === undefined || value === '') return '--';
  if (typeof value === 'number') return `${new Intl.NumberFormat('zh-CN').format(value)}${unit}`;
  const valueText = primitiveText(value);
  return valueText === '--' ? valueText : `${valueText}${unit}`;
}
export function formatDate(value: unknown): string {
  if (!value) return '--';
  const valueText = primitiveText(value);
  if (valueText === '--') return valueText;
  const date = new Date(valueText);
  return Number.isNaN(date.valueOf()) ? valueText : new Intl.DateTimeFormat('zh-CN').format(date);
}
