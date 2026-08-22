import { MobileState } from '@mochat/mobile-foundation';
import { useEffect, useMemo, useState, type FormEvent, type ReactNode } from 'react';
import { useHref, useLocation } from 'react-router';

import type { SidebarRequest } from '../../app/sidebar-router';
import { SidebarPageShell } from '../../ui/sidebar-page-shell';
import type { WeComBridge } from '../../wecom/wecom-bridge';
import {
  loadEmployeeContacts,
  loadEmployeeTasks,
  loadWorkbenchSummary,
  type ContactListPage,
  type EmployeeTaskKind,
  type TaskListPage,
  type WorkbenchSummary,
} from './workbench-api';
import { WorkbenchIcon, type WorkbenchIconKind } from './workbench-icons';

type LoadState<T> = { kind: 'loading' } | { kind: 'ready'; value: T } | { kind: 'error'; message: string };
type WorkbenchPageProps = { request: SidebarRequest; bridge: WeComBridge };
type WorkbenchTab = 'customers' | 'conversations' | 'profile';

function errorMessage(error: unknown): string {
  return error instanceof Error && error.message.trim() !== '' ? error.message : '请检查网络后重试。';
}

function workspaceTab(search: string): WorkbenchTab {
  const tab = new URLSearchParams(search).get('tab');
  return tab === 'conversations' || tab === 'profile' ? tab : 'customers';
}

function keepContext(search: string): URLSearchParams {
  const source = new URLSearchParams(search);
  const target = new URLSearchParams();
  for (const key of ['agentId', 'wxExternalUserid', 'contactId', 'contactSopId', 'roomSopId', 'batchId']) {
    const value = source.get(key)?.trim();
    if (value) target.set(key, value);
  }
  return target;
}

function contextTarget(path: string, search: string, entries: Record<string, string> = {}): string {
  const params = keepContext(search);
  for (const [key, value] of Object.entries(entries)) params.set(key, value);
  const query = params.toString();
  return `${path}${query ? `?${query}` : ''}`;
}

function WorkbenchLink({ to, className, children, ariaLabel }: { to: string; className: string; children: ReactNode; ariaLabel?: string }) {
  const href = useHref(to);
  return <a {...(ariaLabel ? { 'aria-label': ariaLabel } : {})} className={className} href={href}>{children}</a>;
}

function FeatureLink({ to, icon, label, detail, tone = 'blue' }: {
  to: string | null; icon: WorkbenchIconKind; label: string; detail?: string;
  tone?: 'blue' | 'cyan' | 'purple' | 'orange' | 'green';
}) {
  const content = <><span className={`workbench-feature__icon workbench-feature__icon--${tone}`}><WorkbenchIcon kind={icon} /></span><span><strong>{label}</strong>{detail ? <small>{detail}</small> : null}</span></>;
  return to === null ? <div aria-disabled="true" className="workbench-feature workbench-feature--disabled">{content}</div> : (
    <WorkbenchLink className="workbench-feature" to={to}>
      {content}
    </WorkbenchLink>
  );
}

function Illustration({ kind }: { kind: 'customer' | 'conversation' }) {
  return (
    <svg aria-hidden="true" className="workbench-illustration" viewBox="0 0 180 116">
      <path className="workbench-illustration__cloud" d="M16 93c0-13 10-23 23-23 4-18 20-30 39-30 20 0 37 13 41 32 5-4 11-6 18-6 16 0 28 12 28 27Z" />
      {kind === 'customer' ? <><circle cx="76" cy="47" r="17" /><path d="M46 99c4-25 14-37 30-37s27 12 31 37" /><rect x="110" y="45" width="38" height="46" rx="7" /><path d="M119 59h20M119 70h15" /><circle cx="136" cy="38" r="11" /><path d="M136 32v12M130 38h12" /></> : <><rect x="31" y="31" width="91" height="57" rx="13" /><path d="m53 88-13 14 3-18M51 50h49M51 65h35" /><circle cx="136" cy="69" r="25" /><path d="m125 69 8 8 15-17" /></>}
    </svg>
  );
}

function CustomerWorkspace({ summary, search }: { summary: WorkbenchSummary; search: string }) {
  const hasCurrentCustomer = Boolean(new URLSearchParams(search).get('wxExternalUserid')?.trim());
  const metrics = [
    ['客户总数', summary.customers.total], ['今日新增', summary.customers.addedToday],
    ['已打标签', summary.customers.taggedTotal], ['负责群聊', summary.customers.ownedRoomTotal],
  ] as const;
  return (
    <div className="workbench-page">
      <section className="workbench-hero workbench-hero--customer">
        <div><p>客户经营工作台</p><h2>每一次跟进，都更有准备</h2><span>真实客户与任务数据实时汇总</span></div><Illustration kind="customer" />
      </section>
      <section aria-label="客户统计" className="workbench-metrics">{metrics.map(([label, value]) => <div key={label}><strong>{value}</strong><span>{label}</span></div>)}</section>
      <section className="workbench-section"><header><h2>常用功能</h2></header><div className="workbench-features workbench-features--five">
        <FeatureLink icon="contacts" label="通讯录" to={contextTarget('/', search, { tab: 'customers', view: 'contacts' })} />
        <FeatureLink icon="message" label="当前客户" tone="cyan" to={hasCurrentCustomer ? contextTarget('/contact', search) : null} />
        <FeatureLink icon="batch" label="批量加好友" tone="purple" to={contextTarget('/', search, { tab: 'conversations', view: 'batchAdd' })} />
        <FeatureLink icon="material" label="素材库" tone="orange" to={contextTarget('/medium', search)} />
        <FeatureLink icon="groups" label="群任务" tone="green" to={contextTarget('/', search, { tab: 'conversations', view: 'roomSop' })} />
      </div></section>
      <section className="workbench-section workbench-tasks-overview"><header><h2>待办任务</h2><span>{summary.tasks.contactSopPending + summary.tasks.roomSopPending + summary.tasks.batchAddPending} 项</span></header><div className="workbench-task-tabs">
        <FeatureLink detail={`${summary.tasks.contactSopPending} 项待处理`} icon="sop" label="个人客户 SOP" to={contextTarget('/', search, { tab: 'conversations', view: 'contactSop' })} />
        <FeatureLink detail={`${summary.tasks.roomSopPending} 项待处理`} icon="groups" label="客户群 SOP" tone="green" to={contextTarget('/', search, { tab: 'conversations', view: 'roomSop' })} />
        <FeatureLink detail={`${summary.tasks.batchAddPending} 项待处理`} icon="batch" label="批量加好友" tone="purple" to={contextTarget('/', search, { tab: 'conversations', view: 'batchAdd' })} />
      </div></section>
    </div>
  );
}

const taskKinds: Array<{ kind: EmployeeTaskKind; label: string; icon: WorkbenchIconKind; tone: 'blue' | 'green' | 'purple' }> = [
  { kind: 'contactSop', label: '个人客户 SOP', icon: 'sop', tone: 'blue' },
  { kind: 'roomSop', label: '客户群 SOP', icon: 'groups', tone: 'green' },
  { kind: 'batchAdd', label: '批量加好友', icon: 'batch', tone: 'purple' },
];

function taskDetailTarget(task: TaskListPage['items'][number], search: string): string {
  if (task.kind === 'contactSop') return contextTarget('/contactSop', search, { id: String(task.id) });
  if (task.kind === 'roomSop') return contextTarget('/roomSop', search, { id: String(task.id) });
  return contextTarget('/contactBatchAdd', search, { batchId: String(task.id) });
}

function TaskList({ state, search, retry }: { state: LoadState<TaskListPage>; search: string; retry: () => void }) {
  if (state.kind === 'loading') return <MobileState kind="loading" title="正在加载任务" description="正在读取你的任务数据。" />;
  if (state.kind === 'error') return <div className="workbench-inline-state"><MobileState kind="error" title="任务加载失败" description={state.message} /><button onClick={retry} type="button">重新加载</button></div>;
  if (state.value.items.length === 0) return <MobileState kind="empty" title="暂无任务" description="当前筛选下没有需要处理的任务。" />;
  return <ul className="workbench-list">{state.value.items.map((task) => <li key={`${task.kind}-${task.id}`}><div><strong>{task.title}</strong><span>{task.subjectName || '未命名对象'}</span><time>{task.scheduledAt || '未设置执行时间'}</time></div><WorkbenchLink ariaLabel={`打开任务：${task.title}`} className="workbench-list__action" to={taskDetailTarget(task, search)}>打开任务<WorkbenchIcon kind="chevron" /></WorkbenchLink></li>)}</ul>;
}

function ConversationWorkspace({ summary, search, request }: { summary: WorkbenchSummary; search: string; request: SidebarRequest }) {
  const rawKind = new URLSearchParams(search).get('view');
  const selected = taskKinds.find((item) => item.kind === rawKind)?.kind ?? null;
  const [reload, setReload] = useState(0);
  const [tasks, setTasks] = useState<LoadState<TaskListPage>>({ kind: 'loading' });
  useEffect(() => {
    if (selected === null) return;
    let active = true;
    setTasks({ kind: 'loading' });
    void loadEmployeeTasks(request, { kind: selected, state: 'pending', page: 1, perPage: 20 })
      .then((value) => { if (active) setTasks({ kind: 'ready', value }); })
      .catch((error: unknown) => { if (active) setTasks({ kind: 'error', message: errorMessage(error) }); });
    return () => { active = false; };
  }, [reload, request, selected]);
  const counts: Record<EmployeeTaskKind, number> = { contactSop: summary.tasks.contactSopPending, roomSop: summary.tasks.roomSopPending, batchAdd: summary.tasks.batchAddPending };
  return <div className="workbench-page"><section className="workbench-hero workbench-hero--conversation"><div><p>会话与任务</p><h2>把该说的话，在合适的时间送达</h2><span>仅展示可验证的 SOP 与加好友任务</span></div><Illustration kind="conversation" /></section><section className="workbench-section workbench-conversation-shortcuts"><header><h2>工作任务</h2><WorkbenchLink className="workbench-section__text-link" to={contextTarget('/medium', search)}>素材库</WorkbenchLink></header><div className="workbench-task-tabs">{taskKinds.map((item) => <FeatureLink key={item.kind} detail={`${counts[item.kind]} 项待处理`} icon={item.icon} label={item.label} tone={item.tone} to={contextTarget('/', search, { tab: 'conversations', view: item.kind })} />)}</div></section>{selected ? <section className="workbench-section"><header><h2>{taskKinds.find((item) => item.kind === selected)?.label}</h2><span>待处理</span></header><TaskList retry={() => setReload((value) => value + 1)} search={search} state={tasks} /></section> : <section className="workbench-honest-notice"><WorkbenchIcon kind="shield" /><div><strong>数据范围说明</strong><p>当前系统尚未接入真实会话存档，因此不展示聊天统计、敏感词或回复率。这里使用真实 SOP 和批量加好友任务替代。</p></div></section>}</div>;
}

function ContactDirectory({ request, search }: { request: SidebarRequest; search: string }) {
  const [draft, setDraft] = useState('');
  const [keyword, setKeyword] = useState('');
  const [reload, setReload] = useState(0);
  const [contacts, setContacts] = useState<LoadState<ContactListPage>>({ kind: 'loading' });
  useEffect(() => {
    let active = true;
    setContacts({ kind: 'loading' });
    void loadEmployeeContacts(request, { keyword, page: 1, perPage: 20 }).then((value) => { if (active) setContacts({ kind: 'ready', value }); }).catch((error: unknown) => { if (active) setContacts({ kind: 'error', message: errorMessage(error) }); });
    return () => { active = false; };
  }, [keyword, reload, request]);
  const submit = (event: FormEvent) => { event.preventDefault(); setKeyword(draft.trim()); };
  return <div className="workbench-page workbench-directory"><form aria-label="搜索客户" className="workbench-search" onSubmit={submit} role="search"><WorkbenchIcon kind="contacts" /><input aria-label="搜索客户" onChange={(event) => setDraft(event.target.value)} placeholder="搜索客户昵称或备注" type="search" value={draft} />{draft ? <button onClick={() => { setDraft(''); setKeyword(''); }} type="button">清除</button> : null}</form><div className="workbench-directory__tabs"><strong>我的客户</strong><span>按添加时间排序</span></div>{contacts.kind === 'loading' ? <MobileState kind="loading" title="正在加载客户" description="正在读取你的客户通讯录。" /> : null}{contacts.kind === 'error' ? <div className="workbench-inline-state"><MobileState kind="error" title="客户加载失败" description={contacts.message} /><button onClick={() => setReload((value) => value + 1)} type="button">重新加载</button></div> : null}{contacts.kind === 'ready' && contacts.value.items.length === 0 ? <MobileState kind="empty" title={keyword ? '没有匹配客户' : '暂无客户'} description={keyword ? '换一个昵称或备注关键词试试。' : '企业微信同步客户后会显示在这里。'} /> : null}{contacts.kind === 'ready' && contacts.value.items.length > 0 ? <><p className="workbench-directory__total">共 {contacts.value.total} 位客户</p><ul className="workbench-contact-list">{contacts.value.items.map((contact) => <li key={contact.id}><WorkbenchLink ariaLabel={`查看${contact.name}`} className="workbench-contact" to={contextTarget('/contact', search, { wxExternalUserid: contact.wxExternalUserid })}>{contact.avatar ? <img alt="" src={contact.avatar} /> : <span className="workbench-avatar">{contact.name.slice(0, 1) || '?'}</span>}<span className="workbench-contact__body"><strong>{contact.name || '未命名客户'}</strong><small>{contact.remark || '暂无备注'} · {contact.addedAt || '添加时间未知'}</small><span>{contact.tags.map((tag) => <em key={tag}>{tag}</em>)}</span></span><WorkbenchIcon kind="chevron" /></WorkbenchLink></li>)}</ul></> : null}</div>;
}

function ProfileWorkspace({ summary, bridge, search }: { summary: WorkbenchSummary; bridge: WeComBridge; search: string }) {
  const departments = summary.employee.departmentNames.join(' · ') || '未同步部门';
  return <div className="workbench-page workbench-profile"><section className="workbench-profile-card">{summary.employee.avatar ? <img alt="" src={summary.employee.avatar} /> : <span className="workbench-profile-card__avatar">{summary.employee.name.slice(0, 1) || '?'}</span>}<div><h2>{summary.employee.name}</h2><p>{summary.employee.corpName || '未同步企业名称'}</p><span>{departments}</span></div></section><section className="workbench-setting-group"><div><span className="workbench-setting-group__icon workbench-setting-group__icon--green"><WorkbenchIcon kind="shield" /></span><span><strong>企业微信身份已验证</strong><small>{bridge.available() ? '当前运行于企业微信客户端' : '当前运行于浏览器验收环境'}</small></span><em>正常</em></div></section><section className="workbench-setting-group"><div><span className="workbench-setting-group__icon"><WorkbenchIcon kind="building" /></span><span><strong>所属企业</strong><small>{summary.employee.corpName || '未同步'}</small></span></div><div><span className="workbench-setting-group__icon workbench-setting-group__icon--purple"><WorkbenchIcon kind="contacts" /></span><span><strong>客户数据范围</strong><small>仅展示当前员工可见客户</small></span></div><WorkbenchLink className="workbench-setting-link" to={contextTarget('/login', search, { target: contextTarget('/', search, { tab: 'profile' }) })}><span className="workbench-setting-group__icon workbench-setting-group__icon--orange"><WorkbenchIcon kind="reload" /></span><span><strong>重新授权</strong><small>登录失效或身份切换时使用</small></span><WorkbenchIcon kind="chevron" /></WorkbenchLink></section><p className="workbench-version">MoChat 员工侧边栏 · 数据来自持久化业务服务</p></div>;
}

export function WorkbenchPage({ request, bridge }: WorkbenchPageProps) {
  const location = useLocation();
  const tab = workspaceTab(location.search);
  const view = new URLSearchParams(location.search).get('view');
  const [reload, setReload] = useState(0);
  const [summary, setSummary] = useState<LoadState<WorkbenchSummary>>({ kind: 'loading' });
  useEffect(() => {
    let active = true;
    setSummary({ kind: 'loading' });
    void loadWorkbenchSummary(request).then((value) => { if (active) setSummary({ kind: 'ready', value }); }).catch((error: unknown) => { if (active) setSummary({ kind: 'error', message: errorMessage(error) }); });
    return () => { active = false; };
  }, [reload, request]);
  const title = useMemo(() => view === 'contacts' ? '客户通讯录' : ({ customers: '客户', conversations: '会话', profile: '我的' } as const)[tab], [tab, view]);
  let content: ReactNode;
  if (summary.kind === 'loading') content = <MobileState kind="loading" title="正在加载工作台" description="正在读取你的客户与任务数据。" />;
  else if (summary.kind === 'error') content = <div className="workbench-load-error"><MobileState kind="error" title="工作台加载失败" description={summary.message} /><button onClick={() => setReload((value) => value + 1)} type="button">重新加载</button></div>;
  else if (view === 'contacts' && tab === 'customers') content = <ContactDirectory request={request} search={location.search} />;
  else if (tab === 'customers') content = <CustomerWorkspace search={location.search} summary={summary.value} />;
  else if (tab === 'conversations') content = <ConversationWorkspace request={request} search={location.search} summary={summary.value} />;
  else content = <ProfileWorkspace bridge={bridge} search={location.search} summary={summary.value} />;
  return <SidebarPageShell title={title}>{content}</SidebarPageShell>;
}
