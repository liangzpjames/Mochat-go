import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Activity, CheckCircle2, Database, HardDrive, RefreshCw, ServerCog, ShieldAlert } from 'lucide-react'
import { toast } from 'sonner'
import { apiRequest, hasPermission, jsonRequest } from '@/lib/api'
import type { BackupOverviewData, OperationsData, SystemHealthData } from '@/lib/types'
import {
  Badge,
  Button,
  EmptyState,
  ErrorState,
  LoadingState,
  PageHeader,
  SectionHeader,
  StatCard,
  TableShell,
  formatBytes,
  formatDate,
} from '@/components/ui'
import type { PageProps } from './shared'
import { queryKeys } from './shared'

const coreCheckCodes = new Set([
  'database_connection',
  'redis_connection',
  'schema_migration',
  'background_tasks_failed',
  'background_executions_failed',
  'backup_automation',
  'backup_encryption',
  'backup_failures',
  'backup_freshness',
  'restore_drill_freshness',
  'wecom_credential_protection',
  'wechat_open_credential_protection',
])

function actionName(action: string) {
  if (action.includes('system.health')) return '系统健康扫描'
  if (action.includes('backup')) return '数据库备份'
  if (action.includes('tenant.provision')) return '客户开户'
  if (action.includes('package')) return '套餐变更'
  if (action.includes('release')) return '上线检查'
  if (action.includes('credential')) return '凭据维护'
  if (action.includes('audit')) return '审计检查'
  return '平台操作'
}

export default function SystemPage({ profile }: PageProps) {
  const queryClient = useQueryClient()
  const canManageSystem = hasPermission(profile.permissions, 'platform.system.manage')
  const canManageBackups = hasPermission(profile.permissions, 'platform.backups.manage')

  const healthQuery = useQuery({
    queryKey: queryKeys.systemHealth,
    queryFn: () => apiRequest<SystemHealthData>('/dashboard/saasAdmin/systemHealth?failureWindowHours=24&notificationStaleMinutes=15'),
  })
  const backupQuery = useQuery({
    queryKey: queryKeys.backup,
    queryFn: () => apiRequest<BackupOverviewData>('/dashboard/saasAdmin/backupOverview?limit=20'),
    enabled: hasPermission(profile.permissions, 'platform.backups.read'),
  })
  const operationsQuery = useQuery({
    queryKey: queryKeys.operations,
    queryFn: () => apiRequest<OperationsData>('/dashboard/saasAdmin/operations?limit=40'),
    enabled: hasPermission(profile.permissions, 'platform.audit.read'),
  })

  const scanMutation = useMutation({
    mutationFn: () => apiRequest<Record<string, number>>('/dashboard/saasAdmin/systemHealthScan', jsonRequest('POST', { failureWindowHours: 24, notificationStaleMinutes: 15, notify: true })),
    onSuccess: async (data) => {
      toast.success(`扫描完成：新增 ${data.openedCount || 0}，恢复 ${data.recoveredCount || 0}`)
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.systemHealth }),
        queryClient.invalidateQueries({ queryKey: ['system-health', 'header'] }),
        queryClient.invalidateQueries({ queryKey: queryKeys.operations }),
      ])
    },
    onError: (error) => toast.error(error instanceof Error ? error.message : '系统扫描失败'),
  })

  const backupMutation = useMutation({
    mutationFn: () => apiRequest<Record<string, unknown>>('/dashboard/saasAdmin/backupRun', jsonRequest('POST', { action: 'create' })),
    onSuccess: async () => {
      toast.success('加密备份创建并校验完成')
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.backup }),
        queryClient.invalidateQueries({ queryKey: queryKeys.systemHealth }),
        queryClient.invalidateQueries({ queryKey: queryKeys.operations }),
      ])
    },
    onError: (error) => toast.error(error instanceof Error ? error.message : '备份创建失败'),
  })

  if (healthQuery.isLoading) return <LoadingState label="正在加载系统状态" />
  if (healthQuery.isError || !healthQuery.data) return <ErrorState message={healthQuery.error instanceof Error ? healthQuery.error.message : '无法加载系统状态'} onRetry={() => healthQuery.refetch()} />

  const health = healthQuery.data
  const checks = health.checks.filter((item) => coreCheckCodes.has(item.code) || item.status !== 'healthy')
  const backup = backupQuery.data
  const latestBackup = backup?.runs?.[0]
  const operations = operationsQuery.data?.operations || []

  return (
    <div className="space-y-7">
      <PageHeader
        title="系统运维"
        description="运行状态、数据库备份和平台审计"
        actions={<>{canManageBackups && <Button variant="secondary" loading={backupMutation.isPending} onClick={() => backupMutation.mutate()}><HardDrive className="h-4 w-4" />立即备份</Button>}{canManageSystem && <Button loading={scanMutation.isPending} onClick={() => scanMutation.mutate()}><RefreshCw className="h-4 w-4" />运行扫描</Button>}</>}
      />

      <section className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <StatCard label="健康检查" value={`${health.summary.healthyCount}/${health.summary.checkCount}`} hint={health.summary.healthState === 'healthy' ? '全部正常' : `${health.summary.issueCount} 项异常`} icon={<CheckCircle2 className="h-5 w-5" />} />
        <StatCard label="严重问题" value={health.summary.criticalCount} hint={`${health.summary.warningCount} 项预警`} icon={<ShieldAlert className="h-5 w-5" />} />
        <StatCard label="成功备份" value={backup?.summary.successfulCount ?? '-'} hint={`${backup?.summary.verifiedCount ?? 0} 份已校验`} icon={<Database className="h-5 w-5" />} />
        <StatCard label="恢复演练" value={backup?.summary.successfulDrillCount ?? '-'} hint={latestBackup ? `最近 ${formatDate(latestBackup.finishedAt)}` : '暂无备份'} icon={<ServerCog className="h-5 w-5" />} />
      </section>

      <section className="space-y-3">
        <SectionHeader title="核心服务" description="数据库、缓存、任务、备份和外部凭据" />
        <TableShell>
          <table className="min-w-[820px]">
            <thead><tr><th>检查项</th><th>分类</th><th>状态</th><th>结果</th></tr></thead>
            <tbody>{checks.map((check) => <tr key={check.code}><td><strong className="font-medium text-zinc-950">{check.name}</strong><div className="mt-1 text-xs text-zinc-500">{check.code}</div></td><td>{check.category}</td><td><Badge tone={check.status === 'healthy' ? 'success' : check.severity === 'critical' ? 'danger' : 'warning'}>{check.status === 'healthy' ? '正常' : check.severity === 'critical' ? '严重' : '预警'}</Badge></td><td className="max-w-2xl text-zinc-600">{check.detail}</td></tr>)}</tbody>
          </table>
        </TableShell>
      </section>

      <section className="space-y-3">
        <SectionHeader title="数据库备份" description="最近加密备份与校验结果" />
        <TableShell>
          {backupQuery.isLoading ? <LoadingState /> : !backup?.runs?.length ? (
            <EmptyState icon={<HardDrive className="h-5 w-5" />} title="还没有数据库备份" description="创建首份加密备份后显示校验结果。" action={canManageBackups ? <Button onClick={() => backupMutation.mutate()}>创建首份备份</Button> : undefined} />
          ) : (
            <table className="min-w-[900px]">
              <thead><tr><th>备份</th><th>状态</th><th>结构</th><th>工件</th><th>完成时间</th></tr></thead>
              <tbody>{backup.runs.slice(0, 8).map((run) => <tr key={run.id}><td><strong className="font-medium text-zinc-950">{run.backupNo}</strong><div className="mt-1 max-w-xs truncate text-xs text-zinc-500">{run.artifactName}</div></td><td><div className="flex flex-wrap gap-1"><Badge tone={run.status === 'succeeded' ? 'success' : run.status === 'failed' ? 'danger' : 'warning'}>{run.status === 'succeeded' ? '成功' : run.status}</Badge><Badge tone={run.verificationStatus === 'passed' ? 'success' : 'warning'}>{run.verificationStatus === 'passed' ? '已校验' : '待校验'}</Badge></div></td><td>{run.tableCount} 张表<div className="mt-1 text-xs text-zinc-500">{run.migrationVersion}</div></td><td>{formatBytes(run.sizeBytes)}<div className="mt-1 max-w-36 truncate font-mono text-xs text-zinc-500" title={run.sha256}>{run.sha256 || '-'}</div></td><td>{formatDate(run.finishedAt)}</td></tr>)}</tbody>
            </table>
          )}
        </TableShell>
      </section>

      <section className="space-y-3">
        <SectionHeader title="平台操作记录" description="总后台关键变更审计" />
        <TableShell>
          {operations.length === 0 ? (
            <EmptyState icon={<Activity className="h-5 w-5" />} title="暂无操作记录" description="平台变更会自动记录。" />
          ) : (
            <table className="min-w-[900px]"><thead><tr><th>操作</th><th>目标</th><th>执行人</th><th>说明</th><th>时间</th></tr></thead><tbody>{operations.map((item) => <tr key={item.id}><td><strong className="font-medium text-zinc-950">{actionName(item.action)}</strong><div className="mt-1 max-w-56 truncate text-xs text-zinc-500">{item.action}</div></td><td>{item.targetName || item.targetType || '-'}</td><td>{item.actorUserId > 0 ? `用户 ${item.actorUserId}` : '系统'}</td><td className="max-w-xl">{item.remark || '-'}</td><td>{formatDate(item.createdAt)}</td></tr>)}</tbody></table>
          )}
        </TableShell>
      </section>
    </div>
  )
}
