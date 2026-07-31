import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';

import { useDashboardAccess } from '../../app/access-context';
import { PageState } from '../../components/page-state/page-state';
import type { SensitiveWordApi } from './sensitive-word-api';

export function SensitiveWordPage({ api }: { api: SensitiveWordApi }) {
  const access = useDashboardAccess();
  const queryClient = useQueryClient();
  const [groupId, setGroupId] = useState(0);
  const [keywords, setKeywords] = useState('');
  const [name, setName] = useState('');
  const groups = useQuery({ queryKey: ['sensitive-word', access.corp.id, 'groups'], queryFn: api.groups });
  const words = useQuery({ queryKey: ['sensitive-word', access.corp.id, groupId, keywords], queryFn: () => api.list({ groupId, keywords, page: 1, perPage: 10 }) });
  const create = useMutation({ mutationFn: () => api.create({ groupId, names: [name.trim()], idempotencyKey: `sensitive-${Date.now()}` }), onSuccess: () => { setName(''); void queryClient.invalidateQueries({ queryKey: ['sensitive-word', access.corp.id] }); } });
  const canAdd = access.allowedActions.has('/ai-insight/v2/sensitive-word@add');

  if (words.isPending || groups.isPending) return <PageState state="loading" />;
  if (words.isError || groups.isError) return <PageState state="error" onRetry={() => { void words.refetch(); void groups.refetch(); }} />;

  return (
    <section className="sensitive-word-page">
      <header>
        <h1>敏感词库</h1>
        <p>管理当前企业的敏感词、分组和命中记录。</p>
      </header>
      <div className="sensitive-word-toolbar">
        <label>敏感词名称<input aria-label="敏感词名称" placeholder="输入敏感词" value={name} onChange={(event) => setName(event.target.value)} /></label>
        <label>敏感词分组<select aria-label="敏感词分组" value={groupId} onChange={(event) => setGroupId(Number(event.target.value))}><option value={0}>全部分组</option>{groups.data.map((group) => <option key={group.id} value={group.id}>{group.name}</option>)}</select></label>
        <label>搜索敏感词<input aria-label="搜索敏感词" placeholder="搜索" value={keywords} onChange={(event) => setKeywords(event.target.value)} /></label>
        {canAdd && <button type="button" disabled={name.trim() === '' || groupId === 0 || create.isPending} onClick={() => create.mutate()}>新增敏感词</button>}
      </div>
        {create.isError && <p role="alert">保存失败，请刷新后重试。</p>}
        <table><thead><tr><th>敏感词</th><th>分组</th><th>状态</th></tr></thead><tbody>{words.data.items.map((item) => <tr key={item.id}><td>{item.name}</td><td>{item.groupName}</td><td>{item.status === 1 ? '启用' : '停用'}</td></tr>)}</tbody></table>
    </section>
  );
}
