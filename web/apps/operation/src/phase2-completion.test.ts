import { describe, expect, it } from 'vitest';
import manifest from './migration-routes.json';
import { operationBasename } from './deployment';
import { operationRouteRegistry } from './routes/registry';

describe('Operation Phase 2 migration', () => {
  it.each(manifest)('$path is React and has an explicit activity module', (route) => {
    expect(route.target).toBe('react');
    expect(operationRouteRegistry.find((registered) => registered.path === route.path))
      .toBeDefined();
  });

  it('supports mounted and independent-port paths', () => {
    expect(operationBasename('/operation-app/workFission')).toBe('/operation-app');
    expect(operationBasename('/workFission')).toBe('/');
  });
});
