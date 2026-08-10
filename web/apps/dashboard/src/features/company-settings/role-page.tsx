/* eslint-disable @typescript-eslint/no-unused-vars */
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useRef, useState } from 'react';
import { Phase35PageShell } from '../phase35/components/phase35-page-shell';
import { Phase35DataState } from '../phase35/components/data-state';
import { ConfirmAction } from '../../components/confirm-action';
import { DashboardDialog } from '../../components/dashboard-dialog';
import { createRoleApi, type PermissionNode, type RoleItem } from '../role/role-api';
import type { AccessRole } from '../access/access-admin-api';
import type { AccessCatalogItem } from '../access/access-api';
import { AccessRolePage as NewAccessRolePage } from './access-role-page';

type RoleApi = ReturnType<typeof createRoleApi>;
const permissionNameMap: Record<string, string> = { 'Friends circle': '朋友圈' };
const permissionName = (node: PermissionNode) => permissionNameMap[node.name] ?? (/\?{2,}|provider/i.test(node.name) ? `未命名权限（${node.id}）` : node.name || `未命名权限（${node.id}）`);

function PermissionTree({ nodes, selected, onToggle }: { nodes: PermissionNode[]; selected: Set<number>; onToggle: (id: number) => void }) {
  return (
    <ul>
      {nodes.map((node) => (
        <li key={node.id}>
          <label><input type="checkbox" checked={selected.has(node.id)} onChange={() => onToggle(node.id)} />{permissionName(node)}</label>
          {node.children?.length > 0 && <PermissionTree nodes={node.children} selected={selected} onToggle={onToggle} />}
        </li>
      ))}
    </ul>
  );
}

function LegacyCompanyRolePage({ api }: { api: RoleApi }) {
  const queryClient = useQueryClient();
  const [keyword, setKeyword] = useState('');
  const [page, setPage] = useState(1);
  const [editing, setEditing] = useState<RoleItem | null>(null);
  const [creating, setCreating] = useState(false);
  const [name, setName] = useState('');
  const [remarks, setRemarks] = useState('');
  const [permissionTarget, setPermissionTarget] = useState<RoleItem | null>(null);
  const [selected, setSelected] = useState<Set<number>>(new Set());
  const [error, setError] = useState('');
  const triggerRef = useRef<HTMLElement | null>(null);

  const query = useQuery({ queryKey: ['company-role', keyword, page], queryFn: () => api.list({ name: keyword, page, perPage: 20 }) });
  const items = query.data?.list ?? [];
  const total = query.data?.page?.total ?? 0;
  const totalPage = query.data?.page?.totalPage ?? 1;
  const permissions = useQuery({ queryKey: ['role-permissions', permissionTarget?.roleId], queryFn: () => api.permissions(Number(permissionTarget?.roleId)), enabled: Boolean(permissionTarget) });
  const collect = (nodes: PermissionNode[], into: Set<number>) => { for (const node of nodes) { if (node.checked === '1' || node.checked === '2') into.add(node.id); if (node.children) collect(node.children, into); } return into; };

  const save = useMutation({
    mutationFn: () => editing ? api.update(editing.roleId, { name: name.trim(), remarks: remarks.trim(), dataPermission: 1 }) : api.create({ name: name.trim(), remarks: remarks.trim(), dataPermission: 1 }),
    onSuccess: () => { setEditing(null); setCreating(false); setError(''); void queryClient.invalidateQueries({ queryKey: ['company-role'] }); },
    onError: (err) => setError(err instanceof Error ? err.message : '保存失败'),
  });
  const savePermissions = useMutation({
    mutationFn: () => api.savePermissions(Number(permissionTarget?.roleId), [...selected]),
    onSuccess: () => setPermissionTarget(null),
  });
  const remove = useMutation({ mutationFn: (roleId: number) => api.remove(roleId), onSuccess: () => void queryClient.invalidateQueries({ queryKey: ['company-role'] }) });
  const toggleStatus = useMutation({ mutationFn: (input: { roleId: number; next: number }) => api.updateStatus(input.roleId, input.next), onSuccess: () => void queryClient.invalidateQueries({ queryKey: ['company-role'] }) });

  const openCreate = () => { setCreating(true); setEditing(null); setName(''); setRemarks(''); setError(''); };
  const openEdit = (item: RoleItem) => { setEditing(item); setCreating(false); setName(item.name); setRemarks(item.remarks ?? ''); setError(''); };
  const openPermissions = (item: RoleItem) => { setPermissionTarget(item); void permissions.refetch(); };
  const valid = Boolean(name.trim().length >= 2);
  const closeEditor = () => { setEditing(null); setCreating(false); setError(''); };
  const closePermissions = () => { setPermissionTarget(null); setSelected(new Set()); };

  return (
    <Phase35PageShell title="角色管理" description="角色增删改查与菜单权限勾选" actions={<button type="button" onClick={(event) => { triggerRef.current = event.currentTarget; openCreate(); }}>新建角色</button>}>
      <div className="phase35-page">
        <section className="phase35-card phase35-filter-card">
          <form className="dashboard-filter-bar" onSubmit={(event) => { event.preventDefault(); setPage(1); void query.refetch(); }}>
            <input aria-label="角色名称" value={keyword} onChange={(event) => setKeyword(event.target.value)} placeholder="按角色名称筛选" />
            <button type="submit">查询</button>
            <button type="button" onClick={() => setKeyword('')}>重置</button>
          </form>
        </section>

        <section className="phase35-card phase35-table-card">
          <header className="phase35-card-header"><div><h2>角色列表</h2><p>创建、编辑、权限配置与停用</p></div><span className="phase35-chip">共 {total} 条，第 {page}/{totalPage} 页</span></header>
          <Phase35DataState loading={query.isLoading} error={query.isError} empty={!items.length} emptyContent={<p className="phase35-empty">暂无角色</p>} onRetry={() => void query.refetch()}>
            <div className="phase35-table">
              <table>
                <thead><tr><th>角色名</th><th>备注</th><th>员工数</th><th>状态</th><th>更新时间</th><th>操作</th></tr></thead>
                <tbody>
                  {items.map((item) => (
                    <tr key={item.roleId}>
                      <td>{item.name}</td><td>{item.remarks || '—'}</td><td>{item.employeeNum}</td><td>{item.status === 1 ? '启用' : '停用'}</td><td>{item.updatedAt}</td>
                      <td>
                        <button type="button" onClick={(event) => { triggerRef.current = event.currentTarget; openPermissions(item); }}>权限</button>
                        <button type="button" onClick={(event) => { triggerRef.current = event.currentTarget; openEdit(item); }}>编辑</button>
                        <ConfirmAction title={item.status === 1 ? `确认停用角色“${item.name}”？` : `确认启用角色“${item.name}”？`} onConfirm={() => toggleStatus.mutate({ roleId: item.roleId, next: item.status === 1 ? 0 : 1 })}><button type="button">{item.status === 1 ? '停用' : '启用'}</button></ConfirmAction>
                        <ConfirmAction title={`确认删除角色“${item.name}”？`} onConfirm={() => remove.mutate(item.roleId)}><button type="button">删除</button></ConfirmAction>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <div className="dashboard-pagination">
              <button type="button" disabled={page <= 1} onClick={() => setPage((value) => value - 1)}>上一页</button>
              <button type="button" disabled={page >= totalPage} onClick={() => setPage((value) => value + 1)}>下一页</button>
            </div>
          </Phase35DataState>
        </section>

        <DashboardDialog open={creating || editing !== null} title={editing ? '编辑角色' : '新建角色'} triggerRef={triggerRef} confirmDisabled={!valid} confirmLoading={save.isPending} onCancel={closeEditor} onConfirm={() => save.mutate()}>
            {error && <p role="alert" className="phase35-limits">{error}</p>}
            <form onSubmit={(event) => { event.preventDefault(); if (valid) save.mutate(); }}>
              <label>角色名<input value={name} onChange={(event) => setName(event.target.value)} /></label>
              <label>备注<input value={remarks} onChange={(event) => setRemarks(event.target.value)} /></label>
            </form>
        </DashboardDialog>

        <DashboardDialog open={permissionTarget !== null} title={permissionTarget ? `“${permissionTarget.name}”的菜单权限` : '角色权限'} triggerRef={triggerRef} confirmText="保存权限" confirmLoading={savePermissions.isPending} onCancel={closePermissions} onConfirm={() => savePermissions.mutate()}>
            <p>勾选后保存，角色所辖员工登录时按此过滤菜单</p>
            {permissions.isLoading && <p>正在加载权限…</p>}
            {permissions.isError && <p role="alert" className="phase35-limits">权限加载失败</p>}
            {permissions.data && (
              <PermissionTree
                nodes={permissions.data}
                selected={selected}
                onToggle={(id) => setSelected((current) => { const next = new Set(current); if (next.has(id)) next.delete(id); else next.add(id); return next; })}
              />
            )}
            <button type="button" onClick={() => { setSelected(collect(permissions.data ?? [], new Set())); }}>全选当前</button>
        </DashboardDialog>
      </div>
    </Phase35PageShell>
  );
}

type AccessRoleApi = {
  roles: (input: { page: number; perPage: number }) => Promise<{ list: AccessRole[]; page: { total: number; totalPage: number } }>;
  createRole: (input: { name: string; remark: string; status: number; permissions: { code: string; scope: string }[] }) => Promise<AccessRole>;
  updateRole: (id: number, input: { name: string; remark: string; permissions: { code: string; scope: string }[]; expectedVersion: number }) => Promise<AccessRole>;
  updateRoleStatus: (id: number, input: { status: number; expectedVersion: number }) => Promise<AccessRole>;
  deleteRole: (id: number, expectedVersion: number) => Promise<unknown>;
  catalog: () => Promise<AccessCatalogItem[]>;
};

function AccessRolePage({ api }: { api: AccessRoleApi }) {
  const queryClient = useQueryClient();
  const [editing, setEditing] = useState<AccessRole | null>(null);
  const [name, setName] = useState('');
  const [remark, setRemark] = useState('');
  const [error, setError] = useState('');
  const [permissionCodes, setPermissionCodes] = useState<string[]>([]);
  const query = useQuery({ queryKey: ['dashboard-access-roles'], queryFn: () => api.roles({ page: 1, perPage: 50 }) });
  const catalog = useQuery({ queryKey: ['dashboard-access-catalog-for-role'], queryFn: api.catalog });
  const save = useMutation({ mutationFn: () => editing ? api.updateRole(editing.id, { name: name.trim(), remark: remark.trim(), permissions: permissionCodes.map((code) => ({ code, scope: 'self' })), expectedVersion: editing.version }) : api.createRole({ name: name.trim(), remark: remark.trim(), status: 1, permissions: permissionCodes.map((code) => ({ code, scope: 'self' })) }), onSuccess: () => { setEditing(null); void queryClient.invalidateQueries({ queryKey: ['dashboard-access-roles'] }); }, onError: (reason) => setError(reason instanceof Error ? reason.message : '保存失败') });
  const toggle = useMutation({ mutationFn: (role: AccessRole) => api.updateRoleStatus(role.id, { status: role.status === 1 ? 2 : 1, expectedVersion: role.version }), onSuccess: () => void queryClient.invalidateQueries({ queryKey: ['dashboard-access-roles'] }) });
  const remove = useMutation({ mutationFn: (role: AccessRole) => api.deleteRole(role.id, role.version), onSuccess: () => void queryClient.invalidateQueries({ queryKey: ['dashboard-access-roles'] }), onError: (reason) => setError(reason instanceof Error ? reason.message : '删除失败（角色可能仍有成员）') });
  return <Phase35PageShell title="角色管理" description="角色完整 CRUD、启停与成员约束" actions={<button type="button" onClick={() => { setEditing({ id: 0, name: '', remark: '', status: 1, isSystem: false, memberCount: 0, permissions: [], version: 1 }); setName(''); setRemark(''); setPermissionCodes([]); }}>新建角色</button>}><div className="phase35-page"><section className="phase35-card phase35-table-card"><table><thead><tr><th>角色</th><th>状态</th><th>成员</th><th>操作</th></tr></thead><tbody>{(query.data?.list ?? []).map((role) => <tr key={role.id}><td>{role.name}<br /><small>{role.remark}</small></td><td>{role.status === 1 ? '启用' : '停用'}</td><td>{role.memberCount}</td><td><button type="button" onClick={() => { setEditing(role); setName(role.name); setRemark(role.remark); setPermissionCodes(role.permissions.map((permission) => permission.code)); }}>编辑</button><ConfirmAction title={`${role.status === 1 ? '停用' : '启用'}角色“${role.name}”？`} onConfirm={() => toggle.mutate(role)}><button type="button">{role.status === 1 ? '停用' : '启用'}</button></ConfirmAction><ConfirmAction title={`确认删除角色“${role.name}”？`} {...(role.memberCount > 0 ? { description: '有成员时服务端将返回 409。' } : {})} onConfirm={() => remove.mutate(role)}><button type="button">删除</button></ConfirmAction></td></tr>)}</tbody></table>{error && <p role="alert">{error}</p>}</section>{editing && <DashboardDialog open title={editing.id ? '编辑角色' : '新建角色'} confirmText="保存" confirmLoading={save.isPending} onCancel={() => setEditing(null)} onConfirm={() => save.mutate()}><label>角色名称<input value={name} onChange={(event) => setName(event.target.value)} /></label><label>备注<input value={remark} onChange={(event) => setRemark(event.target.value)} /></label><fieldset><legend>页面权限</legend>{(catalog.data ?? []).map((item) => <label key={item.code}><input type="checkbox" checked={permissionCodes.includes(item.code)} disabled={item.superadminOnly} onChange={() => setPermissionCodes((current) => current.includes(item.code) ? current.filter((code) => code !== item.code) : [...current, item.code])} />{item.name}</label>)}</fieldset></DashboardDialog>}</div></Phase35PageShell>;
}

export function CompanyRolePage({ api }: { api: RoleApi | AccessRoleApi }) {
  return 'roles' in api ? <NewAccessRolePage api={api as never} /> : <LegacyCompanyRolePage api={api} />;
}
