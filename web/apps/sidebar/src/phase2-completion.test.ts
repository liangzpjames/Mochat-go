import { describe, expect, it } from 'vitest';
import manifest from './migration-routes.json';
import { pageLoaders } from './page-loaders';
import { sidebarBasename } from './deployment';

describe('Sidebar Phase 2 migration', () => {
  it.each(manifest)('$path is React and dynamically imported', (route) => {
    expect(route.target).toBe('react');
    expect(pageLoaders[route.path]).toEqual(expect.any(Function));
  });

  it('supports mounted and independent-port paths', () => {
    expect(sidebarBasename('/sidebar-app/contact')).toBe('/sidebar-app');
    expect(sidebarBasename('/contact')).toBe('/');
  });
});
