import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';

import { MobileCard, MobileIconTile } from './mobile-card';

afterEach(cleanup);

describe('MobileCard', () => {
  it('renders the requested visual tone and padding density', () => {
    render(
      <MobileCard tone="accent" padding="compact">
        重点
      </MobileCard>,
    );

    const card = screen.getByText('重点').closest('section');
    expect(card?.classList.contains('mobile-card--accent')).toBe(true);
    expect(card?.classList.contains('mobile-card--padding-compact')).toBe(true);
  });

  it('keeps the muted comfortable card semantic when no business content is supplied', () => {
    render(
      <MobileCard tone="muted" padding="comfortable">
        <span>基础内容</span>
      </MobileCard>,
    );

    const card = screen.getByText('基础内容').closest('section');
    expect(card?.classList.contains('mobile-card--muted')).toBe(true);
    expect(card?.classList.contains('mobile-card--padding-comfortable')).toBe(true);
  });
});

describe('MobileIconTile', () => {
  it('renders an enabled entry as a named link', () => {
    render(<MobileIconTile icon={<span />} title="客户" href="/contact" />);

    expect(screen.getByRole('link', { name: '客户' }).getAttribute('href')).toBe('/contact');
  });

  it('renders a disabled entry without an empty link', () => {
    render(
      <MobileIconTile
        icon={<span />}
        title="建设中"
        description="暂未开放"
        badge="即将推出"
        href="/contact"
        disabled
      />,
    );

    expect(screen.queryByRole('link', { name: '建设中' })).toBeNull();
    expect(screen.getByText('建设中').closest('[aria-disabled="true"]')).not.toBeNull();
    expect(screen.getByText('即将推出')).not.toBeNull();
  });
});
