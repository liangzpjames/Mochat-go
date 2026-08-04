import { useQuery } from '@tanstack/react-query';
import { useMemo } from 'react';

import { useDashboardAccess } from '../../../app/access-context';
import type { BusinessWorkbenchApi } from '../../business-workbench/business-workbench-page';

export type MaterialSelectorItem = {
  id: number;
  name: string;
  type: string;
  preview: string;
  groupId?: number;
  groupName?: string;
  scopeType?: string;
};

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function text(value: unknown): string {
  return typeof value === 'string' && value.trim() ? value.trim() : '';
}

function payloadRows(payload: unknown): unknown[] {
  if (Array.isArray(payload)) return payload;
  if (!isRecord(payload)) return [];
  const source = isRecord(payload.data) ? payload.data : payload;
  const list = source.list ?? source.items ?? source.rows;
  return Array.isArray(list) ? list : [];
}

export function materialSelectorItems(payload: unknown): MaterialSelectorItem[] {
  return payloadRows(payload).flatMap((value) => {
    if (!isRecord(value)) return [];
    const id = Number(value.id);
    if (!Number.isInteger(id) || id <= 0) return [];
    const content = isRecord(value.content) ? value.content : {};
    const name = text(value.name) || text(content.title) || text(content.name) || `素材 #${id}`;
    const preview = text(value.preview) || text(content.content) || text(content.description) || name;
    return [{
      id,
      name,
      type: text(value.type) || '素材',
      preview,
      ...(Number.isInteger(Number(value.groupId)) ? { groupId: Number(value.groupId) } : {}),
      ...(text(value.groupName) ? { groupName: text(value.groupName) } : {}),
      ...(text(value.scopeType) ? { scopeType: text(value.scopeType) } : {}),
    }];
  });
}

export function MaterialSelector({
  api,
  scene,
  value,
  onChange,
  label = '引用素材',
  disabled = false,
}: {
  api: BusinessWorkbenchApi;
  scene: string;
  value: number | null | undefined;
  onChange: (item: MaterialSelectorItem | null) => void;
  label?: string;
  disabled?: boolean;
}) {
  const access = useDashboardAccess();
  const query = useQuery({
    queryKey: ['phase34-material-selector', access.corp.id, scene],
    queryFn: () => api.read('/materialSelector/index', { scene }),
    enabled: access.corp.authorized,
  });
  const items = useMemo(() => materialSelectorItems(query.data), [query.data]);
  const selected = items.find((item) => item.id === Number(value));

  return (
    <div className="phase34-material-selector">
      <label>
        <span>{label}</span>
        <select
          aria-label={label}
          value={value ? String(value) : ''}
          disabled={disabled || query.isPending || query.isError}
          onChange={(event) => {
            const item = items.find((candidate) => candidate.id === Number(event.target.value));
            onChange(item ?? null);
          }}
        >
          <option value="">不引用，手动填写</option>
          {items.map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}
        </select>
      </label>
      {query.isPending && <small>正在加载当前企业可引用素材…</small>}
      {query.isError && <small role="alert">素材加载失败，请稍后重试。</small>}
      {selected && <small className="phase34-material-selector-preview">预览：{selected.preview}</small>}
    </div>
  );
}
