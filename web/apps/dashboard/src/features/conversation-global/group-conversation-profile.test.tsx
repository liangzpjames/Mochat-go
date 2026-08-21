import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';

import type { GroupRoomMembersPage, GroupRoomProfile } from './conversation-global-api';
import { GroupConversationProfile } from './group-conversation-profile';

const profile: GroupRoomProfile = { id: 71, externalId: 'wr_71', name: '星河客户群', avatar: '', ownerId: 9, ownerName: '张三', memberCount: 6, employeeCount: 2, customerCount: 4, createdAt: '2026-08-01 10:00:00', status: 'active', dissolved: false, focused: false, riskCount: 1, timeoutCount: 0, capabilities: [], limitations: [] };
const members: GroupRoomMembersPage = { items: [{ id: 9, externalId: 'zhangsan', kind: 'employee', name: '张三', avatar: '', joinedAt: '2026-08-01', leftAt: '', status: 'active', employeeId: 9, customerId: 0 }], total: 1, page: 1, pageSize: 50, capabilities: [] };

describe('GroupConversationProfile', () => {
  afterEach(cleanup);
  it('renders a full-height profile column from real profile and member data', () => {
    render(<GroupConversationProfile profile={profile} members={members} memberMode="all" memberPage={1} profilePending={false} membersPending={false} profileError={null} membersError={null} onMemberModeChange={() => undefined} onMemberPageChange={() => undefined} onRefreshProfile={() => undefined} onRefreshMembers={() => undefined} />);
    expect(screen.getByRole('heading', { name: '星河客户群' })).toBeTruthy();
    expect(screen.getAllByText('张三').length).toBeGreaterThanOrEqual(2);
    expect(screen.getByText('群成员')).toBeTruthy();
    expect(screen.getByText('暂无风险记录明细')).toBeTruthy();
  });
});
