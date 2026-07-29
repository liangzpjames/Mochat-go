export type OperationRouteConfig = {
  title: string;
  subtitle: string;
  stages: string[];
  primaryAction: string;
  progress?: string;
};

export const operationRoutes: Readonly<Record<string, OperationRouteConfig>> = {
  '/': {
    title: '营销活动中心',
    subtitle: '选择活动并查看参与进度。',
    stages: ['活动信息', '参与入口'],
    primaryAction: '查看活动',
  },
  '/explain': {
    title: '活动说明',
    subtitle: '了解参与条件、奖励和注意事项。',
    stages: ['参与条件', '活动规则', '奖励说明'],
    primaryAction: '我知道了',
  },
  '/lottery': {
    title: '抽奖活动',
    subtitle: '完成指定任务后获得抽奖机会。',
    stages: ['活动任务', '抽奖机会', '中奖结果'],
    primaryAction: '立即抽奖',
  },
  '/roomClockIn': {
    title: '群打卡',
    subtitle: '在活动群中完成每日打卡。',
    stages: ['今日任务', '连续打卡', '奖励进度'],
    primaryAction: '立即打卡',
    progress: '已完成 3 / 7',
  },
  '/roomFission': {
    title: '群裂变活动',
    subtitle: '邀请好友进群并解锁活动奖励。',
    stages: ['邀请海报', '好友进群', '领取奖励'],
    primaryAction: '生成邀请海报',
  },
  '/fissionSpeed': {
    title: '群裂变进度',
    subtitle: '查看有效邀请和目标完成情况。',
    stages: ['有效邀请', '目标进度', '奖励状态'],
    primaryAction: '刷新进度',
    progress: '已完成 3 / 5',
  },
  '/roomInfinitePull': {
    title: '无限拉群',
    subtitle: '扫描当前可用二维码进入客户群。',
    stages: ['选择群聊', '扫码进群', '入群结果'],
    primaryAction: '刷新二维码',
  },
  '/shopCode': {
    title: '门店活码',
    subtitle: '选择附近门店并添加专属服务员工。',
    stages: ['选择门店', '门店员工', '扫码添加'],
    primaryAction: '选择门店',
  },
  '/workFission': {
    title: '任务宝活动',
    subtitle: '邀请好友助力完成企业任务。',
    stages: ['活动任务', '好友助力', '任务奖励'],
    primaryAction: '邀请好友助力',
  },
  '/speed': {
    title: '任务进度',
    subtitle: '查看任务宝助力进度和奖励状态。',
    stages: ['助力人数', '目标进度', '领奖状态'],
    primaryAction: '刷新进度',
    progress: '已完成 2 / 3',
  },
};
