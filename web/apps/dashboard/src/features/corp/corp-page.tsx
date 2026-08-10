import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Alert, Button, Card, Form, Input, Modal, Space, Table } from 'antd';
import { useState } from 'react';

import { useDashboardAccess } from '../../app/access-context';
import type {
  CorpDetail,
  CorpListInput,
  CorpListResult,
  CorpUpdateInput,
  CorpWriteInput,
} from './corp-admin-api';

export type CorpPageApi = {
  list(input: CorpListInput): Promise<CorpListResult>;
  show(corpId: number): Promise<CorpDetail>;
  create(input: CorpWriteInput): Promise<void>;
  update(input: CorpUpdateInput): Promise<void>;
};

type EditorState = {
  mode: 'create' | 'edit' | 'view';
  detail?: CorpDetail;
};

export function CorpPage({ api }: { api: CorpPageApi }) {
  const access = useDashboardAccess();
  const queryClient = useQueryClient();
  const [corpName, setCorpName] = useState('');
  const [search, setSearch] = useState('');
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [editor, setEditor] = useState<EditorState | null>(null);
  const [feedback, setFeedback] = useState<string | null>(null);
  const [detailError, setDetailError] = useState<string | null>(null);
  const queryKey = ['corp', access.corp.id, 'admin-list'] as const;
  const listQuery = useQuery({
    queryKey: [...queryKey, search, page, pageSize],
    queryFn: () => api.list({
      corpId: access.corp.id,
      corpName: search,
      page,
      perPage: pageSize,
    }),
  });
  const saveMutation = useMutation({
    mutationFn: async (values: CorpWriteInput) => {
      if (editor?.mode === 'edit' && editor.detail !== undefined) {
        await api.update({ corpId: editor.detail.corpId, ...values });
      } else {
        await api.create(values);
      }
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey });
      setEditor(null);
      setFeedback('保存成功');
    },
  });

  const can = (action: string) => access.allowedActions.size === 0 || access.allowedActions.has(`/corp/index@${action}`);
  const openDetail = async (corpId: number, mode: 'edit' | 'view') => {
    setFeedback(null);
    setDetailError(null);
    try {
      setEditor({ mode, detail: await api.show(corpId) });
    } catch (error) {
      setDetailError(error instanceof Error ? error.message : '详情加载失败');
    }
  };
  const error = detailError ?? (listQuery.error instanceof Error
    ? listQuery.error.message
    : saveMutation.error instanceof Error
      ? saveMutation.error.message
      : null);

  return (
    <Card title="企业微信授权">
      <Space wrap>
        <Input
          onChange={(event) => setCorpName(event.target.value)}
          placeholder="搜索企业微信名称"
          value={corpName}
        />
        {can('search') && (
          <Button type="primary" onClick={() => {
            setPage(1);
            setSearch(corpName);
          }}>查找</Button>
        )}
        <Button onClick={() => {
          setCorpName('');
          setPage(1);
          setSearch('');
        }}>清空</Button>
        {can('addwx') && (listQuery.data?.list.length ?? 0) < 1 && (
          <Button type="primary" onClick={() => setEditor({ mode: 'create' })}>
            添加企业微信
          </Button>
        )}
      </Space>
      {error !== null && <Alert title={error} role="alert" type="error" />}
      {feedback !== null && <Alert title={feedback} type="success" />}
      <Table
        columns={[
          { title: '企业名称', dataIndex: 'corpName' },
          { title: '企业ID', dataIndex: 'wxCorpId' },
          { title: '绑定时间', dataIndex: 'createdAt' },
          {
            title: '操作',
            render: (_, record) => (
              <Space>
                {can('check') && (
                  <Button type="link" onClick={() => void openDetail(record.corpId, 'view')}>
                    查看
                  </Button>
                )}
                {can('edit') && (
                  <Button type="link" onClick={() => void openDetail(record.corpId, 'edit')}>
                    修改
                  </Button>
                )}
              </Space>
            ),
          },
        ]}
        dataSource={listQuery.data?.list ?? []}
        loading={listQuery.isLoading}
        locale={{ emptyText: '暂无企业微信授权' }}
        pagination={{
          current: page,
          pageSize,
          showSizeChanger: true,
          total: listQuery.data?.page.total ?? 0,
          onChange: (nextPage, nextPageSize) => {
            setPage(nextPageSize === pageSize ? nextPage : 1);
            setPageSize(nextPageSize);
          },
        }}
        rowKey="corpId"
      />
      <CorpEditor
        editor={editor}
        loading={saveMutation.isPending}
        onCancel={() => setEditor(null)}
        onSave={(values) => saveMutation.mutate(values)}
      />
    </Card>
  );
}

function CorpEditor({
  editor,
  loading,
  onCancel,
  onSave,
}: {
  editor: EditorState | null;
  loading: boolean;
  onCancel: () => void;
  onSave: (values: CorpWriteInput) => void;
}) {
  const [form] = Form.useForm<CorpWriteInput>();
  const readOnly = editor?.mode === 'view';
  return (
    <Modal
      footer={readOnly ? <Button onClick={onCancel}>关闭</Button> : undefined}
      forceRender
      onCancel={onCancel}
      onOk={() => void form.validateFields().then(onSave)}
      okButtonProps={{ loading }}
      okText="保存配置"
      open={editor !== null}
      title={editor?.mode === 'create' ? '添加企业微信' : readOnly ? '查看企业微信' : '修改企业微信'}
    >
      <Form
        form={form}
        {...(editor?.detail === undefined ? {} : { initialValues: editor.detail })}
        key={`${editor?.mode ?? 'closed'}-${editor?.detail?.corpId ?? 'new'}`}
        layout="vertical"
      >
        <Form.Item label="企业名称" name="corpName" rules={[{ required: true }]}>
          <Input disabled={readOnly} />
        </Form.Item>
        <Form.Item label="企业ID" name="wxCorpId" rules={[{ required: true }]}>
          <Input disabled={readOnly} maxLength={18} />
        </Form.Item>
        <Form.Item label="自建应用 Secret" name="employeeSecret" rules={[{ required: true }]}>
          <Input disabled={readOnly} maxLength={43} />
        </Form.Item>
        <Form.Item label="外部联系人管理 Secret" name="contactSecret" rules={[{ required: true }]}>
          <Input disabled={readOnly} maxLength={43} />
        </Form.Item>
        {editor?.detail?.eventCallback !== undefined && (
          <Form.Item label="通讯录事件服务器 URL">
            <Input disabled value={editor.detail.eventCallback} />
          </Form.Item>
        )}
        {readOnly && editor?.detail !== undefined && (
          <>
            <Form.Item label="Token">
              <Input disabled value={editor.detail.token} />
            </Form.Item>
            <Form.Item label="EncodingAESKey">
              <Input disabled value={editor.detail.encodingAesKey} />
            </Form.Item>
          </>
        )}
      </Form>
    </Modal>
  );
}
