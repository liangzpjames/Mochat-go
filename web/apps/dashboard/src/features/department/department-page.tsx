import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Alert, Button, Card, Input, Modal, Space, Table } from 'antd';
import { useState } from 'react';

import { useDashboardAccess } from '../../app/access-context';
import { DashboardPagination } from '../../components/dashboard-pagination';
import type { EmployeeConditions } from '../employee/employee-api';
import type {
  DepartmentListInput, DepartmentListResult, DepartmentMemberInput,
  DepartmentMemberResult,
} from './department-api';

export type DepartmentPageApi = {
  list(input: DepartmentListInput): Promise<DepartmentListResult>;
  members(input: DepartmentMemberInput): Promise<DepartmentMemberResult>;
  conditions(): Promise<EmployeeConditions>;
  sync(): Promise<void>;
};

export function DepartmentPage({ api }: { api: DepartmentPageApi }) {
  const access = useDashboardAccess();
  const queryClient = useQueryClient();
  const [draftName, setDraftName] = useState('');
  const [draftParent, setDraftParent] = useState('');
  const [filters, setFilters] = useState({ name: '', parentName: '' });
  const [page, setPage] = useState(1);
  const [perPage, setPerPage] = useState(10);
  const [departmentId, setDepartmentId] = useState<number | null>(null);
  const [memberPage, setMemberPage] = useState(1);
  const [memberPerPage, setMemberPerPage] = useState(10);
  const listKey = ['department', access.corp.id, 'list'] as const;
  const conditionKey = ['employee', access.corp.id, 'conditions'] as const;
  const listQuery = useQuery({
    queryKey: [...listKey, filters, page, perPage],
    queryFn: () => api.list({ ...filters, page, perPage }),
  });
  const conditionQuery = useQuery({ queryKey: conditionKey, queryFn: () => api.conditions() });
  const memberQuery = useQuery({
    enabled: departmentId !== null,
    queryKey: ['department', access.corp.id, 'members', departmentId, memberPage, memberPerPage],
    queryFn: () => api.members({ departmentId: departmentId ?? 0, page: memberPage, perPage: memberPerPage }),
  });
  const syncMutation = useMutation({
    mutationFn: () => api.sync(),
    onSuccess: async () => Promise.all([
      queryClient.invalidateQueries({ queryKey: listKey }),
      queryClient.invalidateQueries({ queryKey: conditionKey }),
    ]),
  });
  const can = (action: string) => access.allowedActions.size === 0 || access.allowedActions.has(`/department/index@${action}`);
  const error = listQuery.error ?? conditionQuery.error ?? memberQuery.error ?? syncMutation.error;

  return <Card title="组织架构">
    <Space wrap>
      <span>最后一次同步时间：{conditionQuery.data?.syncTime || '-'}</span>
      {can('sync') && <Button loading={syncMutation.isPending} onClick={() => syncMutation.mutate()}>同步企业微信通讯录</Button>}
    </Space>
    <Space wrap>
      <label htmlFor="department-name">组织名称</label>
      <Input id="department-name" value={draftName} onChange={(event) => setDraftName(event.target.value)} />
      <label htmlFor="department-parent">上级组织名称</label>
      <Input id="department-parent" value={draftParent} onChange={(event) => setDraftParent(event.target.value)} />
      {can('search') && <Button type="primary" onClick={() => {
        setPage(1); setFilters({ name: draftName, parentName: draftParent });
      }}>查询</Button>}
      <Button onClick={() => {
        setDraftName(''); setDraftParent(''); setPage(1); setFilters({ name: '', parentName: '' });
      }}>重置</Button>
    </Space>
    {error !== null && <Alert role="alert" type="error" title={error instanceof Error ? error.message : '加载失败'} />}
    <Table
      columns={[
        { title: '序号', dataIndex: 'departmentPath' },
        { title: '组织架构名称', dataIndex: 'name' },
        { title: '部门级别', dataIndex: 'level' },
        { title: '操作', render: (_, row: { departmentId: number }) => can('check')
          ? <Button type="link" onClick={() => { setMemberPage(1); setDepartmentId(row.departmentId); }}>查看成员</Button>
          : null },
      ]}
      dataSource={listQuery.data?.list ?? []}
      loading={listQuery.isLoading}
      pagination={false}
      rowKey="departmentId"
    />
    <DashboardPagination page={page} pageSize={perPage} total={listQuery.data?.page.total ?? 0}
      onPageChange={setPage} onPageSizeChange={(size) => { setPage(1); setPerPage(size); }} />
    <Modal footer={null} open={departmentId !== null} title="查看人员" onCancel={() => setDepartmentId(null)}>
      <Table
        columns={[
          { title: '姓名', dataIndex: 'employeeName' },
          { title: '手机号码', dataIndex: 'phone' },
          { title: '角色', dataIndex: 'roleName' },
        ]}
        dataSource={memberQuery.data?.list ?? []}
        loading={memberQuery.isLoading}
        pagination={false}
        rowKey="employeeId"
      />
      <DashboardPagination page={memberPage} pageSize={memberPerPage} total={memberQuery.data?.page.total ?? 0}
        onPageChange={setMemberPage} onPageSizeChange={(size) => { setMemberPage(1); setMemberPerPage(size); }} />
    </Modal>
  </Card>;
}
