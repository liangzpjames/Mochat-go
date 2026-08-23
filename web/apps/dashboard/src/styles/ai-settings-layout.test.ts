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

  it('分析助手使用单卡片两列用途布局，所有顶层操作按钮等高', () => {
    expect(block('.ai-assistant-card')).toContain('border-radius: 14px');
    expect(block('.ai-assistant-use-grid')).toContain('grid-template-columns: repeat(2, minmax(0, 1fr))');
    expect(block('.ai-settings-action-button')).toContain('min-height: 36px');
    expect(block('.ai-insight-query-actions button')).toContain('min-height: 36px');
  });
});
