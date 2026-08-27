import { useQuery } from '@tanstack/react-query';
import { useEffect, useMemo, useState } from 'react';
import type { FormEvent } from 'react';
import { useSearchParams } from 'react-router';

import { useDashboardAccess } from '../../app/access-context';
import { updateSearch } from '../../shared/query-state';
import type { ConversationGlobalApi, GroupMemberMode, GroupRoomDirectoryInput, GroupRoomMessage, GroupRoomMode } from './conversation-global-api';
import { ConversationArchiveUnavailableState, isConversationArchiveUnavailable } from './conversation-archive-state';
import { GroupConversationDirectory } from './group-conversation-directory';
import { GroupConversationFilterDrawer, type GroupConversationFilterValue } from './group-conversation-filter-drawer';
import { GroupConversationMessages } from './group-conversation-messages';
import { GroupConversationProfile } from './group-conversation-profile';

const pageSize = 50;
function positive(value: string | null, fallback: number) { const parsed = Number(value); return Number.isInteger(parsed) && parsed > 0 ? parsed : fallback; }
function roomMode(value: string | null): GroupRoomMode { return value === 'dissolved' ? 'dissolved' : 'active'; }
function ids(search: URLSearchParams, key: string) { return search.getAll(key).map(Number).filter((value) => Number.isInteger(value) && value > 0); }
function errorOf(error: unknown) { return error instanceof Error ? error : error === null || error === undefined ? null : new Error('请求失败'); }

export function GroupConversationPage({ api }: { api: ConversationGlobalApi }) {
  const access = useDashboardAccess();
  const [searchParams, setSearchParams] = useSearchParams();
  const mode = roomMode(searchParams.get('roomMode'));
  const roomKeyword = searchParams.get('roomKeyword') ?? '';
  const roomPage = positive(searchParams.get('roomPage'), 1);
  const roomIdValue = positive(searchParams.get('roomId'), 0);
  const roomId = roomIdValue > 0 ? roomIdValue : null;
  const appliedMessageKeyword = searchParams.get('messageKeyword') ?? '';
  const appliedDate = searchParams.get('messageDate') ?? '';
  const appliedMessageTypes = searchParams.getAll('messageTypes');
  const [roomKeywordDraft, setRoomKeywordDraft] = useState(roomKeyword);
  const [messageKeywordDraft, setMessageKeywordDraft] = useState(appliedMessageKeyword);
  const [filterOpen, setFilterOpen] = useState(false);
  const [profileOpen, setProfileOpen] = useState(false);
  const [profileClosed, setProfileClosed] = useState(false);
  const [filterValue, setFilterValue] = useState<GroupConversationFilterValue>({ employeeId: '', customerId: '', groupId: '' });
  const [memberMode, setMemberMode] = useState<GroupMemberMode>('all');
  const [memberPage, setMemberPage] = useState(1);
  const [before, setBefore] = useState('');
  const [loadedMessages, setLoadedMessages] = useState<readonly GroupRoomMessage[]>([]);

  useEffect(() => setRoomKeywordDraft(roomKeyword), [roomKeyword]);
  useEffect(() => setMessageKeywordDraft(appliedMessageKeyword), [appliedMessageKeyword]);
  useEffect(() => { setBefore(''); setLoadedMessages([]); }, [roomId, appliedMessageKeyword, appliedDate, appliedMessageTypes.join('|')]);
  useEffect(() => { setMemberPage(1); }, [roomId, memberMode]);

  const directoryInput = useMemo<GroupRoomDirectoryInput>(() => ({ mode, keyword: roomKeyword, page: roomPage, pageSize, employeeIds: ids(searchParams, 'employeeIds'), customerIds: ids(searchParams, 'customerIds'), roomGroupIds: ids(searchParams, 'roomGroupIds') }), [mode, roomKeyword, roomPage, searchParams]);
  const directoryQuery = useQuery({ queryKey: ['room-directory', access.corp.id, directoryInput], queryFn: () => api.groupRoomDirectory!(directoryInput), enabled: api.groupRoomDirectory !== undefined, retry: false });
  const selectedRoom = directoryQuery.data?.items.find((item) => item.id === roomId);
  const profileQuery = useQuery({ queryKey: ['room-profile', access.corp.id, roomId], queryFn: () => api.groupRoomProfile!(roomId!), enabled: roomId !== null && api.groupRoomProfile !== undefined, retry: false });
  const messageInput = useMemo(() => ({ roomId: roomId!, keyword: appliedMessageKeyword, date: appliedDate, messageTypes: appliedMessageTypes, ...(before ? { before } : {}), pageSize: 50 as const }), [roomId, appliedDate, appliedMessageKeyword, appliedMessageTypes.join('|'), before]);
  const messagesQuery = useQuery({ queryKey: ['room-messages', access.corp.id, messageInput], queryFn: () => api.groupRoomMessages!(messageInput), enabled: roomId !== null && api.groupRoomMessages !== undefined, retry: false, placeholderData: (previous) => previous });
  const membersQuery = useQuery({ queryKey: ['room-members', access.corp.id, roomId, memberMode, memberPage], queryFn: () => api.groupRoomMembers!({ roomId: roomId!, mode: memberMode, keyword: '', page: memberPage, pageSize: 50 }), enabled: roomId !== null && api.groupRoomMembers !== undefined, retry: false, placeholderData: (previous) => previous });
  const optionsQuery = useQuery({ queryKey: ['room-filter-options', access.corp.id], queryFn: () => api.groupRoomFilterOptions!(), enabled: filterOpen && api.groupRoomFilterOptions !== undefined, retry: false });

  useEffect(() => {
    if (directoryQuery.data === undefined || roomId !== null || directoryQuery.data.items.length === 0) return;
    setSearchParams(updateSearch(searchParams, { roomId: directoryQuery.data.items[0]!.id }), { replace: true });
  }, [directoryQuery.data, roomId, searchParams, setSearchParams]);
  useEffect(() => {
    if (messagesQuery.data === undefined) return;
    setLoadedMessages((current) => {
      if (!before) return messagesQuery.data.messages;
      const map = new Map<string, GroupRoomMessage>();
      [...messagesQuery.data.messages, ...current].forEach((message) => map.set(message.id, message));
      return [...map.values()].sort((a, b) => a.sentAt.localeCompare(b.sentAt) || a.id.localeCompare(b.id));
    });
  }, [before, messagesQuery.data]);

  function submitRoomSearch(event: FormEvent<HTMLFormElement>) { event.preventDefault(); setSearchParams(updateSearch(searchParams, { roomKeyword: roomKeywordDraft, roomPage: 1 })); }
  function changeMode(nextMode: GroupRoomMode) { const next = updateSearch(searchParams, { roomMode: nextMode, roomPage: 1 }); next.delete('roomId'); setSearchParams(next); }
  function selectRoom(room: { id: number }) { const next = updateSearch(searchParams, { roomId: room.id, messageKeyword: '', messageDate: '' }); next.delete('messageTypes'); setSearchParams(next); setBefore(''); setLoadedMessages([]); }
  function submitMessageSearch(event: FormEvent<HTMLFormElement>) { event.preventDefault(); setSearchParams(updateSearch(searchParams, { messageKeyword: messageKeywordDraft })); setBefore(''); }
  function toggleMessageType(value: string) { const next = new URLSearchParams(searchParams); const values = new Set(next.getAll('messageTypes')); if (values.has(value)) values.delete(value); else values.add(value); next.delete('messageTypes'); [...values].forEach((item) => next.append('messageTypes', item)); setSearchParams(next); setBefore(''); }
  function changeDate(value: string) { setSearchParams(updateSearch(searchParams, { messageDate: value })); setBefore(''); }
  function applyFilter(value: GroupConversationFilterValue) { setFilterValue(value); const next = new URLSearchParams(searchParams); next.delete('employeeIds'); next.delete('customerIds'); next.delete('roomGroupIds'); if (value.employeeId) next.set('employeeIds', value.employeeId); if (value.customerId) next.set('customerIds', value.customerId); if (value.groupId) next.set('roomGroupIds', value.groupId); next.set('roomPage', '1'); next.delete('roomId'); setSearchParams(next); setFilterOpen(false); }
  function loadOlder() { if (messagesQuery.data?.hasMore && messagesQuery.data.nextBefore) setBefore(messagesQuery.data.nextBefore); }
  const mergedMessages = messagesQuery.data === undefined ? undefined : { ...messagesQuery.data, messages: loadedMessages.length > 0 ? loadedMessages : messagesQuery.data.messages };
  const directoryError = api.groupRoomDirectory === undefined ? new Error('群聊目录能力未接入') : errorOf(directoryQuery.error);

  if (isConversationArchiveUnavailable(directoryQuery.error)) return <ConversationArchiveUnavailableState />;

  function openProfile() { setProfileClosed(false); setProfileOpen(true); }
  function closeProfile() { setProfileOpen(false); setProfileClosed(true); }

  return <section aria-label="群聊会话工作台" className={`group-conversation-workspace${profileClosed ? ' is-profile-closed' : ''}`}><GroupConversationDirectory data={directoryQuery.data} mode={mode} keyword={roomKeywordDraft} selectedRoomId={roomId} pending={directoryQuery.isPending || directoryQuery.isFetching} error={directoryError} onKeywordChange={setRoomKeywordDraft} onSearch={submitRoomSearch} onRefresh={() => void directoryQuery.refetch()} onModeChange={changeMode} onSelect={selectRoom} onPageChange={(page) => setSearchParams(updateSearch(searchParams, { roomPage: page }))} onOpenFilter={() => setFilterOpen(true)} /><GroupConversationMessages room={selectedRoom} data={mergedMessages} keyword={messageKeywordDraft} date={appliedDate} messageTypes={appliedMessageTypes} pending={messagesQuery.isPending} fetching={messagesQuery.isFetching} loadingOlder={before !== '' && messagesQuery.isFetching} error={errorOf(messagesQuery.error)} onKeywordChange={setMessageKeywordDraft} onDateChange={changeDate} onToggleMessageType={toggleMessageType} onSearch={submitMessageSearch} onRefresh={() => void messagesQuery.refetch()} onLoadOlder={loadOlder} onOpenProfile={openProfile} /><>{profileOpen && <button aria-label="关闭群资料遮罩" className="group-conversation-profile-backdrop" onClick={closeProfile} type="button" />}</><GroupConversationProfile profile={profileQuery.data} members={membersQuery.data} {...(mergedMessages?.stats ? { stats: mergedMessages.stats } : {})} memberMode={memberMode} memberPage={memberPage} profilePending={profileQuery.isPending} membersPending={membersQuery.isPending} profileError={errorOf(profileQuery.error)} membersError={errorOf(membersQuery.error)} onMemberModeChange={setMemberMode} onMemberPageChange={setMemberPage} onRefreshProfile={() => void profileQuery.refetch()} onRefreshMembers={() => void membersQuery.refetch()} drawerOpen={profileOpen} onCloseDrawer={closeProfile} /><GroupConversationFilterDrawer open={filterOpen} value={filterValue} options={optionsQuery.data} onSubmit={applyFilter} onClose={() => setFilterOpen(false)} /></section>;
}
