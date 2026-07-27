import * as DialogPrimitive from '@radix-ui/react-dialog'
import { LoaderCircle, X } from 'lucide-react'
import type { ButtonHTMLAttributes, ChangeEventHandler, HTMLAttributes, InputHTMLAttributes, ReactNode } from 'react'
import { twMerge } from 'tailwind-merge'
import clsx, { type ClassValue } from 'clsx'

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'danger'

export function Button({
  className,
  variant = 'primary',
  loading = false,
  children,
  disabled,
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & { variant?: ButtonVariant; loading?: boolean }) {
  return (
    <button
      className={cn(
        'inline-flex h-9 items-center justify-center gap-2 rounded-md px-3 text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-emerald-600 focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50',
        variant === 'primary' && 'bg-emerald-700 text-white hover:bg-emerald-800',
        variant === 'secondary' && 'border border-zinc-300 bg-white text-zinc-800 hover:bg-zinc-50',
        variant === 'ghost' && 'text-zinc-600 hover:bg-zinc-100 hover:text-zinc-950',
        variant === 'danger' && 'bg-red-600 text-white hover:bg-red-700',
        className,
      )}
      disabled={disabled || loading}
      {...props}
    >
      {loading && <LoaderCircle className="h-4 w-4 animate-spin" aria-hidden="true" />}
      {children}
    </button>
  )
}

export function IconButton({ label, className, children, ...props }: ButtonHTMLAttributes<HTMLButtonElement> & { label: string }) {
  return (
    <button
      type="button"
      title={label}
      aria-label={label}
      className={cn('inline-flex h-9 w-9 shrink-0 items-center justify-center rounded-md text-zinc-600 hover:bg-zinc-100 hover:text-zinc-950 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-emerald-600', className)}
      {...props}
    >
      {children}
    </button>
  )
}

export function Badge({ tone = 'neutral', children, className }: { tone?: 'neutral' | 'success' | 'warning' | 'danger' | 'info'; children: ReactNode; className?: string }) {
  return (
    <span className={cn(
      'inline-flex min-h-6 items-center rounded-full border px-2 py-0.5 text-xs font-medium',
      tone === 'neutral' && 'border-zinc-200 bg-zinc-50 text-zinc-700',
      tone === 'success' && 'border-emerald-200 bg-emerald-50 text-emerald-800',
      tone === 'warning' && 'border-amber-200 bg-amber-50 text-amber-800',
      tone === 'danger' && 'border-red-200 bg-red-50 text-red-700',
      tone === 'info' && 'border-sky-200 bg-sky-50 text-sky-800',
      className,
    )}>
      {children}
    </span>
  )
}

export function PageHeader({ title, description, actions }: { title: string; description: string; actions?: ReactNode }) {
  return (
    <header className="flex min-h-16 flex-col justify-between gap-3 border-b border-zinc-200 pb-5 sm:flex-row sm:items-end">
      <div className="min-w-0">
        <h1 className="text-2xl font-semibold text-zinc-950">{title}</h1>
        <p className="mt-1 text-sm text-zinc-500">{description}</p>
      </div>
      {actions && <div className="flex shrink-0 flex-wrap items-center gap-2">{actions}</div>}
    </header>
  )
}

export function StatCard({ label, value, hint, icon }: { label: string; value: ReactNode; hint?: ReactNode; icon: ReactNode }) {
  return (
    <article className="rounded-lg border border-zinc-200 bg-white p-4 shadow-sm shadow-zinc-950/[0.02]">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <p className="text-sm text-zinc-500">{label}</p>
          <div className="mt-2 text-2xl font-semibold text-zinc-950">{value}</div>
          {hint && <div className="mt-1 text-xs text-zinc-500">{hint}</div>}
        </div>
        <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-md bg-zinc-100 text-zinc-700">{icon}</div>
      </div>
    </article>
  )
}

export function SectionHeader({ title, description, actions }: { title: string; description?: string; actions?: ReactNode }) {
  return (
    <div className="flex flex-col justify-between gap-2 sm:flex-row sm:items-end">
      <div>
        <h2 className="text-base font-semibold text-zinc-950">{title}</h2>
        {description && <p className="mt-1 text-sm text-zinc-500">{description}</p>}
      </div>
      {actions && <div className="flex shrink-0 items-center gap-2">{actions}</div>}
    </div>
  )
}

export function TableShell({ children, className }: { children: ReactNode; className?: string }) {
  return <div className={cn('overflow-x-auto rounded-lg border border-zinc-200 bg-white', className)}>{children}</div>
}

export function EmptyState({ icon, title, description, action }: { icon: ReactNode; title: string; description: string; action?: ReactNode }) {
  return (
    <div className="flex min-h-56 flex-col items-center justify-center px-6 py-10 text-center">
      <div className="flex h-11 w-11 items-center justify-center rounded-lg bg-zinc-100 text-zinc-500">{icon}</div>
      <h3 className="mt-4 text-base font-semibold text-zinc-900">{title}</h3>
      <p className="mt-1 max-w-md text-sm text-zinc-500">{description}</p>
      {action && <div className="mt-5">{action}</div>}
    </div>
  )
}

export function LoadingState({ label = '正在加载' }: { label?: string }) {
  return (
    <div className="flex min-h-48 items-center justify-center gap-2 text-sm text-zinc-500">
      <LoaderCircle className="h-4 w-4 animate-spin" aria-hidden="true" />
      {label}
    </div>
  )
}

export function ErrorState({ message, onRetry }: { message: string; onRetry?: () => void }) {
  return (
    <div className="flex min-h-48 flex-col items-center justify-center gap-4 px-6 text-center">
      <p className="text-sm text-red-700">{message}</p>
      {onRetry && <Button variant="secondary" onClick={onRetry}>重新加载</Button>}
    </div>
  )
}

export function Field({ label, hint, className, children }: { label: string; hint?: string; className?: string; children: ReactNode }) {
  return (
    <label className={cn('grid gap-1.5 text-sm font-medium text-zinc-700', className)}>
      <span>{label}</span>
      {children}
      {hint && <span className="text-xs font-normal text-zinc-500">{hint}</span>}
    </label>
  )
}

export const inputClassName = 'h-9 w-full rounded-md border border-zinc-300 bg-white px-3 text-sm text-zinc-950 outline-none placeholder:text-zinc-400 focus:border-emerald-600 focus:ring-2 focus:ring-emerald-600/15 disabled:bg-zinc-100 disabled:text-zinc-500'
export const textareaClassName = 'min-h-24 w-full resize-y rounded-md border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-950 outline-none placeholder:text-zinc-400 focus:border-emerald-600 focus:ring-2 focus:ring-emerald-600/15 disabled:bg-zinc-100 disabled:text-zinc-500'

export function Input(props: InputHTMLAttributes<HTMLInputElement>) {
  return <input {...props} className={cn(inputClassName, props.className)} />
}

export function Select(props: HTMLAttributes<HTMLSelectElement> & { value?: string | number; onChange?: ChangeEventHandler<HTMLSelectElement>; disabled?: boolean; children: ReactNode }) {
  return <select {...props} className={cn(inputClassName, props.className)} />
}

export function ProgressBar({ value, tone = 'success' }: { value: number; tone?: 'success' | 'warning' | 'danger' }) {
  const normalized = Math.max(0, Math.min(100, Number.isFinite(value) ? value : 0))
  return (
    <div className="h-1.5 w-full overflow-hidden rounded-full bg-zinc-100" role="progressbar" aria-valuemin={0} aria-valuemax={100} aria-valuenow={Math.round(normalized)}>
      <div
        className={cn('h-full rounded-full', tone === 'success' && 'bg-emerald-600', tone === 'warning' && 'bg-amber-500', tone === 'danger' && 'bg-red-600')}
        style={{ width: `${normalized}%` }}
      />
    </div>
  )
}

export function Dialog({ open, onOpenChange, title, description, children, footer, size = 'md' }: {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: string
  description?: string | undefined
  children: ReactNode
  footer?: ReactNode
  size?: 'sm' | 'md' | 'lg'
}) {
  return (
    <DialogPrimitive.Root open={open} onOpenChange={onOpenChange}>
      <DialogPrimitive.Portal>
        <DialogPrimitive.Overlay className="fixed inset-0 z-50 bg-zinc-950/35 data-[state=open]:animate-in data-[state=closed]:animate-out" />
        <DialogPrimitive.Content className={cn(
          'fixed left-1/2 top-1/2 z-50 flex max-h-[calc(100vh-2rem)] w-[calc(100vw-2rem)] -translate-x-1/2 -translate-y-1/2 flex-col overflow-hidden rounded-lg border border-zinc-200 bg-white shadow-xl outline-none',
          size === 'sm' && 'max-w-md',
          size === 'md' && 'max-w-xl',
          size === 'lg' && 'max-w-3xl',
        )}>
          <div className="flex items-start justify-between gap-4 border-b border-zinc-200 px-5 py-4">
            <div>
              <DialogPrimitive.Title className="text-base font-semibold text-zinc-950">{title}</DialogPrimitive.Title>
              {description && <DialogPrimitive.Description className="mt-1 text-sm text-zinc-500">{description}</DialogPrimitive.Description>}
            </div>
            <DialogPrimitive.Close asChild>
              <IconButton label="关闭"><X className="h-4 w-4" /></IconButton>
            </DialogPrimitive.Close>
          </div>
          <div className="overflow-y-auto px-5 py-5">{children}</div>
          {footer && <div className="flex flex-wrap justify-end gap-2 border-t border-zinc-200 bg-zinc-50 px-5 py-3">{footer}</div>}
        </DialogPrimitive.Content>
      </DialogPrimitive.Portal>
    </DialogPrimitive.Root>
  )
}

export function formatDate(value: string | undefined, fallback = '-') {
  if (!value) return fallback
  const normalized = value.includes('T') ? value : value.replace(' ', 'T')
  const date = new Date(normalized)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false }).format(date)
}

export function formatBytes(value: number) {
  if (!Number.isFinite(value) || value <= 0) return '-'
  if (value < 1024) return `${value} B`
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`
  return `${(value / 1024 / 1024).toFixed(1)} MB`
}
