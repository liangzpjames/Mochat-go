import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { CheckCircle2, ShieldCheck, UserCog, UsersRound } from 'lucide-react'
import { toast } from 'sonner'
import { apiRequest, executeGoverned, hasPermission, jsonRequest } from '@/lib/api'
import type { AccessAssignment, AccessRole, ApprovalItem, ApprovalsData } from '@/lib/types'
import {
  Badge,
  Button,
  Dialog,
  EmptyState,
  ErrorState,
  Field,
  LoadingState,
  PageHeader,
  StatCard,
  TableShell,
  formatDate,
  textareaClassName,
} from '@/components/ui'
import type { PageProps } from './shared'
import { queryKeys } from './shared'

interface ApprovalExecuteResponse {
  approval: Record<string, unknown>
  result: Record<string, unknown>
}

export default function TeamPage({ profile, approvalMode }: PageProps) {
  const queryClient = useQueryClient()
  const [editingMember, setEditingMember] = useState<AccessAssignment | null>(null)
  const [selectedRoles, setSelectedRoles] = useState<number[]>([])
  const [decisionItem, setDecisionItem] = useState<ApprovalItem | null>(null)
  const [decision, setDecision] = useState<'approve' | 'reject'>('approve')
  const [decisionReason, setDecisionReason] = useState('')
  const [activationToken, setActivationToken] = useState('')
  const [activationTokenOpen, setActivationTokenOpen] = useState(false)

  const assignmentsQuery = useQuery({
    queryKey: queryKeys.accessAssignments,
    queryFn: () => apiRequest<{ assignments: AccessAssignment[] }>('/dashboard/saasAdmin/accessAssignments?limit=100'),
  })
  const rolesQuery = useQuery({
    queryKey: queryKeys.accessRoles,
    queryFn: () => apiRequest<{ roles: AccessRole[] }>('/dashboard/saasAdmin/accessRoles'),
  })
  const approvalsQuery = useQuery({
    queryKey: queryKeys.approvals,
    queryFn: () => apiRequest<ApprovalsData>('/dashboard/saasAdmin/approvals?status=all&riskLevel=all&limit=50'),
    enabled: approvalMode.required && hasPermission(profile.permissions, 'platform.approvals.read'),
  })

  const invalidate = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: queryKeys.accessAssignments }),
      queryClient.invalidateQueries({ queryKey: queryKeys.accessRoles }),
      queryClient.invalidateQueries({ queryKey: queryKeys.approvals }),
      queryClient.invalidateQueries({ queryKey: queryKeys.operations }),
    ])
  }

  const assignmentMutation = useMutation({
    mutationFn: async () => {
      if (!editingMember) throw new Error('未选择平台账号')
      const payload = { userId: editingMember.userId, roleIds: selectedRoles, expectedVersion: editingMember.version }
      return executeGoverned<Record<string, unknown>>({
        approvalMode,
        actionType: 'access.assignment.save',
        payload,
        reason: `调整平台账号 ${editingMember.userName} 的岗位`,
        directPath: '/dashboard/saasAdmin/accessAssignment',
        directMethod: 'PUT',
      })
    },
    onSuccess: async (result) => {
      toast.success(result.approvalRequested ? '账号授权已提交审批' : '账号授权已保存')
      setEditingMember(null)
      await invalidate()
    },
    onError: (error) => toast.error(error instanceof Error ? error.message : '授权保存失败'),
  })

  const decisionMutation = useMutation({
    mutationFn: async () => {
      if (!decisionItem) throw new Error('未选择审批单')
      if (!decisionReason.trim()) throw new Error('请填写复核意见')
      return apiRequest<Record<string, unknown>>('/dashboard/saasAdmin/approvalDecision', jsonRequest('POST', {
        approvalId: decisionItem.id,
        expectedVersion: decisionItem.version,
        decision,
        reason: decisionReason.trim(),
      }))
    },
    onSuccess: async () => {
      toast.success(decision === 'approve' ? '会签已记录' : '审批已驳回')
      setDecisionItem(null)
      setDecisionReason('')
      await invalidate()
    },
    onError: (error) => toast.error(error instanceof Error ? error.message : '审批处理失败'),
  })

  const executeMutation = useMutation<ApprovalExecuteResponse, unknown, ApprovalItem>({
    mutationFn: (item) => apiRequest<ApprovalExecuteResponse>('/dashboard/saasAdmin/approvalExecute', jsonRequest('POST', { approvalId: item.id, expectedVersion: item.version })),
    onSuccess: async (result) => {
      const token = typeof result.result.activationToken === 'string' ? result.result.activationToken : ''
      if (token) {
        setActivationToken(token)
        setActivationTokenOpen(true)
      }
      toast.success(token ? '已批准操作执行完成，请安全记录一次性激活令牌' : '已批准操作执行完成')
      await invalidate()
      await queryClient.invalidateQueries({ queryKey: queryKeys.overview })
    },
    onError: (error) => toast.error(error instanceof Error ? error.message : '审批执行失败'),
  })

  if (assignmentsQuery.isLoading || rolesQuery.isLoading) return <LoadingState label="正在加载平台账号" />
  if (assignmentsQuery.isError || !assignmentsQuery.data) return <ErrorState message={assignmentsQuery.error instanceof Error ? assignmentsQuery.error.message : '无法加载平台账号'} onRetry={() => assignmentsQuery.refetch()} />

  const assignments = assignmentsQuery.data.assignments || []
  const roles = rolesQuery.data?.roles || []
  const pendingApprovals = (approvalsQuery.data?.items || []).filter((item) => item.status === 'pending' || item.status === 'approved')
  const superAdminCount = assignments.filter((item) => item.isSuperAdmin).length
  const activeMemberCount = assignments.filter((item) => item.status === 1).length
  const assignedRoleCount = new Set(assignments.flatMap((item) => item.roles.map((role) => role.id))).size
  const canReview = hasPermission(profile.permissions, 'platform.approvals.review')
  const canExecute = hasPermission(profile.permissions, 'platform.approvals.execute')

  const openRoleEditor = (member: AccessAssignment) => {
    setEditingMember(member)
    setSelectedRoles(member.roles.map((role) => role.id))
  }

  const openDecision = (item: ApprovalItem, nextDecision: 'approve' | 'reject') => {
    setDecisionItem(item)
    setDecision(nextDecision)
    setDecisionReason(nextDecision === 'approve' ? '复核通过' : '')
  }

  return (
    <div className="space-y-7">
      <PageHeader title="平台账号" description="控制总后台访问与运营岗位" />

      <section className={`flex items-center gap-3 rounded-lg border px-4 py-3 ${approvalMode.required ? 'border-amber-200 bg-amber-50' : 'border-sky-200 bg-sky-50'}`}>
        <ShieldCheck className={`h-5 w-5 shrink-0 ${approvalMode.required ? 'text-amber-700' : 'text-sky-700'}`} />
        <div className="min-w-0"><strong className="block text-sm text-zinc-900">{approvalMode.required ? '团队审批模式' : '创始人单人运营模式'}</strong><span className="text-xs text-zinc-600">{approvalMode.required ? '关键变更需要独立复核后执行' : '关键变更直接执行并保留完整审计'}</span></div>
      </section>

      <section className="grid gap-4 sm:grid-cols-3">
        <StatCard label="有效账号" value={activeMemberCount} hint={`${superAdminCount} 个超级管理员`} icon={<UsersRound className="h-5 w-5" />} />
        <StatCard label="已使用岗位" value={assignedRoleCount} hint={`${roles.length} 个可用岗位`} icon={<UserCog className="h-5 w-5" />} />
        <StatCard label="待处理审批" value={pendingApprovals.length} hint={approvalMode.required ? '待复核或待执行' : '当前无需双人审批'} icon={<CheckCircle2 className="h-5 w-5" />} />
      </section>

      <TableShell>
        {assignments.length === 0 ? (
          <EmptyState icon={<UsersRound className="h-5 w-5" />} title="暂无平台账号" description="平台管理租户中启用的成员会显示在这里。" />
        ) : (
          <table className="min-w-[820px]">
            <thead><tr><th>账号</th><th>平台岗位</th><th>权限</th><th>状态</th><th className="w-24">操作</th></tr></thead>
            <tbody>
              {assignments.map((member) => (
                <tr key={member.userId}>
                  <td><strong className="font-medium text-zinc-950">{member.userName}</strong><div className="mt-1 text-xs text-zinc-500">{member.phone} / ID {member.userId}</div></td>
                  <td>{member.isSuperAdmin ? <Badge tone="success">超级管理员</Badge> : member.roles.length ? <div className="flex flex-wrap gap-1">{member.roles.map((role) => <Badge key={role.id}>{role.name}</Badge>)}</div> : <span className="text-zinc-400">未分配</span>}</td>
                  <td>{member.isSuperAdmin ? '全部权限' : `${member.permissions.length} 项`}</td>
                  <td><Badge tone={member.status === 1 ? 'success' : 'danger'}>{member.status === 1 ? '正常' : '已停用'}</Badge></td>
                  <td>{!member.isSuperAdmin && <Button variant="ghost" onClick={() => openRoleEditor(member)}>调整岗位</Button>}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </TableShell>

      {approvalMode.required && (
        <section className="space-y-3">
          <div><h2 className="text-base font-semibold text-zinc-950">审批队列</h2><p className="mt-1 text-sm text-zinc-500">仅保留关键变更的复核与执行</p></div>
          <TableShell>
            {approvalsQuery.isLoading ? <LoadingState /> : pendingApprovals.length === 0 ? (
              <EmptyState icon={<CheckCircle2 className="h-5 w-5" />} title="没有待处理审批" description="当前关键变更均已处理。" />
            ) : (
              <table className="min-w-[980px]">
                <thead><tr><th>审批单</th><th>事项</th><th>申请人</th><th>进度</th><th>时限</th><th className="w-52">操作</th></tr></thead>
                <tbody>{pendingApprovals.map((item) => { const requester = item.requesterUserId === profile.userId; return <tr key={item.id}><td><strong className="font-medium text-zinc-950">{item.requestNo}</strong><div className="mt-1 text-xs text-zinc-500">#{item.id}</div></td><td>{item.targetName || item.actionType}<div className="mt-1 max-w-sm truncate text-xs text-zinc-500">{item.reason}</div></td><td>{item.requesterName || `用户 ${item.requesterUserId}`}</td><td><Badge tone={item.status === 'approved' ? 'success' : 'warning'}>{item.status === 'approved' ? '待执行' : `${item.approvalCount || 0}/${item.requiredApprovals}`}</Badge></td><td>{formatDate(item.slaDueAt || item.expiresAt)}</td><td><div className="flex flex-wrap gap-1">{item.status === 'pending' && !requester && canReview && <><Button variant="secondary" onClick={() => openDecision(item, 'approve')}>批准</Button><Button variant="ghost" onClick={() => openDecision(item, 'reject')}>驳回</Button></>}{item.status === 'approved' && !requester && canExecute && <Button loading={executeMutation.isPending} onClick={() => executeMutation.mutate(item)}>执行</Button>}</div></td></tr> })}</tbody>
              </table>
            )}
          </TableShell>
        </section>
      )}

      <Dialog open={Boolean(editingMember)} onOpenChange={(open) => !open && setEditingMember(null)} title="调整平台岗位" description={editingMember?.userName} footer={<><Button variant="secondary" onClick={() => setEditingMember(null)}>取消</Button><Button loading={assignmentMutation.isPending} onClick={() => assignmentMutation.mutate()}>{approvalMode.required ? '提交授权' : '保存授权'}</Button></>}>
        <div className="grid gap-2">{roles.filter((role) => role.status === 1).map((role) => <label key={role.id} className="flex cursor-pointer items-start gap-3 rounded-md border border-zinc-200 p-3 hover:bg-zinc-50"><input type="checkbox" className="mt-0.5 h-4 w-4 accent-emerald-700" checked={selectedRoles.includes(role.id)} onChange={(event) => setSelectedRoles((values) => event.target.checked ? [...values, role.id] : values.filter((id) => id !== role.id))} /><span><strong className="block text-sm font-medium text-zinc-900">{role.name}</strong><span className="mt-0.5 block text-xs leading-5 text-zinc-500">{role.description}</span></span></label>)}</div>
      </Dialog>

      <Dialog open={Boolean(decisionItem)} onOpenChange={(open) => !open && setDecisionItem(null)} title={decision === 'approve' ? '批准审批' : '驳回审批'} description={decisionItem?.requestNo} size="sm" footer={<><Button variant="secondary" onClick={() => setDecisionItem(null)}>取消</Button><Button variant={decision === 'approve' ? 'primary' : 'danger'} loading={decisionMutation.isPending} onClick={() => decisionMutation.mutate()}>{decision === 'approve' ? '确认批准' : '确认驳回'}</Button></>}>
        <Field label="复核意见"><textarea className={textareaClassName} value={decisionReason} onChange={(event) => setDecisionReason(event.target.value)} autoFocus /></Field>
      </Dialog>

      <Dialog open={activationTokenOpen} onOpenChange={(open) => { setActivationTokenOpen(open); if (!open) setActivationToken('') }} title="一次性激活令牌" description="该令牌只在本次审批执行响应后显示，关闭后无法再次查看。" size="sm">
        <div className="space-y-3"><p className="text-sm text-amber-800">请使用受控交付渠道传给对应管理员；不要写入审批意见、日志或截图。</p><code className="block break-all rounded-md bg-zinc-950 px-3 py-3 text-xs text-emerald-300">{activationToken}</code><Button type="button" variant="secondary" onClick={() => { setActivationTokenOpen(false); setActivationToken('') }}>我已记录并关闭</Button></div>
      </Dialog>
    </div>
  )
}
