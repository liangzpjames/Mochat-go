import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { Phase35PageShell } from '../phase35/components/phase35-page-shell';
import { Phase35DataState } from '../phase35/components/data-state';
import { createCorpAdminApi, type CorpDetail, type CorpListInput } from '../corp/corp-admin-api';

type CorpAdminApi = ReturnType<typeof createCorpAdminApi>;

export function CompanyWebsitePage({ api }: { api: CorpAdminApi }) {
  const queryClient = useQueryClient();
  const [keyword, setKeyword] = useState('');
  const [editing, setEditing] = useState<CorpDetail | null>(null);
  const [creating, setCreating] = useState(false);
  const [corpName, setCorpName] = useState('');
  const [wxCorpId, setWxCorpId] = useState('');
  const [employeeSecret, setEmployeeSecret] = useState('');
  const [contactSecret, setContactSecret] = useState('');
  const [error, setError] = useState('');

  const input: CorpListInput = { corpId: '', corpName: keyword, page: 1, perPage: 20 };
  const query = useQuery({ queryKey: ['company-website', keyword], queryFn: () => api.list(input) });
  const items = query.data?.list ?? [];
  const total = query.data?.page?.total ?? 0;

  const save = useMutation({
    mutationFn: () => editing
      ? api.update({ corpId: editing.corpId, corpName: corpName.trim(), wxCorpId: wxCorpId.trim(), employeeSecret: employeeSecret.trim(), contactSecret: contactSecret.trim() })
      : api.create({ corpName: corpName.trim(), wxCorpId: wxCorpId.trim(), employeeSecret: employeeSecret.trim(), contactSecret: contactSecret.trim() }),
    onSuccess: () => {
      setEditing(null); setCreating(false); setCorpName(''); setWxCorpId(''); setEmployeeSecret(''); setContactSecret(''); setError('');
      void queryClient.invalidateQueries({ queryKey: ['company-website'] });
    },
    onError: (err) => setError(err instanceof Error ? err.message : '保存失败'),
  });
  const openCreate = () => { setCreating(true); setEditing(null); setCorpName(''); setWxCorpId(''); setEmployeeSecret(''); setContactSecret(''); setError(''); };
  const openEdit = (item: CorpDetail) => {
    setEditing(item); setCreating(false); setCorpName(item.corpName); setWxCorpId(item.wxCorpId);
    setEmployeeSecret(item.employeeSecret); setContactSecret(item.contactSecret); setError('');
  };
  const valid = Boolean(corpName.trim() && wxCorpId.trim());

  return (
    <Phase35PageShell title="企业信息" description="查看与维护企业微信企业信息与密钥配置" actions={<button type="button" onClick={openCreate}>新建企业</button>}>
      <div className="phase35-page">
        <section className="phase35-card phase35-filter-card">
          <form className="dashboard-filter-bar" onSubmit={(event) => { event.preventDefault(); void query.refetch(); }}>
            <input aria-label="企业名称" value={keyword} onChange={(event) => setKeyword(event.target.value)} placeholder="按企业名称筛选" />
            <button type="submit">查询</button>
            <button type="button" onClick={() => setKeyword('')}>重置</button>
          </form>
        </section>

        <section className="phase35-kpis" aria-label="企业指标">
          <article className="phase35-kpi phase35-kpi-primary"><span>企业总数</span><strong>{total}</strong><small>当前租户下已接入的企业微信企业</small></article>
          <article className="phase35-kpi phase35-kpi-green"><span>当前页</span><strong>{items.length}</strong><small>筛选条件下本页展示条数</small></article>
        </section>

        <section className="phase35-card phase35-table-card">
          <header className="phase35-card-header"><div><h2>企业列表</h2><p>企业信息编辑与密钥维护</p></div><span className="phase35-chip">共 {total} 条</span></header>
          <Phase35DataState loading={query.isLoading} error={query.isError} empty={!items.length} emptyContent={<p className="phase35-empty">暂无企业，点击“新建企业”接入</p>} onRetry={() => void query.refetch()}>
            <div className="phase35-table">
              <table>
                <thead><tr><th>企业名称</th><th>企业微信 CorpId</th><th>创建时间</th><th>操作</th></tr></thead>
                <tbody>
                  {items.map((item) => (
                    <tr key={item.corpId}>
                      <td>{item.corpName}</td><td>{item.wxCorpId}</td><td>{item.createdAt}</td>
                      <td><button type="button" onClick={() => void api.show(item.corpId).then(openEdit)}>编辑</button></td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </Phase35DataState>
        </section>

        {(creating || editing) && (
          <section className="phase35-card" role="dialog" aria-label="企业信息表单">
            <header className="phase35-card-header"><div><h2>{editing ? '编辑企业' : '新建企业'}</h2></div></header>
            {error && <p role="alert" className="phase35-limits">{error}</p>}
            <form onSubmit={(event) => { event.preventDefault(); if (valid) save.mutate(); }}>
              <label>企业名称<input value={corpName} onChange={(event) => setCorpName(event.target.value)} /></label>
              <label>企业微信 CorpId<input value={wxCorpId} onChange={(event) => setWxCorpId(event.target.value)} /></label>
              <label>员工密钥<input value={employeeSecret} onChange={(event) => setEmployeeSecret(event.target.value)} /></label>
              <label>客户密钥<input value={contactSecret} onChange={(event) => setContactSecret(event.target.value)} /></label>
              <button type="submit" disabled={!valid || save.isPending}>保存</button>
              <button type="button" onClick={() => { setEditing(null); setCreating(false); setError(''); }}>取消</button>
            </form>
          </section>
        )}
      </div>
    </Phase35PageShell>
  );
}
