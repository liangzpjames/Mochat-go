import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';

import { MobileBottomNavigation } from './mobile-navigation';

afterEach(cleanup);

describe('MobileBottomNavigation', () => {
  it('renders the supplied navigation label and marks the current link', () => {
    render(
      <MobileBottomNavigation
        label="员工工作台"
        items={[
          { key: 'customers', label: '客户', icon: <span />, href: '/contact', current: true },
          { key: 'profile', label: '我的', icon: <span />, href: '/', current: false },
        ]}
      />,
    );

    expect(screen.getByRole('navigation', { name: '员工工作台' })).not.toBeNull();
    expect(screen.getByRole('link', { name: '客户' }).getAttribute('aria-current')).toBe('page');
    expect(screen.getByRole('link', { name: '我的' }).getAttribute('aria-current')).toBeNull();
  });

  it('uses the safe-area class for a fixed bottom navigation', () => {
    render(
      <MobileBottomNavigation
        label="员工工作台"
        items={[{ key: 'customers', label: '客户', icon: <span />, href: '/contact', current: true }]}
      />,
    );

    expect(
      screen.getByRole('navigation', { name: '员工工作台' }).classList.contains(
        'mobile-bottom-navigation--safe-area',
      ),
    ).toBe(true);
  });
});
