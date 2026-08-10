import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Alert, Button, Card, Form, Input, Modal, Radio, Select, Space, Table } from 'antd';
import { useEffect, useState } from 'react';
import { useDashboardAccess } from '../../app/access-context';
import type { Department, RoleOption, UserCreate, UserItem, UserListResult, UserWrite } from './user-admin-api';

export type UserAdminPageApi = {
  list(input: { phone: string; status: number | ''; page: number; perPage: number }): Promise<UserListResult>;
  detail(id: number): Promise<UserItem>; departments(phone: string): Promise<Department[]>;
  roles(): Promise<RoleOption[]>; create(input: UserCreate): Promise<void>;
  update(id: number, input: UserWrite): Promise<void>; updateStatus(ids: number[], status: number): Promise<void>;
  resetPassword(id: number, password: string): Promise<void>;
};
type Editor = { id?: number; initial: Partial<UserCreate> };
const empty: UserCreate = { userName: '', phone: '', password: '', confirmPass: '', gender: 1, roleId: 0, status: 0 };
export function UserAdminPage({ api }: { api: UserAdminPageApi }) {
  const access = useDashboardAccess(), qc = useQueryClient();
  const [phoneDraft, setPhoneDraft] = useState(''), [phone, setPhone] = useState('');
  const [statusDraft, setStatusDraft] = useState<number | ''>(''), [status, setStatus] = useState<number | ''>('');
  const [page, setPage] = useState(1), [perPage, setPerPage] = useState(10);
  const [selected, setSelected] = useState<React.Key[]>([]), [editor, setEditor] = useState<Editor | null>(null);
  const [resetId, setResetId] = useState<number | null>(null), [resetPass, setResetPass] = useState('');
  const key = ['user-admin', access.corp.id] as const;
  const query = useQuery({ queryKey: [...key, phone, status, page, perPage],
    queryFn: () => api.list({ phone, status, page, perPage }) });
  const roles = useQuery({ queryKey: [...key, 'roles'], queryFn: () => api.roles() });
  const mutation = useMutation({ mutationFn: (op: () => Promise<void>) => op(),
    onSuccess: () => qc.invalidateQueries({ queryKey: key }) });
  const can = (a: string) => access.allowedActions.size === 0 || access.allowedActions.has(`/user/index@${a}`);
  const edit = async (id: number) => setEditor({ id, initial: await api.detail(id) });
  return <Card title="子账户管理"><Space wrap>
    <Input aria-label="手机号码筛选" placeholder="按手机号码筛选" value={phoneDraft} onChange={e => setPhoneDraft(e.target.value)} />
    <Select aria-label="状态" allowClear value={statusDraft === '' ? undefined : statusDraft}
      options={[{ label: '未启用', value: 0 }, { label: '正常', value: 1 }, { label: '禁用', value: 2 }]}
      onChange={v => setStatusDraft(v ?? '')} />
    {can('search') && <Button type="primary" onClick={() => { setPage(1); setPhone(phoneDraft); setStatus(statusDraft); }}>查询</Button>}
    <Button onClick={() => { setPhoneDraft(''); setStatusDraft(''); setPhone(''); setStatus(''); }}>重置</Button>
    {can('add') && <Button onClick={() => setEditor({ initial: empty })}>添加</Button>}
  </Space>
  <Alert type="info" title={`已启用 ${query.data?.normalNum ?? 0} 项，未启用 ${query.data?.notEnabledNum ?? 0} 人`} />
  <Button disabled={selected.length === 0} onClick={() => mutation.mutate(async () => {
    await api.updateStatus(selected.map(Number), 1); setSelected([]);
  })}>启用选中账户</Button>
  <Table rowKey="userId" dataSource={query.data?.list ?? []} loading={query.isLoading}
    rowSelection={{ selectedRowKeys: selected, onChange: setSelected }}
    columns={[{ title: '企业成员', dataIndex: 'userName' },
      { title: '所属部门', render: (_, r) => r.department.map(d => d.departmentName).join('、') },
      { title: '职务', dataIndex: 'roleName' }, { title: '手机号码（登录账号）', dataIndex: 'phone' },
      { title: '状态', dataIndex: 'statusText' }, { title: '时间', dataIndex: 'createdAt' },
      { title: '操作', render: (_, r) => <Space><Button type="link" onClick={() => void edit(r.userId)}>修改</Button>
        <Button type="link" onClick={() => setResetId(r.userId)}>重置密码</Button></Space> }]}
    pagination={{ current: page, pageSize: perPage, total: query.data?.page.total ?? 0,
      onChange: (p, s) => { setPage(s === perPage ? p : 1); setPerPage(s); } }} />
  <UserEditor editor={editor} roles={roles.data ?? []} loading={mutation.isPending}
    onCancel={() => setEditor(null)} onSave={values => mutation.mutate(async () => {
      if (editor?.id) await api.update(editor.id, values);
      else await api.create(values); setEditor(null);
    })} />
  <Modal title="重置密码" open={resetId !== null} onCancel={() => setResetId(null)}
    onOk={() => resetId && mutation.mutate(async () => { await api.resetPassword(resetId, resetPass); setResetId(null); })}>
    <Input.Password aria-label="新密码" value={resetPass} onChange={e => setResetPass(e.target.value)} />
  </Modal></Card>;
}
function UserEditor({ editor, roles, loading, onCancel, onSave }: {
  editor: Editor | null; roles: RoleOption[]; loading: boolean; onCancel: () => void;
  onSave: (v: UserCreate) => void;
}) {
  const [form] = Form.useForm<UserCreate>();
  useEffect(() => { if (editor) form.setFieldsValue({ ...empty, ...editor.initial }); }, [editor, form]);
  return <Modal title="基础信息" open={editor !== null} onCancel={onCancel} forceRender
    okButtonProps={{ loading }} onOk={() => void form.validateFields().then(v => {
      if (!/^1[3456789]\d{9}$/.test(v.phone)) return form.setFields([{ name: 'phone', errors: ['手机号码格式错误'] }]);
      if (!editor?.id && v.password !== v.confirmPass) return form.setFields([{ name: 'confirmPass', errors: ['两次密码不一致'] }]);
      onSave(v);
    })}>
    <Form form={form} layout="vertical"><Form.Item label="员工姓名" name="userName" rules={[{ required: true }]}><Input /></Form.Item>
      <Form.Item label="手机号码" name="phone" rules={[{ required: true }]}><Input /></Form.Item>
      {!editor?.id && <><Form.Item label="密码" name="password" rules={[{ required: true }]}><Input.Password /></Form.Item>
        <Form.Item label="确认密码" name="confirmPass" rules={[{ required: true }]}><Input.Password /></Form.Item></>}
      <Form.Item label="性别" name="gender"><Radio.Group options={[{ label: '男', value: 1 }, { label: '女', value: 2 }]} /></Form.Item>
      <Form.Item label="状态" name="status"><Radio.Group options={[{ label: '正常', value: 1 }, { label: '禁用', value: 2 }, { label: '未启用', value: 0 }]} /></Form.Item>
      <Form.Item label="角色" name="roleId" rules={[{ required: true }]}><Select options={roles.map(r => ({ label: r.name, value: r.roleId }))} /></Form.Item>
    </Form></Modal>;
}
