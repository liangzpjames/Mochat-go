import { ApiError } from '@mochat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useRef, useState } from 'react';
import type { KeyboardEvent, MouseEvent } from 'react';
import { useOptionalDashboardAccess } from '../../app/access-context';
import { DashboardDialog } from '../../components/dashboard-dialog';
import { Phase35PageShell } from '../phase35/components/phase35-page-shell';
import { Phase35DataState } from '../phase35/components/data-state';
import type {
  AISettingsApi,
  ConversationType,
  KnowledgeBaseItem,
  SessionAnalysisAgentItem,
  SmartAnalysisAgentItem,
} from './ai-settings-api';
import { resolveKnowledgeBaseNames } from './list-state';

function operationError(error: unknown): string {
  if (error instanceof ApiError) {
    const messages: Record<string, string> = {
      AI_SETTINGS_KNOWLEDGE_BASE_INVALID: '关联知识库已失效或不属于当前企业，请刷新后重新选择。',
      AI_SETTINGS_NOT_FOUND: '系统助手或分析规则不存在，请刷新后重试。',
      AI_SETTINGS_STATUS_INVALID: '状态值无效，请重新选择。',
      AI_SETTINGS_DESCRIPTION_INVALID: '助手说明过长，请精简后重试。',
      AI_SETTINGS_SESSION_RULE_INVALID: '会话分析提示词无效，请检查客户分析与员工质检提示词。',
      AI_SETTINGS_SMART_RULE_INVALID: '智能分析规则无效，请检查分析目标、会话范围、回看天数和最少消息数。',
      AI_SETTINGS_INVALID_JSON: '请求格式无效，请刷新后重试。',
      AI_SETTINGS_STORAGE_FAILURE: '服务暂时不可用，请稍后重试。',
    };
    return messages[error.machineCode ?? ''] ?? '操作失败，请稍后重试。';
  }
  return '操作失败，请稍后重试。';
}

function textLength(value: string): number { return Array.from(value).length; }
function sameStrings(left: string[], right: string[]): boolean { return JSON.stringify(left) === JSON.stringify(right); }
function isIntegerInRange(value: number, min: number, max: number): boolean {
  return Number.isInteger(value) && value >= min && value <= max;
}

type SessionDraft = {
  description: string;
  selectedBases: string[];
  status: number;
  customerAnalysisPrompt: string;
  employeeQaPrompt: string;
  conversationTypes: ConversationType[];
  lookbackDays: number;
  minimumMessages: number;
};

type SmartDraft = {
  description: string;
  selectedBases: string[];
  status: number;
  objective: string;
  conversationTypes: ConversationType[];
  lookbackDays: number;
  minimumMessages: number;
};

function sessionDraftFromAgent(agent: SessionAnalysisAgentItem): SessionDraft {
  return {
    description: agent.description,
    selectedBases: [...agent.knowledgeBaseIds],
    status: agent.status,
    customerAnalysisPrompt: agent.sessionAnalysisRule.customerAnalysisPrompt,
    employeeQaPrompt: agent.sessionAnalysisRule.employeeQaPrompt,
    conversationTypes: [...agent.sessionAnalysisRule.conversationTypes],
    lookbackDays: agent.sessionAnalysisRule.lookbackDays,
    minimumMessages: agent.sessionAnalysisRule.minimumMessages,
  };
}

function smartDraftFromAgent(agent: SmartAnalysisAgentItem): SmartDraft {
  return {
    description: agent.description,
    selectedBases: [...agent.knowledgeBaseIds],
    status: agent.status,
    objective: agent.smartAnalysisRule.objective,
    conversationTypes: [...agent.smartAnalysisRule.conversationTypes],
    lookbackDays: agent.smartAnalysisRule.lookbackDays,
    minimumMessages: agent.smartAnalysisRule.minimumMessages,
  };
}

function keyboardToggle(event: KeyboardEvent<HTMLElement>, toggle: () => void) {
  if (event.key === 'Enter' || event.key === ' ') {
    event.preventDefault();
    toggle();
  }
}

function knowledgeBaseChoices(
  knowledgeBases: KnowledgeBaseItem[],
  selectedBaseIds: string[],
  initialBaseIds: string[],
) {
  const listedBaseIds = new Set(knowledgeBases.map((item) => item.id));
  const invalidOriginalBases = initialBaseIds.filter((id) => !listedBaseIds.has(id));
  const choices = knowledgeBases.map((knowledgeBase) => {
    const previouslyRelated = initialBaseIds.includes(knowledgeBase.id);
    const selectable = knowledgeBase.status === 1 || previouslyRelated;
    return {
      id: knowledgeBase.id,
      label: knowledgeBase.name,
      checked: selectedBaseIds.includes(knowledgeBase.id),
      disabled: !selectable,
      hint: knowledgeBase.status === 1
        ? '已启用'
        : previouslyRelated
          ? '已停用，仅可保留既有关联或移除'
          : '已停用，不可新关联',
    };
  });
  const staleChoices = invalidOriginalBases.map((id) => ({
    id,
    label: `已失效（ID: ${id}）`,
    checked: selectedBaseIds.includes(id),
    disabled: !selectedBaseIds.includes(id),
    hint: '仅可从既有关联中移除',
  }));
  return { choices, staleChoices };
}

function AssistantKnowledgeBaseFieldset({
  knowledgeBases,
  isLoading,
  isError,
  selectedBaseIds,
  initialBaseIds,
  onToggle,
}: {
  knowledgeBases: KnowledgeBaseItem[];
  isLoading: boolean;
  isError: boolean;
  selectedBaseIds: string[];
  initialBaseIds: string[];
  onToggle: (id: string) => void;
}) {
  const { choices, staleChoices } = knowledgeBaseChoices(knowledgeBases, selectedBaseIds, initialBaseIds);

  return (
    <fieldset className="ai-settings-base-options">
      <legend className="sr-only">关联知识库</legend>
      {isLoading && <p role="status">正在加载可关联知识库…</p>}
      {isError && <p role="alert">知识库加载失败，暂时无法安全保存关联配置。请刷新后重试。</p>}
      {!isLoading && !isError && choices.length === 0 && staleChoices.length === 0 && (
        <p className="phase35-empty">暂无已启用知识库可选。<a href="/ai-setting/ai-knowledge-base">前往 AI 知识库</a></p>
      )}
      {choices.map((choice) => (
        <label
          key={choice.id}
          className={choice.disabled ? 'ai-settings-base-option--disabled' : undefined}
        >
          <input
            aria-label={choice.label}
            type="checkbox"
            checked={choice.checked}
            disabled={choice.disabled}
            onChange={() => onToggle(choice.id)}
          />
          {choice.label}
          <small>{choice.hint}</small>
        </label>
      ))}
      {staleChoices.map((choice) => (
        <label key={choice.id} className="ai-settings-base-option--invalid">
          <input
            aria-label={choice.label}
            type="checkbox"
            checked={choice.checked}
            disabled={choice.disabled}
            onChange={() => onToggle(choice.id)}
          />
          {choice.label}
          <small>{choice.hint}</small>
        </label>
      ))}
    </fieldset>
  );
}

function conversationScopeLabel(value: ConversationType): string {
  return value === 'direct' ? '客户单聊' : '客户群聊';
}

function conversationScopeDescription(value: ConversationType): string {
  return value === 'direct'
    ? '适合客户单聊中的意向、流失和服务质量判断。'
    : '适合群聊中的协同信号、异议和群体互动观察。';
}

function ConversationScopeFieldset({
  conversationTypes,
  onToggle,
}: {
  conversationTypes: ConversationType[];
  onToggle: (value: ConversationType) => void;
}) {
  return (
    <fieldset className="ai-conversation-scope-fieldset">
      <legend>会话范围</legend>
      <p className="ai-conversation-scope-caption">至少保留一种会话类型，当前选择只作用于当前助手。</p>
      <div className="ai-conversation-scope-grid">
        {(['direct', 'group'] as const).map((type) => {
          const checked = conversationTypes.includes(type);
          const title = conversationScopeLabel(type);
          return (
            <label
              key={type}
              className={`ai-conversation-scope-option${checked ? ' ai-conversation-scope-option--selected' : ''}`}
              tabIndex={0}
              onKeyDown={(event) => keyboardToggle(event, () => onToggle(type))}
            >
              <input
                aria-label={title}
                type="checkbox"
                checked={checked}
                onChange={() => onToggle(type)}
              />
              <div>
                <strong>{title}</strong>
                <small>{conversationScopeDescription(type)}</small>
              </div>
            </label>
          );
        })}
      </div>
    </fieldset>
  );
}

function SessionAssistantCard({
  agent,
  knowledgeBaseNames,
  onRefresh,
  onEdit,
  pending,
}: {
  agent: SessionAnalysisAgentItem;
  knowledgeBaseNames: string;
  onRefresh: () => void;
  onEdit: (event: MouseEvent<HTMLButtonElement>) => void;
  pending: boolean;
}) {
  return (
    <article className="ai-assistant-card">
      <header className="ai-assistant-card__header">
        <div className="ai-assistant-card__identity">
          <span className="ai-assistant-card__icon" aria-hidden="true">AI</span>
          <div>
            <div className="ai-assistant-card__title">
              <h2>{agent.name}</h2>
              <span className="phase35-chip">固定系统助手</span>
              <span className={`ai-settings-status ai-settings-status--${agent.status === 1 ? 'enabled' : 'disabled'}`}>{agent.status === 1 ? '已启用' : '已停用'}</span>
            </div>
            <p>{agent.description || '尚未填写助手说明'}</p>
          </div>
        </div>
        <div className="ai-assistant-card__actions">
          <button aria-label={`刷新 ${agent.name}`} className="ai-settings-action-button ai-settings-secondary-button" type="button" onClick={onRefresh} disabled={pending}>刷新</button>
          <button aria-label={`编辑配置 ${agent.name}`} className="ai-settings-action-button ai-settings-primary-button" type="button" onClick={onEdit} disabled={pending}>编辑配置</button>
        </div>
      </header>
      <div className="ai-assistant-summary-grid">
        <div><dt>服务范围</dt><dd>客户洞察与员工质检</dd></div>
        <div><dt>提示词版本</dt><dd>v{agent.sessionAnalysisRule.currentVersion} · 双提示词配置</dd></div>
        <div><dt>关联知识库</dt><dd>{knowledgeBaseNames}</dd></div>
        <div><dt>文档准备情况</dt><dd>{agent.knowledgeBaseCount} 个知识库 · {agent.readyDocumentCount} 份就绪文档</dd></div>
      </div>
      <div className="ai-assistant-summary-grid">
        <div><dt>客户分析提示词</dt><dd>{agent.sessionAnalysisRule.customerAnalysisPrompt}</dd></div>
        <div><dt>员工质检提示词</dt><dd>{agent.sessionAnalysisRule.employeeQaPrompt}</dd></div>
        <div><dt>会话范围</dt><dd>{agent.sessionAnalysisRule.conversationTypes.map(conversationScopeLabel).join('、')}</dd></div>
        <div><dt>规则窗口</dt><dd>回看 {agent.sessionAnalysisRule.lookbackDays} 天 · 至少 {agent.sessionAnalysisRule.minimumMessages} 条消息</dd></div>
      </div>
      <div className="ai-assistant-summary-grid">
        <div><dt>运行状态</dt><dd>{agent.status === 1 ? '仅影响会话分析运行' : '当前已停用，会话分析不会产出新结果'}</dd></div>
        <div><dt>结果入口</dt><dd><a href="/ai-insight/session-analysis">前往会话分析</a></dd></div>
      </div>
    </article>
  );
}

function SmartAssistantCard({
  agent,
  knowledgeBaseNames,
  onRefresh,
  onEdit,
  pending,
}: {
  agent: SmartAnalysisAgentItem;
  knowledgeBaseNames: string;
  onRefresh: () => void;
  onEdit: (event: MouseEvent<HTMLButtonElement>) => void;
  pending: boolean;
}) {
  return (
    <article className="ai-assistant-card">
      <header className="ai-assistant-card__header">
        <div className="ai-assistant-card__identity">
          <span className="ai-assistant-card__icon" aria-hidden="true">AI</span>
          <div>
            <div className="ai-assistant-card__title">
              <h2>{agent.name}</h2>
              <span className="phase35-chip">固定系统助手</span>
              <span className={`ai-settings-status ai-settings-status--${agent.status === 1 ? 'enabled' : 'disabled'}`}>{agent.status === 1 ? '已启用' : '已停用'}</span>
            </div>
            <p>{agent.description || '尚未填写助手说明'}</p>
          </div>
        </div>
        <div className="ai-assistant-card__actions">
          <button aria-label={`刷新 ${agent.name}`} className="ai-settings-action-button ai-settings-secondary-button" type="button" onClick={onRefresh} disabled={pending}>刷新</button>
          <button aria-label={`编辑配置 ${agent.name}`} className="ai-settings-action-button ai-settings-primary-button" type="button" onClick={onEdit} disabled={pending}>编辑配置</button>
        </div>
      </header>
      <div className="ai-assistant-summary-grid">
        <div><dt>服务范围</dt><dd>业务信号识别</dd></div>
        <div><dt>规则版本</dt><dd>v{agent.smartAnalysisRule.currentVersion}</dd></div>
        <div><dt>关联知识库</dt><dd>{knowledgeBaseNames}</dd></div>
        <div><dt>文档准备情况</dt><dd>{agent.knowledgeBaseCount} 个知识库 · {agent.readyDocumentCount} 份就绪文档</dd></div>
      </div>
      <div className="ai-assistant-summary-grid">
        <div><dt>智能分析目标</dt><dd>{agent.smartAnalysisRule.objective}</dd></div>
        <div><dt>会话范围</dt><dd>{agent.smartAnalysisRule.conversationTypes.map(conversationScopeLabel).join('、')}</dd></div>
        <div><dt>规则窗口</dt><dd>回看 {agent.smartAnalysisRule.lookbackDays} 天 · 至少 {agent.smartAnalysisRule.minimumMessages} 条消息</dd></div>
        <div><dt>结果入口</dt><dd><a href="/ai-insight/smart-analysis">前往智能分析</a></dd></div>
      </div>
    </article>
  );
}

export function AgentPage({ api }: { api: AISettingsApi }) {
  const corpId = useOptionalDashboardAccess()?.corp.id;
  const queryClient = useQueryClient();
  const sessionTriggerRef = useRef<HTMLButtonElement>(null);
  const smartTriggerRef = useRef<HTMLButtonElement>(null);
  const [sessionDraft, setSessionDraft] = useState<SessionDraft | null>(null);
  const [smartDraft, setSmartDraft] = useState<SmartDraft | null>(null);
  const [sessionError, setSessionError] = useState('');
  const [smartError, setSmartError] = useState('');
  const [feedback, setFeedback] = useState<{ kind: 'error' | 'success'; text: string } | null>(null);
  const [discardTarget, setDiscardTarget] = useState<'session' | 'smart' | null>(null);

  const agentsQuery = useQuery({ queryKey: ['ai-agents', corpId], queryFn: () => api.listAgents(Number(corpId)), enabled: Boolean(corpId) });
  const knowledgeBasesQuery = useQuery({ queryKey: ['ai-kb', corpId], queryFn: () => api.listKnowledgeBases(Number(corpId)), enabled: Boolean(corpId) });
  const agents = agentsQuery.data ?? [];
  const sessionAgent = agents.find((item): item is SessionAnalysisAgentItem => item.systemKey === 'session-analysis');
  const smartAgent = agents.find((item): item is SmartAnalysisAgentItem => item.systemKey === 'smart-analysis');
  const knowledgeBases = knowledgeBasesQuery.data ?? [];
  const refreshPending = agentsQuery.isFetching || knowledgeBasesQuery.isFetching;

  const sessionInitial = sessionAgent ? sessionDraftFromAgent(sessionAgent) : null;
  const smartInitial = smartAgent ? smartDraftFromAgent(smartAgent) : null;
  const sessionDirty = Boolean(sessionDraft && sessionInitial && (
    sessionDraft.description !== sessionInitial.description
    || !sameStrings(sessionDraft.selectedBases, sessionInitial.selectedBases)
    || sessionDraft.status !== sessionInitial.status
    || sessionDraft.customerAnalysisPrompt !== sessionInitial.customerAnalysisPrompt
    || sessionDraft.employeeQaPrompt !== sessionInitial.employeeQaPrompt
    || !sameStrings(sessionDraft.conversationTypes, sessionInitial.conversationTypes)
    || sessionDraft.lookbackDays !== sessionInitial.lookbackDays
    || sessionDraft.minimumMessages !== sessionInitial.minimumMessages
  ));
  const smartDirty = Boolean(smartDraft && smartInitial && (
    smartDraft.description !== smartInitial.description
    || !sameStrings(smartDraft.selectedBases, smartInitial.selectedBases)
    || smartDraft.status !== smartInitial.status
    || smartDraft.objective !== smartInitial.objective
    || !sameStrings(smartDraft.conversationTypes, smartInitial.conversationTypes)
    || smartDraft.lookbackDays !== smartInitial.lookbackDays
    || smartDraft.minimumMessages !== smartInitial.minimumMessages
  ));

  async function refetchAll() {
    setFeedback(null);
    await Promise.all([agentsQuery.refetch(), knowledgeBasesQuery.refetch()]);
  }

  function closeSessionEditor() {
    setSessionDraft(null);
    setSessionError('');
    setDiscardTarget(null);
  }

  function closeSmartEditor() {
    setSmartDraft(null);
    setSmartError('');
    setDiscardTarget(null);
  }

  function requestSessionClose() {
    if (saveSession.isPending) return;
    if (sessionDirty) {
      setDiscardTarget('session');
      return;
    }
    closeSessionEditor();
  }

  function requestSmartClose() {
    if (saveSmart.isPending) return;
    if (smartDirty) {
      setDiscardTarget('smart');
      return;
    }
    closeSmartEditor();
  }

  function toggleSessionBase(id: string) {
    setSessionDraft((current) => current
      ? { ...current, selectedBases: current.selectedBases.includes(id) ? current.selectedBases.filter((item) => item !== id) : [...current.selectedBases, id] }
      : current);
  }

  function toggleSmartBase(id: string) {
    setSmartDraft((current) => current
      ? { ...current, selectedBases: current.selectedBases.includes(id) ? current.selectedBases.filter((item) => item !== id) : [...current.selectedBases, id] }
      : current);
  }

  function toggleSessionConversationType(value: ConversationType) {
    setSessionDraft((current) => {
      if (!current) return current;
      if (current.conversationTypes.includes(value)) {
        if (current.conversationTypes.length === 1) return current;
        return { ...current, conversationTypes: current.conversationTypes.filter((item) => item !== value) };
      }
      return { ...current, conversationTypes: [...current.conversationTypes, value] };
    });
  }

  function toggleConversationType(value: ConversationType) {
    setSmartDraft((current) => {
      if (!current) return current;
      if (current.conversationTypes.includes(value)) {
        if (current.conversationTypes.length === 1) return current;
        return { ...current, conversationTypes: current.conversationTypes.filter((item) => item !== value) };
      }
      return { ...current, conversationTypes: [...current.conversationTypes, value] };
    });
  }

  const saveSession = useMutation({
    mutationFn: () => api.updateAgent(Number(corpId), sessionAgent?.id ?? '', {
      name: sessionAgent?.name ?? '会话分析助手',
      description: sessionDraft?.description.trim() ?? '',
      knowledgeBaseIds: sessionDraft?.selectedBases ?? [],
      status: sessionDraft?.status ?? 0,
      sessionAnalysisRule: {
        customerAnalysisPrompt: sessionDraft?.customerAnalysisPrompt.trim() ?? '',
        employeeQaPrompt: sessionDraft?.employeeQaPrompt.trim() ?? '',
        conversationTypes: sessionDraft?.conversationTypes ?? [],
        lookbackDays: sessionDraft?.lookbackDays ?? 0,
        minimumMessages: sessionDraft?.minimumMessages ?? 0,
      },
    }),
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['ai-agents', corpId] }),
        queryClient.invalidateQueries({ queryKey: ['ai-kb', corpId] }),
      ]);
      setFeedback({ kind: 'success', text: '会话分析助手配置已保存。' });
      closeSessionEditor();
    },
    onError: (error) => setSessionError(operationError(error)),
  });

  const saveSmart = useMutation({
    mutationFn: () => api.updateAgent(Number(corpId), smartAgent?.id ?? '', {
      name: smartAgent?.name ?? '智能分析助手',
      description: smartDraft?.description.trim() ?? '',
      knowledgeBaseIds: smartDraft?.selectedBases ?? [],
      status: smartDraft?.status ?? 0,
      smartAnalysisRule: {
        objective: smartDraft?.objective.trim() ?? '',
        conversationTypes: smartDraft?.conversationTypes ?? [],
        lookbackDays: smartDraft?.lookbackDays ?? 0,
        minimumMessages: smartDraft?.minimumMessages ?? 0,
      },
    }),
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['ai-agents', corpId] }),
        queryClient.invalidateQueries({ queryKey: ['ai-kb', corpId] }),
      ]);
      setFeedback({ kind: 'success', text: '智能分析助手配置已保存。' });
      closeSmartEditor();
    },
    onError: (error) => setSmartError(operationError(error)),
  });

  const sessionValid = Boolean(
    corpId
    && sessionDraft
    && sessionAgent
    && knowledgeBasesQuery.isSuccess
    && textLength(sessionDraft.description) <= 512
    && textLength(sessionDraft.customerAnalysisPrompt.trim()) >= 2
    && textLength(sessionDraft.customerAnalysisPrompt.trim()) <= 4000
    && textLength(sessionDraft.employeeQaPrompt.trim()) >= 2
    && textLength(sessionDraft.employeeQaPrompt.trim()) <= 4000
    && sessionDraft.conversationTypes.length > 0
    && isIntegerInRange(sessionDraft.lookbackDays, 1, 30)
    && isIntegerInRange(sessionDraft.minimumMessages, 2, 50),
  );
  const smartValid = Boolean(
    corpId
    && smartDraft
    && smartAgent
    && knowledgeBasesQuery.isSuccess
    && textLength(smartDraft.description) <= 512
    && textLength(smartDraft.objective.trim()) >= 2
    && textLength(smartDraft.objective.trim()) <= 4000
    && smartDraft.conversationTypes.length > 0
    && isIntegerInRange(smartDraft.lookbackDays, 1, 30)
    && isIntegerInRange(smartDraft.minimumMessages, 2, 50),
  );

  const sessionKnowledgeNames = sessionAgent && knowledgeBasesQuery.isSuccess
    ? resolveKnowledgeBaseNames(sessionAgent.knowledgeBaseIds, knowledgeBases)
    : '—';
  const smartKnowledgeNames = smartAgent && knowledgeBasesQuery.isSuccess
    ? resolveKnowledgeBaseNames(smartAgent.knowledgeBaseIds, knowledgeBases)
    : '—';

  return (
    <Phase35PageShell title="分析助手" description="分别配置会话分析与智能分析使用的固定系统助手">
      <div className="phase35-page ai-settings-workspace ai-assistant-workspace">
        <section className="ai-assistant-intro" aria-label="AI 洞察与设置关系">
          <div>
            <strong>AI 设置管配置、AI 洞察看结果</strong>
            <p>系统只提供两个有真实消费页面的助手，不支持新增、删除或改名。</p>
          </div>
          <a href="/ai-setting/ai-knowledge-base">管理 AI 知识库</a>
        </section>
        {feedback && <p className={`ai-settings-feedback ai-settings-feedback--${feedback.kind}`} role={feedback.kind === 'error' ? 'alert' : 'status'}>{feedback.text}</p>}
        <Phase35DataState
          loading={agentsQuery.isLoading}
          error={agentsQuery.isError}
          empty={!sessionAgent || !smartAgent}
          emptyContent={<p className="phase35-empty">系统助手返回异常，请刷新后重试。</p>}
          onRetry={() => void refetchAll()}
        >
          {sessionAgent && smartAgent && (
            <div className="ai-assistant-card-grid">
              <SessionAssistantCard
                agent={sessionAgent}
                knowledgeBaseNames={sessionKnowledgeNames}
                pending={refreshPending}
                onRefresh={() => { void refetchAll(); }}
                onEdit={(event) => {
                  sessionTriggerRef.current = event.currentTarget;
                  setSessionError('');
                  setFeedback(null);
                  setSessionDraft(sessionDraftFromAgent(sessionAgent));
                }}
              />
              <SmartAssistantCard
                agent={smartAgent}
                knowledgeBaseNames={smartKnowledgeNames}
                pending={refreshPending}
                onRefresh={() => { void refetchAll(); }}
                onEdit={(event) => {
                  smartTriggerRef.current = event.currentTarget;
                  setSmartError('');
                  setFeedback(null);
                  setSmartDraft(smartDraftFromAgent(smartAgent));
                }}
              />
            </div>
          )}
        </Phase35DataState>

        <DashboardDialog
          open={Boolean(sessionDraft && sessionAgent)}
          title="配置会话分析助手"
          triggerRef={sessionTriggerRef}
          confirmDisabled={!sessionValid}
          confirmLoading={saveSession.isPending}
          onCancel={requestSessionClose}
          onConfirm={() => saveSession.mutate()}
        >
          {sessionDraft && sessionAgent && (
            <div className="ai-settings-dialog-body ai-assistant-dialog-body">
              {sessionError && <p role="alert" className="ai-settings-feedback ai-settings-feedback--error">{sessionError}</p>}
              <form onSubmit={(event) => { event.preventDefault(); if (sessionValid && !saveSession.isPending) saveSession.mutate(); }}>
                <section className="ai-assistant-editor-section">
                  <header><h3>助手基础信息</h3><p>名称固定，状态和说明只影响会话分析。</p></header>
                  <label>名称<input value={sessionAgent.name} readOnly aria-readonly="true" /></label>
                  <label>助手说明<textarea value={sessionDraft.description} onChange={(event) => setSessionDraft({ ...sessionDraft, description: event.target.value })} rows={2} /></label>
                  <label>运行状态<select aria-label="运行状态" value={sessionDraft.status} onChange={(event) => setSessionDraft({ ...sessionDraft, status: Number(event.target.value) })}><option value={1}>启用</option><option value={0}>停用</option></select><small>只影响会话分析，不影响智能分析助手。</small></label>
                </section>
                <section className="ai-assistant-editor-section">
                  <header><h3>关联知识库</h3><p>保留既有关联的停用知识库，但不能新关联其他停用库。</p></header>
                  <AssistantKnowledgeBaseFieldset
                    knowledgeBases={knowledgeBases}
                    isLoading={knowledgeBasesQuery.isLoading}
                    isError={knowledgeBasesQuery.isError}
                    selectedBaseIds={sessionDraft.selectedBases}
                    initialBaseIds={sessionAgent.knowledgeBaseIds}
                    onToggle={toggleSessionBase}
                  />
                </section>
                <section className="ai-assistant-editor-section">
                  <header><h3>会话分析范围</h3><p>配置会话分析助手覆盖的会话类型与规则窗口，不影响智能分析助手。</p></header>
                  <ConversationScopeFieldset
                    conversationTypes={sessionDraft.conversationTypes}
                    onToggle={toggleSessionConversationType}
                  />
                  <div className="ai-assistant-rule-numbers">
                    <label>回看天数<input aria-label="回看天数" type="number" min={1} max={30} value={sessionDraft.lookbackDays} onChange={(event) => setSessionDraft({ ...sessionDraft, lookbackDays: Number(event.target.value) })} /><small>1–30 天</small></label>
                    <label>最少消息数<input aria-label="最少消息数" type="number" min={2} max={50} value={sessionDraft.minimumMessages} onChange={(event) => setSessionDraft({ ...sessionDraft, minimumMessages: Number(event.target.value) })} /><small>2–50 条</small></label>
                  </div>
                </section>
                <section className="ai-assistant-editor-section">
                  <header><h3>会话分析提示词</h3><p>客户洞察与员工质检分别使用独立提示词。</p></header>
                  <label>客户分析提示词<textarea aria-label="客户分析提示词" value={sessionDraft.customerAnalysisPrompt} onChange={(event) => setSessionDraft({ ...sessionDraft, customerAnalysisPrompt: event.target.value })} rows={4} /><small>用于购买意向、流失风险、需求和行动建议。</small></label>
                  <label>员工质检提示词<textarea aria-label="员工质检提示词" value={sessionDraft.employeeQaPrompt} onChange={(event) => setSessionDraft({ ...sessionDraft, employeeQaPrompt: event.target.value })} rows={4} /><small>用于质检维度、未解决问题/异议和改进建议。</small></label>
                  <p className="ai-assistant-rule-version">当前规则版本 v{sessionAgent.sessionAnalysisRule.currentVersion}</p>
                </section>
              </form>
            </div>
          )}
        </DashboardDialog>

        <DashboardDialog
          open={Boolean(smartDraft && smartAgent)}
          title="配置智能分析助手"
          triggerRef={smartTriggerRef}
          confirmDisabled={!smartValid}
          confirmLoading={saveSmart.isPending}
          onCancel={requestSmartClose}
          onConfirm={() => saveSmart.mutate()}
        >
          {smartDraft && smartAgent && (
            <div className="ai-settings-dialog-body ai-assistant-dialog-body">
              {smartError && <p role="alert" className="ai-settings-feedback ai-settings-feedback--error">{smartError}</p>}
              <form onSubmit={(event) => { event.preventDefault(); if (smartValid && !saveSmart.isPending) saveSmart.mutate(); }}>
                <section className="ai-assistant-editor-section">
                  <header><h3>助手基础信息</h3><p>名称固定，状态和说明只影响智能分析。</p></header>
                  <label>名称<input value={smartAgent.name} readOnly aria-readonly="true" /></label>
                  <label>助手说明<textarea value={smartDraft.description} onChange={(event) => setSmartDraft({ ...smartDraft, description: event.target.value })} rows={2} /></label>
                  <label>运行状态<select aria-label="运行状态" value={smartDraft.status} onChange={(event) => setSmartDraft({ ...smartDraft, status: Number(event.target.value) })}><option value={1}>启用</option><option value={0}>停用</option></select><small>只影响智能分析，不影响会话分析助手。</small></label>
                </section>
                <section className="ai-assistant-editor-section">
                  <header><h3>关联知识库</h3><p>智能分析助手使用独立知识库集合。</p></header>
                  <AssistantKnowledgeBaseFieldset
                    knowledgeBases={knowledgeBases}
                    isLoading={knowledgeBasesQuery.isLoading}
                    isError={knowledgeBasesQuery.isError}
                    selectedBaseIds={smartDraft.selectedBases}
                    initialBaseIds={smartAgent.knowledgeBaseIds}
                    onToggle={toggleSmartBase}
                  />
                </section>
                <section className="ai-assistant-editor-section">
                  <header><h3>智能分析规则</h3><p>配置目标、会话范围和窗口约束。</p></header>
                  <label>智能分析目标<textarea aria-label="智能分析目标" value={smartDraft.objective} onChange={(event) => setSmartDraft({ ...smartDraft, objective: event.target.value })} rows={4} /></label>
                  <ConversationScopeFieldset
                    conversationTypes={smartDraft.conversationTypes}
                    onToggle={toggleConversationType}
                  />
                  <div className="ai-assistant-rule-numbers">
                    <label>回看天数<input aria-label="回看天数" type="number" min={1} max={30} value={smartDraft.lookbackDays} onChange={(event) => setSmartDraft({ ...smartDraft, lookbackDays: Number(event.target.value) })} /><small>1–30 天</small></label>
                    <label>最少消息数<input aria-label="最少消息数" type="number" min={2} max={50} value={smartDraft.minimumMessages} onChange={(event) => setSmartDraft({ ...smartDraft, minimumMessages: Number(event.target.value) })} /><small>2–50 条</small></label>
                  </div>
                  <p className="ai-assistant-rule-version">当前规则版本 v{smartAgent.smartAnalysisRule.currentVersion}</p>
                </section>
              </form>
            </div>
          )}
        </DashboardDialog>

        <DashboardDialog
          open={discardTarget !== null}
          title="放弃未保存更改？"
          danger
          confirmText="放弃更改"
          cancelText="继续编辑"
          onCancel={() => setDiscardTarget(null)}
          onConfirm={() => { if (discardTarget === 'session') closeSessionEditor(); if (discardTarget === 'smart') closeSmartEditor(); }}
        >
          <p>当前修改尚未保存，放弃后无法恢复。</p>
        </DashboardDialog>
      </div>
    </Phase35PageShell>
  );
}
