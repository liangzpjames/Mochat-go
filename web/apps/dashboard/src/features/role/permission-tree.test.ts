import { describe, expect, it } from 'vitest';

import { collectCheckedPermissionIds, togglePermission } from './permission-tree';
import type { PermissionNode } from './role-api';

const tree: PermissionNode[] = [
  {
    id: 1,
    name: '客户管理',
    checked: '3',
    children: [
      { id: 2, name: '客户列表', checked: '2', children: [] },
      { id: 3, name: '删除客户', checked: '1', children: [] },
    ],
  },
];

describe('permission tree', () => {
  it('collects only fully checked permission ids', () => {
    expect(collectCheckedPermissionIds(tree)).toEqual([2]);
  });

  it('checks descendants and recalculates ancestor state immutably', () => {
    const checked = togglePermission(tree, 3, true);

    expect(collectCheckedPermissionIds(checked)).toEqual([1, 2, 3]);
    expect(checked[0]?.checked).toBe('2');
    expect(tree[0]?.checked).toBe('3');
  });

  it('clears all descendants when a parent is unchecked', () => {
    const unchecked = togglePermission(tree, 1, false);

    expect(collectCheckedPermissionIds(unchecked)).toEqual([]);
    expect(unchecked[0]?.children.every((item) => item.checked === '1')).toBe(true);
  });
});
