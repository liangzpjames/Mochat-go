import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Alert, Button, Card, Input, Modal, Popconfirm, Select, Space, Table } from 'antd';
import { useState } from 'react';
import { useDashboardAccess } from '../../app/access-context';
import type { TagDetail, TagGroup, TagListResult } from './contact-tag-api';
export type ContactTagPageApi = {
  list(v: { groupId: number; page: number; perPage: number }): Promise<TagListResult>;
  groups(): Promise<TagGroup[]>; detail(id: number): Promise<TagDetail>;
  createTags(groupId: number, names: string[]): Promise<void>;
  updateTag(v: TagDetail & { isUpdate: number }): Promise<void>;
  removeTags(ids: number[]): Promise<void>; moveTags(ids: number[], groupId: number): Promise<void>;
  sync(): Promise<void>; createGroup(name: string): Promise<void>;
  updateGroup(id: number, name: string): Promise<void>; removeGroup(id: number): Promise<void>;
};
type Dialog = 'add-tag' | 'edit-tag' | 'add-group' | 'edit-group' | 'move' | null;
export function ContactTagPage({ api }: { api: ContactTagPageApi }) {
  const access = useDashboardAccess(), qc = useQueryClient();
  const [groupId, setGroupId] = useState(0), [page, setPage] = useState(1), [perPage, setPerPage] = useState(10);
  const [selected, setSelected] = useState<React.Key[]>([]), [dialog, setDialog] = useState<Dialog>(null);
  const [name, setName] = useState(''), [targetGroup, setTargetGroup] = useState(0);
  const [editTag, setEditTag] = useState<TagDetail | null>(null);
  const key = ['contact-tag', access.corp.id] as const;
  const groups = useQuery({ queryKey: [...key, 'groups'], queryFn: () => api.groups() });
  const tags = useQuery({ queryKey: [...key, groupId, page, perPage],
    queryFn: () => api.list({ groupId, page, perPage }) });
  const mutation = useMutation({ mutationFn: (op: () => Promise<void>) => op(),
    onSuccess: () => qc.invalidateQueries({ queryKey: key }) });
  const can = (a: string) => access.allowedActions.has(`/workContactTag/index@${a}`);
  const close = () => { setDialog(null); setName(''); setEditTag(null); };
  const selectedIds = selected.map(Number);
  const saveTags = () => {
    const names = [...new Set(name.trim().split(/\s+/))];
    if (names.length === 0 || names.some(n => !n || n.length > 15)) return;
    mutation.mutate(async () => { await api.createTags(targetGroup, names); close(); });
  };
  return <Card title="客户标签">
    <Alert type="info" title="标签分组名称不能重复；每个标签名称最多 15 个字符；未分组不可修改或删除。" />
    <Space wrap>
      <Select aria-label="选择分组" value={groupId} options={(groups.data ?? []).map(g => ({ label: g.groupName, value: g.groupId }))}
        onChange={v => { setGroupId(v); setPage(1); }} />
      <span>最后一次同步时间：{tags.data?.syncTagTime || '--'}</span>
      {can('sync') && <Button onClick={() => mutation.mutate(() => api.sync())}>同步企业微信标签</Button>}
      {can('addGroup') && <Button onClick={() => setDialog('add-group')}>新增分组</Button>}
      {can('editGroup') && <Button disabled={groupId === 0} onClick={() => {
        setName(groups.data?.find(g => g.groupId === groupId)?.groupName ?? ''); setDialog('edit-group');
      }}>修改分组</Button>}
      {can('add') && <Button onClick={() => { setTargetGroup(groupId); setDialog('add-tag'); }}>新建标签</Button>}
      {can('move') && <Button disabled={!selected.length} onClick={() => { setTargetGroup(groupId); setDialog('move'); }}>移动标签</Button>}
      {can('deleteTag') && <Popconfirm title="确认删除选中标签？" onConfirm={() => mutation.mutate(() => api.removeTags(selectedIds))}>
        <Button disabled={!selected.length}>删除标签</Button></Popconfirm>}
    </Space>
    <Table rowKey="id" dataSource={tags.data?.list ?? []} rowSelection={{ selectedRowKeys: selected, onChange: setSelected }}
      columns={[{ title: '标签名称', dataIndex: 'name' }, { title: '客户数', dataIndex: 'contactNum' },
        { title: '操作', render: (_, row) => <Space>
          {can('edit') && <Button type="link" onClick={() => void api.detail(row.id).then(d => {
            setEditTag(d); setName(d.tagName); setTargetGroup(d.groupId); setDialog('edit-tag');
          })}>编辑</Button>}
          {can('delete') && <Popconfirm title="确认删除？" onConfirm={() => mutation.mutate(() => api.removeTags([row.id]))}>
            <Button type="link" danger>删除</Button></Popconfirm>}
        </Space> }]}
      pagination={{ current: page, pageSize: perPage, total: tags.data?.page.total ?? 0,
        onChange: (p, s) => { setPage(s === perPage ? p : 1); setPerPage(s); } }} />
    <Modal title={dialog === 'add-tag' ? '新增标签' : dialog === 'edit-tag' ? '修改标签名称' : ''}
      open={dialog === 'add-tag' || dialog === 'edit-tag'} onCancel={close}
      onOk={() => dialog === 'add-tag' ? saveTags() : editTag && mutation.mutate(async () => {
        await api.updateTag({ tagId: editTag.tagId, groupId: targetGroup, tagName: name, isUpdate: 1 }); close();
      })}>
      <Select aria-label="标签分组" value={targetGroup} options={(groups.data ?? []).map(g => ({ label: g.groupName, value: g.groupId }))} onChange={setTargetGroup} />
      <Input aria-label="标签名称" value={name} maxLength={dialog === 'edit-tag' ? 15 : undefined} onChange={e => setName(e.target.value)} />
    </Modal>
    <Modal title={dialog === 'add-group' ? '新增分组' : '修改分组'} open={dialog === 'add-group' || dialog === 'edit-group'}
      onCancel={close} onOk={() => name.trim() && mutation.mutate(async () => {
        if (dialog === 'add-group') await api.createGroup(name.trim()); else await api.updateGroup(groupId, name.trim()); close();
      })}>
      <Input aria-label="分组名称" maxLength={15} value={name} onChange={e => setName(e.target.value)} />
      {dialog === 'edit-group' && <Popconfirm title="删除分组会同时删除其标签，确认吗？"
        onConfirm={() => mutation.mutate(async () => { await api.removeGroup(groupId); setGroupId(0); close(); })}>
        <Button danger>删除分组</Button></Popconfirm>}
    </Modal>
    <Modal title="移动标签" open={dialog === 'move'} onCancel={close}
      onOk={() => mutation.mutate(async () => { await api.moveTags(selectedIds, targetGroup); setSelected([]); close(); })}>
      <Select aria-label="目标分组" value={targetGroup} options={(groups.data ?? []).map(g => ({ label: g.groupName, value: g.groupId }))} onChange={setTargetGroup} />
    </Modal>
  </Card>;
}
