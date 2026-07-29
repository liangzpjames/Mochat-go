import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  Alert, Button, Card, Form, Input, Modal, Popconfirm, Radio, Space, Switch, Table,
} from 'antd';
import { useEffect, useState } from 'react';

import { useDashboardAccess } from '../../app/access-context';
import type {
  RoleDetail, RoleItem, RoleMember, RolePageResult, RoleWrite,
} from './role-api';

export type RolePageApi = {
  list(input: { name: string; page: number; perPage: number }): Promise<RolePageResult<RoleItem>>;
  detail(roleId: number): Promise<RoleDetail>;
  members(input: { roleId: number; page: number; perPage: number }): Promise<RolePageResult<RoleMember>>;
  create(input: RoleWrite): Promise<void>;
  copy(roleId: number, input: RoleWrite): Promise<void>;
  update(roleId: number, input: RoleWrite): Promise<void>;
  updateStatus(roleId: number, status: number): Promise<void>;
  remove(roleId: number): Promise<void>;
};

type Editor = { kind: 'add' | 'copy' | 'edit'; roleId?: number; initial: RoleWrite };
const emptyRole: RoleWrite = { name: '', remarks: '', dataPermission: 2 };

export function RolePage({ api, navigate }: {
  api: RolePageApi;
  navigate: (path: string) => void;
}) {
  const access = useDashboardAccess();
  const queryClient = useQueryClient();
  const [search, setSearch] = useState('');
  const [name, setName] = useState('');
  const [page, setPage] = useState(1);
  const [perPage, setPerPage] = useState(10);
  const [editor, setEditor] = useState<Editor | null>(null);
  const [memberRoleId, setMemberRoleId] = useState<number | null>(null);
  const [memberPage, setMemberPage] = useState(1);
  const [memberPerPage, setMemberPerPage] = useState(10);
  const key = ['role', access.corp.id, 'list'] as const;
  const query = useQuery({
    queryKey: [...key, name, page, perPage],
    queryFn: () => api.list({ name, page, perPage }),
  });
  const memberQuery = useQuery({
    queryKey: ['role', access.corp.id, 'members', memberRoleId, memberPage, memberPerPage],
    queryFn: () => api.members({ roleId: memberRoleId!, page: memberPage, perPage: memberPerPage }),
    enabled: memberRoleId !== null,
  });
  const refresh = async () => queryClient.invalidateQueries({ queryKey: key });
  const mutation = useMutation({
    mutationFn: async (operation: () => Promise<void>) => operation(),
    onSuccess: refresh,
  });
  const can = (action: string) => access.allowedActions.has(`/role/index@${action}`);
  const error = query.error ?? memberQuery.error ?? mutation.error;

  const openEdit = async (roleId: number) => {
    const detail = await api.detail(roleId);
    setEditor({ kind: 'edit', roleId, initial: detail });
  };

  return <Card title="角色管理">
    <Space wrap>
      <Input placeholder="请输入角色名称" value={search} onChange={(event) => setSearch(event.target.value)} />
      {can('search') && <Button type="primary" onClick={() => { setPage(1); setName(search); }}>查询</Button>}
      <Button onClick={() => { setSearch(''); setName(''); setPage(1); }}>重置</Button>
      {can('add') && <Button onClick={() => setEditor({ kind: 'add', initial: emptyRole })}>添加</Button>}
    </Space>
    {error !== null && <Alert role="alert" type="error"
      title={error instanceof Error ? error.message : '操作失败'} />}
    <Table
      columns={[
        { title: '角色名称', dataIndex: 'name' },
        { title: '角色人员', dataIndex: 'employeeNum', render: (value: number, row: RoleItem) =>
          can('checkMember') ? <Button type="link" onClick={() => {
            setMemberPage(1); setMemberRoleId(row.roleId);
          }}>{value}</Button> : value },
        { title: '角色描述', dataIndex: 'remarks' },
        { title: '更新时间', dataIndex: 'updatedAt' },
        { title: '启用', render: (_, row: RoleItem) => can('use') ? <Switch
          checked={row.status === 1}
          disabled={row.status === 1 && row.employeeNum > 0}
          onChange={(checked) => mutation.mutate(() => api.updateStatus(row.roleId, checked ? 1 : 2))}
        /> : row.status === 1 ? '启用' : '禁用' },
        { title: '操作', render: (_, row: RoleItem) => <Space>
          {access.allowedActions.has('/role/permissionShow') && <Button type="link"
            onClick={() => navigate(`/role/permissionShow?roleId=${row.roleId}`)}>设置权限</Button>}
          {can('edit') && <Button type="link" onClick={() => void openEdit(row.roleId)}>编辑</Button>}
          {can('copy') && <Button type="link" onClick={() => setEditor({
            kind: 'copy', roleId: row.roleId,
            initial: { name: row.name, remarks: row.remarks, dataPermission: row.dataPermission ?? 2 },
          })}>复制权限</Button>}
          {can('delete') && row.employeeNum === 0 && <Popconfirm title="确认删除此角色吗？"
            onConfirm={() => mutation.mutate(() => api.remove(row.roleId))}>
            <Button type="link" danger>删除</Button>
          </Popconfirm>}
        </Space> },
      ]}
      dataSource={query.data?.list ?? []}
      loading={query.isLoading}
      pagination={{ current: page, pageSize: perPage, total: query.data?.page.total ?? 0,
        showSizeChanger: true, onChange: (next, size) => {
          setPage(size === perPage ? next : 1); setPerPage(size);
        } }}
      rowKey="roleId"
    />
    <RoleEditor editor={editor} loading={mutation.isPending} onCancel={() => setEditor(null)}
      onSave={(values) => mutation.mutate(async () => {
        if (editor?.kind === 'add') await api.create(values);
        else if (editor?.kind === 'copy' && editor.roleId !== undefined) await api.copy(editor.roleId, values);
        else if (editor?.kind === 'edit' && editor.roleId !== undefined) await api.update(editor.roleId, values);
        setEditor(null);
      })} />
    <Modal open={memberRoleId !== null} title="角色人员" footer={null}
      onCancel={() => setMemberRoleId(null)}>
      <Table columns={[
        { title: '姓名', dataIndex: 'employeeName' }, { title: '手机号码', dataIndex: 'phone' },
        { title: '邮箱地址', dataIndex: 'email' }, { title: '部门', dataIndex: 'department' },
      ]} dataSource={memberQuery.data?.list ?? []} loading={memberQuery.isLoading}
      pagination={{ current: memberPage, pageSize: memberPerPage,
        total: memberQuery.data?.page.total ?? 0, onChange: (next, size) => {
          setMemberPage(size === memberPerPage ? next : 1); setMemberPerPage(size);
        } }} rowKey="employeeId" />
    </Modal>
  </Card>;
}

function RoleEditor({ editor, loading, onCancel, onSave }: {
  editor: Editor | null; loading: boolean; onCancel: () => void; onSave: (values: RoleWrite) => void;
}) {
  const [form] = Form.useForm<RoleWrite>();
  useEffect(() => {
    if (editor !== null) form.setFieldsValue(editor.initial);
  }, [editor, form]);
  return <Modal open={editor !== null} title="角色信息" onCancel={onCancel}
    onOk={() => void form.validateFields().then(onSave)} okText="确定"
    okButtonProps={{ loading }} forceRender>
    <Form form={form} initialValues={editor?.initial ?? emptyRole}
      key={`${editor?.kind ?? 'closed'}-${editor?.roleId ?? ''}`} layout="vertical">
      <Form.Item label="角色名称" name="name" rules={[{ required: true, message: '请输入角色名称' }]}>
        <Input maxLength={8} placeholder="请输入角色名称" />
      </Form.Item>
      <Form.Item label="角色描述" name="remarks"><Input.TextArea rows={4} /></Form.Item>
      <Form.Item label="部门数据全览" name="dataPermission">
        <Radio.Group options={[{ label: '是', value: 1 }, { label: '否', value: 2 }]} />
      </Form.Item>
    </Form>
  </Modal>;
}
