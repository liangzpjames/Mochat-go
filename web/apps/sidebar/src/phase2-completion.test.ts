import { describe, expect, it } from 'vitest';
import manifest from './migration-routes.json';
import { sidebarBasename } from './deployment';
import { sidebarRoutes } from './app/catalog';

describe('Sidebar Phase 2 migration', () => {
  it.each(manifest)('$path is React and has functional content', (route) => {
    expect(route.target).toBe('react');
    expect(sidebarRoutes[route.path]).toBeDefined();
  });

  it('supports mounted and independent-port paths', () => {
    expect(sidebarBasename('/sidebar-app/contact')).toBe('/sidebar-app');
    expect(sidebarBasename('/contact')).toBe('/');
  });
});
