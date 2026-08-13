import { describe, expect, it } from 'vitest';

import { operationCatalog } from './catalog';

describe('Operation route catalog', () => {
  it('keeps the approved Chinese title for every historical activity URL', () => {
    expect(Object.fromEntries(
      Object.entries(operationCatalog).map(([path, entry]) => [path, entry.title]),
    )).toEqual({
      '/': '营销活动中心',
      '/explain': '活动说明',
      '/lottery': '抽奖活动',
      '/roomClockIn': '群打卡',
      '/roomFission': '群裂变活动',
      '/fissionSpeed': '群裂变进度',
      '/roomInfinitePull': '无限拉群',
      '/shopCode': '门店活码',
      '/workFission': '任务宝活动',
      '/speed': '任务宝进度',
    });
  });
});
