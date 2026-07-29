import { describe, expect, it } from 'vitest';
import manifest from './migration-routes.json';
import { pageLoaders } from './page-loaders';
import { operationBasename } from './deployment';

describe('Operation Phase 2 migration', () => {
  it.each(manifest)('$path is React and dynamically imported', (route) => {
    expect(route.target).toBe('react');
    expect(pageLoaders[route.path]).toEqual(expect.any(Function));
  });

  it('supports mounted and independent-port paths', () => {
    expect(operationBasename('/operation-app/workFission')).toBe('/operation-app');
    expect(operationBasename('/workFission')).toBe('/');
  });
});
