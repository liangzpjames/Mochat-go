import type { AccessProfile, ApprovalPoliciesData, ViewKey } from '@/lib/types'

export interface PageProps {
  navigate: (view: ViewKey) => void
  profile: AccessProfile
  approvalMode: ApprovalPoliciesData
}

export const queryKeys = {
  overview: ['saas-overview'] as const,
  packages: ['saas-packages'] as const,
  accessAssignments: ['access-assignments'] as const,
  accessRoles: ['access-roles'] as const,
  systemHealth: ['system-health'] as const,
  backup: ['backup-overview'] as const,
  operations: ['saas-operations'] as const,
  release: ['release-readiness'] as const,
  approvals: ['saas-approvals'] as const,
}

export const tenantStatusView = (value: number) => value === 1
  ? { label: '正常', tone: 'success' as const }
  : { label: '已停用', tone: 'danger' as const }

export function dueStateView(tenant: { expired: boolean; expiringSoon: boolean; expiresAt: string }) {
  if (!tenant.expiresAt) return { label: '长期有效', tone: 'neutral' as const }
  if (tenant.expired) return { label: '已到期', tone: 'danger' as const }
  if (tenant.expiringSoon) return { label: '即将到期', tone: 'warning' as const }
  return { label: '有效', tone: 'success' as const }
}

export function usageTone(ratio: number) {
  if (ratio >= 1) return 'danger' as const
  if (ratio >= 0.8) return 'warning' as const
  return 'success' as const
}

export const coreLimitFields = [
  { key: 'maxUsers', label: '子账号数', defaultValue: 10 },
  { key: 'maxCorps', label: '企业数', defaultValue: 1 },
  { key: 'maxContacts', label: '客户数', defaultValue: 1000 },
  { key: 'maxRooms', label: '客户群数', defaultValue: 50 },
  { key: 'storageMb', label: '素材存储（MB）', defaultValue: 512 },
  { key: 'channelCodes', label: '渠道活码数', defaultValue: 20 },
  { key: 'asyncExecutions', label: '月异步执行量', defaultValue: 1000 },
] as const

export const packageLimitKeys = [
  'maxCorps', 'maxUsers', 'maxContacts', 'maxRooms', 'maxAgents', 'channelCodes', 'shopCodes', 'radars', 'lotteries',
  'roomInfinitePulls', 'roomFissions', 'roomClockIns', 'roomQualities', 'roomCalendars', 'roomReminds', 'contactSops',
  'roomSops', 'sensitiveWords', 'storageMb', 'contactMessageBatches', 'roomMessageBatches', 'roomTagPulls',
  'workRoomAutoPulls', 'workFissions', 'officialAccounts', 'asyncExecutions',
] as const
