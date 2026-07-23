import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Boxes, Copy, Pencil, Plus } from 'lucide-react'
import { toast } from 'sonner'
import { apiRequest, executeGoverned, hasPermission } from '@/lib/api'
import type { PackagePlan, PackagesData } from '@/lib/types'
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
  Select,
  StatCard,
  TableShell,
} from '@/components/ui'
import type { PageProps } from './shared'
import { coreLimitFields, queryKeys } from './shared'

interface PackageDraft {
  code: string
  name: string
  description: string
  status: number
  version: number
  limits: Record<string, number>
  isNew: boolean
}

function emptyDraft(template?: PackagePlan): PackageDraft {
  const limits = template ? { ...template.limits } : Object.fromEntries(coreLimitFields.map((item) => [item.key, item.defaultValue]))
  return { code: '', name: '', description: '', status: 1, version: 0, limits, isNew: true }
}

export default function PackagesPage({ profile, approvalMode }: PageProps) {
  const queryClient = useQueryClient()
  const canManage = hasPermission(profile.permissions, 'platform.tenants.manage')
  const [dialogOpen, setDialogOpen] = useState(false)
  const [draft, setDraft] = useState<PackageDraft>(emptyDraft())

  const packagesQuery = useQuery({
    queryKey: queryKeys.packages,
    queryFn: () => apiRequest<PackagesData>('/dashboard/saasAdmin/packages'),
  })

  const saveMutation = useMutation({
    mutationFn: async () => {
      const code = draft.code.trim()
      const name = draft.name.trim()
      if (!/^[A-Za-z0-9_-]{2,64}$/.test(code)) throw new Error('套餐编码需为 2-64 位字母、数字、中划线或下划线')
      if (!name) throw new Error('请填写套餐名称')
      const limits = Object.fromEntries(Object.entries(draft.limits).map(([key, value]) => [key, Math.max(0, Math.trunc(Number(value) || 0))]))
      const payload = { code, name, description: draft.description.trim(), status: draft.status, limits, expectedVersion: draft.version }
      return executeGoverned<Record<string, unknown>>({
        approvalMode,
        actionType: 'package.upsert',
        payload,
        reason: `${draft.isNew ? '创建' : '更新'}平台套餐：${name}`,
        directPath: '/dashboard/saasAdmin/package',
      })
    },
    onSuccess: async (result) => {
      toast.success(result.approvalRequested ? '套餐变更已提交审批' : '套餐已保存')
      setDialogOpen(false)
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.packages }),
        queryClient.invalidateQueries({ queryKey: queryKeys.overview }),
        queryClient.invalidateQueries({ queryKey: queryKeys.operations }),
      ])
    },
    onError: (error) => toast.error(error instanceof Error ? error.message : '套餐保存失败'),
  })

  if (packagesQuery.isLoading) return <LoadingState label="正在加载套餐" />
  if (packagesQuery.isError || !packagesQuery.data) return <ErrorState message={packagesQuery.error instanceof Error ? packagesQuery.error.message : '无法加载套餐'} onRetry={() => packagesQuery.refetch()} />

  const packages = packagesQuery.data.packages || []
  const enabledCount = packages.filter((item) => item.status === 1).length

  const openCreate = (template?: PackagePlan) => {
    const next = emptyDraft(template || packages.find((item) => item.status === 1))
    if (template) {
      next.name = `${template.name}副本`
      next.description = template.description
    }
    setDraft(next)
    setDialogOpen(true)
  }

  const openEdit = (plan: PackagePlan) => {
    setDraft({ code: plan.code, name: plan.name, description: plan.description, status: plan.status, version: plan.version, limits: { ...plan.limits }, isNew: false })
    setDialogOpen(true)
  }

  return (
    <div className="space-y-7">
      <PageHeader title="销售套餐" description="维护首版可销售的权益与核心额度" actions={canManage ? <Button onClick={() => openCreate()}><Plus className="h-4 w-4" />新建套餐</Button> : undefined} />

      <section className="grid gap-4 sm:grid-cols-3" aria-label="套餐指标">
        <StatCard label="套餐总数" value={packages.length} hint="平台套餐定义" icon={<Boxes className="h-5 w-5" />} />
        <StatCard label="已启用" value={enabledCount} hint="当前可用于开户" icon={<Boxes className="h-5 w-5" />} />
        <StatCard label="已停用" value={packages.length - enabledCount} hint="不会出现在开户选项" icon={<Boxes className="h-5 w-5" />} />
      </section>

      <TableShell>
        {packages.length === 0 ? (
          <EmptyState icon={<Boxes className="h-5 w-5" />} title="还没有销售套餐" description="创建首个套餐后即可开通客户租户。" action={canManage ? <Button onClick={() => openCreate()}>创建首个套餐</Button> : undefined} />
        ) : (
          <table className="min-w-[980px]">
            <thead><tr><th>套餐</th><th>核心额度</th><th>状态</th><th>版本</th><th className="w-44">操作</th></tr></thead>
            <tbody>
              {packages.map((plan) => (
                <tr key={plan.code}>
                  <td><strong className="font-medium text-zinc-950">{plan.name}</strong><div className="mt-1 text-xs text-zinc-500">{plan.code}</div>{plan.description && <div className="mt-1 max-w-md text-xs text-zinc-500">{plan.description}</div>}</td>
                  <td><div className="grid grid-cols-2 gap-x-5 gap-y-1 text-xs text-zinc-600">{coreLimitFields.slice(0, 6).map((field) => <span key={field.key}><span className="text-zinc-400">{field.label}</span> {plan.limits[field.key] ?? 0}</span>)}</div></td>
                  <td><Badge tone={plan.status === 1 ? 'success' : 'neutral'}>{plan.status === 1 ? '已启用' : '已停用'}</Badge></td>
                  <td>v{plan.version}</td>
                  <td>{canManage && <div className="flex gap-1"><Button variant="ghost" onClick={() => openEdit(plan)}><Pencil className="h-4 w-4" />编辑</Button><Button variant="ghost" onClick={() => openCreate(plan)}><Copy className="h-4 w-4" />复制</Button></div>}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </TableShell>

      <Dialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        title={draft.isNew ? '新建销售套餐' : '编辑销售套餐'}
        description="首版仅维护销售和开户真正需要的核心额度"
        size="lg"
        footer={<><Button variant="secondary" onClick={() => setDialogOpen(false)}>取消</Button><Button loading={saveMutation.isPending} onClick={() => saveMutation.mutate()}>{approvalMode.required ? '提交变更' : '保存套餐'}</Button></>}
      >
        <div className="grid gap-4 sm:grid-cols-2">
          <Field label="套餐编码"><Input value={draft.code} disabled={!draft.isNew} onChange={(event) => setDraft((value) => ({ ...value, code: event.target.value }))} placeholder="starter" autoFocus={draft.isNew} /></Field>
          <Field label="套餐名称"><Input value={draft.name} onChange={(event) => setDraft((value) => ({ ...value, name: event.target.value }))} placeholder="入门版" /></Field>
          <Field label="套餐说明" className="sm:col-span-2"><Input value={draft.description} onChange={(event) => setDraft((value) => ({ ...value, description: event.target.value }))} placeholder="适用客户与权益边界" /></Field>
          <Field label="状态"><Select value={draft.status} onChange={(event) => setDraft((value) => ({ ...value, status: Number(event.target.value) }))}><option value={1}>启用</option><option value={2}>停用</option></Select></Field>
          <div className="hidden sm:block" />
          {coreLimitFields.map((field) => (
            <Field key={field.key} label={field.label}>
              <Input type="number" min={0} step={1} value={draft.limits[field.key] ?? field.defaultValue} onChange={(event) => setDraft((value) => ({ ...value, limits: { ...value.limits, [field.key]: Number(event.target.value) } }))} />
            </Field>
          ))}
        </div>
      </Dialog>
    </div>
  )
}
