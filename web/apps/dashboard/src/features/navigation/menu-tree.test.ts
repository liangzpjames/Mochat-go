import { describe, expect, it } from 'vitest';

import { buildMenuAccess, type MenuNode } from './menu-tree';

function node(
  name: string,
  linkUrl: string | null,
  children: MenuNode[] = [],
  linkType: 1 | 2 = 1,
): MenuNode {
  return { name, icon: null, linkUrl, linkType, children };
}

describe('buildMenuAccess', () => {
  it('registers third-level pages as routes and deeper entries as actions', () => {
    const tree = [
      node('top', null, [
        node('section', null, [
          node('contacts', '/workContact/index', [
            node('create', '/workContact/create', [
              node('submit', '/workContact/create@submit'),
            ]),
          ]),
        ]),
      ]),
    ];

    const access = buildMenuAccess(tree);

    expect([...access.routes]).toEqual(['/workContact/index']);
    expect([...access.actions]).toEqual([
      '/workContact/create',
      '/workContact/create@submit',
    ]);
  });

  it('does not grant internal route access for external links', () => {
    const tree = [
      node('top', null, [
        node('section', null, [
          node('docs', 'https://docs.example.test', [], 2),
        ]),
      ]),
    ];

    expect([...buildMenuAccess(tree).routes]).toEqual([]);
  });

  it('ignores missing, invalid, and hidden-style links', () => {
    const tree = [
      node('top', null, [
        node('section', null, [
          node('missing', null),
          node('relative', 'workContact/index'),
          node('javascript', 'javascript:alert(1)'),
          node('hidden action', '/workContact/index@delete'),
        ]),
      ]),
    ];

    expect([...buildMenuAccess(tree).routes]).toEqual([]);
  });

  it('grants only routes present in the application route registry', () => {
    const tree = [
      node('top', null, [
        node('section', null, [
          node('registered', '/workContact/index'),
          node('hidden', '/workContact/hidden'),
        ]),
      ]),
    ];

    expect([
      ...buildMenuAccess(tree, new Set(['/workContact/index'])).routes,
    ]).toEqual(['/workContact/index']);
  });

  it('registers a known fourth-level page while preserving its action semantics', () => {
    const tree = [
      node('top', null, [
        node('section', null, [
          node('contacts', '/workContact/index', [
            node('create', '/workContact/create'),
          ]),
        ]),
      ]),
    ];
    const access = buildMenuAccess(
      tree,
      new Set(['/workContact/index', '/workContact/create']),
    );

    expect([...access.routes]).toEqual([
      '/workContact/index',
      '/workContact/create',
    ]);
    expect([...access.actions]).toEqual(['/workContact/create']);
  });

  it('deduplicates repeated actions and supports an empty menu', () => {
    const repeated = node('delete', '/workContact/index@delete');
    const tree = [
      node('top', null, [
        node('section', null, [
          node('contacts', '/workContact/index', [repeated, repeated]),
        ]),
      ]),
    ];

    expect([...buildMenuAccess(tree).actions]).toEqual([
      '/workContact/index@delete',
    ]);
    expect(buildMenuAccess([]).routes.size).toBe(0);
    expect(buildMenuAccess([]).actions.size).toBe(0);
  });
});
