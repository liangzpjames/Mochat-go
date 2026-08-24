import { useEffect, useState } from 'react';
import {
  AiInsightField,
  AiInsightHeader,
  AiInsightQueryBar,
  AiInsightStatusStrip,
  EmployeeSearchField,
  InsightDrawer,
  InsightPagination,
  SessionTable,
  readEmployeeName,
  type WorkspaceProps,
} from './ai-insight-workspace';
import type { InsightDetail, InsightPage, InsightRunStatus, SessionInsightFilters, SessionInsightRow } from './ai-insight-workspace-api';
import { readSessionFilters, writeSessionFilters } from './ai-insight-url-state';

const emptyFilters: SessionInsightFilters = { page: 1 };

export function SessionAnalysisPage({ api, navigate }: WorkspaceProps) {
  const initial = readSessionFilters();
  const [draft, setDraft] = useState<SessionInsightFilters>(initial);
  const [applied, setApplied] = useState<SessionInsightFilters>(initial);
  const [employeeName, setEmployeeName] = useState(() => readEmployeeName(initial.employeeId));
  const [page, setPage] = useState<InsightPage<SessionInsightRow>>({ page: 1, pageSize: 20, total: 0, items: [] });
  const [status, setStatus] = useState<InsightRunStatus>();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [detail, setDetail] = useState<InsightDetail<SessionInsightRow>>();
  const [detailLoading, setDetailLoading] = useState(false);

  useEffect(() => {
    let active = true;
    setLoading(true);
    setError('');
    void Promise.all([api.sessionRecords(applied), api.sessionStatus()])
      .then(([rows, run]) => {
        if (!active) return;
        setPage(rows ?? { page: 1, pageSize: 20, total: 0, items: [] });
        setStatus(run);
      })
      .catch((reason: unknown) => {
        if (active) setError(reason instanceof Error ? reason.message : '加载失败');
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [api, applied]);

  useEffect(() => {
    const restore = () => {
      const next = readSessionFilters();
      setDraft(next);
      setApplied(next);
      setEmployeeName(readEmployeeName(next.employeeId));
    };
    window.addEventListener('popstate', restore);
    return () => window.removeEventListener('popstate', restore);
  }, []);

  const apply = (next: SessionInsightFilters, historyMode: 'push' | 'replace' = 'push') => {
    const normalized = { ...next, page: next.page || 1 };
    setApplied(normalized);
    setDraft(normalized);
    setEmployeeName(readEmployeeName(normalized.employeeId));
    window.history[historyMode === 'push' ? 'pushState' : 'replaceState']({}, '', writeSessionFilters(normalized));
  };

  const open = (id: number) => {
    setDetailLoading(true);
    void api.sessionDetail(id)
      .then(setDetail)
      .catch((reason: unknown) => setError(reason instanceof Error ? reason.message : '详情加载失败'))
      .finally(() => setDetailLoading(false));
  };

  return <div className="ai-insight-workspace">
    <AiInsightHeader title="会话分析" description="按真实归档会话查看客户判断与员工质检结果" actions={<button className="ai-insight-secondary" type="button" onClick={() => { window.location.href = api.sessionExportUrl(applied); }}>导出结果</button>} />
    {status?.assistant && <section className="ai-insight-assistant-summary" aria-label="会话分析助手配置"><div><strong>{status.assistant.name || '会话分析助手'}</strong><span className={`ai-settings-status ai-settings-status--${status.assistant.enabled ? 'enabled' : 'disabled'}`}>{status.assistant.enabled ? '已启用' : '已停用'}</span></div><p>本页分析由该系统助手生成，当前关联 {status.assistant.knowledgeBaseCount} 个已启用知识库、{status.assistant.readyDocumentCount} 份可用文档。</p><a href="/ai-setting/agent">前往配置</a></section>}
    <AiInsightQueryBar onSubmit={() => apply({ ...draft, page: 1 }, 'push')} onReset={() => apply(emptyFilters, 'push')} onRefresh={() => apply(applied, 'replace')}>
      <AiInsightField label="结果关键词"><input value={draft.keyword ?? ''} onChange={(event) => setDraft({ ...draft, keyword: event.target.value })} placeholder="搜索结果摘要或结论" /></AiInsightField>
      <AiInsightField label="客户名称"><input value={draft.customerName ?? ''} onChange={(event) => setDraft({ ...draft, customerName: event.target.value || undefined })} placeholder="按客户名称筛选" /></AiInsightField>
      <AiInsightField label="会话类型"><select value={draft.conversationType ?? ''} onChange={(event) => setDraft({ ...draft, conversationType: (event.target.value || undefined) as SessionInsightFilters['conversationType'] })}><option value="">全部会话</option><option value="direct">客户单聊</option><option value="group">客户群聊</option></select></AiInsightField>
      <EmployeeSearchField
        label="员工"
        selectedEmployeeId={draft.employeeId}
        knownEmployeeName={employeeName}
        loadOptions={(keyword, limit) => api.sessionFilterOptions(keyword, limit)}
        onSelect={(employee) => {
          setEmployeeName(employee?.name ?? '');
          setDraft((current) => ({ ...current, employeeId: employee?.id }));
        }}
      />
      <AiInsightField label="开始日期"><input type="date" value={draft.startDate ?? ''} onChange={(event) => setDraft({ ...draft, startDate: event.target.value || undefined })} /></AiInsightField>
      <AiInsightField label="结束日期"><input type="date" value={draft.endDate ?? ''} onChange={(event) => setDraft({ ...draft, endDate: event.target.value || undefined })} /></AiInsightField>
    </AiInsightQueryBar>
    <AiInsightStatusStrip status={status} />
    <section className="ai-insight-results"><header className="ai-insight-results-header"><div><h2>分析结果</h2><p>每条记录对应一个真实归档会话，固定每页 20 条</p></div><span>{page.total} 条</span></header>{loading ? <div className="ai-insight-loading">正在读取分析结果…</div> : error ? <div className="ai-insight-error" role="alert">{error}</div> : page.items.length === 0 ? <div className="ai-insight-empty">当前筛选暂无分析结果</div> : <SessionTable page={page} onOpen={open} />}<InsightPagination page={page.page} total={page.total} onChange={(next) => apply({ ...applied, page: next }, 'push')} /></section>
    {detailLoading ? <div className="ai-insight-status">正在打开分析详情…</div> : detail ? <InsightDrawer detail={detail} onClose={() => setDetail(undefined)} onNavigate={navigate} /> : null}
  </div>;
}

export function SessionAnalysisDetail({ detail, onClose }: { detail: InsightDetail<SessionInsightRow>; onClose: () => void }) { return <InsightDrawer detail={detail} onClose={onClose} />; }
