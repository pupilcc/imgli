import type { ButtonHTMLAttributes, ReactNode, SelectHTMLAttributes } from 'react'
import { Link } from 'react-router'
import { cn } from '../../../lib/cn'

/** Shared admin filter row (PageHeader.extra). */
export function AdminFilters({ children, className }: { children: ReactNode; className?: string }) {
  return <div className={cn('flex flex-wrap items-center gap-2', className)}>{children}</div>
}

/** Native select styled for admin filters / forms. */
export function AdminSelect({ className, ...rest }: SelectHTMLAttributes<HTMLSelectElement>) {
  return (
    <select
      className={cn(
        'h-[34px] rounded-sm border border-border bg-surface px-2.5 font-inherit text-sm-plus text-ink',
        className,
      )}
      {...rest}
    />
  )
}

/** Compact search field for admin lists. */
export function AdminSearch({
  value,
  onChange,
  placeholder,
  className,
}: {
  value: string
  onChange(v: string): void
  placeholder?: string
  className?: string
}) {
  return (
    <div
      className={cn(
        'flex h-[34px] w-[230px] items-center gap-2 rounded-sm border border-border bg-surface px-3',
        className,
      )}
    >
      <span className="text-[13px] text-muted" aria-hidden>
        ⌕
      </span>
      <input
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        className="w-full border-0 bg-transparent font-inherit text-[13px] text-ink outline-none"
      />
    </div>
  )
}

/** Card-like table shell. */
export function AdminTable({ children, className, minWidth }: { children: ReactNode; className?: string; minWidth?: number }) {
  return (
    <div className={cn('overflow-x-auto rounded-sm border border-border bg-surface', className)}>
      <div style={minWidth ? { minWidth } : undefined}>{children}</div>
    </div>
  )
}

export function AdminTableHead({ children, className, columns }: { children: ReactNode; className?: string; columns: string }) {
  return (
    <div
      className={cn(
        'grid items-center gap-x-3 border-b border-border bg-soft px-4 py-2 font-mono text-2xs tracking-[0.06em] text-muted uppercase',
        className,
      )}
      style={{ gridTemplateColumns: columns }}
      role="row"
    >
      {children}
    </div>
  )
}

export function AdminTableRow({
  children,
  className,
  columns,
}: {
  children: ReactNode
  className?: string
  columns: string
}) {
  return (
    <div
      className={cn('grid items-center gap-x-3 border-b border-border px-4 py-2.5 last:border-b-0', className)}
      style={{ gridTemplateColumns: columns }}
      role="row"
    >
      {children}
    </div>
  )
}

/** Sortable column header button. */
export function AdminSortTh({
  label,
  active,
  align = 'start',
  onClick,
  sortAria,
}: {
  label: string
  active?: boolean
  align?: 'start' | 'end'
  onClick(): void
  /** e.g. "Sort by Bandwidth" — falls back to label */
  sortAria?: string
}) {
  return (
    <button
      type="button"
      className={cn(
        'inline-flex max-w-full cursor-pointer items-center gap-1 border-0 bg-transparent p-0 font-inherit tracking-inherit text-inherit uppercase',
        align === 'end' ? 'justify-self-end' : 'justify-self-start',
        active && 'font-bold text-ink',
      )}
      aria-label={sortAria ?? label}
      aria-pressed={!!active}
      onClick={onClick}
    >
      <span>{label}</span>
      <span className="flex-none text-[9px] opacity-65" aria-hidden>
        {active ? '▼' : '↕'}
      </span>
    </button>
  )
}

/** Status pill for active/banned/ok/err. */
export function StatusPill({
  ok,
  children,
  className,
}: {
  ok: boolean
  children: ReactNode
  className?: string
}) {
  return (
    <span
      className={cn(
        'justify-self-start whitespace-nowrap rounded-sm border bg-surface px-[7px] py-0.5 font-mono text-[9.5px] tracking-[0.08em]',
        ok ? 'border-ok text-ok' : 'border-err text-err',
        className,
      )}
    >
      {children}
    </span>
  )
}

/** Icon-sized action control (works with ArmedButton className). */
export const iconActionClass =
  'inline-flex h-7 min-w-7 cursor-pointer items-center justify-center rounded-sm border border-border bg-surface px-1.5 font-mono text-xs leading-none text-muted hover:enabled:border-muted hover:enabled:text-ink disabled:cursor-not-allowed disabled:opacity-35'

export const iconActionArmedClass =
  'min-w-0 border-err px-2 text-2xs tracking-[0.02em] text-err'

export const iconActionArmedOkClass =
  'min-w-0 border-ok px-2 text-2xs tracking-[0.02em] text-ok'

export function IconLink({
  to,
  title,
  children,
  className,
}: {
  to: string
  title: string
  children: ReactNode
  className?: string
}) {
  return (
    <Link to={to} title={title} aria-label={title} className={cn(iconActionClass, 'no-underline', className)}>
      {children}
    </Link>
  )
}

export function ThemeIconButton(props: ButtonHTMLAttributes<HTMLButtonElement>) {
  return (
    <button
      type="button"
      {...props}
      className={cn(
        'flex h-[30px] w-[30px] cursor-pointer items-center justify-center rounded-sm border border-border bg-surface text-sm text-ink hover:bg-soft',
        props.className,
      )}
    />
  )
}

export function AdminField({
  label,
  hint,
  children,
  className,
}: {
  label?: ReactNode
  hint?: ReactNode
  children: ReactNode
  className?: string
}) {
  return (
    <div className={cn('flex flex-col gap-1.5', className)}>
      {label && <span className="text-[13px] text-muted">{label}</span>}
      {children}
      {hint && <span className="text-xs leading-snug text-muted">{hint}</span>}
    </div>
  )
}
