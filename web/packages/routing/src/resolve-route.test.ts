import { describe, expect, it } from 'vitest';

import { parseRouteManifest, type MigrationRoute } from './manifest';
import { resolveRoute } from './resolve-route';

const routes = [
  {
    path: '/contacts',
    target: 'react',
    auth: true,
    corpContext: true,
    permission: 'contacts.read',
  },
  {
    path: '/reports/:reportId',
    target: 'legacy',
    auth: true,
    corpContext: false,
    permission: null,
  },
] as const satisfies readonly MigrationRoute[];

describe('resolveRoute', () => {
  it('returns the route whose static path matches exactly', () => {
    expect(resolveRoute('/contacts', routes)).toBe(routes[0]);
  });

  it('matches one pathname segment for a named parameter', () => {
    expect(resolveRoute('/reports/monthly', routes)).toBe(routes[1]);
    expect(resolveRoute('/reports/monthly/detail', routes)).toBeNull();
  });

  it('normalizes a trailing slash before matching', () => {
    expect(resolveRoute('/contacts/', routes)).toBe(routes[0]);
  });

  it('ignores query strings and hashes when matching', () => {
    expect(resolveRoute('/contacts?tab=active#top', routes)).toBe(routes[0]);
  });

  it('returns null for an unknown route', () => {
    expect(resolveRoute('/unknown', routes)).toBeNull();
  });

  it('fails closed when given an ambiguous manifest', () => {
    expect(resolveRoute('/reports/monthly', [
      routes[1],
      { ...routes[1], path: '/reports/:slug', target: 'react' },
    ])).toBeNull();
  });
});

describe('parseRouteManifest', () => {
  it('parses a valid route manifest at the configuration boundary', () => {
    expect(parseRouteManifest(routes)).toEqual(routes);
  });

  it('rejects duplicate route patterns', () => {
    expect(() => parseRouteManifest([routes[0], routes[0]])).toThrow(/duplicate route pattern/i);
  });

  it('rejects patterns made ambiguous by differently named parameters', () => {
    expect(() => parseRouteManifest([
      routes[1],
      { ...routes[1], path: '/reports/:slug' },
    ])).toThrow(/ambiguous route pattern/i);
  });
});
