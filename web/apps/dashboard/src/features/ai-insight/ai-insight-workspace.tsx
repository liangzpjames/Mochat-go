import { useEffect, useRef, useState, type KeyboardEvent as ReactKeyboardEvent, type ReactNode } from 'react';
import { DashboardPagination } from '../../components/dashboard-pagination';
import type {
  AiInsightWorkspaceApi,
  EmployeeFilterOptions,
  InsightDetail,
  InsightPage,
  InsightRunStatus,
  Person,
  SessionInsightFilters,
  SessionInsightRow,
  SmartInsightFilters,
  SmartInsightRow,
} from './ai-insight-workspace-api';
import { readSessionFilters, readSmartState, writeSessionFilters, writeSmartState } from './ai-insight-url-state';

const EMPLOYEE_CACHE_KEY = 'ai-insight.employee-name';

function asRecord(value: unknown): Record<string, unknown> {
  return value && typeof value === 'object' && !Array.isArray(value) ? value as Record<string, unknown> : {};
}

function asArray(value: unknown): unknown[] {
  return Array.isArray(value) ? value : [];
}

function asText(value: unknown): string {
  return typeof value === 'string' || typeof value === 'number' ? String(value) : '';
}

function asNumber(value: unknown): number | null {
  if (typeof value === 'number' && Number.isFinite(value)) return value;
  return null;
}

function scoreText(value: unknown): string {
  const score = asNumber(value);
  return score === null ? '证据不足' : `${score}分`;
}

function percentText(value: unknown): string {
  const number = asNumber(value);
  return number === null ? '证据不足' : `${Math.round(number * 100)}%`;
}

function nonEmptyList(value: unknown): string[] {
  return asArray(value).map(asText).filter((item) => item.trim() !== '');
}

function listOrFallback(value: unknown): string[] {
  const items = nonEmptyList(value);
  return items.length > 0 ? items : ['证据不足'];
}

function metricValue(primary: unknown, secondary?: unknown): string {
  const score = asNumber(primary);
  if (score !== null) return `${score}分${secondary ? ` · ${asText(secondary)}` : ''}`;
  const secondaryText = asText(secondary).trim();
  return secondaryText || '证据不足';
}

type Dimension = { name: string; score: unknown; weight: string; reason: string };
type IssueItem = { title: string; reason: string };

function parseDimensions(value: unknown): Dimension[] {
  return asArray(value).map((entry) => {
    const item = asRecord(entry);
    const weight = asNumber(item.weight);
    return {
      name: asText(item.name) || '未命名维度',
      score: item.score,
      weight: weight === null ? '—' : `${Math.round(weight * 100)}%`,
      reason: asText(item.reason) || asText(item.comment) || '证据不足',
    };
  });
}

function parseIssueItems(value: unknown): IssueItem[] {
  return asArray(value).map((entry) => {
    const item = asRecord(entry);
    return {
      title: asText(item.title) || '未命名问题',
      reason: asText(item.reason) || '证据不足',
    };
  });
}

function metricCard(label: string, value: string, secondary?: string) {
  return (
    <div className="ai-insight-metric-card" key={label}>
      <span>{label}</span>
      <strong>{value}</strong>
      {secondary && <small>{secondary}</small>}
    </div>
  );
}

function FailureResult({ reason }: { reason: string }) {
  return (
    <>
      <span className="ai-insight-pill ai-insight-pill-danger">分析失败</span>
      <span className="ai-insight-summary">{reason || '未记录失败原因'}</span>
    </>
  );
}

function readEmployeeCache(): Record<string, string> {
  try {
    return JSON.parse(window.sessionStorage.getItem(EMPLOYEE_CACHE_KEY) ?? '{}') as Record<string, string>;
  } catch {
    return {};
  }
}

function writeEmployeeCache(cache: Record<string, string>) {
  try {
    window.sessionStorage.setItem(EMPLOYEE_CACHE_KEY, JSON.stringify(cache));
  } catch {
    // ignore storage failures
  }
}

export function rememberEmployee(employee: Person) {
  if (employee.id <= 0 || employee.name.trim() === '') return;
  const cache = readEmployeeCache();
  cache[String(employee.id)] = employee.name;
  writeEmployeeCache(cache);
}

export function readEmployeeName(employeeId?: number): string {
  if (!employeeId) return '';
  const cache = readEmployeeCache();
  return cache[String(employeeId)] ?? '';
}

export function AiInsightHeader({ title, description, actions }: { title: string; description: string; actions?: ReactNode }) {
  return <header className="ai-insight-header"><div><h1>{title}</h1><p>{description}</p></div><div className="ai-insight-actions">{actions}</div></header>;
}

export function AiInsightQueryBar({ children, onSubmit, onReset, onRefresh, extraActions }: { children: ReactNode; onSubmit: () => void; onReset: () => void; onRefresh: () => void; extraActions?: ReactNode }) {
  return <form className="ai-insight-query" onSubmit={(event) => { event.preventDefault(); onSubmit(); }}><div className="ai-insight-fields">{children}</div><div className="ai-insight-query-actions">{extraActions}<button type="button" className="ai-insight-secondary" onClick={onReset}>重置</button><button type="button" className="ai-insight-secondary" onClick={onRefresh}>刷新</button><button type="submit" className="ai-insight-primary">查询</button></div></form>;
}

export function AiInsightField({ label, children }: { label: string; children: ReactNode }) {
  return <label><span>{label}</span>{children}</label>;
}

export function AiInsightStatusStrip({ status }: { status?: InsightRunStatus | undefined }) {
  const run = status?.run;
  const providerUnavailable = !!status && status.provider.state !== 'ready';
  if ((!run && !providerUnavailable) || (run && run.status !== 'failed' && run.backlogCount <= 0 && !providerUnavailable)) return null;
  const messages: string[] = [];
  if (run?.status === 'failed') messages.push(`最近一次分析失败${run.errorSummary ? `：${run.errorSummary}` : ''}`);
  if (run && run.backlogCount > 0) messages.push(`还有 ${run.backlogCount} 个会话等待分析`);
  if (providerUnavailable) messages.push(`AI 服务暂不可用${status.provider.message ? `：${status.provider.message}` : '，已保留历史结果'}`);
  return <div className="ai-insight-status" role="status">{messages.join('；')}</div>;
}

export function Avatar({ name, avatar }: { name: string; avatar: string }) {
  const fallback = name.trim().slice(0, 1) || '客';
  return avatar ? <img className="ai-insight-avatar" src={avatar} alt="" /> : <span className="ai-insight-avatar ai-insight-avatar-fallback" aria-label={name}>{fallback}</span>;
}

export function InsightPagination({ page, total, onChange }: { page: number; total: number; onChange: (page: number) => void }) {
  return <DashboardPagination page={page} pageSize={20} total={total} onPageChange={onChange} ariaLabel="AI 洞察分页" />;
}

export function EmployeeSearchField({
  label,
  selectedEmployeeId,
  knownEmployeeName,
  loadOptions,
  onSelect,
}: {
  label: string;
  selectedEmployeeId: number | undefined;
  knownEmployeeName: string | undefined;
  loadOptions: (keyword?: string, limit?: number) => Promise<EmployeeFilterOptions>;
  onSelect: (employee: Person | undefined) => void;
}) {
  const [open, setOpen] = useState(false);
  const [inputValue, setInputValue] = useState(knownEmployeeName ?? readEmployeeName(selectedEmployeeId));
  const [options, setOptions] = useState<Person[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [activeIndex, setActiveIndex] = useState(-1);
  const requestRef = useRef(0);
  const listboxId = 'ai-insight-employee-options';
  const activeOptionId = open && activeIndex >= 0 && options[activeIndex] ? `ai-insight-employee-option-${options[activeIndex].id}` : undefined;

  useEffect(() => {
    setInputValue(knownEmployeeName ?? readEmployeeName(selectedEmployeeId));
  }, [knownEmployeeName, selectedEmployeeId]);

  useEffect(() => {
    if (!open) return undefined;
    const currentRequest = ++requestRef.current;
    setLoading(true);
    setError('');
    const timer = window.setTimeout(() => {
      void loadOptions(inputValue.trim() || undefined, 20)
        .then((result) => {
          if (currentRequest !== requestRef.current) return;
          setOptions(result.employees);
          setActiveIndex(result.employees.length > 0 ? 0 : -1);
        })
        .catch(() => {
          if (currentRequest !== requestRef.current) return;
          setOptions([]);
          setActiveIndex(-1);
          setError('员工列表加载失败');
        })
        .finally(() => {
          if (currentRequest === requestRef.current) setLoading(false);
        });
    }, 150);
    return () => window.clearTimeout(timer);
  }, [inputValue, loadOptions, open]);

  function commit(employee: Person) {
    rememberEmployee(employee);
    setInputValue(employee.name);
    setOpen(false);
    setOptions([]);
    setActiveIndex(-1);
    setError('');
    onSelect(employee);
  }

  function clear() {
    setInputValue('');
    setOpen(false);
    setOptions([]);
    setActiveIndex(-1);
    setError('');
    onSelect(undefined);
  }

  function handleKeyDown(event: ReactKeyboardEvent<HTMLInputElement>) {
    if (!open && (event.key === 'ArrowDown' || event.key === 'Enter')) {
      setOpen(true);
      return;
    }
    if (event.key === 'Escape') {
      setOpen(false);
      return;
    }
    if (event.key === 'ArrowDown' && options.length > 0) {
      event.preventDefault();
      setActiveIndex((current) => Math.min(current + 1, options.length - 1));
    }
    if (event.key === 'ArrowUp' && options.length > 0) {
      event.preventDefault();
      setActiveIndex((current) => Math.max(current - 1, 0));
    }
    if (event.key === 'Enter' && open && activeIndex >= 0 && options[activeIndex]) {
      event.preventDefault();
      commit(options[activeIndex]);
    }
  }

  return (
    <AiInsightField label={label}>
      <div className="ai-insight-combobox">
        <input
          aria-activedescendant={activeOptionId}
          aria-controls={listboxId}
          aria-expanded={open}
          aria-label={label}
          aria-autocomplete="list"
          role="combobox"
          value={inputValue}
          placeholder="搜索员工姓名"
          onFocus={() => setOpen(true)}
          onChange={(event) => {
            setInputValue(event.target.value);
            setOpen(true);
            setError('');
            if (selectedEmployeeId !== undefined) onSelect(undefined);
          }}
          onKeyDown={handleKeyDown}
        />
        {(selectedEmployeeId !== undefined || inputValue) && <button type="button" className="ai-insight-combobox-clear" aria-label="清除员工" onClick={clear}>清除</button>}
        {open && (
          <div className="ai-insight-combobox-panel">
            {loading && <div role="status" className="ai-insight-combobox-state">正在加载员工…</div>}
            {!loading && error && <div role="alert" className="ai-insight-combobox-state">{error}</div>}
            {!loading && !error && options.length === 0 && <div role="status" className="ai-insight-combobox-state">没有匹配员工</div>}
            {!loading && !error && options.length > 0 && (
              <ul id={listboxId} role="listbox">
                {options.map((employee, index) => (
                  <li
                    key={employee.id}
                    id={`ai-insight-employee-option-${employee.id}`}
                    aria-selected={index === activeIndex}
                    role="option"
                    tabIndex={-1}
                    onMouseDown={(event) => {
                      event.preventDefault();
                      commit(employee);
                    }}
                  >
                    <Avatar {...employee} />
                    <span>{employee.name}</span>
                  </li>
                ))}
              </ul>
            )}
          </div>
        )}
      </div>
    </AiInsightField>
  );
}

function renderSessionMetrics(result: Record<string, unknown>) {
  const customer = asRecord(result.customer);
  const purchase = asRecord(customer.purchaseIntent);
  const churn = asRecord(customer.churnRisk);
  const employeeQa = asRecord(result.employeeQa);
  return (
    <div className="ai-insight-row-metrics">
      {metricCard('购买意向分', scoreText(purchase.score))}
      {metricCard('流失风险分', scoreText(churn.score))}
      {metricCard('员工质检分', scoreText(employeeQa.score))}
    </div>
  );
}

function isSmartV2(result: Record<string, unknown>): boolean {
  return asNumber(result.schemaVersion) === 2;
}

function renderSmartMetrics(result: Record<string, unknown>) {
  if (!isSmartV2(result)) {
    const matched = result.matched === true ? '已命中' : result.matched === false ? '未命中' : '证据不足';
    return (
      <div className="ai-insight-row-metrics">
        {metricCard('历史命中', matched)}
        {metricCard('置信度', percentText(result.confidence))}
        {metricCard('说明', '历史结果无综合分')}
      </div>
    );
  }
  return (
    <div className="ai-insight-row-metrics">
      {metricCard('命中度', scoreText(result.matchScore))}
      {metricCard('置信度', scoreText(result.confidenceScore))}
      {metricCard('优先级', metricValue(result.priorityScore, result.priorityLevel), asNumber(result.evidenceCoverageScore) === null ? undefined : `覆盖度 ${scoreText(result.evidenceCoverageScore)}`)}
    </div>
  );
}

export function SessionTable({ page, onOpen }: { page: InsightPage<SessionInsightRow>; onOpen: (id: number) => void }) {
  return (
    <div className="ai-insight-table-wrap">
      <table className="ai-insight-table">
        <thead><tr><th>沟通人</th><th>客户 / 群聊</th><th>量化结果</th><th>结果摘要</th><th>来源</th><th>分析时间</th><th>操作</th></tr></thead>
        <tbody>{page.items.map((item) => {
          rememberEmployee(item.employee);
          const result = asRecord(item.result);
          return (
            <tr key={item.id} onClick={() => onOpen(item.id)}>
              <td><span className="ai-insight-person"><Avatar {...item.employee} /><span>{item.employee.name}</span></span></td>
              <td><span className="ai-insight-person"><Avatar {...item.target} /><span>{item.target.name}<small className="ai-insight-subline">{item.target.type === 'group' ? '群聊' : '客户'}</small></span></span></td>
              <td>{item.status === 'failed' ? <FailureResult reason={item.errorSummary} /> : renderSessionMetrics(result)}</td>
              <td><span className="ai-insight-summary ai-insight-summary--multiline">{item.summary || '暂无摘要'}</span></td>
              <td>{item.sourceWindow.messageCount} 条消息</td>
              <td>{formatTime(item.analysisAt)}</td>
              <td><button className="ai-insight-secondary" type="button" onClick={(event) => { event.stopPropagation(); onOpen(item.id); }}>查看详情</button></td>
            </tr>
          );
        })}</tbody>
      </table>
    </div>
  );
}

export function SmartTable({ page, onOpen }: { page: InsightPage<SmartInsightRow>; onOpen: (id: number) => void }) {
  return (
    <div className="ai-insight-table-wrap">
      <table className="ai-insight-table">
        <thead><tr><th>沟通人</th><th>客户 / 群聊</th><th>关联规则</th><th>量化结果</th><th>结果摘要</th><th>分析时间</th><th>操作</th></tr></thead>
        <tbody>{page.items.map((item) => {
          rememberEmployee(item.employee);
          const result = asRecord(item.result);
          return (
            <tr key={item.id} onClick={() => onOpen(item.id)}>
              <td><span className="ai-insight-person"><Avatar {...item.employee} /><span>{item.employee.name}</span></span></td>
              <td>{item.target.name}<small className="ai-insight-subline">{item.target.type === 'group' ? '群聊' : '客户'}</small></td>
              <td>{item.rule?.name}<small className="ai-insight-subline">v{item.rule?.version}</small></td>
              <td>{item.status === 'failed' ? <FailureResult reason={item.errorSummary} /> : renderSmartMetrics(result)}</td>
              <td><span className="ai-insight-summary ai-insight-summary--multiline">{asText(result.summary) || asText(result.conclusion) || item.summary || '暂无结论'}</span></td>
              <td>{formatTime(item.analysisAt)}</td>
              <td><button className="ai-insight-secondary" type="button" onClick={(event) => { event.stopPropagation(); onOpen(item.id); }}>查看详情</button></td>
            </tr>
          );
        })}</tbody>
      </table>
    </div>
  );
}

function DetailField({ label, value }: { label: string; value: string }) {
  return <div className="ai-insight-detail-pair"><span>{label}</span><strong>{value}</strong></div>;
}

function DetailList({ title, items }: { title: string; items: string[] }) {
  return (
    <div className="ai-insight-detail-list">
      <span>{title}</span>
      <ul>{items.map((item) => <li key={`${title}-${item}`}>{item}</li>)}</ul>
    </div>
  );
}

function IssueList({ title, items }: { title: string; items: IssueItem[] }) {
  if (items.length === 0) return <DetailList title={title} items={['证据不足']} />;
  return (
    <div className="ai-insight-detail-list">
      <span>{title}</span>
      <ul>
        {items.map((item) => (
          <li key={`${title}-${item.title}-${item.reason}`}>
            <strong>{item.title}</strong>
            <small>{item.reason}</small>
          </li>
        ))}
      </ul>
    </div>
  );
}

function DimensionTable({ dimensions }: { dimensions: Dimension[] }) {
  if (dimensions.length === 0) return <p className="ai-insight-detail-muted">证据不足</p>;
  return (
    <div className="ai-insight-dimension-table">
      {dimensions.map((dimension) => (
        <div key={`${dimension.name}-${dimension.reason}`} className="ai-insight-dimension-row">
          <strong>{dimension.name}</strong>
          <span>{scoreText(dimension.score)}</span>
          <span>权重 {dimension.weight}</span>
          <small>{dimension.reason}</small>
        </div>
      ))}
    </div>
  );
}

function SessionDetailSections({ result }: { result: Record<string, unknown> }) {
  const customer = asRecord(result.customer);
  const purchase = asRecord(customer.purchaseIntent);
  const churn = asRecord(customer.churnRisk);
  const emotion = asRecord(customer.emotion);
  const employeeQa = asRecord(result.employeeQa);
  return (
    <>
      <section className="ai-insight-detail-card">
        <h3>客户分析</h3>
        <div className="ai-insight-detail-grid">
          <DetailField label="质量分 / 等级" value={metricValue(customer.qualityScore, customer.qualityLevel)} />
          <DetailField label="购买意向" value={metricValue(purchase.score, purchase.level)} />
          <DetailField label="流失风险" value={metricValue(churn.score, churn.level)} />
          <DetailField label="客户情绪" value={asText(emotion.label) || '证据不足'} />
        </div>
        <DetailField label="质量依据" value={asText(customer.qualityReason) || '证据不足'} />
        <DetailField label="购买意向依据" value={asText(purchase.reason) || '证据不足'} />
        <DetailField label="流失风险依据" value={asText(churn.reason) || '证据不足'} />
        <DetailField label="情绪依据" value={asText(emotion.reason) || '证据不足'} />
        <DetailList title="关键词" items={listOrFallback(customer.keywords)} />
        <DetailList title="明确需求" items={listOrFallback(customer.explicitNeeds)} />
        <DetailList title="潜在需求" items={listOrFallback(customer.implicitNeeds)} />
        <div className="ai-insight-detail-list"><span>购买意向维度</span><DimensionTable dimensions={parseDimensions(purchase.dimensions)} /></div>
        <div className="ai-insight-detail-list"><span>流失风险维度</span><DimensionTable dimensions={parseDimensions(churn.dimensions)} /></div>
        <DetailList title="推荐回复" items={listOrFallback(customer.recommendedReply ? [customer.recommendedReply] : [])} />
        <DetailList title="行动建议" items={listOrFallback(customer.actions)} />
        <DetailList title="补充备注" items={listOrFallback(customer.notes)} />
      </section>
      <section className="ai-insight-detail-card">
        <h3>员工质检</h3>
        <div className="ai-insight-detail-grid">
          <DetailField label="总分" value={scoreText(employeeQa.score)} />
        </div>
        <div className="ai-insight-detail-list"><span>质检维度</span><DimensionTable dimensions={parseDimensions(employeeQa.dimensions)} /></div>
        <IssueList title="未解决客户问题" items={parseIssueItems(employeeQa.unresolvedCustomerIssues)} />
        <IssueList title="未解决异议" items={parseIssueItems(employeeQa.unresolvedObjections)} />
        <DetailList title="优点" items={listOrFallback(employeeQa.strengths)} />
        <DetailList title="问题" items={listOrFallback(employeeQa.issues)} />
        <DetailList title="建议" items={listOrFallback(employeeQa.suggestions)} />
      </section>
    </>
  );
}

function SmartDetailSections({ result }: { result: Record<string, unknown> }) {
  if (!isSmartV2(result)) {
    return (
      <section className="ai-insight-detail-card">
        <h3>历史结果</h3>
        <div className="ai-insight-detail-grid">
          <DetailField label="命中状态" value={result.matched === true ? '已命中' : result.matched === false ? '未命中' : '证据不足'} />
          <DetailField label="置信度" value={percentText(result.confidence)} />
          <DetailField label="说明" value="历史结果无综合分" />
        </div>
        <DetailList title="历史结论" items={listOrFallback(result.conclusion ? [asText(result.conclusion)] : [])} />
      </section>
    );
  }
  return (
    <>
      <section className="ai-insight-detail-card">
        <h3>量化指标</h3>
        <div className="ai-insight-detail-grid">
          <DetailField label="命中度" value={scoreText(result.matchScore)} />
          <DetailField label="置信度" value={scoreText(result.confidenceScore)} />
          <DetailField label="优先级" value={metricValue(result.priorityScore, result.priorityLevel)} />
          <DetailField label="覆盖度" value={scoreText(result.evidenceCoverageScore)} />
        </div>
        <DetailList title="分析结论" items={listOrFallback(result.conclusion ? [asText(result.conclusion)] : [])} />
      </section>
      <section className="ai-insight-detail-card">
        <h3>维度分析</h3>
        <DimensionTable dimensions={parseDimensions(result.dimensions)} />
      </section>
      <section className="ai-insight-detail-card">
        <h3>推荐动作</h3>
        <DetailList title="推荐动作" items={listOrFallback(result.recommendations)} />
      </section>
    </>
  );
}

function extractEvidenceIds(result: Record<string, unknown>): Set<string> {
  const ids = new Set<string>();

  function visit(value: unknown) {
    if (Array.isArray(value)) {
      value.forEach(visit);
      return;
    }
    if (!value || typeof value !== 'object') return;
    const source = asRecord(value);
    for (const id of nonEmptyList(source.evidenceMessageIds)) ids.add(id);
    for (const nested of Object.values(source)) visit(nested);
  }

  visit(result);
  return ids;
}

export function InsightDrawer<T extends SessionInsightRow>({ detail, onClose, onNavigate }: { detail: InsightDetail<T>; onClose: () => void; onNavigate?: ((path: string) => void) | undefined }) {
  useEffect(() => {
    const handler = (event: KeyboardEvent) => { if (event.key === 'Escape') onClose(); };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, [onClose]);

  const result = asRecord(detail.result);
  const evidenceIds = extractEvidenceIds(result);
  const isFailed = detail.status === 'failed';
  const isSmart = 'rule' in detail;

  return (
    <div className="ai-insight-drawer-layer">
      <button className="ai-insight-drawer-overlay" aria-label="关闭详情" onClick={onClose} />
      <aside className="ai-insight-drawer" role="dialog" aria-modal="true" aria-label="分析详情">
        <header className="ai-insight-drawer-header">
          <div><h2>分析详情</h2><p>{detail.target.name} · {detail.employee.name}</p></div>
          <button className="ai-insight-close" type="button" onClick={onClose} aria-label="关闭">×</button>
        </header>
        <div className="ai-insight-drawer-body">
          {isFailed ? (
            <section className="ai-insight-detail-card"><h3>分析失败</h3><p>{detail.errorSummary || '未记录失败原因'}</p></section>
          ) : isSmart ? (
            <SmartDetailSections result={result} />
          ) : (
            <SessionDetailSections result={result} />
          )}
          <section className="ai-insight-detail-card">
            <h3>运行上下文</h3>
            <div className="ai-insight-detail-grid">
              <DetailField label="来源窗口" value={`${formatTime(detail.sourceWindow.startedAt)} — ${formatTime(detail.sourceWindow.endedAt)}`} />
              <DetailField label="消息数量" value={`${detail.sourceWindow.messageCount} 条`} />
              <DetailField label="Provider / Model" value={`${detail.provider || '未记录'} / ${detail.model || '未记录'}`} />
              <DetailField label="Prompt 版本" value={detail.promptVersion || '未记录'} />
            </div>
          </section>
          <section className="ai-insight-detail-card">
            <h3>证据消息</h3>
            <ul className="ai-insight-evidence">{detail.messages.length ? detail.messages.map((message) => (
              <li key={message.id} className={evidenceIds.has(message.id) ? 'is-highlighted' : undefined}>
                <small>{message.senderName} · {formatTime(message.time)} · {message.direction === 'outbound' ? '员工发送' : '客户发送'}</small>
                <span>{message.content || '（非文本消息）'}</span>
                {evidenceIds.has(message.id) && <em>关键证据</em>}
              </li>
            )) : <li>暂无可展示的证据消息</li>}</ul>
          </section>
          <div className="ai-insight-rule-actions">
            <button className="ai-insight-secondary" type="button" aria-label="查看原会话" onClick={() => { onNavigate?.(detail.conversationUrl); if (!onNavigate) window.location.href = detail.conversationUrl; }}>查看原会话</button>
            <button className="ai-insight-primary" type="button" onClick={onClose}>完成</button>
          </div>
        </div>
      </aside>
    </div>
  );
}

export function formatTime(value: string): string {
  if (!value) return '--';
  return value.replace('T', ' ').slice(0, 16);
}

export function useFilters<T extends SessionInsightFilters>(smart: boolean): [T, (value: T) => void] {
  const initial = smart ? readSmartState().filters : readSessionFilters();
  const [filters, setFilters] = useState<T>(initial as T);
  const set = (value: T) => {
    setFilters(value);
    const url = smart ? writeSmartState({ filters: value as SmartInsightFilters }) : writeSessionFilters(value as SessionInsightFilters);
    window.history.replaceState({}, '', url);
  };
  return [filters, set];
}

export type WorkspaceProps = { api: AiInsightWorkspaceApi; navigate?: (path: string) => void };
