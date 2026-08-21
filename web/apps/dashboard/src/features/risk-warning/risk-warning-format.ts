const labels: Record<string, string> = {
  private_transaction: '私下交易', sensitive_word: '敏感词', promise_rebate: '承诺返利',
  low: '低风险', medium: '中风险', high: '高风险', pending: '待审核', confirmed: '已确认', ignored: '已忽略', reviewed: '已复核',
  customer: '客户会话', group: '客户群', internal: '内部会话', enabled: '启用', disabled: '停用', employee: '员工', both: '员工与客户',
};

export function riskBehaviorLabel(value: string): string { return labels[value] ?? (value || '未标注'); }
export function riskLevelLabel(value: string): string { return labels[value] ?? (value || '未标注'); }
export function auditStatusLabel(value: string): string { return labels[value] ?? (value || '未处理'); }
export function conversationTypeLabel(value: string): string { return labels[value] ?? (value || '未标注'); }
export function formatRiskDateTime(value: string): string {
  if (!value) return '--';
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return value.replace('T', ' ').replace(/Z$/, '');
  return parsed.toLocaleString('zh-CN', { hour12: false }).replace(/\//g, '-');
}
