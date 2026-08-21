import type { LiveCodeRecord, LiveCodeStatisticsPage } from './live-code-types';

function isRecord(value: unknown): value is LiveCodeRecord {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

export function recordsFrom(payload: unknown): LiveCodeRecord[] {
  if (Array.isArray(payload)) return payload.filter(isRecord);
  if (!isRecord(payload)) return [];
  const data = isRecord(payload.data) ? payload.data : payload;
  const candidates = [data.list, data.items, data.rows, isRecord(data.data) ? data.data.list : undefined];
  const list = candidates.find(Array.isArray);
  return Array.isArray(list) ? list.filter(isRecord) : [];
}

export function paginationFrom(payload: unknown, fallbackTotal: number) {
  if (!isRecord(payload)) return { total: fallbackTotal, totalPage: fallbackTotal > 0 ? 1 : 0, perPage: 20 };
  const data = isRecord(payload.data) ? payload.data : payload;
  const page = isRecord(data.page) ? data.page : data;
  const total = Number(page.total ?? data.total ?? fallbackTotal);
  const perPage = Number(page.perPage ?? 20);
  const totalPage = Number(page.totalPage ?? (total > 0 ? Math.ceil(total / Math.max(perPage, 1)) : 0));
  return {
    total: Number.isFinite(total) ? total : fallbackTotal,
    totalPage: Number.isFinite(totalPage) ? totalPage : 0,
    perPage: Number.isFinite(perPage) && perPage > 0 ? perPage : 20,
  };
}

export function statisticsFrom(payload: unknown): LiveCodeStatisticsPage {
  const data = isRecord(payload) && isRecord(payload.data) ? payload.data : isRecord(payload) ? payload : {};
  const source = isRecord(data.summary) ? data.summary : {};
  const rows = recordsFrom(data.list ?? data.rows);
  const page = paginationFrom(data.page, rows.length);
  return {
    summary: {
      addedAttempts: Number(source.addedAttempts ?? 0),
      lostAttempts: Number(source.lostAttempts ?? 0),
      addedCustomers: Number(source.addedCustomers ?? 0),
      retainedCustomers: Number(source.retainedCustomers ?? 0),
      codeCount: Number(source.codeCount ?? page.total),
      available: source.available !== false,
      asOf: textOf(source.asOf, ''),
      timezone: textOf(source.timezone, 'Asia/Shanghai'),
      definition: textOf(source.definition, '按渠道码归因的客户关联状态变化'),
    },
    rows,
    total: page.total,
    totalPage: page.totalPage,
    perPage: page.perPage,
  };
}

export function textOf(value: unknown, fallback = '—'): string {
  if (value === null || value === undefined || value === '') return fallback;
  if (Array.isArray(value)) return value.map((item) => textOf(item, '')).filter(Boolean).join('、') || fallback;
  if (isRecord(value)) return textOf(value.name ?? value.title ?? value.label, fallback);
  if (typeof value === 'string') return value;
  if (typeof value === 'number' || typeof value === 'boolean' || typeof value === 'bigint') return value.toString();
  return fallback;
}

export function numberOf(value: unknown): number {
  const number = Number(value);
  return Number.isFinite(number) ? number : 0;
}

export function stateText(value: unknown): string {
  switch (textOf(value, '')) {
    case 'active': return '运行中';
    case 'paused': return '已暂停';
    case 'expired': return '已过期';
    case 'draft': return '待配置';
    default: return textOf(value, '待确认');
  }
}

export function stateClass(value: unknown): string {
  switch (textOf(value, '')) {
    case 'active': return 'is-active';
    case 'paused': return 'is-paused';
    case 'expired': return 'is-expired';
    default: return 'is-draft';
  }
}
