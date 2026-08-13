import { describe, expect, it } from 'vitest';

import { dashboardLandingRoute } from './router';

describe('dashboardLandingRoute', () => {
  it('prefers overview and otherwise opens the first authorized route', () => {
    expect(dashboardLandingRoute(new Set(['/chat/v2-all', '/index']))).toBe('/index');
    expect(dashboardLandingRoute(new Set(['/chat/v2-all', '/customer/contact']))).toBe('/chat/v2-all');
    expect(dashboardLandingRoute(new Set())).toBeNull();
  });
});
