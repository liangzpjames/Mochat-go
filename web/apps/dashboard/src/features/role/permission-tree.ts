import type { PermissionNode } from './role-api';

function setBranch(node: PermissionNode, checked: boolean): PermissionNode {
  return {
    ...node,
    checked: checked ? '2' : '1',
    children: node.children.map((child) => setBranch(child, checked)),
  };
}

function stateFromChildren(children: PermissionNode[]): PermissionNode['checked'] {
  if (children.every((child) => child.checked === '2')) {
    return '2';
  }
  if (children.every((child) => child.checked === '1')) {
    return '1';
  }
  return '3';
}

function updateNode(node: PermissionNode, id: number, checked: boolean): PermissionNode {
  if (node.id === id) {
    return setBranch(node, checked);
  }

  const children = node.children.map((child) => updateNode(child, id, checked));
  if (children.every((child, index) => child === node.children[index])) {
    return node;
  }

  return {
    ...node,
    children,
    checked: stateFromChildren(children),
  };
}

export function togglePermission(
  tree: PermissionNode[],
  id: number,
  checked: boolean,
): PermissionNode[] {
  return tree.map((node) => updateNode(node, id, checked));
}

export function collectCheckedPermissionIds(tree: PermissionNode[]): number[] {
  return tree.flatMap((node) => [
    ...(node.checked === '2' ? [node.id] : []),
    ...collectCheckedPermissionIds(node.children),
  ]);
}
