import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Alert, Button, Card, Form, Input, InputNumber, Modal, Popconfirm, Select, Space, Table } from 'antd';
import { useState } from 'react';

import { useDashboardAccess } from '../../app/access-context';
import type {
  ContactFieldBatchInput, ContactFieldItem, ContactFieldListResult, ContactFieldWrite,
} from './contact-field-api';

export type ContactFieldPageApi = {
  list(input: { status: number; page: number; perPage: number }): Promise<ContactFieldListResult>;
  create(input: ContactFieldWrite): Promise<void>;
  update(input: ContactFieldWrite & { id: number }): Promise<void>;
  updateStatus(id: number, status: number): Promise<void>;
  remove(id: number): Promise<void>;
  batchUpdate(input: ContactFieldBatchInput): Promise<void>;
};

const defaults: ContactFieldWrite = { label: '', type: 0, options: [], order: 0, status: 1 };

export function ContactFieldPage({ api }: { api: ContactFieldPageApi }) {
  const access = useDashboardAccess();
  const queryClient = useQueryClient();
  const [status, setStatus] = useState(2);
  const [page, setPage] = useState(1);
  const [perPage, setPerPage] = useState(10);
  const [editor, setEditor] = useState<ContactFieldItem | 'new' | null>(null);
  const [batchMode, setBatchMode] = useState(false);
  const [batchRows, setBatchRows] = useState<ContactFieldItem[]>([]);
  const key = ['contact-field', access.corp.id, 'list'] as const;
  const query = useQuery({
    queryKey: [...key, status, page, perPage],
    queryFn: () => api.list({ status, page, perPage }),
  });
  const refresh = async () => queryClient.invalidateQueries({ queryKey: key });
  const mutation = useMutation({
    mutationFn: async (operation: () => Promise<void>) => operation(),
    onSuccess: refresh,
  });
  const can = (action: string) => access.allowedActions.size === 0 || access.allowedActions.has(`/contactField/index@${action}`);
  if (!can('advanced')) return null;
  const rows = batchMode ? batchRows : (query.data?.list ?? []);
  const error = query.error ?? mutation.error;

  return <Card title="高级属性">
    <Alert type="info" title="系统通用字段只能修改状态和排序；自定义字段可以新增、编辑或删除。" />
    <Space wrap>
      {can('all') && <Select aria-label="状态筛选" value={status} options={[
        { label: '关闭', value: 0 }, { label: '开启', value: 1 }, { label: '全部状态', value: 2 },
      ]} onChange={(value) => { setPage(1); setStatus(value); }} />}
      {!batchMode && can('add') && <Button onClick={() => setEditor('new')}>新增属性</Button>}
      {!batchMode && can('batch') && <Button disabled={rows.length === 0} onClick={() => {
        setBatchRows((query.data?.list ?? []).map((item) => ({ ...item }))); setBatchMode(true);
      }}>批量修改</Button>}
      {batchMode && <>
        <Button onClick={() => setBatchMode(false)}>取消</Button>
        <Button type="primary" onClick={() => mutation.mutate(async () => {
          await api.batchUpdate({ update: batchRows, destroy: [] }); setBatchMode(false);
        })}>提交</Button>
      </>}
    </Space>
    {error !== null && <Alert role="alert" type="error" title={error instanceof Error ? error.message : '操作失败'} />}
    <Table
      columns={[
        { title: '字段名称', dataIndex: 'label', render: (value: string, row: ContactFieldItem, index) =>
          batchMode && row.isSys === 0
            ? <Input value={value} onChange={(event) => setBatchRows(batchRows.map((item, i) =>
              i === index ? { ...item, label: event.target.value } : item))} />
            : value },
        { title: '填写格式', dataIndex: 'typeText' },
        { title: '选项内容', dataIndex: 'options', render: (options: string[]) => options.join('，') || '--' },
        { title: '排序展示', dataIndex: 'order' },
        { title: '状态', render: (_, row: ContactFieldItem) => row.status === 1 ? '开启' : '关闭' },
        { title: '操作', render: (_, row: ContactFieldItem) => batchMode ? null : <Space>
          {can('close') && row.name !== 'gender' && <Button type="link" onClick={() =>
            mutation.mutate(() => api.updateStatus(row.id, row.status === 1 ? 0 : 1))
          }>{row.status === 1 ? '关闭' : '开启'}</Button>}
          {can('edit') && <Button type="link" onClick={() => setEditor(row)}>编辑</Button>}
          {can('edit') && row.isSys === 0 && <Popconfirm title="确认删除该字段？" onConfirm={() =>
            mutation.mutate(() => api.remove(row.id))
          }><Button type="link" danger>删除</Button></Popconfirm>}
        </Space> },
      ]}
      dataSource={rows}
      loading={query.isLoading}
      pagination={{ current: page, pageSize: perPage, total: query.data?.page.total ?? 0,
        showSizeChanger: true, onChange: (next, size) => {
          setPage(size === perPage ? next : 1); setPerPage(size);
        } }}
      rowKey="id"
    />
    <FieldEditor editor={editor} loading={mutation.isPending} onCancel={() => setEditor(null)}
      onSave={(values) => mutation.mutate(async () => {
        const normalized = { ...defaults, ...values };
        if (editor === 'new') await api.create(normalized);
        else if (editor !== null) await api.update({ id: editor.id, ...normalized });
        setEditor(null);
      })} />
  </Card>;
}

function FieldEditor({ editor, loading, onCancel, onSave }: {
  editor: ContactFieldItem | 'new' | null; loading: boolean;
  onCancel: () => void; onSave: (values: ContactFieldWrite) => void;
}) {
  const [form] = Form.useForm<ContactFieldWrite>();
  const initial = editor === 'new' || editor === null ? defaults : editor;
  return <Modal open={editor !== null} title={editor === 'new' ? '新增属性' : '编辑属性'}
    onCancel={onCancel} onOk={() => void form.validateFields().then(onSave)}
    okButtonProps={{ loading }} okText="确定" forceRender>
    <Form form={form} initialValues={initial} key={editor === 'new' ? 'new' : editor?.id ?? 'closed'} layout="vertical">
      <Form.Item label="字段名称" name="label" rules={[{ required: true }]}><Input maxLength={8} disabled={editor !== 'new' && editor?.isSys === 1} /></Form.Item>
      <Form.Item label="字段类型" name="type"><Select disabled={editor !== 'new'} options={[
        { label: '文本', value: 0 }, { label: '单选', value: 1 }, { label: '多选', value: 2 },
        { label: '日期', value: 6 }, { label: '手机号', value: 9 }, { label: '邮箱', value: 10 },
        { label: '区域', value: 5 }, { label: '图片', value: 11 },
      ]} /></Form.Item>
      <Form.Item label="排序展示" name="order"><InputNumber /></Form.Item>
      <Form.Item label="使用状态" name="status"><Select options={[
        { label: '开启', value: 1 }, { label: '关闭', value: 0 },
      ]} /></Form.Item>
    </Form>
  </Modal>;
}
