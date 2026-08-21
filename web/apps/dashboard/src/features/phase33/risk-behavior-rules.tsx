import { useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import { ConfirmAction } from '../../components/confirm-action';
import { DashboardPagination } from '../../components/dashboard-pagination';
import { PageState } from '../../components/page-state/page-state';
import { RiskWarningDrawer } from '../risk-warning/risk-warning-shell';
import type { RiskBehaviorApi, RiskRule, RiskRuleInput, RiskRulePage } from './risk-behavior-api';
import { riskBehaviorLabel, riskLevelLabel } from '../risk-warning/risk-warning-format';

function initialRule(rule?: RiskRule): RiskRuleInput {
  const strategy = rule?.strategies[0];
  const input: RiskRuleInput = {
    name: rule?.name ?? '',
    status: rule?.status === 'disabled' ? 'disabled' : 'enabled',
    subject: rule?.subject === 'customer' || rule?.subject === 'both' ? rule.subject : 'employee',
    whitelist: rule?.whitelist ?? [],
    aiInsightEnabled: rule?.aiInsightEnabled ?? false,
    strategies: [{
      ...(strategy?.id ? { id: strategy.id } : {}),
      behavior: strategy?.behavior ?? 'private_transaction',
      pattern: strategy?.pattern ?? '',
      notifyType: 'none',
      riskLevel: strategy?.riskLevel === 'high' || strategy?.riskLevel === 'low' ? strategy.riskLevel : 'medium',
    }],
  };
  return rule?.id ? { ...input, id: rule.id } : input;
}

export function RiskBehaviorRules({ api, page, loading, error, onChanged, onPageChange, onRetry, createOpen = false, onCreateClose }: { api: RiskBehaviorApi; page: RiskRulePage | undefined; loading: boolean; error: unknown; onChanged: () => void; onPageChange: (page: number) => void; onRetry: () => void; createOpen?: boolean; onCreateClose?: () => void }) {
  const [editing, setEditing] = useState<RiskRuleInput | null>(null); const [feedback, setFeedback] = useState('');
  const save = useMutation({ mutationFn: (input: RiskRuleInput) => input.id ? api.updateRule(input) : api.createRule(input), onSuccess: () => { setEditing(null); setFeedback('规则已保存'); onChanged(); }, onError: (value) => setFeedback(value instanceof Error ? value.message : '规则保存失败') });
  const toggle = useMutation({ mutationFn: (rule: RiskRule) => api.setRuleEnabled({ id: rule.id, status: rule.status === 'enabled' ? 'disabled' : 'enabled' }), onSuccess: onChanged, onError: (value) => setFeedback(value instanceof Error ? value.message : '规则状态更新失败') });
  const remove = useMutation({ mutationFn: (rule: RiskRule) => api.removeRule(rule.id), onSuccess: onChanged, onError: (value) => setFeedback(value instanceof Error ? value.message : '规则删除失败') });
  if (loading && !page) return <PageState state="loading" />;
  if (error && !page) return <PageState state="error" onRetry={onRetry} />;
  return <>{feedback ? <div className="risk-warning-inline-feedback" role="status">{feedback}</div> : null}{!page || page.items.length === 0 ? <PageState state="empty" /> : <div className="risk-warning-table-wrap"><table className="risk-warning-table"><thead><tr><th>规则名称</th><th>识别对象</th><th>策略</th><th>触发次数</th><th>状态</th><th>操作</th></tr></thead><tbody>{page.items.map((rule) => { const strategy = rule.strategies[0]; return <tr key={rule.id}><td>{rule.name || '未命名规则'}</td><td>{rule.subject === 'customer' ? '客户' : rule.subject === 'both' ? '员工与客户' : '员工'}</td><td>{strategy ? `${riskBehaviorLabel(strategy.behavior)} · ${riskLevelLabel(strategy.riskLevel)}` : '未配置策略'}</td><td>{rule.triggerCount}</td><td>{rule.status === 'enabled' ? '启用' : '停用'}</td><td className="risk-warning-row-control"><button type="button" onClick={() => setEditing(initialRule(rule))}>编辑</button>{rule.status === 'enabled' ? <ConfirmAction title={`确认停用规则“${rule.name}”？`} onConfirm={() => toggle.mutate(rule)}><button type="button">停用</button></ConfirmAction> : <ConfirmAction title={`确认启用规则“${rule.name}”？`} onConfirm={() => toggle.mutate(rule)}><button type="button">启用</button></ConfirmAction>}<ConfirmAction title={`确认删除规则“${rule.name}”？`} description="已有风险记录的规则只能停用，不能删除。" onConfirm={() => remove.mutate(rule)}><button type="button">删除</button></ConfirmAction></td></tr>; })}</tbody></table></div>}{page ? <div className="risk-warning-pagination"><DashboardPagination page={page.page} pageSize={20} total={page.total} onPageChange={onPageChange} ariaLabel="风险规则分页" /></div> : null}<RiskWarningDrawer open={editing !== null || createOpen} title={editing?.id ? '编辑风险规则' : '新增风险规则'} description="只配置当前已接入的文本匹配能力。" onClose={() => { setEditing(null); onCreateClose?.(); }}>{(editing ?? (createOpen ? initialRule() : null)) ? <RuleForm value={editing ?? initialRule()} saving={save.isPending} onChange={setEditing} onSave={() => { const value = editing ?? initialRule(); save.mutate(value); }} /> : null}</RiskWarningDrawer></>;
}

function RuleForm({ value, saving, onChange, onSave }: { value: RiskRuleInput; saving: boolean; onChange: (value: RiskRuleInput) => void; onSave: () => void }) {
  const strategy: RiskRuleInput['strategies'][number] = value.strategies[0] ?? { behavior: 'private_transaction', pattern: '', notifyType: 'none', riskLevel: 'medium' };
  const updateStrategy = (next: Partial<RiskRuleInput['strategies'][number]>) => onChange({ ...value, strategies: [{ ...strategy, ...next }] });
  return <div className="risk-warning-form"><label>规则名称<input aria-label="规则名称" value={value.name} onChange={(event) => onChange({ ...value, name: event.target.value })} /></label><label>识别对象<select aria-label="识别对象" value={value.subject} onChange={(event) => onChange({ ...value, subject: event.target.value as RiskRuleInput['subject'] })}><option value="employee">员工</option><option value="customer">客户</option><option value="both">员工与客户</option></select></label><label>风险行为<select aria-label="风险行为" value={strategy.behavior} onChange={(event) => updateStrategy({ behavior: event.target.value })}><option value="private_transaction">私下交易</option><option value="promise_rebate">承诺返利</option><option value="sensitive_word">敏感词</option></select></label><label>匹配内容<input aria-label="匹配内容" value={strategy.pattern} onChange={(event) => updateStrategy({ pattern: event.target.value })} /></label><label>风险等级<select aria-label="风险等级" value={strategy.riskLevel} onChange={(event) => updateStrategy({ riskLevel: event.target.value as RiskRuleInput['strategies'][number]['riskLevel'] })}><option value="low">低风险</option><option value="medium">中风险</option><option value="high">高风险</option></select></label><p className="risk-warning-form-note">规则仅按匹配内容执行，保存后可在风险记录中复核。</p><button type="button" className="risk-warning-primary-button" disabled={saving || !value.name.trim() || !strategy.pattern.trim()} onClick={onSave}>保存规则</button></div>;
}
