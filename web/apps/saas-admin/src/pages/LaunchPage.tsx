import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { CheckCircle2, ClipboardCheck, ExternalLink, Pencil, Rocket, ShieldAlert } from 'lucide-react'
import { toast } from 'sonner'
import { apiRequest, executeGoverned, hasPermission, jsonRequest } from '@/lib/api'
import type { ReleaseAction, ReleaseEvidence, ReleaseReadinessData } from '@/lib/types'
import {
  Badge,
  Button,
  Dialog,
  EmptyState,
  ErrorState,
  Field,
  Input,
  LoadingState,
  PageHeader,
  SectionHeader,
  Select,
  StatCard,
  TableShell,
  formatDate,
  textareaClassName,
} from '@/components/ui'
import type { PageProps } from './shared'
import { queryKeys } from './shared'

const evidenceStatus = (status: string) => {
  if (status === 'passed') return { label: '已通过', tone: 'success' as const }
  if (status === 'in_progress') return { label: '进行中', tone: 'warning' as const }
  if (status === 'failed') return { label: '未通过', tone: 'danger' as const }
  return { label: '缺失', tone: 'neutral' as const }
}

const dueState = (state: string) => {
  if (state === 'overdue') return { label: '已逾期', tone: 'danger' as const }
  if (state === 'due_soon') return { label: '即将到期', tone: 'warning' as const }
  if (state === 'scheduled') return { label: '已排期', tone: 'info' as const }
  if (state === 'resolved') return { label: '已完成', tone: 'success' as const }
  return { label: '未排期', tone: 'neutral' as const }
}

function dateTimeInput(value: string) {
  return value ? value.replace(' ', 'T').slice(0, 16) : ''
}

function dateTimePayload(value: string) {
  if (!value) return ''
  const normalized = value.replace('T', ' ')
  return normalized.length === 16 ? `${normalized}:00` : normalized
}

export default function LaunchPage({ profile, approvalMode }: PageProps) {
  const queryClient = useQueryClient()
  const canManage = hasPermission(profile.permissions, 'platform.release.manage')
  const [actionDraft, setActionDraft] = useState<ReleaseAction | null>(null)
  const [evidenceDraft, setEvidenceDraft] = useState<ReleaseEvidence | null>(null)
  const [releaseVersion, setReleaseVersion] = useState('')

  const releaseQuery = useQuery({
    queryKey: queryKeys.release,
    queryFn: () => apiRequest<ReleaseReadinessData>('/dashboard/saasAdmin/releaseReadiness'),
  })

  const refresh = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: queryKeys.release }),
      queryClient.invalidateQueries({ queryKey: queryKeys.operations }),
    ])
  }

  const actionMutation = useMutation({
    mutationFn: async () => {
      if (!actionDraft) throw new Error('未选择上线任务')
      if (!actionDraft.nextAction.trim()) throw new Error('下一步动作不能为空')
      if (actionDraft.ownerUserId > 0 && !actionDraft.dueAt) throw new Error('分配负责人时必须设置截止时间')
      return apiRequest<Record<string, unknown>>('/dashboard/saasAdmin/releaseEvidenceAction', jsonRequest('PUT', {
        key: actionDraft.key,
        ownerUserId: actionDraft.ownerUserId,
        dueAt: actionDraft.ownerUserId > 0 ? dateTimePayload(actionDraft.dueAt) : '',
        nextAction: actionDraft.nextAction.trim(),
        note: actionDraft.note.trim(),
        expectedVersion: actionDraft.version,
      }))
    },
    onSuccess: async () => {
      toast.success('上线任务已保存')
      setActionDraft(null)
      await refresh()
    },
    onError: (error) => toast.error(error instanceof Error ? error.message : '上线任务保存失败'),
  })

  const evidenceMutation = useMutation({
    mutationFn: async () => {
      if (!evidenceDraft) throw new Error('未选择生产证据')
      if (evidenceDraft.status === 'passed') {
        if (!/^https:\/\//i.test(evidenceDraft.evidenceUrl)) throw new Error('通过状态必须填写 HTTPS 证据地址')
        if (!/^[0-9a-f]{64}$/i.test(evidenceDraft.artifactSha256)) throw new Error('通过状态必须填写 64 位工件 SHA-256')
        if (Number(evidenceDraft.artifactSizeBytes) <= 0) throw new Error('通过状态必须填写工件大小')
      }
      return apiRequest<Record<string, unknown>>('/dashboard/saasAdmin/releaseEvidence', jsonRequest('PUT', {
        key: evidenceDraft.key,
        status: evidenceDraft.status,
        evidenceUrl: evidenceDraft.evidenceUrl.trim(),
        environment: evidenceDraft.environment.trim(),
        sourceFingerprint: evidenceDraft.sourceFingerprint.trim().toLowerCase(),
        artifactSha256: evidenceDraft.artifactSha256.trim().toLowerCase(),
        artifactSizeBytes: Math.trunc(Number(evidenceDraft.artifactSizeBytes) || 0),
        note: evidenceDraft.note.trim(),
        expectedVersion: evidenceDraft.version,
      }))
    },
    onSuccess: async () => {
      toast.success('生产证据已保存')
      setEvidenceDraft(null)
      await refresh()
    },
    onError: (error) => toast.error(error instanceof Error ? error.message : '生产证据保存失败'),
  })

  const gateMutation = useMutation({
    mutationFn: async () => {
      if (!releaseVersion.trim()) throw new Error('请填写发布版本')
      const fingerprint = releaseQuery.data?.summary.targetSourceFingerprint || ''
      return executeGoverned<Record<string, unknown>>({
        approvalMode,
        actionType: 'release.candidate.gate',
        payload: { releaseVersion: releaseVersion.trim(), sourceFingerprint: fingerprint },
        reason: `运行发布候选门禁：${releaseVersion.trim()}`,
        directPath: '/dashboard/saasAdmin/releaseCandidate',
      })
    },
    onSuccess: async (result) => {
      toast.success(result.approvalRequested ? '发布候选已提交审批' : '发布门禁执行完成')
      await refresh()
    },
    onError: (error) => toast.error(error instanceof Error ? error.message : '发布门禁执行失败'),
  })

  if (releaseQuery.isLoading) return <LoadingState label="正在加载上线检查" />
  if (releaseQuery.isError || !releaseQuery.data) return <ErrorState message={releaseQuery.error instanceof Error ? releaseQuery.error.message : '无法加载上线检查'} onRetry={() => releaseQuery.refetch()} />

  const data = releaseQuery.data
  const summary = data.summary
  const actionSummary = data.actionSummary
  const readyRatio = summary.requiredCount > 0 ? Math.round(summary.passedCount / summary.requiredCount * 100) : 0

  const openEvidence = (item: ReleaseEvidence) => setEvidenceDraft({
    ...item,
    sourceFingerprint: item.sourceFingerprint || summary.targetSourceFingerprint,
  })

  return (
    <div className="space-y-7">
      <PageHeader
        title="上线检查"
        description="只保留发布前必须完成的生产证据"
        actions={summary.ready ? <Badge tone="success"><CheckCircle2 className="mr-1 h-4 w-4" />满足发布门禁</Badge> : <Badge tone="warning"><ShieldAlert className="mr-1 h-4 w-4" />仍有 {summary.missingCount + summary.failedCount} 项未完成</Badge>}
      />

      <section className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <StatCard label="生产证据" value={`${summary.passedCount}/${summary.requiredCount}`} hint={`${readyRatio}% 已完成`} icon={<ClipboardCheck className="h-5 w-5" />} />
        <StatCard label="已分配任务" value={actionSummary.assignedCount} hint={`${actionSummary.unassignedCount} 项未分配`} icon={<Rocket className="h-5 w-5" />} />
        <StatCard label="已逾期" value={actionSummary.overdueCount} hint={`${actionSummary.dueSoonCount} 项即将到期`} icon={<ShieldAlert className="h-5 w-5" />} />
        <StatCard label="源码指纹" value={<span className="font-mono text-base">{summary.targetSourceFingerprint ? summary.targetSourceFingerprint.slice(0, 12) : '-'}</span>} hint={summary.sourceFingerprintAuthoritative ? `来源：${summary.sourceFingerprintSource}` : '当前构建无权威指纹'} icon={<CheckCircle2 className="h-5 w-5" />} />
      </section>

      <section className="space-y-3">
        <SectionHeader title="生产补证任务" description="负责人、截止时间和下一步动作" />
        <TableShell>
          {data.actions.length === 0 ? (
            <EmptyState icon={<ClipboardCheck className="h-5 w-5" />} title="暂无上线任务" description="发布证据迁移后会自动生成任务。" />
          ) : (
            <table className="min-w-[1080px]">
              <thead><tr><th>检查项</th><th>证据</th><th>负责人</th><th>截止时间</th><th>下一步</th><th className="w-24">操作</th></tr></thead>
              <tbody>{data.actions.map((item) => { const evidence = evidenceStatus(item.evidenceStatus); const due = dueState(item.dueState); return <tr key={item.key}><td><strong className="font-medium text-zinc-950">{item.title}</strong><div className="mt-1 text-xs text-zinc-500">{item.key}</div></td><td><Badge tone={evidence.tone}>{evidence.label}</Badge></td><td>{item.ownerUserId > 0 ? <><span>{item.ownerName || `用户 ${item.ownerUserId}`}</span><div className="mt-1 text-xs text-zinc-500">{item.ownerPhone}</div></> : <span className="text-zinc-400">未分配</span>}</td><td><Badge tone={due.tone}>{due.label}</Badge><div className="mt-1 text-xs text-zinc-500">{formatDate(item.dueAt)}</div></td><td className="max-w-xl"><span className="line-clamp-2">{item.nextAction || '-'}</span>{item.note && <div className="mt-1 line-clamp-1 text-xs text-zinc-500">{item.note}</div>}</td><td>{canManage && <Button variant="ghost" onClick={() => setActionDraft({ ...item, dueAt: dateTimeInput(item.dueAt) })}><Pencil className="h-4 w-4" />编辑</Button>}</td></tr> })}</tbody>
            </table>
          )}
        </TableShell>
      </section>

      <section className="space-y-3">
        <SectionHeader title="生产证据" description="真实目标环境、联调和浏览器验收记录" />
        <TableShell>
          <table className="min-w-[1050px]">
            <thead><tr><th>证据</th><th>状态</th><th>环境</th><th>工件</th><th>复核时间</th><th className="w-24">操作</th></tr></thead>
            <tbody>{data.evidence.map((item) => { const status = evidenceStatus(item.status); return <tr key={item.key}><td><strong className="font-medium text-zinc-950">{item.title}</strong><div className="mt-1 text-xs text-zinc-500">{item.category} / {item.key}</div></td><td><Badge tone={status.tone}>{status.label}</Badge></td><td>{item.environment || '-'}</td><td>{item.evidenceUrl ? <a className="inline-flex items-center gap-1 text-emerald-700 hover:underline" href={item.evidenceUrl} target="_blank" rel="noreferrer">查看证据<ExternalLink className="h-3.5 w-3.5" /></a> : <span className="text-zinc-400">未上传</span>}{item.artifactSha256 && <div className="mt-1 max-w-40 truncate font-mono text-xs text-zinc-500" title={item.artifactSha256}>{item.artifactSha256}</div>}</td><td>{formatDate(item.checkedAt)}</td><td>{canManage && <Button variant="ghost" onClick={() => openEvidence(item)}><Pencil className="h-4 w-4" />维护</Button>}</td></tr> })}</tbody>
          </table>
        </TableShell>
      </section>

      {canManage && (
        <section className="flex flex-col gap-3 border-t border-zinc-200 pt-6 sm:flex-row sm:items-end sm:justify-between">
          <div><h2 className="text-base font-semibold text-zinc-950">发布候选</h2><p className="mt-1 text-sm text-zinc-500">六项生产证据全部通过后生成不可变候选</p></div>
          <div className="flex flex-col gap-2 sm:flex-row"><Input value={releaseVersion} onChange={(event) => setReleaseVersion(event.target.value)} placeholder="v1.0.0" className="sm:w-44" /><Button disabled={!summary.candidateGateEnabled || !summary.metadataReady} loading={gateMutation.isPending} onClick={() => gateMutation.mutate()}>运行发布门禁</Button></div>
        </section>
      )}

      <Dialog open={Boolean(actionDraft)} onOpenChange={(open) => !open && setActionDraft(null)} title="编辑生产补证任务" description={actionDraft?.title} footer={<><Button variant="secondary" onClick={() => setActionDraft(null)}>取消</Button><Button loading={actionMutation.isPending} onClick={() => actionMutation.mutate()}>保存任务</Button></>}>
        {actionDraft && <div className="grid gap-4 sm:grid-cols-2"><Field label="负责人"><Select value={actionDraft.ownerUserId} onChange={(event) => setActionDraft((value) => value ? { ...value, ownerUserId: Number(event.target.value), dueAt: Number(event.target.value) === 0 ? '' : value.dueAt } : value)}><option value={0}>未分配</option>{data.owners.map((owner) => <option key={owner.userId} value={owner.userId}>{owner.name} / {owner.phone}</option>)}</Select></Field><Field label="截止时间"><Input type="datetime-local" value={actionDraft.dueAt} disabled={actionDraft.ownerUserId === 0} onChange={(event) => setActionDraft((value) => value ? { ...value, dueAt: event.target.value } : value)} /></Field><Field label="下一步动作" className="sm:col-span-2"><textarea className={textareaClassName} value={actionDraft.nextAction} onChange={(event) => setActionDraft((value) => value ? { ...value, nextAction: event.target.value } : value)} /></Field><Field label="协作备注" className="sm:col-span-2"><textarea className={textareaClassName} value={actionDraft.note} onChange={(event) => setActionDraft((value) => value ? { ...value, note: event.target.value } : value)} /></Field></div>}
      </Dialog>

      <Dialog open={Boolean(evidenceDraft)} onOpenChange={(open) => !open && setEvidenceDraft(null)} title="维护生产证据" description={evidenceDraft?.title} size="lg" footer={<><Button variant="secondary" onClick={() => setEvidenceDraft(null)}>取消</Button><Button loading={evidenceMutation.isPending} onClick={() => evidenceMutation.mutate()}>保存证据</Button></>}>
        {evidenceDraft && <div className="grid gap-4 sm:grid-cols-2"><Field label="状态"><Select value={evidenceDraft.status} onChange={(event) => setEvidenceDraft((value) => value ? { ...value, status: event.target.value } : value)}><option value="missing">缺失</option><option value="in_progress">进行中</option><option value="passed">已通过</option><option value="failed">未通过</option></Select></Field><Field label="执行环境"><Input value={evidenceDraft.environment} onChange={(event) => setEvidenceDraft((value) => value ? { ...value, environment: event.target.value } : value)} placeholder="production / amd64" /></Field><Field label="证据 HTTPS 地址" className="sm:col-span-2"><Input type="url" value={evidenceDraft.evidenceUrl} onChange={(event) => setEvidenceDraft((value) => value ? { ...value, evidenceUrl: event.target.value } : value)} placeholder="https://evidence.example.com/report" /></Field><Field label="源码 SHA-256" className="sm:col-span-2"><Input className="font-mono text-xs" value={evidenceDraft.sourceFingerprint} onChange={(event) => setEvidenceDraft((value) => value ? { ...value, sourceFingerprint: event.target.value } : value)} /></Field><Field label="工件 SHA-256"><Input className="font-mono text-xs" value={evidenceDraft.artifactSha256} onChange={(event) => setEvidenceDraft((value) => value ? { ...value, artifactSha256: event.target.value } : value)} /></Field><Field label="工件大小（字节）"><Input type="number" min={0} value={evidenceDraft.artifactSizeBytes || ''} onChange={(event) => setEvidenceDraft((value) => value ? { ...value, artifactSizeBytes: Number(event.target.value) } : value)} /></Field><Field label="复核备注" className="sm:col-span-2"><textarea className={textareaClassName} value={evidenceDraft.note} onChange={(event) => setEvidenceDraft((value) => value ? { ...value, note: event.target.value } : value)} /></Field></div>}
      </Dialog>
    </div>
  )
}
