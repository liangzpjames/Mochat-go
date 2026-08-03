import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  Alert,
  Button,
  Card,
  Empty,
  Form,
  Input,
  Select,
  Space,
  Spin,
  Statistic,
  Table,
  Typography,
} from 'antd';
import { useMemo, useState } from 'react';

import { useDashboardAccess } from '../../app/access-context';
import type { BusinessField, BusinessRouteConfig } from './catalog';

type QueryValues = Record<string, string | number>;
type BusinessRecord = Record<string, unknown>;

export type BusinessWorkbenchApi = {
  read(endpoint: string, query: QueryValues): Promise<unknown>;
  write(endpoint: string, values: Record<string, unknown>, method?: 'POST' | 'PUT' | 'DELETE'): Promise<void>;
};

function normalizeRecords(payload: unknown): { rows: BusinessRecord[]; total: number } {
  if (Array.isArray(payload)) {
    return { rows: payload.filter((item): item is BusinessRecord =>
      typeof item === 'object' && item !== null), total: payload.length };
  }
  if (typeof payload !== 'object' || payload === null) {
    return { rows: [], total: 0 };
  }
  const source = payload as Record<string, unknown>;
  const candidates = [source.list, source.data, source.items, source.rows];
  const list = candidates.find(Array.isArray);
  const rows = Array.isArray(list)
    ? list.filter((item): item is BusinessRecord => typeof item === 'object' && item !== null)
    : [source];
  const page = typeof source.page === 'object' && source.page !== null
    ? source.page as Record<string, unknown>
    : {};
  const total = Number(page.total ?? source.total ?? rows.length);
  return { rows, total: Number.isFinite(total) ? total : rows.length };
}

function displayValue(value: unknown): string {
  if (value === null || value === undefined || value === '') return '--';
  if (Array.isArray(value)) return value.map(displayValue).join('、');
  if (typeof value === 'object') return JSON.stringify(value);
  if (typeof value === 'string') return value;
  if (typeof value === 'number' || typeof value === 'boolean' || typeof value === 'bigint') {
    return value.toString();
  }
  return '--';
}

function recordKey(record: BusinessRecord): string {
  const candidate = record.id ?? record[Object.keys(record)[0] ?? ''];
  if (typeof candidate === 'string') return candidate;
  if (typeof candidate === 'number' || typeof candidate === 'bigint') return candidate.toString();
  return JSON.stringify(record);
}

function fieldControl(field: BusinessField) {
  if (field.kind === 'textarea') {
    return <Input.TextArea aria-label={field.label} rows={4} />;
  }
  if (field.kind === 'select') {
    return (
      <Select
        aria-label={field.label}
        options={(field.options ?? []).map((value) => ({ label: value, value }))}
      />
    );
  }
  return <Input aria-label={field.label} />;
}

function indexPath(path: string): string {
  const feature = path.split('/').filter(Boolean)[0] ?? '';
  return `/${feature}/index`;
}

export function BusinessWorkbenchPage({
  config,
  api,
  navigate,
}: {
  config: BusinessRouteConfig;
  api: BusinessWorkbenchApi;
  navigate: (path: string) => void;
}) {
  const access = useDashboardAccess();
  const queryClient = useQueryClient();
  const [draftFilters, setDraftFilters] = useState<Record<string, string>>({});
  const [filters, setFilters] = useState<Record<string, string>>({});
  const [page, setPage] = useState(1);
  const [perPage, setPerPage] = useState(10);
  const [form] = Form.useForm<Record<string, unknown>>();
  const queryKey = ['business-workbench', access.corp.id, config.path, filters, page, perPage];
  const query = useQuery({
    queryKey,
    queryFn: () => api.read(config.readEndpoint, { ...filters, page, perPage }),
    enabled: config.mode !== 'form',
  });
  const normalized = useMemo(() => normalizeRecords(query.data), [query.data]);
  const mutation = useMutation({
    mutationFn: (input: { endpoint: string; values: Record<string, unknown> }) =>
      api.write(input.endpoint, input.values),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['business-workbench', access.corp.id] });
    },
  });
  const columns = useMemo(() => {
    const first = normalized.rows[0];
    const keys = first === undefined ? [] : Object.keys(first).slice(0, 6);
    return keys.map((key) => ({
      title: key,
      dataIndex: key,
      key,
      render: (value: unknown) => displayValue(value),
    }));
  }, [normalized.rows]);
  const can = (action: string) =>
    access.allowedActions.has(`${config.path}@${action}`) ||
    access.allowedActions.has(`${indexPath(config.path)}@${action}`) ||
    access.allowedActions.size === 0;

  async function save(values: Record<string, unknown>) {
    if (config.writeEndpoint === undefined) return;
    await mutation.mutateAsync({ endpoint: config.writeEndpoint, values });
    navigate(indexPath(config.path));
  }

  const extra = (
    <Space wrap>
      {config.actions.includes('refresh') && (
        <Button onClick={() => void query.refetch()}>刷新</Button>
      )}
      {config.actions.includes('back') && (
        <Button onClick={() => navigate(indexPath(config.path))}>返回</Button>
      )}
    </Space>
  );

  if (config.mode === 'form') {
    return (
      <Card title={config.title} extra={extra}>
        <Typography.Paragraph type="secondary">{config.description}</Typography.Paragraph>
        <Form form={form} layout="vertical" onFinish={(values) => void save(values)}>
          {config.fields.map((field) => (
            <Form.Item
              key={field.key}
              name={field.key}
              label={field.label}
              rules={field.key === 'name' ? [{ required: true, message: `请输入${field.label}` }] : []}
            >
              {fieldControl(field)}
            </Form.Item>
          ))}
          {config.actions.includes('save') && can('save') && (
            <Button type="primary" htmlType="submit" loading={mutation.isPending}>保存</Button>
          )}
        </Form>
      </Card>
    );
  }

  return (
    <Card title={config.title} extra={extra}>
      <Typography.Paragraph type="secondary">{config.description}</Typography.Paragraph>
      {(config.mode === 'list' || config.mode === 'statistics') && (
        <Space wrap align="end">
          {config.fields.map((field) => (
            <label key={field.key}>
              <span>{field.label}</span>
              {field.kind === 'select' ? (
                <Select
                  aria-label={field.label}
                  style={{ minWidth: 140 }}
                  value={draftFilters[field.key] ?? null}
                  options={(field.options ?? []).map((value) => ({ label: value, value }))}
                  onChange={(value) => setDraftFilters((current) => ({
                    ...current,
                    [field.key]: value ?? '',
                  }))}
                />
              ) : (
                <Input
                  aria-label={field.label}
                  value={draftFilters[field.key] ?? ''}
                  onChange={(event) => setDraftFilters((current) => ({
                    ...current,
                    [field.key]: event.target.value,
                  }))}
                />
              )}
            </label>
          ))}
          {config.actions.includes('search') && can('search') && (
            <Button type="primary" onClick={() => { setPage(1); setFilters(draftFilters); }}>
              查询
            </Button>
          )}
          {config.actions.includes('reset') && (
            <Button onClick={() => { setDraftFilters({}); setFilters({}); setPage(1); }}>重置</Button>
          )}
          {config.actions.includes('create') && can('add') && (
            <Button onClick={() => navigate(config.path.replace(/\/index$/, '/store'))}>新建</Button>
          )}
          {config.actions.includes('sync') && can('sync') && (
            <Button
              loading={mutation.isPending}
              onClick={() => mutation.mutate({
                endpoint: `/${config.path.split('/').filter(Boolean)[0]}/syn`,
                values: {},
              })}
            >
              同步数据
            </Button>
          )}
        </Space>
      )}
      {query.isPending && <Spin aria-label="正在加载" />}
      {query.isError && <Alert type="error" title="数据加载失败" />}
      {query.isSuccess && normalized.rows.length === 0 && <Empty description="暂无数据" />}
      {query.isSuccess && normalized.rows.length > 0 && config.mode === 'statistics' && (
        <Space wrap>
          <Statistic title="记录总数" value={normalized.total} />
          <Statistic title="当前页" value={page} />
        </Space>
      )}
      {query.isSuccess && normalized.rows.length > 0 && (
        <Table
          rowKey={recordKey}
          columns={columns}
          dataSource={normalized.rows}
          pagination={{
            current: page,
            pageSize: perPage,
            total: normalized.total,
            onChange: (nextPage, nextPerPage) => {
              setPage(nextPerPage === perPage ? nextPage : 1);
              setPerPage(nextPerPage);
            },
          }}
        />
      )}
    </Card>
  );
}
