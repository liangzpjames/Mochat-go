import { useQuery } from '@tanstack/react-query'
import {
  Activity,
  AlertTriangle,
  ArrowRight,
  Building2,
  Check,
  Circle,
  Database,
  PackageCheck,
  Rocket,
  ServerCog,
} from 'lucide-react'
import { apiRequest } from '@/lib/api'
import type { OperationsData, OverviewData, ReleaseReadinessData, SystemHealthData } from '@/lib/types'
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
  formatDate,
} from '@/components/ui'
import type { PageProps } from './shared'
import { dueStateView, queryKeys, tenantStatusView } from './shared'

const actionLabels: Record<string, string> = {
  'saas.admin.tenant.provision': '开通租户',
  'saas.admin.tenant.package.update': '调整套餐',
  'saas.admin.tenant.status.update': '变更租户状态',
  'saas.admin.package.upsert': '维护套餐',
  'saas.admin.system.health.scan': '系统健康扫描',
  'saas.admin.backup.create': '创建备份',
  'saas.admin.release.evidence.action.save': '更新上线任务',
}

function operationLabel(action: string) {
  if (actionLabels[action]) return actionLabels[action]
  if (action.includes('backup')) return '备份与恢复'
  if (action.includes('release')) return '上线检查'
  if (action.includes('audit')) return '审计检查'
  if (action.includes('credential')) return '凭据维护'
  return '平台操作'
}

export default function OverviewPage({ navigate }: PageProps) {
  const overviewQuery = useQuery({
    queryKey: queryKeys.overview,
    queryFn: () => apiRequest<OverviewData>('/dashboard/saasAdmin/overview?scope=platform&expiringDays=30&riskHighUsageRatio=0.8'),
  })
  const healthQuery = useQuery({
    queryKey: queryKeys.systemHealth,
    queryFn: () => apiRequest<SystemHealthData>('/dashboard/saasAdmin/systemHealth?failureWindowHours=24&notificationStaleMinutes=15'),
  })
  const releaseQuery = useQuery({
    queryKey: queryKeys.release,
    queryFn: () => apiRequest<ReleaseReadinessData>('/dashboard/saasAdmin/releaseReadiness'),
  })
  const operationsQuery = useQuery({
    queryKey: queryKeys.operations,
    queryFn: () => apiRequest<OperationsData>('/dashboard/saasAdmin/operations?limit=12'),
  })

  if (overviewQuery.isLoading) return <LoadingState label="正在加载平台概览" />
  if (overviewQuery.isError || !overviewQuery.data) {
    return <ErrorState message={overviewQuery.error instanceof Error ? overviewQuery.error.message : '无法加载平台概览'} onRetry={() => overviewQuery.refetch()} />
  }

  const data = overviewQuery.data
  const summary = data.summary
  const health = healthQuery.data?.summary
  const release = releaseQuery.data
  const operations = operationsQuery.data?.operations || []
  const setupItems = [
    { label: '平台服务', complete: health?.healthState === 'healthy', detail: health ? `${health.healthyCount}/${health.checkCount} 项正常` : '等待检查' },
    { label: '销售套餐', complete: summary.enabledPackageCount > 0, detail: `${summary.enabledPackageCount} 个已启用` },
    { label: '生产证据', complete: Boolean(release?.summary.ready), detail: release ? `${release.summary.passedCount}/${release.summary.requiredCount} 项通过` : '等待检查' },
    { label: '客户租户', complete: summary.tenantCount > 0, detail: summary.tenantCount > 0 ? `${summary.tenantCount} 个客户` : '等待首个客户' },
  ]
  const coreCheckCodes = new Set(['database_connection', 'redis_connection', 'schema_migration', 'background_tasks_failed', 'backup_freshness', 'backup_encryption'])
  const coreChecks = (healthQuery.data?.checks || []).filter((item) => coreCheckCodes.has(item.code))

  return (
    <div className="space-y-7">
      <PageHeader
        title="平台概览"
        description="客户、套餐、系统和上线状态"
        actions={<Button onClick={() => navigate('tenants')}><Building2 className="h-4 w-4" />开通客户</Button>}
      />

      <section className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4" aria-label="平台关键指标">
        <StatCard label="客户租户" value={summary.tenantCount} hint={`${summary.activeTenantPackageCount} 个有效套餐`} icon={<Building2 className="h-5 w-5" />} />
        <StatCard label="已启用套餐" value={summary.enabledPackageCount} hint="当前可销售" icon={<PackageCheck className="h-5 w-5" />} />
        <StatCard label="即将到期" value={summary.expiringSoonTenantCount} hint={`${summary.expiredTenantCount} 个已到期`} icon={<AlertTriangle className="h-5 w-5" />} />
        <StatCard label="开放告警" value={summary.openAlertCount} hint={health?.healthState === 'healthy' ? '平台服务正常' : `${health?.issueCount || 0} 项系统异常`} icon={<Activity className="h-5 w-5" />} />
      </section>

      <section className="border-y border-zinc-200 bg-white px-4 py-5 sm:px-5" aria-label="首版上线进度">
        <SectionHeader title="上线进度" actions={<Button variant="secondary" onClick={() => navigate('launch')}>查看检查项<ArrowRight className="h-4 w-4" /></Button>} />
        <div className="mt-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
          {setupItems.map((item) => (
            <div key={item.label} className="flex items-center gap-3 rounded-md border border-zinc-200 px-3 py-3">
              <span className={item.complete ? 'text-emerald-700' : 'text-zinc-300'}>{item.complete ? <Check className="h-5 w-5" /> : <Circle className="h-5 w-5" />}</span>
              <div><strong className="block text-sm font-medium text-zinc-900">{item.label}</strong><span className="text-xs text-zinc-500">{item.detail}</span></div>
            </div>
          ))}
        </div>
      </section>

      <div className="grid grid-cols-[minmax(0,1fr)] gap-7 xl:grid-cols-[minmax(0,1.35fr)_minmax(320px,.65fr)]">
        <section className="min-w-0 space-y-3">
          <SectionHeader title="客户租户" description="最近需要关注的客户" actions={<Button variant="ghost" onClick={() => navigate('tenants')}>全部客户<ArrowRight className="h-4 w-4" /></Button>} />
          <TableShell>
            {data.tenants.length === 0 ? (
              <EmptyState icon={<Building2 className="h-5 w-5" />} title="还没有客户租户" description="客户确认后可从这里一次完成账号、套餐和到期时间配置。" action={<Button onClick={() => navigate('tenants')}>开通首个客户</Button>} />
            ) : (
              <table>
                <thead><tr><th>客户</th><th>套餐</th><th>状态</th><th>最高用量</th></tr></thead>
                <tbody>
                  {data.tenants.slice(0, 8).map((tenant) => {
                    const status = tenantStatusView(tenant.tenantStatus)
                    const due = dueStateView(tenant)
                    return (
                      <tr key={tenant.tenantId}>
                        <td><strong className="font-medium text-zinc-900">{tenant.tenantName}</strong><div className="mt-1 text-xs text-zinc-500">ID {tenant.tenantId}</div></td>
                        <td>{tenant.packageName || '未配置'}<div className="mt-1 text-xs text-zinc-500">{tenant.expiresAt || '长期有效'}</div></td>
                        <td><div className="flex flex-wrap gap-1"><Badge tone={status.tone}>{status.label}</Badge><Badge tone={due.tone}>{due.label}</Badge></div></td>
                        <td>{tenant.maxUsageLabel || '-'}<div className="mt-1 text-xs text-zinc-500">{tenant.maxUsageCurrent || 0} / {tenant.maxUsageLimit || 0}</div></td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            )}
          </TableShell>
        </section>

        <section className="space-y-3">
          <SectionHeader title="核心服务" description="上线必需的运行状态" actions={<Button variant="ghost" onClick={() => navigate('system')}>运维详情<ArrowRight className="h-4 w-4" /></Button>} />
          <div className="divide-y divide-zinc-200 rounded-lg border border-zinc-200 bg-white">
            {healthQuery.isLoading && <LoadingState />}
            {!healthQuery.isLoading && coreChecks.length === 0 && <EmptyState icon={<ServerCog className="h-5 w-5" />} title="暂无健康数据" description="运行一次系统扫描后显示结果。" />}
            {coreChecks.map((check) => (
              <div key={check.code} className="flex items-start gap-3 px-4 py-3">
                <span className={check.status === 'healthy' ? 'mt-0.5 text-emerald-700' : 'mt-0.5 text-red-600'}>{check.category === 'database' || check.category === 'runtime' ? <Database className="h-4 w-4" /> : <ServerCog className="h-4 w-4" />}</span>
                <div className="min-w-0 flex-1"><strong className="block text-sm font-medium text-zinc-900">{check.name}</strong><p className="mt-0.5 text-xs leading-5 text-zinc-500">{check.detail}</p></div>
                <Badge tone={check.status === 'healthy' ? 'success' : check.severity === 'critical' ? 'danger' : 'warning'}>{check.status === 'healthy' ? '正常' : '异常'}</Badge>
              </div>
            ))}
          </div>
        </section>
      </div>

      <section className="space-y-3">
        <SectionHeader title="最近操作" description="平台级变更与检查记录" />
        {operations.length === 0 ? (
          <TableShell>
            <EmptyState icon={<Activity className="h-5 w-5" />} title="暂无平台操作" description="平台变更会自动记录在这里。" />
          </TableShell>
        ) : (
          <>
            <div className="divide-y divide-zinc-200 rounded-lg border border-zinc-200 bg-white sm:hidden">
              {operations.slice(0, 8).map((item) => (
                <article key={item.id} className="space-y-3 px-4 py-4">
                  <div className="flex items-start justify-between gap-4">
                    <strong className="text-sm font-medium text-zinc-900">{operationLabel(item.action)}</strong>
                    <span className="shrink-0 text-xs text-zinc-500">{formatDate(item.createdAt)}</span>
                  </div>
                  <p className="break-words text-xs leading-5 text-zinc-500">{item.remark || item.action}</p>
                  <dl className="grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-1 text-xs">
                    <dt className="text-zinc-400">目标</dt>
                    <dd className="break-words text-zinc-700">{item.targetName || item.targetType || '-'}</dd>
                    <dt className="text-zinc-400">执行人</dt>
                    <dd className="text-zinc-700">{item.actorUserId > 0 ? `用户 ${item.actorUserId}` : '系统'}</dd>
                  </dl>
                </article>
              ))}
            </div>
            <TableShell className="hidden sm:block">
              <table className="min-w-[760px]">
                <thead><tr><th>操作</th><th>目标</th><th>执行人</th><th>时间</th></tr></thead>
                <tbody>{operations.slice(0, 8).map((item) => <tr key={item.id}><td><strong className="font-medium text-zinc-900">{operationLabel(item.action)}</strong><div className="mt-1 max-w-xl truncate text-xs text-zinc-500">{item.remark || item.action}</div></td><td>{item.targetName || item.targetType || '-'}</td><td>{item.actorUserId > 0 ? `用户 ${item.actorUserId}` : '系统'}</td><td>{formatDate(item.createdAt)}</td></tr>)}</tbody>
              </table>
            </TableShell>
          </>
        )}
      </section>
    </div>
  )
}
