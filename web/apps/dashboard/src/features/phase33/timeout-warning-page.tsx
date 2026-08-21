import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useMemo, useState } from 'react';
import { useSearchParams } from 'react-router';

import { useDashboardAccess } from '../../app/access-context';
import { pageStateForError, PageState } from '../../components/page-state/page-state';
import { ConfirmAction } from '../../components/confirm-action';
import { RiskWarningDrawer, RiskWarningPageHeader, RiskWarningQueryBar, RiskWarningShell, RiskWarningTabs } from '../risk-warning/risk-warning-shell';
import { createRiskWarningApi, type TimeoutFilter, type TimeoutRecord, type TimeoutRule } from '../risk-warning/risk-warning-api';
import type { BusinessWorkbenchApi } from '../business-workbench/business-workbench-page';

const pageSize = 20;
const riskLabels: Record<string, string> = { low: '低风险', medium: '中风险', high: '高风险' };
const statusLabels: Record<string, string> = { pending: '待处置', confirmed: '已确认', ignored: '已忽略', closed: '已关闭' };

function formatDate(value: string): string {
  if (!value) return '--';
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value.replace('T', ' ').replace(/Z$/, '') : date.toLocaleString('zh-CN', { hour12: false }).replace(/\//g, '-');
}

function duration(seconds: number): string {
  if (seconds < 60) return `${seconds} 秒`;
  const minutes = Math.floor(seconds / 60);
  const rest = seconds % 60;
  return rest ? `${minutes} 分 ${rest} 秒` : `${minutes} 分钟`;
}

function riskPill(value: string) {
  const tone = value === 'high' ? 'risk-warning-pill-high' : value === 'medium' ? 'risk-warning-pill-medium' : value === 'low' ? 'risk-warning-pill-low' : 'risk-warning-pill-neutral';
  return <span className={`risk-warning-pill ${tone}`}>{(riskLabels[value] ?? value) || '--'}</span>;
}

function initials(name: string): string { return name.trim().slice(0, 1) || '客'; }

function Person({ name, avatar }: { name: string; avatar?: string }) {
  return <span className="risk-warning-person">{avatar ? <img className="risk-warning-avatar" src={avatar} alt="" /> : <span className="risk-warning-avatar risk-warning-avatar-fallback" aria-hidden="true">{initials(name)}</span>}<span>{name}</span></span>;
}

type Strategy = { timeoutMinutes: number; riskLevel: string; notifyType: string };

export function TimeoutWarningPage({ api: workbenchApi }: { api: BusinessWorkbenchApi }) {
  const access = useDashboardAccess();
  const api = useMemo(() => createRiskWarningApi(workbenchApi), [workbenchApi]);
  const queryClient = useQueryClient();
  const [params, setParams] = useSearchParams();
  const tab = params.get('tab') === 'rules' ? 'rules' : params.get('tab') === 'settings' ? 'settings' : 'records';
  const [draft, setDraft] = useState({ customer: params.get('customer') ?? '', riskLevel: params.get('riskLevel') ?? '', conversationType: params.get('conversationType') ?? '', auditStatus: params.get('auditStatus') ?? '' });
  const [ruleNameFilter, setRuleNameFilter] = useState(params.get('ruleName') ?? '');
  const [filters, setFilters] = useState<TimeoutFilter>({ customer: params.get('customer') ?? undefined, riskLevel: params.get('riskLevel') ?? undefined, conversationType: params.get('conversationType') ?? undefined, auditStatus: params.get('auditStatus') ?? undefined, page: Number(params.get('page') ?? 1) || 1, perPage: pageSize });
  const [selected, setSelected] = useState<TimeoutRecord | null>(null);
  const [checked, setChecked] = useState<Set<number>>(new Set());
  const [editing, setEditing] = useState<TimeoutRule | null>(null);
  const [ruleName, setRuleName] = useState('');
  const [scopeSingle, setScopeSingle] = useState(true);
  const [scopeGroup, setScopeGroup] = useState(false);
  const [strategies, setStrategies] = useState<Strategy[]>([{ timeoutMinutes: 3, riskLevel: 'low', notifyType: 'none' }]);
  const [closingPhrases, setClosingPhrases] = useState<string[]>([]);
  const [newPhrase, setNewPhrase] = useState('');
  const [whitelist, setWhitelist] = useState<string[]>([]);
  const [loadedSettings, setLoadedSettings] = useState(false);

  const records = useQuery({ queryKey: ['timeout-records', access.corp.id, filters], queryFn: () => api.timeoutRecords(filters), enabled: access.corp.authorized && tab === 'records', retry: false });
  const rules = useQuery({ queryKey: ['timeout-rules', access.corp.id, ruleNameFilter], queryFn: () => api.timeoutRules({ name: ruleNameFilter.trim() || undefined, page: 1, perPage: pageSize }), enabled: access.corp.authorized && tab === 'rules', retry: false });
  const settings = useQuery({ queryKey: ['timeout-settings', access.corp.id], queryFn: api.timeoutSettings, enabled: access.corp.authorized && tab === 'settings', retry: false });
  const audit = useMutation({ mutationFn: (action: string) => api.write('/timeout-warning/records/audit', { ids: [...checked], action, remark: '' }), onSuccess: async () => { setChecked(new Set()); await queryClient.invalidateQueries({ queryKey: ['timeout-records', access.corp.id] }); } });
  const toggleRule = useMutation({ mutationFn: (rule: TimeoutRule) => api.write('/timeout-warning/rules/status', { id: rule.id, status: rule.status === 'enabled' ? 'disabled' : 'enabled' }, 'PUT'), onSuccess: () => void queryClient.invalidateQueries({ queryKey: ['timeout-rules', access.corp.id] }) });
  const deleteRule = useMutation({ mutationFn: (rule: TimeoutRule) => api.write(`/timeout-warning/rules?id=${rule.id}`, {}, 'DELETE'), onSuccess: () => void queryClient.invalidateQueries({ queryKey: ['timeout-rules', access.corp.id] }) });
  const saveRule = useMutation({ mutationFn: () => api.write(`/timeout-warning/rules${editing?.id ? '' : ''}`, { ...(editing?.id ? { id: editing.id } : {}), name: ruleName.trim(), status: editing?.status || 'enabled', monitorTarget: 'all', monitorTargetIds: [], conversationScopes: [ ...(scopeSingle ? ['single'] : []), ...(scopeGroup ? ['group'] : []) ], aiInsightEnabled: false, strategies: strategies.map((item, index) => ({ ...item, sortOrder: index })), quietPeriods: [], notifyTargets: [] }, editing?.id ? 'PUT' : 'POST'), onSuccess: async () => { setEditing(null); await queryClient.invalidateQueries({ queryKey: ['timeout-rules', access.corp.id] }); } });
  const saveSettings = useMutation({ mutationFn: () => api.write('/timeout-warning/settings', { closingPhraseGroups: closingPhrases.map((value) => [value]), whitelistMessageTypes: whitelist }, 'PUT'), onSuccess: async () => { setLoadedSettings(false); await queryClient.invalidateQueries({ queryKey: ['timeout-settings', access.corp.id] }); } });

  const updateURL = (next: { page?: number; tab?: string } & Partial<typeof draft>) => { const value = new URLSearchParams(params); for (const key of ['customer', 'riskLevel', 'conversationType', 'auditStatus', 'page', 'tab']) value.delete(key); for (const [key, raw] of Object.entries(next)) if (raw !== undefined && raw !== '') value.set(key, String(raw)); setParams(value); };
  const submit = () => { const next: TimeoutFilter = { customer: draft.customer.trim() || undefined, riskLevel: draft.riskLevel || undefined, conversationType: draft.conversationType || undefined, auditStatus: draft.auditStatus || undefined, page: 1, perPage: pageSize }; setFilters(next); updateURL({ ...draft, page: 1 }); setChecked(new Set()); setSelected(null); };
  const reset = () => { const next = { customer: '', riskLevel: '', conversationType: '', auditStatus: '' }; setDraft(next); setFilters({ page: 1, perPage: pageSize }); updateURL({ page: 1 }); setChecked(new Set()); setSelected(null); };
  const refresh = () => { void queryClient.invalidateQueries({ queryKey: ['timeout-records', access.corp.id] }); void queryClient.invalidateQueries({ queryKey: ['timeout-rules', access.corp.id] }); void queryClient.invalidateQueries({ queryKey: ['timeout-settings', access.corp.id] }); };
  const openRule = (rule?: TimeoutRule) => {
    setEditing(rule ?? { id: 0, name: '', status: 'enabled', monitorTarget: 'all', conversationScopes: ['single'], triggerCount: 0, strategies: [], createdAt: '' });
    setRuleName(rule?.name ?? '');
    setScopeSingle(rule?.conversationScopes.includes('single') ?? true);
    setScopeGroup(rule?.conversationScopes.includes('group') ?? false);
    setStrategies(rule?.strategies.length
      ? rule.strategies.map((strategy) => ({ timeoutMinutes: Number(strategy.timeoutMinutes) || 3, riskLevel: strategy.riskLevel || 'low', notifyType: strategy.notifyType || 'none' }))
      : [{ timeoutMinutes: 3, riskLevel: 'low', notifyType: 'none' }]);
  };
  const page = records.data;
  const rulePage = rules.data;
  const settingsData = settings.data;
  useEffect(() => {
    if (!settingsData || loadedSettings) return;
    const groups = Array.isArray(settingsData.closingPhraseGroups) ? settingsData.closingPhraseGroups.flat().filter((value): value is string => typeof value === 'string') : [];
    setClosingPhrases(groups);
    setWhitelist(Array.isArray(settingsData.whitelistMessageTypes) ? settingsData.whitelistMessageTypes.filter((value): value is string => typeof value === 'string') : []);
    setLoadedSettings(true);
  }, [loadedSettings, settingsData]);

  return <RiskWarningShell className="timeout-warning-page">
    <RiskWarningPageHeader title="超时预警" meta="按真实归档消息的等待时长识别风险，并在同一工作台完成处置。" />
    <RiskWarningTabs active={tab} tabs={[{ id: 'records', label: '超时记录' }, { id: 'rules', label: '规则配置' }, { id: 'settings', label: '高级设置' }]} onChange={(next) => { const value = new URLSearchParams(params); value.set('tab', next); value.delete('recordId'); setParams(value); setSelected(null); }} />
    {tab === 'records' ? <>
      <RiskWarningQueryBar fetching={records.isFetching} onQuery={submit} onReset={reset} onRefresh={refresh}><label>客户<input aria-label="客户" value={draft.customer} placeholder="客户名称或 ID" onChange={(event) => setDraft((current) => ({ ...current, customer: event.target.value }))} /></label><label>风险等级<select aria-label="风险等级" value={draft.riskLevel} onChange={(event) => setDraft((current) => ({ ...current, riskLevel: event.target.value }))}><option value="">全部</option><option value="low">低风险</option><option value="medium">中风险</option><option value="high">高风险</option></select></label><label>会话类型<select aria-label="会话类型" value={draft.conversationType} onChange={(event) => setDraft((current) => ({ ...current, conversationType: event.target.value }))}><option value="">全部</option><option value="single">单聊</option><option value="group">群聊</option></select></label><label>处置状态<select aria-label="处置状态" value={draft.auditStatus} onChange={(event) => setDraft((current) => ({ ...current, auditStatus: event.target.value }))}><option value="">全部</option><option value="pending">待处置</option><option value="confirmed">已确认</option><option value="ignored">已忽略</option><option value="closed">已关闭</option></select></label></RiskWarningQueryBar>
      <section className="risk-warning-results"><div className="risk-warning-results-header"><div><h2>超时记录</h2><p>只展示归档消息实际命中的超时规则记录。</p></div><span className="risk-warning-muted">共 {page?.total ?? 0} 条</span></div>{records.isPending ? <PageState state="loading" /> : records.isError ? <PageState state={pageStateForError(records.error)} onRetry={() => void records.refetch()} /> : !page || page.items.length === 0 ? <PageState state="empty" title="暂无超时记录" description="当前筛选条件下没有待复核的真实记录。" /> : <><div className="risk-warning-batch-bar">{checked.size ? <><span>已选 {checked.size} 条</span><ConfirmAction title={`确认将 ${checked.size} 条记录标记为已确认？`} onConfirm={() => audit.mutate('confirmed')}><button type="button">确认超时</button></ConfirmAction><ConfirmAction title={`确认忽略 ${checked.size} 条记录？`} onConfirm={() => audit.mutate('ignored')}><button type="button">忽略记录</button></ConfirmAction><ConfirmAction title={`确认关闭 ${checked.size} 条记录？`} onConfirm={() => audit.mutate('closed')}><button type="button">关闭记录</button></ConfirmAction></> : <span className="risk-warning-muted">选择记录后可批量处置</span>}</div><div className="risk-warning-table-wrap"><table className="risk-warning-table timeout-record-table"><thead><tr><th><input type="checkbox" aria-label="全选当前页" checked={page.items.length > 0 && page.items.every((row) => checked.has(row.id))} onChange={(event) => setChecked(event.target.checked ? new Set(page.items.map((row) => row.id)) : new Set())} /></th><th>超时时长</th><th>触发消息</th><th>相关人</th><th>会话类型</th><th>风险等级</th><th>规则</th><th>处置状态</th><th>记录时间</th><th>操作</th></tr></thead><tbody>{page.items.map((row) => <tr key={row.id}><td className="risk-warning-row-control"><input type="checkbox" aria-label={`选择记录 ${row.id}`} checked={checked.has(row.id)} onChange={(event) => setChecked((current) => { const next = new Set(current); if (event.target.checked) next.add(row.id); else next.delete(row.id); return next; })} /></td><td>{duration(row.timeoutSeconds)}</td><td><span className="risk-warning-inline-preview" title={row.triggerMessage}>{row.triggerMessage || `[${row.messageType || '消息'}]`}</span></td><td><Person name={row.customerName} avatar={row.customerAvatar} /><small className="risk-warning-subline">{row.employeeName}</small></td><td>{row.conversationType === 'group' ? '群聊' : '单聊'}</td><td>{riskPill(row.riskLevel)}</td><td>{row.ruleName}</td><td>{statusLabels[row.auditStatus] ?? row.auditStatus}</td><td>{formatDate(row.occurredAt)}</td><td className="risk-warning-row-control"><button type="button" onClick={() => { setSelected(row); const value = new URLSearchParams(params); value.set('recordId', String(row.id)); setParams(value); }}>查看详情</button></td></tr>)}</tbody></table></div><div className="risk-warning-pagination"><button type="button" disabled={(filters.page ?? 1) <= 1} onClick={() => { const pageNumber = (filters.page ?? 1) - 1; setFilters((current) => ({ ...current, page: pageNumber })); updateURL({ ...draft, page: pageNumber }); }}>上一页</button><span>第 {filters.page ?? 1} 页</span><button type="button" disabled={page.items.length < pageSize} onClick={() => { const pageNumber = (filters.page ?? 1) + 1; setFilters((current) => ({ ...current, page: pageNumber })); updateURL({ ...draft, page: pageNumber }); }}>下一页</button></div></>}</section>
      <RiskWarningDrawer open={selected !== null} title="超时记录详情" {...(selected ? { description: `${selected.customerName} · ${selected.ruleName}` } : {})} onClose={() => { setSelected(null); const value = new URLSearchParams(params); value.delete('recordId'); setParams(value); }}>{selected ? <dl className="risk-warning-detail-grid"><div><dt>触发消息</dt><dd>{selected.triggerMessage || `[${selected.messageType || '消息'}]`}</dd></div><div><dt>客户</dt><dd><Person name={selected.customerName} avatar={selected.customerAvatar} /></dd></div><div><dt>责任员工</dt><dd>{selected.employeeName}</dd></div><div><dt>会话类型</dt><dd>{selected.conversationType === 'group' ? '群聊' : '单聊'}</dd></div><div><dt>超时时长</dt><dd>{duration(selected.timeoutSeconds)}</dd></div><div><dt>规则</dt><dd>{selected.ruleName}</dd></div><div><dt>风险等级</dt><dd>{riskPill(selected.riskLevel)}</dd></div><div><dt>处置状态</dt><dd>{statusLabels[selected.auditStatus] ?? selected.auditStatus}</dd></div><div><dt>记录时间</dt><dd>{formatDate(selected.occurredAt)}</dd></div></dl> : null}</RiskWarningDrawer>
    </> : tab === 'rules' ? <>
      <RiskWarningQueryBar fetching={rules.isFetching} onQuery={() => void rules.refetch()} onReset={() => { setRuleNameFilter(''); void rules.refetch(); }} onRefresh={refresh}><label>规则名称<input aria-label="规则名称" value={ruleNameFilter} onChange={(event) => setRuleNameFilter(event.target.value)} placeholder="按名称筛选" /></label></RiskWarningQueryBar>
      <section className="risk-warning-results"><div className="risk-warning-results-header"><div><h2>规则配置</h2><p>管理监听范围和分级超时策略。</p></div><button type="button" className="risk-warning-primary-button" onClick={() => openRule()}>新增规则</button></div>{rules.isPending ? <PageState state="loading" /> : rules.isError ? <PageState state={pageStateForError(rules.error)} onRetry={() => void rules.refetch()} /> : !rulePage || rulePage.items.length === 0 ? <PageState state="empty" title="暂无超时规则" description="新增规则后，归档消息才会进入超时评估。" /> : <div className="risk-warning-table-wrap"><table className="risk-warning-table timeout-rule-table"><thead><tr><th>规则名称</th><th>监听对象</th><th>会话范围</th><th>超时策略</th><th>触发次数</th><th>状态</th><th>创建时间</th><th>操作</th></tr></thead><tbody>{rulePage.items.map((rule) => <tr key={rule.id}><td>{rule.name}</td><td>{rule.monitorTarget === 'all' ? '全部员工' : rule.monitorTarget === 'employee' ? '指定员工' : '指定部门'}</td><td>{rule.conversationScopes.map((value) => value === 'group' ? '群聊' : '单聊').join('、') || '--'}</td><td>{rule.strategies.map((value) => `${value.timeoutMinutes} 分钟 · ${riskLabels[value.riskLevel] ?? value.riskLevel}`).join('、') || '--'}</td><td>{rule.triggerCount}</td><td>{rule.status === 'enabled' ? '启用' : '停用'}</td><td>{formatDate(rule.createdAt)}</td><td className="risk-warning-row-control"><button type="button" onClick={() => openRule(rule)}>编辑</button><ConfirmAction title={`确认${rule.status === 'enabled' ? '停用' : '启用'}规则“${rule.name}”？`} onConfirm={() => toggleRule.mutate(rule)}><button type="button">{rule.status === 'enabled' ? '停用' : '启用'}</button></ConfirmAction><ConfirmAction title={`确认删除规则“${rule.name}”？`} onConfirm={() => deleteRule.mutate(rule)}><button type="button">删除</button></ConfirmAction></td></tr>)}</tbody></table></div>}</section>
      <RiskWarningDrawer open={editing !== null} title={editing?.id ? '编辑超时规则' : '新增超时规则'} description="只配置当前已接入的归档消息超时能力。" onClose={() => setEditing(null)}><div className="risk-warning-form"><label>规则名称<input aria-label="规则名称" value={ruleName} onChange={(event) => setRuleName(event.target.value)} placeholder="例如：客户消息 10 分钟未回复" /></label><fieldset><legend>会话范围</legend><label><input type="checkbox" checked={scopeSingle} onChange={(event) => setScopeSingle(event.target.checked)} />单聊</label><label><input type="checkbox" checked={scopeGroup} onChange={(event) => setScopeGroup(event.target.checked)} />群聊</label></fieldset><div className="risk-warning-form-section"><h3>超时策略</h3>{strategies.map((strategy, index) => <div className="risk-warning-strategy-row" key={index}><label>分钟<input aria-label={`超时时长 ${index + 1}`} type="number" min={3} max={180} value={strategy.timeoutMinutes} onChange={(event) => setStrategies((current) => current.map((item, itemIndex) => itemIndex === index ? { ...item, timeoutMinutes: Number(event.target.value) } : item))} /></label><label>风险<select aria-label={`风险等级 ${index + 1}`} value={strategy.riskLevel} onChange={(event) => setStrategies((current) => current.map((item, itemIndex) => itemIndex === index ? { ...item, riskLevel: event.target.value } : item))}><option value="low">低风险</option><option value="medium">中风险</option><option value="high">高风险</option></select></label>{strategies.length > 1 ? <button type="button" onClick={() => setStrategies((current) => current.filter((_, itemIndex) => itemIndex !== index))}>删除</button> : null}</div>)}<button type="button" disabled={strategies.length >= 5} onClick={() => setStrategies((current) => [...current, { timeoutMinutes: 10, riskLevel: 'medium', notifyType: 'none' }])}>添加策略</button></div><div className="risk-warning-drawer-actions"><button type="button" className="risk-warning-secondary-button" onClick={() => setEditing(null)}>取消</button><button type="button" className="risk-warning-primary-button" disabled={!ruleName.trim() || (!scopeSingle && !scopeGroup) || saveRule.isPending} onClick={() => saveRule.mutate()}>保存规则</button></div></div></RiskWarningDrawer>
    </> : <>
      <section className="risk-warning-results timeout-settings-panel"><div className="risk-warning-results-header"><div><h2>高级设置</h2><p>配置结束语和白名单消息类型，避免真实业务消息被误判为超时。</p></div><button type="button" className="risk-warning-secondary-button" onClick={() => { setLoadedSettings(false); void settings.refetch(); }}>重新载入</button></div>{settings.isPending ? <PageState state="loading" /> : settings.isError ? <PageState state={pageStateForError(settings.error)} onRetry={() => void settings.refetch()} /> : <div className="timeout-settings-grid"><div className="risk-warning-form"><h3>结束语</h3><p className="risk-warning-form-note">结束语命中后，客户消息不再生成超时记录。</p><div className="risk-warning-tag-list">{closingPhrases.map((value) => <span className="risk-warning-tag" key={value}>{value}<button type="button" aria-label={`删除结束语 ${value}`} onClick={() => setClosingPhrases((current) => current.filter((item) => item !== value))}>×</button></span>)}</div><div className="risk-warning-inline-input"><input aria-label="新增结束语" value={newPhrase} placeholder="输入结束语后添加" onChange={(event) => setNewPhrase(event.target.value)} /><button type="button" disabled={!newPhrase.trim()} onClick={() => { const phrase = newPhrase.trim(); if (phrase) setClosingPhrases((current) => [...new Set([...current, phrase])]); setNewPhrase(''); }}>添加</button></div></div><div className="risk-warning-form"><h3>白名单消息类型</h3><p className="risk-warning-form-note">选择当前归档链路可以识别的消息类型。</p><div className="risk-warning-check-list">{([['image', '图片'], ['voice', '语音'], ['video', '视频'], ['file', '文件'], ['link', '链接']] as const).map(([value, label]) => <label key={value}><input type="checkbox" checked={whitelist.includes(value)} onChange={(event) => setWhitelist((current) => event.target.checked ? [...current, value] : current.filter((item) => item !== value))} />{label}</label>)}</div></div><div className="risk-warning-drawer-actions"><button type="button" className="risk-warning-primary-button" disabled={saveSettings.isPending} onClick={() => saveSettings.mutate()}>保存设置</button></div></div>}</section>
    </>}
  </RiskWarningShell>;
}
