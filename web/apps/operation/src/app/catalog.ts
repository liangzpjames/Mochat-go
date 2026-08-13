export type OperationCatalogEntry = {
  title: string;
  description: string;
};

export const operationCatalog = {
  '/': {
    title: '营销活动中心',
    description: '营销活动 H5 历史入口。',
  },
  '/explain': {
    title: '活动说明',
    description: '活动参与条件、奖励规则与注意事项。',
  },
  '/lottery': {
    title: '抽奖活动',
    description: '营销抽奖活动参与入口。',
  },
  '/roomClockIn': {
    title: '群打卡',
    description: '客户群打卡活动参与入口。',
  },
  '/roomFission': {
    title: '群裂变活动',
    description: '客户群裂变活动参与入口。',
  },
  '/fissionSpeed': {
    title: '群裂变进度',
    description: '客户群裂变活动进度入口。',
  },
  '/roomInfinitePull': {
    title: '无限拉群',
    description: '客户群可用二维码入口。',
  },
  '/shopCode': {
    title: '门店活码',
    description: '门店专属服务员工添加入口。',
  },
  '/workFission': {
    title: '任务宝活动',
    description: '邀请好友助力并查看任务进度。',
  },
  '/speed': {
    title: '任务宝进度',
    description: '任务宝助力与奖励状态入口。',
  },
} as const satisfies Record<string, OperationCatalogEntry>;
