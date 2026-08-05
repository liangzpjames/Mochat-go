const labels: Record<string, string> = {
  customer: '客户数', contact: '联系人', lead: '线索', opportunity: '商机', won: '已赢单', order: '订单',
  employee: '会话员工', behavior: '行为事件', report: '综合报表', total: '总数', amount: '金额', rate: '转化率',
  createdAt: '创建时间', updatedAt: '更新时间', ownerId: '负责人', status: '状态', source: '来源', limitations: '数据限制',
};
export function metricLabel(key: string): string {
  return labels[key] ?? key.replace(/([A-Z])/g, ' $1').replace(/^./, (c) => c.toUpperCase());
}
