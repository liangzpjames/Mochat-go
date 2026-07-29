import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Alert, Avatar, Button, Card, Drawer, Input, Space, Table } from 'antd';
import { useState } from 'react';

import { useDashboardAccess } from '../../app/access-context';
import type {
  EmployeeConditions,
  EmployeeListInput,
  EmployeeListResult,
} from './employee-api';

export type EmployeePageApi = {
  list(input: EmployeeListInput): Promise<EmployeeListResult>;
  conditions(): Promise<EmployeeConditions>;
  sync(): Promise<void>;
};

type Filters = Pick<EmployeeListInput, 'name' | 'status' | 'contactAuth'>;
const emptyFilters: Filters = { name: '', status: null, contactAuth: null };

export function EmployeePage({ api }: { api: EmployeePageApi }) {
  const access = useDashboardAccess();
  const queryClient = useQueryClient();
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [draft, setDraft] = useState<Filters>(emptyFilters);
  const [filters, setFilters] = useState<Filters>(emptyFilters);
  const [page, setPage] = useState(1);
  const [perPage, setPerPage] = useState(10);
  const [feedback, setFeedback] = useState<string | null>(null);
  const listKey = ['employee', access.corp.id, 'list'] as const;
  const conditionKey = ['employee', access.corp.id, 'conditions'] as const;
  const listQuery = useQuery({
    queryKey: [...listKey, filters, page, perPage],
    queryFn: () => api.list({ ...filters, page, perPage }),
  });
  const conditionQuery = useQuery({
    queryKey: conditionKey,
    queryFn: () => api.conditions(),
  });
  const syncMutation = useMutation({
    mutationFn: () => api.sync(),
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: listKey }),
        queryClient.invalidateQueries({ queryKey: conditionKey }),
      ]);
      setFeedback('通讯录同步完成');
    },
  });
  const can = (action: string) => access.allowedActions.has(`/workEmployee/index@${action}`);
  const error = listQuery.error ?? conditionQuery.error ?? syncMutation.error;

  return (
    <Card title="企业成员">
      <Space align="center" wrap>
        {can('search') && <Button type="primary" onClick={() => setDrawerOpen(true)}>条件筛选</Button>}
        <span>最后一次同步时间：{conditionQuery.data?.syncTime || '-'}</span>
        {can('sync') && (
          <Button loading={syncMutation.isPending} onClick={() => {
            setFeedback(null);
            syncMutation.mutate();
          }}>同步企业微信通讯录</Button>
        )}
      </Space>
      {error !== null && <Alert role="alert" title={error instanceof Error ? error.message : '加载失败'} type="error" />}
      {feedback !== null && <Alert title={feedback} type="success" />}
      <Table
        columns={[
          { title: '企业成员', dataIndex: 'name', render: (name: string, row: { thumbAvatar: string }) => (
            <Space><Avatar src={row.thumbAvatar || undefined}>{name.slice(0, 1)}</Avatar>{name}</Space>
          ) },
          { title: '状态', dataIndex: 'statusName' },
          { title: '外部联系人权限', dataIndex: 'contactAuthName' },
          { title: '发起申请数', dataIndex: 'applyNums' },
          { title: '新增客户数', dataIndex: 'addNums' },
          { title: '聊天数', dataIndex: 'messageNums' },
          { title: '发送消息数', dataIndex: 'sendMessageNums' },
          { title: '已回复聊天占比', dataIndex: 'replyMessageRatio' },
          { title: '平均首次回复时长', dataIndex: 'averageReply' },
          { title: '删除/拉黑客户数', dataIndex: 'invalidContact' },
        ]}
        dataSource={listQuery.data?.list ?? []}
        loading={listQuery.isLoading}
        locale={{ emptyText: '暂无企业成员' }}
        pagination={{
          current: page, pageSize: perPage, showSizeChanger: true,
          total: listQuery.data?.page.total ?? 0,
          onChange: (nextPage, nextSize) => {
            setPage(nextSize === perPage ? nextPage : 1);
            setPerPage(nextSize);
          },
        }}
        rowKey="id"
        scroll={{ x: 1200 }}
      />
      <Drawer open={drawerOpen} size="default" title="条件筛选" onClose={() => setDrawerOpen(false)}>
        <Space orientation="vertical" style={{ width: '100%' }}>
          <label htmlFor="employee-name">成员姓名</label>
          <Input id="employee-name" value={draft.name} onChange={(event) => setDraft({ ...draft, name: event.target.value })} />
          <label htmlFor="employee-status">成员状态</label>
          <select id="employee-status" value={draft.status ?? ''} onChange={(event) => setDraft({
            ...draft, status: event.target.value === '' ? null : Number(event.target.value),
          })}>
            <option value="">全部</option>
            {conditionQuery.data?.status.map((option) => <option key={option.id} value={option.id}>{option.name}</option>)}
          </select>
          <label htmlFor="employee-contact-auth">外部联系人权限</label>
          <select id="employee-contact-auth" value={draft.contactAuth ?? ''} onChange={(event) => setDraft({
            ...draft, contactAuth: event.target.value === '' ? null : Number(event.target.value),
          })}>
            <option value="">全部</option>
            {conditionQuery.data?.contactAuth.map((option) => <option key={option.id} value={option.id}>{option.name}</option>)}
          </select>
          <Space>
            <Button onClick={() => { setDraft(emptyFilters); setDrawerOpen(false); }}>取消</Button>
            <Button type="primary" onClick={() => {
              setPage(1);
              setFilters(draft);
              setDrawerOpen(false);
            }}>确定</Button>
          </Space>
        </Space>
      </Drawer>
    </Card>
  );
}
