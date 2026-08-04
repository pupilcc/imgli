import type { ReactNode } from 'react'
import { cn } from '../lib/cn'

interface Props {
  badge?: string
  title: string
  desc?: string
  children?: ReactNode
  className?: string
}

export function EmptyState({ badge = 'EMPTY', title, desc, children, className }: Props) {
  return (
    <div className={cn('animate-[fadeIn_0.2s] px-4 py-12 text-center', className)}>
      <div className="stripe mx-auto mb-3.5 flex h-[90px] w-[120px] items-center justify-center rounded-sm border border-border">
        <span className="font-mono text-[9px] tracking-[0.1em] text-muted">{badge}</span>
      </div>
      <div className="mb-1 text-[13px] font-bold">{title}</div>
      {desc && <div className="mb-3.5 text-[11px] text-muted">{desc}</div>}
      {children && <div className="flex justify-center">{children}</div>}
    </div>
  )
}
