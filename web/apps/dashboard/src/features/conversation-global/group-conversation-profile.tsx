import { DashboardPagination } from '../../components/dashboard-pagination';
import { PageState } from '../../components/page-state/page-state';
import type { GroupMemberMode, GroupRoomMembersPage, GroupRoomMessageStats, GroupRoomProfile } from './conversation-global-api';

type Props = {
  profile: GroupRoomProfile | undefined;
  members: GroupRoomMembersPage | undefined;
  stats?: GroupRoomMessageStats;
  memberMode: GroupMemberMode;
  memberPage: number;
  profilePending: boolean;
  membersPending: boolean;
  profileError: Error | null;
  membersError: Error | null;
  onMemberModeChange: (mode: GroupMemberMode) => void;
  onMemberPageChange: (page: number) => void;
  onRefreshProfile: () => void;
  onRefreshMembers: () => void;
  drawerOpen?: boolean;
  onCloseDrawer?: () => void;
};

const memberModes: readonly [GroupMemberMode, string][] = [['all', '全部'], ['employee', '员工'], ['customer', '客户'], ['left', '已退出']];
function statusLabel(profile: GroupRoomProfile) { return profile.dissolved ? '已解散' : profile.status === 'active' ? '进行中' : profile.status || '状态暂缺'; }

export function GroupConversationProfile(props: Props) {
  if (props.profile === undefined && props.profilePending) return <section aria-label="群资料" className="group-conversation-profile"><PageState state="loading" title="正在读取群资料" /></section>;
  if (props.profile === undefined && props.profileError !== null) return <section aria-label="群资料" className="group-conversation-profile"><PageState state="error" title="群资料加载失败" description={props.profileError.message} onRetry={props.onRefreshProfile} /></section>;
  if (props.profile === undefined) return <section aria-label="群资料" className="group-conversation-profile"><PageState state="empty" title="群资料" description="选择客户群后显示真实资料。" /></section>;
  const profile = props.profile;
  return <section aria-label="群资料" className={`group-conversation-profile${props.drawerOpen ? ' is-drawer-open' : ''}`}><header className="group-conversation-profile-header"><div className="group-conversation-profile-avatar">{profile.avatar.trim() !== '' ? <img alt="" src={profile.avatar} /> : <span>{profile.name.trim().slice(0, 1) || '群'}</span>}</div><div><p className="group-conversation-eyebrow">群资料</p><h2>{profile.name.trim() || '未命名客户群'}</h2><span>{profile.externalId || '群 ID 暂缺'}</span></div><div className="group-conversation-profile-header-actions">{props.onCloseDrawer && <button aria-label="关闭群资料" className="group-conversation-text-button" onClick={props.onCloseDrawer} type="button">关闭</button>}<button aria-label="刷新群资料" className="group-conversation-icon-button" disabled={props.profilePending} onClick={props.onRefreshProfile} type="button">刷新</button></div></header>
    <section aria-label="群概况" className="group-conversation-profile-section"><div className="group-conversation-profile-status"><span>状态</span><strong>{statusLabel(profile)}</strong></div><dl><div><dt>群主</dt><dd>{profile.ownerName || `员工 ${profile.ownerId}`}</dd></div><div><dt>建群时间</dt><dd>{profile.createdAt || '数据暂缺'}</dd></div><div><dt>成员</dt><dd>{profile.memberCount} 人</dd></div><div><dt>员工 / 客户</dt><dd>{profile.employeeCount} / {profile.customerCount}</dd></div></dl></section>
    {props.stats !== undefined && <section aria-label="消息统计" className="group-conversation-profile-section"><h3>消息统计</h3><div className="group-conversation-profile-stats"><span><strong>{props.stats.messageTotal}</strong><small>消息</small></span><span><strong>{props.stats.employeeTotal}</strong><small>员工</small></span><span><strong>{props.stats.customerTotal}</strong><small>客户</small></span></div></section>}
    <section aria-label="群成员" className="group-conversation-profile-section group-conversation-members"><header><h3>群成员</h3><button aria-label="刷新群成员" className="group-conversation-text-button" disabled={props.membersPending} onClick={props.onRefreshMembers} type="button">刷新</button></header><div aria-label="群成员类型" className="group-conversation-member-tabs" role="tablist">{memberModes.map(([mode, label]) => <button aria-selected={props.memberMode === mode} key={mode} onClick={() => props.onMemberModeChange(mode)} role="tab" type="button">{label}</button>)}</div>{props.membersPending && <PageState state="loading" title="正在读取成员" />}{props.membersError !== null && <PageState state="error" title="成员加载失败" description={props.membersError.message} onRetry={props.onRefreshMembers} />}{!props.membersPending && props.membersError === null && props.members !== undefined && <>{props.members.items.length === 0 ? <p className="group-conversation-muted">暂无对应成员</p> : <ul>{props.members.items.map((member) => <li key={`${member.kind}-${member.id}`}><span className="group-conversation-member-avatar">{member.name.trim().slice(0, 1) || '群'}</span><span><strong>{member.name || '群成员'}</strong><small>{member.kind === 'employee' ? '员工' : member.kind === 'customer' ? '客户' : member.status}</small></span></li>)}</ul>}<DashboardPagination ariaLabel="群成员分页" page={props.members.page} pageSize={50} total={props.members.total} onPageChange={props.onMemberPageChange} /></>}</section>
    <section aria-label="风险与超时" className="group-conversation-profile-section group-conversation-signal-grid"><article><span>风险消息</span><strong>{profile.riskCount}</strong><small>暂无风险记录明细</small></article><article><span>超时消息</span><strong>{profile.timeoutCount}</strong><small>暂无超时记录明细</small></article></section>
    {profile.limitations.length > 0 && <section className="group-conversation-profile-section"><h3>能力提示</h3>{profile.limitations.map((item) => <p className="group-conversation-limitation" key={item.key}>{item.reason}</p>)}</section>}
  </section>;
}
