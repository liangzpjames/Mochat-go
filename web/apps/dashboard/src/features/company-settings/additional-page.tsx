import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { ConfirmAction } from '../../components/confirm-action';
import { Phase35PageShell } from '../phase35/components/phase35-page-shell';
import { Phase35DataState } from '../phase35/components/data-state';
import { createMenuAdminApi, type MenuNode } from '../menu-admin/menu-admin-api';

type MenuAdminApi = ReturnType<typeof createMenuAdminApi>;

export function CompanyAdditionalPage({ api }: { api: MenuAdminApi }) {
  const queryClient = useQueryClient();
  const [keyword, setKeyword] = useState('');
  const [page, setPage] = useState(1);
  const [editing, setEditing] = useState<MenuNode | null>(null);
  const [creating, setCreating] = useState(false);
  const [name, setName] = useState('');
  const [linkUrl, setLinkUrl] = useState('');
  const [level, setLevel] = useState(1);
  const [icon, setIcon] = useState('');
  const [error, setError] = useState('');

  const query = useQuery({ queryKey: ['company-additional', keyword, page], queryFn: () => api.list({ name: keyword, page, perPage: 20 }) });
  const items = query.data?.list ?? [];
  const total = query.data?.page?.total ?? 0;
  const totalPage = query.data?.page?.totalPage ?? 1;

  const save = useMutation({
    mutationFn: () => editing
      ? api.update(editing.menuId, { level, name: name.trim(), icon, linkUrl: linkUrl.trim(), linkType: 1 })
      : api.create({ level, name: name.trim(), icon, linkUrl: linkUrl.trim(), linkType: 1 }),
    onSuccess: () => { setEditing(null); setCreating(false); setError(''); void queryClient.invalidateQueries({ queryKey: ['company-additional'] }); },
    onError: (err) => setError(err instanceof Error ? err.message : '保存失败'),
  });
  const toggleStatus = useMutation({ mutationFn: (input: { id: number; next: number }) => api.updateStatus(input.id, input.next), onSuccess: () => void queryClient.invalidateQueries({ queryKey: ['company-additional'] }) });
  const remove = useMutation({ mutationFn: (id: number) => api.remove(id), onSuccess: () => void queryClient.invalidateQueries({ queryKey: ['company-additional'] }) });

  const openCreate = () => { setCreating(true); setEditing(null); setName(''); setLinkUrl(''); setLevel(1); setIcon(''); setError(''); };
  const openEdit = (item: MenuNode) => { setEditing(item); setCreating(false); setName(item.name); setLinkUrl(item.menuPath); setLevel(item.level); setIcon(item.icon); setError(''); };
  const valid = Boolean(name.trim().length >= 2);

  return (
    <Phase35PageShell title="附加权限" description="维护菜单/功能点，作为附加权限的来源" actions={<button type="button" onClick={openCreate}>新增菜单</button>}>
      <div className="phase35-page">
        <section className="phase35-card phase35-filter-card">
          <form className="dashboard-filter-bar" onSubmit={(event) => { event.preventDefault(); setPage(1); void query.refetch(); }}>
            <input aria-label="菜单名称" value={keyword} onChange={(event) => setKeyword(event.target.value)} placeholder="按菜单名称筛选" />
            <button type="submit">查询</button>
            <button type="button" onClick={() => setKeyword('')}>重置</button>
          </form>
        </section>

        <section className="phase35-card phase35-table-card">
          <header className="phase35-card-header"><div><h2>菜单/功能点</h2><p>附加权限的载体</p></div><span className="phase35-chip">共 {total} 条，第 {page}/{totalPage} 页</span></header>
          <Phase35DataState loading={query.isLoading} error={query.isError} empty={!items.length} emptyContent={<p className="phase35-empty">暂无菜单</p>} onRetry={() => void query.refetch()}>
            <div className="phase35-table">
              <table>
                <thead><tr><th>名称</th><th>层级</th><th>路径</th><th>状态</th><th>更新时间</th><th>操作</th></tr></thead>
                <tbody>
                  {items.map((item) => (
                    <tr key={item.menuId}>
                      <td>{item.icon ? `${item.icon} ` : ''}{item.name}</td><td>{item.levelName}</td><td>{item.menuPath}</td><td>{item.status === 1 ? '启用' : '停用'}</td><td>{item.updatedAt}</td>
                      <td>
                        <button type="button" onClick={() => openEdit(item)}>编辑</button>
                        <ConfirmAction title={item.status === 1 ? `确认停用菜单“${item.name}”？` : `确认启用菜单“${item.name}”？`} onConfirm={() => toggleStatus.mutate({ id: item.menuId, next: item.status === 1 ? 0 : 1 })}>
                          <button type="button">{item.status === 1 ? `停用 ${item.name}` : `启用 ${item.name}`}</button>
                        </ConfirmAction>
                        <ConfirmAction title={`确认删除菜单“${item.name}”？`} description="删除后引用该菜单的授权将失效。" onConfirm={() => remove.mutate(item.menuId)}>
                          <button type="button">{`删除 ${item.name}`}</button>
                        </ConfirmAction>
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

        {(creating || editing) && (
          <section className="phase35-card" role="dialog" aria-label="菜单表单">
            <header className="phase35-card-header"><div><h2>{editing ? '编辑菜单' : '新增菜单'}</h2></div></header>
            {error && <p role="alert" className="phase35-limits">{error}</p>}
            <form onSubmit={(event) => { event.preventDefault(); if (valid) save.mutate(); }}>
              <label>名称<input value={name} onChange={(event) => setName(event.target.value)} /></label>
              <label>层级<select value={level} onChange={(event) => setLevel(Number(event.target.value))}><option value={1}>一级</option><option value={2}>二级</option><option value={3}>三级</option></select></label>
              <label>路径<input value={linkUrl} onChange={(event) => setLinkUrl(event.target.value)} /></label>
              <label>图标<input value={icon} onChange={(event) => setIcon(event.target.value)} /></label>
              <button type="submit" disabled={!valid || save.isPending}>保存</button>
              <button type="button" onClick={() => { setEditing(null); setCreating(false); setError(''); }}>取消</button>
            </form>
          </section>
        )}
      </div>
    </Phase35PageShell>
  );
}
