import { useEffect, useState } from 'react';
import type { ConversationExportTaskInput, ConversationExportType } from './conversation-global-api';
import { exportTypeLabel } from './conversation-export-stepper';

function dateValue(date: Date): string {
  const parts = new Intl.DateTimeFormat('en-CA', {
    timeZone: 'Asia/Shanghai', year: 'numeric', month: '2-digit', day: '2-digit',
  }).formatToParts(date);
  const value = Object.fromEntries(parts.map((part) => [part.type, part.value]));
  return `${value.year}-${value.month}-${value.day}`;
}
function toShanghaiISO(date: string, end: boolean): string { return `${date}T${end ? '23:59:59' : '00:00:00'}+08:00`; }
export function toExportEndISO(date: string, now = new Date()): string {
  return date === dateValue(now) ? now.toISOString() : toShanghaiISO(date, true);
}

export function ConversationExportDialog({
  type, selectedIds, submitting, submitError, onClearError, onClose, onSubmit,
}: {
  type: ConversationExportType;
  selectedIds: ReadonlySet<number>;
  submitting: boolean;
  submitError: string;
  onClearError: () => void;
  onClose: () => void;
  onSubmit: (input: ConversationExportTaskInput) => void;
}) {
  const today = new Date();
  const [startAt, setStartAt] = useState(dateValue(new Date(today.getTime() - 6 * 24 * 60 * 60 * 1000)));
  const [endAt, setEndAt] = useState(dateValue(today));
  const [fileMode, setFileMode] = useState<'split' | 'merge'>('split');
  const [scopes, setScopes] = useState<string[]>(['customer_direct', 'external_group']);
  const [error, setError] = useState('');
  useEffect(() => { const handler = (event: KeyboardEvent) => { if (event.key === 'Escape') onClose(); }; window.addEventListener('keydown', handler); return () => window.removeEventListener('keydown', handler); }, [onClose]);
  function clearErrors() { setError(''); onClearError(); }
  function submit() {
    clearErrors();
    if (!startAt || !endAt || endAt < startAt) { setError('请填写正确的日期范围'); return; }
    const start = new Date(`${startAt}T00:00:00+08:00`); const end = new Date(toExportEndISO(endAt));
    if ((end.getTime() - start.getTime()) > 2 * 366 * 24 * 60 * 60 * 1000) { setError('日期跨度不能超过 2 年'); return; }
    if (scopes.length === 0) { setError('至少选择一种会话范围'); return; }
    onSubmit({ exportType: type, objectIds: [...selectedIds], conversationScopes: scopes, employeeIds: [], startAt: toShanghaiISO(startAt, false), endAt: toExportEndISO(endAt), fileMode, format: 'zip' });
  }
  return <div className="conversation-export-modal" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget) onClose(); }}><section className="conversation-export-dialog" role="dialog" aria-modal="true" aria-labelledby="conversation-export-dialog-title">
    <header><div><span className="conversation-export-eyebrow">导出设置</span><h2 id="conversation-export-dialog-title">确认导出{exportTypeLabel(type)}数据</h2></div><button type="button" aria-label="关闭" onClick={onClose}>×</button></header>
    <div className="conversation-export-dialog-body"><div className="conversation-export-dialog-summary"><strong>{selectedIds.size}</strong><span>个{exportTypeLabel(type)}对象已选择</span></div><label>消息日期范围<div className="conversation-export-date-row"><input aria-label="开始日期" type="date" value={startAt} onChange={(event) => { setStartAt(event.target.value); clearErrors(); }} /><span>至</span><input aria-label="结束日期" type="date" value={endAt} onChange={(event) => { setEndAt(event.target.value); clearErrors(); }} /></div></label><fieldset><legend>会话范围</legend><label><input type="checkbox" checked={scopes.includes('customer_direct')} onChange={(event) => { clearErrors(); setScopes((current) => event.target.checked ? [...current, 'customer_direct'] : current.filter((item) => item !== 'customer_direct')); }} />客户单聊</label><label><input type="checkbox" checked={scopes.includes('external_group')} onChange={(event) => { clearErrors(); setScopes((current) => event.target.checked ? [...current, 'external_group'] : current.filter((item) => item !== 'external_group')); }} />外部客户群</label></fieldset><fieldset><legend>文件组织</legend><label><input type="radio" name="export-file-mode" checked={fileMode === 'split'} onChange={() => { setFileMode('split'); clearErrors(); }} />按对象拆分 CSV</label><label><input type="radio" name="export-file-mode" checked={fileMode === 'merge'} onChange={() => { setFileMode('merge'); clearErrors(); }} />合并为一个 CSV</label></fieldset><p className="conversation-export-dialog-tip">仅导出系统已有归档消息，不包含媒体二进制；单任务最多 100,000 条消息，文件保留 7 天。</p>{(error || submitError) && <div className="conversation-export-form-error" role="alert">{error || submitError}</div>}</div>
    <footer><button className="conversation-export-secondary" type="button" onClick={onClose}>取消</button><button className="conversation-export-primary" type="button" disabled={submitting} onClick={submit}>{submitting ? '创建中…' : '确认创建任务'}</button></footer>
  </section></div>;
}
