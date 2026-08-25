import type { DerivedInsightFilters, DerivedInsightView, EmotionLabel, InsightStatus, SessionInsightFilters, SmartInsightFilters } from './ai-insight-workspace-api';

function positive(value: string | null): number | undefined { if (!value) return undefined; const parsed = Number(value); return Number.isInteger(parsed) && parsed > 0 ? parsed : undefined; }
function validDate(value: string | null): string | undefined { if (!value || !/^\d{4}-\d{2}-\d{2}$/.test(value)) return undefined; const [year, month, day] = value.split('-').map(Number); const date = new Date(Date.UTC(year ?? 0, (month ?? 1) - 1, day ?? 0)); return date.getUTCFullYear() === year && date.getUTCMonth() + 1 === month && date.getUTCDate() === day ? value : undefined; }
function insightStatus(value: string | null): InsightStatus | undefined { return value && ['pending', 'running', 'succeeded', 'failed'].includes(value) ? value as InsightStatus : undefined; }

function readCommon(params: URLSearchParams): SessionInsightFilters {
  const filters: SessionInsightFilters = { page: positive(params.get('page')) ?? 1 };
  const employeeId = positive(params.get('employeeId')); if (employeeId !== undefined) filters.employeeId = employeeId;
  const customerName = params.get('customerName') || ''; if (customerName) filters.customerName = customerName;
  const targetId = params.get('targetId') || ''; if (targetId) filters.targetId = targetId;
  const keyword = params.get('keyword') || ''; if (keyword) filters.keyword = keyword;
  const conversationType = params.get('conversationType'); if (conversationType === 'direct' || conversationType === 'group') filters.conversationType = conversationType;
  const status = insightStatus(params.get('status')); if (status) filters.status = status;
  const startDate = validDate(params.get('startDate')); if (startDate) filters.startDate = startDate;
  const endDate = validDate(params.get('endDate')); if (endDate) filters.endDate = endDate;
  return filters;
}

export function readSessionFilters(input: string | URL = window.location.href): SessionInsightFilters { return readCommon(new URL(input, window.location.origin).searchParams); }
export function readSmartState(input: string | URL = window.location.href): { filters: SmartInsightFilters } { const url = new URL(input, window.location.origin); return { filters: { ...readCommon(url.searchParams) } }; }
function writeCommon(params: URLSearchParams, filters: SessionInsightFilters): void { params.delete('page'); if (filters.page > 1) params.set('page', String(filters.page)); for (const key of ['employeeId', 'customerName', 'conversationType', 'targetId', 'keyword', 'status', 'startDate', 'endDate'] as const) { params.delete(key); const value = filters[key]; if (value !== undefined && value !== '') params.set(key, String(value)); } }
export function writeSessionFilters(filters: SessionInsightFilters, input: string | URL = window.location.href): string { const url = new URL(input, window.location.origin); writeCommon(url.searchParams, filters); return url.toString(); }
export function writeSmartState(state: { filters: SmartInsightFilters }, input: string | URL = window.location.href): string { const url = new URL(input, window.location.origin); url.searchParams.delete('tab'); url.searchParams.delete('ruleVersionId'); writeCommon(url.searchParams, state.filters); return url.toString(); }

function score(value: string | null): number | undefined { if (value === null || value === '') return undefined; const parsed = Number(value); return Number.isInteger(parsed) && parsed >= 0 && parsed <= 100 ? parsed : undefined; }

export function readDerivedFilters(view: DerivedInsightView, input: string | URL = window.location.href): DerivedInsightFilters {
  const params = new URL(input, window.location.origin).searchParams;
  const common = readCommon(params);
  const filters: DerivedInsightFilters = { page: common.page };
  const customerId = positive(params.get('customerId')); if (customerId !== undefined) filters.customerId = customerId;
  for (const key of ['employeeId', 'customerName', 'status', 'startDate', 'endDate'] as const) { const value = common[key]; if (value !== undefined) Object.assign(filters, { [key]: value }); }
  if (view === 'emotion') {
    const emotion = params.get('emotion');
    if (emotion && ['positive', 'neutral', 'negative', 'mixed', 'unknown'].includes(emotion)) filters.emotion = emotion as EmotionLabel;
  } else if (view === 'employee-score') {
    const minScore = score(params.get('minScore'));
    const maxScore = score(params.get('maxScore'));
    if (minScore !== undefined && maxScore !== undefined && minScore > maxScore) return filters;
    if (minScore !== undefined) filters.minScore = minScore;
    if (maxScore !== undefined) filters.maxScore = maxScore;
  } else {
    const keyword = params.get('keyword') || '';
    if (keyword) filters.keyword = keyword;
  }
  return filters;
}

export function writeDerivedFilters(view: DerivedInsightView, filters: DerivedInsightFilters, input: string | URL = window.location.href): string {
  const url = new URL(input, window.location.origin);
  for (const key of ['page', 'employeeId', 'customerId', 'customerName', 'status', 'startDate', 'endDate', 'emotion', 'minScore', 'maxScore', 'keyword']) url.searchParams.delete(key);
  if (filters.page > 1) url.searchParams.set('page', String(filters.page));
  for (const key of ['employeeId', 'customerId', 'customerName', 'status', 'startDate', 'endDate'] as const) { const value = filters[key]; if (value !== undefined && value !== '') url.searchParams.set(key, String(value)); }
  if (view === 'emotion' && filters.emotion) url.searchParams.set('emotion', filters.emotion);
  if (view === 'employee-score') { if (filters.minScore !== undefined) url.searchParams.set('minScore', String(filters.minScore)); if (filters.maxScore !== undefined) url.searchParams.set('maxScore', String(filters.maxScore)); }
  if (view === 'communication-keyword' && filters.keyword) url.searchParams.set('keyword', filters.keyword);
  return url.toString();
}
