import { useEffect, useState } from 'react';
import { AiInsightField, AiInsightHeader, AiInsightQueryBar, AiInsightStatusStrip, InsightDrawer, InsightPagination, SmartTable, type WorkspaceProps } from './ai-insight-workspace';
import type { InsightDetail, InsightPage, InsightRunStatus, SmartInsightFilters, SmartInsightRow } from './ai-insight-workspace-api';
import { readSmartState, writeSmartState } from './ai-insight-url-state';

const emptyFilters: SmartInsightFilters = { page: 1 };

export function SmartAnalysisPage({ api, navigate }: WorkspaceProps) {
  const initial = readSmartState().filters;
  const [draft, setDraft] = useState<SmartInsightFilters>(initial);
  const [applied, setApplied] = useState<SmartInsightFilters>(initial);
  const [page, setPage] = useState<InsightPage<SmartInsightRow>>({ page: 1, pageSize: 20, total: 0, items: [] });
  const [status, setStatus] = useState<InsightRunStatus>();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [detail, setDetail] = useState<InsightDetail<SmartInsightRow>>();
  const [detailLoading, setDetailLoading] = useState(false);

  useEffect(() => {
    let active = true;
    setLoading(true);
    setError('');
    void Promise.all([api.smartRecords(applied), api.smartStatus()])
      .then(([rows, run]) => {
        if (!active) return;
        setPage(rows ?? { page: 1, pageSize: 20, total: 0, items: [] });
        setStatus(run);
      })
      .catch((reason: unknown) => { if (active) setError(reason instanceof Error ? reason.message : '加载失败'); })
      .finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [api, applied]);

  const apply = (next: SmartInsightFilters) => {
    const normalized = { ...next, page: next.page || 1 };
    setApplied(normalized);
    setDraft(normalized);
    window.history.replaceState({}, '', writeSmartState({ filters: normalized }));
  };
  const open = (id: number) => {
    setDetailLoading(true);
    void api.smartDetail(id)
      .then(setDetail)
      .catch((reason: unknown) => setError(reason instanceof Error ? reason.message : '详情加载失败'))
      .finally(() => setDetailLoading(false));
  };

  return <div className="ai-insight-workspace">
    <AiInsightHeader title="智能分析" description="集中查看默认分析助手识别出的业务信号" />
    {status?.assistant && <section className="ai-insight-assistant-summary" aria-label="智能分析助手配置"><div><strong>{status.assistant.name || '会话分析助手'}</strong><span className={`ai-settings-status ai-settings-status--${status.assistant.enabled ? 'enabled' : 'disabled'}`}>{status.assistant.enabled ? '已启用' : '已停用'}</span></div><p>本页结果统一使用分析助手中的默认智能分析规则，当前关联 {status.assistant.knowledgeBaseCount} 个已启用知识库、{status.assistant.readyDocumentCount} 份可用文档。</p><a href="/ai-setting/agent">前往配置</a></section>}
    <AiInsightQueryBar onSubmit={() => apply({ ...draft, page: 1 })} onReset={() => apply(emptyFilters)} onRefresh={() => apply(applied)}>
      <AiInsightField label="关键词"><input value={draft.keyword ?? ''} onChange={(event) => setDraft({ ...draft, keyword: event.target.value })} placeholder="搜索分析结论" /></AiInsightField>
      <AiInsightField label="会话类型"><select value={draft.conversationType ?? ''} onChange={(event) => setDraft({ ...draft, conversationType: (event.target.value || undefined) as SmartInsightFilters['conversationType'] })}><option value="">全部会话</option><option value="direct">客户单聊</option><option value="group">客户群聊</option></select></AiInsightField>
      <AiInsightField label="员工 ID"><input inputMode="numeric" value={draft.employeeId ?? ''} onChange={(event) => setDraft({ ...draft, employeeId: event.target.value ? Number(event.target.value) : undefined })} placeholder="可选" /></AiInsightField>
      <AiInsightField label="开始日期"><input type="date" value={draft.startDate ?? ''} onChange={(event) => setDraft({ ...draft, startDate: event.target.value || undefined })} /></AiInsightField>
      <AiInsightField label="结束日期"><input type="date" value={draft.endDate ?? ''} onChange={(event) => setDraft({ ...draft, endDate: event.target.value || undefined })} /></AiInsightField>
    </AiInsightQueryBar>
    <AiInsightStatusStrip status={status} />
    <section className="ai-insight-results"><header className="ai-insight-results-header"><div><h2>分析结果</h2><p>规则版本随结果快照保留；规则配置统一在分析助手中维护</p></div><span>{page.total} 条</span></header>{loading ? <div className="ai-insight-loading">正在读取智能分析结果…</div> : error ? <div className="ai-insight-error" role="alert">{error}</div> : page.items.length === 0 ? <div className="ai-insight-empty">当前暂无匹配的智能分析结果</div> : <SmartTable page={page} onOpen={open} />}<InsightPagination page={page.page} total={page.total} onChange={(next) => apply({ ...applied, page: next })} /></section>
    {detailLoading ? <div className="ai-insight-status">正在打开分析详情…</div> : detail ? <InsightDrawer detail={detail} onClose={() => setDetail(undefined)} onNavigate={navigate} /> : null}
  </div>;
}
