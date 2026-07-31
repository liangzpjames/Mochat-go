export type DemoColumn = {
  key: 'name' | 'status' | 'updatedAt';
  title: string;
};

export type DemoRow = {
  id: string;
  name: string;
  status: '正常' | '待跟进' | '预警';
  updatedAt: string;
  detail: readonly { label: string; value: string }[];
};

export type DemoPageConfig = {
  title: string;
  searchLabel: string;
  columns: readonly DemoColumn[];
  rows: readonly DemoRow[];
};

const columns: readonly DemoColumn[] = [
  { key: 'name', title: '名称' },
  { key: 'status', title: '状态' },
  { key: 'updatedAt', title: '更新时间' },
];

function row(id: string, name: string, status: DemoRow['status'], updatedAt: string): DemoRow {
  return {
    id,
    name,
    status,
    updatedAt,
    detail: [
      { label: '名称', value: name },
      { label: '状态', value: status },
      { label: '更新时间', value: updatedAt },
      { label: '数据说明', value: '稳定演示 fixture，非真实持久化数据' },
    ],
  };
}

function createDemo(title: string, names: readonly string[]): DemoPageConfig {
  return {
    title,
    searchLabel: `搜索${title}`,
    columns,
    rows: names.map((name, index) => row(
      `${title}-${index + 1}`,
      name,
      index % 3 === 0 ? '正常' : index % 3 === 1 ? '待跟进' : '预警',
      `2026-07-${String(28 - index).padStart(2, '0')} 10:${String(index).padStart(2, '0')}`,
    )),
  };
}

export const staffConversationDemo = createDemo('员工会话', ['林晨', '周明', '许诺']);
export const customerConversationDemo = createDemo('客户会话', ['星河科技', '云帆商贸', '远望咨询']);
export const groupConversationDemo = createDemo('群聊会话', ['产品交流 01 群', '客户服务群', '华东运营群']);
export const riskBehaviorDemo = createDemo('风险行为', ['异常转发提醒', '敏感内容提醒', '高频外发提醒']);
export const timeoutWarningDemo = createDemo('超时预警', ['华东客户服务', 'VIP 续约咨询', '新客接待']);
export const sessionAnalysisDemo = createDemo('会话分析', ['本周会话趋势', '客户响应分析', '员工服务分析']);
export const channelCodeDemo = createDemo('渠道活码', ['官网咨询入口', '活动落地页', '华东渠道码']);
export const contactDemo = createDemo('联系人', ['陈佳', '王博', '赵宁']);
export const customerGroupDemo = createDemo('客户群', ['新品体验群', '华东客户群', '服务支持群']);
