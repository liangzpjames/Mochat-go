import { describe, expect, it } from 'vitest';

import {
  buildYuanhuNavigation,
  buildYuanhuTopLevelNavigation,
  type YuanhuManifest,
} from './yuanhu-navigation';

const manifest: YuanhuManifest = {
  groups: [
    { id: 'conversation', title: '会话' },
    { id: 'risk-warning', title: '风险预警' },
    { id: 'ai-insight', title: 'AI 洞察' },
    { id: 'marketing-tools', title: '营销工具' },
    { id: 'scrm', title: 'SCRM' },
    { id: 'data-reports', title: '数据报表' },
    { id: 'ai-settings', title: 'AI 设置' },
    { id: 'company-settings', title: '企业设置' },
  ],
  pages: [
    { path: '/chat/v2-all', title: '全部消息', groupId: 'conversation' },
    { path: '/chat/v2-staff', title: '员工会话', groupId: 'conversation' },
    { path: '/chat/v2-all', title: '重复页面', groupId: 'conversation' },
    { path: '/ai-insight/v2/risk', title: '风险行为', groupId: 'risk-warning' },
    { path: '/index', title: '数据概览', groupId: null },
  ],
};

describe('buildYuanhuNavigation', () => {
  it('preserves manifest group order, filters unauthorized routes, and de-duplicates paths', () => {
    const navigation = buildYuanhuNavigation({
      allowedRoutes: new Set(['/chat/v2-all', '/ai-insight/v2/risk']),
      pathname: '/chat/v2-all',
    }, manifest);

    expect(navigation.map((group) => group.title)).toEqual([
      '会话', '风险预警', 'AI 洞察', '营销工具', 'SCRM', '数据报表', 'AI 设置', '企业设置',
    ]);
    expect(navigation[0]?.items).toEqual([{ title: '全部消息', path: '/chat/v2-all', activePath: '/chat/v2-all' }]);
    expect(navigation[1]?.items).toEqual([{ title: '风险行为', path: '/ai-insight/v2/risk', activePath: null }]);
    expect(navigation.slice(2).every((group) => group.items.length === 0)).toBe(true);
  });

  it('returns a stable empty tree when access is unavailable', () => {
    expect(buildYuanhuNavigation(null, manifest)).toEqual([]);
  });

  it('returns authorized ungrouped pages as top-level navigation items', () => {
    expect(buildYuanhuTopLevelNavigation({
      allowedRoutes: new Set(['/index']),
      pathname: '/index',
    }, manifest)).toEqual([
      { title: '数据概览', path: '/index', activePath: '/index' },
    ]);
  });
});
