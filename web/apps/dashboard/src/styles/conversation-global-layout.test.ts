import { readFileSync } from 'node:fs';

import { describe, expect, it } from 'vitest';

const css = readFileSync('src/styles/index.css', 'utf8');

describe('全局消息 27 寸宽屏工作台布局', () => {
  it('在宽内容区铺开八项指标和四列会话卡片', () => {
    expect(css).toContain('@media (min-width: 1600px)');
    expect(css).toContain('.conversation-global-page.is-global-workbench .conversation-global-overview { grid-template-columns: repeat(8, minmax(0, 1fr)); }');
    expect(css).toContain('.conversation-global-page.is-global-workbench .conversation-global-cards { grid-template-columns: repeat(4, minmax(0, 1fr)); }');
  });

  it('为窄窗口保留两列和单列降级，页面不依赖横向滚动', () => {
    expect(css).toContain('@media (min-width: 721px) and (max-width: 1199px)');
    expect(css).toContain('.conversation-global-page.is-global-workbench .conversation-global-cards { grid-template-columns: repeat(2, minmax(0, 1fr)); }');
    expect(css).toContain('.conversation-global-page.is-global-workbench .conversation-global-cards { grid-template-columns: 1fr; }');
    expect(css).not.toContain('.conversation-global-card:hover { transform:');
    expect(css).not.toContain('.conversation-global-card:hover { filter:');
  });
});
