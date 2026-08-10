/* eslint-disable @typescript-eslint/no-base-to-string */
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { ConfirmAction } from '../../components/confirm-action';
import { Phase35PageShell } from '../phase35/components/phase35-page-shell';
import { Phase35DataState } from '../phase35/components/data-state';
import { createMenuAdminApi, type MenuNode } from '../menu-admin/menu-admin-api';
import type { Page } from '../access/access-admin-api';

type MenuAdminApi = ReturnType<typeof createMenuAdminApi>;

const authorizationNameMap: Record<string, string> = {
  'Friends circle task query': '朋友圈任务查询',
  'Friends circle material query': '朋友圈素材查询',
  'Friends circle task draft': '朋友圈任务草稿',
  'Friends circle material create': '朋友圈素材创建',
  'Friends circle publish': '朋友圈发布',
};

function authorizationName(node: MenuNode): string {
  return authorizationNameMap[node.name]
    ?? (/\?{2,}|provider/i.test(node.name) ? `未命名权限（${node.menuId}）` : node.name || `未命名权限（${node.menuId}）`);
}

function countNodes(nodes: MenuNode[]): number {
  return nodes.reduce((sum, node) => sum + 1 + countNodes(node.children ?? []), 0);
}

function OptionTree({ nodes, depth, onToggle }: { nodes: MenuNode[]; depth: number; onToggle: (id: number) => void }) {
  return (
    <ul style={{ marginLeft: depth * 18 }}>
      {nodes.map((node) => (
        <li key={node.menuId}>
          <span>{authorizationName(node)}</span>
          <span className="phase35-chip">{node.status === 1 ? '已授权' : '已停用'}</span>
          <ConfirmAction
            title={node.status === 1 ? `确认停用授权节点“${authorizationName(node)}”？` : `确认启用授权节点“${authorizationName(node)}”？`}
            {...(node.status === 1 ? { description: '停用后该菜单不再授予当前企业用户。' } : {})}
            onConfirm={() => onToggle(node.menuId)}
          >
            <button type="button">{node.status === 1 ? `停用 ${authorizationName(node)}` : `启用 ${authorizationName(node)}`}</button>
          </ConfirmAction>
          {node.children?.length > 0 && <OptionTree nodes={node.children} depth={depth + 1} onToggle={onToggle} />}
        </li>
      ))}
    </ul>
  );
}

function LegacyCompanyAuthorizationPage({ api }: { api: MenuAdminApi }) {
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

type AuditApi = { audits: (input: { page: number; perPage: number }) => Promise<Page<Record<string, unknown>>> };
function AuditPage({ api }: { api: AuditApi }) {
  const query = useQuery({ queryKey: ['dashboard-access-audits'], queryFn: () => api.audits({ page: 1, perPage: 50 }) });
  return <Phase35PageShell title="权限审计" description="同事务记录的授权变更与 expectedVersion 结果"><div className="phase35-page"><section className="phase35-card phase35-table-card"><table><thead><tr><th>时间</th><th>操作者</th><th>动作</th><th>目标</th><th>请求</th><th>结果版本</th></tr></thead><tbody>{(query.data?.list ?? []).map((audit, index) => <tr key={String(audit.id ?? index)}><td>{String(audit.time ?? '')}</td><td>{String(audit.actorName ?? audit.actorUserId ?? '')}</td><td>{String(audit.action ?? '')}</td><td>{String(audit.targetType ?? '')} {String(audit.targetId ?? '')}</td><td>{String(audit.requestId ?? '')}</td><td>{String(audit.resultVersion ?? '')}</td></tr>)}</tbody></table></section></div></Phase35PageShell>;
}

export function CompanyAuthorizationPage({ api }: { api: MenuAdminApi | AuditApi }) {
  return 'audits' in api ? <AuditPage api={api} /> : <LegacyCompanyAuthorizationPage api={api} />;
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
