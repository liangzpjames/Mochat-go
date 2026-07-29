import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Alert, Button, Card, Form, Input, InputNumber, Modal, Popconfirm, Radio, Select, Space, Table } from 'antd';
import { useEffect, useState } from 'react';
import { useDashboardAccess } from '../../app/access-context';
import type { MenuDetail, MenuListResult, MenuOption, MenuWrite } from './menu-admin-api';

export type MenuAdminPageApi = {
  list(input: { name: string; page: number; perPage: number }): Promise<MenuListResult>;
  options(): Promise<MenuOption[]>; detail(menuId: number): Promise<MenuDetail>;
  usedIcons(): Promise<string[]>; create(input: MenuWrite): Promise<void>;
  update(menuId: number, input: MenuWrite): Promise<void>;
  updateStatus(menuId: number, status: number): Promise<void>; remove(menuId: number): Promise<void>;
};
type Editor = { kind: 'add' | 'edit'; menuId?: number; status?: number; initial: MenuWrite };
const empty: MenuWrite = { level: 1, name: '', icon: '', linkUrl: '', linkType: 1 };
export function MenuAdminPage({ api }: { api: MenuAdminPageApi }) {
  const access = useDashboardAccess(), qc = useQueryClient();
  const [draft, setDraft] = useState(''), [name, setName] = useState('');
  const [page, setPage] = useState(1), [perPage, setPerPage] = useState(10);
  const [editor, setEditor] = useState<Editor | null>(null);
  const key = ['menu-admin', access.corp.id] as const;
  const query = useQuery({ queryKey: [...key, name, page, perPage],
    queryFn: () => api.list({ name, page, perPage }) });
  const options = useQuery({ queryKey: [...key, 'options'], queryFn: () => api.options() });
  const mutation = useMutation({ mutationFn: (op: () => Promise<void>) => op(),
    onSuccess: () => qc.invalidateQueries({ queryKey: key }) });
  const can = (a: string) => access.allowedActions.has(`/menu/index@${a}`);
  const edit = async (menuId: number) => {
    const d = await api.detail(menuId); setEditor({ kind: 'edit', menuId, status: d.status, initial: d });
  };
  return <Card title="菜单管理"><Space wrap>
    <Input placeholder="请输入菜单名称" value={draft} onChange={e => setDraft(e.target.value)} />
    {can('search') && <Button type="primary" onClick={() => { setPage(1); setName(draft); }}>查询</Button>}
    <Button onClick={() => { setDraft(''); setName(''); setPage(1); }}>重置</Button>
    {can('add') && <Button onClick={() => setEditor({ kind: 'add', initial: empty })}>添加</Button>}
  </Space>
  {(query.error ?? mutation.error) && <Alert type="error" title="操作失败" />}
  <Table rowKey="menuId" dataSource={query.data?.list ?? []} loading={query.isLoading}
    columns={[{ title: '序号', dataIndex: 'menuPath' }, { title: '菜单名称', dataIndex: 'name' },
      { title: '菜单级别', dataIndex: 'levelName' }, { title: '图标', dataIndex: 'icon' },
      { title: '状态', render: (_, r) => r.status === 1 ? '启用' : '禁用' },
      { title: '最后操作人', dataIndex: 'operateName' }, { title: '最后操作时间', dataIndex: 'updatedAt' },
      { title: '操作', render: (_, r) => can('edit') &&
        <Button type="link" onClick={() => void edit(r.menuId)}>编辑</Button> }]}
    pagination={{ current: page, pageSize: perPage, total: query.data?.page.total ?? 0,
      onChange: (p, s) => { setPage(s === perPage ? p : 1); setPerPage(s); } }} />
  <MenuEditor editor={editor} options={options.data ?? []} loading={mutation.isPending}
    onCancel={() => setEditor(null)}
    onSave={v => mutation.mutate(async () => {
      if (editor?.kind === 'add') await api.create(v);
      else if (editor?.menuId) await api.update(editor.menuId, v);
      setEditor(null);
    })}
    onStatus={() => editor?.menuId && mutation.mutate(async () => {
      await api.updateStatus(editor.menuId!, editor.status === 1 ? 2 : 1); setEditor(null);
    })}
    onRemove={() => editor?.menuId && mutation.mutate(async () => {
      await api.remove(editor.menuId!); setEditor(null);
    })} />
  </Card>;
}
function MenuEditor({ editor, options, loading, onCancel, onSave, onStatus, onRemove }: {
  editor: Editor | null; options: MenuOption[]; loading: boolean; onCancel: () => void;
  onSave: (v: MenuWrite) => void; onStatus: () => void; onRemove: () => void;
}) {
  const [form] = Form.useForm<MenuWrite>(); const level = Form.useWatch('level', form) ?? 1;
  useEffect(() => { if (editor) form.setFieldsValue(editor.initial); }, [editor, form]);
  return <Modal open={editor !== null} title={editor?.kind === 'add' ? '添加菜单' : '编辑菜单'}
    onCancel={onCancel} footer={<Space>
      <Button type="primary" loading={loading} onClick={() => void form.validateFields().then(onSave)}>确定</Button>
      {editor?.kind === 'edit' && editor.status === 2 && <Popconfirm title="确认移除？" onConfirm={onRemove}><Button>移除</Button></Popconfirm>}
      {editor?.kind === 'edit' && <Button onClick={onStatus}>{editor.status === 1 ? '禁用' : '启用'}</Button>}
      <Button onClick={onCancel}>取消</Button>
    </Space>} forceRender>
    <Form form={form} initialValues={empty} layout="vertical">
      {editor?.kind === 'add' && <Form.Item label="菜单级别" name="level" rules={[{ required: true }]}>
        <InputNumber min={1} max={5} /></Form.Item>}
      {editor?.kind === 'add' && level > 1 && <Form.Item label="上级菜单" name="firstMenuId" rules={[{ required: true }]}>
        <Select options={options.map(x => ({ label: x.name, value: x.menuId }))} /></Form.Item>}
      <Form.Item label="名称" name="name" rules={[{ required: true }]}><Input maxLength={10} /></Form.Item>
      {level < 4 && <Form.Item label="图标" name="icon"><Input /></Form.Item>}
      {level > 2 && <><Form.Item label="地址" name="linkUrl" rules={[{ required: true }]}><Input /></Form.Item>
        <Form.Item label="链接类型" name="linkType"><Radio.Group options={[{ label: '内部链接', value: 1 }, { label: '外部链接', value: 2 }]} /></Form.Item></>}
      {level > 3 && <Form.Item label="页面权限" name="isPageMenu"><Radio.Group options={[{ label: '是', value: 1 }, { label: '否', value: 2 }]} /></Form.Item>}
    </Form>
  </Modal>;
}
