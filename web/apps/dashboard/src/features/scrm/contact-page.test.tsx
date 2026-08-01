import { ApiError } from '@mochat/api-client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, useLocation } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ContactPage } from './contact-page';

vi.mock('../../app/access-context', () => ({ useDashboardAccess: () => ({ corp: { id: '7' }, allowedActions: new Set(['/customer/contact@edit', '/customer/contact@follow-up', '/customer/contact@opportunity']) }) }));
const contact = { id: 'c1', name: '张三', phone: '13800000000', ownerId: 11, assignmentStatus: 'owned', tagNames: ['VIP'], version: 2, assignmentVersion: 4, updatedAt: '2026-08-01T00:00:00Z' };
const detail = { ...contact, assignment: { id: 'a1', contactId: 'c1', ownerId: 11, collaboratorIds: [12], status: 'owned', version: 4 }, tags: [{ id: 'vip', name: 'VIP', version: 3, usageCount: 2 }], wecomFriendsAvailable: true, wecomFriends: [{ externalUserId: 'wx-1', name: '张三', employeeId: 11, addedAt: '2026-08-01T00:00:00Z' }], opportunities: [{ id: 'o1', stage: 'proposal', status: 'open', amount: 100, version: 1 }], followUps: [{ id: 'f1', contactId: 'c1', content: '首次沟通', createdAt: '2026-08-01T00:00:00Z', createdBy: 11 }] };
const LocationProbe = () => <output data-testid="location">{useLocation().search}</output>;
const wrap = (ui: React.ReactNode, entries = ['/customer/contact']) => render(<MemoryRouter initialEntries={entries}><QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })}>{ui}<LocationProbe /></QueryClientProvider></MemoryRouter>);
afterEach(cleanup);

describe('ContactPage', () => {
  it('restores combined filters from URL and keeps pagination in URL', async () => {
    const api = { listContacts: vi.fn().mockResolvedValue({ items: [contact], nextCursor: '20' }) } as any;
    wrap(<ContactPage api={api} />, ['/customer/contact?keyword=%E5%BC%A0&ownerId=11&tagId=vip&status=owned']);
    expect(await screen.findByText('张三')).toBeTruthy();
    expect(api.listContacts).toHaveBeenCalledWith({ corpId: 7, keyword: '张', ownerIds: [11], tagIds: ['vip'], statuses: ['owned'], pageSize: 20 });
    fireEvent.click(screen.getByRole('button', { name: '下一页' }));
    expect(screen.getByTestId('location').textContent).toContain('cursor=20');
  });

  it('opens aggregate detail and executes lifecycle actions', async () => {
    const important = { id: 'important', groupId: 'g1', name: '重点', version: 8, usageCount: 4 };
    const api = { listContacts: vi.fn().mockResolvedValue({ items: [contact], nextCursor: '' }), getContact: vi.fn().mockResolvedValue(detail), listTagCatalog: vi.fn().mockResolvedValue({ groups: [], tags: [important] }), updateAssignment: vi.fn().mockResolvedValue(detail.assignment), maintainTagContacts: vi.fn().mockResolvedValue({ ...important, version: 9, usageCount: 5 }), listFollowUps: vi.fn().mockResolvedValue({ items: detail.followUps, nextCursor: '' }), appendFollowUp: vi.fn().mockResolvedValue({}), releaseToPublicPool: vi.fn().mockResolvedValue({ ...detail.assignment, status: 'public_pool', version: 5 }), createOpportunity: vi.fn().mockResolvedValue({}) } as any;
    wrap(<ContactPage api={api} />);
    fireEvent.click(await screen.findByRole('button', { name: '查看张三' }));
    expect(await screen.findByText('企微好友')).toBeTruthy();
    expect(await screen.findByText(/首次沟通/)).toBeTruthy();
    fireEvent.change(await screen.findByLabelText('联系人标签'), { target: { value: 'important' } });
    fireEvent.click(screen.getByRole('button', { name: '添加标签' }));
    await waitFor(() => expect(api.maintainTagContacts).toHaveBeenCalledWith(expect.objectContaining({ corpId: 7, addContactIds: ['c1'], removeContactIds: [], tagId: 'important', version: 8, idempotencyKey: 'contact-tag-add-c1-important-8' })));
    fireEvent.click(screen.getByRole('button', { name: '移除标签 VIP' }));
    await waitFor(() => expect(api.maintainTagContacts).toHaveBeenCalledWith(expect.objectContaining({ corpId: 7, addContactIds: [], removeContactIds: ['c1'], tagId: 'vip', version: 3, idempotencyKey: 'contact-tag-remove-c1-vip-3' })));
    fireEvent.change(screen.getByLabelText('联系人入海原因'), { target: { value: '长期未跟进' } });
    fireEvent.click(screen.getByRole('button', { name: '进入公海' }));
    await waitFor(() => expect(api.releaseToPublicPool).toHaveBeenCalledWith({ corpId: 7, contactId: 'c1', version: 4, action: 'enter', reason: '长期未跟进', idempotencyKey: 'pool-enter-c1-4' }));
  });

  it('renders empty, forbidden, conflict and retry states with PageState', async () => {
    const empty = wrap(<ContactPage api={{ listContacts: vi.fn().mockResolvedValue({ items: [], nextCursor: '' }) } as any} />);
    await waitFor(() => expect(empty.container.querySelector('.page-state-empty')).not.toBeNull()); empty.unmount();
    const forbidden = wrap(<ContactPage api={{ listContacts: vi.fn().mockRejectedValue(new ApiError('forbidden', 'forbidden', { status: 403 })) } as any} />);
    await waitFor(() => expect(forbidden.container.querySelector('.page-state-forbidden')).not.toBeNull()); forbidden.unmount();
    const api = { listContacts: vi.fn().mockResolvedValue({ items: [contact], nextCursor: '' }), getContact: vi.fn().mockRejectedValue(new ApiError('validation', 'conflict', { status: 409 })) } as any;
    const conflict = wrap(<ContactPage api={api} />); fireEvent.click(await screen.findByRole('button', { name: '查看张三' }));
    await waitFor(() => expect(conflict.container.querySelector('.page-state-conflict')).not.toBeNull());
  });
});
