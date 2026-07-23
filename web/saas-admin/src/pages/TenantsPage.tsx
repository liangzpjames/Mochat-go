import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Building2, Eye, PackageCheck, Plus, Search, UserRoundPlus } from 'lucide-react'
import { toast } from 'sonner'
import { apiRequest, executeGoverned, hasPermission } from '@/lib/api'
import type { OverviewData, PackagesData, TenantDetailData, TenantSummary } from '@/lib/types'
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
  ProgressBar,
  Select,
  SectionHeader,
  TableShell,
  formatDate,
  inputClassName,
} from '@/components/ui'
import type { PageProps } from './shared'
import { dueStateView, queryKeys, tenantStatusView, usageTone } from './shared'

interface CreateTenantForm {
  tenantName: string
  adminPhone: string
  adminName: string
  password: string
  packageCode: string
  expiresAt: string
}

const emptyCreateForm: CreateTenantForm = {
  tenantName: '',
  adminPhone: '',
  adminName: '',
  password: '',
  packageCode: '',
  expiresAt: '',
}

export default function TenantsPage({ profile, approvalMode }: PageProps) {
  const queryClient = useQueryClient()
  const canManage = hasPermission(profile.permissions, 'platform.tenants.manage')
  const [keyword, setKeyword] = useState('')
  const [statusFilter, setStatusFilter] = useState('all')
  const [packageFilter, setPackageFilter] = useState('all')
  const [createOpen, setCreateOpen] = useState(false)
  const [createForm, setCreateForm] = useState<CreateTenantForm>(emptyCreateForm)
  const [selectedTenantId, setSelectedTenantId] = useState(0)
  const [packageOpen, setPackageOpen] = useState(false)
  const [packageCode, setPackageCode] = useState('')
  const [packageExpiresAt, setPackageExpiresAt] = useState('')
  const [packageRemark, setPackageRemark] = useState('')
  const [statusOpen, setStatusOpen] = useState(false)
  const [statusRemark, setStatusRemark] = useState('')

  const overviewQuery = useQuery({
    queryKey: queryKeys.overview,
    queryFn: () => apiRequest<OverviewData>('/dashboard/saasAdmin/overview?scope=platform&expiringDays=30&riskHighUsageRatio=0.8'),
  })
  const packagesQuery = useQuery({
    queryKey: queryKeys.packages,
    queryFn: () => apiRequest<PackagesData>('/dashboard/saasAdmin/packages'),
  })
  const detailQuery = useQuery({
    queryKey: ['tenant-detail', selectedTenantId],
    queryFn: () => apiRequest<TenantDetailData>(`/dashboard/saasAdmin/tenant?tenantId=${selectedTenantId}&expiringDays=30&operationLimit=12`),
    enabled: selectedTenantId > 0,
  })

  const invalidateTenantData = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: queryKeys.overview }),
      queryClient.invalidateQueries({ queryKey: ['tenant-detail'] }),
      queryClient.invalidateQueries({ queryKey: queryKeys.operations }),
    ])
  }

  const createMutation = useMutation({
    mutationFn: async () => {
      if (!createForm.tenantName.trim()) throw new Error('请填写客户名称')
      if (!/^1\d{10}$/.test(createForm.adminPhone.trim())) throw new Error('管理员手机号格式不正确')
      if (!/^[A-Za-z0-9]{6,}$/.test(createForm.password)) throw new Error('初始密码至少 6 位，只能使用字母和数字')
      if (!createForm.packageCode) throw new Error('请选择套餐')
      const payload = {
        tenantName: createForm.tenantName.trim(),
        adminPhone: createForm.adminPhone.trim(),
        adminName: createForm.adminName.trim() || '超级管理员',
        password: createForm.password,
        packageCode: createForm.packageCode,
        expiresAt: createForm.expiresAt,
        configCopyMode: 'missing',
        remark: 'SaaS 总后台开通客户',
      }
      return executeGoverned<Record<string, unknown>>({
        approvalMode,
        actionType: 'tenant.provision',
        payload,
        reason: `开通客户租户：${payload.tenantName}`,
        directPath: '/dashboard/saasAdmin/tenantProvision',
      })
    },
    onSuccess: async (result) => {
      toast.success(result.approvalRequested ? '开户申请已提交审批' : '客户租户已开通')
      setCreateOpen(false)
      setCreateForm(emptyCreateForm)
      await invalidateTenantData()
    },
    onError: (error) => toast.error(error instanceof Error ? error.message : '开通失败'),
  })

  const packageMutation = useMutation({
    mutationFn: async () => {
      const tenant = detailQuery.data?.tenant
      if (!tenant) throw new Error('租户数据尚未加载')
      if (!packageCode) throw new Error('请选择套餐')
      const target = packagesQuery.data?.packages.find((item) => item.code === packageCode)
      const payload = {
        tenantId: tenant.tenantId,
        packageCode,
        expiresAt: packageExpiresAt,
        remark: packageRemark.trim(),
        expectedVersion: tenant.packageVersion,
      }
      return executeGoverned<Record<string, unknown>>({
        approvalMode,
        actionType: 'tenant.package.update',
        payload,
        reason: payload.remark || `将 ${tenant.tenantName} 调整为 ${target?.name || packageCode}`,
        directPath: '/dashboard/saasAdmin/tenantPackage',
      })
    },
    onSuccess: async (result) => {
      toast.success(result.approvalRequested ? '套餐调整已提交审批' : '租户套餐已更新')
      setPackageOpen(false)
      await invalidateTenantData()
    },
    onError: (error) => toast.error(error instanceof Error ? error.message : '套餐调整失败'),
  })

  const statusMutation = useMutation({
    mutationFn: async () => {
      const tenant = detailQuery.data?.tenant
      if (!tenant) throw new Error('租户数据尚未加载')
      if (!statusRemark.trim()) throw new Error('请填写状态变更原因')
      const nextStatus = tenant.tenantStatus === 1 ? 2 : 1
      const actionType = nextStatus === 2 ? 'tenant.disable' : 'tenant.enable'
      const payload = { tenantId: tenant.tenantId, status: nextStatus, remark: statusRemark.trim() }
      return executeGoverned<Record<string, unknown>>({
        approvalMode,
        actionType,
        payload,
        reason: statusRemark.trim(),
        directPath: '/dashboard/saasAdmin/tenantStatus',
      })
    },
    onSuccess: async (result) => {
      toast.success(result.approvalRequested ? '状态变更已提交审批' : '租户状态已更新')
      setStatusOpen(false)
      setStatusRemark('')
      await invalidateTenantData()
    },
    onError: (error) => toast.error(error instanceof Error ? error.message : '状态更新失败'),
  })

  const enabledPackages = (packagesQuery.data?.packages || []).filter((item) => item.status === 1)
  const tenants = overviewQuery.data?.tenants || []
  const filteredTenants = useMemo(() => tenants.filter((tenant) => {
    const normalizedKeyword = keyword.trim().toLowerCase()
    const matchesKeyword = !normalizedKeyword || tenant.tenantName.toLowerCase().includes(normalizedKeyword) || String(tenant.tenantId).includes(normalizedKeyword) || tenant.packageName.toLowerCase().includes(normalizedKeyword)
    const matchesStatus = statusFilter === 'all' || String(tenant.tenantStatus) === statusFilter
    const matchesPackage = packageFilter === 'all' || tenant.packageCode === packageFilter
    return matchesKeyword && matchesStatus && matchesPackage
  }), [keyword, packageFilter, statusFilter, tenants])

  const openCreate = () => {
    setCreateForm({ ...emptyCreateForm, packageCode: enabledPackages[0]?.code || '' })
    setCreateOpen(true)
  }

  const openTenant = (tenant: TenantSummary) => setSelectedTenantId(tenant.tenantId)
  const selectedTenant = detailQuery.data?.tenant
  const coreMetrics = (detailQuery.data?.metrics || []).filter((item) => ['users', 'corps', 'contacts', 'rooms', 'storage_mb', 'channel_codes'].includes(item.metric))

  if (overviewQuery.isLoading || packagesQuery.isLoading) return <LoadingState label="正在加载客户租户" />
  if (overviewQuery.isError || !overviewQuery.data) return <ErrorState message={overviewQuery.error instanceof Error ? overviewQuery.error.message : '无法加载客户租户'} onRetry={() => overviewQuery.refetch()} />

  return (
    <div className="space-y-7">
      <PageHeader title="客户租户" description="开通客户并维护账号、套餐、状态和用量" actions={canManage ? <Button onClick={openCreate}><Plus className="h-4 w-4" />开通客户</Button> : undefined} />

      <section className="flex flex-col gap-3 rounded-lg border border-zinc-200 bg-white p-3 lg:flex-row lg:items-center" aria-label="客户筛选">
        <label className="relative min-w-0 flex-1">
          <Search className="pointer-events-none absolute left-3 top-2.5 h-4 w-4 text-zinc-400" />
          <Input value={keyword} onChange={(event) => setKeyword(event.target.value)} className="pl-9" placeholder="搜索客户名称、ID 或套餐" />
        </label>
        <Select value={statusFilter} onChange={(event) => setStatusFilter(event.target.value)} className="lg:w-36"><option value="all">全部状态</option><option value="1">正常</option><option value="2">已停用</option></Select>
        <Select value={packageFilter} onChange={(event) => setPackageFilter(event.target.value)} className="lg:w-44"><option value="all">全部套餐</option>{enabledPackages.map((item) => <option key={item.code} value={item.code}>{item.name}</option>)}</Select>
        <span className="px-1 text-sm text-zinc-500">{filteredTenants.length} 个客户</span>
      </section>

      <section className="space-y-3">
        <SectionHeader title="客户列表" />
        <TableShell>
          {filteredTenants.length === 0 ? (
            <EmptyState
              icon={<Building2 className="h-5 w-5" />}
              title={tenants.length === 0 ? '还没有客户租户' : '没有匹配的客户'}
              description={tenants.length === 0 ? '签约首个客户后，在这里一次完成账号与套餐开通。' : '调整搜索条件后重试。'}
              action={tenants.length === 0 && canManage ? <Button onClick={openCreate}><UserRoundPlus className="h-4 w-4" />开通首个客户</Button> : undefined}
            />
          ) : (
            <table className="min-w-[900px]">
              <thead><tr><th>客户</th><th>套餐与到期</th><th>状态</th><th>最高用量</th><th>告警</th><th className="w-20">操作</th></tr></thead>
              <tbody>
                {filteredTenants.map((tenant) => {
                  const status = tenantStatusView(tenant.tenantStatus)
                  const due = dueStateView(tenant)
                  const ratio = Math.round((tenant.maxUsageRatio || 0) * 100)
                  return (
                    <tr key={tenant.tenantId}>
                      <td><strong className="font-medium text-zinc-950">{tenant.tenantName}</strong><div className="mt-1 text-xs text-zinc-500">租户 ID {tenant.tenantId}</div></td>
                      <td><div>{tenant.packageName || '未配置'}</div><div className="mt-1 text-xs text-zinc-500">{tenant.expiresAt || '长期有效'}</div></td>
                      <td><div className="flex flex-wrap gap-1"><Badge tone={status.tone}>{status.label}</Badge><Badge tone={due.tone}>{due.label}</Badge></div></td>
                      <td><div className="flex min-w-40 items-center justify-between gap-3 text-xs"><span>{tenant.maxUsageLabel || '-'}</span><span>{ratio}%</span></div><div className="mt-2"><ProgressBar value={ratio} tone={usageTone(tenant.maxUsageRatio || 0)} /></div></td>
                      <td>{tenant.openAlertCount > 0 ? <Badge tone="danger">{tenant.openAlertCount} 条</Badge> : <Badge tone="success">正常</Badge>}</td>
                      <td><Button variant="ghost" onClick={() => openTenant(tenant)}><Eye className="h-4 w-4" />详情</Button></td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          )}
        </TableShell>
      </section>

      <Dialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        title="开通客户租户"
        description="创建客户账号并绑定首个套餐"
        footer={<><Button variant="secondary" onClick={() => setCreateOpen(false)}>取消</Button><Button loading={createMutation.isPending} onClick={() => createMutation.mutate()}>{approvalMode.required ? '提交开户' : '确认开通'}</Button></>}
      >
        <div className="grid gap-4 sm:grid-cols-2">
          <Field label="客户名称" className="sm:col-span-2"><Input value={createForm.tenantName} onChange={(event) => setCreateForm((form) => ({ ...form, tenantName: event.target.value }))} placeholder="客户公司名称" autoFocus /></Field>
          <Field label="管理员姓名"><Input value={createForm.adminName} onChange={(event) => setCreateForm((form) => ({ ...form, adminName: event.target.value }))} placeholder="超级管理员" /></Field>
          <Field label="管理员手机号"><Input value={createForm.adminPhone} onChange={(event) => setCreateForm((form) => ({ ...form, adminPhone: event.target.value }))} inputMode="numeric" maxLength={11} placeholder="11 位手机号" /></Field>
          <Field label="初始密码" hint="至少 6 位字母或数字"><Input type="password" value={createForm.password} onChange={(event) => setCreateForm((form) => ({ ...form, password: event.target.value }))} autoComplete="new-password" /></Field>
          <Field label="销售套餐"><Select value={createForm.packageCode} onChange={(event) => setCreateForm((form) => ({ ...form, packageCode: event.target.value }))}><option value="">请选择</option>{enabledPackages.map((item) => <option key={item.code} value={item.code}>{item.name}</option>)}</Select></Field>
          <Field label="到期日期" className="sm:col-span-2"><Input type="date" value={createForm.expiresAt} onChange={(event) => setCreateForm((form) => ({ ...form, expiresAt: event.target.value }))} /></Field>
        </div>
      </Dialog>

      <Dialog open={selectedTenantId > 0} onOpenChange={(open) => !open && setSelectedTenantId(0)} title={selectedTenant?.tenantName || '客户详情'} description={selectedTenant ? `租户 ID ${selectedTenant.tenantId}` : '正在加载'} size="lg">
        {detailQuery.isLoading && <LoadingState label="正在加载客户详情" />}
        {detailQuery.isError && <ErrorState message={detailQuery.error instanceof Error ? detailQuery.error.message : '无法加载客户详情'} onRetry={() => detailQuery.refetch()} />}
        {selectedTenant && (
          <div className="space-y-6">
            <div className="grid gap-3 sm:grid-cols-3">
              <div className="rounded-md border border-zinc-200 p-3"><span className="text-xs text-zinc-500">当前套餐</span><strong className="mt-1 block text-sm">{selectedTenant.packageName || '未配置'}</strong></div>
              <div className="rounded-md border border-zinc-200 p-3"><span className="text-xs text-zinc-500">账号状态</span><div className="mt-1"><Badge tone={tenantStatusView(selectedTenant.tenantStatus).tone}>{tenantStatusView(selectedTenant.tenantStatus).label}</Badge></div></div>
              <div className="rounded-md border border-zinc-200 p-3"><span className="text-xs text-zinc-500">套餐到期</span><strong className="mt-1 block text-sm">{selectedTenant.expiresAt || '长期有效'}</strong></div>
            </div>
            {canManage && <div className="flex flex-wrap gap-2"><Button variant="secondary" onClick={() => { setPackageCode(selectedTenant.packageCode); setPackageExpiresAt(selectedTenant.expiresAt || ''); setPackageRemark(''); setPackageOpen(true) }}><PackageCheck className="h-4 w-4" />调整套餐</Button><Button variant={selectedTenant.tenantStatus === 1 ? 'danger' : 'secondary'} onClick={() => setStatusOpen(true)}>{selectedTenant.tenantStatus === 1 ? '停用租户' : '启用租户'}</Button></div>}
            <section className="space-y-3"><SectionHeader title="核心用量" /><div className="grid gap-3 sm:grid-cols-2">{coreMetrics.map((metric) => { const ratio = metric.limit > 0 ? metric.current / metric.limit : 0; return <div key={metric.metric} className="rounded-md border border-zinc-200 p-3"><div className="flex items-center justify-between gap-3 text-sm"><span>{metric.label}</span><strong>{metric.current} / {metric.limit}</strong></div><div className="mt-3"><ProgressBar value={ratio * 100} tone={usageTone(ratio)} /></div></div> })}</div></section>
            <section className="space-y-3"><SectionHeader title="最近变更" /><TableShell>{(detailQuery.data?.operations || []).length === 0 ? <EmptyState icon={<Building2 className="h-5 w-5" />} title="暂无变更记录" description="租户变更会自动记录。" /> : <table><thead><tr><th>目标</th><th>说明</th><th>时间</th></tr></thead><tbody>{(detailQuery.data?.operations || []).slice(0, 8).map((item) => <tr key={item.id}><td>{item.targetName || item.targetType}</td><td>{item.remark || item.action}</td><td>{formatDate(item.createdAt)}</td></tr>)}</tbody></table>}</TableShell></section>
          </div>
        )}
      </Dialog>

      <Dialog open={packageOpen} onOpenChange={setPackageOpen} title="调整客户套餐" description={selectedTenant?.tenantName} footer={<><Button variant="secondary" onClick={() => setPackageOpen(false)}>取消</Button><Button loading={packageMutation.isPending} onClick={() => packageMutation.mutate()}>{approvalMode.required ? '提交调整' : '确认调整'}</Button></>}>
        <div className="grid gap-4 sm:grid-cols-2"><Field label="套餐"><Select value={packageCode} onChange={(event) => setPackageCode(event.target.value)}>{enabledPackages.map((item) => <option key={item.code} value={item.code}>{item.name}</option>)}</Select></Field><Field label="到期日期"><Input type="date" value={packageExpiresAt} onChange={(event) => setPackageExpiresAt(event.target.value)} /></Field><Field label="变更说明" className="sm:col-span-2"><Input value={packageRemark} onChange={(event) => setPackageRemark(event.target.value)} placeholder="升级、降级或权益调整原因" /></Field></div>
      </Dialog>

      <Dialog open={statusOpen} onOpenChange={setStatusOpen} title={selectedTenant?.tenantStatus === 1 ? '停用客户租户' : '启用客户租户'} description={selectedTenant?.tenantName} size="sm" footer={<><Button variant="secondary" onClick={() => setStatusOpen(false)}>取消</Button><Button variant={selectedTenant?.tenantStatus === 1 ? 'danger' : 'primary'} loading={statusMutation.isPending} onClick={() => statusMutation.mutate()}>确认变更</Button></>}>
        <Field label="变更原因"><textarea className={inputClassName.replace('h-9', 'min-h-24 py-2')} value={statusRemark} onChange={(event) => setStatusRemark(event.target.value)} placeholder="填写停用或恢复原因" /></Field>
      </Dialog>
    </div>
  )
}
