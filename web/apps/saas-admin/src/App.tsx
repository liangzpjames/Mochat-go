import { useEffect, useMemo, useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Activity,
  Blocks,
  Boxes,
  Building2,
  ChevronDown,
  LayoutDashboard,
  LogOut,
  Menu,
  PanelLeftClose,
  PanelLeftOpen,
  RefreshCw,
  Rocket,
  ShieldCheck,
  UsersRound,
  X,
} from 'lucide-react'
import { apiRequest, hasPermission, loginURL, logoutSaaS, readStoredToken } from '@/lib/api'
import type { AccessProfileData, ApprovalPoliciesData, SystemHealthData, ViewKey } from '@/lib/types'
import { Badge, ErrorState, IconButton, LoadingState, cn } from '@/components/ui'
import OverviewPage from '@/pages/OverviewPage'
import TenantsPage from '@/pages/TenantsPage'
import PackagesPage from '@/pages/PackagesPage'
import TeamPage from '@/pages/TeamPage'
import SystemPage from '@/pages/SystemPage'
import LaunchPage from '@/pages/LaunchPage'
import LoginPage from '@/pages/LoginPage'

const views: Array<{
  key: ViewKey
  label: string
  icon: typeof LayoutDashboard
  permission: string
}> = [
  { key: 'overview', label: '概览', icon: LayoutDashboard, permission: 'platform.overview.read' },
  { key: 'tenants', label: '客户租户', icon: Building2, permission: 'platform.tenants.read' },
  { key: 'packages', label: '套餐', icon: Boxes, permission: 'platform.tenants.read' },
  { key: 'team', label: '平台账号', icon: UsersRound, permission: 'platform.access.manage' },
  { key: 'system', label: '系统运维', icon: Activity, permission: 'platform.system.read' },
  { key: 'launch', label: '上线检查', icon: Rocket, permission: 'platform.release.read' },
]

function currentViewFromURL(): ViewKey {
  const value = new URLSearchParams(location.search).get('view') as ViewKey | null
  return views.some((item) => item.key === value) ? value! : 'overview'
}

function AuthenticatedApp() {
  const queryClient = useQueryClient()
  const [view, setView] = useState<ViewKey>(currentViewFromURL)
  const [mobileOpen, setMobileOpen] = useState(false)
  const [accountOpen, setAccountOpen] = useState(false)
  const [collapsed, setCollapsed] = useState(() => localStorage.getItem('mochat_saas_admin_sidebar_collapsed') === '1')

  useEffect(() => {
    localStorage.setItem('mochat_saas_admin_sidebar_collapsed', collapsed ? '1' : '0')
  }, [collapsed])

  useEffect(() => {
    const handlePopState = () => setView(currentViewFromURL())
    window.addEventListener('popstate', handlePopState)
    return () => window.removeEventListener('popstate', handlePopState)
  }, [])

  const profileQuery = useQuery({
    queryKey: ['access-profile'],
    queryFn: () => apiRequest<AccessProfileData>('/dashboard/saasAdmin/accessProfile'),
  })

  const permissions = profileQuery.data?.profile.permissions || []
  const canReadApprovals = hasPermission(permissions, 'platform.approvals.read')
  const approvalQuery = useQuery({
    queryKey: ['approval-policies'],
    queryFn: () => apiRequest<ApprovalPoliciesData>('/dashboard/saasAdmin/approvalPolicies'),
    enabled: canReadApprovals,
  })
  const healthQuery = useQuery({
    queryKey: ['system-health', 'header'],
    queryFn: () => apiRequest<SystemHealthData>('/dashboard/saasAdmin/systemHealth?failureWindowHours=24&notificationStaleMinutes=15'),
    enabled: hasPermission(permissions, 'platform.system.read'),
    refetchInterval: 60_000,
  })

  const availableViews = useMemo(
    () => views.filter((item) => hasPermission(permissions, item.permission)),
    [permissions],
  )

  useEffect(() => {
    if (!profileQuery.data || availableViews.some((item) => item.key === view)) return
    const fallback = availableViews[0]?.key || 'overview'
    setView(fallback)
  }, [availableViews, profileQuery.data, view])

  const navigate = (next: ViewKey) => {
    const params = new URLSearchParams(location.search)
    params.set('view', next)
    history.pushState({}, '', `${location.pathname}?${params.toString()}`)
    setView(next)
    setMobileOpen(false)
    window.scrollTo({ top: 0, behavior: 'smooth' })
  }

  if (profileQuery.isLoading) return <div className="min-h-screen bg-white"><LoadingState label="正在进入总后台" /></div>
  if (profileQuery.isError || !profileQuery.data) {
    return (
      <div className="min-h-screen bg-white">
        <ErrorState message={profileQuery.error instanceof Error ? profileQuery.error.message : '无法加载平台账号'} onRetry={() => profileQuery.refetch()} />
      </div>
    )
  }

  const profile = profileQuery.data.profile
  const approvalMode = approvalQuery.data || { required: false, policies: [] }
  const activeView = views.find((item) => item.key === view) ?? views[0]!
  const health = healthQuery.data?.summary

  const pageProps = { navigate, profile, approvalMode }
  const page = {
    overview: <OverviewPage {...pageProps} />,
    tenants: <TenantsPage {...pageProps} />,
    packages: <PackagesPage {...pageProps} />,
    team: <TeamPage {...pageProps} />,
    system: <SystemPage {...pageProps} />,
    launch: <LaunchPage {...pageProps} />,
  }[view]

  const signOut = () => {
    void logoutSaaS().finally(() => location.assign('/saas/login'))
  }

  return (
    <div className="min-h-screen bg-[#f4f5f7] text-zinc-900">
      {mobileOpen && (
        <button
          type="button"
          aria-label="关闭导航"
          className="fixed inset-0 z-40 bg-zinc-950/30 lg:hidden"
          onClick={() => setMobileOpen(false)}
        />
      )}

      <aside className={cn(
        'fixed inset-y-0 left-0 z-50 flex flex-col border-r border-zinc-200 bg-white transition-[width,transform] duration-200',
        collapsed ? 'w-[72px]' : 'w-60',
        mobileOpen ? 'translate-x-0' : '-translate-x-full lg:translate-x-0',
      )}>
        <div className="flex h-16 items-center gap-3 border-b border-zinc-200 px-4">
          <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-emerald-700 text-white">
            <Blocks className="h-5 w-5" aria-hidden="true" />
          </div>
          {!collapsed && <div className="min-w-0"><strong className="block truncate text-sm">MoChat SaaS</strong><span className="block text-xs text-zinc-500">总后台</span></div>}
          <IconButton label="关闭导航" className="ml-auto lg:hidden" onClick={() => setMobileOpen(false)}><X className="h-4 w-4" /></IconButton>
        </div>

        <nav className="flex-1 overflow-y-auto px-2 py-3" aria-label="总后台导航">
          <div className="space-y-1">
            {availableViews.map((item) => {
              const Icon = item.icon
              const selected = item.key === view
              return (
                <button
                  key={item.key}
                  type="button"
                  title={collapsed ? item.label : undefined}
                  className={cn(
                    'flex h-10 w-full items-center gap-3 rounded-md px-3 text-sm font-medium transition-colors',
                    collapsed && 'justify-center px-0',
                    selected ? 'bg-emerald-50 text-emerald-800' : 'text-zinc-600 hover:bg-zinc-100 hover:text-zinc-950',
                  )}
                  onClick={() => navigate(item.key)}
                >
                  <Icon className="h-[18px] w-[18px] shrink-0" aria-hidden="true" />
                  {!collapsed && <span>{item.label}</span>}
                </button>
              )
            })}
          </div>
        </nav>

        <div className="border-t border-zinc-200 p-2">
          <button
            type="button"
            title={collapsed ? profile.userName : undefined}
            className={cn('flex h-11 w-full items-center gap-3 rounded-md px-2 text-left hover:bg-zinc-100', collapsed && 'justify-center')}
            onClick={() => setAccountOpen((value) => !value)}
          >
            <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-zinc-900 text-xs font-semibold text-white">{profile.userName.slice(0, 1)}</span>
            {!collapsed && <><span className="min-w-0 flex-1"><strong className="block truncate text-sm font-medium">{profile.userName}</strong><span className="block truncate text-xs text-zinc-500">{profile.phone}</span></span><ChevronDown className="h-4 w-4 text-zinc-400" /></>}
          </button>
          {accountOpen && !collapsed && (
            <div className="absolute bottom-16 left-3 w-[216px] rounded-lg border border-zinc-200 bg-white p-1 shadow-lg">
              <button type="button" className="flex h-9 w-full items-center gap-2 rounded-md px-3 text-sm text-zinc-700 hover:bg-zinc-100" onClick={signOut}>
                <LogOut className="h-4 w-4" />退出登录
              </button>
            </div>
          )}
        </div>
      </aside>

      <div className={cn('min-h-screen transition-[padding] duration-200', collapsed ? 'lg:pl-[72px]' : 'lg:pl-60')}>
        <header className="sticky top-0 z-30 flex h-16 items-center gap-3 border-b border-zinc-200 bg-white/95 px-4 backdrop-blur sm:px-6">
          <IconButton label="打开导航" className="lg:hidden" onClick={() => setMobileOpen(true)}><Menu className="h-5 w-5" /></IconButton>
          <IconButton label={collapsed ? '展开侧栏' : '收起侧栏'} className="hidden lg:inline-flex" onClick={() => setCollapsed((value) => !value)}>
            {collapsed ? <PanelLeftOpen className="h-5 w-5" /> : <PanelLeftClose className="h-5 w-5" />}
          </IconButton>
          <div className="min-w-0 flex-1 text-sm"><span className="text-zinc-400">SaaS 总后台</span><span className="px-2 text-zinc-300">/</span><strong className="font-medium text-zinc-800">{activeView.label}</strong></div>
          {health && (
            <Badge tone={health.healthState === 'healthy' ? 'success' : health.healthState === 'warning' ? 'warning' : 'danger'} className="hidden sm:inline-flex">
              <ShieldCheck className="mr-1 h-3.5 w-3.5" />{health.healthState === 'healthy' ? '系统正常' : `${health.issueCount} 项异常`}
            </Badge>
          )}
          {!approvalMode.required && <Badge tone="info" className="hidden md:inline-flex">单人运营</Badge>}
          <IconButton label="刷新当前数据" onClick={() => queryClient.invalidateQueries()}><RefreshCw className="h-4 w-4" /></IconButton>
        </header>
        <main className="mx-auto w-full max-w-[1600px] p-4 sm:p-6 lg:p-7">{page}</main>
      </div>
    </div>
  )
}

function App() {
  const loginRoute = location.pathname === '/saas/login' || new URLSearchParams(location.search).get('login') === '1'
  const hasToken = Boolean(readStoredToken())

  useEffect(() => {
    if (!hasToken && !loginRoute) location.assign(loginURL())
  }, [hasToken, loginRoute])

  if (!hasToken && loginRoute) return <LoginPage />
  if (!hasToken) return <div className="min-h-screen bg-white"><LoadingState label="Loading" /></div>
  return <AuthenticatedApp />
}

export default App
