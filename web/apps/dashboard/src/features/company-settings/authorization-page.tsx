import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { Phase35PageShell } from '../phase35/components/phase35-page-shell';
import { Phase35DataState } from '../phase35/components/data-state';
import { createMenuAdminApi, type MenuNode } from '../menu-admin/menu-admin-api';

type MenuAdminApi = ReturnType<typeof createMenuAdminApi>;

function countNodes(nodes: MenuNode[]): number {
  return nodes.reduce((sum, node) => sum + 1 + countNodes(node.children ?? []), 0);
}

function OptionTree({ nodes, depth, onToggle }: { nodes: MenuNode[]; depth: number; onToggle: (id: number) => void }) {
  return (
    <ul style={{ marginLeft: depth * 18 }}>
      {nodes.map((node) => (
        <li key={node.menuId}>
          <span>{node.name}</span>
          <span className="phase35-chip">{node.status === 1 ? '已授权' : '已停用'}</span>
          <button type="button" onClick={() => onToggle(node.menuId)}>{node.status === 1 ? '停用' : '启用'}</button>
          {node.children?.length > 0 && <OptionTree nodes={node.children} depth={depth + 1} onToggle={onToggle} />}
        </li>
      ))}
    </ul>
  );
}

export function CompanyAuthorizationPage({ api }: { api: MenuAdminApi }) {
  const queryClient = useQueryClient();
  const [error, setError] = useState('');
  const query = useQuery({ queryKey: ['company-authorization'], queryFn: () => api.list({ name: '', page: 1, perPage: 200 }) });
  const flat = query.data?.list ?? [];
  const nodes = buildMenuTree(flat);
  const count = countNodes(nodes);
  const toggle = useMutation({
    mutationFn: (input: { id: number; next: number }) => api.updateStatus(input.id, input.next),
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: ['company-authorization'] }),
    onError: (err) => setError(err instanceof Error ? err.message : '切换失败'),
  });

  return (
    <Phase35PageShell title="授权管理" description="按菜单树查看授权状态；此处以“切换状态”演示授权开关（停用后该菜单不再授予）" actions={<span className="phase35-chip">共 {count} 个节点</span>}>
      <div className="phase35-page">
        <section className="phase35-card">
          <header className="phase35-card-header"><div><h2>菜单授权树</h2><p>企业微信后台菜单/功能授权来源</p></div></header>
          {error && <p role="alert" className="phase35-limits">{error}</p>}
          <Phase35DataState loading={query.isLoading} error={query.isError} empty={!nodes.length} emptyContent={<p className="phase35-empty">暂无授权菜单</p>} onRetry={() => void query.refetch()}>
            <OptionTree nodes={nodes} depth={0} onToggle={(id) => {
              const node = flat.find((item) => item.menuId === id);
              if (node) toggle.mutate({ id, next: node.status === 1 ? 0 : 1 });
            }} />
          </Phase35DataState>
        </section>
      </div>
    </Phase35PageShell>
  );
}

function buildMenuTree(flat: MenuNode[]): MenuNode[] {
  const byId = new Map<number, MenuNode>(flat.map((node) => [node.menuId, { ...node, children: [] }]));
  const roots: MenuNode[] = [];
  for (const node of byId.values()) {
    const parent = node.parentId > 0 ? byId.get(node.parentId) : undefined;
    if (parent) {
      parent.children = [...(parent.children ?? []), node];
    } else {
      roots.push(node);
    }
  }
  return roots;
}
