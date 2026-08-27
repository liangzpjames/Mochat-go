import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

const css = readFileSync(resolve(process.cwd(), 'src/styles/index.css'), 'utf8');

function luminance(hex: string): number {
  const channels = hex.match(/[a-f\d]{2}/gi)?.map((value) => Number.parseInt(value, 16) / 255) ?? [];
  const linear = channels.map((value) => value <= .04045 ? value / 12.92 : ((value + .055) / 1.055) ** 2.4);
  return .2126 * (linear[0] ?? 0) + .7152 * (linear[1] ?? 0) + .0722 * (linear[2] ?? 0);
}

describe('phase35 empty state layout contract', () => {
  it('centers the empty state without creating a fixed-width mobile overflow', () => {
    const block = css.match(/\.phase35-empty-state\s*\{([^}]*)\}/)?.[1] ?? '';
    expect(block).toMatch(/display:\s*grid/);
    expect(block).toMatch(/justify-items:\s*center/);
    expect(block).toMatch(/text-align:\s*center/);
    expect(block).toMatch(/width:\s*100%/);
    expect(block).not.toMatch(/min-width|width:\s*\d+px/);
  });

  it('keeps empty-state body copy at WCAG AA contrast on white cards', () => {
    const block = css.match(/\.phase35-empty-state p,[\s\S]*?\{([^}]*)\}/)?.[1] ?? '';
    const foreground = block.match(/color:\s*(#[a-f\d]{6})/i)?.[1] ?? '#ffffff';
    const contrast = (luminance('#ffffff') + .05) / (luminance(foreground) + .05);
    expect(contrast).toBeGreaterThanOrEqual(4.5);
  });
});
