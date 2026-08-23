export type AISettingsListState = {
  q: string;
  status: 'all' | 'enabled' | 'disabled';
  page: number;
  pageSize: 10 | 20 | 50;
};

type ListItem = { name: string; description: string; status: number };
type KnowledgeBaseName = { id: string; name: string };

const pageSizes = new Set<AISettingsListState['pageSize']>([10, 20, 50]);

function positiveInteger(value: string | null): number | undefined {
  if (!value || !/^\d+$/.test(value)) return undefined;
  const number = Number(value);
  return Number.isSafeInteger(number) && number > 0 ? number : undefined;
}

function normalizePageSize(value: number): AISettingsListState['pageSize'] {
  return pageSizes.has(value as AISettingsListState['pageSize'])
    ? value as AISettingsListState['pageSize']
    : 10;
}

export function parseAISettingsListState(input: URLSearchParams | string): AISettingsListState {
  const params = input instanceof URLSearchParams ? input : new URLSearchParams(input);
  const status = params.get('status');
  return {
    q: (params.get('q') ?? '').trim(),
    status: status === 'enabled' || status === 'disabled' ? status : 'all',
    page: positiveInteger(params.get('page')) ?? 1,
    pageSize: normalizePageSize(positiveInteger(params.get('pageSize')) ?? 10),
  };
}

export function filterAndPageAISettings<T extends ListItem>(items: readonly T[], state: AISettingsListState) {
  const keyword = state.q.trim().toLocaleLowerCase();
  const pageSize = normalizePageSize(state.pageSize);
  const filtered = items.filter((item) => {
    const matchesKeyword = !keyword || `${item.name} ${item.description}`.toLocaleLowerCase().includes(keyword);
    const matchesStatus = state.status === 'all'
      || item.status === (state.status === 'enabled' ? 1 : 0);
    return matchesKeyword && matchesStatus;
  });
  const pages = Math.max(1, Math.ceil(filtered.length / pageSize));
  const requestedPage = Number.isSafeInteger(state.page) && state.page > 0 ? state.page : 1;
  const page = Math.min(requestedPage, pages);
  const start = (page - 1) * pageSize;
  return { items: filtered.slice(start, start + pageSize), total: filtered.length, page, pageSize };
}

export function formatAISettingsTime(value: string | null | undefined): string {
  if (!value) return '—';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '—';
  const twoDigits = (part: number) => String(part).padStart(2, '0');
  return `${date.getFullYear()}-${twoDigits(date.getMonth() + 1)}-${twoDigits(date.getDate())} ${twoDigits(date.getHours())}:${twoDigits(date.getMinutes())}`;
}

export function resolveKnowledgeBaseNames(ids: readonly string[], knowledgeBases: readonly KnowledgeBaseName[]): string {
  if (ids.length === 0) return '—';
  const names = new Map(knowledgeBases.map((knowledgeBase) => [knowledgeBase.id, knowledgeBase.name]));
  return ids.map((id) => names.get(id) ?? `已失效（ID: ${id}）`).join('、');
}
