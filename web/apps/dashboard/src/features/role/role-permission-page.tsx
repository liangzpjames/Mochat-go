import { useMutation, useQuery } from '@tanstack/react-query';
import { Alert, Button, Card, Checkbox, Space, Spin } from 'antd';
import { useState } from 'react';
import { useSearchParams } from 'react-router';

import { useDashboardAccess } from '../../app/access-context';
import {
  collectCheckedPermissionIds,
  togglePermission,
} from './permission-tree';
import type { PermissionNode } from './role-api';

export type RolePermissionPageApi = {
  permissions(roleId: number): Promise<PermissionNode[]>;
  savePermissions(roleId: number, menuIds: number[]): Promise<void>;
};

function PermissionBranch({
  nodes,
  onToggle,
}: {
  nodes: PermissionNode[];
  onToggle: (id: number, checked: boolean) => void;
}) {
  return (
    <ul>
      {nodes.map((node) => (
        <li key={node.id}>
          <Checkbox
            aria-label={node.name}
            checked={node.checked === '2'}
            indeterminate={node.checked === '3'}
            onChange={(event) => onToggle(node.id, event.target.checked)}
          >
            {node.name}
          </Checkbox>
          {node.children.length > 0 && (
            <PermissionBranch nodes={node.children} onToggle={onToggle} />
          )}
        </li>
      ))}
    </ul>
  );
}

export function RolePermissionPage({
  api,
  navigate,
}: {
  api: RolePermissionPageApi;
  navigate: (path: string) => void;
}) {
  const access = useDashboardAccess();
  const [search] = useSearchParams();
  const roleId = Number(search.get('roleId'));
  const validRoleId = Number.isInteger(roleId) && roleId > 0;
  const [tree, setTree] = useState<PermissionNode[] | null>(null);
  const query = useQuery({
    queryKey: ['role-permissions', access.corp.id, roleId],
    queryFn: () => api.permissions(roleId),
    enabled: validRoleId,
  });
  const permissions = tree ?? query.data ?? [];
  const mutation = useMutation({
    mutationFn: () => api.savePermissions(roleId, collectCheckedPermissionIds(permissions)),
    onSuccess: () => navigate('/role/index'),
  });

  if (!validRoleId) {
    return <Alert type="error" title="缺少角色编号" />;
  }

  return (
    <Card
      title="权限设置"
      extra={(
        <Space>
          <Button onClick={() => navigate('/role/index')}>取消</Button>
          {(access.allowedActions.size === 0 || access.allowedActions.has('/role/permissionShow@save')) && (
            <Button
              type="primary"
              loading={mutation.isPending}
              disabled={permissions.length === 0}
              onClick={() => mutation.mutate()}
            >
              保存权限
            </Button>
          )}
        </Space>
      )}
    >
      {query.isPending && <Spin aria-label="正在加载权限" />}
      {query.isError && <Alert type="error" title="权限加载失败" />}
      {query.isSuccess && (
        <PermissionBranch
          nodes={permissions}
          onToggle={(id, checked) => setTree(togglePermission(permissions, id, checked))}
        />
      )}
    </Card>
  );
}
