import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useMemo, useState } from 'react';
import { useSearchParams } from 'react-router';

import { useDashboardAccess } from '../../app/access-context';
import { pageStateForError, PageState } from '../../components/page-state/page-state';
import type { ScrmApi, Tag, TagGroup } from './scrm-api';

export type CustomerTagApi = Required<Pick<ScrmApi, 'listTagCatalog' | 'createTagGroup' | 'renameTagGroup' | 'createTag' | 'renameTag' | 'moveTag' | 'previewDeleteTag' | 'deleteTag' | 'maintainTagContacts' | 'listContactOptions'>>;
type FailedOperation = { error: unknown; retry: () => void };
type DeleteTarget = { id: string; name: string; version: number; affectedResourceCount: number };

const operationKey = (prefix: string) => `${prefix}-${Date.now()}-${Math.random().toString(36).slice(2)}`;

export function TagPage({ api }: { api: CustomerTagApi }) {
  const access = useDashboardAccess();
  const corpId = Number(access.corp.id);
  const client = useQueryClient();
  const [searchParams, setSearchParams] = useSearchParams();
  const searchText = searchParams.toString();
  const appliedGroupId = searchParams.get('groupId')?.trim() || undefined;
  const appliedKeyword = searchParams.get('keyword')?.trim() || undefined;
  const [keyword, setKeyword] = useState(appliedKeyword ?? '');
  const [newGroupName, setNewGroupName] = useState('');
  const [newTagName, setNewTagName] = useState('');
  const [groupNames, setGroupNames] = useState<Record<string, string>>({});
  const [tagNames, setTagNames] = useState<Record<string, string>>({});
  const [addContacts, setAddContacts] = useState<string[]>([]);
  const [removeContacts, setRemoveContacts] = useState<string[]>([]);
  const [deleteTarget, setDeleteTarget] = useState<DeleteTarget | null>(null);
  const [failed, setFailed] = useState<FailedOperation | null>(null);
  const [feedback, setFeedback] = useState('');

  const query = useQuery({
    queryKey: ['scrm-tag-catalog', corpId, appliedGroupId ?? '', appliedKeyword ?? ''],
    queryFn: () => api.listTagCatalog({ corpId, ...(appliedGroupId ? { groupId: appliedGroupId } : {}), ...(appliedKeyword ? { keyword: appliedKeyword } : {}) }),
  });
  const contacts = useQuery({ queryKey: ['scrm-contact-options', corpId], queryFn: () => api.listContactOptions({ corpId }) });
  const refresh = () => {
    void client.invalidateQueries({ queryKey: ['scrm-tag-catalog', corpId] });
    void client.invalidateQueries({ queryKey: ['scrm-contacts', corpId] });
    void client.invalidateQueries({ queryKey: ['scrm-contact-detail', corpId] });
  };
  const selectedGroupId = appliedGroupId ?? '';
  const selectedGroup = query.data?.groups.find((group) => group.id === selectedGroupId);
  const visibleTags = useMemo(() => query.data?.tags ?? [], [query.data?.tags]);

  useEffect(() => {
    if (!query.data) return;
    setGroupNames((current) => Object.fromEntries(query.data.groups.map((group) => [group.id, current[group.id] ?? group.name])));
    setTagNames((current) => Object.fromEntries(query.data.tags.map((tag) => [tag.id, current[tag.id] ?? tag.name])));
  }, [query.data]);
  useEffect(() => setKeyword(new URLSearchParams(searchText).get('keyword')?.trim() ?? ''), [searchText]);

  const mutationOptions = <T,>(success: string) => ({
    onSuccess: (_value: T) => { setFailed(null); setFeedback(success); void refresh(); },
  });
  const createGroup = useMutation({ mutationFn: (input: Parameters<CustomerTagApi['createTagGroup']>[0]) => api.createTagGroup(input), ...mutationOptions<TagGroup>('标签组已新增。') });
  const renameGroup = useMutation({ mutationFn: (input: Parameters<CustomerTagApi['renameTagGroup']>[0]) => api.renameTagGroup(input), ...mutationOptions<TagGroup>('标签组名称已更新。') });
  const createTag = useMutation({ mutationFn: (input: Parameters<CustomerTagApi['createTag']>[0]) => api.createTag(input), ...mutationOptions<Tag>('标签已新增。') });
  const renameTag = useMutation({ mutationFn: (input: Parameters<CustomerTagApi['renameTag']>[0]) => api.renameTag(input), ...mutationOptions<Tag>('标签名称已更新。') });
  const moveTag = useMutation({ mutationFn: (input: Parameters<CustomerTagApi['moveTag']>[0]) => api.moveTag(input), ...mutationOptions<Tag>('标签已移动。') });
  const maintain = useMutation({ mutationFn: (input: Parameters<CustomerTagApi['maintainTagContacts']>[0]) => api.maintainTagContacts(input), ...mutationOptions<Tag>('联系人标签关系已更新。') });
  const previewDelete = useMutation({ mutationFn: (tag: Tag) => api.previewDeleteTag({ corpId, tagId: tag.id }), onSuccess: (preview, tag) => { setFailed(null); setDeleteTarget({ id: tag.id, name: tag.name, version: preview.version, affectedResourceCount: preview.affectedResourceCount }); } });
  const removeTag = useMutation({ mutationFn: (input: Parameters<CustomerTagApi['deleteTag']>[0]) => api.deleteTag(input), onSuccess: (result) => { setFailed(null); setFeedback(`标签已删除，已清理 ${result.affectedResourceCount} 个联系人关联。`); refresh(); } });

  const run = <T,>(mutation: { mutate: (input: T, options?: { onError?: (error: unknown) => void }) => void }, input: T) => {
    setFeedback('');
    const execute = () => mutation.mutate(input, { onError: (error) => setFailed({ error, retry: execute }) });
    execute();
  };
  const applyFilters = (groupId = selectedGroupId, value = keyword) => {
    const next = new URLSearchParams();
    if (groupId) next.set('groupId', groupId);
    if (value.trim()) next.set('keyword', value.trim());
    setSearchParams(next);
  };

  return <section className="scrm-tag-page">
    <header className="scrm-page-header dashboard-page-header dashboard-data-card"><div><p className="scrm-page-eyebrow">SCRM · 客户分类</p><h1>客户标签</h1><p>按标签组维护客户分类，并在同一处查看使用数量和批量更新联系人关联。</p></div><span className="scrm-page-badge">企业标签目录</span></header>
    <div aria-label="标签类型" className="scrm-tag-type-tabs" role="tablist"><button aria-selected="true" role="tab" type="button">企业标签</button><button aria-selected="false" disabled role="tab" title="当前后端暂未提供系统标签目录" type="button">系统标签</button><button aria-selected="false" disabled role="tab" title="当前后端暂未提供群聊标签目录" type="button">群聊标签</button><span>企业成员可用标签快速识别并筛选客户。</span></div>
    <div aria-label="标签数据概览" className="dashboard-stat-grid scrm-overview" role="region"><article><span>标签组</span><strong>{query.data?.groups.length ?? 0}</strong><small>分类目录数量</small></article><article><span>标签</span><strong>{query.data?.tags.length ?? 0}</strong><small>当前筛选结果</small></article><article><span>当前分组</span><strong>{selectedGroup?.name ?? '全部'}</strong><small>URL 可恢复</small></article><article><span>关联使用</span><strong>{query.data?.tags.reduce((sum, tag) => sum + tag.usageCount, 0) ?? 0}</strong><small>联系人关联总数</small></article></div>
    <div className="dashboard-filter-bar">
      <label>标签组<select aria-label="标签组筛选" value={selectedGroupId} onChange={(event) => applyFilters(event.target.value, keyword)}><option value="">全部分组</option>{query.data?.groups.map((group) => <option key={group.id} value={group.id}>{group.name}</option>)}</select></label>
      <label>关键词<input aria-label="标签关键词" value={keyword} placeholder="搜索标签名称" onChange={(event) => setKeyword(event.target.value)} /></label>
      <div className="dashboard-table-actions"><button type="button" onClick={() => applyFilters()}>查询</button><button type="button" onClick={() => { setKeyword(''); setSearchParams(new URLSearchParams()); }}>重置</button></div>
    </div>

    <div className="dashboard-data-card"><div className="dashboard-card-heading"><div><h2>标签组</h2><p>分组用于组织标签；版本冲突会提示刷新后重试。</p></div></div>
      <div className="dashboard-table-actions"><label>新标签组<input aria-label="新标签组" value={newGroupName} onChange={(event) => setNewGroupName(event.target.value)} /></label><button type="button" disabled={!newGroupName.trim() || createGroup.isPending} onClick={() => { const name = newGroupName.trim(); run(createGroup, { corpId, name, idempotencyKey: operationKey('tag-group-create') }); setNewGroupName(''); }}>新增标签组</button></div>
      {query.data?.groups.length ? <div className="dashboard-table-scroll"><table><thead><tr><th>分组名称</th><th>标签数</th><th>版本</th><th>操作</th></tr></thead><tbody>{query.data.groups.map((group) => <tr key={group.id}><td><input aria-label={`标签组名称 ${group.name}`} value={groupNames[group.id] ?? group.name} onChange={(event) => setGroupNames({ ...groupNames, [group.id]: event.target.value })} /></td><td>{group.tagCount}</td><td>{group.version}</td><td><button type="button" aria-label={`改名标签组 ${group.name}`} disabled={!groupNames[group.id]?.trim()} onClick={() => run(renameGroup, { corpId, groupId: group.id, name: groupNames[group.id]!.trim(), version: group.version, idempotencyKey: operationKey('tag-group-rename') })}>保存名称</button></td></tr>)}</tbody></table></div> : <p className="dashboard-inline-feedback">暂无标签组，请先新增一个标签组。</p>}
    </div>

    <div className="dashboard-data-card scrm-list-card"><div className="dashboard-card-heading"><div><h2>标签目录</h2><p>新增、改名、移动和删除均实时刷新目录与联系人详情缓存。</p></div><button aria-label="刷新标签" disabled={query.isFetching} onClick={() => void query.refetch()} type="button">刷新</button></div>
      <div className="dashboard-table-actions"><label>新标签<input aria-label="新标签" value={newTagName} onChange={(event) => setNewTagName(event.target.value)} /></label><button type="button" disabled={!selectedGroupId || !newTagName.trim() || createTag.isPending} onClick={() => { const name = newTagName.trim(); run(createTag, { corpId, groupId: selectedGroupId, name, idempotencyKey: operationKey('tag-create') }); setNewTagName(''); }}>新增标签</button></div>
      {query.isPending ? <PageState state="loading" /> : query.isError ? <PageState state={pageStateForError(query.error)} onRetry={() => void query.refetch()} /> : visibleTags.length === 0 ? <PageState state="empty" /> : <div className="dashboard-table-scroll"><table><thead><tr><th>标签</th><th>分组</th><th>使用数</th><th>联系人维护</th><th>操作</th></tr></thead><tbody>{visibleTags.map((tag) => {
        const targetGroup = query.data.groups.find((group) => group.id !== tag.groupId);
        return <tr key={tag.id}><td><strong>{tag.name}</strong><input aria-label={`标签名称 ${tag.name}`} value={tagNames[tag.id] ?? tag.name} onChange={(event) => setTagNames({ ...tagNames, [tag.id]: event.target.value })} /><small>版本 {tag.version}</small></td><td>{query.data.groups.find((group) => group.id === tag.groupId)?.name ?? '未分组'}</td><td><strong>使用 {tag.usageCount}</strong></td><td><div className="scrm-tag-contact-editor"><fieldset><legend>绑定联系人</legend>{(contacts.data ?? []).map((contact) => <label key={`add-${contact.id}`}><input type="checkbox" aria-label={`绑定联系人 ${contact.name}`} checked={addContacts.includes(contact.id)} onChange={(event) => setAddContacts((current) => event.target.checked ? [...current, contact.id] : current.filter((id) => id !== contact.id))} />{contact.name}</label>)}</fieldset><fieldset><legend>解绑联系人</legend>{(contacts.data ?? []).map((contact) => <label key={`remove-${contact.id}`}><input type="checkbox" aria-label={`解绑联系人 ${contact.name}`} checked={removeContacts.includes(contact.id)} onChange={(event) => setRemoveContacts((current) => event.target.checked ? [...current, contact.id] : current.filter((id) => id !== contact.id))} />{contact.name}</label>)}</fieldset><button type="button" aria-label={`维护联系人 ${tag.name}`} disabled={addContacts.length + removeContacts.length === 0} onClick={() => run(maintain, { corpId, tagId: tag.id, addContactIds: addContacts, removeContactIds: removeContacts, version: tag.version, idempotencyKey: operationKey('tag-contacts') })}>批量维护</button></div></td><td><div className="dashboard-table-actions"><button type="button" aria-label={`改名标签 ${tag.name}`} disabled={!tagNames[tag.id]?.trim()} onClick={() => run(renameTag, { corpId, tagId: tag.id, name: tagNames[tag.id]!.trim(), version: tag.version, idempotencyKey: operationKey('tag-rename') })}>保存名称</button><button type="button" aria-label={`移动标签 ${tag.name}`} disabled={!targetGroup} onClick={() => targetGroup && run(moveTag, { corpId, tagId: tag.id, groupId: targetGroup.id, version: tag.version, idempotencyKey: operationKey('tag-move') })}>移动到{targetGroup?.name ?? '其他组'}</button><button type="button" aria-label={`删除标签 ${tag.name}`} onClick={() => { const execute = () => previewDelete.mutate(tag, { onError: (error) => setFailed({ error, retry: execute }) }); execute(); }}>删除</button></div></td></tr>;
      })}</tbody></table></div>}
    </div>

    {deleteTarget && <div className="dashboard-data-card scrm-tag-delete-confirm" role="alertdialog" aria-label="删除标签确认"><h2>确认删除“{deleteTarget.name}”</h2><p>将影响 {deleteTarget.affectedResourceCount} 个联系人</p><div className="dashboard-table-actions"><button type="button" onClick={() => setDeleteTarget(null)}>取消</button><button type="button" onClick={() => { const tag = deleteTarget; setDeleteTarget(null); run(removeTag, { corpId, tagId: tag.id, version: tag.version, idempotencyKey: operationKey('tag-delete') }); }}>确认删除</button></div></div>}
    {feedback && <p role="status" className="dashboard-inline-feedback">{feedback}</p>}
    {failed && <PageState state={pageStateForError(failed.error)} description="标签操作未完成；版本冲突请刷新目录，其他错误可重试原操作。" retryLabel={pageStateForError(failed.error) === 'conflict' ? '刷新目录' : '重试原操作'} onRetry={() => { if (pageStateForError(failed.error) === 'conflict') { setFailed(null); void query.refetch(); } else failed.retry(); }} />}
  </section>;
}
