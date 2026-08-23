import { ApiError } from '@mochat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useRef, useState } from 'react';
import type { MouseEvent } from 'react';
import { useOptionalDashboardAccess } from '../../app/access-context';
import { DashboardDialog } from '../../components/dashboard-dialog';
import { Phase35PageShell } from '../phase35/components/phase35-page-shell';
import { Phase35DataState } from '../phase35/components/data-state';
import type { AgentItem, AISettingsApi, SmartAnalysisRule } from './ai-settings-api';
import { formatAISettingsTime, resolveKnowledgeBaseNames } from './list-state';

function operationError(error: unknown): string {
  if (error instanceof ApiError) {
    const messages: Record<string, string> = {
      AI_SETTINGS_KNOWLEDGE_BASE_INVALID: '关联知识库已失效或不属于当前企业，请刷新后重新选择。',
      AI_SETTINGS_NOT_FOUND: '系统助手或默认规则不存在，请刷新后重试。',
      AI_SETTINGS_STATUS_INVALID: '状态值无效，请重新选择。',
      AI_SETTINGS_DESCRIPTION_INVALID: '分析要求过长，请精简后重试。',
      AI_SETTINGS_SMART_RULE_INVALID: '默认智能分析规则无效，请检查分析目标、会话范围、回看天数和最少消息数。',
      AI_SETTINGS_INVALID_JSON: '请求格式无效，请刷新后重试。',
      AI_SETTINGS_STORAGE_FAILURE: '服务暂时不可用，请稍后重试。',
    };
    return messages[error.machineCode ?? ''] ?? '操作失败，请稍后重试。';
  }
  return '操作失败，请稍后重试。';
}

function textLength(value: string): number { return Array.from(value).length; }
function sameStrings(left: string[], right: string[]): boolean { return JSON.stringify(left) === JSON.stringify(right); }

type EditorState = {
  description: string;
  selectedBases: string[];
  status: number;
  objective: string;
  conversationTypes: Array<'direct' | 'group'>;
  lookbackDays: number;
  minimumMessages: number;
};

function editorState(agent: AgentItem, rule: SmartAnalysisRule): EditorState {
  return {
    description: agent.description,
    selectedBases: agent.knowledgeBaseIds ?? [],
    status: agent.status,
    objective: rule.objective,
    conversationTypes: [...rule.conversationTypes],
    lookbackDays: rule.lookbackDays,
    minimumMessages: rule.minimumMessages,
  };
}

export function AgentPage({ api }: { api: AISettingsApi }) {
  const corpId = useOptionalDashboardAccess()?.corp.id;
  const queryClient = useQueryClient();
  const [editing, setEditing] = useState<AgentItem | null>(null);
  const [draft, setDraft] = useState<EditorState | null>(null);
  const [editorError, setEditorError] = useState('');
  const [feedback, setFeedback] = useState<{ kind: 'error' | 'success'; text: string } | null>(null);
  const [discardOpen, setDiscardOpen] = useState(false);
  const editorTriggerRef = useRef<HTMLButtonElement>(null);

  const query = useQuery({ queryKey: ['ai-agents', corpId], queryFn: () => api.listAgents(Number(corpId)), enabled: Boolean(corpId) });
  const knowledgeBases = useQuery({ queryKey: ['ai-kb', corpId], queryFn: () => api.listKnowledgeBases(Number(corpId)), enabled: Boolean(corpId) });
  const agent = query.data?.[0];
  const rule = agent?.smartAnalysisRule;
  const activeBases = (knowledgeBases.data ?? []).filter((item) => item.status === 1);
  const relatedBaseNames = agent && knowledgeBases.isSuccess ? resolveKnowledgeBaseNames(agent.knowledgeBaseIds ?? [], knowledgeBases.data ?? []) : '—';
  const initial = editing?.smartAnalysisRule ? editorState(editing, editing.smartAnalysisRule) : null;
  const dirty = Boolean(draft && initial && (
    draft.description !== initial.description || !sameStrings(draft.selectedBases, initial.selectedBases)
    || draft.status !== initial.status || draft.objective !== initial.objective
    || !sameStrings(draft.conversationTypes, initial.conversationTypes)
    || draft.lookbackDays !== initial.lookbackDays || draft.minimumMessages !== initial.minimumMessages
  ));

  function resetEditor() { setEditing(null); setDraft(null); setEditorError(''); setDiscardOpen(false); }
  function requestEditorClose() {
    if (save.isPending) return;
    if (dirty) { setDiscardOpen(true); return; }
    resetEditor();
  }
  function openEditor(item: AgentItem, event: MouseEvent<HTMLButtonElement>) {
    if (!item.smartAnalysisRule) return;
    editorTriggerRef.current = event.currentTarget;
    setEditing(item); setDraft(editorState(item, item.smartAnalysisRule)); setEditorError(''); setFeedback(null);
  }
  function toggleBase(id: string) {
    setDraft((current) => current ? { ...current, selectedBases: current.selectedBases.includes(id) ? current.selectedBases.filter((item) => item !== id) : [...current.selectedBases, id] } : current);
  }
  function toggleConversationType(value: 'direct' | 'group') {
    setDraft((current) => current ? { ...current, conversationTypes: current.conversationTypes.includes(value) ? current.conversationTypes.filter((item) => item !== value) : [...current.conversationTypes, value] } : current);
  }

  const save = useMutation({
    mutationFn: () => api.updateAgent(Number(corpId), String(editing?.id), {
      name: '会话分析助手', description: draft?.description.trim() ?? '', knowledgeBaseIds: draft?.selectedBases ?? [], status: draft?.status ?? 0,
      smartAnalysisRule: { objective: draft?.objective.trim() ?? '', conversationTypes: draft?.conversationTypes ?? [], lookbackDays: draft?.lookbackDays ?? 0, minimumMessages: draft?.minimumMessages ?? 0 },
    }),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['ai-agents', corpId] });
      setFeedback({ kind: 'success', text: '分析助手配置已保存，会话分析和智能分析将在下一次运行使用新配置。' }); resetEditor();
    },
    onError: (error) => setEditorError(operationError(error)),
  });

  const valid = Boolean(corpId && editing && draft && editing.smartAnalysisRule && knowledgeBases.isSuccess
    && textLength(draft.description) <= 512 && textLength(draft.objective.trim()) >= 2 && textLength(draft.objective.trim()) <= 500
    && draft.conversationTypes.length > 0 && draft.lookbackDays >= 1 && draft.lookbackDays <= 30 && draft.minimumMessages >= 2 && draft.minimumMessages <= 50);
  const listedBaseIds = new Set((knowledgeBases.data ?? []).map((item) => item.id));
  const initialBases = editing?.knowledgeBaseIds ?? [];
  const invalidOriginalBases = initialBases.filter((id) => !listedBaseIds.has(id));

  return (
    <Phase35PageShell title="分析助手" description="统一配置 AI 洞察使用的系统助手、知识库和默认智能分析规则">
      <div className="phase35-page ai-settings-workspace ai-assistant-workspace">
        <section className="ai-assistant-intro" aria-label="AI 洞察与设置关系"><div><strong>AI 设置负责配置，AI 洞察负责查看结果</strong><p>系统只提供一个有真实业务入口的会话分析助手，不开放新增或删除。</p></div><a href="/ai-setting/ai-knowledge-base">管理 AI 知识库</a></section>
        {feedback && <p className={`ai-settings-feedback ai-settings-feedback--${feedback.kind}`} role={feedback.kind === 'error' ? 'alert' : 'status'}>{feedback.text}</p>}
        <Phase35DataState loading={query.isLoading} error={query.isError} empty={!agent} emptyContent={<p className="phase35-empty">系统助手初始化失败，请刷新重试</p>} onRetry={() => void query.refetch()}>
          {agent && <article className="ai-assistant-card">
            <header className="ai-assistant-card__header"><div className="ai-assistant-card__identity"><span className="ai-assistant-card__icon" aria-hidden="true">AI</span><div><div className="ai-assistant-card__title"><h2>{agent.name}</h2><span className="phase35-chip">固定系统助手</span><span className={`ai-settings-status ai-settings-status--${agent.status === 1 ? 'enabled' : 'disabled'}`}>{agent.status === 1 ? '已启用' : '已停用'}</span></div><p>{agent.description || '尚未填写分析要求'}</p></div></div><div className="ai-assistant-card__actions"><button className="ai-settings-action-button ai-settings-secondary-button" type="button" onClick={() => { setFeedback(null); void Promise.all([query.refetch(), knowledgeBases.refetch()]); }} disabled={query.isFetching || knowledgeBases.isFetching}>刷新</button><button className="ai-settings-action-button ai-settings-primary-button" type="button" onClick={(event) => openEditor(agent, event)} disabled={!rule}>编辑配置</button></div></header>
            {!rule && <p className="ai-settings-feedback ai-settings-feedback--error" role="alert">默认智能分析规则初始化失败，请刷新后重试。</p>}
            <div className="ai-assistant-use-grid"><a className="ai-assistant-use-card" href="/ai-insight/session-analysis"><span>用于会话分析</span><strong>客户洞察与员工质检</strong><small>{agent.knowledgeBaseCount} 个知识库 · {agent.readyDocumentCount} 份就绪文档</small></a><a className="ai-assistant-use-card" href="/ai-insight/smart-analysis"><span>用于智能分析</span><strong>{rule?.name ?? '默认规则不可用'}</strong><small>{rule ? `v${rule.currentVersion} · 回看 ${rule.lookbackDays} 天 · 至少 ${rule.minimumMessages} 条消息` : '请刷新后重试'}</small></a></div>
            <dl className="ai-assistant-summary-grid"><div><dt>关联知识库</dt><dd>{knowledgeBases.isLoading ? '正在加载…' : knowledgeBases.isError ? '加载失败' : relatedBaseNames}</dd></div><div><dt>智能分析目标</dt><dd>{rule?.objective ?? '—'}</dd></div><div><dt>会话范围</dt><dd>{rule ? rule.conversationTypes.map((item) => item === 'direct' ? '客户单聊' : '客户群聊').join('、') : '—'}</dd></div><div><dt>最后更新</dt><dd>{formatAISettingsTime(agent.updatedAt)}</dd></div></dl>
          </article>}
        </Phase35DataState>

        <DashboardDialog open={Boolean(editing && draft)} title="配置会话分析助手" triggerRef={editorTriggerRef} confirmDisabled={!valid} confirmLoading={save.isPending} onCancel={requestEditorClose} onConfirm={() => save.mutate()}>
          {draft && <div className="ai-settings-dialog-body ai-assistant-dialog-body">{editorError && <p role="alert" className="ai-settings-feedback ai-settings-feedback--error">{editorError}</p>}<form onSubmit={(event) => { event.preventDefault(); if (valid && !save.isPending) save.mutate(); }}>
            <section className="ai-assistant-editor-section"><header><h3>助手要求</h3><p>同时应用到会话分析与智能分析。</p></header><label>名称<input value="会话分析助手" readOnly aria-readonly="true" /><small>系统固定名称，不支持新增或改名。</small></label><label>AI 分析要求<textarea value={draft.description} onChange={(event) => setDraft({ ...draft, description: event.target.value })} rows={4} placeholder="例如：重点识别客户采购意向、流失风险和员工服务质量" /></label><label>运行状态<select aria-label="运行状态" value={draft.status} onChange={(event) => setDraft({ ...draft, status: Number(event.target.value) })}><option value={1}>启用</option><option value={0}>停用</option></select><small>停用后两类 AI 洞察都会记录真实停用状态，不会生成假结果。</small></label></section>
            <section className="ai-assistant-editor-section"><header><h3>关联知识库</h3><p>知识只作背景资料，来源会话仍是唯一事实证据。</p></header><fieldset className="ai-settings-base-options"><legend className="sr-only">关联知识库</legend>{knowledgeBases.isLoading && <p role="status">正在加载可关联知识库…</p>}{knowledgeBases.isError && <p role="alert">知识库加载失败，暂时无法安全保存关联配置。请刷新后重试。</p>}{knowledgeBases.isSuccess && activeBases.length === 0 && initialBases.length === 0 && <p className="phase35-empty">暂无已启用知识库可选。<a href="/ai-setting/ai-knowledge-base">前往 AI 知识库</a></p>}{(knowledgeBases.data ?? []).map((kb) => { const previouslyRelated = initialBases.includes(kb.id); const selectable = kb.status === 1 || previouslyRelated; return <label key={kb.id} className={!selectable ? 'ai-settings-base-option--disabled' : undefined}><input type="checkbox" checked={draft.selectedBases.includes(kb.id)} disabled={!selectable} onChange={() => toggleBase(kb.id)} />{kb.name}<small>{kb.status === 1 ? '已启用' : previouslyRelated ? '已停用，仅可保留既有关联或移除' : '已停用，不可新关联'}</small></label>; })}{invalidOriginalBases.map((id) => <label key={id} className="ai-settings-base-option--invalid"><input type="checkbox" checked={draft.selectedBases.includes(id)} disabled={!draft.selectedBases.includes(id)} onChange={() => toggleBase(id)} />已失效（ID: {id}）<small>仅可从既有关联中移除</small></label>)}</fieldset></section>
            <section className="ai-assistant-editor-section"><header><h3>默认智能分析规则</h3><p>所有智能分析只使用这一条规则；每次规则变更会生成新的可追溯版本。</p></header><label>规则名称<input value="默认智能分析规则" readOnly aria-readonly="true" /></label><label>智能分析目标<textarea aria-label="智能分析目标" value={draft.objective} onChange={(event) => setDraft({ ...draft, objective: event.target.value })} rows={3} /></label><fieldset className="ai-assistant-conversation-types"><legend>会话范围</legend><label><input aria-label="客户单聊" type="checkbox" checked={draft.conversationTypes.includes('direct')} onChange={() => toggleConversationType('direct')} />客户单聊</label><label><input aria-label="客户群聊" type="checkbox" checked={draft.conversationTypes.includes('group')} onChange={() => toggleConversationType('group')} />客户群聊</label></fieldset><div className="ai-assistant-rule-numbers"><label>回看天数<input aria-label="回看天数" type="number" min={1} max={30} value={draft.lookbackDays} onChange={(event) => setDraft({ ...draft, lookbackDays: Number(event.target.value) })} /><small>1–30 天</small></label><label>最少消息数<input aria-label="最少消息数" type="number" min={2} max={50} value={draft.minimumMessages} onChange={(event) => setDraft({ ...draft, minimumMessages: Number(event.target.value) })} /><small>2–50 条</small></label></div></section>
          </form></div>}
        </DashboardDialog>
        <DashboardDialog open={discardOpen} title="放弃未保存更改？" danger confirmText="放弃更改" cancelText="继续编辑" onCancel={() => setDiscardOpen(false)} onConfirm={resetEditor}><p>当前修改尚未保存，放弃后无法恢复。</p></DashboardDialog>
      </div>
    </Phase35PageShell>
  );
}
