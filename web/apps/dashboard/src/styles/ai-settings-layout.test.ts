import { readFileSync } from 'node:fs';

import { describe, expect, it } from 'vitest';

const stylesheet = readFileSync('src/styles/index.css', 'utf8');

function block(selector: string): string {
  const escaped = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const match = stylesheet.match(new RegExp(`(?:^|\\n)\\s*${escaped}\\s*\\{([^}]*)\\}`));
  expect(match, `missing ${selector} rule`).not.toBeNull();
  return match?.[1] ?? '';
}

describe('AI 设置响应式布局合同', () => {
  it('让工作台占满可用宽度并把横向滚动限制在表格内', () => {
    expect(block('.ai-settings-workspace')).toContain('width: 100%');
    expect(block('.ai-settings-table-scroll')).toContain('overflow-x: auto');
    expect(block('.ai-settings-table-scroll table')).toContain('min-width: 860px');
  });

  it('窄屏筛选换行，小高度弹窗只滚动内容区', () => {
    expect(stylesheet).toContain('.ai-settings-filter .dashboard-filter-panel__fields');
    expect(stylesheet).toContain('flex-direction: column');
    expect(block('.ai-settings-dialog-body')).toContain('overflow-y: auto');
    expect(block('.ai-settings-dialog-body')).toContain('max-height: calc(100dvh - 250px)');
  });

  it('交互 hover 不使用位移或滤镜', () => {
    const hover = block('.ai-settings-workspace button:hover:not(:disabled)');
    expect(hover).not.toContain('transform:');
    expect(hover).not.toContain('filter:');
  });

  it('双助手卡片在桌面并排、移动端改为单列，会话范围卡也遵守同样合同', () => {
    expect(block('.ai-assistant-card-grid')).toContain('grid-template-columns: repeat(2, minmax(0, 1fr))');
    expect(block('.ai-conversation-scope-grid')).toContain('grid-template-columns: repeat(2, minmax(0, 1fr))');
    expect(stylesheet).toContain('.ai-assistant-card-grid, .ai-conversation-scope-grid { grid-template-columns: 1fr; }');
  });

  it('知识库刷新按钮使用统一 secondary action 尺寸合同', () => {
    const secondary = block('.dashboard-secondary-action');
    expect(secondary).toContain('box-sizing: border-box');
    expect(secondary).toContain('min-height: 36px');
    expect(secondary).toContain('min-width: 68px');
    expect(secondary).toContain('padding: 7px 14px');
    expect(secondary).toContain('border-radius: 8px');
  });
});
