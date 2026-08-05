import type { BusinessWorkbenchApi } from '../business-workbench/business-workbench-page';

export type Phase35Api = BusinessWorkbenchApi;
export type Row = Record<string, unknown>;

export function records(payload: unknown): Row[] {
  if (Array.isArray(payload)) return payload.filter((value): value is Row => !!value && typeof value === 'object');
  if (!payload || typeof payload !== 'object') return [];
  const body = payload as Row;
  for (const candidate of [body.list, body.items, body.rows, body.data]) {
    if (Array.isArray(candidate)) return candidate.filter((value): value is Row => !!value && typeof value === 'object');
  }
  return [];
}

export const text = (value: unknown) => value === null || value === undefined || value === '' ? '--' : String(value);
export const numericId = (row: Row) => Number(row.id ?? row.contactId ?? row.roomId ?? 0);
