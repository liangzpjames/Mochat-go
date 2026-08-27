import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react';
import {
  AiInsightField,
  AiInsightHeader,
  AiInsightQueryBar,
  Avatar,
  CustomerSearchField,
  EmployeeSearchField,
  InsightDrawer,
  InsightPagination,
  formatTime,
  insightErrorMessage,
  readEmployeeName,
  rememberEmployee,
} from './ai-insight-workspace';
import type {
  AiInsightWorkspaceApi,
  AiInsightExportDownloader,
  DerivedInsightFilters,
  DerivedInsightView,
  EmotionLabel,
  InsightDetail,
  InsightDirectoryCoverage,
  InsightPage,
  InsightRunStatus,
  SessionInsightRow,
} from './ai-insight-workspace-api';
import { readDerivedFilters, writeDerivedFilters } from './ai-insight-url-state';

type DerivedPageProps = { api: AiInsightWorkspaceApi; downloadExport?: AiInsightExportDownloader | undefined; onNavigate?: ((path: string) => void) | undefined };
type ViewConfig = { view: DerivedInsightView; title: string; description: string; specializedFilter: (draft: DerivedInsightFilters, update: (next: Partial<DerivedInsightFilters>) => void) => ReactNode };

const emotionLabels: Record<EmotionLabel, string> = { positive: '正向', neutral: '中性', negative: '负向', mixed: '混合', unknown: '未知' };
const emptyPage: InsightPage<SessionInsightRow> = { page: 1, pageSize: 20, total: 0, items: [] };

function asRecord(value: unknown): Record<string, unknown> { return value && typeof value === 'object' && !Array.isArray(value) ? value as Record<string, unknown> : {}; }
function asText(value: unknown): string { return typeof value === 'string' || typeof value === 'number' ? String(value) : ''; }
function asStrings(value: unknown): string[] { return Array.isArray(value) ? value.filter((entry): entry is string => typeof entry === 'string') : []; }
function customer(row: SessionInsightRow): Record<string, unknown> { return asRecord(asRecord(row.result).customer); }
function employeeQa(row: SessionInsightRow): Record<string, unknown> { return asRecord(asRecord(row.result).employeeQa); }

function ViewStatus({ status }: { status?: InsightRunStatus | undefined }) {
  if (!status) return null;
  const messages: string[] = [];
  if (status.provider.state !== 'ready') messages.push('AI 服务暂不可用，请检查 AI 设置。');
  if (status.run?.status === 'failed') messages.push(`最近一次分析失败${status.run.errorSummary ? `：${status.run.errorSummary}` : ''}`);
  if (status.run && status.run.backlogCount > 0) messages.push(`还有 ${status.run.backlogCount} 个会话等待分析`);
  return messages.length ? <div className="ai-insight-status" role="status">{messages.join('；')}</div> : null;
}

function SummaryCards({ page, view }: { page: InsightPage<SessionInsightRow>; view: DerivedInsightView }) {
  const succeeded = page.items.filter((item) => item.status === 'succeeded');
  const failed = page.items.filter((item) => item.status === 'failed').length;
  let businessValue = '—';
  let businessLabel = '当前页指标';
  if (view === 'emotion') {
    businessLabel = '当前页正向';
    businessValue = String(succeeded.filter((item) => asText(asRecord(customer(item).emotion).label) === 'positive').length);
  } else if (view === 'employee-score') {
    businessLabel = '当前页平均分';
    const scores = succeeded.map((item) => employeeQa(item).score).filter((score): score is number => typeof score === 'number' && Number.isFinite(score));
    businessValue = scores.length ? `平均 ${Math.round(scores.reduce((sum, score) => sum + score, 0) / scores.length)} 分` : '—';
  } else {
    businessLabel = '当前页关键词';
    businessValue = String(succeeded.reduce((total, item) => total + asStrings(customer(item).keywords).length, 0));
  }
  return <div className="ai-insight-derived-summary" aria-label="当前页摘要"><div><span>全部记录</span><strong>{page.total}</strong></div><div><span>当前页完成 / 失败</span><strong>{succeeded.length} / {failed}</strong></div><div><span>{businessLabel}</span><strong>{businessValue}</strong></div></div>;
}

function DirectoryCoverage({ coverage }: { coverage?: InsightDirectoryCoverage | undefined }) {
  if (!coverage) return null;
  return <div className="ai-insight-directory-coverage" aria-label="AI 洞察目录覆盖"><span>可用员工 {coverage.availableEmployeeCount} · 已分析 {coverage.analyzedEmployeeCount}</span><span>可用客户 {coverage.availableCustomerCount} · 已分析 {coverage.analyzedCustomerCount}</span><small>只有具备已归档消息并成功落库的会话才会形成洞察结果</small></div>;
}

function ResultCell({ row, view }: { row: SessionInsightRow; view: DerivedInsightView }) {
  if (row.status === 'failed') return <span className="ai-insight-derived-failure">分析失败：{row.errorSummary || '未记录失败原因'}</span>;
  if (row.status !== 'succeeded') return <span className="ai-insight-pill ai-insight-pill-warning">{row.status === 'running' ? '分析中' : '等待分析'}</span>;
  if (view === 'emotion') {
    const emotion = asRecord(customer(row).emotion);
    const label = asText(emotion.label) as EmotionLabel;
    return <div className="ai-insight-derived-result"><span className={`ai-insight-pill ai-insight-emotion-${label}`}>{emotionLabels[label]}</span><small>{asText(emotion.reason) || '未记录情绪依据'}</small></div>;
  }
  if (view === 'employee-score') {
    const qa = employeeQa(row);
    return <div className="ai-insight-derived-result"><strong>{String(qa.score)} 分</strong><small>{asStrings(qa.strengths).join('；') || row.summary || '未记录评分说明'}</small></div>;
  }
  const keywords = asStrings(customer(row).keywords);
  return <div className="ai-insight-keyword-tags">{keywords.length ? keywords.map((keyword, index) => <span key={`${keyword}-${index}`}>{keyword}</span>) : <small>未提取关键词</small>}</div>;
}

function DerivedTable({ page, view, onOpen }: { page: InsightPage<SessionInsightRow>; view: DerivedInsightView; onOpen: (id: number) => void }) {
  const resultHeading = view === 'emotion' ? '客户情绪' : view === 'employee-score' ? '员工评分' : '沟通关键词';
  return <div className="ai-insight-table-wrap"><table className="ai-insight-table ai-insight-derived-table"><thead><tr><th>沟通员工</th><th>客户 / 群聊</th><th>{resultHeading}</th><th>结果摘要</th><th>来源窗口</th><th>分析时间</th><th>操作</th></tr></thead><tbody>{page.items.map((row) => {
    rememberEmployee(row.employee);
    return <tr key={row.id} onClick={() => onOpen(row.id)}><td data-label="沟通员工"><span className="ai-insight-person"><Avatar {...row.employee} /><span>{row.employee.name}</span></span></td><td data-label="客户 / 群聊"><span className="ai-insight-person"><Avatar {...row.target} /><span>{row.target.name}<small className="ai-insight-subline">{row.target.type === 'group' ? '群聊' : '客户'}</small></span></span></td><td data-label={resultHeading}><ResultCell row={row} view={view} /></td><td data-label="结果摘要"><span className="ai-insight-summary ai-insight-summary--multiline">{row.summary || '暂无摘要'}</span></td><td data-label="来源窗口">{row.sourceWindow.messageCount} 条消息<small className="ai-insight-subline">{formatTime(row.sourceWindow.startedAt)} — {formatTime(row.sourceWindow.endedAt)}</small></td><td data-label="分析时间">{formatTime(row.analysisAt)}</td><td data-label="操作"><button className="ai-insight-secondary" type="button" onClick={(event) => { event.stopPropagation(); onOpen(row.id); }}>查看详情</button></td></tr>;
  })}</tbody></table></div>;
}

function scoreInput(value: number | undefined): string { return value === undefined ? '' : String(value); }
function parseScore(value: string): number | undefined { if (value === '') return undefined; const parsed = Number(value); return Number.isInteger(parsed) && parsed >= 0 && parsed <= 100 ? parsed : undefined; }

const configs: Record<DerivedInsightView, ViewConfig> = {
  emotion: {
    view: 'emotion', title: '客户情绪洞察', description: '从已持久化会话分析中查看客户情绪、依据与消息证据',
    specializedFilter: (draft, update) => <AiInsightField label="客户情绪"><select aria-label="客户情绪" value={draft.emotion ?? ''} onChange={(event) => update({ emotion: (event.target.value || undefined) as EmotionLabel | undefined })}><option value="">全部情绪</option><option value="positive">正向客户</option><option value="neutral">中性客户</option><option value="negative">负向客户</option><option value="mixed">混合情绪</option><option value="unknown">未知情绪</option></select></AiInsightField>,
  },
  'employee-score': {
    view: 'employee-score', title: '员工评分洞察', description: '从已持久化会话质检结果中查看评分、说明与消息证据',
    specializedFilter: (draft, update) => <><AiInsightField label="最低分"><input aria-label="最低分" type="number" min="0" max="100" step="1" value={scoreInput(draft.minScore)} onChange={(event) => update({ minScore: parseScore(event.target.value) })} /></AiInsightField><AiInsightField label="最高分"><input aria-label="最高分" type="number" min="0" max="100" step="1" value={scoreInput(draft.maxScore)} onChange={(event) => update({ maxScore: parseScore(event.target.value) })} /></AiInsightField></>,
  },
  'communication-keyword': {
    view: 'communication-keyword', title: '沟通关键词洞察', description: '从已持久化会话分析中查看关键词、上下文与消息证据',
    specializedFilter: (draft, update) => <AiInsightField label="沟通关键词"><input aria-label="沟通关键词" value={draft.keyword ?? ''} onChange={(event) => update({ keyword: event.target.value || undefined })} placeholder="搜索已提取关键词" /></AiInsightField>,
  },
};

function DerivedInsightWorkspace({ api, downloadExport, onNavigate, config }: DerivedPageProps & { config: ViewConfig }) {
  const initial = readDerivedFilters(config.view);
  const [draft, setDraft] = useState<DerivedInsightFilters>(initial);
  const [applied, setApplied] = useState<DerivedInsightFilters>(initial);
  const [employeeName, setEmployeeName] = useState(() => readEmployeeName(initial.employeeId));
  const [customerName, setCustomerName] = useState('');
  const [coverage, setCoverage] = useState<InsightDirectoryCoverage>();
  const [page, setPage] = useState<InsightPage<SessionInsightRow>>(emptyPage);
  const [status, setStatus] = useState<InsightRunStatus>();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [detail, setDetail] = useState<InsightDetail<SessionInsightRow>>();
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailError, setDetailError] = useState('');
  const [exporting, setExporting] = useState(false);
  const [exportError, setExportError] = useState('');
  const detailRequest = useRef(0);

  useEffect(() => {
    let active = true;
    setLoading(true);
    setError('');
    void Promise.all([
      api.derivedRecords(config.view, applied),
      api.derivedStatus(config.view).catch((reason: unknown): InsightRunStatus => ({ provider: { state: 'unavailable', message: insightErrorMessage(reason, '状态读取失败') } })),
      api.derivedFilterOptions(config.view, undefined, 100),
    ])
      .then(([records, runStatus, options]) => { if (!active) return; setPage(records); setStatus(runStatus); setCoverage(options.coverage); const selectedEmployee = options.employees.find((item) => item.id === applied.employeeId); if (selectedEmployee) setEmployeeName(selectedEmployee.name); const selectedCustomer = options.customers.find((item) => item.id === applied.customerId); setCustomerName(selectedCustomer?.name ?? ''); })
      .catch((reason: unknown) => { if (active) setError(insightErrorMessage(reason, '洞察数据加载失败')); })
      .finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [api, applied, config.view]);

  useEffect(() => {
    const restore = () => { const next = readDerivedFilters(config.view); setDraft(next); setApplied(next); setEmployeeName(readEmployeeName(next.employeeId)); setCustomerName(''); };
    window.addEventListener('popstate', restore);
    return () => window.removeEventListener('popstate', restore);
  }, [config.view]);

  const apply = (next: DerivedInsightFilters, mode: 'push' | 'replace') => {
    if (config.view === 'employee-score' && next.minScore !== undefined && next.maxScore !== undefined && next.minScore > next.maxScore) { setError('最低分不能高于最高分'); return; }
    const normalized = { ...next, page: next.page || 1 };
    setDraft(normalized);
    setApplied(normalized);
    setEmployeeName(readEmployeeName(normalized.employeeId));
    if (normalized.customerId === undefined) setCustomerName('');
    window.history[mode === 'push' ? 'pushState' : 'replaceState']({}, '', writeDerivedFilters(config.view, normalized));
  };
  const updateDraft = (next: Partial<DerivedInsightFilters>) => setDraft((current) => ({ ...current, ...next }));
  const open = (id: number) => {
    const request = detailRequest.current + 1;
    detailRequest.current = request;
    setDetail(undefined);
    setDetailError('');
    setDetailLoading(true);
    void api.derivedDetail(config.view, id)
      .then((next) => { if (detailRequest.current === request) setDetail(next); })
      .catch((reason: unknown) => { if (detailRequest.current === request) setDetailError(insightErrorMessage(reason, '详情加载失败')); })
      .finally(() => { if (detailRequest.current === request) setDetailLoading(false); });
  };
  const download = async () => {
    setExportError('');
    setExporting(true);
    try {
      if (!downloadExport) throw new Error('当前客户端未接入认证导出能力');
      const result = await downloadExport(config.view, applied);
      const url = URL.createObjectURL(result.blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = result.filename || `${config.view}-insights.csv`;
      link.click();
      URL.revokeObjectURL(url);
    } catch (reason) {
      setExportError(insightErrorMessage(reason, '导出 CSV 失败'));
    } finally {
      setExporting(false);
    }
  };
  const totalPages = Math.max(1, Math.ceil(page.total / 20));
  const loadEmployees = useCallback((keyword?: string, limit?: number) => api.derivedFilterOptions(config.view, keyword, limit), [api, config.view]);
  const loadCustomers = useCallback((keyword?: string, limit?: number) => api.derivedFilterOptions(config.view, undefined, limit, keyword), [api, config.view]);

  return <div className="ai-insight-workspace ai-insight-derived-workspace">
    <AiInsightHeader title={config.title} description={config.description} actions={<button className="ai-insight-secondary ai-insight-export-link" type="button" disabled={exporting} onClick={() => void download()}>{exporting ? '正在导出…' : '导出 CSV'}</button>} />
    <AiInsightQueryBar onSubmit={() => apply({ ...draft, page: 1 }, 'push')} onReset={() => apply({ page: 1 }, 'push')} onRefresh={() => apply(applied, 'replace')}>
      <EmployeeSearchField label="员工" selectedEmployeeId={draft.employeeId} knownEmployeeName={employeeName} loadOptions={loadEmployees} onSelect={(employee) => { setEmployeeName(employee?.name ?? ''); setDraft((current) => ({ ...current, employeeId: employee?.id })); }} />
      <CustomerSearchField selectedCustomerId={draft.customerId} knownCustomerName={customerName} loadOptions={loadCustomers} onSelect={(customer) => { setCustomerName(customer?.name ?? ''); setDraft((current) => ({ ...current, customerId: customer?.id, customerName: undefined })); }} />
      <AiInsightField label="分析状态"><select aria-label="分析状态" value={draft.status ?? ''} onChange={(event) => updateDraft({ status: (event.target.value || undefined) as DerivedInsightFilters['status'] })}><option value="">全部状态</option><option value="pending">待处理</option><option value="running">分析中</option><option value="succeeded">已完成</option><option value="failed">失败</option></select></AiInsightField>
      <AiInsightField label="开始日期"><input aria-label="开始日期" type="date" value={draft.startDate ?? ''} onChange={(event) => updateDraft({ startDate: event.target.value || undefined })} /></AiInsightField>
      <AiInsightField label="结束日期"><input aria-label="结束日期" type="date" value={draft.endDate ?? ''} onChange={(event) => updateDraft({ endDate: event.target.value || undefined })} /></AiInsightField>
      {config.specializedFilter(draft, updateDraft)}
    </AiInsightQueryBar>
    <DirectoryCoverage coverage={coverage} />
    <ViewStatus status={status} />
    {exportError && <div className="ai-insight-error ai-insight-inline-error" role="alert">{exportError}</div>}
    {detailError && <div className="ai-insight-error ai-insight-inline-error" role="alert">{detailError}</div>}
    {!loading && !error && <SummaryCards page={page} view={config.view} />}
    <section className="ai-insight-results"><header className="ai-insight-results-header"><div><h2>洞察结果</h2><p>只展示已持久化的会话分析结果与来源证据</p></div><div className="ai-insight-derived-page-summary"><span>共 {page.total} 条，当前页 {page.page} / {totalPages}</span><span>每页 20 条</span></div></header>{loading ? <div className="ai-insight-loading">正在加载洞察数据…</div> : error ? <div className="ai-insight-error" role="alert"><span>{error}</span><button type="button" className="ai-insight-secondary" onClick={() => apply(applied, 'replace')}>重试</button></div> : page.items.length === 0 ? <div className="ai-insight-empty">当前目录实体没有符合时间窗的已持久化分析结果</div> : <DerivedTable page={page} view={config.view} onOpen={open} />}<InsightPagination page={page.page} total={page.total} onChange={(next) => apply({ ...applied, page: next }, 'push')} /></section>
    {detailLoading ? <div className="ai-insight-status">正在打开分析详情…</div> : detail ? <InsightDrawer detail={detail} onClose={() => setDetail(undefined)} onNavigate={onNavigate} /> : null}
  </div>;
}

export function EmotionInsightPage(props: DerivedPageProps) { return <DerivedInsightWorkspace {...props} config={configs.emotion} />; }
export function EmployeeScoreInsightPage(props: DerivedPageProps) { return <DerivedInsightWorkspace {...props} config={configs['employee-score']} />; }
export function CommunicationKeywordInsightPage(props: DerivedPageProps) { return <DerivedInsightWorkspace {...props} config={configs['communication-keyword']} />; }
